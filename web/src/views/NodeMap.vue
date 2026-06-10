<template>
  <div class="node-map-page">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          节点地理分布
        </h1>
        <p class="page-subtitle">
          用真实世界地图查看节点覆盖区域、状态分布和地区负载密度
        </p>
      </div>
      <div class="page-actions">
        <el-button
          :loading="loading"
          @click="fetchNodes"
        >
          <el-icon class="el-icon--left">
            <Refresh />
          </el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">当前视图节点</span>
        <strong class="overview-value">{{ visibleNodeCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">在线节点</span>
        <strong class="overview-value is-success">{{ visibleOnlineCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">覆盖地区</span>
        <strong class="overview-value is-primary">{{ mappedRegionsCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">平均延迟</span>
        <strong class="overview-value is-warning">{{ visibleAverageLatency }}ms</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">热点地区</span>
        <strong class="overview-value">{{ topRegionName }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-filters">
        <span class="toolbar-summary">当前区域：{{ selectedRegionLabel }}</span>
        <span class="toolbar-summary">在线 {{ visibleOnlineCount }} / 离线 {{ visibleOfflineCount }} / 异常 {{ visibleUnhealthyCount }}</span>
      </div>
      <div class="toolbar-actions">
        <el-button
          v-if="selectedRegion"
          @click="clearRegionFilter"
        >
          显示全部地区
        </el-button>
      </div>
    </div>

    <div class="map-layout">
      <el-card
        shadow="never"
        class="map-card"
      >
        <template #header>
          <div class="card-header map-card__header">
            <div class="map-card__heading">
              <span class="panel-title">全球节点散点图</span>
              <span class="toolbar-summary">可拖拽、可缩放；点击节点选中，点击右侧地区筛选视图</span>
            </div>
            <span :class="['metric-pill', selectedRegion ? 'is-primary' : 'is-muted']">
              {{ selectedRegionLabel }}
            </span>
          </div>
        </template>

        <div class="map-stage">
          <div
            ref="mapChartRef"
            class="map-chart"
          />
          <div class="map-stage__glow map-stage__glow--top" />
          <div class="map-stage__glow map-stage__glow--bottom" />
        </div>

        <div class="map-legend">
          <div class="legend-item">
            <span class="legend-dot online" />
            <span>在线 {{ visibleOnlineCount }}</span>
          </div>
          <div class="legend-item">
            <span class="legend-dot offline" />
            <span>离线 {{ visibleOfflineCount }}</span>
          </div>
          <div class="legend-item">
            <span class="legend-dot unhealthy" />
            <span>不健康 {{ visibleUnhealthyCount }}</span>
          </div>
        </div>

        <div class="focus-strip">
          <button
            v-for="region in topRegions"
            :key="region.name"
            type="button"
            class="focus-card"
            :class="{ active: selectedRegion === region.name }"
            @click="filterByRegion(region.name)"
          >
            <span class="focus-card__title">{{ region.name }}</span>
            <span class="focus-card__meta">{{ region.count }} 节点</span>
            <span class="focus-card__value">{{ region.online }} 在线</span>
          </button>
        </div>
      </el-card>

      <div class="side-panels">
        <el-card
          shadow="never"
          class="region-card"
        >
          <template #header>
            <div class="card-header">
              <span>地区热度榜</span>
              <span class="toolbar-summary">{{ regionStats.length }} 个地区</span>
            </div>
          </template>

          <div class="region-list">
            <button
              v-for="region in regionStats"
              :key="region.name"
              type="button"
              class="region-item"
              :class="{ active: selectedRegion === region.name }"
              @click="filterByRegion(region.name)"
            >
              <div class="region-item__top">
                <div class="region-info">
                  <span class="region-name">{{ region.name }}</span>
                  <span class="region-count">{{ region.count }} 节点 / {{ region.users }} 用户</span>
                </div>
                <span :class="['metric-pill', getRegionStatusClass(region)]">
                  {{ getRegionHealthRate(region) }}%
                </span>
              </div>
              <div class="region-item__bottom">
                <span class="toolbar-summary">平均延迟 {{ region.avgLatency }}ms</span>
                <div class="region-status">
                  <span
                    v-if="region.online > 0"
                    class="status-dot online"
                  >{{ region.online }}</span>
                  <span
                    v-if="region.offline > 0"
                    class="status-dot offline"
                  >{{ region.offline }}</span>
                  <span
                    v-if="region.unhealthy > 0"
                    class="status-dot unhealthy"
                  >{{ region.unhealthy }}</span>
                </div>
              </div>
            </button>
          </div>
        </el-card>

        <el-card
          shadow="never"
          class="detail-card"
        >
          <template #header>
            <div class="card-header">
              <span>{{ selectedNode ? '节点详情' : '地图说明' }}</span>
              <el-button
                v-if="selectedNode"
                link
                @click="selectedNode = null"
              >
                <el-icon><Close /></el-icon>
              </el-button>
            </div>
          </template>

          <template v-if="selectedNode">
            <div class="stack-cell">
              <div class="stack-item">
                <span class="stack-label">名称</span>
                <span class="stack-value is-strong">{{ selectedNode.name }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">地址</span>
                <span class="stack-value">{{ selectedNode.address }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">地区</span>
                <span class="stack-value">{{ selectedNode.region || '-' }}</span>
              </div>
              <div class="stack-item stack-item--inline">
                <span class="stack-label">状态</span>
                <span :class="['metric-pill', getNodeStatusClass(selectedNode.status)]">
                  {{ getStatusText(selectedNode.status) }}
                </span>
              </div>
              <div class="stack-item">
                <span class="stack-label">用户数</span>
                <span class="stack-value">{{ selectedNode.current_users }}/{{ selectedNode.max_users || '∞' }}</span>
              </div>
              <div class="stack-item">
                <span class="stack-label">延迟</span>
                <span :class="['stack-value', getLatencyValueClass(selectedNode.latency)]">{{ selectedNode.latency }}ms</span>
              </div>
            </div>

            <div class="detail-card__actions">
              <el-button
                type="primary"
                @click="openSelectedNodeDetail"
              >
                查看详情
              </el-button>
            </div>
          </template>

          <template v-else>
            <div class="entity-cell">
              <div class="entity-cell__hint">
                在线节点会以更亮的散点呈现，节点数量越多、用户越多的区域会自然更密集。地图支持拖拽和缩放，适合快速观察全局部署是否均衡。
              </div>
              <div class="surface-inline">
                <div class="stack-item">
                  <span class="stack-label">当前区域</span>
                  <span class="stack-value">{{ selectedRegionLabel }}</span>
                </div>
                <div class="stack-item">
                  <span class="stack-label">节点规模</span>
                  <span class="stack-value">{{ visibleNodeCount }} 个</span>
                </div>
                <div class="stack-item">
                  <span class="stack-label">热点地区</span>
                  <span class="stack-value">{{ topRegionName }}</span>
                </div>
              </div>
            </div>
          </template>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useTheme } from '@/composables/useTheme'
import { Close, Refresh } from '@element-plus/icons-vue'
import * as echarts from 'echarts/core'
import { MapChart, ScatterChart, EffectScatterChart } from 'echarts/charts'
import { GeoComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { useNodeStore } from '@/stores/node'

echarts.use([GeoComponent, TooltipComponent, MapChart, ScatterChart, EffectScatterChart, CanvasRenderer])

const router = useRouter()
const nodeStore = useNodeStore()
const { isDark } = useTheme()

const loading = ref(false)
const selectedNode = ref(null)
const selectedRegion = ref('')
const mapChartRef = ref(null)

let mapChart = null
let resizeObserver = null
let worldMapReady = false

const REGION_COORDINATES = [
  { pattern: /(hong\s*kong|香港)/i, coord: [114.1694, 22.3193] },
  { pattern: /(taipei|taiwan|台北|台湾)/i, coord: [121.5654, 25.033] },
  { pattern: /(tokyo|osaka|japan|日本)/i, coord: [139.6917, 35.6895] },
  { pattern: /(seoul|korea|韩国)/i, coord: [126.978, 37.5665] },
  { pattern: /(singapore|新加坡)/i, coord: [103.8198, 1.3521] },
  { pattern: /(bangkok|thailand|泰国)/i, coord: [100.5018, 13.7563] },
  { pattern: /(ho\s*chi\s*minh|hanoi|vietnam|越南)/i, coord: [106.6297, 10.8231] },
  { pattern: /(manila|philippines|菲律宾)/i, coord: [120.9842, 14.5995] },
  { pattern: /(kuala\s*lumpur|malaysia|马来西亚)/i, coord: [101.6869, 3.139] },
  { pattern: /(jakarta|indonesia|印尼)/i, coord: [106.8456, -6.2088] },
  { pattern: /(mumbai|delhi|india|印度)/i, coord: [72.8777, 19.076] },
  { pattern: /(dubai|uae|阿联酋|迪拜)/i, coord: [55.2708, 25.2048] },
  { pattern: /(istanbul|turkey|土耳其)/i, coord: [28.9784, 41.0082] },
  { pattern: /(frankfurt|berlin|germany|德国)/i, coord: [8.6821, 50.1109] },
  { pattern: /(london|uk|britain|england|英国)/i, coord: [-0.1276, 51.5072] },
  { pattern: /(paris|france|法国)/i, coord: [2.3522, 48.8566] },
  { pattern: /(amsterdam|netherlands|荷兰)/i, coord: [4.9041, 52.3676] },
  { pattern: /(madrid|spain|西班牙)/i, coord: [-3.7038, 40.4168] },
  { pattern: /(milan|rome|italy|意大利)/i, coord: [12.4964, 41.9028] },
  { pattern: /(zurich|switzerland|瑞士)/i, coord: [8.5417, 47.3769] },
  { pattern: /(stockholm|sweden|瑞典)/i, coord: [18.0686, 59.3293] },
  { pattern: /(warsaw|poland|波兰)/i, coord: [21.0122, 52.2297] },
  { pattern: /(moscow|russia|俄罗斯)/i, coord: [37.6173, 55.7558] },
  { pattern: /(cairo|egypt|埃及)/i, coord: [31.2357, 30.0444] },
  { pattern: /(johannesburg|south\s*africa|南非)/i, coord: [28.0473, -26.2041] },
  { pattern: /(sydney|melbourne|australia|澳大利亚)/i, coord: [151.2093, -33.8688] },
  { pattern: /(auckland|new\s*zealand|新西兰)/i, coord: [174.7633, -36.8485] },
  { pattern: /(toronto|vancouver|montreal|canada|加拿大)/i, coord: [-79.3832, 43.6532] },
  { pattern: /(los\s*angeles|san\s*jose|seattle|fremont|san\s*francisco|us\s*west|美国西)/i, coord: [-118.2437, 34.0522] },
  { pattern: /(new\s*york|newark|virginia|washington|dallas|chicago|us\s*east|美国东)/i, coord: [-74.006, 40.7128] },
  { pattern: /(america|usa|united\s*states|美国)/i, coord: [-98.5795, 39.8283] },
  { pattern: /(mexico|mexico\s*city|墨西哥)/i, coord: [-99.1332, 19.4326] },
  { pattern: /(sao\s*paulo|rio|brazil|巴西)/i, coord: [-46.6333, -23.5505] },
  { pattern: /(santiago|chile|智利)/i, coord: [-70.6693, -33.4489] },
  { pattern: /(buenos\s*aires|argentina|阿根廷)/i, coord: [-58.3816, -34.6037] }
]

const visibleNodes = computed(() => {
  if (!selectedRegion.value) {
    return nodeStore.nodes
  }

  return nodeStore.nodes.filter((node) => node.region === selectedRegion.value)
})

const visibleNodeCount = computed(() => visibleNodes.value.length)
const visibleOnlineCount = computed(() => visibleNodes.value.filter((node) => node.status === 'online').length)
const visibleOfflineCount = computed(() => visibleNodes.value.filter((node) => node.status === 'offline').length)
const visibleUnhealthyCount = computed(() => visibleNodes.value.filter((node) => node.status === 'unhealthy').length)
const mappedRegionsCount = computed(() => regionStats.value.length)
const topRegionName = computed(() => regionStats.value[0]?.name || '暂无')
const topRegions = computed(() => regionStats.value.slice(0, 4))
const selectedRegionLabel = computed(() => selectedRegion.value || '全部地区')

const visibleAverageLatency = computed(() => {
  const nodes = visibleNodes.value.filter((node) => node.status === 'online' && node.latency > 0)
  if (!nodes.length) return 0
  return Math.round(nodes.reduce((sum, node) => sum + node.latency, 0) / nodes.length)
})

const regionStats = computed(() => {
  const stats = {}

  nodeStore.nodes.forEach((node) => {
    const region = node.region || '未知'
    if (!stats[region]) {
      stats[region] = {
        name: region,
        count: 0,
        online: 0,
        offline: 0,
        unhealthy: 0,
        users: 0,
        latencyTotal: 0,
        latencyCount: 0
      }
    }

    stats[region].count += 1
    stats[region].users += Number(node.current_users || 0)

    if (node.status === 'online') stats[region].online += 1
    else if (node.status === 'offline') stats[region].offline += 1
    else if (node.status === 'unhealthy') stats[region].unhealthy += 1

    if (node.latency > 0) {
      stats[region].latencyTotal += Number(node.latency)
      stats[region].latencyCount += 1
    }
  })

  return Object.values(stats)
    .map((region) => ({
      ...region,
      avgLatency: region.latencyCount ? Math.round(region.latencyTotal / region.latencyCount) : 0
    }))
    .sort((a, b) => b.count - a.count)
})

const getStatusText = (status) => {
  const texts = { online: '在线', offline: '离线', unhealthy: '不健康' }
  return texts[status] || status
}

const getNodeStatusClass = (status) => {
  const classes = { online: 'is-success', offline: 'is-muted', unhealthy: 'is-danger' }
  return classes[status] || 'is-muted'
}

const getLatencyValueClass = (latency) => {
  if (!latency) return ''
  if (latency < 100) return 'is-success'
  if (latency < 300) return 'is-warning'
  return 'is-danger'
}

const getRegionHealthRate = (region) => {
  if (!region.count) return 0
  return Math.round((region.online / region.count) * 100)
}

const getRegionStatusClass = (region) => {
  const rate = getRegionHealthRate(region)
  if (rate >= 80) return 'is-success'
  if (rate >= 50) return 'is-warning'
  return 'is-danger'
}

const buildLocationSearchText = (node) => {
  if (!node) return ''
  return [node.region, node.name, node.address]
    .filter(Boolean)
    .join(' ')
}

const resolveRegionCoordinate = (source) => {
  const normalizedSource = String(source || '').trim()
  const matched = REGION_COORDINATES.find((item) => item.pattern.test(normalizedSource))
  return matched?.coord || [0, 20]
}

const buildScatterGroups = () => {
  const groups = {
    online: [],
    offline: [],
    unhealthy: []
  }

  const offsets = {}

  visibleNodes.value.forEach((node) => {
    const locationSource = buildLocationSearchText(node)
    const regionKey = locationSource || 'default'
    const index = offsets[regionKey] || 0
    offsets[regionKey] = index + 1

    const baseCoord = resolveRegionCoordinate(locationSource)
    const col = index % 3
    const row = Math.floor(index / 3)
    const offsetLng = (col - 1) * 3.5
    const offsetLat = (row - 1) * 2.2

    const item = {
      name: node.name,
      value: [
        Number((baseCoord[0] + offsetLng).toFixed(2)),
        Number((baseCoord[1] + offsetLat).toFixed(2)),
        Number(node.current_users || 0)
      ],
      symbolSize: Math.max(12, Math.min(26, 12 + Number(node.current_users || 0) * 1.4)),
      nodeData: node
    }

    groups[node.status] = groups[node.status] || []
    groups[node.status].push(item)
  })

  return groups
}

const getThemeColor = (name, fallback) => {
  const appContainer = document.querySelector('.app-container')
  if (!appContainer) return fallback
  const value = getComputedStyle(appContainer).getPropertyValue(name).trim()
  return value || fallback
}

const updateChart = () => {
  if (!mapChart || !worldMapReady) return

  const scatterGroups = buildScatterGroups()
  const selectedScatterItem = selectedNode.value
    ? Object.values(scatterGroups)
        .flat()
        .find((item) => item.nodeData?.id === selectedNode.value.id)
    : null
  const textColor = getThemeColor('--admin-title', '#0f172a')
  const mutedColor = getThemeColor('--admin-text-muted', '#64748b')
  const borderColor = getThemeColor('--admin-border', 'rgba(148, 163, 184, 0.22)')
  const mapAreaColor = isDark.value
    ? 'rgba(71, 85, 105, 0.34)'
    : 'rgba(203, 213, 225, 0.34)'
  const mapCenter = selectedRegion.value
    ? resolveRegionCoordinate(selectedRegion.value)
    : [12, 18]

  const buildScatterSeries = (name, status, color) => ({
    name,
    type: 'scatter',
    coordinateSystem: 'geo',
    data: scatterGroups[status] || [],
    symbolSize: (value, params) => params.data?.symbolSize || 12,
    itemStyle: {
      color,
      shadowBlur: 14,
      shadowColor: color
    },
    emphasis: {
      scale: true,
      itemStyle: {
        borderColor: '#fff',
        borderWidth: 2
      }
    }
  })

  const option = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: 'rgba(15, 23, 42, 0.92)',
      borderWidth: 0,
      padding: 12,
      textStyle: {
        color: '#e2e8f0'
      },
      formatter: (params) => {
        const node = params.data?.nodeData
        if (!node) return ''

        return [
          `<div style="font-weight:700;margin-bottom:8px;">${node.name}</div>`,
          `<div>地区：${node.region || '-'}</div>`,
          `<div>状态：${getStatusText(node.status)}</div>`,
          `<div>用户：${node.current_users}/${node.max_users || '∞'}</div>`,
          `<div>延迟：${node.latency || 0}ms</div>`,
          `<div>地址：${node.address || '-'}</div>`
        ].join('')
      }
    },
    geo: {
      map: 'WORLD',
      roam: true,
      zoom: selectedRegion.value ? 1.4 : 1.12,
      center: mapCenter,
      scaleLimit: {
        min: 1,
        max: 8
      },
      itemStyle: {
        areaColor: mapAreaColor,
        borderColor,
        borderWidth: 0.8
      },
      emphasis: {
        label: {
          show: false
        },
        itemStyle: {
          areaColor: 'rgba(59, 130, 246, 0.18)'
        }
      }
    },
    series: [
      buildScatterSeries('在线', 'online', '#22c55e'),
      buildScatterSeries('离线', 'offline', '#94a3b8'),
      buildScatterSeries('不健康', 'unhealthy', '#ef4444'),
      {
        name: '选中节点',
        type: 'effectScatter',
        coordinateSystem: 'geo',
        data: selectedNode.value
          ? [
              {
                name: selectedNode.value.name,
                value: selectedScatterItem?.value || resolveRegionCoordinate(buildLocationSearchText(selectedNode.value)),
                symbolSize: 30,
                nodeData: selectedNode.value
              }
            ]
          : [],
        rippleEffect: {
          scale: 3.2,
          brushType: 'stroke'
        },
        itemStyle: {
          color: '#f97316',
          shadowBlur: 18,
          shadowColor: 'rgba(249, 115, 22, 0.48)'
        },
        zlevel: 3
      }
    ],
    graphic: [
      {
        type: 'text',
        left: 22,
        top: 18,
        style: {
          text: selectedRegion.value ? `已过滤：${selectedRegion.value}` : '全球视图',
          fill: mutedColor,
          fontSize: 12,
          fontWeight: 600
        }
      },
      {
        type: 'text',
        right: 22,
        top: 18,
        style: {
          text: `节点 ${visibleNodeCount.value}  ·  在线 ${visibleOnlineCount.value}`,
          fill: textColor,
          fontSize: 12,
          fontWeight: 700
        }
      }
    ]
  }

  mapChart.setOption(option, true)
}

const initChart = async () => {
  await nextTick()
  if (!mapChartRef.value) return

  if (!worldMapReady) {
    const worldGeoJsonModule = await import('@/assets/maps/world.json')
    echarts.registerMap('WORLD', worldGeoJsonModule.default || worldGeoJsonModule)
    worldMapReady = true
  }

  if (mapChart) {
    mapChart.dispose()
  }

  mapChart = echarts.init(mapChartRef.value)
  mapChart.on('click', (params) => {
    const node = params.data?.nodeData
    if (node) {
      selectedNode.value = node
    }
  })

  updateChart()

  if (resizeObserver) {
    resizeObserver.disconnect()
  }

  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => {
      mapChart?.resize()
    })
    resizeObserver.observe(mapChartRef.value)
  }
}

const filterByRegion = (region) => {
  selectedRegion.value = selectedRegion.value === region ? '' : region
}

const clearRegionFilter = () => {
  selectedRegion.value = ''
}

const openSelectedNodeDetail = () => {
  if (!selectedNode.value) return
  router.push(`/admin/nodes/${selectedNode.value.id}`)
}

const fetchNodes = async () => {
  loading.value = true
  try {
    await nodeStore.fetchNodes()
  } catch (e) {
    ElMessage.error(e.message || '获取节点列表失败')
  } finally {
    loading.value = false
  }
}

watch(
  () => [nodeStore.nodes, selectedRegion.value, selectedNode.value?.id, isDark.value],
  () => {
    if (selectedNode.value && selectedRegion.value && selectedNode.value.region !== selectedRegion.value) {
      selectedNode.value = null
    }
    updateChart()
  },
  { deep: true }
)

onMounted(async () => {
  await fetchNodes()
  await initChart()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  if (mapChart) {
    mapChart.dispose()
    mapChart = null
  }
})
</script>

<style scoped>
.node-map-page {
  padding: 20px;
}

.map-layout {
  display: grid;
  grid-template-columns: minmax(0, 2.1fr) minmax(300px, 360px);
  gap: 20px;
}

.side-panels {
  display: grid;
  gap: 20px;
  align-content: start;
}

.map-card {
  overflow: hidden;
}

.map-card__header {
  align-items: center;
}

.map-card__heading {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.map-stage {
  position: relative;
  min-height: 620px;
  border-radius: 22px;
  overflow: hidden;
  border: 1px solid rgba(37, 99, 235, 0.12);
  background:
    radial-gradient(circle at 10% 10%, rgba(56, 189, 248, 0.18), transparent 18%),
    radial-gradient(circle at 90% 20%, rgba(37, 99, 235, 0.16), transparent 20%),
    linear-gradient(180deg, #f8fbff 0%, #eef4fb 100%);
}

.map-chart {
  position: relative;
  z-index: 2;
  width: 100%;
  height: 620px;
}

.map-stage__glow {
  position: absolute;
  z-index: 1;
  width: 260px;
  height: 260px;
  border-radius: 999px;
  filter: blur(80px);
  opacity: 0.34;
  pointer-events: none;
}

.map-stage__glow--top {
  top: -70px;
  right: 0;
  background: rgba(59, 130, 246, 0.26);
}

.map-stage__glow--bottom {
  bottom: -90px;
  left: 0;
  background: rgba(14, 165, 233, 0.2);
}

:global(html.dark) .map-stage {
  border-color: rgba(96, 165, 250, 0.18);
  background:
    radial-gradient(circle at 10% 10%, rgba(56, 189, 248, 0.12), transparent 18%),
    radial-gradient(circle at 90% 20%, rgba(59, 130, 246, 0.14), transparent 20%),
    linear-gradient(180deg, #111827 0%, #0f172a 100%);
}

:global(html.dark) .map-stage__glow--top {
  background: rgba(59, 130, 246, 0.2);
}

:global(html.dark) .map-stage__glow--bottom {
  background: rgba(14, 165, 233, 0.16);
}

.map-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 18px;
  align-items: center;
  padding-top: 16px;
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--admin-text);
}

.legend-dot {
  width: 12px;
  height: 12px;
  border-radius: 999px;
}

.legend-dot.online {
  background: #22c55e;
}

.legend-dot.offline {
  background: #94a3b8;
}

.legend-dot.unhealthy {
  background: #ef4444;
}

.focus-strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
  margin-top: 18px;
}

.focus-card {
  display: grid;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid var(--admin-border);
  border-radius: 16px;
  background: var(--admin-surface-soft);
  text-align: left;
  cursor: pointer;
  transition: transform 0.18s ease, border-color 0.18s ease, background 0.18s ease;
}

.focus-card:hover {
  transform: translateY(-2px);
  border-color: var(--admin-border-strong);
}

.focus-card.active {
  border-color: rgba(37, 99, 235, 0.32);
  background: rgba(37, 99, 235, 0.08);
}

.focus-card__title {
  font-size: 13px;
  font-weight: 700;
  color: var(--admin-title);
}

.focus-card__meta {
  font-size: 12px;
  color: var(--admin-text-muted);
}

.focus-card__value {
  font-size: 13px;
  font-weight: 700;
  color: #15803d;
}

.region-list {
  display: grid;
  gap: 10px;
  max-height: 520px;
  overflow-y: auto;
}

.region-item {
  display: grid;
  gap: 10px;
  width: 100%;
  padding: 12px;
  border: 1px solid var(--admin-border);
  border-radius: 14px;
  background: var(--admin-surface-soft);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, transform 0.18s ease, background 0.18s ease;
}

.region-item:hover {
  transform: translateY(-2px);
  border-color: var(--admin-border-strong);
}

.region-item.active {
  border-color: rgba(37, 99, 235, 0.32);
  background: rgba(37, 99, 235, 0.08);
}

.region-item__top,
.region-item__bottom {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.region-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.region-name {
  font-size: 14px;
  font-weight: 700;
  color: var(--admin-title);
}

.region-count {
  font-size: 12px;
  color: var(--admin-text-muted);
}

.region-status {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.status-dot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 24px;
  height: 24px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  color: #fff;
}

.status-dot.online {
  background: #22c55e;
}

.status-dot.offline {
  background: #94a3b8;
}

.status-dot.unhealthy {
  background: #ef4444;
}

.detail-card__actions {
  margin-top: 16px;
}

.detail-card__actions .el-button {
  width: 100%;
}

@media (max-width: 1280px) {
  .map-layout {
    grid-template-columns: 1fr;
  }

  .map-stage,
  .map-chart {
    min-height: 480px;
    height: 480px;
  }
}

@media (max-width: 768px) {
  .node-map-page {
    padding: 12px;
  }

  .map-stage,
  .map-chart {
    min-height: 360px;
    height: 360px;
  }

  .region-item__top,
  .region-item__bottom {
    flex-direction: column;
    align-items: flex-start;
  }

  .focus-strip {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
