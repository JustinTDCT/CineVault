-- Phase 14: DLNA / Chromecast

-- ── DLNA service state ──
CREATE TABLE IF NOT EXISTS dlna_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    friendly_name TEXT NOT NULL DEFAULT 'CineVault',
    advertise_ip TEXT,
    port INT NOT NULL DEFAULT 1900,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default row
INSERT INTO dlna_config (id) VALUES (gen_random_uuid()) ON CONFLICT DO NOTHING;

-- ── Cast sessions for progress tracking ──
CREATE TABLE IF NOT EXISTS cast_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    device_name TEXT NOT NULL,
    device_type TEXT NOT NULL DEFAULT 'chromecast' CHECK (device_type IN ('chromecast', 'dlna', 'airplay')),
    state TEXT NOT NULL DEFAULT 'idle' CHECK (state IN ('idle', 'buffering', 'playing', 'paused', 'stopped')),
    current_time_sec FLOAT NOT NULL DEFAULT 0,
    duration_sec FLOAT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_cast_sessions_user ON cast_sessions(user_id);
