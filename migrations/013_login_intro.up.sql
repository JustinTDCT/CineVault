-- Login intro settings (default: intro plays with audio)
INSERT INTO system_settings (key, value, updated_at)
VALUES ('login_intro_enabled', 'true', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_settings (key, value, updated_at)
VALUES ('login_intro_muted', 'false', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
