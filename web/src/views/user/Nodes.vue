<template>
  <div class="nodes-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">
        节点列表
      </h1>
      <p class="page-subtitle">
        选择合适的节点获取最佳连接体验
      </p>
    </div>

    <!-- 筛选和操作栏 -->
    <div class="filter-bar">
      <div class="filter-left">
        <el-select 
          v-model="filters.region" 
          placeholder="全部地区" 
          clearable
          style="width: 140px"
        >
          <el-option 
            v-for="region in regions" 
            :key="region.value" 
            :label="region.label" 
            :value="region.value" 
          />
        </el-select>

        <el-select 
          v-model="filters.protocol" 
          placeholder="全部协议" 
          clearable
          style="width: 140px"
        >
          <el-option 
            v-for="protocol in protocols" 
            :key="protocol.value" 
            :label="protocol.label" 
            :value="protocol.value" 
          />
        </el-select>

        <el-select 
          v-model="filters.status" 
          placeholder="全部状态" 
          clearable
          style="width: 120px"
        >
          <el-option
            label="在线"
            value="online"
          />
          <el-option
            label="离线"
            value="offline"
          />
          <el-option
            label="维护中"
            value="maintenance"
          />
        </el-select>
      </div>

      <div class="filter-right">
        <el-select
          v-model="sortBy"
          style="width: 140px"
        >
          <el-option
            label="默认排序"
            value="default"
          />
          <el-option
            label="按名称"
            value="name"
          />
          <el-option
            label="按地区"
            value="region"
          />
          <el-option
            label="按延迟"
            value="latency"
          />
          <el-option
            label="按负载"
            value="load"
          />
        </el-select>

        <el-button-group>
          <el-button 
            :type="viewMode === 'card' ? 'primary' : 'default'"
            @click="viewMode = 'card'"
          >
            <el-icon><Grid /></el-icon>
          </el-button>
          <el-button 
            :type="viewMode === 'list' ? 'primary' : 'default'"
            @click="viewMode = 'list'"
          >
            <el-icon><List /></el-icon>
          </el-button>
        </el-button-group>

        <el-button
          type="primary"
          :loading="testingAll"
          @click="testAllLatency"
        >
          <el-icon><Timer /></el-icon>
          测速全部
        </el-button>
      </div>
    </div>

    <!-- 加载状态 -->
    <div
      v-if="loading"
      class="loading-state"
    >
      <el-icon class="loading-icon">
        <Loading />
      </el-icon>
      <p>加载节点列表...</p>
    </div>

    <!-- 空状态 -->
    <el-empty
      v-else-if="filteredNodes.length === 0"
      description="暂无可用节点"
    />

    <!-- 卡片视图 -->
    <div
      v-else-if="viewMode === 'card'"
      class="nodes-grid"
    >
      <NodeCard
        v-for="node in filteredNodes"
        :key="node.id"
        :node="node"
        :latency="latencyResults[node.id]"
        :testing="testingNodes[node.id]"
        @test="testLatency(node)"
        @copy="copyNodeConfig(node)"
      />
    </div>

    <!-- 列表视图 -->
    <el-table
      v-else
      :data="filteredNodes"
      class="nodes-table"
    >
      <el-table-column
        label="节点名称"
        min-width="200"
      >
        <template #default="{ row }">
          <div class="node-name-cell">
            <span class="node-flag">{{ getRegionFlag(row.region) }}</span>
            <span class="node-name">{{ row.name }}</span>
          </div>
        </template>
      </el-table-column>

      <el-table-column
        label="地区"
        prop="region"
        width="100"
      >
        <template #default="{ row }">
          {{ getRegionLabel(row.region) }}
        </template>
      </el-table-column>

      <el-table-column
        label="协议"
        prop="protocol"
        width="100"
      >
        <template #default="{ row }">
          <el-tag size="small">
            {{ row.protocol }}
          </el-tag>
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
        label="负载"
        width="120"
      >
        <template #default="{ row }">
          <el-progress 
            :percentage="row.load" 
            :color="getLoadColor(row.load)"
            :stroke-width="6"
            :show-text="false"
          />
          <span class="load-text">{{ row.load }}%</span>
        </template>
      </el-table-column>

      <el-table-column
        label="延迟"
        width="100"
      >
        <template #default="{ row }">
          <span
            v-if="testingNodes[row.id]"
            class="latency-testing"
          >
            <el-icon class="is-loading"><Loading /></el-icon>
          </span>
          <span
            v-else-if="latencyResults[row.id]"
            :class="getLatencyClass(latencyResults[row.id])"
          >
            {{ latencyResults[row.id] }}ms
          </span>
          <span
            v-else
            class="latency-unknown"
          >-</span>
        </template>
      </el-table-column>

      <el-table-column
        label="操作"
        width="150"
        fixed="right"
      >
        <template #default="{ row }">
          <el-button
            link
            type="primary"
            :loading="testingNodes[row.id]"
            @click="testLatency(row)"
          >
            测速
          </el-button>
          <el-button
            link
            type="primary"
            @click="copyNodeConfig(row)"
          >
            复制
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Grid, List, Timer, Loading } from '@element-plus/icons-vue'
import { usePortalNodesStore } from '@/stores/portalNodes'
import { proxiesApi } from '@/api/modules/proxies'
import { copyText } from '@/utils/clipboard'
import NodeCard from '@/components/user/NodeCard.vue'

const nodesStore = usePortalNodesStore()

// 状态
const loading = ref(false)
const viewMode = ref('card')
const sortBy = ref('default')
const testingAll = ref(false)
const testingNodes = reactive({})
const latencyResults = reactive({})

// 筛选条件
const filters = reactive({
  region: '',
  protocol: '',
  status: ''
})

// 地区选项
const regions = [
  { value: 'hk', label: '香港' },
  { value: 'tw', label: '台湾' },
  { value: 'jp', label: '日本' },
  { value: 'sg', label: '新加坡' },
  { value: 'us', label: '美国' },
  { value: 'kr', label: '韩国' },
  { value: 'de', label: '德国' },
  { value: 'uk', label: '英国' }
]

// 协议选项
const protocols = [
  { value: 'vmess', label: 'VMess' },
  { value: 'vless', label: 'VLESS' },
  { value: 'trojan', label: 'Trojan' },
  { value: 'shadowsocks', label: 'Shadowsocks' }
]

// 计算属性
const filteredNodes = computed(() => {
  let nodes = [...nodesStore.nodes]

  // 筛选
  if (filters.region) {
    nodes = nodes.filter(n => n.region === filters.region)
  }
  if (filters.protocol) {
    nodes = nodes.filter(n => n.protocol === filters.protocol)
  }
  if (filters.status) {
    nodes = nodes.filter(n => n.status === filters.status)
  }

  // 排序
  if (sortBy.value === 'name') {
    nodes.sort((a, b) => a.name.localeCompare(b.name))
  } else if (sortBy.value === 'region') {
    nodes.sort((a, b) => a.region.localeCompare(b.region))
  } else if (sortBy.value === 'latency') {
    nodes.sort((a, b) => {
      const la = latencyResults[a.id] || 9999
      const lb = latencyResults[b.id] || 9999
      return la - lb
    })
  } else if (sortBy.value === 'load') {
    nodes.sort((a, b) => a.load - b.load)
  }

  return nodes
})

// 方法
function getRegionFlag(region) {
  const flags = {
    hk: '🇭🇰',
    tw: '🇹🇼',
    jp: '🇯🇵',
    sg: '🇸🇬',
    us: '🇺🇸',
    kr: '🇰🇷',
    de: '🇩🇪',
    uk: '🇬🇧'
  }
  return flags[region] || '🌐'
}

function getRegionLabel(region) {
  const labels = {
    hk: '香港',
    tw: '台湾',
    jp: '日本',
    sg: '新加坡',
    us: '美国',
    kr: '韩国',
    de: '德国',
    uk: '英国'
  }
  return labels[region] || region
}

function getStatusType(status) {
  const types = {
    online: 'success',
    offline: 'danger',
    maintenance: 'warning'
  }
  return types[status] || 'info'
}

function getStatusLabel(status) {
  const labels = {
    online: '在线',
    offline: '离线',
    maintenance: '维护中'
  }
  return labels[status] || status
}

function getLoadColor(load) {
  if (load >= 80) return '#f56c6c'
  if (load >= 60) return '#e6a23c'
  return '#67c23a'
}

function getLatencyClass(latency) {
  if (latency < 100) return 'latency-good'
  if (latency < 200) return 'latency-fair'
  return 'latency-poor'
}

async function testLatency(node) {
  testingNodes[node.id] = true
  try {
    const latency = await nodesStore.testNodeLatency(node.id)
    latencyResults[node.id] = latency
  } catch (error) {
    latencyResults[node.id] = null
    ElMessage.error(`测速失败: ${node.name}`)
  } finally {
    testingNodes[node.id] = false
  }
}

async function testAllLatency() {
  testingAll.value = true
  const onlineNodes = filteredNodes.value.filter(n => n.status === 'online')
  
  for (const node of onlineNodes) {
    await testLatency(node)
  }
  
  testingAll.value = false
  ElMessage.success('测速完成')
}

async function copyNodeConfig(node) {
  try {
    const response = await proxiesApi.generateLink(node.id)
    const link = response?.link
    await copyText(link)
    ElMessage.success(`已复制 ${node.name} 配置`)
  } catch (error) {
    ElMessage.error(`复制失败: ${node.name}`)
  }
}

// 加载数据
async function loadNodes() {
  loading.value = true
  try {
    await nodesStore.fetchNodes()
  } catch (error) {
    ElMessage.error('加载节点列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadNodes()
})
</script>

<style scoped>
.nodes-page {
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

/* 筛选栏 */
.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 12px;
}

.filter-left,
.filter-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 加载状态 */
.loading-state {
  text-align: center;
  padding: 60px 0;
  color: #909399;
}

.loading-icon {
  font-size: 32px;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* 卡片网格 */
.nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

/* 表格样式 */
.nodes-table {
  border-radius: 8px;
}

.node-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.node-flag {
  font-size: 18px;
}

.node-name {
  font-weight: 500;
}

.load-text {
  font-size: 12px;
  color: #909399;
  margin-left: 8px;
}

.latency-testing {
  color: #409eff;
}

.latency-good {
  color: #67c23a;
  font-weight: 500;
}

.latency-fair {
  color: #e6a23c;
  font-weight: 500;
}

.latency-poor {
  color: #f56c6c;
  font-weight: 500;
}

.latency-unknown {
  color: #c0c4cc;
}

/* 响应式 */
@media (max-width: 768px) {
  .filter-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .filter-left,
  .filter-right {
    display: grid;
    grid-template-columns: 1fr;
    flex-wrap: wrap;
  }

  .filter-left :deep(.el-select),
  .filter-right :deep(.el-select),
  .filter-right :deep(.el-button-group),
  .filter-right :deep(.el-button) {
    width: 100% !important;
  }

  .nodes-grid {
    grid-template-columns: 1fr;
  }
}
</style>
