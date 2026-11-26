import { apiClient } from './client'

export interface ActivityWalletLedgerItem {
  id: number
  direction: 'credit' | 'withdraw' | 'revoke'
  kind: string
  amount_cents: number
  wallet_after_cents: number
  user_balance_after?: string
  created_at: string
}

export interface ActivityWalletSummary {
  visible?: boolean
  available_cents: number
  available: number
  reserved_cents?: number
  reserved?: number
  total_gift_cents?: number
  total_gift?: number
  total_earned_cents: number
  total_earned: number
  total_withdrawn_cents: number
  total_withdrawn: number
  unabsorbed_gift: string
  recovery_debt_cents?: number
  withdraw_block_reason?: string
  withdraw_blocked: boolean
  withdrawn_cents?: number
  withdrawn?: number
  user_balance_after?: string
  ledger?: ActivityWalletLedgerItem[]
}

export interface ActivityStatus {
  server_day?: string
  checkin_block_reason?: string
  next_reward_cents?: number
  today_reward_cents?: number
  checkin_visible?: boolean
  checkin_eligible: boolean
  checkin_require_recharge?: boolean
  checkin_has_recharge?: boolean
  checkin_min_spend: number
  eligible: boolean
  first_call_at?: string
  checkin_enabled: boolean
  checked_today: boolean
  daily_reward?: number
  milestone_reward?: number
  streak: number
  next_milestone: number
  month_reward: number
  month_cap: number
  checked_days: string[]
  wallet_balance_cents?: number
  wallet_balance?: number
  wallet_total_earned?: number
  wallet_total_withdrawn?: number
  withdraw_blocked?: boolean
  withdraw_block_reason?: string
  second_recharge_enabled: boolean
  second_recharge_eligible: boolean
  second_recharge_claimed: boolean
  second_recharge_offer: boolean
  second_recharge_reward: number
}

export async function getActivityStatus(): Promise<ActivityStatus> {
  const { data } = await apiClient.get<ActivityStatus>('/activity/status')
  return data
}

export async function checkin(): Promise<ActivityStatus> {
  const { data } = await apiClient.post<ActivityStatus>('/activity/checkin')
  return data
}

export async function getActivityWallet(): Promise<ActivityWalletSummary> {
  const { data } = await apiClient.get<ActivityWalletSummary>('/activity/wallet')
  return data
}

export async function withdrawActivityWallet(): Promise<ActivityWalletSummary> {
  const { data } = await apiClient.post<ActivityWalletSummary>('/activity/wallet/withdraw')
  return data
}

export async function claimSecondRecharge(): Promise<{ credited: boolean }> {
  const { data } = await apiClient.post<{ credited: boolean }>('/activity/second-recharge/claim')
  return data
}

export interface LotteryPublicTier {
  probability?: number
  position: number
  reward_cents: number
  reward: number
}

export interface LotteryStatus {
  visible: boolean
  enabled: boolean
  awards_enabled: boolean
  unit_pay_cents: number
  unit_pay_yuan: number
  ticket_debt?: number
  remaining_draws: number
  total_won_cents: number
  total_won: number
  today_won_cents: number
  today_won: number
  draw_blocked: boolean
  pool_paused: boolean
  ten_draw_paused: boolean
  wallet_balance_cents: number
  wallet_balance: number
  wallet_total_earned: number
  wallet_total_withdrawn: number
  withdraw_blocked: boolean
  tiers: LotteryPublicTier[]
  withdraw_block_reason?: string
}

export interface LotteryDrawItem {
  revoked_at?: string
  id: number
  batch_key: string
  draw_count: number
  draw_index: number
  tier_position: number
  reward_cents: number
  reward: number
  created_at: string
}

export interface LotteryDrawResponse {
  items: LotteryDrawItem[]
  total_reward_cents: number
  total_reward: number
  status: LotteryStatus
}

export interface LotteryHistoryResponse {
  items: LotteryDrawItem[]
  total: number
  page: number
  page_size: number
}

export async function getLotteryStatus(): Promise<LotteryStatus> {
  const { data } = await apiClient.get<LotteryStatus>('/activity/lottery')
  return data
}

export async function drawLottery(count: 1 | 10, idempotencyKey: string): Promise<LotteryDrawResponse> {
  const { data } = await apiClient.post<LotteryDrawResponse>('/activity/lottery/draw', {
    count,
    idempotency_key: idempotencyKey,
  })
  return data
}

export async function getLotteryHistory(page = 1, pageSize = 20): Promise<LotteryHistoryResponse> {
  const { data } = await apiClient.get<LotteryHistoryResponse>('/activity/lottery/history', {
    params: { page, page_size: pageSize },
  })
  return data
}
