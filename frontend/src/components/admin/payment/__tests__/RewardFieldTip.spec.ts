import {afterEach,describe,expect,it,vi} from 'vitest'
import {enableAutoUnmount,mount,flushPromises} from '@vue/test-utils'
import {ref} from 'vue'
import Tip from '../RewardFieldTip.vue'
vi.mock('vue-i18n',()=>({useI18n:()=>({locale:ref('zh')})}))
enableAutoUnmount(afterEach)
afterEach(()=>vi.restoreAllMocks())
function rect(left:number,top:number,width:number,height:number){return {left,top,width,height,right:left+width,bottom:top+height,x:left,y:top,toJSON:()=>({})}}
describe('reward help visibility',()=>{
 it.each([[5,36,12,72],[980,700,612,624]])('keeps help inside the viewport for an anchor at (%s,%s)',async(left,top,expectedLeft,expectedTop)=>{
  vi.spyOn(HTMLElement.prototype,'getBoundingClientRect').mockImplementation(function(this:HTMLElement){return this.tagName==='BUTTON'?rect(left,top,28,28):rect(0,0,400,68)})
  const wrapper=mount(Tip,{props:{label:'奖励',text:'完整说明'},attachTo:document.body})
  await wrapper.get('button').trigger('mouseenter');await wrapper.get('button').trigger('focus');await flushPromises()
  const tooltip=document.querySelector<HTMLElement>('[role=tooltip]')!
  // Default jsdom viewport is 1024 × 768. The left-edge case stays at 12px.
  expect(tooltip.style.left).toBe(`${expectedLeft}px`)
  expect(tooltip.style.top).toBe(`${expectedTop}px`)
  expect(wrapper.get('button').attributes('aria-expanded')).toBe('true')
  document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape'}));await flushPromises()
  expect(document.querySelector('[role=tooltip]')).toBeNull()
 })
 it('dismisses on outside pointer interaction without submitting the form',async()=>{
  const wrapper=mount(Tip,{props:{label:'预算',text:'预算说明'},attachTo:document.body})
  expect(wrapper.get('button').attributes('type')).toBe('button')
  await wrapper.get('button').trigger('click');await flushPromises()
  document.body.dispatchEvent(new Event('pointerdown',{bubbles:true}));await flushPromises()
  expect(wrapper.get('button').attributes('aria-expanded')).toBe('false')
 })
})
