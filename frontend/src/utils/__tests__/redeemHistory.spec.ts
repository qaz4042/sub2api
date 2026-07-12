import { describe, expect, it } from 'vitest'
import { isOnlinePaymentTopUp } from '../redeemHistory'

describe('isOnlinePaymentTopUp', () => {
  it('recognizes payment-generated balance records', () => {
    expect(isOnlinePaymentTopUp({ type: 'balance', code: 'PAY-3-32145' })).toBe(true)
  })

  it('keeps manually redeemed balance records separate', () => {
    expect(isOnlinePaymentTopUp({ type: 'balance', code: 'PROMO-2026' })).toBe(false)
  })

  it('does not classify non-balance records as payments', () => {
    expect(isOnlinePaymentTopUp({ type: 'subscription', code: 'PAY-3-32145' })).toBe(false)
  })
})
