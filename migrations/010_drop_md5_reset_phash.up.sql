-- Remove MD5 hashes and reset phash values so they get recomputed with multi-point sampling.
-- Also convert any "exact" duplicate statuses to empty (MD5-based exact dupe no longer exists).
UPDATE media_items SET file_hash = NULL WHERE file_hash IS NOT NULL;
UPDATE media_items SET phash = NULL WHERE phash IS NOT NULL;
UPDATE media_items SET duplicate_status = '' WHERE duplicate_status = 'exact';
