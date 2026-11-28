<template>
  <section class="marketing-rewards" aria-labelledby="lottery-admin-title">
    <form ref="formRef" aria-labelledby="lottery-admin-title" novalidate @submit.prevent="save">
      <header class="page-intro">
        <div>
          <h2 id="lottery-admin-title" style="font-size: 24px">{{ t('operationsConsole.lotteryConfig.title') }}</h2>
          <p class="intro-copy">{{ t('operationsConsole.lotteryConfig.intro') }} · {{ t('operationsConsole.lotteryConfig.unit') }}</p>
        </div>
        <label
          v-if="form"
          class="enable-control rounded-full border border-slate-200 bg-white px-4 py-1.5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
        >
          <input
            :checked="form.enabled"
            role="switch"
            type="checkbox"
            name="lottery_enabled"
            :disabled="saving"
            :aria-label="t('operationsConsole.lotteryConfig.switchLabel')"
            @change="toggleEnabled"
          />
          <span class="switch-track" aria-hidden="true"></span>
          <span class="font-semibold">{{ form.enabled ? t('operationsConsole.lotteryConfig.enabled') : t('operationsConsole.lotteryConfig.disabled') }}</span>
        </label>
      </header>

      <p
        v-if="message"
        class="reward-status mb-4"
        :class="failed ? 'text-red-600 dark:text-red-400' : 'text-emerald-700 dark:text-emerald-300'"
        :role="failed ? 'alert' : 'status'"
      >
        {{ message }}
      </p>

      <ActivityAudience kind="lottery" />

      <template v-if="form">
        <!-- Eight independent prizes with a central draw action. -->
        <section class="tier-story" :aria-label="t('operationsConsole.lotteryConfig.preview')">
          <div class="tier-story-heading">
            <div>
              <h2>{{ t('operationsConsole.lotteryConfig.preview') }}</h2>
              <p>{{ t('operationsConsole.lotteryConfig.ruleSummary', { yuan: dollar(form.unit_pay_yuan) }) }}</p>
            </div>
            <div class="total-reward">
              <span>{{ t('operationsConsole.lotteryConfig.expectedCost') }}</span>
              <strong data-testid="lottery-expected-ev">
                ${{ expectedDollars.toFixed(2) }} ({{ rebatePercent.toFixed(2) }}%)
              </strong>
            </div>
          </div>
          <div class="tier-track lottery-tier-track" data-testid="lottery-admin-nine-grid">
            <div v-for="(tier, idx) in form.tiers" :key="tier.position" class="tier-stop" :style="{ gridArea: `tier-${tier.position}` }">
              <span class="tier-index">{{ t('operationsConsole.lotteryConfig.slot', { n: tier.position }) }}</span>
              <span class="tier-node" aria-hidden="true">{{ tier.position }}</span>
              <span class="tier-condition">{{ t('operationsConsole.lotteryConfig.probability') }} <b>{{ tierPercent(idx) }}%</b></span>
              <div class="tier-value">
                <span>{{ t('operationsConsole.lotteryConfig.prize') }}</span>
                <strong>${{ dollar(tier.reward_usd) }}</strong>
              </div>
            </div>
            <div class="lottery-preview-center">{{ t('operationsConsole.lotteryConfig.drawAction') }}</div>
          </div>
          <div class="tier-story-foot">{{ t('operationsConsole.lotteryConfig.guardRule') }}</div>
        </section>

        <div class="workspace-grid">
          <section class="config-surface">
            <div class="surface-title">
              <div>
                <h2>{{ t('operationsConsole.lotteryConfig.rulesTitle') }}</h2>
                <p>{{ t('operationsConsole.lotteryConfig.pending') }}</p>
              </div>
            </div>

            <section class="section-group">
              <h3>{{ t('operationsConsole.lotteryConfig.grantSection') }}</h3>
              <div class="field-grid">
                <div class="field">
                  <div>
                    <label for="lottery-unit-pay">{{ t('operationsConsole.lotteryConfig.unitPayLabel') }}</label>
                    <RewardFieldTip :label="t('operationsConsole.lotteryConfig.unitPayLabel')" :text="t('operationsConsole.lotteryConfig.unitPayHelp')" />
                  </div>
                  <div class="input-unit">
                    <span aria-hidden="true">¥</span>
                    <input
                      id="lottery-unit-pay"
                      v-model.number="form.unit_pay_yuan"
                      name="unit_pay_yuan"
                      type="number"
                      min="1"
                      step="0.01"
                      :aria-invalid="!!errors.unit_pay_yuan || undefined"
                      :aria-describedby="errors.unit_pay_yuan ? 'lottery-unit-pay-help lottery-unit-pay-err' : 'lottery-unit-pay-help'"
                      @input="clearError('unit_pay_yuan')"
                    />
                  </div>
                  <small id="lottery-unit-pay-help">{{ t('operationsConsole.lotteryConfig.unitPayHelp') }}</small>
                  <span v-if="errors.unit_pay_yuan" id="lottery-unit-pay-err" class="error-text">
                    {{ errors.unit_pay_yuan }}
                  </span>
                </div>
              </div>
            </section>

            <section class="section-group">
              <h3>{{ t('operationsConsole.lotteryConfig.tiersSection') }}</h3>
              <p class="section-copy">{{ t('operationsConsole.lotteryConfig.tiersHelp') }}</p>
              <p v-if="hasLegacyPool" class="section-copy" role="status">{{ t('operationsConsole.lotteryConfig.legacyPool') }}</p>
              <button type="button" data-testid="lottery-use-defaults" class="action-secondary lottery-defaults" :disabled="saving || accounting?.tiers_locked" @click="useDefaultTiers">{{ t('operationsConsole.lotteryConfig.useDefaults') }}</button>
              <p v-if="accounting?.tiers_locked" class="section-copy" role="status">{{ t('operationsConsole.lotteryConfig.tiersLocked') }}</p>
              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                  <caption class="sr-only">{{ t('operationsConsole.lotteryConfig.tiersSection') }}</caption>
                  <thead>
                    <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                      <th scope="col" class="py-2 pr-3">{{ t('operationsConsole.lotteryConfig.colSlot') }}</th>
                      <th scope="col" class="py-2 pr-3">{{ t('operationsConsole.lotteryConfig.colReward') }}</th>
                      <th scope="col" class="py-2 pr-3">{{ t('operationsConsole.lotteryConfig.colWeight') }}</th>
                      <th scope="col" class="py-2">{{ t('operationsConsole.lotteryConfig.colRate') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="(tier, idx) in form.tiers"
                      :key="tier.position"
                      class="border-b border-gray-100 dark:border-dark-800"
                    >
                      <td class="py-2.5 pr-3 font-semibold">{{ t('operationsConsole.lotteryConfig.slot', { n: tier.position }) }}</td>
                      <td class="py-2.5 pr-3">
                        <label class="sr-only" :for="`lottery-reward-${tier.position}`">
                          {{ t('operationsConsole.lotteryConfig.slot', { n: tier.position }) }} {{ t('operationsConsole.lotteryConfig.colReward') }}
                        </label>
                        <div class="input-unit" style="max-width: 140px">
                          <span aria-hidden="true">$</span>
                          <input
                            :id="`lottery-reward-${tier.position}`"
                            v-model.number="tier.reward_usd"
                            :disabled="saving || accounting?.tiers_locked"
                            :name="`reward_${tier.position}`"
                            type="number"
                            min="0.01"
                            step="0.01"
                            :aria-invalid="!!errors[`reward_${tier.position}`] || undefined"
                            @input="clearError(`reward_${tier.position}`)"
                          />
                        </div>
                        <span v-if="errors[`reward_${tier.position}`]" class="error-text">
                          {{ errors[`reward_${tier.position}`] }}
                        </span>
                      </td>
                      <td class="py-2.5 pr-3">
                        <label class="sr-only" :for="`lottery-weight-${tier.position}`">
                          {{ t('operationsConsole.lotteryConfig.slot', { n: tier.position }) }} {{ t('operationsConsole.lotteryConfig.colWeight') }}
                        </label>
                        <div class="input-unit" style="max-width: 140px">
                          <span aria-hidden="true">#</span>
                          <input
                            :id="`lottery-weight-${tier.position}`"
                            v-model.number="tier.weight"
                            :disabled="saving || accounting?.tiers_locked"
                            :name="`weight_${tier.position}`"
                            type="number"
                            min="1"
                            max="1000000"
                            step="1"
                            :aria-invalid="!!errors[`weight_${tier.position}`] || undefined"
                            @input="clearError(`weight_${tier.position}`)"
                          />
                        </div>
                        <span v-if="errors[`weight_${tier.position}`]" class="error-text">
                          {{ errors[`weight_${tier.position}`] }}
                        </span>
                      </td>
                      <td class="py-2.5 font-medium tabular-nums text-emerald-700 dark:text-emerald-300">
                        {{ tierPercent(idx) }}%
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="form-foot">
                <span>{{ t('operationsConsole.lotteryConfig.pending') }}</span>
                <button type="submit" class="action-primary" :disabled="saving" :aria-busy="saving">
                  {{ t('dashboard.activity.save') }}
                </button>
              </div>
            </section>
          </section>

          <aside class="side-stack">
            <section class="budget-panel">
              <div class="surface-title"><h2>{{ t('operationsConsole.lotteryConfig.budgetTitle') }}</h2></div>
              <div class="budget-label">{{ t('operationsConsole.lotteryConfig.spentLabel') }}</div>
              <div class="budget-amount">
                ${{ dollar(form.spent_usd) }} <span>/ ${{ dollar(form.budget_usd) }}</span>
              </div>
              <div class="budget-bar" aria-hidden="true">
                <span :style="{ width: `${budgetUsage}%` }"></span>
              </div>
              <div class="field">
                <div>
                  <label for="lottery-budget">{{ t('operationsConsole.lotteryConfig.budgetField') }}</label>
                  <RewardFieldTip :label="t('operationsConsole.lotteryConfig.budgetField')" :text="t('operationsConsole.lotteryConfig.budgetHelp')" />
                </div>
                <div class="input-unit">
                  <span aria-hidden="true">$</span>
                  <input
                    id="lottery-budget"
                    v-model.number="form.budget_usd"
                    name="budget_usd"
                    type="number"
                    min="0"
                    step="0.01"
                    :aria-invalid="!!errors.budget_usd || undefined"
                    :aria-describedby="errors.budget_usd ? 'lottery-budget-help lottery-budget-err' : 'lottery-budget-help'"
                    @input="clearError('budget_usd')"
                  />
                </div>
                <small id="lottery-budget-help">{{ t('operationsConsole.lotteryConfig.budgetHelp') }}</small>
                <span v-if="errors.budget_usd" id="lottery-budget-err" class="error-text">
                  {{ errors.budget_usd }}
                </span>
              </div>
              <div class="inline-note">{{ t('operationsConsole.lotteryConfig.budgetPauseNote') }}</div>
              <p class="section-copy">{{ t('operationsConsole.lotteryConfig.outstanding') }}: {{ accounting?.outstanding_tickets ?? 0 }} · {{ t('operationsConsole.lotteryConfig.liability') }}: ${{ ((accounting?.expected_liability_cents ?? 0)/100).toFixed(2) }}</p>
              <p v-if="(accounting?.expected_liability_cents ?? 0) > (Number(form.budget_usd)-form.spent_usd)*100" class="inline-note" role="status">{{ t('operationsConsole.lotteryConfig.liabilityWarning') }}</p>
            </section>
          </aside>
        </div>
      </template>

      <p v-else-if="!failed" class="section-copy" role="status">{{ t('operationsConsole.lotteryConfig.loading') }}</p>
    </form>

    <!-- User Lottery & Ticket Lookup (separate non-nested form) -->
    <details class="accounting-panel">
      <summary class="cursor-pointer text-sm font-semibold text-gray-800 dark:text-dark-100">
        {{ t('operationsConsole.lotteryConfig.lookupTitle') }}
      </summary>
      <p class="mt-2 text-xs leading-relaxed text-gray-500">{{ t('operationsConsole.lotteryConfig.lookupHelp') }}</p>
      <form class="mt-4 flex flex-wrap items-end gap-3" @submit.prevent="lookupUser">
        <div class="input-label">
          <label for="lottery-lookup-uid">{{ t('operationsConsole.lotteryConfig.userID') }}</label>
          <input
            id="lottery-lookup-uid"
            v-model.number="lookupUserID"
            name="lookup_user_id"
            type="number"
            min="1"
            step="1"
            class="input mt-1"
            required
          />
        </div>
        <button type="submit" class="btn btn-secondary" :disabled="lookingUp">
          {{ lookingUp ? t('common.loading') : t('operationsConsole.lotteryConfig.lookupBtn') }}
        </button>
      </form>
      <p v-if="lookupError" class="mt-3 text-xs text-red-600 dark:text-red-400" role="alert">{{ lookupError }}</p>
      <div v-if="userSummary" class="mt-4 space-y-4 text-xs">
        <p class="text-sm text-gray-700 dark:text-dark-200">
          {{ t('operationsConsole.lotteryConfig.remainingDraws') }}: <strong>{{ userSummary.remaining_draws }}</strong> ({{ t('operationsConsole.lotteryConfig.netTickets') }}:
          {{ userSummary.net_tickets }}) · {{ t('operationsConsole.lotteryConfig.totalWon') }}:
          <strong>${{ userSummary.total_won.toFixed(2) }}</strong> · {{ t('operationsConsole.lotteryConfig.walletAvailable') }}:
          <strong>${{ userSummary.wallet.available.toFixed(2) }}</strong>
          <span v-if="userSummary.draw_blocked" class="ml-2 text-amber-600 dark:text-amber-400">
            [{{ t('operationsConsole.lotteryConfig.frozen') }}]
          </span>
        </p>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs text-gray-600 dark:text-dark-300">
            <caption class="sr-only">{{ t('operationsConsole.lotteryConfig.ticketLedger') }}</caption>
            <thead>
              <tr class="border-b border-gray-200 dark:border-dark-700">
                <th scope="col" class="p-2">ID</th>
                <th scope="col" class="p-2">{{ t('operationsConsole.lotteryConfig.source') }}</th>
                <th scope="col" class="p-2">{{ t('operationsConsole.lotteryConfig.delta') }}</th>
                <th scope="col" class="p-2">{{ t('operationsConsole.lotteryConfig.time') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in userSummary.tickets"
                :key="row.id"
                class="border-t border-gray-100 dark:border-dark-800"
              >
                <td class="p-2">#{{ row.id }}</td>
                <td class="p-2">{{ row.source_type }} ({{ row.source_id }})</td>
                <td class="p-2 font-semibold tabular-nums">
                  {{ row.delta_tickets > 0 ? `+${row.delta_tickets}` : row.delta_tickets }}
                </td>
                <td class="p-2">{{ row.created_at }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </details>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ActivityAudience from './ActivityAudience.vue'
import RewardFieldTip from './RewardFieldTip.vue'
import {
  getAdminLotteryConfig,
  getAdminLotteryUser,
  updateAdminLotteryConfig,
  type AdminLotteryConfig as ApiAdminLotteryConfig,
  type AdminLotteryUserSummary,
} from '@/api/admin/activity'
import { extractApiErrorMessage } from '@/utils/apiError'

interface LotteryTierForm {
  position: number
  reward_usd: number
  weight: number
}

interface LotteryFormState {
  enabled: boolean
  unit_pay_yuan: number
  budget_usd: number
  spent_usd: number
  tiers: LotteryTierForm[]
}

const { t } = useI18n()
const formRef = ref<HTMLFormElement | null>(null)
const accounting = ref<ApiAdminLotteryConfig | null>(null)
const form = ref<LotteryFormState | null>(null)
const savedPayload = ref<Omit<ApiAdminLotteryConfig, 'spent_cents' | 'expected_cents'> | null>(null)
const errors = ref<Record<string, string>>({})
const saving = ref(false)
const failed = ref(false)
const message = ref('')

const lookupUserID = ref<number>()
const lookingUp = ref(false)
const lookupError = ref('')
const userSummary = ref<AdminLotteryUserSummary | null>(null)


const dollar = (value: number) => (Number.isFinite(Number(value)) ? Number(value).toFixed(2) : '0.00')

const defaultTiers: LotteryTierForm[] = [
  { position: 1, reward_usd: 0.5, weight: 8738 },
  { position: 2, reward_usd: 1, weight: 961 },
  { position: 3, reward_usd: 2, weight: 150 },
  { position: 4, reward_usd: 3, weight: 100 },
  { position: 5, reward_usd: 5, weight: 40 },
  { position: 6, reward_usd: 10, weight: 9 },
  { position: 7, reward_usd: 20, weight: 1 },
  { position: 8, reward_usd: 50, weight: 1 },
]
const hasLegacyPool = computed(() => accounting.value?.tiers.some((tier) => tier.weight === 0) ?? false)
function useDefaultTiers() {
  if (!form.value || saving.value || accounting.value?.tiers_locked) return
  form.value.tiers = defaultTiers.map((tier) => ({ ...tier }))
  errors.value = {}
  message.value = ''
}

const totalWeight = computed(() =>
  (form.value?.tiers ?? []).reduce((sum, item) => sum + (Number.isFinite(item.weight) && item.weight > 0 ? item.weight : 0), 0)
)

const expectedDollars = computed(() => {
  if (!form.value || totalWeight.value <= 0) return 0
  const weighted = form.value.tiers.reduce(
    (sum, item) => sum + (Number(item.reward_usd) > 0 && item.weight > 0 ? Number(item.reward_usd) * item.weight : 0),
    0
  )
  return weighted / totalWeight.value
})

const rebatePercent = computed(() => {
  const unit = Number(form.value?.unit_pay_yuan)
  if (!Number.isFinite(unit) || unit <= 0) return 0
  return (expectedDollars.value / unit) * 100
})

const budgetUsage = computed(() =>
  form.value && Number(form.value.budget_usd) > 0
    ? Math.min(100, (form.value.spent_usd / Number(form.value.budget_usd)) * 100)
    : 0
)

function tierPercent(idx: number): string {
  const tier = form.value?.tiers[idx]
  if (!tier || totalWeight.value <= 0 || tier.weight <= 0) return '0.00'
  return ((tier.weight / totalWeight.value) * 100).toFixed(2)
}

function toForm(api: ApiAdminLotteryConfig): LotteryFormState {
  accounting.value = api
  return {
    enabled: api.enabled,
    unit_pay_yuan: api.unit_pay_cents / 100,
    budget_usd: api.budget_cents / 100,
    spent_usd: api.spent_cents / 100,
    tiers: api.tiers.map((item) => ({
      position: item.position,
      reward_usd: item.reward_cents / 100,
      weight: item.weight,
    })),
  }
}

function validMoneyCents(v: number): number | null {
  if (!Number.isFinite(v) || v < 0 || Math.abs(v * 100 - Math.round(v * 100)) > 1e-6) {
    return null
  }
  return Math.round(v * 100)
}

function clearError(key: string) {
  delete errors.value[key]
  message.value = ''
}

function snapshotPayload(api: ApiAdminLotteryConfig) {
  return {
    enabled: api.enabled,
    unit_pay_cents: api.unit_pay_cents,
    budget_cents: api.budget_cents,
    tiers: api.tiers.map((item) => ({
      position: item.position,
      reward_cents: item.reward_cents,
      weight: item.weight,
    })),
  }
}

function buildValidatedPayload(state: LotteryFormState) {
  const nextErrors: Record<string, string> = {}

  const unitPayCents = validMoneyCents(state.unit_pay_yuan)
  if (unitPayCents === null || unitPayCents < 100) {
    nextErrors.unit_pay_yuan = t('operationsConsole.lotteryConfig.errUnitPay')
  }

  // Budget-only saves of a locked historical pool must remain possible.
  const unchangedTiers = JSON.stringify(state.tiers.map((tier) => ({ position: tier.position, reward_cents: Math.round(tier.reward_usd * 100), weight: tier.weight }))) === JSON.stringify(savedPayload.value?.tiers)
  let maxRewardCents = 0
  const apiTiers = state.tiers.map((tier) => {
    const rewardCents = validMoneyCents(tier.reward_usd)
    if (rewardCents === null || rewardCents <= 0) {
      nextErrors[`reward_${tier.position}`] = t('operationsConsole.lotteryConfig.errReward')
    } else if (tier.weight > 0 && rewardCents > maxRewardCents) {
      maxRewardCents = rewardCents
    }
    if (!Number.isInteger(tier.weight) || (tier.weight <= 0 && !unchangedTiers) || tier.weight < 0 || tier.weight > 1_000_000) {
      nextErrors[`weight_${tier.position}`] = t('operationsConsole.lotteryConfig.errWeight')
    }
    return {
      position: tier.position,
      reward_cents: rewardCents ?? 0,
      weight: tier.weight,
    }
  })

  if (!state.tiers.some((tier) => Number.isInteger(tier.weight) && tier.weight > 0)) {
    nextErrors.weight_1 = t('operationsConsole.lotteryConfig.errWeight')
  }

  const budgetCents = validMoneyCents(state.budget_usd)
  const spentCents = Math.round(state.spent_usd * 100)
  if (
    budgetCents === null ||
    budgetCents < spentCents ||
    (state.enabled && budgetCents - spentCents < maxRewardCents)
  ) {
    nextErrors.budget_usd = t('operationsConsole.lotteryConfig.errBudget')
  }

  return {
    nextErrors,
    payload: {
      enabled: state.enabled,
      unit_pay_cents: unitPayCents ?? 0,
      budget_cents: budgetCents ?? 0,
      tiers: apiTiers,
    },
  }
}

onMounted(async () => {
  try {
    const loaded = await getAdminLotteryConfig()
    savedPayload.value = snapshotPayload(loaded)
    form.value = toForm(loaded)
  } catch {
    failed.value = true
    message.value = t('dashboard.activity.loadFailed')
  }
})

async function toggleEnabled(event: Event) {
  const input = event.target as HTMLInputElement
  if (!form.value || saving.value) return
  const targetEnabled = input.checked
  const snapshot = savedPayload.value!
  const { nextErrors, payload: draftPayload } = buildValidatedPayload({ ...form.value, enabled: targetEnabled })
  let payload = { ...snapshot, enabled: targetEnabled }
  if (Object.keys(nextErrors).length > 0) {
    if (!targetEnabled && savedPayload.value) {
      payload = { ...savedPayload.value, enabled: false }
      errors.value = {}
    } else {
      input.checked = form.value.enabled
      errors.value = nextErrors
      failed.value = true
      message.value = t('operationsConsole.lotteryConfig.errSummary')
      await nextTick()
      formRef.value?.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()
      return
    }
  } else {
    errors.value = {}
  }

  if (targetEnabled && JSON.stringify(draftPayload) !== JSON.stringify({ ...snapshot, enabled: targetEnabled })) {
    input.checked = form.value.enabled
    failed.value = true
    message.value = t('dashboard.activity.saveBeforeEnable')
    return
  }
  form.value.enabled = targetEnabled
  saving.value = true
  message.value = ''
  try {
    const updated = await updateAdminLotteryConfig(payload)
    accounting.value = updated
    savedPayload.value = snapshotPayload(updated)
    form.value.enabled = updated.enabled
    failed.value = false
    message.value = t('dashboard.activity.saved')
  } catch (e) {
    form.value.enabled = !targetEnabled
    input.checked = form.value.enabled
    failed.value = true
    message.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed'))
  } finally {
    saving.value = false
  }
}

async function save() {
  if (!form.value || saving.value) return
  const { nextErrors, payload } = buildValidatedPayload(form.value)
  errors.value = nextErrors
  if (Object.keys(nextErrors).length > 0) {
    failed.value = true
    message.value = t('operationsConsole.lotteryConfig.errSummary')
    await nextTick()
    formRef.value?.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()
    return
  }

  saving.value = true
  message.value = ''
  try {
    const updated = await updateAdminLotteryConfig(payload)
    accounting.value = updated
    savedPayload.value = snapshotPayload(updated)
    form.value = toForm(updated)
    failed.value = false
    message.value = t('dashboard.activity.saved')
  } catch (e) {
    failed.value = true
    message.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed'))
  } finally {
    saving.value = false
  }
}

async function lookupUser() {
  if (!lookupUserID.value || lookingUp.value) return
  lookingUp.value = true
  lookupError.value = ''
  try {
    userSummary.value = await getAdminLotteryUser(lookupUserID.value)
  } catch (e) {
    lookupError.value = extractApiErrorMessage(e, t('dashboard.activity.actionFailed'))
  } finally {
    lookingUp.value = false
  }
}
</script>

<style scoped src="./marketingRewards.css"></style>
<style src="./marketingRewardsDark.css"></style>

<style scoped>
.marketing-rewards .lottery-tier-track {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-template-areas:
    "tier-1 tier-2 tier-3"
    "tier-8 draw tier-4"
    "tier-7 tier-6 tier-5";
  gap: 12px;
  max-inline-size: 560px;
  margin-inline: auto;
}
.marketing-rewards .lottery-tier-track::before,
.marketing-rewards .lottery-tier-track .tier-stop::after { display: none; }
.marketing-rewards .lottery-tier-track .tier-stop {
  min-inline-size: 0;
  padding: 10px 4px;
  border: 1px solid var(--line);
  border-radius: 10px;
}
.marketing-rewards .lottery-tier-track .tier-value strong { font-size: clamp(18px, 3vw, 28px); }
.marketing-rewards .lottery-tier-track .tier-value { display: block; }
.marketing-rewards .lottery-tier-track .tier-value span { display: block; }
.lottery-defaults { margin-block: 12px; }
.lottery-preview-center {
  grid-area: draw;
  display: grid;
  place-items: center;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--mint);
  color: var(--accent-strong);
  font-size: 18px;
  font-weight: 600;
}
@media (max-width: 560px) {
  .marketing-rewards .tier-story-heading { flex-direction: column; align-items: stretch; gap: 16px; }
  .marketing-rewards .total-reward { text-align: start; }
}
</style>
