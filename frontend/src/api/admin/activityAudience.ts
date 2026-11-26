import { apiClient } from '../client'

export type ActivityKind = 'checkin' | 'consumption' | 'lottery'
export interface ActivityAudience { mode: 'selected' | 'all'; user_ids: number[] }
export const activityAudienceAPI = {
  async get(kind: ActivityKind) { return (await apiClient.get<ActivityAudience>(`/admin/activity/audience/${kind}`)).data },
  async save(kind: ActivityKind, value: ActivityAudience) { await apiClient.put(`/admin/activity/audience/${kind}`, value) }
}
