<template>
  <div class="invoices-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        交易凭证
      </h1>
      <p class="page-subtitle">
        下载已生成的订单交易凭证
      </p>
    </div>

    <!-- 发票列表 -->
    <el-card
      shadow="never"
      class="invoices-card"
    >
      <!-- 加载状态 -->
      <div
        v-if="loading"
        class="loading-container"
      >
        <el-skeleton
          :rows="5"
          animated
        />
      </div>

      <el-alert v-else-if="loadError" :title="loadError" type="error" :closable="false">
        <el-button link type="primary" @click="fetchInvoices">重试</el-button>
      </el-alert>

      <el-table
        v-else-if="invoices.length"
        :data="invoices"
        style="width: 100%"
      >
        <el-table-column
          prop="invoice_no"
          label="凭证编号"
          width="180"
        />
        <el-table-column
          label="订单号"
          width="180"
        >
          <template #default="{ row }">{{ row.order_no || `#${row.order_id}` }}</template>
        </el-table-column>
        <el-table-column
          label="金额"
          width="120"
        >
          <template #default="{ row }">
            <span class="amount">¥{{ formatPrice(row.amount) }}</span>
          </template>
        </el-table-column>
        <el-table-column
          prop="created_at"
          label="生成时间"
        />
        <el-table-column
          label="操作"
          width="120"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button
              type="primary"
              link
              :loading="downloading === row.id"
              :disabled="downloading !== null"
              @click="downloadInvoice(row)"
            >
              <el-icon><Download /></el-icon>
              下载 PDF
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 空状态 -->
      <el-empty
        v-if="!loading && !loadError && invoices.length === 0"
        description="暂无交易凭证"
      >
        <template #description>
          <p class="empty-description">如需已支付订单的交易凭证，可联系工作人员生成。</p>
        </template>
        <el-button
          type="primary"
          @click="goToOrders"
        >
          查看订单列表
        </el-button>
      </el-empty>

      <!-- 分页 -->
      <div
        v-if="!loadError && pagination.total > 0"
        class="pagination-container"
      >
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Download } from '@element-plus/icons-vue'
import { invoicesApi } from '@/api/index'
import { extractErrorMessage } from '@/utils/entitlement'

const router = useRouter()

// 状态
const loading = ref(false)
const loadError = ref('')
const invoices = ref([])
const downloading = ref(null)
const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

// 方法
const formatPrice = (price) => (price / 100).toFixed(2)

const goToOrders = () => {
  router.push({ name: 'user-orders' }).catch(error => {
    console.error('跳转到订单页面失败:', error)
  })
}

const fetchInvoices = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const response = await invoicesApi.list({
      page: pagination.page,
      page_size: pagination.pageSize
    })
    invoices.value = response.invoices || []
    pagination.total = response.total || 0
  } catch (error) {
    loadError.value = extractErrorMessage(error) || '获取交易凭证失败'
  } finally {
    loading.value = false
  }
}

const downloadInvoice = async (invoice) => {
  if (downloading.value !== null) return

  downloading.value = invoice.id
  try {
    const blob = await invoicesApi.download(invoice.id)
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `receipt-${invoice.invoice_no}.pdf`
    link.click()
    setTimeout(() => window.URL.revokeObjectURL(url), 1000)
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '下载失败')
  } finally {
    downloading.value = null
  }
}

const handlePageChange = (page) => {
  pagination.page = page
  fetchInvoices()
}

const handleSizeChange = (size) => {
  pagination.pageSize = size
  pagination.page = 1
  fetchInvoices()
}

onMounted(() => {
  fetchInvoices()
})
</script>

<style scoped>
.invoices-page {
  padding: 20px;
  max-width: 1100px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.page-subtitle {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

.invoices-card {
  border-radius: 16px;
}

.invoices-card :deep(.el-card__body) {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.loading-container {
  padding: 20px;
}

.invoices-card :deep(.el-empty) {
  padding: 24px 0 8px;
}

.amount {
  font-weight: 500;
  color: var(--color-text-primary);
}

.empty-description {
  margin: 0 0 12px;
  color: #909399;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 768px) {
  .invoices-page {
    padding: 12px 12px 96px;
  }

  .pagination-container {
    justify-content: center;
  }
}
</style>
