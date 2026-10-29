import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { beforeEach, expect, it, vi } from 'vitest'
import OverviewPage from '../OverviewPage.vue'
import { loadOverview, type Overview } from '../api'
vi.mock('../api', () => ({ loadOverview: vi.fn() }))
function fixture(): Overview {
  return {
    total: 1, page: 1, page_size: 50, from: '2026-10-28T00:00:00Z', to: '2026-10-29T00:00:00Z',
    rows: [{ id: 1, name: 'Local proxy', proxy_status: 'active', eligible: true, config: { enabled: true, interval_seconds: 10 }, stale: false,
      latest: { id: 1, started_at: '2026-10-29T00:00:00Z', finished_at: '2026-10-29T00:00:00Z', success: false, latency_ms: null, http_status: 429, error_class: 'http_429' },
      summary: { count: 2, successes: 0, failures: 2, success_rate: 0, average_ms: null, p95_ms: null } }]
  }
}
function render() {
  return mount(OverviewPage, { global: {
    plugins: [createI18n({ legacy: false, locale: 'zh', missingWarn: false, fallbackWarn: false })],
    stubs: {
      OverviewTrends: { props: ['overview'], template: '<div />' },
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
      RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
      MonitorDialog: { name: 'MonitorDialog', props: ['proxy', 'label'], template: '<button>{{ label }}</button>' }
    }
  } })
}
beforeEach(() => {
  vi.resetAllMocks()
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
})
it('shows failed latency as empty metrics, forwards filters and reuses the detail entry', async () => {
  vi.mocked(loadOverview).mockResolvedValue(fixture())
  const wrapper = render(); await flushPromises()
  expect(wrapper.text()).toContain('Local proxy')
  expect(wrapper.text()).toContain('连通失败')
  expect(wrapper.text()).toContain('失败 (429)')
  expect(wrapper.text()).toContain('0.0%')
  const average = wrapper.find('[data-field="average"]')
  expect(average.exists() ? average.text() : wrapper.findAll('td')[4].text()).toContain('—')
  expect(wrapper.findComponent({ name: 'MonitorDialog' }).props('proxy').id).toBe(1)
  await wrapper.get('select[aria-label="排序"]').setValue('failures'); await flushPromises()
  expect(loadOverview).toHaveBeenLastCalledWith(expect.objectContaining({ sort: 'failures', page: 1 }), expect.any(AbortSignal))
  await wrapper.get('input').setValue('Local')
  await wrapper.get('form').trigger('submit'); await flushPromises()
  expect(loadOverview).toHaveBeenLastCalledWith(expect.objectContaining({ search: 'Local', sort: 'failures' }), expect.any(AbortSignal))
  wrapper.unmount()
})
it('shows a retryable request failure and then an empty result', async () => {
  vi.mocked(loadOverview).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ ...fixture(), total: 0, rows: [] })
  const wrapper = render(); await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('更新失败')
  await wrapper.findAll('button').find(button => button.text() === '刷新')!.trigger('click'); await flushPromises()
  expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  expect(wrapper.text()).toContain('没有符合条件的代理')
  wrapper.unmount()
})
