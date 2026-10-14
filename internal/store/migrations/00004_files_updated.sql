-- +goose Up
CREATE INDEX files_updated ON files(updated_at DESC, id DESC);

-- +goose Down
DROP INDEX files_updated;
