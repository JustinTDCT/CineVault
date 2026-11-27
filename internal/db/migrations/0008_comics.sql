-- 0008: comics. A comic series is a title and each issue is a track whose
-- files are versions; page_count is how many pages an issue's file has.
ALTER TABLE versions ADD COLUMN page_count integer;
