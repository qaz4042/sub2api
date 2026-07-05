import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AccountActionMenu from '../AccountActionMenu.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/components/icons', () => ({
  Icon: {
    props: ['name'],
    template: '<span :data-icon="name"></span>'
  }
}))

const baseAccount = {
  id: 1,
  name: 'account',
  platform: 'openai',
  type: 'oauth',
  credentials: {},
  extra: {},
  proxy_id: null,
  concurrency: 1,
  priority: 0,
  status: 'active',
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  schedulable: true,
  rate_limited_at: null,
  rate_limit_reset_at: null,
  overload_until: null,
  temp_unschedulable_until: null,
  temp_unschedulable_reason: null,
  session_window_start: null,
  session_window_end: null,
  session_window_status: null
} as any

describe('AccountActionMenu', () => {
  it('shows refresh subscription action for OpenAI OAuth accounts', async () => {
    const wrapper = mount(AccountActionMenu, {
      props: {
        show: true,
        account: baseAccount,
        position: { top: 10, left: 10 }
      },
      global: {
        stubs: {
          Teleport: true
        }
      }
    })

    const button = wrapper.findAll('button').find(item => item.text() === 'admin.accounts.refreshSubscription')
    expect(button).toBeTruthy()

    await button!.trigger('click')
    expect(wrapper.emitted('refresh-subscription')?.[0]).toEqual([baseAccount])
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('hides refresh subscription action for non OpenAI OAuth accounts', () => {
    const wrapper = mount(AccountActionMenu, {
      props: {
        show: true,
        account: { ...baseAccount, platform: 'anthropic' },
        position: { top: 10, left: 10 }
      },
      global: {
        stubs: {
          Teleport: true
        }
      }
    })

    expect(wrapper.text()).not.toContain('admin.accounts.refreshSubscription')
  })
})
