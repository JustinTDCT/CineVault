-- User display preferences (overlay badges, future: theme, grid density, poster size)
CREATE TABLE IF NOT EXISTS user_display_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    overlay_settings JSONB NOT NULL DEFAULT '{
        "resolution_hdr": true,
        "audio_codec": true,
        "ratings": true,
        "content_rating": false,
        "edition_type": true,
        "source_type": false
    }'::jsonb,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id)
);
