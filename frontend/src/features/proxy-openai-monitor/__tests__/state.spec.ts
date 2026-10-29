import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useMonitor } from '../state'
import { loadReport, loadSettings, saveConfig, type Report } from '../api'
vi.mock('../api', () => ({ loadReport: vi.fn(), loadSettings: vi.fn(), saveConfig: vi.fn() }))
const report = (enabled = true) => ({ config: { enabled, interval_seconds: 60 } }) as Report
function setup() {
  let state!: ReturnType<typeof useMonitor>
  const wrapper = mount(defineComponent({ setup() { state = useMonitor(); return () => null } }))
  return { state, wrapper }
}
beforeEach(() => {
 vi.useFakeTimers(); vi.resetAllMocks()
 vi.mocked(loadSettings).mockImplementation(async id => ({config: {enabled:id!==2,interval_seconds:60},eligible:true}))
})
afterEach(() => { vi.useRealTimers() })
describe('monitor requests', () => {
  it('loads config, saves only opt-in fields, and stops refresh after close', async () => {
    vi.mocked(loadReport).mockResolvedValue(report())
    vi.mocked(saveConfig).mockResolvedValue()
    const { state, wrapper } = setup()
    await state.load(1, 24, 1, true)
    expect(state.config).toEqual({ enabled: true, interval_seconds: 60 })
    state.config.enabled = false
    await state.save()
    expect(saveConfig).toHaveBeenCalledWith(1, { enabled: false, interval_seconds: 60 }, expect.any(AbortSignal))
    expect(state.saved.value).toBe(true)
    state.close()
    const calls = vi.mocked(loadReport).mock.calls.length
    await vi.advanceTimersByTimeAsync(30000)
    expect(loadReport).toHaveBeenCalledTimes(calls)
    wrapper.unmount()
  })
  it('aborts obsolete requests and rejects responses even if transport ignores cancellation', async () => {
    let resolveOld!: (result: Report) => void
    vi.mocked(loadReport).mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
      .mockResolvedValueOnce(report(false))
    const { state, wrapper } = setup()
    const old = state.load(1, 24, 1, true)
    const signal = vi.mocked(loadReport).mock.calls[0][3]
    await state.load(2, 1, 1, true)
    expect(signal.aborted).toBe(true)
    resolveOld(report(true))
    await old
    expect(state.report.value?.config.enabled).toBe(false)
    expect(state.config.enabled).toBe(false)
    wrapper.unmount()
  })
  it('recovers initial configuration after a failed first request', async () => {
    vi.mocked(loadReport).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(report())
    const { state, wrapper } = setup()
    await state.load(1, 1, 1, true)
    expect(state.error.value).toBe(true)
    expect(state.report.value).toBeNull()
    await state.load(1, 1, 1)
    expect(state.error.value).toBe(false)
    expect(state.config.interval_seconds).toBe(60)
    wrapper.unmount()
  })
  it('cancels an in-flight save when closed', async () => {
    vi.mocked(loadReport).mockResolvedValue(report())
    let finish!: () => void
    vi.mocked(saveConfig).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const { state, wrapper } = setup()
    await state.load(1, 24, 1, true)
    const saving = state.save()
    const signal = vi.mocked(saveConfig).mock.calls[0][2]
    state.close()
    expect(signal.aborted).toBe(true)
    finish()
    await saving
    expect(state.report.value).toBeNull()
    expect(loadReport).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
})

describe('monitor recovery and pagination', () => {
 it('allows pause when history fails, including after a successful pause save',async()=>{
  vi.mocked(loadReport).mockRejectedValue(new Error('history timeout'))
  vi.mocked(saveConfig).mockResolvedValue()
  const {state,wrapper}=setup()
  await state.load(1,24,1,true)
  expect(state.error.value).toBe(true)
  expect(state.settings.value).not.toBeNull()
  state.config.enabled=false
  await state.save()
  expect(saveConfig).toHaveBeenCalledWith(1,{enabled:false,interval_seconds:60},expect.any(AbortSignal))
  expect(state.saved.value).toBe(true)
  expect(state.error.value).toBe(true)
  wrapper.unmount()
 })
 it('keeps the cutoff for later pages and polling, and renews it on page one or range change',async()=>{
  const cutoff='2026-10-29T00:00:00Z'
  vi.mocked(loadReport).mockImplementation(async(_id,_hours,_page,_signal,until)=>({...report(),to:until||cutoff}))
  const {state,wrapper}=setup()
  await state.load(1,24,1,true)
  await state.load(1,24,2)
  expect(loadReport).toHaveBeenLastCalledWith(1,24,2,expect.any(AbortSignal),cutoff)
  await vi.advanceTimersByTimeAsync(15000)
  expect(loadReport).toHaveBeenLastCalledWith(1,24,2,expect.any(AbortSignal),cutoff)
  await state.load(1,24,1)
  expect(loadReport).toHaveBeenLastCalledWith(1,24,1,expect.any(AbortSignal),undefined)
  await state.load(1,1,1)
  expect(loadReport).toHaveBeenLastCalledWith(1,1,1,expect.any(AbortSignal),undefined)
  wrapper.unmount()
 })
})
