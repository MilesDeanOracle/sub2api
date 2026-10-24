import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'

import type { AdminUser } from '@/types'
import UsersViewElement from '../UsersViewElement.vue'

const {
  listUsers,
  getUserOverview,
  getUserModelStats,
  getUserBalanceHistory,
  getGroupStats,
  getModelStats,
  usageGetStats,
  showError,
  showSuccess,
  getAllGroups,
  getBatchUsersUsage,
  listEnabledDefinitions,
  getBatchUserAttributes
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getUserOverview: vi.fn(),
  getUserModelStats: vi.fn(),
  getUserBalanceHistory: vi.fn(),
  getGroupStats: vi.fn(),
  getModelStats: vi.fn(),
  usageGetStats: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      getUserOverview,
      getUserModelStats,
      getUserBalanceHistory
    },
    groups: {
      getAll: getAllGroups
    },
    dashboard: {
      getBatchUsersUsage,
      getGroupStats,
      getModelStats
    },
    usage: {
      getStats: usageGetStats
    },
    userAttributes: {
      listEnabledDefinitions,
      getBatchUserAttributes
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const { ref } = await vi.importActual<typeof import('vue')>('vue')
  return {
    ...actual,
    useI18n: () => ({
      locale: ref('zh'),
      t: (key: string, params?: { count?: number }) =>
        params?.count === undefined ? key : `${key}:${params.count}`
    })
  }
})

const createAdminUser = (overrides: Partial<AdminUser> = {}): AdminUser => ({
  id: 42,
  username: 'scoped-user',
  email: 'scoped@example.com',
  role: 'user',
  balance: 12.34,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-17T00:00:00Z',
  updated_at: '2026-04-17T00:00:00Z',
  notes: '',
  last_active_at: '2026-04-16T02:00:00Z',
  last_used_at: '2026-04-17T02:00:00Z',
  current_concurrency: 0,
  ...overrides
})

const AppLayoutStub = {
  template: '<div><slot /></div>'
}

const mountView = async () => {
  listUsers.mockResolvedValue({
    items: [createAdminUser()],
    total: 1,
    page: 1,
    page_size: 20,
    pages: 1
  })
  listEnabledDefinitions.mockResolvedValue([])
  getBatchUsersUsage.mockResolvedValue({ stats: {} })
  getAllGroups.mockResolvedValue([])
  getUserOverview.mockResolvedValue({
    recharge: { first_at: null, last_at: null, count: 2, total_amount: 100 },
    usage: { total_requests: 10, total_tokens: 1234, total_actual_cost: 5.5, total_account_cost: 4.4 }
  })
  getUserModelStats.mockResolvedValue({ groups: [] })

  const wrapper = mount(UsersViewElement, {
    global: {
      plugins: [ElementPlus],
      stubs: {
        AppLayout: AppLayoutStub,
        // jsdom 下 ElSelect 存在递归更新的兼容问题，与抽屉逻辑无关，stub 掉
        'el-select': { template: '<div class="el-select-stub" />' }
      }
    }
  })
  await flushPromises()
  return wrapper
}

describe('UsersViewElement 用户详情抽屉', () => {
  it('点击用户名打开抽屉并加载概览与模型统计', async () => {
    const wrapper = await mountView()

    // 用户列（邮箱）渲染为可点击链接
    const trigger = wrapper.find('[data-test="open-user-drawer"]')
    expect(trigger.exists()).toBe(true)
    expect(trigger.text()).toBe('scoped@example.com')

    // 点击前抽屉未加载过数据（EP drawer 可能预渲染隐藏 DOM，以接口调用为准）
    expect(getUserOverview).not.toHaveBeenCalled()

    await trigger.trigger('click')
    await flushPromises()

    // 抽屉打开（append-to-body 挂到 body），且内容是当前用户的邮箱
    const drawer = document.querySelector('.el-drawer')
    expect(drawer).not.toBeNull()
    expect(drawer!.textContent).toContain('scoped@example.com')
    expect(getUserOverview).toHaveBeenCalledWith(42)
    expect(getUserModelStats).toHaveBeenCalledWith(42)

    wrapper.unmount()
  })

  it('新接口不可用时自动回退现有接口（兼容旧后端）', async () => {
    const wrapper = await mountView()

    // 模拟旧后端：新接口 404
    getUserOverview.mockRejectedValueOnce(new Error('404'))
    getUserModelStats.mockRejectedValueOnce(new Error('404'))
    getUserBalanceHistory.mockResolvedValueOnce({
      items: [
        { id: 2, type: 'balance', value: 50, used_at: '2026-04-18T00:00:00Z', created_at: '2026-04-18T00:00:00Z', code: 'bbbb1111' },
        { id: 1, type: 'balance', value: 50, used_at: '2026-04-10T00:00:00Z', created_at: '2026-04-10T00:00:00Z', code: 'aaaa1111' }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
      total_recharged: 100
    })
    getGroupStats.mockResolvedValueOnce({
      groups: [
        { group_id: 3, group_name: '专属组', requests: 8, total_tokens: 500000, account_cost: 1.5, actual_cost: 2.5 }
      ],
      start_date: '2000-01-01',
      end_date: ''
    })
    getModelStats.mockResolvedValueOnce({
      models: [
        { model: 'claude-x', requests: 5, total_tokens: 300000, account_cost: 1.0, actual_cost: 1.8, input_tokens: 1, output_tokens: 1, cache_creation_tokens: 0, cache_read_tokens: 0 }
      ],
      start_date: '2000-01-01',
      end_date: ''
    })
    getAllGroups.mockResolvedValueOnce([
      { id: 3, name: '专属组', rate_multiplier: 1.5 }
    ] as any)
    usageGetStats.mockResolvedValueOnce({
      total_requests: 5,
      total_input_tokens: 1000,
      total_output_tokens: 2000,
      total_cache_tokens: 0,
      total_cache_creation_tokens: 0,
      total_cache_read_tokens: 0,
      total_tokens: 3000,
      total_account_cost: 1.0,
      total_actual_cost: 1.8,
      total_account_cost: 0,
      average_duration_ms: 2000
    })

    const trigger = wrapper.find('[data-test="open-user-drawer"]')
    await trigger.trigger('click')
    await flushPromises()

    // 回退接口被调用（全时间段）
    expect(getUserBalanceHistory).toHaveBeenCalledWith(42, 1, 20, 'balance')
    expect(getGroupStats).toHaveBeenCalledWith({ user_id: 42, start_date: '2000-01-01' })
    expect(getModelStats).toHaveBeenCalledWith({ user_id: 42, group_id: 3, start_date: '2000-01-01' })
    // 平均值来自 usage/stats（user+group+model 过滤）
    expect(usageGetStats).toHaveBeenCalledWith({ user_id: 42, group_id: 3, model: 'claude-x', start_date: '2000-01-01' })

    // 抽屉内容包含分组名与模型名，卡片显示回退数据
    const drawer = document.querySelector('.el-drawer')
    expect(drawer).not.toBeNull()
    expect(drawer!.textContent).toContain('专属组')
    expect(drawer!.textContent).toContain('claude-x')
    expect(drawer!.textContent).toContain('$2.50')
    // 成本列（account_cost）
    expect(drawer!.textContent).toContain('$1.5000')
    // 分组倍率（来自分组列表配置）
    expect(drawer!.textContent).toContain('1.5x')
    // 请求次数
    expect(drawer!.textContent).toContain('8')
    // 利润 = 实际 - 成本：分组 2.5-1.5=1.0，模型 1.8-1.0=0.8
    expect(drawer!.textContent).toContain('$1.0000')
    expect(drawer!.textContent).toContain('$0.8000')
    // 平均耗时 2000ms → "2.00 s"；输出速率 2000tok / (2000ms*5/1000)=10s → 200 tok/s
    expect(drawer!.textContent).toContain('2.00 s')
    expect(drawer!.textContent).toContain('200.0 tok/s')
    // compat 模式列头切换为平均值语义（i18n mock 返回 key）
    expect(drawer!.textContent).toContain('admin.users.drawer.colAvgDuration')
    expect(drawer!.textContent).toContain('admin.users.drawer.colAvgTps')
    // 首字列在 compat 模式下隐藏
    expect(document.querySelector('.el-drawer')!.textContent).not.toContain('admin.users.drawer.colTtft')

    wrapper.unmount()
  })

  it('点击不同用户的邮箱打开对应抽屉', async () => {
    listUsers.mockResolvedValueOnce({
      items: [createAdminUser({ id: 7, email: 'other@example.com' })],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    const wrapper = await mountView()

    const trigger = wrapper.find('[data-test="open-user-drawer"]')
    expect(trigger.text()).toBe('other@example.com')

    await trigger.trigger('click')
    await flushPromises()
    expect(getUserOverview).toHaveBeenCalledWith(7)

    wrapper.unmount()
  })
})
