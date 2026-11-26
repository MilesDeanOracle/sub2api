import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {enableAutoUnmount,flushPromises,mount} from '@vue/test-utils'
import {createI18n} from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import Rewards from '../ConsumptionRewards.vue'
const mocks=vi.hoisted(()=>({status:vi.fn(),claim:vi.fn(),withdraw:vi.fn(),refreshUser:vi.fn()}))
vi.mock('@/api/consumption',()=>({consumptionAPI:mocks}))
vi.mock('@/stores/auth',()=>({useAuthStore:()=>({refreshUser:mocks.refreshUser})}))
enableAutoUnmount(afterEach)
beforeEach(()=>vi.resetAllMocks())
function summary(spend='30.5', availableCents=0){
 return {
  awards_enabled:true,processing_pending:false,spend,blocked:false,budget_blocked:false,claimable_cents:0,history:[],
  wallet:{available_cents:availableCents,available:availableCents/100,total_earned_cents:availableCents,total_earned:availableCents/100,total_withdrawn_cents:0,total_withdrawn:0,unabsorbed_gift:'0',withdraw_blocked:false},
  campaign:{id:1,name:'本月消费奖励',state:'active',starts_at:'2026-01-01T00:00:00Z',ends_at:'2099-01-01T00:00:00Z',claim_until:'2099-01-08T00:00:00Z',tiers:[{position:1,threshold_cents:2000,reward_cents:300,state:'claimed'},{position:2,threshold_cents:5000,reward_cents:500,state:'locked'},{position:3,threshold_cents:10000,reward_cents:700,state:'locked'}]}
 }
}
const options=()=>({global:{plugins:[createI18n({legacy:false,locale:'zh',messages:{zh}})]}})
describe('dashboard reward progress',()=>{
 it('reads configured milestones, exact spend and the next reward without inventing screenshot amounts',async()=>{
  mocks.status.mockResolvedValue(summary());const wrapper=mount(Rewards,options());await flushPromises()
  expect(wrapper.get('.reward-strip-spent').text()).toContain('$30.50')
  expect(wrapper.get('.reward-strip-actions').text()).toContain('$19.50')
  expect(wrapper.findAll('.reward-milestone')).toHaveLength(3)
  expect(wrapper.findAll('.is-reached')).toHaveLength(1)
  expect(wrapper.get('progress').attributes('max')).toBe('100')
  await wrapper.get('.reward-rules-toggle').trigger('click')
  expect(wrapper.get('.reward-rules').text()).toContain('本月消费奖励')
 })
 it('does not visually mark just-below-threshold spend as qualified',async()=>{
  const value=summary('49.99999999');value.campaign.tiers[0]!.state='locked';mocks.status.mockResolvedValue(value)
  const wrapper=mount(Rewards,options());await flushPromises()
  expect(wrapper.findAll('.is-reached')).toHaveLength(1)
  expect(wrapper.get('.reward-strip-spent').text()).toContain('$49.99')
  expect(wrapper.get('.reward-strip-actions').text()).toContain('$0.01')
 })
 it('claims into the isolated activity wallet first and only refreshes account balance on manual withdraw',async()=>{
  const value=summary('50',0);value.campaign.tiers[1]!.state='reserved';value.claimable_cents=500
  mocks.status.mockResolvedValue(value)
  let resolve!:(value:unknown)=>void;mocks.claim.mockImplementation(()=>new Promise(done=>{resolve=done}))
  const wrapper=mount(Rewards,options());await flushPromises()
  const claim=wrapper.get('.reward-milestone .reward-claim');await claim.trigger('click');await claim.trigger('click')
  expect(mocks.claim).toHaveBeenCalledTimes(1);expect(mocks.claim).toHaveBeenCalledWith(1,2)
  resolve(summary('50',500));await flushPromises()
  expect(mocks.refreshUser).not.toHaveBeenCalled()
  expect(wrapper.get('.reward-notice').text()).toContain('点击“全部转入余额”')
  expect(wrapper.get('[data-testid=consumption-withdraw]').text()).toContain('$5.00')
  mocks.withdraw.mockResolvedValueOnce({available_cents:0,available:0,withdrawn_cents:500,withdrawn:5})
  mocks.status.mockResolvedValueOnce(summary('50',0))
  await wrapper.get('[data-testid=consumption-withdraw]').trigger('click');await flushPromises()
  expect(mocks.withdraw).toHaveBeenCalledTimes(1)
  expect(mocks.refreshUser).toHaveBeenCalledTimes(1)
 })
 it('shows settlement delay and hides claims while payouts are stopped',async()=>{
  const value=summary('50');value.processing_pending=true;value.awards_enabled=false;value.campaign.tiers[1]!.state='reserved';value.claimable_cents=500
  mocks.status.mockResolvedValue(value);const wrapper=mount(Rewards,options());await flushPromises()
  expect(wrapper.get('.reward-strip-notice').text()).toContain('暂停')
  expect(wrapper.find('.reward-claim').exists()).toBe(false)
 })
 it('shows the actual claim error and prioritizes a budget shortage over generic processing',async()=>{
  const value=summary('50');value.campaign.tiers[1]!.state='reserved';value.claimable_cents=500;value.budget_blocked=true;value.processing_pending=true
  mocks.status.mockResolvedValue(value);mocks.claim.mockRejectedValue({response:{data:{message:'此档奖励正在复核'}}})
  const wrapper=mount(Rewards,options());await flushPromises()
  expect(wrapper.get('.reward-strip-notice').text()).toContain('预算')
  await wrapper.get('.reward-milestone .reward-claim').trigger('click');await flushPromises()
  expect(wrapper.get('[role=alert]').text()).toContain('此档奖励正在复核')
 })
 it('keeps first milestone progress visible and refreshes a campaign selected during a request',async()=>{
  const first={...summary('10'),campaigns:[{id:1,name:'本月消费奖励'},{id:2,name:'新活动'}]}
  mocks.status.mockResolvedValueOnce(first)
  const wrapper=mount(Rewards,options());await flushPromises()
  expect(wrapper.get('.reward-first-progress').text()).toContain('50%')
  let resolve!:(value:unknown)=>void
  mocks.status.mockImplementationOnce(()=>new Promise(done=>{resolve=done}))
  window.dispatchEvent(new Event('focus'))
  await wrapper.get('select').setValue('2')
  mocks.status.mockResolvedValueOnce({...first,campaign:{...first.campaign,id:2,name:'新活动'}})
  resolve(first);await flushPromises()
  expect(mocks.status).toHaveBeenLastCalledWith(2)
  expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('2')
 })
 it('hides unavailable campaigns when wallet is empty and exposes a retry if loading fails',async()=>{
  mocks.status.mockResolvedValue({...summary(),campaign:null});const wrapper=mount(Rewards,options());await flushPromises();expect(wrapper.find('section').exists()).toBe(false)
  wrapper.unmount();mocks.status.mockRejectedValueOnce(new Error('offline'))
  const failed=mount(Rewards,options());await flushPromises();expect(failed.get('[role=alert]').text()).toContain('重试')
 })
 it('explains the master pause while preserving claims for reserved rewards',async()=>{
  const value={...summary('50'),enabled:false}
  value.campaign.tiers[1]!.state='reserved';value.claimable_cents=500
  mocks.status.mockResolvedValue(value)
  const wrapper=mount(Rewards,options());await flushPromises()
  expect(wrapper.get('.reward-strip-notice').text()).toContain('关闭期间的消费不累计')
  expect(wrapper.find('.reward-milestone .reward-claim').exists()).toBe(true)
 })

 it('explains why rewards cannot transfer and retains success if the profile refresh fails',async()=>{
  const value={...summary('50',500),wallet:{...summary('50',500).wallet,withdraw_blocked:true,withdraw_block_reason:'refund_review'}}
  mocks.status.mockResolvedValue(value)
  const wrapper=mount(Rewards,options());await flushPromises()
  expect(wrapper.text()).toContain('退款或奖励正在复核，完成后可转入余额')
  expect(wrapper.get('[data-testid=consumption-withdraw]').attributes('disabled')).toBeDefined()
  mocks.status.mockResolvedValue(summary('50',500));window.dispatchEvent(new CustomEvent('activity-wallet-updated'));await flushPromises()
  mocks.withdraw.mockResolvedValue({...summary('50',0).wallet,withdrawn:5})
  mocks.status.mockResolvedValue(summary('50',0));mocks.refreshUser.mockRejectedValue(new Error('offline'))
  await wrapper.get('[data-testid=consumption-withdraw]').trigger('click');await flushPromises()
  expect(wrapper.get('.reward-notice').text()).toContain('$5.00 已转入账户余额')
  expect(wrapper.find('[role=alert]').exists()).toBe(false)
 })

 it('focuses the reward card from the wallet shortcut and resets an explicit campaign selection',async()=>{
  mocks.status.mockResolvedValue({...summary(),campaigns:[{id:1,name:'本月消费奖励'},{id:2,name:'其他活动'}]})
  const wrapper=mount(Rewards,{...options(),attachTo:document.body});await flushPromises()
  const scroll=vi.fn();Object.defineProperty(wrapper.element,'scrollIntoView',{value:scroll})
  await wrapper.get('select').setValue('2');await flushPromises()
  window.dispatchEvent(new CustomEvent('activity-claim-requested'));await flushPromises()
  expect(mocks.status).toHaveBeenLastCalledWith(undefined)
  expect(scroll).toHaveBeenCalled();expect(document.activeElement).toBe(wrapper.element)
 })

 it('does not let a stale refresh overwrite a newly claimed reward wallet',async()=>{
  const value=summary('50');value.campaign.tiers[1]!.state='reserved';value.claimable_cents=500
  mocks.status.mockResolvedValueOnce(value)
  const wrapper=mount(Rewards,options());await flushPromises()
  let resolve!:(value:unknown)=>void;mocks.status.mockImplementationOnce(()=>new Promise(done=>{resolve=done}))
  let finishClaim!:(value:unknown)=>void;mocks.claim.mockImplementationOnce(()=>new Promise(done=>{finishClaim=done}))
  await wrapper.get('.reward-milestone .reward-claim').trigger('click')
  window.dispatchEvent(new Event('focus'))
  finishClaim(summary('50',500));await flushPromises()
  resolve(value);await flushPromises()
  expect(wrapper.get('[data-testid=consumption-withdraw]').text()).toContain('$5.00')
 })

})
