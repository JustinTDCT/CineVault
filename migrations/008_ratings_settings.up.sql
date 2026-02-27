-- Add rating columns to media_items
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS imdb_rating DECIMAL(3,1);
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS rt_rating INTEGER;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS audience_score INTEGER;

-- System settings key-value store
CREATE TABLE IF NOT EXISTS system_settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Seed OMDb API key for this instance
INSERT INTO system_settings (key, value) VALUES ('omdb_api_key', 'f98d128c')
ON CONFLICT (key) DO NOTHING;
