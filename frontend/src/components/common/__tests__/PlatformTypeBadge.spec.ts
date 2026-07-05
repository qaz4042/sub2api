import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import PlatformTypeBadge from '../PlatformTypeBadge.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const messages: Record<string, string> = {
        'admin.accounts.subscriptionAbnormal': '异常',
        'admin.accounts.subscriptionExpires': '到期',
        'admin.accounts.subscriptionExpired': '套餐已到期',
        'admin.accounts.subscriptionRemainingDays': `套餐剩余 ${params?.days} 天`
      }
      return messages[key] ?? key
    }
  })
}))

const mountBadge = (subscriptionExpiresAt: string, planType = 'plus') =>
  mount(PlatformTypeBadge, {
    props: {
      platform: 'openai',
      type: 'oauth',
      planType,
      subscriptionExpiresAt
    },
    global: {
      stubs: {
        PlatformIcon: true,
        Icon: true
      }
    }
  })

describe('PlatformTypeBadge subscription expiry', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-22T00:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows the remaining days and exact expiration date', () => {
    const wrapper = mountBadge('2026-07-02T00:00:00Z')

    expect(wrapper.text()).toContain('Plus')
    expect(wrapper.text()).toContain('到期 2026-07-02')
    expect(wrapper.text()).toContain('套餐剩余 10 天')
  })

  it('highlights plans expiring within seven days', () => {
    const wrapper = mountBadge('2026-06-29T00:00:00Z')
    const expiry = wrapper.get('[title="2026-06-29T00:00:00Z"]')

    expect(expiry.text()).toContain('Plus')
    expect(expiry.text()).toContain('到期 2026-06-29')
    expect(expiry.text()).toContain('套餐剩余 7 天')
    expect(expiry.classes()).toContain('text-red-600')
  })

  it('shows an expired state', () => {
    const wrapper = mountBadge('2026-06-21T00:00:00Z')
    const expiry = wrapper.get('[title="2026-06-21T00:00:00Z"]')

    expect(wrapper.text()).toContain('到期 2026-06-21')
    expect(wrapper.text()).toContain('套餐已到期')
    expect(expiry.classes()).toContain('text-red-600')
  })

  it('does not show expiration for free plans', () => {
    const wrapper = mountBadge('2026-07-02T00:00:00Z', 'free')

    expect(wrapper.text()).not.toContain('套餐剩余')
  })
})
