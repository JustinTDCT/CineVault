# CineVault Metadata Architecture

This document describes how CineVault gathers, enriches, caches, and applies metadata to media items.

---

## Overview

CineVault uses a **cache-first architecture** with multi-source aggregation. Metadata flows through this priority chain:

1. **Local sources** (NFO files, inline provider IDs) — highest trust, no network required
2. **CineVault Cache Server** — centralized, pre-enriched from all sources in a single call
3. **Direct API fallback** — used only when the cache server is unreachable

The cache server is the primary metadata source. On a miss, it fetches from external APIs itself, enriches the result with ratings and artwork, stores it, and returns the fully-enriched entry. The app's direct API fallback is a safety net, not the primary flow.

---

## Data Sources

### TMDB (The Movie Database)
- **Used for**: Movies, TV shows
- **Provides**: Title, description, tagline, year, release date, original language, production country, poster, backdrop, genres, content rating, runtime, cast/crew, trailer URL, IMDB ID, TVDB ID (for TV), collection membership
- **Access**: Via cache server (primary) or direct API (fallback)

### OMDb (Open Movie Database)
- **Used for**: Movies, TV shows (requires IMDB ID)
- **Provides**: IMDB rating, Rotten Tomatoes critic score, Rotten Tomatoes audience score
- **Access**: Enrichment applied by cache server inline during lookup, or via background worker for entries missing ratings

### fanart.tv
- **Used for**: Movies (by TMDB ID), TV shows (by TVDB ID)
- **Provides**: HD logos/clearlogos, banners, clearart
- **Access**: Enrichment applied by cache server inline during lookup, or via background worker. Also fetched directly by CineVault app if `fanart_api_key` is configured.

### TVDB (TheTVDB)
- **Used for**: TV shows
- **Provides**: TVDB ID cross-reference from TMDB external IDs. TVDB ID enables fanart.tv lookups for TV content.

### MusicBrainz
- **Used for**: Music, music videos
- **Provides**: Artist, album, track metadata, release year, cover art (via Cover Art Archive)
- **Access**: Via cache server or direct fallback

### OpenLibrary
- **Used for**: Audiobooks
- **Provides**: Title, author, year, description, cover image, ISBN
- **Access**: Via cache server or direct fallback

### PornDB (ThePornDB)
- **Used for**: Adult content
- **Provides**: Title, description, performers, poster, backdrop. Merged with TMDB data when both have results.
- **Access**: Via cache server

### NFO Files (Local)
- **Used for**: Any media type
- **Provides**: Full Kodi-compatible metadata including title, tagline, plot, year, runtime, content rating, country, trailer URL, ratings, genres, studios, cast/crew, artwork references, and provider IDs (IMDB, TMDB, TVDB)
- **Access**: Read from sidecar `.nfo` files during scan. Can also be written/exported after metadata is resolved.

---

## Cache Server Architecture

The CineVault Cache Server (`cache.cine-vault.tv:8090`) is a centralized metadata aggregation service that serves as the primary metadata source for all CineVault instances.

### How It Works

```
CineVault App                    Cache Server                     External APIs
    |                                |                                |
    |-- POST /api/v1/lookup -------->|                                |
    |                                |-- Check Redis hot cache        |
    |                                |-- Check PostgreSQL             |
    |                                |                                |
    |                                |   (cache miss)                 |
    |                                |-- Fetch from TMDB ------------>|
    |                                |-- Enrich with OMDb ----------->|
    |                                |-- Enrich with fanart.tv ------>|
    |                                |-- Store in DB + Redis          |
    |                                |                                |
    |<-- Full enriched entry --------|                                |
    |                                |                                |
    |-- Apply to media item          |                                |
```

### What the Cache Server Returns

A single lookup returns all of the following (when available):

| Field | Source |
|---|---|
| Title, original title, sort title | TMDB |
| Description | TMDB (multi-source array with PornDB/MusicBrainz alternatives) |
| Year, release date | TMDB |
| Tagline | TMDB |
| Original language | TMDB |
| Country | TMDB (production country for movies, origin country for TV) |
| Trailer URL | TMDB (YouTube, prefers official trailers) |
| Poster URL | TMDB (multi-source array) |
| Backdrop URL | TMDB (multi-source array) |
| Logo URL | fanart.tv (HD logo preferred, English preferred) |
| Banner URL | fanart.tv |
| Content rating | TMDB (US certification) |
| Runtime | TMDB |
| Genres | TMDB (merged across sources) |
| Cast/crew | TMDB (top 20 cast + key crew) |
| IMDB ID | TMDB |
| TVDB ID | TMDB external IDs (TV shows only) |
| Collection ID/name | TMDB `belongs_to_collection` (movies only) |
| IMDB rating | OMDb |
| RT critic score | OMDb |
| RT audience score | OMDb |

### Cache Server Features

- **Multi-source aggregation**: Posters, backdrops, and descriptions stored as JSON arrays with source attribution
- **Concurrent miss deduplication**: If multiple instances look up the same title simultaneously, only one TMDB request is made
- **Background enrichment**: OMDb ratings and fanart.tv artwork are enriched in the background for any entries that missed inline enrichment
- **Metadata revalidation**: Stale entries are periodically re-fetched from TMDB to pick up updates
- **Image caching**: TMDB posters and backdrops are downloaded and served locally
- **File hash lookups**: Content-addressed matching by file hash for instant identification

### Cache Server Settings

| Setting | Description |
|---|---|
| `cache_server_enabled` | Enable/disable cache server integration (default: enabled) |
| `cache_server_api_key` | Auto-registered on first use, stored in settings |

---

## Metadata Application Flow

### During Initial Scan

```
For each new media file:
  1. Parse filename → extract title, year, edition, season/episode
  2. Check for inline provider IDs ([tmdbid=X], [imdbid=X], [tvdbid=X])
     → If found: direct TMDB lookup by ID, skip fuzzy matching
  3. Check for NFO sidecar
     → If NFO has provider ID: direct lookup
     → If NFO has full metadata + lockdata: apply directly, skip external lookup
  4. Detect local artwork (if prefer_local_artwork enabled)
  5. Query cache server (if enabled)
     → Cache server fetches from TMDB + OMDb + fanart.tv on miss
     → Returns fully-enriched entry
  6. Fallback: direct TMDB/MusicBrainz/OpenLibrary search
  7. Apply metadata:
     - Title, year, description, content rating, tagline, language, country, trailer
     - Download poster
     - Link genres as tags
     - Apply ratings (IMDB, RT)
     - Import cast/crew as performers
     - Store external IDs (TMDB, IMDB, TVDB, etc.)
     - Auto-create movie collection from TMDB collection data
     - Fetch fanart.tv logos/banners
  8. Export NFO (if nfo_export enabled)
  9. Contribute result to cache server (background, helps other instances)
```

### During Re-Enrichment (Automatic — runs after scan)

Library re-enrichment runs automatically at the end of a scan for existing items. It focuses on **filling gaps** without clearing existing data:
- Replaces generated screenshot posters with TMDB posters
- Adds missing genres, ratings, cast/crew
- Applies extended metadata (tagline, country, trailer, etc.)
- Creates auto-collections from TMDB collection data
- Skips items that are fully locked (`metadata_locked`, `locked_fields = ["*"]`)
- All updates respect per-field locks

### During Metadata Refresh (User-Triggered)

Metadata refresh (`POST /libraries/{id}/refresh-metadata`) is a **destructive re-fetch**: it clears all existing metadata for unlocked items, then re-queries all sources from scratch.

```
For each item in the library:
  1. Skip fully locked items (metadata_locked or locked_fields = ["*"])
  2. Clear all metadata fields (respects per-field locks — locked values preserved)
  3. Remove genre tags and performer links (unless those fields are locked)
  4. Re-query cache server or direct scrapers
  5. Apply metadata using lock-aware methods
  6. Apply extended metadata, genres, credits, artwork from cache or TMDB enrichment
  7. If no match found: generate screenshot poster as fallback
```

This is different from re-enrichment, which only fills gaps. Refresh is intended for cases where metadata needs to be corrected or updated from scratch.

### During Auto-Match (User-Triggered)

Auto-match (`POST /libraries/{id}/auto-match`) searches for metadata for unlocked items that may not have been matched yet. It uses the same cache-first flow as the initial scan but operates on existing items. All updates respect per-field locks.

### During Manual Match

When a user manually identifies a media item:
1. Cache server is queried for supplementary data (ratings, cast/crew, genres)
2. If cache has the entry: ratings and cast/crew are applied from cache
3. If not: direct TMDB details fetch, then contribute to cache

---

## Per-Field Metadata Locking

CineVault supports locking metadata at two levels:

- **Full lock** (`metadata_locked = true` or `locked_fields = ["*"]`): All fields are locked. Auto-match and re-enrichment skip this item entirely.
- **Per-field lock** (`locked_fields = ["title", "description", ...]`): Only specified fields are protected from automatic updates.

When a user manually edits metadata, `metadata_locked` is set to `true` to prevent the next scan from overwriting their changes.

NFO files can also lock metadata via `<lockdata>true</lockdata>`, which applies the NFO data and prevents external lookups from overwriting it.

---

## External IDs

Each media item stores a JSON object of external identifiers:

```json
{
  "source": "tmdb",
  "cache_server": true,
  "tmdb_id": "550",
  "imdb_id": "tt0137523",
  "tvdb_id": "73255"
}
```

Non-TMDB sources use their own ID keys:
- `tpdb_id` — PornDB scene ID
- `musicbrainz_id` — MusicBrainz release ID
- `openlibrary_id` — OpenLibrary work key

The `cache_server` flag indicates whether the metadata was sourced from the cache server or fetched directly. Note: `tvdb_id` is populated from cache server results or TMDB external IDs; it is not available from direct title matching alone.

---

## Auto-Collections

When TMDB returns `belongs_to_collection` data for a movie (e.g., "The Lord of the Rings Collection"), CineVault automatically:

1. Checks if a movie series with that TMDB collection ID already exists
2. Checks if a series with the same name exists (in case it was manually created)
3. Creates a new movie series if neither exists, storing the TMDB collection ID in `external_ids`
4. Links the media item to the series

This works from both the cache server path and direct TMDB path.

---

## Metadata Fields Stored Per Media Item

| Field | Source | Description |
|---|---|---|
| `title` | All | Display title |
| `sort_title` | Generated | Title without leading articles (The, A, An) |
| `original_title` | TMDB | Title in the original language |
| `description` | TMDB/NFO | Plot summary |
| `tagline` | TMDB/NFO | Short tagline |
| `year` | Filename/TMDB/NFO | Release year |
| `release_date` | TMDB | Full release date |
| `content_rating` | TMDB/NFO | MPAA rating (PG-13, R, etc.) |
| `original_language` | TMDB | ISO language code (en, fr, etc.) |
| `country` | TMDB/NFO | Production/origin country |
| `trailer_url` | TMDB/NFO | YouTube trailer link |
| `poster_path` | TMDB/Local/fanart.tv | Poster image path |
| `backdrop_path` | TMDB/Local | Backdrop/fanart image path |
| `logo_path` | fanart.tv/Local | Clearlogo image path |
| `rating` | TMDB | TMDB vote average |
| `imdb_rating` | OMDb | IMDB user rating |
| `rt_rating` | OMDb | Rotten Tomatoes critic score |
| `audience_score` | OMDb | Rotten Tomatoes audience score |
| `runtime` | FFprobe/TMDB | Duration in seconds (from FFprobe) or minutes (from TMDB) |
| `genres` | TMDB/NFO | Linked as tags |
| `cast/crew` | TMDB/NFO | Linked as performers with roles |
| `external_ids` | All | JSON object of provider IDs |
| `locked_fields` | User/NFO | Array of field names protected from auto-update |

---

## Configuration Settings

| Setting | Description |
|---|---|
| `cache_server_enabled` | Use the CineVault Cache Server for lookups (default: enabled) |
| `omdb_api_key` | OMDb API key for ratings enrichment |
| `fanart_api_key` | fanart.tv API key for logos and banners |
| `tvdb_api_key` | TVDB API key (optional, TVDB IDs come from TMDB external IDs) |

### Per-Library Settings

| Setting | Default | Description |
|---|---|---|
| `retrieve_metadata` | `true` | Enable automatic metadata retrieval from external sources on scan |
| `nfo_import` | `false` | Read Kodi-compatible NFO files during scan |
| `nfo_export` | `false` | Write NFO files after metadata is resolved |
| `prefer_local_artwork` | `true` | Use local artwork files over downloaded artwork |
