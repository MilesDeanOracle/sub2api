/**
 * Pelican test (鹈鹕测智) API.
 *
 * The run endpoint is a raw SSE stream, so it intentionally bypasses the axios
 * client and uses fetch + ReadableStream. Everything else uses the shared client.
 */
import { apiClient } from './client'
import { buildApiUrl } from './url'
import { getLocale } from '@/i18n'
import { readSSE } from '@/lib/pelican/stream'

export interface PelicanTest {
  id: string
  title: string
  prompt_version: string
  prompt: string
}

export interface PelicanConfig {
  default_base_url: string
  fallback_base_url: string
  base_url_source: 'api_base_url' | 'frontend_url' | 'request' | string
  test: PelicanTest
  reasoning_levels: string[]
  limits: {
    max_duration_seconds: number
    max_concurrent_runs: number
  }
}

export interface PelicanModel {
  id: string
  owned_by?: string
}

export interface PelicanModelsResponse {
  base_url: string
  base_url_source: string
  models: PelicanModel[]
  cached: boolean
}

export interface PelicanUsage {
  prompt_tokens?: number
  completion_tokens?: number
  total_tokens?: number
  completion_tokens_details?: {
    reasoning_tokens?: number
  }
}

export interface PelicanRunPayload {
  key_id?: number
  api_key?: string
  model: string
  reasoning_effort?: string
  base_url?: string
  test_id?: string
}

export interface ListModelsOptions {
  keyId?: number
  apiKey?: string
  baseUrl?: string
}

export interface PelicanStreamHandlers {
  onDelta: (text: string) => void
  onUsage?: (usage: PelicanUsage) => void
  /** Gateway request ID; only sent when the base URL is this site. */
  onMeta?: (meta: { requestId: string }) => void
}

export interface PelicanReportPayload {
  run_id: string
  request_id: string
  model: string
  reasoning_effort?: string
  note?: string
  source: string
  duration_ms?: number | null
  ttft_ms?: number | null
  prompt_version?: string
}

export class PelicanHttpError extends Error {
  status: number
  reason: string

  constructor(status: number, message: string, reason = '') {
    super(message)
    this.name = 'PelicanHttpError'
    this.status = status
    this.reason = reason
  }
}

export class PelicanStreamError extends Error {
  code: string

  constructor(code: string, message: string) {
    super(message)
    this.name = 'PelicanStreamError'
    this.code = code
  }
}

export async function getConfig(): Promise<PelicanConfig> {
  const { data } = await apiClient.get<PelicanConfig>('/pelican/config')
  return data
}

export async function listModels(options: ListModelsOptions): Promise<PelicanModelsResponse> {
  const params: Record<string, string | number> = {}
  if (options.keyId) {
    params.key_id = options.keyId
  }
  if (options.baseUrl) {
    params.base_url = options.baseUrl
  }
  const headers: Record<string, string> = {}
  if (options.apiKey) {
    // Header instead of query parameter so a manually entered key does not end
    // up in access-log URLs.
    headers['X-Pelican-Api-Key'] = options.apiKey
  }
  const { data } = await apiClient.get<PelicanModelsResponse>('/pelican/models', {
    params,
    headers
  })
  return data
}

interface PelicanChunk {
  choices?: Array<{
    delta?: {
      content?: string | null
    }
  }>
  usage?: PelicanUsage | null
}

export async function streamRun(
  payload: PelicanRunPayload,
  handlers: PelicanStreamHandlers,
  signal?: AbortSignal
): Promise<void> {
  const response = await fetch(buildApiUrl('/pelican/runs'), {
    method: 'POST',
    credentials: 'include',
    signal,
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      'Accept-Language': getLocale(),
      ...authHeader()
    },
    body: JSON.stringify(payload)
  })

  if (!response.ok) {
    const parsed = await parseErrorResponse(response)
    throw new PelicanHttpError(response.status, parsed.message, parsed.reason)
  }
  if (!response.body) {
    throw new PelicanHttpError(response.status, '接口未返回可读取的流', 'EMPTY_STREAM')
  }

  let completed = false
  await readSSE(response.body, (message) => {
    if (message.event === 'pelican-error') {
      const parsed = parseJSON(message.data)
      throw new PelicanStreamError(
        typeof parsed?.code === 'string' ? parsed.code : 'UPSTREAM_STREAM_ERROR',
        typeof parsed?.message === 'string' ? parsed.message : '上游连接中断，请重试'
      )
    }
    if (message.event === 'pelican-meta') {
      const parsed = parseJSON(message.data)
      if (typeof parsed?.request_id === 'string' && parsed.request_id && handlers.onMeta) {
        handlers.onMeta({ requestId: parsed.request_id })
      }
      return
    }
    if (message.data === '[DONE]') {
      completed = true
      return
    }
    const chunk = parseJSON(message.data) as PelicanChunk | null
    if (!chunk) {
      return
    }
    const delta = chunk.choices?.[0]?.delta?.content
    if (typeof delta === 'string' && delta.length > 0) {
      handlers.onDelta(delta)
    }
    if (chunk.usage && handlers.onUsage) {
      handlers.onUsage(chunk.usage)
    }
  })

  if (!completed) {
    throw new PelicanStreamError('STREAM_INCOMPLETE', '流式响应意外结束，请重试')
  }
}

export async function reportRun(payload: PelicanReportPayload): Promise<{ id: number }> {
  const { data } = await apiClient.post<{ id: number }>('/pelican/reports', payload, { timeout: 60_000 })
  return data
}

function authHeader(): Record<string, string> {
  const token = localStorage.getItem('auth_token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function parseErrorResponse(response: Response): Promise<{ message: string; reason: string }> {
  try {
    const payload = (await response.json()) as { message?: string; reason?: string }
    return {
      message: payload.message || `请求失败 (${response.status})`,
      reason: payload.reason || ''
    }
  } catch {
    return { message: `请求失败 (${response.status})`, reason: '' }
  }
}

function parseJSON(value: string): Record<string, unknown> | null {
  try {
    const parsed: unknown = JSON.parse(value)
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : null
  } catch {
    return null
  }
}
