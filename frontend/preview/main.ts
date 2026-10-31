// 临时预览入口：以真实组件 + 项目 Tailwind 配置渲染左侧导航菜单（对照 demo 用，勿提交）
import { createApp } from 'vue'
import { createI18n } from 'vue-i18n'
import '../src/style.css'
import zh from '../src/i18n/locales/zh'
import ResourcePoolNavigatorElement from '../src/components/element/resource-pool/ResourcePoolNavigatorElement.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  fallbackLocale: 'zh',
  messages: { zh }
})

// 与 demo resource_pool_demo.html 相同的 mock 分组数据（映射为 AdminGroup 字段）
const groups = [
  { id: 1, name: 'Claude-3.5-Sonnet VIP核心池', platform: 'anthropic', status: 'active', sort_order: 1, rate_multiplier: 1.2, account_count: 8, active_account_count: 7, rate_limited_account_count: 1 },
  { id: 2, name: 'Claude-3-Opus 深度长文创作池', platform: 'anthropic', status: 'active', sort_order: 2, rate_multiplier: 1.5, account_count: 4, active_account_count: 4, rate_limited_account_count: 0 },
  { id: 3, name: 'OpenAI GPT-4o 极速全功能池', platform: 'openai', status: 'active', sort_order: 1, rate_multiplier: 1, account_count: 12, active_account_count: 11, rate_limited_account_count: 1 },
  { id: 4, name: 'OpenAI O1 Reasoning 专线池', platform: 'openai', status: 'active', sort_order: 2, rate_multiplier: 2, account_count: 5, active_account_count: 5, rate_limited_account_count: 0 },
  { id: 5, name: 'DeepSeek-R1 普惠大吞吐池', platform: 'deepseek', status: 'active', sort_order: 1, rate_multiplier: 0.8, account_count: 16, active_account_count: 16, rate_limited_account_count: 0 },
  { id: 6, name: 'Gemini 2.0 Flash 实验池', platform: 'gemini', status: 'active', sort_order: 1, rate_multiplier: 1, account_count: 6, active_account_count: 6, rate_limited_account_count: 0 },
  { id: 7, name: 'Kimi Moonshot 200k 专线池', platform: 'kimi', status: 'inactive', sort_order: 1, rate_multiplier: 1, account_count: 2, active_account_count: 0, rate_limited_account_count: 2 }
]

createApp(ResourcePoolNavigatorElement, {
  groups,
  scope: 'group',
  groupId: 1,
  allCount: 23,
  ungroupedCount: 3,
  limitedCount: 5
}).use(i18n).mount('#app')
