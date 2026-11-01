<template>
  <el-dialog
    :model-value="show"
    :title="t('admin.users.balanceHistoryTitle')"
    width="860px"
    align-center
    :close-on-click-modal="true"
    @update:model-value="(v: boolean) => !v && emit('close')"
    @close="emit('close')"
  >
    <div v-if="user" class="space-y-4">
      <!-- 用户信息头部 -->
      <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30">
            <span class="text-lg font-medium text-primary-700 dark:text-primary-300">
              {{ user.email.charAt(0).toUpperCase() }}
            </span>
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <p class="truncate font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
              <el-tag v-if="user.deleted_at" size="small" type="danger" effect="plain" disable-transitions class="flex-shrink-0">
                {{ t('admin.usage.userDeletedBadge') }}
              </el-tag>
              <span
                v-if="user.username"
                class="flex-shrink-0 rounded bg-primary-50 px-1.5 py-0.5 text-xs text-primary-600 dark:bg-primary-900/20 dark:text-primary-400"
              >
                {{ user.username }}
              </span>
            </div>
            <p class="text-xs text-gray-400 dark:text-dark-500">
              {{ t('admin.users.createdAt') }}: {{ formatDateTime(user.created_at) }}
            </p>
          </div>
          <!-- 当前余额 -->
          <div class="flex-shrink-0 text-right">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.users.currentBalance') }}</p>
            <p class="text-xl font-bold text-gray-900 dark:text-white">
              ${{ user.balance?.toFixed(2) || '0.00' }}
            </p>
          </div>
        </div>
        <div class="mt-2.5 flex items-center justify-between border-t border-gray-200/60 pt-2.5 dark:border-dark-600/60">
          <p class="min-w-0 flex-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="user.notes || ''">
            <template v-if="user.notes">{{ t('admin.users.notes') }}: {{ user.notes }}</template>
            <template v-else>&nbsp;</template>
          </p>
          <p class="ml-4 flex-shrink-0 text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.users.totalRecharged') }}: <span class="font-semibold text-emerald-600 dark:text-emerald-400">${{ totalRecharged.toFixed(2) }}</span>
          </p>
        </div>
      </div>

      <!-- 类型筛选 + 操作按钮 -->
      <div class="flex items-center gap-3">
        <el-select v-model="typeFilter" class="!w-56" @change="loadHistory(1)">
          <el-option
            v-for="opt in typeOptions"
            :key="opt.value"
            :value="opt.value"
            :label="opt.label"
          />
        </el-select>
        <el-button v-if="!hideActions" plain :icon="PlusIcon" type="success" @click="emit('deposit')">
          {{ t('admin.users.deposit') }}
        </el-button>
        <el-button v-if="!hideActions" plain :icon="MinusIcon" type="warning" @click="emit('withdraw')">
          {{ t('admin.users.withdraw') }}
        </el-button>
      </div>

      <!-- Loading -->
      <div v-if="loading" v-loading="true" class="h-32" />

      <!-- 空状态 -->
      <el-empty v-else-if="history.length === 0" :description="t('admin.users.noBalanceHistory')" :image-size="80" />

      <!-- 历史列表 -->
      <div v-else class="max-h-[28rem] space-y-3 overflow-y-auto">
        <div
          v-for="item in history"
          :key="item.id"
          class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="flex items-start justify-between">
            <!-- 左侧：类型图标 + 描述 -->
            <div class="flex items-start gap-3">
              <div :class="['flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg', getIconBg(item)]">
                <el-icon :class="getIconColor(item)">
                  <Coin v-if="isBalanceType(item.type)" />
                  <Present v-else-if="isSubscriptionType(item.type)" />
                  <Lightning v-else />
                </el-icon>
              </div>
              <div>
                <p class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ getItemTitle(item) }}
                </p>
                <p v-if="item.notes" class="mt-0.5 text-xs text-gray-500 dark:text-dark-400" :title="item.notes">
                  {{ item.notes.length > 60 ? item.notes.substring(0, 55) + '...' : item.notes }}
                </p>
                <p class="mt-0.5 text-xs text-gray-400 dark:text-dark-500">
                  {{ formatDateTime(item.used_at || item.created_at) }}
                </p>
              </div>
            </div>
            <!-- 右侧：数值 -->
            <div class="text-right">
              <p :class="['text-sm font-semibold', getValueColor(item)]">
                {{ formatValue(item) }}
              </p>
              <p v-if="isAdminType(item.type)" class="text-xs text-gray-400 dark:text-dark-500">
                {{ t('redeem.adminAdjustment') }}
              </p>
              <p v-else class="font-mono text-xs text-gray-400 dark:text-dark-500">
                {{ item.code.slice(0, 8) }}...
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- 分页 -->
      <div v-if="totalPages > 1" class="flex items-center justify-center gap-2 pt-2">
        <el-button size="small" :disabled="currentPage <= 1" @click="loadHistory(currentPage - 1)">
          {{ t('pagination.previous') }}
        </el-button>
        <span class="text-sm text-gray-500 dark:text-dark-400">
          {{ currentPage }} / {{ totalPages }}
        </span>
        <el-button size="small" :disabled="currentPage >= totalPages" @click="loadHistory(currentPage + 1)">
          {{ t('pagination.next') }}
        </el-button>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Coin, Lightning, Present, Plus as PlusIcon, Minus as MinusIcon } from '@element-plus/icons-vue'
import { adminAPI, type BalanceHistoryItem } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import type { AdminUser } from '@/types'

const props = defineProps<{ show: boolean; user: AdminUser | null; hideActions?: boolean }>()
const emit = defineEmits(['close', 'deposit', 'withdraw'])
const { t } = useI18n()

const history = ref<BalanceHistoryItem[]>([])
const loading = ref(false)
const currentPage = ref(1)
const total = ref(0)
const totalRecharged = ref(0)
const pageSize = 15
const typeFilter = ref('')
let requestVersion = 0

onUnmounted(() => { requestVersion++ })

const totalPages = computed(() => Math.ceil(total.value / pageSize) || 1)

// 类型筛选选项
const typeOptions = computed(() => [
  { value: '', label: t('admin.users.allTypes') },
  { value: 'balance', label: t('admin.users.typeBalance') },
  { value: 'affiliate_balance', label: t('admin.users.typeAffiliateBalance') },
  { value: 'admin_balance', label: t('admin.users.typeAdminBalance') },
  { value: 'concurrency', label: t('admin.users.typeConcurrency') },
  { value: 'admin_concurrency', label: t('admin.users.typeAdminConcurrency') },
  { value: 'subscription', label: t('admin.users.typeSubscription') }
])

watch(() => props.show, (v) => {
  requestVersion++
  if (v && props.user) {
    typeFilter.value = ''
    loadHistory(1)
  }
})

const loadHistory = async (page: number) => {
  if (!props.user) return
  const version = ++requestVersion
  loading.value = true
  currentPage.value = page
  try {
    const res = await adminAPI.users.getUserBalanceHistory(
      props.user.id,
      page,
      pageSize,
      typeFilter.value || undefined
    )
    if (version !== requestVersion) return
    history.value = res.items || []
    total.value = res.total || 0
    totalRecharged.value = res.total_recharged || 0
  } catch (error) {
    if (version !== requestVersion) return
    console.error('Failed to load balance history:', error)
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

// 类型判断
const isAdminType = (type: string) => type === 'admin_balance' || type === 'admin_concurrency'
const isBalanceType = (type: string) => type === 'balance' || type === 'admin_balance' || type === 'affiliate_balance'
const isSubscriptionType = (type: string) => type === 'subscription'

// 图标背景色
const getIconBg = (item: BalanceHistoryItem) => {
  if (isBalanceType(item.type)) {
    return item.value >= 0
      ? 'bg-emerald-100 dark:bg-emerald-900/30'
      : 'bg-red-100 dark:bg-red-900/30'
  }
  if (isSubscriptionType(item.type)) return 'bg-purple-100 dark:bg-purple-900/30'
  return item.value >= 0
    ? 'bg-blue-100 dark:bg-blue-900/30'
    : 'bg-orange-100 dark:bg-orange-900/30'
}

// 图标颜色
const getIconColor = (item: BalanceHistoryItem) => {
  if (isBalanceType(item.type)) {
    return item.value >= 0
      ? 'text-emerald-600 dark:text-emerald-400'
      : 'text-red-600 dark:text-red-400'
  }
  if (isSubscriptionType(item.type)) return 'text-purple-600 dark:text-purple-400'
  return item.value >= 0
    ? 'text-blue-600 dark:text-blue-400'
    : 'text-orange-600 dark:text-orange-400'
}

// 数值颜色
const getValueColor = (item: BalanceHistoryItem) => {
  if (isBalanceType(item.type)) {
    return item.value >= 0
      ? 'text-emerald-600 dark:text-emerald-400'
      : 'text-red-600 dark:text-red-400'
  }
  if (isSubscriptionType(item.type)) return 'text-purple-600 dark:text-purple-400'
  return item.value >= 0
    ? 'text-blue-600 dark:text-blue-400'
    : 'text-orange-600 dark:text-orange-400'
}

// 条目标题
const getItemTitle = (item: BalanceHistoryItem) => {
  switch (item.type) {
    case 'balance':
      return t('redeem.balanceAddedRedeem')
    case 'affiliate_balance':
      return t('redeem.balanceAddedAffiliate')
    case 'admin_balance':
      return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
    case 'concurrency':
      return t('redeem.concurrencyAddedRedeem')
    case 'admin_concurrency':
      return item.value >= 0 ? t('redeem.concurrencyAddedAdmin') : t('redeem.concurrencyReducedAdmin')
    case 'subscription':
      return t('redeem.subscriptionAssigned')
    default:
      return t('common.unknown')
  }
}

// 格式化数值
const formatValue = (item: BalanceHistoryItem) => {
  if (isBalanceType(item.type)) {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}$${item.value.toFixed(2)}`
  }
  if (isSubscriptionType(item.type)) {
    const days = item.validity_days || Math.round(item.value)
    const groupName = item.group?.name || ''
    return groupName ? `${days}d - ${groupName}` : `${days}d`
  }
  const sign = item.value >= 0 ? '+' : ''
  return `${sign}${item.value}`
}
</script>
