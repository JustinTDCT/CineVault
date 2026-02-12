-- Phase 13: Music Player

-- ── Lyrics storage ──
CREATE TABLE IF NOT EXISTS media_lyrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    source TEXT NOT NULL DEFAULT 'embedded' CHECK (source IN ('embedded', 'lrc', 'manual')),
    lyrics_type TEXT NOT NULL DEFAULT 'plain' CHECK (lyrics_type IN ('plain', 'synced')),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (media_item_id, source)
);
CREATE INDEX IF NOT EXISTS idx_lyrics_media ON media_lyrics(media_item_id);
