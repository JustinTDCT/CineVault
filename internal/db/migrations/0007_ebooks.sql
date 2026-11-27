-- 0007: ebooks. Reading position is saved like listening position, plus a
-- locator (an EPUB CFI) that marks the exact spot; position_seconds holds
-- the fraction of the book read (0 to 1) for books.
ALTER TABLE playback_progress ADD COLUMN locator text;
