import { beforeEach, describe, expect, it } from 'vitest'
import { clearArtifactCache, getCachedArtifact, storeArtifact } from '../contentCache'

beforeEach(() => clearArtifactCache())

describe('pelican artifact content cache', () => {
  it('returns undefined for unknown ids', () => {
    expect(getCachedArtifact(1)).toBeUndefined()
  })

  it('stores and refreshes LRU order on read', () => {
    storeArtifact(1, 'a')
    storeArtifact(2, 'b')
    getCachedArtifact(1)
    storeArtifact(3, 'c')
    expect(getCachedArtifact(2)).toBe('b')
    expect(getCachedArtifact(1)).toBe('a')
    expect(getCachedArtifact(3)).toBe('c')
  })

  it('evicts the least recently used entry beyond the entry cap', () => {
    for (let id = 1; id <= 24; id++) storeArtifact(id, String(id))
    storeArtifact(25, 'new')
    expect(getCachedArtifact(1)).toBeUndefined()
    expect(getCachedArtifact(2)).toBe('2')
    expect(getCachedArtifact(25)).toBe('new')
  })

  it('evicts by byte budget when entries are large', () => {
    const entry = 'x'.repeat(500 * 1024) // 500KB each; 18 entries exceed the 8MB budget
    for (let id = 1; id <= 18; id++) storeArtifact(id, entry)
    expect(getCachedArtifact(1)).toBeUndefined()
    expect(getCachedArtifact(2)).toBeUndefined()
    expect(getCachedArtifact(3)).toBeDefined()
    expect(getCachedArtifact(18)).toBeDefined()
  })
})
