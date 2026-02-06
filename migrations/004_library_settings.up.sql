-- Library permission levels
DO $$ BEGIN
    CREATE TYPE library_access AS ENUM ('everyone', 'select_users', 'admin_only');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Add season grouping and access level to libraries
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS season_grouping BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS access_level library_access NOT NULL DEFAULT 'everyone';

-- Per-user library access (used when access_level = 'select_users')
CREATE TABLE IF NOT EXISTS library_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(library_id, user_id)
);
