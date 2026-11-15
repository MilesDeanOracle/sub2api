<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { operationsApi, type OperationsAccount, type OperationsTask, type ModelCatalog, type StorageSource } from '@/api/admin/operationsConsole'

const { t } = useI18n()
const app = useAppStore()
const tasks = ref<OperationsTask[]>([])
const accounts = ref<OperationsAccount[]>([])
const sources = ref<StorageSource[]>([])
const catalogs = reactive<Record<number, ModelCatalog>>({})
const modelLoading = reactive<Record<number, boolean>>({})
const modelErrors = reactive<Record<number, string>>({})
const busy = ref(new Set<number>())
const saving = ref(false)
const testing = ref(false)
const showEditor = ref(false)
const deleteTask = ref<OperationsTask | null>(null)
const group = ref<string | number | boolean | null>(0)
const batchModel = ref<string | number | boolean | null>('')
const blank = (): OperationsTask => ({ id: 0, name: '', enabled: false, interval_hours: 2, account_configs: [], reasoning_effort: '', prompt: t('operationsConsole.defaultPrompt'), storage_source: 'backup_s3', config_version: 0, last_triggered_at: null, next_run_at: null })
const form = ref<OperationsTask>(blank())
const groups = computed(() => [{ value: 0, label: t('operationsConsole.allGroups') }, { value: -1, label: t('operationsConsole.ungrouped') }, ...Array.from(new Map(accounts.value.flatMap(a => a.groups).map(g => [g.id, g])).values()).map(g => ({ value: g.id, label: g.name }))])
const visibleAccounts = computed(() => accounts.value.filter(a => !group.value || (Number(group.value) === -1 ? a.groups.length === 0 : a.groups.some(g => g.id === Number(group.value)))))
const unavailableChoices = computed(() => form.value.account_configs.filter(c => !accounts.value.some(a => a.id === c.account_id)))
const effortOptions = computed(() => [{ value: '', label: t('operationsConsole.defaultEffort') }, ...['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'].map(value => ({ value, label: value }))])
const commonModels = computed(() => {
  const choices = form.value.account_configs
  if (!choices.length || choices.some(c => !catalogs[c.account_id])) return []
  return (catalogs[choices[0]!.account_id]?.models || []).filter(m => choices.every(c => catalogs[c.account_id]?.models.includes(m))).map(m => ({ value: m, label: m }))
})
const storageOptions = computed(() => sources.value.map(s => ({ value: s.id, label: `${t(s.id === 'backup_s3' ? 'operationsConsole.backupS3' : 'operationsConsole.imageStorage')} · ${s.available ? s.bucket : t('operationsConsole.unavailable')}`, disabled: !s.available })))
let timer: ReturnType<typeof setInterval> | undefined
let loadVersion = 0
function error(err: unknown) { app.showError((err as { response?: { data?: { message?: string } }; message?: string })?.response?.data?.message || t('operationsConsole.error')) }
async function load() { const request = ++loadVersion; try { const rows = await operationsApi.tasks(); if (request === loadVersion) tasks.value = rows } catch (err) { if (request === loadVersion) error(err) } }
async function fetchModels(id: number, refresh = false) {
  if (modelLoading[id]) return
  modelLoading[id] = true; delete modelErrors[id]
  try { catalogs[id] = await operationsApi.models(id, refresh) } catch { delete catalogs[id]; modelErrors[id] = t('operationsConsole.modelFailed') } finally { modelLoading[id] = false }
}
function choice(id: number) { return form.value.account_configs.find(c => c.account_id === id) }
function toggleAccount(id: number, checked: boolean) {
  if (checked && !choice(id)) { form.value.account_configs.push({ account_id: id, model_id: '' }); void fetchModels(id) }
  else if (!checked) form.value.account_configs = form.value.account_configs.filter(c => c.account_id !== id)
}
function setModel(id: number, value: string | number | boolean | null) { const c = choice(id); if (c) c.model_id = String(value || '') }
function edit(task?: OperationsTask) {
  form.value = task ? JSON.parse(JSON.stringify(task)) : blank(); group.value = 0; batchModel.value = ''; showEditor.value = true
  for (const c of form.value.account_configs) void fetchModels(c.account_id)
}
function applyModel() { if (batchModel.value) for (const c of form.value.account_configs) c.model_id = String(batchModel.value) }
async function save() {
  if (form.value.account_configs.some(c => !catalogs[c.account_id]?.models.includes(c.model_id))) { app.showError(t('operationsConsole.modelsRequired')); return }
  saving.value = true
  try { await operationsApi.save(form.value); app.showSuccess(t('operationsConsole.saved')); showEditor.value = false; await load() } catch (err) { error(err) } finally { saving.value = false }
}
async function action(task: OperationsTask, kind: 'enable' | 'run') {
  if (busy.value.has(task.id)) return
  busy.value.add(task.id)
  try { if (kind === 'enable') await operationsApi.enable(task); else { const out = await operationsApi.trigger(task.id, crypto.randomUUID()); app.showSuccess(t(out.existing ? 'operationsConsole.roundExists' : 'operationsConsole.triggered')) }
    await load() } catch (err) { error(err) } finally { busy.value.delete(task.id) }
}
async function remove() {
  const target = deleteTask.value
  if (!target || busy.value.has(target.id)) return
  busy.value.add(target.id)
  try { await operationsApi.remove(target.id); if (deleteTask.value?.id === target.id) deleteTask.value = null; await load() } catch (err) { error(err) } finally { busy.value.delete(target.id) }
}
async function testStorage() { testing.value = true; try { await operationsApi.testStorage(form.value.storage_source); app.showSuccess(t('operationsConsole.storageOk')) } catch (err) { error(err) } finally { testing.value = false } }
function date(value: string | null) { return value ? new Date(value).toLocaleString() : '—' }
onMounted(async () => { await load(); try { [accounts.value, sources.value] = await Promise.all([operationsApi.accounts(), operationsApi.storage()]) } catch (err) { error(err) }
  timer = setInterval(() => { if (!document.hidden) void load() }, 10000) })
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <section>
    <div class="mb-4 flex justify-end">
      <button type="button" class="flex items-center gap-1.5 rounded-lg bg-primary-50 px-4 py-2 text-sm font-medium text-primary-700 transition-colors hover:bg-primary-100 dark:bg-dark-800 dark:text-primary-300 dark:hover:bg-dark-700" @click="edit()"><Icon name="plus" size="sm" />{{ t('operationsConsole.create') }}</button>
    </div>
    <div v-if="!tasks.length" class="flex flex-col items-center gap-3 rounded-2xl bg-white py-16 text-center text-sm text-slate-400 dark:bg-dark-900"><Icon name="inbox" size="xl" class="text-slate-300 dark:text-dark-600" /><p>{{ t('operationsConsole.taskEmpty') }}</p></div>
    <div class="space-y-4">
      <article v-for="task in tasks" :key="task.id" class="rounded-2xl bg-white p-5 shadow-sm transition-shadow hover:shadow-md dark:bg-dark-900">
        <div class="flex flex-wrap justify-between gap-4">
          <div class="min-w-0"><div class="flex items-center gap-2"><span class="h-2 w-2 shrink-0 rounded-full" :class="task.enabled ? 'bg-emerald-500' : 'bg-slate-300 dark:bg-dark-600'"></span><h3 class="truncate font-semibold">{{ task.name }}</h3></div><div class="mt-2 flex flex-wrap gap-1.5 text-xs text-slate-500"><span class="rounded-full bg-slate-100 px-2 py-0.5 dark:bg-dark-800">{{ t('operationsConsole.interval') }} {{ task.interval_hours }}</span><span class="rounded-full bg-slate-100 px-2 py-0.5 dark:bg-dark-800">{{ t('operationsConsole.accountsCount') }} {{ task.account_configs.length }}</span><span class="rounded-full bg-slate-100 px-2 py-0.5 dark:bg-dark-800">{{ t('operationsConsole.effort') }} {{ task.reasoning_effort || t('operationsConsole.defaultEffort') }}</span></div></div>
          <div class="flex flex-wrap items-center gap-1"><button type="button" role="switch" :aria-checked="task.enabled" :title="t(task.enabled ? 'operationsConsole.disable' : 'operationsConsole.enable')" :aria-label="t(task.enabled ? 'operationsConsole.disable' : 'operationsConsole.enable')" class="relative mr-2 h-5 w-9 shrink-0 rounded-full transition-colors disabled:opacity-50" :class="task.enabled ? 'bg-primary-500' : 'bg-slate-200 dark:bg-dark-700'" :disabled="busy.has(task.id)" @click="action(task, 'enable')"><span class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform" :class="task.enabled ? 'translate-x-4' : ''"></span></button><button type="button" class="flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-primary-600 transition-colors hover:bg-primary-50 disabled:opacity-50 dark:text-primary-300 dark:hover:bg-dark-800" :disabled="busy.has(task.id)" @click="action(task, 'run')"><Icon name="play" size="sm" />{{ t('operationsConsole.run') }}</button><button type="button" class="rounded-lg p-2 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-dark-800" :title="t('operationsConsole.edit')" :aria-label="t('operationsConsole.edit')" @click="edit(task)"><Icon name="edit" size="sm" /></button><button type="button" class="rounded-lg p-2 text-slate-400 transition-colors hover:bg-red-50 hover:text-red-500 disabled:opacity-50 dark:hover:bg-red-950" :title="t('operationsConsole.remove')" :aria-label="t('operationsConsole.remove')" :disabled="busy.has(task.id)" @click="deleteTask = task"><Icon name="trash" size="sm" /></button></div>
        </div>
        <div class="mt-4 flex flex-wrap gap-x-6 gap-y-2 border-t border-slate-100 pt-3 text-xs text-slate-400 dark:border-dark-800"><span>{{ t('operationsConsole.lastRun') }}: {{ date(task.last_triggered_at) }}</span><span>{{ t('operationsConsole.nextRun') }}: {{ date(task.next_run_at) }}</span><span v-for="(count, state) in task.progress" :key="state">{{ t(`operationsConsole.statuses.${state}`) }} {{ count }}</span></div>
      </article>
    </div>
    <p class="mt-6 text-xs text-slate-400">{{ t('operationsConsole.retention') }}</p>

    <BaseDialog :show="showEditor" :title="t(form.id ? 'operationsConsole.edit' : 'operationsConsole.create')" width="extra-wide" :close-on-click-outside="false" @close="!saving && (showEditor = false)">
      <form id="operations-task-form" class="space-y-7" @submit.prevent="save">
        <div class="grid gap-4 md:grid-cols-2"><label class="space-y-2 text-sm"><span>{{ t('operationsConsole.name') }}</span><Input v-model="form.name" required maxlength="200" /></label><label class="space-y-2 text-sm"><span>{{ t('operationsConsole.interval') }}</span><Input :model-value="form.interval_hours" @update:model-value="form.interval_hours = Number($event)" type="number" min="1" max="8760" required /></label></div>
        <div class="flex items-center justify-between gap-4 rounded-xl bg-slate-50 px-4 py-3 dark:bg-dark-800"><div class="min-w-0"><p class="text-sm font-medium">{{ t('operationsConsole.enabled') }}</p><p class="mt-0.5 text-xs text-slate-400">{{ t('operationsConsole.enabledHint') }}</p></div><button type="button" role="switch" :aria-checked="form.enabled" :aria-label="t('operationsConsole.enabled')" class="relative h-5 w-9 shrink-0 rounded-full transition-colors" :class="form.enabled ? 'bg-primary-500' : 'bg-slate-200 dark:bg-dark-700'" @click="form.enabled = !form.enabled"><span class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform" :class="form.enabled ? 'translate-x-4' : ''"></span></button></div>
        <div><label class="mb-2 block text-sm">{{ t('operationsConsole.effort') }}</label><Select v-model="form.reasoning_effort" :options="effortOptions" /><p class="mt-2 text-xs text-slate-500">{{ t('operationsConsole.sharedEffort') }}</p></div>
        <div class="space-y-3">
          <h3 class="text-sm font-medium">{{ t('operationsConsole.selectAccounts') }}</h3><p class="text-xs text-slate-400">{{ t('operationsConsole.accountHint') }}</p><Select v-model="group" :options="groups" />
          <div class="flex gap-2"><Select v-model="batchModel" class="flex-1" :options="commonModels" :placeholder="t('operationsConsole.batchModel')" /><Button type="button" variant="outline" :disabled="!batchModel" @click="applyModel">{{ t('operationsConsole.apply') }}</Button></div>
          <div class="max-h-80 space-y-1 overflow-y-auto rounded-xl bg-slate-50 p-2 dark:bg-dark-800">
            <label v-for="item in unavailableChoices" :key="`unavailable-${item.account_id}`" class="flex items-center gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"><input type="checkbox" checked class="h-4 w-4" @change="toggleAccount(item.account_id, false)" />{{ t('operationsConsole.accountUnavailable', { id: item.account_id }) }} · {{ item.model_id }}</label>
            <p v-if="!visibleAccounts.length" class="p-4 text-sm text-slate-500">{{ t('operationsConsole.noAccounts') }}</p>
            <div v-for="account in visibleAccounts" :key="account.id" class="space-y-2 rounded-lg px-3 py-2.5 transition-colors" :class="choice(account.id) ? 'bg-white shadow-sm dark:bg-dark-900' : 'hover:bg-white/70 dark:hover:bg-dark-900/60'">
              <label class="flex items-center gap-2 text-sm"><input type="checkbox" :checked="!!choice(account.id)" class="h-4 w-4 accent-primary-600" @change="toggleAccount(account.id, ($event.target as HTMLInputElement).checked)" /><span class="font-medium">{{ account.name }}</span><span class="text-xs text-slate-500">{{ account.groups.map(g => g.name).join(' / ') || t('operationsConsole.ungrouped') }} · {{ account.protocol }}</span></label>
              <div v-if="choice(account.id)" class="flex items-center gap-2 pl-6"><Select :model-value="choice(account.id)?.model_id || ''" class="min-w-0 flex-1" :options="(catalogs[account.id]?.models || []).map(m => ({ value: m, label: m }))" :placeholder="t('operationsConsole.selectModel')" :loading="modelLoading[account.id]" @update:model-value="setModel(account.id, $event)" /><Button type="button" variant="outline" :disabled="modelLoading[account.id]" @click="fetchModels(account.id, true)">{{ t('operationsConsole.discover') }}</Button></div>
              <p v-if="modelErrors[account.id]" class="pl-6 text-xs text-red-600">{{ modelErrors[account.id] }}</p>
            </div>
          </div>
        </div>
        <label class="block space-y-2 text-sm"><span>{{ t('operationsConsole.prompt') }}</span><textarea v-model="form.prompt" required rows="6" class="w-full resize-y rounded-xl border-0 bg-slate-50 p-3 font-mono text-sm outline-none ring-1 ring-transparent transition focus:ring-primary-300 dark:bg-dark-800 dark:focus:ring-primary-700"></textarea></label>
        <div class="space-y-2"><label class="block text-sm">{{ t('operationsConsole.storage') }}</label><Select v-model="form.storage_source" :options="storageOptions" /><p class="text-xs text-slate-500">{{ t('operationsConsole.storageHint') }}</p><button type="button" class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm text-primary-600 transition-colors hover:bg-primary-50 disabled:opacity-50 dark:text-primary-300 dark:hover:bg-dark-800" :disabled="testing" @click="testStorage"><Icon name="database" size="sm" :class="{ 'animate-pulse': testing }" />{{ t('operationsConsole.testStorage') }}</button></div>
      </form>
      <template #footer><div class="flex justify-end gap-2"><Button variant="outline" :disabled="saving" @click="showEditor = false">{{ t('operationsConsole.cancel') }}</Button><Button type="submit" form="operations-task-form" :disabled="saving || !form.account_configs.length">{{ t(saving ? 'operationsConsole.loading' : 'operationsConsole.save') }}</Button></div></template>
    </BaseDialog>
    <ConfirmDialog :show="!!deleteTask" :title="t('operationsConsole.removeTitle')" :message="t('operationsConsole.removeHint')" danger @confirm="remove" @cancel="deleteTask = null" />
  </section>
</template>
