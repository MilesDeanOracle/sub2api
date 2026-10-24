<template>
  <el-dialog
    :model-value="show"
    :title="t('admin.users.userApiKeys')"
    width="860px"
    align-center
    @update:model-value="(v: boolean) => !v && handleClose()"
    @close="handleClose"
  >
    <div v-if="user" class="space-y-4">
      <div class="flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30">
          <span class="text-lg font-medium text-primary-700 dark:text-primary-300">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div>
          <p class="font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ user.username }}</p>
        </div>
      </div>

      <div v-if="loading" v-loading="true" class="h-32" />

      <el-empty v-else-if="apiKeys.length === 0" :description="t('admin.users.noApiKeys')" :image-size="80" />

      <div v-else class="max-h-96 space-y-3 overflow-y-auto">
        <div
          v-for="key in apiKeys"
          :key="key.id"
          class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="flex items-start justify-between">
            <div class="min-w-0 flex-1">
              <div class="mb-1 flex items-center gap-2">
                <span class="font-medium text-gray-900 dark:text-white">{{ key.name }}</span>
                <el-tag :type="key.status === 'active' ? 'success' : 'danger'" size="small" disable-transitions>
                  {{ key.status }}
                </el-tag>
              </div>
              <p class="truncate font-mono text-sm text-gray-500">{{ key.key.substring(0, 20) }}...{{ key.key.substring(key.key.length - 8) }}</p>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap gap-4 text-xs text-gray-500">
            <div class="flex items-center gap-1">
              <span>{{ t('admin.users.group') }}:</span>
              <!-- 分组下拉：更换密钥绑定的分组 -->
              <el-dropdown
                trigger="click"
                popper-class="ep-key-group-dropdown"
                :disabled="updatingKeyIds.has(key.id)"
                @command="(groupId: number | null) => changeGroup(key, groupId)"
              >
                <button
                  class="-mx-1 -my-0.5 flex cursor-pointer items-center gap-1 rounded-md px-1 py-0.5 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed dark:hover:bg-dark-700"
                  :disabled="updatingKeyIds.has(key.id)"
                  @click.stop
                >
                  <GroupBadge
                    v-if="key.group_id && key.group"
                    :name="key.group.name"
                    :platform="key.group.platform"
                    :subscription-type="key.group.subscription_type"
                    :rate-multiplier="key.group.rate_multiplier"
                    :peak-rate-enabled="key.group.peak_rate_enabled"
                    :peak-start="key.group.peak_start"
                    :peak-end="key.group.peak_end"
                    :peak-rate-multiplier="key.group.peak_rate_multiplier"
                  />
                  <span v-else class="text-gray-400 italic">{{ t('admin.users.none') }}</span>
                  <el-icon v-if="updatingKeyIds.has(key.id)" class="is-loading text-primary-500"><Loading /></el-icon>
                  <el-icon v-else class="text-gray-400"><Switch /></el-icon>
                </button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item :command="null">
                      <span class="flex w-full items-center justify-between">
                        <span class="text-gray-500 italic">{{ t('admin.users.none') }}</span>
                        <el-icon v-if="!key.group_id" class="text-primary-600 dark:text-primary-400"><Check /></el-icon>
                      </span>
                    </el-dropdown-item>
                    <el-dropdown-item
                      v-for="group in allGroups"
                      :key="group.id"
                      :command="group.id"
                    >
                      <GroupOptionItem
                        :name="group.name"
                        :platform="group.platform"
                        :subscription-type="group.subscription_type"
                        :rate-multiplier="group.rate_multiplier"
                        :peak-rate-enabled="group.peak_rate_enabled"
                        :peak-start="group.peak_start"
                        :peak-end="group.peak_end"
                        :peak-rate-multiplier="group.peak_rate_multiplier"
                        :description="group.description"
                        :selected="key.group_id === group.id"
                      />
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
            <div class="flex items-center gap-1">
              <span>{{ t('admin.users.columns.created') }}: {{ formatDateTime(key.created_at) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Loading, Switch } from '@element-plus/icons-vue'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import type { AdminUser, AdminGroup, ApiKey } from '@/types'
import GroupBadge from '../../common/GroupBadge.vue'
import GroupOptionItem from '../../common/GroupOptionItem.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close'])
const { t } = useI18n()
const appStore = useAppStore()

const apiKeys = ref<ApiKey[]>([])
const allGroups = ref<AdminGroup[]>([])
const loading = ref(false)
let requestVersion = 0
const updatingKeyIds = ref(new Set<number>())

watch(() => [props.show, props.user?.id] as const, ([show]) => {
  requestVersion++
  if (show && props.user) {
    load()
    loadGroups()
  }
})

const load = async () => {
  if (!props.user) return
  const version = ++requestVersion
  apiKeys.value = []
  loading.value = true
  try {
    const res = await adminAPI.users.getUserApiKeys(props.user.id)
    if (version === requestVersion) apiKeys.value = res.items || []
  } catch (error) {
    if (version !== requestVersion) return
    console.error('Failed to load API keys:', error)
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

const loadGroups = async () => {
  try {
    const groups = await adminAPI.groups.getAll()
    allGroups.value = groups
  } catch (error) {
    console.error('Failed to load groups:', error)
  }
}

const changeGroup = async (key: ApiKey, newGroupId: number | null) => {
  if (key.group_id === newGroupId || (!key.group_id && newGroupId === null)) return

  updatingKeyIds.value.add(key.id)
  try {
    const result = await adminAPI.apiKeys.updateApiKeyGroup(key.id, newGroupId)
    // Update local data
    const idx = apiKeys.value.findIndex((k) => k.id === key.id)
    if (idx !== -1) {
      apiKeys.value[idx] = result.api_key
    }
    if (result.auto_granted_group_access && result.granted_group_name) {
      appStore.showSuccess(t('admin.users.groupChangedWithGrant', { group: result.granted_group_name }))
    } else {
      appStore.showSuccess(t('admin.users.groupChangedSuccess'))
    }
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.users.groupChangeFailed'))
  } finally {
    updatingKeyIds.value.delete(key.id)
  }
}

const handleClose = () => {
  emit('close')
}
</script>

<style>
/* 分组下拉项内容较宽，放宽默认宽度限制 */
.ep-key-group-dropdown {
  max-width: 420px;
}

.ep-key-group-dropdown .el-dropdown-menu__item {
  padding: 6px 12px;
  line-height: normal;
}
</style>
