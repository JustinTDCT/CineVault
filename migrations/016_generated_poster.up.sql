-- Add flag to track posters generated from video screenshots vs downloaded from metadata sources
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS generated_poster BOOLEAN NOT NULL DEFAULT false;

-- Mark all existing items as generated (so next rescan replaces them with TMDB posters)
UPDATE media_items SET generated_poster = true WHERE poster_path IS NOT NULL;
