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

// Render country flag emojis cross-OS. Windows / some Linux normally show the
// regional indicator letter pairs ("JP", "US") because the system font has
// no flag glyphs. We unconditionally inject a @font-face for Twemoji Country
// Flags (bundled locally) and add the font as the FIRST family on body, plus
// explicit usage on .node-flag etc. We don't rely on the polyfill's runtime
// detection because it has been observed to skip on some Windows setups.
import twemojiFlagsUrl from './assets/fonts/TwemojiCountryFlags.woff2?url'
;(function injectFlagFont() {
  const el = document.createElement('style')
  el.textContent = `
    @font-face {
      font-family: "Twemoji Country Flags";
      unicode-range: U+1F1E6-1F1FF, U+1F3F4, U+E0062-E0063, U+E0065, U+E0067,
        U+E006C, U+E006E, U+E0073-E0074, U+E0077, U+E007F;
      src: url('${twemojiFlagsUrl}') format('woff2');
      font-display: swap;
    }
    body, .node-flag {
      font-family: "Twemoji Country Flags", Arial, sans-serif;
    }
  `
  document.head.appendChild(el)
})()

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
