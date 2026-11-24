import { createApp } from 'vue'
import { createPinia } from 'pinia'
// element-plus 按需引入：全站仅资源池页（views/admin/ResourcePoolsViewElement.vue
// 及其 components/element/ 子组件）使用 element-plus。此处只注册实际用到的组件，
// 避免 app.use(ElementPlus) 全量注册把整个组件库（约 650KB JS）塞进首屏 vendor 包。
// ⚠️ 若在其他页面新增 element-plus 组件，需同步在此处注册并补充对应样式导入。
import {
  ElAlert,
  ElButton,
  ElCheckbox,
  ElCheckboxGroup,
  ElConfigProvider,
  ElDatePicker,
  ElDialog,
  ElDrawer,
  ElDropdown,
  ElDropdownItem,
  ElDropdownMenu,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElIcon,
  ElInput,
  ElOption,
  ElPagination,
  ElRadio,
  ElRadioGroup,
  ElSelect,
  ElSwitch,
  ElTable,
  ElTableColumn,
  ElTag,
  ElTooltip,
  ElLoading,
} from 'element-plus'
import App from './App.vue'
import router from './router'
import i18n, { initI18n } from './i18n'
import { useAppStore } from '@/stores/app'
import { updateFavicon } from '@/utils/branding'
import { isIOSDevice } from '@/utils/device'
import { initGoogleTagManager } from '@/utils/tagManager'
import './style.css'
// element-plus 按需样式：与上方注册的组件一一对应（kebab-case）。
// ElMessage/ElMessageBox 为程序化调用（components/element/ 下 7 个文件），
// 无模板组件可解析，样式必须在此显式导入。
import 'element-plus/es/components/alert/style/css'
import 'element-plus/es/components/button/style/css'
import 'element-plus/es/components/checkbox/style/css'
import 'element-plus/es/components/checkbox-group/style/css'
import 'element-plus/es/components/config-provider/style/css'
import 'element-plus/es/components/date-picker/style/css'
import 'element-plus/es/components/dialog/style/css'
import 'element-plus/es/components/drawer/style/css'
import 'element-plus/es/components/dropdown/style/css'
import 'element-plus/es/components/dropdown-item/style/css'
import 'element-plus/es/components/dropdown-menu/style/css'
import 'element-plus/es/components/empty/style/css'
import 'element-plus/es/components/form/style/css'
import 'element-plus/es/components/form-item/style/css'
import 'element-plus/es/components/icon/style/css'
import 'element-plus/es/components/input/style/css'
import 'element-plus/es/components/option/style/css'
import 'element-plus/es/components/pagination/style/css'
import 'element-plus/es/components/radio/style/css'
import 'element-plus/es/components/radio-group/style/css'
import 'element-plus/es/components/select/style/css'
import 'element-plus/es/components/switch/style/css'
import 'element-plus/es/components/table/style/css'
import 'element-plus/es/components/table-column/style/css'
import 'element-plus/es/components/tag/style/css'
import 'element-plus/es/components/tooltip/style/css'
import 'element-plus/es/components/loading/style/css'
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'
// 暗色模式变量挂在 html.dark 上，与项目 initThemeClass 的写法一致
import 'element-plus/theme-chalk/dark/css-vars.css'

function initIOSViewportZoomFix() {
  // iOS Safari 在输入框字号小于 16px 时聚焦会自动放大页面，且失焦后不会恢复。
  // 限制 maximum-scale 可阻止该行为；iOS 10+ 用户仍可双指手动缩放，不影响可访问性。
  // 仅在 iOS 设备上注入，避免影响 Android Chrome 的手动缩放能力。
  if (!isIOSDevice()) return

  const viewport = document.querySelector('meta[name="viewport"]')
  if (!viewport) return

  const content = viewport.getAttribute('content') || ''
  if (/maximum-scale/i.test(content)) return
  viewport.setAttribute('content', `${content}, maximum-scale=1.0`)
}

function initThemeClass() {
  const savedTheme = localStorage.getItem('theme')
  const shouldUseDark =
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', shouldUseDark)
}

async function bootstrap() {
  // Apply theme class globally before app mount to keep all routes consistent.
  initThemeClass()
  initIOSViewportZoomFix()

  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  // Initialize settings from injected config BEFORE mounting (prevents flash)
  // This must happen after pinia is installed but before router and i18n
  const appStore = useAppStore()
  appStore.initFromInjectedConfig()

  // Set document title immediately after config is loaded
  if (appStore.siteName && appStore.siteName !== 'Sub2API') {
    document.title = `${appStore.siteName} - AI API Gateway`
  }
  updateFavicon(appStore.siteLogo)

  await initI18n()

  app.use(router)
  app.use(i18n)
  // 按需注册 element-plus（等价于 app.use(ElementPlus) 的全量注册，但只含实际使用的组件）
  for (const component of [
    ElAlert, ElButton, ElCheckbox, ElCheckboxGroup, ElConfigProvider,
    ElDatePicker, ElDialog, ElDrawer, ElDropdown, ElDropdownItem, ElDropdownMenu,
    ElEmpty, ElForm, ElFormItem, ElIcon, ElInput, ElOption, ElPagination,
    ElRadio, ElRadioGroup, ElSelect, ElSwitch, ElTable, ElTableColumn,
    ElTag, ElTooltip,
  ]) {
    app.use(component)
  }
  app.use(ElLoading) // 注册 v-loading 指令

  // 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
  await router.isReady()
  app.mount('#app')

  // Load once after initial navigation; local development must not send analytics.
  if (import.meta.env.PROD) {
    initGoogleTagManager()
  }
}

bootstrap()
