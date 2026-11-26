import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import en from '@/i18n/locales/en'
import AdminRewardMoments from '../AdminRewardMoments.vue'
const mocks = vi.hoisted(() => ({ config: vi.fn(), saveConfig: vi.fn() }))
vi.mock('@/api/activity', () => ({ rewardMomentsAPI: mocks }))
enableAutoUnmount(afterEach)
beforeEach(() => vi.resetAllMocks())
const options = (locale = 'zh') => ({ global: { plugins: [createI18n({ legacy: false, locale, messages: { zh, en } })] } })
describe('reward moments admin controls', () => {
  it('renders the English settings and validation through the locale resources', async () => {
    mocks.config.mockResolvedValue({ enabled: false, user_ids: [] })
    const wrapper = mount(AdminRewardMoments, options('en')); await flushPromises()
    expect(wrapper.get('h2').text()).toBe('Anonymous reward feed')
    await wrapper.get('textarea').setValue('invalid'); await wrapper.get('form').trigger('submit')
    expect(wrapper.get('[role=alert]').text()).toBe('Enter up to 500 positive integer user IDs.')
    expect(mocks.saveConfig).not.toHaveBeenCalled()
  })
  it('loads the independent off switch and validates viewer IDs before saving', async () => {
    mocks.config.mockResolvedValue({ enabled: false, user_ids: [] })
    const wrapper = mount(AdminRewardMoments, options()); await flushPromises()
    expect(wrapper.get('input').element).toHaveProperty('checked', false)
    await wrapper.get('textarea').setValue('someone@example.com'); await wrapper.get('form').trigger('submit')
    expect(mocks.saveConfig).not.toHaveBeenCalled()
    expect(wrapper.get('textarea').attributes('aria-invalid')).toBe('true')
    await wrapper.get('textarea').setValue('2, 3, 2')
    await wrapper.get('input').setValue(true)
    mocks.saveConfig.mockResolvedValue({ enabled: true, user_ids: [2, 3] })
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(mocks.saveConfig).toHaveBeenCalledWith({ enabled: true, user_ids: [2, 3] })
    expect(wrapper.get('[role=status]').text()).toContain('已保存')
  })
  it('prevents overwrite after a failed config load and provides a retry', async () => {
    mocks.config.mockRejectedValue(new Error('offline'))
    const wrapper = mount(AdminRewardMoments, options()); await flushPromises()
    expect(wrapper.find('form').exists()).toBe(false)
    mocks.config.mockResolvedValue({ enabled: false, user_ids: [] })
    await wrapper.get('[role=alert] button').trigger('click'); await flushPromises()
    expect(wrapper.find('form').exists()).toBe(true)
  })
})
