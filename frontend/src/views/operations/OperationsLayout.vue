<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import Icon from '@/components/icons/Icon.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import { operationsApi, type OperationsStatus } from '@/api/admin/operationsConsole'

const { t } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const status = ref<OperationsStatus>({ running: 0, queued: 0, cleanup_failed: 0, max_workers: 4 })
const nav = [
  { path: '/operations/pelican', key: 'wall', icon: 'grid', hint: 'retention' },
  { path: '/operations/config', key: 'config', icon: 'cog', hint: 'pool' },
  { path: '/operations/group-cache', key: 'groupCache', icon: 'database', hint: 'groupCacheHint' }
] as const
const current = computed(() => nav.find(item => route.path.startsWith(item.path)) ?? nav[0]!)
const slots = computed(() => Array.from({ length: status.value.max_workers || 4 }, (_, i) => i < status.value.running))
let timer: ReturnType<typeof setInterval> | undefined
async function refresh() { try { status.value = await operationsApi.status() } catch { /* Each page reports its own fetch errors. */ } }
onMounted(() => { void refresh(); timer = setInterval(() => { if (!document.hidden) void refresh() }, 5000) })
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="min-h-screen bg-slate-50/70 text-slate-900 dark:bg-dark-950 dark:text-slate-100">
    <aside class="flex flex-col gap-4 bg-white px-4 py-4 shadow-[0_1px_0_0_rgba(15,23,42,0.05)] dark:bg-dark-900 dark:shadow-none lg:fixed lg:inset-y-0 lg:w-56 lg:gap-6 lg:px-3 lg:py-6 lg:shadow-[1px_0_0_0_rgba(15,23,42,0.05)]">
      <div class="flex items-center justify-between gap-3 lg:block lg:px-3">
        <div class="flex items-center gap-2.5">
          <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-dark-800 dark:text-primary-300"><Icon name="beaker" size="sm" /></span>
          <div class="min-w-0">
            <h1 class="truncate text-base font-semibold leading-tight">{{ t('operationsConsole.title') }}</h1>
            <p class="hidden truncate text-xs text-slate-400 lg:block">{{ t('operationsConsole.subtitle') }}</p>
          </div>
        </div>
        <RouterLink to="/admin/dashboard" class="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-slate-500 transition-colors hover:bg-slate-100 hover:text-primary-600 dark:hover:bg-dark-800 lg:hidden">
          <Icon name="arrowLeft" size="xs" />{{ t('operationsConsole.back') }}
        </RouterLink>
      </div>

      <nav class="flex gap-1 lg:flex-col" :aria-label="t('operationsConsole.title')">
        <RouterLink
          v-for="item in nav"
          :key="item.path"
          :to="item.path"
          class="flex flex-1 items-center justify-center gap-2.5 rounded-lg px-3 py-2.5 text-sm text-slate-600 transition-colors hover:bg-slate-50 dark:text-slate-300 dark:hover:bg-dark-800 lg:flex-none lg:justify-start"
          active-class="!bg-primary-50 font-medium !text-primary-700 dark:!bg-dark-800 dark:!text-primary-300"
        >
          <Icon :name="item.icon" size="sm" />
          <span>{{ t(`operationsConsole.${item.key}`) }}</span>
        </RouterLink>
      </nav>

      <div class="mt-auto hidden space-y-3 px-3 lg:block">
        <RouterLink to="/admin/dashboard" class="flex items-center gap-1.5 text-sm text-slate-500 transition-colors hover:text-primary-600">
          <Icon name="arrowLeft" size="xs" />{{ t('operationsConsole.back') }}
        </RouterLink>
        <p class="truncate text-xs text-slate-400" :title="auth.user?.email">{{ auth.user?.email }}</p>
      </div>
    </aside>

    <main class="p-4 md:p-6 lg:ml-56">
      <div class="mx-auto max-w-[1600px]">
        <header class="mb-4 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
          <div class="flex min-w-0 items-center">
            <h2 class="truncate text-lg font-semibold tracking-tight">{{ t(`operationsConsole.${current.key}`) }}</h2>
            <HelpTooltip :content="t(`operationsConsole.${current.hint}`)" width-class="w-72" />
          </div>
          <div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-slate-500" aria-live="polite">
            <span class="flex items-center gap-2 rounded-full bg-white px-3 py-1.5 shadow-sm dark:bg-dark-900">
              <span class="flex items-center gap-1" aria-hidden="true">
                <span v-for="(busy, index) in slots" :key="index" class="h-1.5 w-1.5 rounded-full transition-colors" :class="busy ? 'bg-emerald-500' : 'bg-slate-200 dark:bg-dark-700'"></span>
              </span>
              <span>{{ t('operationsConsole.running') }} <strong class="font-semibold text-slate-700 dark:text-slate-200">{{ status.running }} / {{ status.max_workers || 4 }}</strong></span>
            </span>
            <span v-if="status.queued" class="flex items-center gap-1"><Icon name="clock" size="xs" />{{ t('operationsConsole.queued') }} {{ status.queued }}</span>
            <span v-if="status.cleanup_failed" class="flex items-center gap-1 text-red-600 dark:text-red-400"><Icon name="exclamationTriangle" size="xs" />{{ t('operationsConsole.cleanupFailed') }}: {{ status.cleanup_failed }}</span>
          </div>
        </header>
        <RouterView />
      </div>
    </main>
  </div>
</template>