import type { MonitorStatus } from '@/api/channelMonitor'

/** 百分比保留一位小数，如 85.0%。 */
export function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`
}

/** 分组倍率的展示文本，如 1.25x。 */
export function formatMultiplier(value: number): string {
  if (!Number.isFinite(value)) return '-'
  const normalized = Number.isInteger(value) ? value.toFixed(0) : value.toFixed(3).replace(/\.?0+$/, '')
  return `${normalized}x`
}

export function multiplierBadgeClass(value: number): string {
  if (!Number.isFinite(value)) {
    return 'bg-gray-100 text-gray-500 ring-gray-200 dark:bg-dark-800 dark:text-dark-300 dark:ring-dark-700'
  }
  if (value <= 0.25) {
    return 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:ring-emerald-900/60'
  }
  if (value < 1) {
    return 'bg-teal-50 text-teal-700 ring-teal-200 dark:bg-teal-950/40 dark:text-teal-300 dark:ring-teal-900/60'
  }
  if (value === 1) {
    return 'bg-slate-50 text-slate-700 ring-slate-200 dark:bg-dark-800 dark:text-dark-200 dark:ring-dark-700'
  }
  if (value <= 2) {
    return 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-900/60'
  }
  return 'bg-rose-50 text-rose-700 ring-rose-200 dark:bg-rose-950/40 dark:text-rose-300 dark:ring-rose-900/60'
}

/** 把公开快照里的监测状态归一到前端使用的四种状态，未知状态按 error 处理。 */
export function normalizeStatus(status: string): MonitorStatus {
  if (status === 'operational' || status === 'degraded' || status === 'failed' || status === 'error') {
    return status
  }
  return 'error'
}
