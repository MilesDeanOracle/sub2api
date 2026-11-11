<template>
  <BaseDialog :show="show" :title="t('feedback.title')" width="wide" :z-index="110" :close-on-escape="!busy" :show-close-button="!busy" @close="$emit('close')">
    <div class="mb-5 flex flex-wrap items-center gap-2 border-b border-gray-200 pb-3 dark:border-dark-700">
      <button type="button" class="btn" :class="tab === 'create' ? 'btn-primary' : 'btn-ghost'" :disabled="busy" @click="tab = 'create'">{{ t('feedback.newTicket') }}</button>
      <button type="button" class="btn" :class="tab === 'mine' ? 'btn-primary' : 'btn-ghost'" :disabled="busy" @click="tab = 'mine'">{{ t('feedback.myTickets') }}<span v-if="feedbackStore.unreadCount" class="ml-1 rounded-full bg-red-500 px-1.5 text-xs text-white">{{ feedbackStore.unreadCount }}</span></button>
      <button v-if="tab === 'mine'" type="button" class="btn btn-secondary ml-auto" :disabled="loading || replying" @click="refresh">{{ t('common.refresh') }}</button>
    </div>
    <form v-if="tab === 'create'" class="space-y-5" @submit.prevent="submit">
      <label class="block"><span class="mb-1.5 block text-sm font-medium">{{ t('feedback.ticketTitle') }}</span><input v-model="form.title" class="input" maxlength="120" required :disabled="submitting" :placeholder="t('feedback.titlePlaceholder')"></label>
      <label class="block"><span class="mb-1.5 block text-sm font-medium">{{ t('feedback.category') }}</span>
        <select v-model="form.category" class="input" :disabled="submitting"><option v-for="category in feedbackCategories" :key="category" :value="category">{{ t(`feedback.categories.${category}`) }}</option></select>
      </label>
      <label class="block"><span class="mb-1.5 block text-sm font-medium">{{ t('feedback.content') }}</span><textarea v-model="form.content" class="input min-h-36" rows="6" maxlength="10000" required :disabled="submitting" :placeholder="t('feedback.contentPlaceholder')" /></label>
      <div><p class="mb-2 text-sm font-medium">{{ t('feedback.rating') }}</p><StarRating v-model="form.rating" :disabled="submitting" /></div>
      <p v-if="contextNotice" class="whitespace-pre-wrap text-xs text-gray-500 dark:text-dark-400">{{ contextNotice }}</p>
      <p v-if="submitError" role="alert" class="text-sm text-red-600">{{ submitError }}</p>
      <div class="flex justify-end"><button class="btn btn-primary" type="submit" :disabled="submitting || !form.title.trim() || !form.content.trim() || !form.rating">{{ submitting ? t('feedback.submitting') : t('feedback.submit') }}</button></div>
    </form>
    <div v-else class="space-y-4">
      <div v-if="selected" class="flex justify-between gap-2">
        <button v-if="selected" class="btn btn-ghost" :disabled="replying" @click="backToList">{{ t('feedback.back') }}</button>
      </div>
      <p v-if="loadError" role="alert" class="text-sm text-red-600">{{ loadError }}</p>
      <p v-if="loading" role="status" class="py-8 text-center text-gray-500">{{ t('common.loading') }}</p>
      <template v-else-if="selected">
        <FeedbackThread :ticket="selected" />
        <form v-if="canReply" class="space-y-3 border-t border-gray-200 pt-5 dark:border-dark-700" @submit.prevent="sendReply">
          <label class="block"><span class="mb-2 block font-medium">{{ t('feedback.continueReply') }}</span><textarea v-model="replyContent" class="input" rows="4" maxlength="10000" required :disabled="replying" :placeholder="t('feedback.userReplyPlaceholder')" /></label>
          <p v-if="replyError" role="alert" class="text-sm text-red-600">{{ replyError }}</p>
          <div class="flex justify-end"><button type="submit" class="btn btn-primary" :disabled="replying || !replyContent.trim()">{{ replying ? t('feedback.sending') : t('feedback.sendReply') }}</button></div>
        </form>
      </template>
      <template v-else>
        <p v-if="!tickets.length && !loadError" class="py-10 text-center text-gray-500">{{ t('feedback.empty') }}</p>
        <button v-for="ticket in tickets" :key="ticket.id" class="flex w-full items-center justify-between gap-3 rounded-xl border border-gray-200 p-4 text-left transition-colors hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800" @click="openTicket(ticket.id)">
          <span class="min-w-0"><span class="block break-words font-medium">#{{ ticket.id }} · {{ ticket.title }}</span><span class="mt-1 block text-xs text-gray-500">{{ formatDateTime(ticket.created_at) }} · {{ t('feedback.replyCount', { count: ticket.reply_count }) }}</span></span>
          <span class="flex shrink-0 flex-col items-end gap-1"><span class="badge badge-primary">{{ t(`feedback.statuses.${ticket.status}`) }}</span><span v-if="ticket.unread_reply_count" class="text-xs font-medium text-red-500">{{ t('feedback.unreadReplies', { count: ticket.unread_reply_count }) }}</span></span>
        </button>
        <Pagination v-if="total > 0" :page="page" :page-size="10" :total="total" :show-page-size-selector="false" @update:page="changePage" />
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useFeedbackStore } from '@/stores/feedback'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import StarRating from './StarRating.vue'
import FeedbackThread from './FeedbackThread.vue'
import { feedbackCategories, submitFeedback, listFeedback, getFeedback, replyToMyFeedback, type FeedbackInput, type FeedbackTicket } from '@/api/feedback'
import { formatDateTime } from '@/utils/format'

const props = withDefaults(defineProps<{ show: boolean; initialTab?: 'create' | 'mine' }>(), { initialTab: 'create' })
defineEmits<{ close: [] }>()
const { t } = useI18n()
const app = useAppStore()
const feedbackStore = useFeedbackStore()
const contextNotice = computed(() => feedbackStore.contextNotice ?? t('feedback.contextNotice'))
const tab = ref<'create' | 'mine'>(props.initialTab)
const form = reactive<FeedbackInput>({ title: '', category: 'usage', content: '', rating: 0 })
const submitting = ref(false)
const submitError = ref('')
const loadError = ref('')
const loading = ref(false)
const tickets = ref<FeedbackTicket[]>([])
const selected = ref<FeedbackTicket | null>(null)
const replying = ref(false)
const busy = computed(() => submitting.value || replying.value)
const replyError = ref('')
const replyDrafts = reactive<Record<number, string>>({})
const replyContent = computed({
  get: () => selected.value ? replyDrafts[selected.value.id] || '' : '',
  set: value => { if (selected.value) replyDrafts[selected.value.id] = value }
})
const canReply = computed(() => selected.value?.replies?.some(reply => reply.author_role === 'admin'))
const total = ref(0)
const page = ref(1)
let requestVersion = 0

async function submit() {
  if (submitting.value) return
  submitting.value = true
  submitError.value = ''
  try {
    await submitFeedback({ ...form, title: form.title.trim(), content: form.content.trim() })
    Object.assign(form, { title: '', category: 'usage', content: '', rating: 0 })
    app.showSuccess(t('feedback.submitted'))
    page.value = 1
    selected.value = null
    tab.value = 'mine'
    void feedbackStore.refresh()
  } catch { submitError.value = t('feedback.submitFailed') }
  finally { submitting.value = false }
}
async function refresh() {
  void feedbackStore.refresh()
  const version = ++requestVersion
  const id = selected.value?.id
  loading.value = true
  loadError.value = ''
  try {
    if (id) {
      const ticket = await getFeedback(id)
      if (version === requestVersion) { selected.value = ticket; void markVisibleRead(ticket, version) }
    } else {
      const result = await listFeedback({ page: page.value, page_size: 10 })
      if (version === requestVersion) { tickets.value = result.items; total.value = result.total }
    }
  } catch { if (version === requestVersion) loadError.value = t('feedback.loadFailed') }
  finally { if (version === requestVersion) loading.value = false }
}
async function sendReply() {
  const id = selected.value?.id
  const content = replyContent.value.trim()
  if (!id || replying.value || !canReply.value || !content) return
  replying.value = true
  replyError.value = ''
  try {
    await replyToMyFeedback(id, content)
    delete replyDrafts[id]
    app.showSuccess(t('feedback.userReplySent'))
    await refresh()
  } catch { replyError.value = t('feedback.replyFailed') }
  finally { replying.value = false }
}
async function openTicket(id: number) {
  const version = ++requestVersion
  loading.value = true
  loadError.value = ''
  try { const ticket = await getFeedback(id); if (version === requestVersion) { selected.value = ticket; void markVisibleRead(ticket, version) } }
  catch { if (version === requestVersion) loadError.value = t('feedback.loadFailed') }
  finally { if (version === requestVersion) loading.value = false }
}
async function markVisibleRead(ticket: FeedbackTicket, version: number) {
  await nextTick()
  if (version !== requestVersion || !props.show || tab.value !== 'mine' || selected.value?.id !== ticket.id) return
  try {
    await feedbackStore.markRead(ticket)
    if (version === requestVersion) {
      ticket.unread_reply_count = 0
      const item = tickets.value.find(item => item.id === ticket.id)
      if (item) item.unread_reply_count = 0
    }
  } catch { if (version === requestVersion) loadError.value = t('feedback.readFailed') }
}
function changePage(value: number) { page.value = value; void refresh() }
function backToList() {
  ++requestVersion
  selected.value = null
  loading.value = false
  void refresh()
}
watch(() => props.show, show => {
  if (show) {
    tab.value = props.initialTab
    selected.value = null
    page.value = 1
    void feedbackStore.loadSettings()
  }
}, { immediate: true })
watch(() => selected.value?.id, () => { replyError.value = '' })
watch([() => props.show, tab], ([show, currentTab]) => {
  if (show && currentTab === 'mine') void refresh()
  else { ++requestVersion; loading.value = false }
}, { immediate: true })
</script>
