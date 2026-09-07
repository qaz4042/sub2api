import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchKeyModels } from '../keyModels'

afterEach(() => vi.unstubAllGlobals())

describe('key models', () => {
  it('uses the key credential and preserves discovered models without a static whitelist', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ data: [
      { id: 'gpt-6-astra', display_name: 'GPT-6 Astra' },
      { id: 'custom-model' }, { id: 'custom-model' }, { id: '' }, null
    ] }) })
    vi.stubGlobal('fetch', fetchMock)
    const signal = new AbortController().signal
    expect(await fetchKeyModels('https://example.com/api/v1/', 'sk-test', 'openai', signal)).toEqual([
      { value: 'gpt-6-astra', label: 'GPT-6 Astra' },
      { value: 'custom-model', label: 'custom-model' }
    ])
    expect(fetchMock).toHaveBeenCalledWith('https://example.com/api/v1/models', expect.objectContaining({
      headers: { Accept: 'application/json', Authorization: 'Bearer sk-test' }, signal
    }))
  })

  it('normalizes Gemini and Antigravity endpoints', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ data: [] }) })
    vi.stubGlobal('fetch', fetchMock)
    await fetchKeyModels('https://example.com/v1beta', 'key', 'gemini')
    expect(fetchMock.mock.calls[0][0]).toBe('https://example.com/v1/models')
    await fetchKeyModels('https://example.com', 'key', 'antigravity')
    expect(fetchMock.mock.calls[1][0]).toBe('https://example.com/antigravity/v1/models')
  })

  it('rejects failed and malformed responses', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce({ ok: false, status: 403 })
      .mockResolvedValueOnce({ ok: true, json: async () => ({}) })
    vi.stubGlobal('fetch', fetchMock)
    await expect(fetchKeyModels('', 'key')).rejects.toThrow('403')
    await expect(fetchKeyModels('', 'key')).rejects.toThrow('Invalid models response')
  })
})
