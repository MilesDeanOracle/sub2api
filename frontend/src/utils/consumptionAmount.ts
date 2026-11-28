// Keep the ledger's eight decimals while comparing thresholds. Converting to
// a floating-point dollar value can turn a just-below-threshold value into it.
export function consumptionUnits(value: string): bigint {
  const match = /^(\d+)(?:\.(\d{0,8}))?$/.exec(value)
  if (!match) return 0n
  return BigInt(match[1]) * 100000000n + BigInt((match[2] || '').padEnd(8, '0'))
}
export const thresholdUnits = (cents: number): bigint => BigInt(cents) * 1000000n
export function consumptionDisplay(value: string): string {
  const cents = consumptionUnits(value) / 1000000n
  return `$${cents / 100n}.${String(cents % 100n).padStart(2, '0')}`
}
export function consumptionGap(value: string, threshold: number): number {
  const gap = thresholdUnits(threshold) - consumptionUnits(value)
  return gap > 0n ? Number((gap + 999999n) / 1000000n) : 0
}
