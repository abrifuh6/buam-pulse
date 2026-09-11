-- Three billing periods per plan. Longer commitments get a discount, which is
-- the standard trade in this category: the customer pays less per month, we
-- get predictable revenue and less churn.
--
-- Prices are stored per period rather than computed from a monthly rate, so a
-- promotion or a rounded price ("$182" rather than "$182.40") is a data change
-- and never a code change.
ALTER TABLE plans ADD COLUMN price_cents_quarterly INT;
ALTER TABLE plans ADD COLUMN price_cents_yearly    INT;
ALTER TABLE plans ADD COLUMN stripe_price_quarterly TEXT;
ALTER TABLE plans ADD COLUMN stripe_price_yearly    TEXT;

-- Roughly 10% off quarterly and 20% off yearly, rounded to prices a human
-- would write.
UPDATE plans SET price_cents_quarterly = 5100,  price_cents_yearly = 18200 WHERE code = 'starter';
UPDATE plans SET price_cents_quarterly = 13200, price_cents_yearly = 47000 WHERE code = 'pro';

-- Which period a tenant is actually on, so the UI can say "renews yearly"
-- rather than guessing from the amount.
ALTER TABLE tenants ADD COLUMN billing_period TEXT
  CHECK (billing_period IN ('monthly','quarterly','yearly'));
-- Set while a trial is running. Distinct from an active subscription: the
-- tenant has full access but has not paid yet, and the UI should say so.
ALTER TABLE tenants ADD COLUMN trial_ends_at TIMESTAMPTZ;
