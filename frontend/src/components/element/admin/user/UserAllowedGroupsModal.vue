<template>
  <el-dialog
    :model-value="show"
    :title="t('admin.users.groupConfig')"
    width="860px"
    align-center
    @update:model-value="(v: boolean) => !v && emit('close')"
    @close="emit('close')"
  >
    <div v-if="user" class="space-y-6">
      <!-- 用户信息头部 -->
      <div class="flex items-center gap-4 rounded-2xl bg-primary-50 p-5 dark:bg-primary-900/30">
        <div class="flex h-14 w-14 items-center justify-center rounded-full bg-white shadow-sm dark:bg-dark-700">
          <span class="text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="flex-1">
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ user.email }}</p>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.users.groupConfigHint', { email: user.email }) }}</p>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" v-loading="true" class="h-32" />

      <div v-else class="space-y-6">
        <!-- 专属分组区域 -->
        <div v-if="exclusiveGroups.length > 0">
          <div class="mb-3 flex items-center gap-2">
            <div class="h-1.5 w-1.5 rounded-full bg-purple-500"></div>
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">{{ t('admin.users.exclusiveGroups') }}</h4>
            <span class="text-xs text-gray-400">({{ exclusiveGroupConfigs.filter(c => c.isSelected).length }}/{{ exclusiveGroupConfigs.length }})</span>
          </div>
          <div class="grid gap-3">
            <div
              v-for="config in exclusiveGroupConfigs"
              :key="config.groupId"
              :class="[
                'rounded-xl border-2 p-4 transition-all duration-200',
                config.isSelected
                  ? 'border-primary-400 bg-primary-50/50 dark:border-primary-500 dark:bg-primary-900/20'
                  : 'border-gray-200 bg-white hover:border-gray-300 dark:border-dark-600 dark:bg-dark-800'
              ]"
            >
              <div class="flex items-center gap-4">
                <el-checkbox
                  :model-value="config.isSelected"
                  size="large"
                  @change="toggleExclusiveGroup(config.groupId)"
                />

                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="text-base font-semibold text-gray-900 dark:text-white">{{ config.groupName }}</span>
                    <el-tag size="small" effect="plain" round disable-transitions>
                      {{ t('admin.groups.exclusive') }}
                    </el-tag>
                  </div>
                  <div class="mt-1.5 flex items-center gap-3 text-sm">
                    <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
                      {{ config.platform }}
                    </span>
                    <span class="text-gray-300 dark:text-dark-500">•</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      {{ t('admin.users.defaultRate') }}: <span class="font-medium text-gray-700 dark:text-gray-300">{{ config.defaultRate }}x</span>
                    </span>
                  </div>
                </div>

                <div class="flex flex-shrink-0 items-center gap-3">
                  <label class="text-sm font-medium text-gray-600 dark:text-gray-400">{{ t('admin.users.customRate') }}</label>
                  <el-input
                    :model-value="config.customRate ?? ''"
                    type="number"
                    step="0.001"
                    min="0.001"
                    :placeholder="String(config.defaultRate)"
                    class="!w-24"
                    @update:model-value="(val: string | number) => updateCustomRate(config.groupId, String(val ?? ''))"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 公开分组区域 -->
        <div v-if="publicGroups.length > 0">
          <div class="mb-3 flex flex-wrap items-center gap-2">
            <div class="h-1.5 w-1.5 rounded-full bg-green-500"></div>
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">
              {{ restrictPublicGroups ? t('admin.users.publicGroupsRestricted') : t('admin.users.publicGroups') }}
            </h4>
            <span class="text-xs text-gray-400">({{ publicGroupConfigs.length }})</span>
            <el-checkbox
              :model-value="restrictPublicGroups"
              class="ml-auto"
              @change="toggleRestrictPublicGroups"
            >
              {{ t('admin.users.restrictPublicGroups') }}
            </el-checkbox>
          </div>
          <p class="mb-3 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.users.restrictPublicGroupsHint') }}</p>
          <div class="grid gap-3">
            <div
              v-for="config in publicGroupConfigs"
              :key="config.groupId"
              class="rounded-xl border-2 border-green-200 bg-green-50/50 p-4 dark:border-green-800/50 dark:bg-green-900/10"
            >
              <div class="flex items-center gap-4">
                <!-- 未开启限制时公开分组恒可用，此处仅作展示；开启后才是真实开关 -->
                <el-checkbox
                  :model-value="restrictPublicGroups ? config.isSelected : true"
                  :disabled="!restrictPublicGroups"
                  size="large"
                  @change="togglePublicGroup(config.groupId)"
                />

                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="text-base font-semibold text-gray-900 dark:text-white">{{ config.groupName }}</span>
                  </div>
                  <div class="mt-1.5 flex items-center gap-3 text-sm">
                    <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
                      {{ config.platform }}
                    </span>
                    <span class="text-gray-300 dark:text-dark-500">•</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      {{ t('admin.users.defaultRate') }}: <span class="font-medium text-gray-700 dark:text-gray-300">{{ config.defaultRate }}x</span>
                    </span>
                  </div>
                </div>

                <div class="flex flex-shrink-0 items-center gap-3">
                  <label class="text-sm font-medium text-gray-600 dark:text-gray-400">{{ t('admin.users.customRate') }}</label>
                  <el-input
                    :model-value="config.customRate ?? ''"
                    type="number"
                    step="0.001"
                    min="0.001"
                    :placeholder="String(config.defaultRate)"
                    class="!w-24"
                    @update:model-value="(val: string | number) => updateCustomRate(config.groupId, String(val ?? ''))"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 无分组提示 -->
        <el-empty
          v-if="groups.length === 0"
          :description="t('common.noGroupsAvailable')"
          :image-size="100"
        />
      </div>
    </div>

    <template #footer>
      <el-button @click="emit('close')">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="submitting" :disabled="!loaded" @click="handleSave">
        {{ submitting ? t('common.saving') : t('common.save') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser, Group, GroupPlatform } from '@/types'

interface GroupRateConfig {
  groupId: number
  groupName: string
  platform: GroupPlatform
  isExclusive: boolean
  defaultRate: number
  customRate: number | null
  isSelected: boolean
}

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<Group[]>([])
const groupConfigs = ref<GroupRateConfig[]>([])
const originalGroupRates = ref<Record<number, number>>({}) // 记录原始专属倍率，用于检测删除
const loading = ref(false)
const loaded = ref(false)
const submitting = ref(false)
const restrictPublicGroups = ref(false)

// 分离专属分组和公开分组
const exclusiveGroups = computed(() => groups.value.filter((g) => g.is_exclusive))
const publicGroups = computed(() => groups.value.filter((g) => !g.is_exclusive))

const exclusiveGroupConfigs = computed(() => groupConfigs.value.filter((c) => c.isExclusive))
const publicGroupConfigs = computed(() => groupConfigs.value.filter((c) => !c.isExclusive))

watch(
  () => props.show,
  (v) => {
    if (v && props.user) {
      load()
    }
  }
)

const load = async () => {
  loaded.value = false
  loading.value = true
  try {
    const res = await adminAPI.groups.list(1, 1000)
    // 只显示标准类型且活跃的分组
    groups.value = res.items.filter((g) => g.subscription_type === 'standard' && g.status === 'active')

    // 初始化配置
    const userAllowedGroups = props.user?.allowed_groups || []
    const userGroupRates = props.user?.group_rates || {}
    restrictPublicGroups.value = props.user?.restrict_public_groups ?? false

    // 保存原始专属倍率，用于检测删除操作
    originalGroupRates.value = { ...userGroupRates }

    groupConfigs.value = groups.value.map((g) => ({
      groupId: g.id,
      groupName: g.name,
      platform: g.platform,
      isExclusive: g.is_exclusive,
      defaultRate: g.rate_multiplier,
      customRate: userGroupRates[g.id] ?? null,
      // 专属分组：检查是否在 allowed_groups 中
      // 公开分组：未开启限制时恒可用；开启后同样以 allowed_groups 为准
      isSelected:
        g.is_exclusive || restrictPublicGroups.value ? userAllowedGroups.includes(g.id) : true,
    }))
    loaded.value = true
  } catch (error) {
    console.error('Failed to load groups:', error)
  } finally {
    loading.value = false
  }
}

const toggleExclusiveGroup = (groupId: number) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config && config.isExclusive) {
    config.isSelected = !config.isSelected
  }
}

const togglePublicGroup = (groupId: number) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config && !config.isExclusive) {
    config.isSelected = !config.isSelected
  }
}

// 关闭限制时把公开分组全部勾回，避免保存出一份"限制已关但只勾了两个"的误导状态。
const toggleRestrictPublicGroups = () => {
  restrictPublicGroups.value = !restrictPublicGroups.value
  if (!restrictPublicGroups.value) {
    for (const config of groupConfigs.value) {
      if (!config.isExclusive) config.isSelected = true
    }
  }
}

const updateCustomRate = (groupId: number, value: string) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config) {
    if (value === '' || value === null || value === undefined) {
      config.customRate = null
    } else {
      const numValue = parseFloat(value)
      config.customRate = isNaN(numValue) ? null : numValue
    }
  }
}

const handleSave = async () => {
  if (!props.user || !loaded.value) return
  submitting.value = true

  try {
    // 构建 allowed_groups：专属分组中被勾选的，以及开启限制后被勾选的公开分组。
    // 未开启限制时不写入公开分组，保持该表"额外授予"的原有语义。
    const allowedGroups = groupConfigs.value
      .filter((c) => c.isSelected && (c.isExclusive || restrictPublicGroups.value))
      .map((c) => c.groupId)

    // 构建 group_rates
    // - 有新专属倍率: 设置为该值
    // - 原本有专属倍率但现在被清空: 设置为 null（表示删除）
    const groupRates: Record<number, number | null> = {}
    for (const c of groupConfigs.value) {
      const hadOriginalRate = originalGroupRates.value[c.groupId] !== undefined

      if (c.customRate !== null) {
        groupRates[c.groupId] = c.customRate
      } else if (hadOriginalRate) {
        groupRates[c.groupId] = null
      }
    }

    await adminAPI.users.update(props.user.id, {
      allowed_groups: allowedGroups,
      restrict_public_groups: restrictPublicGroups.value,
      group_rates: Object.keys(groupRates).length > 0 ? groupRates : undefined,
    })

    appStore.showSuccess(t('admin.users.groupConfigUpdated'))
    emit('success')
    emit('close')
  } catch (error) {
    console.error('Failed to update user group config:', error)
  } finally {
    submitting.value = false
  }
}
</script>
