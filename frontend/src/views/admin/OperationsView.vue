<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="card flex flex-wrap items-center justify-between gap-3 p-4">
        <div class="flex flex-col items-start gap-2 sm:flex-row sm:items-center sm:gap-3">
          <span class="text-sm font-medium">{{ t('admin.dashboard.timeRange') }}</span>
          <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" @change="loadUsage" />
        </div>
        <Button variant="outline" :disabled="usageLoading" @click="loadUsage">{{ t('common.refresh') }}</Button>
      </div>
      <p v-if="usageError" role="alert" class="text-sm text-red-500">{{ t('operations.loadFailed') }}</p>
      <div class="grid grid-cols-2 gap-4 xl:grid-cols-4" :aria-busy="usageLoading">
        <div class="card col-span-2 min-w-0 p-4">
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3 sm:divide-x sm:divide-gray-100 dark:sm:divide-dark-700">
            <div v-for="card in financialCards" :key="card.key" class="min-w-0 sm:pl-4 sm:first:pl-0">
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ card.label }}</p>
              <p class="mt-2 break-all text-2xl font-semibold tabular-nums" :class="card.color" :data-testid="`operations-${card.key}`">{{ usageLoading ? '…' : card.value }}</p>
            </div>
          </div>
        </div>
        <div v-for="card in usageCards" :key="card.key" class="card min-w-0 p-4">
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ card.label }}</p>
          <p class="mt-2 break-all text-2xl font-semibold tabular-nums text-gray-900 dark:text-white" :data-testid="`operations-${card.key}`">{{ usageLoading ? '…' : card.value }}</p>
        </div>
      </div>
      <section class="card p-4">
        <div class="mb-2 flex flex-wrap items-center justify-between gap-3">
          <h3 class="font-semibold">{{ t('operations.upstreamBalances') }}</h3>
          <div class="flex items-center gap-2">
            <Button variant="outline" :disabled="balanceLoading" @click="loadBalances">{{ balanceLoading ? t('common.loading') : t('common.refresh') }}</Button>
            <Button
              variant="ghost"
              size="icon"
              :aria-label="t(balancesVisible ? 'operations.hideBalances' : 'operations.showBalances')"
              :title="t(balancesVisible ? 'operations.hideBalances' : 'operations.showBalances')"
              :aria-pressed="balancesVisible"
              data-testid="toggle-balance-visibility"
              @click="balancesVisible = !balancesVisible"
            >
              <Icon :name="balancesVisible ? 'eyeOff' : 'eye'" size="sm" />
            </Button>
          </div>
        </div>
        <p v-if="balanceError" role="alert" class="py-4 text-sm text-red-500">{{ t(balanceError) }}</p>
        <div v-else-if="balanceLoading" class="flex justify-center py-8"><LoadingSpinner /></div>
        <p v-else-if="!balances.length" class="py-8 text-center text-sm text-gray-500">{{ t('operations.noUpstreams') }}</p>
        <div v-else class="overflow-x-auto">
          <table class="w-full whitespace-nowrap text-left text-sm">
            <thead class="text-xs text-gray-500"><tr>
              <th class="p-2">{{ t('operations.upstreamAddress') }}</th>
              <th class="p-2">{{ t('operations.accountCount') }}</th><th class="p-2">{{ t('operations.balance') }}</th>
              <th class="p-2">{{ t('common.status') }}</th><th class="p-2">{{ t('operations.queriedAt') }}</th>
            </tr></thead>
            <tbody :class="{ 'select-none blur-sm': !balancesVisible }" :inert="balancesVisible ? undefined : true" :aria-hidden="!balancesVisible"><tr v-for="row in balances" :key="row.base_url" class="border-t border-gray-100 dark:border-dark-700">
              <td class="p-2 font-mono text-xs">
                <a :href="row.base_url" target="_blank" rel="noopener noreferrer" class="text-blue-600 hover:underline dark:text-blue-400">{{ row.base_url }}</a>
              </td>
              <td class="p-2 tabular-nums">{{ row.account_count }}</td>
              <td class="p-2 tabular-nums">{{ row.status === 'ok' && row.balance != null ? `${row.balance.toFixed(4)} ${row.unit}` : '—' }}</td>
              <td class="p-2" :class="row.status === 'ok' ? 'text-green-600' : 'text-amber-600'">{{ row.status === 'ok' ? t('operations.success') : balanceErrorLabel(row) }}</td>
              <td class="whitespace-nowrap p-2 text-xs text-gray-500">{{ row.queried_at ? new Date(row.queried_at).toLocaleString() : '—' }}</td>
            </tr></tbody>
          </table>
        </div>
      </section>
      <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
        <OperationsDistributionChart v-model:model-source="modelSource" type="model" :title="t('admin.dashboard.modelDistribution')" :rows="modelRows" :loading="modelLoading" :start-date="startDate" :end-date="endDate" />
        <OperationsDistributionChart type="group" :title="t('admin.dashboard.groupDistribution')" :rows="groupRows" :loading="usageLoading" :start-date="startDate" :end-date="endDate" />
      </div>
      <p v-if="modelError" role="alert" class="text-sm text-red-500">{{ t('operations.modelLoadFailed') }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { Button } from '@/components/ui/button'
import OperationsDistributionChart from '@/components/charts/OperationsDistributionChart.vue'
import { adminUsageAPI, type AdminUsageStatsResponse } from '@/api/admin/usage'
import { getGroupStats, getModelStats } from '@/api/admin/dashboard'
import { getUpstreamBalances, type UpstreamBalance } from '@/api/admin/operations'
import { formatCompactNumber } from '@/utils/format'
import type { ModelStat, GroupStat } from '@/types'

const { t } = useI18n()
const localDate = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
const today = new Date()
const startDate = ref(localDate(today))
const endDate = ref(localDate(today))
const stats = ref<AdminUsageStatsResponse | null>(null)
const models = ref<ModelStat[]>([])
const groups = ref<GroupStat[]>([])
const modelSource = ref<'requested' | 'upstream' | 'mapping'>('requested')
const usageLoading = ref(false)
const modelLoading = ref(false)
const usageError = ref(false)
const modelError = ref(false)
let usageSeq = 0
let modelSeq = 0
const profit = computed(() => stats.value && stats.value.total_account_cost != null ? stats.value.total_actual_cost - stats.value.total_account_cost : null)
const money = (value: number | null | undefined) => value == null ? '—' : `$${value.toFixed(4)}`
const totalTokensLabel = computed(() => {
  const tokens = stats.value?.total_tokens
  if (tokens == null) return '—'
  return tokens >= 1_000_000 ? formatCompactNumber(tokens) : tokens.toLocaleString()
})
const financialCards = computed(() => [
  { key: 'consumption', label: t('usage.totalCost'), value: money(stats.value?.total_actual_cost), color: 'text-gray-900 dark:text-white' },
  { key: 'cost', label: t('operations.cost'), value: money(stats.value?.total_account_cost), color: 'text-green-600 dark:text-green-400' },
  { key: 'profit', label: t('operations.profit'), value: money(profit.value), color: 'text-orange-500 dark:text-orange-400' },
])
const usageCards = computed(() => [
  { key: 'requests', label: t('usage.totalRequests'), value: stats.value?.total_requests.toLocaleString() ?? '—' },
  { key: 'tokens', label: t('usage.totalTokens'), value: totalTokensLabel.value },
])
const modelRows = computed(() => models.value.map(row => ({ ...row, id: row.model, name: row.model })))
const groupRows = computed(() => groups.value.map(row => ({ ...row, id: row.group_id, name: row.group_name || t('admin.dashboard.noGroup') })))

async function loadModels() {
  const seq = ++modelSeq
  modelLoading.value = true
  modelError.value = false
  models.value = []
  try {
    const result = await getModelStats({ start_date: startDate.value, end_date: endDate.value, model_source: modelSource.value })
    if (seq === modelSeq) models.value = result.models || []
  } catch {
    if (seq === modelSeq) modelError.value = true
  } finally {
    if (seq === modelSeq) modelLoading.value = false
  }
}
async function loadUsage() {
  const seq = ++usageSeq
  usageLoading.value = true
  usageError.value = false
  stats.value = null
  groups.value = []
  void loadModels()
  const params = { start_date: startDate.value, end_date: endDate.value }
  const results = await Promise.allSettled([adminUsageAPI.getStats(params), getGroupStats(params)])
  if (seq !== usageSeq) return
  if (results[0].status === 'fulfilled') stats.value = results[0].value
  if (results[1].status === 'fulfilled') groups.value = results[1].value.groups || []
  usageError.value = results.some(result => result.status === 'rejected')
  usageLoading.value = false
}
const balances = ref<UpstreamBalance[]>([])
const balancesVisible = ref(false)
const balanceLoading = ref(false)
const balanceError = ref('')
let balanceController: AbortController | undefined
async function loadBalances() {
  balanceController?.abort()
  const controller = new AbortController()
  balanceController = controller
  balanceLoading.value = true
  balanceError.value = ''
  try {
    const result = await getUpstreamBalances(controller.signal)
    if (!controller.signal.aborted) balances.value = result
  } catch (error) {
    if (!controller.signal.aborted) {
      // Older backends route the new static path to /accounts/:id.
      const apiError = error as { status?: number; message?: string } | null
      balanceError.value = apiError?.status === 400 && apiError.message === 'Invalid account ID'
        ? 'operations.balanceBackendOutdated'
        : 'operations.balanceLoadFailed'
    }
  } finally {
    if (!controller.signal.aborted) balanceLoading.value = false
  }
}
function balanceErrorLabel(row: UpstreamBalance) {
  if (row.error === 'no_balance') return t('operations.noBalance')
  if (row.error === 'http_error') return `HTTP ${row.http_status}`
  if (row.error === 'invalid_base_url') return t('operations.invalidBaseUrl')
  if (row.error === 'invalid_api_key') return t('operations.invalidApiKey')
  return t('operations.queryFailed')
}
watch(modelSource, loadModels)
onMounted(() => { void loadUsage(); void loadBalances() })
onBeforeUnmount(() => { usageSeq++; modelSeq++; balanceController?.abort() })
</script>
