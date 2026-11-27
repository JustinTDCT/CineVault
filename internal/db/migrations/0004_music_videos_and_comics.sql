-- 0004: music video and comic libraries, and the artist of a title.

ALTER TABLE libraries DROP CONSTRAINT libraries_media_type_check;
ALTER TABLE libraries ADD CONSTRAINT libraries_media_type_check
    CHECK (media_type IN ('movie', 'tv', 'music', 'musicvideo', 'audiobook', 'book', 'comic'));

ALTER TABLE titles DROP CONSTRAINT titles_media_type_check;
ALTER TABLE titles ADD CONSTRAINT titles_media_type_check
    CHECK (media_type IN ('movie', 'tv', 'music', 'musicvideo', 'audiobook', 'book', 'comic'));

-- Who made it: a music video's artist (later an album's artist or a book's
-- author). Part of how scans find a title, so two songs called "Poison" by
-- different artists stay apart.
ALTER TABLE titles ADD COLUMN artist text;

-- Music video editions. Live, Explicit and Clean already exist for music.
INSERT INTO edition_types (id, display_name, short_label, media_types, aliases, sort_order, is_default, builtin) VALUES
('official_video',    'Official Video',      'Official',   '{musicvideo}',       '{official video,official music video,official}', 700, true,  true),
('acoustic',          'Acoustic/Unplugged',  'Acoustic',   '{music,musicvideo}', '{acoustic,unplugged,mtv unplugged,acoustic version}', 710, false, true),
('lyric_video',       'Lyric Video',         'Lyric',      '{musicvideo}',       '{lyric video,lyrics video,lyric,lyrics}',     720, false, true),
('alternate_version', 'Alternate Version',   'Alternate',  '{musicvideo}',       '{alternate version,alt version,alternate,us version,u s version,uk version,u k version,international version,version 2}', 730, false, true),
('extended_mix',      'Extended/Remix',      'Remix',      '{music,musicvideo}', '{extended mix,extended version,remix,club mix}', 740, false, true);

UPDATE edition_types SET media_types = media_types || '{musicvideo}'
WHERE id IN ('live', 'explicit', 'clean', 'remaster');

-- Comics share the book editions.
UPDATE edition_types SET media_types = media_types || '{comic}'
WHERE id IN ('original', 'revised', 'annotated', 'omnibus');
