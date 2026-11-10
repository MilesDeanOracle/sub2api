<template>
  <fieldset :disabled="disabled" class="flex items-center gap-2">
    <legend class="sr-only">{{ t('feedback.rating') }}</legend>
    <label v-for="star in 5" :key="star" class="relative cursor-pointer rounded p-0.5 focus-within:ring-2 focus-within:ring-amber-500">
      <input
        type="radio" class="absolute inset-0 z-10 h-full w-full cursor-pointer opacity-0" :name="name" :value="star" :checked="modelValue === star"
        :aria-label="t('feedback.stars', { count: star })" required
        @change="$emit('update:modelValue', star)"
      >
      <svg class="h-8 w-8 transition-colors" :class="star <= modelValue ? 'text-amber-400' : 'text-gray-300 dark:text-dark-600'" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
        <path d="m12 2.5 2.94 5.96 6.58.96-4.76 4.64 1.12 6.55L12 17.52l-5.88 3.09 1.12-6.55L2.48 9.42l6.58-.96L12 2.5Z" />
      </svg>
    </label>
    <span class="ml-2 text-sm text-gray-500 dark:text-dark-400">{{ modelValue ? `${modelValue} / 5` : t('feedback.selectRating') }}</span>
  </fieldset>
</template>

<script setup lang="ts">
import { useId } from 'vue'
import { useI18n } from 'vue-i18n'
defineProps<{ modelValue: number; disabled?: boolean }>()
defineEmits<{ 'update:modelValue': [value: number] }>()
const { t } = useI18n()
const name = useId()
</script>
