# M1a 存储内核 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `kist init` 能建出一个真实的库，且底下的内容寻址存储经过完整校验——字节存得进、取得出、动过手脚查得出来。

**Architecture:** 三层，自下而上：`domain` 是零依赖的纯类型（ID、变精度日期、折叠函数）；`blob` 是内容寻址存储与链接能力探测，完全不认识数据库；`store` 只负责连接与 schema 迁移。上面 `library` 目前只有目录布局与建库，`cmd/kist` 只加一个 `init`。**M1a 不写任何 op**——op log、library 的写入口、CLI 的其余命令都在 M1b。

**Tech Stack:** Go 1.26 / `modernc.org/sqlite`（纯 Go）/ `pressly/goose` 迁移 / `google/uuid` UUIDv7 / `BurntSushi/toml` / `golang.org/x/sys` 平台系统调用 / `golang.org/x/text` Unicode 规范化 / `spf13/cobra` / 标准库 `testing`（不引入断言库）

**Spec:** `docs/design/2026-08-22-kist-v0.1-design.md`（配套待决问题见 `docs/design/open-questions.md`）

## Global Constraints

这些是全局要求，每个 task 的验收都隐含包含它们：

- **Go 1.26.1**，版本从 `go.mod` 读，CI 与本地不得漂移
- **`CGO_ENABLED=0` 必须能构建**。任何引入 cgo 的依赖直接否决——单二进制是 v0.1 的核心承诺
- **四个目标平台**：`darwin/arm64` `darwin/amd64` `linux/amd64` `windows/amd64`。平台相关代码用 build tag 分文件，且**每个平台都要有实现**（哪怕是"返回不支持"）
- **不变量 3：`Apply` 必须是纯函数。** M1a 没有 `Apply`，但同源规则现在就生效——**`blob.Put` 决定出的最终 key（含碰撞后缀）必须由调用方拿到并写进 payload**，不能让下游重算（spec 2.3.1）
- **时间戳一律 UTC，格式 `2006-01-02T15:04:05.000Z`**，定宽，字典序即时间序。变精度的 `LocalDateTime` 是另一套，不带时区（spec 1.3）
- **UUID 在 DB 里存 `BLOB(16)`，不是 36 字符 TEXT**（spec 1.6.2）
- **凡是要被 `LIKE` 搜的文本，都由 Go 生成折叠列**（`name_fold`），DB 只做字节比较（spec 4.7）
- **语言：代码、注释、godoc 用英文；设计文档与 commit message 用中文**（`AGENTS.md`）
- 每个 task 结束时 `make check` 必须通过

---

## 文件结构

```
internal/domain/           纯类型，零依赖（除 uuid / x/text）
  id.go        id_test.go          ID 与四个实体 ID 类型
  localdate.go localdate_test.go   变精度日期
  fold.go      fold_test.go        折叠函数

internal/blob/             内容寻址存储，不认识数据库
  key.go       key_test.go         Key 类型与路径推导
  store.go                         Store 接口 + Info / PutResult / Outcome
  dir.go       dir_test.go         本地目录实现
  compare.go   compare_test.go     单次遍历的"重哈希 + 字节比对"
  syncdir_unix.go syncdir_windows.go   目录 fsync 的平台差异
  link.go      link_test.go        链接能力探测与降级
  reflink_darwin.go reflink_linux.go reflink_other.go

internal/store/
  open.go      open_test.go        连接与 pragma
  migrate.go   migrate_test.go     goose 迁移
  migrations/00001_init.sql        v0.1 全量 schema

internal/config/
  config.go    config_test.go      kist.toml

internal/library/
  layout.go    layout_test.go      目录布局
  create.go    create_test.go      建库

internal/cliutil/
  prompt.go    prompt_test.go      交互提问（交互与非交互同一条路径）
  diskfree_unix.go diskfree_windows.go

cmd/kist/
  init.go      init_test.go        kist init
```

拆分依据是**依赖方向单向向下**：`domain` 不 import 任何本项目包；`blob` 只 import `domain`；`store` 只 import `domain`；`library` import 三者；`cmd` 只 import `library` 和 `cliutil`。这条在 M1b 加进 `oplog` 和 `library` 的写入口时必须继续成立，否则不变量 2（`library` 是唯一写入口）在结构上就守不住。

---

### Task 1: domain — ID 类型

**Files:**
- Create: `internal/domain/id.go`
- Test: `internal/domain/id_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `domain.ID`、`domain.BoxID` / `FolderID` / `FileID` / `TagID`、`domain.NewID() ID`、`domain.NewBoxID() BoxID`（及其余三个）、`domain.ParseID(string) (ID, error)`。四个实体类型都内嵌 `ID`，因此 `String()` / `Scan` / `Value` / `MarshalText` 全部通过方法提升可用

**为什么是四个独立类型而不是一个 `ID` 到处传：** `MoveFiles(boxID, folderID, fileIDs)` 这种签名里三个 UUID 挨着，传反了编译器一声不吭。内嵌 `struct{ ID }` 而不是 `type BoxID ID` 是关键——前者能把 `ID` 上的全部方法提升上来（含 `*ID` 上的 `Scan`），后者每个类型都要重抄一遍。且 `struct{ [16]byte }` 仍然可比较，能直接当 map key。

- [ ] **Step 1: 写失败的测试**

```go
package domain

import (
	"database/sql/driver"
	"testing"
)

func TestIDRoundTrip(t *testing.T) {
	id := NewID()

	parsed, err := ParseID(id.String())
	if err != nil {
		t.Fatalf("ParseID(%q): %v", id.String(), err)
	}
	if parsed != id {
		t.Errorf("round trip gave %v, want %v", parsed, id)
	}
}

func TestIDIsTimeOrdered(t *testing.T) {
	// UUIDv7 puts a millisecond timestamp in the high bits, which is why the
	// scheme was chosen (spec 3.12): index locality. Byte order must therefore
	// track creation order.
	first := NewID()
	var last ID
	for i := 0; i < 1000; i++ {
		last = NewID()
	}
	if string(first[:]) >= string(last[:]) {
		t.Errorf("%v was generated first but does not sort before %v", first, last)
	}
}

func TestIDUnique(t *testing.T) {
	seen := make(map[ID]bool, 10000)
	for i := 0; i < 10000; i++ {
		id := NewID()
		if seen[id] {
			t.Fatalf("NewID returned %v twice after %d draws", id, i)
		}
		seen[id] = true
	}
}

func TestParseIDRejectsGarbage(t *testing.T) {
	for _, s := range []string{
		"",
		"not-a-uuid",
		"0198c1f4-0000-7000-8000-00000000000",  // one char short
		"0198c1f4-0000-7000-8000-0000000000000", // one char long
	} {
		if _, err := ParseID(s); err == nil {
			t.Errorf("ParseID(%q) = nil error, want a rejection", s)
		}
	}
}

func TestEntityIDsAreDistinctTypes(t *testing.T) {
	// This is really a compile-time assertion; the runtime part just checks
	// that the embedded value survives the wrapping.
	b := NewBoxID()
	f := NewFileID()

	if b.IsZero() || f.IsZero() {
		t.Fatal("constructors returned zero IDs")
	}
	if b.String() == f.String() {
		t.Error("two constructors produced the same ID")
	}

	// Method promotion: driver.Valuer and sql.Scanner must reach through the
	// embedded ID, or every query has to unwrap by hand.
	var _ driver.Valuer = b
	var v, err = b.Value()
	if err != nil {
		t.Fatalf("Value(): %v", err)
	}
	raw, ok := v.([]byte)
	if !ok || len(raw) != 16 {
		t.Fatalf("Value() = %#v, want a 16-byte blob", v)
	}

	var got BoxID
	if err := got.Scan(raw); err != nil {
		t.Fatalf("Scan(): %v", err)
	}
	if got != b {
		t.Errorf("Scan round trip gave %v, want %v", got, b)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/domain/ -run TestID -v`
Expected: FAIL，`undefined: NewID`

- [ ] **Step 3: 实现**

```go
// Package domain holds kist's core types. It depends on nothing else in this
// repository — everything else may depend on it.
package domain

import (
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ID is a UUIDv7: random, but time-ordered in its high bits so that index
// locality stays good as the library grows (spec 3.12). Stored as BLOB(16),
// never as a 36-character string.
type ID [16]byte

// NewID returns a fresh UUIDv7.
func NewID() ID {
	return ID(uuid.Must(uuid.NewV7()))
}

// ParseID reads the canonical hyphenated form.
func ParseID(s string) (ID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return ID{}, fmt.Errorf("parse id %q: %w", s, err)
	}
	return ID(u), nil
}

func (id ID) String() string { return uuid.UUID(id).String() }

// IsZero reports whether id is the zero value, which is never a valid ID.
func (id ID) IsZero() bool { return id == ID{} }

func (id ID) Value() (driver.Value, error) { return append([]byte(nil), id[:]...), nil }

func (id *ID) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		if len(v) != 16 {
			return fmt.Errorf("scan id: got %d bytes, want 16", len(v))
		}
		copy(id[:], v)
		return nil
	case nil:
		return errors.New("scan id: got NULL")
	default:
		return fmt.Errorf("scan id: cannot read %T", src)
	}
}

func (id ID) MarshalText() ([]byte, error) { return []byte(id.String()), nil }

func (id *ID) UnmarshalText(b []byte) error {
	parsed, err := ParseID(string(b))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// Entity IDs are distinct types so the compiler catches a FolderID passed
// where a BoxID belongs. Embedding rather than defining (`type BoxID ID`)
// promotes every method above, including the pointer-receiver Scan.
type (
	BoxID    struct{ ID }
	FolderID struct{ ID }
	FileID   struct{ ID }
	TagID    struct{ ID }
)

func NewBoxID() BoxID       { return BoxID{NewID()} }
func NewFolderID() FolderID { return FolderID{NewID()} }
func NewFileID() FileID     { return FileID{NewID()} }
func NewTagID() TagID       { return TagID{NewID()} }
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/domain/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
go get github.com/google/uuid
go mod tidy
git add go.mod go.sum internal/domain/id.go internal/domain/id_test.go
git commit -m "feat(domain): UUIDv7 ID 类型，四个实体 ID 互不兼容"
```

---

### Task 2: domain — 变精度日期

**Files:**
- Create: `internal/domain/localdate.go`
- Test: `internal/domain/localdate_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `domain.LocalDate`（零值合法，表示"未设置"）、`domain.Precision` 及常量 `PrecNone` / `PrecYear` / `PrecMonth` / `PrecDay` / `PrecMinute`、`domain.ParseLocalDate(string) (LocalDate, error)`、方法 `String()` / `Precision()` / `IsZero()` / `DirPrefix()`。实现 `TextMarshaler` / `TextUnmarshaler` / `driver.Valuer` / `sql.Scanner`，NULL 扫成零值

**要点：精度不单独存**（spec 1.3）。它由字符串长度推导，`Precision()` 是一个 `switch len(s)`。DB 里只有字符串一列。

`DirPrefix()` 是投影层用的（spec 3.4）：分钟精度渲染成 `2019-08-12_1430` 而不是 ISO 的 `2019-08-12T14:30`，因为 `:` 在 Windows 文件名里非法。**DB 里必须保留带 `T` 和 `:` 的 ISO 形式**，字典序即时间序这条性质建立在它上面。

- [ ] **Step 1: 写失败的测试**

```go
package domain

import (
	"sort"
	"testing"
)

func TestParseLocalDatePrecision(t *testing.T) {
	tests := []struct {
		in   string
		want Precision
	}{
		{"2019", PrecYear},
		{"2019-08", PrecMonth},
		{"2019-08-12", PrecDay},
		{"2019-08-12T14:30", PrecMinute},
	}
	for _, tt := range tests {
		d, err := ParseLocalDate(tt.in)
		if err != nil {
			t.Errorf("ParseLocalDate(%q): %v", tt.in, err)
			continue
		}
		if d.Precision() != tt.want {
			t.Errorf("ParseLocalDate(%q).Precision() = %v, want %v", tt.in, d.Precision(), tt.want)
		}
		if d.String() != tt.in {
			t.Errorf("ParseLocalDate(%q).String() = %q, want the input back", tt.in, d.String())
		}
	}
}

func TestLocalDateRejectsInvalid(t *testing.T) {
	for _, s := range []string{
		"2019-13",           // month 13
		"2019-02-30",        // no such day
		"2019-08-12T25:00",  // hour 25
		"2019-8",            // not zero-padded
		"20190812",          // no separators
		"2019-08-12T14:30:00", // seconds: not a precision we support
		"2019-08-12 14:30",  // space instead of T
		"",                  // use the zero value for "unset", not an empty parse
	} {
		if _, err := ParseLocalDate(s); err == nil {
			t.Errorf("ParseLocalDate(%q) = nil error, want a rejection", s)
		}
	}
}

// The whole variable-precision scheme rests on this property (spec 1.3): the
// lexicographic order of the ISO-8601 prefixes IS chronological order, so no
// database needs special comparison code.
func TestLexicographicOrderIsChronological(t *testing.T) {
	chronological := []string{
		"2018",
		"2019",
		"2019-08",
		"2019-08-12",
		"2019-08-12T09:05",
		"2019-08-12T14:30",
		"2019-09",
		"2020",
	}

	shuffled := []string{
		"2019-08-12T14:30", "2018", "2020", "2019-08",
		"2019-08-12T09:05", "2019", "2019-09", "2019-08-12",
	}
	sort.Strings(shuffled)

	for i := range chronological {
		if shuffled[i] != chronological[i] {
			t.Fatalf("sorted[%d] = %q, want %q\nsorted: %v", i, shuffled[i], chronological[i], shuffled)
		}
	}
}

func TestDirPrefix(t *testing.T) {
	// spec 3.4: the projection layer absorbs platform differences. ':' is
	// illegal in Windows filenames, so minute precision drops the ISO form.
	tests := []struct{ in, want string }{
		{"2019", "2019"},
		{"2019-08", "2019-08"},
		{"2019-08-12", "2019-08-12"},
		{"2019-08-12T14:30", "2019-08-12_1430"},
	}
	for _, tt := range tests {
		d, err := ParseLocalDate(tt.in)
		if err != nil {
			t.Fatalf("ParseLocalDate(%q): %v", tt.in, err)
		}
		if got := d.DirPrefix(); got != tt.want {
			t.Errorf("ParseLocalDate(%q).DirPrefix() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestZeroLocalDate(t *testing.T) {
	var d LocalDate
	if !d.IsZero() {
		t.Error("zero value should report IsZero")
	}
	if d.Precision() != PrecNone {
		t.Errorf("zero value Precision() = %v, want PrecNone", d.Precision())
	}
	v, err := d.Value()
	if err != nil {
		t.Fatalf("Value(): %v", err)
	}
	if v != nil {
		t.Errorf("Value() = %#v, want nil so it stores as NULL", v)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/domain/ -run 'LocalDate|DirPrefix|Lexicographic' -v`
Expected: FAIL，`undefined: ParseLocalDate`

- [ ] **Step 3: 实现**

```go
package domain

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// Precision says how much of a LocalDate is actually known. It is derived from
// the string, never stored alongside it: a second copy of the same fact is a
// second thing that can be wrong (spec 1.3).
type Precision uint8

const (
	PrecNone Precision = iota
	PrecYear
	PrecMonth
	PrecDay
	PrecMinute
)

func (p Precision) String() string {
	switch p {
	case PrecYear:
		return "year"
	case PrecMonth:
		return "month"
	case PrecDay:
		return "day"
	case PrecMinute:
		return "minute"
	default:
		return "none"
	}
}

// LocalDate is a wall-clock date of variable precision, held as an ISO-8601
// prefix: "2019", "2019-08", "2019-08-12", "2019-08-12T14:30".
//
// No timezone, deliberately. It describes human time — "August 12th in
// Yunnan" is that day, and converting to UTC would shorten the trip
// (spec 1.3). Machine timestamps (created_at, updated_at) are a different
// thing entirely and are stored as UTC.
//
// The zero value means "not set" and stores as NULL.
type LocalDate struct {
	s string
}

// layouts maps the length of an accepted prefix to its time layout. Length is
// the discriminator because each precision has exactly one width.
var layouts = map[int]string{
	4:  "2006",
	7:  "2006-01",
	10: "2006-01-02",
	16: "2006-01-02T15:04",
}

// ParseLocalDate accepts exactly the four ISO-8601 prefixes above. It rejects
// everything else, including seconds — widening the set later is easy, but a
// value that slipped in under a looser parser cannot be narrowed again.
func ParseLocalDate(s string) (LocalDate, error) {
	layout, ok := layouts[len(s)]
	if !ok {
		return LocalDate{}, fmt.Errorf("parse local date %q: not one of the accepted forms 2006, 2006-01, 2006-01-02, 2006-01-02T15:04", s)
	}
	if _, err := time.Parse(layout, s); err != nil {
		return LocalDate{}, fmt.Errorf("parse local date %q: %w", s, err)
	}
	return LocalDate{s: s}, nil
}

func (d LocalDate) String() string { return d.s }

func (d LocalDate) IsZero() bool { return d.s == "" }

func (d LocalDate) Precision() Precision {
	switch len(d.s) {
	case 4:
		return PrecYear
	case 7:
		return PrecMonth
	case 10:
		return PrecDay
	case 16:
		return PrecMinute
	default:
		return PrecNone
	}
}

// DirPrefix renders the date for a directory name (spec 3.4). Minute precision
// becomes "2019-08-12_1430": ':' cannot appear in a Windows filename, so the
// ISO form would be uncreatable there. The database keeps the ISO form.
func (d LocalDate) DirPrefix() string {
	if d.Precision() != PrecMinute {
		return d.s
	}
	return strings.NewReplacer("T", "_", ":", "").Replace(d.s)
}

func (d LocalDate) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.s, nil
}

func (d *LocalDate) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*d = LocalDate{}
		return nil
	case string:
		parsed, err := ParseLocalDate(v)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case []byte:
		return d.Scan(string(v))
	default:
		return fmt.Errorf("scan local date: cannot read %T", src)
	}
}

func (d LocalDate) MarshalText() ([]byte, error) { return []byte(d.s), nil }

func (d *LocalDate) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*d = LocalDate{}
		return nil
	}
	parsed, err := ParseLocalDate(string(b))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/domain/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/domain/localdate.go internal/domain/localdate_test.go
git commit -m "feat(domain): 变精度日期，精度由长度推导不单独存"
```

---

### Task 3: domain — 折叠函数

**Files:**
- Create: `internal/domain/fold.go`
- Test: `internal/domain/fold_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `domain.Fold(string) string`

**这是 spec 4.7 那条约束的落点。** `LIKE` 的大小写行为在 SQLite（ASCII 不敏感）和 Postgres（敏感）之间是分歧的，而 v0.1 的搜索完全建立在 `LIKE` 上——如果依赖引擎默认行为，将来接 Postgres 时搜索行为会**静默改变**。做法是写入时由 Go 生成折叠列，查询时用同一个函数处理模式串，DB 只做字节比较。

同一个函数还负责 tag 名的全局唯一（spec 1.6，唯一索引建在 `name_fold` 上）——否则 `Apple` 和 `apple` 会是两个 tag，"唯一"就成了骗人的。

- [ ] **Step 1: 写失败的测试**

```go
package domain

import "testing"

func TestFoldIsCaseInsensitive(t *testing.T) {
	if Fold("Passport") != Fold("passport") {
		t.Error("case must not survive folding")
	}
	if Fold("PASSPORT") != Fold("Passport") {
		t.Error("case must not survive folding")
	}
}

func TestFoldNormalizesUnicode(t *testing.T) {
	// macOS hands out NFD, Linux keeps whatever bytes it was given. Two
	// spellings of the same character must fold to one (spec 3.4).
	nfc := "café"                 // é is U+00E9
	nfd := "cafe\u0301"           // e followed by combining acute
	if nfc == nfd {
		t.Fatal("test inputs are byte-identical; the case under test is missing")
	}
	if Fold(nfc) != Fold(nfd) {
		t.Errorf("Fold(NFC)=%q != Fold(NFD)=%q", Fold(nfc), Fold(nfd))
	}
}

func TestFoldIsIdempotent(t *testing.T) {
	// The query side folds the pattern with the same function that built the
	// column. If folding twice differed from folding once, a folded value
	// re-entering the pipeline would stop matching itself.
	for _, s := range []string{"Passport", "café", "护照", "ÅNGSTRÖM", ""} {
		if once, twice := Fold(s), Fold(Fold(s)); once != twice {
			t.Errorf("Fold(%q) = %q but Fold(Fold(%q)) = %q", s, once, s, twice)
		}
	}
}

func TestFoldLeavesCJKAlone(t *testing.T) {
	// Chinese has no case and does not decompose; folding must be a no-op
	// rather than mangling it.
	if got := Fold("护照"); got != "护照" {
		t.Errorf("Fold(\"护照\") = %q, want it unchanged", got)
	}
}

func TestFoldPreservesInnerSpacing(t *testing.T) {
	// Fold normalizes representation, it does not clean up input. Rejecting
	// leading/trailing whitespace in names is validation's job (spec 1.4's
	// ghost-directory problem) and must not be smuggled in here, or "a b" and
	// "ab" would become the same tag.
	if Fold("a b") == Fold("ab") {
		t.Error("folding must not collapse spaces")
	}
	if Fold(" x ") == Fold("x") {
		t.Error("folding must not trim; that is validation's job")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/domain/ -run TestFold -v`
Expected: FAIL，`undefined: Fold`

- [ ] **Step 3: 实现**

```go
package domain

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Fold produces the comparison form of a piece of user text: NFC-normalized
// and lowercased.
//
// It exists because LIKE's case behaviour differs between engines — SQLite is
// case-insensitive for ASCII, Postgres is not — and v0.1's entire search is
// built on LIKE. Rather than inherit whichever behaviour the engine happens to
// have, kist owns the decision: written into a folded column, matched against
// a pattern put through this same function, compared byte for byte
// (spec 4.7). Search is therefore case-insensitive because we say so.
//
// Fold must stay idempotent: the query side feeds it patterns that may already
// be folded.
func Fold(s string) string {
	// Lowercase first, then normalize: a few characters denormalize when
	// lowercased, and NFC last guarantees the output is in normal form.
	return norm.NFC.String(strings.ToLower(norm.NFC.String(s)))
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/domain/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
go get golang.org/x/text
go mod tidy
git add go.mod go.sum internal/domain/fold.go internal/domain/fold_test.go
git commit -m "feat(domain): 折叠函数，把 LIKE 的大小写语义从引擎手里拿回来"
```

---

### Task 4: blob — Key 类型与路径推导

**Files:**
- Create: `internal/blob/key.go`
- Test: `internal/blob/key_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `blob.Key`（底层 `string`）、`blob.NewKey(algo, hex string, suffix int) Key`、`blob.ParseKey(string) (Key, error)`、方法 `Algo()` / `Hex()` / `Suffix()` / `RelPath()`、常量 `blob.AlgoSHA256 = "sha256"`

**形式是 `sha256:<64 位小写 hex>`，碰撞时 `sha256:<hex>.1`**（spec 3.3）。算法前缀必须同时出现在 key 值和磁盘路径里——只在路径里加而 DB 存裸 hex，拿到哈希值时就不知道该去哪个算法目录找。

顺带一个免费的护栏：`:` 在 Windows 文件名里非法，所以**想拿 key 直接拼路径的代码会当场炸掉**，而不是悄悄写出坏文件。因此 `RelPath()` 是唯一合法的 key→路径通道。

- [ ] **Step 1: 写失败的测试**

```go
package blob

import (
	"path/filepath"
	"strings"
	"testing"
)

const testHex = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

func TestKeyRoundTrip(t *testing.T) {
	tests := []struct {
		suffix int
		want   string
	}{
		{0, "sha256:" + testHex},
		{1, "sha256:" + testHex + ".1"},
		{42, "sha256:" + testHex + ".42"},
	}
	for _, tt := range tests {
		k := NewKey(AlgoSHA256, testHex, tt.suffix)
		if string(k) != tt.want {
			t.Errorf("NewKey(suffix=%d) = %q, want %q", tt.suffix, k, tt.want)
		}

		parsed, err := ParseKey(string(k))
		if err != nil {
			t.Fatalf("ParseKey(%q): %v", k, err)
		}
		if parsed.Algo() != AlgoSHA256 {
			t.Errorf("Algo() = %q, want %q", parsed.Algo(), AlgoSHA256)
		}
		if parsed.Hex() != testHex {
			t.Errorf("Hex() = %q, want %q", parsed.Hex(), testHex)
		}
		if parsed.Suffix() != tt.suffix {
			t.Errorf("Suffix() = %d, want %d", parsed.Suffix(), tt.suffix)
		}
	}
}

func TestParseKeyRejects(t *testing.T) {
	for _, s := range []string{
		"",
		testHex,                            // no algorithm prefix
		"sha256:",                          // no hash
		"sha256:" + testHex[:63],           // too short
		"sha256:" + testHex + "0",          // too long
		"sha256:ABCDEF" + testHex[6:],      // uppercase hex
		"sha256:zzzz" + testHex[4:],        // not hex
		"md5:" + testHex,                   // unknown algorithm
		"sha256:" + testHex + ".",          // empty suffix
		"sha256:" + testHex + ".0",         // suffix 0 must be written as no suffix
		"sha256:" + testHex + ".-1",        // negative
		"sha256:" + testHex + ".x",         // non-numeric suffix
		"sha256/" + testHex,                // path form is not a key
	} {
		if _, err := ParseKey(s); err == nil {
			t.Errorf("ParseKey(%q) = nil error, want a rejection", s)
		}
	}
}

func TestRelPathShards(t *testing.T) {
	k := NewKey(AlgoSHA256, testHex, 0)
	want := filepath.Join("sha256", "ab", "cd", testHex)
	if got := k.RelPath(); got != want {
		t.Errorf("RelPath() = %q, want %q", got, want)
	}

	withSuffix := NewKey(AlgoSHA256, testHex, 1)
	if got, want := withSuffix.RelPath(), want+".1"; got != want {
		t.Errorf("RelPath() with suffix = %q, want %q", got, want)
	}
}

func TestKeyIsNotAPath(t *testing.T) {
	// The ':' is a free guardrail: code that tries to build a path by
	// concatenating a key fails loudly on Windows instead of quietly writing
	// a broken file (spec 3.3).
	k := NewKey(AlgoSHA256, testHex, 0)
	if !strings.Contains(string(k), ":") {
		t.Error("key must contain ':' so it cannot be mistaken for a path")
	}
	if strings.Contains(k.RelPath(), ":") {
		t.Error("RelPath must not contain ':'")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/blob/ -v`
Expected: FAIL，`undefined: NewKey`

- [ ] **Step 3: 实现**

```go
// Package blob is kist's content-addressed store. It knows nothing about the
// database — the only thing linking the two is the key.
package blob

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// AlgoSHA256 is the only hash in v0.1. SHA-256 was chosen over faster options
// because a user can verify it themselves with `shasum -a 256` — an archive's
// trustworthiness comes from being independently checkable (spec 3.3).
const AlgoSHA256 = "sha256"

// Key addresses a blob: "sha256:<64 lowercase hex>", plus ".N" when a hash
// collision forced a second object under the same digest.
//
// The algorithm prefix lives in the key itself, not just in the path, so a key
// read out of the database is enough to find the object without knowing which
// algorithm era it came from.
type Key string

// NewKey builds a key. suffix 0 means no suffix.
func NewKey(algo, hex string, suffix int) Key {
	if suffix == 0 {
		return Key(algo + ":" + hex)
	}
	return Key(algo + ":" + hex + "." + strconv.Itoa(suffix))
}

// ParseKey validates and returns a key. Everything that is not exactly the
// accepted shape is rejected: a key is an address, and a malformed address
// that gets stored is a dangling reference nobody notices until restore time.
func ParseKey(s string) (Key, error) {
	algo, rest, ok := strings.Cut(s, ":")
	if !ok {
		return "", fmt.Errorf("parse blob key %q: missing algorithm prefix", s)
	}
	if algo != AlgoSHA256 {
		return "", fmt.Errorf("parse blob key %q: unknown algorithm %q", s, algo)
	}

	hex, suffixStr, hasSuffix := strings.Cut(rest, ".")
	if len(hex) != 64 {
		return "", fmt.Errorf("parse blob key %q: hash is %d characters, want 64", s, len(hex))
	}
	for i := 0; i < len(hex); i++ {
		c := hex[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return "", fmt.Errorf("parse blob key %q: hash must be lowercase hex", s)
		}
	}

	if hasSuffix {
		n, err := strconv.Atoi(suffixStr)
		if err != nil || n < 1 || suffixStr != strconv.Itoa(n) {
			return "", fmt.Errorf("parse blob key %q: collision suffix must be a positive integer", s)
		}
	}
	return Key(s), nil
}

func (k Key) Algo() string {
	algo, _, _ := strings.Cut(string(k), ":")
	return algo
}

// Hex returns the bare digest, without algorithm prefix or collision suffix.
// This is what goes in the blobs.sha256 column, which is deliberately not
// unique: a collision produces two rows with the same digest (spec 3.3).
func (k Key) Hex() string {
	_, rest, _ := strings.Cut(string(k), ":")
	hex, _, _ := strings.Cut(rest, ".")
	return hex
}

// Suffix returns the collision counter, 0 when there is none.
func (k Key) Suffix() int {
	_, rest, _ := strings.Cut(string(k), ":")
	_, s, ok := strings.Cut(rest, ".")
	if !ok {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

// RelPath is the only legal way to turn a key into a path: "sha256/ab/cd/<hex>".
// Two levels of sharding keep directory sizes manageable; the shard is derived
// from the hash rather than stored, so there is one source of truth.
func (k Key) RelPath() string {
	hex := k.Hex()
	name := hex
	if s := k.Suffix(); s != 0 {
		name += "." + strconv.Itoa(s)
	}
	return filepath.Join(k.Algo(), hex[0:2], hex[2:4], name)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/blob/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/blob/key.go internal/blob/key_test.go
git commit -m "feat(blob): Key 类型与分片路径推导"
```

---

### Task 5: blob — 单次遍历的重哈希 + 字节比对

**Files:**
- Create: `internal/blob/compare.go`
- Test: `internal/blob/compare_test.go`

**Interfaces:**
- Consumes: `blob.AlgoSHA256`
- Produces: `blob.compareAndHash(existingPath, candidatePath string) (identical bool, existingHex string, err error)`（包内私有，Task 6 使用）

**这是 spec 3.3 那个修正的核心。** 两个检查回答不同的问题，缺一不可：

- **重哈希**查"已存在的 blob 有没有烂"
- **字节比对**查"是不是真碰撞"

只做重哈希检测不出碰撞——碰撞的定义就是两份不同内容哈希相同，重哈希在"内容相同"和"真碰撞"下给出完全一样的结果。合成一次遍历后成本没有增加：读已存在的文件时每块既喂哈希器又与候选文件对应块比对。

**长度不同时不能提前返回。** 即使一眼看出字节不同，也必须把已存在的文件读完——因为还要知道它有没有损坏，而那需要完整哈希。

- [ ] **Step 1: 写失败的测试**

```go
package blob

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func hexOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestCompareAndHashIdentical(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "the same bytes")
	b := writeFile(t, dir, "b", "the same bytes")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if !identical {
		t.Error("identical = false, want true")
	}
	if want := hexOf("the same bytes"); h != want {
		t.Errorf("existingHex = %q, want %q", h, want)
	}
}

func TestCompareAndHashDifferentSameLength(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "aaaaaaaa")
	b := writeFile(t, dir, "b", "aaaaaaab")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if identical {
		t.Error("identical = true, want false")
	}
	// The hash must describe the EXISTING file, not the candidate — the caller
	// uses it to decide whether the existing blob is corrupt.
	if want := hexOf("aaaaaaaa"); h != want {
		t.Errorf("existingHex = %q, want the hash of the existing file %q", h, want)
	}
}

func TestCompareAndHashDifferentLength(t *testing.T) {
	dir := t.TempDir()
	// The existing file is longer. A naive implementation that stops at the
	// first mismatch would return a hash of the prefix only, and the caller
	// would then wrongly conclude the existing blob is corrupt.
	a := writeFile(t, dir, "a", "hello world, and then some more")
	b := writeFile(t, dir, "b", "hello world")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if identical {
		t.Error("identical = true, want false")
	}
	if want := hexOf("hello world, and then some more"); h != want {
		t.Errorf("existingHex = %q, want the full hash of the existing file", h)
	}
}

func TestCompareAndHashCandidateLonger(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "hello")
	b := writeFile(t, dir, "b", "hello world")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if identical {
		t.Error("identical = true, want false")
	}
	if want := hexOf("hello"); h != want {
		t.Errorf("existingHex = %q, want %q", h, want)
	}
}

func TestCompareAndHashEmpty(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "")
	b := writeFile(t, dir, "b", "")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if !identical {
		t.Error("two empty files should compare identical")
	}
	if want := hexOf(""); h != want {
		t.Errorf("existingHex = %q, want the hash of the empty string", h)
	}
}

func TestCompareAndHashLargerThanBuffer(t *testing.T) {
	// Exercise the multi-block path, with the difference deliberately placed
	// past the first block boundary.
	dir := t.TempDir()
	base := strings.Repeat("x", 300_000)
	a := writeFile(t, dir, "a", base+"A")
	b := writeFile(t, dir, "b", base+"B")

	identical, h, err := compareAndHash(a, b)
	if err != nil {
		t.Fatalf("compareAndHash: %v", err)
	}
	if identical {
		t.Error("identical = true, want false")
	}
	if want := hexOf(base + "A"); h != want {
		t.Errorf("existingHex is wrong for a multi-block file")
	}
}

func TestCompareAndHashMissingFile(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", "x")

	if _, _, err := compareAndHash(a, filepath.Join(dir, "nope")); err == nil {
		t.Error("expected an error for a missing candidate")
	}
	if _, _, err := compareAndHash(filepath.Join(dir, "nope"), a); err == nil {
		t.Error("expected an error for a missing existing file")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/blob/ -run TestCompareAndHash -v`
Expected: FAIL，`undefined: compareAndHash`

- [ ] **Step 3: 实现**

```go
package blob

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

const compareBlockSize = 128 * 1024

// compareAndHash reads the existing blob once and answers two different
// questions with that one pass:
//
//   - existingHex: does the existing blob still hash to what its name claims?
//     That is a corruption check.
//   - identical: is the candidate byte-for-byte the same content? That is a
//     collision check.
//
// Both are needed. Rehashing alone cannot detect a collision — when two
// different contents hash alike, which is the definition of a collision, the
// rehash result is identical to the deduplication case (spec 3.3).
//
// The existing file is always read to the end, even once a difference is
// known, because the corruption verdict needs its complete hash.
func compareAndHash(existingPath, candidatePath string) (identical bool, existingHex string, err error) {
	existing, err := os.Open(existingPath)
	if err != nil {
		return false, "", fmt.Errorf("open existing blob: %w", err)
	}
	defer existing.Close()

	candidate, err := os.Open(candidatePath)
	if err != nil {
		return false, "", fmt.Errorf("open candidate: %w", err)
	}
	defer candidate.Close()

	h := sha256.New()
	same := true

	eb := make([]byte, compareBlockSize)
	cb := make([]byte, compareBlockSize)

	for {
		en, eerr := io.ReadFull(existing, eb)
		if en > 0 {
			h.Write(eb[:en])
		}

		if same {
			cn, cerr := io.ReadFull(candidate, cb)
			if cn != en || !bytes.Equal(eb[:en], cb[:cn]) {
				same = false
			} else if cerr != nil && !isEOF(cerr) {
				return false, "", fmt.Errorf("read candidate: %w", cerr)
			}
		}

		if eerr != nil {
			if isEOF(eerr) {
				break
			}
			return false, "", fmt.Errorf("read existing blob: %w", eerr)
		}
	}

	if same {
		// The existing file is exhausted; the candidate must be too, otherwise
		// it is a longer file sharing a prefix.
		var probe [1]byte
		if n, _ := candidate.Read(probe[:]); n > 0 {
			same = false
		}
	}

	return same, hex.EncodeToString(h.Sum(nil)), nil
}

func isEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/blob/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/blob/compare.go internal/blob/compare_test.go
git commit -m "feat(blob): 单次遍历同时做重哈希与字节比对

只做重哈希检测不出真碰撞——碰撞的定义就是两份不同内容哈希相同，
重哈希在'内容相同'和'真碰撞'下结果完全一样。两个检查回答不同问题，
合成一次遍历后成本不变。见设计文档 3.3。"
```

---

### Task 6: blob — 本地目录 Store

**Files:**
- Create: `internal/blob/store.go`, `internal/blob/dir.go`, `internal/blob/syncdir_unix.go`, `internal/blob/syncdir_windows.go`
- Test: `internal/blob/dir_test.go`

**Interfaces:**
- Consumes: `blob.Key`、`blob.compareAndHash`
- Produces:
  - `blob.Store` 接口：`Put(ctx, io.Reader) (PutResult, error)` / `Get(ctx, Key) (io.ReadSeekCloser, error)` / `Stat(ctx, Key) (Info, error)` / `Delete(ctx, Key) error` / `LocalPath(Key) (string, bool)`
  - `blob.Info{Key Key; Size int64}`、`blob.PutResult{Info; Outcome Outcome}`
  - `blob.Outcome` 及常量 `OutcomeStored` / `OutcomeDeduped` / `OutcomeRepaired` / `OutcomeCollided`
  - `blob.NewDir(root string, log *slog.Logger) (*Dir, error)`

**与 spec 3.10 的一处偏离：`Put` 返回 `PutResult` 而不是 `(Key, int64, error)`。** 调用方必须能区分"新存了"/"去重命中"/"修复了一个损坏的 blob"/"碰上真碰撞"——导入摘要要报"1200 个文件，340 个去重命中"，而后两者是要高声告警的事件。塞进 error 不对（它们都不是失败），再开一个方法更丑。**本 task 的 Step 6 要把这个签名同步进设计文档 3.10。**

**`chmod 0444` 推迟到"决定保留"之后**（spec 3.3）：去重命中时要删临时文件，而 Windows 上删只读文件必须先清只读属性。推迟之后丢弃路径上的临时文件始终可写，省掉一整条平台分支。对崩溃安全无影响——内容的持久化由 fsync 保证。

**父目录 fsync 在 Windows 上不可用**（`os.Open` 一个目录再 `Sync` 会失败），用 build tag 分文件，Windows 上是 no-op。

- [ ] **Step 1: 写失败的测试**

```go
package blob

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newTestDir(t *testing.T) *Dir {
	t.Helper()
	d, err := NewDir(t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewDir: %v", err)
	}
	return d
}

func put(t *testing.T, d *Dir, content string) PutResult {
	t.Helper()
	res, err := d.Put(context.Background(), strings.NewReader(content))
	if err != nil {
		t.Fatalf("Put(%q): %v", content, err)
	}
	return res
}

func TestPutThenGet(t *testing.T) {
	d := newTestDir(t)
	res := put(t, d, "hello archive")

	if res.Outcome != OutcomeStored {
		t.Errorf("Outcome = %v, want OutcomeStored", res.Outcome)
	}
	if res.Size != int64(len("hello archive")) {
		t.Errorf("Size = %d, want %d", res.Size, len("hello archive"))
	}
	if res.Key.Hex() != hexOf("hello archive") {
		t.Errorf("Key = %q, does not address its content", res.Key)
	}

	r, err := d.Get(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "hello archive" {
		t.Errorf("Get returned %q, want %q", got, "hello archive")
	}
}

func TestPutIsContentAddressedAndDeduplicates(t *testing.T) {
	d := newTestDir(t)

	first := put(t, d, "same content")
	second := put(t, d, "same content")

	if first.Key != second.Key {
		t.Errorf("same content produced different keys: %q and %q", first.Key, second.Key)
	}
	if second.Outcome != OutcomeDeduped {
		t.Errorf("second Outcome = %v, want OutcomeDeduped", second.Outcome)
	}
}

func TestPutLeavesNoTempFiles(t *testing.T) {
	d := newTestDir(t)
	put(t, d, "one")
	put(t, d, "one") // dedup path must clean up its temp file too
	put(t, d, "two")

	entries, err := os.ReadDir(filepath.Join(d.Root(), "tmp"))
	if err != nil {
		t.Fatalf("read tmp: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("tmp/ holds %d leftovers, want none", len(entries))
	}
}

func TestStoredBlobIsReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not apply")
	}
	d := newTestDir(t)
	res := put(t, d, "immutable")

	p, ok := d.LocalPath(res.Key)
	if !ok {
		t.Fatal("LocalPath returned false for a local store")
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// 0444 is the second line of defence behind reflink: if a user edits a file
	// under boxes/ that happens to be a hard link, the write fails instead of
	// corrupting the blob (spec 3.5).
	if perm := fi.Mode().Perm(); perm != 0o444 {
		t.Errorf("stored blob mode = %v, want 0444", perm)
	}
}

func TestPutRepairsCorruptExistingBlob(t *testing.T) {
	d := newTestDir(t)
	res := put(t, d, "original content")

	// Simulate rot: overwrite the blob in place, keeping its name.
	p, _ := d.LocalPath(res.Key)
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := os.WriteFile(p, []byte("rotted!"), 0o600); err != nil {
		t.Fatalf("corrupt the blob: %v", err)
	}

	again := put(t, d, "original content")
	if again.Outcome != OutcomeRepaired {
		t.Errorf("Outcome = %v, want OutcomeRepaired", again.Outcome)
	}
	if again.Key != res.Key {
		t.Errorf("repair changed the key: %q -> %q", res.Key, again.Key)
	}

	r, err := d.Get(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if string(got) != "original content" {
		t.Errorf("after repair Get returned %q, want the original content", got)
	}
}

func TestPutHandlesTrueCollision(t *testing.T) {
	// A real SHA-256 collision cannot be manufactured, so the collision branch
	// is reached by planting different content under a name that claims the
	// candidate's hash — which is exactly the state a collision produces.
	d := newTestDir(t)

	key := NewKey(AlgoSHA256, hexOf("the real content"), 0)
	p := filepath.Join(d.Root(), key.RelPath())
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Content whose own hash matches the filename, but which differs from what
	// we are about to Put: only possible under a collision.
	if err := os.WriteFile(p, []byte("the real content"), 0o444); err != nil {
		t.Fatalf("plant: %v", err)
	}

	// Now Put content that hashes to the same value. We cannot actually
	// produce one, so drive the branch directly.
	res, err := d.putWithHash(context.Background(), strings.NewReader("colliding content"), key.Hex(), int64(len("colliding content")))
	if err != nil {
		t.Fatalf("putWithHash: %v", err)
	}
	if res.Outcome != OutcomeCollided {
		t.Errorf("Outcome = %v, want OutcomeCollided", res.Outcome)
	}
	if res.Key.Suffix() != 1 {
		t.Errorf("Key = %q, want collision suffix .1", res.Key)
	}

	// The original must be untouched — git's SHAttered bug was exactly this:
	// silently keeping one and discarding the other (spec 3.3).
	r, err := d.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get original: %v", err)
	}
	defer r.Close()
	got, _ := io.ReadAll(r)
	if string(got) != "the real content" {
		t.Errorf("the pre-existing blob was modified: %q", got)
	}
}

func TestStat(t *testing.T) {
	d := newTestDir(t)
	res := put(t, d, "measure me")

	info, err := d.Stat(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Key != res.Key || info.Size != int64(len("measure me")) {
		t.Errorf("Stat = %+v, want key %q size %d", info, res.Key, len("measure me"))
	}

	missing := NewKey(AlgoSHA256, hexOf("never stored"), 0)
	if _, err := d.Stat(context.Background(), missing); err == nil {
		t.Error("Stat of a missing blob = nil error, want a rejection")
	}
}

func TestDelete(t *testing.T) {
	d := newTestDir(t)
	res := put(t, d, "temporary")

	// Blobs are 0444; Delete must cope with that on every platform.
	if err := d.Delete(context.Background(), res.Key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := d.Stat(context.Background(), res.Key); err == nil {
		t.Error("blob still present after Delete")
	}
}

func TestGetIsSeekable(t *testing.T) {
	// Range requests over HTTP need this (M4), and it is free for a file.
	d := newTestDir(t)
	res := put(t, d, "0123456789")

	r, err := d.Get(context.Background(), res.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()

	if _, err := r.Seek(5, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	got, _ := io.ReadAll(r)
	if string(got) != "56789" {
		t.Errorf("after Seek(5) read %q, want %q", got, "56789")
	}
}

func TestPutLargeStream(t *testing.T) {
	d := newTestDir(t)
	payload := bytes.Repeat([]byte("kist"), 500_000) // ~2 MB

	res, err := d.Put(context.Background(), bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if res.Size != int64(len(payload)) {
		t.Errorf("Size = %d, want %d", res.Size, len(payload))
	}
}

func TestNewDirCreatesLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "blobs")
	if _, err := NewDir(root, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatalf("NewDir: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(root, "tmp")); err != nil || !fi.IsDir() {
		t.Errorf("tmp/ was not created: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/blob/ -run 'TestPut|TestGet|TestStat|TestDelete|TestNewDir|TestStored' -v`
Expected: FAIL，`undefined: NewDir`

- [ ] **Step 3: 写接口与 Outcome**

```go
package blob

import (
	"context"
	"io"
)

// Info describes a stored blob.
type Info struct {
	Key  Key
	Size int64
}

// Outcome says what Put actually did. Callers need the distinction: an import
// summary reports how much was deduplicated, and the last two are events worth
// shouting about.
type Outcome int

const (
	// OutcomeStored: the content was new.
	OutcomeStored Outcome = iota
	// OutcomeDeduped: identical content was already present.
	OutcomeDeduped
	// OutcomeRepaired: a blob was on disk under this name but no longer hashed
	// to it. It has been replaced with the freshly verified content.
	OutcomeRepaired
	// OutcomeCollided: different content hashed to an existing blob's digest.
	// Stored under a ".N" suffix; the existing blob is untouched.
	OutcomeCollided
)

func (o Outcome) String() string {
	switch o {
	case OutcomeStored:
		return "stored"
	case OutcomeDeduped:
		return "deduped"
	case OutcomeRepaired:
		return "repaired"
	case OutcomeCollided:
		return "collided"
	default:
		return "unknown"
	}
}

// PutResult is what Put returns.
type PutResult struct {
	Info
	Outcome Outcome
}

// Store is content-addressed blob storage. v0.1 has two implementations: the
// library's own directory, and a remote directory used as a backup and repair
// source (spec 3.10).
type Store interface {
	// Put stores the reader's content and returns the key it was addressed
	// under. It is idempotent for identical content.
	//
	// The returned key is authoritative: a collision suffix is decided here,
	// and the caller must write that key into the op payload rather than
	// recomputing it later (spec 2.3.1).
	Put(ctx context.Context, r io.Reader) (PutResult, error)

	Get(ctx context.Context, k Key) (io.ReadSeekCloser, error)
	Stat(ctx context.Context, k Key) (Info, error)
	Delete(ctx context.Context, k Key) error

	// LocalPath returns a filesystem path for the blob, if this store has one.
	// The link farm under boxes/ only works for local blobs; once an object
	// moves to cold storage this returns false and the projection has to put
	// a stub there instead (spec 3.10). v0.1 is all local.
	LocalPath(k Key) (string, bool)
}
```

- [ ] **Step 4: 写目录实现**

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
	"log/slog"
	"os"
	"path/filepath"
)

// Dir stores blobs in a local directory: <root>/sha256/ab/cd/<hex>, with
// <root>/tmp as the staging area.
type Dir struct {
	root string
	log  *slog.Logger
}

var _ Store = (*Dir)(nil)

// NewDir prepares root as a blob directory, creating it if needed.
func NewDir(root string, log *slog.Logger) (*Dir, error) {
	if log == nil {
		log = slog.Default()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve blob root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(abs, "tmp"), 0o755); err != nil {
		return nil, fmt.Errorf("create blob layout: %w", err)
	}
	return &Dir{root: abs, log: log}, nil
}

// Root is the directory this store writes into.
func (d *Dir) Root() string { return d.root }

func (d *Dir) LocalPath(k Key) (string, bool) {
	return filepath.Join(d.root, k.RelPath()), true
}

// Put streams r into the staging area while hashing it, then decides where it
// belongs. The order — blob durable before the database ever hears about it —
// is what makes a crash leave an orphan rather than a dangling reference
// (spec 3.7).
func (d *Dir) Put(ctx context.Context, r io.Reader) (PutResult, error) {
	tmp, err := os.CreateTemp(filepath.Join(d.root, "tmp"), "put-*")
	if err != nil {
		return PutResult{}, fmt.Errorf("create temp blob: %w", err)
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
	size, err := io.Copy(io.MultiWriter(tmp, h), r)
	if err != nil {
		return PutResult{}, fmt.Errorf("stage blob: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return PutResult{}, fmt.Errorf("fsync staged blob: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return PutResult{}, fmt.Errorf("close staged blob: %w", err)
	}

	res, err := d.place(ctx, tmpPath, hex.EncodeToString(h.Sum(nil)), size)
	if err != nil {
		return PutResult{}, err
	}
	keep = res.Outcome == OutcomeStored || res.Outcome == OutcomeRepaired || res.Outcome == OutcomeCollided
	return res, nil
}

// putWithHash exists for tests that need to drive the collision branch, which
// cannot be reached by supplying honest content.
func (d *Dir) putWithHash(ctx context.Context, r io.Reader, digest string, size int64) (PutResult, error) {
	tmp, err := os.CreateTemp(filepath.Join(d.root, "tmp"), "put-*")
	if err != nil {
		return PutResult{}, err
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		tmp.Close()
		if !keep {
			os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, r); err != nil {
		return PutResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return PutResult{}, err
	}
	res, err := d.place(ctx, tmpPath, digest, size)
	if err != nil {
		return PutResult{}, err
	}
	keep = res.Outcome != OutcomeDeduped
	return res, nil
}

// place walks the collision suffixes until it finds a home for the staged file.
func (d *Dir) place(ctx context.Context, tmpPath, digest string, size int64) (PutResult, error) {
	for suffix := 0; ; suffix++ {
		if err := ctx.Err(); err != nil {
			return PutResult{}, err
		}

		key := NewKey(AlgoSHA256, digest, suffix)
		dst := filepath.Join(d.root, key.RelPath())

		_, err := os.Stat(dst)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := d.commit(tmpPath, dst); err != nil {
				return PutResult{}, err
			}
			outcome := OutcomeStored
			if suffix != 0 {
				outcome = OutcomeCollided
			}
			return PutResult{Info: Info{Key: key, Size: size}, Outcome: outcome}, nil
		case err != nil:
			return PutResult{}, fmt.Errorf("stat existing blob: %w", err)
		}

		identical, existingHex, err := compareAndHash(dst, tmpPath)
		if err != nil {
			return PutResult{}, err
		}

		switch {
		case existingHex != digest:
			// The blob on disk no longer hashes to its own name: bit rot, a
			// half-restored backup, an antivirus quarantine. The staged file
			// was just hashed and is trustworthy, so heal with it.
			d.log.Error("blob on disk did not match its own hash; replacing it with freshly verified content",
				"key", key, "expected", digest, "found", existingHex, "path", dst)
			if err := d.replace(tmpPath, dst); err != nil {
				return PutResult{}, err
			}
			return PutResult{Info: Info{Key: key, Size: size}, Outcome: OutcomeRepaired}, nil

		case identical:
			return PutResult{Info: Info{Key: key, Size: size}, Outcome: OutcomeDeduped}, nil

		default:
			// Same digest, different bytes. Keep both — git kept only one and
			// called it a data-loss bug for years (spec 3.3).
			d.log.Error("SHA-256 collision: different content hashes to an existing blob; storing under the next suffix",
				"digest", digest, "existing", key)
			continue
		}
	}
}

// commit makes the staged file permanent at dst.
func (d *Dir) commit(tmpPath, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create shard directory: %w", err)
	}
	// chmod happens here, not right after fsync, so that every path which
	// discards the temp file is dealing with a writable one — deleting a
	// read-only file needs extra work on Windows (spec 3.3).
	if err := os.Chmod(tmpPath, 0o444); err != nil {
		return fmt.Errorf("seal blob: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("commit blob: %w", err)
	}
	return syncDir(filepath.Dir(dst))
}

// replace overwrites a corrupt blob. The existing file is read-only, which
// blocks rename on Windows, so its mode is cleared first.
func (d *Dir) replace(tmpPath, dst string) error {
	if err := os.Chmod(dst, 0o644); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("unseal corrupt blob: %w", err)
	}
	return d.commit(tmpPath, dst)
}

func (d *Dir) Get(_ context.Context, k Key) (io.ReadSeekCloser, error) {
	p, _ := d.LocalPath(k)
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("open blob %s: %w", k, err)
	}
	return f, nil
}

func (d *Dir) Stat(_ context.Context, k Key) (Info, error) {
	p, _ := d.LocalPath(k)
	fi, err := os.Stat(p)
	if err != nil {
		return Info{}, fmt.Errorf("stat blob %s: %w", k, err)
	}
	return Info{Key: k, Size: fi.Size()}, nil
}

func (d *Dir) Delete(_ context.Context, k Key) error {
	p, _ := d.LocalPath(k)
	// Blobs are 0444. On Windows that alone blocks deletion.
	if err := os.Chmod(p, 0o644); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("unseal blob %s: %w", k, err)
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete blob %s: %w", k, err)
	}
	return nil
}
```

`internal/blob/syncdir_unix.go`:

```go
//go:build !windows

package blob

import (
	"fmt"
	"os"
)

// syncDir flushes a directory entry so a rename survives a power cut. Without
// it the file's contents are durable but the name pointing at them may not be.
func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for fsync: %w", err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("fsync directory: %w", err)
	}
	return nil
}
```

`internal/blob/syncdir_windows.go`:

```go
//go:build windows

package blob

// syncDir is a no-op on Windows: directories cannot be opened for
// synchronization there. NTFS journals its metadata, so a committed rename is
// durable without an explicit flush.
func syncDir(string) error { return nil }
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/blob/ -v`
Expected: PASS（全部）

- [ ] **Step 6: 把接口偏离同步进设计文档**

修改 `docs/design/2026-08-22-kist-v0.1-design.md` 的 3.10 节，把 `Put` 的签名改成返回 `PutResult`，并写明理由：

```
Put(ctx context.Context, r io.Reader) (PutResult, error)   // 内容寻址，幂等
```

> **`Put` 返回 `PutResult` 而不是裸 key。** 调用方必须能区分四种结果——新存入 / 去重命中 / 修复了一个损坏的 blob / 撞上真碰撞。前两个是导入摘要要报的数字（"1200 个文件，340 个去重命中"），后两个是要高声告警的事件。它们都不是错误，塞进 `error` 语义不对；再开第二个方法则会让远端实现也被迫实现两遍。

- [ ] **Step 7: 提交**

```bash
git add internal/blob/ docs/design/2026-08-22-kist-v0.1-design.md
git commit -m "feat(blob): 本地目录 Store，原子写 + 去重 + 损坏自愈 + 碰撞分支"
```

---

### Task 7: blob — 链接能力探测与降级

**Files:**
- Create: `internal/blob/link.go`, `internal/blob/reflink_darwin.go`, `internal/blob/reflink_linux.go`, `internal/blob/reflink_other.go`
- Test: `internal/blob/link_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `blob.LinkMode`（底层 `string`）及常量 `LinkReflink` / `LinkHardlink` / `LinkSymlink` / `LinkCopy`、`blob.DetectLinkMode(fromDir, toDir string) (LinkMode, error)`、`blob.Link(mode LinkMode, src, dst string) error`、`blob.ErrUnsupportedLink`

**降级顺序：reflink → hardlink → symlink（非 Windows）→ 完整拷贝**（spec 3.5）。

**探测是真做一次，不靠猜文件系统类型。** 猜的方式要维护一张文件系统对照表，还答不了"这两个目录是不是同一个卷"。

**与 spec 的一处简化：不单独比对 `st_dev`。** spec 提到用 `st_dev` 判断是否跨文件系统，但真实探测已经涵盖了它——跨设备的 `os.Link` 直接返回 `EXDEV`。省掉 `st_dev` 的好处是不用为 Windows 另写一套卷序列号的代码（那边根本没有 `st_dev`），一条代码路径管四个平台。

**macOS 上 reflink 优先于硬链接**：硬链接共享 inode，用户就地编辑 `boxes/` 下的文件会直接改坏 blob；reflink 是独立 inode 共享块，没有这个问题。**Windows 上不用符号链接**：它需要管理员权限或开发者模式，硬链接不需要。

- [ ] **Step 1: 写失败的测试**

```go
package blob

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectLinkModeReturnsSomethingUsable(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()

	mode, err := DetectLinkMode(from, to)
	if err != nil {
		t.Fatalf("DetectLinkMode: %v", err)
	}
	switch mode {
	case LinkReflink, LinkHardlink, LinkSymlink, LinkCopy:
	default:
		t.Fatalf("DetectLinkMode returned %q, not one of the four modes", mode)
	}

	// Copy always works, so detection must never fall below it.
	t.Logf("detected %q on %s", mode, runtime.GOOS)
}

func TestDetectLinkModeLeavesNothingBehind(t *testing.T) {
	from, to := t.TempDir(), t.TempDir()

	if _, err := DetectLinkMode(from, to); err != nil {
		t.Fatalf("DetectLinkMode: %v", err)
	}
	for _, dir := range []string{from, to} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		if len(entries) != 0 {
			t.Errorf("%s holds %d probe leftovers, want none", dir, len(entries))
		}
	}
}

func TestLinkReproducesContentInEverySupportedMode(t *testing.T) {
	for _, mode := range []LinkMode{LinkReflink, LinkHardlink, LinkSymlink, LinkCopy} {
		t.Run(string(mode), func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "src")
			dst := filepath.Join(dir, "dst")
			if err := os.WriteFile(src, []byte("linked content"), 0o444); err != nil {
				t.Fatalf("write source: %v", err)
			}

			err := Link(mode, src, dst)
			if isUnsupported(err) {
				t.Skipf("%s is not available here", mode)
			}
			if err != nil {
				t.Fatalf("Link(%s): %v", mode, err)
			}

			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read destination: %v", err)
			}
			if string(got) != "linked content" {
				t.Errorf("destination holds %q, want %q", got, "linked content")
			}
		})
	}
}

func TestLinkRejectsUnknownMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Link(LinkMode("teleport"), src, filepath.Join(dir, "dst")); err == nil {
		t.Error("Link with an unknown mode = nil error, want a rejection")
	}
}

func TestNoSymlinksOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only policy")
	}
	// Symlinks need administrator rights or developer mode there, hard links
	// do not (spec 3.5). Detection must never choose symlink on Windows.
	from, to := t.TempDir(), t.TempDir()
	mode, err := DetectLinkMode(from, to)
	if err != nil {
		t.Fatalf("DetectLinkMode: %v", err)
	}
	if mode == LinkSymlink {
		t.Error("detection chose symlink on Windows")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/blob/ -run 'Link' -v`
Expected: FAIL，`undefined: DetectLinkMode`

- [ ] **Step 3: 实现**

`internal/blob/link.go`:

```go
package blob

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// LinkMode is how the boxes/ projection points at a blob.
type LinkMode string

const (
	// LinkReflink shares blocks but not the inode: copy-on-write. Preferred on
	// macOS and on btrfs/XFS because editing the projected file in place
	// cannot corrupt the blob.
	LinkReflink LinkMode = "reflink"
	// LinkHardlink shares the inode. Free, but an in-place edit of the
	// projection would reach the blob — 0444 is the guard.
	LinkHardlink LinkMode = "hardlink"
	// LinkSymlink is a fallback for filesystems with neither. Not used on
	// Windows, where it needs elevation.
	LinkSymlink LinkMode = "symlink"
	// LinkCopy duplicates the bytes. Always works, costs the space.
	LinkCopy LinkMode = "copy"
)

// ErrUnsupportedLink means the platform or filesystem cannot do this.
var ErrUnsupportedLink = errors.New("link mode not supported here")

func isUnsupported(err error) bool { return errors.Is(err, ErrUnsupportedLink) }

// candidateModes lists the modes to try, best first. Symlink is skipped on
// Windows: it requires administrator rights or developer mode there, while
// hard links do not (spec 3.5).
func candidateModes() []LinkMode {
	if runtime.GOOS == "windows" {
		return []LinkMode{LinkReflink, LinkHardlink, LinkCopy}
	}
	return []LinkMode{LinkReflink, LinkHardlink, LinkSymlink, LinkCopy}
}

// DetectLinkMode finds the best mode that actually works between two
// directories, by trying each one for real.
//
// Probing beats inspecting the filesystem type: a type table has to be
// maintained forever, and it still cannot answer "are these two directories on
// the same volume". A genuine attempt answers both — a cross-device hard link
// simply fails with EXDEV — which is also why there is no separate st_dev
// check: it would need its own Windows implementation to say something the
// probe already established.
func DetectLinkMode(fromDir, toDir string) (LinkMode, error) {
	probe, err := os.CreateTemp(fromDir, ".kist-probe-*")
	if err != nil {
		return "", fmt.Errorf("create link probe: %w", err)
	}
	probePath := probe.Name()
	defer os.Remove(probePath)

	if _, err := probe.WriteString("kist link capability probe"); err != nil {
		probe.Close()
		return "", fmt.Errorf("write link probe: %w", err)
	}
	if err := probe.Close(); err != nil {
		return "", fmt.Errorf("close link probe: %w", err)
	}

	target := filepath.Join(toDir, ".kist-probe-target")
	for _, mode := range candidateModes() {
		os.Remove(target)
		if err := Link(mode, probePath, target); err == nil {
			os.Remove(target)
			return mode, nil
		}
	}
	os.Remove(target)
	return "", fmt.Errorf("no usable link mode between %s and %s", fromDir, toDir)
}

// Link points dst at src using the given mode.
func Link(mode LinkMode, src, dst string) error {
	switch mode {
	case LinkReflink:
		return reflink(src, dst)
	case LinkHardlink:
		return os.Link(src, dst)
	case LinkSymlink:
		if runtime.GOOS == "windows" {
			return fmt.Errorf("symlink on windows: %w", ErrUnsupportedLink)
		}
		return os.Symlink(src, dst)
	case LinkCopy:
		return copyFile(src, dst)
	default:
		return fmt.Errorf("unknown link mode %q", mode)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
```

`internal/blob/reflink_darwin.go`:

```go
//go:build darwin

package blob

import "golang.org/x/sys/unix"

// reflink uses clonefile(2), available on APFS. Preferred over a hard link
// because the clone gets its own inode: editing the projected file cannot
// reach the blob (spec 3.5).
func reflink(src, dst string) error {
	return unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW)
}
```

`internal/blob/reflink_linux.go`:

```go
//go:build linux

package blob

import (
	"os"

	"golang.org/x/sys/unix"
)

// reflink uses the FICLONE ioctl, available on btrfs and XFS. It fails with
// EOPNOTSUPP on ext4, which is what makes detection fall through to hard links.
func reflink(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}
```

`internal/blob/reflink_other.go`:

```go
//go:build !darwin && !linux

package blob

import "fmt"

// reflink is unavailable: Windows has no copy-on-write clone API that NTFS
// supports, and other platforms are not targeted.
func reflink(_, _ string) error {
	return fmt.Errorf("reflink: %w", ErrUnsupportedLink)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/blob/ -v`
Expected: PASS。macOS 上 `TestLinkReproducesContentInEverySupportedMode/reflink` 应当真的跑而不是 skip；Linux ext4 上它会 skip

- [ ] **Step 5: 提交**

```bash
go get golang.org/x/sys
go mod tidy
git add go.mod go.sum internal/blob/
git commit -m "feat(blob): 链接能力真实探测与四级降级"
```

---

### Task 8: store — 连接与 pragma

**Files:**
- Create: `internal/store/open.go`
- Test: `internal/store/open_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `store.Open(path string) (*sql.DB, error)`、`store.OpenMemory() (*sql.DB, error)`（测试用）

**pragma 必须走 DSN，不能开完连接再执行。** `sql.DB` 是连接池，随时可能开新连接；`db.Exec("PRAGMA ...")` 只作用于当时那一条。`modernc.org/sqlite` 支持 DSN 里的 `_pragma=`，每条新连接都会应用。**这一条写错的表现是间歇性的外键不生效**，极难查。

四条 pragma 及理由：

| pragma | 值 | 为什么 |
|---|---|---|
| `journal_mode` | `WAL` | 读不阻塞写。导入时前端还要能浏览 |
| `foreign_keys` | `1` | **SQLite 默认是关的**。schema 里那些外键不开就是注释 |
| `busy_timeout` | `5000` | 拿不到锁时等待而不是立刻 `SQLITE_BUSY` |
| `synchronous` | `NORMAL` | WAL 下的推荐值。`FULL` 每次提交都 fsync，导入会慢一个量级；崩溃最多丢最后一个事务，而 blob 已先落盘（spec 3.7） |

- [ ] **Step 1: 写失败的测试**

```go
package store

import (
	"path/filepath"
	"testing"
)

func TestOpenAppliesPragmas(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "kist.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var journal string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journal != "wal" {
		t.Errorf("journal_mode = %q, want wal", journal)
	}

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

// The pragmas have to be in the DSN, not executed after opening: database/sql
// is a pool and will hand out fresh connections at will. This test forces
// several connections to exist at once and checks them all.
func TestPragmasApplyToEveryPooledConnection(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "kist.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	const parallel = 4
	conns := make([]*sql.Conn, 0, parallel)
	for i := 0; i < parallel; i++ {
		c, err := db.Conn(context.Background())
		if err != nil {
			t.Fatalf("Conn %d: %v", i, err)
		}
		conns = append(conns, c)
	}
	for i, c := range conns {
		var fk int
		if err := c.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatalf("conn %d: %v", i, err)
		}
		if fk != 1 {
			t.Errorf("conn %d has foreign_keys = %d, want 1", i, fk)
		}
		c.Close()
	}
}

func TestForeignKeysActuallyBite(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE parent (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE child (parent_id INTEGER REFERENCES parent(id))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO child VALUES (999)`); err == nil {
		t.Error("inserted a dangling reference; foreign keys are not enforced")
	}
}

func TestOpenRejectsUnwritablePath(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "no-such-dir", "kist.db")); err == nil {
		t.Error("Open into a missing directory = nil error, want a failure")
	}
}
```

（测试文件顶部的 import 需要 `context`、`database/sql`、`path/filepath`、`testing`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/store/ -v`
Expected: FAIL，`undefined: Open`

- [ ] **Step 3: 实现**

```go
// Package store owns the database connection and its schema. Query code lives
// with the callers; this package only guarantees a correctly configured
// handle.
package store

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// pragmas are appended to every DSN.
//
// They must travel in the DSN rather than being executed after opening:
// database/sql pools connections and opens new ones on demand, so a PRAGMA run
// once applies to exactly one connection. The failure mode of getting this
// wrong — foreign keys silently unenforced on some queries but not others — is
// close to undebuggable.
var pragmas = []string{
	"journal_mode(WAL)",  // readers do not block the writer
	"foreign_keys(1)",    // OFF by default in SQLite; without it the schema's FKs are comments
	"busy_timeout(5000)", // wait for a lock instead of failing instantly
	"synchronous(NORMAL)", // WAL's recommended setting; blobs are already durable (spec 3.7)
}

func dsn(path string) string {
	q := url.Values{}
	for _, p := range pragmas {
		q.Add("_pragma", p)
	}
	return "file:" + path + "?" + q.Encode()
}

// Open opens the library database at path, creating it if absent.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	return db, nil
}

// OpenMemory returns a private in-memory database with the same configuration,
// for tests.
func OpenMemory() (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn(":memory:"))
	if err != nil {
		return nil, fmt.Errorf("open in-memory database: %w", err)
	}
	// An in-memory database lives inside one connection; more than one would
	// mean more than one database.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

若 `TestPragmasApplyToEveryPooledConnection` 失败，说明 `_pragma=` 没被驱动接受——先用 `go doc modernc.org/sqlite` 核对当前版本的 DSN 参数名，不要改成"开完连接再 Exec"绕过去。

- [ ] **Step 5: 提交**

```bash
go get modernc.org/sqlite
go mod tidy
git add go.mod go.sum internal/store/open.go internal/store/open_test.go
git commit -m "feat(store): SQLite 连接，pragma 走 DSN 以覆盖连接池每条连接"
```

---

### Task 9: store — schema 迁移

**Files:**
- Create: `internal/store/migrate.go`, `internal/store/migrations/00001_init.sql`
- Test: `internal/store/migrate_test.go`

**Interfaces:**
- Consumes: `store.Open` / `store.OpenMemory`
- Produces: `store.Migrate(db *sql.DB) error`

**一次性写入 v0.1 全量 schema。** 尽管 M1a 只用到其中几张表——发布前没有存量库，多攒几次迁移只是给自己添麻烦；而 schema 在设计文档里已经定稿。

测试的重点不是"表建出来了"，而是**约束真的会咬**。spec 1.4 自己警告过的 NULL 坑（`UNIQUE(parent_id, name)` 对根行不生效）就是那种"两个引擎上都看起来能跑、实际两边都漏"的问题，必须有测试钉住。

- [ ] **Step 1: 写失败的测试**

```go
package store

import (
	"strings"
	"testing"
)

func migrated(t *testing.T) *sql.DB {
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

func TestMigrateCreatesEveryTable(t *testing.T) {
	db := migrated(t)

	want := []string{
		"blobs", "box_tags", "boxes", "file_tags", "files", "folders",
		"metadata", "op_boxes", "op_files", "ops", "tag_rejections", "tags",
	}
	for _, table := range want {
		var n int
		err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n)
		if err != nil {
			t.Fatalf("query sqlite_master: %v", err)
		}
		if n != 1 {
			t.Errorf("table %q is missing", table)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := migrated(t)
	if err := Migrate(db); err != nil {
		t.Errorf("second Migrate: %v", err)
	}
}

func TestOneRootFolderPerBox(t *testing.T) {
	// The naive UNIQUE(parent_id, name) does not bite for root folders: their
	// parent_id is NULL and NULLs are never equal, so a box could grow two
	// roots and fork the whole projection (spec 1.4).
	db := migrated(t)
	box := insertBox(t, db, "trip")

	insertRootFolder(t, db, box)
	if err := tryInsertRootFolder(db, box); err == nil {
		t.Error("a second root folder was accepted for the same box")
	}
}

func TestFolderNamesUniqueWithinParent(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "trip")
	root := insertRootFolder(t, db, box)

	insertFolder(t, db, box, root, "receipts")
	if err := tryInsertFolder(db, box, root, "receipts"); err == nil {
		t.Error("duplicate folder name accepted under the same parent")
	}
}

func TestTagNamesAreGloballyUnique(t *testing.T) {
	// spec 1.6: the hierarchy is not a namespace. Uniqueness is on the folded
	// name, or "Apple" and "apple" would be two tags and "unique" would be a
	// lie to the user.
	db := migrated(t)

	people := insertTag(t, db, nil, "People", "people")
	if err := tryInsertTag(db, &people, "People", "people"); err == nil {
		t.Error("the same tag name was accepted under a different parent")
	}
	if err := tryInsertTag(db, nil, "PEOPLE", "people"); err == nil {
		t.Error("a case variant was accepted as a second tag")
	}
}

func TestFileNamesUniqueWithinFolder(t *testing.T) {
	db := migrated(t)
	box := insertBox(t, db, "trip")
	root := insertRootFolder(t, db, box)
	key := insertBlob(t, db, "sha256:"+strings.Repeat("ab", 32))

	insertFile(t, db, box, root, "receipt.jpg", key)
	if err := tryInsertFile(db, box, root, "receipt.jpg", key); err == nil {
		t.Error("duplicate file name accepted in the same folder")
	}
}

func TestBlobSha256IsNotUnique(t *testing.T) {
	// A collision produces two rows with the same digest under different keys
	// (spec 3.3). A unique index here would make that unstorable.
	db := migrated(t)
	digest := strings.Repeat("cd", 32)

	insertBlob(t, db, "sha256:"+digest)
	if _, err := db.Exec(`INSERT INTO blobs (key, sha256, size) VALUES (?, ?, ?)`,
		"sha256:"+digest+".1", digest, 1); err != nil {
		t.Errorf("second row with the same digest was rejected: %v", err)
	}
}

func TestOpIndexTablesSurviveEntityDeletion(t *testing.T) {
	// History outlives its subject: op_boxes must not have a foreign key to
	// boxes, or deleting a box would take the record of that deletion with it
	// (spec 4.6).
	db := migrated(t)
	box := insertBox(t, db, "doomed")

	res, err := db.Exec(`INSERT INTO ops (ts, action, source, payload) VALUES (?, ?, ?, ?)`,
		"2026-08-22T00:00:00.000Z", "delete_box", "user", `{}`)
	if err != nil {
		t.Fatalf("insert op: %v", err)
	}
	opID, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO op_boxes (op_id, box_id) VALUES (?, ?)`, opID, box); err != nil {
		t.Fatalf("insert op_boxes: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM boxes WHERE id = ?`, box); err != nil {
		t.Fatalf("delete box: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM op_boxes WHERE box_id = ?`, box).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("op_boxes rows after deleting the box = %d, want 1", n)
	}
}

func TestOpIDsAreMonotonic(t *testing.T) {
	// ops.id is the version number; the entire history model assumes it only
	// goes up.
	db := migrated(t)

	var ids []int64
	for i := 0; i < 3; i++ {
		res, err := db.Exec(`INSERT INTO ops (ts, action, source, payload) VALUES (?, ?, ?, ?)`,
			"2026-08-22T00:00:00.000Z", "noop", "user", `{}`)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		ids = append(ids, id)
	}
	if _, err := db.Exec(`DELETE FROM ops WHERE id = ?`, ids[2]); err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO ops (ts, action, source, payload) VALUES (?, ?, ?, ?)`,
		"2026-08-22T00:00:00.000Z", "noop", "user", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := res.LastInsertId()
	if next <= ids[2] {
		t.Errorf("op id %d was reused after a delete; AUTOINCREMENT is missing", next)
	}
}
```

测试辅助函数写在同一个文件里：

```go
func insertBox(t *testing.T, db *sql.DB, name string) []byte {
	t.Helper()
	id := randomID(t)
	_, err := db.Exec(`INSERT INTO boxes (id, name, name_fold, description, created_at, updated_at)
	                   VALUES (?, ?, ?, '', ?, ?)`,
		id, name, strings.ToLower(name), testTS, testTS)
	if err != nil {
		t.Fatalf("insert box: %v", err)
	}
	return id
}

const testTS = "2026-08-22T00:00:00.000Z"

func randomID(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func insertRootFolder(t *testing.T, db *sql.DB, box []byte) []byte {
	t.Helper()
	id := randomID(t)
	if _, err := db.Exec(`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
	                      VALUES (?, ?, NULL, '', '', ?, ?)`, id, box, testTS, testTS); err != nil {
		t.Fatalf("insert root folder: %v", err)
	}
	return id
}

func tryInsertRootFolder(db *sql.DB, box []byte) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := db.Exec(`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
	                   VALUES (?, ?, NULL, '', '', ?, ?)`, b, box, testTS, testTS)
	return err
}

func insertFolder(t *testing.T, db *sql.DB, box, parent []byte, name string) []byte {
	t.Helper()
	id := randomID(t)
	if _, err := db.Exec(`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
	                      VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, box, parent, name, strings.ToLower(name), testTS, testTS); err != nil {
		t.Fatalf("insert folder: %v", err)
	}
	return id
}

func tryInsertFolder(db *sql.DB, box, parent []byte, name string) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := db.Exec(`INSERT INTO folders (id, box_id, parent_id, name, name_fold, created_at, updated_at)
	                   VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b, box, parent, name, strings.ToLower(name), testTS, testTS)
	return err
}

func insertTag(t *testing.T, db *sql.DB, parent *[]byte, name, fold string) []byte {
	t.Helper()
	id := randomID(t)
	var p any
	if parent != nil {
		p = *parent
	}
	if _, err := db.Exec(`INSERT INTO tags (id, parent_id, name, name_fold) VALUES (?, ?, ?, ?)`,
		id, p, name, fold); err != nil {
		t.Fatalf("insert tag: %v", err)
	}
	return id
}

func tryInsertTag(db *sql.DB, parent *[]byte, name, fold string) error {
	b := make([]byte, 16)
	rand.Read(b)
	var p any
	if parent != nil {
		p = *parent
	}
	_, err := db.Exec(`INSERT INTO tags (id, parent_id, name, name_fold) VALUES (?, ?, ?, ?)`, b, p, name, fold)
	return err
}

func insertBlob(t *testing.T, db *sql.DB, key string) string {
	t.Helper()
	digest := key[len("sha256:"):]
	if i := strings.IndexByte(digest, '.'); i >= 0 {
		digest = digest[:i]
	}
	if _, err := db.Exec(`INSERT INTO blobs (key, sha256, size) VALUES (?, ?, ?)`, key, digest, 1); err != nil {
		t.Fatalf("insert blob: %v", err)
	}
	return key
}

func insertFile(t *testing.T, db *sql.DB, box, folder []byte, name, key string) []byte {
	t.Helper()
	id := randomID(t)
	if _, err := db.Exec(`INSERT INTO files (id, box_id, folder_id, name, name_fold, blob_key, created_at, updated_at)
	                      VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, box, folder, name, strings.ToLower(name), key, testTS, testTS); err != nil {
		t.Fatalf("insert file: %v", err)
	}
	return id
}

func tryInsertFile(db *sql.DB, box, folder []byte, name, key string) error {
	b := make([]byte, 16)
	rand.Read(b)
	_, err := db.Exec(`INSERT INTO files (id, box_id, folder_id, name, name_fold, blob_key, created_at, updated_at)
	                   VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b, box, folder, name, strings.ToLower(name), key, testTS, testTS)
	return err
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/store/ -run TestMigrate -v`
Expected: FAIL，`undefined: Migrate`

- [ ] **Step 3: 写迁移 SQL**

`internal/store/migrations/00001_init.sql`：

```sql
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
```

- [ ] **Step 4: 写 Migrate**

`internal/store/migrate.go`:

```go
package store

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate brings the database up to the current schema. It is idempotent.
//
// The migrations are embedded rather than shipped alongside the binary: a
// single self-contained executable is the whole distribution promise, and a
// library that cannot open because its migration files went missing would be
// a spectacular way to break it.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
go get github.com/pressly/goose/v3
go mod tidy
git add go.mod go.sum internal/store/
git commit -m "feat(store): v0.1 全量 schema，约束用测试钉住

重点不是表建出来了，而是约束真的会咬——尤其 folders 的两条部分索引，
朴素的 UNIQUE(parent_id, name) 对根行不生效，正是 spec 1.4 自己
警告过的 NULL 坑。"
```

---

### Task 10: config — kist.toml

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: `domain.ID`、`blob.LinkMode`
- Produces: `config.Config`、`config.Default(linkMode blob.LinkMode) *Config`、`config.Load(path string) (*Config, error)`、`config.Save(path string, c *Config) error`、`config.CurrentVersion`

**`library.id` 现在就生成。** spec 3.12 说将来多设备同步时 op ID 要扩成 `(instance_id, seq)`。生成一个 UUIDv7 是零成本的，而将来给存量库补一个则要一次迁移加一轮"这个库是谁"的猜测。

**未知字段要报错而不是忽略。** 一个拼错的 key 被静默丢掉，用户会以为配置生效了。

- [ ] **Step 1: 写失败的测试**

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/blob"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.toml")
	want := Default(blob.LinkReflink)

	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Version != want.Version {
		t.Errorf("Version = %d, want %d", got.Version, want.Version)
	}
	if got.Library.ID != want.Library.ID {
		t.Errorf("Library.ID = %v, want %v", got.Library.ID, want.Library.ID)
	}
	if got.Storage.LinkMode != blob.LinkReflink {
		t.Errorf("Storage.LinkMode = %q, want %q", got.Storage.LinkMode, blob.LinkReflink)
	}
}

func TestDefaultGeneratesLibraryID(t *testing.T) {
	// spec 3.12: op ids become (instance_id, seq) once devices sync. Minting
	// the instance id now is free; backfilling it later is a migration plus a
	// guess about which library is which.
	a := Default(blob.LinkCopy)
	b := Default(blob.LinkCopy)

	if a.Library.ID.IsZero() {
		t.Error("Default did not generate a library id")
	}
	if a.Library.ID == b.Library.ID {
		t.Error("two libraries got the same id")
	}
}

func TestSaveIsHumanReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := Save(path, Default(blob.LinkHardlink)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[library]", "[storage]", "link_mode", "hardlink"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("config file does not contain %q:\n%s", want, raw)
		}
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	// A typo that gets silently dropped leaves the user believing a setting
	// took effect.
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := os.WriteFile(path, []byte("version = 1\n[storage]\nlink_moed = \"copy\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load accepted an unknown key")
	}
}

func TestLoadRejectsFutureVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := os.WriteFile(path, []byte("version = 99\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load accepted a config from the future")
	}
	if !strings.Contains(err.Error(), "newer") {
		t.Errorf("error = %q, want it to explain that a newer kist wrote this", err)
	}
}

func TestLoadRejectsUnknownLinkMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kist.toml")
	if err := os.WriteFile(path, []byte("version = 1\n[storage]\nlink_mode = \"teleport\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load accepted an unknown link mode")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/config/ -v`
Expected: FAIL，`undefined: Default`

- [ ] **Step 3: 实现**

```go
// Package config reads and writes kist.toml, the library's configuration.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/domain"
)

// CurrentVersion is the config schema this build writes.
const CurrentVersion = 1

type Config struct {
	Version int     `toml:"version"`
	Library Library `toml:"library"`
	Storage Storage `toml:"storage"`
}

type Library struct {
	// ID identifies this library. Nothing uses it in v0.1; it exists because
	// op ids become (instance_id, seq) when devices start syncing (spec 3.12),
	// and minting it at init is free while backfilling it later is not.
	ID domain.ID `toml:"id"`
}

type Storage struct {
	// LinkMode is what init probed. Rechecked at startup: an external drive
	// can come back mounted differently.
	LinkMode blob.LinkMode `toml:"link_mode"`
}

// Default builds the configuration for a fresh library.
func Default(linkMode blob.LinkMode) *Config {
	c := &Config{Version: CurrentVersion}
	c.Library.ID = domain.NewID()
	c.Storage.LinkMode = linkMode
	return c
}

func Load(path string) (*Config, error) {
	var c Config
	meta, err := toml.DecodeFile(path, &c)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("read %s: unknown setting %q", path, undecoded[0].String())
	}
	if c.Version > CurrentVersion {
		return nil, fmt.Errorf("read %s: config version %d was written by a newer kist; upgrade to open this library",
			path, c.Version)
	}
	switch c.Storage.LinkMode {
	case blob.LinkReflink, blob.LinkHardlink, blob.LinkSymlink, blob.LinkCopy, "":
	default:
		return nil, fmt.Errorf("read %s: unknown link_mode %q", path, c.Storage.LinkMode)
	}
	return &c, nil
}

func Save(path string, c *Config) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.WriteString("# kist library configuration.\n# Managed by kist; hand edits are read back on the next run.\n\n"); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
go get github.com/BurntSushi/toml
go mod tidy
git add go.mod go.sum internal/config/
git commit -m "feat(config): kist.toml，未知字段与未来版本一律报错"
```

---

### Task 11: library — 目录布局与建库

**Files:**
- Create: `internal/library/layout.go`, `internal/library/create.go`
- Test: `internal/library/layout_test.go`, `internal/library/create_test.go`

**Interfaces:**
- Consumes: `store.Open` / `store.Migrate`、`config.Default` / `config.Save` / `config.Load`、`blob.NewDir` / `blob.DetectLinkMode`
- Produces:
  - `library.Layout{Root string}`，方法 `DB()` / `ConfigPath()` / `Blobs()` / `Boxes()` / `Derived()` / `Notice()`
  - `library.Probe{LinkMode blob.LinkMode; FreeBytes uint64; FreeKnown bool}`
  - `library.Create(ctx context.Context, root string) (*CreateResult, error)`
  - `library.CreateResult{Layout Layout; Probe Probe; Config *config.Config}`
  - `library.ErrAlreadyLibrary`、`library.ErrNotEmpty`
  - `library.OpenLayout(root string) (Layout, error)`

**建库顺序有依赖：** 目录 → 探测链接能力（要 `blobs/` 和 `boxes/` 都存在）→ 建库 + 迁移 → 写配置。配置最后写，因为它要记录探测结果。

**已存在目录的处理分三种：** 不存在 → 建；存在且是个库 → `ErrAlreadyLibrary`；存在且非空但不是库 → `ErrNotEmpty`。**不往一个有别人东西的目录里铺库**——那会把用户的文件和 `blobs/` 混在一起，而 `kist relink` 将来会重建 `boxes/`。

- [ ] **Step 1: 写失败的测试**

```go
package library

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLayoutPaths(t *testing.T) {
	l := Layout{Root: filepath.Join("home", "kist")}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"DB", l.DB(), filepath.Join("home", "kist", "kist.db")},
		{"ConfigPath", l.ConfigPath(), filepath.Join("home", "kist", "kist.toml")},
		{"Blobs", l.Blobs(), filepath.Join("home", "kist", "blobs")},
		{"Boxes", l.Boxes(), filepath.Join("home", "kist", "boxes")},
		{"Derived", l.Derived(), filepath.Join("home", "kist", "derived")},
		{"Notice", l.Notice(), filepath.Join("home", "kist", "DO-NOT-EDIT.txt")},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s() = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestCreateBuildsTheWholeLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	res, err := Create(context.Background(), root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, p := range []string{
		res.Layout.Blobs(),
		filepath.Join(res.Layout.Blobs(), "tmp"),
		res.Layout.Boxes(),
		res.Layout.Derived(),
	} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Errorf("missing directory %s: %v", p, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s is not a directory", p)
		}
	}
	for _, p := range []string{res.Layout.DB(), res.Layout.ConfigPath(), res.Layout.Notice()} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing file %s: %v", p, err)
		}
	}
}

func TestCreateMigratesTheDatabase(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	res, err := Create(context.Background(), root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	db, err := store.Open(res.Layout.DB())
	if err != nil {
		t.Fatalf("open the created database: %v", err)
	}
	defer db.Close()

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='ops'`).Scan(&n); err != nil {
		t.Fatalf("query schema: %v", err)
	}
	if n != 1 {
		t.Error("the ops table is missing; migrations did not run")
	}
}

func TestCreateRecordsTheProbe(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	res, err := Create(context.Background(), root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if res.Probe.LinkMode == "" {
		t.Error("Create did not probe the link mode")
	}
	loaded, err := config.Load(res.Layout.ConfigPath())
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	if loaded.Storage.LinkMode != res.Probe.LinkMode {
		t.Errorf("config records %q but the probe found %q", loaded.Storage.LinkMode, res.Probe.LinkMode)
	}
}

func TestNoticeExplainsWhatNotToTouch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	res, err := Create(context.Background(), root)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	raw, err := os.ReadFile(res.Layout.Notice())
	if err != nil {
		t.Fatal(err)
	}
	// It has to name what is authoritative and what is regenerable, or it is
	// just decoration (spec 3.2).
	for _, want := range []string{"blobs/", "kist.db", "boxes/", "kist relink"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("DO-NOT-EDIT.txt does not mention %q", want)
		}
	}
}

func TestCreateRefusesAnExistingLibrary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := Create(context.Background(), root)
	if !errors.Is(err, ErrAlreadyLibrary) {
		t.Errorf("second Create returned %v, want ErrAlreadyLibrary", err)
	}
}

func TestCreateRefusesANonEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "holiday.jpg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Create(context.Background(), root)
	if !errors.Is(err, ErrNotEmpty) {
		t.Errorf("Create into an occupied directory returned %v, want ErrNotEmpty", err)
	}
}

func TestCreateAcceptsAnEmptyExistingDirectory(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(context.Background(), root); err != nil {
		t.Errorf("Create into an empty existing directory: %v", err)
	}
}

func TestOpenLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	if _, err := Create(context.Background(), root); err != nil {
		t.Fatalf("Create: %v", err)
	}

	l, err := OpenLayout(root)
	if err != nil {
		t.Fatalf("OpenLayout: %v", err)
	}
	if l.Root == "" {
		t.Error("OpenLayout returned an empty root")
	}

	if _, err := OpenLayout(t.TempDir()); err == nil {
		t.Error("OpenLayout accepted a directory that is not a library")
	}
}
```

（测试文件需要 import `errors`、`github.com/jinzhao1994/kist/internal/config`、`github.com/jinzhao1994/kist/internal/store`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/library/ -v`
Expected: FAIL，`undefined: Layout`

- [ ] **Step 3: 写 Layout**

```go
// Package library owns a kist library: its directory layout, and — from M1b
// onward — the only path through which anything is written (spec 2.6).
package library

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout locates everything inside a library root.
type Layout struct {
	Root string
}

func (l Layout) DB() string         { return filepath.Join(l.Root, "kist.db") }
func (l Layout) ConfigPath() string { return filepath.Join(l.Root, "kist.toml") }
func (l Layout) Blobs() string      { return filepath.Join(l.Root, "blobs") }
func (l Layout) Boxes() string      { return filepath.Join(l.Root, "boxes") }
func (l Layout) Derived() string    { return filepath.Join(l.Root, "derived") }
func (l Layout) Notice() string     { return filepath.Join(l.Root, "DO-NOT-EDIT.txt") }

// OpenLayout resolves root and checks it actually holds a library.
func OpenLayout(root string) (Layout, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve library path: %w", err)
	}
	l := Layout{Root: abs}
	if _, err := os.Stat(l.ConfigPath()); err != nil {
		return Layout{}, fmt.Errorf("%s is not a kist library (no kist.toml)", abs)
	}
	return l, nil
}

// isLibrary reports whether root already holds one.
func isLibrary(root string) bool {
	_, err := os.Stat(filepath.Join(root, "kist.toml"))
	return err == nil
}
```

- [ ] **Step 4: 写 Create**

```go
package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/config"
	"github.com/jinzhao1994/kist/internal/store"
)

var (
	// ErrAlreadyLibrary means root already holds a kist library.
	ErrAlreadyLibrary = errors.New("already a kist library")
	// ErrNotEmpty means root holds something else. kist will not lay a library
	// over a user's files: boxes/ gets rebuilt wholesale by `kist relink`, and
	// mixing the two would put that rebuild next to things it does not own.
	ErrNotEmpty = errors.New("directory is not empty")
)

// Probe is what init found out about the chosen location.
type Probe struct {
	LinkMode  blob.LinkMode
	FreeBytes uint64
	FreeKnown bool
}

// CreateResult describes the library that was just built.
type CreateResult struct {
	Layout Layout
	Probe  Probe
	Config *config.Config
}

// Create builds a new library at root.
//
// The order matters: directories first, then probe (which needs both blobs/
// and boxes/ to exist), then the database, then the config — which records
// what the probe found.
func Create(ctx context.Context, root string) (*CreateResult, error) {
	l, err := prepareRoot(root)
	if err != nil {
		return nil, err
	}

	for _, dir := range []string{l.Blobs(), l.Boxes(), l.Derived()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create library layout: %w", err)
		}
	}
	if _, err := blob.NewDir(l.Blobs(), slog.Default()); err != nil {
		return nil, err
	}

	mode, err := blob.DetectLinkMode(l.Blobs(), l.Boxes())
	if err != nil {
		return nil, fmt.Errorf("probe link capability: %w", err)
	}
	free, freeKnown := freeSpace(l.Root)

	db, err := store.Open(l.DB())
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return nil, err
	}

	if err := os.WriteFile(l.Notice(), []byte(noticeText), 0o644); err != nil {
		return nil, fmt.Errorf("write notice: %w", err)
	}

	cfg := config.Default(mode)
	if err := config.Save(l.ConfigPath(), cfg); err != nil {
		return nil, err
	}

	return &CreateResult{
		Layout: l,
		Probe:  Probe{LinkMode: mode, FreeBytes: free, FreeKnown: freeKnown},
		Config: cfg,
	}, nil
}

func prepareRoot(root string) (Layout, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve library path: %w", err)
	}
	l := Layout{Root: abs}

	entries, err := os.ReadDir(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return Layout{}, fmt.Errorf("create library directory: %w", err)
		}
	case err != nil:
		return Layout{}, fmt.Errorf("inspect %s: %w", abs, err)
	case isLibrary(abs):
		return Layout{}, fmt.Errorf("%s: %w", abs, ErrAlreadyLibrary)
	case len(entries) > 0:
		return Layout{}, fmt.Errorf("%s: %w", abs, ErrNotEmpty)
	}
	return l, nil
}

const noticeText = `This directory is a kist library.

WHAT IS AUTHORITATIVE — do not edit, move, or rename:

  blobs/     your files, stored by content hash and kept read-only
  kist.db    every operation ever performed on this library

WHAT IS A PROJECTION — safe to browse, pointless to edit:

  boxes/     a human-readable view of blobs/. kist rebuilds it wholesale, so
             changes made here are not read back and will be overwritten.
             If it looks wrong, run: kist relink
  derived/   thumbnails and previews, regenerated on demand

TO BACK THIS UP:

  rsync -a --exclude boxes/ --exclude derived/ <this directory> <destination>

boxes/ and derived/ are both reconstructible from the two authoritative
things above, so backing them up only costs space.
`
```

`internal/library/diskfree_unix.go`:

```go
//go:build !windows

package library

import "golang.org/x/sys/unix"

// freeSpace reports the bytes available to an unprivileged user. Failure is
// not fatal: init prints "unknown" and carries on.
func freeSpace(path string) (uint64, bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, false
	}
	return st.Bavail * uint64(st.Bsize), true
}
```

`internal/library/diskfree_windows.go`:

```go
//go:build windows

package library

import "golang.org/x/sys/windows"

func freeSpace(path string) (uint64, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0, false
	}
	return free, true
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/library/ -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/library/
git commit -m "feat(library): 目录布局与建库，拒绝铺在非空目录上"
```

---

### Task 12: cmd — `kist init`

**Files:**
- Create: `internal/cliutil/prompt.go`, `cmd/kist/init.go`
- Modify: `cmd/kist/root.go`（注册 init 子命令）
- Test: `internal/cliutil/prompt_test.go`, `cmd/kist/init_test.go`

**Interfaces:**
- Consumes: `library.Create` / `library.ErrAlreadyLibrary` / `library.ErrNotEmpty`、`blob.LinkMode`
- Produces: `cliutil.Prompter`、`cliutil.NewPrompter(in io.Reader, out io.Writer, auto bool) *Prompter`、方法 `Ask(question, def string) (string, error)` / `Confirm(question string, def bool) (bool, error)`；`cmd/kist` 的 `newInitCmd()`

**交互与非交互必须是同一条代码路径**（spec 3.1）。做法是 `Prompter` 带一个 `auto` 开关：打开时每个问题直接返回默认值，不读 stdin。`--yes` 和"stdin 不是终端"都只是把这个开关打开——于是测试覆盖到的就是用户真正走的那条路。

默认路径 `~/kist`，一路回车即可。

- [ ] **Step 1: 写 Prompter 的失败测试**

```go
package cliutil

import (
	"bytes"
	"strings"
	"testing"
)

func TestAskReturnsTheAnswer(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("/mnt/archive\n"), &out, false)

	got, err := p.Ask("Library location", "~/kist")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "/mnt/archive" {
		t.Errorf("Ask = %q, want %q", got, "/mnt/archive")
	}
	if !strings.Contains(out.String(), "~/kist") {
		t.Errorf("prompt %q does not show the default", out.String())
	}
}

func TestAskFallsBackToDefaultOnBareEnter(t *testing.T) {
	p := NewPrompter(strings.NewReader("\n"), &bytes.Buffer{}, false)

	got, err := p.Ask("Library location", "~/kist")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "~/kist" {
		t.Errorf("Ask = %q, want the default %q", got, "~/kist")
	}
}

func TestAskTrimsWhitespace(t *testing.T) {
	p := NewPrompter(strings.NewReader("  /mnt/archive  \n"), &bytes.Buffer{}, false)
	got, _ := p.Ask("Library location", "~/kist")
	if got != "/mnt/archive" {
		t.Errorf("Ask = %q, want it trimmed", got)
	}
}

func TestAutoModeNeverReadsInput(t *testing.T) {
	// --yes and a non-terminal stdin both land here. It must be the same code
	// path the interactive user takes, or the tested path is not the shipped
	// one (spec 3.1).
	in := strings.NewReader("this must not be consumed\n")
	p := NewPrompter(in, &bytes.Buffer{}, true)

	got, err := p.Ask("Library location", "~/kist")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got != "~/kist" {
		t.Errorf("Ask = %q, want the default %q", got, "~/kist")
	}
	if in.Len() == 0 {
		t.Error("auto mode consumed stdin")
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		input string
		def   bool
		want  bool
	}{
		{"y\n", false, true},
		{"Y\n", false, true},
		{"yes\n", false, true},
		{"n\n", true, false},
		{"no\n", true, false},
		{"\n", true, true},   // bare enter takes the default
		{"\n", false, false},
	}
	for _, tt := range tests {
		p := NewPrompter(strings.NewReader(tt.input), &bytes.Buffer{}, false)
		got, err := p.Confirm("Create it?", tt.def)
		if err != nil {
			t.Fatalf("Confirm(%q): %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("Confirm(input=%q, default=%v) = %v, want %v", tt.input, tt.def, got, tt.want)
		}
	}
}

func TestConfirmRejectsGibberishUntilItGetsAnAnswer(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("maybe\nsure\ny\n"), &out, false)

	got, err := p.Confirm("Create it?", false)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if !got {
		t.Error("Confirm = false, want true from the final y")
	}
}

func TestAskFailsOnClosedInput(t *testing.T) {
	p := NewPrompter(strings.NewReader(""), &bytes.Buffer{}, false)
	if _, err := p.Ask("Library location", "~/kist"); err == nil {
		t.Error("Ask on empty input = nil error, want a failure")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/cliutil/ -v`
Expected: FAIL，`undefined: NewPrompter`

- [ ] **Step 3: 实现 Prompter**

```go
// Package cliutil holds the pieces of the command line that are worth testing
// on their own.
package cliutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Prompter asks questions with defaults, in the style of ssh-keygen: every
// question has an answer ready, so holding down Enter completes the flow.
//
// auto makes every question return its default without touching the input.
// Both --yes and a non-terminal stdin set it, which keeps the scripted path
// and the interactive path the same code.
type Prompter struct {
	in   *bufio.Reader
	out  io.Writer
	auto bool
}

func NewPrompter(in io.Reader, out io.Writer, auto bool) *Prompter {
	return &Prompter{in: bufio.NewReader(in), out: out, auto: auto}
}

// Auto reports whether this prompter answers on the user's behalf.
func (p *Prompter) Auto() bool { return p.auto }

// Printf writes to the prompter's output.
func (p *Prompter) Printf(format string, args ...any) {
	fmt.Fprintf(p.out, format, args...)
}

// Ask asks for a value, returning def when the user just presses Enter.
func (p *Prompter) Ask(question, def string) (string, error) {
	if p.auto {
		return def, nil
	}
	fmt.Fprintf(p.out, "%s [%s]: ", question, def)

	line, err := p.in.ReadString('\n')
	if err != nil && (line == "" || !errors.Is(err, io.EOF)) {
		return "", fmt.Errorf("read answer: %w", err)
	}
	if answer := strings.TrimSpace(line); answer != "" {
		return answer, nil
	}
	return def, nil
}

// Confirm asks a yes/no question, repeating until it gets one of them.
func (p *Prompter) Confirm(question string, def bool) (bool, error) {
	if p.auto {
		return def, nil
	}
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	for {
		fmt.Fprintf(p.out, "%s [%s]: ", question, hint)

		line, err := p.in.ReadString('\n')
		if err != nil && (line == "" || !errors.Is(err, io.EOF)) {
			return false, fmt.Errorf("read answer: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(p.out, "Please answer y or n.")
		}
	}
}
```

- [ ] **Step 4: 写 `kist init` 的失败测试**

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesALibraryNonInteractively(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	out, err := exec(t, "init", root, "--yes")
	if err != nil {
		t.Fatalf("kist init: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "kist.toml")); err != nil {
		t.Errorf("no library at %s: %v", root, err)
	}
	// The summary has to name the link mode: "this storage cannot deduplicate,
	// boxes/ will cost an extra N GB" is something the user must see (spec 3.5).
	if !strings.Contains(out, "Link mode") {
		t.Errorf("init output does not report the link mode:\n%s", out)
	}
}

func TestInitInteractiveAcceptsAPathAndConfirmation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	out, err := execStdin(t, root+"\ny\n", "init")
	if err != nil {
		t.Fatalf("kist init: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "kist.toml")); err != nil {
		t.Errorf("no library at %s: %v", root, err)
	}
}

func TestInitAbortsWhenTheUserDeclines(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	out, err := execStdin(t, root+"\nn\n", "init")
	if err != nil {
		t.Fatalf("declining should not be an error: %v\n%s", err, out)
	}
	if _, err := os.Stat(root); err == nil {
		t.Error("the library was created even though the user said no")
	}
}

func TestInitRefusesAnExistingLibrary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")
	if _, err := exec(t, "init", root, "--yes"); err != nil {
		t.Fatalf("first init: %v", err)
	}

	_, err := exec(t, "init", root, "--yes")
	if err == nil {
		t.Fatal("second init = nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), "already") {
		t.Errorf("error = %q, want it to say the library already exists", err)
	}
}

func TestInitShowsWhereItWillGoBeforeAsking(t *testing.T) {
	root := filepath.Join(t.TempDir(), "archive")

	// Declining still shows the location and the space — displaying them is
	// the entire reason for the confirmation step (spec 3.1). The link mode
	// cannot appear here: probing it needs the directories to exist.
	out, err := execStdin(t, root+"\nn\n", "init")
	if err != nil {
		t.Fatalf("kist init: %v", err)
	}
	for _, want := range []string{"Location", "Free space"} {
		if !strings.Contains(out, want) {
			t.Errorf("init did not show %q before asking:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Link mode") {
		t.Errorf("init claimed a link mode without having created anything:\n%s", out)
	}
}
```

在 `cmd/kist/main_test.go` 里加一个能喂 stdin 的辅助函数：

```go
// execStdin runs the root command with args, feeding stdin from a string.
func execStdin(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)

	err := root.Execute()
	return out.String(), err
}
```

- [ ] **Step 5: 跑测试确认失败**

Run: `go test ./cmd/kist/ -run TestInit -v`
Expected: FAIL，`unknown command "init"`

- [ ] **Step 6: 实现 `kist init`**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jinzhao1994/kist/internal/blob"
	"github.com/jinzhao1994/kist/internal/cliutil"
	"github.com/jinzhao1994/kist/internal/library"
)

func newInitCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Create a new library",
		Long: `Create a new kist library.

Run without arguments to be asked where it should go; every question has a
default, so holding down Enter is a complete answer.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var given string
			if len(args) == 1 {
				given = args[0]
			}
			return runInit(cmd, given, yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "accept every default without asking")
	return cmd
}

func runInit(cmd *cobra.Command, given string, yes bool) error {
	// Answering on the user's behalf when there is nobody to ask: a piped or
	// redirected stdin would otherwise block forever waiting for a newline.
	auto := yes || !isTerminal(cmd.InOrStdin())
	p := cliutil.NewPrompter(cmd.InOrStdin(), cmd.OutOrStdout(), auto)

	path := given
	if path == "" {
		def, err := defaultLibraryPath()
		if err != nil {
			return err
		}
		path, err = p.Ask("Library location", def)
		if err != nil {
			return err
		}
	}
	path, err := expandHome(path)
	if err != nil {
		return err
	}

	// Show what is known before creating anything: a few hundred gigabytes
	// landing on the system disk is a real trap (spec 3.1). The link mode is
	// not knowable yet — probing it needs the directories to exist — so it is
	// reported afterwards.
	free, freeKnown := library.FreeSpaceAt(nearestExistingDir(path))

	p.Printf("\n  Location   %s\n", path)
	if freeKnown {
		p.Printf("  Free space %s\n", humanBytes(free))
	} else {
		p.Printf("  Free space unknown\n")
	}
	p.Printf("\n")

	create, err := p.Confirm("Create the library here?", true)
	if err != nil {
		return err
	}
	if !create {
		p.Printf("Nothing was created.\n")
		return nil
	}

	res, err := library.Create(cmd.Context(), path)
	if err != nil {
		return err
	}

	p.Printf("  Link mode  %s — %s\n\n", res.Probe.LinkMode, linkModeExplanation(res.Probe.LinkMode))
	p.Printf("Library created at %s.\n", res.Layout.Root)
	p.Printf("Nothing is in it yet; `kist box new` and `kist import` come with M1b.\n")
	return nil
}

// nearestExistingDir walks up until it finds a directory that exists, so that
// free space can be measured for a library path that has not been created yet.
func nearestExistingDir(path string) string {
	for {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}

func linkModeExplanation(m blob.LinkMode) string {
	switch m {
	case blob.LinkReflink:
		return "boxes/ shares storage with blobs/ and costs almost nothing"
	case blob.LinkHardlink:
		return "boxes/ shares storage with blobs/ and costs almost nothing"
	case blob.LinkSymlink:
		return "boxes/ holds symlinks; some tools will not follow them"
	case blob.LinkCopy:
		return "this storage cannot deduplicate, so boxes/ will double the space used"
	default:
		return "unknown"
	}
}

func defaultLibraryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, "kist"), nil
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~"+string(os.PathSeparator)) && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, strings.TrimPrefix(path[1:], "/")), nil
}

func isTerminal(r any) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
```

- [ ] **Step 7: 在 `internal/library/create.go` 里导出空间查询**

```go
// FreeSpaceAt reports the bytes available at path, and whether that could be
// determined at all. init shows this before creating anything, so it has to be
// callable without a library.
func FreeSpaceAt(path string) (uint64, bool) { return freeSpace(path) }
```

- [ ] **Step 8: 注册子命令**

在 `cmd/kist/root.go` 的 `newRootCmd` 里，`root.AddCommand(newVersionCmd())` 旁边加上：

```go
	root.AddCommand(newInitCmd())
```

- [ ] **Step 9: 跑全部测试**

Run: `make check`
Expected: PASS

- [ ] **Step 10: 手工验证交互流程**

```bash
go run ./cmd/kist init
```
一路回车，确认：提问带默认值 `~/kist`、创建前显示位置与可用空间、创建后报告链接方式。然后 `ls -la ~/kist` 核对布局，`rm -rf ~/kist` 清掉。

- [ ] **Step 11: 更新 README 与 AGENTS.md**

`README.md` 的状态段落从"`kist version` is the only command that exists"改为说明 `kist init` 可用但库里还不能放东西；`README.zh-CN.md` 同步。`AGENTS.md` 的"当前状态"改成 M1a 完成、下一步 M1b。

- [ ] **Step 12: 提交**

```bash
go get golang.org/x/term
go mod tidy
git add .
git commit -m "feat(cli): kist init，交互与非交互同一条代码路径"
```

---

## 自查

**Spec 覆盖：** M1a 认领的范围是 spec 1.3 / 1.6（类型）、3.1 / 3.3 / 3.5 / 3.10（blob 与布局）、4.7（折叠列）的一部分。逐条对照：

| spec | 落在哪 |
|---|---|
| 1.3 变精度日期 | Task 2 |
| 1.4 / 1.5 / 1.6 实体与约束 | Task 9（schema + 约束测试）；实体的读写在 M1b |
| 3.1 磁盘布局、init 流程 | Task 11、12 |
| 3.3 blob 层 | Task 4、5、6 |
| 3.5 链接能力探测 | Task 7 |
| 3.10 BlobStore 接口 | Task 6 |
| 3.12 ID 方案 | Task 1（UUIDv7）、Task 9（ops.id AUTOINCREMENT） |
| 4.6 索引清单 | Task 9 |
| 4.7 折叠列 | Task 3（函数）、Task 9（列） |

**明确不在 M1a 的：** op log 的一切（2.x 全节）、`library` 的写入口、`boxes/` 投影的增量维护（3.4.1）、导入管线（3.7）、Query AST 与检索（4.2–4.5）、`verify` / `scrub` / `rebuild`（3.8，属 M2）、metadata 的实际写入（1.7，M1b 的导入才产生 `file:size` / `file:mtime`）。这些都在 M1b 或更后。

**跨 task 的类型一致性：** `blob.Key` 在 Task 4 定义、5/6/7 使用；`blob.LinkMode` 在 Task 7 定义、10/11/12 使用；`domain.ID` 在 Task 1 定义、10 使用；`library.Layout` 在 Task 11 定义、12 使用；`cliutil.Prompter` 在 Task 12 内定义并使用。`compareAndHash` 是 `blob` 包内私有，只有 Task 6 调它。

**执行顺序上的一条依赖：** Task 10（config）import 了 `blob.LinkMode`，Task 11（library）import 了三个包，所以 Task 1–9 必须在它们之前完成。1–3（domain）、4–7（blob）、8–9（store）三组之间没有依赖，可以并行。

**一处刻意的取舍：** `Dir.putWithHash` 是生产代码里只被测试调用的方法。留着是因为**碰撞分支没有别的办法触达**——真造一个 SHA-256 碰撞是不可能的，而那个分支恰恰是 spec 3.3 花最多篇幅论证的一条。不用 `export_test.go` 是因为它需要的不是"导出私有符号"，而是"绕过哈希计算"，那是一条真实存在的替代入口。
