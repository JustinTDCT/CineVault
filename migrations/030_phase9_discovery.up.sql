-- Phase 9: Discovery & Engagement Enhancements

-- ── Home page customization layout ──
CREATE TABLE IF NOT EXISTS user_home_layout (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    row_type TEXT NOT NULL, -- 'continue', 'on_deck', 'watchlist', 'favorites', 'trending', 'recent', 'genre', 'custom'
    row_id TEXT,            -- genre slug, library id, etc. depending on row_type
    sort_position INT NOT NULL DEFAULT 0,
    is_visible BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, row_type, row_id)
);
CREATE INDEX IF NOT EXISTS idx_home_layout_user ON user_home_layout(user_id, sort_position);

-- ── Content requests ──
CREATE TABLE IF NOT EXISTS content_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tmdb_id INT,
    title TEXT NOT NULL,
    year INT,
    media_type TEXT NOT NULL DEFAULT 'movie',
    poster_url TEXT,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'denied', 'fulfilled')),
    admin_note TEXT,
    resolved_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_content_requests_user ON content_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_content_requests_status ON content_requests(status);

-- ── Video preview clips ──
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS preview_path TEXT;
