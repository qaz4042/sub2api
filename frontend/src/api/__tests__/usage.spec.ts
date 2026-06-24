import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
  },
}))

import { getApiKeyRanking } from '@/api/usage'

describe('usage api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('normalizes null API key ranking arrays to empty arrays', async () => {
    get.mockResolvedValue({
      data: {
        ranking: null,
        my_rankings: null,
        total_keys: null,
      },
    })

    const result = await getApiKeyRanking({ limit: 10 })

    expect(get).toHaveBeenCalledWith('/usage/ranking', { params: { limit: 10 } })
    expect(result).toEqual({
      ranking: [],
      my_rankings: [],
      total_keys: 0,
      start_date: '',
      end_date: '',
    })
  })
})
