import { describe, expect, it, vi } from 'vitest'
const { get } = vi.hoisted(() => ({ get: vi.fn().mockResolvedValue({ data: {} }) }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))
import { getStats } from '../admin/accounts'

describe('account statistics API', () => {
  it('defaults to daily and forwards hourly selection and cancellation', async () => {
    await getStats(7)
    expect(get).toHaveBeenLastCalledWith('/admin/accounts/7/stats', {
      params: { days: 30, granularity: 'day', include_tests: true }, signal: undefined
    })
    const controller = new AbortController()
    await getStats(7, 1, { granularity: 'hour', includeTests: false, signal: controller.signal })
    expect(get).toHaveBeenLastCalledWith('/admin/accounts/7/stats', {
      params: { days: 1, granularity: 'hour', include_tests: false }, signal: controller.signal
    })
  })
  it('sends custom calendar dates instead of a rolling day count', async () => {
    await getStats(7, 30, { startDate: '2026-10-01', endDate: '2026-10-03' })
    expect(get).toHaveBeenLastCalledWith('/admin/accounts/7/stats', {
      params: { start_date: '2026-10-01', end_date: '2026-10-03', granularity: 'day', include_tests: true },
      signal: undefined
    })
  })
})
