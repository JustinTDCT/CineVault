-- 0009: the CineVault Cache client (cache spec §3–§5a). Each title keeps its
-- cache state and the record version it has; editions keep what the cache
-- says about them, and editions the library doesn't own are added as
-- placeholders (from_cache, no versions). Settings holds server-wide choices.

ALTER TABLE titles
    ADD COLUMN cache_state text NOT NULL DEFAULT 'none'
        CHECK (cache_state IN ('none', 'pending', 'cached', 'not_found', 'error')),
    ADD COLUMN cache_version integer NOT NULL DEFAULT 0,
    ADD COLUMN cache_checked_at timestamptz,
    ADD COLUMN cache_retry_at timestamptz,   -- not before this (not found: 30 days)
    ADD COLUMN cache_error text,
    ADD COLUMN cache_withdrawn boolean NOT NULL DEFAULT false, -- "No longer verified"
    ADD COLUMN cache_sources jsonb,          -- the record's sources, for display
    ADD COLUMN contributed_hash text;        -- what was last shared (spec §5a)

ALTER TABLE editions
    ADD COLUMN from_cache boolean NOT NULL DEFAULT false, -- created by the cache: a placeholder until a file arrives
    ADD COLUMN cache_display_name text,
    ADD COLUMN cache_runtime_min integer,
    ADD COLUMN cache_runtime_sources text[],
    ADD COLUMN cache_difference_short text,
    ADD COLUMN cache_difference_full jsonb,
    ADD COLUMN cache_confidence text,
    ADD COLUMN cache_status text;            -- 'published' or 'withdrawn'

CREATE TABLE settings (
    key         text PRIMARY KEY,
    value       jsonb NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);
