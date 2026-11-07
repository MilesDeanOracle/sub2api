<template>
  <div class="card" data-testid="recharge-quick-button-editor">
    <!-- Header：与充值优惠阶梯卡片同款 -->
    <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
      <div class="flex items-center justify-between gap-3">
        <div class="min-w-0">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('admin.settings.payment.quickButtons.title') }}
          </h2>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.payment.quickButtons.hint') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm shrink-0"
          :disabled="saveLoading"
          data-testid="recharge-quick-button-save"
          @click="save"
        >
          {{ saveLoading ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </div>

    <div class="space-y-4 p-4">
      <div
        v-if="loading"
        class="rounded-xl border border-dashed border-gray-300 px-4 py-3 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
      >
        {{ t('common.loading') }}
      </div>
      <div
        v-else-if="buttons.length === 0"
        class="rounded-xl border border-dashed border-gray-300 px-4 py-3 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
      >
        {{ t('admin.settings.payment.quickButtons.empty') }}
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="(btn, index) in buttons"
          :key="btn.key"
          class="rounded-xl border border-gray-200 p-3 dark:border-dark-600"
          data-testid="recharge-quick-button-row"
        >
          <div class="flex flex-wrap items-start gap-3">
            <!-- 名称 / 地址 / 打开方式 / 图标 -->
            <div class="grid min-w-0 flex-1 grid-cols-1 gap-3 sm:grid-cols-2">
              <div>
                <label class="input-label">{{ t('admin.settings.payment.quickButtons.label') }}</label>
                <input
                  v-model="btn.label"
                  type="text"
                  maxlength="50"
                  class="input"
                  :placeholder="t('admin.settings.payment.quickButtons.labelPlaceholder')"
                  data-testid="recharge-quick-button-label"
                />
              </div>
              <div>
                <label class="input-label">{{ t('admin.settings.payment.quickButtons.openMethod') }}</label>
                <Select
                  v-model="btn.open_method"
                  :options="openMethodOptions"
                  data-testid="recharge-quick-button-open-method"
                />
              </div>
              <div class="sm:col-span-2">
                <label class="input-label">{{ t('admin.settings.payment.quickButtons.url') }}</label>
                <input
                  v-model="btn.url"
                  type="text"
                  class="input"
                  placeholder="https://example.com/recharge"
                  data-testid="recharge-quick-button-url"
                />
              </div>
              <div class="sm:col-span-2">
                <label class="input-label">{{ t('admin.settings.payment.quickButtons.icon') }}</label>
                <ImageUpload
                  v-model="btn.icon_url"
                  size="sm"
                  :upload-label="t('admin.settings.site.uploadImage')"
                  :remove-label="t('admin.settings.site.remove')"
                  :hint="t('admin.settings.payment.quickButtons.iconHint')"
                />
              </div>
            </div>
            <!-- 启用 / 排序 / 删除 -->
            <div class="flex shrink-0 flex-col items-end gap-2">
              <div class="flex items-center gap-2">
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.payment.quickButtons.enabled') }}
                </span>
                <Toggle v-model="btn.enabled" />
              </div>
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                  :disabled="index === 0"
                  :title="t('admin.settings.payment.quickButtons.moveUp')"
                  :data-testid="`recharge-quick-button-up-${index}`"
                  @click="move(index, -1)"
                >
                  <Icon name="chevronUp" size="sm" />
                </button>
                <button
                  type="button"
                  class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                  :disabled="index === buttons.length - 1"
                  :title="t('admin.settings.payment.quickButtons.moveDown')"
                  :data-testid="`recharge-quick-button-down-${index}`"
                  @click="move(index, 1)"
                >
                  <Icon name="chevronDown" size="sm" />
                </button>
                <button
                  type="button"
                  class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-dark-700 dark:hover:text-red-400"
                  :title="t('admin.settings.payment.quickButtons.remove')"
                  :data-testid="`recharge-quick-button-remove-${index}`"
                  @click="remove(index)"
                >
                  <Icon name="x" size="sm" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="buttons.length >= MAX_QUICK_BUTTONS"
        data-testid="recharge-quick-button-add"
        @click="add"
      >
        <Icon name="plus" size="sm" class="mr-1" />
        {{ t('admin.settings.payment.quickButtons.add') }}
      </button>
      <p v-if="buttons.length >= MAX_QUICK_BUTTONS" class="text-xs text-gray-400 dark:text-gray-500">
        {{ t('admin.settings.payment.quickButtons.maxHint', { max: MAX_QUICK_BUTTONS }) }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'
import ImageUpload from '@/components/common/ImageUpload.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'
import type { AdminRechargeQuickButton, RechargeQuickButtonOpenMethod } from '@/types/payment'

/** 后端对快捷按钮数量的上限，与服务层校验保持一致 */
const MAX_QUICK_BUTTONS = 10

interface QuickButtonDraft extends AdminRechargeQuickButton {
  /** 前端行 key，与后端无关 */
  key: string
}

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saveLoading = ref(false)
const buttons = reactive<QuickButtonDraft[]>([])

const openMethodOptions = computed(() => [
  { value: 'new_tab', label: t('admin.settings.payment.quickButtons.openMethodNewTab') },
  { value: 'popup', label: t('admin.settings.payment.quickButtons.openMethodPopup') },
])

let keySeq = 0
function newKey(): string {
  keySeq += 1
  return `quick-btn-${keySeq}`
}

function toDraft(btn: AdminRechargeQuickButton): QuickButtonDraft {
  return {
    key: newKey(),
    id: btn.id,
    label: btn.label ?? '',
    icon_url: btn.icon_url ?? '',
    url: btn.url ?? '',
    open_method: btn.open_method ?? 'new_tab',
    enabled: btn.enabled ?? true,
    sort_order: btn.sort_order,
  }
}

async function load() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getQuickButtons()
    const list = res.data ?? []
    buttons.splice(0, buttons.length, ...list.map(toDraft))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.settings.payment.quickButtons', t('common.error')))
  } finally {
    loading.value = false
  }
}

function add() {
  if (buttons.length >= MAX_QUICK_BUTTONS) return
  buttons.push(toDraft({
    label: '',
    icon_url: '',
    url: '',
    open_method: 'new_tab',
    enabled: true,
  }))
}

function remove(index: number) {
  buttons.splice(index, 1)
}

function move(index: number, delta: -1 | 1) {
  const target = index + delta
  if (target < 0 || target >= buttons.length) return
  const [item] = buttons.splice(index, 1)
  buttons.splice(target, 0, item!)
}

function validate(): boolean {
  for (const [index, btn] of buttons.entries()) {
    if (!/^https?:\/\/.+/i.test(btn.url.trim())) {
      appStore.showError(t('admin.settings.payment.quickButtons.invalidUrl', { index: index + 1 }))
      return false
    }
  }
  return true
}

async function save() {
  if (!validate() || saveLoading.value) return
  saveLoading.value = true
  try {
    const payload: AdminRechargeQuickButton[] = buttons.map((btn) => ({
      label: btn.label.trim(),
      icon_url: btn.icon_url.trim(),
      url: btn.url.trim(),
      open_method: btn.open_method as RechargeQuickButtonOpenMethod,
      enabled: btn.enabled,
      sort_order: 0,
    }))
    const res = await adminPaymentAPI.updateQuickButtons(payload)
    const list = res.data ?? []
    buttons.splice(0, buttons.length, ...list.map(toDraft))
    appStore.showSuccess(t('admin.settings.payment.quickButtons.saved'))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.settings.payment.quickButtons', t('common.error')))
  } finally {
    saveLoading.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
