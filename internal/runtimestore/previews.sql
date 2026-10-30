CREATE TABLE preview_cache (
    blob_key TEXT NOT NULL,
    recipe TEXT NOT NULL,
    generation TEXT NOT NULL,
    bytes INTEGER NOT NULL CHECK(bytes > 0),
    accessed_at INTEGER NOT NULL,
    PRIMARY KEY(blob_key, recipe)
) STRICT;
CREATE INDEX preview_eviction ON preview_cache(accessed_at, blob_key, recipe);
