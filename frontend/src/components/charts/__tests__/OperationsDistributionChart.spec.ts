import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import OperationsDistributionChart from '../OperationsDistributionChart.vue'

const { init, getUserBreakdown } = vi.hoisted(() => ({ init: vi.fn(), getUserBreakdown: vi.fn() }))
vi.mock('echarts/core', () => ({ init, use: vi.fn() }))
vi.mock('@/api/admin/dashboard', () => ({ getUserBreakdown }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('en') }) }))
const selectStub = {
  props: ['modelValue', 'options'], emits: ['update:modelValue'],
  template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>',
}
const rows = [
  { id: 'A', name: 'A', requests: 1, total_tokens: 10, actual_cost: 5, cost: 5 },
  { id: 'B', name: 'B', requests: 2, total_tokens: 20, actual_cost: 2, cost: 2 },
]
const props = { type: 'model' as const, title: 'Models', rows, loading: false, modelSource: 'requested' as const, startDate: '2026-10-01', endDate: '2026-10-31' }
const options = { global: { stubs: { Select: selectStub, LoadingSpinner: true, UserBreakdownSubTable: true } } }

beforeEach(() => {
  vi.resetAllMocks()
  init.mockImplementation(() => ({ setOption: vi.fn(), resize: vi.fn(), dispose: vi.fn() }))
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} })
})

describe('OperationsDistributionChart', () => {
  it('renders ECharts and keeps metric selections independent without refetching', async () => {
    const model = mount(OperationsDistributionChart, { ...options, props })
    const group = mount(OperationsDistributionChart, { ...options, props: { ...props, type: 'group', title: 'Groups' } })
    await flushPromises()
    const modelChart = init.mock.results[0].value
    const groupChart = init.mock.results[1].value
    expect(modelChart.setOption.mock.lastCall[0].series[0].data).toEqual([{ name: 'B', value: 20 }, { name: 'A', value: 10 }])
    await model.findAll('select')[1].setValue('actual_cost')
    expect(modelChart.setOption.mock.lastCall[0].series[0].data).toEqual([{ name: 'A', value: 5 }, { name: 'B', value: 2 }])
    expect(groupChart.setOption.mock.lastCall[0].series[0].data[0].value).toBe(20)
    expect(getUserBreakdown).not.toHaveBeenCalled()
    model.unmount(); group.unmount()
    expect(modelChart.dispose).toHaveBeenCalledOnce()
    expect(groupChart.dispose).toHaveBeenCalledOnce()
  })

  it('loads user breakdowns for the current model source and resets them on date changes', async () => {
    getUserBreakdown.mockResolvedValue({ users: [{ user_id: 1 }] })
    const wrapper = mount(OperationsDistributionChart, { ...options, props: { ...props, modelSource: 'mapping' } })
    await wrapper.find('tbody button').trigger('click')
    await flushPromises()
    expect(getUserBreakdown).toHaveBeenCalledWith({ start_date: props.startDate, end_date: props.endDate, model: 'B', model_source: 'mapping' })
    expect(wrapper.findComponent({ name: 'UserBreakdownSubTable' }).exists()).toBe(true)
    await wrapper.setProps({ startDate: '2026-11-01', endDate: '2026-11-03' })
    expect(wrapper.findComponent({ name: 'UserBreakdownSubTable' }).exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not draw nonzero pie sectors for zero usage', async () => {
    const wrapper = mount(OperationsDistributionChart, { ...options, props: { ...props, rows: [{ ...rows[0], total_tokens: 0, actual_cost: 0 }] } })
    expect(wrapper.text()).toContain('common.noData')
    expect(init.mock.results[0].value.setOption.mock.lastCall[0].series[0].data).toEqual([])
    wrapper.unmount()
  })
})
