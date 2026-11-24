import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OperationsResultCard from '../OperationsResultCard.vue'
import { clearArtifactCache } from '@/lib/pelican/contentCache'
import type { OperationsResult } from '@/api/admin/operationsConsole'

const { content } = vi.hoisted(() => ({ content: vi.fn() }))
vi.mock('@/api/admin/operationsConsole', () => ({ operationsApi: { content } }))
// Partial mock: api/client.ts -> @/i18n needs the real createI18n at import time.
vi.mock('vue-i18n', async (importOriginal) => ({ ...(await importOriginal<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string) => key }) }))

let resize!: () => void
beforeEach(() => {
  vi.clearAllMocks()
  clearArtifactCache()
  // The shared setup installs a no-op IntersectionObserver; cards need one
  // that reports the card as intersecting so content loading starts.
  vi.stubGlobal('IntersectionObserver', class {
    constructor(callback: (entries: Array<{ isIntersecting: boolean }>) => void) {
      callback([{ isIntersecting: true }])
    }
    observe() {}
    disconnect() {}
  })
  vi.stubGlobal('ResizeObserver', class {
    constructor(callback: () => void) { resize = callback }
    observe() {}
    disconnect() {}
  })
})
afterEach(() => vi.unstubAllGlobals())

function makeResult(overrides: Partial<OperationsResult> = {}): OperationsResult {
  return {
    id: 1, task_id: 1, run_id: 'run', account_id: 1, task_name: 'task', account_name: 'account',
    groups: [], model_id: 'model', upstream_model_id: '', reasoning_effort: 'high',
    status: 'succeeded', created_at: '2026-01-01T00:00:00Z', finished_at: '2026-01-01T00:01:00Z',
    queued_at: '2026-01-01T00:00:00Z', expires_at: '2026-01-08T00:00:00Z',
    ttft_ms: 100, first_text_ms: 100, tps: 10, input_tokens: 1, output_tokens: 1,
    reasoning_tokens: 1, upstream_duration_ms: 1000, ...overrides
  }
}

// jsdom reports clientWidth as 0, so the fitted stage never becomes ready
// until the test measures it through the stubbed ResizeObserver.
async function settleStage(wrapper: ReturnType<typeof mount>) {
  const stage = wrapper.get('.pelican-preview-stage').element
  Object.defineProperties(stage, { clientWidth: { value: 400, configurable: true }, clientHeight: { value: 250, configurable: true } })
  resize()
  await flushPromises()
}

describe('Operations result card', () => {
  it('renders the artifact preview once content resolves', async () => {
    content.mockResolvedValue('<html><body>artwork-a</body></html>')
    const wrapper = mount(OperationsResultCard, { props: { result: makeResult() } })
    try {
      await flushPromises()
      await settleStage(wrapper)
      expect(wrapper.get('iframe').attributes('srcdoc')).toContain('artwork-a')
    } finally { wrapper.unmount() }
  })

  it('serves remounted cards from the cache without refetching', async () => {
    content.mockResolvedValue('<html><body>artwork-a</body></html>')
    const first = mount(OperationsResultCard, { props: { result: makeResult({ id: 7 }) } })
    await flushPromises(); first.unmount()

    content.mockClear()
    const second = mount(OperationsResultCard, { props: { result: makeResult({ id: 7 }) } })
    try {
      await flushPromises()
      await settleStage(second)
      expect(content).not.toHaveBeenCalled()
      expect(second.get('iframe').attributes('srcdoc')).toContain('artwork-a')
    } finally { second.unmount() }
  })

  it('drops stale html when a reused stack slot shifts to a new result id', async () => {
    content.mockResolvedValue('<html><body>artwork-a</body></html>')
    const wrapper = mount(OperationsResultCard, { props: { result: makeResult({ id: 1 }) } })
    try {
      await flushPromises()
      await settleStage(wrapper)
      expect(wrapper.get('iframe').attributes('srcdoc')).toContain('artwork-a')

      content.mockResolvedValue('<html><body>artwork-b</body></html>')
      await wrapper.setProps({ result: makeResult({ id: 2 }) })
      await flushPromises()
      // The id shift unmounts and remounts the preview; measure its new stage.
      await settleStage(wrapper)
      expect(content).toHaveBeenCalledWith(2)
      expect(wrapper.get('iframe').attributes('srcdoc')).toContain('artwork-b')
    } finally { wrapper.unmount() }
  })
})
