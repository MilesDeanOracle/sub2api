<template>
  <button
    v-if="quickEntry && (status?.visible === true || pendingDraw)"
    type="button"
    data-testid="lottery-entry"
    class="group relative inline-flex items-center gap-2 rounded-lg px-3 py-1.5 text-base font-medium text-amber-700 transition-colors hover:bg-amber-50/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-400 dark:text-amber-300 dark:hover:bg-amber-900/20 2xl:pointer-events-auto"
    aria-haspopup="dialog"
    @click="openModal"
  >
    <Icon name="gift" size="lg" />
    <span>{{ t('dashboard.activity.lottery.entry') }}</span>
    <span
      v-if="status?.admin_debug || (status?.remaining_draws ?? 0) > 0"
      data-testid="lottery-remaining-badge"
      class="absolute -right-1.5 -top-1.5 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold leading-none text-white shadow-sm ring-2 ring-white dark:ring-dark-900"
    >
      {{ status?.admin_debug ? '∞' : (status?.remaining_draws ?? 0) > 99 ? '99+' : status?.remaining_draws }}
    </span>
  </button>

  <BaseDialog
    :show="showModal && (status?.visible === true || !!pendingDraw)"
    :title="t('dashboard.activity.lottery.modalTitle')"
    width="normal"
    :close-on-click-outside="drawingCount === 0"
    @close="closeModal"
  >
    <div class="mystery-chest">
      <p v-if="!status" class="chest-loading">{{ error || t('common.loading') }}</p>
      <template v-else-if="viewMode === 'chest'">
        <div class="chest-card-shell">
          <header class="chest-hero">
            <h4 class="chest-hero-title">{{ t('dashboard.activity.lottery.modalTitle') }}</h4>
            <p class="chest-hero-sub">{{ t('dashboard.activity.lottery.heroSubtitle') }}</p>
            <p class="chest-rule-caption" :data-testid="status.admin_debug ? 'lottery-admin-debug' : undefined">
              {{
                status.admin_debug
                  ? t('dashboard.activity.lottery.adminDebugNotice')
                  : status.enabled
                    ? t('dashboard.activity.lottery.ruleHint', { yuan: formatYuan(status.unit_pay_yuan) })
                    : t('dashboard.activity.lottery.closedRemainingHint')
              }}
            </p>
          </header>

          <!-- Eight independent prizes surround the single-draw action. -->
          <div class="prize-panel">
            <div class="prize-grid" data-testid="lottery-nine-grid" :aria-busy="drawingCount !== 0">
              <div
                v-for="tier in status.tiers"
                :key="tier.position"
                data-testid="lottery-prize-card"
                class="prize-card"
                :class="{ 'is-hit': !opening && highlightedPositions.has(tier.position), 'is-spinning': opening && spinPosition === tier.position, 'is-inactive': tier.probability === 0 }"
                :style="{ gridArea: `tier-${tier.position}` }"
                :data-tier-position="tier.position"
              >
                <span class="prize-icon" :class="tier.reward_cents >= 1000 ? 'is-coin' : 'is-gift'" aria-hidden="true">
                  <!-- Highest configured prize: laurel coin emblem -->
                  <svg v-if="tier.reward_cents === Math.max(...status.tiers.map((item) => item.reward_cents))" viewBox="0 0 32 32" fill="none" stroke="currentColor" class="h-6 w-6">
                    <circle cx="16" cy="15" r="8.5" fill="currentColor" stroke="none" opacity="0.18" />
                    <circle cx="16" cy="15" r="7" stroke-width="1.8" />
                    <path
                      d="M7 19c-1.5-3-1-7 1.5-9.5M25 19c1.5-3 1-7-1.5-9.5M10 23c2 1.5 4 2 6 2s4-.5 6-2"
                      stroke-width="1.6"
                      stroke-linecap="round"
                    />
                    <path
                      d="M16 11.2v7.6M17.8 13c-.3-.6-1-.9-1.8-.9-1 0-1.7.5-1.7 1.2 0 .8.7 1.1 1.8 1.4 1.1.3 1.8.6 1.8 1.5 0 .8-.8 1.3-1.9 1.3-.9 0-1.6-.3-1.9-1"
                      stroke-width="1.5"
                      stroke-linecap="round"
                    />
                  </svg>
                  <!-- Larger prizes: solid coin emblem -->
                  <svg v-else-if="tier.reward_cents >= 1000" viewBox="0 0 32 32" fill="none" stroke="currentColor" class="h-6 w-6">
                    <circle cx="16" cy="16" r="9.5" stroke-width="2" />
                    <path
                      d="M16 11v10M18.3 13.3c-.4-.8-1.2-1.2-2.3-1.2-1.3 0-2.2.6-2.2 1.6 0 1 .9 1.4 2.3 1.8 1.4.4 2.3.8 2.3 1.9 0 1.1-1 1.7-2.4 1.7-1.2 0-2-.4-2.4-1.3"
                      stroke-width="1.8"
                      stroke-linecap="round"
                    />
                  </svg>
                  <!-- Standard prizes: gift ribbon emblem -->
                  <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" class="h-5 w-5">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M20 12v7a2 2 0 01-2 2H6a2 2 0 01-2-2v-7m16 0V9a2 2 0 00-2-2H6a2 2 0 00-2 2v3m16 0H4m8-5v14m0-14c-1.8 0-3-1.1-3-2.4S10.2 2.5 11.6 4L12 7zm0 0c1.8 0 3-1.1 3-2.4S13.8 2.5 12.4 4L12 7z"
                    />
                  </svg>
                </span>
                <strong class="prize-amount">${{ formatPrize(tier.reward) }}</strong>
                <span v-if="tier.probability === 0" class="prize-state">{{ t('dashboard.activity.lottery.inactivePrize') }}</span>
                <span
                  v-if="lastBatch.length > 1 && !opening && batchWinCounts.has(tier.position)"
                  class="prize-win-count"
                  :data-testid="`lottery-win-count-${tier.position}`"
                >x{{ batchWinCounts.get(tier.position) }}</span>
              </div>
              <button
                type="button"
                data-testid="lottery-draw-1"
                class="prize-center"
                :class="{ 'is-opening': opening }"
                :disabled="!canDrawOne"
                :aria-busy="drawingCount !== 0"
                @click="handleDraw(1)"
              >
                <Icon name="gift" size="lg" class="draw-icon" aria-hidden="true" />
                <strong>{{ drawingCount !== 0 ? t('common.loading') : t('dashboard.activity.lottery.drawOnce') }}</strong>
                <span>{{ status.admin_debug ? t('dashboard.activity.lottery.adminDebugDraws') : t('dashboard.activity.lottery.drawsLeft', { count: status.remaining_draws }) }}</span>
              </button>
            </div>
            <div ref="confettiLayer" class="confetti-layer" aria-hidden="true">
              <span v-for="piece in confettiPieces" :key="piece.id" class="confetti-piece" :style="piece.style"></span>
            </div>
          </div>

          <p class="grid-odds-hint">{{ t('dashboard.activity.lottery.gridOddsHint') }}<template v-if="status.tiers.some((tier) => tier.probability === 0)"> {{ t('dashboard.activity.lottery.legacyOddsHint') }}</template></p>
          <details class="chest-rules">
            <summary>{{ t('dashboard.activity.lottery.rulesTitle') }}</summary>
            <p>{{ t('dashboard.activity.lottery.rulesDetail', { yuan: formatYuan(status.unit_pay_yuan) }) }}</p>
            <ul><li v-for="tier in status.tiers" :key="tier.position">${{ formatPrize(tier.reward) }}<span v-if="tier.probability != null"> · {{ (tier.probability * 100).toFixed(4).replace(/0+$/, '').replace(/\.$/, '') }}%</span></li></ul>
          </details>

          <!-- 3-Column Stats Bar -->
          <div class="chest-stats">
            <div class="chest-stat-item">
              <strong data-testid="lottery-stat-remaining">{{ status.admin_debug ? '∞' : status.remaining_draws }}</strong>
              <span>{{ t('dashboard.activity.lottery.remainingDraws') }}</span>
            </div>
            <div class="chest-stat-divider" aria-hidden="true"></div>
            <div class="chest-stat-item">
              <strong data-testid="lottery-stat-total">${{ status.total_won.toFixed(2) }}</strong>
              <span>{{ t('dashboard.activity.lottery.totalWon') }}</span>
            </div>
            <div class="chest-stat-divider" aria-hidden="true"></div>
            <div class="chest-stat-item">
              <strong data-testid="lottery-stat-today">${{ status.today_won.toFixed(2) }}</strong>
              <span>{{ t('dashboard.activity.lottery.todayWon') }}</span>
            </div>
          </div>

          <!-- Activity Reward Wallet Strip (shared isolated wallet) -->
          <div v-if="status.wallet_balance_cents > 0" class="chest-wallet">
            <div>
              <span class="chest-wallet-label">{{ t('dashboard.activity.walletAvailable') }}</span>
              <strong class="chest-wallet-amount">${{ status.wallet_balance.toFixed(2) }}</strong>
            </div>
            <button
              type="button"
              data-testid="lottery-wallet-withdraw"
              class="chest-wallet-btn"
              :disabled="!canWithdraw"
              :aria-busy="withdrawing"
              @click="doWithdraw"
            >
              {{ withdrawing ? t('dashboard.activity.transferring') : t('dashboard.activity.transferAll',{amount:status.wallet_balance.toFixed(2)}) }}
            </button>
          </div>
          <p class="chest-wallet-help">{{ t('dashboard.activity.walletShared') }} {{ t('dashboard.activity.walletUse') }}</p>
          <p v-if="status.withdraw_blocked" class="chest-wallet-help" role="status">{{ t(walletTransferBlockKey(status.withdraw_block_reason)) }}</p>

          <!-- Batch action complements the central single draw. -->
          <div class="chest-actions">
            <button
              type="button"
              data-testid="lottery-draw-10"
              class="chest-btn chest-btn-ten"
              :disabled="!canDrawTen"
              :aria-busy="drawingCount === 10"
              @click="handleDraw(10)"
            >
              {{
                drawingCount === 10
                  ? t('common.loading')
                  : status.admin_debug || status.remaining_draws >= 10
                    ? t('dashboard.activity.lottery.drawTen')
                    : t('dashboard.activity.lottery.insufficientDraws')
              }}
            </button>
          </div>

          <p class="chest-ten-hint">
            <template v-if="status.admin_debug">{{ t('dashboard.activity.lottery.adminDebugDraws') }}</template>
            <template v-else-if="(status.ticket_debt ?? 0)>0">{{ t('dashboard.activity.lottery.ticketDebt',{count:status.ticket_debt}) }}</template>
            <template v-else-if="status.draw_blocked">{{ t('dashboard.activity.lottery.reviewBlocked') }}</template>
            <template v-else-if="status.pool_paused">{{ t('dashboard.activity.lottery.poolPaused') }}</template>
            <template v-else-if="status.remaining_draws >= 10 && status.ten_draw_paused">{{
              t('dashboard.activity.lottery.tenDrawPaused')
            }}</template>
            <template v-else-if="status.remaining_draws < 10">{{
              t('dashboard.activity.lottery.needMoreForTen', { count: Math.max(0, 10 - status.remaining_draws) })
            }}</template>
            <template v-else>{{ t('dashboard.activity.lottery.readyForTen') }}</template>
          </p>

          <p v-if="notice" class="chest-notice" role="status">{{ notice }}</p>
          <RouterLink v-if="status.enabled && !status.admin_debug && !status.pool_paused && !status.draw_blocked && status.remaining_draws < 1 && !pendingDraw" to="/purchase" data-testid="lottery-go-recharge" class="chest-history-link" @click="closeModal">{{ t('dashboard.activity.lottery.goRecharge') }}</RouterLink>

          <div class="chest-footer">
            <button
              type="button"
              data-testid="lottery-open-history"
              class="chest-history-link"
              @click="openHistory"
            >
              <Icon name="clipboard" size="sm" />
              <span>{{ t('dashboard.activity.lottery.myRecords') }}</span>
            </button>
          </div>
        </div>
      </template>

      <!-- Winning Records View -->
      <template v-else>
        <div class="chest-history-panel">
          <div class="chest-history-header">
            <button type="button" class="chest-back-btn" @click="viewMode = 'chest'">
              <Icon name="arrowLeft" size="sm" />
              <span>{{ t('dashboard.activity.lottery.backToChest') }}</span>
            </button>
            <span class="chest-history-total">{{ status.total_won > 0 ? `$${status.total_won.toFixed(2)}` : '' }}</span>
          </div>
          <p v-if="historyLoading" class="chest-loading">{{ t('common.loading') }}</p>
          <p v-else-if="historyItems.length === 0" class="chest-loading">
            {{ t('dashboard.activity.lottery.recordsEmpty') }}
          </p>
          <ul v-else role="list" class="chest-history-list">
            <li v-for="row in historyItems" :key="row.id" class="chest-history-row">
              <div>
                <span class="chest-history-tag">
                  {{
                    row.draw_count === 10
                      ? t('dashboard.activity.lottery.tenTag', { index: row.draw_index })
                      : t('dashboard.activity.lottery.singleTag')
                  }}
                </span>
                <time class="chest-history-time" :datetime="row.created_at">{{ formatTime(row.created_at) }}</time>
              </div>
              <strong class="chest-history-reward">{{ row.revoked_at ? t('dashboard.activity.lottery.revoked') : '+' }}${{ row.reward.toFixed(2) }}</strong>
            </li>
          </ul>
          <div v-if="historyTotalPages > 1" class="chest-pagination">
            <button
              type="button"
              class="chest-page-btn"
              :disabled="historyPage <= 1 || historyLoading"
              @click="loadHistory(historyPage - 1)"
            >
              {{ t('pagination.previous') }}
            </button>
            <span>{{ t('pagination.pageOf', { page: historyPage, total: historyTotalPages }) }}</span>
            <button
              type="button"
              class="chest-page-btn"
              :disabled="historyPage >= historyTotalPages || historyLoading"
              @click="loadHistory(historyPage + 1)"
            >
              {{ t('pagination.next') }}
            </button>
          </div>
        </div>
      </template>
          <div v-if="pendingDraw" class="chest-notice" role="status">
            <p>{{ t('dashboard.activity.lottery.pendingResult') }}</p>
            <button type="button" data-testid="lottery-draw-retry" class="chest-btn chest-btn-single" :disabled="drawingCount !== 0" @click="handleDraw(pendingDraw.count)">{{ t('dashboard.activity.lottery.recoverResult') }}</button>
          </div>
      <p v-if="error" class="chest-error" role="alert">{{ error }}</p>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  drawLottery,
  getLotteryHistory,
  getLotteryStatus,
  withdrawActivityWallet,
  type LotteryDrawItem,
  type LotteryStatus,
} from '@/api/activity'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { walletTransferBlockKey } from '@/utils/activityWallet'

const props = withDefaults(defineProps<{ quickEntry?: boolean }>(), { quickEntry: false })
const { t, locale } = useI18n()
const route = useRoute()
const auth = useAuthStore()

const status = ref<LotteryStatus | null>(null)
const showModal = ref(false)
const viewMode = ref<'chest' | 'history'>('chest')
interface ConfettiPiece {
  id: number
  style: Record<string, string>
}

const drawingCount = ref<0 | 1 | 10>(0)
const opening = ref(false)
const spinPosition = ref<number | null>(null)
let spinTimer: ReturnType<typeof setTimeout> | null = null
let spinResolve: (() => void) | null = null
let spinGeneration = 0
const confettiPieces = ref<ConfettiPiece[]>([])
const confettiLayer = ref<HTMLElement | null>(null)
let confettiTimer: ReturnType<typeof setTimeout> | null = null
let confettiGeneration = 0
const withdrawing = ref(false)
const error = ref('')
const notice = ref('')
const lastBatch = ref<LotteryDrawItem[]>([])
const pendingDraw = ref<{ key: string; count: 1 | 10 } | null>(null)
const pendingStorageKey = computed(() => `activity-lottery-pending:${auth.user?.id ?? 'session'}`)
function rememberPending(value: typeof pendingDraw.value) {
  if (value) sessionStorage.setItem(pendingStorageKey.value, JSON.stringify(value))
  else sessionStorage.removeItem(pendingStorageKey.value)
  pendingDraw.value = value
}
function restorePending() {
  pendingDraw.value = null
  try {
    const raw = sessionStorage.getItem(pendingStorageKey.value)
    if (!raw) return
    const value: unknown = JSON.parse(raw)
    if (value && typeof value === 'object' && 'key' in value && typeof value.key === 'string' && value.key.length <= 64 && value.key.length > 0 && 'count' in value && (value.count === 1 || value.count === 10)) pendingDraw.value = { key: value.key, count: value.count }
  } catch { /* The draw action reports storage failures before sending a request. */ }
}
restorePending()

const historyItems = ref<LotteryDrawItem[]>([])
const historyTotal = ref(0)
const historyPage = ref(1)
const historyPageSize = 15
const historyLoading = ref(false)
let suppressSelfSync = false

function notifyWalletPeers() {
  suppressSelfSync = true
  try {
    window.dispatchEvent(new CustomEvent('activity-wallet-updated'))
  } finally {
    suppressSelfSync = false
  }
}

const highlightedPositions = computed(() => new Set(lastBatch.value.map((item) => item.tier_position)))
const batchWinCounts = computed(() => {
  const counts = new Map<number, number>()
  for (const item of lastBatch.value) {
    counts.set(item.tier_position, (counts.get(item.tier_position) ?? 0) + 1)
  }
  return counts
})
const activePrizePositions = computed(() =>
  (status.value?.tiers ?? [])
    .filter((tier) => (tier.probability ?? 0) > 0)
    .map((tier) => tier.position)
    .sort((a, b) => a - b)
)

const CONFETTI_COLORS = ['#f59e0b', '#ef4444', '#10b981', '#3b82f6', '#8b5cf6', '#f97316', '#eab308']

function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function clearSpinTimer() {
  if (spinTimer !== null) {
    clearTimeout(spinTimer)
    spinTimer = null
  }
  spinGeneration++
  const finish = spinResolve
  spinResolve = null
  finish?.()
}

function resetSpin() {
  clearSpinTimer()
  spinPosition.value = null
}

function beginSpinLoop() {
  resetSpin()
  const positions = activePrizePositions.value
  if (!positions.length) return
  const generation = spinGeneration
  let index = 0
  spinPosition.value = positions[0]
  const delay = 35
  const tick = () => {
    if (generation !== spinGeneration) return
    index = (index + 1) % positions.length
    spinPosition.value = positions[index]
    spinTimer = setTimeout(tick, delay)
  }
  spinTimer = setTimeout(tick, delay)
}

// Timing profile for the landing spin: the first two rounds stay very fast,
// rounds 3-4 ramp down, and the final half round crawls before stopping.
function landingDelay(completedSteps: number, totalSteps: number, roundSize: number): number {
  const fastSteps = roundSize * 2
  const slowTail = Math.max(2, Math.floor(roundSize / 2))
  const rampSteps = Math.max(1, totalSteps - fastSteps - slowTail)
  if (completedSteps < fastSteps) return 35

  const rampIndex = completedSteps - fastSteps
  if (rampIndex < rampSteps) {
    const progress = (rampIndex + 1) / rampSteps
    return Math.round(40 + 110 * progress * progress)
  }

  const tailIndex = Math.min(slowTail - 1, rampIndex - rampSteps)
  const progress = (tailIndex + 1) / slowTail
  return Math.round(180 + 300 * progress * progress)
}

// Continue from the current highlight and land on the drawn tier with
// progressively longer steps, so the spin visibly slows to a stop.
function landSpin(targetPosition: number): Promise<void> {
  clearSpinTimer()
  const positions = activePrizePositions.value
  const targetIndex = positions.indexOf(targetPosition)
  if (targetIndex < 0) {
    spinPosition.value = null
    return Promise.resolve()
  }
  const generation = spinGeneration
  let currentIndex = positions.indexOf(spinPosition.value ?? positions[0])
  if (currentIndex < 0) currentIndex = 0
  const offset = (targetIndex - currentIndex + positions.length) % positions.length
  const steps = positions.length * 5 + offset
  let step = 0
  return new Promise((resolve) => {
    const finish = () => {
      if (spinResolve === finish) spinResolve = null
      resolve()
    }
    spinResolve = finish
    const tick = () => {
      if (generation !== spinGeneration) {
        finish()
        return
      }
      step++
      currentIndex = (currentIndex + 1) % positions.length
      spinPosition.value = positions[currentIndex]
      if (step >= steps) {
        spinPosition.value = targetPosition
        spinTimer = null
        finish()
        return
      }
      spinTimer = setTimeout(tick, landingDelay(step, steps, positions.length))
    }
    spinTimer = setTimeout(tick, 35)
  })
}

function spinTargetPosition(items: LotteryDrawItem[]): number | null {
  if (!items.length) return null
  return items.reduce((best, item) => (item.reward_cents > best.reward_cents ? item : best), items[0]).tier_position
}

function launchConfetti(position: number) {
  if (prefersReducedMotion()) return
  const layer = confettiLayer.value
  if (!layer) return
  const layerRect = layer.getBoundingClientRect()
  const card = layer.parentElement?.querySelector<HTMLElement>(`[data-tier-position="${position}"]`)
  const cardRect = card?.getBoundingClientRect()
  const originX = cardRect ? cardRect.left + cardRect.width / 2 - layerRect.left : layerRect.width / 2
  const originY = cardRect ? cardRect.top + cardRect.height / 2 - layerRect.top : layerRect.height / 2
  const generation = ++confettiGeneration
  confettiPieces.value = Array.from({ length: 42 }, (_, index) => {
    const angle = (Math.PI * 2 * index) / 42 + (Math.random() - 0.5) * 0.32
    const distance = 50 + Math.random() * 130
    const size = 4 + Math.random() * 5
    return {
      id: generation * 1000 + index,
      style: {
        left: `${originX.toFixed(1)}px`,
        top: `${originY.toFixed(1)}px`,
        width: `${size.toFixed(1)}px`,
        height: `${(size * (0.55 + Math.random())).toFixed(1)}px`,
        backgroundColor: CONFETTI_COLORS[index % CONFETTI_COLORS.length],
        borderRadius: index % 3 === 0 ? '50%' : '2px',
        '--dx': `${(Math.cos(angle) * distance).toFixed(1)}px`,
        '--dy': `${(Math.sin(angle) * distance + 28).toFixed(1)}px`,
        '--rot': `${Math.round((Math.random() - 0.5) * 720)}deg`,
        animationDelay: `${Math.round(Math.random() * 90)}ms`,
        animationDuration: `${720 + Math.round(Math.random() * 380)}ms`,
      },
    }
  })
  if (confettiTimer !== null) clearTimeout(confettiTimer)
  confettiTimer = setTimeout(() => {
    if (generation === confettiGeneration) confettiPieces.value = []
    confettiTimer = null
  }, 1800)
}

function clearConfetti() {
  confettiGeneration++
  if (confettiTimer !== null) {
    clearTimeout(confettiTimer)
    confettiTimer = null
  }
  confettiPieces.value = []
}

const historyTotalPages = computed(() => Math.max(1, Math.ceil(historyTotal.value / historyPageSize)))

const canDrawOne = computed(
  () =>
    !!status.value &&
    (status.value.admin_debug ||
      (status.value.remaining_draws >= 1 &&
        !status.value.draw_blocked &&
        !status.value.pool_paused)) &&
    drawingCount.value === 0 && !pendingDraw.value
)
const canDrawTen = computed(
  () =>
    !!status.value &&
    (status.value.admin_debug ||
      (status.value.remaining_draws >= 10 &&
        !status.value.draw_blocked &&
        !status.value.pool_paused &&
        !status.value.ten_draw_paused)) &&
    drawingCount.value === 0 && !pendingDraw.value
)
const canWithdraw = computed(
  () =>
    (status.value?.wallet_balance_cents ?? 0) > 0 &&
    !status.value?.withdraw_blocked &&
    !withdrawing.value &&
    drawingCount.value === 0
)

function formatPrize(reward: number): string {
  if (!Number.isFinite(reward)) return '0'
  return Number.isInteger(reward) ? String(reward) : reward.toFixed(2).replace(/0$/, '')
}

function formatYuan(yuan: number): string {
  if (!Number.isFinite(yuan)) return '10'
  return Number.isInteger(yuan) ? String(yuan) : yuan.toFixed(2)
}

function formatTime(iso: string): string {
  try {
    return new Intl.DateTimeFormat(locale.value, {
      timeZone: 'Asia/Shanghai',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(iso))
  } catch {
    return iso
  }
}

function makeBatchKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `lot-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

async function refresh() {
  const requestedUserID = auth.user?.id
  try {
    const result = await getLotteryStatus()
    if (requestedUserID !== auth.user?.id) return
    status.value = result
    if (!status.value.visible && !pendingDraw.value) {
      showModal.value = false
    }
  } catch {
    if (showModal.value) {
      error.value = t('dashboard.activity.loadFailed')
    }
  }
}

async function openModal() {
  showModal.value = true
  viewMode.value = 'chest'
  error.value = ''
  notice.value = ''
  await refresh()
}

function closeModal() {
  if (drawingCount.value !== 0) return
  showModal.value = false
  opening.value = false
}

async function handleDraw(count: 1 | 10) {
  if (drawingCount.value !== 0) return
  drawingCount.value = count
  opening.value = true
  error.value = ''
  notice.value = ''
  const unresolved = pendingDraw.value
  if (unresolved && unresolved.count !== count) { drawingCount.value = 0; opening.value = false; return }
  const key = unresolved?.key ?? makeBatchKey()
  const drawUserID = auth.user?.id
  const reducedMotion = prefersReducedMotion()
  clearConfetti()
  if (!reducedMotion) beginSpinLoop()
  try {
    rememberPending({ key, count })
    const res = await drawLottery(count, key)
    if (drawUserID !== auth.user?.id) return
    if (!reducedMotion) {
      const target = spinTargetPosition(res.items)
      if (target !== null) {
        await landSpin(target)
        launchConfetti(target)
      }
    }
    if (drawUserID !== auth.user?.id) return
    rememberPending(null)
    status.value = res.status
    lastBatch.value = res.items
    notifyWalletPeers()
  } catch (e) {
    if (drawUserID !== auth.user?.id) return
    const statusCode = (e as { response?: { status?: number } })?.response?.status
    if (statusCode && [400, 422].includes(statusCode)) {
      try { rememberPending(null) } catch { pendingDraw.value = null }
    }
    error.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed'))
    await refresh()
  } finally {
    opening.value = false
    drawingCount.value = 0
    resetSpin()
  }
}

async function doWithdraw() {
  if (!canWithdraw.value) return
  withdrawing.value = true
  error.value = ''
  notice.value = ''
  try {
    const res = await withdrawActivityWallet()
    notice.value = t('dashboard.activity.withdrawSuccess', { amount: (res.withdrawn ?? 0).toFixed(2) })
    if (status.value) { status.value.wallet_balance_cents=res.available_cents; status.value.wallet_balance=res.available }
    lastBatch.value = []
    notifyWalletPeers()
    await Promise.allSettled([refresh(), auth.refreshUser()])
  } catch (e) {
    error.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed'))
  } finally {
    withdrawing.value = false
  }
}

async function openHistory() {
  viewMode.value = 'history'
  await loadHistory(1)
}

async function loadHistory(page: number) {
  historyLoading.value = true
  try {
    const res = await getLotteryHistory(page, historyPageSize)
    historyItems.value = res.items
    historyTotal.value = res.total
    historyPage.value = res.page
  } catch (e) {
    error.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed'))
  } finally {
    historyLoading.value = false
  }
}

function onExternalSync() {
  if (!suppressSelfSync && !document.hidden) {
    void refresh()
  }
}

watch(() => auth.user?.id, () => {
  status.value = null
  showModal.value = false
  lastBatch.value = []
  historyItems.value = []
  error.value = ''
  resetSpin()
  clearConfetti()
  restorePending()
  void refresh()
})

watch(
  () => route.fullPath,
  () => {
    void refresh()
  }
)

function onLotteryRequested() { if (props.quickEntry) void openModal() }

onMounted(() => {
  window.addEventListener('activity-lottery-requested', onLotteryRequested)
  void refresh()
  window.addEventListener('focus', onExternalSync)
  document.addEventListener('visibilitychange', onExternalSync)
  window.addEventListener('activity-wallet-updated', onExternalSync)
  window.addEventListener('activity-lottery-updated', onExternalSync)
})

onUnmounted(() => {
  resetSpin()
  clearConfetti()
  window.removeEventListener('activity-lottery-requested', onLotteryRequested)
  window.removeEventListener('focus', onExternalSync)
  document.removeEventListener('visibilitychange', onExternalSync)
  window.removeEventListener('activity-wallet-updated', onExternalSync)
  window.removeEventListener('activity-lottery-updated', onExternalSync)
})
</script>

<style scoped>
.chest-wallet-help{margin:8px 0;font-size:12px;line-height:1.7;color:#64748b}
:global(.dark .mystery-chest .chest-wallet-help){color:#94a3b8}
.chest-rules { margin: 12px 0; font-size: 12px; line-height: 1.7; }
.chest-rules summary { cursor: pointer; min-height: 32px; }
.chest-rules ul { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 4px 16px; padding-top: 8px; }
@media (pointer: coarse) { .chest-btn, .chest-page-btn, .chest-history-link, .chest-wallet-btn, .chest-rules summary { min-height: 48px; } }
:global(.modal-content:has(.mystery-chest)) {
  max-width: 31rem;
  border-radius: 20px;
  overflow: hidden;
}
:global(.modal-content:has(.mystery-chest) .modal-header) {
  padding: 16px 22px 12px;
  border-bottom: none;
  background: #ffffff;
}
:global(.modal-content:has(.mystery-chest) .modal-title) {
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}
:global(.modal-content:has(.mystery-chest) .modal-body) {
  padding: 0 18px 18px;
}
:global(.modal-overlay:has(.mystery-chest)) {
  background: rgb(15 23 42 / 46%);
  backdrop-filter: blur(4px);
}

.mystery-chest {
  color: #1e293b;
  font-size: 13px;
}

.chest-card-shell {
  border: 1px solid #f1ece1;
  border-radius: 18px;
  padding: 20px 16px 14px;
  background: radial-gradient(circle at 50% 0%, #fff7e6 0%, #ffffff 42%);
}

.chest-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.chest-hero-title {
  font-size: 21px;
  font-weight: 700;
  line-height: 1.25;
  color: #0f172a;
}

.chest-hero-sub {
  margin-top: 4px;
  font-size: 12.5px;
  color: #64748b;
}

.chest-rule-caption {
  margin-top: 10px;
  font-size: 11.5px;
  color: #94a3b8;
}

.prize-panel {
  position: relative;
  margin-top: 14px;
  padding: 10px;
  border-radius: 14px;
  background: #f8fafc;
  border: 1px solid #eef2f6;
}

.prize-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-template-rows: repeat(3, minmax(90px, 1fr));
  grid-template-areas:
    "tier-1 tier-2 tier-3"
    "tier-8 draw tier-4"
    "tier-7 tier-6 tier-5";
  gap: 8px;
}

.prize-card {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-inline-size: 0;
  padding: 10px 6px;
  border-radius: 11px;
  background: #ffffff;
  border: 1px solid #edf2f7;
  transition: border-color 0.12s ease, background-color 0.12s ease, transform 0.12s ease, box-shadow 0.12s ease;
}

.prize-card.is-hit {
  border-color: #f59e0b;
  background: #fffbeb;
  box-shadow: inset 0 0 0 1px #fde68a;
}

.prize-card.is-spinning {
  border-color: #f59e0b;
  background: #fff7ed;
  box-shadow: 0 0 0 2px rgba(245, 158, 11, 0.4), 0 0 20px rgba(245, 158, 11, 0.3);
  transform: scale(1.04);
}

.confetti-layer {
  position: absolute;
  inset: 0;
  z-index: 3;
  overflow: visible;
  pointer-events: none;
}

.confetti-piece {
  position: absolute;
  display: block;
  opacity: 0;
  transform: translate3d(-50%, -50%, 0);
  animation-name: confetti-burst;
  animation-timing-function: cubic-bezier(0.16, 0.62, 0.36, 1);
  animation-fill-mode: forwards;
  will-change: transform, opacity;
}

@keyframes confetti-burst {
  0% {
    transform: translate3d(-50%, -50%, 0) rotate(0deg) scale(0.8);
    opacity: 1;
  }
  12% { opacity: 1; }
  100% {
    transform: translate3d(calc(-50% + var(--dx)), calc(-50% + var(--dy)), 0) rotate(var(--rot)) scale(0.85);
    opacity: 0;
  }
}

.prize-center {
  grid-area: draw;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-inline-size: 0;
  padding: 10px 4px;
  border: 1px solid #b45309;
  border-radius: 12px;
  background: linear-gradient(145deg, #fbbf24, #d97706);
  color: #451a03;
  text-align: center;
  overflow-wrap: anywhere;
  cursor: pointer;
}

.prize-center strong { font-size: 14px; line-height: 1.4; }
.prize-center span { font-size: 11px; line-height: 1.4; }
.prize-center:hover:not(:disabled) { background: #fbbf24; }
.prize-center:disabled { background: #f5ead7; border-color: #d6c2a6; color: #735630; cursor: not-allowed; }
.prize-center:focus-visible { outline: 2px solid #b45309; outline-offset: 3px; }
.prize-card.is-inactive { background: #f1f5f9; border-style: dashed; }
.prize-card.is-inactive .prize-icon { background: #e2e8f0; color: #64748b; }
.prize-state { font-size: 10px; line-height: 1.3; color: #64748b; }

.prize-win-count {
  position: absolute;
  top: 4px;
  right: 4px;
  min-inline-size: 22px;
  padding: 1px 6px;
  border-radius: 999px;
  background: #f59e0b;
  color: #ffffff;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.6;
  text-align: center;
  font-variant-numeric: tabular-nums;
  box-shadow: 0 1px 3px rgb(0 0 0 / 18%);
}
.grid-odds-hint { margin-top: 8px; font-size: 11px; line-height: 1.6; color: #64748b; text-align: center; }

@keyframes draw-pulse {
  50% { transform: scale(1.12) rotate(6deg); }
}

@media (prefers-reduced-motion: no-preference) {
  .prize-center.is-opening .draw-icon { animation: draw-pulse 0.8s ease-in-out 5; }
}

.prize-icon {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.prize-icon.is-gift {
  background: #dcfce7;
  color: #10b981;
}

.prize-icon.is-coin {
  background: #d1fae5;
  color: #059669;
}

.prize-amount {
  font-size: 13.5px;
  font-weight: 700;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}

.chest-stats {
  margin-top: 12px;
  display: grid;
  grid-template-columns: 1fr 1px 1fr 1px 1fr;
  align-items: center;
  padding: 11px 8px;
  border-radius: 13px;
  border: 1px solid #edf2f7;
  background: #ffffff;
}

.chest-stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.chest-stat-item strong {
  font-size: 18px;
  line-height: 1.2;
  font-weight: 700;
  color: #d97706;
  font-variant-numeric: tabular-nums;
}

.chest-stat-item span {
  font-size: 11.5px;
  color: #64748b;
}

.chest-stat-divider {
  height: 24px;
  background: #e2e8f0;
}

.chest-wallet {
  margin-top: 10px;
  padding: 8px 12px;
  border-radius: 11px;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.chest-wallet-label {
  display: block;
  font-size: 11px;
  color: #64748b;
}

.chest-wallet-amount {
  font-size: 14px;
  font-weight: 700;
  color: #059669;
  font-variant-numeric: tabular-nums;
}

.chest-wallet-btn {
  min-height: 32px;
  padding: 5px 12px;
  border-radius: 8px;
  background: #059669;
  color: #ffffff;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.chest-wallet-btn:hover:not(:disabled) {
  background: #047857;
}

.chest-wallet-btn:disabled {
  background: #cbd5e1;
  color: #64748b;
  cursor: not-allowed;
}

.chest-actions {
  margin-top: 12px;
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
}

.chest-btn {
  min-height: 44px;
  padding: 10px 14px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  transition: background-color 0.15s ease, opacity 0.15s ease;
  cursor: pointer;
}

.chest-btn:focus-visible,
.chest-wallet-btn:focus-visible,
.chest-history-link:focus-visible,
.chest-back-btn:focus-visible,
.chest-page-btn:focus-visible {
  outline: 2px solid #d97706;
  outline-offset: 2px;
}

.chest-btn-single {
  background: #fdf6ec;
  border: 1px solid #f5e6cf;
  color: #9a6b32;
}

.chest-btn-single:hover:not(:disabled) {
  background: #faecd8;
}

.chest-btn-single:disabled {
  background: #faf5ef;
  color: #b89b78;
  cursor: not-allowed;
}

.chest-btn-ten {
  background: linear-gradient(90deg, #e5a95c 0%, #dc9849 100%);
  color: #ffffff;
  border: 1px solid transparent;
}

.chest-btn-ten:hover:not(:disabled) {
  background: linear-gradient(90deg, #d99742 0%, #c98332 100%);
}

.chest-btn-ten:disabled {
  background: linear-gradient(90deg, #eac089 0%, #e4b578 100%);
  color: rgb(255 255 255 / 92%);
  cursor: not-allowed;
}

.chest-ten-hint {
  margin-top: 8px;
  text-align: center;
  font-size: 12px;
  color: #64748b;
}

.chest-notice {
  margin-top: 8px;
  text-align: center;
  font-size: 12px;
  font-weight: 500;
  color: #059669;
}

.chest-error {
  margin-top: 8px;
  text-align: center;
  font-size: 12px;
  color: #dc2626;
}

.chest-footer {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid #f1f5f9;
  display: flex;
  justify-content: center;
}

.chest-history-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border-radius: 8px;
  font-size: 12.5px;
  color: #475569;
  cursor: pointer;
}

.chest-history-link:hover {
  color: #0f172a;
  background: #f8fafc;
}

.chest-loading {
  padding: 32px 12px;
  text-align: center;
  color: #64748b;
}

.chest-history-panel {
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 14px;
}

.chest-history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 10px;
  border-bottom: 1px solid #f1f5f9;
}

.chest-back-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-radius: 8px;
  font-size: 12.5px;
  font-weight: 600;
  color: #334155;
  cursor: pointer;
}

.chest-back-btn:hover {
  background: #f1f5f9;
}

.chest-history-total {
  font-size: 13px;
  font-weight: 700;
  color: #d97706;
  font-variant-numeric: tabular-nums;
}

.chest-history-list {
  margin-top: 8px;
  max-height: 320px;
  overflow-y: auto;
}

.chest-history-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 9px 4px;
  border-bottom: 1px solid #f8fafc;
}

.chest-history-tag {
  display: inline-block;
  margin-right: 8px;
  padding: 2px 7px;
  border-radius: 6px;
  background: #fef3c7;
  color: #92400e;
  font-size: 11px;
  font-weight: 600;
}

.chest-history-time {
  font-size: 11.5px;
  color: #64748b;
  font-variant-numeric: tabular-nums;
}

.chest-history-reward {
  font-size: 13.5px;
  font-weight: 700;
  color: #059669;
  font-variant-numeric: tabular-nums;
}

.chest-pagination {
  margin-top: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: #64748b;
}

.chest-page-btn {
  padding: 5px 10px;
  border-radius: 8px;
  border: 1px solid #cbd5e1;
  font-size: 12px;
  color: #334155;
  cursor: pointer;
}

.chest-page-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

:global(.dark .modal-content:has(.mystery-chest) .modal-header) {
  background: #1e293b;
}
:global(.dark .modal-content:has(.mystery-chest) .modal-title) {
  color: #f8fafc;
}
:global(.dark .mystery-chest) {
  color: #e2e8f0;
}
:global(.dark .mystery-chest .chest-card-shell) {
  border-color: #334155;
  background: radial-gradient(circle at 50% 0%, #3b2a14 0%, #0f172a 48%);
}
:global(.dark .mystery-chest .chest-hero-title) {
  color: #f8fafc;
}
:global(.dark .mystery-chest .prize-panel) {
  background: #0f172a;
  border-color: #1e293b;
}
:global(.dark .mystery-chest .prize-card) {
  background: #1e293b;
  border-color: #334155;
}
:global(.dark .mystery-chest .prize-card.is-inactive) { background: #172033; }
:global(.dark .mystery-chest .prize-state) { color: #94a3b8; }
:global(.dark .mystery-chest .prize-center:disabled) { background: #292524; border-color: #57534e; color: #d6c2a6; }
:global(.dark .mystery-chest .prize-card.is-hit) {
  background: #422b13;
  border-color: #f59e0b;
}
:global(.dark .mystery-chest .prize-amount) {
  color: #f8fafc;
}
:global(.dark .mystery-chest .prize-icon.is-gift),
:global(.dark .mystery-chest .prize-icon.is-coin) {
  background: rgb(16 185 129 / 18%);
  color: #34d399;
}
:global(.dark .mystery-chest .chest-stats) {
  background: #1e293b;
  border-color: #334155;
}
:global(.dark .mystery-chest .chest-stat-item strong) {
  color: #fbbf24;
}
:global(.dark .mystery-chest .chest-stat-divider) {
  background: #334155;
}
:global(.dark .mystery-chest .chest-wallet) {
  background: #123529;
  border-color: #166534;
}
:global(.dark .mystery-chest .chest-wallet-amount) {
  color: #6ee7b7;
}
:global(.dark .mystery-chest .chest-btn-single) {
  background: #292524;
  border-color: #44403c;
  color: #fcd34d;
}
:global(.dark .mystery-chest .chest-btn-single:disabled) {
  background: #1f2937;
  border-color: #334155;
  color: #64748b;
}
:global(.dark .mystery-chest .chest-footer),
:global(.dark .mystery-chest .chest-history-header),
:global(.dark .mystery-chest .chest-history-row) {
  border-color: #334155;
}
:global(.dark .mystery-chest .chest-history-panel) {
  border-color: #334155;
}
:global(.dark .mystery-chest .chest-history-link),
:global(.dark .mystery-chest .chest-back-btn),
:global(.dark .mystery-chest .chest-page-btn) {
  color: #cbd5e1;
}
:global(.dark .mystery-chest .chest-history-link:hover),
:global(.dark .mystery-chest .chest-back-btn:hover) {
  background: #1e293b;
  color: #ffffff;
}

@media (max-width: 480px) {
  .chest-actions {
    grid-template-columns: 1fr;
  }
  .chest-wallet-btn { width: 100%; }
}
@media (pointer: coarse) { .mystery-chest .chest-wallet-btn { min-height: 48px; } }
</style>
