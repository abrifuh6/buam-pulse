-- Deletion requests are recorded before anything is destroyed. If the delete
-- fails partway, this row is the evidence of what was asked for and when —
-- and for a regulator, "we deleted it" needs a record that survives the data.
CREATE TABLE deletion_log (
    id            BIGSERIAL PRIMARY KEY,
    tenant_slug   TEXT NOT NULL,
    tenant_name   TEXT NOT NULL,
    requested_by  TEXT NOT NULL,   -- email, not a FK: the user is being deleted
    requested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ,
    monitors_removed INT,
    results_removed  BIGINT
);
