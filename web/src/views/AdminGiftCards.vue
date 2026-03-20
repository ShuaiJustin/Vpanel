<template>
  <div class="admin-giftcards-page">
    <div class="page-header">
      <h1 class="page-title">礼品卡管理</h1>
      <el-button type="primary" @click="showCreateDialog">
        <el-icon><Plus /></el-icon>
        批量创建
      </el-button>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-row">
      <el-card shadow="never" class="stat-card">
        <div class="stat-value">{{ stats.total_cards || 0 }}</div>
        <div class="stat-label">总数量</div>
      </el-card>
      <el-card shadow="never" class="stat-card stat-card--active">
        <div class="stat-value">{{ stats.active_cards || 0 }}</div>
        <div class="stat-label">可用</div>
      </el-card>
      <el-card shadow="never" class="stat-card stat-card--redeemed">
        <div class="stat-value">{{ stats.redeemed_cards || 0 }}</div>
        <div class="stat-label">已兑换</div>
      </el-card>
      <el-card shadow="never" class="stat-card stat-card--value">
        <div class="stat-value">¥{{ formatPrice(stats.active_value || 0) }}</div>
        <div class="stat-label">可用面值</div>
      </el-card>
    </div>

    <!-- 筛选 -->
    <el-card shadow="never" class="filter-card">
      <el-form :inline="true">
        <el-form-item label="状态">
          <el-select v-model="filter.status" placeholder="全部" clearable @change="fetchGiftCards">
            <el-option label="可用" value="active" />
            <el-option label="已兑换" value="redeemed" />
            <el-option label="已过期" value="expired" />
            <el-option label="已禁用" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item label="批次">
          <el-input v-model="filter.batch_id" placeholder="批次ID" clearable @change="fetchGiftCards" />
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 列表 -->
    <el-card shadow="never">
      <el-table :data="giftCards" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="code" label="礼品卡码" width="220">
          <template #default="{ row }">
            <code class="gc-code">{{ row.code }}</code>
            <el-button type="primary" link size="small" @click="copyCode(row.code)">
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="面值" width="100">
          <template #default="{ row }">
            ¥{{ formatPrice(row.value) }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">
              {{ getStatusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="batch_id" label="批次" width="180" />
        <el-table-column label="过期时间" width="180">
          <template #default="{ row }">
            {{ row.expires_at ? formatTime(row.expires_at) : '永不过期' }}
          </template>
        </el-table-column>
        <el-table-column label="兑换时间" width="180">
          <template #default="{ row }">
            {{ row.redeemed_at ? formatTime(row.redeemed_at) : '-' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'active'">
              <el-button type="warning" link @click="disableGiftCard(row)">禁用</el-button>
            </template>
            <template v-else-if="row.status === 'disabled'">
              <el-button type="success" link @click="enableGiftCard(row)">启用</el-button>
            </template>
            <el-button 
              v-if="row.status !== 'redeemed'" 
              type="danger" 
              link 
              @click="deleteGiftCard(row)"
            >删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="pagination.total > 0" class="pagination-container">
        <el-pagination
          v-model:current-page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.pageSize"
          layout="total, prev, pager, next"
          @current-change="fetchGiftCards"
        />
      </div>
    </el-card>

    <!-- 批量创建对话框 -->
    <el-dialog v-model="dialogVisible" title="批量创建礼品卡" width="500px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="数量" prop="count">
          <el-input-number v-model="form.count" :min="1" :max="1000" />
          <span style="margin-left: 8px; color: #909399;">最多1000张</span>
        </el-form-item>
        <el-form-item label="面值" prop="value">
          <el-input-number v-model="form.value" :min="0.01" :precision="2" />
          <span style="margin-left: 8px;">元</span>
        </el-form-item>
        <el-form-item label="前缀" prop="prefix">
          <el-input v-model="form.prefix" placeholder="可选，如 GIFT" maxlength="10" />
        </el-form-item>
        <el-form-item label="过期时间" prop="expires_at">
          <el-date-picker 
            v-model="form.expires_at" 
            type="datetime" 
            placeholder="可选，留空永不过期"
            :disabled-date="disabledDate"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitForm">创建</el-button>
      </template>
    </el-dialog>

    <!-- 创建成功对话框 -->
    <el-dialog v-model="successDialogVisible" title="创建成功" width="600px">
      <div class="success-info">
        <p>成功创建 <strong>{{ createdResult.count }}</strong> 张礼品卡</p>
        <p>批次ID: <code>{{ createdResult.batch_id }}</code></p>
      </div>
      <div class="codes-list">
        <div v-for="gc in createdResult.gift_cards?.slice(0, 10)" :key="gc.id" class="code-item">
          <code>{{ gc.code }}</code>
          <el-button type="primary" link size="small" @click="copyCode(gc.code)">复制</el-button>
        </div>
        <div v-if="createdResult.gift_cards?.length > 10" class="more-hint">
          ... 还有 {{ createdResult.gift_cards.length - 10 }} 张
        </div>
      </div>
      <template #footer>
        <el-button @click="exportCodes">导出全部</el-button>
        <el-button type="primary" @click="successDialogVisible = false">确定</el-button>
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

const formatPrice = (price) => (price / 100).toFixed(2)
const formatTime = (time) => time ? new Date(time).toLocaleString('zh-CN') : '-'
const disabledDate = (date) => date < new Date()
const unwrapPayload = (response) => response?.data ?? response ?? {}

const getStatusType = (status) => {
  const types = { active: 'success', redeemed: 'info', expired: 'warning', disabled: 'danger' }
  return types[status] || 'info'
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
    ElMessage.error(e.response?.data?.error || '获取礼品卡列表失败')
  } finally {
    loading.value = false
  }
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
    ElMessage.error(e.response?.data?.error || '创建失败')
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
    ElMessage.error(e.response?.data?.error || '操作失败')
  }
}

const enableGiftCard = async (gc) => {
  try {
    await giftCardsApi.admin.setStatus(gc.id, { status: 'active' })
    ElMessage.success('已启用')
    fetchGiftCards()
    fetchStats()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '操作失败')
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
    ElMessage.error(e.response?.data?.error || '删除失败')
  }
}

const exportCodes = () => {
  if (!createdResult.value.gift_cards?.length) return
  const codes = createdResult.value.gift_cards.map(gc => gc.code).join('\n')
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

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
}

.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  text-align: center;
}

.stat-card :deep(.el-card__body) {
  padding: 20px;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 8px;
}

.stat-card--active .stat-value {
  color: #67c23a;
}

.stat-card--redeemed .stat-value {
  color: #909399;
}

.stat-card--value .stat-value {
  color: #409eff;
}

.filter-card {
  margin-bottom: 20px;
}

.filter-card :deep(.el-card__body) {
  padding: 16px 20px;
}

.gc-code {
  font-family: monospace;
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.success-info {
  margin-bottom: 16px;
}

.success-info p {
  margin: 8px 0;
}

.success-info code {
  background: #f5f7fa;
  padding: 2px 8px;
  border-radius: 4px;
}

.codes-list {
  max-height: 300px;
  overflow-y: auto;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  padding: 12px;
}

.code-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid #f5f5f5;
}

.code-item:last-child {
  border-bottom: none;
}

.code-item code {
  font-family: monospace;
  font-size: 14px;
}

.more-hint {
  text-align: center;
  color: #909399;
  padding: 12px 0;
}
</style>
