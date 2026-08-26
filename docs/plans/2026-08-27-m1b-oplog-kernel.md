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

- [ ] **Step 1: 写失败的测试**

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

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/domain/ -run 'TestNewBox|TestValidateName|TestNewTag|TestBoxDates' -v`
Expected: FAIL，`undefined: NewBox`

- [ ] **Step 3: 实现名字校验**

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

- [ ] **Step 4: 实现三个实体**

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

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/domain/ -v`
Expected: PASS

- [ ] **Step 6: 提交**

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

- [ ] **Step 1: 写失败的测试**

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

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/oplog/ -v`
Expected: FAIL，`undefined: Lookup`

- [ ] **Step 3: 写 Action 与 Source**

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

- [ ] **Step 4: 写 Op、payload 编解码、Targets、Tx**

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

- [ ] **Step 5: 写注册表**

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

- [ ] **Step 6: 把 `Inverse` 的签名同步进设计文档**

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

- [ ] **Step 7: 跑测试确认通过**

Run: `go test ./internal/oplog/ -v`
Expected: PASS。注意 `TestLookupFindsRegisteredHandlers` 此时会失败——九个 handler 要到 Task 5–7 才注册。**这是预期的**：把 `AllActions()` 里尚未实现的项暂时注释掉，每完成一个 handler 就放回来一项，Task 7 结束时整份列表恢复。这条在 Task 5/6/7 的步骤里会逐个提醒。

- [ ] **Step 8: 提交**

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

- [ ] **Step 1: 写失败的测试**

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

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/store/ -run 'TestAppendOp|TestGetOp|TestTimestamp|TestReverts|TestListOps|TestInsertOpBoxes|TestLastOp' -v`
Expected: FAIL，`undefined: AppendOp`

- [ ] **Step 3: 实现**

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

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

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

- [ ] **Step 1: 写失败的测试**

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

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/store/ -run 'TestInsertAndGetBox|TestUpdateBox|TestTag|TestBoxTags|TestTouch|TestClearViews|TestListBoxes|TestDeleteBox|TestAddBoxTag|TestGetBoxMissing|TestBoxDates' -v`
Expected: FAIL，`undefined: NewViewTx`

- [ ] **Step 3: 实现 ViewTx**

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

- [ ] **Step 4: 实现读函数**

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

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 6: 提交**

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

- [ ] **Step 1: 写一个假的 Tx**

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

- [ ] **Step 2: 写失败的测试**

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

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./internal/oplog/ -run 'Box' -v`
Expected: FAIL，`undefined: CreateBoxPayload`

- [ ] **Step 4: 实现**

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

- [ ] **Step 5: 把三个 action 放回 `AllActions()`**

Task 2 的 Step 7 说过，`AllActions()` 里未实现的项要暂时注释掉。现在把 `ActionCreateBox` / `ActionDeleteBox` / `ActionUpdateBox` 三项放回来。

- [ ] **Step 6: 跑测试确认通过**

Run: `go test ./internal/oplog/ -v`
Expected: PASS

- [ ] **Step 7: 确认依赖方向没有被破坏**

Run: `go list -deps ./internal/oplog/ | grep kist`
Expected: 只出现 `github.com/jinzhao1994/kist/internal/domain`。**如果出现 `store`，这个 task 就没做对**——`Apply` 必须够不到数据库。

- [ ] **Step 8: 把严格模式的位置写进设计文档**

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

- [ ] **Step 9: 提交**

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
