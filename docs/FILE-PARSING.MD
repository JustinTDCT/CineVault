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

### Step 4: NFO Sidecar Detection

After filename parsing, the scanner checks for `.nfo` sidecar files containing IMDB IDs:

1. **Sidecar match**: looks for `MovieName.nfo` next to `MovieName.mkv`
2. **Directory match**: if only one video file exists in the folder, any `.nfo` file in the folder is checked
3. **IMDB extraction**: scans the NFO content for `tt` followed by 7+ digits (e.g., `tt0133093`)

When an IMDB ID is found, it can be used for **direct metadata lookup**, bypassing fuzzy title matching entirely.

### Step 5: FFprobe Analysis

For video and audio files, ffprobe extracts technical metadata:
- Duration, resolution, width, height
- Video codec, audio codec, bitrate
- Container format (from file extension)

FFprobe values **override** any filename-parsed values for resolution and container.

### Step 6: Hierarchy Creation

**TV Shows** (when season grouping is enabled):
- Show name → `tv_shows` table
- Season number → `tv_seasons` table
- Episode → linked to `media_items`

**Music**:
- Artist → `artists` table (found or created by name)
- Album → `albums` table (found or created by title under artist)
- Track → linked to `media_items`

### Step 7: Metadata Matching

After parsing and insertion, items are matched against external providers:

1. **Cache server** checked first (if enabled)
2. **Direct API**: TMDB (movies/TV), MusicBrainz (music), OpenLibrary (audiobooks)
3. **Confidence scoring**: 0.6 minimum threshold, +0.20 boost for year match, -0.40 penalty for year mismatch
4. **Enrichment**: genres, IMDB ID, OMDb ratings, cast/crew, poster download

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

## Comparison With Previous Parser

| Feature | Old Parser | New Parser |
|---|---|---|
| Movie parsing | Single strict regex only | Strict first, universal fallback |
| TV episode patterns | 1 pattern (`Name - SxxExx`) | 5+ patterns (S01E01, 1x02, date, verbose, episode-only) |
| Filename cleaning | ~30 junk tokens, single regex | 100+ tokens, token-bitmap approach with year breakpoint |
| Year extraction | No false-positive protection | Delimiter-aware, avoids matching episode numbers |
| Extras detection | None (ingested as media) | 15 directory rules + 11 suffix rules + sample size check |
| NFO sidecar | Not supported | Reads IMDB ID from .nfo files for direct matching |
| Source detection | Not tracked | Identifies bluray, dvd, web, hdtv, etc. |
| Multi-episode | Not supported | S01E01-E03 range detection |
| Delimiter handling | Only `.` and `_` normalized | All common delimiters handled |
