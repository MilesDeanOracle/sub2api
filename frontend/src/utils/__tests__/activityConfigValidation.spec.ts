import { describe, expect, it } from 'vitest'
import type { AdminActivityConfig } from '@/api/admin/activity'
import { activityConfigToApi, activityConfigToForm, validateActivityConfig } from '../activityConfigValidation'

const config = (): AdminActivityConfig => ({
  checkin_min_spend_cents: 1, checkin_enabled: true, checkin_rollout_percent: 50, checkin_daily_cents: 10,
  checkin_milestone_cents: 50, checkin_monthly_cap_cents: 1500, checkin_monthly_budget_cents: 0,
  second_recharge_enabled: true, second_recharge_reward_cents: 0, second_recharge_total_budget_cents: 0,
  second_recharge_min_pay_cents: 5000, total_budget_cents: 0, user_lifetime_cap_cents: 0,
  total_spent_cents: 0, checkin_month_spent_cents: 0, second_total_spent_cents: 0,
})
describe('activity config validation', () => {
  it('identifies the missing budgets and rewards from the reported configuration', () => {
    expect(validateActivityConfig(config())).toEqual({
      checkin_monthly_budget_cents: 'positive', second_recharge_reward_cents: 'positive',
      second_recharge_total_budget_cents: 'rewardBudget', total_budget_cents: 'positive', user_lifetime_cap_cents: 'positive',
    })
  })
  it('permits disabled activities with zero budgets and rejects empty numeric values', () => {
    const value = { ...config(), checkin_enabled: false, second_recharge_enabled: false }
    expect(validateActivityConfig(value)).toEqual({})
    value.checkin_daily_cents = '' as unknown as number
    expect(validateActivityConfig(value).checkin_daily_cents).toBe('nonNegativeInteger')
  })
  it('accepts a funded check-in without requiring a disabled second recharge reward', () => {
    expect(validateActivityConfig({ ...config(), second_recharge_enabled: false, checkin_monthly_budget_cents: 10000, total_budget_cents: 10000, user_lifetime_cap_cents: 1500 })).toEqual({})
  })
  it('rejects a second recharge reward that exceeds its budget and user cap', () => {
    const errors = validateActivityConfig({ ...config(), second_recharge_reward_cents: 500, second_recharge_total_budget_cents: 100, user_lifetime_cap_cents: 100 })
    expect(errors.second_recharge_total_budget_cents).toBe('rewardBudget')
    expect(errors.user_lifetime_cap_cents).toBe('rewardCap')
  })
})

describe('activity currency conversion', () => {
  it('displays dollars and yuan while preserving API amounts on round trip', () => {
    const original = config()
    const form = activityConfigToForm(original)
    expect(form.checkin_min_spend_cents).toBe(0.01)
    expect(form.checkin_daily_cents).toBe(0.10)
    expect(form.checkin_milestone_cents).toBe(0.50)
    expect(form.checkin_monthly_cap_cents).toBe(15)
    expect(form.second_recharge_min_pay_cents).toBe(50)
    expect(form.checkin_rollout_percent).toBe(50)
    expect(activityConfigToApi(form)).toEqual(original)
  })
  it('handles floating point fractions and rejects sub-cent or empty amounts', () => {
    const form = activityConfigToForm(config())
    form.checkin_daily_cents = 0.29
    expect(activityConfigToApi(form).checkin_daily_cents).toBe(29)
    form.checkin_daily_cents = 0.001
    expect(activityConfigToApi(form).checkin_daily_cents).toBeNaN()
    form.checkin_daily_cents = '' as unknown as number
    expect(activityConfigToApi(form).checkin_daily_cents).toBeNaN()
  })
})

it('allows a zero check-in spend threshold and rejects negative thresholds', () => {
  const value = { ...config(), checkin_enabled: false, second_recharge_enabled: false, checkin_min_spend_cents: 0 }
  expect(validateActivityConfig(value)).toEqual({})
  expect(validateActivityConfig({ ...value, checkin_min_spend_cents: -1 }).checkin_min_spend_cents).toBe('nonNegativeInteger')
})
