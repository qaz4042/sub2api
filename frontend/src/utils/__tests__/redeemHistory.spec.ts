import { describe, expect, it } from 'vitest'
import { isOnlinePaymentTopUp } from '../redeemHistory'

describe('isOnlinePaymentTopUp', () => {
  it('recognizes automatically redeemed payment order codes', () => {
    expect(isOnlinePaymentTopUp({ type: 'balance', code: 'PAY-3-32145' })).toBe(true)
  })

  it('does not classify manually redeemed balance codes as online payments', () => {
    expect(isOnlinePaymentTopUp({ type: 'balance', code: 'PROMO-2026' })).toBe(false)
  })

  it('requires a balance record even when the code uses the payment prefix', () => {
    expect(isOnlinePaymentTopUp({ type: 'subscription', code: 'PAY-3-32145' })).toBe(false)
  })
})
