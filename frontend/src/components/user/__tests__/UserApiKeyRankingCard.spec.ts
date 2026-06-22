import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UserApiKeyRankingCard from '../UserApiKeyRankingCard.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, number>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

describe('UserApiKeyRankingCard', () => {
  it('highlights the current user rank and keeps other keys anonymous', async () => {
    const myKey = {
      rank: 18,
      api_key_id: 42,
      key_name: 'production',
      is_mine: true,
      actual_cost: 2.5,
      requests: 10,
      tokens: 500,
    }
    const wrapper = mount(UserApiKeyRankingCard, {
      props: {
        totalKeys: 30,
        myRankings: [myKey],
        ranking: [
          {
            rank: 1,
            key_name: 's******y',
            is_mine: false,
            actual_cost: 20,
            requests: 100,
            tokens: 5000,
          },
        ],
      },
      global: {
        stubs: { LoadingSpinner: true },
      },
    })

    expect(wrapper.text()).toContain('#18')
    expect(wrapper.text()).toContain('production')
    expect(wrapper.text()).toContain('s******y')
    expect(wrapper.text()).not.toContain('secret-key')

    const ownKeyButton = wrapper.findAll('button').find((button) => button.text().includes('production'))
    expect(ownKeyButton).toBeTruthy()
    await ownKeyButton!.trigger('click')
    expect(wrapper.emitted('keyClick')?.[0]).toEqual([myKey])
  })
})
