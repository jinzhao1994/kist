# kist

[English](README.md) · **中文**

自部署的个人归档应用。基本单元是 **box**——一次旅行、一次看病、一批要办的手续——
配合一棵全局 **tag** 树做正交检索。既装照片视频，也装护照、合同和体检报告。

Go 单二进制 + SQLite，Apache-2.0。

> **v0.1.0 已发布。** 可下载[四平台安装包](https://github.com/jinzhao1994/kist/releases/tag/v0.1.0)，或使用 Docker 部署。
> M0–M7 已完成，见[里程碑](#里程碑)。当前没有内置登录，历史原文件永久保留。

## 想法

大多数工具让你把一份文件归到某个位置，之后也只能从那个位置找回来。kist 把这两件事分开：

| | 回答的问题 |
|---|---|
| **box** | 这份文件是怎么来的？ |
| **tag** | 这份文件是什么？ |
| metadata | 它现在是什么状态？*（v0.1 之后）* |

box 是东西落地的地方——扁平、不可嵌套，一个 box 对应一次活动，一个文件属于且只属于一个 box。
tag 树与所有 box 正交，于是"我历年所有的保险单据"和"京都那一周"都是一次查询，
而且两者都不需要在归档的那一刻就预先想到。

## 为什么不用现成的

- **业务操作支持严格撤销。** 权威数据是只追加的操作日志，不是那些表。撤销 = 追加一条逆操作，
  后续改动无冲突时，撤销本身也能被撤销；请求回执不支持撤销。整个库还能从日志重建。
- **档案和照片在同一个归档里。** 照片管理器假定万物都有 EXIF 时间；
  文档管理器则完全不知道拿 4000 张照片怎么办。
- **数据不会被锁住。** 一个库就是两个普通目录：内容按内容寻址存在 `blobs/` 下，
  操作日志是 `ops/` 里每天一个的 JSON 行文件。`boxes/` 是一棵人可读的投影，
  用任何文件管理器都能浏览；`kist.db` 是可以扔掉重建的缓存。
  停止所有写入后，备份就是复制普通目录；把 kist 删掉，文件和它们的历史都还在，
  而且都是 `less` 和 `grep` 能打开的格式。

## 设计文档

用中文写的，值得在读代码之前先读：

- [`docs/design/2026-08-22-kist-v0.1-design.md`](docs/design/2026-08-22-kist-v0.1-design.md)
  ——权威设计。它刻意在每个决策旁边记下被否掉的替代方案。
- [`docs/design/open-questions.md`](docs/design/open-questions.md)——还没定的东西。

## 安装与运维

发布包包含可执行文件、许可证和使用文档。
平台选择、校验和、备份恢复及升级步骤见[运维说明](docs/operations.md)。
稳定版本见 [GitHub Releases](https://github.com/jinzhao1994/kist/releases/tag/v0.1.0)；无标签工作流产物属于开发构建。Docker 使用下方配置。

## 从源码构建

从源码构建需要 Go 1.26.1 和 Node.js 24 LTS 或 26+。检查和发布打包还需要 Python 3。生成的单二进制包含网页资源，运行时不需要 Node.js 或 C 工具链。

```sh
npm --prefix web ci
make check   # 前端检查以及 tidy、gofmt、vet、race 测试和构建
make build   # -> bin/kist
make dist    # -> dist/，四平台压缩包与 SHA256SUMS（需要 Python 3）
```

## Docker（内置 FFmpeg）

群晖用户请看[网页部署指南](docs/synology.zh-CN.md)：专用用户、共享文件夹直接挂载，全程无需 SSH。

预构建镜像支持 Linux amd64 / arm64，Docker 会自动选择本机架构。使用 `compose.image.yaml`，详见 [NAS 部署与升级说明](docs/docker.md)。Docker Hub 和 GHCR 镜像均公开，无需账号、PAT 或 registry 登录：

```sh
docker compose -f compose.image.yaml pull
docker compose -f compose.image.yaml run --rm kist init /library --yes
docker compose -f compose.image.yaml up -d
```

只对新库执行初始化。默认镜像为 `ghcr.io/jinzhao1994/kist:v0.1.0`，在 `.env` 设置 `KIST_IMAGE=jinzhao1994/kist:latest` 可改用 Docker Hub 最新稳定版，固定版本可用 `jinzhao1994/kist:v0.1.2`。群晖优先使用 Docker Hub，以便使用映像更新提示。NAS 局域网访问时，在 `.env` 中设置 `KIST_BIND=NAS的局域网IP` 和 `KIST_ORIGIN=http://NAS的局域网IP:8473`，初始化前可设置 `TZ=Asia/Shanghai`。

如需从源码构建，继续使用原有 Compose 文件：

```sh
docker compose build
docker compose run --rm kist init /library --yes
docker compose up -d
```

打开 `http://127.0.0.1:8473`。镜像以 UID/GID `10001:10001` 运行，内置 FFmpeg、ffprobe、libheif 工具和时区数据。整个库保存在命名卷中，重建容器不会丢失；`docker compose down` 保留数据，`docker compose down -v` 会删除数据。已有库不要再次初始化。CLI 使用同一个库，例如：

```sh
docker compose exec kist kist --library /library verify --deep
```

初始化前设置 `TZ=Asia/Shanghai` 可指定库时区。改用本机其他端口时，同时设置 `KIST_PORT=18473` 和 `KIST_ORIGIN=http://127.0.0.1:18473`。默认只发布宿主机 loopback 端口；远程访问须显式设置 `KIST_BIND` 为宿主机网卡地址，并把 `KIST_ORIGIN` 设置为浏览器使用的可信来源。当前没有登录，只能部署在可信网络或带认证的反向代理后。

使用现有库时，将命名卷改为宿主机绝对路径，并确保 UID/GID `10001:10001` 有权限访问，或用 `user:` 配置库的现有所有者。必须挂载整个库，不能只挂 `kist.db` 或 `blobs/`。启动不会自动创建库，也不会递归修改文件所有权。

镜像内置缩略图、HEIC 预览和视频转码所需工具。CI 在 Linux amd64 与 arm64 原生 runner 分别构建并验收真实媒体处理及容器重建后的持久化。版本化 GHCR 镜像在发布前、从 registry 拉取后分别通过同样的验收。可运行 `sh scripts/docker-smoke.sh kist:local`（需要 Docker、curl、Python 3）检查真实派生任务、HTTP 范围读取、访问保护与容器重建后的数据保留。

## 怎么用

```sh
kist init                                  # 交互式建库
kist box new "云南" --start 2019-08-12      # 一个 box 就是一次出行、一次就诊、一批档案
kist import --box 云南 ~/Pictures/2019/     # 把文件放进去；Ctrl-C 是安全的，重跑会跳过已导入的
kist ls 云南                                # 看看里面有什么
kist extract                               # 读 EXIF，让 --sort captured 有真实结果
kist file show <id>                        # 一个文件的全部已知信息
kist find --name jpg --tag 人物             # 跨 box 检索
kist find --tag 人物 --not-tag 隐私 --sort name --asc
kist file tag 人物 <file-id>                # 给文件打 tag
kist relink                                # 重建 boxes/，那个人可读的镜像
```

业务变更可以通过追加补偿操作撤销，前提是没有后续修改与它冲突；请求回执本身不可撤销：

```sh
kist log            # 这个库都发生过什么
kist undo <op-id>   # 把其中一条收回来
```

### Web 界面

```sh
kist serve --library ~/kist
```

打开 `http://127.0.0.1:8473`。先创建 box，再选择或拖入文件；可以浏览文件、查看 metadata、下载原文件和管理 tag。侧栏提供名称、日期、类型及 tag AND/OR/NOT 组合筛选。网格显示已加载数量，点击「加载更多」继续分页。

从侧栏或 box 页面进入「操作历史」，查看操作详情并撤销。文件详情中的「文件版本」打开该文件的操作记录；「比较版本」对照两个操作编号结束后的状态，包括 metadata、标签路径以及文件当时是否存在。「查看此时的归档箱」展示该操作结束时的 box，下载也绑定该历史状态。这些视图只读，不启动提取或预览任务。

在归档盒详情页点击「移入回收站」，确认盒名和全部文件数量，即可将盒子与内容整体移入回收站。目录、标签、元数据及其屏蔽记录会一并保留，恢复时整体还原。若确认期间内容发生变化，需要重新确认。使用这个功能后，旧版 kist 无法理解新增的日志操作，请勿降级运行该资料库。

「回收站」列出尚未恢复的删除操作。恢复以整条操作为单位，一次批量删除中的文件一起恢复。撤销与恢复都会追加新操作，不会抹掉历史；后续修改产生冲突时整条拒绝，可以打开提示中的阻挡操作再决定如何处理。尤其是撤销导入前，需要先撤销后续的 metadata 提取或屏蔽。v0.1 永久保留历史原文件，不提供清空回收站或保留期设置。历史查询需要重放日志；历史较大时可能超过 30 秒请求限制，页面会报错并允许重试。

「显示设置」分别控制中文/英文、地区数字与日期格式、显示时区和阅读方向，偏好只保存在浏览器。界面采用 Mantine，主色可选灰、绿、黄、粉、蓝、橙；已有六套皮肤偏好自动迁移到相近主色。明暗模式独立选择「浅色」「深色」或「跟随系统」；跟随系统时自动响应系统配色变化。网格按需加载派生缩略图，详情后台生成大图或 H.264/AAC 视频；不支持预览的格式仍提供原文件下载入口。

上传默认每批最多 1000 个文件、总 multipart 正文 2 GiB，可用 `--max-upload-files` 和 `--max-upload-bytes` 调整。完整收到一批后才提交。响应丢失时使用「重试」或「核对结果」；刷新同一标签页后仍保留请求 key，可核对回执。明确开始新操作后重新选择文件，会创建新请求。metadata 提取独立执行，提取失败不意味着原文件丢失。

当前没有登录。远程访问需要显式监听地址和可信来源，例如可信局域网中：

```sh
kist serve --library ~/kist --listen 0.0.0.0:8473 --allow-origin http://192.168.1.20:8473
```

地址需替换成服务器的实际地址。使用反向代理时配置代理的完整外部 origin，并在代理上保护访问；Host/Origin 检查不等于用户认证。版本化 API 契约位于 `/api/v1/openapi.json`；写请求必须携带 `Idempotency-Key`，`/api/v1/requests/{key}` 返回持久操作回执。CLI 与 Web 可以使用同一个库，点击「刷新」即可读取 CLI 的改动。修复、重建等运维操作继续使用 CLI。

### 脚本

每个命令都接受 `--json` 并打印一个对象，所以 kist 能和别的工具串起来：

```sh
kist box new 四川 --json | jq -r .box.id
kist find --tag 人物 --json | jq -r '.files[].id' | xargs kist file tag 已扫描
kist verify --json | jq -e '.findings | length == 0'   # 有问题就退出 1
```

`find` 用游标分页：`--json` 给你 `.cursor`，`--cursor` 把它传回去。错误写入 stderr 并返回非零退出码；
`--json` 模式输出带稳定错误码的结构化错误，普通模式输出文本。

### 确认没出事

```sh
kist verify          # 有没有东西丢了、对不上、放错了？（秒级）
kist verify --deep   # 再加一步：重放整份日志，和当前的表逐行比对
kist scrub           # 读完每个字节重新哈希，抓静默的位翻转
kist repair          # 从远端副本把坏掉的内容取回来
kist rebuild         # 扔掉派生的表，重放日志重建
```

`verify` 只 stat，`scrub` 读内容。这就是两者的全部区别，也是为什么一个便宜到能挂 cron、另一个不能。

`repair` 需要一个可以修回来的地方。远端就是同一份内容的另一个副本——移动硬盘、网络共享、另一台机器——手写进 `kist.toml`：

```toml
[[remote]]
name = "backup"
path = "/Volumes/Archive/kist/blobs"
```

做一个远端最常见的办法，就是你本来就该跑的那条备份命令：

```sh
rsync -a ~/kist/blobs/ /Volumes/Archive/kist/blobs/
```

完整备份需要先停止所有写入（包括导入和修复），并在整个复制期间保持停止，
把 `kist.toml`、`ops/` 和 `blobs/` 复制到新的备份目录。
运行中复制即使先拷 blobs 也不保证一致。新备份复制成功前保留旧备份。
在独立的恢复副本上依次运行 `kist rebuild`、`kist relink`、
`kist verify --deep` 和 `kist scrub`，通过后再使用。
上面的 blob 副本只是修复源，不包含库的操作历史。


kist 从不往远端写，也从不信任远端：取回来的内容边读边哈希，对不上就丢掉。用 `kist remote list` 让路径打错的那天是今天，而不是你真需要它的那天。

## 派生媒体

打开网格或文件详情才提交后台任务，导入不等待解码。普通 JPEG/PNG/GIF/WebP 等图片由 Go 生成预览；HEIC 需要 `heif-info` 和 `heif-dec`（或 `heif-convert`）及 HEVC 解码器；视频需要带 H.264/AAC 编码器的 `ffmpeg` 与 `ffprobe`。Docker 已内置这些工具。Ubuntu 原生部署安装 `ffmpeg libheif-examples libheif-plugin-libde265`；不装也能归档、检索、下载原文件。

缺依赖任务每 30 秒重新检查，安装工具后无需重启服务。详情提供取消与重试，任务重启后恢复，相同内容共享派生结果。需要清空缓存时，先停服务再删除 `derived/`，下次浏览会按需重建，不影响原文件与撤销历史。

首版 recipe：`thumb_512_v1`（JPEG，最长边 512）、`preview_2048_v1`（JPEG，最长边 2048）、`preview_1080p_v1`（H.264/AAC MP4，最高 1920×1080）。图片保持比例与方向，透明底铺白，动图取首帧；多顶层图片的 HEIF 集合暂不支持。限制包括 6400 万像素、256 MiB 压缩静态图、6 小时视频输入和 2 GiB 输出，超限仍可下载原文件。CLI 导入后需运行 `kist extract` 或点击「读取元数据」以分类媒体类型。

派生任务通过 `GET/POST/DELETE /api/v1/files/{id}/derived/{recipe}` 查询、请求和取消，
`GET/HEAD .../content` 读取生成结果。POST 的 `{ "retry": true }` 可重试终止任务。
这些可丢弃任务不使用 `Idempotency-Key`、不生成归档操作；普通归档写入仍须携带请求键。


## 里程碑

| | | |
|---|---|---|
| M0 | 仓库地基 | 已完成 |
| M1 | 内核：op log、blob 存储、命令行导入与检索 | 已完成 |
| M2a | 完整性：`verify`、`scrub`、`repair`、`rebuild` | 已完成 |
| M2b | Query 契约与 conformance suite | 已完成 |
| M3 | metadata 提取，含 EXIF | 已完成 |
| M4 | HTTP API 与内嵌 Web UI | 已完成 |
| M5 | 派生媒体：缩略图、预览、转码；内置 FFmpeg/libheif 的 Docker | 已完成 |
| M6 | 操作历史、box 回溯、文件版本对比、严格撤销与回收站 | 已完成 |
| M7 | 文档、打包、v0.1 | 已完成 |

## 许可证

Apache-2.0，见 [LICENSE](LICENSE)。

页面视觉回归在 GitHub Actions 的固定 Linux 环境中分别生成已认可版本和当前版本的截图。Git 仅保存已认可提交的 SHA，截图与差异报告保留在 Actions artifacts。审阅后可通过工作流自动校验并更新基准提交，操作见[视觉回归说明](docs/testing/visual.md)。

## 安全与第三方许可

安全问题请按照 [SECURITY.md](SECURITY.md) 私下报告。第三方许可全文记录在
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)，随二进制发布包和容器镜像分发。
依赖更新后执行 `npm --prefix web ci` 和 `python3 scripts/third-party-notices.py` 重新生成；
`make check` 会检查通知是否与实际依赖一致。
