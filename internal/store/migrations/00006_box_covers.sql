-- +goose Up
-- A soft reference survives trash and restore. Zero means no selected cover.
ALTER TABLE boxes ADD COLUMN cover_file_id BLOB NOT NULL DEFAULT X'00000000000000000000000000000000'
    CHECK(typeof(cover_file_id) = 'blob' AND length(cover_file_id) = 16);
