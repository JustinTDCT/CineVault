-- Phase 11: Security & Infrastructure

-- ── 2FA / TOTP ──
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS recovery_codes JSONB;

-- ── Rate limiting tracking ──
CREATE TABLE IF NOT EXISTS rate_limit_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address TEXT NOT NULL,
    blocked_until TIMESTAMPTZ NOT NULL,
    failure_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_rate_limit_ip ON rate_limit_blocks(ip_address);
