-- Stripe identifiers on the tenant. Nullable: a tenant on the free plan has
-- never touched Stripe and shouldn't be forced to have a customer record.
ALTER TABLE tenants ADD COLUMN stripe_customer_id     TEXT UNIQUE;
ALTER TABLE tenants ADD COLUMN stripe_subscription_id TEXT UNIQUE;
-- Mirrors Stripe's subscription status so the app can act on it without an API
-- call on every request. Stripe remains the source of truth; this is a cache
-- kept current by webhooks.
ALTER TABLE tenants ADD COLUMN subscription_status TEXT;
ALTER TABLE tenants ADD COLUMN current_period_end  TIMESTAMPTZ;

-- Stripe price IDs live with the plan, so adding a tier is still just data.
ALTER TABLE plans ADD COLUMN stripe_price_id TEXT;

-- Every webhook Stripe delivers, recorded by its event ID. Stripe retries on
-- any non-2xx and can deliver the same event more than once; the PRIMARY KEY
-- makes a duplicate a no-op instead of a double-applied plan change.
CREATE TABLE stripe_events (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload      JSONB
);
