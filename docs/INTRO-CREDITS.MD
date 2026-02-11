# CineVault Skip Detection System

This document describes how CineVault detects and skips intros, credits, recaps, and anime OP/ED sequences.

---

## Overview

CineVault's skip detection system has four components:

1. **Detection Engine** — FFmpeg-powered algorithms that identify skippable segments
2. **Segment Storage** — database-backed markers with timestamps, confidence, and source tracking
3. **Player Integration** — Netflix-style skip button overlay with auto-skip support
4. **Per-User Preferences** — individual toggle controls for skip behavior

```
Library Scan → Detection Job → Segments Stored → Player Loads Segments
                                                       ↓
                                           Show Skip Button / Auto-Skip
```

---

## Detection Algorithms

### 1. Cross-Episode Intro Detection (Audio Fingerprinting)

**Target:** TV show intros that repeat across episodes in a season.

**How it works:**

1. For each episode in a season, extract audio fingerprints from the first 5 minutes
2. Audio is analyzed in 15-second sliding windows using FFmpeg's `astats` filter
3. Each window produces an MD5 hash of the audio statistics output
4. Hashes are compared across all episodes in the season
5. Windows appearing in ≥60% of episodes are flagged as intro content
6. Contiguous matching windows are merged into a single intro region (with a 2-second gap tolerance)

**Requirements:**
- At least 2 episodes in a season
- Detected intro must be ≥15 seconds long

**Confidence:** Based on the ratio of matching windows to total analyzed windows. Higher when more episodes share the same audio pattern.

```
Episode 1:  [window1][window2][window3][window4]...
Episode 2:  [window1][window2][window3][window4]...
Episode 3:  [window1][window2][window3][window4]...
                 ↓         ↓        ↓
            Compare hashes across episodes
                 ↓
            Windows in 60%+ episodes → INTRO
```

### 2. Credits Detection (Black Frame + Silence)

**Target:** End credits in movies and TV episodes.

**How it works:**

1. Only analyze the last 20% of the video (credits are always at the end)
2. Run FFmpeg `blackdetect` filter to find black frame regions (≥0.5s duration, pixel threshold 0.10)
3. Run FFmpeg `silencedetect` filter to find silence regions (≥2s duration, noise floor -50dB)
4. Look for black frame + silence combinations within 5 seconds of each other
5. The earliest qualifying black frame marks the start of credits
6. Fallback: if no black frames found, use the first significant silence (≥2s)

**Validation rules:**
- Credits must be 30 seconds to 10 minutes from the end of the video
- Videos shorter than 2 minutes are skipped

**Confidence:**
- 0.85 when both black frames and silence are detected together
- 0.70 when only one signal is found

```
Video: [════════════════════|████████████████████]
                            ↑ Last 20% analyzed
                            
FFmpeg blackdetect  → [black_start:1842.5 black_end:1844.0]
FFmpeg silencedetect → [silence_start:1843.0 silence_end:1846.0]
                            ↓
                    Credits start: 1842s
```

### 3. Anime OP/ED Detection (Timing Heuristics)

**Target:** Anime opening and ending sequences, which follow predictable timing patterns.

**Key insight:** Anime OPs are almost always 85-95 seconds (1:25-1:35), and so are EDs. This is an industry standard driven by broadcast slot formatting.

**OP detection:**

1. Analyze silence boundaries in the first 4 minutes
2. Look for a silence event around the 85-95 second mark (standard OP ending)
3. If found: OP = 0:00 to that silence point
4. Also handles cold opens: look for a silence at 30-120s (cold open end), then another silence ~90 seconds later (OP end)

**ED detection:**

1. Analyze silence boundaries in the last 6 minutes
2. Look for a silence event where the remaining video is 80-120 seconds (standard ED length)
3. If found: ED = that silence point to end of video

**Anime content identification:**
- `original_language` is "ja" or "japanese"
- Keywords contain "anime" or "animation"

**Confidence:** 0.70-0.75 (heuristic-based, not fingerprint-matched)

```
Standard anime episode (24 min):

[Cold Open?][    OP ~90s    ][   Episode Content   ][    ED ~90s    ][Preview?]
 0:00-1:30   1:30-3:00        3:00-21:00             21:00-22:30     22:30-24:00
```

### 4. Recap Detection (Scene-Change Density)

**Target:** "Previously on..." recap segments that appear before intros in episodes 2+.

**How it works:**

1. Only runs on episodes with `episode_number > 1`
2. Analyzes the pre-intro section (or first 3 minutes if no intro detected)
3. Uses FFmpeg scene change filter (`select='gt(scene,0.3)'`) to detect cuts
4. Calculates scene-change density (cuts per second) in 30-second sliding windows
5. Recaps are identified by high density (≥0.4 changes/second = rapid cuts)
6. The densest window is selected and extended to cover the full recap region

**Requirements:**
- At least 5 scene changes in the analyzed region
- Overall density ≥0.3 changes/second
- Best window density ≥0.4 changes/second
- Recap must be ≥10 seconds long

**Confidence:** Capped at 0.6 (lowest of all detection types — recaps are the hardest to detect reliably)

---

## Database Schema

### media_segments

Stores detected or manually-set skip markers.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| media_item_id | UUID | FK to media_items |
| segment_type | VARCHAR(20) | `intro`, `credits`, `recap`, or `preview` |
| start_seconds | DOUBLE | Start timestamp in seconds |
| end_seconds | DOUBLE | End timestamp in seconds |
| confidence | DOUBLE | Detection confidence (0.0-1.0) |
| source | VARCHAR(20) | `auto`, `manual`, or `community` |
| verified | BOOLEAN | Admin-verified flag |

Unique constraint on `(media_item_id, segment_type)` — one segment per type per item.

### user_skip_preferences

Per-user auto-skip toggle settings.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| user_id | UUID | FK to users |
| skip_intros | BOOLEAN | Auto-skip intros (default: false) |
| skip_credits | BOOLEAN | Auto-skip credits (default: false) |
| skip_recaps | BOOLEAN | Auto-skip recaps (default: false) |
| show_skip_button | BOOLEAN | Show skip button overlay (default: true) |

Unique constraint on `user_id`.

---

## API Endpoints

### Segments

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/v1/media/{mediaId}/segments` | User | Get all segments for a media item |
| POST | `/api/v1/media/{mediaId}/segments` | Admin | Add/update a segment (manual) |
| DELETE | `/api/v1/media/{mediaId}/segments/{type}` | Admin | Delete a segment by type |
| POST | `/api/v1/libraries/{id}/detect-segments` | Admin | Trigger detection job for a library |

### Skip Preferences

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/v1/settings/skip` | User | Get current user's skip preferences |
| PUT | `/api/v1/settings/skip` | User | Update skip preferences |

### POST /api/v1/media/{mediaId}/segments (request body)

```json
{
    "segment_type": "intro",
    "start_seconds": 5.0,
    "end_seconds": 92.0,
    "verified": true
}
```

### PUT /api/v1/settings/skip (request body)

```json
{
    "skip_intros": true,
    "skip_credits": false,
    "skip_recaps": false,
    "show_skip_button": true
}
```

---

## Background Job

Detection runs as an Asynq background job (`detect:segments`) with two phases:

### Phase 1: Cross-Episode Intro Detection (0-40% progress)

- Lists all distinct seasons in the library
- For each season, loads all episodes and runs the audio fingerprint comparison
- Detected intros are saved for all matching episodes

### Phase 2: Per-Item Detection (40-100% progress)

- Lists all movies and TV episodes without existing segments
- For each item, runs `DetectAll`:
  - Anime content → anime OP/ED detection
  - All content → credits detection (black frame + silence)
  - TV episodes (ep 2+) → recap detection (scene-change density)
- Segments are upserted (won't overwrite manual entries for the same type)

The job is triggered from Settings → Skip → Detect Segments, or via API. Progress is broadcast over WebSocket.

---

## Player Behavior

### Skip Button

When a media item plays, the player loads its segments from the API. During playback:

1. The `timeupdate` event checks the current position against all segments
2. When the playhead enters a segment region (or is 2 seconds before it), the skip button appears
3. The button shows context-specific text: "Skip Intro", "Skip Credits", or "Skip Recap"
4. Clicking the button seeks to the end of the segment
5. The button disappears when the playhead leaves the segment region

### Auto-Skip

If the user has enabled auto-skip for a segment type (e.g., `skip_intros: true`):

1. When the playhead enters the segment, the player automatically seeks to the end
2. No button is shown — the skip happens silently
3. Works for both direct play and MPEGTS streaming (restarts stream at new position)

### Manual Override

Auto-skip only triggers once per segment per playback session. If the user manually seeks back into a segment, the skip button appears instead of auto-skipping again.

---

## Detection Priority

When multiple detection methods could apply, the system uses this priority:

1. **Manual segments** (source = `manual`) are never overwritten by auto-detection
2. **Anime OP/ED detection** takes priority over generic intro/credits detection for anime content
3. **Credits detection** won't create a duplicate if anime ED was already detected
4. **Recap detection** only runs after intro detection (it needs the intro start time as a boundary)

---

## Confidence Levels

| Detection Method | Typical Confidence | Notes |
|-----------------|-------------------|-------|
| Cross-episode audio fingerprint (intro) | 0.6-1.0 | Higher with more episodes sharing the pattern |
| Black frame + silence (credits) | 0.7-0.85 | 0.85 when both signals present |
| Anime OP heuristic | 0.70-0.75 | Standard OP at 0:00, 0.70 for cold-open variant |
| Anime ED heuristic | 0.70 | Based on timing pattern match |
| Recap scene-change density | 0.3-0.6 | Capped at 0.6, least reliable |
| Manual entry | 1.0 | Admin-verified |

---

## Settings UI

### User Settings (Settings → Skip)

All users can configure:
- **Show Skip Button** — toggle the overlay button on/off
- **Auto-skip Intros** — automatically skip detected intros
- **Auto-skip Credits** — automatically skip detected credits
- **Auto-skip Recaps** — automatically skip detected recaps

### Admin Panel (Settings → Skip)

Admins also see a library list with "Detect Segments" buttons to trigger background detection jobs for TV and movie libraries.

### Media Detail (Segments Tab)

Each media item's detail view has a "Segments" tab showing:
- All detected segments with type, start/end times, confidence, and source
- Delete button per segment (admin only)
- "Add Segment" form for manual entry (admin only)

---

## File Locations

| Component | Path |
|-----------|------|
| Detection engine | `internal/detection/detector.go` |
| Repository (DB layer) | `internal/repository/segment_repository.go` |
| API handlers | `internal/api/handlers_segments.go` |
| Job handler | `internal/jobs/tasks.go` (DetectSegmentsHandler) |
| Models | `internal/models/models.go` (MediaSegment, UserSkipPreference) |
| Migration | `migrations/022_media_segments.up.sql` |
| Player UI | `web/index.html` (skip button, checkSegments) |
| Settings UI | `web/settings.html` (skip section) |
| CSS | `web/styles.css` (skip-segment-btn) |
