<template>
  <div class="users-container">
    <div class="page-header">
      <h1>用户管理</h1>
      <div class="actions">
        <el-button type="primary" @click="showAddDialog">添加用户</el-button>
        <el-input
          v-model="searchQuery"
          placeholder="搜索用户名或邮箱"
          clearable
          style="width: 260px"
          @input="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>
    </div>

    <el-table :data="paginatedUsers" border v-loading="loading" style="width: 100%">
      <el-table-column prop="username" label="用户名" min-width="140" />
      <el-table-column prop="email" label="邮箱" min-width="220" />
      <el-table-column prop="role" label="角色" width="100">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'primary'">
            {{ row.role === 'admin' ? '管理员' : '普通用户' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created" label="创建时间" width="180" />
      <el-table-column prop="lastLogin" label="最后登录" width="180" />
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status ? 'success' : 'danger'">
            {{ row.status ? '启用' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="320" fixed="right">
        <template #default="{ row }">
          <el-space wrap>
            <el-button size="small" type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button size="small" @click="handleResetPassword(row)">重置密码</el-button>
            <el-button
              size="small"
              :type="row.status ? 'warning' : 'success'"
              @click="handleToggleStatus(row)"
            >
              {{ row.status ? '禁用' : '启用' }}
            </el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </el-space>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        :total="displayTotal"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>

    <el-dialog
      :title="dialogType === 'add' ? '添加用户' : '编辑用户'"
      v-model="dialogVisible"
      width="520px"
    >
      <el-form
        ref="userFormRef"
        :model="userForm"
        :rules="rules"
        label-width="100px"
      >
        <el-form-item label="用户名" prop="username">
          <el-input v-model="userForm.username" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="userForm.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item v-if="dialogType === 'add'" label="密码" prop="password">
          <el-input v-model="userForm.password" type="password" show-password placeholder="请输入密码" />
        </el-form-item>
        <el-form-item v-else label="新密码">
          <el-input
            v-model="userForm.password"
            type="password"
            show-password
            placeholder="留空则不修改密码"
          />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="userForm.role" placeholder="请选择角色" style="width: 100%">
            <el-option label="管理员" value="admin" />
            <el-option label="普通用户" value="user" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSaveUser">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, h, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { usersApi } from '@/api'

const users = ref([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const dialogType = ref('add')
const searchQuery = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const userFormRef = ref(null)

const userForm = reactive({
  id: null,
  username: '',
  email: '',
  password: '',
  role: 'user'
})

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '长度在 3 到 20 个字符', trigger: 'blur' }
  ],
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱地址', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于 6 个字符', trigger: 'blur' }
  ],
  role: [
    { required: true, message: '请选择角色', trigger: 'change' }
  ]
}

const formatDateTime = (value) => {
  if (!value) {
    return '-'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString('zh-CN', { hour12: false })
}

const normalizeUser = (user) => ({
  id: user.id,
  username: user.username,
  email: user.email || '-',
  role: user.role || 'user',
  status: user.status ?? user.enabled ?? true,
  created: formatDateTime(user.created_at || user.created),
  lastLogin: formatDateTime(user.last_login || user.last_login_at || user.lastLogin)
})

const filteredUsers = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) {
    return users.value
  }

  return users.value.filter(user =>
    user.username.toLowerCase().includes(query) ||
    user.email.toLowerCase().includes(query)
  )
})

const displayTotal = computed(() => filteredUsers.value.length)

const paginatedUsers = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredUsers.value.slice(start, end)
})

const syncCurrentPage = () => {
  const maxPage = Math.max(1, Math.ceil(displayTotal.value / pageSize.value))
  if (currentPage.value > maxPage) {
    currentPage.value = maxPage
  }
}

const clearFormValidation = async () => {
  await nextTick()
  userFormRef.value?.clearValidate()
}

const fetchUsers = async () => {
  loading.value = true
  try {
    const response = await usersApi.list()
    const list = Array.isArray(response) ? response : response?.users || response?.list || []
    users.value = list.map(normalizeUser)
    syncCurrentPage()
  } catch (error) {
    console.error('Failed to fetch users:', error)
    ElMessage.error(error.message || '获取用户列表失败')
    users.value = []
  } finally {
    loading.value = false
  }
}

const showAddDialog = async () => {
  dialogType.value = 'add'
  Object.assign(userForm, {
    id: null,
    username: '',
    email: '',
    password: '',
    role: 'user'
  })
  dialogVisible.value = true
  await clearFormValidation()
}

const handleEdit = async (row) => {
  dialogType.value = 'edit'
  Object.assign(userForm, {
    id: row.id,
    username: row.username,
    email: row.email === '-' ? '' : row.email,
    password: '',
    role: row.role
  })
  dialogVisible.value = true
  await clearFormValidation()
}

const handleSaveUser = async () => {
  if (!userFormRef.value) {
    return
  }

  await userFormRef.value.validate()

  saving.value = true
  try {
    if (dialogType.value === 'add') {
      await usersApi.create({
        username: userForm.username,
        email: userForm.email,
        password: userForm.password,
        role: userForm.role
      })
      ElMessage.success('添加成功')
    } else {
      const payload = {
        username: userForm.username,
        email: userForm.email,
        role: userForm.role
      }

      if (userForm.password.trim()) {
        payload.password = userForm.password.trim()
      }

      await usersApi.update(userForm.id, payload)
      ElMessage.success('更新成功')
    }

    dialogVisible.value = false
    await fetchUsers()
  } catch (error) {
    console.error('Failed to save user:', error)
    ElMessage.error(error.message || '保存用户失败')
  } finally {
    saving.value = false
  }
}

const handleToggleStatus = async (row) => {
  try {
    if (row.status) {
      await usersApi.disable(row.id)
    } else {
      await usersApi.enable(row.id)
    }

    row.status = !row.status
    ElMessage.success(`已${row.status ? '启用' : '禁用'}用户：${row.username}`)
  } catch (error) {
    console.error('Failed to update user status:', error)
    ElMessage.error(error.message || '更新用户状态失败')
  }
}

const handleResetPassword = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要重置用户 ${row.username} 的密码吗？`,
      '重置密码',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    const response = await usersApi.resetPassword(row.id)
    const temporaryPassword = response?.temporary_password || response?.temporaryPassword || ''

    await ElMessageBox.alert(
      h('div', { class: 'reset-password-result' }, [
        h('p', null, `用户：${row.username}`),
        h('p', null, '临时密码：'),
        h('code', { class: 'temp-password-code' }, temporaryPassword || '未返回临时密码'),
        h('p', { style: 'margin-top: 12px;' }, '用户下次登录后需要修改密码。')
      ]),
      '密码已重置',
      {
        confirmButtonText: '知道了'
      }
    )
  } catch (error) {
    if (error === 'cancel') {
      return
    }

    console.error('Failed to reset password:', error)
    ElMessage.error(error.message || '重置密码失败')
  }
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除用户 ${row.username} 吗？`,
      '警告',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    await usersApi.delete(row.id)
    ElMessage.success('删除成功')
    await fetchUsers()
  } catch (error) {
    if (error === 'cancel') {
      return
    }

    console.error('Failed to delete user:', error)
    ElMessage.error(error.message || '删除用户失败')
  }
}

const handleSearch = () => {
  currentPage.value = 1
}

const handleSizeChange = (value) => {
  pageSize.value = value
  syncCurrentPage()
}

const handleCurrentChange = (value) => {
  currentPage.value = value
}

onMounted(fetchUsers)
</script>

<style scoped>
.users-container {
  padding: 20px;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.page-header h1 {
  margin: 0;
}

.actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}

:deep(.temp-password-code) {
  display: inline-block;
  margin-top: 4px;
  padding: 8px 10px;
  border-radius: 6px;
  background: #f5f7fa;
  color: #111827;
  font-size: 14px;
}
</style>
