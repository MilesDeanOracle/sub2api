<template>
  <div
    v-if="wallet?.visible"
    ref="rootRef"
    class="relative flex items-center"
    @keydown.esc="close(true)"
    @focusout="onFocusOut"
  >
    <button
      type="button"
      data-testid="header-gift-pill"
      :style="{'anchor-name': anchorName}"
      class="inline-flex items-center gap-1.5 rounded-xl border border-amber-200/80 bg-amber-50/90 px-3 py-1.5 text-sm font-semibold tabular-nums text-amber-700 transition-all hover:bg-amber-100/80 active:scale-[0.98] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-amber-600 dark:border-amber-800/60 dark:bg-amber-900/25 dark:text-amber-300 dark:hover:bg-amber-900/40"
      :aria-expanded="open"
      :aria-label="t('dashboard.activity.walletTitle') + ': ' + formatMoney(wallet.available)"
      :aria-controls="popoverID"
      @click="toggle"
    >
      <Icon name="gift" size="sm" class="text-amber-600 dark:text-amber-400" />
      <span class="flex flex-col items-start sm:flex-row sm:items-center sm:gap-1.5">
        <span class="text-xs sm:hidden">{{ t('dashboard.activity.walletShortTitle') }}</span>
        <span class="hidden sm:inline">{{ t('dashboard.activity.walletTitle') }}</span>
        <span class="whitespace-nowrap">{{ formatMoney(wallet.available) }}</span>
      </span>
      <span v-if="(wallet.reserved_cents ?? 0) > 0" class="h-2 w-2 rounded-full bg-emerald-500" :aria-label="t('dashboard.activity.giftReserved')"></span>
    </button>

    <div
      v-show="supportsPopover || open"
      ref="panelRef"
      :id="popoverID"
      :popover="supportsPopover ? 'auto' : undefined"
      :style="{'position-anchor': anchorName}"
      @toggle="onPopoverToggle"
      data-testid="header-gift-popover"
      class="gift-popover-wrap absolute right-0 top-full z-40 pt-2"
    >
      <div class="gift-popover rounded-xl border border-gray-200 bg-white p-4 text-xs shadow-lg dark:border-dark-700 dark:bg-dark-800">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.activity.walletTitle') }}</h3>
        <p class="mb-3 mt-1 leading-relaxed text-gray-500 dark:text-dark-300">{{ t('dashboard.activity.walletShared') }}</p>
        <div class="flex items-center justify-between">
          <span class="text-gray-500 dark:text-dark-400">{{ t('dashboard.activity.giftAvailable') }}</span>
          <span class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatMoney(wallet.available) }}</span>
        </div>
        <div class="mt-2.5 flex items-center justify-between">
          <span class="text-gray-500 dark:text-dark-400">{{ t('dashboard.activity.giftReserved') }}</span>
          <span class="font-medium tabular-nums text-amber-600 dark:text-amber-400">{{ formatMoney(wallet.reserved ?? 0) }}</span>
        </div>
        <div class="mt-2.5 border-t border-gray-100 pt-2.5 dark:border-dark-700">
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-dark-400">{{ t('dashboard.activity.giftTotal') }}</span>
            <span class="font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatMoney(wallet.total_gift ?? wallet.available + (wallet.reserved ?? 0)) }}</span>
          </div>
        </div>

        <p class="mt-3 leading-relaxed text-gray-600 dark:text-dark-300">{{ t('dashboard.activity.walletUse') }}</p>
        <p v-if="wallet.available_cents > 0" class="mt-1 leading-relaxed text-gray-500 dark:text-dark-400">{{ t('dashboard.activity.walletNext') }}</p>
        <p v-else-if="!notice && !(wallet.reserved_cents ?? 0)" class="mt-3 leading-relaxed text-gray-500 dark:text-dark-400">{{ t('dashboard.activity.walletEmpty') }}</p>

        <button
          v-if="wallet.available_cents > 0"
          type="button"
          data-testid="header-wallet-withdraw"
          class="mt-3 w-full rounded-lg bg-amber-600 px-3 py-1.5 text-xs font-semibold text-white transition-all hover:bg-amber-700 active:scale-[0.98] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-amber-600 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-amber-500 dark:text-slate-950 dark:hover:bg-amber-400"
          :disabled="withdrawing || wallet.withdraw_blocked"
          :aria-busy="withdrawing"
          @click="doWithdraw"
        >
          {{ withdrawing ? t('dashboard.activity.transferring') : t('dashboard.activity.transferAll', {amount: wallet.available.toFixed(2)}) }}
        </button>

        <button
          v-if="(wallet.reserved_cents ?? 0) > 0"
          type="button"
          class="mt-2 w-full text-center text-xs font-medium text-emerald-600 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-emerald-600 dark:text-emerald-400"
          @click="goClaim"
        >
          {{ t('dashboard.activity.goClaimReserved') }} →
        </button>
        <p v-if="(wallet.reserved_cents ?? 0) > 0" class="mt-1 leading-relaxed text-gray-500 dark:text-dark-400">{{ t('dashboard.activity.walletReservedHelp') }}</p>
        <button v-else-if="wallet.available_cents === 0 && !notice" type="button" class="mt-2 w-full text-left font-medium text-emerald-700 dark:text-emerald-300" @click="goClaim">{{ t('dashboard.activity.walletActivities') }} →</button>

        <p v-if="wallet.withdraw_blocked && (wallet.available_cents > 0 || (wallet.recovery_debt_cents ?? 0) > 0)" class="mt-2 text-[11px] leading-relaxed text-amber-700 dark:text-amber-300">
          {{ t(walletTransferBlockKey(wallet.withdraw_block_reason)) }}
        </p>
        <p v-if="notice" class="mt-2 text-[11px] font-medium text-emerald-600 dark:text-emerald-400" role="status">
          {{ notice }}
        </p>
        <p v-if="error" class="mt-2 text-[11px] text-red-600 dark:text-red-400" role="alert">
          {{ error }}
        </p>
        <details class="mt-3 border-t border-gray-100 pt-2 dark:border-dark-700">
          <summary class="cursor-pointer py-2 font-medium text-gray-700 dark:text-dark-200">{{ t('dashboard.activity.walletHistory') }}</summary>
          <p class="my-2 leading-relaxed text-gray-500 dark:text-dark-400">{{ t('dashboard.activity.walletEarned') }} {{ formatMoney(wallet.total_earned) }} · {{ t('dashboard.activity.walletTransferred') }} {{ formatMoney(wallet.total_withdrawn) }}</p>
          <ul v-if="wallet.ledger?.length" class="space-y-2">
            <li v-for="item in wallet.ledger" :key="item.id" class="flex items-start justify-between gap-3 border-t border-gray-100 pt-2 dark:border-dark-700">
              <span class="min-w-0 text-gray-600 dark:text-dark-300">{{ t(`dashboard.activity.ledgerKinds.${item.kind}`) }}<time :datetime="item.created_at" class="mt-1 block text-[11px] text-gray-500 dark:text-dark-400">{{ formatTime(item.created_at) }}</time></span>
              <strong class="whitespace-nowrap tabular-nums" :class="item.direction==='credit'?'text-emerald-700 dark:text-emerald-300':'text-gray-700 dark:text-dark-200'">{{ item.direction==='credit'?'+':'−' }}{{ formatMoney(item.amount_cents/100) }}</strong>
            </li>
          </ul>
          <p v-else class="py-2 text-gray-500 dark:text-dark-400">{{ t('dashboard.activity.walletLedgerEmpty') }}</p>
        </details>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { getActivityWallet, withdrawActivityWallet, type ActivityWalletSummary } from '@/api/activity'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { walletTransferBlockKey } from '@/utils/activityWallet'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const popoverID = useId()
const anchorName = `--reward-wallet-${popoverID.replace(/[^a-zA-Z0-9_-]/g, '')}`
const supportsPopover = typeof HTMLElement !== 'undefined' && 'popover' in HTMLElement.prototype
const rootRef = ref<HTMLElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const wallet = ref<ActivityWalletSummary | null>(null)
const open = ref(false)
const withdrawing = ref(false)
const notice = ref('')
const error = ref('')
let refreshVersion = 0
const formatTime = (value: string) => new Intl.DateTimeFormat(locale.value, { timeZone: 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(new Date(value))

function formatMoney(value: number) {
  return `$${(Number.isFinite(value) ? value : 0).toFixed(2)}`
}

async function refresh() {
  const version = ++refreshVersion
  try {
    const value = await getActivityWallet()
    if (version === refreshVersion && !withdrawing.value) wallet.value = value
  } catch {
    /* Non-blocking header indicator */
  }
}

function toggle() {
  if (open.value) {
    close()
  } else {
    open.value = true
    if (supportsPopover) panelRef.value?.showPopover()
    void refresh()
  }
}

function onPopoverToggle(event: Event) {
  open.value = (event as ToggleEvent).newState === 'open'
}

function onFocusOut(event: FocusEvent) {
  if (event.relatedTarget instanceof Node && !rootRef.value?.contains(event.relatedTarget)) close()
}

function close(restoreFocus = false) {
  open.value = false
  if (supportsPopover) panelRef.value?.hidePopover()
  if (restoreFocus) rootRef.value?.querySelector<HTMLButtonElement>('[data-testid=header-gift-pill]')?.focus()
}

function onClickOutside(event: MouseEvent) {
  if (rootRef.value && !rootRef.value.contains(event.target as Node)) {
    close()
  }
}

async function doWithdraw() {
  if (withdrawing.value || !wallet.value || wallet.value.available_cents <= 0 || wallet.value.withdraw_blocked) return
  ++refreshVersion
  withdrawing.value = true
  error.value = ''
  notice.value = ''
  try {
    const res = await withdrawActivityWallet()
    ++refreshVersion
    wallet.value = wallet.value ? { ...wallet.value, ...res } : res
    notice.value = t('dashboard.activity.withdrawSuccess', { amount: (res.withdrawn ?? 0).toFixed(2) })
    if (open.value && panelRef.value?.contains(document.activeElement)) rootRef.value?.querySelector<HTMLButtonElement>('[data-testid=header-gift-pill]')?.focus()
    window.dispatchEvent(new CustomEvent('activity-wallet-updated', { detail: { source: 'header-pill' } }))
    await Promise.allSettled([authStore.refreshUser()])
  } catch (e) {
    error.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed'))
  } finally {
    withdrawing.value = false
  }
}

function goClaim() {
  close()
  if (route.path === '/dashboard') window.dispatchEvent(new CustomEvent('activity-claim-requested'))
  else void router.push('/dashboard#consumption-rewards')
}

function onWalletUpdated(event: Event) {
  if ((event as CustomEvent<{ source?: string }>)?.detail?.source === 'header-pill') return
  void refresh()
}

onMounted(() => {
  void refresh()
  document.addEventListener('click', onClickOutside)
  window.addEventListener('activity-wallet-updated', onWalletUpdated)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onClickOutside)
  window.removeEventListener('activity-wallet-updated', onWalletUpdated)
})
</script>

<style scoped>
@media (pointer: coarse) { [data-testid="header-gift-pill"], .gift-popover :is(button,summary) { min-height: 48px; } }
.gift-popover-wrap{width:min(340px,calc(100vw - 24px))}
.gift-popover-wrap[popover]{position:fixed;inset:auto 12px 12px;margin:0 auto;padding:0;border:0;background:transparent;color:inherit}
@supports(position-area:block-end){.gift-popover-wrap[popover]{inset:auto;margin:8px 0 0;position-area:block-end span-inline-start;position-try-fallbacks:flip-block,flip-inline,flip-block flip-inline}}
@media(max-width:640px){.gift-popover-wrap[popover],.gift-popover-wrap:not([popover]){position:fixed;position-area:none;inset:auto 12px 12px;margin:0 auto;padding:0}}
.gift-popover {
  max-height:min(70dvh,40rem);
  overflow-y:auto;
  transform-origin: top right;
}
.gift-popover :is(button,summary):focus-visible{outline:2px solid #b45309;outline-offset:2px}
@media (prefers-reduced-motion: no-preference) {
  .gift-popover {
    animation: gift-pop-in 150ms ease-out both;
  }
}
@keyframes gift-pop-in {
  from {
    opacity: 0;
    transform: scale(0.95);
  }
}
</style>
