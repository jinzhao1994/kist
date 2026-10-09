# M5 派生媒体实现记录

权威设计：`docs/design/2026-08-22-kist-v0.1-design.md` §3.6、§5.3、§7。本文不替代待决设计。

## 阶段一：Docker 媒体运行环境

用户确认以 Docker 承载内置 ffmpeg 的完整环境，同时保留独立二进制交付。实现 Docker 多阶段构建、非 root 运行、整个库的持久化卷、显式初始化与本机端口发布。镜像构建上下文使用白名单，排除本地归档、依赖缓存与 Git 数据。未发布 registry 镜像，未修改发布工作流。

验收（2026-10-10）：

- `make check` 通过，含 40 个前端测试、Go race 测试与单二进制构建。
- 本机 Docker `linux/arm64` 构建通过；镜像 ffmpeg 7.1.5。
- `scripts/docker-smoke.sh` 通过：非 root、WebP、H.264/AAC 实际编码，生成 PNG 后导入测试库，网页和 API 可读，未知 Origin 返回 403，删除并重建容器后文件仍在，`verify --deep` 与 `scrub` 通过。
- 独立 Compose 测试项目完成初始化并在本机 18473 端口返回库信息；测试容器、网络和测试卷已清理。
- CI 增加镜像构建与同一 smoke 脚本，覆盖 Linux amd64；不把仅运行 `ffmpeg -version` 当作媒体能力验收。

## 阶段二：媒体生成器

新增独立 `internal/derive`，架构测试禁止它依赖任何仓库内包，无法修改数据库或 op log。普通图片由纯 Go 解码，先检查输入大小和像素预算，处理 JPEG/PNG/WebP/TIFF 的 EXIF 方向，再按比例缩小并编码 JPEG；透明底色为白色。视频由受限 ffmpeg 子进程生成 H.264/AAC MP4，支持超时、取消、输出体积检查与有界诊断。

实测发现 ffmpeg 默认仅解出 tiled HEIC 的一个 512×512 图块。为避免静默产出错误照片，HEIC 改用独立 libheif 工具拼图和应用变换，Docker 与 Linux CI 同时安装它；Go 二进制不链接 CGO。仓库新增自行生成的 128×96 与 3072×2048 HEIC 像素样本，后者含 24 个图块。多顶层图像集合暂明确报 unsupported，不猜测主图。

生成器测试覆盖：无外部工具的普通图片、八种 EXIF 方向、PNG EXIF、透明底、尺寸限制、未知 recipe、缺依赖、取消子进程、诊断长度、真实短视频和完整 HEIC 拼图。Docker smoke 同样检查完整 HEIC 尺寸。

## 阶段三：持久队列与 HTTP 接口

`library.Media` 管理 derived 下的 JSON 状态与原子输出，短状态锁和独立 worker 锁协调进程。请求按 blob/recipe 去重，保留碰撞后缀；取消和显式重试更换代次，旧 renderer 即使忽略取消也不能发布。serve 退出时停止并等待 worker；接手 worker 锁的进程重做未完成任务。缺依赖每 30 秒重新尝试，普通失败只显式重试。已删除的派生结果重新请求即可生成。

`/api/v1/files/{id}/derived/{recipe}` 提供 GET 状态、POST 请求/重试和 DELETE 取消；`/content` 提供 GET/HEAD、ETag 与范围读取。这些是可丢弃运行状态，不产生 op 或撤销历史，也不使用归档写入回执。未提交请求返回 missing；未就绪内容返回 `media_not_ready`（409）。支持三种版本化 recipe，未知 recipe 返回 400。

测试覆盖跨控制器共享状态、内容去重、碰撞路径、取消/重试代次隔离、重启恢复、自动补做、并发 worker 互斥、生成期间归档写入、深度校验不受影响，以及 API 状态流转、跨站保护和内容范围读取。

## 阶段四：Web UI 与端到端

网格只请求派生缩略图，保留虚拟滚动和最多 8 个活动媒体的限制；HEIC 与视频也受同一限制。详情请求图片预览或 MP4，状态包括等待、生成中、缺依赖、失败、取消；提供取消与显式重试，保留原文件下载。API 客户端与 Go OpenAPI 契约同步。前端测试覆盖按需请求、缺依赖、取消后重试、输出被删除后的重建、视频地址与网络失败。

2026-10-10 本机验收：`make check` 通过（44 个前端测试）；Docker 冒烟新增真实 API 请求，分别生成 PNG/HEIC 缩略图、大图和 H.264/AAC 视频，检查内容范围读取，容器重建后再次读取，深度校验与 scrub 通过。Chrome 扩展在独立测试库验证了 2048×1365 完整 HEIC、640×360 视频实际播放到 2.02 秒结束、损坏图片失败占位、缺依赖提示和取消/重试按钮。

## 收尾与验收结论

M5 已完成。额外验证：

- 独立测试服务以空工具搜索路径启动，Chrome 明确显示缺依赖，普通 PNG 仍正常预览；取消后显示取消状态，显式重试重新排队。向该进程的工具目录加入真实 ffmpeg/libheif 后，不重启服务，HEIC 预览自动恢复到 2048×1365，浏览器控制台无错误。
- 在实际 HEIC 任务处于 running 时 SIGKILL 测试服务，磁盘保留 running；新服务接手后同代任务恢复 ready，生成耗时 304 ms，遗留 `.render-*` / `.heif-*` 清理干净。清理只作用于合法 blob 目录中的内部临时名称，保留其他文件。
- Ubuntu CI 曾发现 libheif 工具已安装但缺 HEVC 插件，现显式安装 `libheif-plugin-libde265`；工具报告缺 codec/encoder 时归为缺依赖，支持自动补做，而不是显示为文件损坏。
- 最终 `make check` 通过（44 个前端测试、Go race、格式、静态检查与单二进制构建）；Docker 重建后重复真实派生请求、范围读取、持久化及深度校验。
- 队列/API 提交 `5a42ae5` 与 UI 提交 `7af8e3d` 的 GitHub Actions 四项均通过：Linux 主检查、macOS、Windows、容器。

本机验收截图：`/private/tmp/kist-m5-evidence/heic-restored.png`。临时样本均为自行生成的色条、短视频和损坏测试字节，无个人内容。归档库原始数据未作为测试输入。Docker 镜像仅构建与验收，未发布 registry 或版本标签。

支持边界：多顶层图像 HEIF 集合明确降级；外部工具必须具备相应 codec，不能承诺任意 HEIC/视频都可解码；超出尺寸、时长或输出预算时保留原文件下载。更通用的同步任务框架仍由 B2/B7 管理。
