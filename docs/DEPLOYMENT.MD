# CineVault Deployment Guide

This document describes how to deploy CineVault using Docker, configure environment variables, enable hardware acceleration, and set up CI/CD.

---

## Architecture

CineVault runs as three containers managed by Docker Compose:

```
┌──────────────────────────────────────────────────────┐
│  Docker Host                                         │
│                                                      │
│  ┌──────────────┐  ┌────────────┐  ┌──────────────┐ │
│  │  CineVault   │  │ PostgreSQL │  │    Redis     │ │
│  │  (Go + Web)  │──│    16      │  │      7       │ │
│  │  Port 8080   │  │  Port 5432 │  │  Port 6379   │ │
│  └──────────────┘  └────────────┘  └──────────────┘ │
│         │                                            │
│    /media (bind mount)                               │
│    /previews (named volume)                          │
│    /thumbnails (named volume)                        │
└──────────────────────────────────────────────────────┘
```

---

## Quick Start

```bash
# Clone the repository
git clone https://github.com/JustinTDCT/CineVault.git
cd CineVault

# Copy and edit environment file
cp docker/.env.example docker/.env
# Edit docker/.env with your settings

# Start all services
docker compose -f docker/docker-compose.yml up -d
```

Migrations are applied automatically on startup. Open `http://localhost:8080` and complete the setup wizard to create the admin account.

---

## Docker Compose Services

### cinevault (Application)

- **Image:** Built from `Dockerfile` in the repository root
- **Container name:** `cinevault-app`
- **Port:** `${SERVER_PORT:-8080}:8080`
- **Depends on:** postgres (healthy), redis (healthy)
- **Restart policy:** `unless-stopped`

**Volumes:**

| Host Path | Container Path | Purpose |
|---|---|---|
| `/media` (configurable) | `/media` | Media files (read-write) |
| `previews_data` (named) | `/previews` | Generated preview files |
| `thumbnails_data` (named) | `/thumbnails` | Generated thumbnails |

### postgres (Database)

- **Image:** `postgres:16-alpine`
- **Container name:** `cinevault-postgres`
- **Port:** `${DB_EXPOSE_PORT:-5432}:5432`
- **Volume:** `postgres_data` → `/var/lib/postgresql/data`
- **Health check:** `pg_isready`

### redis (Cache / Job Queue)

- **Image:** `redis:7-alpine`
- **Container name:** `cinevault-redis`
- **Port:** `${REDIS_EXPOSE_PORT:-6379}:6379`
- **Volume:** `redis_data` → `/data`
- **Health check:** `redis-cli ping`

---

## Environment Variables

### Database

| Variable | Default | Description |
|---|---|---|
| `DB_HOST` | `localhost` | PostgreSQL hostname |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `cinevault` | Database username |
| `DB_PASSWORD` | `cinevault_dev_pass` | Database password |
| `DB_NAME` | `cinevault` | Database name |
| `DB_SSLMODE` | `disable` | SSL mode (`disable`, `require`, `verify-full`) |
| `DB_EXPOSE_PORT` | `5432` | Docker-mapped PostgreSQL port on host |

### Redis

| Variable | Default | Description |
|---|---|---|
| `REDIS_HOST` | `localhost` | Redis hostname |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | (empty) | Redis password (optional) |
| `REDIS_EXPOSE_PORT` | `6379` | Docker-mapped Redis port on host |

### Server

| Variable | Default | Description |
|---|---|---|
| `SERVER_HOST` | `0.0.0.0` | HTTP listen address |
| `SERVER_PORT` | `8080` | HTTP listen port |

### Authentication

| Variable | Default | Description |
|---|---|---|
| `JWT_SECRET` | `your-secret-key-change-in-production` | JWT signing secret (change this!) |
| `JWT_EXPIRES_IN` | `24h` | JWT token lifetime |
| `JWT_REFRESH_EXPIRES_IN` | `168h` | Refresh token lifetime (7 days) |

### Paths

| Variable | Default | Description |
|---|---|---|
| `MEDIA_PATH` | `/media` | Root directory for media files |
| `PREVIEW_PATH` | `/previews` | Output directory for preview files |
| `THUMBNAIL_PATH` | `/thumbnails` | Output directory for thumbnails |

### FFmpeg

| Variable | Default | Description |
|---|---|---|
| `FFMPEG_PATH` | `/usr/lib/jellyfin-ffmpeg/ffmpeg` | FFmpeg binary path |
| `FFPROBE_PATH` | `/usr/lib/jellyfin-ffmpeg/ffprobe` | FFprobe binary path |

### Metadata APIs

| Variable | Default | Description |
|---|---|---|
| `TMDB_API_KEY` | (empty) | TMDB API key for metadata lookups |

---

## Build Process

The Dockerfile uses a multi-stage build:

### Stage 1: Builder

- Base: `golang:1.24-alpine`
- Downloads Go dependencies
- Builds the binary with `CGO_ENABLED=0 GOOS=linux` and `-ldflags="-s -w"` for a minimal binary

### Stage 2: Runtime

- Base: `debian:bookworm-slim`
- Installs:
  - `curl`, `gnupg`, `ca-certificates`, `tzdata`, `postgresql-client`
  - `jellyfin-ffmpeg7` (from Jellyfin's official Debian repository)
  - `intel-media-va-driver`, `mesa-va-drivers`, `vainfo` (VAAPI support)
- Copies: binary, `version.json`, `web/` assets, `migrations/`, `entrypoint.sh`
- Creates: `/previews`, `/thumbnails`, `/data` directories
- Exposes: port 8080

The Jellyfin FFmpeg build includes hardware acceleration support (NVENC, QSV, VAAPI) out of the box.

---

## Startup Process

The `entrypoint.sh` script runs before the application:

1. Displays the CineVault startup banner
2. Waits for PostgreSQL to become available (connection check loop)
3. Auto-applies database migrations:
   - Scans `/app/migrations/*.up.sql`
   - Sorts files numerically
   - Applies each migration in order
   - Silently skips "already exists" errors (idempotent)
4. Starts the CineVault binary

After the binary starts, the application:
- Connects to PostgreSQL and Redis
- Initializes the Asynq job queue worker
- Starts background goroutines:
  - System metrics collector (60-second intervals)
  - Daily stats rollup scheduler (midnight)
  - Alert evaluator (5-minute intervals)
  - Filesystem watcher (monitors library folders)
  - Scheduled scan checker (60-second intervals)
- Starts the HTTP server on the configured port

---

## Hardware Acceleration

### NVIDIA (NVENC)

Requirements:
- NVIDIA GPU with NVENC support
- `nvidia-container-toolkit` installed on the Docker host

```bash
docker compose -f docker/docker-compose.yml -f docker/docker-compose.hwaccel.yml up -d
```

The hwaccel overlay adds `runtime: nvidia` and passes the GPU device into the container.

### Intel Quick Sync (QSV)

Requirements:
- Intel CPU or GPU with Quick Sync support
- `/dev/dri` device available on the host

```bash
docker compose -f docker/docker-compose.yml -f docker/docker-compose.hwaccel.yml up -d
```

The hwaccel overlay maps `/dev/dri` into the container. The runtime image includes Intel media drivers.

### AMD / Intel VAAPI

Requirements:
- GPU with VAAPI support
- `/dev/dri` device available on the host

Same compose command as QSV — VAAPI drivers are included in the runtime image.

### Verification

CineVault auto-detects hardware acceleration on startup. Check the application logs for:

```
Detected H.264 encoder: h264_nvenc    (NVIDIA)
Detected H.264 encoder: h264_qsv     (Intel QSV)
Detected H.264 encoder: h264_vaapi   (VAAPI)
Detected H.264 encoder: libx264      (software fallback)
```

The analytics dashboard also shows the hardware accelerator in use under the Transcodes tab.

---

## Unraid Deployment

CineVault provides an Unraid-specific compose file:

```bash
docker compose -f docker/docker-compose.unraid.yml up -d
```

Use `docker/.env.unraid.example` as the environment template for Unraid-specific path mappings.

---

## CI/CD Pipeline

CineVault uses GitHub Actions to build and publish Docker images:

### Trigger

Push to the `main` branch triggers the pipeline.

### Steps

1. Checkout code
2. Login to GitHub Container Registry (`ghcr.io`)
3. Read version from `version.json`
4. Build Docker image with tags: `latest` and the version number (e.g., `1.19.00`)
5. Push to `ghcr.io/{repository}`

### Image Tags

| Tag | Description |
|---|---|
| `latest` | Most recent build from main |
| `{version}` | Specific version (e.g., `1.19.00`) |

---

## Versioning

CineVault uses semantic versioning stored in `version.json`:

```json
{
  "version": "1.19.00",
  "phase": 19,
  "released": "2026-02-17",
  "notes": "Description of changes"
}
```

Version format: `MAJOR.MINOR.PATCH`

| Segment | Meaning |
|---|---|
| `MAJOR` (xx.00.00) | Breaking change (incompatible with previous versions) |
| `MINOR` (00.xx.00) | Feature addition |
| `PATCH` (00.00.xx) | Bug fix |

---

## Database Migrations

Migrations are stored in `migrations/` as numbered SQL files:

```
migrations/
  001_initial_schema.up.sql
  001_initial_schema.down.sql
  002_phase2_schema.up.sql
  002_phase2_schema.down.sql
  ...
  043_library_preview_settings.up.sql
  043_library_preview_settings.down.sql
```

- `.up.sql` files create or alter schema
- `.down.sql` files revert changes
- Migrations are applied automatically by `entrypoint.sh` on startup
- Each migration is idempotent (uses `IF NOT EXISTS` / `IF EXISTS` guards)

---

## Backup and Restore

CineVault provides built-in backup capabilities:

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/admin/backup` | Admin | Create a database backup |
| GET | `/api/v1/admin/backups` | Admin | List available backups |
| GET | `/api/v1/admin/backups/{id}/download` | Admin | Download a backup file |

Backups include the full database state. Media files are not included in backups (they should be backed up separately via your storage solution).

---

## PWA Support

CineVault is installable as a Progressive Web App:

- **Manifest:** `web/manifest.json`
- **Service Worker:** `web/sw.js`
- **Display:** standalone (no browser chrome)
- **Theme:** Dark (#0a0f1c background, #00d9ff accent)
- **Shortcuts:** Home, Search, Libraries

---

## File Locations

| Component | Path |
|---|---|
| Dockerfile | `Dockerfile` |
| Docker Compose | `docker/docker-compose.yml` |
| HW Accel Overlay | `docker/docker-compose.hwaccel.yml` |
| Unraid Compose | `docker/docker-compose.unraid.yml` |
| Entrypoint Script | `docker/entrypoint.sh` |
| Environment Template | `docker/.env.example` |
| Unraid Env Template | `docker/.env.unraid.example` |
| Migrations | `migrations/*.sql` |
| CI/CD Workflow | `.github/workflows/docker-publish.yml` |
| Version Info | `version.json` |
| PWA Manifest | `web/manifest.json` |
| Service Worker | `web/sw.js` |
| Configuration | `internal/config/config.go` |
