import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OperationsView from '../OperationsView.vue'

const { getStats, getGroupStats, getModelStats, getUpstreamBalances } = vi.hoisted(() => ({
  getStats: vi.fn(), getGroupStats: vi.fn(), getModelStats: vi.fn(), getUpstreamBalances: vi.fn(),
}))
vi.mock('@/api/admin/usage', () => ({ adminUsageAPI: { getStats } }))
vi.mock('@/api/admin/dashboard', () => ({ getGroupStats, getModelStats }))
vi.mock('@/api/admin/operations', () => ({ getUpstreamBalances }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const chartStub = {
  props: ['type', 'rows', 'loading', 'modelSource', 'startDate', 'endDate'],
  emits: ['update:modelSource'], template: '<div />',
}
function render() {
  return mount(OperationsView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    DateRangePicker: { name: 'DateRangePicker', props: ['startDate', 'endDate'], emits: ['update:startDate', 'update:endDate', 'change'], template: '<div />' },
    OperationsDistributionChart: chartStub,
    LoadingSpinner: true,
  } } })
}

beforeEach(() => {
  vi.resetAllMocks()
  getStats.mockResolvedValue({ total_actual_cost: 100, total_account_cost: 35, total_requests: 123, total_tokens: 98765 })
  getGroupStats.mockResolvedValue({ groups: [{ group_id: 1, group_name: 'Group A', total_tokens: 100 }] })
  getModelStats.mockResolvedValue({ models: [{ model: 'gpt-test', total_tokens: 100 }] })
  getUpstreamBalances.mockResolvedValue([
    { base_url: 'https://upstream.example', account_name: 'Primary', account_id: 1, account_count: 3, balance: 0, unit: 'USD', status: 'ok', kind: 'wallet' },
    { base_url: 'https://failed.example', account_name: 'Other', account_id: 2, account_count: 1, balance: null, unit: 'USD', status: 'failed', kind: 'unknown', error: 'http_error', http_status: 401 },
  ])
})

describe('OperationsView', () => {
  it('blurs table data by default and enables address links only after revealing it', async () => {
    const wrapper = render()
    await flushPromises()
    const body = wrapper.get('tbody')
    const toggle = wrapper.get('[data-testid="toggle-balance-visibility"]')
    expect(body.classes()).toContain('blur-sm')
    expect(body.attributes('inert')).toBeDefined()
    expect(body.attributes('aria-hidden')).toBe('true')
    expect(toggle.attributes('aria-pressed')).toBe('false')

    await toggle.trigger('click')
    expect(body.classes()).not.toContain('blur-sm')
    expect(body.attributes('inert')).toBeUndefined()
    expect(body.attributes('aria-hidden')).toBe('false')
    expect(toggle.attributes('aria-pressed')).toBe('true')
    expect(body.get('a').attributes('href')).toBe('https://upstream.example')
    expect(body.get('a').attributes('target')).toBe('_blank')

    await toggle.trigger('click')
    expect(body.classes()).toContain('blur-sm')
    expect(body.attributes('inert')).toBeDefined()
    wrapper.unmount()
  })

  it('identifies an outdated backend route and clears the notice after a successful retry', async () => {
    getUpstreamBalances.mockRejectedValueOnce({ status: 400, message: 'Invalid account ID' })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('operations.balanceBackendOutdated')
    expect(wrapper.text()).not.toContain('operations.balanceLoadFailed')
    await wrapper.find('section button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('operations.balanceBackendOutdated')
    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
    wrapper.unmount()
  })

  it('shows actual charges, account costs and profit, and keeps zero upstream balances distinct from failures', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="operations-consumption"]').text()).toBe('$100.0000')
    expect(wrapper.get('[data-testid="operations-cost"]').text()).toBe('$35.0000')
    expect(wrapper.get('[data-testid="operations-profit"]').text()).toBe('$65.0000')
    expect(wrapper.get('[data-testid="operations-requests"]').text()).toBe('123')
    expect(wrapper.get('[data-testid="operations-tokens"]').text()).toBe((98765).toLocaleString())
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('0.0000 USD')
    expect(rows[0].text()).not.toContain('Primary')
    expect(rows[1].text()).toContain('HTTP 401')
    expect(rows[1].text()).not.toContain('0.0000 USD')
    wrapper.unmount()
  })

  it('applies the same date range to all usage statistics without refreshing live balances', async () => {
    const wrapper = render()
    await flushPromises()
    const picker = wrapper.findComponent({ name: 'DateRangePicker' })
    picker.vm.$emit('update:startDate', '2026-10-01')
    picker.vm.$emit('update:endDate', '2026-10-31')
    await wrapper.vm.$nextTick()
    picker.vm.$emit('change')
    await flushPromises()
    const dates = { start_date: '2026-10-01', end_date: '2026-10-31' }
    expect(getStats).toHaveBeenLastCalledWith(dates)
    expect(getGroupStats).toHaveBeenLastCalledWith(dates)
    expect(getModelStats).toHaveBeenLastCalledWith({ ...dates, model_source: 'requested' })
    expect(getUpstreamBalances).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('keeps negative profits and unknown costs accurate', async () => {
    getStats.mockResolvedValueOnce({ total_actual_cost: 10, total_account_cost: 15, total_requests: 1, total_tokens: 1 })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="operations-profit"]').text()).toBe('$-5.0000')
    getStats.mockResolvedValueOnce({ total_actual_cost: 10, total_requests: 1, total_tokens: 1 })
    await wrapper.findAll('button')[0].trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="operations-profit"]').text()).toBe('—')
    wrapper.unmount()
  })

  it('does not let an older model source response overwrite the latest selection', async () => {
    const wrapper = render()
    await flushPromises()
    let resolveOld!: (value: unknown) => void
    getModelStats.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    const chart = wrapper.findAllComponents(chartStub)[0]
    chart.vm.$emit('update:modelSource', 'upstream')
    await flushPromises()
    getModelStats.mockResolvedValueOnce({ models: [{ model: 'mapped', total_tokens: 4 }] })
    chart.vm.$emit('update:modelSource', 'mapping')
    await flushPromises()
    resolveOld({ models: [{ model: 'stale', total_tokens: 1 }] })
    await flushPromises()
    expect(chart.props('rows')[0].name).toBe('mapped')
    wrapper.unmount()
  })

  it('shows failures without replacing missing statistics with zeros', async () => {
    getStats.mockRejectedValueOnce(new Error('unavailable'))
    getUpstreamBalances.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="operations-profit"]').text()).toBe('—')
    expect(wrapper.text()).toContain('operations.loadFailed')
    expect(wrapper.text()).toContain('operations.balanceLoadFailed')
    wrapper.unmount()
  })
})
