<template>
  <div ref="shellRef" class="pelican-preview">
    <div class="pelican-preview-toolbar">
      <div class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('pelican.play') }}
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button type="button" class="btn btn-secondary btn-sm" :disabled="!html" @click="replay">
          {{ t('pelican.replay') }}
        </button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="!html" @click="toggleFullscreen">
          {{ t('pelican.fullscreen') }}
        </button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="!html" @click="openNewTab">
          {{ t('pelican.openNewTab') }}
        </button>
      </div>
    </div>

    <div v-if="prepared" class="pelican-preview-stage">
      <iframe
        :key="frameKey"
        :srcdoc="prepared"
        :title="title"
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
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { prepareArtifactDocument } from '@/lib/pelican/artifact'

const props = withDefaults(
  defineProps<{
    html: string | null
    title?: string
  }>(),
  { title: 'Pelican animation' }
)

const { t } = useI18n()
const shellRef = ref<HTMLElement | null>(null)
const frameKey = ref(0)
const scriptNonce = ref<string | undefined>()

const prepared = computed(() =>
  props.html ? prepareArtifactDocument(props.html, scriptNonce.value) : ''
)

onMounted(() => {
  const script = document.querySelector<HTMLScriptElement>('script[nonce]')
  scriptNonce.value = script?.nonce || undefined
})

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
  if (!prepared.value) {
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
