-- Phase: Enhanced recommendations, mood tags, smart collections

-- Add 'mood' to tag_category enum
ALTER TYPE tag_category ADD VALUE IF NOT EXISTS 'mood';

-- Add smart collection rules (JSONB) to collections table
ALTER TABLE collections ADD COLUMN IF NOT EXISTS rules JSONB;

-- Add keywords column to media_items for TMDB keywords (JSON array of strings)
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS keywords TEXT;

-- Index on watch_history for recommendation queries (recency + completion)
CREATE INDEX IF NOT EXISTS idx_watch_history_user_completed ON watch_history(user_id, completed, last_watched_at DESC);
