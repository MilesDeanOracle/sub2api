<template>
  <div class="flex min-w-0 items-center gap-1 flex-wrap">
    <!-- Membership pills（demo 同款：点击直接切换到该资源池） -->
    <el-tag
      v-for="id in selectedIds"
      :key="id"
      :closable="!disabled"
      :disable-transitions="true"
      :data-testid="`membership-pill-${id}`"
      :title="groupName(id)"
      class="membership-pill !inline-flex !max-w-[320px] !cursor-pointer"
      @click.stop="emit('group-click', id)"
      @close.stop="removeGroup(id)"
    >
      <span class="inline-flex min-w-0 max-w-full items-center gap-1">
        <span class="w-1.5 h-1.5 rounded-full bg-primary-500 shrink-0"></span>
        <span class="truncate">{{ displayGroupName(id) }}</span>
      </span>
    </el-tag>

    <!-- Ungrouped prompt tag -->
    <span
      v-if="selectedIds.length === 0"
      class="inline-flex items-center px-2 py-0.5 rounded-full border border-dashed border-slate-300 dark:border-dark-600 text-[11px] font-medium text-slate-400 dark:text-dark-400 whitespace-nowrap"
    >
      {{ t('resourcePool.accounts.membershipEmpty') }}
    </span>

    <!-- Pending-change pulse -->
    <span
      v-if="pending"
      class="w-1.5 h-1.5 rounded-full bg-amber-500 animate-pulse shrink-0"
      :title="t('resourcePool.accounts.membershipPendingHint')"
    ></span>

    <!-- Quick add to pool action -->
    <button
      :ref="el => (triggerRef = el as HTMLButtonElement | null)"
      type="button"
      data-testid="membership-trigger"
      class="shrink-0 p-0.5 rounded text-slate-400 hover:text-primary-500 hover:bg-slate-200/60 dark:hover:bg-dark-800 transition-colors"
      :title="t('resourcePool.accounts.membershipMenu')"
      :disabled="disabled"
      @click.stop="toggleMenu"
    >
      <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M12 4v16m8-8H4" />
      </svg>
    </button>

    <!-- Membership quick menu（Teleport 到 body，避免被表格 overflow 裁剪） -->
    <Teleport to="body">
      <div v-if="menuOpen" class="fixed inset-0 z-[1999]" @click="closeMenu"></div>
      <div
        v-if="menuOpen"
        data-testid="membership-menu"
        class="fixed z-[2000] w-96 max-h-[calc(100vh-16px)] max-w-[calc(100vw-16px)] bg-white dark:bg-dark-900 border border-slate-200 dark:border-dark-700 rounded-xl shadow-lg overflow-hidden flex flex-col"
        :style="menuStyle"
      >
        <div class="flex shrink-0 items-center justify-between px-3 py-2 border-b border-slate-100 dark:border-dark-800">
          <span class="text-[10px] font-bold uppercase tracking-wider text-slate-400">
            {{ t('resourcePool.accounts.membershipMenu') }}
          </span>
          <span class="text-[10px] font-mono text-slate-400">{{ selectedIds.length }}</span>
        </div>

        <div class="shrink-0 p-2 border-b border-slate-100 dark:border-dark-800">
          <el-input
            v-model="menuSearch"
            clearable
            size="small"
            :prefix-icon="Search"
            :placeholder="t('resourcePool.accounts.membershipSearch')"
            data-testid="membership-search"
          />
        </div>

        <div class="min-h-[40px] max-h-[320px] overflow-y-auto p-1">
          <button
            v-for="group in filteredCompatibleGroups"
            :key="group.id"
            type="button"
            :data-testid="`membership-option-${group.id}`"
            :class="isSelected(group.id)
              ? 'bg-primary-500/10 text-primary-700 dark:text-primary-300 font-medium'
              : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-dark-800'"
            class="w-full h-8 flex items-center justify-between gap-2 px-2.5 rounded-lg text-xs text-left transition-colors"
            @click="toggleOption(group.id)"
          >
            <div class="flex items-center gap-2 min-w-0 flex-1">
              <PlatformIcon :platform="group.platform" size="xs" class="shrink-0" />
              <span class="truncate" :title="group.name">{{ group.name }}</span>
            </div>
            <span class="w-12 shrink-0 text-right font-mono text-[10px] text-orange-600 dark:text-orange-400">
              {{ formatGroupMultiplier(group.rate_multiplier) }}
            </span>
            <span class="w-12 shrink-0 text-right font-mono text-[10px] text-blue-600 dark:text-blue-400" title="当前账号倍率">
              {{ Number(accountMultiplier ?? 1).toFixed(2) }}x
            </span>
            <svg v-if="isSelected(group.id)" class="w-3.5 h-3.5 text-primary-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7" />
            </svg>
            <span v-else class="w-3.5 shrink-0" />
          </button>

          <div v-if="filteredCompatibleGroups.length === 0" class="py-6 text-center text-xs text-slate-400">
            <template v-if="compatibleGroups.length === 0">
              {{ t('resourcePool.accounts.membershipNoneCompatible') }}
            </template>
            <template v-else>
              {{ t('resourcePool.accounts.membershipSearchEmpty') }}
            </template>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Search } from '@element-plus/icons-vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { AdminGroup } from '@/types'

const props = defineProps<{
  selectedIds: number[]
  allGroups: AdminGroup[]
  compatibleGroups: AdminGroup[]
  accountMultiplier?: number
  pending?: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  'group-click': [groupId: number]
  change: [groupIds: number[]]
  remove: [groupId: number]
  'menu-close': []
}>()

const { t } = useI18n()

const menuOpen = ref(false)
const menuChanged = ref(false)
const menuSearch = ref('')
const triggerRef = ref<HTMLButtonElement | null>(null)
const menuStyle = ref<Record<string, string>>({})
const filteredCompatibleGroups = computed(() => {
  const query = menuSearch.value.trim().toLocaleLowerCase()
  if (!query) return props.compatibleGroups
  return props.compatibleGroups.filter(group =>
    `${group.name} ${group.platform} ${group.description ?? ''}`.toLocaleLowerCase().includes(query)
  )
})

function groupName(id: number) {
  const group = props.allGroups.find(item => item.id === id)
  return group ? group.name : `资源池 #${id}`
}

function displayGroupName(id: number) {
  const name = groupName(id)
  const characters = Array.from(name)
  return characters.length > 20 ? `${characters.slice(0, 20).join('')}…` : name
}

function formatGroupMultiplier(value?: number) {
  const multiplier = Number(value ?? 1)
  return `${multiplier < 0.01 ? multiplier.toFixed(3) : multiplier.toFixed(2)}x`
}

function isSelected(id: number) {
  return props.selectedIds.includes(id)
}

function toggleMenu() {
  if (menuOpen.value) {
    closeMenu()
    return
  }
  const rect = triggerRef.value?.getBoundingClientRect()
  if (rect) {
    const left = Math.min(rect.left, window.innerWidth - 384 - 8)
    const visibleRows = Math.min(10, Math.max(1, filteredCompatibleGroups.value.length))
    const estimatedHeight = 96 + visibleRows * 32
    const top = Math.min(rect.bottom + 6, Math.max(8, window.innerHeight - estimatedHeight - 8))
    menuStyle.value = { left: `${Math.max(8, left)}px`, top: `${top}px` }
  }
  menuChanged.value = false
  menuSearch.value = ''
  menuOpen.value = true
}

function closeMenu() {
  if (!menuOpen.value) return
  menuOpen.value = false
  if (menuChanged.value || props.pending) emit('menu-close')
  menuChanged.value = false
}

function toggleOption(groupId: number) {
  menuChanged.value = true
  const next = isSelected(groupId)
    ? props.selectedIds.filter(id => id !== groupId)
    : [...props.selectedIds, groupId]
  emit('change', next)
}

function removeGroup(groupId: number) {
  if (props.disabled) return
  emit('remove', groupId)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') closeMenu()
}

watch(menuOpen, open => {
  if (open) window.addEventListener('keydown', onKeydown)
  else window.removeEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
:deep(.membership-pill .el-tag__content) {
  min-width: 0;
  overflow: hidden;
}

:deep(.membership-pill .el-tag__close) {
  flex-shrink: 0;
}
</style>
