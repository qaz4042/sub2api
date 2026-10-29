import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import { createI18n } from 'vue-i18n'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import OverviewTrends from '../OverviewTrends.vue'
import { loadTrends, type Overview, type Trends } from '../api'
import { useOverviewTrends } from '../trend-state'
vi.mock('../api', () => ({ loadTrends: vi.fn() }))
const ChartStub = defineComponent({ name: 'ChartLine', props: ['data', 'options'], template: '<div class="chart" />' })
vi.mock('vue-chartjs', () => ({ Line: { name: 'ChartLine', props: ['data', 'options'], template: '<div class="chart" />' } }))
function overview(count = 2): Overview {
  return { total: count, page: 1, page_size: 50, from: '2026-10-28T00:00:00Z', to: '2026-10-29T00:00:00Z',
    rows: Array.from({ length: count }, (_, index) => ({ id: index + 1, name: `proxy-${index + 1}`, eligible: true, config: { enabled: true, interval_seconds: 10 }, summary: { count: 1 } })) } as Overview
}
function trends(): Trends {
  return { from: '2026-10-28T00:00:00Z', to: '2026-10-29T00:00:00Z', bucket_seconds: 300,
    series: [{ id: 1, name: 'proxy-1', points: [
      { at: '2026-10-28T00:00:00Z', count: 2, successes: 2, failures: 0, average_ms: 50, p95_ms: 90, success_rate: 1 },
      { at: '2026-10-28T00:05:00Z', count: 1, successes: 0, failures: 1, average_ms: null, p95_ms: null, success_rate: 0 },
      { at: '2026-10-28T00:10:00Z', count: 0, successes: 0, failures: 0, average_ms: null, p95_ms: null, success_rate: null }
    ] }] }
}
beforeEach(() => { vi.resetAllMocks(); vi.spyOn(document, 'hidden', 'get').mockReturnValue(false) })
afterEach(() => vi.restoreAllMocks())
it('renders P95 and success rate, preserving failed latency and missing buckets as null', async () => {
  vi.mocked(loadTrends).mockResolvedValue(trends())
  const wrapper = mount(OverviewTrends, { props: { overview: overview() }, global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] } })
  await flushPromises()
  const charts = wrapper.findAllComponents({ name: ChartStub.name })
  expect(charts).toHaveLength(2)
  expect(charts[0].props('data').datasets[0].data).toEqual([90, null, null])
  expect(charts[1].props('data').datasets[0].data).toEqual([100, 0, null])
  expect(charts[0].props('data').datasets[0].spanGaps).toBe(false)
  expect(charts[1].props('options').plugins.tooltip.callbacks.afterLabel({ datasetIndex: 0, dataIndex: 1 })).toContain('失败: 1')
  await wrapper.get('select').setValue('average_ms')
  expect(charts[0].props('data').datasets[0].data).toEqual([50, null, null])
  wrapper.unmount()
})
it('shows an explicit all-failure explanation while retaining the success-rate chart', async () => {
  const result = trends(); result.series[0].points = [result.series[0].points[1]]
  vi.mocked(loadTrends).mockResolvedValue(result)
  const wrapper = mount(OverviewTrends, { props: { overview: overview() }, global: { plugins: [createI18n({ legacy: false, locale: 'zh' })] } }); await flushPromises()
  expect(wrapper.text()).toContain('没有成功样本')
  expect(wrapper.findAllComponents({ name: 'ChartLine' })[1].props('data').datasets[0].data).toEqual([0])
  wrapper.unmount()
})
function setup(count = 2) {
  const source = ref<Overview | null>(overview(count))
  let state!: ReturnType<typeof useOverviewTrends>
  const wrapper = mount(defineComponent({ setup() { state = useOverviewTrends(source); return () => null } }))
  return { source, state, wrapper }
}
it('bounds comparison to ten and preserves selection across ranking changes', async () => {
  vi.mocked(loadTrends).mockResolvedValue(trends())
  const { source, state, wrapper } = setup(12); await flushPromises()
  expect(state.selected.value).toHaveLength(10)
  const calls = vi.mocked(loadTrends).mock.calls.length
  state.toggle(11); expect(loadTrends).toHaveBeenCalledTimes(calls)
  state.toggle(1); state.toggle(11); await flushPromises()
  expect(state.selected.value).toContain(11); expect(state.selected.value).not.toContain(1)
  source.value = { ...source.value!, rows: [...source.value!.rows].reverse() }; await flushPromises()
  expect(state.selected.value).toContain(11); expect(state.selected.value).not.toContain(1)
  expect(loadTrends).toHaveBeenLastCalledWith(state.selected.value, 24, source.value.to, expect.any(AbortSignal))
  wrapper.unmount()
})
it('rejects stale responses, cancels on hiding and leaving, and retries failures', async () => {
  let resolveOld!: (value: Trends) => void
  vi.mocked(loadTrends).mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve })).mockRejectedValueOnce(new Error('offline')).mockResolvedValue(trends())
  const { state, wrapper } = setup()
  const signal = vi.mocked(loadTrends).mock.calls[0][3]
  state.toggle(2); await flushPromises()
  expect(signal.aborted).toBe(true); expect(state.error.value).toBe(true)
  resolveOld(trends()); await flushPromises(); expect(state.data.value).toBeNull()
  await state.load(); expect(state.error.value).toBe(false); expect(state.data.value?.series).toHaveLength(1)
  const current = vi.mocked(loadTrends).mock.calls.at(-1)![3]
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(true); document.dispatchEvent(new Event('visibilitychange'))
  expect(current.aborted).toBe(true)
  wrapper.unmount()
})
