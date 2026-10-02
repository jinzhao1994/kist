-- +goose Up

-- Derived from request metadata in the checksummed journal envelope.
CREATE TABLE op_requests (
  request_id TEXT PRIMARY KEY,
  digest TEXT NOT NULL,
  op_id INTEGER NOT NULL UNIQUE REFERENCES ops(id)
);
