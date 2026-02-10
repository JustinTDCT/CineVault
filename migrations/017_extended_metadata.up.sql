-- Migration 017: Extended Metadata Fields
-- Best-of-breed merge from Plex, Jellyfin, Emby, and Kodi

-- ── media_items: new metadata fields ──
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS tagline VARCHAR(1000);
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS original_language VARCHAR(10);
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS country VARCHAR(255);
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS trailer_url TEXT;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS logo_path TEXT;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS locked_fields TEXT[] NOT NULL DEFAULT '{}';

-- ── tv_shows: new metadata fields ──
ALTER TABLE tv_shows ADD COLUMN IF NOT EXISTS banner_path TEXT;
ALTER TABLE tv_shows ADD COLUMN IF NOT EXISTS tagline VARCHAR(1000);
ALTER TABLE tv_shows ADD COLUMN IF NOT EXISTS network VARCHAR(255);
ALTER TABLE tv_shows ADD COLUMN IF NOT EXISTS rating DECIMAL(3,1);
ALTER TABLE tv_shows ADD COLUMN IF NOT EXISTS content_rating VARCHAR(20);
ALTER TABLE tv_shows ADD COLUMN IF NOT EXISTS external_ids JSONB;

-- ── tv_seasons: external IDs for TVDB/TMDB season matching ──
ALTER TABLE tv_seasons ADD COLUMN IF NOT EXISTS external_ids JSONB;

-- ── libraries: NFO and local artwork settings ──
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS nfo_import BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS nfo_export BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS prefer_local_artwork BOOLEAN NOT NULL DEFAULT true;

-- ── movie_series: external IDs for TMDB auto-collections ──
ALTER TABLE movie_series ADD COLUMN IF NOT EXISTS external_ids JSONB;
ALTER TABLE movie_series ADD COLUMN IF NOT EXISTS backdrop_path TEXT;
ALTER TABLE movie_series ADD COLUMN IF NOT EXISTS description TEXT;

-- ── Backfill locked_fields from existing metadata_locked boolean ──
UPDATE media_items SET locked_fields = '{\"*\"}' WHERE metadata_locked = true;
