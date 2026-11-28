import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import BackupUploadRestoreDialog from '../BackupUploadRestoreDialog.vue'

const { uploadBackup, getBackup, restoreBackup } = vi.hoisted(() => ({
  uploadBackup: vi.fn(),
  getBackup: vi.fn(),
  restoreBackup: vi.fn(),
}))

vi.mock('@/api', () => ({
  adminAPI: {
    backup: {
      uploadBackup,
      getBackup,
      restoreBackup,
    },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
  }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: (fn: () => unknown) => fn() }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => '',
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params?.percent !== undefined ? `${key}:${params.percent}` : key,
  }),
}))

const BaseDialogStub = {
  props: ['show', 'title', 'width', 'closeOnEscape'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const runningRecord = {
  id: 'up-1',
  status: 'running',
  backup_type: 'postgres',
  file_name: 'upload.sql.gz',
  s3_key: '',
  parts: [],
  size_bytes: 10,
  triggered_by: 'uploaded',
  started_at: '2026-11-28T00:00:00Z',
}

const completedRecord = { ...runningRecord, status: 'completed' }
const restoredRecord = { ...completedRecord, restore_status: 'running' }

const wrappers: ReturnType<typeof mount>[] = []

function mountDialog() {
  const wrapper = mount(BackupUploadRestoreDialog, {
    props: { show: true },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        TotpStepUpDialog: true,
      },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

async function chooseFile(wrapper: ReturnType<typeof mount>, file: File) {
  const input = wrapper.get('input[type="file"]')
  Object.defineProperty(input.element, 'files', {
    configurable: true,
    value: [file],
  })
  await input.trigger('change')
}

function findSubmit(wrapper: ReturnType<typeof mount>) {
  return wrapper.findAll('button').find(button => button.text().includes('admin.backup.upload.submit'))
}

describe('admin BackupUploadRestoreDialog', () => {
  beforeEach(() => {
    uploadBackup.mockReset()
    getBackup.mockReset()
    restoreBackup.mockReset()
    vi.useFakeTimers()
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.useRealTimers()
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('uploads the file, waits for the record, then restores with safety backup', async () => {
    uploadBackup.mockResolvedValue(runningRecord)
    getBackup.mockResolvedValue(completedRecord)
    restoreBackup.mockResolvedValue(restoredRecord)

    const wrapper = mountDialog()
    await chooseFile(wrapper, new File([new Uint8Array([1, 2, 3])], 'backup.sql.gz'))
    await wrapper.get('input[type="password"]').setValue('secret')

    const submit = findSubmit(wrapper)
    expect(submit).toBeTruthy()
    await submit!.trigger('click')
    await flushPromises()

    expect(uploadBackup).toHaveBeenCalledTimes(1)
    expect(restoreBackup).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()

    expect(restoreBackup).toHaveBeenCalledWith('up-1', 'secret', { backupBeforeRestore: true })
    expect(wrapper.emitted('uploaded')).toBeTruthy()
    expect(wrapper.emitted('restore-started')).toBeTruthy()
  })

  it('rejects a backup larger than 5 GB before uploading', async () => {
    const wrapper = mountDialog()
    const oversized = new File([new Uint8Array([1])], 'backup.sql.gz')
    Object.defineProperty(oversized, 'size', { configurable: true, value: 5 * 1024 * 1024 * 1024 + 1 })

    await chooseFile(wrapper, oversized)

    expect(wrapper.text()).toContain('admin.backup.upload.fileTooLarge')
    expect(uploadBackup).not.toHaveBeenCalled()
  })

  it('shows the server body-limit hint when upload is rejected with 413', async () => {
    uploadBackup.mockRejectedValue({ status: 413 })

    const wrapper = mountDialog()
    await chooseFile(wrapper, new File([new Uint8Array([1])], 'backup.sql.gz'))
    await wrapper.get('input[type="password"]').setValue('secret')

    await findSubmit(wrapper)!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.backup.upload.serverLimitHint')
  })

  it('rejects files without a gzip extension', async () => {
    const wrapper = mountDialog()

    await chooseFile(wrapper, new File(['select 1'], 'backup.sql'))

    expect(wrapper.text()).toContain('admin.backup.upload.invalidFile')
  })
})
