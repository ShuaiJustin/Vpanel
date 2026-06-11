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

      <!-- 2FA 验证表单 -->
      <el-form
        v-else
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
import { User, Lock, Key } from '@element-plus/icons-vue'
import { useUserPortalStore } from '@/stores/userPortal'
import { verifyEmail as verifyPortalEmail } from '@/api/modules/portal/auth'
import { extractErrorMessage, getErrorStatus } from '@/utils/entitlement'

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
  font-size: 15px;
  color: #909399;
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
  color: #a8abb2;
}

.login-form :deep(.el-input__inner) {
  min-width: 0;
  height: 48px;
  font-size: 15px;
  color: var(--color-text-primary);
}

.login-form :deep(.el-input__inner::placeholder) {
  color: #a8abb2;
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

.login-footer {
  text-align: center;
  font-size: 14px;
  color: #909399;
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
  color: #909399;
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
