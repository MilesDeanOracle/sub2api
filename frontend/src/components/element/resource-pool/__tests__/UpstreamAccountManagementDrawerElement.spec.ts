import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import UpstreamAccountManagementDrawerElement from '../UpstreamAccountManagementDrawerElement.vue'

const { probeUpstreamGroups } = vi.hoisted(() => ({
  probeUpstreamGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  groupsAPI: { probeUpstreamGroups }
}))

vi.mock('@vueuse/core', () => ({
  useMediaQuery: () => ref(false)
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: { count?: number }) =>
        params?.count === undefined ? key : `${key}:${params.count}`
    })
  }
})

const ElDrawerStub = {
  name: 'ElDrawer',
  inheritAttrs: false,
  props: ['modelValue', 'size'],
  emits: ['update:modelValue', 'closed'],
  template: '<aside v-if="modelValue" :data-size="size"><slot /></aside>'
}

const ElInputStub = {
  name: 'ElInput',
  inheritAttrs: false,
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<input v-bind="$attrs" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
}

const ElTableStub = {
  name: 'ElTable',
  inheritAttrs: false,
  props: ['data'],
  template: '<table v-bind="$attrs"><tbody><tr v-for="row in data" :key="row.id"><td>{{ row.name }}</td><td>{{ row.platform }}</td></tr></tbody></table>'
}

function mountDrawer() {
  return mount(UpstreamAccountManagementDrawerElement, {
    props: { modelValue: true },
    global: {
      directives: {
        loading: () => undefined
      },
      stubs: {
        ElDrawer: ElDrawerStub,
        ElInput: ElInputStub,
        ElSelect: { props: ['modelValue'], emits: ['update:modelValue', 'change'], template: '<select v-bind="$attrs"><slot /></select>' },
        ElOption: { props: ['label', 'value'], template: '<option :value="value">{{ label }}</option>' },
        ElTable: ElTableStub,
        ElTableColumn: true,
        ElForm: { template: '<form><slot /></form>' },
        ElFormItem: { template: '<label><slot /></label>' },
        ElButton: { template: '<button v-bind="$attrs" @click="$emit(\'click\')"><slot /></button>' },
        ElAlert: { props: ['title'], template: '<div>{{ title }}</div>' },
        ElTag: { template: '<span><slot /></span>' },
        ElEmpty: { props: ['description'], template: '<div>{{ description }}</div>' }
      }
    }
  })
}

describe('UpstreamAccountManagementDrawerElement', () => {
  beforeEach(() => {
    probeUpstreamGroups.mockReset()
    localStorage.clear()
    sessionStorage.clear()
  })

  it('uses a 50% desktop drawer and renders every probed group in the table', async () => {
    probeUpstreamGroups.mockResolvedValue({
      groups: [
        { id: 1, name: 'Claude Max', platform: 'anthropic', status: 'active' },
        { id: 2, name: 'Codex', platform: 'openai', status: 'inactive' }
      ],
      total: 2
    })
    const wrapper = mountDrawer()

    expect(wrapper.get('aside').attributes('data-size')).toBe('50%')
    await wrapper.get('[data-testid="upstream-base-url"]').setValue(' https://upstream.example ')
    await wrapper.get('[data-testid="upstream-access-token"]').setValue(' jwt-access-token ')
    await wrapper.get('[data-testid="upstream-probe"]').trigger('click')
    await flushPromises()

    expect(probeUpstreamGroups).toHaveBeenCalledWith({
      base_url: 'https://upstream.example',
      access_token: 'jwt-access-token'
    })
    expect(wrapper.get('[data-testid="upstream-group-table"]').text()).toContain('Claude Max')
    expect(wrapper.get('[data-testid="upstream-group-table"]').text()).toContain('Codex')
    expect(wrapper.text()).toContain('resourcePool.upstream.groupCount:2')
  })

  it('clears credentials, results, and errors when the drawer closes', async () => {
    probeUpstreamGroups.mockRejectedValue(new Error('authentication failed'))
    const wrapper = mountDrawer()
    await wrapper.get('[data-testid="upstream-base-url"]').setValue('https://upstream.example')
    await wrapper.get('[data-testid="upstream-access-token"]').setValue('sensitive-jwt')
    await wrapper.get('[data-testid="upstream-probe"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="upstream-probe-error"]').exists()).toBe(true)

    wrapper.findComponent(ElDrawerStub).vm.$emit('closed')
    await nextTick()

    const inputs = wrapper.findAllComponents(ElInputStub)
    expect(inputs.map(input => input.props('modelValue'))).toEqual(['', ''])
    expect(wrapper.find('[data-testid="upstream-probe-error"]').exists()).toBe(false)
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.length).toBe(0)
  })
})
