import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getFeedbackSettings, getFeedbackSummary, markFeedbackRead, type FeedbackTicket } from '@/api/feedback'

export const useFeedbackStore = defineStore('feedback', () => {
  const pendingCount = ref(0)
  const unreadCount = ref(0)
  const latestUnreadReplyID = ref(0)
  const dialogOpen = ref(false)
  const dialogTab = ref<'create' | 'mine'>('create')
  const contextNotice = ref<string | null>(null)
  let identity: string | null = null
  let admin = false
  let generation = 0
  let inFlight: Promise<void> | null = null
  let notifiedReplyID = 0

  function setIdentity(userID: number | null, isAdmin = false) {
    const next = userID ? `${userID}:${isAdmin}` : null
    if (next === identity) return
    ++generation
    identity = next
    admin = isAdmin
    inFlight = null
    pendingCount.value = 0
    unreadCount.value = 0
    latestUnreadReplyID.value = 0
    notifiedReplyID = 0
    dialogOpen.value = false
    dialogTab.value = 'create'
    contextNotice.value = null
  }

  async function refresh() {
    if (!identity) return
    if (inFlight) return inFlight
    const version = generation
    const isAdmin = admin
    const task = (async () => {
      try {
        const summary = await getFeedbackSummary(isAdmin)
        if (version !== generation) return
        pendingCount.value = summary.pending_count
        unreadCount.value = summary.unread_count
        latestUnreadReplyID.value = summary.latest_unread_reply_id
      } catch {
        // Polling errors retain the last known counts; the next poll retries.
      }
    })()
    inFlight = task
    try { await task } finally { if (version === generation && inFlight === task) inFlight = null }
  }

  function openDialog(tab: 'create' | 'mine' = unreadCount.value ? 'mine' : 'create') {
    dialogTab.value = tab
    if (tab === 'mine') notifiedReplyID = Math.max(notifiedReplyID, latestUnreadReplyID.value)
    dialogOpen.value = true
  }

  function showUnreadDialog() {
    if (!identity || admin || dialogOpen.value || !unreadCount.value || latestUnreadReplyID.value <= notifiedReplyID) return
    openDialog('mine')
  }

  async function loadSettings() {
    const version = generation
    try {
      const settings = await getFeedbackSettings(admin)
      if (version === generation) contextNotice.value = settings.context_notice
    } catch { /* Retain the last successful value or the localized default. */ }
  }

  async function markRead(ticket: FeedbackTicket) {
    const adminReplies = ticket.replies?.filter(reply => reply.author_role === 'admin') || []
    const replyID = Math.max(0, ...adminReplies.map(reply => reply.id))
    if (!replyID || !adminReplies.some(reply => !reply.read_at)) return
    const version = generation
    await markFeedbackRead(ticket.id, replyID)
    if (version !== generation) return
    // Wait for an older poll before requesting authoritative post-read counts.
    if (inFlight) await inFlight
    if (version === generation) await refresh()
  }

  return { pendingCount, unreadCount, latestUnreadReplyID, dialogOpen, dialogTab, contextNotice, setIdentity, refresh, loadSettings, openDialog, showUnreadDialog, markRead }
})
