<template>
  <form ref="form" class="marketing-rewards" aria-labelledby="activity-config-title" novalidate @submit.prevent="save">
    <header class="page-intro"><div><h2 id="activity-config-title" style="font-size:24px">{{ copy.title }}</h2><p class="intro-copy">{{ copy.intro }} · {{ copy.unit }}</p></div><button type="submit" class="action-primary" :disabled="saving || !config" :aria-busy="saving">{{ t('dashboard.activity.save') }}</button></header>
    <ActivityAudience kind="checkin" />
    <template v-if="config">
      <section class="checkin-story" :aria-label="copy.preview">
        <div class="story-copy"><div class="story-icon" aria-hidden="true">✓</div><h2>{{ t('dashboard.activity.checkin') }}</h2><p>{{ t('dashboard.activity.configHelp.checkin') }}</p><div class="story-foot">{{ copy.cap }}</div></div>
        <div class="seven-days"><div class="preview-caption"><span>{{ copy.preview }}</span><span class="muted">{{ copy.unit }}</span></div><div class="day-track"><div v-for="day in 7" :key="day" class="day-stop" :class="{'bonus-stop':day===7}"><span class="day-number">{{ copy.day.replace('{n}',String(day)) }}</span><span class="day-symbol" aria-hidden="true">{{ day===7?'✦':'✓' }}</span><strong>${{ dollar(Number(config.checkin_daily_cents)+(day===7?Number(config.checkin_milestone_cents):0)) }}</strong><span v-if="day===7" class="bonus-label">{{ copy.bonus }}</span></div></div><div class="story-rule"><span>{{ t('dashboard.activity.capCents') }} <b>${{ dollar(config.checkin_monthly_cap_cents) }}</b></span></div></div>
      </section>
      <div class="workspace-grid">
        <section class="config-surface"><div class="surface-title"><div><h2>{{ copy.rules }}</h2><p>{{ copy.pending }}</p></div><label class="enable-control"><input v-model="config.checkin_enabled" role="switch" type="checkbox" name="checkin_enabled" :aria-label="t('dashboard.activity.checkin')"><span class="switch-track" aria-hidden="true"></span><span>{{ config.checkin_enabled?copy.enabled:copy.disabled }}</span></label></div>
          <section v-for="group in fieldGroups" :key="group.title" class="section-group"><h3>{{ group.title }}</h3><div class="field-grid">
            <div v-for="field in group.fields" :key="field.key" class="field"><div><label :for="`activity-${field.key}`">{{ t(field.label) }}</label><RewardFieldTip :label="t(field.label)" :text="t(`dashboard.activity.configHelp.${field.key}`)" /></div><div class="input-unit" :class="{suffix:field.key==='checkin_rollout_percent'}"><span aria-hidden="true">{{ field.key==='checkin_rollout_percent'?'%':'$' }}</span><input :id="`activity-${field.key}`" :name="field.key" :aria-invalid="!!errors[field.key]" :aria-describedby="`activity-${field.key}-help${errors[field.key] ? ` activity-${field.key}-error` : ''}`" @input="clearError(field.key)" v-model.number="config[field.key]" type="number" min="0" :max="field.key==='checkin_monthly_cap_cents'?15:field.key==='checkin_rollout_percent'?100:undefined" :step="field.key==='checkin_rollout_percent'?1:0.01"></div><span :id="`activity-${field.key}-help`" class="sr-help">{{ t(`dashboard.activity.configHelp.${field.key}`) }}</span><small v-if="field.key==='checkin_monthly_cap_cents'" class="essential">{{ copy.cap }}</small><span v-if="errors[field.key]" :id="`activity-${field.key}-error`" class="error-text">{{ t(`dashboard.activity.validation.${errors[field.key]}`) }}</span></div>
          </div></section>
          <div class="form-foot"><span>{{ copy.pending }}</span><button type="submit" class="action-primary" :disabled="saving" :aria-busy="saving">{{ t('dashboard.activity.save') }}</button></div>
        </section>
        <aside class="side-stack"><section class="budget-panel"><div class="surface-title"><h2>{{ copy.budget }}</h2></div><div class="budget-label">{{ copy.month }}</div><div class="budget-amount">${{ dollar(config.checkin_month_spent_cents) }} <span>/ ${{ dollar(config.checkin_monthly_budget_cents) }}</span></div><div class="budget-bar" aria-hidden="true"><span :style="{width:budgetUsage+'%'}"></span></div>
          <div v-for="field in budgetFields" :key="field.key" class="field"><div><label :for="`activity-${field.key}`">{{ t(field.label) }}</label><RewardFieldTip :label="t(field.label)" :text="t(`dashboard.activity.configHelp.${field.key}`)" /></div><div class="input-unit"><span aria-hidden="true">$</span><input :id="`activity-${field.key}`" :name="field.key" v-model.number="config[field.key]" type="number" min="0" step="0.01" :aria-invalid="!!errors[field.key]" :aria-describedby="`activity-${field.key}-help${errors[field.key]?` activity-${field.key}-error`:''}`" @input="clearError(field.key)"></div><span :id="`activity-${field.key}-help`" class="sr-help">{{ t(`dashboard.activity.configHelp.${field.key}`) }}</span><span v-if="errors[field.key]" :id="`activity-${field.key}-error`" class="error-text">{{ t(`dashboard.activity.validation.${errors[field.key]}`) }}</span></div>
          <div class="inline-note">{{ copy.note }}</div><h3>{{ copy.shared }}</h3><p class="section-copy" style="margin-top:10px">{{ t('dashboard.activity.totalSpent',{spent:dollar(config.total_spent_cents),budget:dollar(config.total_budget_cents)}) }}</p>
        </section><div class="quiet-note"><div><h3>{{ t('dashboard.activity.configHelp.budgetTitle') }}</h3><p>{{ t('dashboard.activity.configHelp.budgetRule') }}</p><p>{{ t('dashboard.activity.configHelp.example') }}</p></div></div></aside>
      </div>
    </template>
    <p v-else-if="!failed" class="section-copy" role="status">{{ copy.loading }}</p>
    <p v-if="message" class="reward-status" :class="failed?'text-red-600 dark:text-red-400':'text-emerald-700 dark:text-emerald-300'" role="status">{{ message }}</p>
    <section v-if="reviews.length" class="accounting-panel"><h3>{{ t('dashboard.activity.refundReviews') }}</h3><div v-for="review in reviews" :key="review.order_id" class="review-row"><span>{{ t('dashboard.activity.refundRow',{order:review.order_id,user:review.user_id,status:review.order_status}) }}</span><button type="button" class="action-secondary" @click="markReviewed(review.order_id)">{{ t('dashboard.activity.markReviewed') }}</button></div></section>
  </form>
</template>
<script setup lang="ts">
import ActivityAudience from './ActivityAudience.vue'
import { computed, nextTick, onMounted, ref } from 'vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { activityConfigToForm, activityConfigToApi, validateActivityConfig, type ActivityAmountKey, type ActivityConfigErrors } from '@/utils/activityConfigValidation'
import { useI18n } from 'vue-i18n'
import { getAdminActivityConfig, updateAdminActivityConfig, getActivityRefundReviews, markActivityRefundReviewed, type AdminActivityConfig, type ActivityRefundReview } from '@/api/admin/activity'

const { t, locale } = useI18n()
import RewardFieldTip from './RewardFieldTip.vue'
const copy = computed(() => locale.value.startsWith('zh') ? { title:'让每一次回来，都有一点收获', intro:'签到奖励与预算配置', preview:'七天奖励路径', day:'第 {n} 天', bonus:'含连续签到奖励', rules:'奖励规则', eligibility:'参与资格', reward:'奖励设置', budget:'预算与额度', month:'本月已发放', shared:'所有活动共用额度', pending:'开关与规则修改后，点击保存才会生效。', enabled:'已开启', disabled:'已关闭', loading:'正在加载签到配置…', cap:'每人每月最多 $15，仍受充值约束与其他额度限制。', unit:'所有金额均为美元（USD）', note:'预算为 0 时不发放奖励。', help:'字段说明' } : { title:'A little reward for coming back', intro:'Check-in rewards and budget', preview:'Seven-day reward path', day:'Day {n}', bonus:'Includes streak bonus', rules:'Reward rules', eligibility:'Eligibility', reward:'Reward amounts', budget:'Budget and limits', month:'Paid this month', shared:'Shared across all activities', pending:'Switch and rule changes take effect after saving.', enabled:'Enabled', disabled:'Disabled', loading:'Loading check-in settings…', cap:'Up to $15 per user per month, subject to recharge and other limits.', unit:'All amounts in USD', note:'A zero budget stops rewards.', help:'Field help' })
const dollar = (value: number) => Number.isFinite(Number(value)) ? Number(value).toFixed(2) : '—'
const budgetUsage = computed(() => config.value && Number(config.value.checkin_monthly_budget_cents) > 0 ? Math.min(100, config.value.checkin_month_spent_cents / Number(config.value.checkin_monthly_budget_cents) * 100) : 0)
const fieldGroups = computed(() => [ {title:copy.value.eligibility, fields:checkinFields.slice(0,2)}, {title:copy.value.reward, fields:checkinFields.slice(2,5)} ])
const budgetFields: { key: ActivityAmountKey; label: string }[] = [{key:'checkin_monthly_budget_cents',label:'dashboard.activity.monthBudgetCents'},{key:'total_budget_cents',label:'dashboard.activity.totalBudgetCents'},{key:'user_lifetime_cap_cents',label:'dashboard.activity.userCapCents'}]
const config = ref<AdminActivityConfig | null>(null)
const form = ref<HTMLFormElement | null>(null)
const errors = ref<ActivityConfigErrors>({})
function clearError(key: ActivityAmountKey) { delete errors.value[key]; message.value = '' }
const saving = ref(false)
const failed = ref(false)
const message = ref('')
const reviews = ref<ActivityRefundReview[]>([])
type AmountKey = ActivityAmountKey
const checkinFields: { key: AmountKey; label: string }[] = [
  { key: 'checkin_min_spend_cents', label: 'dashboard.activity.minSpend' },
  { key: 'checkin_rollout_percent', label: 'dashboard.activity.rolloutPercent' },
  { key: 'checkin_daily_cents', label: 'dashboard.activity.dailyCents' },
  { key: 'checkin_milestone_cents', label: 'dashboard.activity.milestoneCents' },
  { key: 'checkin_monthly_cap_cents', label: 'dashboard.activity.capCents' },
  { key: 'checkin_monthly_budget_cents', label: 'dashboard.activity.monthBudgetCents' },
]
onMounted(async () => { try { const [loaded, loadedReviews] = await Promise.all([getAdminActivityConfig(), getActivityRefundReviews()]); config.value = activityConfigToForm({...loaded, second_recharge_enabled:false}); reviews.value = loadedReviews } catch { failed.value = true; message.value = t('dashboard.activity.loadFailed') } })
async function markReviewed(orderID: number) {
  try { await markActivityRefundReviewed(orderID); reviews.value = reviews.value.filter(review => review.order_id !== orderID) }
  catch (error) { failed.value = true; message.value = extractApiErrorMessage(error, t('dashboard.activity.actionFailed')) }
}
async function save() {
  if (!config.value || saving.value) return
  const payload = activityConfigToApi(config.value)
  errors.value = validateActivityConfig(payload)
  for (const key of Object.keys(errors.value) as ActivityAmountKey[]) {
    if (key !== 'checkin_rollout_percent' && errors.value[key] === 'nonNegativeInteger') errors.value[key] = 'moneyPrecision'
  }
  if (Object.keys(errors.value).length) {
    failed.value = true; message.value = t('dashboard.activity.validation.summary')
    await nextTick()
    form.value?.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()
    return
  }
  saving.value = true; message.value = ''
  try { config.value = activityConfigToForm(await updateAdminActivityConfig(payload)); failed.value = false; message.value = t('dashboard.activity.saved') }
  catch (error) { failed.value = true; message.value = extractApiErrorMessage(error, t('dashboard.activity.actionFailed')) }
  finally { saving.value = false }
}
</script>

<style scoped src="./marketingRewards.css"></style>
<style src="./marketingRewardsDark.css"></style>
