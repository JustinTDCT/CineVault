-- Cast and directors from TMDb's credits, for the title page. NULL means not
-- fetched yet: titles matched before this migration get them at the next match.
ALTER TABLE titles ADD COLUMN cast_members jsonb;
ALTER TABLE titles ADD COLUMN directors text[];

-- Home's "Recently added" sorts by when files arrived.
CREATE INDEX versions_added_at ON versions (added_at DESC);
