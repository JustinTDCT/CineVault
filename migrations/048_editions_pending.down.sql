DROP INDEX IF EXISTS idx_media_items_editions_pending;
ALTER TABLE media_items DROP COLUMN IF EXISTS editions_pending;
