import { apiClient } from '@/api/client'

export interface AdminActivityConfig {
  checkin_require_recharge?: boolean
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

export interface ActivityHealth {
  processing_enabled: boolean
  awards_enabled: boolean
  last_usage_log_id: number
  pending_events: number
  failed_events: number
  oldest_pending_at?: string
  last_success_at?: string
  last_error: string
}

export async function getAdminActivityHealth(): Promise<ActivityHealth> {
  const { data } = await apiClient.get<ActivityHealth>('/admin/activity/health')
  return data
}

export async function updateAdminActivityRuntime(processing_enabled: boolean, awards_enabled: boolean): Promise<ActivityHealth> {
  const { data } = await apiClient.put<ActivityHealth>('/admin/activity/runtime', { processing_enabled, awards_enabled })
  return data
}

export interface AdminLotteryTier {
  position: number
  reward_cents: number
  reward?: number
  weight: number
}

export interface AdminLotteryConfig {
  outstanding_tickets?: number
  expected_liability_cents?: number
  tiers_locked?: boolean
  enabled: boolean
  unit_pay_cents: number
  budget_cents: number
  spent_cents: number
  expected_cents: number
  tiers: AdminLotteryTier[]
}

export interface LotteryTicketLedgerItem {
  id: number
  source_type: 'order_grant' | 'order_refund' | 'draw' | 'refund_recovery'
  source_id: string
  order_id?: number
  delta_tickets: number
  created_at: string
}

export interface AdminLotteryDrawItem {
  id: number
  batch_key: string
  draw_count: number
  draw_index: number
  tier_position: number
  reward_cents: number
  reward: number
  created_at: string
}

export interface AdminLotteryUserSummary {
  user_id: number
  net_tickets: number
  remaining_draws: number
  total_won_cents: number
  total_won: number
  draw_blocked: boolean
  wallet: {
    available_cents: number
    available: number
    total_earned_cents: number
    total_earned: number
    total_withdrawn_cents: number
    total_withdrawn: number
    unabsorbed_gift: string
    withdraw_blocked: boolean
  }
  tickets: LotteryTicketLedgerItem[]
  draws: AdminLotteryDrawItem[]
}

export async function getAdminLotteryConfig(): Promise<AdminLotteryConfig> {
  const { data } = await apiClient.get<AdminLotteryConfig>('/admin/activity/lottery')
  return data
}

export async function updateAdminLotteryConfig(config: Omit<AdminLotteryConfig, 'spent_cents' | 'total_spent_cents' | 'expected_cents'>): Promise<AdminLotteryConfig> {
  const { data } = await apiClient.put<AdminLotteryConfig>('/admin/activity/lottery', config)
  return data
}

export async function getAdminLotteryUser(userID: number): Promise<AdminLotteryUserSummary> {
  const { data } = await apiClient.get<AdminLotteryUserSummary>(`/admin/activity/lottery/users/${userID}`)
  return data
}
