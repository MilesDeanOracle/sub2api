## Purpose

为管理员提供一个统一的资源池工作台，使分组配置、账号状态与账号分组关系可以在同一业务上下文中完成浏览和维护，同时保持现有账号与分组能力兼容。

## ADDED Requirements

### Requirement: Unified resource pool navigation
系统 SHALL 在管理端提供单一的“资源池管理”菜单入口，并在一个工作台内提供分组导航和账号池管理。

#### Scenario: Open resource pool workspace
- **WHEN** 管理员点击“资源池管理”菜单
- **THEN** 系统展示分组导航和账号列表组成的统一工作台

#### Scenario: Legacy account route
- **WHEN** 管理员访问原账号管理路由
- **THEN** 系统进入资源池工作台的全部账号视图

#### Scenario: Legacy group route
- **WHEN** 管理员访问原分组管理路由
- **THEN** 系统进入资源池工作台并保留分组管理上下文

### Requirement: Resource pool views
系统 SHALL 提供全部账号、未分组账号和真实分组三类资源池视图，并显示每个视图的账号数量或状态摘要。

#### Scenario: View all accounts
- **WHEN** 管理员选择“全部账号”
- **THEN** 系统显示所有符合当前筛选条件的账号

#### Scenario: View ungrouped accounts
- **WHEN** 管理员选择“未分组账号”
- **THEN** 系统仅显示未绑定任何分组的账号

#### Scenario: View a group pool
- **WHEN** 管理员选择一个真实分组
- **THEN** 系统显示该分组摘要以及属于该分组的账号

### Requirement: Contextual account maintenance
系统 SHALL 允许管理员在当前资源池上下文中新建账号、编辑账号、添加现有账号以及执行批量账号操作。

#### Scenario: Create account in selected group
- **WHEN** 管理员在真实分组视图中创建账号
- **THEN** 创建表单默认选中当前分组，并允许管理员在提交前调整分组

#### Scenario: Add existing accounts to group
- **WHEN** 管理员从真实分组视图选择“添加现有账号”并确认账号集合
- **THEN** 系统为所选账号添加当前分组关系且不移除账号已有的其他分组关系

#### Scenario: Remove accounts from group
- **WHEN** 管理员在真实分组视图中选择账号并执行“移出当前分组”
- **THEN** 系统仅删除这些账号与当前分组的关系，不删除账号本身或其他分组关系

#### Scenario: Bulk account actions
- **WHEN** 管理员选中多个账号
- **THEN** 系统提供与当前视图适配的批量编辑、加入分组和移出当前分组操作

### Requirement: Account-group relationship clarity
系统 SHALL 明确表达账号和分组为多对多关系，并允许管理员查看账号所属分组及当前分组内关系属性。

#### Scenario: Display account memberships
- **WHEN** 账号属于一个或多个分组
- **THEN** 系统在账号行展示可识别的分组标签

#### Scenario: Navigate through membership
- **WHEN** 管理员点击账号的分组标签
- **THEN** 系统切换到对应分组资源池视图

#### Scenario: Edit group-relative priority
- **WHEN** 管理员修改账号在当前分组中的关系优先级
- **THEN** 系统只更新该账号与当前分组的关系优先级

### Requirement: Contextual group maintenance
系统 SHALL 在资源池工作台提供创建、编辑和访问分组高级配置的入口。

#### Scenario: Create group
- **WHEN** 管理员在资源池工作台创建分组成功
- **THEN** 系统刷新分组导航并选中新建分组

#### Scenario: Edit selected group
- **WHEN** 管理员在真实分组视图编辑分组
- **THEN** 系统保存分组配置并刷新当前分组摘要

#### Scenario: Open advanced group settings
- **WHEN** 管理员需要维护模型路由、定价、RPM 或其他高级分组配置
- **THEN** 系统从当前分组上下文进入既有的完整分组配置能力

### Requirement: Responsive Element management experience
资源池工作台 SHALL 使用管理端 Element 组件体系，并在桌面和窄屏下保持可操作。

#### Scenario: Desktop layout
- **WHEN** 可用宽度满足桌面布局
- **THEN** 系统同时展示左侧资源池导航和右侧账号工作区

#### Scenario: Narrow layout
- **WHEN** 可用宽度不足以同时展示两栏
- **THEN** 系统将资源池导航折叠为抽屉或选择器，账号操作仍然可用

### Requirement: Safe group deletion semantics
系统 SHALL 在删除分组前明确告知删除只影响分组与绑定关系，不删除账号，并继续遵循既有服务端约束。

#### Scenario: Confirm group deletion
- **WHEN** 管理员请求删除分组
- **THEN** 系统显示影响范围并要求确认后才提交删除

### Requirement: Inspect compatible upstream resource pools
系统 SHALL 在资源池管理页提供“上游账号管理”入口，使管理员可以临时连接兼容 Sub2API 管理接口的上游并查看其全部分组，且不得持久化探测凭据。

#### Scenario: Open upstream account management
- **WHEN** 管理员点击“上游账号管理”
- **THEN** 系统从右侧打开占浏览器宽度 50% 的抽屉，并显示 Base URL、API Key 与探测操作

#### Scenario: Probe upstream groups
- **WHEN** 管理员填写有效的上游 Base URL 与管理员 API Key 并点击“探测”
- **THEN** 当前服务端使用 Bearer 凭据请求该上游兼容的 `/api/v1/keys` 接口，并聚合响应中 API Key 绑定的分组
- **AND** 抽屉通过表格展示响应中的全部分组

#### Scenario: Reject an invalid upstream probe
- **WHEN** 上游不可达、鉴权失败或返回不兼容的分组响应
- **THEN** 系统显示可操作的失败信息且不保留上一次响应为本次成功结果

#### Scenario: Keep probe credentials ephemeral
- **WHEN** 管理员关闭抽屉或刷新页面
- **THEN** 系统不从数据库、浏览器持久化存储或日志中恢复本次 Base URL 与 API Key

