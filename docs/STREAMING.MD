# CineVault Streaming System

This document describes how CineVault handles media playback, including direct play, remuxing, HLS transcoding, hardware acceleration, subtitle handling, and quality selection.

---

## Overview

CineVault supports three playback modes, selected automatically based on the media container and codec:

1. **Direct Play** — native formats (MP4/WebM/M4V) served with HTTP range requests, zero server overhead
2. **Direct Stream (Remux)** — non-native containers (MKV/AVI/etc.) remuxed to MPEG-TS on-the-fly, video is copied without re-encoding
3. **Transcode** — HLS segmented output with video re-encoding, quality selection, and optional hardware acceleration

```
Request arrives
  │
  ├─ Quality parameter present? → Transcode (HLS)
  │
  └─ No quality selected
      │
      ├─ MP4/WebM/M4V container? → Direct Play (HTTP Range)
      │
      └─ MKV/AVI/other container
          │
          ├─ Audio browser-compatible? → Remux (video copy, audio copy)
          │
          └─ Audio not compatible → Remux (video copy, audio transcode)
```

---

## Direct Play

Direct play serves the file as-is with full HTTP Range request support (206 Partial Content). This is the most efficient mode — no FFmpeg process, no CPU usage.

**Eligible formats:** `.mp4`, `.webm`, `.m4v`

**How it works:**

1. Client requests `GET /api/v1/stream/{mediaId}/direct`
2. Server checks the container format
3. If natively compatible, serves the file with `Content-Type` based on extension
4. Range headers are parsed and honored for seeking support
5. Client uses the browser's native `<video>` element

**Seeking:** Handled by the browser via HTTP Range requests. The server responds with 206 Partial Content and the requested byte range.

---

## Direct Stream (MPEG-TS Remux)

When the container is not browser-native but the video codec is compatible, CineVault remuxes the file to MPEG-TS on-the-fly using FFmpeg. Video is copied (`-c:v copy`), and audio is copied if compatible or transcoded if not.

**Triggers:** Any non-MP4/WebM/M4V container (MKV, AVI, MOV, etc.)

**How it works:**

1. Client requests `GET /api/v1/stream/{mediaId}/direct`
2. Server detects the container needs remuxing
3. FFmpeg starts with `-c:v copy` and streams MPEG-TS directly to the HTTP response
4. Client uses mpegts.js (MSE-based) for playback

**Audio handling:**

| Source Audio | Action | Output |
|---|---|---|
| AAC, MP3, Opus, Vorbis, FLAC | Copy (browser-compatible) | Same codec |
| DTS, TrueHD, EAC3, AC3 (stereo) | Transcode | AAC stereo 192kbps |
| DTS, TrueHD, EAC3, AC3 (multichannel) | Transcode | AC3 5.1 384kbps |

**Audio stream selection:** Users can select which audio stream to use via the `audio_stream` parameter. The selected stream is mapped with `-map 0:a:{index}`.

**Seeking:** Implemented by restarting the FFmpeg process with `-ss {position}` before the input file, which performs a keyframe-accurate seek.

---

## HLS Transcoding

When quality selection is requested (or the video codec is not browser-compatible), CineVault transcodes to HLS with adaptive quality.

**How it works:**

1. Client requests `GET /api/v1/stream/{mediaId}/master.m3u8`
2. Server generates a master playlist listing available quality levels
3. Client (HLS.js) selects a quality and requests segments
4. Server starts an FFmpeg transcode session, writing HLS segments to disk
5. Segments are served as they become available
6. Sessions are cached and reused for the same media+quality combination

### Quality Profiles

| Quality | Resolution | Video Bitrate | Audio Bitrate |
|---------|-----------|---------------|---------------|
| 360p | 640x360 | 800 kbps | 96 kbps |
| 480p | 854x480 | 1,400 kbps | 128 kbps |
| 720p | 1,280x720 | 2,800 kbps | 128 kbps |
| 1080p | 1,920x1,080 | 5,000 kbps | 192 kbps |
| 4K | 3,840x2,160 | 14,000 kbps | 192 kbps |

Available qualities are filtered based on the source resolution — only qualities at or below the source height are offered. Resolution matching uses a 15% tolerance (e.g., a 1920x800 ultrawide still qualifies as 1080p).

### HLS Configuration

- Segment duration: 6 seconds (`-hls_time 6`)
- Playlist size: unlimited (`-hls_list_size 0`)
- Independent segments flag enabled for better seeking
- H.264 output uses MPEG-TS segments
- HEVC output uses fMP4 segments

### Session Management

Each transcode session is keyed by `{mediaItemID}-{quality}`. Sessions are reused if the same media and quality are requested again.

- Sessions track: segment count, start time, last access time, FFmpeg stderr log
- Expired sessions are cleaned up automatically based on last access time
- Stopping a session kills the FFmpeg process and removes the output directory
- Active session count is tracked for analytics

### Codec Selection

Transcode output codec is determined by hardware acceleration availability:

| Hardware | H.264 Encoder | HEVC Encoder |
|----------|--------------|--------------|
| NVIDIA | `h264_nvenc` | `hevc_nvenc` |
| Intel QSV | `h264_qsv` | `hevc_qsv` |
| AMD/Intel VAAPI | `h264_vaapi` | `hevc_vaapi` |
| Software fallback | `libx264` | `libx265` |

---

## Hardware Acceleration

CineVault auto-detects hardware encoding capabilities on startup by probing FFmpeg's encoder list and running a test encode.

### Detection Strategy

1. Check FFmpeg's available encoders (`-encoders`)
2. Test the encoder with a single-frame encode to verify it works
3. Cache the result for the lifetime of the process
4. Detection order: NVENC → QSV → VAAPI → software fallback

### Supported Accelerators

**NVIDIA (CUDA/NVENC):**
- Hardware decode: `-hwaccel cuda -hwaccel_output_format cuda`
- Encoding: `h264_nvenc` / `hevc_nvenc`
- Quality control: `-preset p4 -cq 23 -b:v {bitrate}`
- HDR tone mapping: zscale + tonemap (Hable algorithm)

**Intel Quick Sync (QSV):**
- Hardware decode: `-hwaccel qsv -qsv_device /dev/dri/renderD128 -hwaccel_output_format qsv`
- Encoding: `h264_qsv` / `hevc_qsv`
- Quality control: `-preset faster -global_quality 23`
- Special handling: frames are downloaded from GPU memory (`hwdownload` + `format=nv12`) for software filters, then re-uploaded by the QSV encoder internally
- HDR tone mapping: `vpp_qsv=tonemap=1`

**VAAPI (AMD/Intel):**
- Hardware decode: `-hwaccel vaapi -vaapi_device /dev/dri/renderD128 -hwaccel_output_format vaapi`
- Encoding: `h264_vaapi` / `hevc_vaapi`
- Quality control: `-qp 23 -b:v {bitrate}`
- Filter suffix: `format=nv12|vaapi,hwupload`
- HDR tone mapping: `tonemap_vaapi`

### Docker Hardware Access

For hardware acceleration in Docker, use the overlay compose file:

**NVIDIA:** Requires `nvidia-container-toolkit`. The hwaccel compose file adds `runtime: nvidia` and GPU device passthrough.

**Intel QSV / VAAPI:** The hwaccel compose file maps `/dev/dri` into the container. The runtime image includes `intel-media-va-driver` and `mesa-va-drivers`.

---

## HDR-to-SDR Tone Mapping

When transcoding HDR content for SDR displays, CineVault applies tone mapping based on the available hardware:

| Hardware | Filter Chain |
|---|---|
| NVIDIA/CUDA | `zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p` |
| QSV | `vpp_qsv=tonemap=1` |
| VAAPI | `tonemap_vaapi` |
| Software | Same as NVIDIA chain but without CUDA decode |

HDR tone mapping is triggered by setting `HDRToSDR: true` in the transcode options. The HDR format (Dolby Vision, HDR10, HLG, PQ) is detected during FFprobe analysis and stored on the media item.

---

## Subtitle Handling

### Subtitle Sources

CineVault supports both external and embedded subtitles:

| Source | Formats | Discovery |
|---|---|---|
| External files | SRT, ASS, SSA, VTT | Detected during scan (same directory, matching filename) |
| Embedded streams | SRT, ASS, SSA, VTT, PGS, VobSub, DVD | Extracted from media file via FFprobe |

### WebVTT Conversion

All subtitles are converted to WebVTT for browser playback:

| Input Format | Conversion Method |
|---|---|
| SRT | Timestamp format conversion (comma → period), header insertion |
| ASS/SSA | Dialogue line parsing, style tag stripping, `\N` → newline |
| VTT | Pass-through (already WebVTT) |
| Other | FFmpeg conversion fallback |

### Subtitle Burn-In

For formats that cannot be rendered by the browser (PGS, VobSub, DVD), CineVault burns subtitles into the video during transcoding:

| Subtitle Type | FFmpeg Filter |
|---|---|
| Text-based (SRT, ASS, SSA, VTT) | `subtitles` filter with stream index |
| Image-based (PGS, VobSub, DVD) | `overlay` filter via `filter_complex` |

Burn-in is triggered by setting `BurnSubtitles: true` and `SubtitleIndex` in the transcode options.

### API

- `GET /api/v1/stream/{mediaId}/subtitles/{id}` — serves an external or embedded subtitle as WebVTT
- Stream info endpoint includes all available subtitle tracks with language, title, format, and source

---

## Stream Info

Before playback begins, the client fetches stream information:

`GET /api/v1/stream/{mediaId}/info`

Returns:

```json
{
  "media_id": "uuid",
  "file_path": "/media/movies/...",
  "container": "mkv",
  "needs_remux": true,
  "needs_audio_transcode": false,
  "available_qualities": ["360p", "480p", "720p", "1080p"],
  "hw_accel": "nvenc",
  "subtitles": [
    {
      "id": 0,
      "language": "eng",
      "title": "English",
      "format": "srt",
      "source": "external",
      "is_default": true
    }
  ],
  "audio_tracks": [
    {
      "index": 0,
      "language": "eng",
      "title": "English",
      "codec": "truehd",
      "channels": 8,
      "is_default": true
    }
  ]
}
```

---

## DASH Manifest

CineVault also supports DASH manifest generation for compatibility with Chromecast and other DASH-compatible clients:

`GET /api/v1/stream/{mediaId}/manifest.mpd`

---

## Analytics Integration

All streaming sessions are recorded for analytics:

| Field | Description |
|---|---|
| `playback_type` | `direct_play`, `direct_stream`, or `transcode` |
| `quality` | Selected quality profile (for transcodes) |
| `codec` | Source video codec |
| `resolution` | Source resolution |
| `container` | Source container format |
| `bytes_served` | Total bytes delivered |
| `duration_seconds` | Session duration |
| `is_active` | Whether the session is still active |

Stream session data feeds the analytics dashboard (see [ANALYTICS-DASHBOARD.MD](ANALYTICS-DASHBOARD.MD)).

---

## API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/stream/{mediaId}/info` | User | Stream info (codecs, subtitles, audio tracks, available qualities) |
| GET | `/api/v1/stream/{mediaId}/master.m3u8` | User | HLS master playlist |
| GET | `/api/v1/stream/{mediaId}/{quality}/{segment}` | User | HLS segment or playlist |
| GET | `/api/v1/stream/{mediaId}/direct` | User | Direct play or remux stream |
| GET | `/api/v1/stream/{mediaId}/subtitles/{id}` | User | Subtitle track as WebVTT |
| GET | `/api/v1/stream/{mediaId}/manifest.mpd` | User | DASH manifest |

All streaming endpoints accept a `?token=` query parameter as an alternative to the `Authorization` header, which is required for HLS.js and mpegts.js clients that cannot set custom headers on segment requests.

---

## File Locations

| Component | Path |
|---|---|
| Direct play | `internal/stream/direct.go` |
| MPEG-TS remux | `internal/stream/remux.go` |
| HLS transcoder | `internal/stream/transcoder.go` |
| Subtitle handling | `internal/stream/subtitle.go` |
| HW acceleration detection | `internal/ffmpeg/hwaccel.go` |
| FFprobe analysis | `internal/ffmpeg/ffprobe.go` |
| Stream API handlers | `internal/api/handlers_stream.go` |
| Preview generation | `internal/preview/preview.go` |
| Frontend player | `web/js/player.js` |
| CSS styles | `web/styles.css` |
