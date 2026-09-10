-- Whether the subscription is set to end at the period boundary. Distinct from
-- 'canceled': the tenant still has full access until the date passes, and the
-- UI must say so or they will assume the cancellation failed.
ALTER TABLE tenants ADD COLUMN cancel_at_period_end BOOLEAN NOT NULL DEFAULT false;
