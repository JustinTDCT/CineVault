-- Music system overhaul: album_artist distinction, MBIDs at all levels, play tracking

-- Track-level artist name (separate from album_artist used for hierarchy)
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS album_artist TEXT;

-- MusicBrainz IDs at all levels
ALTER TABLE albums ADD COLUMN IF NOT EXISTS mbid TEXT;
ALTER TABLE albums ADD COLUMN IF NOT EXISTS release_group_id TEXT;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS recording_mbid TEXT;

-- Play count and last-played for music
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS play_count INTEGER DEFAULT 0;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS last_played_at TIMESTAMP;

-- Indexes for music queries
CREATE INDEX IF NOT EXISTS idx_media_items_album_id ON media_items(album_id) WHERE album_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_media_items_artist_id ON media_items(artist_id) WHERE artist_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_albums_artist_id ON albums(artist_id);
CREATE INDEX IF NOT EXISTS idx_albums_mbid ON albums(mbid) WHERE mbid IS NOT NULL;
