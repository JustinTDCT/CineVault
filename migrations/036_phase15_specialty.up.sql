-- Phase 15: Specialty Content

-- ── Scene markers for adult content (P15-02) ──
CREATE TABLE IF NOT EXISTS scene_markers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    tag_id UUID REFERENCES tags(id) ON DELETE SET NULL,
    start_seconds FLOAT NOT NULL,
    end_seconds FLOAT,
    preview_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_scene_markers_media ON scene_markers(media_item_id, start_seconds);

-- ── Extended performer fields (P15-03) ──
ALTER TABLE performers ADD COLUMN IF NOT EXISTS gender TEXT;
ALTER TABLE performers ADD COLUMN IF NOT EXISTS birth_place TEXT;
ALTER TABLE performers ADD COLUMN IF NOT EXISTS height_cm INT;
ALTER TABLE performers ADD COLUMN IF NOT EXISTS weight_kg INT;
ALTER TABLE performers ADD COLUMN IF NOT EXISTS ethnicity TEXT;
ALTER TABLE performers ADD COLUMN IF NOT EXISTS hair_color TEXT;
ALTER TABLE performers ADD COLUMN IF NOT EXISTS eye_color TEXT;
ALTER TABLE performers ADD COLUMN IF NOT EXISTS measurements TEXT;
ALTER TABLE performers ADD COLUMN IF NOT EXISTS aliases TEXT[];
ALTER TABLE performers ADD COLUMN IF NOT EXISTS urls JSONB;

-- ── Per-user streaming limits (P15-04) ──
ALTER TABLE users ADD COLUMN IF NOT EXISTS max_simultaneous_streams INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS max_bitrate_kbps INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS remote_quality_cap TEXT;

-- ── Live TV / DVR (P15-05) ──
CREATE TABLE IF NOT EXISTS tuner_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    device_type TEXT NOT NULL DEFAULT 'hdhomerun',
    url TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    channel_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS epg_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tuner_id UUID REFERENCES tuner_devices(id) ON DELETE CASCADE,
    channel_number TEXT NOT NULL,
    name TEXT NOT NULL,
    icon_url TEXT,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_epg_channels_tuner ON epg_channels(tuner_id);

CREATE TABLE IF NOT EXISTS epg_programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID NOT NULL REFERENCES epg_channels(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    category TEXT,
    episode_info TEXT,
    icon_url TEXT
);
CREATE INDEX IF NOT EXISTS idx_epg_programs_channel_time ON epg_programs(channel_id, start_time);

CREATE TABLE IF NOT EXISTS dvr_recordings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID NOT NULL REFERENCES epg_channels(id) ON DELETE CASCADE,
    program_id UUID REFERENCES epg_programs(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    file_path TEXT,
    state TEXT NOT NULL DEFAULT 'scheduled' CHECK (state IN ('scheduled', 'recording', 'completed', 'failed')),
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Comics / eBooks library type (P15-06) ──
-- Reuse existing media_items table with new type values
-- Add reading progress tracking
CREATE TABLE IF NOT EXISTS reading_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    current_page INT NOT NULL DEFAULT 0,
    total_pages INT NOT NULL DEFAULT 0,
    current_chapter TEXT,
    font_size INT NOT NULL DEFAULT 16,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, media_item_id)
);
