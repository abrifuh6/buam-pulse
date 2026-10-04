# From your laptop to production

Everything needed to work on Pulse and ship it, in order. Written so someone
who has never seen this repo can follow it.

---

## 1. What you need installed

```bash
make doctor
```

That checks for everything. If it complains:

| Tool | Why | Install |
|---|---|---|
| Go 1.26+ | The backend | `brew install go` |
| Node 22+ | The frontends | `brew install node` |
| Docker Desktop | Containers, and the local Kubernetes cluster | docker.com |
| golang-migrate | Database migrations | `brew install golang-migrate` |
| Helm | Deploying to Kubernetes | `brew install helm` |
| Terraform 1.10+ | AWS infrastructure | `brew install terraform` |
| AWS CLI | Talking to AWS | `brew install awscli` |
| Stripe CLI | Testing billing locally | `brew install stripe/stripe-cli/stripe` |

---

## 2. Running it locally

```bash
cp .env.example .env     # then fill in the Stripe keys if you need billing
make dev-up              # Postgres, Redis, Mailpit, and all five Go services
make migrate-up          # create the database tables
```

`make dev-up` runs the Go services in containers with hot reload — change a
`.go` file and the service rebuilds in a couple of seconds. The frontends run
separately because Vite's reload is faster than a container rebuild:

```bash
cd apps/web && npm run dev      # dashboard        localhost:5173
cd apps/status && npm run dev   # status pages     localhost:5174
cd apps/site && npm run dev     # marketing site   localhost:5175
```

| | |
|---|---|
| API | http://localhost:8080 |
| Mail (everything sent lands here) | http://localhost:8025 |
| `make dev-logs` | all logs; `S=worker` for one service |
| `make dev-restart S=worker` | restart one service |
| `make dev-down` | stop |

**Why containers rather than `go run`?** Running services by hand left orphaned
processes across terminal tabs. Three workers once ended up racing on the same
queue, producing alternating pass/fail results that looked exactly like a
flapping monitor. Compose replaces containers rather than stacking them, which
makes that impossible.

### Billing locally

```bash
make stripe-setup        # creates products and prices in your Stripe sandbox
stripe listen --forward-to localhost:8080/api/v1/billing/webhook
```

The webhook listener must be running or plan changes never reach the app.

---

## 3. Making a change

### Changing the database

Migrations are numbered pairs in `migrations/`. Never edit an applied one —
add a new one.

```bash
# create 000014_your_change.up.sql and .down.sql by hand, then:
make migrate-up
```

The down file matters. It is how you reverse a bad migration without restoring
a backup.

### Changing a Go service

Save the file. Hot reload rebuilds it. Watch for errors:

```bash
make dev-logs S=api
```

Before committing:

```bash
make lint        # golangci-lint
make test        # go test with the race detector
```

### Changing a frontend

Vite reloads in the browser automatically. Before committing:

```bash
cd apps/web && npm run build     # this type-checks as well as builds
```

### Changing infrastructure

```bash
cd infra/terraform/environments/prod
terraform fmt
terraform validate
checkov -d . --compact --quiet    # security scan
terraform plan                    # read this properly before applying
```

Checkov will report skipped checks. Each one has a comment in the code
explaining why it was accepted. If you add a new skip, write the reason — an
undocumented suppression looks like diligence while hiding a decision nobody
argued for.

---

## 4. What CI does

Four jobs on every push:

| Job | What it checks |
|---|---|
| `test` | `go mod tidy` is clean, vet, golangci-lint, tests with the race detector, `helm lint`, all three frontends type-check |
| `infra` | `terraform fmt`, `terraform validate`, Checkov scan uploaded to the Security tab |
| `build` | Builds nine images in parallel, scans each with Trivy |
| `push` | Publishes to GHCR and Docker Hub, tagged by commit SHA |

**Trivy fails the build** on any critical or high vulnerability that has a fix
available. Unfixable ones are ignored — there is nothing to do about them and
blocking on them trains people to disable the scanner.

Checkov does **not** fail the build. Every remaining finding is deliberate and
documented; failing on them would push people toward blanket suppressions.

---

## 5. Deploying to AWS

### First time, or after a teardown

```bash
cd infra/terraform/environments/prod
terraform init
terraform apply          # ~18 minutes
```

Then connect and install the two cluster controllers. These are not managed by
Terraform because they are Kubernetes resources, not AWS ones:

```bash
aws eks update-kubeconfig --region ca-central-1 --name pulse-prod
kubectl get nodes        # expect two, Ready

ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
VPC_ID=$(aws ec2 describe-vpcs --region ca-central-1 \
  --filters "Name=tag:Name,Values=pulse-vpc" --query 'Vpcs[0].VpcId' --output text)

helm repo add eks https://aws.github.io/eks-charts
helm repo add external-secrets https://charts.external-secrets.io
helm repo update

helm upgrade --install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=pulse-prod \
  --set serviceAccount.create=true \
  --set serviceAccount.name=aws-load-balancer-controller \
  --set "serviceAccount.annotations.eks\.amazonaws\.com/role-arn=arn:aws:iam::${ACCOUNT}:role/pulse-prod-alb-controller" \
  --set region=ca-central-1 --set vpcId="${VPC_ID}" --wait

helm upgrade --install external-secrets external-secrets/external-secrets \
  -n external-secrets --create-namespace \
  --set installCRDs=true \
  --set serviceAccount.create=true \
  --set serviceAccount.name=external-secrets \
  --set "serviceAccount.annotations.eks\.amazonaws\.com/role-arn=arn:aws:iam::${ACCOUNT}:role/pulse-prod-external-secrets" \
  --wait
```

### Build and push the images

ECR tags are immutable, so every deploy needs a new tag. This is deliberate —
a tag always means exactly one artifact.

```bash
export TAG=$(date +%Y%m%d-%H%M%S)
export REGISTRY=$(cd infra/terraform/environments/prod && terraform output -raw ecr_registry)/pulse
export ALB=$(kubectl get ingress pulse -n pulse -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')

aws ecr get-login-password --region ca-central-1 \
  | docker login --username AWS --password-stdin "${REGISTRY%/pulse}"

for s in api scheduler worker notifier retention migrate; do
  docker buildx build --platform linux/amd64 --build-arg SERVICE="${s}" \
    -t "${REGISTRY}/${s}:${TAG}" --push . || break
done

docker buildx build --platform linux/amd64 -f Dockerfile.web --build-arg APP=web \
  -t "${REGISTRY}/web:${TAG}" --push .
docker buildx build --platform linux/amd64 -f Dockerfile.web --build-arg APP=status \
  -t "${REGISTRY}/status:${TAG}" --push .
docker buildx build --platform linux/amd64 -f Dockerfile.web --build-arg APP=site \
  --build-arg VITE_APP_URL="http://${ALB}/app" \
  -t "${REGISTRY}/site:${TAG}" --push .
```

**Note on `--platform linux/amd64`:** the nodes are x86 and a Mac is ARM. Without
this the images build for the wrong architecture and the pods fail to start.

**Note on `VITE_APP_URL`:** the marketing site's links are baked in at build
time, because a static site has no server to read environment variables. Change
the dashboard's address and the site must be rebuilt.

### Deploy

```bash
kubectl create namespace pulse --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install pulse deploy/helm/pulse \
  -n pulse -f deploy/helm/pulse/values-prod.yaml \
  --set image.tag="${TAG}" --wait --timeout 10m
```

Do not interrupt this. A cancelled upgrade leaves the release in
`pending-upgrade` and the next attempt refuses to run. If that happens:

```bash
helm rollback pulse -n pulse
kubectl delete secret -n pulse -l owner=helm,status=pending-upgrade
```

### Verify

```bash
kubectl get pods -n pulse
curl -s -o /dev/null -w "site: %{http_code}\n" "http://${ALB}/"
curl -s -o /dev/null -w "dashboard: %{http_code}\n" "http://${ALB}/app/"
curl -s -o /dev/null -w "api: %{http_code}\n" "http://${ALB}/api/v1/monitors"
```

The API returning **401** is correct — it means the API is reachable and
rejecting an unauthenticated request.

---

## 6. Tearing down

```bash
make aws-down
```

**Do not run `terraform destroy` on its own.** The load balancer is created by
a controller inside the cluster, so Terraform does not know it exists, and its
network interfaces block subnet deletion. `make aws-down` uninstalls the Helm
release first so the controller cleans up its own resources.

If a destroy fails anyway with `DependencyViolation`, something is still
holding the network:

```bash
VPC=<the vpc id from the error>

aws ec2 describe-network-interfaces --region ca-central-1 \
  --filters "Name=vpc-id,Values=$VPC" \
  --query 'NetworkInterfaces[].[NetworkInterfaceId,Description]' --output text

aws ec2 describe-security-groups --region ca-central-1 \
  --filters "Name=vpc-id,Values=$VPC" \
  --query 'SecurityGroups[?GroupName!=`default`].[GroupId,GroupName]' --output text
```

Security groups named `k8s-*` are leftovers from the load balancer controller.
Delete the `k8s-traffic-*` one first — the other references it.

---

## 7. When things break

### Pods crash-looping with empty logs

Almost always the database. Check in this order:

```bash
# Did External Secrets fetch the credentials?
kubectl get externalsecret -n pulse          # want SecretSynced / True

# Can a pod reach the database at all?
kubectl run psql-check --rm -i --restart=Never -n pulse \
  --image=postgres:16-alpine \
  --overrides='{"spec":{"containers":[{"name":"psql-check","image":"postgres:16-alpine","command":["sh","-c","psql \"$DATABASE_URL\" -c \"\\dt\" | head -20"],"envFrom":[{"secretRef":{"name":"pulse-secrets"}}]}]}}'
```

**"Did not find any relations"** → the database is empty, migrations never ran:

```bash
kubectl run pulse-migrate --rm -i --restart=Never -n pulse \
  --image="${REGISTRY}/migrate:${TAG}" \
  --overrides='{"spec":{"containers":[{"name":"pulse-migrate","image":"'"${REGISTRY}"'/migrate:'"${TAG}"'","envFrom":[{"secretRef":{"name":"pulse-secrets"}}]}]}}'
```

**"Operation timed out"** → a network problem, not a database problem. Timeout
means packets were dropped; refused would mean something rejected them. Check
that the security group allows the **EKS-managed cluster security group**, not
just the node group — pods get their own network interfaces carrying the
cluster group.

### Ingress has no address

```bash
kubectl describe ingress -n pulse | tail -20
kubectl logs -n kube-system -l app.kubernetes.io/name=aws-load-balancer-controller --tail=30
```

`TargetGroup port is empty` means the Ingress is missing its ALB annotations,
specifically `alb.ingress.kubernetes.io/target-type: ip`.

### Image won't pull

```bash
kubectl get pods -n pulse -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\n"}{end}'
```

Compare against what is actually in ECR. The usual cause is a mismatch between
the registry path and what the chart renders.

---

## 8. Known gaps

Be aware of these before relying on any of it:

- **The notifier and retention services are not in the Helm chart.** They run
  locally but are never deployed to AWS, which means **no alerts are sent in
  production**. This is the most important gap.
- **No GitOps.** Deployment is a manual `helm upgrade` from a laptop. Nothing
  reconciles the cluster if someone changes it by hand.
- **No HTTPS.** The ALB serves plain HTTP. Needs an ACM certificate and a real
  domain.
- **No monitoring of Pulse itself.** If it stops checking, nobody finds out.
- **Single region, single-AZ database.** Deliberate cost decisions, and the
  first things to change for anything carrying real traffic.
