# 群晖 NAS 部署指南

通过 DSM 的 Container Manager、控制面板和 File Station 完成部署，无需启用 SSH。
适用于支持 Container Manager 的群晖；公开镜像
`jinzhao1994/kist:latest`（Docker Hub，最新稳定版）支持 amd64/arm64，内置 FFmpeg、ffprobe、libheif，
无需 Docker Hub 或 GitHub 账号，也无需 registry 登录。GHCR 保留为备用来源。

2026-10-11 已在 x86 群晖验证：匿名拉取、专用用户读写、初始化、深度校验、网页建盒、
容器重启后数据保留。本指南没有把媒体上传和所有 DSM 版本列为本次实机验收结果。

kist 是一个服务容器，内置 SQLite。「项目」只是 Container Manager 保存 Compose
配置的方式，不意味着需要多个服务。当前没有登录功能，只用于可信局域网或带认证的
反向代理，不要直接把端口转发到公网。

## 1. 创建共享文件夹和专用用户

1. 在套件中心安装 Container Manager。
2. 在「控制面板 → 共享文件夹」创建 `kist`。选择 NAS 本地存储空间，不使用 SMB/NFS 远程挂载目录。
3. 在「控制面板 → 用户与群组 → 新增」创建专用用户 `kist`，设置强密码。
4. 用户组只保留普通 `users`，不加入 `administrators`。
5. 共享文件夹权限只给 `kist` **可读写**；检查其他目录的最终权限为禁止访问，包括 `homes`。
6. 应用程序权限设为拒绝，包括 DSM、File Station、SMB、FTP 等。容器使用数字身份访问本地挂载，不需要这些应用登录入口。
7. 保存用户后重新打开应用权限确认结果；表头的文字和复选框可能是不同的点击目标。

只修改新建专用用户的权限，不改变现有管理员权限，也不向 Everyone 授权。
容器挂载的读写选项只控制挂载是否只读，不能代替 DSM 文件系统权限。

资料库直接放在共享文件夹根目录：

```text
/volume1/kist/
├── boxes/           ← 在 File Station 或网络共享中浏览归档文件
├── blobs/           ← 权威原文件
├── ops/             ← 权威操作历史
├── derived/         ← 可再生预览
├── kist.db          ← 可重建数据库
├── kist.toml
└── compose.yaml     ← Container Manager 项目配置（以实际文件名为准）
```

这里没有 `kist/library/boxes` 这一层。`boxes` 用于浏览，不要直接改动其中的文件；
上传、重命名和删除通过 kist 操作。备份也不能只复制 `boxes`。

## 2. 查询用户 UID/GID

DSM 用户名不等于容器用户名。不能在 Compose 中直接写 `user: kist`：镜像内同名用户
默认是 `10001:10001`，与 DSM 新建用户通常不同。

通过网页查询数字身份：

1. 「控制面板 → 任务计划 → 新增 → 计划的任务 → 用户定义的脚本」。
2. 名称填 `kist-identity-check`，用户选新建的 `kist`，**取消勾选“已启动”**，避免定时执行。
3. 在「任务设置」填写以下脚本，不启用邮件通知：

   ```sh
   set -C
   id > /volume1/kist/kist-identity-check.txt
   ```

4. 保存后选中任务，点击「运行」。在 File Station 用文本编辑器打开生成的文本文件。

示例输出为 `uid=1028(kist) gid=100(users) groups=100(users)`，则配置中使用
`user: "1028:100"`。**以自己的实际输出为准，不照抄示例数字。** 成功生成文件也验证了
专用用户能写入共享文件夹。`set -C` 防止再次运行时覆盖已有同名文件；不要反复运行。
任务保持禁用即可，诊断文件不是资料库数据。

## 3. 首次初始化

**仅用于新安装，已有资料库跳到第 5 节，不要重新初始化。** 以下假设路径是 `/volume1/kist`，用户身份
是 `1028:100`；请将配置中的路径和身份全部替换为自己查到的值。

在「Container Manager → 项目 → 新增」中，名称填 `kist`，路径选 `kist` 共享文件夹，
来源选「创建 docker-compose.yml」，粘贴以下一次性配置：

```yaml
services:
  kist:
    image: jinzhao1994/kist:latest
    user: "1028:100"
    restart: "no"
    network_mode: none
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    environment:
      TZ: Asia/Shanghai
    volumes:
      - /volume1/kist:/library
    entrypoint: ["/bin/sh", "-ec"]
    command:
      - |
        for name in blobs boxes ops derived kist.db kist.toml DO-NOT-EDIT.txt; do
          test ! -e "/library/$$name" && test ! -L "/library/$$name" || exit 1
        done
        mkdir /library/.kist-init
        kist init /library/.kist-init --yes
        mv -n /library/.kist-init/* /library/
        kist --library /library verify --deep
```

`kist init` 要求空目录，而共享文件夹通常已有 `#recycle` 和项目配置。因此先在全新的
临时子目录建库，再移动新库内容到共享根目录，最后校验根目录的完整性。
这不会把 `boxes` 永久留在子目录，也不修改所有者、不使用 root。
脚本在发现已有资料库组件或临时目录时停止，`mv -n` 不覆盖目标文件。`$$` 是 Compose
传递美元符号的写法，不要改成单个 `$`。

不启用 Web Station 门户，勾选创建后启动。等待镜像下载、构建完成，然后打开
**容器详情 → 日志**，应看到 `Library created` 和根目录校验的 `no problems found`。
一次性容器执行后停止是正常现象；DSM 可能显示停止通知。**项目构建 Exit Code 0
只证明容器启动过，必须检查容器日志中的建库和校验结果。**

若 YAML 编辑器保存后没有生效，用 File Station 打开实际生成的配置文件（可能叫
`compose.yaml`），在 DSM 文本编辑器粘贴完整内容并保存，再执行项目「操作 → 构建」。
文件大小为 0 或报 `Top-level object must be a mapping` 时先检查实际文件。

初始化失败时先看日志和目录内容，不删除整个目录、不覆盖已有组件、不自动重试。
如移动中断，保留临时目录和根目录内容，停止写入并先查清状态。成功后临时目录通常
为空，可确认内容后清理；`#recycle` 保持原样。

## 4. 启动日常服务

初始化及校验成功后，用 File Station 文本编辑器将项目 YAML **完整替换**为下方内容。
除路径和 UID/GID 外，将两处 `192.168.1.10` 替换为 NAS 实际局域网 IP：

```yaml
services:
  kist:
    image: jinzhao1994/kist:latest
    init: true
    user: "1028:100"
    restart: unless-stopped
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    environment:
      TZ: Asia/Shanghai
    ports:
      - "192.168.1.10:8473:8473"
    volumes:
      - /volume1/kist:/library
    command: ["serve", "--library", "/library", "--listen", "0.0.0.0:8473", "--allow-origin", "http://192.168.1.10:8473"]
```

保存后执行项目「操作 → 构建」，完成后若项目未运行，再点击「启动」。打开 `http://NAS的局域网IP:8473`。
以后始终保留运行配置，**不要再换回初始化配置**。
建议固定 NAS 的 IP；若 DSM 防火墙已启用，配置可信客户端对相应 TCP 端口的访问。
更改外部端口时，同时调整 `ports` 中间的数字和 `--allow-origin` 的端口。

验收步骤：

1. 网页能显示归档盒列表，新建测试归档盒成功。
2. 重启 kist 项目，刷新网页后归档盒仍在。
3. 通过 File Station 确认 `boxes`、`ops`、`blobs` 位于共享根目录。
4. 上传小图片、视频及自己的测试文件，检查预览和原文件下载。
5. 需要完整性检查时，可使用 Container Manager 容器终端运行
   `kist --library /library verify --deep`，不需要开放 NAS SSH。

## 5. 备份、升级和迁移

备份期间停止 kist 项目和其他写入者，复制整个 `kist` 共享文件夹到独立备份位置，
或使用已配置好的 NAS 备份工具。`ops/`、`blobs/` 是权威数据，`kist.toml` 保存配置；
不要只备份数据库。完整恢复步骤见[运维说明](operations.md#back-up-and-restore)。

**普通兼容升级不要求每次全量备份。** 定期备份独立安排；只有发行说明要求破坏性迁移，或需要保留旧版本回退点时，才在升级前创建经过验证的备份。备份仍须遵守上面的停写规则。

升级时阅读目标版本说明，停止旧服务及其他写入者，修改运行配置的镜像版本，构建并启动，再检查版本、启动日志和文件访问。不要每次升级都执行全库 `scrub`；将其安排为周期性完整性检查。
版本标签不覆盖。也可选择 `latest`（通过验收的最新正式版）；本指南使用 `latest` 跟进稳定版；需要固定版本时可改为 `jinzhao1994/kist:v0.1.2`。切换标签后仍须手动拉取镜像并重建容器，运行中的容器不会自行升级。不要通过删共享文件夹或重新建库升级。
降级不保证兼容；需要回退时，用旧镜像配合升级前备份在独立目录恢复。v0.1.2 可以直接打开 v0.1.0/v0.1.1 库，无需迁移或额外全量备份；但使用整盒删除/恢复后会产生旧版不认识的新日志 action，不能只替换旧镜像降级。

迁移已有资料库时，停止旧服务和所有写入者，保留原库备份，将**整个资料库的内容**
复制到共享根目录，检查专用用户的有效权限后直接使用运行配置，跳过初始化。
已有文件的权限不一定自动符合新用户；不要对不明目录递归修改权限。

### 从 GHCR 切换到 Docker Hub（已有资料库）

这只是更换镜像来源，不是重新部署资料库，也不需要重新初始化。

1. 进入「Container Manager → 项目」，选中现有 `kist`，停止项目及其他库写入者。
2. 打开项目详情的「YAML 配置」，仅将服务的镜像改为：

   ```yaml
   image: jinzhao1994/kist:latest
   ```

3. 保留现有项目名称、`user`、`volumes`、`ports`、`command`、时区等全部配置。尤其保留
   `/volume1/kist:/library`，不要变成新建的命名卷，也不要套上 `library` 子目录。
4. 保存并执行「操作 → 构建」，等待拉取完成。构建成功但未运行时，再点击「启动」。
   若 YAML 编辑器未生效，通过 File Station 修改项目实际配置文件后重新构建。
5. 检查容器日志、网页底部版本、已有归档盒与文件、缩略图和原文件下载。
   在容器详情确认使用 `jinzhao1994/kist:latest`，不能只凭标签名称 `latest` 判断来源。

正式镜像见 [Docker Hub](https://hub.docker.com/r/jinzhao1994/kist/tags)。
`latest` 与相应版本标签在发布验收成功后指向同一份双架构 index，和 GHCR 保持相同 digest。
切换来源不会自行迁移或复制资料库。此次从 v0.1.0/v0.1.1 升到 v0.1.2 不要求额外全量备份，
回退限制见上文。

### 后续更新

Docker Hub `latest` 若在「映像」页显示更新，选中 kist 后使用「操作 → 更新」或标签旁的更新入口。
完成后检查实际运行版本；若仍为旧版，构建并启动现有项目以使用新镜像，单纯重启旧容器不会切换镜像。
更新提示不一定立即出现。没有入口时，可以把项目 YAML 改为已发布的新版本标签，再构建、启动；
固定版本不会自动跟进后续正式版。

**如何判断是发布延迟还是 NAS 尚未检测到：**

1. 核对完整镜像名是 `jinzhao1994/kist:latest`，而不只是看到 `latest` 字样。
   固定的 `v0.1.x` 标签不会被下一版本覆盖，`edge` 已停止发布。
2. 确认目标版本的 Docker Hub 同步及 `latest` 验收完成。GitHub Release 创建成功不等于
   两个注册表与通道任务已经全部完成；具体阶段见[发布流程](releases/README.md#completion-evidence)。
3. 若镜像已发布而 NAS 没有按钮，可稍后再看，或按上面的明确版本标签方式更新。
   群晖[官方发布说明](https://www.synology.com/en-eu/releaseNote/ContainerManager)
   明确支持 `latest` 自动更新检测，但没有在此说明固定检测时限。
   网上“约 12 小时”的说法不能当作保证；[群晖社区](https://community.synology.com/enu/forum/1/post/187926)
   也有等待 24 小时仍未出现按钮的报告（2026-10-17 查阅）。持续没有提示时检查 NAS 的
   Container Manager 版本、访问 Docker Hub 的网络及镜像代理缓存，而不是无限延长等待。
4. 拉取后重建使用新镜像的容器，最后检查 kist 的实际运行版本。标签相同、镜像下载成功
   或单纯重启旧容器，都不能单独证明已经升级。

群晖对非 Docker Hub 仓库可能不显示更新入口，不能据此判断 GHCR 没有新版本。
不要删除项目或共享文件夹来强制更新，旧 `edge` 标签已停止更新，请使用正式版或 `latest`。

### 缩略图功能升级后的处理

以下适用于包含新版视频封面功能的版本；代码合入 main 不代表正式镜像已经发布。先按上节确认目标版本已发布、NAS 的容器实际运行该版本，再刷新浏览器以加载新前端。

- 视频使用新版 `video_thumb_512_v2` 缓存。浏览到视频时，即使旧版缩略图存在，也会按新配方生成；新结果就绪后继续复用。不会在升级时同时扫描整库。
- 图片和 PDF 仍使用原配方。它们原来的缩略图保留完整比例，过去的裁切来自网页显示方式；新网页可直接完整显示旧缓存，不需要重新生成。PDF 只预览第一页。
- 较早导入的 MOV 如果仍显示普通文件图标，使用网页的「读取元数据」更新类型，再请求预览；无需重新上传。
- 保留 `derived/` 即可，不必清空缓存、重新建库或运行 `rebuild`。旧视频缓存文件不会自动删除，但不会被新版视频请求使用。
- 新封面优先使用内嵌封面或有效的 QuickTime 封面时间，否则分散采样最多 100 张候选。采样有时间预算，NAS 较慢时可能少于 100 张；首次显示可以短暂处于等待或处理中状态。

## 常见问题

| 现象 | 检查 |
|---|---|
| 从 URL 添加映像提示仓库错误 | 本次实测通过「项目」成功匿名拉取 GHCR；不要因此要求用户填写 PAT。 |
| GHCR 没有更新入口 | 按第 5 节切换到 Docker Hub，或使用明确的新版本标签；无需新增登录 token。 |
| `permission denied` | 检查 DSM 用户数字身份、共享 ACL、容器 `user` 和挂载路径；`rw` 不等于获得文件系统授权。 |
| 初始化目录已存在 | 检查是否已建库或上次只完成部分步骤，不能删除后盲目重试。 |
| 网页无法连接 | 检查运行状态、启动日志、NAS IP、端口占用和防火墙。 |
| 网页返回 403 | 浏览器协议、主机、端口应与 `--allow-origin` 一致，保留 Host/Origin 保护。 |
| 重建后出现空库 | 核对挂载的真实宿主路径，停止继续上传，避免向错误目录写入。 |

官方参考：[容器设置](https://kb.synology.com/index.php/th-th/DSM/help/ContainerManager/docker_container?version=7)、
[挂载目录权限](https://kb.synology.com/tr-tr/DSM/tutorial/Docker_container_cant_access_the_folder_or_file)。


### GPX 地图服务与缓存

GPX 预览默认由浏览器访问 OpenFreeMap（道路、地名）和 Mapzen AWS 高程瓦片（地形阴影），无须注册或 key。NAS 只解析 GPX 并缓存 `gpx_map_v1.geojson`，不会向地图商发送原文件或完整坐标；地图商仍能从瓦片请求推知地区。浏览器最多同时绘制两个地图实例，缩略图完成后释放实例，保留最多 32 张本页快照。刷新会重绘地图，浏览器仍可复用 HTTP 瓦片缓存；网络不可用则显示轨迹轮廓和提示。

要换服务，在容器启动命令的 `serve` 后配置 `--map-style`；若字体、精灵或瓦片来自额外域名，重复添加 `--map-resource-origin`。高程服务通过 `--map-terrain-tiles` 和 `--map-terrain-attribution` 单独替换，空模板关闭高程层。完整参数与格式限制见 README。容器重新创建／重启后刷新网页即可，既有 GPX 派生数据可继续使用，快照按新配置生成，无需清空 NAS 的旧图片、视频或 PDF 缓存。
