<template>
  <div class="users-container">
    <div class="page-header">
      <div class="page-heading">
        <h1>用户管理</h1>
        <p class="page-subtitle">
          统一维护账户资料、权限角色和启用状态
        </p>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">当前匹配</span>
        <strong class="overview-value">{{ displayTotal }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">管理员</span>
        <strong class="overview-value is-danger">{{ adminUserCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">启用中</span>
        <strong class="overview-value is-success">{{ enabledUserCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">已禁用</span>
        <strong class="overview-value is-muted">{{ disabledUserCount }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-filters">
        <el-input
          v-model="searchQuery"
          class="toolbar-search"
          placeholder="搜索用户名或邮箱"
          clearable
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select
          v-model="roleFilter"
          clearable
          placeholder="角色"
          @change="handleFilterChange"
        >
          <el-option
            label="管理员"
            value="admin"
          />
          <el-option
            label="普通用户"
            value="user"
          />
        </el-select>
        <el-select
          v-model="statusFilter"
          clearable
          placeholder="状态"
          @change="handleFilterChange"
        >
          <el-option
            label="启用中"
            value="enabled"
          />
          <el-option
            label="已禁用"
            value="disabled"
          />
        </el-select>
        <el-button @click="resetFilters">
          重置
        </el-button>
      </div>
      <div class="toolbar-actions">
        <span class="toolbar-summary">当前筛选 {{ displayTotal }} 个账户，当前页 {{ paginatedUsers.length }} 个</span>
        <el-button
          type="primary"
          @click="showAddDialog"
        >
          添加用户
        </el-button>
      </div>
    </div>

    <div class="users-table-wrap table-shell">
      <el-table
        v-loading="loading"
        :data="paginatedUsers"
        border
        stripe
        row-key="id"
        class="users-table"
        :empty-text="displayTotal ? '当前页暂无数据' : (hasActiveFilters ? '暂无匹配用户' : '暂无用户')"
      >
        <el-table-column
          label="用户信息"
          min-width="280"
        >
          <template #default="{ row }">
            <div class="entity-cell">
              <div class="entity-cell__header">
                <span
                  class="entity-cell__title"
                  :title="row.username"
                >{{ row.username }}</span>
                <span :class="['metric-pill', row.role === 'admin' ? 'is-danger' : 'is-primary']">
                  {{ row.role === 'admin' ? '管理员' : '普通用户' }}
                </span>
              </div>
              <div class="entity-cell__meta">
                <span>ID：{{ row.id }}</span>
                <span :title="row.email">邮箱：{{ row.email }}</span>
              </div>
              <div class="entity-cell__hint">
                {{ getUserHint(row) }}
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="账户概况"
          min-width="220"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">创建时间</span>
                <span class="stack-value">{{ row.created }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">最后登录</span>
                <span class="stack-value">{{ row.lastLogin }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="状态与权限"
          min-width="220"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item stack-item--inline">
                <span class="stack-label">账户状态</span>
                <span :class="['metric-pill', row.status ? 'is-success' : 'is-muted']">
                  {{ row.status ? '启用中' : '已禁用' }}
                </span>
              </div>
              <div class="stack-item">
                <span class="stack-label">后台权限</span>
                <span class="stack-value is-strong">{{ row.role === 'admin' ? '完整管理权限' : '普通使用权限' }}</span>
              </div>
              <div class="entity-cell__hint">
                {{ row.role === 'admin' ? '可访问后台管理和系统配置。' : '仅用于前台订阅与面板使用。' }}
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="操作"
          width="190"
          align="right"
          fixed="right"
        >
          <template #default="{ row }">
            <div class="operation-btns">
              <el-button
                size="small"
                class="row-action row-action--primary"
                @click="handleEdit(row)"
              >
                编辑
              </el-button>
              <el-button
                size="small"
                class="row-action"
                :class="row.status ? 'row-action--warning' : 'row-action--success'"
                :disabled="row.id === currentUserId"
                @click="handleToggleStatus(row)"
              >
                {{ row.status ? '禁用' : '启用' }}
              </el-button>
              <el-dropdown
                trigger="click"
                @command="(command) => handleRowCommand(command, row)"
              >
                <el-button
                  size="small"
                  class="row-action row-action--more"
                  circle
                  title="更多操作"
                >
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item
                      command="resetPassword"
                      :disabled="row.id === currentUserId"
                    >
                      重置密码
                    </el-dropdown-item>
                    <el-dropdown-item
                      command="delete"
                      divided
                      :disabled="row.id === currentUserId"
                    >
                      删除用户
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :layout="isMobile ? 'prev, pager, next' : isCompact ? 'total, prev, pager, next' : 'total, sizes, prev, pager, next, jumper'"
        :total="displayTotal"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '添加用户' : '编辑用户'"
      :width="isMobile ? 'calc(100vw - 24px)' : '520px'"
    >
      <el-form
        ref="userFormRef"
        :model="userForm"
        :rules="rules"
        :label-width="isMobile ? '72px' : '100px'"
      >
        <el-form-item
          label="用户名"
          prop="username"
        >
          <el-input
            v-model="userForm.username"
            placeholder="请输入用户名"
          />
        </el-form-item>
        <el-form-item
          label="邮箱"
          prop="email"
        >
          <el-input
            v-model="userForm.email"
            placeholder="请输入邮箱"
          />
        </el-form-item>
        <el-form-item
          v-if="dialogType === 'add'"
          label="密码"
          prop="password"
        >
          <el-input
            v-model="userForm.password"
            type="password"
            show-password
            placeholder="请输入密码"
          />
        </el-form-item>
        <el-form-item
          v-else
          label="新密码"
        >
          <el-input
            v-model="userForm.password"
            type="password"
            show-password
            placeholder="留空则不修改密码"
          />
        </el-form-item>
        <el-form-item
          label="角色"
          prop="role"
        >
          <el-select
            v-model="userForm.role"
            placeholder="请选择角色"
            style="width: 100%"
          >
            <el-option
              label="管理员"
              value="admin"
            />
            <el-option
              label="普通用户"
              value="user"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="handleSaveUser"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, h, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { MoreFilled, Search } from '@element-plus/icons-vue'
import { usersApi } from '@/api'
import { useViewport } from '@/composables/useViewport'
import { debounce } from '@/utils/debounce'
import { extractErrorMessage } from '@/utils/entitlement'

const { isMobile, viewportWidth } = useViewport()

const users = ref([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const dialogType = ref('add')
const searchQuery = ref('')
const roleFilter = ref('')
const statusFilter = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const userFormRef = ref(null)
const currentUserId = ref(null)
const summary = reactive({
  adminTotal: 0,
  enabledTotal: 0,
  disabledTotal: 0
})

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
    { min: 3, max: 50, message: '长度在 3 到 50 个字符', trigger: 'blur' }
  ],
  email: [
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

const totalUsers = ref(0)

const normalizeUser = (user) => ({
  id: user.id,
  username: user.username,
  email: user.email || '-',
  role: user.role || 'user',
  status: user.status ?? user.enabled ?? true,
  created: formatDateTime(user.created_at || user.created),
  lastLogin: formatDateTime(user.last_login || user.last_login_at || user.lastLogin),
  forcePasswordChange: Boolean(user.force_password_change ?? user.forcePasswordChange)
})

const displayTotal = computed(() => totalUsers.value)
const hasActiveFilters = computed(() => Boolean(searchQuery.value.trim() || roleFilter.value || statusFilter.value))
const isCompact = computed(() => viewportWidth.value <= 1366)
const adminUserCount = computed(() => summary.adminTotal)
const enabledUserCount = computed(() => summary.enabledTotal)
const disabledUserCount = computed(() => summary.disabledTotal)
const paginatedUsers = computed(() => users.value)

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

const getUserHint = (user) => {
  if (user.forcePasswordChange) {
    return '该账户已被重置密码，下一次登录后必须先修改密码。'
  }

  if (!user.status) {
    return '当前账户已禁用，不允许登录和订阅。'
  }

  if (user.lastLogin === '-') {
    return '账户已创建，但暂时没有登录记录。'
  }

  return `最近一次登录时间：${user.lastLogin}`
}

const fetchUsers = async () => {
  loading.value = true
  try {
    const response = await usersApi.list({
      page: currentPage.value,
      page_size: pageSize.value,
      search: searchQuery.value.trim() || undefined,
      role: roleFilter.value || undefined,
      status: statusFilter.value || undefined
    })
    const list = Array.isArray(response) ? response : response?.users || response?.list || []
    users.value = list.map(normalizeUser)
    totalUsers.value = Number(response?.total || list.length)
    currentUserId.value = Number(response?.current_user_id || 0) || null
    summary.adminTotal = Number(response?.admin_total ?? users.value.filter((user) => user.role === 'admin').length)
    summary.enabledTotal = Number(response?.enabled_total ?? users.value.filter((user) => user.status).length)
    summary.disabledTotal = Number(response?.disabled_total ?? users.value.filter((user) => !user.status).length)
    syncCurrentPage()
  } catch (error) {
    console.error('Failed to fetch users:', error)
    ElMessage.error(extractErrorMessage(error) || '获取用户列表失败')
    users.value = []
    totalUsers.value = 0
    currentUserId.value = null
    summary.adminTotal = 0
    summary.enabledTotal = 0
    summary.disabledTotal = 0
  } finally {
    loading.value = false
  }
}

const debouncedFetchUsers = debounce(async () => {
  currentPage.value = 1
  await fetchUsers()
}, 300)

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
        username: userForm.username.trim(),
        email: userForm.email.trim(),
        password: userForm.password,
        role: userForm.role
      })
      ElMessage.success('添加成功')
    } else {
      const payload = {
        username: userForm.username.trim(),
        email: userForm.email.trim(),
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
    ElMessage.error(extractErrorMessage(error) || '保存用户失败')
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

    ElMessage.success(`已${row.status ? '禁用' : '启用'}用户：${row.username}`)
    await fetchUsers()
  } catch (error) {
    console.error('Failed to update user status:', error)
    ElMessage.error(extractErrorMessage(error) || '更新用户状态失败')
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
    await fetchUsers()
  } catch (error) {
    if (error === 'cancel') {
      return
    }

    console.error('Failed to reset password:', error)
    ElMessage.error(extractErrorMessage(error) || '重置密码失败')
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
    ElMessage.error(extractErrorMessage(error) || '删除用户失败')
  }
}

const handleRowCommand = (command, row) => {
  if (command === 'resetPassword') {
    handleResetPassword(row)
    return
  }

  if (command === 'delete') {
    handleDelete(row)
  }
}

const handleFilterChange = () => {
  currentPage.value = 1
  fetchUsers()
}

const resetFilters = () => {
  searchQuery.value = ''
  roleFilter.value = ''
  statusFilter.value = ''
  currentPage.value = 1
  fetchUsers()
}

const handleSizeChange = (value) => {
  pageSize.value = value
  currentPage.value = 1
  fetchUsers()
}

const handleCurrentChange = (value) => {
  currentPage.value = value
  fetchUsers()
}

watch(searchQuery, () => {
  debouncedFetchUsers()
})

onMounted(fetchUsers)

onUnmounted(() => {
  debouncedFetchUsers.cancel?.()
})
</script>

<style scoped>
.users-container {
  padding: 20px;
}

.users-table {
  width: 100%;
  min-width: 900px;
}

:deep(.users-table .cell) {
  word-break: break-word;
}

:deep(.temp-password-code) {
  display: inline-block;
  margin-top: 4px;
  padding: 8px 10px;
  border-radius: 10px;
  background: var(--color-bg-page);
  color: #111827;
  font-size: 14px;
}

@media (max-width: 768px) {
  .users-container {
    padding: 12px;
  }

  .users-table {
    min-width: 720px;
  }
}
</style>
