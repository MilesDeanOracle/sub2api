<script setup lang="ts">
import { computed, nextTick, onMounted, ref, useTemplateRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { formatCompactNumber } from '@/utils/format'
import { platformBadgeLightClass, platformLabel } from '@/utils/platformColors'
import { formatMultiplier, formatPercent, multiplierBadgeClass, normalizeStatus } from '@/utils/publicTransitFormat'
import { operationsApi, type GroupCacheQuery, type GroupCacheRow, type GroupCacheSetting } from '@/api/admin/operationsConsole'
import { GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'

// Enabled rows whose real cumulative rate is below this percentage are highlighted.
const LOW_RATE_THRESHOLD = 70

const { t } = useI18n()
const app = useAppStore()
const monitorFormat = useChannelMonitorFormat()

const rows = ref<GroupCacheRow[]>([])
const loading = ref(true)
const updatedAt = ref<Date | null>(null)
const saving = ref(new Set<number>())
const editing = ref<number | null>(null)
const draft = ref('')
// The editor is rendered inside the row loop, so Vue collects its template ref into an array.
type FloorEditor = { $el?: HTMLInputElement }
const floorInput = useTemplateRef<ReadonlyArray<FloorEditor> | FloorEditor>('floorInput')
const updatedLabel = computed(() => updatedAt.value?.toLocaleTimeString() ?? '')

// The form edits draft conditions; the table always shows the result of the last query.
const draftSearch = ref('')
const draftPlatform = ref<string | number | boolean | null>('')
const draftStatus = ref<string | number | boolean | null>('')
const appliedQuery = ref<GroupCacheQuery>({ search: '', platform: '', status: '' })
const platformOptions = computed(() => [
  { value: '', label: t('operationsConsole.groupCacheAllPlatforms') },
  ...GROUP_PLATFORM_OPTIONS.map(option => ({ ...option })),
])
const statusOptions = computed(() => [
  { value: '', label: t('operationsConsole.allStatuses') },
  { value: 'enabled', label: t('operationsConsole.groupCacheActive') },
  { value: 'paused', label: t('operationsConsole.groupCachePaused') },
])
const isFiltered = computed(() => appliedQuery.value.search !== '' || appliedQuery.value.platform !== '' || appliedQuery.value.status !== '')

function runQuery() {
  appliedQuery.value = {
    search: draftSearch.value.trim(),
    platform: String(draftPlatform.value ?? ''),
    status: String(draftStatus.value ?? ''),
  }
  void load()
}

function resetQuery() {
  draftSearch.value = ''
  draftPlatform.value = ''
  draftStatus.value = ''
  runQuery()
}
// Settings saved while a refresh is in flight win over that refresh's stale copy.
const savedDuringLoad = new Map<number, GroupCacheSetting>()
let loadVersion = 0

function showError(err: unknown, fallbackKey: string) {
  app.showError(extractApiErrorMessage(err, t(fallbackKey)))
}

async function load() {
  const request = ++loadVersion
  savedDuringLoad.clear()
  loading.value = true
  try {
    const page = await operationsApi.groupCache(appliedQuery.value)
    if (request !== loadVersion) return
    const list = page.items
    for (const row of list) {
      const saved = savedDuringLoad.get(row.group_id)
      if (saved) row.setting = saved
    }
    rows.value = list
    updatedAt.value = new Date()
    editing.value = null
  } catch (err) {
    if (request === loadVersion) showError(err, 'operationsConsole.groupCacheLoadFailed')
  } finally {
    if (request === loadVersion) loading.value = false
  }
}

async function saveSetting(row: GroupCacheRow, setting: GroupCacheSetting) {
  if (saving.value.has(row.group_id)) return
  saving.value.add(row.group_id)
  try {
    row.setting = await operationsApi.updateGroupCache(row.group_id, setting)
    savedDuringLoad.set(row.group_id, row.setting)
  } catch (err) {
    showError(err, 'operationsConsole.error')
  } finally {
    saving.value.delete(row.group_id)
  }
}

async function toggleEnabled(row: GroupCacheRow) {
  const wasEnabled = row.setting.enabled
  await saveSetting(row, { enabled: !wasEnabled, floor: row.setting.floor })
  // Under a status query the row no longer matches, so reload instead of keeping stale data.
  if (appliedQuery.value.status && row.setting.enabled !== wasEnabled) void load()
}

function isLowRate(row: GroupCacheRow) {
  return row.setting.enabled && row.cache_hit_rate < LOW_RATE_THRESHOLD
}

/** Returns the floor typed by the administrator, or null when the text is not a valid floor. */
function parseFloor(raw: string): number | null {
  const value = Number(raw)
  if (!Number.isFinite(value) || value <= 0 || value >= 100) return null
  return Math.abs(Math.round(value * 10) - value * 10) > 1e-9 ? null : value
}

function startEdit(row: GroupCacheRow) {
  if (saving.value.has(row.group_id)) return
  editing.value = row.group_id
  draft.value = row.setting.floor == null ? '' : String(row.setting.floor)
  void nextTick(() => {
    const target = floorInput.value
    const input = (Array.isArray(target) ? target[0] : target)?.$el
    input?.focus()
    input?.select()
  })
}

async function commitEdit(row: GroupCacheRow, blurred: boolean) {
  if (editing.value !== row.group_id) return
  const raw = draft.value.trim()
  const floor = raw === '' ? null : parseFloor(raw)
  if (raw !== '' && floor === null) {
    showError(t('operationsConsole.groupCacheFloorInvalid'), 'operationsConsole.groupCacheFloorInvalid')
    // A blur means the administrator moved on, so drop the invalid text instead of trapping focus.
    if (blurred) editing.value = null
    return
  }
  editing.value = null
  if (floor === row.setting.floor) return
  await saveSetting(row, { enabled: row.setting.enabled, floor })
}

function cancelEdit() {
  editing.value = null
}

onMounted(load)
</script>

<template>
  <section>
    <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
      <div class="max-w-3xl space-y-1">
        <p class="text-xs text-slate-500 dark:text-dark-300">{{ t('operationsConsole.groupCacheRule') }}</p>
        <p v-if="updatedLabel" class="text-xs text-slate-400 dark:text-dark-400" data-testid="group-cache-updated">{{ t('operationsConsole.groupCacheUpdatedAt', { time: updatedLabel }) }}</p>
      </div>
      <Button type="button" variant="outline" size="sm" class="h-8" :disabled="loading" @click="load">
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />{{ loading ? t('operationsConsole.loading') : t('operationsConsole.refresh') }}
      </Button>
    </div>

    <form class="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_180px_180px_auto] xl:items-end" data-testid="group-cache-query-form" @submit.prevent="runQuery">
      <label class="min-w-0 space-y-1 text-xs text-slate-500 dark:text-dark-300">
        <span>{{ t('operationsConsole.groupCacheName') }}</span>
        <SearchInput v-model="draftSearch" :placeholder="t('operationsConsole.groupCacheSearchPlaceholder')" :debounce-ms="0" data-testid="group-cache-search" />
      </label>
      <label class="min-w-0 space-y-1 text-xs text-slate-500 dark:text-dark-300">
        <span>{{ t('operationsConsole.groupCachePlatform') }}</span>
        <Select v-model="draftPlatform" :options="platformOptions" :aria-label="t('operationsConsole.groupCachePlatform')" data-testid="group-cache-platform" />
      </label>
      <label class="min-w-0 space-y-1 text-xs text-slate-500 dark:text-dark-300">
        <span>{{ t('operationsConsole.groupCacheStatus') }}</span>
        <Select v-model="draftStatus" :options="statusOptions" :aria-label="t('operationsConsole.groupCacheStatus')" data-testid="group-cache-status" />
      </label>
      <div class="flex gap-2 sm:col-span-2 xl:col-span-1">
        <Button type="submit" data-testid="group-cache-query">{{ t('operationsConsole.groupCacheQuery') }}</Button>
        <Button type="button" variant="outline" data-testid="group-cache-reset" @click="resetQuery">{{ t('operationsConsole.groupCacheReset') }}</Button>
      </div>
    </form>
    <p v-if="!loading || rows.length" class="mb-2 text-xs text-slate-500 dark:text-dark-300" data-testid="group-cache-total">{{ t('operationsConsole.groupCacheTotal', { total: rows.length }) }}</p>

    <div class="overflow-x-auto rounded-2xl bg-white shadow-sm dark:bg-dark-900">
      <table class="min-w-full divide-y divide-slate-200 text-sm dark:divide-dark-800">
        <thead class="bg-slate-50 text-xs uppercase text-slate-500 dark:bg-dark-950 dark:text-dark-400">
          <tr>
            <th class="whitespace-nowrap px-4 py-3 text-left font-medium">{{ t('operationsConsole.groups') }}</th>
            <th class="whitespace-nowrap px-4 py-3 text-left font-medium">{{ t('operationsConsole.groupCacheMultiplier') }}</th>
            <th class="whitespace-nowrap px-4 py-3 text-left font-medium">{{ t('operationsConsole.groupCacheModelCount') }}</th>
            <th class="whitespace-nowrap px-4 py-3 text-left font-medium">{{ t('operationsConsole.groupCacheAvailability') }}</th>
            <th class="whitespace-nowrap px-4 py-3 text-left font-medium">{{ t('operationsConsole.groupCacheRealRate') }}</th>
            <th class="whitespace-nowrap px-4 py-3 text-left font-medium">{{ t('operationsConsole.groupCacheTokens') }}</th>
            <th class="whitespace-nowrap px-4 py-3 text-left font-medium">{{ t('operationsConsole.groupCacheFloor') }}</th>
            <th class="whitespace-nowrap px-4 py-3 text-left font-medium">{{ t('operationsConsole.groupCacheStatus') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-dark-800">
          <tr v-if="loading && !rows.length">
            <td colspan="8" class="px-4 py-10 text-center text-slate-500 dark:text-dark-300">{{ t('operationsConsole.loading') }}</td>
          </tr>
          <tr v-else-if="!rows.length">
            <td colspan="8" class="px-4 py-10 text-center text-slate-500 dark:text-dark-300">{{ t(isFiltered ? 'operationsConsole.groupCacheQueryEmpty' : 'operationsConsole.groupCacheEmpty') }}</td>
          </tr>
          <tr v-for="row in rows" :key="row.group_id" data-testid="group-cache-row">
            <td class="px-4 py-3">
              <div class="flex min-w-48 items-center gap-3">
                <span class="inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg" :class="platformBadgeLightClass(row.platform)">
                  <PlatformIcon :platform="row.platform" size="md" />
                </span>
                <span class="min-w-0">
                  <span class="block truncate font-medium">{{ row.name }}</span>
                  <span class="block truncate text-xs text-slate-500 dark:text-dark-400">{{ platformLabel(row.platform) }}</span>
                </span>
              </div>
            </td>
            <td class="px-4 py-3">
              <span class="inline-flex min-w-[58px] items-center justify-center rounded-full px-2.5 py-1 text-xs font-semibold tabular-nums ring-1" :class="multiplierBadgeClass(row.rate_multiplier)">{{ formatMultiplier(row.rate_multiplier) }}</span>
            </td>
            <td class="px-4 py-3">
              <span class="font-medium">{{ row.model_count }}</span>
            </td>
            <td class="px-4 py-3 text-xs">
              <div v-if="row.monitor" class="space-y-1">
                <span class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px]" :class="monitorFormat.statusBadgeClass(normalizeStatus(row.monitor.status))">
                  {{ monitorFormat.statusLabel(normalizeStatus(row.monitor.status)) }}
                </span>
                <div class="text-slate-500 dark:text-dark-400">{{ row.monitor.model }}</div>
              </div>
              <span v-else class="text-slate-400">-</span>
            </td>
            <td class="px-4 py-3">
              <span
                class="tabular-nums"
                :class="{ 'font-semibold text-red-600 dark:text-red-400': isLowRate(row) }"
                :title="isLowRate(row) ? t('operationsConsole.groupCacheLowRateHint') : undefined"
                data-testid="group-cache-real-rate"
              >{{ formatPercent(row.cache_hit_rate) }}</span>
            </td>
            <td class="px-4 py-3 text-xs">
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 tabular-nums" data-testid="group-cache-tokens">
                <dt class="text-slate-500 dark:text-dark-400">{{ t('operationsConsole.groupCacheTokenRead') }}</dt>
                <dd class="text-slate-700 dark:text-dark-100">{{ formatCompactNumber(row.cache_read_tokens) }}</dd>
                <dt class="text-slate-500 dark:text-dark-400">{{ t('operationsConsole.groupCacheTokenCreation') }}</dt>
                <dd class="text-slate-700 dark:text-dark-100">{{ formatCompactNumber(row.cache_creation_tokens) }}</dd>
                <dt class="text-slate-500 dark:text-dark-400">{{ t('operationsConsole.groupCacheTokenInput') }}</dt>
                <dd class="text-slate-700 dark:text-dark-100">{{ formatCompactNumber(row.input_tokens) }}</dd>
              </dl>
            </td>
            <td class="px-4 py-3 text-xs">
              <div v-if="editing === row.group_id" class="flex items-center gap-1">
                <Input
                  ref="floorInput"
                  :model-value="draft"
                  type="text"
                  inputmode="decimal"
                  class="h-8 w-24 text-center tabular-nums"
                  :aria-label="t('operationsConsole.groupCacheFloor')"
                  data-testid="group-cache-floor-input"
                  @update:model-value="draft = String($event)"
                  @keydown.enter.prevent="commitEdit(row, false)"
                  @keydown.esc.prevent="cancelEdit"
                  @blur="commitEdit(row, true)"
                />
                <span class="text-slate-400">%</span>
              </div>
              <button
                v-else
                type="button"
                class="inline-flex min-w-[72px] items-center justify-center gap-1 rounded-lg px-2 py-1 font-mono tabular-nums transition-colors hover:bg-slate-100 disabled:opacity-50 dark:hover:bg-dark-800"
                :class="row.setting.floor == null ? 'text-slate-400' : 'text-slate-700 dark:text-slate-200'"
                :disabled="saving.has(row.group_id)"
                :title="t('operationsConsole.groupCacheEditHint')"
                :aria-label="t('operationsConsole.groupCacheEditHint')"
                data-testid="group-cache-floor-value"
                @click="startEdit(row)"
              >
                {{ row.setting.floor == null ? t('operationsConsole.groupCacheUnset') : formatPercent(row.setting.floor) }}
                <Icon name="edit" size="xs" class="text-slate-400" />
              </button>
            </td>
            <td class="px-4 py-3">
              <div class="flex items-center gap-2">
                <Toggle :model-value="row.setting.enabled" :disabled="saving.has(row.group_id)" :aria-label="t('operationsConsole.groupCacheStatus')" @update:model-value="toggleEnabled(row)" />
                <span class="text-xs" :class="row.setting.enabled ? 'text-emerald-700 dark:text-emerald-300' : 'text-slate-500 dark:text-dark-300'">{{ t(row.setting.enabled ? 'operationsConsole.groupCacheActive' : 'operationsConsole.groupCachePaused') }}</span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
