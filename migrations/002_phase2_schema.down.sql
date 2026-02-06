-- Phase 2 rollback
DROP TABLE IF EXISTS watch_history;
DROP TABLE IF EXISTS collection_items;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS edition_items;
DROP TABLE IF EXISTS edition_groups;

ALTER TABLE media_items DROP CONSTRAINT IF EXISTS fk_media_sister_group;
DROP TABLE IF EXISTS sister_groups;

ALTER TABLE media_items DROP COLUMN IF EXISTS artist_id;
ALTER TABLE media_items DROP COLUMN IF EXISTS album_id;
ALTER TABLE media_items DROP COLUMN IF EXISTS track_number;
ALTER TABLE media_items DROP COLUMN IF EXISTS disc_number;
ALTER TABLE media_items DROP COLUMN IF EXISTS author_id;
ALTER TABLE media_items DROP COLUMN IF EXISTS book_id;
ALTER TABLE media_items DROP COLUMN IF EXISTS chapter_number;
ALTER TABLE media_items DROP COLUMN IF EXISTS image_gallery_id;
ALTER TABLE media_items DROP COLUMN IF EXISTS sister_group_id;
ALTER TABLE media_items DROP COLUMN IF EXISTS phash;
ALTER TABLE media_items DROP COLUMN IF EXISTS audio_fingerprint;

DROP TABLE IF EXISTS image_galleries;
DROP TABLE IF EXISTS books;
DROP TABLE IF EXISTS book_series;
DROP TABLE IF EXISTS authors;
DROP TABLE IF EXISTS albums;
DROP TABLE IF EXISTS artists;

-- NOTE: Cannot remove values from enums in PostgreSQL without recreating them
