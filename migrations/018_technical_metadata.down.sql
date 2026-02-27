DROP INDEX IF EXISTS idx_media_dynamic_range;
DROP INDEX IF EXISTS idx_media_source_type;
ALTER TABLE media_items DROP COLUMN IF EXISTS custom_tags;
ALTER TABLE media_items DROP COLUMN IF EXISTS custom_notes;
ALTER TABLE media_items DROP COLUMN IF EXISTS dynamic_range;
ALTER TABLE media_items DROP COLUMN IF EXISTS hdr_format;
ALTER TABLE media_items DROP COLUMN IF EXISTS source_type;
