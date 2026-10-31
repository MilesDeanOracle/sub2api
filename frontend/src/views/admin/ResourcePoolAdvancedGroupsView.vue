<template>
  <div class="advanced-groups-page">
    <el-alert type="info" :closable="false" show-icon>
      <template #title>{{ t('resourcePool.actions.advanced') }}</template>
      <div class="advanced-groups-page__notice">
        <span>{{ t('resourcePool.advanced.description') }}</span>
        <el-button type="primary" plain @click="backToPool">{{ t('resourcePool.advanced.back') }}</el-button>
      </div>
    </el-alert>
    <GroupsView />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import GroupsView from './GroupsView.vue'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
function backToPool() {
  const groupId = typeof route.query.return_group_id === 'string' ? route.query.return_group_id : undefined
  void router.push({ path: '/admin/resource-pools', query: groupId ? { scope: 'group', group_id: groupId } : { scope: 'all' } })
}
</script>

<style scoped>
.advanced-groups-page { display: grid; gap: 12px; }
.advanced-groups-page__notice { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
</style>
