import { effectScope, nextTick, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { useAccountStats } from '../useAccountStats'
import type { AccountUsageStatsResponse } from '@/types'

const { getStats } = vi.hoisted(() => ({ getStats: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { getStats } } }))

const scopes: ReturnType<typeof effectScope>[] = []
function setup() {
  const show = ref(true)
  const id = ref<number | undefined>(7)
  const scope = effectScope()
  scopes.push(scope)
  const state = scope.run(() => useAccountStats(() => show.value, () => id.value))!
  return { show, id, ...state }
}
const result = (granularity: 'day' | 'hour') => ({ granularity, history: [] }) as unknown as AccountUsageStatsResponse

describe('useAccountStats', () => {
  beforeEach(() => getStats.mockReset())
  afterEach(() => {
    scopes.splice(0).forEach((scope) => scope.stop())
    vi.useRealTimers()
  })

  it('loads a visible account immediately and reloads the chosen range and interval', async () => {
    getStats.mockResolvedValue(result('day'))
    const state = setup()
    await flushPromises()
    expect(state.includeTests.value).toBe(true)
    expect(getStats).toHaveBeenLastCalledWith(7, 30, expect.objectContaining({ granularity: 'day', includeTests: true }))
    getStats.mockResolvedValue(result('hour'))
    state.days.value = 1
    state.granularity.value = 'hour'
    await flushPromises()
    expect(getStats).toHaveBeenLastCalledWith(7, 1, expect.objectContaining({ granularity: 'hour' }))
    expect(state.stats.value?.granularity).toBe('hour')
    state.includeTests.value = false
    await flushPromises()
    expect(getStats).toHaveBeenLastCalledWith(7, 1, expect.objectContaining({ granularity: 'hour', includeTests: false }))
  })

  it('cancels old requests and ignores late results after switching or closing', async () => {
    let resolveOld!: (data: AccountUsageStatsResponse) => void
    getStats.mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
    const state = setup()
    const firstSignal = getStats.mock.calls[0][2].signal as AbortSignal
    getStats.mockResolvedValueOnce(result('hour'))
    state.granularity.value = 'hour'
    await flushPromises()
    expect(firstSignal.aborted).toBe(true)
    resolveOld(result('day'))
    await flushPromises()
    expect(state.stats.value?.granularity).toBe('hour')
    const currentSignal = getStats.mock.calls[1][2].signal as AbortSignal
    state.show.value = false
    await nextTick()
    expect(currentSignal.aborted).toBe(true)
    expect(state.stats.value).toBeNull()
    expect(state.loading.value).toBe(false)
  })

  it('clears failures on retry and reloads when the account changes', async () => {
    getStats.mockRejectedValueOnce(new Error('unavailable'))
    const state = setup()
    await flushPromises()
    expect(state.error.value).toBe(true)
    getStats.mockResolvedValue(result('day'))
    state.id.value = 8
    await flushPromises()
    expect(state.error.value).toBe(false)
    expect(getStats).toHaveBeenLastCalledWith(8, 30, expect.anything())
  })

  it('loads inclusive custom dates atomically and skips invalid ranges', async () => {
    getStats.mockResolvedValue(result('day'))
    const state = setup()
    await flushPromises()
    state.setDateRange({ startDate: '2026-10-01', endDate: '2026-10-03' })
    await flushPromises()
    expect(getStats).toHaveBeenCalledTimes(2)
    expect(getStats).toHaveBeenLastCalledWith(7, 30, expect.objectContaining({
      startDate: '2026-10-01', endDate: '2026-10-03'
    }))
    state.includeTests.value = false
    await flushPromises()
    expect(getStats).toHaveBeenLastCalledWith(7, 30, expect.objectContaining({
      startDate: '2026-10-01', endDate: '2026-10-03', includeTests: false
    }))
    const count = getStats.mock.calls.length
    for (const range of [
      { startDate: '2026-10-03', endDate: '2026-10-01' },
      { startDate: '2026-01-01', endDate: '2026-04-01' },
      { startDate: '', endDate: '' },
      { startDate: '2026-02-30', endDate: '2026-03-01' }
    ]) {
      state.setDateRange(range)
      await flushPromises()
      expect(state.invalidRange.value).toBe(true)
      expect(state.stats.value).toBeNull()
    }
    expect(getStats).toHaveBeenCalledTimes(count)
    state.setDateRange({ startDate: '2026-01-01', endDate: '2026-03-31' })
    await flushPromises()
    expect(state.invalidRange.value).toBe(false)
    expect(getStats).toHaveBeenCalledTimes(count + 1)
  })

  it('refreshes rolling dates after midnight and queries exactly the displayed dates', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 29, 23, 50))
    getStats.mockResolvedValue(result('day'))
    const state = setup()
    await flushPromises()
    expect(state.endDate.value).toBe('2026-10-29')
    expect(getStats).toHaveBeenLastCalledWith(7, 30, expect.objectContaining({
      startDate: state.startDate.value, endDate: state.endDate.value
    }))
    state.show.value = false
    await nextTick()
    vi.setSystemTime(new Date(2026, 9, 30, 0, 10))
    state.show.value = true
    await flushPromises()
    expect(state.startDate.value).toBe('2026-10-01')
    expect(state.endDate.value).toBe('2026-10-30')
    expect(getStats).toHaveBeenLastCalledWith(7, 30, expect.objectContaining({
      startDate: state.startDate.value, endDate: state.endDate.value
    }))

    state.setDateRange({ startDate: '2026-10-01', endDate: '2026-10-03' })
    await flushPromises()
    state.show.value = false
    await nextTick()
    vi.setSystemTime(new Date(2026, 10, 1, 12))
    state.show.value = true
    await flushPromises()
    expect(state.endDate.value).toBe('2026-10-03')
    expect(getStats).toHaveBeenLastCalledWith(7, 30, expect.objectContaining({
      startDate: '2026-10-01', endDate: '2026-10-03'
    }))
  })
})
