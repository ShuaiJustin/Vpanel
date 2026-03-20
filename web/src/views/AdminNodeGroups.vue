<template>
  <div class="admin-node-groups-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">节点分组管理</h1>
        <p class="page-subtitle">统一查看地区分组、调度策略、节点健康度和用户规模</p>
      </div>
      <div class="page-actions">
        <el-button @click="fetchGroups">
          <el-icon class="el-icon--left"><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="primary" @click="showCreateDialog">
          <el-icon class="el-icon--left"><Plus /></el-icon>
          创建分组
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">当前匹配</span>
        <strong class="overview-value">{{ displayGroupTotal }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">覆盖节点</span>
        <strong class="overview-value is-primary">{{ coveredNodeTotal }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">健康节点</span>
        <strong class="overview-value is-success">{{ healthyNodeTotal }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">覆盖用户</span>
        <strong class="overview-value">{{ coveredUserTotal }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">空分组</span>
        <strong class="overview-value is-warning">{{ emptyGroupCount }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-filters">
        <el-input
          v-model="groupStore.filters.search"
          class="toolbar-search"
          placeholder="搜索分组名称、描述或地区"
          clearable
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select v-model="groupStore.filters.region" placeholder="地区" clearable>
          <el-option
            v-for="region in groupStore.regions"
            :key="region"
            :label="region"
            :value="region"
          />
        </el-select>
        <el-select v-model="strategyFilter" placeholder="调度策略" clearable>
          <el-option
            v-for="strategy in strategyOptions"
            :key="strategy.value"
            :label="strategy.label"
            :value="strategy.value"
          />
        </el-select>
        <el-button @click="resetFilters">重置</el-button>
      </div>
      <div class="toolbar-actions">
        <span class="toolbar-summary">
          当前页 {{ paginatedGroups.length }} 个分组，筛选后 {{ displayGroupTotal }} 个，总计 {{ groupStore.total }} 个
        </span>
      </div>
    </div>

    <div class="groups-shell" v-loading="groupStore.loading">
      <div v-if="paginatedGroups.length" class="groups-grid">
        <article
          v-for="group in paginatedGroups"
          :key="group.id"
          class="group-panel"
          @click="viewGroupDetail(group)"
        >
          <div class="group-panel__header">
            <div class="group-panel__heading">
              <div class="group-panel__title-row">
                <h3 class="group-panel__title">{{ group.name }}</h3>
                <span :class="['metric-pill', getStrategyClass(group.strategy)]">
                  {{ getStrategyText(group.strategy) }}
                </span>
              </div>
              <div class="group-panel__meta">
                <span>ID：{{ group.id }}</span>
                <span>地区：{{ group.region || '未设置地区' }}</span>
              </div>
            </div>

            <div class="group-panel__menu" @click.stop>
              <el-dropdown trigger="click" @command="(command) => handleGroupCommand(command, group)">
                <el-button size="small" class="row-action row-action--more" circle title="更多操作">
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="edit">编辑分组</el-dropdown-item>
                    <el-dropdown-item command="nodes">管理节点</el-dropdown-item>
                    <el-dropdown-item command="delete" divided>删除分组</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>

          <div class="entity-cell__hint">
            {{ group.description || getGroupHint(group) }}
          </div>

          <div class="group-panel__stats">
            <div class="group-panel__stat">
              <span class="group-panel__stat-value">{{ getNodeCount(group) }}</span>
              <span class="group-panel__stat-label">节点</span>
            </div>
            <div class="group-panel__stat">
              <span class="group-panel__stat-value is-success">{{ getHealthyCount(group) }}</span>
              <span class="group-panel__stat-label">健康</span>
            </div>
            <div class="group-panel__stat">
              <span class="group-panel__stat-value">{{ getUserCount(group) }}</span>
              <span class="group-panel__stat-label">用户</span>
            </div>
          </div>

          <div class="group-panel__health">
            <div class="stack-item">
              <span class="stack-label">健康占比</span>
              <span class="stack-value">{{ getHealthRate(group) }}%</span>
            </div>
            <el-progress
              :percentage="getHealthRate(group)"
              :stroke-width="6"
              :show-text="false"
              :status="getHealthProgressStatus(group)"
            />
          </div>

          <div class="group-panel__actions" @click.stop>
            <el-button
              size="small"
              class="row-action row-action--primary"
              @click="viewGroupDetail(group)"
            >
              详情
            </el-button>
            <el-button
              size="small"
              class="row-action row-action--success"
              @click="manageNodes(group)"
            >
              管理节点
            </el-button>
          </div>
        </article>
      </div>

      <el-empty
        v-else
        :description="hasActiveFilters ? '暂无匹配分组' : '暂无分组'"
      />
    </div>

    <div v-if="displayGroupTotal > 0" class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :total="displayGroupTotal"
        :page-sizes="[6, 9, 12, 24]"
        :layout="isMobile ? 'total, prev, next' : 'total, sizes, prev, pager, next'"
        @size-change="handleSizeChange"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑分组' : '创建分组'"
      :width="isMobile ? 'calc(100vw - 24px)' : '520px'"
    >
      <el-form :model="form" :rules="rules" ref="formRef" :label-width="isMobile ? '84px' : '100px'">
        <el-form-item label="分组名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入分组名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="请输入分组描述" />
        </el-form-item>
        <el-form-item label="地区" prop="region">
          <el-select v-model="form.region" filterable allow-create placeholder="选择或输入地区" style="width: 100%">
            <el-option label="香港" value="香港" />
            <el-option label="日本" value="日本" />
            <el-option label="新加坡" value="新加坡" />
            <el-option label="美国" value="美国" />
            <el-option label="韩国" value="韩国" />
            <el-option label="台湾" value="台湾" />
            <el-option label="德国" value="德国" />
            <el-option label="英国" value="英国" />
          </el-select>
        </el-form-item>
        <el-form-item label="调度策略" prop="strategy">
          <el-select v-model="form.strategy" placeholder="选择策略" style="width: 100%">
            <el-option label="轮询" value="round-robin" />
            <el-option label="最少连接" value="least-connections" />
            <el-option label="加权" value="weighted" />
            <el-option label="地理位置" value="geographic" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitForm">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="nodesDialogVisible"
      title="管理分组节点"
      :width="isMobile ? 'calc(100vw - 24px)' : '760px'"
    >
      <div v-if="currentGroup" class="nodes-dialog-content">
        <div class="surface-inline nodes-dialog-summary">
          <div class="stack-item">
            <span class="stack-label">当前分组</span>
            <span class="stack-value is-strong">{{ currentGroup.name }}</span>
          </div>
          <div class="stack-item">
            <span class="stack-label">策略 / 地区</span>
            <span class="stack-value">{{ getStrategyText(currentGroup.strategy) }} / {{ currentGroup.region || '未设置地区' }}</span>
          </div>
        </div>

        <el-transfer
          v-model="selectedNodeIds"
          :data="transferData"
          :titles="['可用节点', '已选节点']"
          :filter-method="filterNodes"
          filterable
          filter-placeholder="搜索节点"
        />
      </div>
      <template #footer>
        <el-button @click="nodesDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingNodes" @click="saveGroupNodes">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="detailDialogVisible"
      title="分组详情"
      :width="isMobile ? 'calc(100vw - 24px)' : '680px'"
    >
      <div v-if="currentGroup" class="group-detail">
        <el-descriptions :column="isMobile ? 1 : 2" border>
          <el-descriptions-item label="ID">{{ currentGroup.id }}</el-descriptions-item>
          <el-descriptions-item label="名称">{{ currentGroup.name }}</el-descriptions-item>
          <el-descriptions-item label="地区">{{ currentGroup.region || '-' }}</el-descriptions-item>
          <el-descriptions-item label="策略">{{ getStrategyText(currentGroup.strategy) }}</el-descriptions-item>
          <el-descriptions-item label="节点数">{{ getNodeCount(currentGroup) }}</el-descriptions-item>
          <el-descriptions-item label="健康节点">{{ getHealthyCount(currentGroup) }}</el-descriptions-item>
          <el-descriptions-item label="用户数">{{ getUserCount(currentGroup) }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatTime(currentGroup.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="描述" :span="isMobile ? 1 : 2">
            {{ currentGroup.description || '-' }}
          </el-descriptions-item>
        </el-descriptions>

        <div class="group-nodes-section">
          <div class="card-header">
            <span>分组节点</span>
            <span class="toolbar-summary">当前共 {{ groupNodes.length }} 个节点</span>
          </div>

          <div v-if="groupNodes.length" class="table-shell">
            <el-table :data="groupNodes" size="small" border stripe class="group-nodes-table">
              <el-table-column prop="name" label="名称" min-width="140" />
              <el-table-column prop="address" label="地址" min-width="160" />
              <el-table-column label="状态" width="90">
                <template #default="{ row }">
                  <span :class="['metric-pill', getNodeStatusClass(row.status)]">
                    {{ getStatusText(row.status) }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column prop="current_users" label="用户数" width="90" />
            </el-table>
          </div>
          <el-empty v-else description="该分组暂未分配节点" />
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { MoreFilled, Plus, Refresh, Search } from '@element-plus/icons-vue'
import { useNodeGroupStore } from '@/stores/nodeGroup'
import { useNodeStore } from '@/stores/node'
import { useViewport } from '@/composables/useViewport'

const groupStore = useNodeGroupStore()
const nodeStore = useNodeStore()
const { isMobile } = useViewport()

const dialogVisible = ref(false)
const nodesDialogVisible = ref(false)
const detailDialogVisible = ref(false)
const isEdit = ref(false)
const submitting = ref(false)
const savingNodes = ref(false)
const formRef = ref(null)
const currentGroup = ref(null)
const groupNodes = ref([])
const selectedNodeIds = ref([])
const strategyFilter = ref('')
const currentPage = ref(1)
const pageSize = ref(9)

const form = reactive({
  id: null,
  name: '',
  description: '',
  region: '',
  strategy: 'round-robin'
})

const rules = {
  name: [{ required: true, message: '请输入分组名称', trigger: 'blur' }]
}

const strategyOptions = [
  { label: '轮询', value: 'round-robin' },
  { label: '最少连接', value: 'least-connections' },
  { label: '加权', value: 'weighted' },
  { label: '地理位置', value: 'geographic' }
]

const transferData = computed(() =>
  nodeStore.nodes.map((node) => ({
    key: node.id,
    label: `${node.name} (${node.address})`,
    disabled: false
  }))
)

const displayGroups = computed(() => {
  if (!strategyFilter.value) {
    return groupStore.filteredGroups
  }

  return groupStore.filteredGroups.filter((group) => group.strategy === strategyFilter.value)
})

const displayGroupTotal = computed(() => displayGroups.value.length)
const coveredNodeTotal = computed(() => displayGroups.value.reduce((sum, group) => sum + getNodeCount(group), 0))
const healthyNodeTotal = computed(() => displayGroups.value.reduce((sum, group) => sum + getHealthyCount(group), 0))
const coveredUserTotal = computed(() => displayGroups.value.reduce((sum, group) => sum + getUserCount(group), 0))
const emptyGroupCount = computed(() => displayGroups.value.filter((group) => getNodeCount(group) === 0).length)
const hasActiveFilters = computed(() => Boolean(groupStore.filters.search || groupStore.filters.region || strategyFilter.value))
const paginatedGroups = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return displayGroups.value.slice(start, end)
})

const syncCurrentPage = () => {
  const maxPage = Math.max(1, Math.ceil(displayGroupTotal.value / pageSize.value))
  if (currentPage.value > maxPage) {
    currentPage.value = maxPage
  }
}

watch(
  [() => groupStore.filters.search, () => groupStore.filters.region, strategyFilter],
  () => {
    currentPage.value = 1
  }
)

watch(displayGroupTotal, () => {
  syncCurrentPage()
})

const getNodeCount = (group) => Number(group?.total_nodes || group?.node_count || 0)
const getHealthyCount = (group) => Number(group?.healthy_nodes || 0)
const getUserCount = (group) => Number(group?.total_users || group?.user_count || 0)

const getStrategyText = (strategy) => {
  const texts = {
    'round-robin': '轮询',
    'least-connections': '最少连接',
    weighted: '加权',
    geographic: '地理位置'
  }
  return texts[strategy] || strategy || '未设置'
}

const getStrategyClass = (strategy) => {
  const classes = {
    'round-robin': 'is-primary',
    'least-connections': 'is-success',
    weighted: 'is-warning',
    geographic: 'is-muted'
  }
  return classes[strategy] || 'is-muted'
}

const getNodeStatusClass = (status) => {
  const classes = {
    online: 'is-success',
    offline: 'is-muted',
    unhealthy: 'is-danger'
  }
  return classes[status] || 'is-muted'
}

const getStatusText = (status) => {
  const texts = { online: '在线', offline: '离线', unhealthy: '不健康' }
  return texts[status] || status
}

const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

const getHealthRate = (group) => {
  const totalNodes = getNodeCount(group)
  if (!totalNodes) return 0
  return Math.round((getHealthyCount(group) / totalNodes) * 100)
}

const getHealthProgressStatus = (group) => {
  const rate = getHealthRate(group)
  if (rate >= 80) return 'success'
  if (rate >= 50) return 'warning'
  return 'exception'
}

const getGroupHint = (group) => {
  const totalNodes = getNodeCount(group)
  const totalUsers = getUserCount(group)

  if (!totalNodes) {
    return '当前分组还没有分配节点，适合先配置地区和策略后再接入节点。'
  }

  if (!totalUsers) {
    return `当前已挂 ${totalNodes} 个节点，但暂时没有用户流量进入该分组。`
  }

  return `当前策略为 ${getStrategyText(group.strategy)}，已承载 ${totalUsers} 个用户连接。`
}

const filterNodes = (query, item) => item.label.toLowerCase().includes(query.toLowerCase())

const fetchGroups = async () => {
  try {
    await groupStore.fetchGroupsWithStats()
  } catch (e) {
    ElMessage.error(e.message || '获取分组列表失败')
  }
}

const fetchAllNodes = async () => {
  try {
    await nodeStore.fetchNodes({ limit: 1000 })
  } catch (e) {
    console.error('获取节点列表失败:', e)
  }
}

const resetFilters = () => {
  groupStore.clearFilters()
  strategyFilter.value = ''
  currentPage.value = 1
}

const handleSizeChange = (value) => {
  pageSize.value = value
  syncCurrentPage()
}

const showCreateDialog = () => {
  isEdit.value = false
  Object.assign(form, { id: null, name: '', description: '', region: '', strategy: 'round-robin' })
  dialogVisible.value = true
}

const editGroup = (group) => {
  isEdit.value = true
  Object.assign(form, {
    id: group.id,
    name: group.name,
    description: group.description || '',
    region: group.region || '',
    strategy: group.strategy || 'round-robin'
  })
  dialogVisible.value = true
}

const submitForm = async () => {
  await formRef.value.validate()
  submitting.value = true
  try {
    const data = {
      name: form.name,
      description: form.description,
      region: form.region,
      strategy: form.strategy
    }
    if (isEdit.value) {
      await groupStore.updateGroup(form.id, data)
      ElMessage.success('更新成功')
    } else {
      await groupStore.createGroup(data)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    fetchGroups()
  } catch (e) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const deleteGroup = async (group) => {
  await ElMessageBox.confirm(
    `确定要删除分组 "${group.name}" 吗？分组内的节点不会被删除。`,
    '删除确认',
    { type: 'warning' }
  )
  try {
    await groupStore.deleteGroup(group.id)
    ElMessage.success('删除成功')
    fetchGroups()
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

const handleGroupCommand = (command, group) => {
  if (command === 'edit') {
    editGroup(group)
    return
  }

  if (command === 'nodes') {
    manageNodes(group)
    return
  }

  if (command === 'delete') {
    deleteGroup(group)
  }
}

const manageNodes = async (group) => {
  currentGroup.value = group
  await fetchAllNodes()

  try {
    const nodes = await groupStore.fetchGroupNodes(group.id)
    selectedNodeIds.value = nodes.map((node) => node.id)
  } catch (e) {
    selectedNodeIds.value = []
  }

  nodesDialogVisible.value = true
}

const saveGroupNodes = async () => {
  savingNodes.value = true
  try {
    await groupStore.setGroupNodes(currentGroup.value.id, selectedNodeIds.value)
    ElMessage.success('保存成功')
    nodesDialogVisible.value = false
    fetchGroups()
  } catch (e) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    savingNodes.value = false
  }
}

const viewGroupDetail = async (group) => {
  currentGroup.value = group
  try {
    const nodes = await groupStore.fetchGroupNodes(group.id)
    groupNodes.value = nodes
  } catch (e) {
    groupNodes.value = []
  }
  detailDialogVisible.value = true
}

onMounted(fetchGroups)
</script>

<style scoped>
.admin-node-groups-page {
  padding: 20px;
}

.groups-shell {
  min-height: 220px;
}

.groups-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 18px;
}

.group-panel {
  display: grid;
  gap: 16px;
  padding: 20px;
  border-radius: 20px;
  border: 1px solid var(--admin-border);
  box-shadow: var(--admin-shadow-soft);
  background: linear-gradient(180deg, var(--admin-surface-strong) 0%, var(--admin-surface) 100%);
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.group-panel:hover {
  transform: translateY(-3px);
  box-shadow: var(--admin-shadow);
  border-color: var(--admin-border-strong);
}

.group-panel__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 14px;
}

.group-panel__heading {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.group-panel__title-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  flex-wrap: wrap;
}

.group-panel__title {
  margin: 0;
  font-size: 18px;
  line-height: 1.2;
  font-weight: 700;
  color: var(--admin-title);
}

.group-panel__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  font-size: 12px;
  color: var(--admin-text-muted);
}

.group-panel__stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.group-panel__stat {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
  border-radius: 14px;
  border: 1px solid var(--admin-border);
  background: var(--admin-surface-soft);
  text-align: center;
}

.group-panel__stat-value {
  font-size: 22px;
  line-height: 1;
  font-weight: 700;
  color: var(--admin-title);
}

.group-panel__stat-value.is-success {
  color: #15803d;
}

.group-panel__stat-label {
  font-size: 12px;
  color: var(--admin-text-muted);
}

.group-panel__health {
  display: grid;
  gap: 8px;
}

.group-panel__actions {
  display: flex;
  gap: 10px;
}

.nodes-dialog-content {
  display: grid;
  gap: 16px;
}

.nodes-dialog-summary {
  display: grid;
  gap: 8px;
}

.group-detail {
  display: grid;
  gap: 20px;
}

.group-nodes-section {
  display: grid;
  gap: 14px;
}

.group-nodes-table {
  min-width: 480px;
}

@media (max-width: 768px) {
  .admin-node-groups-page {
    padding: 12px;
  }

  .groups-grid {
    grid-template-columns: 1fr;
  }

  .group-panel {
    padding: 16px;
  }

  .group-panel__stats {
    grid-template-columns: 1fr;
  }

  .group-panel__actions {
    flex-direction: column;
  }

  .group-panel__actions .el-button {
    width: 100%;
  }

  .group-nodes-table {
    min-width: 560px;
  }
}
</style>
