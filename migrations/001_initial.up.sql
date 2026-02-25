-- CineVault v2.0.0 Initial Schema

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_type VARCHAR(20) NOT NULL DEFAULT 'owner'
        CHECK (account_type IN ('owner', 'shared', 'sub')),
    parent_id UUID REFERENCES users(id) ON DELETE CASCADE,
    full_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    pin VARCHAR(20),
    is_child BOOLEAN DEFAULT false,
    child_restrictions JSONB DEFAULT '{}',
    avatar_path VARCHAR(500),
    is_admin BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- User Profiles
CREATE TABLE user_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE UNIQUE,
    default_video_quality VARCHAR(50) DEFAULT 'original',
    auto_play_music BOOLEAN DEFAULT false,
    auto_play_videos BOOLEAN DEFAULT false,
    auto_play_music_videos BOOLEAN DEFAULT false,
    auto_play_audiobooks BOOLEAN DEFAULT false,
    overlay_settings JSONB DEFAULT '{}',
    library_order TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Libraries
CREATE TABLE libraries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    library_type VARCHAR(30) NOT NULL,
    folders TEXT[] NOT NULL DEFAULT '{}',
    include_in_homepage BOOLEAN DEFAULT true,
    include_in_search BOOLEAN DEFAULT true,
    retrieve_metadata BOOLEAN DEFAULT true,
    import_nfo BOOLEAN DEFAULT false,
    export_nfo BOOLEAN DEFAULT false,
    normalize_audio BOOLEAN DEFAULT false,
    timeline_scrubbing BOOLEAN DEFAULT false,
    preview_videos BOOLEAN DEFAULT false,
    intro_detection BOOLEAN DEFAULT false,
    credits_detection BOOLEAN DEFAULT false,
    recap_detection BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Library Permissions
CREATE TABLE library_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_level VARCHAR(10) NOT NULL DEFAULT 'none'
        CHECK (permission_level IN ('none', 'view', 'edit')),
    UNIQUE(library_id, user_id)
);

-- Media Items
CREATE TABLE media_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    cache_id VARCHAR(100),
    parent_id UUID REFERENCES media_items(id) ON DELETE SET NULL,
    title VARCHAR(500),
    original_title VARCHAR(500),
    sort_title VARCHAR(500),
    description TEXT,
    release_date VARCHAR(20),
    release_year INTEGER,
    runtime_minutes INTEGER,
    file_path TEXT NOT NULL,
    file_size BIGINT,
    file_hash VARCHAR(64),
    file_mod_time TIMESTAMPTZ,
    video_codec VARCHAR(50),
    audio_codec VARCHAR(50),
    resolution VARCHAR(20),
    bitrate INTEGER,
    phash VARCHAR(64),
    match_confidence FLOAT DEFAULT 0,
    metadata_locked BOOLEAN DEFAULT false,
    manual_override_fields TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    season_number INTEGER,
    episode_number INTEGER,
    date_added TIMESTAMPTZ DEFAULT NOW(),
    date_modified TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_media_library_sort ON media_items(library_id, sort_title);
CREATE INDEX idx_media_library_year ON media_items(library_id, release_year);
CREATE INDEX idx_media_library_added ON media_items(library_id, date_added);
CREATE INDEX idx_media_cache_id ON media_items(cache_id) WHERE cache_id IS NOT NULL;
CREATE INDEX idx_media_parent ON media_items(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_media_file_path ON media_items(file_path);
CREATE INDEX idx_media_phash ON media_items(phash) WHERE phash IS NOT NULL;
CREATE INDEX idx_media_metadata ON media_items USING GIN(metadata);

-- Full-text search
ALTER TABLE media_items ADD COLUMN search_vector tsvector;
CREATE INDEX idx_media_search ON media_items USING GIN(search_vector);

CREATE OR REPLACE FUNCTION media_items_search_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english',
        coalesce(NEW.title, '') || ' ' ||
        coalesce(NEW.original_title, '') || ' ' ||
        coalesce(NEW.description, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER media_items_search_trigger
    BEFORE INSERT OR UPDATE ON media_items
    FOR EACH ROW EXECUTE FUNCTION media_items_search_update();

-- Media Segments
CREATE TABLE media_segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    segment_type VARCHAR(20) NOT NULL
        CHECK (segment_type IN ('intro', 'credits', 'recap')),
    start_time FLOAT NOT NULL,
    end_time FLOAT NOT NULL,
    confidence FLOAT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_segments_item ON media_segments(media_item_id);

-- Collections
CREATE TABLE collections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    poster_path VARCHAR(500),
    is_smart BOOLEAN DEFAULT false,
    smart_filters JSONB DEFAULT '{}',
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Collection Items
CREATE TABLE collection_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    sort_order INTEGER DEFAULT 0,
    UNIQUE(collection_id, media_item_id)
);

-- Watch History
CREATE TABLE watch_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    position_seconds FLOAT DEFAULT 0,
    duration_seconds FLOAT,
    completed BOOLEAN DEFAULT false,
    last_watched TIMESTAMPTZ DEFAULT NOW(),
    play_count INTEGER DEFAULT 1
);

CREATE INDEX idx_watch_user ON watch_history(user_id, last_watched DESC);
CREATE INDEX idx_watch_item ON watch_history(media_item_id);
CREATE UNIQUE INDEX idx_watch_user_item ON watch_history(user_id, media_item_id);

-- Settings
CREATE TABLE settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Sessions
CREATE TABLE sessions (
    token VARCHAR(64) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_admin BOOLEAN DEFAULT false,
    expires_at BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- Scan State
CREATE TABLE scan_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE UNIQUE,
    last_scan_started TIMESTAMPTZ,
    last_scan_completed TIMESTAMPTZ,
    files_scanned INTEGER DEFAULT 0,
    files_added INTEGER DEFAULT 0,
    files_removed INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'idle'
);
