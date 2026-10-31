<template>
  <el-drawer
    :model-value="modelValue"
    direction="rtl"
    :size="drawerSize"
    :title="t('resourcePool.upstream.title')"
    destroy-on-close
    class="upstream-account-drawer"
    @update:model-value="emit('update:modelValue', $event)"
    @closed="resetState"
  >
    <div class="flex h-full min-h-0 flex-col gap-5">
      <el-alert
        :title="t('resourcePool.upstream.securityHint')"
        type="info"
        :closable="false"
        show-icon
      />

      <div class="flex items-center gap-2">
        <el-select
          v-model="selectedConnectionId"
          class="min-w-0 flex-1"
          filterable
          clearable
          data-testid="upstream-saved-connection"
          :placeholder="t('resourcePool.upstream.savedConnectionPlaceholder')"
          @change="selectSavedConnection"
        >
          <el-option
            v-for="connection in savedConnections"
            :key="connection.id"
            :label="connection.base_url"
            :value="connection.id"
          />
        </el-select>
        <el-button
          plain
          type="danger"
          data-testid="upstream-delete-saved-connection"
          :disabled="!selectedConnectionId"
          @click="deleteSelectedConnection"
        >
          {{ t('resourcePool.upstream.deleteSavedConnection') }}
        </el-button>
      </div>

      <el-form label-position="top" @submit.prevent="probe">
        <el-form-item :label="t('resourcePool.upstream.baseUrl')" required>
          <el-input
            v-model="form.base_url"
            data-testid="upstream-base-url"
            autocomplete="off"
            :placeholder="t('resourcePool.upstream.baseUrlPlaceholder')"
            @keyup.enter="probe"
          />
        </el-form-item>
        <el-form-item :label="t('resourcePool.upstream.accessToken')" required>
          <el-input
            v-model="form.access_token"
            data-testid="upstream-access-token"
            type="password"
            show-password
            autocomplete="off"
            :placeholder="t('resourcePool.upstream.accessTokenPlaceholder')"
            @keyup.enter="probe"
          />
        </el-form-item>
        <div class="flex items-center justify-end">
          <el-button
            type="primary"
            data-testid="upstream-probe"
            :loading="loading"
            :disabled="!canProbe"
            @click="probe"
          >
            {{ t('resourcePool.upstream.probe') }}
          </el-button>
        </div>
      </el-form>

      <el-alert
        v-if="errorMessage"
        data-testid="upstream-probe-error"
        :title="errorMessage"
        type="error"
        :closable="false"
        show-icon
      />

      <section class="flex min-h-0 flex-1 flex-col gap-3">
        <div v-if="probed" class="flex items-center justify-between">
          <h3 class="m-0 text-sm font-semibold text-gray-900 dark:text-gray-100">
            {{ t('resourcePool.upstream.groupsTitle') }}
          </h3>
          <div class="flex items-center gap-2">
            <el-tag type="info" round>
              {{ t('resourcePool.upstream.groupCount', { count: groups.length }) }}
            </el-tag>
            <el-tag type="success" round data-testid="upstream-selected-count">
              {{ t('resourcePool.upstream.selectedCount', { count: selectedGroups.length }) }}
            </el-tag>
            <el-button
              type="primary"
              size="small"
              data-testid="upstream-add-accounts"
              :loading="creatingAccounts"
              :disabled="selectedGroups.length === 0 || loading"
              @click="addSelectedAccounts"
            >
              {{ t('resourcePool.upstream.addAccounts') }}
            </el-button>
          </div>
        </div>

        <el-table
          v-if="probed"
          ref="groupTableRef"
          v-loading="loading"
          :data="groups"
          data-testid="upstream-group-table"
          height="100%"
          border
          stripe
          class="min-h-0 flex-1"
          :empty-text="t('resourcePool.upstream.empty')"
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="48" :selectable="isSelectable" />
          <el-table-column prop="name" :label="t('resourcePool.upstream.columns.name')" min-width="170" show-overflow-tooltip />
          <el-table-column prop="platform" :label="t('resourcePool.upstream.columns.platform')" min-width="120" />
          <el-table-column :label="t('resourcePool.upstream.columns.status')" width="100">
            <template #default="{ row }">
              <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
                {{ statusLabel(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="description" :label="t('resourcePool.upstream.columns.description')" min-width="220" show-overflow-tooltip />
          <el-table-column :label="t('resourcePool.upstream.columns.addable')" width="110">
            <template #default="{ row }">
              <el-tag v-if="isSelectable(row)" type="success" size="small">
                {{ t('resourcePool.upstream.addable') }}
              </el-tag>
              <el-tag v-else-if="row.already_added" type="warning" size="small">
                {{ t('resourcePool.upstream.alreadyAdded') }}
              </el-tag>
              <el-tag v-else type="info" size="small">
                {{ t('resourcePool.upstream.notAddable') }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>

        <el-empty
          v-else
          :description="t('resourcePool.upstream.notProbed')"
          class="flex-1"
        />
      </section>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMediaQuery } from '@vueuse/core'
import { ElMessage, ElMessageBox } from 'element-plus'
import { accountsAPI, groupsAPI } from '@/api/admin'
import type { UpstreamGroupSummary } from '@/api/admin/groups'
import type { CreateAccountRequest } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

defineProps<{ modelValue: boolean }>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: boolean): void
  (event: 'created'): void
}>()

const { t } = useI18n()
const savedConnectionsStorageKey = 'sub2api:resource-pool:upstream-connections:v1'
const isNarrow = useMediaQuery('(max-width: 960px)')
const drawerSize = computed(() => (isNarrow.value ? '100%' : '50%'))
const form = reactive({ base_url: '', access_token: '' })
const savedConnections = ref<SavedUpstreamConnection[]>(loadSavedConnections())
const selectedConnectionId = ref('')
const groups = ref<UpstreamGroupSummary[]>([])
const selectedGroups = ref<UpstreamGroupSummary[]>([])
const groupTableRef = ref<{ clearSelection: () => void } | null>(null)
const loading = ref(false)
const creatingAccounts = ref(false)
const probed = ref(false)
const errorMessage = ref('')
const canProbe = computed(() => Boolean(
  form.base_url.trim() && form.access_token.trim()
) && !loading.value)

// 这些平台均支持通过 Base URL + API Key 创建账号。国产多协议平台在
// buildCredentialsForPlatform 中额外写入按量付费、自适应协议及分协议端点。
const supportedPlatforms = new Set([
  'anthropic',
  'openai',
  'gemini',
  'grok',
  'kimi',
  'zhipu',
  'deepseek',
  'minimax'
])

interface SavedUpstreamConnection {
  id: string
  base_url: string
  access_token: string
}

function loadSavedConnections(): SavedUpstreamConnection[] {
  try {
    const raw = globalThis.localStorage?.getItem(savedConnectionsStorageKey)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((item): item is SavedUpstreamConnection => {
      if (!item || typeof item !== 'object') return false
      const value = item as Partial<SavedUpstreamConnection>
      return typeof value.id === 'string' && typeof value.base_url === 'string' && typeof value.access_token === 'string' &&
        value.base_url.trim() !== '' && value.access_token.trim() !== ''
    })
  } catch {
    return []
  }
}

function persistSavedConnections() {
  try {
    globalThis.localStorage?.setItem(savedConnectionsStorageKey, JSON.stringify(savedConnections.value))
  } catch {
    // Browser storage may be disabled or full; probing still works without persistence.
  }
}

function newConnectionID() {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

function saveSuccessfulConnection() {
  const baseURL = form.base_url.trim()
  const accessToken = form.access_token.trim()
  const existing = savedConnections.value.find(connection => connection.base_url === baseURL)
  const connection: SavedUpstreamConnection = {
    id: existing?.id ?? newConnectionID(),
    base_url: baseURL,
    access_token: accessToken
  }
  savedConnections.value = [connection, ...savedConnections.value.filter(item => item.id !== connection.id)]
  selectedConnectionId.value = connection.id
  persistSavedConnections()
}

function selectSavedConnection(id: string | undefined) {
  if (!id) return
  const connection = savedConnections.value.find(item => item.id === id)
  if (!connection) return
  form.base_url = connection.base_url
  form.access_token = connection.access_token
}

async function deleteSelectedConnection() {
  const id = selectedConnectionId.value
  if (!id) return
  const connection = savedConnections.value.find(item => item.id === id)
  if (!connection) return
  try {
    await ElMessageBox.confirm(
      t('resourcePool.upstream.deleteSavedConnectionConfirm', { baseUrl: connection.base_url }),
      t('resourcePool.upstream.deleteSavedConnection'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )
  } catch {
    return
  }
  savedConnections.value = savedConnections.value.filter(item => item.id !== id)
  selectedConnectionId.value = ''
  persistSavedConnections()
  ElMessage.success(t('resourcePool.upstream.savedConnectionDeleted'))
}

async function probe() {
  if (!canProbe.value) return
  loading.value = true
  errorMessage.value = ''
  groups.value = []
  probed.value = false
  try {
    const result = await groupsAPI.probeUpstreamGroups({
      base_url: form.base_url.trim(),
      access_token: form.access_token.trim()
    })
    groups.value = result.groups.map(group => ({ ...group, already_added: false }))
    await markAlreadyAddedGroups()
    selectedGroups.value = []
    probed.value = true
    saveSuccessfulConnection()
  } catch (error) {
    errorMessage.value = extractApiErrorMessage(error, t('resourcePool.upstream.probeFailed'))
  } finally {
    loading.value = false
  }
}

function handleSelectionChange(selection: UpstreamGroupSummary[]) {
  selectedGroups.value = selection
}

function isSelectable(group: UpstreamGroupSummary) {
  return supportedPlatforms.has(group.platform.trim().toLowerCase()) && Boolean(group.api_key?.trim()) && !group.already_added
}

function normalizeBaseURL(value: unknown) {
  return typeof value === 'string' ? value.trim().replace(/\/+$/, '').toLowerCase() : ''
}

function accountMatchesUpstreamGroup(account: { name?: string; platform?: string; credentials?: Record<string, unknown> }, group: UpstreamGroupSummary, baseURL: string) {
  if (account.platform?.trim().toLowerCase() !== group.platform.trim().toLowerCase()) return false
  const accountBaseURL = normalizeBaseURL(account.credentials?.base_url)
  if (!accountBaseURL || accountBaseURL !== normalizeBaseURL(baseURL)) return false
  const accountName = account.name?.trim() ?? ''
  const groupName = group.name.trim()
  return accountName === groupName || accountName.endsWith(groupName)
}

async function markAlreadyAddedGroups() {
  if (groups.value.length === 0) return
  try {
    const existingAccounts = [] as Array<{ name?: string; platform?: string; credentials?: Record<string, unknown> }>
    let page = 1
    while (page <= 10) {
      const result = await accountsAPI.list(page, 1000, { lite: '1' })
      existingAccounts.push(...result.items)
      if (result.items.length === 0 || page >= result.pages) break
      page += 1
    }
    groups.value = groups.value.map(group => ({
      ...group,
      already_added: existingAccounts.some(account => accountMatchesUpstreamGroup(account, group, form.base_url))
    }))
  } catch {
    // The probe remains usable when the local admin account list is unavailable.
  }
}

function buildModelMapping(models: string[]): Record<string, string> | undefined {
  const mapping = Object.fromEntries(
    models
      .map(model => model.trim())
      .filter(Boolean)
      .map(model => [model, model])
  )
  return Object.keys(mapping).length > 0 ? mapping : undefined
}

function buildCredentialsForPlatform(platform: string, baseURL: string, apiKey: string): Record<string, unknown> {
  const credentials: Record<string, unknown> = {
    base_url: baseURL,
    api_key: apiKey,
    // 与账号创建表单勾选“池模式”时保持一致；未显式填写时后端默认重试 3 次。
    pool_mode: true,
    pool_mode_retry_count: 3
  }

  // 国产平台统一按量付费 + 自适应协议，并把抽屉中的上游地址用于所有
  // 原生协议端点。后端会根据平台能力决定实际是否使用某个协议。
  if (['kimi', 'zhipu', 'deepseek', 'minimax'].includes(platform)) {
    credentials.account_mode = 'payg'
    credentials.api_protocol = 'adaptive'
    credentials.api_base_urls = {
      chat_completions: baseURL,
      anthropic: baseURL,
      responses: baseURL
    }
  }

  return credentials
}

async function addSelectedAccounts() {
  if (selectedGroups.value.length === 0 || creatingAccounts.value) return

  let prefix = ''
  try {
    const result = await ElMessageBox.prompt(
      t('resourcePool.upstream.prefixPrompt'),
      t('resourcePool.upstream.addAccounts'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        inputPlaceholder: t('resourcePool.upstream.prefixPlaceholder'),
        inputValidator: value => value.trim() ? true : t('resourcePool.upstream.prefixRequired')
      }
    )
    prefix = result.value.trim()
  } catch {
    return
  }

  creatingAccounts.value = true
  errorMessage.value = ''
  const requests: CreateAccountRequest[] = []
  const failures: string[] = []

  try {
    for (const group of selectedGroups.value) {
      const platform = group.platform.trim().toLowerCase()
      if (!supportedPlatforms.has(platform)) {
        failures.push(`${group.name}: ${t('resourcePool.upstream.unsupportedPlatform', { platform })}`)
        continue
      }

      const apiKey = group.api_key?.trim()
      if (!apiKey) {
        failures.push(`${group.name}: ${t('resourcePool.upstream.apiKeyRequired')}`)
        continue
      }
      let modelMapping: Record<string, string> | undefined
      try {
        const catalog = await accountsAPI.syncUpstreamModelsPreview({
          platform,
          type: 'apikey',
          base_url: form.base_url.trim(),
          api_key: apiKey
        })
        modelMapping = buildModelMapping(catalog.models)
        if (!modelMapping) {
          failures.push(`${group.name}: ${t('resourcePool.upstream.modelsEmpty')}`)
        }
      } catch (error) {
        failures.push(`${group.name}: ${extractApiErrorMessage(error, t('resourcePool.upstream.modelsSyncFallback'))}`)
      }

      const credentials = buildCredentialsForPlatform(platform, form.base_url.trim(), apiKey)
      if (modelMapping) {
        credentials.model_mapping = modelMapping
      }

      requests.push({
        name: `${prefix}${group.name}`,
        platform,
        type: 'apikey',
        credentials,
        rate_multiplier: Number.isFinite(group.rate_multiplier) ? Math.max(0, group.rate_multiplier) : 1,
        group_ids: [],
        upstream_billing_probe_enabled: true,
        upstream_billing_rate_sync_enabled: true
      })
    }

    if (requests.length > 0) {
      const result = await accountsAPI.batchCreate(requests)
      const failedCreates = result.results.filter(item => !item.success)
      if (failedCreates.length > 0) {
        failures.push(...failedCreates.map(item => item.error || t('resourcePool.upstream.accountCreateFailed')))
      }
      if (result.success > 0) {
        const createdNames = new Set(
          result.results.filter(item => item.success).map(item => item.name).filter((name): name is string => Boolean(name))
        )
        groups.value = groups.value.map(group => ({
          ...group,
          already_added: createdNames.has(`${prefix}${group.name}`) || group.already_added
        }))
        ElMessage.success(t('resourcePool.upstream.accountsCreated', { count: result.success }))
        emit('created')
      }
    }

    if (failures.length > 0) {
      ElMessage.warning(failures.join('\n'))
    }
    if (requests.length > 0) {
      selectedGroups.value = []
      groupTableRef.value?.clearSelection()
    }
  } finally {
    creatingAccounts.value = false
  }
}

function statusLabel(status: string) {
  if (status === 'active') return t('resourcePool.status.active')
  if (status === 'inactive') return t('resourcePool.status.inactive')
  return status
}

function resetState() {
  groupTableRef.value?.clearSelection()
  form.base_url = ''
  form.access_token = ''
  selectedConnectionId.value = ''
  groups.value = []
  selectedGroups.value = []
  errorMessage.value = ''
  probed.value = false
  loading.value = false
  creatingAccounts.value = false
}
</script>
