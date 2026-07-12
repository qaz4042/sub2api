export interface RedeemHistorySourceItem {
  type: string
  code: string
}

export function isOnlinePaymentTopUp(item: RedeemHistorySourceItem): boolean {
  return item.type === 'balance' && item.code.startsWith('PAY-')
}
