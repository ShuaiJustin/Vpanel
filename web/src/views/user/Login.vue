<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <h1 class="login-title">
          用户登录
        </h1>
        <p class="login-subtitle">
          欢迎回来，请登录您的账户
        </p>
      </div>

      <!-- 登录表单 -->
      <el-form
        v-if="!show2FA"
        ref="loginFormRef"
        :model="loginForm"
        :rules="loginRules"
        class="login-form"
        @submit.prevent="handleLogin"
      >
        <el-form-item prop="username">
          <el-input
            v-model="loginForm.username"
            placeholder="用户名或邮箱"
            size="large"
            :prefix-icon="User"
            autocomplete="username"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="密码"
            size="large"
            :prefix-icon="Lock"
            show-password
            autocomplete="current-password"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <div class="form-options">
          <el-checkbox v-model="loginForm.remember">
            记住我
          </el-checkbox>
          <router-link
            to="/user/forgot-password"
            class="forgot-link"
          >
            忘记密码？
          </router-link>
        </div>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            class="login-btn"
            @click="handleLogin"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>

      <div
        v-if="!show2FA && oauthProviders.length"
        class="oauth-login"
      >
        <div class="oauth-divider">
          <span>第三方登录</span>
        </div>
        <div class="oauth-buttons">
          <button
            v-for="provider in oauthProviders"
            :key="provider.key"
            type="button"
            class="oauth-button"
            :class="{ 'oauth-button--wecom': provider.key === 'wecom' }"
            :disabled="Boolean(oauthLoading)"
            @click="startOAuthLogin(provider.key)"
          >
            <img
              v-if="provider.key === 'wecom'"
              :src="weComLogo"
              class="wecom-logo"
              alt=""
              aria-hidden="true"
            >
            <span
              v-else
              class="oauth-mark"
            >{{ provider.label.slice(0, 1) }}</span>
            <span class="oauth-copy">
              <strong>{{ oauthLoading === provider.key ? provider.key === 'wecom' ? '正在打开企业微信…' : '正在跳转…' : provider.key === 'wecom' ? '企业微信扫码登录' : provider.label }}</strong>
            </span>
            <el-icon
              v-if="provider.key === 'wecom'"
              class="oauth-arrow"
            >
              <Loading
                v-if="oauthLoading === provider.key"
                class="is-loading"
              />
              <ArrowRight v-else />
            </el-icon>
          </button>
        </div>
      </div>

      <!-- 2FA 验证表单 -->
      <el-form
        v-if="show2FA"
        ref="twoFAFormRef"
        :model="twoFAForm"
        :rules="twoFARules"
        class="login-form"
        @submit.prevent="handle2FAVerify"
      >
        <div class="twofa-header">
          <el-icon class="twofa-icon">
            <Key />
          </el-icon>
          <h3>两步验证</h3>
          <p>请输入您的验证器应用中的验证码</p>
        </div>

        <el-form-item prop="code">
          <el-input
            v-model="twoFAForm.code"
            placeholder="6位验证码"
            size="large"
            maxlength="6"
            class="twofa-input"
            @keyup.enter="handle2FAVerify"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            class="login-btn"
            @click="handle2FAVerify"
          >
            验证
          </el-button>
        </el-form-item>

        <div class="twofa-options">
          <el-button
            link
            type="primary"
            @click="useBackupCode = !useBackupCode"
          >
            {{ useBackupCode ? '使用验证码' : '使用备份码' }}
          </el-button>
          <el-button
            link
            @click="cancelTwoFA"
          >
            返回登录
          </el-button>
        </div>
      </el-form>

      <div class="login-footer">
        <span>还没有账户？</span>
        <router-link
          to="/user/register"
          class="register-link"
        >
          立即注册
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowRight, Key, Loading, Lock, User } from '@element-plus/icons-vue'
import { useUserPortalStore } from '@/stores/userPortal'
import {
  getOAuthProviders,
  getOAuthStartUrl,
  verifyEmail as verifyPortalEmail
} from '@/api/modules/portal/auth'
import { extractErrorMessage, getErrorStatus } from '@/utils/entitlement'
import weComLogo from '@/assets/wecom-logo.png'

const router = useRouter()
const route = useRoute()
const userStore = useUserPortalStore()

// 表单引用
const loginFormRef = ref(null)
const twoFAFormRef = ref(null)

// 状态
const loading = ref(false)
const show2FA = ref(false)
const useBackupCode = ref(false)
const pendingUserId = ref(null)
const oauthProviders = ref([])
const oauthLoading = ref('')

// 登录表单
const loginForm = reactive({
  username: '',
  password: '',
  remember: false
})

// 2FA 表单
const twoFAForm = reactive({
  code: ''
})

// 登录表单验证规则
const loginRules = {
  username: [
    { required: true, message: '请输入用户名或邮箱', trigger: 'blur' },
    { min: 3, max: 64, message: '长度应为 3-64 个字符', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于 6 个字符', trigger: 'blur' }
  ]
}

// 2FA 表单验证规则
const twoFARules = {
  code: [
    { required: true, message: '请输入验证码', trigger: 'blur' },
    { 
      pattern: /^\d{6}$|^[A-Za-z0-9]{8}$/, 
      message: '请输入6位数字验证码或8位备份码', 
      trigger: 'blur' 
    }
  ]
}

function getSafeRedirectPath() {
  const redirect = route.query.redirect
  if (typeof redirect !== 'string' || !redirect.startsWith('/') || redirect.startsWith('//')) {
    return ''
  }
  return redirect
}

async function loadOAuthProviders() {
  try {
    const response = await getOAuthProviders()
    oauthProviders.value = Array.isArray(response?.providers) ? response.providers : []
  } catch {
    oauthProviders.value = []
  }
}

function startOAuthLogin(providerKey) {
  const redirect = getSafeRedirectPath()
  if (providerKey === 'wecom') {
    router.push({
      name: 'UserWeComLogin',
      query: redirect ? { redirect } : {}
    })
    return
  }
  oauthLoading.value = providerKey
  window.location.href = getOAuthStartUrl(providerKey, redirect)
}

function getPostLoginPath(user) {
  const redirect = getSafeRedirectPath()
  if (user?.role === 'admin') {
    return redirect.startsWith('/admin') ? redirect : userStore.adminEntryPath
  }

  return redirect && !redirect.startsWith('/admin') ? redirect : '/user/dashboard'
}

// 处理登录
async function handleLogin() {
  if (!loginFormRef.value) return
  
  try {
    await loginFormRef.value.validate()
    loading.value = true
    
    const response = await userStore.login({
      username: loginForm.username,
      password: loginForm.password,
      remember: loginForm.remember
    })
    
    // 检查是否需要 2FA 验证
    if (response.requires_2fa) {
      pendingUserId.value = response.user_id
      twoFAForm.code = ''
      show2FA.value = true
      ElMessage.info('请完成两步验证')
      return
    }
    
    ElMessage.success('登录成功')
    router.push(getPostLoginPath(response.user))
  } catch (error) {
    let message = extractErrorMessage(error) || '登录失败'
    const status = getErrorStatus(error)
    if (status === 404) {
      message = '账号不存在，请检查邮箱/用户名是否正确'
    } else if (status === 401) {
      message = '密码错误，请重新输入'
    } else if (status === 403) {
      message = extractErrorMessage(error) || '账号已被禁用，请联系管理员'
    } else if (status === 429) {
      message = '登录尝试过于频繁，请稍后再试'
    }
    ElMessage.error(message)
  } finally {
    loading.value = false
  }
}

// 处理 2FA 验证
async function handle2FAVerify() {
  if (!twoFAFormRef.value) return
  
  try {
    await twoFAFormRef.value.validate()
    loading.value = true
    
    const response = await userStore.completeTwoFactorLogin(
      {
        user_id: pendingUserId.value,
        code: twoFAForm.code.trim()
      },
      loginForm.remember
    )
    
    ElMessage.success('登录成功')
    router.push(getPostLoginPath(response.user))
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '验证失败')
  } finally {
    loading.value = false
  }
}

// 取消 2FA 验证
function cancelTwoFA() {
  show2FA.value = false
  pendingUserId.value = null
  twoFAForm.code = ''
  useBackupCode.value = false
}

onMounted(async () => {
  await loadOAuthProviders()
  const oauthError = route.query.oauth_error
  if (typeof oauthError === 'string' && oauthError) {
    ElMessage.error(oauthError)
    const query = { ...route.query }
    delete query.oauth_error
    router.replace({ path: route.path, query })
    return
  }

  const token = route.query.verify_email_token
  if (!token || typeof token !== 'string') {
    return
  }

  try {
    await verifyPortalEmail(token)
    ElMessage.success('邮箱验证成功，现在可以登录了')
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '邮箱验证失败')
  } finally {
    const query = { ...route.query }
    delete query.verify_email_token
    router.replace({ path: route.path, query })
  }
})
</script>

<style scoped>
.login-page {
  width: 100%;
}

.login-card {
  width: 100%;
  max-width: 360px;
  margin: 0 auto;
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.login-title {
  font-size: 28px;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.2;
  margin: 0 0 12px;
}

.login-subtitle {
  font-size: 14px;
  color: var(--color-text-secondary);
  margin: 0;
}

.login-form {
  margin-bottom: 24px;
}

.login-form :deep(.el-form-item) {
  margin-bottom: 20px;
}

.login-form :deep(.el-input__wrapper) {
  min-height: 48px;
  border-radius: 8px;
  padding: 0 14px;
  transition: box-shadow 0.18s ease, border-color 0.18s ease;
}

.login-form :deep(.el-input__prefix) {
  margin-right: 10px;
  color: var(--color-text-placeholder);
}

.login-form :deep(.el-input__inner) {
  min-width: 0;
  height: 48px;
  font-size: 15px;
  color: var(--color-text-primary);
}

.login-form :deep(.el-input__inner::placeholder) {
  color: var(--color-text-placeholder);
}

.form-options {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
  min-width: 0;
}

.form-options :deep(.el-checkbox) {
  min-width: 0;
}

.form-options :deep(.el-checkbox__label) {
  font-size: 14px;
}

.forgot-link {
  font-size: 14px;
  color: #409eff;
  text-decoration: none;
}

.forgot-link:hover {
  text-decoration: underline;
}

.login-btn {
  width: 100%;
  height: 48px;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 600;
}

.oauth-login {
  margin-bottom: 24px;
}

.oauth-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0 0 16px;
  color: var(--color-text-secondary);
  font-size: 13px;
}

.oauth-divider::before,
.oauth-divider::after {
  content: "";
  height: 1px;
  flex: 1;
  background: var(--el-border-color-lighter);
}

.oauth-buttons {
  display: grid;
  grid-template-columns: 1fr;
  gap: 10px;
}

.oauth-button {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  height: 46px;
  justify-content: center;
  padding: 0 16px;
  border-radius: 8px;
  border: 1px solid var(--el-border-color);
  background: var(--color-bg-card);
  color: var(--color-text-primary);
  font: inherit;
  cursor: pointer;
  transition: border-color 0.18s ease, background-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.oauth-button:hover:not(:disabled) {
  border-color: var(--el-color-primary-light-5);
  background: var(--color-bg-elevated-hover);
}

.oauth-button:focus-visible {
  outline: 3px solid color-mix(in srgb, var(--el-color-primary) 20%, transparent);
  outline-offset: 2px;
}

.oauth-button:disabled {
  cursor: wait;
  opacity: 0.72;
}

.oauth-button--wecom {
  height: 52px;
  justify-content: flex-start;
  padding: 0 14px;
}

.oauth-button--wecom:hover:not(:disabled) {
  border-color: color-mix(in srgb, #07c160 55%, var(--el-border-color));
  background: color-mix(in srgb, #07c160 4%, var(--color-bg-card));
}

.oauth-button:not(.oauth-button--wecom) .oauth-copy {
  flex: 0 1 auto;
  align-items: center;
}

.oauth-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  margin-right: 2px;
  border-radius: 50%;
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
  font-size: 13px;
  font-weight: 700;
}

.wecom-logo {
  flex: 0 0 30px;
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.oauth-copy {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  align-items: flex-start;
  gap: 0;
  text-align: left;
}

.oauth-copy strong {
  color: var(--color-text-primary);
  font-size: 15px;
  font-weight: 600;
}

.oauth-arrow {
  flex: 0 0 auto;
  color: #07c160;
  font-size: 16px;
}

.login-footer {
  text-align: center;
  font-size: 14px;
  color: var(--color-text-secondary);
}

.register-link {
  color: #409eff;
  text-decoration: none;
  margin-left: 4px;
}

.register-link:hover {
  text-decoration: underline;
}

/* 2FA 样式 */
.twofa-header {
  text-align: center;
  margin-bottom: 22px;
}

.twofa-icon {
  font-size: 44px;
  color: #409eff;
  margin-bottom: 14px;
}

.twofa-header h3 {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.twofa-header p {
  font-size: 14px;
  color: var(--color-text-secondary);
  margin: 0;
}

.twofa-input :deep(.el-input__inner) {
  text-align: center;
  font-size: 22px;
  letter-spacing: 6px;
}

.twofa-options {
  display: flex;
  justify-content: center;
  gap: 16px;
}

/* 响应式 */
@media (max-width: 480px) {
  .login-title {
    font-size: 26px;
  }

  .login-card {
    max-width: 100%;
  }

  .form-options {
    align-items: flex-start;
  }
}
</style>
