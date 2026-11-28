import { describe, expect, it } from 'vitest'
import { simulateLotteryDraws, type LotterySimulationTier } from '../lotterySimulation'

const tiers: LotterySimulationTier[] = [
  { position: 1, rewardCents: 50, weight: 8000 },
  { position: 2, rewardCents: 100, weight: 1500 },
  { position: 3, rewardCents: 500, weight: 500 },
]

function sequence(values: number[]) {
  let index = 0
  return () => values[index++ % values.length]
}

describe('simulateLotteryDraws', () => {
  it('counts every draw and assigns boundary picks to the ordered tiers', () => {
    // Total weight 10000: [0, 7999] -> tier 1, [8000, 9499] -> tier 2, [9500, 9999] -> tier 3.
    const rows = simulateLotteryDraws(
      tiers,
      6,
      sequence([0, 0.7999, 0.8, 0.9499, 0.95, 0.9999]),
    )
    expect(rows.map((row) => row.wins)).toEqual([2, 2, 2])
    expect(rows.reduce((sum, row) => sum + row.wins, 0)).toBe(6)
  })

  it('ignores inactive tiers and keeps every configured draw accounted for', () => {
    const rows = simulateLotteryDraws(
      [
        ...tiers,
        { position: 4, rewardCents: 1000, weight: 0 },
        { position: 5, rewardCents: 1000, weight: 1.5 },
        { position: 6, rewardCents: 0, weight: 1 },
      ],
      100,
      () => 0.9999,
    )
    expect(rows.map((row) => row.position)).toEqual([1, 2, 3])
    expect(rows.reduce((sum, row) => sum + row.wins, 0)).toBe(100)
  })

  it('returns ordered empty rows without active tiers or draws', () => {
    const rows = simulateLotteryDraws(
      [
        { position: 3, rewardCents: 100, weight: 10 },
        { position: 1, rewardCents: 50, weight: 0 },
      ],
      0,
    )
    expect(rows).toEqual([{ position: 3, rewardCents: 100, weight: 10, wins: 0 }])
    expect(simulateLotteryDraws([], 100)).toEqual([])
  })
})
