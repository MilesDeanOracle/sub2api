import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import AdminLotteryConfig from '../AdminLotteryConfig.vue'

const mocks = vi.hoisted(() => ({
  config: vi.fn(),
  save: vi.fn(),
  user: vi.fn(),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }),
}))
vi.mock('@/api/admin/activity', () => ({
  getAdminLotteryConfig: mocks.config,
  updateAdminLotteryConfig: mocks.save,
  getAdminLotteryUser: mocks.user,
}))
vi.mock('@/api/admin/activityAudience', () => ({
  activityAudienceAPI: { get: vi.fn(), save: vi.fn() },
}))
enableAutoUnmount(afterEach)

const tiers = [
  { position: 1, reward_cents: 50, weight: 8738 },
  { position: 2, reward_cents: 100, weight: 961 },
  { position: 3, reward_cents: 200, weight: 150 },
  { position: 4, reward_cents: 300, weight: 100 },
  { position: 5, reward_cents: 500, weight: 40 },
  { position: 6, reward_cents: 1000, weight: 9 },
  { position: 7, reward_cents: 2000, weight: 1 },
  { position: 8, reward_cents: 5000, weight: 1 },
]

const config = {
  enabled: true,
  unit_pay_cents: 1000,
  budget_cents: 500000,
  spent_cents: 0,
  expected_cents: 629,
  outstanding_tickets: 0,
  expected_liability_cents: 0,
  tiers_locked: false,
  tiers,
}

const options = {
  global: {
    stubs: {
      ActivityAudience: true,
      RewardFieldTip: true,
    },
  },
}

beforeEach(() => {
  vi.restoreAllMocks()
  vi.resetAllMocks()
  mocks.config.mockResolvedValue({ ...config, tiers: tiers.map((tier) => ({ ...tier })) })
})

describe('AdminLotteryConfig probability test', () => {
  it('simulates the requested draws locally and reports wins for every active tier', async () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.999999)
    const wrapper = mount(AdminLotteryConfig, options)
    await flushPromises()

    await wrapper.get('#lottery-test-count').setValue('100')
    await wrapper.get('[data-testid="lottery-test-run"]').trigger('click')
    await flushPromises()

    const rows = wrapper.findAll('[data-testid="lottery-test-results"] li')
    expect(rows).toHaveLength(8)
    expect(rows.reduce((sum, row) => sum + Number(row.attributes('data-wins')), 0)).toBe(100)
    expect(rows.at(-1)?.attributes('data-wins')).toBe('100')
    expect(wrapper.get('[data-testid="lottery-test-results"]').text()).toContain('testSummary')
    expect(mocks.save).not.toHaveBeenCalled()
  })

  it('rejects counts outside the supported range without producing results', async () => {
    const wrapper = mount(AdminLotteryConfig, options)
    await flushPromises()

    await wrapper.get('#lottery-test-count').setValue('0')
    await wrapper.get('[data-testid="lottery-test-run"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="lottery-test-results"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('testCountInvalid')
  })

  it('discards previous results when a prize weight changes', async () => {
    const wrapper = mount(AdminLotteryConfig, options)
    await flushPromises()

    await wrapper.get('[data-testid="lottery-test-run"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="lottery-test-results"]').exists()).toBe(true)

    await wrapper.get('#lottery-weight-1').setValue('9000')
    expect(wrapper.find('[data-testid="lottery-test-results"]').exists()).toBe(false)
  })
})
