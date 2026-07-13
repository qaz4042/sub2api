import { describe, expect, it } from 'vitest'
import { resolveContactMethods } from '@/utils/contactMethods'

describe('resolveContactMethods', () => {
  it('prefers enabled structured methods and sorts them', () => {
    const methods = resolveContactMethods([
      { type: 'email', label: '邮箱', value: 'support@example.com', enabled: true, sort: 2 },
      { type: 'link', label: '帮助中心', value: '', url: 'https://example.com/help', enabled: true, sort: 1 },
      { type: 'text', label: '停用入口', value: 'hidden', enabled: false, sort: 0 },
    ], '旧联系方式', '联系客服')

    expect(methods.map((method) => method.label)).toEqual(['帮助中心', '邮箱'])
    expect(methods[0].href).toBe('https://example.com/help')
    expect(methods[1].href).toBe('mailto:support@example.com')
  })

  it('falls back to the legacy contact info when no structured method exists', () => {
    const methods = resolveContactMethods([], '邮箱：support@example.com | Telegram: @support_bot', '联系客服')

    expect(methods).toHaveLength(2)
    expect(methods[0]).toMatchObject({ label: 'Email', value: 'support@example.com' })
    expect(methods[1]).toMatchObject({ label: 'Telegram', value: '@support_bot', href: 'https://t.me/support_bot' })
  })

  it('does not create unsafe links from structured values', () => {
    const methods = resolveContactMethods([
      { type: 'link', label: '不安全', value: 'javascript:alert(1)', url: 'javascript:alert(1)' },
      { type: 'email', label: '邮箱', value: 'not-an-email' },
    ], '', '联系客服')

    expect(methods[0].href).toBeUndefined()
    expect(methods[1].href).toBeUndefined()
  })

  it('does not fall back to legacy contacts when all structured methods are disabled', () => {
    const methods = resolveContactMethods([
      { type: 'email', label: '邮箱', value: 'support@example.com', enabled: false },
    ], '旧联系方式', '联系客服')

    expect(methods).toEqual([])
  })
})
