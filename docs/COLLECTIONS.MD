# CineVault Collections System

This document describes how CineVault organizes media into collections, including manual collections, smart collections, nested hierarchies, movie series, statistics, and template presets.

---

## Overview

CineVault has two independent grouping systems:

1. **Collections** — user-created groups (manual or smart) that can hold any media type and support nesting
2. **Movie Series** — auto-detected franchise groupings from TMDB collection data (e.g., "The Lord of the Rings Collection")

Collections are user-scoped (each user has their own). Movie series are library-scoped and shared across all users.

---

## Collection Types

### Manual Collections

Items are added individually by the user. Items can be movies, TV shows, albums, books, or edition groups. Each item has a custom sort position and optional notes.

### Smart Collections

Items are populated dynamically by evaluating a set of filter rules against the library. Smart collections re-evaluate every time they are viewed — no items are stored permanently. Rules are stored as a JSONB object on the collection.

---

## Collection Model

| Field | Type | Description |
|---|---|---|
| `id` | UUID | Primary key |
| `user_id` | UUID | Owner of the collection |
| `library_id` | UUID (nullable) | Optional library scope |
| `name` | string | Display name |
| `description` | string (nullable) | Optional description |
| `poster_path` | string (nullable) | Collection poster image |
| `collection_type` | string | `manual` or `smart` |
| `visibility` | string | `private`, `shared`, or `public` |
| `item_sort_mode` | string | How items are sorted (see Sort Modes) |
| `sort_position` | int | Position of this collection in lists |
| `rules` | JSONB (nullable) | Smart collection filter rules |
| `parent_collection_id` | UUID (nullable) | Parent collection for nesting |
| `created_at` | timestamp | Creation time |
| `updated_at` | timestamp | Last modification time |

Computed fields returned by the API:
- `item_count` — number of items in the collection
- `child_count` — number of child (sub) collections

---

## Collection Items

Each item in a manual collection links to one of five media types via nullable foreign keys:

| Field | Type | Description |
|---|---|---|
| `media_item_id` | UUID (nullable) | Movie, episode, or other media item |
| `edition_group_id` | UUID (nullable) | Edition group |
| `tv_show_id` | UUID (nullable) | TV show |
| `album_id` | UUID (nullable) | Music album |
| `book_id` | UUID (nullable) | Audiobook |
| `sort_position` | int | Custom ordering within the collection |
| `notes` | string (nullable) | User notes for this item |
| `added_at` | timestamp | When the item was added |
| `added_by` | UUID (nullable) | User who added the item |

When listing items, the API JOINs with the referenced tables to return display metadata: `title`, `year`, `poster_path`, `rating`, `duration_seconds`, `resolution`, and `media_type`.

---

## Sort Modes

Collections support six sort modes via the `item_sort_mode` field:

| Mode | Description |
|---|---|
| `custom` | Manual ordering by `sort_position` (default) |
| `title` | Alphabetical by title, ascending |
| `year` | By release year, descending |
| `rating` | By TMDB rating, descending |
| `added` | By date added to collection, most recent first |
| `duration` | By runtime, longest first |

The sort mode applies to manual collection items. Smart collections have their own `sort_by` and `sort_order` fields in the rules.

---

## Nested Collections

Collections support a single level of parent-child nesting via `parent_collection_id`. Any collection can be a parent, and child collections appear as a sub-grid within the parent's detail view.

When listing collections, top-level collections (no parent) are shown first. The library collections view filters to show only top-level entries.

Deleting a parent collection sets `parent_collection_id` to NULL on its children (ON DELETE SET NULL) — children are not deleted.

---

## Smart Collection Rules

Smart collections store their filter criteria as a JSONB `rules` object. All fields are optional. When multiple fields are set, they combine with AND logic. Within a field, values are OR.

### Rules Schema

```json
{
  "genres": ["action", "sci-fi"],
  "exclude_genres": ["horror"],
  "moods": ["intense", "adrenaline"],
  "year_from": 2000,
  "year_to": 2025,
  "min_rating": 7.0,
  "content_rating": ["PG-13", "R"],
  "media_types": ["movies"],
  "keywords": ["heist", "time travel"],
  "performers": ["Tom Hanks", "Meryl Streep"],
  "studios": ["A24", "Marvel Studios"],
  "min_duration": 90,
  "max_duration": 180,
  "added_within": 30,
  "released_within": 5,
  "sort_by": "rating",
  "sort_order": "desc",
  "max_results": 50
}
```

### Rules Reference

| Field | Type | Description |
|---|---|---|
| `genres` | string[] | Match items tagged with any of these genres (case-insensitive) |
| `exclude_genres` | string[] | Exclude items tagged with any of these genres (NOT filter) |
| `moods` | string[] | Match items tagged with any of these moods (case-insensitive) |
| `year_from` | int | Minimum release year (inclusive) |
| `year_to` | int | Maximum release year (inclusive) |
| `min_rating` | float | Minimum TMDB rating |
| `content_rating` | string[] | Allowed content ratings (NULL ratings are always included) |
| `media_types` | string[] | Restrict to specific media types (e.g., `["movies", "tv_shows"]`) |
| `keywords` | string[] | Match items whose TMDB keywords contain any of these (case-insensitive substring match) |
| `performers` | string[] | Match items featuring any of these actors or directors (case-insensitive exact name match via `performers` table) |
| `studios` | string[] | Match items produced by any of these studios (case-insensitive exact name match via `studios` table) |
| `min_duration` | int | Minimum runtime in minutes |
| `max_duration` | int | Maximum runtime in minutes |
| `added_within` | int | Only items added to the library within N days |
| `released_within` | int | Only items released within N years of the current year |
| `sort_by` | string | Sort field: `title`, `year`, `rating`, `added`, `duration`, `random` (default: `rating`) |
| `sort_order` | string | `asc` or `desc` (default: `desc`) |
| `max_results` | int | Maximum items returned (default 100, cap 500) |

### Evaluation

When a smart collection is evaluated (`GET /collections/{id}/evaluate`), the rules are translated into a SQL query with:
- JOINs against `media_tags`/`tags` for genre and mood filters
- JOINs against `media_performers`/`performers` for cast/director filters
- JOINs against `media_studios`/`studios` for studio filters
- WHERE clauses for year, rating, duration, content rating, keywords, and recency
- Non-default editions are always excluded
- Only items from enabled libraries are included
- Results are sorted by the specified `sort_by`/`sort_order` or defaulting to rating DESC, title ASC

---

## Collection Statistics

Each collection has a statistics endpoint returning aggregate data:

```json
{
  "total_items": 42,
  "total_runtime_seconds": 302400,
  "avg_rating": 7.8,
  "genres": [
    {"name": "Action", "count": 18},
    {"name": "Sci-Fi", "count": 12},
    {"name": "Drama", "count": 8}
  ]
}
```

Stats are computed from `collection_items` joined with `media_items` and `media_tags`. The genre breakdown returns the top 10 genres by count.

---

## Template Presets

The templates endpoint creates a set of pre-built smart collections for the user:

| Template | Rules |
|---|---|
| Recently Added | `added_within: 30`, sorted by added date descending, max 50 |
| Top Rated | `min_rating: 8.0`, sorted by rating descending, max 50 |
| Short Films | `max_duration: 60` (under 1 hour), sorted by duration ascending, max 50 |
| Classic Cinema | `year_from: 1920, year_to: 1979, min_rating: 7.0`, sorted by year ascending, max 100 |

Templates are created as private smart collections. Duplicate creation is handled gracefully (errors are silently skipped).

---

## Movie Series (Auto-Collections)

Movie series are a separate system from user collections. They represent franchise groupings detected from TMDB's `belongs_to_collection` data.

### How Auto-Detection Works

1. During metadata enrichment, TMDB returns `belongs_to_collection` with a collection ID and name
2. The scanner checks if a movie series with that TMDB collection ID already exists in the library
3. Falls back to name-based lookup if no ID match is found
4. Creates a new `movie_series` entry if neither exists, storing the TMDB collection ID in `external_ids`
5. Links the media item to the series with `sort_order` set to the release year

### Movie Series Model

| Field | Type | Description |
|---|---|---|
| `id` | UUID | Primary key |
| `library_id` | UUID | Library this series belongs to |
| `name` | string | Series/franchise name |
| `description` | string (nullable) | Description |
| `poster_path` | string (nullable) | Series poster |
| `backdrop_path` | string (nullable) | Series backdrop |
| `external_ids` | JSONB (nullable) | `{"tmdb_collection_id": "12345"}` |

Series items link movies to a series with a `sort_order` (typically the release year).

---

## API Endpoints

### Collections

| Method | Path | Auth | Description |
|---|---|------|-------------|
| GET | `/api/v1/collections` | User | List all collections for the user. Optional `?library_id=` filter. |
| POST | `/api/v1/collections` | User | Create a collection (manual or smart) |
| POST | `/api/v1/collections/templates` | User | Create default smart collection presets |
| GET | `/api/v1/collections/{id}` | User | Get collection details (includes items for manual collections) |
| PUT | `/api/v1/collections/{id}` | User | Update collection metadata, rules, parent, or sort mode |
| DELETE | `/api/v1/collections/{id}` | User | Delete a collection |
| GET | `/api/v1/collections/{id}/evaluate` | User | Evaluate smart collection rules and return matching items |
| GET | `/api/v1/collections/{id}/stats` | User | Get collection statistics (item count, runtime, avg rating, genres) |
| GET | `/api/v1/collections/{id}/children` | User | List child (sub) collections |
| POST | `/api/v1/collections/{id}/items` | User | Add a single item to a manual collection |
| POST | `/api/v1/collections/{id}/items/bulk` | User | Add multiple items in a single transaction |
| DELETE | `/api/v1/collections/{id}/items/{itemId}` | User | Remove an item from a collection |

### Movie Series

| Method | Path | Auth | Description |
|---|---|------|-------------|
| GET | `/api/v1/series` | User | List series in a library (requires `?library_id=`) |
| POST | `/api/v1/series` | Admin | Create a movie series |
| GET | `/api/v1/series/{id}` | User | Get series details with items |
| PUT | `/api/v1/series/{id}` | Admin | Update series metadata |
| DELETE | `/api/v1/series/{id}` | Admin | Delete a series |
| POST | `/api/v1/series/{id}/items` | Admin | Add a movie to a series |
| DELETE | `/api/v1/series/{id}/items/{itemId}` | Admin | Remove a movie from a series |
| GET | `/api/v1/media/{id}/series` | User | Check if a media item belongs to a series |

### Request Examples

**Create a manual collection:**

```json
POST /api/v1/collections
{
  "name": "Weekend Watchlist",
  "description": "Movies for the weekend",
  "visibility": "private",
  "collection_type": "manual",
  "item_sort_mode": "custom"
}
```

**Create a smart collection:**

```json
POST /api/v1/collections
{
  "name": "Recent Sci-Fi Hits",
  "collection_type": "smart",
  "visibility": "private",
  "rules": "{\"genres\":[\"science fiction\"],\"min_rating\":7.5,\"added_within\":90,\"sort_by\":\"rating\",\"sort_order\":\"desc\"}"
}
```

**Create a nested sub-collection:**

```json
POST /api/v1/collections
{
  "name": "Phase 4",
  "parent_collection_id": "uuid-of-parent-collection",
  "collection_type": "manual"
}
```

**Add item to collection:**

```json
POST /api/v1/collections/{id}/items
{
  "media_item_id": "uuid-of-media-item",
  "sort_position": 5,
  "notes": "Must watch"
}
```

**Bulk add items:**

```json
POST /api/v1/collections/{id}/items/bulk
[
  {"media_item_id": "uuid-1"},
  {"media_item_id": "uuid-2"},
  {"tv_show_id": "uuid-3"}
]
```

**Change sort mode:**

```json
PUT /api/v1/collections/{id}
{
  "item_sort_mode": "rating"
}
```

---

## Frontend Views

### Collections List (Global)

Accessed from the sidebar navigation. Shows all user collections grouped by top-level and nested. Each card displays the name, item count, visibility, type badge (Smart/Manual), and child count. Includes buttons for creating new collections and generating template presets.

### Collection Detail View

Clicking a collection card opens the detail view with:
- **Header** — poster, name, description, type and visibility badges, item count
- **Stats bar** — total items, total runtime, average rating, top genre chips
- **Sub-collections** — child collections rendered as a card grid (if any)
- **Sort controls** — dropdown to change sort mode (triggers re-render)
- **Items grid** — poster grid of media items using the standard media card layout. Smart collections evaluate rules on load; manual collections display joined items with metadata.

### Library Collections Tab

Within a library view, the Collections tab shows only collections scoped to that library. Only top-level collections are shown; clicking into one reveals its children.

### Create Collection Form

The create form supports both manual and smart types:
- **Manual** — name, description, visibility, sort mode
- **Smart** — all manual fields plus a rule builder UI with inputs for genres, exclude genres, moods, performers, studios, keywords, year range, min rating, duration range, added within, content rating, sort by, sort order, and max results. Includes a preview button that evaluates the rules and shows the match count.

Both types support setting a parent collection for nesting.

---

## Database Schema

### Tables

**collections** (migration 002, extended by 021 and 023):

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| user_id | UUID NOT NULL | — |
| library_id | UUID (nullable) | — |
| name | VARCHAR(500) NOT NULL | — |
| description | TEXT | — |
| poster_path | TEXT | — |
| collection_type | VARCHAR(20) NOT NULL | 'manual' |
| visibility | VARCHAR(20) NOT NULL | 'private' |
| item_sort_mode | VARCHAR(30) | 'custom' |
| sort_position | INTEGER | 0 |
| rules | JSONB | NULL (added in 021) |
| parent_collection_id | UUID (nullable) | NULL (added in 023) |
| created_at | TIMESTAMP | CURRENT_TIMESTAMP |
| updated_at | TIMESTAMP | CURRENT_TIMESTAMP |

**collection_items** (migration 002):

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| collection_id | UUID NOT NULL | — |
| media_item_id | UUID (nullable) | — |
| edition_group_id | UUID (nullable) | — |
| tv_show_id | UUID (nullable) | — |
| album_id | UUID (nullable) | — |
| book_id | UUID (nullable) | — |
| sort_position | INTEGER | 0 |
| notes | TEXT | — |
| added_at | TIMESTAMP | CURRENT_TIMESTAMP |
| added_by | UUID (nullable) | — |

**movie_series** (migration 015, extended by 017):

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| library_id | UUID NOT NULL | — |
| name | TEXT NOT NULL | — |
| description | TEXT | NULL (added in 017) |
| poster_path | TEXT | — |
| backdrop_path | TEXT | NULL (added in 017) |
| external_ids | JSONB | NULL (added in 017) |
| created_at | TIMESTAMPTZ | CURRENT_TIMESTAMP |
| updated_at | TIMESTAMPTZ | CURRENT_TIMESTAMP |

Unique constraint: `(library_id, name)`

**movie_series_items** (migration 015):

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| series_id | UUID NOT NULL | — |
| media_item_id | UUID NOT NULL | — |
| sort_order | INT NOT NULL | 0 |
| added_at | TIMESTAMPTZ | CURRENT_TIMESTAMP |

Unique constraint: `(series_id, media_item_id)`

### Indexes

- `idx_collections_parent` on `collections(parent_collection_id)`
- `idx_collection_items_collection` on `collection_items(collection_id)`
- `idx_movie_series_library` on `movie_series(library_id)`
- `idx_movie_series_items_series` on `movie_series_items(series_id)`
- `idx_movie_series_items_media` on `movie_series_items(media_item_id)`

---

## File Locations

| Component | Path |
|---|---|
| Models | `internal/models/models.go` (Collection, CollectionItem, SmartCollectionRules, CollectionStats) |
| Repository | `internal/repository/collection_repository.go` |
| API handlers | `internal/api/handlers_collections.go` |
| Series repository | `internal/repository/series_repository.go` |
| Series handlers | `internal/api/handlers_series.go` |
| Route registration | `internal/api/server.go` |
| Auto-detection | `internal/scanner/scanner.go` (autoCreateCollection) |
| Frontend (global) | `web/index.html` (loadCollectionsView, loadCollectionDetailView, showCreateCollection) |
| Frontend (library) | `web/index.html` (showLibraryCollections) |
| CSS | `web/styles.css` (collection-grid, collection-card, collection-detail-header, etc.) |
| Migration (base) | `migrations/002_phase2_schema.up.sql` |
| Migration (series) | `migrations/015_movie_series.up.sql` |
| Migration (smart rules) | `migrations/021_recommendations_moods_smart.up.sql` |
| Migration (nesting) | `migrations/023_enhanced_collections.up.sql` |
