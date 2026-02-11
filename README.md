# CineVault

> **This is still under heavy development and is far from complete but you are welcome to try it out and give me feedback. I am taking what I like best from Plex, Jellyfin, and StashApp and making one media player that does it all.**
>
> This is now and always will be free. There is no charge for sharing or downloading apps and what not! The only thing I ask is that you donate or support my work. Totally voluntary. This just helps me offset the cost of time and effort I spend doing this - and while my ideas spawn from my own needs I try to make them open because I know other people can benefit from them also!
>
> **[Buy Me A Pizza](https://buymeacoffee.com/jtdct)**

Self-hosted media server with AI-powered organization, duplicate detection, and multi-format streaming.

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.24 |
| Database | PostgreSQL 16 |
| Cache / Queue | Redis 7 + Asynq |
| Streaming | FFmpeg (HLS transcoding, MPEG-TS remux) |
| Frontend | Single-page HTML app, mpegts.js, HLS.js, SortableJS |
| Deployment | Docker Compose |

## Features at a Glance

### Media Management
9 media types (Movies, Adult Movies, TV Shows, Music, Music Videos, Home Videos, Other Videos, Photos, Audiobooks), multi-folder libraries with access control, TV show hierarchy auto-created from file paths, edition groups, sister groups, and drag-and-drop sort ordering.

See [docs/FILE-PARSING.MD](docs/FILE-PARSING.MD) for filename conventions, folder structure, and the ingestion pipeline.

### Playback and Streaming
Direct Play for natively compatible formats, MPEG-TS remux for MKV/AVI via FFmpeg, and HLS transcoding with hardware acceleration (NVENC, QSV, VAAPI) and quality selection. Full video player UI with watch history and continue watching.

### Metadata and Enrichment
Cache-first architecture pulling from TMDB, OMDb, fanart.tv, MusicBrainz, OpenLibrary, and PornDB. NFO import/export, per-field metadata locking, auto-collections from TMDB franchise data, and mood tagging derived from TMDB keywords.

See [docs/METADATA.MD](docs/METADATA.MD) for the full metadata flow, cache server architecture, and field reference.

### Duplicate Detection
Perceptual hashing (pHash) and audio fingerprinting with similarity scoring. Review queue with side-by-side comparison and resolution options (merge as edition, delete, ignore, sister group).

### Filtering and Search
Library-level dropdown filters (genre, year, rating, mood, resolution, source type, and more) combined with AND logic. Global cross-library text search and missing episode detection for TV shows.

See [docs/FILTER-SEARCH.MD](docs/FILTER-SEARCH.MD) for filter mechanics, query building, and the duplicate finder.

### Collections
Manual and smart collections with nesting support. Smart collections use JSONB rule sets that re-evaluate dynamically. Movie series auto-created from TMDB collection data. Collection statistics, template presets, and full CRUD API.

See [docs/COLLECTIONS.MD](docs/COLLECTIONS.MD) for rule syntax, hierarchy model, and endpoints.

### Recommendations
Personalized scoring based on recency-weighted genre affinity and cast/director overlap. "Because You Watched" similarity rows. Mood tags mapped from TMDB keywords. Smart collections as dynamic playlists.

See [docs/RECOMMENDATIONS.MD](docs/RECOMMENDATIONS.MD) for the scoring algorithm, mood mapping, and filters.

### Intro and Credits Detection
Audio fingerprint cross-episode intro detection, black-frame/silence credits detection, anime OP/ED heuristics, and recap scene-change density analysis. Per-user auto-skip preferences with skip button overlay.

See [docs/INTRO-CREDITS.MD](docs/INTRO-CREDITS.MD) for detection methods, confidence levels, and player behavior.

### Users and Households
Multi-user with roles (Admin / User / Guest), JWT auth, PIN-based fast login, master/sub-profile household system, parental controls with content rating caps, kids mode, and per-library access control.

See [docs/USERS.MD](docs/USERS.MD) for authentication flows, household architecture, and permissions.

### Analytics and Monitoring
Real-time stream and transcode tracking, system metrics polling, nightly rollup aggregation, configurable alert rules with webhook delivery, and a Chart.js admin dashboard.

See [docs/ANALYTICS-DASHBOARD.MD](docs/ANALYTICS-DASHBOARD.MD) for data collection, alerting, and dashboard components.

## Quick Start

```bash
# Start services
docker compose -f docker/docker-compose.yml up -d

# Apply migrations
psql -h localhost -U cinevault -d cinevault < migrations/001_initial_schema.up.sql
psql -h localhost -U cinevault -d cinevault < migrations/002_phase2_schema.up.sql
psql -h localhost -U cinevault -d cinevault < migrations/003_phase3_schema.up.sql

# Build and run
go build -o cinevault ./cmd/cinevault/
./cinevault
```

Server starts on `http://localhost:8080`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `cinevault` | Database user |
| `DB_PASSWORD` | `cinevault_dev_pass` | Database password |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `SERVER_PORT` | `8080` | HTTP server port |
| `JWT_SECRET` | (dev key) | JWT signing secret |
| `TMDB_API_KEY` | (empty) | TMDB API key for metadata |
| `FFMPEG_PATH` | `/usr/bin/ffmpeg` | FFmpeg binary path |
| `FFPROBE_PATH` | `/usr/bin/ffprobe` | FFprobe binary path |
