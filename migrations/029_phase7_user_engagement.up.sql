-- Phase 7: User Engagement — watchlist, ratings, favorites, playlists, saved filters

-- ── Watchlist ──
CREATE TABLE IF NOT EXISTS user_watchlist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_item_id UUID REFERENCES media_items(id) ON DELETE CASCADE,
    tv_show_id UUID REFERENCES tv_shows(id) ON DELETE CASCADE,
    edition_group_id UUID REFERENCES edition_groups(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, media_item_id),
    CHECK (media_item_id IS NOT NULL OR tv_show_id IS NOT NULL OR edition_group_id IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_watchlist_user ON user_watchlist(user_id);
CREATE INDEX IF NOT EXISTS idx_watchlist_added ON user_watchlist(user_id, added_at DESC);

-- ── User Ratings ──
CREATE TABLE IF NOT EXISTS user_ratings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    rating DECIMAL(3,1) NOT NULL CHECK (rating >= 0 AND rating <= 10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, media_item_id)
);
CREATE INDEX IF NOT EXISTS idx_ratings_user ON user_ratings(user_id);
CREATE INDEX IF NOT EXISTS idx_ratings_media ON user_ratings(media_item_id);

-- ── Favorites ──
CREATE TABLE IF NOT EXISTS user_favorites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_item_id UUID REFERENCES media_items(id) ON DELETE CASCADE,
    tv_show_id UUID REFERENCES tv_shows(id) ON DELETE CASCADE,
    performer_id UUID REFERENCES performers(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (media_item_id IS NOT NULL OR tv_show_id IS NOT NULL OR performer_id IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_favorites_user ON user_favorites(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_favorites_user_media ON user_favorites(user_id, media_item_id) WHERE media_item_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_favorites_user_show ON user_favorites(user_id, tv_show_id) WHERE tv_show_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_favorites_user_performer ON user_favorites(user_id, performer_id) WHERE performer_id IS NOT NULL;

-- ── Playlists ──
CREATE TABLE IF NOT EXISTS playlists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    poster_path TEXT,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    shuffle_mode BOOLEAN NOT NULL DEFAULT FALSE,
    repeat_mode TEXT NOT NULL DEFAULT 'off' CHECK (repeat_mode IN ('off', 'all', 'one')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_playlists_user ON playlists(user_id);

CREATE TABLE IF NOT EXISTS playlist_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_playlist_items_playlist ON playlist_items(playlist_id, sort_order);

-- ── Saved Filter Presets ──
CREATE TABLE IF NOT EXISTS user_saved_filters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    library_id UUID REFERENCES libraries(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    filters JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_saved_filters_user ON user_saved_filters(user_id);
