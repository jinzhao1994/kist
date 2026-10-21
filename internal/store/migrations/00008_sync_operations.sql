-- +goose Up
CREATE TABLE op_sync (
 op_id INTEGER PRIMARY KEY REFERENCES ops(id),
 local_id TEXT NOT NULL UNIQUE,
 digest TEXT NOT NULL,
 accepted_at INTEGER NOT NULL
);
