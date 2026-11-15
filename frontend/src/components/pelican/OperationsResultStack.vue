<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import OperationsResultCard from './OperationsResultCard.vue'
import type { OperationsResult } from '@/api/admin/operationsConsole'

// Activate cards after the first expansion, then keep their cached HTML and
// iframe instances mounted so folding never reloads an already viewed result.
const props = defineProps<{
  accountName: string
  results: OperationsResult[]
  loading: boolean
  loaded: boolean
  error: boolean
}>()
const emit = defineEmits<{ expand: []; reveal: [] }>()
const { t } = useI18n()
const root = ref<HTMLElement | null>(null)
const open = ref(false)
const ready = ref(false)
const revealed = ref(false)
const slots = computed(() => props.loaded ? Math.max(1, props.results.length) : 3)
let revealTimer: ReturnType<typeof setTimeout> | undefined

function reveal() {
  clearTimeout(revealTimer)
  if (!open.value || ready.value) return
  ready.value = true
  revealed.value = true
  if (!props.loaded && !props.loading) emit('reveal')
}
function expand() {
  if (open.value) return
  open.value = true
  emit('expand')
  if (props.loaded) { ready.value = true; revealed.value = true; return }
  // Fallback also covers reduced-motion preferences and interrupted transitions.
  revealTimer = setTimeout(reveal, 300)
}
function collapse() {
  if (document.fullscreenElement && root.value?.contains(document.fullscreenElement)) return
  clearTimeout(revealTimer)
  open.value = false
  ready.value = false
}
function onTransitionEnd(event: TransitionEvent) {
  if (event.propertyName === 'width') reveal()
}
function onFocusOut(event: FocusEvent) {
  if (!(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node | null)) collapse()
}
onUnmounted(() => clearTimeout(revealTimer))
</script>

<template>
  <div
    ref="root"
    class="operations-result-stack"
    :class="{ 'is-open': open }"
    :style="{ '--stack-count': slots }"
    role="group"
    :aria-label="t('operationsConsole.stackLabel', { name: accountName })"
    :aria-busy="loading"
    @mouseenter="expand"
    @mouseleave="collapse"
    @focusin="expand"
    @focusout="onFocusOut"
    @keydown.esc.stop="collapse"
    @transitionend.self="onTransitionEnd"
  >
    <div v-for="slot in slots" :key="slot" class="operations-stack-card" :style="{ '--stack-index': slot - 1 }" :inert="!open || !ready">
      <OperationsResultCard
        v-if="revealed && results[slot - 1]"
        class="h-full"
        :preview-enabled="open"
        retain-preview
        :result="results[slot - 1]!"
        :active="results[slot - 1]!.active_round && !['queued', 'running', 'uploading'].includes(results[slot - 1]!.status)"
      />
      <div v-else class="relative flex h-full flex-col items-center justify-center gap-3 overflow-hidden rounded-xl border border-slate-200 bg-white text-xs text-slate-400 dark:border-dark-700 dark:bg-dark-900">
        <div v-if="!open" class="operations-stack-back" aria-hidden="true">
          <div class="operations-stack-back-heading"><span></span><span></span></div>
          <div class="operations-stack-back-preview"></div>
          <div class="operations-stack-back-footer"><span></span><span></span><span></span></div>
        </div>
        <template v-else>
          <Icon v-if="loading || !ready" name="refresh" size="sm" class="animate-spin" />
          <span v-if="error">{{ t('operationsConsole.stackLoadFailed') }}</span>
          <span v-else-if="loaded && ready">{{ t('operationsConsole.noMoreResults') }}</span>
          <span v-else>{{ t('operationsConsole.loading') }}</span>
          <button v-if="ready && error" type="button" class="rounded-md px-3 py-1 text-primary-600 hover:bg-primary-50 dark:text-primary-300 dark:hover:bg-dark-700" @click="emit('reveal')">{{ t('operationsConsole.retry') }}</button>
        </template>
      </div>
    </div>
    <button
      type="button"
      class="operations-stack-trigger"
      :class="{ 'is-hidden': open }"
      :aria-label="t('operationsConsole.stackHint')"
      :aria-expanded="open"
      :title="t('operationsConsole.stackHint')"
      @click="expand"
    ><Icon name="chevronRight" size="sm" /><span>{{ slots }}</span></button>
  </div>
</template>

<style scoped>
.operations-result-stack {
  position: relative;
  isolation: isolate;
  z-index: 0;
  flex: 0 0 auto;
  align-self: stretch;
  width: calc(var(--operations-card-width) * 0.36);
  margin-left: -1rem;
  min-height: calc(var(--operations-card-width) * 0.625 + 4.75rem);
  overflow: visible;
  border-radius: 0.75rem;
  transition: width 260ms cubic-bezier(0.22, 1, 0.36, 1), margin-left 260ms cubic-bezier(0.22, 1, 0.36, 1);
}
.operations-result-stack.is-open {
  width: calc(var(--operations-card-width) * var(--stack-count) + (var(--stack-count) - 1) * 1rem);
  margin-left: 0;
}
.operations-result-stack:focus-within {
  outline: 2px solid rgb(100 116 139);
  outline-offset: -2px;
}
.operations-stack-card {
  position: absolute;
  inset-block: 0;
  left: calc(var(--operations-card-width) * -1);
  z-index: calc(3 - var(--stack-index));
  width: var(--operations-card-width);
  border-radius: 0.75rem;
  box-shadow: 0 5px 14px rgb(15 23 42 / 0.1);
  opacity: calc(1 - (var(--stack-index) + 1) * 0.14);
  transform-origin: left center;
  transform: translateX(calc((var(--stack-index) + 1) * var(--operations-card-width) * 0.19)) scale(calc(1 - (var(--stack-index) + 1) * 0.07));
  transition: left 260ms cubic-bezier(0.22, 1, 0.36, 1), transform 260ms cubic-bezier(0.22, 1, 0.36, 1), opacity 260ms ease;
}
.is-open .operations-stack-card {
  left: 0;
  opacity: 1;
  transform: translateX(calc(var(--stack-index) * (var(--operations-card-width) + 1rem)));
}
.operations-stack-back {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  padding: 0.75rem;
  gap: 0.75rem;
}
.operations-stack-back-heading {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}
.operations-stack-back-heading span,
.operations-stack-back-footer span {
  height: 0.375rem;
  flex: 1;
  border-radius: 999px;
  background: rgb(148 163 184 / 0.2);
}
.operations-stack-back-preview {
  flex: 1;
  border-radius: 0.375rem;
  background: linear-gradient(145deg, rgb(148 163 184 / 0.08), rgb(148 163 184 / 0.25));
}
.operations-stack-back-footer {
  display: flex;
  gap: 1rem;
  padding-block: 0.5rem;
}
.operations-stack-trigger {
  position: absolute;
  z-index: 4;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: flex-end;
  gap: 0.375rem;
  padding: 1.25rem 0.5rem;
  color: rgb(100 116 139);
  font-size: 0.75rem;
}
.operations-stack-trigger:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: -3px;
  border-radius: 0.75rem;
}
.operations-stack-trigger.is-hidden {
  opacity: 0;
  pointer-events: none;
}
@media (prefers-reduced-motion: reduce) {
  .operations-result-stack, .operations-stack-card { transition: none; }
}
</style>
