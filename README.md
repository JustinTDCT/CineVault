# CineVault

Self-hosted media server with AI-powered organization, duplicate detection, and multi-format support.

## Phase 2 - Advanced Organization

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
- Full Plex-style web UI with all media types

## Tech Stack

- **Backend**: Go 1.24
- **Database**: PostgreSQL 16
- **Cache/Queue**: Redis 7
- **Deployment**: Docker Compose

## Quick Start

```bash
# Start services
docker compose -f docker/docker-compose.yml up -d

# Apply migrations
psql -h localhost -U cinevault -d cinevault < migrations/001_initial_schema.up.sql
psql -h localhost -U cinevault -d cinevault < migrations/002_phase2_schema.up.sql

# Build and run
go build -o cinevault ./cmd/cinevault/
./cinevault
```

Server starts on `http://localhost:8080`

## API Endpoints

### Authentication
- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login and get JWT token

### Libraries (requires auth)
- `GET /api/v1/libraries` - List all libraries
- `POST /api/v1/libraries` - Create library (Admin)
- `GET /api/v1/libraries/{id}` - Get library details
- `PUT /api/v1/libraries/{id}` - Update library (Admin)
- `DELETE /api/v1/libraries/{id}` - Delete library (Admin)
- `POST /api/v1/libraries/{id}/scan` - Trigger library scan (Admin)

### Media (requires auth)
- `GET /api/v1/libraries/{id}/media` - List media in library
- `GET /api/v1/media/{id}` - Get media details
- `GET /api/v1/media/search?q=query` - Search media

### Edition Groups (requires auth)
- `GET /api/v1/editions` - List edition groups
- `POST /api/v1/editions` - Create edition group (Admin)
- `GET /api/v1/editions/{id}` - Get edition group with items
- `PUT /api/v1/editions/{id}` - Update edition group (Admin)
- `DELETE /api/v1/editions/{id}` - Delete edition group (Admin)
- `POST /api/v1/editions/{id}/items` - Add item to group (Admin)
- `DELETE /api/v1/editions/{id}/items/{itemId}` - Remove item (Admin)

### Sister Groups (requires auth)
- `GET /api/v1/sisters` - List sister groups
- `POST /api/v1/sisters` - Create sister group (Admin)
- `GET /api/v1/sisters/{id}` - Get sister group with members
- `POST /api/v1/sisters/{id}/items` - Add member (Admin)
- `DELETE /api/v1/sisters/{id}/items/{itemId}` - Remove member (Admin)
- `DELETE /api/v1/sisters/{id}` - Delete group (Admin)

### Collections (requires auth)
- `GET /api/v1/collections` - List user's collections
- `POST /api/v1/collections` - Create collection
- `GET /api/v1/collections/{id}` - Get collection with items
- `POST /api/v1/collections/{id}/items` - Add item
- `DELETE /api/v1/collections/{id}/items/{itemId}` - Remove item
- `DELETE /api/v1/collections/{id}` - Delete collection

### Watch History (requires auth)
- `POST /api/v1/watch/{mediaId}/progress` - Update watch progress
- `GET /api/v1/watch/continue` - Get continue watching list

### Admin
- `GET /api/v1/users` - List users (Admin only)

All protected endpoints require `Authorization: Bearer <token>` header.

## Project Structure

```
cinevault/
  cmd/cinevault/main.go          # Entry point
  internal/
    api/
      server.go                  # HTTP server, routes, middleware
      handlers_auth.go           # Auth & user endpoints
      handlers_library.go        # Library CRUD + scan
      handlers_media.go          # Media CRUD + search
      handlers_editions.go       # Edition group endpoints
      handlers_sisters.go        # Sister group endpoints
      handlers_collections.go    # Collection endpoints
      handlers_watch.go          # Watch history endpoints
    auth/auth.go                 # JWT + bcrypt
    config/config.go             # Env-based config
    db/db.go                     # PostgreSQL connection
    ffmpeg/ffprobe.go            # FFprobe wrapper
    models/models.go             # All data models
    repository/
      user_repository.go
      library_repository.go
      media_repository.go
      tv_repository.go
      music_repository.go
      audiobook_repository.go
      gallery_repository.go
      edition_repository.go
      sister_repository.go
      collection_repository.go
      watch_history_repository.go
    scanner/scanner.go           # Media scanner with DB persistence
  migrations/
    001_initial_schema.up.sql
    001_initial_schema.down.sql
    002_phase2_schema.up.sql
    002_phase2_schema.down.sql
  docker/docker-compose.yml
  web/index.html                 # Plex-style web UI
```

## Development

See `DESIGN.md` for full architecture and roadmap.
