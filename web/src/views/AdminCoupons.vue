<template>
  <div class="admin-coupons-page">
    <div class="page-header">
      <h1 class="page-title">优惠券管理</h1>
      <el-button type="primary" @click="showCreateDialog">
        <el-icon><Plus /></el-icon>
        创建优惠券
      </el-button>
    </div>

    <el-card shadow="never">
      <el-table :data="coupons" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="code" label="优惠码" width="150" />
        <el-table-column prop="name" label="名称" min-width="180" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.type === 'fixed' ? 'success' : 'warning'" size="small">
              {{ row.type === 'fixed' ? '固定金额' : '百分比' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="优惠值" width="100">
          <template #default="{ row }">
            {{ row.type === 'fixed' ? `¥${formatPrice(row.value)}` : `${formatPercent(row.value)}%` }}
          </template>
        </el-table-column>
        <el-table-column label="使用情况" width="120">
          <template #default="{ row }">
            {{ row.used_count }} / {{ row.total_limit || '∞' }}
          </template>
        </el-table-column>
        <el-table-column label="有效期" width="200">
          <template #default="{ row }">
            {{ row.start_at }} ~ {{ row.expire_at }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row)" size="small">{{ getStatusLabel(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="editCoupon(row)">编辑</el-button>
            <el-button type="danger" link @click="deleteCoupon(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="pagination.total > 0" class="pagination-container">
        <el-pagination
          v-model:current-page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.pageSize"
          layout="total, prev, pager, next"
          @current-change="fetchCoupons"
        />
      </div>
    </el-card>

    <!-- 创建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑优惠券' : '创建优惠券'" width="600px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="优惠码" prop="code">
          <el-input v-model="form.code" placeholder="留空自动生成" :disabled="isEdit" />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入优惠券名称" />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="form.type">
            <el-radio label="fixed">固定金额</el-radio>
            <el-radio label="percentage">百分比</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="优惠值" prop="value">
          <el-input-number v-model="form.value" :min="0" :max="form.type === 'percentage' ? 100 : 999999" />
          <span style="margin-left: 8px;">{{ form.type === 'fixed' ? '元' : '%' }}</span>
        </el-form-item>
        <el-form-item label="最低消费" prop="min_order_amount">
          <el-input-number v-model="form.min_order_amount" :min="0" />
          <span style="margin-left: 8px;">元</span>
        </el-form-item>
        <el-form-item v-if="form.type === 'percentage'" label="最大折扣" prop="max_discount">
          <el-input-number v-model="form.max_discount" :min="0" />
          <span style="margin-left: 8px;">元</span>
        </el-form-item>
        <el-form-item label="总数量" prop="total_limit">
          <el-input-number v-model="form.total_limit" :min="0" placeholder="0表示无限制" />
        </el-form-item>
        <el-form-item label="每人限用" prop="per_user_limit">
          <el-input-number v-model="form.per_user_limit" :min="0" placeholder="0表示无限制" />
        </el-form-item>
        <el-form-item label="有效期" prop="dateRange">
          <el-date-picker v-model="form.dateRange" type="datetimerange" start-placeholder="开始时间" end-placeholder="结束时间" />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.is_active" />
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
import { couponsApi } from '@/api/index'

const loading = ref(false)
const coupons = ref([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const dialogVisible = ref(false)
const isEdit = ref(false)
const submitting = ref(false)
const formRef = ref(null)

const form = reactive({
  id: null,
  code: '',
  name: '',
  type: 'fixed',
  value: 0,
  min_order_amount: 0,
  max_discount: 0,
  total_limit: 0,
  per_user_limit: 1,
  dateRange: null,
  is_active: true
})

const rules = {
  name: [{ required: true, message: '请输入优惠券名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  value: [{ required: true, message: '请输入优惠值', trigger: 'blur' }],
  dateRange: [{ required: true, message: '请选择有效期', trigger: 'change' }]
}

const formatPrice = (price) => (price / 100).toFixed(2)
const formatPercent = (value) => (value / 100).toFixed(value % 100 === 0 ? 0 : 2)
const formatDateTime = (value) => {
  if (!value) return undefined

  const date = new Date(value)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

const getStatusType = (coupon) => {
  const now = new Date()
  if (!coupon.is_active) return 'info'
  if (new Date(coupon.expire_at) < now) return 'warning'
  if (coupon.total_limit && coupon.used_count >= coupon.total_limit) return 'danger'
  return 'success'
}

const getStatusLabel = (coupon) => {
  const now = new Date()
  if (!coupon.is_active) return '已禁用'
  if (new Date(coupon.expire_at) < now) return '已过期'
  if (coupon.total_limit && coupon.used_count >= coupon.total_limit) return '已用完'
  return '有效'
}

const fetchCoupons = async () => {
  loading.value = true
  try {
    const res = await couponsApi.admin.list({ page: pagination.page, page_size: pagination.pageSize })
    coupons.value = res.coupons || []
    pagination.total = res.total || 0
  } catch (e) {
    ElMessage.error(e.message || '获取优惠券列表失败')
  } finally {
    loading.value = false
  }
}

const showCreateDialog = () => {
  isEdit.value = false
  Object.assign(form, { id: null, code: '', name: '', type: 'fixed', value: 0, min_order_amount: 0, max_discount: 0, total_limit: 0, per_user_limit: 1, dateRange: null, is_active: true })
  dialogVisible.value = true
}

const editCoupon = (coupon) => {
  isEdit.value = true
  Object.assign(form, {
    id: coupon.id,
    code: coupon.code,
    name: coupon.name,
    type: coupon.type,
    value: coupon.type === 'fixed' ? coupon.value / 100 : coupon.value / 100,
    min_order_amount: coupon.min_order_amount / 100,
    max_discount: coupon.max_discount / 100,
    total_limit: coupon.total_limit,
    per_user_limit: coupon.per_user_limit,
    dateRange: [new Date(coupon.start_at), new Date(coupon.expire_at)],
    is_active: coupon.is_active
  })
  dialogVisible.value = true
}

const submitForm = async () => {
  await formRef.value.validate()
  submitting.value = true
  try {
    const data = {
      code: form.code || undefined,
      name: form.name,
      type: form.type,
      value: form.type === 'fixed' ? Math.round(form.value * 100) : Math.round(form.value * 100),
      min_order_amount: Math.round(form.min_order_amount * 100),
      max_discount: Math.round(form.max_discount * 100),
      total_limit: form.total_limit,
      per_user_limit: form.per_user_limit,
      start_at: formatDateTime(form.dateRange?.[0]),
      expire_at: formatDateTime(form.dateRange?.[1]),
      is_active: form.is_active
    }
    if (isEdit.value) {
      await couponsApi.admin.update(form.id, data)
      ElMessage.success('更新成功')
    } else {
      await couponsApi.admin.create(data)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    fetchCoupons()
  } catch (e) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const deleteCoupon = async (coupon) => {
  await ElMessageBox.confirm('确定要删除此优惠券吗？', '提示', { type: 'warning' })
  try {
    await couponsApi.admin.delete(coupon.id)
    ElMessage.success('删除成功')
    fetchCoupons()
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

onMounted(fetchCoupons)
</script>

<style scoped>
.admin-coupons-page { padding: 20px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-title { font-size: 24px; font-weight: 600; margin: 0; }
.pagination-container { margin-top: 20px; display: flex; justify-content: flex-end; }
</style>
