export interface RedeemHistorySourceItem {
  type: string
  code: string
}

/**
 * Balance payment fulfillment is implemented internally as an automatically redeemed
 * code. Keep it distinct from codes that users redeem manually in activity labels.
 */
export const isOnlinePaymentTopUp = (item: RedeemHistorySourceItem): boolean => {
  return item.type === 'balance' && item.code.startsWith('PAY-')
}
