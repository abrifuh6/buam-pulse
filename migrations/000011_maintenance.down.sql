ALTER TABLE incidents DROP COLUMN IF EXISTS notified_at;
ALTER TABLE monitors DROP COLUMN IF EXISTS alert_delay_seconds;
ALTER TABLE incidents DROP COLUMN IF EXISTS planned;
DROP TABLE IF EXISTS maintenance_monitors;
DROP TABLE IF EXISTS maintenance_windows;
