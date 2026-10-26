import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountErrorStats from '../AccountErrorStats.vue'
import type { AccountErrorStats as ErrorStats } from '@/types'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-chartjs', () => ({
  Line: { name: 'Line', props: ['data', 'options'], template: '<div class="chart" />' }
}))

describe('AccountErrorStats', () => {
  it('compares total and failed requests on the same count axis with a separate rate axis', () => {
    const stats: ErrorStats = {
      coverage_start: "2026-10-21T00:00:00+08:00", coverage_end: "2026-10-23T00:00:00+08:00", partial: false, uncorrelated_ws_errors: 0,
      total_requests: 100, error_count: 3, error_rate: 3,
      history: [
        { date: '2026-10-21', label: '10/21', requests: 100, error_count: 3, error_rate: 3 },
        { date: '2026-10-22', label: '10/22', requests: 0, error_count: 0, error_rate: null }
      ]
    }
    const wrapper = mount(AccountErrorStats, { props: { stats } })
    expect(wrapper.get('[data-testid="total-requests"]').text()).toBe('100')
    expect(wrapper.get('[data-testid="error-count"]').text()).toBe('3')
    expect(wrapper.get('[data-testid="error-rate"]').text()).toBe('3.00%')
    const chart = wrapper.getComponent({ name: 'Line' })
    expect(chart.props('data').datasets.map((dataset: { data: number[] }) => dataset.data)).toEqual([[100, 0], [3, 0], [3, null]])
    expect(chart.props('data').datasets.map((dataset: { yAxisID: string }) => dataset.yAxisID)).toEqual(['y', 'y', 'rate'])
    expect(chart.props('options').scales.rate.max).toBe(4)
  })

  it('distinguishes unavailable data from zero errors', () => {
    const wrapper = mount(AccountErrorStats)
    expect(wrapper.text()).toContain('admin.accounts.stats.errorStatsUnavailable')
    expect(wrapper.find('[data-testid="error-count"]').exists()).toBe(false)
    const zero = mount(AccountErrorStats, {
      props: { stats: { coverage_start: "2026-10-21T00:00:00Z", coverage_end: "2026-10-22T00:00:00Z", partial: false, uncorrelated_ws_errors: 0, total_requests: 0, error_count: 0, error_rate: null, history: [] } }
    })
    expect(zero.get('[data-testid="error-rate"]').text()).toBe('—')
    expect(zero.get('[data-testid="total-requests"]').text()).toBe('0')
    expect(zero.find('.chart').exists()).toBe(false)
  })

  it('explains disabled monitoring and hides unreliable WS/policy denominators', () => {
    const disabled = mount(AccountErrorStats, { props: { unavailableReason: 'monitoring_disabled' } })
    expect(disabled.text()).toContain('admin.accounts.stats.errorMonitoringDisabled')
    const partial = mount(AccountErrorStats, { props: { stats: {
      coverage_start: '2026-10-21T00:00:00Z', coverage_end: '2026-10-22T00:00:00Z',
      partial: true, uncorrelated_ws_errors: 2, total_requests: 5, error_count: 2,
      error_rate: null, history: []
    } } })
    expect(partial.text()).toContain('admin.accounts.stats.errorStatsPartial')
    expect(partial.text()).toContain('admin.accounts.stats.errorStatsUncorrelated')
    expect(partial.text()).not.toContain('admin.accounts.stats.errorRateBasis')
    expect(partial.get('[data-testid="error-rate"]').text()).toBe('—')
    expect(partial.get('[data-testid="total-requests"]').text()).toBe('—')
  })

  it('also hides the denominator for policy failures without WS error rows', () => {
    const wrapper = mount(AccountErrorStats, { props: { stats: {
      coverage_start: '2026-10-21T00:00:00Z', coverage_end: '2026-10-22T00:00:00Z',
      partial: true, uncorrelated_ws_errors: 0, uncorrelated_errors: 1,
      total_requests: 2, error_count: 1, error_rate: null,
      history: [{ date: '2026-10-21', label: '10/21', requests: 2, error_count: 1, error_rate: null }]
    } } })
    expect(wrapper.text()).toContain('admin.accounts.stats.errorStatsUncorrelated')
    expect(wrapper.text()).not.toContain('admin.accounts.stats.errorRateBasis')
    expect(wrapper.get('[data-testid="error-rate"]').text()).toBe('—')
    expect(wrapper.get('[data-testid="total-requests"]').text()).toBe('—')
    expect(wrapper.getComponent({ name: 'Line' }).props('data').datasets.map((dataset: { label: string }) => dataset.label))
      .not.toContain('admin.accounts.stats.totalRequests')
  })
})
