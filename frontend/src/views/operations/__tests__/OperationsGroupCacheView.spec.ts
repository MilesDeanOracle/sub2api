import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import OperationsGroupCacheView from '../OperationsGroupCacheView.vue'
import type { GroupCacheRow } from '@/api/admin/operationsConsole'

const { groupCache, updateGroupCache, showError } = vi.hoisted(() => ({ groupCache: vi.fn(), updateGroupCache: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin/operationsConsole', () => ({ operationsApi: { groupCache, updateGroupCache } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
// utils/format loads the real i18n module, so keep its exports and only stub useI18n.
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function group(overrides: Partial<GroupCacheRow>): GroupCacheRow {
  return {
    group_id: 1, name: 'Claude', platform: 'anthropic', rate_multiplier: 1.25, model_count: 3, cache_hit_rate: 50,
    input_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, setting: { enabled: false, floor: null }, ...overrides
  }
}
const lowEnabled = group({
  group_id: 1, name: 'Low enabled', cache_hit_rate: 64.7, setting: { enabled: true, floor: 85 },
  monitor: { status: 'operational', model: 'claude-sonnet-4' },
  input_tokens: 12464104, cache_creation_tokens: 52012175, cache_read_tokens: 797266866,
})
const lowPaused = group({ group_id: 2, name: 'Low paused', cache_hit_rate: 20.9 })
const highEnabled = group({ group_id: 3, name: 'High enabled', cache_hit_rate: 92.5, setting: { enabled: true, floor: 85 } })
const catalog = [lowEnabled, lowPaused, highEnabled]
// The view writes saved settings back into its rows, so every test gets its own copy.
const freshCatalog = () => catalog.map(item => ({ ...item, setting: { ...item.setting } }))
// The list endpoint answers with a page whose items are always an array.
const page = (items: GroupCacheRow[] = freshCatalog()) => ({ items })
const NO_QUERY = { search: '', platform: '', status: '' }

// Native stand-in for the shared Select so tests can pick an option.
const selectStub = { props: ['modelValue', 'options'], emits: ['update:modelValue'], template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option></select>' }

beforeEach(() => { vi.resetAllMocks(); groupCache.mockResolvedValue(page()) })

async function mountView(): Promise<VueWrapper> {
  // Attached to the document so focus can be asserted on the quick editor.
  const wrapper = mount(OperationsGroupCacheView, { attachTo: document.body, global: { stubs: { Select: selectStub } } })
  await flushPromises()
  return wrapper
}
async function submitQuery(wrapper: VueWrapper) {
  await wrapper.get('[data-testid="group-cache-query-form"]').trigger('submit')
  await flushPromises()
}
function tableRows(wrapper: VueWrapper) { return wrapper.findAll('[data-testid="group-cache-row"]') }
function realRate(row: VueWrapper) { return row.get('[data-testid="group-cache-real-rate"]') }

describe('Operations group cache', () => {
  it('shows real rates and highlights only enabled groups below 70%', async () => {
    const wrapper = await mountView()
    try {
      const rows = tableRows(wrapper)
      expect(rows).toHaveLength(3)
      expect(rows[0]!.text()).toContain('Low enabled')
      expect(rows[0]!.text()).toContain('1.25x')
      expect(rows[0]!.text()).toContain('monitorCommon.status.operational')
      expect(rows[0]!.text()).toContain('claude-sonnet-4')
      expect(rows[1]!.text()).toContain('-')
      expect(realRate(rows[0]!).text()).toBe('64.7%')
      expect(realRate(rows[1]!).text()).toBe('20.9%')
      expect(realRate(rows[2]!).text()).toBe('92.5%')
      expect(realRate(rows[0]!).classes()).toContain('text-red-600')
      expect(realRate(rows[1]!).classes()).not.toContain('text-red-600')
      expect(realRate(rows[2]!).classes()).not.toContain('text-red-600')
      // The real tokens are listed as three labelled rows in the order read, creation, input, using K/M/B units.
      const tokens = rows[0]!.get('[data-testid="group-cache-tokens"]')
      expect(tokens.findAll('dt').map(term => term.text())).toEqual([
        'operationsConsole.groupCacheTokenRead',
        'operationsConsole.groupCacheTokenCreation',
        'operationsConsole.groupCacheTokenInput',
      ])
      expect(tokens.findAll('dd').map(value => value.text())).toEqual(['797.3M', '52.0M', '12.5M'])
      // The floor column shows the configured value next to the real rate.
      expect(rows[0]!.get('[data-testid="group-cache-floor-value"]').text()).toContain('85.0%')
      expect(rows[1]!.get('[data-testid="group-cache-floor-value"]').text()).toContain('operationsConsole.groupCacheUnset')
    } finally { wrapper.unmount() }
  })

  it('saves a floor typed into the quick editor with Enter', async () => {
    updateGroupCache.mockResolvedValue({ enabled: false, floor: 85.5 })
    const wrapper = await mountView()
    try {
      await tableRows(wrapper)[1]!.get('[data-testid="group-cache-floor-value"]').trigger('click')
      const input = tableRows(wrapper)[1]!.get('[data-testid="group-cache-floor-input"]')
      // Opening the editor puts the caret in the field so typing can start at once.
      await flushPromises()
      expect(document.activeElement).toBe(input.element)
      await input.setValue('85.5')
      await input.trigger('keydown.enter')
      await flushPromises()
      expect(updateGroupCache).toHaveBeenCalledTimes(1)
      expect(updateGroupCache).toHaveBeenCalledWith(2, { enabled: false, floor: 85.5 })
      expect(tableRows(wrapper)[1]!.find('[data-testid="group-cache-floor-input"]').exists()).toBe(false)
      expect(tableRows(wrapper)[1]!.get('[data-testid="group-cache-floor-value"]').text()).toContain('85.5%')
    } finally { wrapper.unmount() }
  })

  it('cancels on Esc, saves on blur and skips unchanged values', async () => {
    const wrapper = await mountView()
    try {
      await tableRows(wrapper)[2]!.get('[data-testid="group-cache-floor-value"]').trigger('click')
      await tableRows(wrapper)[2]!.get('[data-testid="group-cache-floor-input"]').setValue('90')
      await tableRows(wrapper)[2]!.get('[data-testid="group-cache-floor-input"]').trigger('keydown.esc')
      expect(updateGroupCache).not.toHaveBeenCalled()
      expect(tableRows(wrapper)[2]!.get('[data-testid="group-cache-floor-value"]').text()).toContain('85.0%')

      // Confirming the unchanged value does not call the API.
      await tableRows(wrapper)[2]!.get('[data-testid="group-cache-floor-value"]').trigger('click')
      await tableRows(wrapper)[2]!.get('[data-testid="group-cache-floor-input"]').trigger('keydown.enter')
      expect(updateGroupCache).not.toHaveBeenCalled()

      // Leaving the field empty clears the floor.
      updateGroupCache.mockResolvedValue({ enabled: true, floor: null })
      await tableRows(wrapper)[2]!.get('[data-testid="group-cache-floor-value"]').trigger('click')
      await tableRows(wrapper)[2]!.get('[data-testid="group-cache-floor-input"]').setValue('')
      await tableRows(wrapper)[2]!.get('[data-testid="group-cache-floor-input"]').trigger('blur')
      await flushPromises()
      expect(updateGroupCache).toHaveBeenCalledTimes(1)
      expect(updateGroupCache).toHaveBeenCalledWith(3, { enabled: true, floor: null })
    } finally { wrapper.unmount() }
  })

  it('rejects floors outside 0 to 100 or with more than one decimal', async () => {
    const wrapper = await mountView()
    try {
      await tableRows(wrapper)[1]!.get('[data-testid="group-cache-floor-value"]').trigger('click')
      await tableRows(wrapper)[1]!.get('[data-testid="group-cache-floor-input"]').setValue('100')
      await tableRows(wrapper)[1]!.get('[data-testid="group-cache-floor-input"]').trigger('keydown.enter')
      expect(showError).toHaveBeenCalledWith('operationsConsole.groupCacheFloorInvalid')
      // Enter keeps the editor open so the value can be corrected.
      expect(tableRows(wrapper)[1]!.find('[data-testid="group-cache-floor-input"]').exists()).toBe(true)

      await tableRows(wrapper)[1]!.get('[data-testid="group-cache-floor-input"]').setValue('85.12')
      await tableRows(wrapper)[1]!.get('[data-testid="group-cache-floor-input"]').trigger('blur')
      expect(showError).toHaveBeenCalledTimes(2)
      // Blur drops the invalid text instead of trapping focus.
      expect(tableRows(wrapper)[1]!.find('[data-testid="group-cache-floor-input"]').exists()).toBe(false)
      expect(updateGroupCache).not.toHaveBeenCalled()
    } finally { wrapper.unmount() }
  })

  it('toggles the enabled state without changing the floor', async () => {
    updateGroupCache.mockResolvedValue({ enabled: true, floor: null })
    const wrapper = await mountView()
    try {
      const toggle = tableRows(wrapper)[1]!.get('button[role="switch"]')
      expect(toggle.attributes('aria-checked')).toBe('false')
      await toggle.trigger('click')
      await flushPromises()
      expect(updateGroupCache).toHaveBeenCalledWith(2, { enabled: true, floor: null })
      expect(tableRows(wrapper)[1]!.get('button[role="switch"]').attributes('aria-checked')).toBe('true')
      expect(tableRows(wrapper)[1]!.text()).toContain('operationsConsole.groupCacheActive')
      expect(realRate(tableRows(wrapper)[1]!).classes()).toContain('text-red-600')
    } finally { wrapper.unmount() }
  })

  it('reports a failed load and shows the empty state', async () => {
    groupCache.mockRejectedValue({ message: 'boom' })
    const wrapper = await mountView()
    try {
      expect(showError).toHaveBeenCalledWith('boom')
      expect(tableRows(wrapper)).toHaveLength(0)
      expect(wrapper.text()).toContain('operationsConsole.groupCacheEmpty')
    } finally { wrapper.unmount() }
  })

  it('keeps a setting saved while a refresh is in flight', async () => {
    let finishRefresh: (value: { items: GroupCacheRow[] }) => void = () => {}
    groupCache
      .mockResolvedValueOnce(page())
      .mockReturnValueOnce(new Promise<{ items: GroupCacheRow[] }>((resolve) => { finishRefresh = resolve }))
    updateGroupCache.mockResolvedValue({ enabled: true, floor: null })
    const wrapper = await mountView()
    try {
      await wrapper.findAll('button').find(button => button.text() === 'operationsConsole.refresh')!.trigger('click')
      await flushPromises()
      await tableRows(wrapper)[1]!.get('button[role="switch"]').trigger('click')
      await flushPromises()
      // The refresh started before the save and still carries the paused state.
      finishRefresh(page())
      await flushPromises()
      expect(tableRows(wrapper)[1]!.get('button[role="switch"]').attributes('aria-checked')).toBe('true')
    } finally { wrapper.unmount() }
  })

  it('queries the server with the form conditions and keeps them on refresh', async () => {
    const wrapper = await mountView()
    try {
      expect(groupCache).toHaveBeenLastCalledWith(NO_QUERY)

      // Editing the form does not query by itself; the table still shows the last result.
      await wrapper.get('[data-testid="group-cache-search"] input').setValue('claude')
      await wrapper.get('[data-testid="group-cache-platform"]').setValue('anthropic')
      await wrapper.get('[data-testid="group-cache-status"]').setValue('enabled')
      expect(groupCache).toHaveBeenCalledTimes(1)
      expect(tableRows(wrapper)).toHaveLength(3)

      await submitQuery(wrapper)
      expect(groupCache).toHaveBeenLastCalledWith({ search: 'claude', platform: 'anthropic', status: 'enabled' })

      // A refresh repeats the query instead of dropping back to the full list.
      await wrapper.findAll('button').find(button => button.text() === 'operationsConsole.refresh')!.trigger('click')
      await flushPromises()
      expect(groupCache).toHaveBeenCalledTimes(3)
      expect(groupCache).toHaveBeenLastCalledWith({ search: 'claude', platform: 'anthropic', status: 'enabled' })
    } finally { wrapper.unmount() }
  })

  it('trims the group name and resets every condition', async () => {
    const wrapper = await mountView()
    try {
      await wrapper.get('[data-testid="group-cache-search"] input').setValue('  claude  ')
      await submitQuery(wrapper)
      expect(groupCache).toHaveBeenLastCalledWith({ search: 'claude', platform: '', status: '' })

      await wrapper.get('[data-testid="group-cache-reset"]').trigger('click')
      await flushPromises()
      expect(groupCache).toHaveBeenLastCalledWith(NO_QUERY)
      expect((wrapper.get('[data-testid="group-cache-search"] input').element as HTMLInputElement).value).toBe('')
      expect((wrapper.get('[data-testid="group-cache-status"]').element as HTMLSelectElement).value).toBe('')
    } finally { wrapper.unmount() }
  })

  it('reloads a status query after a row is toggled', async () => {
    groupCache.mockResolvedValue(page([lowEnabled]))
    updateGroupCache.mockResolvedValue({ enabled: false, floor: 85 })
    const wrapper = await mountView()
    try {
      await wrapper.get('[data-testid="group-cache-status"]').setValue('paused')
      await submitQuery(wrapper)
      expect(groupCache).toHaveBeenLastCalledWith({ search: '', platform: '', status: 'paused' })
      const callsBeforeToggle = groupCache.mock.calls.length

      // The row stops matching the query once it is paused, so the result is fetched again.
      await tableRows(wrapper)[0]!.get('button[role="switch"]').trigger('click')
      await flushPromises()
      expect(updateGroupCache).toHaveBeenCalledWith(1, { enabled: false, floor: 85 })
      expect(groupCache).toHaveBeenCalledTimes(callsBeforeToggle + 1)
      expect(groupCache).toHaveBeenLastCalledWith({ search: '', platform: '', status: 'paused' })
    } finally { wrapper.unmount() }
  })

  it('shows the query empty state when nothing matches', async () => {
    groupCache.mockResolvedValue({ items: [] })
    const wrapper = await mountView()
    try {
      await wrapper.get('[data-testid="group-cache-search"] input').setValue('nomatch')
      await submitQuery(wrapper)
      expect(tableRows(wrapper)).toHaveLength(0)
      expect(wrapper.text()).toContain('operationsConsole.groupCacheQueryEmpty')
      expect(wrapper.text()).not.toContain('operationsConsole.groupCacheEmpty')
    } finally { wrapper.unmount() }
  })
})
