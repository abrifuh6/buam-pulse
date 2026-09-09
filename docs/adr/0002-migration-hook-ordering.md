# ADR 0002: Migrations run post-install, pre-upgrade

Status: Accepted · Date: 2026-09-09

## Context
The migrate Job was a `pre-install` Helm hook. In dev, Postgres is part of the
same chart, so the hook ran before the database existed and the install timed out.

## Decision
Hook on `post-install,pre-upgrade`. On first install the schema is applied right
after the release is created; on every upgrade it runs before new pods roll out.

## Consequences
- First install in dev has a few-second window where API pods are up before the
  schema exists. Acceptable for dev.
- Stage/prod use an external database (RDS) that outlives the chart, so they only
  ever take the pre-upgrade path: schema is always ahead of code.
