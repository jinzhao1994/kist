-- +goose Up
CREATE TABLE saved_searches (
    id BLOB PRIMARY KEY CHECK(length(id) = 16),
    name TEXT NOT NULL,
    name_fold TEXT NOT NULL UNIQUE,
    definition TEXT NOT NULL
);

-- +goose Down
DROP TABLE saved_searches;
