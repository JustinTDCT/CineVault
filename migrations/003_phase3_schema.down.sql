-- Phase 3 rollback
DROP TABLE IF EXISTS job_history;
DROP TABLE IF EXISTS media_studios;
DROP TABLE IF EXISTS studios;
DROP TABLE IF EXISTS media_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS media_performers;
DROP TABLE IF EXISTS performers;
DROP TABLE IF EXISTS user_edition_preferences;
DROP TABLE IF EXISTS user_playback_preferences;
DROP TABLE IF EXISTS transcode_sessions;

ALTER TABLE duplicate_decisions DROP COLUMN IF EXISTS sister_group_id;
ALTER TABLE duplicate_decisions DROP COLUMN IF EXISTS edition_group_id;

-- NOTE: Cannot remove enum types without recreating them in PostgreSQL
