<template>
  <div class="admin-plans-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          套餐管理
        </h1>
        <p class="page-subtitle">
          集中维护销售套餐的价格、时长、流量和上架状态
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
          创建套餐
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">套餐总数</span>
        <strong class="overview-value">{{ pagination.total }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页已启用</span>
        <strong class="overview-value is-success">{{ activePlanCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页已停用</span>
        <strong class="overview-value is-muted">{{ inactivePlanCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">不限流量</span>
        <strong class="overview-value is-primary">{{ unlimitedPlanCount }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-filters">
        <span class="toolbar-summary">显示停用套餐</span>
        <el-switch
          v-model="includeInactive"
          @change="handleIncludeInactiveChange"
        />
      </div>
      <div class="toolbar-actions">
        <span class="toolbar-summary">当前页 {{ plans.length }} 个套餐，共 {{ pagination.total }} 个</span>
        <el-button @click="fetchPlans">
          刷新
        </el-button>
      </div>
    </div>

    <div class="table-shell">
      <el-table
        v-loading="loading"
        :data="plans"
        border
        stripe
        class="plans-table"
        row-key="id"
      >
        <el-table-column
          label="套餐信息"
          min-width="300"
        >
          <template #default="{ row }">
            <div class="entity-cell">
              <div class="entity-cell__header">
                <span class="entity-cell__title">{{ row.name }}</span>
                <span :class="['metric-pill', row.is_active ? 'is-success' : 'is-muted']">
                  {{ row.is_active ? '已上架' : '已停用' }}
                </span>
              </div>
              <div class="entity-cell__meta">
                <span>ID：{{ row.id }}</span>
                <span>特性 {{ row.features?.length || 0 }} 项</span>
              </div>
              <div class="entity-cell__hint">
                {{ row.description || '未填写套餐描述。' }}
              </div>
              <div
                v-if="row.features?.length"
                class="stack-tags"
              >
                <el-tag
                  v-for="feature in row.features.slice(0, 3)"
                  :key="feature"
                  size="small"
                  effect="plain"
                >
                  {{ feature }}
                </el-tag>
                <el-tag
                  v-if="row.features.length > 3"
                  size="small"
                  effect="plain"
                  type="info"
                >
                  +{{ row.features.length - 3 }}
                </el-tag>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="计费规则"
          min-width="220"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">售价</span>
                <span class="stack-value is-strong">¥{{ formatPrice(row.price) }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">时长</span>
                <span class="stack-value">{{ row.duration }} 天</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">排序</span>
                <span class="stack-value">{{ row.sort_order || 0 }}</span>
              </div>
              <div class="entity-cell__hint">
                {{ getBillingHint(row) }}
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="配额与状态"
          min-width="250"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">流量限制</span>
                <span class="stack-value">{{ formatTraffic(row.traffic_limit) }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">并发 IP</span>
                <span class="stack-value">{{ row.ip_limit ? `${row.ip_limit} 个` : '不限制' }}</span>
              </div>
              <div class="stack-item stack-item--inline">
                <span class="stack-label">启用状态</span>
                <el-switch
                  v-model="row.is_active"
                  @change="toggleStatus(row)"
                />
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
                @click="editPlan(row)"
              >
                编辑
              </el-button>
              <el-button
                size="small"
                class="row-action row-action--danger"
                @click="deletePlan(row)"
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
        @current-change="fetchPlans"
        @size-change="handleSizeChange"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑套餐' : '创建套餐'"
      :width="isMobile ? 'calc(100vw - 24px)' : '600px'"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        :label-width="isMobile ? '82px' : '100px'"
      >
        <el-form-item
          label="名称"
          prop="name"
        >
          <el-input
            v-model="form.name"
            placeholder="请输入套餐名称"
          />
        </el-form-item>
        <el-form-item
          label="描述"
          prop="description"
        >
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="2"
            placeholder="请输入套餐描述"
          />
        </el-form-item>
        <el-form-item
          label="价格(元)"
          prop="price"
        >
          <el-input-number
            v-model="form.price"
            :min="0"
            :precision="2"
          />
        </el-form-item>
        <el-form-item
          label="时长(天)"
          prop="duration"
        >
          <el-input-number
            v-model="form.duration"
            :min="1"
          />
        </el-form-item>
        <el-form-item
          label="流量限制"
          prop="traffic_limit"
        >
          <el-input-number
            v-model="form.traffic_limit"
            :min="0"
            placeholder="0表示无限制"
          />
          <span class="form-unit">GB</span>
        </el-form-item>
        <el-form-item
          label="并发 IP"
          prop="ip_limit"
        >
          <el-input-number
            v-model="form.ip_limit"
            :min="0"
            placeholder="0表示不限制"
          />
        </el-form-item>
        <el-form-item
          label="排序"
          prop="sort_order"
        >
          <el-input-number
            v-model="form.sort_order"
            :min="0"
          />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.is_active" />
        </el-form-item>
        <el-form-item label="功能特性">
          <div class="feature-editor">
            <div class="feature-tags">
              <el-tag
                v-for="(feature, index) in form.features"
                :key="`${feature}-${index}`"
                closable
                @close="removeFeature(index)"
              >
                {{ feature }}
              </el-tag>
            </div>
            <div class="feature-controls">
              <el-input
                v-if="showFeatureInput"
                v-model="newFeature"
                size="small"
                class="feature-input"
                @keyup.enter="addFeature"
                @blur="addFeature"
              />
              <el-button
                v-else
                size="small"
                @click="showFeatureInput = true"
              >
                添加特性
              </el-button>
            </div>
          </div>
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
import { Plus } from '@element-plus/icons-vue'
import { plansApi } from '@/api/index'
import { useViewport } from '@/composables/useViewport'

const { isMobile } = useViewport()

const loading = ref(false)
const plans = ref([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const includeInactive = ref(true)
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

const activePlanCount = computed(() => plans.value.filter((plan) => plan.is_active).length)
const inactivePlanCount = computed(() => plans.value.filter((plan) => !plan.is_active).length)
const unlimitedPlanCount = computed(() => plans.value.filter((plan) => !plan.traffic_limit).length)

const formatPrice = (price) => (Number(price || 0) / 100).toFixed(2)
const formatTraffic = (bytes) => {
  if (!bytes) return '无限制'
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(0)} GB`
}

const getBillingHint = (plan) => {
  const dailyPrice = Number(plan.duration) > 0 ? Number(formatPrice(plan.price)) / Number(plan.duration) : 0
  return `折算日均 ¥${dailyPrice.toFixed(2)}，${plan.is_active ? '当前已上架可售卖。' : '当前处于停用状态。'}`
}

const clearFormValidation = async () => {
  await nextTick()
  formRef.value?.clearValidate()
}

const fetchPlans = async () => {
  loading.value = true
  try {
    const res = await plansApi.admin.list({
      page: pagination.page,
      page_size: pagination.pageSize,
      include_inactive: includeInactive.value
    })
    plans.value = res.plans || []
    pagination.total = res.total || 0
  } catch (e) {
    ElMessage.error(e.message || '获取套餐列表失败')
  } finally {
    loading.value = false
  }
}

const resetForm = () => {
  Object.assign(form, {
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
  showFeatureInput.value = false
  newFeature.value = ''
}

const showCreateDialog = async () => {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
  await clearFormValidation()
}

const editPlan = async (plan) => {
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
  showFeatureInput.value = false
  newFeature.value = ''
  dialogVisible.value = true
  await clearFormValidation()
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

const handleIncludeInactiveChange = async () => {
  pagination.page = 1
  await fetchPlans()
}

const handleSizeChange = async (pageSize) => {
  pagination.page = 1
  pagination.pageSize = pageSize
  await fetchPlans()
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
.admin-plans-page {
  padding: 20px;
}

.plans-table {
  width: 100%;
  min-width: 940px;
}

.form-unit {
  margin-left: 8px;
  font-size: 12px;
  color: #64748b;
}

.feature-editor {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.feature-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.feature-controls {
  display: flex;
  align-items: center;
}

.feature-input {
  width: 220px;
}

@media (max-width: 768px) {
  .admin-plans-page {
    padding: 12px;
  }

  .plans-table {
    min-width: 760px;
  }

  .feature-input {
    width: 100%;
  }
}
</style>
