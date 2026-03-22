<template>
  <div class="protocol-settings-container">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">
          协议设置
        </h1>
        <p class="page-subtitle">
          统一管理基础协议、传输层能力和 Trojan 默认参数
        </p>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">已启用协议</span>
        <strong class="overview-value is-success">{{ enabledProtocolCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">已启用传输层</span>
        <strong class="overview-value is-primary">{{ enabledTransportCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">Trojan 默认端口</span>
        <strong class="overview-value">{{ trojanDefaultPort }}</strong>
      </div>
    </div>

    <el-card
      shadow="hover"
      class="settings-card"
    >
      <template #header>
        <div class="card-header">
          <span>基础协议设置</span>
          <el-button
            type="primary"
            @click="saveSettings"
          >
            保存设置
          </el-button>
        </div>
      </template>
      
      <div class="protocol-grid">
        <div class="protocol-item">
          <div class="protocol-label">
            启用 Trojan 协议
          </div>
          <el-switch v-model="enableTrojan" />
          <div class="protocol-description">
            Trojan 协议: 基于 TLS 的轻量级协议，伪装成 HTTPS 流量。
            <el-tag
              v-if="enableTrojan"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
        
        <div class="protocol-item">
          <div class="protocol-label">
            启用 VMess 协议
          </div>
          <el-switch v-model="enableVMess" />
          <div class="protocol-description">
            VMess 协议: V2Ray 的核心传输协议，支持多种传输层。
            <el-tag
              v-if="enableVMess"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
        
        <div class="protocol-item">
          <div class="protocol-label">
            启用 VLESS 协议
          </div>
          <el-switch v-model="enableVLESS" />
          <div class="protocol-description">
            VLESS 协议: 轻量化的 VMess 协议，去除不必要的加密。
            <el-tag
              v-if="enableVLESS"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
        
        <div class="protocol-item">
          <div class="protocol-label">
            启用 Shadowsocks 协议
          </div>
          <el-switch v-model="enableShadowsocks" />
          <div class="protocol-description">
            Shadowsocks 协议: 经典的加密代理协议。
            <el-tag
              v-if="enableShadowsocks"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
        
        <div class="protocol-item">
          <div class="protocol-label">
            启用 SOCKS 协议
          </div>
          <el-switch v-model="enableSocks" />
          <div class="protocol-description">
            SOCKS 协议: 标准代理协议，支持 TCP/UDP。
            <el-tag
              v-if="enableSocks"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
        
        <div class="protocol-item">
          <div class="protocol-label">
            启用 HTTP 协议
          </div>
          <el-switch v-model="enableHTTP" />
          <div class="protocol-description">
            HTTP 协议: 基础代理协议，明文传输。
            <el-tag
              v-if="enableHTTP"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
      </div>
    </el-card>
    
    <el-card
      shadow="hover"
      class="settings-card"
    >
      <template #header>
        <div class="card-header">
          <span>传输层设置</span>
        </div>
      </template>
      
      <div class="protocol-grid">
        <div class="protocol-item">
          <div class="protocol-label">
            启用 TCP 传输
          </div>
          <el-switch v-model="enableTCP" />
          <div class="protocol-description">
            TCP 传输: 最基础的传输方式。
            <el-tag
              v-if="enableTCP"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
        
        <div class="protocol-item">
          <div class="protocol-label">
            启用 WebSocket 传输
          </div>
          <el-switch v-model="enableWebSocket" />
          <div class="protocol-description">
            WebSocket 传输: 基于HTTP协议的持久化连接，兼容性好。
            <el-tag
              v-if="enableWebSocket"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
        
        <div class="protocol-item">
          <div class="protocol-label">
            启用 HTTP/2 传输
          </div>
          <el-switch v-model="enableHTTP2" />
          <div class="protocol-description">
            HTTP/2 传输: 新一代HTTP协议，多路复用，需启用TLS。
            <el-tag
              v-if="enableHTTP2"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
        
        <div class="protocol-item">
          <div class="protocol-label">
            启用 gRPC 传输
          </div>
          <el-switch v-model="enableGRPC" />
          <div class="protocol-description">
            gRPC 传输: 基于HTTP/2的高性能RPC框架，抗干扰力强。
            <el-tag
              v-if="enableGRPC"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
        
        <div class="protocol-item">
          <div class="protocol-label">
            启用 QUIC 传输
          </div>
          <el-switch v-model="enableQUIC" />
          <div class="protocol-description">
            QUIC 传输: 基于UDP的传输层协议，低延迟。
            <el-tag
              v-if="enableQUIC"
              type="success"
              size="small"
              style="margin-left: 10px"
            >
              已启用
            </el-tag>
            <el-tag
              v-else
              type="info"
              size="small"
              style="margin-left: 10px"
            >
              已禁用
            </el-tag>
          </div>
        </div>
      </div>
    </el-card>
    
    <el-card
      shadow="hover"
      class="settings-card"
    >
      <template #header>
        <div class="card-header">
          <span>Trojan 协议高级设置</span>
        </div>
      </template>
      
      <el-form label-position="top">
        <el-form-item label="Trojan 默认端口">
          <el-input-number 
            v-model="trojanDefaultPort" 
            :min="1" 
            :max="65535" 
            :step="1" 
            style="width: 100%"
            controls-position="right" 
          />
          <div class="setting-description">
            设置 Trojan 协议的默认端口，通常为 443
          </div>
        </el-form-item>
        
        <el-form-item label="Trojan 默认发送/接收窗口大小">
          <el-row :gutter="isMobile ? 12 : 20">
            <el-col :span="isMobile ? 24 : 12">
              <el-input-number 
                v-model="trojanSendWindow" 
                :min="1" 
                :max="2147483647" 
                :step="1024" 
                style="width: 100%"
                controls-position="right" 
              />
              <div class="setting-description">
                发送窗口大小
              </div>
            </el-col>
            <el-col :span="isMobile ? 24 : 12">
              <el-input-number 
                v-model="trojanReceiveWindow" 
                :min="1" 
                :max="2147483647" 
                :step="1024" 
                style="width: 100%"
                controls-position="right" 
              />
              <div class="setting-description">
                接收窗口大小
              </div>
            </el-col>
          </el-row>
        </el-form-item>
        
        <el-form-item label="TCP 快速打开">
          <el-switch v-model="trojanTCPFastOpen" />
          <div class="setting-description">
            启用 TCP Fast Open，可能会提高连接速度（取决于系统支持）
          </div>
        </el-form-item>
        
        <el-form-item label="默认流控模式">
          <el-select
            v-model="trojanDefaultFlow"
            style="width: 100%"
          >
            <el-option
              label="无流控"
              value="none"
            />
            <el-option
              label="xtls-rprx-direct"
              value="xtls-rprx-direct"
            />
            <el-option
              label="xtls-rprx-direct-udp443"
              value="xtls-rprx-direct-udp443"
            />
            <el-option
              label="xtls-rprx-splice"
              value="xtls-rprx-splice"
            />
            <el-option
              label="xtls-rprx-splice-udp443"
              value="xtls-rprx-splice-udp443"
            />
          </el-select>
          <div class="setting-description">
            设置 Trojan 默认流控模式，影响数据传输方式
          </div>
        </el-form-item>
        
        <el-form-item label="密码策略">
          <el-select
            v-model="trojanPasswordPolicy"
            style="width: 100%"
          >
            <el-option
              label="任意字符"
              value="any"
            />
            <el-option
              label="仅允许字母和数字"
              value="alphanumeric"
            />
            <el-option
              label="强密码要求"
              value="strong"
            />
          </el-select>
          <div class="setting-description">
            设置 Trojan 密码生成和验证策略
          </div>
        </el-form-item>
      </el-form>
      
      <div class="form-actions">
        <el-button
          type="primary"
          @click="saveTrojanSettings"
        >
          保存 Trojan 设置
        </el-button>
        <el-button
          type="danger"
          @click="resetTrojanSettings"
        >
          重置为默认
        </el-button>
      </div>
    </el-card>
    
    <el-card
      shadow="hover"
      class="settings-card"
    >
      <template #header>
        <div class="card-header">
          <span>全局操作</span>
        </div>
      </template>
      
      <div class="global-actions">
        <el-button
          type="primary"
          @click="saveAllSettings"
        >
          保存所有设置
        </el-button>
        <el-button
          type="success"
          @click="saveAndRestart"
        >
          保存并重启服务
        </el-button>
        <el-button
          type="warning"
          @click="resetAllSettings"
        >
          重置所有设置
        </el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useViewport } from '@/composables/useViewport'

const { isMobile } = useViewport()

// 协议开关
const enableTrojan = ref(true)
const enableVMess = ref(true)
const enableVLESS = ref(true)
const enableShadowsocks = ref(true)
const enableSocks = ref(false)
const enableHTTP = ref(false)

// 传输层设置
const enableTCP = ref(true)
const enableWebSocket = ref(true)
const enableHTTP2 = ref(true)
const enableGRPC = ref(true)
const enableQUIC = ref(false)

// Trojan 协议高级设置
const trojanDefaultPort = ref(443)
const trojanSendWindow = ref(2 * 1024 * 1024) // 2MB
const trojanReceiveWindow = ref(2 * 1024 * 1024) // 2MB
const trojanTCPFastOpen = ref(true)
const trojanDefaultFlow = ref('none')
const trojanPasswordPolicy = ref('alphanumeric')

const enabledProtocolCount = computed(() =>
  [
    enableTrojan.value,
    enableVMess.value,
    enableVLESS.value,
    enableShadowsocks.value,
    enableSocks.value,
    enableHTTP.value
  ].filter(Boolean).length
)

const enabledTransportCount = computed(() =>
  [
    enableTCP.value,
    enableWebSocket.value,
    enableHTTP2.value,
    enableGRPC.value,
    enableQUIC.value
  ].filter(Boolean).length
)

// 保存基础设置
const saveSettings = async () => {
  try {
    await new Promise(resolve => setTimeout(resolve, 500))
    
    const protocols = {
      trojan: enableTrojan.value,
      vmess: enableVMess.value,
      vless: enableVLESS.value,
      shadowsocks: enableShadowsocks.value,
      socks: enableSocks.value,
      http: enableHTTP.value
    }
    
    const transports = {
      tcp: enableTCP.value,
      ws: enableWebSocket.value,
      http2: enableHTTP2.value,
      grpc: enableGRPC.value,
      quic: enableQUIC.value
    }
    
    console.log('保存协议设置:', { protocols, transports })
    ElMessage.success('协议设置已保存')
  } catch (error) {
    console.error('保存设置失败:', error)
    ElMessage.error('保存设置失败')
  }
}

// 保存Trojan高级设置
const saveTrojanSettings = async () => {
  try {
    await new Promise(resolve => setTimeout(resolve, 500))
    
    const trojanSettings = {
      defaultPort: trojanDefaultPort.value,
      sendWindow: trojanSendWindow.value,
      receiveWindow: trojanReceiveWindow.value,
      tcpFastOpen: trojanTCPFastOpen.value,
      defaultFlow: trojanDefaultFlow.value,
      passwordPolicy: trojanPasswordPolicy.value
    }
    
    console.log('保存Trojan高级设置:', trojanSettings)
    ElMessage.success('Trojan高级设置已保存')
  } catch (error) {
    console.error('保存Trojan设置失败:', error)
    ElMessage.error('保存Trojan设置失败')
  }
}

// 重置Trojan设置为默认值
const resetTrojanSettings = () => {
  ElMessageBox.confirm(
    '确定要将Trojan设置重置为默认值吗？',
    '重置确认',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  )
    .then(() => {
      trojanDefaultPort.value = 443
      trojanSendWindow.value = 2 * 1024 * 1024
      trojanReceiveWindow.value = 2 * 1024 * 1024
      trojanTCPFastOpen.value = true
      trojanDefaultFlow.value = 'none'
      trojanPasswordPolicy.value = 'alphanumeric'
      
      ElMessage.success('Trojan设置已重置为默认值')
    })
    .catch(() => {
      ElMessage.info('已取消重置')
    })
}

// 保存所有设置
const saveAllSettings = async () => {
  try {
    await saveSettings()
    await saveTrojanSettings()
    ElMessage.success('所有设置已保存')
  } catch (error) {
    console.error('保存所有设置失败:', error)
    ElMessage.error('保存所有设置失败')
  }
}

// 保存并重启服务
const saveAndRestart = async () => {
  ElMessageBox.confirm(
    '确定要保存所有设置并重启服务吗？重启期间所有连接将会断开。',
    '重启确认',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  )
    .then(async () => {
      try {
        await saveAllSettings()
        
        // 重启服务
        ElMessage({
          message: '正在重启服务...',
          type: 'info',
          duration: 0,
          showClose: true
        })
        
        await new Promise(resolve => setTimeout(resolve, 2000))
        
        ElMessage.closeAll()
        ElMessage.success('服务已成功重启')
      } catch (error) {
        console.error('重启服务失败:', error)
        ElMessage.error('重启服务失败')
      }
    })
    .catch(() => {
      ElMessage.info('已取消重启')
    })
}

// 重置所有设置
const resetAllSettings = () => {
  ElMessageBox.confirm(
    '确定要将所有设置重置为默认值吗？',
    '重置确认',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  )
    .then(() => {
      // 重置协议设置
      enableTrojan.value = true
      enableVMess.value = true
      enableVLESS.value = true
      enableShadowsocks.value = true
      enableSocks.value = false
      enableHTTP.value = false
      
      // 重置传输层设置
      enableTCP.value = true
      enableWebSocket.value = true
      enableHTTP2.value = true
      enableGRPC.value = true
      enableQUIC.value = false
      
      // 重置Trojan设置
      resetTrojanSettings()
      
      ElMessage.success('所有设置已重置为默认值')
    })
    .catch(() => {
      ElMessage.info('已取消重置')
    })
}
</script>

<style scoped>
.protocol-settings-container {
  padding: 20px;
  background-color: #f5f7fa;
  min-height: 100vh;
}

.page-header {
  margin-bottom: 20px;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: #1f2937;
  margin: 0 0 8px;
}

.page-subtitle {
  margin: 0;
  font-size: 14px;
  color: #6b7280;
}

.overview-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.overview-card {
  padding: 18px 20px;
  border-radius: 16px;
  border: 1px solid #e5e7eb;
  background: linear-gradient(135deg, #ffffff 0%, #f8fafc 100%);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.05);
}

.overview-label {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  color: #64748b;
}

.overview-value {
  font-size: 22px;
  font-weight: 700;
  color: #111827;
}

.overview-value.is-success {
  color: #059669;
}

.overview-value.is-primary {
  color: #2563eb;
}

.settings-card {
  margin-bottom: 20px;
  border-radius: 18px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.card-header span {
  font-size: 16px;
  font-weight: 600;
}

.protocol-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
}

.protocol-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
  min-height: 100%;
  border-radius: 14px;
  background-color: #f8f9fa;
}

.protocol-label {
  font-size: 16px;
  font-weight: 500;
  color: #303133;
}

.protocol-description {
  font-size: 14px;
  color: #606266;
  margin-top: 8px;
}

.setting-description {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.form-actions, .global-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 20px;
}

@media (max-width: 768px) {
  .protocol-settings-container {
    padding: 12px;
  }

  .overview-strip {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .protocol-grid {
    grid-template-columns: 1fr;
  }

  .card-header {
    align-items: flex-start;
  }

  .form-actions,
  .global-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .form-actions .el-button,
  .global-actions .el-button {
    width: 100%;
    margin-left: 0;
  }
}
</style>
