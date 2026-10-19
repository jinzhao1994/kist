-- +goose Up
-- Logical bytes of current files, including descendants and repeated references.
ALTER TABLE folders ADD COLUMN total_size INTEGER NOT NULL DEFAULT 0
    CHECK(typeof(total_size) = 'integer' AND total_size >= 0);

-- One-time backfill from indexed metadata; never read file contents.
WITH RECURSIVE ancestry(descendant, ancestor) AS (
    SELECT id, id FROM folders
    UNION
    SELECT a.descendant, f.parent_id FROM ancestry a
    JOIN folders f ON f.id = a.ancestor WHERE f.parent_id IS NOT NULL
), direct AS (
    SELECT f.folder_id, sum(b.size) AS size FROM files f
    JOIN blobs b ON b.key = f.blob_key GROUP BY f.folder_id
), totals AS (
    SELECT a.ancestor, sum(d.size) AS size FROM ancestry a
    JOIN direct d ON d.folder_id = a.descendant GROUP BY a.ancestor
)
UPDATE folders SET total_size = coalesce((SELECT size FROM totals WHERE ancestor = folders.id), 0);

-- +goose StatementBegin
CREATE TRIGGER file_size_insert AFTER INSERT ON files

BEGIN
    UPDATE folders SET total_size = total_size + (SELECT size FROM blobs WHERE key = NEW.blob_key)
    WHERE id IN (
        WITH RECURSIVE ancestors(id) AS (
            SELECT id FROM folders WHERE id = NEW.folder_id
            UNION
            SELECT f.parent_id FROM folders f JOIN ancestors a ON f.id = a.id
            WHERE f.parent_id IS NOT NULL
        ) SELECT id FROM ancestors
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER file_size_delete AFTER DELETE ON files

BEGIN
    UPDATE folders SET total_size = total_size + (-(SELECT size FROM blobs WHERE key = OLD.blob_key))
    WHERE id IN (
        WITH RECURSIVE ancestors(id) AS (
            SELECT id FROM folders WHERE id = OLD.folder_id
            UNION
            SELECT f.parent_id FROM folders f JOIN ancestors a ON f.id = a.id
            WHERE f.parent_id IS NOT NULL
        ) SELECT id FROM ancestors
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER file_size_update AFTER UPDATE OF folder_id, blob_key ON files
WHEN OLD.folder_id != NEW.folder_id OR OLD.blob_key != NEW.blob_key
BEGIN
    UPDATE folders SET total_size = total_size + (-(SELECT size FROM blobs WHERE key = OLD.blob_key))
    WHERE id IN (
        WITH RECURSIVE ancestors(id) AS (
            SELECT id FROM folders WHERE id = OLD.folder_id
            UNION
            SELECT f.parent_id FROM folders f JOIN ancestors a ON f.id = a.id
            WHERE f.parent_id IS NOT NULL
        ) SELECT id FROM ancestors
    );
    UPDATE folders SET total_size = total_size + (SELECT size FROM blobs WHERE key = NEW.blob_key)
    WHERE id IN (
        WITH RECURSIVE ancestors(id) AS (
            SELECT id FROM folders WHERE id = NEW.folder_id
            UNION
            SELECT f.parent_id FROM folders f JOIN ancestors a ON f.id = a.id
            WHERE f.parent_id IS NOT NULL
        ) SELECT id FROM ancestors
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER folder_size_move AFTER UPDATE OF parent_id ON folders
WHEN OLD.parent_id IS NOT NEW.parent_id
BEGIN
    UPDATE folders SET total_size = total_size + (-OLD.total_size)
    WHERE id IN (
        WITH RECURSIVE ancestors(id) AS (
            SELECT id FROM folders WHERE id = OLD.parent_id
            UNION
            SELECT f.parent_id FROM folders f JOIN ancestors a ON f.id = a.id
            WHERE f.parent_id IS NOT NULL
        ) SELECT id FROM ancestors
    );
    UPDATE folders SET total_size = total_size + (NEW.total_size)
    WHERE id IN (
        WITH RECURSIVE ancestors(id) AS (
            SELECT id FROM folders WHERE id = NEW.parent_id
            UNION
            SELECT f.parent_id FROM folders f JOIN ancestors a ON f.id = a.id
            WHERE f.parent_id IS NOT NULL
        ) SELECT id FROM ancestors
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER blob_size_update AFTER UPDATE OF size ON blobs
WHEN OLD.size != NEW.size
BEGIN
    UPDATE folders SET total_size = total_size + (NEW.size - OLD.size) * (
        WITH RECURSIVE ancestry(file, ancestor) AS (
            SELECT id, folder_id FROM files WHERE blob_key = NEW.key
            UNION
            SELECT a.file, f.parent_id FROM ancestry a JOIN folders f ON f.id = a.ancestor
            WHERE f.parent_id IS NOT NULL
        ) SELECT count(*) FROM ancestry WHERE ancestor = folders.id
    ) WHERE id IN (
        WITH RECURSIVE ancestors(id) AS (
            SELECT folder_id FROM files WHERE blob_key = NEW.key
            UNION
            SELECT f.parent_id FROM folders f JOIN ancestors a ON f.id = a.id
            WHERE f.parent_id IS NOT NULL
        ) SELECT id FROM ancestors
    );
END;
-- +goose StatementEnd

