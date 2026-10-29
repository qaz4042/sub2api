import { effectScope } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { useProxyQuality } from '../state'
import { loadSettings, saveSettings, type Settings } from '../api'

vi.mock('../api', () => ({ loadSettings: vi.fn(), saveSettings: vi.fn() }))
const settings = (overrides: Partial<Settings> = {}): Settings => ({
  enabled: false, proxy_ids: [], version: 0, current_proxy_id: 1, reason: 'disabled',
  last_evaluated_at: null, last_switch_at: null, choices: [{ id: 1, name: 'A' }, { id: 2, name: 'B' }], events: [], samples: [], ...overrides
})
function harness() {
  vi.resetAllMocks()
  const scope = effectScope()
  const state = scope.run(useProxyQuality)!
  return { state, stop: () => scope.stop() }
}
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(r => { resolve = r }); return { promise, resolve } }

describe('account proxy quality state', () => {
  it('sends explicit version and reloads the saved configuration', async () => {
    const { state, stop } = harness()
    vi.mocked(loadSettings).mockResolvedValueOnce(settings()).mockResolvedValueOnce(settings({ enabled: true, proxy_ids: [1, 2], version: 1, reason: 'waiting' }))
    await state.load(7)
    state.enabled.value = true; state.ids.value = [1, 2]
    expect(await state.save()).toBe(true)
    expect(saveSettings).toHaveBeenCalledWith(7, { enabled: true, proxy_ids: [1, 2], version: 0 }, expect.any(AbortSignal))
    expect(state.settings.value?.version).toBe(1)
    expect(state.saved.value).toBe(true)
    stop()
  })
  it('ignores a late response after changing accounts', async () => {
    const { state, stop } = harness()
    const first = deferred<Settings>()
    vi.mocked(loadSettings).mockReturnValueOnce(first.promise).mockResolvedValueOnce(settings({ current_proxy_id: 3 }))
    const pending = state.load(1)
    const signal = vi.mocked(loadSettings).mock.calls[0]![1]
    state.close(); await state.load(2)
    first.resolve(settings({ current_proxy_id: 1 })); await pending
    expect(signal.aborted).toBe(true)
    expect(state.settings.value?.current_proxy_id).toBe(3)
    stop()
  })
  it('retains edited values on save conflict and supports disabling an invalid pool', async () => {
    const { state, stop } = harness()
    vi.mocked(loadSettings).mockResolvedValue(settings({ enabled: true, proxy_ids: [1, 2], version: 4 }))
    await state.load(1)
    state.enabled.value = false
    vi.mocked(saveSettings).mockRejectedValue(new Error('conflict'))
    expect(await state.save()).toBe(false)
    expect(state.enabled.value).toBe(false)
    expect(state.error.value).toBe(true)
    expect(state.saved.value).toBe(false)
    stop()
  })
  it('never displays success if saved state cannot be confirmed', async () => {
    const { state, stop } = harness()
    vi.mocked(loadSettings).mockResolvedValueOnce(settings()).mockRejectedValueOnce(new Error('network'))
    await state.load(1)
    expect(await state.save()).toBe(false)
    expect(state.error.value).toBe(true)
    expect(state.saved.value).toBe(false)
    stop()
  })
  it('cancels work when its owning view is disposed', async () => {
    const { state, stop } = harness()
    const first = deferred<Settings>()
    vi.mocked(loadSettings).mockReturnValue(first.promise)
    const pending = state.load(1)
    const signal = vi.mocked(loadSettings).mock.calls[0]![1]
    stop(); first.resolve(settings()); await pending
    expect(signal.aborted).toBe(true)
    expect(state.settings.value).toBeNull()
    expect(state.busy.value).toBe(false)
  })
})
