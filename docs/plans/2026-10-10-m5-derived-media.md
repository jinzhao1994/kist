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

## 后续阶段

尚未完成：持久派生状态与后台队列、原子发布与重启恢复、API/UI 接入、浏览器端完整验收。生成器已独立实现，尚未接入用户请求。Docker 内置依赖不等于这些功能已实现；M5 仍在进行中。
