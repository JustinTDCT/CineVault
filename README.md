# CineVault

A self-hosted media server for movies, TV, music, audiobooks and books.

This is a fresh start (November 2026). Earlier work is preserved on these branches:

- `v2-archive`: the previous main branch, archived 26 Nov 2026
- `v1-archive`: the earlier archive

## Development quick start

Requires Go 1.24 or newer, FFmpeg, and the development PostgreSQL container described in [`docs/environment/test-server.md`](docs/environment/test-server.md). That container's credentials live in `~/.config/cinevault/db.env`.

```bash
make test     # unit tests (no database needed)
make test-db  # all tests, including database tests on temporary databases
make run      # build and start the server against the dev database
```

On startup the server connects to PostgreSQL and applies any pending schema migrations (`internal/db/migrations`), then starts the API. Check it from another terminal:

```bash
curl -s http://localhost:8456/api/health
curl -s 'http://localhost:8456/api/edition-types?media_type=movie'
```

### API

| Endpoint | Purpose |
|---|---|
| `GET /api/health` | Server, database and transcoding status |
| `GET /api/edition-types` | The Edition List. Optional `media_type` (`movie`, `tv`, `music`, `audiobook`, `book`) and `include_hidden=true`. |

The health check reports:
- `status`: `ok`, or `degraded` with HTTP 503 when the database can't be reached or FFmpeg is missing
- whether the database is reachable, and its schema version
- the FFmpeg version and hardware acceleration methods
- each NVIDIA encoder, and whether it worked in a real test encode
- the GPUs found

### Settings

Settings come from environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `CINEVAULT_DATABASE_URL` | none, required | PostgreSQL URL, for example `postgres://user:pass@127.0.0.1:5432/cinevault` |
| `CINEVAULT_LISTEN` | `:8456` | HTTP listen address |
| `CINEVAULT_FFMPEG` | `ffmpeg` | Path to the FFmpeg binary |

## Project layout

| Path | Contents |
|---|---|
| `cmd/cinevault` | Server entry point |
| `internal/catalog` | Library data types shared by the database and the API |
| `internal/config` | Settings from environment variables |
| `internal/db` | PostgreSQL connection, migrations and queries |
| `internal/probe` | FFmpeg, GPU and hardware-encoder detection |
| `internal/server` | HTTP API |

## Design docs

- Decisions: [`docs/decisions`](docs/decisions/)
- Feature specs: [`docs/specs`](docs/specs/README.md)
- Feature research (Plex, Emby, Jellyfin and user requests): [`docs/research/feature-master-list.md`](docs/research/feature-master-list.md)
- Test server setup (Hyper-V VM with NVIDIA A2): [`docs/environment/test-server.md`](docs/environment/test-server.md)
