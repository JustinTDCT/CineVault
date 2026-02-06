-- Rollback extended library settings
DROP TABLE IF EXISTS library_folders;
ALTER TABLE libraries DROP COLUMN IF EXISTS include_in_homepage;
ALTER TABLE libraries DROP COLUMN IF EXISTS include_in_search;
ALTER TABLE libraries DROP COLUMN IF EXISTS retrieve_metadata;
ALTER TABLE libraries DROP COLUMN IF EXISTS adult_content_type;
