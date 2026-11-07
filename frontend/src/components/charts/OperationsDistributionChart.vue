<template>
  <section class="card min-w-0 p-4">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ title }}</h3>
      <div class="flex flex-wrap gap-2">
        <Select v-if="type === 'model'" class="w-36" :model-value="modelSource" :options="sourceOptions" :aria-label="t('operations.modelSource')" @update:model-value="emit('update:modelSource', $event as ModelSource)" />
        <Select v-model="metric" class="w-36" :options="metricOptions" :aria-label="t('operations.metric')" />
      </div>
    </div>
    <div class="relative">
      <div ref="chartElement" class="h-72 w-full" :aria-label="title" />
      <div v-if="loading" class="absolute inset-0 flex items-center justify-center bg-white/90 dark:bg-dark-800/90"><LoadingSpinner /></div>
      <div v-else-if="!hasChartData" class="absolute inset-0 flex items-center justify-center text-sm text-gray-500">{{ t('common.noData') }}</div>
    </div>
    <div v-if="!loading && sortedRows.length" class="max-h-80 overflow-auto">
      <table class="w-full whitespace-nowrap text-xs">
        <thead class="text-gray-500 dark:text-gray-400">
          <tr>
            <th class="pb-2 text-left">{{ type === 'model' ? t('usage.model') : t('admin.dashboard.group') }}</th>
            <th class="px-2 pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
            <th class="px-2 pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
            <th class="px-2 pb-2 text-right">{{ t('admin.dashboard.actual') }}</th>
            <th class="px-2 pb-2 text-right">{{ t('admin.dashboard.accountCost') }}</th>
            <th class="pl-2 pb-2 text-right">{{ t('admin.dashboard.standard') }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="row in sortedRows" :key="row.id">
            <tr class="border-t border-gray-100 dark:border-dark-700">
              <td class="max-w-56 truncate py-2" :title="row.name">
                <Button v-if="type === 'model' || Number(row.id) > 0" variant="link" size="sm" class="h-auto max-w-full truncate p-0 text-xs" :aria-expanded="expandedId === row.id" @click="toggleBreakdown(row.id)">{{ row.name }}</Button>
                <span v-else>{{ row.name }}</span>
              </td>
              <td class="px-2 text-right tabular-nums">{{ row.requests.toLocaleString() }}</td>
              <td class="px-2 text-right tabular-nums">{{ row.total_tokens.toLocaleString() }}</td>
              <td class="px-2 text-right tabular-nums text-green-600">${{ row.actual_cost.toFixed(4) }}</td>
              <td class="px-2 text-right tabular-nums text-orange-500">{{ row.account_cost == null ? '—' : `$${row.account_cost.toFixed(4)}` }}</td>
              <td class="pl-2 text-right tabular-nums">${{ row.cost.toFixed(4) }}</td>
            </tr>
            <tr v-if="expandedId === row.id"><td colspan="6" class="p-0">
              <p v-if="breakdownError" role="alert" class="p-3 text-red-500">{{ t('operations.loadFailed') }}</p>
              <UserBreakdownSubTable v-else :items="breakdownItems" :loading="breakdownLoading" :show-account-cost="true" />
            </td></tr>
          </template>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { init, use, type EChartsType } from 'echarts/core'
import { PieChart } from 'echarts/charts'
import { AriaComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import { Button } from '@/components/ui/button'
import UserBreakdownSubTable from './UserBreakdownSubTable.vue'
import { getUserBreakdown } from '@/api/admin/dashboard'
import type { UserBreakdownItem } from '@/types'

// Existing distribution components use Chart.js. Keep this ECharts renderer
// separate while reusing their API, Select and user-breakdown presentation.
use([PieChart, LegendComponent, TooltipComponent, AriaComponent, CanvasRenderer])
type ModelSource = 'requested' | 'upstream' | 'mapping'
interface Row {
  id: number | string
  name: string
  requests: number
  total_tokens: number
  actual_cost: number
  account_cost?: number
  cost: number
}
const props = defineProps<{
  title: string
  type: 'model' | 'group'
  rows: Row[]
  loading: boolean
  modelSource?: ModelSource
  startDate: string
  endDate: string
}>()
const emit = defineEmits<{ 'update:modelSource': [value: ModelSource] }>()
const { t, locale } = useI18n()
const sourceOptions = computed(() => [
  { value: 'requested', label: t('usage.requestedModel') },
  { value: 'upstream', label: t('usage.upstreamModel') },
  { value: 'mapping', label: t('usage.mapping') },
])
const metricOptions = computed(() => [
  { value: 'tokens', label: t('admin.dashboard.metricTokens') },
  { value: 'actual_cost', label: t('admin.dashboard.metricActualCost') },
])
const metric = ref<'tokens' | 'actual_cost'>('tokens')
const chartElement = ref<HTMLElement>()
let chart: EChartsType | undefined
let resizeObserver: ResizeObserver | undefined
let themeObserver: MutationObserver | undefined
const metricKey = computed(() => metric.value === 'tokens' ? 'total_tokens' : 'actual_cost')
const sortedRows = computed(() => [...props.rows].sort((a, b) => b[metricKey.value] - a[metricKey.value]))
const hasChartData = computed(() => sortedRows.value.some(row => row[metricKey.value] > 0))

function renderChart() {
  if (!chart) return
  const color = document.documentElement.classList.contains('dark') ? '#d1d5db' : '#4b5563'
  chart.setOption({
    color: ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ef4444', '#06b6d4', '#ec4899', '#f97316'],
    aria: { enabled: true },
    textStyle: { color },
    tooltip: {
      trigger: 'item',
      renderMode: 'richText',
      valueFormatter: (value: unknown) => metric.value === 'actual_cost' ? `$${Number(value).toFixed(4)}` : Number(value).toLocaleString(),
    },
    legend: { type: 'scroll', bottom: 0, textStyle: { color } },
    series: [{
      type: 'pie', radius: ['42%', '68%'], center: ['50%', '44%'],
      stillShowZeroSum: false,
      label: { show: false },
      emphasis: { label: { show: true, formatter: '{d}%', color } },
      data: sortedRows.value.filter(row => row[metricKey.value] > 0).map(row => ({ name: row.name, value: row[metricKey.value] })),
    }],
  }, true)
}

const expandedId = ref<number | string | null>(null)
const breakdownItems = ref<UserBreakdownItem[]>([])
const breakdownLoading = ref(false)
const breakdownError = ref(false)
let breakdownSeq = 0
async function toggleBreakdown(id: number | string) {
  const seq = ++breakdownSeq
  if (expandedId.value === id) { expandedId.value = null; return }
  expandedId.value = id
  breakdownItems.value = []
  breakdownLoading.value = true
  breakdownError.value = false
  try {
    const result = await getUserBreakdown({
      start_date: props.startDate, end_date: props.endDate,
      ...(props.type === 'model' ? { model: String(id), model_source: props.modelSource } : { group_id: Number(id) }),
    })
    if (seq === breakdownSeq) breakdownItems.value = result.users || []
  } catch {
    if (seq === breakdownSeq) breakdownError.value = true
  } finally {
    if (seq === breakdownSeq) breakdownLoading.value = false
  }
}
watch(() => [props.startDate, props.endDate, props.modelSource, props.rows], () => {
  breakdownSeq++
  expandedId.value = null
})
watch([sortedRows, metric, locale], renderChart)
onMounted(() => {
  if (!chartElement.value) return
  chart = init(chartElement.value)
  renderChart()
  resizeObserver = new ResizeObserver(() => chart?.resize())
  resizeObserver.observe(chartElement.value)
  themeObserver = new MutationObserver(renderChart)
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})
onBeforeUnmount(() => {
  breakdownSeq++
  resizeObserver?.disconnect()
  themeObserver?.disconnect()
  chart?.dispose()
})
</script>
