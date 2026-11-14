<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { operationsApi, type OperationsStatus } from '@/api/admin/operationsConsole'
const { t } = useI18n()
const auth = useAuthStore()
const status = ref<OperationsStatus>({ running: 0, queued: 0, cleanup_failed: 0, max_workers: 4 })
let timer: ReturnType<typeof setInterval> | undefined
async function refresh() { try { status.value = await operationsApi.status() } catch { /* Each page reports its own fetch errors. */ } }
onMounted(() => { void refresh(); timer = setInterval(() => { if (!document.hidden) void refresh() }, 5000) })
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="min-h-screen bg-slate-50 text-slate-900 dark:bg-dark-950 dark:text-slate-100">
    <aside class="border-b border-slate-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-900 lg:fixed lg:inset-y-0 lg:w-60 lg:border-b-0 lg:border-r">
      <p class="text-xs font-semibold uppercase tracking-widest text-primary-600">SUB2API</p>
      <h1 class="mt-2 text-xl font-semibold">{{ t('operationsConsole.title') }}</h1>
      <p class="mt-2 text-xs text-slate-500">{{ t('operationsConsole.subtitle') }}</p>
      <nav class="mt-6 flex gap-2 lg:flex-col" :aria-label="t('operationsConsole.title')">
        <RouterLink v-for="item in [{ path: '/operations/pelican', key: 'wall' }, { path: '/operations/config', key: 'config' }]" :key="item.path" :to="item.path" class="rounded-lg px-4 py-3 text-sm hover:bg-slate-100 dark:hover:bg-dark-800" active-class="bg-primary-50 font-semibold text-primary-700 dark:bg-dark-800 dark:text-primary-300">{{ t(`operationsConsole.${item.key}`) }}</RouterLink>
      </nav>
      <RouterLink to="/admin/dashboard" class="mt-6 block text-sm text-slate-500 hover:text-primary-600">← {{ t('operationsConsole.back') }}</RouterLink>
      <p class="mt-6 truncate text-xs text-slate-400">{{ auth.user?.email }}</p>
    </aside>
    <main class="p-4 md:p-8 lg:ml-60">
      <div class="mb-6 flex flex-wrap items-center gap-3 rounded-xl border border-slate-200 bg-white px-4 py-3 text-sm dark:border-dark-700 dark:bg-dark-900" aria-live="polite">
        <span class="h-2 w-2 rounded-full bg-emerald-500"></span>
        <span>{{ t('operationsConsole.running') }} <strong>{{ status.running }} / 4</strong></span>
        <span class="text-slate-500">{{ t('operationsConsole.queued') }} {{ status.queued }}</span>
        <span v-if="status.cleanup_failed" class="text-red-600">{{ t('operationsConsole.cleanupFailed') }}: {{ status.cleanup_failed }}</span>
      </div>
      <RouterView />
    </main>
  </div>
</template>
