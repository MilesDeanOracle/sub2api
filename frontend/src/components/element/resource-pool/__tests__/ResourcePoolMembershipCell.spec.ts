import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import ResourcePoolMembershipCell from '../ResourcePoolMembershipCell.vue'
import type { AdminGroup } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const allGroups = [
  { id: 1, name: 'Claude 主池', platform: 'anthropic', status: 'active', rate_multiplier: 0.001, sort_order: 1 },
  { id: 2, name: 'Claude 备用池', platform: 'anthropic', status: 'active', sort_order: 2 },
  { id: 3, name: 'GPT 极速池', platform: 'openai', status: 'active', sort_order: 1 }
] as AdminGroup[]

const compatibleGroups = allGroups.filter(group => group.platform === 'anthropic')

let activeWrapper: VueWrapper | null = null

afterEach(() => {
  activeWrapper?.unmount()
  activeWrapper = null
  document.body.innerHTML = ''
})

function mountCell(selectedIds: number[], extra: Record<string, unknown> = {}) {
  const wrapper = mount(ResourcePoolMembershipCell, {
    props: { selectedIds, allGroups, compatibleGroups, ...extra },
    global: {
      stubs: {
        ElInput: {
          props: ['modelValue', 'size', 'clearable', 'prefixIcon'],
          emits: ['update:modelValue'],
          template: '<input v-bind="$attrs" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        },
        ElTag: {
          emits: ['click', 'close'],
          template: '<span v-bind="$attrs" @click="$emit(\'click\', $event)"><slot /><button class="el-tag__close" @click.stop="$emit(\'close\', $event)">x</button></span>'
        }
      }
    }
  })
  activeWrapper = wrapper
  return wrapper
}

function bodyMenu() {
  return document.querySelector('[data-testid="membership-menu"]')
}

describe('ResourcePoolMembershipCell', () => {
  it('renders membership pills and emits group-click on pill click', async () => {
    const wrapper = mountCell([1, 2])
    const pills = wrapper.findAll('[data-testid^="membership-pill-"]')
    expect(pills).toHaveLength(2)
    expect(wrapper.text()).toContain('Claude 主池')
    expect(wrapper.text()).toContain('Claude 备用池')
    await pills[1].trigger('click')
    expect(wrapper.emitted('group-click')?.[0]).toEqual([2])
  })

  it('emits a direct remove event from the pool pill', async () => {
    const wrapper = mountCell([1, 2])
    await wrapper.get('[data-testid="membership-pill-1"] .el-tag__close').trigger('click')
    expect(wrapper.emitted('remove')?.[0]).toEqual([1])
    expect(wrapper.emitted('change')).toBeUndefined()
  })

  it('does not remove a pool from a disabled pill', async () => {
    const wrapper = mountCell([1], { disabled: true })
    await wrapper.get('[data-testid="membership-pill-1"] .el-tag__close').trigger('click')
    expect(wrapper.emitted('change')).toBeUndefined()
  })

  it('shows the unassigned hint when the account belongs to no pool', () => {
    const wrapper = mountCell([])
    expect(wrapper.text()).toContain('resourcePool.accounts.membershipEmpty')
  })

  it('shows a pending pulse when there are staged changes', () => {
    const wrapper = mountCell([1], { pending: true })
    expect(wrapper.find('.animate-pulse').exists()).toBe(true)
  })

  it('opens the teleported menu listing only compatible groups and toggles memberships', async () => {
    const wrapper = mountCell([1], { accountMultiplier: 1.5 })
    await wrapper.get('[data-testid="membership-trigger"]').trigger('click')

    const menu = bodyMenu()
    expect(menu).not.toBeNull()
    const options = menu!.querySelectorAll('button[data-testid^="membership-option-"]')
    // 仅列出同平台（anthropic）资源池，不含 openai
    expect(options).toHaveLength(2)
    expect(menu!.textContent).not.toContain('GPT 极速池')
    expect(menu!.textContent).toContain('0.001x')
    expect(menu!.textContent).toContain('1.50x')
    expect(menu!.querySelector('[title="当前账号倍率"]')?.className).toContain('text-blue-600')

    // 勾选加入池 2
    ;(menu!.querySelector('[data-testid="membership-option-2"]') as HTMLElement).click()
    await Promise.resolve()
    expect(wrapper.emitted('change')?.[0]).toEqual([[1, 2]])

    // 再次点击池 1 → 从当前 props.selection 移除
    ;(menu!.querySelector('[data-testid="membership-option-1"]') as HTMLElement).click()
    await Promise.resolve()
    expect(wrapper.emitted('change')?.[1]).toEqual([[]])
  })

  it('closes the menu via backdrop click and Escape', async () => {
    const wrapper = mountCell([1])
    await wrapper.get('[data-testid="membership-trigger"]').trigger('click')
    expect(bodyMenu()).not.toBeNull()

    const backdrop = document.querySelector('.z-\\[1999\\]') as HTMLElement
    backdrop.click()
    await nextTick()
    expect(bodyMenu()).toBeNull()
    expect(wrapper.emitted('change')).toBeUndefined()
    expect(wrapper.emitted('menu-close')).toBeUndefined()

    await wrapper.get('[data-testid="membership-trigger"]').trigger('click')
    expect(bodyMenu()).not.toBeNull()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(bodyMenu()).toBeNull()
    expect(wrapper.emitted('menu-close')).toBeUndefined()
  })

  it('requests a save when the menu closes after a membership change', async () => {
    const wrapper = mountCell([1])
    await wrapper.get('[data-testid="membership-trigger"]').trigger('click')
    const menu = bodyMenu()
    const optionsList = menu?.querySelector('[data-testid="membership-option-1"]')?.parentElement
    expect(optionsList?.className).toContain('min-h-[40px]')
    expect(optionsList?.className).toContain('max-h-[320px]')
    expect(menu?.querySelector('[data-testid="membership-search"]')?.parentElement?.className).toContain('shrink-0')
    expect(optionsList?.className).toContain('overflow-y-auto')
    ;(menu!.querySelector('[data-testid="membership-option-2"]') as HTMLElement).click()
    await Promise.resolve()
    ;(document.querySelector('.z-\\[1999\\]') as HTMLElement).click()
    await nextTick()
    expect(wrapper.emitted('menu-close')).toHaveLength(1)
  })

  it('filters compatible groups by name in the dropdown search', async () => {
    const wrapper = mountCell([1])
    await wrapper.get('[data-testid="membership-trigger"]').trigger('click')
    const search = document.querySelector('[data-testid="membership-search"]') as HTMLInputElement
    search.value = '备用'
    search.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()

    const menu = bodyMenu()!
    expect(menu.querySelectorAll('button[data-testid^="membership-option-"]')).toHaveLength(1)
    expect(menu.querySelector('[data-testid="membership-option-2"]')).not.toBeNull()
  })
})
