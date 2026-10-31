import type { LocationQuery, LocationQueryRaw } from 'vue-router'

export interface ResourcePoolRouteState {
  scope: 'all' | 'ungrouped' | 'issues' | 'group'
  groupId?: number
  page: number
  pageSize: number
  search: string
  platform: string
  status: string
  type: string
}

function first(value: unknown) {
  return Array.isArray(value) ? value[0] : typeof value === 'string' ? value : ''
}

function positive(value: unknown, fallback: number) {
  const parsed = Number(first(value))
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback
}

export function parseResourcePoolRoute(query: LocationQuery): ResourcePoolRouteState {
  const groupId = positive(query.group_id, 0)
  const requestedScope = first(query.scope)
  return {
    scope: requestedScope === 'ungrouped'
      ? 'ungrouped'
      : requestedScope === 'issues'
        ? 'issues'
        : groupId
          ? 'group'
          : 'all',
    groupId: groupId || undefined,
    page: positive(query.page, 1),
    pageSize: positive(query.page_size, 20),
    search: first(query.search),
    platform: first(query.platform),
    status: first(query.status),
    type: first(query.type)
  }
}

export function buildResourcePoolQuery(state: ResourcePoolRouteState): LocationQueryRaw {
  const query: Record<string, string> = { scope: state.scope }
  if (state.groupId) query.group_id = String(state.groupId)
  if (state.page !== 1) query.page = String(state.page)
  if (state.pageSize !== 20) query.page_size = String(state.pageSize)
  for (const key of ['search', 'platform', 'status', 'type'] as const) if (state[key]) query[key] = state[key]
  return query
}

export function accountGroupFilter(state: Pick<ResourcePoolRouteState, 'scope' | 'groupId'>) {
  if (state.scope === 'group') return String(state.groupId)
  if (state.scope === 'ungrouped') return 'ungrouped'
  return undefined
}

export function redirectLegacyAccounts(query: LocationQuery): { path: string; query: LocationQueryRaw } {
  return { path: '/admin/resource-pools', query: { ...query, scope: query.scope ?? 'all' } }
}

export function redirectLegacyGroups(query: LocationQuery): { path: string; query: LocationQueryRaw } {
  return { path: '/admin/resource-pools', query: { ...query, scope: query.scope ?? 'all', focus: 'groups' } }
}
