<template>
  <div class="test-page-container">
    <div class="page-header">
      <div class="title">
        VMess 协议测试
      </div>
    </div>
    
    <el-card
      shadow="hover"
      class="test-card"
    >
      <template #header>
        <div class="card-header">
          <span>测试 VMess 配置</span>
        </div>
      </template>
      
      <div class="form-container">
        <el-form 
          :model="configForm" 
          label-position="top"
          label-width="100px"
        >
          <el-row :gutter="20">
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="连接名称">
                <el-input
                  v-model="configForm.name"
                  placeholder="请输入连接名称"
                />
              </el-form-item>
            </el-col>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="服务器地址">
                <el-input
                  v-model="configForm.server"
                  placeholder="请输入服务器地址"
                />
              </el-form-item>
            </el-col>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="端口">
                <el-input-number 
                  v-model="configForm.port" 
                  :min="1" 
                  :max="65535" 
                  style="width: 100%" 
                  controls-position="right" 
                />
              </el-form-item>
            </el-col>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="UUID">
                <el-input 
                  v-model="configForm.uuid" 
                  placeholder="请输入UUID" 
                  show-password 
                >
                  <template #append>
                    <el-button @click="generateRandomUUID">
                      随机
                    </el-button>
                  </template>
                </el-input>
              </el-form-item>
            </el-col>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="Alter ID">
                <el-input-number 
                  v-model="configForm.alterId" 
                  :min="0" 
                  :max="65535" 
                  style="width: 100%" 
                  controls-position="right" 
                />
              </el-form-item>
            </el-col>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="安全类型">
                <el-select
                  v-model="configForm.security"
                  style="width: 100%"
                >
                  <el-option
                    label="无加密"
                    value="none"
                  />
                  <el-option
                    label="TLS"
                    value="tls"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            
            <el-col
              v-if="configForm.security === 'tls'"
              :xs="24"
              :md="12"
            >
              <el-form-item label="SNI">
                <el-input
                  v-model="configForm.sni"
                  placeholder="请输入SNI"
                />
              </el-form-item>
            </el-col>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="传输方式">
                <el-select
                  v-model="configForm.transport"
                  style="width: 100%"
                >
                  <el-option
                    label="TCP"
                    value="tcp"
                  />
                  <el-option
                    label="WebSocket"
                    value="ws"
                  />
                  <el-option
                    label="HTTP/2"
                    value="http"
                  />
                  <el-option
                    label="gRPC"
                    value="grpc"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="伪装类型">
                <el-select
                  v-model="configForm.headerType"
                  style="width: 100%"
                >
                  <el-option
                    label="无伪装"
                    value="none"
                  />
                  <el-option
                    label="HTTP"
                    value="http"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            
            <!-- WebSocket 特有设置 -->
            <template v-if="configForm.transport === 'ws'">
              <el-col
                :xs="24"
                :md="12"
              >
                <el-form-item label="WebSocket 路径">
                  <el-input
                    v-model="configForm.wsPath"
                    placeholder="/vmess-ws"
                  />
                </el-form-item>
              </el-col>
              
              <el-col
                :xs="24"
                :md="12"
              >
                <el-form-item label="WebSocket 主机">
                  <el-input
                    v-model="configForm.wsHost"
                    placeholder="example.com"
                  />
                </el-form-item>
              </el-col>
            </template>
            
            <!-- HTTP/2 特有设置 -->
            <template v-if="configForm.transport === 'http'">
              <el-col
                :xs="24"
                :md="12"
              >
                <el-form-item label="HTTP/2 路径">
                  <el-input
                    v-model="configForm.httpPath"
                    placeholder="/vmess-h2"
                  />
                </el-form-item>
              </el-col>
              
              <el-col
                :xs="24"
                :md="12"
              >
                <el-form-item label="HTTP/2 主机">
                  <el-input
                    v-model="configForm.httpHost"
                    placeholder="example.com"
                  />
                </el-form-item>
              </el-col>
            </template>
            
            <!-- gRPC 特有设置 -->
            <template v-if="configForm.transport === 'grpc'">
              <el-col
                :xs="24"
                :md="12"
              >
                <el-form-item label="gRPC 服务名称">
                  <el-input
                    v-model="configForm.grpcServiceName"
                    placeholder="vmess-grpc-service"
                  />
                </el-form-item>
              </el-col>
            </template>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="跳过证书验证">
                <el-switch v-model="configForm.allowInsecure" />
              </el-form-item>
            </el-col>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="备注">
                <el-input
                  v-model="configForm.remark"
                  placeholder="自定义备注"
                />
              </el-form-item>
            </el-col>
          </el-row>
          
          <div class="actions">
            <el-button
              type="primary"
              @click="applyConfig"
            >
              应用配置
            </el-button>
            <el-button @click="resetConfig">
              重置
            </el-button>
          </div>
        </el-form>
      </div>
    </el-card>
    
    <el-card
      shadow="hover"
      class="test-card"
    >
      <template #header>
        <div class="card-header">
          <span>VMess 二维码</span>
        </div>
      </template>
      
      <div class="qrcode-container">
        <VMessQRCode 
          :connection-name="configForm.name"
          :server="configForm.server"
          :port="configForm.port"
          :uuid="configForm.uuid"
          :alter-id="configForm.alterId"
          :security="configForm.security"
          :sni="configForm.sni"
          :transport="configForm.transport"
          :header-type="configForm.headerType"
          :ws-path="configForm.wsPath"
          :ws-host="configForm.wsHost"
          :http-path="configForm.httpPath"
          :http-host="configForm.httpHost"
          :grpc-service-name="configForm.grpcServiceName"
          :allow-insecure="configForm.allowInsecure"
          :remark="configForm.remark"
        />
      </div>
    </el-card>
    
    <el-card
      shadow="hover"
      class="test-card"
    >
      <template #header>
        <div class="card-header">
          <span>预设配置</span>
        </div>
      </template>
      
      <div class="presets">
        <el-button
          type="primary"
          @click="loadPreset('tcp')"
        >
          TCP 预设
        </el-button>
        <el-button
          type="success"
          @click="loadPreset('ws')"
        >
          WebSocket 预设
        </el-button>
        <el-button
          type="warning"
          @click="loadPreset('http')"
        >
          HTTP/2 预设
        </el-button>
        <el-button
          type="info"
          @click="loadPreset('grpc')"
        >
          gRPC 预设
        </el-button>
        <el-button @click="loadRealConfig">
          加载真实配置
        </el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { reactive } from 'vue'
import { ElMessage } from 'element-plus'
import VMessQRCode from '../components/VMessQRCode.vue'

// 配置表单
const configForm = reactive({
  name: 'VMess 测试连接',
  server: 'vmess.example.com',
  port: 443,
  uuid: '38cb2d91-9411-4568-a8c2-8f39e5f04196',
  alterId: 0,
  security: 'tls',
  sni: 'vmess.example.com',
  transport: 'tcp',
  headerType: 'none',
  wsPath: '',
  wsHost: '',
  httpPath: '',
  httpHost: '',
  grpcServiceName: '',
  allowInsecure: false,
  remark: ''
})

// 生成随机UUID
const generateRandomUUID = () => {
  const uuid = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8)
    return v.toString(16)
  })
  configForm.uuid = uuid
}

// 应用配置
const applyConfig = () => {
  // 在这里可以添加配置验证逻辑
  if (!configForm.server) {
    ElMessage.error('请输入服务器地址')
    return
  }
  
  if (!configForm.uuid) {
    ElMessage.error('请输入UUID')
    return
  }
  
  ElMessage.success('配置已应用')
}

// 重置配置
const resetConfig = () => {
  configForm.name = 'VMess 测试连接'
  configForm.server = 'vmess.example.com'
  configForm.port = 443
  configForm.uuid = '38cb2d91-9411-4568-a8c2-8f39e5f04196'
  configForm.alterId = 0
  configForm.security = 'tls'
  configForm.sni = 'vmess.example.com'
  configForm.transport = 'tcp'
  configForm.headerType = 'none'
  configForm.wsPath = ''
  configForm.wsHost = ''
  configForm.httpPath = ''
  configForm.httpHost = ''
  configForm.grpcServiceName = ''
  configForm.allowInsecure = false
  configForm.remark = ''
  
  ElMessage.info('配置已重置')
}

// 加载预设配置
const loadPreset = (type) => {
  // 基础配置
  configForm.name = `VMess ${type.toUpperCase()} 测试`
  configForm.server = 'vmess.example.com'
  configForm.port = 443
  configForm.uuid = '38cb2d91-9411-4568-a8c2-8f39e5f04196'
  configForm.alterId = 0
  configForm.security = 'tls'
  configForm.sni = 'vmess.example.com'
  configForm.remark = `VMess ${type.toUpperCase()} 配置`
  configForm.allowInsecure = false
  
  // 传输方式特有配置
  switch (type) {
    case 'tcp':
      configForm.transport = 'tcp'
      configForm.headerType = 'none'
      configForm.wsPath = ''
      configForm.wsHost = ''
      configForm.httpPath = ''
      configForm.httpHost = ''
      configForm.grpcServiceName = ''
      break
      
    case 'ws':
      configForm.transport = 'ws'
      configForm.headerType = 'none'
      configForm.wsPath = '/vmess-ws'
      configForm.wsHost = 'vmess.example.com'
      configForm.httpPath = ''
      configForm.httpHost = ''
      configForm.grpcServiceName = ''
      break
      
    case 'http':
      configForm.transport = 'http'
      configForm.headerType = 'none'
      configForm.wsPath = ''
      configForm.wsHost = ''
      configForm.httpPath = '/vmess-h2'
      configForm.httpHost = 'vmess.example.com'
      configForm.grpcServiceName = ''
      break
      
    case 'grpc':
      configForm.transport = 'grpc'
      configForm.headerType = 'none'
      configForm.wsPath = ''
      configForm.wsHost = ''
      configForm.httpPath = ''
      configForm.httpHost = ''
      configForm.grpcServiceName = 'vmess-grpc-service'
      break
      
    default:
      break
  }
  
  ElMessage.success(`已加载 ${type.toUpperCase()} 预设`)
}

// 加载真实配置样例
const loadRealConfig = () => {
  configForm.name = 'VMess1'
  configForm.server = 'vmess1.example.com'
  configForm.port = 443
  configForm.uuid = 'a3d304e2-7a23-4fe0-b8a4-c58b82563a4f'
  configForm.alterId = 0
  configForm.security = 'tls'
  configForm.sni = 'vmess1.example.com'
  configForm.transport = 'ws'
  configForm.headerType = 'none'
  configForm.wsPath = '/ws'
  configForm.wsHost = 'vmess1.example.com'
  configForm.httpPath = ''
  configForm.httpHost = ''
  configForm.grpcServiceName = ''
  configForm.allowInsecure = false
  configForm.remark = 'VMess1'
  
  ElMessage.success('已加载真实配置')
}
</script>

<style scoped>
.test-page-container {
  padding: 20px;
  background-color: #f5f7fa;
  min-height: 100vh;
}

.page-header {
  margin-bottom: 20px;
}

.title {
  font-size: 22px;
  font-weight: 600;
  color: #303133;
}

.test-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header span {
  font-size: 16px;
  font-weight: 600;
}

.form-container {
  padding: 0 10px;
}

.actions {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.qrcode-container {
  display: flex;
  justify-content: center;
  padding: 10px;
}

.presets {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  justify-content: center;
}

@media (max-width: 768px) {
  .presets {
    flex-direction: column;
  }
  
  .presets .el-button {
    width: 100%;
  }
}
</style> 