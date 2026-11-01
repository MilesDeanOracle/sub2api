import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(process.cwd(), 'src/components/layout/AppSidebar.vue'), 'utf8')

describe('admin resource pool sidebar entry', () => {
  it('replaces the separate group and account entries and stays before channel management', () => {
    const entry = "{ path: '/admin/resource-pools', label: t('nav.resourcePools'), icon: FolderIcon }"
    expect(source.match(new RegExp(entry.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g'))).toHaveLength(1)
    expect(source).not.toContain("{ path: '/admin/groups', label: t('nav.groups')")
    expect(source).not.toContain("{ path: '/admin/accounts', label: t('nav.accounts')")
    expect(source.indexOf(entry)).toBeLessThan(source.indexOf("path: '/admin/channels'"))
  })
})
