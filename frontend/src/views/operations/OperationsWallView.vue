<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import AccountPriorityCell from '@/components/account/AccountPriorityCell.vue'
import OperationsResultCard from '@/components/pelican/OperationsResultCard.vue'
import OperationsResultStack from '@/components/pelican/OperationsResultStack.vue'
import { operationsApi, type OperationsAccount, type OperationsResult, type OperationsTask, type OperationsWallGroups, type OperationsWallGroup, type ResultQuery } from '@/api/admin/operationsConsole'

const { t } = useI18n()
const app = useAppStore()
const scope = ref<'current' | 'past'>('current')
const catalog = ref<OperationsWallGroups>({ current: [], past: [] })
const cleaning = ref(false)
const groupError = ref('')
const cleanupGroup = ref<OperationsWallGroup | null>(null)
const cleanupAccount = ref<{ id: number; name: string; groupID?: number; groupName: string } | null>(null)
const cleanupMessage = computed(() => cleanupAccount.value
  ? t('operationsConsole.cleanupAccountConfirm', { name: cleanupAccount.value.name, group: cleanupAccount.value.groupName })
  : t('operationsConsole.cleanupConfirm', { name: cleanupGroup.value ? groupName(cleanupGroup.value) : '' }))
const group = ref<string | number | boolean | null>(0)
const account = ref<string | number | boolean | null>(0)
const task = ref<string | number | boolean | null>(0)
const state = ref<string | number | boolean | null>('')
const tasks = ref<OperationsTask[]>([])
const items = ref<OperationsResult[]>([])
const hasMore = ref(false)
const nextAccountID = ref(0)
const loading = ref(false)
const rowHasMore = ref<Record<number, boolean>>({})
const rowLoading = ref(new Set<number>())
const rowHistoryLoaded = ref(new Set<number>())
const rowHistoryError = ref(new Set<number>())
const rowControllers = new Map<number, AbortController>()
let expanded = false
let timer: ReturnType<typeof setInterval> | undefined
let controller: AbortController | undefined
let generation = 0
let metadataVersion = 0
const visibleGroups = computed(() => catalog.value[scope.value])
const selectedGroup = computed(() => visibleGroups.value.find(g => g.id === Number(group.value)))
const accounts = computed(() => {
  const byID = new Map<number, OperationsAccount>()
  for (const g of visibleGroups.value) for (const a of g.accounts) {
    let entry = byID.get(a.id)
    if (!entry) { entry = { ...a, groups: [], has_ungrouped: false }; byID.set(a.id, entry) }
    if (g.id === -1) entry.has_ungrouped = true
    else entry.groups.push({ id: g.id, name: g.name })
  }
  return [...byID.values()]
})
const groups = computed(() => [{ value: 0, label: t('operationsConsole.allGroups') }, ...visibleGroups.value.map(g => ({ value: g.id, label: groupName(g) }))])
const accountOptions = computed(() => [{ value: 0, label: t('operationsConsole.allAccounts') }, ...accounts.value.filter(a => !group.value || (Number(group.value) === -1 ? a.has_ungrouped || !a.groups.length : a.groups.some(g => g.id === Number(group.value)))).map(a => ({ value: a.id, label: a.name }))])
const taskOptions = computed(() => [{ value: 0, label: t('operationsConsole.allTasks') }, ...tasks.value.map(t => ({ value: t.id, label: t.name }))])
const statusOptions = computed(() => [{ value: '', label: t('operationsConsole.allStatuses') }, ...['queued', 'running', 'uploading', 'succeeded', 'failed', 'cancelled'].map(value => ({ value, label: t(`operationsConsole.statuses.${value}`) }))])
const accountRows = computed(() => {
  const priorities = new Map(accounts.value.map(a => [a.id, a.priority]))
  const detectPaused = new Map(accounts.value.map(a => [a.id, a.detect_paused]))
  const rows = new Map<number, { id: number; name: string; priority?: number; detect_paused?: boolean; results: OperationsResult[] }>()
  for (const result of [...items.value].sort((a, b) => (Date.parse(b.created_at) - Date.parse(a.created_at) || b.id - a.id))) {
    let row = rows.get(result.account_id)
    if (!row) {
      row = { id: result.account_id, name: result.account_name, priority: priorities.get(result.account_id), detect_paused: detectPaused.get(result.account_id), results: [] }
      rows.set(row.id, row)
    }
    if (!row.results.some(item => item.id === result.id)) row.results.push(result)
  }
  return [...rows.values()].sort((a, b) => b.id - a.id)
})
async function load(append = false, quiet = false) {
  if (append && loading.value) return
  for (const pending of rowControllers.values()) pending.abort()
  rowControllers.clear(); rowLoading.value.clear()
  controller?.abort(); controller = new AbortController(); const request = ++generation
  if (!quiet) loading.value = true
  try {
    const query: ResultQuery = { ...resultFilters(), group_by: 'account', limit: 12 }
    if (append) query.before_account_id = nextAccountID.value
    const page = await operationsApi.results(query, controller.signal)
    if (request !== generation) return
    if (!append) { rowHistoryLoaded.value.clear(); rowHistoryError.value.clear() }
    items.value = append ? [...items.value, ...page.items] : page.items; hasMore.value = page.has_more; nextAccountID.value = page.next_account_id || 0
    rowHasMore.value = append ? { ...rowHasMore.value, ...page.account_has_more } : { ...page.account_has_more }
    if (append || !quiet) expanded = append
  } catch (err) { if (request === generation && !controller?.signal.aborted && !quiet) app.showError((err as { response?: { data?: { message?: string } } })?.response?.data?.message || t('operationsConsole.error')) } finally { if (request === generation) loading.value = false }
}
function resultFilters(): ResultQuery {
  // The wall needs consecutive tests, rather than the legacy latest-per-plan view.
  return { view: 'history', scope: scope.value, group_id: Number(group.value) || undefined, account_id: Number(account.value) || undefined, task_id: Number(task.value) || undefined, status: String(state.value || '') }
}
function onRowWheel(event: WheelEvent) {
  if (event.ctrlKey || document.fullscreenElement) return
  const track = event.currentTarget as HTMLElement
  const delta = Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY
  const scale = event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? track.clientWidth : 1
  const next = Math.max(0, Math.min(track.scrollWidth - track.clientWidth, track.scrollLeft + delta * scale))
  if (next === track.scrollLeft || track.scrollWidth <= track.clientWidth) return
  event.preventDefault()
  track.scrollLeft = next
}
async function loadMoreResults(accountID: number) {
  if (loading.value || rowLoading.value.has(accountID) || rowHistoryLoaded.value.has(accountID)) return
  const row = accountRows.value.find(row => row.id === accountID)
  const last = row?.results.slice(0, 3).at(-1)
  if (!last) return
  const request = generation
  const pending = new AbortController()
  rowControllers.set(accountID, pending); rowLoading.value.add(accountID); rowHistoryError.value.delete(accountID); expanded = true
  try {
    const page = await operationsApi.results({ ...resultFilters(), account_id: accountID, before_id: last.id, limit: 3 }, pending.signal)
    if (request !== generation || pending.signal.aborted) return
    const knownIDs = new Set(items.value.map(item => item.id))
    items.value = [...items.value, ...page.items.filter(item => !knownIDs.has(item.id))]
    rowHistoryLoaded.value.add(accountID)
    rowHasMore.value[accountID] = page.has_more
  } catch (err) {
    if (request === generation && !pending.signal.aborted) { rowHistoryError.value.add(accountID); app.showError(extractApiErrorMessage(err, t('operationsConsole.error'))) }
  } finally {
    if (rowControllers.get(accountID) === pending) { rowControllers.delete(accountID); rowLoading.value.delete(accountID) }
  }
}
function groupName(g: OperationsWallGroup) { return g.id === -1 ? t('operationsConsole.ungrouped') : g.name }
async function switchTab(direction: number) {
  const tabs = ['current', 'past'] as const
  scope.value = tabs[(tabs.indexOf(scope.value) + direction + tabs.length) % tabs.length]!
  await nextTick(); document.getElementById(`operations-${scope.value}-tab`)?.focus()
}
async function metadata() {
  const request = ++metadataVersion
  // A failed task-list request must not discard a successfully loaded group
  // catalog: cleanup only depends on the latter.
  const [groupResult, taskResult] = await Promise.allSettled([operationsApi.groups(), operationsApi.tasks()])
  if (request !== metadataVersion) return false
  if (taskResult.status === 'fulfilled') tasks.value = taskResult.value
  else app.showError(extractApiErrorMessage(taskResult.reason, t('operationsConsole.error')))
  if (groupResult.status === 'fulfilled') {
    const nextCatalog = groupResult.value
    const changed = JSON.stringify(nextCatalog) !== JSON.stringify(catalog.value)
    catalog.value = nextCatalog; groupError.value = ''
    if (group.value && !visibleGroups.value.some(g => g.id === Number(group.value))) group.value = 0
    if (account.value && !accountOptions.value.some(a => a.value === account.value)) account.value = 0
    return changed
  }
  groupError.value = extractApiErrorMessage(groupResult.reason, t('operationsConsole.error'))
}
function updatePriority(updated: { id: number; priority: number }) {
  // Keep cached cards mounted and prevent a pre-save catalog response from
  // replacing the saved priority with an older value.
  metadataVersion++
  for (const tab of ['current', 'past'] as const) {
    for (const g of catalog.value[tab]) {
      for (const a of g.accounts) if (a.id === updated.id) a.priority = updated.priority
    }
  }
}
const detectBusy = ref(new Set<number>())
async function toggleDetection(target: { id: number; detect_paused?: boolean }) {
  if (detectBusy.value.has(target.id)) return
  detectBusy.value.add(target.id)
  try {
    const paused = !target.detect_paused
    await operationsApi.setDetectPaused(target.id, paused)
    metadataVersion++
    for (const tab of ['current', 'past'] as const) {
      for (const g of catalog.value[tab]) {
        for (const a of g.accounts) if (a.id === target.id) a.detect_paused = paused
      }
    }
  } catch (err) { app.showError(extractApiErrorMessage(err, t('operationsConsole.error'))) }
  finally { detectBusy.value.delete(target.id) }
}
async function refresh(manual = false) {
  const changed = await metadata()
  if (manual || changed || !expanded) await load(false, !manual)
}
async function cleanGroup() {
  const target = cleanupGroup.value
  const targetAccount = cleanupAccount.value
  if ((!target && !targetAccount) || cleaning.value) return
  cleaning.value = true
  try {
    const out = targetAccount
      ? await operationsApi.cleanAccount(targetAccount.id, targetAccount.groupID)
      : await operationsApi.cleanGroup(target!.id)
    cleanupGroup.value = null; cleanupAccount.value = null
    if (out.failed) app.showError(t('operationsConsole.cleanupPartial', { count: out.failed }))
    else app.showSuccess(t('operationsConsole.cleaned'))
    await metadata(); await load()
  } catch (err) { app.showError(extractApiErrorMessage(err, t('operationsConsole.error'))) }
  finally { cleaning.value = false }
}
function selectAccountCleanup(row: { id: number; name: string }) {
  cleanupGroup.value = null
  cleanupAccount.value = { ...row, groupID: Number(group.value) || undefined, groupName: selectedGroup.value ? groupName(selectedGroup.value) : t('operationsConsole.allGroups') }
}
watch(scope, () => { group.value = 0; account.value = 0; cleanupGroup.value = null; cleanupAccount.value = null; items.value = []; void metadata() })
watch(group, () => { account.value = 0 })
watch([scope, group, account, task, state], () => { void load() })
onMounted(() => { void metadata(); void load(); timer = setInterval(() => { if (!document.hidden && !loading.value) void refresh() }, 5000) })
onUnmounted(() => { clearInterval(timer); controller?.abort(); for (const pending of rowControllers.values()) pending.abort() })
</script>

<template>
  <section>
    <div class="mb-4 flex flex-wrap items-end justify-between gap-3 border-b border-slate-100 dark:border-dark-800">
      <div role="tablist" :aria-label="t('operationsConsole.groupTabs')" class="flex gap-6">
        <button v-for="tab in (['current', 'past'] as const)" :id="`operations-${tab}-tab`" :key="tab" type="button" role="tab" :aria-selected="scope === tab" aria-controls="operations-group-panel" :tabindex="scope === tab ? 0 : -1" class="-mb-px border-b-2 px-1 pb-2 text-sm transition-colors" :class="scope === tab ? 'border-primary-500 font-medium text-primary-700 dark:text-primary-300' : 'border-transparent text-slate-500 hover:text-slate-700 dark:hover:text-slate-300'" @click="scope = tab" @keydown.left.prevent="switchTab(-1)" @keydown.right.prevent="switchTab(1)">{{ t(tab === 'current' ? 'operationsConsole.currentGroups' : 'operationsConsole.pastGroups') }}</button>
      </div>
      <div class="flex items-center gap-1">
        <button v-if="scope === 'past'" type="button" class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm text-red-500 transition-colors hover:bg-red-50 disabled:opacity-40 dark:hover:bg-red-950" :disabled="!selectedGroup || cleaning" @click="cleanupGroup = selectedGroup || null"><Icon name="trash" size="sm" />{{ t(cleaning ? 'operationsConsole.cleaning' : 'operationsConsole.cleanGroup') }}</button>
        <button type="button" class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm text-slate-500 transition-colors hover:bg-white hover:text-primary-600 disabled:opacity-50 dark:hover:bg-dark-800" :disabled="loading" @click="refresh(true)"><Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />{{ t('operationsConsole.refresh') }}</button>
      </div>
    </div>
    <div v-if="scope === 'past'" class="mb-4 flex items-start gap-1.5 text-xs text-slate-400"><Icon name="infoCircle" size="xs" class="mt-0.5 shrink-0" /><span>{{ t('operationsConsole.cleanupHint') }}</span></div>
    <div v-if="groupError" role="alert" class="mb-4 flex flex-wrap items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"><span>{{ t('operationsConsole.groupsLoadFailed') }} {{ groupError }}</span><button type="button" class="underline" @click="refresh(true)">{{ t('operationsConsole.retry') }}</button></div>
    <div id="operations-group-panel" role="tabpanel" :aria-labelledby="`operations-${scope}-tab`">
    <div class="mb-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4"><label class="min-w-0 space-y-1 text-xs text-slate-500"><span>{{ t('operationsConsole.groups') }}</span><Select :key="scope" v-model="group" :options="groups" :aria-label="t('operationsConsole.groups')" /></label><label class="min-w-0 space-y-1 text-xs text-slate-500"><span>{{ t('operationsConsole.account') }}</span><Select v-model="account" :options="accountOptions" :aria-label="t('operationsConsole.account')" /></label><label class="min-w-0 space-y-1 text-xs text-slate-500"><span>{{ t('operationsConsole.task') }}</span><Select v-model="task" :options="taskOptions" :aria-label="t('operationsConsole.task')" /></label><label class="min-w-0 space-y-1 text-xs text-slate-500"><span>{{ t('operationsConsole.allStatuses') }}</span><Select v-model="state" :options="statusOptions" :aria-label="t('operationsConsole.allStatuses')" /></label></div>
    <p v-if="loading && !items.length" class="p-12 text-center text-sm text-slate-500">{{ t('operationsConsole.loading') }}</p>
    <div v-else-if="!items.length" class="flex flex-col items-center gap-3 rounded-2xl bg-white py-16 text-center text-sm text-slate-400 dark:bg-dark-900"><Icon name="inbox" size="xl" class="text-slate-300 dark:text-dark-600" /><p>{{ t(scope === 'current' ? 'operationsConsole.currentEmpty' : 'operationsConsole.pastEmpty') }}</p></div>
    <div class="space-y-5">
      <section v-for="row in accountRows" :key="row.id" class="operations-account-row min-w-0" :aria-label="row.name" data-account-row>
        <div class="mb-3 flex flex-wrap items-center gap-x-2.5 gap-y-1">
          <span class="h-4 w-1 shrink-0 rounded-full bg-primary-400" aria-hidden="true"></span>
          <h3 class="min-w-0 max-w-full truncate text-sm font-semibold" :title="row.name">{{ row.name }}</h3>
          <div v-if="typeof row.priority === 'number'" class="inline-flex shrink-0 items-center gap-1 text-xs text-slate-500 dark:text-slate-400" :title="t('operationsConsole.priorityHint')">
            <span>{{ t('operationsConsole.priority') }}</span>
            <AccountPriorityCell :account="{ id: row.id, priority: row.priority }" always-visible @updated="updatePriority" @error="app.showError" />
          </div>
          <span v-else class="inline-flex shrink-0 items-center gap-1 text-xs text-slate-400" :title="t('operationsConsole.priorityUnavailable')"><Icon name="infoCircle" size="xs" />{{ t('operationsConsole.unavailable') }}</span>
          <span class="shrink-0 rounded-full bg-slate-100 px-2 py-0.5 text-[11px] text-slate-500 dark:bg-dark-800 dark:text-slate-400">{{ t('operationsConsole.recentCount', { count: Math.min(3, row.results.length) }) }}</span>
          <span v-if="row.detect_paused" class="inline-flex shrink-0 items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-[11px] text-amber-700 dark:bg-amber-900/40 dark:text-amber-300" :title="t('operationsConsole.detectPausedHint')"><Icon name="pause" size="xs" />{{ t('operationsConsole.detectPausedBadge') }}</span>
          <button v-if="row.detect_paused" type="button" class="ml-auto flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs text-green-600 transition-colors hover:bg-green-50 disabled:opacity-40 dark:text-green-400 dark:hover:bg-green-950" :disabled="detectBusy.has(row.id)" :aria-label="t('operationsConsole.resumeDetection')" @click="toggleDetection(row)"><Icon name="play" size="xs" />{{ t('operationsConsole.resumeDetection') }}</button>
          <button v-else-if="scope === 'current'" type="button" class="ml-auto flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs text-slate-500 transition-colors hover:bg-slate-100 disabled:opacity-40 dark:text-slate-400 dark:hover:bg-dark-800" :disabled="detectBusy.has(row.id)" :aria-label="t('operationsConsole.pauseDetection')" @click="toggleDetection(row)"><Icon name="pause" size="xs" />{{ t('operationsConsole.pauseDetection') }}</button>
          <button v-if="scope === 'past'" type="button" class="flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs text-red-500 hover:bg-red-50 disabled:opacity-40 dark:hover:bg-red-950" :disabled="cleaning" :aria-label="t('operationsConsole.cleanAccountLabel', { name: row.name })" @click="selectAccountCleanup(row)"><Icon name="trash" size="xs" />{{ t('operationsConsole.cleanAccount') }}</button>
        </div>
        <div class="operations-result-track flex min-w-0 items-stretch gap-4 overflow-x-auto pb-2" tabindex="0" :aria-label="t('operationsConsole.accountResults', { name: row.name })" @wheel="onRowWheel">
          <!-- retain-preview: scrolling a row must not rebuild already viewed
               iframes, which would restart each artwork from its blank intro. -->
          <OperationsResultCard v-for="result in row.results.slice(0, 3)" :key="result.id" class="operations-recent-card" :result="result" retain-preview :active="result.active_round && !['queued', 'running', 'uploading'].includes(result.status)" />
          <!-- Stable key: quiet refreshes shift results without remounting the
               stack, so an expanded stack never collapses under the pointer. -->
          <OperationsResultStack
            v-if="rowHasMore[row.id] !== false || row.results.length > 3"
            :key="row.id"
            :account-name="row.name"
            :results="row.results.slice(3, 6)"
            :loading="loading || rowLoading.has(row.id)"
            :loaded="rowHistoryLoaded.has(row.id)"
            :error="rowHistoryError.has(row.id)"
            @expand="expanded = true"
            @reveal="loadMoreResults(row.id)"
          />
        </div>
      </section>
    </div>
    <div v-if="hasMore" class="mt-8 text-center"><button type="button" class="inline-flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm text-slate-500 transition-colors hover:bg-white hover:text-primary-600 disabled:opacity-50 dark:hover:bg-dark-800" :disabled="loading" @click="load(true)">{{ t('operationsConsole.loadMore') }}<Icon name="chevronDown" size="xs" /></button></div>
    </div>
    <ConfirmDialog :show="!!cleanupGroup || !!cleanupAccount" :title="t(cleanupAccount ? 'operationsConsole.cleanAccount' : 'operationsConsole.cleanGroup')" :message="cleanupMessage" :confirm-text="t(cleaning ? 'operationsConsole.cleaning' : 'operationsConsole.cleanGroup')" danger @confirm="cleanGroup" @cancel="!cleaning && (cleanupGroup = null, cleanupAccount = null)" />
  </section>
</template>

<style scoped>
.operations-account-row {
  container-type: inline-size;
}
.operations-result-track {
  --operations-card-width: 82cqw;
}
.operations-recent-card {
  position: relative;
  z-index: 1;
  flex: 0 0 var(--operations-card-width);
}
@media (min-width: 640px) {
  .operations-result-track {
    --operations-card-width: calc((100cqw - 4rem) / 2);
  }
}
@media (min-width: 1024px) {
  .operations-result-track {
    --operations-card-width: calc((100cqw - 2rem) / 3.36);
  }
}
</style>
