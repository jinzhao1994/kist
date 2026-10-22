# 笔记本离线客户端（开发版）

[English](offline-client.md)

开发版已提供本地网页、按需缓存、离线编辑和手动同步。完整故障验收与视觉验收仍在进行，尚未发布为正式版本。

NAS 是唯一权威资料库，保存全集数据。笔记本同步完整对象目录和历史，但只下载选中的原文件与预览；没有缓存的文件仍能出现在目录和检索结果中，不会被误报为 NAS 原文件丢失。离线检索反映上次完整同步的目录，不包含此后 NAS 上的新修改。

## 连接 NAS

在 NAS 上创建可独立撤销的同步读写凭据：

```sh
kist --library /path/to/nas-library sync-access create
kist --library /path/to/nas-library sync-access list
```

创建时只显示一次密钥，列表仅显示凭据 ID 和创建时间。密钥摘要保存在 NAS 的 `sync-access.json`，不写入归档历史，也不会被归档撤销恢复。

按原有 allowed-host 配置启动 NAS 服务。在笔记本上从私有文件的标准输入读取密钥：

```sh
kist client init /path/to/laptop-client --server https://nas.example \
  --token-stdin < /path/to/private-credential.txt
kist client sync /path/to/laptop-client
kist client status /path/to/laptop-client
```

`init` 验证连接并绑定 NAS 库 UUID，`sync` 才下载目录。客户端目录独立于 NAS 资料库，普通 NAS 命令不能将其当作完整库打开。客户端命令必须显式指定目录。

非回环地址默认要求 HTTPS；确需明文 HTTP 时显式指定 `--allow-http`，客户端不会跟随重定向。不要将凭据放在 URL 或命令行参数里。连接配置保存在本地 `connection.json`，POSIX 系统权限为 0600。

## 出发前准备

启动笔记本网页服务：

```sh
kist client serve /path/to/laptop-client --listen 127.0.0.1:8474
```

打开 `http://127.0.0.1:8474`。网页资源随本地二进制提供，打开网页不依赖 NAS。先完成同步，再在文件菜单中保留需要的文件或组合。批量准备时，点击显示方式旁的离线选择入口，选择文件或组合，再点击保留离线。选择已加载项目只覆盖当前已加载的结果，不包括尚未请求的搜索页。

也可以用命令行创建跨归档盒的命名选择；组合使用其目录 ID，两个成员参数都可重复：

```sh
kist client offline select /path/to/laptop-client trip --file FILE_UUID --fileset FILESET_FOLDER_UUID
kist client offline prepare /path/to/laptop-client trip
kist --json client offline status /path/to/laptop-client trip
kist client offline list /path/to/laptop-client
kist client offline remove /path/to/laptop-client trip
```

`select` 替换该名称下的选择并保留内容，尚不下载；`prepare` 下载缺少的原件并准备网页所需预览；`status` 可离线校验当前已同步视图下的就绪情况。组合会包含配置成员、嵌套组合和显式主文件，包括 Live Photo 视频与字幕轨；同目录内的其他文件不会自动加入。

只有原件和所需预览都可用，才会显示准备完成。本地转换工具不可用且 NAS 不可达时，准备可能不完整；补齐条件后重试。地图底图等外部资源仍可能在离线时不可用。同步或调整组合成员后应重新检查、准备新成员。

网页「离线客户端 → 缓存」列出所有保留选择，可检查可用性、继续准备或取消保留。多个选择共享内容时，取消其中一个不会解除另一个的保护；取消保留不删除归档文件。批量准备先保存所有选择再下载，中断后可继续校验并跳过已就绪内容。

## 旅行期间与回家同步

笔记本可浏览已准备内容、导入照片并编辑对象、标签或文件组合；NAS 同时可以独立修改。新原件和待同步操作单独持久保存，不属于可逐出的缓存。

回家后点击网页「立即同步」，或执行：

```sh
kist --json client sync /path/to/laptop-client
kist --json client status /path/to/laptop-client
```

同步按本地操作顺序校验并提交，不按修改时间覆盖。NAS 接纳后分配正式操作 ID，同时保留原始操作时间和接纳时间。遇到连接中断，重新执行同步即可续传原件、确认已提交操作，重复请求不会再次执行。

「等待确认」表示 NAS 可能已经提交，不能直接放弃或重做；先恢复连接确认结果。明确的冲突会停止后续提交，可保留 NAS 结果，或通过支持的领域编辑器检查原值、当前值和准备提交的值，保存后再次同步。名称、日期、组合配置和封面已有编辑入口；归档盒、目录和组合成员的删除冲突提供专用范围确认：检查原范围与当前范围后，勾选确认才能保存新方案。新方案可能包含 NAS 后来加入的文件或标注，保存后再次同步才会提交；查看后 NAS 再有修改仍会拒绝过期方案。已不存在的目标只能保留 NAS 结果，不会被复活。其他尚未接入的动作仍需独立领域编辑入口，不提供绕过前提的强制覆盖。

封面编辑可选择对照中仍有效的文件或清空封面；已删除文件不可提交。要选其他文件，可解决阻挡后使用文件菜单的设置封面功能。NAS 再次改变已确认的前提时，仍需重新检查。

「本地操作」列表显示对象名称、改名摘要、时间、状态及正式回执；从当前状态解析的名称有明确标记。打开详情可查看关联操作、替代方案和可用的严格撤销入口。原始意图和冲突处理决定保留，不会被重做覆盖。

客户端沿用 NAS 的明确时区（如 `Asia/Shanghai` 或 `UTC`），已有待同步操作不会因重新连接而改写。NAS 缺少可移植时区或协议能力不兼容时，需先升级或修正配置。

## 缓存限额与恢复

以下命令无需连接 NAS：

```sh
kist client cache status /path/to/laptop-client
kist client cache configure /path/to/laptop-client --max-age 720h --max-bytes 10737418240
kist client cache prune /path/to/laptop-client
kist client cache rebuild /path/to/laptop-client
```

保留时间从最近访问算起。超期或超额时，按最近最少使用顺序逐出可清理内容；正在读取、传输或明确保留的内容不会被清理。限额包含原件缓存、派生预览、转换工作目录和未完成传输的空间预留。降低上限后，受保护内容可能使占用仍然超限，需调整选择或上限。

这里的占用不是笔记本整个客户端目录大小：待同步原件和本地历史单独统计且不受缓存清理影响。同步时仍被读取的已确认原件，可在读取结束后通过 `prune` 重试纳入缓存管理，不必再次联网同步。

`rebuild` 根据已知内容事实及缓存字节恢复跟踪记录，区分完整、部分、损坏、缺失及正在使用的对象。损坏内容不会被当成可用，需重新获取；活跃对象可稍后再次检查。此命令不删除离线选择、待同步操作或受保护的新原件，也不是 NAS 资料库的重建命令。

派生预览的映射丢失或格式损坏时，显式重建也可恢复残留字节的计量；没有索引的字节必须通过完整哈希校验才标为完整。缺少映射仍会显示预览未缓存，重新准备取得可信描述后可复用已校验字节。未知版本的记录会被拒绝处理，不会被覆盖。

## 更换地址或凭据

先在 NAS 创建新凭据，再更新连接，最后撤销旧凭据：

```sh
kist client connect /path/to/laptop-client --server https://nas.example \
  --token-stdin < /path/to/replacement-credential.txt
kist --library /path/to/nas-library sync-access revoke OLD_CREDENTIAL_ID
```

重新连接必须验证同一 NAS 库 UUID，不能把待同步操作转交给另一个库。NAS 每次新请求都会重新检查授权，运行中的服务也会看到凭据撤销。
