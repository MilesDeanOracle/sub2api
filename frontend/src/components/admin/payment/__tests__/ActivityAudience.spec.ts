import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {enableAutoUnmount,flushPromises,mount} from '@vue/test-utils'
import {ref} from 'vue'
import ActivityAudience from '../ActivityAudience.vue'
const mocks=vi.hoisted(()=>({get:vi.fn(),save:vi.fn()}))
vi.mock('vue-i18n',()=>({useI18n:()=>({locale:ref('zh')})}))
vi.mock('@/api/admin/activityAudience',()=>({activityAudienceAPI:mocks}))
enableAutoUnmount(afterEach)
beforeEach(()=>{vi.resetAllMocks();mocks.get.mockResolvedValue({mode:'selected',user_ids:[]});mocks.save.mockResolvedValue(undefined)})
describe('activity audience controls',()=>{
 it('loads restricted scope and saves deduplicated real IDs only on explicit save',async()=>{
  const w=mount(ActivityAudience,{props:{kind:'checkin'}});await flushPromises()
  expect(w.text()).toContain('指定用户测试中')
  await w.get('textarea').setValue('12，35\n12 108')
  expect(mocks.save).not.toHaveBeenCalled()
  await w.get('footer button').trigger('click');await flushPromises()
  expect(mocks.save).toHaveBeenCalledWith('checkin',{mode:'selected',user_ids:[12,35,108]})
 })
 it('does not publish on radio selection and retains the list when switching to all',async()=>{
  mocks.get.mockResolvedValue({mode:'selected',user_ids:[12]})
  const w=mount(ActivityAudience,{props:{kind:'consumption'}});await flushPromises()
  await w.get('input[value=all]').setValue(true)
  expect(mocks.save).not.toHaveBeenCalled();expect(w.get('.audience-badge').text()).toContain('指定用户')
  await w.get('footer button').trigger('click');await flushPromises()
  expect(mocks.save).toHaveBeenCalledWith('consumption',{mode:'all',user_ids:[12]})
  expect(w.get('.audience-badge').text()).toContain('全量')
 })
 it('blocks invalid IDs with a linked validation error',async()=>{
  const w=mount(ActivityAudience,{props:{kind:'checkin'}});await flushPromises()
  await w.get('textarea').setValue('1,hello,-3')
  await w.get('footer button').trigger('click');await flushPromises()
  expect(mocks.save).not.toHaveBeenCalled();expect(w.get('textarea').attributes('aria-invalid')).toBe('true');expect(w.get('[role=alert]').text()).toContain('正整数')
 })
 it('fails closed on load error and retains edits on save failure',async()=>{
  mocks.get.mockRejectedValueOnce(new Error('offline'))
  const w=mount(ActivityAudience,{props:{kind:'checkin'}});await flushPromises()
  expect(w.get('fieldset').attributes('disabled')).toBeDefined()
  await w.get('[role=alert] button').trigger('click');await flushPromises()
  mocks.save.mockRejectedValueOnce(new Error('offline'))
  await w.get('textarea').setValue('123');await w.get('footer button').trigger('click');await flushPromises()
  expect((w.get('textarea').element as HTMLTextAreaElement).value).toBe('123');expect(w.find('[role=alert]').exists()).toBe(true)
 })
})
