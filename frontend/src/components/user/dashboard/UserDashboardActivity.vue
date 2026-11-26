<template>
  <button
    v-if="quickEntry && status?.checkin_visible === true && status?.checkin_enabled === true"
    type="button"
    data-testid="checkin-entry"
    class="group inline-flex items-center gap-2 rounded-lg px-3 py-1.5 text-base font-medium text-emerald-700 transition-colors hover:bg-emerald-50/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400 dark:text-emerald-300 dark:hover:bg-emerald-900/20 2xl:pointer-events-auto"
    aria-haspopup="dialog"
    @click="openCheckin"
  >
    <Icon name="calendar" size="lg" />
    <span>{{ t('dashboard.activity.checkinEntry') }}</span>
  </button>
  <section v-if="!quickEntry && (status || error)" class="border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800 sm:p-6" aria-labelledby="activity-title">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 id="activity-title" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('dashboard.activity.title') }}</h2>
        <p v-if="status" class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ status.eligible ? t('dashboard.activity.started') : t('dashboard.activity.start') }}</p>
      </div>
      <button v-if="status && !status.eligible" class="btn btn-primary" @click="router.push('/keys')">{{ t('dashboard.activity.openKeys') }}</button>
    </div>
    <div v-if="status && !status.eligible" class="mt-4 flex flex-wrap items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
      <span>{{ t('dashboard.activity.stepKey') }}</span><Icon name="chevronRight" size="sm" />
      <span>{{ t('dashboard.activity.stepModel') }}</span><Icon name="chevronRight" size="sm" />
      <span>{{ t('dashboard.activity.stepCall') }}</span>
    </div>
    <div v-if="status && !status.eligible && model" class="mt-4 space-y-2">
      <div class="flex items-center justify-between gap-2">
        <label class="text-xs font-medium text-gray-600 dark:text-dark-300">{{ t('dashboard.activity.sample') }}
          <select v-model="model" class="ml-2 max-w-48 border border-gray-300 bg-white px-2 py-1 text-xs text-gray-800 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-100">
            <option v-for="name in availableModels" :key="name" :value="name">{{ name }}</option>
          </select>
        </label>
        <button class="text-xs text-primary-600 hover:underline dark:text-primary-400" @click="copySample">{{ copied ? t('dashboard.activity.copied') : t('dashboard.activity.copy') }}</button>
      </div>
      <pre class="overflow-x-auto bg-gray-50 p-3 text-xs leading-relaxed text-gray-800 dark:bg-dark-900 dark:text-dark-100"><code>{{ sampleCommand }}</code></pre>
    </div>
    <p v-if="error" class="mt-3 text-sm text-red-600" role="alert">{{ error }}</p>
  </section>

  <BaseDialog :show="showCheckin && status?.checkin_visible === true && status?.checkin_enabled === true" :title="t('dashboard.activity.checkin')" width="wide" @close="showCheckin = false">
    <div class="activity-checkin">
      <p class="checkin-subtitle">{{ t('dashboard.activity.subtitle') }}</p>
      <p v-if="!status" class="checkin-message" role="status">{{ error || t('common.loading') }}</p>
      <template v-else>
        <div class="checkin-stats">
          <div><span class="stat-icon green"><Icon name="calendar" /></span><span>{{ t('dashboard.activity.currentStreak') }}</span><strong>{{ status.streak }}<small>{{ t('dashboard.activity.daysUnit') }}</small></strong></div>
          <div><span class="stat-icon blue"><Icon name="trophy" /></span><span>{{ t('dashboard.activity.monthCheckins') }}</span><strong>{{ checkedDays.size }}<small>{{ t('dashboard.activity.timesUnit') }}</small></strong></div>
          <div><span class="stat-icon orange"><Icon name="gift" /></span><span>{{ t('dashboard.activity.bonusCycle') }}</span><strong>7<small>{{ t('dashboard.activity.daysUnit') }}</small></strong></div>
        </div>
        <div class="checkin-columns">
          <section class="checkin-calendar">
            <div class="checkin-section-heading">
              <h3>{{ t('dashboard.activity.calendarTitle') }}</h3><strong>{{ monthLabel }}</strong>
            </div>
            <p class="checkin-caption">{{ dateLabel }} · {{ t('dashboard.activity.calendarNote') }}</p>
            <div class="checkin-weekdays"><span v-for="(weekday, index) in weekdays" :key="index">{{ weekday }}</span></div>
            <div class="checkin-days" :aria-label="t('dashboard.activity.calendarTitle')">
              <template v-for="(day, index) in calendar" :key="index">
                <span v-if="!day" class="checkin-day outside" aria-hidden="true"></span>
                <component :is="day.key === today ? 'button' : 'div'" v-else :type="day.key === today ? 'button' : undefined"
                  class="checkin-day" :class="{ 'is-checked': checkedDays.has(day.key), 'is-today': day.key === today }"
                  :disabled="day.key === today ? !canCheckin : undefined"
                  :aria-label="day.key === today ? checkinLabel : undefined" @click="day.key === today && canCheckin && doCheckin()">
                  <span class="checkin-day-number">{{ day.number }}</span>
                  <template v-if="checkedDays.has(day.key)"><Icon name="checkCircle" class="checkin-stamp" /><span class="checkin-watermark">{{ t('dashboard.activity.signed') }}</span></template>
                  <span v-else class="checkin-day-dot" aria-hidden="true"></span>
                </component>
              </template>
            </div>
            <div class="checkin-legend"><span><i class="green-dot"></i>{{ t('dashboard.activity.signed') }}</span><span><i class="orange-dot"></i>{{ t('dashboard.activity.todayLabel') }}</span></div>
          </section>
          <div class="checkin-sidebar">
            <section class="checkin-rewards">
              <div class="checkin-section-heading"><h3><Icon name="trophy" size="sm" />{{ t('dashboard.activity.stages') }}</h3><span class="checkin-pill">{{ t('dashboard.activity.streak', { days: status.streak }) }}</span></div>
              <p class="checkin-caption">{{ t('dashboard.activity.stageNote') }}</p>
              <div class="checkin-milestones"><div v-for="day in [7, 14, 21, 28]" :key="day" :class="{ reached: status.streak >= day }"><span>{{ t('dashboard.activity.stageDay', { days: day }) }}</span><strong>{{ (status.milestone_reward ?? 0) > 0 ? `+$${(status.milestone_reward ?? 0).toFixed(2)}` : t('dashboard.activity.streakBonus') }}</strong><Icon :name="status.streak >= day ? 'checkCircle' : 'gift'" /></div></div>
              <p class="checkin-next">{{ t('dashboard.activity.nextMilestone', { days: status.next_milestone }) }}</p>
            </section>
            <div class="checkin-base"><span class="stat-icon orange"><Icon name="gift" size="sm" /></span>{{ t('dashboard.activity.dailyReward') }} <strong v-if="(status.daily_reward ?? 0) > 0" class="text-emerald-600 dark:text-emerald-400">+${{ (status.daily_reward ?? 0).toFixed(2) }}</strong><span>{{ t('dashboard.activity.onceDaily') }}</span></div>
            <p v-if="canCheckin && status.next_reward_cents != null" class="checkin-caption" role="status">{{ t('dashboard.activity.expectedCheckinReward',{amount:(status.next_reward_cents/100).toFixed(2)}) }}</p>
            <section class="checkin-budget">
              <div class="checkin-section-heading"><h3><Icon name="gift" size="sm" />{{ t('dashboard.activity.monthRewards') }}</h3><span class="checkin-budget-cap">{{ t('dashboard.activity.monthCap', { amount: status.month_cap.toFixed(2) }) }}</span></div>
              <strong class="checkin-amount">${{ status.month_reward.toFixed(2) }}</strong>
              <div class="checkin-progress" role="progressbar" :aria-label="t('dashboard.activity.monthRewards')" :aria-valuenow="status.month_reward" :aria-valuemax="status.month_cap || 1" :aria-valuemin="0"><span :style="{ width: `${rewardProgress}%` }"></span></div>
              <p class="checkin-caption">{{ t('dashboard.activity.capNote') }}</p>
              <div class="checkin-wallet">
                <div>
                  <span class="checkin-wallet-label">{{ t('dashboard.activity.walletAvailable') }}</span>
                  <strong class="checkin-wallet-amount">${{ (status.wallet_balance ?? 0).toFixed(2) }}</strong>
                </div>
                <button
                  type="button"
                  data-testid="checkin-withdraw"
                  class="checkin-withdraw"
                  :disabled="!canWithdraw"
                  :aria-busy="withdrawing"
                  @click="doWithdraw"
                >
                  {{ withdrawing ? t('dashboard.activity.transferring') : t('dashboard.activity.transferAll',{amount:(status.wallet_balance??0).toFixed(2)}) }}
                </button>
              </div>
              <p class="checkin-caption">{{ t('dashboard.activity.walletShared') }} {{ t('dashboard.activity.walletUse') }}</p>
              <p v-if="status.withdraw_blocked" class="checkin-availability" role="status">{{ t(walletTransferBlockKey(status.withdraw_block_reason)) }}</p>
              <p v-if="!status.checkin_enabled" class="checkin-availability">{{ t('dashboard.activity.unavailableDetail') }}</p>
              <button type="button" class="checkin-action" :disabled="!canCheckin" :aria-busy="checking" @click="doCheckin">{{ checking ? t('common.loading') : checkinLabel }}</button>
              <details class="checkin-ledger" @toggle="onLedgerToggle">
                <summary>{{ t('dashboard.activity.walletLedger') }}</summary>
                <p v-if="!walletLedger.length" class="checkin-caption">{{ t('dashboard.activity.walletLedgerEmpty') }}</p>
                <ul v-else class="checkin-ledger-list">
                  <li v-for="item in walletLedger" :key="item.id">
                    <span>{{ t(`dashboard.activity.ledgerKinds.${item.kind}`) }} · {{ formatLedgerTime(item.created_at) }}</span>
                    <strong :class="item.direction !== 'credit' ? 'is-out' : 'is-in'">{{ item.direction !== 'credit' ? '-' : '+' }}${{ (item.amount_cents / 100).toFixed(2) }}</strong>
                  </li>
                </ul>
              </details>
            </section>
          </div>
        </div>
        <p v-if="notice" class="checkin-notice" role="status">{{ notice }}</p>
        <p v-if="error" class="checkin-error" role="alert">{{ error }}</p>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { checkin, getActivityStatus, getActivityWallet, withdrawActivityWallet, type ActivityStatus, type ActivityWalletLedgerItem } from '@/api/activity'
import { authAPI } from '@/api/auth'
import { userChannelsAPI } from '@/api/channels'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { walletTransferBlockKey } from '@/utils/activityWallet'

const { t, locale } = useI18n()
const props = withDefaults(defineProps<{ quickEntry?: boolean }>(), { quickEntry: false })
const router = useRouter()
const auth = useAuthStore()
const status = ref<ActivityStatus | null>(null)
const error = ref('')
const notice = ref('')
const checking = ref(false)
const withdrawing = ref(false)
const showCheckin = ref(false)
const ledgerOpen = ref(false)
const walletLedger = ref<ActivityWalletLedgerItem[]>([])
const formatLedgerTime = (value: string) => new Intl.DateTimeFormat(locale.value, { timeZone: 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(new Date(value))
async function loadLedger() {
  try { walletLedger.value = (await getActivityWallet()).ledger ?? [] } catch { /* Non-blocking detail list */ }
}
function onLedgerToggle(e: Event) {
  ledgerOpen.value = (e.target as HTMLDetailsElement).open
  if (ledgerOpen.value) void loadLedger()
}
const copied = ref(false)
const baseURL = ref(window.location.origin)
const model = ref('')
const availableModels = ref<string[]>([])
const platform = ref('openai')
const sampleCommand = computed(() => platform.value === 'anthropic' ? [
  `curl ${baseURL.value.replace(/\/$/, '')}/v1/messages \\`,
  '  -H "x-api-key: YOUR_API_KEY" \\',
  '  -H "anthropic-version: 2023-06-01" \\',
  '  -H "Content-Type: application/json" \\',
  `  -d '{"model":"${model.value}","max_tokens":64,"messages":[{"role":"user","content":"Hello"}]}'`,
].join('\n') : [
  `curl ${baseURL.value.replace(/\/$/, '')}/v1/chat/completions \\`,
  '  -H "Authorization: Bearer YOUR_API_KEY" \\',
  '  -H "Content-Type: application/json" \\',
  `  -d '{"model":"${model.value}","messages":[{"role":"user","content":"Hello"}]}'`,
].join('\n'))
const now = ref(Date.now())
const today = computed(() => status.value?.server_day || new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(now.value)))
const year = computed(() => Number(today.value.split('-')[0]))
const month = computed(() => Number(today.value.split('-')[1]))
const monthLabel = computed(() => new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'long', timeZone: 'Asia/Shanghai' }).format(new Date(Date.UTC(year.value, month.value - 1, 1))))
const weekdays = computed(() => locale.value.startsWith('zh') ? ['一', '二', '三', '四', '五', '六', '日'] : ['M', 'T', 'W', 'T', 'F', 'S', 'S'])
const checkedDays = computed(() => new Set(status.value?.checked_days ?? []))
const dateLabel = computed(() => new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'long', day: 'numeric', weekday: 'long', timeZone: 'Asia/Shanghai' }).format(new Date(`${today.value}T12:00:00+08:00`)))
const canCheckin = computed(() => !!status.value?.checkin_enabled && status.value.checkin_eligible && !status.value.checked_today && (status.value.next_reward_cents == null || status.value.next_reward_cents > 0) && !checking.value)
const canWithdraw = computed(() => (status.value?.wallet_balance_cents ?? 0) > 0 && !status.value?.withdraw_blocked && !withdrawing.value)
const checkinLabel = computed(() => {
 if (quotaHint.value) return quotaHint.value
 const current=status.value
 if (!current?.checkin_enabled) return t('dashboard.activity.unavailable')
 if (current.checked_today) return t('dashboard.activity.checked')
 if (current.checkin_require_recharge && current.checkin_has_recharge===false) return t('dashboard.activity.needRecharge')
 return current.checkin_eligible ? t('dashboard.activity.checkNow') : t('dashboard.activity.needSpend',{amount:current.checkin_min_spend.toFixed(2)})
})
const quotaHint = computed(() => {
 const reason=status.value?.checkin_block_reason
 return reason && ['user_cap','monthly_cap','budget_exhausted','no_reward','refund_review','awards_paused'].includes(reason) ? t(`dashboard.activity.checkinBlocks.${reason}`) : ''
})
const rewardProgress = computed(() => status.value?.month_cap ? Math.min(100, Math.max(0, status.value.month_reward / status.value.month_cap * 100)) : 0)
const calendar = computed(() => {
  const offset = (new Date(Date.UTC(year.value, month.value - 1, 1)).getUTCDay() + 6) % 7
  const count = new Date(Date.UTC(year.value, month.value, 0)).getUTCDate()
  return [...Array(offset).fill(null), ...Array.from({ length: count }, (_, i) => ({ number: i + 1, key: `${year.value}-${String(month.value).padStart(2, '0')}-${String(i + 1).padStart(2, '0')}` }))] as ({ number: number; key: string } | null)[]
})
async function refresh() { now.value = Date.now(); try { status.value = await getActivityStatus(); if (!status.value.checkin_visible || !status.value.checkin_enabled) showCheckin.value = false } catch { error.value = t('dashboard.activity.loadFailed') } }
async function openCheckin() {
  showCheckin.value = true
  error.value = ''
  notice.value = ''
  await refresh()
}
async function loadSample() {
  try {
    const [settings, channels] = await Promise.all([authAPI.getPublicSettings(), userChannelsAPI.getAvailable()])
    baseURL.value = settings.api_base_url || window.location.origin
    const platforms = channels.flatMap(channel => channel.platforms)
    const textModels = (names: string[]) => names.filter(name => !/(image|embedding|audio|speech|video|dall-e)/i.test(name))
    const selected = platforms.find(entry => entry.platform === 'openai' && textModels(entry.supported_models.map(item => item.name)).length) || platforms.find(entry => entry.platform === 'anthropic' && textModels(entry.supported_models.map(item => item.name)).length)
    if (selected) {
      platform.value = selected.platform
      availableModels.value = textModels(selected.supported_models.map(item => item.name))
      model.value = availableModels.value[0]
    }
  } catch { /* Keep a visible placeholder until channel settings are available. */ }
}
async function copySample() {
  try { await navigator.clipboard.writeText(sampleCommand.value); copied.value = true; window.setTimeout(() => { copied.value = false }, 2000) }
  catch { error.value = t('dashboard.activity.actionFailed') }
}
async function doCheckin() {
  checking.value = true; error.value = ''; notice.value = ''
  try { status.value = await checkin(); notice.value=t('dashboard.activity.checkinReceived',{amount:((status.value.today_reward_cents??0)/100).toFixed(2)}); window.dispatchEvent(new CustomEvent('activity-wallet-updated')) } catch (e) { error.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed')); await refresh() } finally { checking.value = false }
}
async function doWithdraw() {
  if (!canWithdraw.value) return
  withdrawing.value = true; error.value = ''; notice.value = ''
  try {
    const result = await withdrawActivityWallet()
    notice.value = t('dashboard.activity.withdrawSuccess', { amount: (result.withdrawn ?? 0).toFixed(2) })
    if (status.value) { status.value.wallet_balance_cents=result.available_cents; status.value.wallet_balance=result.available }
    window.dispatchEvent(new CustomEvent('activity-wallet-updated'))
    await Promise.allSettled([refresh(), auth.refreshUser(), ...(ledgerOpen.value?[loadLedger()]:[])])
  } catch (e) {
    error.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed'))
  } finally {
    withdrawing.value = false
  }
}
function onWalletUpdated() { if (!document.hidden) { void refresh(); if (ledgerOpen.value) void loadLedger() } }
let dayTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => { dayTimer=setInterval(()=>{const previous=today.value;now.value=Date.now();const local=new Intl.DateTimeFormat('sv-SE',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date(now.value));if(local!==previous)void refresh()},30000); void refresh(); window.addEventListener('focus', onWalletUpdated); document.addEventListener('visibilitychange', onWalletUpdated); window.addEventListener('activity-wallet-updated', onWalletUpdated); if (!props.quickEntry) void loadSample() })
onUnmounted(() => { clearInterval(dayTimer); window.removeEventListener('focus', onWalletUpdated); document.removeEventListener('visibilitychange', onWalletUpdated); window.removeEventListener('activity-wallet-updated', onWalletUpdated) })
</script>

<style scoped>
/* All Teleport overrides target only this panel. Shared dialogs stay unchanged. */
:global(.modal-content:has(.activity-checkin)) { position: relative; width: 100%; max-width: 58rem; border-radius: 16px; overflow: hidden; }
:global(.modal-content:has(.activity-checkin) .modal-header) { min-height: 58px; padding: 10px 24px 20px 76px; background: #fffbeb; border-bottom: 1px solid #fef3c7; }
:global(.modal-content:has(.activity-checkin) .modal-header::before) { content: '🗓️'; position: absolute; top: 13px; left: 26px; font-size: 27px; }
:global(.modal-content:has(.activity-checkin) .modal-title) { font-size: 18px; line-height: 24px; }
:global(.modal-content:has(.activity-checkin) .modal-body) { padding: 8px 16px; }
:global(.modal-overlay:has(.activity-checkin)) { background: rgb(15 23 42 / 42%); backdrop-filter: blur(5px); }
.activity-checkin { container-type: inline-size; color: #334155; font-size: 12px; }
.checkin-subtitle { position: absolute; top: 35px; left: 76px; color: #64748b; font-size: 11px; }
.checkin-stats { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-bottom: 8px; }
.checkin-stats > div { display: grid; grid-template-columns: 30px 1fr; column-gap: 7px; align-items: center; border: 1px solid #e2e8f0; border-radius: 12px; padding: 7px 12px; }
.checkin-stats > div > span:not(.stat-icon) { color: #64748b; font-size: 11px; }
.stat-icon { display: inline-flex; align-items: center; justify-content: center; width: 30px; height: 30px; border-radius: 10px; }
.checkin-stats .stat-icon { grid-row: span 2; }
.green { background: #dcfce7; color: #22c55e; }.blue { background: #dbeafe; color: #60a5fa; }.orange { background: #ffedd5; color: #fb923c; }
.checkin-stats strong { font-size: 18px; line-height: 21px; color: #0f172a; }
.checkin-stats small { margin-left: 3px; font-size: 12px; font-weight: 400; color: #64748b; }
.checkin-columns { display: grid; grid-template-columns: minmax(0, 1.525fr) minmax(0, 1fr); gap: 10px; }
.checkin-calendar, .checkin-rewards, .checkin-budget { min-width: 0; border: 1px solid #e2e8f0; border-radius: 16px; padding: 11px; }
.checkin-section-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 6px; }
.checkin-section-heading h3 { display: flex; align-items: center; gap: 6px; font-size: 14px; font-weight: 600; color: #0f172a; }
.checkin-section-heading h3 svg { color: #f59e0b; }
.checkin-calendar h3::before { content: ''; width: 3px; height: 12px; background: #f59e0b; border-radius: 2px; margin-right: 6px; }
.checkin-section-heading > strong { font-size: 12px; }
.checkin-caption { margin-top: 5px; font-size: 11px; line-height: 1.6; color: #64748b; }
.checkin-weekdays, .checkin-days { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 5px; }
.checkin-weekdays { margin-top: 9px; margin-bottom: 7px; text-align: center; font-size: 11px; color: #94a3b8; }
.checkin-day { position: relative; min-width: 0; min-height: 54px; padding: 6px; display: flex; flex-direction: column; align-items: flex-start; justify-content: space-between; border: 1px solid #dde5f0; border-radius: 12px; overflow: hidden; text-align: left; background: transparent; }
.checkin-day.outside { border-color: transparent; }
.checkin-day-number { font-size: 13px; font-weight: 600; }.checkin-day-dot { width: 5px; height: 5px; border-radius: 50%; background: #cbd5e1; }
.checkin-day.is-today { border-color: #f59e0b; color: #c2410c; box-shadow: inset 0 0 0 1px #fff7ed; }
.checkin-day.is-today:not(:disabled):hover { background: #fffbeb; cursor: pointer; }
.checkin-day:focus-visible, .checkin-action:focus-visible, .checkin-withdraw:focus-visible { outline: 2px solid #f59e0b; outline-offset: 3px; }
.checkin-day.is-checked { border-color: #a7f3d0; background: #ecfdf5; }
.checkin-stamp { position: absolute; top: 7px; right: 6px; width: 15px; height: 15px; color: #22c55e; }
.checkin-watermark { align-self: flex-end; transform: rotate(-16deg); font-size: 10px; font-weight: 600; color: #059669; opacity: .65; white-space: nowrap; }
.checkin-legend { display: flex; gap: 16px; margin-top: 9px; color: #64748b; font-size: 10px; }.checkin-legend span { display: flex; align-items: center; gap: 6px; }.checkin-legend i { width: 5px; height: 5px; border: 1px solid; border-radius: 50%; }.green-dot { color: #22c55e; }.orange-dot { color: #f59e0b; }
.checkin-sidebar { display: flex; flex-direction: column; gap: 8px; }.checkin-pill { background: #fef3c7; color: #b45309; border-radius: 20px; padding: 5px 9px; font-size: 10px; }
.checkin-milestones { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; margin-top: 7px; }.checkin-milestones > div { position: relative; display: flex; flex-direction: column; gap: 3px; padding: 7px 9px; border: 1px solid #dcf0e3; background: #f6fbf8; border-radius: 11px; }.checkin-milestones strong { color: #059669; font-size: 13px; }.checkin-milestones svg { position: absolute; right: 9px; bottom: 10px; width: 18px; color: #a7dfba; }.checkin-milestones .reached { background: #dcfce7; border-color: #86efac; }.checkin-next { color: #64748b; font-size: 10px; margin-top: 8px; }
.checkin-base { display: flex; align-items: center; gap: 8px; padding: 5px 10px; background: #f8fafc; border-radius: 12px; font-weight: 600; }.checkin-base .stat-icon { width: 27px; height: 27px; }.checkin-base > span:last-child { margin-left: auto; font-size: 10px; font-weight: 400; color: #64748b; }
.checkin-budget { border-color: #c7d2fe; }.checkin-budget-cap { background: #eff6ff; color: #2563eb; padding: 4px 7px; font-size: 10px; border-radius: 14px; }.checkin-amount { display: block; margin: 8px 0; color: #059669; font-size: 25px; line-height: 1.2; }.checkin-progress { height: 5px; overflow: hidden; border-radius: 8px; background: #e2e8f0; }.checkin-progress span { display: block; height: 100%; background: #10b981; border-radius: inherit; }
.checkin-wallet { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; margin-top: 10px; padding: 8px 10px; border-radius: 10px; background: #f0fdf4; border: 1px solid #bbf7d0; }
.checkin-wallet-label { display: block; font-size: 10px; color: #64748b; }
.checkin-wallet-amount { font-size: 14px; color: #059669; font-variant-numeric: tabular-nums; }
.checkin-withdraw { min-height: 30px; padding: 4px 10px; border-radius: 7px; background: #059669; color: #fff; font-size: 11px; font-weight: 600; cursor: pointer; }
.checkin-withdraw:hover:not(:disabled) { background: #047857; }
.checkin-withdraw:disabled { background: #cbd5e1; color: #64748b; cursor: not-allowed; }
.checkin-ledger { margin-top: 10px; padding-top: 8px; border-top: 1px solid #e2e8f0; font-size: 11px; }
.checkin-ledger summary { cursor: pointer; color: #475569; font-weight: 500; min-height: 24px; }
.checkin-ledger-list { margin-top: 6px; max-height: 120px; overflow-y: auto; display: flex; flex-direction: column; gap: 4px; }
.checkin-ledger-list li { display: flex; justify-content: space-between; gap: 8px; padding: 3px 0; border-bottom: 1px dashed #e2e8f0; color: #64748b; font-variant-numeric: tabular-nums; }
.checkin-ledger-list .is-in { color: #059669; }
.checkin-ledger-list .is-out { color: #0284c7; }
:global(.dark .activity-checkin .checkin-ledger), :global(.dark .activity-checkin .checkin-ledger-list li) { border-color: #334155; }
:global(.dark .activity-checkin .checkin-ledger summary) { color: #cbd5e1; }
.checkin-availability { margin-top: 8px; font-size: 11px; line-height: 1.6; color: #64748b; }.checkin-action { width: 100%; margin-top: 10px; padding: 9px; border-radius: 9px; background: #f59e0b; color: #fff; font-size: 13px; font-weight: 600; }.checkin-action:hover:not(:disabled) { background: #d97706; }.checkin-action:disabled { background: #e2e8f0; color: #64748b; cursor: not-allowed; }
.checkin-notice { margin-top: 8px; color: #059669; font-weight: 500; }
.checkin-error { margin-top: 8px; color: #dc2626; }.checkin-message { padding: 28px; color: #64748b; text-align: center; }
:global(.dark .modal-content:has(.activity-checkin) .modal-header) { background: #29251b; border-color: #443824; }
:global(.dark .activity-checkin) { color: #cbd5e1; }
:global(.dark .activity-checkin :is(.checkin-stats > div, .checkin-calendar, .checkin-rewards, .checkin-day)) { border-color: #334155; }
:global(.dark .activity-checkin :is(.checkin-stats strong, .checkin-section-heading h3)) { color: #f1f5f9; }
:global(.dark .activity-checkin :is(.checkin-milestones > div, .checkin-base)) { background: #1e293b; border-color: #334155; }
:global(.dark .activity-checkin .checkin-wallet) { background: #123529; border-color: #166534; }
:global(.dark .activity-checkin .checkin-wallet-amount) { color: #6ee7b7; }
:global(.dark .activity-checkin :is(.checkin-caption, .checkin-availability, .checkin-subtitle, .checkin-wallet-label)) { color: #94a3b8; }
:global(.dark .activity-checkin .checkin-day.outside) { border-color: transparent; }
:global(.dark .activity-checkin .checkin-day.is-checked) { background: #123529; border-color: #166534; }
:global(.dark .activity-checkin .checkin-day.is-today) { border-color: #f59e0b; box-shadow: none; }
:global(.dark .activity-checkin .checkin-day.is-today:not(:disabled):hover) { background: #422b13; }
:global(.dark .activity-checkin :is(.checkin-action:disabled, .checkin-withdraw:disabled)) { background: #334155; color: #94a3b8; }
@container (max-width: 620px) { .checkin-columns { grid-template-columns: minmax(0, 1fr); }.checkin-stats > div { padding: 7px; column-gap: 5px; grid-template-columns: 25px 1fr; }.checkin-stats > div > span:not(.stat-icon) { font-size: 10px; }.checkin-day { min-height: 48px; padding: 5px; }.checkin-stats .stat-icon { width: 25px; height: 25px; } }
@container (max-width: 400px) { .checkin-withdraw { width: 100%; } }
@media (pointer: coarse) { .activity-checkin :is(.checkin-withdraw,.checkin-action,.checkin-ledger summary) { min-height: 48px; } }
</style>
