# Google Tag Manager / GA4

生产构建在 `src/main.ts` 完成首次路由导航、挂载应用后，通过
`src/utils/tagManager.ts` 异步加载 GTM 容器 `GTM-PQ6C6RBB`，整个应用只加载一次。
`pnpm dev` 不加载统计脚本；`pnpm build` 的产物会加载，包括本地生产预览。
后端 CSP 同时允许 GTM 脚本和 GA4 数据收集域名。

## GTM 容器配置

加载 GTM 容器不等于已经配置 GA4。在 Google Tag Manager 中：

1. 创建 Google 标签，填写 GA4 数据流的标签 ID（`G-...`），设置在所有页面初始化时触发。
2. 在 GA4 数据流的增强型衡量设置中，启用网页浏览，并在高级设置中启用
   根据浏览器历史记录事件追踪网页变化。Vue Router 使用 History 模式，可由此追踪页面切换。
3. 发布容器，在 GA4 实时报告或 DebugView 中验证首次访问和页面切换。

本项目不额外手动发送 `page_view`，也不添加重复的 GA4 配置脚本。
容器中应避免再用 History Change 触发器发送同一份页面浏览，防止重复计数。
若容器已经有 Google 标签，检查现有配置即可。
