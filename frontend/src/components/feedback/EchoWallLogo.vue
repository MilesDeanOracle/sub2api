<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 32 32"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    role="img"
    aria-hidden="true"
    class="shrink-0"
  >
    <defs>
      <linearGradient :id="bgId" x1="0" y1="0" x2="32" y2="32" gradientUnits="userSpaceOnUse">
        <stop offset="0" stop-color="#a78bfa" />
        <stop offset="1" stop-color="#db2777" />
      </linearGradient>
    </defs>
    <!-- badge background -->
    <rect x="0.5" y="0.5" width="31" height="31" rx="9" :fill="`url(#${bgId})`" />
    <!-- speech bubble -->
    <path d="M5 10.5C5 8 7 6 9.5 6h8C20 6 22 8 22 10.5v5c0 2.5-2 4.5-4.5 4.5H12l-4.6 3.6c-.5.4-1.2 0-1.1-.6l.6-3.3C5.8 19 5 17.4 5 15.5z" fill="#ffffff" />
    <!-- voice bars -->
    <path d="M10 11.6v2.8M13.5 9.8v6.4M17 11.6v2.8" stroke="#a21caf" stroke-width="1.9" stroke-linecap="round" />
    <!-- echo arcs -->
    <path d="M24.4 9.6a6.6 6.6 0 0 1 0 7.8" stroke="#ffffff" stroke-width="1.8" stroke-linecap="round" />
    <path d="M27 7.4a10.2 10.2 0 0 1 0 12.2" stroke="#ffffff" stroke-width="1.6" stroke-linecap="round" opacity="0.6" />
  </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ size?: number | string }>(), { size: 20 })
const bgId = `echo-wall-bg-${Math.random().toString(36).slice(2, 8)}`
</script>
