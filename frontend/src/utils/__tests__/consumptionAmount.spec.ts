import {describe, expect, it} from 'vitest'
import {consumptionUnits, thresholdUnits, consumptionDisplay, consumptionGap} from '../consumptionAmount'
describe('consumption ledger precision',()=>{
  it('never rounds an unreached threshold into eligibility',()=>{
    expect(consumptionUnits('49.99999999') < thresholdUnits(5000)).toBe(true)
    expect(consumptionDisplay('49.99999999')).toBe('$49.99')
    expect(consumptionGap('49.99999999',5000)).toBe(1)
    expect(consumptionUnits('50.00000000')).toBe(thresholdUnits(5000))
    expect(consumptionGap('50.00000000',5000)).toBe(0)
  })
})
