import { apiClient } from './client'

export interface ActivityStatus {
  checkin_visible?: boolean
  checkin_eligible: boolean
  checkin_min_spend: number
  eligible: boolean
  first_call_at?: string
  checkin_enabled: boolean
  checked_today: boolean
  streak: number
  next_milestone: number
  month_reward: number
  month_cap: number
  checked_days: string[]
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

export async function claimSecondRecharge(): Promise<{ credited: boolean }> {
  const { data } = await apiClient.post<{ credited: boolean }>('/activity/second-recharge/claim')
  return data
}
