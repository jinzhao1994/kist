-- +goose Up

-- ── 时间的两种表示 ──────────────────────────────────────────────────────
--
-- 瞬时一律 INTEGER unix 纳秒：ops.ts、由它派生的 created_at / updated_at，
-- 以及 captured_at。纳秒是 time.Time 的原生精度，写进去和读出来逐位相同，
-- 不需要先截断。源本身有多少精度是源的属性——EXIF 落在整秒上，文件系统
-- mtime 留住它记下的东西——存储不替它们做取舍。
--
-- 墙钟（start_at / end_at，domain.LocalDate）是 TEXT：变精度（"2019" 和
-- "2019-08" 的精度由长度承载）加无时区语义，整数表达不了。
--
-- 这条线是 1.5.2 那条原则的延伸：不为了统一表示去合并两个不同的概念。
-- 加时间列之前先问它是哪一种。
--
-- int64 纳秒能表示 1678–2262 年。

-- ── 权威数据：ops。永不 UPDATE，永不 DELETE（spec 2.1、3.9）──────────────

-- AUTOINCREMENT 而非裸 INTEGER PRIMARY KEY：id 就是版本号，整个历史模型
-- 建立在它单调递增上。裸 rowid 会在删行后被复用，而"不删 op"是个靠自觉的
-- 约定，值得用一行 SQL 变成结构保证。
--
-- 顺序的权威是 id，不是 ts。墙钟会被 NTP 校正往回拨，所以 ts 无论什么精度
-- 都不是可靠的排序键；需要顺序的地方一律用 id。ts 也因此不需要唯一。
CREATE TABLE ops (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  ts         INTEGER NOT NULL,   -- unix 纳秒（UTC）。见文件头"时间的两种表示"
  action     TEXT    NOT NULL,   -- Go 侧 typed 常量；DB 层刻意不加 CHECK（spec 2.2）
  origin     TEXT    NOT NULL,   -- user | extractor | maintenance | sync。
                                 -- 刻意不叫 source：那个词归 file_metadata.source
                                 -- 和 file_tag_sources.source，它们存的是"哪一个
                                 -- 提取器"（身份），这一列存的是"哪一类执行者"
                                 -- （类别）。同一条 import_files 里两者取值不同
                                 -- （origin=user，行上 source=file），所以它们
                                 -- 推导不出彼此
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

-- 内容的历史。file 行会随删除消失，blob 不会，所以 blob -> file -> op 答不了
-- 孤儿 blob。
CREATE TABLE op_blobs (
  op_id    INTEGER NOT NULL REFERENCES ops(id),
  blob_key TEXT    NOT NULL,
  PRIMARY KEY (blob_key, op_id)
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
  created_at       INTEGER NOT NULL,  -- unix 纳秒，取自 ops.ts 而非 time.Now()（spec 2.3.1）
  updated_at       INTEGER NOT NULL,
  start_at         TEXT,              -- 变精度 ISO-8601 前缀，精度由长度推导（spec 1.3）
  end_at           TEXT,
  root_folder_id   BLOB NOT NULL REFERENCES folders(id)   -- box → 根 folder 的唯一一条边（spec 1.4）
);
-- 一个 folder 只能当一个 box 的根。加上 root_folder_id 是单列，"一个 box 恰好
-- 一个根"就不再靠索引拦，而是根本表示不出来。
CREATE UNIQUE INDEX boxes_root ON boxes(root_folder_id);
CREATE INDEX boxes_updated ON boxes(updated_at DESC);
CREATE INDEX boxes_name_fold ON boxes(name_fold);

CREATE TABLE folders (
  id         BLOB NOT NULL PRIMARY KEY,
  parent_id  BLOB REFERENCES folders(id),
  name       TEXT NOT NULL,   -- 根 folder 为 ''
  name_fold  TEXT NOT NULL,
  created_at INTEGER NOT NULL,   -- unix 纳秒
  updated_at INTEGER NOT NULL
);
-- 部分索引而不是朴素的 UNIQUE(parent_id, name)：根 folder 的 parent_id 是
-- NULL，而 SQL 里 NULL 互不相等，朴素写法对根目录根本不触发（spec 1.4）。
-- "一个 box 两个根"不需要索引拦，boxes.root_folder_id 是单列。
CREATE UNIQUE INDEX folders_parent_name ON folders(parent_id, name) WHERE parent_id IS NOT NULL;
CREATE INDEX folders_parent ON folders(parent_id);

CREATE TABLE files (
  id          BLOB NOT NULL PRIMARY KEY,
  folder_id   BLOB NOT NULL REFERENCES folders(id),
  name        TEXT NOT NULL,
  name_fold   TEXT NOT NULL,
  blob_key    TEXT NOT NULL REFERENCES blobs(key),
  captured_at INTEGER,                                -- unix 纳秒。内核从 metadata 算出的结论（spec 1.5.2）
  created_at  INTEGER NOT NULL,                       -- unix 纳秒
  updated_at  INTEGER NOT NULL
);
CREATE UNIQUE INDEX files_folder_name ON files(folder_id, name);
CREATE INDEX files_folder ON files(folder_id, id);
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

-- 一行就是"有这个 tag"，没有例外（spec 1.6.1）。来源不在这里——一个 tag
-- 可以被多个来源认领，放进来就得进主键，而那会让 TagDirectCounts 之类的
-- count(*) 静默翻倍。
CREATE TABLE file_tags (
  file_id BLOB NOT NULL REFERENCES files(id),
  tag_id  BLOB NOT NULL REFERENCES tags(id),
  PRIMARY KEY (file_id, tag_id)
);
CREATE INDEX file_tags_tag ON file_tags(tag_id, file_id);

-- 谁认领了这个 tag。source 是提取器 id，或保留值 "user"——和
-- file_metadata.source 同一套值（spec 1.6.1 ①）。
--
-- 多行：exif 和 OCR 可以都认为这张照片是"猫"。重跑 exif 只删自己的行，
-- 主表那行要等到没有任何来源认领了才消失。
--
-- 只有提取管线查它，和 tag_rejections 同一个位置。
CREATE TABLE file_tag_sources (
  file_id BLOB NOT NULL REFERENCES files(id),
  tag_id  BLOB NOT NULL REFERENCES tags(id),
  source  TEXT NOT NULL,
  PRIMARY KEY (file_id, tag_id, source)
);

-- box 侧没有自动推导，所以没有来源可记：每一行都会是同一个值。真要有 box
-- 级推导时，照 file 的形状拆一张 box_tag_sources，那时存量行全是 "user"。
CREATE TABLE box_tags (
  box_id BLOB NOT NULL REFERENCES boxes(id),
  tag_id BLOB NOT NULL REFERENCES tags(id),
  PRIMARY KEY (box_id, tag_id)
);
CREATE INDEX box_tags_tag ON box_tags(tag_id, box_id);

-- 提取器的抑制名单，不是可筛选的"负向 tag"（spec 1.6.1）。v0.1 建表但空着。
CREATE TABLE tag_rejections (
  file_id BLOB NOT NULL REFERENCES files(id),
  tag_id  BLOB NOT NULL REFERENCES tags(id),
  PRIMARY KEY (file_id, tag_id)
);

-- v0.1 只写 extracted 侧（spec 1.7）。
--
-- 只挂在 file 上（spec 1.7.3）。第一版是一张多态表 (target_type, target_id)
-- 好让它也能描述 box，那个形状建不了外键——target_id 指着两张表——于是删掉
-- 一个文件会留下没人拦得住的孤儿行。而 1.6 早就为同样的形状给出过相反的
-- 结论：box_tags 和 file_tags 是两张表，正是为了各自能指认自己的父亲。
--
-- 没有 box metadata：检索归 tag，日期归 boxes 的列，聚合算出来不存。
-- 真需要时加一张 box_metadata 是零数据的纯增量迁移（spec 8）。
--
-- value 是提取器看到的原样（"f/1.8"、带前导零的 "0100"），type 是提取器
-- 声明的类型——不由 Apply 推断，否则换个数字识别逻辑就会让 rebuild 产出
-- 一个不同的库（spec 1.7.5、不变量 3）。
--
-- v0.1 不建非主键索引，因为 v0.1 不查 metadata。数值比较要的是另外的
-- value_int / value_real 旁列，同样等到需要时再加（spec 1.7.5、8）。
-- value / type 为 NULL 的行是墓碑：人删掉了这个 key，而不是这个 key 从来
-- 没有过。两者必须分得开，否则下一次重新提取会把它加回来（spec 1.7.7）。
-- 墓碑的 source 恒为 "user"，抑制因此不需要新机制——提取管线本来就会跳过
-- 不属于自己的 key。
CREATE TABLE file_metadata (
  file_id BLOB NOT NULL REFERENCES files(id),
  key     TEXT NOT NULL,   -- 必须带命名空间："exif:Make"、"file:mtime"、"user:有效期"
  value   TEXT,            -- NULL = 墓碑
  type    TEXT,            -- text | int | real；墓碑为 NULL
  source  TEXT NOT NULL,   -- 提取器 id，或保留值 "user" / "file"
  PRIMARY KEY (file_id, key)
);

-- "哪些文件还没被 exif v3 处理过"（spec 1.7.6 ③）。payload 里也记了提取器
-- 和版本，但 payload 不可查询，所以按不变量 4 提成索引表——和 op_boxes /
-- op_files 是同一个形状，都能从 payload 单向重建。
--
-- 一个 (file, extractor) 一行：重跑是替换版本，不是追加。否则"这个文件被
-- 提取过吗"的答案取决于你恰好读到哪一行。
--
-- captured_at 的解析规则也注册在这里，用保留 id "kist:captured"（spec 1.5.2）。
-- 规则版本和提取器版本共用一张表，所以"规则改进了、哪些文件要重算"不需要
-- 另造一套机制。
CREATE TABLE file_extractions (
  file_id   BLOB NOT NULL REFERENCES files(id),
  extractor TEXT NOT NULL,
  version   INTEGER NOT NULL,
  PRIMARY KEY (file_id, extractor)
);

-- 没有 Down 方向。`DROP TABLE ops` 不是回滚是删库——ops 是权威数据（不变量
-- 1），blobs/ 里的内容还在，但哪个文件叫什么、在哪个 box、打了什么 tag 全没了。
-- 物化视图坏了的恢复路径是 `kist rebuild`，从 ops 重放。
