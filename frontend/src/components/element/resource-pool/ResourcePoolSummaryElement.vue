<template>
  <!-- Scenario A: Specific Resource Pool Selected -->
  <div
    v-if="group"
    class="pool-summary-card shrink-0 bg-white dark:bg-dark-900 border border-gray-200/80 dark:border-dark-700/80 rounded-2xl shadow-sm p-5 transition-all relative overflow-hidden"
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
            <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100 tracking-tight m-0">
              {{ group.name }}
            </h2>

            <!-- Status Pill -->
            <span
              :class="[
                'inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold',
                group.status === 'active'
                  ? 'bg-emerald-50 text-emerald-700 border border-emerald-200/80 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-800/60'
                  : 'bg-gray-100 text-gray-600 border border-gray-200 dark:bg-dark-800 dark:text-gray-400 dark:border-dark-700'
              ]"
            >
              <span
                class="w-1.5 h-1.5 rounded-full"
                :class="group.status === 'active' ? 'bg-emerald-500 animate-pulse' : 'bg-gray-400'"
              />
              {{ group.status === 'active' ? t('resourcePool.status.active') : t('resourcePool.status.inactive') }}
            </span>

            <!-- Platform Tag -->
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs font-medium bg-gray-100 dark:bg-dark-800 text-gray-600 dark:text-gray-300 border border-gray-200/60 dark:border-dark-700">
              <PlatformIcon :platform="group.platform" size="xs" />
              {{ platformDisplayName(group.platform) }}
            </span>

            <!-- Multiplier Tag -->
            <span class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-mono font-medium bg-purple-50 text-purple-700 dark:bg-purple-950/40 dark:text-purple-300 border border-purple-200/60 dark:border-purple-800/40">
              {{ t('resourcePool.summary.multiplier') }}: {{ Number(group.rate_multiplier || 1).toFixed(2) }}x
            </span>
          </div>

          <p class="text-xs text-gray-500 dark:text-gray-400 line-clamp-2 m-0">
            {{ group.description || t('resourcePool.summary.noDescription') }}
          </p>
        </div>
      </div>

      <!-- Right: Action Buttons Group -->
      <div class="flex items-center flex-wrap gap-2 shrink-0">
        <!-- Add Existing Accounts (Primary Action for Resource Pool) -->
        <el-button
          type="primary"
          class="!rounded-lg font-medium shadow-sm hover:shadow"
          @click="onCommand('add-existing')"
        >
          <el-icon class="mr-1"><Plus /></el-icon>
          {{ t('resourcePool.actions.addExisting') }}
        </el-button>

        <!-- Create Account Directly -->
        <el-button
          plain
          class="!rounded-lg font-medium"
          @click="onCommand('create-account')"
        >
          <el-icon class="mr-1"><CirclePlus /></el-icon>
          {{ t('resourcePool.actions.createAccount') }}
        </el-button>

        <!-- Edit Pool -->
        <el-button
          plain
          class="!rounded-lg"
          :title="t('common.edit')"
          @click="onCommand('edit')"
        >
          <el-icon class="mr-1"><Edit /></el-icon>
          {{ t('common.edit') }}
        </el-button>

        <!-- Advanced Config -->
        <el-tooltip :content="t('resourcePool.advanced.description')" placement="top">
          <el-button
            plain
            class="!rounded-lg"
            @click="onCommand('advanced')"
          >
            <el-icon class="mr-1"><Setting /></el-icon>
            {{ t('resourcePool.actions.advancedSettings') }}
          </el-button>
        </el-tooltip>

        <!-- More Actions Dropdown -->
        <el-dropdown trigger="click" @command="onCommand">
          <el-button plain class="!rounded-lg !px-2.5">
            <el-icon><MoreFilled /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu class="pool-action-dropdown">
              <el-dropdown-item command="duplicate">
                <el-icon><CopyDocument /></el-icon>
                <span>{{ t('resourcePool.actions.duplicate') }}</span>
              </el-dropdown-item>
              <el-dropdown-item command="refresh">
                <el-icon><Refresh /></el-icon>
                <span>{{ t('common.refresh') }}</span>
              </el-dropdown-item>
              <el-dropdown-item divided command="delete" class="!text-red-500 hover:!bg-red-50 dark:hover:!bg-red-950/30">
                <el-icon><Delete /></el-icon>
                <span>{{ t('common.delete') }}</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <!-- 4 KPI Metrics Grid -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 mt-4 pt-4 border-t border-gray-100 dark:border-dark-800">
      <!-- Total Accounts -->
      <div class="metric-card bg-gray-50/70 dark:bg-dark-800/50 border border-gray-100 dark:border-dark-700/60 rounded-xl p-3 flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-950/60 dark:text-blue-400 flex items-center justify-center shrink-0">
          <el-icon class="text-lg"><UserFilled /></el-icon>
        </div>
        <div class="min-w-0">
          <span class="text-xs text-gray-500 dark:text-gray-400 block truncate">{{ t('resourcePool.summary.accounts') }}</span>
          <span class="text-lg font-bold text-gray-900 dark:text-gray-100 tracking-tight leading-none block my-0.5">
            {{ group.account_count ?? 0 }}
          </span>
          <span class="text-[11px] text-gray-400 dark:text-dark-400 block truncate">{{ t('resourcePool.summary.accountsSubtitle') }}</span>
        </div>
      </div>

      <!-- Available Accounts -->
      <div class="metric-card bg-gray-50/70 dark:bg-dark-800/50 border border-gray-100 dark:border-dark-700/60 rounded-xl p-3 flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400 flex items-center justify-center shrink-0">
          <el-icon class="text-lg"><CircleCheckFilled /></el-icon>
        </div>
        <div class="min-w-0">
          <span class="text-xs text-gray-500 dark:text-gray-400 block truncate">{{ t('resourcePool.summary.available') }}</span>
          <span class="text-lg font-bold text-emerald-600 dark:text-emerald-400 tracking-tight leading-none block my-0.5">
            {{ group.active_account_count ?? 0 }}
          </span>
          <span class="text-[11px] text-gray-400 dark:text-dark-400 block truncate">{{ t('resourcePool.summary.availableSubtitle') }}</span>
        </div>
      </div>

      <!-- Rate Limited or Error Accounts -->
      <div class="metric-card bg-gray-50/70 dark:bg-dark-800/50 border border-gray-100 dark:border-dark-700/60 rounded-xl p-3 flex items-center gap-3">
        <div
          :class="[
            'w-10 h-10 rounded-lg flex items-center justify-center shrink-0',
            (group.rate_limited_account_count && group.rate_limited_account_count > 0)
              ? 'bg-amber-50 text-amber-600 dark:bg-amber-950/60 dark:text-amber-400'
              : 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-gray-500'
          ]"
        >
          <el-icon class="text-lg"><WarningFilled /></el-icon>
        </div>
        <div class="min-w-0">
          <span class="text-xs text-gray-500 dark:text-gray-400 block truncate">{{ t('resourcePool.summary.rateLimited') }}</span>
          <span
            :class="[
              'text-lg font-bold tracking-tight leading-none block my-0.5',
              (group.rate_limited_account_count && group.rate_limited_account_count > 0)
                ? 'text-amber-600 dark:text-amber-400'
                : 'text-gray-700 dark:text-gray-300'
            ]"
          >
            {{ group.rate_limited_account_count ?? 0 }}
          </span>
          <span class="text-[11px] text-gray-400 dark:text-dark-400 block truncate">
            {{ (group.rate_limited_account_count && group.rate_limited_account_count > 0) ? '需关注恢复时间' : '当前正常无受限' }}
          </span>
        </div>
      </div>

      <!-- Multiplier -->
      <div class="metric-card bg-gray-50/70 dark:bg-dark-800/50 border border-gray-100 dark:border-dark-700/60 rounded-xl p-3 flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-purple-50 text-purple-600 dark:bg-purple-950/60 dark:text-purple-400 flex items-center justify-center shrink-0">
          <el-icon class="text-lg"><Coin /></el-icon>
        </div>
        <div class="min-w-0">
          <span class="text-xs text-gray-500 dark:text-gray-400 block truncate">{{ t('resourcePool.summary.multiplier') }}</span>
          <span class="text-lg font-bold text-purple-600 dark:text-purple-400 font-mono tracking-tight leading-none block my-0.5">
            {{ Number(group.rate_multiplier || 1).toFixed(2) }}x
          </span>
          <span class="text-[11px] text-gray-400 dark:text-dark-400 block truncate">{{ t('resourcePool.summary.multiplierSubtitle') }}</span>
        </div>
      </div>
    </div>
  </div>

  <!-- Scenario B: Global Scope Selected (All Accounts / Ungrouped Accounts) -->
  <div
    v-else
    class="pool-summary-card scope-hero shrink-0 bg-white dark:bg-dark-900 border border-gray-200/80 dark:border-dark-700/80 rounded-2xl shadow-sm p-5 relative overflow-hidden"
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
import { useI18n } from 'vue-i18n'
import {
  Box,
  CircleCheckFilled,
  CirclePlus,
  Coin,
  CopyDocument,
  Delete,
  Edit,
  Grid,
  MoreFilled,
  Plus,
  Refresh,
  Setting,
  UserFilled,
  WarningFilled
} from '@element-plus/icons-vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { AdminGroup } from '@/types'

defineProps<{
  group: AdminGroup | null
  scope?: 'all' | 'ungrouped' | 'issues' | 'group'
  allCount?: number
  ungroupedCount?: number
}>()

const emit = defineEmits<{ action: [command: string] }>()
const { t } = useI18n()

function onCommand(command: string | number | object) {
  emit('action', String(command))
}

function platformDisplayName(platform: string) {
  const map: Record<string, string> = {
    anthropic: 'Anthropic Claude',
    openai: 'OpenAI',
    gemini: 'Google Gemini',
    antigravity: 'Antigravity',
    grok: 'xAI Grok',
    bedrock: 'AWS Bedrock',
    vertex: 'Google Vertex AI',
    typesafe: 'TypeSafe',
    kimi: 'Moonshot Kimi',
    zhipu: '智谱 GLM',
    deepseek: 'DeepSeek',
    opencode_go: 'OpenCode Go',
    ollama: 'Ollama',
    minimax: 'MiniMax'
  }
  return map[platform] ?? platform
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

.metric-card {
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.metric-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.04);
}
</style>
