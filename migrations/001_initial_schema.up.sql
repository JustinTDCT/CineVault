CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest');
CREATE TYPE media_type AS ENUM ('movies', 'tv_shows');
CREATE TYPE duplicate_action AS ENUM ('merged', 'deleted', 'ignored');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role user_role NOT NULL DEFAULT 'user',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE libraries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    media_type media_type NOT NULL,
    path TEXT NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    scan_on_startup BOOLEAN DEFAULT false,
    last_scan_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE media_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    media_type media_type NOT NULL,
    file_path TEXT NOT NULL UNIQUE,
    file_name VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL,
    file_hash VARCHAR(64),
    title VARCHAR(500) NOT NULL,
    sort_title VARCHAR(500),
    original_title VARCHAR(500),
    description TEXT,
    year INTEGER,
    release_date DATE,
    duration_seconds INTEGER,
    rating DECIMAL(3,1),
    resolution VARCHAR(20),
    width INTEGER,
    height INTEGER,
    codec VARCHAR(50),
    container VARCHAR(20),
    bitrate BIGINT,
    framerate DECIMAL(6,3),
    audio_codec VARCHAR(50),
    audio_channels INTEGER,
    poster_path TEXT,
    thumbnail_path TEXT,
    backdrop_path TEXT,
    metadata JSONB,
    external_ids JSONB,
    tv_show_id UUID,
    tv_season_id UUID,
    episode_number INTEGER,
    sort_position INTEGER DEFAULT 0,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_scanned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tv_shows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    sort_title VARCHAR(500),
    original_title VARCHAR(500),
    description TEXT,
    year INTEGER,
    first_air_date DATE,
    last_air_date DATE,
    status VARCHAR(50),
    poster_path TEXT,
    backdrop_path TEXT,
    metadata JSONB,
    external_ids JSONB,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tv_seasons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tv_show_id UUID NOT NULL REFERENCES tv_shows(id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL,
    title VARCHAR(500),
    description TEXT,
    air_date DATE,
    episode_count INTEGER DEFAULT 0,
    poster_path TEXT,
    metadata JSONB,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tv_show_id, season_number)
);

CREATE TABLE duplicate_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id_a UUID REFERENCES media_items(id) ON DELETE SET NULL,
    media_id_b UUID REFERENCES media_items(id) ON DELETE SET NULL,
    action duplicate_action NOT NULL,
    primary_media_id UUID REFERENCES media_items(id) ON DELETE SET NULL,
    decided_by UUID REFERENCES users(id) ON DELETE SET NULL,
    decided_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    notes TEXT,
    similarity_score DECIMAL(5,4)
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_libraries_media_type ON libraries(media_type);
CREATE INDEX idx_media_items_library ON media_items(library_id);
CREATE INDEX idx_media_items_type ON media_items(media_type);
CREATE INDEX idx_media_items_title ON media_items(title);
CREATE INDEX idx_tv_shows_library ON tv_shows(library_id);
CREATE INDEX idx_tv_seasons_show ON tv_seasons(tv_show_id);
