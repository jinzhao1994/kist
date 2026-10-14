# 网页上传端到端校验

范围：完整 multipart 上传的端到端校验，不实现断点续传或分片；不发布、不修改 NAS。权威决策见设计文档「请求去重与上传」。

## 阶段与验收

1. 存储与导入：按独立预期 SHA-256/长度写入 blob；拒绝整个错误批次；准备结束后重新读取最终 blob，覆盖去重。保留普通 CLI 导入的部分成功语义。
2. HTTP 与浏览器：必填客户端摘要/长度；接收时核对，再传递给 library；Worker 顺序分段增量计算；取消、重试、回执核对与明确进度阶段；旧客户端明确拒绝。
3. 验收：API 故障与幂等回归、Worker/界面测试、真实浏览器、内存与响应性测量、make check、GitHub Actions、审阅视觉差异。

## 当前证据

- 第一阶段相关包 `go test ./internal/library ./internal/blob` 通过。
- 新增测试覆盖空文件、正常内容、去重、错误摘要、错误长度、截断、取消后的临时文件清理、整批不归档，以及最终 blob 在准备期间被修改。
- 底层阶段完整 `make check` 通过，提交并推送 `a901c4e`。错误码守卫要求每个 sentinel 有独立码，已分别登记 blob 与上传校验错误。
- HTTP 和浏览器已接入必填 SHA-256/长度；新增 API 错误内容、错误长度、截短及旧清单拒绝测试通过，确认 op 数量不变、列表无文件、上传暂存清理。
- 浏览器采用锁定版本 `@noble/hashes` 2.4.0（MIT、无运行时依赖），Worker 产物约 6.21 kB。逐文件顺序处理，每次最多读取 4 MiB；独立 Node crypto 摘要对照、分段读取边界、Worker 取消和失败测试通过。真实浏览器与 Node 内存测量见后文；这些测试不证明浏览器实际峰值内存。
- API/界面现有重试与结果核对测试通过。第二阶段完整 `make check` 通过，提交并推送 `d7fe4d0`；GitHub 主 CI 与 Secret scan 通过。视觉报告已审阅：差异来自此前用户要求的侧栏标题改名。已通过 Accept visual baselines 运行 38499811474 接受 3944dce 的报告 38499200159。
- 每个代码阶段已运行完整 make check；验收脚本及进度阶段回归补充后再次通过。最终提交的远端 CI 仍需核对，不推断尚未实测的平台或文件规模。

## 浏览器验收进展

- macOS Chrome，通过扩展操作本机开发资料库，新增独立归档盒「上传校验验收 20261014」（`01a13814-ad9f-70e6-b253-848de41d62cb`）。
- 文本样本及 256 MiB 二进制样本通过真实文件选择器上传成功，页面显示文件已保存。256 MiB 计算阶段观察到 0% 和 66% 进度；尚不能据此给出峰值内存结论。
- 验收发现 react-dropzone 的 disabled 会给整个区域添加 aria-disabled，使取消按钮也被识别为禁用。已将禁用限制在文件接收行为和输入框，保留取消/重试按钮可访问；新增回归测试、完整检查和 Chrome 计算期间取消复验通过，修复提交 3944dce。
- 底层浏览器性能采样接口调用不受支持；未将该失败当作内存测量结果。随后已完成 1 GiB 上传和 Node 测量，详见下文；32 GiB 和其他平台均尚未实测。

## 内容与内存测量（2026-10-14）

- Chrome 1 GiB 哈希期间「取消」立即可操作，显示取消提示；同一操作重试成功。页面出现三个样本文件。
- 使用独立 Python hashlib 重新读取三个原文件与数据库所引用的实际 blob，长度和 SHA-256 全部一致：文本 50 字节；256 MiB 摘要 `486cc817b95d853d3c357ff283b204c0144bd255e73fe2deb1389493b257e3c0`；1 GiB 摘要 `49bc20df15e412a64472421e13fe86ff1c5165e18b2afccf160d4dc19fe68a14`。
- `web/scripts/benchmark-upload-hash.mjs` 复用生产 hashFile，在 Node v24.21.0 Worker 中读取磁盘 Blob，每块完成后采样 memoryUsage；主线程 10 ms 定时器观察调度。1 GiB：5.389 秒，峰值 RSS 178257920 字节、ArrayBuffer 79765471 字节，主线程最大间隔 11.34 ms。3 GiB：16.584 秒，RSS 178962432 字节、ArrayBuffer 79765471 字节，最大间隔 11.48 ms。采样峰值不是操作系统连续峰值；这些是 Node 数据，不声称是 Chrome 内存测量。
- 一次 4 GiB 测量被判无效：Node openAsBlob 返回 size=0。没有把空内容的哈希结果当成大文件成功；测量脚本增加了与 stat 的长度检查。这不证明浏览器 File 存在同一问题。32 GiB、移动浏览器及其他操作系统尚未实际验收。
- 真实 Chrome 验收使用 macOS、本机 HTTP、最大完整文件 1 GiB；主线程响应性的直接证据是计算期间可取消，未测量 Chrome 的帧延迟和峰值内存。每次读取不超过 4 MiB、顺序等待单个分块的边界另有自动测试。

## 逐项验收依据

| 要求 | 证据 |
| --- | --- |
| 原文件 SHA-256 与字节数 | upload-hash.ts 分段哈希；Node crypto 独立摘要对照；真实 Chrome 三个样本与最终 blob 独立对照一致 |
| 有界内存与主线程响应 | 4 MiB 单块读取测试；1/3 GiB Worker 内存测量；真实 Chrome 在 1 GiB 计算中可取消；限制见上文 |
| 接收、暂存导入、最终 blob | receiveUpload 独立比较；PutVerified 放置前拒绝；PrepareFiles 最终 Rehash；内容损坏/长度/截断/最终 blob 改动测试不提交 |
| 整批拒绝与清理 | API 测试确认只有原建盒 op，文件列表为空，上传暂存为空；blob 测试确认拒绝后无暂存和正式内容；已验证但未引用 blob 沿用孤儿规则 |
| 取消、重试、回执 | 原 API 并发与重启后幂等回归通过；前端同 key/manifest 重试、刷新后回执核对测试；真实 Chrome 取消后重试成功 |
| 旧客户端兼容 | 清单摘要和长度必填；旧格式返回 400 和刷新提示；旧服务端拒绝新增字段；README 明确需刷新及不支持续传 |
| 不提前显示完成 | api.test.ts 验证 upload.onload 只进入 processing，直到服务端响应前 Promise 不完成；哈希失败不创建 XHR |
| 架构与写入不变量 | 正式提交仍调用 PreparedImport.Commit，经 library.write；未新增 action/schema/index；make check 的架构、race、rebuild 等既有回归通过 |
| 本机服务 | 开发资料库 /private/tmp/kist-m6-browser.Iw2nEr，127.0.0.1:8473；未修改 NAS、未发布版本、未创建 PR |

本记录包含实际测试范围及限制，不声称 32 GiB 整批或移动端经过真实浏览器验收。正式视觉图仍在 Actions artifacts，Git 只保存已认可提交引用。
