CREATE TABLE video_sources (
    blob_key TEXT NOT NULL,
    recipe TEXT NOT NULL,
    duration REAL NOT NULL CHECK(duration > 0),
    PRIMARY KEY(blob_key, recipe)
) STRICT;
CREATE TABLE video_segments (
    blob_key TEXT NOT NULL,
    recipe TEXT NOT NULL,
    segment INTEGER NOT NULL CHECK(segment >= 0),
    bytes INTEGER NOT NULL CHECK(bytes > 0),
    digest TEXT NOT NULL,
    accessed_at TEXT NOT NULL,
    PRIMARY KEY(blob_key, recipe, segment)
) STRICT;
CREATE INDEX video_eviction ON video_segments(accessed_at, blob_key, recipe, segment);
