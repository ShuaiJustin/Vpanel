<template>
  <div class="register-page">
    <div class="register-card">
      <div class="register-header">
        <h1 class="register-title">
          创建账户
        </h1>
        <p class="register-subtitle">
          注册一个新账户开始使用
        </p>
      </div>

      <el-form
        ref="registerFormRef"
        :model="registerForm"
        :rules="registerRules"
        class="register-form"
        @submit.prevent="handleRegister"
      >
        <el-form-item prop="username">
          <el-input
            v-model="registerForm.username"
            placeholder="用户名"
            size="large"
            :prefix-icon="User"
            autocomplete="username"
          />
        </el-form-item>

        <el-form-item prop="email">
          <el-input
            v-model="registerForm.email"
            placeholder="邮箱地址"
            size="large"
            :prefix-icon="Message"
            autocomplete="email"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="registerForm.password"
            type="password"
            placeholder="密码"
            size="large"
            :prefix-icon="Lock"
            show-password
            autocomplete="new-password"
          />
        </el-form-item>

        <el-form-item prop="confirmPassword">
          <el-input
            v-model="registerForm.confirmPassword"
            type="password"
            placeholder="确认密码"
            size="large"
            :prefix-icon="Lock"
            show-password
            autocomplete="new-password"
          />
        </el-form-item>

        <el-form-item
          v-if="showInviteCode"
          prop="inviteCode"
        >
          <el-input
            v-model="registerForm.inviteCode"
            placeholder="邀请码（可选）"
            size="large"
            :prefix-icon="Ticket"
          />
        </el-form-item>

        <el-form-item
          prop="agreement"
          class="agreement-item"
        >
          <el-checkbox v-model="registerForm.agreement">
            <span class="agreement-text">
              我已阅读并同意
              <a
                href="#"
                class="agreement-link"
                @click.prevent="showTerms"
              >服务条款</a>
              和
              <a
                href="#"
                class="agreement-link"
                @click.prevent="showPrivacy"
              >隐私政策</a>
            </span>
          </el-checkbox>
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            class="register-btn"
            @click="handleRegister"
          >
            注册
          </el-button>
        </el-form-item>
      </el-form>

      <!-- 密码强度指示器 -->
      <div
        v-if="registerForm.password"
        class="password-strength"
      >
        <div class="strength-bar">
          <div 
            class="strength-fill" 
            :class="passwordStrengthClass"
            :style="{ width: passwordStrengthPercent + '%' }"
          />
        </div>
        <span
          class="strength-text"
          :class="passwordStrengthClass"
        >
          {{ passwordStrengthText }}
        </span>
      </div>

      <div class="register-footer">
        <span>已有账户？</span>
        <router-link
          to="/user/login"
          class="login-link"
        >
          立即登录
        </router-link>
      </div>
    </div>

    <!-- 注册成功对话框 -->
    <el-dialog
      v-model="showSuccessDialog"
      title="注册成功"
      width="400px"
      class="register-success-dialog"
      align-center
      :close-on-click-modal="false"
      :show-close="false"
    >
      <div class="success-content">
        <el-icon class="success-icon">
          <CircleCheck />
        </el-icon>
        <h3>账户创建成功！</h3>
        <p v-if="needEmailVerification">
          我们已向 <strong>{{ registerForm.email }}</strong> 发送了一封验证邮件，
          请查收并点击链接完成验证。
        </p>
        <p v-else>
          您的账户已创建成功，现在可以登录了。
        </p>
      </div>
      <template #footer>
        <el-button
          type="primary"
          @click="goToLogin"
        >
          前往登录
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock, Message, Ticket, CircleCheck } from '@element-plus/icons-vue'
import { useUserPortalStore } from '@/stores/userPortal'
import { extractErrorMessage } from '@/utils/entitlement'

const router = useRouter()
const route = useRoute()
const userStore = useUserPortalStore()

// 表单引用
const registerFormRef = ref(null)

// 状态
const loading = ref(false)
const showInviteCode = ref(true) // 可根据系统配置控制
const showSuccessDialog = ref(false)
const needEmailVerification = ref(true)

// 注册表单
const registerForm = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
  inviteCode: typeof route.query.ref === 'string' ? route.query.ref : '',
  agreement: false
})

// 密码强度计算
const passwordStrength = computed(() => {
  const password = registerForm.password
  if (!password) return 0
  
  let strength = 0
  
  // 长度检查
  if (password.length >= 8) strength += 1
  if (password.length >= 12) strength += 1
  
  // 包含数字
  if (/\d/.test(password)) strength += 1
  
  // 包含小写字母
  if (/[a-z]/.test(password)) strength += 1
  
  // 包含大写字母
  if (/[A-Z]/.test(password)) strength += 1
  
  // 包含特殊字符
  if (/[!@#$%^&*(),.?":{}|<>]/.test(password)) strength += 1
  
  return Math.min(strength, 4)
})

const passwordStrengthPercent = computed(() => {
  return (passwordStrength.value / 4) * 100
})

const passwordStrengthClass = computed(() => {
  const strength = passwordStrength.value
  if (strength <= 1) return 'weak'
  if (strength <= 2) return 'fair'
  if (strength <= 3) return 'good'
  return 'strong'
})

const passwordStrengthText = computed(() => {
  const strength = passwordStrength.value
  if (strength <= 1) return '弱'
  if (strength <= 2) return '一般'
  if (strength <= 3) return '良好'
  return '强'
})

// 验证确认密码
const validateConfirmPassword = (rule, value, callback) => {
  if (value !== registerForm.password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

// 验证协议
const validateAgreement = (rule, value, callback) => {
  if (!value) {
    callback(new Error('请阅读并同意服务条款'))
  } else {
    callback()
  }
}

// 注册表单验证规则
const registerRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 50, message: '用户名长度应为 3-50 个字符', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]+$/, message: '用户名只能包含字母、数字和下划线', trigger: 'blur' }
  ],
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码长度不能少于 8 个字符', trigger: 'blur' },
    { pattern: /^(?=.*[A-Za-z])(?=.*\d)/, message: '密码必须包含字母和数字', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ],
  agreement: [
    { validator: validateAgreement, trigger: 'change' }
  ]
}

// 处理注册
async function handleRegister() {
  if (!registerFormRef.value) return
  
  try {
    await registerFormRef.value.validate()
    loading.value = true
    
    const response = await userStore.register({
      username: registerForm.username.trim(),
      email: registerForm.email.trim().toLowerCase(),
      password: registerForm.password,
      invite_code: registerForm.inviteCode?.trim() || undefined
    })
    
    needEmailVerification.value = response.need_email_verification !== false
    showSuccessDialog.value = true
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '注册失败')
  } finally {
    loading.value = false
  }
}

// 前往登录
function goToLogin() {
  showSuccessDialog.value = false
  router.push('/user/login')
}

// 显示服务条款
function showTerms() {
  router.push('/user/terms')
}

// 显示隐私政策
function showPrivacy() {
  router.push('/user/privacy')
}
</script>

<style scoped>
.register-page {
  width: 100%;
}

.register-card {
  width: 100%;
  max-width: 380px;
  margin: 0 auto;
}

.register-header {
  text-align: center;
  margin-bottom: 26px;
}

.register-title {
  font-size: 30px;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.2;
  margin: 0 0 10px;
}

.register-subtitle {
  font-size: 15px;
  color: #909399;
  margin: 0;
}

.register-form {
  margin-bottom: 14px;
}

.register-form :deep(.el-form-item) {
  margin-bottom: 14px;
}

.register-form :deep(.el-input__wrapper) {
  min-height: 46px;
  border-radius: 8px;
  padding: 0 14px;
  transition: box-shadow 0.18s ease, border-color 0.18s ease;
}

.register-form :deep(.el-input__prefix) {
  margin-right: 10px;
  color: #a8abb2;
}

.register-form :deep(.el-input__inner) {
  min-width: 0;
  height: 46px;
  font-size: 15px;
  color: var(--color-text-primary);
}

.register-form :deep(.el-input__inner::placeholder) {
  color: #a8abb2;
}

.register-btn {
  width: 100%;
  height: 48px;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 600;
}

.agreement-item {
  margin-bottom: 18px;
}

.agreement-item :deep(.el-form-item__content) {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  line-height: normal;
}

.agreement-item :deep(.el-checkbox) {
  display: flex;
  align-items: flex-start;
  align-self: flex-start;
  max-width: 100%;
  white-space: normal;
}

.agreement-item :deep(.el-checkbox__label) {
  white-space: normal;
  line-height: 1.45;
  word-break: break-word;
}

.agreement-text {
  display: inline;
  line-height: 1.45;
  font-size: 14px;
}

.agreement-item :deep(.el-form-item__error) {
  position: static;
  margin-top: 8px;
  line-height: 1.4;
}

.agreement-link {
  color: #409eff;
  text-decoration: none;
}

.agreement-link:hover {
  text-decoration: underline;
}

/* 协议勾选框样式 */
:deep(.el-form-item) {
  .el-checkbox {
    white-space: normal;
    line-height: 1.5;
  }
  
  .el-checkbox__label {
    white-space: normal;
    line-height: 1.5;
  }
}

/* 密码强度指示器 */
.password-strength {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
  padding: 0 2px;
}

.strength-bar {
  flex: 1;
  height: 4px;
  background: #ebeef5;
  border-radius: 2px;
  overflow: hidden;
}

.strength-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.3s, background-color 0.3s;
}

.strength-fill.weak {
  background-color: #f56c6c;
}

.strength-fill.fair {
  background-color: #e6a23c;
}

.strength-fill.good {
  background-color: #409eff;
}

.strength-fill.strong {
  background-color: #67c23a;
}

.strength-text {
  font-size: 12px;
  min-width: 32px;
}

.strength-text.weak {
  color: #f56c6c;
}

.strength-text.fair {
  color: #e6a23c;
}

.strength-text.good {
  color: #409eff;
}

.strength-text.strong {
  color: #67c23a;
}

.register-footer {
  text-align: center;
  font-size: 14px;
  color: #909399;
}

.login-link {
  color: #409eff;
  text-decoration: none;
  margin-left: 4px;
}

.login-link:hover {
  text-decoration: underline;
}

/* 成功对话框 */
.success-content {
  text-align: center;
  padding: 16px 0 12px;
}

.success-icon {
  font-size: 64px;
  color: #67c23a;
  margin-bottom: 16px;
}

.success-content h3 {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 12px 0;
}

.success-content p {
  font-size: 14px;
  color: #606266;
  margin: 0;
  line-height: 1.6;
  word-break: break-word;
}

:global(.register-success-dialog) {
  max-width: calc(100vw - 32px);
  border-radius: 14px;
}

:global(.register-success-dialog .el-dialog__header) {
  padding-bottom: 10px;
}

:global(.register-success-dialog .el-dialog__footer) {
  padding-top: 0;
}

:global(.register-success-dialog .el-dialog__footer .el-button) {
  min-width: 128px;
}

/* 响应式 */
@media (max-width: 480px) {
  .register-title {
    font-size: 26px;
  }

  .register-card {
    max-width: 100%;
  }

  .register-form :deep(.el-form-item) {
    margin-bottom: 12px;
  }

  .agreement-text {
    font-size: 13px;
  }

  .success-content {
    padding: 8px 0 6px;
  }

  .success-icon {
    font-size: 48px;
    margin-bottom: 12px;
  }

  .success-content h3 {
    font-size: 18px;
    margin-bottom: 10px;
  }

  .success-content p {
    font-size: 13px;
    line-height: 1.55;
  }

  :global(.register-success-dialog) {
    width: calc(100vw - 32px) !important;
    max-width: 360px;
  }

  :global(.register-success-dialog .el-dialog__header) {
    padding: 18px 18px 8px;
  }

  :global(.register-success-dialog .el-dialog__title) {
    font-size: 18px;
    line-height: 1.35;
  }

  :global(.register-success-dialog .el-dialog__body) {
    padding: 8px 22px 18px;
    max-height: none;
  }

  :global(.register-success-dialog .el-dialog__footer) {
    padding: 0 22px 22px;
  }

  :global(.register-success-dialog .el-dialog__footer .el-button) {
    width: 100%;
    min-height: 42px;
    margin: 0;
  }
}
</style>
