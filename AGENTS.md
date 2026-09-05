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

**metadata 的删除已定并实现**（2026-09-03，见设计文档 1.7.7）：**用户删掉一条 metadata 写的是墓碑（`value IS NULL`、`source = user`），不是删行。**「从来没有过」和「有过、被人拿掉了」必须分得开——「没有」正是提取器存在的意义，直接删行等于删除什么也没做，只是慢一次 `kist extract` 才生效。

**否掉了「照抄一张 `metadata_rejections` 侧表」**：tag 的存在性是多对多的（exif 和 OCR 都可能打「猫」），所以抑制塞不进 `file_tags` 那一行；而 metadata 的 `(file_id, key)` 单一所有者（命名空间互斥），槽位本来就在表里。**同一个问题两种答案，分界线是「存在性有几个claimant」。**

**抑制机制本身没有新代码**：墓碑的 `source` 是 `user`，而提取管线早有「不属于自己的 key 整条跳过」的规则。这是选这个形式的第三个好处——没有新的检查点，也就没有将来会被忘记挂上的检查点。

新增 `set_metadata`（第十五个 action，payload 是 before/after 两个可空状态，`Inverse` 是互换）、`Tx` 的 `TombstoneFileMetadata` / `GetFileMetadata`、`kist file metadata rm`。**`value` / `type` 改成可空现在做而不推迟**，因为 SQLite 不支持 `DROP NOT NULL` 而这张表从 M3 起每次导入都有行。删掉 `exif:DateTimeOriginal` 会重算 `captured_at`，重算在生成 op 时做（要读时区，而配置不是 payload）。

顺手修掉三处过期的设计文档：`file_tags.source（user / derived）`（正是上一轮推翻的方案）、四处「unix 秒」、以及 1.7 里 `source=extracted` 那一段——它和自己下面第 509 行「三类值：提取器 id、`user`、`file`」自相矛盾。

**`snapshotTables` 现在被钉在 schema 上**（2026-09-03，`internal/store/snapshot_test.go`）。它定义了「重放出来的库和原库一样」里的「一样」，`Snapshot()` 的逐字节证明和 `verify --deep` 共用它，**漏一张表或一列，两个检查同时不再看它而且全绿**。两个守卫都验证过会红。这一处**没有**改成从 `pragma_table_info` 推导，虽然那样能消掉重复——**那会让守卫退化成恒真式**。两份独立写下的同一事实可以互相对照，一份推导出来的不能。

**所以「清单会漏」这条教训有两个方向**：能推导就推导（action 常量表、`DROP TABLE` 清单）；推导会让守卫变成恒真式的，就保留重复、加守卫（`snapshotTables`）。同一轮里 `TestMigrateCreatesEveryTable` 那份手抄表清单已经漏了 `op_blobs` 和 `file_tag_sources`，直接删掉由新守卫取代。

**`ops.source` 改名 `ops.origin`**（2026-09-04，见设计文档 2.2）：库里**两个不同的轴共用了同一个词**，而且在字面量 `"user"` 上撞了一次。`ops.origin` 是**哪一类**执行者（user / extractor / maintenance / sync，闭集）；`file_metadata.source` / `file_tag_sources.source` 是**哪一个**提取器（开集：提取器 id、`user`、`file`）。

**一条 `import_files` 就能证明它们不是一回事**：op 的 `origin` 是 `user`，而它写下的 `file:size` / `file:mtime` 两行的 `source` 是 `file`。任一方向都推导不出另一方，所以**合并是不可能的，只能改名**。

**否掉了「`origin` 也存提取器 id」**：`PropagatesUpdatedAt()` 在 `rebuild` 重放时要跑，存 `"exif"` 就得查一张住在 Go 里、会随版本变、卸载插件后就没有那一项的映射表——重放出来的库会取决于当时装了哪些插件，不变量 3 不再成立。**版本同理不进列**：它已经在 payload 里，而按不变量 4 那正是它该待的地方。

配套补了两个守卫（都验证过会红）：`TestEveryOriginDeclaresWhetherItPropagatesUpdatedAt` 用 ast 扫常量，取代原来手抄的三个——**那份手抄清单漏的正是 `SourceSync`，唯一一个答案预期会变的值**（注释自己写着「{user} today, {user, sync} 之后」）。以及 `write()` 现在校验 origin，和它早就在做的 `oplog.Lookup(action)` 对称。这是「手抄清单会漏」的第四次。

顺手修了一处我自己造成的文档破损：上一轮移动 1.7.7 时用 `\n#### ` 找边界，跨过了三个井号的 2.1 / 2.2，把 `## 2.` 的标题挪错了位置。

**`box_id` 从 `files` 和 `folders` 上删掉，`boxes` 加 `root_folder_id`**（2026-09-04，见设计文档 1.4 / 1.5）：一个文件属于哪个 box，原来有**两份说法**——它自己那一列，和它的 folder 链——而两份说法就会对不上。现在只有一份：folder 链走到哪个根、那个根被哪个 box 认领。

**去掉的那一列是按推理加的，不是按实测加的。** 原来的理由是「`Query.Boxes` 是最热的筛选路径」，但真正的读者只有 `find --box` 一处（`kist ls` 走 `folder_id`），而「最热」说的是还不存在的 M4 网格视图。仓库在别处的惯例正相反：tag 闭包表、locale 排序都写着「实测到需要再上」。

换来两样东西：**跨 box 移动目录从 O(文件数) 降到 O(子孙目录数)**；以及**一整类损坏变得不可表示**——`FilesWithBoxMismatch` 和 `CrossBoxFolders` 两条检查直接删掉，因为漂移需要两份说法。取而代之的 `BoxesWithBadRoot` / `UnclaimedRootFolders` 扫的是 `boxes` 表，失败面从 O(文件数) 降到 O(box 数)。

代价是按 box 筛要一次递归下降（`store.FoldersOfBoxSQL`），形状照抄 `TagDescendants`，`UNION` 不是 `UNION ALL`。**实测过 SQLite 和 PG 都允许 `WITH` 出现在 `IN (...)` 子查询里**，所以 `translate()` 仍然只发出一个可拼接的 `AND` 片段。

**❌ 否掉「Go 结构体上保留 `BoxID`，由 join 填充」**：那只是把冗余从表挪进内存。`wire.File.box_id` 改成和 `path` 一样按需解析，两者共用 `library.FileLocation` 的同一趟向上走——而 `FilePath` 反而变便宜了，它原来读整个 box 的 folder 表才能解析一条路径。

**外键方向从此是 box → folder，和 file → folder 相反。** 所以 `create_box` 先插 folder 再插 box、`delete_box` 先删 box 再删 folder、`ClearViews` 里 `boxes` 排在 `folders` 前面。三处都有测试钉着。

**还没量过的是新方案的代价**（记进 open-questions）：4000 文件的测试库上量不出来，要造十万行的合成库。真慢的话加回来是一次 `rebuild` 不是数据迁移。

**op 加了组标签 `ops.root_op_id`**（2026-09-04，见设计文档 2.5.1）：把**同一条命令写出来的若干 op** 串起来，后面几条指向第一条，第一条自己是 NULL。`kist import a/ b/ c/` 今天就是三条 op，代码注释自己写着「an import of three directories is three ops」——所以这个标签有现成的用户，不是给将来铺的空管道。

**第一版铸了一个 UUID `group_id`，当场被自己推翻。** 写完立刻给 `log` 加了 `--group <op-id>`，因为没人愿意抄一个界面从不打印的 UUID——**那个 flag 本身就是证据**：如果组的自然名字是一条 op 的 id，键就该是 op 的 id。换成 `root_op_id` 省掉一个 id 类型、一个构造函数、一个 BLOB 扫描辅助函数，显示反而更好——`(with 2)` 既是标记也是要传回去的参数。教训和「两端能复用」那条同形：**先量一量你造的那个新标识符，到底谁会去读它。**

**第一条为什么是 NULL 不是自指**：`ops.id` 由 AUTOINCREMENT 在插入时给出，写它的时候还不知道；补自指要 UPDATE 一行 op，而 ops 从不更新。代价是「这条是不是组头」要一次反向查询，形状和 `reverts_id` / `RevertedBy` 一模一样。

**组头是第一条真的写出来的 op，不是第一个参数**：`kist import empty/ a/ b/` 里空目录写不出 op，所以组头是 `a/` 那条。哪一条当头要跑过才知道，这也是组不能预先铸 id 的另一面。

**是列不是 payload**，理由和 `reverts_id` 一样：它说的是这条 op **怎么来的**，不是它**做了什么**。`Apply` 看不见它，所以 `rebuild` 不受影响。有测试钉着（写两条 op，第二条一次带组一次不带，物化状态必须逐字节相同，验证过会红）。

**否掉三条**：① 给组建一张表——组没有属于自己的属性，「这是导入还是删除」从成员 action 就能看出来，**没有属性的组不是实体**；② 撤销时继承组——`reverts_id` 已经连起来了，再套一层会把「一起产生的」和「有关系的」混成一个词；③ **把组做成原子撤销单位**——一个必须整体撤销、也只能整体撤销的组，跟一条 op 的唯一区别是**它能被观察到只应用了一半**，严格更差。原子单位已经有了，它叫 op。

**这一轮不做批量撤销**：撤一个分批导入要倒着撤 N 条 `delete_files`，任何一条被 tag 挡住就会**撤了一半然后卡住**，正好落回 2.5 要拦的事。要做必须先全部预检再动手，是新机器。

顺带把 `write()` 的尾参改成 `writeOptions` 结构体——`revertsID` 和 `rootOpID` 两个可空指针并排传，调用点 `..., nil, nil)` 说不清哪个是哪个。

**`update_box` 的 payload 改成只记改动过的字段**（2026-09-05，见设计文档 2.3）。规则一句话：**`Apply` 断言的范围，必须恰好等于它写入的范围。**

原来 payload 记全部四个字段（`Before` / `After` 两个完整 `BoxFields`），于是断言也必须比全部四个——**改名 op 撤不掉，只因为别人后来改了说明**。

**只改一半是错的，两个方向都实测过**：只放宽断言不收窄写入，撤销改名会**静默抹掉**别人写的说明（2.5 要拦的正是这个）；只收窄写入不放宽断言，撤销仍被挡住，而错误信息在叫用户去撤销一段他想留着的说明——安全，但是死胡同。

**❌ 否掉「不改 payload，从 `Before.X == After.X` 推导」**：推导可靠（CLI 未设置的 flag 就填当前值，两侧相等恰好等价于没改过），也符合仓库「能推导就推导」的惯例。但它把「这条 op 改了什么」存成了**要两次比较才读得出来**的形式，而那正是 payload 该直说的事。多两个类型（`TextChange` / `DateChange`），换 payload 自己就是答案。

**`delete_box` 保持总断言**：update 只写它声明改的那几列，delete 是把整行拿走，所以必须对整行负责。同一条规则的两个结论。

**这是一次 op log 的破坏性改动**：旧的 `update_box` payload 用新结构解不出来。`Payload[T]` 开了 `DisallowUnknownFields`，所以失败是**响的**（`unknown field "before"`）不是哑的。1.0 之前免费，之后不免费。

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
