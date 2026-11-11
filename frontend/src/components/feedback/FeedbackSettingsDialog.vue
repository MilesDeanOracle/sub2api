<template>
  <BaseDialog :show="show" :title="t('feedback.settingsTitle')" width="wide" :close-on-escape="!saving" :show-close-button="!saving" @close="$emit('close')">
    <p v-if="loading" role="status">{{ t('common.loading') }}</p>
    <form v-else class="space-y-4" @submit.prevent="save">
      <label class="block">
        <span class="mb-2 block text-sm font-medium">{{ t('feedback.noticeLabel') }}</span>
        <textarea v-model="notice" class="input" rows="5" maxlength="2000" :disabled="saving || !loaded" />
      </label>
      <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('feedback.noticeHint') }}</p>
      <button type="button" class="btn btn-ghost btn-sm" :disabled="saving || !loaded" @click="notice = t('feedback.contextNotice')">{{ t('feedback.restoreNotice') }}</button>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
      <div class="flex justify-end gap-2">
        <button v-if="!loaded" type="button" class="btn btn-secondary" @click="load">{{ t('common.refresh') }}</button>
        <button type="submit" class="btn btn-primary" :disabled="saving || !loaded">{{ saving ? t('common.loading') : t('common.save') }}</button>
      </div>
    </form>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getFeedbackSettings, updateFeedbackSettings } from '@/api/feedback'
import { useFeedbackStore } from '@/stores/feedback'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const store = useFeedbackStore()
const app = useAppStore()
const notice = ref('')
const loading = ref(false)
const loaded = ref(false)
const saving = ref(false)
const error = ref('')
let version = 0

async function load() {
  const current = ++version
  loaded.value = false
  loading.value = true
  error.value = ''
  try {
    const settings = await getFeedbackSettings(true)
    if (current !== version) return
    notice.value = settings.context_notice ?? t('feedback.contextNotice')
    loaded.value = true
  } catch { if (current === version) error.value = t('feedback.settingsLoadFailed') }
  finally { if (current === version) loading.value = false }
}
async function save() {
  if (saving.value || !loaded.value) return
  saving.value = true
  error.value = ''
  try {
    const value = notice.value.trim()
    await updateFeedbackSettings({ context_notice: value })
    store.contextNotice = value
    app.showSuccess(t('feedback.settingsSaved'))
    emit('close')
  } catch { error.value = t('feedback.settingsSaveFailed') }
  finally { saving.value = false }
}
watch(() => props.show, show => { if (show) void load(); else ++version }, { immediate: true })
</script>
