-- Daily rollup: one row per monitor per day, replacing thousands of raw rows
-- for anything older than today. This is what the status page reads, so its
-- cost stays flat as history grows.
CREATE TABLE daily_uptime (
    monitor_id   UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    day          DATE NOT NULL,
    checks_total INT  NOT NULL,
    checks_ok    INT  NOT NULL,
    -- Percentiles, not just the mean: an average hides the slow tail, and the
    -- tail is what users actually feel.
    latency_p50  INT,
    latency_p95  INT,
    latency_p99  INT,
    latency_max  INT,
    downtime_minutes INT NOT NULL DEFAULT 0,
    PRIMARY KEY (monitor_id, day)
);

CREATE INDEX daily_uptime_tenant_day_idx ON daily_uptime (tenant_id, day DESC);

-- Tracks how far the rollup has progressed so a restart resumes rather than
-- recomputing everything.
CREATE TABLE retention_state (
    id                INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    last_rollup_day   DATE,
    last_run_at       TIMESTAMPTZ,
    rows_deleted      BIGINT NOT NULL DEFAULT 0
);
INSERT INTO retention_state (id) VALUES (1) ON CONFLICT DO NOTHING;

-- check_results is queried almost exclusively by time for deletion, and by
-- (monitor, time) for display. This index serves the deletion path.
CREATE INDEX IF NOT EXISTS check_results_checked_at_idx ON check_results (checked_at);
