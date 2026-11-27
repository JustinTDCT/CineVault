-- 0002: metadata from TMDb and match tracking on titles.

ALTER TABLE titles
    -- The title as read from file names. Scans match files to titles by this,
    -- so renaming a title to TMDb's spelling doesn't split it on the next scan.
    ADD COLUMN file_title          text,
    ADD COLUMN original_title      text,
    ADD COLUMN tagline             text,
    ADD COLUMN release_date        date,
    ADD COLUMN runtime_minutes     integer,
    ADD COLUMN poster_path         text,   -- TMDb image path, e.g. /abc.jpg
    ADD COLUMN backdrop_path       text,
    ADD COLUMN imdb_id             text,
    ADD COLUMN genres              text[] NOT NULL DEFAULT '{}',
    ADD COLUMN rating              numeric(3, 1),
    ADD COLUMN collection_tmdb_id  integer, -- TMDb collection, e.g. Alien Collection
    ADD COLUMN collection_name     text,
    ADD COLUMN match_status        text NOT NULL DEFAULT 'unmatched'
        CHECK (match_status IN ('unmatched', 'matched', 'not_found', 'error')),
    ADD COLUMN match_error         text,
    ADD COLUMN matched_at          timestamptz;

UPDATE titles SET file_title = name;
ALTER TABLE titles ALTER COLUMN file_title SET NOT NULL;

CREATE INDEX titles_file_title_idx ON titles (library_id, lower(file_title));
CREATE INDEX titles_to_match_idx ON titles (library_id) WHERE match_status IN ('unmatched', 'error');
