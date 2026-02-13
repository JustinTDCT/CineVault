-- Add columns for unified cache server metadata
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS metacritic_score INTEGER;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS content_ratings_json TEXT;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS taglines_json TEXT;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS trailers_json TEXT;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS descriptions_json TEXT;
