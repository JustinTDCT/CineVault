# Decision 0001: Stack and runtime

Date: 26 Nov 2026 · Status: Accepted

## Decision

| Area | Choice |
|---|---|
| Server language | **Go** (module `github.com/JustinTDCT/CineVault`, minimum Go 1.24; the test server runs 1.27) |
| Database | **PostgreSQL** (18 on the test server) |
| Transcoding | **FFmpeg as a separate process** that CineVault runs and controls |
| Web interface | **TypeScript** single-page app served by the Go server. React or Svelte, to be decided when UI work starts. |
| Development | Go server runs **natively** on the test VM; PostgreSQL runs in a **Docker container** |
| Releases and real installs | **Docker Compose** (CineVault + PostgreSQL), with the NVIDIA Container Toolkit for GPU access |
| Default HTTP port | **8456** (`CINEVAULT_LISTEN`) |

## Reasons

- **Go:**
  - builds a single, fast program with low memory use
  - handles many simultaneous jobs well (scans, transcodes, streams)
  - builds easily for Linux, Windows, macOS and ARM NAS devices
  - the archived version (`v2-archive`) was Go, so its code is available for reference
- **PostgreSQL:**
  - copes with many things happening at once: scans, several users, and the editions and collections data
  - SQLite-based servers struggle with this; "a MySQL/Postgres backend" has 663 votes on Jellyfin's request board
  - the paid cache server needs a real database too, so one database type serves both
  - users never set it up by hand, because Compose runs it
- **FFmpeg as a separate process:** the same approach Jellyfin and Emby take. It keeps transcoding swappable, debuggable and isolated from crashes.
- **Native for development, Docker for releases:** fast rebuilds and direct GPU access while developing, and an easy install for users. The Compose setup is tested on the same VM regularly, so it doesn't fall behind.

## Consequences

- **GPU encoders are tested, not assumed.** At startup CineVault runs a two-frame test encode for each NVENC encoder FFmpeg lists (`internal/probe`). FFmpeg lists encoders the hardware may lack (the A2 lists `av1_nvenc` but can't encode AV1), and listed encoders fail when driver libraries are missing.
- **Containers that transcode** need `NVIDIA_DRIVER_CAPABILITIES` to include `video`, as well as `compute,utility`, or NVENC and NVDEC won't be available inside the container.
- **PostgreSQL driver: pgx v5** (`github.com/jackc/pgx/v5`), added with the first schema. Only `internal/db` imports it. The API and other packages depend on plain Go types (`internal/catalog`) and functions, so they can be tested without a database.
- **Schema changes are numbered migrations** (`internal/db/migrations/NNNN_name.sql`), embedded in the binary and applied at startup inside a transaction. A PostgreSQL advisory lock stops two servers applying the same migration at once, and migrations are never edited after release.
