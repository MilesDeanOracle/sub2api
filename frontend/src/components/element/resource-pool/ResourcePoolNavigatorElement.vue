<template>
  <!-- 样式与交互 1:1 复刻自 frontend/public/resource_pool_demo.html 的左侧导航器（LEFT NAVIGATOR）；
       实现层为 shadcn-vue：ui 原语（Input/Badge/Separator/Button）+ cn()/cva() 组合 -->
  <aside class="w-full h-full border-r border-slate-200 dark:border-dark-800 bg-white dark:bg-dark-900/60 flex flex-col shrink-0 overflow-hidden select-none">

    <!-- Search and Filter in Sidebar -->
    <div class="p-3 space-y-2">
      <div class="relative">
        <svg class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 z-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <Input
          v-model="search"
          placeholder="搜索资源池或平台..."
          class="h-auto w-full pl-8 pr-7 py-1.5 rounded-lg bg-slate-100 dark:bg-dark-850 text-xs text-slate-900 dark:text-slate-100 placeholder:text-slate-400 focus-visible:ring-1 focus-visible:ring-primary-500 border border-transparent dark:border-dark-700/60 shadow-none transition-all"
        />
        <button
          v-if="search"
          type="button"
          class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
          @click="search = ''"
        >
          <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
        <span v-else class="text-[10px] font-mono text-slate-400 absolute right-2.5 top-1/2 -translate-y-1/2 px-1 rounded bg-slate-200/60 dark:bg-dark-800 border border-slate-300/60 dark:border-dark-700">/</span>
      </div>

      <!-- Quick Scope Filter Pills -->
      <div class="flex items-center gap-1 text-[11px]">
        <button
          v-for="pill in filterPills"
          :key="pill.key"
          type="button"
          :class="cn(
            filterPillVariants({ tone: pill.tone, active: statusFilter === pill.key }),
            pill.key === 'all' ? '' : 'flex items-center gap-1'
          )"
          @click="statusFilter = pill.key"
        >
          <span v-if="pill.dot" :class="pill.dot"></span>
          <span>{{ pill.label() }}</span>
        </button>
      </div>
    </div>
    <Separator class="bg-slate-100 dark:bg-dark-800/80" />

    <!-- Scrollable Navigation Items -->
    <div class="flex-1 overflow-y-auto custom-scrollbar p-2 space-y-4">

      <!-- Section 1: Global Smart Views -->
      <div class="space-y-1">
        <div class="px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-400">全局视图</div>

        <!-- View All Accounts -->
        <button
          type="button"
          data-testid="pool-all"
          :class="navItemVariants({ tone: 'teal', active: scope === 'all' })"
          @click="$emit('select', { scope: 'all' })"
        >
          <div class="flex items-center gap-2.5 min-w-0">
            <div class="w-6 h-6 rounded-lg bg-teal-50 dark:bg-teal-950/60 text-teal-600 dark:text-teal-400 flex items-center justify-center shrink-0">
              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" /></svg>
            </div>
            <div class="truncate">
              <div class="leading-tight">{{ t('resourcePool.navigator.allAccounts') }}</div>
              <div class="text-[10px] text-slate-400 dark:text-slate-400 font-normal">系统所有已入库账号</div>
            </div>
          </div>
          <Badge variant="count">{{ allCount }}</Badge>
        </button>

        <!-- View Ungrouped Accounts -->
        <button
          type="button"
          data-testid="pool-ungrouped"
          :class="navItemVariants({ tone: 'amber', active: scope === 'ungrouped' })"
          @click="$emit('select', { scope: 'ungrouped' })"
        >
          <div class="flex items-center gap-2.5 min-w-0">
            <div class="w-6 h-6 rounded-lg bg-amber-50 dark:bg-amber-950/60 text-amber-600 dark:text-amber-400 flex items-center justify-center shrink-0">
              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" /></svg>
            </div>
            <div class="truncate">
              <div class="leading-tight">{{ t('resourcePool.navigator.ungrouped') }}</div>
              <div class="text-[10px] text-slate-400 dark:text-slate-400 font-normal">需归属至对应资源池</div>
            </div>
          </div>
          <Badge :variant="ungroupedCount > 0 ? 'count-amber' : 'count-muted'">{{ ungroupedCount }}</Badge>
        </button>

        <!-- View Issues Accounts -->
        <button
          type="button"
          data-testid="pool-issues"
          :class="navItemVariants({ tone: 'rose', active: scope === 'issues' })"
          @click="$emit('select', { scope: 'issues' })"
        >
          <div class="flex items-center gap-2.5 min-w-0">
            <div class="w-6 h-6 rounded-lg bg-rose-50 dark:bg-rose-950/60 text-rose-600 dark:text-rose-400 flex items-center justify-center shrink-0">
              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" /></svg>
            </div>
            <div class="truncate">
              <div class="leading-tight">限流与异常监控</div>
              <div class="text-[10px] text-slate-400 dark:text-slate-400 font-normal">需排查或冷却中账号</div>
            </div>
          </div>
          <Badge variant="rose">{{ limitedCount ?? 0 }}</Badge>
        </button>
      </div>

      <!-- Section 2: Platform Grouped Pools List -->
      <div class="space-y-3">
        <div class="flex items-center justify-between px-2 text-[10px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-400">
          <span>平台资源池群组</span>
          <span class="font-mono">{{ filteredGroupsCount }} 个</span>
        </div>

        <!-- Loop Platform Categories -->
        <div v-for="section in groupedPools" :key="section.platform" class="space-y-1">
          <!-- Platform Category Header -->
          <div class="flex items-center justify-between px-2 py-1 text-xs font-semibold text-slate-500 dark:text-slate-400">
            <div class="flex items-center gap-1.5">
              <!-- Platform Icon -->
              <span class="w-4 h-4 flex items-center justify-center">
                <PlatformIcon :platform="section.platform" size="md" :class="platformIconColor(section.platform)" />
              </span>
              <span>{{ platformLabel(section.platform) }}</span>
            </div>
            <Badge variant="platform-count">{{ section.groups.length }}</Badge>
          </div>

          <!-- Pool Items Under Platform -->
          <div class="space-y-0.5 pl-1.5">
            <div
              v-for="group in section.groups"
              :key="group.id"
              :data-testid="`pool-group-${group.id}`"
              :class="poolItemVariants({ active: scope === 'group' && groupId === group.id })"
              @click="$emit('select', { scope: 'group', groupId: group.id })"
            >
              <!-- Left: Status Dot + Name + Multiplier Tag -->
              <div class="flex items-center gap-2 min-w-0 flex-1 pr-2">
                <!-- Status Indicator with Ring -->
                <span
                  :class="group.status === 'active' ? 'bg-emerald-500 shadow-emerald-500/40' : 'bg-slate-400'"
                  class="w-2 h-2 rounded-full shrink-0 shadow-sm"
                ></span>

                <span class="truncate font-medium text-slate-800 dark:text-slate-200" :title="group.name">
                  {{ group.name }}
                </span>

                <Badge v-if="group.rate_multiplier && group.rate_multiplier !== 1" variant="multiplier">
                  {{ Number(group.rate_multiplier).toFixed(1) }}x
                </Badge>
              </div>

              <!-- Right: Account Load Metrics & Alert Badge -->
              <div class="flex items-center gap-1.5 shrink-0">
                <span
                  v-if="group.rate_limited_account_count && group.rate_limited_account_count > 0"
                  class="w-1.5 h-1.5 rounded-full bg-amber-500 animate-pulse"
                  :title="`${group.rate_limited_account_count} 个账号速率受限中`"
                ></span>

                <Badge :variant="scope === 'group' && groupId === group.id ? 'count-active' : 'pool-count'">
                  {{ group.active_account_count ?? 0 }}/{{ group.account_count ?? 0 }}
                </Badge>
              </div>
            </div>
          </div>
        </div>

        <!-- Empty Search State in Navigator -->
        <div v-if="groupedPools.length === 0" class="py-6 text-center text-xs text-slate-400">
          未找到匹配的资源池
        </div>
      </div>
    </div>

    <!-- Navigator Bottom Fixed Action -->
    <Separator class="bg-slate-100 dark:bg-dark-800/80" />
    <div class="p-3 bg-slate-50/50 dark:bg-dark-900/50 flex items-center justify-between">
      <div class="text-[11px] text-slate-400">
        已同步 <strong>{{ groups.length }}</strong> 个资源池
      </div>
      <Button
        variant="ghost"
        class="h-auto gap-1 px-2.5 py-1 rounded-lg text-xs font-medium text-primary-600 dark:text-primary-400 hover:bg-primary-500/10 hover:text-primary-600 dark:hover:text-primary-400"
        @click="$emit('create')"
      >
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
        <span>添加池</span>
      </Button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { filterPillVariants, navItemVariants, poolItemVariants } from './navigatorVariants'
import type { AdminGroup, GroupPlatform } from '@/types'

type PoolSelection = { scope: 'all' | 'ungrouped' | 'issues' | 'group'; groupId?: number }
type StatusFilter = 'all' | 'active' | 'issue'

const props = defineProps<{ groups: AdminGroup[]; scope: PoolSelection['scope']; groupId?: number; allCount: number; ungroupedCount: number; limitedCount?: number; loading?: boolean }>()
defineEmits<{ select: [selection: PoolSelection]; create: []; refresh: [] }>()
const { t } = useI18n()

const search = ref('')
const statusFilter = ref<StatusFilter>('all')

const filterPills: Array<{ key: StatusFilter; tone: 'primary' | 'emerald' | 'amber'; label: () => string; dot?: string }> = [
  { key: 'all', tone: 'primary', label: () => `全部 (${props.groups.length})` },
  { key: 'active', tone: 'emerald', label: () => '正常', dot: 'w-1.5 h-1.5 rounded-full bg-emerald-500' },
  { key: 'issue', tone: 'amber', label: () => '受限/异常', dot: 'w-1.5 h-1.5 rounded-full bg-amber-500' }
]

const groupedPools = computed(() => {
  let list = props.groups
  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter(g => g.name.toLowerCase().includes(q) || g.platform.toLowerCase().includes(q))
  }
  if (statusFilter.value === 'active') {
    list = list.filter(g => g.status === 'active')
  } else if (statusFilter.value === 'issue') {
    list = list.filter(g => (g.rate_limited_account_count ?? 0) > 0 || g.status !== 'active')
  }

  // Group by platform
  const map: Record<string, AdminGroup[]> = {}
  for (const g of list) {
    if (!map[g.platform]) map[g.platform] = []
    map[g.platform].push(g)
  }
  return Object.keys(map).map(p => ({
    platform: p as GroupPlatform,
    groups: map[p].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
  }))
})

const filteredGroupsCount = computed(() => groupedPools.value.reduce((acc, s) => acc + s.groups.length, 0))

function platformLabel(platform: string) {
  const labels: Record<string, string> = { anthropic: 'Anthropic Claude', openai: 'OpenAI', gemini: 'Google Gemini', antigravity: 'Antigravity', grok: 'xAI Grok', bedrock: 'AWS Bedrock', vertex: 'Google Vertex AI', typesafe: 'TypeSafe', kimi: 'Moonshot Kimi', zhipu: '智谱 GLM', deepseek: 'DeepSeek', opencode_go: 'OpenCode Go', ollama: 'Ollama', minimax: 'MiniMax' }
  return labels[platform] ?? platform
}

// 平台图标配色与 demo 的 getPlatformIconSvg 内嵌颜色一致
function platformIconColor(platform: string) {
  const colors: Record<string, string> = {
    anthropic: 'text-amber-600 dark:text-amber-400',
    openai: 'text-emerald-600 dark:text-emerald-400',
    deepseek: 'text-sky-600 dark:text-sky-400',
    gemini: 'text-indigo-500'
  }
  return colors[platform] ?? 'text-slate-500'
}
</script>

<style scoped>
/* 复刻 demo 的 .custom-scrollbar */
.custom-scrollbar {
  scrollbar-gutter: stable;
}
</style>
