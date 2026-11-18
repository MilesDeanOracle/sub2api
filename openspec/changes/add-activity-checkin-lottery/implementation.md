# 首期活动实施与启用

## 已实施

- 首调事件：`usage_logs` 中正常计费且非 cyber 拒绝的新增记录触发一次性 `activity_first_calls`。老用户可由历史用量取得签到资格，但历史记录不回填为上线后的新用户首调事件。
- 签到：北京时间自然日，每日一次；每日 10 美分，连续第 7/14/21/28 天加 50 美分。每人每月上限取配置值与当前 `¥15` 对应账户面值中的较小值（当前 1:1 时最多 1500 美分），全站月预算独立控制。预算用尽后仍记签到，奖励为零。
- 签到实验：按用户 ID 模 100 稳定分组，覆盖比例默认 50%，可在管理界面调到 100%。未命中的账户不显示签到入口，也不能通过接口领奖。
- 二充：用户 ID 偶数为试验组。按支付时间排序前两笔付费余额充值，必须都是已完成的 CNY 订单，实付金额分别达到配置门槛；两单间有非 cyber 的计费调用，且间隔不超过 30 天。仅第二笔可领取一次固定奖励。第三笔不能替代不合格的第二笔。
- 退款：已领二充奖励的首笔或第二笔充值进入退款状态时，产生待审记录；系统不自动扣回活动余额。管理员在支付仪表盘查看和标记已审。
- 发奖：签到记录、奖励流水、用户余额在同一数据库事务内提交。用户行和全站配置行锁防并发超发；提交后清理认证及计费余额缓存。
- 共用预算：签到和二充还受活动全站总预算与单用户累计上限限制，未配置这两项时不能开启任何现金活动。

## 默认配置

迁移 `257_activity_rewards.sql` 创建所有活动表，两个活动开关均默认关闭。支付管理仪表盘的“增长活动配置”可设置并启用活动。配置金额使用整数最小单位：奖励和预算为美元账户余额的**美分**，最低实付金额为**人民币分**。每人签到月上限数据库约束为最多 1500 美分。

启用前须核对 `balance_recharge_multiplier`、充值优惠叠加后的毛利、可承担的全站预算和退款处理人员。`¥1` 充值到账 `$1` 是账户面值口径，并非汇率。未配置二充奖励和总预算时接口拒绝启用。

## 接口

- 用户：`GET /api/v1/activity/status`、`POST /api/v1/activity/checkin`、`POST /api/v1/activity/second-recharge/claim`。
- 管理员：`GET/PUT /api/v1/admin/activity/config`、`GET /api/v1/admin/activity/refund-reviews`、`POST /api/v1/admin/activity/refund-reviews/:orderID/review`。

## 上线核对

1. 在 PostgreSQL 测试环境执行迁移，确认重复签到、并发签到、月度封顶及全站预算耗尽时余额与流水一致。
2. 用支付沙箱覆盖二充履约回调重试、非 CNY 订单、退款订单及第三笔订单不替代第二笔。
3. 核对首调事件只由成功计费用量触发；观察首充后 30 天二充率和签到用户次月真实调用留存。若仅签到次数增加、实际调用未改善，停止发奖。

首期不含常驻抽奖、补签卡或首调现金奖；这些仍是备选阶段的方案。


### Configurable check-in eligibility

Check-in now has its own cumulative actual-spend threshold (`checkin_min_spend_cents`), entered in USD in the admin form. Zero removes the API-call requirement; positive thresholds compare exact cumulative `actual_cost` on non-cyber usage. Both status and transactional check-in enforce the same rule. First-call metrics and second-recharge eligibility remain independent. Migration 258 defaults to $0.01 to avoid silently opening eligibility for every account; administrators may explicitly save zero. Rollout, active-account and budget checks still apply.
