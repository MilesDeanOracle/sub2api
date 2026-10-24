<template>
  <el-dialog
    :model-value="show"
    :title="t('admin.users.editUser')"
    width="520px"
    align-center
    @update:model-value="(v: boolean) => !v && emit('close')"
    @close="emit('close')"
  >
    <el-form v-if="user" label-position="top" @submit.prevent="handleUpdateUser">
      <el-form-item :label="t('admin.users.email')">
        <el-input v-model="form.email" type="email" />
      </el-form-item>

      <el-form-item :label="t('admin.users.password')">
        <div class="flex w-full gap-2">
          <el-input v-model="form.password" :placeholder="t('admin.users.enterNewPassword')" class="flex-1">
            <template v-if="form.password" #suffix>
              <el-icon
                :class="passwordCopied ? 'text-green-500' : 'text-gray-400'"
                class="cursor-pointer"
                @click="copyPassword"
              >
                <CircleCheck v-if="passwordCopied" />
                <CopyDocument v-else />
              </el-icon>
            </template>
          </el-input>
          <el-button plain :icon="RefreshIcon" @click="generatePassword" />
        </div>
      </el-form-item>

      <el-form-item :label="t('admin.users.username')">
        <el-input v-model="form.username" />
      </el-form-item>

      <el-form-item :label="t('admin.users.form.roleLabel')">
        <el-select v-model="form.role">
          <el-option value="user" :label="t('admin.users.roles.user')" />
          <el-option value="admin" :label="t('admin.users.roles.admin')" />
        </el-select>
      </el-form-item>

      <el-form-item :label="t('admin.users.notes')">
        <el-input v-model="form.notes" type="textarea" :rows="3" />
      </el-form-item>

      <el-form-item :label="t('admin.users.columns.concurrency')">
        <el-input
          v-model.number="form.concurrency"
          type="number"
          :min="0"
          step="1"
          :placeholder="t('admin.users.form.concurrencyPlaceholder')"
          data-test="concurrency-input"
        />
        <p class="ep-form-hint">{{ t('admin.users.form.concurrencyHint') }}</p>
      </el-form-item>

      <el-form-item :label="t('admin.users.form.rpmLimit')">
        <el-input
          v-model.number="form.rpm_limit"
          type="number"
          :min="0"
          step="1"
          :placeholder="t('admin.users.form.rpmLimitPlaceholder')"
        />
        <p class="ep-form-hint">{{ t('admin.users.form.rpmLimitHint') }}</p>
      </el-form-item>

      <UserAttributeForm v-model="form.customAttributes" :user-id="user?.id" />
    </el-form>

    <template #footer>
      <el-button @click="emit('close')">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="submitting" @click="handleUpdateUser">
        {{ submitting ? t('admin.users.updating') : t('common.update') }}
      </el-button>
    </template>
  </el-dialog>

  <!-- 角色提升为管理员时后端要求 step-up 2FA，弹出 TOTP 验证后自动重试 -->
  <TotpStepUpDialog :controller="stepUp" />
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleCheck, CopyDocument, Refresh as RefreshIcon } from '@element-plus/icons-vue'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { adminAPI } from '@/api/admin'
import type { AdminUser, UserAttributeValuesMap } from '@/types'
import UserAttributeForm from '../../user/UserAttributeForm.vue'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '../../auth/TotpStepUpDialog.vue'

const props = defineProps<{ show: boolean, user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n(); const appStore = useAppStore(); const { copyToClipboard } = useClipboard()

const submitting = ref(false); const passwordCopied = ref(false)
const form = reactive({
  email: '',
  password: '',
  username: '',
  notes: '',
  role: 'user' as AdminUser['role'],
  concurrency: 1,
  rpm_limit: 0,
  customAttributes: {} as UserAttributeValuesMap
})

watch(() => props.user, (u) => {
  if (u) {
    Object.assign(form, { email: u.email, password: '', username: u.username || '', notes: u.notes || '', role: u.role || 'user', concurrency: u.concurrency, rpm_limit: u.rpm_limit ?? 0, customAttributes: {} })
    passwordCopied.value = false
  }
}, { immediate: true })

const generatePassword = () => {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%^&*'
  let p = ''; for (let i = 0; i < 16; i++) p += chars.charAt(Math.floor(Math.random() * chars.length))
  form.password = p
}
const copyPassword = async () => {
  if (form.password && await copyToClipboard(form.password, t('admin.users.passwordCopied'))) {
    passwordCopied.value = true; setTimeout(() => passwordCopied.value = false, 2000)
  }
}
const stepUp = useStepUp()

const handleUpdateUser = async () => {
  if (!props.user) return
  if (!form.email.trim()) {
    appStore.showError(t('admin.users.emailRequired'))
    return
  }
  // 0 = 不限制，与网关 (AcquireUserSlot: maxConcurrency <= 0) 和批量改限额一致
  if (!Number.isInteger(form.concurrency) || form.concurrency < 0) {
    appStore.showError(t('admin.users.concurrencyNonNegative'))
    return
  }
  const userId = props.user.id
  submitting.value = true
  try {
    const data: any = { email: form.email, username: form.username, notes: form.notes, role: form.role, concurrency: form.concurrency, rpm_limit: form.rpm_limit }
    if (form.password.trim()) data.password = form.password.trim()
    // 提升为管理员属敏感操作：后端返回 STEP_UP_REQUIRED 时弹 TOTP 验证并重试
    await stepUp.run(() => adminAPI.users.update(userId, data))
    if (Object.keys(form.customAttributes).length > 0) await adminAPI.userAttributes.updateUserAttributeValues(userId, form.customAttributes)
    appStore.showSuccess(t('admin.users.userUpdated'))
    emit('success'); emit('close')
  } catch (e: any) {
    if (isStepUpCancelled(e)) {
      // 用户主动取消二次验证：静默返回，表单保持打开。
    } else if (isStepUpBlocked(e)) {
      appStore.showError(
        stepUpBlockReason(e) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
          ? t('stepUp.adminApiKeyForbidden')
          : t('stepUp.notEnabled')
      )
    } else {
      appStore.showError(e?.message || t('admin.users.failedToUpdate'))
    }
  } finally { submitting.value = false }
}
</script>

<style scoped>
.ep-form-hint {
  @apply mt-1 w-full text-xs text-gray-400 dark:text-gray-500;
}
</style>
