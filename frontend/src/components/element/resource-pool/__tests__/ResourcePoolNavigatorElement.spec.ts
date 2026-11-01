import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ResourcePoolNavigatorElement from '../ResourcePoolNavigatorElement.vue'
import type { AdminGroup } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const groups = [
  { id: 1, name: 'Claude 主池', description: null, platform: 'anthropic', status: 'active', account_count: 3, sort_order: 1 },
  { id: 2, name: 'Codex 备用池', description: null, platform: 'openai', status: 'inactive', account_count: 2, sort_order: 1 }
] as AdminGroup[]

function mountNavigator(scope: 'all' | 'ungrouped' | 'group' = 'all', groupId?: number) {
  return mount(ResourcePoolNavigatorElement, {
    props: { groups, scope, groupId, allCount: 9, ungroupedCount: 4 },
    global: {
      stubs: {
        ElCard: { template: '<section><slot name="header"/><slot/></section>' },
        ElScrollbar: { template: '<div><slot/></div>' },
        ElInput: true,
        ElButton: { template: '<button><slot/></button>' },
        ElTag: { template: '<span><slot/></span>' },
        ElIcon: { template: '<i><slot/></i>' },
        ElTooltip: { template: '<span><slot/></span>' },
        ElEmpty: true,
        Grid: true,
        Box: true
      }
    }
  })
}

describe('ResourcePoolNavigatorElement', () => {
  it('renders synthetic pools and platform-grouped real pools with counts', () => {
    const wrapper = mountNavigator()
    expect(wrapper.text()).toContain('resourcePool.navigator.allAccounts')
    expect(wrapper.text()).toContain('resourcePool.navigator.ungrouped')
    expect(wrapper.text()).toContain('Claude 主池')
    expect(wrapper.text()).toContain('Codex 备用池')
    expect(wrapper.text()).toContain('9')
  })

  it('emits a selected real group', async () => {
    const wrapper = mountNavigator('group', 1)
    await wrapper.get('[data-testid="pool-group-2"]').trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual([{ scope: 'group', groupId: 2 }])
  })
})
