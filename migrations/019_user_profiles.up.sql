-- 019: User profile enhancements — parental controls, kids mode, avatars
ALTER TABLE users ADD COLUMN IF NOT EXISTS max_content_rating VARCHAR(10) DEFAULT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_kids_profile BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_id VARCHAR(20) DEFAULT NULL;

COMMENT ON COLUMN users.max_content_rating IS 'Maximum allowed content rating (G, PG, PG-13, R, NC-17). NULL = unrestricted.';
COMMENT ON COLUMN users.is_kids_profile IS 'When true, enables simplified kids-mode UI and enforces content filtering.';
COMMENT ON COLUMN users.avatar_id IS 'Pre-built avatar identifier for profile display.';
