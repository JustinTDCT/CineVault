-- Rollback: Enhanced collections
DROP INDEX IF EXISTS idx_collections_parent;
ALTER TABLE collections DROP COLUMN IF EXISTS parent_collection_id;
