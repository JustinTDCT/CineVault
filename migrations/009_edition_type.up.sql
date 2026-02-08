-- Add edition_type column to media_items (default: Theatrical)
ALTER TABLE media_items ADD COLUMN IF NOT EXISTS edition_type VARCHAR(50) NOT NULL DEFAULT 'Theatrical';

-- Backfill: parse {edition-XXX} from file_name and set edition_type
UPDATE media_items
SET edition_type = regexp_replace(
    substring(file_name FROM '\{edition-([^}]+)\}'),
    '^\s+|\s+$', '', 'g'
)
WHERE file_name ~ '\{edition-[^}]+\}';

-- Backfill: clean titles that contain {edition-...} or {anything} or [...] tags
UPDATE media_items
SET title = trim(regexp_replace(
    regexp_replace(
        regexp_replace(title, '\{[^}]*\}', '', 'g'),
        '\[[^\]]*\]', '', 'g'
    ),
    '\s+', ' ', 'g'
))
WHERE title ~ '\{[^}]*\}' OR title ~ '\[[^\]]*\]';

-- Also clean up titles that have year in parens still (from bad parse)
-- and trailing dashes/whitespace
UPDATE media_items
SET title = trim(regexp_replace(
    regexp_replace(title, '\s*-\s*$', ''),
    '\s+', ' ', 'g'
))
WHERE title ~ '\s+-\s*$';
