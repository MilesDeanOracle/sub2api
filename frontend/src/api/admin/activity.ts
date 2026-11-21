import { apiClient } from '@/api/client'

export interface AdminActivityConfig {
  checkin_min_spend_cents: number
  checkin_enabled: boolean
  checkin_rollout_percent: number
  second_recharge_enabled: boolean
  checkin_daily_cents: number
  checkin_milestone_cents: number
  checkin_monthly_cap_cents: number
  checkin_monthly_budget_cents: number
  second_recharge_reward_cents: number
  second_recharge_total_budget_cents: number
  second_recharge_min_pay_cents: number
  total_budget_cents: number
  user_lifetime_cap_cents: number
  total_spent_cents: number
  checkin_month_spent_cents: number
  second_total_spent_cents: number
}

export async function getAdminActivityConfig(): Promise<AdminActivityConfig> {
  const { data } = await apiClient.get<AdminActivityConfig>('/admin/activity/config')
  return data
}

export async function updateAdminActivityConfig(config: AdminActivityConfig): Promise<AdminActivityConfig> {
  const { data } = await apiClient.put<AdminActivityConfig>('/admin/activity/config', config)
  return data
}

export interface ActivityRefundReview {
  order_id: number
  user_id: number
  order_status: string
  review_status: string
  created_at: string
}

export async function getActivityRefundReviews(): Promise<ActivityRefundReview[]> {
  const { data } = await apiClient.get<ActivityRefundReview[]>('/admin/activity/refund-reviews')
  return data
}

export async function markActivityRefundReviewed(orderID: number): Promise<void> {
  await apiClient.post(`/admin/activity/refund-reviews/${orderID}/review`)
}
