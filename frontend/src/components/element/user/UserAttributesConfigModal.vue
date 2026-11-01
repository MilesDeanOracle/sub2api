<template>
  <el-dialog
    :model-value="show"
    :title="t('admin.users.attributes.title')"
    width="860px"
    align-center
    @update:model-value="(v: boolean) => !v && emit('close')"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <!-- 头部：说明 + 新增按钮 -->
      <div class="flex items-center justify-between">
        <p class="text-sm text-gray-500 dark:text-dark-400">
          {{ t('admin.users.attributes.description') }}
        </p>
        <el-button type="primary" size="small" :icon="PlusIcon" @click="openCreateModal">
          {{ t('admin.users.attributes.addAttribute') }}
        </el-button>
      </div>

      <!-- Loading -->
      <div v-if="loading" v-loading="true" class="h-40" />

      <!-- 空状态 -->
      <el-empty v-else-if="attributes.length === 0" :image-size="90">
        <template #description>
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.users.attributes.noAttributes') }}</p>
          <p class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.users.attributes.noAttributesHint') }}</p>
        </template>
      </el-empty>

      <!-- 属性列表 -->
      <div v-else class="max-h-96 space-y-2 overflow-y-auto">
        <div
          v-for="attr in attributes"
          :key="attr.id"
          class="flex items-center gap-3 rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800"
        >
          <!-- 拖拽手柄（视觉指示） -->
          <el-icon class="cursor-move text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" :title="t('admin.users.attributes.dragToReorder')">
            <Rank />
          </el-icon>

          <!-- 属性信息 -->
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="font-medium text-gray-900 dark:text-white">{{ attr.name }}</span>
              <el-tag size="small" type="info" effect="plain" class="!font-mono" disable-transitions>
                {{ attr.key }}
              </el-tag>
              <el-tag v-if="attr.required" size="small" type="danger" disable-transitions>
                {{ t('admin.users.attributes.required') }}
              </el-tag>
              <el-tag v-if="!attr.enabled" size="small" type="info" disable-transitions>
                {{ t('common.disabled') }}
              </el-tag>
            </div>
            <div class="mt-0.5 flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
              <el-tag size="small" effect="plain" disable-transitions>
                {{ t(`admin.users.attributes.types.${attr.type}`) }}
              </el-tag>
              <span v-if="attr.description" class="truncate">{{ attr.description }}</span>
            </div>
          </div>

          <!-- 操作 -->
          <div class="flex items-center gap-1">
            <el-button link :icon="EditIcon" :title="t('common.edit')" @click="openEditModal(attr)" />
            <el-button link type="danger" :icon="DeleteIcon" :title="t('common.delete')" @click="confirmDelete(attr)" />
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button @click="emit('close')">{{ t('common.close') }}</el-button>
    </template>
  </el-dialog>

  <!-- 新建/编辑属性弹窗（嵌套，挂到 body） -->
  <el-dialog
    v-model="showEditModal"
    :title="editingAttribute ? t('admin.users.attributes.editAttribute') : t('admin.users.attributes.addAttribute')"
    width="520px"
    align-center
    append-to-body
  >
    <el-form label-position="top" @submit.prevent="handleSave">
      <el-form-item :label="t('admin.users.attributes.key')" required>
        <el-input
          v-model="form.key"
          class="!font-mono"
          :placeholder="t('admin.users.attributes.keyHint')"
          :disabled="!!editingAttribute"
        />
        <p class="ep-form-hint">{{ t('admin.users.attributes.keyHint') }}</p>
      </el-form-item>

      <el-form-item :label="t('admin.users.attributes.name')" required>
        <el-input v-model="form.name" :placeholder="t('admin.users.attributes.nameHint')" />
      </el-form-item>

      <el-form-item :label="t('admin.users.attributes.type')">
        <el-select v-model="form.type">
          <el-option
            v-for="type in attributeTypes"
            :key="type"
            :value="type"
            :label="t(`admin.users.attributes.types.${type}`)"
          />
        </el-select>
      </el-form-item>

      <!-- 选项（select/multi_select） -->
      <el-form-item v-if="form.type === 'select' || form.type === 'multi_select'" :label="t('admin.users.attributes.options')">
        <div class="w-full space-y-2">
          <div v-for="(option, index) in form.options" :key="getOptionKey(option)" class="flex items-center gap-2">
            <el-input
              v-model="option.value"
              class="flex-1 !font-mono"
              size="small"
              :placeholder="t('admin.users.attributes.optionValue')"
            />
            <el-input
              v-model="option.label"
              class="flex-1"
              size="small"
              :placeholder="t('admin.users.attributes.optionLabel')"
            />
            <el-button link type="danger" :icon="CloseIcon" size="small" @click="removeOption(index)" />
          </div>
          <el-button plain size="small" :icon="PlusIcon" @click="addOption">
            {{ t('admin.users.attributes.addOption') }}
          </el-button>
        </div>
      </el-form-item>

      <el-form-item :label="t('admin.users.attributes.fieldDescription')">
        <el-input v-model="form.description" :placeholder="t('admin.users.attributes.fieldDescriptionHint')" />
      </el-form-item>

      <el-form-item :label="t('admin.users.attributes.placeholder')">
        <el-input v-model="form.placeholder" :placeholder="t('admin.users.attributes.placeholderHint')" />
      </el-form-item>

      <div class="flex items-center gap-6">
        <el-checkbox v-model="form.required">{{ t('admin.users.attributes.required') }}</el-checkbox>
        <el-checkbox v-model="form.enabled">{{ t('admin.users.attributes.enabled') }}</el-checkbox>
      </div>
    </el-form>

    <template #footer>
      <el-button @click="closeEditModal">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="saving" @click="handleSave">
        {{ saving ? t('common.saving') : (editingAttribute ? t('common.update') : t('common.create')) }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import { Plus as PlusIcon, Rank, Edit as EditIcon, Delete as DeleteIcon, Close as CloseIcon } from '@element-plus/icons-vue'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { UserAttributeDefinition, UserAttributeType, UserAttributeOption } from '@/types'
import { createStableObjectKeyResolver } from '@/utils/stableObjectKey'

const { t } = useI18n()
const appStore = useAppStore()

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const attributeTypes: UserAttributeType[] = ['text', 'textarea', 'number', 'email', 'url', 'date', 'select', 'multi_select']

const loading = ref(false)
const saving = ref(false)
const attributes = ref<UserAttributeDefinition[]>([])
const showEditModal = ref(false)
const editingAttribute = ref<UserAttributeDefinition | null>(null)
const deletingAttribute = ref<UserAttributeDefinition | null>(null)
const getOptionKey = createStableObjectKeyResolver<UserAttributeOption>('user-attr-option')

const form = reactive({
  key: '',
  name: '',
  type: 'text' as UserAttributeType,
  description: '',
  placeholder: '',
  required: false,
  enabled: true,
  options: [] as UserAttributeOption[]
})

const loadAttributes = async () => {
  loading.value = true
  try {
    attributes.value = await adminAPI.userAttributes.listDefinitions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.attributes.failedToLoad'))
  } finally {
    loading.value = false
  }
}

const openCreateModal = () => {
  editingAttribute.value = null
  form.key = ''
  form.name = ''
  form.type = 'text'
  form.description = ''
  form.placeholder = ''
  form.required = false
  form.enabled = true
  form.options = []
  showEditModal.value = true
}

const openEditModal = (attr: UserAttributeDefinition) => {
  editingAttribute.value = attr
  form.key = attr.key
  form.name = attr.name
  form.type = attr.type
  form.description = attr.description || ''
  form.placeholder = attr.placeholder || ''
  form.required = attr.required
  form.enabled = attr.enabled
  form.options = attr.options ? attr.options.map((opt) => ({ ...opt })) : []
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingAttribute.value = null
}

const addOption = () => {
  form.options.push({ value: '', label: '' })
}

const removeOption = (index: number) => {
  form.options.splice(index, 1)
}

const handleSave = async () => {
  if (!form.key.trim()) {
    appStore.showError(t('admin.users.attributes.keyRequired'))
    return
  }
  if (!form.name.trim()) {
    appStore.showError(t('admin.users.attributes.nameRequired'))
    return
  }
  if ((form.type === 'select' || form.type === 'multi_select') && form.options.length === 0) {
    appStore.showError(t('admin.users.attributes.optionsRequired'))
    return
  }
  saving.value = true
  try {
    const data = {
      key: form.key,
      name: form.name,
      type: form.type,
      description: form.description,
      placeholder: form.placeholder,
      required: form.required,
      enabled: form.enabled,
      options: (form.type === 'select' || form.type === 'multi_select') ? form.options : undefined
    }

    if (editingAttribute.value) {
      await adminAPI.userAttributes.updateDefinition(editingAttribute.value.id, data)
      appStore.showSuccess(t('admin.users.attributes.updated'))
    } else {
      await adminAPI.userAttributes.createDefinition(data)
      appStore.showSuccess(t('admin.users.attributes.created'))
    }

    closeEditModal()
    loadAttributes()
  } catch (error: any) {
    const msg = editingAttribute.value
      ? t('admin.users.attributes.failedToUpdate')
      : t('admin.users.attributes.failedToCreate')
    appStore.showError(error.response?.data?.detail || msg)
  } finally {
    saving.value = false
  }
}

const confirmDelete = async (attr: UserAttributeDefinition) => {
  deletingAttribute.value = attr
  try {
    await ElMessageBox.confirm(
      t('admin.users.attributes.deleteConfirm', { name: attr.name }),
      t('admin.users.attributes.deleteAttribute'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel'),
        confirmButtonClass: 'el-button--danger',
        autofocus: false
      }
    )
  } catch {
    deletingAttribute.value = null
    return
  }

  try {
    await adminAPI.userAttributes.deleteDefinition(attr.id)
    appStore.showSuccess(t('admin.users.attributes.deleted'))
    loadAttributes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.attributes.failedToDelete'))
  } finally {
    deletingAttribute.value = null
  }
}

watch(() => props.show, (isShow) => {
  if (isShow) {
    loadAttributes()
  }
})
</script>

<style scoped>
.ep-form-hint {
  @apply mt-1 w-full text-xs text-gray-400 dark:text-gray-500;
}
</style>
