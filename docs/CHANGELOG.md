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