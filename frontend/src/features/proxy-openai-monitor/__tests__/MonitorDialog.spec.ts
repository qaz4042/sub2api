import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MonitorDialog from '../MonitorDialog.vue'
import { loadReport, loadSettings, saveConfig, type Report } from '../api'
import type { Proxy } from '@/types'
vi.mock('../api', () => ({ loadReport: vi.fn(), loadSettings: vi.fn(), saveConfig: vi.fn() }))
vi.mock('vue-chartjs', () => ({ Line: { template: '<div />' } }))
const fixture = (count: number): Report => ({
  config: { enabled: false, interval_seconds: 30, runtime_state: 'paused' }, eligible: true,
  summary: { count, successes: 0, failures: count, success_rate: count ? 0 : null, average_ms: null, p95_ms: null },
  latest: null, stale: true, stale_seconds: 106, from: '2026-10-29T00:00:00Z', to: '2026-10-29T01:00:00Z',
  first_sample_at: null, last_sample_at: null, bucket_seconds: 60, trend: [], records: [], page: 1, page_size: 50, truncated: false
})
function render() {
  return mount(MonitorDialog, {
    props: { proxy: { id: 1, name: 'test' } as Proxy },
    global: { plugins: [createI18n({ legacy: false, locale: 'zh' })], stubs: {
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }
    } }
  })
}
beforeEach(() => {
 vi.resetAllMocks()
 vi.mocked(loadSettings).mockResolvedValue({config:{enabled:false,interval_seconds:30,runtime_state:'paused'},eligible:true})
})
describe('monitor display states', () => {
  it('displays empty and all-failure history without inventing zero latency', async () => {
    vi.mocked(loadReport).mockResolvedValueOnce(fixture(0)).mockResolvedValueOnce(fixture(3))
    const wrapper = render()
    expect(loadReport).not.toHaveBeenCalled()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('该区间暂无样本')
    const refresh = wrapper.findAll('button').find(b => b.text() === '刷新')!
    await refresh.trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('该区间样本全部失败')
    expect(wrapper.text()).toContain('0.0%')
    expect(wrapper.findAll('strong').map(n => n.text())).toEqual(['3', '0.0%', '—', '—', '3'])
    wrapper.unmount()
  })
  it('shows loading and a retryable API failure', async () => {
    let reject!: (error: Error) => void
    vi.mocked(loadReport).mockImplementation(() => new Promise((_, fail) => { reject = fail }))
    const wrapper = render()
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('加载中')
    reject(new Error('offline')); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('请求失败')
    expect(wrapper.findAll('button').find(b => b.text() === '刷新')!.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
})

it('keeps pause controls usable while history is unavailable',async()=>{
 vi.mocked(loadSettings).mockResolvedValue({config:{enabled:true,interval_seconds:10,runtime_state:'waiting'},eligible:true})
 vi.mocked(loadReport).mockRejectedValue(new Error('history timeout'))
 vi.mocked(saveConfig).mockResolvedValue()
 const wrapper=render()
 await wrapper.get('button').trigger('click');await flushPromises()
 expect(wrapper.get('[role="alert"]').text()).toContain('请求失败')
 expect(wrapper.get('input[type="checkbox"]').attributes('disabled')).toBeUndefined()
 await wrapper.get('input[type="checkbox"]').setValue(false)
 const save=wrapper.findAll('button').find(b=>b.text()==='保存配置')!
 expect(save.attributes('disabled')).toBeUndefined()
 await save.trigger('click');await flushPromises()
 expect(saveConfig).toHaveBeenCalledWith(1,{enabled:false,interval_seconds:10},expect.any(AbortSignal))
 wrapper.unmount()
})
