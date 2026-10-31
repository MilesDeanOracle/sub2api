<template>
  <!-- Scenario A: Specific Resource Pool Selected -->
  <div
    v-if="group"
    class="pool-summary-card shrink-0 bg-white dark:bg-dark-900 border border-gray-200/80 dark:border-dark-700/80 rounded-tr-2xl rounded-tl-none rounded-b-none border-b-0 shadow-sm p-5 transition-all relative overflow-hidden"
  >
    <!-- Background subtle gradient accent -->
    <div class="pointer-events-none absolute -right-20 -top-20 w-64 h-64 bg-primary-500/5 dark:bg-primary-500/10 rounded-full blur-3xl" />

    <!-- Top Info and Actions Row -->
    <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 relative z-10">
      <!-- Left: Platform Avatar + Name + Tags + Description -->
      <div class="flex items-start gap-3.5 min-w-0">
        <div class="platform-avatar p-2.5 rounded-xl bg-gray-50 dark:bg-dark-800 border border-gray-100 dark:border-dark-700 text-gray-700 dark:text-gray-200 shadow-xs shrink-0 mt-0.5">
          <PlatformIcon :platform="group.platform" size="md" />
        </div>

        <div class="space-y-1.5 min-w-0 flex-1">
          <div class="flex items-center flex-wrap gap-2">
            <h2 class="text-lg font-bold text-gray-900 dark:text-gray-100 tracking-tight m-0">
              {{ group.name }}
            </h2>

            <!-- Multiplier Tag（双击快捷修改倍率） -->
            <input
              v-if="rateEditing"
              ref="rateInputRef"
              v-model="rateValue"
              type="number"
              min="0"
              step="0.01"
              class="w-24 px-2 py-0.5 rounded-md text-xs font-mono font-medium bg-white dark:bg-dark-850 text-purple-700 dark:text-purple-300 border border-purple-300 dark:border-purple-700 focus:outline-none focus:ring-1 focus:ring-purple-500"
              @keydown.enter.prevent="confirmRateEdit"
              @keydown.escape.prevent="cancelRateEdit"
              @blur="confirmRateEdit"
            >
            <span
              v-else
              class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-mono font-medium bg-purple-50 text-purple-700 dark:bg-purple-950/40 dark:text-purple-300 border border-purple-200/60 dark:border-purple-800/40 cursor-text select-none"
              title="双击修改倍率"
              @dblclick="startRateEdit"
            >
              {{ t('resourcePool.summary.multiplier') }}: {{ Number(group.rate_multiplier || 1).toFixed(2) }}x
            </span>

            <!-- Status Pill（点击快捷切换启用/停用） -->
            <span
              :class="[
                'inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold cursor-pointer select-none',
                group.status === 'active'
                  ? 'bg-emerald-50 text-emerald-700 border border-emerald-200/80 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-800/60'
                  : 'bg-gray-100 text-gray-600 border border-gray-200 dark:bg-dark-800 dark:text-gray-400 dark:border-dark-700'
              ]"
              :title="t('resourcePool.actions.toggleStatus')"
              @click="onToggleStatus"
            >
              <span
                class="w-1.5 h-1.5 rounded-full"
                :class="group.status === 'active' ? 'bg-emerald-500 animate-pulse' : 'bg-gray-400'"
              />
              {{ group.status === 'active' ? t('resourcePool.status.active') : t('resourcePool.status.inactive') }}
            </span>

            <!-- Exclusive Group Toggle（从分组设置挪到标题的专属分组开关） -->
            <div class="flex items-center gap-2 ml-1">
              <Toggle :model-value="Boolean(group.is_exclusive)" @update:model-value="onToggleExclusive" />
              <span class="text-sm text-gray-500 dark:text-gray-400">
                {{ group.is_exclusive ? t('admin.groups.exclusive') : t('admin.groups.public') }}
              </span>
            </div>

          </div>

          <p class="text-xs text-gray-500 dark:text-gray-400 line-clamp-2 m-0">
            {{ group.description || t('resourcePool.summary.noDescription') }}
          </p>
        </div>
      </div>

      <!-- Right: Pool Quick Actions（复刻原系统分组操作：编辑/复制/专属倍率/专属 RPM/删除） -->
      <div class="flex items-center gap-1 shrink-0">
        <button
          type="button"
          class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
          @click="onCommand('edit')"
        >
          <Icon name="edit" size="sm" />
          <span class="text-xs">{{ t("common.edit") }}</span>
        </button>
        <button
          type="button"
          class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
          @click="onCommand('duplicate')"
        >
          <Icon name="copy" size="sm" />
          <span class="text-xs">{{ t("admin.groups.duplicate") }}</span>
        </button>
        <button
          type="button"
          data-testid="group-rate-multipliers"
          class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-purple-600 dark:hover:bg-dark-700 dark:hover:text-purple-400"
          @click="onCommand('rate-multipliers')"
        >
          <Icon name="dollar" size="sm" />
          <span class="text-xs">{{ t("admin.groups.rateMultipliers") }}</span>
        </button>
        <button
          type="button"
          data-testid="group-rpm-overrides"
          class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-orange-600 dark:hover:bg-dark-700 dark:hover:text-orange-400"
          @click="onCommand('rpm-overrides')"
        >
          <Icon name="bolt" size="sm" />
          <span class="text-xs">{{ t("admin.groups.rpmOverrides") }}</span>
        </button>
        <button
          type="button"
          class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
          @click="onCommand('delete')"
        >
          <Icon name="trash" size="sm" />
          <span class="text-xs">{{ t("common.delete") }}</span>
        </button>
      </div>
    </div>

  </div>

  <!-- Scenario B: Global Scope Selected (All Accounts / Ungrouped Accounts) -->
  <div
    v-else
    class="pool-summary-card scope-hero shrink-0 bg-white dark:bg-dark-900 border border-gray-200/80 dark:border-dark-700/80 rounded-tr-2xl rounded-tl-none rounded-b-none border-b-0 shadow-sm p-5 relative overflow-hidden"
  >
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 relative z-10">
      <div class="flex items-start gap-3.5">
        <div
          :class="[
            'p-3 rounded-xl border shadow-xs shrink-0',
            scope === 'ungrouped'
              ? 'bg-amber-50 text-amber-600 border-amber-200/80 dark:bg-amber-950/50 dark:text-amber-400 dark:border-amber-800/60'
              : 'bg-blue-50 text-blue-600 border-blue-200/80 dark:bg-blue-950/50 dark:text-blue-400 dark:border-blue-800/60'
          ]"
        >
          <el-icon class="text-2xl"><component :is="scope === 'ungrouped' ? Box : Grid" /></el-icon>
        </div>
        <div class="space-y-1">
          <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100 tracking-tight m-0">
            {{ scope === 'ungrouped' ? t('resourcePool.summary.ungroupedTitle') : t('resourcePool.summary.allTitle') }}
          </h2>
          <p class="text-xs text-gray-500 dark:text-gray-400 max-w-2xl m-0 leading-relaxed">
            {{ scope === 'ungrouped' ? t('resourcePool.summary.ungroupedDescription') : t('resourcePool.summary.allDescription') }}
          </p>
        </div>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <el-button
          type="primary"
          class="!rounded-lg font-medium shadow-sm hover:shadow"
          @click="onCommand('create-account')"
        >
          <el-icon class="mr-1"><CirclePlus /></el-icon>
          {{ t('resourcePool.actions.createAccount') }}
        </el-button>
        <el-button
          plain
          class="!rounded-lg font-medium"
          @click="onCommand('create-group')"
        >
          <el-icon class="mr-1"><Plus /></el-icon>
          {{ t('resourcePool.actions.createGroup') }}
        </el-button>
        <el-button
          plain
          class="!rounded-lg !px-2.5"
          :title="t('common.refresh')"
          @click="onCommand('refresh')"
        >
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Box,
  CirclePlus,
  Grid,
  Plus,
  Refresh
} from '@element-plus/icons-vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { Icon } from '@/components/icons'
import type { AdminGroup } from '@/types'

const props = defineProps<{
  group: AdminGroup | null
  scope?: 'all' | 'ungrouped' | 'issues' | 'group'
  allCount?: number
  ungroupedCount?: number
}>()

const emit = defineEmits<{ action: [command: string, payload?: number | boolean] }>()
const { t } = useI18n()

function onCommand(command: string | number | object) {
  emit('action', String(command))
}

// —— 双击计费倍率标签,就地修改资源池倍率 ——
const rateEditing = ref(false)
const rateValue = ref('')
const rateInputRef = ref<HTMLInputElement | null>(null)

function startRateEdit() {
  if (!props.group) return
  rateValue.value = String(Number(props.group.rate_multiplier ?? 1))
  rateEditing.value = true
  nextTick(() => rateInputRef.value?.select())
}

function cancelRateEdit() {
  rateEditing.value = false
}

function confirmRateEdit() {
  if (!rateEditing.value) return
  const value = Number(rateValue.value)
  if (rateValue.value === '' || !Number.isFinite(value) || value < 0 || value === Number(props.group?.rate_multiplier ?? 1)) {
    cancelRateEdit()
    return
  }
  rateEditing.value = false
  emit('action', 'set-rate', value)
}

// 切换资源池时退出编辑态
watch(() => props.group?.id, () => {
  rateEditing.value = false
})

function onToggleExclusive(value: boolean) {
  emit('action', 'toggle-exclusive', value)
}

function onToggleStatus() {
  emit('action', 'toggle-status')
}

</script>

<style scoped>
.pool-summary-card {
  position: relative;
}

.platform-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
}

</style>
