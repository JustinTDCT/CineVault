-- Extended library settings: homepage visibility, search inclusion, metadata retrieval, adult content type, multi-folder support

-- Add new setting columns to libraries
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS include_in_homepage BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS include_in_search BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS retrieve_metadata BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS adult_content_type VARCHAR(10) DEFAULT NULL;
-- adult_content_type: 'clips' or 'movies' (only used when media_type = 'adult_movies')

-- Multi-folder support: each library can have multiple folder paths
CREATE TABLE IF NOT EXISTS library_folders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    folder_path TEXT NOT NULL,
    sort_position INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(library_id, folder_path)
);

-- Migrate existing library paths into library_folders table
INSERT INTO library_folders (library_id, folder_path, sort_position)
SELECT id, path, 0 FROM libraries
WHERE path IS NOT NULL AND path != ''
ON CONFLICT (library_id, folder_path) DO NOTHING;
