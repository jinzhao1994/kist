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

## 后续阶段

尚未完成：生成时机与持久队列边界的确认、recipe 与派生状态、图片解码与 orientation、缩略图/预览图、视频队列、API/UI 接入、有效 HEIC 像素样本验收。Docker 内置依赖不等于这些功能已实现；M5 仍在进行中。
