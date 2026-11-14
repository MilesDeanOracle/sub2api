<template>
  <FeedbackDialog
    v-if="feedbackEnabled && auth.isAuthenticated && !auth.isAdmin"
    :key="auth.user?.id"
    :show="store.dialogOpen"
    :initial-tab="store.dialogTab"
    @close="store.dialogOpen = false"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { useAuthStore } from '@/stores/auth'
import { useAnnouncementStore } from '@/stores/announcements'
import { useFeedbackStore } from '@/stores/feedback'
import FeedbackDialog from './FeedbackDialog.vue'

const auth = useAuthStore()
const announcements = useAnnouncementStore()
const store = useFeedbackStore()
const feedbackEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.feedback))
let poll: ReturnType<typeof setInterval> | undefined

watch(() => [auth.user?.id, auth.isAdmin, auth.isAuthenticated, feedbackEnabled.value] as const, ([id, admin, authenticated, enabled]) => {
  store.setIdentity(authenticated && enabled ? id || null : null, admin)
  if (authenticated && enabled) void store.refresh()
}, { immediate: true })
watch(
  () => [auth.isAuthenticated, auth.isAdmin, store.latestUnreadReplyID, store.unreadCount, store.dialogOpen, announcements.currentPopup] as const,
  () => {
    if (feedbackEnabled.value && auth.isAuthenticated && !auth.isAdmin && !announcements.currentPopup && document.visibilityState === 'visible') store.showUnreadDialog()
  },
  { immediate: true }
)
function refreshVisible() {
  if (feedbackEnabled.value && document.visibilityState === 'visible' && auth.isAuthenticated) {
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
