# Pulse — uptime monitoring & status pages

Built by Buam Technologies Inc. as a reference SaaS implementation for a
multi-tenant monitoring platform. Go services, React frontends, Postgres,
Redis; deployed to k3s (dev) and EKS (stage/prod) via Helm and ArgoCD.

## Prerequisites (macOS)

| Tool | Why | Install |
|------|-----|---------|
| Docker Desktop | runs Postgres + Redis locally, later k3s | https://www.docker.com/products/docker-desktop |
| Homebrew | package manager for everything below | see below |
| Go 1.22+ | the three backend services | `brew install go` |
| golang-migrate | applies SQL migrations | `brew install golang-migrate` |
| golangci-lint | linting (`make lint`) | `brew install golangci-lint` |

```bash
# Homebrew (skip if `which brew` prints a path)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

brew install go golang-migrate golangci-lint
go version && migrate -version
```

If `go` is still "command not found" on Apple Silicon, Homebrew is not on your
PATH yet:

```bash
echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
source ~/.zprofile
```

Run `make doctor` at any time to confirm every tool is present.

## Phase 1 — running locally

One-time setup:

```bash
cp .env.example .env
make doctor        # verifies docker, go, migrate are installed
make up            # starts postgres + redis in Docker
make migrate-up    # creates the schema
go mod tidy        # downloads Go dependencies (writes go.sum — commit it)
make test          # unit tests, should pass
```

The three services are separate processes. Open **three terminal tabs**
(Cmd+T in Terminal/iTerm), `cd` into the repo in each, and run one per tab:

| Tab | Command | What you should see |
|-----|---------|---------------------|
| 1 | `make run-api` | `"msg":"api listening","port":"8080"` |
| 2 | `make run-scheduler` | `"msg":"enqueued","count":N` every interval |
| 3 | `make run-worker` | `"msg":"checked","ok":true,...` per monitor |

Stop any service with Ctrl+C; it shuts down gracefully.

Seed a tenant and a monitor (fourth tab, or a DB client):

```bash
docker compose exec postgres psql -U pulse -d pulse
```
```sql
INSERT INTO tenants (name, slug) VALUES ('Northgate Digital','northgate') RETURNING id;
-- paste the returned id below
INSERT INTO monitors (tenant_id, name, type, target)
VALUES ('<tenant id>', 'Example site', 'http', 'https://example.com');
```

Within ~60s tab 2 logs an enqueue, tab 3 logs a check, and
`SELECT * FROM check_results ORDER BY checked_at DESC LIMIT 5;` shows results.

Smoke-test the API: `curl -i localhost:8080/readyz` (expect 200) and
`curl localhost:8080/metrics | head`.

Tear down: `make down` (data persists in the `pgdata` volume;
`docker compose down -v` wipes it).

## Repository layout

```
apps/        api, scheduler, worker (Go) — web, status (React) added in Phase 3
internal/    shared Go packages: config, db, queue, checks
migrations/  SQL schema, applied with golang-migrate
deploy/      Helm chart + ArgoCD manifests (Phase 2/4)
infra/       Terraform modules + Ansible (Phase 4)
docs/adr/    architecture decision records
```

## Roadmap

1. Local Go services (this phase)
2. Dockerfiles, Helm chart, k3s on Docker Desktop
3. GitHub Actions CI, React dashboard + status pages
4. Terraform: VPC, EKS, RDS, ElastiCache; ArgoCD stage
5. Prod, alerting, SLO dashboards, autoscaling
6. Chaos exercises, postmortems, runbooks



## API walkthrough (Phase 1 step 2)

Add `JWT_SECRET=<any long random string>` to your `.env`, restart the API, then:

```bash
# 1. sign up — creates the tenant and returns a token
curl -s localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"company":"Northgate Digital","email":"you@example.com","password":"correct-horse-battery"}'

export TOKEN=<paste token>

# 2. create a monitor
curl -s localhost:8080/api/v1/monitors -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Example","type":"http","target":"https://example.com","interval_seconds":30}'

# 3. list monitors (status flips to "up" after the first check)
curl -s localhost:8080/api/v1/monitors -H "Authorization: Bearer $TOKEN"

# 4. results
curl -s localhost:8080/api/v1/monitors/<id>/results -H "Authorization: Bearer $TOKEN"
```