-- 020: Household profiles — master user / sub-profile relationship
ALTER TABLE users ADD COLUMN IF NOT EXISTS parent_user_id UUID DEFAULT NULL
    REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_users_parent ON users(parent_user_id);

COMMENT ON COLUMN users.parent_user_id IS 'NULL = master user. Set = sub-profile belonging to that master user household.';
