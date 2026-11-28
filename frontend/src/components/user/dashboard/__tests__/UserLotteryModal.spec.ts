import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import UserLotteryModal from '../UserLotteryModal.vue'
import AdminLotteryConfig from '@/components/admin/payment/AdminLotteryConfig.vue'

const mocks = vi.hoisted(() => ({
  status: vi.fn(),
  draw: vi.fn(),
  history: vi.fn(),
  withdraw: vi.fn(),
  adminGet: vi.fn(),
  adminSave: vi.fn(),
  adminUser: vi.fn(),
  refreshUser: vi.fn(),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
    locale: ref('zh'),
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/purchase' }),
  RouterLink: { props: ['to'], template: '<a :href="to" @click.prevent><slot /></a>' },
}))

const authUser = ref({ id: 1 })
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ refreshUser: mocks.refreshUser, get user() { return authUser.value } }),
}))

vi.mock('@/api/activity', () => ({
  getLotteryStatus: mocks.status,
  drawLottery: mocks.draw,
  getLotteryHistory: mocks.history,
  withdrawActivityWallet: mocks.withdraw,
}))

vi.mock('@/api/admin/activity', () => ({
  getAdminLotteryConfig: mocks.adminGet,
  updateAdminLotteryConfig: mocks.adminSave,
  getAdminLotteryUser: mocks.adminUser,
}))

vi.mock('@/api/admin/activityAudience', () => ({
  activityAudienceAPI: { get: vi.fn(), save: vi.fn() },
}))

enableAutoUnmount(afterEach)

const baseTiers = [
  { position: 1, reward_cents: 50, reward: 0.5, weight: 8738 },
  { position: 2, reward_cents: 100, reward: 1, weight: 961 },
  { position: 3, reward_cents: 200, reward: 2, weight: 150 },
  { position: 4, reward_cents: 300, reward: 3, weight: 100 },
  { position: 5, reward_cents: 500, reward: 5, weight: 40 },
  { position: 6, reward_cents: 1000, reward: 10, weight: 9 },
  { position: 7, reward_cents: 2000, reward: 20, weight: 1 },
  { position: 8, reward_cents: 5000, reward: 50, weight: 1 },
]

describe('UserLotteryModal & AdminLotteryConfig', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    Object.defineProperty(window, 'matchMedia', { writable: true, value: vi.fn().mockReturnValue({ matches: true }) })
    sessionStorage.clear()
    authUser.value = { id: 1 }
    mocks.status.mockResolvedValue({
      visible: true,
      enabled: true,
      awards_enabled: true,
      unit_pay_cents: 1000,
      unit_pay_yuan: 10,
      remaining_draws: 3,
      total_won_cents: 150,
      total_won: 1.5,
      today_won_cents: 50,
      today_won: 0.5,
      draw_blocked: false,
      pool_paused: false,
      ten_draw_paused: false,
      wallet_balance_cents: 150,
      wallet_balance: 1.5,
      wallet_total_earned: 1.5,
      wallet_total_withdrawn: 0,
      withdraw_blocked: false,
      tiers: baseTiers.map(({ position, reward_cents, reward, weight }) => ({ position, reward_cents, reward, probability: weight / 10000 })),
    })
    mocks.adminGet.mockResolvedValue({
      enabled: false,
      unit_pay_cents: 1000,
      budget_cents: 20000,
      spent_cents: 0,
      expected_cents: 62.9,
      tiers: baseTiers,
    })
    mocks.adminSave.mockImplementation(async (payload) => ({
      ...payload,
      spent_cents: 0,
      expected_cents: 62.9,
    }))
  })

  it('opens the existing nine-grid from the shared reward action without drawing', async () => {
    const wrapper = mount(UserLotteryModal, {
      props: { quickEntry: true },
      global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } },
    })
    await flushPromises()
    window.dispatchEvent(new CustomEvent('activity-lottery-requested'))
    await flushPromises()
    expect(wrapper.findAll('[data-testid="lottery-prize-card"]')).toHaveLength(8)
    expect(mocks.draw).not.toHaveBeenCalled()
  })

  it('reuses the unresolved batch key after a lost response', async () => {
    mocks.draw.mockRejectedValue(new Error('Network Error'))
    const wrapper = mount(UserLotteryModal, {
      props: { quickEntry: true },
      global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } },
    })
    await flushPromises()
    await wrapper.get('[data-testid="lottery-entry"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="lottery-draw-1"]').trigger('click')
    await flushPromises()
    const originalKey = mocks.draw.mock.calls[0][1]
    await wrapper.get('[data-testid="lottery-draw-retry"]').trigger('click')
    await flushPromises()
    expect(mocks.draw).toHaveBeenCalledTimes(2)
    expect(mocks.draw.mock.calls[1]).toEqual([1, originalKey])
  })

  it('restores an unresolved request after remount even when status loading fails', async () => {
    mocks.draw.mockRejectedValue(new Error('Network Error'))
    const options = { props: { quickEntry: true }, global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } } }
    const first = mount(UserLotteryModal, options)
    await flushPromises()
    await first.get('[data-testid="lottery-entry"]').trigger('click')
    await flushPromises()
    await first.get('[data-testid="lottery-draw-1"]').trigger('click')
    await flushPromises()
    const key = mocks.draw.mock.calls[0][1]
    first.unmount()
    mocks.status.mockRejectedValue(new Error('offline'))
    const second = mount(UserLotteryModal, options)
    await flushPromises()
    await second.get('[data-testid="lottery-entry"]').trigger('click')
    await flushPromises()
    await second.get('[data-testid="lottery-draw-retry"]').trigger('click')
    await flushPromises()
    expect(mocks.draw.mock.calls[1]).toEqual([1, key])
  })

  it('clears a known rejected request and keeps unresolved requests isolated per user', async () => {
    mocks.draw.mockRejectedValue({ response: { status: 400, data: { message: 'Insufficient draws' } } })
    const wrapper = mount(UserLotteryModal, {
      props: { quickEntry: true },
      global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } },
    })
    await flushPromises()
    await wrapper.get('[data-testid="lottery-entry"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="lottery-draw-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="lottery-draw-retry"]').exists()).toBe(false)
    mocks.draw.mockRejectedValue(new Error('offline'))
    await wrapper.get('[data-testid="lottery-draw-1"]').trigger('click')
    await flushPromises()
    expect(sessionStorage.getItem('activity-lottery-pending:1')).toBeTruthy()
    authUser.value = { id: 2 }
    await flushPromises()
    await wrapper.get('[data-testid="lottery-entry"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="lottery-draw-retry"]').exists()).toBe(false)
    expect(sessionStorage.getItem('activity-lottery-pending:1')).toBeTruthy()
  })

  it('retains the previous key when authentication expires during result recovery', async () => {
    mocks.draw.mockRejectedValueOnce(new Error('offline')).mockRejectedValueOnce({response:{status:401}})
    const wrapper=mount(UserLotteryModal,{props:{quickEntry:true},global:{stubs:{BaseDialog:{props:['show'],template:'<div v-if="show"><slot /></div>'}}}})
    await flushPromises()
    await wrapper.get('[data-testid="lottery-entry"]').trigger('click');await flushPromises()
    await wrapper.get('[data-testid="lottery-draw-1"]').trigger('click');await flushPromises()
    const key=mocks.draw.mock.calls[0][1]
    await wrapper.get('[data-testid="lottery-draw-retry"]').trigger('click');await flushPromises()
    expect(wrapper.find('[data-testid="lottery-draw-retry"]').exists()).toBe(true)
    expect(JSON.parse(sessionStorage.getItem('activity-lottery-pending:1')!).key).toBe(key)
  })

  it('disables prize edits when existing tickets lock the pool', async () => {
    mocks.adminGet.mockResolvedValueOnce({enabled:false,unit_pay_cents:1000,budget_cents:20000,spent_cents:0,expected_cents:62.9,tiers:baseTiers,tiers_locked:true,outstanding_tickets:10,expected_liability_cents:629})
    const wrapper = mount(AdminLotteryConfig,{global:{stubs:{ActivityAudience:true}}})
    await flushPromises()
    expect(wrapper.get('input[name=reward_1]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('input[name=weight_1]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('#lottery-budget').attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).toContain('operationsConsole.lotteryConfig.outstanding: 10')
  })

  it('preserves locked historical odds for budget saves until the entire pool can switch', async () => {
    const loaded = await mocks.adminGet()
    const historical = { ...loaded, tiers_locked: true, tiers: loaded.tiers.map((tier: { position: number; weight: number }) => ({ ...tier, weight: tier.position >= 7 ? 0 : tier.weight })) }
    mocks.adminGet.mockResolvedValue(historical)
    const wrapper = mount(AdminLotteryConfig, { global: { stubs: { ActivityAudience: true } } })
    await flushPromises()
    expect(wrapper.get('[data-testid="lottery-use-defaults"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#lottery-budget').setValue('250')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.adminSave).toHaveBeenCalledTimes(1)
    expect(mocks.adminSave.mock.calls[0][0].tiers[6].weight).toBe(0)
  })

  it('renders eight perimeter prizes with a central draw and highlights the server result', async () => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockReturnValue({ matches: true }),
    })
    mocks.draw.mockResolvedValue({
      items: [
        {
          id: 11,
          batch_key: 'k1',
          draw_count: 1,
          draw_index: 1,
          tier_position: 2,
          reward_cents: 100,
          reward: 1,
          created_at: '2026-11-25T03:00:00Z',
        },
      ],
      total_reward_cents: 100,
      total_reward: 1,
      status: {
        visible: true,
        enabled: true,
        awards_enabled: true,
        unit_pay_cents: 1000,
        unit_pay_yuan: 10,
        remaining_draws: 2,
        total_won_cents: 250,
        total_won: 2.5,
        today_won_cents: 150,
        today_won: 1.5,
        draw_blocked: false,
        pool_paused: false,
        ten_draw_paused: false,
        wallet_balance_cents: 250,
        wallet_balance: 2.5,
        wallet_total_earned: 2.5,
        wallet_total_withdrawn: 0,
        withdraw_blocked: false,
        tiers: baseTiers.map(({ position, reward_cents, reward, weight }) => ({ position, reward_cents, reward, probability: weight / 10000 })),
      },
    })

    const wrapper = mount(UserLotteryModal, {
      props: { quickEntry: true },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<div v-if="show" data-testid="dialog-stub"><slot /></div>',
          },
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="lottery-remaining-badge"]').text()).toBe('3')
    await wrapper.get('[data-testid="lottery-entry"]').trigger('click')
    await flushPromises()

    const cards = wrapper.findAll('[data-testid="lottery-prize-card"]')
    expect(cards).toHaveLength(8)
    expect(wrapper.get('[data-testid="lottery-nine-grid"]').element.children).toHaveLength(9)
    expect(wrapper.get('[data-testid="lottery-nine-grid"] [data-testid="lottery-draw-1"]').exists()).toBe(true)
    expect(cards[0]!.text()).toBe('$0.5')
    expect(cards[7]!.text()).toBe('$50')
    expect(cards.every((card) => !card.classes().includes('is-inactive'))).toBe(true)
    expect(cards.every((card) => !card.text().includes('%'))).toBe(true)
    expect(wrapper.get('.chest-rules').text()).toContain('87.38%')
    // 3 draws < 10 -> 10x disabled, 1x enabled
    expect(wrapper.get('[data-testid="lottery-draw-1"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="lottery-draw-10"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-testid="lottery-draw-1"]').trigger('click')
    await flushPromises()
    expect(mocks.draw).toHaveBeenCalledWith(1, expect.any(String))
    expect(wrapper.get('[data-testid="lottery-stat-remaining"]').text()).toBe('2')
    expect(wrapper.get('[data-testid="lottery-stat-total"]').text()).toBe('$2.50')
    expect(wrapper.findAll('.prize-card.is-hit')).toHaveLength(1)
    expect(wrapper.get('.prize-card.is-hit').attributes('data-tier-position')).toBe('2')
    expect(wrapper.get('.prize-card.is-hit').text()).toContain('dashboard.activity.lottery.winningPrize')
  })

  it('links users without draws to the purchase route', async () => {
    const status = await mocks.status()
    mocks.status.mockResolvedValue({ ...status, remaining_draws: 0 })
    const wrapper = mount(UserLotteryModal, {
      props: { quickEntry: true },
      global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } },
    })
    await flushPromises()
    await wrapper.get('[data-testid="lottery-entry"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="lottery-go-recharge"]').attributes('href')).toBe('/purchase')
    expect(wrapper.get('[data-testid="lottery-draw-1"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="lottery-go-recharge"]').trigger('click')
    expect(wrapper.find('[data-testid="lottery-nine-grid"]').exists()).toBe(false)
  })

  it('blocks repeated draws while pending and supports prizes seven and eight in ten draws', async () => {
    const status = { ...await mocks.status(), remaining_draws: 10 }
    status.tiers = status.tiers.map((tier: { position: number; probability: number }) => ({ ...tier, probability: 0.125 }))
    mocks.status.mockResolvedValue(status)
    let finishDraw!: (result: unknown) => void
    mocks.draw.mockImplementation(() => new Promise((resolve) => { finishDraw = resolve }))
    const wrapper = mount(UserLotteryModal, {
      props: { quickEntry: true },
      global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } },
    })
    await flushPromises()
    await wrapper.get('[data-testid="lottery-entry"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="lottery-draw-10"]').trigger('click')
    expect(wrapper.get('[data-testid="lottery-nine-grid"]').attributes('aria-busy')).toBe('true')
    expect(wrapper.get('[data-testid="lottery-draw-1"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="lottery-draw-1"]').trigger('click')
    await wrapper.get('[data-testid="lottery-draw-10"]').trigger('click')
    expect(mocks.draw).toHaveBeenCalledTimes(1)
    finishDraw({
      items: Array.from({ length: 10 }, (_, i) => ({ id: i + 1, tier_position: i % 2 ? 7 : 8, reward: i % 2 ? 20 : 50, reward_cents: i % 2 ? 2000 : 5000 })),
      total_reward: 350,
      status: { ...status, remaining_draws: 0 },
    })
    await flushPromises()
    expect(mocks.draw).toHaveBeenCalledWith(10, expect.any(String))
    expect(wrapper.findAll('.batch-chip')).toHaveLength(10)
    expect(wrapper.findAll('.prize-card.is-hit').map((card) => card.attributes('data-tier-position'))).toEqual(['7', '8'])
    expect(wrapper.get('[data-testid="lottery-nine-grid"]').attributes('aria-busy')).toBe('false')
  })

  it('edits all eight active tiers together and rejects any inactive prize', async () => {
    const wrapper = mount(AdminLotteryConfig, { global: { stubs: { ActivityAudience: true } } })
    await flushPromises()
    expect(wrapper.get('[data-testid="lottery-admin-nine-grid"]').element.children).toHaveLength(9)
    expect(wrapper.findAll('input[name^="reward_"]')).toHaveLength(8)
    expect((wrapper.get('input[name="weight_7"]').element as HTMLInputElement).value).toBe('1')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.adminSave).toHaveBeenCalledTimes(1)
    expect(mocks.adminSave.mock.calls[0][0].tiers).toHaveLength(8)
    await wrapper.get('input[name="weight_7"]').setValue(0)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.adminSave).toHaveBeenCalledTimes(1)
    expect(wrapper.get('input[name="weight_7"]').attributes('aria-invalid')).toBe('true')
    await wrapper.get('[data-testid="lottery-use-defaults"]').trigger('click')
    expect((wrapper.get('input[name="weight_7"]').element as HTMLInputElement).value).toBe('1')
    expect((wrapper.get('input[name="reward_8"]').element as HTMLInputElement).value).toBe('50')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.adminSave).toHaveBeenCalledTimes(2)
  })

  it('computes live expected cost & rebate in AdminLotteryConfig and blocks underfunded enable', async () => {
    const wrapper = mount(AdminLotteryConfig, {
      global: {
        stubs: {
          ActivityAudience: true,
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.find('#lottery-total-budget').exists()).toBe(false)

    // $0.629 rounded to 2 decimals -> $0.63 (6.29%)
    expect(wrapper.get('[data-testid="lottery-expected-ev"]').text()).toContain('$0.63')
    expect(wrapper.get('[data-testid="lottery-expected-ev"]').text()).toContain('6.29%')

    // Enable via top-right switch when budget is $30 (< $50 top prize) -> blocked, switch reverts, aria-invalid set
    await wrapper.get('#lottery-budget').setValue('30')
    await wrapper.get('input[role="switch"]').setValue(true)
    await flushPromises()
    expect(mocks.adminSave).not.toHaveBeenCalled()
    expect((wrapper.get('input[role="switch"]').element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.get('#lottery-budget').attributes('aria-invalid')).toBe('true')

    // Fund budget to $200 (>= $50 top prize) and toggle top-right switch ON -> immediately saves integer cents
    await wrapper.get('#lottery-budget').setValue('200')
    await wrapper.get('input[role="switch"]').setValue(true)
    await flushPromises()
    expect(mocks.adminSave).toHaveBeenCalledWith({
      enabled: true,
      unit_pay_cents: 1000,
      budget_cents: 20000,
      tiers: baseTiers.map(({ position, reward_cents, weight }) => ({ position, reward_cents, weight })),
    })
  })
})
