# M3 提取实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `find --sort captured` 第一次返回真实结果。在此之前 `captured_at` 整列是 NULL，那个排序退化成纯 `id DESC`。附带一个能接第三方提取器的契约。

**Architecture:** 新包 `internal/extract` 拥有提取器契约，**只依赖 `domain`**（和 `projection` / `query` 同规则）——这既让它脱离数据库可测，也保证插件够不到写入路径。

提取器在 **op 之外**跑，结果记进 `extract_metadata` 的 payload，`Apply` 只把记下来的东西写进表。这是 1.7.1 那个决定买下的能力：提取器可以任意不纯（调 ffmpeg、读网络、跑模型），都威胁不到重放的确定性。

`captured_at` **不是任何提取器的产出**，是内核从若干产出里算出的结论（1.5.2）。解析规则住在 `internal/extract` 里但不可插拔，注册在 `file_extractions` 的保留 id `kist:captured` 下。

**Tech Stack:** 新增一个依赖 `github.com/bep/imagemeta`（纯 Go，EXIF / IPTC / XMP，JPEG + HEIC + TIFF + RAW）。实测见 Task 3。

**Spec:** `docs/design/2026-08-22-kist-v0.1-design.md`。**1.5.2（`captured_at`）和 1.7（metadata 与提取器契约）是本计划的正文**；2.2.1（payload 与索引表）、2.3（Apply / Inverse）、3.7.1（中断与部分失败）、4.2.1 / 4.5.1（时间筛选与排序）、4.7（引擎分歧）都要照着。

**前置：** M1 + M2a + M2b 全部完成。**M3 不再有设计前置**——`open-questions.md` B1f 已经全部定完，A1（HEIC / ffmpeg）已经完全移出 M3。

## Global Constraints

- **Go 1.26.1**；**`CGO_ENABLED=0` 四平台可构建**。新依赖必须是纯 Go
- **不变量 2**：`cmd` 不得 import `store` / `oplog`
- **不变量 3**：`Apply` 必须是纯函数。**类型识别、时区选择、回退链的每一次选择都在生成 op 时做完，结果写进 payload**
- **不变量 4**：payload 是不可查询的载荷。提取器 id 与版本要提成 `file_extractions`
- **`internal/extract` 只许 import `domain`**，由 `internal/arch` 守着
- **语言：代码、注释、godoc 用英文；设计文档与 commit message 用中文**
- 每个 task 结束时 `make check` 必须通过
- **每一条守护性测试都要先验证它会红**，再接受它。这是本仓库从 M1b 起的习惯，M2a / M2b 各靠它抓到过计划里的错误论断

---

## 五条要一直记着的原则

### ① 提取器产出观察，内核产出结论

```
提取器（可插拔）                    内核（不可插拔）
──────────────                    ──────────────
exif:DateTimeOriginal   ┐
exif:OffsetTimeOriginal ├────────▶ captured_at
geo:timezone            │
file:mtime              ┘
```

一个用户插件永远碰不到排序键。它只能往**自己声明的命名空间**里放观察。这是信任边界，不是分层洁癖。

### ② 只写真的变了的

和 `tag_files` 同规则（`TestTagFilesSkipsFilesThatAlreadyHaveIt`）。重提取时 payload 只带发生变化的键，连同 before 值供 `Inverse`。

后果：首次提取 before 为空、不翻倍；同一个提取器同一个版本重跑，**一条 op 都不写**。

### ③ 值是字符串 + 类型标签，不是 typed 字段

```go
type KV struct {
    Key   string `json:"key"`
    Value string `json:"value"`   // 提取器看到的原样，十进制
    Type  string `json:"type"`    // text | int | real
}
```

三条理由见 1.7.5。实现上最要紧的一条：**类型由提取器声明，`Apply` 只按标签路由**。换一个更严格的数字识别逻辑不能让 `rebuild` 产出不同的库。

### ④ 展开规则按形状，不按有用性

标量、可打印、有界的标签全展开；二进制块（嵌入缩略图、ICC profile、MakerNote 原始字节）不展开。

判据是客观的、不会过期的。实测数据见 Task 3——一张 iPhone 15 Pro 的 HEIC 里 `MakerNoteApple` 就是一坨二进制，正是这条规则要挡的。

### ⑤ 两种时间

| | 类型 | 翻译 |
|---|---|---|
| `Box.start_at` / `end_at` | `LocalDate` | 字符串区间重叠，**不动** |
| `File.captured_at` | unix 秒 | `LocalDate.Start()/End()` 按库时区转成瞬时，`BETWEEN` |

conformance suite 的时间那一组要跟着分成两半。

---

## 文件结构

```
internal/extract/                 ★ 新包，只许 import domain
  extract.go                        KV / Type / Extractor / Result
  registry.go                       注册表 + 命名空间所有权校验
  captured.go                       captured_at 的解析规则（内核所有）
  exif/
    exif.go                         内置的 exif 提取器
    tags.go                         imagemeta 的标签 → kist 的命名空间与类型
    testdata/                       自制的小图（见 Task 3 的许可要求）

internal/store/
  migrations/00001_init.sql       改：file_metadata / file_extractions / captured_at
  views.go                        改：ClearViews、metadata 的读写
  metadata.go                     ★ 新：file_metadata / file_extractions 的读写
  snapshot.go                     改：snapshotTables
  translate.go                    改：文件侧的时间谓词变成瞬时
  conformance_test.go             改：时间那一组

internal/oplog/
  metadata.go                     ★ 新：extract_metadata 的 Apply / Inverse / Targets
  tx.go                           改：加 metadata 与 extractions 的方法

internal/library/
  extract.go                      ★ 新：Extract —— 分批、可中断、per-file 结果

cmd/kist/
  extract.go                      ★ 新：kist extract
  file.go                         改：kist file show 展示 metadata

internal/arch/arch_test.go        改：加 extract 的依赖规则
```

---

### Task 1: schema —— 改初始迁移，不加新迁移

**Files:**
- Modify: `internal/store/migrations/00001_init.sql`
- Modify: `internal/store/views.go`（`ClearViews`）
- Modify: `internal/store/snapshot.go`（`snapshotTables`）
- Test: `internal/store/migrate_test.go`

**Interfaces:**
- Produces: `file_metadata(file_id, key, value, type, source)` PK `(file_id, key)`；`file_extractions(file_id, extractor, version)` PK `(file_id, extractor)`；`files.captured_at INTEGER`

**为什么改初始迁移而不是加一个新的：`metadata` 表现在一行数据都没有**，`captured_at` 整列是 NULL。没有任何现存库需要迁移，而多一个迁移文件是永久的阅读成本。这是这个决定唯一的窗口期。

- [ ] **Step 1: 先写会红的测试**

`internal/store/migrate_test.go` 里加：

```go
// spec 1.7.3：metadata 只挂在 file 上，而且要有真外键 —— 多态的
// (target_type, target_id) 建不了外键，删掉一个文件就会留下数据库拦不住的
// 孤儿。1.6 早就为 box_tags / file_tags 写下过同样的结论。
func TestFileMetadataHasARealForeignKey(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`INSERT INTO file_metadata (file_id, key, value, type, source)
	                   VALUES (X'00000000000000000000000000000000', 'exif:Make', 'Apple', 'text', 'exif')`)
	if err == nil {
		t.Error("a metadata row for a file that does not exist was accepted")
	}
}

// spec 1.7.6 ③：「哪些文件还没被 X v3 处理过」必须可查，所以提取器和版本
// 是索引表而不是 payload 里的字段（不变量 4）。
func TestFileExtractionsIsOnePerFilePerExtractor(t *testing.T) {
	db := openTestDB(t)
	file := insertTestFile(t, db)
	if _, err := db.Exec(`INSERT INTO file_extractions (file_id, extractor, version) VALUES (?, 'exif', 1)`, file); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO file_extractions (file_id, extractor, version) VALUES (?, 'exif', 2)`, file); err == nil {
		t.Error("two rows for the same (file, extractor) were accepted; the version must be replaced, not appended")
	}
}

// spec 1.5.2：captured_at 是 unix 秒的瞬时，不是变精度墙钟。存成 TEXT 会
// 让 1960 年的老照片（负数时间戳）排在所有正数前面。
func TestCapturedAtIsAnInteger(t *testing.T) {
	db := openTestDB(t)
	var typ string
	err := db.QueryRow(`SELECT type FROM pragma_table_info('files') WHERE name = 'captured_at'`).Scan(&typ)
	if err != nil {
		t.Fatal(err)
	}
	if typ != "INTEGER" {
		t.Errorf("files.captured_at is %s, want INTEGER (spec 1.5.2)", typ)
	}
}
```

- [ ] **Step 2: 跑一遍，确认三条都红**

`go test ./internal/store/ -run 'FileMetadata|FileExtractions|CapturedAtIsAnInteger'`
预期：编译通过但三条都失败（表不存在 / 列类型不对）。

- [ ] **Step 3: 改迁移**

`00001_init.sql` 里把 `metadata` 整块换掉：

```sql
-- v0.1 只写 extracted 侧（spec 1.7）。key 必须带命名空间，value 是提取器
-- 看到的原样，type 是提取器声明的类型（spec 1.7.5 —— 不由 Apply 推断，
-- 否则换个数字识别逻辑就会让 rebuild 产出不同的库）。
--
-- 只挂在 file 上（spec 1.7.3）。没有 box metadata：检索归 tag，日期归
-- boxes 的列，聚合算出来。加一张 box_metadata 是零数据的纯增量迁移，
-- 而多态的 target_id 建不了外键。
CREATE TABLE file_metadata (
  file_id BLOB NOT NULL REFERENCES files(id),
  key     TEXT NOT NULL,   -- "exif:Make"、"file:mtime"、"user:有效期"
  value   TEXT NOT NULL,
  type    TEXT NOT NULL,   -- text | int | real
  source  TEXT NOT NULL,   -- 提取器 id，或保留值 "user"
  PRIMARY KEY (file_id, key)
);

-- "哪些文件还没被 X v3 处理过"（spec 1.7.6 ③）。和 op_files 一样，是从
-- payload 单向重建的索引表 —— payload 不可查询（不变量 4）。
-- captured_at 的解析规则也注册在这里，用保留 id "kist:captured"。
CREATE TABLE file_extractions (
  file_id   BLOB NOT NULL REFERENCES files(id),
  extractor TEXT NOT NULL,
  version   INTEGER NOT NULL,
  PRIMARY KEY (file_id, extractor)
);
```

`files` 的 `captured_at` 从 `TEXT` 改成 `INTEGER`，注释改成：

```sql
  captured_at INTEGER,          -- unix 秒。内核从 metadata 算出的结论，spec 1.5.2
```

`-- +goose Down` 里 `DROP TABLE metadata` 换成两条 `DROP TABLE file_metadata` / `file_extractions`。

- [ ] **Step 4: 改 `ClearViews` 和 `snapshotTables`**

`views.go`：

```go
for _, table := range []string{
    "box_tags", "file_tags", "tag_rejections", "file_metadata", "file_extractions",
    "files", "folders", "boxes", "tags",
} {
```

**两张新表都进 `ClearViews`**：它们都是 op 的物化视图（1.7.1），`rebuild` 清空重放。这和 `blobs` 不同——那张表是 `blobs/` 目录的索引，所以不清。

`snapshot.go` 的 `snapshotTables`：

```go
{"file_metadata", "hex(file_id), key, value, type, source", "file_id, key"},
{"file_extractions", "hex(file_id), extractor, version", "file_id, extractor"},
```

原来那行 `{"metadata", …}` 删掉。

- [ ] **Step 5: 跑测试，三条转绿**

`go test ./internal/store/`

- [ ] **Step 6: 验证守护会红**

把 `file_metadata` 的 `REFERENCES files(id)` 去掉，`TestFileMetadataHasARealForeignKey` 必须红。改回来。

把 `snapshotTables` 里的 `file_metadata` 那行注释掉，`internal/library` 的 rebuild 快照测试**应该仍然绿**（因为还没有 metadata 数据）——**这是预期的**，Task 8 会补上真正咬得住的那一版。记在这里免得有人以为快照已经覆盖了。

- [ ] **Step 7: `make check` + commit**

```bash
git add -A
git commit -m "feat(store): file_metadata 与 file_extractions，captured_at 改成瞬时

改的是初始迁移而不是加一个新迁移，因为那张表现在一行数据都没有、
captured_at 整列是 NULL。这是这个决定唯一的窗口期。

file_metadata 带真外键（spec 1.7.3）——多态的 (target_type, target_id)
建不了外键，删文件会留下数据库拦不住的孤儿，而 1.6 早就为 box_tags /
file_tags 写下过同样的结论。

file_extractions 是索引表不是 payload 字段，因为「哪些文件还没被 X v3
处理过」必须可查（不变量 4）。"
```

---

### Task 2: `internal/extract` 的契约

**Files:**
- Create: `internal/extract/extract.go`
- Create: `internal/extract/registry.go`
- Test: `internal/extract/extract_test.go`, `internal/extract/registry_test.go`
- Modify: `internal/arch/arch_test.go`

**Interfaces:**
- Produces:
  ```go
  type Type string
  const (TypeText Type = "text"; TypeInt Type = "int"; TypeReal Type = "real")
  type KV struct { Key string; Value string; Type Type }
  type Extractor interface {
      ID() string
      Version() int
      Namespaces() []string
      Wants(mime string) bool
      Extract(ctx context.Context, r io.Reader) ([]KV, error)
  }
  func Register(e Extractor)
  func Lookup(id string) (Extractor, error)
  func All() []Extractor
  func For(mime string) []Extractor
  const UserSource = "user"
  const ReservedNamespace = "user"
  ```

- [ ] **Step 1: 写会红的测试**

```go
// spec 1.7.6 ②：命名空间归提取器所有，不许重叠。没有这条，一个用户插件
// 可以静默覆盖相机写的 exif:Make。
func TestRegisterRejectsAnOverlappingNamespace(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "exif", ns: []string{"exif"}}); err != nil {
		t.Fatal(err)
	}
	err := r.register(fake{id: "evil", ns: []string{"exif"}})
	if err == nil {
		t.Fatal("two extractors were allowed to claim the same namespace")
	}
	if !strings.Contains(err.Error(), "exif") {
		t.Errorf("the error does not name the contested namespace: %v", err)
	}
}

// user: 是人手填的那一半（spec 1.7 硬规则 ②）。一个提取器写进去，就会把
// 用户填的护照有效期洗掉 —— 而重新扫描本来就永远不该碰它。
func TestRegisterRejectsTheReservedNamespace(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "greedy", ns: []string{"user"}}); err == nil {
		t.Error("an extractor was allowed to claim the user namespace")
	}
}

// 同理，"user" 不能被当成提取器 id —— source 列上它是保留值。
func TestRegisterRejectsTheReservedID(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "user", ns: []string{"whatever"}}); err == nil {
		t.Error("an extractor was allowed to call itself user")
	}
}

// 一个提取器必须声明至少一个命名空间，否则它写出来的东西无处安放，而
// 「只替换这个提取器写的行」这条规则也就无从执行。
func TestRegisterRejectsAnExtractorThatOwnsNothing(t *testing.T) {
	r := newRegistry()
	if err := r.register(fake{id: "empty"}); err == nil {
		t.Error("an extractor with no namespace was accepted")
	}
}

// 产出必须落在自己声明的命名空间里。这条在写入路径上再查一次 —— 声明是
// 承诺，检查是执行，而外部进程插件的产出是不可信输入。
func TestCheckRejectsAKeyOutsideTheNamespace(t *testing.T) {
	e := fake{id: "exif", ns: []string{"exif"}}
	err := Check(e, []KV{{Key: "exif:Make", Value: "Apple", Type: TypeText},
		{Key: "user:有效期", Value: "2029", Type: TypeText}})
	if err == nil {
		t.Fatal("a key outside the extractor's namespaces was accepted")
	}
	if !strings.Contains(err.Error(), "user:有效期") {
		t.Errorf("the error does not name the offending key: %v", err)
	}
}

// 类型标签是封闭集合，因为 Apply 按它路由（spec 1.7.5）。一个不认识的
// 标签必须在写 op 之前就被拦下，而不是在重放时。
func TestCheckRejectsAnUnknownType(t *testing.T) {
	e := fake{id: "exif", ns: []string{"exif"}}
	if err := Check(e, []KV{{Key: "exif:X", Value: "1", Type: "rational"}}); err == nil {
		t.Error("an unknown type tag was accepted")
	}
}

// 类型标签必须和值对得上，否则 Apply 里的 strconv 会失败 —— 而那是重放
// 期，太晚了。
func TestCheckRejectsAValueThatDoesNotParse(t *testing.T) {
	e := fake{id: "exif", ns: []string{"exif"}}
	if err := Check(e, []KV{{Key: "exif:ISO", Value: "f/1.8", Type: TypeInt}}); err == nil {
		t.Error("an int-tagged value that is not an integer was accepted")
	}
}
```

- [ ] **Step 2: 跑，确认全红（编译失败即可）**

- [ ] **Step 3: 实现 `extract.go`**

```go
// Package extract defines what an extractor is: what it owns, what it
// produces, and what a library may accept from it.
//
// It knows nothing about databases or the op log. An extractor's output is
// data, and library turns that data into an op — which is what lets an
// extractor be arbitrarily impure (shell out to ffmpeg, call a model, read
// the network) without threatening the determinism of a rebuild (spec 1.7.1).
// That property is the whole reason extraction results live in the log rather
// than being regenerated from blobs/.
package extract

// Type is what an extractor says a value is.
//
// It is declared, never inferred. Inferring "is this string a number" inside
// Apply would make a rebuild depend on which version of the inference ran,
// which is exactly what invariant 3 forbids (spec 1.7.5).
type Type string

const (
	TypeText Type = "text"
	TypeInt  Type = "int"  // fits int64 exactly; stored as a decimal string
	TypeReal Type = "real"
)

// KV is one observation.
//
// Value is what the extractor saw, verbatim and decimal — "f/1.8", "0100"
// with its leading zeros, "1.780" with its trailing one. The display needs
// that, and a typed field would normalize it away. Type says how to read it
// when a queryable projection is wanted (spec 1.7.5).
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  Type   `json:"type"`
}

// UserSource is the reserved value of file_metadata.source for rows a person
// wrote. No extractor may claim it, and no extractor may overwrite such a row
// (spec 1.7 rule ②).
const UserSource = "user"

// ReservedNamespace is the key prefix that belongs to people.
const ReservedNamespace = "user"

type Extractor interface {
	// ID goes in file_metadata.source and file_extractions.extractor. It is
	// how "re-run exif" knows to replace only exif's rows.
	ID() string
	// Version gates re-runs. Nothing re-extracts because it feels like it:
	// a nondeterministic extractor would write a different batch every time
	// (spec 1.7.6 ④).
	Version() int
	// Namespaces are the key prefixes this extractor owns. No two extractors
	// may claim the same one.
	Namespaces() []string
	Wants(mime string) bool
	Extract(ctx context.Context, r io.Reader) ([]KV, error)
}
```

`Check(e Extractor, kvs []KV) error` 逐条校验命名空间归属、类型标签在封闭集合内、值与标签对得上（`strconv.ParseInt` / `ParseFloat`）。

**为什么写入前再查一遍而不是信任声明**：外部进程插件（1.7.6）的产出是不可信输入。而且一个内置提取器的 bug 在这里被拦下，比在 `Apply` 里重放时炸掉便宜得多。

- [ ] **Step 4: 实现 `registry.go`**

包级注册表 + 一个可测的 `newRegistry()`，形状抄 `oplog/registry.go`。`register` 校验：id 非空、id ≠ `UserSource`、至少一个命名空间、命名空间不含 `ReservedNamespace`、不与已注册的重叠。

- [ ] **Step 5: 跑测试，全绿**

- [ ] **Step 6: 加 arch 规则并验证它会咬**

```go
// TestExtractKnowsOnlyDomain keeps the extractor contract testable without a
// library, and keeps plugins away from the write path.
//
// An extractor is a pure function from bytes to observations. If this package
// could see store or library, an extractor could write its own rows — and
// then "extraction results are recorded in the log" would stop being true by
// construction (spec 1.7.1, 1.7.6).
func TestExtractKnowsOnlyDomain(t *testing.T) {
	for _, imp := range imports(t, "internal/extract") {
		if imp != "internal/domain" {
			t.Errorf("internal/extract imports %s.\n\n"+
				"Extractors produce data; library turns it into an op. Reaching for\n"+
				"store or oplog would let an extractor write directly, and the\n"+
				"determinism a rebuild depends on would stop being structural.", imp)
		}
	}
}
```

**验证方式**：往 `internal/extract/extract.go` 的 **import 块里**（不是文件末尾——`build.ImportDir` 看不到那里，M2b 踩过）加一个 `_ "…/internal/store"`，测试必须红。

- [ ] **Step 7: `make check` + commit**

---

### Task 3: `exif` 提取器

**Files:**
- Create: `internal/extract/exif/exif.go`, `internal/extract/exif/tags.go`
- Create: `internal/extract/exif/testdata/`（自制图片，见下）
- Test: `internal/extract/exif/exif_test.go`
- Modify: `go.mod`

**Interfaces:**
- Consumes: Task 2 的 `extract.KV` / `Extractor` / `Check`
- Produces: `exif.New() extract.Extractor`，`ID() == "exif"`，`Namespaces() == []string{"exif", "gps"}`

#### 依赖选型：`github.com/bep/imagemeta`

实测（拿它自带的 `testdata/images/iphone.heic`，iPhone 15 Pro）：

```
=== iphone.heic  (err=<nil>, 58 tags)
  IFD0:Make                          Apple
  IFD0:Model                         iPhone 15 Pro
  IFD0:Orientation                   1
  IFD0/ExifIFDP:DateTimeOriginal     2026:02:19 14:22:37
  IFD0/ExifIFDP:OffsetTimeOriginal   -08:00
  IFD0/ExifIFDP:SubSecTimeOriginal   146
  IFD0/ExifIFDP:ISO                  400
  IFD0/ExifIFDP:FNumber              1244236/699009
  IFD0/ExifIFDP:ExposureTime         1/60
  IFD0/ExifIFDP:LensModel            iPhone 15 Pro back triple camera 6.765mm f/1.78
  IFD0/ExifIFDP:MakerNoteApple       Apple iOSMM:<一坨二进制>
  IFD0/GPSInfoIFD:GPSLatitude        37.93633333333333
  IFD0/GPSInfoIFD:GPSLongitude       122.36528888888888
```

**三条结论直接来自这份实测：**

1. **EXIF 能从 HEIC 里纯 Go 读出来。** iPhone 照片在 M3 就能拿到拍摄时间、机型、GPS——**不需要 ffmpeg**。ffmpeg 只挡住像素（M5），不挡元数据。这比设计文档 5.3 原来暗示的乐观
2. **`OffsetTimeOriginal` 在 iPhone 照片里是有的**（`-08:00`）。1.5.2 的时区链第一档就能命中主力场景，GPS 反查（tzf，第 8 节的逃生舱）确实是可选的
3. **`MakerNoteApple` 就是一坨二进制**，正是原则 ④ 要挡的东西。这条规则不是假想

体积：一个只调 imagemeta 的 hello world 是 **2.6 MB**（裸 Go 约 1.2 MB），即约 +1.2 MB。唯一的间接依赖是 `golang.org/x/text`，kist 已经有了。

**否掉的两条：**

- **`dsoprea/go-exif`** —— HEIC 要再配一个 `go-heic-exif-extractor`，两个包两套 API，而 HEIC 是主流情况不是边角
- **`rwcarlsen/goexif`** —— 事实上停止维护，不支持 HEIC

- [ ] **Step 1: 准备 fixture**

`internal/extract/exif/testdata/` 放三个文件：一张带完整 EXIF 的 JPEG、一张 HEIC、一张没有任何 EXIF 的 PNG。

**许可要求：必须是自己拍的或明确公有领域的**，并在 `testdata/README.md` 里写明出处。不要从依赖的 testdata 里拷——那些有自己的 license。图片要小（各 < 200 KB，必要时裁剪）。

- [ ] **Step 2: 写会红的测试**

```go
// 实测过 iPhone 15 Pro 的 HEIC：EXIF 是纯 Go 读得出来的，不需要 ffmpeg。
// ffmpeg 只挡像素（M5），不挡元数据 —— 这条值得有测试钉住，因为它决定了
// M3 对 iPhone 用户是不是完整的。
func TestExtractsFromHEIC(t *testing.T) {
	kvs := extractFile(t, "testdata/iphone.heic")
	want := map[string]string{
		"exif:Make":                 "Apple",
		"exif:DateTimeOriginal":     "2026:02:19 14:22:37",
		"exif:OffsetTimeOriginal":   "-08:00",
	}
	got := index(kvs)
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

// spec 1.7.2：展开规则按形状，不按有用性。二进制块不展开 —— 而 MakerNote
// 是这条规则在真实文件里的样子。
func TestSkipsBinaryBlobs(t *testing.T) {
	for _, kv := range extractFile(t, "testdata/iphone.heic") {
		if strings.Contains(kv.Key, "MakerNote") {
			t.Errorf("MakerNote was expanded: %s = %.40q", kv.Key, kv.Value)
		}
		for _, r := range kv.Value {
			if r != '\t' && r != '\n' && r < 0x20 {
				t.Errorf("%s holds a control character; the value is not printable: %q", kv.Key, kv.Value)
			}
		}
	}
}

// 每个值都要有一个对得上的类型标签，否则 Apply 里的 strconv 会在重放期
// 失败。Check 是同一个函数，所以这条同时钉住了提取器和契约。
func TestEveryValueSurvivesCheck(t *testing.T) {
	e := New()
	for _, name := range []string{"testdata/iphone.heic", "testdata/camera.jpg"} {
		if err := extract.Check(e, extractFile(t, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// 命名空间只有两个，而且产出必须落在里面（spec 1.7.6 ②）。
func TestKeysStayInsideTheDeclaredNamespaces(t *testing.T) {
	kvs := extractFile(t, "testdata/iphone.heic")
	if len(kvs) == 0 {
		t.Fatal("no tags at all")
	}
	for _, kv := range kvs {
		ns, _, ok := strings.Cut(kv.Key, ":")
		if !ok || (ns != "exif" && ns != "gps") {
			t.Errorf("key %q is outside exif: and gps:", kv.Key)
		}
	}
}

// 没有 EXIF 不是错误。一个 PNG、一份 PDF、一段录音都会走到这里，而
// spec 3.7.1 的规则是「单文件失败不拖累整批」—— 但这根本不是失败。
func TestAFileWithNoExifIsNotAnError(t *testing.T) {
	kvs, err := New().Extract(t.Context(), open(t, "testdata/plain.png"))
	if err != nil {
		t.Fatalf("a file with no EXIF returned an error: %v", err)
	}
	if len(kvs) != 0 {
		t.Errorf("got %d tags from a file with no EXIF", len(kvs))
	}
}

// 同一个文件提两次必须逐字节相同。提取器不必是纯的（spec 1.7.1 就是为了
// 这个才把结果记进 op），但这一个是，而「只写真的变了的」在它身上应该
// 一条都不写。
func TestExtractionIsDeterministic(t *testing.T) {
	a := extractFile(t, "testdata/iphone.heic")
	b := extractFile(t, "testdata/iphone.heic")
	if !reflect.DeepEqual(a, b) {
		t.Error("two extractions of the same file differ")
	}
}
```

- [ ] **Step 3: 跑，确认全红**

- [ ] **Step 4: 实现 `tags.go`——命名空间与类型的映射**

imagemeta 给的是 `IFD0:Make`、`IFD0/ExifIFDP:ISO`、`IFD0/GPSInfoIFD:GPSLatitude`。映射规则：

| imagemeta 的 namespace | kist 的前缀 |
|---|---|
| `IFD0`、`IFD0/ExifIFDP`、`IFD1` | `exif:` |
| `IFD0/GPSInfoIFD` | `gps:` |

**只取标签名，丢掉 IFD 路径。** 理由：`IFD0:Make` 和 `IFD1:Make`（缩略图那份）语义相同，而用户搜的是 `exif:Make`。同名冲突时**先到先得**，因为 imagemeta 的遍历顺序是主图在前。

类型判定按 imagemeta 给的 Go 值：

```go
func typeOf(v any) (string, extract.Type, bool) {
	switch x := v.(type) {
	case string:
		if !printable(x) {
			return "", "", false      // 二进制块，不展开（原则 ④）
		}
		return x, extract.TypeText, true
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(x), extract.TypeInt, true
	case float32, float64:
		return strconv.FormatFloat(toF64(x), 'f', -1, 64), extract.TypeReal, true
	default:
		// Rationals and everything else render as text. FNumber comes out as
		// "1244236/699009" and that is deliberately kept verbatim: it is what
		// the camera wrote, and spec 1.7.5 says the value is what the
		// extractor saw. A decimal approximation would be a second, lossy
		// copy — and nothing queries FNumber in v0.1 anyway.
		s := fmt.Sprint(v)
		if !printable(s) {
			return "", "", false
		}
		return s, extract.TypeText, true
	}
}
```

`printable` 拒绝任何控制字符（`\t` / `\n` 除外）和超过上限的长度。**上限设 1 KB**：真正的标量标签没有这么长的，超过它的一定是被当成字符串的二进制块。

- [ ] **Step 5: 实现 `exif.go`**

```go
func New() extract.Extractor { return exifExtractor{} }

func (exifExtractor) ID() string            { return "exif" }
func (exifExtractor) Version() int          { return 1 }
func (exifExtractor) Namespaces() []string  { return []string{"exif", "gps"} }
func (exifExtractor) Wants(mime string) bool { … }
```

`Extract` 调 `imagemeta.Decode`，`HandleTag` 里做映射与过滤，**输出按 key 排序**——确定性是 `TestExtractionIsDeterministic` 要的，也是「只写真的变了的」能靠比较得出结论的前提。

**`Wants` 按 MIME 而不是扩展名**，因为导入的文件名可能骗人。MIME 从内容嗅探得到（`http.DetectContentType` 够用；HEIC 要自己认 ISO-BMFF 的 `ftyp` box，写一个小函数）。

- [ ] **Step 6: 跑测试，全绿**

- [ ] **Step 7: 验证守护会红**

- 把 `printable` 的长度上限拿掉 → `TestSkipsBinaryBlobs` 必须红
- 把 `typeOf` 里整数那一支改成返回 `TypeText` → `TestEveryValueSurvivesCheck` **不会**红（text 什么都能装）。**这说明那条测试对类型正确性没有约束力**——补一条断言 `exif:ISO` 的 `Type` 必须是 `int`，再验证它会红
- 把输出排序去掉 → `TestExtractionIsDeterministic` 大概率红（map 遍历顺序）。如果**没有**红，说明 `HandleTag` 的顺序本来就稳定，那就把测试改成直接断言输出有序，并把这条发现写进注释

- [ ] **Step 8: `make check` + commit**

---

### Task 4: `captured_at` 的解析规则

**Files:**
- Create: `internal/extract/captured.go`
- Test: `internal/extract/captured_test.go`

**Interfaces:**
- Produces:
  ```go
  const CapturedRule = "kist:captured"   // file_extractions 里的保留 id
  const CapturedRuleVersion = 1

  type Captured struct {
      Unix     int64
      Known    bool
      WallFrom string   // 墙钟来自哪个键："exif:DateTimeOriginal" / "file:mtime" / ""
      ZoneFrom string   // 时区怎么来的："exif:OffsetTimeOriginal" / "geo:timezone" / "library"
  }
  func ResolveCaptured(tags []KV, libraryZone *time.Location) Captured
  ```

**这是内核，不是提取器。** 它不注册进 Task 2 的注册表，`Namespaces()` 之类一概没有。放在 `internal/extract` 里只是因为它的输入是 `[]KV`——**一个插件永远碰不到排序键**（1.5.2 的信任边界）。

- [ ] **Step 1: 写会红的测试**

```go
// spec 1.5.2 的墙钟链：EXIF → 视频 → file:mtime → 没有。
func TestWallClockFallbackChain(t *testing.T) {
	zone := time.FixedZone("test", 8*3600)
	cases := []struct {
		name     string
		tags     []KV
		wantFrom string
		wantUnix int64
	}{
		{
			"EXIF 优先",
			[]KV{
				{Key: "exif:DateTimeOriginal", Value: "2019:08:12 14:03:22", Type: TypeText},
				{Key: "file:mtime", Value: "1700000000", Type: TypeInt},
			},
			"exif:DateTimeOriginal",
			time.Date(2019, 8, 12, 14, 3, 22, 0, zone).Unix(),
		},
		{
			"没有 EXIF 就用 mtime，按库时区转成墙钟再转回去",
			[]KV{{Key: "file:mtime", Value: "1700000000", Type: TypeInt}},
			"file:mtime",
			1700000000,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveCaptured(c.tags, zone)
			if !got.Known {
				t.Fatal("no capture time resolved")
			}
			if got.Unix != c.wantUnix {
				t.Errorf("Unix = %d, want %d", got.Unix, c.wantUnix)
			}
			if got.WallFrom != c.wantFrom {
				t.Errorf("WallFrom = %q, want %q", got.WallFrom, c.wantFrom)
			}
		})
	}
}

// spec 1.5.2 的时区链：offset 标签 → geo:timezone → 库配置的时区。
// 实测过一张 iPhone 15 Pro 的 HEIC 带 OffsetTimeOriginal = -08:00，所以
// 第一档就覆盖了主力场景，GPS 反查是真的可选。
func TestTimezoneFallbackChain(t *testing.T) {
	lib := time.FixedZone("lib", 8*3600)
	wall := "2019:08:12 14:03:22"

	offset := ResolveCaptured([]KV{
		{Key: "exif:DateTimeOriginal", Value: wall, Type: TypeText},
		{Key: "exif:OffsetTimeOriginal", Value: "-08:00", Type: TypeText},
	}, lib)
	if offset.ZoneFrom != "exif:OffsetTimeOriginal" {
		t.Errorf("ZoneFrom = %q, want the offset tag", offset.ZoneFrom)
	}
	if want := time.Date(2019, 8, 12, 14, 3, 22, 0, time.FixedZone("", -8*3600)).Unix(); offset.Unix != want {
		t.Errorf("Unix = %d, want %d", offset.Unix, want)
	}

	geo := ResolveCaptured([]KV{
		{Key: "exif:DateTimeOriginal", Value: wall, Type: TypeText},
		{Key: "geo:timezone", Value: "America/Los_Angeles", Type: TypeText},
	}, lib)
	if geo.ZoneFrom != "geo:timezone" {
		t.Errorf("ZoneFrom = %q, want geo:timezone", geo.ZoneFrom)
	}

	// 都没有 → 库时区，而不是 UTC。
	fallback := ResolveCaptured([]KV{
		{Key: "exif:DateTimeOriginal", Value: wall, Type: TypeText},
	}, lib)
	if fallback.ZoneFrom != "library" {
		t.Errorf("ZoneFrom = %q, want library", fallback.ZoneFrom)
	}
	if want := time.Date(2019, 8, 12, 14, 3, 22, 0, lib).Unix(); fallback.Unix != want {
		t.Errorf("Unix = %d, want %d — the fallback must be the library zone, not UTC", fallback.Unix, want)
	}
}

// 兜底假设的是库时区而不是 UTC，这条要单独钉住 —— Immich 把 fallback 从
// 「服务器本地」改成 UTC 之后收到了回归报告（immich#24107）：个人库里
// 绝大多数照片拍在自己家的时区，假设 UTC 会把它们整体推移几小时，混在
// 有 GPS 的照片里就是乱序。
func TestTheFallbackZoneIsNotUTC(t *testing.T) {
	lib := time.FixedZone("lib", 8*3600)
	got := ResolveCaptured([]KV{
		{Key: "exif:DateTimeOriginal", Value: "2019:08:12 14:03:22", Type: TypeText},
	}, lib)
	utc := time.Date(2019, 8, 12, 14, 3, 22, 0, time.UTC).Unix()
	if got.Unix == utc {
		t.Error("the unknown-timezone fallback resolved to UTC")
	}
}

// 什么都没有不是错误，是 captured_at 为空。spec 4.5.1 已经定了这类文件
// 排最后。
func TestNoTimeAtAll(t *testing.T) {
	got := ResolveCaptured([]KV{{Key: "exif:Make", Value: "Apple", Type: TypeText}}, time.UTC)
	if got.Known {
		t.Errorf("resolved a capture time out of nothing: %+v", got)
	}
}

// 同一组输入必须给同一个结果，而且不读时钟、不读环境 —— 时区是参数。
// 否则同一个库在两台机器上导入会得到不同的 captured_at，而这一列决定
// 用户看见的排序。
func TestResolutionDoesNotReadTheEnvironment(t *testing.T) {
	tags := []KV{{Key: "exif:DateTimeOriginal", Value: "2019:08:12 14:03:22", Type: TypeText}}
	a := ResolveCaptured(tags, time.FixedZone("a", 3600))
	b := ResolveCaptured(tags, time.FixedZone("b", 7200))
	if a.Unix == b.Unix {
		t.Error("the library zone had no effect; the resolution is reading something else")
	}
}

// 坏掉的 EXIF 时间不该让整个解析失败，应该退到下一档。相机写出 0000:00:00
// 是真实存在的情况。
func TestAnUnparsableExifTimeFallsThrough(t *testing.T) {
	got := ResolveCaptured([]KV{
		{Key: "exif:DateTimeOriginal", Value: "0000:00:00 00:00:00", Type: TypeText},
		{Key: "file:mtime", Value: "1700000000", Type: TypeInt},
	}, time.UTC)
	if got.WallFrom != "file:mtime" {
		t.Errorf("WallFrom = %q; a garbage EXIF time must fall through to mtime", got.WallFrom)
	}
}
```

- [ ] **Step 2: 跑，确认全红**

- [ ] **Step 3: 实现**

要点：

- EXIF 的时间格式是 `2006:01:02 15:04:05`（冒号分隔日期），不是 ISO
- `0000:00:00 00:00:00` 和任何解析不出来的值**退到下一档**，不报错
- offset 标签形如 `-08:00`，用 `time.Parse` 组合而不是手算
- `geo:timezone` 是 IANA 名字，`time.LoadLocation`。**加载失败退到库时区**——Go 的 tzdata 在某些精简系统上可能缺失，而这不该让一个文件失去拍摄时间。若要彻底消除这个变数，`cmd` 侧 `import _ "time/tzdata"`（约 +450 KB，Task 7 决定）
- **秒是精度终点**，亚秒（`exif:SubSecTimeOriginal`）不参与——1.5.2 已经论证过连拍顺序由 tiebreaker 决定

- [ ] **Step 4: 跑测试，全绿**

- [ ] **Step 5: 验证守护会红**

把兜底时区改成 `time.UTC` → `TestTheFallbackZoneIsNotUTC` 和 `TestTimezoneFallbackChain` 必须红。

把 `AnUnparsableExifTime` 那一档改成返回错误 → 对应测试必须红。

- [ ] **Step 6: `make check` + commit**

---

### Task 5: `oplog` 的 `extract_metadata`

**Files:**
- Create: `internal/oplog/metadata.go`
- Modify: `internal/oplog/action.go`（加 `ActionExtractMetadata`）
- Modify: `internal/oplog/tx.go`（加四个方法）
- Test: `internal/oplog/metadata_test.go`, `internal/oplog/faketx_test.go`

**Interfaces:**
- Produces:
  ```go
  const ActionExtractMetadata Action = "extract_metadata"

  type ExtractedFile struct {
      FileID     domain.FileID `json:"file_id"`
      Set        []extract.KV  `json:"set,omitempty"`      // 新值
      Was        []extract.KV  `json:"was,omitempty"`      // 旧值，供 Inverse
      Removed    []string      `json:"removed,omitempty"`  // 这次不再产出的键
      CapturedAt *int64        `json:"captured_at,omitempty"`
      CapturedWas *int64       `json:"captured_was,omitempty"`
  }
  type ExtractMetadataPayload struct {
      Extractor string          `json:"extractor"`
      Version   int             `json:"version"`
      Files     []ExtractedFile `json:"files"`
  }
  ```
- Tx 新增：`SetFileMetadata(file domain.FileID, kv extract.KV, source string) error`、`DeleteFileMetadata(file domain.FileID, key string) error`、`SetFileExtraction(file domain.FileID, extractor string, version int) error`、`SetCapturedAt(file domain.FileID, unix *int64) error`

**`oplog` 会因此 import `internal/extract`。** 这是允许的——`extract` 只依赖 `domain`，所以依赖方向没有反转。arch 测试要显式允许它，并写清楚理由。

- [ ] **Step 1: 写会红的测试**

```go
// spec 2.3：Inverse 要能把状态换回去，所以 before 值必须在 payload 里。
// 而 spec 2.5 的严格模式还要拿它做断言。
func TestExtractMetadataInverseRestoresTheOldValues(t *testing.T) {
	tx := newFakeTx()
	file := domain.NewFileID()
	tx.files[file] = domain.File{ID: file}

	forward := mustMarshal(t, ExtractMetadataPayload{
		Extractor: "exif", Version: 2,
		Files: []ExtractedFile{{
			FileID: file,
			Set:    []extract.KV{{Key: "exif:Make", Value: "Apple", Type: "text"}},
			Was:    []extract.KV{{Key: "exif:Make", Value: "APPLE", Type: "text"}},
		}},
	})
	if err := Lookup2(t, ActionExtractMetadata).Apply(tx, forward); err != nil {
		t.Fatal(err)
	}
	if got := tx.metadata[metaKey{file, "exif:Make"}]; got.Value != "Apple" {
		t.Fatalf("forward apply wrote %q", got.Value)
	}

	action, inverse, err := Lookup2(t, ActionExtractMetadata).Inverse(forward)
	if err != nil {
		t.Fatal(err)
	}
	if err := Lookup2(t, action).Apply(tx, inverse); err != nil {
		t.Fatal(err)
	}
	if got := tx.metadata[metaKey{file, "exif:Make"}]; got.Value != "APPLE" {
		t.Errorf("after undo the value is %q, want the old one back", got.Value)
	}
}

// 一个键第一次出现时 Was 是空的，撤销必须把它删掉而不是留个空串。
func TestUndoingAFirstExtractionRemovesTheKey(t *testing.T) { … }

// 撤销也要把 captured_at 换回去 —— 它和 metadata 行在同一条 op 里，
// 因为它是从这些行算出来的（spec 1.5.2）。
func TestInverseAlsoRestoresCapturedAt(t *testing.T) { … }

// 空批不写 op（和 tag_files 同规则）。一条什么都不改的 op 是一次什么都
// 不做的撤销，白白弄脏历史。
func TestExtractMetadataRejectsAnEmptyBatch(t *testing.T) { … }

// 不变量 3：Apply 不生成 ID、不读时钟、不做依赖当前状态的判断。类型标签
// 由 payload 带来，Apply 只按它路由 —— 所以同一条 payload 重放两次结果
// 必须一致。
func TestApplyIsIdempotentOverTheSamePayload(t *testing.T) { … }

// 不变量 4：Targets 要报出这条 op 碰过的文件，否则 op_files 索引不到它，
// 「这个文件都经历过什么」就少一段。
func TestTargetsListsEveryFile(t *testing.T) { … }

// 类型标签是封闭集合，Apply 按它路由。payload 里出现一个不认识的标签
// 必须报错而不是静默当成 text —— 那会让新旧版本重放出不同的库。
func TestApplyRejectsAnUnknownType(t *testing.T) { … }
```

- [ ] **Step 2: 跑，确认全红**

- [ ] **Step 3: 扩 `oplog.Tx` 和 `faketx_test.go`**

- [ ] **Step 4: 实现 `metadata.go`**

`Apply`：按 `Set` 写、按 `Removed` 删、写 `SetFileExtraction(file, p.Extractor, p.Version)`、按 `CapturedAt` 写列。**`source` 取 `p.Extractor`**，不是常量。

`Inverse`：同一个 action，`Set` 与 `Was` 对调，`Removed` 与「`Was` 里有而 `Set` 里没有的键」对调，`CapturedAt` 与 `CapturedWas` 对调。**`Version` 也要换回去**——否则撤销之后 `file_extractions` 会说这批文件已经被新版本处理过，重提取就不会再来。

> 这一条是本 task 最容易漏的地方，单独写一个测试钉住它。

`Targets`：`Files` 里每个 `FileID`。**boxes 为空**——metadata 不改变 box 的任何东西，而 1.5.1 ② 说提取器源的 op 本来就不传播 `updated_at`（`SourceExtractor` 在 M1b 就定好了）。

- [ ] **Step 5: 跑测试，全绿**

- [ ] **Step 6: 验证守护会红**

- 把 `Inverse` 里的 `Version` 换回去那一步删掉 → 对应测试必须红
- 把 `Apply` 的 `source` 从 `p.Extractor` 改成常量 `"extracted"` → 加一条断言 `source` 的测试并验证它红
- 往 `Apply` 里塞一个「跑了几次」的计数器 → `internal/library` 的 rebuild 快照测试必须红（Task 8 之后）

- [ ] **Step 7: `make check` + commit**

---

### Task 6: `store` 的读写，以及时间筛选改成瞬时

**Files:**
- Create: `internal/store/metadata.go`
- Modify: `internal/store/views.go`（`ViewTx` 实现新方法）
- Modify: `internal/store/translate.go`（`timePredicate`）
- Modify: `internal/store/files.go`（`captured_at` 的扫描类型）
- Modify: `internal/query/cursor.go`（`SortCaptured` 的游标值）
- Test: `internal/store/metadata_test.go`, `internal/store/translate_test.go`, `internal/store/conformance_test.go`

**Interfaces:**
- Produces：`store.FileMetadata(q Queryer, file domain.FileID) ([]extract.KV, string, error)`、`store.Extractions(q Queryer, file domain.FileID) (map[string]int, error)`、`store.FilesNeedingExtraction(q Queryer, extractor string, version int, limit int) ([]domain.FileID, error)`

- [ ] **Step 1: 写会红的测试**

```go
// spec 1.7.6 ③：这就是 file_extractions 存在的理由。没有它，「哪些文件
// 还没被 exif v2 处理过」只能全表扫 payload —— 而 payload 不可查询。
func TestFilesNeedingExtractionFindsNewAndStale(t *testing.T) {
	// 三个文件：没提过的、v1 提过的、v2 提过的。查 v2 应当只返回前两个。
}

// spec 1.5.2 / 4.2.1：文件侧的时间筛选变成瞬时的 BETWEEN，而变精度语义
// 原样保留 —— --since 2019 是那一年的头到尾。
func TestTimePredicateTurnsALocalDateIntoInstants(t *testing.T) {
	zone := time.FixedZone("test", 8*3600)
	sql, args := timePredicate("captured_at", query.DateRange{
		From: mustDate(t, "2019"),
	}, zone)
	if strings.Contains(sql, "substr(") {
		t.Errorf("the file-side predicate still does string overlap: %s", sql)
	}
	want := time.Date(2019, 1, 1, 0, 0, 0, 0, zone).Unix()
	if args[0] != want {
		t.Errorf("lower bound = %v, want %d", args[0], want)
	}
}

// box 侧不动 —— 它的日期是用户声明的模糊区间，重叠匹配是对的（spec 1.3）。
func TestBoxTimePredicateStillOverlaps(t *testing.T) { … }

// spec 4.5.1：captured_at 为空的文件排最后，两个方向都要显式写。M2b 已经
// 证明过 NULLS LAST 在升序上是能测出来的，列类型改了之后要重新证一遍。
func TestNullCapturedAtSortsLastInBothDirections(t *testing.T) { … }
```

- [ ] **Step 2: 跑，确认全红**

- [ ] **Step 3: 实现 `metadata.go` 与 `ViewTx` 的新方法**

- [ ] **Step 4: 改 `timePredicate`**

它现在是：

```go
AND col IS NOT NULL AND col <= ? AND col >= substr(?, 1, length(col))
```

文件侧变成：

```go
AND col IS NOT NULL AND col >= ? AND col <= ?
```

参数由 `LocalDate.Start()` / `End()` 在**库时区**里转成 unix 秒。

**库时区从哪来**：`translate` 是纯函数，所以时区是参数，由 `library` 从配置传进来（和 `ResolveCaptured` 同形）。**不要在 `store` 里读 `time.Local`**——那会让同一个查询在不同机器上返回不同结果，而 4.7 说的正是这件事。

- [ ] **Step 5: 改 conformance suite 的时间那一组**

`internal/query/querytest/cases.go` 里，文件时间的 case 从「变精度重叠」改成「瞬时区间」。**box 那一组不动。**

fixture 也要改：`Builder` 现在拿 `LocalDate` 塞 `captured_at`，改成塞 unix 秒。

**新增至少两个 case**：

- 一张 `captured_at` 为负数的文件（1960 年的扫描件），验证它排在 1970 年之后的文件前面。**这是 TEXT 列时会错的那个案例**，用它把这次改动钉死
- `--since 2019` 匹配的是那一年的第一秒到最后一秒，边界包含

- [ ] **Step 6: 跑测试，全绿**

- [ ] **Step 7: 验证守护会红**

- 把 `captured_at` 列改回 TEXT 并把负数写进去 → 那条 1960 年的 conformance case 必须红
- 把库时区参数换成 `time.Local` → `TestTimePredicate…` 在非目标时区的机器上会红。**如果本地时区恰好一样测不出来，就在测试里显式用两个不同的时区跑两遍**，像 `TestResolutionDoesNotReadTheEnvironment` 那样

- [ ] **Step 8: `make check` + commit**

---

### Task 7: `library.Extract` 与 CLI

**Files:**
- Create: `internal/library/extract.go`
- Create: `cmd/kist/extract.go`
- Modify: `cmd/kist/file.go`（`kist file show`）
- Modify: `internal/config/config.go`（库时区）
- Test: `internal/library/extract_test.go`, `cmd/kist/extract_test.go`

**Interfaces:**
- Produces:
  ```go
  type ExtractOptions struct {
      Box       domain.BoxID   // 零值表示全库
      Extractor string         // 空表示全部
      Force     bool           // 忽略版本，全部重提
      BatchSize int            // 默认 1000
  }
  type ExtractResult struct {
      OpIDs      []int64
      Files      []FileResult   // 复用导入那套 per-file 结果
      Interrupted bool
      Remaining  int
  }
  func (l *Library) Extract(ctx context.Context, opt ExtractOptions, progress func(done, total int)) (*ExtractResult, error)
  ```

- [ ] **Step 1: 库时区进配置**

```toml
[library]
id = "…"
timezone = "Asia/Shanghai"   # 空表示用机器的本地时区，并在 init 时写实
```

**`kist init` 把探测到的时区写实**，而不是留空。理由和 3.5 的 link mode 一样：留空意味着同一个库在另一台机器上打开会改变行为，而这一列决定用户看见的排序。`init` 时展示它，让用户能改。

- [ ] **Step 2: 写会红的测试**

```go
// spec 1.7.6 ④ + 原则 ②：同一个提取器同一个版本重跑，一条 op 都不写。
func TestReExtractingWithTheSameVersionWritesNoOp(t *testing.T) { … }

// 版本变了就重提，而且 payload 里只有真的变了的键。
func TestABumpedVersionRewritesOnlyWhatChanged(t *testing.T) { … }

// spec 3.7.1 ①：第一次 Ctrl-C 停下并提交已完成的批次。提取是分批的，所以
// 「已完成的」是批而不是全部。
func TestExtractInterruptedKeepsFinishedBatches(t *testing.T) { … }

// spec 3.7.1 ③：一个文件解析失败不拖累整批，而且它必须能被重试 ——
// 所以失败的文件不写 file_extractions。
func TestAFailedFileIsNotRecordedAsExtracted(t *testing.T) { … }

// spec 1.5.2：captured_at 在同一条 op 里更新，而且是内核算的。
func TestExtractSetsCapturedAt(t *testing.T) { … }

// 装上一个新提取器之后，captured_at 要跟着重算 —— 这是 1.5.2 说的
// 「输入变了就要重算」，而重算发生在生成 op 的时候。
func TestANewExtractorTriggersACapturedAtRecompute(t *testing.T) {
	// 先只跑 exif（无 offset 标签，落到库时区），再注册一个产出
	// geo:timezone 的假提取器，captured_at 必须变。
}

// 用户手填的行永远不被覆盖（spec 1.7 硬规则 ②）。v0.1 没有写入 UI，所以
// 这条要用直接插一行 source='user' 的方式来测 —— 规则现在就要立住，
// 因为 v0.2 一旦开始写就来不及了。
func TestExtractionNeverOverwritesAUserRow(t *testing.T) { … }
```

- [ ] **Step 3: 跑，确认全红**

- [ ] **Step 4: 实现 `library.Extract`**

流程：

1. `store.FilesNeedingExtraction(extractor, version, batch)` 取一批
2. 每个文件：从 `blobs/` 打开内容 → `Wants(mime)` → `Extract` → `extract.Check`
3. 和现有行读出来的比，算出 `Set` / `Was` / `Removed`。**`source == "user"` 的行整个跳过**
4. `ResolveCaptured(所有行的合并结果, 库时区)` → `CapturedAt` / `CapturedWas`
5. 一批一条 op，走 `l.write(…, oplog.SourceExtractor, nil)`
6. 下一批

**分批而不是一条大 op**：3.7.1 为导入否掉「整批回滚」的理由原样适用——几小时的长事务加上「跑到 99% 被打断就全部白做」。

**`captured_at` 的重算要覆盖那些「这次没提到但受影响」的文件吗？** 不。重算只发生在这条 op 碰到的文件上，因为解析规则的输入只有那个文件自己的行。`kist:captured` 的版本变了时，用同一个 `FilesNeedingExtraction` 机制走一遍即可——**规则版本和提取器版本共用一张表，这正是 1.7.6 ③ 那个设计的收益。**

- [ ] **Step 5: 实现 CLI**

```
kist extract [--box 云南] [--extractor exif] [--force]
kist file show <id>        # 列出 metadata，标注 source
```

`kist extract` 复用 `interruptContext`（M1c 提出来的）和导入那套 per-file 汇总（`imported / skipped / failed`，这里是 `extracted / skipped / failed`）。

`kist file show` 要**显式标注时区是猜的**：

```
拍摄时间  2019-08-12 14:03:22  (时区来自库配置，文件里没有)
```

1.5.2 说了这是必须做对的一条——用户要能看出「这是我缺信息」而不是「这个时间是准的」。

- [ ] **Step 6: 跑测试，全绿**

- [ ] **Step 7: 验证守护会红 + 手工走一遍**

手工：建库 → 导入几张真照片（含 iPhone 的 HEIC）→ `kist extract` → `kist find --sort captured` → `kist file show` → `kist undo` → 确认 metadata 和 `captured_at` 都回去了 → `kist extract` 再跑一遍 → 确认它把撤销掉的又提了回来（因为 `Inverse` 把 `file_extractions` 的版本也换回去了）。

**最后那一步是 Task 5 那个「最容易漏的地方」的端到端验证。**

- [ ] **Step 8: `make check` + commit**

---

### Task 8: 收尾——不变量 3 要覆盖到 metadata

**Files:**
- Modify: `internal/library/rebuild_test.go`
- Modify: `README.md`, `README.zh-CN.md`, `AGENTS.md`
- Modify: `docs/plans/2026-08-28-m3-extraction.md`（执行记录）

- [ ] **Step 1: 把 metadata 加进 rebuild 的快照证明**

现有的证明是：建一批数据 → 记全库快照 → `Rebuild` → 快照必须逐字节相同。**fixture 要加上提取**：

- 一次首次提取（`Was` 为空）
- 一次版本升级后的重提取（`Set` 和 `Was` 都非空）
- 一次被撤销的提取
- 一个 `source='user'` 的行，验证重放不会碰它
- 一个 `captured_at` 为负数的文件

- [ ] **Step 2: 验证它会红**

往 `extract_metadata` 的 `Apply` 里塞一个「跑了几次」的计数器，快照必须指出是 `file_metadata` 或 `file_extractions` 哪一行不一样。

**M1c 的记录里写过：这个证明验证过它会红，指出的是 `files` 表哪一行的 name 不一样。** 这次要确认它同样指得出新表。

- [ ] **Step 3: 更新文档**

- `AGENTS.md` 的「当前状态」加 M3
- 两个 README 的功能列表加 `kist extract`
- **设计文档 5.1 的技术选型表加一行 `bep/imagemeta`**，并写明否掉 `dsoprea/go-exif`（HEIC 要再配一个包）和 `rwcarlsen/goexif`（停止维护、不支持 HEIC）的理由
- **设计文档 5.3 要更新一个乐观的发现**：EXIF 能从 HEIC 里纯 Go 读出来，ffmpeg 只挡像素不挡元数据。这比 5.3 原来暗示的好

- [ ] **Step 4: 写执行记录**

按 M2a / M2b 的惯例，在本文件末尾记下**哪些地方实测推翻了计划**。如果一处都没有，那八成是没认真验证守护会红。

- [ ] **Step 5: `make check` + commit + push，等 CI 三平台绿**

---

## 自查

**Spec 覆盖：**

| spec | task |
|---|---|
| 1.5.2 `captured_at` 是瞬时、解析规则、时区链 | 1, 4, 6 |
| 1.7.1 权威是 op log | 5, 8 |
| 1.7.2 展开按形状 | 3 |
| 1.7.3 只挂在 file 上、真外键 | 1 |
| 1.7.4 v0.1 不建非主键索引 | 1 |
| 1.7.5 值的类型、payload 带 Type | 2, 3, 5 |
| 1.7.6 提取器契约、命名空间、版本、`file_extractions` | 2, 3, 5, 6 |
| 2.3 / 2.5 Inverse 与严格模式 | 5 |
| 3.7.1 中断与部分失败 | 7 |
| 4.2.1 / 4.5.1 时间筛选与 NULL 排序 | 6 |
| 4.7 引擎分歧（时区不从环境读） | 6 |

**没有覆盖到的（有意）：** 外部进程插件（1.7.6 的后半段）、`value_int` / `value_real` 旁列、GPS 反查时区。三条都在第 8 节有触发条件。

**类型一致性：** `extract.KV` 在 `extract` / `exif` / `oplog` / `store` / `library` 五个包里流转，字段名和 JSON tag 必须一致。`oplog` import `extract` 是允许的（`extract` 只依赖 `domain`），arch 测试要显式写明。

**最容易做错的三处：**

1. **`Inverse` 忘了把 `Version` 换回去**（Task 5）——撤销之后重提取不会再来，而且不报错
2. **`store` 里读了 `time.Local`**（Task 6）——同一个查询在不同机器上返回不同结果
3. **失败的文件被写进了 `file_extractions`**（Task 7）——重试永远不会再碰它

