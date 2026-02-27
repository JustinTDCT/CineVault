ALTER TABLE media_items DROP COLUMN IF EXISTS audience_score;
ALTER TABLE media_items DROP COLUMN IF EXISTS rt_rating;
ALTER TABLE media_items DROP COLUMN IF EXISTS imdb_rating;
DROP TABLE IF EXISTS system_settings;
