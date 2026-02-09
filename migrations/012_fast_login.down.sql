ALTER TABLE users DROP COLUMN IF EXISTS pin_hash;
ALTER TABLE users DROP COLUMN IF EXISTS display_name;

DELETE FROM system_settings WHERE key IN ('fast_login_enabled', 'fast_login_pin_length');
