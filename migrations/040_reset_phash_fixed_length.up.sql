-- Reset all phash values again: previous algorithm produced variable-length hashes
-- that broke Hamming distance comparison. New algorithm always produces exactly
-- 112-char hashes (7 frames × 8 bytes × 2 hex = 112 chars).
UPDATE media_items SET phash = NULL WHERE phash IS NOT NULL;
UPDATE media_items SET duplicate_status = 'none' WHERE duplicate_status != 'none';
