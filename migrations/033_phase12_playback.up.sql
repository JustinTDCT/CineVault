-- Phase 12: Advanced Playback

-- ── Watch Together / SyncPlay sessions ──
CREATE TABLE IF NOT EXISTS sync_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    host_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invite_code TEXT NOT NULL UNIQUE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    state TEXT NOT NULL DEFAULT 'waiting' CHECK (state IN ('waiting', 'playing', 'paused', 'ended')),
    current_time_sec FLOAT NOT NULL DEFAULT 0,
    max_participants INT NOT NULL DEFAULT 10,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sync_sessions_code ON sync_sessions(invite_code);

CREATE TABLE IF NOT EXISTS sync_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sync_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (session_id, user_id)
);

CREATE TABLE IF NOT EXISTS sync_chat (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sync_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sync_chat_session ON sync_chat(session_id, created_at);

-- ── Cinema mode pre-rolls ──
CREATE TABLE IF NOT EXISTS pre_roll_videos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    duration_seconds INT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Cinema mode settings ──
-- (stored in settings table as JSON: cinema_trailer_count, cinema_randomize)
