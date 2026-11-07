## Why

管理员需要统一查看真实上游余额和指定时间内的运营收益；从现有使用记录页复制模型与分组分布到独立运营页面，原使用记录页保留原有图表及其功能。

## What Changes

- 新增管理员「运营管理」导航和页面，展示按日期筛选的总消费、成本、利润、总请求数、总 Token。
- 后端使用账号保存的 base_url、api_key 请求上游 `/v1/usage`，按规范化 Base URL 去重，返回实时余额及查询状态。
- 复制模型与分组分布，在运营管理页按用户明确要求新增 ECharts 依赖重写，保留模型来源、指标切换与用户明细；使用记录页继续使用原有图表。

## Capabilities

### New Capabilities

- `operations-management`: 运营统计、上游余额、ECharts 模型与分组分布。

## Impact

复用已有统计、管理员认证、上游 HTTP 传输、地址验证、代理、日期选择器、Select、shadcn Button 和用户明细组件；不需要数据库迁移。使用记录页及其他页面保留现有 Chart.js 分布组件，运营管理页新增独立 ECharts 实现。
