DROP TABLE IF EXISTS user_notification_preferences;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS password_reset_tokens;
ALTER TABLE notification_channels DROP COLUMN IF EXISTS config;
