<template>
  <BaseDialog :show="show" :title="t('pelican.viewTitle')" width="wide" @close="emit('close')">
    <div v-if="record" class="space-y-4">
      <div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-gray-500 dark:text-dark-400">
        <span>{{ formatDate(record.createdAt) }}</span>
        <span class="font-mono">{{ record.model }}</span>
        <span v-if="record.reasoningEffort">{{ record.reasoningEffort }}</span>
        <span :class="statusClass(record.status)">{{ statusLabel(record.status) }}</span>
      </div>

      <div class="flex items-center gap-1 border-b border-gray-200 dark:border-dark-700">
        <button
          v-for="item in tabs"
          :key="item"
          type="button"
          class="border-b-2 px-3 py-2 text-sm font-medium transition-colors"
          :class="
            tab === item
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-gray-200'
          "
          @click="tab = item"
        >
          {{ t(`pelican.${item}`) }}
        </button>
      </div>

      <PelicanReasoningPanel :key="record.id" :text="record.reasoningText ?? ''" />

      <div v-if="tab === 'play'">
        <PelicanArtifactPreview :html="record.html" :title="record.model" />
      </div>

      <div v-else-if="tab === 'source'" class="space-y-2">
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="!source" @click="copySource">
            {{ t('pelican.copy') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="!source" @click="downloadHtml">
            {{ t('pelican.download') }}
          </button>
        </div>
        <pre class="max-h-[60vh] overflow-auto rounded-xl bg-gray-950 p-4 text-xs leading-relaxed text-gray-100">{{ source }}</pre>
      </div>

      <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div class="rounded-xl border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('pelican.elapsed') }}</div>
          <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatDuration(record.durationMs) }}</div>
        </div>
        <div class="rounded-xl border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('pelican.firstToken') }}</div>
          <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatDuration(record.ttftMs) }}</div>
        </div>
        <div class="rounded-xl border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('pelican.outputTokens') }}</div>
          <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatNumber(record.usage?.outputTokens) }}</div>
        </div>
        <div class="rounded-xl border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('pelican.tps') }}</div>
          <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatTps(record.tps) }}</div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-between gap-2">
        <button
          v-if="record"
          type="button"
          class="btn btn-secondary btn-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/20"
          @click="emit('delete', record.id)"
        >
          {{ t('pelican.historyDelete') }}
        </button>
        <button type="button" class="btn btn-secondary btn-sm ml-auto" @click="emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PelicanArtifactPreview from './PelicanArtifactPreview.vue'
import PelicanReasoningPanel from './PelicanReasoningPanel.vue'
import { useAppStore } from '@/stores/app'
import type { PelicanRunRecord } from '@/lib/pelican/storage'

const props = defineProps<{
  show: boolean
  record: PelicanRunRecord | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'delete', id: string): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const tabs = ['play', 'source', 'metrics'] as const
const tab = ref<(typeof tabs)[number]>('play')

const source = computed(() => props.record?.html ?? props.record?.rawText ?? '')

watch(
  () => props.show,
  (show) => {
    if (show) {
      tab.value = 'play'
    }
  }
)

function statusLabel(status: PelicanRunRecord['status']): string {
  if (status === 'done') return t('pelican.statusDone')
  if (status === 'cancelled') return t('pelican.statusCancelled')
  return t('pelican.statusError')
}

function statusClass(status: PelicanRunRecord['status']): string {
  if (status === 'done') return 'text-emerald-600 dark:text-emerald-400'
  if (status === 'cancelled') return 'text-amber-600 dark:text-amber-400'
  return 'text-red-600 dark:text-red-400'
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

async function copySource(): Promise<void> {
  if (!source.value) return
  try {
    await navigator.clipboard.writeText(source.value)
    appStore.showSuccess(t('pelican.toastCopied'))
  } catch {
    appStore.showError(t('pelican.toastCopyFailed'))
  }
}

function downloadHtml(): void {
  if (!source.value || !props.record) return
  try {
    const blob = new Blob([source.value], { type: 'text/html;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `pelican-${props.record.id}.html`
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch {
    appStore.showError(t('pelican.toastDownloadFailed'))
  }
}
</script>
