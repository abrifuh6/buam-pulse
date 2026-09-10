DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS monitor_channels;
ALTER TABLE incidents DROP COLUMN IF EXISTS notify_count;
ALTER TABLE incidents DROP COLUMN IF EXISTS last_notified_at;
ALTER TABLE alert_channels DROP COLUMN IF EXISTS verify_token;
ALTER TABLE alert_channels DROP COLUMN IF EXISTS verified_at;
ALTER TABLE alert_channels DROP COLUMN IF EXISTS enabled;
ALTER TABLE alert_channels DROP COLUMN IF EXISTS name;
