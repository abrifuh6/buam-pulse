ALTER TABLE tenants DROP COLUMN IF EXISTS trial_ends_at;
ALTER TABLE tenants DROP COLUMN IF EXISTS billing_period;
ALTER TABLE plans DROP COLUMN IF EXISTS stripe_price_yearly;
ALTER TABLE plans DROP COLUMN IF EXISTS stripe_price_quarterly;
ALTER TABLE plans DROP COLUMN IF EXISTS price_cents_yearly;
ALTER TABLE plans DROP COLUMN IF EXISTS price_cents_quarterly;
