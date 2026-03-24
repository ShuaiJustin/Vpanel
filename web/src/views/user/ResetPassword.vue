<template>
  <div class="reset-page">
    <div class="reset-card">
      <!-- 加载状态 -->
      <div
        v-if="verifying"
        class="verifying-state"
      >
        <el-icon class="loading-icon">
          <Loading />
        </el-icon>
        <p>正在验证重置链接...</p>
      </div>

      <!-- 令牌无效 -->
      <div
        v-else-if="tokenInvalid"
        class="invalid-state"
      >
        <el-icon class="error-icon">
          <CircleClose />
        </el-icon>
        <h3>链接无效或已过期</h3>
        <p>该密码重置链接已失效，请重新申请。</p>
        <el-button
          type="primary"
          @click="goToForgotPassword"
        >
          重新申请
        </el-button>
      </div>

      <!-- 重置成功 -->
      <div
        v-else-if="resetSuccess"
        class="success-state"
      >
        <el-icon class="success-icon">
          <CircleCheck />
        </el-icon>
        <h3>密码重置成功</h3>
        <p>您的密码已成功重置，现在可以使用新密码登录了。</p>
        <el-button
          type="primary"
          @click="goToLogin"
        >
          前往登录
        </el-button>
      </div>

      <!-- 重置表单 -->
      <template v-else>
        <div class="reset-header">
          <el-icon class="reset-icon">
            <Key />
          </el-icon>
          <h1 class="reset-title">
            重置密码
          </h1>
          <p class="reset-subtitle">
            请输入您的新密码
          </p>
        </div>

        <el-form
          ref="resetFormRef"
          :model="resetForm"
          :rules="resetRules"
          class="reset-form"
          @submit.prevent="handleReset"
        >
          <el-form-item prop="password">
            <el-input
              v-model="resetForm.password"
              type="password"
              placeholder="新密码"
              size="large"
              :prefix-icon="Lock"
              show-password
              autocomplete="new-password"
            />
          </el-form-item>

          <el-form-item prop="confirmPassword">
            <el-input
              v-model="resetForm.confirmPassword"
              type="password"
              placeholder="确认新密码"
              size="large"
              :prefix-icon="Lock"
              show-password
              autocomplete="new-password"
              @keyup.enter="handleReset"
            />
          </el-form-item>

          <!-- 密码强度指示器 -->
          <div
            v-if="resetForm.password"
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

          <el-form-item>
            <el-button
              type="primary"
              size="large"
              :loading="loading"
              class="reset-btn"
              @click="handleReset"
            >
              重置密码
            </el-button>
          </el-form-item>
        </el-form>
      </template>

      <div
        v-if="!verifying && !tokenInvalid && !resetSuccess"
        class="reset-footer"
      >
        <router-link
          to="/user/login"
          class="back-link"
        >
          <el-icon><ArrowLeft /></el-icon>
          返回登录
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Key, Lock, CircleCheck, CircleClose, ArrowLeft, Loading } from '@element-plus/icons-vue'
import { useUserPortalStore } from '@/stores/userPortal'

const router = useRouter()
const route = useRoute()
const userStore = useUserPortalStore()

// 表单引用
const resetFormRef = ref(null)

// 状态
const loading = ref(false)
const verifying = ref(true)
const tokenInvalid = ref(false)
const resetSuccess = ref(false)
const token = ref('')

// 表单数据
const resetForm = reactive({
  password: '',
  confirmPassword: ''
})

// 密码强度计算
const passwordStrength = computed(() => {
  const password = resetForm.password
  if (!password) return 0
  
  let strength = 0
  if (password.length >= 8) strength += 1
  if (password.length >= 12) strength += 1
  if (/\d/.test(password)) strength += 1
  if (/[a-z]/.test(password)) strength += 1
  if (/[A-Z]/.test(password)) strength += 1
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
  if (value !== resetForm.password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

// 表单验证规则
const resetRules = {
  password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 8, message: '密码长度不能少于 8 个字符', trigger: 'blur' },
    { pattern: /^(?=.*[A-Za-z])(?=.*\d)/, message: '密码必须包含字母和数字', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

// 验证令牌
async function verifyToken() {
  token.value = route.query.token
  
  if (!token.value) {
    tokenInvalid.value = true
    verifying.value = false
    return
  }
  
  // 简单验证令牌格式
  // 实际验证会在提交时由后端完成
  if (token.value.length < 32) {
    tokenInvalid.value = true
  }
  
  verifying.value = false
}

// 处理重置
async function handleReset() {
  if (!resetFormRef.value) return
  
  try {
    await resetFormRef.value.validate()
    loading.value = true
    
    await userStore.resetPassword({
      token: token.value,
      password: resetForm.password
    })
    
    resetSuccess.value = true
    ElMessage.success('密码重置成功')
  } catch (error) {
    const message = error.response?.data?.message || error.message || '重置失败'
    
    // 检查是否是令牌无效错误
    if (error.response?.data?.code === 'INVALID_TOKEN' || 
        error.response?.data?.code === 'TOKEN_USED') {
      tokenInvalid.value = true
    } else {
      ElMessage.error(message)
    }
  } finally {
    loading.value = false
  }
}

// 前往登录
function goToLogin() {
  router.push('/user/login')
}

// 前往忘记密码页面
function goToForgotPassword() {
  router.push('/user/forgot-password')
}

// 组件挂载时验证令牌
onMounted(() => {
  verifyToken()
})
</script>

<style scoped>
.reset-page {
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.reset-card {
  width: 100%;
  max-width: 400px;
  background: var(--color-bg-card);
  border-radius: 12px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);
  padding: 40px;
}

/* 验证状态 */
.verifying-state {
  text-align: center;
  padding: 40px 0;
}

.loading-icon {
  font-size: 48px;
  color: #409eff;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.verifying-state p {
  margin-top: 16px;
  color: #909399;
}

/* 无效状态 */
.invalid-state,
.success-state {
  text-align: center;
  padding: 20px 0;
}

.error-icon {
  font-size: 64px;
  color: #f56c6c;
  margin-bottom: 16px;
}

.success-icon {
  font-size: 64px;
  color: #67c23a;
  margin-bottom: 16px;
}

.invalid-state h3,
.success-state h3 {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 12px 0;
}

.invalid-state p,
.success-state p {
  font-size: 14px;
  color: #606266;
  margin: 0 0 24px 0;
  line-height: 1.6;
}

/* 重置表单 */
.reset-header {
  text-align: center;
  margin-bottom: 32px;
}

.reset-icon {
  font-size: 48px;
  color: #409eff;
  margin-bottom: 16px;
}

.reset-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.reset-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

.reset-form {
  margin-bottom: 24px;
}

.reset-btn {
  width: 100%;
  height: 44px;
  font-size: 16px;
}

/* 密码强度指示器 */
.password-strength {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
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

.strength-fill.weak { background-color: #f56c6c; }
.strength-fill.fair { background-color: #e6a23c; }
.strength-fill.good { background-color: #409eff; }
.strength-fill.strong { background-color: #67c23a; }

.strength-text {
  font-size: 12px;
  min-width: 32px;
}

.strength-text.weak { color: #f56c6c; }
.strength-text.fair { color: #e6a23c; }
.strength-text.good { color: #409eff; }
.strength-text.strong { color: #67c23a; }

.reset-footer {
  text-align: center;
}

.back-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 14px;
  color: #409eff;
  text-decoration: none;
}

.back-link:hover {
  text-decoration: underline;
}

/* 响应式 */
@media (max-width: 480px) {
  .reset-card {
    padding: 24px;
  }
  
  .reset-title {
    font-size: 20px;
  }
}
</style>
