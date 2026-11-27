-- 0006: audiobooks. A book is a title (artist = author) with a narrator and
-- a place in a series. A single .m4b is one part; a folder of MP3s is a book
-- whose files are parts (tracks). Chapters belong to a file. Listening
-- position is kept per title so a book resumes where you stopped.

ALTER TABLE titles
    ADD COLUMN series        text,
    ADD COLUMN series_index  numeric(6, 2),
    ADD COLUMN narrator      text;

CREATE TABLE chapters (
    version_id     bigint NOT NULL REFERENCES versions (id) ON DELETE CASCADE,
    idx            integer NOT NULL,
    start_seconds  numeric(10, 3) NOT NULL,
    end_seconds    numeric(10, 3) NOT NULL,
    name           text NOT NULL DEFAULT '',
    PRIMARY KEY (version_id, idx)
);

CREATE TABLE playback_progress (
    title_id          bigint PRIMARY KEY REFERENCES titles (id) ON DELETE CASCADE,
    version_id        bigint NOT NULL REFERENCES versions (id) ON DELETE CASCADE,
    position_seconds  numeric(10, 3) NOT NULL CHECK (position_seconds >= 0),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
