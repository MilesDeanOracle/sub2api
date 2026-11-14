<template>
  <div ref="shellRef" class="pelican-preview" :class="{ 'pelican-preview-embedded': embedded }">
    <div class="pelican-preview-toolbar">
      <div v-if="!embedded" class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('pelican.play') }}
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="btn btn-secondary btn-sm" :disabled="!html" @click="replay">
          {{ t('pelican.replay') }}
        </button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="!html" @click="toggleFullscreen">
          {{ t('pelican.fullscreen') }}
        </button>
        <button v-if="allowNewTab" type="button" class="btn btn-secondary btn-sm" :disabled="!html" @click="openNewTab">
          {{ t('pelican.openNewTab') }}
        </button>
      </div>
    </div>

    <div v-if="prepared" ref="stageRef" class="pelican-preview-stage" :class="{ 'pelican-preview-fitted': fitContent }">
      <iframe
        :key="frameKey"
        :srcdoc="prepared"
        :title="title"
        :style="frameStyle"
        :scrolling="fitContent ? 'no' : undefined"
        class="pelican-preview-frame"
        sandbox="allow-scripts"
        referrerpolicy="no-referrer"
        loading="eager"
      />
    </div>
    <div v-else class="pelican-preview-empty">
      {{ t('pelican.noHtml') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'
import { prepareArtifactDocument } from '@/lib/pelican/artifact'

const props = withDefaults(
  defineProps<{
    html: string | null
    title?: string
    allowNewTab?: boolean
    embedded?: boolean
    fitContent?: boolean
  }>(),
  { title: 'Pelican animation', allowNewTab: true, embedded: false, fitContent: false }
)

const { t } = useI18n()
const shellRef = ref<HTMLElement | null>(null)
const stageRef = ref<HTMLElement | null>(null)
const stageSize = ref({ width: 0, height: 0 })
let resizeObserver: ResizeObserver | undefined
const frameKey = ref(0)
const scriptNonce = ref<string | undefined>()

const prepared = computed(() =>
  props.html ? prepareArtifactDocument(props.html, scriptNonce.value, props.fitContent) : ''
)

const frameStyle = computed<CSSProperties | undefined>(() => {
  if (!props.fitContent) return undefined
  const width = 1280, height = 800
  const scale = Math.min(stageSize.value.width / width, stageSize.value.height / height) || 0
  return { width: `${width}px`, height: `${height}px`, transform: `translate(-50%, -50%) scale(${scale})` }
})

function measureStage() {
  const stage = stageRef.value
  if (stage) stageSize.value = { width: stage.clientWidth, height: stage.clientHeight }
}

watch(stageRef, stage => {
  resizeObserver?.disconnect()
  if (stage) resizeObserver?.observe(stage)
  measureStage()
})

onMounted(() => {
  const script = document.querySelector<HTMLScriptElement>('script[nonce]')
  scriptNonce.value = script?.nonce || undefined
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(measureStage)
    if (stageRef.value) resizeObserver.observe(stageRef.value)
  }
  measureStage()
})

onUnmounted(() => resizeObserver?.disconnect())

function replay(): void {
  frameKey.value += 1
}

async function toggleFullscreen(): Promise<void> {
  const shell = shellRef.value
  if (!shell) {
    return
  }
  try {
    if (document.fullscreenElement) {
      await document.exitFullscreen()
      return
    }
    await shell.requestFullscreen()
  } catch {
    openNewTab()
  }
}

function openNewTab(): void {
  // Operations artifacts must remain inside the iframe sandbox, including
  // browsers where Fullscreen API is unavailable.
  if (!props.allowNewTab || !prepared.value) {
    return
  }
  const blob = new Blob([prepared.value], { type: 'text/html;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  window.open(url, '_blank', 'noopener')
  window.setTimeout(() => URL.revokeObjectURL(url), 60_000)
}
</script>

<style scoped>
.pelican-preview {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.pelican-preview-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

/* Wall cards need an edge-to-edge canvas; the existing standalone preview
   keeps its toolbar and frame. Controls overlay the canvas in embedded mode. */
.pelican-preview-embedded {
  position: relative;
  gap: 0;
}

.pelican-preview-embedded .pelican-preview-toolbar {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  z-index: 1;
  opacity: 0;
  transition: opacity 150ms;
}

.pelican-preview-embedded:hover .pelican-preview-toolbar,
.pelican-preview-embedded:focus-within .pelican-preview-toolbar {
  opacity: 1;
}

@media (hover: none) {
  .pelican-preview-embedded .pelican-preview-toolbar {
    opacity: 1;
  }
}

.pelican-preview-embedded .pelican-preview-stage {
  border: 0;
  border-radius: 0;
}

.pelican-preview-stage {
  position: relative;
  width: 100%;
  aspect-ratio: 16 / 10;
  max-height: 70vh;
  overflow: hidden;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.75rem;
  background: #0f172a;
}

:global(.dark) .pelican-preview-stage {
  border-color: rgb(51 65 85);
}

.pelican-preview-frame {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
  background: #ffffff;
}

.pelican-preview-fitted .pelican-preview-frame {
  position: absolute;
  top: 50%;
  left: 50%;
  max-width: none;
  transform-origin: center;
}

.pelican-preview-empty {
  display: flex;
  min-height: 12rem;
  align-items: center;
  justify-content: center;
  border: 1px dashed rgb(209 213 219);
  border-radius: 0.75rem;
  color: rgb(107 114 128);
  font-size: 0.875rem;
}

:global(.dark) .pelican-preview-empty {
  border-color: rgb(51 65 85);
  color: rgb(148 163 184);
}

.pelican-preview:fullscreen {
  padding: 1rem;
  background: #0b1220;
}

.pelican-preview:fullscreen .pelican-preview-stage {
  flex: 1;
  aspect-ratio: auto;
  max-height: none;
}

.pelican-preview:fullscreen .pelican-preview-toolbar {
  color: #e2e8f0;
}
</style>
