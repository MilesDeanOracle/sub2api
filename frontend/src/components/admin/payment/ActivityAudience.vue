<template>
  <section class="audience-card" :aria-labelledby="titleID" :aria-busy="loading || saving">
    <header><div><h3 :id="titleID">{{ copy.title }}</h3><p>{{ copy.intro }}</p></div><span class="audience-badge">{{ loaded ? savedMode === 'selected' ? copy.testing : copy.public : copy.unavailable }}</span></header>
    <fieldset :disabled="loading || saving || !loaded">
      <legend class="sr-only">{{ copy.title }}</legend>
      <div class="audience-options">
        <label :class="{selected: mode === 'selected'}"><input v-model="mode" :name="id" type="radio" value="selected"><span><strong>{{ copy.selected }}</strong><small>{{ copy.selectedHelp }}</small></span></label>
        <label :class="{selected: mode === 'all'}"><input v-model="mode" :name="id" type="radio" value="all"><span><strong>{{ copy.all }}</strong><small>{{ copy.allHelp }}</small></span></label>
      </div>
      <div v-show="mode === 'selected'" class="audience-users">
        <label :for="id">{{ copy.ids }}</label>
        <textarea :id="id" v-model="ids" :name="`${kind}_test_user_ids`" rows="2" maxlength="10000" :aria-describedby="invalid ? `${helpID} ${messageID}` : helpID" :aria-invalid="invalid || undefined" placeholder="12, 35, 108" spellcheck="false" @input="invalid=false" />
        <p :id="helpID">{{ copy.idsHelp }}</p>
      </div>
      <p class="audience-note">{{ kind === 'consumption' ? copy.consumption : copy.checkin }}</p>
      <footer><span>{{ dirty ? copy.unsaved : copy.independent }}</span><button type="button" class="btn btn-primary" :disabled="saving" @click="save">{{ saving ? copy.saving : copy.save }}</button></footer>
    </fieldset>
    <p v-if="message" :id="messageID" :role="failed ? 'alert' : 'status'" :class="{'audience-error':failed}">{{ message }} <button v-if="!loaded" type="button" class="underline" @click="load">{{ copy.retry }}</button></p>
  </section>
</template>

<script setup lang="ts">
import {computed,onMounted,ref,useId} from 'vue'
import {useI18n} from 'vue-i18n'
import {activityAudienceAPI,type ActivityKind} from '@/api/admin/activityAudience'
import {extractApiErrorMessage} from '@/utils/apiError'
const props=defineProps<{kind:ActivityKind}>()
const {locale}=useI18n()
const id=useId(),titleID=useId(),helpID=useId(),messageID=useId()
const mode=ref<'selected'|'all'>('selected'),savedMode=ref<'selected'|'all'>('selected'),ids=ref('')
const savedIDs=ref('')
const dirty=computed(()=>loaded.value && (mode.value!==savedMode.value || ids.value!==savedIDs.value))
const loading=ref(true),loaded=ref(false),saving=ref(false),message=ref(''),failed=ref(false),invalid=ref(false)
const copy=computed(()=>locale.value.startsWith('zh')?{
 title:'活动开放范围',intro:'先邀请测试账号验证，再向用户开放。',testing:'指定用户测试中',public:'已全量可见',unavailable:'范围未加载',selected:'指定用户',selectedHelp:'仅名单内用户可见并参与',all:'所有用户',allHelp:'移除测试名单限制',ids:'测试用户 ID',idsHelp:'从用户管理复制 ID，以逗号、空格或换行分隔，最多 500 个。名单为空时所有用户均不可见；保存时校验用户是否存在。',consumption:'活动发布、参与比例和预算限制仍然生效。切换全量不会重置进度；同一期内的有效消费仍按活动规则统计。建议测试结束后新建正式活动，避免测试周期影响正式奖励。',checkin:'可见不等于可以领取：签到开关、参与比例、消费门槛与发奖安全开关仍须满足。',unsaved:'有未保存的范围更改',independent:'本区域需单独保存；不改变金额与活动状态。',save:'保存开放范围',saving:'保存中…',success:'开放范围已保存，下次用户页面刷新时生效。',error:'开放范围保存失败',loadError:'开放范围加载失败；禁止覆盖保存。',invalid:'请输入正整数用户 ID，最多 500 个，不支持邮箱或昵称。',retry:'重试'
}:{title:'Activity audience',intro:'Verify with test accounts before opening access.',testing:'Selected users only',public:'Visible to everyone',unavailable:'Audience unavailable',selected:'Selected users',selectedHelp:'Only listed users can see and participate',all:'All users',allHelp:'Remove the test user restriction',ids:'Test user IDs',idsHelp:'Copy IDs from user management. Separate with commas, spaces or new lines; up to 500. An empty list hides the activity from everyone. IDs are checked on save.',consumption:'Publication, rollout and budgets still apply. Opening access retains progress and counts eligible spending within the same campaign. Create a new campaign after testing to separate test and public periods.',checkin:'Visibility does not guarantee rewards: check-in activation, rollout, spend requirements and payout gates still apply.',unsaved:'Unsaved audience changes',independent:'Save this section separately; amounts and activity state stay unchanged.',save:'Save audience',saving:'Saving…',success:'Audience saved; user pages apply it on refresh.',error:'Unable to save audience',loadError:'Unable to load audience; saving is blocked.',invalid:'Enter positive integer user IDs, up to 500; emails and names are not supported.',retry:'Retry'})
async function load(){loading.value=true;message.value='';try{const value=await activityAudienceAPI.get(props.kind);mode.value=value.mode;savedMode.value=value.mode;ids.value=value.user_ids.join(', ');savedIDs.value=ids.value;loaded.value=true}catch(e){failed.value=true;message.value=extractApiErrorMessage(e,copy.value.loadError)}finally{loading.value=false}}
async function save(){if(!loaded.value||saving.value)return;message.value='';failed.value=false
 const tokens=ids.value.trim()?ids.value.trim().split(/[,，\s]+/):[]
 if(tokens.length>500||tokens.some(value=>!/^\d+$/.test(value)||!Number.isSafeInteger(Number(value))||Number(value)<=0)){invalid.value=true;failed.value=true;message.value=copy.value.invalid;return}
 saving.value=true;try{await activityAudienceAPI.save(props.kind,{mode:mode.value,user_ids:[...new Set(tokens.map(Number))]});savedMode.value=mode.value;savedIDs.value=ids.value;message.value=copy.value.success}catch(e){failed.value=true;message.value=extractApiErrorMessage(e,copy.value.error)}finally{saving.value=false}}
onMounted(load)
</script>

<style scoped>
.audience-card{margin-block:20px;padding:22px;border:1px solid #dbe4df;border-radius:16px;background:linear-gradient(120deg,#f5faf7,#fff);color:#334155}.audience-card header,.audience-card footer{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap}.audience-card h3{font-size:16px;font-weight:650}.audience-card p,.audience-card small,.audience-card footer>span{font-size:12px;line-height:1.8;color:#64748b}.audience-badge{border:1px solid #cce6d9;background:#eaf6ef;color:#22724c;border-radius:20px;padding:5px 12px;font-size:12px}.audience-card fieldset{border:0;padding:0;margin:18px 0 0;min-width:0}.audience-options{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.audience-options label{display:flex;align-items:flex-start;gap:10px;border:1px solid #cbd5e1;border-radius:10px;padding:14px;cursor:pointer;min-height:72px}.audience-options label.selected{border-color:#27956b;background:#edf8f1}.audience-options input{margin-top:4px;accent-color:#16845c}.audience-options strong,.audience-options small{display:block}.audience-options strong{font-size:14px}.audience-users{margin-top:16px}.audience-users>label{display:block;font-size:13px;font-weight:600;margin-bottom:6px}.audience-users textarea{display:block;width:100%;font:inherit;font-size:16px;padding:12px;border:1px solid #94a3b8;border-radius:8px;background:transparent;resize:vertical}.audience-users textarea[aria-invalid=true]{border-color:#dc2626}.audience-card .audience-note{margin-top:12px}.audience-card footer{margin-top:18px;padding-top:14px;border-top:1px solid #e2e8f0}.audience-card .audience-error{color:#b91c1c}.audience-card :is(button,input,textarea):focus-visible{outline:2px solid #16845c;outline-offset:3px}.audience-card fieldset:disabled{opacity:.6}.audience-card>p{margin-top:12px}
@media(max-width:640px){.audience-card{padding:16px}.audience-options{grid-template-columns:1fr}.audience-card footer button{width:100%;min-height:48px}}
</style>

<style>
.dark .audience-card{background:#17252a;border-color:#34474b;color:#e2e8f0}.dark .audience-options label.selected{background:#173b30;border-color:#409876}.dark .audience-card p,.dark .audience-card small,.dark .audience-card footer>span{color:#b2c2cd}.dark .audience-card .audience-error{color:#fca5a5}.dark .audience-badge{background:#173b30;border-color:#409876;color:#a5e5c4}.dark .audience-card footer{border-color:#34474b}

</style>
