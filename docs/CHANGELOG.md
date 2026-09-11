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
- Phase 2 complete: full stack verified on Docker Desktop Kubernetes (6 pods), end-to-end check passing through the cluster


## 2026-09-09 — Phase 3 step 1: CI
- GitHub Actions: vet, golangci-lint, race tests, helm lint → matrix build of 4 images → Trivy scan (fails on unfixed CRITICAL/HIGH) → push to GHCR and Docker Hub (buamtech)
- Tags: git SHA always, latest on main, semver on v* tags; PRs build+scan only
- .golangci.yml added
- Dependencies updated for Trivy findings; toolchain moved to Go 1.26 (go.mod, Dockerfile, CI together)
- Removed chi RealIP middleware: trusts X-Forwarded-For from any client (GHSA-3fxj-6jh8-hvhx). Client IP handling will be done at the load balancer in Phase 4.
- CI publishes multi-arch images (linux/amd64 + linux/arm64) via QEMU + buildx; needed for Apple Silicon dev and Graviton nodes


## 2026-09-09 — Phase 3 step 3: React dashboard
- apps/web: Vite + React + TypeScript dashboard — signup/login, monitor list with status dot, latency sparkline (last 30 checks), add/delete, 15s polling
- Vite dev proxy sends /api to :8080 so dev is same-origin, matching the planned production routing
- Token in localStorage with Bearer header; ADR 0003 records the plan to move to HttpOnly cookies


## 2026-09-09 — Phase 3 step 4: public status page
- apps/status: standalone React app on :5174, reads /api/v1/public/status/{slug}, no auth
- Light theme, overall banner (all operational / N down / pending), per-service uptime and average latency, 60s refresh
- Deliberately a separate origin from the dashboard; slug comes from the URL path


## 2026-09-10 — Phase 3.5 step 1: alerting foundations + SSRF protection
- Migration 000002: alert channel verification, monitor→channel routing, notifications table with per-attempt state and UNIQUE(incident, channel, kind) idempotency
- internal/checks/validate.go: rejects loopback, private, link-local, CGNAT and cloud-metadata targets at creation; ResolveGuard re-checks DNS at check time (rebinding defence)
- Mailpit added to docker-compose for local mail (arm64-native); SMTP settings via env so SES drops in for stage/prod
## 2026-09-10 — Phase 3.5 step 2: alerting works end to end
- internal/alerting: incident open/close in one transaction with a row lock; notifications queued per channel
- apps/notifier: separate service draining the queue with exponential backoff (1→32 min) and a dead state after 6 attempts
- Alert channels API: email (with mandatory verification) and Slack (https hooks.slack.com only); webhook URLs masked when read back; per-channel test endpoint
- Config split: APP_URL for verification links, STATUS_URL for status-page links — different origins per ADR 0003
- Metrics: pulse_notifications_total{channel,kind,outcome}, pulse_notification_backlog
- Verified: failure → 2 consecutive fails → incident → DOWN email; recovery → incident resolved → RECOVERED email with downtime duration
- Dashboard: Monitors/Alerts tabs; per-monitor edit, pause and resume (resume schedules an immediate check); channel management with test-send
- PATCH /monitors/{id}: partial update via nullable fields; monitor type is immutable

## 2026-09-10 — Phase 3.5 step 3: auth hardening
- Password reset and signup email verification via single-use, time-limited tokens; only SHA-256 hashes stored, so a DB leak yields no usable links
- /auth/forgot returns the same response for known and unknown addresses (no account-enumeration oracle)
- Reset claims the token and updates the password in one transaction, and voids the user's other outstanding reset tokens
- Rate limiting on all auth endpoints: 10 requests per IP per minute, in-memory (per-pod) — see ADR 0005 for the Redis follow-up
- X-Forwarded-For only trusted when TRUST_PROXY=true; header is spoofable when the API is directly reachable
- Verified: 429 after 10 attempts; reset works end to end; token reuse rejected

## 2026-09-10 — Phase 3.5 step 4: incidents on the status page
- Public endpoint returns per-day uptime for 90 days (generate_series so gaps stay visible), active incidents, and 90 days of resolved incidents
- Status page renders uptime bars, an "investigating" block for open incidents, and a past-incidents list with durations
- Known N+1: daily uptime is one query per monitor; behind the 30s cache for now, single grouped query is the fix

## 2026-09-10 — Phase 3.6 step 1: teams and roles
- Roles enforced server-side via RequireRole middleware ranked owner > admin > member; reads open to all, writes admin+, membership changes owner-only
- Invitations: hashed single-use tokens, 7-day expiry, re-invite replaces rather than duplicates, accept creates the user and consumes the invite in one transaction
- Ownership transfer demotes the current owner in the same transaction, so a tenant never has zero or two owners
- Config split three ways: API_URL for endpoint links, DASHBOARD_URL for links to app pages, STATUS_URL for status pages
- Known limitation: a role change takes effect only when the user's 24h token is reissued; token versioning is the follow-up

## 2026-09-10 — Phase 3.6 step 2: plan limits
- plans table holds tiers as data (free/starter/pro); changing a limit is an UPDATE, not a deploy
- Limits enforced inside a transaction with the tenant row locked, closing the count-then-insert race
- Interval floor enforced on create AND update, so create-then-edit can't bypass it
- Pending invitations count toward the member limit
- 402 Payment Required rather than 403: the action is legitimate, the plan is the obstacle
- GET /billing/plan (usage) and public GET /plans (pricing)
- Migration grandfathers existing tenants onto starter so no live data violates a new limit
- Billing tab: usage bars against plan limits (amber at 80%) and tier comparison
- Account control in header: GET /account returns tenant, user role, plan, member/monitor counts and the tenant's own status page URL; menu closes on outside click and Escape

## 2026-09-10 — Phase 3.6 step 3: Stripe billing
- stripe-setup command creates products and prices via the API and stores the price IDs, so the Stripe side is reproducible rather than clicked into a dashboard
- Checkout for first purchase, billing portal for every subsequent change: card data never touches Pulse
- Webhook handler verifies signatures and records every event id, making Stripe's retries idempotent by construction
- Fixed: handlers matched on customer id alone, so cancelling a duplicate subscription revoked a plan the tenant was still paying for. Now scoped to the tracked subscription — see ADR 0006
- cancel_at_period_end tracked separately from canceled; the UI says access continues until the period ends
- Debugging notes: CLI was authorized against a different sandbox than the API key (events fired where nothing listened); stripe-go v81 rejects newer account API versions unless IgnoreAPIVersionMismatch is set

## 2026-09-10 — Phase 3.6 step 4: export and deletion
- GET /account/export streams the tenant's monitors, results, incidents, members and channels as JSON; deliberately excludes password hashes, auth tokens, Stripe IDs and channel configs (a Slack webhook is a live credential)
- DELETE /account requires typing the organization name, cancels the Stripe subscription before deleting, and relies on ON DELETE CASCADE for atomic removal
- deletion_log records the request before anything is destroyed and the counts after, so there is evidence the data no longer exists
- Verified: 3 monitors and 966 results removed, no orphaned rows, other tenants untouched

## 2026-09-10 — Phase 3.6 step 4: export and deletion
- GET /account/export streams the tenant's monitors, results, incidents, members and channels as JSON; deliberately excludes password hashes, auth tokens, Stripe IDs and channel configs (a Slack webhook is a live credential)
- DELETE /account requires typing the organization name, cancels the Stripe subscription before deleting, and relies on ON DELETE CASCADE for atomic removal
- deletion_log records the request before anything is destroyed and the counts after, so there is evidence the data no longer exists
- Verified: 3 monitors and 966 results removed, no orphaned rows, other tenants untouched

## 2026-09-11 — Phase 3.6 step 5: retention and rollups
- daily_uptime table: one row per monitor per day with checks, successes, p50/p95/p99/max latency and downtime minutes from incident spans
- retention service rolls up completed days (idempotent, resumable via retention_state) and prunes raw results past each plan's window in 10k batches so no single transaction holds long locks
- Status page now reads the rollup in ONE query for all monitors, removing the earlier N+1; today is still computed live since it is incomplete
- Scale rationale: 149 raw rows occupy 328 kB (~2.2 kB/row with overhead and index); at 30s intervals a hundred monitors reach ~26M rows and ~57 GB over 90 days. The rollup replaces each monitor-day with one ~100-byte row.
- RUN_ONCE=true makes the service usable as a Kubernetes CronJob

## 2026-09-11 — Phase 3.7 step 1: content checks and TLS expiry
- Keyword assertions: body must (or must not) contain a string, case-insensitive, body capped at 1 MiB so a huge response can't exhaust a worker
- failure_kind on every result (connect/timeout/status/keyword/blocked) so an alert can explain itself
- Checks now use a fresh transport per request: reusing a pooled connection would skip DNS and the TLS handshake, flattering the measured latency and never re-reading the certificate
- TLS expiry recorded per monitor; warning email once per certificate (ssl_alerted_for), NOT an incident — the site is up, and opening one would corrupt uptime figures
- pulse_cert_days_remaining gauge for Prometheus alerting
- SSRF guard made an explicit option rather than unconditional, so tests can target 127.0.0.1 deliberately instead of the guard being weakened to allow localhost

## 2026-09-11 — Phase 3.7 step 2: maintenance windows and alert delay
- Maintenance windows suppress alerts for selected monitors (or the whole tenant when none are selected); incidents inside a window are recorded as planned rather than hidden, so headline uptime excludes them but the record survives for a stricter calculation
- Recovery is only announced if the failure was — a recovery notice for an outage nobody heard about is noise
- Per-monitor alert_delay_seconds: a swept timer queues the notification once the incident has been open long enough. A timer rather than an in-process sleep, because a sleep loses every pending alert when the pod restarts — exactly when the alert matters most
- Verified: same failure produced a planned, silent incident inside a window and an alerting one outside it

## 2026-09-11 — Phase 3.7 complete
- Per-monitor alert routing: PUT replaces the whole channel set (idempotent, no add/remove semantics needed); an empty set means all channels, because silence is the worst failure mode for a monitoring product
- Status page customization: custom title, subtitle and support link; per-monitor public visibility and display name so internal checks can stay private
- Support URL validated as http(s) — a javascript: URL there would be stored XSS against the customer's own visitors
- "Powered by Pulse" removal gated on a paid plan, enforced server-side rather than by hiding the toggle
- Planned incidents no longer appear as unexplained outages in the public incident list

## 2026-09-11 — Dashboard redesign
- Sidebar layout replaces the horizontal tab bar, which was already cramped at six sections; account control and theme switch pinned to the bottom
- Status strip answers "is anything wrong" before anything else is read, and changes character when the answer is yes
- Monitors are a dense table with inline latency sparklines rather than cards: a real customer has forty, and cards showed six
- Design tokens: cool blue-shifted neutrals so status colour is the only saturated colour in the interface, and an accent outside the status family so a button can never read as a state
- Day / Night / Auto, with separately tuned status colours per theme — the green that works on a dark panel fails contrast on white
- IBM Plex Sans throughout, Plex Mono only in numeric cells where digit alignment is functional
- Real labels on every input, skeleton loading states, and empty states that say what to do next
- Monitor settings moved to a side sheet: eleven settings would not fit an inline row, which is why keyword and routing were previously buried

## 2026-09-11 — Dashboard completion
- Monitor detail view: latency chart with p50/p95 band, bucketed server-side (30 days at 30s intervals is 86,400 points, which no chart should be handed), uptime for three windows, TLS expiry, and incident history
- Chart is clickable: hover to scan, click to pin a bucket and read the numbers
- Incidents screen across all monitors, ongoing sorted first; "nobody told" badge surfaces outages that fired with no alert channel configured
- Monitor search, with / to focus and n to add — ignored while typing so they never swallow a character
- Account screen: change password (current password required, so an unattended session cannot lock out the owner), export data, delete account
