<template>
  <div class="logs-container">
    <div class="header">
      <div class="page-heading">
        <h1>日志管理</h1>
        <p class="page-subtitle">
          按级别、来源和时间范围快速定位后台运行问题
        </p>
      </div>
      <div class="actions">
        <el-button
          type="primary"
          @click="refreshLogs"
        >
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button
          type="success"
          @click="handleExport"
        >
          <el-icon><Download /></el-icon>
          导出
        </el-button>
        <el-button
          type="warning"
          @click="showCleanupDialog"
        >
          <el-icon><Delete /></el-icon>
          清理
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">当前筛选总量</span>
        <strong class="overview-value">{{ total }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页日志</span>
        <strong class="overview-value is-primary">{{ logs.length }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页错误</span>
        <strong class="overview-value is-danger">{{ currentErrorCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">当前页告警</span>
        <strong class="overview-value is-warning">{{ currentWarningCount }}</strong>
      </div>
    </div>

    <!-- 过滤控件 -->
    <el-card class="filter-card">
      <el-form
        :inline="!isMobile"
        :model="filterForm"
        class="filter-form"
      >
        <el-form-item label="日志级别">
          <el-select
            v-model="filterForm.level"
            placeholder="全部级别"
            clearable
            class="filter-select"
          >
            <el-option
              label="Debug"
              value="debug"
            />
            <el-option
              label="Info"
              value="info"
            />
            <el-option
              label="Warn"
              value="warn"
            />
            <el-option
              label="Error"
              value="error"
            />
            <el-option
              label="Fatal"
              value="fatal"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="日志来源">
          <el-input
            v-model="filterForm.source"
            placeholder="来源"
            clearable
            class="filter-source"
          />
        </el-form-item>
        <el-form-item label="关键词">
          <el-input
            v-model="filterForm.keyword"
            placeholder="搜索关键词"
            clearable
            class="filter-keyword"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="filterForm.dateRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            class="filter-range"
          />
        </el-form-item>
        <el-form-item class="filter-actions">
          <el-button
            type="primary"
            @click="handleFilter"
          >
            查询
          </el-button>
          <el-button @click="resetFilter">
            重置
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 日志列表 -->
    <div class="table-shell">
      <el-table
        v-loading="loading"
        :data="logs"
        border
        class="logs-table"
        :row-class-name="getRowClassName"
        @row-click="handleRowClick"
      >
        <el-table-column
          prop="id"
          label="ID"
          width="80"
        />
        <el-table-column
          prop="level"
          label="级别"
          width="100"
        >
          <template #default="scope">
            <el-tag
              :type="getLevelTagType(scope.row.level)"
              size="small"
            >
              {{ (scope.row.level || '').toUpperCase() }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="source"
          label="来源"
          width="150"
        />
        <el-table-column
          prop="message"
          label="消息"
          min-width="300"
          show-overflow-tooltip
        />
        <el-table-column
          prop="created_at"
          label="时间"
          width="180"
        >
          <template #default="scope">
            {{ formatDateTime(scope.row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column
          label="操作"
          width="100"
          fixed="right"
        >
          <template #default="scope">
            <el-button
              type="primary"
              link
              size="small"
              @click.stop="showLogDetail(scope.row)"
            >
              详情
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 分页控件 -->
    <div class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[20, 50, 100, 200]"
        :layout="isMobile ? 'total, prev, next' : 'total, sizes, prev, pager, next, jumper'"
        :total="total"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>

    <!-- 日志详情弹窗 -->
    <el-dialog
      v-model="detailDialogVisible"
      title="日志详情"
      :width="detailDialogWidth"
      class="log-detail-dialog"
      destroy-on-close
    >
      <div
        v-if="selectedLog"
        class="log-detail-content"
      >
        <el-descriptions
          :column="detailDescriptionColumns"
          border
          class="log-detail-meta"
        >
          <el-descriptions-item label="ID">
            {{ selectedLog.id }}
          </el-descriptions-item>
          <el-descriptions-item label="级别">
            <el-tag
              :type="getLevelTagType(selectedLog.level)"
              size="small"
            >
              {{ (selectedLog.level || '').toUpperCase() }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="来源">
            {{ selectedLog.source }}
          </el-descriptions-item>
          <el-descriptions-item label="时间">
            {{ formatDateTime(selectedLog.created_at) }}
          </el-descriptions-item>
          <el-descriptions-item
            v-if="selectedLog.user_id"
            label="用户 ID"
          >
            {{ selectedLog.user_id }}
          </el-descriptions-item>
          <el-descriptions-item
            v-if="selectedLog.ip"
            label="IP 地址"
          >
            {{ selectedLog.ip }}
          </el-descriptions-item>
          <el-descriptions-item
            v-if="selectedLog.user_agent"
            label="User Agent"
            :span="detailDescriptionColumns"
          >
            <div class="log-detail-text">
              {{ selectedLog.user_agent }}
            </div>
          </el-descriptions-item>
          <el-descriptions-item
            v-if="selectedLog.request_id"
            label="请求 ID"
            :span="detailDescriptionColumns"
          >
            <div class="log-detail-text">
              {{ selectedLog.request_id }}
            </div>
          </el-descriptions-item>
        </el-descriptions>
        <section class="log-detail-section">
          <div class="log-detail-section-title">
            消息
          </div>
          <pre class="log-message">{{ selectedLog.message }}</pre>
        </section>
        <section
          v-if="formattedLogFields"
          class="log-detail-section"
        >
          <div class="log-detail-section-title">
            附加字段
          </div>
          <pre class="log-fields">{{ formattedLogFields }}</pre>
        </section>
      </div>
    </el-dialog>

    <!-- 清理对话框 -->
    <el-dialog
      v-model="cleanupDialogVisible"
      title="清理日志"
      :width="compactDialogWidth"
    >
      <el-form
        :model="cleanupForm"
        :label-width="isMobile ? '88px' : '100px'"
      >
        <el-form-item label="保留天数">
          <el-input-number
            v-model="cleanupForm.retentionDays"
            :min="1"
            :max="365"
          />
          <span style="margin-left: 10px; color: #909399;">天</span>
        </el-form-item>
        <el-alert
          type="warning"
          :closable="false"
          show-icon
          style="margin-top: 10px"
        >
          此操作将删除 {{ cleanupForm.retentionDays }} 天前的所有日志，不可恢复！
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="cleanupDialogVisible = false">
          取消
        </el-button>
        <el-button
          type="danger"
          :loading="cleanupLoading"
          @click="handleCleanup"
        >
          确认清理
        </el-button>
      </template>
    </el-dialog>

    <!-- 导出对话框 -->
    <el-dialog
      v-model="exportDialogVisible"
      title="导出日志"
      :width="compactDialogWidth"
    >
      <el-form
        :model="exportForm"
        :label-width="isMobile ? '88px' : '100px'"
      >
        <el-form-item label="导出格式">
          <el-radio-group v-model="exportForm.format">
            <el-radio value="json">
              JSON
            </el-radio>
            <el-radio value="csv">
              CSV
            </el-radio>
          </el-radio-group>
        </el-form-item>
        <el-alert
          type="info"
          :closable="false"
          show-icon
          style="margin-top: 10px"
        >
          将导出当前筛选条件下的所有日志
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="exportDialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="exportLoading"
          @click="confirmExport"
        >
          导出
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh, Download, Delete } from '@element-plus/icons-vue'
import { logsApi } from '@/api'
import { useViewport } from '@/composables/useViewport'

const { isMobile } = useViewport()

// 状态
const logs = ref([])
const loading = ref(false)
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(50)

// 过滤表单
const filterForm = reactive({
  level: '',
  source: '',
  keyword: '',
  dateRange: null
})

// 详情弹窗
const detailDialogVisible = ref(false)
const selectedLog = ref(null)
const formattedLogFields = computed(() => {
  if (!selectedLog.value?.fields || selectedLog.value.fields === '{}') {
    return ''
  }

  if (typeof selectedLog.value.fields === 'string') {
    try {
      return JSON.stringify(JSON.parse(selectedLog.value.fields), null, 2)
    } catch {
      return selectedLog.value.fields
    }
  }

  return JSON.stringify(selectedLog.value.fields, null, 2)
})

const detailDialogWidth = computed(() => (isMobile.value ? '94%' : '760px'))
const compactDialogWidth = computed(() => (isMobile.value ? '92%' : '400px'))
const detailDescriptionColumns = computed(() => (isMobile.value ? 1 : 2))

// 清理弹窗
const cleanupDialogVisible = ref(false)
const cleanupLoading = ref(false)
const cleanupForm = reactive({
  retentionDays: 30
})

// 导出弹窗
const exportDialogVisible = ref(false)
const exportLoading = ref(false)
const exportForm = reactive({
  format: 'json'
})

const currentErrorCount = computed(() =>
  logs.value.filter((item) => item.level === 'error' || item.level === 'fatal').length
)
const currentWarningCount = computed(() =>
  logs.value.filter((item) => item.level === 'warn').length
)

// 生命周期
onMounted(() => {
  fetchLogs()
})

// 获取日志列表
const fetchLogs = async () => {
  loading.value = true
  try {
    const params = {
      page: currentPage.value,
      page_size: pageSize.value
    }

    if (filterForm.level) {
      params.level = filterForm.level
    }
    if (filterForm.source) {
      params.source = filterForm.source
    }
    if (filterForm.keyword) {
      params.keyword = filterForm.keyword
    }
    if (filterForm.dateRange && filterForm.dateRange.length === 2) {
      params.start_time = filterForm.dateRange[0]
      params.end_time = filterForm.dateRange[1]
    }

    const response = await logsApi.getLogs(params)
    logs.value = response.logs || []
    total.value = response.total || 0
  } catch (error) {
    if (!error.cancelled) {
      console.error('Failed to fetch logs:', error)
      ElMessage.error('获取日志列表失败')
    }
  } finally {
    loading.value = false
  }
}

// 刷新日志
const refreshLogs = () => {
  fetchLogs()
}

// 过滤查询
const handleFilter = () => {
  currentPage.value = 1
  fetchLogs()
}

// 重置过滤
const resetFilter = () => {
  filterForm.level = ''
  filterForm.source = ''
  filterForm.keyword = ''
  filterForm.dateRange = null
  currentPage.value = 1
  fetchLogs()
}

// 分页处理
const handleSizeChange = (val) => {
  pageSize.value = val
  currentPage.value = 1
  fetchLogs()
}

const handleCurrentChange = (val) => {
  currentPage.value = val
  fetchLogs()
}

// 行点击
const handleRowClick = (row) => {
  showLogDetail(row)
}

// 显示日志详情
const showLogDetail = async (log) => {
  try {
    const response = await logsApi.getLog(log.id)
    selectedLog.value = response
    detailDialogVisible.value = true
  } catch (error) {
    if (!error.cancelled) {
      console.error('Failed to fetch log detail:', error)
      ElMessage.error('获取日志详情失败')
    }
  }
}

// 显示清理对话框
const showCleanupDialog = () => {
  cleanupDialogVisible.value = true
}

// 执行清理
const handleCleanup = async () => {
  cleanupLoading.value = true
  try {
    const response = await logsApi.cleanup({
      retention_days: cleanupForm.retentionDays
    })

    const deletedCount = Number(response.deleted_count ?? response.deleted ?? 0)

    ElMessage.success(`清理完成，共删除 ${deletedCount} 条日志`)
    cleanupDialogVisible.value = false
    await fetchLogs()
  } catch (error) {
    if (!error.cancelled) {
      console.error('Failed to cleanup logs:', error)
      ElMessage.error(error.message || '清理日志失败')
    }
  } finally {
    cleanupLoading.value = false
  }
}

// 显示导出对话框
const handleExport = () => {
  exportDialogVisible.value = true
}

// 确认导出
const confirmExport = async () => {
  exportLoading.value = true
  try {
    const params = {
      format: exportForm.format
    }

    if (filterForm.level) {
      params.level = filterForm.level
    }
    if (filterForm.source) {
      params.source = filterForm.source
    }
    if (filterForm.keyword) {
      params.keyword = filterForm.keyword
    }
    if (filterForm.dateRange && filterForm.dateRange.length === 2) {
      params.start_time = filterForm.dateRange[0]
      params.end_time = filterForm.dateRange[1]
    }

    const response = await logsApi.exportLogs(params)
    
    // 创建下载链接
    const blob = new Blob([response], {
      type: exportForm.format === 'json' ? 'application/json' : 'text/csv'
    })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `logs_${new Date().toISOString().slice(0, 10)}.${exportForm.format}`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)

    ElMessage.success('导出成功')
    exportDialogVisible.value = false
  } catch (error) {
    if (!error.cancelled) {
      console.error('Failed to export logs:', error)
      ElMessage.error('导出日志失败')
    }
  } finally {
    exportLoading.value = false
  }
}

// 获取级别标签类型
const getLevelTagType = (level) => {
  const types = {
    debug: 'info',
    info: 'success',
    warn: 'warning',
    error: 'danger',
    fatal: 'danger'
  }
  return types[level] || 'info'
}

// 获取行样式类名
const getRowClassName = ({ row }) => {
  if (row.level === 'error' || row.level === 'fatal') {
    return 'log-row-error'
  }
  return ''
}

// 格式化日期时间
const formatDateTime = (dateStr) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}
</script>

<style scoped>
.logs-container {
  padding: 20px;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.filter-card {
  margin-bottom: 20px;
}

.filter-form {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.filter-select {
  width: 120px;
}

.filter-source {
  width: 150px;
}

.filter-keyword {
  width: 200px;
}

.filter-range {
  width: 360px;
}

.table-shell {
  overflow-x: auto;
}

.logs-table {
  min-width: 900px;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}

.log-message {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: anywhere;
  max-height: 240px;
  overflow: auto;
  line-height: 1.6;
}

.log-fields {
  background-color: #f5f7fa;
  padding: 12px;
  border-radius: 8px;
  margin: 0;
  max-height: 280px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: anywhere;
}

.log-detail-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.log-detail-text {
  white-space: normal;
  word-break: break-word;
  overflow-wrap: anywhere;
  line-height: 1.6;
}

.log-detail-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.log-detail-section-title {
  font-size: 14px;
  font-weight: 600;
  color: #374151;
}

:deep(.log-detail-dialog .el-dialog) {
  max-width: calc(100vw - 32px);
}

:deep(.log-detail-dialog .el-dialog__body) {
  max-height: min(72vh, 720px);
  overflow: auto;
}

:deep(.log-detail-meta .el-descriptions__table) {
  table-layout: fixed;
  width: 100%;
}

:deep(.log-detail-meta .el-descriptions__cell) {
  word-break: break-word;
  overflow-wrap: anywhere;
}

@media (max-width: 768px) {
  .logs-container {
    padding: 12px;
  }

  .header {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .actions {
    width: 100%;
  }

  .actions .el-button {
    flex: 1 1 calc(50% - 6px);
    min-width: 0;
  }

  .filter-form {
    flex-direction: column;
    align-items: stretch;
  }

  .filter-form :deep(.el-form-item) {
    width: 100%;
    margin-right: 0;
    margin-bottom: 0;
  }

  .filter-select,
  .filter-source,
  .filter-keyword,
  .filter-range {
    width: 100%;
  }

  .filter-actions :deep(.el-form-item__content) {
    width: 100%;
    justify-content: flex-start;
    gap: 8px;
  }

  .filter-actions .el-button {
    flex: 1 1 0;
  }

  :deep(.logs-table .cell) {
    padding: 12px 10px;
  }

  .log-detail-section-title {
    font-size: 13px;
  }

  .pagination-container {
    justify-content: center;
    overflow-x: auto;
  }
}

:deep(.log-row-error) {
  background-color: #fef0f0 !important;
}

:deep(.log-row-error:hover > td) {
  background-color: #fde2e2 !important;
}
</style>
