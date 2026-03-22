<template>
  <div class="test-page-container">
    <div class="page-header">
      <div class="title">
        Trojan 协议测试
      </div>
    </div>
    
    <el-card
      shadow="hover"
      class="test-card"
    >
      <template #header>
        <div class="card-header">
          <span>测试 Trojan 配置</span>
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
              <el-form-item label="密码">
                <el-input 
                  v-model="configForm.password" 
                  placeholder="请输入密码" 
                  show-password 
                >
                  <template #append>
                    <el-button @click="generateRandomPassword">
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
              <el-form-item label="安全类型">
                <el-select
                  v-model="configForm.security"
                  style="width: 100%"
                >
                  <el-option
                    label="TLS"
                    value="tls"
                  />
                  <el-option
                    label="XTLS"
                    value="xtls"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            
            <el-col
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
              <el-form-item label="跳过证书验证">
                <el-switch v-model="configForm.allowInsecure" />
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
                    placeholder="/trojan-ws"
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
                    placeholder="/trojan-h2"
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
                    placeholder="trojan-grpc-service"
                  />
                </el-form-item>
              </el-col>
            </template>
            
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
          <span>Trojan 二维码</span>
        </div>
      </template>
      
      <div class="qrcode-container">
        <TrojanQRCode 
          :connection-name="configForm.name"
          :server="configForm.server"
          :port="configForm.port"
          :password="configForm.password"
          :security="configForm.security"
          :sni="configForm.sni"
          :transport="configForm.transport"
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
        <el-button
          type="danger"
          @click="loadPreset('xtls')"
        >
          XTLS 预设
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
import TrojanQRCode from '../components/TrojanQRCode.vue'

// 配置表单
const configForm = reactive({
  name: 'Trojan 测试连接',
  server: 'trojan.example.com',
  port: 443,
  password: 'password123',
  security: 'tls',
  sni: 'trojan.example.com',
  transport: 'tcp',
  wsPath: '',
  wsHost: '',
  httpPath: '',
  httpHost: '',
  grpcServiceName: '',
  allowInsecure: false,
  remark: ''
})

// 生成随机密码
const generateRandomPassword = () => {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let password = ''
  for (let i = 0; i < 16; i++) {
    password += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  configForm.password = password
}

// 应用配置
const applyConfig = () => {
  // 在这里可以添加配置验证逻辑
  if (!configForm.server) {
    ElMessage.error('请输入服务器地址')
    return
  }
  
  if (!configForm.password) {
    ElMessage.error('请输入密码')
    return
  }
  
  ElMessage.success('配置已应用')
}

// 重置配置
const resetConfig = () => {
  configForm.name = 'Trojan 测试连接'
  configForm.server = 'trojan.example.com'
  configForm.port = 443
  configForm.password = 'password123'
  configForm.security = 'tls'
  configForm.sni = 'trojan.example.com'
  configForm.transport = 'tcp'
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
  configForm.name = `Trojan ${type.toUpperCase()} 测试`
  configForm.server = 'trojan.example.com'
  configForm.port = 443
  configForm.password = 'password123'
  configForm.security = 'tls'
  configForm.sni = 'trojan.example.com'
  configForm.remark = `Trojan ${type.toUpperCase()} 配置`
  configForm.allowInsecure = false
  
  // 传输方式特有配置
  switch (type) {
    case 'tcp':
      configForm.transport = 'tcp'
      configForm.wsPath = ''
      configForm.wsHost = ''
      configForm.httpPath = ''
      configForm.httpHost = ''
      configForm.grpcServiceName = ''
      break
      
    case 'ws':
      configForm.transport = 'ws'
      configForm.wsPath = '/trojan-ws'
      configForm.wsHost = 'trojan.example.com'
      configForm.httpPath = ''
      configForm.httpHost = ''
      configForm.grpcServiceName = ''
      break
      
    case 'http':
      configForm.transport = 'http'
      configForm.wsPath = ''
      configForm.wsHost = ''
      configForm.httpPath = '/trojan-h2'
      configForm.httpHost = 'trojan.example.com'
      configForm.grpcServiceName = ''
      break
      
    case 'grpc':
      configForm.transport = 'grpc'
      configForm.wsPath = ''
      configForm.wsHost = ''
      configForm.httpPath = ''
      configForm.httpHost = ''
      configForm.grpcServiceName = 'trojan-grpc-service'
      break
      
    case 'xtls':
      configForm.transport = 'tcp'
      configForm.security = 'xtls'
      configForm.wsPath = ''
      configForm.wsHost = ''
      configForm.httpPath = ''
      configForm.httpHost = ''
      configForm.grpcServiceName = ''
      break
      
    default:
      break
  }
  
  ElMessage.success(`已加载 ${type.toUpperCase()} 预设`)
}

// 加载真实配置样例
const loadRealConfig = () => {
  configForm.name = 'Trojan1'
  configForm.server = 'trojan1.sh-crystalcg.com'
  configForm.port = 60506
  configForm.password = '123456'
  configForm.security = 'tls'
  configForm.sni = 'trojan1.sh-crystalcg.com'
  configForm.transport = 'tcp'
  configForm.wsPath = ''
  configForm.wsHost = ''
  configForm.httpPath = ''
  configForm.httpHost = ''
  configForm.grpcServiceName = ''
  configForm.allowInsecure = false
  configForm.remark = 'Trojan1'
  
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