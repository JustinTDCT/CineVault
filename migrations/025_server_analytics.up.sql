-- 025: Server Analytics & Dashboard
-- Adds stream sessions, transcode history, system metrics, daily stats,
-- notification channels, alert rules, and alert log tables.

-- ── Stream Sessions ──
CREATE TABLE IF NOT EXISTS stream_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_item_id   UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    playback_type   VARCHAR(20) NOT NULL DEFAULT 'direct_play',  -- direct_play, direct_stream, transcode
    quality         VARCHAR(20),
    codec           VARCHAR(50),
    resolution      VARCHAR(20),
    container       VARCHAR(20),
    bytes_served    BIGINT NOT NULL DEFAULT 0,
    duration_seconds INT NOT NULL DEFAULT 0,
    client_info     VARCHAR(255),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at        TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_stream_sessions_user ON stream_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_stream_sessions_media ON stream_sessions(media_item_id);
CREATE INDEX IF NOT EXISTS idx_stream_sessions_started ON stream_sessions(started_at);
CREATE INDEX IF NOT EXISTS idx_stream_sessions_active ON stream_sessions(is_active) WHERE is_active = true;

-- ── Transcode History ──
CREATE TABLE IF NOT EXISTS transcode_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_item_id   UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    input_codec     VARCHAR(50),
    output_codec    VARCHAR(50),
    input_resolution  VARCHAR(20),
    output_resolution VARCHAR(20),
    hw_accel        VARCHAR(50),
    quality         VARCHAR(20),
    duration_seconds INT NOT NULL DEFAULT 0,
    file_size_bytes BIGINT NOT NULL DEFAULT 0,
    success         BOOLEAN NOT NULL DEFAULT true,
    error_message   TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_transcode_history_media ON transcode_history(media_item_id);
CREATE INDEX IF NOT EXISTS idx_transcode_history_user ON transcode_history(user_id);
CREATE INDEX IF NOT EXISTS idx_transcode_history_started ON transcode_history(started_at);
CREATE INDEX IF NOT EXISTS idx_transcode_history_success ON transcode_history(success);

-- ── System Metrics (periodic snapshots) ──
CREATE TABLE IF NOT EXISTS system_metrics (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cpu_percent         REAL NOT NULL DEFAULT 0,
    memory_percent      REAL NOT NULL DEFAULT 0,
    memory_used_mb      INT NOT NULL DEFAULT 0,
    gpu_encoder_percent REAL,
    gpu_memory_percent  REAL,
    gpu_temp_celsius    REAL,
    disk_total_gb       REAL NOT NULL DEFAULT 0,
    disk_used_gb        REAL NOT NULL DEFAULT 0,
    disk_free_gb        REAL NOT NULL DEFAULT 0,
    active_streams      INT NOT NULL DEFAULT 0,
    active_transcodes   INT NOT NULL DEFAULT 0,
    recorded_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_system_metrics_recorded ON system_metrics(recorded_at);

-- ── Daily Stats (rollup table) ──
CREATE TABLE IF NOT EXISTS daily_stats (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stat_date           DATE NOT NULL UNIQUE,
    total_plays         INT NOT NULL DEFAULT 0,
    unique_users        INT NOT NULL DEFAULT 0,
    total_watch_minutes INT NOT NULL DEFAULT 0,
    total_bytes_served  BIGINT NOT NULL DEFAULT 0,
    transcodes          INT NOT NULL DEFAULT 0,
    direct_plays        INT NOT NULL DEFAULT 0,
    direct_streams      INT NOT NULL DEFAULT 0,
    transcode_failures  INT NOT NULL DEFAULT 0,
    new_media_added     INT NOT NULL DEFAULT 0,
    library_size_total  INT NOT NULL DEFAULT 0,
    storage_used_bytes  BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_daily_stats_date ON daily_stats(stat_date);

-- ── Notification Channels ──
CREATE TABLE IF NOT EXISTS notification_channels (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    channel_type VARCHAR(20) NOT NULL,  -- discord, slack, generic
    webhook_url TEXT NOT NULL,
    is_enabled  BOOLEAN NOT NULL DEFAULT true,
    events      JSONB NOT NULL DEFAULT '["all"]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ── Alert Rules ──
CREATE TABLE IF NOT EXISTS alert_rules (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             VARCHAR(100) NOT NULL,
    condition_type   VARCHAR(50) NOT NULL,  -- disk_space_low, transcode_failure, stream_error, gpu_temp_high
    threshold        REAL NOT NULL DEFAULT 0,
    cooldown_minutes INT NOT NULL DEFAULT 60,
    channel_id       UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    is_enabled       BOOLEAN NOT NULL DEFAULT true,
    last_triggered_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_channel ON alert_rules(channel_id);
CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(is_enabled) WHERE is_enabled = true;

-- ── Alert Log ──
CREATE TABLE IF NOT EXISTS alert_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id     UUID REFERENCES alert_rules(id) ON DELETE SET NULL,
    channel_id  UUID REFERENCES notification_channels(id) ON DELETE SET NULL,
    message     TEXT NOT NULL,
    success     BOOLEAN NOT NULL DEFAULT true,
    error_detail TEXT,
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_alert_log_sent ON alert_log(sent_at);
CREATE INDEX IF NOT EXISTS idx_alert_log_rule ON alert_log(rule_id);
