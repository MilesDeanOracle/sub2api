<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<{
  orientation?: 'horizontal' | 'vertical'
  decorative?: boolean
  class?: HTMLAttributes['class']
}>(), {
  orientation: 'horizontal',
  decorative: true
})

const role = computed(() => (props.decorative ? 'none' : 'separator'))
</script>

<template>
  <div
    :role="role"
    :class="cn(
      'shrink-0 bg-border',
      props.orientation === 'horizontal' ? 'h-[1px] w-full' : 'w-[1px] self-stretch',
      props.class
    )"
  />
</template>
