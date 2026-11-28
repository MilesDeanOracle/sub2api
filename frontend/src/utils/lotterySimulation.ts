export interface LotterySimulationTier {
  position: number
  rewardCents: number
  weight: number
}

export interface LotterySimulationRow extends LotterySimulationTier {
  wins: number
}

/** Upper bound for the local simulation so the operator UI cannot freeze. */
export const MAX_LOTTERY_TEST_DRAWS = 100_000

/**
 * Simulate weighted lottery draws with the same ordered cumulative selection
 * used by the backend (`pickWeightedLotteryTier`). Only active tiers (positive
 * integer weight and positive reward) participate. The result is local-only:
 * no tickets are consumed and no rewards are credited.
 */
export function simulateLotteryDraws(
  tiers: readonly LotterySimulationTier[],
  draws: number,
  random: () => number = Math.random,
): LotterySimulationRow[] {
  const active = tiers
    .filter((tier) => Number.isInteger(tier.weight) && tier.weight > 0 && tier.rewardCents > 0)
    .sort((a, b) => a.position - b.position)
    .map((tier) => ({ ...tier, wins: 0 }))
  const totalWeight = active.reduce((sum, tier) => sum + tier.weight, 0)
  if (totalWeight <= 0 || !Number.isFinite(draws) || draws <= 0) {
    return active
  }

  const count = Math.floor(draws)
  for (let draw = 0; draw < count; draw++) {
    let ticket = Math.floor(random() * totalWeight)
    for (const tier of active) {
      if (ticket < tier.weight) {
        tier.wins++
        break
      }
      ticket -= tier.weight
    }
  }
  return active
}
