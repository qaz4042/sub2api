import { describe, expect, it } from 'vitest'
import { sanitizeHtml, sanitizeSvg } from '@/utils/sanitize'

describe('sanitizeHtml', () => {
  it('keeps common presentation markup', () => {
    const result = sanitizeHtml('<section class="hero" style="color:red"><a href="https://example.com" target="_blank">Link</a><img src="/logo.png" alt="Logo"></section>')

    expect(result).toContain('class="hero"')
    expect(result).toContain('style="color:red"')
    expect(result).toContain('href="https://example.com"')
    expect(result).toContain('target="_blank"')
    expect(result).toContain('src="/logo.png"')
  })

  it('removes executable content and dangerous URLs', () => {
    const result = sanitizeHtml('<script>alert(1)</script><img src=x onerror="alert(1)"><a href="javascript:alert(1)">bad</a><iframe src="https://example.com"></iframe>')

    expect(result).not.toContain('<script')
    expect(result).not.toContain('onerror')
    expect(result).not.toContain('javascript:')
    expect(result).not.toContain('<iframe')
  })
})

describe('sanitizeSvg', () => {
  it('keeps safe svg fragments and removes event handlers', () => {
    const result = sanitizeSvg('<path d="M1 1L2 2" onclick="alert(1)"/>')

    expect(result).toContain('<path')
    expect(result).toContain('d="M1 1L2 2"')
    expect(result).not.toContain('onclick')
  })
})
