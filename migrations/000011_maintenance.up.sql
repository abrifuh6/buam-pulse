-- Scheduled maintenance. Alerts are suppressed during a window, and any
-- incident that opens inside one is marked planned rather than hidden: the
-- headline uptime number excludes it, but the record survives so the stricter
-- figure can still be computed later. Discarding the data would be irreversible.
CREATE TABLE maintenance_windows (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT,
    starts_at   TIMESTAMPTZ NOT NULL,
    ends_at     TIMESTAMPTZ NOT NULL,
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);

CREATE INDEX maintenance_active_idx
    ON maintenance_windows (tenant_id, starts_at, ends_at);

-- Which monitors a window covers. No rows means the whole tenant, which is the
-- common case (a deploy takes everything down) and saves the user from ticking
-- every box.
CREATE TABLE maintenance_monitors (
    window_id  UUID NOT NULL REFERENCES maintenance_windows(id) ON DELETE CASCADE,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    PRIMARY KEY (window_id, monitor_id)
);

-- Incidents that began during a window.
ALTER TABLE incidents ADD COLUMN planned BOOLEAN NOT NULL DEFAULT false;

-- Escalation delay: wait this long after a monitor goes down before notifying.
-- Distinct from the failure threshold — the threshold filters flapping, this
-- filters brevity. 0 means alert immediately, which stays the default.
ALTER TABLE monitors ADD COLUMN alert_delay_seconds INT NOT NULL DEFAULT 0
    CHECK (alert_delay_seconds >= 0 AND alert_delay_seconds <= 3600);

-- Set when an incident's delayed notification has been queued, so the sweeper
-- doesn't queue it twice.
ALTER TABLE incidents ADD COLUMN notified_at TIMESTAMPTZ;
