import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {enableAutoUnmount,flushPromises,mount} from '@vue/test-utils'
import {createI18n} from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import Checkin from '../UserDashboardActivity.vue'
const mocks=vi.hoisted(()=>({status:vi.fn(),checkin:vi.fn(),wallet:vi.fn(),withdraw:vi.fn()}))
vi.mock('@/api/activity',()=>({getActivityStatus:mocks.status,checkin:mocks.checkin,getActivityWallet:mocks.wallet,withdrawActivityWallet:mocks.withdraw}))
vi.mock('vue-router',()=>({useRouter:()=>({push:vi.fn()})}))
vi.mock('@/stores/auth',()=>({useAuthStore:()=>({refreshUser:vi.fn()})}))
vi.mock('@/api/auth',()=>({authAPI:{getPublicSettings:vi.fn()}}))
vi.mock('@/api/userChannels',()=>({userChannelsAPI:{getAvailable:vi.fn()}}))
enableAutoUnmount(afterEach)
beforeEach(()=>vi.resetAllMocks())
afterEach(()=>vi.useRealTimers())
const status=()=>({server_day:'2026-11-30',checkin_visible:true,checkin_enabled:true,checkin_eligible:true,checked_today:false,eligible:true,checkin_min_spend:0,next_reward_cents:5,daily_reward:0.25,milestone_reward:1,streak:1,next_milestone:7,month_cap:15,month_reward:1,checked_days:[],wallet_balance:0})
const options=()=>({props:{quickEntry:true},global:{plugins:[createI18n({legacy:false,locale:'zh',messages:{zh}})],stubs:{BaseDialog:{props:['show'],template:'<div v-if="show"><slot /></div>'}}}})
describe('checkin availability and calendar',()=>{
 it('shows the full wallet transfer amount and explains the actual restriction',async()=>{
  mocks.status.mockResolvedValue({...status(),wallet_balance_cents:269,wallet_balance:2.69,withdraw_blocked:true,withdraw_block_reason:'recovery_debt'})
  const wrapper=mount(Checkin,options());await flushPromises()
  await wrapper.get('[data-testid=checkin-entry]').trigger('click');await flushPromises()
  expect(wrapper.get('[data-testid=checkin-withdraw]').text()).toContain('$2.69')
  expect(wrapper.get('[data-testid=checkin-withdraw]').attributes('disabled')).toBeDefined()
  expect(wrapper.text()).toContain('有待扣回奖励')
  expect(wrapper.text()).toContain('不支持提取现金')
 })
 it('displays the actual constrained payout and the refund review reason',async()=>{
  mocks.status.mockResolvedValue(status());const wrapper=mount(Checkin,options());await flushPromises()
  await wrapper.get('[data-testid=checkin-entry]').trigger('click');await flushPromises()
  expect(wrapper.text()).toContain('本次预计到账 $0.05')
  mocks.status.mockResolvedValue({...status(),checkin_eligible:false,checkin_block_reason:'refund_review'})
  window.dispatchEvent(new Event('focus'));await flushPromises()
  expect(wrapper.get('.checkin-action').text()).toContain('复核')
  expect(wrapper.get('.checkin-action').attributes('disabled')).toBeDefined()
 })
 it('refreshes a checked calendar into the next Beijing day and month',async()=>{
  vi.useFakeTimers({toFake:['Date','setInterval','clearInterval']});vi.setSystemTime(new Date('2026-11-30T15:59:50Z'))
  mocks.status.mockImplementation(async()=>Date.now()<Date.parse('2026-11-30T16:00:00Z')?{...status(),checked_today:true,checked_days:['2026-11-30']}:{...status(),server_day:'2026-12-01',checked_days:[],month_reward:0})
  const wrapper=mount(Checkin,options());await flushPromises()
  await wrapper.get('[data-testid=checkin-entry]').trigger('click');await flushPromises()
  expect(wrapper.get('.is-today .checkin-day-number').text()).toBe('30')
  expect(wrapper.get('.checkin-action').attributes('disabled')).toBeDefined()
  await vi.advanceTimersByTimeAsync(30000);await flushPromises()
  expect(wrapper.get('.is-today .checkin-day-number').text()).toBe('1')
  expect(wrapper.get('.checkin-action').attributes('disabled')).toBeUndefined()
 })
})
