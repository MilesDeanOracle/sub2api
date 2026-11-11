import { describe, expect, it } from 'vitest'
import { loadUISettings, pruneRecords, saveUISettings } from '../storage'

describe('pruneRecords', () => {
  const make = (id: number, createdAt: number, byteSize: number) => ({ id, createdAt, byteSize })

  it('keeps only the newest records by count', () => {
    const records = [make(1, 100, 10), make(2, 200, 10), make(3, 300, 10)]
    expect(pruneRecords(records, 2, 1000).map((record) => record.id)).toEqual([2, 3])
  })

  it('removes oldest records until under the byte limit', () => {
    const records = [make(1, 100, 60), make(2, 200, 60), make(3, 300, 60)]
    expect(pruneRecords(records, 50, 100).map((record) => record.id)).toEqual([3])
  })

  it('keeps everything when under both limits', () => {
    const records = [make(1, 100, 10), make(2, 200, 10)]
    expect(pruneRecords(records, 50, 1000)).toHaveLength(2)
  })
})

describe('ui settings', () => {
  it('round-trips the latest selection', () => {
    saveUISettings(7, {
      keyId: 12,
      keyMode: 'saved',
      modelByKey: { '12': 'gpt-5.1' },
      reasoningEffort: 'low',
      baseUrl: 'https://api.example.com'
    })
    expect(loadUISettings(7)).toEqual({
      keyId: 12,
      keyMode: 'saved',
      modelByKey: { '12': 'gpt-5.1' },
      reasoningEffort: 'low',
      baseUrl: 'https://api.example.com'
    })
  })

  it('returns null for missing or invalid settings', () => {
    expect(loadUISettings(999)).toBeNull()
    localStorage.setItem('pelican:ui:998', '{bad json')
    expect(loadUISettings(998)).toBeNull()
  })
})
