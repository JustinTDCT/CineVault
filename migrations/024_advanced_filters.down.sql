DROP INDEX IF EXISTS idx_media_bitrate;
DROP INDEX IF EXISTS idx_media_audio_format;
DROP INDEX IF EXISTS idx_media_audio_codec;
DROP INDEX IF EXISTS idx_media_resolution;
DROP INDEX IF EXISTS idx_media_hdr_format;
DROP INDEX IF EXISTS idx_media_codec;
ALTER TABLE media_items DROP COLUMN IF EXISTS audio_format;
