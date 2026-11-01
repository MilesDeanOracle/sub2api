<template>
  <el-dialog
    :model-value="controller.visible.value"
    :title="t('stepUp.title')"
    width="420px"
    align-center
    :show-close="false"
    :close-on-click-modal="false"
    :close-on-press-escape="!verifying"
    append-to-body
    class="ep-step-up-dialog"
    @update:model-value="(v: boolean) => !v && handleCancel()"
  >
    <div class="text-center">
      <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30">
        <el-icon class="h-6 w-6 text-primary-600 dark:text-primary-400"><Lock /></el-icon>
      </div>
      <p class="mt-3 text-sm text-gray-500 dark:text-gray-400">
        {{ t('stepUp.hint') }}
      </p>

      <div class="mt-5">
        <input
          ref="hiddenOtpInputRef"
          type="text"
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="6"
          class="pointer-events-none absolute left-0 top-0 h-px w-px opacity-0"
          aria-hidden="true"
          tabindex="-1"
          @input="handleHiddenOtpInput"
        />
        <div class="flex justify-center gap-2">
          <el-input
            v-for="(_, index) in 6"
            :key="index"
            :ref="(el: any) => setInputRef(el, index)"
            :model-value="code[index]"
            maxlength="1"
            class="ep-otp-cell"
            size="large"
            :disabled="verifying"
            @update:model-value="(val: string | number) => handleCellInput(String(val ?? ''), index)"
            @keydown="handleKeydown($event, index)"
            @paste="handlePaste"
          />
        </div>
        <div v-if="verifying" class="mt-3 flex items-center justify-center gap-2 text-sm text-gray-500">
          <el-icon class="is-loading"><Loading /></el-icon>
          {{ t('common.verifying') }}
        </div>
      </div>
    </div>

    <template #footer>
      <el-button :disabled="verifying" class="w-full" @click="handleCancel">
        {{ t('common.cancel') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Lock, Loading } from '@element-plus/icons-vue'
import { useAppStore } from '@/stores'
import { totpAPI } from '@/api'
import type { StepUpController } from '@/composables/useStepUp'

const props = defineProps<{
  controller: StepUpController
}>()

const { t } = useI18n()
const appStore = useAppStore()

const verifying = ref(false)
const code = ref<string[]>(['', '', '', '', '', ''])
const inputRefs = ref<(InstanceType<typeof import('element-plus')['ElInput']> | null)[]>([])
const hiddenOtpInputRef = ref<HTMLInputElement | null>(null)

const codeString = computed(() => code.value.join(''))

// Focus the first cell whenever the dialog opens.
watch(
  () => props.controller.visible.value,
  (open) => {
    if (open) {
      resetInputs()
      nextTick(() => inputRefs.value[0]?.focus())
    }
  }
)

// Auto-submit once 6 digits are entered.
watch(codeString, (newCode) => {
  if (newCode.length === 6 && !verifying.value) {
    submit(newCode)
  }
})

async function submit(otp: string) {
  verifying.value = true
  try {
    await totpAPI.stepUp(otp)
    verifying.value = false
    resetInputs()
    props.controller.onVerified()
  } catch (err: any) {
    verifying.value = false
    appStore.showError(err?.message || t('stepUp.verifyFailed'))
    resetInputs()
    nextTick(() => inputRefs.value[0]?.focus())
  }
}

function resetInputs() {
  code.value = ['', '', '', '', '', '']
  if (hiddenOtpInputRef.value) hiddenOtpInputRef.value.value = ''
}

function handleCancel() {
  if (verifying.value) return
  props.controller.onCancel()
}

const setInputRef = (el: any, index: number) => {
  inputRefs.value[index] = el as (InstanceType<typeof import('element-plus')['ElInput']> | null) || null
}

function focusCell(index: number) {
  nextTick(() => inputRefs.value[index]?.focus())
}

const handleCellInput = (raw: string, index: number) => {
  const value = raw.replace(/[^0-9]/g, '').slice(-1)
  code.value[index] = value
  // 同步 el-input 展示值（受控值可能残留两位数字）
  const inst = inputRefs.value[index] as any
  if (inst && value !== raw) {
    inst.refInput?.forEach?.((i: HTMLInputElement) => { i.value = value })
  }
  if (value && index < 5) {
    focusCell(index + 1)
  }
}

const handleHiddenOtpInput = (event: Event) => {
  const input = event.target as HTMLInputElement
  const digits = input.value.replace(/[^0-9]/g, '').slice(0, 6).split('')
  for (let i = 0; i < 6; i++) {
    code.value[i] = digits[i] || ''
  }
}

const handleKeydown = (event: KeyboardEvent, index: number) => {
  if (event.key === 'Backspace') {
    if (!code.value[index] && index > 0) {
      event.preventDefault()
      focusCell(index - 1)
    }
  }
}

const handlePaste = (event: ClipboardEvent) => {
  event.preventDefault()
  const pastedData = event.clipboardData?.getData('text') || ''
  const digits = pastedData.replace(/[^0-9]/g, '').slice(0, 6).split('')
  for (let i = 0; i < 6; i++) {
    code.value[i] = digits[i] || ''
  }
  const focusIndex = Math.min(digits.length, 5)
  focusCell(focusIndex)
}
</script>

<style scoped>
.ep-otp-cell {
  width: 44px;
}

.ep-otp-cell :deep(.el-input__inner) {
  text-align: center;
  font-size: 18px;
  font-weight: 600;
  padding-left: 0;
  padding-right: 0;
}
</style>
