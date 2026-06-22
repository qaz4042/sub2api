import { describe, expect, it } from 'vitest'

import { availableLocales } from '../index'
import zhTW from '../locales/zh-TW'
import zh from '../locales/zh'

function collectKeys(value: unknown, prefix = ''): string[] {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return [prefix]
  }

  return Object.entries(value).flatMap(([key, child]) =>
    collectKeys(child, prefix ? `${prefix}.${key}` : key)
  )
}

describe('Traditional Chinese locale', () => {
  it('appears immediately before Simplified Chinese', () => {
    expect(availableLocales.map(({ code, name }) => ({ code, name }))).toEqual([
      { code: 'en', name: 'English' },
      { code: 'zh-TW', name: '繁體中文' },
      { code: 'zh', name: '简体中文' }
    ])
  })

  it('has the same translation keys as Simplified Chinese', () => {
    expect(collectKeys(zhTW).sort()).toEqual(collectKeys(zh).sort())
  })

  it('uses Traditional Chinese copy', () => {
    expect(zhTW.home.login).toBe('登入')
    expect(zhTW.home.switchToDark).toBe('切換到深色模式')
  })
})
