CREATE TABLE instance_identity (singleton INTEGER PRIMARY KEY CHECK(singleton=1), mode TEXT NOT NULL, id TEXT NOT NULL) STRICT;

CREATE TABLE migrations (source TEXT PRIMARY KEY) STRICT;

CREATE TABLE background_jobs (
    kind TEXT NOT NULL,
    job_key TEXT NOT NULL,
    file_id TEXT NOT NULL DEFAULT '',
    blob_key TEXT NOT NULL,
    recipe TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    automatic INTEGER NOT NULL DEFAULT 0,
    manual INTEGER NOT NULL DEFAULT 0,
    force INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT '',
    result TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY(kind, job_key)
) STRICT;
CREATE INDEX jobs_dispatch ON background_jobs(kind, state, position, job_key);
CREATE INDEX jobs_order ON background_jobs(kind, position, job_key);
CREATE INDEX jobs_file ON background_jobs(file_id, blob_key, kind);
CREATE INDEX jobs_request ON background_jobs(request_id) WHERE request_id != '';

CREATE TABLE job_attempts (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL,
    job_key TEXT NOT NULL,
    file_id TEXT NOT NULL,
    blob_key TEXT NOT NULL,
    state TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    output_text TEXT NOT NULL DEFAULT '',
    output_bytes INTEGER NOT NULL DEFAULT 0,
    truncated INTEGER NOT NULL DEFAULT 0
) STRICT;
CREATE INDEX attempts_file ON job_attempts(file_id, kind, sequence);
CREATE INDEX attempts_job ON job_attempts(kind, job_key, started_at, request_id);

CREATE TABLE queue_controls (
    kind TEXT PRIMARY KEY,
    paused INTEGER NOT NULL,
    error TEXT NOT NULL,
    auto_cursor INTEGER NOT NULL,
    auto_after INTEGER,
    auto_through INTEGER,
    auto_file TEXT,
    auto_retry INTEGER,
    backfill_after INTEGER,
    backfill_through INTEGER,
    backfill_file TEXT,
    backfill_retry INTEGER
) STRICT;

CREATE TABLE service_limits (service TEXT PRIMARY KEY, next_at TEXT NOT NULL) STRICT;

CREATE TABLE cache_objects (
    key TEXT PRIMARY KEY,
    version INTEGER NOT NULL,
    size INTEGER NOT NULL,
    offset INTEGER NOT NULL,
    complete INTEGER NOT NULL,
    corrupt INTEGER NOT NULL,
    accessed_at TEXT NOT NULL
) STRICT;
CREATE INDEX cache_eviction ON cache_objects(complete, accessed_at, key);
CREATE TABLE cache_derivations (
    version INTEGER NOT NULL,
    original TEXT NOT NULL,
    recipe TEXT NOT NULL,
    content_key TEXT NOT NULL,
    descriptor TEXT NOT NULL,
    outputs TEXT NOT NULL,
    PRIMARY KEY(original, recipe)
) STRICT;
CREATE INDEX derived_content ON cache_derivations(content_key);
CREATE TABLE cache_selections (owner TEXT PRIMARY KEY, selection TEXT) STRICT;
CREATE TABLE cache_pins (
    owner TEXT NOT NULL REFERENCES cache_selections(owner) ON DELETE CASCADE,
    content_key TEXT NOT NULL,
    PRIMARY KEY(owner, content_key)
) STRICT;
CREATE INDEX pins_content ON cache_pins(content_key);
CREATE TABLE cache_workspaces (
    id TEXT PRIMARY KEY,
    version INTEGER NOT NULL,
    budget INTEGER NOT NULL,
    created_at TEXT NOT NULL
) STRICT;

CREATE TABLE sync_boundaries (
    name TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    op_id INTEGER NOT NULL,
    sum TEXT NOT NULL
) STRICT;
CREATE TABLE view_stamps (
    name TEXT PRIMARY KEY,
    version INTEGER NOT NULL,
    catalog_id INTEGER NOT NULL,
    catalog_digest TEXT NOT NULL,
    pending_id INTEGER NOT NULL,
    pending_digest TEXT NOT NULL
) STRICT;

CREATE TABLE upload_sessions (
    namespace TEXT NOT NULL,
    id TEXT NOT NULL,
    version INTEGER NOT NULL,
    manifest TEXT NOT NULL,
    expires TEXT NOT NULL,
    canceled INTEGER NOT NULL,
    op_id INTEGER NOT NULL,
    local_id TEXT NOT NULL,
    noop INTEGER NOT NULL,
    PRIMARY KEY(namespace, id)
) STRICT;
CREATE INDEX upload_expiry ON upload_sessions(namespace, expires);
CREATE TABLE upload_files (
    namespace TEXT NOT NULL,
    session_id TEXT NOT NULL,
    position INTEGER NOT NULL,
    path TEXT NOT NULL,
    modified INTEGER NOT NULL,
    size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    offset INTEGER NOT NULL,
    digest TEXT,
    PRIMARY KEY(namespace, session_id, position),
    FOREIGN KEY(namespace, session_id) REFERENCES upload_sessions(namespace, id) ON DELETE CASCADE
) STRICT;

-- Rebuildable indexes of the client's authoritative pending journal.
CREATE TABLE pending_entries (
    id TEXT PRIMARY KEY,
    position INTEGER NOT NULL UNIQUE,
    sequence INTEGER NOT NULL,
    upstream_position INTEGER,
    active INTEGER NOT NULL,
    action TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    awaiting_root INTEGER NOT NULL,
    awaiting_receipt INTEGER NOT NULL,
    conflict TEXT NOT NULL,
    record TEXT NOT NULL
) STRICT;
CREATE INDEX pending_dispatch ON pending_entries(active, upstream_position, sequence);
