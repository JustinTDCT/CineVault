-- 0012: metadata lookups for albums (MusicBrainz), books and audiobooks
-- (Open Library) and comic series (Comic Vine); the per-library choice of
-- what the files say over what lookups find; whole-item locks.
--
-- Locks: titles.locked_fields (0001) lists the fields a person edited, which
-- no lookup or cache record changes. '*' in it locks the whole item.

ALTER TABLE libraries
    -- false: looked-up data replaces what the files say (the default).
    -- true: what the files say wins; lookups only fill empty fields.
    ADD COLUMN prefer_local_metadata boolean NOT NULL DEFAULT false;

ALTER TABLE titles
    -- The artist or author as read from tags or file names. Scans find a
    -- title by file_title, file_year and file_artist, so a lookup that
    -- changes the shown artist doesn't split the title on the next scan.
    ADD COLUMN file_artist          text,
    ADD COLUMN mb_release_group_id  text,     -- albums: MusicBrainz release group
    ADD COLUMN ol_work_id           text,     -- books, audiobooks: Open Library work (OL45883W)
    ADD COLUMN comicvine_id         integer,  -- comic series: Comic Vine volume
    ADD COLUMN cover_url            text,     -- art found by a lookup (Cover Art Archive, Open Library, Comic Vine)
    ADD COLUMN publisher            text;     -- comics (and books when known)

UPDATE titles SET file_artist = artist;
