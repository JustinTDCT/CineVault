-- Revert phash column size (data is already cleared, cannot restore old values)
ALTER TABLE media_items ALTER COLUMN phash TYPE VARCHAR(64);
