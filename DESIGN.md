# CineVault — Complete Design Summary

> **Version:** 2.0  
> **Status:** Planning Phase  
> **Last Updated:** February 2025

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Technology Stack](#technology-stack)
3. [User System](#user-system)
4. [Supported Media Types](#supported-media-types)
5. [Data Architecture](#data-architecture)
6. [Edition System](#edition-system)
7. [Duplicate Detection System](#duplicate-detection-system)
8. [Sister Groups](#sister-groups)
9. [Collections System](#collections-system)
10. [Sort Order System](#sort-order-system)
11. [Playback System](#playback-system)
12. [Core Services / Modules](#core-services--modules)
13. [Streaming Approach](#streaming-approach)
14. [Web UI Structure](#web-ui-structure)
15. [Project Structure](#project-structure)
16. [Still To Design](#still-to-design)

---

## Project Overview

**CineVault** is a self-hosted, distributable media server combining features of Plex, Jellyfin, and StashApp. It supports multiple media types, multi-user access, on-the-fly transcoding, and advanced organization features including duplicate detection, sister file relationships, and edition grouping.

### Key Features

- Multi-user with role-based access control
- Support for 9 distinct media types
- On-the-fly transcoding with hardware acceleration
- Perceptual hash (pHash) duplicate detection
- Edition groups for multiple versions of same content
- Sister groups for related but separate content
- Collections for custom organization
- Drag-and-drop custom sorting throughout

---

## Technology Stack

| Component | Choice |
|-----------|--------|
| Backend Language | Go (with possible Rust utilities for heavy compute) |
| Database | PostgreSQL 16 |
| Cache / Job Queue | Redis + Asynq |
| Transcoding | FFmpeg with hardware auto-detection (NVENC, QSV, VAAPI) |
| Web UI | Embedded SPA (Svelte or React, TBD) |
| API | REST + WebSocket for real-time updates |
| Authentication | JWT + refresh tokens, role-based access control |
| Deployment | Docker with docker-compose |

---

## User System

### Roles & Permissions

| Role | Capabilities |
|------|--------------|
| Admin | Full system control — users, libraries, settings, everything |
| Manager | Add/edit/delete media metadata, tags, performers, run scans |
| User | Browse, stream, manage own watch history and preferences |
| Guest | Browse and stream only, no history, limited library access |

Libraries can have per-user visibility restrictions.

---

## Supported Media Types

| # | Type | Structure | Hierarchy |
|---|------|-----------|-----------|
| 1 | Movies | Standalone | None |
| 2 | Adult Movies | Standalone | None |
| 3 | TV Shows | Hierarchical | Show → Season → Episode |
| 4 | Music | Hierarchical | Artist → Album → Track |
| 5 | Music Videos | Linked | Links to Artist/Track |
| 6 | Home Videos | Date-based | Optional Albums/Events |
| 7 | Other Videos | Standalone | None |
| 8 | Images | Gallery-based | Gallery → Image |
| 9 | Audiobooks | Hierarchical | Author → Series (optional) → Book → Chapter |

Media types have a configurable sort order (drag-and-drop for admins).

---

## Data Architecture

### Strategy

- **Base `media_items` table** for all playable/viewable content with common fields
- **Type-specific parent tables** for hierarchical containers (shows, albums, books, galleries, artists, authors)
- **Edition groups** for grouping multiple versions of the same content
- **JSONB `metadata` column** for flexible type-specific attributes
- **Unified relationships** so tags, performers, and collections work across all types

### Hierarchical Parent Entities

| Entity | Contains | Parent |
|--------|----------|--------|
| tv_shows | seasons | library |
| tv_seasons | episodes (media_items) | tv_show |
| artists | albums | library |
| albums | tracks (media_items) | artist |
| authors | books, series | library |
| book_series | books | author |
| books | chapters (media_items) | author, series (optional) |
| image_galleries | images (media_items) | library |

### Core Media Item Fields

**Identity:**
- ID, library, media type, hierarchy references

**File Info:**
- Path, name, size, hash

**Common Metadata:**
- Title, sort title, original title, description, year, release date, duration, rating

**Media Specs:**
- Resolution, codec, container, bitrate, framerate, audio specs

**Ordering:**
- Sort position, episode/track/disc/chapter numbers

**Fingerprinting:**
- pHash, audio fingerprint, sister group reference

**Images:**
- Poster, thumbnail, preview sprite, animated preview

**Timestamps:**
- Added, updated, last scanned

**Flexible:**
- JSONB metadata and external IDs per type

---

## Edition System

### Concept

Editions allow multiple versions of the same content to display as **ONE entry** in the library, with a version picker on playback. This is distinct from sister groups, which keep items as separate entries.

### Supported Media Types for Editions

| Media Type | Supports Editions | Rationale |
|------------|-------------------|-----------|
| Movies | ✅ Yes | Director's cuts, remasters, theatrical vs extended |
| Adult Movies | ✅ Yes | Same as movies |
| TV Shows (Episodes) | ✅ Yes | Extended episodes, broadcast vs streaming cuts |
| Music (Albums) | ✅ Yes | Remastered, deluxe, anniversary editions |
| Music (Tracks) | ✅ Yes | Radio edit, explicit, acoustic, remixes |
| Music Videos | ✅ Yes | Explicit/clean, extended, director's cut |
| Audiobooks | ✅ Yes | Abridged, unabridged, different narrators |
| Home Videos | ❌ No | Not applicable |
| Images | ❌ No | Not applicable |
| Other Videos | ✅ Yes | User-defined editions |

### Edition Types by Media Type

#### Movies / Adult Movies / Other Videos

| Edition Type | Description |
|--------------|-------------|
| Normal | Standard theatrical release |
| Uncut | Uncensored/unedited version |
| Director's Edition | Director's preferred cut |
| Extended Edition | Additional scenes added |
| Theatrical | Original theatrical release |
| Remastered | Updated audio/video quality |
| Special Edition | Studio re-release with changes |
| Other | User-defined custom name |

#### TV Show Episodes

| Edition Type | Description |
|--------------|-------------|
| Broadcast | Original TV broadcast version |
| Extended | Extended/uncut version |
| Director's Cut | Director's preferred edit |
| Streaming | Streaming platform version |
| Syndicated | Edited for syndication |
| Other | User-defined custom name |

#### Music Albums

| Edition Type | Description |
|--------------|-------------|
| Original | Original release |
| Remastered | Remastered audio |
| Deluxe | Deluxe edition with bonus tracks |
| Anniversary | Anniversary re-release |
| Expanded | Expanded with additional tracks |
| Mono | Mono mix |
| Stereo | Stereo mix |
| Other | User-defined custom name |

#### Music Tracks

| Edition Type | Description |
|--------------|-------------|
| Original | Album version |
| Radio Edit | Shortened for radio |
| Explicit | Explicit/uncensored version |
| Clean | Censored version |
| Acoustic | Acoustic version |
| Live | Live recording |
| Remix | Remixed version |
| Instrumental | Instrumental version |
| Demo | Demo recording |
| Other | User-defined custom name |

#### Music Videos

| Edition Type | Description |
|--------------|-------------|
| Original | Standard music video |
| Explicit | Uncensored version |
| Clean | Censored version |
| Extended | Extended cut |
| Director's Cut | Director's preferred edit |
| Live | Live performance video |
| Lyric Video | Lyric video version |
| Other | User-defined custom name |

#### Audiobooks

| Edition Type | Description |
|--------------|-------------|
| Unabridged | Full unabridged reading |
| Abridged | Shortened version |
| Dramatized | Full cast dramatization |
| Author Narrated | Read by the author |
| Anniversary | Anniversary edition recording |
| Remastered | Remastered audio |
| Other | User-defined custom name |

### Quality Tiers

Automatically detected from media specs and prefixed to edition display name for 4K+ content:

#### Video Quality Tiers

| Tier | Detection Rule | Display Prefix |
|------|----------------|----------------|
| SD | height < 720 | (none) |
| HD | height ≥ 720 < 1080 | (none) |
| FHD | height ≥ 1080 < 2160 | (none) |
| 4K | height ≥ 2160 < 4320 | "4K " |
| 8K | height ≥ 4320 | "8K " |

#### Audio Quality Tiers (for Music/Audiobooks)

| Tier | Detection Rule | Display Suffix |
|------|----------------|----------------|
| Lossy | MP3, AAC, OGG < 320kbps | (none) |
| High | MP3/AAC 320kbps | (none) |
| Lossless | FLAC, ALAC, WAV 16-bit | "Lossless" |
| Hi-Res | FLAC/ALAC 24-bit or higher | "Hi-Res" |

### Display Name Examples

**Movies:**
- "Normal" (1080p file)
- "Director's Edition" (1080p file)
- "4K Normal" (4K file)
- "4K Remastered" (4K remastered file)

**Music Albums:**
- "Original"
- "Remastered"
- "Deluxe (Hi-Res)"
- "Anniversary (Lossless)"

**Audiobooks:**
- "Unabridged"
- "Abridged"
- "Author Narrated"
- "Dramatized"

### Edition Group Creation Methods

| Method | Description |
|--------|-------------|
| Via Duplicate Detection | When duplicates detected, user chooses "Create Edition Group" |
| Manual Creation | User selects multiple items and groups them manually |
| Single Item Start | User creates edition group from one item, adds others later |
| During Scan | Auto-suggest based on filename parsing (user confirms) |

### Manual Edition Group Creation Flow

**From Library View:**
1. User enters "Select Mode"
2. User selects 2+ items of same type
3. User clicks "Create Edition Group"
4. UI prompts for group title, year, and edition assignment for each item
5. Items merged into single visible entry with version picker

**From Item Detail Page:**
1. User views a single media item
2. User clicks "Create Edition Group" or "Add to Edition Group"
3. Option A: Create new group (item becomes first version)
4. Option B: Add to existing group (search/browse existing groups)
5. User assigns edition type and confirms

### Edition Group Management

**Group-Level Actions:**
- Edit shared metadata (title, year, description, poster)
- Set default version
- Reorder versions (drag-and-drop)
- Delete entire group (versions become standalone items)
- Merge with another edition group

**Version-Level Actions:**
- Change edition type
- Set custom edition name
- Set as default
- Add notes
- Remove from group (becomes standalone)
- Replace file (keep metadata, swap underlying file)
- Delete version entirely

---

## Duplicate Detection System

### Core Concept

Perceptual hashing (pHash) computed from video/image keyframes, with Hamming distance comparison for similarity detection. Audio fingerprinting for music/audiobooks.

### User Actions on Detected Duplicates

| Action | Result | Library Display |
|--------|--------|-----------------|
| Merge | Combine metadata into primary, delete secondary | Single item |
| Delete | Remove one file entirely | Remaining item only |
| Ignore | Never suggest this pair again | Both items separate |
| Keep as Sisters | Link in sister group, acknowledged related | Both items separate (linked) |
| Create Edition Group | Group as versions of same content | ONE item with version picker |

### Duplicate Detection Logic

For each potential duplicate pair:

1. Compute similarity (pHash for video/images, audio fingerprint for audio)

2. **Exclude** if:
   - Already in same sister group
   - Already in same edition group
   - Pair exists in ignore list
   - Decision already made for this pair

3. **Include and flag** if:
   - New file matches existing edition group → offer "Add to Edition Group"
   - New file matches sister group member → offer all options
   - Cross-group match detected → alert user

4. **Suggest edition grouping** for:
   - Movies, Adult Movies
   - Music Albums, Music Tracks
   - Music Videos
   - Audiobooks
   - TV Episodes
   - Other Videos

### Audit Trail

All duplicate decisions are logged with:
- Which files were involved
- What decision was made
- Who made it and when
- Which file survived (for merges)
- Which sister group was used (for splits)

---

## Sister Groups

### Purpose

Group files that are similar/related but should remain as **separate entries** in the library. Unlike editions, sister files are individually visible and playable.

### Use Cases

- Same event from different camera angles
- Similar but distinct content (two versions of same concert)
- User wants both visible but linked for reference

### Sister Group Features

- Named and manageable by users
- Optional notes/description
- Files never flagged as duplicates of each other
- New files CAN still be detected as matching sisters (user prompted to add)
- Cross-group duplicates detected if groups start to overlap

### Sister Group Actions

- Create new group
- Add to existing group
- Rename group
- Remove item from group (becomes independent)
- Merge groups together
- Delete group (items become independent, not deleted)

---

## Collections System

### Collection Types

| Type | Behavior |
|------|----------|
| Manual | User explicitly adds/removes items |
| Smart | Rule-based auto-population (future feature) |

### Collection Features

- Can contain individual media items OR parent entities (shows, albums, books)
- Can contain edition groups (displays as single item)
- Cross-library collections supported
- Per-collection sort mode (custom drag-and-drop, or automatic)
- Visibility: private, shared with specific users, or public
- Optional notes per item within a collection
- Shared collections have permission levels (view, edit, admin)

### Smart Collections (Future)

Rule-based collections that auto-populate based on criteria:
- Media type
- Genre/tags
- Resolution/quality
- Date added/released
- Watch status
- Rating
- And/or/not logic

---

## Sort Order System

### Sort Position Strategy

Every sortable entity has a `sort_position` field:
- Media types (sidebar ordering)
- Hierarchical parents (shows, artists, albums, books, galleries)
- Media items (within their parent context)
- Edition group versions (ordering in version picker)
- Collections (ordering of collections list)
- Collection items (ordering within a collection)

### Custom Sort Behavior

- Drag-and-drop reordering in UI
- Position calculated using gaps (e.g., 1000, 2000, 3000)
- When item dragged between two others, new position = midpoint
- Periodic rebalancing job if gaps get too small

### Available Sort Modes

| Mode | Description |
|------|-------------|
| Custom | User-defined drag-and-drop order |
| Title | Alphabetical by title |
| Sort Title | Alphabetical by sort title ("The Matrix" → "Matrix") |
| Date Added | When added to library |
| Release Date | Original release date |
| Year | Release year |
| Duration | Length |
| Rating | User rating |
| Random | Randomized order |
| File Size | Size on disk |

### Per-User Sort Preferences

Users can save sort configurations per context:
- Library view
- Collection view
- Show seasons/episodes
- Album tracks
- Version picker order

---

## Playback System

### Edition Playback Flow

When user plays content that has multiple editions:

1. Check user preference for edition playback mode
2. If "always ask" or no preference saved → show version picker
3. If preference set → auto-select and play

### User Playback Preferences

| Preference | Behavior |
|------------|----------|
| Always Ask | Show version picker every time |
| Play Default | Auto-play the version marked as default |
| Highest Quality | Auto-play highest resolution/bitrate version |
| Lowest Quality | Auto-play lowest (for bandwidth saving) |
| Last Played | Auto-play whichever version user last watched |

### Per-Item Overrides

Users can set "Remember my choice" for specific edition groups to override global preference.

### Watch History with Editions

- Tracks exact file/version watched
- Also tracks edition group for unified "Continue Watching"
- If user switches versions mid-watch, prompt to continue from same timestamp (if durations are close)

---

## Core Services / Modules

| Module | Responsibility |
|--------|----------------|
| Scanner | Watch folders, detect changes, extract basic metadata via ffprobe |
| Fingerprinter | Generate pHash, audio fingerprints, detect duplicates |
| Preview Generator | Thumbnail grids, sprite sheets, animated previews, chapter thumbnails |
| Scene Detector | FFmpeg scene detection, store chapter markers, allow manual adjustment |
| Transcoder | On-the-fly HLS transcoding, hardware acceleration, segment caching |
| Metadata Scraper | Pluggable sources (TMDB, TVDB, MusicBrainz, StashDB, etc.) |
| API Layer | REST + WebSocket, auth, pagination, filtering |
| Job Queue | Persistent background task processing via Redis/Asynq |

---

## Streaming Approach

- **On-the-fly transcoding** (not pre-transcoded)
- Client requests stream → server checks device compatibility
- If direct play compatible → serve file directly
- Otherwise → FFmpeg transcodes to HLS in real-time
- Hardware acceleration auto-detected and prioritized: NVENC → QSV → VAAPI → software
- Segments cached temporarily with TTL cleanup

### Streaming API Endpoints

```
GET /api/v1/stream/{media_id}/master.m3u8
    → Returns HLS manifest with quality options

GET /api/v1/stream/{media_id}/{quality}/segment_{n}.ts
    → Returns transcoded segment (generated on demand)

GET /api/v1/stream/{media_id}/direct
    → Direct file stream (if compatible)
```

---

## Web UI Structure

| Route | Purpose |
|-------|---------|
| `/` | Dashboard — continue watching, recently added |
| `/libraries` | Library grid |
| `/library/{id}` | Media browser with filters |
| `/media/{id}` | Detail page + player |
| `/media/{id}/versions` | Edition version management (if applicable) |
| `/performers` | Performer grid with search |
| `/performer/{id}` | Performer detail + linked media |
| `/tags` | Tag cloud / hierarchy |
| `/collections` | User's collections |
| `/collection/{id}` | Collection detail |
| `/duplicates` | Duplicate review queue |
| `/sisters` | Sister group management |
| `/editions` | Edition group management |
| `/settings` | User preferences |
| `/admin` | User management, libraries, jobs, logs |

---

## Project Structure

```
cinevault/
├── cmd/cinevault/          → Main entry point
├── internal/
│   ├── api/                → HTTP handlers, middleware, routes
│   ├── auth/               → JWT, sessions, RBAC
│   ├── config/             → Config loading
│   ├── db/                 → PostgreSQL, migrations, queries
│   ├── ffmpeg/             → FFmpeg/ffprobe wrappers
│   ├── fingerprint/        → pHash, audio fingerprint computation
│   ├── jobs/               → Background job definitions
│   ├── library/            → Scanner, watcher
│   ├── media/              → Media service logic
│   ├── editions/           → Edition group logic
│   ├── duplicates/         → Duplicate detection logic
│   ├── stream/             → HLS generation, transcoding
│   └── models/             → Domain types
├── web/                    → Embedded SvelteKit/React app
├── migrations/             → SQL migration files
├── docker/                 → Dockerfile, docker-compose.yml
└── README.md
```

---

## Relationship Summary

| Relationship | Library Display | Playback | Creation Methods |
|--------------|-----------------|----------|------------------|
| **None** | Individual entries | Direct play | Default state |
| **Ignored Duplicate** | Individual entries | Direct play | Via duplicate detection |
| **Sister Group** | Individual entries (linked) | Direct play each | Via duplicate detection, manual |
| **Edition Group** | ONE entry | Version picker | Via duplicate detection, manual, single-item start |

---

## Still To Design

1. **Performers / People** — unified entity for actors, adult performers, music artists, narrators
2. **Tags / Genres** — hierarchical tagging system across all media types
3. **Studios / Labels / Publishers** — production entities
4. **Library Configuration** — how libraries map to media types, scan settings
5. **Full API Contract** — OpenAPI spec for all endpoints
6. **Database Migrations** — complete SQL schema
7. **Job Definitions** — all background tasks and their payloads
8. **Streaming Protocol Details** — HLS segment duration, caching strategy, adaptive bitrate profiles
9. **Metadata Scraper Architecture** — plugin system for different sources
10. **Search / Filtering** — full-text search, advanced filters, saved searches
11. **Notifications** — scan complete, new content, transcode finished
12. **Mobile App (iOS)** — API integration, offline sync considerations

---

## Data Model Diagrams

### Edition Groups

```
┌─────────────────────────┐
│    edition_groups       │  ← The "canonical" entry users see
├─────────────────────────┤
│ id                      │
│ library_id              │
│ title                   │
│ sort_title              │
│ year                    │
│ description             │
│ poster_path             │
│ backdrop_path           │
│ external_ids            │ JSONB
│ metadata                │ JSONB
│ default_edition_id      │ FK → edition_items
│ created_at              │
│ updated_at              │
└───────────┬─────────────┘
            │ 1:N
            ▼
┌─────────────────────────┐
│    edition_items        │  ← Individual files/versions
├─────────────────────────┤
│ id                      │
│ edition_group_id        │ FK
│ media_item_id           │ FK → media_items
│ edition_type_id         │ FK → edition_types
│ custom_edition_name     │ nullable
│ quality_tier            │ ENUM
│ display_name            │ computed
│ is_default              │ BOOLEAN
│ sort_order              │ INT
│ notes                   │ TEXT
│ added_at                │
│ added_by                │ FK → users
└─────────────────────────┘
```

### Sister Groups

```
┌───────────────────┐
│   sister_groups   │
├───────────────────┤
│ id                │
│ name              │
│ notes             │
│ created_at        │
│ created_by        │
└────────┬──────────┘
         │ 1:N
         ▼
┌───────────────────┐
│   media_items     │
├───────────────────┤
│ ...               │
│ sister_group_id   │ FK, nullable
│ ...               │
└───────────────────┘
```

### Duplicate Decisions

```
┌────────────────────────┐
│  duplicate_decisions   │
├────────────────────────┤
│ id                     │
│ media_id_a             │ FK
│ media_id_b             │ FK
│ decision               │ ENUM: merged, deleted, ignored, 
│                        │       split_as_sister, edition_grouped
│ primary_media_id       │ FK, nullable
│ sister_group_id        │ FK, nullable
│ edition_group_id       │ FK, nullable
│ decided_by             │ FK → users
│ decided_at             │
│ notes                  │
└────────────────────────┘
```

### Collections

```
┌───────────────────────┐
│     collections       │
├───────────────────────┤
│ id                    │
│ library_id            │ FK, nullable
│ user_id               │ FK
│ name                  │
│ description           │
│ poster_path           │
│ collection_type       │ ENUM: manual, smart
│ visibility            │ ENUM: private, shared, public
│ sort_position         │
│ item_sort_mode        │
│ created_at            │
│ updated_at            │
└───────────┬───────────┘
            │ 1:N
            ▼
┌───────────────────────┐
│   collection_items    │
├───────────────────────┤
│ id                    │
│ collection_id         │ FK
│ media_item_id         │ FK, nullable
│ edition_group_id      │ FK, nullable
│ tv_show_id            │ FK, nullable
│ album_id              │ FK, nullable
│ book_id               │ FK, nullable
│ sort_position         │
│ added_at              │
│ added_by              │ FK
│ notes                 │
└───────────────────────┘
```

---

## UI Mockups (ASCII)

### Duplicate Detection Dialog

```
┌─────────────────────────────────────────────────────────────────┐
│  POTENTIAL DUPLICATE DETECTED                                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────┐      ┌─────────────────────────┐  │
│  │  Aliens (1986)          │      │  Aliens (1986)          │  │
│  │                         │      │                         │  │
│  │  📁 /movies/Aliens.mkv  │      │  📁 /4k/Aliens.mkv      │  │
│  │  1920 × 1080 (FHD)      │      │  3840 × 2160 (4K)       │  │
│  │  H.264 • 8.2 GB         │      │  HEVC • 42.1 GB         │  │
│  │  2h 34m                 │      │  2h 34m                 │  │
│  │                         │      │                         │  │
│  └─────────────────────────┘      └─────────────────────────┘  │
│                                                                 │
│  Similarity: 97%                                                │
│                                                                 │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌─────────┐ ┌─────────────┐  │
│  │ Merge  │ │ Delete │ │ Ignore │ │ Keep as │ │ Create      │  │
│  │        │ │        │ │        │ │ Sisters │ │ Edition     │  │
│  └────────┘ └────────┘ └────────┘ └─────────┘ └─────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Version Picker on Playback

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│                        Select Version                           │
│                                                                 │
│   ┌─────────────────────────────────────────────────────────┐  │
│   │  ● Normal                     1080p • H.264 • 8.2 GB    │  │
│   ├─────────────────────────────────────────────────────────┤  │
│   │  ○ Director's Edition         1080p • H.264 • 9.1 GB    │  │
│   ├─────────────────────────────────────────────────────────┤  │
│   │  ○ 4K Normal                  4K • HEVC • 42.1 GB       │  │
│   └─────────────────────────────────────────────────────────┘  │
│                                                                 │
│   ☐ Remember my choice for this movie                          │
│                                                                 │
│                    ┌──────────────────────┐                    │
│                    │       ▶  PLAY        │                    │
│                    └──────────────────────┘                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Edition Group Management

```
┌─────────────────────────────────────────────────────────────────┐
│  ALIENS (1986) — VERSIONS                                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ ⋮⋮  Normal              1080p • 8.2 GB   ★ DEFAULT [Edit]│   │
│  ├─────────────────────────────────────────────────────────┤   │
│  │ ⋮⋮  Director's Edition  1080p • 9.1 GB           [Edit] │   │
│  ├─────────────────────────────────────────────────────────┤   │
│  │ ⋮⋮  4K Normal           4K • 42.1 GB             [Edit] │   │
│  └─────────────────────────────────────────────────────────┘   │
│       ↑                                                        │
│   Drag handle for custom sort order                            │
│                                                                 │
│  ┌─────────────────────┐                                       │
│  │  + Add Version      │                                       │
│  └─────────────────────┘                                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

*End of Design Document*
