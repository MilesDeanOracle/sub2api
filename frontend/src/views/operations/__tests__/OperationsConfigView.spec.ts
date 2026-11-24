import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OperationsConfigView from '../OperationsConfigView.vue'

const { accounts, tasks, storage, models, save, setDetectPaused, showError } = vi.hoisted(() => ({ accounts: vi.fn(), tasks: vi.fn(), storage: vi.fn(), models: vi.fn(), save: vi.fn(), setDetectPaused: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin/operationsConsole', () => ({ operationsApi: { accounts, tasks, storage, models, save, setDetectPaused } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
beforeEach(() => { vi.resetAllMocks(); accounts.mockResolvedValue([{ id: 1, name: 'Multi-group account', groups: [{ id: 1, name: 'G1' }, { id: 2, name: 'G2' }], protocol: 'anthropic' }]); tasks.mockResolvedValue([]); storage.mockResolvedValue([]); models.mockResolvedValue({ models: ['upstream-model'], metadata: {} }) })
function mountView() {
  return mount(OperationsConfigView, { global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }, ConfirmDialog: true, Select: true } } })
}
describe('Operations configuration', () => {
  it('selects a multi-group account once and discovers its upstream model', async () => {
    const wrapper = mountView()
    await flushPromises(); await wrapper.findAll('button').find(b => b.text() === 'operationsConsole.create')!.trigger('click')
    const boxes = wrapper.findAll('input[type="checkbox"]')
    expect(boxes).toHaveLength(1)
    expect(wrapper.find('button[role="switch"]').exists()).toBe(true)
    await boxes[0]!.setValue(true); await flushPromises()
    expect(models).toHaveBeenCalledWith(1, false)
    expect(models).toHaveBeenCalledTimes(1)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(showError).toHaveBeenCalledWith('operationsConsole.modelsRequired')
    expect(save).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('starts paused accounts unchecked when editing and resumes detection when re-checked', async () => {
    accounts.mockResolvedValue([
      { id: 1, name: 'Active account', groups: [], protocol: 'anthropic' },
      { id: 2, name: 'Paused account', groups: [], protocol: 'anthropic', detect_paused: true },
    ])
    tasks.mockResolvedValue([{ id: 5, name: 'Plan', enabled: true, interval_hours: 2, account_configs: [{ account_id: 1, model_id: 'upstream-model' }, { account_id: 2, model_id: 'upstream-model' }], reasoning_effort: '', prompt: 'p', storage_source: 'backup_s3', config_version: 1, last_triggered_at: null, next_run_at: null }])
    setDetectPaused.mockResolvedValue({ id: 2, detect_paused: false })
    const wrapper = mountView()
    try {
      await flushPromises()
      await wrapper.find('button[aria-label="operationsConsole.edit"]').trigger('click'); await flushPromises()
      const boxes = wrapper.findAll('input[type="checkbox"]')
      expect(boxes).toHaveLength(2)
      // The paused account starts unchecked in the editor.
      expect((boxes[0]!.element as HTMLInputElement).checked).toBe(true)
      expect((boxes[1]!.element as HTMLInputElement).checked).toBe(false)
      expect(wrapper.text()).toContain('operationsConsole.detectPausedBadge')
      // Re-checking it clears the paused flag so detection actually resumes.
      await boxes[1]!.setValue(true); await flushPromises()
      expect(setDetectPaused).toHaveBeenCalledWith(2, false)
      expect(wrapper.text()).not.toContain('operationsConsole.detectPausedBadge')
    } finally { wrapper.unmount() }
  })
})
