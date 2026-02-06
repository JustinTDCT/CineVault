# CineVault

Self-hosted media server with AI-powered organization, duplicate detection, and multi-format support.

## Phase 1 Features

- Movies + TV Shows support
- Multi-user authentication (Admin/User/Guest)
- Library scanning with ffprobe metadata extraction
- Basic duplicate detection (merge/delete/ignore)
- PostgreSQL database with migrations
- Docker deployment

## Tech Stack

- **Backend**: Go 1.24
- **Database**: PostgreSQL 16
- **Cache/Queue**: Redis + Asynq
- **Deployment**: Docker Compose

## Quick Start

```bash
# Start services
docker-compose -f docker/docker-compose.yml up -d

# Run migrations (manual for now)
# psql -h localhost -U cinevault -d cinevault < migrations/001_initial_schema.up.sql

# Build and run
make build
./cinevault
```

Server will start on `http://localhost:8080`

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

### Media (requires auth)
- `GET /api/v1/libraries/{id}/media` - List media in library
- `GET /api/v1/media/{id}` - Get media details
- `GET /api/v1/media/search?q=query` - Search media

### Admin
- `GET /api/v1/users` - List users (Admin only)

All protected endpoints require `Authorization: Bearer <token>` header.

## Development

See `DESIGN.md` for full architecture and roadmap.
