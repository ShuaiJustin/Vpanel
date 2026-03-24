<template>
  <div class="login-container">
    <div class="login-box">
      <div class="title">
        V 多协议代理面板
      </div>
      <el-form
        ref="loginFormRef"
        :model="loginForm"
        :rules="rules"
      >
        <el-form-item prop="username">
          <el-input 
            v-model="loginForm.username" 
            prefix-icon="User" 
            placeholder="用户名" 
            autocomplete="username"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input 
            v-model="loginForm.password" 
            prefix-icon="Lock" 
            type="password" 
            placeholder="密码" 
            autocomplete="current-password"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-form-item>
          <el-button 
            type="primary" 
            :loading="userStore.loading" 
            style="width: 100%" 
            @click="handleLogin"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>
      <div class="version">
        V 1.0.0
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { ElMessage } from 'element-plus'

const router = useRouter()
const userStore = useUserStore()
const loginFormRef = ref(null)

const loginForm = ref({
  username: '',
  password: ''
})

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '用户名长度应为3-20个字符', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于6个字符', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  if (!loginFormRef.value) return
  
  try {
    await loginFormRef.value.validate()
    
    await userStore.login(loginForm.value)
    ElMessage.success('登录成功')
    router.push('/')
  } catch (error) {
    // 只显示简洁的错误消息，不显示错误 ID
    const message = typeof error === 'string' ? error : (error?.message || '用户名或密码错误')
    ElMessage.error(message)
  }
}
</script>

<style scoped>
.login-container {
  height: 100vh;
  display: flex;
  justify-content: center;
  align-items: center;
  background-color: var(--color-bg-page);
  background-image: linear-gradient(135deg, var(--color-bg-page) 0%, var(--color-border-light) 100%);
}

.login-box {
  width: min(350px, calc(100vw - 32px));
  padding: 30px 20px;
  background-color: var(--color-bg-card);
  border-radius: 8px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--color-border);
  position: relative;
}

.title {
  font-size: 24px;
  color: var(--color-primary);
  text-align: center;
  margin-bottom: 30px;
  font-weight: bold;
}

.version {
  position: absolute;
  bottom: 10px;
  right: 10px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

:deep(.el-input__wrapper) {
  box-shadow: 0 0 0 1px var(--color-border) inset;
}

:deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--color-primary) inset;
}

:deep(.el-button) {
  font-weight: bold;
  letter-spacing: 1px;
}
</style> 
