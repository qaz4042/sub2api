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
  it('loads independent history and updates the displayed latency after testing', async () => {
    vi.mocked(loadOpenAILatency).mockResolvedValue(success)
    vi.mocked(testOpenAILatency).mockResolvedValue({ ...success, latency_ms: 77 })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('123ms')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('77ms')
    expect(testOpenAILatency).toHaveBeenCalledWith(11, expect.any(AbortSignal))
    wrapper.unmount()
  })
  it('shows failed connectivity instead of an older successful result', async () => {
    vi.mocked(loadOpenAILatency).mockResolvedValue(success)
    vi.mocked(testOpenAILatency).mockResolvedValue({ ...success, success: false, latency_ms: undefined, http_status: 403 })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('连接失败')
    expect(wrapper.text()).not.toContain('123ms')
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
  it('allows retry after a request error', async () => {
    vi.mocked(testOpenAILatency).mockRejectedValue(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('请重试')
    expect(wrapper.get('button').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
})
