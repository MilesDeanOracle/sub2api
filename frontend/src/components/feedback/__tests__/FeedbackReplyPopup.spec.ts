import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import FeedbackReplyPopup from '../FeedbackReplyPopup.vue'
import { useFeedbackStore } from '@/stores/feedback'
import { getFeedbackSummary } from '@/api/feedback'

const mocks = vi.hoisted(() => ({ auth: { user: { id: 7 }, isAuthenticated: true, isAdmin: false }, announcements: { currentPopup: null as unknown } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => reactive(mocks.auth) }))
vi.mock('@/stores/announcements', () => ({ useAnnouncementStore: () => reactive(mocks.announcements) }))
vi.mock('@/api/feedback', () => ({ getFeedbackSummary: vi.fn(), getFeedbackSettings: vi.fn(), markFeedbackRead: vi.fn() }))
beforeEach(() => {
  vi.resetAllMocks()
  setActivePinia(createPinia())
  mocks.auth.isAuthenticated = true
  mocks.auth.isAdmin = false
  mocks.announcements.currentPopup = null
  vi.mocked(getFeedbackSummary).mockResolvedValue({ unread_count: 1, pending_count: 0, latest_unread_reply_id: 42 })
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
})
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

function render() {
  return mount(FeedbackReplyPopup, { global: { stubs: {
    FeedbackDialog: { props: ['show', 'initialTab'], template: '<section v-if="show" :data-tab="initialTab"><button @click="$emit(\'close\')">close</button></section>' }
  } } })
}

describe('reply popup visibility', () => {
  it('opens My tickets immediately on login and leaves it closed after dismissal', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('section').attributes('data-tab')).toBe('mine')
    await wrapper.get('button').trigger('click')
    await useFeedbackStore().refresh()
    await flushPromises()
    expect(wrapper.find('section').exists()).toBe(false)
    wrapper.unmount()
  })

  it('waits for announcements and the manual feedback form to close', async () => {
    mocks.announcements.currentPopup = { id: 1 }
    const wrapper = render()
    await flushPromises()
    const store = useFeedbackStore()
    expect(store.dialogOpen).toBe(false)
    store.openDialog('create')
    reactive(mocks.announcements).currentPopup = null
    await flushPromises()
    expect(wrapper.get('section').attributes('data-tab')).toBe('create')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.get('section').attributes('data-tab')).toBe('mine')
    wrapper.unmount()
  })

  it('polls for new replies and stops polling on unmount', async () => {
    vi.useFakeTimers()
    vi.mocked(getFeedbackSummary).mockResolvedValue({ unread_count: 0, pending_count: 0, latest_unread_reply_id: 0 })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('section').exists()).toBe(false)
    vi.mocked(getFeedbackSummary).mockResolvedValue({ unread_count: 1, pending_count: 0, latest_unread_reply_id: 43 })
    await vi.advanceTimersByTimeAsync(30000)
    expect(wrapper.get('section').attributes('data-tab')).toBe('mine')
    wrapper.unmount()
    const requests = vi.mocked(getFeedbackSummary).mock.calls.length
    await vi.advanceTimersByTimeAsync(30000)
    expect(getFeedbackSummary).toHaveBeenCalledTimes(requests)
  })

  it('defers hidden-tab notifications until the user returns', async () => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('section').exists()).toBe(false)
    visibility.mockReturnValue('visible')
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(wrapper.get('section').attributes('data-tab')).toBe('mine')
    wrapper.unmount()
  })

  it('does not show user tickets for admins', async () => {
    mocks.auth.isAdmin = true
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('section').exists()).toBe(false)
    wrapper.unmount()
  })
})
