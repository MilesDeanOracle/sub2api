<template>
  <div class="flex min-w-0 flex-1 items-start justify-between gap-3">
    <!-- 左侧：徽章 + 描述 -->
    <div
      class="flex min-w-0 flex-1 flex-col items-start"
      :title="description || undefined"
    >
      <GroupBadge
        :name="name"
        :platform="platform"
        :subscription-type="subscriptionType"
        :show-rate="false"
        class="ep-option-badge"
      />
      <span
        v-if="description"
        class="mt-1.5 w-full whitespace-pre-line [overflow-wrap:anywhere] text-left text-xs leading-relaxed text-gray-500 dark:text-gray-400 line-clamp-3"
      >
        {{ description }}
      </span>
    </div>

    <!-- 右侧：倍率标签 + 选中勾 -->
    <div class="flex shrink-0 items-center gap-2 pt-0.5">
      <div class="flex shrink-0 flex-col items-end gap-1">
        <el-tag
          v-if="rateMultiplier !== undefined"
          :class="ratePillClass"
          size="small"
          round
          disable-transitions
        >
          <template v-if="hasCustomRate">
            <span class="mr-1 line-through opacity-50">{{ rateMultiplier }}x</span>
            <span class="font-bold">{{ userRateMultiplier }}x</span>
          </template>
          <template v-else>
            {{ rateMultiplier }}x {{ t('admin.groups.rateLabel') }}
          </template>
        </el-tag>
        <el-tag
          v-if="hasPeakRate"
          class="bg-amber-100 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
          size="small"
          round
          disable-transitions
          :title="peakRateTitle"
        >
          {{ peakRateText }}
        </el-tag>
      </div>
      <el-icon v-if="showCheckmark && selected" class="h-4 w-4 shrink-0 text-primary-600 dark:text-primary-400">
        <Check />
      </el-icon>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check } from '@element-plus/icons-vue'
import GroupBadge from './GroupBadge.vue'
import type { SubscriptionType, GroupPlatform } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'

const { t } = useI18n()

interface Props {
  name: string
  platform: GroupPlatform
  subscriptionType?: SubscriptionType
  rateMultiplier?: number
  userRateMultiplier?: number | null
  peakRateEnabled?: boolean
  peakStart?: string
  peakEnd?: string
  peakRateMultiplier?: number
  description?: string | null
  selected?: boolean
  showCheckmark?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  subscriptionType: 'standard',
  selected: false,
  showCheckmark: true,
  userRateMultiplier: null,
  peakRateEnabled: false
})

const hasCustomRate = computed(() => {
  return (
    props.userRateMultiplier !== null &&
    props.userRateMultiplier !== undefined &&
    props.rateMultiplier !== undefined &&
    props.userRateMultiplier !== props.rateMultiplier
  )
})

const appStore = useAppStore()

const hasPeakRate = computed(() => {
  return Boolean(props.peakRateEnabled && props.peakStart && props.peakEnd)
})

const peakRateText = computed(() => {
  return formatPeakRateWindow(
    {
      peak_rate_enabled: props.peakRateEnabled,
      peak_start: props.peakStart,
      peak_end: props.peakEnd,
      peak_rate_multiplier: props.peakRateMultiplier
    },
    serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)
  )
})

const peakRateTitle = computed(() => {
  return t('common.peakRateTooltip', { window: peakRateText.value })
})

const ratePillClass = computed(() => {
  switch (props.platform) {
    case 'anthropic':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
    case 'openai':
      return 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
    case 'gemini':
      return 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-400'
    default:
      return 'bg-violet-50 text-violet-700 dark:bg-violet-900/20 dark:text-violet-400'
  }
})
</script>

<style scoped>
.ep-option-badge :deep(span) {
  font-weight: 600;
}
</style>
