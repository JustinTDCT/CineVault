# CineVault Recommendation System

This document describes how CineVault generates personalized recommendations, content-similarity rows, mood tagging, and smart collections.

---

## Overview

CineVault's recommendation engine is built on three pillars:

1. **Personalized Recommendations** — a weighted scoring algorithm that learns from watch history
2. **"Because You Watched" Rows** — content-similarity suggestions grouped by recently completed titles
3. **Smart Collections** — user-defined rule-based dynamic playlists

All three are powered by the same underlying data: watch history, genre/mood tags, cast/crew links, and TMDB keywords.

---

## Personalized Recommendations

### Endpoint

`GET /api/v1/recommendations` — returns up to 20 unwatched items ranked by relevance.

### How It Works

The algorithm builds a per-user affinity profile from their watch history, then scores every unwatched item in accessible libraries against that profile.

```
Watch History → Genre Affinity (recency-weighted) ┐
                                                    ├─ Score → Rank → Return top N
Watch History → Cast/Director Affinity ────────────┘
```

### Scoring Components

#### 1. Genre Affinity (Recency-Weighted)

For each genre the user has watched, an affinity score is calculated:

```
affinity = SUM(completion_weight / (1 + days_since_watch / 30))
```

- **Completion weight**: 1.0 for completed items, 0.5 for items watched past 50%, 0.0 for abandoned items
- **Recency decay**: `1 / (1 + days/30)` — an item watched today scores ~1.0, an item watched 30 days ago scores ~0.5, an item watched 90 days ago scores ~0.25
- The top 15 genres by affinity are used for scoring

This means a user who binge-watched sci-fi films last week will see more sci-fi recommendations than someone who watched sci-fi months ago.

#### 2. Cast/Director Affinity

The algorithm identifies the user's top 20 most-watched actors and directors (from completed items only). Each match between a candidate item and a preferred performer adds 0.5 to the score.

#### 3. Final Score

```
total_score = genre_score + performer_score
```

Items are ranked by `total_score DESC`, then by `rating DESC` as a tiebreaker. Only items with `total_score > 0` are returned.

### Filters Applied

| Filter | Description |
|---|---|
| Rating floor | Items with a TMDB rating below 5.0 are excluded (NULL ratings are allowed) |
| Skip detection | Items where the user started watching but abandoned before 10% progress are excluded |
| Already watched | Any item with watch history for this user is excluded |
| Edition dedup | Non-default editions (e.g., Director's Cut when Theatrical is default) are excluded |
| Parental controls | If the user has `max_content_rating` set, only items at or below that rating appear |
| Library access | Only items from enabled libraries the user has permission to access |
| Homepage inclusion | Only items from libraries with `include_in_homepage = true` |

---

## "Because You Watched" Rows

### Endpoint

`GET /api/v1/recommendations/because-you-watched` — returns up to 5 grouped rows, each with up to 8 similar items.

### How It Works

1. **Source selection**: The user's 5 most recently completed items are selected as source titles
2. **Similarity scoring**: For each source title, the system finds unwatched items that share genres, actors, or directors
3. **Minimum threshold**: A row is only shown if at least 2 similar items are found

### Similarity Algorithm

For a given source item, every candidate item is scored:

```
similarity = shared_genre_count + (shared_performer_count * 2)
```

- Each shared genre adds 1 point
- Each shared actor or director adds 2 points (cast/crew overlap is a stronger signal than genre)
- Candidates are ranked by similarity score, then by rating

### Filters

The same filters from personalized recommendations apply: rating floor (5.0+), skip detection, already-watched exclusion, edition dedup, parental controls, and library access.

### Frontend Display

On the home page, each row renders as:

```
Because You Watched [Source Title]
  [Card] [Card] [Card] [Card] ...
```

These rows appear between "Recommended For You" and "Recently Added".

---

## Mood Tags

### What They Are

Mood tags are a tag category (`mood`) that describes the emotional tone of content. Unlike genres (Action, Comedy, Drama), moods capture how content *feels* (Intense, Funny, Scary).

### Available Moods

| Mood | Derived From Keywords |
|---|---|
| Feel-Good | feel-good, heartwarming, uplifting, inspirational, friendship, underdog, coming of age, road trip, buddy |
| Dark | dark, gritty, dystopia, neo-noir, nihilism, bleak, post-apocalyptic, apocalypse |
| Intense | suspense, tension, psychological, thriller, conspiracy, paranoia, chase, hostage, cat and mouse, survival |
| Romantic | romance, love, love triangle, forbidden love, wedding, first love, soulmates |
| Funny | comedy, slapstick, satire, parody, absurd, dark comedy, farce, quirky, witty |
| Emotional | tearjerker, tragedy, grief, loss, death, dying, terminal illness, loss of loved one |
| Mind-Bending | mind-bending, twist ending, time travel, time loop, alternate reality, dream, parallel universe, nonlinear timeline, surreal, hallucination |
| Scary | horror, haunted house, ghost, demon, slasher, paranormal, zombie, vampire, werewolf, monster, serial killer, possession, supernatural, occult |
| Epic | epic, war, battle, medieval, ancient, mythology, sword and sorcery, historical |
| Adrenaline | action, explosion, car chase, martial arts, heist, revenge, gunfight, fight |

### How They Are Assigned

1. During metadata enrichment (scan, auto-match, or re-enrichment), TMDB keywords are fetched along with other metadata
2. Keywords are stored as a JSON array on the `media_items.keywords` column
3. Each keyword is checked against the mood mapping table
4. Matched moods are deduplicated and linked as tags with `category = 'mood'`
5. Tags are created on first use (find-or-create pattern)

### Where They Are Used

- Smart collection rules can filter by mood
- The tag system exposes moods alongside genres and custom tags
- Future: mood-based recommendation rows, mood filtering in browse views

---

## Smart Collections

Smart collections are dynamic playlists that automatically populate based on user-defined rules. Unlike manual collections where items are added one by one, smart collections evaluate their rules against the library each time they are viewed.

Smart collections support a wide range of filter criteria including genres, moods, performers, studios, duration ranges, recency, and configurable sorting. Results can be sorted by title, year, rating, added date, duration, or randomly.

For full documentation of collections (manual, smart, nested, statistics, templates, and all API endpoints), see [COLLECTIONS.MD](COLLECTIONS.MD).

---

## Discovery Hubs

CineVault provides curated discovery endpoints for browsing content by trending, genre, and decade.

### Trending

`GET /api/v1/discover/trending`

Returns trending content based on recent watch activity across all users. Items are ranked by play count and recency.

### Genre Hub

`GET /api/v1/discover/genre/{slug}`

Returns a genre-focused browse page with content tagged with the specified genre. Includes related genres and mood suggestions.

### Decade Hub

`GET /api/v1/discover/decade/{year}`

Returns content grouped by decade (e.g., `/decade/1980` returns all 1980s content). Useful for era-based browsing.

---

## API Reference

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/recommendations` | Personalized recommendations (top 20) |
| GET | `/api/v1/recommendations/because-you-watched` | Content-similarity rows (5 sources x 8 similar) |
| GET | `/api/v1/discover/trending` | Trending content |
| GET | `/api/v1/discover/genre/{slug}` | Genre hub browse |
| GET | `/api/v1/discover/decade/{year}` | Decade hub browse |

All endpoints require authentication (Bearer token) and `user` role minimum.

For collection-related endpoints, see [COLLECTIONS.MD](COLLECTIONS.MD).
For watchlist and favorites, see [PLAYBACK.MD](PLAYBACK.MD).

---

## Database Schema

### Migration 021

```sql
-- Mood tag category
ALTER TYPE tag_category ADD VALUE IF NOT EXISTS 'mood';

-- Smart collection rules
ALTER TABLE collections ADD COLUMN IF NOT EXISTS rules JSONB;

-- TMDB keywords storage
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS keywords TEXT;

-- Index for recommendation queries
CREATE INDEX IF NOT EXISTS idx_watch_history_user_completed
  ON watch_history(user_id, completed, last_watched_at DESC);
```

### Key Tables

| Table | Role in Recommendations |
|---|---|
| `watch_history` | Tracks what users have watched, progress, and completion status |
| `tags` / `media_tags` | Genre and mood tags linked to media items |
| `performers` / `media_performers` | Cast and crew linked to media items |
| `collections` | Stores smart collection rules in the `rules` JSONB column |
| `media_items` | `keywords` column stores raw TMDB keywords as JSON |
