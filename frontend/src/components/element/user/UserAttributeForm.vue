<template>
  <div v-if="attributes.length > 0" class="space-y-4">
    <div v-for="attr in attributes" :key="attr.id">
      <label class="ep-field-label">
        {{ attr.name }}
        <span v-if="attr.required" class="text-red-500">*</span>
      </label>

      <!-- 文本 / 邮箱 / URL -->
      <el-input
        v-if="attr.type === 'text' || attr.type === 'email' || attr.type === 'url'"
        v-model="localValues[attr.id]"
        :type="attr.type === 'text' ? 'text' : attr.type"
        :placeholder="attr.placeholder"
        @input="emitChange"
      />

      <!-- 数字 -->
      <el-input
        v-else-if="attr.type === 'number'"
        :model-value="localValues[attr.id]"
        type="number"
        :placeholder="attr.placeholder"
        @update:model-value="(val: string | number) => { localValues[attr.id] = String(val ?? ''); emitChange() }"
      />

      <!-- 日期 -->
      <el-date-picker
        v-else-if="attr.type === 'date'"
        v-model="localValues[attr.id]"
        type="date"
        value-format="YYYY-MM-DD"
        :placeholder="attr.placeholder || undefined"
        class="!w-full"
        @change="emitChange"
      />

      <!-- 多行文本 -->
      <el-input
        v-else-if="attr.type === 'textarea'"
        v-model="localValues[attr.id]"
        type="textarea"
        :rows="3"
        :placeholder="attr.placeholder"
        @input="emitChange"
      />

      <!-- 单选 -->
      <el-select
        v-else-if="attr.type === 'select'"
        v-model="localValues[attr.id]"
        clearable
        @change="emitChange"
      >
        <el-option
          v-for="opt in attr.options || []"
          :key="opt.value"
          :value="opt.value"
          :label="opt.label"
        />
      </el-select>

      <!-- 多选 -->
      <el-checkbox-group
        v-else-if="attr.type === 'multi_select'"
        :model-value="selectedValues(attr.id)"
        @update:model-value="(vals: CheckboxValue[]) => setMultiSelect(attr.id, vals.map(String))"
      >
        <el-checkbox v-for="opt in attr.options || []" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </el-checkbox>
      </el-checkbox-group>

      <p v-if="attr.description" class="ep-field-hint">{{ attr.description }}</p>
    </div>
  </div>

  <!-- Loading State -->
  <div v-else-if="loading" class="flex justify-center py-4">
    <el-icon class="is-loading text-gray-400"><Loading /></el-icon>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { adminAPI } from '@/api/admin'
import type { UserAttributeDefinition, UserAttributeValuesMap } from '@/types'

type CheckboxValue = string | number | boolean

interface Props {
  userId?: number
  modelValue: UserAttributeValuesMap
}

interface Emits {
  (e: 'update:modelValue', value: UserAttributeValuesMap): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const loading = ref(false)
const attributes = ref<UserAttributeDefinition[]>([])
const localValues = ref<UserAttributeValuesMap>({})
let requestVersion = 0

const loadAttributes = async () => {
  loading.value = true
  try {
    attributes.value = await adminAPI.userAttributes.listEnabledDefinitions()
  } catch (error) {
    console.error('Failed to load attributes:', error)
  } finally {
    loading.value = false
  }
}

const loadUserValues = async () => {
  if (!props.userId) return
  const version = ++requestVersion

  try {
    const values = await adminAPI.userAttributes.getUserAttributeValues(props.userId)
    if (version !== requestVersion) return
    const valuesMap: UserAttributeValuesMap = {}
    values.forEach(v => {
      valuesMap[v.attribute_id] = v.value
    })
    localValues.value = { ...valuesMap }
    emit('update:modelValue', localValues.value)
  } catch (error) {
    console.error('Failed to load user attribute values:', error)
  }
}

const emitChange = () => {
  emit('update:modelValue', { ...localValues.value })
}

// 多选值以 JSON 字符串存储，与原版数据格式保持一致
const selectedValues = (attrId: number): string[] => {
  const value = localValues.value[attrId]
  if (!value) return []
  try {
    const arr = JSON.parse(value)
    return Array.isArray(arr) ? arr : []
  } catch {
    return []
  }
}

const setMultiSelect = (attrId: number, vals: string[]) => {
  localValues.value[attrId] = JSON.stringify(vals ?? [])
  emitChange()
}

watch(() => props.modelValue, (newVal) => {
  if (newVal && Object.keys(newVal).length > 0) {
    localValues.value = { ...newVal }
  }
}, { immediate: true })

watch(() => props.userId, (newUserId, _, onCleanup) => {
  onCleanup(() => { requestVersion++ })
  localValues.value = {}
  if (newUserId) {
    loadUserValues()
  }
}, { immediate: true })

onMounted(() => {
  loadAttributes()
})
</script>

<style scoped>
.ep-field-label {
  @apply mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300;
}

.ep-field-hint {
  @apply mt-1 text-xs text-gray-400 dark:text-gray-500;
}
</style>
