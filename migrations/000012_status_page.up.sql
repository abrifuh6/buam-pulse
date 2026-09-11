-- Status page presentation. A customer's public page should show their brand
-- and their language, not ours, and should not necessarily expose every
-- monitor they run — internal checks are legitimate to keep private.
ALTER TABLE tenants ADD COLUMN status_title       TEXT;
ALTER TABLE tenants ADD COLUMN status_description TEXT;
ALTER TABLE tenants ADD COLUMN status_support_url TEXT;
-- Hides the "Powered by Pulse" footer. Reserved for paid plans: it is the
-- standard lever for a product like this, and the enforcement lives in the
-- handler rather than here so the plan table stays the single source of truth.
ALTER TABLE tenants ADD COLUMN status_hide_branding BOOLEAN NOT NULL DEFAULT false;

-- Per-monitor visibility on the public page. A monitor can be watched
-- privately without being published.
ALTER TABLE monitors ADD COLUMN public BOOLEAN NOT NULL DEFAULT true;
-- An optional friendlier name for customers: "Checkout API" reads better on a
-- status page than "prod-checkout-svc-healthz".
ALTER TABLE monitors ADD COLUMN public_name TEXT;
