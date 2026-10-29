import { describe, expect, it } from 'vitest'
import { formatAccountStatsDuration } from '../accountStatsDuration'

describe('account duration display', () => {
  it('distinguishes missing samples from a measured zero and supports older responses', () => {
    expect(formatAccountStatsDuration({ avg_duration_ms: 0, duration_sample_count: 0 })).toBe('—')
    expect(formatAccountStatsDuration({ avg_duration_ms: 100 })).toBe('—')
    expect(formatAccountStatsDuration({ avg_duration_ms: 0, duration_sample_count: 1 })).toBe('0ms')
    expect(formatAccountStatsDuration({ avg_duration_ms: 150, duration_sample_count: 2 })).toBe('150ms')
    expect(formatAccountStatsDuration({ avg_duration_ms: 1250, duration_sample_count: 2 })).toBe('1.25s')
  })
})
