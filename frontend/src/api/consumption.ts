import { apiClient } from './client'
export interface ConsumptionTier { position: number; threshold_cents: number; reward_cents: number; state?: string }
export interface ConsumptionCampaign {
 id: number; name: string; starts_at: string; ends_at: string; claim_until: string;
 rollout_percent: number; budget_cents: number; state: string; published_at?: string;
 tiers: ConsumptionTier[]; reserved_cents: number; claimed_cents: number; participants: number; qualified_users: number
}
export interface ConsumptionSummary {
 campaign: ConsumptionCampaign | null; spend: string; claimable_cents: number; blocked: boolean; budget_blocked: boolean;
 funds: { principal: string; gift: string; unverified: string };
 history: {campaign: string; position: number; reward_cents: number; state: string; created_at: string}[]
}
export const consumptionAPI = {
 async status() { return (await apiClient.get<ConsumptionSummary>('/activity/consumption')).data },
 async claim(campaign_id: number, position = 0) { return (await apiClient.post<ConsumptionSummary>('/activity/consumption/claim', {campaign_id,position})).data },
 async campaigns() { return (await apiClient.get<ConsumptionCampaign[]>('/admin/activity/consumption/campaigns')).data },
 async save(campaign: ConsumptionCampaign) { return (await apiClient.put<{id:number}>('/admin/activity/consumption/campaigns',campaign)).data },
 async state(id: number,state: string) { await apiClient.post('/admin/activity/consumption/state',{id,state}) },
 async funds(id: number) { return (await apiClient.get<{status:ConsumptionSummary;ledger:Record<string,string|number>[]}>(`/admin/activity/consumption/funds/${id}`)).data },
 async reconcile(user_id: number, amount: string, source: string, note: string) { await apiClient.post('/admin/activity/consumption/reconcile',{user_id,amount,source,note}) },
 async correct(ledger_id: number,note: string) { await apiClient.post('/admin/activity/consumption/correct',{ledger_id,note}) },
 async review(user_id: number, approve: boolean,note: string) { await apiClient.post('/admin/activity/consumption/review',{user_id,approve,note}) },
 async audit() { return (await apiClient.get<Record<string,string|number>[]>('/admin/activity/consumption/audit')).data }
}
