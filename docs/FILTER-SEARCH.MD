# CineVault Filtering and Search System

This document describes how CineVault's library filtering, global search, missing episode detection, and duplicate finder work.

---

## Overview

CineVault provides three ways to find and narrow down media:

1. **Library Filters** — dropdown-based filtering within a single library, combining multiple criteria with AND logic
2. **Global Search** — cross-library text search by title or filename
3. **Missing Episode Detection** — gap analysis for TV show seasons comparing actual vs expected episodes

All three are backed by the same `media_items` table and respect edition deduplication (non-default editions are excluded from library listings).

---

## Library Filters

### How It Works

When a user opens a library, the frontend fetches available filter options from the API, then renders dropdown selects for each filter category that has values. Selecting any filter triggers a reload of the media grid with the new criteria applied.

```
User selects filter → applyLibFilter() builds filter state
  → buildFilterQS() generates query string
  → GET /libraries/{id}/media?...filters...
  → Backend: parseMediaFilter() → MediaFilter struct
  → buildFilterClauses() → SQL JOIN/WHERE/ORDER BY
  → Filtered results returned with total count
```

### Available Filters

| Filter | Query Param | DB Column | Match Type |
|---|---|---|---|
| Genre | `genre` | `tags.name` (via JOIN) | Exact match on genre tag |
| Folder | `folder` | `file_path` | LIKE prefix match |
| Content Rating | `content_rating` | `content_rating` | Exact match |
| Edition | `edition` | `edition_type` | Exact match |
| Source | `source` | `source_type` | Exact match |
| Dynamic Range | `dynamic_range` | `dynamic_range` | Exact match (SDR or HDR) |
| Video Codec | `codec` | `codec` | Exact match (hevc, h264, av1, etc.) |
| HDR Format | `hdr_format` | `hdr_format` | Exact match (Dolby Vision, HDR10, HLG, PQ) |
| Resolution | `resolution` | `resolution` | Exact match (4K, 1080p, 720p, 480p, SD) |
| Audio Codec | `audio_codec` | `audio_codec` | Exact match (truehd, eac3, ac3, aac, dts, etc.) |
| Bitrate Range | `bitrate_range` | `bitrate` | Range buckets (see below) |

### Bitrate Range Buckets

The bitrate filter uses predefined ranges rather than exact values for usability:

| Value | Label | SQL Condition |
|---|---|---|
| `low` | < 5 Mbps | `bitrate < 5000000` |
| `medium` | 5-15 Mbps | `bitrate >= 5000000 AND bitrate < 15000000` |
| `high` | 15-30 Mbps | `bitrate >= 15000000 AND bitrate < 30000000` |
| `ultra` | 30+ Mbps | `bitrate >= 30000000` |

All bitrate filters also require `bitrate IS NOT NULL` to exclude items without bitrate data.

### Sort Options

| Value | Query Param | DB Column |
|---|---|---|
| Title (default) | `sort=` (empty) | `COALESCE(sort_title, title)` |
| Year | `sort=year` | `year` |
| Resolution | `sort=resolution` | `height` (pixel height for accurate ordering) |
| Length | `sort=duration` | `duration_seconds` |
| Bitrate | `sort=bitrate` | `bitrate` |
| Rotten Tomatoes | `sort=rt_rating` | `rt_rating` |
| TMDB Rating | `sort=rating` | `rating` |
| Audience Score | `sort=audience_score` | `audience_score` |

Sort direction is controlled by the `order` param: `asc` (default) or `desc`. All sort columns use `NULLS LAST` to push items without data to the end.

### Filter Combination Logic

All filters combine with **AND** logic. A query with `genre=Action&codec=hevc&resolution=4K` returns only items that are tagged Action AND encoded in HEVC AND are 4K resolution.

Edition deduplication is always applied: items that are non-default editions in an edition group are excluded from library listings via a NOT EXISTS subquery.

### Codec Display Names

The frontend maps raw codec values to user-friendly labels:

| Raw Value | Display Label |
|---|---|
| `hevc` | HEVC (H.265) |
| `h264` | H.264 |
| `av1` | AV1 |
| `mpeg4` | MPEG-4 |
| `mpeg2video` | MPEG-2 |
| `vp9` | VP9 |
| `vc1` | VC-1 |

### Audio Codec Display Names

| Raw Value | Display Label |
|---|---|
| `truehd` | TrueHD |
| `eac3` | EAC3 (DD+) |
| `ac3` | AC3 (DD) |
| `aac` | AAC |
| `dts` | DTS |
| `flac` | FLAC |
| `opus` | Opus |
| `mp3` | MP3 |

---

## Filter Options API

Before rendering the filter toolbar, the frontend fetches available values from the library:

`GET /api/v1/libraries/{id}/filters`

Returns:

```json
{
  "genres": ["Action", "Comedy", "Drama"],
  "folders": ["/media/movies/4k", "/media/movies/1080p"],
  "content_ratings": ["PG-13", "R"],
  "editions": ["Director's Cut", "Extended"],
  "sources": ["bluray", "web-dl"],
  "dynamic_ranges": ["HDR", "SDR"],
  "codecs": ["h264", "hevc", "av1"],
  "hdr_formats": ["Dolby Vision", "HDR10", "HLG"],
  "resolutions": ["4K", "1080p", "720p"],
  "audio_codecs": ["truehd", "eac3", "aac", "dts"]
}
```

Each array contains only values that actually exist in the library (DISTINCT queries). Empty arrays are returned for categories with no data. Resolutions are ordered by quality (4K first), all others are alphabetically sorted.

Only dropdowns with available values are rendered in the toolbar. The bitrate range filter is always shown since it uses fixed buckets rather than data-driven options.

---

## Letter Index

The letter index powers the A-Z jump navigation sidebar. It returns cumulative offsets for each starting letter, respecting any active filters:

`GET /api/v1/libraries/{id}/media/index?genre=Action&codec=hevc`

Returns:

```json
[
  {"letter": "#", "count": 3, "offset": 0},
  {"letter": "A", "count": 12, "offset": 3},
  {"letter": "B", "count": 8, "offset": 15}
]
```

The `offset` value tells the frontend how many items to load before reaching that letter. Clicking a letter triggers batch loading until the target offset is reached, then scrolls to the first matching card.

Letters are derived from `COALESCE(sort_title, title)`. Non-alphabetic characters are grouped under `#`.

---

## Global Search

### How It Works

The global search searches across all libraries the user has access to. It uses a simple ILIKE pattern match against both `title` and `file_name`.

```
User types in search box → 400ms debounce → loadSearchView()
  → GET /api/v1/media/search?q={query}
  → Backend: SearchInLibraries() with accessible library IDs
  → Results returned (max 50 items)
```

### Search Query

`GET /api/v1/media/search?q=matrix`

The query is wrapped in `%` wildcards for ILIKE matching: `WHERE title ILIKE '%matrix%' OR file_name ILIKE '%matrix%'`

### Access Control

Search respects library access settings:
1. The handler calls `ListSearchableLibraryIDs()` which returns library IDs where `is_enabled = true` AND `include_in_search = true`
2. Admin users see all enabled libraries with search enabled
3. Regular users only see libraries matching their access permissions (`everyone` or explicit `library_permissions` entry)
4. Non-default edition items are excluded from search results

### Limitations

- Text search only (no fuzzy matching, no stemming)
- No filter combination with search (search is a separate view from the filtered library grid)
- Maximum 50 results returned
- No pagination on search results

---

## Enhanced Audio Format Detection

CineVault detects spatial audio features beyond the base audio codec. The `audio_format` field stores an enhanced display string.

### Detection Logic

| Codec | Check | Result |
|---|---|---|
| TrueHD | Side data contains "atmos" or "joint object coding", or channels > 8 | TrueHD Atmos |
| EAC3 | Profile contains "atmos", or channels > 8 | EAC3 Atmos |
| DTS | Profile contains "dts:x" or "dtsx" | DTS-HD MA DTS:X |
| DTS | Profile contains "dts-hd ma" or "ma" | DTS-HD MA |
| DTS | Profile contains "dts-hd hra" or "hra" | DTS-HD HRA |
| Any | No spatial audio detected | Base codec display name (TrueHD, EAC3, AC3, AAC, FLAC, etc.) |

Detection runs during FFprobe analysis in the scan pipeline. The `audio_format` field is separate from `audio_codec` — `audio_codec` stores the raw FFprobe codec name (e.g., `truehd`), while `audio_format` stores the display-friendly enhanced name (e.g., `TrueHD Atmos`).

---

## Missing Episode Detection

### How It Works

CineVault detects gaps in TV show episode sequences by comparing actual episode numbers in the database against the expected range.

```
For each season in a TV library:
  1. Query actual episode numbers from media_items
  2. Determine expected count = MAX(max episode number, season.episode_count)
  3. Generate sequence 1..expected_count
  4. Report any numbers not present in the actual set
```

The `season.episode_count` field (populated during scanning) is used as an additional expected count source. If TMDB reports 12 episodes but you only have up to episode 10, the system still checks through 12.

### API Endpoints

**Library-wide detection:**

`GET /api/v1/libraries/{id}/missing-episodes`

Returns all shows with missing episodes:

```json
{
  "shows": [
    {
      "show_id": "uuid",
      "show_title": "Breaking Bad",
      "poster_path": "/previews/...",
      "total_missing": 3,
      "seasons": [
        {
          "season_id": "uuid",
          "season_number": 2,
          "have_count": 10,
          "expected_count": 13,
          "missing_numbers": [5, 11, 12]
        }
      ]
    }
  ],
  "total_missing": 3
}
```

**Per-season detection:**

`GET /api/v1/tv/seasons/{id}/missing`

Returns missing episodes for a single season with the same structure as a single season entry above.

### Frontend Display

Missing episode data appears in two places:

1. **Show detail view** — a warning badge next to the episode count (e.g., "3 missing episodes") and per-season missing count on season cards
2. **Season detail view** — an orange banner showing the have/expected ratio and listing the specific missing episode numbers (e.g., "10/13 episodes - Missing: 5, 11, 12")

---

## Duplicate Finder

CineVault has a built-in duplicate detection system using perceptual hashing.

### Detection Method

1. **Perceptual hashing (pHash)**: Extracts frames at 7 sample points (5%, 15%, 30%, 50%, 70%, 85%, 95% of duration), scales each to 32x32, computes hash bits, and creates a composite MD5 hash
2. **Audio fingerprinting**: Uses Chromaprint (`fpcalc`) when available, falls back to FFmpeg `astats` filter
3. **Pre-filtering**: Candidates are narrowed by duration similarity (within 5% difference)
4. **Similarity scoring**: Hash similarity calculated as a 0-1 score; threshold of 0.90 for duplicate detection

### Duplicate Status

Each media item has a `duplicate_status` field:

| Status | Description |
|---|---|
| `none` | No duplicates detected |
| `exact` | High-confidence duplicate (similarity >= 0.90) |
| `potential` | Possible duplicate (flagged for review) |
| `addressed` | User has reviewed and resolved |

### Resolution Actions

When reviewing duplicates, users can take one of four actions:

| Action | Description |
|---|---|
| `edit` | Fixed the issue (renamed, re-tagged) |
| `edition` | Converted to an edition group (e.g., Theatrical + Director's Cut) |
| `deleted` | Deleted the duplicate file |
| `ignored` | Acknowledged but left as-is |

Decisions are logged in the `duplicate_decisions` table with similarity scores and optional notes.

### API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/duplicates` | Admin | List all items with unresolved duplicate status |
| GET | `/api/v1/duplicates/count` | Admin | Count of unreviewed duplicates |
| GET | `/api/v1/media/{id}/duplicates` | Admin | Get duplicate matches for a specific item |
| POST | `/api/v1/duplicates/resolve` | Admin | Resolve a duplicate pair with an action |
| POST | `/api/v1/libraries/{id}/phash` | Admin | Trigger background pHash computation for a library |

The pHash job runs as an Asynq background task with a 6-hour timeout. It computes hashes for all items missing a pHash, then cross-compares all hashed items within the library to detect duplicates.

---

## Database Schema

### Migration 024 (Advanced Filters)

```sql
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS audio_format VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_media_codec ON media_items (codec);
CREATE INDEX IF NOT EXISTS idx_media_hdr_format ON media_items (hdr_format);
CREATE INDEX IF NOT EXISTS idx_media_resolution ON media_items (resolution);
CREATE INDEX IF NOT EXISTS idx_media_audio_codec ON media_items (audio_codec);
CREATE INDEX IF NOT EXISTS idx_media_audio_format ON media_items (audio_format);
CREATE INDEX IF NOT EXISTS idx_media_bitrate ON media_items (bitrate);
```

All indexes use partial index conditions (`WHERE column IS NOT NULL AND column != ''`) to minimize index size.

### Pre-existing Indexes (Migration 018)

```sql
CREATE INDEX idx_media_source_type ON media_items (source_type);
CREATE INDEX idx_media_dynamic_range ON media_items (dynamic_range);
```

### Key Columns Used by Filters

| Column | Type | Populated By |
|---|---|---|
| `codec` | VARCHAR(50) | FFprobe (`GetVideoCodec`) |
| `audio_codec` | VARCHAR(50) | FFprobe (`GetAudioCodec`) |
| `audio_format` | VARCHAR(100) | FFprobe (`GetAudioFormat`) |
| `resolution` | VARCHAR(20) | FFprobe (`GetResolution`) — 4K, 1080p, 720p, 480p, SD |
| `bitrate` | BIGINT | FFprobe (`GetBitrate`) — bits per second |
| `hdr_format` | VARCHAR(100) | FFprobe (`GetHDRFormat`) — Dolby Vision, HDR10, HDR10+, HLG, PQ |
| `dynamic_range` | VARCHAR(20) | FFprobe (`GetDynamicRange`) — SDR or HDR |
| `source_type` | VARCHAR(50) | Filename parser — bluray, web-dl, hdtv, remux, etc. |
| `content_rating` | VARCHAR(20) | TMDB/NFO — PG-13, R, etc. |
| `edition_type` | VARCHAR(100) | Filename parser — Director's Cut, Extended, etc. |

---

## API Reference

### Library Filtering

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/libraries/{id}/media` | User | List media with filters (see query params above) |
| GET | `/api/v1/libraries/{id}/media/index` | User | Letter index with filters applied |
| GET | `/api/v1/libraries/{id}/filters` | User | Available filter option values for a library |

### Search

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/media/search?q={query}` | User | Cross-library text search (max 50 results) |

### Missing Episodes

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/libraries/{id}/missing-episodes` | User | Detect missing episodes across all shows in a library |
| GET | `/api/v1/tv/seasons/{id}/missing` | User | Detect missing episodes for a specific season |

### Duplicates

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/duplicates` | Admin | List unresolved duplicates |
| GET | `/api/v1/duplicates/count` | Admin | Unreviewed duplicate count |
| GET | `/api/v1/media/{id}/duplicates` | Admin | Duplicates for a specific item |
| POST | `/api/v1/duplicates/resolve` | Admin | Resolve a duplicate pair |
| POST | `/api/v1/libraries/{id}/phash` | Admin | Trigger pHash computation job |

---

## Saved Filters

Users can save frequently used filter combinations as named presets for quick reuse.

### API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/filters` | User | List saved filter presets |
| POST | `/api/v1/filters` | User | Save a filter preset (name + filter parameters) |
| DELETE | `/api/v1/filters/{id}` | User | Delete a saved filter preset |

Saved filters store the complete filter state (genre, codec, resolution, sort, etc.) as a reusable preset. Clicking a saved filter applies all its parameters to the current library view.

---

## File Locations

| Component | Path |
|---|---|
| MediaFilter struct | `internal/repository/media_repository.go` |
| Filter clause builder | `internal/repository/media_repository.go` (`buildFilterClauses`) |
| Filter options query | `internal/repository/media_repository.go` (`GetLibraryFilterOptions`) |
| Filter API handler | `internal/api/handlers_library.go` (`handleLibraryFilters`) |
| Filter param parser | `internal/api/handlers_media.go` (`parseMediaFilter`) |
| Search handler | `internal/api/handlers_media.go` (`handleSearchMedia`) |
| Search repository | `internal/repository/media_repository.go` (`SearchInLibraries`) |
| Missing episodes repo | `internal/repository/tv_repository.go` (`GetMissingEpisodes`, `GetSeasonMissingEpisodes`) |
| Missing episodes handler | `internal/api/handlers_tv.go` (`handleMissingEpisodes`, `handleSeasonMissingEpisodes`) |
| Duplicate detection | `internal/fingerprint/fingerprint.go` |
| Duplicate handlers | `internal/api/handlers_duplicates.go` |
| Audio format detection | `internal/ffmpeg/ffprobe.go` (`GetAudioFormat`) |
| Frontend filter toolbar | `web/index.html` (`buildFilterToolbar`, `applyLibFilter`, `resetLibFilters`, `buildFilterQS`) |
| Frontend search | `web/index.html` (`loadSearchView`) |
| Frontend missing episodes | `web/index.html` (`loadShowView`, `loadSeasonView`) |
| CSS | `web/styles.css` (`.filter-toolbar`, `.missing-episodes-banner`, `.tag-warning`) |
| Migration | `migrations/024_advanced_filters.up.sql` |
