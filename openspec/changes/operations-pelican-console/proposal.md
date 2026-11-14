## Why

管理员需要使用既有账号持续生成、比较 HTML 作品及上游性能，现有本地鹈鹕检测不支持独立运营、多任务和共享历史。

## What Changes

- 新增仅管理员可见、在新标签打开的运营台，提供鹈鹕墙与配置管理。
- 新增两张表保存多任务配置、持久执行队列、结果与对象位置。
- 按账号协议发现模型并生成 HTML，所有生成共享四个数据库执行槽位。
- 复用 S3 连接、沙箱预览；独立调度及七天数据库与对象双清理。

## Capabilities

### New Capabilities
- `operations-pelican-console`: 管理员运营台、多任务生成、全局并发及结果保留。

### Modified Capabilities

无。

## Impact

涉及 Vue 路由、管理员入口、Gin 管理 API、service/repository、Ent schema、SQL 迁移与启动注入。沿用已有依赖及登录策略。详细设计为 docs/operations-console-design.md。
