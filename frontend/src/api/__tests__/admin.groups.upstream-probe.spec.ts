import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { post }
}))

import { probeUpstreamGroups } from '@/api/admin/groups'

describe('admin upstream group probe API', () => {
  beforeEach(() => {
    post.mockReset()
  })

  it('posts ephemeral upstream credentials to the protected probe endpoint', async () => {
    const request = {
      base_url: 'https://upstream.example',
      access_token: 'jwt-access-token'
    }
    const result = {
      groups: [{ id: 1, name: 'Claude Max', platform: 'anthropic', status: 'active' }],
      total: 1
    }
    post.mockResolvedValue({ data: result })

    await expect(probeUpstreamGroups(request)).resolves.toEqual(result)
    expect(post).toHaveBeenCalledWith('/admin/groups/upstream-probe', request)
  })
})
