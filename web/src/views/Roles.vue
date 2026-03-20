<template>
  <div class="roles-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">角色管理</h1>
        <p class="page-subtitle">整理角色权限、系统角色和用户归属关系</p>
      </div>
      <div class="page-actions">
        <el-button type="primary" @click="showCreateDialog">
          <el-icon><Plus /></el-icon>
          新建角色
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">角色总数</span>
        <strong class="overview-value">{{ roles.length }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">系统角色</span>
        <strong class="overview-value is-warning">{{ systemRoleCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">自定义角色</span>
        <strong class="overview-value is-primary">{{ customRoleCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">角色关联用户</span>
        <strong class="overview-value is-success">{{ assignedUserCount }}</strong>
      </div>
    </div>

    <el-card>
      <template #header>
        <div class="card-header">
          <span>角色列表</span>
          <span class="toolbar-summary">共 {{ roles.length }} 个角色</span>
        </div>
      </template>

      <el-table :data="roles" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="角色名称" width="150">
          <template #default="{ row }">
            <el-tag :type="row.is_system ? 'danger' : 'primary'">{{ row.name }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" />
        <el-table-column prop="permissions" label="权限" min-width="250">
          <template #default="{ row }">
            <div class="permissions-list">
              <el-tag 
                v-for="perm in row.permissions" 
                :key="perm" 
                size="small" 
                type="info"
                style="margin: 2px"
              >
                {{ getPermissionName(perm) }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="user_count" label="用户数" width="100" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.is_system ? 'warning' : 'success'" size="small">
              {{ row.is_system ? '系统角色' : '自定义' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button 
              type="primary" 
              size="small" 
              @click="editRole(row)"
              :disabled="row.is_system"
            >
              编辑
            </el-button>
            <el-button 
              type="danger" 
              size="small" 
              @click="deleteRole(row)"
              :disabled="row.is_system || row.user_count > 0"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 创建/编辑角色对话框 -->
    <el-dialog 
      v-model="dialogVisible" 
      :title="isEdit ? '编辑角色' : '新建角色'"
      width="600px"
    >
      <el-form :model="roleForm" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="角色名称" prop="name">
          <el-input v-model="roleForm.name" placeholder="请输入角色名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input 
            v-model="roleForm.description" 
            type="textarea" 
            :rows="3"
            placeholder="请输入角色描述" 
          />
        </el-form-item>
        <el-form-item label="权限" prop="permissions">
          <el-checkbox-group v-model="roleForm.permissions">
            <el-checkbox 
              v-for="perm in allPermissions" 
              :key="perm.key" 
              :label="perm.key"
            >
              {{ perm.name }} - {{ perm.description }}
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitForm" :loading="submitting">
          {{ isEdit ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { roles as rolesApi, usersApi } from '@/api/index'

const loading = ref(false)
const submitting = ref(false)
const roles = ref([])
const allPermissions = ref([])
const dialogVisible = ref(false)
const isEdit = ref(false)
const editingId = ref(null)
const formRef = ref(null)

const roleForm = ref({
  name: '',
  description: '',
  permissions: []
})

const rules = {
  name: [
    { required: true, message: '请输入角色名称', trigger: 'blur' },
    { min: 2, max: 50, message: '长度在 2 到 50 个字符', trigger: 'blur' }
  ]
}

const systemRoleCount = computed(() => roles.value.filter((role) => role.is_system).length)
const customRoleCount = computed(() => roles.value.filter((role) => !role.is_system).length)
const assignedUserCount = computed(() =>
  roles.value.reduce((sum, role) => sum + Number(role.user_count || 0), 0)
)

// 获取权限显示名称
const getPermissionName = (key) => {
  const perm = allPermissions.value.find(p => p.key === key)
  return perm ? perm.name : key
}

// 加载角色列表
const loadRoles = async () => {
  loading.value = true
  try {
    const response = await rolesApi.getRoles()
    if (response.code === 200) {
      roles.value = response.data || []
    }
  } catch (error) {
    console.error('加载角色列表失败:', error)
    ElMessage.error('加载角色列表失败')
  } finally {
    loading.value = false
  }
}

// 加载权限列表
const loadPermissions = async () => {
  try {
    const response = await usersApi.getPermissions()
    if (response.code === 200) {
      allPermissions.value = response.data || []
    }
  } catch (error) {
    console.error('加载权限列表失败:', error)
  }
}

// 显示创建对话框
const showCreateDialog = () => {
  isEdit.value = false
  editingId.value = null
  roleForm.value = {
    name: '',
    description: '',
    permissions: []
  }
  dialogVisible.value = true
}

// 编辑角色
const editRole = (row) => {
  isEdit.value = true
  editingId.value = row.id
  roleForm.value = {
    name: row.name,
    description: row.description,
    permissions: [...row.permissions]
  }
  dialogVisible.value = true
}

// 删除角色
const deleteRole = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除角色 "${row.name}" 吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    const response = await rolesApi.deleteRole(row.id)
    if (response.code === 200) {
      ElMessage.success('删除成功')
      loadRoles()
    } else {
      ElMessage.error(response.message || '删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除角色失败:', error)
      ElMessage.error('删除角色失败')
    }
  }
}

// 提交表单
const submitForm = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    
    submitting.value = true
    try {
      let response
      if (isEdit.value) {
        response = await rolesApi.updateRole(editingId.value, roleForm.value)
      } else {
        response = await rolesApi.createRole(roleForm.value)
      }
      
      if (response.code === 200 || response.code === 201) {
        ElMessage.success(isEdit.value ? '保存成功' : '创建成功')
        dialogVisible.value = false
        loadRoles()
      } else {
        ElMessage.error(response.message || '操作失败')
      }
    } catch (error) {
      console.error('操作失败:', error)
      ElMessage.error('操作失败')
    } finally {
      submitting.value = false
    }
  })
}

onMounted(() => {
  loadRoles()
  loadPermissions()
})
</script>

<style scoped>
.roles-page {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.permissions-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.el-checkbox {
  display: block;
  margin-bottom: 8px;
}
</style>
