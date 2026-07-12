import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

describe('AmountInput', () => {
  it('shows the configured currency symbol', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        currency: 'CNY',
      },
    })

    expect(wrapper.find('.relative > span').text()).toBe('¥')
  })

  it('supports non-CNY payment currencies', () => {
    const wrapper = mount(AmountInput, {
      props: {
        modelValue: null,
        currency: 'USD',
      },
    })

    expect(wrapper.find('.relative > span').text()).toBe('$')
  })
})
