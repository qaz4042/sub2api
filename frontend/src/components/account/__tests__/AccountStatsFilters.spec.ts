import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import AccountStatsFilters from '../AccountStatsFilters.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key, locale: ref('zh') })
}))

describe('AccountStatsFilters', () => {
  it('applies custom whole-day ranges only after confirmation and shows range errors', async () => {
    const wrapper = mount(AccountStatsFilters, {
      props: {
        startDate: '2026-10-01', endDate: '2026-10-03',
        invalidRange: false, granularity: 'day', includeTests: true
      },
      global: { stubs: { Icon: true, Select: true } }
    })
    await wrapper.find('.date-picker-trigger').trigger('click')
    expect(wrapper.text()).not.toContain('dates.last24Hours')
    expect(wrapper.text()).toContain('dates.last7Days')
    const dates = wrapper.findAll('input[type="date"]')
    await dates[0].setValue('2026-09-02')
    await dates[1].setValue('2026-09-05')
    expect(wrapper.emitted('change:dateRange')).toBeUndefined()
    await wrapper.find('.date-picker-apply').trigger('click')
    expect(wrapper.emitted('change:dateRange')?.[0]).toEqual([
      { startDate: '2026-09-02', endDate: '2026-09-05', preset: null }
    ])
    await wrapper.setProps({ invalidRange: true })
    expect(wrapper.find('[role="alert"]').text()).toBe('admin.accounts.stats.invalidDateRange')
    wrapper.unmount()
  })
})
