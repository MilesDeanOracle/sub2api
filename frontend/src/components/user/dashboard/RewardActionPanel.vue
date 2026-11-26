<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import Icon from '@/components/icons/Icon.vue'
import { checkin, getActivityStatus, getActivityWallet, getLotteryStatus, withdrawActivityWallet, rewardMomentsAPI, type ActivityStatus, type ActivityWalletSummary, type LotteryStatus, type RewardMoment, type RewardSharing } from '@/api/activity'
import { consumptionAPI, type ConsumptionSummary } from '@/api/consumption'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { walletTransferBlockKey } from '@/utils/activityWallet'

const { t, locale } = useI18n()
const auth = useAuthStore()
const id = useId()
const money = (cents: number) => `$${(cents / 100).toFixed(2)}`
const time = (value: string) => new Intl.DateTimeFormat(locale.value, { timeZone: 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(new Date(value))
const status = ref<ActivityStatus | null>(null)
const consumption = ref<ConsumptionSummary | null>(null)
const lottery = ref<LotteryStatus | null>(null)
const wallet = ref<ActivityWalletSummary | null>(null)
const sharing = ref<RewardSharing | null>(null)
const moments = ref<RewardMoment[]>([])
const expanded = ref(false)
const busy = ref('')
const savingSharing = ref(false)
const notice = ref('')
const error = ref('')
const loadFailed = ref(false)
const needsRefresh = ref(false)
const now = ref(Date.now())
const root = ref<HTMLElement | null>(null)
let version = 0
let feedVersion = 0
let sharingVersion = 0
let disposed = false
let timer: ReturnType<typeof setInterval> | undefined
const campaign = computed(() => consumption.value?.campaign)
const consumptionVisible = computed(() => !!campaign.value)
const checkinVisible = computed(() => status.value?.checkin_visible === true)
const lotteryVisible = computed(() => lottery.value?.visible === true)
const visible = computed(() => checkinVisible.value || lotteryVisible.value || consumptionVisible.value || wallet.value?.visible || sharing.value?.allowed || loadFailed.value)
const canCheckin = computed(() => !!status.value?.checkin_enabled && status.value.checkin_eligible && !status.value.checked_today && (status.value.next_reward_cents ?? 0) > 0)
const claimable = computed(() => campaign.value && Date.parse(campaign.value.claim_until) > now.value ? consumption.value?.claimable_cents ?? 0 : 0)
const canClaim = computed(() => claimable.value > 0 && consumption.value?.awards_enabled !== false && !consumption.value?.blocked)
const canDraw = computed(() => (lottery.value?.remaining_draws ?? 0) > 0 && !lottery.value?.draw_blocked && !lottery.value?.pool_paused && lottery.value?.awards_enabled !== false)
const checkinHint = computed(() => {
  const s = status.value
  if (!s) return ''
  if (s.checked_today) return t('dashboard.activity.checked')
  if (s.checkin_block_reason && ['user_cap', 'monthly_cap', 'budget_exhausted', 'no_reward', 'refund_review', 'awards_paused'].includes(s.checkin_block_reason)) return t(`dashboard.activity.checkinBlocks.${s.checkin_block_reason}`)
  if (!s.checkin_enabled) return t('dashboard.activity.unavailableDetail')
  if (canCheckin.value) return t('dashboard.activity.expectedCheckinReward', { amount: ((s.next_reward_cents ?? 0) / 100).toFixed(2) })
  if (s.checkin_require_recharge && !s.checkin_has_recharge) return t('dashboard.activity.needRecharge')
  return t('dashboard.activity.needSpend', { amount: s.checkin_min_spend.toFixed(2) })
})
const claimHint = computed(() => {
  if (consumption.value?.awards_enabled === false) return t('dashboard.rewards.awardsPaused')
  if (consumption.value?.blocked) return t('dashboard.consumption.review')
  if (claimable.value && campaign.value) return t('dashboard.rewards.claimDeadline', { amount: money(claimable.value), date: time(campaign.value.claim_until) })
  if (consumption.value?.enabled === false) return t('dashboard.rewards.eligibilityClosed')
  if (consumption.value?.budget_blocked) return t('dashboard.rewards.budgetUnavailable')
  return t('dashboard.rewards.progressHint')
})
const hero = computed(() => {
  if (wallet.value?.withdraw_blocked) return { title: t('dashboard.rewards.reviewTitle'), value: '', body: t(walletTransferBlockKey(wallet.value.withdraw_block_reason)), label: t('dashboard.rewards.viewWallet'), action: 'wallet' }
  if (canClaim.value) return { title: t('dashboard.rewards.claimReady'), value: money(claimable.value), body: claimHint.value, label: t('dashboard.consumption.claim', { amount: money(claimable.value) }), action: 'claim' }
  if (canCheckin.value) return { title: t('dashboard.rewards.checkinReady'), value: money(status.value!.next_reward_cents!), body: t('dashboard.rewards.checkinHint'), label: t('dashboard.rewards.checkinAction', { amount: money(status.value!.next_reward_cents!) }), action: 'checkin' }
  if (canDraw.value) return { title: t('dashboard.rewards.drawsReady'), value: t('dashboard.rewards.drawCount', { count: lottery.value!.remaining_draws }), body: t('dashboard.rewards.drawHint'), label: t('dashboard.rewards.openDraw'), action: 'lottery' }
  return { title: t('dashboard.rewards.exploreTitle'), value: '', body: t('dashboard.rewards.exploreHint'), label: t('dashboard.rewards.exploreEligibility'), action: checkinVisible.value ? 'checkin-details' : consumptionVisible.value ? 'consumption' : lotteryVisible.value ? 'lottery' : 'wallet' }
})

async function refresh() {
  const current = ++version
  const owner = auth.user?.id
  now.value = Date.now()
  const results = await Promise.allSettled([getActivityStatus(), consumptionAPI.status(), getLotteryStatus(), getActivityWallet()])
  if (disposed || current !== version || owner !== auth.user?.id || busy.value) return
  if (results[0].status === 'fulfilled') status.value = results[0].value
  if (results[1].status === 'fulfilled') consumption.value = results[1].value
  if (results[2].status === 'fulfilled') lottery.value = results[2].value
  if (results[3].status === 'fulfilled') wallet.value = results[3].value
  loadFailed.value = results.some(result => result.status === 'rejected')
  needsRefresh.value = loadFailed.value
}
async function refreshFeed() {
  if (!sharing.value?.display_enabled || document.hidden) return
  const current = ++feedVersion
  const owner = auth.user?.id
  try {
    const value = await rewardMomentsAPI.feed()
    if (disposed || current !== feedVersion || owner !== auth.user?.id) return
    moments.value = value.enabled ? value.items : []
    if (!value.enabled && sharing.value) sharing.value.display_enabled = false
  } catch { if (current === feedVersion) moments.value = [] }
}
async function refreshSharing() {
  const current = ++sharingVersion
  const owner = auth.user?.id
  try {
    const value = await rewardMomentsAPI.sharing()
    if (disposed || current !== sharingVersion || savingSharing.value || owner !== auth.user?.id) return
    sharing.value = value
    if (!value.display_enabled) { ++feedVersion; moments.value = [] }
    else await refreshFeed()
  } catch { if (current === sharingVersion) { ++feedVersion; moments.value = []; sharing.value = null } }
}
async function setSharing(event: Event) {
  const input = event.target as HTMLInputElement
  if (savingSharing.value || !sharing.value) return
  const allowed = input.checked
  ++sharingVersion; ++feedVersion
  moments.value = []
  savingSharing.value = true
  error.value = ''; notice.value = ''
  try {
    sharing.value = await rewardMomentsAPI.setSharing(allowed)
    notice.value = allowed ? t('dashboard.rewards.consentGranted') : t('dashboard.rewards.consentRevoked')
  } catch (e) { input.checked = sharing.value.allowed; error.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed')) }
  finally { savingSharing.value = false }
}
async function focusTask(kind: string) {
  await nextTick()
  const element = root.value?.querySelector<HTMLElement>(`[data-task="${kind}"]`)
  element?.focus({ preventScroll: true })
  element?.scrollIntoView({ block: 'nearest', behavior: 'instant' })
}
function openDetails(kind: string, campaignId = campaign.value?.id) {
  if (kind === 'checkin') window.dispatchEvent(new CustomEvent('activity-checkin-requested'))
  if (kind === 'lottery') window.dispatchEvent(new CustomEvent('activity-lottery-requested'))
  if (kind === 'consumption') window.dispatchEvent(new CustomEvent('activity-claim-requested', { detail: { campaignId } }))
}
function focusMoment(item: RewardMoment) {
  if (item.kind === 'consumption' && item.campaign_id && item.campaign_id !== campaign.value?.id) return openDetails('consumption', item.campaign_id)
  void focusTask(item.kind)
}
async function run(action: string) {
  if (busy.value || needsRefresh.value) return
  if (action === 'wallet') return focusTask('wallet')
  if (action === 'lottery' || action === 'consumption' || action === 'checkin-details') return openDetails(action === 'checkin-details' ? 'checkin' : action)
  if ((action === 'claim' && !canClaim.value) || (action === 'checkin' && !canCheckin.value) || (action === 'transfer' && (!wallet.value || wallet.value.available_cents <= 0 || wallet.value.withdraw_blocked))) return
  busy.value = action; ++version; notice.value = ''; error.value = ''
  try {
    if (action === 'claim' && campaign.value) {
      consumption.value = await consumptionAPI.claim(campaign.value.id)
      wallet.value = consumption.value.wallet
      notice.value = t('dashboard.activity.walletClaimed')
    } else if (action === 'checkin') {
      status.value = await checkin()
      notice.value = t('dashboard.activity.checkinReceived', { amount: ((status.value.today_reward_cents ?? 0) / 100).toFixed(2) })
    } else if (action === 'transfer') {
      wallet.value = await withdrawActivityWallet()
      notice.value = t('dashboard.activity.withdrawSuccess', { amount: (wallet.value.withdrawn ?? 0).toFixed(2) })
      await Promise.allSettled([auth.refreshUser()])
    }
    window.dispatchEvent(new CustomEvent('activity-wallet-updated'))
  } catch (e) {
    error.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed'))
  } finally {
    busy.value = ''
    await refresh()
  }
}
function sync() { if (!document.hidden && !busy.value) { void refresh(); void refreshSharing() } }
onMounted(() => {
  sync()
  timer = setInterval(sync, 60000)
  window.addEventListener('activity-wallet-updated', sync)
  window.addEventListener('activity-lottery-updated', sync)
  window.addEventListener('focus', sync)
  document.addEventListener('visibilitychange', sync)
})
onUnmounted(() => {
  disposed = true; ++version; ++feedVersion; ++sharingVersion; clearInterval(timer)
  window.removeEventListener('activity-wallet-updated', sync)
  window.removeEventListener('activity-lottery-updated', sync)
  window.removeEventListener('focus', sync)
  document.removeEventListener('visibilitychange', sync)
})
</script>

<template>
  <section v-if="visible" ref="root" class="reward-center" :aria-labelledby="`${id}-title`" data-testid="reward-action-panel">
    <header class="center-heading"><div><h2 :id="`${id}-title`">{{ t('dashboard.rewards.title') }}</h2><p>{{ t('dashboard.rewards.subtitle') }}</p></div><span class="site-only">{{ t('dashboard.rewards.siteOnly') }}</span></header>
    <div class="center-columns" :class="{ 'without-feed': !moments.length }">
      <div class="personal">
        <div class="next-action"><div><span class="action-caption">{{ t('dashboard.rewards.nextStep') }}</span><h3>{{ hero.title }}</h3><strong v-if="hero.value" class="hero-value">{{ hero.value }}</strong><p>{{ hero.body }}</p></div><Button type="button" :disabled="!!busy || needsRefresh" :aria-busy="!!busy" data-testid="reward-primary" @click="run(hero.action)">{{ busy ? t('common.loading') : hero.label }}</Button></div>
        <div class="task-list">
          <article v-if="checkinVisible" data-task="checkin" class="task" tabindex="-1"><span class="task-icon"><Icon name="calendar" aria-hidden="true" /></span><div class="task-copy"><h3>{{ t('dashboard.rewards.checkin') }}</h3><p>{{ checkinHint }}</p></div><Button type="button" variant="outline" :disabled="!!busy || needsRefresh || !status?.checkin_enabled" data-testid="reward-checkin" @click="canCheckin ? run('checkin') : openDetails('checkin')">{{ canCheckin ? t('dashboard.rewards.checkinAction', { amount: money(status!.next_reward_cents!) }) : !status?.checkin_enabled ? t('dashboard.rewards.activityClosed') : t('dashboard.rewards.viewEligibility') }}</Button></article>
          <article v-if="lotteryVisible" data-task="lottery" class="task" tabindex="-1"><span class="task-icon amber"><Icon name="gift" aria-hidden="true" /></span><div class="task-copy"><h3>{{ t('dashboard.rewards.lottery') }}</h3><p>{{ t('dashboard.rewards.drawsRemaining', { count: lottery!.remaining_draws }) }}<template v-if="lottery!.draw_blocked || lottery!.pool_paused || !lottery!.awards_enabled"> · {{ t('dashboard.rewards.drawingUnavailable') }}</template></p></div><Button type="button" variant="outline" :disabled="!!busy || needsRefresh" data-testid="reward-lottery" @click="openDetails('lottery')">{{ canDraw ? t('dashboard.rewards.openDraw') : t('dashboard.rewards.viewRules') }}</Button></article>
          <article v-if="consumptionVisible" data-task="consumption" class="task" tabindex="-1"><span class="task-icon"><Icon name="trophy" aria-hidden="true" /></span><div class="task-copy"><h3>{{ campaign!.name }}</h3><p>{{ claimHint }}</p></div><Button type="button" variant="outline" :disabled="!!busy || needsRefresh" data-testid="reward-claim" @click="canClaim ? run('claim') : openDetails('consumption')">{{ canClaim ? t('dashboard.consumption.claim', { amount: money(claimable) }) : t('dashboard.rewards.viewProgress') }}</Button></article>
        </div>
        <p class="result-notice" role="status">{{ notice }}</p>
        <p v-if="error" class="result-error" role="alert">{{ error }}</p>
        <p v-if="loadFailed" class="outside-note">{{ t('dashboard.rewards.loadFailed') }} <button type="button" class="underline" @click="refresh">{{ t('dashboard.rewards.retry') }}</button></p>
      </div>
      <aside v-if="moments.length" class="community" :aria-labelledby="`${id}-feed`">
        <div class="feed-heading"><h3 :id="`${id}-feed`">{{ t('dashboard.rewards.feedTitle') }}</h3><p>{{ t('dashboard.rewards.feedTime') }}</p></div>
        <ul class="feed-list" role="list"><li v-for="item in moments.slice(0, expanded ? 6 : 3)" :key="item.id" class="feed-event"><span class="avatar" aria-hidden="true"><Icon name="user" /></span><div class="feed-copy"><div class="feed-user"><span>{{ t('dashboard.rewards.anonymous') }} {{ item.alias }}</span><strong>+{{ money(item.amount_cents) }}</strong></div><button type="button" class="feed-kind" @click="focusMoment(item)">{{ item.campaign_name || (item.kind === 'checkin' ? t('dashboard.rewards.checkinReward') : item.kind === 'lottery' ? t('dashboard.rewards.lotteryReward') : t('dashboard.rewards.consumptionReward')) }} →</button><time :datetime="item.created_at">{{ time(item.created_at) }}</time></div></li></ul>
        <Button v-if="moments.length > 3" variant="ghost" type="button" :aria-expanded="expanded" @click="expanded = !expanded">{{ expanded ? t('dashboard.rewards.showLess') : t('dashboard.rewards.showMore') }}</Button>
        <p class="feed-note">{{ t('dashboard.rewards.feedPrivacy') }}</p>
      </aside>
    </div>
    <div v-if="wallet" data-task="wallet" class="wallet-panel" tabindex="-1"><div class="wallet-copy"><Icon name="creditCard" aria-hidden="true" /><div><h3>{{ t('dashboard.rewards.walletTitle') }}</h3><p>{{ t('dashboard.rewards.walletHint') }}</p><p v-if="wallet.withdraw_blocked">{{ t(walletTransferBlockKey(wallet.withdraw_block_reason)) }}</p></div></div><div class="wallet-action"><strong>{{ money(wallet.available_cents) }}</strong><Button type="button" :disabled="!!busy || needsRefresh || wallet.withdraw_blocked || wallet.available_cents <= 0" :aria-busy="busy === 'transfer'" data-testid="reward-transfer" @click="run('transfer')">{{ busy === 'transfer' ? t('dashboard.activity.transferring') : t('dashboard.activity.transferAll', { amount: (wallet.available_cents / 100).toFixed(2) }) }}</Button></div></div>
    <footer class="center-footer"><span>{{ t('dashboard.rewards.walletFooter') }}</span><details><summary>{{ t('dashboard.rewards.privacyTitle') }}</summary><p>{{ t('dashboard.rewards.privacyHint') }}</p><label v-if="sharing" class="privacy-option" :for="`${id}-sharing`"><input :id="`${id}-sharing`" type="checkbox" :checked="sharing.allowed" :disabled="savingSharing" data-testid="reward-sharing" @change="setSharing">{{ t('dashboard.rewards.allowSharing') }}</label><button v-else type="button" class="underline" @click="refreshSharing">{{ t('dashboard.rewards.retryPrivacy') }}</button></details></footer>
  </section>
</template>

<style scoped>
.reward-center{container:rewards/inline-size;border:1px solid hsl(var(--border));border-radius:16px;background:hsl(var(--card));color:hsl(var(--foreground));--reward-positive:#087b66;--reward-tint:#f0faf6;--reward-amber:#956114}
:global(.dark .reward-center){--reward-positive:#72d9bd;--reward-tint:#142e2a;--reward-amber:#e7b769}
.center-heading{display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:12px;padding:26px 28px 22px}.center-heading h2{font-size:19px;font-weight:650;text-wrap:balance}.center-heading p{margin-top:6px;color:hsl(var(--muted-foreground));font-size:13px}.site-only{border:1px solid hsl(var(--border));border-radius:20px;padding:4px 10px;font-size:11px}.center-columns{display:grid;grid-template-columns:minmax(0,1.65fr) minmax(280px,1fr)}.center-columns.without-feed{grid-template-columns:minmax(0,1fr)}.personal{min-width:0;padding:0 28px 24px}.next-action{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:20px;padding:23px 24px;background:var(--reward-tint);border:1px solid color-mix(in srgb,var(--reward-positive) 18%,transparent);border-radius:10px}.next-action>div{flex:1 1 200px;min-width:0}.action-caption{color:var(--reward-positive);font-size:12px;font-weight:550}.next-action h3{margin-top:10px;font-size:14px;font-weight:550}.hero-value{display:block;margin:4px 0 5px;font-size:38px;line-height:1.3;font-weight:650;font-variant-numeric:tabular-nums;color:var(--reward-positive)}.next-action p,.task p{font-size:12px;line-height:1.8;color:hsl(var(--muted-foreground));text-wrap:pretty}.next-action>button{min-height:44px;flex-shrink:0}.task-list{margin-top:12px}.task{display:flex;align-items:center;gap:12px;padding:17px 0;border-bottom:1px solid hsl(var(--border));scroll-margin-top:120px}.task:last-child{border:0}.task-icon{display:grid;place-items:center;width:34px;height:34px;flex:none;border-radius:9px;background:var(--reward-tint);color:var(--reward-positive)}.task-icon.amber{color:var(--reward-amber)}.task-copy{flex:1;min-width:0}.task h3,.wallet-copy h3{font-size:13px;font-weight:600}.task p{margin-top:5px}.task button{min-height:40px}.community{min-width:0;border-left:1px solid hsl(var(--border));padding:0 26px 20px}.feed-heading h3{font-size:14px;font-weight:600}.feed-heading p,.feed-note{margin-top:6px;font-size:11px;line-height:1.8;color:hsl(var(--muted-foreground))}.feed-list{list-style:none;margin:0;padding:0}.feed-event{display:flex;gap:10px;padding:18px 0;border-bottom:1px solid hsl(var(--border))}.avatar{display:grid;place-items:center;width:30px;height:30px;flex:none;border-radius:50%;background:hsl(var(--secondary));color:hsl(var(--muted-foreground))}.avatar svg{width:16px;height:16px}.feed-copy{min-width:0;flex:1}.feed-user{display:flex;justify-content:space-between;gap:8px;font-size:11px;flex-wrap:wrap;overflow-wrap:anywhere}.feed-user strong{color:var(--reward-positive);font-variant-numeric:tabular-nums;white-space:nowrap}.feed-kind{min-height:32px;text-align:start;color:var(--reward-positive);font-size:12px}.feed-copy time{display:block;font-size:11px;color:hsl(var(--muted-foreground))}.wallet-panel{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:20px;padding:20px 28px;border-top:1px solid hsl(var(--border));background:var(--reward-tint)}.wallet-copy{display:flex;align-items:center;gap:12px}.wallet-copy>svg{flex:none;color:var(--reward-positive)}.wallet-copy p{margin-top:4px;font-size:12px;line-height:1.8;color:hsl(var(--muted-foreground))}.wallet-action{display:flex;align-items:center;flex-wrap:wrap;gap:18px}.wallet-action strong{font-size:24px;font-weight:650;font-variant-numeric:tabular-nums;color:var(--reward-positive)}.center-footer{display:flex;justify-content:space-between;flex-wrap:wrap;gap:16px;padding:15px 28px;font-size:11px;line-height:1.8;color:hsl(var(--muted-foreground))}.center-footer details{max-width:360px}.center-footer summary{cursor:pointer;min-height:28px}.privacy-option{display:flex;align-items:center;gap:8px;font-size:12px;min-height:44px;cursor:pointer}.privacy-option input{width:16px;height:16px;accent-color:var(--reward-positive);flex:none}.result-notice,.result-error,.outside-note{font-size:12px;line-height:1.8;margin-top:12px}.result-notice{color:var(--reward-positive)}.result-error{color:#b91c1c}:global(.dark .reward-center .result-error){color:#fca5a5}.outside-note{color:hsl(var(--muted-foreground))}.reward-center :is(button,input,summary,.task,.wallet-panel):focus-visible{outline:2px solid var(--reward-positive);outline-offset:3px}
@container rewards (max-width:820px){.center-columns{grid-template-columns:minmax(0,1fr)}.community{border-left:0;border-top:1px solid hsl(var(--border));padding:20px 24px}}
@media(max-width:600px){.center-heading{padding:20px 18px}.personal{padding:0 18px 18px}.next-action{padding:18px}.next-action>button{width:100%}.hero-value{font-size:34px}.task{flex-wrap:wrap;gap:10px}.task-copy{flex-basis:calc(100% - 48px)}.task button{margin-left:44px;min-height:44px}.community{padding:20px 18px}.wallet-panel{padding:18px}.wallet-action{width:100%;justify-content:space-between;gap:12px}.wallet-action button{white-space:normal;height:auto;min-height:44px}.center-footer{padding:14px 18px;flex-direction:column}.center-footer summary,.feed-kind{min-height:44px}}
@media(pointer:coarse){.reward-center :is(button,summary){min-height:48px}}
</style>
