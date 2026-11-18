<template>
  <section v-if="summary?.campaign || summary?.history.length" class="card p-5 sm:p-6" aria-labelledby="consumption-title">
    <template v-if="campaign">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-3"><h2 id="consumption-title" class="font-semibold text-gray-900 dark:text-white">{{ campaign.name }}</h2><span class="text-sm text-gray-500">{{ t('dashboard.consumption.spent', {amount:displaySpend}) }}</span></div>
        <p class="text-sm text-gray-600 dark:text-dark-300">{{ nextTier ? t('dashboard.consumption.next', {amount:remaining, reward:money(nextTier.reward_cents)}) : t('dashboard.consumption.allReached') }}</p>
      </div>
      <p class="mt-2 text-xs text-gray-500">{{ date(campaign.starts_at) }} — {{ date(campaign.ends_at) }} · {{ t('dashboard.consumption.beijing') }}</p>
      <p v-if="phase !== 'active'" class="mt-3 rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950/30 dark:text-amber-200">{{ t(`dashboard.consumption.phases.${phase}`) }}</p>
      <progress class="sr-only" :value="Number(summary?.spend || 0)" :max="(campaign.tiers.at(-1)?.threshold_cents || 1)/100" :aria-label="t('dashboard.consumption.progress')"></progress>
      <div class="consumption-stages mt-6" :aria-label="t('dashboard.consumption.progress')">
        <div v-for="tier in campaign.tiers" :key="tier.position" class="consumption-stage" :class="{'is-reached': spendUnits >= thresholdUnits(tier.threshold_cents)}" :style="{'--segment-progress': `${segmentProgress(tier.position)}%`}">
          <span class="consumption-node"><Icon :name="tier.state === 'claimed' ? 'check' : tier.state === 'reserved' ? 'gift' : 'lock'" size="sm" /></span>
          <div><p class="text-sm font-medium text-gray-800 dark:text-dark-100">{{ t('dashboard.consumption.threshold', {amount:money(tier.threshold_cents)}) }}</p><p class="mt-1 text-xs text-gray-500">{{ t('dashboard.consumption.reward', {amount:money(tier.reward_cents)}) }}</p></div>
          <button v-if="tier.state === 'reserved' && !summary?.blocked && !expired" class="btn btn-primary mt-2 text-xs" :disabled="claiming" @click="claim(tier.position)">{{ t('dashboard.consumption.claim', {amount:money(tier.reward_cents)}) }}</button>
          <span v-else class="mt-2 text-xs text-gray-500">{{ t(`dashboard.consumption.states.${tier.state || 'locked'}`) }}</span>
        </div>
      </div>
      <div v-if="summary?.claimable_cents && !expired" class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-4 dark:border-dark-700"><span class="text-sm text-emerald-700">{{ t('dashboard.consumption.claimable', {amount:money(summary.claimable_cents)}) }}</span><button class="btn btn-primary" :disabled="claiming" @click="claim(0)">{{ t('dashboard.consumption.claimAll') }}</button></div>
      <p v-if="summary?.blocked || summary?.budget_blocked" class="mt-4 text-sm text-amber-700 dark:text-amber-300">{{ t(summary.blocked ? 'dashboard.consumption.review' : 'dashboard.consumption.noBudget') }}</p>
      <div class="mt-5 rounded-xl border border-emerald-100 bg-emerald-50/70 p-3 text-xs leading-relaxed text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/20 dark:text-emerald-200">{{ t('dashboard.consumption.rule') }}<br>{{ t('dashboard.consumption.deadline', {date:date(campaign.claim_until)}) }}</div>
    </template>
    <details class="mt-4 text-sm text-gray-600 dark:text-dark-300"><summary class="cursor-pointer">{{ t('dashboard.consumption.balanceAndHistory') }}</summary><p class="mt-3 text-xs">{{ t('dashboard.consumption.funds', {principal:Number(summary?.funds.principal).toFixed(2),gift:Number(summary?.funds.gift).toFixed(2),unverified:Number(summary?.funds.unverified).toFixed(2)}) }}</p><p class="mt-2 text-xs text-gray-500">{{ t('dashboard.consumption.unverified') }}</p><ul class="mt-3 space-y-2"><li v-for="(item,index) in summary?.history" :key="index" class="flex flex-wrap justify-between gap-2 border-b border-gray-100 py-2 dark:border-dark-700"><span>{{ item.campaign }} · {{ t('dashboard.consumption.stage', {n:item.position}) }}</span><span>{{ money(item.reward_cents) }} · {{ t(`dashboard.consumption.states.${item.state}`) }}</span></li></ul></details>
    <p v-if="error" class="mt-3 text-sm text-red-600" role="alert">{{ error }}</p>
  </section>
</template>
<script setup lang="ts">
import {computed,onMounted,onUnmounted,ref} from 'vue'
import {consumptionUnits, thresholdUnits, consumptionDisplay, consumptionGap} from '@/utils/consumptionAmount'
import {useI18n} from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import {consumptionAPI,type ConsumptionSummary} from '@/api/consumption'
import {extractApiErrorMessage} from '@/utils/apiError'
import {useAuthStore} from '@/stores/auth'
const {t,locale}=useI18n();const auth=useAuthStore();const summary=ref<ConsumptionSummary|null>(null);const error=ref('');const claiming=ref(false)
const campaign=computed(()=>summary.value?.campaign)
const money=(cents:number)=>`$${(cents/100).toFixed(2)}`
const date=(value:string)=>new Intl.DateTimeFormat(locale.value,{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hourCycle:'h23'}).format(new Date(value))
// Floor display so $49.99999999 never appears to have met a $50 threshold.
const spendUnits=computed(()=>consumptionUnits(summary.value?.spend||'0'))
const displaySpend=computed(()=>consumptionDisplay(summary.value?.spend||'0'))
const nextTier=computed(()=>campaign.value?.tiers.find(tier=>thresholdUnits(tier.threshold_cents)>spendUnits.value))
function segmentProgress(position:number){const tiers=campaign.value?.tiers||[];const current=tiers[position-1];const previous=tiers[position-2];if(!current||!previous)return 0;return Math.max(0,Math.min(100,(Number(summary.value?.spend||0)*100-previous.threshold_cents)/(current.threshold_cents-previous.threshold_cents)*100))}
const remaining=computed(()=>money(consumptionGap(summary.value?.spend||'0',nextTier.value?.threshold_cents||0)))
const now=ref(Date.now())
const timer=setInterval(()=>{now.value=Date.now()},10000)
onUnmounted(()=>clearInterval(timer))
const expired=computed(()=>!!campaign.value&&now.value>=Date.parse(campaign.value.claim_until))
const phase=computed(()=>{const c=campaign.value;if(!c)return 'ended';if(expired.value)return 'expired';if(now.value>=Date.parse(c.ends_at))return 'ended';if(c.state!=='active')return 'paused';if(now.value<Date.parse(c.starts_at))return 'upcoming';return 'active'})
async function claim(position:number){if(!campaign.value||claiming.value)return;claiming.value=true;error.value='';try{summary.value=await consumptionAPI.claim(campaign.value.id,position);await auth.refreshUser()}catch(e){error.value=extractApiErrorMessage(e,t('dashboard.activity.actionFailed'))}finally{claiming.value=false}}
onMounted(async()=>{try{summary.value=await consumptionAPI.status()}catch(e){error.value=extractApiErrorMessage(e,t('dashboard.activity.loadFailed'))}})
</script>
<style scoped>
.consumption-stages{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.consumption-stage{position:relative;display:flex;flex-direction:column;align-items:center;text-align:center;padding-top:0}.consumption-stage::before{content:'';position:absolute;top:15px;left:-50%;right:50%;height:4px;background:linear-gradient(to right,#34b68d var(--segment-progress,0%),#e5e7eb var(--segment-progress,0%))}.consumption-stage:first-child::before{display:none}.consumption-stage.is-reached::before{background:#34b68d}.consumption-node{position:relative;z-index:1;display:flex;align-items:center;justify-content:center;width:32px;height:32px;border:1px solid #e2e8f0;border-radius:50%;background:hsl(var(--card));color:#94a3b8;margin-bottom:10px}.is-reached .consumption-node{background:#159a74;border-color:#159a74;color:white;box-shadow:0 0 0 4px #e3f8ef}@media(max-width:640px){.consumption-stages{grid-template-columns:1fr;gap:16px}.consumption-stage{display:grid;grid-template-columns:32px minmax(0,1fr) auto;gap:12px;text-align:left;align-items:center}.consumption-node{margin:0}.consumption-stage::before{left:15px;right:auto;width:2px;height:20px;top:-18px}.consumption-stage button,.consumption-stage>span:last-child{margin:0}}
</style>
