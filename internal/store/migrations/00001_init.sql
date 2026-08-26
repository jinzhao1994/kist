-- +goose Up

-- ── 权威数据：ops。永不 UPDATE，永不 DELETE（spec 2.1、3.9）──────────────

-- AUTOINCREMENT 而非裸 INTEGER PRIMARY KEY：id 就是版本号，整个历史模型
-- 建立在它单调递增上。裸 rowid 会在删行后被复用，而"不删 op"是个靠自觉的
-- 约定，值得用一行 SQL 变成结构保证。
CREATE TABLE ops (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  ts         TEXT    NOT NULL,   -- UTC, 2006-01-02T15:04:05.000Z
  action     TEXT    NOT NULL,   -- Go 侧 typed 常量；DB 层刻意不加 CHECK（spec 2.2）
  source     TEXT    NOT NULL,   -- user | extractor | maintenance | sync
  actor      TEXT,
  payload    TEXT    NOT NULL,   -- 不透明 JSON，绝不查进去（spec 2.2.1）
  reverts_id INTEGER REFERENCES ops(id)
);
CREATE INDEX ops_reverts ON ops(reverts_id) WHERE reverts_id IS NOT NULL;

-- payload 的索引表。派生的、可从 payload 单向重建。
-- 刻意不对 boxes/files 建外键：历史必须比它的主体活得久，删掉一个 box
-- 不能把"删除了这个 box"这条记录一起带走（spec 4.6）。
CREATE TABLE op_boxes (
  op_id  INTEGER NOT NULL REFERENCES ops(id),
  box_id BLOB    NOT NULL,
  PRIMARY KEY (box_id, op_id)
);
CREATE TABLE op_files (
  op_id   INTEGER NOT NULL REFERENCES ops(id),
  file_id BLOB    NOT NULL,
  PRIMARY KEY (file_id, op_id)
);

-- ── 内容索引：blobs。可从扫 blobs/ 目录重建（spec 2.1）─────────────────

CREATE TABLE blobs (
  key    TEXT    PRIMARY KEY,  -- "sha256:abcd…"，碰撞时 "sha256:abcd….1"
  sha256 TEXT    NOT NULL,     -- 裸值。刻意非唯一：碰撞会产生同 digest 的两行
  size   INTEGER NOT NULL
);
CREATE INDEX blobs_sha256 ON blobs(sha256);

-- ── 物化视图：全部可从 ops 重放重建 ────────────────────────────────────

CREATE TABLE boxes (
  id               BLOB PRIMARY KEY,
  name             TEXT NOT NULL,
  name_fold        TEXT NOT NULL,  -- Go 生成（spec 4.7）
  description      TEXT NOT NULL DEFAULT '',
  description_fold TEXT NOT NULL DEFAULT '',
  created_at       TEXT NOT NULL,  -- 取自 ops.ts，非 time.Now()（spec 2.3.1）
  updated_at       TEXT NOT NULL,
  start_at         TEXT,           -- 变精度 ISO-8601 前缀，精度由长度推导（spec 1.3）
  end_at           TEXT
);
CREATE INDEX boxes_updated ON boxes(updated_at DESC);
CREATE INDEX boxes_name_fold ON boxes(name_fold);

CREATE TABLE folders (
  id         BLOB NOT NULL PRIMARY KEY,
  box_id     BLOB NOT NULL REFERENCES boxes(id),
  parent_id  BLOB REFERENCES folders(id),
  name       TEXT NOT NULL,   -- 根 folder 为 ''
  name_fold  TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
-- 两条部分索引而不是一条 UNIQUE(parent_id, name)：根 folder 的 parent_id 是
-- NULL，而 SQL 里 NULL 互不相等，朴素写法拦不住"一个 box 两个根目录"，
-- 那会让整棵投影树分叉（spec 1.4）。
CREATE UNIQUE INDEX folders_parent_name ON folders(parent_id, name) WHERE parent_id IS NOT NULL;
CREATE UNIQUE INDEX folders_one_root ON folders(box_id) WHERE parent_id IS NULL;
CREATE INDEX folders_box ON folders(box_id);

CREATE TABLE files (
  id          BLOB NOT NULL PRIMARY KEY,
  box_id      BLOB NOT NULL REFERENCES boxes(id),   -- 冗余但必要：最热的筛选路径（spec 1.5）
  folder_id   BLOB NOT NULL REFERENCES folders(id),
  name        TEXT NOT NULL,
  name_fold   TEXT NOT NULL,
  blob_key    TEXT NOT NULL REFERENCES blobs(key),
  captured_at TEXT,                                  -- 从 EXIF 提取，M3 才会有值
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);
CREATE UNIQUE INDEX files_folder_name ON files(folder_id, name);
CREATE INDEX files_box ON files(box_id, id);
CREATE INDEX files_captured ON files(captured_at DESC, id DESC);
CREATE INDEX files_name_fold ON files(name_fold);
CREATE INDEX files_blob ON files(blob_key);

CREATE TABLE tags (
  id        BLOB NOT NULL PRIMARY KEY,
  parent_id BLOB REFERENCES tags(id),
  name      TEXT NOT NULL,
  name_fold TEXT NOT NULL
);
-- 全局唯一，层级不构成命名空间（spec 1.6）。建在折叠列上，否则
-- "Apple" 和 "apple" 会是两个 tag。
CREATE UNIQUE INDEX tags_name_fold ON tags(name_fold);
CREATE INDEX tags_parent ON tags(parent_id);

CREATE TABLE file_tags (
  file_id BLOB NOT NULL REFERENCES files(id),
  tag_id  BLOB NOT NULL REFERENCES tags(id),
  source  TEXT NOT NULL,   -- user | derived
  PRIMARY KEY (file_id, tag_id)
);
CREATE INDEX file_tags_tag ON file_tags(tag_id, file_id);

CREATE TABLE box_tags (
  box_id BLOB NOT NULL REFERENCES boxes(id),
  tag_id BLOB NOT NULL REFERENCES tags(id),
  source TEXT NOT NULL,
  PRIMARY KEY (box_id, tag_id)
);
CREATE INDEX box_tags_tag ON box_tags(tag_id, box_id);

-- 提取器的抑制名单，不是可筛选的"负向 tag"（spec 1.6.1）。v0.1 建表但空着。
CREATE TABLE tag_rejections (
  file_id BLOB NOT NULL REFERENCES files(id),
  tag_id  BLOB NOT NULL REFERENCES tags(id),
  PRIMARY KEY (file_id, tag_id)
);

-- v0.1 只写 extracted 侧，且只有 file:size / file:mtime（spec 1.7）。
CREATE TABLE metadata (
  target_type TEXT NOT NULL,   -- box | file
  target_id   BLOB NOT NULL,
  key         TEXT NOT NULL,   -- 必须带命名空间："exif:Make"、"file:mtime"
  value       TEXT NOT NULL,
  source      TEXT NOT NULL,   -- extracted | user
  PRIMARY KEY (target_type, target_id, key)
);

-- +goose Down
DROP TABLE metadata;
DROP TABLE tag_rejections;
DROP TABLE box_tags;
DROP TABLE file_tags;
DROP TABLE tags;
DROP TABLE files;
DROP TABLE folders;
DROP TABLE boxes;
DROP TABLE blobs;
DROP TABLE op_files;
DROP TABLE op_boxes;
DROP TABLE ops;
