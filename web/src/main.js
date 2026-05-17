import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import App from './App.vue'
import './assets/styles/main.css'
import './assets/styles/base.scss'
import './styles/theme.css'
import './styles/responsive.css'
import './styles/admin-ui.css'
import './styles/dark-mode-fixes.css'
import router from './router'
import { initializeViewportScaling } from './composables/useViewport'

// Cross-OS country flag emoji rendering (Windows Segoe UI Emoji lacks flags).
// Loaded as a regular stylesheet so it's allowed under CSP default-src 'self';
// uses a space-free font name so quoting can't be stripped by minifiers.
import './styles/flag-emoji.css'

// 导入事件源客户端
import xrayEventSource from './utils/eventSourceClient'

// 导入全局错误处理器
import { installGlobalErrorHandler } from './utils/globalErrorHandler'

initializeViewportScaling()

// 创建Vue实例和状态管理
const app = createApp(App)
const pinia = createPinia()

// 安装全局错误处理器
installGlobalErrorHandler(app)

// 使用插件
app.use(router)
app.use(pinia)
app.use(ElementPlus)

// 初始化SSE连接监听Xray版本事件
xrayEventSource.init()

// 挂载应用
app.mount('#app') 
