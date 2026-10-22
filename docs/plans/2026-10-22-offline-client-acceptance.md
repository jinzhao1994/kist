# 离线客户端最终验收审计

状态：进行中，不能据此宣布 Goal 完成。要求编号对应 [Goal 任务说明](2026-10-21-offline-client-goal.md) 的章节与条目。实施记录中的历史成功不自动证明当前全部要求；下表保留证据入口和仍需核验的范围。

## 需求映射

| 要求 | 当前证据入口 | 审计结论与下一步 |
| --- | --- | --- |
| 一.1、五.5–6：NAS 唯一权威、library 写入口、非 SQLite 合并 | `client_sync.go`、`sync.go`、`sync_envelope_test.go`、架构守卫 | 已有日志后事务、接纳回执重建与 view drift 检测证据；最终需随当前提交完整检查核对。 |
| 一.2–3、七.1–4：旅行离线编辑、NAS 独立写、最终合并 | `internal/api/client_journey_test.go`、真实 Chrome 合成双实例记录 | 已有真实 HTTP、部分缓存、离线导入/标签/两层组合、NAS 独立修改、重开客户端、重复同步及 rebuild/deep verify 的组合验收。该用例用 HTTP 503 模拟离线，不等同进程强杀。 |
| 一.4、四.1–3：按需原件、完整目录、未缓存语义 | `client_catalog_reads_test.go`、`client_search_test.go`、`client_content_test.go` | 源码/测试入口已定位；最终审计仍需逐项复核未完成目录提示、历史版本和原件校验失败的区分。 |
| 一.5：手动连接/同步/冲突 | `OfflineClient.tsx`、`ClientConflictEditor.tsx`、双语 `docs/offline-client*` | 真实网页操作已验收；自动同步、多主、手机后台不在本次要求内。专用编辑器不是每种 action 的必备条件，但每种允许的离线写操作都必须有正确 patch 与显式冲突语义。 |
| 二.1–2、五.4：独立 pending、稳定身份与可恢复决定 | `client_queue_test.go`、`client_request_test.go`、`client_local_history_test.go` | 已复核追加日志、断写、跨句柄、决定重试与原意图保留。`TestPendingQueueUncertainSendAndConflictBlockLaterIntents` 明确拒绝放弃/替代不确定提交，并在已确认拒绝后仍阻止后续意图越过替代操作。 |
| 二.3–5：按本地顺序 patch、两类时间、正式 ID 排序 | `sync_envelope_test.go`、`sync_export_test.go`、`client_history_test.go`、`client_history_view_test.go` | 已复核 `applyOp` 用 accepted_at 构造 ViewTx，普通写入/追赶/rebuild 共用此路径；journal 测试验证日志按接纳时间分桶。`history_acceptance_test.go` 验证 NAS、rebuild、客户端均按正式 ID 排序并保留原始时间；旧库迁移新增测试验证时间倒序时的分页。历史回溯按正式前缀，已有客户端前缀测试；完整撤销/动作审计仍在后续条目。 |
| 二.6–7、五.9：origin、凭据和操作身份分离 | `sync_access_test.go`、`internal/api/sync_test.go`、`sync_test.go` 协议校验 | 用户操作仍为 user，授权不写归档事件；最终核对旧 sync origin 兼容与能力拒绝。 |
| 三.1–5：合并前缀、冲突停止、保留意图和依赖 | `client_overlay_test.go`、`client_queue_test.go`、`client_sync_test.go` | 已有竞争修改/解决和不确定结果证据；完整动作覆盖不能仅由通用路径推断。 |
| 三.6：每种离线写操作的 patch/幂等/冲突/撤销 | `sync_test.go` 的注册表覆盖；`client_{commands,tags,saved_searches,file_commands,extraction,import,deletion}_test.go` | 入口已定位；需把真实注册表与实现策略逐项复核，不能把覆盖清单等同语义测试。 |
| 三.7、七.7：历史、严格撤销、操作组与映射 | `client_undo_test.go`、`client_import_test.go`、本地历史及删除冲突测试 | 已有本地撤销、接纳后撤销、批次、正式映射和替代意图测试；最终复核 UI 可达性与引用边界。 |
| 三.8：fileset 角色/主文件/页序/嵌套/删除依赖 | `client_commands_test.go`、`client_deletion_test.go`、`client_conflict_deletion_test.go`、`client_offline_test.go` | 已有配置竞争、嵌套媒体依赖、成员删除与严格恢复；真实浏览器删除后恢复 NAS 新增文件通过。 |
| 四.4–7、七.6：TTL/LRU、配额、保留、活跃使用 | `client_cache_test.go`、`client_cache_workspace_test.go`、`client_offline_test.go` | 针对性测试与真实网页选择管理已通过；需最终确认原件、预览、中间文件均计入预算。 |
| 四.8：未同步原件与操作不逐出、空间不足失败 | `client_storage_test.go`、`client_cache_adopt_test.go`、`client_cache_rebuild_test.go` | 已有未确认内容保护和确认迁移测试；磁盘不足/持久化失败路径还须按源码和故障测试核验。 |
| 四.9：组合所需成员全部准备 | `client_offline_test.go`、`client_offline_previews_test.go`、`client_offline_prepare_test.go` | 已有递归 Live Photo/字幕、独立主文件、共享内容配方并集、准备期间成员变化检测。 |
| 四.10、七.10：网页资源与离线预览 | `client_serve_test.go`、`client_preview_test.go`、真实 Chrome 媒体验收记录 | 本地二进制提供网页、NAS 不可达仍启动；地图背景等非必要外部资源有明确离线限制。最终检查打包资源与预览工具依赖。 |
| 五.1–3、七.5：续传、校验、接纳前落盘、丢响应/崩溃 | `internal/syncclient/{http,crash}_test.go`、`sync_process_crash_test.go`、`internal/upload/session_test.go` | 已补真实客户端四阶段和 NAS 三阶段 Process.Kill 验收；共享上传内核另有写入/fsync/进度保存边界的子进程退出测试。详见下方故障矩阵；本地 race 通过，跨平台结果仍待本阶段 CI。 |
| 五.7：持续写入中的一致目录边界 | `sync_export_test.go`、`client_test.go` | 固定 head、分页及部分下载恢复入口已定位；最终核验新写入不混入已确定前缀。 |
| 五.8：客户端专属完整性与存储边界 | `client_test.go`、`client_cache_rebuild.go`、NAS 拒绝客户端目录路径 | 本轮发现并修复派生映射丢失导致重建计量失败；未知版本仍拒绝覆盖，残留字节不能伪装成可用预览。最终检查普通 NAS 命令拒绝误开。 |
| 五.9、七.9：配对/撤销授权、协议不兼容、旧库升级 | `sync_access_test.go`、`client_connection_test.go`、`sync_test.go`、store 迁移测试 | 逐条协议拒绝测试存在；新增 `store/TestSyncMigrationPreservesOldHistoryAndIndexesNewReceipts` 从实际 schema 7 升级，保留历史 origin=sync、原时间/payload/请求索引，不伪造回执；新回执、snapshot 与重复迁移/重开均通过。跨平台验证待本阶段 CI。 |
| 五.10：不引入无关分布式框架 | 模块依赖与架构守卫 | 沿用 Go/SQLite/普通文件和现有锁；最终 diff 审查核对。 |
| 六：中文设计、分阶段交付、检查、不发布 | 权威离线设计、实施记录、main 提交与 Actions、双语使用指南 | 已持续分阶段提交推送，无 PR/发布；最终 main CI、正式视觉接受后的 CI/Visual 必须核对。 |
| 七.8：NAS rebuild/snapshot/deep verify 保证 | `sync_envelope_test.go`、`client_journey_test.go`、历史视图测试 | 已有真实 sync envelope 索引漂移检测/重建和旅行完整流程 deep verify；最终复核 snapshot 的同步字段与时间语义。 |

## 当前明确未收口项

1. 新增进程强杀故障矩阵本地通过，仍需本阶段跨平台 CI；这些边界测试不代表任意指令位置或整机掉电测试。
2. 旧库升级、时间排序/分页及 snapshot 的证据已补齐；逐 action 的 patch/冲突/撤销语义全范围复核尚未结束。
3. 正式视觉 gate 仍有旧 `entry-menu-box` 326 像素间歇差异；不能通过放宽阈值、遮罩或把一次匹配说成修复来收口。新增历史/删除冲突页面已审阅，仍需最终完整当前-main 基准接受与绿色复验。
4. 缓存映射丢失/损坏修复的最终 make check 已通过（`/tmp/kist-derived-mapping-final-check.log`）；b325fe0 的 CI 39700016805 与 Secret scan 39700016809 均通过。

上述清单是下一步审计入口，不缩减 Goal，也不要求为每个潜在未来功能继续扩展界面。

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
