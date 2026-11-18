import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {enableAutoUnmount,flushPromises,mount} from '@vue/test-utils'
import {ref} from 'vue'
import Checkin from '../AdminActivityConfig.vue'
import Consumption from '../AdminConsumptionCampaigns.vue'
const mocks=vi.hoisted(()=>({config:vi.fn(),save:vi.fn(),reviews:vi.fn(),campaigns:vi.fn(),saveCampaign:vi.fn(),state:vi.fn()}))
vi.mock('vue-i18n',()=>({useI18n:()=>({t:(key:string)=>key,locale:ref('zh')})}))
vi.mock('@/api/admin/activity',()=>({getAdminActivityConfig:mocks.config,updateAdminActivityConfig:mocks.save,getActivityRefundReviews:mocks.reviews,markActivityRefundReviewed:vi.fn()}))
vi.mock('@/api/consumption',()=>({consumptionAPI:{campaigns:mocks.campaigns,save:mocks.saveCampaign,state:mocks.state}}))
enableAutoUnmount(afterEach)
const config={checkin_min_spend_cents:0,checkin_enabled:false,checkin_rollout_percent:100,checkin_daily_cents:10,checkin_milestone_cents:50,checkin_monthly_cap_cents:1500,checkin_monthly_budget_cents:30000,total_budget_cents:100000,user_lifetime_cap_cents:3000,total_spent_cents:0,checkin_month_spent_cents:0,second_recharge_enabled:false,second_recharge_reward_cents:0,second_recharge_total_budget_cents:0,second_recharge_min_pay_cents:0,second_total_spent_cents:0}
beforeEach(()=>{vi.resetAllMocks();mocks.config.mockResolvedValue({...config});mocks.reviews.mockResolvedValue([]);mocks.save.mockImplementation(async value=>value);mocks.campaigns.mockResolvedValue([])})
const options={global:{stubs:{RouterLink:{template:'<a><slot /></a>'}}}}
describe('production marketing reward forms',()=>{
 it('changes the switch locally and saves dollars as integer API cents, retaining a zero threshold',async()=>{
  const wrapper=mount(Checkin,options);await flushPromises()
  await wrapper.get('input[role=switch]').setValue(true)
  await wrapper.get('#activity-checkin_daily_cents').setValue('0.29')
  expect(mocks.save).not.toHaveBeenCalled()
  expect(wrapper.findAll('.day-stop').at(-1)?.text()).toContain('$0.79')
  await wrapper.get('form').trigger('submit');await flushPromises()
  expect(mocks.save).toHaveBeenCalledWith(expect.objectContaining({checkin_enabled:true,checkin_daily_cents:29,checkin_min_spend_cents:0}))
 })
 it('blocks invalid enabled configurations and links the field error',async()=>{
  const wrapper=mount(Checkin,options);await flushPromises()
  await wrapper.get('input[role=switch]').setValue(true)
  await wrapper.get('#activity-checkin_monthly_cap_cents').setValue('16')
  await wrapper.get('form').trigger('submit');await flushPromises()
  expect(mocks.save).not.toHaveBeenCalled()
  expect(wrapper.get('#activity-checkin_monthly_cap_cents').attributes('aria-invalid')).toBe('true')
  expect(wrapper.find('#activity-checkin_monthly_cap_cents-error').exists()).toBe(true)
 })
 it('retains entered values after save failure',async()=>{
  mocks.save.mockRejectedValue(new Error('Offline'))
  const wrapper=mount(Checkin,options);await flushPromises()
  await wrapper.get('#activity-checkin_daily_cents').setValue('0.25')
  await wrapper.get('form').trigger('submit');await flushPromises()
  expect((wrapper.get('#activity-checkin_daily_cents').element as HTMLInputElement).value).toBe('0.25')
  expect(wrapper.get('[role=status]').exists()).toBe(true)
 })
 it('locks published dates and tiers while leaving the budget editable',async()=>{
  mocks.campaigns.mockResolvedValue([{id:1,name:'Campaign',starts_at:'2026-11-20T00:00:00Z',ends_at:'2026-12-20T00:00:00Z',claim_until:'2026-12-27T00:00:00Z',published_at:'2026-11-18T00:00:00Z',state:'active',rollout_percent:100,budget_cents:30000,reserved_cents:100,claimed_cents:0,participants:2,qualified_users:1,tiers:[{position:1,threshold_cents:1000,reward_cents:30},{position:2,threshold_cents:5000,reward_cents:120},{position:3,threshold_cents:10000,reward_cents:250}]}])
  const wrapper=mount(Consumption,options);await flushPromises()
  expect(wrapper.get('input[name=starts_at]').attributes('disabled')).toBeDefined()
  expect(wrapper.get('input[name=threshold_1]').attributes('disabled')).toBeDefined()
  expect(wrapper.get('input[name=coverage]').attributes('disabled')).toBeDefined()
  expect(wrapper.get('#campaign-budget').attributes('disabled')).toBeUndefined()
  expect(wrapper.get('.total-reward').text()).toContain('$4.00')
  expect(wrapper.get('.campaign-tab').text()).toContain('Campaign')
  expect(wrapper.get('.campaign-tab').attributes('aria-current')).toBe('true')
  expect(wrapper.find('.campaign-selector').exists()).toBe(false)
 })
 it('shows draft editing without publishing automatically',async()=>{
  const wrapper=mount(Consumption,options);await flushPromises()
  await wrapper.get('input[name=reward_1]').setValue('0.50')
  expect(wrapper.get('.total-reward').text()).toContain('$4.20')
  expect(mocks.state).not.toHaveBeenCalled()
  expect(wrapper.find('.campaign-tabs').exists()).toBe(false)
  expect(wrapper.get('.campaign-empty-state').text()).toContain('草稿')
  expect(wrapper.get('input[name=threshold_1]').attributes('disabled')).toBeUndefined()
 })
})
