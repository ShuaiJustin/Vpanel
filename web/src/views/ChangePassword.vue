<template>
  <div class="change-password-container">
    <div class="change-password-header">
      <h2>修改密码</h2>
      <p>修改您的登录密码</p>
    </div>
    
    <div class="change-password-content">
      <el-card class="change-password-card">
        <el-form 
          :model="passwordForm" 
          label-width="120px" 
          class="password-form"
          :rules="rules"
          ref="passwordFormRef"
        >
          <el-form-item label="当前密码" prop="currentPassword">
            <el-input 
              v-model="passwordForm.currentPassword" 
              type="password" 
              placeholder="请输入当前密码"
              show-password
            ></el-input>
          </el-form-item>
          
          <el-form-item label="新密码" prop="newPassword">
            <el-input 
              v-model="passwordForm.newPassword" 
              type="password" 
              placeholder="请输入新密码"
              show-password
            ></el-input>
            <div class="password-strength" v-if="passwordForm.newPassword">
              <span>密码强度:</span>
              <div class="strength-indicator">
                <div 
                  class="strength-bar" 
                  :class="passwordStrengthClass"
                  :style="{ width: passwordStrength + '%' }"
                ></div>
              </div>
              <span class="strength-text" :class="passwordStrengthClass">{{ passwordStrengthText }}</span>
            </div>
          </el-form-item>
          
          <el-form-item label="确认新密码" prop="confirmPassword">
            <el-input 
              v-model="passwordForm.confirmPassword" 
              type="password" 
              placeholder="请再次输入新密码"
              show-password
            ></el-input>
          </el-form-item>
          
          <el-form-item>
            <el-button type="primary" :loading="saving" @click="changePassword">确认修改</el-button>
            <el-button :disabled="saving" @click="resetForm">重置</el-button>
          </el-form-item>
        </el-form>
        
        <div class="password-tips">
          <h4>密码要求:</h4>
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

const router = useRouter()
const userStore = useUserStore()
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
    userStore.logout()
    router.replace('/admin/login')
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

.change-password-header {
  margin-bottom: 20px;
}

.change-password-header h2 {
  font-size: 24px;
  margin: 0 0 8px;
  color: #303133;
}

.change-password-header p {
  margin: 0;
  color: #909399;
  font-size: 14px;
}

.change-password-content {
  display: flex;
  justify-content: center;
}

.change-password-card {
  width: 100%;
  max-width: 700px;
}

.password-form {
  max-width: 500px;
  margin: 0 auto 20px;
}

.password-strength {
  margin-top: 8px;
  display: flex;
  align-items: center;
  font-size: 12px;
}

.strength-indicator {
  width: 100px;
  height: 5px;
  background-color: #e0e0e0;
  margin: 0 8px;
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

.password-tips {
  border-top: 1px solid #ebeef5;
  padding-top: 15px;
}

.password-tips h4 {
  font-size: 14px;
  color: #606266;
  margin-top: 0;
  margin-bottom: 10px;
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
</style>
