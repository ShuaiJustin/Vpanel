<template>
  <div class="admin-subscriptions-page">
    <div class="page-header">
      <div class="page-heading">
        <div class="title">
          订阅管理
        </div>
        <div class="page-subtitle">
          统一查看订阅凭据、访问活跃度和最后使用记录
        </div>
      </div>
      <el-button
        type="primary"
        class="refresh-btn"
        @click="fetchSubscriptions"
      >
        <el-icon class="el-icon--left">
          <RefreshRight />
        </el-icon> 刷新列表
      </el-button>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">当前匹配</span>
        <strong class="overview-value">{{ filteredSubscriptions.length }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">已访问</span>
        <strong class="overview-value is-success">{{ visitedCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">从未访问</span>
        <strong class="overview-value is-muted">{{ neverVisitedCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">近7天活跃</span>
        <strong class="overview-value is-primary">{{ recentActiveCount }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-filters">
        <el-input
          v-model="filters.keyword"
          clearable
          class="toolbar-search"
          placeholder="搜索用户ID、用户名、短码、IP或令牌"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select
          v-model="filters.accessRange"
          clearable
          placeholder="访问次数"
        >
          <el-option
            label="从未访问"
            value="0"
          />
          <el-option
            label="1-10 次"
            value="1-10"
          />
          <el-option
            label="11-100 次"
            value="11-100"
          />
          <el-option
            label="100 次以上"
            value="100+"
          />
        </el-select>
        <el-select
          v-model="filters.activity"
          clearable
          placeholder="活跃状态"
        >
          <el-option
            label="从未访问"
            value="never"
          />
          <el-option
            label="近 7 天活跃"
            value="recent"
          />
          <el-option
            label="30 天未访问"
            value="stale"
          />
        </el-select>
        <el-select
          v-model="sortKey"
          placeholder="排序方式"
        >
          <el-option
            label="最近访问优先"
            value="recent_access"
          />
          <el-option
            label="访问次数优先"
            value="access_desc"
          />
          <el-option
            label="创建时间优先"
            value="created_desc"
          />
          <el-option
            label="用户 ID 优先"
            value="user_desc"
          />
        </el-select>
        <el-button @click="resetFilters">
          重置
        </el-button>
        <el-button @click="fetchSubscriptions">
          刷新
        </el-button>
      </div>
      <div class="toolbar-summary">
        总记录 {{ total }} 条，当前筛选 {{ filteredSubscriptions.length }} 条
      </div>
    </div>

    <el-table
      v-loading="loading"
      :data="paginatedSubscriptions"
      border
      stripe
      class="subscriptions-table"
      row-key="id"
      :empty-text="filteredSubscriptions.length ? '当前页暂无数据' : (hasActiveFilters ? '暂无匹配的订阅' : '暂无订阅记录')"
    >
      <el-table-column
        label="订阅对象"
        min-width="230"
      >
        <template #default="{ row }">
          <div class="user-cell">
            <div class="user-cell__header">
              <span
                class="user-name"
                :title="row.username_display"
              >{{ row.username_display }}</span>
              <span :class="['activity-pill', row.activity_class]">
                {{ row.activity_label }}
              </span>
            </div>
            <div class="user-cell__meta">
              <span>用户ID：{{ row.user_id }}</span>
              <span>订阅ID：{{ row.id }}</span>
              <span>创建：{{ row.created_at_display }}</span>
            </div>
            <div class="user-cell__hint">
              {{ row.activity_hint }}
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column
        label="订阅凭据"
        min-width="290"
      >
        <template #default="{ row }">
          <div class="credential-cell">
            <div class="credential-item">
              <span class="credential-label">令牌</span>
              <div class="credential-main">
                <span
                  class="credential-value"
                  :title="row.token"
                >{{ maskToken(row.token) }}</span>
                <el-button
                  text
                  class="copy-token-btn"
                  @click="copyToken(row.token)"
                >
                  <el-icon><DocumentCopy /></el-icon>
                </el-button>
              </div>
            </div>
            <div class="credential-item">
              <span class="credential-label">短码</span>
              <div class="credential-main">
                <span
                  class="credential-value"
                  :title="row.short_code || '未设置'"
                >
                  {{ row.short_code || '未设置' }}
                </span>
                <el-button
                  text
                  class="copy-token-btn"
                  :disabled="!row.short_code"
                  @click="copyShortCode(row.short_code)"
                >
                  <el-icon><DocumentCopy /></el-icon>
                </el-button>
              </div>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column
        label="访问情况"
        min-width="220"
      >
        <template #default="{ row }">
          <div class="detail-cell">
            <div class="detail-item">
              <span class="detail-label">访问次数</span>
              <span :class="['access-badge', row.activity_class]">
                {{ row.access_count }}
              </span>
            </div>
            <div class="detail-item">
              <span class="detail-label">最后访问</span>
              <span class="detail-value">{{ row.last_access_display }}</span>
            </div>
            <div class="detail-item">
              <span class="detail-label">最后 IP</span>
              <span class="detail-value">{{ row.last_ip || '-' }}</span>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column
        label="操作"
        width="126"
        align="right"
        fixed="right"
      >
        <template #default="{ row }">
          <div class="operation-btns">
            <el-button
              size="small"
              class="row-action row-action--warning"
              @click="handleResetStats(row)"
            >
              重置
            </el-button>
            <el-button
              size="small"
              class="row-action row-action--danger"
              @click="handleRevoke(row)"
            >
              撤销
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        :total="filteredSubscriptions.length"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { DocumentCopy, RefreshRight, Search } from '@element-plus/icons-vue'
import { subscriptionApi } from '@/api/index'

const route = useRoute()

const subscriptions = ref([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const sortKey = ref('recent_access')
const filters = reactive({
  keyword: '',
  accessRange: '',
  activity: ''
})

const unwrapPayload = (response) => response?.data ?? response ?? {}
const normalizeString = (value) => typeof value === 'string' ? value.trim() : ''

const toTimestamp = (value) => {
  const normalized = normalizeString(value)
  if (!normalized) return 0

  const timestamp = new Date(normalized).getTime()
  return Number.isFinite(timestamp) ? timestamp : 0
}

const formatDate = (dateStr) => {
  const timestamp = toTimestamp(dateStr)
  if (!timestamp) return '-'

  const date = new Date(timestamp)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

const isWithinDays = (dateStr, days) => {
  const timestamp = toTimestamp(dateStr)
  if (!timestamp) return false
  return (Date.now() - timestamp) <= days * 24 * 60 * 60 * 1000
}

const getActivityProfile = (row = {}) => {
  const accessCount = Number(row.access_count || 0)
  const lastAccessAt = row.last_access_at

  if (!accessCount) {
    return { label: '未访问', className: 'dormant' }
  }

  if (accessCount >= 100) {
    return { label: '高频使用', className: 'intense' }
  }

  if (isWithinDays(lastAccessAt, 7)) {
    return { label: '近期活跃', className: 'active' }
  }

  return { label: '已使用', className: 'steady' }
}

const getActivityHint = (row = {}) => {
  const accessCount = Number(row.access_count || 0)

  if (!accessCount) {
    return '尚未拉取过订阅'
  }

  if (isWithinDays(row.last_access_at, 7)) {
    return '最近 7 天内有访问记录'
  }

  if (isWithinDays(row.last_access_at, 30)) {
    return '最近 30 天内访问过'
  }

  return '超过 30 天未访问'
}

const normalizeSubscription = (item = {}) => {
  const accessCount = Number(item.access_count || 0)
  const lastAccessAt = normalizeString(item.last_access_at)
  const profile = getActivityProfile({
    access_count: accessCount,
    last_access_at: lastAccessAt
  })

  return {
    ...item,
    user_id: item.user_id,
    username_display: normalizeString(item.username) || `用户 #${item.user_id ?? '-'}`,
    token: normalizeString(item.token),
    short_code: normalizeString(item.short_code),
    access_count: Number.isFinite(accessCount) ? accessCount : 0,
    last_ip: normalizeString(item.last_ip),
    created_at: normalizeString(item.created_at),
    last_access_at: lastAccessAt,
    created_at_display: formatDate(item.created_at),
    last_access_display: formatDate(lastAccessAt),
    activity_label: profile.label,
    activity_class: profile.className,
    activity_hint: getActivityHint({
      access_count: accessCount,
      last_access_at: lastAccessAt
    })
  }
}

const matchesAccessRange = (count, range) => {
  if (!range) return true
  if (range === '0') return count === 0
  if (range === '1-10') return count >= 1 && count <= 10
  if (range === '11-100') return count >= 11 && count <= 100
  if (range === '100+') return count >= 101
  return true
}

const filteredSubscriptions = computed(() => {
  const keyword = normalizeString(filters.keyword).toLowerCase()

  return subscriptions.value.filter((row) => {
    if (!matchesAccessRange(row.access_count, filters.accessRange)) {
      return false
    }

    if (filters.activity === 'never' && row.access_count > 0) {
      return false
    }

    if (filters.activity === 'recent' && !isWithinDays(row.last_access_at, 7)) {
      return false
    }

    if (
      filters.activity === 'stale' &&
      (row.access_count === 0 || !row.last_access_at || isWithinDays(row.last_access_at, 30))
    ) {
      return false
    }

    if (!keyword) {
      return true
    }

    const searchableText = [
      row.id,
      row.user_id,
      row.username_display,
      row.short_code,
      row.last_ip,
      row.token,
      row.created_at_display,
      row.last_access_display,
      row.activity_label
    ].join(' ').toLowerCase()

    return searchableText.includes(keyword)
  })
})

const sortedSubscriptions = computed(() => {
  const rows = [...filteredSubscriptions.value]

  if (sortKey.value === 'access_desc') {
    return rows.sort((a, b) => b.access_count - a.access_count || b.user_id - a.user_id)
  }

  if (sortKey.value === 'created_desc') {
    return rows.sort((a, b) => toTimestamp(b.created_at) - toTimestamp(a.created_at) || b.user_id - a.user_id)
  }

  if (sortKey.value === 'user_desc') {
    return rows.sort((a, b) => Number(b.user_id || 0) - Number(a.user_id || 0))
  }

  return rows.sort((a, b) => {
    const recentDiff = toTimestamp(b.last_access_at) - toTimestamp(a.last_access_at)
    if (recentDiff !== 0) return recentDiff

    const accessDiff = b.access_count - a.access_count
    if (accessDiff !== 0) return accessDiff

    return toTimestamp(b.created_at) - toTimestamp(a.created_at)
  })
})

const paginatedSubscriptions = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return sortedSubscriptions.value.slice(start, start + pageSize.value)
})

const visitedCount = computed(() => filteredSubscriptions.value.filter((item) => item.access_count > 0).length)
const neverVisitedCount = computed(() => filteredSubscriptions.value.filter((item) => item.access_count === 0).length)
const recentActiveCount = computed(() => filteredSubscriptions.value.filter((item) => isWithinDays(item.last_access_at, 7)).length)
const hasActiveFilters = computed(() => Boolean(
  normalizeString(filters.keyword) || filters.accessRange || filters.activity || sortKey.value !== 'recent_access'
))

const resetFilters = () => {
  filters.keyword = ''
  filters.accessRange = ''
  filters.activity = ''
  sortKey.value = 'recent_access'
}

const handleSizeChange = (value) => {
  pageSize.value = value
  currentPage.value = 1
}

const handleCurrentChange = (value) => {
  currentPage.value = value
}

const copyValue = async (value, label) => {
  const text = normalizeString(value)

  if (!text) {
    ElMessage.warning(`${label}为空`)
    return
  }

  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`${label}已复制到剪贴板`)
  } catch (error) {
    const textarea = document.createElement('textarea')
    textarea.value = text
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    ElMessage.success(`${label}已复制到剪贴板`)
  }
}

const copyToken = async (token) => {
  await copyValue(token, '令牌')
}

const copyShortCode = async (shortCode) => {
  await copyValue(shortCode, '短码')
}

const fetchSubscriptions = async () => {
  loading.value = true

  try {
    const pageLimit = 100
    const allRows = []
    let expectedTotal = 0
    let page = 1
    let requestCount = 0

    while (requestCount < 50) {
      const response = await subscriptionApi.admin.list({
        page,
        page_size: pageLimit
      })
      const payload = unwrapPayload(response)
      const list = Array.isArray(payload) ? payload : (payload?.subscriptions || [])

      if (!Array.isArray(list) || list.length === 0) {
        if (!Array.isArray(payload)) {
          expectedTotal = Number(payload?.total || allRows.length)
        }
        break
      }

      allRows.push(...list)
      requestCount += 1

      if (Array.isArray(payload)) {
        if (list.length < pageLimit) {
          break
        }
      } else {
        expectedTotal = Number(payload?.total || allRows.length)
        if (allRows.length >= expectedTotal || list.length < pageLimit) {
          break
        }
      }

      page += 1
    }

    const dedupedRows = Array.from(
      new Map(allRows.map((item) => [String(item.id), normalizeSubscription(item)])).values()
    )

    subscriptions.value = dedupedRows
    total.value = expectedTotal || dedupedRows.length
  } catch (error) {
    console.error('获取订阅列表失败:', error)
    ElMessage.error('获取订阅列表失败')
    subscriptions.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const handleResetStats = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要重置用户 "${row.username_display}" 的订阅访问统计吗？`,
      '确认重置',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    await subscriptionApi.admin.resetStats(row.user_id)
    ElMessage.success('访问统计已重置')
    await fetchSubscriptions()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      console.error('重置统计失败:', error)
      ElMessage.error('重置统计失败')
    }
  }
}

const handleRevoke = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要撤销用户 "${row.username_display}" 的订阅吗？撤销后用户需要重新获取订阅链接。`,
      '确认撤销',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    await subscriptionApi.admin.revoke(row.user_id)
    ElMessage.success('订阅已撤销')
    await fetchSubscriptions()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      console.error('撤销订阅失败:', error)
      ElMessage.error('撤销订阅失败')
    }
  }
}

const maskToken = (token) => {
  const normalized = normalizeString(token)
  if (!normalized) return '-'
  if (normalized.length <= 8) return normalized
  return `${normalized.substring(0, 6)}...${normalized.substring(normalized.length - 6)}`
}

watch(() => route.query.user_id, (value) => {
  filters.keyword = normalizeString(value)
}, { immediate: true })

watch(() => [filters.keyword, filters.accessRange, filters.activity, sortKey.value], () => {
  currentPage.value = 1
})

watch(filteredSubscriptions, (rows) => {
  const maxPage = Math.max(1, Math.ceil(rows.length / pageSize.value))
  if (currentPage.value > maxPage) {
    currentPage.value = maxPage
  }
})

onMounted(() => {
  fetchSubscriptions()
})
</script>

<style scoped>
.admin-subscriptions-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 18px 20px;
  background: linear-gradient(135deg, #ffffff 0%, #f7faff 100%);
  border-radius: 16px;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
  border: 1px solid rgba(148, 163, 184, 0.16);
}

.page-heading {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.title {
  font-size: 22px;
  font-weight: 700;
  color: var(--el-text-color-primary, #1f2937);
  letter-spacing: 0.02em;
}

.page-subtitle {
  font-size: 13px;
  color: var(--el-text-color-secondary, #6b7280);
}

.refresh-btn {
  font-size: 13px;
  padding: 10px 18px;
}

.overview-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.overview-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px 18px;
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.96) 0%, rgba(248, 250, 252, 0.96) 100%);
  border: 1px solid rgba(148, 163, 184, 0.14);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.04);
}

.overview-label {
  font-size: 12px;
  color: var(--el-text-color-secondary, #6b7280);
}

.overview-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--el-text-color-primary, #111827);
}

.overview-value.is-success {
  color: #15803d;
}

.overview-value.is-muted {
  color: #64748b;
}

.overview-value.is-primary {
  color: #1d4ed8;
}

.toolbar-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  padding: 16px 18px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid rgba(148, 163, 184, 0.14);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.04);
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar-search {
  width: 320px;
}

.toolbar-filters .el-select {
  width: 150px;
}

.toolbar-summary {
  font-size: 12px;
  color: var(--el-text-color-secondary, #6b7280);
}

:deep(.subscriptions-table.el-table) {
  width: 100%;
  table-layout: auto;
  background-color: var(--el-bg-color, #fff);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.05);
  font-size: 13px;
}

:deep(.subscriptions-table.el-table--border) {
  border: 1px solid rgba(148, 163, 184, 0.16);
}

:deep(.subscriptions-table .el-table__header th) {
  background: #f8fafc;
  font-weight: 600;
  color: #475569;
  font-size: 12px;
  letter-spacing: 0.02em;
}

:deep(.subscriptions-table .el-table__cell) {
  vertical-align: top;
}

:deep(.subscriptions-table .cell) {
  padding: 12px 12px;
  line-height: 1.4;
  white-space: normal;
  overflow: hidden;
  text-overflow: ellipsis;
}

:deep(.subscriptions-table .el-table__body tr:hover > td) {
  background-color: #f8fbff;
}

:deep(.subscriptions-table.el-table--striped .el-table__body tr.el-table__row--striped > td) {
  background-color: #fcfdff;
}

.user-cell {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.user-cell__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.user-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary, #111827);
}

.user-cell__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  font-size: 12px;
  color: var(--el-text-color-secondary, #6b7280);
}

.user-cell__hint {
  font-size: 12px;
  color: #475569;
  background: #f8fafc;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 10px;
  padding: 6px 10px;
}

.activity-pill,
.access-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 24px;
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.activity-pill.dormant,
.access-badge.dormant {
  color: #475569;
  background: #f1f5f9;
}

.activity-pill.steady,
.access-badge.steady {
  color: #0369a1;
  background: #e0f2fe;
}

.activity-pill.active,
.access-badge.active {
  color: #166534;
  background: #ecfdf5;
}

.activity-pill.intense,
.access-badge.intense {
  color: #b45309;
  background: #fffbeb;
}

.credential-cell {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.credential-item {
  display: flex;
  align-items: center;
  gap: 10px;
}

.credential-label {
  flex-shrink: 0;
  width: 28px;
  font-size: 12px;
  color: var(--el-text-color-secondary, #6b7280);
}

.credential-main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 1;
  padding: 5px 8px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid rgba(148, 163, 184, 0.14);
}

.credential-value {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 12px;
  color: #334155;
}

:deep(.copy-token-btn.el-button) {
  margin: 0;
  padding: 0;
  min-width: 24px;
  height: 24px;
  color: #2563eb;
}

.detail-cell {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.detail-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.detail-label {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary, #6b7280);
}

.detail-value {
  text-align: right;
  font-size: 13px;
  color: var(--el-text-color-primary, #334155);
  word-break: break-word;
}

.operation-btns {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  flex-wrap: nowrap;
  gap: 4px;
  width: 100%;
}

.operation-btns .el-button {
  margin: 0 !important;
}

:deep(.operation-btns .el-button) {
  min-width: 0;
  height: 30px;
  padding: 0 9px !important;
  border-radius: 10px;
  box-shadow: none;
}

:deep(.operation-btns .row-action.el-button) {
  border: 1px solid transparent;
  font-size: 12px;
  font-weight: 600;
}

:deep(.operation-btns .row-action--warning.el-button) {
  color: #b45309;
  background: #fff7ed;
  border-color: #fed7aa;
}

:deep(.operation-btns .row-action--danger.el-button) {
  color: #b91c1c;
  background: #fef2f2;
  border-color: #fecaca;
}

.pagination-container {
  display: flex;
  justify-content: flex-end;
  margin-top: 4px;
}

:deep(.el-pagination) {
  padding: 10px 0;
  font-weight: normal;
}

:deep(.el-pagination button) {
  min-width: 30px;
  height: 30px;
}

:deep(.el-pagination .el-select .el-input) {
  width: 104px;
}

@media (max-width: 1280px) {
  .overview-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .toolbar-card {
    flex-direction: column;
    align-items: stretch;
  }

  .user-cell__header {
    flex-direction: column;
    align-items: flex-start;
  }
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .overview-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .toolbar-search,
  .toolbar-filters,
  .toolbar-filters .el-select {
    width: 100%;
  }

  .toolbar-filters {
    flex-direction: column;
    align-items: stretch;
  }

  :deep(.subscriptions-table .cell) {
    padding: 12px 10px;
  }

  .credential-item,
  .detail-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .credential-label {
    width: auto;
  }

  .detail-value {
    text-align: left;
  }

  .operation-btns {
    justify-content: flex-start;
  }

  .pagination-container {
    justify-content: center;
    overflow-x: auto;
  }
}
</style>
