<template>
  <el-dialog
    :model-value="show"
    :title="t('admin.users.replaceGroupTitle')"
    width="440px"
    align-center
    @update:model-value="(v: boolean) => !v && emit('close')"
    @close="emit('close')"
  >
    <div v-if="oldGroup" class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-gray-400">
        {{ t('admin.users.replaceGroupHint', { old: oldGroup.name }) }}
      </p>

      <!-- 当前分组 → 新分组 -->
      <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-800">
        <div class="flex items-center gap-2">
          <el-icon class="text-purple-500"><Star /></el-icon>
          <span class="font-medium text-gray-900 dark:text-white">{{ oldGroup.name }}</span>
          <el-icon class="ml-auto text-gray-400"><ArrowRight /></el-icon>
          <span v-if="selectedGroupId" class="font-medium text-primary-600 dark:text-primary-400">
            {{ availableGroups.find(g => g.id === selectedGroupId)?.name }}
          </span>
          <span v-else class="text-sm text-gray-400">?</span>
        </div>
      </div>

      <!-- 可选分组列表（el-radio 卡片式） -->
      <el-radio-group v-if="availableGroups.length > 0" v-model="selectedGroupId" class="!block max-h-64 overflow-y-auto">
        <el-radio
          v-for="group in availableGroups"
          :key="group.id"
          :value="group.id"
          border
          class="!mr-0 !mb-2 !w-full !h-auto"
        >
          <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
          <span class="ml-2 text-xs text-gray-400">{{ group.platform }}</span>
        </el-radio>
      </el-radio-group>

      <!-- 无可选分组 -->
      <div v-else class="py-6 text-center text-sm text-gray-400">
        {{ t('admin.users.noOtherGroups') }}
      </div>
    </div>

    <template #footer>
      <el-button @click="emit('close')">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="submitting" :disabled="!selectedGroupId" @click="handleReplace">
        {{ submitting ? t('common.saving') : t('admin.users.replaceGroupConfirm') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Star, ArrowRight } from '@element-plus/icons-vue'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AdminUser, AdminGroup } from '@/types'

interface Props {
  show: boolean
  user: AdminUser | null
  oldGroup: { id: number; name: string } | null
  allGroups: AdminGroup[]
}

const props = defineProps<Props>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const selectedGroupId = ref<number | null>(null)
const submitting = ref(false)

// 可选的专属标准分组（排除当前 oldGroup）
const availableGroups = computed(() => {
  if (!props.oldGroup) return []
  return props.allGroups.filter(
    g => g.status === 'active' && g.is_exclusive && g.subscription_type === 'standard' && g.id !== props.oldGroup!.id
  )
})

watch(() => props.show, (v) => {
  if (v) {
    selectedGroupId.value = null
  }
})

const handleReplace = async () => {
  if (!props.user || !props.oldGroup || !selectedGroupId.value) return
  submitting.value = true

  try {
    const result = await adminAPI.users.replaceGroup(props.user.id, props.oldGroup.id, selectedGroupId.value)
    appStore.showSuccess(t('admin.users.replaceGroupSuccess', { count: result.migrated_keys }))
    emit('success')
    emit('close')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
    console.error('Failed to replace group:', error)
  } finally {
    submitting.value = false
  }
}
</script>
