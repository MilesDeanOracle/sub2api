<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { rewardMomentsAPI, type RewardMomentsConfig } from '@/api/activity'
import { extractApiErrorMessage } from '@/utils/apiError'
const { t } = useI18n()
const id = useId()
const config = ref<RewardMomentsConfig | null>(null)
const ids = ref('')
const saving = ref(false)
const message = ref('')
const failed = ref(false)
const invalid = ref(false)
const help = computed(() => t('operationsConsole.rewardMoments.help'))
async function load() {
  try { config.value = await rewardMomentsAPI.config(); ids.value = config.value.user_ids.join(', '); failed.value = false; message.value = '' }
  catch { failed.value = true; message.value = t('operationsConsole.rewardMoments.loadFailed') }
}
async function save() {
  if (!config.value || saving.value) return
  const tokens = ids.value.trim() ? ids.value.trim().split(/[,，\s]+/) : []
  invalid.value = tokens.length > 500 || tokens.some(value => !/^\d+$/.test(value) || !Number.isSafeInteger(Number(value)) || Number(value) <= 0)
  if (invalid.value) { failed.value = true; message.value = t('operationsConsole.rewardMoments.invalidIDs'); return }
  saving.value = true; message.value = ''
  try {
    config.value = await rewardMomentsAPI.saveConfig({ enabled: config.value.enabled, user_ids: [...new Set(tokens.map(Number))] })
    ids.value = config.value.user_ids.join(', ')
    failed.value = false
    message.value = t('operationsConsole.rewardMoments.saved')
  } catch (e) { failed.value = true; message.value = extractApiErrorMessage(e, t('operationsConsole.rewardMoments.saveFailed')) }
  finally { saving.value = false }
}
onMounted(load)
</script>
<template>
  <section class="moments-config card mb-6 p-5" :aria-labelledby="`${id}-title`">
    <h2 :id="`${id}-title`" class="text-base font-semibold">{{ t('operationsConsole.rewardMoments.title') }}</h2>
    <p :id="`${id}-help`" class="mt-2 text-xs leading-6 text-gray-600 dark:text-dark-300">{{ help }}</p>
    <form v-if="config" class="mt-3 space-y-3" @submit.prevent="save">
      <fieldset :disabled="saving" class="min-w-0 space-y-3">
        <legend class="sr-only">{{ t('operationsConsole.rewardMoments.legend') }}</legend>
        <label :for="`${id}-enabled`" class="flex min-h-11 items-center gap-2 text-sm"><input :id="`${id}-enabled`" v-model="config.enabled" type="checkbox" role="switch" :aria-describedby="`${id}-help`" data-testid="reward-moments-enabled">{{ t('operationsConsole.rewardMoments.enabled') }}</label>
        <div><label :for="`${id}-ids`" class="mb-2 block text-sm">{{ t('operationsConsole.rewardMoments.viewerIDs') }}</label><textarea :id="`${id}-ids`" v-model="ids" name="reward_moments_viewer_ids" rows="2" maxlength="10000" class="input w-full" :aria-invalid="invalid || undefined" :aria-describedby="`${id}-help`" spellcheck="false" @input="invalid = false" /></div>
        <button type="submit" class="btn btn-primary" :disabled="saving" :aria-busy="saving">{{ saving ? t('operationsConsole.rewardMoments.saving') : t('operationsConsole.rewardMoments.save') }}</button>
      </fieldset>
    </form>
    <p v-if="message" class="mt-3 text-sm" :class="failed ? 'text-red-600 dark:text-red-400' : 'text-emerald-700 dark:text-emerald-300'" :role="failed ? 'alert' : 'status'">{{ message }} <button v-if="!config" type="button" class="underline" @click="load">{{ t('operationsConsole.rewardMoments.retry') }}</button></p>
  </section>
</template>
<style scoped>
.moments-config :is(button,input,textarea):focus-visible{outline:2px solid #138467;outline-offset:3px}.moments-config input{width:16px;height:16px;accent-color:#138467}.moments-config textarea{font-size:16px}
@media(pointer:coarse){.moments-config button{min-height:48px}}
</style>
