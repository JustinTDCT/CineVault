-- 0005: music. An album is a title (with its artist), album editions are
-- editions (Deluxe, Remaster...), and each song is a track. A track's files
-- are versions linked to the track and to the album edition they came from.

CREATE TABLE tracks (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title_id      bigint NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    disc_number   integer NOT NULL DEFAULT 1 CHECK (disc_number >= 0),
    track_number  integer NOT NULL DEFAULT 0 CHECK (track_number >= 0), -- 0 when unknown
    name          text NOT NULL,
    artist        text, -- set when it differs from the album artist
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- One row per song on an album: the same disc, number and name is the same
-- track, so a FLAC and an MP3 of it are two versions of one track.
CREATE UNIQUE INDEX tracks_identity_idx ON tracks (title_id, disc_number, track_number, lower(name));

ALTER TABLE versions ADD COLUMN track_id bigint REFERENCES tracks (id) ON DELETE CASCADE;
CREATE INDEX versions_track_idx ON versions (track_id) WHERE track_id IS NOT NULL;
