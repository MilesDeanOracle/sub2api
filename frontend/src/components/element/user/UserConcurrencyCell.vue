<template>
  <el-tag :type="statusType" size="small" disable-transitions effect="light" class="font-mono">
    <el-icon class="mr-0.5"><Grid /></el-icon>
    {{ current }}
    <span class="mx-0.5 opacity-50">/</span>
    {{ max }}
  </el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Grid } from '@element-plus/icons-vue'

const props = defineProps<{
  current: number
  max: number
}>()

// 满载红 / 占用黄 / 空闲灰，与原版语义一致
const statusType = computed<'danger' | 'warning' | 'info'>(() => {
  const { current, max } = props
  if (current >= max && max > 0) return 'danger'
  if (current > 0) return 'warning'
  return 'info'
})
</script>
