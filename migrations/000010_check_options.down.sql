ALTER TABLE check_results DROP COLUMN IF EXISTS failure_kind;
ALTER TABLE monitors DROP COLUMN IF EXISTS ssl_alerted_for;
ALTER TABLE monitors DROP COLUMN IF EXISTS ssl_issuer;
ALTER TABLE monitors DROP COLUMN IF EXISTS ssl_expires_at;
ALTER TABLE monitors DROP COLUMN IF EXISTS ssl_warn_days;
ALTER TABLE monitors DROP COLUMN IF EXISTS check_ssl;
ALTER TABLE monitors DROP COLUMN IF EXISTS keyword_present;
ALTER TABLE monitors DROP COLUMN IF EXISTS keyword;
