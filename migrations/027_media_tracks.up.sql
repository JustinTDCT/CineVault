-- Phase 1: Core Playback — subtitle tracks, audio tracks, and chapter markers

-- ── Subtitle Tracks ──
CREATE TABLE IF NOT EXISTS media_subtitles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    language TEXT,
    title TEXT,
    format TEXT NOT NULL,
    file_path TEXT,
    stream_index INT,
    source TEXT NOT NULL DEFAULT 'external',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_forced BOOLEAN NOT NULL DEFAULT FALSE,
    is_sdh BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_subtitles_media ON media_subtitles(media_item_id);

-- ── Audio Tracks ──
CREATE TABLE IF NOT EXISTS media_audio_tracks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    stream_index INT NOT NULL,
    language TEXT,
    title TEXT,
    codec TEXT NOT NULL,
    channels INT NOT NULL DEFAULT 2,
    channel_layout TEXT,
    bitrate INT,
    sample_rate INT,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_commentary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_audio_tracks_media ON media_audio_tracks(media_item_id);

-- ── Chapter Markers ──
CREATE TABLE IF NOT EXISTS media_chapters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    title TEXT,
    start_seconds DECIMAL(10,3) NOT NULL,
    end_seconds DECIMAL(10,3),
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_chapters_media ON media_chapters(media_item_id);
