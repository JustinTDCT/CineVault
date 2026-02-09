# CineVault

> **This is still under heavy development and is far from complete but you are welcome to try it out and give me feedback. I am taking what I like best from Plex, Jellyfin, and StashApp and making one media player that does it all.**

Self-hosted media server with AI-powered organization, duplicate detection, and multi-format streaming.

## What's Working

### Media Management
- **9 media types** — Movies, Adult Movies, TV Shows, Music, Music Videos, Home Videos, Other Videos, Photos, Audiobooks
- **Multi-folder libraries** with folder browser, per-library permissions (Everyone / Select Users / Admin Only)
- **Library scanning** with ffprobe metadata extraction and DB persistence
- **TV show hierarchy** auto-created from file paths (show / season / episode)
- **Edition groups** (Director's Cut, Remastered, etc.)
- **Sister groups** for linking related content
- **User collections** for curating favorites
- **Drag-and-drop sort ordering** for media, collections, editions, performers

### Playback & Streaming
- **Direct Play** — Range-request streaming for natively compatible formats (MP4, WebM)
- **MPEG-TS Remux** — On-the-fly FFmpeg remux for MKV/AVI with mpegts.js playback
- **HLS Transcoding** — Hardware-accelerated transcoding (NVENC, QSV, VAAPI) with quality selection
- **Full video player UI** — Play/pause, seek, skip, volume, fullscreen, keyboard shortcuts
- **Watch history & continue watching**

### Metadata & Enrichment
- **TMDB** scraping for movies and TV shows
- **MusicBrainz** for music metadata
- **Open Library** for audiobooks
- **OMDb API** integration for IMDB ratings, Rotten Tomatoes, audience scores
- **Metadata cache server** for faster lookups with automatic fallback to direct API calls
- **Per-item metadata lock** to prevent overwrites on re-scan

### Duplicate Detection
- **Perceptual hashing (pHash)** and audio fingerprinting with similarity scoring
- **Review queue** — side-by-side comparison, merge as edition, delete, or ignore
- **Badge count** in sidebar for pending duplicates

### People, Tags & Studios
- **Performers / People** — Actors, directors, musicians, narrators with media linking
- **Tags / Genres** — Hierarchical tag system with categories
- **Studios / Labels** — Studio, label, publisher, network, distributor entities

### Authentication & Access
- **Multi-user** with roles (Admin / User / Guest) and JWT auth
- **Fast Login** — PIN-based quick login with user avatar selection screen
- **Cinematic login intro** with admin toggle for skip/mute
- **User profile editing** with display name, email, avatar

### UI & Experience
- **Dedicated settings page** with left navigation (Video, Transcoder, Users, Security, Experience, Libraries, Metadata)
- **Admin panel** — System status, user management, job queue monitor
- **User avatar dropdown** — Edit Profile, Settings, Logout
- **Real-time WebSocket updates** — Live scan progress, job status, toast notifications
- **Background job queue** (Asynq + Redis) for async scanning, fingerprinting, preview generation, metadata scraping

## Tech Stack

- **Backend**: Go 1.24
- **Database**: PostgreSQL 16
- **Cache/Queue**: Redis 7 + Asynq
- **Streaming**: FFmpeg (HLS transcoding, MPEG-TS remux)
- **Frontend**: Single-page HTML app, mpegts.js, HLS.js, SortableJS
- **Deployment**: Docker Compose

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

## API Endpoints (70+)

### Authentication
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login and get JWT token
- `PUT /api/v1/auth/pin` - Set fast login PIN

### Libraries
- `GET /api/v1/libraries` - List all libraries
- `POST /api/v1/libraries` - Create library (Admin)
- `GET /api/v1/libraries/{id}` - Get library details
- `PUT /api/v1/libraries/{id}` - Update library (Admin)
- `DELETE /api/v1/libraries/{id}` - Delete library (Admin)
- `POST /api/v1/libraries/{id}/scan` - Trigger library scan (async via job queue)
- `POST /api/v1/libraries/{id}/auto-match` - Bulk metadata auto-match (Admin)

### Media
- `GET /api/v1/libraries/{id}/media` - List media in library
- `GET /api/v1/media/{id}` - Get media details
- `GET /api/v1/media/search?q=query` - Search media
- `POST /api/v1/media/{id}/identify` - Search external metadata sources
- `POST /api/v1/media/{id}/apply-meta` - Apply metadata match

### Streaming
- `GET /api/v1/stream/{mediaId}/master.m3u8` - HLS master playlist
- `GET /api/v1/stream/{mediaId}/{quality}/{segment}` - HLS segment
- `GET /api/v1/stream/{mediaId}/direct` - Direct file stream (range requests)

### Performers
- `GET /api/v1/performers` - List/search performers
- `POST /api/v1/performers` - Create performer (Admin)
- `GET /api/v1/performers/{id}` - Get performer with linked media
- `PUT /api/v1/performers/{id}` - Update performer (Admin)
- `DELETE /api/v1/performers/{id}` - Delete performer (Admin)
- `POST /api/v1/media/{id}/performers` - Link performer to media
- `DELETE /api/v1/media/{id}/performers/{performerId}` - Unlink performer

### Tags / Genres
- `GET /api/v1/tags?tree=true` - List tags (flat or tree)
- `POST /api/v1/tags` - Create tag (Admin)
- `PUT /api/v1/tags/{id}` - Update tag (Admin)
- `DELETE /api/v1/tags/{id}` - Delete tag (Admin)
- `POST /api/v1/media/{id}/tags` - Assign tags to media
- `DELETE /api/v1/media/{id}/tags/{tagId}` - Remove tag from media

### Studios / Labels
- `GET /api/v1/studios` - List studios
- `POST /api/v1/studios` - Create studio (Admin)
- `GET /api/v1/studios/{id}` - Get studio details
- `PUT /api/v1/studios/{id}` - Update studio (Admin)
- `DELETE /api/v1/studios/{id}` - Delete studio (Admin)
- `POST /api/v1/media/{id}/studios` - Link studio to media
- `DELETE /api/v1/media/{id}/studios/{studioId}` - Unlink studio

### Duplicates
- `GET /api/v1/duplicates` - List pending duplicate pairs
- `POST /api/v1/duplicates/resolve` - Resolve duplicate (merge/delete/ignore/sister/edition)

### Edition Groups / Sister Groups / Collections
- Full CRUD endpoints

### Watch History
- `POST /api/v1/watch/{mediaId}/progress` - Update watch progress
- `GET /api/v1/watch/continue` - Get continue watching list

### WebSocket
- `GET /api/v1/ws?token=JWT` - Real-time event stream

### Settings & Admin
- `GET /api/v1/settings/playback` - Get playback preferences
- `PUT /api/v1/settings/playback` - Update playback preferences
- `GET /api/v1/settings/system` - Get system settings (Admin)
- `PUT /api/v1/settings/system` - Update system settings (Admin)
- `GET /api/v1/jobs` - List recent jobs (Admin)
- `GET /api/v1/jobs/{id}` - Get job status (Admin)
- `PATCH /api/v1/sort` - Update sort order for any entity type (Admin)
- `GET /api/v1/users` - List users (Admin)

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
