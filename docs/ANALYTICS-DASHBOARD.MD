# CineVault Analytics Dashboard

This document describes how CineVault's server analytics, monitoring, alerting, and dashboard system works.

---

## Overview

CineVault provides a real-time analytics and monitoring system with five components:

1. **Data Collection** — automatic instrumentation of streams and transcodes, plus periodic system metrics polling
2. **Daily Rollup** — nightly aggregation of raw data into summary statistics
3. **Alert System** — configurable rules that evaluate conditions and send webhook notifications
4. **API Layer** — 21 admin-only endpoints exposing analytics, notification, and alert data
5. **Frontend Dashboard** — a tabbed admin view with Chart.js visualizations

```
Streams/Transcodes → stream_sessions / transcode_history
                                ↓
System Metrics Collector (60s) → system_metrics
                                ↓
Daily Rollup (midnight) → daily_stats
                                ↓
Alert Evaluator (5m) → alert_rules → webhooks → alert_log
                                ↓
Dashboard UI (Chart.js) ← API endpoints
```

---

## Data Collection

### Stream Session Tracking

Every playback event creates a record in `stream_sessions`. The stream handlers (`handleStreamDirect`, `handleStreamSegment`) are instrumented to:

1. Create a session when playback starts, recording user, media item, playback type, codec, resolution, and container
2. Update `bytes_served` and `duration_seconds` as content is delivered
3. Mark the session as ended (`is_active = false`) when playback stops

Playback types:

| Type | Description |
|---|---|
| `direct_play` | File served as-is, no transcoding |
| `direct_stream` | Container remuxed without re-encoding |
| `transcode` | Video/audio re-encoded via FFmpeg |

### Transcode History

Each transcode operation (success or failure) is recorded in `transcode_history` with:

- Input and output codec/resolution
- Hardware accelerator used (e.g., `h264_nvenc`, `h264_vaapi`)
- Duration, file size, success/failure status, and error message if applicable

### System Metrics Collector

A background goroutine (`analytics.Collector`) polls system resources every **60 seconds** and stores snapshots in `system_metrics`:

| Metric | Source | Description |
|---|---|---|
| CPU usage (%) | `/proc/stat` | Total CPU utilization across all cores |
| Memory usage (%) | `/proc/meminfo` | RAM used vs available |
| Memory used (MB) | `/proc/meminfo` | Absolute RAM consumption |
| GPU encoder (%) | `nvidia-smi` | NVIDIA encoder utilization (if GPU detected) |
| GPU memory (%) | `nvidia-smi` | NVIDIA VRAM utilization |
| GPU temperature (°C) | `nvidia-smi` | GPU die temperature |
| Disk total/used/free (GB) | `syscall.Statfs` | Filesystem stats for the primary media path |
| Active streams | `stream_sessions` count | Currently active playback sessions |
| Active transcodes | Transcoder session count | Currently running FFmpeg transcode processes |

GPU metrics are only collected when `nvidia-smi` is found in PATH. The collector auto-detects NVIDIA GPU availability on startup.

Old metrics are automatically cleaned up — records older than **30 days** are purged on each collection cycle.

---

## Daily Rollup

A scheduler goroutine (`analytics.StartRollupScheduler`) runs the daily rollup:

1. **On startup**: immediately computes yesterday's stats
2. **Every night at 00:05**: computes the previous day's aggregate stats

The rollup queries `stream_sessions`, `transcode_history`, and `media_items` to produce a `daily_stats` row containing:

| Field | Description |
|---|---|
| `total_plays` | Total stream sessions for the day |
| `unique_users` | Distinct users who streamed |
| `total_watch_minutes` | Sum of all session durations |
| `total_bytes_served` | Total bandwidth consumed |
| `transcodes` | Number of transcode sessions |
| `direct_plays` | Number of direct play sessions |
| `direct_streams` | Number of direct stream sessions |
| `transcode_failures` | Failed transcode count |
| `new_media_added` | Items added to the library that day |
| `library_size_total` | Total item count at end of day |
| `storage_used_bytes` | Total file size across all media |

Stats are upserted by date — re-running for the same date overwrites the previous values.

---

## Alert System

### Notification Channels

Channels define where alerts are delivered. Three webhook types are supported:

| Type | Payload Format |
|---|---|
| `discord` | Discord embed with title, description, color, footer, and timestamp |
| `slack` | Slack Block Kit with header, section, and context blocks |
| `generic` | Simple JSON with `title`, `message`, `source`, and `timestamp` fields |

Each channel has:
- A name and webhook URL
- An `is_enabled` toggle
- An `events` JSONB field for filtering (defaults to `["all"]`)

Channels can be tested via the API, which sends a standard test message to validate the webhook URL.

### Alert Rules

Alert rules define conditions that trigger notifications. Each rule specifies:

| Field | Description |
|---|---|
| `name` | Display name for the alert |
| `condition_type` | What to check (see below) |
| `threshold` | Numeric threshold value |
| `cooldown_minutes` | Minimum time between alerts (default: 60 minutes) |
| `channel_id` | Which notification channel to use |
| `is_enabled` | Active/inactive toggle |

### Condition Types

| Condition | Checks | Threshold Meaning |
|---|---|---|
| `disk_space_low` | Free disk space from latest `system_metrics` | Alert when free GB drops below threshold |
| `gpu_temp_high` | GPU temperature from latest `system_metrics` | Alert when temp exceeds threshold (°C) |
| `transcode_failure` | Failed transcode count in the last 60 minutes | Alert when failure count exceeds threshold |
| `stream_error` | Stream error count in the last 60 minutes | Alert when error count exceeds threshold |

### Alert Evaluation Loop

The `AlertEvaluator` runs every **5 minutes** as a background goroutine:

1. Fetch all enabled alert rules
2. Get the latest system metrics snapshot
3. For each rule:
   - Check if the cooldown period has elapsed since the last trigger
   - Evaluate the condition against current metrics or recent activity
   - If triggered: send the webhook, log the alert, and update `last_triggered_at`
4. Both successful and failed sends are recorded in `alert_log`

---

## Database Schema

### Tables (Migration 025)

**stream_sessions**

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| user_id | UUID NOT NULL | FK → users |
| media_item_id | UUID NOT NULL | FK → media_items |
| playback_type | VARCHAR(20) NOT NULL | 'direct_play' |
| quality | VARCHAR(20) | NULL |
| codec | VARCHAR(50) | NULL |
| resolution | VARCHAR(20) | NULL |
| container | VARCHAR(20) | NULL |
| bytes_served | BIGINT NOT NULL | 0 |
| duration_seconds | INT NOT NULL | 0 |
| client_info | VARCHAR(255) | NULL |
| started_at | TIMESTAMPTZ NOT NULL | CURRENT_TIMESTAMP |
| ended_at | TIMESTAMPTZ | NULL |
| is_active | BOOLEAN NOT NULL | true |

**transcode_history**

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| media_item_id | UUID NOT NULL | FK → media_items |
| user_id | UUID NOT NULL | FK → users |
| input_codec | VARCHAR(50) | NULL |
| output_codec | VARCHAR(50) | NULL |
| input_resolution | VARCHAR(20) | NULL |
| output_resolution | VARCHAR(20) | NULL |
| hw_accel | VARCHAR(50) | NULL |
| quality | VARCHAR(20) | NULL |
| duration_seconds | INT NOT NULL | 0 |
| file_size_bytes | BIGINT NOT NULL | 0 |
| success | BOOLEAN NOT NULL | true |
| error_message | TEXT | NULL |
| started_at | TIMESTAMPTZ NOT NULL | CURRENT_TIMESTAMP |
| completed_at | TIMESTAMPTZ | NULL |

**system_metrics**

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| cpu_percent | REAL NOT NULL | 0 |
| memory_percent | REAL NOT NULL | 0 |
| memory_used_mb | INT NOT NULL | 0 |
| gpu_encoder_percent | REAL | NULL |
| gpu_memory_percent | REAL | NULL |
| gpu_temp_celsius | REAL | NULL |
| disk_total_gb | REAL NOT NULL | 0 |
| disk_used_gb | REAL NOT NULL | 0 |
| disk_free_gb | REAL NOT NULL | 0 |
| active_streams | INT NOT NULL | 0 |
| active_transcodes | INT NOT NULL | 0 |
| recorded_at | TIMESTAMPTZ NOT NULL | CURRENT_TIMESTAMP |

**daily_stats**

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| stat_date | DATE NOT NULL UNIQUE | — |
| total_plays | INT NOT NULL | 0 |
| unique_users | INT NOT NULL | 0 |
| total_watch_minutes | INT NOT NULL | 0 |
| total_bytes_served | BIGINT NOT NULL | 0 |
| transcodes | INT NOT NULL | 0 |
| direct_plays | INT NOT NULL | 0 |
| direct_streams | INT NOT NULL | 0 |
| transcode_failures | INT NOT NULL | 0 |
| new_media_added | INT NOT NULL | 0 |
| library_size_total | INT NOT NULL | 0 |
| storage_used_bytes | BIGINT NOT NULL | 0 |

**notification_channels**

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| name | VARCHAR(100) NOT NULL | — |
| channel_type | VARCHAR(20) NOT NULL | — |
| webhook_url | TEXT NOT NULL | — |
| is_enabled | BOOLEAN NOT NULL | true |
| events | JSONB NOT NULL | '["all"]' |
| created_at | TIMESTAMPTZ NOT NULL | CURRENT_TIMESTAMP |
| updated_at | TIMESTAMPTZ NOT NULL | CURRENT_TIMESTAMP |

**alert_rules**

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| name | VARCHAR(100) NOT NULL | — |
| condition_type | VARCHAR(50) NOT NULL | — |
| threshold | REAL NOT NULL | 0 |
| cooldown_minutes | INT NOT NULL | 60 |
| channel_id | UUID NOT NULL | FK → notification_channels |
| is_enabled | BOOLEAN NOT NULL | true |
| last_triggered_at | TIMESTAMPTZ | NULL |
| created_at | TIMESTAMPTZ NOT NULL | CURRENT_TIMESTAMP |
| updated_at | TIMESTAMPTZ NOT NULL | CURRENT_TIMESTAMP |

**alert_log**

| Column | Type | Default |
|---|---|---|
| id | UUID | gen_random_uuid() |
| rule_id | UUID | FK → alert_rules (SET NULL) |
| channel_id | UUID | FK → notification_channels (SET NULL) |
| message | TEXT NOT NULL | — |
| success | BOOLEAN NOT NULL | true |
| error_detail | TEXT | NULL |
| sent_at | TIMESTAMPTZ NOT NULL | CURRENT_TIMESTAMP |

### Indexes

- `idx_stream_sessions_user` on `stream_sessions(user_id)`
- `idx_stream_sessions_media` on `stream_sessions(media_item_id)`
- `idx_stream_sessions_started` on `stream_sessions(started_at)`
- `idx_stream_sessions_active` on `stream_sessions(is_active)` — partial, WHERE `is_active = true`
- `idx_transcode_history_media` on `transcode_history(media_item_id)`
- `idx_transcode_history_user` on `transcode_history(user_id)`
- `idx_transcode_history_started` on `transcode_history(started_at)`
- `idx_transcode_history_success` on `transcode_history(success)`
- `idx_system_metrics_recorded` on `system_metrics(recorded_at)`
- `idx_daily_stats_date` on `daily_stats(stat_date)`
- `idx_alert_rules_channel` on `alert_rules(channel_id)`
- `idx_alert_rules_enabled` on `alert_rules(is_enabled)` — partial, WHERE `is_enabled = true`
- `idx_alert_log_sent` on `alert_log(sent_at)`
- `idx_alert_log_rule` on `alert_log(rule_id)`

---

## API Endpoints

### Analytics (Admin Only)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/analytics/overview` | Dashboard overview: active streams, today's plays, bandwidth, CPU/memory/disk, transcodes, failures |
| GET | `/api/v1/analytics/streams` | Active and recent stream sessions (last 50) |
| GET | `/api/v1/analytics/streams/breakdown` | Playback type breakdown (direct play vs stream vs transcode). Optional `?days=` (default 30) |
| GET | `/api/v1/analytics/watch-activity` | Who-watched-what activity feed. Optional filters: `?user_id=`, `?from=`, `?to=` (YYYY-MM-DD), `?media_type=`, `?limit=` (default 100, max 500) |
| GET | `/api/v1/analytics/users/activity` | Per-user activity summaries: total plays, watch minutes, last active, favorite genre |
| GET | `/api/v1/analytics/transcodes` | Transcode statistics (success rate, HW accel %, codec distribution) and recent transcodes. Optional `?days=` (default 30) |
| GET | `/api/v1/analytics/system` | Latest system metrics snapshot (CPU, RAM, GPU, disk, active streams/transcodes) |
| GET | `/api/v1/analytics/system/history` | System metrics time series. Optional `?hours=` (default 24, max 168) |
| GET | `/api/v1/analytics/storage` | Per-library storage breakdown (file count and total bytes) |
| GET | `/api/v1/analytics/library-health` | Per-library health report: metadata completeness, codec/resolution distribution, HDR/Atmos counts |
| GET | `/api/v1/analytics/trends` | Daily stats time series from `daily_stats`. Optional `?from=` and `?to=` (YYYY-MM-DD, default last 30 days) |

### Notification Channels (Admin Only)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/notifications/channels` | List all notification channels |
| POST | `/api/v1/notifications/channels` | Create a channel (requires: `name`, `channel_type`, `webhook_url`) |
| PUT | `/api/v1/notifications/channels/{id}` | Update a channel |
| DELETE | `/api/v1/notifications/channels/{id}` | Delete a channel |
| POST | `/api/v1/notifications/channels/{id}/test` | Send a test message to validate the webhook |

### Alert Rules (Admin Only)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/notifications/alerts` | List all alert rules |
| POST | `/api/v1/notifications/alerts` | Create an alert rule (requires: `name`, `condition_type`, `channel_id`) |
| PUT | `/api/v1/notifications/alerts/{id}` | Update an alert rule |
| DELETE | `/api/v1/notifications/alerts/{id}` | Delete an alert rule |

### Alert Log (Admin Only)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/notifications/log` | Recent alert history (last 100 entries) with rule and channel names |

### Request Examples

**Create a Discord notification channel:**

```json
POST /api/v1/notifications/channels
{
  "name": "Discord Alerts",
  "channel_type": "discord",
  "webhook_url": "https://discord.com/api/webhooks/...",
  "is_enabled": true,
  "events": "[\"all\"]"
}
```

**Create a disk space alert:**

```json
POST /api/v1/notifications/alerts
{
  "name": "Low Disk Space",
  "condition_type": "disk_space_low",
  "threshold": 50.0,
  "cooldown_minutes": 120,
  "channel_id": "uuid-of-channel",
  "is_enabled": true
}
```

**Create a transcode failure alert:**

```json
POST /api/v1/notifications/alerts
{
  "name": "Transcode Failures",
  "condition_type": "transcode_failure",
  "threshold": 3,
  "cooldown_minutes": 60,
  "channel_id": "uuid-of-channel"
}
```

---

## Frontend Dashboard

The analytics dashboard is accessible from the sidebar navigation (admin users only). It uses **Chart.js 4.x** for all visualizations.

### Dashboard Tabs

| Tab | Content |
|---|---|
| **Overview** | Stat cards for active streams, plays today/this week, unique users, bandwidth, library size, transcodes, direct plays, failures. CPU, memory, and disk gauges. |
| **Activity** | Who-watched-what table showing user, title, playback type, timestamp, duration, and completion status. Filterable by user, date range, and media type. |
| **Streams** | Active streams table (user, title, type, quality, duration, bytes served). Recent streams history. Playback type breakdown pie chart (direct play vs stream vs transcode). |
| **Transcodes** | Transcode stats cards (total, success rate, HW acceleration %). Codec distribution chart. Recent transcode history table with success/failure indicators. |
| **System** | Real-time system metrics: CPU %, memory %, GPU encoder/memory/temp (if available), disk usage. Historical time series charts (configurable: 1h, 6h, 24h, 7d). |
| **Library Health** | Per-library health cards showing metadata completeness percentage, missing metadata/poster counts, codec distribution, resolution distribution, HDR and Atmos counts. |
| **Alerts** | Notification channel management (create, edit, delete, test). Alert rule management (create, edit, delete) with condition type selector, threshold input, and cooldown. Recent alert log table showing trigger history. |

### Stat Cards

Overview stat cards display:
- A label (e.g., "Active Streams")
- A value (numeric, formatted with `formatBytes()` for bandwidth)
- Optional danger styling for failure counts

### Chart Types

| Chart | Type | Data Source |
|---|---|---|
| Playback breakdown | Doughnut | `/analytics/streams/breakdown` |
| Codec distribution | Bar | `/analytics/transcodes` |
| CPU history | Line | `/analytics/system/history` |
| Memory history | Line | `/analytics/system/history` |
| Daily trends | Line | `/analytics/trends` |

---

## Background Workers

Three goroutines are started in `main.go` alongside the HTTP server:

| Worker | Interval | Purpose |
|---|---|---|
| `analytics.Collector` | 60 seconds | Poll CPU, RAM, GPU, disk, active streams/transcodes and store in `system_metrics` |
| `analytics.RollupScheduler` | Once at startup + daily at 00:05 | Aggregate raw data into `daily_stats` for the previous day |
| `notifications.AlertEvaluator` | 5 minutes | Evaluate enabled alert rules and send webhooks when conditions are met |

All workers run for the lifetime of the application and are stopped gracefully via channel signals on shutdown.

---

## File Locations

| Component | Path |
|---|---|
| Models | `internal/models/models.go` (StreamSession, TranscodeHistoryRecord, SystemMetric, DailyStat, NotificationChannel, AlertRule, AlertLogEntry, AnalyticsOverview, StreamTypeBreakdown, UserActivitySummary, LibraryHealthReport, TranscodeStats, WatchActivityEntry, StorageInfo, NameCount) |
| Analytics repository | `internal/repository/analytics_repository.go` |
| Notification repository | `internal/repository/notification_repository.go` |
| System metrics collector | `internal/analytics/collector.go` |
| Daily rollup scheduler | `internal/analytics/rollup.go` |
| Webhook sender | `internal/notifications/webhook.go` |
| Alert evaluator | `internal/notifications/alerts.go` |
| Analytics API handlers | `internal/api/handlers_analytics.go` |
| Notification API handlers | `internal/api/handlers_notifications.go` |
| Stream instrumentation | `internal/api/handlers_stream.go` |
| Transcoder instrumentation | `internal/stream/transcoder.go` |
| Route registration | `internal/api/server.go` |
| Startup wiring | `cmd/cinevault/main.go` |
| Frontend dashboard | `web/index.html` (loadAnalyticsView) |
| CSS styles | `web/styles.css` (analytics-tabs, analytics-grid, stat-card, chart-container, activity-table) |
| Migration (up) | `migrations/025_server_analytics.up.sql` |
| Migration (down) | `migrations/025_server_analytics.down.sql` |
