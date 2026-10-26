# kist

[English](README.md) · **中文**

自部署的个人归档应用。基本单元是 **box**——一次旅行、一次看病、一批要办的手续——
配合一棵全局 **tag** 树做正交检索。既装照片视频，也装护照、合同和体检报告。

Go 单二进制 + SQLite，Apache-2.0。

> 可下载[最新四平台安装包](https://github.com/jinzhao1994/kist/releases/latest)，或使用 Docker 部署。
> M0–M7 已完成，见[里程碑](#里程碑)。当前没有内置登录，历史原文件永久保留。

v0.1.7 起支持本地客户端按需缓存、离线编辑及与权威主库同步，也可由客户端为其他客户端中继。参见[离线与同步指南](docs/offline-client.zh-CN.md)及[实例设置](docs/settings.zh-CN.md)。

**开发版：图片 OCR。** 在同一台电脑运行 kist 和浏览器，进入「设置 → AI 连接」，使用 ChatGPT 登录并保存默认模型（账号提供时可选 GPT-6 Luna）。支持 JPEG、PNG、静态 WebP；文件详情的 OCR 分组可单张排队。自动识别默认关闭，启用后只处理以后新增入库的图片；「补全已有图片的文字识别」显式扫描未完成项，跳过已成功或排队中的图片。后台串行执行，关闭网页仍继续；可暂停，失败或中断后需明确重试并继续队列，不会无限重复调用。所选模式下的图片发送至 OpenAI，使用你的 ChatGPT 套餐；成功结果保存为文件 metadata，可复制或从操作历史撤销。单图限 20 MiB，正文限 256 KiB，超限拒绝而不截断；切换模型不重跑成功结果，暂不支持正文检索。

图片描述独立保存为 metadata（`desc:text`），不修改文件或归档盒的描述。可设置资料库语言（默认 `zh-CN`），开启新增图片自动描述，或手动补全已有图片；共用 AI 模型和代理设置。修改模型或语言不会重新生成已完成结果。支持 JPEG、PNG 和静态 WebP。

AI 设置支持一个 HTTP/HTTPS/SOCKS5 代理地址，留空继承进程环境代理；它用于后端的授权、模型列表、OCR 与图片描述请求，浏览器登录仍使用浏览器自身网络。模型、自动模式与代理保存在本实例 `kist.toml` 的 `[ai_options]`，不进入归档同步；代理含密码时同样应保密。连接凭据也是实例私有配置。首版仅允许本机回环访问，不含 NAS 登录与客户端发起识别。

### 文件组合

在归档盒内打开目录菜单，选择「配置文件组合」，设置 Live Photo、视频与字幕或多页文档，分配成员角色后保存。页序可以调整；主文件默认取照片、视频或第一页（可递归到子组合），也可手动指定。组合名称仍是目录名，独立于主文件名。

默认检索将组合当作主文件；打开「包含组合内文件」可独立找到其他成员。标签和 metadata 仍属于原文件，换主文件就会跟随切换。预览侧栏显示当前成员的详情与标签；成员目录提供「查看组合」入口。

文件系统投影可保留目录，或使用统一消解冲突的共同前缀平铺，不改原始 blob。取消组合保留目录与文件；配置修改和整体删除支持严格撤销。视频字幕预览支持不超过 4 MiB 的 UTF-8 SRT/WebVTT，其他字幕仍可单独访问。首版手工配置现有目录，导入时不自动配对。


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

打开 `http://127.0.0.1:8473`。镜像以 UID/GID `10001:10001` 运行，内置 FFmpeg、ffprobe、libheif 工具、Poppler 和时区数据。整个库保存在命名卷中，重建容器不会丢失；`docker compose down` 保留数据，`docker compose down -v` 会删除数据。已有库不要再次初始化。CLI 使用同一个库，例如：

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

实例的主色、存储、连接和服务配置统一在「设置」中管理，保存到 `kist.toml`。详见[实例设置](docs/settings.zh-CN.md)。

「显示设置」分别控制中文/英文、地区数字与日期格式、显示时区和阅读方向，偏好只保存在浏览器。界面采用 Mantine，主色可选灰、绿、黄、粉、蓝、橙；主色以实例设置为准，旧浏览器明暗偏好继续保留。明暗模式独立选择「浅色」「深色」或「跟随系统」；跟随系统时自动响应系统配色变化。网格按需加载派生缩略图，详情后台生成大图或 H.264/AAC 视频；不支持预览的格式仍提供原文件下载入口。

上传默认每批最多 1000 个文件、32 GiB，可用 `--max-upload-files` 和 `--max-upload-bytes` 调整。完整一批通过校验后才归档。新会话在 Worker 中边读取、边校验、边上传，每块原文件只读取一次，发送当前块时最多预读下一块。每块最多 8 MiB，独立核对 SHA-256，完成时封存全文件摘要；服务端核对分块、导入输入和最终 blob 后才提交。上传摘要保留最近一次尝试的总耗时、有效传输速度与各阶段耗时；阶段有重叠，不能直接相加。

「暂停」保留已确认进度，「继续上传」先查询服务端，再只补传缺失部分。刷新或重新打开页面后，回到原归档盒和目录，重新选择整个原始批次；浏览器重新计算已上传前缀的摘要，与服务端暂存字节核对后继续，剩余部分边传边计算；不能只凭文件名、大小和修改时间续传。旧会话保留原有的整文件预校验。服务端重启后也可恢复。「核对结果」无需原文件即可找回成功回执。「取消」清理暂存，但不会撤销已经成功的归档。metadata 提取独立执行，失败不意味着原文件丢失。

会话放在资料库卷的 `uploads/`，是可丢弃暂存，不属于归档历史。请为暂存与最终 blob 预留空间。最多保留 8 个会话，声明总量最多为单批字节上限的两倍；这不保证磁盘实际剩余空间。空闲 7 天过期，成功分块和显式继续会续租，单纯查看进度不会续租。创建、取消时及每小时定期回收过期／已取消暂存。浏览器恢复依赖网站存储，需使用同一浏览器配置和网站地址；网页被挂起或手机锁屏可能停止上传，不承诺手机后台持续上传。无需一直盯着页面，但连续上传需要保持页面打开和设备唤醒；挂起后可继续上传，无需从头开始。

升级后请刷新旧页面。已校验的普通 multipart 上传接口继续保留，兼容未启用续传的客户端／服务端；该回退路径仍整批提交并保留回执，但传输失败时需重新发送整批。不会静默回退到缺少原文件校验的上传。

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

## 归档盒封面与大小

在文件右键菜单选择「设为归档盒封面」，大／中图标卡片完整显示文件预览。
归档盒菜单的「移除封面」只清空关联，不删除文件。封面按文件 ID 保存，
重命名或更新内容后仍然有效；文件回收时暂时显示默认图标，恢复后封面重新出现。
关联写入操作日志，支持撤销和重建。目录与归档盒显示数据库维护的逻辑总大小，
不包含已回收文件；文件卡片和列表均可添加、展示标签。

v0.1.5 启动时自动升级到 schema 6，首次从已有索引回填目录总大小；不扫描原文件。
这是非破坏性升级，无需重新建库。旧版本不保证理解新 schema 与封面日志字段，
若需回退，请使用升级前恢复点及对应旧版本，不要只替换旧二进制。

## 派生媒体

旧库中被旧版提取器识别为普通文件的 MOV，执行 `kist extract --extractor media` 或点击网页的提取元数据即可更新，无需重新上传。PDF 网格显示首页缩略图，详情使用 PDF.js，可翻页、搜索、缩放和复制已有文本；新标签页使用浏览器原生阅读器。桌面 Chrome 可自动识别扫描件文字，具体能力由浏览器决定，识别结果不会写入归档。

打开网格或文件详情才提交后台任务，导入不等待解码。普通 JPEG/PNG/GIF/WebP 等图片由 Go 生成预览；HEIC 需要 `heif-info` 和 `heif-dec`（或 `heif-convert`）及 HEVC 解码器；PDF 首页缩略图需要 Poppler（`pdftoppm`）；视频（包括 MOV）需要带 H.264/AAC 编码器的 `ffmpeg` 与 `ffprobe`。DOC/DOCX 需要 PATH 中的 LibreOffice Writer（`libreoffice` 或 `soffice`），PPT/PPTX 需要 LibreOffice Impress，两者的缩略图还需 Poppler。Docker 已内置这些工具及 Noto CJK、Carlito、Caladea 字体。Ubuntu 原生部署安装 `poppler-utils ffmpeg libheif-examples libheif-plugin-libde265 libreoffice-writer libreoffice-impress fonts-noto-cjk fonts-crosextra-carlito fonts-crosextra-caladea`；不装也能归档、检索、下载原文件。

DOC/DOCX、PPT/PPTX 列表显示首页／首张幻灯片缩略图，详情读取本地转换并缓存的 PDF，可翻页、搜索、复制文本。转换就绪后可在新标签页打开 PDF；下载始终保留原始 Office 文件。旧文件无需重新导入或提取元数据。复杂排版、字体替代可能与 Microsoft Office 不同。幻灯片仅呈现静态页面，不播放动画、转场、内嵌音视频或交互内容。输入及转换 PDF 各限 128 MiB，转换最长 120 秒，临时目录限 512 MiB；禁用宏和 Writer 自动链接更新，不支持含宏或外部资源关系的 DOCX/PPTX（普通超链接可保留）。

缺依赖任务每 30 秒重新检查，安装工具后无需重启服务。详情提供取消与重试，任务重启后恢复，相同内容共享派生结果。需要清空缓存时，先停服务再删除 `derived/`，下次浏览会按需重建，不影响原文件与撤销历史。

STL（ASCII 和二进制）可生成完整模型缩略图与详情静态图，纯 Go 渲染，无需额外工具。旧库 `.stl` 文件也可直接请求预览；后端验证真实格式，损坏或超限时保留下载入口。输入上限 32 MiB／20 万三角面，复杂模型另受渲染工作量和 90 秒预算限制。采用固定视角和统一材质，不提供旋转操作或打印质量验证；配方为 `stl_thumb_512_v1`、`stl_preview_2048_v1`。

视频缩略图优先使用文件内嵌封面，其次使用有效的 QuickTime 封面时间；没有时，跨视频分散抽取最多 100 张低分辨率候选，排除黑场并按画面颜色分布选代表帧。采样有 60 秒预算，慢设备可能少于 100 张；短片减少采样。此算法不保证识别电影内容或最佳镜头。

GPX（轨迹、路线、航点）可显示带道路、地名和地形阴影的地图缩略图，详情可缩放。默认使用免账户／免 key 的 OpenFreeMap Liberty 与 Mapzen AWS 高程瓦片；浏览器需要访问这两个服务。GPX 在 NAS 本地解析，原文件与完整坐标不上传地图商，但瓦片请求会暴露浏览地区。支持旧库 `.gpx` 文件，无需重新导入；上限 32 MiB／10 万点，格式错误或超限保留下载入口。

NAS 缓存 `gpx_map_v1.geojson`，浏览器按需绘制缩略图（完成后释放地图实例）并在本页保留最多 32 张成功快照；刷新后重绘，底图瓦片沿用浏览器 HTTP 缓存。底图断网时显示轨迹轮廓和提示，详情可重试。此功能不是离线地图包。

可通过启动参数替换地图商或使用自托管服务：

```sh
kist serve --map-style https://maps.example/style.json \
  --map-resource-origin https://tiles.example \
  --map-terrain-tiles 'https://elevation.example/terrarium/{z}/{x}/{y}.png' \
  --map-terrain-attribution 'Elevation provider attribution'
```

`--map-style` 接受 MapLibre 样式 URL（样式中也可以配置栅格底图），`--map-resource-origin` 可重复传入字体／精灵／瓦片的额外 origin；样式与高程 URL 本身的 origin 自动允许。相对本站根路径也受支持。高程模板须为 256 像素 Terrarium 编码，最高源级别 15；`--map-terrain-tiles ''` 可关闭高程层。跨域资源需支持 CORS，HTTPS 部署应使用 HTTPS 资源。替换服务后重启并刷新页面，无需删除 GPX 派生数据；快照键包含服务配置，旧背景不会复用。各源归属信息由地图样式和高程参数提供并保留。

升级后刷新网页，可见视频请求 `video_thumb_512_v2`，即使已有旧版缩略图，也会按新版本生成并缓存。图片和 PDF 的生成算法未变，继续复用 `thumb_512_v1`；完整显示由网页布局控制。无需清空整个 `derived/`、重新上传或 rebuild。旧版缓存文件仍保留，不自动清理。

当前 recipe：`video_thumb_512_v2`（视频封面 JPEG，最长边 512）、`thumb_512_v1`（JPEG，最长边 512）、`preview_2048_v1`（JPEG，最长边 2048）、`preview_1080p_v1`（H.264/AAC MP4，最高 1920×1080）。图片保持比例与方向，透明底铺白，动图取首帧；多顶层图片的 HEIF 集合暂不支持。限制包括 6400 万像素、256 MiB 压缩静态图、6 小时视频输入和 2 GiB 输出，超限仍可下载原文件。CLI 导入后需运行 `kist extract` 或点击「读取元数据」以分类媒体类型。

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

### 收藏搜索

点击搜索栏旁的星标，为当前文件或归档盒搜索命名并收藏，保存范围、条件和排序。
侧栏「收藏搜索」可随时重新执行；行尾菜单可改名、更新为当前搜索条件或取消收藏。
结果随资料库内容实时更新，不保存某次结果快照。

收藏保存在资料库中，电脑和手机共用，纳入操作历史、备份、重建与严格撤销。
引用的标签、归档盒或目录不存在时会提示，原条件保留，不会悄悄扩大搜索范围。
写入新的收藏搜索操作后，旧二进制将无法回放这些操作，不应再用于打开此库继续写入。
