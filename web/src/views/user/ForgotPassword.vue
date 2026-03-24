<template>
  <div class="forgot-page">
    <div class="forgot-card">
      <div class="forgot-header">
        <el-icon class="forgot-icon">
          <Key />
        </el-icon>
        <h1 class="forgot-title">
          忘记密码
        </h1>
        <p class="forgot-subtitle">
          输入您的邮箱地址，我们将发送密码重置链接
        </p>
      </div>

      <!-- 发送邮件表单 -->
      <el-form
        v-if="!emailSent"
        ref="forgotFormRef"
        :model="forgotForm"
        :rules="forgotRules"
        class="forgot-form"
        @submit.prevent="handleSubmit"
      >
        <el-form-item prop="email">
          <el-input
            v-model="forgotForm.email"
            placeholder="邮箱地址"
            size="large"
            :prefix-icon="Message"
            autocomplete="email"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            class="submit-btn"
            @click="handleSubmit"
          >
            发送重置链接
          </el-button>
        </el-form-item>
      </el-form>

      <!-- 发送成功提示 -->
      <div
        v-else
        class="success-message"
      >
        <el-icon class="success-icon">
          <CircleCheck />
        </el-icon>
        <h3>邮件已发送</h3>
        <p>
          我们已向 <strong>{{ forgotForm.email }}</strong> 发送了密码重置链接。
          请查收邮件并点击链接重置密码。
        </p>
        <p class="hint">
          如果没有收到邮件，请检查垃圾邮件文件夹，或者
          <el-button
            link
            type="primary"
            @click="resendEmail"
          >
            重新发送
          </el-button>
        </p>
      </div>

      <div class="forgot-footer">
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
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Key, Message, CircleCheck, ArrowLeft } from '@element-plus/icons-vue'
import { useUserPortalStore } from '@/stores/userPortal'

const userStore = useUserPortalStore()

// 表单引用
const forgotFormRef = ref(null)

// 状态
const loading = ref(false)
const emailSent = ref(false)

// 表单数据
const forgotForm = reactive({
  email: ''
})

// 表单验证规则
const forgotRules = {
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
  ]
}

// 提交表单
async function handleSubmit() {
  if (!forgotFormRef.value) return
  
  try {
    await forgotFormRef.value.validate()
    loading.value = true
    
    await userStore.forgotPassword(forgotForm.email)
    emailSent.value = true
    ElMessage.success('重置链接已发送')
  } catch (error) {
    const message = error.response?.data?.message || error.message || '发送失败'
    ElMessage.error(message)
  } finally {
    loading.value = false
  }
}

// 重新发送邮件
async function resendEmail() {
  loading.value = true
  try {
    await userStore.forgotPassword(forgotForm.email)
    ElMessage.success('重置链接已重新发送')
  } catch (error) {
    const message = error.response?.data?.message || error.message || '发送失败'
    ElMessage.error(message)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.forgot-page {
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.forgot-card {
  width: 100%;
  max-width: 400px;
  background: var(--color-bg-card);
  border-radius: 12px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);
  padding: 40px;
}

.forgot-header {
  text-align: center;
  margin-bottom: 32px;
}

.forgot-icon {
  font-size: 48px;
  color: #409eff;
  margin-bottom: 16px;
}

.forgot-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.forgot-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
  line-height: 1.6;
}

.forgot-form {
  margin-bottom: 24px;
}

.submit-btn {
  width: 100%;
  height: 44px;
  font-size: 16px;
}

/* 成功消息 */
.success-message {
  text-align: center;
  padding: 20px 0;
  margin-bottom: 24px;
}

.success-icon {
  font-size: 64px;
  color: #67c23a;
  margin-bottom: 16px;
}

.success-message h3 {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 12px 0;
}

.success-message p {
  font-size: 14px;
  color: #606266;
  margin: 0 0 8px 0;
  line-height: 1.6;
}

.success-message .hint {
  color: #909399;
  font-size: 13px;
}

.forgot-footer {
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
  .forgot-card {
    padding: 24px;
  }
  
  .forgot-title {
    font-size: 20px;
  }
}
</style>
