-- Phase 3: Streaming, duplicate detection, performers, tags, studios, jobs

-- Performer types
CREATE TYPE performer_type AS ENUM ('actor', 'director', 'producer', 'musician', 'narrator', 'adult_performer', 'other');

-- Tag categories
CREATE TYPE tag_category AS ENUM ('genre', 'tag', 'custom');

-- Studio types
CREATE TYPE studio_type AS ENUM ('studio', 'label', 'publisher', 'network', 'distributor');

-- Job statuses
CREATE TYPE job_status AS ENUM ('pending', 'running', 'completed', 'failed', 'cancelled');

-- Playback preference modes
CREATE TYPE playback_mode AS ENUM ('always_ask', 'play_default', 'highest_quality', 'lowest_quality', 'last_played');

-- Transcode sessions
CREATE TABLE transcode_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    quality VARCHAR(20) NOT NULL DEFAULT '720p',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    output_dir TEXT NOT NULL,
    pid INTEGER,
    segments_ready INTEGER DEFAULT 0,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_access_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP
);

-- User playback preferences
CREATE TABLE user_playback_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    playback_mode playback_mode NOT NULL DEFAULT 'always_ask',
    preferred_quality VARCHAR(20) DEFAULT '1080p',
    auto_play_next BOOLEAN DEFAULT true,
    subtitle_language VARCHAR(10),
    audio_language VARCHAR(10),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id)
);

-- Per edition group user preference overrides
CREATE TABLE user_edition_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    edition_group_id UUID NOT NULL REFERENCES edition_groups(id) ON DELETE CASCADE,
    preferred_edition_id UUID REFERENCES edition_items(id) ON DELETE SET NULL,
    remember_choice BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, edition_group_id)
);

-- Performers / People
CREATE TABLE performers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(500) NOT NULL,
    sort_name VARCHAR(500),
    performer_type performer_type NOT NULL DEFAULT 'actor',
    photo_path TEXT,
    bio TEXT,
    birth_date DATE,
    death_date DATE,
    metadata JSONB,
    external_ids JSONB,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Media-Performer join table
CREATE TABLE media_performers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    performer_id UUID NOT NULL REFERENCES performers(id) ON DELETE CASCADE,
    role VARCHAR(100) DEFAULT 'cast',
    character_name VARCHAR(500),
    sort_order INTEGER DEFAULT 0,
    UNIQUE(media_item_id, performer_id, role)
);

-- Tags
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    parent_id UUID REFERENCES tags(id) ON DELETE SET NULL,
    category tag_category NOT NULL DEFAULT 'tag',
    description TEXT,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Media-Tag join table
CREATE TABLE media_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    UNIQUE(media_item_id, tag_id)
);

-- Studios / Labels / Publishers
CREATE TABLE studios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(500) NOT NULL,
    studio_type studio_type NOT NULL DEFAULT 'studio',
    logo_path TEXT,
    description TEXT,
    website VARCHAR(500),
    metadata JSONB,
    external_ids JSONB,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Media-Studio join table
CREATE TABLE media_studios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    studio_id UUID NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    role VARCHAR(100) DEFAULT 'production',
    UNIQUE(media_item_id, studio_id)
);

-- Job history
CREATE TABLE job_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type VARCHAR(100) NOT NULL,
    payload JSONB,
    status job_status NOT NULL DEFAULT 'pending',
    progress INTEGER DEFAULT 0,
    result JSONB,
    error_message TEXT,
    started_by UUID REFERENCES users(id) ON DELETE SET NULL,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add sister_group_id and edition_group_id to duplicate_decisions
ALTER TABLE duplicate_decisions ADD COLUMN IF NOT EXISTS sister_group_id UUID REFERENCES sister_groups(id) ON DELETE SET NULL;
ALTER TABLE duplicate_decisions ADD COLUMN IF NOT EXISTS edition_group_id UUID REFERENCES edition_groups(id) ON DELETE SET NULL;

-- Indexes
CREATE INDEX idx_transcode_sessions_media ON transcode_sessions(media_item_id);
CREATE INDEX idx_transcode_sessions_user ON transcode_sessions(user_id);
CREATE INDEX idx_transcode_sessions_status ON transcode_sessions(status);
CREATE INDEX idx_user_playback_prefs_user ON user_playback_preferences(user_id);
CREATE INDEX idx_user_edition_prefs_user ON user_edition_preferences(user_id);
CREATE INDEX idx_performers_name ON performers(name);
CREATE INDEX idx_performers_type ON performers(performer_type);
CREATE INDEX idx_media_performers_media ON media_performers(media_item_id);
CREATE INDEX idx_media_performers_performer ON media_performers(performer_id);
CREATE INDEX idx_tags_slug ON tags(slug);
CREATE INDEX idx_tags_parent ON tags(parent_id);
CREATE INDEX idx_tags_category ON tags(category);
CREATE INDEX idx_media_tags_media ON media_tags(media_item_id);
CREATE INDEX idx_media_tags_tag ON media_tags(tag_id);
CREATE INDEX idx_studios_name ON studios(name);
CREATE INDEX idx_studios_type ON studios(studio_type);
CREATE INDEX idx_media_studios_media ON media_studios(media_item_id);
CREATE INDEX idx_media_studios_studio ON media_studios(studio_id);
CREATE INDEX idx_job_history_type ON job_history(job_type);
CREATE INDEX idx_job_history_status ON job_history(status);
CREATE INDEX idx_job_history_started ON job_history(started_at);
