<template>
  <div class="audit-logs-page">
    <div class="page-header">
      <h1>操作日志</h1>
      <p class="subtitle">
        管理员关键操作（登录、用户管理、设置变更等）的审计记录。在 系统设置 → 日志配置 中可启用/关闭。
      </p>
    </div>

    <el-card v-loading="loading" shadow="never">
      <template #header>
        <div class="card-header">
          <span>共 {{ total }} 条</span>
          <el-button :icon="Refresh" @click="load">
            刷新
          </el-button>
        </div>
      </template>

      <el-table :data="logs" stripe>
        <el-table-column prop="created_at" label="时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="username" label="操作人" width="140" />
        <el-table-column prop="action" label="动作" width="180">
          <template #default="{ row }">
            <el-tag size="small">
              {{ row.action }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="resource_type" label="资源类型" width="120" />
        <el-table-column prop="resource_id" label="资源 ID" width="100" />
        <el-table-column prop="status" label="状态" width="80">
          <template #default="{ row }">
            <el-tag
              :type="row.status === 'success' ? 'success' : 'danger'"
              size="small"
            >
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip_address" label="IP" width="130" />
        <el-table-column prop="details" label="详情" show-overflow-tooltip />
      </el-table>

      <div class="pager">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[20, 50, 100, 200]"
          layout="total, sizes, prev, pager, next"
          @current-change="load"
          @size-change="load"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import api from '@/api/index'
import { extractErrorMessage } from '@/utils/entitlement'

const logs = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const loading = ref(false)

const formatDate = (s) => {
  if (!s) return '-'
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString('zh-CN', { hour12: false })
}

const load = async () => {
  loading.value = true
  try {
    const response = await api.get('/audit-logs', {
      params: { page: page.value, page_size: pageSize.value }
    })
    const data = response?.data || response || {}
    logs.value = data.logs || []
    total.value = data.total || 0
  } catch (error) {
    ElMessage.error('加载操作日志失败：' + (extractErrorMessage(error) || '未知错误'))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.audit-logs-page {
  padding: 20px;
}
.page-header {
  margin-bottom: 24px;
}
.page-header h1 {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 8px;
  color: var(--color-text-primary);
}
.page-header .subtitle {
  font-size: 14px;
  color: var(--color-text-secondary);
  margin: 0;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.pager {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>
