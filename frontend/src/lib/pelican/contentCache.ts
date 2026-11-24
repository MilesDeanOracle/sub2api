/**
 * In-memory LRU for fetched operations artifacts.
 *
 * Wall cards refetch `content(id)` on every remount (filter switches, stack
 * reveals, refresh key churn). Artifacts can reach the 5 MB upload limit, so
 * the cache is byte-budgeted: repeat views render instantly while bounded
 * memory keeps long wall sessions safe.
 */
const MAX_ENTRIES = 24
const MAX_BYTES = 8 << 20

const cache = new Map<number, string>()

/** Map already refreshes insertion order, so the oldest key is the LRU victim. */
export function getCachedArtifact(id: number): string | undefined {
  const html = cache.get(id)
  if (html !== undefined) {
    cache.delete(id)
    cache.set(id, html)
  }
  return html
}

export function storeArtifact(id: number, html: string): void {
  if (html.length > MAX_BYTES) return
  cache.delete(id)
  cache.set(id, html)
  let bytes = 0
  for (const value of cache.values()) bytes += value.length
  while (cache.size > MAX_ENTRIES || bytes > MAX_BYTES) {
    const oldest = cache.keys().next().value
    if (oldest === undefined) break
    bytes -= cache.get(oldest)?.length ?? 0
    cache.delete(oldest)
  }
}

/** Test hook; production keeps artifacts for the session. */
export function clearArtifactCache(): void {
  cache.clear()
}
