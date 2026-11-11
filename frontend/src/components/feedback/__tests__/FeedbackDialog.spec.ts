import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import FeedbackDialog from '../FeedbackDialog.vue'
import { getFeedback, listFeedback, submitFeedback, formatFeedbackTokens, getFeedbackSettings, markFeedbackRead, replyToMyFeedback } from '@/api/feedback'
import { useFeedbackStore } from '@/stores/feedback'
import zh from '@/i18n/locales/zh'

vi.mock('@/stores', () => ({ useAppStore: () => ({ showSuccess: vi.fn() }) }))
vi.mock('@/api/feedback', async (original) => ({
  ...await original<typeof import('@/api/feedback')>(),
  submitFeedback: vi.fn(), listFeedback: vi.fn(), getFeedback: vi.fn(), getFeedbackSettings: vi.fn(), markFeedbackRead: vi.fn(), replyToMyFeedback: vi.fn()
}))
const ticket = { id: 23, user_id: 7, title: '测试工单', category: 'usage' as const, content: '问题描述', rating: 4, status: 'replied' as const, reply_count: 1, unread_reply_count: 1, enrichment_status: 'pending' as const, created_at: '2026-11-10T10:00:00Z', updated_at: '2026-11-10T10:10:00Z' }
function render(initialTab: 'create' | 'mine' = 'create') {
  return mount(FeedbackDialog, { props: { show: true, initialTab }, global: {
    plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })],
    stubs: { BaseDialog: { template: '<section><slot /></section>' }, Pagination: true }
  } })
}
function button(wrapper: ReturnType<typeof render>, text: string) { return wrapper.findAll('button').find(b => b.text() === text)! }
async function fill(wrapper: ReturnType<typeof render>) {
  await wrapper.get('input[maxlength="120"]').setValue(' 测试标题 ')
  await wrapper.get('textarea').setValue(' 问题内容 ')
  await wrapper.get('input[type="radio"][value="4"]').setValue()
}
beforeEach(() => {
  vi.resetAllMocks()
  setActivePinia(createPinia())
  vi.mocked(getFeedbackSettings).mockResolvedValue({ context_notice: null })
  vi.mocked(markFeedbackRead).mockResolvedValue()
  vi.mocked(listFeedback).mockResolvedValue({ items: [ticket], total: 1, page: 1, page_size: 10, category_counts: { usage: 1 } })
})
describe('FeedbackDialog', () => {
  it('sends a follow-up once on the same resolved ticket and renders both authors', async () => {
    const adminReply = { id: 1, author_role: 'admin' as const, content: '管理员已处理', created_at: ticket.updated_at, read_at: ticket.updated_at }
    vi.mocked(getFeedback).mockResolvedValue({ ...ticket, status: 'resolved', replies: [adminReply] })
    const wrapper = render('mine')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text().includes('#23'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('继续回复')
    await wrapper.get('textarea').setValue(' 还有一个问题 ')
    let finish!: () => void
    vi.mocked(replyToMyFeedback).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    await wrapper.get('form').trigger('submit')
    await wrapper.get('form').trigger('submit')
    expect(replyToMyFeedback).toHaveBeenCalledTimes(1)
    expect(replyToMyFeedback).toHaveBeenCalledWith(23, '还有一个问题')
    expect(button(wrapper, '返回工单列表').attributes('disabled')).toBeDefined()
    vi.mocked(getFeedback).mockResolvedValue({ ...ticket, status: 'open', replies: [adminReply, { id: 2, author_role: 'user', content: '还有一个问题', created_at: ticket.updated_at }] })
    finish()
    await flushPromises()
    expect(wrapper.text()).toContain('用户回复')
    expect(wrapper.text()).toContain('管理员回复')
    expect(wrapper.text()).toContain('还有一个问题')
    expect(wrapper.text()).toContain('待回复')
    expect(wrapper.get<HTMLTextAreaElement>('textarea').element.value).toBe('')
    wrapper.unmount()
  })

  it('retains a failed reply draft across refresh and hides follow-up before the first admin reply', async () => {
    vi.mocked(getFeedback).mockResolvedValue({ ...ticket, status: 'open', replies: [] })
    const wrapper = render('mine')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text().includes('#23'))!.trigger('click')
    await flushPromises()
    expect(wrapper.find('textarea').exists()).toBe(false)
    vi.mocked(getFeedback).mockResolvedValue({ ...ticket, replies: [{ id: 1, author_role: 'admin', content: '管理员已回复', created_at: ticket.updated_at, read_at: ticket.updated_at }] })
    await button(wrapper, '刷新').trigger('click')
    await flushPromises()
    await wrapper.get('textarea').setValue('保留我的追问')
    vi.mocked(replyToMyFeedback).mockRejectedValue(new Error('offline'))
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('回复发送失败')
    await button(wrapper, '刷新').trigger('click')
    await flushPromises()
    expect(wrapper.get<HTMLTextAreaElement>('textarea').element.value).toBe('保留我的追问')
    wrapper.unmount()
  })

  it('loads My tickets on initial mount without marking the list read', async () => {
    const wrapper = render('mine')
    await flushPromises()
    expect(listFeedback).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(wrapper.text()).toContain('测试工单')
    expect(wrapper.find('form').exists()).toBe(false)
    expect(markFeedbackRead).not.toHaveBeenCalled()
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, initialTab: 'create' })
    expect(wrapper.find('form').exists()).toBe(true)
    wrapper.unmount()
  })
  it('submits the rating once and opens My tickets immediately after persistence', async () => {
    let finish!: (data: { id: number }) => void
    vi.mocked(submitFeedback).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const wrapper = render()
    await fill(wrapper)
    await wrapper.get('form').trigger('submit')
    await wrapper.get('form').trigger('submit')
    expect(submitFeedback).toHaveBeenCalledTimes(1)
    expect(submitFeedback).toHaveBeenCalledWith({ title: '测试标题', content: '问题内容', category: 'usage', rating: 4 })
    finish({ id: 23 })
    await flushPromises()
    expect(listFeedback).toHaveBeenCalled()
    expect(wrapper.text()).toContain('测试工单')
    expect(wrapper.find('form').exists()).toBe(false)
    wrapper.unmount()
  })
  it('keeps the user draft when submission fails', async () => {
    vi.mocked(submitFeedback).mockRejectedValue(new Error('offline'))
    const wrapper = render()
    await fill(wrapper)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get<HTMLTextAreaElement>('textarea').element.value).toBe(' 问题内容 ')
    expect(wrapper.get('[role="alert"]').text()).toContain('提交失败')
    wrapper.unmount()
  })
  it('opens the reply thread from the user ticket list', async () => {
    vi.mocked(getFeedback).mockResolvedValue({ ...ticket, replies: [{ id: 1, author_role: 'admin', content: '管理员已处理您的问题', created_at: ticket.updated_at }] })
    const wrapper = render()
    await button(wrapper, '我的工单').trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text().includes('#23'))!.trigger('click')
    await flushPromises()
    expect(getFeedback).toHaveBeenCalledWith(23)
    expect(wrapper.text()).toContain('管理员已处理您的问题')
    expect(markFeedbackRead).toHaveBeenCalledWith(23, 1)
    wrapper.unmount()
  })
  it('renders the saved notice and puts refresh beside the tabs', async () => {
    vi.mocked(getFeedbackSettings).mockResolvedValue({ context_notice: '自定义提示文案' })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('自定义提示文案')
    await button(wrapper, '我的工单').trigger('click')
    await flushPromises()
    expect(button(wrapper, '刷新').element.parentElement).toBe(button(wrapper, '提交反馈').element.parentElement)
    wrapper.unmount()
  })
  it('hides a deliberately blank notice', async () => {
    vi.mocked(getFeedbackSettings).mockResolvedValue({ context_notice: '' })
    const wrapper = render()
    await flushPromises()
    expect(useFeedbackStore().contextNotice).toBe('')
    expect(wrapper.text()).not.toContain(zh.feedback.contextNotice)
    wrapper.unmount()
  })
  it('uses decimal millions and billions for token usage', () => {
    expect(formatFeedbackTokens(0)).toBe('0.00 M')
    expect(formatFeedbackTokens(1500000)).toBe('1.50 M')
    expect(formatFeedbackTokens(1250000000)).toBe('1.25 B')
  })
})
