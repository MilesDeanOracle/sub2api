<template>
  <section class="marketing-rewards" aria-labelledby="lottery-admin-title">
    <form ref="formRef" aria-labelledby="lottery-admin-title" novalidate @submit.prevent="save">
      <header class="page-intro">
        <div>
          <h2 id="lottery-admin-title" style="font-size: 24px">{{ copy.title }}</h2>
          <p class="intro-copy">{{ copy.intro }} · {{ copy.unit }}</p>
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
            :aria-label="copy.switchLabel"
            @change="toggleEnabled"
          />
          <span class="switch-track" aria-hidden="true"></span>
          <span class="font-semibold">{{ form.enabled ? copy.enabled : copy.disabled }}</span>
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
        <section class="tier-story" :aria-label="copy.preview">
          <div class="tier-story-heading">
            <div>
              <h2>{{ copy.preview }}</h2>
              <p>{{ copy.ruleSummary.replace('{yuan}', dollar(form.unit_pay_yuan)) }}</p>
            </div>
            <div class="total-reward">
              <span>{{ copy.expectedCost }}</span>
              <strong data-testid="lottery-expected-ev">
                ${{ expectedDollars.toFixed(2) }} ({{ rebatePercent.toFixed(2) }}%)
              </strong>
            </div>
          </div>
          <div class="tier-track lottery-tier-track" data-testid="lottery-admin-nine-grid">
            <div v-for="(tier, idx) in form.tiers" :key="tier.position" class="tier-stop" :style="{ gridArea: `tier-${tier.position}` }">
              <span class="tier-index">{{ copy.slot.replace('{n}', String(tier.position)) }}</span>
              <span class="tier-node" aria-hidden="true">{{ tier.position }}</span>
              <span class="tier-condition">{{ copy.probability }} <b>{{ tierPercent(idx) }}%</b></span>
              <div class="tier-value">
                <span>{{ copy.prize }}</span>
                <strong>${{ dollar(tier.reward_usd) }}</strong>
              </div>
            </div>
            <div class="lottery-preview-center">{{ copy.drawAction }}</div>
          </div>
          <div class="tier-story-foot">{{ copy.guardRule }}</div>
        </section>

        <div class="workspace-grid">
          <section class="config-surface">
            <div class="surface-title">
              <div>
                <h2>{{ copy.rulesTitle }}</h2>
                <p>{{ copy.pending }}</p>
              </div>
            </div>

            <section class="section-group">
              <h3>{{ copy.grantSection }}</h3>
              <div class="field-grid">
                <div class="field">
                  <div>
                    <label for="lottery-unit-pay">{{ copy.unitPayLabel }}</label>
                    <RewardFieldTip :label="copy.unitPayLabel" :text="copy.unitPayHelp" />
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
                  <small id="lottery-unit-pay-help">{{ copy.unitPayHelp }}</small>
                  <span v-if="errors.unit_pay_yuan" id="lottery-unit-pay-err" class="error-text">
                    {{ errors.unit_pay_yuan }}
                  </span>
                </div>
              </div>
            </section>

            <section class="section-group">
              <h3>{{ copy.tiersSection }}</h3>
              <p class="section-copy">{{ copy.tiersHelp }}</p>
              <p v-if="hasLegacyPool" class="section-copy" role="status">{{ copy.legacyPool }}</p>
              <button type="button" data-testid="lottery-use-defaults" class="action-secondary lottery-defaults" :disabled="saving || accounting?.tiers_locked" @click="useDefaultTiers">{{ copy.useDefaults }}</button>
              <p v-if="accounting?.tiers_locked" class="section-copy" role="status">{{ copy.tiersLocked }}</p>
              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs">
                  <caption class="sr-only">{{ copy.tiersSection }}</caption>
                  <thead>
                    <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                      <th scope="col" class="py-2 pr-3">{{ copy.colSlot }}</th>
                      <th scope="col" class="py-2 pr-3">{{ copy.colReward }}</th>
                      <th scope="col" class="py-2 pr-3">{{ copy.colWeight }}</th>
                      <th scope="col" class="py-2">{{ copy.colRate }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="(tier, idx) in form.tiers"
                      :key="tier.position"
                      class="border-b border-gray-100 dark:border-dark-800"
                    >
                      <td class="py-2.5 pr-3 font-semibold">{{ copy.slot.replace('{n}', String(tier.position)) }}</td>
                      <td class="py-2.5 pr-3">
                        <label class="sr-only" :for="`lottery-reward-${tier.position}`">
                          {{ copy.slot.replace('{n}', String(tier.position)) }} {{ copy.colReward }}
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
                          {{ copy.slot.replace('{n}', String(tier.position)) }} {{ copy.colWeight }}
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
                <span>{{ copy.pending }}</span>
                <button type="submit" class="action-primary" :disabled="saving" :aria-busy="saving">
                  {{ t('dashboard.activity.save') }}
                </button>
              </div>
            </section>
          </section>

          <aside class="side-stack">
            <section class="budget-panel">
              <div class="surface-title"><h2>{{ copy.budgetTitle }}</h2></div>
              <div class="budget-label">{{ copy.spentLabel }}</div>
              <div class="budget-amount">
                ${{ dollar(form.spent_usd) }} <span>/ ${{ dollar(form.budget_usd) }}</span>
              </div>
              <div class="budget-bar" aria-hidden="true">
                <span :style="{ width: `${budgetUsage}%` }"></span>
              </div>
              <div class="field">
                <div>
                  <label for="lottery-budget">{{ copy.budgetField }}</label>
                  <RewardFieldTip :label="copy.budgetField" :text="copy.budgetHelp" />
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
                <small id="lottery-budget-help">{{ copy.budgetHelp }}</small>
                <span v-if="errors.budget_usd" id="lottery-budget-err" class="error-text">
                  {{ errors.budget_usd }}
                </span>
              </div>
              <div class="inline-note">{{ copy.budgetPauseNote }}</div>
              <p class="section-copy">{{ copy.outstanding }}: {{ accounting?.outstanding_tickets ?? 0 }} · {{ copy.liability }}: ${{ ((accounting?.expected_liability_cents ?? 0)/100).toFixed(2) }}</p>
              <p v-if="(accounting?.expected_liability_cents ?? 0) > (Number(form.budget_usd)-form.spent_usd)*100" class="inline-note" role="status">{{ copy.liabilityWarning }}</p>
            </section>
          </aside>
        </div>
      </template>

      <p v-else-if="!failed" class="section-copy" role="status">{{ copy.loading }}</p>
    </form>

    <!-- User Lottery & Ticket Lookup (separate non-nested form) -->
    <details class="accounting-panel">
      <summary class="cursor-pointer text-sm font-semibold text-gray-800 dark:text-dark-100">
        {{ copy.lookupTitle }}
      </summary>
      <p class="mt-2 text-xs leading-relaxed text-gray-500">{{ copy.lookupHelp }}</p>
      <form class="mt-4 flex flex-wrap items-end gap-3" @submit.prevent="lookupUser">
        <div class="input-label">
          <label for="lottery-lookup-uid">{{ copy.userID }}</label>
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
          {{ lookingUp ? t('common.loading') : copy.lookupBtn }}
        </button>
      </form>
      <p v-if="lookupError" class="mt-3 text-xs text-red-600 dark:text-red-400" role="alert">{{ lookupError }}</p>
      <div v-if="userSummary" class="mt-4 space-y-4 text-xs">
        <p class="text-sm text-gray-700 dark:text-dark-200">
          {{ copy.remainingDraws }}: <strong>{{ userSummary.remaining_draws }}</strong> ({{ copy.netTickets }}:
          {{ userSummary.net_tickets }}) · {{ copy.totalWon }}:
          <strong>${{ userSummary.total_won.toFixed(2) }}</strong> · {{ copy.walletAvailable }}:
          <strong>${{ userSummary.wallet.available.toFixed(2) }}</strong>
          <span v-if="userSummary.draw_blocked" class="ml-2 text-amber-600 dark:text-amber-400">
            [{{ copy.frozen }}]
          </span>
        </p>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs text-gray-600 dark:text-dark-300">
            <caption class="sr-only">{{ copy.ticketLedger }}</caption>
            <thead>
              <tr class="border-b border-gray-200 dark:border-dark-700">
                <th scope="col" class="p-2">ID</th>
                <th scope="col" class="p-2">{{ copy.source }}</th>
                <th scope="col" class="p-2">{{ copy.delta }}</th>
                <th scope="col" class="p-2">{{ copy.time }}</th>
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

const { t, locale } = useI18n()
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

const copy = computed(() =>
  locale.value.startsWith('zh')
    ? {
        title: '九宫格 · 充值送抽奖配置',
        intro: '常驻开关型充值回馈，奖励入账活动钱包，退款同步核对并扣回对应奖励',
        unit: '奖金与预算单位为美元（USD），充值门槛为人民币（CNY）',
        preview: '九宫格 · 8 档奖池与单抽期望预览',
        drawAction: '抽奖',
        tiersHelp: '八档奖金与正整数权重统一配置、一起保存，八档全部参与抽奖。',
        legacyPool: '当前历史奖池继续兑现原概率。关闭赠次并结清已有和待同步次数后，可整套切换八档；修改后需点击保存。',
        useDefaults: '填入八档默认配置（单抽期望 $0.629）',
        ruleSummary: '单笔人民币余额充值每实付满 ¥{yuan} 赠送 1 次（向下取整，不追溯开启前历史订单）',
        expectedCost: '单抽期望奖励（1:1 名义折算）',
        tiersLocked: '赠次窗口开启或仍有未结清次数时，奖金与概率锁定；先关闭赠次并结清已有次数后再修改。',
        outstanding: '未使用次数', liability: '未使用次数的期望奖励', liabilityWarning: '剩余预算低于现有次数的期望奖励，请补充预算；预算不足时次数保留，抽奖暂停。',
        slot: '第 {n} 格',
        probability: '中奖率',
        prize: '奖金',
        guardRule:
          '不受个人终身上限限制；当抽奖专用剩余预算低于最高档奖金时自动暂停抽奖并保留次数，绝不暗改概率。',
        rulesTitle: '抽奖规则与奖池权重',
        pending: '下方规则与预算修改后，点击保存配置生效；右上角开关点击即刻生效。',
        switchLabel: '充值抽奖总开关',
        enabled: '活动已开启',
        disabled: '活动已关闭',
        grantSection: '充值赠次条件',
        unitPayLabel: '单次抽奖实付门槛（人民币元）',
        unitPayHelp: '默认 ¥10.00，仅计活动开启期间新完成支付的人民币余额充值订单，单笔向下取整。',
        tiersSection: '8 档奖金与正整数权重（用户可在活动规则中查看概率）',
        colSlot: '格位',
        colReward: '单次奖金（USD）',
        colWeight: '整数权重',
        colRate: '真实概率',
        budgetTitle: '抽奖专用预算',
        spentLabel: '累计已发奖金',
        budgetField: '抽奖专用总预算（USD）',
        budgetHelp: '启用抽奖时，专用剩余预算须至少覆盖 1 次最高档奖金；抽奖预算独立使用。',
        budgetPauseNote: '关闭活动开关后停止对新充值赠次，但已有剩余次数的用户仍可在预算内抽完。',
        loading: '正在加载充值抽奖配置…',
        lookupTitle: '用户抽奖次数与流水查询',
        lookupHelp: '按用户 ID 同步并查看剩余次数、净票券账本、退款扣回记录及活动钱包状态。',
        userID: '用户 ID',
        lookupBtn: '查询用户明细',
        remainingDraws: '可用次数',
        netTickets: '账面净票数',
        totalWon: '累计中奖',
        walletAvailable: '活动钱包可转入',
        frozen: '退款审核冻结中',
        ticketLedger: '最近票券流水',
        source: '来源',
        delta: '变动次数',
        time: '时间',
        errUnitPay: '请输入不少于 ¥1.00 的金额，最多两位小数。',
        errReward: '请输入大于 0 的美元金额，最多两位小数。',
        errWeight: '八档均需配置 1 至 1000000 的整数权重。',
        errBudget: '请输入不小于已发放金额的预算，且启用时须至少覆盖一次最高档奖金。',
        errSummary: '配置未保存，请修正标红字段后重试。',
      }
    : {
        title: 'Nine-Grid · Recharge Lottery',
        intro: 'Always-on switchable recharge lottery credited to the isolated activity wallet',
        unit: 'Prizes & budgets in USD; recharge threshold in CNY',
        preview: 'Nine-grid · 8-tier prize pool & expected cost',
        drawAction: 'Draw',
        tiersHelp: 'Configure and save all eight prize amounts and positive weights together. All eight participate.',
        legacyPool: 'This historical pool keeps its original odds. Close grants and settle existing and unsynced draws before switching the entire pool. Save to apply changes.',
        useDefaults: 'Fill eight-tier defaults ($0.629 expected reward)',
        ruleSummary: '1 draw per ¥{yuan} paid in a single CNY balance order (floored per order; no historical backfill)',
        expectedCost: 'Expected reward / draw (nominal 1:1)',
        tiersLocked: 'Prizes and odds are locked while grants are open or tickets remain. Close grants and settle existing tickets before editing.',
        outstanding: 'Outstanding draws', liability: 'Expected outstanding rewards', liabilityWarning: 'Remaining budget is below expected outstanding rewards. Add budget; draws pause with tickets preserved when underfunded.',
        slot: 'Slot {n}',
        probability: 'Odds',
        prize: 'Prize',
        guardRule:
          'Exempt from per-user lifetime cap; drawing pauses automatically if remaining budget cannot cover the top prize, preserving exact odds.',
        rulesTitle: 'Rules & prize weights',
        pending: 'Save rule and budget edits below; the top-right activity switch applies immediately.',
        switchLabel: 'Recharge lottery switch',
        enabled: 'Activity ON',
        disabled: 'Activity OFF',
        grantSection: 'Grant threshold',
        unitPayLabel: 'CNY paid per draw (¥)',
        unitPayHelp: 'Default ¥10.00. Only completed CNY balance orders settled while enabled qualify.',
        tiersSection: '8 prize amounts & positive integer weights (odds disclosed in rules)',
        colSlot: 'Slot',
        colReward: 'Prize (USD)',
        colWeight: 'Weight',
        colRate: 'Exact odds',
        budgetTitle: 'Dedicated lottery budget',
        spentLabel: 'Total awarded',
        budgetField: 'Lottery budget (USD)',
        budgetHelp: 'When enabled, the dedicated remaining budget must cover at least one top-tier prize. This budget is independent.',
        budgetPauseNote: 'Disabling stops new ticket grants while allowing users to spend existing draws within budget.',
        loading: 'Loading recharge lottery settings…',
        lookupTitle: 'User ticket & draw lookup',
        lookupHelp: 'Sync and inspect a user’s remaining draws, net ticket ledger, refund deductions and activity wallet.',
        userID: 'User ID',
        lookupBtn: 'Look up user',
        remainingDraws: 'Available draws',
        netTickets: 'Net tickets',
        totalWon: 'Total won',
        walletAvailable: 'Wallet withdrawable',
        frozen: 'Frozen for refund review',
        ticketLedger: 'Recent ticket ledger',
        source: 'Source',
        delta: 'Delta',
        time: 'Time',
        errUnitPay: 'Enter at least ¥1.00 with at most two decimal places.',
        errReward: 'Enter a positive USD amount with at most two decimal places.',
        errWeight: 'All eight tiers require an integer weight from 1 to 1000000.',
        errBudget: 'Enter a valid budget covering spent rewards (and at least one top prize when enabled).',
        errSummary: 'Not saved. Fix the highlighted fields and try again.',
      }
)

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
    nextErrors.unit_pay_yuan = copy.value.errUnitPay
  }

  // Budget-only saves of a locked historical pool must remain possible.
  const unchangedTiers = JSON.stringify(state.tiers.map((tier) => ({ position: tier.position, reward_cents: Math.round(tier.reward_usd * 100), weight: tier.weight }))) === JSON.stringify(savedPayload.value?.tiers)
  let maxRewardCents = 0
  const apiTiers = state.tiers.map((tier) => {
    const rewardCents = validMoneyCents(tier.reward_usd)
    if (rewardCents === null || rewardCents <= 0) {
      nextErrors[`reward_${tier.position}`] = copy.value.errReward
    } else if (tier.weight > 0 && rewardCents > maxRewardCents) {
      maxRewardCents = rewardCents
    }
    if (!Number.isInteger(tier.weight) || (tier.weight <= 0 && !unchangedTiers) || tier.weight < 0 || tier.weight > 1_000_000) {
      nextErrors[`weight_${tier.position}`] = copy.value.errWeight
    }
    return {
      position: tier.position,
      reward_cents: rewardCents ?? 0,
      weight: tier.weight,
    }
  })

  if (!state.tiers.some((tier) => Number.isInteger(tier.weight) && tier.weight > 0)) {
    nextErrors.weight_1 = copy.value.errWeight
  }

  const budgetCents = validMoneyCents(state.budget_usd)
  const spentCents = Math.round(state.spent_usd * 100)
  if (
    budgetCents === null ||
    budgetCents < spentCents ||
    (state.enabled && budgetCents - spentCents < maxRewardCents)
  ) {
    nextErrors.budget_usd = copy.value.errBudget
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
      message.value = copy.value.errSummary
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
    message.value = copy.value.errSummary
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
