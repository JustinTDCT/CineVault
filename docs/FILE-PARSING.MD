# CineVault File Parsing Guide

This document describes how CineVault parses media filenames, the expected file/folder structure, and how files flow through the ingestion pipeline.

---

## Parsing Architecture

CineVault uses a **hybrid parsing approach** combining three strategies:

1. **CineVault Strict Patterns** — tried first for well-named files following the recommended naming convention
2. **Token-Based Garbage Detection** (Plex-inspired) — tokenizes the filename, identifies junk tokens, and uses the year as a breakpoint
3. **Multi-Pattern Episode Detection** (Jellyfin-inspired) — applies 5+ episode regex patterns covering all common TV naming formats

If a strict pattern matches, the result is used immediately. If not, the universal fallback handles any filename format.

---

## Recommended File Structure

### Movies

```
/Movies/
  Movie Name (2020)/
    Movie Name (2020).mkv
    Movie Name (2020).nfo          ← optional, contains IMDB ID
    Movie Name (2020).srt          ← optional subtitles
  Movie Name (2020) {Director's Cut}/
    Movie Name (2020) {Director's Cut}.mkv
```

**Preferred naming:** `Title (Year) {Edition} [Resolution/Container]`

```
Aliens (1986).mkv
Aliens (1986) {Director's Cut} [1080p/MKV].mkv
Blade Runner (1982) {edition-Final Cut} [2160p/MKV].mkv
```

**Also supported (universal fallback):**
```
The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv
The Matrix (1999) Remastered 1080p BluRay.mkv
The.Matrix.1999.mkv
```

### TV Shows

```
/TV Shows/
  Show Name/
    Season 01/
      Show Name - S01E01.mkv
      Show Name - S01E02 - Episode Title.mkv
    Season 02/
      Show Name - S02E01.mkv
```

**Preferred naming:** `Show Name - S01E01.ext`

**Also supported (universal fallback):**
```
Show.Name.S01E01.720p.HDTV.x264-GROUP.mkv
Show Name 1x01 Episode Title.mkv
Show.Name.S01E01-E03.mkv                     ← multi-episode
Show.Name.2024.05.21.mkv                     ← date-based
Season 1 Episode 5.mkv                       ← verbose format
Show Name Episode 12.mkv                     ← episode only
```

### Music

```
/Music/
  Artist Name/
    Album Name/
      Artist Name - Album Name - D01T01.flac
      Artist Name - Album Name - D01T02.flac
```

**Naming:** `Artist - Album - DxxTxx.ext`

### Music Videos

```
/Music Videos/
  Artist Name - Song Title.mp4
```

**Naming:** `Artist - Song.ext`

### Adult Content

```
/Adult/
  XXX - Title (Year) {Edition} [Resolution/Container].mkv
```

### Multi-Part Files

Files with part indicators are automatically grouped into sister sets:

```
Movie Name (2020) CD1.mkv
Movie Name (2020) CD2.mkv
Movie Name (2020) DISC-1.mkv
Movie Name (2020) PART-2.mkv
```

Recognized part indicators: `CD`, `DISC`, `DISK`, `PART`, `PT` followed by a number.

---

## Parsing Pipeline (Step by Step)

### Step 1: Extension Validation

Files are filtered by extension based on library type:

| Library Type | Valid Extensions |
|---|---|
| Movies, TV, Music Videos, Home Videos | `.mp4`, `.mkv`, `.avi`, `.mov`, `.m4v`, `.wmv`, `.flv`, `.webm`, `.ts`, `.m2ts`, `.mpg`, `.mpeg` |
| Music | `.mp3`, `.flac`, `.aac`, `.ogg`, `.wav`, `.m4a`, `.alac`, `.wma`, `.opus` |
| Audiobooks | `.mp3`, `.m4b`, `.aac`, `.flac` |
| Images | `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`, `.bmp`, `.tiff`, `.tif` |

### Step 2: Extras Filtering

Before any parsing, files are checked against extras detection rules. Matched files are **skipped entirely** and not added to the library.

**Directory-based rules** — files inside these directories are classified as extras:
- `trailers/`, `trailer/`
- `samples/`, `sample/`
- `extras/`, `extra/`
- `bonus/`
- `deleted scenes/`, `deleted scene/`
- `behind the scenes/`
- `interviews/`, `interview/`
- `featurettes/`, `featurette/`
- `scenes/`, `scene/`
- `shorts/`, `short/`
- `clips/`, `clip/`
- `other/`
- `backdrops/`
- `theme-music/`
- `special features/`

**Filename suffix rules** — files ending with these suffixes are classified as extras:
- `-trailer`, `.trailer`, `_trailer`
- `-sample`, `.sample`, `_sample`
- `-behindthescenes`
- `-deleted`, `-deletedscene`
- `-featurette`
- `-short`
- `-interview`
- `-scene`
- `-clip`
- `-extra`
- `-other`

**Sample size threshold**: Files matching a sample pattern are only skipped if they are under 300 MB. Larger files are assumed to be legitimate content.

### Step 3: Filename Parsing

The parser extracts: **Title**, **Year**, **Edition**, **Resolution**, **Source**, **Season/Episode** (for TV), **Artist/Album/Track** (for music).

#### Movies — Two-Tier Parsing

**Tier 1 (Strict)**: Matches `Title (Year) {Edition} [Resolution/Container]`
```
Input:  "Aliens (1986) {Director's Cut} [1080p/MKV]"
Result: title="Aliens", year=1986, edition="Director's Cut", resolution="1080p", container="mkv"
```

**Tier 2 (Universal Fallback)**: Token-based garbage detection
```
Input:  "The.Matrix.1999.1080p.BluRay.x264-GROUP"
Process:
  1. Strip brackets: [xxx] and {xxx} removed
  2. Extract year: "1999" found, used as breakpoint → take "The.Matrix" before it
  3. Normalize: dots→spaces → "The Matrix"
  4. Tokenize: ["The", "Matrix"]
  5. Garbage bitmap: both tokens are good (not in garbage set)
  6. Result: title="The Matrix", year=1999, resolution="1080p", source="bluray"
```

#### TV Shows — Multi-Pattern Detection

Patterns are tried in order of specificity:

| Priority | Pattern | Example | Extracted |
|---|---|---|---|
| 1 | `S01E01` | `Show.S01E01.720p.mkv` | S=1, E=1 |
| 2 | `1x01` | `Show.1x02.mkv` | S=1, E=2 |
| 3 | `Season X Episode Y` | `Season 1 Episode 5.mkv` | S=1, E=5 |
| 4 | `Episode N` | `Show Episode 12.mkv` | S=1, E=12 |
| 5 | Date-based `YYYY.MM.DD` | `Show.2024.05.21.mkv` | S=2024, E=521 |

Multi-episode files are supported: `S01E01-E03` → episode=1, episodeEnd=3

The show name is extracted from the portion of the filename **before** the episode indicator, then cleaned of dots, underscores, and trailing junk.

### Step 4: Inline Provider ID Extraction

Before any title cleaning, filenames are scanned for embedded provider IDs in bracket notation:

| Pattern | Example | Result |
|---|---|---|
| `[tmdbid=12345]` | `Movie [tmdbid=550].mkv` | TMDB ID 550 |
| `[imdbid=tt1234567]` | `Movie [imdbid=tt0133093].mkv` | IMDB ID tt0133093 |
| `[tvdbid=12345]` | `Show [tvdbid=73255].mkv` | TVDB ID 73255 |

When a TMDB ID is found, the scanner performs a **direct TMDB lookup** by ID, bypassing both the cache server and fuzzy title matching entirely. The brackets are stripped from the title before further parsing.

### Step 5: NFO Sidecar Detection

After filename parsing, the scanner checks for `.nfo` sidecar files. CineVault supports full **Kodi-compatible NFO parsing**, not just IMDB ID extraction.

**File discovery:**
1. **Sidecar match**: looks for `MovieName.nfo` next to `MovieName.mkv`
2. **Directory match**: if only one video file exists in the folder, any `.nfo` file in the folder is checked
3. **TV show-level**: `tvshow.nfo` in the show root directory

**NFO types supported:**
- **Movie NFOs** (`<movie>` root element) — title, tagline, plot, year, runtime, MPAA, country, trailer URL, ratings, genres, studios, cast, directors, writers, artwork references, and provider IDs (IMDB, TMDB, TVDB)
- **TV Show NFOs** (`<tvshow>` root element) — title, plot, year, MPAA, genres, studios, cast, provider IDs
- **Episode NFOs** (`<episodedetails>` root element) — title, plot, season/episode numbers, aired date, cast, provider IDs

**What NFO data can do:**
- If the NFO contains a provider ID (IMDB, TMDB, or TVDB), it triggers a **direct lookup** bypassing fuzzy search
- If the NFO has full metadata (title + plot + year) and `<lockdata>true</lockdata>`, metadata is applied directly without external API calls
- NFO export is also supported: when enabled on a library, CineVault writes Kodi-compatible NFO files after metadata is fetched

**Library settings control NFO behavior:**
- `nfo_import` — read NFO files during scan
- `nfo_export` — write NFO files after metadata is resolved

### Step 6: Local Artwork Detection

When `prefer_local_artwork` is enabled on a library, the scanner checks for artwork files alongside the media using Plex/Jellyfin/Kodi naming conventions.

**Movie/episode artwork** (searched next to the media file):
| Type | Names checked |
|---|---|
| Poster | `poster`, `cover`, `folder`, `<filename>-poster` |
| Backdrop | `backdrop`, `fanart`, `background`, `<filename>-fanart` |
| Logo | `logo`, `clearlogo` |
| Banner | `banner` |
| Thumbnail | `<filename>-thumb` |

**TV show artwork** (searched in the show root directory):
| Type | Names checked |
|---|---|
| Poster | `poster`, `show`, `folder`, `cover` |
| Backdrop | `backdrop`, `fanart`, `background` |
| Logo | `logo`, `clearlogo` |
| Banner | `banner` |

**Season artwork**: `season01-poster.jpg`, `season01.jpg`, etc.

Supported image extensions: `.jpg`, `.jpeg`, `.png`, `.webp`

Local artwork takes priority over downloaded artwork when the library setting is enabled.

### Step 7: FFprobe Analysis

For video and audio files, ffprobe extracts technical metadata:
- Duration, resolution, width, height
- Video codec, audio codec, bitrate
- Container format (from file extension)

FFprobe values **override** any filename-parsed values for resolution and container.

### Step 8: Hierarchy Creation

**TV Shows** (when season grouping is enabled):
- Show name → `tv_shows` table
- Season number → `tv_seasons` table
- Episode → linked to `media_items`

**Music**:
- Artist → `artists` table (found or created by name)
- Album → `albums` table (found or created by title under artist)
- Track → linked to `media_items`

### Step 9: Metadata Matching

After parsing and insertion, items are matched against external providers. See [METADATA.MD](METADATA.MD) for the full metadata gathering architecture.

**Matching priority order:**
1. **Inline provider ID** (TMDB/IMDB/TVDB from filename) → direct lookup, highest accuracy
2. **NFO provider ID** (from sidecar file) → direct lookup
3. **NFO full metadata** (title + plot + year + lockdata) → applied directly, no API call
4. **Cache server** (if enabled) → returns fully-enriched data from all sources
5. **Direct API fallback**: TMDB (movies/TV), MusicBrainz (music), OpenLibrary (audiobooks)

**Confidence scoring**: 0.6 minimum threshold, +0.20 boost for year match, -0.40 penalty for year mismatch

**Enrichment applied after matching:**
- Tagline, original language, country, trailer URL
- Genres, content rating
- IMDB/OMDb ratings (IMDB, Rotten Tomatoes critic + audience)
- Cast/crew (top 20 cast + key crew)
- Poster and backdrop download
- Logo and banner from fanart.tv
- Auto-collection creation from TMDB `belongs_to_collection`
- External IDs stored (TMDB, IMDB, TVDB, MusicBrainz, OpenLibrary, PornDB)

---

## Garbage Token Reference

The following tokens are recognized as junk and stripped during universal fallback parsing:

### Video Codecs
`x264`, `x265`, `h264`, `h265`, `h.264`, `h.265`, `hevc`, `avc`, `divx`, `xvid`, `10bit`, `8bit`, `hi10p`, `av1`, `vp9`, `mpeg4`

### Audio Codecs
`aac`, `ac3`, `dts`, `dts-hd`, `truehd`, `atmos`, `flac`, `mp3`, `ogg`, `vorbis`, `opus`, `eac3`

### Resolution
`480p`, `480i`, `576p`, `576i`, `720p`, `720i`, `1080p`, `1080i`, `2160p`, `4k`, `uhd`, `ultrahd`

### Source
`bluray`, `blu-ray`, `bdrip`, `brrip`, `bdremux`, `hdrip`, `hddvd`, `dvd`, `dvdrip`, `dvdscr`, `webrip`, `web-dl`, `webdl`, `hdtv`, `pdtv`, `dsr`, `dsrip`, `tvrip`, `cam`, `screener`, `telecine`, `telesync`, `retail`

### Release / Misc
`remux`, `proper`, `repack`, `rerip`, `internal`, `limited`, `custom`, `extended`, `unrated`, `remastered`, `multi`, `multisubs`, `dubbed`, `subbed`, `subs`, `ws`, `fs`, `fragment`, `xxx`, `nfo`, `read.nfo`, `cd1`–`cd4`

---

## How Year-as-Breakpoint Works

This is the key technique borrowed from Plex. When a year is found in the filename:

1. Everything **before** the year is assumed to be the title
2. Everything **after** the year is assumed to be technical metadata / junk
3. The year itself is extracted and stored separately

**Example:**
```
Input:  "The.Grand.Budapest.Hotel.2014.1080p.BluRay.x264-SPARKS"
Step 1: Year "2014" found at position 31
Step 2: Take before → "The.Grand.Budapest.Hotel"
Step 3: Normalize dots → "The Grand Budapest Hotel"
Result: title="The Grand Budapest Hotel", year=2014
```

This is significantly more reliable than trying to strip junk tokens from the full filename, because the year creates a natural boundary between title and technical info.

---

## Edition Support

CineVault supports edition tags in the Radarr/Sonarr convention:

```
{Director's Cut}
{edition-Remastered}
{edition-Extended}
{Special Edition}
```

The `edition-` prefix is stripped automatically. If no edition tag is found, the default is "Theatrical".

---

## Pipeline Summary

| Step | Description |
|---|---|
| 1. Extension Validation | Filter files by library type |
| 2. Extras Filtering | Skip trailers, samples, behind-the-scenes, etc. |
| 3. Filename Parsing | Extract title, year, edition, resolution, season/episode |
| 4. Inline Provider IDs | Extract `[tmdbid=X]`, `[imdbid=X]`, `[tvdbid=X]` from filename |
| 5. NFO Sidecar Detection | Parse Kodi-compatible NFO for metadata and provider IDs |
| 6. Local Artwork Detection | Find poster, backdrop, logo, banner alongside media files |
| 7. FFprobe Analysis | Extract technical metadata (duration, codec, resolution) |
| 8. Hierarchy Creation | Build TV show/season/episode or artist/album/track structures |
| 9. Metadata Matching | Match against cache server, TMDB, MusicBrainz, OpenLibrary |
