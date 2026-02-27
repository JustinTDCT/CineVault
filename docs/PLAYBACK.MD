# CineVault Playback and User Features

This document describes watch tracking, continue watching, playlists, watchlist, favorites, user ratings, playback preferences, SyncPlay (Watch Together), cinema mode, and display preferences.

---

## Watch History

CineVault tracks per-user watch progress for every media item. Progress is recorded automatically during playback and persisted to the database.

### How It Works

1. During playback, the player periodically sends progress updates
2. Each update records the current position, total duration, and completion status
3. A watch history entry is created or updated (upsert by user + media item)
4. An item is marked as completed when the user reaches the end or manually marks it

### Watch History Model

| Field | Type | Description |
|---|---|---|
| `user_id` | UUID | The user watching |
| `media_item_id` | UUID | The media item being watched |
| `edition_group_id` | UUID (nullable) | Edition group context (if applicable) |
| `progress_seconds` | int | Current playback position |
| `duration_seconds` | int | Total duration of the item |
| `completed` | boolean | Whether the item has been fully watched |
| `last_watched_at` | timestamp | When the user last watched this item |

---

## Continue Watching

The continue watching queue surfaces items the user has started but not finished.

`GET /api/v1/watch/continue`

Returns the top 20 items where the user has progress but `completed = false`, ordered by `last_watched_at` descending (most recently watched first).

Each item includes the media metadata (title, poster, year, etc.) alongside the progress information, allowing the UI to show a progress bar overlay on cards.

---

## On Deck

The on-deck endpoint provides the next episode to watch for TV shows.

`GET /api/v1/watch/on-deck`

Returns unwatched episodes from shows the user is actively watching, prioritizing the next sequential episode after the most recently completed one.

---

## Watchlist

The watchlist is a per-user list of items they want to watch later. It functions as a simple bookmark system.

### API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/watchlist` | User | Get all watchlist items |
| POST | `/api/v1/watchlist/{itemId}` | User | Add an item to the watchlist |
| DELETE | `/api/v1/watchlist/{itemId}` | User | Remove an item from the watchlist |
| GET | `/api/v1/watchlist/{itemId}/check` | User | Check if an item is on the watchlist |

The watchlist appears on the home page and is accessible from media detail views.

---

## Favorites

Favorites are a per-user toggle on media items. Favoriting an item marks it for quick access.

### API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/favorites/{itemId}` | User | Toggle favorite status |
| GET | `/api/v1/favorites/{itemId}/check` | User | Check if an item is favorited |
| GET | `/api/v1/favorites` | User | Get all favorited items |

---

## User Ratings

Users can rate media items on a personal scale. Ratings are separate from TMDB/IMDB ratings and represent the user's own opinion.

### API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/media/{id}/rating` | User | Rate a media item |
| GET | `/api/v1/media/{id}/rating` | User | Get the user's rating for an item |
| DELETE | `/api/v1/media/{id}/rating` | User | Remove the user's rating |
| GET | `/api/v1/media/{id}/community-rating` | User | Get the aggregate community rating from all users |

---

## Playlists

Playlists are user-created ordered lists of media items for sequential playback.

### API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/playlists` | User | List all playlists |
| POST | `/api/v1/playlists` | User | Create a playlist |
| PUT | `/api/v1/playlists/{id}` | User | Update playlist metadata |
| DELETE | `/api/v1/playlists/{id}` | User | Delete a playlist |
| GET | `/api/v1/playlists/{id}/items` | User | Get playlist items in order |
| POST | `/api/v1/playlists/{id}/items` | User | Add an item to the playlist |
| DELETE | `/api/v1/playlists/{id}/items/{itemId}` | User | Remove an item from the playlist |
| PUT | `/api/v1/playlists/{id}/reorder` | User | Reorder playlist items |

Playlists differ from collections in that they are designed for sequential playback with auto-play-next behavior, while collections are organizational groupings.

---

## Playback Preferences

Each user has configurable playback preferences that control default behavior:

`GET /api/v1/settings/playback` / `PUT /api/v1/settings/playback`

### Preference Fields

| Field | Type | Description |
|---|---|---|
| `playback_mode` | string | Default edition selection: `always_ask`, `play_default`, `highest_quality`, `lowest_quality`, `last_played` |
| `preferred_quality` | string | Default transcode quality (e.g., `1080p`) |
| `auto_play_next` | boolean | Automatically play the next episode |
| `subtitle_language` | string | Preferred subtitle language (ISO code) |
| `audio_language` | string | Preferred audio language (ISO code) |

---

## Display Preferences

Users can customize UI display behavior:

`GET /api/v1/settings/display` / `PUT /api/v1/settings/display`

Display preferences are stored as a JSON object (`overlay_settings`) allowing flexible UI customization without schema changes.

---

## Home Layout

Users can customize the layout of their home page:

`GET /api/v1/settings/home-layout` / `PUT /api/v1/settings/home-layout`

The home layout controls which sections appear, their order, and visibility.

---

## SyncPlay (Watch Together)

SyncPlay enables multiple users to watch the same content in sync, with one host controlling playback for all participants.

### How It Works

1. **Host creates a session** — generates an 8-character hex invite code
2. **Participants join** — enter the invite code to join the session
3. **Host controls playback** — play, pause, and seek actions are broadcast to all participants via WebSocket
4. **Chat** — participants can send messages during the session
5. **Session ends** — host or timeout ends the session

### Session Lifecycle

```
Host: POST /api/v1/sync/create
  → Returns session ID + invite code
  → Session state: "waiting"

Participant: POST /api/v1/sync/join
  → Validates invite code
  → Adds user to participant list
  → Broadcasts join event via WebSocket

Host: POST /api/v1/sync/{id}/action
  → Actions: play, pause, seek
  → Broadcasts action to all participants
  → Participants sync playback position

Any: POST /api/v1/sync/{id}/chat
  → Stores message
  → Broadcasts to all participants

Host: DELETE /api/v1/sync/{id}
  → Marks session as ended
  → Broadcasts end event
```

### API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/sync/create` | User | Create a sync session (becomes host) |
| POST | `/api/v1/sync/join` | User | Join a session by invite code |
| POST | `/api/v1/sync/{id}/action` | User | Send playback action (host only) |
| POST | `/api/v1/sync/{id}/chat` | User | Send a chat message |
| GET | `/api/v1/sync/{id}` | User | Get session info (participants, chat history) |
| DELETE | `/api/v1/sync/{id}` | User | End the session |

### WebSocket Integration

SyncPlay actions are broadcast via the existing WebSocket hub (`/api/v1/ws`). Participants receive real-time updates for:

- Playback state changes (play, pause, seek)
- New chat messages
- Participant join/leave events
- Session end notification

---

## Cinema Mode

Cinema mode creates a theater-like experience by combining pre-rolls, trailers, and the main feature into a seamless queue.

### Pre-Rolls

Pre-rolls are custom intro videos (studio logos, custom animations, etc.) that play before the feature.

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/cinema/pre-rolls` | Admin | List configured pre-rolls |
| POST | `/api/v1/cinema/pre-rolls` | Admin | Add a pre-roll video |
| DELETE | `/api/v1/cinema/pre-rolls/{id}` | Admin | Remove a pre-roll |

### Cinema Queue

The cinema queue endpoint builds a complete playback sequence for a media item:

`GET /api/v1/cinema/queue/{mediaId}`

Returns an ordered list combining:
1. Pre-roll videos (configured by admin)
2. Trailers (from TMDB trailer URL or related content)
3. The main feature

The player processes the queue sequentially, providing a cinema-like experience.

---

## Chromecast Support

CineVault tracks Chromecast casting sessions with remote control capabilities:

### API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/cast/session` | User | Create or update a cast session |
| GET | `/api/v1/cast/sessions` | User | List active cast sessions |
| DELETE | `/api/v1/cast/{id}` | User | End a cast session |
| PUT | `/api/v1/cast/session/{id}/command` | User | Remote control command |

### Commands

| Command | Description |
|---|---|
| `play` | Resume playback |
| `pause` | Pause playback |
| `seek` | Seek to a position |
| `volume` | Set volume level |
| `stop` | Stop playback |

Cast sessions track the device name, device type, playback state, current position, and duration.

---

## DLNA / UPnP

CineVault includes a DLNA server for streaming to network media players (smart TVs, game consoles, etc.).

### Configuration

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/dlna/config` | Admin | Get DLNA settings |
| PUT | `/api/v1/dlna/config` | Admin | Update DLNA settings (enabled, friendly name, port) |

### SSDP Discovery

When enabled, CineVault advertises itself on the local network via SSDP multicast (239.255.255.250:1900). DLNA clients can discover the server automatically.

### Content Directory

The UPnP ContentDirectory service exposes:
- Libraries as containers
- Media items as playable items

Endpoints:
- `GET /dlna/description.xml` — UPnP device description
- `GET /dlna/content/{id}` — Browse content directory

---

## Lyrics

For music content, CineVault can serve lyrics:

`GET /api/v1/media/{id}/lyrics`

Returns lyrics data associated with the media item (if available).

---

## Content Requests

Users can request new content to be added to the server:

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/requests` | User | Submit a content request |
| GET | `/api/v1/requests/mine` | User | View own requests |
| GET | `/api/v1/requests` | Admin | View all requests |
| PUT | `/api/v1/requests/{id}` | Admin | Resolve a request (approve/deny) |

---

## External Integrations

### Trakt.tv

Scrobble watch activity to Trakt.tv for external tracking and social features.

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/trakt/device-code` | User | Get device authorization code |
| POST | `/api/v1/trakt/activate` | User | Activate device after authorization |
| GET | `/api/v1/trakt/status` | User | Check connection status |
| DELETE | `/api/v1/trakt/disconnect` | User | Disconnect Trakt account |
| POST | `/api/v1/trakt/scrobble` | User | Scrobble a watch event |

### Last.fm

Scrobble music playback to Last.fm.

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/lastfm/connect` | User | Connect Last.fm account |
| GET | `/api/v1/lastfm/status` | User | Check connection status |
| DELETE | `/api/v1/lastfm/disconnect` | User | Disconnect Last.fm account |
| POST | `/api/v1/lastfm/scrobble` | User | Scrobble a music play |

### Sonarr / Radarr / Lidarr Webhooks

CineVault can receive webhook notifications from *arr applications to trigger automatic library scans when new content is downloaded.

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/webhooks/arr` | None (secret) | Receive *arr webhook (no auth, uses webhook secret) |
| GET | `/api/v1/admin/webhooks` | Admin | List webhook secrets |
| POST | `/api/v1/admin/webhooks` | Admin | Create a webhook secret |
| DELETE | `/api/v1/admin/webhooks/{id}` | Admin | Delete a webhook secret |

---

## File Locations

| Component | Path |
|---|---|
| Watch history handlers | `internal/api/handlers_watch.go` |
| Watch history repository | `internal/repository/watch_history_repository.go` |
| SyncPlay handlers | `internal/api/handlers_syncplay.go` |
| Casting handlers | `internal/api/handlers_casting.go` |
| Display preference handlers | `internal/api/handlers_display.go` |
| Engagement handlers | `internal/api/handlers_engagement.go` |
| Integration handlers | `internal/api/handlers_integrations.go` |
| DLNA SSDP server | `internal/dlna/ssdp.go` |
| DLNA content directory | `internal/dlna/contentdirectory.go` |
| DLNA connection manager | `internal/dlna/connectionmanager.go` |
| WebSocket hub | `internal/api/websocket.go` |
| Player UI | `web/js/player.js` |
| Sync UI | `web/js/sync.js` |
| Music features | `web/js/music.js` |
| Engagement UI | `web/js/engagement.js` |
