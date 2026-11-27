-- 0003: TV shows. A show is a title; its files are versions linked to an
-- episode (and to the show's edition, so per-episode editions can come later).

ALTER TABLE titles
    -- The year as read from file or folder names. Scans find titles by
    -- file_title and file_year, so TMDb filling in the display year for a
    -- folder without one doesn't split the title on the next scan.
    ADD COLUMN file_year integer;

UPDATE titles SET file_year = year;

CREATE TABLE seasons (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title_id       bigint NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    season_number  integer NOT NULL CHECK (season_number >= 0), -- 0 is Specials
    name           text,
    overview       text,
    poster_path    text,
    air_date       date,
    UNIQUE (title_id, season_number)
);

CREATE TABLE episodes (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title_id         bigint NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    season_number    integer NOT NULL CHECK (season_number >= 0),
    episode_number   integer NOT NULL CHECK (episode_number >= 0),
    name             text,
    overview         text,
    air_date         date,
    runtime_minutes  integer,
    still_path       text,
    rating           numeric(3, 1),
    tmdb_id          integer, -- null until matched; set for episodes not owned too
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (title_id, season_number, episode_number)
);

ALTER TABLE versions
    ADD COLUMN episode_id    bigint REFERENCES episodes (id) ON DELETE CASCADE,
    -- For a file holding several episodes (S01E01-E02), the last one's number.
    ADD COLUMN last_episode  integer;

CREATE INDEX versions_episode_idx ON versions (episode_id) WHERE episode_id IS NOT NULL;
