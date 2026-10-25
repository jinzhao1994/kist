# 离线客户端最终验收审计

状态：实现与逐项验收完成，未发布版本。要求编号对应 [Goal 任务说明](2026-10-21-offline-client-goal.md) 的章节与条目。实施记录中的历史成功不自动证明当前全部要求；下表保留证据入口、验证范围及其限制。

## 需求映射

| 要求 | 当前证据入口 | 审计结论与下一步 |
| --- | --- | --- |
| 一.1、五.5–6：NAS 唯一权威、library 写入口、非 SQLite 合并 | `client_sync.go`、`sync.go`、`sync_envelope_test.go`、架构守卫 | 已核对日志后事务、接纳回执重建与 view drift 检测；完整 make check 已包含架构守卫与这些恢复测试。最终远端检查见下方交付项。 |
| 一.2–3、七.1–4：旅行离线编辑、NAS 独立写、最终合并 | `internal/api/client_journey_test.go`、真实 Chrome 合成双实例记录 | 已有真实 HTTP、部分缓存、离线导入/标签/两层组合、NAS 独立修改、重开客户端、重复同步及 rebuild/deep verify 的组合验收。该用例用 HTTP 503 模拟离线，不等同进程强杀。 |
| 一.4、四.1–3：按需原件、完整目录、未缓存语义 | `client_catalog_reads_test.go`、`client_search_test.go`、`client_content_test.go` | 已直接复核未缓存原件仍可搜索/标注，完整目录边界确认前不发布半截目录；App 挂载 ClientCatalogNotice 明示不完整。历史视图按已确认前缀读取。`client_cache_content_test.go` 区分 ErrNotCached 与 ErrIntegrityMismatch，并验证坏内容不发布、缺失缓存可重下。 |
| 一.5：手动连接/同步/冲突 | `OfflineClient.tsx`、`ClientConflictEditor.tsx`、双语 `docs/offline-client*` | 真实网页操作已验收；自动同步、多主、手机后台不在本次要求内。专用编辑器不是每种 action 的必备条件，但每种允许的离线写操作都必须有正确 patch 与显式冲突语义。 |
| 二.1–2、五.4：独立 pending、稳定身份与可恢复决定 | `client_queue_test.go`、`client_request_test.go`、`client_local_history_test.go` | 已复核追加日志、断写、跨句柄、决定重试与原意图保留。`TestPendingQueueUncertainSendAndConflictBlockLaterIntents` 明确拒绝放弃/替代不确定提交，并在已确认拒绝后仍阻止后续意图越过替代操作。 |
| 二.3–5：按本地顺序 patch、两类时间、正式 ID 排序 | `sync_envelope_test.go`、`sync_export_test.go`、`client_history_test.go`、`client_history_view_test.go` | 已复核 `applyOp` 用 accepted_at 构造 ViewTx，普通写入/追赶/rebuild 共用此路径；journal 测试验证日志按接纳时间分桶。`history_acceptance_test.go` 验证 NAS、rebuild、客户端均按正式 ID 排序并保留原始时间；旧库迁移新增测试验证时间倒序时的分页。历史回溯按正式前缀，已有客户端前缀测试；完整撤销/动作审计仍在后续条目。 |
| 二.6–7、五.9：origin、凭据和操作身份分离 | `sync_access_test.go`、`internal/api/sync_test.go`、`sync_test.go` 协议校验 | 已直接复核 user/extractor 保留、配对不写归档事件；旧 origin=sync 随迁移保留，新提案 origin=sync/未知动作/不兼容版本拒绝且 op 数不增加。凭据摘要持久化、跨句柄撤销即时生效，客户端状态不暴露 token。 |
| 三.1–5：合并前缀、冲突停止、保留意图和依赖 | `client_overlay_test.go`、`client_queue_test.go`、`client_sync_test.go` | 已有竞争修改/解决和不确定结果证据；完整动作覆盖不能仅由通用路径推断。 |
| 三.6：每种离线写操作的 patch/幂等/冲突/撤销 | `sync_test.go` 的注册表覆盖；`client_{commands,tags,saved_searches,file_commands,extraction,import,deletion}_test.go` | 逐项审计已发现并修复 box tag 并发重复增删的空修改可撤销问题，增加同步、严格撤销和 rebuild 证据；文件标签重复移除问题也已复现并按来源增加新写前校验，回归验证其他来源保留、严格逆操作和 rebuild；其余动作已与真实 handler/逆操作及具体回归逐族核对，详见动作审计记录；发现的提取版本与删除分类问题已修复并通过完整检查。 |
| 三.7、七.7：历史、严格撤销、操作组与映射 | `client_undo_test.go`、`client_import_test.go`、本地历史及删除冲突测试 | 已有本地撤销、接纳后撤销、批次、正式映射和替代意图测试；已核对正式历史与 LocalHistory 的 Undo API 入口、API 幂等回归及本地/正式引用边界；补齐撤销事件再次撤销的本地入口，真实 Chrome 桌面/窄屏确认界面通过，正式视觉 39707088420 完整采集并审阅，接受流程 39708286364 已成功。 |
| 三.8：fileset 角色/主文件/页序/嵌套/删除依赖 | `client_commands_test.go`、`client_deletion_test.go`、`client_conflict_deletion_test.go`、`client_offline_test.go` | 已有配置竞争、嵌套媒体依赖、成员删除与严格恢复；真实浏览器删除后恢复 NAS 新增文件通过。 |
| 四.4–7、七.6：TTL/LRU、配额、保留、活跃使用 | `client_cache_test.go`、`client_cache_workspace_test.go`、`client_offline_test.go` | 已直接复核 cache 与 workspace 测试：原件/派生统一计量；中间文件计 PartialBytes、剩余预约计 ReservedBytes，跨句柄不超售；活跃读者/转换租约不可逐出，超预算转换取消且发布前再次检查。TTL/LRU 与保留选择既有验收见实施记录。 |
| 四.8：未同步原件与操作不逐出、空间不足失败 | `client_storage_test.go`、`client_cache_adopt_test.go`、`client_cache_rebuild_test.go` | 已复核 ProtectedStorage 单独计量、极小 TTL/容量和 prune 不改未确认原件/pending。PutOriginal 直接返回 blob 写入/fsync 错误；临时文件失败只清自身，不删既有原件。Stage 在原件持久化前拒绝且不入队；pending 继续使用 append/fsync 日志。不将逻辑配额测试声称为物理满盘试验。 |
| 四.9：组合所需成员全部准备 | `client_offline_test.go`、`client_offline_previews_test.go`、`client_offline_prepare_test.go` | 已有递归 Live Photo/字幕、独立主文件、共享内容配方并集、准备期间成员变化检测。 |
| 四.10、七.10：网页资源与离线预览 | `cmd/kist/client_serve_test.go`、`internal/api/client_preview_test.go`、真实 Chrome 媒体验收记录 | 已核对 client serve 不接触 NAS 即启动，使用 web.Handler 嵌入 static/build，PDF worker 及上传 worker 从本地构建加载；真实离线浏览器记录验证预览。所需预览缺失时准备不能标为完成，转换工具及地图外部底图限制在双语指南明确说明。 |
| 五.1–3、七.5：续传、校验、接纳前落盘、丢响应/崩溃 | `internal/syncclient/{http,crash}_test.go`、`sync_process_crash_test.go`、`internal/upload/session_test.go` | 已补真实客户端四阶段和 NAS 三阶段 Process.Kill 验收；共享上传内核另有写入/fsync/进度保存边界的子进程退出测试。详见下方故障矩阵；本地 race 及 08bf088 跨平台 CI 39702009286 通过。 |
| 五.7：持续写入中的一致目录边界 | `sync_export_test.go`、`client_test.go` | 已直接复核 TestSyncCatalogFixedBoundaryPreservesEnvelopes：固定 head 后另一句柄新增 NAS 操作，分页仍截止原 head，保存 v2/v3 信封，rebuild 后导出字节相同；错误库身份/head 摘要拒绝。部分目录重开及重复页继续下载回归通过。 |
| 五.8：客户端专属完整性与存储边界 | `client_test.go`、`client_cache_rebuild.go`、NAS 拒绝客户端目录路径 | 本轮发现并修复派生映射丢失导致重建计量失败；未知版本仍拒绝覆盖，残留字节不能伪装成可用预览。已复核 TestClientPartialCatalogResumeAndLocalPersistence 中 Open(clientRoot) 返回 ErrNotALibrary，并在删除 client.db 后从独立目录/pending 权威记录恢复。 |
| 五.9、七.9：配对/撤销授权、协议不兼容、旧库升级 | `sync_access_test.go`、`client_connection_test.go`、`sync_test.go`、store 迁移测试 | 逐条协议拒绝测试存在；新增 `store/TestSyncMigrationPreservesOldHistoryAndIndexesNewReceipts` 从实际 schema 7 升级，保留历史 origin=sync、原时间/payload/请求索引，不伪造回执；新回执、snapshot 与重复迁移/重开均通过。fbf7985 的 CI 39703093445 全平台通过。 |
| 五.10：不引入无关分布式框架 | 模块依赖与架构守卫 | 沿用 Go/SQLite/普通文件和现有锁；本阶段起 go.mod/go.sum 无新提交，架构依赖守卫随完整 make check 通过。 |
| 六：中文设计、分阶段交付、检查、不发布 | 权威离线设计、实施记录、main 提交与 Actions、双语使用指南 | 已持续分阶段提交推送，无 PR/发布；最终 main CI、正式视觉接受后的 CI/Visual 必须核对。 |
| 七.8：NAS rebuild/snapshot/deep verify 保证 | `sync_envelope_test.go`、`client_journey_test.go`、历史视图测试 | 已复核 store.Snapshot 包含 op_sync 的四列，envelope 和迁移回归比较完整快照；篡改同步索引触发 deep verify ViewDrift，rebuild 恢复，原始时间与接纳时间保持。旅行流程另有端到端 deep verify。 |

## 交付验证

- 最终实现已通过本地完整 `make check`（296 项前端测试及 Go race/架构/构建检查），日志 `/tmp/kist-local-redo-complete-check.log`。
- ad57d3e 的 CI 39705965237、Visual 39705965158、Secret scan 39705965168 全部成功。
- b6d92bc（包含 39ee967 的最终 UI）的 [CI 39707088422](https://github.com/jinzhao1994/kist/actions/runs/39707088422) 和 Secret scan 39707088370 成功，包括 Linux、macOS、Windows 与双架构容器检查。
- [正式视觉 39707088420](https://github.com/jinzhao1994/kist/actions/runs/39707088420) 完整采集基准 154 / 当前 156 场景，无跳过、异常或 flaky。154 个旧场景全部 MATCH；仅新增桌面/手机本地历史撤销确认两张 NEW，已全尺寸查看。该比较任务因新增场景返回 failure，不是采集失败。
- [接受流程 39708286364](https://github.com/jinzhao1994/kist/actions/runs/39708286364) 成功，生成基准提交 3efe6ba。文档收尾提交须通过最终 main 的 CI、Visual 与 Secret scan 后才关闭 Goal；不创建 PR，不发布版本。

七个真实强杀边界已在 08bf088 跨平台 CI 39702009286 通过，旧库升级已在 fbf7985 跨平台 CI 39703093445 通过；这些测试不代表任意机器指令位置或整机掉电试验。之前菜单存在间歇栅格差异，多个完整复验已绿色；不宣称已查明根因，不放宽阈值或遮罩。

## 真实进程终止故障矩阵

以下均使用合成数据，不触碰用户归档。强杀测试由父进程等待明确检查点后调用 `Process.Kill`，不执行子进程的关闭清理。

| 进程与边界 | 检查点及恢复断言 | 证据 |
| --- | --- | --- |
| 客户端：上传块已保存，响应未交付 | 9 MiB 文件上传第一块后强杀；重开只发送剩余块，偏移恰为 0、8388608 | `TestHTTPClientRecoversAfterProcessKill/upload-block` |
| 客户端：原件安装完成，响应未交付 | NAS 已验证完整原件；客户端重开后接续提交，不重复上传块 | 同测试 `original-installed` |
| 客户端：NAS 已接纳，回执未交付 | 重开重复同步仍只产生一个正式 op，本地映射恢复 | 同测试 `nas-committed` |
| 客户端：本地确认已持久化 | 同步返回后、Close 前强杀；恢复已接纳历史、原始时间与正式映射 | 同测试 `local-confirmed` |
| NAS：原件临时复制中 | 拷贝一半后强杀，不发布不完整原件；重传后完整字节一致、没有归档 op | `TestSyncNASProcessKillDuringOriginalPlacement/original-copy` |
| NAS：原件安装完成 | 安装后强杀；重开可读，重复安装幂等、没有归档 op | 同测试 `original-installed` |
| NAS：日志 fsync 后、数据库 apply 前 | 重开追赶正式 op；重复接纳与 rebuild 保留同一回执；deep verify 仅报告预期投影缺失，relink 后无问题 | `TestSyncNASProcessKillAfterDurableAppend` |
| 共享上传内核：write、数据 fsync、进度保存前后 | 子进程 `os.Exit(71)`，重开截断到确认偏移、从该偏移续传并校验完整内容；同步上传 API 复用该内核 | `internal/upload/TestProcessDeathAtDurabilityBoundaries` |

客户端四场景 race 9.339 秒、NAS 三场景 race 2.766 秒通过。完整检查及跨平台 CI 记录见实施进度；不以响应中断或正常 Close/reopen 代替以上真实终止证据。

## 动作审计记录

- 归档盒标签增删：原 Apply 的幂等行为不足以保护新写逆操作，已用 `validateWriteState` 增加存在性前提。真实双实例竞争、同身份回执重试、严格撤销阻挡与解除、rebuild 已验证；修复提交 15386ec。
- 文件标签增删：添加同来源由唯一约束拒绝；移除同来源的重复操作已复现为空修改，补精确来源存在性校验。多来源同时认领时只移除/恢复目标来源，其他来源保留。完整 make check 已通过（`/tmp/kist-file-tag-conflict-complete-check.log`），3450e06 的 CI 39705064048、Visual 39705064543、Secret scan 39705063922 全部通过。
- 保存搜索 create/update/delete：已直接复核 `oplog/saved_search.go` 的 canonical 校验、完整 Before 比较、同名检查以及 Inverse 前后交换。`client_saved_searches_test.go` 真实流程覆盖请求重试、离线删除与本地撤销后同步、NAS 独立更新冲突、rebuild 状态一致。不存在依赖墙钟选胜者的路径。

下列记录结合协议注册表对照覆盖当前支持的全部领域动作；各项具体失败和修复记录保留在实施进度中。

- 导入/覆盖/撤销：直接复核 `oplog/file.go`、`upload.go` 的身份/位置/完整替换快照、标签及目录非空约束，原件在接纳前验证；`client_import_test.go` 验证部分缓存下新增、同身份覆盖、pending 重命名后覆盖、操作组映射、严格批次校验、NAS 新标签阻止旧覆盖及接纳后撤销。
- 删除/恢复/嵌套引用：直接复核 box_contents/entry/fileset_entry 的完整快照比较、父链与名称检查及同事务引用变化；`client_deletion_test.go` 验证本地重命名后删除、NAS ABA 修改拒绝、整盒离线删除恢复、多页内 Live Photo 删除及撤销/rebuild。`client_undo_test.go` 验证本地/正式引用映射、撤销与重做、后续修改阻挡及失败不入队。
- 提取/metadata：直接复核字段 before、来源、拍摄时间与 Inverse 版本交换；审计发现仅字段检查无法发现“字段相同、版本已升级”的竞争，已复现并增加 PreviousVersion 新写校验。针对性竞争、撤销、rebuild 和最终完整检查均通过；远端 CI 39705965237 已通过。

- 标签删除冲突分类：`deleteTagOp.Apply` 在 NAS 新增子标签或挂载后返回 `ErrTagInUse`，原先 `clientRebaseConflict` 和 AcceptSync 冲突归类未列出此错误。已真实双实例复现并补齐这两条路径的分类；回归覆盖子标签、box 挂载两种竞争，Status 仍可读，Resolve 保留 NAS 结果后恢复同步且不改变 NAS op 数。针对性测试通过，最终完整检查 `/tmp/kist-extraction-and-tag-conflicts-final-check.log` 已通过；远端 CI 39705965237 已通过。

- box/folder/tag 的创建、更新、空对象删除与改名/移动：已直接核对各 handler 的字段 before、父节点/名称/子对象/引用约束和 Inverse；`client_commands_test.go` 验证独立字段合并、目录所有权和重复名称，`client_tags_test.go` 验证树约束和移动合并 NAS 独立改名。set_fileset 以完整配置比较，不对页序逐行合并；引用校验与嵌套删除回归已核对。协议能力清单由 TestSyncActionCoverage 对照实际注册表，内部 request_receipt 排除。
- 本地历史 UI 可达性：`LocalHistory.tsx` 原先用 `!op.undo` 隐藏撤销事件的撤销按钮，而客户端 Undo API 与 `TestClientUndoLocalThenRedoAfterNASAcceptance` 已允许对撤销事件再次撤销（重做）。已移除多余隐藏条件，两个状态的组件回归先失败后通过，保留确认与幂等行为；新增桌面/手机正式视觉场景已完整采集审阅，完整检查已通过（`/tmp/kist-local-redo-complete-check.log`），真实 Chrome 两种尺寸已查看；正式视觉接受流程 39708286364 已通过。

### 2026-10-24：在线浏览按需缓存补充

- 新增原件与元数据提取的在线缓存缺失测试，覆盖真实认证远端、凭据撤销、断连后缓存命中以及未缓存内容的 `not_cached` 响应。
- 补充未提取媒体事实时的扩展名预览选择，以及组合显示名称与主文件名分离的契约断言；连接状态组件覆盖连接正常、不可达、凭据失效、库不匹配和未配置。
- 本地 Chrome 验证 `19604/files` 的 8 张缩略图全部加载（包含嵌套多页、实况与视频字幕组合），无页面脚本错误；未缓存原图的详情页也能自动下载并显示。浏览验证仅填充缓存，不执行目录同步、批量提取或提交待同步修改。
- 本轮按用户要求仅做本地验证，不运行或等待 GitHub 工作流；正式视觉基准未更新。
- 最终 `GO_TEST_TIMEOUT=20m make check` 通过（前端契约、格式、测试、构建、Go vet、全量 race 测试与单二进制构建）。

### 2026-10-24：单向自动同步

- 回归覆盖纯下载、连续上传、双向暂停、提交前远端新增操作、接收期间本机新增操作、已提交响应丢失后手动恢复，以及既有回执优先于过期边界检查。
- 真实 HTTP 测试运行客户端后台 worker，确认启动时自动获取远端操作、网页写入立即唤醒条件上传，并在阻塞上传期间读到 `syncing=true`。
- 前端覆盖「同步中…」「已同步」「需要手动合并」与零计数隐藏，视觉场景补充同步中状态。此次仍按用户要求仅在本地工作，不运行或等待 GitHub 工作流。
- 本地 Chrome 验证：客户端启动后自动下载原有的 33 个远端操作，状态变为「已同步」，全部文件列表显示新增文件并加载预览；未手动触发同步。
- 最终 `GO_TEST_TIMEOUT=20m make check` 通过，包含 316 个前端测试、API 契约、格式、构建、Go vet 和全量 race 测试；正式视觉基准未更新。

### 2026-10-24：页面检查立即唤醒同步及方向进度

- 页面检查发现只有单向待同步操作时，调用独立 POST 唤醒服务 worker；GET 检查仍只读，后台照常重新校验边界，双方有修改或冲突时不触发自动合并。
- 同步标记读取后端当前阶段与剩余操作数，显示「正在同步下载 N 个操作」或「正在同步上传 N 个操作」；准备阶段保留通用提示，同步期间隐藏重复的待同步计数。
- 回归覆盖页面检查后的立即下载、后台阻塞上传的阶段与数量、任务结束清理，以及前端单向唤醒与双向/断连/冲突/进行中不唤醒；视觉 fixture 与断言同步更新。
- 最终 `GO_TEST_TIMEOUT=20m make check` 通过（325 个前端测试及全量 Go race 检查）；本地预览已更新并在 Chrome 确认连接和完成状态正常。本轮不推送 GitHub，正式视觉基准未更新。

### 2026-10-25：离线操作记录与回收站

- 断线时隐藏未知待下载数量，保留已知非零待上传数量。本机已生效但未确认的导入、删除等操作进入普通操作记录；未撤销的删除进入回收站，沿用本机严格撤销和幂等请求恢复。
- 新增只读分页接口使用独立本机操作 ID，按既有索引过滤归档盒、文件、组合与回收站；同步确认后由正式历史替代。已确认删除被本机恢复时，立即从客户端回收站隐藏。没有数据库结构或持久化格式变化。
- library/API 回归覆盖分页、范围过滤、离线导入与删除、恢复、撤销引用、确认后不重复，以及本机恢复已确认删除；前端覆盖本机回收站恢复请求及空状态切换。`make check` 通过（346 个前端测试及全量 Go race 检查）；最终样式调整后前端完整检查再次通过。
- 保持 19603 远端关闭，更新 19604/19605 客户端。真实 Chrome 只读检查确认新 PDF、删除的 PDF 和新增归档盒均出现在普通历史，删除项出现在回收站，侧栏显示未连接及 5 个待上传操作；没有恢复用户删除或触发同步。
- 本轮仅本地提交，不推送 GitHub，不修改正式 Linux 视觉基准。

- 最终本机 Chromium 完整候选 176 个场景通过，新增离线历史／回收站场景已审阅。补齐客户端状态 fixture，并让画廊分页截图等待 PDF 阅读器就绪；画廊请求计数一次波动后单独复核及完整重跑均通过。Mac 截图仅作本地补充验证，不冒充正式 Linux 基准。
