-- media_segments stores detected or manually-set skip regions within a media item
CREATE TABLE IF NOT EXISTS media_segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    segment_type VARCHAR(20) NOT NULL CHECK (segment_type IN ('intro', 'credits', 'recap', 'preview')),
    start_seconds DOUBLE PRECISION NOT NULL,
    end_seconds DOUBLE PRECISION NOT NULL,
    confidence DOUBLE PRECISION DEFAULT 0.0,
    source VARCHAR(20) DEFAULT 'auto' CHECK (source IN ('auto', 'manual', 'community')),
    verified BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_item_id, segment_type)
);

CREATE INDEX IF NOT EXISTS idx_media_segments_media ON media_segments(media_item_id);
CREATE INDEX IF NOT EXISTS idx_media_segments_type ON media_segments(segment_type);

-- user_skip_preferences stores per-user auto-skip toggle settings
CREATE TABLE IF NOT EXISTS user_skip_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skip_intros BOOLEAN DEFAULT false,
    skip_credits BOOLEAN DEFAULT false,
    skip_recaps BOOLEAN DEFAULT false,
    show_skip_button BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id)
);
