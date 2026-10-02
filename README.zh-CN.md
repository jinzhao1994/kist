# kist

[English](README.md) · **中文**

自部署的个人归档应用。基本单元是 **box**——一次旅行、一次看病、一批要办的手续——
配合一棵全局 **tag** 树做正交检索。既装照片视频，也装护照、合同和体检报告。

Go 单二进制 + SQLite，Apache-2.0。

> **还不能用。** 设计已定稿并写了下来。现在能建库、建 box、建 tag 树，
> 并且其中任何一步都能撤销——每次改动都是 append-only 日志里的一条记录，
> 整个库可以只从这份日志重建出来。还不能做的是把文件放进去，`kist import`
> 是下一步。见[里程碑](#里程碑)。

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

- **任意操作可回退。** 权威数据是只追加的操作日志，不是那些表。撤销 = 追加一条逆操作，
  所以撤销本身也能被撤销，整个库还能从日志重建。
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

## 从源码构建

从源码构建需要 Go 1.26 和 Node.js 24 或更新版本。生成的单二进制包含网页资源，运行时不需要 Node.js 或 C 工具链。

```sh
npm --prefix web ci
make check   # 前端检查以及 tidy、gofmt、vet、race 测试和构建
make build   # -> bin/kist
make dist    # -> dist/，四个发布平台
```

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

每一次写入都是 append-only 日志里的一条记录，所以什么都能撤销：

```sh
kist log            # 这个库都发生过什么
kist undo <op-id>   # 把其中一条收回来
```

### 脚本

每个命令都接受 `--json` 并打印一个对象，所以 kist 能和别的工具串起来：

```sh
kist box new 四川 --json | jq -r .box.id
kist find --tag 人物 --json | jq -r '.files[].id' | xargs kist file tag 已扫描
kist verify --json | jq -e '.findings | length == 0'   # 有问题就退出 1
```

`find` 用游标分页：`--json` 给你 `.cursor`，`--cursor` 把它传回去。出错仍然
是 stderr 上一行纯文本加一个非零退出码。

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

## 里程碑

| | | |
|---|---|---|
| M0 | 仓库地基 | 已完成 |
| M1 | 内核：op log、blob 存储、命令行导入与检索 | 已完成 |
| M2a | 完整性：`verify`、`scrub`、`repair`、`rebuild` | 已完成 |
| **M2b** | Query 契约与 conformance suite | ← 当前 |
| M3 | 提取：EXIF、缩略图、预览图 | |
| M4 | Web UI | 第一个能给别人看的版本 |
| M5 | 历史 UI：浏览并撤销任意历史操作 | |
| M6 | 文档、打包、v0.1 | |

## 许可证

Apache-2.0，见 [LICENSE](LICENSE)。
