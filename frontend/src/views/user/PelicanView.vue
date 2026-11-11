<template>
  <AppLayout>
    <div class="grid gap-5 lg:grid-cols-[20rem_minmax(0,1fr)] lg:items-start 2xl:grid-cols-[20rem_minmax(0,1fr)_19rem]">
      <!-- 左侧：检测配置 -->
      <aside class="card flex flex-col lg:sticky lg:top-4 lg:max-h-[calc(100vh-64px-4rem)]" :aria-busy="configLoading || keysLoading">
        <div class="flex items-center gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <PelicanLogo :size="22" />
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('pelican.configTitle') }}</h2>
        </div>

        <div class="flex-1 space-y-5 overflow-y-auto px-5 py-4">
          <div>
            <label class="input-label" for="pelican-key">{{ t('pelican.keyLabel') }}</label>
            <div class="mb-2 grid grid-cols-2 rounded-xl bg-gray-100 p-1 text-xs dark:bg-dark-900">
              <button
                v-for="mode in (['saved', 'manual'] as KeyMode[])"
                :key="mode"
                type="button"
                class="rounded-lg px-2 py-1.5 font-medium transition-colors"
                :class="keyMode === mode ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-gray-200'"
                :disabled="running"
                @click="setKeyMode(mode)"
              >
                {{ mode === 'saved' ? t('pelican.keyModeSaved') : t('pelican.keyModeManual') }}
              </button>
            </div>

            <template v-if="keyMode === 'saved'">
              <Select
                id="pelican-key"
                :model-value="selectedKeyId ?? ''"
                :options="keyOptions"
                :placeholder="t('pelican.keyPlaceholder')"
                :loading="keysLoading"
                :disabled="running"
                aria-label="API key"
                @update:model-value="onKeyChange"
              />
              <p v-if="!keysLoading && keys.length === 0" class="input-hint text-amber-600 dark:text-amber-400">
                {{ t('pelican.keyEmpty') }}
                <router-link to="/keys" class="ml-1 underline">{{ t('pelican.createKey') }}</router-link>
              </p>
            </template>

            <template v-else>
              <div class="relative">
                <input
                  id="pelican-manual-key"
                  v-model="manualKey"
                  :type="showManualKey ? 'text' : 'password'"
                  class="input pr-10 font-mono text-sm"
                  :placeholder="t('pelican.manualKeyPlaceholder')"
                  :disabled="running"
                  autocomplete="off"
                  spellcheck="false"
                  @blur="onManualKeyBlur"
                  @keyup.enter="onManualKeyBlur"
                >
                <button
                  type="button"
                  class="absolute right-2 top-1/2 -translate-y-1/2 rounded-lg p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                  :aria-label="showManualKey ? t('pelican.hideKey') : t('pelican.showKey')"
                  @click="showManualKey = !showManualKey"
                >
                  <Icon :name="showManualKey ? 'eyeOff' : 'eye'" size="sm" />
                </button>
              </div>
              <p class="input-hint">{{ t('pelican.manualKeyHint') }}</p>
            </template>
          </div>

          <div>
            <div class="mb-1.5 flex items-center justify-between gap-2">
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300" for="pelican-model">{{ t('pelican.modelLabel') }}</label>
              <button
                type="button"
                class="inline-flex items-center gap-1 text-xs text-primary-600 hover:underline disabled:opacity-50 dark:text-primary-400"
                :disabled="!hasCredential || modelsLoading || running"
                @click="reloadModels"
              >
                <Icon name="refresh" size="xs" />{{ t('pelican.modelRefresh') }}
              </button>
            </div>
            <Select
              id="pelican-model"
              :model-value="selectedModel"
              :options="modelOptions"
              :placeholder="modelsLoading ? t('pelican.modelLoading') : t('pelican.modelPlaceholder')"
              :loading="modelsLoading"
              :disabled="!hasCredential || modelsLoading || running"
              :empty-text="t('pelican.modelEmpty')"
              aria-label="Model"
              @update:model-value="onModelChange"
            />
            <p v-if="!modelsLoading && hasCredential && models.length === 0" class="input-hint text-amber-600 dark:text-amber-400">
              {{ t('pelican.modelEmpty') }}
            </p>
          </div>

          <div>
            <label class="input-label" for="pelican-reasoning">{{ t('pelican.reasoningLabel') }}</label>
            <Select
              id="pelican-reasoning"
              :model-value="reasoningEffort"
              :options="reasoningOptions"
              :disabled="running"
              aria-label="Reasoning level"
              @update:model-value="onReasoningChange"
            />
          </div>

          <details class="group rounded-xl border border-gray-200 dark:border-dark-700">
            <summary class="flex cursor-pointer list-none items-center justify-between px-3 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-300">
              <span>{{ t('pelican.baseUrlLabel') }}<span v-if="baseUrlSourceLabel" class="ml-2 text-xs font-normal text-gray-400 dark:text-dark-500">{{ baseUrlSourceLabel }}</span></span>
              <Icon name="chevronDown" size="sm" class="text-gray-400 transition-transform group-open:rotate-180" />
            </summary>
            <div class="border-t border-gray-100 px-3 py-3 dark:border-dark-700">
              <input
                id="pelican-base-url"
                v-model="baseUrl"
                type="url"
                class="input font-mono text-sm"
                :placeholder="baseUrlPlaceholder"
                :disabled="running"
                spellcheck="false"
                @blur="onBaseUrlBlur"
              >
              <p class="input-hint">{{ t('pelican.baseUrlHint') }}</p>
            </div>
          </details>

          <details v-if="config?.test" class="group rounded-xl border border-gray-200 dark:border-dark-700">
            <summary class="flex cursor-pointer list-none items-center justify-between px-3 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-300">
              <span>{{ t('pelican.promptTitle') }}<span class="ml-2 text-xs font-normal text-gray-400 dark:text-dark-500">{{ t('pelican.promptVersion') }} {{ config.test.prompt_version }}</span></span>
              <Icon name="chevronDown" size="sm" class="text-gray-400 transition-transform group-open:rotate-180" />
            </summary>
            <pre class="max-h-64 overflow-auto whitespace-pre-wrap break-words border-t border-gray-100 px-3 py-3 text-xs leading-relaxed text-gray-600 dark:border-dark-700 dark:text-gray-300">{{ config.test.prompt }}</pre>
          </details>
        </div>

        <div class="border-t border-gray-100 px-5 py-4 dark:border-dark-700">
          <button v-if="!running" type="button" class="btn btn-primary w-full" :disabled="!canStart" @click="startRun">
            <Icon name="play" size="sm" />
            {{ runState === 'idle' ? t('pelican.start') : t('pelican.retry') }}
          </button>
          <button v-else type="button" class="btn btn-secondary w-full" @click="cancelRun">
            <Icon name="x" size="sm" />
            {{ t('pelican.cancel') }}
          </button>
          <p class="mt-2 text-center text-xs text-gray-400 dark:text-dark-500">{{ canStart || running ? t('pelican.intro') : t('pelican.emptyResultHint') }}</p>
        </div>
      </aside>

      <!-- 中间：检测结果 -->
      <section class="card flex min-w-0 flex-col" aria-live="polite">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-3 dark:border-dark-700">
          <div class="flex min-w-0 items-center gap-2">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('pelican.resultTitle') }}</h2>
            <span v-if="runState !== 'idle'" class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium" :class="statusBadgeClass">
              <span v-if="running" class="h-1.5 w-1.5 animate-pulse rounded-full bg-current"></span>{{ statusLabel }}
            </span>
            <span v-if="runState !== 'idle' && selectedModel" class="truncate font-mono text-xs text-gray-500 dark:text-dark-400">{{ selectedModel }}</span>
          </div>
          <div v-if="runState !== 'idle' && !running" class="flex items-center gap-2">
            <div class="inline-flex rounded-xl bg-gray-100 p-1 text-xs dark:bg-dark-900">
              <button
                v-for="tab in resultTabs"
                :key="tab"
                type="button"
                class="rounded-lg px-3 py-1 font-medium transition-colors"
                :class="activeTab === tab ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-gray-200'"
                @click="activeTab = tab"
              >
                {{ t(`pelican.${tab}`) }}
              </button>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="!resultSource" :title="t('pelican.copy')" @click="copySource"><Icon name="copy" size="xs" /></button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="!resultSource" :title="t('pelican.download')" @click="downloadCurrent"><Icon name="download" size="xs" /></button>
            <button
              v-if="runState === 'done'"
              type="button"
              data-testid="pelican-report"
              class="btn btn-sm"
              :class="isReported ? 'btn-secondary' : 'btn-primary'"
              :disabled="isReported || reporting || !gatewayRequestId || !resultSource"
              :title="reportButtonTitle"
              @click="openReportDialog"
            >
              <Icon :name="isReported ? 'checkCircle' : 'chatBubble'" size="xs" />
              {{ isReported ? t('pelican.reported') : t('pelican.report') }}
            </button>
          </div>
        </div>

        <dl v-if="runState !== 'idle'" class="grid grid-cols-3 divide-x divide-y divide-gray-100 border-b border-gray-100 dark:divide-dark-700 dark:border-dark-700 md:grid-cols-6 md:divide-y-0">
          <div v-for="metric in metricItems" :key="metric.label" class="px-4 py-3">
            <dt class="text-xs text-gray-500 dark:text-dark-400">{{ metric.label }}</dt>
            <dd class="mt-0.5 text-base font-semibold tabular-nums text-gray-900 dark:text-white">{{ metric.value }}</dd>
          </div>
        </dl>

        <div class="flex-1 p-5">
          <div v-if="runState === 'idle'" class="flex h-full min-h-[24rem] flex-col items-center justify-center rounded-2xl border border-dashed border-gray-200 px-4 text-center dark:border-dark-700">
            <div class="rounded-3xl bg-sky-50 p-3 dark:bg-sky-900/20">
              <PelicanLogo :size="56" />
            </div>
            <p class="mt-4 text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('pelican.emptyResult') }}</p>
            <p class="mt-1 max-w-sm text-xs text-gray-500 dark:text-dark-400">{{ t('pelican.description') }}</p>
          </div>

          <div v-else-if="running" class="flex h-full flex-col">
            <p class="mb-2 flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
              <span class="h-3 w-3 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></span>
              {{ rawText ? t('pelican.running') : t('pelican.waiting') }}
              <span v-if="rawText">· {{ rawText.length.toLocaleString() }} chars</span>
            </p>
            <pre ref="streamRef" class="min-h-[24rem] flex-1 overflow-auto rounded-2xl bg-gray-950 p-4 text-xs leading-relaxed text-gray-100 lg:max-h-[65vh]">{{ rawText }}</pre>
          </div>

          <div v-else class="space-y-4">
            <div v-if="errorMessage" class="flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/20 dark:text-red-300">
              <Icon name="exclamationCircle" size="sm" class="mt-0.5 shrink-0" />{{ errorMessage }}
            </div>
            <PelicanArtifactPreview v-if="activeTab === 'play'" :html="artifact.html" :title="selectedModel || t('pelican.title')" />
            <pre v-else class="max-h-[70vh] overflow-auto rounded-2xl bg-gray-950 p-4 text-xs leading-relaxed text-gray-100">{{ resultSource }}</pre>
          </div>
        </div>
      </section>

      <!-- 右侧：本地历史 -->
      <aside class="card flex flex-col lg:col-span-2 2xl:sticky 2xl:top-4 2xl:col-span-1 2xl:max-h-[calc(100vh-64px-4rem)]">
        <div class="flex items-center justify-between gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div class="flex items-center gap-2">
            <Icon name="clock" size="sm" class="text-gray-400" />
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('pelican.historyTitle') }}</h2>
            <span v-if="history.length" class="rounded-full bg-gray-100 px-1.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-dark-300">{{ history.length }}</span>
          </div>
          <button type="button" class="text-xs text-gray-500 hover:text-red-600 disabled:opacity-40 dark:text-dark-400" :disabled="history.length === 0" @click="askClearHistory">
            {{ t('pelican.historyClear') }}
          </button>
        </div>

        <div class="flex-1 overflow-y-auto p-2">
          <div v-if="historyLoading" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</div>
          <div v-else-if="history.length === 0" class="px-4 py-10 text-center text-sm text-gray-400 dark:text-dark-500">{{ t('pelican.historyEmpty') }}</div>
          <ul v-else class="grid gap-1 sm:grid-cols-2 2xl:grid-cols-1">
            <li v-for="item in history" :key="item.id" class="group relative">
              <button type="button" class="w-full rounded-xl px-3 py-2.5 text-left transition-colors hover:bg-gray-50 dark:hover:bg-dark-700/60" @click="openRecord(item)">
                <div class="flex items-center gap-2 pr-7">
                  <span class="h-2 w-2 shrink-0 rounded-full" :class="historyDotClass(item.status)" :title="historyStatusLabel(item.status)"></span>
                  <span class="truncate font-mono text-sm text-gray-900 dark:text-white">{{ item.model }}</span>
                </div>
                <div class="mt-1 flex flex-wrap items-center gap-x-2 pl-4 text-xs text-gray-500 dark:text-dark-400">
                  <span>{{ formatDate(item.createdAt) }}</span>
                  <span>· {{ formatDuration(item.durationMs) }}</span>
                  <span v-if="item.usage?.outputTokens != null">· {{ formatNumber(item.usage.outputTokens) }} tok</span>
                  <span v-if="item.reasoningEffort">· {{ item.reasoningEffort }}</span>
                </div>
              </button>
              <button
                type="button"
                class="absolute right-2 top-2 rounded-lg p-1 text-gray-400 opacity-0 transition hover:bg-red-50 hover:text-red-600 focus:opacity-100 group-hover:opacity-100 dark:hover:bg-red-950/20"
                :aria-label="t('pelican.historyDelete')"
                @click="askDeleteRecord(item.id)"
              >
                <Icon name="trash" size="xs" />
              </button>
            </li>
          </ul>
        </div>
        <p class="border-t border-gray-100 px-5 py-3 text-xs text-gray-400 dark:border-dark-700 dark:text-dark-500">{{ t('pelican.historyHint') }}</p>
      </aside>
    </div>

    <PelicanHistoryDialog
      :show="historyDialogVisible"
      :record="selectedRecord"
      @close="historyDialogVisible = false"
      @delete="onDeleteFromDialog"
    />

    <BaseDialog
      :show="reportDialogOpen"
      :title="t('pelican.reportTitle')"
      width="normal"
      :close-on-escape="!reporting"
      :show-close-button="!reporting"
      @close="closeReportDialog"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('pelican.reportIntro') }}</p>
        <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 rounded-xl bg-gray-50 p-3 text-xs dark:bg-dark-800">
          <dt class="text-gray-500 dark:text-dark-400">{{ t('pelican.modelLabel') }}</dt>
          <dd class="truncate font-mono text-gray-900 dark:text-gray-100">{{ runModel }}</dd>
          <dt class="text-gray-500 dark:text-dark-400">{{ t('pelican.reasoningLabel') }}</dt>
          <dd class="text-gray-900 dark:text-gray-100">{{ runReasoning || t('pelican.reasoningDefault') }}</dd>
          <dt class="text-gray-500 dark:text-dark-400">{{ t('pelican.runId') }}</dt>
          <dd class="truncate font-mono text-gray-900 dark:text-gray-100">{{ currentRunId }}</dd>
        </dl>
        <label class="block">
          <span class="input-label">{{ t('pelican.reportNoteLabel') }}</span>
          <textarea
            v-model="reportNote"
            class="input"
            rows="3"
            :maxlength="reportNoteMax"
            :disabled="reporting"
            :placeholder="t('pelican.reportNotePlaceholder')"
          />
          <span class="input-hint">{{ t('pelican.reportNoteHint', { max: reportNoteMax }) }}</span>
        </label>
        <p v-if="reportError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ reportError }}</p>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="reporting" @click="closeReportDialog">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary" :disabled="reporting" @click="submitReport">
          {{ reporting ? t('pelican.reporting') : t('pelican.reportSubmit') }}
        </button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="confirmMode !== null"
      :title="confirmTitle"
      :message="confirmMessage"
      :danger="true"
      @confirm="confirmHistoryAction"
      @cancel="cancelHistoryAction"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PelicanArtifactPreview from '@/components/pelican/PelicanArtifactPreview.vue'
import PelicanHistoryDialog from '@/components/pelican/PelicanHistoryDialog.vue'
import PelicanLogo from '@/components/pelican/PelicanLogo.vue'
import keysAPI from '@/api/keys'
import {
  PelicanHttpError,
  PelicanStreamError,
  getConfig as getPelicanConfig,
  listModels as listPelicanModels,
  streamRun as streamPelicanRun,
  reportRun as reportPelicanRun,
  type PelicanConfig,
  type PelicanModel,
  type PelicanUsage
} from '@/api/pelican'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractArtifact } from '@/lib/pelican/artifact'
import {
  clearRuns,
  deleteRun,
  listRuns,
  loadUISettings,
  saveRun,
  saveUISettings,
  type PelicanRunRecord
} from '@/lib/pelican/storage'
import type { ApiKey } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

type RunState = 'idle' | 'running' | 'done' | 'cancelled' | 'error'
type ResultTab = 'play' | 'source'
type KeyMode = 'saved' | 'manual'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const keys = ref<ApiKey[]>([])
const keysLoading = ref(false)
const selectedKeyId = ref<number | null>(null)
const keyMode = ref<KeyMode>('saved')
const manualKey = ref('')
const showManualKey = ref(false)
const models = ref<PelicanModel[]>([])
const modelsLoading = ref(false)
const selectedModel = ref('')
const config = ref<PelicanConfig | null>(null)
const configLoading = ref(true)
const reasoningLevels = ref<string[]>([])
const reasoningEffort = ref('')
const baseUrl = ref('')

const runState = ref<RunState>('idle')
const rawText = ref('')
const usage = ref<PelicanUsage | null>(null)
const errorMessage = ref('')
const startedAt = ref(0)
const firstTokenAt = ref<number | null>(null)
const finishedAt = ref(0)
const nowTick = ref(0)
const currentRunId = ref('')
const activeTab = ref<ResultTab>('play')
const cancelRequested = ref(false)
const timedOut = ref(false)

const history = ref<PelicanRunRecord[]>([])
const historyLoading = ref(false)
const historyDialogVisible = ref(false)
const selectedRecord = ref<PelicanRunRecord | null>(null)
const confirmMode = ref<'clear' | 'delete' | null>(null)

// One-click report to administrators (only for successful runs on this site).
const reportNoteMax = 500
const gatewayRequestId = ref('')
const runModel = ref('')
const runReasoning = ref('')
const reportedRunIds = ref<Set<string>>(new Set())
const reportDialogOpen = ref(false)
const reportNote = ref('')
const reporting = ref(false)
const reportError = ref('')
const deleteTargetId = ref<string | null>(null)

const resultTabs: ResultTab[] = ['play', 'source']
const streamRef = ref<HTMLElement | null>(null)

let abortController: AbortController | null = null
let timeoutHandle: number | null = null
let ticker: number | null = null
let modelsRequest = 0
let initialized = false

const userId = computed(() => authStore.user?.id ?? 0)
const running = computed(() => runState.value === 'running')
const keyOptions = computed(() =>
  keys.value.map((key) => ({
    value: key.id,
    label: [key.name, key.group?.name, maskKey(key.key)].filter(Boolean).join(' · ')
  }))
)
const modelOptions = computed(() => models.value.map((model) => ({ value: model.id, label: model.id })))
const reasoningOptions = computed(() => [
  { value: '', label: t('pelican.reasoningDefault') },
  ...reasoningLevels.value.map((level) => ({ value: level, label: level }))
])
const baseUrlPlaceholder = computed(() => {
  const fallback = config.value?.fallback_base_url || window.location.origin
  return t('pelican.baseUrlPlaceholder', { url: fallback })
})
const baseUrlSourceLabel = computed(() => {
  const configured = (config.value?.default_base_url ?? '').trim()
  if (baseUrl.value.trim() && baseUrl.value.trim() !== configured) {
    return t('pelican.baseUrlSourceManual')
  }
  const source = config.value?.base_url_source
  if (source === 'api_base_url') return t('pelican.baseUrlSourceApi')
  if (source === 'frontend_url') return t('pelican.baseUrlSourceFrontend')
  if (source === 'request') return t('pelican.baseUrlSourceRequest')
  return ''
})
const hasCredential = computed(() =>
  keyMode.value === 'saved' ? !!selectedKeyId.value : !!manualKey.value.trim()
)
const canStart = computed(
  () => hasCredential.value && !!selectedModel.value && !running.value && !modelsLoading.value
)
const artifact = computed(() => extractArtifact(rawText.value))
const resultSource = computed(() => artifact.value.source || rawText.value)
const elapsedMs = computed(() => {
  if (!startedAt.value) return 0
  const end = finishedAt.value || (running.value ? nowTick.value : performance.now())
  return Math.max(0, end - startedAt.value)
})
const ttftMs = computed(() => {
  if (firstTokenAt.value == null || !startedAt.value) return null
  return Math.max(0, firstTokenAt.value - startedAt.value)
})
const tps = computed(() => {
  const output = usage.value?.completion_tokens
  if (output == null || elapsedMs.value <= 0) return null
  return output / (elapsedMs.value / 1000)
})
const metricItems = computed(() => [
  { label: t('pelican.elapsed'), value: formatDuration(elapsedMs.value) },
  { label: t('pelican.firstToken'), value: formatDuration(ttftMs.value) },
  { label: t('pelican.outputTokens'), value: formatNumber(usage.value?.completion_tokens) },
  { label: t('pelican.reasoningTokens'), value: formatNumber(usage.value?.completion_tokens_details?.reasoning_tokens) },
  { label: t('pelican.totalTokens'), value: formatNumber(usage.value?.total_tokens) },
  { label: t('pelican.tps'), value: formatTps(tps.value) }
])
const statusLabel = computed(() => {
  if (runState.value === 'running') return t('pelican.running')
  if (runState.value === 'done') return t('pelican.statusDone')
  if (runState.value === 'cancelled') return t('pelican.statusCancelled')
  return t('pelican.statusError')
})
const statusBadgeClass = computed(() => {
  if (runState.value === 'running') return 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
  if (runState.value === 'done') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (runState.value === 'cancelled') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
})
const confirmTitle = computed(() =>
  confirmMode.value === 'clear' ? t('pelican.historyClear') : t('pelican.historyDelete')
)
const confirmMessage = computed(() =>
  confirmMode.value === 'clear' ? t('pelican.historyClearConfirm') : t('pelican.confirmDelete')
)

onBeforeUnmount(() => {
  abortController?.abort()
  stopTicker()
  clearRunTimeout()
})

watch(
  userId,
  (id) => {
    if (id > 0 && !initialized) {
      initialized = true
      void init()
    }
  },
  { immediate: true }
)

watch([selectedKeyId, selectedModel, reasoningEffort, baseUrl], () => {
  persistUISettings()
})

watch(selectedKeyId, (id) => {
  if (keyMode.value !== 'saved') {
    return
  }
  if (id) {
    void loadModels()
  } else {
    models.value = []
    selectedModel.value = ''
  }
})

async function init(): Promise<void> {
  if (!userId.value) {
    return
  }
  const saved = loadUISettings(userId.value)
  if (saved) {
    keyMode.value = saved.keyMode
    reasoningEffort.value = saved.reasoningEffort
    baseUrl.value = saved.baseUrl
    selectedKeyId.value = saved.keyId
  }
  await Promise.all([loadConfig(), loadKeys(), loadHistory()])
}

async function loadConfig(): Promise<void> {
  configLoading.value = true
  try {
    const result = await getPelicanConfig()
    config.value = result
    reasoningLevels.value = result.reasoning_levels
    if (!baseUrl.value && result.default_base_url) {
      baseUrl.value = result.default_base_url
    }
  } catch (error) {
    appStore.showError(extractError(error))
  } finally {
    configLoading.value = false
  }
}

async function loadKeys(): Promise<void> {
  keysLoading.value = true
  try {
    const result = await keysAPI.list(1, 100, { status: 'active' })
    keys.value = result.items
    if (!selectedKeyId.value || !keys.value.some((key) => key.id === selectedKeyId.value)) {
      selectedKeyId.value = keys.value[0]?.id ?? null
    }
  } catch (error) {
    appStore.showError(extractError(error))
  } finally {
    keysLoading.value = false
  }
}

async function loadModels(): Promise<void> {
  const keyId = keyMode.value === 'saved' ? (selectedKeyId.value ?? undefined) : undefined
  const apiKey = keyMode.value === 'manual' ? manualKey.value.trim() : undefined
  if (!keyId && !apiKey) {
    models.value = []
    selectedModel.value = ''
    return
  }

  const seq = ++modelsRequest
  modelsLoading.value = true
  selectedModel.value = ''
  try {
    const result = await listPelicanModels({
      keyId,
      apiKey,
      baseUrl: baseUrl.value.trim() || undefined
    })
    if (seq !== modelsRequest) {
      return
    }
    models.value = result.models
    const saved = loadUISettings(userId.value)
    const savedModel = keyId ? saved?.modelByKey[String(keyId)] : ''
    selectedModel.value =
      (savedModel && result.models.some((model) => model.id === savedModel) ? savedModel : result.models[0]?.id) ?? ''
  } catch (error) {
    if (seq === modelsRequest) {
      models.value = []
      appStore.showError(extractError(error))
    }
  } finally {
    if (seq === modelsRequest) {
      modelsLoading.value = false
    }
  }
}

function reloadModels(): void {
  if (hasCredential.value) {
    void loadModels()
  }
}

function onBaseUrlBlur(): void {
  reloadModels()
}

function onManualKeyBlur(): void {
  if (keyMode.value === 'manual' && manualKey.value.trim()) {
    void loadModels()
  }
}

function setKeyMode(mode: KeyMode): void {
  if (running.value || keyMode.value === mode) {
    return
  }
  keyMode.value = mode
  models.value = []
  selectedModel.value = ''
  if (hasCredential.value) {
    void loadModels()
  }
  persistUISettings()
}

function onKeyChange(value: string | number | boolean | null | undefined): void {
  const parsed = typeof value === 'number' ? value : Number(value)
  selectedKeyId.value = Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

function onModelChange(value: string | number | boolean | null | undefined): void {
  selectedModel.value = typeof value === 'string' ? value : ''
}

function onReasoningChange(value: string | number | boolean | null | undefined): void {
  reasoningEffort.value = typeof value === 'string' ? value : ''
}

async function startRun(): Promise<void> {
  if (!hasCredential.value || !selectedModel.value || running.value) {
    return
  }
  const savedKey = keyMode.value === 'saved' ? keys.value.find((item) => item.id === selectedKeyId.value) : undefined
  const keyName = savedKey?.name ?? t('pelican.manualKeyLabel')
  currentRunId.value = newRunId()
  gatewayRequestId.value = ''
  runModel.value = selectedModel.value
  runReasoning.value = reasoningEffort.value
  rawText.value = ''
  usage.value = null
  errorMessage.value = ''
  startedAt.value = performance.now()
  firstTokenAt.value = null
  finishedAt.value = 0
  nowTick.value = startedAt.value
  cancelRequested.value = false
  timedOut.value = false
  activeTab.value = 'play'
  runState.value = 'running'
  startTicker()

  abortController = new AbortController()
  timeoutHandle = window.setTimeout(() => {
    timedOut.value = true
    abortController?.abort()
  }, (config.value?.limits.max_duration_seconds ?? 600) * 1000)

  try {
    await streamPelicanRun(
      {
        ...(keyMode.value === 'saved'
          ? { key_id: selectedKeyId.value ?? undefined }
          : { api_key: manualKey.value.trim() }),
        model: selectedModel.value,
        reasoning_effort: reasoningEffort.value || undefined,
        base_url: baseUrl.value.trim() || undefined,
        test_id: 'pelican-riding'
      },
      {
        onDelta: (text) => {
          if (firstTokenAt.value == null) {
            firstTokenAt.value = performance.now()
          }
          rawText.value += text
          void scrollStreamToBottom()
        },
        onUsage: (value) => {
          usage.value = value
        },
        onMeta: ({ requestId }) => {
          gatewayRequestId.value = requestId
        }
      },
      abortController.signal
    )
    if (!cancelRequested.value && !timedOut.value) {
      runState.value = 'done'
    }
  } catch (error) {
    if (timedOut.value) {
      runState.value = 'error'
      errorMessage.value = t('pelican.errorTimeout')
    } else if (cancelRequested.value || isAbortError(error)) {
      runState.value = 'cancelled'
      errorMessage.value = ''
    } else {
      runState.value = 'error'
      errorMessage.value = extractError(error)
    }
  } finally {
    clearRunTimeout()
    stopTicker()
    finishedAt.value = performance.now()
    abortController = null
    if (runState.value !== 'running') {
      await persistRun(keyName)
    }
    if (runState.value === 'cancelled') {
      appStore.showInfo(t('pelican.toastCancelled'))
    }
  }
}

function cancelRun(): void {
  if (!running.value) {
    return
  }
  cancelRequested.value = true
  abortController?.abort()
}

async function persistRun(keyName: string): Promise<void> {
  if (!rawText.value.trim() || !userId.value) {
    return
  }
  const html = artifact.value.html
  const durationMs = Math.max(0, finishedAt.value - startedAt.value)
  const outputTokens = usage.value?.completion_tokens ?? null
  const record: PelicanRunRecord = {
    id: currentRunId.value,
    createdAt: Date.now(),
    userId: userId.value,
    testId: 'pelican-riding',
    promptVersion: config.value?.test.prompt_version ?? '',
    keyId: keyMode.value === 'saved' ? (selectedKeyId.value ?? 0) : 0,
    keyName,
    model: selectedModel.value,
    reasoningEffort: reasoningEffort.value || null,
    status: runState.value === 'done' ? 'done' : runState.value === 'cancelled' ? 'cancelled' : 'error',
    error: errorMessage.value || null,
    durationMs,
    ttftMs: ttftMs.value,
    usage: usage.value
      ? {
          inputTokens: usage.value.prompt_tokens ?? null,
          outputTokens,
          reasoningTokens: usage.value.completion_tokens_details?.reasoning_tokens ?? null,
          totalTokens: usage.value.total_tokens ?? null
        }
      : null,
    tps: outputTokens != null && durationMs > 0 ? outputTokens / (durationMs / 1000) : null,
    html,
    rawText: rawText.value,
    byteSize: new Blob([html ?? rawText.value]).size,
    baseUrl: baseUrl.value.trim() || config.value?.fallback_base_url || ''
  }
  try {
    await saveRun(userId.value, record)
    await loadHistory()
    appStore.showSuccess(t('pelican.toastSaved'))
  } catch {
    appStore.showError(t('pelican.toastStorageFailed'))
  }
}

async function loadHistory(): Promise<void> {
  if (!userId.value) {
    return
  }
  historyLoading.value = true
  try {
    history.value = await listRuns(userId.value)
  } catch {
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

function openRecord(record: PelicanRunRecord): void {
  selectedRecord.value = record
  historyDialogVisible.value = true
}

function askDeleteRecord(id: string): void {
  deleteTargetId.value = id
  confirmMode.value = 'delete'
}

function askClearHistory(): void {
  confirmMode.value = 'clear'
}

async function confirmHistoryAction(): Promise<void> {
  const mode = confirmMode.value
  const id = deleteTargetId.value
  confirmMode.value = null
  deleteTargetId.value = null
  if (!userId.value) {
    return
  }
  try {
    if (mode === 'clear') {
      await clearRuns(userId.value)
      appStore.showSuccess(t('pelican.toastCleared'))
    } else if (mode === 'delete' && id) {
      await deleteRun(userId.value, id)
      if (selectedRecord.value?.id === id) {
        historyDialogVisible.value = false
        selectedRecord.value = null
      }
    }
    await loadHistory()
  } catch {
    appStore.showError(t('pelican.errorUnknown'))
  }
}

const isReported = computed(() => !!currentRunId.value && reportedRunIds.value.has(currentRunId.value))
const reportButtonTitle = computed(() => {
  if (isReported.value) return t('pelican.reported')
  if (!gatewayRequestId.value) return t('pelican.reportNotLocal')
  return t('pelican.reportHint')
})

function openReportDialog(): void {
  if (runState.value !== 'done' || !gatewayRequestId.value || isReported.value) {
    return
  }
  reportNote.value = ''
  reportError.value = ''
  reportDialogOpen.value = true
}

function closeReportDialog(): void {
  if (reporting.value) {
    return
  }
  reportDialogOpen.value = false
}

async function submitReport(): Promise<void> {
  const runId = currentRunId.value
  if (reporting.value || !runId || !gatewayRequestId.value || !resultSource.value) {
    return
  }
  reporting.value = true
  reportError.value = ''
  try {
    await reportPelicanRun({
      run_id: runId,
      request_id: gatewayRequestId.value,
      model: runModel.value,
      reasoning_effort: runReasoning.value || undefined,
      note: reportNote.value.trim() || undefined,
      source: resultSource.value,
      duration_ms: Math.round(elapsedMs.value),
      ttft_ms: ttftMs.value == null ? null : Math.round(ttftMs.value),
      prompt_version: config.value?.test.prompt_version
    })
    reportedRunIds.value = new Set([...reportedRunIds.value, runId])
    reportDialogOpen.value = false
    appStore.showSuccess(t('pelican.reportSent'))
  } catch (error) {
    reportError.value = extractApiErrorMessage(error, t('pelican.reportFailed'))
  } finally {
    reporting.value = false
  }
}

function cancelHistoryAction(): void {
  confirmMode.value = null
  deleteTargetId.value = null
}

function onDeleteFromDialog(id: string): void {
  historyDialogVisible.value = false
  askDeleteRecord(id)
}

async function copySource(): Promise<void> {
  if (!resultSource.value) {
    return
  }
  try {
    await navigator.clipboard.writeText(resultSource.value)
    appStore.showSuccess(t('pelican.toastCopied'))
  } catch {
    appStore.showError(t('pelican.toastCopyFailed'))
  }
}

function downloadCurrent(): void {
  if (!resultSource.value) {
    return
  }
  try {
    const blob = new Blob([resultSource.value], { type: 'text/html;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `pelican-${currentRunId.value || 'result'}.html`
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch {
    appStore.showError(t('pelican.toastDownloadFailed'))
  }
}

function persistUISettings(): void {
  if (!userId.value) {
    return
  }
  const previous = loadUISettings(userId.value)
  const modelByKey = { ...(previous?.modelByKey ?? {}) }
  if (keyMode.value === 'saved' && selectedKeyId.value && selectedModel.value) {
    modelByKey[String(selectedKeyId.value)] = selectedModel.value
  }
  saveUISettings(userId.value, {
    keyId: selectedKeyId.value,
    keyMode: keyMode.value,
    modelByKey,
    reasoningEffort: reasoningEffort.value,
    baseUrl: baseUrl.value
  })
}

function startTicker(): void {
  stopTicker()
  ticker = window.setInterval(() => {
    nowTick.value = performance.now()
  }, 250)
}

function stopTicker(): void {
  if (ticker != null) {
    window.clearInterval(ticker)
    ticker = null
  }
}

function clearRunTimeout(): void {
  if (timeoutHandle != null) {
    window.clearTimeout(timeoutHandle)
    timeoutHandle = null
  }
}

function extractError(error: unknown): string {
  if (error instanceof PelicanHttpError) {
    if (error.status === 401) return t('pelican.errorUnauthorized')
    if (error.status === 403) return t('pelican.errorForbidden')
    if (error.status === 429) return t('pelican.errorRateLimited')
    return error.message || t('pelican.errorUpstream')
  }
  if (error instanceof PelicanStreamError) {
    return error.message || t('pelican.errorUnknown')
  }
  if (error instanceof TypeError) {
    return t('pelican.errorNetwork')
  }
  return t('pelican.errorUnknown')
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

function maskKey(key: string): string {
  if (!key) return ''
  return `••••${key.slice(-4)}`
}

function newRunId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `run-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function formatDate(value: number): string {
  return new Date(value).toLocaleString()
}

function formatDuration(ms: number | null | undefined): string {
  if (ms == null || !Number.isFinite(ms)) return '—'
  if (ms < 1000) return `${Math.round(ms)} ms`
  return `${(ms / 1000).toFixed(1)} s`
}

function formatNumber(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return value.toLocaleString()
}

function formatTps(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return value.toFixed(1)
}

function historyStatusLabel(status: PelicanRunRecord['status']): string {
  if (status === 'done') return t('pelican.statusDone')
  if (status === 'cancelled') return t('pelican.statusCancelled')
  return t('pelican.statusError')
}

function historyDotClass(status: PelicanRunRecord['status']): string {
  if (status === 'done') return 'bg-emerald-500'
  if (status === 'cancelled') return 'bg-amber-500'
  return 'bg-red-500'
}

async function scrollStreamToBottom(): Promise<void> {
  const el = streamRef.value
  if (!el) return
  const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 48
  if (!nearBottom) return
  await nextTick()
  el.scrollTop = el.scrollHeight
}

</script>
