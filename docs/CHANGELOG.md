# Changelog

## 2026-09-08 — Phase 1 scaffold
- Initial schema (tenants, users, monitors, check_results, incidents, alert_channels)
- Go services: api, scheduler, worker; shared packages config, db, queue, checks
- docker-compose for Postgres + Redis; Makefile targets
- ADR 0001: monorepo and service split
- README: added macOS prerequisites (Homebrew, Go, golang-migrate, golangci-lint),
  PATH fix for Apple Silicon, and the three-terminal run layout
- Makefile: added `make doctor` to verify tooling before first run


## 2026-09-09 — Phase 1 step 2: real API
- Signup (tenant + owner user in one transaction), login, JWT auth middleware
- Monitors: list, create, delete, and last-100 results — all tenant-scoped
- Router moved to internal/http so it can be tested without a live server
- New env var JWT_SECRET (see .env.example); new deps golang-jwt, x/crypto
- Fix: Config struct was missing the JWTSecret field
- Fix: checks_test used t.Context() (Go 1.24+); switched to context.Background() to honour go.mod's 1.22

- Phase 1 verified end to end: signup → create monitor → scheduler enqueue → worker check → status "up"


## 2026-09-09 — Phase 1 step 3: Prometheus metrics
- internal/metrics: checks total/latency, queue depth, scheduler enqueued, API request count/latency
- Worker and scheduler expose /metrics and /healthz on METRICS_PORT (scheduler runs on 9091 locally)
- API records per-route metrics via middleware

- Phase 1 complete: all services verified locally with metrics exposed


## 2026-09-09 — Phase 2 step 1: containers
- migrate command with embedded SQL; runs as a K8s Job before deploys
- Single multi-stage Dockerfile (Go build → distroless static, non-root) with SERVICE build arg
- make docker-build produces pulse-{api,scheduler,worker,migrate}:dev
-Dev cluster is Docker Desktop Kubernetes (6 nodes), not k3s

- `go.mod bumped to Go 1.25 by golang-migrate v4.20 dependency; Dockerfile build image pinned to golang:1.25-alpine to match. Rule: go.mod, Dockerfile, and CI must agree on the Go version.`


## 2026-09-09 — Phase 2 step 2: Helm chart + dev cluster
- Helm chart deploy/helm/pulse: api (2 replicas prod / 1 dev), scheduler (Recreate strategy), worker (N replicas), migrate Job as pre-install/upgrade hook
- Dev-only in-cluster Postgres (PVC) and Redis, toggled by postgres.enabled / redis.enabled
- Secrets from values (dev); stage/prod will source from AWS Secrets Manager
- Pods run non-root (uid 65532), have readiness/liveness probes, requests/limits, Prometheus scrape annotations
- make kind-load, deploy-dev, undeploy-dev
- Fix: migrate hook changed from pre-install to post-install,pre-upgrade (dev Postgres lives in the chart). See ADR 0002.