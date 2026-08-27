# M2b Query 契约与 conformance suite 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 把 4.2 的 `Query` 从一段设计文档里的伪代码变成**真的契约**——一个不含 SQL 的结构、一份任何实现都必须通过的 conformance suite、以及 `ls` / `find` 迁到它上面。**产出：DAL 的契约定死了**，v0.2 写 Postgres 实现时有现成的验收标准。

**Architecture:** 一个新包 `internal/query` 拥有 AST 和游标，**只依赖 `domain`**——`cmd` 要构造 Query，而 `cmd` 不许 import `store`（不变量 2），所以契约不能住在 `store` 里。这和 `oplog` 是契约、`store` 结构化地满足它是同一个形状。

`store` 是**唯一的实现**：一个把 `query.Query` 翻译成 SQL 的函数。翻译是纯的——给定 Query 出同一串 SQL 和同一组参数——所以它可以被单独测，而不用开库。

conformance suite 住在 `internal/query/querytest`，导出 `Run(t, newBackend)`。它是 Go 里 `testing/fstest` 那个形状：一个包，import `testing`，被实现方的测试调用。**v0.1 只有 SQLite 一个 backend 调它**，但套件本身照写不误——那正是 4.2 说「这部分工作量不该省」的意思。

**tag 后代展开不在 `query` 里，也不在 `store` 里。** 4.3 定的两步走：第一步是 `library` 的内存树遍历，第二步才是 `IN (…)`。所以 `TagExpr` 的叶子拿到的已经是**展开好的 ID 集合**。

**Tech Stack:** 不新增依赖。

**Spec:** `docs/design/2026-08-22-kist-v0.1-design.md`，**4.2 / 4.2.1 是本计划的正文**；4.3（tag 语义）、4.4（LIKE 全扫）、4.5 / 4.5.1（keyset 与 NULL）、4.6（索引）、4.7（引擎分歧）、4.8（字节序）都要照着。

**前置：** M1 + M2a 全部完成。M2b **不需要任何 migration**——4.2.1 砍掉 `Kinds` 和 `size` 排序之后，现有 schema 够用。

## Global Constraints

- **Go 1.26.1**；**`CGO_ENABLED=0` 四平台可构建**
- **不变量 2：`cmd` 不得 import `store` / `oplog`。** 这条直接决定了 `query` 必须是独立的包
- **不变量 4** 不受影响：M2b 不碰 payload
- **排序一律字节序**（4.8）；**可空列的排序必须显式写 `NULLS LAST` / `NULLS FIRST`**（4.5.1、4.7）——SQLite 上加不加输出相同，本地永远测不出来
- **`LIKE` 一律走 Go 生成的折叠列**（4.7），模式串用**同一个** `domain.Fold` 处理
- **语言：代码、注释、godoc 用英文；设计文档与 commit message 用中文**
- 每个 task 结束时 `make check` 必须通过

---

## 四条要一直记着的原则

### ① 契约里不出现 SQL

4.1 说得很直接：「正确的抽象方式不是包一层通用 SQL，而是接口里不出现 SQL，只出现领域语义。」`internal/query` 里一个字符串拼接都不该有。

### ② 空集合的语义要定死，而且要测

`TagAnd{}`、`TagOr{}`、`TagAny{}` 各是什么意思？标准代数答案是「全真 / 全假 / 全假」，但**不写下来就会有人凭直觉写反**，而写反的表现是筛选静默地多给或少给。这是 conformance suite 最该覆盖的那种东西。

### ③ 游标对不上就报错，不是尽力而为

4.2.1 定的：`cursor = base64(sortID ␀ sortKey ␀ id)`。拿 A 排序的游标去请求 B 排序，**返回一个 error**。尽力而为的版本会给出一页看起来正常、实际乱序的结果，而那种 bug 没人会发现。

### ④ conformance suite 断言的是**结果**，不是 SQL

套件里不许出现 `EXPLAIN`、不许断言查询计划、不许知道有没有索引。它只问「给这批数据和这个 Query，回来的应该是哪几行、什么顺序」。否则它就不是契约，是 SQLite 的回归测试。

---

## 文件结构

| 文件 | 职责 |
|---|---|
| `internal/query/query.go`（新） | `Query` / `SortSpec` / `DateRange` 与校验 |
| `internal/query/tagexpr.go`（新） | `TagExpr` 及其四个构造子 |
| `internal/query/cursor.go`（新） | 游标编解码，含排序标识 |
| `internal/query/querytest/suite.go`（新） | conformance suite：`Backend`、`Fixture`、`Run` |
| `internal/query/querytest/cases.go`（新） | 全部 case，按 4.2.1 的字段分组 |
| `internal/store/translate.go`（新） | `query.Query` → SQL + args，纯函数 |
| `internal/store/files.go` | `ListFiles` 换成收 `query.Query` |
| `internal/store/conformance_test.go`（新） | 把 SQLite 接进套件 |
| `internal/library/files.go` | `Files` 收 `query.Query`；tag 展开留在这里 |
| `cmd/kist/find.go` | `--sort` / `--since` / `--until` |
| `cmd/kist/ls.go` | 迁到 `query.Query` |
| `internal/arch/arch_test.go` | 新增：`query` 只许依赖 `domain` |

---

### Task 1: `internal/query` 的 AST

**Files:**
- Create: `internal/query/query.go`, `internal/query/tagexpr.go`, `internal/query/query_test.go`
- Modify: `internal/arch/arch_test.go`

**Interfaces:**
- Consumes: `internal/domain` —— **只有它**
- Produces: `query.Query`、`query.SortKey`（`SortCaptured` / `SortImported` / `SortName`）、`query.SortSpec`、`query.DateRange`、`query.TagExpr` 与 `TagAny` / `TagAll` / `TagAnyOf` / `TagNot`、`(Query).Validate() error`

先把结构定死，再谈翻译。这一步不碰数据库，也不该碰。

- [x] **Step 1: 写失败的测试**

`internal/query/query_test.go`：

```go
package query

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

// The zero Query is "everything, newest first". A query type whose zero value
// means nothing would make every caller write boilerplate before it could ask
// the simplest question there is.
func TestZeroQueryIsEverythingNewestFirst(t *testing.T) {
	var q Query
	if err := q.Validate(); err != nil {
		t.Fatalf("the zero query is invalid: %v", err)
	}
	if q.Sort.Key != SortCaptured || !q.Sort.Desc {
		t.Errorf("default sort = %+v, want captured descending", q.Sort)
	}
}

// A limit has to be bounded somewhere, or one request can ask for a hundred
// thousand rows and the answer arrives as a memory spike.
func TestValidateBoundsTheLimit(t *testing.T) {
	q := Query{Limit: MaxLimit + 1}
	if err := q.Validate(); err == nil {
		t.Errorf("a limit of %d was accepted", q.Limit)
	}
	if err := (Query{Limit: -1}).Validate(); err == nil {
		t.Error("a negative limit was accepted")
	}
}

// A date range whose end is before its start is a mistake, and returning zero
// rows for it looks exactly like "you have nothing from then".
func TestValidateRejectsABackwardsRange(t *testing.T) {
	from, _ := domain.ParseLocalDate("2019-06")
	to, _ := domain.ParseLocalDate("2019-03")
	if err := (Query{Time: &DateRange{From: from, To: to}}).Validate(); err == nil {
		t.Error("a range ending before it starts was accepted")
	}
}

// An unknown sort key must be refused rather than silently treated as the
// default: a caller asking for something we do not have wants to hear so.
func TestValidateRejectsAnUnknownSortKey(t *testing.T) {
	if err := (Query{Sort: SortSpec{Key: SortKey(99)}}).Validate(); err == nil {
		t.Error("an unknown sort key was accepted")
	}
}
```

`internal/query/tagexpr_test.go`：

```go
package query

import "testing"

// The empty-set semantics, written down because the standard algebraic answer
// is exactly the thing people get backwards when writing the translator: an
// empty AND is the identity (matches everything), an empty OR or an empty leaf
// matches nothing.
//
// Getting one of these wrong does not error — it silently widens or narrows
// every query built by a caller that happened to pass no tags, which is the
// common case for "the user did not tick anything".
func TestEmptyExpressionsHaveDefinedMeanings(t *testing.T) {
	cases := []struct {
		name string
		expr TagExpr
		want bool // matches everything?
	}{
		{"an empty AND is the identity", TagAll{}, true},
		{"an empty OR matches nothing", TagAnyOf{}, false},
		{"an empty leaf matches nothing", TagAny{}, false},
	}
	for _, c := range cases {
		if got := MatchesEverything(c.expr); got != c.want {
			t.Errorf("%s: MatchesEverything = %v, want %v", c.name, got, c.want)
		}
	}
}

// nil is not an expression; it is the absence of one, and it must not be
// confused with "an expression that matches nothing".
func TestNilExpressionIsNoFilter(t *testing.T) {
	if !MatchesEverything(nil) {
		t.Error("a nil TagExpr filtered something out")
	}
}

// Nesting has to survive being written down. This is the property the whole
// tree exists for, and v0.1 has no UI that produces one — the conformance
// suite is its only exerciser (spec 4.3).
func TestNestingComposes(t *testing.T) {
	e := TagAll{
		TagAny{tagID(t, 1)},
		TagAnyOf{TagAny{tagID(t, 2)}, TagAny{tagID(t, 3)}},
		TagNot{TagAny{tagID(t, 4)}},
	}
	if MatchesEverything(e) {
		t.Error("a tree with real conditions was treated as no filter")
	}
	if got := Leaves(e); len(got) != 4 {
		t.Errorf("walked %d leaves, want 4", len(got))
	}
}
```

`tagID` 是个测试小工具，用 `domain.NewTagID()` 造几个不同的 ID 即可。

- [x] **Step 2: 跑一遍确认它红**

```
go test ./internal/query/
```

Expected: FAIL，包不存在。

- [x] **Step 3: 实现 `tagexpr.go`**

```go
package query

import "github.com/jinzhao1994/kist/internal/domain"

// TagExpr is a boolean condition over the tags a file carries.
//
// The leaves hold tag ids that are already expanded: spec 4.3's step one is a
// walk of the in-memory tag tree, done by library before a query is built, and
// step two is the id set arriving here. Putting the expansion behind this
// interface would drag the tag tree into every implementation of the contract.
//
// v0.1 has no UI that produces anything but "several tags intersected plus an
// exclusion list" (spec 4.3). The tree exists anyway because this is the
// contract, and a contract that grows a case later is a contract its
// conformance suite was not written against. The suite is its only exerciser,
// and that is the point rather than a gap.
type TagExpr interface {
	// isTagExpr keeps the set of cases closed. An implementation of the
	// contract can then translate by type switch and be sure it is exhaustive.
	isTagExpr()
}

// TagAny is the leaf: the file carries at least one of these tags.
//
// An empty TagAny matches nothing. That is the algebra — the disjunction of no
// alternatives is false — and it is the answer that makes "the user selected
// no tags" fail closed rather than quietly select everything.
type TagAny []domain.TagID

// TagAll matches when every child does. Empty matches everything: it is the
// identity of AND, and it is what "no conditions" has to mean.
type TagAll []TagExpr

// TagAnyOf matches when any child does. Empty matches nothing.
type TagAnyOf []TagExpr

// TagNot inverts its child.
type TagNot struct{ Expr TagExpr }

func (TagAny) isTagExpr()   {}
func (TagAll) isTagExpr()   {}
func (TagAnyOf) isTagExpr() {}
func (TagNot) isTagExpr()   {}

// MatchesEverything reports whether an expression filters nothing out, so a
// caller can skip building a predicate for it.
//
// It is deliberately conservative: it recognises nil and the identity forms,
// and says false for anything else rather than trying to prove tautologies.
func MatchesEverything(e TagExpr) bool {
	switch v := e.(type) {
	case nil:
		return true
	case TagAll:
		for _, child := range v {
			if !MatchesEverything(child) {
				return false
			}
		}
		return true
	}
	return false
}

// Leaves returns every TagAny in the tree, so a caller can check id counts
// against an engine's parameter limit (spec 4.3 records SQLite's 32766).
func Leaves(e TagExpr) []TagAny {
	var out []TagAny
	var walk func(TagExpr)
	walk = func(e TagExpr) {
		switch v := e.(type) {
		case TagAny:
			out = append(out, v)
		case TagAll:
			for _, c := range v {
				walk(c)
			}
		case TagAnyOf:
			for _, c := range v {
				walk(c)
			}
		case TagNot:
			walk(v.Expr)
		}
	}
	walk(e)
	return out
}
```

- [x] **Step 4: 实现 `query.go`**

```go
// Package query is the data access layer's contract.
//
// It holds no SQL and knows nothing about any engine (spec 4.1): a Query says
// what is wanted in the vocabulary of boxes, folders, tags, time and text, and
// each implementation translates it into whatever it is good at.
//
// It lives in its own package rather than in store because cmd builds queries
// and cmd may not import store (invariant 2). That is the same shape oplog
// has: the contract is one package, the thing that satisfies it is another.
package query

import (
	"fmt"

	"github.com/jinzhao1994/kist/internal/domain"
)

// MaxLimit bounds one page. A caller that wants everything pages for it; a
// caller that asks for a hundred thousand rows at once gets told no, because
// the alternative is a memory spike an implementation cannot defend against.
const MaxLimit = 1000

// DefaultLimit is what an unset Limit means.
const DefaultLimit = 100

// SortKey names an ordering. Byte order throughout (spec 4.8).
type SortKey int

const (
	// SortCaptured orders by when the content was made. Files with no known
	// capture time sort last in both directions — unknown is unknown, and it
	// belongs at the end whichever way you are reading (spec 4.5.1).
	SortCaptured SortKey = iota
	// SortImported orders by when the file entered the library. It is the id:
	// ids are UUIDv7 and are minted when the op is generated, so id order is
	// creation order for free (spec 4.5.1, 3.12).
	SortImported
	// SortName orders by the folded name, so "Apple" and "apple" sort
	// together, in byte order (spec 4.8).
	SortName

	numSortKeys
)

// SortSpec is an ordering and a direction.
type SortSpec struct {
	Key  SortKey
	Desc bool
}

// DateRange is a closed interval. Either end may be the zero LocalDate,
// meaning open.
//
// Both ends and the values being matched are variable-precision (spec 1.3), so
// matching is overlap rather than containment: a box known only to be "2019"
// overlaps "spring 2019" and should be found. For an archive whose whole
// purpose is finding your things, a false positive beats a false negative
// (spec 4.2.1).
type DateRange struct {
	From domain.LocalDate
	To   domain.LocalDate
}

// Query is one question about the files in a library.
//
// The zero value is a valid query: everything, newest first. Kinds is
// deliberately absent — see spec 4.2.1 for why it waits for M3.
type Query struct {
	// Boxes restricts to these boxes. Empty means all of them.
	Boxes []domain.BoxID
	// Folder restricts to one directory, without recursing. It is what
	// `kist ls` asks for.
	Folder *domain.FolderID
	// Tags is the tag condition. nil means no tag filtering.
	Tags TagExpr
	// Time restricts by capture time for files, and by the box's own range for
	// boxes. A file with no capture time is not in any time range: nothing is
	// known about when it was made, and a time filter is an explicit
	// narrowing. Before M3 no file has one, so a time-filtered file query is
	// empty — which is the truth rather than a defect (spec 4.2.1).
	Time *DateRange
	// Text is matched as a folded substring of the name (spec 4.4). The
	// implementation folds it with the same function that built the column, so
	// case-insensitivity is ours rather than the engine's (spec 4.7).
	Text string
	Sort SortSpec
	// Cursor continues a previous page. It carries the sort it was made under,
	// and an implementation must refuse a cursor from a different one
	// (spec 4.2.1).
	Cursor string
	Limit  int
}

// Validate reports whether this query is answerable at all.
//
// It is the implementation's first line: every backend calls it, so a bad
// query fails the same way everywhere. That is part of the contract too — a
// conformance suite that only pins successes would let two implementations
// disagree about what an error is.
func (q Query) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("limit %d is negative", q.Limit)
	}
	if q.Limit > MaxLimit {
		return fmt.Errorf("limit %d is above the maximum of %d", q.Limit, MaxLimit)
	}
	if q.Sort.Key < 0 || q.Sort.Key >= numSortKeys {
		return fmt.Errorf("unknown sort key %d", q.Sort.Key)
	}
	if q.Time != nil {
		from, to := q.Time.From, q.Time.To
		if !from.IsZero() && !to.IsZero() && from.String() > to.String() {
			return fmt.Errorf("time range ends (%s) before it starts (%s)", to, from)
		}
	}
	return nil
}

// PageSize is the limit to actually use.
func (q Query) PageSize() int {
	if q.Limit == 0 {
		return DefaultLimit
	}
	return q.Limit
}
```

> **`SortCaptured` 是 iota 的 0，所以零值 Query 的排序键就是它；`Desc` 的零值是 false，也就是升序。** 但 Step 1 的测试要求零值是**降序**。两条路：把 `Desc` 换成 `Asc`（零值降序，但字段名读起来别扭），或者给 `Query.Sort` 一个规范化方法。**选前者**——`SortSpec{Key, Asc bool}`，零值就是「拍摄时间，最新在前」，这正是 4.5 假设的顺序，而且不需要任何人记得调用规范化。实现时把上面 `Desc` 全改成 `Asc` 并反转判断，测试同步改。

- [x] **Step 5: 跑测试确认绿**

```
go test ./internal/query/
```

- [x] **Step 6: 加 arch 测试**

```go
// TestQueryIsOnlyTheContract keeps the data access layer's contract free of
// its implementations.
//
// cmd builds queries and cmd may not import store (invariant 2), so the Query
// type cannot live there. Keeping this package down at domain is also what
// makes "the contract holds no SQL" (spec 4.1) checkable rather than a habit.
func TestQueryIsOnlyTheContract(t *testing.T) {
	for _, imp := range imports(t, "internal/query") {
		if imp != "internal/domain" {
			t.Errorf("internal/query imports %s.\n\n"+
				"The contract may depend on domain and nothing else. It is built by cmd,\n"+
				"which cannot reach store (invariant 2), and translated by store — if it\n"+
				"could see either, the contract and one implementation would be welded\n"+
				"together and spec 4.1's \"no SQL in the interface\" would stop being\n"+
				"something a test can hold you to.", imp)
		}
	}
}
```

- [x] **Step 7: 确认 arch 测试会咬**

往 `internal/query/query.go` 的 import 块里加 `_ "github.com/jinzhao1994/kist/internal/store"`，跑 `go test ./internal/arch/ -run TestQueryIsOnlyTheContract`：必须红。删掉。

> M2a 的经验：**import 一定要加进 import 块里**，加在文件末尾的话 `build.ImportDir` 读不到，测试会假绿。

- [x] **Step 8: 提交**

```bash
git add internal/query/ internal/arch/arch_test.go
git commit -m "feat(query): DAL 的契约——AST、排序、时间区间，不含一个字符的 SQL

4.1 说得很直接：正确的抽象不是包一层通用 SQL，而是接口里不出现 SQL。
所以 internal/query 只依赖 domain，arch 测试守着这一点。

它必须是独立的包，不能住在 store 里：cmd 要构造 Query，而 cmd 不许 import
store（不变量 2）。这和 oplog 是一个形状——契约一个包，满足它的东西另一个包。

空集合的语义写在类型的 godoc 里并且有测试：空 AND 是恒真（AND 的单位元），
空 OR 和空叶子恒假。标准代数答案，但正是翻译器最容易写反的地方，而写反的
表现是「用户一个 tag 都没勾」时静默地放行全部或全部拦掉。

TagExpr 的叶子拿到的是**展开好的**后代 ID 集合。4.3 定的两步走里第一步是
内存树遍历，归 library；把展开塞进契约后面，等于要求每个实现都拖一棵 tag
树进来。"
```

---

### Task 2: 带排序标识的游标

**Files:**
- Create: `internal/query/cursor.go`, `internal/query/cursor_test.go`

**Interfaces:**
- Produces: `(SortSpec).ID() string`、`(SortSpec).Value(domain.File) string`、`query.EncodeCursor(SortSpec, domain.File) string`、`query.DecodeCursor(SortSpec, string) (key string, id domain.FileID, err error)`、`query.ErrCursorSortMismatch`

B1 里挂了很久的那条到这里落地。**游标住在 `query` 而不是 `store`**：它是契约的一部分——两个实现必须对「这个游标是什么意思」达成一致，否则换后端就要重新分页。

- [x] **Step 1: 写失败的测试**

```go
package query

import (
	"errors"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
)

func fileWith(t *testing.T, captured, name string) domain.File {
	t.Helper()
	f := domain.File{ID: domain.NewFileID(), Name: name, NameFold: domain.Fold(name)}
	if captured != "" {
		d, err := domain.ParseLocalDate(captured)
		if err != nil {
			t.Fatal(err)
		}
		f.CapturedAt = d
	}
	return f
}

func TestCursorRoundTrips(t *testing.T) {
	for _, s := range []SortSpec{
		{Key: SortCaptured},
		{Key: SortCaptured, Asc: true},
		{Key: SortImported},
		{Key: SortName, Asc: true},
	} {
		f := fileWith(t, "2019-08-12", "IMG_1.jpg")
		key, id, err := DecodeCursor(s, EncodeCursor(s, f))
		if err != nil {
			t.Fatalf("%+v: %v", s, err)
		}
		if id != f.ID {
			t.Errorf("%+v: id = %s, want %s", s, id, f.ID)
		}
		if key != s.Value(f) {
			t.Errorf("%+v: key = %q, want %q", s, key, s.Value(f))
		}
	}
}

// The whole reason the sort travels inside the cursor. Paging under one order
// and then continuing under another must fail — the alternative is a page that
// looks perfectly normal and is silently wrong, which nobody ever notices
// (spec 4.2.1).
func TestCursorFromAnotherSortIsRefused(t *testing.T) {
	f := fileWith(t, "2019-08-12", "IMG_1.jpg")
	c := EncodeCursor(SortSpec{Key: SortCaptured}, f)

	for _, other := range []SortSpec{
		{Key: SortName},
		{Key: SortImported},
		{Key: SortCaptured, Asc: true}, // same key, other direction
	} {
		if _, _, err := DecodeCursor(other, c); !errors.Is(err, ErrCursorSortMismatch) {
			t.Errorf("a captured-desc cursor was accepted for %+v (err %v)", other, err)
		}
	}
}

// A file with no capture time still needs a usable cursor: it is where the
// NULLS LAST block begins, and paging has to be able to stop inside it
// (spec 4.5.1).
func TestCursorForAFileWithNoCaptureTime(t *testing.T) {
	f := fileWith(t, "", "scan.pdf")
	key, id, err := DecodeCursor(SortSpec{Key: SortCaptured}, EncodeCursor(SortSpec{Key: SortCaptured}, f))
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		t.Errorf("key = %q, want empty", key)
	}
	if id != f.ID {
		t.Error("the id did not survive")
	}
}

// Rubbish in must be an error, not a panic and not an empty first page.
func TestDecodeRejectsRubbish(t *testing.T) {
	for _, c := range []string{"not base64!!", "", "AAAA"} {
		if _, _, err := DecodeCursor(SortSpec{Key: SortCaptured}, c); err == nil {
			t.Errorf("%q was accepted as a cursor", c)
		}
	}
}

// The name key is the folded name, so that paging agrees with the ordering —
// a cursor holding the raw name would compare against a folded column and skip
// or repeat rows whose case differs (spec 4.8).
func TestNameCursorCarriesTheFoldedName(t *testing.T) {
	f := fileWith(t, "", "IMG_1.jpg")
	if got := (SortSpec{Key: SortName}).Value(f); got != f.NameFold {
		t.Errorf("name sort value = %q, want the folded name %q", got, f.NameFold)
	}
}
```

- [x] **Step 2: 跑一遍确认它红**

```
go test ./internal/query/ -run Cursor
```

- [x] **Step 3: 实现**

`internal/query/cursor.go`：

```go
package query

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
)

// ErrCursorSortMismatch means a cursor was made under a different ordering.
var ErrCursorSortMismatch = errors.New("this cursor belongs to a different sort order")

// ID names one ordering, and travels inside every cursor made under it.
func (s SortSpec) ID() string {
	name := "captured"
	switch s.Key {
	case SortImported:
		name = "imported"
	case SortName:
		name = "name"
	}
	if s.Asc {
		return name + ":asc"
	}
	return name + ":desc"
}

// Value is the sort key of one file under this ordering — the value a cursor
// carries and a predicate compares against.
//
// It is a string for every key, including the ones that are not naturally one,
// because a cursor is opaque bytes and the comparison is byte order anyway
// (spec 4.8). Keeping one representation means one predicate shape rather than
// three.
func (s SortSpec) Value(f domain.File) string {
	switch s.Key {
	case SortImported:
		return f.ID.String()
	case SortName:
		// The folded name, not the raw one: the column being ordered is
		// name_fold, and a cursor comparing a raw name against a folded column
		// would step over rows (spec 4.8).
		return f.NameFold
	default:
		return f.CapturedAt.String()
	}
}

// EncodeCursor renders the position of the last row of a page.
//
// The ordering travels with it. Continuing one order with another order's
// cursor is refused rather than approximated, because the approximation is a
// page that looks right and is not (spec 4.2.1).
func EncodeCursor(s SortSpec, f domain.File) string {
	raw := s.ID() + "\x00" + s.Value(f) + "\x00" + f.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor reads a cursor back, refusing one made under another ordering.
func DecodeCursor(s SortSpec, cursor string) (string, domain.FileID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: %w", err)
	}
	parts := strings.SplitN(string(raw), "\x00", 3)
	if len(parts) != 3 {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: expected three fields")
	}
	if parts[0] != s.ID() {
		return "", domain.FileID{}, fmt.Errorf("%w: it was made under %s, this query is %s",
			ErrCursorSortMismatch, parts[0], s.ID())
	}
	id, err := domain.ParseID(parts[2])
	if err != nil {
		return "", domain.FileID{}, fmt.Errorf("bad cursor: %w", err)
	}
	return parts[1], domain.FileID{ID: id}, nil
}
```

- [x] **Step 4: 跑测试确认绿**

- [x] **Step 5: 确认排序标识真的会咬**

把 `EncodeCursor` 里的 `s.ID() + "\x00" +` 去掉、`DecodeCursor` 相应改成两段，跑 `TestCursorFromAnotherSortIsRefused`：必须红。恢复。

**这一条特别值得验**：它守的是一个不会报错、不会崩、只会给出一页看起来完全正常的乱序结果的 bug。

- [x] **Step 6: 提交**

```bash
git add internal/query/cursor.go internal/query/cursor_test.go
git commit -m "feat(query): 游标带排序标识，对不上就报错

B1 里挂了很久的「排序键换了旧游标必须失效而不是给出错乱结果」到这里落地：
cursor = base64(sortID ␀ sortKey ␀ id)，请求的排序和游标里的对不上返回
ErrCursorSortMismatch。

验证过它会咬。这一条守的 bug 不报错、不崩溃、只会给出一页看起来完全正常
的乱序结果——那种东西没人会发现。

游标住在 query 而不是 store，因为它是契约的一部分：两个实现必须对「这个
游标是什么意思」达成一致，否则换后端就要重新分页。

排序值一律用字符串，包括本来不是字符串的键。游标本来就是不透明字节，而
比较是字节序（4.8），统一表示换来的是一种谓词形状而不是三种。name 那个键
取的是折叠名——被排序的列是 name_fold，游标里放原名会跨过大小写不同的行。"
```

---

### Task 3: `store` 的翻译器——谓词

**Files:**
- Create: `internal/store/translate.go`, `internal/store/translate_test.go`

**Interfaces:**
- Consumes: `query.Query`
- Produces: `store.translate(query.Query) (where string, args []any, err error)`（包内）、`store.tagPredicate(query.TagExpr) (string, []any)`

翻译是**纯函数**：同一个 Query 出同一串 SQL 和同一组参数。因此它可以在不开库的情况下被穷举测试，而 conformance suite 去管「跑出来的结果对不对」。两层分工要分清：**这里测的是形状，套件测的是语义。**

- [x] **Step 1: 写失败的测试**

`internal/store/translate_test.go`。挑几条真正会写错的：

```go
package store

import (
	"strings"
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// An empty query filters nothing, and must not emit a stray AND.
func TestTranslateEmptyQuery(t *testing.T) {
	where, args, err := translate(query.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(where, "AND") || len(args) != 0 {
		t.Errorf("where = %q, args = %v; want no conditions", where, args)
	}
}

// LIKE without ESCAPE is the bug where searching for "100%" matches
// everything. It is invisible until somebody types a percent sign.
func TestTextSearchEscapesWildcards(t *testing.T) {
	where, args, err := translate(query.Query{Text: "100%_x"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(where, `ESCAPE '\'`) {
		t.Errorf("no ESCAPE clause: %q", where)
	}
	pattern, _ := args[0].(string)
	if strings.Contains(pattern, `%_x`) || !strings.Contains(pattern, `\%`) {
		t.Errorf("pattern %q does not neutralise the wildcards", pattern)
	}
	// And it matches the folded column with a folded pattern (spec 4.7).
	if !strings.Contains(where, "name_fold") {
		t.Errorf("text search does not use the folded column: %q", where)
	}
}

// The empty-set algebra, at the SQL level. Getting one of these backwards
// silently widens or narrows every query a caller builds with no tags.
func TestTagPredicateEmptySets(t *testing.T) {
	cases := []struct {
		name string
		expr query.TagExpr
		want string // the whole predicate
	}{
		{"an empty AND is true", query.TagAll{}, "1"},
		{"an empty OR is false", query.TagAnyOf{}, "0"},
		{"an empty leaf is false", query.TagAny{}, "0"},
		{"NOT of an empty leaf is true", query.TagNot{Expr: query.TagAny{}}, "NOT (0)"},
	}
	for _, c := range cases {
		got, _ := tagPredicate(c.expr)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// Nesting has to come out parenthesised, or precedence changes the meaning:
// "a AND (b OR c)" written without brackets is "(a AND b) OR c".
func TestTagPredicateParenthesisesNesting(t *testing.T) {
	a, b, c := domain.NewTagID(), domain.NewTagID(), domain.NewTagID()
	expr := query.TagAll{
		query.TagAny{a},
		query.TagAnyOf{query.TagAny{b}, query.TagAny{c}},
	}
	got, args := tagPredicate(expr)
	if !strings.Contains(got, "OR") || !strings.Contains(got, "AND") {
		t.Fatalf("predicate lost the structure: %q", got)
	}
	if strings.Count(got, "(") != strings.Count(got, ")") {
		t.Errorf("unbalanced brackets: %q", got)
	}
	if len(args) != 3 {
		t.Errorf("%d args, want 3", len(args))
	}
	// The OR must be bracketed as a unit inside the AND.
	if !strings.Contains(got, "(EXISTS") {
		t.Errorf("the nested OR was not grouped: %q", got)
	}
}

// The overlap predicate for variable-precision dates (spec 4.2.1). The shape
// matters: comparing the value against only the first length(value) characters
// of the lower bound is what makes "2019" overlap "2019-03..2019-06".
func TestTimeRangeUsesOverlapNotContainment(t *testing.T) {
	from, _ := domain.ParseLocalDate("2019-03")
	to, _ := domain.ParseLocalDate("2019-06")
	where, args, err := translate(query.Query{Time: &query.DateRange{From: from, To: to}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(where, "substr(") || !strings.Contains(where, "length(captured_at)") {
		t.Errorf("not the overlap form: %q", where)
	}
	if len(args) != 2 {
		t.Errorf("%d args, want 2", len(args))
	}
	// An open end must not emit a condition for the end it does not have.
	where, args, err = translate(query.Query{Time: &query.DateRange{From: from}})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 {
		t.Errorf("an open-ended range emitted %d args: %q", len(args), where)
	}
}

// A file with no capture time is in no time range (spec 4.2.1). The predicate
// has to say so, because NULL comparisons are neither true nor false and a
// missing IS NOT NULL would let them through or not depending on the engine.
func TestTimeRangeExcludesFilesWithNoCaptureTime(t *testing.T) {
	from, _ := domain.ParseLocalDate("2019")
	where, _, err := translate(query.Query{Time: &query.DateRange{From: from}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(where, "captured_at IS NOT NULL") {
		t.Errorf("nulls are not excluded from the range: %q", where)
	}
}

// A tag id set larger than the engine's parameter limit must be a clear error
// rather than a driver-level failure nobody can read (spec 4.3 records 32766).
func TestTooManyTagIdsIsAClearError(t *testing.T) {
	huge := make(query.TagAny, maxParameters+1)
	for i := range huge {
		huge[i] = domain.NewTagID()
	}
	if _, _, err := translate(query.Query{Tags: huge}); err == nil {
		t.Error("a tag set beyond the parameter limit was accepted")
	}
}
```

- [x] **Step 2: 跑一遍确认它红**

- [x] **Step 3: 实现**

`internal/store/translate.go`：

```go
package store

import (
	"fmt"
	"strings"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// maxParameters is SQLite's MAX_VARIABLE_NUMBER, measured (spec 4.3).
//
// It is checked rather than trusted to be large enough: a tag tree of a few
// thousand nodes is well inside it, but "well inside" is not a guarantee, and
// the failure without this check is a driver error mentioning a number nobody
// can connect to a tag.
const maxParameters = 32766

// translate turns a Query into a WHERE fragment and its arguments.
//
// It is a pure function of the query — same query in, same SQL out — so it can
// be tested exhaustively without a database. That division matters: this file
// is tested for the shape of what it emits, and the conformance suite is
// tested for what the results mean. Neither can stand in for the other.
//
// The fragment it returns starts with " AND " for every condition, so callers
// append it after a "WHERE 1 = 1".
func translate(q query.Query) (string, []any, error) {
	if err := q.Validate(); err != nil {
		return "", nil, err
	}

	var (
		sb   strings.Builder
		args []any
	)

	if len(q.Boxes) > 0 {
		if len(q.Boxes) > maxParameters {
			return "", nil, fmt.Errorf("%d boxes is beyond the %d-parameter limit", len(q.Boxes), maxParameters)
		}
		sb.WriteString(` AND box_id IN (` + placeholders(len(q.Boxes)) + `)`)
		for _, b := range q.Boxes {
			args = append(args, b)
		}
	}
	if q.Folder != nil {
		sb.WriteString(` AND folder_id = ?`)
		args = append(args, *q.Folder)
	}
	if q.Text != "" {
		// ESCAPE is not optional: without it a search for "100%" matches
		// everything and "a_b" matches "axb". The pattern is folded with the
		// same function that built the column, so case-insensitivity is ours
		// rather than the engine's (spec 4.7).
		sb.WriteString(` AND name_fold LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(domain.Fold(q.Text))+"%")
	}
	if q.Time != nil {
		frag, targs := timePredicate("captured_at", *q.Time)
		sb.WriteString(frag)
		args = append(args, targs...)
	}
	if !query.MatchesEverything(q.Tags) {
		for _, leaf := range query.Leaves(q.Tags) {
			if len(leaf) > maxParameters {
				return "", nil, fmt.Errorf("%d tags is beyond the %d-parameter limit", len(leaf), maxParameters)
			}
		}
		pred, targs := tagPredicate(q.Tags)
		sb.WriteString(` AND ` + pred)
		args = append(args, targs...)
	}
	return sb.String(), args, nil
}

// tagPredicate translates a tag expression.
//
// The empty cases are the ones worth stating: an empty AND is the identity and
// becomes "1", an empty OR and an empty leaf are false and become "0". That is
// the algebra, and it is what makes "the user selected no tags" fail closed
// (spec 4.2.1). Everything composite is bracketed, because unbracketed
// "a AND b OR c" does not mean what it was built to mean.
func tagPredicate(e query.TagExpr) (string, []any) {
	switch v := e.(type) {
	case nil:
		return "1", nil

	case query.TagAny:
		if len(v) == 0 {
			return "0", nil
		}
		args := make([]any, len(v))
		for i, id := range v {
			args[i] = id
		}
		return `EXISTS (SELECT 1 FROM file_tags WHERE file_id = files.id AND tag_id IN (` +
			placeholders(len(v)) + `))`, args

	case query.TagAll:
		return join(v, " AND ", "1")

	case query.TagAnyOf:
		return join(v, " OR ", "0")

	case query.TagNot:
		inner, args := tagPredicate(v.Expr)
		return "NOT (" + inner + ")", args
	}
	// The interface is closed, so this is unreachable — but a silent "match
	// everything" for an unknown case would be the worst possible answer.
	return "0", nil
}

func join(children []query.TagExpr, op, empty string) (string, []any) {
	if len(children) == 0 {
		return empty, nil
	}
	parts := make([]string, len(children))
	var args []any
	for i, c := range children {
		p, a := tagPredicate(c)
		parts[i] = p
		args = append(args, a...)
	}
	if len(parts) == 1 {
		return parts[0], args
	}
	return "(" + strings.Join(parts, op) + ")", args
}

// timePredicate matches variable-precision values that overlap a range.
//
// Values are stored as ISO-8601 prefixes (spec 1.3), so a value denotes an
// interval: "2019" is the whole of 2019. Overlap rather than containment is
// the decision in spec 4.2.1 — an archive exists to help you find things, so a
// box known only to be "2019" should turn up in a search for spring 2019.
//
// Both halves fall out of prefix ordering:
//
//   - the interval starts at or before the upper bound  ⟺  value <= :hi
//     ("2019" <= "2019-06-30" lexicographically, and that is the same claim)
//   - the interval ends at or after the lower bound     ⟺  value >= the first
//     length(value) characters of :lo — compare only as far as the value's own
//     precision goes, which is exactly what "we only know this much" means
//
// NULL is excluded explicitly. A file with no capture time is in no range
// (spec 4.2.1), and leaving it to NULL comparison semantics would make the
// answer depend on the engine.
func timePredicate(column string, r query.DateRange) (string, []any) {
	var (
		sb   strings.Builder
		args []any
	)
	if r.From.IsZero() && r.To.IsZero() {
		return "", nil
	}
	sb.WriteString(` AND ` + column + ` IS NOT NULL`)
	if !r.To.IsZero() {
		sb.WriteString(` AND ` + column + ` <= ?`)
		args = append(args, r.To.End())
	}
	if !r.From.IsZero() {
		sb.WriteString(` AND ` + column + ` >= substr(?, 1, length(` + column + `))`)
		args = append(args, r.From.Start())
	}
	return sb.String(), args
}
```

> **`LocalDate.Start()` / `.End()` 要在 `domain` 里加**（如果还没有）：把变精度前缀补成完整的 `YYYY-MM-DD`，下界补最小、上界补当月最后一天。**闰年和月末天数要真算**，`2019-02` 的 `End()` 是 `2019-02-28`，`2020-02` 是 `2020-02-29`。这两个方法各写一个表驱动测试，含闰年、含 12 月、含只有年份和只有年月三种精度。

- [x] **Step 4: 跑测试确认绿**

```
go test ./internal/store/ -run 'Translate|TagPredicate|TimeRange|TextSearch|TooManyTag'
```

- [x] **Step 5: 确认空集合的语义会咬**

把 `TagAll` 的 `empty` 从 `"1"` 改成 `"0"`，跑 `TestTagPredicateEmptySets`：必须红。改回去。

再把 `join` 里的括号去掉，跑 `TestTagPredicateParenthesisesNesting`：必须红。

- [x] **Step 6: 提交**

```bash
git add internal/store/translate.go internal/store/translate_test.go internal/domain/
git commit -m "feat(store): Query 到 SQL 的翻译器，纯函数

翻译是纯的——同一个 Query 出同一串 SQL 和同一组参数——所以它可以在不开库
的情况下被穷举测。两层分工要分清：**这里测的是形状，conformance suite 测
的是语义**，谁也顶替不了谁。

时间用重叠而不是包含（4.2.1）。两半都从前缀序里掉出来：区间起点 ≤ 上界
就是 value <= :hi（'2019' <= '2019-06-30' 字面上就是这个意思），区间终点
≥ 下界就是 value 和 :lo 的前 length(value) 个字符比——只比到这个值自己的
精度为止，而那正是「我们只知道这么多」的意思。

NULL 显式排除。没有拍摄时间的文件不属于任何时间范围（4.2.1），交给 NULL
比较语义的话答案会取决于引擎。

参数上限查而不是信。tag 树几千个节点是远在 32766 以内，但「远在以内」不是
保证，而漏查的表现是一句驱动层的报错，里面那个数字没人能对应到某个 tag。"
```

---

### Task 4: 排序与 keyset —— 三个键 × 两个方向

**Files:**
- Modify: `internal/store/translate.go`, `internal/store/files.go`
- Test: `internal/store/translate_test.go`（补）、`internal/store/files_test.go`（补）

**Interfaces:**
- Produces: `store.orderBy(query.SortSpec) string`、`store.cursorPredicate(query.SortSpec, key string, id domain.FileID) (string, []any)`、`ListFiles(q Queryer, qry query.Query) ([]domain.File, string, error)`

M1c 的 `cursorPredicate` 只处理一个排序、一个方向。现在是**六种组合**，而且 `captured` 那两种还要跨 NULL 边界。**这是整个 M2b 最容易写出静默 bug 的地方**——写错不报错，只是某一页少几行或多几行。

- [x] **Step 1: 先把 M1c 的教训抄过来**

M1c 那个 `TestListFilesCursorWalksEveryRowExactlyOnce` 是这样咬住 bug 的：**一次只取一行**，走完全表，断言「每行恰好出现一次」。把它推广成对**六种排序**都跑一遍的表驱动测试，fixture 里必须有：

- 若干有 `captured_at` 的文件，**含重复值**（tie-breaker 才有意义）
- 若干没有 `captured_at` 的文件（NULL 边界）
- **大小写不同、折叠后相邻**的文件名（`name` 排序的 tie）

```go
// Paging must visit every row exactly once, in every order the contract
// offers. This is the shape of bug that does not error: a predicate that is
// wrong at the boundary silently drops or repeats a page, and the only way to
// see it is to walk the whole table one row at a time and count.
func TestCursorWalksEveryRowExactlyOnceInEveryOrder(t *testing.T) {
	db := migrated(t)
	want := seedPagingFixture(t, db) // returns the ids that must be visited

	for _, s := range allSortSpecs() {
		t.Run(s.ID(), func(t *testing.T) {
			seen := map[domain.FileID]int{}
			var order []domain.FileID
			cursor := ""
			for i := 0; i < len(want)+5; i++ {
				page, next, err := ListFiles(db, query.Query{Sort: s, Cursor: cursor, Limit: 1})
				if err != nil {
					t.Fatal(err)
				}
				if len(page) == 0 {
					break
				}
				seen[page[0].ID]++
				order = append(order, page[0].ID)
				if next == "" {
					break
				}
				cursor = next
			}
			if len(order) != len(want) {
				t.Errorf("walked %d rows, want %d", len(order), len(want))
			}
			for id, n := range seen {
				if n != 1 {
					t.Errorf("%s was returned %d times", id, n)
				}
			}
			// And the order the walk produced must equal one whole-page query.
			whole, _, err := ListFiles(db, query.Query{Sort: s, Limit: len(want)})
			if err != nil {
				t.Fatal(err)
			}
			for i := range whole {
				if i < len(order) && whole[i].ID != order[i] {
					t.Errorf("row %d: paged %s, whole page %s", i, order[i], whole[i].ID)
					break
				}
			}
		})
	}
}
```

- [x] **Step 2: 跑一遍确认它红**

- [x] **Step 3: 实现 `orderBy`**

```go
// orderBy renders one ordering, with the tie-breaker that makes keyset paging
// possible at all.
//
// NULLS LAST is written out in both directions. SQLite would put nulls last
// under DESC anyway and first under ASC, and Postgres does the opposite for
// DESC — so leaving it implicit means the two engines disagree and no test run
// on SQLite can ever notice (spec 4.7). Unknown belongs at the end whichever
// way you are reading, so both directions say LAST.
//
// The id tie-breaker is not decoration: without it, two files with the same
// capture time can swap places between pages, and keyset paging then repeats
// one and drops the other (spec 4.5).
func orderBy(s query.SortSpec) string {
	dir, nulls := " DESC", " NULLS LAST"
	if s.Asc {
		dir = " ASC"
	}
	switch s.Key {
	case query.SortImported:
		// The id is the key, so there is nothing to break ties between.
		return ` ORDER BY id` + dir
	case query.SortName:
		// name_fold is NOT NULL, so no NULLS clause is needed or emitted.
		return ` ORDER BY name_fold` + dir + `, id` + dir
	default:
		return ` ORDER BY captured_at` + dir + nulls + `, id` + dir
	}
}
```

- [x] **Step 4: 实现 `cursorPredicate`**

**先在计划里把六种情况列清楚，再写代码。** 记 `k` 为游标里的排序键、`i` 为游标里的 id：

| 排序 | "严格在这一行之后" |
|---|---|
| `imported` DESC | `id < i` |
| `imported` ASC | `id > i` |
| `name` DESC | `name_fold < k OR (name_fold = k AND id < i)` |
| `name` ASC | `name_fold > k OR (name_fold = k AND id > i)` |
| `captured` DESC，`k` 非空 | `captured_at IS NULL OR captured_at < k OR (captured_at = k AND id < i)` |
| `captured` DESC，`k` 为空 | `captured_at IS NULL AND id < i` |
| `captured` ASC，`k` 非空 | `captured_at IS NULL OR captured_at > k OR (captured_at = k AND id > i)` |
| `captured` ASC，`k` 为空 | `captured_at IS NULL AND id > i` |

> **两个方向的 NULL 块长得一样**，因为 NULL 在两个方向上都排最后——所以「还没走到的部分」在两种情况下都包含整个 NULL 块（当游标不在 NULL 块里时），或者只剩 NULL 块里 id 更靠后的部分（当游标在 NULL 块里时）。这不对称于直觉，是 `NULLS LAST` 两处都写死的直接后果。

```go
func cursorPredicate(s query.SortSpec, key string, id domain.FileID) (string, []any) {
	lt, gt := " < ?", " > ?"
	cmp := lt
	if s.Asc {
		cmp = gt
	}

	switch s.Key {
	case query.SortImported:
		return ` AND id` + cmp, []any{id}

	case query.SortName:
		return ` AND (name_fold` + cmp + ` OR (name_fold = ? AND id` + cmp + `))`,
			[]any{key, key, id}

	default:
		if key == "" {
			// The cursor is inside the nulls block, which is last in both
			// directions — so everything not yet visited is also inside it.
			return ` AND captured_at IS NULL AND id` + cmp, []any{id}
		}
		return ` AND (captured_at IS NULL OR captured_at` + cmp +
				` OR (captured_at = ? AND id` + cmp + `))`,
			[]any{key, key, id}
	}
}
```

- [x] **Step 5: 改 `ListFiles` 收 `query.Query`**

删掉 `ListFilesOptions` 和 M1c 的 `EncodeCursor` / `decodeCursor` / `fileOrder`（游标搬到了 `query`）。`ListFiles` 变成：`Validate` → `translate` → `cursorPredicate` → `orderBy` → `LIMIT` → 取 `PageSize()+1` 行判断有没有下一页。

> **取 `PageSize()+1` 行，而不是「取满一页就认为还有下一页」。** M1c 是后者，代价是最后一页恰好占满时会多给一个游标，翻过去是空页。功能上无害，但 UI 上是「还有更多」按钮点下去什么都没有。既然重写，顺手修掉，并在 `ListFiles` 的 godoc 里写明。

- [x] **Step 6: 跑测试确认绿，并确认边界会咬**

三处分别验：

1. `cursorPredicate` 的 `captured` NULL 分支换成朴素版本（`captured_at < ?` 一条），跑 Step 1 的测试——**必须有某个排序报「walked N rows, want M」**
2. `orderBy` 里去掉 `NULLS LAST`——在 SQLite 上 DESC 不会变，**ASC 会变**，所以 `captured:asc` 那一档必须红。（这条证明了那个测试确实覆盖到了 4.7 记的引擎分歧的一半）
3. 去掉 `id` tie-breaker，跑同一个测试——重复值那几行必须出问题

**三处都要看到红。** 第 2 条尤其重要：它是 M2b 里唯一一个能在 SQLite 上验证 `NULLS LAST` 有效性的机会。

- [x] **Step 7: 提交**

```bash
git add internal/store/
git commit -m "feat(store): 三个排序键 × 两个方向的 keyset 分页

M1c 只有一个排序一个方向，现在是六种组合，而 captured 那两种还要跨 NULL
边界。这是 M2b 里最容易写出静默 bug 的地方——写错不报错，只是某一页少
几行或多几行。

所以测试沿用 M1c 那个形状并推广：**一次只取一行**走完全表，对六种排序
各跑一遍，断言每行恰好出现一次，并且和一次取完的顺序逐行相同。fixture
里有重复的 captured_at（tie-breaker 才有意义）、NULL、以及折叠后相邻的
大小写不同的文件名。

两个方向都显式写 NULLS LAST。SQLite 的 DESC 天然如此、ASC 相反，Postgres
的 DESC 相反——不写死就是两个引擎给相反的结果，而且在 SQLite 上永远测不
出来（4.7）。「不知道」在哪个方向上读都该排最后。

顺手修掉 M1c 一个小毛病：判断有没有下一页改成多取一行，而不是「取满一页
就认为还有」。后者在最后一页恰好占满时会多给一个游标，翻过去是空页——
功能上无害，UI 上是「还有更多」点下去什么都没有。"
```

---

### Task 5: conformance suite

**Files:**
- Create: `internal/query/querytest/suite.go`, `internal/query/querytest/fixture.go`, `internal/query/querytest/cases.go`
- Create: `internal/store/conformance_test.go`

**Interfaces:**
- Produces: `querytest.Backend`、`querytest.Fixture`、`querytest.Run(t *testing.T, open func(*testing.T) Backend)`

这是 4.2 那句「这部分工作量不该省」的兑现，也是 M2b 的产出本身。

**它是 `testing/fstest` 那个形状**：一个普通包，import `testing`，被实现方的测试调用。不是 `_test.go`——那样别的包就用不了它，而「别的包能用」正是 conformance suite 的定义。

- [x] **Step 1: 定 `Backend` 与 fixture**

`internal/query/querytest/suite.go`：

```go
// Package querytest is the conformance suite for the query contract.
//
// Any implementation of spec 4.2's Query must pass it. v0.1 has exactly one —
// SQLite — and the suite is written anyway, because it is the acceptance
// criterion a second implementation will be held to and writing it afterwards
// means writing it against whatever the first implementation happened to do
// (spec 4.2).
//
// It asserts results, never plans. There is no EXPLAIN here, no assertion
// about indexes, no knowledge of any engine: the questions are only "given
// this library and this query, which rows come back, in what order". A suite
// that knew how the answer was computed would be a regression test for one
// backend rather than a contract.
package querytest

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/query"
)

// Backend is one implementation under test.
type Backend interface {
	// Seed populates an empty library with exactly this content. It is called
	// once per subtest, on a fresh backend.
	Seed(t *testing.T, f Fixture)
	// Files answers a query.
	Files(q query.Query) ([]domain.File, string, error)
}

// Fixture is the library a case runs against. It is plain data so that the
// suite can describe what it needs without knowing how any backend stores it.
type Fixture struct {
	Boxes    []domain.Box
	Folders  []domain.Folder
	Files    []domain.File
	Tags     []domain.Tag
	FileTags map[domain.FileID][]domain.TagID
}

// Run executes the whole suite against a backend.
//
// open is called once per case with a fresh, empty library, because a case
// that had to undo the previous one's fixture would be a case nobody wants to
// add to.
func Run(t *testing.T, open func(*testing.T) Backend) {
	t.Helper()
	for _, c := range cases() {
		t.Run(c.name, func(t *testing.T) {
			b := open(t)
			b.Seed(t, c.fixture())
			c.run(t, b)
		})
	}
}
```

`internal/query/querytest/fixture.go` 提供一个**建 fixture 的小 DSL**，否则每个 case 都要手工造 UUID 和 folder 树：

```go
// Builder assembles a Fixture. Every id is derived from the name, so a case
// can refer to "IMG_1.jpg" rather than to a UUID, and a failure message names
// something a person recognises.
type Builder struct{ … }

func NewBuilder() *Builder
func (b *Builder) Box(name string, start, end string) *Builder
func (b *Builder) Folder(box, path string) *Builder
func (b *Builder) File(box, folder, name, captured string, tags ...string) *Builder
func (b *Builder) Tag(name, parent string) *Builder
func (b *Builder) Build() Fixture
func (b *Builder) ID(name string) domain.FileID   // for assertions
```

> **ID 由名字派生（例如对名字取 SHA-256 前 16 字节再打上 UUIDv7 的版本位），不用 `domain.NewFileID()`。** 两个理由：① 断言可以写 `want: []string{"a.txt", "b.txt"}` 而不是一串 UUID；② **导入序排序需要 id 有确定的先后**，随机 ID 会让 `SortImported` 的期望值每次都不一样。派生时要保证「后加的名字 id 更大」——最简单的做法是按加入顺序给一个递增的时间戳位。**这一条要在 `Builder` 的 godoc 里写明**，因为它是 `imported` 排序那几个 case 能有确定期望的唯一原因。

- [x] **Step 2: 写 cases**

`internal/query/querytest/cases.go`，按 4.2.1 的字段分组。**每个 case 断言的是「哪些文件、什么顺序」**：

| 组 | 至少要有的 case |
|---|---|
| **空查询** | 零值 Query 返回全部，按 captured DESC；空库返回空且游标为空 |
| **Boxes** | 单个、多个、不存在的 box 返回空 |
| **Folder** | 只返回那一层，**不递归**（子目录里的文件不出现） |
| **Text** | 子串匹配；**大小写不敏感**；`%` 和 `_` 被当字面量；中文；不匹配 folder 名也不匹配 box 名 |
| **Tags** | 单叶子；`TagAll` 两个条件取交；`TagAnyOf`；`TagNot`；**三层嵌套**；空 `TagAll` 返回全部；空 `TagAnyOf` 和空 `TagAny` 返回空；`TagNot{空叶子}` 返回全部 |
| **Time** | 完全落在范围内；**只知年份的值与范围重叠时必须命中**；范围完全在值之前 / 之后时不命中；开区间（只有 From / 只有 To）；**没有 captured_at 的文件永远不在任何时间范围里** |
| **Sort** | 六种组合各一个，断言完整顺序；`captured` 的 NULL 在两个方向上都排最后；`name` 用折叠序（`Apple` 和 `apple` 相邻） |
| **Cursor** | 逐页走完等于一次取完；**换排序的游标报错**；乱码游标报错；最后一页的游标为空 |
| **Limit** | 超过 `MaxLimit` 报错；负数报错；0 用默认值 |
| **组合** | box + tag + text + sort 一起用，断言不是各自结果的并集而是交集 |

**「不递归」和「空集合语义」这两组要写得最细**，它们是最容易在第二个实现上写错、而且写错了不报错的两处。

- [x] **Step 3: 把 SQLite 接进去**

`internal/store/conformance_test.go`：

```go
package store

import (
	"testing"

	"github.com/jinzhao1994/kist/internal/query"
	"github.com/jinzhao1994/kist/internal/query/querytest"
)

// TestSQLiteConformance is the SQLite implementation's half of spec 4.2's
// contract. When Postgres lands in v0.2 it gets a file exactly like this one
// and the same suite runs against it — which is the whole point of writing the
// suite now, with one implementation, rather than then, with two.
func TestSQLiteConformance(t *testing.T) {
	querytest.Run(t, func(t *testing.T) querytest.Backend {
		db := migrated(t)
		return &sqliteBackend{db: db}
	})
}

type sqliteBackend struct{ db *sql.DB }

func (b *sqliteBackend) Seed(t *testing.T, f querytest.Fixture) { … }

func (b *sqliteBackend) Files(q query.Query) ([]domain.File, string, error) {
	return ListFiles(b.db, q)
}
```

`Seed` 直接 INSERT。它是测试代码，所以可以碰 `store` 的内部，而且**必须**——conformance suite 不该知道 `library` 的写路径存在。

- [x] **Step 4: 跑套件**

```
go test ./internal/store/ -run TestSQLiteConformance -v
```

**预期第一次会红好几个。** 这是套件在干活——把每一个红都当成一个真问题去看，不要为了让它绿而改期望值。改期望值之前必须能说出「为什么原来的期望是错的」。

- [x] **Step 5: 确认套件真的会咬**

挑三处分别破坏实现，跑套件：

1. `tagPredicate` 里把 `TagAll` 的空值从 `1` 改成 `0` → 「空 `TagAll` 返回全部」必须红
2. `translate` 里把 `q.Folder` 那个条件改成也匹配子目录（例如换成 `box_id = ?`）→ 「不递归」必须红
3. `timePredicate` 里把重叠改成包含（下界那条换成 `column >= ?`）→ 「只知年份的值与范围重叠时必须命中」必须红

- [x] **Step 6: 提交**

```bash
git add internal/query/querytest/ internal/store/conformance_test.go
git commit -m "test(query): conformance suite——契约本身

4.2 说这部分工作量不该省，这是兑现。它是 testing/fstest 那个形状：一个
普通包 import testing，被实现方的测试调用；不是 _test.go，因为别的包能
用它正是 conformance suite 的定义。

**它断言结果，从不断言查询计划。** 没有 EXPLAIN、没有关于索引的断言、
不知道任何引擎——问题只有「给这批数据和这个 Query，回来的是哪几行、什么
顺序」。知道答案是怎么算出来的套件，是某一个后端的回归测试，不是契约。

fixture 的 ID 由名字派生而不是随机生成，两个理由：断言可以写文件名而不是
一串 UUID；而且导入序排序需要 id 有确定的先后，随机 ID 会让那几个 case
每次期望都不一样。

写完先破坏三处实现确认套件会咬：空 TagAll 的语义、folder 不递归、时间
用重叠而不是包含。这三处都是换个实现最容易写错、而且写错了不报错的地方。"
```

---

### Task 6: `library` 与 `cmd` 迁到 `Query`

**Files:**
- Modify: `internal/library/files.go`, `cmd/kist/find.go`, `cmd/kist/ls.go`
- Test: `internal/library/files_test.go`（补）、`cmd/kist/find_test.go`（新）

**Interfaces:**
- Produces: `(l *Library).Files(ctx, query.Query) ([]domain.File, string, error)`、`(l *Library).TagCondition(refs []string) (query.TagExpr, error)`、`(l *Library).ExcludeTags(refs []string) (query.TagExpr, error)`

tag 展开留在 `library`（4.3 第一步），`cmd` 只管把命令行翻译成 `Query`。

- [x] **Step 1: `library` 侧**

```go
// Files answers a query (spec 4.2).
//
// The tag expression arriving here already holds expanded id sets: spec 4.3's
// first step is a walk of the in-memory tag tree, and it belongs to whoever
// owns that tree, which is this package. TagCondition and ExcludeTags are how
// a caller gets from tag names to that expression.
func (l *Library) Files(ctx context.Context, q query.Query) ([]domain.File, string, error) {
	return store.ListFiles(l.db, q)
}

// TagCondition turns tag names into "carries all of these, each including its
// descendants" — the shape v0.1's UI and CLI expose (spec 4.3).
func (l *Library) TagCondition(refs []string) (query.TagExpr, error) {
	var all query.TagAll
	for _, ref := range refs {
		tag, err := l.ResolveTag(ref)
		if err != nil {
			return nil, err
		}
		all = append(all, query.TagAny(l.tags.Descendants(tag.ID)))
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}
```

> **`len(all) == 0` 时返回 `nil` 而不是空的 `TagAll{}`。** 两者在翻译器里同样是「不筛选」，但 `nil` 说的是「没有条件」，`TagAll{}` 说的是「零个条件的合取」——前者才是「用户没给 `--tag`」的实情。这个区别在 Task 1 的 `TestNilExpressionIsNoFilter` 里已经钉住了，这里保持一致。

`ExcludeTags` 同理，返回 `query.TagNot{Expr: query.TagAnyOf{…}}`。

- [x] **Step 2: `cmd/kist/find.go` 加三个 flag**

```
--sort   captured | imported | name          默认 captured
--asc                                        默认降序（captured 最新在前）
--since  变精度日期
--until  变精度日期
```

`--sort` 的取值要在 cobra 层校验，报错里列出合法值——`unknown sort key 99` 对用户没有意义。

**帮助文本里要写清楚时间筛选的两件事**，因为两者都会让人以为是 bug：

- 变精度是**重叠**匹配：`--since 2019-03 --until 2019-06` 会带出只标了「2019」的东西
- **M3 之前文件没有拍摄时间**，所以带 `--since` / `--until` 的 `find` 现在返回空

- [x] **Step 3: `ls` 迁过去**

`kist ls <box> <path>` 变成 `query.Query{Boxes: […], Folder: &id, Sort: {Key: SortName, Asc: true}}`。

> **`ls` 的默认排序换成文件名升序。** 目录浏览按拍摄时间倒序是反直觉的——`ls` 就该像 `ls`。这是一处行为改变，要写进 commit message 和设计文档第 7 节的 CLI 说明。

- [x] **Step 4: 写 cmd 层测试**

`cmd/kist/find_test.go`，用 Task 9（M2a）留下的 `runKist` 帮手：

```go
// The CLI's job is translating flags into a Query. What is worth testing here
// is the translation, not the query — the query has a whole conformance suite.
func TestFindTranslatesFlags(t *testing.T) { … }

// An unknown --sort must name the valid values. "unknown sort key 99" is what
// the contract says; it is not what a person needs to read.
func TestFindRejectsAnUnknownSortWithAUsefulMessage(t *testing.T) { … }

// A tag that does not exist is a mistake worth naming: returning nothing looks
// exactly like "you have no such files" (the rule M1c established).
func TestFindRejectsAnUnknownTag(t *testing.T) { … }
```

- [x] **Step 5: 跑全部**

```
make check
```

- [x] **Step 6: 手动走查**

```bash
kist --library /tmp/m2b find --name img --sort name --asc
kist --library /tmp/m2b find --tag 人物 --not-tag 隐私
kist --library /tmp/m2b find --since 2019-03 --until 2019-06   # 期望：空，并且帮助里解释了为什么
kist --library /tmp/m2b ls 云南 day1                            # 期望：按文件名升序
```

- [x] **Step 7: 提交**

---

### Task 7: 收尾

- [x] **Step 1: 把实现中冒出来的偏离写回设计文档**，每条附否掉另一条路的理由
- [x] **Step 2: `open-questions.md`** 划掉解决掉的，记下 M2b 新暴露的
- [x] **Step 3: `CLAUDE.md`** 写 M2a/M2b 已完成，下一步 M3（并提醒 A1 那个 HEIC / ffmpeg 的决定必须在 M3 开写前定）
- [x] **Step 4: 设计文档第 7 节** 的 CLI 清单补上 `--sort` / `--since` / `--until`，并记 `ls` 默认排序的改变
- [x] **Step 5: README 两份** 的检索例子更新
- [x] **Step 6: `make check`**，然后在 4000 文件的库上量一次带 tag 和文本的组合查询耗时，写回 4.4——**4.4 那句「实测 50 万行全扫 66ms」是在别处量的，本项目自己的库上还没量过**
- [x] **Step 7: 提交、推、等三平台 CI 绿**

---

## 自查

**规范覆盖：** 4.2 的每个字段都有归宿（`Kinds` 的归宿是 4.2.1 明写的「砍掉」）；4.3 的两步走、嵌套、参数上限都在 Task 1/3；4.4 的 LIKE 与折叠列在 Task 3；4.5 / 4.5.1 的 keyset 与 NULL 在 Task 2/4；4.6 的索引不变；4.7 的 `NULLS LAST` 在 Task 4 有专门的验证步骤；4.8 的字节序在 `orderBy` 和 `SortSpec.Value`。

**不在 M2b 的：** `Kinds`（M3）、`size` 排序（有触发条件的逃生舱）、Postgres 实现（v0.2，但套件为它写好了）、全文倒排索引（4.4 的触发条件未到）。

**类型一致性：** `query.SortSpec` 用 `Asc` 而非 `Desc`（Task 1 Step 4 的说明），`ID()` / `Value()` / `EncodeCursor` / `orderBy` / `cursorPredicate` 六处都按 `Asc` 写；`TagExpr` 的四个构造子在 `tagPredicate` 的 type switch 里必须齐全。

**每个 task 都有「确认守卫会咬」的步骤**（Task 1 Step 7、Task 2 Step 5、Task 3 Step 5、Task 4 Step 6 的三处、Task 5 Step 5 的三处）。Task 4 Step 6 的第 2 条是全计划唯一一次能在 SQLite 上验证 `NULLS LAST` 有效性的机会，别跳过。

---

## 执行记录（2026-08-27）

七个 task 全部完成。**46 个 conformance case，全绿。**

### 计划被实测修正的两处

**① `NULLS LAST` 不是「本地永远测不出来」。** 4.7 一直这么写着，M2b 加了升序之后拿到了反证：SQLite 的 `ASC` 默认 `NULLS FIRST`，所以去掉显式的 `NULLS LAST`，`captured:desc` 全绿而 `captured:asc` 当场红。**这是 4.7 那条缓解办法第一次在本地拿到证据**，规范已改。

**② 43 个 case 第一次跑就全绿，而其中一个是假的。** 按计划破坏三处实现验证套件会咬时，「空 `TagAll` 改成恒假」**一个 case 都没红**。查下去是真发现：`translate` 先问 `MatchesEverything`，`TagAll{}` 直接被短路，`tagPredicate` 的空 AND 分支**只有嵌套时才可达**——而套件里一个嵌套的空集合都没有。补了三个 case（嵌套的空 AND / 空 OR / 空叶子），再跑同一个破坏，当场红。

> 规则被写下来但从来没被行使过，和没有这条规则是一回事。这是本里程碑最值得记的一条。

### 实测数字（4000 文件 / 5 MB 的 `kist.db`）

| | 端到端 |
|---|---|
| `kist version`（进程启动的地板） | 22 ms |
| 任何一种 `find`（换页大小 1→1000、加 tag、换排序） | 19 ms |

**这个规模上查询本身量不出来，全部成本是进程启动。** 不是「很快」，是「太小了」。有价值的是它排除了一件事：4.4 认下的全扫在四千个文件上不构成问题。写进了 4.4。

### 一处行为改变

**`kist ls` 的默认排序从拍摄时间倒序改成文件名升序。** `find` 保持拍摄时间倒序。写进了第 7 节。

**最终数字**：12 个包，`make check` 绿。
