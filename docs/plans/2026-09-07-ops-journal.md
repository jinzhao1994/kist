# 日志搬出数据库（`ops/`）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 让不变量 1 变成「`ops/` + `blobs/`，两个都是普通文件」，`kist.db` 整个降级成可重建的缓存。做完之后备份就是 `cp -a` 一个目录，换数据库引擎是一次 `rebuild`。

**Architecture:** 一个新包，两阶段写入，一条追赶路径。

新包 **`internal/journal`** 是 `ops/` 的文件层：行的形状、`sum` 的算法、桶命名与单调、append + fsync、顺序迭代、完整性校验、尾部恢复。**零仓库依赖**——和 `blob` 完全同形，理由也同形：权威的另一半必须能在不碰数据库的情况下被检查。

`library.write()` 从「一个事务四步」变成「先 append+fsync，再一个事务三步」。op id 因此自己分配（日志末尾 + 1），`store.AppendOp` 换成带显式 id 的 `InsertOp`。

`library.Open` 多一步追赶：日志末尾的 id 大于 `MAX(ops.id)` 就重放尾巴。`Rebuild` 的数据源从 `ops` 表换成 `ops/`，并且现在也要重灌 `ops` 表本身。`verify --deep` 因此变成真正的交叉验证。

**Spec:** `docs/design/2026-08-22-kist-v0.1-design.md` 的 2.1.1 / 2.2.2 / 3.1.1 是本计划的正文；配套改动在 2.6 / 3.8 / 3.11 / 2.7。

**前置：** M1a–M3 全部完成。**不需要 migration 兼容旧库**（2.1.1 明确否掉），但初始迁移 `00001_init.sql` 要改一处（`ops.id` 不再需要 `AUTOINCREMENT`）。

## Global Constraints

- **Go 1.26.1**，`CGO_ENABLED=0` 必须能构建，四个目标平台
- **不变量 1（新表述）：权威数据只有 `ops/` 目录 + `blobs/` 目录。** `kist.db` 每一张表都可重建，`ops` 表也在内
- **不变量 2（新表述）：`library` 是唯一写入口；先 append+fsync 到 `ops/`，再在一个事务里 apply。** 崩溃只可能让库落后，永不让库超前
- **不变量 3：`Apply` 必须是纯函数。** 这一轮不新增 op，`Apply` 的签名和语义一个字不动
- **不变量 4：payload 是不可查询的载荷。** `journal` 把 payload 当 `json.RawMessage` 原样搬运，不解析
- **`internal/journal` 不得 import 仓库里的任何东西**（新 arch 测试）
- **语言：代码、注释、godoc 用英文；设计文档与 commit message 用中文**
- 每个 task 结束 `make check` 必须通过

---

## 三条要一直记着的原则

### ① 先文件后库，永不反过来

一条视图里有、日志里没有的 op 是不可恢复的。任何时候拿不准顺序，问一句「崩在这里，库是落后还是超前」。

### ② `journal` 不认识 op 的语义

它搬运 `action` / `origin` 字符串和一坨 `json.RawMessage`，不知道 `import_files` 是什么意思。转换住在 `library`——和 `store.OpRow` 是同一个先例。

### ③ 撕裂写和位腐是两回事

文件末尾没有换行 = 没写完 = 可以安全截断（挪进 `ops/tmp/`）。JSON 完好但哈希对不上 = 写完了又坏了 = 拒绝，报 `damaged`。混为一谈会真的丢一条 op。

---

## 文件结构

```
internal/journal/
  journal.go        Dir、Open、路径与桶命名
  line.go           Line、MarshalLine/UnmarshalLine、sum 的算法
  append.go         Append（含桶单调）
  iter.go           Iter（按文件名序 → 行序）、Last
  verify.go         Check（行哈希 + id 连续 + 桶单调）、尾部恢复
  *_test.go
internal/arch/arch_test.go            + TestJournalDependsOnNothing
internal/store/ops.go                 AppendOp → InsertOp（显式 id）、ClearOps、MaxOpID
internal/store/migrations/00001_init.sql   ops.id 去掉 AUTOINCREMENT
internal/library/write.go             两阶段
internal/library/applyop.go（新）     插 op + Apply + 索引表 + touchAfter，三个调用方共用
internal/library/catchup.go（新）     追赶 + 旧库检测
internal/library/rebuild.go           数据源换成 journal
internal/library/create.go            建 ops/ 和 ops/tmp/
internal/library/deep.go              空库 + 从 journal 重放
internal/integrity/                   两个新 finding
```

---

## Tasks

### Task 1: `internal/journal` 的行与 sum

- [x] `Line` 结构体，字段顺序固定：`v` `id` `ts` `action` `origin` `reverts_id` `root_op_id` `payload` `sum`
- [x] `ts` 用定宽 RFC3339Nano：`Format("2006-01-02T15:04:05.000000000Z07:00")`，写入前 `.UTC()`
- [x] `payload` 是 `json.RawMessage`，原样进出
- [x] `sum` = 「清空 `sum` → 重新序列化 → sha256」，前缀 `sha256:`
- [x] `MarshalLine` / `UnmarshalLine`，后者校验 `sum`

**验收：** round-trip 逐字节稳定的测试（解出来再序列化必须和原始字节相同，含带各种 payload 的 case）；改一个字节 `sum` 必须对不上；`v` 不认识时报明确的错。

### Task 2: 桶命名与单调

- [x] `bucketFor(t time.Time) string` → `2026/09/2026-09-07_ops.jsonl`，UTC
- [x] `Dir.lastBucket()` 扫目录取最大的那个（文件名序）
- [x] 实际使用的桶 = `max(bucketFor(now), lastBucket())`

**验收：** 时钟往回拨一天，新行仍然落进原来那个桶，`Iter` 出来的 id 严格递增（这条验证过会红——去掉 `max` 就红）。

### Task 3: `Append` 与 `Iter`

- [x] `Dir.Append(Line) error`：建目录 → `O_APPEND|O_CREATE` → 写「行 + `\n`」→ `Sync()` → 关
- [x] `Dir.Iter(func(Line) error) error`：按文件名序遍历桶，逐行解析并校验 `sum`
- [x] `Dir.Last() (Line, bool, error)`：只读最后一个桶的最后一行（seek 到末尾往回找换行，不扫全文件）

**验收：** 写 1000 行读回来一致；`Last` 在空目录上返回 `false`；`Last` 不随行数变慢（不做基准，但实现里不许出现全量扫描）。

### Task 4: 完整性校验与尾部恢复

- [x] `Dir.Check() ([]Problem, error)`：行哈希、id 从 1 连续严格递增、桶名单调
- [x] `Dir.RecoverTail() (recovered bool, err error)`：最后一个桶不以 `\n` 结尾 → 截断，残字节写进 `ops/tmp/`
- [x] JSON 完好但 `sum` 对不上 → 不截断，返回错误

**验收：** 手工造四种坏法（位腐、半截行、删一行、删一个文件），每种都被对应的检查抓到；半截行被 `RecoverTail` 挪走且 `ops/tmp/` 里能找到原字节；位腐的行不被自动截掉。

### Task 5: arch 规则

- [x] `TestJournalDependsOnNothing`

**验收：** 往 `journal` 里加一句 `import ".../internal/store"` 必须红（验证一次再删掉）。

### Task 6: `store` 侧——`ops` 表变派生

- [x] `00001_init.sql`：`ops.id` 从 `INTEGER PRIMARY KEY AUTOINCREMENT` 改成 `INTEGER PRIMARY KEY`，注释改成「id 由 journal 分配，见 3.1.1」
- [x] `AppendOp` → `InsertOp(tx, OpRow)`，id 由调用方给
- [x] `MaxOpID(q Queryer) (int64, error)`
- [x] `ClearOps(tx)`，并进 `Rebuild` 的清理路径
- [x] `snapshotTables` 与 `ClearViews` 把 `ops` 纳入（顺序：`ops` 在 `op_*` 之后清）

**验收：** `make check` 绿；`TestRebuildReproducesTheLibraryExactly` 仍然绿。

### Task 7: `library.write()` 两阶段

- [x] 锁内：`RecoverTail` → 追赶 → `Last().ID + 1` 作为新 id
- [x] `journal.Append` + fsync
- [x] 事务：`InsertOp`（显式 id）→ `Apply` → `InsertOpIndexes` → `touchAfter` → commit
- [x] append 成功而事务失败：**不回滚文件**，返回错误；下一次 `Open` 会追赶上

**验收：** 一个注入错误的测试——让事务在 `Apply` 之后失败，断言日志里有这条、库里没有，然后 `Open` 一次两边一致（这条验证过会红：把顺序换成先库后文件，断言当场失败）。

### Task 8: `library.Open` 的追赶与旧库检测

- [x] `Open` 取一次锁：`RecoverTail` → 比 `journal.Last().ID` 和 `store.MaxOpID` → 落后就重放尾巴
- [x] 没有 `ops/` 但 `ops` 表非空 → 报 `damaged` 类错误，一句人话让用户重建
- [x] `library.Create` 建 `ops/` 和 `ops/tmp/`

**验收：** 造一个「日志有 3 条、库有 2 条」的库，`Open` 之后两边一致；造一个旧格式库，任何命令都报那句人话且退出码正确。

### Task 9: `Rebuild` 与 `verify --deep`

- [x] `rebuildInto` 的数据源从 `store.IterOps` 换成 `journal.Iter`，并先 `ClearOps` 再逐条 `InsertOp`
- [x] `deep.go`：不再 `VACUUM INTO`，改成建一个空库跑迁移 → 从 `journal` 重放 → `ATTACH` 逐表 `EXCEPT`
- [x] 两个新 finding：`LogAhead`（库落后日志 N 条）、`LogCorrupt`（`Dir.Check` 的问题）
- [x] `verify` 不修，不持锁

**验收：** `verify --deep` 在一个手工改坏了 `ops` 表某一行的库上必须报差异（**这是旧形状发现不了的那一类**，值得单独一个测试）。

### Task 10: 收尾

- [x] 设计文档已在本轮改完，核对交叉引用
- [x] `CLAUDE.md` 的当前状态 + 四条不变量
- [x] `README.md` / `README.zh-CN.md` 的「数据不被锁住」那一段升级成「历史也是普通文件」
- [x] 重测 `verify` / `verify --deep` / `scrub` 三个数字（3.8 的表要更新）
- [x] `make check`

---

## 自查

- [x] 崩在任何一步，库都只会落后不会超前
- [x] `journal` 不认识任何 op 语义，也 import 不到仓库里任何东西
- [x] 撕裂写和位腐走两条不同的路
- [x] `ops/` 的每种坏法都有一条测试
- [x] 不变量 3 的快照测试仍然绿

---

## 执行记录（2026-09-07）

全部十个 task 做完，`make check` 绿。四处偏离计划，都记在这里：

**① 计划里没有的一条：`library.WithoutRecovery()`。** 追赶放进 `Open` 之后 `kist verify` 也会追赶，于是它在检查之前先改了库——而 3.8 承诺它一个字节都不写。挂在 cron 上的 verify 会替别人完成一次被中断的写，然后报告一个它刚刚改过的库。`verify` / `scrub` / `repair` 三个命令改走 `openForChecks`，有测试钉着。**这是这一轮唯一一个自己给自己造出来的 bug**，而且是端到端跑真二进制时才发现的——单元测试全绿。

**② `store.IterOps` 变成死代码，删了。** 它存在的唯一理由是 `Rebuild` 要流式读日志，而日志换源之后 `Rebuild` 走 `journal.Iter`。连同它的两个测试一起删。

**③ 三处重复合并成 `applyOp`。** 写路径、追赶、`Rebuild` 都要「插 op + Apply + 索引表 + `touchAfter`」。不合并的话这一轮就会造出第三份——而这正是 `op_blobs` 那次漏更新的形状。

**④ 四个测试的前提被这次改动推翻，逐个改而不是删：**

| 测试 | 原来断言 | 现在 |
|---|---|---|
| `TestDiffViewsIgnoresOps` | ops 不许被比较（比了会恒真） | `TestDiffViewsComparesOps`——源换成文件之后比较才有意义 |
| `TestOpIDsAreMonotonic` | AUTOINCREMENT 不复用 rowid | `TestOpIDsComeFromTheCaller`——那条保证搬去了 journal |
| `TestRebuildStreamsTheLog` | 用 `store.IterOps` | 用 `ops.Iter`；`store.ListOps` 仍然禁止 |
| `TestAGroupChangesNothingAboutTheState` | 全库快照逐字节相同 | 排除 `ops` 段——`root_op_id` 是 ops 的列，本来就该不同 |

**验证过会红的守卫（这一轮新增或改动的）：**

- `TestBucketsNeverMoveBackwards`——去掉桶单调的 clamp，op 2 落进 09-06 的桶
- `TestJournalDependsOnNothing`——往 `journal` 里加 `import store`
- `TestACrashLeavesTheDatabaseBehindNeverAhead`——把 `Append` 挪到 `tx.Commit` 之后，日志当场是空的
- `TestDeepVerifyCatchesACorruptedOpsTable`——把 `ops` 从 `snapshotTables` 拿掉

**实测（4000 文件 / 16 MB / 2 条 op）：** `verify` 0.17 s，`verify --deep` 0.54 s（旧形状 0.38 s，慢约 40%——临时库要跑迁移，且多比一张 `ops` 表）。`ops/` 1.4 MB，`kist.db` 6.5 MB。数字写进了设计文档 3.8。
