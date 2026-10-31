import { beforeEach, describe, expect, it, vi } from 'vitest'

const { put, del } = vi.hoisted(() => ({ put: vi.fn(), del: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { put, delete: del } }))

import { removeGroupMembership, setGroupMembership } from '@/api/admin/accounts'

describe('account-group membership API', () => {
  beforeEach(() => { put.mockReset(); del.mockReset() })

  it('updates one relationship with group-relative priority', async () => {
    put.mockResolvedValue({ data: { id: 42 } })
    await setGroupMembership(42, 7, { priority: 9, confirm_mixed_channel_risk: true })
    expect(put).toHaveBeenCalledWith('/admin/accounts/42/groups/7', { priority: 9, confirm_mixed_channel_risk: true })
  })

  it('removes only the selected relationship', async () => {
    del.mockResolvedValue({ data: { id: 42 } })
    await removeGroupMembership(42, 7)
    expect(del).toHaveBeenCalledWith('/admin/accounts/42/groups/7')
  })
})
