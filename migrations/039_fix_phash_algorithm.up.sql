-- Widen the column to hold actual perceptual hash hex strings (112 chars for 7-frame composite).
ALTER TABLE media_items ALTER COLUMN phash TYPE VARCHAR(256);
