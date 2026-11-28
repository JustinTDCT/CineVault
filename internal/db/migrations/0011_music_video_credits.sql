-- Music videos matched to MusicBrainz, and their credits from the CineVault
-- Cache (cache decision 11).

-- The MusicBrainz recording a music video is matched to, and the album and
-- year read from it.
ALTER TABLE titles ADD COLUMN mb_recording_id text;
ALTER TABLE titles ADD COLUMN album text;
ALTER TABLE titles ADD COLUMN album_year integer;

-- The cache's credits for a music video: {song, artist, album, year, label,
-- director, confidence}, shown MTV-style as it plays.
ALTER TABLE titles ADD COLUMN cache_credits jsonb;
