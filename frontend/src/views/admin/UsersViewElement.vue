<template>
  <AppLayout>
    <el-config-provider :locale="epLocale">
      <div class="ep-users-page">
        <!-- Page Top Header -->
        <div class="page-top-header flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-gray-200/70 dark:border-dark-800">
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-2xl bg-gradient-to-tr from-primary-600 to-indigo-500 text-white flex items-center justify-center shadow-md shadow-primary-500/20 shrink-0">
              <el-icon class="text-xl"><UserFilledIcon /></el-icon>
            </div>
            <div>
              <h1 class="text-xl font-bold text-gray-900 dark:text-gray-100 tracking-tight m-0">
                {{ t('admin.users.title') }}
              </h1>
              <p class="text-xs text-gray-500 dark:text-gray-400 m-0 mt-0.5">
                {{ t('admin.users.description') }}
              </p>
            </div>
          </div>

          <!-- Quick Metrics & Top Actions -->
          <div class="flex items-center flex-wrap gap-2.5">
            <!-- Header Badges -->
            <div class="hidden lg:flex items-center gap-2 mr-1 text-xs">
              <span class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-gray-100 dark:bg-dark-800 text-gray-600 dark:text-gray-300 font-medium border border-gray-200/60 dark:border-dark-700">
                <span class="text-gray-400 font-normal">{{ t('common.total') || '总计' }}:</span>
                <strong class="text-gray-900 dark:text-gray-100 font-semibold">{{ pagination.total }}</strong>
              </span>
              <span class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-emerald-50 dark:bg-emerald-950/40 text-emerald-700 dark:text-emerald-300 font-medium border border-emerald-200/60 dark:border-emerald-800/60">
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-500" />
                <span class="text-emerald-600 dark:text-emerald-400 font-normal">{{ t('common.active') }}:</span>
                <strong>{{ activeUsersCount }}</strong>
              </span>
              <span v-if="disabledUsersCount > 0" class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-rose-50 dark:bg-rose-950/40 text-rose-700 dark:text-rose-300 font-medium border border-rose-200/60 dark:border-rose-800/60">
                <span class="w-1.5 h-1.5 rounded-full bg-rose-500" />
                <span class="text-rose-600 dark:text-rose-400 font-normal">{{ t('admin.users.disabled') }}:</span>
                <strong>{{ disabledUsersCount }}</strong>
              </span>
              <span v-if="adminUsersCount > 0" class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-amber-50 dark:bg-amber-950/40 text-amber-700 dark:text-amber-300 font-medium border border-amber-200/60 dark:border-amber-800/60">
                <span class="w-1.5 h-1.5 rounded-full bg-amber-500" />
                <span class="text-amber-600 dark:text-amber-400 font-normal">{{ t('admin.users.admin') }}:</span>
                <strong>{{ adminUsersCount }}</strong>
              </span>
            </div>

            <!-- Refresh Button -->
            <el-tooltip :content="t('common.refresh')" placement="top">
              <el-button
                plain
                circle
                :icon="RefreshIcon"
                :class="{ 'ep-refresh-loading': loading }"
                class="!border-gray-200 dark:!border-dark-700 hover:!border-primary-500"
                @click="loadUsers"
              />
            </el-tooltip>

            <!-- Attributes Config Button -->
            <el-tooltip :content="t('admin.users.attributes.configButton')" placement="top">
              <el-button plain class="!rounded-xl text-xs font-medium" @click="showAttributesModal = true">
                <el-icon class="mr-1"><SettingIcon /></el-icon>
                <span class="hidden sm:inline">{{ t('admin.users.attributes.configButton') }}</span>
              </el-button>
            </el-tooltip>

            <!-- Create User Button -->
            <el-button
              type="primary"
              class="!rounded-xl text-xs font-medium shadow-sm hover:shadow"
              @click="showCreateModal = true"
            >
              <el-icon class="mr-1"><PlusIcon /></el-icon>
              {{ t('admin.users.createUser') }}
            </el-button>
          </div>
        </div>

        <!-- Filter Card -->
        <div class="ep-filter-card bg-white dark:bg-dark-900 border border-gray-200/80 dark:border-dark-700/80 rounded-2xl p-3.5 shadow-sm flex flex-col gap-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <!-- Left: Search and active filters -->
            <div class="flex flex-1 flex-wrap items-center gap-2.5 min-w-0">
              <!-- Search Box -->
              <div class="w-full sm:w-64">
                <el-input
                  v-model="searchQuery"
                  clearable
                  :placeholder="t('admin.users.searchUsers')"
                  :prefix-icon="SearchIcon"
                  class="ep-filter-input"
                  @input="handleSearch"
                  @clear="handleSearch"
                />
              </div>

              <!-- Role Filter (visible when enabled) -->
              <div v-if="visibleFilters.has('role')" class="w-full sm:w-36">
                <el-select v-model="filters.role" clearable :placeholder="t('admin.users.allRoles')" @change="applyFilter">
                  <el-option value="" :label="t('admin.users.allRoles')" />
                  <el-option value="admin" :label="t('admin.users.admin')" />
                  <el-option value="user" :label="t('admin.users.user')" />
                </el-select>
              </div>

              <!-- Status Filter (visible when enabled) -->
              <div v-if="visibleFilters.has('status')" class="w-full sm:w-36">
                <el-select v-model="filters.status" clearable :placeholder="t('admin.users.allStatus')" @change="applyFilter">
                  <el-option value="" :label="t('admin.users.allStatus')" />
                  <el-option value="active" :label="t('common.active')" />
                  <el-option value="disabled" :label="t('admin.users.disabled')" />
                </el-select>
              </div>

              <!-- Group Filter (visible when enabled) -->
              <div v-if="visibleFilters.has('group')" class="w-full sm:w-44">
                <el-select
                  v-model="filters.group"
                  clearable
                  filterable
                  allow-create
                  :placeholder="t('admin.users.fuzzySearch')"
                  @change="applyFilter"
                >
                  <el-option
                    v-for="opt in groupFilterOptions"
                    :key="opt.value"
                    :value="opt.value"
                    :label="opt.label"
                  />
                </el-select>
              </div>

              <!-- API Key Group Filter (visible when enabled) -->
              <div v-if="visibleFilters.has('apiKeyGroup')" class="w-full sm:w-48">
                <el-select
                  v-model="filters.apiKeyGroup"
                  clearable
                  filterable
                  :placeholder="t('admin.users.searchApiKeyGroups')"
                  @change="applyFilter"
                >
                  <el-option
                    v-for="opt in apiKeyGroupFilterOptions"
                    :key="String(opt.value)"
                    :value="opt.value"
                    :label="opt.label"
                    :disabled="opt.disabled === true"
                  />
                </el-select>
              </div>

              <!-- Dynamic Attribute Filters -->
              <template v-for="(value, attrId) in activeAttributeFilters" :key="attrId">
                <div
                  v-if="visibleFilters.has(`attr_${attrId}`)"
                  class="w-full sm:w-40"
                >
                  <!-- Select/Multi-select type -->
                  <el-select
                    v-if="['select', 'multi_select'].includes(getAttributeDefinition(Number(attrId))?.type || '')"
                    :model-value="value"
                    clearable
                    :placeholder="getAttributeDefinitionName(Number(attrId))"
                    @update:model-value="(val: SelectEmittedValue) => { updateAttributeFilter(Number(attrId), String(val ?? '')); applyFilter() }"
                  >
                    <el-option value="" :label="getAttributeDefinitionName(Number(attrId))" />
                    <el-option
                      v-for="opt in getAttributeDefinition(Number(attrId))?.options || []"
                      :key="opt.value"
                      :value="opt.value"
                      :label="opt.label"
                    />
                  </el-select>
                  <!-- Number type -->
                  <el-input
                    v-else-if="getAttributeDefinition(Number(attrId))?.type === 'number'"
                    :model-value="value"
                    type="number"
                    clearable
                    :placeholder="getAttributeDefinitionName(Number(attrId))"
                    @update:model-value="(val: SelectEmittedValue) => updateAttributeFilter(Number(attrId), String(val ?? ''))"
                    @keyup.enter="applyFilter"
                  />
                  <!-- Text/Email/URL/Textarea/Date type -->
                  <el-input
                    v-else
                    :model-value="value"
                    clearable
                    :placeholder="getAttributeDefinitionName(Number(attrId))"
                    @update:model-value="(val: SelectEmittedValue) => updateAttributeFilter(Number(attrId), String(val ?? ''))"
                    @keyup.enter="applyFilter"
                  />
                </div>
              </template>

              <!-- Reset Active Filters Button -->
              <button
                v-if="hasActiveFilters"
                type="button"
                class="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium text-primary-600 dark:text-primary-400 bg-primary-50 dark:bg-primary-950/50 hover:bg-primary-100 dark:hover:bg-primary-900/40 transition-colors"
                @click="resetAllFilters"
              >
                <el-icon><CloseIcon /></el-icon>
                <span>{{ t('common.reset') || '重置筛选' }}</span>
              </button>
            </div>

            <!-- Right: Filter Settings & Column Settings -->
            <div class="flex items-center gap-2 shrink-0">
              <!-- Filter Settings Dropdown -->
              <el-dropdown trigger="click" popper-class="ep-check-dropdown">
                <el-button plain class="!rounded-xl text-xs" :title="t('admin.users.filterSettings')">
                  <el-icon class="el-icon--left"><FilterIcon /></el-icon>
                  <span class="hidden md:inline">{{ t('admin.users.filterSettings') }}</span>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item
                      v-for="filter in builtInFilters"
                      :key="filter.key"
                      @click="toggleBuiltInFilter(filter.key)"
                    >
                      <span class="ep-check-row">
                        <span>{{ filter.name }}</span>
                        <el-icon v-if="visibleFilters.has(filter.key)" class="ep-check-row-mark"><CheckIcon /></el-icon>
                      </span>
                    </el-dropdown-item>
                    <div v-if="filterableAttributes.length > 0" class="ep-dropdown-divider"></div>
                    <el-dropdown-item
                      v-for="attr in filterableAttributes"
                      :key="attr.id"
                      @click="toggleAttributeFilter(attr)"
                    >
                      <span class="ep-check-row">
                        <span>{{ attr.name }}</span>
                        <el-icon v-if="visibleFilters.has(`attr_${attr.id}`)" class="ep-check-row-mark"><CheckIcon /></el-icon>
                      </span>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>

              <!-- Column Settings Dropdown -->
              <el-dropdown trigger="click" popper-class="ep-check-dropdown ep-column-dropdown">
                <el-button plain class="!rounded-xl text-xs" :title="t('admin.users.columnSettings')">
                  <el-icon class="el-icon--left"><GridIcon /></el-icon>
                  <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item
                      v-for="col in toggleableColumns"
                      :key="col.key"
                      :disabled="isForcedVisibleColumn(col.key)"
                      :title="isForcedVisibleColumn(col.key) ? t('admin.users.columnAlwaysVisible') : ''"
                      @click="toggleColumn(col.key)"
                    >
                      <span class="ep-check-row" :class="{ 'is-disabled': isForcedVisibleColumn(col.key) }">
                        <span>{{ col.label }}</span>
                        <el-icon v-if="isColumnVisible(col.key)" class="ep-check-row-mark"><CheckIcon /></el-icon>
                      </span>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>
        </div>

        <!-- Floating Batch Selection Dock -->
        <transition name="el-zoom-in-top">
          <div
            v-if="selectedCount > 0"
            class="ep-batch-dock p-3 rounded-2xl bg-gradient-to-r from-primary-50 via-indigo-50/70 to-blue-50/60 dark:from-primary-950/40 dark:via-indigo-950/30 dark:to-blue-950/30 border border-primary-200/70 dark:border-primary-800/50 shadow-sm flex flex-wrap items-center justify-between gap-3 shrink-0"
          >
            <div class="flex items-center gap-2.5">
              <span class="relative flex h-2.5 w-2.5">
                <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary-400 opacity-75"></span>
                <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-primary-600"></span>
              </span>
              <span class="text-sm font-semibold text-primary-900 dark:text-primary-100">
                {{ t('admin.users.bulkLimits.selectedCount', { count: selectedCount }) }}
              </span>
            </div>
            <div class="flex items-center flex-wrap gap-2">
              <el-button
                plain
                size="small"
                data-test="bulk-edit-limits"
                :icon="UserIcon"
                class="!rounded-xl font-medium"
                @click="showBulkEditModal = true"
              >
                {{ t('admin.users.bulkLimits.action', { count: selectedCount }) }}
              </el-button>

              <el-button
                type="danger"
                plain
                size="small"
                data-test="bulk-delete-users"
                :disabled="bulkDeleting"
                :icon="DeleteIcon"
                class="!rounded-xl font-medium"
                @click="handleBulkDeleteClick"
              >
                {{ t('admin.users.bulkDelete.action', { count: selectedCount }) }}
              </el-button>

              <el-button
                size="small"
                text
                class="!rounded-xl text-gray-500 hover:text-gray-700 dark:text-dark-400"
                @click="clearTableSelection"
              >
                {{ t('common.cancel') }}
              </el-button>
            </div>
          </div>
        </transition>

        <!-- Users Table Card -->
        <div class="ep-table-card bg-white dark:bg-dark-900 border border-gray-200/80 dark:border-dark-700/80 rounded-2xl shadow-sm overflow-hidden flex flex-col flex-1 min-h-0">
          <el-table
            ref="tableRef"
            v-loading="loading"
            :data="sortedUsers"
            row-key="id"
            :height="isMobile ? undefined : '100%'"
            :default-sort="defaultTableSort"
            class="ep-refined-table"
            @sort-change="handleSortChange"
            @selection-change="handleSelectionChange"
          >
            <el-table-column type="selection" width="44" reserve-selection />

            <el-table-column prop="email" :label="t('admin.users.columns.user')" min-width="240" sortable="custom" :sort-orders="SORT_ORDERS">
              <template #default="{ row }">
                <el-tooltip :content="t('admin.users.drawer.viewHint')" placement="top">
                  <div class="flex items-center gap-2.5 py-1">
                    <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gradient-to-tr from-primary-500 to-indigo-600 text-white font-semibold text-xs shadow-sm shadow-primary-500/20">
                      {{ (row.email || 'U').charAt(0).toUpperCase() }}
                    </div>
                    <div class="min-w-0 flex flex-col items-start leading-tight">
                      <div class="flex items-center gap-1.5 max-w-full">
                        <el-button
                          link
                          type="primary"
                          class="ep-username-link truncate font-semibold !text-sm"
                          data-test="open-user-drawer"
                          @click.stop="openUserDrawer(row)"
                        >
                          {{ row.email }}
                        </el-button>
                        <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-mono font-medium text-gray-500 bg-gray-100 dark:bg-dark-800 dark:text-dark-400 shrink-0">
                          #{{ row.id }}
                        </span>
                      </div>
                      <span v-if="row.username" class="text-xs text-gray-400 dark:text-dark-400 truncate max-w-[190px] mt-0.5">
                        @{{ row.username }}
                      </span>
                    </div>
                  </div>
                </el-tooltip>
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('id')" prop="id" :label="t('admin.users.columns.id')" min-width="70" sortable="custom" :sort-orders="SORT_ORDERS" />

            <el-table-column v-if="isColumnVisible('username')" prop="username" :label="t('admin.users.columns.username')" min-width="110" sortable="custom" :sort-orders="SORT_ORDERS">
              <template #default="{ row }">
                <span class="text-sm text-gray-700 dark:text-gray-300">{{ row.username || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('notes')" :label="t('admin.users.columns.notes')" min-width="150">
              <template #default="{ row }">
                <div class="max-w-xs">
                  <span
                    v-if="row.notes"
                    :title="row.notes.length > 30 ? row.notes : undefined"
                    class="block truncate text-sm text-gray-600 dark:text-gray-400"
                  >
                    {{ row.notes.length > 30 ? row.notes.substring(0, 25) + '...' : row.notes }}
                  </span>
                  <span v-else class="text-sm text-gray-400">-</span>
                </div>
              </template>
            </el-table-column>

            <!-- Dynamic attribute columns -->
            <template v-for="def in enabledAttributeDefinitions" :key="`attr_${def.id}`">
              <el-table-column
                v-if="isColumnVisible(`attr_${def.id}`)"
                :label="def.name"
                min-width="130"
              >
                <template #default="{ row }">
                  <div class="max-w-xs">
                    <span
                      class="block truncate text-sm text-gray-700 dark:text-gray-300"
                      :title="getAttributeValue(row.id, def.id)"
                    >
                      {{ getAttributeValue(row.id, def.id) }}
                    </span>
                  </div>
                </template>
              </el-table-column>
            </template>

            <el-table-column v-if="isColumnVisible('role')" prop="role" :label="t('admin.users.columns.role')" min-width="110" sortable="custom" :sort-orders="SORT_ORDERS">
              <template #default="{ row }">
                <span
                  v-if="row.role === 'admin'"
                  class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-50 text-amber-700 border border-amber-200/80 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-800/60"
                >
                  <el-icon class="text-[11px]"><StarIcon /></el-icon>
                  <span>{{ t('admin.users.roles.' + row.role) }}</span>
                </span>
                <span
                  v-else
                  class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-600 border border-gray-200/60 dark:bg-dark-800 dark:text-dark-300 dark:border-dark-700"
                >
                  {{ t('admin.users.roles.' + row.role) }}
                </span>
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('groups')" :label="t('admin.users.columns.groups')" min-width="130">
              <template #default="{ row }">
                <div v-if="allGroups.length > 0" class="flex flex-col gap-1">
                  <!-- 专属分组行：悬停列出分组，点击分组可替换 -->
                  <el-popover
                    v-if="getUserGroups(row).exclusive.length > 0"
                    trigger="hover"
                    placement="bottom-start"
                    :width="200"
                    :show-arrow="false"
                  >
                    <template #reference>
                      <span class="inline-flex cursor-pointer items-center gap-1 whitespace-nowrap text-xs">
                        <el-icon class="h-3.5 w-3.5 text-purple-500 dark:text-purple-400"><StarIcon /></el-icon>
                        <span class="font-medium text-purple-600 dark:text-purple-400">{{ getUserGroups(row).exclusive.length }}</span>
                        <span class="text-gray-500 dark:text-dark-400">{{ t('admin.users.exclusiveLabel') }}</span>
                      </span>
                    </template>
                    <div class="text-[10px] font-medium uppercase tracking-wider text-gray-400">
                      {{ t('admin.users.clickToReplace') }}
                    </div>
                    <div
                      v-for="g in getUserGroups(row).exclusive"
                      :key="g.id"
                      class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-xs text-gray-700 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:text-dark-200 dark:hover:bg-primary-900/30"
                      @click="openGroupReplace(row, g)"
                    >
                      <el-icon class="h-3.5 w-3.5 flex-shrink-0 opacity-50"><SwitchIcon /></el-icon>
                      <span class="flex-1">{{ g.name }}</span>
                    </div>
                  </el-popover>
                  <!-- 公开分组行：悬停提示 -->
                  <el-tooltip
                    v-if="getUserGroups(row).publicGroups.length > 0"
                    placement="bottom"
                    effect="dark"
                  >
                    <template #content>
                      <div class="flex flex-col gap-0.5">
                        <span v-for="g in getUserGroups(row).publicGroups" :key="g.id">{{ g.name }}</span>
                      </div>
                    </template>
                    <span class="inline-flex cursor-default items-center gap-1 whitespace-nowrap text-xs">
                      <el-icon class="h-3.5 w-3.5 text-gray-400 dark:text-dark-500"><ConnectionIcon /></el-icon>
                      <span class="font-medium text-gray-600 dark:text-dark-300">{{ getUserGroups(row).publicGroups.length }}</span>
                      <span class="text-gray-400 dark:text-dark-500">{{ t('admin.users.publicLabel') }}</span>
                    </span>
                  </el-tooltip>
                  <!-- 都没有 -->
                  <span
                    v-if="getUserGroups(row).exclusive.length === 0 && getUserGroups(row).publicGroups.length === 0"
                    class="text-xs text-gray-400 dark:text-dark-500"
                  >-</span>
                </div>
                <span v-else class="text-xs text-gray-400 dark:text-dark-500">-</span>
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('subscriptions')" :label="t('admin.users.columns.subscriptions')" min-width="160">
              <template #default="{ row }">
                <div
                  v-if="row.subscriptions && row.subscriptions.length > 0"
                  class="flex flex-wrap gap-1.5"
                >
                  <GroupBadge
                    v-for="sub in row.subscriptions"
                    :key="sub.id"
                    :name="sub.group?.name || ''"
                    :platform="sub.group?.platform"
                    :subscription-type="sub.group?.subscription_type"
                    :rate-multiplier="sub.group?.rate_multiplier"
                    :days-remaining="sub.expires_at ? getDaysRemaining(sub.expires_at) : null"
                    :title="sub.expires_at ? formatDateTime(sub.expires_at) : ''"
                  />
                </div>
                <span v-else class="inline-flex items-center gap-1.5 rounded-md bg-gray-50 px-2 py-1 text-xs text-gray-400 dark:bg-dark-700/50 dark:text-dark-500">
                  <el-icon class="h-3.5 w-3.5"><CircleCloseIcon /></el-icon>
                  <span>{{ t('admin.users.noSubscription') }}</span>
                </span>
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('balance')" prop="balance" :label="t('admin.users.columns.balance')" min-width="140" sortable="custom" :sort-orders="SORT_ORDERS" align="left">
              <template #default="{ row }">
                <div class="flex items-center gap-2">
                  <el-tooltip :content="t('admin.users.balanceHistoryTip')" placement="top">
                    <button class="ep-link-btn" @click="handleBalanceHistory(row)">
                      ${{ row.balance.toFixed(2) }}
                    </button>
                  </el-tooltip>
                  <el-button link type="success" size="small" @click.stop="handleDeposit(row)">
                    {{ t('admin.users.deposit') }}
                  </el-button>
                </div>
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('balance_platform_quota')" :label="t('admin.users.columns.balancePlatformQuota')" min-width="150">
              <template #default="{ row }">
                <el-tooltip :content="t('admin.users.platformQuota.cellColumnTooltip')" placement="top">
                  <button class="ep-link-btn" @click="handlePlatformQuota(row)">
                    <UserPlatformQuotaCell :quotas="platformQuotaStats[row.id]" />
                  </button>
                </el-tooltip>
              </template>
            </el-table-column>

            <!-- 用量列：表头带"今日/近30天"三态排序菜单，与原页保持一致 -->
            <el-table-column
              v-for="usageKey in visibleUsageColumnKeys"
              :key="usageKey"
              :label="usageColumnLabel(usageKey)"
              min-width="140"
            >
              <template #header>
                <div class="flex items-center gap-1.5">
                  <span>{{ usageColumnLabel(usageKey) }}</span>
                  <el-dropdown
                    trigger="click"
                    @command="(metric: string) => toggleUsageSort(usageKey, metric === 'total' ? 'total' : 'today')"
                  >
                    <span
                      class="ep-usage-sort-trigger"
                      :class="{ 'is-active': usageSort?.key === usageKey }"
                      :data-test="`usage-sort-trigger-${usageKey}`"
                      @click.stop
                    >
                      <span v-if="usageSort?.key === usageKey" class="text-[10px] font-medium tracking-normal">
                        {{ usageSort.metric === 'today' ? t('admin.users.today') : t('admin.users.total') }}
                      </span>
                      <el-icon :class="{ 'ep-rotate-180': usageSort?.key === usageKey && usageSort.order === 'desc' }">
                        <SortIcon />
                      </el-icon>
                    </span>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item
                          v-for="metric in (['today', 'total'] as const)"
                          :key="metric"
                          :command="metric"
                          :class="{ 'ep-active-item': isUsageSortActive(usageKey, metric) }"
                          :data-test="`usage-sort-${usageKey}-${metric}`"
                        >
                          <span class="flex w-full items-center justify-between gap-3">
                            <span>{{ metric === 'today' ? t('admin.users.today') : t('admin.users.total') }}</span>
                            <el-icon v-if="getUsageSortOrder(usageKey, metric)" :class="{ 'ep-rotate-180': getUsageSortOrder(usageKey, metric) === 'desc' }">
                              <CaretTopIcon />
                            </el-icon>
                          </span>
                        </el-dropdown-item>
                        <div class="px-3 py-1 text-[10px] text-gray-400">
                          {{ t('admin.users.sortCurrentPageOnly') }}
                        </div>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
              <template #default="{ row }">
                <PlatformUsageBreakdown
                  v-if="usageKey === 'usage'"
                  :today="usageStats[row.id]?.today_actual_cost ?? 0"
                  :total="usageStats[row.id]?.total_actual_cost ?? 0"
                  :by-platform="usageStats[row.id]?.by_platform"
                />
                <PlatformCostCell v-else :usage="getPlatformUsage(row.id, USAGE_COLUMN_PLATFORMS[usageKey]!)" />
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('concurrency')" prop="concurrency" :label="t('admin.users.columns.concurrency')" min-width="110" sortable="custom" :sort-orders="SORT_ORDERS">
              <template #default="{ row }">
                <UserConcurrencyCell :current="row.current_concurrency ?? 0" :max="row.concurrency" />
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('status')" prop="status" :label="t('admin.users.columns.status')" min-width="110" sortable="custom" :sort-orders="SORT_ORDERS">
              <template #default="{ row }">
                <span
                  v-if="row.status === 'active'"
                  class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200/80 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-800/60"
                >
                  <span class="relative flex h-2 w-2">
                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                    <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                  </span>
                  <span>{{ t('common.active') }}</span>
                </span>
                <span
                  v-else
                  class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200/80 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-800/60"
                >
                  <span class="h-2 w-2 rounded-full bg-rose-500"></span>
                  <span>{{ t('admin.users.disabled') }}</span>
                </span>
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('last_active_at')" prop="last_active_at" :label="t('admin.users.columns.lastActive')" min-width="160" sortable="custom" :sort-orders="SORT_ORDERS">
              <template #default="{ row }">
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  {{ row.last_active_at ? formatDateTime(row.last_active_at) : '-' }}
                </span>
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('last_used_at')" prop="last_used_at" :label="t('admin.users.columns.lastUsed')" min-width="160" sortable="custom" :sort-orders="SORT_ORDERS">
              <template #default="{ row }">
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  {{ row.last_used_at ? formatDateTime(row.last_used_at) : '-' }}
                </span>
              </template>
            </el-table-column>

            <el-table-column v-if="isColumnVisible('created_at')" prop="created_at" :label="t('admin.users.columns.created')" min-width="160" sortable="custom" :sort-orders="SORT_ORDERS">
              <template #default="{ row }">
                <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(row.created_at) }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t('admin.users.columns.actions')" width="180" fixed="right" align="center">
              <template #default="{ row }">
                <div class="flex items-center justify-center gap-1.5">
                  <!-- Edit Button -->
                  <el-tooltip :content="t('common.edit')" placement="top">
                    <el-button
                      size="small"
                      plain
                      class="!rounded-lg !px-2"
                      @click="handleEdit(row)"
                    >
                      <el-icon><EditIcon /></el-icon>
                      <span class="ml-1 text-xs">{{ t('common.edit') }}</span>
                    </el-button>
                  </el-tooltip>

                  <!-- Toggle Status Button (not for admin) -->
                  <el-tooltip
                    v-if="row.role !== 'admin'"
                    :content="row.status === 'active' ? t('admin.users.disable') : t('admin.users.enable')"
                    placement="top"
                  >
                    <el-button
                      size="small"
                      :type="row.status === 'active' ? 'warning' : 'success'"
                      plain
                      class="!rounded-lg !px-2"
                      @click="handleToggleStatus(row)"
                    >
                      <el-icon><LockIcon v-if="row.status === 'active'" /><CircleCheckIcon v-else /></el-icon>
                      <span class="ml-1 text-xs">{{ row.status === 'active' ? t('admin.users.disable') : t('admin.users.enable') }}</span>
                    </el-button>
                  </el-tooltip>

                  <!-- More Actions Menu -->
                  <el-dropdown trigger="click" @command="(cmd: string) => handleRowCommand(cmd, row)">
                    <el-button
                      size="small"
                      plain
                      class="!rounded-lg !px-1.5"
                      @click.stop
                    >
                      <el-icon><MoreFilledIcon /></el-icon>
                    </el-button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="apiKeys" :icon="KeyIcon">
                          {{ t('admin.users.apiKeys') }}
                        </el-dropdown-item>
                        <el-dropdown-item command="groups" :icon="UserFilledIcon">
                          {{ t('admin.users.groups') }}
                        </el-dropdown-item>
                        <el-dropdown-item command="deposit" divided :icon="PlusIcon">
                          {{ t('admin.users.deposit') }}
                        </el-dropdown-item>
                        <el-dropdown-item command="withdraw" :icon="MinusIcon">
                          {{ t('admin.users.withdraw') }}
                        </el-dropdown-item>
                        <el-dropdown-item command="platformQuota" :icon="DataLineIcon">
                          {{ t('admin.users.platformQuota.menuItem') }}
                        </el-dropdown-item>
                        <el-dropdown-item command="balanceHistory" :icon="CoinIcon">
                          {{ t('admin.users.balanceHistory') }}
                        </el-dropdown-item>
                        <el-dropdown-item
                          v-if="row.role !== 'admin'"
                          command="delete"
                          divided
                          :icon="DeleteIcon"
                          class="ep-danger-item"
                        >
                          {{ t('common.delete') }}
                        </el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>

            <template #empty>
              <div class="py-12 flex flex-col items-center justify-center">
                <div class="w-16 h-16 rounded-2xl bg-gray-100 dark:bg-dark-800 text-gray-400 dark:text-dark-400 flex items-center justify-center mb-3">
                  <el-icon class="text-3xl"><UserFilledIcon /></el-icon>
                </div>
                <div class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.users.noUsersYet') }}
                </div>
                <div class="mt-1 text-xs text-gray-500 max-w-sm text-center">
                  {{ t('admin.users.createFirstUser') }}
                </div>
                <div class="mt-4 flex items-center gap-2">
                  <el-button v-if="hasActiveFilters" plain size="small" class="!rounded-xl" @click="resetAllFilters">
                    {{ t('common.reset') || '重置筛选' }}
                  </el-button>
                  <el-button type="primary" size="small" class="!rounded-xl" :icon="PlusIcon" @click="showCreateModal = true">
                    {{ t('admin.users.createUser') }}
                  </el-button>
                </div>
              </div>
            </template>
          </el-table>
        </div>

        <!-- Pagination Bar -->
        <div v-if="pagination.total > 0" class="ep-pagination-bar bg-white dark:bg-dark-900 border border-gray-200/80 dark:border-dark-700/80 rounded-2xl p-3 shadow-sm flex items-center justify-between flex-shrink-0">
          <div class="text-xs text-gray-500 dark:text-dark-400 hidden sm:block">
            {{ t('common.total') || '共' }} <strong class="text-gray-900 dark:text-gray-100 font-semibold">{{ pagination.total }}</strong> {{ t('admin.users.drawer.times') || '条记录' }}
          </div>
          <el-pagination
            background
            :total="pagination.total"
            :current-page="pagination.page"
            :page-size="pagination.page_size"
            :page-sizes="pageSizeOptions"
            :layout="isMobile ? 'prev, pager, next' : 'total, sizes, prev, pager, next, jumper'"
            class="!m-0"
            @current-change="handlePageChange"
            @size-change="handlePageSizeChange"
          />
        </div>
      </div>
    </el-config-provider>

    <UserCreateModal :show="showCreateModal" @close="showCreateModal = false" @success="loadUsers" />
    <UserEditModal :show="showEditModal" :user="editingUser" @close="closeEditModal" @success="loadUsers" />
    <BulkEditUserModal
      :show="showBulkEditModal"
      :selected-ids="selectedIds"
      @close="showBulkEditModal = false"
      @success="handleBulkLimitsSuccess"
    />
    <UserPlatformQuotaModal
      :show="showPlatformQuotaModal"
      :user="platformQuotaUser"
      @close="closePlatformQuotaModal"
      @success="loadUsers"
    />
    <UserApiKeysModal :show="showApiKeysModal" :user="viewingUser" @close="closeApiKeysModal" />
    <UserAllowedGroupsModal :show="showAllowedGroupsModal" :user="allowedGroupsUser" @close="closeAllowedGroupsModal" @success="loadUsers" />
    <UserBalanceModal :show="showBalanceModal" :user="balanceUser" :operation="balanceOperation" @close="closeBalanceModal" @success="loadUsers" />
    <UserBalanceHistoryModal :show="showBalanceHistoryModal" :user="balanceHistoryUser" @close="closeBalanceHistoryModal" @deposit="handleDepositFromHistory" @withdraw="handleWithdrawFromHistory" />
    <GroupReplaceModal :show="showGroupReplaceModal" :user="groupReplaceUser" :old-group="groupReplaceOldGroup" :all-groups="allGroups" @close="closeGroupReplaceModal" @success="loadUsers" />
    <UserDrawer :show="showUserDrawer" :user="drawerUser" @close="closeUserDrawer" />
    <UserAttributesConfigModal :show="showAttributesModal" @close="handleAttributesModalClose" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import { ElConfigProvider } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import enUs from 'element-plus/es/locale/lang/en'
import {
  Search as SearchIcon,
  Refresh as RefreshIcon,
  Filter as FilterIcon,
  Setting as SettingIcon,
  Grid as GridIcon,
  Check as CheckIcon,
  Plus as PlusIcon,
  Delete as DeleteIcon,
  User as UserIcon,
  UserFilled as UserFilledIcon,
  Key as KeyIcon,
  Edit as EditIcon,
  Lock as LockIcon,
  CircleCheck as CircleCheckIcon,
  MoreFilled as MoreFilledIcon,
  Minus as MinusIcon,
  DataLine as DataLineIcon,
  Coin as CoinIcon,
  Sort as SortIcon,
  CaretTop as CaretTopIcon,
  Star as StarIcon,
  Connection as ConnectionIcon,
  Switch as SwitchIcon,
  CircleClose as CircleCloseIcon,
  Close as CloseIcon
} from '@element-plus/icons-vue'
import type { TableInstance } from 'element-plus'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize, setPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { getConfiguredTablePageSizeOptions } from '@/utils/tablePreferences'
import { formatDateTime } from '@/utils/format'

const { t, locale } = useI18n()
import { adminAPI } from '@/api/admin'
import type { AdminUser, AdminGroup, UserAttributeDefinition } from '@/types'
import type { BatchUserUsageStats } from '@/api/admin/dashboard'
import type { PlatformQuotaItem } from '@/api/admin/users'
import AppLayout from '@/components/layout/AppLayout.vue'
import GroupBadge from '@/components/element/common/GroupBadge.vue'
import { buildApiKeyGroupFilterOptions } from './apiKeyGroupFilterOptions'
import UserAttributesConfigModal from '@/components/element/user/UserAttributesConfigModal.vue'
import UserConcurrencyCell from '@/components/element/user/UserConcurrencyCell.vue'
import PlatformUsageBreakdown from '@/components/element/user/PlatformUsageBreakdown.vue'
import PlatformCostCell from '@/components/element/user/PlatformCostCell.vue'
import UserPlatformQuotaCell from '@/components/element/user/UserPlatformQuotaCell.vue'
import UserCreateModal from '@/components/element/admin/user/UserCreateModal.vue'
import UserEditModal from '@/components/element/admin/user/UserEditModal.vue'
import BulkEditUserModal from '@/components/element/admin/user/BulkEditUserModal.vue'
import UserPlatformQuotaModal from '@/components/element/admin/user/UserPlatformQuotaModal.vue'
import UserApiKeysModal from '@/components/element/admin/user/UserApiKeysModal.vue'
import UserAllowedGroupsModal from '@/components/element/admin/user/UserAllowedGroupsModal.vue'
import UserBalanceModal from '@/components/element/admin/user/UserBalanceModal.vue'
import UserBalanceHistoryModal from '@/components/element/admin/user/UserBalanceHistoryModal.vue'
import GroupReplaceModal from '@/components/element/admin/user/GroupReplaceModal.vue'
import UserDrawer from '@/components/element/admin/user/UserDrawer.vue'

const appStore = useAppStore()

// Element Plus 组件语言跟随应用语言
const epLocale = computed(() => (locale.value === 'zh' ? zhCn : enUs))

// el-select / el-input 的 update:model-value 回调参数类型
type SelectEmittedValue = string | number | boolean | undefined | null

// ==================== 列可见性 ====================
// Hidden columns (stored in Set - columns NOT in this set are visible)
const hiddenColumns = reactive<Set<string>>(new Set())

// 与原页相同的默认隐藏列
const DEFAULT_HIDDEN_COLUMNS = [
  'notes', 'groups', 'subscriptions', 'usage', 'concurrency',
  'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity',
  'balance_platform_quota'
]
const REMOVED_COLUMNS = new Set(['last_login_at'])
const FORCED_VISIBLE_COLUMNS = new Set<string>()

// localStorage keys（新页独立存储，避免与原用户管理页互相影响）
const HIDDEN_COLUMNS_KEY = 'user-ep-hidden-columns'
const COLUMN_SETTINGS_VERSION_KEY = 'user-ep-column-settings-version'
const COLUMN_SETTINGS_VERSION = 3
const VERSION_NEW_HIDDEN_COLUMNS: Record<number, string[]> = {
  2: ['usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity'],
  3: ['balance_platform_quota']
}

const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      parsed
        .filter(key => !REMOVED_COLUMNS.has(key) && !FORCED_VISIBLE_COLUMNS.has(key))
        .forEach(key => hiddenColumns.add(key))

      const storedVersion = Number(localStorage.getItem(COLUMN_SETTINGS_VERSION_KEY) ?? '1')
      if (storedVersion < COLUMN_SETTINGS_VERSION) {
        let mutated = false
        for (let v = storedVersion + 1; v <= COLUMN_SETTINGS_VERSION; v++) {
          for (const key of VERSION_NEW_HIDDEN_COLUMNS[v] ?? []) {
            if (REMOVED_COLUMNS.has(key) || FORCED_VISIBLE_COLUMNS.has(key)) continue
            if (!hiddenColumns.has(key)) {
              hiddenColumns.add(key)
              mutated = true
            }
          }
        }
        if (mutated) saveColumnsToStorage()
        else localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
      }
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
      localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
    }
  } catch (e) {
    console.error('Failed to load saved columns:', e)
    DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
  }
}

const saveColumnsToStorage = () => {
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
    localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

const isForcedVisibleColumn = (key: string) => FORCED_VISIBLE_COLUMNS.has(key)
const toggleColumn = (key: string) => {
  if (FORCED_VISIBLE_COLUMNS.has(key)) return
  const wasHidden = hiddenColumns.has(key)
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
  if (wasHidden && (key === 'usage' || key.startsWith('usage_') || key.startsWith('attr_') || key === 'balance_platform_quota')) {
    refreshCurrentPageSecondaryData()
  }
  if (key === 'subscriptions') {
    loadUsers()
  }
  if (wasHidden && key === 'groups') {
    loadAllGroups()
  }
}

const isColumnVisible = (key: string) => !hiddenColumns.has(key)

// ==================== 用户属性列 ====================
const attributeDefinitions = ref<UserAttributeDefinition[]>([])
const userAttributeValues = ref<Record<number, Record<number, string>>>({})
const enabledAttributeDefinitions = computed(() => attributeDefinitions.value.filter(def => def.enabled))

const getAttributeValue = (userId: number, attrId: number): string => {
  const userAttrs = userAttributeValues.value[userId]
  if (!userAttrs) return '-'
  const value = userAttrs[attrId]
  if (!value) return '-'

  const def = attributeDefinitions.value.find(d => d.id === attrId)
  if (!def) return value

  if (def.type === 'multi_select' && value) {
    try {
      const arr = JSON.parse(value)
      if (Array.isArray(arr)) {
        return arr.map(v => {
          const opt = def.options?.find(o => o.value === v)
          return opt?.label || v
        }).join(', ')
      }
    } catch {
      return value
    }
  }

  if (def.type === 'select' && value && def.options) {
    const opt = def.options.find(o => o.value === value)
    return opt?.label || value
  }

  return value
}

// 列设置菜单使用（email/actions 固定可见，不在列表中）
const attributeColumnLabels = computed(() =>
  enabledAttributeDefinitions.value.map(def => ({ key: `attr_${def.id}`, label: def.name }))
)

const toggleableColumns = computed(() => [
  { key: 'email', label: t('admin.users.columns.user') },
  { key: 'id', label: t('admin.users.columns.id') },
  { key: 'username', label: t('admin.users.columns.username') },
  { key: 'notes', label: t('admin.users.columns.notes') },
  ...attributeColumnLabels.value,
  { key: 'role', label: t('admin.users.columns.role') },
  { key: 'groups', label: t('admin.users.columns.groups') },
  { key: 'subscriptions', label: t('admin.users.columns.subscriptions') },
  { key: 'balance', label: t('admin.users.columns.balance') },
  { key: 'balance_platform_quota', label: t('admin.users.columns.balancePlatformQuota') },
  { key: 'usage', label: t('admin.users.columns.usage') },
  { key: 'usage_anthropic', label: t('admin.users.columns.usageAnthropic') },
  { key: 'usage_openai', label: t('admin.users.columns.usageOpenAI') },
  { key: 'usage_gemini', label: t('admin.users.columns.usageGemini') },
  { key: 'usage_antigravity', label: t('admin.users.columns.usageAntigravity') },
  { key: 'concurrency', label: t('admin.users.columns.concurrency') },
  { key: 'status', label: t('admin.users.columns.status') },
  { key: 'last_active_at', label: t('admin.users.columns.lastActive') },
  { key: 'last_used_at', label: t('admin.users.columns.lastUsed') },
  { key: 'created_at', label: t('admin.users.columns.created') }
])

const usageColumnLabel = (key: string) =>
  toggleableColumns.value.find(col => col.key === key)?.label ?? key

// ==================== 数据与排序 ====================
const users = ref<AdminUser[]>([])
const loading = ref(false)
const searchQuery = ref('')
const isMobile = ref(false)
const pageSizeOptions = getConfiguredTablePageSizeOptions()

const checkMobile = () => {
  isMobile.value = window.innerWidth < 1024
}

const USER_SORT_STORAGE_KEY = 'admin-users-table-sort-ep'
const loadInitialSortState = (): { sort_by: string; sort_order: 'asc' | 'desc' } => {
  const fallback = { sort_by: 'created_at', sort_order: 'desc' as 'asc' | 'desc' }
  const sortable = new Set(['email', 'id', 'username', 'role', 'balance', 'concurrency', 'status', 'last_used_at', 'last_active_at', 'created_at'])
  try {
    const raw = localStorage.getItem(USER_SORT_STORAGE_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!sortable.has(key)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'asc' ? 'asc' : 'desc'
    }
  } catch {
    return fallback
  }
}
const sortState = reactive(loadInitialSortState())

// el-table 两态循环排序（升/降），与原 DataTable 行为一致
const SORT_ORDERS = ['ascending', 'descending'] as const
const defaultTableSort = computed(() => ({
  prop: sortState.sort_by,
  order: (sortState.sort_order === 'asc' ? 'ascending' : 'descending') as 'ascending' | 'descending'
}))

// ==================== 分组数据 ====================
const allGroups = ref<AdminGroup[]>([])
const loadAllGroups = async () => {
  if (allGroups.value.length > 0) return
  try {
    allGroups.value = await adminAPI.groups.getAll()
  } catch (e) {
    console.error('Failed to load groups:', e)
  }
}

const allGroupsForApiKeyFilter = ref<AdminGroup[]>([])
const loadAllGroupsForApiKeyFilter = async () => {
  if (allGroupsForApiKeyFilter.value.length > 0) return
  try {
    allGroupsForApiKeyFilter.value = await adminAPI.groups.getAllIncludingInactive()
  } catch (e) {
    console.error('Failed to load groups for API key filter:', e)
  }
}

const getUserGroups = (user: AdminUser) => {
  const exclusive: AdminGroup[] = []
  const publicGroups: AdminGroup[] = []
  for (const g of allGroups.value) {
    if (g.status !== 'active' || g.subscription_type !== 'standard') continue
    if (g.is_exclusive) {
      if (user.allowed_groups?.includes(g.id)) {
        exclusive.push(g)
      }
    } else {
      publicGroups.push(g)
    }
  }
  return { exclusive, publicGroups }
}

const groupFilterOptions = computed(() => {
  const options: { value: string; label: string }[] = [
    { value: '', label: t('admin.users.allAuthorizedGroups') }
  ]
  for (const g of allGroups.value) {
    if (g.status !== 'active' || !g.is_exclusive || g.subscription_type !== 'standard') continue
    options.push({ value: g.name, label: g.name })
  }
  return options
})

const apiKeyGroupFilterOptions = computed(() =>
  buildApiKeyGroupFilterOptions(allGroupsForApiKeyFilter.value, {
    all: t('admin.users.allApiKeyGroups'),
    exclusive: t('admin.users.apiKeyGroupExclusive'),
    public: t('admin.users.apiKeyGroupPublic'),
    subscription: t('admin.users.apiKeyGroupSubscription'),
    disabled: t('admin.users.apiKeyGroupDisabled'),
  })
)

// ==================== 筛选器 ====================
const filters = reactive({
  role: '',
  status: '',
  group: '',
  apiKeyGroup: null as number | null
})
const activeAttributeFilters = reactive<Record<number, string>>({})
const visibleFilters = reactive<Set<string>>(new Set())

const FILTER_VALUES_KEY = 'user-ep-filter-values'
const VISIBLE_FILTERS_KEY = 'user-ep-visible-filters'

const filterableAttributes = computed(() =>
  attributeDefinitions.value.filter(def => def.enabled)
)

const builtInFilters = computed(() => [
  { key: 'role', name: t('admin.users.columns.role') },
  { key: 'status', name: t('admin.users.columns.status') },
  { key: 'group', name: t('admin.users.authorizedGroupFilter') },
  { key: 'apiKeyGroup', name: t('admin.users.apiKeyGroupFilter') }
])

const loadSavedFilters = () => {
  try {
    const savedVisible = localStorage.getItem(VISIBLE_FILTERS_KEY)
    if (savedVisible) {
      const parsed = JSON.parse(savedVisible) as string[]
      parsed.forEach(key => visibleFilters.add(key))
    }
    const savedValues = localStorage.getItem(FILTER_VALUES_KEY)
    if (savedValues) {
      const parsed = JSON.parse(savedValues)
      if (parsed.role) filters.role = parsed.role
      if (parsed.status) filters.status = parsed.status
      if (parsed.group) filters.group = parsed.group
      if (typeof parsed.apiKeyGroup === 'number') filters.apiKeyGroup = parsed.apiKeyGroup
      if (parsed.attributes) {
        Object.assign(activeAttributeFilters, parsed.attributes)
      }
    }
  } catch (e) {
    console.error('Failed to load saved filters:', e)
  }
}

const saveFiltersToStorage = () => {
  try {
    localStorage.setItem(VISIBLE_FILTERS_KEY, JSON.stringify([...visibleFilters]))
    const values = {
      role: filters.role,
      status: filters.status,
      group: filters.group,
      apiKeyGroup: filters.apiKeyGroup,
      attributes: activeAttributeFilters
    }
    localStorage.setItem(FILTER_VALUES_KEY, JSON.stringify(values))
  } catch (e) {
    console.error('Failed to save filters:', e)
  }
}

const getAttributeDefinition = (attrId: number): UserAttributeDefinition | undefined => {
  return attributeDefinitions.value.find(d => d.id === attrId)
}

const getAttributeDefinitionName = (attrId: number): string => {
  const def = attributeDefinitions.value.find(d => d.id === attrId)
  return def?.name || String(attrId)
}

const toggleBuiltInFilter = (key: string) => {
  if (visibleFilters.has(key)) {
    visibleFilters.delete(key)
    if (key === 'role') filters.role = ''
    if (key === 'status') filters.status = ''
    if (key === 'group') filters.group = ''
    if (key === 'apiKeyGroup') filters.apiKeyGroup = null
  } else {
    visibleFilters.add(key)
    if (key === 'group') loadAllGroups()
    if (key === 'apiKeyGroup') loadAllGroupsForApiKeyFilter()
  }
  saveFiltersToStorage()
  pagination.page = 1
  loadUsers()
}

const toggleAttributeFilter = (attr: UserAttributeDefinition) => {
  const key = `attr_${attr.id}`
  if (visibleFilters.has(key)) {
    visibleFilters.delete(key)
    delete activeAttributeFilters[attr.id]
  } else {
    visibleFilters.add(key)
    activeAttributeFilters[attr.id] = ''
  }
  saveFiltersToStorage()
  pagination.page = 1
  loadUsers()
}

const updateAttributeFilter = (attrId: number, value: string) => {
  activeAttributeFilters[attrId] = value
}

const applyFilter = () => {
  saveFiltersToStorage()
  loadUsers()
}

const hasActiveFilters = computed(() => {
  if (searchQuery.value.trim()) return true
  if (filters.role) return true
  if (filters.status) return true
  if (filters.group) return true
  if (typeof filters.apiKeyGroup === 'number') return true
  for (const val of Object.values(activeAttributeFilters)) {
    if (val) return true
  }
  return false
})

const resetAllFilters = () => {
  searchQuery.value = ''
  filters.role = ''
  filters.status = ''
  filters.group = ''
  filters.apiKeyGroup = null
  for (const key of Object.keys(activeAttributeFilters)) {
    activeAttributeFilters[Number(key)] = ''
  }
  saveFiltersToStorage()
  pagination.page = 1
  loadUsers()
}

// ==================== 用量数据与本地排序 ====================
const usageStats = ref<Record<string, BatchUserUsageStats>>({})
const platformQuotaStats = ref<Record<number, PlatformQuotaItem[]>>({})

const getPlatformUsage = (userId: number, platform: string) =>
  usageStats.value[userId]?.by_platform?.find((p) => p.platform === platform)

type UsageMetric = 'today' | 'total'
type UsageSortState = { key: string; metric: UsageMetric; order: 'asc' | 'desc' } | null
const USAGE_SORT_STORAGE_KEY = 'admin-users-usage-sort-ep'

const USAGE_COLUMN_KEYS: readonly string[] = ['usage', 'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity']
const USAGE_COLUMN_PLATFORMS: Record<string, string | null> = {
  usage: null,
  usage_anthropic: 'anthropic',
  usage_openai: 'openai',
  usage_gemini: 'gemini',
  usage_antigravity: 'antigravity'
}

const visibleUsageColumnKeys = computed(() =>
  USAGE_COLUMN_KEYS.filter(key => isColumnVisible(key))
)

const loadInitialUsageSort = (): UsageSortState => {
  try {
    const raw = localStorage.getItem(USAGE_SORT_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<{ key: string; metric: string; order: string }>
    if (!parsed.key || !USAGE_COLUMN_KEYS.includes(parsed.key)) return null
    const metric: UsageMetric = parsed.metric === 'total' ? 'total' : 'today'
    const order: 'asc' | 'desc' = parsed.order === 'asc' ? 'asc' : 'desc'
    return { key: parsed.key, metric, order }
  } catch {
    return null
  }
}
const usageSort = ref<UsageSortState>(loadInitialUsageSort())
const persistUsageSort = () => {
  try {
    if (usageSort.value) {
      localStorage.setItem(USAGE_SORT_STORAGE_KEY, JSON.stringify(usageSort.value))
    } else {
      localStorage.removeItem(USAGE_SORT_STORAGE_KEY)
    }
  } catch (e) {
    console.error('Failed to persist usage sort:', e)
  }
}

const clearUsageSort = () => {
  if (!usageSort.value) return
  usageSort.value = null
  persistUsageSort()
}

const isUsageSortActive = (key: string, metric: UsageMetric) =>
  !!usageSort.value && usageSort.value.key === key && usageSort.value.metric === metric
const getUsageSortOrder = (key: string, metric: UsageMetric): 'asc' | 'desc' | null =>
  isUsageSortActive(key, metric) ? usageSort.value!.order : null

// 三态循环：desc → asc → off
const toggleUsageSort = (key: string, metric: UsageMetric) => {
  const cur = usageSort.value
  if (cur && cur.key === key && cur.metric === metric) {
    usageSort.value = cur.order === 'desc' ? { key, metric, order: 'asc' } : null
  } else {
    usageSort.value = { key, metric, order: 'desc' }
  }
  persistUsageSort()
}

const getUsageValue = (userId: number, key: string, metric: UsageMetric): number => {
  const stats = usageStats.value[userId]
  if (!stats) return 0
  const platform = USAGE_COLUMN_PLATFORMS[key]
  if (platform === null) {
    return metric === 'today' ? stats.today_actual_cost ?? 0 : stats.total_actual_cost ?? 0
  }
  const p = stats.by_platform?.find((x) => x.platform === platform)
  if (!p) return 0
  return metric === 'today' ? p.today_actual_cost ?? 0 : p.total_actual_cost ?? 0
}

// 在 server-side 排序结果之上叠加用量列的本地排序（仅当前页），等值按原 index 保序
const sortedUsers = computed(() => {
  const s = usageSort.value
  if (!s) return users.value
  return [...users.value]
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const av = getUsageValue(a.row.id, s.key, s.metric)
      const bv = getUsageValue(b.row.id, s.key, s.metric)
      if (av !== bv) return s.order === 'asc' ? av - bv : bv - av
      return a.index - b.index
    })
    .map((x) => x.row)
})

// ==================== 选择状态 ====================
const tableRef = ref<TableInstance>()

const {
  selectedIds,
  selectedCount,
  setSelectedIds,
  clear: clearSelection,
  removeMany: removeSelectedIds
} = useTableSelection<AdminUser>({
  rows: sortedUsers,
  getId: (user) => user.id
})

const handleSelectionChange = (rows: AdminUser[]) => {
  setSelectedIds(rows.map(row => row.id))
}

const clearTableSelection = () => {
  clearSelection()
  tableRef.value?.clearSelection()
}

const activeUsersCount = computed(() => users.value.filter(u => u.status === 'active').length)
const disabledUsersCount = computed(() => users.value.filter(u => u.status === 'disabled').length)
const adminUsersCount = computed(() => users.value.filter(u => u.role === 'admin').length)

// ==================== 分页 ====================
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

// ==================== 弹窗状态 ====================
const showCreateModal = ref(false)
const showEditModal = ref(false)
const showBulkEditModal = ref(false)
const showApiKeysModal = ref(false)
const showAttributesModal = ref(false)
const showPlatformQuotaModal = ref(false)
const editingUser = ref<AdminUser | null>(null)
const viewingUser = ref<AdminUser | null>(null)
const platformQuotaUser = ref<AdminUser | null>(null)
const bulkDeleting = ref(false)

const handlePlatformQuota = (user: AdminUser) => {
  platformQuotaUser.value = user
  showPlatformQuotaModal.value = true
}

const closePlatformQuotaModal = () => {
  showPlatformQuotaModal.value = false
  platformQuotaUser.value = null
}

// ==================== 数据加载 ====================
let abortController: AbortController | null = null
let secondaryDataSeq = 0

const hasVisibleUsageColumn = computed(() =>
  visibleUsageColumnKeys.value.length > 0
)
const hasVisibleAttributeColumns = computed(() =>
  attributeDefinitions.value.some((def) => def.enabled && !hiddenColumns.has(`attr_${def.id}`))
)
const hasVisiblePlatformQuotaColumn = computed(() => !hiddenColumns.has('balance_platform_quota'))

const loadUsersSecondaryData = async (
  userIds: number[],
  signal?: AbortSignal,
  expectedSeq?: number
) => {
  if (userIds.length === 0) return

  const tasks: Promise<void>[] = []

  if (hasVisibleUsageColumn.value) {
    tasks.push(
      (async () => {
        try {
          const usageResponse = await adminAPI.dashboard.getBatchUsersUsage(userIds)
          if (signal?.aborted) return
          if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
          usageStats.value = usageResponse.stats
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load usage stats:', e)
        }
      })()
    )
  }

  if (attributeDefinitions.value.length > 0 && hasVisibleAttributeColumns.value) {
    tasks.push(
      (async () => {
        try {
          const attrResponse = await adminAPI.userAttributes.getBatchUserAttributes(userIds)
          if (signal?.aborted) return
          if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
          userAttributeValues.value = attrResponse.attributes
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load user attribute values:', e)
        }
      })()
    )
  }

  if (hasVisiblePlatformQuotaColumn.value) {
    tasks.push(
      (async () => {
        try {
          // 无批量端点：对当前页用户逐个拉取，分块并发（每批 6），批间检查中止条件
          const CHUNK = 6
          for (let i = 0; i < userIds.length; i += CHUNK) {
            if (signal?.aborted) return
            if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
            const chunk = userIds.slice(i, i + CHUNK)
            const results = await Promise.allSettled(
              chunk.map((id) => adminAPI.users.getPlatformQuotas(id))
            )
            if (signal?.aborted) return
            if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
            const merged = { ...platformQuotaStats.value }
            results.forEach((r, idx) => {
              if (r.status === 'fulfilled') {
                merged[chunk[idx]] = r.value.platform_quotas || []
              }
            })
            platformQuotaStats.value = merged
          }
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load platform quotas:', e)
        }
      })()
    )
  }

  if (tasks.length > 0) {
    await Promise.allSettled(tasks)
  }
}

const refreshCurrentPageSecondaryData = () => {
  const userIds = users.value.map((u) => u.id)
  if (userIds.length === 0) return
  const seq = ++secondaryDataSeq
  void loadUsersSecondaryData(userIds, undefined, seq)
}

const loadAttributeDefinitions = async () => {
  try {
    attributeDefinitions.value = await adminAPI.userAttributes.listEnabledDefinitions()
  } catch (e) {
    console.error('Failed to load attribute definitions:', e)
  }
}

const handleAttributesModalClose = async () => {
  showAttributesModal.value = false
  await loadAttributeDefinitions()
  loadUsers()
}

const loadUsers = async () => {
  abortController?.abort()
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  const { signal } = currentAbortController
  loading.value = true
  try {
    const attrFilters: Record<number, string> = {}
    for (const [attrId, value] of Object.entries(activeAttributeFilters)) {
      if (value) {
        attrFilters[Number(attrId)] = value
      }
    }

    const response = await adminAPI.users.list(
      pagination.page,
      pagination.page_size,
      {
        role: filters.role as any,
        status: filters.status as any,
        search: searchQuery.value || undefined,
        group_name: filters.group || undefined,
        api_key_group_id: filters.apiKeyGroup ?? undefined,
        attributes: Object.keys(attrFilters).length > 0 ? attrFilters : undefined,
        include_subscriptions: true,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      { signal }
    )
    if (signal.aborted) {
      return
    }
    users.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
    usageStats.value = {}
    userAttributeValues.value = {}
    platformQuotaStats.value = {}

    // Defer heavy secondary data so table can render first.
    if (response.items.length > 0) {
      const userIds = response.items.map((u) => u.id)
      const seq = ++secondaryDataSeq
      window.setTimeout(() => {
        if (signal.aborted || seq !== secondaryDataSeq) return
        void loadUsersSecondaryData(userIds, signal, seq)
      }, 50)
    }
  } catch (error: any) {
    const errorInfo = error as { name?: string; code?: string }
    if (errorInfo?.name === 'AbortError' || errorInfo?.name === 'CanceledError' || errorInfo?.code === 'ERR_CANCELED') {
      return
    }
    const message = error.response?.data?.detail || error.message || t('admin.users.failedToLoad')
    appStore.showError(message)
    console.error('Error loading users:', error)
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
    }
  }
}

// ==================== 搜索 / 分页 / 排序 ====================
let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadUsers()
  }, 300)
}

const handlePageChange = (page: number) => {
  const validPage = Math.max(1, Math.min(page, pagination.pages || 1))
  pagination.page = validPage
  loadUsers()
}

const handlePageSizeChange = (pageSize: number) => {
  setPersistedPageSize(pageSize)
  pagination.page_size = pageSize
  pagination.page = 1
  loadUsers()
}

const handleSortChange = ({ prop, order }: { prop: string; order: 'ascending' | 'descending' | null }) => {
  if (!prop || !order) return
  clearUsageSort()
  sortState.sort_by = prop
  sortState.sort_order = order === 'ascending' ? 'asc' : 'desc'
  try {
    localStorage.setItem(USER_SORT_STORAGE_KEY, JSON.stringify({ key: prop, order: sortState.sort_order }))
  } catch (e) {
    console.error('Failed to persist sort:', e)
  }
  pagination.page = 1
  loadUsers()
}

// ==================== 行操作 ====================
const handleRowCommand = (command: string, user: AdminUser) => {
  switch (command) {
    case 'apiKeys':
      handleViewApiKeys(user)
      break
    case 'groups':
      handleAllowedGroups(user)
      break
    case 'deposit':
      handleDeposit(user)
      break
    case 'withdraw':
      handleWithdraw(user)
      break
    case 'platformQuota':
      handlePlatformQuota(user)
      break
    case 'balanceHistory':
      handleBalanceHistory(user)
      break
    case 'delete':
      handleDelete(user)
      break
  }
}

const handleEdit = (user: AdminUser) => {
  editingUser.value = user
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingUser.value = null
}

const handleToggleStatus = async (user: AdminUser) => {
  const newStatus = user.status === 'active' ? 'disabled' : 'active'
  try {
    const updated = await adminAPI.users.toggleStatus(user.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('admin.users.userEnabled') : t('admin.users.userDisabled')
    )
    if (loading.value) {
      loadUsers()
      return
    }
    const row = users.value.find((u) => u.id === user.id)
    if (row) {
      row.status = updated.status
      row.updated_at = updated.updated_at
    }
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToToggle'))
    console.error('Error toggling user status:', error)
  }
}

const handleViewApiKeys = (user: AdminUser) => {
  viewingUser.value = user
  showApiKeysModal.value = true
}

const closeApiKeysModal = () => {
  showApiKeysModal.value = false
  viewingUser.value = null
}

const handleAllowedGroups = (user: AdminUser) => {
  allowedGroupsUser.value = user
  showAllowedGroupsModal.value = true
}

const closeAllowedGroupsModal = () => {
  showAllowedGroupsModal.value = false
  allowedGroupsUser.value = null
}

// ==================== 用户详情抽屉 ====================
const showUserDrawer = ref(false)
const drawerUser = ref<AdminUser | null>(null)

const openUserDrawer = (user: AdminUser) => {
  drawerUser.value = user
  showUserDrawer.value = true
}

const closeUserDrawer = () => {
  showUserDrawer.value = false
  drawerUser.value = null
}

// ==================== 分组替换 ====================
const showGroupReplaceModal = ref(false)
const groupReplaceUser = ref<AdminUser | null>(null)
const groupReplaceOldGroup = ref<{ id: number; name: string } | null>(null)

const openGroupReplace = (user: AdminUser, group: { id: number; name: string }) => {
  groupReplaceUser.value = user
  groupReplaceOldGroup.value = group
  showGroupReplaceModal.value = true
}

const closeGroupReplaceModal = () => {
  showGroupReplaceModal.value = false
  groupReplaceUser.value = null
  groupReplaceOldGroup.value = null
}

// ==================== 删除 ====================
const handleDelete = async (user: AdminUser) => {
  try {
    await ElMessageBox.confirm(
      t('admin.users.deleteConfirm', { email: user.email }),
      t('admin.users.deleteUser'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel'),
        confirmButtonClass: 'el-button--danger',
        autofocus: false
      }
    )
  } catch {
    return
  }
  try {
    await adminAPI.users.delete(user.id)
    appStore.showSuccess(t('common.success'))
    loadUsers()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToDelete'))
    console.error('Error deleting user:', error)
  }
}

const handleBulkDeleteClick = async () => {
  const ids = [...selectedIds.value]
  try {
    await ElMessageBox.confirm(
      t('admin.users.bulkDelete.confirm', { count: ids.length }),
      t('admin.users.bulkDelete.title'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel'),
        confirmButtonClass: 'el-button--danger',
        autofocus: false
      }
    )
  } catch {
    return
  }
  bulkDeleting.value = true
  const deletedIds: number[] = []
  for (const id of ids) {
    try {
      await adminAPI.users.delete(id)
      deletedIds.push(id)
    } catch (error) {
      console.error('Error deleting user:', error)
    }
  }
  removeSelectedIds(deletedIds)
  if (deletedIds.length > 0) {
    appStore.showSuccess(t('admin.users.bulkDelete.success', { count: deletedIds.length }))
    pagination.page = 1
  }
  const failed = ids.length - deletedIds.length
  if (failed > 0) appStore.showError(t('admin.users.bulkDelete.failed', { count: failed }))
  await loadUsers()
  bulkDeleting.value = false
}

// ==================== 余额 ====================
const showBalanceModal = ref(false)
const balanceUser = ref<AdminUser | null>(null)
const balanceOperation = ref<'add' | 'subtract'>('add')

const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)

const showAllowedGroupsModal = ref(false)
const allowedGroupsUser = ref<AdminUser | null>(null)

const getDaysRemaining = (expiresAt: string): number => {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diffMs = expires.getTime() - now.getTime()
  return Math.ceil(diffMs / (1000 * 60 * 60 * 24))
}

const handleDeposit = (user: AdminUser) => {
  balanceUser.value = user
  balanceOperation.value = 'add'
  showBalanceModal.value = true
}

const handleWithdraw = (user: AdminUser) => {
  balanceUser.value = user
  balanceOperation.value = 'subtract'
  showBalanceModal.value = true
}

const closeBalanceModal = () => {
  showBalanceModal.value = false
  balanceUser.value = null
}

const handleBalanceHistory = (user: AdminUser) => {
  balanceHistoryUser.value = user
  showBalanceHistoryModal.value = true
}

const closeBalanceHistoryModal = () => {
  showBalanceHistoryModal.value = false
  balanceHistoryUser.value = null
}

const handleDepositFromHistory = () => {
  if (balanceHistoryUser.value) {
    handleDeposit(balanceHistoryUser.value)
  }
}

const handleWithdrawFromHistory = () => {
  if (balanceHistoryUser.value) {
    handleWithdraw(balanceHistoryUser.value)
  }
}

const handleBulkLimitsSuccess = async () => {
  clearSelection()
  tableRef.value?.clearSelection()
  await loadUsers()
}

// ==================== 生命周期 ====================
onMounted(async () => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  await loadAttributeDefinitions()
  loadSavedFilters()
  loadSavedColumns()
  loadUsers()
  if (isColumnVisible('groups') || visibleFilters.has('group')) {
    loadAllGroups()
  }
  if (visibleFilters.has('apiKeyGroup')) {
    loadAllGroupsForApiKeyFilter()
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>

<style scoped>
.ep-users-page {
  @apply flex flex-col gap-3.5;
  height: calc(100vh - 64px - 4rem); /* 减去 header + lg:p-8 的上下padding */
}

.ep-filter-card :deep(.el-select) {
  width: 100%;
}

.ep-table-card {
  @apply flex-1 min-h-0 flex flex-col;
}

.ep-refined-table {
  --el-table-header-bg-color: transparent;
}

.ep-refined-table :deep(.el-table__header-wrapper th) {
  background-color: rgb(249 250 251 / 0.85);
  font-weight: 600;
  color: rgb(75 85 99);
  font-size: 0.75rem;
  letter-spacing: 0.025em;
}

:global(.dark) .ep-refined-table :deep(.el-table__header-wrapper th) {
  background-color: rgb(17 24 39 / 0.75);
  color: rgb(156 163 175);
}

.ep-refined-table :deep(.el-table__row:hover > td) {
  background-color: rgb(248 250 252 / 0.8) !important;
}

:global(.dark) .ep-refined-table :deep(.el-table__row:hover > td) {
  background-color: rgb(30 41 59 / 0.35) !important;
}

/* 用户列邮箱链接按钮：点击打开详情抽屉（主要样式由 el-button link type=primary 提供） */
.ep-username-link {
  @apply text-left text-sm font-semibold;
  height: auto;
  padding: 0;
}

/* 虚线链接风格按钮（余额、平台配额单元格） */
.ep-link-btn {
  @apply font-medium text-gray-900 underline decoration-dashed decoration-gray-300 underline-offset-4 transition-colors hover:text-primary-600 dark:text-white dark:decoration-dark-500 dark:hover:text-primary-400;
}

.ep-usage-sort-trigger {
  @apply flex cursor-pointer items-center gap-1 rounded px-1 py-0.5 text-gray-400 transition-colors hover:bg-gray-200 dark:text-dark-500 dark:hover:bg-dark-700;
}

.ep-usage-sort-trigger.is-active {
  @apply text-primary-600 dark:text-primary-400;
}

.ep-rotate-180 {
  transform: rotate(180deg);
}

/* 下拉设置菜单里的勾选行 */
.ep-check-row {
  @apply flex w-full items-center justify-between gap-4;
}

.ep-check-row.is-disabled {
  @apply cursor-not-allowed text-gray-400 dark:text-gray-500;
}

.ep-dropdown-divider {
  @apply my-1 border-t border-gray-100 dark:border-dark-700;
}

@media (max-width: 1023px) {
  .ep-users-page {
    height: auto;
  }

  .ep-table-card {
    @apply min-h-0 overflow-auto;
  }
}
</style>

<style>
/* 全局（非 scoped）：el-dropdown 渲染在 body 下，需要全局样式控制勾选行外观 */
.ep-check-dropdown .el-dropdown-menu__item {
  padding: 6px 16px;
}

.ep-column-dropdown .el-dropdown-menu {
  max-height: 320px;
  overflow-y: auto;
}

.ep-check-dropdown .ep-check-row-mark {
  @apply text-primary-500;
}

.ep-check-dropdown .el-dropdown-menu__item.is-disabled .ep-check-row-mark {
  @apply text-gray-400 dark:text-gray-500;
}

/* 危险操作项（删除） */
.el-dropdown-menu__item.ep-danger-item {
  @apply text-red-600 dark:text-red-400;
}

.el-dropdown-menu__item.ep-danger-item:not(.is-disabled):hover,
.el-dropdown-menu__item.ep-danger-item:not(.is-disabled):focus {
  @apply bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400;
}

/* 用量排序菜单激活项 */
.el-dropdown-menu__item.ep-active-item {
  @apply font-medium text-primary-600 dark:text-primary-400;
}

/* 刷新按钮加载旋转 */
.ep-refresh-loading > .el-icon {
  animation: ep-rotate 1s linear infinite;
}

@keyframes ep-rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
