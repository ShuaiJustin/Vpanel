<template>
  <div class="profile-container">
    <div class="profile-header">
      <h2>个人资料</h2>
      <p>管理当前管理员账号的基础信息</p>
    </div>

    <div class="profile-content">
      <el-card class="profile-card">
        <div class="avatar-section">
          <el-avatar :size="88" class="avatar">{{ avatarText }}</el-avatar>
          <p class="avatar-hint">头像当前根据用户名首字母生成。</p>
        </div>

        <el-form
          ref="profileForm"
          :model="userForm"
          :rules="rules"
          label-width="100px"
          class="profile-form"
        >
          <el-form-item label="用户名">
            <el-input v-model="userForm.username" disabled />
          </el-form-item>

          <el-form-item label="邮箱" prop="email">
            <el-input v-model="userForm.email" placeholder="请输入邮箱" clearable />
          </el-form-item>

          <el-form-item label="角色">
            <el-input :value="roleLabel" disabled />
          </el-form-item>

          <el-form-item label="上次登录">
            <div>{{ formatDate(userForm.lastLogin) }}</div>
          </el-form-item>

          <el-form-item label="注册时间">
            <div>{{ formatDate(userForm.createdAt) }}</div>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="saving" @click="saveProfile">保存修改</el-button>
            <el-button :disabled="saving" @click="resetForm">重置</el-button>
          </el-form-item>
        </el-form>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()
const profileForm = ref(null)
const saving = ref(false)
const initialForm = ref({
  username: '',
  email: '',
  role: '',
  lastLogin: '',
  createdAt: ''
})
const userForm = ref({
  username: '',
  email: '',
  role: '',
  lastLogin: '',
  createdAt: ''
})

const avatarText = computed(() => {
  const username = userForm.value.username || 'A'
  return username.charAt(0).toUpperCase()
})

const roleLabel = computed(() => {
  return userForm.value.role === 'admin' ? '管理员' : userForm.value.role || '--'
})

const rules = {
  email: [
    { type: 'email', message: '请输入正确的邮箱地址', trigger: 'blur' }
  ]
}

const formatDate = (date) => {
  if (!date) return '--'
  const value = new Date(date)
  if (Number.isNaN(value.getTime())) return '--'
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')} ${String(value.getHours()).padStart(2, '0')}:${String(value.getMinutes()).padStart(2, '0')}`
}

const syncForm = (profile) => {
  const normalized = {
    username: profile?.username || '',
    email: profile?.email || '',
    role: profile?.role || '',
    lastLogin: profile?.last_login || profile?.lastLogin || '',
    createdAt: profile?.created_at || profile?.createdAt || ''
  }
  initialForm.value = { ...normalized }
  userForm.value = { ...normalized }
}

const loadProfile = async () => {
  const profile = await userStore.getUser()
  syncForm(profile)
}

const saveProfile = async () => {
  if (!profileForm.value) return

  const valid = await profileForm.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    const updatedUser = await userStore.updateUserProfile({
      email: userForm.value.email.trim()
    })
    syncForm(updatedUser)
    ElMessage.success('个人资料已更新')
  } catch (error) {
    ElMessage.error(typeof error === 'string' ? error : '更新个人资料失败')
  } finally {
    saving.value = false
  }
}

const resetForm = () => {
  userForm.value = { ...initialForm.value }
  profileForm.value?.clearValidate()
}

onMounted(async () => {
  try {
    await loadProfile()
  } catch (error) {
    ElMessage.error(typeof error === 'string' ? error : '获取用户资料失败')
  }
})
</script>

<style scoped>
.profile-container {
  padding: 20px;
}

.profile-header {
  margin-bottom: 20px;
}

.profile-header h2 {
  margin: 0 0 8px;
  font-size: 24px;
  color: #303133;
}

.profile-header p {
  margin: 0;
  color: #909399;
  font-size: 14px;
}

.profile-content {
  display: flex;
  justify-content: center;
}

.profile-card {
  width: 100%;
  max-width: 720px;
}

.avatar-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin-bottom: 30px;
}

.avatar {
  background-color: #3b82f6;
  font-size: 34px;
  font-weight: 600;
}

.avatar-hint {
  margin: 0;
  color: #909399;
  font-size: 13px;
}

.profile-form {
  max-width: 520px;
  margin: 0 auto;
}

@media (max-width: 768px) {
  .profile-container {
    padding: 12px;
  }

  .profile-card {
    border-radius: 16px;
  }

  .profile-form {
    max-width: none;
  }
}
</style>
