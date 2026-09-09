-- Pulse: initial schema
-- Every table that belongs to a customer carries tenant_id. This is the
-- foundation of multi-tenancy: one database, rows isolated by tenant.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,          -- used in public status page URL
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE monitors (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    type             TEXT NOT NULL CHECK (type IN ('http','tcp')),
    target           TEXT NOT NULL,             -- URL for http, host:port for tcp
    interval_seconds INT  NOT NULL DEFAULT 60 CHECK (interval_seconds BETWEEN 30 AND 3600),
    timeout_seconds  INT  NOT NULL DEFAULT 10,
    expected_status  INT  DEFAULT 200,          -- http only
    enabled          BOOLEAN NOT NULL DEFAULT true,
    status           TEXT NOT NULL DEFAULT 'unknown' CHECK (status IN ('up','down','unknown')),
    consecutive_fails INT NOT NULL DEFAULT 0,   -- alert after 2 in a row
    next_run_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The scheduler asks "which monitors are due?" every few seconds.
-- This partial index makes that query cheap no matter how many tenants exist.
CREATE INDEX monitors_due_idx ON monitors (next_run_at) WHERE enabled = true;
CREATE INDEX monitors_tenant_idx ON monitors (tenant_id);

CREATE TABLE check_results (
    id            BIGSERIAL PRIMARY KEY,
    monitor_id    UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    tenant_id     UUID NOT NULL,
    checked_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    ok            BOOLEAN NOT NULL,
    status_code   INT,
    latency_ms    INT,
    error         TEXT,
    region        TEXT NOT NULL DEFAULT 'local'
);

-- Time-series access pattern: "give me the last N results for this monitor".
CREATE INDEX check_results_monitor_time_idx ON check_results (monitor_id, checked_at DESC);

CREATE TABLE incidents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    monitor_id   UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at  TIMESTAMPTZ,
    cause        TEXT
);

CREATE INDEX incidents_open_idx ON incidents (monitor_id) WHERE resolved_at IS NULL;

CREATE TABLE alert_channels (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    type       TEXT NOT NULL CHECK (type IN ('email','slack')),
    config     JSONB NOT NULL,                 -- {"address": ...} or {"webhook_url": ...}
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
