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
  afterEach(() => scopes.splice(0).forEach((scope) => scope.stop()))

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
})
