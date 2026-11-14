<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { Button } from '@/components/ui/button'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import OperationsResultCard from '@/components/pelican/OperationsResultCard.vue'
import { operationsApi, type OperationsAccount, type OperationsResult, type OperationsTask, type OperationsWallGroups, type OperationsWallGroup, type ResultQuery } from '@/api/admin/operationsConsole'

const { t } = useI18n()
const app = useAppStore()
const scope = ref<'current' | 'past'>('current')
const catalog = ref<OperationsWallGroups>({ current: [], past: [] })
const cleaning = ref(false)
const cleanupGroup = ref<OperationsWallGroup | null>(null)
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
const rowControllers = new Map<number, AbortController>()
let expanded = false
let timer: ReturnType<typeof setInterval> | undefined
let controller: AbortController | undefined
let generation = 0
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
  const rows = new Map<number, { id: number; name: string; results: OperationsResult[] }>()
  for (const result of [...items.value].sort((a, b) => (Date.parse(b.created_at) - Date.parse(a.created_at) || b.id - a.id))) {
    let row = rows.get(result.account_id)
    if (!row) {
      row = { id: result.account_id, name: result.account_name, results: [] }
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
  if (loading.value || rowLoading.value.has(accountID)) return
  const row = accountRows.value.find(row => row.id === accountID)
  const last = row?.results.at(-1)
  if (!last) return
  const request = generation
  const pending = new AbortController()
  rowControllers.set(accountID, pending); rowLoading.value.add(accountID); expanded = true
  try {
    const page = await operationsApi.results({ ...resultFilters(), account_id: accountID, before_id: last.id, limit: 3 }, pending.signal)
    if (request !== generation || pending.signal.aborted) return
    const knownIDs = new Set(items.value.map(item => item.id))
    items.value = [...items.value, ...page.items.filter(item => !knownIDs.has(item.id))]
    rowHasMore.value[accountID] = page.has_more
  } catch (err) {
    if (request === generation && !pending.signal.aborted) app.showError((err as { response?: { data?: { message?: string } } })?.response?.data?.message || t('operationsConsole.error'))
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
  try {
    const [nextCatalog, nextTasks] = await Promise.all([operationsApi.groups(), operationsApi.tasks()])
    const changed = JSON.stringify(nextCatalog) !== JSON.stringify(catalog.value)
    catalog.value = nextCatalog; tasks.value = nextTasks
    if (group.value && !visibleGroups.value.some(g => g.id === Number(group.value))) group.value = 0
    if (account.value && !accountOptions.value.some(a => a.value === account.value)) account.value = 0
    return changed
  } catch (err) { app.showError((err as { response?: { data?: { message?: string } } })?.response?.data?.message || t('operationsConsole.error')) }
}
async function refresh() {
  const changed = await metadata()
  if (changed || !expanded) await load(false, true)
}
async function cleanGroup() {
  const target = cleanupGroup.value
  if (!target || cleaning.value) return
  cleaning.value = true
  try {
    const out = await operationsApi.cleanGroup(target.id)
    cleanupGroup.value = null
    if (out.failed) app.showError(t('operationsConsole.cleanupPartial', { count: out.failed }))
    else app.showSuccess(t('operationsConsole.cleaned'))
    await metadata(); await load()
  } catch (err) { app.showError((err as { response?: { data?: { message?: string } } })?.response?.data?.message || t('operationsConsole.error')) }
  finally { cleaning.value = false }
}
watch(scope, () => { group.value = 0; account.value = 0; cleanupGroup.value = null; items.value = [] })
watch(group, () => { account.value = 0 })
watch([scope, group, account, task, state], () => { void load() })
onMounted(() => { void metadata(); void load(); timer = setInterval(() => { if (!document.hidden && !loading.value) void refresh() }, 5000) })
onUnmounted(() => { clearInterval(timer); controller?.abort(); for (const pending of rowControllers.values()) pending.abort() })
</script>

<template>
  <section>
    <div class="mb-6 flex flex-wrap items-center justify-between gap-4"><div><h2 class="text-2xl font-semibold">{{ t('operationsConsole.wall') }}</h2><p class="mt-2 text-xs text-slate-500">{{ t('operationsConsole.retention') }}</p></div><Button variant="outline" :disabled="loading" @click="load()">{{ t('operationsConsole.refresh') }}</Button></div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div role="tablist" :aria-label="t('operationsConsole.groupTabs')" class="flex gap-1 rounded-lg bg-slate-100 p-1 dark:bg-dark-800">
        <Button v-for="tab in (['current', 'past'] as const)" :id="`operations-${tab}-tab`" :key="tab" role="tab" :aria-selected="scope === tab" aria-controls="operations-group-panel" :tabindex="scope === tab ? 0 : -1" :variant="scope === tab ? 'default' : 'ghost'" @click="scope = tab" @keydown.left.prevent="switchTab(-1)" @keydown.right.prevent="switchTab(1)">{{ t(tab === 'current' ? 'operationsConsole.currentGroups' : 'operationsConsole.pastGroups') }}</Button>
      </div>
      <Button v-if="scope === 'past'" variant="outline" class="border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950" :disabled="!selectedGroup || cleaning" @click="cleanupGroup = selectedGroup || null">{{ t(cleaning ? 'operationsConsole.cleaning' : 'operationsConsole.cleanGroup') }}</Button>
    </div>
    <div v-if="scope === 'past'" class="mb-3 text-xs text-slate-500">{{ t('operationsConsole.cleanupHint') }}</div>
    <div id="operations-group-panel" role="tabpanel" :aria-labelledby="`operations-${scope}-tab`">
    <div class="mb-6 grid gap-3 rounded-xl border border-slate-200 bg-white p-4 sm:grid-cols-2 xl:grid-cols-4 dark:border-dark-700 dark:bg-dark-900"><Select v-model="group" :options="groups" :aria-label="t('operationsConsole.groups')" /><Select v-model="account" :options="accountOptions" :aria-label="t('operationsConsole.account')" /><Select v-model="task" :options="taskOptions" :aria-label="t('operationsConsole.task')" /><Select v-model="state" :options="statusOptions" :aria-label="t('operationsConsole.allStatuses')" /></div>
    <p v-if="loading && !items.length" class="p-12 text-center text-sm text-slate-500">{{ t('operationsConsole.loading') }}</p>
    <p v-else-if="!items.length" class="rounded-xl border border-dashed p-12 text-center text-sm text-slate-500">{{ t(scope === 'current' ? 'operationsConsole.currentEmpty' : 'operationsConsole.pastEmpty') }}</p>
    <div class="space-y-5">
      <section v-for="row in accountRows" :key="row.id" class="min-w-0" :aria-label="row.name" data-account-row>
        <div class="mb-2 flex items-center gap-2"><h3 class="truncate text-sm font-semibold" :title="row.name">{{ row.name }}</h3><span class="shrink-0 text-[11px] text-slate-500">{{ t('operationsConsole.recentCount', { count: row.results.length }) }}</span></div>
        <div class="operations-result-track grid min-w-0 grid-flow-col items-start gap-4 overflow-x-auto pb-2" tabindex="0" :aria-label="t('operationsConsole.accountResults', { name: row.name })" @wheel="onRowWheel">
          <OperationsResultCard v-for="result in row.results" :key="result.id" :result="result" :active="result.active_round && !['queued', 'running', 'uploading'].includes(result.status)" />
          <div class="flex items-center justify-center self-stretch">
            <Button v-if="rowHasMore[row.id] !== false" variant="outline" :disabled="loading || rowLoading.has(row.id)" @click="loadMoreResults(row.id)">{{ t(rowLoading.has(row.id) ? 'operationsConsole.loading' : 'operationsConsole.viewMore') }}</Button>
            <span v-else class="text-xs text-slate-400">{{ t('operationsConsole.noMoreResults') }}</span>
          </div>
        </div>
      </section>
    </div>
    <div v-if="hasMore" class="mt-8 text-center"><Button variant="outline" :disabled="loading" @click="load(true)">{{ t('operationsConsole.loadMore') }}</Button></div>
    </div>
    <ConfirmDialog :show="!!cleanupGroup" :title="t('operationsConsole.cleanGroup')" :message="t('operationsConsole.cleanupConfirm', { name: cleanupGroup ? groupName(cleanupGroup) : '' })" :confirm-text="t(cleaning ? 'operationsConsole.cleaning' : 'operationsConsole.cleanGroup')" danger @confirm="cleanGroup" @cancel="!cleaning && (cleanupGroup = null)" />
  </section>
</template>

<style scoped>
.operations-result-track {
  grid-auto-columns: 80%;
}
@media (min-width: 640px) {
  .operations-result-track {
    grid-auto-columns: calc((100% - 1rem) / 2 * 0.8);
  }
}
@media (min-width: 1024px) {
  .operations-result-track {
    grid-auto-columns: calc((100% - 2rem) / 3 * 0.8);
  }
}
</style>
