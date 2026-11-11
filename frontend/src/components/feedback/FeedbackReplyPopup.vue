<template>
  <FeedbackDialog
    v-if="auth.isAuthenticated && !auth.isAdmin"
    :key="auth.user?.id"
    :show="store.dialogOpen"
    :initial-tab="store.dialogTab"
    @close="store.dialogOpen = false"
  />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useAnnouncementStore } from '@/stores/announcements'
import { useFeedbackStore } from '@/stores/feedback'
import FeedbackDialog from './FeedbackDialog.vue'

const auth = useAuthStore()
const announcements = useAnnouncementStore()
const store = useFeedbackStore()
let poll: ReturnType<typeof setInterval> | undefined

watch(() => [auth.user?.id, auth.isAdmin, auth.isAuthenticated] as const, ([id, admin, authenticated]) => {
  store.setIdentity(authenticated ? id || null : null, admin)
  if (authenticated) void store.refresh()
}, { immediate: true })
watch(
  () => [auth.isAuthenticated, auth.isAdmin, store.latestUnreadReplyID, store.unreadCount, store.dialogOpen, announcements.currentPopup] as const,
  () => {
    if (auth.isAuthenticated && !auth.isAdmin && !announcements.currentPopup && document.visibilityState === 'visible') store.showUnreadDialog()
  },
  { immediate: true }
)
function refreshVisible() {
  if (document.visibilityState === 'visible' && auth.isAuthenticated) {
    if (!announcements.currentPopup) store.showUnreadDialog()
    void store.refresh()
  }
}
onMounted(() => {
  poll = setInterval(refreshVisible, 30000)
  document.addEventListener('visibilitychange', refreshVisible)
})
onBeforeUnmount(() => {
  clearInterval(poll)
  document.removeEventListener('visibilitychange', refreshVisible)
})
</script>
