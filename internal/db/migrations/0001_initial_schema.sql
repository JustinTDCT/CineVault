-- 0001: libraries, the Edition List, and the Title → Edition → Version model.
-- See docs/specs/editions-and-versions.md.

CREATE TABLE libraries (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        text NOT NULL,
    media_type  text NOT NULL CHECK (media_type IN ('movie', 'tv', 'music', 'audiobook', 'book')),
    paths       text[] NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (name)
);

-- The Edition List (spec §4): a fixed vocabulary of edition names.
-- Built-in entries are seeded below; admins can add custom ones (builtin = false).
CREATE TABLE edition_types (
    id            text PRIMARY KEY CHECK (id ~ '^[a-z0-9_]+$'),
    display_name  text NOT NULL,
    short_label   text NOT NULL CHECK (char_length(short_label) BETWEEN 1 AND 12),
    media_types   text[] NOT NULL CHECK (cardinality(media_types) > 0),
    aliases       text[] NOT NULL DEFAULT '{}',
    sort_order    integer NOT NULL,
    is_default    boolean NOT NULL DEFAULT false, -- default edition for its media types
    builtin       boolean NOT NULL DEFAULT false,
    hidden        boolean NOT NULL DEFAULT false
);

CREATE TABLE titles (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    library_id      bigint NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    media_type      text NOT NULL CHECK (media_type IN ('movie', 'tv', 'music', 'audiobook', 'book')),
    name            text NOT NULL,
    sort_name       text NOT NULL,
    year            integer,
    overview        text,
    tmdb_id         integer,
    tvdb_id         integer,
    musicbrainz_id  uuid,
    locked_fields   text[] NOT NULL DEFAULT '{}', -- fields the user edited by hand
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX titles_library_sort_idx ON titles (library_id, sort_name);
CREATE INDEX titles_tmdb_idx ON titles (tmdb_id) WHERE tmdb_id IS NOT NULL;
CREATE INDEX titles_tvdb_idx ON titles (tvdb_id) WHERE tvdb_id IS NOT NULL;

-- One row per edition of a title (spec §3). Each title has at least one.
CREATE TABLE editions (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title_id         bigint NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    edition_type_id  text NOT NULL REFERENCES edition_types (id),
    runtime_seconds  integer CHECK (runtime_seconds > 0),
    description      text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (title_id, edition_type_id)
);

-- One row per file (or multi-part file set) of an edition (spec §3).
CREATE TABLE versions (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    edition_id        bigint NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    path              text NOT NULL UNIQUE,
    size_bytes        bigint NOT NULL CHECK (size_bytes >= 0),
    fingerprint       text, -- size + partial hash, survives renames and drive moves (spec §6)
    label             text, -- e.g. "4K · HDR10 · Atmos"
    container         text,
    video_codec       text,
    width             integer,
    height            integer,
    hdr               text, -- null, 'hdr10', 'hdr10plus', 'dolby_vision', 'hlg'
    audio_codec       text,
    audio_channels    integer,
    duration_seconds  numeric(10, 3),
    bitrate           bigint,
    edition_locked    boolean NOT NULL DEFAULT false, -- user assigned the edition by hand
    missing_since     timestamptz, -- set when the file goes offline (spec §13)
    added_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX versions_edition_idx ON versions (edition_id);
CREATE INDEX versions_fingerprint_idx ON versions (fingerprint) WHERE fingerprint IS NOT NULL;

-- Starter Edition List, approved 26 Nov 2026 (spec §4, decision 7).
INSERT INTO edition_types (id, display_name, short_label, media_types, aliases, sort_order, is_default, builtin) VALUES
-- Movies
('theatrical',         'Theatrical',          'Theatrical',  '{movie}',        '{theatrical}',                                   10, true,  true),
('directors_cut',      'Director''s Cut',     'DC',          '{movie,tv}',     '{directors cut,director''s cut,directors.cut,dc}', 20, false, true),
('extended',           'Extended',            'Extended',    '{movie,tv}',     '{extended,extended edition,extended cut}',      30, false, true),
('unrated',            'Unrated',             'Unrated',     '{movie}',        '{unrated}',                                      40, false, true),
('uncut',              'Uncut',               'Uncut',       '{movie,tv}',     '{uncut}',                                        50, false, true),
('special_edition',    'Special Edition',     'Special Ed.', '{movie}',        '{special edition,se}',                           60, false, true),
('final_cut',          'Final/Ultimate Cut',  'Final Cut',   '{movie}',        '{final cut,ultimate cut,ultimate edition}',     70, false, true),
('imax',               'IMAX',                'IMAX',        '{movie}',        '{imax,imax enhanced}',                           80, false, true),
('remastered',         'Remastered',          'Remastered',  '{movie,tv}',     '{remastered,remaster}',                          90, false, true),
('restored',           'Restored',            'Restored',    '{movie}',        '{restored,restoration}',                        100, false, true),
('anniversary_edition','Anniversary Edition', 'Anniversary', '{movie}',        '{anniversary edition,anniversary}',             110, false, true),
('black_and_white',    'Black & White',       'B&W',         '{movie}',        '{black and white,black & white,b&w,chrome,noir edition}', 120, false, true),
('open_matte',         'Open Matte',          'Open Matte',  '{movie}',        '{open matte}',                                  130, false, true),
('three_d',            '3D',                  '3D',          '{movie}',        '{3d,sbs,hsbs,ou,hou}',                          140, false, true),
('tv_edit',            'TV/Censored Edit',    'TV Edit',     '{movie}',        '{tv edit,tv cut,censored}',                     150, false, true),
('alternate_ending',   'Alternate Ending',    'Alt Ending',  '{movie}',        '{alternate ending,alt ending}',                 160, false, true),
('fan_edit',           'Fan Edit',            'Fan Edit',    '{movie}',        '{fan edit,fanedit}',                            170, false, true),
-- TV (Director's Cut, Extended, Uncut and Remastered above also apply to TV)
('broadcast',          'Broadcast',           'Broadcast',   '{tv}',           '{broadcast}',                                     5, true,  true),
('original_aspect',    'Original Aspect (4:3)', '4:3',       '{tv}',           '{4x3,4:3,original aspect}',                     210, false, true),
('widescreen',         'Widescreen',          'Widescreen',  '{tv}',           '{widescreen,16x9,16:9}',                        220, false, true),
-- Music
('original',           'Original',            'Original',    '{music,book}',   '{original}',                                    300, true,  true),
('remaster',           'Remaster',            'Remaster',    '{music}',        '{remaster,remastered}',                         310, false, true),
('deluxe',             'Deluxe/Expanded',     'Deluxe',      '{music}',        '{deluxe,deluxe edition,expanded,expanded edition}', 320, false, true),
('anniversary',        'Anniversary',         'Anniversary', '{music}',        '{anniversary,anniversary edition}',             330, false, true),
('mono',               'Mono',                'Mono',        '{music}',        '{mono}',                                        340, false, true),
('stereo',             'Stereo',              'Stereo',      '{music}',        '{stereo}',                                      350, false, true),
('live',               'Live',                'Live',        '{music}',        '{live}',                                        360, false, true),
('explicit',           'Explicit',            'Explicit',    '{music}',        '{explicit}',                                    370, false, true),
('clean',              'Clean',               'Clean',       '{music}',        '{clean,edited}',                                380, false, true),
('instrumental',       'Instrumental',        'Instr.',      '{music}',        '{instrumental}',                                390, false, true),
('demos',              'Demos',               'Demos',       '{music}',        '{demos,demo}',                                  400, false, true),
-- Audiobooks
('unabridged',         'Unabridged',          'Unabridged',  '{audiobook}',    '{unabridged}',                                  500, true,  true),
('abridged',           'Abridged',            'Abridged',    '{audiobook}',    '{abridged}',                                    510, false, true),
('dramatized',         'Dramatized',          'Dramatized',  '{audiobook}',    '{dramatized,dramatised,full cast}',             520, false, true),
('alternate_narrator', 'Alternate Narrator',  'Alt Narrator','{audiobook}',    '{alternate narrator}',                          530, false, true),
-- Books and comics (Original above also applies to books)
('revised',            'Revised',             'Revised',     '{book}',         '{revised,revised edition}',                     610, false, true),
('annotated',          'Annotated',           'Annotated',   '{book}',         '{annotated}',                                   620, false, true),
('omnibus',            'Collected/Omnibus',   'Omnibus',     '{book}',         '{omnibus,collected,collected edition}',         630, false, true);
