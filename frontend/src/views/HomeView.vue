<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home: 沉浸式粒子首页 -->
  <div
    v-else
    data-testid="immersive-home"
    class="relative flex h-screen flex-col overflow-hidden"
    :class="isDark ? 'immersive-dark' : 'immersive-light'"
  >
    <!-- 粒子画布：漂浮 ↔ 聚合成 Logo，鼠标牵引涟漪 -->
    <canvas ref="particleCanvas" class="absolute inset-0 h-full w-full"></canvas>

    <!-- 顶部导航 -->
    <header class="relative z-20 flex items-center justify-between px-5 py-4 sm:px-8">
      <router-link :to="'/home'" class="flex min-w-0 items-center gap-3">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="h-9 w-9 shrink-0 rounded-xl object-contain"
        />
        <span class="min-w-0 truncate text-base font-semibold tracking-wide"
          :class="isDark ? 'text-white' : 'text-slate-900'">
          {{ siteName }}
        </span>
      </router-link>
      <div class="flex shrink-0 items-center gap-1.5 sm:gap-2">
        <LocaleSwitcher />
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="flex h-9 w-9 items-center justify-center rounded-lg transition-colors"
          :class="isDark ? 'text-white/60 hover:bg-white/10 hover:text-white' : 'text-slate-500 hover:bg-slate-900/5 hover:text-slate-900'"
          :title="t('home.viewDocs')"
        >
          <Icon name="book" size="md" />
        </a>
        <button
          class="flex h-9 w-9 items-center justify-center rounded-lg transition-colors"
          :class="isDark ? 'text-white/60 hover:bg-white/10 hover:text-white' : 'text-slate-500 hover:bg-slate-900/5 hover:text-slate-900'"
          :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          @click="toggleTheme"
        >
          <Icon v-if="isDark" name="sun" size="md" />
          <Icon v-else name="moon" size="md" />
        </button>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="ml-1 inline-flex h-9 items-center justify-center rounded-lg px-4 text-sm font-medium transition-all"
          :class="isDark
            ? 'bg-white text-slate-900 hover:bg-white/85'
            : 'bg-slate-900 text-white hover:bg-slate-800'"
        >
          {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
        </router-link>
      </div>
    </header>

    <!-- 主视觉：文字垂直居中，粒子 Logo 在其后方作大背景 -->
    <main class="relative z-10 flex min-h-0 flex-1 flex-col items-center justify-center px-6 pb-10 text-center">
      <p class="immersive-halo mb-5 text-xs font-medium uppercase tracking-[0.35em] sm:text-sm"
        :class="isDark ? 'text-white/40' : 'text-slate-500'">
        · {{ siteName }} · {{ t('home.immersion.tagline') }} ·
      </p>

      <!-- 轮播标语（强调词高亮） -->
      <Transition name="slogan" mode="out-in">
        <h1 :key="sloganIndex"
          class="immersive-halo max-w-4xl text-balance text-4xl font-bold leading-tight tracking-tight sm:text-5xl lg:text-6xl"
          :class="isDark ? 'text-white' : 'text-slate-900'"
        >
          <template v-for="(seg, i) in currentSlogan" :key="i">
            <span v-if="seg.accent" class="immersive-accent">{{ t(seg.k) }}</span>
            <span v-else>{{ t(seg.k) }}</span>
          </template>
        </h1>
      </Transition>

      <!-- CTA -->
      <div class="mt-10 flex flex-wrap items-center justify-center gap-4">
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="group inline-flex h-12 items-center gap-2 rounded-xl px-7 text-base font-semibold shadow-lg transition-all hover:-translate-y-0.5"
          :class="isDark
            ? 'bg-gradient-to-r from-white to-slate-200 text-slate-900 shadow-white/10 hover:shadow-white/20'
            : 'bg-gradient-to-r from-primary-600 to-primary-500 text-white shadow-primary-600/30 hover:shadow-primary-600/40'"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
          <span class="transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5">↗</span>
        </router-link>
        <router-link
          v-if="showModelPlazaEntry"
          to="/model-plaza"
          class="inline-flex h-12 items-center gap-2 rounded-xl border px-7 text-base font-medium transition-all hover:-translate-y-0.5"
          :class="isDark
            ? 'border-white/20 text-white/85 hover:border-white/40 hover:bg-white/5'
            : 'border-slate-900/15 text-slate-700 hover:border-slate-900/30 hover:bg-slate-900/5'"
        >
          {{ t('home.immersion.exploreModels') }}
          <span>＋</span>
        </router-link>
      </div>

      <!-- 支持的模型 -->
      <div class="mt-14 flex flex-wrap items-center justify-center gap-x-8 gap-y-4">
        <span class="text-xs uppercase tracking-widest"
          :class="isDark ? 'text-white/35' : 'text-slate-400'">
          {{ t('home.immersion.supportedModels') }}
        </span>
        <div class="flex items-center gap-7">
          <div v-for="p in supportedPlatforms" :key="p.platform"
            class="flex items-center gap-1.5 opacity-70 transition-opacity hover:opacity-100"
            :class="isDark ? 'text-white/80' : 'text-slate-700'">
            <PlatformIcon :platform="p.platform" size="sm" />
            <span class="text-sm font-medium">{{ p.label }}</span>
          </div>
        </div>
      </div>
    </main>

    <!-- 两侧竖排装饰文字 -->
    <span class="immersive-side left-6" :class="isDark ? 'text-white/15' : 'text-slate-900/15'">ONE GATEWAY · ALL MODELS</span>
    <span class="immersive-side right-6" :class="isDark ? 'text-white/15' : 'text-slate-900/15'">API FIRST · PAY AS YOU GO</span>

    <!-- 页脚 -->
    <footer class="relative z-20 flex flex-wrap items-center justify-center gap-x-4 gap-y-1 px-6 pb-5 text-xs"
      :class="isDark ? 'text-white/30' : 'text-slate-400'">
      <span>&copy; {{ currentYear }} {{ siteName }} · {{ t('home.footer.allRightsReserved') }}</span>
      <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer"
        class="transition-colors" :class="isDark ? 'hover:text-white/60' : 'hover:text-slate-600'">
        {{ t('home.docs') }}
      </a>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// ==================== 沉浸式首页：标语轮播 ====================
interface SloganSegment {
  k: string
  accent?: boolean
}

const slogans = computed<SloganSegment[][]>(() => [
  [
    { k: 'home.immersion.slogans.s1a', accent: true },
    { k: 'home.immersion.slogans.s1b' },
    { k: 'home.immersion.slogans.s1c', accent: true },
    { k: 'home.immersion.slogans.s1d' }
  ],
  [
    { k: 'home.immersion.slogans.s2a' },
    { k: 'home.immersion.slogans.s2b', accent: true },
    { k: 'home.immersion.slogans.s2c' },
    { k: 'home.immersion.slogans.s2d', accent: true },
    { k: 'home.immersion.slogans.s2e' }
  ],
  [
    { k: 'home.immersion.slogans.s3a' },
    { k: 'home.immersion.slogans.s3b', accent: true },
    { k: 'home.immersion.slogans.s3c' },
    { k: 'home.immersion.slogans.s3d', accent: true },
    { k: 'home.immersion.slogans.s3e' }
  ],
  [
    { k: 'home.immersion.slogans.s4a' },
    { k: 'home.immersion.slogans.s4b', accent: true },
    { k: 'home.immersion.slogans.s4c' },
    { k: 'home.immersion.slogans.s4d', accent: true },
    { k: 'home.immersion.slogans.s4e' }
  ]
])

const sloganIndex = ref(0)
const currentSlogan = computed(() => slogans.value[sloganIndex.value] ?? [])

const supportedPlatforms = [
  { platform: 'openai', label: 'ChatGPT' },
  { platform: 'anthropic', label: 'Claude' },
  { platform: 'gemini', label: 'Gemini' },
  { platform: 'grok', label: 'Grok' }
]

const SLOGAN_INTERVAL_MS = 5000
let sloganTimer: ReturnType<typeof setInterval> | undefined

const startSloganRotation = () => {
  stopSloganRotation()
  sloganTimer = setInterval(() => {
    sloganIndex.value = (sloganIndex.value + 1) % slogans.value.length
  }, SLOGAN_INTERVAL_MS)
}

const stopSloganRotation = () => {
  if (sloganTimer !== undefined) {
    clearInterval(sloganTimer)
    sloganTimer = undefined
  }
}

// ==================== 粒子引擎 ====================
// 漂浮星光 ↔ 聚合成 Logo ↔ 时空穿梭（以文字为中心的径向光轨）
// 鼠标移动产生排斥涟漪，点击扩散波纹。
const particleCanvas = ref<HTMLCanvasElement | null>(null)
const reducedMotion = typeof window !== 'undefined'
  && window.matchMedia('(prefers-reduced-motion: reduce)').matches

// 聚合 Logo：实心剪影（内部镂空填实），置于标语文字后方作大背景
const LOGO_CENTER_Y = 0.44
const LOGO_SIZE_RATIO = 0.75

function shuffled<T>(arr: T[]): T[] {
  const a = arr.slice()
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[a[i], a[j]] = [a[j], a[i]]
  }
  return a
}

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  size: number
  twinkle: number
  twinkleSpeed: number
  accent: boolean
  targetX: number
  targetY: number
}

interface Ripple {
  x: number
  y: number
  r: number
  strength: number
}

class ParticleEngine {
  private canvas: HTMLCanvasElement
  private ctx: CanvasRenderingContext2D
  private particles: Particle[] = []
  private ripples: Ripple[] = []
  // 实心剪影采样点（归一化到自身 bbox 0..1）与宽高比
  private logoSolid: { points: { x: number; y: number }[]; aspect: number } | null = null
  private width = 0
  private height = 0
  private dpr = 1
  private mouse = { x: 0, y: 0, active: false }
  private raf = 0
  private lastTime = 0
  private time = 0
  // 相位机：free（漂浮）→ assemble（聚合成 Logo）→ hold（保持）→ warp（时空穿梭）→ free
  private phase: 'free' | 'assemble' | 'hold' | 'warp' = 'free'
  private phaseElapsed = 0
  private phasePaused = false
  // 穿梭速度包络系数（0..1）：控制加速/匀速/减速与拖尾长度
  private warpK = 0
  private dark = true
  private destroyed = false
  private onPointerMove = (e: PointerEvent) => this.handlePointerMove(e)
  private onPointerLeave = () => { this.mouse.active = false }
  private onClick = (e: MouseEvent) => this.handleClick(e)
  private onResize = () => this.resize()

  constructor(canvas: HTMLCanvasElement, dark: boolean, logoSrc: string) {
    this.canvas = canvas
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('canvas 2d context unavailable')
    this.ctx = ctx
    this.dark = dark
    this.resize()
    this.spawn()
    this.sampleLogo(logoSrc)
    window.addEventListener('pointermove', this.onPointerMove, { passive: true })
    window.addEventListener('pointerleave', this.onPointerLeave)
    this.canvas.addEventListener('click', this.onClick)
    window.addEventListener('resize', this.onResize)
    this.lastTime = performance.now()
    this.raf = requestAnimationFrame(this.tick)
  }

  setTheme(dark: boolean) {
    this.dark = dark
  }

  setPhasePaused(paused: boolean) {
    this.phasePaused = paused
  }

  resampleLogo(logoSrc: string) {
    this.sampleLogo(logoSrc)
  }

  destroy() {
    this.destroyed = true
    cancelAnimationFrame(this.raf)
    window.removeEventListener('pointermove', this.onPointerMove)
    window.removeEventListener('pointerleave', this.onPointerLeave)
    this.canvas.removeEventListener('click', this.onClick)
    window.removeEventListener('resize', this.onResize)
  }

  private resize() {
    const rect = this.canvas.getBoundingClientRect()
    this.dpr = Math.min(window.devicePixelRatio || 1, 2)
    this.width = rect.width
    this.height = rect.height
    this.canvas.width = Math.round(rect.width * this.dpr)
    this.canvas.height = Math.round(rect.height * this.dpr)
    this.ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0)
    if (this.particles.length > 0) this.assignTargets()
  }

  private spawn() {
    // 粒子小而密：细沙般的星点铺满实心 Logo
    const count = Math.round(Math.min(1800, Math.max(700, (this.width * this.height) / 900)))
    this.particles = Array.from({ length: count }, () => ({
      x: Math.random() * this.width,
      y: Math.random() * this.height,
      vx: (Math.random() - 0.5) * 0.3,
      vy: (Math.random() - 0.5) * 0.3,
      size: Math.random() * 1.1 + 0.4,
      twinkle: Math.random() * Math.PI * 2,
      twinkleSpeed: Math.random() * 0.02 + 0.006,
      accent: Math.random() < 0.14,
      targetX: 0,
      targetY: 0
    }))
    this.assignTargets()
  }

  // 将 Logo 位图采样为归一化目标点（失败时退化为圆环）
  private sampleLogo(logoSrc: string) {
    const fallbackRing = () => {
      const points: { x: number; y: number }[] = []
      for (let i = 0; i < 220; i++) {
        const a = (i / 220) * Math.PI * 2
        points.push({ x: Math.cos(a) * 0.5 + 0.5, y: Math.sin(a) * 0.5 + 0.5 })
      }
      this.logoSolid = { points, aspect: 1 }
      this.assignTargets()
    }

    if (!logoSrc) {
      fallbackRing()
      return
    }

    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => {
      if (this.destroyed) return
      const size = 220
      const off = document.createElement('canvas')
      off.width = size
      off.height = size
      const octx = off.getContext('2d')
      if (!octx) return fallbackRing()
      // contain 绘制，保持比例
      const scale = Math.min(size / img.width, size / img.height)
      const dw = img.width * scale
      const dh = img.height * scale
      octx.drawImage(img, (size - dw) / 2, (size - dh) / 2, dw, dh)
      let data: Uint8ClampedArray
      try {
        data = octx.getImageData(0, 0, size, size).data
      } catch {
        return fallbackRing()
      }

      // 自适应采样：有透明通道用 alpha 阈值；全不透明（如白底 JPG 式 PNG）
      // 则取与四周边框背景色的亮度差，避免退化成实心方块
      let transparent = 0
      const borderLumas: number[] = []
      for (let y = 0; y < size; y++) {
        for (let x = 0; x < size; x++) {
          const i = (y * size + x) * 4
          if (data[i + 3] < 120) {
            transparent++
          } else if (x < 4 || y < 4 || x >= size - 4 || y >= size - 4) {
            borderLumas.push(0.299 * data[i] + 0.587 * data[i + 1] + 0.114 * data[i + 2])
          }
        }
      }
      const hasAlpha = transparent > size * size * 0.02
      const bgLuma = borderLumas.length
        ? borderLumas.reduce((s, v) => s + v, 0) / borderLumas.length
        : 255

      // 外部洪泛：从网格边缘的背景格出发标记连通外部区域；
      // 到不了的背景格属于 Logo 内部镂空（花纹），按实体处理，
      // 最终只保留外轮廓描边——实心/花纹填充会糊成一团
      const step = 2
      const gridSize = Math.ceil(size / step)
      const hitGrid = new Array<boolean>(gridSize * gridSize).fill(false)
      for (let gy = 0; gy < gridSize; gy++) {
        for (let gx = 0; gx < gridSize; gx++) {
          const x = gx * step
          const y = gy * step
          if (x >= size || y >= size) continue
          const i = (y * size + x) * 4
          let hit: boolean
          if (hasAlpha) {
            hit = data[i + 3] > 120
          } else {
            const luma = 0.299 * data[i] + 0.587 * data[i + 1] + 0.114 * data[i + 2]
            hit = Math.abs(luma - bgLuma) > 40
          }
          hitGrid[gy * gridSize + gx] = hit
        }
      }

      const outside = new Uint8Array(gridSize * gridSize)
      const floodStack: number[] = []
      const pushBackground = (gx: number, gy: number) => {
        const idx = gy * gridSize + gx
        if (!hitGrid[idx] && outside[idx] === 0) {
          outside[idx] = 1
          floodStack.push(idx)
        }
      }
      for (let i = 0; i < gridSize; i++) {
        pushBackground(i, 0)
        pushBackground(i, gridSize - 1)
        pushBackground(0, i)
        pushBackground(gridSize - 1, i)
      }
      while (floodStack.length > 0) {
        const idx = floodStack.pop()!
        const gx = idx % gridSize
        const gy = (idx - gx) / gridSize
        if (gx > 0) pushBackground(gx - 1, gy)
        if (gx < gridSize - 1) pushBackground(gx + 1, gy)
        if (gy > 0) pushBackground(gx, gy - 1)
        if (gy < gridSize - 1) pushBackground(gx, gy + 1)
      }

      // 实心剪影：outside==0 即实体（前景 + 内部镂空全部填实）。
      // 组合 Logo（图形+下方字标）时按行前景计数找空白带，裁掉字标只保留上部图形
      const rowFg = new Array<number>(gridSize).fill(0)
      for (let gy = 0; gy < gridSize; gy++) {
        for (let gx = 0; gx < gridSize; gx++) {
          if (hitGrid[gy * gridSize + gx]) rowFg[gy]++
        }
      }
      const lo = Math.floor(gridSize * 0.3)
      const hi = Math.ceil(gridSize * 0.78)
      let bestRun = 0
      let splitRow = -1
      let runStart = -1
      for (let y = lo; y <= hi; y++) {
        const empty = y < gridSize && rowFg[y] <= 1
        if (empty && runStart < 0) runStart = y
        if ((!empty || y === hi) && runStart >= 0) {
          const len = y - runStart
          if (len > bestRun) {
            bestRun = len
            splitRow = Math.floor((runStart + y) / 2)
          }
          runStart = -1
        }
      }
      const fgAbove = splitRow > 0 ? rowFg.slice(0, splitRow).reduce((s, v) => s + v, 0) : 0
      const fgBelow = splitRow >= 0 ? rowFg.slice(splitRow).reduce((s, v) => s + v, 0) : 0
      const cropY = bestRun >= 3 && fgAbove > gridSize * 2 && fgBelow > gridSize * 2
        ? splitRow
        : gridSize

      const solid: { x: number; y: number }[] = []
      let minX = gridSize
      let maxX = 0
      let minY = gridSize
      let maxY = 0
      for (let gy = 0; gy < cropY; gy++) {
        for (let gx = 0; gx < gridSize; gx++) {
          const idx = gy * gridSize + gx
          if (outside[idx] === 1) continue
          solid.push({ x: gx, y: gy })
          if (gx < minX) minX = gx
          if (gx > maxX) maxX = gx
          if (gy < minY) minY = gy
          if (gy > maxY) maxY = gy
        }
      }
      if (solid.length >= 20 && maxX > minX && maxY > minY) {
        const bw = maxX - minX + 1
        const bh = maxY - minY + 1
        let points = solid.map((p) => ({ x: (p.x - minX) / bw, y: (p.y - minY) / bh }))
        if (points.length > 1500) {
          const keep = 1500 / points.length
          points = points.filter(() => Math.random() < keep)
        }
        this.logoSolid = { points, aspect: bw / bh }
      } else {
        fallbackRing()
      }
      this.assignTargets()
    }
    img.onerror = () => fallbackRing()
    img.src = logoSrc
  }

  private assignTargets() {
    const particles = this.particles
    const shape = this.logoSolid
    if (particles.length === 0 || !shape || shape.points.length === 0) return

    // 实心 Logo 置于标语文字后方：以短边为基准放大，超宽时按宽收敛
    let h = Math.min(this.width, this.height) * LOGO_SIZE_RATIO
    let w = h * shape.aspect
    if (w > this.width * 0.85) {
      w = this.width * 0.85
      h = w / shape.aspect
    }
    const cx = this.width / 2
    const cy = this.height * LOGO_CENTER_Y
    const pool = shuffled(shape.points)
    particles.forEach((p, i) => {
      const pt = pool[i % pool.length]
      p.targetX = cx + (pt.x - 0.5) * w
      p.targetY = cy + (pt.y - 0.5) * h
    })
  }

  private handlePointerMove(e: PointerEvent) {
    const rect = this.canvas.getBoundingClientRect()
    this.mouse.x = e.clientX - rect.left
    this.mouse.y = e.clientY - rect.top
    this.mouse.active = true
    // 快速移动时生成小涟漪，产生"牵引"感
    const speed = Math.hypot(e.movementX ?? 0, e.movementY ?? 0)
    if (speed > 26 && this.ripples.length < 14) {
      this.ripples.push({ x: this.mouse.x, y: this.mouse.y, r: 0, strength: Math.min(speed / 60, 1.2) })
    }
  }

  private handleClick(e: MouseEvent) {
    const rect = this.canvas.getBoundingClientRect()
    this.ripples.push({ x: e.clientX - rect.left, y: e.clientY - rect.top, r: 0, strength: 2.2 })
  }

  private tick = (now: number) => {
    if (this.destroyed) return
    const dt = Math.min(now - this.lastTime, 50)
    this.lastTime = now
    this.time += dt
    this.updatePhase(dt)
    this.updateParticles(dt)
    this.draw()
    this.raf = requestAnimationFrame(this.tick)
  }

  private updatePhase(dt: number) {
    if (this.phasePaused || reducedMotion) return
    this.phaseElapsed += dt
    if (this.phase === 'free' && this.phaseElapsed > 7000) {
      this.phase = 'assemble'
      this.phaseElapsed = 0
    } else if (this.phase === 'assemble' && this.phaseElapsed > 3000) {
      this.phase = 'hold'
      this.phaseElapsed = 0
    } else if (this.phase === 'hold' && this.phaseElapsed > 5000) {
      // 穿越时空：以文字为中心，全体粒子低速起跳（后续按包络缓加速）
      this.phase = 'warp'
      this.phaseElapsed = 0
      this.warpK = 0
      const cx = this.width / 2
      const cy = this.height * LOGO_CENTER_Y
      for (const p of this.particles) {
        const dx = p.x - cx
        const dy = p.y - cy
        const d = Math.hypot(dx, dy) || 1
        const v = 0.4 + Math.random() * 0.5
        p.vx = (dx / d) * v
        p.vy = (dy / d) * v
      }
    } else if (this.phase === 'warp' && this.phaseElapsed > 10000) {
      this.phase = 'free'
      this.phaseElapsed = 0
      this.warpK = 0
      // 缓柔收速（粒子本已低速爬行），交给漂浮随机游走自然接管
      for (const p of this.particles) {
        p.vx *= 0.3
        p.vy *= 0.3
      }
    }
  }

  private updateParticles(dt: number) {
    const f = dt / 16.67
    const mouseR = 130
    const warpCx = this.width / 2
    const warpCy = this.height * LOGO_CENTER_Y
    // 穿梭速度包络：0-3s 平滑缓起，3-8s 匀速满速，8-10s 平滑缓收至 0
    // （smoothstep 两端零斜率，避免加速/收尾处出现折角）
    let warpCap = Infinity
    if (this.phase === 'warp') {
      const t = this.phaseElapsed
      const smoothstep = (u: number) => u * u * (3 - 2 * u)
      if (t < 3000) {
        this.warpK = 0.08 + 0.92 * smoothstep(t / 3000)
      } else if (t < 8000) {
        this.warpK = 1
      } else {
        this.warpK = 1 - smoothstep(Math.min((t - 8000) / 2000, 1))
      }
      warpCap = Math.max(6.5 * this.warpK, 0.15)
    }
    for (const p of this.particles) {
      if (this.phase === 'free') {
        // 漂浮：缓慢随机游走
        p.vx += (Math.random() - 0.5) * 0.02 * f
        p.vy += (Math.random() - 0.5) * 0.02 * f
        p.vx *= 0.985
        p.vy *= 0.985
      } else if (this.phase === 'warp') {
        // 时空穿梭：以文字为中心持续径向加速，速度上限随包络变化
        const dx = p.x - warpCx
        const dy = p.y - warpCy
        const d = Math.hypot(dx, dy) || 1
        p.vx += (dx / d) * 0.045 * this.warpK * f
        p.vy += (dy / d) * 0.045 * this.warpK * f
        const sp = Math.hypot(p.vx, p.vy)
        if (sp > warpCap) {
          p.vx *= warpCap / sp
          p.vy *= warpCap / sp
        }
      } else {
        // 聚合/保持：向 Logo 目标点弹簧牵引
        const k = this.phase === 'assemble' ? 0.028 : 0.012
        p.vx += (p.targetX - p.x) * k * f
        p.vy += (p.targetY - p.y) * k * f
        p.vx *= 0.88
        p.vy *= 0.88
      }

      // 鼠标排斥（涟漪扰动）
      if (this.mouse.active) {
        const dx = p.x - this.mouse.x
        const dy = p.y - this.mouse.y
        const d = Math.hypot(dx, dy)
        if (d < mouseR && d > 0.01) {
          const force = (1 - d / mouseR) * 0.9
          p.vx += (dx / d) * force * f
          p.vy += (dy / d) * force * f
        }
      }

      // 扩散波纹：波前经过时给径向冲量
      for (const r of this.ripples) {
        const dx = p.x - r.x
        const dy = p.y - r.y
        const d = Math.hypot(dx, dy)
        if (Math.abs(d - r.r) < 46 && d > 0.01) {
          const force = (1 - r.r / 560) * r.strength * 0.5
          if (force > 0) {
            p.vx += (dx / d) * force * f
            p.vy += (dy / d) * force * f
          }
        }
      }

      p.x += p.vx * f
      p.y += p.vy * f
      p.twinkle += p.twinkleSpeed * f

      // 越界处理：漂浮相位环绕；穿梭相位在中心重生（持续隧道流）；其余钳制
      if (this.phase === 'free') {
        if (p.x < -10) p.x = this.width + 10
        if (p.x > this.width + 10) p.x = -10
        if (p.y < -10) p.y = this.height + 10
        if (p.y > this.height + 10) p.y = -10
      } else if (this.phase === 'warp') {
        if (p.x < -30 || p.x > this.width + 30 || p.y < -30 || p.y > this.height + 30) {
          const a = Math.random() * Math.PI * 2
          const r = Math.random() * 36 + 6
          p.x = warpCx + Math.cos(a) * r
          p.y = warpCy + Math.sin(a) * r
          const v = (0.9 + Math.random() * 1.4) * Math.max(this.warpK, 0.08)
          p.vx = Math.cos(a) * v
          p.vy = Math.sin(a) * v
        }
      } else {
        p.x = Math.max(-10, Math.min(this.width + 10, p.x))
        p.y = Math.max(-10, Math.min(this.height + 10, p.y))
      }
    }

    for (const r of this.ripples) {
      r.r += 5.4 * (dt / 16.67)
    }
    this.ripples = this.ripples.filter((r) => r.r < 560)
  }

  private draw() {
    if (this.phase === 'warp') {
      // 穿梭拖尾：半透明擦除旧帧（destination-out 保持画布透明）；
      // 满速时擦得稍慢（拖尾长），起止阶段擦得快（接近粒子点）
      const fade = 0.5 - 0.15 * this.warpK
      this.ctx.globalCompositeOperation = 'destination-out'
      this.ctx.fillStyle = `rgba(0, 0, 0, ${fade})`
      this.ctx.fillRect(0, 0, this.width, this.height)
      this.ctx.globalCompositeOperation = 'source-over'
    } else {
      this.ctx.clearRect(0, 0, this.width, this.height)
    }
    for (const p of this.particles) {
      let alpha = 0.35 + Math.sin(p.twinkle) * 0.3 + 0.2
      // 穿梭期间整体透明度压到 70%
      if (this.phase === 'warp') alpha = Math.min(alpha, 0.7)
      const color = p.accent
        ? this.dark
          ? `rgba(251, 191, 36, ${Math.min(alpha, 0.95)})`
          : `rgba(234, 88, 12, ${Math.min(alpha, 0.85)})`
        : this.dark
          ? `rgba(226, 232, 240, ${Math.min(alpha * 0.8, 0.85)})`
          : `rgba(30, 41, 59, ${Math.min(alpha * 0.55, 0.6)})`
      if (this.phase === 'warp') {
        // 拖尾长度随速度包络变化：慢速退化为粒子点，满速为利落短星轨
        const m = 2.2 * this.warpK
        const tailX = p.x - p.vx * m
        const tailY = p.y - p.vy * m
        if (Math.hypot(p.x - tailX, p.y - tailY) < 1) {
          this.ctx.fillStyle = color
          this.ctx.beginPath()
          this.ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2)
          this.ctx.fill()
        } else {
          this.ctx.strokeStyle = color
          this.ctx.lineWidth = Math.max(p.size * 0.8, 0.5)
          this.ctx.beginPath()
          this.ctx.moveTo(tailX, tailY)
          this.ctx.lineTo(p.x, p.y)
          this.ctx.stroke()
        }
      } else {
        this.ctx.fillStyle = color
        this.ctx.beginPath()
        this.ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2)
        this.ctx.fill()
      }
    }
  }
}

let engine: ParticleEngine | null = null

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
  engine?.setTheme(isDark.value)
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

watch(siteLogo, (src) => {
  engine?.resampleLogo(src)
})

onMounted(async () => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  // 标语轮播（减少动效偏好下不自动轮播）
  if (!reducedMotion) {
    startSloganRotation()
  }

  // 粒子引擎（仅默认首页分支存在 canvas）
  await nextTick()
  if (particleCanvas.value) {
    try {
      engine = new ParticleEngine(particleCanvas.value, isDark.value, siteLogo.value || '/logo.svg')
    } catch (e) {
      console.error('Failed to init particle engine:', e)
    }
  }
})

onUnmounted(() => {
  stopSloganRotation()
  engine?.destroy()
  engine = null
})
</script>

<style scoped>
.immersive-dark {
  background:
    radial-gradient(ellipse 80% 60% at 50% 38%, #101a30 0%, #0a0f1d 55%, #060a13 100%);
}

.immersive-light {
  background:
    radial-gradient(ellipse 80% 60% at 50% 38%, #ffffff 0%, #f1f5fa 55%, #e8edf5 100%);
}

/* 强调词：琥珀色 + 微光 */
.immersive-accent {
  color: #fbbf24;
  text-shadow: 0 0 24px rgba(251, 191, 36, 0.35);
}

:global(.light) .immersive-accent,
.immersive-light .immersive-accent {
  color: #ea580c;
  text-shadow: 0 0 24px rgba(234, 88, 12, 0.2);
}

/* 标语切换动画 */
.slogan-enter-active,
.slogan-leave-active {
  transition: opacity 0.45s ease, transform 0.45s ease, filter 0.45s ease;
}

.slogan-enter-from {
  opacity: 0;
  transform: translateY(14px);
  filter: blur(6px);
}

.slogan-leave-to {
  opacity: 0;
  transform: translateY(-14px);
  filter: blur(6px);
}

/* 文字柔光晕：粒子 Logo 置于文字后方时保证可读性 */
.immersive-halo {
  text-shadow:
    0 0 18px rgba(6, 10, 19, 0.9),
    0 0 36px rgba(6, 10, 19, 0.7);
}

.immersive-light .immersive-halo {
  text-shadow:
    0 0 18px rgba(255, 255, 255, 0.92),
    0 0 36px rgba(255, 255, 255, 0.75);
}

/* 两侧竖排装饰文字 */
.immersive-side {
  position: absolute;
  top: 50%;
  z-index: 10;
  writing-mode: vertical-rl;
  font-size: 10px;
  letter-spacing: 0.5em;
  font-weight: 600;
  pointer-events: none;
  user-select: none;
}

@media (max-width: 1023px) {
  .immersive-side {
    display: none;
  }
}
</style>
