<template>
  <div class="accounts-card grow min-h-0 bg-white dark:bg-dark-900 border border-t-0 border-gray-200/80 dark:border-dark-700/80 rounded-br-2xl rounded-bl-none shadow-sm overflow-hidden flex flex-col">
    <!-- Filter Toolbar -->
    <div class="p-3.5 border-b border-gray-100 dark:border-dark-800 bg-gray-50/50 dark:bg-dark-950/30">
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2.5">
        <!-- Search Input -->
        <el-input
          :model-value="filters.search"
          clearable
          :prefix-icon="Search"
          :placeholder="t('resourcePool.accounts.search')"
          class="filter-input"
          @update:model-value="updateSearch"
        />

        <!-- Platform Filter -->
        <el-select
          :model-value="filters.platform"
          clearable
          :placeholder="t('resourcePool.accounts.allPlatforms')"
          class="filter-select"
          @update:model-value="updatePlatform"
        >
          <el-option
            v-for="platform in platforms"
            :key="platform"
            :label="platformDisplayName(platform)"
            :value="platform"
          >
            <div class="flex items-center gap-2">
              <PlatformIcon :platform="platform" size="xs" />
              <span>{{ platformDisplayName(platform) }}</span>
            </div>
          </el-option>
        </el-select>

        <!-- Status Filter -->
        <el-select
          :model-value="filters.status"
          clearable
          :placeholder="t('resourcePool.accounts.allStatus')"
          class="filter-select"
          @update:model-value="updateStatus"
        >
          <el-option :label="t('resourcePool.status.healthy')" value="active" />
          <el-option :label="t('resourcePool.status.limited')" value="limited" />
          <el-option :label="t('resourcePool.status.error')" value="error" />
          <el-option :label="t('resourcePool.status.inactive')" value="inactive" />
        </el-select>

        <!-- Type Filter -->
        <el-select
          :model-value="filters.type"
          clearable
          :placeholder="t('resourcePool.accounts.allTypes')"
          class="filter-select"
          @update:model-value="updateType"
        >
          <el-option label="OAuth" value="oauth" />
          <el-option label="API Key" value="apikey" />
          <el-option label="Setup Token" value="setup-token" />
          <el-option label="Upstream" value="upstream" />
        </el-select>
      </div>

      <!-- Active Filter Reset Bar -->
      <div v-if="hasActiveFilters" class="flex items-center justify-between mt-2 pt-2 border-t border-gray-200/50 dark:border-dark-800 text-xs text-gray-500 dark:text-gray-400">
        <span class="flex items-center gap-1.5">
          <el-icon><Filter /></el-icon>
          <span>已应用筛选条件</span>
        </span>
        <button
          type="button"
          class="text-primary-600 dark:text-primary-400 hover:underline font-medium flex items-center gap-1"
          @click="resetAllFilters"
        >
          <el-icon><Close /></el-icon>
          {{ t('resourcePool.actions.resetFilters') }}
        </button>
      </div>
    </div>

    <!-- Floating Batch Selection Dock -->
    <transition name="el-zoom-in-top">
      <div
        v-if="selected.length"
        class="batch-bar m-3 mb-0 p-3 rounded-xl bg-gradient-to-r from-primary-50 to-blue-50/80 dark:from-primary-950/40 dark:to-blue-950/30 border border-primary-200/70 dark:border-primary-800/50 shadow-sm flex flex-wrap items-center justify-between gap-3"
      >
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-primary-500 animate-ping" />
          <span class="text-sm font-semibold text-primary-900 dark:text-primary-100">
            {{ t('resourcePool.accounts.selected', { count: selected.length }) }}
          </span>
        </div>
        <div class="flex items-center gap-2 flex-wrap">
          <button type="button" class="btn btn-danger btn-sm" @click="$emit('batch-delete', selected)">
            {{ t('admin.accounts.bulkActions.delete') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="$emit('batch-reset-status', selected)">
            {{ t('admin.accounts.bulkActions.resetStatus') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="$emit('batch-refresh-token', selected)">
            {{ t('admin.accounts.bulkActions.refreshToken') }}
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="probingSelected"
            @click="onProbeSelected"
          >
            <el-icon v-if="probingSelected" class="is-loading"><Loading /></el-icon>
            {{ t('admin.accounts.bulkActions.probeUpstreamBilling') }}
          </button>
          <button type="button" class="btn btn-success btn-sm" @click="$emit('batch-schedulable', selected, true)">
            {{ t('admin.accounts.bulkActions.enableScheduling') }}
          </button>
          <button type="button" class="btn btn-sm batch-stop-btn" @click="$emit('batch-schedulable', selected, false)">
            {{ t('admin.accounts.bulkActions.disableScheduling') }}
          </button>
          <button type="button" class="btn btn-primary btn-sm" @click="$emit('batch-edit-selected', selected)">
            {{ t('admin.accounts.bulkActions.edit') }}
          </button>
          <button type="button" class="btn btn-primary btn-sm" @click="$emit('batch-edit-filtered')">
            {{ t('admin.accounts.bulkEdit.submit') }}
          </button>
          <button v-if="groupId" type="button" class="btn btn-secondary btn-sm" @click="$emit('batch-remove', selected)">
            {{ t('resourcePool.actions.removeFromCurrent') }}
          </button>
          <el-button
            size="small"
            text
            @click="clearSelection"
          >
            {{ t('resourcePool.actions.clearSelection') }}
          </el-button>
        </div>
      </div>
    </transition>

    <!-- Table -->
    <div ref="tableAreaRef" class="p-3 flex-1 min-h-0 overflow-x-auto">
      <el-table
        ref="tableRef"
        v-loading="loading"
        :data="accounts"
        row-key="id"
        :height="isNarrow ? undefined : '100%'"
        class="resource-pool-table !rounded-xl"
        @selection-change="onSelectionChange"
        @sort-change="onSortChange"
      >
        <!-- Selection Checkbox -->
        <el-table-column type="selection" width="36" align="center" />

        <!-- Name -->
        <el-table-column prop="name" sortable="custom" :label="t('resourcePool.accounts.name')" :min-width="nameColumnMinWidth" fixed="left">
          <template #default="{ row }">
            <div class="flex items-center gap-2.5 py-1">
              <el-tooltip placement="right" :show-after="200">
                <span class="inline-flex shrink-0 cursor-help" tabindex="0">
                  <PlatformIcon
                    :platform="row.platform"
                    size="md"
                    :class="!row.schedulable
                      ? 'text-gray-900 dark:text-gray-100'
                      : accountAvailable(row)
                        ? 'text-emerald-500'
                        : 'text-red-500'"
                  />
                </span>
                <template #content>
                  <div class="max-h-[50vh] max-w-[min(360px,80vw)] space-y-1 overflow-y-auto whitespace-pre-wrap break-words text-xs leading-relaxed">
                    <div v-for="(reason, index) in accountStatusReasons(row)" :key="index">{{ reason }}</div>
                  </div>
                </template>
              </el-tooltip>
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <a
                    v-if="accountBaseUrl(row)"
                    :href="accountBaseUrl(row)"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="font-medium text-sm text-gray-900 dark:text-gray-100 truncate hover:text-primary-600 dark:hover:text-primary-400 hover:underline cursor-pointer"
                    :title="accountBaseUrl(row)"
                  >
                    {{ row.name }}
                  </a>
                  <span v-else class="font-medium text-sm text-gray-900 dark:text-gray-100 truncate" :title="row.name">
                    {{ row.name }}
                  </span>
                </div>

                <!-- Subtitle: Proxy + Notes -->
                <div class="flex items-center gap-1.5 mt-1 text-xs text-gray-400 dark:text-dark-400 flex-wrap">
                  <span v-if="row.proxy || row.proxy_id" class="px-1.5 py-0.2 rounded text-[11px] font-medium bg-sky-50 text-sky-700 dark:bg-sky-950/40 dark:text-sky-300 flex items-center gap-0.5">
                    <el-icon class="text-[10px]"><Link /></el-icon>
                    <span>{{ row.proxy?.name || '代理' }}</span>
                  </span>
                  <span v-if="row.notes" class="text-xs text-gray-400 truncate max-w-[160px]" :title="row.notes">
                    · {{ row.notes }}
                  </span>
                </div>
              </div>
            </div>
          </template>
        </el-table-column>

        <!-- Billing multiplier + upstream rate probe -->
        <el-table-column v-if="showRateColumn" prop="rate_multiplier" sortable="custom" :label="t('resourcePool.accounts.multiplier')" width="150" align="center">
          <template #header>
            <!-- inline-flex 保证与排序箭头同行,避免箭头被挤换行 -->
            <div class="inline-flex items-center gap-1">
              <span>{{ t('resourcePool.accounts.multiplier') }}</span>
              <button
                type="button"
                class="probe-trigger"
                :class="{ 'is-loading': batchProbing }"
                :title="t('resourcePool.actions.probeBatch')"
                :disabled="batchProbing"
                @click="onBatchProbe"
              >
                <el-icon v-if="batchProbing" class="is-loading"><Loading /></el-icon>
                <Icon v-else name="bolt" size="sm" />
              </button>
            </div>
          </template>
          <template #default="{ row }">
            <UpstreamBillingRateCell
              :account="row"
              :now="upstreamBillingNow"
              :probing="probingIds.has(row.id)"
              :global-probe-enabled="upstreamBillingProbeGloballyEnabled"
              @probe="onProbe(row)"
            />
          </template>
        </el-table-column>

        <!-- Priority (原系统快捷编辑组件) -->
        <el-table-column prop="priority" sortable="custom" class-name="priority-col" :label="t('admin.accounts.columns.priority')" width="150" align="center">
          <template #default="{ row }">
            <AccountPriorityCell
              :account="row"
              @updated="account => onPriorityUpdated(row, account)"
              @error="message => ElMessage.error(message)"
            />
          </template>
        </el-table-column>

        <!-- Scheduling -->
        <el-table-column :label="t('resourcePool.accounts.scheduling')" width="68" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.schedulable"
              :loading="schedulingAccountId === row.id"
              :aria-label="row.schedulable ? '停用调度' : '启用调度'"
              @change="setAccountSchedulable(row, Boolean($event))"
            />
          </template>
        </el-table-column>

        <!-- Groups -->
        <el-table-column label="分组" :min-width="groupColumnMinWidth" prop="groups" sortable :sort-by="(row: AccountListItem) => (row.groups ?? []).map(group => group.name).join(',')">
          <template #default="{ row }">
            <ResourcePoolMembershipCell
              :selected-ids="accountGroupIds(row)"
              :all-groups="availableGroups"
              :compatible-groups="compatibleGroups(row)"
              :account-multiplier="row.rate_multiplier ?? 1"
              :pending="pendingGroupSelections.has(row.id)"
              :disabled="savingGroupChanges"
              @group-click="id => emit('group-click', id)"
              @change="ids => onGroupSelectionChange(row, ids)"
              @remove="id => onGroupPillClose(row, id)"
              @menu-close="saveGroupChanges"
            />
          </template>
        </el-table-column>

        <!-- Capacity -->
        <el-table-column v-if="showCapacity" :label="t('resourcePool.accounts.capacity')" width="80">
          <template #default="{ row }">
            <AccountCapacityCell :account="row" />
          </template>
        </el-table-column>

        <!-- Usage windows (5h/7d 等滚动用量窗口，数据复用账号页的批量接口) -->
        <el-table-column
          v-if="showUsageWindows"
          :label="t('admin.accounts.columns.usageWindows')"
          min-width="190"
        >
          <template #header>
            <div class="inline-flex items-center gap-1">
              <span>{{ t('admin.accounts.columns.usageWindows') }}</span>
              <el-tooltip
                :content="t('admin.accounts.usageWindowsHint')"
                placement="top"
                :show-after="200"
              >
                <el-icon class="cursor-help text-gray-400"><QuestionFilled /></el-icon>
              </el-tooltip>
            </div>
          </template>
          <template #default="{ row }">
            <AccountUsageCell
              :account="row"
              :today-stats="todayStatsByAccountId[String(row.id)] ?? null"
              :today-stats-loading="todayStatsLoading"
              :batched-usage="usageWindowsByAccountId[String(row.id)] ?? null"
              :batched-usage-error="usageWindowsErrorByAccountId[String(row.id)] ?? null"
              :batched-usage-loading="usageWindowsLoadingByAccountId[String(row.id)] === true"
              :request-batched-usage="queueUsageWindows"
              @account-updated="account => onUsageAccountUpdated(row, account)"
              @usage-loaded="usage => onUsageWindowsLoaded(row.id, usage)"
            />
          </template>
        </el-table-column>

        <!-- Today's stats(点击列头按用户扣费在当前页内排序) -->
        <el-table-column
          v-if="showTodayStats"
          :label="t('resourcePool.accounts.todayStats')"
          min-width="160"
          prop="today_user_cost"
          sortable
          :sort-by="(row: AccountListItem) => todayStatsByAccountId[String(row.id)]?.user_cost ?? 0"
        >
          <template #default="{ row }">
            <AccountTodayStatsCell
              :stats="todayStatsByAccountId[String(row.id)] ?? null"
              :loading="todayStatsLoading"
              :error="todayStatsError"
            />
          </template>
        </el-table-column>

        <!-- Actions -->
        <el-table-column :label="t('common.actions')" width="132" fixed="right" align="center">
          <template #default="{ row }">
            <div class="action-cell">
              <el-button link size="small" class="action-button" @click="$emit('edit', row)">
                <el-icon class="action-button-icon"><Edit /></el-icon>
                <span class="action-button-label">{{ t('common.edit') }}</span>
              </el-button>
              <el-button link size="small" class="action-button delete-action" @click="onRowCommand('delete', row)">
                <el-icon class="action-button-icon"><Delete /></el-icon>
                <span class="action-button-label">{{ t('common.delete') }}</span>
              </el-button>
              <el-button link size="small" class="action-button" @click="openOriginalActionMenu(row, $event)">
                <el-icon class="action-button-icon"><MoreFilled /></el-icon>
                <span class="action-button-label">{{ t('common.more') }}</span>
              </el-button>
            </div>
          </template>
        </el-table-column>

        <!-- Empty State in Table -->
        <template #empty>
          <div class="py-12 px-4 text-center">
            <el-empty
              :image-size="64"
              :description="hasActiveFilters ? t('resourcePool.accounts.emptyFiltered') : t('resourcePool.accounts.emptyPool')"
            >
              <template #default>
                <div class="flex items-center justify-center gap-2 mt-2">
                  <el-button v-if="hasActiveFilters" size="small" plain @click="resetAllFilters">
                    {{ t('resourcePool.actions.resetFilters') }}
                  </el-button>
                  <template v-else>
                    <el-button v-if="groupId" size="small" type="primary" @click="$emit('add-existing')">
                      <el-icon class="mr-1"><Plus /></el-icon>
                      {{ t('resourcePool.actions.addExisting') }}
                    </el-button>
                    <el-button size="small" plain @click="$emit('create')">
                      <el-icon class="mr-1"><CirclePlus /></el-icon>
                      {{ t('resourcePool.actions.createAccount') }}
                    </el-button>
                  </template>
                </div>
              </template>
            </el-empty>
          </div>
        </template>
      </el-table>
    </div>

    <!-- Pagination Footer -->
    <div class="p-3 border-t border-gray-100 dark:border-dark-800 flex items-center justify-between flex-wrap gap-2">
      <div class="text-xs text-gray-400 dark:text-dark-400">
        {{ t('resourcePool.accounts.total', { count: total }) }}
      </div>
      <el-pagination
        background
        size="small"
        layout="total, sizes, prev, pager, next"
        :current-page="page"
        :page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="total"
        class="accounts-pagination"
        @update:current-page="changeCurrentPage"
        @update:page-size="changePageSize"
      />
    </div>

    <AccountActionMenu
      :show="rowMenuOpen"
      :account="rowMenuAccount"
      :anchor-rect="rowMenuAnchor"
      resource-pool-mode
      @close="rowMenuOpen = false"
      @test="runOriginalMenuAction('test')"
      @stats="runOriginalMenuAction('stats')"
      @schedule="$emit('schedule', $event)"
      @duplicate="runOriginalMenuAction('duplicate')"
      @recover-state="$emit('recover-state', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useElementSize, useIntervalFn, useMediaQuery } from '@vueuse/core'
import { ElMessage, ElMessageBox, type TableInstance } from 'element-plus'
import {
  CirclePlus,
  Close,
  Delete,
  Edit,
  Filter,
  Link,
  Loading,
  MoreFilled,
  Plus,
  QuestionFilled,
  Search
} from '@element-plus/icons-vue'
import { accountsAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import UpstreamBillingRateCell from '@/components/account/UpstreamBillingRateCell.vue'
import { Icon } from '@/components/icons'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'
import AccountCapacityCell from '@/components/account/AccountCapacityCell.vue'
import AccountPriorityCell from '@/components/account/AccountPriorityCell.vue'
import AccountUsageCell from '@/components/account/AccountUsageCell.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import AccountTodayStatsCell from '@/components/account/AccountTodayStatsCell.vue'
import ResourcePoolMembershipCell from './ResourcePoolMembershipCell.vue'
import type { Account, AccountListItem, AccountUsageInfo, AdminGroup, GroupPlatform, WindowStats } from '@/types'

export interface ResourcePoolAccountFilters {
  search: string
  platform: string
  status: string
  type: string
}

const props = defineProps<{
  accounts: AccountListItem[]
  total: number
  page: number
  pageSize: number
  loading: boolean
  filters: ResourcePoolAccountFilters
  groupId?: number
  availableGroups: AdminGroup[]
  /** 当前视图的列表过滤上下文(分组/异常监控),批量探测按它拉取全量账号 */
  scopeFilter?: { group?: string; status?: string }
}>()

const emit = defineEmits<{
  refresh: []
  create: []
  'add-existing': []
  'batch-remove': [accounts: AccountListItem[]]
  'batch-delete': [accounts: AccountListItem[]]
  'batch-reset-status': [accounts: AccountListItem[]]
  'batch-refresh-token': [accounts: AccountListItem[]]
  'batch-schedulable': [accounts: AccountListItem[], schedulable: boolean]
  'batch-edit-selected': [accounts: AccountListItem[]]
  'batch-edit-filtered': []
  'page-change': [page: number, pageSize: number]
  'filter-change': [filters: ResourcePoolAccountFilters]
  'sort-change': [prop: string, order: 'ascending' | 'descending' | null]
  'group-click': [groupId: number]
  edit: [account: AccountListItem]
  test: [account: AccountListItem]
  stats: [account: AccountListItem]
  schedule: [account: AccountListItem]
  'recover-state': [account: AccountListItem]
  'row-action': [command: string, account: AccountListItem]
  'membership-changed': []
}>()

const { t } = useI18n()
const isNarrow = useMediaQuery('(max-width: 960px)')
// 按容器实际宽度渐进显隐次要列,保证核心列永远不被固定操作列遮挡
// (固定列组合约 580px 起,容量 +130,今日统计 +160,用量窗口 +190)
const tableAreaRef = ref<HTMLElement>()
const { width: tableAreaWidth } = useElementSize(tableAreaRef)
const showRateColumn = computed(() => tableAreaWidth.value >= 600)
const showCapacity = computed(() => tableAreaWidth.value >= 730)
const showUsageWindows = computed(() => tableAreaWidth.value >= 1060)
const showTodayStats = computed(() => tableAreaWidth.value >= 890)
const nameColumnMinWidth = computed(() => (tableAreaWidth.value >= 730 ? 200 : 150))
const groupColumnMinWidth = computed(() => (tableAreaWidth.value >= 730 ? 170 : 130))
const tableRef = ref<TableInstance>()
const selected = ref<AccountListItem[]>([])
const pendingGroupSelections = ref(new Map<number, number[]>())
const pendingGroupAccounts = ref(new Map<number, AccountListItem>())
const savingGroupChanges = ref(false)
const todayStatsByAccountId = ref<Record<string, WindowStats>>({})
const todayStatsLoading = ref(false)
const todayStatsError = ref<string | null>(null)
const schedulingAccountId = ref<number | null>(null)
const rowMenuOpen = ref(false)
const rowMenuAccount = ref<AccountListItem | null>(null)
const rowMenuAnchor = ref<DOMRect | null>(null)
let todayStatsRequest = 0

const platforms: GroupPlatform[] = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  'bedrock',
  'vertex',
  'typesafe',
  'kimi',
  'zhipu',
  'deepseek',
  'opencode_go',
  'ollama',
  'minimax'
] as GroupPlatform[]

const hasActiveFilters = computed(() => {
  return Boolean(
    props.filters.search?.trim() ||
    props.filters.platform ||
    props.filters.status ||
    props.filters.type
  )
})

function updateFilter(key: keyof ResourcePoolAccountFilters, value: string) {
  emit('filter-change', { ...props.filters, [key]: value })
}

function updateSearch(value: string) {
  updateFilter('search', String(value ?? ''))
}

function updatePlatform(value: string) {
  updateFilter('platform', String(value ?? ''))
}

function updateStatus(value: string) {
  updateFilter('status', String(value ?? ''))
}

function updateType(value: string) {
  updateFilter('type', String(value ?? ''))
}

function resetAllFilters() {
  emit('filter-change', { search: '', platform: '', status: '', type: '' })
}

function onSelectionChange(rows: AccountListItem[]) {
  selected.value = rows
}

// 名称/倍率/优先级为后端排序(sortable="custom"),转发给父组件重查列表
function onSortChange({ prop, order }: { prop: string; order: 'ascending' | 'descending' | null }) {
  emit('sort-change', String(prop), order)
}

// —— 探测上游倍率:单个 + 批量,复用原系统 upstream-billing-probe API 与 UpstreamBillingRateCell ——
const probingIds = reactive(new Set<number>())
const batchProbing = ref(false)
const probingSelected = ref(false)

// 工具条"探测上游倍率":仅探测勾选的账号(与原系统一致,单批上限 20)
async function onProbeSelected() {
  if (probingSelected.value) return
  const ids = selected.value.filter(row => row.type === 'apikey').map(row => row.id)
  if (ids.length === 0) {
    ElMessage.error(t('admin.accounts.upstreamBilling.noEligibleAccounts'))
    return
  }
  if (ids.length > 20) {
    ElMessage.error(t('admin.accounts.upstreamBilling.batchLimit'))
    return
  }
  probingSelected.value = true
  ids.forEach(id => probingIds.add(id))
  try {
    const results = await accountsAPI.probeUpstreamBillingBatch(ids)
    const failed = results.filter(result => result.error).length
    if (failed > 0) {
      ElMessage.error(t('admin.accounts.upstreamBilling.batchPartial', { success: results.length - failed, failed }))
    } else {
      ElMessage.success(t('admin.accounts.upstreamBilling.batchCompleted', { count: results.length }))
    }
    if (results.some(result => result.snapshot)) emit('refresh')
  } catch (error: any) {
    ElMessage.error(extractApiErrorMessage(error, t('admin.accounts.upstreamBilling.probeFailed')))
  } finally {
    ids.forEach(id => probingIds.delete(id))
    probingSelected.value = false
  }
}
const upstreamBillingNow = ref(Date.now())
useIntervalFn(() => { upstreamBillingNow.value = Date.now() }, 60_000)
const upstreamBillingProbeGloballyEnabled = ref<boolean | undefined>(undefined)

onMounted(async () => {
  try {
    const settings = await accountsAPI.getUpstreamBillingProbeSettings()
    upstreamBillingProbeGloballyEnabled.value = settings.enabled
  } catch {
    // 全局探测开关状态未知时按默认(启用)展示
  }
})

async function onProbe(row: AccountListItem) {
  if (probingIds.has(row.id)) return
  probingIds.add(row.id)
  try {
    const result = await accountsAPI.probeUpstreamBilling(row.id)
    if (result.snapshot) {
      // 探测成功会把上游倍率写回账号(rate_multiplier 同步),刷新列表展示
      emit('refresh')
    } else if (result.error) {
      ElMessage.error(result.error)
    }
  } catch (error: any) {
    ElMessage.error(extractApiErrorMessage(error, t('admin.accounts.upstreamBilling.probeFailed')))
  } finally {
    probingIds.delete(row.id)
  }
}

async function onBatchProbe() {
  if (batchProbing.value) return
  batchProbing.value = true
  let success = 0
  let failed = 0
  try {
    // 拉取当前视图(分组/筛选)下的全部账号;仅 API Key 账号支持上游倍率探测
    const result = await accountsAPI.list(1, 1000, {
      group: props.scopeFilter?.group,
      search: props.filters.search || undefined,
      platform: props.filters.platform || undefined,
      status: props.scopeFilter?.status || props.filters.status || undefined,
      type: props.filters.type || undefined,
      lite: '1'
    })
    const ids = result.items.filter(row => row.type === 'apikey').map(row => row.id)
    if (ids.length === 0) {
      ElMessage.error('当前视图暂无 API Key 账号')
      return
    }
    // 后端单批上限 20 个,自动分批串行探测
    for (let index = 0; index < ids.length; index += 20) {
      const chunk = ids.slice(index, index + 20)
      chunk.forEach(id => probingIds.add(id))
      try {
        const results = await accountsAPI.probeUpstreamBillingBatch(chunk)
        const chunkFailed = results.filter(result => result.error).length
        success += results.length - chunkFailed
        failed += chunkFailed
      } catch {
        failed += chunk.length
      } finally {
        chunk.forEach(id => probingIds.delete(id))
      }
    }
    if (failed > 0) {
      ElMessage.error(t('admin.accounts.upstreamBilling.batchPartial', { success, failed }))
    } else {
      ElMessage.success(t('admin.accounts.upstreamBilling.batchCompleted', { count: success }))
    }
    if (success > 0) emit('refresh')
  } catch (error: any) {
    ElMessage.error(extractApiErrorMessage(error, t('admin.accounts.upstreamBilling.probeFailed')))
  } finally {
    batchProbing.value = false
  }
}

// Refreshes and pagination replace the current page data. Clear the selection
// so deleted or no longer visible accounts cannot remain in the batch bar.
watch(
  () => props.accounts,
  () => {
    if (selected.value.length > 0) clearSelection()
  }
)

watch(
  () => props.accounts.map(account => account.id).join(','),
  async () => {
    const requestId = ++todayStatsRequest
    const ids = props.accounts.map(account => account.id)
    todayStatsByAccountId.value = {}
    todayStatsError.value = null
    if (ids.length === 0) {
      todayStatsLoading.value = false
      return
    }

    todayStatsLoading.value = true
    try {
      const result = await accountsAPI.getBatchTodayStats(ids)
      if (requestId === todayStatsRequest) todayStatsByAccountId.value = result.stats
    } catch {
      if (requestId === todayStatsRequest) todayStatsError.value = t('resourcePool.accounts.todayStatsFailed')
    } finally {
      if (requestId === todayStatsRequest) todayStatsLoading.value = false
    }
  },
  { immediate: true }
)

// —— 用量窗口(5h/7d 等滚动窗口):复用账号页的批量用量接口与 AccountUsageCell ——
// 队列 + 5min 缓存 + 请求令牌与 AccountsView 保持一致;列被宽度阈值隐藏时跳过加载。
const usageWindowsByAccountId = ref<Record<string, AccountUsageInfo | null>>({})
const usageWindowsErrorByAccountId = ref<Record<string, string | null>>({})
const usageWindowsLoadingByAccountId = ref<Record<string, boolean>>({})
const usageWindowsRequestTokenByAccountId = ref<Record<string, number>>({})
const usageWindowsCache = new Map<number, { data: AccountUsageInfo; ts: number }>()
const USAGE_WINDOWS_CACHE_TTL = 5 * 60 * 1000
const pendingUsageWindowIds = new Set<number>()
let usageWindowsFlushTimer: ReturnType<typeof setTimeout> | null = null
let queuedUsageWindowsForce = false
let usageWindowsRequestToken = 0

// 与账号页 accountSupportsBatchUsage 同口径:批量接口仅覆盖这些平台/类型,
// 其余平台由 AccountUsageCell 内部按需主动查询(usage-loaded 回写)。
function accountSupportsUsageWindows(account: AccountListItem): boolean {
  if (account.platform === 'anthropic') {
    return account.type === 'oauth' || account.type === 'setup-token'
  }
  if (account.platform === 'gemini') return true
  if (account.platform === 'antigravity') return account.type === 'oauth'
  if (account.platform === 'openai') return account.type === 'oauth'
  if (account.platform === 'grok') return account.type === 'oauth'
  return false
}

function setUsageWindowsLoading(accountID: number, loadingState: boolean) {
  usageWindowsLoadingByAccountId.value = {
    ...usageWindowsLoadingByAccountId.value,
    [String(accountID)]: loadingState
  }
}

function setUsageWindowsState(accountID: number, usage: AccountUsageInfo | null, error: string | null) {
  const key = String(accountID)
  usageWindowsByAccountId.value = {
    ...usageWindowsByAccountId.value,
    [key]: usage
  }
  usageWindowsErrorByAccountId.value = {
    ...usageWindowsErrorByAccountId.value,
    [key]: error
  }
}

// 单元格主动查询(不支持批量的平台)成功后回写,同一份数据供下次渲染复用
function onUsageWindowsLoaded(accountID: number, usage: AccountUsageInfo) {
  if (usageWindowsByAccountId.value[String(accountID)] === usage) return
  setUsageWindowsState(accountID, usage, null)
}

// 单元格内部状态探测(ollama/opencode 用量等)回写行对象,避免整表刷新
function onUsageAccountUpdated(row: AccountListItem, account: Account) {
  Object.assign(row, account)
}

async function flushQueuedUsageWindows() {
  usageWindowsFlushTimer = null
  const accountIDs = Array.from(pendingUsageWindowIds)
  const force = queuedUsageWindowsForce
  pendingUsageWindowIds.clear()
  queuedUsageWindowsForce = false

  if (accountIDs.length === 0) return

  const requestTokensByAccount = accountIDs.reduce<Record<string, number>>((acc, accountID) => {
    acc[String(accountID)] = usageWindowsRequestTokenByAccountId.value[String(accountID)] ?? 0
    return acc
  }, {})

  try {
    const result = await accountsAPI.getBatchUsage(accountIDs, force)
    const usageMap = result.usage ?? {}
    const errorMap = result.errors ?? {}
    const now = Date.now()
    const nextUsage = { ...usageWindowsByAccountId.value }
    const nextErrors = { ...usageWindowsErrorByAccountId.value }
    const nextLoading = { ...usageWindowsLoadingByAccountId.value }

    for (const accountID of accountIDs) {
      const key = String(accountID)
      if ((usageWindowsRequestTokenByAccountId.value[key] ?? 0) !== requestTokensByAccount[key]) {
        continue
      }
      const usage = usageMap[key] ?? null
      nextUsage[key] = usage
      nextErrors[key] = errorMap[key] ?? null
      nextLoading[key] = false
      if (usage) {
        usageWindowsCache.set(accountID, { data: usage, ts: now })
      } else {
        usageWindowsCache.delete(accountID)
      }
    }

    usageWindowsByAccountId.value = nextUsage
    usageWindowsErrorByAccountId.value = nextErrors
    usageWindowsLoadingByAccountId.value = nextLoading
  } catch {
    const nextErrors = { ...usageWindowsErrorByAccountId.value }
    const nextLoading = { ...usageWindowsLoadingByAccountId.value }
    for (const accountID of accountIDs) {
      const key = String(accountID)
      if ((usageWindowsRequestTokenByAccountId.value[key] ?? 0) !== requestTokensByAccount[key]) {
        continue
      }
      nextErrors[key] = 'Failed'
      nextLoading[key] = false
    }
    usageWindowsErrorByAccountId.value = nextErrors
    usageWindowsLoadingByAccountId.value = nextLoading
  }
}

// AccountUsageCell 的 request-batched-usage 回调:带 5min 缓存去重后合并为一次批量请求
function queueUsageWindows(account: Account, options?: { force?: boolean }) {
  if (!showUsageWindows.value) return
  if (!accountSupportsUsageWindows(account as AccountListItem)) return

  const force = options?.force === true
  const cacheKey = account.id
  const key = String(cacheKey)

  if (force) {
    usageWindowsCache.delete(cacheKey)
  } else {
    const cached = usageWindowsCache.get(cacheKey)
    if (cached && Date.now() - cached.ts < USAGE_WINDOWS_CACHE_TTL) {
      setUsageWindowsState(cacheKey, cached.data, null)
      setUsageWindowsLoading(cacheKey, false)
      return
    }
  }

  usageWindowsErrorByAccountId.value = {
    ...usageWindowsErrorByAccountId.value,
    [key]: null
  }
  usageWindowsRequestTokenByAccountId.value = {
    ...usageWindowsRequestTokenByAccountId.value,
    [key]: ++usageWindowsRequestToken
  }
  setUsageWindowsLoading(cacheKey, true)
  pendingUsageWindowIds.add(cacheKey)
  queuedUsageWindowsForce = queuedUsageWindowsForce || force

  if (usageWindowsFlushTimer !== null) return
  usageWindowsFlushTimer = setTimeout(() => {
    void flushQueuedUsageWindows()
  }, 0)
}

// 页数据变化或列从隐藏变为可见时,为当前页支持的账号排队加载(缓存命中不重查)
watch(
  () => `${props.accounts.map(account => account.id).join(',')}|${showUsageWindows.value}`,
  () => {
    if (!showUsageWindows.value) return
    for (const account of props.accounts) {
      queueUsageWindows(account)
    }
  },
  { immediate: true }
)

function clearSelection() {
  tableRef.value?.clearSelection()
  selected.value = []
}

async function setAccountSchedulable(account: AccountListItem, schedulable: boolean) {
  if (schedulingAccountId.value !== null) return
  schedulingAccountId.value = account.id
  try {
    await accountsAPI.setSchedulable(account.id, schedulable)
    ElMessage.success(schedulable ? '已启用调度' : '已停用调度')
    emit('refresh')
  } catch {
    ElMessage.error('调度状态更新失败')
  } finally {
    schedulingAccountId.value = null
  }
}

function onRowCommand(command: string | number | object, account: AccountListItem) {
  emit('row-action', String(command), account)
}

function openOriginalActionMenu(account: AccountListItem, event: MouseEvent) {
  const target = event.currentTarget
  if (!(target instanceof HTMLElement)) return
  if (rowMenuOpen.value && rowMenuAccount.value?.id === account.id) {
    rowMenuOpen.value = false
    return
  }
  rowMenuAccount.value = account
  rowMenuAnchor.value = target.getBoundingClientRect()
  rowMenuOpen.value = true
}

function runOriginalMenuAction(command: string) {
  const account = rowMenuAccount.value
  rowMenuOpen.value = false
  if (!account) return
  if (command === 'test') emit('test', account)
  else if (command === 'stats') emit('stats', account)
  else emit('row-action', command, account)
}

function changeCurrentPage(value: number) {
  emit('page-change', value, props.pageSize)
}

function changePageSize(value: number) {
  emit('page-change', 1, value)
}

function persistedAccountGroupIds(account: AccountListItem): number[] {
  if (account.groups?.length) return account.groups.map(group => group.id)
  return Array.isArray(account.group_ids) ? [...account.group_ids] : []
}

function accountGroupIds(account: AccountListItem): number[] {
  return pendingGroupSelections.value.get(account.id) ?? persistedAccountGroupIds(account)
}

const pendingGroupChangeCount = computed(() => pendingGroupSelections.value.size)

function compatibleGroups(account: AccountListItem): AdminGroup[] {
  return props.availableGroups.filter(group =>
    group.platform === account.platform || group.platform === 'composite'
  )
}

function isMixedChannelConflict(error: unknown) {
  if (typeof error !== 'object' || error === null) return false
  const candidate = error as { status?: number; response?: { status?: number } }
  return candidate.status === 409 || candidate.response?.status === 409
}

async function onGroupSelectionChange(account: AccountListItem, value: unknown) {
  const nextGroupIds = Array.isArray(value)
    ? value.map(item => Number(item)).filter(item => Number.isFinite(item))
    : []
  const previousGroupIds = persistedAccountGroupIds(account)
  const addedGroupIds = nextGroupIds.filter(id => !previousGroupIds.includes(id))
  const removedGroupIds = previousGroupIds.filter(id => !nextGroupIds.includes(id))
  if (addedGroupIds.length === 0 && removedGroupIds.length === 0) {
    const nextPending = new Map(pendingGroupSelections.value)
    nextPending.delete(account.id)
    pendingGroupSelections.value = nextPending
    const nextAccounts = new Map(pendingGroupAccounts.value)
    nextAccounts.delete(account.id)
    pendingGroupAccounts.value = nextAccounts
    return
  }

  pendingGroupSelections.value = new Map(pendingGroupSelections.value).set(account.id, nextGroupIds)
  pendingGroupAccounts.value = new Map(pendingGroupAccounts.value).set(account.id, account)
}

async function onGroupPillClose(account: AccountListItem, groupId: number) {
  if (savingGroupChanges.value) return

  const selectedIds = accountGroupIds(account)
  if (!selectedIds.includes(groupId)) return

  const persistedIds = persistedAccountGroupIds(account)
  const nextIds = selectedIds.filter(id => id !== groupId)

  // Closing a newly staged pill only cancels that addition; it is not persisted yet.
  if (!persistedIds.includes(groupId)) {
    await onGroupSelectionChange(account, nextIds)
    return
  }

  savingGroupChanges.value = true
  try {
    await accountsAPI.removeGroupMembership(account.id, groupId)
    const updatedAccount: AccountListItem = {
      ...account,
      group_ids: persistedIds.filter(id => id !== groupId),
      groups: account.groups?.filter(group => group.id !== groupId),
      account_groups: account.account_groups?.filter(binding => binding.group_id !== groupId)
    }
    const nextPending = new Map(pendingGroupSelections.value)
    if (nextIds.length === persistedAccountGroupIds(updatedAccount).length &&
      nextIds.every(id => persistedAccountGroupIds(updatedAccount).includes(id))) {
      nextPending.delete(account.id)
    } else {
      nextPending.set(account.id, nextIds)
      pendingGroupAccounts.value = new Map(pendingGroupAccounts.value).set(account.id, updatedAccount)
    }
    pendingGroupSelections.value = nextPending
    if (!nextPending.has(account.id)) {
      const nextAccounts = new Map(pendingGroupAccounts.value)
      nextAccounts.delete(account.id)
      pendingGroupAccounts.value = nextAccounts
    }
    emit('membership-changed')
  } catch {
    ElMessage.error(t('resourcePool.accounts.membershipRemoveFailed'))
  } finally {
    savingGroupChanges.value = false
  }
}

async function confirmMixedChannelRisk(account: AccountListItem, groupId: number): Promise<boolean> {
  try {
    const risk = await accountsAPI.checkMixedChannelRisk({
      platform: account.platform,
      group_ids: [groupId],
      account_id: account.id
    })
    if (!risk.has_risk) return false
  } catch (error) {
    if (!isMixedChannelConflict(error)) throw error
  }
  await ElMessageBox.confirm(
    t('resourcePool.assign.mixedWarning', { name: account.name }),
    t('resourcePool.assign.riskTitle'),
    {
      type: 'warning',
      confirmButtonText: t('resourcePool.assign.continue'),
      cancelButtonText: t('common.cancel')
    }
  )
  return true
}

async function saveGroupChanges() {
  if (savingGroupChanges.value || pendingGroupChangeCount.value === 0) return
  savingGroupChanges.value = true
  let success = 0
  let failed = 0
  let changed = false
  try {
    for (const [accountId, nextGroupIds] of pendingGroupSelections.value) {
      const account = pendingGroupAccounts.value.get(accountId)
      if (!account) continue
      const previousGroupIds = persistedAccountGroupIds(account)
      const addedGroupIds = nextGroupIds.filter(id => !previousGroupIds.includes(id))
      const removedGroupIds = previousGroupIds.filter(id => !nextGroupIds.includes(id))
      try {
        for (const groupId of addedGroupIds) {
          const confirmRisk = await confirmMixedChannelRisk(account, groupId)
          await accountsAPI.setGroupMembership(account.id, groupId, {
            priority: 1,
            confirm_mixed_channel_risk: confirmRisk
          })
        }
        for (const groupId of removedGroupIds) {
          await accountsAPI.removeGroupMembership(account.id, groupId)
        }
        const nextPending = new Map(pendingGroupSelections.value)
        nextPending.delete(accountId)
        pendingGroupSelections.value = nextPending
        const nextAccounts = new Map(pendingGroupAccounts.value)
        nextAccounts.delete(accountId)
        pendingGroupAccounts.value = nextAccounts
        success++
        changed = true
      } catch {
        failed++
      }
    }
  } finally {
    savingGroupChanges.value = false
  }

  if (changed) emit('membership-changed')
  if (failed > 0) {
    ElMessage.warning(t('resourcePool.accounts.groupsSavePartial', { success, failed }))
  } else if (success > 0) {
    ElMessage.success(t('resourcePool.accounts.groupsSaved'))
  }
}

function accountAvailable(account: AccountListItem) {
  return account.status === 'active' && account.schedulable && !account.rate_limit_reset_at && !account.temp_unschedulable_until
}

function accountStatusReasons(account: AccountListItem): string[] {
  const reasons: string[] = []
  if (!account.schedulable) reasons.push(t('admin.accounts.schedulableDisabled'))
  if (account.status === 'inactive') reasons.push(t('admin.accounts.status.inactive'))
  if (account.status === 'error') {
    reasons.push(t('admin.accounts.status.error'))
    if (account.error_message?.trim()) reasons.push(account.error_message.trim())
  }
  if (account.rate_limit_reset_at) {
    reasons.push(t('admin.accounts.status.rateLimitedUntil', {
      time: formatDateTime(account.rate_limit_reset_at)
    }))
  }
  if (account.temp_unschedulable_until) {
    reasons.push(t('admin.accounts.status.tempUnschedulable'))
    const reason = account.temp_unschedulable_reason?.trim()
    if (reason) {
      // 临时停调原因可能是旧版纯文本，也可能是后端序列化的 TempUnschedState。
      let description = reason
      try {
        const state: unknown = JSON.parse(reason)
        if (state && typeof state === 'object') {
          const details = state as Record<string, unknown>
          const lines: string[] = []
          if (typeof details.status_code === 'number' && details.status_code > 0) {
            lines.push(`${t('admin.accounts.tempUnschedulable.errorCode')}: ${details.status_code}`)
          }
          if (typeof details.error_message === 'string' && details.error_message.trim()) {
            lines.push(details.error_message.trim())
          } else if (typeof details.matched_keyword === 'string' && details.matched_keyword.trim()) {
            lines.push(`${t('admin.accounts.tempUnschedulable.keywords')}: ${details.matched_keyword.trim()}`)
          }
          if (lines.length) description = lines.join('\n')
        }
      } catch {
        // 非 JSON 的旧版原因直接显示原文。
      }
      reasons.push(description)
    }
    reasons.push(t('admin.accounts.status.tempUnschedulableUntil', {
      time: formatDateTime(account.temp_unschedulable_until)
    }))
  }
  return reasons.length ? [...new Set(reasons)] : [t('admin.accounts.status.active')]
}

function onPriorityUpdated(row: AccountListItem, account: Account) {
  // 优先级快捷保存成功后局部更新行数据,避免整表刷新
  row.priority = account.priority
}

// 悬停账号名称时展示转发端点(credentials 中非敏感字段,列表响应已包含)
function accountBaseUrl(account: AccountListItem): string | undefined {
  const baseUrl = account.credentials?.base_url
  return typeof baseUrl === 'string' && baseUrl.trim() ? baseUrl.trim() : undefined
}

function platformDisplayName(platform: string) {
  const map: Record<string, string> = {
    anthropic: 'Anthropic Claude',
    openai: 'OpenAI',
    gemini: 'Google Gemini',
    antigravity: 'Antigravity',
    grok: 'xAI Grok',
    bedrock: 'AWS Bedrock',
    vertex: 'Google Vertex AI',
    typesafe: 'TypeSafe',
    kimi: 'Moonshot Kimi',
    zhipu: '智谱 GLM',
    deepseek: 'DeepSeek',
    opencode_go: 'OpenCode Go',
    ollama: 'Ollama',
    minimax: 'MiniMax'
  }
  return map[platform] ?? platform
}

</script>

<style scoped>
.accounts-card {
  position: relative;
}

/* 批量停止调度:明确橙色实底(覆盖原系统 btn-warning 的琥珀渐变) */
.batch-stop-btn {
  background: linear-gradient(to right, #f97316, #ea580c);
  color: #fff;
  box-shadow: 0 4px 6px -1px rgba(249, 115, 22, 0.25);
}

.batch-stop-btn:hover {
  background: linear-gradient(to right, #ea580c, #c2410c);
  color: #fff;
}

/* —— 表格精修:通过 el-table CSS 变量统一控制层级与分隔线 —— */
.resource-pool-table {
  --el-table-border-color: rgba(15, 23, 42, 0.06);
  --el-table-header-bg-color: #fcfcfd;
  --el-table-header-text-color: #64748b;
  --el-table-text-color: #334155;
  --el-table-row-hover-bg-color: #f8fafc;
  font-size: 13px;
}

:global(.dark) .resource-pool-table {
  /* 固定列 sticky 需要不透明底色,与 accounts-card 的 dark-900 一致 */
  --el-bg-color: #0f172a;
  --el-table-bg-color: #0f172a;
  --el-table-tr-bg-color: #0f172a;
  --el-table-border-color: rgba(148, 163, 184, 0.14);
  --el-table-header-bg-color: #0f172a;
  --el-table-header-text-color: #94a3b8;
  --el-table-text-color: #cbd5e1;
  --el-table-row-hover-bg-color: #1e293b;
}

:deep(.el-table th.el-table__cell) {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;
  white-space: nowrap;
  text-align: left;
}

/* 表头单元格内层容器统一左对齐,覆盖个别列(如居中列)的表头默认对齐 */
:deep(.el-table th.el-table__cell .cell) {
  text-align: left;
}

/* 优先级列表头例外:与列内容一致居中 */
:deep(.el-table th.el-table__cell.priority-col),
:deep(.el-table th.el-table__cell.priority-col .cell) {
  text-align: center;
}

:deep(.el-table .el-table__cell) {
  padding-top: 6px;
  padding-bottom: 6px;
}

.action-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 2px;
}

:deep(.action-button) {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1px;
  min-width: 36px;
  height: 42px;
  padding: 2px 4px;
  line-height: 1.15;
  color: var(--el-text-color-regular);
}

/* Element Plus 默认给相邻按钮加 margin-left: 12px,间距统一由 .action-cell 的 gap 控制 */
:deep(.action-button + .action-button) {
  margin-left: 0;
}

/* el-button 会把插槽内容包进一个内层 span,图标/文字的竖排要作用在这一层 */
:deep(.action-button > span) {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

:deep(.action-button:hover) {
  color: var(--el-color-primary);
}

:deep(.action-button.delete-action:hover) {
  color: var(--el-color-danger);
}

:deep(.action-button-icon) {
  margin: 0;
  font-size: 18px;
}

:deep(.action-button-label) {
  display: block;
  white-space: nowrap;
}

/* 探测按钮(倍率列):单个/批量 */
.probe-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 0.375rem;
  color: #94a3b8;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.probe-trigger:hover:not(:disabled) {
  background-color: rgba(241, 245, 249, 0.9);
  color: var(--el-color-primary);
}

:global(.dark) .probe-trigger:hover:not(:disabled) {
  background-color: rgba(30, 41, 59, 0.7);
  color: #60a5fa;
}

.probe-trigger:disabled {
  cursor: not-allowed;
  color: var(--el-color-primary);
}

.group-tag {
  cursor: pointer;
}

:deep(.priority-input .el-input__wrapper) {
  padding-left: 8px;
  padding-right: 24px;
}
</style>
