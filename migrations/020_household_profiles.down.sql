DROP INDEX IF EXISTS idx_users_parent;
ALTER TABLE users DROP COLUMN IF EXISTS parent_user_id;
