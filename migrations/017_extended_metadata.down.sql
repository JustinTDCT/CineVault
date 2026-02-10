-- Migration 017 rollback

ALTER TABLE media_items DROP COLUMN IF EXISTS tagline;
ALTER TABLE media_items DROP COLUMN IF EXISTS original_language;
ALTER TABLE media_items DROP COLUMN IF EXISTS country;
ALTER TABLE media_items DROP COLUMN IF EXISTS trailer_url;
ALTER TABLE media_items DROP COLUMN IF EXISTS logo_path;
ALTER TABLE media_items DROP COLUMN IF EXISTS locked_fields;

ALTER TABLE tv_shows DROP COLUMN IF EXISTS banner_path;
ALTER TABLE tv_shows DROP COLUMN IF EXISTS tagline;
ALTER TABLE tv_shows DROP COLUMN IF EXISTS network;
ALTER TABLE tv_shows DROP COLUMN IF EXISTS rating;
ALTER TABLE tv_shows DROP COLUMN IF EXISTS content_rating;
ALTER TABLE tv_shows DROP COLUMN IF EXISTS external_ids;

ALTER TABLE tv_seasons DROP COLUMN IF EXISTS external_ids;

ALTER TABLE libraries DROP COLUMN IF EXISTS nfo_import;
ALTER TABLE libraries DROP COLUMN IF EXISTS nfo_export;
ALTER TABLE libraries DROP COLUMN IF EXISTS prefer_local_artwork;

ALTER TABLE movie_series DROP COLUMN IF EXISTS external_ids;
ALTER TABLE movie_series DROP COLUMN IF EXISTS backdrop_path;
ALTER TABLE movie_series DROP COLUMN IF EXISTS description;
