import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import FeedbackSettingsDialog from '../FeedbackSettingsDialog.vue'
import { getFeedbackSettings, updateFeedbackSettings } from '@/api/feedback'
import { useFeedbackStore } from '@/stores/feedback'
import zh from '@/i18n/locales/zh'

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))
vi.mock('@/api/feedback', () => ({ getFeedbackSettings: vi.fn(), updateFeedbackSettings: vi.fn() }))
function render() {
  return mount(FeedbackSettingsDialog, { props: { show: true }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })], stubs: { BaseDialog: { template: '<section><slot /></section>' } } } })
}
beforeEach(() => {
  vi.resetAllMocks()
  setActivePinia(createPinia())
  vi.mocked(getFeedbackSettings).mockResolvedValue({ context_notice: null })
  vi.mocked(updateFeedbackSettings).mockResolvedValue()
})
describe('feedback settings', () => {
  it('loads the default notice and persists custom plain text', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get<HTMLTextAreaElement>('textarea').element.value).toBe(zh.feedback.contextNotice)
    await wrapper.get('textarea').setValue('  欢迎告诉我们您的想法。  ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(updateFeedbackSettings).toHaveBeenCalledWith({ context_notice: '欢迎告诉我们您的想法。' })
    expect(useFeedbackStore().contextNotice).toBe('欢迎告诉我们您的想法。')
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })
  it('allows intentionally hiding the notice with blank text', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('textarea').setValue('')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(updateFeedbackSettings).toHaveBeenCalledWith({ context_notice: '' })
    wrapper.unmount()
  })
  it('keeps settings open and retains edits on save failure', async () => {
    vi.mocked(updateFeedbackSettings).mockRejectedValue(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('textarea').setValue('新的文案')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get<HTMLTextAreaElement>('textarea').element.value).toBe('新的文案')
    expect(wrapper.get('[role="alert"]').text()).toContain('保存失败')
    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })
})
