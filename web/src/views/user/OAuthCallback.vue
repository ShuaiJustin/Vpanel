<template>
  <div class="oauth-callback-page">
    <div class="oauth-callback-card">
      <el-icon class="loading-icon">
        <Loading />
      </el-icon>
      <h1>正在完成登录</h1>
      <p>请稍候，正在同步第三方账号会话。</p>
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { useUserPortalStore } from '@/stores/userPortal'

const route = useRoute()
const router = useRouter()
const userStore = useUserPortalStore()

function decodeBase64Url(value) {
  const normalized = String(value || '').replace(/-/g, '+').replace(/_/g, '/')
  const padded = normalized.padEnd(normalized.length + ((4 - normalized.length % 4) % 4), '=')
  const binary = window.atob(padded)
  const bytes = Uint8Array.from(binary, char => char.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}

function safeRedirect(value) {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) {
    return ''
  }
  return value
}

function postLoginPath(user, redirect) {
  const target = safeRedirect(redirect)
  if (user?.role === 'admin') {
    return target.startsWith('/admin') ? target : userStore.adminEntryPath
  }
  return target && !target.startsWith('/admin') ? target : '/user/dashboard'
}

onMounted(() => {
  try {
    const params = new URLSearchParams(route.hash.replace(/^#/, ''))
    const token = params.get('token')
    const userPayload = params.get('user')
    const redirect = params.get('redirect') || ''
    if (!token || !userPayload) {
      throw new Error('missing oauth payload')
    }
    const user = JSON.parse(decodeBase64Url(userPayload))
    userStore.completeOAuthLogin({ token, user }, false)
    ElMessage.success('登录成功')
    router.replace(postLoginPath(user, redirect))
  } catch {
    ElMessage.error('第三方登录失败，请重新尝试')
    router.replace('/user/login')
  }
})
</script>

<style scoped>
.oauth-callback-page {
  width: 100%;
}

.oauth-callback-card {
  width: 100%;
  max-width: 360px;
  margin: 0 auto;
  text-align: center;
}

.loading-icon {
  margin-bottom: 16px;
  color: #409eff;
  font-size: 42px;
  animation: spin 1s linear infinite;
}

.oauth-callback-card h1 {
  margin: 0 0 10px;
  color: var(--color-text-primary);
  font-size: 24px;
  font-weight: 600;
}

.oauth-callback-card p {
  margin: 0;
  color: #909399;
  font-size: 14px;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
