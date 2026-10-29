import { describe, expect, it } from 'vitest'
import { accountStatsDurationSeconds, accountStatsTrendHistory, formatAccountStatsDuration } from '../accountStatsDuration'
import type { AccountUsageHistory } from '@/types'

describe('account duration display', () => {
  it('distinguishes missing samples from a measured zero and supports older responses', () => {
    expect(formatAccountStatsDuration({ avg_duration_ms: 0, duration_sample_count: 0 })).toBe('—')
    expect(formatAccountStatsDuration({ avg_duration_ms: 100 })).toBe('—')
    expect(formatAccountStatsDuration({ avg_duration_ms: 0, duration_sample_count: 1 })).toBe('0ms')
    expect(formatAccountStatsDuration({ avg_duration_ms: 150, duration_sample_count: 2 })).toBe('150ms')
    expect(formatAccountStatsDuration({ avg_duration_ms: 1250, duration_sample_count: 2 })).toBe('1.25s')
  })

  it('uses seconds for the curve and keeps missing samples as gaps rather than zeros', () => {
    expect([
      { avg_duration_ms: 1500, duration_sample_count: 2 },
      { avg_duration_ms: 0, duration_sample_count: 0 },
      { avg_duration_ms: 0, duration_sample_count: 1 },
      {}
    ].map(accountStatsDurationSeconds)).toEqual([1.5, null, 0, null])

    const history = [{ date: '2026-10-03', label: '10/03', requests: 2, tokens: 0, cost: 0, actual_cost: 0, user_cost: 0, avg_duration_ms: 1500, duration_sample_count: 2 }] as AccountUsageHistory[]
    const daily = accountStatsTrendHistory(history, 'day', '2026-10-01', '2026-10-03')
    expect(daily.map((point) => point.date)).toEqual(['2026-10-01', '2026-10-02', '2026-10-03'])
    expect(daily.map(accountStatsDurationSeconds)).toEqual([null, null, 1.5])
    expect(history).toHaveLength(1)
    expect(accountStatsTrendHistory(history, 'hour', '2026-10-01', '2026-10-03')).toBe(history)
  })
})
