# 表格缩略图缓存

实际客户端 `instancetypes.csv` 的重复加载来自 `sheet-preview` JSON 接口的 `no-store`，不是已版本化的图片缩略图接口。浏览器探测确认整页刷新仍请求该接口。

- 表格预览请求携带解析版本、blob key 和文件名；只有服务端核对成功且支持预览的响应才采用私有 immutable 缓存。
- 无版本请求保持兼容；版本不符返回 409，不缓存错误。文件名参与身份，避免同一 blob 从 CSV 改为 TSV 后错误复用。
- 前端成功预览在相同内容及选项下保持新鲜；缩略图组件直接加载，交互式重型表格阅读器继续延迟加载。
- API 测试覆盖版本响应、无版本响应及改名失效；组件测试覆盖卸载重挂复用、改名、内容替换；视觉 fixture 校验版本参数和缓存响应。

不更改数据库或本机缓存淘汰规则。仅本地提交，不推送 GitHub。

完成验证：`make check` 通过（345 项前端测试、Go race 与构建），日志 `/tmp/kist-sheet-cache-check.log`。相关浏览器 7 个场景通过，截图与目录刷新阶段一致，无新增样式差异，日志 `/tmp/kist-sheet-cache-browser.log`。

在 19604 的真实 `instancetypes.csv` 上通过 Chrome DevTools 网络事件验证：首次请求 `fromDiskCache=false`，整页刷新后 `fromDiskCache=true`，普通目录返回没有额外表格预览请求。成功响应为 `private, max-age=31536000, immutable`。记录 `/tmp/kist-csv-cache-probe.json`。19603／19604／19605 已更新到本次构建。
