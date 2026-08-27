# kist

自部署的个人归档应用。基本单元是 **box**（一次活动、一段经历、一批档案），配合一棵全局 **tag** 树做正交检索。既装照片视频，也装证件合同报告。

Go 单二进制 + SQLite，Apache-2.0。

## 先读设计文档

**`docs/design/2026-08-22-kist-v0.1-design.md`** 是权威设计，动手前先读。
**`docs/design/open-questions.md`** 是配套的活文档，记录待决问题和尚未设计的领域。

前者记录**已经决定**的设计，后者记录**还没决定**的东西。

前者还记录了每个决策**被否掉的替代方案和否掉的理由**——这个惯例要保持下去。这份设计里相当一部分结论是推翻了第一版才得到的，只留结论的话很容易被重新提出来。

## 四条不能破的不变量

1. **权威数据只有 `ops` 表 + `blobs/` 目录。** `kist.db` 里其余每张表都可重建（含 `blobs` 表——扫目录即可）。`boxes/` 目录是人可读的**投影**，被改坏就 `kist relink` 整棵重建。
2. **`library` 是唯一写入口。** 每个写操作在一个事务里完成「插 op + 更新物化视图」。`api` 和 `cmd` 不能碰 `store` 的写接口。
3. **`Apply` 必须是纯函数。** 不生成 ID、不读时钟、不做任何依赖当前状态的判断——决策在生成 op 时做完，结果写进 payload。否则 `kist rebuild` 会重放出一个不同的库。
4. **payload 是不可查询的载荷。** 任何需要被查询的东西必须提取成列或索引表（`reverts_id` / `op_boxes` / `op_files`）。反过来 payload 必须自足，每张索引表都能从它单向重建。

## 当前状态

**M0 已完成**：LICENSE、双语 README、Makefile、CI/release workflow、issue/PR 模板，以及一个只有 `kist version` 的 CLI 骨架（用来验证四平台单二进制）。

**M1a 已完成**（计划见 `docs/plans/2026-08-22-m1a-storage-kernel.md`）：`domain` 的纯类型（UUIDv7 ID、变精度日期、折叠函数）、`blob` 的内容寻址存储（原子写、去重、损坏自愈、碰撞分支、链接能力探测）、`store` 的连接与 v0.1 全量 schema、`config`、`library.Create`，以及一个能真跑的 `kist init`。

**M1b 已完成**（计划见 `docs/plans/2026-08-27-m1b-oplog-kernel.md`）：`oplog`（九个 action 的 `Apply` / `Inverse` / `Targets`、注册表、`Tx` 接口）、`store` 的 ops 表与物化视图读写、`library` 的唯一写路径与内存 tag 树、`Undo`（严格模式）、`Rebuild`，以及 `box` / `tag` / `log` / `undo` 四组 CLI。

**M1c 已完成**（计划见 `docs/plans/2026-08-27-m1c-import-and-projection.md`）：`domain` 的 `File` / 子 `Folder` / 冲突改名、`oplog` 的 `import_files` / `delete_files` / `tag_files` / `untag_files`、`store` 的文件读写与 keyset 检索、`library.Import`（可中断、幂等、单文件失败不拖累整批）、新包 `internal/projection`（跨平台命名 + 期望树 + 磁盘 diff）、`Relink` 与写后增量投影，以及 `import` / `ls` / `find` / `file tag` / `relink` 五组 CLI。

**不变量 3 现在有测试撑着，而且覆盖到了文件**：建一批数据（含两层目录的导入、去重的重复内容、被撤销的导入、文件的 tag）→ 记全库快照 → `Rebuild` → 快照必须逐字节相同。验证过它会红：往 `Apply` 里塞一个「跑了几次」的计数器，快照当场指出是 `files` 表哪一行的 name 不一样了。

不变量 2 和依赖方向由 `internal/arch` 的测试守着（`oplog` 不得 import `store` / `blob`，`projection` 不得 import `store` / `oplog` / `library`，`cmd` 不得 import `store` / `oplog`）。

**M2a 已完成**（计划见 `docs/plans/2026-08-27-m2a-integrity.md`）：新包 `internal/integrity`（发现模型 + 结构 / 内容 / 投影三层检查）、`store` 的一致性查询与 `DiffViews`、`blob` 的 `Walk` / `Rehash` / `CleanTmp` / `Restore` / 只读 `OpenDir`、`projection` 的 `DiffDir` 与 `CheckLinks`、`library` 的 `Verify` / `Scrub` / `Repair`，以及 `verify` / `scrub` / `repair` / `remote list` / `rebuild` 五组 CLI。

**「M2a 通过之前别删导入源」这条建议现在解除了。** `kist verify` 和 `kist scrub` 能告诉你库里的东西有没有问题，`kist repair` 能从第二份副本修回来——**前提是你真的有第二份副本**。`kist remote list` 会在你没配的时候说这件事。

**不变量 1 现在有用户能跑的检查**：`kist verify --deep` 把日志重放到一份 `VACUUM INTO` 出来的副本上，再逐表 `EXCEPT` 双向比。在 M2a 之前这条只在测试里被验证过。

**「修复不写 op」是结构保证**，不是约定：`internal/arch` 禁止 `internal/integrity` import `oplog`，所以它连一条 op 都写不出来。

**M2b 已完成**（计划见 `docs/plans/2026-08-27-m2b-query-contract.md`）：新包 `internal/query`（4.2 的 `Query` AST、`TagExpr` 树、带排序标识的游标）、`store` 的纯函数翻译器与六种排序的 keyset 分页、`internal/query/querytest` 的 46 个 conformance case，以及 `find --sort/--asc/--since/--until`。

**`internal/query` 只许依赖 `domain`**，arch 测试守着。契约不能住在 `store` 里，因为 `cmd` 要构造 `Query` 而 `cmd` 不许 import `store`——和 `oplog` 是同一个形状。

**conformance suite 断言结果，从不断言查询计划。** 没有 `EXPLAIN`、没有关于索引的断言。v0.2 的 Postgres 实现只需要加一个和 `internal/store/conformance_test.go` 一模一样的文件。

**B3（并发与多进程）已定并实现了一半**（2026-08-27，见设计文档 2.7）。

**已实现**：新包 `internal/lockfile`（`flock` / `LockFileEx` + 进程内信号量 + 持锁者诊断），`write()` / `Relink` / `Rebuild` 加锁，`Relink` 拆成加锁的导出版本和不加锁的内部版本（因为投影的回退路径在 `write()` 里面，再拿一次会和自己死锁）。`verify` / `scrub` / `repair` 明确不持锁。

**没实现**：① 读路径比 `MAX(ops.id)`——只有 serve 需要，等 serve。③ 导入的名字分配——它的窗口是整个导入，持锁到底代价太大，选了「撞了就整批失败」，真正的修法写在 2.7 的实现记录里。

**A1（HEIC）的截止条件改了**：从「M3 开始前」改成「网格浏览成为主要界面时」。HEIC 只挡住缩略图 / 预览，存储和检索都不需要解码，所以 M3 可以开写——但要按「**降级是默认路径**」规划。

**M3 的存储模型已定**（2026-08-28，见设计文档 1.7）：metadata 的权威是 op log 而不是 `blobs/`（因为提取不是字节的稳定函数，而且多设备同步会静默分叉）；`extract_metadata` 的 payload 只带真的变了的键；展开规则按**形状**（标量、可打印、有界）而不是按**有用性**（白名单被推翻了）；只挂在 file 上，`file_metadata(file_id, key)` 带真外键（多态的 `target_type` 被推翻了——1.6 早就为 tag 写下过相反的结论）。**schema 要改，但那张表现在一行数据都没有。**

M3 还剩五件没定，见 `open-questions.md` B1f。

**下一步是 M3（提取与派生）。** 里程碑顺序讨论过一轮，结论是 §7 原来的顺序（M3 → M4）就是对的：把 HTTP API 单独拆成一个里程碑会制造一个没有客户端、因而没有可交付产出的里程碑，而前端恰恰是 API 的压力测试。否掉的理由写进了 §7 的 M4。

M3 的产出偏弱（它的价值要等 M4 的网格才兑现），§7 里如实记了这一点，没有再借 M4 的产出充数。

里程碑拆分见设计文档第 7 节，明确的范围外清单见第 8 节，待决问题见 `open-questions.md`。

## 约定

- **新决策要更新设计文档，并写明为什么不选另一条路。** 这是本仓库唯一真正要紧的规矩
- 语言分工：
  | | 语言 |
  |---|---|
  | 设计文档、`README.zh-CN.md`、commit message | 中文 |
  | 代码、注释、godoc、`README.md`、issue/PR 模板 | 英文 |

  分界线是**谁读它**：设计文档是作者的思考工具，代码和仓库门面是外部贡献者第一眼看到的东西
- 推之前跑 `make check`——它与 CI 等价
