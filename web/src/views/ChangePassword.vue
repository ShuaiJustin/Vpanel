<template>
  <div class="change-password-container">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          修改密码
        </h1>
        <p class="page-subtitle">
          更新登录凭据并完成基础安全校验
        </p>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">当前状态</span>
        <strong
          class="overview-value"
          :class="passwordForm.newPassword ? 'is-primary' : ''"
        >
          {{ passwordForm.newPassword ? passwordStrengthText : '待输入' }}
        </strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">最少长度</span>
        <strong class="overview-value">8 位</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">安全校验</span>
        <strong class="overview-value is-success">5 项</strong>
      </div>
    </div>

    <div class="change-password-layout">
      <el-card
        class="change-password-card"
        shadow="never"
      >
        <template #header>
          <div class="card-header">
            <span>更新登录密码</span>
            <span class="card-hint">保存后将重新登录</span>
          </div>
        </template>

        <el-form
          ref="passwordFormRef"
          :model="passwordForm"
          :label-width="isMobile ? 'auto' : '110px'"
          :label-position="isMobile ? 'top' : 'left'"
          class="password-form"
          :rules="rules"
        >
          <el-form-item
            label="当前密码"
            prop="currentPassword"
          >
            <el-input
              v-model="passwordForm.currentPassword"
              type="password"
              placeholder="请输入当前密码"
              show-password
            />
          </el-form-item>

          <el-form-item
            label="新密码"
            prop="newPassword"
          >
            <el-input
              v-model="passwordForm.newPassword"
              type="password"
              placeholder="请输入新密码"
              show-password
            />
            <div
              v-if="passwordForm.newPassword"
              class="password-strength"
            >
              <span class="strength-label">密码强度</span>
              <div class="strength-indicator">
                <div
                  class="strength-bar"
                  :class="passwordStrengthClass"
                  :style="{ width: passwordStrength + '%' }"
                />
              </div>
              <span
                class="strength-text"
                :class="passwordStrengthClass"
              >{{ passwordStrengthText }}</span>
            </div>
          </el-form-item>

          <el-form-item
            label="确认新密码"
            prop="confirmPassword"
          >
            <el-input
              v-model="passwordForm.confirmPassword"
              type="password"
              placeholder="请再次输入新密码"
              show-password
            />
          </el-form-item>

          <el-form-item>
            <div class="form-actions">
              <el-button
                type="primary"
                :loading="saving"
                @click="changePassword"
              >
                确认修改
              </el-button>
              <el-button
                :disabled="saving"
                @click="resetForm"
              >
                重置
              </el-button>
            </div>
          </el-form-item>
        </el-form>
      </el-card>

      <el-card
        class="password-tips-card"
        shadow="never"
      >
        <template #header>
          <div class="card-header">
            <span>密码要求</span>
            <span class="card-hint">建议定期更新</span>
          </div>
        </template>

        <div class="password-tips">
          <ul>
            <li>密码长度至少为8个字符</li>
            <li>包含至少一个大写字母和一个小写字母</li>
            <li>包含至少一个数字</li>
            <li>包含至少一个特殊字符 (!@#$%^&*)</li>
            <li>不能与当前密码相同</li>
          </ul>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/stores/user'
import { useViewport } from '@/composables/useViewport'

const router = useRouter()
const userStore = useUserStore()
const { isMobile } = useViewport()
const passwordFormRef = ref(null)
const saving = ref(false)

const passwordForm = ref({
  currentPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const validatePass = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请输入密码'))
  } else {
    if (value === passwordForm.value.currentPassword) {
      callback(new Error('新密码不能与当前密码相同'))
      return
    }

    // 密码强度验证
    const hasUpperCase = /[A-Z]/.test(value)
    const hasLowerCase = /[a-z]/.test(value)
    const hasNumber = /[0-9]/.test(value)
    const hasSpecialChar = /[!@#$%^&*]/.test(value)
    const isLongEnough = value.length >= 8
    
    if (!isLongEnough) {
      callback(new Error('密码长度至少为8个字符'))
    } else if (!hasUpperCase) {
      callback(new Error('密码必须包含至少一个大写字母'))
    } else if (!hasLowerCase) {
      callback(new Error('密码必须包含至少一个小写字母'))
    } else if (!hasNumber) {
      callback(new Error('密码必须包含至少一个数字'))
    } else if (!hasSpecialChar) {
      callback(new Error('密码必须包含至少一个特殊字符 (!@#$%^&*)'))
    } else if (passwordForm.value.confirmPassword !== '') {
      passwordFormRef.value.validateField('confirmPassword')
      callback()
    } else {
      callback()
    }
  }
}

const validateConfirmPass = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请再次输入密码'))
  } else if (value !== passwordForm.value.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  currentPassword: [
    { required: true, message: '请输入当前密码', trigger: 'blur' },
    { min: 6, message: '密码长度至少为6个字符', trigger: 'blur' }
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { validator: validatePass, trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: validateConfirmPass, trigger: 'blur' }
  ]
}

// 计算密码强度
const passwordStrength = computed(() => {
  const password = passwordForm.value.newPassword
  if (!password) return 0
  
  let strength = 0
  // 长度检查
  if (password.length >= 8) strength += 25
  // 是否包含大写字母
  if (/[A-Z]/.test(password)) strength += 25
  // 是否包含小写字母和数字
  if (/[a-z]/.test(password) && /[0-9]/.test(password)) strength += 25
  // 是否包含特殊字符
  if (/[!@#$%^&*]/.test(password)) strength += 25
  
  return strength
})

// 密码强度文本
const passwordStrengthText = computed(() => {
  const strength = passwordStrength.value
  if (strength < 25) return '弱'
  if (strength < 50) return '中'
  if (strength < 75) return '强'
  return '非常强'
})

// 密码强度样式类
const passwordStrengthClass = computed(() => {
  const strength = passwordStrength.value
  if (strength < 25) return 'weak'
  if (strength < 50) return 'medium'
  if (strength < 75) return 'strong'
  return 'very-strong'
})

const changePassword = async () => {
  if (!passwordFormRef.value) return

  const valid = await passwordFormRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    await userStore.changePassword({
      currentPassword: passwordForm.value.currentPassword,
      newPassword: passwordForm.value.newPassword
    })

    ElMessage.success('密码修改成功，请重新登录')
    resetForm()
    await userStore.logout()
    router.replace('/user/login')
  } catch (error) {
    ElMessage.error(typeof error === 'string' ? error : '修改密码失败，请检查当前密码是否正确')
  } finally {
    saving.value = false
  }
}

const resetForm = () => {
  if (passwordFormRef.value) {
    passwordFormRef.value.resetFields()
  }
}
</script>

<style scoped>
.change-password-container {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.page-title {
  font-size: 24px;
  margin: 0 0 8px;
  color: #303133;
}

.page-subtitle {
  margin: 0;
  color: #909399;
  font-size: 14px;
}

.overview-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.overview-card {
  padding: 18px 20px;
  border-radius: 16px;
  background: linear-gradient(135deg, #ffffff 0%, #f8fafc 100%);
  border: 1px solid #e5e7eb;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.05);
}

.overview-label {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  color: #64748b;
}

.overview-value {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
}

.overview-value.is-primary {
  color: #2563eb;
}

.overview-value.is-success {
  color: #059669;
}

.change-password-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.8fr) minmax(260px, 1fr);
  gap: 20px;
  align-items: start;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.card-hint {
  font-size: 12px;
  color: #94a3b8;
}

.change-password-card,
.password-tips-card {
  border-radius: 18px;
}

.password-form {
  margin-bottom: 0;
}

.password-strength {
  margin-top: 8px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
}

.strength-label {
  color: #64748b;
}

.strength-indicator {
  flex: 1 1 100px;
  height: 5px;
  background-color: #e0e0e0;
  border-radius: 3px;
  overflow: hidden;
}

.strength-bar {
  height: 100%;
  transition: width 0.3s, background-color 0.3s;
}

.strength-bar.weak {
  background-color: #f56c6c;
}

.strength-bar.medium {
  background-color: #e6a23c;
}

.strength-bar.strong {
  background-color: #67c23a;
}

.strength-bar.very-strong {
  background-color: #409eff;
}

.strength-text {
  font-weight: bold;
}

.strength-text.weak {
  color: #f56c6c;
}

.strength-text.medium {
  color: #e6a23c;
}

.strength-text.strong {
  color: #67c23a;
}

.strength-text.very-strong {
  color: #409eff;
}

.form-actions {
  display: flex;
  gap: 12px;
}

.password-tips {
  color: #475569;
}

.password-tips ul {
  padding-left: 20px;
  margin: 0;
}

.password-tips li {
  font-size: 12px;
  color: #909399;
  line-height: 1.8;
}

@media (max-width: 1024px) {
  .change-password-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .change-password-container {
    padding: 12px;
  }

  .overview-strip {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .form-actions {
    width: 100%;
    flex-direction: column;
  }

  .form-actions .el-button {
    width: 100%;
    margin-left: 0;
  }

  .password-strength {
    align-items: stretch;
  }
}
</style>
