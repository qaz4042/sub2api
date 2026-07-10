import { describe, expect, it } from 'vitest'
import {
  DEFAULT_CC_SWITCH_DIRECT_BASE_URL,
  OPENAI_CC_SWITCH_CODEX_MODEL,
  buildCcSwitchImportDeeplink,
  resolveCcSwitchBaseUrl
} from '@/utils/ccswitchImport'
import type { GroupPlatform } from '@/types'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

describe('ccswitchImport utils', () => {
  const baseInput = {
    baseUrl: 'https://api.example.com',
    providerName: 'Sub2API',
    apiKey: 'sk-test',
    usageScript: 'return true'
  }

  it('uses GPT-5.6 Terra as the default Codex model', () => {
    expect(OPENAI_CC_SWITCH_CODEX_MODEL).toBe('gpt-5.6-terra')
  })

  it('prefers the CCS import base URL over the general API base URL', () => {
    expect(
      resolveCcSwitchBaseUrl(
        {
          ccs_import_base_url: 'https://152.32.190.110',
          api_base_url: 'https://codex.lizubin.online'
        },
        'https://current.example.com'
      )
    ).toBe('https://152.32.190.110')
  })

  it('defaults CCS imports to the IP direct base URL when no dedicated URL is configured', () => {
    expect(
      resolveCcSwitchBaseUrl({ ccs_import_base_url: '', api_base_url: 'https://api.example.com' }, 'https://current.example.com')
    ).toBe(DEFAULT_CC_SWITCH_DIRECT_BASE_URL)
    expect(resolveCcSwitchBaseUrl(null, 'https://current.example.com')).toBe(DEFAULT_CC_SWITCH_DIRECT_BASE_URL)
  })

  it('adds the Codex model parameter for OpenAI imports', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'openai',
        clientType: 'claude'
      })
    )

    expect(params.get('resource')).toBe('provider')
    expect(params.get('app')).toBe('codex')
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.get('model')).toBe(OPENAI_CC_SWITCH_CODEX_MODEL)
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
  })

  it.each([
    { platform: 'anthropic' as GroupPlatform, clientType: 'claude' as const, app: 'claude' },
    { platform: 'gemini' as GroupPlatform, clientType: 'gemini' as const, app: 'gemini' }
  ])('does not add a model parameter for $platform imports', ({ platform, clientType, app }) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform,
        clientType
      })
    )

    expect(params.get('app')).toBe(app)
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.has('model')).toBe(false)
  })

  it('keeps Antigravity imports on the selected client endpoint without a model parameter', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'antigravity',
        clientType: 'gemini'
      })
    )

    expect(params.get('app')).toBe('gemini')
    expect(params.get('endpoint')).toBe(`${baseInput.baseUrl}/antigravity`)
    expect(params.has('model')).toBe(false)
  })
})
