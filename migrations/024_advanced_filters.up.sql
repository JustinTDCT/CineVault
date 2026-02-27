-- Add audio_format column for enhanced audio detection (Atmos, DTS:X, etc.)
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS audio_format VARCHAR(100);

-- Add indexes for new filter columns
CREATE INDEX IF NOT EXISTS idx_media_codec ON media_items (codec) WHERE codec IS NOT NULL AND codec != '';
CREATE INDEX IF NOT EXISTS idx_media_hdr_format ON media_items (hdr_format) WHERE hdr_format IS NOT NULL AND hdr_format != '';
CREATE INDEX IF NOT EXISTS idx_media_resolution ON media_items (resolution) WHERE resolution IS NOT NULL AND resolution != '';
CREATE INDEX IF NOT EXISTS idx_media_audio_codec ON media_items (audio_codec) WHERE audio_codec IS NOT NULL AND audio_codec != '';
CREATE INDEX IF NOT EXISTS idx_media_audio_format ON media_items (audio_format) WHERE audio_format IS NOT NULL AND audio_format != '';
CREATE INDEX IF NOT EXISTS idx_media_bitrate ON media_items (bitrate) WHERE bitrate IS NOT NULL;
