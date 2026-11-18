<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
defineProps<{ text: string; label: string }>()
const { locale } = useI18n()
const id = useId()
const open = ref(false)
const trigger = ref<HTMLButtonElement>()
const bubble = ref<HTMLElement>()
const position = ref({left:'0px',top:'0px'})
let hideTimer: ReturnType<typeof setTimeout> | undefined
function keepOpen() { clearTimeout(hideTimer) }
function dismiss() { keepOpen(); open.value = false }
function hideLater() {
  keepOpen()
  hideTimer = setTimeout(() => {
    if (document.activeElement !== trigger.value) open.value = false
  }, 180)
}
function updatePosition() {
  if (!open.value || !trigger.value || !bubble.value) return
  const anchor = trigger.value.getBoundingClientRect()
  if (anchor.bottom < 0 || anchor.top > window.innerHeight) { dismiss(); return }
  const bounds = bubble.value.getBoundingClientRect()
  const gap = 8, edge = 12
  const below = window.innerHeight - anchor.bottom - gap - edge
  const above = anchor.top - gap - edge
  const top = below >= bounds.height || below >= above ? anchor.bottom + gap : anchor.top - gap - bounds.height
  position.value = {
    left: `${Math.max(edge, Math.min(anchor.left, window.innerWidth - bounds.width - edge))}px`,
    top: `${Math.max(edge, Math.min(top, window.innerHeight - bounds.height - edge))}px`,
  }
}
async function reveal() { keepOpen(); open.value = true; await nextTick(); updatePosition() }
function onKeydown(event:KeyboardEvent) { if (event.key === 'Escape') dismiss() }
function onOutside(event:PointerEvent) {
  if (event.target instanceof Node && !trigger.value?.contains(event.target) && !bubble.value?.contains(event.target)) dismiss()
}
onMounted(() => {
  window.addEventListener('resize',updatePosition)
  window.addEventListener('scroll',updatePosition,true)
  document.addEventListener('keydown',onKeydown)
  document.addEventListener('pointerdown',onOutside)
})
onUnmounted(() => {
  keepOpen()
  window.removeEventListener('resize',updatePosition)
  window.removeEventListener('scroll',updatePosition,true)
  document.removeEventListener('keydown',onKeydown)
  document.removeEventListener('pointerdown',onOutside)
})
</script>
<template>
  <span class="reward-tip" @mouseenter="reveal" @mouseleave="hideLater">
    <button ref="trigger" type="button" :aria-label="label + (locale.startsWith('zh') ? '说明' : ' help')" :aria-expanded="open" :aria-controls="id" @focus="reveal" @blur="hideLater" @click="reveal">?</button>
    <Teleport to="body"><span v-if="open" :id="id" ref="bubble" role="tooltip" class="reward-tip-bubble" :style="position" @mouseenter="keepOpen" @mouseleave="hideLater">{{ text }}</span></Teleport>
  </span>
</template>
<style scoped>
.reward-tip{display:inline-flex;vertical-align:middle;margin-inline-start:4px;flex-shrink:0}.reward-tip button{display:flex;align-items:center;justify-content:center;width:28px;height:28px;border:0;border-radius:50%;background:transparent;color:#647787;font-size:11px;cursor:help;flex-shrink:0}.reward-tip button::before{content:'';position:absolute;width:16px;height:16px;border:1px solid #869ca8;border-radius:50%}.reward-tip button:focus-visible{outline:2px solid #087f73;outline-offset:2px}.reward-tip-bubble{position:fixed;z-index:99999;display:block;width:max-content;max-width:min(320px,calc(100vw - 24px));max-height:calc(100dvh - 24px);overflow:auto;overflow-wrap:anywhere;padding:12px 14px;border-radius:10px;background:#193c43;color:#fff;font-size:12px;line-height:1.8;box-shadow:0 6px 24px #0f23282b}
@media(pointer:coarse){.reward-tip button{width:44px;height:44px}}
</style>
