DROP INDEX IF EXISTS idx_media_items_duplicate_status;
ALTER TABLE media_items DROP COLUMN IF EXISTS duplicate_status;
