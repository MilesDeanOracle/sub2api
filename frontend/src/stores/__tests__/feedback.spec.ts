import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useFeedbackStore } from '../feedback'
import { getFeedbackSummary, markFeedbackRead, type FeedbackSummary, type FeedbackTicket } from '@/api/feedback'

vi.mock('@/api/feedback', () => ({ getFeedbackSummary: vi.fn(), markFeedbackRead: vi.fn(), getFeedbackSettings: vi.fn() }))
const makeTicket = (): FeedbackTicket => ({
  id: 23, user_id: 7, title: '反馈', content: '问题', category: 'usage', rating: 5, status: 'replied',
  enrichment_status: 'ready', reply_count: 1, unread_reply_count: 1, created_at: '2026-11-10T10:00:00Z', updated_at: '2026-11-10T10:00:00Z',
  replies: [{ id: 42, author_role: 'admin', content: '管理员回复', created_at: '2026-11-10T10:00:00Z', read_at: null }]
})
beforeEach(() => {
  vi.resetAllMocks()
  setActivePinia(createPinia())
  vi.mocked(getFeedbackSummary).mockResolvedValue({ pending_count: 0, unread_count: 1, latest_unread_reply_id: 42 })
  vi.mocked(markFeedbackRead).mockResolvedValue()
})

describe('feedback notifications', () => {
  it('marks only administrator messages read when a user follow-up is the last message', async () => {
    const store = useFeedbackStore()
    store.setIdentity(7)
    const ticket = makeTicket()
    ticket.replies!.push({ id: 43, author_role: 'user', content: '继续追问', created_at: ticket.updated_at })
    await store.markRead(ticket)
    expect(markFeedbackRead).toHaveBeenCalledWith(23, 42)
    vi.mocked(markFeedbackRead).mockClear()
    ticket.replies![0].read_at = ticket.updated_at
    await store.markRead(ticket)
    expect(markFeedbackRead).not.toHaveBeenCalled()
  })
  it('does not open a dialog when no replies are unread', async () => {
    vi.mocked(getFeedbackSummary).mockResolvedValue({ pending_count: 0, unread_count: 0, latest_unread_reply_id: 0 })
    const store = useFeedbackStore()
    store.setIdentity(7)
    await store.refresh()
    store.showUnreadDialog()
    expect(store.dialogOpen).toBe(false)
  })

  it('counts pending admin tickets without opening the user dialog', async () => {
    vi.mocked(getFeedbackSummary).mockResolvedValue({ pending_count: 4, unread_count: 0, latest_unread_reply_id: 0 })
    const store = useFeedbackStore()
    store.setIdentity(7, true)
    await store.refresh()
    store.showUnreadDialog()
    expect(getFeedbackSummary).toHaveBeenCalledWith(true)
    expect(store.pendingCount).toBe(4)
    expect(store.dialogOpen).toBe(false)
  })

  it('opens My tickets without marking replies read and does not repeat on every poll', async () => {
    const store = useFeedbackStore()
    store.setIdentity(7)
    await store.refresh()
    store.showUnreadDialog()
    expect(store.dialogOpen).toBe(true)
    expect(store.dialogTab).toBe('mine')
    expect(markFeedbackRead).not.toHaveBeenCalled()
    store.dialogOpen = false
    await store.refresh()
    store.showUnreadDialog()
    expect(store.dialogOpen).toBe(false)
    expect(store.unreadCount).toBe(1)
  })

  it('notifies again for a new reply on the same unread ticket', async () => {
    const store = useFeedbackStore()
    store.setIdentity(7)
    await store.refresh()
    store.showUnreadDialog()
    store.dialogOpen = false
    vi.mocked(getFeedbackSummary).mockResolvedValue({ pending_count: 0, unread_count: 1, latest_unread_reply_id: 43 })
    await store.refresh()
    store.showUnreadDialog()
    expect(store.dialogOpen).toBe(true)
  })

  it('preserves the current form and shows a pending notification after closing it', async () => {
    const store = useFeedbackStore()
    store.setIdentity(7)
    store.openDialog('create')
    await store.refresh()
    store.showUnreadDialog()
    expect(store.dialogTab).toBe('create')
    store.dialogOpen = false
    store.showUnreadDialog()
    expect(store.dialogTab).toBe('mine')
    expect(store.dialogOpen).toBe(true)
  })

  it('acknowledges only displayed replies and preserves newer unread replies', async () => {
    const store = useFeedbackStore()
    store.setIdentity(7)
    await store.refresh()
    vi.mocked(getFeedbackSummary).mockResolvedValue({ pending_count: 0, unread_count: 1, latest_unread_reply_id: 43 })
    await store.markRead(makeTicket())
    expect(markFeedbackRead).toHaveBeenCalledWith(23, 42)
    expect(store.latestUnreadReplyID).toBe(43)
    expect(store.unreadCount).toBe(1)
  })

  it('retains unread state when marking a reply read fails', async () => {
    const store = useFeedbackStore()
    store.setIdentity(7)
    await store.refresh()
    vi.mocked(markFeedbackRead).mockRejectedValue(new Error('offline'))
    await expect(store.markRead(makeTicket())).rejects.toThrow('offline')
    expect(store.unreadCount).toBe(1)
  })

  it('discards late responses and notification deduplication on account switching', async () => {
    const store = useFeedbackStore()
    store.setIdentity(7)
    await store.refresh()
    store.showUnreadDialog()
    let finish!: (value: FeedbackSummary) => void
    vi.mocked(getFeedbackSummary).mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const pending = store.refresh()
    store.setIdentity(8)
    finish({ pending_count: 0, unread_count: 1, latest_unread_reply_id: 100 })
    await pending
    expect(store.dialogOpen).toBe(false)
    expect(store.unreadCount).toBe(0)
    await store.refresh()
    store.showUnreadDialog()
    expect(store.dialogOpen).toBe(true)
  })
})
