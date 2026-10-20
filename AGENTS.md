# kist

自部署的个人归档应用。基本单元是 **box**（一次活动、一段经历、一批档案），配合一棵全局 **tag** 树做正交检索。既装照片视频，也装证件、合同和报告。

Go 单二进制 + SQLite，Apache-2.0。SQLite 是唯一数据库后端。

## 先读设计文档

开始设计或修改代码前，先读：

- `docs/design/2026-08-22-kist-v0.1-design.md`：权威设计，记录已经决定的结论、被否掉的替代方案及理由。
- `docs/design/open-questions.md`：尚未决定的问题及其触发条件。

`docs/plans/` 是已执行计划和实现记录，不高于设计文档。不要把本文件扩写成决策流水账；它只保存稳定、当前、会直接影响工作的规则。

如果任务碰到开放问题，不要静默替项目作决定。若新证据需要推翻既有设计，必须就地更新设计文档，保留旧方案并写清它为什么失效，以及为什么不选其他方案。

## 四条不能破的不变量

1. **权威数据只有 `ops/` 和 `blobs/`，两者都是普通文件。** `kist.db` 整个可重建，`ops` 表也在内；`boxes/` 是人可读投影，损坏后由 `kist relink` 重建。
2. **`library` 是唯一写入口。** 写入先 append + fsync 到 `ops/`，再在一个事务里 apply。崩溃只允许数据库落后于日志，绝不允许数据库超前。`cmd` 和未来的 `api` 不得直接使用 `store` 写接口。
3. **`Apply` 必须是纯函数。** 不生成 ID、不读时钟、不读取配置、不做依赖当前状态的决策。所有决策在生成 op 时完成，结果写进 payload，保证 `kist rebuild` 重放一致。
4. **payload 是不可查询的自足载荷。** 需要查询的字段必须提取成列或索引表；反过来，每张索引表必须能从 payload 单向重建。

## 当前状态

M0–M7 已完成，当前已发布 v0.1.5。包含可校验续传、同名上传处理、目录与归档盒总大小、多格式只读预览、归档盒封面、标签与右键管理、收藏与分段搜索、操作历史与恢复，以及四平台安装包和 GHCR / Docker Hub 双架构正式镜像。op log 位于 `ops/YYYY/MM/YYYY-MM-DD_ops.jsonl`。M7 基础验收见 `docs/plans/2026-10-10-m7-release.md`，版本差异与升级边界见 `docs/releases/`。v0.1 永久保留历史内容，不实现 GC。下一阶段尚未立项，新增功能先对照设计文档及待决问题，不把远期方向视为已批准的实现任务。

更新这个状态时替换旧文字，不要在下面继续追加历史。

## 架构边界

- `internal/domain`、`internal/blob`、`internal/journal` 位于底层，不依赖仓库内其他包。
- `internal/query` 只依赖 `domain`；`internal/wire` 只依赖 `domain`。
- `internal/oplog` 不得依赖 `store` 或 `blob`；`Apply` 只通过它定义的 `Tx` 契约工作。
- `internal/projection` 不得依赖 `store`、`oplog` 或 `library`。
- `internal/integrity` 不得依赖 `oplog`，因此修复路径结构上写不出 op。
- `internal/library` 编排日志、事务、blob 与投影，是唯一写入口。
- `cmd/kist` 和未来的 `internal/api` 只能通过 `library` 修改状态，不得直接依赖 `store` 或 `oplog`。
- 进程内不得保存会被另一个进程写操作悄悄作废的状态。

这些边界由 `internal/arch` 的测试守护。改变依赖方向前，先确认不是在绕过结构保证。

## 工作方式

- 开始前检查工作树，保留用户已有修改，不顺手整理无关代码。
- 优先从源码、schema、签名或单一注册表推导事实，避免新增靠人同步的清单。如果推导会让守卫变成恒真式，则保留独立事实并用测试对照。
- 新 action 必须明确 handler、inverse/严格撤销语义、targets、是否改变内容、op 索引以及 rebuild 行为。
- schema 或物化视图变化必须同时检查普通写路径、追赶、rebuild、snapshot 和 `verify --deep`。
- 涉及递归层级的 SQL 使用 `UNION` 保证损坏数据中也能终止，除非设计文档明确改变了这个约束。
- 不为未出现的规模问题提前加缓存或冗余列；先测量，再按设计文档记录的逃生舱处理。
- 一个阶段的工作完成、相关检查通过后，及时 commit 并 push，利用 GitHub Actions 尽早发现跨平台问题。不要把多个已经可独立验证的阶段长期积在工作树里。
- commit message 使用中文，说明这一阶段交付了什么。push 后检查对应的 GitHub Actions；失败时继续修到通过。不要自行创建 PR 或修改发布状态，除非用户明确要求。
- 不要在子目录创建 `AGENTS.md` 或 `AGENTS.override.md`。包括未来 `web/` 的重要 agent 规则都维护在根目录这一个 `AGENTS.md` 中，并保持精简。
- UI 修改先读 `docs/design/ui-guidelines.md`；实现前先搜索已有同类入口、组件、样式和交互，优先复用，包括悬停提示的定位与透明度等细节。确实没有适用实现时才新增，并优先补到共享组件或语义样式；不在不同页面复制另一套控件外观，修改后检查同类入口。交互、默认状态或请求变化时，同步视觉场景的断言和 API fixtures。
- `make check` 不执行真实浏览器截图。UI 阶段完成后按 `docs/testing/visual.md` 区分采集失败与预期差异，审阅完整候选后更新基准；不能以“CI 还在跑”长期跳过失败。
- 发布按 `docs/releases/README.md`：先收齐文档、测试及视觉基准，再固定候选 SHA 做四平台预演；发布、镜像可拉取、NAS 已升级是不同状态。通用技能不得替换项目的三段版本号、授权范围及验收流程。

## 语言

- 设计文档、`README.zh-CN.md`、commit message：中文。
- 代码、注释、godoc、`README.md`、issue/PR 模板：英文。

分界线是谁读它：设计文档是作者的思考工具，代码和仓库门面面向外部贡献者。

## 验证

- 开发时先运行与改动直接相关的测试。
- 代码变更交付前运行 `make check`；它与 CI 的主检查一致，包含前端契约/格式/测试/构建、`go mod tidy -diff`、gofmt、`go vet`、`go test -race ./...` 和单二进制构建。首次开发先执行 `npm --prefix web ci`。
- 只有文档变更时至少运行 `git diff --check`，无需为此运行完整 Go 测试。
