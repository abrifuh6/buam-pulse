-- Plans as data, not constants in code: adding a tier or adjusting a limit is
-- an INSERT/UPDATE, and the enforcing query reads the current values.
CREATE TABLE plans (
    code             TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    max_monitors     INT  NOT NULL,
    min_interval     INT  NOT NULL,  -- seconds; smaller = more frequent = costlier
    max_members      INT  NOT NULL,
    max_channels     INT  NOT NULL,
    retention_days   INT  NOT NULL,
    price_cents      INT  NOT NULL,  -- monthly, USD
    sort_order       INT  NOT NULL
);

INSERT INTO plans (code, name, max_monitors, min_interval, max_members, max_channels, retention_days, price_cents, sort_order) VALUES
  ('free',    'Free',      3,  300,  1,  1,  30,      0, 1),
  ('starter', 'Starter',  25,   60,  5,  5,  90,   1900, 2),
  ('pro',     'Pro',     100,   30, 25, 25, 365,   4900, 3);

ALTER TABLE tenants ADD COLUMN plan_code TEXT NOT NULL DEFAULT 'free'
  REFERENCES plans(code);

-- Existing tenants predate billing; put them on starter so nothing they
-- already created suddenly violates a limit.
UPDATE tenants SET plan_code = 'starter';
