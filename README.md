# CineVault

A self-hosted media server for movies, TV, music, audiobooks and books.

This is a fresh start (November 2026). Earlier work is preserved on these branches:

- `v2-archive`: the previous main branch, archived 26 Nov 2026
- `v1-archive`: the earlier archive

## Status (27 Nov 2026)

**Working and tested on the test server** (10.150.125.60, NVIDIA A2):
- Movies with editions and versions, matched to TMDb; playback with direct play or NVENC HLS conversion, throttled so FFmpeg pauses 3 min ahead
- TV shows with seasons and episodes (owned and missing)
- Music, music videos, audiobooks (chapters, resume, speed), ebooks (EPUB/MOBI/AZW3 reader, PDF in the browser) and comics (CBZ natively; CBR/CB7/CBT need `libarchive-tools`; comics haven't been tested on the server yet)

**Built, not yet tested on the server:** the CineVault Cache client (`internal/cachesync`, migration 0009): lookups after matching, the scheduled update check, the daily share of titles, greyed "Not in library" editions, "What's different in this cut", "Report a mistake", a "Medium confidence" label, and a Settings page. The cache's research worker now fills pending titles on its own (cache build step 4, 27 Nov 2026): *Alien* is published there, and shows up here at the next update check.

**Next, in order:**
1. The cache's `POST /v1/contribute` endpoint, so shared titles are received (in [JustinTDCT/cinevault-cache](https://github.com/JustinTDCT/cinevault-cache))
2. Subtitles (external `.srt` files next to the video first)
3. A Settings section for transcoding (device, throttle values), QSV and VA-API, running as a systemd service, preloading hls.js

## Development quick start

Requires Go 1.25 or newer, Node.js 20.19+ with npm (for the web app), FFmpeg, and the development PostgreSQL container described in [`docs/environment/test-server.md`](docs/environment/test-server.md). That container's credentials live in `~/.config/cinevault/db.env`.

```bash
make test     # unit tests, Go and web (no database needed)
make test-db  # all tests, including database tests on temporary databases
make run      # build the web app and server, then start it against the dev database
```

Open `http://<server>:8456/` in a browser for the web interface.

On startup the server connects to PostgreSQL and applies any pending schema migrations (`internal/db/migrations`), then starts the API and web app. Check the API from another terminal:

```bash
curl -s http://localhost:8456/api/health
curl -s 'http://localhost:8456/api/edition-types?media_type=movie'
```

### Working on the web app

The web app lives in `web/` (React + TypeScript + Vite). For live reloading while editing it, keep the Go server running (`make run`) and in another terminal:

```bash
cd web
npm run dev    # serves on port 5173 and forwards /api to the Go server on 8456
```

### API

| Endpoint | Purpose |
|---|---|
| `GET /api/health` | Server, database and transcoding status |
| `GET /api/edition-types` | The Edition List. Optional `media_type` (`movie`, `tv`, `music`, `audiobook`, `book`) and `include_hidden=true`. |
| `GET /api/libraries` | All libraries |
| `POST /api/libraries` | Add a library: `{"name": "Movies", "media_type": "movie", "paths": ["/media/movies"]}` |
| `POST /api/libraries/{id}/scan` | Scan a library now, then match new titles to TMDb, and return a summary. Movie and TV libraries. |
| `POST /api/libraries/{id}/match` | Match unmatched titles to TMDb without scanning. Titles that failed with an error are retried; ones TMDb didn't find are not. |
| `GET /api/libraries/{id}/titles` | Titles with their editions and versions. Shows list their season and episode counts instead of files. |
| `GET /api/titles/{id}/tracks` | An album's songs (or an audiobook's parts) and their files per edition, with chapter marks |
| `GET /api/titles/{id}/progress`, `PUT …` | Where playback of a title stopped (audiobooks resume from it): `{"version_id": 5, "position_seconds": 812.4}` |
| `GET /api/titles/{id}/episodes` | A show's seasons and episodes with their files, including episodes TMDb lists that the library doesn't have |
| `POST /api/playback` | Start playing a file: `{"version_id": 2, "caps": {...}}`. Returns a direct file URL, or an HLS playlist URL when the file needs converting. |
| `GET /api/playback/{session}/index.m3u8`, `…/segN.ts` | HLS playlist and segments for a session |
| `DELETE /api/playback/{session}` | Stop a session |
| `GET /api/versions/{id}/thumb` | A still from a video file (a third of the way in), made on first request and cached |
| `GET /api/versions/{id}/cover` | Album cover for a music file: `cover.jpg`/`folder.jpg` next to it, or the art embedded in the file |
| `GET /api/versions/{id}/pages/{n}` | Page n (from 0) of a comic, as an image |
| `GET /api/versions/{id}/file` | The original file, with range requests for seeking |
| `GET /api/cache` | CineVault Cache status for Settings: subscription, settings, last check and share, titles per cache state |
| `PUT /api/cache/settings` | `{"check_interval": "24h", "share": true}`; intervals are `1h`, `6h`, `24h`, `168h` or `manual` |
| `POST /api/cache/check` | "Check now": starts a check in the background (202); `GET /api/cache` shows `checking` until it ends |
| `POST /api/titles/{id}/report` | Report a mistake in the cache's information: `{"edition": "directors_cut", "field": "runtime", "message": "…"}` |

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
| `CINEVAULT_FFPROBE` | `ffprobe` | Path to ffprobe, used to read media files |
| `CINEVAULT_TRANSCODE` | `auto` | Video conversion device: `auto` (best working, falling back to CPU), `nvenc` or `cpu` |
| `CINEVAULT_TRANSCODE_GPU` | any | NVIDIA GPU number to use, e.g. `0` |
| `CINEVAULT_TRANSCODE_DIR` | system temp folder | Where stream segments are written while videos play; cleaned up automatically |
| `CINEVAULT_CACHE_DIR` | `~/.cache/cinevault` | Thumbnails and other things CineVault can recreate |
| `CINEVAULT_TRANSCODE_PAUSE_AHEAD` | `3m` | Pause a conversion once it is this far ahead of the player |
| `CINEVAULT_TRANSCODE_RESUME_AHEAD` | `90s` | Resume it when the lead drops to this; must be less than the pause setting |
| `CINEVAULT_TRANSCODE_KEEP_BEHIND` | `3m` | Keep this much already-watched video for quick rewinds; older segments are deleted |
| `CINEVAULT_CACHE_URL` | `https://cache.cine-vault.tv` | The CineVault Cache service |
| `CINEVAULT_CACHE_TOKEN` | none | A CineVault Cache subscription token. Without it the server only shares titles with the cache (if sharing is on in Settings). `make run` reads both from `~/.config/cinevault/cache.env`. |
| `CINEVAULT_TMDB_TOKEN` | none | TMDb API read access token. Without it, titles aren't matched to TMDb. `make run` reads it from `~/.config/cinevault/tmdb.env`. |

### TV file names

A TV library holds one folder per show. The show name comes from that folder (a year in brackets and a `{tmdb-123}` tag are optional), and the season and episode from the file name:

```
3 Body Problem/Season 1/3 Body Problem - S01E01 - Countdown.mkv
Doctor Who (2005) {tmdb-57243}/Season 01/Doctor Who (2005) - S01E01 - Rose.mkv
Lost/Season 1/Lost - S01E01-E02 - Pilot.mkv          (one file, two episodes)
Sherlock/Specials/Sherlock - S00E01 - The Abominable Bride.mkv
```

`S01E01`, `s1e1`, `S01.E01` and `1x01` all work. Files with no season and episode number are listed as scan errors. Subtitle files next to the video (`.srt`) are ignored for now.

### Music and music video file names

Music is read from the files' tags first (album, album artist, track, disc, date, genre). The folders fill any gaps, and an album folder's year and edition win over tags, so one oddly tagged song can't split an album:

```
3 Doors Down/Away From the Sun (2002)/3 Doors Down - 01 - When I'm Gone.flac
Adele/25 (Deluxe Edition) (2015)/01 Hello.flac       (a Deluxe edition of 25)
Pink Floyd/The Wall (1979)/CD2/01 - Hey You.mp3      (disc 2)
```

Music videos are named `Artist - Song (Edition).mp4`, for example `Alice In Chains - No Excuses (MTV Unplugged).mp4`, or kept in an `Artist/` folder. Brackets that name an edition (Live, Unplugged, Lyric Video, U.S. Version...) make an edition of the same song.

### Audiobook file names

An `.m4b` (or any file with chapters) is a whole book; a folder of MP3s is one book whose files are its parts. Tags come first (album or title, author, narrator in `composer` or `narrator`, description). Folders fill the gaps: the top folder is the author and the one nearest the book a series, and a single folder counts as a series only when it's named like one:

```
The John Matherson Series/One Second After.m4b           (series; author from the tags)
Brandon Sanderson/Mistborn/The Final Empire/01 - ….mp3   (author / series / book of parts)
J.R.R. Tolkien - [The Lord of the Rings 02] - The Fellowship of the Ring.m4b   (series and number)
```

### Ebooks

EPUB, AZW3, MOBI and FB2 read in the browser (with [foliate-js](https://github.com/johnfactotum/foliate-js)); PDFs open in the browser's own viewer. Title, author, series and cover come from the book file itself (the EPUB package, the AZW3 header), then from names like `Author/Author - [Series 05] - Title.epub`. Copies of one book in several formats are listed together, and the reading position is saved per book.

### Comics

A comic library holds one folder per series, with one file per issue: `Aliens - Book One/Aliens - Book One - 1.cbr`. The issue number comes from the end of the name (`- 2`, `#002`, `Issue 12`), and a `ComicInfo.xml` inside the archive wins when there is one. CBZ works out of the box; CBR, CB7 and CBT need `bsdtar` (`sudo apt install libarchive-tools`), and are unpacked into the cache on first read so pages turn quickly (the 12 most recent are kept).

## Project layout

| Path | Contents |
|---|---|
| `cmd/cinevault` | Server entry point |
| `web/` | Web app (React + TypeScript), built into `internal/webui/static` |
| `internal/catalog` | Library data types shared by the database and the API |
| `internal/config` | Settings from environment variables |
| `internal/db` | PostgreSQL connection, migrations and queries |
| `internal/match` | Matches titles to TMDb (title and year rules) |
| `internal/mediainfo` | Reads codecs, resolution, HDR and audio with ffprobe |
| `internal/probe` | FFmpeg, GPU and hardware-encoder detection |
| `internal/scan` | Library scanner and file-name parser (title, year, edition) |
| `internal/server` | HTTP API |
| `internal/tmdb` | TMDb API client |
| `internal/transcode` | Direct-play decision, FFmpeg commands, HLS streaming sessions |
| `internal/webui` | Serves the built web app from the server binary |
| `internal/cachesync` | The CineVault Cache client: lookups after matching, the scheduled update check, and the daily share of titles |
| `internal/comic`, `internal/ebook`, `internal/thumb` | Comic archives (pages), ebook metadata and covers, video stills and cover images |
| `pkg/editionlist` | The built-in Edition List as Go data, shared with the CineVault Cache service; a database test keeps it in step with the migrations |
| `pkg/theme` | The design system "CineVault" (look D · Widescreen): tokens, the shared stylesheet and the bundled fonts, used by the web app and by the CineVault Cache's review UI |
| `pkg/cacheapi` | The CineVault Cache API contract (v1): JSON types, validation rules, a client, and golden files in `testdata/` that pin the wire format |

## Design docs

- Decisions: [`docs/decisions`](docs/decisions/)
- Feature specs: [`docs/specs`](docs/specs/README.md)
- Feature research (Plex, Emby, Jellyfin and user requests): [`docs/research/feature-master-list.md`](docs/research/feature-master-list.md)
- Test server setup (Hyper-V VM with NVIDIA A2): [`docs/environment/test-server.md`](docs/environment/test-server.md)

## Data sources

Movie metadata and images come from [TMDb](https://www.themoviedb.org). This product uses the TMDB API but is not endorsed or certified by TMDB.
