-- Movie Series: groups movies in a library into an ordered franchise/series
CREATE TABLE IF NOT EXISTS movie_series (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id    UUID NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    poster_path   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(library_id, name)
);

CREATE TABLE IF NOT EXISTS movie_series_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    series_id     UUID NOT NULL REFERENCES movie_series(id) ON DELETE CASCADE,
    media_item_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    sort_order    INT NOT NULL DEFAULT 0,
    added_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(series_id, media_item_id)
);

CREATE INDEX IF NOT EXISTS idx_movie_series_library ON movie_series(library_id);
CREATE INDEX IF NOT EXISTS idx_movie_series_items_series ON movie_series_items(series_id);
CREATE INDEX IF NOT EXISTS idx_movie_series_items_media ON movie_series_items(media_item_id);
