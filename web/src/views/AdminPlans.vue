<template>
  <div class="admin-plans-page">
    <div class="page-header">
      <h1 class="page-title">套餐管理</h1>
      <el-button type="primary" @click="showCreateDialog">
        <el-icon><Plus /></el-icon>
        创建套餐
      </el-button>
    </div>

    <el-card shadow="never">
      <el-table :data="plans" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="名称" />
        <el-table-column label="价格" width="120">
          <template #default="{ row }">¥{{ formatPrice(row.price) }}</template>
        </el-table-column>
        <el-table-column prop="duration" label="时长(天)" width="100" />
        <el-table-column label="流量限制" width="120">
          <template #default="{ row }">{{ formatTraffic(row.traffic_limit) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-switch v-model="row.is_active" @change="toggleStatus(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="sort_order" label="排序" width="80" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="editPlan(row)">编辑</el-button>
            <el-button type="danger" link @click="deletePlan(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="pagination.total > 0" class="pagination-container">
        <el-pagination
          v-model:current-page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.pageSize"
          layout="total, prev, pager, next"
          @current-change="fetchPlans"
        />
      </div>
    </el-card>

    <!-- 创建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑套餐' : '创建套餐'" width="600px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入套餐名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="请输入套餐描述" />
        </el-form-item>
        <el-form-item label="价格(元)" prop="price">
          <el-input-number v-model="form.price" :min="0" :precision="2" />
        </el-form-item>
        <el-form-item label="时长(天)" prop="duration">
          <el-input-number v-model="form.duration" :min="1" />
        </el-form-item>
        <el-form-item label="流量限制(GB)" prop="traffic_limit">
          <el-input-number v-model="form.traffic_limit" :min="0" placeholder="0表示无限制" />
        </el-form-item>
        <el-form-item label="并发 IP 限制" prop="ip_limit">
          <el-input-number v-model="form.ip_limit" :min="0" placeholder="0表示不限制" />
        </el-form-item>
        <el-form-item label="排序" prop="sort_order">
          <el-input-number v-model="form.sort_order" :min="0" />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.is_active" />
        </el-form-item>
        <el-form-item label="功能特性">
          <el-tag
            v-for="(feature, index) in form.features"
            :key="index"
            closable
            @close="removeFeature(index)"
            style="margin-right: 8px; margin-bottom: 8px;"
          >
            {{ feature }}
          </el-tag>
          <el-input
            v-if="showFeatureInput"
            v-model="newFeature"
            size="small"
            style="width: 200px;"
            @keyup.enter="addFeature"
            @blur="addFeature"
          />
          <el-button v-else size="small" @click="showFeatureInput = true">+ 添加</el-button>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitForm">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { plansApi } from '@/api/index'

const loading = ref(false)
const plans = ref([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const dialogVisible = ref(false)
const isEdit = ref(false)
const submitting = ref(false)
const formRef = ref(null)
const showFeatureInput = ref(false)
const newFeature = ref('')

const form = reactive({
  id: null,
  name: '',
  description: '',
  price: 0,
  duration: 30,
  traffic_limit: 0,
  ip_limit: 0,
  sort_order: 0,
  is_active: true,
  features: []
})

const rules = {
  name: [{ required: true, message: '请输入套餐名称', trigger: 'blur' }],
  price: [{ required: true, message: '请输入价格', trigger: 'blur' }],
  duration: [{ required: true, message: '请输入时长', trigger: 'blur' }]
}

const formatPrice = (price) => (price / 100).toFixed(2)
const formatTraffic = (bytes) => {
  if (!bytes) return '无限制'
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(0)} GB`
}

const fetchPlans = async () => {
  loading.value = true
  try {
    const res = await plansApi.admin.list({ page: pagination.page, page_size: pagination.pageSize, include_inactive: true })
    plans.value = res.plans || []
    pagination.total = res.total || 0
  } catch (e) {
    ElMessage.error(e.message || '获取套餐列表失败')
  } finally {
    loading.value = false
  }
}

const showCreateDialog = () => {
  isEdit.value = false
  Object.assign(form, { id: null, name: '', description: '', price: 0, duration: 30, traffic_limit: 0, ip_limit: 0, sort_order: 0, is_active: true, features: [] })
  dialogVisible.value = true
}

const editPlan = (plan) => {
  isEdit.value = true
  Object.assign(form, {
    id: plan.id,
    name: plan.name,
    description: plan.description,
    price: plan.price / 100,
    duration: plan.duration,
    traffic_limit: plan.traffic_limit ? plan.traffic_limit / (1024 * 1024 * 1024) : 0,
    ip_limit: plan.ip_limit || 0,
    sort_order: plan.sort_order || 0,
    is_active: plan.is_active ?? true,
    features: plan.features || []
  })
  dialogVisible.value = true
}

const submitForm = async () => {
  await formRef.value.validate()
  submitting.value = true
  try {
    const data = {
      name: form.name,
      description: form.description,
      price: Math.round(form.price * 100),
      duration: form.duration,
      traffic_limit: form.traffic_limit ? form.traffic_limit * 1024 * 1024 * 1024 : 0,
      ip_limit: form.ip_limit,
      sort_order: form.sort_order,
      is_active: form.is_active,
      features: form.features
    }
    if (isEdit.value) {
      await plansApi.admin.update(form.id, data)
      ElMessage.success('更新成功')
    } else {
      await plansApi.admin.create(data)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    fetchPlans()
  } catch (e) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const toggleStatus = async (plan) => {
  try {
    await plansApi.admin.setActive(plan.id, plan.is_active)
    ElMessage.success(plan.is_active ? '已启用' : '已禁用')
  } catch (e) {
    plan.is_active = !plan.is_active
    ElMessage.error(e.message || '操作失败')
  }
}

const deletePlan = async (plan) => {
  await ElMessageBox.confirm('确定要删除此套餐吗？', '提示', { type: 'warning' })
  try {
    await plansApi.admin.delete(plan.id)
    ElMessage.success('删除成功')
    fetchPlans()
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

const addFeature = () => {
  if (newFeature.value.trim()) {
    form.features.push(newFeature.value.trim())
    newFeature.value = ''
  }
  showFeatureInput.value = false
}

const removeFeature = (index) => {
  form.features.splice(index, 1)
}

onMounted(fetchPlans)
</script>

<style scoped>
.admin-plans-page { padding: 20px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-title { font-size: 24px; font-weight: 600; margin: 0; }
.pagination-container { margin-top: 20px; display: flex; justify-content: flex-end; }
</style>
