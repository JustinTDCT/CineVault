-- Phase 3: Library Scanning — scheduled scans, filesystem watching, extras/trailers

-- ── Scheduled scan fields on libraries ──
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS scan_interval TEXT NOT NULL DEFAULT 'disabled';
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS next_scan_at TIMESTAMPTZ;
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS watch_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- ── Extras/trailers support on media_items ──
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS parent_media_id UUID REFERENCES media_items(id) ON DELETE SET NULL;
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS extra_type TEXT;

CREATE INDEX IF NOT EXISTS idx_media_items_parent ON media_items(parent_media_id) WHERE parent_media_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_media_items_extra_type ON media_items(extra_type) WHERE extra_type IS NOT NULL;
