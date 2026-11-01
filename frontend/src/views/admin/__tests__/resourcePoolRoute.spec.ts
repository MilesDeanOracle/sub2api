import { describe, expect, it } from 'vitest'
import { accountGroupFilter, buildResourcePoolQuery, parseResourcePoolRoute, redirectLegacyAccounts, redirectLegacyGroups } from '../resourcePoolRoute'

describe('resource pool route state', () => {
  it('restores all, ungrouped, and selected-group scopes', () => {
    expect(parseResourcePoolRoute({ scope: 'all' }).scope).toBe('all')
    expect(parseResourcePoolRoute({ scope: 'ungrouped' }).scope).toBe('ungrouped')
    expect(parseResourcePoolRoute({ scope: 'group', group_id: '17' })).toMatchObject({ scope: 'group', groupId: 17 })
  })

  it('preserves supported filters and pagination in the URL', () => {
    expect(buildResourcePoolQuery({ scope: 'group', groupId: 9, page: 3, pageSize: 50, search: 'codex', platform: 'openai', status: 'active', type: 'oauth' })).toEqual({
      scope: 'group', group_id: '9', page: '3', page_size: '50', search: 'codex', platform: 'openai', status: 'active', type: 'oauth'
    })
  })

  it('maps scopes to the existing account-list group query', () => {
    expect(accountGroupFilter({ scope: 'all' })).toBeUndefined()
    expect(accountGroupFilter({ scope: 'ungrouped' })).toBe('ungrouped')
    expect(accountGroupFilter({ scope: 'group', groupId: 8 })).toBe('8')
  })

  it('redirects both legacy menus while preserving meaningful query context', () => {
    expect(redirectLegacyAccounts({ search: 'claude' })).toEqual({ path: '/admin/resource-pools', query: { search: 'claude', scope: 'all' } })
    expect(redirectLegacyGroups({ group_id: '12' })).toEqual({ path: '/admin/resource-pools', query: { group_id: '12', scope: 'all', focus: 'groups' } })
  })
})
