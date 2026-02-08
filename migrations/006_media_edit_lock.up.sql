-- Add metadata_locked flag to media_items to prevent automated metadata overwrites on user-edited items
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS metadata_locked BOOLEAN NOT NULL DEFAULT false;
