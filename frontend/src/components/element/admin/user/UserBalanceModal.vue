<template>
  <el-dialog
    :model-value="show"
    :title="operation === 'add' ? t('admin.users.deposit') : t('admin.users.withdraw')"
    width="440px"
    align-center
    @update:model-value="(v: boolean) => !v && emit('close')"
    @close="emit('close')"
  >
    <el-form v-if="user" label-position="top" @submit.prevent="handleBalanceSubmit">
      <div class="mb-5 flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-primary-100">
          <span class="text-lg font-medium text-primary-700">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="flex-1">
          <p class="font-medium text-gray-900 dark:text-gray-100">{{ user.email }}</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.users.currentBalance') }}: ${{ formatBalance(user.balance) }}</p>
        </div>
      </div>

      <el-form-item :label="operation === 'add' ? t('admin.users.depositAmount') : t('admin.users.withdrawAmount')">
        <div class="flex w-full gap-2">
          <el-input v-model.number="form.amount" type="number" :min="0" step="any" class="flex-1">
            <template #prefix><span class="font-medium text-gray-500">$</span></template>
          </el-input>
          <el-button v-if="operation === 'subtract'" plain class="whitespace-nowrap" @click="fillAllBalance">
            {{ t('admin.users.withdrawAll') }}
          </el-button>
        </div>
      </el-form-item>

      <el-form-item :label="t('admin.users.notes')">
        <el-input v-model="form.notes" type="textarea" :rows="3" />
      </el-form-item>

      <el-alert v-if="form.amount > 0" type="info" :closable="false" class="!static">
        <div class="flex items-center justify-between text-sm">
          <span class="text-gray-700 dark:text-gray-300">{{ t('admin.users.newBalance') }}:</span>
          <span class="font-bold text-gray-900 dark:text-gray-100">${{ formatBalance(calculateNewBalance()) }}</span>
        </div>
      </el-alert>
    </el-form>

    <template #footer>
      <el-button @click="emit('close')">{{ t('common.cancel') }}</el-button>
      <el-button
        :type="operation === 'add' ? 'success' : 'danger'"
        :loading="submitting"
        :disabled="!form.amount"
        @click="handleBalanceSubmit"
      >
        {{ submitting ? t('common.saving') : t('common.confirm') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AdminUser } from '@/types'

const props = defineProps<{ show: boolean, user: AdminUser | null, operation: 'add' | 'subtract' }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const submitting = ref(false)
const form = reactive({ amount: 0, notes: '' })
watch(() => props.show, (v) => { if (v) { form.amount = 0; form.notes = '' } })

// 格式化余额：显示完整精度，去除尾部多余的0
const formatBalance = (value: number) => {
  if (value === 0) return '0.00'
  const formatted = value.toFixed(8).replace(/\.?0+$/, '')
  const parts = formatted.split('.')
  if (parts.length === 1) return formatted + '.00'
  if (parts[1].length === 1) return formatted + '0'
  return formatted
}

const fillAllBalance = () => {
  if (props.user) {
    form.amount = props.user.balance
  }
}

const calculateNewBalance = () => {
  if (!props.user) return 0
  const result = props.operation === 'add' ? props.user.balance + form.amount : props.user.balance - form.amount
  // 避免浮点数精度问题导致的 -0.00 显示
  return Math.abs(result) < 1e-10 ? 0 : result
}

const handleBalanceSubmit = async () => {
  if (!props.user) return
  if (!form.amount || form.amount <= 0) {
    appStore.showError(t('admin.users.amountRequired'))
    return
  }
  // 退款时验证金额不超过实际余额
  if (props.operation === 'subtract' && form.amount > props.user.balance) {
    appStore.showError(t('admin.users.insufficientBalance'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.users.updateBalance(props.user.id, form.amount, props.operation, form.notes)
    appStore.showSuccess(t('common.success')); emit('success'); emit('close')
  } catch (e: any) {
    console.error('Failed to update balance:', e)
    appStore.showError(extractApiErrorMessage(e, t('common.error')))
  } finally { submitting.value = false }
}
</script>
