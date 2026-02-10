-- Rollback: Enhanced recommendations, mood tags, smart collections

DROP INDEX IF EXISTS idx_watch_history_user_completed;
ALTER TABLE media_items DROP COLUMN IF EXISTS keywords;
ALTER TABLE collections DROP COLUMN IF EXISTS rules;

-- Note: Cannot remove enum value from tag_category in PostgreSQL < 14
-- The 'mood' value will remain but is harmless
