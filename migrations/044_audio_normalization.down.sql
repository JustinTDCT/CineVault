ALTER TABLE media_items DROP COLUMN IF EXISTS loudness_gain_db;
ALTER TABLE media_items DROP COLUMN IF EXISTS loudness_lufs;
ALTER TABLE libraries DROP COLUMN IF EXISTS audio_normalization;
