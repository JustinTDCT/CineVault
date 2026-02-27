-- Phase: Enhanced collections - nested collections support

-- Add parent_collection_id for nested/hierarchical collections
ALTER TABLE collections ADD COLUMN IF NOT EXISTS parent_collection_id UUID REFERENCES collections(id) ON DELETE SET NULL;

-- Index for efficient tree queries
CREATE INDEX IF NOT EXISTS idx_collections_parent ON collections(parent_collection_id);
