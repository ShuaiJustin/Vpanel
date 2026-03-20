<template>
  <div class="admin-subscriptions-container">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>订阅管理</span>
          <div class="header-actions">
            <el-input
              v-model="searchQuery"
              placeholder="搜索用户ID"
              clearable
              style="width: 200px; margin-right: 10px;"
              @keyup.enter="handleSearch"
              @clear="handleSearch"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-select
              v-model="filterAccessCount"
              placeholder="访问次数过滤"
              clearable
              style="width: 150px;"
              @change="handleFilterChange"
            >
              <el-option label="全部" value="" />
              <el-option label="从未访问" value="0" />
              <el-option label="1-10次" value="1-10" />
              <el-option label="10-100次" value="10-100" />
              <el-option label="100次以上" value="100+" />
            </el-select>
            <el-button type="primary" @click="handleSearch">查询</el-button>
          </div>
        </div>
      </template>

      <el-table
        :data="subscriptions"
        v-loading="loading"
        style="width: 100%"
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="user_id" label="用户ID" width="100" />
        <el-table-column prop="username" label="用户名" width="150">
          <template #default="{ row }">
            <span>{{ row.username || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="token" label="令牌" min-width="200">
          <template #default="{ row }">
            <div class="token-cell">
              <span class="token-text">{{ maskToken(row.token) }}</span>
              <el-button link type="primary" size="small" @click="copyToken(row.token)">
                <el-icon><DocumentCopy /></el-icon>
              </el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="short_code" label="短码" width="120">
          <template #default="{ row }">
            <span>{{ row.short_code || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="access_count" label="访问次数" width="120">
          <template #default="{ row }">
            <el-tag :type="getAccessCountType(row.access_count)">
              {{ row.access_count || 0 }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_access_at" label="最后访问" width="180">
          <template #default="{ row }">
            <span>{{ formatDate(row.last_access_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="last_ip" label="最后IP" width="140">
          <template #default="{ row }">
            <span>{{ row.last_ip || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            <span>{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button size="small" type="warning" @click="handleResetStats(row)">
                重置统计
              </el-button>
              <el-button size="small" type="danger" @click="handleRevoke(row)">
                撤销
              </el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-container">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          :total="total"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, DocumentCopy } from '@element-plus/icons-vue'
import { subscriptionApi } from '@/api/index'

// 状态
const route = useRoute()
const subscriptions = ref([])
const loading = ref(false)
const searchQuery = ref('')
const filterAccessCount = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

const buildAccessCountParams = () => {
  switch (filterAccessCount.value) {
    case '0':
      return { max_access_count: 0 }
    case '1-10':
      return { min_access_count: 1, max_access_count: 10 }
    case '10-100':
      return { min_access_count: 11, max_access_count: 100 }
    case '100+':
      return { min_access_count: 101 }
    default:
      return {}
  }
}

// 方法
const fetchSubscriptions = async () => {
  loading.value = true
  try {
    const params = {
      page: currentPage.value,
      page_size: pageSize.value
    }

    if (searchQuery.value) {
      if (!/^\d+$/.test(searchQuery.value.trim())) {
        ElMessage.warning('当前仅支持按用户ID搜索订阅')
        subscriptions.value = []
        total.value = 0
        return
      }

      params.user_id = Number(searchQuery.value.trim())
    }

    Object.assign(params, buildAccessCountParams())

    const response = await subscriptionApi.admin.list(params)

    if (response && response.subscriptions) {
      subscriptions.value = response.subscriptions
      total.value = response.total || response.subscriptions.length
    } else if (Array.isArray(response)) {
      subscriptions.value = response
      total.value = response.length
    } else {
      subscriptions.value = []
      total.value = 0
    }
  } catch (error) {
    console.error('获取订阅列表失败:', error)
    ElMessage.error('获取订阅列表失败')
    subscriptions.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  currentPage.value = 1
  fetchSubscriptions()
}

const handleFilterChange = () => {
  currentPage.value = 1
  fetchSubscriptions()
}

const handleSizeChange = (val) => {
  pageSize.value = val
  currentPage.value = 1
  fetchSubscriptions()
}

const handleCurrentChange = (val) => {
  currentPage.value = val
  fetchSubscriptions()
}

const handleResetStats = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要重置用户 "${row.username || row.user_id}" 的订阅访问统计吗？`,
      '确认重置',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    await subscriptionApi.admin.resetStats(row.user_id)
    ElMessage.success('访问统计已重置')
    fetchSubscriptions()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('重置统计失败:', error)
      ElMessage.error('重置统计失败')
    }
  }
}

const handleRevoke = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要撤销用户 "${row.username || row.user_id}" 的订阅吗？撤销后用户需要重新获取订阅链接。`,
      '确认撤销',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'danger'
      }
    )
    
    await subscriptionApi.admin.revoke(row.user_id)
    ElMessage.success('订阅已撤销')
    fetchSubscriptions()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('撤销订阅失败:', error)
      ElMessage.error('撤销订阅失败')
    }
  }
}

const maskToken = (token) => {
  if (!token) return '-'
  if (token.length <= 8) return token
  return token.substring(0, 4) + '****' + token.substring(token.length - 4)
}

const copyToken = async (token) => {
  try {
    await navigator.clipboard.writeText(token)
    ElMessage.success('令牌已复制到剪贴板')
  } catch (error) {
    const textarea = document.createElement('textarea')
    textarea.value = token
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    ElMessage.success('令牌已复制到剪贴板')
  }
}

const getAccessCountType = (count) => {
  if (!count || count === 0) return 'info'
  if (count < 10) return 'success'
  if (count < 100) return 'warning'
  return 'danger'
}

const formatDate = (dateStr) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

// 生命周期
onMounted(() => {
  if (route.query.user_id) {
    searchQuery.value = String(route.query.user_id)
  }
  fetchSubscriptions()
})
</script>

<style scoped>
.admin-subscriptions-container {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  align-items: center;
}

.token-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.token-text {
  font-family: monospace;
  font-size: 13px;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}
</style>
