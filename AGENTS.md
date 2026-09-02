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

**M1b 已完成**（计划见 `docs/plans/2026-08-27-m1b-oplog-kernel.md`）：`oplog`（九个 action 的 `Apply` / `Inverse` / `Targets`、注册表、`Tx` 接口）、`store` 的 ops 表与物化视图读写、`library` 的唯一写路径与 tag 树、`Undo`（严格模式）、`Rebuild`，以及 `box` / `tag` / `log` / `undo` 四组 CLI。

**M1c 已完成**（计划见 `docs/plans/2026-08-27-m1c-import-and-projection.md`）：`domain` 的 `File` / 子 `Folder` / 冲突改名、`oplog` 的 `import_files` / `delete_files` / `tag_files` / `untag_files`、`store` 的文件读写与 keyset 检索、`library.Import`（可中断、幂等、单文件失败不拖累整批）、新包 `internal/projection`（跨平台命名 + 期望树 + 磁盘 diff）、`Relink` 与写后增量投影，以及 `import` / `ls` / `find` / `file tag` / `relink` 五组 CLI。

**不变量 3 现在有测试撑着，而且覆盖到了文件**：建一批数据（含两层目录的导入、去重的重复内容、被撤销的导入、文件的 tag）→ 记全库快照 → `Rebuild` → 快照必须逐字节相同。验证过它会红：往 `Apply` 里塞一个「跑了几次」的计数器，快照当场指出是 `files` 表哪一行的 name 不一样了。

不变量 2 和依赖方向由 `internal/arch` 的测试守着（`oplog` 不得 import `store` / `blob`，`projection` 不得 import `store` / `oplog` / `library`，`cmd` 不得 import `store` / `oplog`，`domain` 和 `blob` 不许 import 仓库里的任何东西）。

**M2a 已完成**（计划见 `docs/plans/2026-08-27-m2a-integrity.md`）：新包 `internal/integrity`（发现模型 + 结构 / 内容 / 投影三层检查）、`store` 的一致性查询与 `DiffViews`、`blob` 的 `Walk` / `Rehash` / `CleanTmp` / `Restore` / 只读 `OpenDir`、`projection` 的 `DiffDir` 与 `CheckLinks`、`library` 的 `Verify` / `Scrub` / `Repair`，以及 `verify` / `scrub` / `repair` / `remote list` / `rebuild` 五组 CLI。

**「M2a 通过之前别删导入源」这条建议现在解除了。** `kist verify` 和 `kist scrub` 能告诉你库里的东西有没有问题，`kist repair` 能从第二份副本修回来——**前提是你真的有第二份副本**。`kist remote list` 会在你没配的时候说这件事。

**不变量 1 现在有用户能跑的检查**：`kist verify --deep` 把日志重放到一份 `VACUUM INTO` 出来的副本上，再逐表 `EXCEPT` 双向比。在 M2a 之前这条只在测试里被验证过。

**「修复不写 op」是结构保证**，不是约定：`internal/arch` 禁止 `internal/integrity` import `oplog`，所以它连一条 op 都写不出来。

**M2b 已完成**（计划见 `docs/plans/2026-08-27-m2b-query-contract.md`）：新包 `internal/query`（4.2 的 `Query` AST、`TagExpr` 树、带排序标识的游标）、`store` 的纯函数翻译器与六种排序的 keyset 分页、`internal/query/querytest` 的 46 个 conformance case，以及 `find --sort/--asc/--since/--until`。

**`internal/query` 只许依赖 `domain`**，arch 测试守着。契约不能住在 `store` 里，因为 `cmd` 要构造 `Query` 而 `cmd` 不许 import `store`——和 `oplog` 是同一个形状。

**conformance suite 断言结果，从不断言查询计划。** 没有 `EXPLAIN`、没有关于索引的断言。v0.2 的 Postgres 实现只需要加一个和 `internal/store/conformance_test.go` 一模一样的文件。

**B3（并发与多进程）已定并实现完**（2026-08-27 / 08-28，见设计文档 2.7）。

**决定 ①：`kist.lock` 上一把作用域为「一次逻辑写操作」的 OS 咨询锁。** 新包 `internal/lockfile`（`flock` / `LockFileEx` + 进程内信号量 + 持锁者诊断），`write()` / `Relink` / `Rebuild` 加锁，`Relink` 拆成加锁的导出版本和不加锁的内部版本（因为投影的回退路径在 `write()` 里面，再拿一次会和自己死锁）。`verify` / `scrub` / `repair` 明确不持锁。

**决定 ②：进程内不放任何「别的进程能让它作废」的状态。** 这条推翻了原来的「读路径比 `MAX(ops.id)`，不同就重载缓存」——版本号方案要求每一处状态都记得挂上检查，漏一处是个静默 bug 而且**没有任何测试会发现**（测试跑在一个进程里，那正是缓存永远正确的那种情况）。前提是 kist 部署在用户自己的设备上，没有吞吐量在替缓存说话。

落地两处：

- **投影目录名根本不是缓存**（`boxDirs()` 每次写都会重算），它存的是「上一次写之后的状态」。改成在 `write()` 里持锁期间取一次
- **tag 树改成按需的定点查询**：名字 / id 走索引 lookup，子树展开走递归 CTE（`store.TagDescendants`），环检查走向上的 CTE（`store.TagAncestors`）。同一份 SQL 在 SQLite 和 PG 上都能跑。**用 `UNION` 不用 `UNION ALL`**——去重是递归能终止的原因，有测试拿手工造的环证明这条

版本号没被否掉，它**降级**成一个可选的新鲜度特性（浏览器里那个页面新不新），不再是 M4 的前置条件。

**没实现**：③ 导入的名字分配——它的窗口是整个导入，持锁到底代价太大，选了「撞了就整批失败」，真正的修法写在 2.7 的实现记录里。**同一个形状在 tag 命令上也有**（`MoveTag` 先读祖先链再拿锁），修法相同。

**A1（HEIC）的截止条件改了**：从「M3 开始前」改成「网格浏览成为主要界面时」。HEIC 只挡住缩略图 / 预览，存储和检索都不需要解码，所以 M3 可以开写——但要按「**降级是默认路径**」规划。

**M3 的设计已定**（2026-08-28，见设计文档 1.5.2、1.7、§7），一轮讨论定了七件事，其中五件推翻了原来写下的东西。摘要见 `open-questions.md` B1f，几条要点：

- **metadata 的权威是 op log**，不是 `blobs/`。这条同时是插件系统的地基——提取器在 op 之外跑、结果记进 payload，所以它可以任意不纯（调 ffmpeg、读网络、跑模型）而不威胁重放的确定性
- **`captured_at` 是 unix 秒的瞬时**，不是 `LocalDate`。box 的日期是用户声明的模糊区间，文件的拍摄时间是真实时间点——契约里从此有两种时间，而那是对的
- **解析规则归内核不归插件**：插件只能往自己的命名空间放观察（比如 `geo:timezone`），`captured_at` 怎么算由内核决定。这是个信任边界
- **派生媒体整个拆出 M3，排到 Web UI 之后**（§7 的 M5）。这一刀顺手解决掉五个待决问题里的三个

**schema 要改三处**（`file_metadata`、`source` 存提取器 id、`file_extractions`），**但那张表现在一行数据都没有**，所以改的是初始迁移而不是加一个新迁移。

**M3 已完成**（计划见 `docs/plans/2026-08-28-m3-extraction.md`）：新包 `internal/extract`（提取器契约、命名空间所有权、`captured_at` 的解析规则）、内置的 `exif` 提取器（`bep/imagemeta`，**HEIC 的 EXIF 纯 Go 可读**）、`oplog` 的 `extract_metadata`、`store` 的 `file_metadata` / `file_extractions`、`library.Extract`，以及 `extract` / `file show` 两组 CLI。

**`file:size` / `file:mtime` 在导入时抓**，写成 `source = "file"` 的行。`file:` 因此是第三个保留命名空间（`user:` 归人、`file:` 归导入管线），提取器都不许认领——理由是同一条：没有任何东西能重新推导它们。所以没有 EXIF 的文件（扫描件、合同、录音）也有拍摄时间，`find --sort captured` 对整个库都是真的。 里程碑顺序讨论过一轮：把 HTTP API 单独拆成一个里程碑被否掉了（没有客户端的 API 不构成可交付产出，而前端恰恰是 API 的压力测试），理由写进了 §7 的 M4。

**CLI 与 Web UI 的分工已定**（2026-08-28，见设计文档 2.8）：**CLI 是运维工具 + 脚本接口，浏览归 Web UI**，M4 之后两个都留。否掉了「功能对等」（会强迫你为网格写终端版本、为 `repair` 写 web 版本）和「CLI 收缩成纯运维」（它砍掉脚本场景，而那正是 §7 否掉「API 单独成里程碑」的第一条理由）。

配套实现完了：**新包 `internal/wire`**（JSON 形状，只依赖 `domain`，arch 测试守着），**每个命令都支持全局 `--json`**。不是每个命令各自的 flag——一个「有些有有些没有」的 `--json` 比没有更糟，脚本分不清「这个命令没有 JSON 形式」和「你拼错了」。守卫测试递归遍历真实的 cobra 命令树，漏一个就红（已经抓到一个真漏）。

`--json` 现在做而不等 M4，是因为反过来更对：**先让 CLI 用一遍，M4 的 API 继承的就是一个已经被用过的形状。** 但别夸大——CLI 不发 HTTP，它测的是数据形状不是协议。错误暂时不进 JSON，等 B6 的错误模型。

**B6（错误模型）已定并实现**（2026-08-28，见设计文档 2.9）：**码是契约、句子是 fallback**（message 会被改写，要匹配就匹配码）；**七个类**，HTTP 状态和退出码从类推导；**`errcodes.Of()` 是纯函数不是注册表**（`domain` / `query` 结构上不能 import 任何东西，而两个 sentinel 就住在那里；而且 `init()` 注册这个形状这仓库栽过一次——`exif` 的 `init()` 从来没跑）；**结构化字段是命名字段不是自由袋子**（理由同不变量 4）。

新包 `internal/errcodes` 坐在 `library` **上面**——码属于上线格式，上线格式属于边界不属于内核，arch 测试禁止内核反向 import 它。带字段的错误类型（`BlockedUndoError` 等）住在产生它们的包里，不认识 errcodes。

**完备性测试用 `go/ast` 扫源码**：每个 `Err…` sentinel 都必须有码，漏一个就红。它的第一笔收获是发现 **`damaged` 这一类原来几乎是空的**——库自相矛盾的那几处当时都是裸 `fmt.Errorf`，全被归进 `internal`，也就是对用户显示成「这是个 bug」，而它们恰恰是「你的库坏了，跑 `kist verify`」。

**`--json` 出错时 stdout 保持 0 字节**，错误对象走 stderr（管道安全）。退出码一律 1，除了 `unavailable` 是 **75**（`EX_TEMPFAIL`，cron 据此重试）。**`verify` 的 0/1/2 不动也不并进来**——那说的是**发现**不是**错误**，两个轴。

**B2（任务模型）从 M4 前置里拿掉了**，推到手机端自动同步（见 §7 M4 的「为什么 M4 不需要任务模型」）。2.8 把运维命令划归 CLI 之后，M4 剩下的长操作只有浏览器上传，而上传进度是浏览器原生的、提取进度是一次 `COUNT`。最难的「任务与 op 的关系」其实已经被 3.7.1 解决了——它绑的是 `ctx.Done()`，而一个被掐断的 HTTP 请求和 SIGINT 是同一个信号。**失效条件：有客户端挂着等。** 手机后台同步不满足。

**M4 的前置条件现在是空的。**

**前端选型已定**（2026-08-28，见设计文档 5.2 / 5.4）：**Web UI 用 React，手机 app 用 Flutter，而且 app 必须小**——只做备份，浏览交给 PWA。

定 React 当天补上的第三条理由「app 走 RN，两端能复用」**几小时后被自己推翻了**：app 的范围一旦收成「只有备份」，一个备份引擎和一个浏览用的 web UI 就没有东西可共享，剩下的只有 OpenAPI 生成的客户端——而它生成 Dart 和生成 TS 一样顺。复用量归零之后，app 的框架纯按「iOS 后台上传谁更稳」选，于是是 Flutter。**教训记在 5.2 末尾：一条「两端能复用」的理由，先量一量能复用的到底是什么。**

顺手否掉了「两边都用 Flutter」：Flutter Web 画进 canvas，没有 Ctrl-F、没有右键另存为（而这个库装的是合同和扫描件），CanvasKit 的体积也过不了 `go:embed` 那一关。

**`00001_init.sql` 过了一轮逐表 review**（2026-09-02，commit `f16fee8`）。六处修正，理由都记在设计文档对应小节：

- **时间一律 int64 unix 纳秒**（`ts` / `created_at` / `updated_at` / `captured_at`）。定宽 ISO 字符串自带一个失败模式——字典序等于时间序依赖毫秒位补零，少一位则 `ORDER BY` 照常工作但不再正确——为堵它有一条「必须定宽」的纪律和写入前的 `Truncate`，而截断又制造了「写进去和读出来不一样」。整数两个补丁都不需要。**否掉的理由是「和 `start_at`/`end_at` 保持一致」**：那两个是变精度墙钟，`ts` 是绝对瞬时，1.5.2 早就否过同样的推理
- **删掉三处只有一个取值的列**：`ops.actor`（每行都是 `"cli"`，而 `"cli"` 回答的是 `source` 的问题不是 actor 的问题）、`boxes.description_fold`（没有任何东西查它）、`file_tags.source` / `box_tags.source`
- **`updated_at` 的传递从一个轴变成两个**：`source.PropagatesUpdatedAt()` **且** `action.ChangesContent()`。tag 和 metadata 是贴在内容上的标签、不是内容，而这个字段会被 `os.Chtimes` 写成目录 mtime。**否掉了「拆成两个 `updated_at` 字段」**——它想解决的「box 最近有没有活动」是日志问题，`LastOpTouchingBox` 已经在答；这条否定只对 box 成立，folder 的子树聚合必须物化
- **新增 `op_blobs` 和 `file_tag_sources`**。前者因为 file 行会随删除消失而 blob 不会，`blob → file → op` 恰好答不了孤儿 blob；后者因为一个 tag 可以被多个来源认领。**否掉「把 source 加进 `file_tags` 主键」**：热路径安全，但三处计数会静默翻倍
- **tag op 现在进 `op_boxes`**（`TagFilesPayload` 带 `BoxIDs`）。原来 `kist log --box` 看不到在这个 box 里打 tag 的 op，而「跨 box 批量打 tag」正是 `op_boxes` 做成多对多的头号理由
- **迁移不再写 `Down` 方向**：终点必然是 `DROP TABLE ops`，那不是回滚是删库

**这一轮里 `verify --deep` 抓到了一个真 bug**：加 `op_blobs` 时漏了 `Rebuild` 那一边，库里 3 行、重放 0 行。根因是写路径和 `Rebuild` 各自调三次插入，少写一个照样编译。合并成 `store.InsertOpIndexes` 之后，后来改 `touchAfter` 签名时编译器当场拦住了两个调用点——同一个陷阱，第二次逃不掉。

**补了三个守卫，都验证过会红。** `TestBlobDependsOnNothing`（`blob` 原来没有任何 arch 规则，而 `store` 不 import `blob`，所以 `blob → store` 能编译过）；`TestEveryDeclaredActionHasAHandler` 改用 `go/ast` 扫 `action.go`（**它的手抄列表已经漏了 `ActionExtractMetadata`**，正是它自己要防的那种错）；`TestEveryActionDeclaresWhetherItChangesContent`（新 action 必须表态）。

**惯例值得记下来：一份必须靠人记得更新的清单，早晚会漏。** 这一轮抓到三处同形状的——action 常量表、`DROP TABLE` 清单、两处各调三次的索引表插入。修法都是让它从源码/签名推导出来，而不是靠自觉。

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
