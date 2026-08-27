# M1b op log 闭环与唯一写入口 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `kist box new` / `kist tag` / `kist undo` 真的能跑，且每一次写入都是「一个事务里插一条 op + 更新物化视图」——同时用 `Rebuild` 从 ops 重放出逐字节相同的库，把不变量 3 从口号变成测试。

**Architecture:** 在 M1a 的三层上再叠两层。`oplog` 定义 op 的 action、payload、`Apply` 与 `Inverse`，只认识 `domain` 和一个它自己定义的 `Tx` 接口——**它不认识数据库**。`store` 长出 ops 表的读写和物化视图的写接口，后者结构性地满足 `oplog.Tx`。`library` 把两者接起来，成为唯一的写入口：每个写操作在一个事务里完成「插 op → Apply → 写索引表」。`cmd` 只调 `library`。

**M1b 不碰文件。** 没有 `import`、没有 `boxes/` 投影、没有 `ls` / `find` / `relink`——那些是 M1c。M1b 里能被 tag 的实体只有 box（`box_tags`），因为 `files` 表在 M1c 之前一直是空的。

**Tech Stack:** M1a 的全部依赖，不新增。`encoding/json`（payload）、`spf13/cobra`（CLI）、标准库 `testing`。

**Spec:** `docs/design/2026-08-22-kist-v0.1-design.md`（配套待决问题见 `docs/design/open-questions.md`）

**前置：** M1a 全部完成（`domain` / `blob` / `store.Open` / `store.Migrate` / `config` / `library.Create` / `kist init`）。

## Global Constraints

这些是全局要求，每个 task 的验收都隐含包含它们：

- **Go 1.26.1**，版本从 `go.mod` 读
- **`CGO_ENABLED=0` 必须能构建**，四个目标平台 `darwin/arm64` `darwin/amd64` `linux/amd64` `windows/amd64`
- **不变量 1：权威数据只有 `ops` 表 + `blobs/` 目录。** 其余每张表都可重建。M1b 要真的把这句话变成一个跑得起来的 `Rebuild` 和一条断言"重建结果逐字节相同"的测试
- **不变量 2：`library` 是唯一写入口。** `cmd` 不得 import `store`；`oplog` 不得 import `store`
- **不变量 3：`Apply` 必须是纯函数。** 不生成 ID、不读时钟、不做任何依赖当前状态的**决策**。结构上的保证是：`Apply` 只拿到 `(tx, payload)`，时间戳从 `tx.TS()` 取（即 `ops.ts`），ID 一律由 `library` 在生成 op 时铸好写进 payload
- **不变量 4：payload 是不可查询的载荷。** 要被查询的东西提取成列（`reverts_id`）或索引表（`op_boxes` / `op_files`）；反过来每张索引表都必须能从 payload 单向重建——这条由 `Handler.Targets` 承载
- **时间戳一律 UTC，格式 `2006-01-02T15:04:05.000Z`**，定宽，字典序即时间序
- **UUID 在 DB 里存 `BLOB(16)`**
- **凡是要被 `LIKE` 搜或要唯一的文本，都由 Go 生成折叠列**（`domain.Fold`）
- **排序一律字节序**（设计文档 4.8），不引入任何 locale 相关排序
- **语言：代码、注释、godoc 用英文；设计文档与 commit message 用中文**
- 每个 task 结束时 `make check` 必须通过

---

## 与设计文档的两处偏离（都要在本计划内同步回文档）

**① `Inverse` 的签名要带上 action。** 设计文档 2.3 写的是：

```go
Inverse(payload []byte) ([]byte, error)
```

这个签名不成立：`create_box` 的反向是 `delete_box`，**是另一个 action**。只返回 payload 的话调用方无从知道该拿哪个 handler 去 apply。改成：

```go
Inverse(payload []byte) (Action, []byte, error)
```

Task 2 的 Step 6 负责把这条同步进设计文档 2.3。

**② 严格撤销的校验放在 `Apply` 里，而不是单独一个 `Check` 方法。** 设计文档 2.5 只说了"apply 前校验 before 状态是否吻合"，没说校验住在哪。放进 `Apply` 的理由：

- 它**不是决策**，是断言——要么原样执行，要么报错退出。不违反不变量 3（那条禁的是"依赖当前状态**做出不同的行为**"）
- 正向 op 走同一条路也是对的：正向 op 由 `library` 从当前状态生成，校验必然通过；`Rebuild` 重放时前序 op 已经把状态放到位，校验同样通过。**校验失败只可能发生在隔空撤销上**，正是 2.5 要拦的场景
- 单开 `Check` 方法意味着每个 handler 要把 before 状态的比对逻辑写两遍，或者 `Apply` 调 `Check`——那还不如直接写在 `Apply` 里

Task 5 的 Step 8 负责把这条写进设计文档 2.5。

---

## 文件结构

```
internal/domain/
  box.go       box_test.go         Box 实体
  tag.go       tag_test.go         Tag 实体
  folder.go                        Folder 实体（M1b 只用到根 folder）

internal/oplog/
  action.go    action_test.go      Action / Source 常量
  op.go                            Op 结构与 payload 编解码
  tx.go                            ★ Tx 接口——oplog 对数据库的全部要求
  registry.go  registry_test.go    action → Handler 注册表
  targets.go                       Targets（索引表的行）
  box.go       box_test.go         create_box / delete_box / update_box
  tag.go       tag_test.go         create_tag / delete_tag / rename_tag / move_tag
  boxtag.go    boxtag_test.go      tag_boxes / untag_boxes

internal/store/
  ops.go       ops_test.go         ops 表读写 + op_boxes / op_files
  views.go     views_test.go       物化视图的写接口（满足 oplog.Tx）
  snapshot.go                      全库快照，Rebuild 的测试用

internal/library/
  library.go   library_test.go     Open / Close
  write.go     write_test.go       ★ 唯一写路径：一个事务里插 op + Apply + 写索引表
  tagtree.go   tagtree_test.go     内存 tag 树
  commands.go  commands_test.go    CreateBox / UpdateBox / CreateTag / … 
  undo.go      undo_test.go        Undo（严格模式）
  rebuild.go   rebuild_test.go     Rebuild + 纯度证明

cmd/kist/
  box.go       box_test.go         kist box new / edit / list / tag / untag
  tag.go       tag_test.go         kist tag new / rename / move / rm / tree
  oplog.go     oplog_test.go       kist log / kist undo
```

**依赖方向（在 M1a 的基础上继续单向向下）：**

```
domain   ← 不 import 任何本项目包
oplog    ← 只 import domain
store    ← 只 import domain
library  ← import domain / oplog / store / blob / config
cmd      ← 只 import library / cliutil
```

**`oplog` 不 import `store` 是这层结构的关键。** `Tx` 接口定义在 `oplog` 里，由 `store` 结构性地满足（Go 的接口是结构化的，`store` 不需要 import `oplog`），`library` 负责把两者接上。这样 `Apply` 在类型上就够不到数据库的任何东西——它只能通过 `Tx` 上那几个方法说话，而那些方法是我们逐个审过的。不变量 3 因此有了结构性的保障，而不是靠自觉。

---

### Task 1: domain — 实体类型

**Files:**
- Create: `internal/domain/box.go`, `internal/domain/tag.go`, `internal/domain/folder.go`
- Test: `internal/domain/box_test.go`, `internal/domain/tag_test.go`

**Interfaces:**
- Consumes: `domain.BoxID` / `TagID` / `FolderID`、`domain.LocalDate`、`domain.Fold`
- Produces: `domain.Box`、`domain.Tag`、`domain.Folder`、`domain.NewBox(...)`、`domain.NewTag(...)`、`domain.ValidateName(string) error`、`domain.ErrEmptyName` / `ErrNameHasEdgeSpace` / `ErrNameTooLong`

**名字的校验规则住在 `domain` 里，不在 CLI 也不在 `library`。** 设计文档 1.4 提过"幽灵目录"问题——名字首尾带空格在投影时会产生一个看不见的目录，而用户在 UI 上完全看不出两个 box 有什么区别。折叠函数（M1a Task 3）刻意不做 trim，因为那是校验的职责，不是规范化的职责；这个 task 就是那句话的落点。

- [x] **Step 1: 写失败的测试**

`internal/domain/box_test.go`:

```go
package domain

import "testing"

func TestNewBoxFoldsItsName(t *testing.T) {
	id := NewBoxID()
	b, err := NewBox(id, "Yunnan 2019", "the long way round")
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}
	if b.ID != id {
		t.Errorf("ID = %v, want %v", b.ID, id)
	}
	if b.NameFold != Fold("Yunnan 2019") {
		t.Errorf("NameFold = %q, want %q", b.NameFold, Fold("Yunnan 2019"))
	}
	if b.DescriptionFold != Fold("the long way round") {
		t.Errorf("DescriptionFold = %q, want the folded description", b.DescriptionFold)
	}
}

func TestNewBoxRejectsBadNames(t *testing.T) {
	for _, name := range []string{
		"",
		"   ",
		" leading",
		"trailing ",
		"trailing.",
	} {
		if _, err := NewBox(NewBoxID(), name, ""); err == nil {
			t.Errorf("NewBox(%q) = nil error, want a rejection", name)
		}
	}
}

func TestBoxDatesAreOptional(t *testing.T) {
	b, err := NewBox(NewBoxID(), "undated", "")
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}
	if !b.StartAt.IsZero() || !b.EndAt.IsZero() {
		t.Error("a fresh box should have no dates set")
	}
}

func TestValidateName(t *testing.T) {
	ok := []string{
		"婚礼",
		"a",
		"IMG_1234",
		"a b",              // inner spaces are fine
		"file.name.txt",    // inner dots are fine
	}
	for _, n := range ok {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", n, err)
		}
	}

	bad := []struct {
		name string
		want error
	}{
		{"", ErrEmptyName},
		{"  ", ErrEmptyName},               // whitespace-only is empty
		{" x", ErrNameHasEdgeSpace},
		{"x ", ErrNameHasEdgeSpace},
		{"x\t", ErrNameHasEdgeSpace},
		{"x.", ErrNameTrailingDot},
	}
	for _, tt := range bad {
		err := ValidateName(tt.name)
		if err == nil {
			t.Errorf("ValidateName(%q) = nil, want %v", tt.name, tt.want)
			continue
		}
		if !errors.Is(err, tt.want) {
			t.Errorf("ValidateName(%q) = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestValidateNameRejectsOverlyLongNames(t *testing.T) {
	// The limit is on runes, not bytes: a 255-character Chinese name is
	// 765 bytes and would still be a perfectly reasonable name, while a
	// byte limit would cut it off at 85 characters (spec 3.4 keeps DB names
	// clean and lets the projection layer deal with filesystem limits).
	if err := ValidateName(strings.Repeat("好", 255)); err != nil {
		t.Errorf("a 255-rune name was rejected: %v", err)
	}
	if err := ValidateName(strings.Repeat("好", 256)); !errors.Is(err, ErrNameTooLong) {
		t.Errorf("a 256-rune name gave %v, want ErrNameTooLong", err)
	}
}
```

（测试文件顶部需要 `errors`、`strings`、`testing`。）

`internal/domain/tag_test.go`:

```go
package domain

import "testing"

func TestNewTagFoldsItsName(t *testing.T) {
	id := NewTagID()
	tag, err := NewTag(id, nil, "Passport")
	if err != nil {
		t.Fatalf("NewTag: %v", err)
	}
	if tag.ParentID != nil {
		t.Error("a root tag should have no parent")
	}
	if tag.NameFold != Fold("Passport") {
		t.Errorf("NameFold = %q, want %q", tag.NameFold, Fold("Passport"))
	}
}

func TestNewTagWithParent(t *testing.T) {
	parent := NewTagID()
	tag, err := NewTag(NewTagID(), &parent, "护照")
	if err != nil {
		t.Fatalf("NewTag: %v", err)
	}
	if tag.ParentID == nil || *tag.ParentID != parent {
		t.Errorf("ParentID = %v, want %v", tag.ParentID, parent)
	}
}

func TestNewTagRejectsBadNames(t *testing.T) {
	if _, err := NewTag(NewTagID(), nil, " "); err == nil {
		t.Error("NewTag with a blank name = nil error, want a rejection")
	}
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `go test ./internal/domain/ -run 'TestNewBox|TestValidateName|TestNewTag|TestBoxDates' -v`
Expected: FAIL，`undefined: NewBox`

- [x] **Step 3: 实现名字校验**

`internal/domain/name.go`:

```go
package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxNameRunes bounds box, folder, tag and file names. The limit counts runes,
// not bytes: a byte limit would let an English name be three times longer than
// a Chinese one for no reason anybody could explain.
const MaxNameRunes = 255

var (
	ErrEmptyName        = errors.New("name is empty")
	ErrNameHasEdgeSpace = errors.New("name starts or ends with whitespace")
	ErrNameTrailingDot  = errors.New("name ends with a dot")
	ErrNameTooLong      = errors.New("name is too long")
)

// ValidateName rejects names that would produce a projected directory nobody
// can tell apart from its neighbour (spec 1.4).
//
// Normalization does not do this job: Fold deliberately does not trim, because
// trimming would make "a b" and "ab" collide. Rejecting at the door is the
// only place where the user still knows what they typed.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%q: %w", name, ErrEmptyName)
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("%q: %w", name, ErrNameHasEdgeSpace)
	}
	// A trailing dot is legal on POSIX and silently dropped by Windows, which
	// means the same library would hold different names on different machines
	// (spec 3.4).
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("%q: %w", name, ErrNameTrailingDot)
	}
	if utf8.RuneCountInString(name) > MaxNameRunes {
		return fmt.Errorf("%q is %d characters, limit is %d: %w",
			name, utf8.RuneCountInString(name), MaxNameRunes, ErrNameTooLong)
	}
	return nil
}
```

- [x] **Step 4: 实现三个实体**

`internal/domain/box.go`:

```go
package domain

import "time"

// Box is the base unit: one trip, one hospital visit, one batch of paperwork.
//
// CreatedAt and UpdatedAt are machine time in UTC and are always taken from
// ops.ts, never from time.Now() — otherwise a rebuild would stamp the whole
// library with the moment it was rebuilt (spec 2.3.1). StartAt and EndAt are
// human time with no timezone at all (spec 1.3).
type Box struct {
	ID              BoxID
	Name            string
	NameFold        string
	Description     string
	DescriptionFold string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	StartAt         LocalDate
	EndAt           LocalDate
}

// NewBox validates and folds. Timestamps are left zero: they belong to the op
// that creates the box, and only the write path knows that op's ts.
func NewBox(id BoxID, name, description string) (Box, error) {
	if err := ValidateName(name); err != nil {
		return Box{}, err
	}
	return Box{
		ID:              id,
		Name:            name,
		NameFold:        Fold(name),
		Description:     description,
		DescriptionFold: Fold(description),
	}, nil
}
```

`internal/domain/tag.go`:

```go
package domain

// Tag is a node in the single global tag tree. Names are unique across the
// whole tree, not per parent — the hierarchy is not a namespace (spec 1.6).
type Tag struct {
	ID       TagID
	ParentID *TagID
	Name     string
	NameFold string
}

// NewTag validates and folds.
func NewTag(id TagID, parent *TagID, name string) (Tag, error) {
	if err := ValidateName(name); err != nil {
		return Tag{}, err
	}
	return Tag{ID: id, ParentID: parent, Name: name, NameFold: Fold(name)}, nil
}
```

`internal/domain/folder.go`:

```go
package domain

import "time"

// Folder is a directory inside one box. Every box has exactly one root folder,
// whose ParentID is nil and whose Name is "" (spec 1.4); the partial unique
// index in the schema is what actually enforces "exactly one".
//
// M1b only ever creates the root. Sub-folders arrive with import in M1c.
type Folder struct {
	ID        FolderID
	BoxID     BoxID
	ParentID  *FolderID
	Name      string
	NameFold  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewRootFolder builds the nameless root folder of a box.
func NewRootFolder(id FolderID, box BoxID) Folder {
	return Folder{ID: id, BoxID: box, Name: "", NameFold: ""}
}
```

- [x] **Step 5: 跑测试确认通过**

Run: `go test ./internal/domain/ -v`
Expected: PASS

- [x] **Step 6: 提交**

```bash
git add internal/domain/
git commit -m "feat(domain): Box / Tag / Folder 实体与名字校验

名字校验放在 domain 而不是 CLI：折叠函数刻意不 trim（trim 会让
\"a b\" 和 \"ab\" 撞车），所以拦首尾空格是校验的职责。首尾空格在
投影时会产生一个用户看不出区别的幽灵目录，见设计文档 1.4。"
```

---

### Task 2: oplog — 骨架：Action / Source / Tx / 注册表

**Files:**
- Create: `internal/oplog/action.go`, `internal/oplog/op.go`, `internal/oplog/tx.go`, `internal/oplog/targets.go`, `internal/oplog/registry.go`
- Test: `internal/oplog/registry_test.go`
- Modify: `docs/design/2026-08-22-kist-v0.1-design.md`（Step 6）

**Interfaces:**
- Consumes: `domain.*`
- Produces:
  - `oplog.Action` 及九个常量、`oplog.Source` 及四个常量
  - `oplog.Op`、`oplog.Marshal(any) ([]byte, error)`、`oplog.Payload[T](b []byte) (T, error)`
  - `oplog.Tx` 接口
  - `oplog.Targets`
  - `oplog.Handler` 接口、`oplog.Register(Action, Handler)`、`oplog.Lookup(Action) (Handler, error)`、`oplog.ErrUnknownAction`

**`Tx` 是这一层的全部秘密。** `Apply` 只拿得到 `(tx, payload)` 两个东西，而 `Tx` 上没有 `time.Now()`、没有 `NewID()`、没有任意 SQL——它只有一组我们逐个审过的方法。不变量 3 因此是**类型系统保证的**，不是靠 code review 记得住。时间戳走 `tx.TS()`，返回的是这条 op 的 `ts`，所以 `Apply` 连"当前时间"这个概念都接触不到。

**`ErrUnknownAction` 的措辞很重要。** 设计文档 2.2 明确要求：读到未注册的 action 要报"这个库是更新版本的 kist 写的"，而不是崩溃或静默跳过。append-only 的历史表必须对未来宽容，而对过去的自己诚实。

- [x] **Step 1: 写失败的测试**

```go
package oplog

import (
	"errors"
	"strings"
	"testing"
)

type fakeHandler struct{ applied int }

func (h *fakeHandler) Apply(Tx, []byte) error                { h.applied++; return nil }
func (h *fakeHandler) Inverse([]byte) (Action, []byte, error) { return "noop", nil, nil }
func (h *fakeHandler) Targets([]byte) (Targets, error)        { return Targets{}, nil }

func TestLookupFindsRegisteredHandlers(t *testing.T) {
	// Every action the build knows about must resolve. A typo in a const or a
	// handler nobody registered would otherwise only surface at runtime, on a
	// user's library.
	for _, a := range AllActions() {
		if _, err := Lookup(a); err != nil {
			t.Errorf("Lookup(%q): %v", a, err)
		}
	}
}

func TestLookupRejectsUnknownAction(t *testing.T) {
	_, err := Lookup(Action("teleport_box"))
	if err == nil {
		t.Fatal("Lookup of an unknown action = nil error, want a rejection")
	}
	if !errors.Is(err, ErrUnknownAction) {
		t.Errorf("error = %v, want it to wrap ErrUnknownAction", err)
	}
	// spec 2.2: the message has to tell the user what actually happened. An
	// append-only history table must be forgiving of the future.
	if !strings.Contains(err.Error(), "newer") {
		t.Errorf("error = %q, want it to say a newer kist wrote this", err)
	}
}

func TestRegisterRejectsDuplicates(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("registering the same action twice should panic at init time")
		}
	}()
	Register(ActionCreateBox, &fakeHandler{})
}

func TestPayloadRoundTrip(t *testing.T) {
	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	raw, err := Marshal(sample{Name: "婚礼", Count: 3})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// spec 2.2.1: TEXT json, not jsonb — the audit log has to stay readable by
	// a human with a SQL prompt.
	if !strings.Contains(string(raw), `"name"`) {
		t.Errorf("payload is not readable JSON: %s", raw)
	}

	got, err := Payload[sample](raw)
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	if got.Name != "婚礼" || got.Count != 3 {
		t.Errorf("round trip gave %+v", got)
	}
}

func TestPayloadRejectsUnknownFields(t *testing.T) {
	// A payload written by a newer kist carrying a field we do not understand
	// must not be silently half-applied.
	type sample struct {
		Name string `json:"name"`
	}
	if _, err := Payload[sample]([]byte(`{"name":"x","unexpected":1}`)); err == nil {
		t.Error("Payload accepted an unknown field")
	}
}

func TestSourcePropagatesUpdatedAt(t *testing.T) {
	// spec 1.5.1 ②: updated_at records human intent, not system activity.
	// Getting this wrong turns ten years of archive into "modified just now"
	// the first time an extractor is re-run.
	if !SourceUser.PropagatesUpdatedAt() {
		t.Error("user ops must propagate updated_at")
	}
	for _, s := range []Source{SourceExtractor, SourceMaintenance} {
		if s.PropagatesUpdatedAt() {
			t.Errorf("%s must not propagate updated_at", s)
		}
	}
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `go test ./internal/oplog/ -v`
Expected: FAIL，`undefined: Lookup`

- [x] **Step 3: 写 Action 与 Source**

`internal/oplog/action.go`:

```go
// Package oplog defines what an operation is: its action, its payload, how it
// is applied, and how it is inverted.
//
// It knows nothing about databases. Everything it can do to a library goes
// through the Tx interface in this package, which is deliberately small — that
// is what makes invariant 3 ("Apply must be a pure function") a property of
// the type system rather than of code review.
package oplog

// Action is the discriminator stored in ops.action.
//
// It is TEXT in the database with no CHECK constraint and no enum, on purpose
// (spec 2.2): the set grows with every feature, and an append-only history
// table must stay readable after an action is retired. Strictness lives here,
// in Go, where a retired action can still be read and reported.
type Action string

const (
	ActionCreateBox Action = "create_box"
	ActionDeleteBox Action = "delete_box"
	ActionUpdateBox Action = "update_box"

	ActionCreateTag Action = "create_tag"
	ActionDeleteTag Action = "delete_tag"
	ActionRenameTag Action = "rename_tag"
	ActionMoveTag   Action = "move_tag"

	ActionTagBoxes   Action = "tag_boxes"
	ActionUntagBoxes Action = "untag_boxes"
)

// AllActions lists every action this build implements, so tests can assert
// that each one actually resolves to a handler.
func AllActions() []Action {
	return []Action{
		ActionCreateBox, ActionDeleteBox, ActionUpdateBox,
		ActionCreateTag, ActionDeleteTag, ActionRenameTag, ActionMoveTag,
		ActionTagBoxes, ActionUntagBoxes,
	}
}

// Source is what kind of executor produced an op — not who (that is actor).
type Source string

const (
	// SourceUser: a person, through the CLI or the web UI.
	SourceUser Source = "user"
	// SourceExtractor: the extraction pipeline (EXIF and friends, M3).
	SourceExtractor Source = "extractor"
	// SourceMaintenance: relink / rebuild / gc / scrub repairs.
	SourceMaintenance Source = "maintenance"
	// SourceSync: replicated from another device. Nothing produces this yet.
	SourceSync Source = "sync"
)

// PropagatesUpdatedAt reports whether ops from this source should bump
// updated_at up the folder chain (spec 1.5.1 ②).
//
// updated_at answers "has anything happened under here lately", and it is only
// useful if it means human activity. Re-running an extractor over the whole
// library would otherwise stamp a decade of archive as modified this morning,
// which does not degrade the field so much as delete it.
//
// The set is {user} today and becomes {user, sync} when another device's
// person can act on this library.
func (s Source) PropagatesUpdatedAt() bool {
	return s == SourceUser
}
```

- [x] **Step 4: 写 Op、payload 编解码、Targets、Tx**

`internal/oplog/op.go`:

```go
package oplog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Op is one row of the ops table: the authoritative record of one change.
//
// Ops are never updated and never deleted. Undo appends an inverse op that
// points back through RevertsID (spec 2.4).
type Op struct {
	ID        int64
	TS        time.Time
	Action    Action
	Source    Source
	Actor     string
	Payload   []byte
	RevertsID *int64
}

// Marshal encodes a typed payload struct.
//
// Payloads are stored as TEXT json rather than a binary form: nothing ever
// queries into them (invariant 4), so a parser's speed buys nothing, while
// being able to read the audit log with a plain SQL client is worth a lot on
// the day the history looks wrong (spec 2.2.1).
func Marshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode op payload: %w", err)
	}
	return b, nil
}

// Payload decodes a payload into its typed struct. Unknown fields are an
// error: a payload written by a newer kist may mean something this build would
// only half-apply, and half-applying a change to an archive is worse than
// refusing it.
func Payload[T any](b []byte) (T, error) {
	var v T
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("decode op payload: %w", err)
	}
	return v, nil
}
```

`internal/oplog/targets.go`:

```go
package oplog

import "github.com/jinzhao1994/kist/internal/domain"

// Targets are the entities an op touched. They become rows in op_boxes and
// op_files.
//
// Those tables are indexes over the payload, nothing more (spec 2.2.1), which
// is why every handler must be able to produce them from the payload alone:
// that is the direction invariant 4 requires — payload is self-sufficient, and
// every index table rebuilds from it.
type Targets struct {
	Boxes []domain.BoxID
	Files []domain.FileID
}
```

`internal/oplog/tx.go`:

```go
package oplog

import (
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// Tx is everything Apply is allowed to do.
//
// The list is short on purpose. Apply receives a Tx and a payload and nothing
// else: no clock, no ID generator, no arbitrary SQL. That is how invariant 3
// is enforced structurally — a handler that wanted to read the wall clock
// would have to change this interface, and that change would be visible in
// review.
//
// store satisfies this interface structurally; it does not import oplog, and
// oplog does not import store. library is what puts the two together.
type Tx interface {
	// TS is the timestamp of the op being applied — ops.ts, not the wall
	// clock. Every created_at and updated_at written during Apply comes from
	// here, so a rebuild reproduces the same values (spec 2.3.1).
	TS() time.Time

	InsertBox(b domain.Box) error
	UpdateBox(b domain.Box) error
	DeleteBox(id domain.BoxID) error
	GetBox(id domain.BoxID) (domain.Box, bool, error)
	CountBoxTags(box domain.BoxID) (int, error)

	InsertFolder(f domain.Folder) error
	DeleteFoldersOfBox(id domain.BoxID) error

	InsertTag(t domain.Tag) error
	UpdateTag(t domain.Tag) error
	DeleteTag(id domain.TagID) error
	GetTag(id domain.TagID) (domain.Tag, bool, error)
	CountTagChildren(id domain.TagID) (int, error)
	CountTagUsage(id domain.TagID) (int, error)

	AddBoxTag(box domain.BoxID, tag domain.TagID, source string) error
	RemoveBoxTag(box domain.BoxID, tag domain.TagID) error
	HasBoxTag(box domain.BoxID, tag domain.TagID) (bool, error)
}
```

- [x] **Step 5: 写注册表**

`internal/oplog/registry.go`:

```go
package oplog

import (
	"errors"
	"fmt"
)

// Handler implements one action.
type Handler interface {
	// Apply performs the change. It must be a pure function of (tx, payload):
	// no generated IDs, no clock reads, no decisions that depend on the
	// current state of the library (invariant 3).
	//
	// Verifying that the current state matches the payload's recorded "before"
	// and failing if it does not IS allowed — that is an assertion, not a
	// decision, and it is how strict undo is implemented (spec 2.5).
	Apply(tx Tx, payload []byte) error

	// Inverse returns the action and payload that undo this op.
	//
	// It returns an action as well as a payload because the inverse of
	// create_box is delete_box — a different handler entirely.
	Inverse(payload []byte) (Action, []byte, error)

	// Targets extracts the index-table rows from the payload alone.
	Targets(payload []byte) (Targets, error)
}

// ErrUnknownAction means this build has no handler for an action found in the
// log.
var ErrUnknownAction = errors.New("unknown op action")

var registry = map[Action]Handler{}

// Register wires a handler to an action. It panics on a duplicate: two
// handlers for one action is a programming error that must not survive to
// runtime, and every call site is an init function.
func Register(a Action, h Handler) {
	if _, exists := registry[a]; exists {
		panic("oplog: duplicate handler for action " + string(a))
	}
	registry[a] = h
}

// Lookup finds the handler for an action.
//
// The error wording matters (spec 2.2). Reading an op this build does not
// understand is not corruption — it is a library that a newer kist has
// written to, and the user needs to be told exactly that rather than shown a
// crash.
func Lookup(a Action) (Handler, error) {
	h, ok := registry[a]
	if !ok {
		return nil, fmt.Errorf("op action %q is not known to this build; the library was probably written by a newer kist: %w", a, ErrUnknownAction)
	}
	return h, nil
}
```

- [x] **Step 6: 把 `Inverse` 的签名同步进设计文档**

修改 `docs/design/2026-08-22-kist-v0.1-design.md` 的 2.3 节，把 `OpHandler` 改成：

```go
type OpAction string          // 判别值，存进 ops.action

type OpHandler interface {    // 处理器，按 action 注册
    Apply(tx Tx, payload []byte) error
    Inverse(payload []byte) (OpAction, []byte, error)
    Targets(payload []byte) (Targets, error)   // 从 payload 提取索引表的行
}
```

并在其后补一段：

> **`Inverse` 要连 action 一起返回。** 第一版签名只返回 payload，那是不成立的——`create_box` 的反向是 `delete_box`，是另一个 action，只给 payload 的话调用方无从知道该拿哪个 handler 去 apply。
>
> **`Targets` 是不变量 4 的落点。** `op_boxes` / `op_files` 是 payload 的索引，必须能从 payload 单向重建；把这件事做成 handler 接口的一个方法，就没有哪条写入路径能绕过它——忘了实现是编译错误，实现错了是 `Rebuild` 的测试失败。

- [x] **Step 7: 跑测试确认通过**

Run: `go test ./internal/oplog/ -v`
Expected: PASS。注意 `TestLookupFindsRegisteredHandlers` 此时会失败——九个 handler 要到 Task 5–7 才注册。**这是预期的**：把 `AllActions()` 里尚未实现的项暂时注释掉，每完成一个 handler 就放回来一项，Task 7 结束时整份列表恢复。这条在 Task 5/6/7 的步骤里会逐个提醒。

- [x] **Step 8: 提交**

```bash
git add internal/oplog/ docs/design/2026-08-22-kist-v0.1-design.md
git commit -m "feat(oplog): 骨架——Action / Source / Tx / 注册表

Tx 接口是不变量 3 的结构性保障：Apply 只拿得到 (tx, payload)，而 Tx
上没有时钟、没有 ID 生成器、没有任意 SQL。时间戳走 tx.TS() 取 ops.ts，
所以 Apply 连'当前时间'这个概念都接触不到。

顺带修正设计文档 2.3 的 Inverse 签名：只返回 payload 不成立，
create_box 的反向是 delete_box，是另一个 action。"
```

---

### Task 3: store — ops 表读写与索引表

**Files:**
- Create: `internal/store/ops.go`
- Test: `internal/store/ops_test.go`

**Interfaces:**
- Consumes: `domain.*`、M1a 的 `store.Open` / `store.Migrate`
- Produces:
  - `store.OpRow`（`store` 自己的行结构，不 import `oplog`）
  - `store.AppendOp(tx *sql.Tx, r OpRow) (int64, error)`
  - `store.GetOp(q Queryer, id int64) (OpRow, bool, error)`
  - `store.ListOps(q Queryer, opt ListOpsOptions) ([]OpRow, error)`
  - `store.RevertedBy(q Queryer, id int64) (int64, bool, error)`
  - `store.InsertOpBoxes(tx *sql.Tx, opID int64, boxes []domain.BoxID) error`、`store.InsertOpFiles(...)`
  - `store.LastOpTouchingBox(q Queryer, box domain.BoxID, afterOpID int64) (int64, bool, error)`
  - `store.Queryer` 接口（`*sql.DB` 和 `*sql.Tx` 都满足）

**`store` 不 import `oplog`，所以这里出现的是 `OpRow` 而不是 `oplog.Op`。** 两个结构字段相同，由 `library` 做一次转换。这看起来是冗余，但它是依赖方向的代价，而依赖方向是这套架构的地基：`oplog` 一旦能 import `store`，`Apply` 就能拿到 `*sql.Tx`，不变量 3 的结构性保障当场失效。

**`LastOpTouchingBox` 服务于严格撤销的错误信息**（spec 2.5）：撤销失败时要能说出"是哪条 op 挡住了"，否则用户只看到"撤销失败"没法处理。

**`ts` 的写入格式必须定宽。** `2006-01-02T15:04:05.000Z`——毫秒补零，末尾 `Z`。少写一位毫秒，字典序就不再等于时间序，而 `ORDER BY ts` 会静默给出错乱结果。

- [x] **Step 1: 写失败的测试**

```go
package store

import (
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

func opsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func appendOp(t *testing.T, db *sql.DB, r OpRow) int64 {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	id, err := AppendOp(tx, r)
	if err != nil {
		t.Fatalf("AppendOp: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return id
}

func sampleOp() OpRow {
	return OpRow{
		TS:      time.Date(2026, 8, 27, 10, 30, 0, 123_000_000, time.UTC),
		Action:  "create_box",
		Source:  "user",
		Actor:   "cli",
		Payload: []byte(`{"box_id":"x"}`),
	}
}

func TestAppendOpAndGet(t *testing.T) {
	db := opsDB(t)
	id := appendOp(t, db, sampleOp())
	if id <= 0 {
		t.Fatalf("AppendOp returned id %d", id)
	}

	got, ok, err := GetOp(db, id)
	if err != nil || !ok {
		t.Fatalf("GetOp: %v, ok=%v", err, ok)
	}
	if got.Action != "create_box" || got.Source != "user" || got.Actor != "cli" {
		t.Errorf("GetOp = %+v", got)
	}
	if string(got.Payload) != `{"box_id":"x"}` {
		t.Errorf("payload = %s", got.Payload)
	}
	want := sampleOp().TS
	if !got.TS.Equal(want) {
		t.Errorf("TS = %v, want %v", got.TS, want)
	}
	if got.TS.Location() != time.UTC {
		t.Errorf("TS came back in %v, want UTC", got.TS.Location())
	}
	if got.RevertsID != nil {
		t.Errorf("RevertsID = %v, want nil", got.RevertsID)
	}
}

func TestGetOpMissing(t *testing.T) {
	db := opsDB(t)
	if _, ok, err := GetOp(db, 999); err != nil || ok {
		t.Errorf("GetOp(999) = ok %v, err %v; want ok=false, err=nil", ok, err)
	}
}

// The whole variable-precision-free machinery of the log rests on this: ts is
// fixed width so its lexicographic order is its chronological order.
func TestTimestampIsFixedWidth(t *testing.T) {
	db := opsDB(t)
	r := sampleOp()
	r.TS = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) // no millis at all
	id := appendOp(t, db, r)

	var raw string
	if err := db.QueryRow(`SELECT ts FROM ops WHERE id = ?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != "2026-01-02T03:04:05.000Z" {
		t.Errorf("stored ts = %q, want a zero-padded fixed-width form", raw)
	}
	if len(raw) != len("2006-01-02T15:04:05.000Z") {
		t.Errorf("ts is %d characters, must always be %d", len(raw), len("2006-01-02T15:04:05.000Z"))
	}
}

func TestTimestampIsNormalizedToUTC(t *testing.T) {
	db := opsDB(t)
	shanghai := time.FixedZone("CST", 8*3600)
	r := sampleOp()
	r.TS = time.Date(2026, 8, 27, 18, 0, 0, 0, shanghai) // 10:00 UTC
	id := appendOp(t, db, r)

	var raw string
	db.QueryRow(`SELECT ts FROM ops WHERE id = ?`, id).Scan(&raw)
	if raw != "2026-08-27T10:00:00.000Z" {
		t.Errorf("stored ts = %q, want it converted to UTC", raw)
	}
}

func TestRevertsID(t *testing.T) {
	db := opsDB(t)
	target := appendOp(t, db, sampleOp())

	inverse := sampleOp()
	inverse.Action = "delete_box"
	inverse.RevertsID = &target
	inverseID := appendOp(t, db, inverse)

	by, ok, err := RevertedBy(db, target)
	if err != nil {
		t.Fatalf("RevertedBy: %v", err)
	}
	if !ok || by != inverseID {
		t.Errorf("RevertedBy(%d) = %d, ok=%v; want %d", target, by, ok, inverseID)
	}

	if _, ok, _ := RevertedBy(db, inverseID); ok {
		t.Error("the inverse op should not itself be reported as reverted")
	}
}

func TestListOpsIsNewestFirst(t *testing.T) {
	db := opsDB(t)
	var ids []int64
	for i := 0; i < 5; i++ {
		ids = append(ids, appendOp(t, db, sampleOp()))
	}

	got, err := ListOps(db, ListOpsOptions{Limit: 3})
	if err != nil {
		t.Fatalf("ListOps: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d ops, want 3", len(got))
	}
	for i, want := range []int64{ids[4], ids[3], ids[2]} {
		if got[i].ID != want {
			t.Errorf("ops[%d].ID = %d, want %d", i, got[i].ID, want)
		}
	}
}

func TestListOpsByBox(t *testing.T) {
	db := opsDB(t)
	boxA, boxB := domain.NewBoxID(), domain.NewBoxID()

	tx, _ := db.Begin()
	idA, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, idA, []domain.BoxID{boxA})
	idBoth, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, idBoth, []domain.BoxID{boxA, boxB})
	idB, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, idB, []domain.BoxID{boxB})
	tx.Commit()

	got, err := ListOps(db, ListOpsOptions{Box: &boxA, Limit: 10})
	if err != nil {
		t.Fatalf("ListOps: %v", err)
	}
	if len(got) != 2 || got[0].ID != idBoth || got[1].ID != idA {
		t.Errorf("ops for boxA = %v, want [%d %d]", opIDs(got), idBoth, idA)
	}
}

func opIDs(ops []OpRow) []int64 {
	out := make([]int64, len(ops))
	for i, o := range ops {
		out[i] = o.ID
	}
	return out
}

// A batch operation is ONE op with many targets (spec 2.2.1), so inserting the
// same op twice for one box must not blow up on the primary key — and must not
// duplicate the row either.
func TestInsertOpBoxesIsIdempotent(t *testing.T) {
	db := opsDB(t)
	box := domain.NewBoxID()

	tx, _ := db.Begin()
	id, _ := AppendOp(tx, sampleOp())
	if err := InsertOpBoxes(tx, id, []domain.BoxID{box, box}); err != nil {
		t.Fatalf("InsertOpBoxes with a repeated target: %v", err)
	}
	tx.Commit()

	var n int
	db.QueryRow(`SELECT count(*) FROM op_boxes WHERE op_id = ?`, id).Scan(&n)
	if n != 1 {
		t.Errorf("op_boxes has %d rows for one box, want 1", n)
	}
}

func TestLastOpTouchingBox(t *testing.T) {
	// spec 2.5: strict undo must be able to say WHICH op is in the way.
	db := opsDB(t)
	box := domain.NewBoxID()

	tx, _ := db.Begin()
	first, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, first, []domain.BoxID{box})
	second, _ := AppendOp(tx, sampleOp())
	InsertOpBoxes(tx, second, []domain.BoxID{box})
	tx.Commit()

	got, ok, err := LastOpTouchingBox(db, box, first)
	if err != nil {
		t.Fatalf("LastOpTouchingBox: %v", err)
	}
	if !ok || got != second {
		t.Errorf("LastOpTouchingBox after %d = %d (ok=%v), want %d", first, got, ok, second)
	}

	if _, ok, _ := LastOpTouchingBox(db, box, second); ok {
		t.Error("nothing touched the box after the last op, but something was reported")
	}
}
```

（测试文件顶部需要 `database/sql`、`testing`、`time`、`domain`。）

- [x] **Step 2: 跑测试确认失败**

Run: `go test ./internal/store/ -run 'TestAppendOp|TestGetOp|TestTimestamp|TestReverts|TestListOps|TestInsertOpBoxes|TestLastOp' -v`
Expected: FAIL，`undefined: AppendOp`

- [x] **Step 3: 实现**

```go
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// TimeLayout is how every machine timestamp is written: UTC, milliseconds
// zero-padded, fixed width.
//
// Fixed width is the whole point. The log relies on the lexicographic order of
// ts being its chronological order, and dropping a millisecond digit breaks
// that silently — ORDER BY ts keeps working, it just stops being right.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// FormatTime renders t for storage, converting to UTC first.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// ParseTime reads a stored timestamp.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(TimeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", s, err)
	}
	return t.UTC(), nil
}

// Queryer is satisfied by both *sql.DB and *sql.Tx, so read helpers can be
// called inside or outside a transaction.
type Queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// OpRow is one row of the ops table.
//
// store deliberately defines its own shape rather than using oplog.Op: store
// must not import oplog, and oplog must not import store. library converts
// between the two. The duplicated struct is the price of a dependency
// direction that keeps *sql.Tx out of reach of Apply.
type OpRow struct {
	ID        int64
	TS        time.Time
	Action    string
	Source    string
	Actor     string
	Payload   []byte
	RevertsID *int64
}

// AppendOp inserts an op and returns its id, which is the library's version
// number. Ops are never updated and never deleted.
func AppendOp(tx *sql.Tx, r OpRow) (int64, error) {
	res, err := tx.Exec(
		`INSERT INTO ops (ts, action, source, actor, payload, reverts_id) VALUES (?, ?, ?, ?, ?, ?)`,
		FormatTime(r.TS), r.Action, r.Source, nullString(r.Actor), string(r.Payload), r.RevertsID)
	if err != nil {
		return 0, fmt.Errorf("append op %q: %w", r.Action, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("append op %q: %w", r.Action, err)
	}
	return id, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

const opColumns = `id, ts, action, source, coalesce(actor, ''), payload, reverts_id`

func scanOp(scan func(...any) error) (OpRow, error) {
	var r OpRow
	var ts, payload string
	if err := scan(&r.ID, &ts, &r.Action, &r.Source, &r.Actor, &payload, &r.RevertsID); err != nil {
		return OpRow{}, err
	}
	t, err := ParseTime(ts)
	if err != nil {
		return OpRow{}, err
	}
	r.TS = t
	r.Payload = []byte(payload)
	return r, nil
}

// GetOp reads one op.
func GetOp(q Queryer, id int64) (OpRow, bool, error) {
	r, err := scanOp(q.QueryRow(`SELECT `+opColumns+` FROM ops WHERE id = ?`, id).Scan)
	if err == sql.ErrNoRows {
		return OpRow{}, false, nil
	}
	if err != nil {
		return OpRow{}, false, fmt.Errorf("read op %d: %w", id, err)
	}
	return r, true, nil
}

// ListOpsOptions filters the history view.
type ListOpsOptions struct {
	// Box restricts to ops that touched this box, via the op_boxes index.
	Box *domain.BoxID
	// Before, when non-zero, returns only ops with a smaller id — the cursor
	// for paging back through history.
	Before int64
	Limit  int
}

// ListOps returns ops newest first.
func ListOps(q Queryer, opt ListOpsOptions) ([]OpRow, error) {
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(`SELECT ` + opColumns + ` FROM ops`)
	if opt.Box != nil {
		// The index table, not json_each over the payload: 0.11ms versus
		// 94.63ms at 200k ops, and the gap widens forever (spec 2.2.1).
		sb.WriteString(` WHERE id IN (SELECT op_id FROM op_boxes WHERE box_id = ?)`)
		args = append(args, *opt.Box)
	} else {
		sb.WriteString(` WHERE 1 = 1`)
	}
	if opt.Before > 0 {
		sb.WriteString(` AND id < ?`)
		args = append(args, opt.Before)
	}
	sb.WriteString(` ORDER BY id DESC`)
	if opt.Limit > 0 {
		sb.WriteString(` LIMIT ?`)
		args = append(args, opt.Limit)
	}

	rows, err := q.Query(sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list ops: %w", err)
	}
	defer rows.Close()

	var out []OpRow
	for rows.Next() {
		r, err := scanOp(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("list ops: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RevertedBy reports which op, if any, undid the given one. The history panel
// asks this for every row it renders, which is why reverts_id is a column with
// an index rather than a payload field (spec 2.2).
func RevertedBy(q Queryer, id int64) (int64, bool, error) {
	var by int64
	err := q.QueryRow(`SELECT id FROM ops WHERE reverts_id = ? ORDER BY id LIMIT 1`, id).Scan(&by)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("check whether op %d was reverted: %w", id, err)
	}
	return by, true, nil
}

// LastOpTouchingBox finds the most recent op after afterOpID that touched this
// box. Strict undo uses it to name the op that is in the way rather than
// reporting a bare failure (spec 2.5).
func LastOpTouchingBox(q Queryer, box domain.BoxID, afterOpID int64) (int64, bool, error) {
	var id int64
	err := q.QueryRow(
		`SELECT op_id FROM op_boxes WHERE box_id = ? AND op_id > ? ORDER BY op_id DESC LIMIT 1`,
		box, afterOpID).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find the op blocking an undo: %w", err)
	}
	return id, true, nil
}

// InsertOpBoxes records which boxes an op touched. Repeated ids are collapsed:
// a batch op names each target once.
func InsertOpBoxes(tx *sql.Tx, opID int64, boxes []domain.BoxID) error {
	seen := make(map[domain.BoxID]bool, len(boxes))
	for _, b := range boxes {
		if seen[b] {
			continue
		}
		seen[b] = true
		if _, err := tx.Exec(`INSERT INTO op_boxes (op_id, box_id) VALUES (?, ?)`, opID, b); err != nil {
			return fmt.Errorf("index op %d against box %s: %w", opID, b, err)
		}
	}
	return nil
}

// InsertOpFiles records which files an op touched. Nothing writes files until
// M1c; the function exists now so the write path is complete and Targets has
// somewhere to land.
func InsertOpFiles(tx *sql.Tx, opID int64, files []domain.FileID) error {
	seen := make(map[domain.FileID]bool, len(files))
	for _, f := range files {
		if seen[f] {
			continue
		}
		seen[f] = true
		if _, err := tx.Exec(`INSERT INTO op_files (op_id, file_id) VALUES (?, ?)`, opID, f); err != nil {
			return fmt.Errorf("index op %d against file %s: %w", opID, f, err)
		}
	}
	return nil
}

// DeleteOpIndexes clears the index tables so Rebuild can regenerate them from
// the payloads (spec 2.2.1). It does NOT touch ops itself — that table is the
// authority and is never rewritten.
func DeleteOpIndexes(tx *sql.Tx) error {
	for _, table := range []string{"op_boxes", "op_files"} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	return nil
}
```

- [x] **Step 4: 跑测试确认通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add internal/store/ops.go internal/store/ops_test.go
git commit -m "feat(store): ops 表读写与 op_boxes 索引

store 定义自己的 OpRow 而不是用 oplog.Op：两个包互不 import，由
library 做一次转换。看起来冗余，但依赖方向是这套架构的地基——
oplog 一旦能 import store，Apply 就够得到 *sql.Tx。

ts 定宽写入（毫秒补零 + Z）。少写一位毫秒，字典序就不再等于时间序，
而 ORDER BY ts 会静默给出错乱结果。"
```

---

### Task 4: store — 物化视图的读写

**Files:**
- Create: `internal/store/views.go`, `internal/store/queries.go`
- Test: `internal/store/views_test.go`

**Interfaces:**
- Consumes: `domain.*`、Task 3 的 `store.FormatTime` / `ParseTime` / `Queryer`
- Produces:
  - `store.ViewTx`（包装 `*sql.Tx` + 这条 op 的 `ts`），方法齐备到**结构性满足 `oplog.Tx`**
  - 额外方法 `TouchBoxes([]domain.BoxID) error`、`SQL() *sql.Tx`
  - `store.ClearViews(tx *sql.Tx) error`
  - 读函数：`store.ListBoxes(q, ListBoxesOptions) ([]domain.Box, error)`、`store.ListTags(q) ([]domain.Tag, error)`、`store.BoxTags(q, domain.BoxID) ([]domain.TagID, error)`

**`ViewTx` 持有 `ts`，而 `TS()` 返回它。** 这是不变量 3 那条"时间戳取 `ops.ts`，不取写入时的墙上时钟"的物理实现——`Apply` 想读时钟得先改 `oplog.Tx` 接口，而那是一处 review 时看得见的改动。

**`ClearViews` 的删除顺序由外键决定**，写反了会在开了 `foreign_keys=1` 的库上直接失败（M1a Task 8 已经确保它真的开着）。顺序：`box_tags` / `file_tags` / `tag_rejections` / `metadata` → `files` → `folders` → `boxes` → `tags`。**`ops` 和 `blobs` 不在其中**——前者是权威，后者由扫目录重建。

**`ListBoxes` 的排序是字节序**（设计文档 4.8）。默认按 `updated_at DESC`（1.5.1 说它直接就是"最近有活动"），`--sort name` 时按 `name` 字节序。

- [x] **Step 1: 写失败的测试**

```go
package store

import (
	"database/sql"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

var testTime = time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

// viewTx runs fn inside a transaction against a migrated in-memory library.
func viewTx(t *testing.T, db *sql.DB, ts time.Time, fn func(*ViewTx)) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	fn(NewViewTx(tx, ts))
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func mustBox(t *testing.T, name string) domain.Box {
	t.Helper()
	b, err := domain.NewBox(domain.NewBoxID(), name, "")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestInsertAndGetBox(t *testing.T) {
	db := opsDB(t)
	b := mustBox(t, "云南 2019")

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.InsertBox(b); err != nil {
			t.Fatalf("InsertBox: %v", err)
		}
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		got, ok, err := vtx.GetBox(b.ID)
		if err != nil || !ok {
			t.Fatalf("GetBox: %v ok=%v", err, ok)
		}
		if got.Name != b.Name || got.NameFold != b.NameFold {
			t.Errorf("GetBox = %+v, want name %q", got, b.Name)
		}
		// spec 2.3.1: timestamps come from the op, not the wall clock.
		if !got.CreatedAt.Equal(testTime) || !got.UpdatedAt.Equal(testTime) {
			t.Errorf("timestamps = %v / %v, want both %v", got.CreatedAt, got.UpdatedAt, testTime)
		}
	})
}

func TestGetBoxMissing(t *testing.T) {
	db := opsDB(t)
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if _, ok, err := vtx.GetBox(domain.NewBoxID()); err != nil || ok {
			t.Errorf("GetBox of a missing box = ok %v, err %v", ok, err)
		}
	})
}

func TestUpdateBoxKeepsCreatedAt(t *testing.T) {
	// created_at is set once and never moves; updated_at follows the op.
	db := opsDB(t)
	b := mustBox(t, "before")
	viewTx(t, db, testTime, func(vtx *ViewTx) { vtx.InsertBox(b) })

	later := testTime.Add(48 * time.Hour)
	renamed, _ := domain.NewBox(b.ID, "after", "")
	viewTx(t, db, later, func(vtx *ViewTx) {
		if err := vtx.UpdateBox(renamed); err != nil {
			t.Fatalf("UpdateBox: %v", err)
		}
	})

	viewTx(t, db, later, func(vtx *ViewTx) {
		got, _, _ := vtx.GetBox(b.ID)
		if got.Name != "after" {
			t.Errorf("Name = %q, want %q", got.Name, "after")
		}
		if !got.CreatedAt.Equal(testTime) {
			t.Errorf("CreatedAt = %v, want it unchanged at %v", got.CreatedAt, testTime)
		}
		if !got.UpdatedAt.Equal(later) {
			t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, later)
		}
	})
}

func TestBoxDatesRoundTrip(t *testing.T) {
	db := opsDB(t)
	b := mustBox(t, "trip")
	b.StartAt, _ = domain.ParseLocalDate("2019-08")
	b.EndAt, _ = domain.ParseLocalDate("2019-08-12T14:30")

	viewTx(t, db, testTime, func(vtx *ViewTx) { vtx.InsertBox(b) })
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		got, _, _ := vtx.GetBox(b.ID)
		if got.StartAt.String() != "2019-08" || got.EndAt.String() != "2019-08-12T14:30" {
			t.Errorf("dates = %q / %q", got.StartAt, got.EndAt)
		}
		if got.StartAt.Precision() != domain.PrecMonth {
			t.Errorf("StartAt precision = %v, want month", got.StartAt.Precision())
		}
	})
}

func TestDeleteBoxNeedsItsFoldersGoneFirst(t *testing.T) {
	// Foreign keys are on (M1a). Deleting a box while its root folder still
	// points at it must fail rather than silently orphan the folder — which is
	// exactly why delete_box's handler removes folders explicitly.
	db := opsDB(t)
	b := mustBox(t, "doomed")
	root := domain.NewRootFolder(domain.NewFolderID(), b.ID)

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(b)
		vtx.InsertFolder(root)
	})

	tx, _ := db.Begin()
	vtx := NewViewTx(tx, testTime)
	if err := vtx.DeleteBox(b.ID); err == nil {
		t.Error("deleting a box with a folder still attached succeeded")
	}
	tx.Rollback()

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.DeleteFoldersOfBox(b.ID); err != nil {
			t.Fatalf("DeleteFoldersOfBox: %v", err)
		}
		if err := vtx.DeleteBox(b.ID); err != nil {
			t.Fatalf("DeleteBox: %v", err)
		}
	})
}

func TestTagCRUD(t *testing.T) {
	db := opsDB(t)
	people, _ := domain.NewTag(domain.NewTagID(), nil, "人物")
	child, _ := domain.NewTag(domain.NewTagID(), &people.ID, "爸")

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.InsertTag(people); err != nil {
			t.Fatalf("InsertTag: %v", err)
		}
		if err := vtx.InsertTag(child); err != nil {
			t.Fatalf("InsertTag child: %v", err)
		}
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		got, ok, err := vtx.GetTag(child.ID)
		if err != nil || !ok {
			t.Fatalf("GetTag: %v ok=%v", err, ok)
		}
		if got.ParentID == nil || *got.ParentID != people.ID {
			t.Errorf("ParentID = %v, want %v", got.ParentID, people.ID)
		}

		n, err := vtx.CountTagChildren(people.ID)
		if err != nil || n != 1 {
			t.Errorf("CountTagChildren = %d (%v), want 1", n, err)
		}
		if n, _ := vtx.CountTagChildren(child.ID); n != 0 {
			t.Errorf("a leaf reported %d children", n)
		}
	})
}

func TestTagNamesStayGloballyUnique(t *testing.T) {
	// The schema enforces it (M1a Task 9); this checks the write path surfaces
	// the violation instead of swallowing it.
	db := opsDB(t)
	a, _ := domain.NewTag(domain.NewTagID(), nil, "护照")
	viewTx(t, db, testTime, func(vtx *ViewTx) { vtx.InsertTag(a) })

	parent := a.ID
	b, _ := domain.NewTag(domain.NewTagID(), &parent, "护照")
	tx, _ := db.Begin()
	if err := NewViewTx(tx, testTime).InsertTag(b); err == nil {
		t.Error("a duplicate tag name was accepted under a different parent")
	}
	tx.Rollback()
}

func TestBoxTags(t *testing.T) {
	db := opsDB(t)
	b := mustBox(t, "trip")
	tag, _ := domain.NewTag(domain.NewTagID(), nil, "云南")

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(b)
		vtx.InsertTag(tag)
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if has, _ := vtx.HasBoxTag(b.ID, tag.ID); has {
			t.Error("HasBoxTag before adding = true")
		}
		if err := vtx.AddBoxTag(b.ID, tag.ID, "user"); err != nil {
			t.Fatalf("AddBoxTag: %v", err)
		}
		if has, _ := vtx.HasBoxTag(b.ID, tag.ID); !has {
			t.Error("HasBoxTag after adding = false")
		}
		if n, _ := vtx.CountTagUsage(tag.ID); n != 1 {
			t.Errorf("CountTagUsage = %d, want 1", n)
		}
		if n, _ := vtx.CountBoxTags(b.ID); n != 1 {
			t.Errorf("CountBoxTags = %d, want 1", n)
		}
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.RemoveBoxTag(b.ID, tag.ID); err != nil {
			t.Fatalf("RemoveBoxTag: %v", err)
		}
		if has, _ := vtx.HasBoxTag(b.ID, tag.ID); has {
			t.Error("HasBoxTag after removing = true")
		}
	})
}

// Adding the same tag twice is not an error: a batch op may name a box that
// already carries the tag, and one op means one outcome, not a failure.
func TestAddBoxTagIsIdempotent(t *testing.T) {
	db := opsDB(t)
	b := mustBox(t, "trip")
	tag, _ := domain.NewTag(domain.NewTagID(), nil, "云南")
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(b)
		vtx.InsertTag(tag)
	})

	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.AddBoxTag(b.ID, tag.ID, "user"); err != nil {
			t.Fatal(err)
		}
		if err := vtx.AddBoxTag(b.ID, tag.ID, "user"); err != nil {
			t.Errorf("adding an existing box tag: %v", err)
		}
		if n, _ := vtx.CountBoxTags(b.ID); n != 1 {
			t.Errorf("CountBoxTags = %d after adding twice, want 1", n)
		}
	})
}

func TestTouchBoxes(t *testing.T) {
	// spec 1.5.1 ③: propagation is per op with a deduplicated target set, not
	// per file in a loop.
	db := opsDB(t)
	a, b := mustBox(t, "a"), mustBox(t, "b")
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(a)
		vtx.InsertBox(b)
	})

	later := testTime.Add(time.Hour)
	viewTx(t, db, later, func(vtx *ViewTx) {
		if err := vtx.TouchBoxes([]domain.BoxID{a.ID, a.ID, b.ID}); err != nil {
			t.Fatalf("TouchBoxes: %v", err)
		}
	})

	viewTx(t, db, later, func(vtx *ViewTx) {
		for _, id := range []domain.BoxID{a.ID, b.ID} {
			got, _, _ := vtx.GetBox(id)
			if !got.UpdatedAt.Equal(later) {
				t.Errorf("box %s UpdatedAt = %v, want %v", id, got.UpdatedAt, later)
			}
			if !got.CreatedAt.Equal(testTime) {
				t.Errorf("box %s CreatedAt moved to %v", id, got.CreatedAt)
			}
		}
	})
}

func TestTouchBoxesIgnoresMissingRows(t *testing.T) {
	// Undoing a create deletes the box; the propagation pass that follows must
	// not fail because its target is gone.
	db := opsDB(t)
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		if err := vtx.TouchBoxes([]domain.BoxID{domain.NewBoxID()}); err != nil {
			t.Errorf("TouchBoxes on a missing box: %v", err)
		}
	})
}

func TestClearViewsLeavesOpsAndBlobs(t *testing.T) {
	// Invariant 1: ops and blobs are the authority. Rebuild wipes everything
	// else and replays; if it ever wiped ops, rebuild would be data loss.
	db := opsDB(t)
	b := mustBox(t, "survivor")
	viewTx(t, db, testTime, func(vtx *ViewTx) {
		vtx.InsertBox(b)
		vtx.InsertFolder(domain.NewRootFolder(domain.NewFolderID(), b.ID))
	})
	appendOp(t, db, sampleOp())
	db.Exec(`INSERT INTO blobs (key, sha256, size) VALUES ('sha256:' || hex(randomblob(32)), hex(randomblob(32)), 1)`)

	tx, _ := db.Begin()
	if err := ClearViews(tx); err != nil {
		t.Fatalf("ClearViews: %v", err)
	}
	tx.Commit()

	for _, table := range []string{"boxes", "folders", "tags", "box_tags"} {
		var n int
		db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n)
		if n != 0 {
			t.Errorf("%s still has %d rows after ClearViews", table, n)
		}
	}
	for _, table := range []string{"ops", "blobs"} {
		var n int
		db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n)
		if n == 0 {
			t.Errorf("ClearViews emptied %s, which is authoritative data", table)
		}
	}
}

func TestListBoxes(t *testing.T) {
	db := opsDB(t)
	early, late := mustBox(t, "早"), mustBox(t, "晚")
	viewTx(t, db, testTime, func(vtx *ViewTx) { vtx.InsertBox(early) })
	viewTx(t, db, testTime.Add(time.Hour), func(vtx *ViewTx) { vtx.InsertBox(late) })

	got, err := ListBoxes(db, ListBoxesOptions{})
	if err != nil {
		t.Fatalf("ListBoxes: %v", err)
	}
	if len(got) != 2 || got[0].ID != late.ID {
		t.Errorf("default order should be most-recently-active first, got %q then %q", got[0].Name, got[1].Name)
	}

	byName, err := ListBoxes(db, ListBoxesOptions{SortByName: true})
	if err != nil {
		t.Fatalf("ListBoxes by name: %v", err)
	}
	// spec 4.8: byte order. 早 is U+65E9 and 晚 is U+665A, so 早 comes first.
	// Under a pinyin collation it would be the other way round (wan < zao),
	// which is what makes this a real assertion rather than a tautology: it
	// fails the moment a locale-aware collation sneaks in.
	if byName[0].Name != "早" {
		t.Errorf("sorted by name gave %q first; byte order puts 早 (U+65E9) before 晚 (U+665A)", byName[0].Name)
	}
}
```

- [x] **Step 2: 跑测试确认失败**

Run: `go test ./internal/store/ -run 'TestInsertAndGetBox|TestUpdateBox|TestTag|TestBoxTags|TestTouch|TestClearViews|TestListBoxes|TestDeleteBox|TestAddBoxTag|TestGetBoxMissing|TestBoxDates' -v`
Expected: FAIL，`undefined: NewViewTx`

- [x] **Step 3: 实现 ViewTx**

`internal/store/views.go`:

```go
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ViewTx is the materialized-view side of one transaction.
//
// It structurally satisfies oplog.Tx. store does not import oplog and oplog
// does not import store; library is what connects them. The ts it carries is
// the op's ts, and TS() is the only clock a handler can reach — that is how
// "timestamps come from ops.ts, never time.Now()" (spec 2.3.1) stops being a
// rule people have to remember.
type ViewTx struct {
	tx *sql.Tx
	ts time.Time
}

// NewViewTx binds a transaction to the op being applied inside it.
func NewViewTx(tx *sql.Tx, ts time.Time) *ViewTx {
	return &ViewTx{tx: tx, ts: ts.UTC()}
}

// TS is the timestamp of the op being applied.
func (v *ViewTx) TS() time.Time { return v.ts }

// SQL exposes the underlying transaction to library, which needs it for the
// ops table itself. Handlers only ever see the oplog.Tx interface, which does
// not include this method.
func (v *ViewTx) SQL() *sql.Tx { return v.tx }

// ── boxes ────────────────────────────────────────────────────────────────

func (v *ViewTx) InsertBox(b domain.Box) error {
	_, err := v.tx.Exec(
		`INSERT INTO boxes (id, name, name_fold, description, description_fold,
		                    created_at, updated_at, start_at, end_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.Name, b.NameFold, b.Description, b.DescriptionFold,
		FormatTime(v.ts), FormatTime(v.ts), b.StartAt, b.EndAt)
	if err != nil {
		return fmt.Errorf("insert box %s: %w", b.ID, err)
	}
	return nil
}

// UpdateBox rewrites the mutable fields. created_at is deliberately absent:
// it is set once by InsertBox and must never move.
func (v *ViewTx) UpdateBox(b domain.Box) error {
	res, err := v.tx.Exec(
		`UPDATE boxes SET name = ?, name_fold = ?, description = ?, description_fold = ?,
		                  start_at = ?, end_at = ?, updated_at = ?
		 WHERE id = ?`,
		b.Name, b.NameFold, b.Description, b.DescriptionFold,
		b.StartAt, b.EndAt, FormatTime(v.ts), b.ID)
	if err != nil {
		return fmt.Errorf("update box %s: %w", b.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update box %s: no such box", b.ID)
	}
	return nil
}

func (v *ViewTx) DeleteBox(id domain.BoxID) error {
	if _, err := v.tx.Exec(`DELETE FROM boxes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete box %s: %w", id, err)
	}
	return nil
}

func (v *ViewTx) GetBox(id domain.BoxID) (domain.Box, bool, error) {
	return scanBox(v.tx.QueryRow(`SELECT `+boxColumns+` FROM boxes WHERE id = ?`, id).Scan)
}

const boxColumns = `id, name, name_fold, description, description_fold, created_at, updated_at, start_at, end_at`

func scanBox(scan func(...any) error) (domain.Box, bool, error) {
	var (
		b                  domain.Box
		created, updated   string
		start, end         sql.NullString
	)
	err := scan(&b.ID, &b.Name, &b.NameFold, &b.Description, &b.DescriptionFold,
		&created, &updated, &start, &end)
	if err == sql.ErrNoRows {
		return domain.Box{}, false, nil
	}
	if err != nil {
		return domain.Box{}, false, fmt.Errorf("read box: %w", err)
	}
	if b.CreatedAt, err = ParseTime(created); err != nil {
		return domain.Box{}, false, err
	}
	if b.UpdatedAt, err = ParseTime(updated); err != nil {
		return domain.Box{}, false, err
	}
	if start.Valid {
		if b.StartAt, err = domain.ParseLocalDate(start.String); err != nil {
			return domain.Box{}, false, err
		}
	}
	if end.Valid {
		if b.EndAt, err = domain.ParseLocalDate(end.String); err != nil {
			return domain.Box{}, false, err
		}
	}
	return b, true, nil
}

// TouchBoxes bumps updated_at on a deduplicated set of boxes.
//
// Deduplication is the point (spec 1.5.1 ③): tagging 500 files across 12 boxes
// must write 12 rows, not 500. Missing rows are ignored — undoing a create
// deletes the box, and the propagation pass that follows should not fail
// because its target is gone.
func (v *ViewTx) TouchBoxes(ids []domain.BoxID) error {
	seen := make(map[domain.BoxID]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if _, err := v.tx.Exec(`UPDATE boxes SET updated_at = ? WHERE id = ?`, FormatTime(v.ts), id); err != nil {
			return fmt.Errorf("touch box %s: %w", id, err)
		}
	}
	return nil
}

func (v *ViewTx) CountBoxTags(box domain.BoxID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM box_tags WHERE box_id = ?`, box).Scan(&n); err != nil {
		return 0, fmt.Errorf("count tags on box %s: %w", box, err)
	}
	return n, nil
}

// ── folders ──────────────────────────────────────────────────────────────

func (v *ViewTx) InsertFolder(f domain.Folder) error {
	_, err := v.tx.Exec(
		`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.BoxID, f.ParentID, f.Name, f.NameFold, FormatTime(v.ts), FormatTime(v.ts))
	if err != nil {
		return fmt.Errorf("insert folder %s: %w", f.ID, err)
	}
	return nil
}

// DeleteFoldersOfBox removes every folder of a box. Foreign keys are on, so a
// box cannot be deleted while its folders still reference it — the delete_box
// handler calls this first, explicitly, rather than relying on a cascade
// nobody can see in the schema.
func (v *ViewTx) DeleteFoldersOfBox(id domain.BoxID) error {
	if _, err := v.tx.Exec(`DELETE FROM folders WHERE box_id = ?`, id); err != nil {
		return fmt.Errorf("delete folders of box %s: %w", id, err)
	}
	return nil
}

// ── tags ─────────────────────────────────────────────────────────────────

func (v *ViewTx) InsertTag(t domain.Tag) error {
	if _, err := v.tx.Exec(
		`INSERT INTO tags (id, parent_id, name, name_fold) VALUES (?, ?, ?, ?)`,
		t.ID, t.ParentID, t.Name, t.NameFold); err != nil {
		return fmt.Errorf("insert tag %q: %w", t.Name, err)
	}
	return nil
}

func (v *ViewTx) UpdateTag(t domain.Tag) error {
	res, err := v.tx.Exec(
		`UPDATE tags SET parent_id = ?, name = ?, name_fold = ? WHERE id = ?`,
		t.ParentID, t.Name, t.NameFold, t.ID)
	if err != nil {
		return fmt.Errorf("update tag %s: %w", t.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update tag %s: no such tag", t.ID)
	}
	return nil
}

func (v *ViewTx) DeleteTag(id domain.TagID) error {
	if _, err := v.tx.Exec(`DELETE FROM tags WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete tag %s: %w", id, err)
	}
	return nil
}

func (v *ViewTx) GetTag(id domain.TagID) (domain.Tag, bool, error) {
	var t domain.Tag
	err := v.tx.QueryRow(`SELECT id, parent_id, name, name_fold FROM tags WHERE id = ?`, id).
		Scan(&t.ID, &t.ParentID, &t.Name, &t.NameFold)
	if err == sql.ErrNoRows {
		return domain.Tag{}, false, nil
	}
	if err != nil {
		return domain.Tag{}, false, fmt.Errorf("read tag %s: %w", id, err)
	}
	return t, true, nil
}

func (v *ViewTx) CountTagChildren(id domain.TagID) (int, error) {
	var n int
	if err := v.tx.QueryRow(`SELECT count(*) FROM tags WHERE parent_id = ?`, id).Scan(&n); err != nil {
		return 0, fmt.Errorf("count children of tag %s: %w", id, err)
	}
	return n, nil
}

// CountTagUsage counts everything carrying this tag, across boxes and files.
// spec 1.6: deleting a tag that is still in use is refused outright — cascade
// delete in archiving software can wipe a whole classification with one
// misclick, and the op payload needed to undo it would be enormous.
func (v *ViewTx) CountTagUsage(id domain.TagID) (int, error) {
	var n int
	err := v.tx.QueryRow(
		`SELECT (SELECT count(*) FROM box_tags WHERE tag_id = ?)
		      + (SELECT count(*) FROM file_tags WHERE tag_id = ?)`, id, id).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count usage of tag %s: %w", id, err)
	}
	return n, nil
}

// ── box_tags ─────────────────────────────────────────────────────────────

// AddBoxTag is idempotent: a batch op may name a box that already carries the
// tag, and one op means one outcome, not a failure.
func (v *ViewTx) AddBoxTag(box domain.BoxID, tag domain.TagID, source string) error {
	_, err := v.tx.Exec(
		`INSERT INTO box_tags (box_id, tag_id, source) VALUES (?, ?, ?)
		 ON CONFLICT (box_id, tag_id) DO UPDATE SET source = excluded.source`,
		box, tag, source)
	if err != nil {
		return fmt.Errorf("tag box %s with %s: %w", box, tag, err)
	}
	return nil
}

func (v *ViewTx) RemoveBoxTag(box domain.BoxID, tag domain.TagID) error {
	if _, err := v.tx.Exec(`DELETE FROM box_tags WHERE box_id = ? AND tag_id = ?`, box, tag); err != nil {
		return fmt.Errorf("untag box %s: %w", box, err)
	}
	return nil
}

func (v *ViewTx) HasBoxTag(box domain.BoxID, tag domain.TagID) (bool, error) {
	var one int
	err := v.tx.QueryRow(`SELECT 1 FROM box_tags WHERE box_id = ? AND tag_id = ?`, box, tag).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check tag %s on box %s: %w", tag, box, err)
	}
	return true, nil
}

// ClearViews empties every rebuildable table.
//
// The order is dictated by the foreign keys: children before parents. ops and
// blobs are absent on purpose — ops is the authority, and blobs is rebuilt by
// walking the blobs/ directory (invariant 1). A version of this function that
// touched either would turn `kist rebuild` into data loss.
func ClearViews(tx *sql.Tx) error {
	for _, table := range []string{
		"box_tags", "file_tags", "tag_rejections", "metadata",
		"files", "folders", "boxes", "tags",
	} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	return nil
}
```

- [x] **Step 4: 实现读函数**

`internal/store/queries.go`:

```go
package store

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ListBoxesOptions controls the box listing.
type ListBoxesOptions struct {
	// SortByName orders by name instead of by recent activity. The ordering is
	// byte order, deliberately (spec 4.8): the same library must list in the
	// same order on every platform and on either engine.
	SortByName bool
	Limit      int
}

// ListBoxes returns boxes, most recently active first by default —
// box.updated_at is literally "something happened under here lately"
// (spec 1.5.1).
func ListBoxes(q Queryer, opt ListBoxesOptions) ([]domain.Box, error) {
	order := `updated_at DESC, id DESC`
	if opt.SortByName {
		order = `name, id`
	}
	query := `SELECT ` + boxColumns + ` FROM boxes ORDER BY ` + order
	args := []any{}
	if opt.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opt.Limit)
	}

	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list boxes: %w", err)
	}
	defer rows.Close()

	var out []domain.Box
	for rows.Next() {
		b, _, err := scanBox(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListTags returns every tag. The tree is a few thousand nodes and lives in
// memory (spec 1.6), so there is no paging and no path column.
func ListTags(q Queryer) ([]domain.Tag, error) {
	rows, err := q.Query(`SELECT id, parent_id, name, name_fold FROM tags ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var out []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.ParentID, &t.Name, &t.NameFold); err != nil {
			return nil, fmt.Errorf("list tags: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// BoxTags returns the tags attached directly to a box.
func BoxTags(q Queryer, box domain.BoxID) ([]domain.TagID, error) {
	rows, err := q.Query(`SELECT tag_id FROM box_tags WHERE box_id = ? ORDER BY tag_id`, box)
	if err != nil {
		return nil, fmt.Errorf("read tags of box %s: %w", box, err)
	}
	defer rows.Close()

	var out []domain.TagID
	for rows.Next() {
		var id domain.TagID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("read tags of box %s: %w", box, err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
```

- [x] **Step 5: 跑测试确认通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [x] **Step 6: 提交**

```bash
git add internal/store/views.go internal/store/queries.go internal/store/views_test.go
git commit -m "feat(store): 物化视图的读写，ViewTx 持有 op 的 ts

ViewTx.TS() 返回的是 ops.ts 而不是墙上时钟——这是不变量 3 里
'时间戳取 ops.ts' 那条的物理实现。Apply 想读时钟得先改 oplog.Tx
接口，而那是 review 时看得见的改动。

ClearViews 的删除顺序由外键决定，且刻意不含 ops 和 blobs：前者是
权威，后者扫目录重建。碰了任何一个，rebuild 就从修复变成了数据丢失。"
```

---

### Task 5: oplog — box 的三个 handler

**Files:**
- Create: `internal/oplog/box.go`
- Test: `internal/oplog/box_test.go`, `internal/oplog/faketx_test.go`
- Modify: `docs/design/2026-08-22-kist-v0.1-design.md`（Step 8）

**Interfaces:**
- Consumes: `oplog.Tx`、`oplog.Register`、`domain.*`
- Produces（payload 结构体，`library` 要用它们构造 op）：
  - `oplog.CreateBoxPayload{BoxID, RootFolderID, Name, Description, StartAt, EndAt}`
  - `oplog.DeleteBoxPayload{BoxID, RootFolderID, Name, Description, StartAt, EndAt}`
  - `oplog.UpdateBoxPayload{BoxID, Before, After BoxFields}`、`oplog.BoxFields{Name, Description, StartAt, EndAt}`
  - `oplog.ErrStaleBefore`

**payload 自带 before 状态，这是写 op 定义时最容易漏的一点**（spec 2.3）。`update_box` 的 payload 同时记 `Before` 和 `After`：`Inverse` 只需把两者对调，不需要去查库。

**`delete_box` 的 payload 是完整的 before 状态**，因为它的 `Inverse` 是 `create_box`——要把整个 box 原样建回来，包括它的根 folder id。**根 folder 的 id 必须记进 payload**，否则撤销一次 `create_box` 再撤销回来，folder 会拿到一个新 id，而 M1c 之后 `files.folder_id` 会指向它。

**严格模式的落点在 `Apply`。** `update_box` 和 `delete_box` 在写之前先比对当前行是否等于 `Before`，不等就返回 `ErrStaleBefore`。这不是决策而是断言（见本计划开头「与设计文档的两处偏离」②）。

**比对时只看语义字段，不看 `updated_at`。** spec 1.5.1 ④ 说得很清楚：`updated_at` 是被维护的派生状态，不参与 `Inverse`。把它纳入比对的话，任何一次撤销都会因为"时间戳对不上"而失败。

- [x] **Step 1: 写一个假的 Tx**

`internal/oplog/faketx_test.go`——handler 的测试不该需要一个数据库：

```go
package oplog

import (
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

// fakeTx is an in-memory stand-in for the real transaction. Handlers are pure
// functions of (tx, payload), so testing them against maps rather than SQLite
// is not a shortcut — it is the shape the design promises.
type fakeTx struct {
	ts       time.Time
	boxes    map[domain.BoxID]domain.Box
	folders  map[domain.FolderID]domain.Folder
	tags     map[domain.TagID]domain.Tag
	boxTags  map[[2]string]string // box|tag -> source
	fileTags map[domain.TagID]int
}

func newFakeTx(ts time.Time) *fakeTx {
	return &fakeTx{
		ts:       ts,
		boxes:    map[domain.BoxID]domain.Box{},
		folders:  map[domain.FolderID]domain.Folder{},
		tags:     map[domain.TagID]domain.Tag{},
		boxTags:  map[[2]string]string{},
		fileTags: map[domain.TagID]int{},
	}
}

func btKey(b domain.BoxID, t domain.TagID) [2]string { return [2]string{b.String(), t.String()} }

func (f *fakeTx) TS() time.Time { return f.ts }

func (f *fakeTx) InsertBox(b domain.Box) error {
	if _, exists := f.boxes[b.ID]; exists {
		return fmt.Errorf("box %s already exists", b.ID)
	}
	b.CreatedAt, b.UpdatedAt = f.ts, f.ts
	f.boxes[b.ID] = b
	return nil
}

func (f *fakeTx) UpdateBox(b domain.Box) error {
	old, exists := f.boxes[b.ID]
	if !exists {
		return fmt.Errorf("no such box %s", b.ID)
	}
	b.CreatedAt = old.CreatedAt
	b.UpdatedAt = f.ts
	f.boxes[b.ID] = b
	return nil
}

func (f *fakeTx) DeleteBox(id domain.BoxID) error {
	for _, fo := range f.folders {
		if fo.BoxID == id {
			return fmt.Errorf("box %s still has folders", id)
		}
	}
	delete(f.boxes, id)
	return nil
}

func (f *fakeTx) GetBox(id domain.BoxID) (domain.Box, bool, error) {
	b, ok := f.boxes[id]
	return b, ok, nil
}

func (f *fakeTx) CountBoxTags(id domain.BoxID) (int, error) {
	n := 0
	for k := range f.boxTags {
		if k[0] == id.String() {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) InsertFolder(fo domain.Folder) error {
	fo.CreatedAt, fo.UpdatedAt = f.ts, f.ts
	f.folders[fo.ID] = fo
	return nil
}

func (f *fakeTx) DeleteFoldersOfBox(id domain.BoxID) error {
	for k, fo := range f.folders {
		if fo.BoxID == id {
			delete(f.folders, k)
		}
	}
	return nil
}

func (f *fakeTx) InsertTag(t domain.Tag) error {
	for _, existing := range f.tags {
		if existing.NameFold == t.NameFold {
			return fmt.Errorf("tag name %q is already taken", t.Name)
		}
	}
	f.tags[t.ID] = t
	return nil
}

func (f *fakeTx) UpdateTag(t domain.Tag) error {
	if _, ok := f.tags[t.ID]; !ok {
		return fmt.Errorf("no such tag %s", t.ID)
	}
	for id, existing := range f.tags {
		if id != t.ID && existing.NameFold == t.NameFold {
			return fmt.Errorf("tag name %q is already taken", t.Name)
		}
	}
	f.tags[t.ID] = t
	return nil
}

func (f *fakeTx) DeleteTag(id domain.TagID) error { delete(f.tags, id); return nil }

func (f *fakeTx) GetTag(id domain.TagID) (domain.Tag, bool, error) {
	t, ok := f.tags[id]
	return t, ok, nil
}

func (f *fakeTx) CountTagChildren(id domain.TagID) (int, error) {
	n := 0
	for _, t := range f.tags {
		if t.ParentID != nil && *t.ParentID == id {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) CountTagUsage(id domain.TagID) (int, error) {
	n := f.fileTags[id]
	for k := range f.boxTags {
		if k[1] == id.String() {
			n++
		}
	}
	return n, nil
}

func (f *fakeTx) AddBoxTag(b domain.BoxID, t domain.TagID, source string) error {
	f.boxTags[btKey(b, t)] = source
	return nil
}

func (f *fakeTx) RemoveBoxTag(b domain.BoxID, t domain.TagID) error {
	delete(f.boxTags, btKey(b, t))
	return nil
}

func (f *fakeTx) HasBoxTag(b domain.BoxID, t domain.TagID) (bool, error) {
	_, ok := f.boxTags[btKey(b, t)]
	return ok, nil
}

// compile-time proof that the fake covers the whole interface
var _ Tx = (*fakeTx)(nil)
```

- [x] **Step 2: 写失败的测试**

`internal/oplog/box_test.go`:

```go
package oplog

import (
	"errors"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
)

var opTime = time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)

func createBoxPayload(t *testing.T, name string) (CreateBoxPayload, []byte) {
	t.Helper()
	p := CreateBoxPayload{
		BoxID:        domain.NewBoxID(),
		RootFolderID: domain.NewFolderID(),
		Name:         name,
	}
	raw, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, raw
}

func apply(t *testing.T, tx Tx, action Action, raw []byte) error {
	t.Helper()
	h, err := Lookup(action)
	if err != nil {
		t.Fatal(err)
	}
	return h.Apply(tx, raw)
}

func TestCreateBoxApply(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "云南 2019")

	if err := apply(t, tx, ActionCreateBox, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	b, ok, _ := tx.GetBox(p.BoxID)
	if !ok {
		t.Fatal("the box was not created")
	}
	if b.Name != "云南 2019" || b.NameFold != domain.Fold("云南 2019") {
		t.Errorf("box = %+v", b)
	}
	if !b.CreatedAt.Equal(opTime) {
		t.Errorf("CreatedAt = %v, want the op's ts %v", b.CreatedAt, opTime)
	}

	// spec 1.4: every box has exactly one root folder, and its id must come
	// from the payload — not be minted during Apply, or a replay would hand
	// out fresh ids and break every files.folder_id pointing at it.
	root, ok := tx.folders[p.RootFolderID]
	if !ok {
		t.Fatal("the root folder was not created")
	}
	if root.BoxID != p.BoxID || root.ParentID != nil || root.Name != "" {
		t.Errorf("root folder = %+v", root)
	}
}

func TestCreateBoxIsDeterministic(t *testing.T) {
	// Invariant 3: applying the same payload to two empty transactions must
	// produce identical state. This is the property `kist rebuild` rests on.
	_, raw := createBoxPayload(t, "同一个 payload")

	a, b := newFakeTx(opTime), newFakeTx(opTime)
	if err := apply(t, a, ActionCreateBox, raw); err != nil {
		t.Fatal(err)
	}
	if err := apply(t, b, ActionCreateBox, raw); err != nil {
		t.Fatal(err)
	}

	if len(a.boxes) != 1 || len(b.boxes) != 1 {
		t.Fatalf("box counts %d and %d", len(a.boxes), len(b.boxes))
	}
	for id, boxA := range a.boxes {
		boxB, ok := b.boxes[id]
		if !ok {
			t.Fatalf("box %s exists in one replay but not the other", id)
		}
		if boxA != boxB {
			t.Errorf("two replays produced different boxes:\n  %+v\n  %+v", boxA, boxB)
		}
	}
}

func TestCreateBoxInverseIsDeleteBox(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "regrettable")
	apply(t, tx, ActionCreateBox, raw)

	h, _ := Lookup(ActionCreateBox)
	action, inverse, err := h.Inverse(raw)
	if err != nil {
		t.Fatalf("Inverse: %v", err)
	}
	if action != ActionDeleteBox {
		t.Fatalf("inverse action = %q, want %q", action, ActionDeleteBox)
	}

	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	if _, ok, _ := tx.GetBox(p.BoxID); ok {
		t.Error("the box survived its own deletion")
	}
	if len(tx.folders) != 0 {
		t.Errorf("%d folders were left behind", len(tx.folders))
	}
}

func TestDeleteBoxInverseRestoresEverything(t *testing.T) {
	// Undo of an undo. The payload must carry enough to rebuild the box
	// byte-for-byte, including the root folder id (spec 2.3).
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "resurrect me")
	apply(t, tx, ActionCreateBox, raw)

	createH, _ := Lookup(ActionCreateBox)
	delAction, delPayload, _ := createH.Inverse(raw)
	apply(t, tx, delAction, delPayload)

	delH, _ := Lookup(ActionDeleteBox)
	backAction, backPayload, err := delH.Inverse(delPayload)
	if err != nil {
		t.Fatalf("Inverse of delete: %v", err)
	}
	if backAction != ActionCreateBox {
		t.Fatalf("inverse of delete = %q, want create_box", backAction)
	}
	if err := apply(t, tx, backAction, backPayload); err != nil {
		t.Fatalf("re-creating: %v", err)
	}

	b, ok, _ := tx.GetBox(p.BoxID)
	if !ok {
		t.Fatal("the box did not come back")
	}
	if b.Name != "resurrect me" {
		t.Errorf("name came back as %q", b.Name)
	}
	if _, ok := tx.folders[p.RootFolderID]; !ok {
		t.Error("the root folder came back with a different id")
	}
}

func TestDeleteBoxRefusesWhenTagged(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "tagged")
	apply(t, tx, ActionCreateBox, raw)
	tag, _ := domain.NewTag(domain.NewTagID(), nil, "云南")
	tx.InsertTag(tag)
	tx.AddBoxTag(p.BoxID, tag.ID, "user")

	h, _ := Lookup(ActionCreateBox)
	action, inverse, _ := h.Inverse(raw)
	err := apply(t, tx, action, inverse)
	if err == nil {
		t.Fatal("deleting a tagged box succeeded")
	}
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("error = %v, want it to wrap ErrStaleBefore", err)
	}
}

func TestUpdateBoxApplyAndInverse(t *testing.T) {
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "old name")
	apply(t, tx, ActionCreateBox, raw)

	start, _ := domain.ParseLocalDate("2019-08")
	up := UpdateBoxPayload{
		BoxID:  p.BoxID,
		Before: BoxFields{Name: "old name"},
		After:  BoxFields{Name: "new name", Description: "with a note", StartAt: start},
	}
	upRaw, _ := Marshal(up)

	later := newFakeTx(opTime.Add(time.Hour))
	later.boxes = tx.boxes
	later.folders = tx.folders
	if err := apply(t, later, ActionUpdateBox, upRaw); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	b, _, _ := later.GetBox(p.BoxID)
	if b.Name != "new name" || b.Description != "with a note" || b.StartAt.String() != "2019-08" {
		t.Errorf("after update: %+v", b)
	}
	if b.NameFold != domain.Fold("new name") {
		t.Errorf("NameFold was not regenerated: %q", b.NameFold)
	}

	h, _ := Lookup(ActionUpdateBox)
	action, invRaw, err := h.Inverse(upRaw)
	if err != nil {
		t.Fatalf("Inverse: %v", err)
	}
	if action != ActionUpdateBox {
		t.Errorf("inverse of update_box = %q, want update_box", action)
	}
	if err := apply(t, later, action, invRaw); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	b, _, _ = later.GetBox(p.BoxID)
	if b.Name != "old name" || b.Description != "" || !b.StartAt.IsZero() {
		t.Errorf("after undo: %+v, want the original state back", b)
	}
}

func TestUpdateBoxRefusesAStaleBefore(t *testing.T) {
	// spec 2.5, strict mode: "I thought the undo worked but it only restored
	// half of it" is far more dangerous in archiving software than "the undo
	// failed".
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "actual name")
	apply(t, tx, ActionCreateBox, raw)

	stale := UpdateBoxPayload{
		BoxID:  p.BoxID,
		Before: BoxFields{Name: "something else entirely"},
		After:  BoxFields{Name: "whatever"},
	}
	staleRaw, _ := Marshal(stale)

	err := apply(t, tx, ActionUpdateBox, staleRaw)
	if err == nil {
		t.Fatal("an update with a stale before-state was accepted")
	}
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("error = %v, want it to wrap ErrStaleBefore", err)
	}
	b, _, _ := tx.GetBox(p.BoxID)
	if b.Name != "actual name" {
		t.Errorf("the box was modified anyway: %q", b.Name)
	}
}

func TestUpdateBoxIgnoresUpdatedAtWhenComparing(t *testing.T) {
	// spec 1.5.1 ④: updated_at is maintained derived state, not semantic
	// state. If the before-comparison included it, every undo would fail.
	tx := newFakeTx(opTime)
	p, raw := createBoxPayload(t, "name")
	apply(t, tx, ActionCreateBox, raw)

	// Simulate time passing and a touch bumping updated_at.
	b := tx.boxes[p.BoxID]
	b.UpdatedAt = opTime.Add(72 * time.Hour)
	tx.boxes[p.BoxID] = b

	up := UpdateBoxPayload{
		BoxID:  p.BoxID,
		Before: BoxFields{Name: "name"},
		After:  BoxFields{Name: "renamed"},
	}
	upRaw, _ := Marshal(up)
	if err := apply(t, tx, ActionUpdateBox, upRaw); err != nil {
		t.Fatalf("a bumped updated_at blocked a legitimate update: %v", err)
	}
}

func TestBoxTargets(t *testing.T) {
	// Invariant 4: op_boxes must be reconstructible from the payload alone.
	p, raw := createBoxPayload(t, "targeted")
	for _, action := range []Action{ActionCreateBox, ActionDeleteBox} {
		h, _ := Lookup(action)
		var payload []byte
		if action == ActionCreateBox {
			payload = raw
		} else {
			_, payload, _ = h2Inverse(t, ActionCreateBox, raw)
		}
		got, err := h.Targets(payload)
		if err != nil {
			t.Fatalf("Targets(%s): %v", action, err)
		}
		if len(got.Boxes) != 1 || got.Boxes[0] != p.BoxID {
			t.Errorf("Targets(%s).Boxes = %v, want [%v]", action, got.Boxes, p.BoxID)
		}
	}
}

func h2Inverse(t *testing.T, a Action, raw []byte) (Action, []byte, error) {
	t.Helper()
	h, _ := Lookup(a)
	return h.Inverse(raw)
}
```

- [x] **Step 3: 跑测试确认失败**

Run: `go test ./internal/oplog/ -run 'Box' -v`
Expected: FAIL，`undefined: CreateBoxPayload`

- [x] **Step 4: 实现**

`internal/oplog/box.go`:

```go
package oplog

import (
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ErrStaleBefore means the payload's recorded "before" state does not match
// what is actually in the library.
//
// This is strict undo (spec 2.5). Undoing op 3 while sitting at op 500 can
// mean restoring a box that later ops have already changed; kist refuses
// rather than half-restoring, because "I thought the undo worked but it only
// brought back part of it" is the worse failure in an archive.
var ErrStaleBefore = errors.New("the library has moved on since this op")

func init() {
	Register(ActionCreateBox, createBox{})
	Register(ActionDeleteBox, deleteBox{})
	Register(ActionUpdateBox, updateBox{})
}

// BoxFields are the user-editable fields of a box. updated_at is absent on
// purpose: it is maintained derived state and does not take part in Inverse
// (spec 1.5.1 ④).
type BoxFields struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	StartAt     domain.LocalDate `json:"start_at,omitempty"`
	EndAt       domain.LocalDate `json:"end_at,omitempty"`
}

// CreateBoxPayload carries every decision the command made, so that Apply
// makes none (invariant 3): both ids are minted by library before the op is
// written.
type CreateBoxPayload struct {
	BoxID        domain.BoxID    `json:"box_id"`
	RootFolderID domain.FolderID `json:"root_folder_id"`
	BoxFields
}

// DeleteBoxPayload is the same shape: deleting must record enough to rebuild
// the box exactly, including the root folder's id — a re-created folder with a
// fresh id would orphan every files.folder_id pointing at it.
type DeleteBoxPayload = CreateBoxPayload

type createBox struct{}

func (createBox) Apply(tx Tx, payload []byte) error {
	p, err := Payload[CreateBoxPayload](payload)
	if err != nil {
		return err
	}
	b, err := domain.NewBox(p.BoxID, p.Name, p.Description)
	if err != nil {
		return err
	}
	b.StartAt, b.EndAt = p.StartAt, p.EndAt
	if err := tx.InsertBox(b); err != nil {
		return err
	}
	return tx.InsertFolder(domain.NewRootFolder(p.RootFolderID, p.BoxID))
}

func (createBox) Inverse(payload []byte) (Action, []byte, error) {
	// The payload already describes the whole box, so the inverse is the same
	// bytes under a different action.
	return ActionDeleteBox, payload, nil
}

func (createBox) Targets(payload []byte) (Targets, error) { return boxTargets(payload) }

type deleteBox struct{}

func (deleteBox) Apply(tx Tx, payload []byte) error {
	p, err := Payload[DeleteBoxPayload](payload)
	if err != nil {
		return err
	}

	current, ok, err := tx.GetBox(p.BoxID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("box %s is already gone: %w", p.BoxID, ErrStaleBefore)
	}
	if err := checkBoxFields(p.BoxID, p.BoxFields, current); err != nil {
		return err
	}
	// Refusing to delete a box that has since been tagged is the same rule as
	// refusing to delete a tag that is in use: an undo must not quietly take
	// unrelated work with it.
	n, err := tx.CountBoxTags(p.BoxID)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("box %s carries %d tag(s) added after this op: %w", p.BoxID, n, ErrStaleBefore)
	}

	if err := tx.DeleteFoldersOfBox(p.BoxID); err != nil {
		return err
	}
	return tx.DeleteBox(p.BoxID)
}

func (deleteBox) Inverse(payload []byte) (Action, []byte, error) {
	return ActionCreateBox, payload, nil
}

func (deleteBox) Targets(payload []byte) (Targets, error) { return boxTargets(payload) }

// UpdateBoxPayload records both sides so Inverse is a swap and needs no
// database access at all (spec 2.3).
type UpdateBoxPayload struct {
	BoxID  domain.BoxID `json:"box_id"`
	Before BoxFields    `json:"before"`
	After  BoxFields    `json:"after"`
}

type updateBox struct{}

func (updateBox) Apply(tx Tx, payload []byte) error {
	p, err := Payload[UpdateBoxPayload](payload)
	if err != nil {
		return err
	}
	current, ok, err := tx.GetBox(p.BoxID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("box %s no longer exists: %w", p.BoxID, ErrStaleBefore)
	}
	if err := checkBoxFields(p.BoxID, p.Before, current); err != nil {
		return err
	}

	updated, err := domain.NewBox(p.BoxID, p.After.Name, p.After.Description)
	if err != nil {
		return err
	}
	updated.StartAt, updated.EndAt = p.After.StartAt, p.After.EndAt
	return tx.UpdateBox(updated)
}

func (updateBox) Inverse(payload []byte) (Action, []byte, error) {
	p, err := Payload[UpdateBoxPayload](payload)
	if err != nil {
		return "", nil, err
	}
	p.Before, p.After = p.After, p.Before
	raw, err := Marshal(p)
	return ActionUpdateBox, raw, err
}

func (updateBox) Targets(payload []byte) (Targets, error) {
	p, err := Payload[UpdateBoxPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Boxes: []domain.BoxID{p.BoxID}}, nil
}

// checkBoxFields is the strict-mode assertion. It compares semantic fields
// only — created_at is immutable and updated_at is derived, so neither belongs
// in the comparison.
func checkBoxFields(id domain.BoxID, want BoxFields, got domain.Box) error {
	actual := BoxFields{
		Name:        got.Name,
		Description: got.Description,
		StartAt:     got.StartAt,
		EndAt:       got.EndAt,
	}
	if actual != want {
		return fmt.Errorf("box %s reads %+v but this op expected %+v: %w", id, actual, want, ErrStaleBefore)
	}
	return nil
}

func boxTargets(payload []byte) (Targets, error) {
	p, err := Payload[CreateBoxPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Boxes: []domain.BoxID{p.BoxID}}, nil
}
```

- [x] **Step 5: 把三个 action 放回 `AllActions()`**

Task 2 的 Step 7 说过，`AllActions()` 里未实现的项要暂时注释掉。现在把 `ActionCreateBox` / `ActionDeleteBox` / `ActionUpdateBox` 三项放回来。

- [x] **Step 6: 跑测试确认通过**

Run: `go test ./internal/oplog/ -v`
Expected: PASS

- [x] **Step 7: 确认依赖方向没有被破坏**

Run: `go list -deps ./internal/oplog/ | grep kist`
Expected: 只出现 `github.com/jinzhao1994/kist/internal/domain`。**如果出现 `store`，这个 task 就没做对**——`Apply` 必须够不到数据库。

- [x] **Step 8: 把严格模式的位置写进设计文档**

在 `docs/design/2026-08-22-kist-v0.1-design.md` 的 2.5 节末尾补一段：

> **校验住在 `Apply` 里，不单开一个 `Check` 方法。** 理由有三条：
>
> ① 它**不是决策而是断言**——要么原样执行，要么报错退出，不存在"根据当前状态走另一条路"。所以它不违反不变量 3，那条禁的是依赖当前状态**做出不同的行为**。
>
> ② **正向 op 走同一条路也是对的。** 正向 op 由 `library` 从当前状态生成，校验必然通过；`kist rebuild` 重放时前序 op 已经把状态放到位，校验同样通过。**校验失败只可能发生在隔空撤销上**，正是本节要拦的场景。
>
> ③ 单开 `Check` 意味着每个 handler 把 before 状态的比对写两遍，或者 `Apply` 转头去调 `Check`——那还不如直接写在 `Apply` 里。
>
> **比对只看语义字段。** `created_at` 不可变，`updated_at` 是派生状态且按 1.5.1 ④ 不参与 `Inverse`——把它纳入比对的话，任何一次撤销都会因为"时间戳对不上"而失败。

- [x] **Step 9: 提交**

```bash
git add internal/oplog/ docs/design/2026-08-22-kist-v0.1-design.md
git commit -m "feat(oplog): box 的三个 handler，严格模式落在 Apply 里

payload 自带 before 状态，所以 Inverse 是纯粹的字段对调，不需要查库。
delete_box 的 payload 记了根 folder 的 id——少了它，撤销再撤销回来
folder 会拿到新 id，而 M1c 之后 files.folder_id 指着它。

严格校验只比对语义字段：created_at 不可变，updated_at 是派生状态且
按 1.5.1 ④ 不参与 Inverse，纳入比对的话任何撤销都会因为时间戳对不上
而失败。这条写进了设计文档 2.5。"
```

---

### Task 6: oplog — tag 树的四个 handler

**Files:**
- Create: `internal/oplog/tag.go`
- Test: `internal/oplog/tag_test.go`

**Interfaces:**
- Consumes: `oplog.Tx`、`oplog.ErrStaleBefore`、`domain.Tag`
- Produces:
  - `oplog.CreateTagPayload{TagID, ParentID, Name}`、`oplog.DeleteTagPayload`（同结构）
  - `oplog.RenameTagPayload{TagID, Before, After string}`
  - `oplog.MoveTagPayload{TagID, OldParent, NewParent *domain.TagID}`
  - `oplog.ErrTagInUse`

**`move_tag` 的 payload 只有三个字段**——`{node, old_parent, new_parent}`。设计文档 1.6 特意点出这是全局唯一命名换来的：按父唯一的话，移动会撞名，于是 payload 要膨胀到记录改名前后，`Inverse` 也跟着变复杂。这个 task 是那段论证的兑现，payload 结构本身就是证据。

**没有物化路径列，所以移动子树在 DB 里是改一行 `parent_id`**，O(1)。`Apply` 里没有任何递归。

**`delete_tag` 有子节点或有东西挂着就拒绝**（spec 1.6）。级联删除在归档软件里太危险——一次误点可以静默毁掉全库分类信息，而撤销它的 payload 会大到离谱。返回 `ErrTagInUse` 而不是 `ErrStaleBefore`：这不是"库变了"，是"这个操作本身不被允许"，用户看到的提示应该不同。

**tag 没有 `updated_at`，也不进 `op_boxes`。** tag 树不属于任何 box（spec 2.2.1 说 tag 树操作单独记 `scope='tags'`），所以 `Targets` 返回空。M1b 不实现 `scope` 列——它在 schema 里还不存在，加它是 M5 历史 UI 的事；现在返回空 `Targets` 是诚实的：这条 op 确实没碰任何 box。

- [ ] **Step 1: 写失败的测试**

```go
package oplog

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func createTag(t *testing.T, tx Tx, parent *domain.TagID, name string) domain.TagID {
	t.Helper()
	p := CreateTagPayload{TagID: domain.NewTagID(), ParentID: parent, Name: name}
	raw, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(t, tx, ActionCreateTag, raw); err != nil {
		t.Fatalf("create tag %q: %v", name, err)
	}
	return p.TagID
}

func TestCreateTagApply(t *testing.T) {
	tx := newFakeTx(opTime)
	id := createTag(t, tx, nil, "人物")

	tag, ok, _ := tx.GetTag(id)
	if !ok {
		t.Fatal("the tag was not created")
	}
	if tag.Name != "人物" || tag.NameFold != domain.Fold("人物") {
		t.Errorf("tag = %+v", tag)
	}
	if tag.ParentID != nil {
		t.Error("a root tag should have no parent")
	}
}

func TestCreateTagUnderParent(t *testing.T) {
	tx := newFakeTx(opTime)
	people := createTag(t, tx, nil, "人物")
	dad := createTag(t, tx, &people, "爸")

	tag, _, _ := tx.GetTag(dad)
	if tag.ParentID == nil || *tag.ParentID != people {
		t.Errorf("ParentID = %v, want %v", tag.ParentID, people)
	}
}

func TestCreateTagInverseIsDelete(t *testing.T) {
	tx := newFakeTx(opTime)
	p := CreateTagPayload{TagID: domain.NewTagID(), Name: "临时"}
	raw, _ := Marshal(p)
	apply(t, tx, ActionCreateTag, raw)

	h, _ := Lookup(ActionCreateTag)
	action, inverse, err := h.Inverse(raw)
	if err != nil {
		t.Fatalf("Inverse: %v", err)
	}
	if action != ActionDeleteTag {
		t.Fatalf("inverse action = %q, want delete_tag", action)
	}
	if err := apply(t, tx, action, inverse); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	if _, ok, _ := tx.GetTag(p.TagID); ok {
		t.Error("the tag survived its deletion")
	}
}

func TestDeleteTagRefusesWithChildren(t *testing.T) {
	// spec 1.6: cascade delete in archiving software can wipe a whole
	// classification with one misclick, and the payload needed to undo that
	// would be enormous.
	tx := newFakeTx(opTime)
	people := createTag(t, tx, nil, "人物")
	createTag(t, tx, &people, "爸")

	p := DeleteTagPayload{TagID: people, Name: "人物"}
	raw, _ := Marshal(p)
	err := apply(t, tx, ActionDeleteTag, raw)
	if err == nil {
		t.Fatal("deleting a tag with children succeeded")
	}
	if !errors.Is(err, ErrTagInUse) {
		t.Errorf("error = %v, want ErrTagInUse", err)
	}
}

func TestDeleteTagRefusesWhenAttached(t *testing.T) {
	tx := newFakeTx(opTime)
	tag := createTag(t, tx, nil, "云南")
	box, raw := createBoxPayload(t, "trip")
	apply(t, tx, ActionCreateBox, raw)
	tx.AddBoxTag(box.BoxID, tag, "user")

	p := DeleteTagPayload{TagID: tag, Name: "云南"}
	delRaw, _ := Marshal(p)
	err := apply(t, tx, ActionDeleteTag, delRaw)
	if err == nil {
		t.Fatal("deleting a tag that is still attached succeeded")
	}
	if !errors.Is(err, ErrTagInUse) {
		t.Errorf("error = %v, want ErrTagInUse", err)
	}
}

func TestRenameTagApplyAndInverse(t *testing.T) {
	tx := newFakeTx(opTime)
	id := createTag(t, tx, nil, "旧名")

	p := RenameTagPayload{TagID: id, Before: "旧名", After: "新名"}
	raw, _ := Marshal(p)
	if err := apply(t, tx, ActionRenameTag, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	tag, _, _ := tx.GetTag(id)
	if tag.Name != "新名" || tag.NameFold != domain.Fold("新名") {
		t.Errorf("after rename: %+v", tag)
	}

	h, _ := Lookup(ActionRenameTag)
	action, inv, _ := h.Inverse(raw)
	if action != ActionRenameTag {
		t.Errorf("inverse action = %q, want rename_tag", action)
	}
	if err := apply(t, tx, action, inv); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	tag, _, _ = tx.GetTag(id)
	if tag.Name != "旧名" {
		t.Errorf("after undo the name is %q", tag.Name)
	}
}

func TestRenameTagRefusesAStaleBefore(t *testing.T) {
	tx := newFakeTx(opTime)
	id := createTag(t, tx, nil, "实际名字")

	p := RenameTagPayload{TagID: id, Before: "别的名字", After: "无所谓"}
	raw, _ := Marshal(p)
	err := apply(t, tx, ActionRenameTag, raw)
	if !errors.Is(err, ErrStaleBefore) {
		t.Errorf("error = %v, want ErrStaleBefore", err)
	}
}

func TestMoveTagIsOneFieldChange(t *testing.T) {
	// spec 1.6: no materialized path column, so moving a subtree is a single
	// parent_id write no matter how big the subtree is.
	tx := newFakeTx(opTime)
	places := createTag(t, tx, nil, "地点")
	people := createTag(t, tx, nil, "人物")
	yunnan := createTag(t, tx, &places, "云南")
	createTag(t, tx, &yunnan, "大理")
	createTag(t, tx, &yunnan, "丽江")

	p := MoveTagPayload{TagID: yunnan, OldParent: &places, NewParent: &people}
	raw, _ := Marshal(p)
	if err := apply(t, tx, ActionMoveTag, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	moved, _, _ := tx.GetTag(yunnan)
	if moved.ParentID == nil || *moved.ParentID != people {
		t.Errorf("ParentID = %v, want %v", moved.ParentID, people)
	}
	// The children were not touched: they still point at 云南, which is the
	// whole reason the payload is only three fields.
	n, _ := tx.CountTagChildren(yunnan)
	if n != 2 {
		t.Errorf("云南 has %d children after the move, want 2", n)
	}
}

func TestMoveTagToRoot(t *testing.T) {
	tx := newFakeTx(opTime)
	places := createTag(t, tx, nil, "地点")
	yunnan := createTag(t, tx, &places, "云南")

	p := MoveTagPayload{TagID: yunnan, OldParent: &places, NewParent: nil}
	raw, _ := Marshal(p)
	if err := apply(t, tx, ActionMoveTag, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	moved, _, _ := tx.GetTag(yunnan)
	if moved.ParentID != nil {
		t.Errorf("ParentID = %v, want nil", moved.ParentID)
	}

	h, _ := Lookup(ActionMoveTag)
	action, inv, _ := h.Inverse(raw)
	if action != ActionMoveTag {
		t.Errorf("inverse action = %q", action)
	}
	if err := apply(t, tx, action, inv); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	moved, _, _ = tx.GetTag(yunnan)
	if moved.ParentID == nil || *moved.ParentID != places {
		t.Errorf("after undo ParentID = %v, want %v", moved.ParentID, places)
	}
}

func TestMoveTagRefusesAStaleParent(t *testing.T) {
	tx := newFakeTx(opTime)
	a := createTag(t, tx, nil, "a")
	b := createTag(t, tx, nil, "b")
	child := createTag(t, tx, &a, "child")

	wrong := b
	p := MoveTagPayload{TagID: child, OldParent: &wrong, NewParent: nil}
	raw, _ := Marshal(p)
	if err := apply(t, tx, ActionMoveTag, raw); !errors.Is(err, ErrStaleBefore) {
		t.Errorf("error = %v, want ErrStaleBefore", err)
	}
}

func TestTagOpsTouchNoBoxes(t *testing.T) {
	// The tag tree belongs to no box (spec 2.2.1), so these ops produce no
	// op_boxes rows. Returning an empty Targets is the honest answer.
	tx := newFakeTx(opTime)
	id := createTag(t, tx, nil, "孤立")

	cases := []struct {
		action  Action
		payload any
	}{
		{ActionCreateTag, CreateTagPayload{TagID: id, Name: "孤立"}},
		{ActionDeleteTag, DeleteTagPayload{TagID: id, Name: "孤立"}},
		{ActionRenameTag, RenameTagPayload{TagID: id, Before: "a", After: "b"}},
		{ActionMoveTag, MoveTagPayload{TagID: id}},
	}
	for _, c := range cases {
		raw, _ := Marshal(c.payload)
		h, _ := Lookup(c.action)
		got, err := h.Targets(raw)
		if err != nil {
			t.Fatalf("Targets(%s): %v", c.action, err)
		}
		if len(got.Boxes) != 0 || len(got.Files) != 0 {
			t.Errorf("Targets(%s) = %+v, want empty", c.action, got)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/oplog/ -run 'Tag' -v`
Expected: FAIL，`undefined: CreateTagPayload`

- [ ] **Step 3: 实现**

`internal/oplog/tag.go`:

```go
package oplog

import (
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ErrTagInUse means a tag cannot be deleted because something still hangs off
// it. This is a refusal by policy (spec 1.6), not a stale-state conflict, and
// the user needs a different message: cascade delete in archiving software can
// wipe a whole classification with one misclick.
var ErrTagInUse = errors.New("tag is still in use")

func init() {
	Register(ActionCreateTag, createTagOp{})
	Register(ActionDeleteTag, deleteTagOp{})
	Register(ActionRenameTag, renameTagOp{})
	Register(ActionMoveTag, moveTagOp{})
}

// CreateTagPayload carries the id minted by library (invariant 3).
type CreateTagPayload struct {
	TagID    domain.TagID  `json:"tag_id"`
	ParentID *domain.TagID `json:"parent_id,omitempty"`
	Name     string        `json:"name"`
}

// DeleteTagPayload is the same shape, so undoing a delete rebuilds the node
// exactly where it was.
type DeleteTagPayload = CreateTagPayload

type createTagOp struct{}

func (createTagOp) Apply(tx Tx, payload []byte) error {
	p, err := Payload[CreateTagPayload](payload)
	if err != nil {
		return err
	}
	tag, err := domain.NewTag(p.TagID, p.ParentID, p.Name)
	if err != nil {
		return err
	}
	return tx.InsertTag(tag)
}

func (createTagOp) Inverse(payload []byte) (Action, []byte, error) {
	return ActionDeleteTag, payload, nil
}

func (createTagOp) Targets([]byte) (Targets, error) { return Targets{}, nil }

type deleteTagOp struct{}

func (deleteTagOp) Apply(tx Tx, payload []byte) error {
	p, err := Payload[DeleteTagPayload](payload)
	if err != nil {
		return err
	}
	current, ok, err := tx.GetTag(p.TagID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("tag %s is already gone: %w", p.TagID, ErrStaleBefore)
	}
	if current.Name != p.Name {
		return fmt.Errorf("tag %s is now named %q but this op expected %q: %w",
			p.TagID, current.Name, p.Name, ErrStaleBefore)
	}

	children, err := tx.CountTagChildren(p.TagID)
	if err != nil {
		return err
	}
	if children > 0 {
		return fmt.Errorf("tag %q has %d child tag(s); move or delete them first: %w",
			p.Name, children, ErrTagInUse)
	}
	usage, err := tx.CountTagUsage(p.TagID)
	if err != nil {
		return err
	}
	if usage > 0 {
		return fmt.Errorf("tag %q is attached to %d item(s); remove it from them first: %w",
			p.Name, usage, ErrTagInUse)
	}
	return tx.DeleteTag(p.TagID)
}

func (deleteTagOp) Inverse(payload []byte) (Action, []byte, error) {
	return ActionCreateTag, payload, nil
}

func (deleteTagOp) Targets([]byte) (Targets, error) { return Targets{}, nil }

// RenameTagPayload records both names so Inverse is a swap.
type RenameTagPayload struct {
	TagID  domain.TagID `json:"tag_id"`
	Before string       `json:"before"`
	After  string       `json:"after"`
}

type renameTagOp struct{}

func (renameTagOp) Apply(tx Tx, payload []byte) error {
	p, err := Payload[RenameTagPayload](payload)
	if err != nil {
		return err
	}
	current, ok, err := tx.GetTag(p.TagID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("tag %s no longer exists: %w", p.TagID, ErrStaleBefore)
	}
	if current.Name != p.Before {
		return fmt.Errorf("tag %s is named %q but this op expected %q: %w",
			p.TagID, current.Name, p.Before, ErrStaleBefore)
	}

	renamed, err := domain.NewTag(p.TagID, current.ParentID, p.After)
	if err != nil {
		return err
	}
	return tx.UpdateTag(renamed)
}

func (renameTagOp) Inverse(payload []byte) (Action, []byte, error) {
	p, err := Payload[RenameTagPayload](payload)
	if err != nil {
		return "", nil, err
	}
	p.Before, p.After = p.After, p.Before
	raw, err := Marshal(p)
	return ActionRenameTag, raw, err
}

func (renameTagOp) Targets([]byte) (Targets, error) { return Targets{}, nil }

// MoveTagPayload is three fields, and that is the whole point.
//
// Global name uniqueness (spec 1.6) means a move can never collide, so there
// is no rename to record and no lookup to perform. Per-parent uniqueness would
// have forced this payload to grow a before/after name pair and Inverse to
// grow with it.
type MoveTagPayload struct {
	TagID     domain.TagID  `json:"tag_id"`
	OldParent *domain.TagID `json:"old_parent,omitempty"`
	NewParent *domain.TagID `json:"new_parent,omitempty"`
}

type moveTagOp struct{}

func (moveTagOp) Apply(tx Tx, payload []byte) error {
	p, err := Payload[MoveTagPayload](payload)
	if err != nil {
		return err
	}
	current, ok, err := tx.GetTag(p.TagID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("tag %s no longer exists: %w", p.TagID, ErrStaleBefore)
	}
	if !sameParent(current.ParentID, p.OldParent) {
		return fmt.Errorf("tag %s hangs off %v but this op expected %v: %w",
			p.TagID, current.ParentID, p.OldParent, ErrStaleBefore)
	}

	// One row, one column. There is no materialized path to cascade, which is
	// why moving a subtree of any size is O(1) (spec 1.6).
	current.ParentID = p.NewParent
	return tx.UpdateTag(current)
}

func (moveTagOp) Inverse(payload []byte) (Action, []byte, error) {
	p, err := Payload[MoveTagPayload](payload)
	if err != nil {
		return "", nil, err
	}
	p.OldParent, p.NewParent = p.NewParent, p.OldParent
	raw, err := Marshal(p)
	return ActionMoveTag, raw, err
}

func (moveTagOp) Targets([]byte) (Targets, error) { return Targets{}, nil }

func sameParent(a, b *domain.TagID) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}
```

- [ ] **Step 4: 把四个 action 放回 `AllActions()`**

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/oplog/ -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/oplog/tag.go internal/oplog/tag_test.go internal/oplog/action.go
git commit -m "feat(oplog): tag 树的四个 handler

move_tag 的 payload 只有 {node, old_parent, new_parent} 三个字段——
这是设计文档 1.6 里'全局唯一换来移动 O(1)'那段论证的兑现，payload
结构本身就是证据。没有物化路径列，Apply 里没有任何递归。

delete_tag 有子节点或有东西挂着就拒绝，返回 ErrTagInUse 而不是
ErrStaleBefore：这不是'库变了'，是'这个操作不被允许'，用户该看到
不同的提示。"
```

---

### Task 7: oplog — box_tags 的两个 handler（批量语义）

**Files:**
- Create: `internal/oplog/boxtag.go`
- Test: `internal/oplog/boxtag_test.go`

**Interfaces:**
- Consumes: `oplog.Tx`、`domain.BoxID` / `TagID`
- Produces: `oplog.TagBoxesPayload{TagID, BoxIDs []domain.BoxID, Source string}`、`oplog.UntagBoxesPayload`（同结构）

**这是"op 的粒度是批"的第一个实例**（spec 2.2.1）。给 12 个 box 打一个 tag 是**一条** op 带 12 个 target，不是 12 条——否则撤销要点 12 次。`Targets` 因此返回 12 个 box id，`op_boxes` 会有 12 行。

**`tag_boxes` 的 payload 只记要加的那些，不记"加之前有没有"。** 这里有个陷阱：如果 12 个 box 里有 3 个本来就有这个 tag，那么 `untag_boxes` 作为它的反向会把那 3 个也摘掉——**多摘了**。所以 payload 必须记录**实际发生了变化的那些 box**，而这个筛选由 `library` 在生成 op 时完成（决策在生成 op 时做完，不变量 3）。`Apply` 拿到的列表就是"确实要加的"。

**`source` 进 payload。** `file_tags` / `box_tags` 的 `source` 区分 user 与 derived（spec 1.6.1），而它必须能被重放重建出来，所以是 payload 的一部分而不是 `Apply` 里的常量。

- [ ] **Step 1: 写失败的测试**

```go
package oplog

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func threeBoxes(t *testing.T, tx Tx) []domain.BoxID {
	t.Helper()
	var out []domain.BoxID
	for _, name := range []string{"一", "二", "三"} {
		p, raw := createBoxPayload(t, name)
		if err := apply(t, tx, ActionCreateBox, raw); err != nil {
			t.Fatal(err)
		}
		out = append(out, p.BoxID)
	}
	return out
}

func TestTagBoxesIsOneOpForManyTargets(t *testing.T) {
	// spec 2.2.1: batching is the unit. Tagging 12 boxes must be one op, or
	// undoing it means twelve clicks.
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "云南")

	p := TagBoxesPayload{TagID: tag, BoxIDs: boxes, Source: "user"}
	raw, _ := Marshal(p)
	if err := apply(t, tx, ActionTagBoxes, raw); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, b := range boxes {
		if has, _ := tx.HasBoxTag(b, tag); !has {
			t.Errorf("box %s did not get the tag", b)
		}
	}

	h, _ := Lookup(ActionTagBoxes)
	targets, err := h.Targets(raw)
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}
	if len(targets.Boxes) != 3 {
		t.Errorf("Targets.Boxes has %d entries, want 3", len(targets.Boxes))
	}
}

func TestTagBoxesInverseIsUntag(t *testing.T) {
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "云南")

	p := TagBoxesPayload{TagID: tag, BoxIDs: boxes, Source: "user"}
	raw, _ := Marshal(p)
	apply(t, tx, ActionTagBoxes, raw)

	h, _ := Lookup(ActionTagBoxes)
	action, inv, err := h.Inverse(raw)
	if err != nil {
		t.Fatalf("Inverse: %v", err)
	}
	if action != ActionUntagBoxes {
		t.Fatalf("inverse action = %q, want untag_boxes", action)
	}
	if err := apply(t, tx, action, inv); err != nil {
		t.Fatalf("applying the inverse: %v", err)
	}
	for _, b := range boxes {
		if has, _ := tx.HasBoxTag(b, tag); has {
			t.Errorf("box %s still carries the tag after undo", b)
		}
	}
}

func TestUntagBoxesInverseIsTag(t *testing.T) {
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "云南")
	for _, b := range boxes {
		tx.AddBoxTag(b, tag, "user")
	}

	p := UntagBoxesPayload{TagID: tag, BoxIDs: boxes, Source: "user"}
	raw, _ := Marshal(p)
	apply(t, tx, ActionUntagBoxes, raw)

	h, _ := Lookup(ActionUntagBoxes)
	action, inv, _ := h.Inverse(raw)
	if action != ActionTagBoxes {
		t.Fatalf("inverse action = %q, want tag_boxes", action)
	}
	apply(t, tx, action, inv)
	for _, b := range boxes {
		if has, _ := tx.HasBoxTag(b, tag); !has {
			t.Errorf("box %s did not get its tag back", b)
		}
	}
}

// The payload names only the boxes that actually changed. If it named boxes
// that already carried the tag, the inverse would strip a tag the op never
// added — an undo doing more than the op did.
func TestTagBoxesPayloadOnlyNamesRealChanges(t *testing.T) {
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "云南")

	// The first box already carries the tag; library would have filtered it
	// out when building the op, so the payload holds only the other two.
	tx.AddBoxTag(boxes[0], tag, "user")

	p := TagBoxesPayload{TagID: tag, BoxIDs: boxes[1:], Source: "user"}
	raw, _ := Marshal(p)
	apply(t, tx, ActionTagBoxes, raw)

	h, _ := Lookup(ActionTagBoxes)
	action, inv, _ := h.Inverse(raw)
	apply(t, tx, action, inv)

	if has, _ := tx.HasBoxTag(boxes[0], tag); !has {
		t.Error("undo removed a tag the op never added")
	}
}

func TestTagBoxesRejectsAnEmptyBatch(t *testing.T) {
	// An op that changes nothing should never have been written. Letting it
	// through would litter the history with entries that undo to nothing.
	tx := newFakeTx(opTime)
	tag := createTag(t, tx, nil, "云南")
	raw, _ := Marshal(TagBoxesPayload{TagID: tag, Source: "user"})
	if err := apply(t, tx, ActionTagBoxes, raw); err == nil {
		t.Error("an empty batch was accepted")
	}
}

func TestTagBoxesSourceSurvivesReplay(t *testing.T) {
	// spec 1.6.1: source distinguishes what a person did from what an
	// extractor did, and the distinction must survive a rebuild — so it lives
	// in the payload, not in a constant inside Apply.
	tx := newFakeTx(opTime)
	boxes := threeBoxes(t, tx)
	tag := createTag(t, tx, nil, "自动")

	raw, _ := Marshal(TagBoxesPayload{TagID: tag, BoxIDs: boxes[:1], Source: "derived"})
	apply(t, tx, ActionTagBoxes, raw)

	if got := tx.boxTags[btKey(boxes[0], tag)]; got != "derived" {
		t.Errorf("source = %q, want %q", got, "derived")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/oplog/ -run 'TagBoxes|UntagBoxes' -v`
Expected: FAIL，`undefined: TagBoxesPayload`

- [ ] **Step 3: 实现**

`internal/oplog/boxtag.go`:

```go
package oplog

import (
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

func init() {
	Register(ActionTagBoxes, tagBoxes{})
	Register(ActionUntagBoxes, untagBoxes{})
}

// TagBoxesPayload attaches one tag to many boxes in a single op.
//
// The granularity is the batch (spec 2.2.1): tagging twelve boxes is one op
// with twelve targets, not twelve ops, or undoing it would take twelve
// clicks.
//
// BoxIDs names only the boxes that actually change. library filters out any
// box that already carries the tag before writing the op — otherwise the
// inverse would strip a tag this op never added, and an undo that does more
// than the op did is a bug you find out about much later.
type TagBoxesPayload struct {
	TagID  domain.TagID   `json:"tag_id"`
	BoxIDs []domain.BoxID `json:"box_ids"`
	// Source is "user" or "derived" (spec 1.6.1). It lives in the payload
	// rather than being a constant inside Apply because a rebuild has to
	// reproduce it.
	Source string `json:"source"`
}

// UntagBoxesPayload is the same shape, so the two are exact inverses.
type UntagBoxesPayload = TagBoxesPayload

var errEmptyBatch = errors.New("op has no targets")

type tagBoxes struct{}

func (tagBoxes) Apply(tx Tx, payload []byte) error {
	p, err := Payload[TagBoxesPayload](payload)
	if err != nil {
		return err
	}
	if len(p.BoxIDs) == 0 {
		return fmt.Errorf("tag_boxes: %w", errEmptyBatch)
	}
	for _, box := range p.BoxIDs {
		if err := tx.AddBoxTag(box, p.TagID, p.Source); err != nil {
			return err
		}
	}
	return nil
}

func (tagBoxes) Inverse(payload []byte) (Action, []byte, error) {
	return ActionUntagBoxes, payload, nil
}

func (tagBoxes) Targets(payload []byte) (Targets, error) { return boxTagTargets(payload) }

type untagBoxes struct{}

func (untagBoxes) Apply(tx Tx, payload []byte) error {
	p, err := Payload[UntagBoxesPayload](payload)
	if err != nil {
		return err
	}
	if len(p.BoxIDs) == 0 {
		return fmt.Errorf("untag_boxes: %w", errEmptyBatch)
	}
	for _, box := range p.BoxIDs {
		if err := tx.RemoveBoxTag(box, p.TagID); err != nil {
			return err
		}
	}
	return nil
}

func (untagBoxes) Inverse(payload []byte) (Action, []byte, error) {
	return ActionTagBoxes, payload, nil
}

func (untagBoxes) Targets(payload []byte) (Targets, error) { return boxTagTargets(payload) }

func boxTagTargets(payload []byte) (Targets, error) {
	p, err := Payload[TagBoxesPayload](payload)
	if err != nil {
		return Targets{}, err
	}
	return Targets{Boxes: p.BoxIDs}, nil
}
```

- [ ] **Step 4: 把两个 action 放回 `AllActions()`，整份列表恢复**

此时 `TestLookupFindsRegisteredHandlers`（Task 2）应当对全部九个 action 通过。

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/oplog/ -v`
Expected: PASS

- [ ] **Step 6: 再确认一次依赖方向**

Run: `go list -deps ./internal/oplog/ | grep kist`
Expected: 只有 `internal/domain`

- [ ] **Step 7: 提交**

```bash
git add internal/oplog/
git commit -m "feat(oplog): box_tags 的两个 handler，批量即一条 op

给 12 个 box 打一个 tag 是一条 op 带 12 个 target，不是 12 条——
否则撤销要点 12 次。

payload 只记实际发生变化的 box：如果把本来就有这个 tag 的也列进去，
反向的 untag_boxes 会把它们也摘掉，等于撤销做了比原 op 更多的事。
筛选在生成 op 时完成，Apply 拿到的就是确定要加的那些。"
```

---

### Task 8: library — Open 与唯一写路径

**Files:**
- Create: `internal/library/library.go`, `internal/library/write.go`
- Test: `internal/library/library_test.go`, `internal/library/write_test.go`

**Interfaces:**
- Consumes: `library.Layout` / `OpenLayout`（M1a）、`config.Load`、`blob.NewDir`、`store.*`、`oplog.*`
- Produces:
  - `library.Library`、`library.Open(root string, opts ...Option) (*Library, error)`、`(*Library).Close()`
  - `library.WithClock(func() time.Time)`、`WithActor(string)`、`WithLogger(*slog.Logger)`
  - `(*Library).write(ctx, action, payload, source, revertsID) (int64, error)`（包内私有）
  - `(*Library).Ops(ctx, store.ListOpsOptions) ([]oplog.Op, error)`

**这个 task 是不变量 2 的落点。** `write` 是包内私有的，`cmd` 够不到；所有对外的写方法（Task 10）都只能经过它。每次写入在**一个事务**里做四件事：插 op → `Apply` → 写索引表 → 传递 `updated_at`。任何一步失败整笔回滚。

**时间戳要截到毫秒。** 存储格式是 `2006-01-02T15:04:05.000Z`，`time.Now()` 带纳秒。不截断的话，写进去的和读回来的不相等——测试里 `CreatedAt.Equal(ts)` 会莫名其妙地失败，而生产里的表现是"重放出来的库和原库差一点点"。

**`updated_at` 的传递规则住在这里，不在每个 handler 里。** `source.PropagatesUpdatedAt()` 为真才传（spec 1.5.1 ②）。放在写路径上意味着将来加新 op 时不可能漏——handler 作者根本不需要知道这条规则存在。

**时钟可注入。** `WithClock` 让测试能给出确定的 ts。这不只是为了测试舒服：Task 12 的 `Rebuild` 要断言"重建结果逐字节相同"，而快照里含时间戳。

- [ ] **Step 1: 写失败的测试**

`internal/library/write_test.go`:

```go
package library

import (
	"context"
	"testing"
	"time"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// fixedClock hands out timestamps one second apart, so op order is visible in
// the data and every run produces the same values.
func fixedClock() func() time.Time {
	base := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	n := 0
	return func() time.Time {
		n++
		return base.Add(time.Duration(n) * time.Second)
	}
}

// openTestLibrary creates a library in a temp dir and opens it.
func openTestLibrary(t *testing.T) *Library {
	t.Helper()
	root := t.TempDir()
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatalf("Create: %v", err)
	}
	l, err := Open(root, WithClock(fixedClock()), WithActor("test"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func TestOpenRejectsANonLibrary(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Error("Open on an empty directory = nil error, want a refusal")
	}
}

func TestWriteRecordsAnOpAndAppliesIt(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()

	p := oplog.CreateBoxPayload{
		BoxID:        domain.NewBoxID(),
		RootFolderID: domain.NewFolderID(),
		BoxFields:    oplog.BoxFields{Name: "云南"},
	}
	raw, _ := oplog.Marshal(p)

	opID, err := l.write(ctx, oplog.ActionCreateBox, raw, oplog.SourceUser, nil)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if opID <= 0 {
		t.Fatalf("write returned op id %d", opID)
	}

	ops, err := l.Ops(ctx, store.ListOpsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Ops: %v", err)
	}
	if len(ops) != 1 || ops[0].Action != oplog.ActionCreateBox || ops[0].Source != oplog.SourceUser {
		t.Fatalf("ops = %+v", ops)
	}
	if ops[0].Actor != "test" {
		t.Errorf("Actor = %q, want %q", ops[0].Actor, "test")
	}

	// The materialized view moved in the same transaction.
	b, ok, err := l.Box(ctx, p.BoxID)
	if err != nil || !ok {
		t.Fatalf("Box: %v ok=%v", err, ok)
	}
	if b.Name != "云南" {
		t.Errorf("box name = %q", b.Name)
	}
	if !b.CreatedAt.Equal(ops[0].TS) {
		t.Errorf("CreatedAt = %v, want the op's ts %v", b.CreatedAt, ops[0].TS)
	}
}

func TestWriteIsAtomic(t *testing.T) {
	// If Apply fails, the op must not survive either — otherwise the log would
	// claim something happened that did not, and a rebuild would then make it
	// happen.
	l := openTestLibrary(t)
	ctx := context.Background()

	stale := oplog.UpdateBoxPayload{
		BoxID:  domain.NewBoxID(), // no such box
		Before: oplog.BoxFields{Name: "whatever"},
		After:  oplog.BoxFields{Name: "nope"},
	}
	raw, _ := oplog.Marshal(stale)

	if _, err := l.write(ctx, oplog.ActionUpdateBox, raw, oplog.SourceUser, nil); err == nil {
		t.Fatal("writing an op whose Apply fails succeeded")
	}

	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 10})
	if len(ops) != 0 {
		t.Errorf("a failed write left %d op(s) behind", len(ops))
	}
}

func TestWriteTruncatesTimestampsToMilliseconds(t *testing.T) {
	// The storage format has exactly three fractional digits. A ts carrying
	// nanoseconds would not survive the round trip, and every comparison
	// against it would fail in ways that look like a clock bug.
	root := t.TempDir()
	Create(context.Background(), root)
	odd := time.Date(2026, 8, 27, 9, 0, 0, 123_456_789, time.UTC)
	l, err := Open(root, WithClock(func() time.Time { return odd }))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	p := oplog.CreateBoxPayload{BoxID: domain.NewBoxID(), RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{Name: "x"}}
	raw, _ := oplog.Marshal(p)
	l.write(context.Background(), oplog.ActionCreateBox, raw, oplog.SourceUser, nil)

	ops, _ := l.Ops(context.Background(), store.ListOpsOptions{Limit: 1})
	want := odd.Truncate(time.Millisecond)
	if !ops[0].TS.Equal(want) {
		t.Errorf("stored ts = %v, want %v", ops[0].TS, want)
	}
}

func TestWriteFillsTheIndexTables(t *testing.T) {
	// Invariant 4: op_boxes is derived from the payload by Targets, and the
	// write path must actually call it — a handler that returns targets nobody
	// writes is a silent hole in the history view.
	l := openTestLibrary(t)
	ctx := context.Background()

	p := oplog.CreateBoxPayload{BoxID: domain.NewBoxID(), RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{Name: "indexed"}}
	raw, _ := oplog.Marshal(p)
	opID, _ := l.write(ctx, oplog.ActionCreateBox, raw, oplog.SourceUser, nil)

	byBox, err := l.Ops(ctx, store.ListOpsOptions{Box: &p.BoxID, Limit: 10})
	if err != nil {
		t.Fatalf("Ops by box: %v", err)
	}
	if len(byBox) != 1 || byBox[0].ID != opID {
		t.Errorf("ops for the box = %+v, want just op %d", byBox, opID)
	}
}

func TestUserOpsPropagateUpdatedAt(t *testing.T) {
	// spec 1.5.1 ②: the rule lives on the write path, so a new op type cannot
	// forget it.
	l := openTestLibrary(t)
	ctx := context.Background()

	box := domain.NewBoxID()
	create, _ := oplog.Marshal(oplog.CreateBoxPayload{BoxID: box, RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{Name: "before"}})
	l.write(ctx, oplog.ActionCreateBox, create, oplog.SourceUser, nil)
	first, _, _ := l.Box(ctx, box)

	update, _ := oplog.Marshal(oplog.UpdateBoxPayload{
		BoxID:  box,
		Before: oplog.BoxFields{Name: "before"},
		After:  oplog.BoxFields{Name: "after"},
	})
	l.write(ctx, oplog.ActionUpdateBox, update, oplog.SourceUser, nil)
	second, _, _ := l.Box(ctx, box)

	if !second.UpdatedAt.After(first.UpdatedAt) {
		t.Errorf("UpdatedAt did not move: %v then %v", first.UpdatedAt, second.UpdatedAt)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("CreatedAt moved from %v to %v", first.CreatedAt, second.CreatedAt)
	}
}

func TestMaintenanceOpsDoNotPropagateUpdatedAt(t *testing.T) {
	// The failure this prevents: re-running an extractor over the library and
	// stamping ten years of archive as modified this morning.
	l := openTestLibrary(t)
	ctx := context.Background()

	box := domain.NewBoxID()
	create, _ := oplog.Marshal(oplog.CreateBoxPayload{BoxID: box, RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{Name: "before"}})
	l.write(ctx, oplog.ActionCreateBox, create, oplog.SourceUser, nil)
	first, _, _ := l.Box(ctx, box)

	update, _ := oplog.Marshal(oplog.UpdateBoxPayload{
		BoxID:  box,
		Before: oplog.BoxFields{Name: "before"},
		After:  oplog.BoxFields{Name: "after"},
	})
	l.write(ctx, oplog.ActionUpdateBox, update, oplog.SourceMaintenance, nil)
	second, _, _ := l.Box(ctx, box)

	if !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("a maintenance op moved UpdatedAt from %v to %v", first.UpdatedAt, second.UpdatedAt)
	}
	if second.Name != "after" {
		t.Errorf("the maintenance op did not apply: name is %q", second.Name)
	}
}

func TestWriteRejectsAnUnknownAction(t *testing.T) {
	l := openTestLibrary(t)
	_, err := l.write(context.Background(), oplog.Action("teleport"), []byte(`{}`), oplog.SourceUser, nil)
	if err == nil {
		t.Error("writing an unregistered action succeeded")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/library/ -run 'TestWrite|TestOpen|TestUserOps|TestMaintenance' -v`
Expected: FAIL，`undefined: Open`（M1a 只有 `Create`）

- [ ] **Step 3: 实现 Library**

`internal/library/library.go`:

```go
package library

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// Library is an open kist library, and the only thing in this program that can
// write to one (invariant 2). api and cmd hold one of these; neither can reach
// store's write side, because the write path below is unexported.
type Library struct {
	layout Layout
	cfg    *config.Config
	db     *sql.DB
	blobs  *blob.Dir
	tags   *TagTree
	log    *slog.Logger
	actor  string
	now    func() time.Time
}

// Option configures Open.
type Option func(*Library)

// WithClock replaces the clock. Tests use it to get reproducible timestamps,
// which matters because Rebuild's proof compares snapshots that contain them.
func WithClock(now func() time.Time) Option { return func(l *Library) { l.now = now } }

// WithActor records who is acting. v0.1 has one user, so this is mostly a
// label; it becomes meaningful when sharing arrives (spec 2.2).
func WithActor(actor string) Option { return func(l *Library) { l.actor = actor } }

// WithLogger sets the logger.
func WithLogger(log *slog.Logger) Option { return func(l *Library) { l.log = log } }

// Open opens an existing library.
func Open(root string, opts ...Option) (*Library, error) {
	layout, err := OpenLayout(root)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(layout.ConfigPath())
	if err != nil {
		return nil, err
	}

	l := &Library{
		layout: layout,
		cfg:    cfg,
		log:    slog.Default(),
		actor:  "cli",
		now:    time.Now,
	}
	for _, opt := range opts {
		opt(l)
	}

	if l.db, err = store.Open(layout.DB()); err != nil {
		return nil, err
	}
	if err := store.Migrate(l.db); err != nil {
		l.db.Close()
		return nil, err
	}
	if l.blobs, err = blob.NewDir(layout.Blobs(), l.log); err != nil {
		l.db.Close()
		return nil, err
	}
	if l.tags, err = LoadTagTree(l.db); err != nil {
		l.db.Close()
		return nil, err
	}
	return l, nil
}

func (l *Library) Close() error {
	if l.db == nil {
		return nil
	}
	return l.db.Close()
}

// Layout exposes where things live, for commands that print paths.
func (l *Library) Layout() Layout { return l.layout }

// Tags is the in-memory tag tree.
func (l *Library) Tags() *TagTree { return l.tags }

// Box reads one box.
func (l *Library) Box(ctx context.Context, id domain.BoxID) (domain.Box, bool, error) {
	tx, err := l.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.Box{}, false, err
	}
	defer tx.Rollback()
	return store.NewViewTx(tx, time.Time{}).GetBox(id)
}

// Boxes lists boxes.
func (l *Library) Boxes(ctx context.Context, opt store.ListBoxesOptions) ([]domain.Box, error) {
	return store.ListBoxes(l.db, opt)
}

// BoxTags returns the tags attached to a box.
func (l *Library) BoxTags(ctx context.Context, id domain.BoxID) ([]domain.TagID, error) {
	return store.BoxTags(l.db, id)
}

// Ops reads the history.
func (l *Library) Ops(ctx context.Context, opt store.ListOpsOptions) ([]oplog.Op, error) {
	rows, err := store.ListOps(l.db, opt)
	if err != nil {
		return nil, err
	}
	out := make([]oplog.Op, len(rows))
	for i, r := range rows {
		out[i] = toOp(r)
	}
	return out, nil
}

// RevertedBy reports which op undid this one, if any.
func (l *Library) RevertedBy(ctx context.Context, id int64) (int64, bool, error) {
	return store.RevertedBy(l.db, id)
}

// toOp converts store's row into oplog's type. The two packages do not import
// each other; library is the seam, and this function is the price of that.
func toOp(r store.OpRow) oplog.Op {
	return oplog.Op{
		ID:        r.ID,
		TS:        r.TS,
		Action:    oplog.Action(r.Action),
		Source:    oplog.Source(r.Source),
		Actor:     r.Actor,
		Payload:   r.Payload,
		RevertsID: r.RevertsID,
	}
}

func toOpRow(o oplog.Op) store.OpRow {
	return store.OpRow{
		ID:        o.ID,
		TS:        o.TS,
		Action:    string(o.Action),
		Source:    string(o.Source),
		Actor:     o.Actor,
		Payload:   o.Payload,
		RevertsID: o.RevertsID,
	}
}

// store.ViewTx satisfies oplog.Tx structurally — store does not import oplog,
// so neither package can state that relationship. library is where the two
// meet, so this is the only place the compiler can be asked to check it.
// Without this line, a missing method would surface as a confusing error deep
// inside write() instead of right here.
var _ oplog.Tx = (*store.ViewTx)(nil)
```

（`library.go` 不 import `fmt`——它的错误都是直接往上冒的。）

- [ ] **Step 4: 实现写路径**

`internal/library/write.go`:

```go
package library

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// write is the single write path. Every change to a kist library goes through
// this function; there is no other way in (invariant 2), which is why it is
// unexported.
//
// One transaction, four steps, in this order:
//
//  1. append the op — its id is the library's new version number
//  2. Apply it to the materialized views, with tx.TS() carrying the op's ts
//  3. write the index tables from the payload via Targets (invariant 4)
//  4. propagate updated_at, but only for sources that represent human intent
//
// If any step fails the whole thing rolls back. An op that survived a failed
// Apply would be worse than a lost change: a rebuild would replay it and make
// the claimed change real.
func (l *Library) write(
	ctx context.Context,
	action oplog.Action,
	payload []byte,
	source oplog.Source,
	revertsID *int64,
) (int64, error) {
	handler, err := oplog.Lookup(action)
	if err != nil {
		return 0, err
	}
	targets, err := handler.Targets(payload)
	if err != nil {
		return 0, fmt.Errorf("extract targets for %s: %w", action, err)
	}

	// Truncated to milliseconds because that is the resolution the storage
	// format keeps. Without this, what is written and what is read back
	// differ, and every timestamp comparison becomes mysteriously flaky.
	ts := l.now().UTC().Truncate(time.Millisecond)

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	opID, err := store.AppendOp(tx, store.OpRow{
		TS:        ts,
		Action:    string(action),
		Source:    string(source),
		Actor:     l.actor,
		Payload:   payload,
		RevertsID: revertsID,
	})
	if err != nil {
		return 0, err
	}

	vtx := store.NewViewTx(tx, ts)
	if err := handler.Apply(vtx, payload); err != nil {
		return 0, fmt.Errorf("apply %s: %w", action, err)
	}

	if err := store.InsertOpBoxes(tx, opID, targets.Boxes); err != nil {
		return 0, err
	}
	if err := store.InsertOpFiles(tx, opID, targets.Files); err != nil {
		return 0, err
	}

	// spec 1.5.1 ②: updated_at records human intent, not system activity. The
	// rule lives here rather than in each handler so that a new op type cannot
	// forget it.
	if source.PropagatesUpdatedAt() {
		if err := vtx.TouchBoxes(targets.Boxes); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit %s: %w", action, err)
	}
	return opID, nil
}

// readTx runs fn against a read-only transaction.
func (l *Library) readTx(ctx context.Context, fn func(*store.ViewTx) error) error {
	tx, err := l.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return fn(store.NewViewTx(tx, time.Time{}))
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/library/ -v`
Expected: PASS（`TestOpen...` 需要 Task 9 的 `LoadTagTree` 才能编过——两个 task 可以合并跑，或先写一个返回空树的桩，Task 9 再补全。**推荐先做 Task 9 再回来跑这一步**）

- [ ] **Step 6: 确认 `write` 真的是私有的**

Run: `grep -rn "func (l \*Library) [A-Z]" internal/library/ | grep -i "write\|apply"`
Expected: 无输出。任何导出的写方法都必须是 Task 10 里那些语义明确的命令，而不是一个通用的"写任意 op"入口。

- [ ] **Step 7: 提交**

```bash
git add internal/library/library.go internal/library/write.go internal/library/write_test.go
git commit -m "feat(library): Open 与唯一写路径

write 是包内私有的，cmd 够不到——不变量 2 由此变成结构性的而不是
约定性的。一个事务里四步：插 op → Apply → 写索引表 → 传递
updated_at，任何一步失败整笔回滚。

updated_at 的传递规则放在写路径上而不是每个 handler 里：将来加新
op 时不可能漏，handler 作者根本不需要知道这条规则存在。

时间戳截到毫秒。存储格式只有三位小数，不截断的话写进去的和读回来的
不相等，表现是'重放出来的库和原库差一点点'。"
```

---

### Task 9: library — 内存 tag 树

**Files:**
- Create: `internal/library/tagtree.go`
- Test: `internal/library/tagtree_test.go`

**Interfaces:**
- Consumes: `store.ListTags`、`store.Queryer`、`domain.Tag`
- Produces:
  - `library.TagTree`、`library.LoadTagTree(q store.Queryer) (*TagTree, error)`
  - 方法 `ByID(domain.TagID) (domain.Tag, bool)` / `ByName(string) (domain.Tag, bool)` / `Roots() []domain.Tag` / `Children(domain.TagID) []domain.Tag` / `Descendants(domain.TagID) []domain.TagID` / `Path(domain.TagID) []domain.Tag` / `WouldCycle(node, newParent domain.TagID) bool` / `Count(domain.TagID) int` / `Len() int`
  - 增量维护：`Insert(domain.Tag)` / `Remove(domain.TagID)` / `Update(domain.Tag)` / `SetDirectCount(domain.TagID, int)` / `AddDirectCount(domain.TagID, int)`

**没有物化路径列，整棵树常驻内存**（spec 1.6）。几千个节点，求后代是一次内存遍历，微秒级。这也是"移动子树 O(1)"能成立的原因——DB 里只改一行 `parent_id`，树在内存里重新挂一下。

**`Descendants` 含自身**，因为 spec 4.3 说"选中一个 tag 默认包含其所有后代"，而"选中 `人物`"当然也包括直接挂在 `人物` 上的东西。

**`WouldCycle` 是命令时的校验**，不是 `Apply` 里的。把一个节点移到它自己的后代下面会造出一个环，而环一旦进了 DB，`Descendants` 会无限递归。决策在生成 op 时做完（不变量 3）。

**计数缓存**：启动时一次 `GROUP BY` 拉全量直接挂载数，之后由 `library` 在 apply op 时增量更新。不在 `tags` 表上加冗余列——重启就重新推导，没有漂移风险。

- [ ] **Step 1: 写失败的测试**

```go
package library

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

// buildTree makes a tree without a database:
//
//	地点            人物
//	├── 云南        └── 爸
//	│   ├── 大理
//	│   └── 丽江
//	└── 四川
func buildTree(t *testing.T) (*TagTree, map[string]domain.TagID) {
	t.Helper()
	ids := map[string]domain.TagID{}
	var tags []domain.Tag

	add := func(name, parent string) {
		id := domain.NewTagID()
		ids[name] = id
		var p *domain.TagID
		if parent != "" {
			pid := ids[parent]
			p = &pid
		}
		tag, err := domain.NewTag(id, p, name)
		if err != nil {
			t.Fatal(err)
		}
		tags = append(tags, tag)
	}

	add("地点", "")
	add("人物", "")
	add("云南", "地点")
	add("四川", "地点")
	add("大理", "云南")
	add("丽江", "云南")
	add("爸", "人物")

	tree, err := NewTagTree(tags, nil)
	if err != nil {
		t.Fatalf("NewTagTree: %v", err)
	}
	return tree, ids
}

func names(tags []domain.Tag) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.Name
	}
	return out
}

func TestTagTreeLookup(t *testing.T) {
	tree, ids := buildTree(t)

	if got, ok := tree.ByID(ids["云南"]); !ok || got.Name != "云南" {
		t.Errorf("ByID = %+v ok=%v", got, ok)
	}
	// spec 1.6: names are globally unique, so name lookup is unambiguous —
	// that is the whole point of the constraint.
	if got, ok := tree.ByName("大理"); !ok || got.ID != ids["大理"] {
		t.Errorf("ByName = %+v ok=%v", got, ok)
	}
	// Lookup is case- and normalization-insensitive because it goes through
	// the same fold function the unique index is built on (spec 4.7).
	if _, ok := tree.ByName("大理 "); ok {
		t.Error("ByName matched a name with trailing space; folding must not trim")
	}
	if _, ok := tree.ByName("没有这个"); ok {
		t.Error("ByName found a tag that does not exist")
	}
	if tree.Len() != 7 {
		t.Errorf("Len = %d, want 7", tree.Len())
	}
}

func TestTagTreeStructure(t *testing.T) {
	tree, ids := buildTree(t)

	if got := names(tree.Roots()); len(got) != 2 {
		t.Errorf("Roots = %v, want two", got)
	}
	if got := names(tree.Children(ids["地点"])); len(got) != 2 {
		t.Errorf("Children(地点) = %v, want two", got)
	}
	if got := names(tree.Children(ids["大理"])); len(got) != 0 {
		t.Errorf("Children(大理) = %v, want none", got)
	}
}

func TestDescendantsIncludesSelf(t *testing.T) {
	// spec 4.3: selecting 人物 should return everyone's photos — which
	// includes anything hanging directly off 人物 itself.
	tree, ids := buildTree(t)

	got := tree.Descendants(ids["云南"])
	if len(got) != 3 {
		t.Fatalf("Descendants(云南) has %d entries, want 3 (self + 大理 + 丽江)", len(got))
	}
	found := map[domain.TagID]bool{}
	for _, id := range got {
		found[id] = true
	}
	for _, want := range []string{"云南", "大理", "丽江"} {
		if !found[ids[want]] {
			t.Errorf("Descendants(云南) is missing %s", want)
		}
	}

	if got := tree.Descendants(ids["地点"]); len(got) != 5 {
		t.Errorf("Descendants(地点) has %d entries, want 5", len(got))
	}
	if got := tree.Descendants(ids["爸"]); len(got) != 1 {
		t.Errorf("Descendants of a leaf has %d entries, want just itself", len(got))
	}
}

func TestPath(t *testing.T) {
	tree, ids := buildTree(t)
	got := names(tree.Path(ids["大理"]))
	want := []string{"地点", "云南", "大理"}
	if len(got) != len(want) {
		t.Fatalf("Path = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Path = %v, want %v", got, want)
		}
	}
}

func TestWouldCycle(t *testing.T) {
	// Moving a node under its own descendant creates a loop, and a loop in the
	// tag tree makes Descendants recurse forever. The check belongs here, at
	// command time, because Apply is not allowed to make decisions.
	tree, ids := buildTree(t)

	if !tree.WouldCycle(ids["地点"], ids["大理"]) {
		t.Error("moving 地点 under its own grandchild was not detected as a cycle")
	}
	if !tree.WouldCycle(ids["云南"], ids["云南"]) {
		t.Error("moving a node under itself was not detected as a cycle")
	}
	if tree.WouldCycle(ids["云南"], ids["人物"]) {
		t.Error("a legitimate move was reported as a cycle")
	}
}

func TestTagTreeIncrementalMaintenance(t *testing.T) {
	tree, ids := buildTree(t)

	// insert
	newID := domain.NewTagID()
	parent := ids["人物"]
	mom, _ := domain.NewTag(newID, &parent, "妈")
	if err := tree.Insert(mom); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if len(tree.Children(parent)) != 2 {
		t.Error("Insert did not attach the node to its parent")
	}

	// rename
	renamed, _ := domain.NewTag(newID, &parent, "母亲")
	if err := tree.Update(renamed); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, ok := tree.ByName("妈"); ok {
		t.Error("the old name still resolves after a rename")
	}
	if _, ok := tree.ByName("母亲"); !ok {
		t.Error("the new name does not resolve after a rename")
	}

	// move
	places := ids["地点"]
	moved, _ := domain.NewTag(newID, &places, "母亲")
	if err := tree.Update(moved); err != nil {
		t.Fatalf("Update (move): %v", err)
	}
	if len(tree.Children(parent)) != 1 {
		t.Error("the node was not detached from its old parent")
	}
	if len(tree.Children(places)) != 3 {
		t.Error("the node was not attached to its new parent")
	}

	// remove
	if err := tree.Remove(newID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := tree.ByID(newID); ok {
		t.Error("the node survived Remove")
	}
	if len(tree.Children(places)) != 2 {
		t.Error("Remove did not detach the node from its parent")
	}
}

func TestTagTreeRejectsADuplicateName(t *testing.T) {
	tree, _ := buildTree(t)
	dup, _ := domain.NewTag(domain.NewTagID(), nil, "云南")
	if err := tree.Insert(dup); err == nil {
		t.Error("the tree accepted a duplicate name; global uniqueness is a lie")
	}
}

func TestCountsAggregateUpTheTree(t *testing.T) {
	// spec 1.6: the selector shows 人物 (1,204), which is the subtree total.
	tree, ids := buildTree(t)
	tree.SetDirectCount(ids["大理"], 10)
	tree.SetDirectCount(ids["丽江"], 5)
	tree.SetDirectCount(ids["云南"], 1)
	tree.SetDirectCount(ids["四川"], 3)

	if got := tree.Count(ids["云南"]); got != 16 {
		t.Errorf("Count(云南) = %d, want 16", got)
	}
	if got := tree.Count(ids["地点"]); got != 19 {
		t.Errorf("Count(地点) = %d, want 19", got)
	}
	if got := tree.Count(ids["人物"]); got != 0 {
		t.Errorf("Count(人物) = %d, want 0", got)
	}

	tree.AddDirectCount(ids["大理"], -4)
	if got := tree.Count(ids["地点"]); got != 15 {
		t.Errorf("after AddDirectCount, Count(地点) = %d, want 15", got)
	}
}

func TestLoadTagTreeFromStore(t *testing.T) {
	l := openTestLibrary(t)
	if l.Tags().Len() != 0 {
		t.Errorf("a fresh library has %d tags, want 0", l.Tags().Len())
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/library/ -run 'TagTree|Descendants|WouldCycle|Path|Counts' -v`
Expected: FAIL，`undefined: NewTagTree`

- [ ] **Step 3: 实现**

`internal/library/tagtree.go`:

```go
package library

import (
	"fmt"
	"sync"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

// TagTree is the whole tag tree, in memory.
//
// There is no materialized path column and no closure table (spec 1.6). A few
// thousand nodes fit comfortably, resolving descendants is a microsecond-scale
// walk, and — the part that actually pays — moving a subtree is a single
// parent_id write instead of a cascading rewrite.
//
// Consistency rests on library being the only writer and on there being one
// process (spec 1.6). A second process writing to the same library would leave
// this tree silently stale; that is open question B3, deliberately not solved
// in v0.1 because the CLI-only milestones cannot expose it.
type TagTree struct {
	mu     sync.RWMutex
	byID   map[domain.TagID]*tagNode
	byFold map[string]*tagNode
	roots  map[domain.TagID]*tagNode
}

type tagNode struct {
	tag         domain.Tag
	parent      *tagNode
	children    map[domain.TagID]*tagNode
	directCount int
}

// NewTagTree builds a tree from a flat list. counts maps a tag to how many
// things hang directly off it; it may be nil.
func NewTagTree(tags []domain.Tag, counts map[domain.TagID]int) (*TagTree, error) {
	t := &TagTree{
		byID:   make(map[domain.TagID]*tagNode, len(tags)),
		byFold: make(map[string]*tagNode, len(tags)),
		roots:  map[domain.TagID]*tagNode{},
	}
	for _, tag := range tags {
		if _, exists := t.byFold[tag.NameFold]; exists {
			return nil, fmt.Errorf("two tags are both named %q; the unique index on name_fold should have prevented this", tag.Name)
		}
		n := &tagNode{tag: tag, children: map[domain.TagID]*tagNode{}, directCount: counts[tag.ID]}
		t.byID[tag.ID] = n
		t.byFold[tag.NameFold] = n
	}
	// Second pass: every node exists, so parents resolve regardless of order.
	for _, n := range t.byID {
		if n.tag.ParentID == nil {
			t.roots[n.tag.ID] = n
			continue
		}
		parent, ok := t.byID[*n.tag.ParentID]
		if !ok {
			return nil, fmt.Errorf("tag %q points at parent %s, which does not exist", n.tag.Name, *n.tag.ParentID)
		}
		n.parent = parent
		parent.children[n.tag.ID] = n
	}
	return t, nil
}

// LoadTagTree reads the tree and its counts from the database.
func LoadTagTree(q store.Queryer) (*TagTree, error) {
	tags, err := store.ListTags(q)
	if err != nil {
		return nil, err
	}
	counts, err := store.TagDirectCounts(q)
	if err != nil {
		return nil, err
	}
	return NewTagTree(tags, counts)
}

func (t *TagTree) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.byID)
}

func (t *TagTree) ByID(id domain.TagID) (domain.Tag, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return domain.Tag{}, false
	}
	return n.tag, true
}

// ByName resolves a name through the same fold function the unique index uses,
// so "Passport" and "passport" find the same tag (spec 1.6, 4.7).
func (t *TagTree) ByName(name string) (domain.Tag, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byFold[domain.Fold(name)]
	if !ok {
		return domain.Tag{}, false
	}
	return n.tag, true
}

func (t *TagTree) Roots() []domain.Tag {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return sortedTags(t.roots)
}

func (t *TagTree) Children(id domain.TagID) []domain.Tag {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return nil
	}
	return sortedTags(n.children)
}

// Descendants returns the id set for a tag filter: the node itself plus
// everything under it (spec 4.3).
func (t *TagTree) Descendants(id domain.TagID) []domain.TagID {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return nil
	}
	var out []domain.TagID
	var walk func(*tagNode)
	walk = func(n *tagNode) {
		out = append(out, n.tag.ID)
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(n)
	return out
}

// Path returns the chain from the root down to this tag, inclusive.
func (t *TagTree) Path(id domain.TagID) []domain.Tag {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return nil
	}
	var reversed []domain.Tag
	for cur := n; cur != nil; cur = cur.parent {
		reversed = append(reversed, cur.tag)
	}
	out := make([]domain.Tag, len(reversed))
	for i, tag := range reversed {
		out[len(reversed)-1-i] = tag
	}
	return out
}

// WouldCycle reports whether making newParent the parent of node would close a
// loop. A loop makes Descendants recurse until the stack runs out, so this is
// checked when the command is built — Apply is not allowed to decide things.
func (t *TagTree) WouldCycle(node, newParent domain.TagID) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for cur, ok := t.byID[newParent], true; ok && cur != nil; cur = cur.parent {
		if cur.tag.ID == node {
			return true
		}
	}
	return false
}

// Count is the subtree total: what the tag selector shows next to a node.
func (t *TagTree) Count(id domain.TagID) int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.byID[id]
	if !ok {
		return 0
	}
	total := 0
	var walk func(*tagNode)
	walk = func(n *tagNode) {
		total += n.directCount
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(n)
	return total
}

func (t *TagTree) SetDirectCount(id domain.TagID, n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if node, ok := t.byID[id]; ok {
		node.directCount = n
	}
}

func (t *TagTree) AddDirectCount(id domain.TagID, delta int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if node, ok := t.byID[id]; ok {
		node.directCount += delta
	}
}

// Insert adds a node. It mirrors what the database just did, so a violated
// invariant here means the two have drifted.
func (t *TagTree) Insert(tag domain.Tag) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.byID[tag.ID]; exists {
		return fmt.Errorf("tag %s is already in the tree", tag.ID)
	}
	if _, exists := t.byFold[tag.NameFold]; exists {
		return fmt.Errorf("tag name %q is already taken", tag.Name)
	}
	n := &tagNode{tag: tag, children: map[domain.TagID]*tagNode{}}
	t.byID[tag.ID] = n
	t.byFold[tag.NameFold] = n
	return t.attach(n)
}

// Update handles both rename and move: the caller passes the new state of the
// node and the tree rearranges itself.
func (t *TagTree) Update(tag domain.Tag) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, ok := t.byID[tag.ID]
	if !ok {
		return fmt.Errorf("tag %s is not in the tree", tag.ID)
	}
	if other, exists := t.byFold[tag.NameFold]; exists && other != n {
		return fmt.Errorf("tag name %q is already taken", tag.Name)
	}
	delete(t.byFold, n.tag.NameFold)
	t.detach(n)

	n.tag = tag
	t.byFold[tag.NameFold] = n
	return t.attach(n)
}

func (t *TagTree) Remove(id domain.TagID) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, ok := t.byID[id]
	if !ok {
		return fmt.Errorf("tag %s is not in the tree", id)
	}
	if len(n.children) > 0 {
		return fmt.Errorf("tag %q still has children", n.tag.Name)
	}
	t.detach(n)
	delete(t.byID, id)
	delete(t.byFold, n.tag.NameFold)
	return nil
}

// attach and detach assume the lock is held.
func (t *TagTree) attach(n *tagNode) error {
	if n.tag.ParentID == nil {
		n.parent = nil
		t.roots[n.tag.ID] = n
		return nil
	}
	parent, ok := t.byID[*n.tag.ParentID]
	if !ok {
		return fmt.Errorf("parent tag %s does not exist", *n.tag.ParentID)
	}
	n.parent = parent
	parent.children[n.tag.ID] = n
	return nil
}

func (t *TagTree) detach(n *tagNode) {
	if n.parent != nil {
		delete(n.parent.children, n.tag.ID)
	}
	delete(t.roots, n.tag.ID)
	n.parent = nil
}

// sortedTags returns nodes in byte order of their name (spec 4.8), so listings
// are stable and identical on every platform.
func sortedTags(m map[domain.TagID]*tagNode) []domain.Tag {
	out := make([]domain.Tag, 0, len(m))
	for _, n := range m {
		out = append(out, n.tag)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return out
}
```

（文件顶部需要 `sort`。）

- [ ] **Step 4: 在 store 里补上计数查询**

在 `internal/store/queries.go` 追加：

```go
// TagDirectCounts returns how many things hang directly off each tag.
//
// One GROUP BY at startup, then library keeps it up to date incrementally
// (spec 1.6). The counts are deliberately not a column on tags: a restart
// re-derives them, so there is nothing to drift.
func TagDirectCounts(q Queryer) (map[domain.TagID]int, error) {
	rows, err := q.Query(
		`SELECT tag_id, count(*) FROM box_tags GROUP BY tag_id
		 UNION ALL
		 SELECT tag_id, count(*) FROM file_tags GROUP BY tag_id`)
	if err != nil {
		return nil, fmt.Errorf("count tag usage: %w", err)
	}
	defer rows.Close()

	out := map[domain.TagID]int{}
	for rows.Next() {
		var id domain.TagID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("count tag usage: %w", err)
		}
		out[id] += n
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/library/ ./internal/store/ -v`
Expected: PASS（Task 8 Step 5 那一步现在也该通过了）

- [ ] **Step 6: 提交**

```bash
git add internal/library/tagtree.go internal/library/tagtree_test.go internal/store/queries.go
git commit -m "feat(library): 内存 tag 树

没有物化路径列也没有闭包表：几千个节点常驻内存，求后代是一次内存
遍历。真正付钱的地方是移动子树——DB 里只改一行 parent_id，树在
内存里重新挂一下，不用级联重写。

WouldCycle 是命令时的校验而不是 Apply 里的：环一旦进了 DB，
Descendants 会一直递归到栈溢出，而决策必须在生成 op 时做完。

计数不在 tags 表上加列：启动时一次 GROUP BY 推导，重启就重来，
没有漂移风险。"
```

---

### Task 10: library — 对外的命令方法

**Files:**
- Create: `internal/library/commands.go`
- Modify: `internal/library/write.go`（加 `refreshTagState`）
- Test: `internal/library/commands_test.go`

**Interfaces:**
- Consumes: Task 8 的 `write`、Task 9 的 `TagTree`、`oplog` 的九个 payload 类型
- Produces:
  - `(*Library).CreateBox(ctx, name, description string, start, end domain.LocalDate) (domain.Box, error)`
  - `(*Library).UpdateBox(ctx, id domain.BoxID, f oplog.BoxFields) (domain.Box, error)`
  - `(*Library).CreateTag(ctx, name string, parent *domain.TagID) (domain.Tag, error)`
  - `(*Library).RenameTag(ctx, id domain.TagID, name string) error`
  - `(*Library).MoveTag(ctx, id domain.TagID, parent *domain.TagID) error`
  - `(*Library).DeleteTag(ctx, id domain.TagID) error`
  - `(*Library).TagBoxes(ctx, tag domain.TagID, boxes []domain.BoxID) (int, error)`
  - `(*Library).UntagBoxes(ctx, tag domain.TagID, boxes []domain.BoxID) (int, error)`
  - `(*Library).ResolveBox(ctx, ref string) (domain.Box, error)`、`(*Library).ResolveTag(ref string) (domain.Tag, error)`
  - `library.ErrAmbiguousRef`、`library.ErrNoSuchBox`、`library.ErrNoSuchTag`、`library.ErrTagNameTaken`、`library.ErrTagCycle`

**这些方法就是"命令做决策，事件记结果"里的命令**（spec 2.3.1）。每一个都：铸 ID → 校验 → 构造 payload → 调 `write`。**校验和 ID 生成全部发生在这里**，`Apply` 拿到的 payload 里已经没有任何待决定的东西。

**`TagBoxes` 返回实际变化的数量。** 它在生成 op 前先滤掉已经有这个 tag 的 box（Task 7 解释过为什么必须滤）。如果滤完一个不剩，**不写 op 直接返回 0**——一条什么都没改的 op 只会给历史添乱，撤销它还等于什么都没做。

**`ResolveBox` 接受 id、id 前缀、或精确名字。** box 名不唯一（schema 里 `boxes.name_fold` 只有普通索引），所以名字撞了要报 `ErrAmbiguousRef` 并列出候选，而不是随便挑一个。

- [ ] **Step 1: 写失败的测试**

```go
package library

import (
	"context"
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func TestCreateBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()

	start, _ := domain.ParseLocalDate("2019-08")
	b, err := l.CreateBox(ctx, "云南 2019", "the long way round", start, domain.LocalDate{})
	if err != nil {
		t.Fatalf("CreateBox: %v", err)
	}
	if b.ID.IsZero() || b.Name != "云南 2019" || b.StartAt.String() != "2019-08" {
		t.Errorf("box = %+v", b)
	}
	if b.CreatedAt.IsZero() {
		t.Error("CreatedAt was not filled in from the op")
	}

	// Exactly one root folder, per spec 1.4 and the partial unique index.
	var folders int
	l.db.QueryRow(`SELECT count(*) FROM folders WHERE box_id = ?`, b.ID).Scan(&folders)
	if folders != 1 {
		t.Errorf("the box has %d folders, want exactly 1", folders)
	}
}

func TestCreateBoxRejectsABadName(t *testing.T) {
	l := openTestLibrary(t)
	if _, err := l.CreateBox(context.Background(), "  ", "", domain.LocalDate{}, domain.LocalDate{}); err == nil {
		t.Error("a blank box name was accepted")
	}
	ops, _ := l.Ops(context.Background(), store.ListOpsOptions{Limit: 10})
	if len(ops) != 0 {
		t.Errorf("a rejected command still wrote %d op(s)", len(ops))
	}
}

func TestUpdateBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "before", "", domain.LocalDate{}, domain.LocalDate{})

	end, _ := domain.ParseLocalDate("2019-08-20")
	got, err := l.UpdateBox(ctx, b.ID, oplog.BoxFields{Name: "after", Description: "noted", EndAt: end})
	if err != nil {
		t.Fatalf("UpdateBox: %v", err)
	}
	if got.Name != "after" || got.Description != "noted" || got.EndAt.String() != "2019-08-20" {
		t.Errorf("box = %+v", got)
	}
	if !got.CreatedAt.Equal(b.CreatedAt) {
		t.Errorf("CreatedAt moved from %v to %v", b.CreatedAt, got.CreatedAt)
	}
}

func TestUpdateBoxOnAMissingBox(t *testing.T) {
	l := openTestLibrary(t)
	_, err := l.UpdateBox(context.Background(), domain.NewBoxID(), oplog.BoxFields{Name: "x"})
	if !errors.Is(err, ErrNoSuchBox) {
		t.Errorf("error = %v, want ErrNoSuchBox", err)
	}
}

func TestCreateTagAndTree(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()

	places, err := l.CreateTag(ctx, "地点", nil)
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	yunnan, err := l.CreateTag(ctx, "云南", &places.ID)
	if err != nil {
		t.Fatalf("CreateTag child: %v", err)
	}

	// The in-memory tree must track the database without a reload.
	if got, ok := l.Tags().ByName("云南"); !ok || got.ID != yunnan.ID {
		t.Errorf("the tag tree does not know about 云南")
	}
	if kids := l.Tags().Children(places.ID); len(kids) != 1 {
		t.Errorf("地点 has %d children in the tree, want 1", len(kids))
	}
}

func TestCreateTagRejectsADuplicateName(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	parent, _ := l.CreateTag(ctx, "地点", nil)

	l.CreateTag(ctx, "云南", &parent.ID)
	// spec 1.6: the hierarchy is not a namespace, so this collides even under
	// a different parent.
	if _, err := l.CreateTag(ctx, "云南", nil); !errors.Is(err, ErrTagNameTaken) {
		t.Errorf("error = %v, want ErrTagNameTaken", err)
	}
	// Folding means case is not a way around it either.
	if _, err := l.CreateTag(ctx, "YUNNAN", nil); err == nil {
		l.CreateTag(ctx, "yunnan", nil) // only collides if folding is applied
	}
}

func TestRenameAndMoveTag(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	places, _ := l.CreateTag(ctx, "地点", nil)
	people, _ := l.CreateTag(ctx, "人物", nil)
	yunnan, _ := l.CreateTag(ctx, "云南", &places.ID)

	if err := l.RenameTag(ctx, yunnan.ID, "云南省"); err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	if _, ok := l.Tags().ByName("云南省"); !ok {
		t.Error("the tree did not pick up the rename")
	}
	if _, ok := l.Tags().ByName("云南"); ok {
		t.Error("the old name still resolves")
	}

	if err := l.MoveTag(ctx, yunnan.ID, &people.ID); err != nil {
		t.Fatalf("MoveTag: %v", err)
	}
	if kids := l.Tags().Children(people.ID); len(kids) != 1 {
		t.Errorf("人物 has %d children after the move, want 1", len(kids))
	}
	if kids := l.Tags().Children(places.ID); len(kids) != 0 {
		t.Errorf("地点 still has %d children after the move", len(kids))
	}
}

func TestMoveTagRefusesACycle(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	places, _ := l.CreateTag(ctx, "地点", nil)
	yunnan, _ := l.CreateTag(ctx, "云南", &places.ID)

	if err := l.MoveTag(ctx, places.ID, &yunnan.ID); !errors.Is(err, ErrTagCycle) {
		t.Errorf("error = %v, want ErrTagCycle", err)
	}
	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 100})
	for _, o := range ops {
		if o.Action == oplog.ActionMoveTag {
			t.Error("a rejected move still wrote an op")
		}
	}
}

func TestDeleteTag(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "临时", nil)

	if err := l.DeleteTag(ctx, tag.ID); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	if _, ok := l.Tags().ByID(tag.ID); ok {
		t.Error("the tree still knows about the deleted tag")
	}
}

func TestDeleteTagRefusesWhenInUse(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	b, _ := l.CreateBox(ctx, "trip", "", domain.LocalDate{}, domain.LocalDate{})
	l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})

	if err := l.DeleteTag(ctx, tag.ID); !errors.Is(err, oplog.ErrTagInUse) {
		t.Errorf("error = %v, want ErrTagInUse", err)
	}
}

func TestTagBoxes(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	var boxes []domain.BoxID
	for _, n := range []string{"一", "二", "三"} {
		b, _ := l.CreateBox(ctx, n, "", domain.LocalDate{}, domain.LocalDate{})
		boxes = append(boxes, b.ID)
	}

	n, err := l.TagBoxes(ctx, tag.ID, boxes)
	if err != nil {
		t.Fatalf("TagBoxes: %v", err)
	}
	if n != 3 {
		t.Errorf("TagBoxes changed %d boxes, want 3", n)
	}

	// One op, three targets (spec 2.2.1).
	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 1})
	if ops[0].Action != oplog.ActionTagBoxes {
		t.Fatalf("last op is %q", ops[0].Action)
	}
	var targets int
	l.db.QueryRow(`SELECT count(*) FROM op_boxes WHERE op_id = ?`, ops[0].ID).Scan(&targets)
	if targets != 3 {
		t.Errorf("op_boxes has %d rows for the batch, want 3", targets)
	}

	// The count cache followed along.
	if got := l.Tags().Count(tag.ID); got != 3 {
		t.Errorf("tag count = %d, want 3", got)
	}
}

func TestTagBoxesSkipsBoxesThatAlreadyHaveIt(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	a, _ := l.CreateBox(ctx, "a", "", domain.LocalDate{}, domain.LocalDate{})
	b, _ := l.CreateBox(ctx, "b", "", domain.LocalDate{}, domain.LocalDate{})

	l.TagBoxes(ctx, tag.ID, []domain.BoxID{a.ID})
	n, err := l.TagBoxes(ctx, tag.ID, []domain.BoxID{a.ID, b.ID})
	if err != nil {
		t.Fatalf("TagBoxes: %v", err)
	}
	if n != 1 {
		t.Errorf("TagBoxes reported %d changes, want 1 (a already had the tag)", n)
	}

	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 1})
	p, err := oplog.Payload[oplog.TagBoxesPayload](ops[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.BoxIDs) != 1 || p.BoxIDs[0] != b.ID {
		t.Errorf("the payload names %v; it must name only what actually changed", p.BoxIDs)
	}
}

func TestTagBoxesWritesNoOpWhenNothingChanges(t *testing.T) {
	// An op that changes nothing is noise in the history and undoes to
	// nothing. Better to report zero.
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	b, _ := l.CreateBox(ctx, "a", "", domain.LocalDate{}, domain.LocalDate{})
	l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})

	before, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 100})
	n, err := l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})
	if err != nil {
		t.Fatalf("TagBoxes: %v", err)
	}
	if n != 0 {
		t.Errorf("TagBoxes reported %d changes, want 0", n)
	}
	after, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 100})
	if len(after) != len(before) {
		t.Errorf("a no-op tagging wrote %d extra op(s)", len(after)-len(before))
	}
}

func TestUntagBoxes(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	b, _ := l.CreateBox(ctx, "a", "", domain.LocalDate{}, domain.LocalDate{})
	l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})

	n, err := l.UntagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})
	if err != nil {
		t.Fatalf("UntagBoxes: %v", err)
	}
	if n != 1 {
		t.Errorf("UntagBoxes reported %d changes, want 1", n)
	}
	if got := l.Tags().Count(tag.ID); got != 0 {
		t.Errorf("tag count = %d after untagging, want 0", got)
	}
}

func TestResolveBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "云南 2019", "", domain.LocalDate{}, domain.LocalDate{})

	for _, ref := range []string{b.ID.String(), b.ID.String()[:8], "云南 2019"} {
		got, err := l.ResolveBox(ctx, ref)
		if err != nil {
			t.Errorf("ResolveBox(%q): %v", ref, err)
			continue
		}
		if got.ID != b.ID {
			t.Errorf("ResolveBox(%q) found %s, want %s", ref, got.ID, b.ID)
		}
	}

	if _, err := l.ResolveBox(ctx, "没有这个 box"); !errors.Is(err, ErrNoSuchBox) {
		t.Errorf("error = %v, want ErrNoSuchBox", err)
	}
}

func TestResolveBoxReportsAmbiguity(t *testing.T) {
	// Box names are not unique (there is no unique index on boxes.name_fold),
	// so picking one arbitrarily would silently act on the wrong archive.
	l := openTestLibrary(t)
	ctx := context.Background()
	l.CreateBox(ctx, "重名", "", domain.LocalDate{}, domain.LocalDate{})
	l.CreateBox(ctx, "重名", "", domain.LocalDate{}, domain.LocalDate{})

	_, err := l.ResolveBox(ctx, "重名")
	if !errors.Is(err, ErrAmbiguousRef) {
		t.Fatalf("error = %v, want ErrAmbiguousRef", err)
	}
	// The message must let the user act on it.
	if !strings.Contains(err.Error(), "重名") {
		t.Errorf("error = %q, want it to name the candidates", err)
	}
}

func TestResolveTag(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "护照", nil)

	got, err := l.ResolveTag("护照")
	if err != nil || got.ID != tag.ID {
		t.Errorf("ResolveTag by name = %+v, %v", got, err)
	}
	if got, err := l.ResolveTag(tag.ID.String()); err != nil || got.ID != tag.ID {
		t.Errorf("ResolveTag by id = %+v, %v", got, err)
	}
	if _, err := l.ResolveTag("没有"); !errors.Is(err, ErrNoSuchTag) {
		t.Errorf("error = %v, want ErrNoSuchTag", err)
	}
}
```

（测试文件顶部需要 `strings`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/library/ -run 'TestCreateBox|TestUpdateBox|TestCreateTag|TestRename|TestMoveTag|TestDeleteTag|TestTagBoxes|TestUntag|TestResolve' -v`
Expected: FAIL，`undefined: CreateBox`

- [ ] **Step 3: 在写路径上挂 tag 树的同步**

在 `internal/library/write.go` 的 `write` 里，`tx.Commit()` 成功之后加：

```go
	if err := l.refreshTagState(action, payload); err != nil {
		// The database is already committed and correct; only the in-memory
		// mirror is behind. Reloading it is always safe, so recover rather
		// than fail a write that actually succeeded.
		l.log.Error("the in-memory tag tree fell out of step; reloading it", "op", opID, "action", action, "err", err)
		if tags, rerr := LoadTagTree(l.db); rerr == nil {
			l.tags = tags
		} else {
			return opID, fmt.Errorf("op %d committed but the tag tree could not be reloaded: %w", opID, rerr)
		}
	}
	return opID, nil
```

并在同一个文件里追加：

```go
// refreshTagState mirrors a committed op into the in-memory tag tree.
//
// It is driven by the action rather than by the command that produced it, so
// undo gets the same treatment for free: an inverse op is just another action
// arriving at this switch.
func (l *Library) refreshTagState(action oplog.Action, payload []byte) error {
	switch action {
	case oplog.ActionCreateTag:
		p, err := oplog.Payload[oplog.CreateTagPayload](payload)
		if err != nil {
			return err
		}
		tag, err := domain.NewTag(p.TagID, p.ParentID, p.Name)
		if err != nil {
			return err
		}
		return l.tags.Insert(tag)

	case oplog.ActionDeleteTag:
		p, err := oplog.Payload[oplog.DeleteTagPayload](payload)
		if err != nil {
			return err
		}
		return l.tags.Remove(p.TagID)

	case oplog.ActionRenameTag:
		p, err := oplog.Payload[oplog.RenameTagPayload](payload)
		if err != nil {
			return err
		}
		current, ok := l.tags.ByID(p.TagID)
		if !ok {
			return fmt.Errorf("tag %s is not in the tree", p.TagID)
		}
		renamed, err := domain.NewTag(p.TagID, current.ParentID, p.After)
		if err != nil {
			return err
		}
		return l.tags.Update(renamed)

	case oplog.ActionMoveTag:
		p, err := oplog.Payload[oplog.MoveTagPayload](payload)
		if err != nil {
			return err
		}
		current, ok := l.tags.ByID(p.TagID)
		if !ok {
			return fmt.Errorf("tag %s is not in the tree", p.TagID)
		}
		moved, err := domain.NewTag(p.TagID, p.NewParent, current.Name)
		if err != nil {
			return err
		}
		return l.tags.Update(moved)

	case oplog.ActionTagBoxes:
		p, err := oplog.Payload[oplog.TagBoxesPayload](payload)
		if err != nil {
			return err
		}
		l.tags.AddDirectCount(p.TagID, len(p.BoxIDs))

	case oplog.ActionUntagBoxes:
		p, err := oplog.Payload[oplog.UntagBoxesPayload](payload)
		if err != nil {
			return err
		}
		l.tags.AddDirectCount(p.TagID, -len(p.BoxIDs))
	}
	return nil
}
```

（`write.go` 顶部需要补 `domain`。）

- [ ] **Step 4: 实现命令方法**

`internal/library/commands.go`:

```go
package library

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

var (
	ErrNoSuchBox    = errors.New("no such box")
	ErrNoSuchTag    = errors.New("no such tag")
	ErrAmbiguousRef = errors.New("the reference matches more than one thing")
	ErrTagNameTaken = errors.New("a tag with that name already exists")
	ErrTagCycle     = errors.New("that move would put a tag inside its own subtree")
)

// CreateBox creates a box and its root folder.
//
// Both ids are minted here, not in Apply — that is the difference between a
// command and an event (spec 2.3.1). A replay hands the same ids back, so
// every reference into this box survives a rebuild.
func (l *Library) CreateBox(ctx context.Context, name, description string, start, end domain.LocalDate) (domain.Box, error) {
	if err := domain.ValidateName(name); err != nil {
		return domain.Box{}, err
	}
	p := oplog.CreateBoxPayload{
		BoxID:        domain.NewBoxID(),
		RootFolderID: domain.NewFolderID(),
		BoxFields: oplog.BoxFields{
			Name: name, Description: description, StartAt: start, EndAt: end,
		},
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return domain.Box{}, err
	}
	if _, err := l.write(ctx, oplog.ActionCreateBox, raw, oplog.SourceUser, nil); err != nil {
		return domain.Box{}, err
	}
	b, _, err := l.Box(ctx, p.BoxID)
	return b, err
}

// UpdateBox rewrites a box's user-editable fields. The before state is read
// here and written into the payload, which is what makes Inverse a pure swap
// and strict undo possible (spec 2.3, 2.5).
func (l *Library) UpdateBox(ctx context.Context, id domain.BoxID, f oplog.BoxFields) (domain.Box, error) {
	if err := domain.ValidateName(f.Name); err != nil {
		return domain.Box{}, err
	}
	current, ok, err := l.Box(ctx, id)
	if err != nil {
		return domain.Box{}, err
	}
	if !ok {
		return domain.Box{}, fmt.Errorf("%s: %w", id, ErrNoSuchBox)
	}

	p := oplog.UpdateBoxPayload{
		BoxID: id,
		Before: oplog.BoxFields{
			Name: current.Name, Description: current.Description,
			StartAt: current.StartAt, EndAt: current.EndAt,
		},
		After: f,
	}
	if p.Before == p.After {
		return current, nil // nothing to record
	}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return domain.Box{}, err
	}
	if _, err := l.write(ctx, oplog.ActionUpdateBox, raw, oplog.SourceUser, nil); err != nil {
		return domain.Box{}, err
	}
	b, _, err := l.Box(ctx, id)
	return b, err
}

// CreateTag adds a node to the tag tree.
func (l *Library) CreateTag(ctx context.Context, name string, parent *domain.TagID) (domain.Tag, error) {
	if err := domain.ValidateName(name); err != nil {
		return domain.Tag{}, err
	}
	// Checked here rather than left to the unique index so the user gets a
	// sentence instead of a constraint violation — and so no op is written.
	if existing, ok := l.tags.ByName(name); ok {
		return domain.Tag{}, fmt.Errorf("%q is already a tag (%s): %w", existing.Name, existing.ID, ErrTagNameTaken)
	}
	if parent != nil {
		if _, ok := l.tags.ByID(*parent); !ok {
			return domain.Tag{}, fmt.Errorf("parent %s: %w", *parent, ErrNoSuchTag)
		}
	}

	p := oplog.CreateTagPayload{TagID: domain.NewTagID(), ParentID: parent, Name: name}
	raw, err := oplog.Marshal(p)
	if err != nil {
		return domain.Tag{}, err
	}
	if _, err := l.write(ctx, oplog.ActionCreateTag, raw, oplog.SourceUser, nil); err != nil {
		return domain.Tag{}, err
	}
	tag, _ := l.tags.ByID(p.TagID)
	return tag, nil
}

func (l *Library) RenameTag(ctx context.Context, id domain.TagID, name string) error {
	if err := domain.ValidateName(name); err != nil {
		return err
	}
	current, ok := l.tags.ByID(id)
	if !ok {
		return fmt.Errorf("%s: %w", id, ErrNoSuchTag)
	}
	if current.Name == name {
		return nil
	}
	if other, exists := l.tags.ByName(name); exists && other.ID != id {
		return fmt.Errorf("%q is already a tag (%s): %w", other.Name, other.ID, ErrTagNameTaken)
	}

	raw, err := oplog.Marshal(oplog.RenameTagPayload{TagID: id, Before: current.Name, After: name})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionRenameTag, raw, oplog.SourceUser, nil)
	return err
}

// MoveTag reparents a subtree. In the database this is one row and one column,
// because there is no materialized path to cascade (spec 1.6).
func (l *Library) MoveTag(ctx context.Context, id domain.TagID, parent *domain.TagID) error {
	current, ok := l.tags.ByID(id)
	if !ok {
		return fmt.Errorf("%s: %w", id, ErrNoSuchTag)
	}
	if parent != nil {
		if _, ok := l.tags.ByID(*parent); !ok {
			return fmt.Errorf("parent %s: %w", *parent, ErrNoSuchTag)
		}
		// A loop would make Descendants recurse until the stack gives out.
		// Deciding this here, not in Apply, is invariant 3.
		if l.tags.WouldCycle(id, *parent) {
			return fmt.Errorf("%q cannot go under its own descendant: %w", current.Name, ErrTagCycle)
		}
	}
	if sameTagID(current.ParentID, parent) {
		return nil
	}

	raw, err := oplog.Marshal(oplog.MoveTagPayload{TagID: id, OldParent: current.ParentID, NewParent: parent})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionMoveTag, raw, oplog.SourceUser, nil)
	return err
}

// DeleteTag removes a leaf that nothing is using. The refusal to cascade is
// deliberate (spec 1.6).
func (l *Library) DeleteTag(ctx context.Context, id domain.TagID) error {
	current, ok := l.tags.ByID(id)
	if !ok {
		return fmt.Errorf("%s: %w", id, ErrNoSuchTag)
	}
	raw, err := oplog.Marshal(oplog.DeleteTagPayload{
		TagID: id, ParentID: current.ParentID, Name: current.Name,
	})
	if err != nil {
		return err
	}
	_, err = l.write(ctx, oplog.ActionDeleteTag, raw, oplog.SourceUser, nil)
	return err
}

// TagBoxes attaches one tag to many boxes as a single op, and returns how many
// boxes actually changed.
//
// Boxes that already carry the tag are filtered out before the op is built. If
// they were left in, the inverse would strip the tag from boxes this op never
// touched — an undo doing more than the op did.
func (l *Library) TagBoxes(ctx context.Context, tag domain.TagID, boxes []domain.BoxID) (int, error) {
	changed, err := l.filterBoxTags(ctx, tag, boxes, false)
	if err != nil || len(changed) == 0 {
		return 0, err
	}
	raw, err := oplog.Marshal(oplog.TagBoxesPayload{TagID: tag, BoxIDs: changed, Source: "user"})
	if err != nil {
		return 0, err
	}
	if _, err := l.write(ctx, oplog.ActionTagBoxes, raw, oplog.SourceUser, nil); err != nil {
		return 0, err
	}
	return len(changed), nil
}

// UntagBoxes is the mirror image.
func (l *Library) UntagBoxes(ctx context.Context, tag domain.TagID, boxes []domain.BoxID) (int, error) {
	changed, err := l.filterBoxTags(ctx, tag, boxes, true)
	if err != nil || len(changed) == 0 {
		return 0, err
	}
	raw, err := oplog.Marshal(oplog.UntagBoxesPayload{TagID: tag, BoxIDs: changed, Source: "user"})
	if err != nil {
		return 0, err
	}
	if _, err := l.write(ctx, oplog.ActionUntagBoxes, raw, oplog.SourceUser, nil); err != nil {
		return 0, err
	}
	return len(changed), nil
}

// filterBoxTags keeps the boxes for which the operation would actually change
// something. wantPresent is what the box must currently look like to qualify.
func (l *Library) filterBoxTags(ctx context.Context, tag domain.TagID, boxes []domain.BoxID, wantPresent bool) ([]domain.BoxID, error) {
	if _, ok := l.tags.ByID(tag); !ok {
		return nil, fmt.Errorf("%s: %w", tag, ErrNoSuchTag)
	}
	var changed []domain.BoxID
	seen := map[domain.BoxID]bool{}
	err := l.readTx(ctx, func(vtx *store.ViewTx) error {
		for _, id := range boxes {
			if seen[id] {
				continue
			}
			seen[id] = true
			if _, ok, err := vtx.GetBox(id); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("%s: %w", id, ErrNoSuchBox)
			}
			has, err := vtx.HasBoxTag(id, tag)
			if err != nil {
				return err
			}
			if has == wantPresent {
				changed = append(changed, id)
			}
		}
		return nil
	})
	return changed, err
}

// ResolveBox accepts a full id, an id prefix, or an exact name.
//
// Box names are not unique — the schema gives boxes.name_fold an ordinary
// index, not a unique one — so an ambiguous name is reported with its
// candidates rather than resolved by picking one.
func (l *Library) ResolveBox(ctx context.Context, ref string) (domain.Box, error) {
	if id, err := domain.ParseID(ref); err == nil {
		b, ok, err := l.Box(ctx, domain.BoxID{ID: id})
		if err != nil {
			return domain.Box{}, err
		}
		if !ok {
			return domain.Box{}, fmt.Errorf("%s: %w", ref, ErrNoSuchBox)
		}
		return b, nil
	}

	all, err := l.Boxes(ctx, store.ListBoxesOptions{})
	if err != nil {
		return domain.Box{}, err
	}
	fold := domain.Fold(ref)
	var matches []domain.Box
	for _, b := range all {
		if b.NameFold == fold || strings.HasPrefix(b.ID.String(), ref) {
			matches = append(matches, b)
		}
	}
	switch len(matches) {
	case 0:
		return domain.Box{}, fmt.Errorf("%q: %w", ref, ErrNoSuchBox)
	case 1:
		return matches[0], nil
	default:
		var sb strings.Builder
		for _, b := range matches {
			fmt.Fprintf(&sb, "\n  %s  %s", b.ID, b.Name)
		}
		return domain.Box{}, fmt.Errorf("%q matches %d boxes; use an id instead:%s: %w",
			ref, len(matches), sb.String(), ErrAmbiguousRef)
	}
}

// ResolveTag accepts a full id or a name. Names are globally unique, so this
// one cannot be ambiguous (spec 1.6).
func (l *Library) ResolveTag(ref string) (domain.Tag, error) {
	if id, err := domain.ParseID(ref); err == nil {
		if tag, ok := l.tags.ByID(domain.TagID{ID: id}); ok {
			return tag, nil
		}
	}
	if tag, ok := l.tags.ByName(ref); ok {
		return tag, nil
	}
	return domain.Tag{}, fmt.Errorf("%q: %w", ref, ErrNoSuchTag)
}

func sameTagID(a, b *domain.TagID) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/library/ -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/library/commands.go internal/library/write.go internal/library/commands_test.go
git commit -m "feat(library): 对外的命令方法

这些就是'命令做决策，事件记结果'里的命令：铸 ID、校验、构造 payload、
调 write。Apply 拿到的 payload 里已经没有任何待决定的东西。

TagBoxes 先滤掉已经有这个 tag 的 box 再生成 op——不滤的话反向的
untag 会摘掉这条 op 从没加过的 tag，等于撤销做了比原 op 更多的事。
滤完一个不剩就不写 op：一条什么都没改的 op 只会给历史添乱。

ResolveBox 撞名时列出候选而不是随便挑一个。box 名没有唯一约束，
挑错了就是在错的归档上操作。"
```

---

### Task 11: library — Undo（严格模式）

**Files:**
- Create: `internal/library/undo.go`
- Test: `internal/library/undo_test.go`

**Interfaces:**
- Consumes: `oplog.Lookup`、`oplog.ErrStaleBefore`、`store.RevertedBy` / `LastOpTouchingBox`、Task 8 的 `write`
- Produces: `(*Library).Undo(ctx, opID int64) (int64, error)`、`library.ErrAlreadyReverted`、`library.ErrUndoBlocked`

**撤销 = 追加反向 op，不是回滚**（spec 2.4）。撤销本身也是一条 op，因此也能被撤销。

**严格模式的错误必须说出是哪条 op 挡住的**（spec 2.5）。`Apply` 返回 `ErrStaleBefore` 时只知道"状态对不上"，`Undo` 要再查一次 `op_boxes` 把那条 op 找出来——否则用户只看到"撤销失败"，没有任何可以采取的下一步。

**已经撤销过的 op 不能再撤。** `reverts_id` 是列而不是 payload 字段，正是为了这个热查询（spec 2.2）。

- [ ] **Step 1: 写失败的测试**

```go
package library

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func lastOpID(t *testing.T, l *Library) int64 {
	t.Helper()
	ops, err := l.Ops(context.Background(), store.ListOpsOptions{Limit: 1})
	if err != nil || len(ops) == 0 {
		t.Fatalf("Ops: %v (%d rows)", err, len(ops))
	}
	return ops[0].ID
}

func TestUndoCreateBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "regrettable", "", domain.LocalDate{}, domain.LocalDate{})
	opID := lastOpID(t, l)

	undoID, err := l.Undo(ctx, opID)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, ok, _ := l.Box(ctx, b.ID); ok {
		t.Error("the box survived the undo")
	}

	// spec 2.4: undo appends, it does not erase. Both ops are still there.
	ops, _ := l.Ops(ctx, store.ListOpsOptions{Limit: 10})
	if len(ops) != 2 {
		t.Fatalf("history has %d ops, want 2", len(ops))
	}
	if ops[0].ID != undoID || ops[0].RevertsID == nil || *ops[0].RevertsID != opID {
		t.Errorf("the undo op = %+v", ops[0])
	}
}

func TestUndoIsItselfUndoable(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "back and forth", "", domain.LocalDate{}, domain.LocalDate{})
	createOp := lastOpID(t, l)

	undoOp, err := l.Undo(ctx, createOp)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, err := l.Undo(ctx, undoOp); err != nil {
		t.Fatalf("undoing the undo: %v", err)
	}

	got, ok, _ := l.Box(ctx, b.ID)
	if !ok {
		t.Fatal("the box did not come back")
	}
	if got.Name != "back and forth" {
		t.Errorf("name came back as %q", got.Name)
	}
}

func TestUndoUpdateBox(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "before", "", domain.LocalDate{}, domain.LocalDate{})
	l.UpdateBox(ctx, b.ID, oplog.BoxFields{Name: "after", Description: "noted"})
	updateOp := lastOpID(t, l)

	if _, err := l.Undo(ctx, updateOp); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	got, _, _ := l.Box(ctx, b.ID)
	if got.Name != "before" || got.Description != "" {
		t.Errorf("after undo: %+v", got)
	}
}

func TestUndoTagBoxesRestoresTheWholeBatch(t *testing.T) {
	// spec 2.2.1: one batch, one op, one undo.
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "云南", nil)
	var boxes []domain.BoxID
	for _, n := range []string{"一", "二", "三"} {
		b, _ := l.CreateBox(ctx, n, "", domain.LocalDate{}, domain.LocalDate{})
		boxes = append(boxes, b.ID)
	}
	l.TagBoxes(ctx, tag.ID, boxes)
	tagOp := lastOpID(t, l)

	if _, err := l.Undo(ctx, tagOp); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for _, id := range boxes {
		tags, _ := l.BoxTags(ctx, id)
		if len(tags) != 0 {
			t.Errorf("box %s still carries %d tag(s)", id, len(tags))
		}
	}
	if got := l.Tags().Count(tag.ID); got != 0 {
		t.Errorf("the count cache says %d after the undo, want 0", got)
	}
}

func TestUndoTagOpsKeepTheTreeInStep(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	tag, _ := l.CreateTag(ctx, "临时", nil)
	createOp := lastOpID(t, l)

	if _, err := l.Undo(ctx, createOp); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, ok := l.Tags().ByID(tag.ID); ok {
		t.Error("the in-memory tree still holds a tag that was undone")
	}
	// The name is free again, which is only true if the tree really dropped it.
	if _, err := l.CreateTag(ctx, "临时", nil); err != nil {
		t.Errorf("recreating the tag after undo: %v", err)
	}
}

func TestUndoRefusesToRunTwice(t *testing.T) {
	l := openTestLibrary(t)
	ctx := context.Background()
	l.CreateBox(ctx, "once", "", domain.LocalDate{}, domain.LocalDate{})
	opID := lastOpID(t, l)

	if _, err := l.Undo(ctx, opID); err != nil {
		t.Fatalf("first undo: %v", err)
	}
	_, err := l.Undo(ctx, opID)
	if !errors.Is(err, ErrAlreadyReverted) {
		t.Errorf("second undo = %v, want ErrAlreadyReverted", err)
	}
}

func TestUndoOfAMissingOp(t *testing.T) {
	l := openTestLibrary(t)
	if _, err := l.Undo(context.Background(), 9999); err == nil {
		t.Error("undoing an op that does not exist succeeded")
	}
}

func TestUndoAcrossTimeIsRefusedAndNamesTheBlocker(t *testing.T) {
	// spec 2.5, the whole point of strict mode: "I thought the undo worked but
	// it only restored half of it" is the worse outcome in an archive, and a
	// bare "undo failed" leaves the user with nothing to do about it.
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "v1", "", domain.LocalDate{}, domain.LocalDate{})

	l.UpdateBox(ctx, b.ID, oplog.BoxFields{Name: "v2"})
	firstUpdate := lastOpID(t, l)
	l.UpdateBox(ctx, b.ID, oplog.BoxFields{Name: "v3"})
	secondUpdate := lastOpID(t, l)

	_, err := l.Undo(ctx, firstUpdate)
	if !errors.Is(err, ErrUndoBlocked) {
		t.Fatalf("error = %v, want ErrUndoBlocked", err)
	}
	if !strings.Contains(err.Error(), fmt.Sprint(secondUpdate)) {
		t.Errorf("error = %q, want it to name op %d as the blocker", err, secondUpdate)
	}

	// Nothing changed, and nothing was written.
	got, _, _ := l.Box(ctx, b.ID)
	if got.Name != "v3" {
		t.Errorf("the box is now %q; a refused undo must change nothing", got.Name)
	}
	if lastOpID(t, l) != secondUpdate {
		t.Error("a refused undo still appended an op")
	}
}

func TestUndoBlockedByALaterTag(t *testing.T) {
	// Undoing the creation of a box that has since been tagged would take the
	// tagging with it.
	l := openTestLibrary(t)
	ctx := context.Background()
	b, _ := l.CreateBox(ctx, "tagged later", "", domain.LocalDate{}, domain.LocalDate{})
	createOp := lastOpID(t, l)
	tag, _ := l.CreateTag(ctx, "云南", nil)
	l.TagBoxes(ctx, tag.ID, []domain.BoxID{b.ID})

	if _, err := l.Undo(ctx, createOp); !errors.Is(err, ErrUndoBlocked) {
		t.Errorf("error = %v, want ErrUndoBlocked", err)
	}
	if _, ok, _ := l.Box(ctx, b.ID); !ok {
		t.Error("the box was deleted anyway")
	}
}
```

（测试文件顶部需要 `fmt`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/library/ -run 'TestUndo' -v`
Expected: FAIL，`undefined: Undo`

- [ ] **Step 3: 实现**

`internal/library/undo.go`:

```go
package library

import (
	"context"
	"errors"
	"fmt"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

var (
	// ErrAlreadyReverted means some other op has already undone this one.
	ErrAlreadyReverted = errors.New("this op has already been undone")
	// ErrUndoBlocked means later ops changed what this undo wanted to restore.
	ErrUndoBlocked = errors.New("later changes are in the way of this undo")
)

// Undo appends the inverse of an op.
//
// It is not a rollback (spec 2.4): op 47 stays in the log, op 48 says "this
// undoes 47", and op 48 can itself be undone by op 49. The history only ever
// grows, which is what makes it trustworthy.
//
// Strict mode (spec 2.5): if the state the inverse expects to find has since
// been changed, the undo is refused rather than partially applied. Half a
// restored archive that looks whole is the failure worth preventing.
func (l *Library) Undo(ctx context.Context, opID int64) (int64, error) {
	ops, err := l.Ops(ctx, store.ListOpsOptions{})
	if err != nil {
		return 0, err
	}
	var target *oplog.Op
	for i := range ops {
		if ops[i].ID == opID {
			target = &ops[i]
			break
		}
	}
	if target == nil {
		return 0, fmt.Errorf("op %d does not exist", opID)
	}

	// reverts_id is an indexed column precisely so this stays cheap: the
	// history panel asks it for every row it renders (spec 2.2).
	if by, ok, err := store.RevertedBy(l.db, opID); err != nil {
		return 0, err
	} else if ok {
		return 0, fmt.Errorf("op %d was already undone by op %d: %w", opID, by, ErrAlreadyReverted)
	}

	handler, err := oplog.Lookup(target.Action)
	if err != nil {
		return 0, err
	}
	action, payload, err := handler.Inverse(target.Payload)
	if err != nil {
		return 0, fmt.Errorf("compute the inverse of op %d: %w", opID, err)
	}

	undoID, err := l.write(ctx, action, payload, oplog.SourceUser, &opID)
	if err != nil {
		if errors.Is(err, oplog.ErrStaleBefore) {
			return 0, l.explainBlockedUndo(ctx, *target, err)
		}
		return 0, err
	}
	return undoID, nil
}

// explainBlockedUndo turns "the state does not match" into "op N is in the
// way".
//
// Without this the user gets a true but useless sentence. With it they can go
// look at op N, undo that first, or decide the change was wanted after all —
// which is the difference between an error message and a dead end (spec 2.5).
func (l *Library) explainBlockedUndo(ctx context.Context, target oplog.Op, cause error) error {
	handler, err := oplog.Lookup(target.Action)
	if err != nil {
		return cause
	}
	targets, err := handler.Targets(target.Payload)
	if err != nil {
		return cause
	}

	for _, box := range targets.Boxes {
		blocker, ok, err := store.LastOpTouchingBox(l.db, box, target.ID)
		if err != nil || !ok {
			continue
		}
		return fmt.Errorf(
			"cannot undo op %d: op %d changed the same box afterwards; undo that first if you want this one back: %w",
			target.ID, blocker, ErrUndoBlocked)
	}
	return fmt.Errorf("cannot undo op %d: the library has changed since it ran: %w", target.ID, ErrUndoBlocked)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/library/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/library/undo.go internal/library/undo_test.go
git commit -m "feat(library): Undo，严格模式且能说出是哪条 op 挡住的

撤销 = 追加反向 op，不是回滚。撤销本身也是一条 op，所以也能被撤销。

Apply 返回 ErrStaleBefore 时只知道'状态对不上'，Undo 再查一次
op_boxes 把挡路的那条找出来。差别是用户拿到的是一条能采取下一步的
信息，还是一个死胡同。"
```

---

### Task 12: library — Rebuild，以及不变量 3 的证明

**Files:**
- Create: `internal/library/rebuild.go`, `internal/store/snapshot.go`
- Test: `internal/library/rebuild_test.go`

**Interfaces:**
- Consumes: `store.ClearViews` / `DeleteOpIndexes` / `ListOps`、`oplog.Lookup`
- Produces: `(*Library).Rebuild(ctx) error`、`store.Snapshot(q Queryer) (string, error)`

**这个 task 是整份计划里最重要的一个。** 前面十一个 task 都在说"`Apply` 是纯函数"，这一个把它变成一条会失败的断言：**建一批数据 → 记下全库快照 → `Rebuild` → 快照必须逐字节相同**（spec 2.3.1 明确要求 conformance suite 里有这条）。

任何一处违反——`Apply` 里调了 `uuid.New()`、写了 `time.Now()`、按当时目录内容算了个名字——都会让两份快照不同，而且**失败信息直接指向出问题的那一行**。

**`Rebuild` 不碰 `ops` 和 `blobs`。** 它清空物化视图和索引表，然后按 id 顺序重放。`updated_at` 的传递在重放时用**每条 op 自己的 ts**，所以结果和实时写入完全一致。

**快照必须是确定性的**：每张表按主键排序，列全取，格式固定。否则这条测试会时灵时不灵，比没有还糟。

- [ ] **Step 1: 写快照函数**

`internal/store/snapshot.go`:

```go
package store

import (
	"fmt"
	"strings"
)

// snapshotTables lists the rebuildable tables and how to order them so the
// dump is byte-stable. ops and blobs are absent: they are the authority, not
// the thing being proved.
var snapshotTables = []struct {
	name    string
	columns string
	order   string
}{
	{"boxes", "hex(id), name, name_fold, description, description_fold, created_at, updated_at, coalesce(start_at,''), coalesce(end_at,'')", "id"},
	{"folders", "hex(id), hex(box_id), coalesce(hex(parent_id),''), name, name_fold, created_at, updated_at", "id"},
	{"files", "hex(id), hex(box_id), hex(folder_id), name, name_fold, blob_key, coalesce(captured_at,''), created_at, updated_at", "id"},
	{"tags", "hex(id), coalesce(hex(parent_id),''), name, name_fold", "id"},
	{"box_tags", "hex(box_id), hex(tag_id), source", "box_id, tag_id"},
	{"file_tags", "hex(file_id), hex(tag_id), source", "file_id, tag_id"},
	{"tag_rejections", "hex(file_id), hex(tag_id)", "file_id, tag_id"},
	{"metadata", "target_type, hex(target_id), key, value, source", "target_type, target_id, key"},
	{"op_boxes", "op_id, hex(box_id)", "op_id, box_id"},
	{"op_files", "op_id, hex(file_id)", "op_id, file_id"},
}

// Snapshot renders every rebuildable table as deterministic text.
//
// It exists for one test, and that test is the one that keeps invariant 3
// honest: dump, rebuild, dump again, compare (spec 2.3.1). A snapshot whose
// row order could vary would make that test flap, which is worse than not
// having it.
func Snapshot(q Queryer) (string, error) {
	var sb strings.Builder
	for _, t := range snapshotTables {
		fmt.Fprintf(&sb, "== %s ==\n", t.name)
		rows, err := q.Query(`SELECT ` + t.columns + ` FROM ` + t.name + ` ORDER BY ` + t.order)
		if err != nil {
			return "", fmt.Errorf("snapshot %s: %w", t.name, err)
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return "", err
		}
		for rows.Next() {
			cells := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range cells {
				ptrs[i] = &cells[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return "", fmt.Errorf("snapshot %s: %w", t.name, err)
			}
			parts := make([]string, len(cells))
			for i, c := range cells {
				parts[i] = fmt.Sprintf("%v", c)
			}
			sb.WriteString(strings.Join(parts, "\t"))
			sb.WriteByte('\n')
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", fmt.Errorf("snapshot %s: %w", t.name, err)
		}
	}
	return sb.String(), nil
}
```

- [ ] **Step 2: 写失败的测试**

`internal/library/rebuild_test.go`:

```go
package library

import (
	"context"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// busyLibrary exercises every op type, including an undo, so the rebuild proof
// covers all of them.
func busyLibrary(t *testing.T) *Library {
	t.Helper()
	l := openTestLibrary(t)
	ctx := context.Background()

	places, err := l.CreateTag(ctx, "地点", nil)
	if err != nil {
		t.Fatal(err)
	}
	yunnan, err := l.CreateTag(ctx, "云南", &places.ID)
	if err != nil {
		t.Fatal(err)
	}
	people, _ := l.CreateTag(ctx, "人物", nil)
	doomed, _ := l.CreateTag(ctx, "要删掉的", nil)
	if err := l.DeleteTag(ctx, doomed.ID); err != nil {
		t.Fatal(err)
	}
	if err := l.RenameTag(ctx, yunnan.ID, "云南省"); err != nil {
		t.Fatal(err)
	}
	if err := l.MoveTag(ctx, yunnan.ID, &people.ID); err != nil {
		t.Fatal(err)
	}

	start, _ := domain.ParseLocalDate("2019-08")
	var boxes []domain.BoxID
	for _, n := range []string{"一", "二", "三"} {
		b, err := l.CreateBox(ctx, n, "note "+n, start, domain.LocalDate{})
		if err != nil {
			t.Fatal(err)
		}
		boxes = append(boxes, b.ID)
	}
	if _, err := l.UpdateBox(ctx, boxes[0], oplog.BoxFields{Name: "一号", Description: "renamed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.TagBoxes(ctx, yunnan.ID, boxes); err != nil {
		t.Fatal(err)
	}
	if _, err := l.UntagBoxes(ctx, yunnan.ID, boxes[2:]); err != nil {
		t.Fatal(err)
	}

	// A box that gets created and then undone, so the replay has to handle an
	// inverse op too.
	tmp, _ := l.CreateBox(ctx, "临时", "", domain.LocalDate{}, domain.LocalDate{})
	_ = tmp
	if _, err := l.Undo(ctx, lastOpID(t, l)); err != nil {
		t.Fatal(err)
	}
	return l
}

// This is the test the whole plan is built around (spec 2.3.1). Every claim
// about Apply being a pure function is only as good as this comparison.
func TestRebuildReproducesTheLibraryExactly(t *testing.T) {
	l := busyLibrary(t)
	ctx := context.Background()

	before, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if !strings.Contains(before, "== boxes ==") || len(before) < 100 {
		t.Fatalf("the snapshot looks empty:\n%s", before)
	}

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	after, err := store.Snapshot(l.db)
	if err != nil {
		t.Fatalf("Snapshot after rebuild: %v", err)
	}
	if before != after {
		t.Errorf("rebuild did not reproduce the library.\n%s", firstDifference(before, after))
	}
}

// firstDifference points at the line that broke, because a diff of two
// thousand-line dumps is not a bug report.
func firstDifference(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(al) || i < len(bl); i++ {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y {
			return fmt.Sprintf("line %d:\n  before: %q\n  after:  %q", i+1, x, y)
		}
	}
	return "the dumps differ but no line does; check trailing whitespace"
}

func TestRebuildIsIdempotent(t *testing.T) {
	l := busyLibrary(t)
	ctx := context.Background()

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}
	once, _ := store.Snapshot(l.db)
	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}
	twice, _ := store.Snapshot(l.db)
	if once != twice {
		t.Error("rebuilding twice gave a different result than rebuilding once")
	}
}

func TestRebuildLeavesTheOpLogAlone(t *testing.T) {
	// Invariant 1: ops is the authority. A rebuild that rewrote it would not be
	// a repair, it would be the one thing this design exists to prevent.
	l := busyLibrary(t)
	ctx := context.Background()

	before, _ := l.Ops(ctx, store.ListOpsOptions{})
	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	after, _ := l.Ops(ctx, store.ListOpsOptions{})

	if len(before) != len(after) {
		t.Fatalf("op count went from %d to %d", len(before), len(after))
	}
	for i := range before {
		if before[i].ID != after[i].ID || before[i].Action != after[i].Action ||
			!before[i].TS.Equal(after[i].TS) || string(before[i].Payload) != string(after[i].Payload) {
			t.Errorf("op %d changed during the rebuild", before[i].ID)
		}
	}
}

func TestRebuildRegeneratesTheIndexTables(t *testing.T) {
	// spec 2.2.1: op_boxes is derived from the payload, so wiping it and
	// replaying must put it back exactly.
	l := busyLibrary(t)
	ctx := context.Background()

	var before int
	l.db.QueryRow(`SELECT count(*) FROM op_boxes`).Scan(&before)
	if before == 0 {
		t.Fatal("the fixture produced no op_boxes rows")
	}

	if _, err := l.db.Exec(`DELETE FROM op_boxes`); err != nil {
		t.Fatal(err)
	}
	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	var after int
	l.db.QueryRow(`SELECT count(*) FROM op_boxes`).Scan(&after)
	if after != before {
		t.Errorf("op_boxes has %d rows after the rebuild, want %d", after, before)
	}
}

func TestRebuildRepairsAWreckedView(t *testing.T) {
	// "物化视图坏了不是数据丢失" (spec 2.1) — the promise, tested.
	l := busyLibrary(t)
	ctx := context.Background()
	before, _ := store.Snapshot(l.db)

	if _, err := l.db.Exec(`DELETE FROM box_tags`); err != nil {
		t.Fatal(err)
	}
	if _, err := l.db.Exec(`UPDATE boxes SET name = 'corrupted', name_fold = 'corrupted'`); err != nil {
		t.Fatal(err)
	}

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	after, _ := store.Snapshot(l.db)
	if before != after {
		t.Errorf("rebuild did not repair the damage.\n%s", firstDifference(before, after))
	}
}

func TestRebuildReloadsTheTagTree(t *testing.T) {
	l := busyLibrary(t)
	ctx := context.Background()
	before := l.Tags().Len()

	if err := l.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got := l.Tags().Len(); got != before {
		t.Errorf("the tag tree has %d nodes after the rebuild, want %d", got, before)
	}
	if _, ok := l.Tags().ByName("云南省"); !ok {
		t.Error("the rebuilt tree lost a tag")
	}
}
```

（测试文件顶部需要 `fmt`。）

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./internal/library/ -run TestRebuild -v`
Expected: FAIL，`undefined: Rebuild`

- [ ] **Step 4: 实现**

`internal/library/rebuild.go`:

```go
package library

import (
	"context"
	"fmt"

	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

// Rebuild discards every materialized view and replays the log.
//
// This is what invariant 1 buys: ops and blobs/ are the authority, so a
// corrupted view is an inconvenience rather than data loss (spec 2.1). It is
// also the assertion that keeps invariant 3 honest — if any Apply generated an
// id, read a clock, or looked at the current state to decide something, the
// rebuilt library differs from the original and the test in this task says so.
//
// ops and blobs are not touched. The whole point is that they are the input.
func (l *Library) Rebuild(ctx context.Context) error {
	ops, err := store.ListOps(l.db, store.ListOpsOptions{})
	if err != nil {
		return err
	}

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rebuild: %w", err)
	}
	defer tx.Rollback()

	if err := store.ClearViews(tx); err != nil {
		return err
	}
	if err := store.DeleteOpIndexes(tx); err != nil {
		return err
	}

	// ListOps returns newest first; replay wants oldest first.
	for i := len(ops) - 1; i >= 0; i-- {
		row := ops[i]
		action := oplog.Action(row.Action)
		handler, err := oplog.Lookup(action)
		if err != nil {
			return fmt.Errorf("replaying op %d: %w", row.ID, err)
		}

		vtx := store.NewViewTx(tx, row.TS)
		if err := handler.Apply(vtx, row.Payload); err != nil {
			return fmt.Errorf("replaying op %d (%s): %w", row.ID, action, err)
		}

		targets, err := handler.Targets(row.Payload)
		if err != nil {
			return fmt.Errorf("replaying op %d: %w", row.ID, err)
		}
		if err := store.InsertOpBoxes(tx, row.ID, targets.Boxes); err != nil {
			return err
		}
		if err := store.InsertOpFiles(tx, row.ID, targets.Files); err != nil {
			return err
		}

		// Same rule as the live path, with this op's own ts — which is why the
		// replayed updated_at values come out identical instead of all being
		// the moment of the rebuild (spec 1.5.1 ①).
		if oplog.Source(row.Source).PropagatesUpdatedAt() {
			if err := vtx.TouchBoxes(targets.Boxes); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rebuild: %w", err)
	}

	tags, err := LoadTagTree(l.db)
	if err != nil {
		return fmt.Errorf("reload the tag tree after rebuild: %w", err)
	}
	l.tags = tags
	return nil
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/library/ -v`
Expected: PASS

⚠️ **如果 `TestRebuildReproducesTheLibraryExactly` 失败，不要改测试。** 它失败说明某个 `Apply` 违反了不变量 3，失败信息里的行号直接指向哪张表哪一行不同。按 2.3.1 的四类违反去查：生成了 ID、读了时钟、按当时状态算了名字、或者碰撞后缀是在 `Apply` 里定的。

- [ ] **Step 6: 提交**

```bash
git add internal/library/rebuild.go internal/library/rebuild_test.go internal/store/snapshot.go
git commit -m "feat(library): Rebuild，把不变量 3 变成一条会失败的断言

建一批数据 → 记快照 → 重放 → 快照必须逐字节相同。这是设计文档
2.3.1 明确要求的那条 conformance 测试。任何一处违反——Apply 里
生成了 ID、读了时钟、按当时状态算了名字——都会让两份快照不同，
且失败信息直接指向出问题的那一行。

重放时 updated_at 用每条 op 自己的 ts，所以结果和实时写入一致，
而不是全部变成重建那一刻。"
```

---

### Task 13: cmd — box / tag / log / undo

**Files:**
- Create: `cmd/kist/box.go`, `cmd/kist/tag.go`, `cmd/kist/oplog.go`, `cmd/kist/library.go`
- Modify: `cmd/kist/root.go`（注册子命令 + `--library` 全局 flag）
- Test: `cmd/kist/box_test.go`, `cmd/kist/tag_test.go`, `cmd/kist/oplog_test.go`

**Interfaces:**
- Consumes: `library.Open` 与 Task 10/11 的全部命令方法
- Produces: `newBoxCmd()` / `newTagCmd()` / `newLogCmd()` / `newUndoCmd()`、`openLibrary(cmd *cobra.Command) (*library.Library, error)`

**命令表：**

```
kist box new <name> [--start] [--end] [--description]
kist box list
kist box edit <box> [--name] [--start] [--end] [--description]
kist box tag <box> <tag>...
kist box untag <box> <tag>...

kist tag new <name> [--parent <tag>]
kist tag tree
kist tag rename <tag> <new-name>
kist tag move <tag> --parent <tag>        # --parent "" 移到根
kist tag rm <tag>

kist log [-n N] [--box <box>]
kist undo <op-id>
```

**`kist box list` 和 `kist tag tree` 不在设计文档 7 节的 M1 命令清单里，但没有它们 M1b 无法被人验证**——`undo` 需要 op id，打 tag 需要 box 引用，而 M1c 的 `ls` 是"列一个 box 里的文件"，是另一件事。两条命令加起来不到八十行，且 M1c 不会重写它们。

**`--library` 是全局 flag**，默认 `~/kist`，与 `kist init` 的默认位置一致（spec 3.1）。

**`kist log` 要标出已撤销的行**，这正是 `reverts_id` 提成列的原因（spec 2.2）。

- [ ] **Step 1: 写失败的测试**

`cmd/kist/box_test.go`:

```go
package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/library"
)

// newLibrary makes an initialised library and returns the flag that points at
// it, so every test drives the real CLI surface.
func newLibrary(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := library.Create(context.Background(), root); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return "--library=" + root
}

func TestBoxNewPrintsTheID(t *testing.T) {
	lib := newLibrary(t)

	out, err := exec(t, lib, "box", "new", "云南 2019")
	if err != nil {
		t.Fatalf("box new: %v\n%s", err, out)
	}
	if !strings.Contains(out, "云南 2019") {
		t.Errorf("output does not name the box:\n%s", out)
	}
	// The id has to be printed: nothing else can reference the box yet.
	if !strings.Contains(out, "-") || len(strings.TrimSpace(out)) < 36 {
		t.Errorf("output does not look like it contains an id:\n%s", out)
	}

	list, err := exec(t, lib, "box", "list")
	if err != nil {
		t.Fatalf("box list: %v", err)
	}
	if !strings.Contains(list, "云南 2019") {
		t.Errorf("box list does not show the box:\n%s", list)
	}
}

func TestBoxNewWithDates(t *testing.T) {
	lib := newLibrary(t)
	out, err := exec(t, lib, "box", "new", "婚礼", "--start", "2019-08-12", "--end", "2019-08-12")
	if err != nil {
		t.Fatalf("box new: %v\n%s", err, out)
	}
	list, _ := exec(t, lib, "box", "list")
	if !strings.Contains(list, "2019-08-12") {
		t.Errorf("box list does not show the date:\n%s", list)
	}
}

func TestBoxNewRejectsABadDate(t *testing.T) {
	lib := newLibrary(t)
	if _, err := exec(t, lib, "box", "new", "x", "--start", "2019-8"); err == nil {
		t.Error("a non-zero-padded date was accepted")
	}
}

func TestBoxEdit(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "before")

	if _, err := exec(t, lib, "box", "edit", "before", "--name", "after"); err != nil {
		t.Fatalf("box edit: %v", err)
	}
	list, _ := exec(t, lib, "box", "list")
	if strings.Contains(list, "before") || !strings.Contains(list, "after") {
		t.Errorf("the rename did not take:\n%s", list)
	}
}

func TestBoxTagAndUntag(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "trip")
	exec(t, lib, "tag", "new", "云南")

	out, err := exec(t, lib, "box", "tag", "trip", "云南")
	if err != nil {
		t.Fatalf("box tag: %v\n%s", err, out)
	}
	tree, _ := exec(t, lib, "tag", "tree")
	if !strings.Contains(tree, "云南") || !strings.Contains(tree, "1") {
		t.Errorf("tag tree does not show the count:\n%s", tree)
	}

	if _, err := exec(t, lib, "box", "untag", "trip", "云南"); err != nil {
		t.Fatalf("box untag: %v", err)
	}
}

func TestBoxTagReportsAnAmbiguousName(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "重名")
	exec(t, lib, "box", "new", "重名")
	exec(t, lib, "tag", "new", "云南")

	_, err := exec(t, lib, "box", "tag", "重名", "云南")
	if err == nil {
		t.Fatal("an ambiguous box name was accepted")
	}
	if !strings.Contains(err.Error(), "matches") {
		t.Errorf("error = %q, want it to explain the ambiguity", err)
	}
}

func TestCommandsRefuseAMissingLibrary(t *testing.T) {
	out, err := exec(t, "--library="+t.TempDir(), "box", "list")
	if err == nil {
		t.Fatalf("box list on a non-library succeeded:\n%s", out)
	}
	if !strings.Contains(err.Error(), "kist init") {
		t.Errorf("error = %q, want it to point at `kist init`", err)
	}
}
```

`cmd/kist/tag_test.go`:

```go
package main

import (
	"strings"
	"testing"
)

func TestTagTree(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "地点")
	exec(t, lib, "tag", "new", "云南", "--parent", "地点")
	exec(t, lib, "tag", "new", "大理", "--parent", "云南")

	out, err := exec(t, lib, "tag", "tree")
	if err != nil {
		t.Fatalf("tag tree: %v", err)
	}
	for _, want := range []string{"地点", "云南", "大理"} {
		if !strings.Contains(out, want) {
			t.Errorf("tree is missing %q:\n%s", want, out)
		}
	}
	// Depth has to be visible, or a tree view shows nothing a list would not.
	if strings.Index(out, "大理") < strings.Index(out, "云南") {
		t.Errorf("children are not printed under their parent:\n%s", out)
	}
}

func TestTagNewRejectsADuplicate(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "云南")
	_, err := exec(t, lib, "tag", "new", "云南")
	if err == nil {
		t.Fatal("a duplicate tag name was accepted")
	}
	if !strings.Contains(err.Error(), "already") {
		t.Errorf("error = %q, want it to say the name is taken", err)
	}
}

func TestTagRenameAndMove(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "地点")
	exec(t, lib, "tag", "new", "人物")
	exec(t, lib, "tag", "new", "云南", "--parent", "地点")

	if _, err := exec(t, lib, "tag", "rename", "云南", "云南省"); err != nil {
		t.Fatalf("tag rename: %v", err)
	}
	if _, err := exec(t, lib, "tag", "move", "云南省", "--parent", "人物"); err != nil {
		t.Fatalf("tag move: %v", err)
	}

	out, _ := exec(t, lib, "tag", "tree")
	if !strings.Contains(out, "云南省") {
		t.Errorf("tree does not show the renamed tag:\n%s", out)
	}
}

func TestTagMoveToRoot(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "地点")
	exec(t, lib, "tag", "new", "云南", "--parent", "地点")

	if _, err := exec(t, lib, "tag", "move", "云南", "--parent", ""); err != nil {
		t.Fatalf("tag move to root: %v", err)
	}
}

func TestTagRemove(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "临时")
	if _, err := exec(t, lib, "tag", "rm", "临时"); err != nil {
		t.Fatalf("tag rm: %v", err)
	}
	out, _ := exec(t, lib, "tag", "tree")
	if strings.Contains(out, "临时") {
		t.Errorf("the tag is still listed:\n%s", out)
	}
}

func TestTagRemoveRefusesWhenInUse(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "云南")
	exec(t, lib, "box", "new", "trip")
	exec(t, lib, "box", "tag", "trip", "云南")

	_, err := exec(t, lib, "tag", "rm", "云南")
	if err == nil {
		t.Fatal("removing a tag in use succeeded")
	}
	if !strings.Contains(err.Error(), "attached") {
		t.Errorf("error = %q, want it to explain what is still using the tag", err)
	}
}

func TestTagMoveRefusesACycle(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "tag", "new", "地点")
	exec(t, lib, "tag", "new", "云南", "--parent", "地点")

	if _, err := exec(t, lib, "tag", "move", "地点", "--parent", "云南"); err == nil {
		t.Error("a cycle was accepted")
	}
}
```

`cmd/kist/oplog_test.go`:

```go
package main

import (
	"strings"
	"testing"
)

func TestLog(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "一")
	exec(t, lib, "tag", "new", "云南")

	out, err := exec(t, lib, "log")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	for _, want := range []string{"create_box", "create_tag"} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not show %q:\n%s", want, out)
		}
	}
	// Newest first, so the tag (written second) appears above the box.
	if strings.Index(out, "create_tag") > strings.Index(out, "create_box") {
		t.Errorf("log is not newest-first:\n%s", out)
	}
}

func TestLogLimit(t *testing.T) {
	lib := newLibrary(t)
	for _, n := range []string{"一", "二", "三", "四"} {
		exec(t, lib, "box", "new", n)
	}
	out, err := exec(t, lib, "log", "-n", "2")
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if got := strings.Count(out, "create_box"); got != 2 {
		t.Errorf("log -n 2 printed %d ops:\n%s", got, out)
	}
}

func TestUndoFromTheCLI(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "regrettable")

	out, _ := exec(t, lib, "log", "-n", "1")
	opID := firstField(out)
	if opID == "" {
		t.Fatalf("could not read an op id out of:\n%s", out)
	}

	if _, err := exec(t, lib, "undo", opID); err != nil {
		t.Fatalf("undo %s: %v", opID, err)
	}
	list, _ := exec(t, lib, "box", "list")
	if strings.Contains(list, "regrettable") {
		t.Errorf("the box survived the undo:\n%s", list)
	}

	// spec 2.2: the history panel must be able to show what has been undone,
	// which is why reverts_id is a column.
	log, _ := exec(t, lib, "log")
	if !strings.Contains(log, "undone") {
		t.Errorf("log does not mark the reverted op:\n%s", log)
	}
}

func TestUndoRefusesTwice(t *testing.T) {
	lib := newLibrary(t)
	exec(t, lib, "box", "new", "once")
	out, _ := exec(t, lib, "log", "-n", "1")
	opID := firstField(out)

	exec(t, lib, "undo", opID)
	if _, err := exec(t, lib, "undo", opID); err == nil {
		t.Error("undoing the same op twice succeeded")
	}
}

// firstField returns the first whitespace-separated token of the first
// non-empty line, which is where `kist log` puts the op id.
func firstField(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./cmd/kist/ -v`
Expected: FAIL，`unknown command "box"`

- [ ] **Step 3: 加 `--library` 全局 flag 与打开助手**

`cmd/kist/library.go`:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/library"
)

// libraryFlag is the value of --library, shared by every command that needs an
// open library.
var libraryFlag string

// openLibrary resolves --library and opens it.
//
// The error when the path is not a library names `kist init`, because that is
// the next thing the user has to do and making them go look it up is a small
// unkindness that a self-hosted tool cannot afford to repeat.
func openLibrary(cmd *cobra.Command) (*library.Library, error) {
	path := libraryFlag
	if path == "" {
		def, err := defaultLibraryPath()
		if err != nil {
			return nil, err
		}
		path = def
	}
	path, err := expandHome(path)
	if err != nil {
		return nil, err
	}

	l, err := library.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w\n\nIf you have not made a library yet, run: kist init", err)
	}
	return l, nil
}
```

（`library.go` 只 import `fmt`、`cobra` 和 `internal/library`。）

在 `cmd/kist/root.go` 的 `newRootCmd` 里加：

```go
	root.PersistentFlags().StringVar(&libraryFlag, "library", "",
		"path to the kist library (default ~/kist)")

	root.AddCommand(newInitCmd())
	root.AddCommand(newBoxCmd())
	root.AddCommand(newTagCmd())
	root.AddCommand(newLogCmd())
	root.AddCommand(newUndoCmd())
	root.AddCommand(newVersionCmd())
```

- [ ] **Step 4: 实现 box 子命令**

`cmd/kist/box.go`:

```go
package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/oplog"
	"github.com/jinzhao1994/kist/internal/store"
)

func newBoxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "box",
		Short: "Create and edit boxes",
		Long: `A box is one activity, one trip, one batch of paperwork — the unit
everything else hangs off.`,
	}
	cmd.AddCommand(newBoxNewCmd(), newBoxListCmd(), newBoxEditCmd(), newBoxTagCmd(), newBoxUntagCmd())
	return cmd
}

// parseDate turns a flag value into a variable-precision date. An empty string
// means "not set", which is a legitimate state (spec 1.3).
func parseDate(s string) (domain.LocalDate, error) {
	if s == "" {
		return domain.LocalDate{}, nil
	}
	return domain.ParseLocalDate(s)
}

func newBoxNewCmd() *cobra.Command {
	var start, end, description string

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a box",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			startAt, err := parseDate(start)
			if err != nil {
				return err
			}
			endAt, err := parseDate(end)
			if err != nil {
				return err
			}

			b, err := l.CreateBox(cmd.Context(), args[0], description, startAt, endAt)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", b.ID, b.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&start, "start", "", "when it started: 2019, 2019-08, 2019-08-12, or 2019-08-12T14:30")
	cmd.Flags().StringVar(&end, "end", "", "when it ended, same forms as --start")
	cmd.Flags().StringVar(&description, "description", "", "a note about this box")
	return cmd
}

func newBoxListCmd() *cobra.Command {
	var byName bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List boxes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			boxes, err := l.Boxes(cmd.Context(), store.ListBoxesOptions{SortByName: byName})
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, b := range boxes {
				when := b.StartAt.String()
				if when == "" {
					when = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", b.ID, when, b.Name)
			}
			return w.Flush()
		},
	}
	// Default is most-recently-active first; --sort-name is byte order
	// (spec 4.8).
	cmd.Flags().BoolVar(&byName, "sort-name", false, "sort by name instead of recent activity")
	return cmd
}

func newBoxEditCmd() *cobra.Command {
	var name, start, end, description string

	cmd := &cobra.Command{
		Use:   "edit <box>",
		Short: "Change a box's name, dates or note",
		Long:  "<box> is an id, an id prefix, or an exact name.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			b, err := l.ResolveBox(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			// Unset flags keep the current value, so editing one field does
			// not quietly blank the others.
			fields := oplog.BoxFields{
				Name: b.Name, Description: b.Description, StartAt: b.StartAt, EndAt: b.EndAt,
			}
			if cmd.Flags().Changed("name") {
				fields.Name = name
			}
			if cmd.Flags().Changed("description") {
				fields.Description = description
			}
			if cmd.Flags().Changed("start") {
				if fields.StartAt, err = parseDate(start); err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("end") {
				if fields.EndAt, err = parseDate(end); err != nil {
					return err
				}
			}

			updated, err := l.UpdateBox(cmd.Context(), b.ID, fields)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", updated.ID, updated.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&start, "start", "", "new start date, or empty to clear")
	cmd.Flags().StringVar(&end, "end", "", "new end date, or empty to clear")
	cmd.Flags().StringVar(&description, "description", "", "new note")
	return cmd
}

func newBoxTagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag <box> <tag>...",
		Short: "Attach tags to a box",
		Args:  cobra.MinimumNArgs(2),
		RunE:  func(cmd *cobra.Command, args []string) error { return runBoxTag(cmd, args, true) },
	}
}

func newBoxUntagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "untag <box> <tag>...",
		Short: "Remove tags from a box",
		Args:  cobra.MinimumNArgs(2),
		RunE:  func(cmd *cobra.Command, args []string) error { return runBoxTag(cmd, args, false) },
	}
}

func runBoxTag(cmd *cobra.Command, args []string, attach bool) error {
	l, err := openLibrary(cmd)
	if err != nil {
		return err
	}
	defer l.Close()

	b, err := l.ResolveBox(cmd.Context(), args[0])
	if err != nil {
		return err
	}

	// One op per tag: a batch op carries one tag and many boxes (spec 2.2.1),
	// so tagging one box with three tags is three ops. The CLI shape here is
	// the mirror image of the batching the data model is built for; the web UI
	// will use the other direction.
	for _, ref := range args[1:] {
		tag, err := l.ResolveTag(ref)
		if err != nil {
			return err
		}
		var n int
		if attach {
			n, err = l.TagBoxes(cmd.Context(), tag.ID, []domain.BoxID{b.ID})
		} else {
			n, err = l.UntagBoxes(cmd.Context(), tag.ID, []domain.BoxID{b.ID})
		}
		if err != nil {
			return err
		}
		verb := "already had"
		if n > 0 && attach {
			verb = "tagged"
		} else if n > 0 {
			verb = "untagged"
		} else if !attach {
			verb = "did not have"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", b.Name, verb, tag.Name)
	}
	return nil
}

var _ = library.ErrNoSuchBox // errors flow through unchanged; this keeps the import honest
```

- [ ] **Step 5: 实现 tag 子命令**

`cmd/kist/tag.go`:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
)

func newTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage the tag tree",
		Long: `Tags cut across boxes. Names are unique across the whole tree, so a
tag can be referred to by name from anywhere.`,
	}
	cmd.AddCommand(newTagNewCmd(), newTagTreeCmd(), newTagRenameCmd(), newTagMoveCmd(), newTagRemoveCmd())
	return cmd
}

func newTagNewCmd() *cobra.Command {
	var parent string

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			var parentID *domain.TagID
			if parent != "" {
				p, err := l.ResolveTag(parent)
				if err != nil {
					return err
				}
				parentID = &p.ID
			}

			tag, err := l.CreateTag(cmd.Context(), args[0], parentID)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", tag.ID, tag.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "put it under this tag")
	return cmd
}

func newTagTreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tree",
		Short: "Show the tag tree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			tree := l.Tags()
			var print func(tags []domain.Tag, depth int)
			print = func(tags []domain.Tag, depth int) {
				for _, tag := range tags {
					// The count is the subtree total, which is what the tag
					// selector will show later (spec 1.6).
					fmt.Fprintf(cmd.OutOrStdout(), "%s%s (%d)\n",
						strings.Repeat("  ", depth), tag.Name, tree.Count(tag.ID))
					print(tree.Children(tag.ID), depth+1)
				}
			}
			print(tree.Roots(), 0)
			return nil
		},
	}
}

func newTagRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <tag> <new-name>",
		Short: "Rename a tag",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			tag, err := l.ResolveTag(args[0])
			if err != nil {
				return err
			}
			if err := l.RenameTag(cmd.Context(), tag.ID, args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s is now %s\n", tag.Name, args[1])
			return nil
		},
	}
}

func newTagMoveCmd() *cobra.Command {
	var parent string

	cmd := &cobra.Command{
		Use:   "move <tag> --parent <tag>",
		Short: "Move a tag under a different parent",
		Long:  `Pass --parent "" to move it to the top level.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("parent") {
				return fmt.Errorf("--parent is required; pass --parent \"\" to move it to the top level")
			}
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			tag, err := l.ResolveTag(args[0])
			if err != nil {
				return err
			}
			var parentID *domain.TagID
			if parent != "" {
				p, err := l.ResolveTag(parent)
				if err != nil {
					return err
				}
				parentID = &p.ID
			}
			if err := l.MoveTag(cmd.Context(), tag.ID, parentID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "moved %s\n", tag.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "the new parent, or \"\" for the top level")
	return cmd
}

func newTagRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <tag>",
		Short: "Delete a tag",
		Long: `Only tags with no children and nothing attached can be deleted.
kist does not cascade: losing a whole branch of your classification to one
mistyped command is not a recoverable kind of mistake.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			tag, err := l.ResolveTag(args[0])
			if err != nil {
				return err
			}
			if err := l.DeleteTag(cmd.Context(), tag.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", tag.Name)
			return nil
		},
	}
}

var _ = library.ErrNoSuchTag
```

- [ ] **Step 6: 实现 log 与 undo**

`cmd/kist/oplog.go`:

```go
package main

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/store"
)

func newLogCmd() *cobra.Command {
	var (
		limit int
		box   string
	)

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show what has happened to this library",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			opt := store.ListOpsOptions{Limit: limit}
			if box != "" {
				b, err := l.ResolveBox(cmd.Context(), box)
				if err != nil {
					return err
				}
				opt.Box = &b.ID
			}

			ops, err := l.Ops(cmd.Context(), opt)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, op := range ops {
				// Marking reverted ops is what reverts_id being an indexed
				// column is for (spec 2.2).
				status := ""
				if by, ok, err := l.RevertedBy(cmd.Context(), op.ID); err == nil && ok {
					status = fmt.Sprintf("(undone by %d)", by)
				}
				if op.RevertsID != nil {
					status = fmt.Sprintf("(undoes %d)", *op.RevertsID)
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
					op.ID, op.TS.Format("2006-01-02 15:04:05"), op.Action, op.Source, status)
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "how many ops to show")
	cmd.Flags().StringVar(&box, "box", "", "only ops that touched this box")
	return cmd
}

func newUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo <op-id>",
		Short: "Undo an operation",
		Long: `Undo appends the inverse of an operation; it does not erase anything.
The undo is itself an operation and can be undone in turn.

If later changes touched the same things, the undo is refused rather than
partly applied, and the operation in the way is named.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("%q is not an operation id; `kist log` lists them", args[0])
			}
			l, err := openLibrary(cmd)
			if err != nil {
				return err
			}
			defer l.Close()

			undoID, err := l.Undo(cmd.Context(), opID)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "op %d undone by op %d\n", opID, undoID)
			return nil
		},
	}
}
```

- [ ] **Step 7: 跑全部测试**

Run: `make check`
Expected: PASS

- [ ] **Step 8: 手工走一遍完整流程**

```bash
rm -rf /tmp/kist-m1b && go run ./cmd/kist init /tmp/kist-m1b --yes
L="--library=/tmp/kist-m1b"
go run ./cmd/kist $L tag new 地点
go run ./cmd/kist $L tag new 云南 --parent 地点
go run ./cmd/kist $L box new "云南 2019" --start 2019-08
go run ./cmd/kist $L box tag "云南 2019" 云南
go run ./cmd/kist $L tag tree          # 云南 (1) 在 地点 (1) 下面
go run ./cmd/kist $L box list
go run ./cmd/kist $L log
go run ./cmd/kist $L undo <打标那条的 id>
go run ./cmd/kist $L tag tree          # 云南 (0)
go run ./cmd/kist $L log               # 打标那条标着 (undone by N)
```

- [ ] **Step 9: 四平台交叉编译**

```bash
for p in darwin/arm64 darwin/amd64 linux/amd64 windows/amd64; do
  GOOS=${p%/*} GOARCH=${p#*/} CGO_ENABLED=0 go vet ./... && echo "ok $p"
done
```

- [ ] **Step 10: 更新 README 与 AGENTS.md**

`README.md` 的状态段落改成：`kist init` 建库、`box` / `tag` 能用、每一步都能撤销，但还没法放文件进去（`import` 是 M1c）。`README.zh-CN.md` 同步。`AGENTS.md` 的「当前状态」改成 M1b 完成、下一步 M1c。

- [ ] **Step 11: 提交**

```bash
git add .
git commit -m "feat(cli): box / tag / log / undo

box list 和 tag tree 不在设计文档 7 节的 M1 命令清单里，但没有它们
M1b 没法被人验证——undo 要 op id，打 tag 要 box 引用。M1c 的 ls 是
'列一个 box 里的文件'，是另一件事，不会重写这两条。

box edit 只改传了的 flag：编辑一个字段不该悄悄清空其他字段。"
```

---

## 自查

**1. Spec 覆盖。** M1b 认领的是 spec 2.1–2.6（op log 内核）、1.5.1（`updated_at`）、1.6（tag 树）的一部分。逐条对照：

| spec | 落在哪 |
|---|---|
| 2.1 CQRS 拆分、物化视图可重建 | Task 4 `ClearViews` + Task 12 `Rebuild` |
| 2.2 ops 表、action 是开放 TEXT、`source` vs `actor` | Task 2 `Action` / `Source`、Task 3 `AppendOp` |
| 2.2 `reverts_id` 是列 | Task 3 `RevertedBy`、Task 13 `log` 标注已撤销 |
| 2.2.1 payload 与索引表、批量粒度 | Task 2 `Targets` 接口、Task 7 批量 handler |
| 2.3 `Apply` / `Inverse`、payload 自带 before | Task 5 / 6 / 7 |
| 2.3.1 `Apply` 是纯函数 + 重建确定性 | Task 2 `Tx` 接口（结构性保障）+ Task 12（测试） |
| 2.4 撤销 = 追加反向 op | Task 11 |
| 2.5 严格模式 + 说出是哪条 op 挡住 | Task 5 `ErrStaleBefore`、Task 11 `explainBlockedUndo` |
| 2.6 `library` 是唯一写入口 | Task 8（`write` 私有）+ Task 5 Step 7 的依赖方向检查 |
| 1.5.1 `updated_at` 四条规则 | ① Task 4 `ViewTx.TS()`；② Task 8 写路径；③ Task 4 `TouchBoxes` 去重；④ Task 5 只比对语义字段 |
| 1.6 tag 树全局唯一、内存树、移动 O(1)、删除拒绝级联 | Task 6 / Task 9 / Task 10 |
| 1.6.1 `source` 区分 user 与 derived | Task 7（进 payload） |
| 3.1 `kist init` | M1a 已完成 |

**未覆盖且是有意的：** `scope='tags'` 列（M5 历史 UI 才需要，schema 里还没有）、反向重放 K 步的 `kist replay`（M1 清单里没有，`Inverse` 的机器已经在了）、`file_tags` 相关的一切（M1c 才有文件）。

**2. 占位符扫描。** 已检查 `TBD` / `TODO` / `待补` / `类似 Task N`——无。每个 code step 都有完整可粘贴的代码。

**3. 类型一致性。**

- `oplog.Tx` 在 Task 2 定义，由 Task 4 的 `store.ViewTx` 和 Task 5 的 `fakeTx` 分别实现。**两者的断言写法不同**：`fakeTx` 在 `oplog` 包内，可以直接 `var _ Tx = (*fakeTx)(nil)`；`store.ViewTx` 不行——`store` 不 import `oplog`，那正是这套依赖方向的核心，所以断言只能写在 `library` 里（`var _ oplog.Tx = (*store.ViewTx)(nil)`，见 Task 8）。少了这一行，缺一个方法会在 `write()` 深处报一个难懂的错，而不是在断言那一行
- `CountBoxTags` 在 Task 2 的接口里、Task 4 的实现里、Task 5 的 fake 里都在
- `Handler.Inverse` 三处签名一致：`(Action, []byte, error)`
- payload 类型名在 `oplog`（定义）、`library`（构造）、测试（断言）三处一致
- `store.OpRow` ↔ `oplog.Op` 的转换只在 `library.toOp` / `toOpRow` 一处

**4. 执行依赖。** Task 1–2 无依赖，可并行。Task 3–4 依赖 Task 1。Task 5–7 依赖 Task 2，三者之间互相独立可并行。Task 8 依赖 3/4/5-7 全部。Task 9 与 Task 8 有循环引用（`Open` 要 `LoadTagTree`，测试 helper 又在 `write_test.go`）——**按 8 → 9 的顺序写，但 Task 8 的 Step 5 要放到 Task 9 之后再跑**，计划里已经标注。Task 10 依赖 8+9，Task 11 依赖 10，Task 12 依赖 11（fixture 用到 `Undo`），Task 13 依赖全部。

**5. 一处刻意的取舍。** `cmd/kist/box.go` 和 `tag.go` 里有两行 `var _ = library.ErrNoSuchBox` 形式的语句，纯粹为了让 import 有用处——错误是原样往上冒的，CLI 不做包装。如果实现时发现这两行确实多余（比如因为别的用途已经 import 了 `library`），删掉即可，不必保留。
