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
| Streaming | Jellyfin FFmpeg (HLS transcoding, MPEG-TS remux, direct play) |
| Frontend | Single-page HTML app, mpegts.js, HLS.js, SortableJS, Chart.js |
| Deployment | Docker Compose with optional hardware acceleration |
| CI/CD | GitHub Actions → GitHub Container Registry |
| PWA | Installable Progressive Web App with offline support |

## Features at a Glance

### Media Management
9 media types (Movies, Adult Movies, TV Shows, Music, Music Videos, Home Videos, Other Videos, Photos, Audiobooks), multi-folder libraries with per-user access control, TV show hierarchy auto-created from file paths, edition groups, sister groups, drag-and-drop sort ordering, and extras detection (trailers, featurettes, behind-the-scenes).

See [docs/FILE-PARSING.MD](docs/FILE-PARSING.MD) for filename conventions, folder structure, and the ingestion pipeline.

### Playback and Streaming
Three playback modes: Direct Play for natively compatible formats (MP4/WebM), MPEG-TS remux for MKV/AVI via FFmpeg, and HLS transcoding with hardware acceleration (NVENC, QSV, VAAPI) and quality selection (360p through 4K). Subtitle support with WebVTT conversion, burn-in for image-based subs, HDR-to-SDR tone mapping, and multi-audio stream selection.

See [docs/STREAMING.MD](docs/STREAMING.MD) for the full streaming architecture, codec support, and hardware acceleration setup.

### Metadata and Enrichment
Cache-first architecture pulling from TMDB, OMDb, fanart.tv, TVDB, MusicBrainz, AniList, OpenLibrary, and PornDB. NFO import/export (Kodi-compatible), per-field metadata locking, auto-collections from TMDB franchise data, mood tagging derived from TMDB keywords, inline provider ID extraction from filenames, and local artwork detection (Plex/Jellyfin/Kodi naming conventions).

See [docs/METADATA.MD](docs/METADATA.MD) for the full metadata flow, cache server architecture, and field reference.

### Duplicate Detection
Perceptual hashing (pHash) and audio fingerprinting with similarity scoring. Review queue with side-by-side comparison and resolution options (merge as edition, delete, ignore, sister group).

### Filtering and Search
Library-level dropdown filters (genre, year, rating, mood, resolution, source type, codec, HDR format, audio codec, bitrate range, dynamic range, edition, content rating, and folder) combined with AND logic. Global cross-library text search, missing episode detection for TV shows, A-Z letter index navigation, and saved filter presets.

See [docs/FILTER-SEARCH.MD](docs/FILTER-SEARCH.MD) for filter mechanics, query building, and the duplicate finder.

### Collections
Manual and smart collections with nesting support. Smart collections use JSONB rule sets that re-evaluate dynamically. Movie series auto-created from TMDB collection data. Collection statistics, template presets, and full CRUD API.

See [docs/COLLECTIONS.MD](docs/COLLECTIONS.MD) for rule syntax, hierarchy model, and endpoints.

### Recommendations and Discovery
Personalized scoring based on recency-weighted genre affinity and cast/director overlap. "Because You Watched" similarity rows. Mood tags mapped from TMDB keywords. Smart collections as dynamic playlists. Discovery hubs for trending content, genre browsing, and decade browsing.

See [docs/RECOMMENDATIONS.MD](docs/RECOMMENDATIONS.MD) for the scoring algorithm, mood mapping, and filters.

### Intro and Credits Detection
Audio fingerprint cross-episode intro detection, black-frame/silence credits detection, anime OP/ED heuristics, and recap scene-change density analysis. Per-user auto-skip preferences with Netflix-style skip button overlay.

See [docs/INTRO-CREDITS.MD](docs/INTRO-CREDITS.MD) for detection methods, confidence levels, and player behavior.

### Users and Households
Multi-user with roles (Admin / User / Guest), JWT auth with TOTP two-factor authentication, PIN-based fast login, master/sub-profile household system, parental controls with content rating caps, kids mode, per-library access control, per-user streaming limits, session management, and admin-generated password reset tokens.

See [docs/USERS.MD](docs/USERS.MD) for authentication flows, household architecture, and permissions.

### Watch History and Playback
Per-user watch progress tracking, continue watching and on-deck queues, user playback preferences (quality, subtitle/audio language, auto-play), user ratings, watchlist, favorites, and playlists with custom ordering.

See [docs/PLAYBACK.MD](docs/PLAYBACK.MD) for watch tracking, playlists, preferences, and cinema mode.

### SyncPlay (Watch Together)
Host-controlled synchronized playback sessions with invite codes. Real-time play/pause/seek sync via WebSocket. In-session chat. Participant management.

See [docs/PLAYBACK.MD](docs/PLAYBACK.MD) for SyncPlay details.

### Cinema Mode
Pre-roll management for custom intro videos before features. Cinema queue builder that combines pre-rolls, trailers, and the main feature into a seamless playback sequence.

### Chromecast and DLNA
Chromecast session tracking with remote control (play/pause/seek/volume). UPnP/DLNA server with SSDP discovery and ContentDirectory service for network media players.

### Analytics and Monitoring
Real-time stream and transcode tracking, system metrics polling (CPU, RAM, GPU, disk), nightly rollup aggregation, configurable alert rules with webhook delivery (Discord, Slack, generic), and a Chart.js admin dashboard with activity feeds, library health reports, and trend charts.

See [docs/ANALYTICS-DASHBOARD.MD](docs/ANALYTICS-DASHBOARD.MD) for data collection, alerting, and dashboard components.

### External Integrations
Trakt.tv scrobbling with device-based OAuth. Last.fm scrobbling for music. Sonarr/Radarr/Lidarr webhook receiver for automatic library updates. Content request system for users to request new media.

### Preview Generation
Automatic thumbnail extraction, scrubber sprite sheets (hover previews), and animated preview clips (8-segment looping MP4) with hardware-accelerated encoding.

### Backup and Restore
Database backup creation with download support. Import tools for migrating from other media servers.

### Live TV and DVR (Experimental)
Tuner device management, EPG guide data, and DVR recording scheduling.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/JustinTDCT/CineVault.git
cd CineVault

# Copy environment template and configure
cp docker/.env.example docker/.env
# Edit docker/.env with your settings (media paths, API keys, etc.)

# Start all services
docker compose -f docker/docker-compose.yml up -d

# Migrations are applied automatically by the entrypoint script
# Open http://localhost:8080 and complete the setup wizard
```

For hardware acceleration, use the overlay compose file:

```bash
docker compose -f docker/docker-compose.yml -f docker/docker-compose.hwaccel.yml up -d
```

See [docs/DEPLOYMENT.MD](docs/DEPLOYMENT.MD) for full deployment instructions, environment variables, and hardware acceleration setup.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `cinevault` | Database user |
| `DB_PASSWORD` | `cinevault_dev_pass` | Database password |
| `DB_NAME` | `cinevault` | Database name |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | (empty) | Redis password |
| `SERVER_HOST` | `0.0.0.0` | HTTP listen address |
| `SERVER_PORT` | `8080` | HTTP server port |
| `JWT_SECRET` | (dev key) | JWT signing secret — **change in production** |
| `JWT_EXPIRES_IN` | `24h` | JWT token expiration |
| `JWT_REFRESH_EXPIRES_IN` | `168h` | Refresh token expiration |
| `MEDIA_PATH` | `/media` | Base media directory |
| `PREVIEW_PATH` | `/previews` | Preview file output directory |
| `THUMBNAIL_PATH` | `/thumbnails` | Thumbnail output directory |
| `TMDB_API_KEY` | (empty) | TMDB API key for metadata |
| `FFMPEG_PATH` | `/usr/lib/jellyfin-ffmpeg/ffmpeg` | FFmpeg binary path |
| `FFPROBE_PATH` | `/usr/lib/jellyfin-ffmpeg/ffprobe` | FFprobe binary path |

## Documentation

| Document | Description |
|----------|-------------|
| [FILE-PARSING.MD](docs/FILE-PARSING.MD) | Filename parsing, folder structure, ingestion pipeline |
| [METADATA.MD](docs/METADATA.MD) | Metadata architecture, cache server, enrichment flow |
| [STREAMING.MD](docs/STREAMING.MD) | Streaming modes, transcoding, hardware acceleration, subtitles |
| [DEPLOYMENT.MD](docs/DEPLOYMENT.MD) | Docker setup, environment variables, CI/CD, hardware acceleration |
| [PLAYBACK.MD](docs/PLAYBACK.MD) | Watch history, playlists, SyncPlay, cinema mode, preferences |
| [USERS.MD](docs/USERS.MD) | Authentication, households, parental controls, 2FA |
| [COLLECTIONS.MD](docs/COLLECTIONS.MD) | Manual/smart collections, movie series, nesting |
| [RECOMMENDATIONS.MD](docs/RECOMMENDATIONS.MD) | Recommendation engine, mood tags, discovery hubs |
| [FILTER-SEARCH.MD](docs/FILTER-SEARCH.MD) | Library filters, global search, duplicate finder |
| [ANALYTICS-DASHBOARD.MD](docs/ANALYTICS-DASHBOARD.MD) | Analytics, monitoring, alerting, dashboard |
| [INTRO-CREDITS.MD](docs/INTRO-CREDITS.MD) | Skip detection, auto-skip, player integration |

## License

This project is free for personal use. See the repository for details.
