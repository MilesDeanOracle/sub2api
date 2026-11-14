<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import PelicanArtifactPreview from './PelicanArtifactPreview.vue'
import { operationsApi, type OperationsResult } from '@/api/admin/operationsConsole'

const props = defineProps<{ result: OperationsResult; active?: boolean }>()
const { t } = useI18n()
const root = ref<HTMLElement | null>(null)
const visible = ref(false)
const html = ref<string | null>(null)
const failed = ref(false)
const loading = ref(false)
const groupNames = computed(() => props.result.groups.map(group => group.name).join(' / ') || t('operationsConsole.ungrouped'))
const generatedAt = computed(() => props.result.finished_at || props.result.created_at)
const generationTime = computed(() => new Date(generatedAt.value).toLocaleString(undefined, {
  month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false
}))
let observer: IntersectionObserver | undefined
const metrics = computed(() => [
  { key: 'ttft', value: props.result.ttft_ms == null ? t('operationsConsole.unavailable') : `${(props.result.ttft_ms / 1000).toFixed(2)} s`, hint: t('operationsConsole.ttftHint') },
  { key: 'tps', value: props.result.tps == null ? t('operationsConsole.unavailable') : props.result.tps.toFixed(1), hint: t('operationsConsole.tpsHint') },
  { key: 'outputTokens', value: props.result.output_tokens?.toLocaleString() ?? t('operationsConsole.unavailable'), hint: '' }
])
async function load() {
  if (loading.value || html.value || props.result.status !== 'succeeded') return
  loading.value = true; failed.value = false
  try { html.value = await operationsApi.content(props.result.id) } catch { failed.value = true } finally { loading.value = false }
}
watch(visible, value => { if (value) void load() })
watch(() => props.result.status, () => { if (visible.value) void load() })
onMounted(() => {
  if (!('IntersectionObserver' in window)) { visible.value = true; return }
  observer = new IntersectionObserver(entries => { visible.value = entries.some(e => e.isIntersecting) }, { rootMargin: '150px' })
  if (root.value) observer.observe(root.value)
})
onUnmounted(() => observer?.disconnect())
</script>

<template>
  <article ref="root" class="min-w-0 overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
    <header class="space-y-1 px-3 py-2">
      <div class="flex items-center justify-between gap-2">
        <div class="flex min-w-0 items-center gap-1">
          <p class="flex min-w-0 items-center gap-1.5 text-[11px] text-slate-500">
            <span class="min-w-0 truncate" :title="groupNames">{{ groupNames }}</span>
            <span aria-hidden="true">·</span>
            <time class="shrink-0 tabular-nums" :datetime="generatedAt" :title="new Date(generatedAt).toLocaleString()">{{ generationTime }}</time>
          </p>
          <HelpTooltip class="shrink-0" :aria-label="t('operationsConsole.generationParameters')">
            <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
              <dt>{{ t('operationsConsole.model') }}</dt><dd class="break-all">{{ result.model_id }}</dd>
              <template v-if="result.upstream_model_id && result.upstream_model_id !== result.model_id">
                <dt>{{ t('operationsConsole.upstreamModel') }}</dt><dd class="break-all">{{ result.upstream_model_id }}</dd>
              </template>
              <dt>{{ t('operationsConsole.effort') }}</dt><dd>{{ result.reasoning_effort || t('operationsConsole.defaultEffort') }}</dd>
              <dt>{{ t('operationsConsole.task') }}</dt><dd class="break-all">{{ result.task_name }}</dd>
            </dl>
          </HelpTooltip>
          <span v-if="active" class="h-1.5 w-1.5 shrink-0 rounded-full bg-primary-500" :title="t('operationsConsole.activeRound')" :aria-label="t('operationsConsole.activeRound')"></span>
        </div>
        <span class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium" :class="result.status === 'succeeded' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' : result.status === 'failed' ? 'bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300' : 'bg-slate-100 text-slate-600 dark:bg-dark-800 dark:text-slate-300'">{{ t(`operationsConsole.statuses.${result.status}`) }}</span>
      </div>
    </header>

    <div class="border-t border-slate-100 dark:border-dark-700">
      <template v-if="result.status === 'succeeded'">
        <PelicanArtifactPreview v-if="visible && html" :html="html" embedded fit-content :allow-new-tab="false" :title="`${result.account_name} · ${result.model_id}`" />
        <div v-else class="flex aspect-[16/10] items-center justify-center bg-slate-100 text-xs text-slate-500 dark:bg-dark-800">
          <div class="p-4 text-center">
            <p>{{ t(failed ? 'operationsConsole.previewFailed' : loading ? 'operationsConsole.loading' : 'operationsConsole.pendingPreview') }}</p>
            <Button v-if="failed" class="mt-3" variant="outline" @click="load">{{ t('operationsConsole.retry') }}</Button>
          </div>
        </div>
      </template>
      <div v-else class="flex aspect-[16/10] items-center justify-center bg-slate-50 p-4 dark:bg-dark-800">
        <p class="max-h-full overflow-y-auto break-words text-center text-xs" :class="result.status === 'failed' ? 'text-red-600 dark:text-red-300' : 'text-slate-500'">
          {{ result.error_message || result.error_code || t(`operationsConsole.statuses.${result.status}`) }}
        </p>
      </div>
    </div>

    <footer class="grid grid-cols-3 divide-x divide-slate-100 border-t border-slate-100 dark:divide-dark-700 dark:border-dark-700">
      <div v-for="metric in metrics" :key="metric.key" class="min-w-0 px-3 py-2" :title="metric.hint">
        <p class="truncate text-[10px] text-slate-500">{{ t(`operationsConsole.${metric.key}`) }}</p>
        <p class="mt-0.5 text-xs font-semibold tabular-nums">{{ metric.value }}</p>
      </div>
    </footer>
  </article>
</template>

<style scoped>
/* Inline artifacts act as previews so wheel events reach the account row.
   Fullscreen retains the artifact's own mouse and scroll interactions. */
article :deep(.pelican-preview:not(:fullscreen) .pelican-preview-frame) {
  pointer-events: none;
}
</style>

