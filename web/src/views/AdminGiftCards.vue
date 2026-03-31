<template>
  <div class="admin-giftcards-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          礼品卡管理
        </h1>
        <p class="page-subtitle">
          统一维护礼品卡批次、状态切换和兑换记录
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
          批量创建
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">总数量</span>
        <strong class="overview-value">{{ stats.total_cards || 0 }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">可用</span>
        <strong class="overview-value is-success">{{ stats.active_cards || 0 }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">已兑换</span>
        <strong class="overview-value is-muted">{{ stats.redeemed_cards || 0 }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">可用面值</span>
        <strong class="overview-value is-primary">¥{{ formatPrice(stats.active_value || 0) }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-filters">
        <el-select
          v-model="filter.status"
          placeholder="状态"
          clearable
        >
          <el-option
            label="可用"
            value="active"
          />
          <el-option
            label="已兑换"
            value="redeemed"
          />
          <el-option
            label="已过期"
            value="expired"
          />
          <el-option
            label="已禁用"
            value="disabled"
          />
        </el-select>
        <el-input
          v-model="filter.batch_id"
          class="toolbar-search"
          placeholder="筛选批次 ID"
          clearable
        />
        <el-button
          type="primary"
          @click="applyFilters"
        >
          筛选
        </el-button>
        <el-button @click="resetFilters">
          重置
        </el-button>
      </div>
      <div class="toolbar-actions">
        <span class="toolbar-summary">当前页 {{ giftCards.length }} 张礼品卡，共 {{ pagination.total }} 张</span>
        <el-button @click="handleRefresh">
          刷新
        </el-button>
      </div>
    </div>

    <div class="table-shell">
      <el-table
        v-loading="loading"
        :data="giftCards"
        border
        stripe
        class="giftcards-table"
        row-key="id"
      >
        <el-table-column
          label="礼品卡信息"
          min-width="320"
        >
          <template #default="{ row }">
            <div class="entity-cell">
              <div class="entity-cell__header">
                <span class="entity-cell__title">礼品卡 #{{ row.id }}</span>
                <span :class="['metric-pill', getStatusPillClass(row.status)]">{{ getStatusLabel(row.status) }}</span>
              </div>
              <div class="entity-cell__meta">
                <span>批次：{{ row.batch_id || '-' }}</span>
              </div>
              <div class="mono-code">
                <span class="mono-code__value">{{ row.code }}</span>
                <el-button
                  text
                  class="inline-copy-btn"
                  @click="copyCode(row.code)"
                >
                  <el-icon><CopyDocument /></el-icon>
                </el-button>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="面值与状态"
          min-width="220"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">面值</span>
                <span class="stack-value is-strong">¥{{ formatPrice(row.value) }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">过期时间</span>
                <span class="stack-value">{{ row.expires_at ? formatTime(row.expires_at) : '永不过期' }}</span>
              </div>
              <div class="entity-cell__hint">
                {{ row.status === 'redeemed' ? '该礼品卡已完成兑换。' : '可用于后台营销、补偿或线下发卡。' }}
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="兑换记录"
          min-width="220"
        >
          <template #default="{ row }">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">兑换时间</span>
                <span class="stack-value">{{ row.redeemed_at ? formatTime(row.redeemed_at) : '未兑换' }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">当前状态</span>
                <span class="stack-value">{{ getStatusLabel(row.status) }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column
          label="操作"
          min-width="170"
          align="right"
          fixed="right"
        >
          <template #default="{ row }">
            <div class="operation-btns">
              <el-button
                v-if="row.status === 'active'"
                size="small"
                class="row-action row-action--warning"
                @click="disableGiftCard(row)"
              >
                禁用
              </el-button>
              <el-button
                v-else-if="row.status === 'disabled'"
                size="small"
                class="row-action row-action--success"
                @click="enableGiftCard(row)"
              >
                启用
              </el-button>
              <el-button
                v-if="row.status !== 'redeemed'"
                size="small"
                class="row-action row-action--danger"
                @click="deleteGiftCard(row)"
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
        @current-change="fetchGiftCards"
        @size-change="handleSizeChange"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      title="批量创建礼品卡"
      :width="isMobile ? 'calc(100vw - 24px)' : '500px'"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        :label-width="isMobile ? '76px' : '100px'"
      >
        <el-form-item
          label="数量"
          prop="count"
        >
          <el-input-number
            v-model="form.count"
            :min="1"
            :max="1000"
          />
          <span class="form-unit">最多 1000 张</span>
        </el-form-item>
        <el-form-item
          label="面值"
          prop="value"
        >
          <el-input-number
            v-model="form.value"
            :min="0.01"
            :precision="2"
          />
          <span class="form-unit">元</span>
        </el-form-item>
        <el-form-item
          label="前缀"
          prop="prefix"
        >
          <el-input
            v-model="form.prefix"
            placeholder="可选，如 GIFT"
            maxlength="10"
          />
        </el-form-item>
        <el-form-item
          label="过期时间"
          prop="expires_at"
        >
          <el-date-picker
            v-model="form.expires_at"
            type="datetime"
            placeholder="可选，留空永不过期"
            :disabled-date="disabledDate"
          />
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
          创建
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="successDialogVisible"
      title="创建成功"
      :width="isMobile ? 'calc(100vw - 24px)' : '600px'"
    >
      <div class="success-info">
        <p>成功创建 <strong>{{ createdResult.count }}</strong> 张礼品卡</p>
        <p>批次ID: <code>{{ createdResult.batch_id }}</code></p>
      </div>
      <div class="codes-list">
        <div
          v-for="gc in createdResult.gift_cards?.slice(0, 10)"
          :key="gc.id"
          class="code-item"
        >
          <code>{{ gc.code }}</code>
          <el-button
            type="primary"
            link
            size="small"
            @click="copyCode(gc.code)"
          >
            复制
          </el-button>
        </div>
        <div
          v-if="createdResult.gift_cards?.length > 10"
          class="more-hint"
        >
          ... 还有 {{ createdResult.gift_cards.length - 10 }} 张
        </div>
      </div>
      <template #footer>
        <el-button @click="exportCodes">
          导出全部
        </el-button>
        <el-button
          type="primary"
          @click="successDialogVisible = false"
        >
          确定
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, CopyDocument } from '@element-plus/icons-vue'
import { giftCardsApi } from '@/api/index'
import { copyText } from '@/utils/clipboard'
import { useViewport } from '@/composables/useViewport'
import { extractErrorMessage } from '@/utils/entitlement'

const { isMobile } = useViewport()

const loading = ref(false)
const giftCards = ref([])
const stats = ref({})
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const filter = reactive({ status: '', batch_id: '' })
const dialogVisible = ref(false)
const successDialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref(null)
const createdResult = ref({})

const form = reactive({
  count: 10,
  value: 100,
  prefix: '',
  expires_at: null
})

const rules = {
  count: [{ required: true, message: '请输入数量', trigger: 'blur' }],
  value: [{ required: true, message: '请输入面值', trigger: 'blur' }]
}

const formatPrice = (price) => (Number(price || 0) / 100).toFixed(2)
const formatTime = (time) => time ? new Date(time).toLocaleString('zh-CN') : '-'
const disabledDate = (date) => date < new Date()
const unwrapPayload = (response) => response?.data ?? response ?? {}

const getStatusType = (status) => {
  const types = { active: 'success', redeemed: 'info', expired: 'warning', disabled: 'danger' }
  return types[status] || 'info'
}

const getStatusPillClass = (status) => {
  const type = getStatusType(status)
  if (type === 'success') return 'is-success'
  if (type === 'warning') return 'is-warning'
  if (type === 'danger') return 'is-danger'
  return 'is-muted'
}

const getStatusLabel = (status) => {
  const labels = { active: '可用', redeemed: '已兑换', expired: '已过期', disabled: '已禁用' }
  return labels[status] || status
}

const copyCode = async (code) => {
  try {
    await copyText(code)
    ElMessage.success('已复制')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

const fetchStats = async () => {
  try {
    const res = await giftCardsApi.admin.getStats()
    stats.value = unwrapPayload(res)
  } catch (e) {
    console.error('Failed to fetch stats:', e)
  }
}

const fetchGiftCards = async () => {
  loading.value = true
  try {
    const params = { page: pagination.page, page_size: pagination.pageSize }
    if (filter.status) params.status = filter.status
    if (filter.batch_id) params.batch_id = filter.batch_id
    const res = await giftCardsApi.admin.list(params)
    const payload = unwrapPayload(res)
    giftCards.value = payload.gift_cards || []
    pagination.total = payload.total || 0
  } catch (e) {
    ElMessage.error(extractErrorMessage(e) || '获取礼品卡列表失败')
  } finally {
    loading.value = false
  }
}

const applyFilters = async () => {
  pagination.page = 1
  await fetchGiftCards()
}

const resetFilters = async () => {
  filter.status = ''
  filter.batch_id = ''
  pagination.page = 1
  await fetchGiftCards()
}

const handleRefresh = async () => {
  await Promise.all([fetchGiftCards(), fetchStats()])
}

const handleSizeChange = async (pageSize) => {
  pagination.page = 1
  pagination.pageSize = pageSize
  await fetchGiftCards()
}

const showCreateDialog = () => {
  Object.assign(form, { count: 10, value: 100, prefix: '', expires_at: null })
  dialogVisible.value = true
}

const submitForm = async () => {
  await formRef.value.validate()
  submitting.value = true
  try {
    const data = {
      count: form.count,
      value: Math.round(form.value * 100),
      prefix: form.prefix || undefined,
      expires_at: form.expires_at?.toISOString()
    }
    const res = await giftCardsApi.admin.createBatch(data)
    createdResult.value = unwrapPayload(res)
    dialogVisible.value = false
    successDialogVisible.value = true
    fetchGiftCards()
    fetchStats()
  } catch (e) {
    ElMessage.error(extractErrorMessage(e) || '创建失败')
  } finally {
    submitting.value = false
  }
}

const disableGiftCard = async (gc) => {
  await ElMessageBox.confirm('确定要禁用此礼品卡吗？', '提示', { type: 'warning' })
  try {
    await giftCardsApi.admin.setStatus(gc.id, { status: 'disabled' })
    ElMessage.success('已禁用')
    fetchGiftCards()
    fetchStats()
  } catch (e) {
    ElMessage.error(extractErrorMessage(e) || '操作失败')
  }
}

const enableGiftCard = async (gc) => {
  try {
    await giftCardsApi.admin.setStatus(gc.id, { status: 'active' })
    ElMessage.success('已启用')
    fetchGiftCards()
    fetchStats()
  } catch (e) {
    ElMessage.error(extractErrorMessage(e) || '操作失败')
  }
}

const deleteGiftCard = async (gc) => {
  await ElMessageBox.confirm('确定要删除此礼品卡吗？', '提示', { type: 'warning' })
  try {
    await giftCardsApi.admin.delete(gc.id)
    ElMessage.success('已删除')
    fetchGiftCards()
    fetchStats()
  } catch (e) {
    ElMessage.error(extractErrorMessage(e) || '删除失败')
  }
}

const exportCodes = () => {
  if (!createdResult.value.gift_cards?.length) return
  const codes = createdResult.value.gift_cards.map((gc) => gc.code).join('\n')
  const blob = new Blob([codes], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `gift-cards-${createdResult.value.batch_id}.txt`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(() => {
  fetchGiftCards()
  fetchStats()
})
</script>

<style scoped>
.admin-giftcards-page {
  padding: 20px;
}

.giftcards-table {
  width: 100%;
  min-width: 980px;
}

.form-unit {
  margin-left: 8px;
  font-size: 12px;
  color: #64748b;
}

@media (max-width: 768px) {
  .admin-giftcards-page {
    padding: 12px;
  }

  .giftcards-table {
    min-width: 760px;
  }
}
</style>
