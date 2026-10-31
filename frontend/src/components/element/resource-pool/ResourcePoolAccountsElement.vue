<template>
  <div class="accounts-card grow min-h-[360px] bg-white dark:bg-dark-900 border border-gray-200/80 dark:border-dark-700/80 rounded-2xl shadow-sm overflow-hidden flex flex-col">
    <!-- Header -->
    <div class="p-4 border-b border-gray-100 dark:border-dark-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <div class="flex items-center gap-2.5 min-w-0">
        <div class="w-8 h-8 rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-950/60 dark:text-primary-400 flex items-center justify-center shrink-0">
          <el-icon class="text-base"><Operation /></el-icon>
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h3 class="text-base font-bold text-gray-900 dark:text-gray-100 tracking-tight m-0 truncate">
              {{ title }}
            </h3>
            <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-300">
              {{ t('resourcePool.accounts.total', { count: total }) }}
            </span>
          </div>
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center flex-wrap gap-2 shrink-0">
        <el-tooltip :content="t('common.refresh')" placement="top">
          <el-button
            plain
            circle
            :loading="loading"
            class="!border-gray-200 dark:!border-dark-700 hover:!border-primary-500"
            @click="$emit('refresh')"
          >
            <el-icon><Refresh /></el-icon>
          </el-button>
        </el-tooltip>

        <el-button
          v-if="groupId"
          plain
          class="!rounded-lg font-medium"
          @click="$emit('add-existing')"
        >
          <el-icon class="mr-1"><Plus /></el-icon>
          {{ t('resourcePool.actions.addExisting') }}
        </el-button>

        <el-button
          type="primary"
          data-tour="accounts-create-btn"
          class="!rounded-lg font-medium shadow-sm hover:shadow"
          @click="$emit('create')"
        >
          <el-icon class="mr-1"><CirclePlus /></el-icon>
          {{ t('resourcePool.actions.createAccount') }}
        </el-button>
      </div>
    </div>

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
        <div class="flex items-center gap-2">
          <el-button
            size="small"
            type="primary"
            class="!rounded-lg"
            @click="$emit('batch-add', selected)"
          >
            <el-icon class="mr-1"><FolderAdd /></el-icon>
            {{ t('resourcePool.actions.addToGroup') }}
          </el-button>
          <el-button
            v-if="groupId"
            size="small"
            type="danger"
            plain
            class="!rounded-lg"
            @click="$emit('batch-remove', selected)"
          >
            <el-icon class="mr-1"><Delete /></el-icon>
            {{ t('resourcePool.actions.removeFromCurrent') }}
          </el-button>
          <el-button
            size="small"
            type="danger"
            plain
            class="!rounded-lg"
            @click="$emit('batch-delete', selected)"
          >
            <el-icon class="mr-1"><Delete /></el-icon>
            {{ t('resourcePool.actions.deleteSelected') }}
          </el-button>
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
    <div class="p-3 flex-1 min-h-0 overflow-x-auto">
      <el-table
        ref="tableRef"
        v-loading="loading"
        :data="accounts"
        row-key="id"
        :height="isNarrow ? undefined : '100%'"
        class="resource-pool-table !rounded-xl"
        @selection-change="onSelectionChange"
      >
        <!-- Selection Checkbox -->
        <el-table-column type="selection" width="46" align="center" />

        <!-- Name -->
        <el-table-column :label="t('resourcePool.accounts.name')" min-width="220" fixed="left">
          <template #default="{ row }">
            <div class="flex items-center gap-2.5 py-1">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span class="font-bold text-sm text-gray-900 dark:text-gray-100 truncate" :title="row.name">
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

        <!-- Billing multiplier -->
        <el-table-column :label="t('resourcePool.accounts.multiplier')" width="90" align="center">
          <template #default="{ row }">
            <span class="font-mono font-semibold text-gray-700 dark:text-gray-300">{{ Number(row.rate_multiplier ?? 1).toFixed(2) }}x</span>
          </template>
        </el-table-column>

        <!-- Scheduling -->
        <el-table-column :label="t('resourcePool.accounts.scheduling')" width="90" align="center">
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
        <el-table-column label="分组" min-width="220">
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

        <!-- Platform / type -->
        <el-table-column :label="t('resourcePool.accounts.platform')" width="96">
          <template #default="{ row }">
            <div class="flex flex-col items-start gap-1">
              <PlatformIcon
                :platform="row.platform"
                size="md"
                :class="!row.schedulable
                  ? 'text-gray-900 dark:text-gray-100'
                  : accountAvailable(row)
                    ? 'text-emerald-500'
                    : 'text-red-500'"
              />
            </div>
          </template>
        </el-table-column>

        <!-- Capacity -->
        <el-table-column :label="t('resourcePool.accounts.capacity')" min-width="150">
          <template #default="{ row }">
            <div class="space-y-1 py-1">
              <div class="flex items-center justify-between gap-2 text-xs">
                <span class="text-gray-500 dark:text-gray-400">并发</span>
                <span class="font-mono font-semibold text-gray-700 dark:text-gray-300">{{ row.current_concurrency ?? 0 }} / {{ row.concurrency }}</span>
              </div>
              <el-progress :percentage="concurrencyPercent(row)" :status="concurrencyStatus(row)" :show-text="false" :stroke-width="5" />
            </div>
          </template>
        </el-table-column>

        <!-- Today's stats -->
        <el-table-column :label="t('resourcePool.accounts.todayStats')" min-width="190">
          <template #default="{ row }">
            <AccountTodayStatsCell
              :stats="todayStatsByAccountId[String(row.id)] ?? null"
              :loading="todayStatsLoading"
              :error="todayStatsError"
            />
          </template>
        </el-table-column>

        <!-- Actions -->
        <el-table-column :label="t('common.actions')" width="200" fixed="right" align="center">
          <template #default="{ row }">
            <div class="flex items-center justify-center gap-1">
              <el-button link type="primary" size="small" @click="$emit('test', row)">
                {{ t('resourcePool.actions.test') }}
              </el-button>
              <el-button link size="small" @click="$emit('stats', row)">
                {{ t('resourcePool.actions.stats') }}
              </el-button>
              <el-button link size="small" @click="$emit('edit', row)">
                {{ t('common.edit') }}
              </el-button>

              <el-dropdown trigger="click" @command="onRowCommand($event, row)">
                <el-button link size="small" class="!px-1">
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu class="account-row-dropdown">
                    <el-dropdown-item command="duplicate">
                      <el-icon><CopyDocument /></el-icon>
                      <span>{{ t('resourcePool.actions.duplicate') }}</span>
                    </el-dropdown-item>
                    <el-dropdown-item v-if="groupId" command="remove" divided class="!text-amber-600">
                      <el-icon><Delete /></el-icon>
                      <span>{{ t('resourcePool.actions.removeFromCurrent') }}</span>
                    </el-dropdown-item>
                    <el-dropdown-item command="delete" :divided="!groupId" class="!text-red-500">
                      <el-icon><Delete /></el-icon>
                      <span>{{ t('common.delete') }}</span>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
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
    <div class="p-3.5 border-t border-gray-100 dark:border-dark-800 flex items-center justify-between flex-wrap gap-2">
      <div class="text-xs text-gray-400 dark:text-dark-400">
        {{ t('resourcePool.accounts.total', { count: total }) }}
      </div>
      <el-pagination
        background
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
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMediaQuery } from '@vueuse/core'
import { ElMessage, ElMessageBox, type TableInstance } from 'element-plus'
import {
  CirclePlus,
  Close,
  CopyDocument,
  Delete,
  Filter,
  FolderAdd,
  Link,
  MoreFilled,
  Operation,
  Plus,
  Refresh,
  Search
} from '@element-plus/icons-vue'
import { accountsAPI } from '@/api/admin'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import AccountTodayStatsCell from '@/components/account/AccountTodayStatsCell.vue'
import ResourcePoolMembershipCell from './ResourcePoolMembershipCell.vue'
import type { AccountListItem, AdminGroup, GroupPlatform, WindowStats } from '@/types'

export interface ResourcePoolAccountFilters {
  search: string
  platform: string
  status: string
  type: string
}

const props = defineProps<{
  accounts: AccountListItem[]
  title: string
  total: number
  page: number
  pageSize: number
  loading: boolean
  filters: ResourcePoolAccountFilters
  groupId?: number
  availableGroups: AdminGroup[]
}>()

const emit = defineEmits<{
  refresh: []
  create: []
  'add-existing': []
  'batch-add': [accounts: AccountListItem[]]
  'batch-remove': [accounts: AccountListItem[]]
  'batch-delete': [accounts: AccountListItem[]]
  'page-change': [page: number, pageSize: number]
  'filter-change': [filters: ResourcePoolAccountFilters]
  'group-click': [groupId: number]
  edit: [account: AccountListItem]
  test: [account: AccountListItem]
  stats: [account: AccountListItem]
  'row-action': [command: string, account: AccountListItem]
  'membership-changed': []
}>()

const { t } = useI18n()
const isNarrow = useMediaQuery('(max-width: 960px)')
const tableRef = ref<TableInstance>()
const selected = ref<AccountListItem[]>([])
const pendingGroupSelections = ref(new Map<number, number[]>())
const pendingGroupAccounts = ref(new Map<number, AccountListItem>())
const savingGroupChanges = ref(false)
const todayStatsByAccountId = ref<Record<string, WindowStats>>({})
const todayStatsLoading = ref(false)
const todayStatsError = ref<string | null>(null)
const schedulingAccountId = ref<number | null>(null)
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

function concurrencyPercent(account: AccountListItem) {
  if (!account.concurrency) return 0
  return Math.min(100, Math.round(((account.current_concurrency ?? 0) / account.concurrency) * 100))
}

function concurrencyStatus(account: AccountListItem) {
  const percent = concurrencyPercent(account)
  if (percent >= 90) return 'exception'
  if (percent >= 60) return 'warning'
  return 'success'
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

.group-tag {
  cursor: pointer;
}

:deep(.priority-input .el-input__wrapper) {
  padding-left: 8px;
  padding-right: 24px;
}
</style>
