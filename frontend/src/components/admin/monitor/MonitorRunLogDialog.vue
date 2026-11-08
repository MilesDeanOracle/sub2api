<template>
  <BaseDialog
    :show="show"
    :title="t('admin.channelMonitor.runLogTitle', { name: monitor?.name ?? '' })"
    width="wide"
    :close-on-click-outside="false"
    @close="onClose"
  >
    <!-- 阶段提示 -->
    <div class="mb-3 flex items-center gap-2 text-sm">
      <span
        v-if="phase === 'connecting' || phase === 'running'"
        class="inline-flex items-center gap-1.5 text-primary-600 dark:text-primary-400"
      >
        <Icon name="refresh" size="sm" class="animate-spin" />
        {{ phase === 'connecting' ? t('admin.channelMonitor.runLogConnecting') : t('admin.channelMonitor.runLogRunning') }}
      </span>
      <span v-else-if="phase === 'done'" class="inline-flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
        <Icon name="checkCircle" size="sm" />
        {{ t('admin.channelMonitor.runLogDone') }}
      </span>
      <span v-else-if="phase === 'error'" class="inline-flex items-center gap-1.5 text-red-600 dark:text-red-400">
        <Icon name="exclamationCircle" size="sm" />
        {{ t('admin.channelMonitor.runFailed') }}
      </span>
      <span v-else class="text-gray-400 dark:text-gray-500">{{ t('admin.channelMonitor.runLogAborted') }}</span>
      <span
        v-if="phase === 'running' && waitSeconds !== null"
        class="ml-auto text-xs text-gray-400 dark:text-gray-500"
      >
        {{ t('admin.channelMonitor.runLogWait', { s: waitSeconds }) }}
      </span>
    </div>

    <!-- 探测日志 -->
    <div
      ref="logAreaRef"
      class="max-h-[38vh] space-y-1.5 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50/60 p-3 dark:border-dark-600 dark:bg-dark-900/60"
    >
      <p v-if="logs.length === 0" class="py-4 text-center text-xs text-gray-400 dark:text-gray-500">
        {{ t('admin.channelMonitor.runLogConnecting') }}
      </p>
      <template v-for="(entry, index) in logs" :key="index">
        <!-- 单次尝试 -->
        <div v-if="entry.kind === 'attempt'" class="flex items-start justify-between gap-2 text-xs">
          <div class="flex min-w-0 flex-col">
            <span class="text-gray-700 dark:text-gray-200">
              <span class="text-gray-400 dark:text-gray-500">{{ formatTime(entry.checkedAt) }}</span>
              · {{ formatMonitorModel(entry.model) }}
              <span class="text-gray-400 dark:text-gray-500">#{{ entry.seq }}</span>
            </span>
            <span v-if="entry.message" class="break-all pl-1 text-gray-500 dark:text-gray-400" :title="entry.message">
              {{ entry.message }}
            </span>
          </div>
          <div class="flex shrink-0 items-center gap-1.5">
            <span
              class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px]"
              :class="statusBadgeClass(entry.status)"
            >
              {{ statusLabel(entry.status) }}
            </span>
            <span class="text-gray-500 dark:text-gray-400">{{ formatLatency(entry.latencyMs) }} ms</span>
          </div>
        </div>
        <!-- 退避等待 -->
        <div v-else-if="entry.kind === 'wait'" class="pl-4 text-xs text-gray-400 dark:text-gray-500">
          {{ t('admin.channelMonitor.runLogWait', { s: Math.round(entry.delayMs / 1000) }) }}
        </div>
        <!-- 周期开始等信息行 -->
        <div v-else class="text-xs text-gray-500 dark:text-gray-400">{{ entry.text }}</div>
      </template>
    </div>

    <!-- 最终结果 -->
    <div v-if="phase === 'done'" class="mt-3">
      <p class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.channelMonitor.runLogFinalTitle') }}
      </p>
      <div class="space-y-2">
        <div
          v-for="r in results"
          :key="r.model"
          class="flex items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-600"
        >
          <div class="flex min-w-0 flex-col">
            <span class="font-medium text-gray-900 dark:text-white">{{ formatMonitorModel(r.model) }}</span>
            <span v-if="r.message" class="truncate text-xs text-gray-500 dark:text-gray-400" :title="r.message">
              {{ r.message }}
            </span>
            <MonitorQuotaView :snapshot="r.quota" class="mt-1" />
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <span
              class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px]"
              :class="statusBadgeClass(r.status)"
            >
              {{ statusLabel(r.status) }}
            </span>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatLatency(r.latency_ms) }} ms</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 错误信息 -->
    <div
      v-if="phase === 'error'"
      class="mt-3 break-all rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
    >
      {{ errorMessage }}
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button
          v-if="phase === 'error'"
          class="btn btn-secondary"
          @click="start"
        >
          {{ t('admin.channelMonitor.runLogRetry') }}
        </button>
        <button class="btn btn-primary" @click="onClose">
          {{ phase === 'running' || phase === 'connecting' ? t('common.cancel') : t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  ChannelMonitor,
  CheckResult,
  MonitorRunStreamEvent,
  MonitorStatus,
} from '@/api/admin/channelMonitor'
import { adminAPI } from '@/api/admin'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import MonitorQuotaView from '@/components/common/MonitorQuotaView.vue'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

const props = defineProps<{
  show: boolean
  monitor: ChannelMonitor | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'finished', results: CheckResult[]): void
  (e: 'failed', message: string): void
}>()

const { t } = useI18n()
const { statusLabel, statusBadgeClass, formatLatency, formatMonitorModel } = useChannelMonitorFormat()

type logEntry =
  | { kind: 'attempt'; model: string; seq: number; status: MonitorStatus; latencyMs: number | null; message: string; checkedAt: string }
  | { kind: 'wait'; delayMs: number }
  | { kind: 'info'; text: string }

const phase = ref<'idle' | 'connecting' | 'running' | 'done' | 'error'>('idle')
const logs = ref<logEntry[]>([])
const results = ref<CheckResult[]>([])
const errorMessage = ref('')
const logAreaRef = ref<HTMLElement | null>(null)
const waitSeconds = ref<number | null>(null)
let abortController: AbortController | null = null
let waitTimer: ReturnType<typeof setInterval> | null = null

function stopWaitCountdown() {
  waitSeconds.value = null
  if (waitTimer !== null) {
    clearInterval(waitTimer)
    waitTimer = null
  }
}

function startWaitCountdown(delayMs: number) {
  stopWaitCountdown()
  waitSeconds.value = Math.max(1, Math.round(delayMs / 1000))
  waitTimer = setInterval(() => {
    if (waitSeconds.value !== null && waitSeconds.value > 1) {
      waitSeconds.value -= 1
    } else {
      stopWaitCountdown()
    }
  }, 1000)
}

function formatTime(iso: string): string {
  const ts = Date.parse(iso)
  if (Number.isNaN(ts)) return ''
  return new Date(ts).toLocaleTimeString('zh-CN', { hour12: false })
}

function handleEvent(event: MonitorRunStreamEvent) {
  switch (event.type) {
    case 'cycle_start':
      phase.value = 'running'
      logs.value.push({
        kind: 'info',
        text: t('admin.channelMonitor.runLogStartModels', { models: event.models.join('、') }),
      })
      break
    case 'attempt':
      stopWaitCountdown()
      logs.value.push({
        kind: 'attempt',
        model: event.model,
        seq: event.seq,
        status: event.status,
        latencyMs: event.latency_ms,
        message: event.message,
        checkedAt: event.checked_at,
      })
      break
    case 'wait':
      logs.value.push({ kind: 'wait', delayMs: event.delay_ms })
      startWaitCountdown(event.delay_ms)
      break
    case 'done':
      stopWaitCountdown()
      results.value = event.results || []
      phase.value = 'done'
      emit('finished', results.value)
      break
    case 'error':
      stopWaitCountdown()
      errorMessage.value = event.message
      phase.value = 'error'
      emit('failed', event.message)
      break
  }
}

async function start() {
  const monitor = props.monitor
  if (!monitor) return
  abortController?.abort()
  stopWaitCountdown()
  logs.value = []
  results.value = []
  errorMessage.value = ''
  phase.value = 'connecting'

  abortController = new AbortController()
  try {
    await adminAPI.channelMonitor.runNowStream(monitor.id, handleEvent, abortController.signal)
  } catch (error: unknown) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      phase.value = 'idle'
      return
    }
    const message = error instanceof Error ? error.message : 'Unknown error'
    errorMessage.value = message
    phase.value = 'error'
    emit('failed', message)
  }
}

function onClose() {
  if (phase.value === 'connecting' || phase.value === 'running') {
    // 用户主动取消：中断请求，服务端按「本次不算」丢弃周期
    abortController?.abort()
    stopWaitCountdown()
  }
  emit('close')
}

// 日志新增时滚动到底部
watch(
  () => logs.value.length,
  async () => {
    await nextTick()
    const area = logAreaRef.value
    if (area) area.scrollTop = area.scrollHeight
  }
)

watch(
  () => props.show,
  (show) => {
    if (show) {
      void start()
    } else {
      abortController?.abort()
      stopWaitCountdown()
    }
  }
)
</script>
