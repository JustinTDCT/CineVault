-- Phase 17: TV show performance indexes
-- Fixes full table scans on media_items when querying by tv_season_id/tv_show_id

CREATE INDEX IF NOT EXISTS idx_media_items_tv_season ON media_items(tv_season_id) WHERE tv_season_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_media_items_tv_show ON media_items(tv_show_id) WHERE tv_show_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_media_items_season_episode ON media_items(tv_season_id, episode_number) WHERE tv_season_id IS NOT NULL AND episode_number IS NOT NULL;
