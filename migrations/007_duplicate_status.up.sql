-- Add duplicate_status to media_items for tracking duplicate detection and review
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS duplicate_status VARCHAR(20) NOT NULL DEFAULT 'none';
-- Values: 'none', 'exact', 'potential', 'addressed'

CREATE INDEX IF NOT EXISTS idx_media_items_duplicate_status ON media_items(duplicate_status) WHERE duplicate_status != 'none';
