<template>
  <el-dialog
    :model-value="show"
    :title="t('admin.users.platformQuota.title')"
    width="860px"
    align-center
    @update:model-value="(v: boolean) => !v && emit('close')"
    @close="emit('close')"
  >
    <div v-if="user" class="space-y-4">
      <el-alert
        v-if="hasActiveSubscription"
        type="warning"
        :closable="false"
        show-icon
      >
        {{ t('admin.users.platformQuota.subscriptionWarning') }}
      </el-alert>

      <p class="text-sm text-gray-600 dark:text-gray-400">
        {{ t('admin.users.platformQuota.subtitle', { email: user.email }) }}
      </p>

      <div v-if="loading" v-loading="true" class="h-40" element-loading-background="transparent" />

      <template v-else>
        <el-table :data="quotas" size="small" :show-header="true">
          <el-table-column prop="platform" :label="t('admin.users.platformQuota.columns.platform')" width="110">
            <template #default="{ row }">
              <span class="font-mono text-gray-900 dark:text-white">{{ row.platform }}</span>
            </template>
          </el-table-column>
          <el-table-column
            v-for="win in (['daily', 'weekly', 'monthly'] as const)"
            :key="win"
            :label="t(`admin.users.platformQuota.columns.${win}`)"
            min-width="170"
          >
            <template #default="{ row }">
              <div class="flex items-center gap-1">
                <el-input
                  v-model.number="row[`${win}_limit_usd`]"
                  type="number"
                  :min="0"
                  step="0.01"
                  class="!w-28"
                  :placeholder="t('admin.users.platformQuota.placeholder')"
                />
                <el-tooltip
                  :content="savedConfigured.has(row.platform)
                    ? t('admin.users.platformQuota.reset.button')
                    : t('admin.users.platformQuota.reset.unavailable')"
                  placement="top"
                >
                  <span>
                    <el-button
                      link
                      :icon="RefreshRightIcon"
                      :disabled="!!resetting[`${row.platform}.${win}`] || !savedConfigured.has(row.platform)"
                      :loading="!!resetting[`${row.platform}.${win}`]"
                      @click="onReset(row.platform, win)"
                    />
                  </span>
                </el-tooltip>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t('admin.users.platformQuota.columns.usage')" min-width="170">
            <template #default="{ row }">
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ formatUsage(row.daily_usage_usd) }} / {{ formatUsage(row.weekly_usage_usd) }} / {{ formatUsage(row.monthly_usage_usd) }}
              </span>
            </template>
          </el-table-column>
        </el-table>

        <p class="text-xs text-gray-500">{{ t('admin.users.platformQuota.hint') }}</p>
        <div class="mt-1">
          <el-button plain size="small" @click="onClearAll">
            {{ t('admin.users.platformQuota.clearAll') }}
          </el-button>
        </div>
      </template>
    </div>

    <template #footer>
      <el-button @click="emit('close')">{{ t('admin.users.platformQuota.cancel') }}</el-button>
      <el-button type="primary" :loading="submitting" :disabled="loading" @click="onSave">
        {{ submitting ? t('admin.users.platformQuota.saving') : t('admin.users.platformQuota.save') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import { RefreshRight as RefreshRightIcon } from '@element-plus/icons-vue'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { platformQuotaPlatforms } from '@/api/admin/users'
import type { AdminUser, PlatformQuotaItem, PlatformQuotaPlatform, PlatformQuotaWindow } from '@/types'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])

const { t } = useI18n()
const appStore = useAppStore()

interface QuotaRow {
  platform: PlatformQuotaPlatform
  daily_limit_usd: number | null
  weekly_limit_usd: number | null
  monthly_limit_usd: number | null
  daily_usage_usd: number
  weekly_usage_usd: number
  monthly_usage_usd: number
}

const hasActiveSubscription = computed(() =>
  props.user?.subscriptions?.some((s) => s.status === 'active') ?? false
)

const loading = ref(false)
const submitting = ref(false)
const resetting = reactive<Record<string, boolean>>({})
const quotas = ref<QuotaRow[]>([])
// 已保存且至少配置了一档限额的平台。只有这些平台在后端有配额记录，重置用量窗口才有对象。
const savedConfigured = ref<Set<PlatformQuotaPlatform>>(new Set())

function configuredPlatforms(items: PlatformQuotaItem[]): Set<PlatformQuotaPlatform> {
  const out = new Set<PlatformQuotaPlatform>()
  for (const it of items) {
    if (it.daily_limit_usd != null || it.weekly_limit_usd != null || it.monthly_limit_usd != null) {
      out.add(it.platform)
    }
  }
  return out
}

function emptyRow(p: PlatformQuotaPlatform): QuotaRow {
  return {
    platform: p,
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    daily_usage_usd: 0,
    weekly_usage_usd: 0,
    monthly_usage_usd: 0,
  }
}

function normalize(items: PlatformQuotaItem[]): QuotaRow[] {
  const byPlatform = new Map<PlatformQuotaPlatform, PlatformQuotaItem>()
  for (const it of items) byPlatform.set(it.platform, it)
  return platformQuotaPlatforms().map((p) => {
    const it = byPlatform.get(p)
    if (!it) return emptyRow(p)
    return {
      platform: p,
      daily_limit_usd: it.daily_limit_usd ?? null,
      weekly_limit_usd: it.weekly_limit_usd ?? null,
      monthly_limit_usd: it.monthly_limit_usd ?? null,
      daily_usage_usd: it.daily_usage_usd ?? 0,
      weekly_usage_usd: it.weekly_usage_usd ?? 0,
      monthly_usage_usd: it.monthly_usage_usd ?? 0,
    }
  })
}

function formatUsage(n: number): string {
  if (n == null || Number.isNaN(n)) return '-'
  return n.toFixed(2)
}

async function load() {
  if (!props.user) return
  loading.value = true
  try {
    const data = await adminAPI.users.getPlatformQuotas(props.user.id)
    quotas.value = normalize(data.platform_quotas || [])
    savedConfigured.value = configuredPlatforms(data.platform_quotas || [])
  } catch {
    appStore.showError(t('admin.users.platformQuota.loadFailed'))
    quotas.value = platformQuotaPlatforms().map(emptyRow)
    savedConfigured.value = new Set()
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (s) => { if (s && props.user) load() },
)

async function onClearAll() {
  // 二次确认：一键清空全部平台的 daily/weekly/monthly 限额属于高风险批量操作
  try {
    await ElMessageBox.confirm(
      t('admin.users.platformQuota.clearAllConfirm'),
      t('common.confirm'),
      {
        type: 'warning',
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        autofocus: false
      }
    )
  } catch {
    return
  }
  for (const row of quotas.value) {
    row.daily_limit_usd = null
    row.weekly_limit_usd = null
    row.monthly_limit_usd = null
  }
}

async function onSave() {
  if (!props.user) return
  // 拒绝非法数值，避免 normalizeLimit 将其静默转换为 null（无限额）。
  const invalid: string[] = []
  for (const row of quotas.value) {
    for (const win of ['daily', 'weekly', 'monthly'] as const) {
      const v = row[`${win}_limit_usd` as const]
      if (typeof v === 'number' && (!Number.isFinite(v) || v < 0)) {
        invalid.push(`${row.platform}.${win}`)
      }
    }
  }
  if (invalid.length > 0) {
    appStore.showError(t('admin.users.platformQuota.invalidNumber', { fields: invalid.join(', ') }))
    return
  }

  submitting.value = true
  try {
    const payload = quotas.value.map((r) => ({
      platform: r.platform,
      daily_limit_usd: normalizeLimit(r.daily_limit_usd),
      weekly_limit_usd: normalizeLimit(r.weekly_limit_usd),
      monthly_limit_usd: normalizeLimit(r.monthly_limit_usd),
    }))
    await adminAPI.users.updatePlatformQuotas(props.user.id, payload)
    appStore.showSuccess(t('admin.users.platformQuota.updateSuccess'))
    emit('success')
    emit('close')
  } catch (e: any) {
    appStore.showError(e?.response?.data?.message || t('admin.users.platformQuota.updateFailed'))
  } finally {
    submitting.value = false
  }
}

// 仅在合法输入下返回数字：null/undefined/NaN/±Inf/负数 → null（视为"无限额"）
function normalizeLimit(v: number | null | undefined): number | null {
  if (v === null || v === undefined) return null
  if (typeof v === 'number' && Number.isFinite(v) && v >= 0) return v
  return null
}

async function onReset(platform: PlatformQuotaPlatform, quotaWindow: PlatformQuotaWindow) {
  if (!props.user) return
  const windowLabel = t(`admin.users.platformQuota.window${quotaWindow.charAt(0).toUpperCase() + quotaWindow.slice(1)}`)
  try {
    await ElMessageBox.confirm(
      t('admin.users.platformQuota.reset.confirm', { platform, window: windowLabel }),
      t('common.confirm'),
      {
        type: 'warning',
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        autofocus: false
      }
    )
  } catch {
    return
  }
  const key = `${platform}.${quotaWindow}`
  resetting[key] = true
  try {
    const data = await adminAPI.users.resetPlatformQuotaWindow(props.user.id, platform, quotaWindow)
    quotas.value = normalize(data.platform_quotas || [])
    savedConfigured.value = configuredPlatforms(data.platform_quotas || [])
    appStore.showSuccess(t('admin.users.platformQuota.reset.success', { platform, window: windowLabel }))
  } catch (e: any) {
    appStore.showError(e?.response?.data?.message || t('admin.users.platformQuota.reset.failed'))
  } finally {
    resetting[key] = false
  }
}
</script>
