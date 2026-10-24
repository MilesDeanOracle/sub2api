<template>
  <el-drawer
    :model-value="show"
    :title="t('admin.users.drawer.title')"
    direction="rtl"
    size="80%"
    append-to-body
    destroy-on-close
    @update:model-value="(v: boolean) => !v && emit('close')"
    @open="load"
  >
    <div v-if="user" class="flex h-full flex-col gap-5">
      <!-- 用户信息头 -->
      <div class="flex items-center gap-4 rounded-2xl bg-primary-50 p-5 dark:bg-primary-900/30">
        <div class="flex h-14 w-14 items-center justify-center rounded-full bg-white shadow-sm dark:bg-dark-700">
          <span class="text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <p class="truncate text-lg font-semibold text-gray-900 dark:text-white">{{ user.email }}</p>
            <el-tag :type="user.role === 'admin' ? 'warning' : 'info'" size="small" round disable-transitions>
              {{ t('admin.users.roles.' + user.role) }}
            </el-tag>
            <el-tag :type="user.status === 'active' ? 'success' : 'danger'" size="small" disable-transitions>
              {{ user.status === 'active' ? t('common.active') : t('admin.users.disabled') }}
            </el-tag>
          </div>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
            <span class="font-medium">{{ user.username || '-' }}</span>
            <span class="mx-2 text-gray-300 dark:text-dark-500">•</span>
            ID: {{ user.id }}
          </p>
        </div>
      </div>

      <!-- 数字卡片：充值与消费概览 -->
      <div v-loading="overviewLoading" class="grid grid-cols-2 gap-4 sm:grid-cols-3 xl:grid-cols-6">
        <el-card v-for="card in statCards" :key="card.label" shadow="never" class="ep-stat-card">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</div>
          <div class="mt-2 truncate text-xl font-bold text-gray-900 dark:text-white" :title="card.fullValue ?? card.value">
            {{ card.value }}
          </div>
          <div v-if="card.sub" class="mt-1 truncate text-xs text-gray-400">{{ card.sub }}</div>
        </el-card>
      </div>

      <!-- 分组 / 模型用量统计（可展开行） -->
      <el-card shadow="never" class="flex min-h-0 flex-1 flex-col" :body-style="{ display: 'flex', flexDirection: 'column', minHeight: 0, flex: 1, padding: '16px' }">
        <template #header>
          <div class="flex items-center gap-2">
            <span class="font-medium">{{ t('admin.users.drawer.modelStats') }}</span>
            <el-tag v-if="stats" size="small" type="info" effect="plain" round disable-transitions>
              {{ stats.groups.length }}
            </el-tag>
          </div>
        </template>

        <div v-loading="statsLoading" class="min-h-0 flex-1 overflow-auto">
          <el-table
            v-if="stats && stats.groups.length > 0"
            :data="stats.groups"
            row-key="group_id"
            size="default"
            :default-expand-all="stats.groups.length <= 3"
          >
            <el-table-column type="expand">
              <template #default="{ row }">
                <!-- 展开行：该分组下的模型明细 -->
                <el-table
                  :data="row.models"
                  size="small"
                  class="ep-nested-table"
                  :empty-text="t('admin.users.drawer.noData')"
                >
                  <el-table-column :label="t('admin.users.drawer.colName')" min-width="220">
                    <template #default="{ row: model }">
                      <span class="font-mono text-xs">{{ model.model }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column :label="t('admin.users.drawer.colRate')" min-width="90" align="right">
                    <template #default="{ row: model }">
                      <span v-if="model.rate_multiplier != null">{{ model.rate_multiplier }}x</span>
                      <span v-else class="text-gray-400">-</span>
                    </template>
                  </el-table-column>
                  <el-table-column :label="t('admin.users.drawer.colRequests')" min-width="90" align="right">
                    <template #default="{ row: model }">{{ (model.requests ?? 0).toLocaleString() }}</template>
                  </el-table-column>
                  <el-table-column :label="t('admin.users.drawer.colTokens')" min-width="110" align="right">
                    <template #default="{ row: model }">{{ formatTokens(model.total_tokens) }}</template>
                  </el-table-column>
                  <el-table-column :label="t('admin.users.drawer.colActual')" min-width="100" align="right">
                    <template #default="{ row: model }">{{ formatUsd(model.actual_cost) }}</template>
                  </el-table-column>
                  <el-table-column :label="t('admin.users.drawer.colCost')" min-width="100" align="right">
                    <template #default="{ row: model }">{{ formatUsd(model.account_cost) }}</template>
                  </el-table-column>
                  <el-table-column :label="t('admin.users.drawer.colProfit')" min-width="100" align="right">
                    <template #default="{ row: model }">
                      <span :class="profitClass(model.actual_cost, model.account_cost)">{{ formatProfit(model.actual_cost, model.account_cost) }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column v-if="!compat" :label="t('admin.users.drawer.colTtft')" min-width="110" align="right">
                    <template #default="{ row: model }">{{ formatMs(model.ttft_p95_ms) }}</template>
                  </el-table-column>
                  <el-table-column :label="compat ? t('admin.users.drawer.colAvgDuration') : t('admin.users.drawer.colDuration')" min-width="110" align="right">
                    <template #default="{ row: model }">{{ formatSeconds(model.duration_p95_ms) }}</template>
                  </el-table-column>
                  <el-table-column :label="compat ? t('admin.users.drawer.colAvgTps') : t('admin.users.drawer.colTps')" min-width="130" align="right">
                    <template #default="{ row: model }">{{ formatTps(model.output_tps_p95) }}</template>
                  </el-table-column>
                </el-table>
              </template>
            </el-table-column>

            <el-table-column :label="t('admin.users.drawer.colName')" min-width="220">
              <template #default="{ row }">
                <div class="flex items-center gap-2">
                  <el-tag
                    v-if="row.platform"
                    size="small"
                    effect="plain"
                    round
                    disable-transitions
                    :class="platformTagClass(row.platform)"
                  >
                    {{ row.platform }}
                  </el-tag>
                  <span class="font-medium text-gray-900 dark:text-white">
                    {{ row.group_name || t('admin.users.drawer.groupUnknown') }}
                  </span>
                  <span class="text-xs text-gray-400">({{ row.models.length }})</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="t('admin.users.drawer.colRate')" min-width="90" align="right">
              <template #default="{ row }">
                <span v-if="row.rate_multiplier != null">{{ row.rate_multiplier }}x</span>
                <span v-else class="text-gray-400">-</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('admin.users.drawer.colRequests')" min-width="90" align="right">
              <template #default="{ row }">{{ (row.requests ?? 0).toLocaleString() }}</template>
            </el-table-column>
            <el-table-column :label="t('admin.users.drawer.colTokens')" min-width="110" align="right">
              <template #default="{ row }">{{ formatTokens(row.total_tokens) }}</template>
            </el-table-column>
            <el-table-column :label="t('admin.users.drawer.colActual')" min-width="100" align="right">
              <template #default="{ row }">{{ formatUsd(row.actual_cost) }}</template>
            </el-table-column>
            <el-table-column :label="t('admin.users.drawer.colCost')" min-width="100" align="right">
              <template #default="{ row }">{{ formatUsd(row.account_cost) }}</template>
            </el-table-column>
            <el-table-column :label="t('admin.users.drawer.colProfit')" min-width="100" align="right">
              <template #default="{ row }">
                <span :class="profitClass(row.actual_cost, row.account_cost)">{{ formatProfit(row.actual_cost, row.account_cost) }}</span>
              </template>
            </el-table-column>
            <el-table-column v-if="!compat" :label="t('admin.users.drawer.colTtft')" min-width="110" align="right">
              <template #default="{ row }">{{ formatMs(row.ttft_p95_ms) }}</template>
            </el-table-column>
            <el-table-column :label="compat ? t('admin.users.drawer.colAvgDuration') : t('admin.users.drawer.colDuration')" min-width="110" align="right">
              <template #default="{ row }">{{ formatSeconds(row.duration_p95_ms) }}</template>
            </el-table-column>
            <el-table-column :label="compat ? t('admin.users.drawer.colAvgTps') : t('admin.users.drawer.colTps')" min-width="130" align="right">
              <template #default="{ row }">{{ formatTps(row.output_tps_p95) }}</template>
            </el-table-column>
          </el-table>

          <el-empty v-else :description="t('admin.users.drawer.noData')" :image-size="90" />
        </div>
      </el-card>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import type { UserOverview, UserModelUsageStats, UserModelUsageStat } from '@/api/admin/users'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ show: boolean, user: AdminUser | null }>()
const emit = defineEmits(['close'])

const { t } = useI18n()

const overview = ref<UserOverview | null>(null)
const overviewLoading = ref(false)
const stats = ref<UserModelUsageStats | null>(null)
const statsLoading = ref(false)
// compat=true 表示数据来自现有接口拼装：延迟列显示"平均值"而非 P95，且无首字列
const compat = ref(false)

// 数据加载：优先使用专用新接口（/overview、/model-stats，含 P95 指标）；
// 旧后端没有这两个接口（404）时，自动回退到现有接口拼装数据：
//   - 概览卡片：balance-history(type=balance) + total_recharged
//   - 统计表格：dashboard groups + 按分组拉 model stats（无法提供 P95，显示 "-"）
const load = async () => {
  if (!props.user) return
  const userId = props.user.id
  overviewLoading.value = true
  statsLoading.value = true
  compat.value = false
  try {
    try {
      overview.value = await adminAPI.users.getUserOverview(userId)
    } catch (e) {
      console.warn('overview 接口不可用（旧后端），回退 balance-history:', e)
      overview.value = await loadOverviewCompat(userId)
    }
    try {
      stats.value = await adminAPI.users.getUserModelStats(userId)
    } catch (e) {
      console.warn('model-stats 接口不可用（旧后端），回退 dashboard 统计:', e)
      stats.value = await loadStatsCompat(userId)
      compat.value = true
    }
  } finally {
    overviewLoading.value = false
    statsLoading.value = false
  }
}

// 兼容模式概览：balance-history 按 used_at 倒序分页，
// 第一页首条为最后一次充值，末页末条为第一次充值；total 即充值次数，总额用 total_recharged。
const loadOverviewCompat = async (userId: number): Promise<UserOverview> => {
  const first = await adminAPI.users.getUserBalanceHistory(userId, 1, 20, 'balance')
  const items = first.items ?? []
  let firstAt: string | null = null
  if (first.pages > 1 && first.total > 0) {
    const last = await adminAPI.users.getUserBalanceHistory(userId, first.pages, 20, 'balance')
    const lastItems = last.items ?? []
    if (lastItems.length > 0) {
      firstAt = lastItems[lastItems.length - 1].used_at ?? lastItems[lastItems.length - 1].created_at
    }
  } else if (items.length > 0) {
    firstAt = items[items.length - 1].used_at ?? items[items.length - 1].created_at
  }
  const lastAt = items.length > 0 ? (items[0].used_at ?? items[0].created_at) : null
  return {
    recharge: {
      first_at: firstAt,
      last_at: lastAt,
      count: first.total ?? 0,
      total_amount: first.total_recharged ?? 0
    },
    usage: { total_requests: 0, total_tokens: 0, total_actual_cost: 0, total_account_cost: 0 }
  }
}

// 兼容模式统计：dashboard 分组汇总（全时间段），再按分组逐个拉模型统计。
// 现有接口无 P95 数据，三个 P95 列显示 "-"；总花费（各分组实际金额之和）回填概览卡片。
const ALL_TIME_START = '2000-01-01'

// 分组倍率：native 模式由后端返回；compat 模式从分组列表接口取配置倍率
const groupRateMap = async (): Promise<Map<number, number>> => {
  try {
    const list = await adminAPI.groups.getAll()
    return new Map((list ?? []).map((g) => [g.id, g.rate_multiplier]))
  } catch {
    return new Map()
  }
}

const loadStatsCompat = async (userId: number): Promise<UserModelUsageStats> => {
  const [groupsRes, rateMap] = await Promise.all([
    adminAPI.dashboard.getGroupStats({ user_id: userId, start_date: ALL_TIME_START }),
    groupRateMap()
  ])
  const groups = [...(groupsRes.groups ?? [])].sort((a, b) => (b.actual_cost ?? 0) - (a.actual_cost ?? 0))
  const modelLists = await Promise.all(groups.map((g) =>
    adminAPI.dashboard
      .getModelStats({ user_id: userId, group_id: g.group_id, start_date: ALL_TIME_START })
      .then((r) => (r.models ?? []).map((m): UserModelUsageStat => ({
        model: m.model,
        requests: m.requests,
        total_tokens: m.total_tokens,
        actual_cost: m.actual_cost,
        account_cost: m.account_cost ?? 0,
        rate_multiplier: null,
        ttft_p95_ms: null,
        duration_p95_ms: null,
        output_tps_p95: null
      })))
      .catch(() => [])
  ))

  // 逐模型拉 usage/stats（user+group+model 过滤）补延迟平均值：
  // average_duration_ms 即平均总耗时；输出速率 = 总输出tokens / 总耗时（总量速率）。
  // 首字均值无现有接口可取（ops 的 TTFT 聚合不支持按用户过滤），compat 模式隐藏该列。
  const latencyLists = await Promise.all(groups.map((g, i) =>
    Promise.all(modelLists[i].map(async (m) => {
      try {
        const s = await adminAPI.usage.getStats({
          user_id: userId,
          group_id: g.group_id,
          model: m.model,
          start_date: ALL_TIME_START
        })
        const avgDurationMs = s.average_duration_ms ?? 0
        const totalDurationSec = (avgDurationMs * (s.total_requests ?? 0)) / 1000
        const avgTps = totalDurationSec > 0 ? (s.total_output_tokens ?? 0) / totalDurationSec : null
        return { avgDurationMs: avgDurationMs > 0 ? avgDurationMs : null, avgTps }
      } catch {
        return { avgDurationMs: null, avgTps: null }
      }
    }))
  ))

  const spend = groups.reduce((sum, g) => sum + (g.actual_cost ?? 0), 0)
  if (overview.value) {
    overview.value.usage.total_actual_cost = spend
  } else {
    overview.value = {
      recharge: { first_at: null, last_at: null, count: 0, total_amount: 0 },
      usage: { total_requests: 0, total_tokens: 0, total_actual_cost: spend, total_account_cost: 0 }
    }
  }

  return {
    groups: groups.map((g, i) => {
      // 分组级：平均耗时按请求数加权，输出速率 = 总输出tokens / 总耗时
      let durationSum = 0
      let requestSum = 0
      for (let j = 0; j < modelLists[i].length; j++) {
        const m = modelLists[i][j]
        const lat = latencyLists[i][j]
        if (lat.avgDurationMs && m.requests > 0) {
          durationSum += lat.avgDurationMs * m.requests
          requestSum += m.requests
        }
        m.duration_p95_ms = lat.avgDurationMs
        m.output_tps_p95 = lat.avgTps
      }
      const groupAvgMs = requestSum > 0 ? durationSum / requestSum : null
      return {
        group_id: g.group_id,
        group_name: g.group_name,
        platform: '',
        requests: g.requests,
        total_tokens: g.total_tokens,
        actual_cost: g.actual_cost,
        account_cost: g.account_cost ?? 0,
        rate_multiplier: rateMap.get(g.group_id) ?? null,
        ttft_p95_ms: null,
        duration_p95_ms: groupAvgMs,
        output_tps_p95: null,
        models: modelLists[i]
      }
    })
  }
}

// ==================== 数字卡片 ====================
interface StatCard {
  label: string
  value: string
  fullValue?: string
  sub?: string
}

const statCards = computed<StatCard[]>(() => {
  const r = overview.value?.recharge
  const u = overview.value?.usage
  return [
    {
      label: t('admin.users.drawer.firstRecharge'),
      value: r?.first_at ? formatDateTime(r.first_at) : t('admin.users.drawer.never'),
      fullValue: r?.first_at ? formatDateTime(r.first_at) : undefined
    },
    {
      label: t('admin.users.drawer.lastRecharge'),
      value: r?.last_at ? formatDateTime(r.last_at) : t('admin.users.drawer.never'),
      fullValue: r?.last_at ? formatDateTime(r.last_at) : undefined
    },
    {
      label: t('admin.users.drawer.rechargeCount'),
      value: String(r?.count ?? 0),
      sub: r?.count ? `${r.count} ${t('admin.users.drawer.times')}` : undefined
    },
    {
      label: t('admin.users.drawer.rechargeTotal'),
      value: `$${(r?.total_amount ?? 0).toFixed(2)}`
    },
    {
      label: t('admin.users.drawer.balance'),
      value: `$${(props.user?.balance ?? 0).toFixed(2)}`
    },
    {
      label: t('admin.users.drawer.totalSpend'),
      value: `$${(u?.total_actual_cost ?? 0).toFixed(2)}`
    }
  ]
})

// ==================== 格式化 ====================

// Token 数：超过 10 亿用 B，超过 10 万用 M，其余千分位
const formatTokens = (v: number): string => {
  if (v == null || Number.isNaN(v)) return '-'
  if (v >= 1e9) return `${(v / 1e9).toFixed(2)}B`
  if (v >= 1e5) return `${(v / 1e6).toFixed(2)}M`
  return v.toLocaleString()
}

const formatUsd = (v: number): string => {
  if (v == null || Number.isNaN(v)) return '-'
  return `$${v.toFixed(4)}`
}

// 首字 P95：毫秒
const formatMs = (v: number | null): string => {
  if (v == null) return '-'
  return `${Math.round(v)} ms`
}

// 总耗时 P95：毫秒转秒
const formatSeconds = (v: number | null): string => {
  if (v == null) return '-'
  return `${(v / 1000).toFixed(2)} s`
}

// 利润 = 实际金额 - 账号成本（负值标红，正值标绿）
const formatProfit = (actual: number, accountCost: number): string => {
  if (actual == null || accountCost == null) return '-'
  return formatUsd(actual - accountCost)
}

const profitClass = (actual: number, accountCost: number): string => {
  if (actual == null || accountCost == null) return ''
  const profit = actual - accountCost
  if (profit > 0) return 'text-emerald-600 dark:text-emerald-400'
  if (profit < 0) return 'text-red-600 dark:text-red-400'
  return ''
}

// 输出 TPS P95
const formatTps = (v: number | null): string => {
  if (v == null) return '-'
  return `${v.toFixed(1)} tok/s`
}

// 分组行平台标签配色（与 GroupBadge 平台色一致）
const PLATFORM_TAG_CLASSES: Record<string, string> = {
  anthropic: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  openai: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  gemini: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  antigravity: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400',
  grok: 'bg-zinc-200 text-zinc-800 dark:bg-zinc-700 dark:text-zinc-100',
  kimi: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',
  zhipu: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
  deepseek: 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400',
  minimax: 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
  composite: 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/30 dark:text-cyan-300'
}

const platformTagClass = (platform: string) =>
  PLATFORM_TAG_CLASSES[platform] ?? 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400'
</script>

<style scoped>
.ep-stat-card :deep(.el-card__body) {
  padding: 14px 16px;
}

.ep-nested-table {
  margin: 4px 0 4px 48px;
  width: calc(100% - 48px);
}
</style>
