import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OperationsConfigView from '../OperationsConfigView.vue'

const { accounts, tasks, storage, models, save, showError } = vi.hoisted(() => ({ accounts: vi.fn(), tasks: vi.fn(), storage: vi.fn(), models: vi.fn(), save: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin/operationsConsole', () => ({ operationsApi: { accounts, tasks, storage, models, save } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
beforeEach(() => { vi.resetAllMocks(); accounts.mockResolvedValue([{ id: 1, name: 'Multi-group account', groups: [{ id: 1, name: 'G1' }, { id: 2, name: 'G2' }], protocol: 'anthropic' }]); tasks.mockResolvedValue([]); storage.mockResolvedValue([]); models.mockResolvedValue({ models: ['upstream-model'], metadata: {} }) })
describe('Operations configuration', () => {
  it('selects a multi-group account once and discovers its upstream model', async () => {
    const wrapper = mount(OperationsConfigView, { global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }, ConfirmDialog: true, Select: true } } })
    await flushPromises(); await wrapper.findAll('button').find(b => b.text() === 'operationsConsole.create')!.trigger('click')
    const boxes = wrapper.findAll('input[type="checkbox"]')
    expect(boxes).toHaveLength(2)
    await boxes[1]!.setValue(true); await flushPromises()
    expect(models).toHaveBeenCalledWith(1, false)
    expect(models).toHaveBeenCalledTimes(1)
    await wrapper.get('form').trigger('submit'); await flushPromises()
    expect(showError).toHaveBeenCalledWith('operationsConsole.modelsRequired')
    expect(save).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
