/**
 * Local-only persistence for pelican runs.
 *
 * Results are intentionally not sent to the server. Each user gets a separate
 * IndexedDB database so switching accounts on a shared browser does not leak
 * history. Large HTML bodies live in IndexedDB; only lightweight UI state is
 * kept in localStorage.
 */
export interface PelicanUsageRecord {
  inputTokens: number | null
  outputTokens: number | null
  reasoningTokens: number | null
  totalTokens: number | null
}

export interface PelicanRunRecord {
  id: string
  createdAt: number
  userId: number
  testId: string
  promptVersion: string
  keyId: number
  keyName: string
  model: string
  reasoningEffort: string | null
  status: 'done' | 'cancelled' | 'error'
  error: string | null
  durationMs: number
  ttftMs: number | null
  usage: PelicanUsageRecord | null
  tps: number | null
  html: string | null
  rawText: string
  byteSize: number
  baseUrl: string
}

export interface PelicanUISettings {
  keyId: number | null
  keyMode: 'saved' | 'manual'
  modelByKey: Record<string, string>
  reasoningEffort: string
  baseUrl: string
}

const DB_PREFIX = 'sub2api-pelican-'
const DB_VERSION = 1
const STORE = 'runs'
const MAX_RUNS = 50
const MAX_BYTES = 20 * 1024 * 1024

export function pruneRecords<T extends { createdAt: number; byteSize: number }>(
  records: T[],
  maxCount = MAX_RUNS,
  maxBytes = MAX_BYTES
): T[] {
  const sorted = [...records].sort((a, b) => a.createdAt - b.createdAt)
  let total = sorted.reduce((sum, record) => sum + Math.max(0, record.byteSize), 0)
  let removeCount = 0
  while (removeCount < sorted.length && (sorted.length - removeCount > maxCount || total > maxBytes)) {
    total -= Math.max(0, sorted[removeCount].byteSize)
    removeCount++
  }
  return sorted.slice(removeCount)
}

export async function listRuns(userId: number): Promise<PelicanRunRecord[]> {
  const db = await openDB(userId)
  try {
    const records = await requestToPromise<PelicanRunRecord[]>(
      db.transaction(STORE, 'readonly').objectStore(STORE).getAll()
    )
    return records.sort((a, b) => b.createdAt - a.createdAt)
  } finally {
    db.close()
  }
}

export async function saveRun(userId: number, record: PelicanRunRecord): Promise<void> {
  const db = await openDB(userId)
  try {
    const tx = db.transaction(STORE, 'readwrite')
    tx.objectStore(STORE).put(record)
    await waitForTransaction(tx)
    const all = await requestToPromise<PelicanRunRecord[]>(
      db.transaction(STORE, 'readonly').objectStore(STORE).getAll()
    )
    const kept = new Set(pruneRecords(all).map((item) => item.id))
    const removed = all.filter((item) => !kept.has(item.id)).map((item) => item.id)
    if (removed.length > 0) {
      const cleanup = db.transaction(STORE, 'readwrite')
      const store = cleanup.objectStore(STORE)
      for (const id of removed) {
        store.delete(id)
      }
      await waitForTransaction(cleanup)
    }
  } finally {
    db.close()
  }
}

export async function deleteRun(userId: number, id: string): Promise<void> {
  const db = await openDB(userId)
  try {
    const tx = db.transaction(STORE, 'readwrite')
    tx.objectStore(STORE).delete(id)
    await waitForTransaction(tx)
  } finally {
    db.close()
  }
}

export async function clearRuns(userId: number): Promise<void> {
  const db = await openDB(userId)
  try {
    const tx = db.transaction(STORE, 'readwrite')
    tx.objectStore(STORE).clear()
    await waitForTransaction(tx)
  } finally {
    db.close()
  }
}

export function loadUISettings(userId: number): PelicanUISettings | null {
  try {
    const raw = localStorage.getItem(uiSettingsKey(userId))
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw) as Partial<PelicanUISettings>
    return {
      keyId: typeof parsed.keyId === 'number' ? parsed.keyId : null,
      keyMode: parsed.keyMode === 'manual' ? 'manual' : 'saved',
      modelByKey:
        parsed.modelByKey && typeof parsed.modelByKey === 'object' ? parsed.modelByKey : {},
      reasoningEffort: typeof parsed.reasoningEffort === 'string' ? parsed.reasoningEffort : '',
      baseUrl: typeof parsed.baseUrl === 'string' ? parsed.baseUrl : ''
    }
  } catch {
    return null
  }
}

export function saveUISettings(userId: number, settings: PelicanUISettings): void {
  try {
    localStorage.setItem(uiSettingsKey(userId), JSON.stringify(settings))
  } catch {
    // localStorage may be unavailable or full; UI settings are best effort.
  }
}

function uiSettingsKey(userId: number): string {
  return `pelican:ui:${userId}`
}

function openDB(userId: number): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (typeof indexedDB === 'undefined') {
      reject(new Error('IndexedDB is unavailable'))
      return
    }
    const request = indexedDB.open(`${DB_PREFIX}${userId}`, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(STORE)) {
        const store = db.createObjectStore(STORE, { keyPath: 'id' })
        store.createIndex('createdAt', 'createdAt')
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error ?? new Error('Failed to open IndexedDB'))
  })
}

function requestToPromise<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error ?? new Error('IndexedDB request failed'))
  })
}

function waitForTransaction(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error ?? new Error('IndexedDB transaction failed'))
    transaction.onabort = () => reject(transaction.error ?? new Error('IndexedDB transaction aborted'))
  })
}
