import { apiClient } from './client'

export const feedbackCategories = ['usage', 'suggestion', 'cooperation', 'other'] as const
export type FeedbackCategory = typeof feedbackCategories[number]
export type FeedbackStatus = 'open' | 'replied' | 'resolved'
export interface FeedbackInput {
  title: string
  category: FeedbackCategory
  content: string
  rating: number
}
export interface FeedbackSnapshot {
  username: string
  email: string
  total_recharged: number
  total_tokens: number
  window_start: string
  window_end: string
  collected_at: string
  usage: { model: string; group_id: number | null; group_name: string; tokens: number }[]
}
export interface FeedbackTicket extends FeedbackInput {
  id: number
  user_id: number
  status: FeedbackStatus
  reply_count: number
  unread_reply_count: number
  enrichment_status: 'pending' | 'ready' | 'failed'
  snapshot?: FeedbackSnapshot
  created_at: string
  updated_at: string
  replies?: { id: number; author_role: 'admin' | 'user'; content: string; created_at: string; read_at?: string | null }[]
}
export interface FeedbackList {
  items: FeedbackTicket[]
  total: number
  page: number
  page_size: number
  category_counts: Partial<Record<FeedbackCategory, number>>
}
export interface FeedbackQuery { page: number; page_size: number; category?: string; status?: string; unread_only?: boolean }
export interface FeedbackSummary { pending_count: number; unread_count: number; latest_unread_reply_id: number }
export interface FeedbackSettings { context_notice: string | null }

export async function submitFeedback(input: FeedbackInput) {
  return (await apiClient.post<{ id: number }>('/feedback', input)).data
}
export async function listFeedback(params: FeedbackQuery, admin = false) {
  return (await apiClient.get<FeedbackList>(admin ? '/admin/feedback' : '/feedback', { params })).data
}
export async function getFeedback(id: number, admin = false) {
  return (await apiClient.get<FeedbackTicket>(`${admin ? '/admin/feedback' : '/feedback'}/${id}`)).data
}
export async function replyFeedback(id: number, content: string, resolved: boolean) {
  await apiClient.post(`/admin/feedback/${id}/replies`, { content, resolved })
}
export async function replyToMyFeedback(id: number, content: string) {
  await apiClient.post(`/feedback/${id}/replies`, { content })
}

export async function getFeedbackSummary(admin = false) {
  return (await apiClient.get<FeedbackSummary>(`${admin ? '/admin/feedback' : '/feedback'}/summary`)).data
}
export async function markFeedbackRead(id: number, replyID: number) {
  await apiClient.post(`/feedback/${id}/read`, { reply_id: replyID })
}
export async function getFeedbackSettings(admin = false) {
  return (await apiClient.get<FeedbackSettings>(`${admin ? '/admin/feedback' : '/feedback'}/settings`)).data
}
export async function updateFeedbackSettings(settings: FeedbackSettings) {
  await apiClient.put('/admin/feedback/settings', settings)
}

// Decimal SI units: 1 M = 1,000,000 tokens; 1 B = 1,000,000,000 tokens.
export function formatFeedbackTokens(tokens: number) {
  return tokens >= 1e9 ? `${(tokens / 1e9).toFixed(2)} B` : `${(tokens / 1e6).toFixed(2)} M`
}
