-- Add fast login PIN hash to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS pin_hash VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(255);

-- Seed default fast login settings
INSERT INTO system_settings (key, value, updated_at)
VALUES ('fast_login_enabled', 'true', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_settings (key, value, updated_at)
VALUES ('fast_login_pin_length', '4', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
