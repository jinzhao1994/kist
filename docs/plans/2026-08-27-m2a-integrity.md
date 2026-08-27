# M2a 完整性与可恢复 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `kist verify` / `kist scrub` / `kist repair` / `kist rebuild` 真的能告诉用户「你的库有没有问题、问题在哪、能不能修」，并且在能修的时候从远端副本修回来——**M2a 的产出「数据可信了」，以及那句实践建议「M2a 通过之前别删掉导入源的原始副本」到这里才解除**。

**Architecture:** 三层检查，一个新包，一条修复路径。

新包 **`internal/integrity`** 拥有「发现」的词汇表（`Finding` / `Kind` / `Severity` / `Report`）和三层检查的实现：blob 层（`stat`）、结构层（几条 SQL）、投影层（复用 `projection` 的 diff）。它读库、读盘，**从不写**——这一点由 arch 测试变成结构保证：`integrity` 不得 import `oplog`，也就永远写不出一条 op 来（设计文档 3.8「修复不写 op」）。

`library` 把三层接起来（`Verify` / `Scrub` / `Repair`），`cmd` 只负责渲染和退出码。修复的原语落在 `blob` 里：一个 `Dir.Restore(ctx, r, want Key)`，边读边哈希，对不上就丢掉——**不信任远端**是这条路径唯一要紧的性质。

`verify --deep` 是单独一条：`VACUUM INTO` 一份副本 → 在副本上 `Rebuild` → `ATTACH` 后逐表 `EXCEPT` 双向比。它把不变量 1 从「测试里验证过」变成「用户能跑的命令」。

**Tech Stack:** 不新增依赖。用到 SQLite 的 `VACUUM INTO`（3.27+）和 `ATTACH`，两者都要在 Task 6 的第一步实测确认 modernc 支持。

**Spec:** `docs/design/2026-08-22-kist-v0.1-design.md`（3.8 / 3.9 / 3.10 / 3.11 是本计划的正文，配套待决问题见 `docs/design/open-questions.md`）

**前置：** M1a + M1b + M1c 全部完成。M2a **不需要任何 migration**——它不加表也不加列。

## Global Constraints

这些是全局要求，每个 task 的验收都隐含包含它们：

- **Go 1.26.1**，版本从 `go.mod` 读
- **`CGO_ENABLED=0` 必须能构建**，四个目标平台 `darwin/arm64` `darwin/amd64` `linux/amd64` `windows/amd64`
- **不变量 1：权威数据只有 `ops` 表 + `blobs/` 目录。** `verify` 和 `scrub` 一个字节都不许写进库里；`repair` 只写 `blobs/`，不碰 `kist.db`
- **不变量 2：`library` 是唯一写入口。** `cmd` 不得 import `store` / `oplog`；新包 `internal/integrity` 不得 import `oplog` / `library`
- **不变量 3：`Apply` 必须是纯函数。** M2a 不新增任何 op，这条不变量的表面积不变
- **不变量 4：payload 是不可查询的载荷。** M2a 不读 payload，只有 `--deep` 的重放读（走 `store.IterOps`，和 `Rebuild` 同一条路）
- **时间戳一律 UTC，格式 `2006-01-02T15:04:05.000Z`**
- **排序一律字节序**（设计文档 4.8）；**可空列的排序必须显式写 `NULLS LAST`**（4.5.1、4.7）
- **语言：代码、注释、godoc 用英文；设计文档与 commit message 用中文**
- 每个 task 结束时 `make check` 必须通过

---

## 三条要一直记着的原则

### ① verify 只 `stat`，不读内容

这是 verify 和 scrub 的**全部区别**，也是 verify 能被频繁跑、能挂 cron 的唯一理由。一旦某个检查为了严谨去读文件内容，它就该搬到 scrub 里。

直接后果（设计文档 3.8 已写死）：链接指向对不对，只在 hardlink（同 inode）和 symlink（读 link target）下能免费确认；reflink 和 copy 下只查存在和大小。

### ② 什么都不删

`verify` 发现孤儿 blob → 报告。`scrub` 发现坏 blob → 报告。`repair` 只**覆盖**和**补齐**，不删任何东西。

v0.1 不回收任何东西（3.9），而一个会删文件的完整性工具，是偷偷实现了半个 gc。唯一的例外是 `blobs/tmp/` 下超过 24 小时的临时文件——那里的东西从来就不是库的内容。

### ③ 一个发现不中断整趟检查

和 `Relink` 是同一条理由：**在第一个问题处停下的检查工具，检查不出任何东西**。用户跑 verify 是因为怀疑库坏了，这时最没用的输出是「第一个坏的是它，后面不知道」。

所有检查累加 `Report`，只有真正读不下去（DB 打不开、`boxes/` 根不可读）才返回 `error`。

---

## 文件结构

| 文件 | 职责 |
|---|---|
| `internal/integrity/finding.go`（新） | `Kind` / `Severity` / `Finding` / `Report` 与稳定渲染 |
| `internal/integrity/structure.go`（新） | 结构层检查：孤儿 file、box 不一致、folder 不可达、未知 blob_key |
| `internal/integrity/blobs.go`（新） | blob 层检查：存在、大小、孤儿 |
| `internal/integrity/projection.go`（新） | 投影层检查：调 `projection` 的 diff 与链接指向 |
| `internal/store/consistency.go`（新） | 结构层用的几条 SQL |
| `internal/store/diffviews.go`（新） | `--deep` 的逐表 `EXCEPT` 比对 |
| `internal/blob/walk.go`（新） | `Dir.Walk`（扫盘）、`Dir.CleanTmp`（24 小时残留）、`Dir.Rehash` |
| `internal/blob/restore.go`（新） | `Dir.Restore`：边读边哈希，对不上就丢 |
| `internal/blob/dir.go` | 加 `OpenDir`：不建 `tmp/` 的只读构造函数 |
| `internal/projection/sync.go` | 拆出 `Diff`，`SyncDir` 变成 `Diff` + `apply`；加 `CheckLinks` |
| `internal/library/verify.go`（新） | `Verify` / `VerifyOptions`，把三层接起来；`tmp/` 清理接线 |
| `internal/library/deep.go`（新） | `verify --deep`：副本重放 + 逐表比对 |
| `internal/library/scrub.go`（新） | `Scrub`，可中断 |
| `internal/library/repair.go`（新） | `Repair`，dry-run 默认；远端解析 |
| `internal/library/rebuild.go` | 拆出 `rebuildInto(ctx, db)`，供 `--deep` 复用 |
| `internal/config/config.go` | `[[remote]]` |
| `cmd/kist/verify.go`（新） | `kist verify [--deep]` |
| `cmd/kist/scrub.go`（新） | `kist scrub` |
| `cmd/kist/repair.go`（新） | `kist repair [--yes]`、`kist remote list` |
| `cmd/kist/rebuild.go`（新） | `kist rebuild` |
| `internal/arch/arch_test.go` | 新增：`integrity` 不得 import `oplog` / `library` |

---

### Task 1: `internal/integrity` 的发现模型与报告

**Files:**
- Create: `internal/integrity/finding.go`
- Test: `internal/integrity/finding_test.go`

**Interfaces:**
- Consumes: `internal/domain`（`FileID` / `BoxID` 等）——**只有 domain**，这个文件不碰数据库也不碰磁盘
- Produces: `integrity.Kind`、`integrity.Severity`、`integrity.Finding`、`integrity.Report`、`(*Report).Add`、`(*Report).Merge`、`(*Report).Worst`、`(*Report).Write(w io.Writer)`

先做词汇表，再做检查。三层检查会各自往 `Report` 里塞东西，如果词汇表不先定死，三个文件会各自发明一套「怎么描述一个问题」，然后 CLI 要为三种形状各写一遍渲染。

- [ ] **Step 1: 写失败的测试**

`internal/integrity/finding_test.go`：

```go
package integrity

import (
	"strings"
	"testing"
)

// The report is what a worried user reads. It has to be ordered — severity
// first, then kind — so that the thing most likely to be data loss is not
// buried under two hundred lines of "this is only wasting space".
func TestReportPutsTheWorstFirst(t *testing.T) {
	var r Report
	r.Add(Finding{Kind: BlobOrphan, Detail: "sha256:aa"})
	r.Add(Finding{Kind: BlobMissing, Detail: "sha256:bb"})
	r.Add(Finding{Kind: LinkStray, Detail: "boxes/x/junk.txt"})

	var sb strings.Builder
	if err := r.Write(&sb); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	missing := strings.Index(out, "sha256:bb")
	stray := strings.Index(out, "boxes/x/junk.txt")
	orphan := strings.Index(out, "sha256:aa")
	if missing < 0 || stray < 0 || orphan < 0 {
		t.Fatalf("something is not in the report:\n%s", out)
	}
	if !(missing < stray && stray < orphan) {
		t.Errorf("wrong order — error, then warning, then info:\n%s", out)
	}
}

// Worst is what the exit code is built from. Info-level findings must not make
// verify fail: a library with orphan blobs is a normal library (spec 3.9), and
// a check that cries wolf on every run gets taken out of cron.
func TestWorstIgnoresNothingButReportsHonestly(t *testing.T) {
	var r Report
	if got := r.Worst(); got != SeverityNone {
		t.Errorf("an empty report is %v, want SeverityNone", got)
	}
	r.Add(Finding{Kind: BlobOrphan})
	if got := r.Worst(); got != SeverityInfo {
		t.Errorf("orphan blobs are %v, want SeverityInfo", got)
	}
	r.Add(Finding{Kind: LinkStray})
	if got := r.Worst(); got != SeverityWarn {
		t.Errorf("a stray link is %v, want SeverityWarn", got)
	}
	r.Add(Finding{Kind: BlobMissing})
	if got := r.Worst(); got != SeverityError {
		t.Errorf("a missing blob is %v, want SeverityError", got)
	}
}

// A library with fifty thousand missing blobs must not print fifty thousand
// lines. The count is the finding; the examples are the evidence.
func TestReportCapsTheExamples(t *testing.T) {
	var r Report
	for i := 0; i < 500; i++ {
		r.Add(Finding{Kind: BlobMissing, Detail: "blob"})
	}
	var sb strings.Builder
	if err := r.Write(&sb); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(sb.String(), "blob"); n > examplesPerKind+1 {
		t.Errorf("printed %d examples, want at most %d", n, examplesPerKind)
	}
	if !strings.Contains(sb.String(), "500") {
		t.Errorf("the total is missing from the report:\n%s", sb.String())
	}
}

// Merge is how three layers become one report. The counts must survive it —
// including the ones past the example cap, or a library with a thousand
// missing blobs would report ten.
func TestMergeKeepsTheWholeTally(t *testing.T) {
	var blobs Report
	for i := 0; i < 1000; i++ {
		blobs.Add(Finding{Kind: BlobMissing, Detail: "x"})
	}
	blobs.Note("blobs", 1000)

	var all Report
	all.Add(Finding{Kind: LinkStray, Detail: "junk"})
	all.Merge(blobs)

	if got := all.Count(BlobMissing); got != 1000 {
		t.Errorf("merged count = %d, want 1000", got)
	}
	if got := all.Total(); got != 1001 {
		t.Errorf("total = %d, want 1001", got)
	}
	if got := all.Worst(); got != SeverityError {
		t.Errorf("worst = %v, want SeverityError", got)
	}
	if got := all.Checked["blobs"]; got != 1000 {
		t.Errorf("Checked[blobs] = %d, want 1000", got)
	}
}

// A clean library must say so in as many words. "No output means fine" is a
// terrible answer to "is my archive OK?".
func TestACleanReportSaysSo(t *testing.T) {
	var r Report
	var sb strings.Builder
	if err := r.Write(&sb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "no problems") {
		t.Errorf("a clean report reads %q", sb.String())
	}
}
```

- [ ] **Step 2: 跑一遍确认它红**

```
go test ./internal/integrity/
```

Expected: FAIL，包不存在。

- [ ] **Step 3: 实现**

`internal/integrity/finding.go`：

```go
// Package integrity checks a library against itself and reports what it finds.
//
// It reads the database, reads blobs/ and reads boxes/. It never writes any of
// them, and it cannot append an op even by accident: it does not import oplog,
// and an arch test keeps it that way. That is the structural half of spec
// 3.8's "repair does not write an op" — a repair restores content that is
// already correct by definition, so there is no state change to record.
package integrity

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// examplesPerKind bounds how much evidence one kind of finding may print. A
// library with fifty thousand missing blobs has one problem, not fifty
// thousand, and a wall of output is how a report stops being read.
const examplesPerKind = 10

// Severity decides the exit code and the order things are printed in.
type Severity int

const (
	SeverityNone Severity = iota
	// SeverityInfo is "worth knowing, nothing is wrong". Orphan blobs live
	// here: v0.1 collects nothing (spec 3.9), so they are expected.
	SeverityInfo
	// SeverityWarn is "the projection is out of step". boxes/ is a projection,
	// not data — `kist relink` fixes any of these (spec 3.2).
	SeverityWarn
	// SeverityError is "content or the database is wrong". This is the one
	// that should wake somebody up.
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarn:
		return "warning"
	case SeverityError:
		return "error"
	}
	return "ok"
}

// Kind is what went wrong. Each one carries its own severity and its own
// sentence, because the sentence is the product: "your 2019 passport scan is
// damaged" is the whole point of this package (spec 3.8).
type Kind int

const (
	// ── blob layer ──────────────────────────────────────────────────────
	BlobMissing Kind = iota
	BlobWrongSize
	BlobCorrupt      // scrub only: the content no longer hashes to its own name
	BlobOrphan       // on disk, not in the blobs table
	BlobKeyMalformed // a row in the blobs table whose key is not a key

	// ── structure layer ─────────────────────────────────────────────────
	FileOrphanFolder  // files.folder_id names a folder that is gone
	FileBoxMismatch   // files.box_id disagrees with the folder's box
	FileUnknownBlob   // files.blob_key is not in the blobs table
	FolderUnreachable // not reachable from its box's root: gone parent, or a cycle
	FolderCrossBox    // a folder whose parent belongs to another box

	// ── projection layer ────────────────────────────────────────────────
	LinkMissing     // should be under boxes/, is not
	LinkStray       // under boxes/, should not be
	LinkWrongTarget // there, but not pointing at the blob it should

	// ── deep ────────────────────────────────────────────────────────────
	ViewDrift // a materialized-view row that replaying the log does not produce
)

var kindInfo = map[Kind]struct {
	severity Severity
	headline string
}{
	BlobMissing:       {SeverityError, "content is missing from blobs/"},
	BlobWrongSize:     {SeverityError, "content is the wrong size"},
	BlobCorrupt:       {SeverityError, "content no longer matches its hash"},
	BlobOrphan:        {SeverityInfo, "blobs/ holds content the database does not know about"},
	BlobKeyMalformed:  {SeverityError, "the blobs table holds a row whose key is not a valid key"},
	FileOrphanFolder:  {SeverityError, "a file is in a folder that does not exist"},
	FileBoxMismatch:   {SeverityError, "a file and its folder disagree about which box they are in"},
	FileUnknownBlob:   {SeverityError, "a file points at content the database does not know about"},
	FolderUnreachable: {SeverityError, "a folder cannot be reached from its box's root"},
	FolderCrossBox:    {SeverityError, "a folder's parent belongs to a different box"},
	LinkMissing:       {SeverityWarn, "boxes/ is missing something"},
	LinkStray:         {SeverityWarn, "boxes/ holds something that belongs to no file"},
	LinkWrongTarget:   {SeverityWarn, "a link under boxes/ points at the wrong content"},
	ViewDrift:         {SeverityError, "a table does not match what replaying the log produces"},
}

// Severity reports how bad this kind of finding is.
func (k Kind) Severity() Severity { return kindInfo[k].severity }

// Headline is the sentence printed once per kind, above the examples.
func (k Kind) Headline() string { return kindInfo[k].headline }

// Finding is one thing that is wrong. Detail identifies it in whatever way is
// most useful to a human — a path, a blob key, a file id and name.
type Finding struct {
	Kind   Kind
	Detail string
	// Fix is the command that would repair this, when there is one. Printed
	// once per kind. Empty means "nothing in kist fixes this for you".
	Fix string
}

// Report accumulates findings. The zero value is ready to use.
//
// Nothing here stops at the first problem: a check that stops early checks
// nothing, and a user runs verify precisely when they suspect the library is
// broken (same reasoning as Relink, spec 3.4.1).
type Report struct {
	counts   map[Kind]int
	examples map[Kind][]string
	fixes    map[Kind]string
	// Checked counts what was looked at, so a clean report can say how much
	// ground it covered rather than just "fine".
	Checked map[string]int
}

func (r *Report) init() {
	if r.counts == nil {
		r.counts = map[Kind]int{}
		r.examples = map[Kind][]string{}
		r.fixes = map[Kind]string{}
	}
}

func (r *Report) Add(f Finding) {
	r.init()
	r.counts[f.Kind]++
	if len(r.examples[f.Kind]) < examplesPerKind && f.Detail != "" {
		r.examples[f.Kind] = append(r.examples[f.Kind], f.Detail)
	}
	if f.Fix != "" {
		r.fixes[f.Kind] = f.Fix
	}
}

// Note records how much was checked, for the summary line.
func (r *Report) Note(what string, n int) {
	if r.Checked == nil {
		r.Checked = map[string]int{}
	}
	r.Checked[what] += n
}

// Merge folds another report in. Each layer builds its own and hands it up.
func (r *Report) Merge(o Report) {
	r.init()
	for k, n := range o.counts {
		r.counts[k] += n
		for _, d := range o.examples[k] {
			if len(r.examples[k]) >= examplesPerKind {
				break
			}
			r.examples[k] = append(r.examples[k], d)
		}
		if fix := o.fixes[k]; fix != "" {
			r.fixes[k] = fix
		}
	}
	for what, n := range o.Checked {
		r.Note(what, n)
	}
}

// Count returns how many findings of one kind there are.
func (r *Report) Count(k Kind) int { return r.counts[k] }

// Total is how many findings there are altogether.
func (r *Report) Total() int {
	n := 0
	for _, c := range r.counts {
		n += c
	}
	return n
}

// Worst is the highest severity present, and the thing the exit code is built
// from. An info-only report is a passing report: orphan blobs are a normal
// state for a v0.1 library, and a check that cries wolf gets removed from cron.
func (r *Report) Worst() Severity {
	worst := SeverityNone
	for k := range r.counts {
		if s := k.Severity(); s > worst {
			worst = s
		}
	}
	return worst
}

// Write renders the report for a person, worst first.
func (r *Report) Write(w io.Writer) error {
	var sb strings.Builder

	kinds := make([]Kind, 0, len(r.counts))
	for k := range r.counts {
		kinds = append(kinds, k)
	}
	slices.SortFunc(kinds, func(a, b Kind) int {
		if d := int(b.Severity()) - int(a.Severity()); d != 0 {
			return d
		}
		return int(a) - int(b)
	})

	for _, k := range kinds {
		n := r.counts[k]
		fmt.Fprintf(&sb, "%s: %s (%d)\n", k.Severity(), k.Headline(), n)
		for _, e := range r.examples[k] {
			fmt.Fprintf(&sb, "    %s\n", e)
		}
		if rest := n - len(r.examples[k]); rest > 0 {
			fmt.Fprintf(&sb, "    … and %d more\n", rest)
		}
		if fix := r.fixes[k]; fix != "" {
			fmt.Fprintf(&sb, "    fix: %s\n", fix)
		}
		sb.WriteString("\n")
	}

	if len(kinds) == 0 {
		sb.WriteString("no problems found\n")
	}
	if summary := r.summary(); summary != "" {
		sb.WriteString(summary + "\n")
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

func (r *Report) summary() string {
	if len(r.Checked) == 0 {
		return ""
	}
	what := make([]string, 0, len(r.Checked))
	for k := range r.Checked {
		what = append(what, k)
	}
	slices.Sort(what)
	parts := make([]string, 0, len(what))
	for _, k := range what {
		parts = append(parts, fmt.Sprintf("%d %s", r.Checked[k], k))
	}
	return "checked " + strings.Join(parts, ", ")
}
```

- [ ] **Step 4: 跑测试确认它绿**

```
go test ./internal/integrity/
```

Expected: PASS。

- [ ] **Step 5: 确认排序断言真的会咬**

把 `Write` 里的 `slices.SortFunc` 整个删掉，跑 `TestReportPutsTheWorstFirst`。map 遍历顺序随机，它可能偶尔碰巧过——所以加 `-count=20`：

```
go test ./internal/integrity/ -run TestReportPutsTheWorstFirst -count=20
```

Expected: FAIL。**看到红了再把排序放回去。** 这个项目的惯例是每条守卫都要被验证会咬（M1b / M1c 都这么做），一个从没红过的测试和没有测试是一回事。

- [ ] **Step 6: 提交**

```bash
git add internal/integrity/
git commit -m "feat(integrity): 发现模型与报告——最坏的排最前，每类只举十个例子

先定词汇表再写检查。三层检查如果各自发明一套「怎么描述一个问题」，
CLI 就要为三种形状各写一遍渲染。

两个决定写在类型里：孤儿 blob 是 SeverityInfo（v0.1 不回收任何东西，
它是正常状态，报成错误的话 verify 会被从 cron 里拿掉），每类最多印十个
例子（五万个坏 blob 是一个问题，不是五万个问题）。"
```

---

### Task 2: 结构层检查

**Files:**
- Create: `internal/store/consistency.go`, `internal/integrity/structure.go`
- Test: `internal/store/consistency_test.go`, `internal/integrity/structure_test.go`

**Interfaces:**
- Consumes: `store.Queryer`
- Produces: `store.OrphanFiles`、`store.FilesWithBoxMismatch`、`store.FilesWithUnknownBlob`、`store.UnreachableFolders`、`store.CrossBoxFolders`（各返回 `[]store.Inconsistency`），`integrity.CheckStructure(q store.Queryer) (Report, error)`

设计文档 3.8 说明了为什么加这一层：**它恰好是四条不变量被破坏时的可观测面**，而且不用重放。

`UnreachableFolders` 一条查询同时覆盖两种坏法——父目录没了、以及成环。做法是从每个 box 的根 folder 往下递归求可达集，不在可达集里的 folder 就是有问题的，不需要单独写环检测。

- [ ] **Step 1: 写失败的测试**

`internal/store/consistency_test.go`：

```go
package store

import (
	"database/sql"
	"testing"
)

// A file whose folder is gone. This cannot happen through library — the FK
// forbids it — which is exactly why verify should look: the check is for the
// day something got in another way (a hand-edited database, a restored file
// from a different point in time, a bug we have not written yet).
func TestOrphanFilesAreFound(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := insertRootFolder(t, db, box)
	key := insertBlob(t, db, "sha256:aa")
	file := insertFile(t, db, box, root, "a.txt", key)

	// Point the file at a folder that does not exist. Foreign keys are on, so
	// go around them the way a corrupted database would look.
	forceUpdate(t, db, `UPDATE files SET folder_id = randomblob(16) WHERE id = ?`, file)

	got, err := OrphanFiles(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("found %d orphans, want 1: %+v", len(got), got)
	}
	if got[0].Detail == "" {
		t.Error("the finding says nothing about which file it is")
	}
}

// A file and its folder disagreeing about the box is the corruption that
// silently splits the projection: the file is listed under one box and
// projected under another.
func TestBoxMismatchIsFound(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	other := insertBox(t, db, "other")
	root := insertRootFolder(t, db, box)
	key := insertBlob(t, db, "sha256:aa")
	file := insertFile(t, db, box, root, "a.txt", key)

	forceUpdate(t, db, `UPDATE files SET box_id = ? WHERE id = ?`, other, file)

	got, err := FilesWithBoxMismatch(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("found %d, want 1: %+v", len(got), got)
	}
}

// A folder whose parent is gone, and a folder that is its own ancestor, are
// the same finding: not reachable from the box root. One recursive query
// covers both, which is why there is no separate cycle detector.
func TestUnreachableFoldersCoverGoneParentsAndCycles(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := insertRootFolder(t, db, box)
	day1 := insertFolder(t, db, box, root, "day1")
	deeper := insertFolder(t, db, box, day1, "deeper")

	if got, err := UnreachableFolders(db); err != nil || len(got) != 0 {
		t.Fatalf("a healthy library reports %v (err %v), want none", got, err)
	}

	// A cycle: day1's parent becomes its own child.
	forceUpdate(t, db, `UPDATE folders SET parent_id = ? WHERE id = ?`, deeper, day1)

	got, err := UnreachableFolders(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("found %d unreachable folders, want 2 (both members of the cycle): %+v", len(got), got)
	}
}

func TestUnknownBlobKeyIsFound(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := insertRootFolder(t, db, box)
	key := insertBlob(t, db, "sha256:aa")
	file := insertFile(t, db, box, root, "a.txt", key)

	forceUpdate(t, db, `UPDATE files SET blob_key = 'sha256:gone' WHERE id = ?`, file)

	got, err := FilesWithUnknownBlob(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("found %d, want 1: %+v", len(got), got)
	}
}

// A healthy library must come back completely silent, or the checks are
// useless — a report that always has something in it teaches people to ignore
// it.
func TestAHealthyLibraryHasNoStructuralFindings(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := insertRootFolder(t, db, box)
	day1 := insertFolder(t, db, box, root, "day1")
	key := insertBlob(t, db, "sha256:aa")
	insertFile(t, db, box, root, "a.txt", key)
	insertFile(t, db, box, day1, "b.txt", key)

	for name, fn := range map[string]func(Queryer) ([]Inconsistency, error){
		"OrphanFiles":           OrphanFiles,
		"FilesWithBoxMismatch":  FilesWithBoxMismatch,
		"FilesWithUnknownBlob":  FilesWithUnknownBlob,
		"UnreachableFolders":    UnreachableFolders,
		"CrossBoxFolders":       CrossBoxFolders,
	} {
		got, err := fn(db)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got) != 0 {
			t.Errorf("%s found %+v in a healthy library", name, got)
		}
	}
}

// forceUpdate writes something the schema would normally refuse, the way a
// corrupted database looks from the outside. Foreign keys are deferred off for
// this one statement only.
func forceUpdate(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(`PRAGMA foreign_keys = ON`)
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}
```

> **注意 `PRAGMA foreign_keys` 和连接池。** `store.Open` 的注释已经写过这个坑：`database/sql` 会开新连接，一次 `PRAGMA` 只作用于一条连接。`migrated(t)` 用的是 `OpenMemory`，它 `SetMaxOpenConns(1)`，所以上面的写法在测试里成立。**如果这个 helper 将来被搬到文件型库上用，必须先 `db.Conn(ctx)` 钉住一条连接。** 这条要写进 `forceUpdate` 的注释里。

- [ ] **Step 2: 跑一遍确认它红**

```
go test ./internal/store/ -run 'Orphan|Mismatch|Unreachable|UnknownBlob|Healthy'
```

Expected: FAIL，函数不存在。

- [ ] **Step 3: 实现 `store` 那一侧**

`internal/store/consistency.go`：

```go
package store

import "fmt"

// Inconsistency is one row that should not exist. Detail is already
// human-readable: these queries know the names, and making the caller look
// them up again would mean a second round trip per finding.
type Inconsistency struct {
	Detail string
}

func inconsistencies(q Queryer, what, query string) ([]Inconsistency, error) {
	rows, err := q.Query(query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	defer rows.Close()

	var out []Inconsistency
	for rows.Next() {
		var detail string
		if err := rows.Scan(&detail); err != nil {
			return nil, fmt.Errorf("%s: %w", what, err)
		}
		out = append(out, Inconsistency{Detail: detail})
	}
	return out, rows.Err()
}

// OrphanFiles finds files whose folder is gone.
func OrphanFiles(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check for orphan files", `
		SELECT hex(f.id) || ' ' || f.name
		FROM files f
		LEFT JOIN folders d ON d.id = f.folder_id
		WHERE d.id IS NULL
		ORDER BY f.id`)
}

// FilesWithBoxMismatch finds files whose box_id disagrees with their folder's.
//
// files.box_id is denormalised on purpose — it is the hottest filter there is
// (spec 1.5) — and a denormalised column is a column that can drift.
func FilesWithBoxMismatch(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check file/folder box agreement", `
		SELECT hex(f.id) || ' ' || f.name
		FROM files f
		JOIN folders d ON d.id = f.folder_id
		WHERE f.box_id != d.box_id
		ORDER BY f.id`)
}

// FilesWithUnknownBlob finds files pointing at content the database has no row
// for. That is different from the content being missing from disk, which is
// the blob layer's business.
func FilesWithUnknownBlob(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check file blob keys", `
		SELECT hex(f.id) || ' ' || f.name || ' → ' || f.blob_key
		FROM files f
		LEFT JOIN blobs b ON b.key = f.blob_key
		WHERE b.key IS NULL
		ORDER BY f.id`)
}

// UnreachableFolders finds folders that cannot be walked to from their box's
// root.
//
// One query covers two different corruptions — a parent that is gone, and a
// cycle — because from the root's point of view they look the same: you never
// arrive. That is why there is no separate cycle detector; writing one would
// mean two queries that have to agree about what a healthy tree is.
func UnreachableFolders(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check folder reachability", `
		WITH RECURSIVE reachable(id) AS (
		    SELECT id FROM folders WHERE parent_id IS NULL
		  UNION
		    SELECT f.id FROM folders f JOIN reachable r ON f.parent_id = r.id
		)
		SELECT hex(d.id) || ' ' || d.name
		FROM folders d
		WHERE d.id NOT IN (SELECT id FROM reachable)
		ORDER BY d.id`)
}

// CrossBoxFolders finds folders whose parent belongs to another box. The
// projection walks the tree from each box root, so such a folder would be
// projected into a box its files do not belong to.
func CrossBoxFolders(q Queryer) ([]Inconsistency, error) {
	return inconsistencies(q, "check folder box agreement", `
		SELECT hex(d.id) || ' ' || d.name
		FROM folders d
		JOIN folders p ON p.id = d.parent_id
		WHERE d.box_id != p.box_id
		ORDER BY d.id`)
}
```

> `UNION` 而不是 `UNION ALL` 是这条查询的关键：`UNION` 去重，所以递归碰到环时会停下来；`UNION ALL` 会一直转下去直到 SQLite 报错。（`FolderChainOfFiles` 用的是同一个理由，见 `views.go`。）

- [ ] **Step 4: 跑测试确认 `store` 侧绿**

```
go test ./internal/store/ -run 'Orphan|Mismatch|Unreachable|UnknownBlob|Healthy'
```

Expected: PASS。

- [ ] **Step 5: 实现 `integrity` 那一侧**

`internal/integrity/structure.go`：

```go
package integrity

import (
	"github.com/jinzhao1994/kist/internal/store"
)

// CheckStructure looks for materialized-view rows that contradict each other.
//
// None of these can be produced by library: the schema's foreign keys and
// unique indexes forbid them. The check is for the day something arrives
// another way — a hand-edited database, a half-restored backup, a bug not yet
// written. It is spec 3.8's cheap approximation of `verify --deep`: it cannot
// find everything a replay would, but it does not have to replay.
func CheckStructure(q store.Queryer) (Report, error) {
	var r Report

	checks := []struct {
		kind Kind
		fn   func(store.Queryer) ([]store.Inconsistency, error)
	}{
		{FileOrphanFolder, store.OrphanFiles},
		{FileBoxMismatch, store.FilesWithBoxMismatch},
		{FileUnknownBlob, store.FilesWithUnknownBlob},
		{FolderUnreachable, store.UnreachableFolders},
		{FolderCrossBox, store.CrossBoxFolders},
	}
	for _, c := range checks {
		found, err := c.fn(q)
		if err != nil {
			return r, err
		}
		for _, f := range found {
			r.Add(Finding{Kind: c.kind, Detail: f.Detail, Fix: "kist rebuild"})
		}
	}
	return r, nil
}
```

`internal/integrity/structure_test.go`：

```go
package integrity

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/store"
)

// The whole layer must be silent on a healthy library. It is worth a test of
// its own: the individual queries are tested in store, but nothing else says
// "and CheckStructure runs all five of them".
func TestCheckStructureIsSilentOnAHealthyLibrary(t *testing.T) {
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}

	r, err := CheckStructure(db)
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() != 0 {
		t.Errorf("an empty library has %d structural findings", r.Total())
	}
}
```

- [ ] **Step 6: 跑全部测试**

```
go test ./internal/integrity/ ./internal/store/
```

Expected: PASS。

- [ ] **Step 7: 确认可达性查询真的会咬**

手动确认环那个测试：把 `UnreachableFolders` 的 `WHERE d.id NOT IN (…)` 改成 `WHERE 0`，跑 `TestUnreachableFoldersCoverGoneParentsAndCycles`，必须红。改回去。

再确认一次 `UNION` 那条注释是真的：把 `UNION` 改成 `UNION ALL`，跑同一个测试——它应该挂在 SQLite 的递归深度上而不是返回结果。**把观察到的实际报错记进 `UnreachableFolders` 的注释里**，这样下一个人看到 `UNION` 时知道它不是随手写的。

- [ ] **Step 8: 提交**

```bash
git add internal/store/consistency.go internal/store/consistency_test.go internal/integrity/
git commit -m "feat(integrity): 结构层的五条检查

3.8 原本只列了 blob 和链接。加结构层的理由是它恰好是四条不变量被破坏时
的可观测面——而且不用重放。

folder 的两种坏法（父目录没了、成环）合成一条查询：从 box 根往下求可达
集，不在里面的就有问题。分开写意味着两条查询要对「什么是健康的树」保持
一致，而它们迟早不一致。UNION 而非 UNION ALL 是环能停下来的原因。

这些坏法 library 一个都造不出来（外键和唯一索引挡着）。查它们是为了
手改过的库、半恢复的备份、以及还没写出来的 bug。"
```

---

### Task 3: blob 层——扫盘、重新哈希、`tmp/` 残留清理

**Files:**
- Create: `internal/blob/walk.go`, `internal/blob/walk_test.go`, `internal/integrity/blobs.go`, `internal/integrity/blobs_test.go`
- Test: 同上

**Interfaces:**
- Consumes: `blob.Key`、`blob.Dir`
- Produces: `blob.TmpGrace`、`(*Dir).Walk(fn func(Key, int64) error) error`、`(*Dir).CleanTmp(cutoff time.Time) (int, error)`、`(*Dir).Rehash(ctx, Key) (hex string, size int64, err error)`、`integrity.CheckBlobs(q store.Queryer, d *blob.Dir) (Report, error)`

三样东西，都很小，但都只能在 `blob` 里做——只有它知道 `blobs/` 的布局。

`Walk` 必须跳过 `tmp/`：那里的东西不是内容寻址的，把一个半写的临时文件当成孤儿 blob 报出来是纯噪音。

- [ ] **Step 1: 写失败的测试**

`internal/blob/walk_test.go`：

```go
package blob

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func put(t *testing.T, d *Dir, content string) Key {
	t.Helper()
	res, err := d.Put(context.Background(), strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	return res.Key
}

// Walk is what finds orphans: content on disk the database has no row for.
func TestWalkFindsEveryStoredBlob(t *testing.T) {
	d := newTestDir(t)
	want := map[Key]int64{
		put(t, d, "one"):   3,
		put(t, d, "two!!"): 5,
	}

	got := map[Key]int64{}
	if err := d.Walk(func(k Key, size int64) error {
		got[k] = size
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("walked %d blobs, want %d: %v", len(got), len(want), got)
	}
	for k, size := range want {
		if got[k] != size {
			t.Errorf("%s walked with size %d, want %d", k, got[k], size)
		}
	}
}

// tmp/ holds half-written files that are not content-addressed. Reporting one
// as an orphan blob would be pure noise, every time an import is running.
func TestWalkIgnoresTmp(t *testing.T) {
	d := newTestDir(t)
	put(t, d, "real")
	if err := os.WriteFile(filepath.Join(d.Root(), "tmp", "put-123"), []byte("half"), 0o644); err != nil {
		t.Fatal(err)
	}

	n := 0
	if err := d.Walk(func(Key, int64) error { n++; return nil }); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("walked %d entries, want 1 — tmp/ must be skipped", n)
	}
}

// A file under blobs/ whose name is not a key at all — somebody dropped
// something in there — must not stop the walk or be reported as a blob.
func TestWalkSkipsThingsThatAreNotKeys(t *testing.T) {
	d := newTestDir(t)
	real := put(t, d, "real")
	junk := filepath.Join(d.Root(), "sha256", "zz", "zz", "not-a-hash")
	if err := os.MkdirAll(filepath.Dir(junk), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(junk, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var seen []Key
	if err := d.Walk(func(k Key, _ int64) error { seen = append(seen, k); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != real {
		t.Errorf("walked %v, want just %s", seen, real)
	}
}

// Rehash is scrub's primitive: read every byte, hash it, compare.
func TestRehashDetectsAFlippedBit(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "the original content")

	hex, size, err := d.Rehash(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	if hex != k.Hex() {
		t.Fatalf("a fresh blob rehashes to %s, want %s", hex, k.Hex())
	}
	if size != int64(len("the original content")) {
		t.Errorf("size = %d", size)
	}

	corrupt(t, d, k, "the corrupted content")

	hex, _, err = d.Rehash(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	if hex == k.Hex() {
		t.Error("a corrupted blob still rehashes to its own name")
	}
}

// The tmp cleanup is not GC. It removes crash residue and nothing else
// (spec 3.9), and it must never touch a temp file that a running import is
// still writing — hence a cutoff rather than "everything in tmp/".
func TestCleanTmpRemovesOnlyOldResidue(t *testing.T) {
	d := newTestDir(t)
	tmp := filepath.Join(d.Root(), "tmp")

	fresh := filepath.Join(tmp, "put-fresh")
	stale := filepath.Join(tmp, "put-stale")
	for _, p := range []string{fresh, stale} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	n, err := d.CleanTmp(time.Now().Add(-TmpGrace))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("removed %d files, want 1", n)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the fresh temp file was removed: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("the stale temp file survived")
	}
}

// A blob directory with no tmp/ at all — a read-only remote, say — is not an
// error to clean.
func TestCleanTmpOnADirectoryWithoutTmp(t *testing.T) {
	root := t.TempDir()
	d, err := OpenDir(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := d.CleanTmp(time.Now()); err != nil || n != 0 {
		t.Errorf("CleanTmp = %d, %v; want 0, nil", n, err)
	}
}
```

`corrupt` 和 `newTestDir` 大概率已经在 `internal/blob` 的现有测试里；**先 grep，有就用，没有就照下面加进 `walk_test.go`**：

```go
// corrupt overwrites a stored blob with different content, the way bit rot or
// a bad restore would. Blobs are stored read-only, so the mode comes off first.
func corrupt(t *testing.T, d *Dir, k Key, content string) {
	t.Helper()
	p, _ := d.LocalPath(k)
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 跑一遍确认它红**

```
go test ./internal/blob/ -run 'Walk|Rehash|CleanTmp'
```

Expected: FAIL。

- [ ] **Step 3: 实现**

`internal/blob/walk.go`：

```go
package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// TmpGrace is how long a file in tmp/ is left alone before it counts as crash
// residue.
//
// It is the same 24 hours as gc's grace period, for the same reason: it has to
// be comfortably longer than the window between "the blob is written" and "the
// transaction commits" (spec 3.9). An import that takes a day still has the
// whole day of margin, because a temp file only lives for one file's copy.
const TmpGrace = 24 * time.Hour

// Walk calls fn for every blob stored under this directory, with its size on
// disk.
//
// tmp/ is skipped: what is in there is not content-addressed, and reporting a
// half-written import staging file as an unknown blob would put noise in every
// report produced while an import is running.
//
// Files whose name is not a valid key are skipped too, with a log line. This
// directory belongs to kist, but "belongs to" is not "is protected from" —
// somebody's backup tool leaving a .DS_Store in a shard should not turn into
// a finding about their archive.
func (d *Dir) Walk(fn func(k Key, size int64) error) error {
	root := d.root
	return filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if e.IsDir() {
			if p == filepath.Join(root, "tmp") {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		k, ok := keyFromRelPath(rel)
		if !ok {
			d.log.Debug("ignoring a file under blobs/ that is not a blob", "path", rel)
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		return fn(k, info.Size())
	})
}

// keyFromRelPath is the inverse of Key.RelPath, and deliberately strict: it
// rebuilds the key from the file name and then checks that the shard
// directories are the ones that key would have produced. Anything else is not
// a blob, however much it looks like one.
func keyFromRelPath(rel string) (Key, bool) {
	segs := strings.Split(filepath.ToSlash(rel), "/")
	if len(segs) != 4 {
		return "", false
	}
	algo, s1, s2, name := segs[0], segs[1], segs[2], segs[3]

	digest, suffix := name, 0
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		n, err := strconv.Atoi(name[i+1:])
		if err != nil || n <= 0 {
			return "", false
		}
		digest, suffix = name[:i], n
	}
	if len(digest) < 4 || s1 != digest[0:2] || s2 != digest[2:4] {
		return "", false
	}
	k, err := ParseKey(string(NewKey(algo, digest, suffix)))
	if err != nil {
		return "", false
	}
	return k, true
}

// Rehash reads a blob end to end and reports what it actually hashes to. This
// is scrub's whole job (spec 3.8) and the only place in verify's neighbourhood
// that reads content.
func (d *Dir) Rehash(ctx context.Context, k Key) (string, int64, error) {
	p, ok := d.LocalPath(k)
	if !ok {
		return "", 0, fmt.Errorf("blob %s has no local path", k)
	}
	f, err := os.Open(p)
	if err != nil {
		return "", 0, fmt.Errorf("read blob %s: %w", k, err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, readerContext{ctx: ctx, r: f})
	if err != nil {
		return "", 0, fmt.Errorf("read blob %s: %w", k, err)
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

// CleanTmp removes staging files last modified before cutoff, and reports how
// many went.
//
// This is not GC. v0.1 collects nothing (spec 3.9); what this removes is crash
// residue, and the cutoff is what keeps it from eating a running import's
// staging file. Callers pass time.Now().Add(-TmpGrace).
func (d *Dir) CleanTmp(cutoff time.Time) (int, error) {
	dir := filepath.Join(d.root, "tmp")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read %s: %w", dir, err)
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			// One stubborn file must not stop the rest, and this is a cleanup
			// job: nothing downstream depends on it having succeeded.
			d.log.Warn("could not remove crash residue from blobs/tmp", "name", e.Name(), "err", err)
			continue
		}
		removed++
	}
	return removed, nil
}

// readerContext makes a long read cancellable, so scrub answers Ctrl-C in the
// middle of a 4 GB video rather than at the end of it.
type readerContext struct {
	ctx context.Context
	r   io.Reader
}

func (rc readerContext) Read(p []byte) (int, error) {
	if err := rc.ctx.Err(); err != nil {
		return 0, err
	}
	return rc.r.Read(p)
}
```

> `keyFromRelPath` 走的是「重建再比对」而不是「解析」：先按 `RelPath` 的规则把 key 拼回来，再确认分片目录正是那个 key 会产生的。这样只有一份关于布局的知识（`Key.RelPath`），不会出现「写的时候两级分片、读的时候三级」这种最难查的分歧。`ParseKey` 负责校验 digest 的形状（长度、十六进制），这里不重复。

`internal/blob/dir.go` 加只读构造函数：

```go
// OpenDir opens an existing blob directory without creating anything.
//
// A repair source is the case this exists for: spec 3.8 says remotes are
// read-only as far as kist is concerned — backups are rsync's job (3.11) —
// and the most sensible way to mount a backup drive is read-only, where
// NewDir's MkdirAll of tmp/ fails before the caller gets to do anything.
//
// The returned Dir can Get, Stat, LocalPath and Walk. Put will fail, which is
// correct: nothing writes to a remote.
func OpenDir(root string, log *slog.Logger) (*Dir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", root, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("open blob directory %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("open blob directory %s: not a directory", root)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Dir{root: abs, log: log}, nil
}
```

> `NewDir` 保持原样（它照旧 `MkdirAll` 出 `tmp/`），**不要**把它改成调用 `OpenDir`——本地库那一侧「打开即可用」是它现在的契约，而只读打开恰恰不该有那个副作用。

- [ ] **Step 4: 跑测试确认绿**

```
go test ./internal/blob/
```

Expected: PASS，包括原有的全部 blob 测试。

- [ ] **Step 5: 写 `integrity.CheckBlobs`**

`internal/integrity/blobs.go`：

```go
package integrity

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/store"
)

// CheckBlobs confirms that every row in the blobs table has content on disk of
// the right size, and reports content on disk that no row knows about.
//
// It stats and does not read: that is the line between verify and scrub
// (spec 3.8). A blob of the right size whose bytes have rotted is invisible
// here by design, and is exactly what scrub is for.
func CheckBlobs(q store.Queryer, d *blob.Dir) (Report, error) {
	var r Report

	known, err := store.AllBlobs(q)
	if err != nil {
		return r, err
	}
	r.Note("blobs", len(known))

	for _, b := range known {
		key, err := blob.ParseKey(b.Key)
		if err != nil {
			r.Add(Finding{Kind: BlobKeyMalformed, Detail: b.Key, Fix: "kist rebuild"})
			continue
		}
		info, err := d.Stat(nil, key)
		if err != nil {
			r.Add(Finding{Kind: BlobMissing, Detail: b.Key, Fix: "kist repair"})
			continue
		}
		if info.Size != b.Size {
			r.Add(Finding{
				Kind:   BlobWrongSize,
				Detail: fmt.Sprintf("%s: %d bytes on disk, %d in the database", b.Key, info.Size, b.Size),
				Fix:    "kist repair",
			})
		}
	}

	onDisk := make(map[blob.Key]bool, len(known))
	if err := d.Walk(func(k blob.Key, _ int64) error {
		onDisk[k] = true
		return nil
	}); err != nil {
		return r, err
	}
	for _, b := range known {
		delete(onDisk, blob.Key(b.Key))
	}
	for k := range onDisk {
		// Not a problem: v0.1 collects nothing, so a blob written by a
		// transaction that then rolled back simply stays (spec 3.9). Saying so
		// is still better than a silently growing directory.
		r.Add(Finding{Kind: BlobOrphan, Detail: string(k)})
	}
	return r, nil
}
```

`store.AllBlobs` 加进 `internal/store/consistency.go`：

```go
// BlobRow is one row of the blobs table.
type BlobRow struct {
	Key    string
	SHA256 string
	Size   int64
}

// AllBlobs reads the whole blobs table, ordered by key.
//
// The whole table at once is deliberate: a hundred thousand blobs is about
// ten megabytes of these, and both callers (verify and scrub) need to hold a
// set anyway to find what is on disk but not in here.
func AllBlobs(q Queryer) ([]BlobRow, error) {
	rows, err := q.Query(`SELECT key, sha256, size FROM blobs ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("read blobs: %w", err)
	}
	defer rows.Close()

	var out []BlobRow
	for rows.Next() {
		var b BlobRow
		if err := rows.Scan(&b.Key, &b.SHA256, &b.Size); err != nil {
			return nil, fmt.Errorf("read blobs: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
```

> `d.Stat(nil, key)` 传 `nil` context 看着刺眼，但 `Dir.Stat` 现在的签名就是忽略它的（`func (d *Dir) Stat(_ context.Context, …)`）。**实现时改成传 `context.Background()`**，别把 `nil` 留在代码里。

- [ ] **Step 6: 写 `CheckBlobs` 的测试**

`internal/integrity/blobs_test.go`：三个 case——健康库零发现、`blobs` 表有行而盘上没文件（`BlobMissing`）、盘上有文件而表里没行（`BlobOrphan`）。**再加一个关键的负向断言**：

```go
// verify must NOT notice a blob whose bytes changed but whose length did not.
// That is not a gap, it is the contract: verify stats, scrub reads
// (spec 3.8). If this test ever goes green, somebody made verify slow.
func TestCheckBlobsDoesNotNoticeSameLengthCorruption(t *testing.T) {
	// … store "hello", overwrite the file with "world" (same 5 bytes) …
	if r.Total() != 0 {
		t.Errorf("verify read the content: %d findings", r.Total())
	}
}
```

- [ ] **Step 7: 跑全部**

```
go test ./internal/blob/ ./internal/integrity/ ./internal/store/
```

Expected: PASS。

- [ ] **Step 8: 确认 `tmp/` 的截止时间真的会咬**

把 `CleanTmp` 里的 `info.ModTime().Before(cutoff)` 改成 `true`，跑 `TestCleanTmpRemovesOnlyOldResidue`：必须红在「the fresh temp file was removed」。改回去。

这条特别值得验一次：清早了就是在删一个正在跑的导入的暂存文件，而那个 bug 在测试里不会自己暴露——它只在有并发导入时才发作。

- [ ] **Step 9: 提交**

```bash
git add internal/blob/ internal/integrity/ internal/store/consistency.go
git commit -m "feat(blob): 扫盘、重新哈希、tmp/ 残留清理；integrity 的 blob 层

Walk 跳过 tmp/——那里的东西不是内容寻址的，把一个正在写的暂存文件报成
未知 blob，等于每次有导入在跑时报告里都有噪音。名字不是合法 key 的文件
一样跳过：blobs/ 归 kist 管，但「归 kist 管」不是「别人不会往里扔东西」。

CleanTmp 收一个 cutoff 而不是「清空 tmp/」。清早了就是删掉一个正在跑的
导入的暂存文件，而那个 bug 只在有并发导入时发作，测试里不会自己暴露。
TmpGrace = 24h，和 gc 宽限期同一个数、同一个理由。

OpenDir 是不建 tmp/ 的只读构造函数。修复源最该是只读挂载的，而 NewDir
的 MkdirAll 在只读盘上第一步就失败。NewDir 保持原样：本地库「打开即可用」
是它的契约。

CheckBlobs 只 stat 不读内容，并且有一个负向测试钉住这一点——长度不变的
内容篡改 verify 必须看不见，那是 scrub 的活。"
```

---

### Task 4: `projection` 的 dry-run 与链接指向检查

**Files:**
- Modify: `internal/projection/sync.go`
- Create: `internal/projection/check.go`
- Test: `internal/projection/sync_test.go`（补）、`internal/projection/check_test.go`（新）

**Interfaces:**
- Consumes: `projection.Entry`、`blob.LinkMode`
- Produces: `projection.Diff{Create []Entry, Remove []string}`、`(*Syncer).DiffDir(dir string, want []Entry) (Diff, error)`、`(*Syncer).CheckLinks(want []Entry) ([]LinkProblem, error)`、`projection.LinkProblem{Path, Reason}`

设计文档 3.8 定了：**投影检查复用 relink 的代码，不另写检查器。** 做法是把 `SyncDir` 现在混在一起的「算差异」和「动手」拆开。

拆完之后 `SyncDir` 必须逐字保持原有行为——现有的全部 `sync_test.go` 是这次重构的验收标准，一个都不许改。

- [ ] **Step 1: 先跑一遍现有测试，记下基线**

```
go test ./internal/projection/ -v 2>&1 | tail -30
```

把通过的测试名记下来。重构之后必须一个不少、一个不改。

- [ ] **Step 2: 写新测试（先红）**

`internal/projection/check_test.go`：

```go
package projection

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/blob"
)

// DiffDir answers "what would SyncDir do" without doing it. That is verify's
// projection check (spec 3.8): the same code that repairs boxes/ is the code
// that reports on it, so the two can never disagree about what "correct" means.
func TestDiffDirReportsWithoutTouchingAnything(t *testing.T) {
	s, root := newSyncer(t)
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}

	// Break it two ways: remove something real, add something invented.
	if err := os.Remove(filepath.Join(root, "box/a.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "box/invented.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := s.DiffDir("box", want)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Create) != 1 || d.Create[0].Path != "box/a.txt" {
		t.Errorf("Create = %+v, want just box/a.txt", d.Create)
	}
	if len(d.Remove) != 1 || d.Remove[0] != "box/invented.txt" {
		t.Errorf("Remove = %+v, want just box/invented.txt", d.Remove)
	}

	// And nothing was touched: the invented file is still there, the missing
	// one is still missing. verify does not repair (spec 3.8).
	if _, err := os.Stat(filepath.Join(root, "box/invented.txt")); err != nil {
		t.Error("DiffDir removed the stray")
	}
	if _, err := os.Stat(filepath.Join(root, "box/a.txt")); err == nil {
		t.Error("DiffDir created the missing link")
	}
}

// A healthy projection diffs to nothing. Same property as relink idempotence,
// asked as a question instead of as an action.
func TestDiffDirIsEmptyForAHealthyProjection(t *testing.T) {
	s, _ := newSyncer(t)
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}
	d, err := s.DiffDir("box", want)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Create) != 0 || len(d.Remove) != 0 {
		t.Errorf("a freshly synced directory diffs to %+v", d)
	}
}

// Under hardlink, "points at the right blob" is free: the link and the blob
// are the same inode. Both paths exist here, so os.SameFile is usable — unlike
// in TestRenameMovesTheDirectory, where the old path is gone by the time it
// would be asked.
func TestCheckLinksCatchesAWrongTargetUnderHardlink(t *testing.T) {
	s, root := newSyncer(t)
	s.LinkMode = blob.LinkHardlink
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		skipIfLinksUnsupported(t, err)
	}

	problems, err := s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("a healthy projection has problems: %+v", problems)
	}

	// Replace one link with an unrelated file of the same size.
	p := filepath.Join(root, "box/a.txt")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("sha256:XX"), 0o644); err != nil {
		t.Fatal(err)
	}

	problems, err = s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || problems[0].Path != "box/a.txt" {
		t.Errorf("problems = %+v, want one about box/a.txt", problems)
	}
}

// Under copy there is no identity to check, and verify does not read content
// (spec 3.8). Size is all there is, and that has to be enough — a check that
// hashed here would have turned verify into scrub.
func TestCheckLinksUnderCopyChecksOnlySize(t *testing.T) {
	s, root := newSyncer(t)
	s.LinkMode = blob.LinkCopy
	want := treeFixture()
	if _, err := s.SyncDir("box", want); err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(root, "box/a.txt")
	original, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// Same length, different bytes: invisible, on purpose.
	same := make([]byte, len(original))
	for i := range same {
		same[i] = 'z'
	}
	if err := os.WriteFile(p, same, 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err := s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("copy mode read the content: %+v", problems)
	}

	// Different length is visible, because stat says so.
	if err := os.WriteFile(p, []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err = s.CheckLinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Errorf("a wrong-sized file was not noticed: %+v", problems)
	}
}
```

`skipIfLinksUnsupported` 照 `internal/blob` 里已有的探测惯例写：hardlink 在某些文件系统上不可用，测试要 skip 而不是红。

- [ ] **Step 3: 跑一遍确认它红**

```
go test ./internal/projection/ -run 'DiffDir|CheckLinks'
```

Expected: FAIL。

- [ ] **Step 4: 把 `SyncDir` 拆成 diff + apply**

`internal/projection/sync.go` 改成：

```go
// Diff is what one directory needs to become correct: things to make, things
// to remove. Remove is deepest-first, ready to be applied in order.
type Diff struct {
	Create []Entry
	Remove []string
}

// Empty reports whether the projection already matches.
func (d Diff) Empty() bool { return len(d.Create) == 0 && len(d.Remove) == 0 }

// DiffDir computes what SyncDir would do, without doing any of it.
//
// verify's projection check is this function (spec 3.8). Sharing the diff with
// the thing that repairs is the whole point: two implementations of "what does
// boxes/ look like when it is right" would eventually disagree, and then
// neither the report nor the repair could be trusted.
func (s *Syncer) DiffDir(dir string, want []Entry) (Diff, error) {
	desired := make(map[string]Entry, len(want))
	for _, e := range want {
		desired[e.Path] = e
	}
	actual, err := s.walk(dir)
	if err != nil {
		return Diff{}, err
	}

	var d Diff
	for p := range actual {
		if _, ok := desired[p]; !ok {
			d.Remove = append(d.Remove, p)
		}
	}
	// Deepest first, so a directory is empty by the time it goes.
	slices.Sort(d.Remove)
	slices.Reverse(d.Remove)

	// Plan order, which is parents before children.
	for _, e := range want {
		if _, exists := actual[e.Path]; !exists {
			d.Create = append(d.Create, e)
		}
	}
	return d, nil
}

// SyncDir makes one directory under Root match want.
//
// (…现有的 godoc 原样保留，包括那四步顺序的说明…)
func (s *Syncer) SyncDir(dir string, want []Entry) (Stats, error) {
	d, err := s.DiffDir(dir, want)
	if err != nil {
		return Stats{}, err
	}
	return s.apply(d, want), nil
}

// apply carries out a diff. want is passed separately because step 4 stamps
// every directory's mtime, not only the ones that were just created: a sync
// that only removed something still has to leave the times right.
func (s *Syncer) apply(d Diff, want []Entry) Stats {
	var stats Stats

	for _, p := range d.Remove {
		if err := os.RemoveAll(filepath.Join(s.Root, filepath.FromSlash(p))); err != nil {
			s.log().Error("could not remove a stray from the projection", "path", p, "err", err)
			stats.Failed++
			continue
		}
		stats.Removed++
	}

	for _, e := range d.Create {
		// (…现有 SyncDir 步骤 2、3 的循环体原样搬过来…)
	}

	for _, e := range want {
		// (…现有 SyncDir 步骤 4 的 Chtimes 循环原样搬过来…)
	}
	return stats
}
```

> **`apply` 的 mtime 循环走的是 `want` 而不是 `d.Create`，这是搬代码时最容易搞错的一处。** 现有 `SyncDir` 的第 4 步遍历的就是 `want`；改成只 stamp 新建的目录，`TestSyncIsIdempotent` 不会红（第二次没有新建），但一个「只删掉了一个杂物」的 sync 会留下被删除操作抬高了的父目录 mtime。搬完先跑 `TestSyncStampsDirectoryTimes` 和 `TestSyncIsIdempotent`。

- [ ] **Step 5: 跑现有测试，确认重构没改行为**

```
go test ./internal/projection/ -v 2>&1 | tail -30
```

Expected: Step 1 记下的测试**一个不少全部 PASS**，且 `sync_test.go` 一行没改。

- [ ] **Step 6: 实现 `CheckLinks`**

`internal/projection/check.go`：

```go
package projection

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/blob"
)

// LinkProblem is a projected file that exists but is not what it should be.
type LinkProblem struct {
	Path   string
	Reason string
}

// CheckLinks confirms that the files already under boxes/ point at the content
// they are supposed to.
//
// How much can be confirmed depends on the link mode, and spec 3.8 accepts
// that rather than papering over it:
//
//	hardlink   the link and the blob are one inode — os.SameFile settles it
//	symlink    read the link target
//	reflink    size only: a reflink is a separate inode with shared extents
//	copy       size only: there is no identity at all
//
// Under the last two, content damage is scrub's finding, not verify's. Hashing
// here would make verify read every byte in the library, and a verify that
// slow does not get run.
func (s *Syncer) CheckLinks(want []Entry) ([]LinkProblem, error) {
	var out []LinkProblem
	for _, e := range want {
		if e.IsDir || e.BlobKey == "" {
			continue
		}
		abs := filepath.Join(s.Root, filepath.FromSlash(e.Path))
		info, err := os.Lstat(abs)
		if err != nil {
			continue // missing entirely — DiffDir's finding, not this one
		}
		src, ok := s.Locate(e.BlobKey)
		if !ok {
			continue // the blob layer already reported this
		}

		switch s.LinkMode {
		case blob.LinkSymlink:
			target, err := os.Readlink(abs)
			if err != nil {
				out = append(out, LinkProblem{e.Path, fmt.Sprintf("not a symlink: %v", err)})
				continue
			}
			if target != src {
				out = append(out, LinkProblem{e.Path, fmt.Sprintf("points at %s, not %s", target, src)})
			}
		case blob.LinkHardlink:
			blobInfo, err := os.Stat(src)
			if err != nil {
				continue
			}
			if !os.SameFile(info, blobInfo) {
				out = append(out, LinkProblem{e.Path, "not the same file as its blob"})
			}
		default:
			blobInfo, err := os.Stat(src)
			if err != nil {
				continue
			}
			if info.Size() != blobInfo.Size() {
				out = append(out, LinkProblem{
					e.Path,
					fmt.Sprintf("%d bytes, but its blob is %d", info.Size(), blobInfo.Size()),
				})
			}
		}
	}
	return out, nil
}
```

- [ ] **Step 7: 跑全部**

```
go test ./internal/projection/
```

Expected: PASS。

- [ ] **Step 8: 确认 `DiffDir` 真的没有副作用**

把 `DiffDir` 里的 `for p := range actual` 循环临时改成直接 `os.RemoveAll`，跑 `TestDiffDirReportsWithoutTouchingAnything`：必须红在「DiffDir removed the stray」。改回去。

- [ ] **Step 9: 提交**

```bash
git add internal/projection/
git commit -m "refactor(projection): SyncDir 拆成 DiffDir + apply，并加链接指向检查

3.8 定的：投影检查复用 relink 的代码，不另写检查器。另写一个的代价不是
多写一遍，是两套代码迟早对同一个目录给出不同结论，那时你不知道该信谁。

拆完 SyncDir 逐字保持原行为，现有的 sync_test.go 一行没改——它就是这次
重构的验收标准。搬代码时唯一的陷阱是第 4 步的 mtime 循环走 want 而不是
新建列表：改错了幂等测试不会红（第二次没有新建），但一个只删了杂物的
sync 会留下被删除操作抬高的父目录 mtime。

CheckLinks 按 link mode 分级，hardlink 和 symlink 能免费确认指向，reflink
和 copy 只查大小。这不是遗憾是分工：在这里哈希，verify 就变成 scrub 了。"
```

---

### Task 5: `library.Verify` 与 `kist verify`

**Files:**
- Create: `internal/integrity/projection.go`, `internal/library/verify.go`, `internal/library/verify_test.go`, `cmd/kist/verify.go`
- Modify: `internal/library/import.go`（接 `tmp/` 清理）、`internal/arch/arch_test.go`
- Test: 同上

**Interfaces:**
- Consumes: Task 1–4 的全部
- Produces: `library.VerifyOptions{Deep bool}`、`(*Library).Verify(ctx, VerifyOptions) (integrity.Report, error)`、`(*Library).cleanTmp()`

三层接起来，第一个能跑的命令出来。**`--deep` 这个 task 先只留下 flag 与「尚未实现」的错误，Task 6 才填**——先把便宜的那条路走通，因为它是要被频繁跑的那个。

- [ ] **Step 1: 写 `integrity.CheckProjection`**

`internal/integrity/projection.go`：

```go
package integrity

import (
	"github.com/jinzhao1994/kist/internal/projection"
)

// CheckProjection reports how far boxes/ has drifted from the database.
//
// Every finding here is a warning, never an error: boxes/ is a projection, not
// data (spec 3.2). `kist relink` rebuilds the whole thing from the database,
// so nothing found here can lose anything.
func CheckProjection(s *projection.Syncer, dir string, want []projection.Entry) (Report, error) {
	var r Report
	r.Note("projected files", len(want))

	d, err := s.DiffDir(dir, want)
	if err != nil {
		return r, err
	}
	for _, e := range d.Create {
		r.Add(Finding{Kind: LinkMissing, Detail: e.Path, Fix: "kist relink"})
	}
	for _, p := range d.Remove {
		r.Add(Finding{Kind: LinkStray, Detail: p, Fix: "kist relink"})
	}

	problems, err := s.CheckLinks(want)
	if err != nil {
		return r, err
	}
	for _, p := range problems {
		r.Add(Finding{Kind: LinkWrongTarget, Detail: p.Path + ": " + p.Reason, Fix: "kist relink"})
	}
	return r, nil
}
```

- [ ] **Step 2: 写 `library.Verify` 的测试（先红）**

`internal/library/verify_test.go`：

```go
package library

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jinzhao1994/kist/internal/integrity"
)

// A library that was just built by import must verify clean. If this is ever
// red, either the import path or the checker is wrong, and both are worth
// stopping for.
func TestAFreshLibraryVerifiesClean(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x", "day1/b.txt": "y"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	r, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Total() != 0 {
		var sb strings.Builder
		r.Write(&sb)
		t.Errorf("a fresh library has findings:\n%s", sb.String())
	}
	if r.Checked["blobs"] != 2 {
		t.Errorf("checked %d blobs, want 2", r.Checked["blobs"])
	}
}

// The finding that matters most: your content is gone. This is the sentence
// spec 3.8 opens with — knowing in 2026 rather than in 2031.
func TestVerifyFindsMissingContent(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"passport.jpg": "scan"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	// Delete the blob out from under the library, the way a bad restore would.
	removeSomeBlob(t, l)

	r, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count(integrity.BlobMissing) != 1 {
		t.Errorf("BlobMissing = %d, want 1", r.Count(integrity.BlobMissing))
	}
	if r.Worst() != integrity.SeverityError {
		t.Errorf("worst = %v, want SeverityError", r.Worst())
	}
}

// boxes/ drifting is a warning, not an error: it is a projection, and relink
// rebuilds it from the database (spec 3.2). Reporting it as an error would
// teach people that a red verify is normal.
func TestVerifyReportsProjectionDriftAsAWarning(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(l.Layout().Boxes()); err != nil {
		t.Fatal(err)
	}

	r, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count(integrity.LinkMissing) == 0 {
		t.Error("the whole projection is gone and verify did not notice")
	}
	if r.Worst() != integrity.SeverityWarn {
		t.Errorf("worst = %v, want SeverityWarn", r.Worst())
	}
}

// verify must not repair. It is the one tool a worried user reaches for first,
// and a tool that changes things while you are trying to work out what is
// wrong is worse than no tool.
func TestVerifyChangesNothing(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(l.Layout().Boxes(), soleBoxDir(t, l), "invented.txt")
	if err := os.WriteFile(stray, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotOf(t, l)

	if _, err := l.Verify(t.Context(), VerifyOptions{}); err != nil {
		t.Fatal(err)
	}
	if after := snapshotOf(t, l); after != before {
		t.Error("verify wrote to the database")
	}
	if _, err := os.Stat(stray); err != nil {
		t.Error("verify removed the stray it was supposed to report")
	}
}
```

`snapshotOf` 用现有的 `store.Snapshot`（`Rebuild` 的测试里已经在用了，直接复用那个 helper）。`removeSomeBlob` 是新的小 helper：`filepath.WalkDir(l.Layout().Blobs()/"sha256")` 找第一个文件，`os.Chmod(0o644)` 后删掉。

- [ ] **Step 3: 跑一遍确认它红**

```
go test ./internal/library/ -run Verify
```

Expected: FAIL。

- [ ] **Step 4: 实现 `library.Verify`**

`internal/library/verify.go`：

```go
package library

import (
	"context"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/projection"
	"github.com/jinzhao1994/kist/internal/store"
)

// VerifyOptions controls how much work verify does.
type VerifyOptions struct {
	// Deep replays the whole log into a scratch copy and compares it with the
	// live materialized views. It is minutes on a large library, which is why
	// it is not the default: verify has to be cheap enough to run often
	// (spec 3.8).
	Deep bool
}

// Verify walks the library and reports what is wrong with it, without
// changing anything.
//
// Three layers, all of which run to completion: a check that stops at the
// first problem checks nothing, and a user runs verify precisely when they
// suspect the library is broken.
//
// It stats and does not read content — that line is what keeps verify cheap,
// and it is what scrub is for (spec 3.8).
func (l *Library) Verify(ctx context.Context, opt VerifyOptions) (integrity.Report, error) {
	var r integrity.Report

	// Crash residue first: it is the one thing here that is safe to remove,
	// and leaving it would put "orphan" findings next to files that are simply
	// rubbish (spec 3.9).
	l.cleanTmp()

	structural, err := integrity.CheckStructure(l.db)
	if err != nil {
		return r, err
	}
	r.Merge(structural)

	blobs, err := integrity.CheckBlobs(l.db, l.blobs)
	if err != nil {
		return r, err
	}
	r.Merge(blobs)

	proj, err := l.checkProjection(ctx)
	if err != nil {
		return r, err
	}
	r.Merge(proj)

	if opt.Deep {
		deep, err := l.verifyDeep(ctx)
		if err != nil {
			return r, err
		}
		r.Merge(deep)
	}
	return r, nil
}

// checkProjection plans every box the way Relink would and diffs it.
func (l *Library) checkProjection(ctx context.Context) (integrity.Report, error) {
	var r integrity.Report

	dirs, boxes, err := l.boxDirs()
	if err != nil {
		return r, err
	}
	s := l.syncer()
	r.Note("boxes", len(boxes))

	for _, b := range boxes {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		folders, err := store.FoldersOfBox(l.db, b.ID)
		if err != nil {
			return r, err
		}
		files, err := store.FilesOfBox(l.db, b.ID)
		if err != nil {
			return r, err
		}
		want, err := projection.Plan(dirs[b.ID], b, folders, files)
		if err != nil {
			// A box that cannot even be planned is a structural problem, and
			// the structural layer has already said so in its own words. Keep
			// going: the other boxes are still worth checking.
			l.log.Warn("could not plan a box's projection", "box", b.ID, "err", err)
			continue
		}
		one, err := integrity.CheckProjection(s, dirs[b.ID], want)
		if err != nil {
			return r, err
		}
		r.Merge(one)
	}

	// Directories under boxes/ belonging to no box.
	keep := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		keep[d] = true
	}
	strays, err := s.StrayBoxDirs(keep)
	if err != nil {
		return r, err
	}
	for _, name := range strays {
		r.Add(integrity.Finding{Kind: integrity.LinkStray, Detail: name, Fix: "kist relink"})
	}
	return r, nil
}

// cleanTmp removes crash residue from blobs/tmp.
//
// It runs at the start of verify and of import, and deliberately not in Open:
// `kist ls` must not have write side effects. A read-only command that deletes
// files is the kind of behaviour nobody thinks of when something goes wrong
// (spec 3.9).
func (l *Library) cleanTmp() {
	n, err := l.blobs.CleanTmp(time.Now().Add(-blob.TmpGrace))
	if err != nil {
		l.log.Warn("could not clean blobs/tmp", "err", err)
		return
	}
	if n > 0 {
		l.log.Info("removed crash residue from blobs/tmp", "files", n)
	}
}

// verifyDeep is Task 6.
func (l *Library) verifyDeep(context.Context) (integrity.Report, error) {
	return integrity.Report{}, fmt.Errorf("--deep is not implemented yet")
}
```

`projection.Syncer` 要长出 `StrayBoxDirs(keep map[string]bool) ([]string, error)`：和 `PruneRoot` 同一段逻辑，只报不删。**`PruneRoot` 改成调用它**，理由和 `DiffDir` 一样——只留一份关于「哪些目录不属于任何 box」的知识。

- [ ] **Step 5: `import` 开头接上 `tmp/` 清理**

`internal/library/import.go` 的 `Import` 开头加一行 `l.cleanTmp()`，并在注释里写清为什么在这里：导入是唯一会往 `tmp/` 里写的操作，也是唯一会因为崩溃在那里留下东西的操作。

- [ ] **Step 6: 跑测试**

```
go test ./internal/library/ ./internal/integrity/ ./internal/projection/
```

Expected: PASS。

- [ ] **Step 7: 写 `kist verify`**

`cmd/kist/verify.go`：

```go
package main

// newVerifyCmd builds `kist verify`.
//
// Exit codes matter here more than usual, because this is the command people
// put in cron:
//
//	0  clean, or info-level findings only (orphan blobs are normal in v0.1)
//	1  warnings — boxes/ has drifted; `kist relink` fixes it
//	2  errors — content or the database is wrong
//
// Anything above info gets a non-zero code so a cron job notices, but the two
// are kept apart: "your mirror is stale" and "your passport scan is gone" are
// not the same night.
```

flag：`--deep`。输出：`report.Write(cmd.OutOrStdout())`。退出码通过现有的 `quietError` 机制（`cmd/kist/import.go` 里已有）实现——**先读一遍它现在怎么把 error 变成退出码，照那个形状加两个哨兵 error**。

- [ ] **Step 8: 加 arch 测试**

`internal/arch/arch_test.go`：

```go
// TestIntegrityCannotWrite is the structural half of spec 3.8's "repair does
// not write an op".
//
// A repair restores content that is correct by definition — you fetched it by
// its hash and verified it — so nothing about the library's state changed and
// there is nothing to record. More to the point, Apply is a pure function that
// cannot touch blobs/ (invariant 3), so such an op would have to be a no-op on
// replay, and both `kist rebuild` and `verify --deep` would need a special case
// for it forever.
//
// Not importing oplog is how that stays true without anyone remembering it.
func TestIntegrityCannotWrite(t *testing.T) {
	forbid(t, "internal/integrity", []string{"internal/oplog", "internal/library"},
		"integrity reads and reports; it never writes. Repairing content that is\n"+
			"correct by definition is not a state change, and Apply cannot touch\n"+
			"blobs/ anyway — such an op would be a no-op on replay forever.")
}
```

- [ ] **Step 9: 确认 arch 测试会咬**

往 `internal/integrity/finding.go` 里临时加 `import _ "github.com/jinzhao1994/kist/internal/oplog"`，跑：

```
go test ./internal/arch/ -run TestIntegrityCannotWrite
```

Expected: FAIL，且错误信息里带上面那段解释。删掉 import。

- [ ] **Step 10: 手动走查**

```bash
kist init /tmp/m2a && cd /tmp/m2a
kist box new "云南" --start 2019-08-12
kist import --box 云南 ~/some/photos
kist verify              # 期望：no problems found，checked N blobs, M boxes
echo $?                  # 期望：0

rm -rf boxes/
kist verify              # 期望：warning: boxes/ is missing something
echo $?                  # 期望：1
kist relink && kist verify && echo $?   # 期望：干净，0

# 删掉一个 blob
find blobs/sha256 -type f | head -1 | xargs chmod 644
find blobs/sha256 -type f | head -1 | xargs rm
kist verify              # 期望：error: content is missing from blobs/，并给出 fix: kist repair
echo $?                  # 期望：2
```

把实际输出贴进本步骤下面。**特别确认那句 error 读起来像一句人话**——3.8 说这个命令的第一价值是「精确告诉用户哪些文件坏了」，而现在报的是 blob key，不是文件名。**如果读下来觉得 blob key 没用，就在这里把 `CheckBlobs` 改成同时报出引用它的文件名**（`SELECT name FROM files WHERE blob_key = ?`，每个坏 blob 一次，坏 blob 本来就该是少数）。这是本 task 唯一预留的产品判断。

- [ ] **Step 11: 提交**

```bash
git add internal/library/verify.go internal/library/verify_test.go internal/integrity/projection.go \
        internal/library/import.go internal/projection/ cmd/kist/verify.go internal/arch/arch_test.go
git commit -m "feat(verify): 三层检查接起来，kist verify 能跑了

退出码分三档：0 干净或只有 info（孤儿 blob 在 v0.1 是正常状态），1 警告
（boxes/ 漂了，relink 能修），2 错误（内容或库不对）。分开是因为「你的
镜像旧了」和「你的护照扫描件没了」不是同一个夜晚。

tmp/ 清理接在 verify 和 import 开头，不接在 Open 里——kist ls 不该有写
副作用（3.9）。

新增 arch 测试：integrity 不得 import oplog。这是 3.8「修复不写 op」的
结构保证——按哈希取回并验过的内容不构成状态变更，而 Apply 是纯函数、
碰不到 blobs/，那条 op 只能是一条重放时什么都不做的 op，rebuild 和
verify --deep 就要为它永远留一个特例。"
```

---

### Task 6: `verify --deep`——重放到副本，再逐表比对

**Files:**
- Create: `internal/library/deep.go`, `internal/library/deep_test.go`, `internal/store/diffviews.go`, `internal/store/diffviews_test.go`
- Modify: `internal/library/rebuild.go`（拆出 `rebuildInto`）

**Interfaces:**
- Consumes: `store.IterOps`、`oplog.Lookup`、`store` 的 `snapshotTables`
- Produces: `store.DiffViews(ctx, live *sql.DB, rebuiltPath string) ([]store.Drift, error)`、`library.rebuildInto(ctx, db *sql.DB) error`

不变量 1 说「`kist.db` 里除 `ops` 外每张表都可重建」。到 M1c 为止这条只在测试里被验证过。这个 task 把同一个断言变成用户能跑的命令。

- [ ] **Step 1: 先实测 `VACUUM INTO` 和 `ATTACH` 在 modernc 上可用**

这是个探针，不是实现。写一个临时的 `TestSpikeVacuumIntoAndAttach`，跑通之后**保留它**（改名 `TestVacuumIntoAndAttachWork`，加注释说明它守的是什么）：

```go
func TestVacuumIntoAndAttachWork(t *testing.T) {
	// This is not a test of our code. It is a test of the assumption the whole
	// --deep check rests on: that modernc.org/sqlite supports VACUUM INTO
	// (SQLite 3.27+) and ATTACH with a bound parameter. If a dependency bump
	// ever takes either away, the failure should be this line and not a
	// mystery inside verify.
	db := migrated(t)
	insertBox(t, db, "box")

	dst := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, dst); err != nil {
		t.Fatalf("VACUUM INTO: %v", err)
	}

	// ATTACH is per-connection, and database/sql pools connections — the same
	// trap store.Open documents for PRAGMA. Pin one.
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(t.Context(), `ATTACH DATABASE ? AS copy`, dst); err != nil {
		t.Fatalf("ATTACH: %v", err)
	}
	var n int
	if err := conn.QueryRowContext(t.Context(), `SELECT count(*) FROM copy.boxes`).Scan(&n); err != nil {
		t.Fatalf("query the attached database: %v", err)
	}
	if n != 1 {
		t.Errorf("the copy has %d boxes, want 1", n)
	}
}
```

```
go test ./internal/store/ -run VacuumIntoAndAttach -v
```

**如果这一步失败，整个 task 的方案要换**（退路：把副本重放的结果一表一表读回 Go 里比对，慢且吃内存，但可行）。先跑，再往下写。

- [ ] **Step 2: 拆出 `rebuildInto`**

`internal/library/rebuild.go` 现在的 `Rebuild` 里，从 `BeginTx` 到 `Commit` 那一整段与 `l` 只有一个关系：用了 `l.db`。抽出来：

```go
// rebuildInto discards db's materialized views and replays its op log into
// them. db must already hold the ops — it is either the live library or a
// VACUUM INTO copy of it.
//
// It is a package-level function rather than a method because verify --deep
// runs it against a database that is deliberately not this Library's: the
// point is to produce a second, independent answer and compare.
func rebuildInto(ctx context.Context, db *sql.DB) error {
	// (…现有 Rebuild 的正文，l.db 换成 db…)
}

// Rebuild discards every materialized view and replays the log.
//
// (…现有 godoc 原样保留…)
func (l *Library) Rebuild(ctx context.Context) error {
	if err := rebuildInto(ctx, l.db); err != nil {
		return err
	}
	tags, err := LoadTagTree(l.db)
	if err != nil {
		return fmt.Errorf("reload the tag tree after rebuild: %w", err)
	}
	l.tags = tags
	return nil
}
```

跑 `go test ./internal/library/ -run Rebuild`：**现有的全部 rebuild 测试（含不变量 3 的逐字节快照测试）必须原样通过**，这是重构没走样的证据。

- [ ] **Step 3: 写 `store.DiffViews` 的测试（先红）**

`internal/store/diffviews_test.go`：

```go
package store

import (
	"path/filepath"
	"testing"
)

// Two identical databases drift by nothing.
func TestDiffViewsFindsNothingBetweenIdenticalCopies(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := insertRootFolder(t, db, box)
	key := insertBlob(t, db, "sha256:aa")
	insertFile(t, db, box, root, "a.txt", key)

	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, copyPath); err != nil {
		t.Fatal(err)
	}

	drift, err := DiffViews(t.Context(), db, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Errorf("identical databases drifted: %+v", drift)
	}
}

// One changed cell must be found, named, and shown — "the files table differs"
// is not an answer anyone can act on.
func TestDiffViewsNamesTheTableAndShowsTheRow(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "box")
	root := insertRootFolder(t, db, box)
	key := insertBlob(t, db, "sha256:aa")
	file := insertFile(t, db, box, root, "a.txt", key)

	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, copyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE files SET name = 'renamed.txt' WHERE id = ?`, file); err != nil {
		t.Fatal(err)
	}

	drift, err := DiffViews(t.Context(), db, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 {
		t.Fatalf("found %d drifting tables, want 1: %+v", len(drift), drift)
	}
	d := drift[0]
	if d.Table != "files" {
		t.Errorf("table = %q, want files", d.Table)
	}
	if d.OnlyInLive != 1 || d.OnlyInRebuilt != 1 {
		t.Errorf("counts = %d live / %d rebuilt, want 1 and 1", d.OnlyInLive, d.OnlyInRebuilt)
	}
	if len(d.Examples) == 0 {
		t.Error("no example rows — the report would be unactionable")
	}
}

// ops must never be compared: it is the input, not the output. Comparing it
// would make the check vacuously pass on a library whose views are wrong but
// whose log is intact — which is every library this check exists for.
func TestDiffViewsIgnoresOps(t *testing.T) {
	db := migrated(t)
	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if _, err := db.Exec(`VACUUM INTO ?`, copyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ops (ts, action, source, payload) VALUES ('t','a','user','{}')`); err != nil {
		t.Fatal(err)
	}
	drift, err := DiffViews(t.Context(), db, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if d.Table == "ops" {
			t.Error("ops was compared; it is the authority, not a view")
		}
	}
}
```

- [ ] **Step 4: 实现 `store.DiffViews`**

`internal/store/diffviews.go`：

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Drift is one table that does not match what replaying the log produces.
type Drift struct {
	Table         string
	OnlyInLive    int
	OnlyInRebuilt int
	Examples      []string
}

// driftExamples bounds how many rows are pulled back per direction. The count
// is the finding; the rows are evidence, and evidence stops helping at about
// five lines.
const driftExamples = 5

// DiffViews compares every rebuildable table in live against the same table in
// a database at rebuiltPath, in both directions.
//
// This is `verify --deep` (spec 3.8), and it is the only production check of
// invariant 1: everything but ops can be rebuilt from ops. Until now that was
// asserted only in a test.
//
// The comparison happens in SQL rather than by diffing two store.Snapshot
// strings, because a snapshot pulls the whole library into memory — a hundred
// thousand files is tens of megabytes of text on both sides. EXCEPT does the
// work in the engine and only the differing rows come back to Go.
//
// snapshotTables is reused deliberately: it already names the rebuildable
// tables and a byte-stable column list for each, and having a second such list
// would mean two places to update when a column is added — with the failure
// mode being a check that quietly stops looking at the new column.
func DiffViews(ctx context.Context, live *sql.DB, rebuiltPath string) ([]Drift, error) {
	// ATTACH is per-connection, and database/sql hands out whichever
	// connection it likes — the same trap Open documents for PRAGMA. Pin one
	// for the whole comparison.
	conn, err := live.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("open a connection for the comparison: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS rebuilt`, rebuiltPath); err != nil {
		return nil, fmt.Errorf("attach the rebuilt copy: %w", err)
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), `DETACH DATABASE rebuilt`)

	var out []Drift
	for _, t := range snapshotTables {
		liveOnly := fmt.Sprintf(
			`SELECT %[1]s FROM main.%[2]s EXCEPT SELECT %[1]s FROM rebuilt.%[2]s`, t.columns, t.name)
		rebuiltOnly := fmt.Sprintf(
			`SELECT %[1]s FROM rebuilt.%[2]s EXCEPT SELECT %[1]s FROM main.%[2]s`, t.columns, t.name)

		nLive, exLive, err := countAndSample(ctx, conn, liveOnly)
		if err != nil {
			return nil, fmt.Errorf("compare %s: %w", t.name, err)
		}
		nRebuilt, exRebuilt, err := countAndSample(ctx, conn, rebuiltOnly)
		if err != nil {
			return nil, fmt.Errorf("compare %s: %w", t.name, err)
		}
		if nLive == 0 && nRebuilt == 0 {
			continue
		}

		d := Drift{Table: t.name, OnlyInLive: nLive, OnlyInRebuilt: nRebuilt}
		for _, e := range exLive {
			d.Examples = append(d.Examples, "in the library but not in the replay: "+e)
		}
		for _, e := range exRebuilt {
			d.Examples = append(d.Examples, "in the replay but not in the library: "+e)
		}
		out = append(out, d)
	}
	return out, nil
}

// countAndSample runs one EXCEPT twice: once to count, once for a few rows.
// Two queries is cheaper than the alternative of streaming every differing row
// back to Go on a library that has drifted badly.
func countAndSample(ctx context.Context, conn *sql.Conn, except string) (int, []string, error) {
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM (`+except+`)`).Scan(&n); err != nil {
		return 0, nil, err
	}
	if n == 0 {
		return 0, nil, nil
	}

	rows, err := conn.QueryContext(ctx, except+fmt.Sprintf(` LIMIT %d`, driftExamples))
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, nil, err
	}
	var out []string
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return 0, nil, err
		}
		parts := make([]string, len(cells))
		for i, c := range cells {
			parts[i] = fmt.Sprint(c)
		}
		out = append(out, strings.Join(parts, " | "))
	}
	return n, out, rows.Err()
}
```

> `snapshotTables` 里没有 `ops`——注释已经写明「它是权威，不是被证明的东西」。`DiffViews` 复用它就自动继承了这一点，`TestDiffViewsIgnoresOps` 守着它别被改回去。

- [ ] **Step 5: 跑 `store` 的测试**

```
go test ./internal/store/
```

Expected: PASS。

- [ ] **Step 6: 实现 `library.verifyDeep`**

`internal/library/deep.go`：

```go
package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/store"
)

// verifyDeep replays the whole log into a scratch copy and compares it with
// the live materialized views.
//
// The copy is made with VACUUM INTO and lives next to kist.db, not in the
// system temp directory: it is as large as the database, and the disk that
// holds a personal archive is the one with room for it. Temp directories are
// also routinely a different filesystem, and on some of them a memory-backed
// one.
//
// Replaying into a copy rather than in place is not caution, it is the
// contract: verify does not write to the library (spec 3.8). Doing it in place
// would also mean that a verify interrupted halfway leaves the library with
// half its views rebuilt.
func (l *Library) verifyDeep(ctx context.Context) (integrity.Report, error) {
	var r integrity.Report

	scratch := filepath.Join(filepath.Dir(l.layout.DB()), fmt.Sprintf(".kist-verify-%d.db", os.Getpid()))
	defer removeDatabase(scratch)

	if _, err := l.db.ExecContext(ctx, `VACUUM INTO ?`, scratch); err != nil {
		return r, fmt.Errorf("copy the database for the deep check: %w", err)
	}

	copyDB, err := store.Open(scratch)
	if err != nil {
		return r, err
	}
	defer copyDB.Close()

	if err := rebuildInto(ctx, copyDB); err != nil {
		return r, fmt.Errorf("replay the log into the copy: %w", err)
	}
	if err := copyDB.Close(); err != nil {
		return r, err
	}

	drift, err := store.DiffViews(ctx, l.db, scratch)
	if err != nil {
		return r, err
	}
	r.Note("tables replayed", len(store.RebuildableTables()))
	for _, d := range drift {
		detail := fmt.Sprintf("%s: %d row(s) only in the library, %d only in the replay",
			d.Table, d.OnlyInLive, d.OnlyInRebuilt)
		r.Add(integrity.Finding{Kind: integrity.ViewDrift, Detail: detail, Fix: "kist rebuild"})
		for _, e := range d.Examples {
			r.Add(integrity.Finding{Kind: integrity.ViewDrift, Detail: "    " + e, Fix: "kist rebuild"})
		}
	}
	return r, nil
}

// removeDatabase deletes a SQLite database and the two files WAL mode leaves
// beside it. Missing any of them turns a scratch copy into litter that grows
// by one file every time verify --deep is run.
func removeDatabase(path string) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		os.Remove(p)
	}
}
```

`store.RebuildableTables() []string` 是 `snapshotTables` 的名字列表，加在 `snapshot.go` 里。

> **`copyDB.Close()` 显式调用了一次，`defer` 里还有一次。** 这是故意的：`DiffViews` 要 ATTACH 那个文件，而 WAL 模式下没关干净的库会留着 `-wal`，比对读到的可能不是重放后的全部内容。`sql.DB.Close` 是幂等的，defer 那次只是兜底。

- [ ] **Step 7: 写 `--deep` 的测试**

`internal/library/deep_test.go`：

```go
// The point of the whole task: a materialized view that does not match the log
// is invisible to the cheap checks and obvious to this one.
func TestDeepVerifyFindsAViewThatDriftedFromTheLog(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"a.txt": "x", "day1/b.txt": "y"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	// Change a view row behind library's back. The op log still says the old
	// name, so a replay produces the old name — which is exactly the drift
	// this check is for.
	forceRename(t, l, "a.txt", "tampered.txt")

	shallow, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if shallow.Count(integrity.ViewDrift) != 0 {
		t.Error("the cheap check reported drift; it cannot know")
	}

	deep, err := l.Verify(t.Context(), VerifyOptions{Deep: true})
	if err != nil {
		t.Fatal(err)
	}
	if deep.Count(integrity.ViewDrift) == 0 {
		t.Error("--deep did not notice a view row that the log does not produce")
	}
}

// A healthy library must come back clean, or the check is noise. This is the
// same assertion as TestRebuildReproducesTheLibraryExactly, asked through the
// command a user can actually run.
func TestDeepVerifyIsCleanOnAHealthyLibrary(t *testing.T) { /* … */ }

// And it must not have written anything, including leaving its scratch copy
// behind.
func TestDeepVerifyLeavesNothingBehind(t *testing.T) {
	// … run it, then assert the library directory holds no .kist-verify-* file
	// and store.Snapshot is unchanged …
}
```

- [ ] **Step 8: 跑全部并确认 `--deep` 会咬**

```
go test ./internal/library/
```

然后把 `verifyDeep` 里的 `rebuildInto` 调用注释掉（副本原样比对，必然零差异），跑 `TestDeepVerifyFindsAViewThatDriftedFromTheLog`：必须红。恢复。

- [ ] **Step 9: 提交**

```bash
git add internal/store/diffviews.go internal/store/diffviews_test.go \
        internal/library/deep.go internal/library/deep_test.go internal/library/rebuild.go \
        internal/store/snapshot.go
git commit -m "feat(verify): --deep 把不变量 1 交到用户手里

不变量 1 说 kist.db 里除 ops 外每张表都可重建。到 M1c 为止这条只在测试
里被验证过（TestRebuildReproducesTheLibraryExactly）。--deep 是把同一个
断言变成用户能跑的命令：VACUUM INTO 一份副本、在副本上重放、ATTACH 后
逐表 EXCEPT 双向比。

三个决定：

- 在副本上重放而不是原地。verify 不写库这条不为它破例；而且原地做的话，
  中途被打断的 verify 会留下一个只重建了一半视图的库
- 用 EXCEPT 而不是比对两份 store.Snapshot 字符串。快照要把整个库拉进
  内存，十万文件就是两边各几十兆；EXCEPT 让引擎干活，只有差异行回 Go
- 副本放在 kist.db 旁边而不是系统临时目录。它和库一样大，而放得下个人
  归档的那块盘才是有空间的那块；临时目录还经常是另一个文件系统，有些
  平台上是内存盘

复用 snapshotTables 而不是另列一份表清单：另列一份意味着加列时要改两处，
而漏改的表现是一个悄悄不再看新列的检查。ops 不在里面，它是权威不是被
证明的东西，有测试守着这一点。

ATTACH 是 per-connection 的，而 database/sql 会给你任意一条连接——和
store.Open 注释里那个 PRAGMA 陷阱是同一个。所以比对全程钉在一条 Conn 上。"
```

---

### Task 7: `kist scrub`

**Files:**
- Create: `internal/library/scrub.go`, `internal/library/scrub_test.go`, `cmd/kist/scrub.go`
- Test: 同上

**Interfaces:**
- Consumes: `blob.Dir.Rehash`、`store.AllBlobs`
- Produces: `library.ScrubProgress`、`(*Library).Scrub(ctx, func(ScrubProgress)) (integrity.Report, error)`

scrub 是 verify 那条「只 stat」的界限之外的东西：**它读完每个字节**。因此它必须可中断、必须报进度，而且它报出来的坏 blob 是 `repair` 的输入。

- [ ] **Step 1: 写失败的测试**

`internal/library/scrub_test.go`：

```go
// The finding scrub exists for: content that still has the right length but no
// longer hashes to its own name. verify cannot see this, by design (spec 3.8).
func TestScrubFindsBitRotThatVerifyCannotSee(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{"passport.jpg": "hello"})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}
	// Same five bytes, different content — a flipped bit, not a truncation.
	overwriteSomeBlob(t, l, "world")

	shallow, err := l.Verify(t.Context(), VerifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if shallow.Total() != 0 {
		t.Errorf("verify read the content: %d findings", shallow.Total())
	}

	deep, err := l.Scrub(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if deep.Count(integrity.BlobCorrupt) != 1 {
		t.Errorf("BlobCorrupt = %d, want 1", deep.Count(integrity.BlobCorrupt))
	}
}

// A healthy library scrubs clean and says how much it read.
func TestScrubIsCleanAndReportsWhatItRead(t *testing.T) { /* … Checked["blobs"] == 2 … */ }

// scrub on a real library is hours. Ctrl-C must stop it, and what it found so
// far must survive — the same rule import follows (spec 3.7.1).
func TestScrubStopsWhenCancelledAndKeepsWhatItFound(t *testing.T) {
	l, box := openWithBox(t)
	src := writeTree(t, map[string]string{
		"a.txt": "1", "b.txt": "22", "c.txt": "333", "d.txt": "4444",
	})
	if _, err := l.Import(t.Context(), src, ImportOptions{Box: box}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	seen := 0
	r, err := l.Scrub(ctx, func(p ScrubProgress) {
		seen++
		if seen == 2 {
			cancel()
		}
	})
	if err != nil {
		t.Fatalf("cancelling a scrub must not be an error: %v", err)
	}
	if r.Checked["blobs"] >= 4 {
		t.Errorf("checked %d blobs after cancelling at 2", r.Checked["blobs"])
	}
	if r.Checked["blobs"] == 0 {
		t.Error("the work done before the cancel was thrown away")
	}
}

// scrub reads; it must not write. Especially: finding a corrupt blob must not
// delete it — the corrupt copy is sometimes the only copy, and partial
// content beats no content (spec 3.8's "degrade to derived", same instinct).
func TestScrubChangesNothing(t *testing.T) { /* … snapshot before/after, blob still on disk … */ }
```

- [ ] **Step 2: 跑一遍确认它红**

```
go test ./internal/library/ -run Scrub
```

- [ ] **Step 3: 实现**

`internal/library/scrub.go`：

```go
package library

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/integrity"
	"github.com/jinzhao1994/kist/internal/store"
)

// ScrubProgress is reported after each blob, so a command that will run for
// hours can say something.
type ScrubProgress struct {
	Done  int
	Total int
	Bytes int64
	Key   string
}

// Scrub re-hashes every blob and reports the ones whose content no longer
// matches their name.
//
// This is the check verify deliberately cannot do: bit rot leaves the size
// alone, so nothing short of reading every byte finds it (spec 3.8). On a
// real archive that is hours, which is why it takes a progress callback and
// why it stops cleanly when its context is cancelled.
//
// Cancelling is not an error. A scrub that was stopped after four hours still
// checked four hours' worth of blobs, and throwing that away because the user
// pressed Ctrl-C would be the same mistake import used to make (spec 3.7.1).
//
// Nothing is written, and in particular a corrupt blob is not removed: it may
// be the only copy there is, and damaged content still beats none.
func (l *Library) Scrub(ctx context.Context, progress func(ScrubProgress)) (integrity.Report, error) {
	var r integrity.Report

	known, err := store.AllBlobs(l.db)
	if err != nil {
		return r, err
	}

	for i, b := range known {
		if ctx.Err() != nil {
			r.Add(integrity.Finding{Kind: integrity.ScrubIncomplete,
				Detail: fmt.Sprintf("stopped after %d of %d blobs", i, len(known))})
			break
		}

		key, err := blob.ParseKey(b.Key)
		if err != nil {
			continue // the structural layer's finding, not this one
		}
		hex, size, err := l.blobs.Rehash(ctx, key)
		if err != nil {
			if ctx.Err() != nil {
				r.Add(integrity.Finding{Kind: integrity.ScrubIncomplete,
					Detail: fmt.Sprintf("stopped after %d of %d blobs", i, len(known))})
				break
			}
			// Unreadable is a finding, not a stop: the next blob may be fine,
			// and the user needs the whole list.
			r.Add(integrity.Finding{Kind: integrity.BlobMissing, Detail: b.Key, Fix: "kist repair"})
			continue
		}

		r.Note("blobs", 1)
		if hex != key.Hex() {
			r.Add(integrity.Finding{
				Kind:   integrity.BlobCorrupt,
				Detail: fmt.Sprintf("%s hashes to %s", b.Key, hex),
				Fix:    "kist repair",
			})
		} else if size != b.Size {
			r.Add(integrity.Finding{
				Kind:   integrity.BlobWrongSize,
				Detail: fmt.Sprintf("%s: %d bytes on disk, %d in the database", b.Key, size, b.Size),
				Fix:    "kist repair",
			})
		}

		if progress != nil {
			progress(ScrubProgress{Done: i + 1, Total: len(known), Bytes: size, Key: b.Key})
		}
	}
	return r, nil
}
```

新增 `integrity.ScrubIncomplete`（`SeverityWarn`，headline `"the scrub did not finish"`）。**一趟没跑完的 scrub 必须在报告里说出来**——「没发现问题」和「没查完所以没发现问题」是两句完全不同的话，而这个命令的全部价值就是让人敢相信前一句。

- [ ] **Step 4: 跑测试**

```
go test ./internal/library/ ./internal/integrity/
```

- [ ] **Step 5: 写 `kist scrub`**

`cmd/kist/scrub.go`：进度写 stderr（`\r` 单行刷新，非 TTY 时降级成每 N 个一行），报告写 stdout。退出码规则与 `verify` 一致，复用同一个 `severityExit` 函数。

信号处理**复用 `cmd/kist/import.go` 的 `interruptContext`**——它已经解决过同一个问题（第一次 Ctrl-C 优雅停止、第二次强制退出）。如果它现在是 `import.go` 的私有函数，这一步把它搬进一个共用文件（`cmd/kist/interrupt.go`），`import.go` 的行为不变。

- [ ] **Step 6: 确认「没查完要说出来」这条会咬**

把 `ScrubIncomplete` 那两处 `r.Add` 删掉，跑 `TestScrubStopsWhenCancelledAndKeepsWhatItFound` 的一个变体：断言 `r.Count(integrity.ScrubIncomplete) == 1`。必须红。加回去。

- [ ] **Step 7: 手动走查**

在 Task 5 建的 `/tmp/m2a` 上：

```bash
kist scrub                    # 期望：进度条动，最后 no problems found，checked N blobs
# 篡改一个 blob 的内容但保持长度
kist verify                   # 期望：干净——它看不见
kist scrub                    # 期望：error: content no longer matches its hash
# 跑一个大目录的 scrub，中途 Ctrl-C
kist scrub                    # 期望：立刻停下，报告里有 "the scrub did not finish"
```

贴实际输出。**特别看中断那次**：如果它花了几秒才停，说明 `readerContext` 没生效（大文件读到一半不检查 ctx），回 Task 3 修。

- [ ] **Step 8: 提交**

```bash
git add internal/library/scrub.go internal/library/scrub_test.go cmd/kist/scrub.go cmd/kist/interrupt.go internal/integrity/
git commit -m "feat(scrub): 全库重新哈希，可中断，没查完会说出来

scrub 是 verify 那条「只 stat」界限之外的东西：它读完每个字节，所以位翻转
只有它能发现。测试里钉死了这个分工——同样长度、不同内容，verify 必须
看不见，scrub 必须看见。

一趟没跑完要在报告里明说。「没发现问题」和「没查完所以没发现问题」是两句
完全不同的话，而这个命令的全部价值就是让人敢相信前一句。

取消不是错误：跑了四小时的 scrub 被 Ctrl-C，那四小时的结果照样交出去，
和导入是同一条规矩（3.7.1）。发现坏 blob 也不删——它可能是唯一一份，
坏内容也比没有强。"
```

---

### Task 8: 远端与 `kist repair`

**Files:**
- Create: `internal/blob/restore.go`, `internal/blob/restore_test.go`, `internal/library/repair.go`, `internal/library/repair_test.go`, `cmd/kist/repair.go`
- Modify: `internal/config/config.go`
- Test: 同上

**Interfaces:**
- Consumes: `blob.OpenDir`（Task 3）、`integrity.Report`
- Produces: `config.Remote{Name, Path}`、`config.Config.Remotes`、`(*Dir).Restore(ctx, r io.Reader, want Key) (Outcome, error)`、`library.RepairOptions{Apply bool}`、`library.RepairResult`、`(*Library).Repair(ctx, RepairOptions) (RepairResult, error)`

这是 3.8 那句「这是真答案，而内容寻址让它几乎免费」的兑现。**唯一要紧的性质是不信任远端**：边读边哈希，对不上就丢掉。

- [ ] **Step 1: 配置里加 `[[remote]]`**

`internal/config/config.go`：

```go
// Remote is somewhere else the same blobs live: another directory, a mounted
// external drive, a network share.
//
// kist never writes to one. Backups are rsync's job (spec 3.11), and this is
// only ever read from — which is why a remote needs no more than a path, and
// why mounting it read-only is the right thing to do.
type Remote struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}
```

`Config` 加 `Remotes []Remote \`toml:"remote"\``，`Load` 里校验：`Name` 和 `Path` 都不能为空，`Name` 不能重复（它是 `repair` 输出里指代来源的方式）。

测试：一份带两个 `[[remote]]` 的 toml 能读回来；重名报错；`Save` 之后 `Load` 得到同样的东西（round-trip）。

- [ ] **Step 2: 写 `Dir.Restore` 的测试（先红）**

`internal/blob/restore_test.go`：

```go
// Restore is repair's primitive: take content from somewhere untrusted, hash
// it on the way in, and only keep it if it is what was asked for.
func TestRestoreWritesContentThatMatches(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "the original content")
	corrupt(t, d, k, "rot")

	outcome, err := d.Restore(context.Background(), strings.NewReader("the original content"), k)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != OutcomeRepaired {
		t.Errorf("outcome = %v, want OutcomeRepaired", outcome)
	}
	hex, _, err := d.Rehash(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	if hex != k.Hex() {
		t.Error("the blob still does not hash to its own name")
	}
}

// The property the whole feature rests on. A remote that hands back the wrong
// bytes — a truncated copy, a different file, a hostile one — must change
// nothing, and must not leave the bad content anywhere.
func TestRestoreRefusesContentThatDoesNotMatch(t *testing.T) {
	d := newTestDir(t)
	k := put(t, d, "the original content")
	corrupt(t, d, k, "rot")

	_, err := d.Restore(context.Background(), strings.NewReader("something else entirely"), k)
	if err == nil {
		t.Fatal("Restore accepted content that hashes to something else")
	}

	// The corrupt blob is untouched — bad content beats no content — and the
	// wrong content was not stored anywhere.
	body, readErr := os.ReadFile(mustLocalPath(t, d, k))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(body) != "rot" {
		t.Errorf("the existing blob was modified: %q", body)
	}
	if n := countBlobs(t, d); n != 1 {
		t.Errorf("%d blobs on disk, want 1 — the rejected content was stored", n)
	}
	if n := countTmp(t, d); n != 0 {
		t.Errorf("%d files left in tmp/", n)
	}
}

// A blob that is missing entirely is the other half of repair.
func TestRestoreCreatesAMissingBlob(t *testing.T) { /* … outcome == OutcomeStored … */ }

// A key with a collision suffix must be restored into exactly that slot. Put
// would walk the suffixes from zero and land in the wrong one.
func TestRestoreHonoursTheCollisionSuffix(t *testing.T) {
	d := newTestDir(t)
	k := NewKey(AlgoSHA256, sha256Hex("content"), 1)
	if _, err := d.Restore(context.Background(), strings.NewReader("content"), k); err != nil {
		t.Fatal(err)
	}
	p, _ := d.LocalPath(k)
	if _, err := os.Stat(p); err != nil {
		t.Errorf("the blob did not land at suffix 1: %v", err)
	}
}
```

- [ ] **Step 3: 实现 `Dir.Restore`**

`internal/blob/restore.go`：

```go
package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrContentMismatch means the content offered did not hash to the key that
// was asked for.
var ErrContentMismatch = errors.New("content does not match the key it was fetched for")

// Restore puts content fetched from somewhere else at exactly the key it was
// fetched for, after verifying it.
//
// This is repair's primitive (spec 3.8), and it is not Put:
//
//   - Put decides where content goes by hashing it and walking the collision
//     suffixes. Restore is told where it goes. A key like "sha256:ab….1" is a
//     specific slot, and Put would happily land in slot 0.
//   - Put trusts what it is given, because the caller is an import reading the
//     user's own disk. Restore does not: it is reading from a remote, and a
//     remote that hands back a truncated file must change nothing.
//
// If the content does not match, nothing is written and the existing blob —
// corrupt or not — is left alone. Damaged content is sometimes the only
// content there is.
func (d *Dir) Restore(ctx context.Context, r io.Reader, want Key) (Outcome, error) {
	tmp, err := os.CreateTemp(filepath.Join(d.root, "tmp"), "restore-*")
	if err != nil {
		return 0, fmt.Errorf("stage restored content: %w", err)
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		tmp.Close()
		if !keep {
			os.Remove(tmpPath)
		}
	}()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, h), readerContext{ctx: ctx, r: r})
	if err != nil {
		return 0, fmt.Errorf("read the content to restore: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return 0, fmt.Errorf("stage restored content: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("stage restored content: %w", err)
	}

	if got := hex.EncodeToString(h.Sum(nil)); got != want.Hex() {
		return 0, fmt.Errorf("%w: wanted %s, got %s", ErrContentMismatch, want.Hex(), got)
	}

	dst := filepath.Join(d.root, want.RelPath())
	_, statErr := os.Stat(dst)
	switch {
	case errors.Is(statErr, fs.ErrNotExist):
		if err := d.commit(tmpPath, dst); err != nil {
			return 0, err
		}
		keep = true
		return OutcomeStored, nil
	case statErr != nil:
		return 0, fmt.Errorf("stat blob being restored: %w", statErr)
	}

	// Something is already there. It either hashes correctly — in which case
	// there was nothing to repair — or it does not, and we have verified
	// content to put in its place.
	existingHex, _, err := d.rehashPath(ctx, dst)
	if err != nil {
		return 0, err
	}
	if existingHex == want.Hex() {
		return OutcomeDeduped, nil
	}
	if err := d.replace(tmpPath, dst); err != nil {
		return 0, err
	}
	keep = true
	d.log.Info("replaced a damaged blob with verified content", "key", want, "was", existingHex)
	return OutcomeRepaired, nil
}
```

> `Rehash` 抽出一个按路径工作的 `rehashPath(ctx, path)`，`Rehash` 和 `Restore` 共用。`Restore` 不能走 `Rehash(key)`，因为它要哈希的是**将要被覆盖的那个文件**，而那时它是不是该 key 的合法内容正是问题所在。

- [ ] **Step 4: 跑 blob 测试**

```
go test ./internal/blob/
```

- [ ] **Step 5: 实现 `library.Repair`**

`internal/library/repair.go`：

```go
// RepairOptions controls repair.
type RepairOptions struct {
	// Apply actually writes. The default is a dry run, the same shape spec 3.9
	// gives gc: this is the only command in M2a that writes to blobs/, and the
	// only thing standing between a wrong remote path and a library full of
	// somebody else's files is that the user had to type --yes.
	Apply bool
	// Only limits repair to these keys. Empty means everything verify and
	// scrub found.
	Only []string
}

// RepairResult is what repair did, or would do.
type RepairResult struct {
	Broken     []string          // keys that need repairing
	Repaired   map[string]string // key → remote it came from
	Unfixable  []string          // no remote had a good copy
	DryRun     bool
}

// Repair fetches damaged or missing blobs from a configured remote.
//
// This is spec 3.8's real answer to corruption, and content addressing is what
// makes it almost free: you know the hash, you ask any remote for that hash,
// you verify it, you put it back. The verification is not a formality — it is
// the reason a remote can be an unattended external drive, a network share, or
// anything else you would not otherwise trust with your archive.
//
// It writes only blobs/. No op is appended: restoring content that is correct
// by definition is not a state change, and Apply cannot touch blobs/ anyway
// (spec 3.8, "修复不写 op").
func (l *Library) Repair(ctx context.Context, opt RepairOptions) (RepairResult, error) {
	// 1. Work out what is broken: blobs the database knows about that are
	//    missing, the wrong size, or (if scrub ran) do not hash correctly.
	// 2. Open every configured remote with blob.OpenDir. A remote that will
	//    not open is a warning, not a failure — the other one may have it.
	// 3. For each broken key, ask each remote in configuration order for
	//    Get(key), and hand the reader to l.blobs.Restore. First success wins.
	// 4. In a dry run, stop after establishing that a remote has a blob of the
	//    right size under the right key. Do not download it: on a mounted
	//    drive that is the whole file, and a dry run must be cheap.
}
```

> **步骤 4 那条注释是本 task 最容易做错的地方。** dry-run 如果为了「确认真的能修」而把每个文件都从远端拉下来验一遍，它就和真修一样慢一样贵，用户会直接跳过它去 `--yes`——那就等于没有 dry-run。dry-run 的承诺是「我看了一眼，看起来能修」，不是「我保证能修」。这句要写进 `RepairOptions.Apply` 的 godoc 里。

`internal/library/repair_test.go` 的关键 case：

```go
// The end-to-end promise of M2a: a library whose content was damaged comes
// back from a second copy.
func TestRepairRestoresFromARemote(t *testing.T) {
	// … import into l; rsync-style copy of blobs/ to a second directory;
	//    register it as a remote; corrupt a blob; scrub finds it;
	//    Repair with Apply: true; scrub is clean again …
}

// A remote holding the wrong bytes must change nothing. This is the test that
// makes "you may point kist at an untrusted drive" true.
func TestRepairRejectsARemoteThatServesWrongContent(t *testing.T) { /* … */ }

// A dry run must not write. It is the default, so this is the behaviour almost
// every invocation gets.
func TestRepairDryRunWritesNothing(t *testing.T) { /* … */ }

// No remotes configured is not an error — it is the state every new library is
// in — but the result has to say so, or the user sees "repaired 0" and thinks
// their library is fine.
func TestRepairWithNoRemotesSaysSo(t *testing.T) { /* … */ }
```

- [ ] **Step 6: 写 `kist repair` 与 `kist remote list`**

`cmd/kist/repair.go`：

- `kist repair` — 默认 dry-run，打印「would repair N blobs from <remote>；rerun with --yes to do it」
- `kist repair --yes` — 真修，打印每个修好的 key 和来源
- `kist remote list` — 列出配置里的远端、每个能不能打开、里面有多少 blob。**没有 `remote add`**：`kist.toml` 的头部注释已经写着「Managed by kist; hand edits are read back on the next run」，而 v0.1 加一个远端是一辈子一次的操作。`remote list` 存在只是为了让这个功能能被发现，并且在配错路径时给出一句人话

`kist remote list` 在没有配置远端时打印一段带 `[[remote]]` 例子的提示——**这是用户唯一会知道这个功能存在的地方**。

- [ ] **Step 7: 确认「不信任远端」真的会咬**

把 `Restore` 里那句 `if got != want.Hex()` 改成 `if false`，跑 `TestRestoreRefusesContentThatDoesNotMatch`：必须红。改回去。

再确认一次 dry-run：把 `RepairOptions.Apply` 的判断去掉，跑 `TestRepairDryRunWritesNothing`：必须红。

- [ ] **Step 8: 手动走查——M2a 的验收**

```bash
cd /tmp/m2a
rsync -a blobs/ /tmp/m2a-backup/blobs/          # 第二份副本
cat >> kist.toml <<'EOF'

[[remote]]
name = "backup"
path = "/tmp/m2a-backup/blobs"
EOF
kist remote list                                 # 期望：backup，可读，N 个 blob

# 制造两种损坏
find blobs/sha256 -type f | head -1 | xargs chmod 644
find blobs/sha256 -type f | head -1 | xargs -I{} sh -c 'printf X | dd of={} bs=1 seek=0 conv=notrunc'
find blobs/sha256 -type f | tail -1 | xargs chmod 644
find blobs/sha256 -type f | tail -1 | xargs rm

kist verify        # 期望：报出那个被删的（missing），看不见被篡改的
kist scrub         # 期望：两个都报出来
kist repair        # 期望：would repair 2 from backup；什么都没写
kist scrub         # 期望：还是两个——dry run 没修
kist repair --yes  # 期望：repaired 2
kist scrub         # 期望：no problems found
```

**这一串走完就是 M2a 的产出兑现了**：数据坏了能发现，发现了能修。把实际输出贴进本步骤。

- [ ] **Step 9: 提交**

```bash
git add internal/blob/restore.go internal/blob/restore_test.go internal/config/ \
        internal/library/repair.go internal/library/repair_test.go cmd/kist/repair.go
git commit -m "feat(repair): 从远端副本修回来，边读边哈希，对不上就丢

3.8 那句「这是真答案，而内容寻址让它几乎免费」的兑现。唯一要紧的性质是
不信任远端——正因为验一遍是强制的，远端才可以是一块无人值守的移动硬盘
或一个网络共享。

Restore 不是 Put，两处不同都要紧：Put 靠哈希决定放哪并会走碰撞后缀，而
Restore 是被告知放哪（sha256:ab….1 是一个具体的槽，Put 会落进 0 号）；
Put 信任调用方（导入读的是用户自己的盘），Restore 不信任。

对不上的时候什么都不写，原来那个坏 blob 也不动——它可能是唯一一份，
坏内容也比没有强。

dry-run 是默认，而且它不下载。为了「确认真的能修」把每个文件都从远端拉
下来验一遍的话，它就和真修一样慢一样贵，用户会直接跳过它去 --yes，那
就等于没有 dry-run。dry-run 的承诺是「我看了一眼」，不是「我保证」。

没有 remote add：kist.toml 头上就写着手改会被读回去，而加一个远端是一辈子
一次的操作。remote list 存在是为了这个功能能被发现。"
```

---

### Task 9: `kist rebuild`

**Files:**
- Create: `cmd/kist/rebuild.go`, `cmd/kist/rebuild_test.go`
- Test: 同上

**Interfaces:**
- Consumes: `(*Library).Rebuild`、`(*Library).Relink`
- Produces: `kist rebuild [--yes]`

`library.Rebuild` 从 M1b 起就在，只是没有 CLI。它是 `verify --deep` 报出 `ViewDrift` 之后用户要跑的那条命令，所以现在必须有。

- [ ] **Step 1: 写测试**

`cmd/kist/rebuild_test.go`（照 `cmd/kist` 现有的命令测试写法）：

```go
// rebuild without --yes must not touch anything: it discards every
// materialized view, and although that is recoverable by construction
// (invariant 1), "recoverable" is not "expected".
func TestRebuildNeedsConfirmation(t *testing.T) { /* … */ }

// After rebuild, boxes/ has to be right too. The views were just thrown away
// and rebuilt, so the cached directory names are stale — and a rebuild that
// leaves the projection broken is a rebuild people learn not to run.
func TestRebuildRelinksAfterwards(t *testing.T) { /* … */ }
```

- [ ] **Step 2: 实现**

`cmd/kist/rebuild.go`：

```go
// newRebuildCmd builds `kist rebuild`.
//
// It throws away every materialized view and replays the op log (invariant 1).
// That is safe by construction — ops and blobs/ are the authority — but it is
// still minutes of work on a large library and it is not something to do by
// accident, so it asks first, the same way spec 3.9 has gc ask.
//
// It relinks afterwards. The views were just rebuilt, so every cached box
// directory name is stale, and a rebuild that leaves boxes/ wrong is a command
// people learn not to trust.
```

- 无参跑：打印将要重放多少条 op（`store` 那边已有的计数走 `library`，`cmd` 不许 import `store`——**加一个 `(*Library).OpCount(ctx) (int64, error)`**），然后用 `cliutil.Confirm`（`init` 用的那个）问一句
- `--yes` 跳过确认
- 跑完打印重放了多少条 op、`relink` 的统计

- [ ] **Step 3: 跑测试并手动走查**

```bash
cd /tmp/m2a
kist rebuild            # 期望：先问，答 n 什么都不做
kist rebuild --yes      # 期望：replayed N ops, relinked M files
kist verify --deep      # 期望：no problems found
```

- [ ] **Step 4: 提交**

```bash
git add cmd/kist/rebuild.go cmd/kist/rebuild_test.go internal/library/
git commit -m "feat(cli): kist rebuild

library.Rebuild 从 M1b 起就在，只是没有 CLI。它是 verify --deep 报出
ViewDrift 之后用户要跑的那条命令，所以现在必须有。

跑完接一次 relink：视图刚被扔掉重建，缓存的 box 目录名全是旧的，而一条
会把 boxes/ 留在坏状态的 rebuild，用户会学会不去跑它。

问一句再动手。重放是安全的（不变量 1 就是这么设计的），但「可恢复」不是
「本来就打算这么干」，而且大库上它是分钟级的。"
```

---

### Task 10: 收尾——文档同步、CI、里程碑状态

**Files:**
- Modify: `docs/design/2026-08-22-kist-v0.1-design.md`、`docs/design/open-questions.md`、`CLAUDE.md`、`README.md`、`README.zh-CN.md`

- [ ] **Step 1: 把实现中冒出来的偏离写回设计文档**

**这是本仓库唯一真正要紧的规矩。** 逐条过一遍前九个 task 里所有「实现时发现规范说得不对/不够」的地方，每一条都要写进设计文档，**并写明为什么不选另一条路**。已知至少这些：

- `Restore` 不是 `Put`（3.8 只说了「向远端要这个哈希，验一遍，覆盖回去」，没说碰撞后缀那一层）
- dry-run 不下载（3.9 给 gc 定 dry-run 时说的是「显示将永久删除 N 个文件」，那是纯读；repair 的 dry-run 有个「验到什么程度」的取舍，规范里没有）
- `ScrubIncomplete`：报告要能说「没查完」
- 结构层检查报的是 blob key 还是文件名（Task 5 Step 10 那个产品判断的结论）

- [ ] **Step 2: 更新 `open-questions.md`**

划掉本里程碑解决掉的，并把 M2a 期间新暴露的记进去（**大概率至少有这几条，实现时按实际情况增删**）：

- **远端只有「有没有」，没有「新不新」。** 修复只按哈希取，所以永远不会拿到错内容——但也永远不知道远端是不是过期的备份。用户配了一个三年前的远端，`repair` 会静静地什么都修不了
- **`repair` 没有进度。** 从一块 USB 2.0 的移动硬盘修 200 GB 是几个小时
- **多进程下 `scrub` 和 `import` 同时跑会怎样。** B3 的一部分，但 scrub 是第一个真的会跑几小时的命令，它把这个问题从理论变成了日常

- [ ] **Step 3: 更新 `CLAUDE.md` 的当前状态**

「M2a 已完成」段照 M1a/M1b/M1c 的格式写，并把「下一步」改成 M2b（先定 B1c 的四件事）。

**把那句实践建议撤掉**：「M2a 通过之前，别删掉导入源的原始副本」现在解除了，改成一句说明——现在 `kist verify` / `kist scrub` 能告诉你库里的东西有没有问题，`kist repair` 能从第二份副本修回来，**前提是你真的有第二份副本**。

- [ ] **Step 4: README 两份都加上新命令**

`README.md`（英文）和 `README.zh-CN.md`（中文）的命令清单加 `verify` / `scrub` / `repair` / `rebuild`，并加一小节讲远端怎么配——这是 v0.1 唯一一个必须手改 `kist.toml` 的功能，不写进 README 就等于不存在。

- [ ] **Step 5: 全量验证**

```
make check
```

四平台 vet、`go test -race ./...`、gofmt、`go mod tidy` 干净。

- [ ] **Step 6: 在一个真实规模的库上跑一遍**

用 M1c 那个 4000 文件的走查库（或重新导一次）：

```bash
time kist verify
time kist verify --deep
time kist scrub
```

**把三个时间记进设计文档 3.8。** 这三个数字决定了那句「verify 要能被频繁跑、能挂 cron，所以 --deep 不做默认」是不是真的成立——如果 `verify` 和 `verify --deep` 在 4000 文件上差不多快，那条理由在这个规模上不成立，要么改结论要么说明它在什么规模上才成立。

- [ ] **Step 7: 提交并推**

```bash
git add -A
git commit -m "docs: M2a 完成——设计文档同步、README、里程碑状态

（正文列出 Step 1 实际写回去的每一条偏离，以及 Step 6 量到的三个时间）"
git push
```

- [ ] **Step 8: 等 CI 三平台绿**

Windows runner 是唯一会执行到某些代码路径的地方，M1c 已经证明过一次。特别留意：

- `os.SameFile` 在 `CheckLinks` 里用的是**两个都存在**的路径，和 M1c 那次踩的坑不同——但还是要看它绿
- `VACUUM INTO` 的目标路径在 Windows 上带盘符和反斜杠，走的是绑定参数所以应该没问题，但这是第一次这么用
- `CleanTmp` 删的是刚被别的测试写过的文件，Windows 上文件占用比 Unix 严格

---

## 自查

**规范覆盖：** 3.8 的每一段——verify 四类检查、`--deep`、scrub、repair 的独立命令与 dry-run、不写 op、远端不是第二个类型、按 link mode 分级——都能指到一个 task。3.9 的 `tmp/` 清理在 Task 3（实现）和 Task 5（接线）。3.10 的 `LocalPath` 逃生舱不需要改动。3.11 的备份不在 M2a（`kist backup` 没在第 7 节的 M2 清单里，不擅自加）。

**不在 M2a 的：** `gc`（3.9 明确推迟）、`kist backup`（第 7 节没列，`rsync` 已经够用）、Query AST 与 conformance suite（M2b）、远端的写入（3.11 说备份是 rsync 的活）。

**类型一致性：** `integrity.Report` 被五处产出（`CheckStructure` / `CheckBlobs` / `CheckProjection` / `verifyDeep` / `Scrub`），全部通过 `Merge` 汇总；`blob.Outcome` 在 `Restore` 里复用了 `Put` 的四个值，没有新增；`projection.Diff` 同时服务 `SyncDir` 和 `DiffDir`。

**每个 task 都有一个「确认守卫会咬」的步骤**（Task 1 Step 5、Task 2 Step 7、Task 3 Step 8、Task 4 Step 8、Task 5 Step 9、Task 6 Step 8、Task 7 Step 6、Task 8 Step 7）。这是 M1b 起的惯例：一个从没红过的测试和没有测试是一回事。
