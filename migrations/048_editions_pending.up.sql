ALTER TABLE media_items ADD COLUMN IF NOT EXISTS editions_pending BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_media_items_editions_pending ON media_items (editions_pending) WHERE editions_pending = true;
