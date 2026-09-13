# Pulse

Uptime monitoring, alerting and status pages for small teams. Built as a
complete SaaS product — not a demo — to exercise the full lifecycle from
writing the application to running it on AWS.

Pulse checks your sites and APIs from outside your network every 30 seconds,
opens an incident when something fails twice in a row, alerts you by email or
Slack within a minute, and publishes a status page your customers can read.
 Marketing site            Dashboard              Status page
  (public)                 (tenant)                (public)
      │                       │                       │
      └───────────────┬───────┴───────────────────────┘
                      │
                ┌─────▼─────┐
                │    API    │  Go · chi · JWT · tenant-scoped
                └─────┬─────┘
                      │
   ┌──────────────────┼──────────────────┬──────────────┐
   │                  │                  │              │
## What it does

**Monitoring.** HTTP and TCP checks on a schedule you choose, from 30 seconds
to an hour. Content assertions catch the case status codes miss — a page can
return 200 while rendering an error, and checking for expected text is the only
way to see it. Every check opens a fresh connection, so the latency measured is
what a first-time visitor experiences rather than a warm connection pool.

**Alerting.** Two consecutive failures before an incident opens, so a single
blip never pages anyone. An optional per-monitor delay filters short outages.
Maintenance windows suppress alerts during planned work and mark the downtime
as planned so it does not count against published uptime. Delivery retries with
exponential backoff, and recovery is announced too.

**Status pages.** A public page per tenant with 90 days of uptime history, live
incidents, per-service detail and your own branding. Choose which monitors
appear — internal checks stay private.

**TLS certificates.** Every HTTPS check reads the certificate. A warning arrives
two weeks before expiry, which is two weeks more notice than the browser error
your customers would otherwise find first.

**Teams and billing.** Roles enforced server-side, invitations with single-use
tokens, plan limits, Stripe subscriptions with monthly, quarterly and annual
billing, 14-day trials, and data export and account deletion for GDPR and
PIPEDA.

## Architecture

Five Go services sharing one PostgreSQL database, communicating through a Redis
queue. This is a **modular monolith split by workload**, not microservices —
the distinction and its reasoning are in
[ADR 0007](docs/adr/0007-service-boundaries.md).

| Service | Why it is separate |
|---|---|
| `api` | Request/response, scales with user traffic |
| `scheduler` | Must run as exactly one replica; cannot live inside a service you want several copies of |
| `worker` | Checks are I/O-bound and bursty; scales with customer count independently of API traffic |
| `notifier` | A slow email provider must never delay a monitoring check |
| `retention` | Heavy periodic rollups that should not compete with request handling |

The frontends are three separate Vite apps — marketing site, dashboard, and
status pages — deployed independently because they change on different cadences
and the public ones need no API dependency at all.

## Running it locally

One command brings up Postgres, Redis, Mailpit and all five Go services with
hot reload:

```bash
cp .env.example .env
make dev-up
make migrate-up
```

Then the three frontends:

```bash
cd apps/web && npm run dev      # dashboard        http://localhost:5173
cd apps/status && npm run dev   # status pages     http://localhost:5174
cd apps/site && npm run dev     # marketing site   http://localhost:5175
```

| | |
|---|---|
| API | http://localhost:8080 |
| Mail (Mailpit) | http://localhost:8025 |
| `make dev-logs` | tail everything, `S=worker` for one |
| `make dev-down` | stop |

Billing needs a Stripe sandbox key in `.env` and `make stripe-setup` to create
the products and prices. Webhooks need `stripe listen --forward-to
localhost:8080/api/v1/billing/webhook`.

## Infrastructure

Terraform builds a single EKS cluster in `ca-central-1` with namespace
separation per environment, RDS PostgreSQL, ElastiCache Valkey, ECR, and a
three-tier VPC where the data subnets have no internet route in either
direction.

```bash
cd infra/terraform/environments/prod
terraform init && terraform apply
```

Checkov runs against it in CI. All 105 passing checks are genuine; the 24
skipped ones each carry an inline comment explaining the trade-off, because an
undocumented suppression looks like diligence while hiding a decision nobody
argued for.

## Pipeline

Every push runs static checks, race-enabled tests, `helm lint`, frontend type
checks, and Terraform validation. Then a parallel matrix builds nine images,
scans each with Trivy, and — only if the scan finds no fixable critical or high
vulnerability — publishes multi-architecture images to GHCR and Docker Hub
tagged by commit SHA.

## Decisions worth reading

| | |
|---|---|
| [0001](docs/adr/0001-monorepo-and-service-split.md) | Monorepo and the service split |
| [0002](docs/adr/0002-migration-hook-ordering.md) | Migrations run post-install, pre-upgrade |
| [0003](docs/adr/0003-same-origin-frontend.md) | Same-origin frontend, separate deployment |
| [0004](docs/adr/0004-outbound-request-safety.md) | SSRF and abuse: a monitoring service fetches arbitrary URLs |
| [0005](docs/adr/0005-rate-limiting.md) | In-memory rate limiting and its known limits |
| [0006](docs/adr/0006-stripe-subscription-identity.md) | One tracked subscription per tenant |
| [0007](docs/adr/0007-service-boundaries.md) | Split by workload, not by domain |

Also: [architecture](docs/ARCHITECTURE.md), [operations](docs/OPERATIONS.md),
[roadmap](docs/ROADMAP.md), [changelog](docs/CHANGELOG.md).

## Built by

[Buam Technologies Inc.](https://github.com/abrifuh6) — Abri Fuh
