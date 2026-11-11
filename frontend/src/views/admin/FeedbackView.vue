<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="flex flex-wrap items-center gap-2">
        <button v-for="category in ['', ...adminFeedbackCategories]" :key="category" type="button" class="btn btn-sm" :class="filters.category === category ? 'btn-primary' : 'btn-secondary'" @click="setCategory(category)">
          {{ category ? t(`feedback.categories.${category}`) : t('feedback.allCategories') }}
          <span class="rounded-full bg-black/10 px-1.5 text-[11px] leading-5 dark:bg-white/10">{{ category ? counts[category as FeedbackCategory] || 0 : allCount }}</span>
        </button>
        <div class="ml-auto flex flex-wrap items-center gap-2">
          <select v-model="filters.status" class="input w-36 py-1.5" :aria-label="t('feedback.status')" @change="resetPage">
            <option value="">{{ t('feedback.allStatuses') }}</option>
            <option v-for="status in ['open', 'replied', 'resolved']" :key="status" :value="status">{{ t(`feedback.statuses.${status}`) }}</option>
          </select>
          <button type="button" class="btn btn-secondary btn-sm" @click="settingsOpen = true"><Icon name="cog" size="sm" />{{ t('feedback.settings') }}</button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="refresh">{{ t('common.refresh') }}</button>
        </div>
      </div>
      <p v-if="listError" role="alert" class="text-sm text-red-600">{{ listError }}</p>
      <TablePageLayout>
        <template #table>
          <DataTable :columns="columns" :data="tickets" :loading="loading">
            <template #cell-title="{ row }"><div class="max-w-72 whitespace-normal break-words font-medium text-gray-900 dark:text-white">#{{ row.id }} · {{ row.title }}</div><div class="mt-1"><span class="badge badge-gray">{{ t(`feedback.categories.${row.category}`) }}</span></div></template>
            <template #cell-user="{ row }"><div class="font-medium">{{ displayName(row) }}</div><div class="text-xs text-gray-500 dark:text-dark-400">{{ row.snapshot?.email || t(`feedback.enrichment.${row.enrichment_status}`) }}</div></template>
            <template #cell-usage="{ row }"><template v-if="row.snapshot"><div class="tabular-nums">${{ row.snapshot.total_recharged.toFixed(2) }}</div><div class="text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ formatFeedbackTokens(row.snapshot.total_tokens) }}</div></template><span v-else>—</span></template>
            <template #cell-rating="{ row }"><span v-if="row.rating > 0" class="whitespace-nowrap text-amber-500" :aria-label="t('feedback.stars', { count: row.rating })">{{ '★'.repeat(row.rating) }}{{ '☆'.repeat(5 - row.rating) }}</span><span v-else class="text-xs text-gray-400 dark:text-dark-500">{{ t('feedback.pelicanNoRating') }}</span></template>
            <template #cell-status="{ row }"><span class="badge" :class="statusBadge(row.status)">{{ t(`feedback.statuses.${row.status}`) }}</span></template>
            <template #cell-created_at="{ row }"><span class="whitespace-nowrap text-gray-500 dark:text-dark-400">{{ formatDateTime(row.created_at) }}</span></template>
            <template #cell-actions="{ row }"><button type="button" class="btn btn-secondary btn-sm whitespace-nowrap" :disabled="replying || detailLoading" @click="openTicket(row.id)">{{ t('feedback.viewReply') }} ({{ row.reply_count }})</button></template>
          </DataTable>
        </template>
        <template #pagination><Pagination v-if="total > 0" :page="filters.page" :page-size="filters.page_size" :total="total" @update:page="changePage" @update:page-size="changeSize" /></template>
      </TablePageLayout>
      <FeedbackSettingsDialog :show="settingsOpen" @close="settingsOpen = false" />
      <BaseDialog :show="detailOpen" :title="t('feedback.ticketDetails')" width="full" :close-on-escape="!replying" :show-close-button="!replying" @close="closeDetail">
        <p v-if="detailError" role="alert" class="mb-4 rounded-xl bg-red-50 px-4 py-2 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">{{ detailError }}</p>
        <p v-if="detailLoading" role="status" class="py-16 text-center text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</p>
        <div v-else-if="selected" class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_21rem]">
          <section class="min-w-0">
            <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="break-words text-lg font-semibold text-gray-900 dark:text-white">#{{ selected.id }} · {{ selected.title }}</h3>
                <div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-gray-500 dark:text-dark-400">
                  <span class="badge badge-gray">{{ t(`feedback.categories.${selected.category}`) }}</span>
                  <span v-if="selected.rating > 0" class="text-amber-500" :aria-label="t('feedback.stars', { count: selected.rating })">{{ '★'.repeat(selected.rating) }}{{ '☆'.repeat(5 - selected.rating) }}</span>
                  <time>{{ formatDateTime(selected.created_at) }}</time>
                </div>
              </div>
              <span class="badge" :class="statusBadge(selected.status)">{{ t(`feedback.statuses.${selected.status}`) }}</span>
            </div>
            <div ref="threadRef" class="space-y-4">
              <p class="whitespace-pre-wrap break-words rounded-2xl border border-gray-200 bg-gray-50 p-4 text-sm leading-relaxed text-gray-800 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200">{{ selected.content }}</p>
              <div v-if="selected.pelican?.has_source" class="space-y-2">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="pelicanSourceLoading" @click="togglePelicanSource">
                  <Icon name="play" size="xs" />
                  {{ pelicanSourceOpen ? t('feedback.pelicanHideSource') : (pelicanSourceLoading ? t('common.loading') : t('feedback.pelicanViewSource')) }}
                </button>
                <p v-if="pelicanSourceError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ pelicanSourceError }}</p>
                <template v-if="pelicanSourceOpen && pelicanSource">
                  <p v-if="pelicanSource.truncated || selected.pelican.source_truncated" class="text-xs text-amber-600 dark:text-amber-400">{{ t('feedback.pelicanSourceTruncated') }}</p>
                  <PelicanArtifactPreview :html="pelicanSourceHtml" :title="selected.pelican.model" />
                  <details class="rounded-xl border border-gray-200 dark:border-dark-700">
                    <summary class="cursor-pointer px-3 py-2 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('feedback.pelicanSource') }}</summary>
                    <pre class="max-h-96 overflow-auto rounded-b-xl bg-gray-950 p-4 text-xs leading-relaxed text-gray-100">{{ pelicanSource.source }}</pre>
                  </details>
                </template>
              </div>
              <div class="flex items-center gap-3 text-xs text-gray-400 dark:text-dark-500"><span class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></span><span>{{ t('feedback.replies') }}</span><span class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></span></div>
              <p v-if="!selected.replies?.length" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('feedback.noReplies') }}</p>
              <ol v-else class="space-y-3">
                <li v-for="reply in selected.replies" :key="reply.id" class="flex" :class="reply.author_role === 'admin' ? 'justify-end' : 'justify-start'">
                  <div class="max-w-[85%] rounded-2xl px-4 py-3 text-sm" :class="reply.author_role === 'admin' ? 'rounded-tr-sm bg-primary-50 text-gray-800 dark:bg-primary-900/20 dark:text-gray-100' : 'rounded-tl-sm bg-gray-100 text-gray-800 dark:bg-dark-700 dark:text-gray-100'">
                    <div class="mb-1 flex items-center justify-between gap-4 text-xs" :class="reply.author_role === 'admin' ? 'text-primary-700 dark:text-primary-300' : 'text-gray-500 dark:text-gray-400'">
                      <span class="font-medium">{{ t(reply.author_role === 'admin' ? 'feedback.adminReply' : 'feedback.userReply') }}</span>
                      <time>{{ formatDateTime(reply.created_at) }}</time>
                    </div>
                    <p class="whitespace-pre-wrap break-words leading-relaxed">{{ reply.content }}</p>
                  </div>
                </li>
              </ol>
            </div>
          </section>
          <aside class="min-w-0 space-y-4">
            <div v-if="selected.pelican" class="rounded-2xl border border-sky-200 bg-sky-50/40 p-4 dark:border-sky-900/50 dark:bg-sky-900/10">
              <h4 class="mb-3 flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white"><PelicanLogo :size="18" />{{ t('feedback.pelicanInfo') }}</h4>
              <dl class="space-y-2 text-sm">
                <div v-for="item in pelicanFields" :key="item.label" class="flex items-start justify-between gap-3">
                  <dt class="shrink-0 text-xs text-gray-500 dark:text-dark-400">{{ item.label }}</dt>
                  <dd class="min-w-0 break-all text-right" :class="item.mono ? 'font-mono text-xs' : ''">{{ item.value }}</dd>
                </div>
              </dl>
              <div v-if="selected.pelican.note" class="mt-3 rounded-xl bg-white p-3 text-sm dark:bg-dark-800">
                <div class="mb-1 text-xs text-gray-500 dark:text-dark-400">{{ t('feedback.pelicanNote') }}</div>
                <p class="whitespace-pre-wrap break-words">{{ selected.pelican.note }}</p>
              </div>
            </div>
            <div class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
              <h4 class="mb-3 text-sm font-medium text-gray-900 dark:text-white">{{ t('feedback.userContext') }}</h4>
              <template v-if="selected.snapshot">
                <div class="mb-4 flex items-center gap-3">
                  <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary-100 font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-400">{{ displayName(selected).charAt(0).toUpperCase() }}</span>
                  <div class="min-w-0"><div class="truncate font-medium text-gray-900 dark:text-white">{{ displayName(selected) }}</div><div class="truncate text-xs text-gray-500 dark:text-dark-400">{{ selected.snapshot.email }}</div></div>
                </div>
                <div class="grid grid-cols-2 gap-3">
                  <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800"><div class="text-xs text-gray-500 dark:text-dark-400">{{ t('feedback.totalRecharged') }}</div><div class="mt-1 font-semibold tabular-nums text-gray-900 dark:text-white">${{ selected.snapshot.total_recharged.toFixed(2) }}</div></div>
                  <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-800"><div class="text-xs text-gray-500 dark:text-dark-400">{{ t('feedback.tokens7d') }}</div><div class="mt-1 font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatFeedbackTokens(selected.snapshot.total_tokens) }}</div></div>
                </div>
                <p class="mt-3 text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(selected.snapshot.window_start) }} — {{ formatDateTime(selected.snapshot.window_end) }}</p>
              </template>
              <p v-else class="text-sm text-gray-500 dark:text-dark-400">{{ t(`feedback.enrichment.${selected.enrichment_status}`) }}</p>
            </div>
            <div v-if="selected.snapshot" class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
              <div class="mb-2 flex items-center justify-between text-xs font-medium text-gray-500 dark:text-dark-400"><span>{{ t('feedback.model') }} / {{ t('feedback.group') }}</span><span>{{ t('feedback.tokens') }}</span></div>
              <ul class="max-h-64 divide-y divide-gray-100 overflow-y-auto text-sm dark:divide-dark-700">
                <li v-for="(usage, index) in selected.snapshot.usage" :key="index" class="flex items-start justify-between gap-3 py-2">
                  <div class="min-w-0"><div class="break-all text-gray-900 dark:text-gray-100">{{ usage.model }}</div><div class="text-xs text-gray-500 dark:text-dark-400">{{ usage.group_name || t('feedback.noGroup') }}</div></div>
                  <span class="shrink-0 tabular-nums">{{ formatFeedbackTokens(usage.tokens) }}</span>
                </li>
                <li v-if="!selected.snapshot.usage.length" class="py-4 text-center text-gray-500 dark:text-dark-400">{{ t('feedback.noUsage') }}</li>
              </ul>
              <p class="mt-3 text-xs text-gray-400 dark:text-dark-500">{{ t('feedback.tokenUnits') }}</p>
            </div>
          </aside>
        </div>
        <template v-if="selected && !detailLoading" #footer>
          <form class="w-full space-y-3" @submit.prevent="sendReply">
            <textarea v-model="replyContent" class="input" rows="3" maxlength="10000" required :disabled="replying" :aria-label="t('feedback.writeReply')" :placeholder="t('feedback.replyPlaceholder')" />
            <p v-if="replyError" role="alert" class="text-sm text-red-600">{{ replyError }}</p>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300"><input v-model="resolved" type="checkbox" :disabled="replying">{{ t('feedback.markResolved') }}</label>
              <button type="submit" class="btn btn-primary" :disabled="replying || !replyContent.trim()">{{ replying ? t('feedback.sending') : t('feedback.sendReply') }}</button>
            </div>
          </form>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import FeedbackSettingsDialog from '@/components/feedback/FeedbackSettingsDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PelicanArtifactPreview from '@/components/pelican/PelicanArtifactPreview.vue'
import PelicanLogo from '@/components/pelican/PelicanLogo.vue'
import { extractArtifact } from '@/lib/pelican/artifact'
import { useFeedbackStore } from '@/stores/feedback'
import { adminFeedbackCategories, formatFeedbackTokens, listFeedback, getFeedback, getPelicanReportSource, replyFeedback, type FeedbackCategory, type FeedbackStatus, type FeedbackTicket } from '@/api/feedback'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const app = useAppStore()
const filters = reactive({ page: 1, page_size: 20, category: '', status: '' })
const tickets = ref<FeedbackTicket[]>([])
const total = ref(0)
const counts = ref<Partial<Record<FeedbackCategory, number>>>({})
const allCount = computed(() => Object.values(counts.value).reduce((sum, count) => sum + (count || 0), 0))
const loading = ref(false)
const listError = ref('')
const selected = ref<FeedbackTicket | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const detailOpen = ref(false)
const settingsOpen = ref(false)
const feedbackStore = useFeedbackStore()
const replyContent = ref('')
const resolved = ref(false)
const replying = ref(false)
const replyError = ref('')
const threadRef = ref<HTMLElement | null>(null)
const pelicanSource = ref<{ source: string; truncated: boolean } | null>(null)
const pelicanSourceOpen = ref(false)
const pelicanSourceLoading = ref(false)
const pelicanSourceError = ref('')
// The stored source may be raw model output; extract the HTML document like the pelican page does.
const pelicanSourceHtml = computed(() => {
  const source = pelicanSource.value?.source ?? ''
  return extractArtifact(source).html ?? source
})
const pelicanFields = computed(() => {
  const report = selected.value?.pelican
  if (!report) return []
  const usage = report.usage
  const ms = (value: number | null | undefined) => value == null ? '—' : value < 1000 ? `${Math.round(value)} ms` : `${(value / 1000).toFixed(1)} s`
  const account = `${usage.account_name || `#${usage.account_id}`}${usage.account_platform ? ` · ${usage.account_platform}` : ''}${usage.account_type ? ` / ${usage.account_type}` : ''}`
  const group = usage.group_id == null ? t('feedback.noGroup') : `${usage.group_name || `#${usage.group_id}`}${usage.group_platform ? ` · ${usage.group_platform}` : ''}`
  const upstream = usage.upstream_model && usage.upstream_model !== (usage.requested_model || usage.model) ? usage.upstream_model : ''
  return [
    { label: t('feedback.pelicanRunId'), value: report.run_id, mono: true },
    { label: t('feedback.pelicanRequestId'), value: report.request_id, mono: true },
    { label: t('feedback.pelicanModel'), value: usage.requested_model || report.model, mono: true },
    ...(upstream ? [{ label: t('feedback.pelicanUpstreamModel'), value: upstream, mono: true }] : []),
    { label: t('feedback.pelicanReasoning'), value: report.reasoning_effort || t('feedback.pelicanReasoningDefault'), mono: false },
    { label: t('feedback.pelicanGroup'), value: group, mono: false },
    { label: t('feedback.pelicanAccount'), value: account, mono: false },
    { label: t('feedback.pelicanApiKey'), value: usage.api_key_name || `#${usage.api_key_id}`, mono: false },
    { label: t('feedback.pelicanDuration'), value: ms(usage.duration_ms ?? report.client_duration_ms), mono: false },
    { label: t('feedback.pelicanFirstToken'), value: ms(usage.first_token_ms ?? report.client_ttft_ms), mono: false },
    { label: t('feedback.pelicanTokens'), value: `${usage.input_tokens.toLocaleString()} / ${usage.output_tokens.toLocaleString()}`, mono: false },
    { label: t('feedback.pelicanCost'), value: `$${usage.total_cost.toFixed(4)}`, mono: false }
  ]
})
function resetPelicanSource() {
  pelicanSource.value = null
  pelicanSourceOpen.value = false
  pelicanSourceLoading.value = false
  pelicanSourceError.value = ''
}
async function togglePelicanSource() {
  const id = selected.value?.id
  if (!id) return
  if (pelicanSourceOpen.value) { pelicanSourceOpen.value = false; return }
  if (pelicanSource.value) { pelicanSourceOpen.value = true; return }
  pelicanSourceLoading.value = true
  pelicanSourceError.value = ''
  try {
    const result = await getPelicanReportSource(id)
    if (selected.value?.id !== id) return
    pelicanSource.value = result
    pelicanSourceOpen.value = true
  } catch {
    if (selected.value?.id === id) pelicanSourceError.value = t('feedback.pelicanSourceLoadFailed')
  } finally {
    if (selected.value?.id === id) pelicanSourceLoading.value = false
  }
}
let listVersion = 0
let detailVersion = 0
const columns = computed(() => [
  { key: 'title', label: t('feedback.ticketTitle') }, { key: 'user', label: t('feedback.account') },
  { key: 'usage', label: `${t('feedback.totalRecharged')} / ${t('feedback.tokens7d')}` },
  { key: 'rating', label: t('feedback.rating') }, { key: 'status', label: t('feedback.status') },
  { key: 'created_at', label: t('feedback.createdAt') }, { key: 'actions', label: t('feedback.actions') }
])
function displayName(ticket: FeedbackTicket) { return ticket.snapshot?.username || `#${ticket.user_id}` }
function statusBadge(status: FeedbackStatus) {
  return status === 'resolved' ? 'badge-success' : status === 'replied' ? 'badge-gray' : 'badge-warning'
}
async function loadList() {
  const version = ++listVersion
  loading.value = true
  listError.value = ''
  try {
    const data = await listFeedback({ ...filters }, true)
    if (version === listVersion) { tickets.value = data.items; total.value = data.total; counts.value = data.category_counts }
  } catch { if (version === listVersion) listError.value = t('feedback.loadFailed') }
  finally { if (version === listVersion) loading.value = false }
}
async function openTicket(id: number) {
  if (replying.value) return
  const version = ++detailVersion
  selected.value = null
  resetPelicanSource()
  detailOpen.value = true
  detailLoading.value = true
  detailError.value = ''
  try {
    const ticket = await getFeedback(id, true)
    if (version !== detailVersion) return
    selected.value = ticket
    replyContent.value = ''
    replyError.value = ''
    resolved.value = ticket.status === 'resolved'
  } catch { if (version === detailVersion) detailError.value = t('feedback.loadFailed') }
  finally { if (version === detailVersion) detailLoading.value = false }
}
async function sendReply() {
  if (!selected.value || replying.value || !replyContent.value.trim()) return
  const id = selected.value.id
  replying.value = true
  replyError.value = ''
  try { await replyFeedback(id, replyContent.value.trim(), resolved.value) }
  catch { replyError.value = t('feedback.replyFailed'); replying.value = false; return }
  // A failed refresh must never turn a successful reply into a retryable send.
  replyContent.value = ''
  app.showSuccess(t('feedback.replySent'))
  try {
    selected.value = await getFeedback(id, true)
    await nextTick()
    threadRef.value?.scrollTo({ top: threadRef.value.scrollHeight, behavior: 'smooth' })
  }
  catch { detailError.value = t('feedback.loadFailed') }
  finally { replying.value = false }
  void loadList()
  void feedbackStore.refresh()
}
function closeDetail() {
  if (replying.value) return
  ++detailVersion
  detailOpen.value = false
  detailLoading.value = false
  selected.value = null
  resetPelicanSource()
}
function setCategory(category: string) { filters.category = category; resetPage() }
function resetPage() { filters.page = 1; void loadList() }
function changePage(page: number) { filters.page = page; void loadList() }
function changeSize(size: number) { filters.page_size = size; resetPage() }
async function refresh() {
  void feedbackStore.refresh()
  void loadList()
  if (selected.value && !replying.value) {
    const id = selected.value.id
    const version = ++detailVersion
    try { const ticket = await getFeedback(id, true); if (version === detailVersion && selected.value?.id === id) selected.value = ticket }
    catch { if (version === detailVersion) detailError.value = t('feedback.loadFailed') }
  }
}
onMounted(loadList)
onBeforeUnmount(() => { ++listVersion; ++detailVersion })
</script>
