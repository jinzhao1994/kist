# 群晖 NAS 部署指南

通过 DSM 的 Container Manager、控制面板和 File Station 完成部署，无需启用 SSH。
适用于支持 Container Manager 的群晖；公开镜像
`ghcr.io/jinzhao1994/kist:v0.1.0` 支持 amd64/arm64，内置 FFmpeg、ffprobe、libheif，
无需 GitHub 账号或 registry 登录。

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

**仅用于新安装，已有资料库跳到迁移章节。** 以下假设路径是 `/volume1/kist`，用户身份
是 `1028:100`；请将配置中的路径和身份全部替换为自己查到的值。

在「Container Manager → 项目 → 新增」中，名称填 `kist`，路径选 `kist` 共享文件夹，
来源选「创建 docker-compose.yml」，粘贴以下一次性配置：

```yaml
services:
  kist:
    image: ghcr.io/jinzhao1994/kist:v0.1.0
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
    image: ghcr.io/jinzhao1994/kist:v0.1.0
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

保存后执行项目「操作 → 构建」，打开 `http://NAS的局域网IP:8473`。
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

升级前阅读目标版本说明并完成停写备份，修改运行配置的镜像版本，构建并启动，再验收。
版本标签不覆盖。也可选择 `latest`（通过验收的最新正式版）或 `edge`（通过 CI 与镜像验收的 main 开发版）；日常 NAS 部署建议固定版本。切换标签后仍须手动拉取镜像并重建容器，运行中的容器不会自行升级。不要通过删共享文件夹或重新建库升级。
降级不保证兼容；需要回退时，用旧镜像配合升级前备份在独立目录恢复。

迁移已有资料库时，停止旧服务和所有写入者，保留原库备份，将**整个资料库的内容**
复制到共享根目录，检查专用用户的有效权限后直接使用运行配置，跳过初始化。
已有文件的权限不一定自动符合新用户；不要对不明目录递归修改权限。

## 常见问题

| 现象 | 检查 |
|---|---|
| 从 URL 添加映像提示仓库错误 | 本次实测通过「项目」成功匿名拉取 GHCR；不要因此要求用户填写 PAT。 |
| `permission denied` | 检查 DSM 用户数字身份、共享 ACL、容器 `user` 和挂载路径；`rw` 不等于获得文件系统授权。 |
| 初始化目录已存在 | 检查是否已建库或上次只完成部分步骤，不能删除后盲目重试。 |
| 网页无法连接 | 检查运行状态、启动日志、NAS IP、端口占用和防火墙。 |
| 网页返回 403 | 浏览器协议、主机、端口应与 `--allow-origin` 一致，保留 Host/Origin 保护。 |
| 重建后出现空库 | 核对挂载的真实宿主路径，停止继续上传，避免向错误目录写入。 |

官方参考：[容器设置](https://kb.synology.com/index.php/th-th/DSM/help/ContainerManager/docker_container?version=7)、
[挂载目录权限](https://kb.synology.com/tr-tr/DSM/tutorial/Docker_container_cant_access_the_folder_or_file)。
