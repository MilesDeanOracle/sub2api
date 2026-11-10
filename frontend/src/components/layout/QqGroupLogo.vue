<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 1024 1024"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    role="img"
    aria-hidden="true"
    class="shrink-0"
  >
    <defs>
      <linearGradient :id="bgId" x1="0" y1="0" x2="1024" y2="1024" gradientUnits="userSpaceOnUse">
        <stop offset="0" stop-color="#4fc3ff" />
        <stop offset="1" stop-color="#0b8fe0" />
      </linearGradient>
    </defs>
    <!-- badge background (QQ blue) -->
    <rect x="16" y="16" width="992" height="992" rx="288" :fill="`url(#${bgId})`" />
    <!-- QQ penguin silhouette -->
    <path
      transform="translate(512 524) scale(0.74) translate(-512 -521)"
      fill="#ffffff"
      d="M824.8 613.2c-16-51.4-34.4-94.6-62.7-165.3C766.5 262.2 689.3 112 511.5 112 331.7 112 256.2 265.2 261 447.9c-28.4 70.8-46.7 113.7-62.7 165.3-34 109.5-23 154.8-14.6 155.8 18 2.2 70.1-82.4 70.1-82.4 0 49 25.2 112.9 79.8 159-26.4 8.1-85.7 29.9-71.6 53.8 11.4 19.3 196.2 12.3 249.5 6.3 53.3 6 238.1 13 249.5-6.3 14.1-23.8-45.3-45.7-71.6-53.8 54.6-46.2 79.8-110.1 79.8-159 0 0 52.1 84.6 70.1 82.4 8.5-1.1 19.5-46.4-14.5-155.8z"
    />
  </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ size?: number | string }>(), { size: 20 })
const bgId = `qq-group-bg-${Math.random().toString(36).slice(2, 8)}`
</script>
