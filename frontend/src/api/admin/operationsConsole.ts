import { apiClient } from '../client'

export interface OperationsGroup { id: number; name: string }
export interface OperationsWallGroup extends OperationsGroup { accounts: OperationsAccount[] }
export interface OperationsWallGroups { current: OperationsWallGroup[]; past: OperationsWallGroup[] }
export interface OperationsGroupCleanup { removed: number; detached: number; failed: number }
export interface OperationsAccount { id: number; name: string; platform: string; protocol: string; groups: OperationsGroup[]; has_ungrouped?: boolean }
export interface AccountChoice { account_id: number; model_id: string }
export interface OperationsTask {
  id: number; name: string; enabled: boolean; interval_hours: number; account_configs: AccountChoice[]
  reasoning_effort: string; prompt: string; storage_source: string; config_version: number
  last_triggered_at: string | null; next_run_at: string | null; progress?: Record<string, number>
}
export interface OperationsResult {
  id: number; task_id: number; run_id: string; account_id: number; task_name: string; account_name: string
  groups: OperationsGroup[]; model_id: string; upstream_model_id: string; reasoning_effort: string
  effective_reasoning?: Record<string, unknown>; status: string; error_message?: string; error_code?: string
  active_round?: boolean
  created_at: string; finished_at: string | null; queued_at: string; expires_at: string
  ttft_ms: number | null; first_text_ms: number | null; tps: number | null
  input_tokens: number | null; output_tokens: number | null; reasoning_tokens: number | null
  upstream_duration_ms: number | null; object_key?: string
}
export interface OperationsStatus { running: number; queued: number; cleanup_failed: number; max_workers: number }
export interface StorageSource { id: string; available: boolean; endpoint: string; bucket: string; prefix: string; error?: string }
export interface ModelCatalog { models: string[]; source: string; queried_at: string; metadata: Record<string, { supported_reasoning_levels?: string[]; reasoning?: boolean }> }
export interface ResultQuery { view: 'latest' | 'history'; scope?: 'current' | 'past'; group_by?: 'account'; group_id?: number; account_id?: number; task_id?: number; status?: string; before_id?: number; before_account_id?: number; limit?: number; from?: string; to?: string }
export interface ResultPage { items: OperationsResult[]; has_more: boolean; next_id: number; next_account_id?: number; account_has_more?: Record<number, boolean> }
const base = '/admin/operations'
export const operationsApi = {
  async accounts() { return (await apiClient.get<OperationsAccount[]>(`${base}/accounts`)).data },
  async models(id: number, refresh = false) { return (await apiClient.get<ModelCatalog>(`${base}/accounts/${id}/models`, { params: { refresh }, timeout: 25000 })).data },
  async storage() { return (await apiClient.get<StorageSource[]>(`${base}/storage-sources`)).data },
  async testStorage(source: string) { return (await apiClient.post(`${base}/storage-sources/${source}/test`, {}, { timeout: 35000 })).data },
  async tasks() { return (await apiClient.get<OperationsTask[]>(`${base}/tasks`)).data },
  async save(task: OperationsTask) {
    const config = { timeout: 180000 }
    return (task.id ? await apiClient.put<OperationsTask>(`${base}/tasks/${task.id}`, task, config) : await apiClient.post<OperationsTask>(`${base}/tasks`, task, config)).data
  },
  async enable(task: OperationsTask) { return (await apiClient.put<OperationsTask>(`${base}/tasks/${task.id}/enabled`, { enabled: !task.enabled, config_version: task.config_version })).data },
  async remove(id: number) { return (await apiClient.delete(`${base}/tasks/${id}`)).data },
  async trigger(id: number, requestId: string) { return (await apiClient.post<{ run_id: string; queued: number; existing: boolean }>(`${base}/tasks/${id}/runs`, { request_id: requestId })).data },
  async results(params: ResultQuery, signal?: AbortSignal) { return (await apiClient.get<ResultPage>(`${base}/results`, { params, signal })).data },
  async filters() { return (await apiClient.get<OperationsAccount[]>(`${base}/results/filters`)).data },
  async groups() { return (await apiClient.get<OperationsWallGroups>(`${base}/results/groups`)).data },
  async cleanGroup(id: number) { return (await apiClient.delete<OperationsGroupCleanup>(`${base}/results/groups/${id}`, { timeout: 180000 })).data },
  async content(id: number) { return (await apiClient.get<{ html: string }>(`${base}/results/${id}/content`)).data.html },
  async status() { return (await apiClient.get<OperationsStatus>(`${base}/status`)).data },
  async run(id: string) { return (await apiClient.get<OperationsResult[]>(`${base}/runs/${id}`)).data }
}
