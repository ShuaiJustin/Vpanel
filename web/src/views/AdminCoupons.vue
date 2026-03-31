<template>
  <div class="admin-coupons-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          优惠券管理
        </h1>
        <p class="page-subtitle">
          集中维护优惠策略、使用限制和有效期状态
        </p>
      </div>
      <div class="page-actions">
        <el-button
          type="primary"
          @click="showCreateDialog"
        >
          <el-icon class="el-icon--left">
            <Plus />
          </el-icon>
          创建优惠券
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">优惠券总数</span>
        <strong class="overview-value">{{ pagination.total }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页有效</span>
        <strong class="overview-value is-success">{{ validCouponCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页已过期</span>
        <strong class="overview-value is-warning">{{ expiredCouponCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页已用完</span>
        <strong class="overview-value is-danger">{{ exhaustedCouponCount }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-actions">
        <span class="toolbar-summary">当前页 {{ coupons.length }} 张优惠券，共 {{ pagination.total }} 张</span>
        <el-button @click="fetchCoupons">
          刷新
        </el-button>
      </div>
    </div>

    <div class="table-shell">
      <el-table
        v-loading="loading"
        :data="coupons"
        border
        stripe
        class="coupons-table"
        row-key="id"
      >
        <el-table-column
          label="优惠券信息"
          min-width="300"
        >
          <template #default="{ row }">
            <div class="entity-cell">
              <div class="entity-cell__header">
                <span class="entity-cell__title">{{ row.name }}</span>
                <span :class="['metric-pill', getStatusPillClass(row)]">{{ getStatusLabel(row) }}</span>
              </div>
              <div class="entity-cell__meta">
                <span>ID：{{ row.id }}</span>
                <span>优惠码：</span>
                <span class="mono-code">
                  <span class="mono-code__value">{{ row.code }}</span>
                  <el-button
                    text
                    class="inline-copy-btn"
                    @click="copyCode(row.code)"
                  >
                    <el-icon><CopyDocument /></el-icon>
                  </el-button>
                </span>
              </div>
              <div class="entity-cell__hint">
                {{ getCouponHint(row) }}
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="优惠规则"
          min-width="240"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">类型</span>
                <span class="stack-value is-strong">{{ row.type === 'fixed' ? '固定金额' : '百分比折扣' }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">优惠值</span>
                <span class="stack-value is-success">{{ row.type === 'fixed' ? `¥${formatPrice(row.value)}` : `${formatPercent(row.value)}%` }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">最低消费</span>
                <span class="stack-value">{{ row.min_order_amount ? `¥${formatPrice(row.min_order_amount)}` : '无门槛' }}</span>
              </div>
              <div
                v-if="row.type === 'percentage'"
                class="stack-item"
              >
                <span class="stack-label">最大减免</span>
                <span class="stack-value">{{ row.max_discount ? `¥${formatPrice(row.max_discount)}` : '不封顶' }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="使用与有效期"
          min-width="260"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">使用情况</span>
                <span class="stack-value">{{ row.used_count }} / {{ row.total_limit || '∞' }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">每人限用</span>
                <span class="stack-value">{{ row.per_user_limit || '∞' }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">有效期</span>
                <span class="stack-value">{{ formatDateSpan(row) }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="操作"
          width="150"
          align="right"
          fixed="right"
        >
          <template #default="{ row }">
            <div class="operation-btns">
              <el-button
                size="small"
                class="row-action row-action--primary"
                @click="editCoupon(row)"
              >
                编辑
              </el-button>
              <el-button
                size="small"
                class="row-action row-action--danger"
                @click="deleteCoupon(row)"
              >
                删除
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div
      v-if="pagination.total > 0"
      class="pagination-container"
    >
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @current-change="fetchCoupons"
        @size-change="handleSizeChange"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑优惠券' : '创建优惠券'"
      :width="isMobile ? 'calc(100vw - 24px)' : '600px'"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        :label-width="isMobile ? '90px' : '100px'"
      >
        <el-form-item
          label="优惠码"
          prop="code"
        >
          <el-input
            v-model="form.code"
            placeholder="留空自动生成"
            :disabled="isEdit"
          />
        </el-form-item>
        <el-form-item
          label="名称"
          prop="name"
        >
          <el-input
            v-model="form.name"
            placeholder="请输入优惠券名称"
          />
        </el-form-item>
        <el-form-item
          label="类型"
          prop="type"
        >
          <el-radio-group v-model="form.type">
            <el-radio label="fixed">
              固定金额
            </el-radio>
            <el-radio label="percentage">
              百分比
            </el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item
          label="优惠值"
          prop="value"
        >
          <el-input-number
            v-model="form.value"
            :min="0"
            :max="form.type === 'percentage' ? 100 : 999999"
          />
          <span class="form-unit">{{ form.type === 'fixed' ? '元' : '%' }}</span>
        </el-form-item>
        <el-form-item
          label="最低消费"
          prop="min_order_amount"
        >
          <el-input-number
            v-model="form.min_order_amount"
            :min="0"
          />
          <span class="form-unit">元</span>
        </el-form-item>
        <el-form-item
          v-if="form.type === 'percentage'"
          label="最大折扣"
          prop="max_discount"
        >
          <el-input-number
            v-model="form.max_discount"
            :min="0"
          />
          <span class="form-unit">元</span>
        </el-form-item>
        <el-form-item
          label="总数量"
          prop="total_limit"
        >
          <el-input-number
            v-model="form.total_limit"
            :min="0"
            placeholder="0表示无限制"
          />
        </el-form-item>
        <el-form-item
          label="每人限用"
          prop="per_user_limit"
        >
          <el-input-number
            v-model="form.per_user_limit"
            :min="0"
            placeholder="0表示无限制"
          />
        </el-form-item>
        <el-form-item
          label="有效期"
          prop="dateRange"
        >
          <el-date-picker
            v-model="form.dateRange"
            type="datetimerange"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
          />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.is_active" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="submitting"
          @click="submitForm"
        >
          确定
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Plus } from '@element-plus/icons-vue'
import { couponsApi } from '@/api/index'
import { useViewport } from '@/composables/useViewport'
import { copyText } from '@/utils/clipboard'
import { extractErrorMessage } from '@/utils/entitlement'

const { isMobile } = useViewport()

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

const validCouponCount = computed(() => coupons.value.filter((coupon) => getStatusLabel(coupon) === '有效').length)
const expiredCouponCount = computed(() => coupons.value.filter((coupon) => getStatusLabel(coupon) === '已过期').length)
const exhaustedCouponCount = computed(() => coupons.value.filter((coupon) => getStatusLabel(coupon) === '已用完').length)

const formatPrice = (price) => (Number(price || 0) / 100).toFixed(2)
const formatPercent = (value) => (Number(value || 0) / 100).toFixed(value % 100 === 0 ? 0 : 2)
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

const formatDateSpan = (coupon) => `${coupon.start_at || '-'} 至 ${coupon.expire_at || '-'}`

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

const getStatusPillClass = (coupon) => {
  const statusType = getStatusType(coupon)
  if (statusType === 'success') return 'is-success'
  if (statusType === 'warning') return 'is-warning'
  if (statusType === 'danger') return 'is-danger'
  return 'is-muted'
}

const getCouponHint = (coupon) => {
  if (coupon.type === 'percentage') {
    return `按比例优惠，最高减免 ${coupon.max_discount ? `¥${formatPrice(coupon.max_discount)}` : '不设上限'}。`
  }

  return `固定减免 ¥${formatPrice(coupon.value)}，适合营销活动直减。`
}

const clearFormValidation = async () => {
  await nextTick()
  formRef.value?.clearValidate()
}

const fetchCoupons = async () => {
  loading.value = true
  try {
    const res = await couponsApi.admin.list({ page: pagination.page, page_size: pagination.pageSize })
    coupons.value = res.coupons || []
    pagination.total = res.total || 0
  } catch (e) {
    ElMessage.error(extractErrorMessage(e) || '获取优惠券列表失败')
  } finally {
    loading.value = false
  }
}

const copyCode = async (code) => {
  try {
    await copyText(code)
    ElMessage.success('优惠码已复制')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

const resetForm = () => {
  Object.assign(form, {
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
}

const showCreateDialog = async () => {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
  await clearFormValidation()
}

const editCoupon = async (coupon) => {
  isEdit.value = true
  Object.assign(form, {
    id: coupon.id,
    code: coupon.code,
    name: coupon.name,
    type: coupon.type,
    value: coupon.value / 100,
    min_order_amount: coupon.min_order_amount / 100,
    max_discount: coupon.max_discount / 100,
    total_limit: coupon.total_limit,
    per_user_limit: coupon.per_user_limit,
    dateRange: [new Date(coupon.start_at), new Date(coupon.expire_at)],
    is_active: coupon.is_active
  })
  dialogVisible.value = true
  await clearFormValidation()
}

const submitForm = async () => {
  await formRef.value.validate()
  submitting.value = true
  try {
    const data = {
      code: form.code || undefined,
      name: form.name,
      type: form.type,
      value: Math.round(form.value * 100),
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
    ElMessage.error(extractErrorMessage(e) || '操作失败')
  } finally {
    submitting.value = false
  }
}

const handleSizeChange = async (pageSize) => {
  pagination.page = 1
  pagination.pageSize = pageSize
  await fetchCoupons()
}

const deleteCoupon = async (coupon) => {
  await ElMessageBox.confirm('确定要删除此优惠券吗？', '提示', { type: 'warning' })
  try {
    await couponsApi.admin.delete(coupon.id)
    ElMessage.success('删除成功')
    fetchCoupons()
  } catch (e) {
    ElMessage.error(extractErrorMessage(e) || '删除失败')
  }
}

onMounted(fetchCoupons)
</script>

<style scoped>
.admin-coupons-page {
  padding: 20px;
}

.coupons-table {
  width: 100%;
  min-width: 960px;
}

.form-unit {
  margin-left: 8px;
  font-size: 12px;
  color: #64748b;
}

@media (max-width: 768px) {
  .admin-coupons-page {
    padding: 12px;
  }

  .coupons-table {
    min-width: 760px;
  }
}
</style>
