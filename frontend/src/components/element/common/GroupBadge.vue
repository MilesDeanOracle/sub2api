<template>
  <el-tag
    :class="['ep-group-badge', badgeClass, truncate ? 'ep-truncate' : 'ep-wrap']"
    size="small"
    disable-transitions
  >
    <!-- 平台色圆点（Element Plus 图标库无品牌 logo，用平台色圆点标识） -->
    <span v-if="platform" :class="['ep-platform-dot', platformDotClass]" />
    <span :class="truncate ? 'truncate' : 'min-w-0 whitespace-normal [overflow-wrap:anywhere]'">{{ name }}</span>
    <span v-if="showLabel" :class="labelClass">
      <template v-if="hasCustomRate">
        <span class="mr-0.5 line-through opacity-50">{{ rateMultiplier }}x</span>
        <span class="font-bold">{{ userRateMultiplier }}x</span>
      </template>
      <template v-else>
        {{ labelText }}
      </template>
    </span>
    <span v-if="hasPeakRate" :class="peakRateClass" :title="peakRateTitle">
      {{ peakRateText }}
    </span>
  </el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionType, GroupPlatform } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'

interface Props {
  name: string
  platform?: GroupPlatform
  subscriptionType?: SubscriptionType
  rateMultiplier?: number
  userRateMultiplier?: number | null
  peakRateEnabled?: boolean
  peakStart?: string
  peakEnd?: string
  peakRateMultiplier?: number
  showRate?: boolean
  truncate?: boolean
  daysRemaining?: number | null
  alwaysShowRate?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  subscriptionType: 'standard',
  showRate: true,
  truncate: true,
  daysRemaining: null,
  userRateMultiplier: null,
  peakRateEnabled: false,
  alwaysShowRate: false
})

const { t } = useI18n()

const isSubscription = computed(() => props.subscriptionType === 'subscription')

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
  return Boolean(props.showRate && props.peakRateEnabled && props.peakStart && props.peakEnd)
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

const showLabel = computed(() => {
  if (!props.showRate) return false
  if (isSubscription.value) return true
  return props.rateMultiplier !== undefined || hasCustomRate.value
})

const labelText = computed(() => {
  const rateLabel = props.rateMultiplier !== undefined ? `${props.rateMultiplier}x` : ''
  if (isSubscription.value && !props.alwaysShowRate) {
    if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
      if (props.daysRemaining <= 0) {
        return t('admin.users.expired')
      }
      return t('admin.users.daysRemaining', { days: props.daysRemaining })
    }
    return t('groups.subscription')
  }
  return rateLabel
})

// 平台圆点配色
const PLATFORM_DOTS: Record<string, string> = {
  anthropic: 'bg-orange-500',
  openai: 'bg-emerald-500',
  gemini: 'bg-blue-500',
  antigravity: 'bg-purple-500',
  grok: 'bg-zinc-500',
  kimi: 'bg-pink-500',
  zhipu: 'bg-indigo-500',
  deepseek: 'bg-teal-500',
  minimax: 'bg-rose-500',
  composite: 'bg-cyan-500'
}

const platformDotClass = computed(
  () => PLATFORM_DOTS[props.platform ?? ''] ?? 'bg-violet-500'
)

// 徽章配色：具体颜色见 <style> 中的平台类（scoped 属性保证优先级高于 el-tag 默认色）
const badgeClass = computed(() => {
  if (isSubscription.value) return 'ep-tag-subscription'
  return props.platform ? `ep-tag-${props.platform}` : 'ep-tag-default'
})

const labelClass = computed(() => {
  const base = 'px-1.5 py-0.5 rounded text-[10px] font-semibold'

  if (!isSubscription.value) {
    return `${base} bg-black/10 dark:bg-white/10`
  }

  if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
    if (props.daysRemaining <= 0 || props.daysRemaining <= 3) {
      return `${base} bg-red-200/80 text-red-800 dark:bg-red-800/50 dark:text-red-300`
    }
    if (props.daysRemaining <= 7) {
      return `${base} bg-amber-200/80 text-amber-800 dark:bg-amber-800/50 dark:text-amber-300`
    }
  }

  if (props.platform === 'anthropic') {
    return `${base} bg-orange-200/60 text-orange-800 dark:bg-orange-800/40 dark:text-orange-300`
  }
  if (props.platform === 'openai') {
    return `${base} bg-emerald-200/60 text-emerald-800 dark:bg-emerald-800/40 dark:text-emerald-300`
  }
  if (props.platform === 'gemini') {
    return `${base} bg-blue-200/60 text-blue-800 dark:bg-blue-800/40 dark:text-blue-300`
  }
  if (props.platform === 'antigravity') {
    return `${base} bg-purple-200/60 text-purple-800 dark:bg-purple-800/40 dark:text-purple-300`
  }
  if (props.platform === 'grok') {
    return `${base} bg-zinc-300/70 text-zinc-800 dark:bg-zinc-700/60 dark:text-zinc-200`
  }
  if (props.platform === 'kimi') {
    return `${base} bg-pink-200/60 text-pink-800 dark:bg-pink-800/40 dark:text-pink-300`
  }
  if (props.platform === 'zhipu') {
    return `${base} bg-indigo-200/60 text-indigo-800 dark:bg-indigo-800/40 dark:text-indigo-300`
  }
  if (props.platform === 'deepseek') {
    return `${base} bg-teal-200/60 text-teal-800 dark:bg-teal-800/40 dark:text-teal-300`
  }
  if (props.platform === 'minimax') {
    return `${base} bg-rose-200/60 text-rose-800 dark:bg-rose-800/40 dark:text-rose-300`
  }
  if (props.platform === 'composite') {
    return `${base} bg-cyan-200/70 text-cyan-900 dark:bg-cyan-900/50 dark:text-cyan-300`
  }
  return `${base} bg-violet-200/60 text-violet-800 dark:bg-violet-800/40 dark:text-violet-300`
})

const peakRateClass = computed(() => {
  return 'px-1.5 py-0.5 rounded text-[10px] font-semibold bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
})
</script>

<style scoped>
.ep-group-badge {
  height: auto;
  gap: 0.375rem;
  padding: 0.125rem 0.5rem;
  border-color: transparent;
  font-weight: 500;
}

.ep-group-badge.ep-truncate {
  max-width: 100%;
}

.ep-platform-dot {
  display: inline-block;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
  flex-shrink: 0;
}

/* 平台配色：scoped data 属性使这些规则优先于 el-tag 的默认 CSS 变量配色 */
.ep-tag-anthropic { @apply bg-orange-100 text-orange-700; }
html.dark .ep-tag-anthropic { @apply bg-orange-900/30 text-orange-400; }

.ep-tag-openai { @apply bg-emerald-100 text-emerald-700; }
html.dark .ep-tag-openai { @apply bg-emerald-900/30 text-emerald-400; }

.ep-tag-gemini { @apply bg-blue-100 text-blue-700; }
html.dark .ep-tag-gemini { @apply bg-blue-900/30 text-blue-400; }

.ep-tag-antigravity { @apply bg-purple-100 text-purple-700; }
html.dark .ep-tag-antigravity { @apply bg-purple-900/30 text-purple-400; }

.ep-tag-grok { @apply bg-zinc-200 text-zinc-800; }
html.dark .ep-tag-grok { @apply bg-zinc-700 text-zinc-100; }

.ep-tag-kimi { @apply bg-pink-100 text-pink-700; }
html.dark .ep-tag-kimi { @apply bg-pink-900/30 text-pink-400; }

.ep-tag-zhipu { @apply bg-indigo-100 text-indigo-700; }
html.dark .ep-tag-zhipu { @apply bg-indigo-900/30 text-indigo-400; }

.ep-tag-deepseek { @apply bg-teal-100 text-teal-700; }
html.dark .ep-tag-deepseek { @apply bg-teal-900/30 text-teal-400; }

.ep-tag-minimax { @apply bg-rose-100 text-rose-700; }
html.dark .ep-tag-minimax { @apply bg-rose-900/30 text-rose-400; }

.ep-tag-composite { @apply bg-cyan-100 text-cyan-800; }
html.dark .ep-tag-composite { @apply bg-cyan-900/30 text-cyan-300; }

.ep-tag-default { @apply bg-emerald-100 text-emerald-700; }
html.dark .ep-tag-default { @apply bg-emerald-900/30 text-emerald-400; }

.ep-tag-subscription { @apply bg-violet-100 text-violet-700; }
html.dark .ep-tag-subscription { @apply bg-violet-900/30 text-violet-400; }
</style>
