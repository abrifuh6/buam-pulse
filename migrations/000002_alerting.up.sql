-- Alert channels already exist as a table; add what makes them usable.
ALTER TABLE alert_channels ADD COLUMN name TEXT NOT NULL DEFAULT 'default';
ALTER TABLE alert_channels ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT true;
-- Email channels must be confirmed before we send to them: otherwise Pulse can
-- be used to mail arbitrary addresses on someone else's behalf.
ALTER TABLE alert_channels ADD COLUMN verified_at TIMESTAMPTZ;
ALTER TABLE alert_channels ADD COLUMN verify_token TEXT;

-- Which channels a monitor alerts on. Empty = all of the tenant's channels.
CREATE TABLE monitor_channels (
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES alert_channels(id) ON DELETE CASCADE,
    PRIMARY KEY (monitor_id, channel_id)
);

-- Incidents: add the fields a status page and a postmortem need.
ALTER TABLE incidents ADD COLUMN last_notified_at TIMESTAMPTZ;
ALTER TABLE incidents ADD COLUMN notify_count INT NOT NULL DEFAULT 0;

-- Every notification attempt is a row. Retries, failures and duplicates all
-- become visible instead of vanishing into a log line.
CREATE TABLE notifications (
    id           BIGSERIAL PRIMARY KEY,
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    incident_id  UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    channel_id   UUID NOT NULL REFERENCES alert_channels(id) ON DELETE CASCADE,
    kind         TEXT NOT NULL CHECK (kind IN ('down','recovered')),
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','sent','failed','dead')),
    attempts     INT  NOT NULL DEFAULT 0,
    last_error   TEXT,
    next_try_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at      TIMESTAMPTZ,
    -- One notification per incident per channel per kind. This is the
    -- idempotency guarantee: no duplicate pages, enforced by the database
    -- rather than by application logic that can race.
    UNIQUE (incident_id, channel_id, kind)
);

CREATE INDEX notifications_due_idx ON notifications (next_try_at)
    WHERE status IN ('pending','failed');
