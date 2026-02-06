# CineVault

Self-hosted media server with AI-powered organization, duplicate detection, and multi-format support.

## Phase 3 - Complete Systems

### All 9 Media Types
Movies, Adult Movies, TV Shows, Music, Music Videos, Home Videos, Other Videos, Photos, Audiobooks

### Core Features
- Multi-user authentication (Admin/User/Guest) with JWT
- Library scanning with ffprobe metadata extraction and DB persistence
- TV show hierarchy auto-creation (show/season/episode from file paths)
- Edition groups (Director's Cut, Remastered, etc.)
- Sister groups (link related content)
- User collections (curate favorites)
- Watch history / continue watching

### Phase 3 Features
- **Background Job Queue** - Asynq + Redis for async scan, fingerprint, preview generation, metadata scraping
- **WebSocket Real-Time Updates** - Live notifications for job progress, scan updates, toast messages
- **HLS Streaming** - On-the-fly FFmpeg transcoding with hardware acceleration detection (NVENC, QSV, VAAPI)
- **Direct Play** - Range-request streaming for compatible formats
- **HLS.js Video Player** - Full player UI with quality selection, seek, keyboard shortcuts
- **Preview Generation** - Thumbnail extraction, sprite sheets, animated previews via FFmpeg
- **Duplicate Detection** - Perceptual hashing (pHash) and audio fingerprinting with similarity comparison
- **Metadata Scraping** - TMDB (movies/TV), MusicBrainz (music), Open Library (audiobooks)
- **Performers / People** - Actor, director, musician, narrator entities with media linking
- **Tags / Genres** - Hierarchical tag system with categories (genre, tag, custom)
- **Studios / Labels** - Studio, label, publisher, network, distributor entities
- **Sort Order** - Drag-and-drop ordering for media, collections, editions, performers
- **Media Detail Page** - Full metadata display, cast, tags, file info, play/identify actions
- **Settings Page** - Playback preferences (mode, quality, auto-play)
- **Admin Panel** - User management, job queue monitor, system status
- **Duplicate Review Queue** - Review and resolve duplicate pairs

## Tech Stack

- **Backend**: Go 1.24
- **Database**: PostgreSQL 16
- **Cache/Queue**: Redis 7 + Asynq
- **Streaming**: FFmpeg (HLS transcoding)
- **Frontend**: HLS.js, SortableJS
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
- Full CRUD endpoints (same as Phase 2)

### Watch History
- `POST /api/v1/watch/{mediaId}/progress` - Update watch progress
- `GET /api/v1/watch/continue` - Get continue watching list

### WebSocket
- `GET /api/v1/ws?token=JWT` - Real-time event stream

### Settings & Admin
- `GET /api/v1/settings/playback` - Get playback preferences
- `PUT /api/v1/settings/playback` - Update playback preferences
- `GET /api/v1/jobs` - List recent jobs (Admin)
- `GET /api/v1/jobs/{id}` - Get job status (Admin)
- `PATCH /api/v1/sort` - Update sort order for any entity type (Admin)
- `GET /api/v1/users` - List users (Admin)

## Project Structure

```
cinevault/
  cmd/cinevault/main.go          # Entry point, job queue setup
  internal/
    api/
      server.go                  # HTTP server, 70+ routes, middleware
      websocket.go               # WebSocket hub + handler
      handlers_auth.go           # Auth & user endpoints
      handlers_library.go        # Library CRUD + async scan
      handlers_media.go          # Media CRUD + search
      handlers_stream.go         # HLS + direct play streaming
      handlers_performers.go     # Performer CRUD + media linking
      handlers_tags.go           # Tag CRUD + media assignment
      handlers_studios.go        # Studio CRUD + media linking
      handlers_duplicates.go     # Duplicate review + resolution
      handlers_metadata.go       # Metadata identify/apply, sort, jobs, prefs
      handlers_editions.go       # Edition group endpoints
      handlers_sisters.go        # Sister group endpoints
      handlers_collections.go    # Collection endpoints
      handlers_watch.go          # Watch history endpoints
    auth/auth.go                 # JWT + bcrypt
    config/config.go             # Env-based config (TMDB_API_KEY)
    db/db.go                     # PostgreSQL connection
    ffmpeg/ffprobe.go            # FFprobe wrapper
    fingerprint/fingerprint.go   # pHash + audio fingerprinting
    jobs/
      queue.go                   # Asynq client/server
      tasks.go                   # Task handlers (scan, fingerprint, preview, metadata)
    metadata/scraper.go          # TMDB, MusicBrainz, Open Library scrapers
    models/models.go             # All data models (40+ structs)
    preview/preview.go           # Thumbnail, sprite sheet, animated preview generation
    repository/                  # 15 repository files
    scanner/scanner.go           # Media scanner with DB persistence
    stream/
      transcoder.go              # FFmpeg HLS transcoding + HW accel
      direct.go                  # Direct file streaming with range requests
  migrations/
    001_initial_schema.up.sql
    002_phase2_schema.up.sql
    003_phase3_schema.up.sql     # 9 new tables, 4 new enum types
  docker/docker-compose.yml
  web/index.html                 # Full SPA with video player
```

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

## Development

See `CINEVAULT_DESIGN.md` for full architecture and roadmap.
