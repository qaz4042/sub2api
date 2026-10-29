import type { AccountStatsGranularity, AccountUsageHistory } from '@/types'
import { formatLocalDate } from '@/utils/usageDateRange'

interface AccountDurationStats {
  avg_duration_ms?: number
  duration_sample_count?: number
}

export function accountStatsDurationSeconds(point: AccountDurationStats): number | null {
  return point.duration_sample_count && point.avg_duration_ms != null && Number.isFinite(point.avg_duration_ms)
    ? point.avg_duration_ms / 1000 : null
}

export function formatAccountStatsDuration(summary: AccountDurationStats): string {
  const seconds = accountStatsDurationSeconds(summary)
  if (seconds == null) return '—'
  const ms = seconds * 1000
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`
}
// Preserve empty calendar days as gaps in the duration line, without changing summaries.
export function accountStatsTrendHistory(history: AccountUsageHistory[], granularity: AccountStatsGranularity, startDate: string, endDate: string): AccountUsageHistory[] {
  if (granularity === 'hour') return history
  const points = new Map(history.map((point) => [point.date, point]))
  const result: AccountUsageHistory[] = []
  for (const day = new Date(`${startDate}T00:00:00`); formatLocalDate(day) <= endDate; day.setDate(day.getDate() + 1)) {
    const date = formatLocalDate(day)
    result.push(points.get(date) ?? {
      date, label: `${date.slice(5, 7)}/${date.slice(8)}`,
      requests: 0, tokens: 0, cost: 0, actual_cost: 0, user_cost: 0
    })
  }
  return result
}
