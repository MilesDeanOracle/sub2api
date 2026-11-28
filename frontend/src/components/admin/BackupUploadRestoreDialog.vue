<template>
  <BaseDialog
    :show="show"
    :title="t('admin.backup.upload.title')"
    width="normal"
    :close-on-escape="!busy"
    @close="handleClose"
  >
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.backup.upload.description') }}
      </p>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
      >
        {{ t('admin.backup.upload.warning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.backup.upload.file') }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed px-4 py-3 transition-colors"
          :class="dragActive
            ? 'border-primary-400 bg-primary-50/70 dark:border-primary-500 dark:bg-primary-900/20'
            : 'border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800'"
          @dragenter.prevent="handleDragEnter"
          @dragover.prevent
          @dragleave.prevent="handleDragLeave"
          @drop.prevent="handleDrop"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200" :title="selectedFile?.name">
              {{ selectedFile ? selectedFile.name : t('admin.backup.upload.dropHint') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">
              <template v-if="selectedFile">{{ formatFileSize(selectedFile.size) }} · </template>
              {{ t('admin.backup.upload.fileHint') }}
            </div>
          </div>
          <button type="button" class="btn btn-secondary shrink-0" :disabled="busy" @click="openFilePicker">
            {{ t('common.chooseFile') }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          accept=".gz,.sql.gz,application/gzip"
          @change="handleFileChange"
        />
      </div>

      <div>
        <label class="input-label">{{ t('admin.backup.upload.password') }}</label>
        <input
          v-model="password"
          type="password"
          class="input w-full"
          autocomplete="current-password"
          :placeholder="t('admin.backup.upload.passwordPlaceholder')"
          :disabled="busy"
        />
      </div>

      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-dark-200">
        <input v-model="backupBeforeRestore" type="checkbox" class="mt-0.5" :disabled="busy" />
        <span>
          {{ t('admin.backup.upload.backupBeforeRestore') }}
          <span class="block text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.backup.upload.backupBeforeRestoreHint') }}
          </span>
        </span>
      </label>

      <div v-if="phase === 'uploading'" class="space-y-1" data-testid="backup-upload-progress">
        <div class="flex items-center justify-between text-xs text-gray-600 dark:text-dark-300">
          <span>{{ t('admin.backup.upload.uploading', { percent: progress }) }}</span>
          <span>{{ progress }}%</span>
        </div>
        <div class="h-2 overflow-hidden rounded bg-gray-200 dark:bg-dark-700">
          <div class="h-full rounded bg-primary-500 transition-all" :style="{ width: `${progress}%` }"></div>
        </div>
      </div>
      <p v-else-if="phase === 'processing'" class="text-sm text-gray-600 dark:text-dark-300" aria-live="polite">
        {{ t('admin.backup.upload.processing') }}
      </p>
      <p v-else-if="phase === 'restoring'" class="text-sm text-gray-600 dark:text-dark-300" aria-live="polite">
        {{ t('admin.backup.upload.restoring') }}
      </p>

      <p v-if="errorMessage" class="text-sm text-red-600 dark:text-red-400" role="alert">
        {{ errorMessage }}
      </p>
      <p v-if="busy" class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('admin.backup.upload.busyHint') }}
      </p>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="busy" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="button"
          :disabled="busy || !selectedFile || !password"
          @click="submit"
        >
          {{ submitLabel }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <TotpStepUpDialog :controller="backupStepUp" />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { adminAPI } from '@/api'
import type { BackupRecord } from '@/api/admin/backup'
import { useAppStore } from '@/stores'
import { isStepUpBlocked, isStepUpCancelled, stepUpBlockReason, useStepUp } from '@/composables/useStepUp'

const MAX_UPLOAD_BYTES = 5 * 1024 * 1024 * 1024
const MAX_POLL_COUNT = 900
const POLL_INTERVAL_MS = 2000

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'uploaded', record: BackupRecord): void
  (e: 'restore-started', record: BackupRecord): void
}

interface ApiLikeError {
  status?: number
  message?: string
  response?: { status?: number }
}

type Phase = 'idle' | 'uploading' | 'processing' | 'ready' | 'restoring'

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()
const backupStepUp = useStepUp()

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const password = ref('')
const backupBeforeRestore = ref(true)
const dragDepth = ref(0)
const phase = ref<Phase>('idle')
const progress = ref(0)
const errorMessage = ref('')
const uploadedRecord = ref<BackupRecord | null>(null)

let pollTimer: ReturnType<typeof setInterval> | null = null

const dragActive = computed(() => dragDepth.value > 0)
const busy = computed(() => phase.value === 'uploading' || phase.value === 'processing' || phase.value === 'restoring')
const submitLabel = computed(() => {
  switch (phase.value) {
    case 'uploading':
      return t('admin.backup.upload.uploading', { percent: progress.value })
    case 'processing':
      return t('admin.backup.upload.processing')
    case 'restoring':
      return t('admin.backup.upload.restoring')
    case 'ready':
      return t('admin.backup.upload.retryRestore')
    default:
      return t('admin.backup.upload.submit')
  }
})

watch(
  () => props.show,
  (visible) => {
    if (visible) resetForm()
  },
)

onBeforeUnmount(stopPolling)

function resetForm() {
  stopPolling()
  selectedFile.value = null
  password.value = ''
  backupBeforeRestore.value = true
  dragDepth.value = 0
  phase.value = 'idle'
  progress.value = 0
  errorMessage.value = ''
  uploadedRecord.value = null
  if (fileInput.value) fileInput.value.value = ''
}

function handleClose() {
  if (busy.value) return
  stopPolling()
  emit('close')
}

function openFilePicker() {
  fileInput.value?.click()
}

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (file) selectFile(file)
}

function handleDragEnter() {
  if (!busy.value) dragDepth.value++
}

function handleDragLeave() {
  dragDepth.value = Math.max(0, dragDepth.value - 1)
}

function handleDrop(event: DragEvent) {
  dragDepth.value = 0
  if (busy.value) return
  const file = event.dataTransfer?.files?.[0]
  if (file) selectFile(file)
}

function selectFile(file: File): boolean {
  errorMessage.value = ''
  if (!/\.(sql\.)?gz$/i.test(file.name)) {
    errorMessage.value = t('admin.backup.upload.invalidFile')
    return false
  }
  if (file.size <= 0) {
    errorMessage.value = t('admin.backup.upload.emptyFile')
    return false
  }
  if (file.size > MAX_UPLOAD_BYTES) {
    errorMessage.value = t('admin.backup.upload.fileTooLarge')
    return false
  }
  selectedFile.value = file
  uploadedRecord.value = null
  phase.value = 'idle'
  progress.value = 0
  return true
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

async function submit() {
  if (busy.value) return
  errorMessage.value = ''

  if (!selectedFile.value) {
    errorMessage.value = t('admin.backup.upload.fileRequired')
    return
  }
  if (!password.value) {
    errorMessage.value = t('admin.backup.upload.passwordRequired')
    return
  }

  if (uploadedRecord.value) {
    await startRestore()
    return
  }

  phase.value = 'uploading'
  progress.value = 0
  try {
    const record = await backupStepUp.run(() =>
      adminAPI.backup.uploadBackup(selectedFile.value as File, (percent) => {
        progress.value = percent
      }),
    )
    uploadedRecord.value = record
    emit('uploaded', record)

    phase.value = 'processing'
    const completed = await waitForUploadedRecord(record.id)
    uploadedRecord.value = completed
    emit('uploaded', completed)

    phase.value = 'ready'
    await startRestore()
  } catch (error) {
    stopPolling()
    if (isStepUpCancelled(error)) {
      phase.value = uploadedRecord.value ? 'ready' : 'idle'
      return
    }
    if (reportStepUpBlocked(error)) {
      phase.value = uploadedRecord.value ? 'ready' : 'idle'
      return
    }
    errorMessage.value = errorText(error)
    phase.value = uploadedRecord.value ? 'ready' : 'idle'
  }
}

async function startRestore() {
  if (!uploadedRecord.value) return
  phase.value = 'restoring'
  errorMessage.value = ''
  try {
    const record = await backupStepUp.run(() =>
      adminAPI.backup.restoreBackup(uploadedRecord.value!.id, password.value, {
        backupBeforeRestore: backupBeforeRestore.value,
      }),
    )
    emit('restore-started', record)
    emit('close')
    resetForm()
  } catch (error) {
    if (isStepUpCancelled(error)) {
      phase.value = 'ready'
      return
    }
    if (reportStepUpBlocked(error)) {
      phase.value = 'ready'
      return
    }
    errorMessage.value = errorText(error)
    phase.value = 'ready'
  }
}

function waitForUploadedRecord(id: string): Promise<BackupRecord> {
  return new Promise((resolve, reject) => {
    let count = 0
    pollTimer = setInterval(async () => {
      if (count++ >= MAX_POLL_COUNT) {
        stopPolling()
        reject(new Error(t('admin.backup.upload.processingTimeout')))
        return
      }
      try {
        const record = await adminAPI.backup.getBackup(id)
        if (record.status === 'completed') {
          stopPolling()
          resolve(record)
        } else if (record.status === 'failed') {
          stopPolling()
          reject(new Error(record.error_message || t('admin.backup.upload.uploadFailed')))
        }
      } catch {
        // 网络抖动时继续轮询，直到达到上限。
      }
    }, POLL_INTERVAL_MS)
  })
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function reportStepUpBlocked(error: unknown): boolean {
  if (!isStepUpBlocked(error)) return false
  appStore.showError(
    stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
      ? t('stepUp.adminApiKeyForbidden')
      : t('stepUp.notEnabled'),
  )
  return true
}

function errorText(error: unknown): string {
  const err = error as ApiLikeError | null
  const status = err?.status ?? err?.response?.status
  if (status === 413) return t('admin.backup.upload.serverLimitHint')
  if (status === 409) {
    return phase.value === 'restoring'
      ? t('admin.backup.operations.restoreRunning')
      : t('admin.backup.upload.uploadInProgress')
  }
  return err?.message || t('errors.networkError')
}
</script>
