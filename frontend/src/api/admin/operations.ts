import { apiClient } from '../client'

export interface UpstreamBalance {
  base_url: string
  account_id: number
  account_name: string
  account_count: number
  balance: number | null
  unit: string
  kind: 'wallet' | 'quota' | 'subscription' | 'remaining' | 'unknown'
  status: 'ok' | 'failed' | 'unsupported'
  error?: string
  http_status?: number
  queried_at?: string
}

export async function getUpstreamBalances(signal?: AbortSignal): Promise<UpstreamBalance[]> {
  const { data } = await apiClient.get<{ items: UpstreamBalance[] }>('/admin/accounts/operations-balances', { signal })
  return data.items
}
