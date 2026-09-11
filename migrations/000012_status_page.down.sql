ALTER TABLE monitors DROP COLUMN IF EXISTS public_name;
ALTER TABLE monitors DROP COLUMN IF EXISTS public;
ALTER TABLE tenants DROP COLUMN IF EXISTS status_hide_branding;
ALTER TABLE tenants DROP COLUMN IF EXISTS status_support_url;
ALTER TABLE tenants DROP COLUMN IF EXISTS status_description;
ALTER TABLE tenants DROP COLUMN IF EXISTS status_title;
