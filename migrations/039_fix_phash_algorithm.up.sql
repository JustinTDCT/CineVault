-- Clear all existing phash values (they were MD5-based and useless for similarity comparison).
-- Widen the column to hold actual perceptual hash hex strings (112 chars for 7-frame composite).
-- Reset duplicate_status so items get re-evaluated after rehashing.
ALTER TABLE media_items ALTER COLUMN phash TYPE VARCHAR(256);
UPDATE media_items SET phash = NULL WHERE phash IS NOT NULL;
UPDATE media_items SET duplicate_status = 'none' WHERE duplicate_status != 'none';
