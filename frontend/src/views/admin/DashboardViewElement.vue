<template>
  <AppLayout>
    <el-config-provider :locale="epLocale">
      <div v-loading="loading" class="space-y-6">
        <template v-if="stats">
          <!-- Row 1: 核心统计卡片 -->
          <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
            <el-card v-for="card in coreCards" :key="card.label" shadow="never" class="ep-stat-card">
              <div class="flex items-center gap-3">
                <div :class="['rounded-lg p-2', card.bgClass]">
                  <el-icon :class="card.iconClass" :size="22"><component :is="card.icon" /></el-icon>
                </div>
                <div>
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
                  <p class="text-xl font-bold text-gray-900 dark:text-white">{{ card.value }}</p>
                  <p v-if="card.sub" :class="['text-xs', card.subClass]">
                    <span v-html="card.sub"></span>
                  </p>
                </div>
              </div>
            </el-card>
          </div>

          <!-- Row 2: Token 统计卡片 -->
          <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
            <el-card v-for="card in tokenCards" :key="card.label" shadow="never" class="ep-stat-card">
              <div class="flex items-center gap-3">
                <div :class="['rounded-lg p-2', card.bgClass]">
                  <el-icon :class="card.iconClass" :size="22"><component :is="card.icon" /></el-icon>
                </div>
                <div>
                  <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
                  <p class="text-xl font-bold text-gray-900 dark:text-white">{{ card.value }}</p>
                  <p v-if="card.sub" class="text-xs" v-html="card.sub"></p>
                </div>
              </div>
            </el-card>
          </div>

          <!-- 快捷操作 -->
          <el-card shadow="never">
            <template #header>
              <span class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.dashboard.quickActions') }}
              </span>
            </template>
            <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
              <button
                v-if="canUseBatchImage"
                type="button"
                class="ep-quick-action hover:!bg-sky-50 dark:hover:!bg-sky-900/20"
                @click="router.push('/batch-image')"
              >
                <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-sky-100 text-sky-600 dark:bg-sky-900/30 dark:text-sky-400">
                  <el-icon :size="20"><MagicStick /></el-icon>
                </span>
                <span class="min-w-0 flex-1 text-left">
                  <span class="block text-sm font-medium text-gray-900 dark:text-white">
                    {{ t('admin.dashboard.batchImage') }}
                  </span>
                  <span class="block text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.batchImageDesc') }}
                  </span>
                </span>
                <el-icon class="text-gray-400"><ArrowRight /></el-icon>
              </button>
              <button
                type="button"
                class="ep-quick-action hover:!bg-emerald-50 dark:hover:!bg-emerald-900/20"
                @click="router.push('/admin/resource-pools?scope=all&focus=groups')"
              >
                <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400">
                  <el-icon :size="20"><Grid /></el-icon>
                </span>
                <span class="min-w-0 flex-1 text-left">
                  <span class="block text-sm font-medium text-gray-900 dark:text-white">
                    {{ t('admin.dashboard.groupPricing') }}
                  </span>
                  <span class="block text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.dashboard.groupPricingDesc') }}
                  </span>
                </span>
                <el-icon class="text-gray-400"><ArrowRight /></el-icon>
              </button>
            </div>
          </el-card>

          <!-- 图表区 -->
          <div class="space-y-6">
            <!-- 时间范围筛选 -->
            <el-card shadow="never">
              <div class="flex flex-wrap items-center gap-4">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.dashboard.timeRange') }}:
                  </span>
                  <el-date-picker
                    v-model="dateRange"
                    type="daterange"
                    value-format="YYYY-MM-DD"
                    :shortcuts="dateShortcuts"
                    range-separator="→"
                    :start-placeholder="t('admin.dashboard.timeRange')"
                    :end-placeholder="t('admin.dashboard.timeRange')"
                    class="!w-72"
                    @change="onDateRangeChange"
                  />
                </div>
                <el-button plain :icon="RefreshIcon" :disabled="chartsLoading" @click="loadDashboardStats">
                  {{ t('common.refresh') }}
                </el-button>
                <div class="ml-auto flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.dashboard.granularity') }}:
                  </span>
                  <el-select v-model="granularity" class="!w-28" @change="loadChartData">
                    <el-option value="day" :label="t('admin.dashboard.day')" />
                    <el-option value="hour" :label="t('admin.dashboard.hour')" />
                  </el-select>
                </div>
              </div>
            </el-card>

            <!-- 图表网格：模型分布 + Token 趋势 -->
            <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
              <ModelDistributionChart
                :model-stats="modelStats"
                :enable-ranking-view="true"
                :ranking-items="rankingItems"
                :ranking-total-actual-cost="rankingTotalActualCost"
                :ranking-total-requests="rankingTotalRequests"
                :ranking-total-tokens="rankingTotalTokens"
                :loading="chartsLoading"
                :ranking-loading="rankingLoading"
                :ranking-error="rankingError"
                :start-date="startDate"
                :end-date="endDate"
                @ranking-click="goToUserUsage"
              />
              <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
            </div>

            <!-- 用户用量趋势（全宽） -->
            <el-card shadow="never" v-loading="userTrendLoading">
              <div class="mb-4 flex items-center justify-between gap-3">
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.dashboard.recentUsage') }} (Top 12)
                </h3>
                <el-radio-group v-model="userTrendMetric" size="small" @change="setUserTrendMetric">
                  <el-radio-button value="tokens">{{ t('admin.dashboard.tokens') }}</el-radio-button>
                  <el-radio-button value="actual_cost">{{ t('admin.dashboard.actualSpending') }}</el-radio-button>
                </el-radio-group>
              </div>
              <div class="h-64">
                <Line v-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
                <el-empty
                  v-else
                  :description="t('admin.dashboard.noDataAvailable')"
                  :image-size="80"
                  class="!py-0"
                />
              </div>
            </el-card>
          </div>
        </template>
      </div>
    </el-config-provider>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import enUs from 'element-plus/es/locale/lang/en'
import {
  Key,
  Monitor,
  TrendCharts,
  UserFilled,
  Coin,
  Files,
  Lightning,
  Timer,
  MagicStick,
  Grid,
  ArrowRight,
  Refresh as RefreshIcon
} from '@element-plus/icons-vue'

const { t, locale } = useI18n()
const epLocale = computed(() => (locale.value === 'zh' ? zhCn : enUs))
import { adminAPI } from '@/api/admin'
import type {
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  UserUsageTrendPoint,
  UserSpendingRankingItem
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'

// Register Chart.js components（Element Plus 无图表组件，图表沿用 chart.js 绘图库）
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
)

const appStore = useAppStore()
const router = useRouter()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const userTrendLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)

// Chart data
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
const userTrendMetric = ref<'tokens' | 'actual_cost'>('tokens')
const rankingItems = ref<UserSpendingRankingItem[]>([])
const rankingTotalActualCost = ref(0)
const rankingTotalRequests = ref(0)
const rankingTotalTokens = ref(0)
let chartLoadSeq = 0
let usersTrendLoadSeq = 0
let rankingLoadSeq = 0

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

// Date range
const granularity = ref<'day' | 'hour'>('hour')
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const dateRange = ref<[string, string]>([defaultRange.start, defaultRange.end])

// 快捷日期（与原 DateRangePicker 预设一致）
const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate())
const dateShortcuts = computed(() => {
  const today = new Date()
  const daysAgo = (n: number) => {
    const d = new Date()
    d.setDate(d.getDate() - n)
    return startOfDay(d)
  }
  return [
    { text: t('dates.today'), value: (): [Date, Date] => [startOfDay(today), startOfDay(today)] },
    {
      text: t('dates.yesterday'),
      value: (): [Date, Date] => {
        const y = daysAgo(1)
        return [y, y]
      }
    },
    { text: t('dates.last24Hours'), value: (): [Date, Date] => [daysAgo(1), startOfDay(today)] },
    { text: t('dates.last7Days'), value: (): [Date, Date] => [daysAgo(6), startOfDay(today)] },
    { text: t('dates.last14Days'), value: (): [Date, Date] => [daysAgo(13), startOfDay(today)] },
    { text: t('dates.last30Days'), value: (): [Date, Date] => [daysAgo(29), startOfDay(today)] },
    {
      text: t('dates.thisMonth'),
      value: (): [Date, Date] => [new Date(today.getFullYear(), today.getMonth(), 1), startOfDay(today)]
    }
  ]
})

// Dark mode detection
const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

// Chart colors
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

// Line chart options (for user trend chart)
const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      itemSort: (a: any, b: any) => {
        const aValue = typeof a?.raw === 'number' ? a.raw : Number(a?.parsed?.y ?? 0)
        const bValue = typeof b?.raw === 'number' ? b.raw : Number(b?.parsed?.y ?? 0)
        return bValue - aValue
      },
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatUserTrendValue(Number(context.raw))}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatUserTrendValue(Number(value))
      }
    }
  }
}))

// User trend chart data
const userTrendChartData = computed(() => {
  if (!userTrend.value?.length) return null

  const getDisplayName = (point: UserUsageTrendPoint): string => {
    const username = point.username?.trim()
    if (username) {
      return username
    }

    const email = point.email?.trim()
    if (email) {
      return email
    }

    return t('admin.redeem.userPrefix', { id: point.user_id })
  }

  // Group by user_id to avoid merging different users with the same display name
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()

  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    const key = point.user_id
    if (!userGroups.has(key)) {
      userGroups.set(key, { name: getDisplayName(point), data: new Map() })
    }
    userGroups.get(key)!.data.set(point.date, userTrendMetric.value === 'tokens' ? point.tokens : point.actual_cost)
  })

  const sortedDates = Array.from(allDates).sort()
  const colors = [
    '#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899',
    '#14b8a6', '#f97316', '#6366f1', '#84cc16', '#06b6d4', '#a855f7'
  ]

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: colors[idx % colors.length],
    backgroundColor: `${colors[idx % colors.length]}20`,
    fill: false,
    tension: 0.3
  }))

  return {
    labels: sortedDates,
    datasets
  }
})

// Format helpers
const formatUserTrendValue = (value: number): string =>
  userTrendMetric.value === 'tokens' ? formatTokens(value) : `$${formatCost(value)}`

const formatTokens = (value: number | undefined): string => {
  if (value === undefined || value === null) return '0'
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

const formatNumber = (value: number | null | undefined): string => {
  return toFiniteNumber(value).toLocaleString()
}

const formatCost = (value: number | null | undefined): string => {
  const safeValue = toFiniteNumber(value)
  if (safeValue >= 1000) {
    return (safeValue / 1000).toFixed(2) + 'K'
  } else if (safeValue >= 1) {
    return safeValue.toFixed(2)
  } else if (safeValue >= 0.01) {
    return safeValue.toFixed(3)
  }
  return safeValue.toFixed(4)
}

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

// ==================== 统计卡片 ====================
interface StatCard {
  label: string
  value: string
  sub?: string
  subClass?: string
  icon: any
  bgClass: string
  iconClass: string
}

const coreCards = computed<StatCard[]>(() => {
  if (!stats.value) return []
  const s = stats.value
  return [
    {
      label: t('admin.dashboard.apiKeys'),
      value: formatNumber(s.total_api_keys),
      sub: `${formatNumber(s.active_api_keys)} ${t('common.active')}`,
      subClass: 'text-green-600 dark:text-green-400',
      icon: Key,
      bgClass: 'bg-blue-100 dark:bg-blue-900/30',
      iconClass: 'text-blue-600 dark:text-blue-400'
    },
    {
      label: t('admin.dashboard.accounts'),
      value: formatNumber(s.total_accounts),
      sub: `<span class="text-green-600 dark:text-green-400">${formatNumber(s.normal_accounts)} ${t('common.active')}</span>`
        + (s.error_accounts > 0 ? `<span class="ml-1 text-red-500">${formatNumber(s.error_accounts)} ${t('common.error')}</span>` : ''),
      icon: Monitor,
      bgClass: 'bg-purple-100 dark:bg-purple-900/30',
      iconClass: 'text-purple-600 dark:text-purple-400'
    },
    {
      label: t('admin.dashboard.todayRequests'),
      value: formatNumber(s.today_requests),
      sub: `${t('common.total')}: ${formatNumber(s.total_requests)}`,
      subClass: 'text-gray-500 dark:text-gray-400',
      icon: TrendCharts,
      bgClass: 'bg-green-100 dark:bg-green-900/30',
      iconClass: 'text-green-600 dark:text-green-400'
    },
    {
      label: t('admin.dashboard.users'),
      value: `+${formatNumber(s.today_new_users)}`,
      sub: `${t('common.total')}: ${formatNumber(s.total_users)}`,
      subClass: 'text-gray-500 dark:text-gray-400',
      icon: UserFilled,
      bgClass: 'bg-emerald-100 dark:bg-emerald-900/30',
      iconClass: 'text-emerald-600 dark:text-emerald-400'
    }
  ]
})

const costTriple = (prefix: 'today' | 'total'): string => {
  const s = stats.value!
  return `<span class="text-green-600 dark:text-green-400" title="${t('admin.dashboard.actual')}">$${formatCost(s[`${prefix}_actual_cost` as const])}</span>`
    + `<span class="text-gray-400 dark:text-gray-500"> / </span>`
    + `<span class="text-orange-500 dark:text-orange-400" title="${t('admin.dashboard.accountCost')}">$${formatCost(s[`${prefix}_account_cost` as const])}</span>`
    + `<span class="text-gray-400 dark:text-gray-500"> / </span>`
    + `<span class="text-gray-400 dark:text-gray-500" title="${t('admin.dashboard.standard')}">$${formatCost(s[`${prefix}_cost` as const])}</span>`
}

const tokenCards = computed<StatCard[]>(() => {
  if (!stats.value) return []
  const s = stats.value
  return [
    {
      label: t('admin.dashboard.todayTokens'),
      value: formatTokens(s.today_tokens),
      sub: costTriple('today'),
      icon: Coin,
      bgClass: 'bg-amber-100 dark:bg-amber-900/30',
      iconClass: 'text-amber-600 dark:text-amber-400'
    },
    {
      label: t('admin.dashboard.totalTokens'),
      value: formatTokens(s.total_tokens),
      sub: costTriple('total'),
      icon: Files,
      bgClass: 'bg-indigo-100 dark:bg-indigo-900/30',
      iconClass: 'text-indigo-600 dark:text-indigo-400'
    },
    {
      label: t('admin.dashboard.performance'),
      value: formatTokens(s.rpm),
      sub: `<span class="text-sm font-semibold text-violet-600 dark:text-violet-400">${formatTokens(s.tpm)}</span> <span class="text-gray-500 dark:text-gray-400">TPM</span>`,
      icon: Lightning,
      bgClass: 'bg-violet-100 dark:bg-violet-900/30',
      iconClass: 'text-violet-600 dark:text-violet-400'
    },
    {
      label: t('admin.dashboard.avgResponse'),
      value: formatDuration(s.average_duration_ms),
      sub: `${formatNumber(s.active_users)} ${t('admin.dashboard.activeUsers')}`,
      subClass: 'text-gray-500 dark:text-gray-400',
      icon: Timer,
      bgClass: 'bg-rose-100 dark:bg-rose-900/30',
      iconClass: 'text-rose-600 dark:text-rose-400'
    }
  ]
})

const goToUserUsage = (item: UserSpendingRankingItem) => {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(item.user_id),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

// Date range change handler
const onDateRangeChange = (range: string[] | null) => {
  if (!range || range.length !== 2) return
  startDate.value = range[0]
  endDate.value = range[1]

  // Auto-select granularity based on date range
  const start = new Date(range[0])
  const end = new Date(range[1])
  const daysDiff = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))

  // If range is 1 day, use hourly granularity
  if (daysDiff <= 1) {
    granularity.value = 'hour'
  } else {
    granularity.value = 'day'
  }

  loadChartData()
}

// Load data
const loadDashboardSnapshot = async (includeStats: boolean) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) {
    loading.value = true
  }
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      include_stats: includeStats,
      include_trend: true,
      include_model_stats: true,
      include_group_stats: false,
      include_users_trend: false
    })
    if (currentSeq !== chartLoadSeq) return
    if (includeStats && response.stats) {
      stats.value = response.stats
    }
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

const setUserTrendMetric = (metric: 'tokens' | 'actual_cost') => {
  if (userTrendMetric.value === metric) return
  userTrendMetric.value = metric
  userTrend.value = []
  loadUsersTrend()
}

const loadUsersTrend = async () => {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      limit: 12,
      metric: userTrendMetric.value
    })
    if (currentSeq !== usersTrendLoadSeq) return
    userTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== usersTrendLoadSeq) return
    console.error('Error loading users trend:', error)
    userTrend.value = []
  } finally {
    if (currentSeq === usersTrendLoadSeq) {
      userTrendLoading.value = false
    }
  }
}

const loadUserSpendingRanking = async () => {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      start_date: startDate.value,
      end_date: endDate.value,
      limit: 12
    })
    if (currentSeq !== rankingLoadSeq) return
    rankingItems.value = response.ranking || []
    rankingTotalActualCost.value = response.total_actual_cost || 0
    rankingTotalRequests.value = response.total_requests || 0
    rankingTotalTokens.value = response.total_tokens || 0
  } catch (error) {
    if (currentSeq !== rankingLoadSeq) return
    console.error('Error loading user spending ranking:', error)
    rankingItems.value = []
    rankingTotalActualCost.value = 0
    rankingTotalRequests.value = 0
    rankingTotalTokens.value = 0
    rankingError.value = true
  } finally {
    if (currentSeq === rankingLoadSeq) {
      rankingLoading.value = false
    }
  }
}

const loadDashboardStats = async () => {
  await Promise.all([
    loadDashboardSnapshot(true),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

const loadChartData = async () => {
  await Promise.all([
    loadDashboardSnapshot(false),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

onMounted(() => {
  void refreshBatchImageAccess()
  loadDashboardStats()
})
</script>

<style scoped>
.ep-stat-card :deep(.el-card__body) {
  padding: 16px;
}

.ep-quick-action {
  @apply flex items-center gap-3 rounded-lg bg-gray-50 p-3 transition-colors dark:bg-dark-800/50;
}
</style>
