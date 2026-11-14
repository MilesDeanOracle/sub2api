import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import PelicanView from '../PelicanView.vue'
import PelicanHistoryDialog from '@/components/pelican/PelicanHistoryDialog.vue'
import type { PelicanStreamHandlers } from '@/api/pelican'
import type { PelicanRunRecord } from '@/lib/pelican/storage'

const mocks = vi.hoisted(() => ({
  streamRun: vi.fn(), reportRun: vi.fn(), saveRun: vi.fn(), showError: vi.fn()
}))

vi.mock('@/api/pelican', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/api/pelican')>(),
  getConfig: vi.fn().mockResolvedValue({
    test: { prompt_version: 'v1', prompt: 'test' },
    reasoning_levels: ['high'], limits: { max_duration_seconds: 600 }
  }),
  listModels: vi.fn().mockResolvedValue({ models: [{ id: 'test-model' }] }),
  streamRun: mocks.streamRun,
  reportRun: mocks.reportRun
}))
vi.mock('@/api/client', () => ({ apiClient: {} }))
vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<main><slot /></main>' }
}))
vi.mock('@/api/keys', () => ({
  default: { list: vi.fn().mockResolvedValue({ items: [{ id: 1, name: 'test-key', key: 'sk-test' }] }) }
}))
vi.mock('@/lib/pelican/storage', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/lib/pelican/storage')>(),
  loadUISettings: () => null,
  saveUISettings: vi.fn(),
  listRuns: vi.fn().mockResolvedValue([]),
  saveRun: mocks.saveRun
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 1 } }) }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showInfo: vi.fn(), showError: mocks.showError })
}))
vi.mock('@/i18n', () => ({ getLocale: () => 'zh' }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const global = {
  stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    BaseDialog: {
      props: ['show'],
      template: '<div v-if="show" role="dialog"><slot /><slot name="footer" /></div>'
    },
    Select: true, Icon: true, PelicanLogo: true, ConfirmDialog: true, RouterLink: true,
    PelicanArtifactPreview: true
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.saveRun.mockResolvedValue(undefined)
  mocks.reportRun.mockResolvedValue({ id: 7 })
  vi.spyOn(performance, 'now').mockReturnValue(1000)
})

afterEach(() => vi.mocked(performance.now).mockRestore())

describe('Pelican reasoning display', () => {
  it('persists reasoning-based TTFT, displays collapsed reasoning and submits only source', async () => {
    let handlers!: PelicanStreamHandlers
    let finish!: () => void
    mocks.streamRun.mockImplementation((_payload, callbacks) => {
      handlers = callbacks
      return new Promise<void>((resolve) => { finish = resolve })
    })
    const wrapper = mount(PelicanView, { global })
    try {
      await flushPromises()
      await wrapper.findAll('button').find((button) => button.text() === 'pelican.start')!.trigger('click')
      vi.mocked(performance.now).mockReturnValue(1250)
      handlers.onFirstToken?.()
      handlers.onReasoningDelta?.('先分析动画结构。')
      await flushPromises()
      const panel = wrapper.get<HTMLDetailsElement>('[data-testid="pelican-reasoning"]')
      expect(panel.element.open).toBe(false)
      expect(panel.text()).toContain('先分析动画结构。')
      expect(wrapper.text()).toContain('pelican.thinking')

      const source = '<!DOCTYPE html><html><body>动画</body></html>'
      vi.mocked(performance.now).mockReturnValue(2000)
      handlers.onDelta(source)
      handlers.onMeta?.({ requestId: 'request-1' })
      vi.mocked(performance.now).mockReturnValue(3000)
      finish()
      await flushPromises()
      const record = mocks.saveRun.mock.calls[0][1] as PelicanRunRecord
      expect(record).toMatchObject({
        reasoningText: '先分析动画结构。', rawText: source, ttftMs: 250, durationMs: 2000
      })
      expect(record.byteSize).toBe(new Blob([record.html ?? '', source, record.reasoningText!]).size)
      expect(wrapper.get<HTMLDetailsElement>('[data-testid="pelican-reasoning"]').element.open).toBe(false)

      const report = wrapper.get('[data-testid="pelican-report"]')
      expect(report.attributes('disabled')).toBeUndefined()
      await report.trigger('click')
      await wrapper.get('[role="dialog"]').findAll('button').find((button) => button.text() === 'pelican.reportSubmit')!.trigger('click')
      await flushPromises()
      expect(mocks.reportRun).toHaveBeenCalledWith(expect.objectContaining({ source, ttft_ms: 250 }))
      expect(wrapper.get('[data-testid="pelican-report"]').attributes('disabled')).toBeDefined()

      // New history records show reasoning; old records without the optional field still open.
      const history = mount(PelicanHistoryDialog, { global, props: { show: true, record } })
      expect(history.get<HTMLDetailsElement>('details').element.open).toBe(false)
      expect(history.get('pre').text()).toBe('先分析动画结构。')
      await history.setProps({ record: { ...record, id: 'old', reasoningText: undefined } })
      expect(history.find('details').exists()).toBe(false)
      history.unmount()
      expect(mocks.showError).not.toHaveBeenCalled()
    } finally {
      wrapper.unmount()
    }
  })

  it('keeps received reasoning when a run fails before producing source', async () => {
    mocks.streamRun.mockImplementation(async (_payload, handlers: PelicanStreamHandlers) => {
      vi.mocked(performance.now).mockReturnValue(1100)
      handlers.onFirstToken?.()
      handlers.onReasoningDelta?.('部分思考')
      throw new TypeError('connection closed')
    })
    const wrapper = mount(PelicanView, { global })
    try {
      await flushPromises()
      await wrapper.findAll('button').find((button) => button.text() === 'pelican.start')!.trigger('click')
      await flushPromises()
      expect(mocks.saveRun).toHaveBeenCalledWith(1, expect.objectContaining({
        status: 'error', reasoningText: '部分思考', rawText: '', ttftMs: 100
      }))
      expect(wrapper.find('[data-testid="pelican-report"]').exists()).toBe(false)
      expect(wrapper.get<HTMLDetailsElement>('details[data-testid="pelican-reasoning"]').element.open).toBe(false)
    } finally {
      wrapper.unmount()
    }
  })
})
