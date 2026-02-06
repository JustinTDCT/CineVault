-- Phase 2: Expand media types, add edition groups, sister groups, collections, hierarchy tables

-- Expand media_type enum
ALTER TYPE media_type ADD VALUE IF NOT EXISTS 'adult_movies';
ALTER TYPE media_type ADD VALUE IF NOT EXISTS 'music';
ALTER TYPE media_type ADD VALUE IF NOT EXISTS 'music_videos';
ALTER TYPE media_type ADD VALUE IF NOT EXISTS 'home_videos';
ALTER TYPE media_type ADD VALUE IF NOT EXISTS 'other_videos';
ALTER TYPE media_type ADD VALUE IF NOT EXISTS 'images';
ALTER TYPE media_type ADD VALUE IF NOT EXISTS 'audiobooks';

-- Expand duplicate_action enum
ALTER TYPE duplicate_action ADD VALUE IF NOT EXISTS 'split_as_sister';
ALTER TYPE duplicate_action ADD VALUE IF NOT EXISTS 'edition_grouped';

-- Music hierarchy
CREATE TABLE artists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    sort_name VARCHAR(500),
    description TEXT,
    poster_path TEXT,
    metadata JSONB,
    external_ids JSONB,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE albums (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artist_id UUID NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    sort_title VARCHAR(500),
    year INTEGER,
    release_date DATE,
    description TEXT,
    genre VARCHAR(255),
    poster_path TEXT,
    metadata JSONB,
    external_ids JSONB,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audiobook hierarchy
CREATE TABLE authors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    sort_name VARCHAR(500),
    description TEXT,
    poster_path TEXT,
    metadata JSONB,
    external_ids JSONB,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE book_series (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    sort_title VARCHAR(500),
    description TEXT,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE books (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
    series_id UUID REFERENCES book_series(id) ON DELETE SET NULL,
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    sort_title VARCHAR(500),
    year INTEGER,
    description TEXT,
    narrator VARCHAR(500),
    poster_path TEXT,
    metadata JSONB,
    external_ids JSONB,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Image galleries
CREATE TABLE image_galleries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    description TEXT,
    poster_path TEXT,
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add hierarchy FKs to media_items
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS artist_id UUID REFERENCES artists(id) ON DELETE SET NULL;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS album_id UUID REFERENCES albums(id) ON DELETE SET NULL;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS track_number INTEGER;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS disc_number INTEGER;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS author_id UUID REFERENCES authors(id) ON DELETE SET NULL;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS book_id UUID REFERENCES books(id) ON DELETE SET NULL;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS chapter_number INTEGER;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS image_gallery_id UUID REFERENCES image_galleries(id) ON DELETE SET NULL;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS sister_group_id UUID;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS phash VARCHAR(64);
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS audio_fingerprint VARCHAR(255);

-- Edition groups
CREATE TABLE edition_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    media_type media_type NOT NULL,
    title VARCHAR(500) NOT NULL,
    sort_title VARCHAR(500),
    year INTEGER,
    description TEXT,
    poster_path TEXT,
    backdrop_path TEXT,
    metadata JSONB,
    external_ids JSONB,
    default_edition_id UUID,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE edition_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    edition_group_id UUID NOT NULL REFERENCES edition_groups(id) ON DELETE CASCADE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    edition_type VARCHAR(100) NOT NULL DEFAULT 'Normal',
    custom_edition_name VARCHAR(255),
    quality_tier VARCHAR(20),
    display_name VARCHAR(500),
    is_default BOOLEAN DEFAULT false,
    sort_order INTEGER DEFAULT 0,
    notes TEXT,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    added_by UUID REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE(edition_group_id, media_item_id)
);

-- Sister groups
CREATE TABLE sister_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(500) NOT NULL,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Add FK constraint for sister_group_id
ALTER TABLE media_items ADD CONSTRAINT fk_media_sister_group FOREIGN KEY (sister_group_id) REFERENCES sister_groups(id) ON DELETE SET NULL;

-- Collections
CREATE TABLE collections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    library_id UUID REFERENCES libraries(id) ON DELETE SET NULL,
    name VARCHAR(500) NOT NULL,
    description TEXT,
    poster_path TEXT,
    collection_type VARCHAR(20) NOT NULL DEFAULT 'manual',
    visibility VARCHAR(20) NOT NULL DEFAULT 'private',
    item_sort_mode VARCHAR(30) DEFAULT 'custom',
    sort_position INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE collection_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    media_item_id UUID REFERENCES media_items(id) ON DELETE CASCADE,
    edition_group_id UUID REFERENCES edition_groups(id) ON DELETE CASCADE,
    tv_show_id UUID REFERENCES tv_shows(id) ON DELETE CASCADE,
    album_id UUID REFERENCES albums(id) ON DELETE CASCADE,
    book_id UUID REFERENCES books(id) ON DELETE CASCADE,
    sort_position INTEGER DEFAULT 0,
    notes TEXT,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    added_by UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Watch history
CREATE TABLE watch_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    edition_group_id UUID REFERENCES edition_groups(id) ON DELETE SET NULL,
    progress_seconds INTEGER DEFAULT 0,
    duration_seconds INTEGER,
    completed BOOLEAN DEFAULT false,
    last_watched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, media_item_id)
);

-- Indexes
CREATE INDEX idx_artists_library ON artists(library_id);
CREATE INDEX idx_albums_artist ON albums(artist_id);
CREATE INDEX idx_albums_library ON albums(library_id);
CREATE INDEX idx_authors_library ON authors(library_id);
CREATE INDEX idx_book_series_author ON book_series(author_id);
CREATE INDEX idx_books_author ON books(author_id);
CREATE INDEX idx_books_library ON books(library_id);
CREATE INDEX idx_galleries_library ON image_galleries(library_id);
CREATE INDEX idx_media_items_artist ON media_items(artist_id);
CREATE INDEX idx_media_items_album ON media_items(album_id);
CREATE INDEX idx_media_items_author ON media_items(author_id);
CREATE INDEX idx_media_items_book ON media_items(book_id);
CREATE INDEX idx_media_items_gallery ON media_items(image_gallery_id);
CREATE INDEX idx_media_items_sister ON media_items(sister_group_id);
CREATE INDEX idx_media_items_phash ON media_items(phash);
CREATE INDEX idx_edition_groups_library ON edition_groups(library_id);
CREATE INDEX idx_edition_items_group ON edition_items(edition_group_id);
CREATE INDEX idx_edition_items_media ON edition_items(media_item_id);
CREATE INDEX idx_collections_user ON collections(user_id);
CREATE INDEX idx_collection_items_collection ON collection_items(collection_id);
CREATE INDEX idx_watch_history_user ON watch_history(user_id);
CREATE INDEX idx_watch_history_media ON watch_history(media_item_id);
CREATE INDEX idx_watch_history_last ON watch_history(last_watched_at);
