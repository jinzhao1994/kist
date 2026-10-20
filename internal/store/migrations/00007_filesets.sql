-- +goose Up
CREATE TABLE filesets (
 folder_id BLOB PRIMARY KEY REFERENCES folders(id),
 kind TEXT NOT NULL CHECK(kind IN ('live_photo','video_subtitles','document_pages')),
 primary_file_id BLOB REFERENCES files(id) DEFERRABLE INITIALLY DEFERRED,
 layout TEXT NOT NULL CHECK(layout IN ('flat','directory'))
);
CREATE TABLE fileset_members (
 fileset_id BLOB NOT NULL REFERENCES filesets(folder_id),
 position INTEGER NOT NULL CHECK(position>=0),
 file_id BLOB REFERENCES files(id) DEFERRABLE INITIALLY DEFERRED,
 folder_id BLOB REFERENCES filesets(folder_id) DEFERRABLE INITIALLY DEFERRED,
 role TEXT NOT NULL,
 language TEXT NOT NULL,
 is_default INTEGER NOT NULL CHECK(is_default IN (0,1)),
 PRIMARY KEY(fileset_id,position),
 CHECK((file_id IS NULL) != (folder_id IS NULL))
);
CREATE UNIQUE INDEX fileset_member_file ON fileset_members(fileset_id,file_id) WHERE file_id IS NOT NULL;
CREATE UNIQUE INDEX fileset_member_folder ON fileset_members(fileset_id,folder_id) WHERE folder_id IS NOT NULL;
CREATE INDEX fileset_file_refs ON fileset_members(file_id);
CREATE INDEX fileset_folder_refs ON fileset_members(folder_id);
CREATE TABLE op_filesets (
 op_id INTEGER NOT NULL REFERENCES ops(id),
 fileset_id BLOB NOT NULL,
 PRIMARY KEY(op_id,fileset_id)
);
