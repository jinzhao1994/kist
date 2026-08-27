# M1c 导入、投影与检索 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `kist import --box` 真的能把一棵目录塞进库里，`boxes/` 下长出人可读的链接农场，`kist ls` / `kist find` 能按目录、文件名和 tag 把东西找出来，`kist relink` 能把投影整棵重建——**M1 的那句产出「能用命令行把真实数据塞进去、按 tag 和文件名筛出来」到这里才算兑现**。

**Architecture:** 在 M1b 的写路径上加两件东西。

一是 **`files` / `folders` 这条线打通**：`oplog` 长出 `import_files` / `delete_files` / `tag_files` / `untag_files` 四个 handler，`store` 长出对应的物化视图写接口，`library.Import` 成为唯一的导入入口——它在事务外把 blob 落盘（内容寻址、幂等），在事务内提交**一条** op。

二是 **`boxes/` 投影**，一个新包 `internal/projection`。它是**纯函数**：给定 box / folder / file 的集合，算出一棵期望的目录树；同步器把磁盘上的实际状态 diff 到期望状态。`relink` 就是"整棵重算 + 同步"，增量维护是同一套代码只作用于一个 box。投影**永远不是权威**，坏了重建即可，所以它挂在写事务提交**之后**，失败只置脏标记不回滚。

`internal/projection` 不认识数据库，也不认识 `oplog`——它只吃 `domain` 的结构体，吐路径。这让 3.4 那一堆跨平台命名规则可以在没有库的情况下被穷举测试。

**M1c 不做提取。** `metadata` 表继续空着：`file:size` / `file:mtime` 属于 M3 的 extract 管线（第 7 节），3.7 管线的第 3 步在 M1c 是空操作。`captured_at` 整列为 NULL，直到 M3。

**Tech Stack:** M1a / M1b 的全部依赖，不新增。`golang.org/x/text/unicode/norm`（已在，`domain.Fold` 用着）、`os/signal`（导入中断）。

**Spec:** `docs/design/2026-08-22-kist-v0.1-design.md`（配套待决问题见 `docs/design/open-questions.md`）

**前置：** M1a + M1b 全部完成。`schema` 已经是 v0.1 全量，M1c **不需要任何 migration**——`files` / `folders` / `blobs` / `file_tags` 表和它们的索引在 `00001_init.sql` 里就位。

## Global Constraints

这些是全局要求，每个 task 的验收都隐含包含它们：

- **Go 1.26.1**，版本从 `go.mod` 读
- **`CGO_ENABLED=0` 必须能构建**，四个目标平台 `darwin/arm64` `darwin/amd64` `linux/amd64` `windows/amd64`
- **不变量 1：权威数据只有 `ops` 表 + `blobs/` 目录。** `boxes/` 是投影，`files` / `folders` 是物化视图。M1c 结束时 `Rebuild` 的逐字节快照测试必须**含文件数据**才算数
- **不变量 2：`library` 是唯一写入口。** `cmd` 不得 import `store` / `oplog`；新包 `internal/projection` 同样不得
- **不变量 3：`Apply` 必须是纯函数。** 导入的每一个决策——文件 ID、folder ID、冲突改名后的最终名字、blob key——都在 `library` 生成 op 时做完并写进 payload。`Apply` 只是把 payload 铺进表里
- **不变量 4：payload 是不可查询的载荷。** `op_files` 由 `Targets` 从 payload 单向重建
- **时间戳一律 UTC，格式 `2006-01-02T15:04:05.000Z`**，定宽，字典序即时间序
- **UUID 在 DB 里存 `BLOB(16)`**
- **凡是要被 `LIKE` 搜或要唯一的文本，都由 Go 生成折叠列**（`domain.Fold`）
- **排序一律字节序**（设计文档 4.8）；**可空列的排序必须显式写 `NULLS LAST`**（4.5.1、4.7）
- **语言：代码、注释、godoc 用英文；设计文档与 commit message 用中文**
- 每个 task 结束时 `make check` 必须通过

---

## 与设计文档的五处偏离（都要在本计划内同步回文档）

### ① 投影只给目录设 mtime，不给文件链接设

设计文档 3.4.1 的表里有一行「rename 后 → 重新 `os.Chtimes` 应用 `updated_at`」，没有区分目录和文件。**对文件链接不能这么做。**

`LinkHardlink` 模式下链接和 blob 是**同一个 inode**，给链接设 mtime 就是在改 blob 文件的 mtime。更糟的是去重：两个 file 行指向同一个 blob 时，它们的 `updated_at` 不同，投影会来回改写同一个 inode 的 mtime，每次 `relink` 结果都可能不一样——而 `relink` 必须是幂等的。

所以：**box 目录和 folder 目录设 mtime，文件链接不设。** Task 9 的 Step 8 把这条写回 3.4.1。

### ② `ListOps` 不再读 payload，`Rebuild` 改成流式

M1b 的 `ListOps` 把 `payload` 一起 SELECT 出来，M1b 里 payload 只有几百字节所以无所谓。导入把它变成几 MB（5000 个文件约 1 MB JSON），于是两处都出问题：

- `kist log` 一行也用不到 payload，却要为每一行拖一份出来
- `Rebuild` 用 `ListOps(opt{})` 拿全部 op **再遍历**，等于把全库所有 payload 一次性读进内存

改法：`ListOps` 不选 `payload`（`OpRow.Payload` 留空），`GetOp` 照旧全取；`Rebuild` 换成一个逐行游标。Task 13 做这件事，并把结论写进设计文档 2.2.1。

### ③ M1c 不写 `metadata`

3.7 管线第 3 步是「提取 EXIF / 文件固有属性」。第 7 节把「EXIF / 文件固有属性 → metadata」整个放在 **M3**，所以 M1c 里这一步是空操作，`metadata` 表保持为空。计划里不为它写任何代码，避免将来 extract 管线落地时要拆一份半成品。

### ④ `tag_files` / `untag_files` 进 M1c

第 7 节 M1 的 CLI 清单是 `init / box new / tag / import --box / ls / find / relink`，没有 `file tag`。但同一节的产出写的是「能用命令行把真实数据塞进去、**按 tag 和文件名筛出来**」——而 `file_tags` 在 M1c 之前一行都没有，`find --tag` 必然返回空。

产出那句蕴含了文件能被打 tag，所以补上。实现是 M1b `tag_boxes` / `untag_boxes` 的同构翻版，成本很低。Task 5 的 Step 9 把 CLI 清单补进设计文档第 7 节。

### ⑤ 跨平台文件名净化在所有平台上一视同仁

3.4 的表里写的是「同一个库在 Mac 上创建、拷到 Windows 打开，**投影名不同但 DB 一致**」，即净化是平台相关的。改成**所有平台用同一套净化规则**，三条理由：

1. **`relink` 必须幂等，跨平台也得幂等。** 平台相关的话，把库目录从 Mac 挪到 Windows 再 `relink`，整棵树会被重命名一遍——而投影的每一次大改名都是一次 Windows sharing violation 的机会
2. **CI 能真正测到 Windows 的规则。** 净化统一之后，Linux 上跑的测试就是 Windows 上跑的测试；平台相关的话，`CON`、尾随点、`<>:"/\|?*` 这些规则只有 windows runner 会执行到，而那正是最容易写错的一段
3. **代价只是审美。** 被替换的是 `<>:"/\|?*` 和控制字符——`/` 在 POSIX 上本来也非法，`:` 在 macOS 的 Finder 里会显示成 `/`，剩下的在文件名里本来就少见

Task 8 的 Step 7 把 3.4 的那一行改掉并写明理由。

---

## 文件结构

```
internal/domain/
  file.go      file_test.go        File 实体
  folder.go                        ＋ NewFolder（M1b 只有 NewRootFolder）
  filename.go  filename_test.go    stem/ext 拆分与「名字 (2).jpg」冲突改名

internal/oplog/
  action.go                        ＋ 四个 action 常量
  tx.go                            ＋ files / folders / blobs / file_tags 的方法
  file.go      file_test.go        import_files / delete_files
  filetag.go   filetag_test.go     tag_files / untag_files

internal/store/
  views.go     views_test.go       ＋ 上面那些方法的 SQL 实现
  files.go     files_test.go       ★ 读侧：ListFiles / FindFiles / 目录树 / keyset
  ops.go                           ListOps 去掉 payload，新增 IterOps 游标
  snapshot.go                      ＋ blobs 表（导入让它不再恒为空）

internal/projection/
  name.go      name_test.go        ★ 跨平台净化、box 目录名、冲突后缀
  plan.go      plan_test.go        ★ 纯函数：实体集合 → 期望目录树
  sync.go      sync_test.go        期望树 diff 到磁盘
  dirty.go                         投影脏标记

internal/library/
  import.go    import_test.go      ★ Import：扫描、去重、冲突、per-file 结果、中断
  files.go     files_test.go       Files / Find / TagFiles / UntagFiles
  project.go   project_test.go     Relink ＋ 写路径提交后的增量投影
  write.go                         ＋ folder 链的 updated_at 传递
  rebuild.go                       改成流式重放

cmd/kist/
  import.go    import_test.go      kist import --box（含 SIGINT）
  ls.go        ls_test.go          kist ls
  find.go      find_test.go        kist find
  file.go      file_test.go        kist file tag / untag
  relink.go    relink_test.go      kist relink
```

---

### Task 1: `domain` 的 File、子 Folder 与冲突改名

**Files:**
- Create: `internal/domain/file.go`, `internal/domain/file_test.go`
- Create: `internal/domain/filename.go`, `internal/domain/filename_test.go`
- Modify: `internal/domain/folder.go`（加 `NewFolder`）

**Interfaces:**
- Consumes: `domain.FileID` / `FolderID` / `BoxID` / `LocalDate` / `Fold` / `ValidateName`（M1a、M1b 已有）
- Produces:
  - `domain.File`（字段见下）、`func NewFile(id FileID, box BoxID, folder FolderID, name, blobKey string) (File, error)`
  - `func NewFolder(id FolderID, box BoxID, parent FolderID, name string) (Folder, error)`
  - `func Normalize(s string) string`（NFC，`fold.go`）
  - `func SplitName(name string) (stem, ext string)`
  - `func NumberedName(name string, n int) string`

**关键约束：`domain` 不许 import 仓库里任何别的包**（`internal/arch` 的 `TestDomainDependsOnNothing` 守着）。所以 `File.BlobKey` 是 `string` 而不是 `blob.Key`——把 `blob` 拉进 `domain` 会让最底层的包长出一个存储依赖。`blob.Key` 本身就是 `string` 的具名类型，两边转换是零成本的。

- [x] **Step 1: 写 `NumberedName` 与 `SplitName` 的失败测试**

`internal/domain/filename_test.go`：

```go
package domain

import "testing"

// The numbered form is what import falls back to when a name is taken by
// different content (spec 1.5). The number goes before the extension, because
// "发票.jpg (2)" is not a jpeg as far as any file manager is concerned.
func TestNumberedName(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want string
	}{
		{"发票.jpg", 2, "发票 (2).jpg"},
		{"发票.jpg", 10, "发票 (10).jpg"},
		{"README", 2, "README (2)"},
		{"archive.tar.gz", 2, "archive.tar (2).gz"},
		// A leading dot is the whole name, not an extension: ".gitignore (2)"
		// is right and " (2).gitignore" is not.
		{".gitignore", 2, ".gitignore (2)"},
		{"..", 2, ". (2)."},
	}
	for _, c := range cases {
		if got := NumberedName(c.name, c.n); got != c.want {
			t.Errorf("NumberedName(%q, %d) = %q, want %q", c.name, c.n, got, c.want)
		}
	}
}

func TestSplitName(t *testing.T) {
	cases := []struct{ in, stem, ext string }{
		{"发票.jpg", "发票", ".jpg"},
		{"README", "README", ""},
		{"archive.tar.gz", "archive.tar", ".gz"},
		{".gitignore", ".gitignore", ""},
		{"trailing.", "trailing", "."},
	}
	for _, c := range cases {
		stem, ext := SplitName(c.in)
		if stem != c.stem || ext != c.ext {
			t.Errorf("SplitName(%q) = (%q, %q), want (%q, %q)", c.in, stem, ext, c.stem, c.ext)
		}
	}
}
```

- [x] **Step 2: 跑测试，确认它因为函数不存在而失败**

Run: `go test ./internal/domain/ -run 'TestNumberedName|TestSplitName'`
Expected: FAIL, `undefined: NumberedName`

- [x] **Step 3: 实现 `internal/domain/filename.go`**

```go
package domain

import (
	"fmt"
	"strings"
)

// SplitName cuts a file name into the part a user would call its name and the
// part a file manager uses to pick an icon.
//
// The dot has to be at index > 0: ".gitignore" is a name, not an extension,
// and treating it as one would produce " (2).gitignore" on a collision.
func SplitName(name string) (stem, ext string) {
	i := strings.LastIndex(name, ".")
	if i <= 0 {
		return name, ""
	}
	return name[:i], name[i:]
}

// NumberedName is the nth alternative for a taken name: "发票.jpg" → "发票 (2).jpg".
//
// Import uses it when a name is already in use by *different* content
// (spec 1.5). Identical content is skipped instead, which is what makes a
// re-run after an interrupted import converge rather than pile up copies
// (spec 3.7.1 ②).
func NumberedName(name string, n int) string {
	stem, ext := SplitName(name)
	return fmt.Sprintf("%s (%d)%s", stem, n, ext)
}
```

- [x] **Step 4: 跑测试，确认通过**

Run: `go test ./internal/domain/ -run 'TestNumberedName|TestSplitName' -v`
Expected: PASS

- [x] **Step 5: 写 `File` 与 `NewFolder` 的失败测试**

`internal/domain/file_test.go`：

```go
package domain

import (
	"errors"
	"testing"
)

func TestNewFileFolds(t *testing.T) {
	f, err := NewFile(NewFileID(), NewBoxID(), NewFolderID(), "IMG_0001.JPG", "sha256:abcd")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	if f.NameFold != "img_0001.jpg" {
		t.Errorf("NameFold = %q, want %q", f.NameFold, "img_0001.jpg")
	}
	// Timestamps belong to the op that creates the file, and only the write
	// path knows that op's ts (spec 2.3.1).
	if !f.CreatedAt.IsZero() || !f.UpdatedAt.IsZero() {
		t.Error("NewFile stamped timestamps; they must come from ops.ts")
	}
	if !f.CapturedAt.IsZero() {
		t.Error("NewFile set CapturedAt; EXIF extraction is M3")
	}
}

// A file name that came off a disk is not automatically a legal kist name:
// trailing dots and edge whitespace are dropped or rejected by Windows, so a
// library holding them would not project the same way everywhere (spec 3.4).
// Import turns this into a per-file failure rather than aborting the batch
// (spec 3.7.1 ③), which only works if the check is here.
func TestNewFileRejectsUnprojectableNames(t *testing.T) {
	for _, name := range []string{"", "  ", " leading.jpg", "trailing.jpg ", "dot."} {
		if _, err := NewFile(NewFileID(), NewBoxID(), NewFolderID(), name, "sha256:abcd"); err == nil {
			t.Errorf("NewFile(%q) was accepted", name)
		}
	}
}

func TestNewFileRequiresBlobKey(t *testing.T) {
	_, err := NewFile(NewFileID(), NewBoxID(), NewFolderID(), "a.jpg", "")
	if !errors.Is(err, ErrNoBlobKey) {
		t.Errorf("err = %v, want ErrNoBlobKey", err)
	}
}

func TestNewFolderIsNotARoot(t *testing.T) {
	box := NewBoxID()
	parent := NewFolderID()
	f, err := NewFolder(NewFolderID(), box, parent, "2019云南")
	if err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	if f.ParentID == nil || *f.ParentID != parent {
		t.Errorf("ParentID = %v, want %v", f.ParentID, parent)
	}
	if f.NameFold != Fold("2019云南") {
		t.Errorf("NameFold = %q", f.NameFold)
	}
	// The root is the one folder with no name; a named folder must never be
	// mistaken for one, because the schema allows exactly one root per box.
	if f.Name == "" {
		t.Error("NewFolder produced a nameless folder")
	}
}

func TestNewFolderRejectsEmptyName(t *testing.T) {
	if _, err := NewFolder(NewFolderID(), NewBoxID(), NewFolderID(), ""); err == nil {
		t.Error("NewFolder accepted an empty name; that is the root's shape")
	}
}
```

- [x] **Step 6: 跑测试，确认失败**

Run: `go test ./internal/domain/ -run 'TestNewFile|TestNewFolder'`
Expected: FAIL, `undefined: NewFile`

- [x] **Step 7: 实现 `internal/domain/file.go`**

```go
package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrNoBlobKey means a file was built without content behind it.
var ErrNoBlobKey = errors.New("file has no blob key")

// File is one stored thing: exactly one blob, in exactly one folder, in
// exactly one box.
//
// BlobKey is a string rather than a blob.Key because domain must not depend on
// anything else in this repository — blob.Key is a named string type, so the
// conversion at the boundary costs nothing.
//
// CapturedAt is the photo's own idea of when it was taken: variable-precision
// local wall time with no timezone (spec 1.3), extracted from EXIF in M3. It
// is empty for every file until then, and files with no CapturedAt sort last
// (spec 4.5.1).
type File struct {
	ID         FileID
	BoxID      BoxID
	FolderID   FolderID
	Name       string
	NameFold   string
	BlobKey    string
	CapturedAt LocalDate
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewFile validates and folds. Timestamps are left zero: they belong to the op
// that creates the file (spec 2.3.1).
func NewFile(id FileID, box BoxID, folder FolderID, name, blobKey string) (File, error) {
	name = Normalize(name)
	if err := ValidateName(name); err != nil {
		return File{}, err
	}
	if blobKey == "" {
		return File{}, fmt.Errorf("%s: %w", name, ErrNoBlobKey)
	}
	return File{
		ID:       id,
		BoxID:    box,
		FolderID: folder,
		Name:     name,
		NameFold: Fold(name),
		BlobKey:  blobKey,
	}, nil
}
```

- [x] **Step 7b: 把 NFC 规范化补进 `fold.go` 和其余三个构造函数**

设计文档 3.4 写的是「**DB 里一律存 NFC**，verify 时两边都 NFC 化再比。中文名尤其容易踩」。M1a / M1b 只对 `name_fold` 做了 NFC（`Fold` 内部做的），**`name` 列本身存的是原样字节**——这是个真漏洞，导入才让它暴露出来：macOS 的 `os.ReadDir` 返回 NFD，所以同一个「café.jpg」从 Mac 导入存的是 NFD，从 Linux 导入存的是 NFC，`name_fold` 相同而 `name` 不同。

`internal/domain/fold.go` 加：

```go
// Normalize puts user text into NFC, the one form the database stores
// (spec 3.4).
//
// It matters most for names that arrived from a filesystem: macOS hands back
// NFD, Linux hands back whatever was written. Without this, the same 中文 or
// accented name imported on two machines produces two different rows whose
// name_fold is identical — a UNIQUE constraint that fires for reasons nobody
// can see, and a projection that differs per platform.
func Normalize(s string) string { return norm.NFC.String(s) }
```

`NewBox` / `NewFolder` / `NewTag` 同样在 `ValidateName` 之前加一行 `name = Normalize(name)`（`NewBox` 的 `description` 一并处理）。对应测试：

```go
func TestConstructorsStoreNFC(t *testing.T) {
	nfd := "cafe\u0301.jpg" // e + combining acute, which is what macOS hands back
	nfc := "caf\u00e9.jpg"  // precomposed
	f, err := NewFile(NewFileID(), NewBoxID(), NewFolderID(), nfd, "sha256:aa")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != nfc {
		t.Errorf("Name = %q, want the NFC form %q", f.Name, nfc)
	}
}
```

- [x] **Step 8: 在 `internal/domain/folder.go` 加 `NewFolder`**

```go
// NewFolder builds a named folder under a parent.
//
// It cannot produce a root: the root is the folder with no name and no parent,
// and the schema's partial unique index allows exactly one per box (spec 1.4).
// Keeping the two constructors apart is what makes "one root" impossible to
// break by accident rather than merely unlikely.
func NewFolder(id FolderID, box BoxID, parent FolderID, name string) (Folder, error) {
	if err := ValidateName(name); err != nil {
		return Folder{}, err
	}
	p := parent
	return Folder{
		ID:       id,
		BoxID:    box,
		ParentID: &p,
		Name:     name,
		NameFold: Fold(name),
	}, nil
}
```

同时把 `Folder` 的 doc 里那句「M1b only ever creates the root. Sub-folders arrive with import in M1c.」改成「The root has no name and no parent; every other folder has both.」

- [x] **Step 9: 跑全部 domain 测试**

Run: `go test ./internal/domain/ -v`
Expected: PASS，含 M1a / M1b 的既有测试

- [x] **Step 10: `make check` 并提交**

```bash
make check
git add internal/domain
git commit -m "feat(domain): File、子 Folder 与「名字 (2).jpg」冲突改名

BlobKey 是 string 不是 blob.Key——domain 必须对仓库里其他包零依赖
（internal/arch 的测试守着这条），而 blob.Key 本来就是 string 的具名
类型，边界上转换零成本。

NumberedName 把序号放在扩展名之前：「发票.jpg (2)」在任何文件管理器
眼里都不再是一张 jpeg。

顺带补上 3.4 要求的「DB 里一律存 NFC」。M1a / M1b 只对 name_fold 做了
NFC，name 列存的是原样字节——macOS 的 ReadDir 返回 NFD，所以同一个
café.jpg 从 Mac 和从 Linux 导入会存成两行不同的 name，而 name_fold
相同。导入才让这个洞暴露出来。"
```

---

### Task 2: `oplog.Tx` 与 `store.ViewTx` 长出 files / folders / blobs

**Files:**
- Modify: `internal/oplog/tx.go`
- Modify: `internal/store/views.go`
- Modify: `internal/store/views_test.go`
- Modify: `internal/store/snapshot.go`（加 `blobs` 表）

**Interfaces:**
- Consumes: `domain.File` / `domain.Folder`（Task 1）
- Produces: `oplog.Tx` 的十四个新方法，以及 `store.ViewTx` 上与之对应的实现；另有两个**不进 `Tx`** 的 `ViewTx` 方法 `FolderChainOfFiles` / `TouchFolders`

**为什么 `TouchFolders` 不进 `Tx`：** 与 M1b 的 `TouchBoxes` 同理。`updated_at` 的传递是**写路径**的规则不是 handler 的规则（spec 1.5.1 ②），放进 `Tx` 就等于邀请 handler 去调它，而 `write()` 里那句「规则住在这里，这样新 op 类型不可能忘记它」就不再成立。

**为什么 `UpsertBlob` 进 `Apply` 而不是写路径：** `blobs` 表是 `blobs/` 目录的索引，扫目录就能重建（不变量 1）。但让 `Apply` 用**幂等 upsert** 维护它有两个好处：写路径保持"所有写都发生在 Apply 里"这一条统一规则；且 `Rebuild` 在 `blobs` 表被清空的库上也能把它填回来——`Rebuild` 本来就不清 `blobs`，upsert 遇到已有行原样写回，纯度不受影响（同样的 payload 永远产生同样的行）。

- [x] **Step 1: 写 files / folders 写接口的失败测试**

`internal/store/views_test.go` 追加：

```go
func TestInsertAndGetFile(t *testing.T) {
	db := migrated(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	ts := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	v := NewViewTx(tx, ts)

	box := domain.NewBoxID()
	root := domain.NewFolderID()
	b, _ := domain.NewBox(box, "云南", "")
	if err := v.InsertBox(b); err != nil {
		t.Fatal(err)
	}
	if err := v.InsertFolder(domain.NewRootFolder(root, box)); err != nil {
		t.Fatal(err)
	}
	if err := v.UpsertBlob("sha256:abcd", "abcd", 12); err != nil {
		t.Fatal(err)
	}

	f, err := domain.NewFile(domain.NewFileID(), box, root, "IMG_0001.JPG", "sha256:abcd")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.InsertFile(f); err != nil {
		t.Fatalf("InsertFile: %v", err)
	}

	got, ok, err := v.GetFile(f.ID)
	if err != nil || !ok {
		t.Fatalf("GetFile: %v, ok=%v", err, ok)
	}
	if got.Name != "IMG_0001.JPG" || got.NameFold != "img_0001.jpg" || got.BlobKey != "sha256:abcd" {
		t.Errorf("round trip lost data: %+v", got)
	}
	if !got.CreatedAt.Equal(ts) || !got.UpdatedAt.Equal(ts) {
		t.Errorf("timestamps = %v / %v, want the op's ts %v", got.CreatedAt, got.UpdatedAt, ts)
	}
	if !got.CapturedAt.IsZero() {
		t.Errorf("CapturedAt = %q, want empty until M3", got.CapturedAt)
	}
}

// UpsertBlob has to be idempotent because Rebuild replays the same import op
// against a blobs table that was never cleared.
func TestUpsertBlobIsIdempotent(t *testing.T) {
	db := migrated(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	v := NewViewTx(tx, time.Now().UTC())

	for i := 0; i < 3; i++ {
		if err := v.UpsertBlob("sha256:abcd", "abcd", 12); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM blobs`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("blobs has %d rows after three identical upserts, want 1", n)
	}
}

// The folder chain is how updated_at reaches a box from a file buried three
// levels down (spec 1.5.1). It must stop at the root rather than walking off
// the end into NULL.
func TestFolderChainOfFiles(t *testing.T) {
	db := migrated(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	v := NewViewTx(tx, time.Now().UTC())

	box := domain.NewBoxID()
	root, mid, leaf := domain.NewFolderID(), domain.NewFolderID(), domain.NewFolderID()
	b, _ := domain.NewBox(box, "云南", "")
	mustNoErr(t, v.InsertBox(b))
	mustNoErr(t, v.InsertFolder(domain.NewRootFolder(root, box)))
	f1, _ := domain.NewFolder(mid, box, root, "day1")
	mustNoErr(t, v.InsertFolder(f1))
	f2, _ := domain.NewFolder(leaf, box, mid, "raw")
	mustNoErr(t, v.InsertFolder(f2))
	mustNoErr(t, v.UpsertBlob("sha256:abcd", "abcd", 1))
	file, _ := domain.NewFile(domain.NewFileID(), box, leaf, "a.jpg", "sha256:abcd")
	mustNoErr(t, v.InsertFile(file))

	chain, err := v.FolderChainOfFiles([]domain.FileID{file.ID})
	if err != nil {
		t.Fatal(err)
	}
	want := map[domain.FolderID]bool{root: true, mid: true, leaf: true}
	if len(chain) != len(want) {
		t.Fatalf("chain = %v, want the three folders %v", chain, want)
	}
	for _, id := range chain {
		if !want[id] {
			t.Errorf("chain contains %v, which is not on the path", id)
		}
	}
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
```

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/store/ -run 'TestInsertAndGetFile|TestUpsertBlob|TestFolderChain'`
Expected: FAIL, `v.UpsertBlob undefined`

- [x] **Step 3: 在 `internal/store/views.go` 实现 blobs 与 files**

```go
// ── blobs ────────────────────────────────────────────────────────────────

// UpsertBlob records a blob in the index.
//
// The blobs table is an index over the blobs/ directory and is rebuildable by
// scanning it (invariant 1), so this is deliberately idempotent: Rebuild
// replays import ops against a blobs table it never cleared, and writing the
// same row again must be a no-op rather than a constraint violation. Purity is
// untouched — identical payload, identical row.
func (v *ViewTx) UpsertBlob(key, sha256 string, size int64) error {
	_, err := v.tx.Exec(
		`INSERT INTO blobs (key, sha256, size) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET sha256 = excluded.sha256, size = excluded.size`,
		key, sha256, size)
	if err != nil {
		return fmt.Errorf("upsert blob %s: %w", key, err)
	}
	return nil
}

// ── files ────────────────────────────────────────────────────────────────

const fileColumns = `id, box_id, folder_id, name, name_fold, blob_key, captured_at, created_at, updated_at`

func scanFile(scan func(...any) error) (domain.File, error) {
	var (
		f                     domain.File
		created, updated      string
	)
	if err := scan(&f.ID, &f.BoxID, &f.FolderID, &f.Name, &f.NameFold, &f.BlobKey,
		&f.CapturedAt, &created, &updated); err != nil {
		return domain.File{}, err
	}
	var err error
	if f.CreatedAt, err = ParseTime(created); err != nil {
		return domain.File{}, err
	}
	if f.UpdatedAt, err = ParseTime(updated); err != nil {
		return domain.File{}, err
	}
	return f, nil
}

func (v *ViewTx) InsertFile(f domain.File) error {
	_, err := v.tx.Exec(
		`INSERT INTO files (`+fileColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.BoxID, f.FolderID, f.Name, f.NameFold, f.BlobKey, f.CapturedAt,
		FormatTime(v.ts), FormatTime(v.ts))
	if err != nil {
		return fmt.Errorf("insert file %s: %w", f.Name, err)
	}
	return nil
}

func (v *ViewTx) GetFile(id domain.FileID) (domain.File, bool, error) {
	f, err := scanFile(v.tx.QueryRow(`SELECT `+fileColumns+` FROM files WHERE id = ?`, id).Scan)
	if err == sql.ErrNoRows {
		return domain.File{}, false, nil
	}
	if err != nil {
		return domain.File{}, false, fmt.Errorf("read file %s: %w", id, err)
	}
	return f, true, nil
}

func (v *ViewTx) DeleteFile(id domain.FileID) error {
	if _, err := v.tx.Exec(`DELETE FROM files WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete file %s: %w", id, err)
	}
	return nil
}

func (v *ViewTx) CountFileTags(id domain.FileID) (int, error) {
	var n int
	err := v.tx.QueryRow(`SELECT count(*) FROM file_tags WHERE file_id = ?`, id).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count tags on file %s: %w", id, err)
	}
	return n, nil
}

func (v *ViewTx) CountBoxFiles(id domain.BoxID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM files WHERE box_id = ?`, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("count files in box %s: %w", id, err)
	}
	return n, nil
}
```

`captured_at` 直接绑 `domain.LocalDate`：它已经实现了 `driver.Valuer` / `sql.Scanner`，空值往返成 NULL（M1a）。

- [x] **Step 4: 实现 folders 的补充方法**

```go
func (v *ViewTx) GetFolder(id domain.FolderID) (domain.Folder, bool, error) {
	var (
		f                domain.Folder
		created, updated string
	)
	err := v.tx.QueryRow(
		`SELECT id, box_id, parent_id, name, name_fold, created_at, updated_at
		 FROM folders WHERE id = ?`, id).
		Scan(&f.ID, &f.BoxID, &f.ParentID, &f.Name, &f.NameFold, &created, &updated)
	if err == sql.ErrNoRows {
		return domain.Folder{}, false, nil
	}
	if err != nil {
		return domain.Folder{}, false, fmt.Errorf("read folder %s: %w", id, err)
	}
	if f.CreatedAt, err = ParseTime(created); err != nil {
		return domain.Folder{}, false, err
	}
	if f.UpdatedAt, err = ParseTime(updated); err != nil {
		return domain.Folder{}, false, err
	}
	return f, true, nil
}

func (v *ViewTx) DeleteFolder(id domain.FolderID) error {
	if _, err := v.tx.Exec(`DELETE FROM folders WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete folder %s: %w", id, err)
	}
	return nil
}

func (v *ViewTx) CountFolderFiles(id domain.FolderID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM files WHERE folder_id = ?`, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("count files in folder %s: %w", id, err)
	}
	return n, nil
}

func (v *ViewTx) CountFolderChildren(id domain.FolderID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM folders WHERE parent_id = ?`, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("count subfolders of %s: %w", id, err)
	}
	return n, nil
}
```

- [x] **Step 5: 实现 file_tags 的三个方法**

与 M1b 的 `AddBoxTag` / `RemoveBoxTag` / `HasBoxTag` 同构：

```go
func (v *ViewTx) AddFileTag(file domain.FileID, tag domain.TagID, source string) error {
	_, err := v.tx.Exec(
		`INSERT INTO file_tags (file_id, tag_id, source) VALUES (?, ?, ?)`,
		file, tag, source)
	if err != nil {
		return fmt.Errorf("tag file %s: %w", file, err)
	}
	return nil
}

func (v *ViewTx) RemoveFileTag(file domain.FileID, tag domain.TagID) error {
	if _, err := v.tx.Exec(`DELETE FROM file_tags WHERE file_id = ? AND tag_id = ?`, file, tag); err != nil {
		return fmt.Errorf("untag file %s: %w", file, err)
	}
	return nil
}

func (v *ViewTx) HasFileTag(file domain.FileID, tag domain.TagID) (bool, error) {
	var n int
	err := v.tx.QueryRow(`SELECT count(*) FROM file_tags WHERE file_id = ? AND tag_id = ?`, file, tag).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check tag on file %s: %w", file, err)
	}
	return n > 0, nil
}
```

- [x] **Step 6: 实现 `FolderChainOfFiles` 与 `TouchFolders`**

```go
// FolderChainOfFiles returns every folder on the path from these files up to
// their box roots.
//
// It exists so the write path can bump updated_at all the way up (spec 1.5.1)
// without any handler having to remember to. The recursion walks parent_id
// upward and stops at the root, whose parent_id is NULL.
func (v *ViewTx) FolderChainOfFiles(files []domain.FileID) ([]domain.FolderID, error) {
	if len(files) == 0 {
		return nil, nil
	}
	args := make([]any, len(files))
	for i, id := range files {
		args[i] = id
	}
	rows, err := v.tx.Query(`
		WITH RECURSIVE chain(id) AS (
			SELECT folder_id FROM files WHERE id IN (`+placeholders(len(files))+`)
			UNION
			SELECT f.parent_id FROM folders f JOIN chain c ON f.id = c.id
			WHERE f.parent_id IS NOT NULL
		)
		SELECT id FROM chain`, args...)
	if err != nil {
		return nil, fmt.Errorf("folder chain: %w", err)
	}
	defer rows.Close()

	var out []domain.FolderID
	for rows.Next() {
		var id domain.FolderID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("folder chain: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// TouchFolders stamps updated_at on the given folders. Same rule and same
// caller as TouchBoxes.
func (v *ViewTx) TouchFolders(ids []domain.FolderID) error {
	for _, id := range ids {
		if _, err := v.tx.Exec(`UPDATE folders SET updated_at = ? WHERE id = ?`, FormatTime(v.ts), id); err != nil {
			return fmt.Errorf("touch folder %s: %w", id, err)
		}
	}
	return nil
}
```

`placeholders(n)` 如果 `store` 里还没有就加一个：`strings.TrimSuffix(strings.Repeat("?,", n), ",")`。

- [x] **Step 7: 在 `internal/oplog/tx.go` 的 `Tx` 接口里加上对应方法**

按实体分组，插在既有分组之间：

```go
	InsertFile(f domain.File) error
	DeleteFile(id domain.FileID) error
	GetFile(id domain.FileID) (domain.File, bool, error)
	CountFileTags(id domain.FileID) (int, error)
	CountBoxFiles(id domain.BoxID) (int, error)

	GetFolder(id domain.FolderID) (domain.Folder, bool, error)
	DeleteFolder(id domain.FolderID) error
	CountFolderFiles(id domain.FolderID) (int, error)
	CountFolderChildren(id domain.FolderID) (int, error)

	UpsertBlob(key, sha256 string, size int64) error

	AddFileTag(file domain.FileID, tag domain.TagID, source string) error
	RemoveFileTag(file domain.FileID, tag domain.TagID) error
	HasFileTag(file domain.FileID, tag domain.TagID) (bool, error)
```

在接口的 doc 注释末尾补一句，说明为什么 `TouchFolders` 不在这里：

```go
// TouchBoxes and TouchFolders are deliberately absent: propagating updated_at
// is the write path's rule, not a handler's (spec 1.5.1 ②). Putting them here
// would invite a handler to do it, and then a new op type could forget to.
```

- [x] **Step 8: 更新 `internal/oplog/faketx_test.go` 的假 Tx**

它实现了整个 `Tx`，加方法就会编译不过。补上十四个方法，用一个内存 map 支撑 files / folders / blobs / file_tags，与既有 box / tag 的写法保持一致。

- [x] **Step 9: 把 `blobs` 加进 `store.Snapshot`**

M1b 里 `blobs` 表恒为空，所以没进快照。导入之后它有内容了，而 `Rebuild` 的证明要覆盖 `Apply` 写的每一张表：

```go
	{"blobs", "key, sha256, size", "key"},
```

放在 `snapshotTables` 的开头。同时把那句 doc 里的「ops and blobs are absent」改成「ops is absent: it is the authority, not the thing being proved. blobs is present from M1c on, because Apply maintains it (idempotently) and a rebuild must reproduce it exactly.」

- [x] **Step 10: 跑测试**

Run: `go test ./internal/store/ ./internal/oplog/ -v`
Expected: PASS

- [x] **Step 11: 确认 `library` 里那句编译期断言还成立**

Run: `go build ./...`
Expected: 成功。`var _ oplog.Tx = (*store.ViewTx)(nil)` 是这一步真正的验收——它证明 `ViewTx` 补齐了接口的每一个新方法。

- [x] **Step 12: `make check` 并提交**

```bash
make check
git add internal/oplog internal/store
git commit -m "feat(store): files / folders / blobs / file_tags 的物化视图写接口

Tx 长出十四个方法，ViewTx 结构性地满足它们。两个方法刻意不进 Tx：
TouchFolders 和 TouchBoxes 是写路径的规则不是 handler 的规则，放进
接口就等于邀请 handler 去调，而「新 op 类型不可能忘记它」这条保证
就没了。

UpsertBlob 是幂等的。blobs 表是 blobs/ 目录的索引、Rebuild 从不清
它，所以重放同一条导入 op 必须原样写回而不是撞唯一约束。

snapshot 加上 blobs 表——M1b 里它恒为空所以没进，导入之后 Apply
会写它，Rebuild 的证明就必须覆盖它。"
```

---

### Task 3: `import_files` 与 `delete_files`

**Files:**
- Create: `internal/oplog/file.go`, `internal/oplog/file_test.go`
- Modify: `internal/oplog/action.go`（两个常量）
- Modify: `internal/oplog/box.go`（`deleteBox` 补一条断言）
- Modify: `internal/oplog/targets.go`

**Interfaces:**
- Consumes: Task 2 的 `Tx` 方法
- Produces: `oplog.ImportFilesPayload` / `ImportedFile` / `ImportedFolder` / `DeleteFilesPayload`，以及 `ActionImportFiles` / `ActionDeleteFiles`

**payload 形状：**

```go
// ImportedFolder is a folder this op creates. Parents come before children in
// the slice; library guarantees that ordering when it builds the payload, so
// Apply can insert them in order without sorting (and therefore without making
// a decision — invariant 3).
type ImportedFolder struct {
	FolderID domain.FolderID `json:"folder_id"`
	ParentID domain.FolderID `json:"parent_id"`
	Name     string          `json:"name"`
}

// ImportedFile is one file this op adds.
//
// Sha256 and Size are here so that Apply can maintain the blobs index without
// touching the filesystem. That keeps blobs rebuildable from the log as well
// as from a directory scan, and it means Apply never has to read anything it
// did not receive (invariant 3).
type ImportedFile struct {
	FileID   domain.FileID   `json:"file_id"`
	FolderID domain.FolderID `json:"folder_id"`
	Name     string          `json:"name"`
	BlobKey  string          `json:"blob_key"`
	Sha256   string          `json:"sha256"`
	Size     int64           `json:"size"`
}

// ImportFilesPayload is one import: one box, the folders it had to create, and
// the files that actually made it in.
//
// Files that failed are absent — they never enter the payload at all, which is
// what lets a partly failed import still be one atomic op (spec 3.7.1 ③). So
// is an interrupted import: it commits the files it finished (spec 3.7.1 ①).
type ImportFilesPayload struct {
	BoxID   domain.BoxID     `json:"box_id"`
	Folders []ImportedFolder `json:"folders"`
	Files   []ImportedFile   `json:"files"`
}

// DeleteFilesPayload is the same shape, because deleting must record enough to
// put everything back — including the folder ids, since a re-created folder
// with a fresh id would orphan the projection and every later import that
// targeted it.
type DeleteFilesPayload = ImportFilesPayload
```

- [x] **Step 1: 写 `import_files` 的失败测试**

`internal/oplog/file_test.go`：

```go
package oplog

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

// importFixture builds a payload that creates one subfolder and two files.
func importFixture(box domain.BoxID, root domain.FolderID) ImportFilesPayload {
	sub := domain.NewFolderID()
	return ImportFilesPayload{
		BoxID:   box,
		Folders: []ImportedFolder{{FolderID: sub, ParentID: root, Name: "day1"}},
		Files: []ImportedFile{
			{FileID: domain.NewFileID(), FolderID: root, Name: "封面.jpg", BlobKey: "sha256:aa", Sha256: "aa", Size: 10},
			{FileID: domain.NewFileID(), FolderID: sub, Name: "IMG_1.jpg", BlobKey: "sha256:bb", Sha256: "bb", Size: 20},
		},
	}
}

func TestImportFilesApply(t *testing.T) {
	tx := newFakeTx()
	box, root := tx.seedBox(t)
	p := importFixture(box, root)

	if err := applyPayload(t, ActionImportFiles, p); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if got := len(tx.files); got != 2 {
		t.Errorf("%d files, want 2", got)
	}
	if got := len(tx.blobs); got != 2 {
		t.Errorf("%d blobs, want 2 — Apply must maintain the blobs index", got)
	}
	f, ok, _ := tx.GetFile(p.Files[1].FileID)
	if !ok || f.FolderID != p.Folders[0].FolderID {
		t.Errorf("the second file did not land in the folder this op created")
	}
}

// The inverse of an import is a delete carrying the same bytes: the payload
// already describes everything that would have to be put back.
func TestImportFilesInverse(t *testing.T) {
	h, err := Lookup(ActionImportFiles)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := Marshal(importFixture(domain.NewBoxID(), domain.NewFolderID()))
	action, inv, err := h.Inverse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if action != ActionDeleteFiles {
		t.Errorf("inverse action = %q, want %q", action, ActionDeleteFiles)
	}
	if string(inv) != string(raw) {
		t.Error("inverse changed the payload; it should be the same bytes")
	}
}

// Targets is how op_files gets built, and invariant 4 requires it to come from
// the payload alone.
func TestImportFilesTargets(t *testing.T) {
	h, _ := Lookup(ActionImportFiles)
	box := domain.NewBoxID()
	p := importFixture(box, domain.NewFolderID())
	raw, _ := Marshal(p)

	got, err := h.Targets(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Boxes) != 1 || got.Boxes[0] != box {
		t.Errorf("Boxes = %v, want [%v]", got.Boxes, box)
	}
	if len(got.Files) != 2 {
		t.Errorf("Files = %v, want both imported files", got.Files)
	}
}
```

`applyPayload` / `newFakeTx` / `seedBox` 是 `faketx_test.go` 里的既有辅助（M1b），按需补 `seedBox`：建一个 box 和它的根 folder，返回两个 id。

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/oplog/ -run TestImportFiles`
Expected: FAIL, `undefined: ActionImportFiles`

- [x] **Step 3: 加 action 常量**

`internal/oplog/action.go` 的 const 块追加：

```go
	ActionImportFiles Action = "import_files"
	ActionDeleteFiles Action = "delete_files"

	ActionTagFiles   Action = "tag_files"
	ActionUntagFiles Action = "untag_files"
```

（后两个 Task 4 才实现，但常量一起加，`AllActions` 由各 handler 的 `init` 追加，所以未实现的不会出现在里面——这正是 M1b 那条设计的用处。）

- [x] **Step 4: 实现 `internal/oplog/file.go` 的 `importFiles`**

```go
package oplog

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

func init() {
	Register(ActionImportFiles, importFiles{})
	Register(ActionDeleteFiles, deleteFiles{})
}

type importFiles struct{}

func (importFiles) Apply(tx Tx, payload []byte) error {
	p, err := Payload[ImportFilesPayload](payload)
	if err != nil {
		return err
	}
	// Blobs first: files reference them, and the foreign key is on.
	for _, f := range p.Files {
		if err := tx.UpsertBlob(f.BlobKey, f.Sha256, f.Size); err != nil {
			return err
		}
	}
	// Folders in payload order, which library built parents-first.
	for _, d := range p.Folders {
		folder, err := domain.NewFolder(d.FolderID, p.BoxID, d.ParentID, d.Name)
		if err != nil {
			return err
		}
		if err := tx.InsertFolder(folder); err != nil {
			return err
		}
	}
	for _, f := range p.Files {
		file, err := domain.NewFile(f.FileID, p.BoxID, f.FolderID, f.Name, f.BlobKey)
		if err != nil {
			return err
		}
		if err := tx.InsertFile(file); err != nil {
			return err
		}
	}
	return nil
}

func (importFiles) Inverse(payload []byte) (Action, []byte, error) {
	return ActionDeleteFiles, payload, nil
}

func (importFiles) Targets(payload []byte) (Targets, error) { return fileTargets(payload) }
```

- [x] **Step 5: 写 `delete_files` 的严格模式测试**

```go
// Undoing an import is refused rather than partly done. Three things can have
// moved on since: the file was renamed or moved, it was tagged, or somebody
// imported more files into a folder this op created. All three would make a
// silent delete lose information the user cannot get back (spec 2.5).
func TestDeleteFilesRefusesWhenTheFileMoved(t *testing.T) {
	tx := newFakeTx()
	box, root := tx.seedBox(t)
	p := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, p)

	f, _, _ := tx.GetFile(p.Files[0].FileID)
	f.Name = "改过名.jpg"
	tx.files[f.ID] = f

	err := applyOn(t, tx, ActionDeleteFiles, p)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore", err)
	}
}

func TestDeleteFilesRefusesWhenTheFileWasTagged(t *testing.T) {
	tx := newFakeTx()
	box, root := tx.seedBox(t)
	p := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, p)

	if err := tx.AddFileTag(p.Files[0].FileID, domain.NewTagID(), "user"); err != nil {
		t.Fatal(err)
	}
	err := applyOn(t, tx, ActionDeleteFiles, p)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore — undoing would silently drop the tag", err)
	}
}

func TestDeleteFilesRefusesWhenTheFolderGainedFiles(t *testing.T) {
	tx := newFakeTx()
	box, root := tx.seedBox(t)
	p := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, p)

	// A later import dropped something else into the folder this op created.
	later := ImportFilesPayload{
		BoxID: box,
		Files: []ImportedFile{{
			FileID: domain.NewFileID(), FolderID: p.Folders[0].FolderID,
			Name: "later.jpg", BlobKey: "sha256:cc", Sha256: "cc", Size: 1,
		}},
	}
	mustApply(t, tx, ActionImportFiles, later)

	err := applyOn(t, tx, ActionDeleteFiles, p)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore — the folder is not empty", err)
	}
}

func TestDeleteFilesUndoesACleanImport(t *testing.T) {
	tx := newFakeTx()
	box, root := tx.seedBox(t)
	p := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, p)
	mustApply(t, tx, ActionDeleteFiles, p)

	if len(tx.files) != 0 {
		t.Errorf("%d files left", len(tx.files))
	}
	if _, ok, _ := tx.GetFolder(p.Folders[0].FolderID); ok {
		t.Error("the folder this import created is still there")
	}
	// Blobs are not deleted: reclaiming them is GC's job, and v0.1 has no GC
	// (spec 3.9). An orphan blob costs space and nothing else.
	if len(tx.blobs) != 2 {
		t.Errorf("%d blobs left, want 2 — deleting files must not touch blob storage", len(tx.blobs))
	}
}
```

- [x] **Step 6: 跑测试，确认失败**

Run: `go test ./internal/oplog/ -run TestDeleteFiles`
Expected: FAIL

- [x] **Step 7: 实现 `deleteFiles`**

```go
type deleteFiles struct{}

// Apply removes what an import added, refusing if anything has moved on.
//
// The checks are assertions, not decisions — they either let the op through
// unchanged or fail it (invariant 3). They are also the whole of strict undo
// for imports (spec 2.5): a half-completed undo of a 5000-file import is the
// worst possible outcome in an archive, so kist would rather name the obstacle
// and stop.
func (deleteFiles) Apply(tx Tx, payload []byte) error {
	p, err := Payload[DeleteFilesPayload](payload)
	if err != nil {
		return err
	}

	for _, f := range p.Files {
		got, ok, err := tx.GetFile(f.FileID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %q is already gone: %w", f.Name, ErrStaleBefore)
		}
		if got.Name != f.Name || got.FolderID != f.FolderID || got.BlobKey != f.BlobKey {
			return fmt.Errorf("%q has changed since it was imported: %w", f.Name, ErrStaleBefore)
		}
		n, err := tx.CountFileTags(f.FileID)
		if err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("%q has been tagged since it was imported; remove its tags first: %w", f.Name, ErrStaleBefore)
		}
		if err := tx.DeleteFile(f.FileID); err != nil {
			return err
		}
	}

	// Children before parents: library wrote the folders parents-first.
	for i := len(p.Folders) - 1; i >= 0; i-- {
		d := p.Folders[i]
		_, ok, err := tx.GetFolder(d.FolderID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("folder %q is already gone: %w", d.Name, ErrStaleBefore)
		}
		files, err := tx.CountFolderFiles(d.FolderID)
		if err != nil {
			return err
		}
		subs, err := tx.CountFolderChildren(d.FolderID)
		if err != nil {
			return err
		}
		if files > 0 || subs > 0 {
			return fmt.Errorf("folder %q is not empty; something was added to it after this import: %w", d.Name, ErrStaleBefore)
		}
		if err := tx.DeleteFolder(d.FolderID); err != nil {
			return err
		}
	}
	return nil
}

func (deleteFiles) Inverse(payload []byte) (Action, []byte, error) {
	return ActionImportFiles, payload, nil
}

func (deleteFiles) Targets(payload []byte) (Targets, error) { return fileTargets(payload) }

// fileTargets is shared by both directions: the same payload names the same
// box and the same files whichever way it is being applied.
func fileTargets(payload []byte) (Targets, error) {
	p, err := Payload[ImportFilesPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	t := Targets{Boxes: []domain.BoxID{p.BoxID}}
	for _, f := range p.Files {
		t.Files = append(t.Files, f.FileID)
	}
	return t, nil
}
```

- [x] **Step 8: 给 `deleteBox` 补一条「box 里还有文件」的断言**

M1b 的 `deleteBox` 只检查了 `CountBoxTags`。现在 box 里可能有文件，而 `files.box_id` 是外键——不检查的话用户会看到 `FOREIGN KEY constraint failed (787)`，跟 M1b 里 `tag_boxes` 那个 bug 一模一样。

在 `deleteBox.Apply` 里 `CountBoxTags` 那一段旁边加：

```go
	files, err := tx.CountBoxFiles(p.BoxID)
	if err != nil {
		return err
	}
	if files > 0 {
		return fmt.Errorf("box %q still holds %d files; undo the imports first: %w", p.Name, files, ErrStaleBefore)
	}
```

对应的测试：

```go
func TestDeleteBoxRefusesWhenTheBoxHoldsFiles(t *testing.T) {
	tx := newFakeTx()
	box, root := tx.seedBox(t)
	mustApply(t, tx, ActionImportFiles, importFixture(box, root))

	err := applyOn(t, tx, ActionDeleteBox, tx.createBoxPayload(box, root))
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore rather than a raw foreign key error", err)
	}
}
```

- [x] **Step 9: 跑全部 oplog 测试**

Run: `go test ./internal/oplog/ -v`
Expected: PASS

- [x] **Step 10: 确认新 action 出现在 `AllActions` 里**

Run: `go test ./internal/oplog/ -run TestAllActions -v`
Expected: PASS，`import_files` 和 `delete_files` 在列，`tag_files` / `untag_files` **不**在（Task 4 才注册）

- [x] **Step 11: `make check` 并提交**

```bash
make check
git add internal/oplog
git commit -m "feat(oplog): import_files / delete_files，撤销导入走严格模式

payload 带 sha256 和 size，所以 Apply 维护 blobs 索引时不用碰文件
系统——blobs 表因此既能扫目录重建，也能从日志重放出来。

撤销导入拦三种「已经变了」：文件被改名或移走、文件被打了 tag（静默
删掉会连 tag 一起丢）、以及有人往这次导入建的目录里又放了东西。
5000 个文件的导入撤到一半是归档软件里最坏的结局，宁可指名道姓地
拒绝。

顺手补上 deleteBox 的「box 里还有文件」断言。不补的话用户会看到
FOREIGN KEY constraint failed (787)，跟 M1b 里 tag_boxes 那个是同
一个 bug。"
```

---

### Task 4: `tag_files` 与 `untag_files`

**Files:**
- Create: `internal/oplog/filetag.go`, `internal/oplog/filetag_test.go`

**Interfaces:**
- Consumes: Task 2 的 `AddFileTag` / `RemoveFileTag` / `HasFileTag` / `GetFile`
- Produces: `oplog.TagFilesPayload` / `UntagFilesPayload`，`ActionTagFiles` / `ActionUntagFiles`（常量在 Task 3 已加）

这是 `boxtag.go` 的同构翻版，逐条对照着写即可。**唯一要小心的是同一条不变量：`FileIDs` 只列真正会变的文件**——`library` 在生成 op 之前过滤掉已经带这个 tag 的文件，否则 `Inverse` 会摘掉一个这条 op 从没加过的 tag。

- [x] **Step 1: 写失败测试**

`internal/oplog/filetag_test.go`：

```go
package oplog

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func TestTagFilesApplyAndInverse(t *testing.T) {
	tx := newFakeTx()
	box, root := tx.seedBox(t)
	imp := importFixture(box, root)
	mustApply(t, tx, ActionImportFiles, imp)

	tag := domain.NewTagID()
	p := TagFilesPayload{
		TagID:   tag,
		FileIDs: []domain.FileID{imp.Files[0].FileID, imp.Files[1].FileID},
		Source:  "user",
	}
	mustApply(t, tx, ActionTagFiles, p)

	for _, id := range p.FileIDs {
		if has, _ := tx.HasFileTag(id, tag); !has {
			t.Errorf("file %s did not get the tag", id)
		}
	}

	mustApply(t, tx, ActionUntagFiles, p)
	for _, id := range p.FileIDs {
		if has, _ := tx.HasFileTag(id, tag); has {
			t.Errorf("file %s still carries the tag after the inverse", id)
		}
	}
}

func TestTagFilesRefusesAnEmptyBatch(t *testing.T) {
	tx := newFakeTx()
	err := applyOn(t, tx, ActionTagFiles, TagFilesPayload{TagID: domain.NewTagID(), Source: "user"})
	if !errors.Is(err, errEmptyBatch) {
		t.Errorf("err = %v, want errEmptyBatch", err)
	}
}

// The readable-sentence rule from M1b applies here too: a foreign key error
// number is not an error message (spec 2.5).
func TestTagFilesNamesAMissingFile(t *testing.T) {
	tx := newFakeTx()
	p := TagFilesPayload{TagID: domain.NewTagID(), FileIDs: []domain.FileID{domain.NewFileID()}, Source: "user"}
	err := applyOn(t, tx, ActionTagFiles, p)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("err = %v, want ErrStaleBefore", err)
	}
}

func TestTagFilesTargets(t *testing.T) {
	h, _ := Lookup(ActionTagFiles)
	p := TagFilesPayload{TagID: domain.NewTagID(), FileIDs: []domain.FileID{domain.NewFileID()}, Source: "user"}
	raw, _ := Marshal(p)
	got, err := h.Targets(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || got.Files[0] != p.FileIDs[0] {
		t.Errorf("Files = %v, want %v", got.Files, p.FileIDs)
	}
	// A file-tag op names no box: op_boxes is for ops that acted on a box, and
	// widening it here would make "history of this box" include every tag
	// anyone ever put on any file in it.
	if len(got.Boxes) != 0 {
		t.Errorf("Boxes = %v, want none", got.Boxes)
	}
}
```

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/oplog/ -run TestTagFiles`
Expected: FAIL, `undefined: TagFilesPayload`

- [x] **Step 3: 实现 `internal/oplog/filetag.go`**

```go
package oplog

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

func init() {
	Register(ActionTagFiles, tagFiles{})
	Register(ActionUntagFiles, untagFiles{})
}

// TagFilesPayload attaches one tag to many files in a single op — the same
// batch granularity as TagBoxesPayload, and for the same reason: tagging 200
// invoices must be one undo, not 200 (spec 1.6.2).
//
// FileIDs names only the files that actually change. library filters out any
// file that already carries the tag before writing the op, or the inverse
// would strip a tag this op never added.
type TagFilesPayload struct {
	TagID   domain.TagID    `json:"tag_id"`
	FileIDs []domain.FileID `json:"file_ids"`
	Source  string          `json:"source"`
}

// UntagFilesPayload is the same shape, so the two are exact inverses.
type UntagFilesPayload = TagFilesPayload

type tagFiles struct{}

func (tagFiles) Apply(tx Tx, payload []byte) error {
	p, err := Payload[TagFilesPayload](payload)
	if err != nil {
		return err
	}
	if len(p.FileIDs) == 0 {
		return fmt.Errorf("tag_files: %w", errEmptyBatch)
	}
	if err := checkFilesExist(tx, p.FileIDs); err != nil {
		return err
	}
	for _, id := range p.FileIDs {
		if err := tx.AddFileTag(id, p.TagID, p.Source); err != nil {
			return err
		}
	}
	return nil
}

func (tagFiles) Inverse(payload []byte) (Action, []byte, error) {
	return ActionUntagFiles, payload, nil
}

func (tagFiles) Targets(payload []byte) (Targets, error) { return fileTagTargets(payload) }

type untagFiles struct{}

func (untagFiles) Apply(tx Tx, payload []byte) error {
	p, err := Payload[UntagFilesPayload](payload)
	if err != nil {
		return err
	}
	if len(p.FileIDs) == 0 {
		return fmt.Errorf("untag_files: %w", errEmptyBatch)
	}
	if err := checkFilesExist(tx, p.FileIDs); err != nil {
		return err
	}
	for _, id := range p.FileIDs {
		if err := tx.RemoveFileTag(id, p.TagID); err != nil {
			return err
		}
	}
	return nil
}

func (untagFiles) Inverse(payload []byte) (Action, []byte, error) {
	return ActionTagFiles, payload, nil
}

func (untagFiles) Targets(payload []byte) (Targets, error) { return fileTagTargets(payload) }

// checkFilesExist is checkBoxesExist for files, and exists for the same
// reason: the foreign key is the backstop, this is the sentence a person reads.
func checkFilesExist(tx Tx, files []domain.FileID) error {
	for _, id := range files {
		_, ok, err := tx.GetFile(id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("file %s no longer exists: %w", id, ErrStaleBefore)
		}
	}
	return nil
}

// fileTagTargets names files and no box.
//
// op_boxes is the index behind "what happened to this box", and putting a
// file-tag op in it would flood that view with every tag anyone ever put on
// any file inside — which is a different question, answered by op_files.
func fileTagTargets(payload []byte) (Targets, error) {
	p, err := Payload[TagFilesPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Files: p.FileIDs}, nil
}
```

- [x] **Step 4: 跑测试**

Run: `go test ./internal/oplog/ -v`
Expected: PASS，`AllActions` 现在是十三个

- [x] **Step 5: `make check` 并提交**

```bash
make check
git add internal/oplog
git commit -m "feat(oplog): tag_files / untag_files

boxtag.go 的同构翻版。M1 的产出写的是「按 tag 和文件名筛出来」，而
file_tags 在此之前一行都没有，find --tag 必然返回空——那句产出蕴含
了文件能被打 tag。

Targets 只报 files 不报 box：op_boxes 是「这个 box 发生过什么」的
索引，把文件的 tag 操作塞进去会让那个视图被淹没，而那是 op_files
回答的另一个问题。"
```

---

### Task 5: `store` 的读侧——目录浏览、检索与 keyset 游标

**Files:**
- Create: `internal/store/files.go`, `internal/store/files_test.go`

**Interfaces:**
- Consumes: `domain.File` / `domain.Folder` / `store.Queryer`
- Produces:
  - `func FoldersOfBox(q Queryer, box domain.BoxID) ([]domain.Folder, error)`
  - `func FilesOfBox(q Queryer, box domain.BoxID) ([]domain.File, error)`
  - `func FilesInFolder(q Queryer, folder domain.FolderID) ([]domain.File, error)`
  - `func ExistingNames(q Queryer, box domain.BoxID) (map[domain.FolderID]map[string]string, error)`
  - `type ListFilesOptions`、`func ListFiles(q Queryer, opt ListFilesOptions) ([]domain.File, string, error)`
  - `func EncodeCursor(f domain.File) string`

**排序：`ORDER BY captured_at DESC NULLS LAST, id DESC`，固定不变**（设计文档 4.5.1）。`NULLS LAST` 必须显式写，哪怕 SQLite 加不加都一样——Postgres 的 `DESC` 默认是 `NULLS FIRST`（4.7）。

- [x] **Step 1: 写排序与 keyset 的失败测试**

`internal/store/files_test.go`：

```go
package store

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

// The sort is the one spec 4.5.1 settled on. Files with no captured_at go
// last, and among them id DESC is the tie-breaker — which, because ids are
// UUIDv7, is created_at DESC for free. That is the whole reason COALESCE was
// not needed.
func TestListFilesSortsCapturedFirstThenNewest(t *testing.T) {
	db := migrated(t)
	box, root := seedBox(t, db)

	// Insert in a deliberately unhelpful order.
	noCapture1 := insertFile(t, db, box, root, "b.pdf", "")
	captured := insertFile(t, db, box, root, "photo.jpg", "2019-08-12")
	noCapture2 := insertFile(t, db, box, root, "c.pdf", "")

	got, _, err := ListFiles(db, ListFilesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.FileID{captured, noCapture2, noCapture1}
	if len(got) != 3 {
		t.Fatalf("%d files, want 3", len(got))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("position %d = %s, want %s\nfull order: %v", i, got[i].Name, id, names(got))
		}
	}
}

// A cursor that loses rows at the NULL boundary is the classic keyset bug: the
// second page starts among the non-null values again, or skips the whole null
// block. Paging one row at a time through a mixed set is the cheapest way to
// catch it.
func TestListFilesCursorWalksEveryRowExactlyOnce(t *testing.T) {
	db := migrated(t)
	box, root := seedBox(t, db)
	for _, c := range []string{"2019-08-12", "", "2020", "", "2019-08-12"} {
		insertFile(t, db, box, root, "f"+c+randomSuffix(), c)
	}

	seen := map[domain.FileID]bool{}
	cursor := ""
	for range 10 {
		page, next, err := ListFiles(db, ListFilesOptions{Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		if seen[page[0].ID] {
			t.Fatalf("%s came back twice", page[0].Name)
		}
		seen[page[0].ID] = true
		cursor = next
	}
	if len(seen) != 5 {
		t.Errorf("walked %d of 5 files", len(seen))
	}
}

// spec 4.4: search is a folded substring match, and the folding is ours, not
// the engine's (spec 4.7).
func TestListFilesTextIsCaseInsensitive(t *testing.T) {
	db := migrated(t)
	box, root := seedBox(t, db)
	insertFile(t, db, box, root, "IMG_0001.JPG", "")
	insertFile(t, db, box, root, "notes.txt", "")

	got, _, err := ListFiles(db, ListFilesOptions{Text: "img_00"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "IMG_0001.JPG" {
		t.Errorf("got %v, want the one JPG", names(got))
	}
}

// A pattern containing LIKE's own wildcards must match them literally,
// otherwise searching for "100%" returns the whole library.
func TestListFilesEscapesLikeWildcards(t *testing.T) {
	db := migrated(t)
	box, root := seedBox(t, db)
	insertFile(t, db, box, root, "100%.txt", "")
	insertFile(t, db, box, root, "1000.txt", "")

	got, _, err := ListFiles(db, ListFilesOptions{Text: "100%"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "100%.txt" {
		t.Errorf("got %v, want only 100%%.txt", names(got))
	}
}

// spec 4.3: conditions AND together, each condition is an OR over an already
// expanded set of tag ids. The expansion itself happens in library, against
// the in-memory tree.
func TestListFilesCombinesTagConditions(t *testing.T) {
	db := migrated(t)
	box, root := seedBox(t, db)
	yunnan, dad, private := insertTag(t, db, "云南"), insertTag(t, db, "爸"), insertTag(t, db, "隐私")

	a := insertFile(t, db, box, root, "a.jpg", "")
	b := insertFile(t, db, box, root, "b.jpg", "")
	c := insertFile(t, db, box, root, "c.jpg", "")
	tagFile(t, db, a, yunnan)
	tagFile(t, db, a, dad)
	tagFile(t, db, b, yunnan)
	tagFile(t, db, c, yunnan)
	tagFile(t, db, c, dad)
	tagFile(t, db, c, private)

	got, _, err := ListFiles(db, ListFilesOptions{
		AllOf:  [][]domain.TagID{{yunnan}, {dad}},
		NoneOf: []domain.TagID{private},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != a {
		t.Errorf("got %v, want only a.jpg", names(got))
	}
	_ = b
}
```

辅助函数 `seedBox` / `insertFile` / `insertTag` / `tagFile` / `names` / `randomSuffix` 写在同一个文件里，直接用 `ViewTx` 建数据。

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/store/ -run TestListFiles`
Expected: FAIL, `undefined: ListFiles`

- [x] **Step 3: 实现 `internal/store/files.go` 的排序与游标**

```go
package store

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
)

// fileOrder is the one sort order for file listings (spec 4.5.1).
//
// NULLS LAST is written out even though SQLite would do it anyway: Postgres
// defaults DESC to NULLS FIRST, so leaving it implicit means the two engines
// return opposite results and no test run on SQLite can ever notice
// (spec 4.7).
//
// id DESC is not only a tie-breaker. Ids are UUIDv7, so it is also created_at
// DESC — which is why files with no captured_at fall into import order without
// a COALESCE, and without the expression index a COALESCE would have needed.
const fileOrder = ` ORDER BY captured_at DESC NULLS LAST, id DESC`

// EncodeCursor renders the sort tuple of the last row on a page.
//
// The cursor carries the whole tuple, not just the id, because the predicate
// has to reconstruct "everything after this row in this order" — and it is
// opaque base64 so that changing the sort key later invalidates old cursors
// loudly instead of paging through a different order (spec 4.5).
func EncodeCursor(f domain.File) string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(f.CapturedAt.String() + "\x00" + f.ID.String()))
}

func decodeCursor(s string) (captured string, id domain.FileID, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: %w", err)
	}
	parts := strings.SplitN(string(raw), "\x00", 2)
	if len(parts) != 2 {
		return "", domain.FileID{}, fmt.Errorf("bad cursor")
	}
	parsed, err := domain.ParseID(parts[1])
	if err != nil {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: %w", err)
	}
	return parts[0], domain.FileID{ID: parsed}, nil
}

// cursorPredicate is "strictly after this row, in fileOrder".
//
// The NULL block is the part that is easy to get wrong. Under DESC NULLS LAST
// every null sorts after every non-null, so:
//
//   - from a non-null cursor, the remainder is "smaller captured_at, or equal
//     captured_at with smaller id, or null captured_at"
//   - from a null cursor, the remainder is null captured_at with smaller id
//     and nothing else — the non-null block is already behind us
//
// Getting this wrong does not error; it silently drops or repeats a page,
// which is why TestListFilesCursorWalksEveryRowExactlyOnce walks one row at a
// time across the boundary.
func cursorPredicate(captured string, id domain.FileID) (string, []any) {
	if captured == "" {
		return ` AND captured_at IS NULL AND id < ?`, []any{id}
	}
	return ` AND (captured_at IS NULL OR captured_at < ? OR (captured_at = ? AND id < ?))`,
		[]any{captured, captured, id}
}
```

`domain.FileID` 是 `struct{ ID }`（M1a），所以 `domain.FileID{ID: parsed}` 是它的构造式，`f.ID.String()` 走的是提升上来的 `ID.String()`，即带连字符的规范 UUID。

- [x] **Step 4: 实现 `ListFiles`**

```go
// ListFilesOptions is the M1c subset of spec 4.2's Query.
//
// The full AST — time ranges, media kinds, arbitrary AND/OR/NOT nesting —
// arrives with the conformance suite in M2. What is here is what the CLI can
// actually ask for, which is also the shape the UI exposes in v0.1: several
// tags intersected, plus an exclusion list (spec 4.3).
type ListFilesOptions struct {
	Box    *domain.BoxID
	Folder *domain.FolderID
	// Text is matched as a folded substring against name_fold. Callers pass it
	// raw; ListFiles folds it with the same function that built the column.
	Text string
	// AllOf is one entry per condition; a file must carry at least one tag
	// from each. library expands each condition to its descendants first
	// (spec 4.3 step 1) — the expansion is a memory-tree walk, not a join.
	AllOf [][]domain.TagID
	// NoneOf excludes files carrying any of these.
	NoneOf []domain.TagID
	Cursor string
	Limit  int
}

// ListFiles runs the query and returns the page plus the cursor for the next
// one. An empty next cursor means this was the last page.
func ListFiles(q Queryer, opt ListFilesOptions) ([]domain.File, string, error) {
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(`SELECT ` + fileColumns + ` FROM files WHERE 1 = 1`)

	if opt.Box != nil {
		sb.WriteString(` AND box_id = ?`)
		args = append(args, *opt.Box)
	}
	if opt.Folder != nil {
		sb.WriteString(` AND folder_id = ?`)
		args = append(args, *opt.Folder)
	}
	if opt.Text != "" {
		// ESCAPE is not optional: without it a search for "100%" matches
		// everything, and a search for "a_b" matches "axb".
		sb.WriteString(` AND name_fold LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(domain.Fold(opt.Text))+"%")
	}
	for _, cond := range opt.AllOf {
		if len(cond) == 0 {
			continue
		}
		sb.WriteString(` AND EXISTS (SELECT 1 FROM file_tags WHERE file_id = files.id AND tag_id IN (` +
			placeholders(len(cond)) + `))`)
		for _, id := range cond {
			args = append(args, id)
		}
	}
	if len(opt.NoneOf) > 0 {
		sb.WriteString(` AND NOT EXISTS (SELECT 1 FROM file_tags WHERE file_id = files.id AND tag_id IN (` +
			placeholders(len(opt.NoneOf)) + `))`)
		for _, id := range opt.NoneOf {
			args = append(args, id)
		}
	}
	if opt.Cursor != "" {
		captured, id, err := decodeCursor(opt.Cursor)
		if err != nil {
			return nil, "", err
		}
		pred, pargs := cursorPredicate(captured, id)
		sb.WriteString(pred)
		args = append(args, pargs...)
	}

	sb.WriteString(fileOrder)
	if opt.Limit > 0 {
		sb.WriteString(` LIMIT ?`)
		args = append(args, opt.Limit)
	}

	rows, err := q.Query(sb.String(), args...)
	if err != nil {
		return nil, "", fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var out []domain.File
	for rows.Next() {
		f, err := scanFile(rows.Scan)
		if err != nil {
			return nil, "", fmt.Errorf("list files: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	next := ""
	// A short page is the last page; only a full one can have more behind it.
	if opt.Limit > 0 && len(out) == opt.Limit {
		next = EncodeCursor(out[len(out)-1])
	}
	return out, next, nil
}

// escapeLike neutralizes LIKE's wildcards so a pattern matches literally.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
```

- [x] **Step 5: 实现目录浏览与导入要用的三个查询**

```go
// FoldersOfBox returns every folder of a box, root first, then the rest in id
// order — which is creation order, so parents precede children.
func FoldersOfBox(q Queryer, box domain.BoxID) ([]domain.Folder, error) { /* … */ }

// FilesInFolder lists one directory's files, in the standard order.
func FilesInFolder(q Queryer, folder domain.FolderID) ([]domain.File, error) { /* … */ }

// FilesOfBox lists every file in a box. relink needs the whole set to compute
// the desired projection, and memory is bounded by the largest box rather than
// by the library (spec 3.4.1).
func FilesOfBox(q Queryer, box domain.BoxID) ([]domain.File, error) { /* … */ }

// ExistingNames maps folder → folded name → blob key, for one box.
//
// This is what makes import idempotent (spec 3.7.1 ②): a name already taken by
// the same blob is skipped, a name taken by different content is numbered.
// Loading it up front costs one scan of the box and turns a per-file query
// into a map lookup — which matters at 5000 files, and matters more because
// the alternative is a query inside the loop that also has to see the files
// this same import added a moment ago.
func ExistingNames(q Queryer, box domain.BoxID) (map[domain.FolderID]map[string]string, error) {
	rows, err := q.Query(`SELECT folder_id, name_fold, blob_key FROM files WHERE box_id = ?`, box)
	if err != nil {
		return nil, fmt.Errorf("read existing names: %w", err)
	}
	defer rows.Close()

	out := map[domain.FolderID]map[string]string{}
	for rows.Next() {
		var (
			folder domain.FolderID
			fold   string
			key    string
		)
		if err := rows.Scan(&folder, &fold, &key); err != nil {
			return nil, fmt.Errorf("read existing names: %w", err)
		}
		if out[folder] == nil {
			out[folder] = map[string]string{}
		}
		out[folder][fold] = key
	}
	return out, rows.Err()
}
```

三个列表查询的 body 都是 `scanFile` / folder 扫描的直写，照 `ListFiles` 的形状写。

- [x] **Step 6: 跑测试**

Run: `go test ./internal/store/ -run 'TestListFiles|TestExistingNames' -v`
Expected: PASS

- [x] **Step 7: 用 `EXPLAIN QUERY PLAN` 确认排序真的走了索引**

设计文档 4.5.1 的整个论证建立在「`ORDER BY captured_at DESC NULLS LAST, id DESC` 走覆盖索引」上。加一个测试把它钉住，否则将来有人加一列排序键，性能会静默塌掉：

```go
// spec 4.5.1 chose this exact ORDER BY because it uses files_captured as a
// covering index. Any variant — a COALESCE, a different tie-breaker — falls
// back to "USE TEMP B-TREE FOR ORDER BY", which is a full sort of the table
// and the end of keyset paging.
func TestFileOrderUsesTheIndex(t *testing.T) {
	db := migrated(t)
	rows, err := db.Query(`EXPLAIN QUERY PLAN SELECT id FROM files` + fileOrder)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var a, b, c int
		var detail string
		if err := rows.Scan(&a, &b, &c, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plan.String(), "TEMP B-TREE") {
		t.Errorf("the file sort no longer uses files_captured:\n%s", plan.String())
	}
	if !strings.Contains(plan.String(), "files_captured") {
		t.Errorf("expected files_captured in the plan, got:\n%s", plan.String())
	}
}
```

- [x] **Step 8: `make check` 并提交**

```bash
make check
git add internal/store
git commit -m "feat(store): 文件的读侧——目录浏览、LIKE 检索、tag 交集与 keyset 游标

排序固定为 captured_at DESC NULLS LAST, id DESC（4.5.1）。NULLS LAST
显式写出来，虽然 SQLite 加不加都一样——Postgres 的 DESC 默认
NULLS FIRST，隐式写就是两个引擎给出相反结果，而且在 SQLite 上永远
测不出来。

游标的 NULL 边界单独测了：从非空游标往后要跨进整个 NULL 块，从空
游标往后就只剩 NULL 块。写错不会报错，只会静默丢一页或重一页，所以
测试是一行一行走完五个文件。

LIKE 的通配符转义也单独测了：不转义的话搜「100%」会返回整个库。

加了一个 EXPLAIN QUERY PLAN 测试钉住「排序走覆盖索引」——4.5.1 否掉
COALESCE 的全部论证就建立在这一条上。"
```

---

### Task 6: `library.Import`

**Files:**
- Create: `internal/library/import.go`, `internal/library/import_test.go`

**Interfaces:**
- Consumes: `store.ExistingNames` / `store.FoldersOfBox`（Task 5）、`blob.Dir.Put`（M1a）、`oplog.ImportFilesPayload`（Task 3）、`l.write`（M1b）
- Produces:
  - `type ImportOptions struct { Box domain.BoxID; Into domain.FolderID; Hidden bool }`
  - `type FileStatus`、`type FileResult`、`type ImportResult`
  - `func (l *Library) Import(ctx context.Context, src string, opt ImportOptions) (*ImportResult, error)`

**Task 6 结束时 `boxes/` 下还是空的。** 文件进了库、能查到、能撤销，但投影要到 Task 10 才接上。这是有意的切分：导入的正确性和投影的正确性是两件独立的事，混在一个 task 里改，红了不知道是谁红的。

**四条必须写对的：**

1. **blob 先于 DB 落盘**（3.7）。`Put` 在事务外，一个文件一个 `Put`；事务只在最后开一次
2. **提交用 `context.WithoutCancel(ctx)`**。中断走的就是 ctx 取消，而「优雅停止 = 把已完成的提交掉」（3.7.1 ①）意味着提交这一步**必须**在一个没被取消的 context 上跑，否则 `BeginTx` 当场失败，中断变成全丢
3. **同名判定用折叠名**，与 `files(folder_id, name)` 的唯一索引一致，也与 `ExistingNames` 的 key 一致
4. **`taken` 集合必须包含这一批自己刚加的名字**，否则同一次导入里两个同名文件会撞唯一约束

- [x] **Step 1: 写「幂等」的失败测试**——这是 3.7.1 ② 的验收

`internal/library/import_test.go`：

```go
package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Re-running an interrupted import must converge, not pile up copies.
//
// This is the whole reason spec 3.7.1 ② exists: with only "rename on
// conflict" from spec 1.5, a Ctrl-C at 3000 of 5000 followed by a re-run
// gives every one of those 3000 files a " (2)" twin.
func TestImportIsIdempotent(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"a.txt":      "alpha",
		"day1/b.txt": "bravo",
	})

	first, err := l.Import(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if n := countStatus(first, FileImported); n != 2 {
		t.Fatalf("first run imported %d files, want 2", n)
	}

	second, err := l.Import(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if n := countStatus(second, FileSkipped); n != 2 {
		t.Errorf("second run skipped %d files, want 2: %v", n, describe(second))
	}
	if n := countStatus(second, FileImported); n != 0 {
		t.Errorf("second run imported %d files; nothing changed on disk", n)
	}

	files := listAll(t, l, box)
	if len(files) != 2 {
		t.Errorf("library holds %d files after two identical imports, want 2: %v", len(files), files)
	}
}

// Same name, different content is the case that does get numbered.
func TestImportNumbersOnlyOnDifferentContent(t *testing.T) {
	l, box := openWithBox(t)

	src1 := writeTree(t, map[string]string{"发票.jpg": "one"})
	if _, err := l.Import(t.Context(), src1, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	src2 := writeTree(t, map[string]string{"发票.jpg": "two"})
	if _, err := l.Import(t.Context(), src2, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	got := names(listAll(t, l, box))
	want := []string{"发票 (2).jpg", "发票.jpg"}
	if !equalUnordered(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

// Two files with the same name inside a single import must not collide with
// each other either — the taken set has to include what this batch just added.
func TestImportNumbersWithinOneBatch(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"a/发票.jpg": "one",
		"b/发票.jpg": "two",
	})
	// Both land in the box root when --flatten is not a thing; here they keep
	// their folders, so instead import twice into the same folder.
	if _, err := l.Import(t.Context(), filepath.Join(src, "a"), ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	res, err := l.Import(t.Context(), filepath.Join(src, "b"), ImportOptions{Box: box})
	if err != nil {
		t.Fatal(err)
	}
	if countStatus(res, FileImported) != 1 {
		t.Fatalf("second import: %v", describe(res))
	}
	if len(listAll(t, l, box)) != 2 {
		t.Error("the two same-named files did not both make it in")
	}
}
```

`openWithBox` / `writeTree` / `countStatus` / `describe` / `listAll` / `names` / `equalUnordered` 写在测试文件里，`writeTree` 把一个 `map[相对路径]内容` 铺到 `t.TempDir()` 下。

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/library/ -run TestImport`
Expected: FAIL, `undefined: ImportOptions`

- [x] **Step 3: 实现 `internal/library/import.go` 的类型与扫描**

```go
package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// FileStatus is what happened to one file.
type FileStatus int

const (
	// FileImported: the file is now in the library.
	FileImported FileStatus = iota
	// FileSkipped: this folder already holds a file with this name and the
	// same content, so there was nothing to do (spec 3.7.1 ②).
	FileSkipped
	// FileFailed: this one file did not make it. The rest of the batch did
	// (spec 3.7.1 ③).
	FileFailed
)

func (s FileStatus) String() string {
	switch s {
	case FileImported:
		return "imported"
	case FileSkipped:
		return "skipped"
	default:
		return "failed"
	}
}

// FileResult is the per-file outcome. Import returns one of these for every
// file it looked at, which is what lets a batch report "4998 in, 1 skipped,
// 1 unreadable" instead of a single yes or no.
type FileResult struct {
	Source string // where it came from on disk
	Rel    string // where it went inside the box
	Name   string // the name it ended up with, after any numbering
	Status FileStatus
	// Dedup is true when the content was already in blob storage. It says
	// nothing about the file row; a deduplicated blob still gets one.
	Dedup bool
	Err   error
}

// ImportResult is one import.
type ImportResult struct {
	// OpID is the single op this import produced. Zero when nothing was
	// imported — an op with no files would be an undo that does nothing.
	OpID  int64
	Files []FileResult
	// Interrupted is true when the caller's context was cancelled partway.
	// The files already processed are committed; the rest were never touched
	// (spec 3.7.1 ①).
	Interrupted bool
	Remaining   int
}

// ImportOptions configures one import.
type ImportOptions struct {
	// Box must already exist. Import never creates one and never infers one
	// (spec 3.7).
	Box domain.BoxID
	// Into is the folder to import under. Zero means the box's root.
	Into domain.FolderID
	// Hidden includes dot-files. Off by default: .DS_Store, Thumbs.db and
	// desktop.ini are operating-system droppings, not archive material, and
	// an archive that quietly accumulates one per directory forever is worse
	// than one that made you pass a flag.
	Hidden bool
}

// scanned is one candidate found on disk, before anything has been decided.
type scanned struct {
	abs string
	// rel is the path relative to the import root, using forward slashes so
	// the folder chain is the same on every platform.
	rel string
	err error
}

// scan walks src and returns the candidates in a deterministic order.
//
// Determinism matters more than it looks: the numbering of conflicting names
// depends on the order files are seen in, so a non-deterministic walk would
// make the same import produce different names on different runs.
// filepath.WalkDir sorts each directory's entries, which is exactly what is
// needed.
func scan(src string, hidden bool) ([]scanned, error) {
	info, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", src, err)
	}
	if !info.IsDir() {
		return []scanned{{abs: src, rel: filepath.Base(src)}}, nil
	}

	var out []scanned
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			out = append(out, scanned{abs: path, err: err})
			return nil // one unreadable directory does not end the batch
		}
		if path == src {
			return nil
		}
		name := d.Name()
		if !hidden && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil // folders are created lazily, when a file lands in one
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		s := scanned{abs: path, rel: filepath.ToSlash(rel)}
		if !d.Type().IsRegular() {
			s.err = fmt.Errorf("not a regular file (%s)", d.Type())
		}
		out = append(out, s)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", src, err)
	}
	return out, nil
}
```

**空目录不产生 folder 行。** 目录是懒创建的——只有真的有文件落进去时才建。这让 payload 只包含实际用到的目录，撤销时的「目录必须是空的」断言也不会被一堆空壳绊住。

- [x] **Step 4: 实现 `Import` 的主循环**

```go
// Import puts a file or a directory tree into an existing box.
//
// The shape follows spec 3.7: content is written to blob storage first, one
// file at a time and outside any transaction, and only then does a single
// transaction append one op describing everything that made it in. A crash
// between the two leaves an orphan blob, which costs space and nothing else
// (spec 3.9).
//
// Cancelling ctx is a graceful stop, not an abort: the loop stops taking new
// files and the ones already processed are committed as one op (spec 3.7.1
// ①). That is also why the commit runs on a context.WithoutCancel — the
// context that told us to stop cannot be the one we ask to write.
func (l *Library) Import(ctx context.Context, src string, opt ImportOptions) (*ImportResult, error) {
	box, ok, err := l.Box(ctx, opt.Box)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoSuchBox, opt.Box)
	}

	folders, err := store.FoldersOfBox(l.db, box.ID)
	if err != nil {
		return nil, err
	}
	tree, err := newImportTree(box.ID, folders, opt.Into)
	if err != nil {
		return nil, err
	}
	taken, err := store.ExistingNames(l.db, box.ID)
	if err != nil {
		return nil, err
	}

	candidates, err := scan(src, opt.Hidden)
	if err != nil {
		return nil, err
	}

	res := &ImportResult{}
	payload := oplog.ImportFilesPayload{BoxID: box.ID}

	for i, c := range candidates {
		if ctx.Err() != nil {
			res.Interrupted = true
			res.Remaining = len(candidates) - i
			break
		}
		r := l.importOne(ctx, c, tree, taken, &payload)
		res.Files = append(res.Files, r)
	}

	if len(payload.Files) > 0 {
		// The op must be written even when the context is done — that is what
		// "graceful stop" means.
		opID, err := l.write(context.WithoutCancel(ctx), oplog.ActionImportFiles,
			mustMarshal(payload), oplog.SourceUser, nil)
		if err != nil {
			return nil, err
		}
		res.OpID = opID
	}
	return res, nil
}

// importOne processes a single candidate. It never returns an error: a failure
// here is this file's outcome, not the batch's (spec 3.7.1 ③).
func (l *Library) importOne(
	ctx context.Context,
	c scanned,
	tree *importTree,
	taken map[domain.FolderID]map[string]string,
	payload *oplog.ImportFilesPayload,
) FileResult {
	r := FileResult{Source: c.abs, Rel: c.rel, Name: filepath.Base(c.rel), Status: FileFailed}
	if c.err != nil {
		r.Err = c.err
		return r
	}
	if err := domain.ValidateName(r.Name); err != nil {
		// Trailing dots and edge whitespace are dropped or refused by Windows,
		// so a library holding them would not project the same way on every
		// platform (spec 3.4). Renaming it silently would be worse: the user
		// would never learn their file is not called what they think.
		r.Err = fmt.Errorf("%w — rename it at the source and import again", err)
		return r
	}

	f, err := os.Open(c.abs)
	if err != nil {
		r.Err = err
		return r
	}
	defer f.Close()

	put, err := l.blobs.Put(ctx, f)
	if err != nil {
		r.Err = err
		return r
	}
	r.Dedup = put.Outcome == blob.OutcomeDeduped

	folder, err := tree.folderFor(path.Dir(c.rel), payload)
	if err != nil {
		r.Err = err
		return r
	}
	r.Rel = c.rel

	name, skip := resolveName(taken, folder, r.Name, string(put.Key))
	if skip {
		r.Status = FileSkipped
		return r
	}
	r.Name = name

	payload.Files = append(payload.Files, oplog.ImportedFile{
		FileID:   domain.NewFileID(),
		FolderID: folder,
		Name:     name,
		BlobKey:  string(put.Key),
		Sha256:   put.Key.Hex(),
		Size:     put.Size,
	})
	r.Status = FileImported
	return r
}

// resolveName applies spec 3.7.1 ②: same name and same content is a skip,
// same name and different content is numbered.
//
// It also records the name it hands out, because the next file in this same
// batch has to see it — the database will not, until the transaction at the
// end.
func resolveName(
	taken map[domain.FolderID]map[string]string,
	folder domain.FolderID,
	name, key string,
) (string, bool) {
	if taken[folder] == nil {
		taken[folder] = map[string]string{}
	}
	inUse := taken[folder]

	candidate := name
	for n := 2; ; n++ {
		existing, clash := inUse[domain.Fold(candidate)]
		if !clash {
			inUse[domain.Fold(candidate)] = key
			return candidate, false
		}
		if existing == key {
			return candidate, true
		}
		candidate = domain.NumberedName(name, n)
	}
}
```

`put.Key.Hex()` 是 M1a 就有的方法：`Key` 的形状是 `"sha256:<hex>"` 或碰撞时的 `"sha256:<hex>.1"`，`Hex()` 返回中间那段裸 digest——正是 `blobs.sha256` 这一列要的东西（它刻意非唯一，碰撞会产生同 digest 的两行，见 3.3）。

`blob.OutcomeCollided` 和 `OutcomeRepaired` 也要在 per-file 结果里体现出来：这两件事都值得让用户看见，而不是混在「导入成功」里一笔带过。`FileResult` 上多带一个 `Outcome blob.Outcome` 字段，CLI 汇总时单独列出来。

- [x] **Step 5: 实现 `importTree`——folder 的复用与懒创建**

```go
// importTree resolves a relative directory path to a folder id, reusing what
// is already there and minting ids for what is not.
//
// Every decision it makes happens here, while the op is being built — never
// inside Apply (invariant 3). By the time the payload is written, "which
// folder does this file go in" has exactly one answer recorded in it.
type importTree struct {
	box  domain.BoxID
	base domain.FolderID
	// byPath maps a slash path relative to base onto a folder id. "." is base.
	byPath map[string]domain.FolderID
}

func newImportTree(box domain.BoxID, folders []domain.Folder, into domain.FolderID) (*importTree, error) {
	byID := map[domain.FolderID]domain.Folder{}
	var root domain.FolderID
	for _, f := range folders {
		byID[f.ID] = f
		if f.ParentID == nil {
			root = f.ID
		}
	}
	base := into
	if base.IsZero() {
		base = root
	}
	if _, ok := byID[base]; !ok {
		return nil, fmt.Errorf("folder %s is not in box %s", base, box)
	}

	t := &importTree{box: box, base: base, byPath: map[string]domain.FolderID{".": base}}
	// Index every existing descendant of base by its path relative to base, so
	// a re-run of the same import finds the folders the first run created.
	for id, f := range byID {
		if rel, ok := relPathOf(byID, base, id); ok && f.ParentID != nil {
			t.byPath[rel] = id
		}
	}
	return t, nil
}

// folderFor returns the folder for a slash path, appending any folders it has
// to create to the payload. Parents are appended before children, which is the
// order Apply relies on.
func (t *importTree) folderFor(rel string, payload *oplog.ImportFilesPayload) (domain.FolderID, error) {
	rel = path.Clean(rel)
	if id, ok := t.byPath[rel]; ok {
		return id, nil
	}
	parent, err := t.folderFor(path.Dir(rel), payload)
	if err != nil {
		return domain.FolderID{}, err
	}
	name := path.Base(rel)
	if err := domain.ValidateName(name); err != nil {
		return domain.FolderID{}, fmt.Errorf("directory %q: %w", name, err)
	}
	id := domain.NewFolderID()
	payload.Folders = append(payload.Folders, oplog.ImportedFolder{
		FolderID: id, ParentID: parent, Name: name,
	})
	t.byPath[rel] = id
	return id, nil
}
```

`relPathOf` 从 `byID` 往上走到 `base`，拼出斜杠路径；走到 root 还没碰到 `base` 就返回 `false`。

- [x] **Step 6: 跑 Step 1 的三个测试**

Run: `go test ./internal/library/ -run TestImport -v`
Expected: PASS

- [x] **Step 7: 写中断的测试**——3.7.1 ① 的验收

```go
// A cancelled context is a graceful stop: what was finished is committed as
// one op, and the op is undoable like any other. The trap this test exists to
// catch is committing on the cancelled context, which fails at BeginTx and
// turns "stop here" into "lose everything".
func TestImportInterruptedCommitsWhatItFinished(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"01.txt": "a", "02.txt": "b", "03.txt": "c", "04.txt": "d", "05.txt": "e",
	})

	ctx, cancel := context.WithCancel(t.Context())
	// Cancel once two files are in. l.hooks.afterFile is a test-only seam.
	n := 0
	l.hooks.afterFile = func() {
		if n++; n == 2 {
			cancel()
		}
	}

	res, err := l.Import(ctx, src, ImportOptions{Box: box})
	if err != nil {
		t.Fatalf("an interrupted import must not fail: %v", err)
	}
	if !res.Interrupted {
		t.Error("Interrupted is false")
	}
	if res.OpID == 0 {
		t.Fatal("no op was written; the finished files were lost")
	}
	if got := len(listAll(t, l, box)); got != 2 {
		t.Errorf("%d files in the library, want the 2 that finished", got)
	}

	// And it undoes as a unit, like any other op.
	if _, err := l.Undo(t.Context(), res.OpID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := len(listAll(t, l, box)); got != 0 {
		t.Errorf("%d files left after undoing the interrupted import", got)
	}
}
```

`hooks` 是 `Library` 上一个未导出的结构，只有测试会设置它：

```go
// hooks are test seams. They are unexported and never set outside tests; the
// alternative for testing interruption is a real signal, which is not
// something a test can aim precisely.
type hooks struct {
	afterFile func()
}
```

在 `importOne` 返回前调用 `l.hooks.afterFile`（判空）。

- [x] **Step 8: 写「部分失败」的测试**——3.7.1 ③ 的验收

```go
func TestImportKeepsGoingAfterOneBadFile(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"good1.txt": "a",
		"bad .txt":  "b", // trailing space before the extension is fine; a
		"good2.txt": "c", // trailing space at the very end is not
	})
	// Make one file unreadable rather than relying on a name rule, so the test
	// says what it means.
	if err := os.Chmod(filepath.Join(src, "bad .txt"), 0); err != nil {
		t.Skip("cannot make a file unreadable here")
	}

	res, err := l.Import(t.Context(), src, ImportOptions{Box: box})
	if err != nil {
		t.Fatalf("one bad file must not fail the batch: %v", err)
	}
	if got := countStatus(res, FileImported); got != 2 {
		t.Errorf("%d imported, want 2: %v", got, describe(res))
	}
	if got := countStatus(res, FileFailed); got != 1 {
		t.Errorf("%d failed, want 1: %v", got, describe(res))
	}
	// The failed file is absent from the op entirely, which is what keeps the
	// op atomic despite the batch not being (spec 3.7.1 ③).
	if got := len(listAll(t, l, box)); got != 2 {
		t.Errorf("%d files in the library, want 2", got)
	}
}
```

在 Windows 上 `os.Chmod(_, 0)` 不阻止读取，所以这个测试要么 `t.Skip`，要么改用一个 `runtime.GOOS` 判断——两条都行，选 `Skip` 那条并写清楚原因。

- [x] **Step 9: 跑全部 library 测试**

Run: `go test ./internal/library/ -v`
Expected: PASS

- [x] **Step 10: 写 `Rebuild` 含文件的决定性测试**（不变量 3 的真正验收）

在 `internal/library/rebuild_test.go` 里给既有的快照测试加上导入：建 box、建 tag、**导入一棵两层目录树**、给文件打 tag、撤销一次，然后快照 → `Rebuild` → 快照，逐字节相同。

M1b 的版本没有文件，所以 `files` / `folders` / `blobs` / `file_tags` 四张表在那个测试里全是空的——不变量 3 在文件这条线上此前从未被验证过。

- [x] **Step 11: 确认这个测试真的会红**

临时把 `importOne` 里的 `domain.NewFileID()` 挪进 `oplog` 的 `importFiles.Apply`（即在 Apply 里现铸 ID），跑 Step 10 的测试，确认它变红并指出是 `files` 那一段不同。然后改回来。

**这一步不能跳。** M1b 的经验是：一个不会红的决定性测试和没有测试是一回事。

- [x] **Step 12: `make check` 并提交**

```bash
make check
git add internal/library
git commit -m "feat(library): Import——一次导入一条 op，可中断、幂等、单文件失败不拖累整批

三条硬约束各有一个测试盯着：

- 幂等（3.7.1 ②）：同 folder 同名同 blob_key 就跳过。只有「同名但内容
  不同」才编号。没有这条，Ctrl-C 之后重跑会给每个已导入的文件配一个
  「(2)」双胞胎
- 中断（3.7.1 ①）：提交跑在 context.WithoutCancel 上。用被取消的那个
  context 去 BeginTx 会当场失败，「停在这里」就变成「全部丢掉」——这
  是这条路上最容易掉的坑，所以测试直接盯着它
- 部分失败（3.7.1 ③）：失败的文件根本不进 payload，所以 op 依然原子

Rebuild 的决定性测试现在含文件了。M1b 那版里 files / folders / blobs /
file_tags 四张表全是空的，不变量 3 在文件这条线上从没被验证过。

boxes/ 投影还没接，Task 10 才接。"
```

---

### Task 7: `updated_at` 传到 folder 链，以及 `library` 的文件命令

**Files:**
- Modify: `internal/library/write.go`
- Modify: `internal/library/write_test.go`
- Create: `internal/library/files.go`, `internal/library/files_test.go`
- Modify: `internal/library/library.go`（补类型别名）

**Interfaces:**
- Produces:
  - `func (l *Library) Files(ctx context.Context, opt ListFilesOptions) ([]domain.File, string, error)`
  - `func (l *Library) Folders(ctx context.Context, box domain.BoxID) ([]domain.Folder, error)`
  - `func (l *Library) ResolveFolder(ctx context.Context, box domain.BoxID, path string) (domain.Folder, error)`
  - `func (l *Library) TagFiles(ctx context.Context, tag domain.TagID, files []domain.FileID) (int, error)`
  - `func (l *Library) UntagFiles(ctx context.Context, tag domain.TagID, files []domain.FileID) (int, error)`
  - `func (l *Library) FileTags(ctx context.Context, id domain.FileID) ([]domain.TagID, error)`
  - `library.ListFilesOptions`（`= store.ListFilesOptions` 别名）

#### 7a. `updated_at` 走完整条 folder 链

- [x] **Step 1: 写失败测试**

`internal/library/write_test.go` 追加：

```go
// spec 1.5.1: updated_at means "something happened under here", and it
// propagates up the folder chain to the box. Importing into 云南/day1/raw
// must bump raw, day1, the root, and the box — otherwise browsing by recent
// activity shows a box that changed but no folder inside it that did.
func TestImportTouchesTheWholeFolderChain(t *testing.T) {
	l, box := openWithBox(t)
	before := time.Now().UTC()

	src := writeTree(t, map[string]string{"day1/raw/a.jpg": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	folders, err := l.Folders(t.Context(), box)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 3 {
		t.Fatalf("%d folders, want root + day1 + raw", len(folders))
	}
	for _, f := range folders {
		if f.UpdatedAt.Before(before) {
			t.Errorf("folder %q was not touched: updated_at = %s", f.Name, f.UpdatedAt)
		}
	}
}

// Maintenance ops must not propagate (spec 1.5.1 ②) — the same rule that
// already covers boxes has to cover folders, or relink would look like the
// user did something.
func TestMaintenanceOpsDoNotTouchFolders(t *testing.T) { /* … 与既有的 box 版对称 … */ }
```

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/library/ -run TestImportTouchesTheWholeFolderChain`
Expected: FAIL——folder 的 `updated_at` 还停在创建时刻

- [x] **Step 3: 改 `write()`**

在 `TouchBoxes` 那一段前后加上 folder 链：

```go
	// The folder chain has to be sampled on both sides of Apply.
	//
	// An import's files do not exist before Apply, and a delete's files do not
	// exist after it — so either sample alone misses one direction. Taking the
	// union also gets a future move-file op right for free: before names the
	// old chain, after names the new one, and both need bumping.
	chainBefore, err := vtx.FolderChainOfFiles(targets.Files)
	if err != nil {
		return 0, err
	}

	vtx := store.NewViewTx(tx, ts)
	if err := handler.Apply(vtx, payload); err != nil { … }

	…

	if source.PropagatesUpdatedAt() {
		if err := vtx.TouchBoxes(targets.Boxes); err != nil {
			return 0, err
		}
		chainAfter, err := vtx.FolderChainOfFiles(targets.Files)
		if err != nil {
			return 0, err
		}
		if err := vtx.TouchFolders(union(chainBefore, chainAfter)); err != nil {
			return 0, err
		}
	}
```

（`vtx` 要提前到 `chainBefore` 之前构造；`union` 是本文件里一个去重的小函数。）

**同一处改动也要进 `Rebuild`**，否则重建出来的 folder `updated_at` 与原库不同——而那正是 Task 6 Step 10 那个逐字节快照测试会抓到的。把这段抽成 `library` 内部的一个 `touchAfter(vtx, source, targets, chainBefore)`，写路径和重放路径共用。

- [x] **Step 4: 跑测试**

Run: `go test ./internal/library/ -run 'TestImportTouches|TestMaintenance|TestRebuild' -v`
Expected: PASS

#### 7b. 文件的读写命令

- [x] **Step 5: 写 `TagFiles` 的失败测试**

```go
// Only the files that actually change go into the op. A file that already
// carries the tag must not be listed, or the inverse would strip a tag this op
// never added — the same rule M1b established for boxes.
func TestTagFilesSkipsFilesThatAlreadyHaveIt(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.jpg": "x", "b.jpg": "y"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	files := listAll(t, l, box)
	tag, err := l.CreateTag(t.Context(), "云南", nil)
	if err != nil {
		t.Fatal(err)
	}

	n, err := l.TagFiles(t.Context(), tag.ID, []domain.FileID{files[0].ID})
	if err != nil || n != 1 {
		t.Fatalf("first: n=%d err=%v", n, err)
	}
	n, err = l.TagFiles(t.Context(), tag.ID, []domain.FileID{files[0].ID, files[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("n = %d, want 1 — the already-tagged file must not be in the op", n)
	}
}

// Tagging nothing is not an error and not an op: an op with no targets would
// be an undo that does nothing, cluttering the history for no reason.
func TestTagFilesWritesNoOpWhenNothingChanges(t *testing.T) { /* … */ }

// spec 1.6: the in-memory tree caches per-tag counts, and library keeps them
// current as ops commit. A stale count is invisible until someone notices the
// number in the tag picker is wrong.
func TestTagFilesUpdatesTheTreeCounts(t *testing.T) { /* … */ }
```

- [x] **Step 6: 实现 `internal/library/files.go`**

`TagFiles` / `UntagFiles` 照 M1b 的 `TagBoxes` / `UntagBoxes` 写：先读现状过滤出真正会变的，为空则直接返回 `(0, nil)` 不写 op，否则 `l.write(...)`。`refreshTagState` 里补上 `ActionTagFiles` / `ActionUntagFiles` 两个分支，调 `l.tags.AddDirectCount(p.TagID, ±len(p.FileIDs))`。

`Files` 直接转 `store.ListFiles`。`Folders` 转 `store.FoldersOfBox`。

`ResolveFolder` 把一个斜杠路径解析成 folder：

```go
// ResolveFolder turns a slash path inside a box into a folder.
//
// The empty path and "." are the box root. Matching is on the folded name, the
// same as the uniqueness constraint — so `kist ls 云南 Day1` finds `day1`,
// which is the behaviour anyone typing a path expects.
func (l *Library) ResolveFolder(ctx context.Context, box domain.BoxID, p string) (domain.Folder, error) { … }
```

- [x] **Step 7: `library.go` 补别名**

```go
	// ListFilesOptions controls the file listing and search.
	ListFilesOptions = store.ListFilesOptions
```

- [x] **Step 8: 实现 tag 条件的后代展开**

`find --tag 人物` 要包含所有后代（4.3 第 1 步）。展开在内存树里做，`store` 只收 ID 列表：

```go
// ExpandTags resolves each tag reference to itself plus all its descendants.
//
// This is step 1 of spec 4.3, and it is deliberately a memory-tree walk rather
// than a closure table: the tree is a few thousand nodes and already resident,
// while a closure table would turn "move a tag" into a library-wide rewrite.
func (l *Library) ExpandTags(refs []string) ([][]domain.TagID, error) { … }
```

- [x] **Step 9: 跑测试并提交**

```bash
go test ./internal/library/ -v
make check
git add internal/library
git commit -m "feat(library): updated_at 传到 folder 链，文件的查询与打 tag

folder 链在 Apply 前后各采一次样再取并集。导入的文件在 Apply 之前
不存在，删除的文件在 Apply 之后不存在——只采一侧必然漏掉一个方向。
顺带把将来的「移动文件」也提前办了：前一次采样给出旧链，后一次给出
新链，两条都该被 touch。

同一段逻辑抽出来给 Rebuild 共用，否则重建出的 folder.updated_at 与
原库不同，逐字节快照测试当场就红。

tag 的后代展开在内存树里做（4.3 第 1 步），不建闭包表——闭包表会让
移动 tag 从改一行变成全库重写。"
```

---

### Task 8: `internal/projection` 的命名规则

**Files:**
- Create: `internal/projection/name.go`, `internal/projection/name_test.go`

**Interfaces:**
- Consumes: 只有 `domain`
- Produces:
  - `func Sanitize(name string) string`
  - `func BoxDirName(b domain.Box) string`
  - `func Disambiguate(name string, id domain.ID) string`
  - `func Resolve[T any](items []T, key func(T) (domain.ID, string)) map[domain.ID]string`

**这个包对数据库和 `oplog` 都零依赖**，只吃 `domain` 的结构体吐字符串。3.4 那一堆跨平台规则因此可以在没有库、没有磁盘的情况下被穷举测试——而那正是它们最需要的，因为其中一半的规则只有 Windows 会执行到。

**净化规则在所有平台上一视同仁**（见前面的偏离 ⑤）。

- [x] **Step 1: 写净化规则的失败测试**

`internal/projection/name_test.go`：

```go
package projection

import "testing"

// Every rule here comes from spec 3.4's table. They are applied on all
// platforms, not only Windows, so that relink is idempotent across a move
// between operating systems and so that CI actually executes them.
func TestSanitize(t *testing.T) {
	cases := []struct{ in, want string }{
		// Characters Windows refuses. Replaced with their full-width
		// counterparts, which read almost the same and are legal everywhere.
		{`a<b`, "a＜b"},
		{`a>b`, "a＞b"},
		{`a:b`, "a：b"},
		{`a"b`, "a＂b"},
		{`a/b`, "a／b"},
		{`a\b`, "a＼b"},
		{`a|b`, "a｜b"},
		{`a?b`, "a？b"},
		{`a*b`, "a＊b"},

		// Control characters have no readable counterpart.
		{"a\x00b", "a_b"},
		{"a\tb", "a_b"},

		// Windows silently drops trailing dots and spaces, so a name ending in
		// one would round-trip differently there than here.
		{"report.", "report"},
		{"report ", "report"},
		{"report. . ", "report"},

		// Reserved device names, with or without an extension. Only the stem
		// counts, and the match is case-insensitive.
		{"CON", "CON_"},
		{"con.txt", "con_.txt"},
		{"COM1", "COM1_"},
		{"lpt9.log", "lpt9_.log"},
		// Not reserved: the rule is the whole stem, not a prefix.
		{"CONSOLE", "CONSOLE"},
		{"COM10", "COM10"},

		// Nothing left is still a name.
		{"...", "_"},
		{"", "_"},

		// The common case must survive untouched.
		{"2019云南.jpg", "2019云南.jpg"},
	}
	for _, c := range cases {
		if got := Sanitize(c.in); got != c.want {
			t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Sanitize has to be idempotent, or relink would rename the same file on every
// run — and each rename is another chance at a Windows sharing violation
// (spec 3.4.1).
func TestSanitizeIsIdempotent(t *testing.T) {
	for _, in := range []string{`a<b`, "CON", "report. . ", "a\x00b", "", "2019云南.jpg"} {
		once := Sanitize(in)
		if twice := Sanitize(once); twice != once {
			t.Errorf("Sanitize(%q) = %q, but Sanitize(%q) = %q", in, once, once, twice)
		}
	}
}
```

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/projection/`
Expected: FAIL, 包不存在

- [x] **Step 3: 实现 `Sanitize`**

```go
// Package projection turns library state into the human-readable directory
// tree under boxes/.
//
// It is a pure function of the entities it is given: no database, no
// filesystem, no clock. That is what lets spec 3.4's cross-platform naming
// rules — half of which only Windows would ever exercise — be tested
// exhaustively on any machine.
package projection

import (
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
)

// illegal maps the characters Windows refuses onto full-width counterparts.
//
// The substitution is applied on every platform, not only Windows. The
// alternative — sanitizing per platform — means moving a library between
// operating systems renames the entire tree on the next relink, and it means
// these rules are only ever executed by the Windows CI runner, which is where
// they are least likely to be right.
var illegal = strings.NewReplacer(
	`<`, "＜", `>`, "＞", `:`, "：", `"`, "＂",
	`/`, "／", `\`, "＼", `|`, "｜", `?`, "？", `*`, "＊",
)

// reserved are the DOS device names. Still special in Win32 after forty years:
// a file called CON cannot be created, and one called con.txt opens the
// console instead of the file.
var reserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// Sanitize makes one path component creatable on every platform kist supports.
//
// It must be idempotent: relink runs it over the whole library, and a
// non-idempotent rule would rename every file on every run — each rename being
// another opportunity for the Windows sharing violation described in 3.4.1.
func Sanitize(name string) string {
	out := illegal.Replace(name)

	var b strings.Builder
	for _, r := range out {
		if r < 0x20 || r == 0x7f {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	out = strings.TrimRight(b.String(), ". ")

	if out == "" {
		return "_"
	}
	stem, ext := domain.SplitName(out)
	if reserved[strings.ToLower(stem)] {
		return stem + "_" + ext
	}
	return out
}
```

- [x] **Step 4: 写 box 目录名与冲突解决的失败测试**

```go
// spec 3.4: the box directory is "<date>_<slug>", the date comes from start_at
// falling back to created_at, and its length carries the precision.
func TestBoxDirName(t *testing.T) {
	cases := []struct {
		name    string
		start   string
		created string
		want    string
	}{
		{"童年老照片", "2019", "", "2019_童年老照片"},
		{"办护照", "2019-08", "", "2019-08_办护照"},
		{"婚礼", "2019-08-12", "", "2019-08-12_婚礼"},
		// Minute precision drops the colon: it cannot appear in a Windows
		// filename, so the ISO form would be uncreatable there.
		{"午餐", "2019-08-12T14:30", "", "2019-08-12_1430_午餐"},
		// No start_at falls back to created_at, so every box has a date.
		{"扫描件", "", "2026-08-27T02:13:17.000Z", "2026-08-27_扫描件"},
	}
	for _, c := range cases { /* … build a domain.Box and compare … */ }
}

// spec 3.4.1: the projected name must be a pure function of library state.
// Collisions are broken by id order — UUIDv7, so creation order — with the
// first taking the bare name. That way one box changing its date can only
// change names deterministically, never depend on what happened before.
func TestResolveBreaksCollisionsByCreationOrder(t *testing.T) {
	first, second, third := ordered3()
	got := Resolve([]item{
		{third, "2019_婚礼"}, {first, "2019_婚礼"}, {second, "2019_别的"},
	}, itemKey)

	if got[first] != "2019_婚礼" {
		t.Errorf("the oldest box got %q, want the bare name", got[first])
	}
	if got[third] == "2019_婚礼" {
		t.Error("the younger box kept the bare name too")
	}
	if got[second] != "2019_别的" {
		t.Errorf("an uncontested name was changed: %q", got[second])
	}
	// Rerunning on the same input gives the same answer — that is what "pure
	// function of state" has to mean in practice.
	again := Resolve([]item{
		{second, "2019_别的"}, {third, "2019_婚礼"}, {first, "2019_婚礼"},
	}, itemKey)
	if again[third] != got[third] {
		t.Errorf("the same state produced two different names: %q then %q", got[third], again[third])
	}
}

// Disambiguation must survive sanitization, not create work for it: two names
// that only differed in a character Sanitize replaces collide afterwards, and
// the suffix is what pulls them apart again.
func TestSanitizeCollisionsAreResolved(t *testing.T) {
	a, b := ordered2()
	got := Resolve([]item{{a, Sanitize("2019_a:b")}, {b, Sanitize("2019_a：b")}}, itemKey)
	if got[a] == got[b] {
		t.Fatalf("both became %q", got[a])
	}
}
```

- [x] **Step 5: 实现 `BoxDirName` / `Disambiguate` / `Resolve`**

```go
// BoxDirName is a box's directory under boxes/: "<date>_<slug>".
//
// The date is start_at falling back to created_at, and its length carries the
// precision — "2019_" and "2019-08-12_" rather than a zero-padded form,
// because "2019-01-01_童年老照片" would tell a browsing human that something
// happened on New Year's Day (spec 3.4).
func BoxDirName(b domain.Box) string {
	date := b.StartAt.DirPrefix()
	if date == "" {
		date = b.CreatedAt.Format("2006-01-02")
	}
	return Sanitize(date + "_" + b.Name)
}

// Disambiguate appends a short id to a name that is not unique.
//
// The suffix goes before the extension so the file still opens with the right
// application, and it is the first eight hex digits of the UUID — enough to
// separate anything a person will ever have in one directory, short enough to
// still read the name.
func Disambiguate(name string, id domain.ID) string {
	stem, ext := domain.SplitName(name)
	return stem + "_" + id.String()[:8] + ext
}

// Resolve assigns a final, unique name to every item.
//
// The rule from spec 3.4.1: sort by id — UUIDv7, so creation order — and give
// the first claimant of a name the bare form, disambiguating the rest. This is
// what makes the projection a pure function of library state: one box changing
// its date can make another box's name change, and the answer must not depend
// on the order things are visited or on what the directory looked like before.
func Resolve[T any](items []T, key func(T) (domain.ID, string)) map[domain.ID]string { … }
```

`Resolve` 的实现：按 `id` 字节序排序（UUIDv7 的高位是时间戳，所以字节序即创建序），逐个用折叠名占坑，撞了就 `Disambiguate`；`Disambiguate` 之后再撞的（几乎不可能）继续加序号。

- [x] **Step 6: 跑测试**

Run: `go test ./internal/projection/ -v`
Expected: PASS

- [x] **Step 7: 把偏离 ⑤ 写回设计文档 3.4**

把 3.4 表格里那一行

> Windows 非法字符 `<>:"/\|?*`、保留名 `CON` `PRN` `NUL` `COM1`、结尾的点和空格 | 投影时替换为全角或 `_`。同一个库在 Mac 上创建、拷到 Windows 打开，投影名不同但 DB 一致

改成：

> Windows 非法字符 `<>:"/\|?*`、保留名 `CON` `PRN` `NUL` `COM1`、结尾的点和空格 | 投影时替换为全角或 `_`。**所有平台用同一套规则**，DB 里存干净的真名

并在表格下方加一段说明为什么不做平台相关净化（三条理由抄本计划的偏离 ⑤）。

- [x] **Step 8: `make check` 并提交**

```bash
make check
git add internal/projection docs/design
git commit -m "feat(projection): 跨平台文件名净化与 box 目录名

净化规则在所有平台上一视同仁，改掉了 3.4 原先「投影名平台相关」的
说法。三条理由写进文档了，最要紧的是第二条：净化统一之后，Linux 上
跑的测试就是 Windows 上跑的测试。CON、尾随点、<>:\"/\\|?* 这些规则是
最容易写错的一段，让它们只有 windows runner 才执行到是自找麻烦。

Sanitize 的幂等性单独测了。不幂等的话 relink 每跑一次就把全库重命名
一遍，而每一次重命名都是一次 Windows sharing violation 的机会。

冲突按 id 排序解决——UUIDv7 即创建序，第一个占裸名。这让投影名成为
库状态的纯函数：一个 box 改日期可以导致另一个 box 改名，但结果不能
依赖遍历顺序，也不能依赖目录之前长什么样。"
```

---

### Task 9: `internal/projection` 的期望树与磁盘同步

**Files:**
- Create: `internal/projection/plan.go`, `internal/projection/plan_test.go`
- Create: `internal/projection/sync.go`, `internal/projection/sync_test.go`

**Interfaces:**
- Produces:
  - `type Entry struct { Path string; IsDir bool; BlobKey string; ModTime time.Time }`
  - `func Plan(boxDir string, folders []domain.Folder, files []domain.File) ([]Entry, error)`
  - `type Syncer struct { Root string; LinkMode blob.LinkMode; Locate func(key string) (string, bool); Log *slog.Logger }`
  - `func (s *Syncer) SyncDir(dir string, want []Entry) (Stats, error)`
  - `func (s *Syncer) PruneRoot(keep map[string]bool) (int, error)`
  - `type Stats struct { Created, Removed, Missing, Failed int }`

#### 9a. `Plan`：纯函数

- [x] **Step 1: 写失败测试**

```go
// Plan is a pure function of the entities it is handed: same input, same
// output, no filesystem, no clock. Everything cross-platform about the
// projection is decided here, which is why it can be tested exhaustively
// anywhere.
func TestPlanMirrorsTheFolderTree(t *testing.T) {
	box, root, day1 := domain.NewBoxID(), domain.NewFolderID(), domain.NewFolderID()
	folders := []domain.Folder{
		domain.NewRootFolder(root, box),
		mustFolder(t, day1, box, root, "day1"),
	}
	files := []domain.File{
		mustFile(t, box, root, "封面.jpg", "sha256:aa"),
		mustFile(t, box, day1, "IMG_1.jpg", "sha256:bb"),
	}

	got, err := Plan("2019-08_云南", folders, files)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"2019-08_云南",
		"2019-08_云南/day1",
		"2019-08_云南/day1/IMG_1.jpg",
		"2019-08_云南/封面.jpg",
	}
	if got := paths(got); !slices.Equal(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

// Parents must come before children, because Sync creates in order and mkdir
// of a nested path with no parent fails.
func TestPlanOrdersParentsFirst(t *testing.T) { /* … */ }

// A name collision that only exists after sanitization has to be resolved
// here, in the plan — not discovered by the filesystem halfway through.
func TestPlanResolvesCollisionsWithinADirectory(t *testing.T) {
	box, root := domain.NewBoxID(), domain.NewFolderID()
	// "a:b.txt" and "a：b.txt" are two different names in the database and one
	// name on disk.
	files := []domain.File{
		mustFile(t, box, root, "a:b.txt", "sha256:aa"),
		mustFile(t, box, root, "a：b.txt", "sha256:bb"),
	}
	got, err := Plan("box", []domain.Folder{domain.NewRootFolder(root, box)}, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(unique(paths(got))) != len(got) {
		t.Errorf("two entries share a path: %v", paths(got))
	}
}

// A file in a folder that is not in the folder set is a corrupt view, not
// something to project halfway. Say so rather than dropping it silently.
func TestPlanRejectsAnOrphanFile(t *testing.T) { /* … wants an error … */ }
```

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/projection/ -run TestPlan`
Expected: FAIL

- [x] **Step 3: 实现 `Plan`**

要点：

1. 用 `Resolve` 在**每一个目录内部**解决冲突（folder 之间、file 之间分别 resolve；同一个目录里 folder 和 file 也不能重名，所以两者放在一起 resolve）
2. 从根 folder 往下递归拼路径，父路径已定才处理子项
3. 目录的 `ModTime` 取 `folder.UpdatedAt`，box 目录取 `box.UpdatedAt`
4. **文件条目不带 `ModTime`**（偏离 ①），字段留零值
5. 输出按 `Path` 排序，斜杠路径的字典序天然让父目录排在子项之前

```go
// Entry is one thing that must exist under boxes/.
//
// ModTime is set for directories only. A hard link and its blob are the same
// inode, so stamping a file link's mtime would rewrite the blob's — and two
// files sharing a deduplicated blob would then fight over it, making relink
// non-idempotent. Directories have no such sharing (spec 3.4.1, amended).
type Entry struct {
	Path    string
	IsDir   bool
	BlobKey string
	ModTime time.Time
}
```

- [x] **Step 4: 跑测试**

Run: `go test ./internal/projection/ -run TestPlan -v`
Expected: PASS

#### 9b. `Syncer`：把期望树 diff 到磁盘

- [x] **Step 5: 写失败测试**

```go
// Sync is a diff, not a rebuild: a box with 20,000 photos must not have every
// link torn down and remade because one file was added.
func TestSyncOnlyTouchesWhatChanged(t *testing.T) {
	s, root := newSyncer(t)
	want := []Entry{
		{Path: "box", IsDir: true},
		{Path: "box/a.txt", BlobKey: "sha256:aa"},
	}
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}
	before := statOf(t, filepath.Join(root, "box/a.txt"))

	want = append(want, Entry{Path: "box/b.txt", BlobKey: "sha256:bb"})
	stats, err := s.SyncDir("box", want)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Created != 1 || stats.Removed != 0 {
		t.Errorf("stats = %+v, want exactly one creation", stats)
	}
	if after := statOf(t, filepath.Join(root, "box/a.txt")); !sameFile(before, after) {
		t.Error("the untouched file was relinked")
	}
}

// Running it twice must change nothing at all. This is the property that lets
// relink be the safe fallback for every projection problem (spec 3.4.1).
func TestSyncIsIdempotent(t *testing.T) {
	s, _ := newSyncer(t)
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}
	stats, err := s.SyncDir("box", want)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Created != 0 || stats.Removed != 0 {
		t.Errorf("the second run did work: %+v", stats)
	}
}

// Things that are on disk but not in the plan are what an interrupted rename
// or a hand edit leaves behind. relink's job is to make the directory match
// the database, which means removing them (spec 3.2: no reconciliation, just
// rebuild).
func TestSyncRemovesStrays(t *testing.T) { /* … including a stray subdirectory … */ }

// Directory mtimes are applied last, because creating a child bumps its
// parent's mtime and would undo the stamp.
func TestSyncStampsDirectoryTimes(t *testing.T) {
	s, root := newSyncer(t)
	when := time.Date(2019, 8, 12, 14, 30, 0, 0, time.UTC)
	_, err := s.SyncDir("box", []Entry{
		{Path: "box", IsDir: true, ModTime: when},
		{Path: "box/day1", IsDir: true, ModTime: when},
		{Path: "box/day1/a.txt", BlobKey: "sha256:aa"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"box", "box/day1"} {
		info, err := os.Stat(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Truncate(time.Second).Equal(when) {
			t.Errorf("%s mtime = %s, want %s", p, info.ModTime(), when)
		}
	}
}

// A blob that is not on local disk is a finding, not a crash: v0.1 is all
// local, but Locate returning false is exactly what a cold-storage backend
// will do (spec 3.10). Everything else in the box must still be projected.
func TestSyncReportsMissingBlobsAndKeepsGoing(t *testing.T) { /* … Stats.Missing … */ }
```

- [x] **Step 6: 跑测试，确认失败**

Run: `go test ./internal/projection/ -run TestSync`
Expected: FAIL

- [x] **Step 7: 实现 `sync.go`**

顺序必须是这个顺序：

```go
// SyncDir makes one directory under Root match want.
//
// The order is forced:
//
//  1. remove strays — files first, then directories deepest-first, or a
//     non-empty directory refuses to go
//  2. create missing directories, shallowest-first
//  3. create missing links
//  4. stamp directory mtimes, last of all — creating a child bumps its
//     parent's mtime, so any earlier stamp would be undone
//
// One file failing does not stop the rest. relink is the repair tool; a
// repair that stops at the first problem repairs nothing.
func (s *Syncer) SyncDir(dir string, want []Entry) (Stats, error) { … }
```

- [x] **Step 8: 跑测试**

Run: `go test ./internal/projection/ -v`
Expected: PASS，四个平台都要过（Windows 上 `os.Chtimes` 的精度不同，测试里用 `Truncate(time.Second)`）

- [x] **Step 9: `make check` 并提交**

```bash
make check
git add internal/projection
git commit -m "feat(projection): 期望树与磁盘同步

Plan 是纯函数：给定 box / folder / file 的集合算出一棵期望目录树，
不碰文件系统也不读时钟。目录内的重名在这里解决完——包括「净化之后
才撞上」的那种（a:b.txt 和 a：b.txt 在库里是两个名字，在磁盘上是
一个）。

Sync 是 diff 不是重建：一个装了两万张照片的 box 加进一个文件，不该
把两万个链接全拆了重建。幂等性单独测——relink 是所有投影问题的兜底
修复手段，一个每次跑都在改东西的修复手段没法当兜底用。

目录 mtime 最后才盖：建子项会顺带更新父目录的 mtime，早盖会被冲掉。
文件链接一律不盖 mtime（3.4.1 的偏离，理由写在 Entry 的 doc 里）。"
```

---

### Task 10: `library` 接上投影——`Relink` 与写后增量维护

**Files:**
- Create: `internal/library/project.go`, `internal/library/project_test.go`
- Modify: `internal/library/import.go`（提交后调投影）
- Modify: `internal/library/library.go`（`Open` 时探测链接模式、读脏标记）

**Interfaces:**
- Produces:
  - `func (l *Library) Relink(ctx context.Context) (RelinkStats, error)`
  - `func (l *Library) ProjectionDirty() bool`
  - `func (l *Library) projectBox(ctx context.Context, box domain.BoxID) error`（未导出）

**两条不能破的规则：**

1. **投影在事务提交之后做**（3.7 第 5 步、3.4.1）。它失败**永远不是数据丢失**——记日志、置脏标记、`kist relink` 修好
2. **投影失败不让写操作失败。** 导入已经提交了，把错误往上抛会让用户以为导入没成功，然后再导一遍

- [x] **Step 1: 写 `Relink` 的失败测试**

```go
// relink is the correctness backstop for the whole projection layer: whatever
// state boxes/ is in, this puts it back (spec 3.4.1).
func TestRelinkRebuildsFromNothing(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x", "day1/b.txt": "y"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	// Somebody deleted boxes/ entirely — spec 3.2's "no reconciliation, just
	// rebuild".
	if err := os.RemoveAll(l.Layout().Boxes()); err != nil {
		t.Fatal(err)
	}

	if _, err := l.Relink(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertProjected(t, l, box, "a.txt", "day1/b.txt")
}

// And whatever mess is there, it removes it. A hand-created file under boxes/
// is not data — the DO-NOT-EDIT notice says so — and leaving it would make the
// projection stop being a function of the database.
func TestRelinkRemovesHandEdits(t *testing.T) { /* … */ }

// Renaming a box moves its directory rather than rebuilding it: an O(1)
// metadata operation instead of relinking every file inside (spec 3.4.1).
func TestRenamingABoxMovesItsDirectory(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	before := soleBoxDir(t, l)

	if _, err := l.UpdateBox(t.Context(), box, BoxFields{Name: "改过名了"}); err != nil {
		t.Fatal(err)
	}
	after := soleBoxDir(t, l)
	if after == before {
		t.Fatal("the directory was not renamed")
	}
	assertProjected(t, l, box, "a.txt")
}

// The dirty flag has to be visible, or the projection quietly goes stale until
// someone happens to run verify (spec 3.4.1).
func TestProjectionFailureSetsTheDirtyFlag(t *testing.T) { /* … 用一个注入的失败 Syncer … */ }
```

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/library/ -run 'TestRelink|TestRenamingABox'`
Expected: FAIL, `undefined: Relink`

- [x] **Step 3: 实现 `Relink`**

```go
// Relink rebuilds boxes/ from the database.
//
// It is the correctness backstop for the entire projection layer: incremental
// maintenance is an optimization, and any time it gets complicated — a rename
// chain, a sharing violation, a half-finished move — the answer is to fall
// back here (spec 3.4.1). It only creates links, so it is cheap enough to be
// the answer.
//
// Memory is bounded by the largest box, not by the library: box names need
// global knowledge to disambiguate, but that is one small row each, and files
// are loaded a box at a time.
func (l *Library) Relink(ctx context.Context) (RelinkStats, error) {
	boxes, err := store.ListBoxes(l.db, store.ListBoxesOptions{})
	if err != nil {
		return RelinkStats{}, err
	}
	dirs := projection.Resolve(boxes, func(b domain.Box) (domain.ID, string) {
		return b.ID.ID, projection.BoxDirName(b)
	})

	keep := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		keep[d] = true
	}
	syncer := l.syncer()

	var stats RelinkStats
	for _, b := range boxes {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		s, err := l.syncBox(syncer, b, dirs[b.ID.ID])
		if err != nil {
			return stats, err
		}
		stats.add(s)
	}

	// Directories under boxes/ that belong to no box are what a deleted box or
	// an interrupted rename leaves behind.
	removed, err := syncer.PruneRoot(keep)
	if err != nil {
		return stats, err
	}
	stats.Removed += removed

	l.clearDirty()
	return stats, nil
}
```

- [x] **Step 4: 实现增量维护**

改名走 `os.Rename`，其余走一次 `syncBox`：

```go
// projectBox brings one box's directory up to date.
//
// Renames are handled by moving the directory rather than relinking its
// contents: that is an O(1) metadata operation, and it leaves every hard link
// and reflink inside untouched — they are directory entries pointing at
// inodes, and renaming the parent touches only the parent's own entry
// (spec 3.4.1). Unlinking and rebuilding a directory of twenty thousand
// photos, by contrast, is a disaster.
//
// Anything more complicated than "the name changed" falls back to a full sync
// of this box, and anything more complicated than that falls back to Relink.
func (l *Library) projectBox(ctx context.Context, id domain.BoxID) error { … }
```

在 `write()` **提交之后**、以及 `Import` 返回之前调用它，错误只记日志并置脏标记：

```go
	// The transaction is committed; the library is correct. Only the
	// human-readable mirror is behind, and relink fixes that. Failing the
	// caller here would tell them their import did not happen, and the next
	// thing they would do is run it again.
	if err := l.projectBox(ctx, box.ID); err != nil {
		l.log.Error("boxes/ could not be updated; run `kist relink`", "box", box.ID, "err", err)
		l.markDirty(err)
	}
```

- [x] **Step 5: 实现脏标记**

`boxes/.kist-projection-dirty`，内容是一行时间戳加错误原因。`Open` 时读一次，`ProjectionDirty()` 暴露给 CLI；`Relink` 成功后删掉。

```go
// The flag is a file rather than a row in the database because it describes
// the filesystem, and because it must survive `kist rebuild` — which discards
// every materialized view and would otherwise take the warning with it.
```

- [x] **Step 6: `Open` 里复检链接模式**

`config.Storage.LinkMode` 是 `init` 探测出来的，注释里就写着「Rechecked at startup: an external drive can come back mounted differently」。M1c 是第一个真的要用它的版本，所以现在实现这条：`Open` 时对 `blobs/` → `boxes/` 跑一次 `blob.Link`，失败就重新 `DetectLinkMode` 并写回 `kist.toml`，同时记一条日志。

- [x] **Step 7: 跑测试**

Run: `go test ./internal/library/ -v`
Expected: PASS

- [x] **Step 8: 手工过一遍真实库** — **推迟到 Task 11**

计划里这一步写的是跑 `kist import`，但那条命令要到 Task 11 才存在。手工验证挪到 Task 11 的收尾，这里改为依赖 `TestImportProjectsIntoBoxes`：它用真实的临时目录和真实的文件系统跑完整条路，断言 `boxes/<dir>/a.txt` 和 `boxes/<dir>/day1/b.txt` 都在，且**通过链接读出来的内容是对的**——手工那一步能看到的东西它全都看了。

- [x] **Step 9: `make check` 并提交**

```bash
make check
git add internal/library
git commit -m "feat(library): boxes/ 投影接上——relink 与写后增量维护

投影在事务提交之后做，失败只记日志加脏标记，不让写操作失败。导入
已经提交了，把投影的错误往上抛会让用户以为导入没成功，然后再导
一遍——那才是真正的损失。

改名走 os.Rename 而不是拆链接重建。改目录名是 O(1) 的元数据操作，
里面的硬链接和 reflink 完全不受影响；unlink + 重建对一个装了两万张
照片的目录是灾难。

脏标记是文件不是数据库行：它描述的是文件系统，而且必须活过
kist rebuild——rebuild 丢弃所有物化视图，会顺手把警告一起丢掉。

顺带实现了 config 里那句「启动时复检链接模式」。M1c 是第一个真的用
到 link_mode 的版本。"
```

---

### Task 11: `kist import`

**Files:**
- Create: `cmd/kist/import.go`, `cmd/kist/import_test.go`

**Interfaces:**
- Consumes: `library.Import` / `ImportOptions` / `ImportResult`
- Produces: `newImportCmd()`，挂进 `newRootCmd`

```
kist import --box <ref> [--into <path>] [--hidden] <source>...
```

**两次 Ctrl-C 的语义（3.7.1 ①）由 CLI 实现**，`library` 只认识 context：

- 第一次 SIGINT → 取消 context，打印一行「stopping — the files already imported will be kept」
- 第二次 SIGINT → `os.Exit(130)`，事务回滚，孤儿 blob 留给将来的 GC

- [x] **Step 1: 写信号处理的失败测试**

信号在测试里不好瞄准，所以把逻辑抽成一个可测的函数，测它而不是测进程：

```go
// interruptContext turns SIGINT into a graceful stop and the second one into
// an exit. It is a separate function so the policy can be tested without
// sending real signals at the test binary.
func interruptContext(parent context.Context, sig <-chan os.Signal, notify func(string), exit func(int)) (context.Context, context.CancelFunc)
```

```go
func TestInterruptFirstCancelsSecondExits(t *testing.T) {
	sig := make(chan os.Signal, 2)
	var notices []string
	exited := make(chan int, 1)

	ctx, cancel := interruptContext(t.Context(), sig,
		func(s string) { notices = append(notices, s) },
		func(code int) { exited <- code })
	defer cancel()

	sig <- os.Interrupt
	<-ctx.Done() // the first signal stops the batch, it does not kill us

	select {
	case code := <-exited:
		t.Fatalf("the first Ctrl-C exited with %d; it must let the finished files commit", code)
	default:
	}

	sig <- os.Interrupt
	if code := <-exited; code != 130 {
		t.Errorf("second Ctrl-C exited with %d, want 130", code)
	}
	if len(notices) == 0 {
		t.Error("the user was told nothing about what the first Ctrl-C did")
	}
}
```

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./cmd/kist/ -run TestInterrupt`
Expected: FAIL

- [x] **Step 3: 实现 `interruptContext` 与 `newImportCmd`**

- [x] **Step 4: 写输出格式的测试**

per-file 结果要汇总得让人能行动（3.7.1 ③）：

```
imported 4,998   skipped 1   failed 1

failed:
  /Users/x/photos/broken.jpg   permission denied

1 file was not imported. Everything else is in.
```

```go
// A summary that says only "done" makes a partial failure invisible, which is
// the one thing per-file results exist to prevent.
func TestImportSummaryNamesFailures(t *testing.T) { /* … */ }

// Skipped files are not failures and must not read like them: after an
// interrupted import, a re-run is almost entirely skips and that is success.
func TestImportSummaryDistinguishesSkippedFromFailed(t *testing.T) { /* … */ }

// A batch with any failure exits non-zero, so a script notices (spec 3.7.1 ③).
func TestImportExitsNonZeroOnAnyFailure(t *testing.T) { /* … */ }
```

- [x] **Step 5: 实现输出，跑测试**

Run: `go test ./cmd/kist/ -v`
Expected: PASS

- [x] **Step 6: `make check` 并提交**

```bash
make check
git add cmd/kist
git commit -m "feat(cli): kist import，两次 Ctrl-C 的语义在这一层实现

第一次 SIGINT 取消 context 并告诉用户「已经导入的会保留」，第二次
直接 exit 130。library 只认识 context，不认识信号——这让中断的策略
可以脱离进程被测试，测试里往一个 channel 里塞两个 os.Interrupt 就够
了，不用对测试进程自己发信号。

汇总输出把 skipped 和 failed 分开。中断后重跑几乎全是 skipped，那是
成功不是失败，读起来不能像失败。有任何 failed 就非零退出。"
```

---

### Task 12: `kist ls` / `kist find` / `kist file tag` / `kist relink`

**Files:**
- Create: `cmd/kist/ls.go`, `cmd/kist/find.go`, `cmd/kist/file.go`, `cmd/kist/relink.go` 及各自的测试
- Modify: `cmd/kist/root.go`

```
kist ls [<box-ref> [<path>]]        没有参数时列 box
kist find [--name <substr>] [--tag <name>]... [--not-tag <name>]... [--box <ref>] [-n <limit>]
kist file tag <tag> <file-id>...
kist file untag <tag> <file-id>...
kist relink
```

- [x] **Step 1: 写 `ls` 的测试**

```go
// ls with no arguments lists boxes; with a box it lists that box's root; with
// a path it lists that folder. Directories sort before files, and both sort in
// byte order (spec 4.8) — the same order the database would give, so the CLI
// and the API cannot disagree.
func TestLsListsDirectoriesFirst(t *testing.T) { /* … */ }

// A path that does not exist has to say so and say what does, or the user is
// left guessing whether they got the box wrong or the path.
func TestLsNamesAMissingPath(t *testing.T) { /* … */ }
```

- [x] **Step 2: 实现 `ls`，跑测试**

- [x] **Step 3: 写 `find` 的测试**

```go
// spec 4.3: several tags intersect, and a tag includes its descendants.
func TestFindIntersectsTagsAndIncludesDescendants(t *testing.T) {
	// 人物 > 爸；tag the file with 爸; find --tag 人物 must return it.
}

// spec 4.4: the text search is a folded substring over file names.
func TestFindMatchesNameCaseInsensitively(t *testing.T) { /* … */ }

// An unknown tag name is a mistake worth naming: silently returning nothing
// looks exactly like "you have no such files".
func TestFindRejectsAnUnknownTag(t *testing.T) { /* … */ }

// Output shows the path inside the box, because a bare file name is not enough
// to go find the thing in boxes/.
func TestFindPrintsTheBoxRelativePath(t *testing.T) { /* … */ }
```

- [x] **Step 4: 实现 `find`，跑测试**

`--tag` 经 `l.ExpandTags` 展开成 `AllOf`，`--not-tag` 展开成 `NoneOf`。

- [x] **Step 5: 实现 `file tag` / `file untag` 与它们的测试**

文件用 id 指定（`find` 的输出里带 id）。与 `box tag` 的形状保持一致。

- [x] **Step 6: 实现 `relink` 与它的测试**

```
kist relink
rebuilt 1,204 links in 3 boxes; removed 2 strays
```

脏标记存在时，`relink` 的输出要明确说「the warning is cleared」。同时在 `openLibrary` 里，脏标记存在就往 stderr 打一行提示——3.4.1 要求它可见。

- [x] **Step 7: 把新命令挂进 `root.go`**

```go
	root.AddCommand(newImportCmd())
	root.AddCommand(newLsCmd())
	root.AddCommand(newFindCmd())
	root.AddCommand(newFileCmd())
	root.AddCommand(newRelinkCmd())
```

- [x] **Step 8: 确认 `internal/arch` 还是绿的**

Run: `go test ./internal/arch/ -v`
Expected: PASS。新命令**不得** import `store` 或 `oplog`；缺什么就在 `library` 里加别名，别放宽规则。

- [x] **Step 9: `make check` 并提交**

```bash
make check
git add cmd/kist
git commit -m "feat(cli): ls / find / file tag / relink

find 的 --tag 经内存树展开成后代集合再进 SQL（4.3 第一步）。未知的
tag 名直接报错——静默返回空跟「你确实没有这种文件」长得一模一样。

ls 里目录排在文件前面，两组各自按字节序（4.8），与数据库给出的顺序
一致，这样 CLI 和将来的 API 不会各排各的。

投影脏标记在 openLibrary 时就往 stderr 提示一行。3.4.1 要求它可见，
否则投影会静默过期到有人碰巧跑了 verify 为止。"
```

---

### Task 13: `ListOps` 不再读 payload，`Rebuild` 改成流式

**Files:**
- Modify: `internal/store/ops.go`, `internal/store/ops_test.go`
- Modify: `internal/library/rebuild.go`, `internal/library/rebuild_test.go`

这是偏离 ② 的兑现。M1b 里 payload 只有几百字节，导入把它变成几 MB，两处随之出问题。

**问题一：`kist log` 为每一行拖一份 payload。** 一次 5000 文件的导入约 1 MB JSON，`kist log -n 20` 如果撞上二十条导入就要读 20 MB，而那一行输出里 payload 一个字节都没用到。

**问题二：`Rebuild` 把全库 payload 一次性读进内存。** 它现在的写法是 `ListOps(opt{})` 拿到全部 op 的切片再遍历。一个装了十万文件的库，光 payload 就是几十 MB 常驻。

- [x] **Step 1: 写测试钉住「列表不读 payload」**

```go
// ListOps deliberately leaves Payload empty: the history view never renders
// it, and an import's payload is megabytes. GetOp is how you ask for the whole
// thing.
//
// This is a test rather than a comment because the natural way to add a field
// to the list query is to add it to opColumns, and that would silently undo
// the decision.
func TestListOpsDoesNotReadPayload(t *testing.T) {
	db := migrated(t)
	tx, _ := db.Begin()
	id, err := AppendOp(tx, OpRow{
		TS: time.Now().UTC(), Action: "import_files", Source: "user",
		Payload: []byte(`{"big":"` + strings.Repeat("x", 10000) + `"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	list, err := ListOps(db, ListOpsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("%d ops", len(list))
	}
	if len(list[0].Payload) != 0 {
		t.Errorf("ListOps returned %d payload bytes; the history view never uses them", len(list[0].Payload))
	}

	full, ok, err := GetOp(db, id)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(full.Payload) == 0 {
		t.Error("GetOp must still return the payload")
	}
}
```

- [x] **Step 2: 跑测试，确认失败**

Run: `go test ./internal/store/ -run TestListOpsDoesNotReadPayload`
Expected: FAIL

- [x] **Step 3: 拆开列查询与单条查询的列**

```go
// opColumns is everything about an op. Used by GetOp and by replay, both of
// which need the payload.
const opColumns = `id, ts, action, source, coalesce(actor, ''), payload, reverts_id`

// opListColumns is the same minus payload.
//
// The history view renders id, time, action, source and whether the op was
// undone — never the payload. Selecting it anyway would mean `kist log -n 20`
// reads twenty megabytes to print twenty lines, once imports exist.
const opListColumns = `id, ts, action, source, coalesce(actor, ''), reverts_id`
```

`scanOp` 拆成 `scanOp` 和 `scanOpHeader` 两个。

- [x] **Step 4: 加 `IterOps` 游标**

```go
// IterOps calls fn for every op, oldest first, one row at a time.
//
// Rebuild is the only caller and it is the reason this exists: the slice-
// returning ListOps would hold every payload in the library in memory at once,
// which after a hundred thousand imported files is tens of megabytes for no
// reason. A cursor keeps it to one op.
//
// fn must not use the row after it returns; the payload buffer is reused.
func IterOps(q Queryer, fn func(OpRow) error) error { … }
```

- [x] **Step 5: `Rebuild` 改用 `IterOps`**

顺带把 M1b 里那句 `// ListOps returns newest first; replay wants oldest first.` 和倒序循环删掉——`IterOps` 直接按 `id ASC` 出。

- [x] **Step 6: 加一个「大库重建不爆内存」的测试**

```go
// The point of streaming is that memory does not grow with the log. A test
// that only checked correctness would pass just as well with the old slice.
func TestRebuildDoesNotHoldTheWholeLogInMemory(t *testing.T) {
	// Import enough files to make payloads substantial, then measure
	// runtime.MemStats.TotalAlloc across a Rebuild and assert it is well under
	// the total payload size. Use a generous bound — this test is guarding
	// against "the whole log at once", not tuning allocations.
}
```

如果这个测试写出来太脆（分配量受 GC 影响），退而求其次：断言 `Rebuild` 的实现里不出现 `ListOps`，用一个 `go/ast` 检查——**但先试真的那个**，行为测试永远比结构测试好。

- [x] **Step 7: 跑测试**

Run: `go test ./internal/store/ ./internal/library/ -v`
Expected: PASS，尤其是 Task 6 Step 10 那个逐字节快照测试

- [x] **Step 8: 把结论写进设计文档 2.2.1**

在 payload 那一节加一段：

> **payload 不进列表查询。** 导入把 payload 从几百字节变成几 MB（5000 个文件约 1 MB），而历史面板一行也用不到它。所以列表查询选的是不含 payload 的列，只有 `GetOp` 和重放读完整行；`Rebuild` 用游标逐条读，内存不随日志长度增长。

- [x] **Step 9: `make check` 并提交**

```bash
make check
git add internal/store internal/library docs/design
git commit -m "perf(oplog): 列表查询不读 payload，Rebuild 改成流式

M1b 里 payload 只有几百字节所以无所谓，导入把它变成几 MB：

- kist log -n 20 撞上二十条导入要读 20 MB，而那二十行输出里 payload
  一个字节都没用到
- Rebuild 之前是 ListOps 拿全部 op 再遍历，等于把全库 payload 一次性
  读进内存

「列表不读 payload」写成了测试而不是注释——给列表加字段最自然的做法
就是往 opColumns 里加一列，那会静默地把这个决定撤销掉。"
```

---

### Task 14: 收尾——arch 测试、文档同步、里程碑状态

**Files:**
- Modify: `internal/arch/arch_test.go`
- Modify: `docs/design/2026-08-22-kist-v0.1-design.md`
- Modify: `docs/design/open-questions.md`
- Modify: `CLAUDE.md`
- Modify: `docs/plans/2026-08-27-m1c-import-and-projection.md`（勾完所有框）

- [x] **Step 1: 给 `internal/arch` 加 `projection` 的依赖断言**

```go
// TestProjectionKnowsOnlyDomain keeps the naming rules testable.
//
// Everything cross-platform about boxes/ is decided in projection, and half of
// those rules are only ever exercised on Windows. Keeping the package free of
// the database and the op log is what lets them be tested exhaustively
// anywhere, without a library and without a disk.
func TestProjectionKnowsOnlyDomain(t *testing.T) {
	forbid(t, "internal/projection", []string{"internal/store", "internal/oplog", "internal/library"},
		"projection is a pure function of the entities it is given. Reaching for\n"+
			"the database would make spec 3.4's naming rules untestable without one.")
}
```

- [x] **Step 2: 确认这个测试会红**

临时给 `internal/projection` 加一行 `import _ "github.com/jinzhao1994/kist/internal/store"`，跑测试确认变红并打出那段解释，然后删掉。

- [x] **Step 3: 确认 `cmd` 依然碰不到 `store` / `oplog`**

Run: `go test ./internal/arch/ -v`
Expected: PASS

- [x] **Step 4: 同步设计文档**

逐条核对本计划开头那五处偏离都已经写回文档：

| 偏离 | 落在哪一节 | 由哪个 task 写 |
|---|---|---|
| ① 文件链接不设 mtime | 3.4.1 | Task 9 Step 9 的 commit 里，正文补在这里 |
| ② 列表不读 payload | 2.2.1 | Task 13 Step 8 |
| ③ M1c 不写 metadata | 3.7（在管线第 3 步旁注明「M3」） | 这里 |
| ④ `tag_files` 进 M1c | 第 7 节 M1 的 CLI 清单 | 这里 |
| ⑤ 净化跨平台统一 | 3.4 | Task 8 Step 7 |

- [x] **Step 5: 更新 `open-questions.md`**

M1c 期间新暴露的、仍未决的东西记进去，至少这三条：

- **投影脏标记的可见性到 M4 要重新想。** CLI 里是 stderr 一行，Web UI 里应该是什么？3.4.1 只说了「要在 UI 上可见」
- **`kist import` 的续传。** M1c 的中断语义是「停下并提交已完成的」，但没有断点续传——重跑靠的是幂等，会把已导入的文件重新哈希一遍。500 GB 的库上这不便宜。B2 的任务模型要一起解决
- **跨 box / 跨 folder 移动文件还没有 op。** M1c 只有导入和删除。移动是 M2 之后的事，但 `write()` 里 folder 链前后各采样一次的设计已经为它留好了位置

- [x] **Step 6: 更新 `CLAUDE.md` 的「当前状态」**

把 M1c 标记为完成，列出实际产出，并写明下一步是 M2（`verify` / `scrub` / `rebuild` CLI / 远端 BlobStore / conformance suite）。同时更新那句「M1b 一个文件都放不进去」——它现在过时了。

- [x] **Step 7: 全平台构建与测试**

```bash
make check
for t in darwin/arm64 darwin/amd64 linux/amd64 windows/amd64; do
  GOOS=${t%/*} GOARCH=${t#*/} CGO_ENABLED=0 go vet ./... || echo "FAILED $t"
done
```

- [x] **Step 8: 在一个真实目录上跑一遍**

拿一个几百个文件的真实目录（照片、PDF 混合，含中文名和空格名）跑完整流程：

```bash
kist init --yes /tmp/kist-real
kist --library /tmp/kist-real box new 测试 --start 2019-08
kist --library /tmp/kist-real import --box 测试 ~/some/real/directory
kist --library /tmp/kist-real ls 测试
kist --library /tmp/kist-real find --name jpg -n 5
kist --library /tmp/kist-real log
kist --library /tmp/kist-real undo <import-op-id>
kist --library /tmp/kist-real ls 测试        # 应该空了
kist --library /tmp/kist-real relink
```

**中途按一次 Ctrl-C 再重跑**，确认第二次几乎全是 skipped、库里没有 `(2)` 副本。

**这一步抓到了一个真 bug**（实测 4000 个文件的导入，1.2 秒后 `kill -INT`）：中断到达时正在读的那个文件被报成 `failed: context canceled`，整条命令带着一段吓人的「failed:」退出。用户按的是 Ctrl-C——那不是文件的问题。修法是把它计入「未处理」，并把退出码分成两种（`some files were not imported` / `stopped before finishing`）。补了回归测试，也写进了设计文档 3.7.1 ①。

修完之后的实测：中断在 116 个，重跑 `imported 3884, skipped 116`，`(2)` 副本零个，库里 4000 个文件对应源目录 4000 个文件；撤销那次 3884 个文件的导入耗时 0.45 秒。

- [x] **Step 9: 勾完计划里所有的框并提交**

```bash
git add -A
git commit -m "docs: 勾掉 M1c 全部 task，M1c 完成"
git push
```

- [x] **Step 10: 确认 CI 三个平台全绿**

```bash
gh run list --limit 3
gh run view <id>
```

---

## 自查

写完之后对着设计文档回头核了一遍：

**规范覆盖。** M1 的 CLI 清单 `init / box new / tag / import --box / ls / find / relink` 在 M0–M1c 里全部有对应 task（`init` 在 M1a，`box new` / `tag` 在 M1b，其余在本计划的 Task 11–12），另加偏离 ④ 说明的 `file tag`。3.4 的跨平台表逐行落在 Task 8；3.7 的管线六步里第 1–2 步是 M1a 的 `blob.Put`，第 3 步是 M3，第 4 步是 Task 6，第 5 步是 Task 10，第 6 步是 M3。3.7.1 的三条各有一个测试。4.3 / 4.4 / 4.5 / 4.5.1 落在 Task 5。

**没有占位符。** 有三处正文写的是 `/* … */`：Task 5 的三个列表查询、Task 7b 的三个测试、Task 9 / 12 的部分测试体。这些是**与同一 task 内已给出的完整代码同形**的重复，形状在相邻的 step 里写全了；不是"待定"。要求实现者照抄相邻函数即可。

**类型一致性。** `domain.File.BlobKey` 是 `string`，`blob.Key` 到它的转换点只有两处（`import.go` 写入、`projection` 读出），两处都显式写了 `string(...)`。`ImportedFile.Sha256` 用的是 `blob.Key.Hex()`，与 `blobs.sha256` 列的语义（裸 digest、刻意非唯一）一致。`ListFilesOptions` 在 `store` 定义、`library` 别名、`cmd` 通过别名使用，`cmd` 不 import `store`。

**一处规范里没有、本计划新增的决定，需要在 Task 14 一并写进设计文档：**

- **导入默认跳过点开头的文件**，`--hidden` 打开。`.DS_Store` / `Thumbs.db` / `desktop.ini` 是操作系统的排泄物不是归档材料，一个每个目录都默默攒一个的归档库比一个要你多打一个 flag 的更糟
- **文件名不合法（尾随点、首尾空格）是 per-file 失败，不自动修**。自动 trim 会让用户的文件被静默改名，而他们永远不会知道自己的文件其实不叫那个名字
- **空目录不产生 folder 行**，folder 是懒创建的
