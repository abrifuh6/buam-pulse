DROP TABLE IF EXISTS stripe_events;
ALTER TABLE plans DROP COLUMN IF EXISTS stripe_price_id;
ALTER TABLE tenants DROP COLUMN IF EXISTS current_period_end;
ALTER TABLE tenants DROP COLUMN IF EXISTS subscription_status;
ALTER TABLE tenants DROP COLUMN IF EXISTS stripe_subscription_id;
ALTER TABLE tenants DROP COLUMN IF EXISTS stripe_customer_id;
