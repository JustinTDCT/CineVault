-- Add content rating (MPAA: G, PG, PG-13, R, NC-17) to media items
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS content_rating VARCHAR(20);
CREATE INDEX IF NOT EXISTS idx_media_items_content_rating ON media_items(content_rating);
