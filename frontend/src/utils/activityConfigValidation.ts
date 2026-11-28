import type { AdminActivityConfig } from '@/api/admin/activity'

export type ActivityAmountKey = Exclude<keyof AdminActivityConfig, 'checkin_enabled' | 'checkin_require_recharge' | 'second_recharge_enabled' | 'checkin_month_spent_cents' | 'second_total_spent_cents' | 'total_spent_cents'>
export type ActivityConfigErrors = Partial<Record<ActivityAmountKey, 'moneyPrecision' | 'nonNegativeInteger' | 'positive' | 'coverageRange' | 'capRange' | 'rewardBudget' | 'rewardCap'>>

// Mirrors the server constraints; the server remains authoritative.
export function validateActivityConfig(config: AdminActivityConfig): ActivityConfigErrors {
  const errors: ActivityConfigErrors = {}
  const keys: ActivityAmountKey[] = ['checkin_min_spend_cents', 'checkin_rollout_percent', 'checkin_daily_cents', 'checkin_milestone_cents', 'checkin_monthly_cap_cents', 'checkin_monthly_budget_cents', 'second_recharge_reward_cents', 'second_recharge_total_budget_cents', 'second_recharge_min_pay_cents', 'total_budget_cents', 'user_lifetime_cap_cents']
  for (const key of keys) {
    if (!Number.isSafeInteger(config[key]) || config[key] < 0) errors[key] = 'nonNegativeInteger'
  }
  if (config.checkin_rollout_percent > 100) errors.checkin_rollout_percent = 'coverageRange'
  if (config.checkin_monthly_cap_cents > 1500) errors.checkin_monthly_cap_cents = 'capRange'
  const positive = (key: ActivityAmountKey) => { if (config[key] === 0) errors[key] = 'positive' }
  if (config.checkin_enabled) {
    for (const key of ['checkin_rollout_percent', 'checkin_daily_cents', 'checkin_monthly_cap_cents', 'checkin_monthly_budget_cents'] as const) positive(key)
  }
  if (config.second_recharge_enabled) {
    positive('second_recharge_reward_cents')
    positive('second_recharge_min_pay_cents')
    if (config.second_recharge_total_budget_cents < config.second_recharge_reward_cents || config.second_recharge_total_budget_cents === 0) errors.second_recharge_total_budget_cents = 'rewardBudget'
    if (config.second_recharge_reward_cents > config.user_lifetime_cap_cents) errors.user_lifetime_cap_cents = 'rewardCap'
  }
  if (config.checkin_enabled || config.second_recharge_enabled) {
    positive('user_lifetime_cap_cents')
  }
  return errors
}

const moneyKeys = [
  'checkin_min_spend_cents',
  'checkin_daily_cents', 'checkin_milestone_cents', 'checkin_monthly_cap_cents',
  'checkin_monthly_budget_cents', 'second_recharge_reward_cents', 'second_recharge_total_budget_cents',
  'second_recharge_min_pay_cents', 'total_budget_cents', 'user_lifetime_cap_cents',
  'total_spent_cents', 'checkin_month_spent_cents', 'second_total_spent_cents',
] as const

// Form values use major currency units; API values stay in integer minor units.
export function activityConfigToForm(config: AdminActivityConfig): AdminActivityConfig {
  const form = { ...config }
  for (const key of moneyKeys) form[key] = config[key] / 100
  return form
}

export function activityConfigToApi(form: AdminActivityConfig): AdminActivityConfig {
  const config = { ...form }
  for (const key of moneyKeys) {
    const value = form[key]
    const scaled = value * 100
    // Reject empty inputs and fractions smaller than a cent instead of rounding rewards silently.
    config[key] = typeof value === 'number' && Number.isFinite(value) && Math.abs(scaled - Math.round(scaled)) < 1e-7
      ? Math.round(scaled) : Number.NaN
  }
  return config
}
