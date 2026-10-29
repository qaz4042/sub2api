import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { loadOverview, type Overview, type OverviewRow } from '../api'
import { monitorStatus, useOverview } from '../overview-state'
vi.mock('../api', () => ({ loadOverview: vi.fn() }))
const fixture = (total = 1): Overview => ({ rows: [], total, page: 1, page_size: 50, from: '2026-10-28T00:00:00Z', to: '2026-10-29T00:00:00Z' })
function setup() {
  let state!: ReturnType<typeof useOverview>
  const wrapper = mount(defineComponent({ setup() { state = useOverview(); return () => null } }))
  return { state, wrapper }
}
beforeEach(() => { vi.useFakeTimers(); vi.resetAllMocks(); vi.spyOn(document, 'hidden', 'get').mockReturnValue(false) })
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })
describe('overview refresh', () => {
  it('loads, polls only while visible, and cancels on leave', async () => {
    vi.mocked(loadOverview).mockResolvedValue(fixture())
    const { state, wrapper } = setup(); await flushPromises()
    expect(state.data.value?.total).toBe(1)
    await vi.advanceTimersByTimeAsync(15000)
    expect(loadOverview).toHaveBeenCalledTimes(2)
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(30000)
    expect(loadOverview).toHaveBeenCalledTimes(2)
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    expect(loadOverview).toHaveBeenCalledTimes(3)
    const signal = vi.mocked(loadOverview).mock.calls[2][1]
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
    await vi.advanceTimersByTimeAsync(30000)
    expect(loadOverview).toHaveBeenCalledTimes(3)
  })
  it('ignores stale responses after changing range and resets pagination', async () => {
    let resolveOld!: (result: Overview) => void
    vi.mocked(loadOverview).mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve })).mockResolvedValue(fixture(2))
    const { state, wrapper } = setup()
    const oldSignal = vi.mocked(loadOverview).mock.calls[0][1]
    state.query.hours = 1; state.query.page = 2; state.reset(); await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    expect(loadOverview).toHaveBeenLastCalledWith(expect.objectContaining({ hours: 1, page: 1 }), expect.any(AbortSignal))
    resolveOld(fixture(9)); await flushPromises()
    expect(state.data.value?.total).toBe(2)
    wrapper.unmount()
  })
  it('keeps last successful data on refresh failure and recovers', async () => {
    vi.mocked(loadOverview).mockResolvedValueOnce(fixture()).mockRejectedValueOnce(new Error('timeout')).mockResolvedValueOnce(fixture(2))
    const { state, wrapper } = setup(); await flushPromises()
    await state.load()
    expect(state.error.value).toBe(true); expect(state.data.value?.total).toBe(1)
    await state.load()
    expect(state.error.value).toBe(false); expect(state.data.value?.total).toBe(2)
    wrapper.unmount()
  })
  it('returns to the last available page after proxy deletion', async () => {
    vi.mocked(loadOverview).mockResolvedValue(fixture())
    const { state, wrapper } = setup(); await flushPromises()
    state.page(2); await flushPromises()
    expect(state.query.page).toBe(1)
    expect(loadOverview).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1 }), expect.any(AbortSignal))
    wrapper.unmount()
  })
})
it('distinguishes paused, unavailable, empty, stale and failed samples', () => {
  const row = { eligible: true, config: { enabled: true }, latest: { success: true }, stale: false } as OverviewRow
  expect(monitorStatus(row)).toBe('success')
  expect(monitorStatus({ ...row, latest: { ...row.latest!, success: false } })).toBe('failed')
  expect(monitorStatus({ ...row, stale: true })).toBe('stale')
  expect(monitorStatus({ ...row, latest: null })).toBe('empty')
  expect(monitorStatus({ ...row, config: { enabled: false, interval_seconds: 30 } })).toBe('paused')
  expect(monitorStatus({ ...row, eligible: false })).toBe('unavailable')
})
