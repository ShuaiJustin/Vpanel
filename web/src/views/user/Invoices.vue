<template>
  <div class="invoices-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        我的发票
      </h1>
      <p class="page-subtitle">
        查看和下载您的发票
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

      <!-- 发票表格 -->
      <el-table
        v-else
        :data="invoices"
        style="width: 100%"
      >
        <el-table-column
          prop="invoice_no"
          label="发票号"
          width="180"
        />
        <el-table-column
          prop="order_no"
          label="订单号"
          width="180"
        />
        <el-table-column
          label="金额"
          width="120"
        >
          <template #default="{ row }">
            <span class="amount">¥{{ formatPrice(row.amount) }}</span>
          </template>
        </el-table-column>
        <el-table-column
          label="状态"
          width="100"
        >
          <template #default="{ row }">
            <el-tag
              :type="getStatusType(row.status)"
              size="small"
            >
              {{ getStatusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="created_at"
          label="开票时间"
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
              @click="downloadInvoice(row)"
            >
              <el-icon><Download /></el-icon>
              下载
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 空状态 -->
      <el-empty
        v-if="!loading && invoices.length === 0"
        description="暂无发票"
      />

      <!-- 分页 -->
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
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Download } from '@element-plus/icons-vue'
import { invoicesApi } from '@/api/index'

// 状态
const loading = ref(false)
const invoices = ref([])
const downloading = ref(null)
const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

// 方法
const formatPrice = (price) => (price / 100).toFixed(2)

const getStatusType = (status) => {
  const types = {
    pending: 'warning',
    generated: 'success',
    failed: 'danger'
  }
  return types[status] || 'info'
}

const getStatusLabel = (status) => {
  const labels = {
    pending: '生成中',
    generated: '已生成',
    failed: '生成失败'
  }
  return labels[status] || status
}

const fetchInvoices = async () => {
  loading.value = true
  try {
    const response = await invoicesApi.list({
      page: pagination.page,
      page_size: pagination.pageSize
    })
    invoices.value = response.invoices || []
    pagination.total = response.total || 0
  } catch (error) {
    ElMessage.error(error.message || '获取发票列表失败')
  } finally {
    loading.value = false
  }
}

const downloadInvoice = async (invoice) => {
  if (invoice.status !== 'generated') {
    ElMessage.warning('发票尚未生成')
    return
  }

  downloading.value = invoice.id
  try {
    const blob = await invoicesApi.download(invoice.id)
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `invoice-${invoice.invoice_no}.pdf`
    link.click()
    window.URL.revokeObjectURL(url)
    ElMessage.success('发票下载成功')
  } catch (error) {
    ElMessage.error(error.message || '下载失败')
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
  border-radius: 8px;
}

.loading-container {
  padding: 20px;
}

.amount {
  font-weight: 500;
  color: var(--color-text-primary);
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
