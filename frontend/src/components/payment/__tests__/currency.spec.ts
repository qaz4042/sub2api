import { describe, expect, it } from 'vitest'
import { formatPaymentAmount, getPaymentCurrencySymbol } from '../currency'

describe('formatPaymentAmount', () => {
  it('uses the currency default fraction digits', () => {
    expect(formatPaymentAmount(100, 'JPY', 'en-US')).not.toContain('.00')
    expect(formatPaymentAmount(100, 'KRW', 'en-US')).not.toContain('.00')
    expect(formatPaymentAmount(100, 'HKD', 'en-US')).toContain('.00')
  })
})

describe('getPaymentCurrencySymbol', () => {
  it('uses the payment currency instead of assuming USD', () => {
    expect(getPaymentCurrencySymbol('CNY', 'zh-CN')).toBe('¥')
    expect(getPaymentCurrencySymbol('USD', 'en-US')).toBe('$')
  })

  it('defaults to CNY when the provider currency is missing', () => {
    expect(getPaymentCurrencySymbol(undefined, 'zh-CN')).toBe('¥')
  })
})
