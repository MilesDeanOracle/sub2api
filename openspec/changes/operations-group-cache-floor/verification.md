# 验证记录

代码提交在分支 `feat/operations-group-cache`，未合并到 main，未部署。本地后端未运行，因此没有连接真实数据库做端到端验证。

## 自动化验证

- 后端：`go build ./...`、`go vet`（service / handler / routes）与 gofmt 通过。
- 后端新增 13 个测试通过，覆盖以下内容。
  - 比对与 token 同步：一位小数与取整误差、暂停、未设置、不低于调整值、窗口内无数据、三个窗口独立调整。
  - 配置解析：缺失、损坏、越界的行按未设置处理。
  - 按分组保存与校验：范围、小数位、未知分组、不影响其它分组。
  - 管理员列表返回真实命中率与累计窗口的 token 明细，以及监测匹配规则。
  - 查询条件：名称关键字（不区分大小写）、平台、启用状态、多条件同时满足、无结果时仍返回列表、非法状态返回 400。
  - 路由：未登录 401、受限管理员 403、参数校验 400、未知分组 404、正常读写、带查询条件的列表请求。
- `go test -tags unit ./internal/server/routes/` 全量通过。
- 前端：改动文件 ESLint 零问题，`vue-tsc --noEmit` 全量通过。运营台与 i18n 相关的 13 个测试文件 87 项通过，其中新页面 11 项。
- 前端全量：413 个测试文件中 410 个通过、3 个失败，失败的是 `admin.groups.usage-summary.spec.ts`、`GroupsView.columnSettings.spec.ts`、`AccountsView.sparkShadow.spec.ts`，均不在本次改动的文件内。
- `go test -tags unit ./internal/service/` 全量有 3 个失败，均不在本次改动的文件内。
  - `TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort` 单独运行也失败。
  - `TestFilterGrokFreeQuotaAccountsOnlyBlocksExplicitFreeOAuth` 与 `TestCommandCodeGatewayPassesThroughCatalogProtocols` 单独运行、以及与新测试一起运行都通过，只在全量运行时失败。
  - 没有另建干净的 main 做基线对比。

## 顺带修复的既有问题

上游提交 `814ecfba7` 给 `NewAdminService` 增加参数后，两处本地测试没有对齐，service 与 routes 两个包的单元测试在 main 上无法编译。按 AGENT.md 的做法各补了一个 nil，未改动其它内容。

- `internal/service/admin_service_create_user_test.go`
- `internal/server/routes/custom_admin_key_group_models_test.go`

## 浏览器检查

用临时模拟接口配合前端开发服务器，在 1440×900 与 390×844 视口下检查 `/operations/group-cache`。模拟接口与开发服务器在检查后已停止并删除。

- 表格八列、菜单项、说明文案与刷新按钮显示正常。启用且真实命中率低于 70% 的行标红，暂停的行不标红。
- 「Token 明细」一列每行分三行显示缓存命中、缓存创建、输入，数值按 K/M/B 紧凑单位显示并对齐，与真实命中率来自同一份数据。
- 查询表单：分组名称、平台、状态下拉与「查询」「重置」按钮底对齐。输入 claude、选 Anthropic 和启用后点「查询」，模拟接口收到这三个条件，表格显示 2 行和「共 2 个分组」。「重置」清空表单并恢复 5 行。在名称框按回车可以查询。无结果时显示查询空状态和「共 0 个分组」，页面不报错。
- 快捷编辑：点击后输入框获得焦点，输入 88 回车后保存并显示 88.0%。
- 状态开关：切换后该行变为启用，真实命中率随即标红，调整值不变。模拟接口收到该分组的两次保存请求。
- 390px 视口下页面没有整体横向溢出，表格在容器内横向滚动。

检查中发现并修复两个问题：

- 编辑框位于行循环内，Vue 把它的模板引用收集成数组，导致点击后输入框没有获得焦点。已修正并补断言，去掉聚焦后该断言失败。
- 窄屏下列标题被挤成竖排，表头改为不换行。

## 未验证

- 真实后端下的 settings 表读写，以及公开快照接口的实际输出。
- 深色模式下的显示效果。
