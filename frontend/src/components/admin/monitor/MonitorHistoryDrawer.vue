<template>
  <el-drawer
    :model-value="show"
    direction="rtl"
    :size="drawerSize"
    :title="t('admin.channelMonitor.historyTitle', { name: monitor?.name ?? '' })"
    destroy-on-close
    class="monitor-history-drawer"
    @update:model-value="(value: boolean) => !value && emit('close')"
  >
    <div class="flex h-full min-h-0 flex-col">
      <!-- Tab 切换（与视图头部的手写 tabs 保持一致） -->
      <div
        class="tabs mb-4 inline-flex w-full max-w-md shrink-0 flex-wrap"
        role="tablist"
        :aria-label="t('admin.channelMonitor.historyTabAria')"
      >
        <button
          v-for="tab in tabs"
          :key="tab"
          type="button"
          role="tab"
          class="tab flex-1"
          :class="activeTab === tab ? 'tab-active' : ''"
          :aria-selected="activeTab === tab"
          @click="activeTab = tab"
        >
          {{ tabLabel(tab) }}
        </button>
      </div>

      <div v-if="loading" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </div>

      <template v-else>
        <!-- 检测历史 -->
        <div v-if="activeTab === 'history'" class="min-h-0 flex-1 space-y-2 overflow-y-auto pb-4">
          <p
            v-if="history.length === 0"
            class="py-8 text-center text-sm text-gray-500 dark:text-gray-400"
          >
            {{ t('admin.channelMonitor.historyEmpty') }}
          </p>
          <div
            v-for="h in history"
            :key="h.id"
            class="rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-600"
          >
            <div class="flex items-center justify-between gap-2">
              <div class="flex min-w-0 flex-col">
                <div class="flex items-center gap-1.5">
                  <span class="font-medium text-gray-900 dark:text-white">{{ formatMonitorModel(h.model) }}</span>
                  <span
                    v-if="h.substituted"
                    class="inline-flex cursor-pointer items-center rounded-full bg-indigo-100 px-2 py-0.5 text-[11px] font-medium text-indigo-700 hover:bg-indigo-200 dark:bg-indigo-500/15 dark:text-indigo-300"
                    :title="t('admin.channelMonitor.substitutedHint')"
                    @click="jumpToAnomaly(h.anomaly_id)"
                  >
                    {{ t('admin.channelMonitor.substitutedTag') }}
                  </span>
                </div>
                <span v-if="h.message" class="truncate text-xs text-gray-500 dark:text-gray-400" :title="h.message">
                  {{ h.message }}
                </span>
                <span class="text-xs text-gray-400 dark:text-gray-500">{{ formatDateTime(h.checked_at) }}</span>
              </div>
              <div class="flex shrink-0 items-center gap-2">
                <span
                  class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px]"
                  :class="statusBadgeClass(h.status)"
                >
                  {{ statusLabel(h.status) }}
                </span>
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ formatLatency(h.latency_ms) }} ms
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- 探测异常 -->
        <div v-else class="min-h-0 flex-1 space-y-3 overflow-y-auto pb-4">
          <p
            v-if="anomalies.length === 0"
            class="py-8 text-center text-sm text-gray-500 dark:text-gray-400"
          >
            {{ t('admin.channelMonitor.anomaliesEmpty') }}
          </p>
          <div
            v-for="a in anomalies"
            :key="a.id"
            class="rounded-lg border border-gray-200 dark:border-dark-600"
            :class="expandedAnomalyId === a.id ? 'ring-1 ring-primary-300 dark:ring-primary-500/40' : ''"
          >
            <button
              type="button"
              class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm"
              @click="toggleAnomaly(a.id)"
            >
              <div class="flex min-w-0 flex-col">
                <div class="flex flex-wrap items-center gap-1.5">
                  <span class="font-medium text-gray-900 dark:text-white">{{ formatMonitorModel(a.model) }}</span>
                  <span
                    class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium"
                    :class="resolutionBadgeClass(a.resolution)"
                  >
                    {{ resolutionLabel(a.resolution) }}
                  </span>
                  <span
                    v-if="a.reason"
                    class="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-[11px] text-gray-600 dark:bg-dark-700 dark:text-gray-300"
                  >
                    {{ reasonLabel(a.reason) }}
                  </span>
                </div>
                <span class="text-xs text-gray-400 dark:text-gray-500">
                  {{ triggerLabel(a.trigger) }} ·
                  {{ t('admin.channelMonitor.anomalyConsecutiveSeq', { n: a.consecutive_seq }) }} ·
                  {{ t('admin.channelMonitor.anomalyAttemptCount', { n: a.attempt_count }) }} ·
                  {{ formatDateTime(a.cycle_started_at) }}
                </span>
              </div>
              <Icon
                name="chevronDown"
                size="sm"
                class="shrink-0 text-gray-400 transition-transform"
                :class="expandedAnomalyId === a.id ? 'rotate-180' : ''"
              />
            </button>
            <div v-if="expandedAnomalyId === a.id" class="border-t border-gray-100 px-3 py-2 dark:border-dark-700">
              <p v-if="a.reference_checked_at" class="mb-2 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.channelMonitor.anomalyReferenceAt', { time: formatDateTime(a.reference_checked_at) }) }}
              </p>
              <div class="space-y-1.5">
                <div
                  v-for="at in a.attempts"
                  :key="at.seq"
                  class="flex items-start justify-between gap-2 rounded-md bg-gray-50 px-2.5 py-1.5 text-xs dark:bg-dark-700/60"
                >
                  <div class="flex min-w-0 flex-col">
                    <span class="text-gray-700 dark:text-gray-200">
                      #{{ at.seq }}
                      {{ t('admin.channelMonitor.anomalyCheckedAt', { time: formatDateTime(at.checked_at) }) }}
                    </span>
                    <span v-if="at.message" class="break-all text-gray-500 dark:text-gray-400">{{ at.message }}</span>
                  </div>
                  <div class="flex shrink-0 items-center gap-1.5">
                    <span
                      class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px]"
                      :class="statusBadgeClass(at.status)"
                    >
                      {{ statusLabel(at.status) }}
                    </span>
                    <span class="text-gray-500 dark:text-gray-400">{{ formatLatency(at.latency_ms) }} ms</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMediaQuery } from '@vueuse/core'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import type {
  ChannelMonitor,
  HistoryItem,
  ProbeAnomaly,
} from '@/api/admin/channelMonitor'
import Icon from '@/components/icons/Icon.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

const props = defineProps<{
  show: boolean
  monitor: ChannelMonitor | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t, locale } = useI18n()
const appStore = useAppStore()
const { statusLabel, statusBadgeClass, formatLatency, formatMonitorModel } = useChannelMonitorFormat()

// 窄屏用接近全宽的抽屉，桌面固定 520px。
const isNarrow = useMediaQuery('(max-width: 640px)')
const drawerSize = computed(() => (isNarrow.value ? '92%' : '520px'))

type HistoryTab = 'history' | 'anomalies'
const tabs: HistoryTab[] = ['history', 'anomalies']
const activeTab = ref<HistoryTab>('history')
const loading = ref(false)
const history = ref<HistoryItem[]>([])
const anomalies = ref<ProbeAnomaly[]>([])
const expandedAnomalyId = ref<number | null>(null)

function tabLabel(tab: HistoryTab): string {
  return tab === 'history'
    ? t('admin.channelMonitor.tabHistory')
    : t('admin.channelMonitor.tabAnomalies')
}

function triggerLabel(trigger: ProbeAnomaly['trigger']): string {
  return t(`admin.channelMonitor.anomalyTrigger.${trigger}`)
}

function resolutionLabel(resolution: ProbeAnomaly['resolution']): string {
  return t(`admin.channelMonitor.anomalyResolution.${resolution}`)
}

function reasonLabel(reason: ProbeAnomaly['reason']): string {
  return t(`admin.channelMonitor.anomalyReason.${reason}`)
}

function resolutionBadgeClass(resolution: ProbeAnomaly['resolution']): string {
  if (resolution === 'substituted') {
    return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-300'
  }
  return 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
}

function toggleAnomaly(id: number) {
  expandedAnomalyId.value = expandedAnomalyId.value === id ? null : id
}

// 历史里的「已替代」标签点击后跳到异常 Tab 并展开对应记录。
function jumpToAnomaly(anomalyId: number | null | undefined) {
  if (!anomalyId) return
  activeTab.value = 'anomalies'
  expandedAnomalyId.value = anomalyId
}

function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '-'
  const ts = Date.parse(iso)
  if (Number.isNaN(ts)) return iso
  return new Date(ts).toLocaleString(locale.value || 'zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

async function load() {
  const monitor = props.monitor
  if (!monitor) return
  loading.value = true
  expandedAnomalyId.value = null
  try {
    const [historyRes, anomalyRes] = await Promise.all([
      adminAPI.channelMonitor.listHistory(monitor.id, { limit: 100 }),
      adminAPI.channelMonitor.listMonitorProbeAnomalies(monitor.id, { page: 1, page_size: 50 }),
    ])
    history.value = historyRes.items || []
    anomalies.value = anomalyRes.items || []
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.historyLoadError')))
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (show) => {
    if (show) {
      activeTab.value = 'history'
      void load()
    }
  }
)
</script>
