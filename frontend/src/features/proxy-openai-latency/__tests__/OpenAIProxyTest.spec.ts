import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpenAIProxyTest from '../OpenAIProxyTest.vue'
import { loadOpenAILatency, testOpenAILatency } from '../api'
import type { Proxy } from '@/types'

vi.mock('../api', () => ({ loadOpenAILatency: vi.fn(), testOpenAILatency: vi.fn() }))
const proxy = { id: 11, protocol: 'http', host: 'proxy.example', port: 8080 } as Proxy
const success = { success: true, latency_ms: 123, http_status: 401, message: 'Reachable', checked_at: '2026-10-26T00:00:00Z' }
function render() {
  return mount(OpenAIProxyTest, { props: { proxy }, global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] } })
}
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(loadOpenAILatency).mockResolvedValue(null)
})
describe('OpenAI proxy latency', () => {
  it('loads independent history without a per-row test button', async () => {
    vi.mocked(loadOpenAILatency).mockResolvedValue(success)
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('123ms')
    expect(wrapper.find('button').exists()).toBe(false)
    expect(testOpenAILatency).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('shows failed connectivity from history', async () => {
    vi.mocked(loadOpenAILatency).mockResolvedValue({ ...success, success: false, latency_ms: undefined, http_status: 403 })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('连接失败')
    wrapper.unmount()
  })
  it('ignores a stale response when switching proxies', async () => {
    let resolveOld!: (value: typeof success) => void
    vi.mocked(loadOpenAILatency).mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const wrapper = render()
    await wrapper.setProps({ proxy: { ...proxy, id: 12 } })
    await flushPromises()
    resolveOld(success)
    await flushPromises()
    expect(wrapper.text()).not.toContain('123ms')
    wrapper.unmount()
  })
  it('directs history load failures to the batch button', async () => {
    vi.mocked(loadOpenAILatency).mockRejectedValue(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('顶部批量测试')
    expect(wrapper.find('button').exists()).toBe(false)
    wrapper.unmount()
  })
})
