import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Proxy } from '@/types'
import { loadOpenAILatency, testOpenAILatency, type OpenAIProxyLatency } from '../api'
import { createOpenAIProxyTests, provideOpenAIProxyTests } from '../state'
import OpenAIBatchTest from '../OpenAIBatchTest.vue'
import OpenAIProxyTest from '../OpenAIProxyTest.vue'

vi.mock('../api', () => ({ loadOpenAILatency: vi.fn(), testOpenAILatency: vi.fn() }))
const success: OpenAIProxyLatency = { success: true, latency_ms: 88, http_status: 401, message: 'Reachable', checked_at: '2026-10-26T00:00:00Z' }
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(loadOpenAILatency).mockResolvedValue(null)
})

describe('OpenAI batch tests', () => {
  it('limits concurrency to three, deduplicates IDs and continues after failures', async () => {
    const releases: (() => void)[] = []
    let active = 0
    let peak = 0
    vi.mocked(testOpenAILatency).mockImplementation((id) => {
      active++
      peak = Math.max(peak, active)
      return new Promise((resolve, reject) => {
        releases.push(() => {
          active--
          if (id === 2) reject(new Error('network error'))
          else resolve({ ...success, success: id !== 4 })
        })
      })
    })
    const state = createOpenAIProxyTests()
    const run = state.runBatch([1, 2, 3, 4, 5, 5])
    expect(testOpenAILatency).toHaveBeenCalledTimes(3)
    expect(state.batchRunning.value).toBe(true)
    await state.runBatch([6])
    expect(state.total.value).toBe(5)
    releases.splice(0).forEach(release => release())
    await flushPromises()
    expect(testOpenAILatency).toHaveBeenCalledTimes(5)
    expect(state.completed.value).toBe(3)
    expect(state.entry(1).result?.latency_ms).toBe(88)
    releases.splice(0).forEach(release => release())
    await run
    expect(peak).toBe(3)
    expect(state.completed.value).toBe(5)
    expect(state.succeeded.value).toBe(3)
    expect(state.batchRunning.value).toBe(false)
    state.dispose()
  })

  it('reuses an in-flight individual test and does not let history overwrite it', async () => {
    let finishTest!: (value: OpenAIProxyLatency) => void
    let finishHistory!: (value: OpenAIProxyLatency) => void
    vi.mocked(loadOpenAILatency).mockImplementation(() => new Promise(resolve => { finishHistory = resolve }))
    vi.mocked(testOpenAILatency).mockImplementation(() => new Promise(resolve => { finishTest = resolve }))
    const state = createOpenAIProxyTests()
    const history = state.load({ id: 11 } as Proxy)
    const single = state.test(11)
    const batch = state.runBatch([11])
    finishTest(success)
    await single
    finishHistory({ ...success, latency_ms: 999 })
    await history
    await batch
    expect(testOpenAILatency).toHaveBeenCalledTimes(1)
    expect(state.entry(11).result?.latency_ms).toBe(88)
    state.dispose()
  })

  it('updates visible rows and summary from one toolbar click', async () => {
    vi.mocked(testOpenAILatency).mockResolvedValue(success)
    const Host = defineComponent({
      components: { OpenAIBatchTest, OpenAIProxyTest },
      setup() { provideOpenAIProxyTests() },
      template: '<OpenAIBatchTest :proxy-ids="[11,12]" :selected="false" /><OpenAIProxyTest :proxy="{id:11}" /><OpenAIProxyTest :proxy="{id:12}" />'
    })
    const wrapper = mount(Host, { global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] } })
    await flushPromises()
    expect(wrapper.text()).toContain('未勾选时测试当前页')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(testOpenAILatency).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('成功 2，失败 0')
    expect(wrapper.findAll('[role="status"]').filter(node => node.text() === '88ms')).toHaveLength(2)
    wrapper.unmount()
  })

  it('ignores the old measurement when proxy configuration changes during a test', async () => {
    let finish!: (value: OpenAIProxyLatency) => void
    vi.mocked(testOpenAILatency).mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const state = createOpenAIProxyTests()
    const proxy = { id: 11, host: 'old.example' } as Proxy
    await state.load(proxy)
    const single = state.test(11)
    const reload = state.load({ ...proxy, host: 'new.example' })
    finish(success)
    await single
    await reload
    expect(state.entry(11).result).toBeNull()
    expect(state.entry(11).busy).toBe(false)
    state.dispose()
  })

  it('does not start queued tests after disposal', async () => {
    vi.mocked(testOpenAILatency).mockResolvedValue(success)
    const state = createOpenAIProxyTests()
    const run = state.runBatch([1, 2, 3, 4, 5])
    state.dispose()
    await run
    expect(testOpenAILatency).toHaveBeenCalledTimes(3)
  })
})
