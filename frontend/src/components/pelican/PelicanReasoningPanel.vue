<template>
  <details v-if="text" class="group mb-4 rounded-xl border border-violet-200 bg-violet-50/50 dark:border-violet-900/60 dark:bg-violet-950/20" data-testid="pelican-reasoning">
    <summary class="cursor-pointer px-4 py-3 text-sm font-medium text-violet-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-500 dark:text-violet-200">
      {{ t('pelican.reasoningTitle') }}
      <span class="ml-2 text-xs font-normal text-violet-600 dark:text-violet-300">{{ thinking ? t('pelican.thinking') : t('pelican.reasoningHint') }}</span>
    </summary>
    <pre class="max-h-80 overflow-auto whitespace-pre-wrap break-words border-t border-violet-200/60 p-4 text-xs leading-relaxed text-gray-700 dark:border-violet-900/60 dark:text-gray-200">{{ text }}</pre>
  </details>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{ text: string; thinking?: boolean }>()
const { t } = useI18n()
</script>
