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

## Development

See `DESIGN.md` for full architecture and roadmap.
