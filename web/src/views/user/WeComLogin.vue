<template>
  <div class="wecom-login-page">
    <section
      class="scanner-panel"
      aria-live="polite"
    >
      <div class="scanner-heading">
        <div>
          <span class="scanner-kicker">扫码登录</span>
          <h2>企业微信</h2>
        </div>
        <span class="official-badge">官方认证</span>
      </div>

      <div class="qr-stage">
        <div
          v-if="status === 'loading'"
          class="state-panel"
        >
          <el-icon class="state-icon is-loading">
            <Loading />
          </el-icon>
          <strong>正在加载安全二维码</strong>
          <span>请稍候，不要关闭当前页面</span>
        </div>
        <div
          v-else-if="status === 'error'"
          class="state-panel state-panel--error"
        >
          <el-icon class="state-icon">
            <WarningFilled />
          </el-icon>
          <strong>二维码加载失败</strong>
          <span>{{ errorMessage }}</span>
          <el-button
            type="primary"
            @click="loadWeComLogin"
          >
            重新加载
          </el-button>
        </div>
        <div
          v-show="status === 'ready'"
          id="wecom-login-frame"
          class="wecom-frame"
        />
      </div>

      <div class="scan-guide">
        <span class="step-number">1</span>
        <span>打开企业微信</span>
        <span class="step-line" />
        <span class="step-number">2</span>
        <span>点击右上角“+”扫一扫</span>
      </div>

      <div class="scanner-actions">
        <el-button
          :disabled="status === 'loading'"
          @click="loadWeComLogin"
        >
          <el-icon><RefreshRight /></el-icon>
          刷新二维码
        </el-button>
        <el-button
          v-if="authorizeUrl"
          @click="openFallbackLogin"
        >
          在新页面打开
          <el-icon><TopRight /></el-icon>
        </el-button>
      </div>

      <router-link
        :to="loginRoute"
        class="back-link"
      >
        <el-icon><ArrowLeft /></el-icon>
        返回账号密码登录
      </router-link>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  ArrowLeft,
  Loading,
  RefreshRight,
  TopRight,
  WarningFilled
} from '@element-plus/icons-vue'
import { getOAuthEmbedConfig } from '@/api/modules/portal/auth'
import { extractErrorMessage } from '@/utils/entitlement'

const WECOM_SDK_URL = 'https://wwcdn.weixin.qq.com/node/wework/wwopen/js/wwLogin-1.2.7.js'

const route = useRoute()
const status = ref('loading')
const errorMessage = ref('请检查网络连接后重试')
const authorizeUrl = ref('')
let loginWidget = null

const redirectPath = computed(() => {
  const redirect = route.query.redirect
  return typeof redirect === 'string' && redirect.startsWith('/') && !redirect.startsWith('//') ? redirect : ''
})

const loginRoute = computed(() => ({
  path: '/user/login',
  query: redirectPath.value ? { redirect: redirectPath.value } : {}
}))

function loadScript() {
  if (window.WwLogin) return Promise.resolve()
  return new Promise((resolve, reject) => {
    const existing = document.querySelector(`script[src="${WECOM_SDK_URL}"]`)
    if (existing) {
      if (existing.dataset.loaded === 'true') {
        existing.remove()
      } else {
        existing.addEventListener('load', resolve, { once: true })
        existing.addEventListener('error', reject, { once: true })
        return
      }
    }
    const script = document.createElement('script')
    script.src = WECOM_SDK_URL
    script.async = true
    script.onload = () => {
      script.dataset.loaded = 'true'
      resolve()
    }
    script.onerror = () => {
      script.remove()
      reject(new Error('failed to load WeCom login SDK'))
    }
    document.head.appendChild(script)
  })
}

async function loadWeComLogin() {
  status.value = 'loading'
  errorMessage.value = '请检查网络连接后重试'
  loginWidget?.destroyed?.()
  loginWidget = null

  try {
    const config = await getOAuthEmbedConfig('wecom', redirectPath.value)
    authorizeUrl.value = config.authorize_url || ''
    await loadScript()
    await nextTick()
    loginWidget = new window.WwLogin({
      id: 'wecom-login-frame',
      appid: config.appid,
      agentid: config.agentid,
      redirect_uri: encodeURIComponent(config.redirect_uri),
      state: config.state,
      lang: 'zh'
    })
    status.value = 'ready'
  } catch (error) {
    errorMessage.value = extractErrorMessage(error) || '企业微信登录暂时不可用'
    status.value = 'error'
  }
}

function openFallbackLogin() {
  window.location.href = authorizeUrl.value
}

onMounted(loadWeComLogin)
onBeforeUnmount(() => loginWidget?.destroyed?.())
</script>

<style scoped>
.wecom-login-page {
  width: 100%;
  max-width: 420px;
  margin: 0 auto;
  overflow: hidden;
  border-radius: 24px;
  background: var(--color-bg-card);
}

.scanner-panel {
  display: flex;
  min-width: 0;
  flex-direction: column;
  padding: 38px 32px 28px;
}

.scanner-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.scanner-kicker {
  color: #09a568;
}

.scanner-heading h2 {
  margin: 5px 0 0;
  color: var(--color-text-primary);
  font-size: 25px;
}

.official-badge {
  padding: 5px 9px;
  border-radius: 999px;
  color: #087b53;
  background: #e9fbf2;
  font-size: 11px;
  font-weight: 700;
}

.qr-stage {
  display: grid;
  min-height: 350px;
  margin: 22px 0 14px;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background:
    linear-gradient(90deg, transparent 49%, color-mix(in srgb, var(--color-border) 44%, transparent) 50%, transparent 51%) 0 0 / 24px 24px,
    linear-gradient(transparent 49%, color-mix(in srgb, var(--color-border) 44%, transparent) 50%, transparent 51%) 0 0 / 24px 24px,
    var(--color-bg-elevated-hover);
}

.wecom-frame {
  width: 300px;
  height: 400px;
  margin-bottom: -38px;
  overflow: hidden;
}

.state-panel {
  display: flex;
  max-width: 240px;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  color: var(--color-text-secondary);
  text-align: center;
}

.state-panel strong {
  color: var(--color-text-primary);
  font-size: 16px;
}

.state-panel span {
  font-size: 13px;
  line-height: 1.6;
}

.state-icon {
  color: #09a568;
  font-size: 34px;
}

.state-panel--error .state-icon {
  color: var(--el-color-danger);
}

.scan-guide {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  color: var(--color-text-secondary);
  font-size: 12px;
}

.step-number {
  display: inline-grid;
  width: 20px;
  height: 20px;
  place-items: center;
  border-radius: 50%;
  color: #087b53;
  background: #e9fbf2;
  font-size: 11px;
  font-weight: 700;
}

.step-line {
  width: 18px;
  height: 1px;
  background: var(--color-border-dark);
}

.scanner-actions {
  display: flex;
  justify-content: center;
  gap: 8px;
  margin-top: 20px;
}

.scanner-actions :deep(.el-button) {
  margin: 0;
  border-radius: 9px;
}

.back-link {
  display: inline-flex;
  align-items: center;
  align-self: center;
  gap: 6px;
  margin-top: auto;
  padding-top: 20px;
  color: var(--color-text-secondary);
  font-size: 13px;
  text-decoration: none;
}

.back-link:hover {
  color: #09a568;
}

:global(.dark) .official-badge,
:global(.dark) .step-number {
  color: #7bf0b4;
  background: rgba(9, 165, 104, 0.16);
}

@media (max-width: 350px) {
  .wecom-frame {
    margin: -28px -21px -66px;
    transform: scale(0.86);
  }
}

@media (max-width: 480px) {
  .wecom-login-page {
    border-radius: 16px;
  }


  .scanner-panel {
    padding: 24px 16px 20px;
  }

  .qr-stage {
    min-height: 340px;
  }

  .scan-guide {
    flex-wrap: wrap;
  }

  .scanner-actions {
    flex-direction: column;
  }

  .scanner-actions :deep(.el-button) {
    width: 100%;
  }
}
</style>
