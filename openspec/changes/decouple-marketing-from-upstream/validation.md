# 验证记录（2026-11-27）

## 已完成

- 用户和管理员活动路由集中到 `backend/internal/server/routes/activity_routes.go`；
  `user.go` 和 `admin.go` 各保留一次注册调用。
- 活动历史迁移、维护窗口检查和事务锁超时配置集中到
  `backend/internal/repository/activity_migrations.go`；通用迁移执行器只调用两个
  稳定的活动钩子。
- 新增 `make test-marketing-upstream`，默认执行前端类型检查、43 项奖励组件测试、
  后端编译、活动/消费/抽奖/迁移专项和隔离数据库真实登录 HTTP 全链路。
- 活动业务文件不再直接编写 `users`、`usage_logs`、`payment_orders` 的
  `FROM`/`JOIN`/`UPDATE` SQL；核心表读取集中到
  `backend/internal/repository/activitystore/core.go`。升级门禁会静态检查这一约束。

## 门禁执行约束

- 快速和完整模式都需要已运行的 Docker，自动创建临时 PostgreSQL 16，生成随机
  凭证和本机随机端口，覆盖注入 `ACTIVITY_TEST_DSN`，不连接已有数据库。
- `fast` 运行真实数据库专项，但不启动 Redis 和 HTTP 服务；完整模式在同一个临时
  PostgreSQL 中先执行专项，再启动隔离 Redis 和后端进行真实登录 HTTP 验收。
- 使用 `go test -json -count=1` 检查结果。任何测试或子测试 `skip`/`fail`、命令非零
  退出或关键测试未获得 `pass`，都使门禁失败。缺少 Docker 时明确报错，不降级跳过。
- 关键契约清单在 `scripts/test-activity-rewards-http.py` 的 `REQUIRED_CONTRACTS`；
  修改测试名称时须同步更新。测试筛选同时覆盖签到、抽奖幂等、退款和资金回归。
- 9 项门禁自身回归检查覆盖静默跳过、空测试集、漏选关键测试、测试失败、命令失败、
  隔离 DSN 注入及诊断日志凭证脱敏。
- 无论成功或失败，退出时清理本轮创建的容器；HTTP 后端也在退出时停止。

## 适配契约清单

| 契约 | 验收测试 |
| --- | --- |
| 认证、管理权限与动态授权 | HTTP 中真实登录、未登录 401、普通用户访问管理接口 403、匿名展示授权与撤回、受众及后台开关 |
| 签到资格、额度与重复操作 | `TestCheckin*`、`TestActivityCheckinCapTracksRechargeMultiplier`、`TestActivityProductionPostgres` |
| 领取、余额转入与并发账务 | `TestConsumptionPostgres`、`TestActivityWalletExperiencePostgres`、`TestActivityProductionPostgres` |
| 抽奖幂等、预算与历史奖池 | `TestDrawLottery*`、`TestLottery*`、`TestActivityProductionPostgres` |
| 退款终态、回滚及未知响应 | `TestActivityPaymentProductionPostgres`、`TestActivityRefundUncertainResponses`、`TestRegressionRefund*` |
| 计费提交顺序、赠金抵扣与有效消费 | `TestRegressionOutOfOrderUsageCommitPostgres`、`TestRegressionTransferMustNotAbsorbEarlierUsagePostgres`、`TestRegressionConsumptionPendingAndSignedEventsPostgres` |
| 消费总开关与并发 | `TestConsumptionMasterSwitchPostgres`、`TestConsumptionMasterSwitchConcurrentPostgres` |
| 动态隐私、撤回与活动选择 | `TestRewardMoments*`，以及 HTTP 中的隐私字段、重试和开关检查 |
| 新库、旧版、部分升级与当前库迁移 | `TestActivityMigrationLineagesPostgres` 的 `fresh`、`legacy`、`legacy_partial`、`isolated` |
| 维护窗口与原子计费 | `TestActivityMaintenanceGate`，以及迁移谱系中的 `atomic_billing` 子测试 |

## 自动验证

- `CI=1 make test-marketing-upstream`：通过。前端类型检查、43 项组件测试、
  后端编译、9 项门禁自身测试、38 组后端契约（零跳过）、真实登录 HTTP 全链路、
  核心表 SQL 静态检查和 `git diff --check` 均成功。
- `CI=1 make test-marketing-upstream MODE=fast`：通过。前端类型检查、43 项组件
  测试、后端编译、9 项门禁自身测试、38 组后端契约（零跳过）和末尾静态检查均成功。
- 缺少 Docker 的实测：指定不存在的 Docker socket 后命令非零退出，输出实际连接
  错误，不输出通过提示。
- 无 DSN 的实测：迁移测试实际产生 `skip` 事件，即使 Go 测试命令返回 0，结果
  校验器也会拒绝通过。
- 漏选契约的实测：在隔离 PostgreSQL 中仅运行维护窗口测试，门禁因缺少其他关键
  契约而失败；退出后确认该轮创建的 PostgreSQL 容器已清理。

本地快速回归运行 `make test-marketing-upstream MODE=fast`；合并上游后的正式验收
使用 `make test-marketing-upstream`。

## 尚未完成

尚未执行下一次真实上游合并，因此还不能用实际冲突数量证明维护成本已经下降。
认证、计费、退款等外部契约变化仍需修改对应适配文件并运行门禁。
