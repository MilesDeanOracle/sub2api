<template>
  <el-dialog
    :model-value="show"
    :title="t('admin.users.createUser')"
    width="520px"
    align-center
    @update:model-value="(v: boolean) => !v && emit('close')"
    @close="emit('close')"
  >
    <el-form label-position="top" @submit.prevent="submit">
      <el-form-item :label="t('admin.users.email')" required>
        <el-input v-model="form.email" type="email" :placeholder="t('admin.users.enterEmail')" />
      </el-form-item>

      <el-form-item :label="t('admin.users.password')" required>
        <div class="flex w-full gap-2">
          <el-input v-model="form.password" :placeholder="t('admin.users.enterPassword')" class="flex-1" />
          <el-button plain :icon="RefreshIcon" @click="generateRandomPassword" />
        </div>
      </el-form-item>

      <el-form-item :label="t('admin.users.username')">
        <el-input v-model="form.username" :placeholder="t('admin.users.enterUsername')" />
      </el-form-item>

      <el-form-item :label="t('admin.users.form.roleLabel')">
        <el-select v-model="form.role">
          <el-option value="user" :label="t('admin.users.roles.user')" />
          <el-option value="admin" :label="t('admin.users.roles.admin')" />
        </el-select>
      </el-form-item>

      <div class="grid grid-cols-1 gap-x-4 sm:grid-cols-2">
        <el-form-item :label="t('admin.users.columns.balance')">
          <el-input v-model="form.balance" type="number" step="any" />
        </el-form-item>
        <el-form-item :label="t('admin.users.columns.concurrency')">
          <el-input v-model.number="form.concurrency" type="number" />
        </el-form-item>
      </div>

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
    </el-form>

    <template #footer>
      <el-button @click="emit('close')">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="loading" @click="submit">
        {{ loading ? t('admin.users.creating') : t('common.create') }}
      </el-button>
    </template>
  </el-dialog>

  <!-- 创建管理员账号时后端要求 step-up 2FA，弹出 TOTP 验证后自动重试 -->
  <TotpStepUpDialog :controller="stepUp" />
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Refresh as RefreshIcon } from '@element-plus/icons-vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '../../auth/TotpStepUpDialog.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const form = reactive({ email: '', password: '', username: '', notes: '', role: 'user' as 'user' | 'admin', balance: '', concurrency: 1, rpm_limit: 0 })

const stepUp = useStepUp()
const loading = ref(false)

const submit = async () => {
  if (loading.value) return
  loading.value = true
  try {
    const { balance: rawBalance, ...rest } = { ...form }
    const balance = String(rawBalance).trim()
    const payload: typeof rest & { balance?: number } = { ...rest }
    if (balance !== '') {
      payload.balance = Number(balance)
    }
    // 创建管理员属敏感操作：后端返回 STEP_UP_REQUIRED 时弹 TOTP 验证并重试
    await stepUp.run(() => adminAPI.users.create(payload))
    appStore.showSuccess(t('admin.users.userCreated'))
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
      appStore.showError(e?.message || t('admin.users.failedToCreate'))
    }
  } finally { loading.value = false }
}

watch(() => props.show, (v) => { if (v) Object.assign(form, { email: '', password: '', username: '', notes: '', role: 'user', balance: '', concurrency: 1, rpm_limit: 0 }) })

const generateRandomPassword = () => {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%^&*'
  let p = ''; for (let i = 0; i < 16; i++) p += chars.charAt(Math.floor(Math.random() * chars.length))
  form.password = p
}
</script>

<style scoped>
.ep-form-hint {
  @apply mt-1 w-full text-xs text-gray-400 dark:text-gray-500;
}
</style>
