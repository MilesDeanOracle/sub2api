// Unknown or older API reasons still get a useful, translated explanation.
export function walletTransferBlockKey(reason?: string): string {
  const known = ['awards_paused', 'refund_review', 'recovery_debt']
  return `dashboard.activity.transferBlocks.${reason && known.includes(reason) ? reason : 'unavailable'}`
}
