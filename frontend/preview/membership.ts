// 临时预览：所属资源池单元格（对照 demo 药丸标签，勿提交）
import { createApp, h, reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import zh from '../src/i18n/locales/zh'
import ResourcePoolMembershipCell from '../src/components/element/resource-pool/ResourcePoolMembershipCell.vue'
import '../src/style.css'

const i18n = createI18n({ legacy: false, locale: 'zh', fallbackLocale: 'zh', messages: { zh } })

const groups = [
  { id: 1, name: 'Claude-3.5-Sonnet VIP核心池', platform: 'anthropic', status: 'active', sort_order: 1 },
  { id: 2, name: 'Claude-3-Opus 深度长文创作池', platform: 'anthropic', status: 'active', sort_order: 2 },
  { id: 9, name: 'Claude 实验池', platform: 'anthropic', status: 'active', sort_order: 3 }
]

const state = reactive({ rowA: [1, 2], rowB: [] as number[] })

createApp({
  render: () =>
    h('div', { style: 'display:flex;flex-direction:column;gap:28px' }, [
      h(ResourcePoolMembershipCell, {
        selectedIds: state.rowA,
        allGroups: groups,
        compatibleGroups: groups,
        pending: true,
        onChange: (ids: number[]) => { state.rowA = ids }
      }),
      h(ResourcePoolMembershipCell, {
        selectedIds: state.rowB,
        allGroups: groups,
        compatibleGroups: groups,
        onChange: (ids: number[]) => { state.rowB = ids }
      })
    ])
}).use(i18n).mount('#app')
