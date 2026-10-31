<template>
  <AppLayout>
    <el-config-provider :locale="epLocale">
      <div class="resource-pools-container" :class="{ 'is-narrow': isNarrow }">
        <!-- Page Top Header -->
        <div class="page-top-header flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-1 border-b border-gray-200/70 dark:border-dark-800">
          <div>
            <div class="flex items-center gap-2.5">
              <div class="w-9 h-9 rounded-xl bg-gradient-to-tr from-primary-600 to-blue-500 text-white flex items-center justify-center shadow-sm shrink-0">
                <el-icon class="text-lg"><Folder /></el-icon>
              </div>
              <div>
                <h1 class="text-xl font-bold text-gray-900 dark:text-gray-100 tracking-tight m-0">
                  {{ t('resourcePool.title') }}
                </h1>
                <p class="text-xs text-gray-500 dark:text-gray-400 m-0 mt-0.5">
                  {{ t('resourcePool.description') }}
                </p>
              </div>
            </div>
          </div>

          <!-- Quick Metrics & Top Actions -->
          <div class="flex items-center flex-wrap gap-2">
            <!-- Header Mini Badges -->
            <div class="hidden xl:flex items-center gap-2 mr-2 text-xs">
              <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-gray-100 dark:bg-dark-800 text-gray-600 dark:text-gray-300 font-medium border border-gray-200/60 dark:border-dark-700">
                <span class="text-gray-400">资源池:</span>
                <strong class="text-gray-800 dark:text-gray-200">{{ groups.length }}</strong>
              </span>
              <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-gray-100 dark:bg-dark-800 text-gray-600 dark:text-gray-300 font-medium border border-gray-200/60 dark:border-dark-700">
                <span class="text-gray-400">总账号:</span>
                <strong class="text-gray-800 dark:text-gray-200">{{ allCount }}</strong>
              </span>
              <span
                v-if="ungroupedCount > 0"
                class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-amber-50 dark:bg-amber-950/40 text-amber-700 dark:text-amber-300 font-medium border border-amber-200/60 dark:border-amber-800/60"
              >
                <span class="w-1.5 h-1.5 rounded-full bg-amber-500 animate-pulse" />
                <span>未分组:</span>
                <strong>{{ ungroupedCount }}</strong>
              </span>
            </div>

            <!-- Refresh Workspace -->
            <el-tooltip :content="t('common.refresh')" placement="top">
              <el-button
                plain
                circle
                :loading="groupsLoading || accountsLoading"
                class="!border-gray-200 dark:!border-dark-700 hover:!border-primary-500"
                @click="refreshWorkspace"
              >
                <el-icon><Refresh /></el-icon>
              </el-button>
            </el-tooltip>

            <!-- Advanced Config Link -->
            <el-tooltip :content="t('resourcePool.advanced.description')" placement="top">
              <el-button
                plain
                class="!rounded-lg text-xs"
                @click="goToAdvanced"
              >
                <el-icon class="mr-1"><Setting /></el-icon>
                {{ t('resourcePool.actions.advanced') }}
              </el-button>
            </el-tooltip>

            <el-button
              plain
              data-testid="upstream-account-management"
              class="!rounded-lg text-xs"
              @click="showUpstreamAccounts = true"
            >
              <el-icon class="mr-1"><Connection /></el-icon>
              {{ t('resourcePool.actions.upstreamAccounts') }}
            </el-button>

            <!-- Create Pool Button -->
            <el-button
              type="primary"
              data-tour="groups-create-btn"
              class="!rounded-lg text-xs font-medium shadow-sm hover:shadow"
              @click="openCreateGroup"
            >
              <el-icon class="mr-1"><Plus /></el-icon>
              {{ t('resourcePool.actions.createGroup') }}
            </el-button>

            <!-- Create Account Button -->
            <el-button
              plain
              data-tour="accounts-create-btn"
              class="!rounded-lg text-xs font-medium"
              @click="showCreateAccount = true"
            >
              <el-icon class="mr-1"><CirclePlus /></el-icon>
              {{ t('resourcePool.actions.createAccount') }}
            </el-button>

            <!-- Mobile Drawer Trigger -->
            <el-button
              v-if="isNarrow"
              plain
              class="!rounded-lg text-xs"
              @click="navigatorDrawer = true"
            >
              <el-icon class="mr-1"><Menu /></el-icon>
              {{ currentTitle }}
            </el-button>
          </div>
        </div>

        <!-- Mobile Pool Selector Bar -->
        <div v-if="isNarrow" class="mobile-pool-select">
          <el-select
            :model-value="selectionKey"
            size="large"
            class="w-full"
            @change="selectByKey"
          >
            <el-option :label="t('resourcePool.navigator.allAccounts')" value="all" />
            <el-option :label="t('resourcePool.navigator.ungrouped')" value="ungrouped" />
            <el-option label="限流与异常监控" value="issues" />
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="`${group.platform} / ${group.name}`"
              :value="`group:${group.id}`"
            />
          </el-select>
        </div>

        <!-- Desktop Workspace Grid: Left Navigator + Right Main Content -->
        <div class="workspace-grid" :class="{ 'is-narrow': isNarrow }">
          <!-- Left Navigator Column -->
          <aside v-if="!isNarrow" class="workspace-sidebar">
            <ResourcePoolNavigatorElement
              :groups="groups"
              :scope="scope"
              :group-id="selectedGroupId"
              :all-count="allCount"
              :ungrouped-count="ungroupedCount"
              :limited-count="limitedCount"
              :loading="groupsLoading"
              @select="applySelection"
              @create="openCreateGroup"
              @refresh="refreshWorkspace"
            />
          </aside>

          <!-- Right Content Column -->
          <main class="workspace-main space-y-0 min-w-0">
            <!-- Pool Hero Summary / Scope Overview -->
            <ResourcePoolSummaryElement
              :group="selectedGroup"
              :scope="scope"
              :all-count="allCount"
              :ungrouped-count="ungroupedCount"
              @action="handleGroupAction"
            />

            <!-- Accounts Management Table Card -->
            <ResourcePoolAccountsElement
              :accounts="accounts"
              :total="accountsTotal"
              :page="page"
              :pageSize="pageSize"
              :loading="accountsLoading"
              :filters="filters"
              :groupId="selectedGroupId"
              :available-groups="groups"
              @refresh="loadAccounts"
              @create="showCreateAccount = true"
              @add-existing="showAssign = true"
              @batch-add="openBatchGroup"
              @batch-remove="batchRemove"
              @batch-delete="batchDelete"
              @membership-changed="relationshipChanged"
              @page-change="changePage"
              @filter-change="changeFilters"
              @group-click="groupId => applySelection({ scope: 'group', groupId })"
              @edit="openAccountModal('edit', $event)"
              @test="openAccountModal('test', $event)"
              @stats="openAccountModal('stats', $event)"
              @schedule="openScheduledTests"
              @recover-state="recoverAccountState"
              @row-action="handleAccountAction"
            />
          </main>
        </div>

        <!-- Mobile Drawer for Navigator -->
        <el-drawer
          v-model="navigatorDrawer"
          direction="ltr"
          size="min(88vw, 360px)"
          :title="t('resourcePool.navigator.title')"
          class="navigator-drawer"
        >
          <ResourcePoolNavigatorElement
            :groups="groups"
            :scope="scope"
            :group-id="selectedGroupId"
            :all-count="allCount"
            :ungrouped-count="ungroupedCount"
            :limited-count="limitedCount"
            :loading="groupsLoading"
            @select="selection => { applySelection(selection); navigatorDrawer = false }"
            @create="openCreateGroup"
            @refresh="refreshWorkspace"
          />
        </el-drawer>

        <!-- Original system group form dialog -->
        <BaseDialog
          :show="groupDialogVisible"
          :title="editingGroup ? t('admin.groups.editGroup') : t('admin.groups.createGroup')"
          width="wide"
          @close="groupDialogVisible = false"
        >
          <form class="space-y-5" @submit.prevent="saveGroup">
            <div>
              <label class="input-label">{{ t('admin.groups.form.name') }}</label>
              <input
                v-model="groupForm.name"
                type="text"
                maxlength="80"
                required
                class="input"
                :placeholder="t('resourcePool.groupForm.namePlaceholder')"
              />
            </div>
            <div>
              <label class="input-label">{{ t('admin.groups.form.platform') }}</label>
              <Select
                v-model="groupForm.platform"
                :options="groupPlatformOptions"
                :disabled="Boolean(editingGroup)"
              />
              <p v-if="editingGroup" class="input-hint">{{ t('admin.groups.platformNotEditable') }}</p>
              <p v-else class="input-hint">{{ t('admin.groups.platformHint') }}</p>
            </div>
            <div>
              <label class="input-label">{{ t('admin.groups.form.rateMultiplier') }}</label>
              <input
                v-model.number="groupForm.rate_multiplier"
                type="number"
                min="0.001"
                step="0.001"
                required
                class="input"
              />
              <p class="input-hint">{{ t('resourcePool.groupForm.multiplierTip') }}</p>
            </div>
            <div>
              <label class="input-label">{{ t('admin.groups.form.description') }}</label>
              <textarea
                v-model="groupForm.description"
                rows="3"
                maxlength="255"
                class="input"
                :placeholder="t('resourcePool.groupForm.descPlaceholder')"
              />
            </div>
            <div class="flex justify-end gap-2">
              <button type="button" class="btn btn-secondary" @click="groupDialogVisible = false">
                {{ t('common.cancel') }}
              </button>
              <button type="submit" class="btn btn-primary" :disabled="groupSaving || !groupForm.name.trim()">
                {{ groupSaving ? t('common.saving') : t('common.save') }}
              </button>
            </div>
          </form>
        </BaseDialog>

        <!-- Batch Add to Group Dialog -->
        <el-dialog
          v-model="batchGroupVisible"
          :title="t('resourcePool.actions.addToGroup')"
          width="480px"
          class="!rounded-2xl"
          destroy-on-close
        >
          <div class="space-y-3">
            <p class="text-xs text-gray-500 dark:text-gray-400 m-0">
              请选择要将选中的 {{ batchAccounts.length }} 个账号加入的目标资源池：
            </p>
            <el-select
              v-model="batchTargetGroupId"
              filterable
              class="w-full"
              :placeholder="t('resourcePool.assign.chooseGroup')"
            >
              <el-option
                v-for="group in groups"
                :key="group.id"
                :label="`${platformLabel(group.platform)} / ${group.name}`"
                :value="group.id"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <PlatformIcon :platform="group.platform" size="xs" />
                    <span>{{ group.name }}</span>
                  </div>
                  <span class="text-xs text-gray-400">{{ group.account_count ?? 0 }} 账号</span>
                </div>
              </el-option>
            </el-select>
          </div>

          <template #footer>
            <div class="flex items-center justify-end gap-2">
              <el-button @click="batchGroupVisible = false">
                {{ t('common.cancel') }}
              </el-button>
              <el-button
                type="primary"
                :loading="batchMutating"
                :disabled="!batchTargetGroupId"
                class="!rounded-lg shadow-sm"
                @click="confirmBatchAdd"
              >
                {{ t('common.confirm') }}
              </el-button>
            </div>
          </template>
        </el-dialog>

        <!-- Assign Existing Accounts Dialog -->
        <AssignAccountsToGroupDialogElement
          v-model="showAssign"
          :group="selectedGroup"
          @changed="relationshipChanged"
        />

        <UpstreamAccountManagementDrawerElement
          v-model="showUpstreamAccounts"
          @created="refreshWorkspace"
        />

        <!-- Modals: Create / Edit / Test / Stats Account -->
        <CreateAccountModal
          :show="showCreateAccount"
          :proxies="proxies"
          :groups="groups"
          :default-group-ids="selectedGroupId ? [selectedGroupId] : []"
          @close="showCreateAccount = false"
          @created="accountCreated"
        />
        <EditAccountModal
          :show="showEditAccount"
          :account="activeAccount"
          :proxies="proxies"
          :groups="groups"
          @close="showEditAccount = false"
          @updated="accountUpdated"
        />
        <AccountTestModal
          :show="showTestAccount"
          :account="activeAccount"
          @close="showTestAccount = false"
        />
        <AccountStatsModal
          :show="showStatsAccount"
          :account="activeAccount"
          @close="showStatsAccount = false"
        />
        <ScheduledTestsPanel
          :show="showSchedulePanel"
          :account-id="scheduleAccountId"
          :model-options="scheduleModelOptions"
          @close="closeScheduledTests"
        />

        <!-- Group Rate Multipliers Modal（原系统弹窗，直接搬用） -->
        <GroupRateMultipliersModal
          :show="showRateMultipliersModal"
          :group="rateMultipliersGroup"
          @close="showRateMultipliersModal = false"
          @success="loadGroups"
        />

        <!-- Group RPM Overrides Modal（原系统弹窗，直接搬用） -->
        <GroupRPMOverridesModal
          :show="showRPMOverridesModal"
          :group="rpmOverridesGroup"
          @close="showRPMOverridesModal = false"
          @success="loadGroups"
        />

        <div class="hidden" aria-hidden="true">
          <GroupsView
            ref="originalGroupsView"
            embedded-create-only
            @group-created="handleOriginalGroupCreated"
            @group-updated="loadGroups"
          />
        </div>
      </div>
    </el-config-provider>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useMediaQuery } from '@vueuse/core'
import { ElMessage, ElMessageBox } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import enUs from 'element-plus/es/locale/lang/en'
import {
  CirclePlus,
  Connection,
  Folder,
  Menu,
  Plus,
  Refresh,
  Setting
} from '@element-plus/icons-vue'
import { accountsAPI, groupsAPI, proxiesAPI } from '@/api/admin'
import type { Account, AccountListItem, AdminGroup, ClaudeModel, GroupPlatform, Proxy as AccountProxy } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import ResourcePoolNavigatorElement from '@/components/element/resource-pool/ResourcePoolNavigatorElement.vue'
import ResourcePoolSummaryElement from '@/components/element/resource-pool/ResourcePoolSummaryElement.vue'
import ResourcePoolAccountsElement, { type ResourcePoolAccountFilters } from '@/components/element/resource-pool/ResourcePoolAccountsElement.vue'
import AssignAccountsToGroupDialogElement from '@/components/element/resource-pool/AssignAccountsToGroupDialogElement.vue'
import UpstreamAccountManagementDrawerElement from '@/components/element/resource-pool/UpstreamAccountManagementDrawerElement.vue'
import GroupRateMultipliersModal from '@/components/admin/group/GroupRateMultipliersModal.vue'
import GroupRPMOverridesModal from '@/components/admin/group/GroupRPMOverridesModal.vue'
import CreateAccountModal from '@/components/account/CreateAccountModal.vue'
import EditAccountModal from '@/components/account/EditAccountModal.vue'
import AccountTestModal from '@/components/admin/account/AccountTestModal.vue'
import AccountStatsModal from '@/components/admin/account/AccountStatsModal.vue'
import ScheduledTestsPanel from '@/components/admin/account/ScheduledTestsPanel.vue'
import type { SelectOption } from '@/components/common/Select.vue'
import GroupsView from './GroupsView.vue'
import { accountGroupFilter, buildResourcePoolQuery, parseResourcePoolRoute } from './resourcePoolRoute'

type PoolSelection = { scope: 'all' | 'ungrouped' | 'issues' | 'group'; groupId?: number }
type AccountModal = 'edit' | 'test' | 'stats'

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const epLocale = computed(() => (locale.value === 'zh' ? zhCn : enUs))
const isNarrow = useMediaQuery('(max-width: 960px)')

const navigatorDrawer = ref(false)
const groups = ref<AdminGroup[]>([])
const proxies = ref<AccountProxy[]>([])
const groupsLoading = ref(false)
const accounts = ref<AccountListItem[]>([])
const accountsLoading = ref(false)
const accountsTotal = ref(0)
const allCount = ref(0)
const ungroupedCount = ref(0)
const limitedCount = ref(0)
const scope = ref<'all' | 'ungrouped' | 'issues' | 'group'>('all')
const selectedGroupId = ref<number>()
const page = ref(1)
const pageSize = ref(20)
const filters = reactive<ResourcePoolAccountFilters>({ search: '', platform: '', status: '', type: '' })

const showAssign = ref(false)
const showUpstreamAccounts = ref(false)
const showCreateAccount = ref(false)
const showEditAccount = ref(false)
const showTestAccount = ref(false)
const showStatsAccount = ref(false)
const activeAccount = ref<Account | null>(null)
const showSchedulePanel = ref(false)
const scheduleAccountId = ref<number | null>(null)
const scheduleModelOptions = ref<SelectOption[]>([])

const groupDialogVisible = ref(false)
const groupSaving = ref(false)

// 原系统分组弹窗（与 GroupsView 相同的挂载方式）
const showRateMultipliersModal = ref(false)
const rateMultipliersGroup = ref<AdminGroup | null>(null)
const showRPMOverridesModal = ref(false)
const rpmOverridesGroup = ref<AdminGroup | null>(null)
const editingGroup = ref<AdminGroup | null>(null)
const groupForm = reactive({
  name: '',
  platform: 'anthropic' as GroupPlatform,
  description: '',
  rate_multiplier: 1
})
const originalGroupsView = ref<{ openCreateModal: () => void; openEditModal: (group: AdminGroup) => void } | null>(null)

const batchGroupVisible = ref(false)
const batchTargetGroupId = ref<number>()
const batchAccounts = ref<AccountListItem[]>([])
const batchMutating = ref(false)

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
const groupPlatformOptions = computed(() => platforms.map(platform => ({ value: platform, label: platformLabel(platform) })))
const selectedGroup = computed(() => groups.value.find(group => group.id === selectedGroupId.value) ?? null)
const selectionKey = computed(() => (scope.value === 'group' ? `group:${selectedGroupId.value}` : scope.value))
const currentTitle = computed(() => {
  return selectedGroup.value?.name ?? (scope.value === 'ungrouped' ? t('resourcePool.navigator.ungrouped') : scope.value === 'issues' ? '限流与异常监控' : t('resourcePool.navigator.allAccounts'))
})

function restoreFromRoute() {
  const state = parseResourcePoolRoute(route.query)
  scope.value = state.scope
  selectedGroupId.value = state.groupId
  page.value = state.page
  pageSize.value = state.pageSize
  filters.search = state.search
  filters.platform = state.platform
  filters.status = state.status
  filters.type = state.type
}

async function syncRoute() {
  const query = buildResourcePoolQuery({
    scope: scope.value,
    groupId: selectedGroupId.value,
    page: page.value,
    pageSize: pageSize.value,
    ...filters
  })
  await router.replace({ path: '/admin/resource-pools', query })
}

async function loadGroups() {
  groupsLoading.value = true
  try {
    groups.value = await groupsAPI.getAllIncludingInactive()
  } finally {
    groupsLoading.value = false
  }
}

async function loadCounts() {
  const [all, ungrouped, limited] = await Promise.all([
    accountsAPI.list(1, 1, { lite: '1' }),
    accountsAPI.list(1, 1, { group: 'ungrouped', lite: '1' }),
    accountsAPI.list(1, 1, { status: 'rate_limited', lite: '1' })
  ])
  allCount.value = all.total
  ungroupedCount.value = ungrouped.total
  limitedCount.value = limited.total
}

async function loadAccounts() {
  accountsLoading.value = true
  try {
    const group = accountGroupFilter({ scope: scope.value, groupId: selectedGroupId.value })
    const result = await accountsAPI.list(page.value, pageSize.value, {
      group,
      search: filters.search || undefined,
      platform: filters.platform || undefined,
      status: scope.value === 'issues' ? 'rate_limited' : filters.status || undefined,
      type: filters.type || undefined,
      lite: '0'
    })
    accounts.value = result.items
    accountsTotal.value = result.total
  } catch {
    ElMessage.error(t('resourcePool.messages.loadFailed'))
  } finally {
    accountsLoading.value = false
  }
}

async function refreshWorkspace() {
  await Promise.all([loadGroups(), loadCounts(), loadAccounts()])
}

async function applySelection(selection: PoolSelection) {
  scope.value = selection.scope
  selectedGroupId.value = selection.scope === 'group' ? selection.groupId : undefined
  page.value = 1
  await syncRoute()
  await loadAccounts()
}

function selectByKey(value: string) {
  if (value.startsWith('group:')) {
    void applySelection({ scope: 'group', groupId: Number(value.slice(6)) })
  } else {
    void applySelection({ scope: value as 'all' | 'ungrouped' | 'issues' })
  }
}

async function changePage(nextPage: number, nextSize: number) {
  page.value = nextPage
  pageSize.value = nextSize
  await syncRoute()
  await loadAccounts()
}

async function changeFilters(next: ResourcePoolAccountFilters) {
  Object.assign(filters, next)
  page.value = 1
  await syncRoute()
  await loadAccounts()
}

function resetGroupForm(group?: AdminGroup) {
  editingGroup.value = group ?? null
  groupForm.name = group?.name ?? ''
  groupForm.platform = (group?.platform as GroupPlatform) ?? 'anthropic'
  groupForm.description = group?.description ?? ''
  groupForm.rate_multiplier = group?.rate_multiplier ?? 1
}

function openCreateGroup() {
  resetGroupForm()
  groupDialogVisible.value = false
  navigatorDrawer.value = false
  originalGroupsView.value?.openCreateModal()
}

async function handleOriginalGroupCreated(group: AdminGroup) {
  await loadGroups()
  await applySelection({ scope: 'group', groupId: group.id })
}

async function saveGroup() {
  groupSaving.value = true
  try {
    const payload = {
      name: groupForm.name.trim(),
      platform: groupForm.platform,
      description: groupForm.description.trim() || null,
      rate_multiplier: groupForm.rate_multiplier
    }
    const saved = editingGroup.value
      ? await groupsAPI.update(editingGroup.value.id, payload)
      : await groupsAPI.create(payload)
    groupDialogVisible.value = false
    await loadGroups()
    await applySelection({ scope: 'group', groupId: saved.id })
    ElMessage.success(t('resourcePool.messages.groupSaved'))
  } finally {
    groupSaving.value = false
  }
}

async function goToAdvanced() {
  const returnId = selectedGroupId.value ? String(selectedGroupId.value) : undefined
  await router.push({
    path: '/admin/resource-pools/advanced-groups',
    query: returnId ? { return_group_id: returnId } : undefined
  })
}

async function handleGroupAction(command: string, payload?: number | boolean) {
  if (command === 'create-account') {
    showCreateAccount.value = true
    return
  }
  if (command === 'create-group') {
    openCreateGroup()
    return
  }
  if (command === 'add-existing') {
    showAssign.value = true
    return
  }
  if (command === 'refresh') {
    await refreshWorkspace()
    return
  }
  if (command === 'rate-multipliers') {
    rateMultipliersGroup.value = selectedGroup.value
    showRateMultipliersModal.value = true
    return
  }
  if (command === 'rpm-overrides') {
    rpmOverridesGroup.value = selectedGroup.value
    showRPMOverridesModal.value = true
    return
  }

  const group = selectedGroup.value
  if (!group) return

  if (command === 'toggle-exclusive') {
    try {
      await groupsAPI.update(group.id, { is_exclusive: Boolean(payload) })
      ElMessage.success(t('resourcePool.messages.groupSaved'))
      await loadGroups()
    } catch (error) {
      console.error('Failed to toggle exclusive:', error)
      ElMessage.error(t('resourcePool.messages.quickSaveFailed'))
    }
    return
  }
  if (command === 'toggle-status') {
    const next = group.status === 'active' ? 'inactive' : 'active'
    try {
      await groupsAPI.update(group.id, { status: next })
      ElMessage.success(t('resourcePool.messages.groupSaved'))
      await loadGroups()
    } catch (error) {
      console.error('Failed to toggle status:', error)
      ElMessage.error(t('resourcePool.messages.quickSaveFailed'))
    }
    return
  }
  if (command === 'set-rate') {
    const rate = typeof payload === 'number' ? payload : NaN
    if (!Number.isFinite(rate) || rate < 0) return
    try {
      await groupsAPI.update(group.id, { rate_multiplier: rate })
      ElMessage.success(t('resourcePool.messages.groupSaved'))
      await loadGroups()
    } catch (error) {
      console.error('Failed to update rate multiplier:', error)
      ElMessage.error(t('resourcePool.messages.quickSaveFailed'))
    }
    return
  }
  if (command === 'edit') {
    if (!group) return
    navigatorDrawer.value = false
    originalGroupsView.value?.openEditModal(group)
    return
  }
  if (command === 'advanced') {
    await goToAdvanced()
    return
  }
  if (command === 'duplicate') {
    const duplicate = await groupsAPI.duplicate(group.id)
    await loadGroups()
    await applySelection({ scope: 'group', groupId: duplicate.id })
    ElMessage.success(t('resourcePool.messages.groupDuplicated'))
    return
  }
  if (command === 'delete') {
    await ElMessageBox.confirm(
      t('resourcePool.messages.deleteGroupWarning', { name: group.name }),
      t('resourcePool.messages.deleteGroupTitle'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel')
      }
    )
    await groupsAPI.delete(group.id)
    await Promise.all([loadGroups(), loadCounts()])
    await applySelection({ scope: 'all' })
    ElMessage.success(t('resourcePool.messages.groupDeleted'))
  }
}

async function openAccountModal(kind: AccountModal, account: AccountListItem) {
  try {
    activeAccount.value = await accountsAPI.getById(account.id)
    if (kind === 'edit') showEditAccount.value = true
    if (kind === 'test') showTestAccount.value = true
    if (kind === 'stats') showStatsAccount.value = true
  } catch {
    ElMessage.error(t('resourcePool.messages.accountLoadFailed'))
  }
}

async function openScheduledTests(account: AccountListItem) {
  scheduleAccountId.value = account.id
  scheduleModelOptions.value = []
  showSchedulePanel.value = true
  try {
    const models = await accountsAPI.getAvailableModels(account.id)
    scheduleModelOptions.value = models.map((model: ClaudeModel) => ({
      value: model.id,
      label: model.display_name || model.id
    }))
  } catch {
    scheduleModelOptions.value = []
  }
}

function closeScheduledTests() {
  showSchedulePanel.value = false
  scheduleAccountId.value = null
  scheduleModelOptions.value = []
}

async function recoverAccountState(account: AccountListItem) {
  try {
    await accountsAPI.recoverState(account.id)
    await relationshipChanged()
    ElMessage.success(t('admin.accounts.recoverStateSuccess'))
  } catch (error: any) {
    ElMessage.error(error?.message || t('admin.accounts.recoverStateFailed'))
  }
}

async function handleAccountAction(command: string, account: AccountListItem) {
  if (command === 'remove' && selectedGroupId.value) {
    await removeAccounts([account])
    return
  }
  if (command === 'duplicate') {
    await accountsAPI.duplicate(account.id)
    await relationshipChanged()
    ElMessage.success(t('resourcePool.messages.accountDuplicated'))
    return
  }
  if (command === 'delete') {
    await ElMessageBox.confirm(
      t('resourcePool.messages.deleteAccountWarning', { name: account.name }),
      t('resourcePool.messages.deleteAccountTitle'),
      { type: 'warning' }
    )
    await accountsAPI.delete(account.id)
    await relationshipChanged()
  }
}

function openBatchGroup(selected: AccountListItem[]) {
  batchAccounts.value = selected
  batchTargetGroupId.value = selectedGroupId.value
  batchGroupVisible.value = true
}

async function confirmBatchAdd() {
  if (!batchTargetGroupId.value) return
  batchMutating.value = true
  let success = 0
  let failed = 0

  for (const account of batchAccounts.value) {
    try {
      const risk = await accountsAPI.checkMixedChannelRisk({
        platform: account.platform,
        group_ids: [batchTargetGroupId.value],
        account_id: account.id
      })
      let confirmRisk = false
      if (risk.has_risk) {
        await ElMessageBox.confirm(
          t('resourcePool.assign.mixedWarning', { name: account.name }),
          t('resourcePool.assign.riskTitle'),
          {
            type: 'warning',
            confirmButtonText: t('resourcePool.assign.continue'),
            cancelButtonText: t('common.cancel')
          }
        )
        confirmRisk = true
      }
      await accountsAPI.setGroupMembership(account.id, batchTargetGroupId.value, {
        priority: 1,
        confirm_mixed_channel_risk: confirmRisk
      })
      success++
    } catch {
      failed++
    }
  }

  batchMutating.value = false
  batchGroupVisible.value = false
  await relationshipChanged()
  ElMessage.info(t('resourcePool.assign.partial', { success, failed }))
}

async function removeAccounts(selected: AccountListItem[]) {
  if (!selectedGroupId.value) return
  let success = 0
  let failed = 0
  for (const account of selected) {
    try {
      await accountsAPI.removeGroupMembership(account.id, selectedGroupId.value)
      success++
    } catch {
      failed++
    }
  }
  await relationshipChanged()
  ElMessage.info(t('resourcePool.assign.partial', { success, failed }))
}

async function batchRemove(selected: AccountListItem[]) {
  await ElMessageBox.confirm(
    t('resourcePool.messages.removeMembershipWarning', { count: selected.length }),
    t('resourcePool.actions.removeFromCurrent'),
    { type: 'warning' }
  )
  await removeAccounts(selected)
}

async function batchDelete(selected: AccountListItem[]) {
  await ElMessageBox.confirm(
    t('resourcePool.messages.deleteAccountsWarning', { count: selected.length }),
    t('resourcePool.messages.deleteAccountTitle'),
    {
      type: 'warning',
      confirmButtonText: t('common.delete'),
      cancelButtonText: t('common.cancel')
    }
  )

  const result = await accountsAPI.batchDelete(selected.map(account => account.id))
  if (result.failed > 0) {
    ElMessage.warning(t('resourcePool.messages.batchDeletePartial', {
      success: result.success,
      failed: result.failed
    }))
  } else {
    ElMessage.success(t('resourcePool.messages.accountsDeleted', { count: result.success }))
  }
  await relationshipChanged()
}

async function relationshipChanged() {
  await Promise.all([loadGroups(), loadCounts(), loadAccounts()])
}

async function accountCreated() {
  showCreateAccount.value = false
  await relationshipChanged()
}

async function accountUpdated(account: Account) {
  activeAccount.value = account
  showEditAccount.value = false
  await relationshipChanged()
}

function platformLabel(platform: string) {
  const labels: Record<string, string> = {
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
  return labels[platform] ?? platform
}

watch(
  () => route.fullPath,
  () => {
    if (route.path === '/admin/resource-pools') {
      restoreFromRoute()
    }
  }
)

onMounted(async () => {
  restoreFromRoute()
  const [, proxyResult] = await Promise.allSettled([refreshWorkspace(), proxiesAPI.getAll()])
  if (proxyResult.status === 'fulfilled') {
    proxies.value = proxyResult.value
  }
  if (selectedGroupId.value && !selectedGroup.value) {
    await applySelection({ scope: 'all' })
  }
})
</script>

<style scoped>
.resource-pools-container {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
  height: calc(100vh - 64px - 4rem); /* 100vh - AppHeader(64px) - AppLayout lg:p-8 padding(4rem) */
  overflow: hidden;
}

@media (max-width: 1023px) {
  .resource-pools-container {
    height: calc(100vh - 64px - 3rem); /* md:p-6 padding(3rem) */
  }
}

@media (max-width: 767px) {
  .resource-pools-container {
    height: auto;
    min-height: calc(100vh - 64px - 2rem);
    overflow: visible;
  }
}

.page-top-header {
  flex-shrink: 0;
}

.mobile-pool-select {
  flex-shrink: 0;
}

.workspace-grid {
  flex: 1;
  min-height: 0;
  height: 100%;
  display: grid;
  grid-template-columns: 288px minmax(0, 1fr);
  gap: 0;
  align-items: stretch;
  overflow: hidden;
}

@media (min-width: 640px) {
  .workspace-grid {
    grid-template-columns: 320px minmax(0, 1fr);
  }
}

/* 左侧菜单独立固定，不随右侧内容滚动 */
.workspace-sidebar {
  height: 100%;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

/* 右侧内容区域独立纵向滚动 */
.workspace-main {
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  display: flex;
  flex-direction: column;
  gap: 0;
  padding-right: 4px;
  scrollbar-gutter: stable;
}

.workspace-main::-webkit-scrollbar {
  width: 6px;
}
.workspace-main::-webkit-scrollbar-track {
  background: transparent;
}
.workspace-main::-webkit-scrollbar-thumb {
  background-color: rgba(156, 163, 175, 0.35);
  border-radius: 9999px;
}
.workspace-main::-webkit-scrollbar-thumb:hover {
  background-color: rgba(156, 163, 175, 0.6);
}

.workspace-grid.is-narrow {
  grid-template-columns: 1fr;
  height: auto;
  overflow: visible;
}

.workspace-grid.is-narrow .workspace-main {
  height: auto;
  overflow: visible;
  padding-right: 0;
}
</style>
