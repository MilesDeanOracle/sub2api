import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import ActivityWalletPill from '../ActivityWalletPill.vue'

const mocks = vi.hoisted(() => ({
  getWallet: vi.fn(),
  withdraw: vi.fn(),
  refreshUser: vi.fn(),
  push: vi.fn(),
  route: {path:'/keys'}
}))

vi.mock('@/api/activity', () => ({
  getActivityWallet: mocks.getWallet,
  withdrawActivityWallet: mocks.withdraw
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ refreshUser: mocks.refreshUser })
}))
vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({ push: mocks.push })
}))

enableAutoUnmount(afterEach)
beforeEach(() => {vi.resetAllMocks();mocks.route.path='/keys'})

const mountPill = () =>
  mount(ActivityWalletPill, {
    global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] }
  })

describe('ActivityWalletPill', () => {
  it('stays hidden when user is outside activity audience and has zero gift balance', async () => {
    mocks.getWallet.mockResolvedValue({
      visible: false,
      available_cents: 0,
      available: 0,
      reserved_cents: 0,
      reserved: 0,
      total_gift_cents: 0,
      total_gift: 0,
      total_earned_cents: 0,
      total_earned: 0,
      total_withdrawn_cents: 0,
      total_withdrawn: 0,
      unabsorbed_gift: '0',
      withdraw_blocked: false
    })
    const wrapper = mountPill()
    await flushPromises()
    expect(wrapper.find('[data-testid=header-gift-pill]').exists()).toBe(false)
  })

  it('shows $0.00 pill when audience is visible and withdraws available gift into account balance on click', async () => {
    mocks.getWallet.mockResolvedValue({
      visible: true,
      available_cents: 269,
      available: 2.69,
      reserved_cents: 0,
      reserved: 0,
      total_gift_cents: 269,
      total_gift: 2.69,
      total_earned_cents: 269,
      total_earned: 2.69,
      total_withdrawn_cents: 0,
      total_withdrawn: 0,
      unabsorbed_gift: '0',
      withdraw_blocked: false
    })
    const wrapper = mountPill()
    await flushPromises()
    expect(wrapper.get('[data-testid=header-gift-pill]').text()).toContain('$2.69')

    await wrapper.get('[data-testid=header-gift-pill]').trigger('click')
    expect(wrapper.get('[data-testid=header-gift-popover]').text()).toContain('可转入余额')
    expect(wrapper.get('[data-testid=header-gift-popover]').text()).toContain('待领取奖励')
    expect(wrapper.get('[data-testid=header-gift-popover]').text()).toContain('当前奖励合计')

    mocks.withdraw.mockResolvedValueOnce({
      visible: true,
      available_cents: 0,
      available: 0,
      reserved_cents: 0,
      reserved: 0,
      total_gift_cents: 0,
      total_gift: 0,
      total_earned_cents: 269,
      total_earned: 2.69,
      total_withdrawn_cents: 269,
      total_withdrawn: 2.69,
      unabsorbed_gift: '2.69',
      withdraw_blocked: false,
      withdrawn: 2.69,
      withdrawn_cents: 269
    })
    await wrapper.get('[data-testid=header-wallet-withdraw]').trigger('click')
    await flushPromises()
    expect(mocks.withdraw).toHaveBeenCalledTimes(1)
    expect(mocks.refreshUser).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid=header-gift-pill]').text()).toContain('$0.00')
  })

  const wallet = (extra = {}) => ({visible:true,available_cents:250,available:2.5,reserved_cents:0,reserved:0,total_earned:2.5,total_withdrawn:0,total_earned_cents:250,total_withdrawn_cents:0,unabsorbed_gift:'0',withdraw_blocked:false,...extra})

  it('labels the wallet, explains its purpose and shows the exact transfer amount without opening on hover', async () => {
    mocks.getWallet.mockResolvedValue(wallet())
    const wrapper=mountPill();await flushPromises()
    expect(wrapper.get('[data-testid=header-gift-pill]').text()).toContain('奖励钱包')
    await wrapper.get('[data-testid=header-gift-pill]').trigger('mouseenter')
    expect(wrapper.get('[data-testid=header-gift-pill]').attributes('aria-expanded')).toBe('false')
    await wrapper.get('[data-testid=header-gift-pill]').trigger('click');await flushPromises()
    expect(wrapper.get('[data-testid=header-wallet-withdraw]').text()).toBe('全部转入余额 $2.50')
    expect(wrapper.text()).toContain('不支持提取现金')
    expect(wrapper.text()).toContain('签到、抽奖和消费奖励统一存入此钱包')
    await wrapper.trigger('keydown.esc')
    expect(wrapper.get('[data-testid=header-gift-pill]').attributes('aria-expanded')).toBe('false')
  })

  it('explains an empty wallet and keeps reserved rewards separate from transferable funds', async () => {
    mocks.getWallet.mockResolvedValue(wallet({available_cents:0,available:0}))
    const wrapper=mountPill();await flushPromises()
    expect(wrapper.find('[data-testid=header-wallet-withdraw]').exists()).toBe(false)
    expect(wrapper.text()).toContain('暂无可转入奖励')
    mocks.getWallet.mockResolvedValue(wallet({available_cents:0,available:0,reserved_cents:500,reserved:5}))
    window.dispatchEvent(new CustomEvent('activity-wallet-updated'));await flushPromises()
    expect(wrapper.text()).toContain('待领取奖励需先在消费活动中领取')
    await wrapper.get('button:has(+ p)').trigger('click')
    expect(mocks.push).toHaveBeenCalledWith('/dashboard#consumption-rewards')
    expect(mocks.withdraw).not.toHaveBeenCalled()
  })

  it('keeps the claim action available on the dashboard and requests focus on the reward card', async () => {
    mocks.route.path='/dashboard';mocks.getWallet.mockResolvedValue(wallet({reserved_cents:500,reserved:5}))
    const wrapper=mountPill();await flushPromises()
    const onClaim=vi.fn();window.addEventListener('activity-claim-requested',onClaim)
    try {
      const button=wrapper.findAll('button').find(b=>b.text().includes('去领取消费奖励'))!
      await button.trigger('click');expect(onClaim).toHaveBeenCalledTimes(1)
      expect(mocks.push).not.toHaveBeenCalled()
    } finally {window.removeEventListener('activity-claim-requested',onClaim)}
  })

  it.each([
    ['awards_paused','奖励转入暂时暂停'],['refund_review','退款或奖励正在复核'],['recovery_debt','有待扣回奖励'],['unknown','奖励暂不能转入']
  ])('explains blocked transfers: %s',async (reason,text) => {
    mocks.getWallet.mockResolvedValue(wallet({withdraw_blocked:true,withdraw_block_reason:reason}))
    const wrapper=mountPill();await flushPromises()
    expect(wrapper.get('[data-testid=header-wallet-withdraw]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain(text)
    await wrapper.get('[data-testid=header-wallet-withdraw]').trigger('click');expect(mocks.withdraw).not.toHaveBeenCalled()
  })

  it('retains the amount after a failed transfer and prevents duplicate requests while retrying',async () => {
    mocks.getWallet.mockResolvedValue(wallet())
    mocks.withdraw.mockRejectedValueOnce({response:{data:{message:'网络暂时不可用'}}})
    const wrapper=mountPill();await flushPromises()
    await wrapper.get('[data-testid=header-wallet-withdraw]').trigger('click');await flushPromises()
    expect(wrapper.get('[role=alert]').text()).toContain('网络暂时不可用')
    expect(wrapper.get('[data-testid=header-wallet-withdraw]').text()).toContain('$2.50')
    let resolve!:(value:unknown)=>void
    mocks.withdraw.mockImplementationOnce(()=>new Promise(done=>{resolve=done}))
    const button=wrapper.get('[data-testid=header-wallet-withdraw]')
    await button.trigger('click');await button.trigger('click')
    expect(button.text()).toBe('正在转入…');expect(mocks.withdraw).toHaveBeenCalledTimes(2)
    mocks.refreshUser.mockRejectedValueOnce(new Error('profile refresh offline'))
    resolve(wallet({available_cents:0,available:0,withdrawn:2.5}));await flushPromises()
    expect(wrapper.find('[role=alert]').exists()).toBe(false)
    expect(wrapper.get('[role=status]').text()).toContain('$2.50 已转入账户余额')
  })

  it('ignores a refresh started while a transfer was pending',async()=>{
    mocks.getWallet.mockResolvedValue(wallet())
    const wrapper=mountPill();await flushPromises()
    let transfer!:(value:unknown)=>void;let refresh!:(value:unknown)=>void
    mocks.withdraw.mockImplementationOnce(()=>new Promise(done=>{transfer=done}))
    await wrapper.get('[data-testid=header-wallet-withdraw]').trigger('click')
    mocks.getWallet.mockImplementationOnce(()=>new Promise(done=>{refresh=done}))
    window.dispatchEvent(new CustomEvent('activity-wallet-updated'))
    transfer(wallet({available_cents:0,available:0,withdrawn:2.5}));await flushPromises()
    refresh(wallet());await flushPromises()
    expect(wrapper.get('[data-testid=header-gift-pill]').text()).toContain('$0.00')
    expect(wrapper.find('[data-testid=header-wallet-withdraw]').exists()).toBe(false)
  })

  it('shows recent rewards and transfers together without requiring another page',async () => {
    mocks.getWallet.mockResolvedValue(wallet({ledger:[{id:2,direction:'withdraw',kind:'manual_withdraw',amount_cents:250,created_at:'2026-11-26T01:00:00Z'},{id:1,direction:'credit',kind:'lottery',amount_cents:250,created_at:'2026-11-26T00:59:00Z'}]}))
    const wrapper=mountPill();await flushPromises()
    expect(wrapper.get('summary').text()).toContain('最近 20 笔')
    const rows=wrapper.findAll('li')
    expect(rows[0]!.text()).toContain('转入账户余额');expect(rows[0]!.text()).toContain('−$2.50')
    expect(rows[1]!.text()).toContain('九宫格抽奖');expect(rows[1]!.text()).toContain('+$2.50')
  })
})
