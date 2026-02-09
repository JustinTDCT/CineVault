-- One-time cleanup: remove MD5 hashes and convert old "exact" duplicate statuses.
-- NOTE: phash reset removed — it was wiping valid hashes on every container restart.
UPDATE media_items SET file_hash = NULL WHERE file_hash IS NOT NULL;
UPDATE media_items SET duplicate_status = '' WHERE duplicate_status = 'exact';
