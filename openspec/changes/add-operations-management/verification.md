# 验证记录

- 后端：`go test ./internal/service ./internal/handler/admin ./internal/server/routes -count=1` 三个包全部通过；最后的传输配置与代理覆盖改动另经 Operations / UpstreamBillingProbe / Usage Stats 定向测试通过。
- 前端：全量 lint、类型检查、生产构建通过；冻结锁文件安装通过，锁文件仅增加 ECharts 及其所需依赖。
- 前端完整测试：387 个文件通过、1 个文件失败；3085 项测试通过、1 项失败。
- 既有失败：`src/views/user/__tests__/stripeLazyLoading.spec.ts` 的断言查找 `return 'vendor-misc'`，而未修改的 `vite.config.ts` 使用 `return 'vendor'`。本次没有改动 Stripe、Vite 配置或该测试。
- 变更相关测试：OperationsView、OperationsDistributionChart、UsageView、localeKeyCompleteness 共 25 项全部通过。
- 真实浏览器（使用模拟接口数据）：桌面 1440px、移动 390px、明暗主题均正常显示两个 ECharts 图表；页面无横向溢出，移动日期面板展开后也不溢出。日期切为 2026-11-02 后，三个统计接口均携带同一日期；模型来源切为 upstream 后请求参数正确。余额展示成功和失败状态，零值由单元测试覆盖；使用模拟数据未查询真实上游账号。
- 截图保存在 `output/playwright/operations-desktop.png`、`operations-dark.png` 和 `operations-mobile.png`。
- 根据用户澄清，已完整恢复使用记录页及其原有图表测试；两者与变更前 Git 基线一致。运营管理页保留独立 ECharts 副本。恢复后两页面和 ECharts 组件的 22 项测试全部通过，相关页面 lint 通过。

本变更没有数据库迁移。

## 运行环境诊断（2026-11-03）

- 前端开发服务通过 SSH 隧道 `127.0.0.1:8090` 连接远程后端；本地后端代码修改不会更新该服务。
- 用户反馈的 `400 Invalid account ID` 对应账号详情 `GET /admin/accounts/:id` 的参数解析错误，说明新路径 `operations-balances` 被旧后端当作账号 ID。上游查询尚未执行。
- 已核对公开 Key 查询页与运营余额服务都请求 `/v1/usage`，以 `Authorization: Bearer <api_key>` 认证，无需上游 JWT。
- 新增真实账号路由注册回归测试，确认静态余额路径进入余额汇总处理器，返回空列表时为 200；三个后端包的 `TestOperations` 定向测试通过。
- 前端对该旧后端响应明确提示部署新版后端并重启，新增失败后重试恢复测试；运营页面 6 项测试通过。
- 远程 SSH 非交互认证失败，尚未部署或重启远程后端，也未验证真实上游余额成功。

## 最终界面调整与推送前验证

- 上游余额表移至统计卡片下方，移除查询账号、余额类型和说明文字；上游地址显示为可点击链接。统计卡片移除范围和费用说明。
- 表格默认模糊，通过眼睛按钮显示或隐藏；隐藏时使用 inert 禁用链接交互，并通过 aria-hidden 隐藏表格数据。新增默认隐藏、显示、再次隐藏与地址跳转属性测试。
- 最终版本全量前端 lint、生产构建（含类型与中英文完整性检查）通过；运营页面、ECharts、原使用记录页及中英文完整性共 27 项定向测试通过。
