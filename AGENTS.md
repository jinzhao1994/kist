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

**下一步是 M1b**：op log、`library` 的写入口、`boxes/` 投影，以及 `box new` / `tag` / `import --box` / `ls` / `find` / `relink`。M1a **一条 op 都没写**——现在这个库能建出来、能校验，但还进不去东西。

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
