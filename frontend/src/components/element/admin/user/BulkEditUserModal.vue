<template>
  <el-dialog
    :model-value="show"
    :title="t('admin.users.bulkLimits.title')"
    width="520px"
    align-center
    @update:model-value="(v: boolean) => !v && emit('close')"
    @close="emit('close')"
  >
    <div class="space-y-5">
      <p class="text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('admin.users.bulkLimits.selectedCount', { count: selectedIds.length }) }}
      </p>

      <div class="divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-700 dark:border-dark-700">
        <div class="space-y-3 py-4">
          <div class="flex items-center justify-between gap-4">
            <label class="ep-field-label !mb-0">{{ t('admin.users.columns.concurrency') }}</label>
            <el-switch
              v-model="enableConcurrency"
              :aria-label="t('admin.users.bulkLimits.enableConcurrency')"
              data-test="enable-concurrency"
            />
          </div>
          <el-input
            v-if="enableConcurrency"
            v-model="concurrencyValue"
            type="number"
            :min="0"
            step="1"
            data-test="concurrency-input"
          />
        </div>

        <div class="space-y-3 py-4">
          <div class="flex items-center justify-between gap-4">
            <label class="ep-field-label !mb-0">{{ t('admin.users.form.rpmLimit') }}</label>
            <el-switch
              v-model="enableRPMLimit"
              :aria-label="t('admin.users.bulkLimits.enableRPMLimit')"
              data-test="enable-rpm-limit"
            />
          </div>
          <div v-if="enableRPMLimit">
            <el-input
              v-model="rpmLimitValue"
              type="number"
              :min="0"
              step="1"
              data-test="rpm-limit-input"
            />
            <p v-if="parsedRPMLimit === 0" class="ep-form-hint">
              {{ t('admin.users.bulkLimits.unlimited') }}
            </p>
          </div>
        </div>
      </div>

      <p v-if="hasInvalidValue" class="text-sm text-red-600 dark:text-red-400">
        {{ t('admin.users.bulkLimits.nonNegativeInteger') }}
      </p>
      <p v-if="selectionTooLarge" class="text-sm text-red-600 dark:text-red-400">
        {{ t('admin.users.bulkLimits.selectionLimit', { max: MAX_BATCH_USER_IDS }) }}
      </p>
    </div>

    <template #footer>
      <el-button @click="emit('close')">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :disabled="!canSubmit" data-test="submit" @click="handleSubmit">
        {{ submitting ? t('admin.users.bulkLimits.applying') : t('admin.users.bulkLimits.apply') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { BatchUpdateUserLimitsRequest } from '@/api/admin/users'
import { useAppStore } from '@/stores/app'

const props = defineProps<{
  show: boolean
  selectedIds: number[]
}>()

const emit = defineEmits<{
  close: []
  success: [affected: number]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const enableConcurrency = ref(false)
const enableRPMLimit = ref(false)
const concurrencyValue = ref<string | number>('')
const rpmLimitValue = ref<string | number>('')
const submitting = ref(false)
const MAX_BATCH_USER_IDS = 500

const parseLimit = (value: string | number): number | null | undefined => {
  const trimmed = String(value).trim()
  if (!trimmed) return undefined
  const parsed = Number(trimmed)
  if (!Number.isInteger(parsed) || parsed < 0) return null
  return parsed
}

const parsedConcurrency = computed(() =>
  enableConcurrency.value ? parseLimit(concurrencyValue.value) : undefined
)
const parsedRPMLimit = computed(() =>
  enableRPMLimit.value ? parseLimit(rpmLimitValue.value) : undefined
)
const hasInvalidValue = computed(() =>
  parsedConcurrency.value === null || parsedRPMLimit.value === null
)
const hasUpdate = computed(() =>
  (parsedConcurrency.value !== undefined && parsedConcurrency.value !== null)
  || (parsedRPMLimit.value !== undefined && parsedRPMLimit.value !== null)
)
const selectionTooLarge = computed(() => props.selectedIds.length > MAX_BATCH_USER_IDS)
const canSubmit = computed(() =>
  props.selectedIds.length > 0
  && !selectionTooLarge.value
  && hasUpdate.value
  && !hasInvalidValue.value
  && !submitting.value
)

const reset = () => {
  enableConcurrency.value = false
  enableRPMLimit.value = false
  concurrencyValue.value = ''
  rpmLimitValue.value = ''
  submitting.value = false
}

watch(
  () => props.show,
  (show) => {
    if (show) reset()
  }
)

const handleSubmit = async () => {
  if (!canSubmit.value) return

  const request: BatchUpdateUserLimitsRequest = {
    user_ids: [...props.selectedIds],
    all: false
  }
  const fields: string[] = []
  if (parsedConcurrency.value !== undefined && parsedConcurrency.value !== null) {
    request.concurrency = parsedConcurrency.value
    fields.push(
      t('admin.users.bulkLimits.concurrencyValue', { value: parsedConcurrency.value })
    )
  }
  if (parsedRPMLimit.value !== undefined && parsedRPMLimit.value !== null) {
    request.rpm_limit = parsedRPMLimit.value
    fields.push(
      parsedRPMLimit.value === 0
        ? t('admin.users.bulkLimits.rpmUnlimitedValue')
        : t('admin.users.bulkLimits.rpmValue', { value: parsedRPMLimit.value })
    )
  }

  try {
    await ElMessageBox.confirm(
      t('admin.users.bulkLimits.confirm', {
        count: props.selectedIds.length,
        fields: fields.join(', ')
      }),
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

  submitting.value = true
  try {
    const result = await adminAPI.users.batchUpdateLimits(request)
    appStore.showSuccess(
      t('admin.users.bulkLimits.success', { count: result.affected })
    )
    emit('success', result.affected)
    emit('close')
  } catch (error: any) {
    appStore.showError(
      extractApiErrorMessage(error, t('admin.users.bulkLimits.failed'))
    )
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.ep-field-label {
  @apply block text-sm font-medium text-gray-700 dark:text-gray-300;
}

.ep-form-hint {
  @apply mt-1 text-xs text-gray-400 dark:text-gray-500;
}
</style>
