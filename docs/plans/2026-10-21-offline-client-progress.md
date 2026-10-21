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


删除阶段已提交并推送 `f4e1175`；CI `39596164301`、Visual `39596164486` 已启动，Secret scan `39596164485` 已通过。收藏搜索及目录读取工作目录的 tracked diff 经 `git apply --check` 后合入 main，并复制 6 个新增文件，保留删除阶段全部变化。正在执行合入后的 `VITEST_MAX_WORKERS=2 make check`，日志 `/tmp/kist-client-catalog-complete-check.log`（会话 14593）。独立工作目录仍保留，不能重复应用其补丁。

下一步 HTTP 接入的源码核查：`api.Server.library` 仍是具体 `*Library`，构造函数创建 NAS media、uploads、sync-uploads 并注册同步服务端；不可仅传入一个拥有 client.db 的空 Library。`fileReader` 当前要求无错误返回的 `Zone()`，而客户端时区属于带锁读取的持久设置；应在客户端请求快照中固定时区并复用同一读取视图，避免每个 DTO 字段都重建叠加层。另需接齐 FilesWithTags、SearchBoxes、标签条件、封面、请求去重状态、历史/撤销映射、导入/提取和带缓存租约的内容句柄。现有 `OpenContent` 的 `*os.File` 契约也不能丢弃缓存读租约。上述是后续实现工作，不表示网页已经可用。


为保持主工作树完整检查输入不变，后续检索补齐仍在 `/tmp/kist-offline-saved-search`：新增 `client_search.go` 与 `client_search_test.go`，实现 FilesWithTags、SearchBoxes、BoxCover，每次在同一 SQLite 读取快照中获取对象与标签/封面引用。测试验证 pending 标签分配及重命名反映到结果、封面未缓存仍可解析、不存在盒明确报错；`/tmp/kist-client-search-reads.log` 已通过（0.764 秒）。这两个文件尚未复制到 main，后续仅复制这两个新文件，不能重复应用此前已合入的 worktree diff。主检查会话 14593 仍在运行；f4e1175 的容器 amd64/arm64、平台 Linux ARM/macOS 已通过，Windows/check/Visual 仍在运行。


目录读取和收藏搜索阶段完整 `VITEST_MAX_WORKERS=2 make check` 已通过（会话 14593 退出 0，日志 `/tmp/kist-client-catalog-complete-check.log`）。按阶段提交当前 main 内容。独立 worktree 另新增 client_tag_query.go/test.go，并从 commands.go 抽取共用 resolveTag 查询函数；验证 pending 子标签移动、改名后的包含/排除条件与名称/ID 解析，`/tmp/kist-client-tag-query.log` 通过（1.480 秒）。这批标签查询和此前 client_search 两个文件尚未合入；后续合入仅取 commands.go 增量及这 4 个新文件。


目录读取/收藏搜索阶段已提交推送 `42d5f86`，CI `39596936256` 运行中、Visual `39596936245` 排队、Secret scan `39596936185` 通过。随后已合入独立 worktree 的 client_search、client_tag_query 四个新文件及 commands.go 的 resolveTag 增量，勿重复应用。main 另外补 Client.SetBoxCover，使用既有 update_box payload 与共享校验；覆盖跨盒拒绝、未缓存封面修改、同步及同步后通过 localID 严格撤销。针对性测试 `/tmp/kist-client-search-cover.log` 通过（2.415 秒）。下一步执行该阶段完整检查，并继续请求快照/HTTP 接入。


搜索/封面阶段完整检查已启动：`/tmp/kist-client-search-cover-check.log`，会话 42837。上一提交 f4e1175 的 CI `39596164301` 与 Visual `39596164486` 已因后续推送取消，不记为通过；需要在继承该修改的新提交 42d5f86 对应运行收齐 Windows/check/Visual 结果。


在独立 worktree 补客户端请求级只读快照：新增 client_view.go/test.go，client_read.go 增加受作用域保护的同事务读取分支。Client.View 在一次 overlay 刷新后固定 SQLite 事务与时区，ClientView 仅暴露目录读取方法，回调结束后读取返回 ErrRequestScope，并发读取由作用域锁串行，回调错误仍释放锁。避免详情 DTO 每个字段重新获取客户端锁/重建 overlay。针对性 race `/tmp/kist-client-view.log` 已通过（5.175 秒）；首次测试错误地固定 UTC，已改为验证继承 NAS 时区。此增量尚未合入 main，仅在 `/tmp/kist-offline-saved-search`；后续只复制 client_view.go/test.go 与 client_read.go 增量，不覆盖 worktree 上旧版其他文件。主完整检查会话 42837 仍在运行，42d5f86 的 CI/Visual 尚待收齐。


搜索/标签条件/封面编辑阶段完整 `VITEST_MAX_WORKERS=2 make check` 已通过（会话 42837 退出 0，日志 `/tmp/kist-client-search-cover-check.log`）。提交该阶段；等待前一提交 CI/Visual 完成后再推送，避免连续取消尚未收齐的跨平台结果。


搜索/封面阶段已提交 `dbc1abb`（尚未推送，等待前序 Visual 完成避免取消）。42d5f86 的 CI `39596936256` 已全部通过，包含 Windows、macOS、Linux ARM、双架构容器及主检查；Visual `39596936245` 仍运行。已将 ClientView 两个新文件和 client_read.go 增量合入 main，新增 API 层测试确认 ClientView 实现既有 fileReader，分页接收目录后可复用 renderFileDetail 生成与 NAS 相同结果且原文件仍为 ErrNotCached。`/tmp/kist-client-view-integration.log` 针对性 race 已通过（API 2.208 秒、library 2.278 秒）。将执行该阶段完整检查，日志 `/tmp/kist-client-view-complete-check.log`。


只读快照阶段完整检查会话 98693 仍运行。为不改动其输入，独立 worktree 开始 HTTP 接入：server.go 提取共用构造并新增 NewClient，复用 Host/Origin/CSP 和 DTO 渲染；client_reads.go 注册 info、catalog 状态、boxes、file detail、tags、saved-searches 的读取路由，不构造 NAS Library、不创建 NAS uploads/media/sync 服务端。client_server_test.go 验证离线新盒可读取、未接收目录不冒充完整、非法 Host/Origin 拒绝、NAS 同步服务路由不暴露、NAS 存储目录未创建。`/tmp/kist-client-server.log` 针对性测试通过（0.642 秒）。该 HTTP 增量仅在独立 worktree，尚未合入；仍缺其余路由、写接口、本地 serve 命令及网页体验，不可视为完整客户端。

Visual 39596936245 完整采集 134 张，比较出现 1 处差异，退出失败。失败日志 `/tmp/kist-client-visual-failure.log`；正在下载 visual-report-1 至 `/tmp/kist-visual-39596936245`（会话 85446），需查看 expected/current/diff 后判断修复或接受，未更新基准。dbc1abb 仍未推送，避免未审阅就覆盖该证据。


ClientView 阶段完整 make check 已通过：会话 98693 退出 0，日志 `/tmp/kist-client-view-complete-check.log`。提交该阶段，仍等待视觉报告下载会话 85446 完成以审阅前序差异。


ClientView 阶段已提交 2621e99，尚未推送；dbc1abb 也仍待推送。已从独立 worktree 合入 API server.go 增量、client_reads.go 与 client_server_test.go，之后在 main 接入客户端 /files 搜索，抽取 buildFileQuery/buildSearchConditions 供 NAS 与 ClientView 共用，查询解析与结果渲染位于同一客户端快照。现有 ClientView API 测试新增真实 HTTP 文件搜索/结构化排除条件/详情断言，验证 pending 标签与原文件未缓存并存；`/tmp/kist-client-http-search.log` 通过，`/tmp/kist-client-http-search-race.log` 通过。该 HTTP 阶段仍未提交，且未接齐全部客户端功能。视觉报告下载会话 85446 仍 live，未重启或更新基准。


独立 worktree 新增 client_box_reads.go/test.go，并在 clientReadRoutes 调用 clientBoxReadRoutes，补 boxes/{id}/folders、tags、cover，均使用 ClientView；验证 pending 新盒与标签可见、不存在盒返回 404，`/tmp/kist-client-box-http.log` 通过（0.881 秒）。尚未合入 main；后续只复制这两个新文件并添加注册调用，不能覆盖 main 已新增 /files 搜索的 client_reads.go。


HTTP 基础读取与共用搜索解析阶段完整检查已通过：会话 60611 退出 0，日志 `/tmp/kist-client-http-complete-check.log`。阶段提交，前序视觉报告下载会话 85446 仍运行，暂未推送覆盖远端待审阅 SHA。


已合入 client_box_reads 两个新文件及注册调用。main 新增 `client serve DIRECTORY`，默认 127.0.0.1:8474，复用 serveOptions/serveHTTP、嵌入网页、信号关闭，启动不读取连接凭据/联系 NAS；真实 HTTP 测试在未配置连接的客户端读取 pending 新盒并验证关闭。新增 client_filesets.go，接入 filesets 列表/详情与 entries，抽取 buildEntryQuery/renderEntries 给 NAS 和客户端共用，保持成员过滤与游标模式校验；现有 NAS fileset HTTP 测试增加客户端对照。`/tmp/kist-client-browse-serve.log` race 通过（API 4.227 秒、cmd 2.163 秒）。尚未实现完整页面编辑/上传/预览/历史/同步 UI，不能宣称客户端网页完整。正在跑完整检查 `/tmp/kist-client-browse-serve-check.log`。视觉报告下载会话 85446 仍 live。


本地启动/组合读取完整检查失败于 TestEveryPrintingCommandIsCovered：新增 client serve 未列入长运行 transport 例外，其他包通过。按已有 serve 的 stderr-only 规则补一行 covered["client serve"]，不改变 JSON 输出要求；针对性 `/tmp/kist-client-serve-output.log` 已通过，重新完整检查日志 `/tmp/kist-client-browse-serve-final-check.log`。

视觉报告下载 85446 已完成。已查看 expected/current/diff，134 张仅 entry-menu-box 的“在新标签页打开”文字 326 像素差异，布局其余部分一致，与此前出现过的同尺寸差异一致；没有已确认的预期 UI 改动可据此接受。已请求重跑 Visual 39596936245 的失败 job，等待 attempt 2，未更新基准。报告目录 `/tmp/kist-visual-39596936245`。

独立 worktree 新增 client_request_status.go/test.go，提供 RequestStatus/CheckRequest，从持久 pending 回执恢复 lost-response 结果，保持 NAS 接纳前后 localID 不变，校验 digest、noop 与不存在请求。`/tmp/kist-client-request-lookup.log` race 通过（2.987 秒）；尚未合入 main，仅这两个新文件待复制。


本地 serve/组合目录阶段最终完整 make check 已通过：会话 67465 退出 0，日志 `/tmp/kist-client-browse-serve-final-check.log`。阶段提交；Visual 39596936245 attempt 2 仍运行，等待结论后推送后续已验证提交。


本地服务阶段已提交 3b27401。独立 worktree 新增 client_writes.go、client_tag_writes.go、client_writes_test.go，并在 server.go 的客户端分支注册 clientWriteRoutes；接入 /requests/{key}、盒创建/更新、标签创建/改名/移动/删除/关联、fileset 配置写入，全部用 Client.ExecuteRequest，响应为稳定 local_id/noop。`/tmp/kist-client-http-writes.log` race 已通过（API 2.203 秒、library 2.113 秒）；首次测试错误使用 POST 关联标签，已改用既有 PUT 协议。尚未合入 main；待复制 client_request_status 两个 library 文件和以上三个 API 文件，仅向 main server.go 添加注册调用，不能覆盖主线其他新增路由。网页 mutation 结果契约仍需接入 local_id，不能将其当 NAS op_id。


已合入 client_request_status 两个库文件、client_writes/client_tag_writes/client_writes_test 三个 API 文件，并在 main server.go 客户端分支注册写接口。apischema 增加 --client 模式，web/scripts/api-schema.mjs 同时从 NAS 和客户端真实契约生成/检查 api-schema.d.ts 与 client-api-schema.d.ts；网页 Receipt 改为两种生成类型的联合，明确不把 local_id 转成 op_id。新增 lost-response/remount 重试与 noop 回执测试。`/tmp/kist-client-receipt-web.log` 7 项通过；`/tmp/kist-client-writes-merged.log` 合入后 API/library race 通过（2.241/2.314 秒）。Visual attempt 2 尚在运行；本阶段开始完整检查 `/tmp/kist-client-http-writes-check.log`。


客户端写接口完整检查首次在生成文件格式检查失败：新 client-api-schema.d.ts 未按既有 api-schema.d.ts 规则加入 .prettierignore。已加入同样例外，类型内容仍由 generate:api/check:api 约束。随后补 client_entries.go、client_saved_searches.go、client_box_writes.go，注册目录创建、条目重命名/删除预览/删除、盒删除/封面、收藏搜索写入。客户端删除使用字符串确认 revision；网页预览类型改为 NAS/Client 生成类型联合，BoxTrashDialog 测试同时验证数值和不透明字符串原样提交。HTTP 测试验证确认后改名使删除过期、重新确认后成功及同键重试幂等。`/tmp/kist-client-write-delete.log` race 通过（API 2.503 秒、library 2.177 秒），`/tmp/kist-client-write-web.log` 10 项通过。重跑完整检查日志 `/tmp/kist-client-http-writes-final-check.log`。Visual 39596936245 attempt 2 仍在运行。


Visual 39596936245 attempt 2 已通过，基准未更改；attempt 1 的单行文字差异未在同 SHA 重跑复现。现推送已完成完整检查的 dbc1abb/2621e99/f38bb45/3b27401；当前写接口阶段仍在工作树，完整检查会话 34026 未完成。

独立 worktree 新增 client_content.go/test.go、client_content_test.go（API），修改 API content.go 使用 io.ReadSeekCloser 以保留缓存租约，客户端分支调用 Client.OpenContent 并注册共用 contentRoutes。原文件仅本地读取，无网络；未缓存明确 503/not_cached，Range/HEAD 复用 NAS 已有 sniff/disposition/CSP/ServeContent。`/tmp/kist-client-content.log` race 通过（API 2.497 秒、library 2.674 秒），覆盖分段内容、HEAD、未缓存与读期间容量逐出保护。该增量尚未合入；后续只复制两个库文件及 API 测试、应用 content.go 增量并在 server.go 添加 contentRoutes 注册，不能用 worktree 旧版覆盖 main。客户端历史内容 at 参数暂明确拒绝，仍须后续实现历史内容读取，不能作为最终限制来缩小任务。


客户端网页写接口完整检查已通过：会话 34026 退出 0，日志 `/tmp/kist-client-http-writes-final-check.log`。提交本阶段；3b27401 的 CI/Visual 仍运行。独立 worktree 另外补 client_box_search.go，并从 box_search.go 抽取 buildBoxQuery/renderBoxSearch 供两端共用；client_box_reads_test 增加 structured tag_all/tag_not 与非法游标验证，`/tmp/kist-client-box-search.log` race 通过（2.964 秒）。待合入 box_search.go 增量、client_box_search.go、新增测试段及注册调用；worktree 的 files.go/search_conditions.go 已复制自主线供编译，不应重复合入。


网页编辑阶段已提交 326f474，待前序 CI/Visual 收齐后推送。已合入客户端 content/box search 增量：新增库 client_content 两文件、API client_content_test/client_box_search，应用 content.go/box_search.go diff，更新 client_box_reads_test 与两处路由注册，重新生成 API 类型。`/tmp/kist-client-content-search-merged.log` 合入后 race 通过（API 3.477 秒、library 2.428 秒）。开始完整检查 `/tmp/kist-client-content-search-check.log`。上述 worktree 增量均已合入，不要重复应用。


离线导入准备继续在独立 worktree：upload_target.go/upload_conflicts.go 提取共用 uploadTarget/previewUpload，注入单次读取快照；新增 client_upload_target.go/test.go，覆盖 NAS/客户端相同目标指纹与冲突预览、未缓存已有文件冲突、本地文件改名改变冲突但不改变目录目标、盒改名使目标失效。`/tmp/kist-client-upload-target.log` race 通过（5.717 秒），含现有 NAS 上传目标/覆盖/目录冲突回归。该增量尚未合入 main。

源码核查：离线导入不能直接构造临时 Library 调用 PrepareFiles，因为它绑定 cleanTmp、NAS hooks、正式 opID 与 blob 完整性语义。下一步须抽取候选校验/单文件 blob 导入/命名与 captured 时间决策共用部分，客户端独立持久 local-blobs 后生成 pending import_files/upload_files；PreparedClientImport 返回本地引用，不能把临时 view ordinal 暴露为 OpID。覆盖原有替换、strict expected hash、取消/部分结果及提交前目标重校验；覆盖 payload 中时间字段与 NAS 接纳时间差异也需要与删除阶段同样审视。主线内容/搜索完整检查会话 32111 仍 live，前序远端 CI/Visual 尚未收齐。


内容读取/归档盒搜索阶段完整 make check 已通过：会话 32111 退出 0，日志 `/tmp/kist-client-content-search-check.log`。提交本阶段，前序 CI/Visual 仍运行。

独立 worktree 继续抽取导入共用逻辑：import.go 新增 importCandidates（路径/重复/常规文件校验）、prepareImportFile（指定 blob.Dir 与时区，保留 expected hash/size 和命名规则）、captureImportTime；NAS 原方法委托这些函数。upload_conflicts.go 提取 replacementAfter，捕获时间解析由传入函数决定。`/tmp/kist-import-shared-preparation.log` 针对 NAS Import/VerifiedImport/Prepared/Upload 的 race 回归通过（12.690 秒）。这些增量与 client_upload_target 仍未合入 main；客户端 PreparedImport 与上传接入尚未实现，不能把共用逻辑抽取视为离线导入完成。


3b27401 的 CI 39601290465 与 Visual 39601290493 均通过，已推送至 c475528；新 CI 39602617485、Visual 39602617462、Secret scan 39602617421 已运行。

独立 worktree 完成首版 Client.PrepareFiles/PrepareImport/Import/PreparedClientImport：使用共用候选/单文件准备，原文件落 protected local-blobs，独立 ClientImportOptions.Group/ClientImportResult.Reference，不暴露 view ordinal；读快照收集目标/名称/时区，提交锁内复核目标及显式冲突 token，再生成 pending import_files/upload_files。支持 skip/编号、keep_both、overwrite、expected hash/size 回读、取消的准备结果、替换保留用户事实。`/tmp/kist-client-import.log` race 通过（5.698 秒），验证 pending 新盒导入、重复请求、缓存上限不逐出新原文件、同步及撤销、目标变更/坏 hash 拒绝、覆盖保留 ID。`/tmp/kist-client-import-pending-replacement.log` 验证先 pending rename 后覆盖也顺序同步通过（1.182 秒）。

现已合入 main：import.go/upload_target.go/upload_conflicts.go 增量与 client_upload_target/client_import 共四个新文件。后续需扩大覆盖 group/取消/冲突/strict 批次，并接 HTTP 上传流程与 protected blobs 生命周期统计；尚不能宣称用户已能在网页离线导入。


客户端导入合入后的针对性 race `/tmp/kist-client-import-merged.log` 已通过（18.323 秒），覆盖新客户端导入及现有 NAS 导入/上传准备回归；本阶段仍待扩大关键边界测试与完整 make check，未提交。


导入边界测试新增：Group 引用在准备时复制，两个离线导入同步后 NAS RootOpID 映射正确；严格批次第二文件坏 hash 时第一文件也不进入可见目录；已取消准备返回 context.Canceled；NAS 并发新增文件标签使离线覆盖显式冲突，客户端 Status.BlockedID 指向原导入且 NAS 原文件/标签保留。`/tmp/kist-client-import-boundaries.log` race 已通过（9.376 秒）。首次测试误假设冲突仅通过 result 返回，已按既有 ErrSyncConflict 加 Status 的契约断言，未放宽数据保留检查。开始导入阶段完整 make check，日志 `/tmp/kist-client-import-complete-check.log`。


离线导入阶段完整 `make check` 已通过：会话 66217 退出 0，日志 `/tmp/kist-client-import-complete-check.log`。前序 c475528 的 CI 39602617485 已通过，Visual 39602617462 尚在运行。本阶段提交导入内核；网页续传接入、提取与受保护空间统计仍待完成。


离线导入阶段已提交并推送 870ff31；前序 c475528 的 CI/Visual 均通过。新 CI 39603894207、Visual 39603894360 运行中，Secret scan 39603894278 已通过。

网页续传接入：upload_backend.go 统一两端目标/冲突检查、准备提交及持久回执查询，复用原有分块/Seal/最终校验协议；客户端启用 uploads 暂存与定期清理，info 声明协议 v2。暂存状态新增互斥 local_id/noop，完成后的重试、取消保护与清理保持原语义；正式 op_id 不伪造。目标检查显式使用 ExecuteRequest 回调 context，避免客户端锁内重新获取锁。网页共用续传恢复 local_id/noop，生成两端契约。

`/tmp/kist-upload-local-receipt.log` upload race 通过（6.322 秒）；`/tmp/kist-client-upload-http-final.log` API race 通过（5.904 秒）、cmd 通过；`/tmp/kist-client-upload-web.log` 18 项通过。HTTP 新测试覆盖未连接 NAS 的客户端上传、中途换句柄恢复偏移、提交回执保存前故障、重复请求同一 local_id、内容读取与 NAS 零写入。首次测试遗漏 v2 最终 digests 并误认为服务端不会主动恢复回执，已按既有协议修正；旧客户端服务器测试改为允许自身 uploads 目录，仍禁止 ops/blobs/sync-uploads。当前开始完整检查，尚未提交该阶段；客户端提取/预览等后续范围仍未完成。


网页续传阶段完整 make check 已通过：会话 2160 退出 0，日志 `/tmp/kist-client-upload-complete-check.log`。本阶段提交；870ff31 的 CI/Visual 尚在运行。独立 worktree 新增 client_extraction.go/test.go 和 API client_extraction.go/test.go，共用 extractionChange 事实差量逻辑；按单次快照捕获原文件/metadata/版本/时区，读取本地原文件并持缓存租约，提交 extractor origin 与内容前提。`/tmp/kist-client-extraction-http.log` API/library race 通过（3.113/8.303 秒），含本地回执、重复请求、未缓存返回、内容替换拒绝、同步/rebuild/Undo。尚未合入，仅这四个新文件、extract.go 小增量及 server 路由注册待合入，不得复制旧 worktree server.go 覆盖主线续传。


续传阶段提交 4060034 已推送；870ff31 CI 39603894207 与 Visual 39603894360 均通过。4060034 的 CI 39605157381、Visual 39605157438 正在运行，Secret scan 39605157405 已通过。

离线提取四个新文件、extract.go 的 extractionChange 共用逻辑以及 clientExtractionRoutes 注册已合入 main，重新生成客户端 API 类型。`/tmp/kist-client-extraction-boundaries-final.log` race 9.825 秒通过，新增不匹配 EXIF 时保留捕获时间、提取后隐藏媒体事实与再次提取不恢复；首次测试在尚不存在的 metadata 上调用删除，按原有契约修正为先提取再隐藏。`/tmp/kist-client-extraction-merged.log` 主线合入后 API/library race 通过（3.132/10.080 秒）。开始完整检查 `/tmp/kist-client-extraction-complete-check.log`。上述 worktree 提取增量均已合入，不要重复复制。提取目前使用既有单文件 API 与当前版本幂等语义；客户端媒体派生、历史读取、同步管理 UI 与受保护空间统计仍待完成。


离线提取阶段完整 make check 已通过：会话 63805 退出 0，日志 `/tmp/kist-client-extraction-complete-check.log`。本阶段提交；4060034 的远端 CI/Visual 尚在运行。

独立 worktree 完成文本/表格预览增量：新增 openCurrentContent 共用读取，sheetpreview.Source 抽象只保留 Read/Seek/ReadAt/Stat，接受缓存租约句柄；客户端注册既有 text/sheet 路由，未缓存返回 not_cached。新增 client_preview_test.go 使用真实可逐出缓存验证文本与 CSV，`/tmp/kist-client-text-sheet.log` API race 2.959 秒通过、sheetpreview 全格式回归通过。另新增 client_storage.go/test.go，提供 local-blobs 与 pending 日志独立实际字节统计，包括尚未清理的已确认原文件，不冒充待同步字节；`/tmp/kist-client-storage.log` race 2.485 秒通过，验证去重及清理隔离。上述预览与统计增量尚未合入。


提取阶段已提交 5a2e898，待前序 CI/Visual 通过后推送。文本/表格预览与 ProtectedStorage 现已合入 main：新增三个文件（client_preview_test 与 client_storage 两文件），增量修改 sheetpreview 两文件、API text/sheets/content 和客户端路由；增加 /client/protected-storage 只读接口并重新生成契约。`/tmp/kist-client-preview-storage-merged.log` 主线 API/library/sheetpreview 针对性 race 全通过。开始完整检查 `/tmp/kist-client-preview-storage-complete-check.log`。这些 worktree 增量已合入，不要重复覆盖。派生媒体缓存、已确认 local-blobs 的安全转移/清理、同步管理 UI 和完整历史仍待完成。


预览与受保护空间统计阶段完整 make check 已通过：会话 33782 退出 0，日志 `/tmp/kist-client-preview-storage-complete-check.log`。本阶段提交。4060034 的 CI 39605157381 与 Visual 39605157438 均通过，已推送提取提交 5a2e898；其 CI 39606452752、Visual 39606453187 运行中，Secret scan 39606452884 通过。

独立 worktree 新增 API client_management.go/test.go 与 client_sync.go/test.go：本地状态、缓存配置/清理、逻辑离线保留 CRUD/状态、认证连接、手动同步、原文件准备和持久冲突决定；均通过现有 library 边界。同步已知冲突返回携带 blocked/pending 的结果，传输故障仍返回失败，不把不确定提交当作拒绝。`/tmp/kist-client-management-sync-resolve.log` race 3.241 秒通过，使用两实例真实 HTTP 验证连接不泄漏 token、离线创建同步、NAS 并发同字段修改冲突、重复同步不重复操作、明确保留 NAS 修改并持久记录决定后收敛。管理测试覆盖缓存配置跨进程与无 NAS/pending 写入。这四个新文件及 server 的两处注册待合入；不能复制 worktree 旧 server。


预览/统计已提交 1827dbf，待前序远端检查通过后推送。已合入 client_management/client_sync 四个新文件及两处注册调用；info 新增可选 client=true 声明，NAS 不返回该字段，供网页准确识别本地客户端。`/tmp/kist-client-management-merged.log` 主线 race 3.404 秒通过，重新生成两端 API schema。开始完整检查 `/tmp/kist-client-management-complete-check.log`。管理/同步 worktree 增量已合入，不要重复复制。下一步应接实际网页面板（现有 App info 查询和 sidebar-footer / Settings 可复用），当前只有 API，不能称网页同步闭环完成；派生预览/历史等剩余范围继续保持。


管理接口完整检查首次失败于 syncclient 测试包循环导入：内部测试包引用 api，而新 api 客户端管理引用 syncclient。将仅使用公开 API 的 http_test.go 改为 syncclient_test 外部测试包，不改变生产依赖或测试断言。`/tmp/kist-client-management-cycle.log` syncclient/API race 通过（4.289/3.406 秒）。重新完整检查 `/tmp/kist-client-management-final-check.log`。5a2e898 的 CI 39606452752 与 Visual 39606453187 均通过，正在推送预览提交 1827dbf。


客户端管理接口最终完整 make check 已通过：会话 75711 退出 0，日志 `/tmp/kist-client-management-final-check.log`。本阶段提交；预览提交 1827dbf 已推送，CI 39608119224、Visual 39608119289 运行中，Secret scan 39608119052 通过。

独立 worktree 网页面板：新增 OfflineClient.tsx / offline-client.ts / OfflineClient.test.tsx，App 客户端侧栏入口与目录未完整同步的提示；复用 Dialog/Mantine 控件，分同步/缓存/连接，待同步与未确认回执分别展示，放弃冲突需明确选择，停止网络保留进度。新增中英文文案/参数类型、pending 行样式，以及三个确定性视觉场景。复制了主线 web 源文件到该旧 worktree 作为开发基底，node_modules 链接根依赖；不要将基底差异误当新改动。`/tmp/kist-offline-ui-types-final.log` 类型检查通过；`/tmp/kist-offline-ui-workflows.log` 23 项通过；`/tmp/kist-offline-ui-visual-contract.log` 视觉场景契约通过。初次错误断言使用完整错误文字的子串，改为匹配既有 errorMessage 格式。实际浏览器视觉审阅尚未完成，UI 尚未合入。


客户端管理已提交 72dc466，等待前序 CI/Visual 后推送。网页面板已合入主线，仅复制上述三个新文件及 App/i18n/style/视觉场景四个文件，未复制 worktree 的 API/契约/其他基底差异。合入后 `/tmp/kist-offline-ui-merged.log` 23 项通过。开始完整检查 `/tmp/kist-offline-ui-complete-check.log`；新增视觉场景仍需真实浏览器/CI 候选审阅，不能在尚未渲染时宣称 UI 验收完成。尚未提供文件/fileset 的保留离线选择入口、完整冲突替换编辑器、历史与媒体派生预览，目标未完成。


面板首次完整 make check 通过（71709 退出 0，`/tmp/kist-offline-ui-complete-check.log`）。实际浏览器使用隔离 `/tmp/kist-offline-ui-i_9gm0ti`：NAS 回环 19591（会话 10482）、客户端 19592（11252），凭据仅保存 fixture 内 0600 文件。内置 iab 不可用且 Playwright 对应浏览器未安装，因此用已安装 Chrome 作开发验收，不替代正式 CI 视觉环境。首次同步成功、pageerror 为空；截图 `/tmp/kist-offline-client-before-sync.png`、`...-synced.png`、`...-cache.png` 已查看。截图捕获同步完成通知早于按钮状态收敛，后续截图应等待关闭按钮重新启用及 tab 选中状态。发现三项侧栏入口挤压版本文字，已调整客户端 footer 换行，并按已有表单去除必填星号。`/tmp/kist-offline-ui-polish.log` 相关 23 项通过。

1827dbf 的 Visual 39608119289 通过，但 CI 39608119224 的 Windows TestClientCachedTextAndSheetPreviews 失败于 unsupported offline operation version；其合成 OriginalSource 使用 nas.Zone().String() 在 Windows 得到 Local，触发协议正确拒绝。测试源改为明确 UTC（不修改生产 ValidateSyncCapabilities），日志 `/tmp/kist-preview-portable-timezone.log` 针对性 race 通过。下一次完整检查覆盖该测试修正和 UI 小调整，日志 `/tmp/kist-offline-ui-final-check.log`。文件/fileset 保留离线入口尚未动代码，当前本轮主要完成实际浏览器验收和 CI 根因修正。


Windows 合成源时区修正及 UI 小调整完整 make check 已通过：会话 55635 退出 0，日志 `/tmp/kist-offline-ui-final-check.log`。先单独提交测试修正并推送，以恢复跨平台 CI；网页面板仍留待正式视觉候选审阅。

独立 worktree 新增 OfflineEntry.tsx/test.tsx：文件/组合稳定 owner 保留，先保存逻辑选择再下载，失败保留选择与部分进度，已有本地原文件直接就绪不联系 NAS，取消保留只改变保留配置。FileGrid 右键与 Gallery 详情共享对话框，仅 client 模式显示。`/tmp/kist-offline-entry-tests.log` 30 项通过；类型检查发现 Gallery 的 primary 可能为空，正在修正为用已加载文件身份。该增量尚未合入。


跨平台测试修正已提交并推送 ce1d92b（同时包含先前已验证的 72dc466）；CI 39609825023 queued，Visual 39609825069 运行中，Secret scan 39609825088 通过。

保留离线入口增量已合入主线：OfflineEntry 两新文件、FileGrid/Gallery 入口、中英文文案与 offline-retention-file 视觉场景。Gallery 使用已加载 data.file.id，避免可能为空的 primary 身份。保留 fileset 传逻辑 folderID，由 library 递归展开，不能冻结主文件或当次成员列表。GET 就绪判断之后仅在有缺失时准备下载，先持久 PUT 保留选择，重试不会丢失；取消保留通过 DELETE 配置，不删原文件。`/tmp/kist-offline-retention-merged.log` 33 项通过，`/tmp/kist-offline-entry-types-final.log` 类型通过，`/tmp/kist-offline-retention-visual-contract.log` 视觉契约通过；关闭按钮断言限定 .actions 避免与 Modal 的关闭图标重名。开始完整检查 `/tmp/kist-offline-retention-complete-check.log`。UI 整体尚未提交/正式视觉验收，真实文件保留链路待新版二进制重启后浏览器验证。

保留入口完整 make check 已通过（83787 退出 0，`/tmp/kist-offline-retention-complete-check.log`）。真实 Chrome 在合成客户端完成手动同步、文件右键保留、下载就绪，截图 `/tmp/kist-offline-retention-ready.png` 已审阅。确认测试 NAS PID 99570 后停止该进程，再打开客户端 travel.txt，正文仍可读、pageerror 为空（43052 退出 0），截图 `/tmp/kist-offline-text-without-nas.png`。未操作个人库。ce1d92b 的 CI 39609825023 与 Visual 39609825069 均已通过。本 UI 阶段提交后仍须审阅四个新增正式视觉场景，不能将本地 Chrome 作为基准验收。

独立 worktree 的 client_history_view.go/test.go 提供仅确认 NAS 前缀的历史只读视图，临时数据库重放不包含 pending，打开历史内容保留缓存读租约。`/tmp/kist-client-history-view.log` 针对性 race 已通过，尚未合入、尚未接 API；完整历史列表、摘要、撤销映射等仍未完成。

网页管理与保留阶段提交 91abf03 已推送；CI 39612150951、Visual 39612151105 运行中，Secret scan 39612150968 已通过。

历史视图已合入 main，并通过共享只读 historyReader 接入原有历史文件/盒详情与 content?at= API；不包含 pending，未缓存内容返回 not_cached，Range 与历史文件名保持正确。`/tmp/kist-client-history-api.log` API/library race 通过（4.626/3.085 秒）。完整检查 `/tmp/kist-client-history-complete-check.log` 发现取消请求在锁空闲时可能竞争成功而返回 ErrNoSuchOp；已补入口 ctx.Err 检查，与 NAS ViewAt 保持一致。重复 20 次验证取消场景后继续完整检查 `/tmp/kist-client-history-final-check.log`（会话 96959），尚未提交。

独立 worktree 新增 client_history.go/test.go 与 API client_history_list_test.go；共享 readHistory/readHistoryOp，只暴露确认前缀的历史分页/摘要/详情，并通过既有客户端请求回执对正式 op 执行离线 Undo。API history.go 已抽只读接口，server.go 仅新增客户端 historyRoutes 注册；不要复制旧 server 覆盖主线。`/tmp/kist-client-history-list-final.log` library race 通过；API 初次测试将 call 的 key/body 参数写反，已修正。该历史列表增量尚未合入；pending 历史展示、接纳时间及完整同步后历史验收仍待完成。

历史视图取消场景重复 20 次通过（`/tmp/kist-client-history-cancellation.log`），修正后完整 make check 通过（96959 退出 0，`/tmp/kist-client-history-final-check.log`）。91abf03 CI 39612150951 已通过；Visual 39612151105 完整采集、候选校验通过，比较失败，报告下载会话 32874，待审阅后接受或修正，尚未接受基准。

worktree 的历史列表 API race `/tmp/kist-client-history-list-api-final.log` 2.689 秒通过；新增接受时间读取（store.OpAcceptedAt、HistoryEntry.AcceptedAt、wire accepted_at）保持原始 TS 与正式 ID 排序，NAS rebuild 和客户端重放测试通过（`/tmp/kist-client-history-acceptance-final.log`）。首次测试误用带空格的提案 ID，被协议正确拒绝，已修正。历史列表 API 与既有历史版本查询共享并发/超时预算，`/tmp/kist-client-history-list-bounds.log` race 4.209 秒通过。上述均尚未合入；worktree API historical.go/content.go/historical_test.go 已复制主线基底再增量重构，合入时只取新增预算抽取，不重复旧基底。

91abf03 正式视觉报告已完整审阅：baseline 134、candidate 138，均无跳过/失败/重试，134 个已有场景完全匹配，仅四个新增离线面板/保留场景缺基准。六张联系表覆盖全部候选，并逐一查看四张新增原尺寸图，无溢出或布局异常。报告 `/tmp/kist-offline-visual-39612151105/report/index.html`，已发起 Accept visual baselines（run_id=39612151105）；待机器人提交后的 CI/Visual 与本地快进确认。

视觉接受 39614102685 成功，机器人提交 a5658fb 已本地快进；其 CI 39614170713 queued、Visual 39614174487 运行中、Secret scan 39614178163 通过。历史版本阶段仅新增客户端历史 API 契约，不更改页面像素，已完成完整检查后提交，等待基准提交检查完成再推送。

历史版本阶段已提交 e363130（尚待 a5658fb 检查通过后推送）。历史列表/摘要/正式引用撤销、接纳时间字段及详情展示已合入 main：新 store 查询只读 op_sync 的独立列，旧本机事件不填接纳时间；客户端确认前缀按正式 ID 分页，不泄漏 pending 的临时序号或 undone_by。主线 API/library 针对性 race `/tmp/kist-client-history-list-merged.log` 通过（4.395/6.966 秒），History UI 11 项通过。worktree 类型检查因其较旧 API 导出的 schema 缺主线管理字段而失败，未复制这些旧生成文件；主线重新生成两端契约并开始完整检查 `/tmp/kist-client-history-list-complete-check.log`（66379）。所有本节历史列表 worktree 增量现已合入，不能重复覆盖；新 history-synced-operation 视觉场景待本阶段推送后正式采集。

历史列表阶段完整 make check 已通过（66379 退出 0，`/tmp/kist-client-history-list-complete-check.log`）。用新二进制重启已确认身份的合成客户端（19592，会话 84196），合成 NAS（19591，会话 39554）；通过网页同源 API 生成本地创建回执，再手动同步成功。确认测试 NAS PID 9160 后关闭，Chrome 仍可打开最新正式操作及 NAS 接纳时间、无 pageerror。截图 `/tmp/kist-offline-history-detail-without-nas.png` 在等待弹窗透明度稳定后已审阅。浏览器脚本初次断言误加句号，第二次名称同时匹配摘要和对象列表，均修正定位后通过；未改产品行为。合成 NAS 当前关闭，客户端仍运行。

a5658fb CI 39614170713 已通过，但 Visual 39614174487 失败，正在读取失败日志；不能把接受后的检查视为已通过。

历史列表阶段已提交 2d40fb6，尚未推送。a5658fb 视觉复查报告 `/tmp/kist-offline-visual-39614174487/report/index.html`：138 个完整场景，仅既有 entry-menu-box 的“在新标签页打开”文字 326 像素差异，离线四场景及其余场景完全匹配。已查看 expected/current/diff 和文字放大图；a5658fb 与批准的 91abf03 仅 baseline.json 不同，无该菜单产品代码变化。未接受此噪声差异，发起同 SHA 完整重新采集（39614174487 attempt 2），以确认是否为采集不稳定；若重现继续定位。a5658fb 跨平台 CI 已全部通过。

e363130 与 2d40fb6 已推送。原 a5658fb 视觉 attempt 2 一直排队，后被最新 main 的完整检查取代，取消不作为通过证据。最新 CI 39616792702 已通过，Visual 39616792737 失败，正在读取日志/下载报告，需同时确认新增 history-synced-operation 场景和此前菜单渲染差异。

本地意图历史四个新文件已合入 main（client_local_history.go/test.go，API 同名两文件及 clientLocalHistoryRoutes 注册）：按独立本地 ID 分页并读取原始 proposal，保留撤销/操作组引用、替换及放弃决定、已确认 NAS 回执；提交结果未知仍为 awaiting_receipt，不伪装成失败或可放弃状态。分页按追加位置，替换操作虽沿用逻辑队列位置仍有独立游标。新增本地 ID 撤销 HTTP 接口，复用持久请求回执。主线针对性 race `/tmp/kist-client-local-history-merged.log` API/library 通过（1.885/3.050 秒），完整检查 `/tmp/kist-client-local-history-complete-check.log` 会话 9639 进行中。worktree 对应增量全部已合入，不要重复覆盖。测试初次漏传 OpenClient context，随后误以为创建同名归档盒必冲突，已改为同字段并发改名，覆盖真实冲突和替换决定。尚未接本地历史网页详情/撤销入口。

本地意图历史 API 阶段完整 make check 已通过（9639 退出 0，`/tmp/kist-client-local-history-complete-check.log`）。本阶段先提交；2d40fb6 Visual 39616792737 为完整采集后的比较失败（139 张、2 处差异），报告下载会话 40063，尚未审阅/接受。

2d40fb6 的正式视觉报告已复查：完整 139 张，仅新增接纳时间历史场景与既有菜单文字 1px 位移（326 像素）。未接受菜单差异。共享 ContextMenu 现遵循系统 reduced motion，与 Dialog 一致；针对性菜单测试 4 项通过，完整检查日志 `/tmp/kist-context-menu-motion-complete-check.log` 已运行至最终单二进制构建，无失败记录。原终端会话已回收，接下来以该提交的 CI 和正式视觉采集复核，尚不能声称菜单差异已解决。

1676133（含 d086910 本地历史 API）已推送；Secret scan 39619903062 通过，CI 39619903077 排队，Visual 39619903105 采集中。

本地历史网页阶段已接入独立 tab：按本地 ID 翻页、原始时间与同步状态、回执正式编号/接纳时间、替换/撤销/操作组关联、本地撤销确认与幂等重试。未打开 tab 时不请求历史；写入未确认时阻止关闭或切换 tab，复用 useWrites/WriteFeedback。详情摘要复用后端 summarizeHistory，不在前端解析 payload；新增摘要字段和测试已从旧 worktree 按文件增量合入，旧 worktree 现无剩余未合入功能。针对性 UI 6 项通过（`/tmp/kist-local-history-ui-tests.log`），API/library race 2.106/3.384 秒通过（`/tmp/kist-local-history-summary-merged.log`）。首次 UI 测试错误使用了非项目的 Retry 文案与精确文本定位，修正为现有 Try again 和包含状态文字后通过，未更改业务行为。完整 make check `/tmp/kist-local-history-ui-complete-check.log`（45666）进行中；新增 offline-client-history 正式视觉场景，尚未浏览器验收/提交。

本地历史网页阶段完整 make check 已通过（45666 退出 0）。已确认合成客户端 PID 9158 后重启新二进制（33547，19592）；NAS 仍关闭。真实 Chrome 新建独立本地操作、打开详情、确认撤销并返回列表看到撤销事件，无 pageerror（59080 退出 0），截图 `/tmp/kist-local-history-without-nas.png` 已查看。1676133 CI 39619903077 通过；Visual 39619903105 是采集失败而非比较失败，candidate 138 通过/1 失败，box-tags-dialog 的菜单在视口下方导致点击超时，trace 反复报告 element is outside of the viewport。报告 `/tmp/kist-menu-motion-visual-39619903105`。减少动画修正引出了菜单定位问题，须继续修复，不能接受该不完整候选。

本地历史 UI 提交 ecfa19e。已撤回 1676133 的 ContextMenu reduced-motion 改动，恢复原菜单定位/动画路径；该尝试没有证明原 1px 差异根因，却在正式采集中引出边缘菜单不可点击，因此不保留。恢复后完整 make check 通过（57718 退出 0，`/tmp/kist-menu-position-restore-check.log`）。下一次正式视觉仍须完整采集并审阅，原菜单像素差异与新增本地历史场景均未接受。

ab09afa（含 ecfa19e 本地历史 UI）已推送；CI 39622009090 已通过，Visual 39622009268 与 Secret scan 39622009050 仍在运行。

确认后本地原文件生命周期阶段实现中：新增 AdoptConfirmedOriginals，只依据确认日志前缀的 blob 引用迁移，active pending 内容继续受保护；导入准备持共享本地内容租约，迁移非阻塞尝试独占，活跃读取复用对象租约。同步最终拉取目录后执行迁移，同盘重命名且 fsync 两侧目录，再按配额/LRU 清理，离线保留超限不自动删除。未知或未确认原件不动。针对性原有同步/导入测试通过（`/tmp/kist-cache-adopt-initial.log` 16.676 秒），租约增量通过；新迁移测试覆盖未同步保护、活跃读者/导入准备、配额逐出、保留超限、NAS 提交但客户端未确认、重复迁移与受保护体积归零。

检查恢复实现发现：迁移未完成记录若 offset=0，会被缓存重建按续传规则截断已重命名的完整文件；已将初始 offset 记录为已 fsync 原文件的完整大小，Complete 仍为 false，重命名及目录 fsync 后才置 true。新增就绪标记前恢复验证；`/tmp/kist-cache-adopt-recovery-tests.log` race 9.574 秒通过。首次完整检查 `/tmp/kist-cache-adopt-complete-check.log`（11357）仍运行旧偏移代码，不能作为修正后最终验收；待其结束后运行修正后的完整检查。尚未提交本阶段。

首次迁移完整检查（11357）结束为失败：TestHTTPClientResumesDurableBlocksAndLostCommit 假设上传后尚无缓存，但新生命周期已自动转入缓存，故未发生它预期的断线下载。已通过 SetCacheLimits 的真实逐出路径清除已确认缓存，再恢复配额执行 HTTP 续传；针对性 race 4.271 秒通过（`/tmp/kist-cache-adopt-http.log`）。修正后的完整 make check `/tmp/kist-cache-adopt-final-check.log`（12259）运行中，不复用失败结果。

ab09afa 的 Visual 39622009268 完整采集 140 张，5 处差异仅为两个新增历史场景及三个既有离线面板新增本地操作 tab。已查看六张联系表覆盖全部候选，并逐张查看五个差异原尺寸图；entry-menu-box 与所有其余场景匹配，原 1px 差异此次未复现。报告 `/tmp/kist-local-history-visual-39622009268/report/index.html`，接受流程 39623400722 成功。机器人 b724a01 已本地快进，CI 39623456359、Visual 39623459202 运行中，Secret scan 39623461350 通过。先前菜单减动画尝试已完整撤回，不能归因它修复了噪声。

原文件迁移最终完整 make check 已通过（12259 退出 0，`/tmp/kist-cache-adopt-final-check.log`，library race 192.036 秒）。包括修正后的完整偏移记录、导入/读取租约、确认边界、配额/保留与真实 HTTP 续传。接下来提交本阶段并继续跟进 b724a01 的 CI/Visual；尚未完成派生媒体缓存、完整冲突编辑 UI、最终端到端矩阵与使用文档，目标继续保持实施中。

c86910b 原文件迁移阶段已推送，CI 39624124913 与 Secret scan 39624124963 通过。b724a01 的 CI/Visual 被新提交取代，取消不计通过。c86910b Visual 39624125152 完整采集 140 张，仅 entry-menu-box 又出现原有的 326 像素差异；未接受。报告 `/tmp/kist-cache-adopt-visual-39624125152/report/index.html`。此次只在 capture helper 中增加菜单自然淡入完成（opacity=1 且无运行中动画）的就绪断言，避免截图禁用动画时恰处于合成层变化；未再次改产品菜单动画或定位。该稳定性修正需下一次正式采集验证，不能提前宣称根因解决。

派生预览传输/缓存内核阶段实现中：NAS 新增带同步鉴权的 derived/describe 与 derived/content（GET/HEAD/Range），以原文件内容地址和 recipe 为身份，支持无当前文件引用的原件；描述绑定 generation、输出哈希和大小，过期 generation/内容拒绝，不追加归档 op。能力 derived_previews 单独协商。syncclient 复用原件范围校验；library.Client 的 CacheDerived/OpenDerived 复用原件缓存的预算、分块持久化、续传、租约和校验，缓存命中不联系 NAS，不下载原件。派生映射保存已见输出事实供换代后剩余缓存重建，离线原件选择保护当前预览输出；任务未完成不隐藏已缓存兼容预览。

初步共享媒体/API/HTTP 针对性 race 通过，真实 HTTP 覆盖鉴权、哈希、Range、过期 generation 与错误内容身份（`/tmp/kist-sync-derived-http.log` 2.916 秒）。客户端缓存针对性测试覆盖独立下载预览、共享配额/保留/活跃读者、离线命中、两代输出索引重建（`/tmp/kist-client-derived-tests.log` 20.882 秒）。增加中断 20 字节后续传及旧协议拒绝，最终针对性 race `/tmp/kist-derived-final-targeted.log` library/syncclient/api 8.501/2.892/2.127 秒通过。完整 make check `/tmp/kist-derived-complete-check.log`（38995）正在运行。尚未提交；尚未接客户端媒体 API/后台任务、本地生成与完整离线预览准备，不把本阶段视为照片视频离线体验完成。

派生传输/缓存阶段完整 make check 已通过（38995 退出 0，`/tmp/kist-derived-complete-check.log`）。本阶段将内核与契约提交，网页媒体后台队列、本地生成、完整离线准备及最终全场景验收继续待办；正式视觉仍需验证新增的自然动画就绪断言。

客户端媒体 API/队列阶段：新增磁盘任务协调器与跨进程 worker，接现有媒体 GET/POST/DELETE/GET+HEAD Range 路由及 client serve 生命周期。状态读取无网络；实际内容校验/租约复用缓存内核。取消监测中止下载并守卫 generation，重启恢复 running，逐出返回 missing。网页复用预览占位与重试按钮，区分未缓存、配额不足、索引损坏、校验失败与旧协议。针对性队列 race 4.609 秒、真实媒体 API（含 NAS worker、客户端队列、重建 Server 后无连接 Range）race 4.806 秒、Media 前端 26 项通过。补充可丢弃队列损坏恢复用例。完整 make check `/tmp/kist-client-media-complete-check.log`（20860）运行中，尚未提交。6815d33 的 CI 39626848804 和 Visual 39626848732 仍在运行，未宣称通过。

6815d33 的 CI 39626848804 已通过，Visual 39626848732 是采集失败（134 通过、6 失败），不是可接受的视觉差异。日志证实 capture helper 对退出中的菜单固定等待 opacity=1，菜单卸载后定位器永久为空，导致确认/切换视图/组合编辑场景超时。修正为每轮重新查询当前菜单，允许退出后集合为空，仍要求剩余菜单 opacity=1 且动画完成；不修改产品动画、不放宽像素差异阈值。待下次正式采集验证。

客户端媒体队列完整检查通过：首次 make check（20860，`/tmp/kist-client-media-complete-check.log`）退出 0，library race 198.229 秒；视觉 helper 修正后的最终 make check（75200，`/tmp/kist-client-media-final-check.log`）也退出 0。新增损坏队列恢复用例、客户端 API 契约与所有既有检查均通过。正式浏览器视觉待推送后验证，不把静态视觉工具检查当成截图通过；本地生成与全量离线依赖准备继续待办。

本地转换配额准备阶段：新增 cache/workspaces 持久预算与 OS 租约，写入前准入；所有中间文件计入 partial，未使用预算计入 reserved，复用原件/派生缓存的 LRU 与上限。活跃任务在其他客户端实例清理、降低配额时仍受保护；失去租约的残留（包括半创建与损坏记录）可由缓存扫描/重建清理。整目录监测与发布前最终检查拒绝超预算输出，不隐瞒采样间隔内的实际超额。针对性 race `/tmp/kist-workspace-targeted.log` 5.516 秒通过，覆盖预留竞争、跨 Client 活跃租约、TTL/降低上限、保护未同步原件、残留恢复与快/慢超额转换。此阶段尚未接本地 renderer；外部工具子进程的崩溃生命周期需在接入时处理，不能仅凭父进程租约消失就假定所有子进程已退出。

转换工作目录阶段完整 make check 已通过（24331 退出 0，`/tmp/kist-workspace-complete-check.log`，library race 201.358 秒）。尚未接 renderer，下一阶段需完成本地产物发布、队列调用与外部子进程生命周期。af021c8 的 CI 39629459590 全部通过；Visual 39629459500 完整采集 140 张，原 6 个退出菜单超时消除，但 entry-menu-box 仍有 326 像素差异，其余 139 张匹配。已查看完整该页及差异图，仍集中在“在新标签页打开”一行；自然动画等待尚未消除此既有差异，未接受基准。报告 `/tmp/kist-client-media-visual-39629459500/report/index.html`，需继续定位而非放宽阈值。

本地预览生成阶段实现中：新增 GenerateDerived、RenderWorkspace 和共享缓存发布路径，client serve 配置专用 render-worker 子进程，媒体队列在原件本地可用时先转换，缺工具可尝试 NAS。工作目录改为共享使用租约/独占清理；父服务取消或退出通过 stdin 管道 EOF 通知监督进程取消，父侧等待其退出，避免先杀监督进程导致租约提前释放。原件与工作目录在监督进程中独立持有租约。产物大小/哈希/fsync 后，同盘移动进入共享缓存，映射/未完成记录先落盘；与临时预留在同一临界区交接。

本地照片/队列针对性 race library 14.361 秒、API 5.078 秒通过（`/tmp/kist-local-render-targeted.log`）；真实独立进程 PNG 转 JPEG、取消等待清理、强制终止父进程触发管道 EOF 的 CLI race 4.777 秒通过（`/tmp/kist-client-render-process-tests.log`）。已补同哈希输出防自锁与错误大小不发布测试。尚未完整 make check 和浏览器验收；监督进程自身被强制终止、真实外部工具故障矩阵仍需验证，不能以父进程退出测试涵盖全部情况。

20c33e0 的 Visual 39631069081 本轮全部通过，未修改基准；先前 326 像素差异未复现，但不能据此宣称已定位并消除了非确定性。对应 CI 39631069067 仍排队，持续跟进。

本地生成阶段最终针对性 race 通过：`/tmp/kist-client-render-final-targeted.log` library 5.905 秒、cmd 4.519 秒，覆盖输出大小不符拒绝发布、同哈希产物不自锁。完整 make check 首轮（86697，`/tmp/kist-local-render-complete-check.log`，library 206.170 秒）通过；监督进程补充 Ctrl-C/SIGTERM 的 NotifyContext 后，最终 make check（43070，`/tmp/kist-local-render-final-check.log`）也通过，未变化的核心库复用测试缓存。

真实 Chrome 离线验收通过：NAS 19591 无监听，合成客户端 19592 上传 PNG 与 `pattern.heic`，网格缩略图均解码显示；HEIC 详情 ready、engine=libheif，页面错误为空，转换后 partial/reserved 都为 0。随后上传 FFmpeg 生成的 1 秒 MP4，客户端 engine=ffmpeg、ready，Chrome 实际播放时间推进到 0.127859 秒，页面错误为空。日志 `/tmp/kist-local-previews-browser.log`、`/tmp/kist-local-video-browser.log`；截图 `/tmp/kist-local-previews-offline.png`、`/tmp/kist-local-video-offline.png` 已逐张查看。隔离演示 box 为 `01a15f42-dbc4-7d84-9a61-7bb6a7cec8c6`，HEIC 为 `01a15f42-dd09-7850-9ac7-6bae1829da34`，视频为 `01a15f43-ac63-74e5-9f80-4523fbb894fc`；原客户端旧进程 14598 已正常停止，新服务会话 94665 继续运行。

20c33e0 的 CI 39631069067 首次失败原因是 macOS arm64 作业未获得 hosted runner（steps 为空），其他代码、平台和容器任务通过；已仅重跑失败作业，尚排队。Visual 39631069081 全部通过。下一步仍须将派生依赖纳入“准备离线”的完整就绪判断，继续冲突编辑 UI 与完整故障/回归矩阵，不把本地预览阶段当作 Goal 完成。


离线预览准备阶段：将原件和缩略图/详情预览共同纳入 OfflineStatus 与 PrepareOffline；网页增加预览完成数量，CLI 同步输出。共享 derive 规则表生成前端选择表，原件同哈希但不同格式名称的依赖取并集；本地可生成时不读取连接凭据，远端排队等待可取消。补充本地 PNG 原件已在但缩略图未就绪、生成后保留/降低配额、缓存字节丢失、准备期间取消选择、GPX 配方去重与跨格式共享内容、NAS 预览下载及断网复用测试。前端 Media/profile/OfflineEntry 39 项通过，包括原件已齐全而预览缺少时仍发起准备。完整检查与正式视觉仍待执行。

546ec99 的 CI 39634360031、Visual 39634360137、Secret scan 39634360094 均通过。20c33e0 的失败作业重试被新提交取消；最新完整 CI 已包含其全部代码并通过。

离线预览准备完整 make check 通过（16778 退出 0，`/tmp/kist-offline-previews-complete-check.log`），前端 274 项、library race 213.726 秒。真实 Chrome 隔离验收通过：NAS 19591 无监听，客户端 19592 更新构建后，通过缓存配额清理可逐出预览；HEIC 初始原件 1/1、预览 0/2、ready=false，经“检查并下载”在本机生成后显示预览 2/2、ready=true，pageErrors 为空。日志 `/tmp/kist-offline-preparation-browser.log`，截图 `/tmp/kist-offline-preparation-browser.png` 已查看；新版演示服务会话 29059。正式视觉待本阶段推送后验证，Goal 仍有冲突编辑、批量准备与完整故障验收等未完成范围。


冲突对照编辑阶段实施中：新增只读 ConflictEdit 草稿接口，从阻塞操作之前的有效前缀取当前值；文件/目录、标签重命名和归档盒名称/描述可编辑，其他操作仍需各自编辑入口。保存复用原有 Resolve 验证，原始提案不变；草稿身份和前提固定，前端在保存结果不确定时复用请求，关闭编辑解除父层锁定。新增文件/标签/归档盒合并、查看后 NAS 再次改动拒绝旧草稿、无关字段保留、同步前后重试幂等及本地历史保留测试，针对性 library 3.157 秒通过；前端两文件 5 项通过。新增正式视觉 offline-client-conflict 场景。首次完整检查因测试查询多余 exact 参数被 TypeScript 拒绝，已修正并运行 `/tmp/kist-conflict-final-check.log`（4039）。尚未提交，复杂冲突编辑仍待后续完成。

冲突对照完整检查通过：`/tmp/kist-conflict-final-check.log`（4039 退出 0），library race 217.271 秒。真实 Chrome 双实例流程通过：隔离 NAS 19593 与客户端 19594 对同一归档盒分别改名，在网页查看原值 Original travel、当前 NAS travel、本地 Laptop travel，改为 Reconciled travel 并同步；NAS 最终名称正确，独立描述 Keep this NAS note 保留，pageErrors 为空。日志 `/tmp/kist-conflict-browser.log`、截图 `/tmp/kist-conflict-browser.png` 已查看。合成目录由 `/tmp/kist-conflict-ui-fixture-path` 指向，服务会话 NAS 53540、client 24887。

1d36d5a 的 CI 39637017893 全部通过；Visual 39637018072 完整采集 140 张，138 张匹配。offline-retention-file 的 24299 像素变化对应新增预览数量和就绪文案，已查看符合预期；entry-menu-box 再次出现原有“在新标签页打开”一行的 326 像素变化，已查看但尚未定位原因，因此未接受基准。报告 `/tmp/kist-offline-previews-visual-39637018072/report/index.html`。下一次采集应补充菜单布局/字体诊断证据，不能放宽阈值或直接接受此非预期差异。

菜单诊断只在截图之后附加各菜单项的 DOM/text 矩形、字体、行高与 transform，不修改等待条件或像素阈值。最终 make check `/tmp/kist-conflict-diagnostics-final-check.log`（92667）退出 0，未变化的 Go 代码复用此前完整 race 缓存；全部前端契约、格式、测试与构建通过。此前 diagnostics-check 的格式提示已修正，不能算作通过证据。本阶段可提交；复杂动作的领域编辑和菜单偶发像素差异仍未收口。


日期与 fileset 冲突编辑实施中：草稿增加 start_at/end_at 字符串前提，界面复用 PartialDateInput，保持原精度；fileset 草稿保存原配置、有效前缀的当前配置及原本地期望配置，复用 FilesetEditor 的主文件、成员、角色与拖动排序，提交走同一 Resolve。增加只读配置对照，不复制第二套编辑控件；未找到成员不能被复活或静默丢弃。共享编辑器在未知提交结果期间禁止另建写请求。针对性 library 4.609 秒、前端 17 项通过；新增完整缺失成员拒绝与正式 fileset-conflict 视觉场景。`/tmp/kist-conflict-fields-complete-check.log`（22779）正在运行；尚未提交或宣称完成最终验收。

日期/组合完整检查通过：`/tmp/kist-conflict-fields-complete-check.log`（22779 退出 0），library race 220.660 秒。实际 Chrome 双实例验收也通过：在隔离客户端 19594 对组合拖动页序、保留独立主文件并手动选定 NAS 布局，最终 NAS 配置与选择一致，pageErrors 为空（`/tmp/kist-fileset-conflict-browser.log`，截图同名 .png 已查看）。日期冲突从本地年精度 2026 与 NAS 日精度 2026-10-22，明确改为 2026-11；同步后仍为月份精度，页面错误为空（`/tmp/kist-date-conflict-browser.log`，截图已查看）。新版客户端服务会话 27577；未操作个人库。

601901b 的 CI 39639091847 全部通过。Visual 39639091637 完整采集 141 张，无失败/跳过/重试；139 张匹配，只有新增 offline-client-conflict 与已审阅的 offline-retention-file 变化。完整候选通过六张联系表 `/tmp/kist-conflict-review-{1..6}.png` 扫描，新冲突页及变化区域全尺寸查看；旧菜单 326 像素差异本轮未出现，仍不声称已根治。已接受精确运行，Accept visual baselines 39640663501 成功，bot 提交 0282bec 已 ff-only 合入且保留当前工作树；其 CI 39640731826、Visual 39640734918 尚在执行。当前新增组合场景需随下一提交继续正式验收。

包含最终视觉定位文案与已接受基准的 make check 再次通过（59940 退出 0，`/tmp/kist-conflict-fields-final-check.log`）。组合普通编辑入口仍通过原有排序/删除/主文件测试；冲突入口新增日期精度和组合完整 before/after 的前端回归。此阶段可提交，新增组合视觉场景的正式结果需推送后跟进。

离线保留管理阶段：缓存页列出持久选择，兼容文件菜单创建的单项与 CLI 的命名混合选择；单项读取当前名称，解析失败仍保留身份和取消入口。校验按需执行，不随打开列表批量哈希内容；重新准备直接调用现有 prepare，不以旧 UI 状态 PUT 覆盖成员；删除仅解除指定 owner。就绪文案复用单文件入口组件，保留原布局。新增前端四项回归，三文件共 11 项通过（`/tmp/kist-retention-targeted.log`）。首轮完整检查暴露新增 i18n 参数类型遗漏，修正后 `/tmp/kist-retention-final-check.log`（9483）退出 0；最终保留原布局后的交付检查 `/tmp/kist-retention-delivery-check.log`（67631）仍在运行。

实际 Chrome 已通过命名混合选择准备、取消及共享保留不受影响的验证（`/tmp/kist-retention-browser-final.log`），pageErrors 为空，截图 `/tmp/kist-retention-browser.png` 已查看。仅使用既有合成双实例，客户端新会话 65046。初次脚本用了不准确的按钮文案，修正为实际“取消离线保留”后重跑通过；不是产品失败。网页批量选择创建入口仍待实现，本阶段只完成集中管理现有选择。

180e9c7 的 CI 39641189898 与 Secret scan 39641189894 均通过。Visual 39641189893 完整候选 142 张：新增 offline-client-fileset-conflict 已全尺寸查看，配置对照/拖动列表正常；仍有既有 entry-menu-box 的 326 像素差异，因此未接受基准。新增 menu-layout 附件首次确认：该菜单 baseline/candidate 所有项的元素与文字矩形、字体、行高、transform、zoom 完全相同，不能再把差异简单归因于 DOM 布局位置，需继续排查栅格化/合成因素。报告 `/tmp/kist-fileset-fields-visual-39641189893/report/index.html`。

保留管理最终 make check 已通过（67631 退出 0，`/tmp/kist-retention-delivery-check.log`），前端 282 项通过，未修改 Go 逻辑复用通过的 race 缓存。真实浏览器已验收上述行为；本次缓存页的正式视觉差异待推送采集后审阅。批量创建选择、其他领域冲突与完整故障矩阵继续属于未完成 Goal，不在此阶段缩减范围。


批量离线准备阶段：文件网格/列表增加客户端专属选择模式，卡片按钮使用 aria-pressed 与共享 Checkbox.Indicator，保留现有虚拟化及显示方式；普通目录不被错误地当作 fileset 批量保留。“选择已加载项目”明确只选当前已加载结果，按 file/fileset 逻辑身份去重。批量弹窗先逐项保存所有保留选择，再按顺序核验与准备，显示保留/完成数量、当前对象、取消与重试；共享内容由后端既有哈希缓存复用，每项可在缓存页独立管理。单项和批量入口复用相同 retain/prepare helper，不复制请求契约。

新增批量取消、失败重试跳过已准备项、全部保留先于下载、成员变化不误报成功，以及文件网格选择而非打开的测试；针对性 41 项通过（`/tmp/kist-batch-targeted.log`）。完整 make check（39918，`/tmp/kist-batch-complete-check.log`）退出 0，前端 287 项通过，Go race 复用已有通过缓存。随后修正正式列表场景应等待 0 个缩略图（列表本来不渲染媒体），独立视觉工具检查 `/tmp/kist-batch-final-visual-tools.log` 通过。

真实 Chrome 混合准备通过：合成 Packing-list.txt 与 Documents 多页组合一起选择，切换列表保持两项选择，准备完成 2/2，持久清单分别含 file 与 fileset owner，pageErrors 为空。`/tmp/kist-batch-browser-final.log`；网格、列表、窄屏与完成弹窗 `/tmp/kist-batch-{grid,list,mobile,prepared}.png` 已查看。初次截图捕获过渡动画，后改为等待菜单退出及禁用截图动画，稳定状态复核正常。仅更新隔离演示库，客户端会话 58981。正式新增批量视觉场景待推送后采集；全面故障矩阵、其他冲突编辑与最终需求审计继续待办。

3385714 的 CI 39643218000 和 Secret scan 39643218032 均已通过。Visual 39643218095 的两套采集完整（candidate 142、baseline 141，无失败/跳过/重试），仅 offline-client-cache 的预期内容变化与新增 fileset-conflict，旧菜单差异本轮未出现。缓存页全尺寸审阅发现 900px 高度下底部关闭按钮需要滚动，故未立即接受；将保留项名称/数量合为紧凑一行，随本批量阶段重新采集。新增深色批量选择场景；最终完整检查 `/tmp/kist-batch-final-check.log`（31286）正在运行。

本批量阶段最终 make check 已通过（31286 退出 0，`/tmp/kist-batch-final-check.log`）。紧凑缓存布局已用最终二进制在真实 Chrome 1280×900 验证，两个保留项的名称与数量同行，关闭按钮完整处于视口内（`/tmp/kist-retention-compact-browser.log`，截图已查看）；隔离客户端最新会话 30380。准备提交本阶段，正式批量五场景和缓存布局仍需新提交的 Visual 核验。
