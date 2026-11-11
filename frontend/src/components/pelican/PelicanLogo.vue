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
        <stop offset="0" stop-color="#38bdf8" />
        <stop offset="1" stop-color="#2563eb" />
      </linearGradient>
      <linearGradient :id="billId" x1="14" y1="8" x2="29" y2="20" gradientUnits="userSpaceOnUse">
        <stop offset="0" stop-color="#fbbf24" />
        <stop offset="1" stop-color="#f97316" />
      </linearGradient>
    </defs>
    <!-- badge background -->
    <rect x="0.5" y="0.5" width="31" height="31" rx="9" :fill="`url(#${bgId})`" />
    <!-- water line -->
    <path d="M3 27.2c2.2-1.2 4.3-1.2 6.5 0s4.3 1.2 6.5 0 4.3-1.2 6.5 0 4.3 1.2 6.5 0" stroke="#bae6fd" stroke-width="1.3" stroke-linecap="round" opacity="0.8" />
    <!-- body -->
    <path d="M4.5 21.5c0-3.6 3-6 6.6-6 2.6 0 4.4 1.1 5.4 2.9.6 1.1.2 2.4-.9 3l-3.4 1.8c-1.6.8-3.4 1.1-5.2.8-1.5-.3-2.5-1.2-2.5-2.5z" fill="#ffffff" />
    <!-- wing -->
    <path d="M7.2 20.3c1.8.4 4 .2 6-.9" stroke="#93c5fd" stroke-width="1.2" stroke-linecap="round" />
    <!-- neck -->
    <path d="M11.6 16c-.9-1.8-1-3.6-.2-5.4" stroke="#ffffff" stroke-width="3.2" stroke-linecap="round" />
    <!-- head -->
    <circle cx="12.6" cy="9.4" r="3.6" fill="#ffffff" />
    <!-- bill + pouch -->
    <path d="M15.4 8.4 28.6 10.6c.5.1.5.8 0 .9L15.6 12.2z" :fill="`url(#${billId})`" />
    <path d="M15.6 12.2 28.4 11.6c-1.6 3.6-5.6 6.3-9.6 5.9-2-.2-3.2-2.4-3.2-5.3z" fill="#fdba74" />
    <!-- eye -->
    <circle cx="13.3" cy="8.6" r="0.95" fill="#0f172a" />
  </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ size?: number | string }>(), { size: 20 })
const uid = Math.random().toString(36).slice(2, 8)
const bgId = `pelican-bg-${uid}`
const billId = `pelican-bill-${uid}`
</script>
