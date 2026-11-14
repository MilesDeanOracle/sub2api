import { shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import type { PublicSettings } from '@/types'
import AppHeader from '../AppHeader.vue'

const mocks = vi.hoisted(() => ({
  app: {
    cachedPublicSettings: null as Partial<PublicSettings> | null,
    contactInfo: '', docUrl: '', showInfo: vi.fn()
  },
  auth: { isAdmin: false, isSimpleMode: false, user: { email: 'user@example.com', role: 'user' } },
  feedback: { pendingCount: 0, unreadCount: 0, refresh: vi.fn(), openDialog: vi.fn() }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => reactive(mocks.app),
  useAuthStore: () => reactive(mocks.auth),
  useOnboardingStore: () => ({ replay: vi.fn() })
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => reactive(mocks.app) }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: [] }) }))
vi.mock('@/stores/feedback', () => ({ useFeedbackStore: () => mocks.feedback }))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ path: '/dashboard', meta: {}, params: {} })
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))

beforeEach(() => {
  mocks.app.cachedPublicSettings = null
  mocks.auth.isAdmin = false
  vi.clearAllMocks()
})

describe('header feature entries', () => {
  it('uses the configured community label and href and hides disabled entries', async () => {
    const app = reactive(mocks.app)
    app.cachedPublicSettings = {
      pelican_enabled: false, feedback_enabled: false, qq_group_enabled: true,
      qq_group_name: '开发者交流', qq_group_url: 'https://example.com/join'
    }
    const wrapper = shallowMount(AppHeader)
    expect(wrapper.find('[data-testid="pelican-entry"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="feedback-entry"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="qq-group-entry"]').text()).toBe('开发者交流')
    expect(wrapper.get('[data-testid="qq-group-entry"]').attributes('href')).toBe('https://example.com/join')
    app.cachedPublicSettings.qq_group_enabled = false
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="qq-group-entry"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows Pelican only to admins when configured and rejects an empty community link', async () => {
    reactive(mocks.app).cachedPublicSettings = {
      pelican_enabled: true, pelican_visibility: 'admin', qq_group_url: ''
    }
    const wrapper = shallowMount(AppHeader)
    expect(wrapper.find('[data-testid="pelican-entry"]').exists()).toBe(false)
    reactive(mocks.auth).isAdmin = true
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="pelican-entry"]').exists()).toBe(true)
    const click = new MouseEvent('click', { cancelable: true })
    wrapper.get('[data-testid="qq-group-entry"]').element.dispatchEvent(click)
    expect(click.defaultPrevented).toBe(true)
    expect(mocks.app.showInfo).toHaveBeenCalledWith('nav.qqGroupLinkMissing')
    wrapper.unmount()
  })
})
