-- Migration 018: Technical metadata fields (source, HDR, dynamic range, custom notes/tags)
-- Persists data that was previously parsed but lost, and adds power-user annotation fields.

ALTER TABLE media_items ADD COLUMN IF NOT EXISTS source_type VARCHAR(50);
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS hdr_format VARCHAR(100);
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS dynamic_range VARCHAR(20) DEFAULT 'SDR';
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS custom_notes TEXT;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS custom_tags JSONB DEFAULT '{}';

-- Index for filtering by source and dynamic range
CREATE INDEX IF NOT EXISTS idx_media_source_type ON media_items (source_type) WHERE source_type IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_media_dynamic_range ON media_items (dynamic_range) WHERE dynamic_range != 'SDR';
