<template>
  <el-dialog
    :model-value="modelValue"
    width="820px"
    class="assign-dialog !rounded-2xl"
    destroy-on-close
    @update:model-value="onVisibleChange"
    @open="load"
  >
    <template #header>
      <div class="flex items-center gap-2.5">
        <div class="w-8 h-8 rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-950/60 dark:text-primary-400 flex items-center justify-center shrink-0">
          <PlatformIcon v-if="group" :platform="group.platform" size="sm" />
          <el-icon v-else class="text-base"><Plus /></el-icon>
        </div>
        <div class="min-w-0">
          <h3 class="text-base font-bold text-gray-900 dark:text-gray-100 m-0 truncate">
            {{ t('resourcePool.assign.title', { name: group?.name ?? '' }) }}
          </h3>
          <p class="text-xs text-gray-400 dark:text-dark-400 m-0 mt-0.5">
            从全部渠道账号中挑选并批量加入当前资源池
          </p>
        </div>
      </div>
    </template>

    <!-- Search and Filter bar inside Dialog -->
    <div class="flex items-center gap-2 mb-3">
      <el-input
        v-model="search"
        clearable
        :prefix-icon="Search"
        :placeholder="t('resourcePool.accounts.search')"
        class="flex-1"
        @keyup.enter="load"
        @clear="load"
      />
      <el-button type="primary" plain :loading="loading" @click="load">
        {{ t('common.search') }}
      </el-button>
    </div>

    <!-- Accounts Table -->
    <el-table
      v-loading="loading"
      :data="accounts"
      row-key="id"
      max-height="420"
      class="assign-table !rounded-xl border border-gray-100 dark:border-dark-800"
      @selection-change="onSelectionChange"
    >
      <el-table-column type="selection" width="46" align="center" />

      <!-- Account Info -->
      <el-table-column :label="t('resourcePool.accounts.account')" min-width="220">
        <template #default="{ row }">
          <div class="flex items-center gap-2 py-1">
            <div class="w-6 h-6 rounded-md bg-gray-100 dark:bg-dark-800 flex items-center justify-center shrink-0 text-gray-600 dark:text-gray-300">
              <PlatformIcon :platform="row.platform" size="xs" />
            </div>
            <div class="min-w-0">
              <div class="font-bold text-xs text-gray-900 dark:text-gray-100 truncate" :title="row.name">
                {{ row.name }}
              </div>
              <div class="text-[11px] text-gray-400 dark:text-dark-400 font-mono">
                #{{ row.id }} · {{ row.platform }} / {{ row.type }}
              </div>
            </div>
          </div>
        </template>
      </el-table-column>

      <!-- Health -->
      <el-table-column :label="t('resourcePool.accounts.status')" width="110">
        <template #default="{ row }">
          <span
            :class="[
              'inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold',
              row.status === 'active' && row.schedulable && !row.rate_limit_reset_at
                ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
                : 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-400'
            ]"
          >
            {{ row.status === 'active' ? (row.rate_limit_reset_at ? t('resourcePool.status.limited') : t('resourcePool.status.healthy')) : t('resourcePool.status.inactive') }}
          </span>
        </template>
      </el-table-column>

      <!-- Existing Groups -->
      <el-table-column :label="t('resourcePool.accounts.groups')" min-width="180">
        <template #default="{ row }">
          <div class="flex items-center gap-1 flex-wrap">
            <span
              v-for="grp in (row.groups ?? [])"
              :key="grp.id"
              class="inline-flex items-center px-1.5 py-0.2 rounded text-[11px] bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-300"
            >
              {{ grp.name }}
            </span>
            <span v-if="!(row.groups?.length)" class="text-xs text-amber-600 dark:text-amber-400">
              {{ t('resourcePool.navigator.ungrouped') }}
            </span>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- Pagination -->
    <div class="flex items-center justify-between mt-3 text-xs text-gray-400">
      <span>已加载 {{ accounts.length }} 条候选账号</span>
      <el-pagination
        layout="total, prev, pager, next"
        :current-page="page"
        :page-size="pageSize"
        :total="total"
        small
        background
        @update:current-page="changePage"
      />
    </div>

    <!-- Footer -->
    <template #footer>
      <div class="flex items-center justify-between">
        <span class="text-xs text-gray-500 dark:text-gray-400">
          已选择 <strong class="text-primary-600 dark:text-primary-400">{{ selected.length }}</strong> 个账号
        </span>
        <div class="flex items-center gap-2">
          <el-button @click="$emit('update:modelValue', false)">
            {{ t('common.cancel') }}
          </el-button>
          <el-button
            type="primary"
            :loading="submitting"
            :disabled="selected.length === 0"
            class="!rounded-lg shadow-sm"
            @click="submit"
          >
            {{ t('resourcePool.assign.confirm', { count: selected.length }) }}
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { accountsAPI } from '@/api/admin'
import type { AccountListItem, AdminGroup } from '@/types'

const props = defineProps<{
  modelValue: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  changed: [result: { success: number; failed: number }]
}>()

const { t } = useI18n()
const accounts = ref<AccountListItem[]>([])
const selected = ref<AccountListItem[]>([])
const search = ref('')
const loading = ref(false)
const submitting = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)

function onVisibleChange(value: boolean) {
  emit('update:modelValue', value)
}

function onSelectionChange(rows: AccountListItem[]) {
  selected.value = rows
}

function changePage(value: number) {
  page.value = value
  void load()
}

async function load() {
  if (!props.group) return
  loading.value = true
  try {
    const result = await accountsAPI.list(page.value, pageSize, {
      search: search.value || undefined,
      lite: '0'
    })
    // Filter out accounts that already belong to this group
    accounts.value = result.items.filter(
      account => !(account.group_ids ?? []).includes(props.group!.id)
    )
    total.value = Math.max(0, result.total - (result.items.length - accounts.value.length))
  } finally {
    loading.value = false
  }
}

function isMixedChannelConflict(error: unknown) {
  if (typeof error !== 'object' || error === null) return false
  const candidate = error as { status?: number; response?: { status?: number } }
  return candidate.status === 409 || candidate.response?.status === 409
}

async function add(account: AccountListItem, confirmRisk = false) {
  return accountsAPI.setGroupMembership(account.id, props.group!.id, {
    priority: 1,
    confirm_mixed_channel_risk: confirmRisk
  })
}

async function submit() {
  if (!props.group || selected.value.length === 0) return
  submitting.value = true
  let success = 0
  let failed = 0

  for (const account of selected.value) {
    try {
      const risk = await accountsAPI.checkMixedChannelRisk({
        platform: account.platform,
        group_ids: [props.group.id],
        account_id: account.id
      })

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
        await add(account, true)
      } else {
        await add(account)
      }
      success++
    } catch (error) {
      if (isMixedChannelConflict(error)) {
        try {
          await ElMessageBox.confirm(
            t('resourcePool.assign.mixedWarning', { name: account.name }),
            t('resourcePool.assign.riskTitle'),
            {
              type: 'warning',
              confirmButtonText: t('resourcePool.assign.continue'),
              cancelButtonText: t('common.cancel')
            }
          )
          await add(account, true)
          success++
          continue
        } catch {
          // cancelled
        }
      }
      failed++
    }
  }

  submitting.value = false
  emit('changed', { success, failed })
  if (failed) {
    ElMessage.warning(t('resourcePool.assign.partial', { success, failed }))
  } else {
    ElMessage.success(t('resourcePool.assign.success', { count: success }))
  }
  emit('update:modelValue', false)
}
</script>

<style scoped>
.assign-dialog {
  border-radius: 16px;
}
</style>