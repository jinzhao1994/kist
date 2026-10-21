# 离线客户端实施记录

完整目标与最终验收以 [Goal 任务说明](2026-10-21-offline-client-goal.md) 为准。此文件只记录实际进度，不缩减目标。

## 阶段 1：日志与时间基础（已提交并通过 CI）

- 已检查现有主库写路径、日志分桶、重放、请求幂等和 action 注册表，建立 [设计](../design/2026-10-21-offline-client.md)。
- 新增可选 v3 同步信封：独立本地操作 ID、原始意图摘要和 NAS 接纳时间；普通操作仍写 v2，旧 v1/v2 校验字节保持兼容。
- schema 8 新增可重建 op_sync 索引，纳入普通 apply、清理/重建、snapshot 和深度校验。
- 同步事件保留 ts 原始发生时间，分桶和物化变更使用接纳时间；本地旧事件行为不变。
- 相关 journal/library/store 测试通过，包括篡改、超前客户端时钟、正常提交、append 后崩溃追赶、重建一致与索引漂移发现。
- 完整 make check 已通过（前端契约/测试/构建、依赖与格式、vet、全量 race 和单二进制构建）；推送后继续核对远端 Actions。此阶段尚未提供网络接纳接口、客户端队列或网页功能。
- 提交 `3a05279`；[CI](https://github.com/jinzhao1994/kist/actions/runs/39527928633)、[Visual](https://github.com/jinzhao1994/kist/actions/runs/39527928494) 和 Secret scan 均已通过。

## 阶段 2a：NAS 接纳内核（本地检查通过）

- library 新增独立提案接纳与回执查询，写锁内执行追赶、幂等查重、依赖/组引用解析、前提验证与正式提交。
- 本地唯一 ID 与正式 op ID 显式区分，撤销在 NAS 生成实际逆操作；重试先命中回执，不因后来撤销而重新执行。
- 原文件引用在日志追加前验证实际内容哈希和物化声明大小；提取操作绑定原内容哈希。
- 同一事件中重复引用 blob 的声明必须一致，覆盖导入与恢复路径，避免末次 upsert 掩盖早先错误大小。
- 当前全部领域 action 有明确能力列表，测试对照注册表；内部 HTTP request_receipt 不作为可同步领域操作。
- 测试覆盖并发连接去重、丢失回执、重启/重建后重试、撤销映射、组引用、依赖未满足、过期删除、缺少/错误原文件、提取前提和协议输入拒绝。
- 已补 sync_conflict/sync_dependency/sync_version 稳定错误码，包含重复声明与恢复路径补测的最终 make check 已通过（`/tmp/kist-sync-accept-final-check.log`）。此阶段仍不暴露网络接口，不代表客户端已可使用。
- 提交 `6fc227c` 已推送；CI `39529898204`、Secret scan `39529898197` 已通过，Visual `39529898340` 被后续 main 检查替代并取消，继续跟进后续候选。

## 阶段 2b：正式历史分页（本地检查通过）

- 同步边界绑定库 UUID、正式 op ID 与末条校验和；每页确认原边界仍有效，期间 NAS 可以继续提交。
- 从普通日志文件读取原信封，保留旧版本、请求回执、同步身份、原始/接纳时间及校验和。
- 分页限制记录数及目标字节数；单个旧大事件不可拆分，可单独超过目标，内存上界为目标加最大单事件。
- 针对性测试覆盖并发 NAS 新增不混入固定前缀、边界变化拒绝、空库、无效游标、重建后导出一致。
- 完整 make check 已通过（`/tmp/kist-sync-export-check.log`）。
- 提交 `14f1ee0` 已推送，CI `39530461215`、Secret scan `39530461115` 已通过；Visual `39530461173` 被后续 main 检查替代并取消。

## 阶段 3a：客户端 pending 持久化（本地检查通过）

- 独立普通文件日志记录原始提案、发送状态、确定拒绝、回执、放弃与重做；无仅存于 SQLite 或进程内的队列状态。
- 开始发送先持久化；结果不确定时禁止放弃或替换。重做保留原逻辑位置，冲突及失效依赖阻止后续提交。
- 恢复与重试不重复追加，跨进程句柄在同一锁内查重，截断的最后写入可恢复，完整坏记录拒绝读取。
- 针对性 race 测试和完整 make check 均通过（`/tmp/kist-client-queue-check.log`）。
- 尚未接入客户端运行模式、领域写路径与网络传输；这些仍是后续必需工作。
- 提交 `0d74336` 已推送，CI `39531038483`、Secret scan `39531038317` 已通过；Visual `39531038406` 被后续 main 检查替代并取消。

## 阶段 3b：客户端物化重建（本地检查通过）

- 在单事务中重放 NAS 已确认日志，按逻辑顺序叠加 pending；失败操作通过 savepoint 回滚，后续停止。
- 同步身份识别已提交但丢失回执的修改；保留 NAS/本地引用映射，不把视图位置当正式 op ID。
- NAS 新写入与 pending 重建共用严格撤销、封面归属、标签循环和提取内容前提检查；旧日志重放保持原语义。
- 针对性 race 测试通过：离线撤销、丢失回执后收敛、并发改名冲突及重做、未缓存原文件仍可重建目录、后续 metadata 阻挡旧导入撤销、重建失败保留旧视图。
- 此阶段尚未开放实际客户端命令和网页入口。
- 完整 make check 已通过（`/tmp/kist-client-overlay-check.log`）。
- 提交 `3e44d7f` 已推送，CI `39531794463`、Secret scan `39531797530` 已通过。Visual `39531794540` 完整采集 134 张，仅归档盒移动列表有 8 像素差异；已查看 expected/current/diff，原因为菜单关闭后鼠标滞留触发行尾悬停态。将已有文件列表的移开鼠标处理同样应用到归档盒列表，继续核对后续完整采集。

## 阶段 3c：客户端目录与本地编辑入口（本地检查通过）

- 新增独立 CreateClient/OpenClient 内核入口与 client.json 配置，NAS Open 拒绝客户端目录。
- 固定目标分页可断点继续、重复提交；只有完成的前缀成为可见目录，重启保留未完成游标，缓存旧日志保留版本及校验和。
- Stage 在同一客户端锁内验证、持久化 pending 并重建视图；PutOriginal 将新内容保存到非缓存区域。NAS 已有内容无需本地下载，但声明必须与既有事实一致。
- 测试覆盖部分分页与重启恢复、离线创建/撤销、删除 client.db 后完整重建、缺少新原文件拒绝入队、NAS 未缓存内容复用及错误声明拒绝、v1 缓存保真；针对性 race 测试通过。
- 配置已包含缓存年龄及体积，但尚未实现逐出；命令行、网页、认证与网络传输仍未接入。
- 完整 make check 已通过（`/tmp/kist-client-runtime-check.log`）。
- 提交 `c6ff7f5` 已推送，Visual `39533227795`、Secret scan `39533227817` 已通过，移动归档盒悬停波动已修复。CI `39533227877` 仅 Windows 上传既有并发测试因 12 个请求争锁超过默认 5 秒失败，新增客户端测试均通过；测试现按协议对 BusyError 作有界重试，仍要求全部请求成功、偏移正确、内容不重复，针对性 race 连跑 3 次通过，待后续 Windows CI 复核。

## 阶段 4a：同步编排闭环（本地检查通过）

- 新增可替换传输接口与手动同步编排：能力/库身份校验、目录恢复、原文件传输、依序提交、持久回执和最终拉取。
- 独立同步锁阻止重复合并，网络操作不占用本地编辑锁；不确定提交原样重试，由 NAS 验证 ID 和摘要。
- 确定拒绝后保留冲突并刷新 NAS 状态；重做验证前提与原文件，错误方案不覆盖原意图。
- 两个隔离实例的进程内适配器测试已通过：上传中断、响应丢失与重启、并发改名、冲突重做、重复同步及完整元数据收敛。针对性 race 测试通过。
- 尚未实现 HTTP 认证/传输适配器及大文件持久续传；这些测试不能作为真实网络验收证据。
- 完整 make check 已通过（`/tmp/kist-client-sync-check.log`）。

阶段 4a 提交 `bdeba8f` 的 CI `39534020334`、Secret scan `39534020361` 已通过，Windows 并发上传重试修复已获验证。Visual `39534020326` 完整采集 134 张，132 张一致；已查看剩余两张 expected/current/diff：移动归档盒列表行尾 8 像素及盒右键菜单文字 326 像素波动，未改变视觉基准，继续核对后续采集。

## 阶段 4b：同步授权与 HTTP 元数据协议（本地检查通过）

- 新增独立可撤销凭据及 CLI 管理，服务端仅存摘要；凭据变更不写入归档历史，跨进程撤销即时读取。
- 同步端点在请求体处理前认证；能力、历史边界、分页和幂等提交有生成的 OpenAPI/TypeScript 契约。
- HTTP 明确返回确定拒绝与不确定提交；原日志 JSON 和时间精度经传输仍可验证。
- 授权、跨句柄撤销、HTTP 重试、分页校验和、CLI 针对性测试通过。完整检查发现新命令需加入统一 JSON 覆盖清单，已补充并把凭据列表改为对象封装；相关契约测试通过。
- 尚未实现 HTTP 客户端适配器和原文件续传，此阶段不代表完整联网同步已经可用。
- 接纳内核在回执查明前不返回确定拒绝，提交后读取回执失败保留不确定结果；新增争锁中断与持久化后断线测试，并让进程内适配器使用相同分类。
- 完整 make check 通过（`/tmp/kist-sync-access-final-check.log`）；针对性 race 验证通过（`/tmp/kist-sync-access-targeted.log`），包含修正后的确定争锁前提。

阶段 4b 已提交并推送 `db3877c`；Secret scan `39536023824`、CI `39536023809` 通过，Visual `39536023825` 被后续提交替代并取消。

## 阶段 4c：原文件 HTTP 续传（服务端本地检查通过）

- library 提供独立校验、安装和读取入口；缺少原文件与实际校验错误分开，传输不生成归档操作。
- 受认证保护的状态、上传会话、分块、前缀检查、完成及 Range 下载接口已接入；复用暂存内核和配额，独立暂存目录纳入清理。
- 针对性 race 测试通过：错误内容拒绝、重复安装、分块重试、服务重开后的持久偏移、前缀验证、完成后探测和范围读取。
- 尚未完成客户端 HTTP 适配器与真实 TCP 双实例测试，不能视为联网同步验收完成。
- 服务端阶段完整 make check 通过（`/tmp/kist-sync-original-check.log`）。

阶段 4c 已提交并推送 `304f963`；Secret scan `39536609304`、CI `39536609324` 通过，Visual `39536609398` 被后续提交替代并取消。

## 阶段 4d：客户端 HTTP 传输（本地检查通过）

- 独立 syncclient 包实现能力、固定目录分页、提案提交、原文件续传及 Range 下载；不直接写归档或 pending。
- 禁止重定向，非回环明文连接需显式启用；仅明确拒绝信封解除不确定状态。代理失败、截断与矛盾响应保持不确定。
- 真实 TCP 双实例 race 测试通过：NAS 独立新增、提交后断开连接、客户端重开恢复、9 MiB 原文件在首块持久化后断线、从 8 MiB 偏移继续、范围下载、重复同步及撤销凭据。
- 测试检查 NAS 最终原文件字节一致、正式操作无重复，另覆盖错误响应分类与重定向不泄露凭据。尚未完成实际客户端命令、缓存及网页体验。
- 完整 make check 通过（`/tmp/kist-sync-http-check.log`）；最终错误信封补充测试的 race 检查通过（`/tmp/kist-sync-http-targeted.log`）。

阶段 4d 已提交并推送 `1802d16`；CI `39537085473`、Secret scan `39537085389` 通过，Visual `39537085388` 正在完整采集中。

## 阶段 5a：客户端连接与命令入口（本地检查通过）

- 新增独立、原子、受限权限的连接配置；NAS UUID 不可改绑，支持凭据轮换，跨句柄立即读取新配置。
- 新增 client init/connect/sync/status；凭据从标准输入读取，状态可离线查看，不输出密钥，命令显式指定客户端目录。
- 连接与命令 race 测试通过：真实 HTTP 配对、同步本地修改、拒绝另一个 NAS、同库轮换、断网状态、凭据权限和不确定 pending 保护。
- 使用说明见 `docs/offline-client.md`，明确当前尚未接入离线网页与缓存管理。
- 完整 make check 通过（`/tmp/kist-client-connect-check.log`）。

阶段 5a 已提交并推送 `777fb1f`；CI `39537952746`、Secret scan `39537952624` 已通过。完整 Visual `39537952596` 的 134 张候选已审阅，仅移动列表正常非悬停态与旧基准不同；接受流程 `39539127849` 通过并产生 `b27656b`，已快进本地。接受后的 CI `39539164083`、Visual `39539165363` 与 Secret scan `39539166808` 全部通过。

## 阶段 6a：缓存配额与租用内核（本地验证中）

- 共享读取锁允许同一原文件并发预览；非阻塞独占锁供逐出跳过活跃对象，跨进程及进程死亡释放测试通过。
- 原文件缓存记录实际字节、已确认偏移、完成状态与最近访问；部分下载将剩余预留纳入配额。
- owner 级保留集合支持重叠选择；年龄/LRU 清理、降低配额后的超额状态、活跃租用及 pending/本地原文件保护测试通过。
- 尚未接入实际缓存下载、读取、索引恢复、fileset 离线准备和界面；这些仍为必需实现。
- 完整检查发现三种缓存错误尚未注册错误码，已补充 not_cached/cache_quota/cache_metadata_invalid；相关守卫测试通过。

## 阶段 6b：缓存下载与读取（本地验证中）

- 下载前预留容量；确认偏移前 fsync 数据，重开后截断未确认尾部，完整校验后发布缓存。
- 读取器持共享租用，关闭时更新访问时间；未缓存与实际内容损坏采用不同错误码。
- 针对性 race 测试通过：断点恢复、配额前置拒绝、损坏传输重试、活跃读取防逐出、缓存被删除后重新获取。
- 真实 TCP 测试通过：9 MiB 原文件下载在 7 字节处断线，客户端重开后续传并验证最终字节一致。
- 尚未实现缓存索引恢复、派生预览缓存及离线选择的 fileset 依赖展开，也未接入网页。
- 缓存下载与读取阶段完整 make check 已通过（`/tmp/kist-client-cache-content-check.log`）。

## 阶段 6c：缓存索引恢复与管理命令（本地验证中）

- 逐对象从已知内容事实恢复缓存记录，不覆盖未知版本；活跃租用跳过，损坏字节与缺失缓存分别报告。
- 恢复保留离线选择和 pending/local-blobs；损坏内容显式重试时可释放坏字节，避免超额损坏文件阻挡自身修复。
- 新增 client cache status/configure/prune/rebuild，支持离线使用与统一 JSON 输出；操作说明已补入使用文档。
- 针对性 race 验证通过：丢失/损坏索引、共享读取跳过、保留清单不变、受保护本地内容不变、未确认尾部截断、未知版本拒绝，以及 CLI 配额持久化。
- 阶段 6a–6c 合并后的完整 `make check` 通过（`/tmp/kist-client-cache-rebuild-check.log`，退出码 0）；等待提交后的跨平台 CI 验证。

## 后续工作

阶段 6a–6c 已提交并推送 `53e247d`；Secret scan `39541042577` 通过，CI `39541042567` 与 Visual `39541042559` 正在运行。

阶段 6d 正在实现逻辑离线选择：新增 `OfflineOriginals` 从当前客户端视图展开独立文件与嵌套 fileset，包含所有角色成员和独立主文件，按内容去重，不将同目录无关文件自动纳入。针对性测试通过，覆盖多页嵌套实况、多字幕、独立主文件、NAS 更新后重新解析、循环和缺失目标拒绝。当前只提供依赖解析，不代表选择持久化、下载及就绪状态已经完成。

- 依赖解析完整 `make check` 通过（`/tmp/kist-offline-selection-check.log`），已提交并推送 `48efdb3`。CI `39541492051`、Visual `39541492077` 尚未完成，Secret scan `39541492070` 通过。前一阶段 `53e247d` 的 CI `39541042567` 已通过。
- 继续加入逻辑选择持久化：与 owner 的内容保留列表写入同一个原子文件，空选择取消保留，无效替换不覆盖旧意图；低层直接设置内容列表会清除同 owner 的逻辑选择，避免两种意图矛盾。重开恢复、重叠保留和取消后可逐出的 race 测试通过（`/tmp/kist-offline-retention-targeted.log`）。尚需接入自动重新解析、下载、就绪状态与实际命令/网页。
- 新增 `OpenOriginal`、`PrepareOffline` 和 `OfflineStatus`：优先校验读取受保护本地原文件，再读取缓存；准备前刷新选择的内容保留列表，网络期间不持编辑锁，下载结束重新解析当前选择后判断就绪。返回目录同步状态，未缓存与校验损坏保持不同错误。针对性 race 测试通过（`/tmp/kist-offline-prepare-targeted.log`、`/tmp/kist-offline-prepare-race.log`），覆盖续传、部分内容不就绪、完全离线复查、未同步新原文件不重复占用缓存，以及下载中选择改变不误报就绪。尚未接入 CLI/网页、每次编辑后的自动保留刷新及本地原文件确认后的生命周期管理。
- `/tmp/kist-offline-retention-check.log` 的完整检查仍在运行；它启动于本轮准备流程改动之前，不能用其结果覆盖后加的准备流程，待结束后对最终代码运行完整检查。`53e247d` 的 Visual `39541042559` 已被更新提交取消，应验收最新运行。
- 上述 retention 检查已以退出码 0 完成。新增 `client offline select/list/remove/status/prepare`，已通过实际 HTTP 导入内容下载、断开 NAS 后就绪查询/复查、取消选择和统一 JSON 契约测试（`/tmp/kist-offline-cli-targeted.log`）。使用说明同步更新，明确当前就绪只涵盖原文件，自动保留刷新和网页仍待完成。开始对包含准备流程与 CLI 的最终代码执行 `/tmp/kist-offline-prepare-final-check.log` 完整检查。
- 自动保留刷新已接入所有逐出入口（手动清理和下载容量预留）：持编辑锁与缓存锁解析当前逻辑选择，失效目标保留旧集合，循环/索引损坏阻止删除。针对性 race 测试通过（`/tmp/kist-offline-refresh-targeted.log`），验证 NAS 更换成员后不调用准备/状态仍保护新成员并释放旧成员。现有完整检查启动早于该修改，完成后需对最终代码再次验证；未将运行中检查当作新改动的完成证据。
- `/tmp/kist-offline-prepare-final-check.log` 已通过（退出码 0）；包含自动刷新的最终完整检查改用 `/tmp/kist-offline-retention-final-check.log`，正在运行。
- 包含自动刷新的最终完整检查 `/tmp/kist-offline-retention-final-check.log` 已通过（退出码 0）。前一提交 `48efdb3` 的 CI `39541492051`、Visual `39541492077`、Secret scan `39541492070` 均已通过。

### 网页接入前的源码核对

离线准备阶段已提交并推送 `ec53053`，完整检查通过；Secret scan `39542563044` 通过，CI `39542563073` 和 Visual `39542563126` 仍在运行。

客户端请求幂等层已实现，待完整检查：pending v2 将请求键/摘要和操作入队原子记录，无修改请求单独记本地回执，不产生同步操作；重试稳定返回 local ID，不暴露视图序号。`Client.ExecuteRequest` 串行化回执查询与回调，`Stage` 支持受限请求作用域。针对性 race 检查通过（`/tmp/kist-client-request-final-targeted.log`），覆盖重开、同步后重试、无操作、失败后重试、过期作用域、并发句柄及入队后回调失败。业务命令生成与网页接入尚待完成。

开始共享领域命令：NAS 与客户端共用归档盒创建及字段差异 payload 生成，新增 `Client.CreateBox/UpdateBox` 与编辑锁/请求作用域协调。针对性 race 测试通过（`/tmp/kist-client-commands-targeted.log`），验证离线改名与 NAS 独立改说明合并、本地创建后修改、请求重试及无变化更新不生成同步操作。`/tmp/kist-client-request-check.log` 尚在运行，启动早于命令改动；结束后需对最终代码再次完整检查。

共享命令扩展至 fileset：NAS 与客户端共用归属、完整 before/after 和祖先信息的生成，客户端返回独立 local reference；过期配置拒绝和 NAS 并发改配置的冲突测试通过（`/tmp/kist-client-fileset-targeted.log`，race）。请求层完整检查已通过；包含共享业务命令的最终检查记录于 `/tmp/kist-client-commands-check.log`。已推送 `ec53053` 的 CI `39542563073` 已通过，Visual `39542563126` 尚未结束。

共享命令继续接入目录创建：`Client.CreateFolder` 复用 NAS 的名称规范化、同名判断、box 归属、深度与祖先生成。针对性 race 检查通过（`/tmp/kist-client-folder-final-targeted.log`），验证默认根目录、同名拒绝、跨 box 父目录拒绝和实际同步。当前完整检查启动早于目录命令改动，最终提交前仍需涵盖这些最新代码；标签等其他业务命令仍未接入。

`/tmp/kist-client-commands-check.log` 已通过（退出码 0）。冻结当前阶段代码后，对包含目录命令的最终状态运行 `/tmp/kist-client-domain-final-check.log`，完成后提交。上一提交 `ec53053` 的 CI、Visual、Secret scan 均已通过。

包含全部当前命令和请求回执的最终完整检查 `/tmp/kist-client-domain-final-check.log` 已通过（退出码 0），本阶段准备提交。当前提供请求幂等与归档盒、目录、fileset 的客户端业务入口，未将其等同于全部编辑能力或网页离线验收完成。

本阶段已提交并推送 `60d860f`；CI `39543835623` 与 Visual `39543835570` 正在运行。

### 客户端标签业务命令

新增客户端标签创建、改名、移动、删除及文件/归档盒批量添加与移除标签，共享 NAS 的命令生成和目标筛选。保持名称唯一、父节点有效、循环拒绝、非叶节点删除拒绝、user 来源和重复目标去重。针对性 race 检查通过（`/tmp/kist-client-tags-final-targeted.log`），覆盖离线移动与 NAS 独立改名合并、无修改请求回执、实际同步及同步后的严格撤销。对固定的本阶段代码运行 `/tmp/kist-client-tags-check.log` 完整检查；其他文件编辑、历史及网页仍待接入。

下一阶段源码核对：`entrySnapshot` 的 deletion revision 直接采用视图中的 `LastOpTouchingBox` 数字 ID，客户端不能直接复用该对外值；需要稳定引用/状态令牌且不弱化现有范围确认。`RemoveMetadata` 写 tombstone 并在生成 payload 时按库时区重算 captured_at，客户端必须沿用该规则，不能直接删除行。`Undo` 错误中的目标/阻挡操作也需要转换为稳定 NAS/local 引用。

标签阶段完整 `/tmp/kist-client-tags-check.log` 已通过（退出码 0），准备提交。网页文件详情已有 `fileReader` 读取接口可复用，客户端内容句柄必须保留缓存租用，不能直接套用 NAS 的 `*os.File` 假设。

标签阶段已提交并推送 `df2e928`；CI `39544479746`、Visual `39544479872`、Secret scan `39544479839` 待核对最终结果。

### 文件改名与 metadata 删除

新增客户端 `RenameEntry` 和 `RemoveMetadata`，与 NAS 共享改名 payload、metadata tombstone 和 captured_at 重算逻辑；客户端 metadata 操作附带原内容身份前提。针对性 race 检查通过（`/tmp/kist-client-file-commands-targeted.log`），覆盖无原文件缓存时删除已知属性、无变化回执、NAS 独立改名合并、重新提取不能复活 tombstone，以及改名同步后的严格撤销。开始固定代码完整检查 `/tmp/kist-client-file-commands-check.log`；删除/恢复与客户端历史仍待实现。

新增客户端 `Undo` 业务入口，使用 `SyncReference` 明确区分 NAS op 与本地意图，不删除原 pending。命令入队和普通写入共用请求作用域，严格状态检查继续由既有重放/接纳路径执行。针对性 race 检查通过（`/tmp/kist-client-undo-final-targeted.log`），覆盖离线创建后撤销、同步后使用原本地引用重做、NAS 引用撤销、后续修改阻挡及请求作用域错误分类。当前完整检查启动早于撤销入口，结束后需对包含该入口的最终代码再次完整检查；历史列表、操作组 UI 和删除恢复仍待接入。

`/tmp/kist-client-file-commands-check.log` 已通过（退出码 0），包含撤销入口的最终检查运行于 `/tmp/kist-client-file-undo-final-check.log`。标签阶段 `df2e928` 的 CI `39544479746` 已通过，Visual `39544479872` 仍在运行。后续配置审计需检查首次连接的时区继承：当前 `CreateClient` 默认 UTC，而同步能力未携带 NAS 时区；不能将当前 tombstone 测试当作非 UTC 内容时间一致性的证明。

文件编辑与撤销最终完整检查 `/tmp/kist-client-file-undo-final-check.log` 已通过（退出码 0）。下一步补齐 NAS 时区能力与客户端继承，已有 pending payload 不随配置刷新重写；随后继续稳定删除 revision、恢复和历史/网页接入。

本阶段已提交并推送 `477690a`；CI `39545303457`、Secret scan `39545303406` 已通过。Visual `39545303426` 完整采集 134 张，仅 entry-menu-box 的“在新标签页打开”文字出现 326 个差异像素；已查看 expected/current/diff，产品前端源码相对基准无变更，未接纳基准，已重跑同一提交检查采集稳定性。

### NAS 时区继承

同步能力新增 timezone；初始化、重新连接与同步原子刷新客户端设置，缺失/Local/未知名称失败关闭。已记录 pending 不重写，后续 metadata 决策使用 NAS 时区。API 类型已再生成；针对性 race 检查通过（`/tmp/kist-client-timezone-targeted.log`），包含上海时区属性抑制后内容时间重算、设置刷新不改变 pending、东京时区真实 HTTP 初始化及能力拒绝。该正则未执行 syncclient 测试，后续完整检查需覆盖。固定代码完整检查 `/tmp/kist-client-timezone-check.log` 已通过（退出码 0），覆盖完整 race 测试与构建。

- `internal/api/server.go` 当前持有具体 `*library.Library`，初始化还直接根据 NAS 布局创建上传暂存与媒体服务。不能只替换数据库连接就声称支持客户端。
- `library.ExecuteRequest` 的幂等结果是 NAS 数字 op ID，且无业务修改也记录 request receipt；客户端需要独立持久化请求回执并保留无操作重试语义，不能遗漏或暴露客户端视图序号。
- 现有领域命令负责生成 before/after payload，`write`/`writeLocked` 承担正式日志与投影。后续抽取共享命令生成及读视图边界，分别接入 NAS 写入口和客户端 pending 入口；上传、历史、撤销与错误结果中的操作引用须一并审计，不以只读浏览替代离线编辑要求。

1. 缓存阶段完整检查、提交及 CI；扩展真实网络故障与长文件传输的最终验收。
2. 客户端命令和网页接入现有读写能力，完整逐 action patch、冲突处理、历史与撤销引用审计。
3. 按需内容缓存、配额/年龄、租用保护、离线保留、fileset 依赖及未上传内容独立统计。
4. 网页离线提示、准备离线、同步进度、冲突解决与历史。
5. NAS/客户端双实例完整验收、跨平台 CI、视觉审阅、操作文档与逐项完成审计。


### 离线删除与恢复（实施中，尚未提交）

时区阶段 `ff029d5` 已提交并推送；CI `39546249948`、Visual `39546249957` 仍在运行，Secret scan 已通过。旧 Visual 重跑 `39545303426` 被新提交取消，没有通过证据，继续以新提交的完整采集结果为准。

新增客户端条目/整盒删除预览与命令，revision 绑定库、对象、语义快照、fileset 变更及最近 box 操作的稳定身份；复用 NAS 成员删除规则及现有严格撤销。同步接纳重算的创建/更新时间不参与 revision，但仍保留在真实删除 payload。`/tmp/kist-client-deletion-targeted.log` 的三项测试通过：确认跨接纳稳定、同 box 后续修改使确认失效、整盒删除与本地恢复后同步。

新增更严格的连续离线操作回归 `TestClientDeletionAfterPendingRenameSynchronizes` 后发现尚未解决的问题：rename 接纳时间改变文件 UpdatedAt，后续 delete 的完整快照仍是原始本地时间；NAS 拒绝自身连续意图。复现日志 `/tmp/kist-client-deletion-pending-time.log`，当前失败，不提交、不宣称删除功能完成。下一步需在合并与本地 rebase 层依据稳定前提生成合法快照，保留原提案及摘要，不放宽旧 Apply 的严格状态比较，也不能简单忽略所有时间而掩盖实际并发修改。需追加 NAS 并发修改、nested fileset、reopen 与恢复检查，再运行完整 make check。


删除时间回归已修复：新增仅用于删除快照的提案 v2 与 deletion_snapshots 能力，NAS/客户端共用稳定 box 前提与完整语义比较后生成当前快照；不修改旧 Apply。`/tmp/kist-client-deletion-rebase.log` 与 `/tmp/kist-client-deletion-race.log` 均通过，后者覆盖 NAS 改后又改回仍冲突、嵌套 Live Photo 页删除/撤销恢复及 NAS rebuild。API 类型重新生成成功。

时区提交的 CI `39546249948` 在 Windows 失败，原因是测试默认库依赖系统探测，而 Windows fixture 未设置可移植时区；明确 Local 拒绝语义没有改变。已将 library、CLI、真实 HTTP 传输测试库配置为显式 UTC，保留专门的非 UTC 与非法时区覆盖。Visual `39546249957` 仍待结果。开始包含上述修复的完整检查 `/tmp/kist-client-deletion-full-check.log`，本阶段尚未提交。


Visual `39546249957` 已通过完整采集比较，旧菜单单行像素差异未复现，未修改基准。删除阶段首次完整检查 `/tmp/kist-client-deletion-full-check.log` 退出 2，前端 248 测试中 16 项超时，未报告断言失败；当时多个 Vitest worker 同时高 CPU。已确认该进程结束后，以工具支持的 `VITEST_MAX_WORKERS=2 make check` 重跑完整流程，日志 `/tmp/kist-client-deletion-limited-check.log`。未增加超时、修改测试断言或跳过测试；待结果验证资源争用假设。


降低并发的完整检查已退出 2：37/38 文件、246/248 测试通过，仅 theme.test.tsx 前两项超时。单独 `--maxWorkers=1` 运行仍为相同两项超时（`/tmp/kist-theme-isolated.log`）；因此不能仅归因并发争用。相关 Settings/theme 源码及依赖无本阶段差异。已采集独立 worker CPU profile：`/tmp/kist-theme-profile/CPU.20261021.130704.67187.0.001.cpuprofile`，主要自耗时在 React 开发 JSX 创建、Mantine Box 和 DOM selector，尚未作根因结论；未扩大超时、弱化断言或提交当前阶段。所有检查进程已结束，下一步继续定位两项测试耗时并恢复完整验收。


设置页超时根因进一步定位到共享可搜索 Select：Mantine Combobox 默认 keepMounted=true，关闭时仍渲染数百个时区选项。复用组件自带 keepMounted=false，保留原搜索、焦点与关闭过渡实现，不新增样式或提高超时。单独原 theme 测试 `/tmp/kist-theme-unmount.log` 全部 4 项通过（此前相同单 worker 为 2 项超时）；完整检查继续使用两个 worker，并需由视觉 CI 验证共享选择器各场景。开始 `/tmp/kist-client-deletion-final-check.log`。


`/tmp/kist-client-deletion-final-check.log` 的完整前端测试已通过：38 个文件、248 项测试全部通过，保留默认超时与原断言。整个 make check 进程仍在执行后续构建/Go 检查，尚不能记为完整通过；继续等待同一进程。


最终完整检查在 Go library 包累计 10 分钟总时限处被中止（退出 2），其余包通过；当前 TestRebuildReloadsTheTagTree 仅运行 6 秒，堆栈在 SQLite 解析/重放，文件访问记录一直推进，无已报告断言/race 失败。该包现有 277 项测试。对该包单独执行 `go test -race -timeout=20m ./internal/library`，日志 `/tmp/kist-client-deletion-library-race.log`，保留全套测试与断言，仅增加整包累计时限。之后再执行 make check，Go 文档确认成功缓存可跨 timeout 值复用，不跳过任何检查。


为避免等待期间改动受检代码，下一阶段收藏搜索在独立 worktree `/tmp/kist-offline-saved-search`、分支 `codex/offline-saved-search`（基于 ff029d5）推进。已抽取共用更新 payload 校验并补客户端 SavedSearches/Create/Update/Delete，添加请求重试、noop、删除撤销、同步重建及 NAS 并发冲突测试；针对性测试日志 `/tmp/kist-client-saved-search-targeted.log` 正在执行。该工作目录尚未提交，也未合入 main；主工作树仍在运行库层完整 race 验证。合入时须保留 main 的时区测试库修复及删除阶段变化，不用旧分支覆盖它们。


独立收藏搜索工作目录的针对性测试已通过：`/tmp/kist-client-saved-search-targeted.log`。随后补上共用客户端 readView：普通读取持客户端锁刷新叠加视图后读取 SQLite 快照；请求内部共享锁并允许读取刚完成的写入，过期 scope 拒绝。Client.Box 不再构造只有 db 的临时 Library。跨句柄读取、请求内写后读、过期 scope 和已有 Box 命令测试 `/tmp/kist-client-saved-search-reads.log` 已通过。上述代码仍仅在独立 worktree，待主阶段完成后合入与完整验证。


独立 worktree 继续补齐基础目录读取：File/Folders/Boxes、文件和盒标签、metadata、位置、Files/Browse/Filesets、TagTreeCounts，全部经客户端 readView，查询复用现有 store 及 NAS 时区。`/tmp/kist-client-catalog-reads.log` 通过，验证未缓存原文件仍参与标签检索并可读取元数据，原文件接口明确 ErrNotCached。该分支尚未接入 HTTP/UI，也未做完整阶段验收；主工作树删除阶段的完整 race 仍独立运行。


独立 worktree 的收藏搜索、目录读取及 Box 命令针对性 race 检查 `/tmp/kist-client-catalog-race.log` 已通过（24.824 秒）。尚未合入，完整验证与提交需在主阶段收口后进行；未把针对性通过作为整个客户端完成的证据。


主工作树 library 完整 race 检查已通过：`/tmp/kist-client-deletion-library-race.log`，退出码 0，735.236 秒。证实此前是整包累计 10 分钟上限，并非单个测试挂起。保持同一代码，执行最后 `VITEST_MAX_WORKERS=2 make check`，日志 `/tmp/kist-client-deletion-complete-check.log`，让 Go 复用已通过的有效测试缓存并完成全部门禁与二进制构建。


最后检查未命中 library 测试缓存，已观测到实际新进程仍使用 10m 总上限，因此停止该重复执行，不把缓存复用当作验证前提。Makefile 显式增加可覆盖的 GO_TEST_TIMEOUT（默认 20m），用于完整 Go race 检查；依据是同一代码的全套库层测试已通过但实际需要 735 秒。未改变测试集合、断言或单项等待条件，避免开发机正常执行的整包被默认 10m 截断。将按该命令重新完成完整检查，日志 `/tmp/kist-client-deletion-timeout-check.log`。


独立 worktree 已提取 readFilesetDetail/readMediaInfo/readCapturedInfo 供 NAS 与客户端共用，NAS 路径也改为同一读事务快照。客户端补 FilesetDetail、MediaInfo、CapturedInfo；未缓存原文件的测试对比 NAS 与客户端媒体事实/时间来源完全一致。`/tmp/kist-client-detail-reads.log` 针对性检查通过（第一次编译作用域错误已修正），完整验收仍待合入后执行。


独立 worktree 的整个 library 普通测试已通过：`/tmp/kist-client-catalog-library.log`，退出码 0，143.004 秒，覆盖共用详情读取重构后的现有 NAS 库层功能。该分支仍待最终完整 make check 与合入；主工作树 `/tmp/kist-client-deletion-timeout-check.log` 已进入显式 20m 整包时限的完整 race 检查。


删除阶段最终完整 `VITEST_MAX_WORKERS=2 make check` 已通过（进程退出 0，日志 `/tmp/kist-client-deletion-timeout-check.log`），包含全部前端测试、契约、格式、构建、Go vet、全包 race 及单二进制构建。现在提交该阶段并检查远端 CI；独立 worktree 的收藏搜索与目录读取仍待下一阶段合入。
