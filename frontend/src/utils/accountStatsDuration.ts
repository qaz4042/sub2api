import type { AccountUsageSummary } from '@/types'

export function formatAccountStatsDuration(summary: Pick<AccountUsageSummary, 'avg_duration_ms' | 'duration_sample_count'>): string {
  if (!summary.duration_sample_count || !Number.isFinite(summary.avg_duration_ms)) return '—'
  const ms = summary.avg_duration_ms
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`
}
