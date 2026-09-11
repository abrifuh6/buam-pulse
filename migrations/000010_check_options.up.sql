-- Content assertion: a 200 response with an error page rendered inside it is
-- still an outage, and status codes alone never catch that.
ALTER TABLE monitors ADD COLUMN keyword TEXT;
-- true  = body MUST contain the keyword
-- false = body MUST NOT contain it (catches "Database error", stack traces)
ALTER TABLE monitors ADD COLUMN keyword_present BOOLEAN NOT NULL DEFAULT true;

-- TLS certificate expiry. Expired certs are one of the most common
-- self-inflicted outages, and the failure mode is that nobody notices until
-- customers do.
ALTER TABLE monitors ADD COLUMN check_ssl BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE monitors ADD COLUMN ssl_warn_days INT NOT NULL DEFAULT 14;
ALTER TABLE monitors ADD COLUMN ssl_expires_at TIMESTAMPTZ;
ALTER TABLE monitors ADD COLUMN ssl_issuer TEXT;
-- Suppresses repeat warnings: we alert once per certificate, not once per check.
ALTER TABLE monitors ADD COLUMN ssl_alerted_for TIMESTAMPTZ;

-- Record what the check actually asserted, so a failure is explainable after
-- the fact rather than just "it failed".
ALTER TABLE check_results ADD COLUMN failure_kind TEXT;
