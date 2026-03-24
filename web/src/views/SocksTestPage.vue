<template>
  <div class="socks-test-page">
    <h1 class="page-title">
      SOCKS 协议测试
    </h1>
    
    <el-row :gutter="20">
      <el-col
        :xs="24"
        :sm="24"
        :md="12"
        :lg="12"
        :xl="12"
      >
        <el-card
          class="config-card"
          shadow="hover"
        >
          <template #header>
            <div class="card-header">
              <span>SOCKS 配置</span>
            </div>
          </template>
          
          <el-form
            :model="configForm"
            label-position="top"
          >
            <el-form-item label="连接名称">
              <el-input
                v-model="configForm.connectionName"
                placeholder="输入连接名称"
              />
            </el-form-item>
            
            <el-form-item
              label="服务器地址"
              required
            >
              <el-input
                v-model="configForm.server"
                placeholder="服务器地址"
              />
            </el-form-item>
            
            <el-form-item
              label="端口"
              required
            >
              <el-input-number
                v-model="configForm.port"
                :min="1"
                :max="65535"
                class="full-width"
              />
            </el-form-item>
            
            <el-form-item label="SOCKS 版本">
              <el-select
                v-model="configForm.version"
                class="full-width"
              >
                <el-option
                  label="SOCKS 4"
                  value="4"
                />
                <el-option
                  label="SOCKS 4a"
                  value="4a"
                />
                <el-option
                  label="SOCKS 5"
                  value="5"
                />
              </el-select>
            </el-form-item>
            
            <el-form-item label="使用认证">
              <el-switch v-model="configForm.useAuth" />
            </el-form-item>
            
            <template v-if="configForm.useAuth">
              <el-form-item label="用户名">
                <el-input
                  v-model="configForm.username"
                  placeholder="用户名"
                />
              </el-form-item>
              
              <el-form-item label="密码">
                <el-input 
                  v-model="configForm.password" 
                  placeholder="密码" 
                  :type="showPassword ? 'text' : 'password'"
                >
                  <template #append>
                    <el-button @click="showPassword = !showPassword">
                      <el-icon v-if="showPassword">
                        <View />
                      </el-icon>
                      <el-icon v-else>
                        <Hide />
                      </el-icon>
                    </el-button>
                  </template>
                </el-input>
              </el-form-item>
            </template>
            
            <el-form-item label="使用 TLS">
              <el-switch v-model="configForm.useTLS" />
              <div
                v-if="configForm.useTLS"
                class="tip-text"
              >
                注: SOCKS 标准不包括 TLS，这是扩展功能
              </div>
            </el-form-item>
            
            <template v-if="configForm.useTLS">
              <el-form-item label="SNI (服务器名称指示)">
                <el-input
                  v-model="configForm.sni"
                  placeholder="域名"
                />
              </el-form-item>
              
              <el-form-item label="跳过证书验证">
                <el-switch v-model="configForm.allowInsecure" />
              </el-form-item>
            </template>
            
            <el-form-item label="UDP 支持">
              <el-switch
                v-model="configForm.udpSupport"
                :disabled="configForm.version !== '5'"
              />
              <div
                v-if="configForm.version !== '5' && configForm.udpSupport"
                class="tip-text"
              >
                注: 仅 SOCKS5 支持 UDP
              </div>
            </el-form-item>
            
            <el-form-item label="备注">
              <el-input
                v-model="configForm.remark"
                placeholder="可选备注"
              />
            </el-form-item>
            
            <div class="form-actions">
              <el-button
                type="primary"
                @click="applyConfig"
              >
                应用配置
              </el-button>
              <el-button @click="resetForm">
                重置
              </el-button>
            </div>
          </el-form>
        </el-card>
      </el-col>
      
      <el-col
        :xs="24"
        :sm="24"
        :md="12"
        :lg="12"
        :xl="12"
      >
        <el-card
          class="qrcode-card"
          shadow="hover"
        >
          <template #header>
            <div class="card-header">
              <span>SOCKS 二维码</span>
            </div>
          </template>
          
          <SocksQRCode
            :connection-name="activeConfig.connectionName"
            :server="activeConfig.server"
            :port="activeConfig.port"
            :version="activeConfig.version"
            :username="activeConfig.useAuth ? activeConfig.username : ''"
            :password="activeConfig.useAuth ? activeConfig.password : ''"
            :use-tls="activeConfig.useTLS"
            :sni="activeConfig.useTLS ? activeConfig.sni : ''"
            :allow-insecure="activeConfig.useTLS ? activeConfig.allowInsecure : false"
            :udp-support="activeConfig.udpSupport"
            :remark="activeConfig.remark"
          />
        </el-card>
        
        <el-card
          class="presets-card"
          shadow="hover"
        >
          <template #header>
            <div class="card-header">
              <span>预设配置</span>
            </div>
          </template>
          
          <div class="preset-buttons">
            <el-button 
              type="primary" 
              plain 
              @click="loadPreset('socks4')"
            >
              SOCKS4
            </el-button>
            
            <el-button 
              type="success" 
              plain 
              @click="loadPreset('socks5')"
            >
              SOCKS5
            </el-button>
            
            <el-button 
              type="warning" 
              plain 
              @click="loadPreset('socks5auth')"
            >
              SOCKS5 认证
            </el-button>
            
            <el-button 
              type="danger" 
              plain 
              @click="loadPreset('socks5udp')"
            >
              SOCKS5 UDP
            </el-button>
            
            <el-button 
              type="info" 
              plain 
              @click="loadRealConfig"
            >
              实际配置
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { View, Hide } from '@element-plus/icons-vue'
import SocksQRCode from '../components/SocksQRCode.vue'

// 显示密码开关
const showPassword = ref(false)

// 生成随机密码
const generateRandomPassword = () => {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*'
  let password = ''
  
  for (let i = 0; i < 16; i++) {
    const randomIndex = Math.floor(Math.random() * chars.length)
    password += chars[randomIndex]
  }
  
  return password
}

// 表单数据
const configForm = reactive({
  connectionName: 'SOCKS Proxy',
  server: '',
  port: 1080,
  version: '5',
  useAuth: false,
  username: '',
  password: '',
  useTLS: false,
  sni: '',
  allowInsecure: false,
  udpSupport: false,
  remark: ''
})

// 应用到二维码的活动配置
const activeConfig = reactive({
  connectionName: 'SOCKS Proxy',
  server: '',
  port: 1080,
  version: '5',
  useAuth: false,
  username: '',
  password: '',
  useTLS: false,
  sni: '',
  allowInsecure: false,
  udpSupport: false,
  remark: ''
})

// 监听版本变化，如果不是 SOCKS5，则禁用 UDP 支持
watch(() => configForm.version, (newVersion) => {
  if (newVersion !== '5' && configForm.udpSupport) {
    configForm.udpSupport = false
    ElMessage.info('仅 SOCKS5 支持 UDP，已自动禁用 UDP 支持')
  }
})

// 应用配置
const applyConfig = () => {
  // 验证必填字段
  if (!configForm.server) {
    ElMessage.error('请输入服务器地址')
    return
  }
  
  if (!configForm.port || configForm.port < 1 || configForm.port > 65535) {
    ElMessage.error('请输入有效的端口号(1-65535)')
    return
  }
  
  if (configForm.useAuth && (!configForm.username || !configForm.password)) {
    ElMessage.error('启用认证时，用户名和密码不能为空')
    return
  }
  
  if (configForm.version !== '5' && configForm.udpSupport) {
    ElMessage.warning('仅 SOCKS5 支持 UDP，已自动禁用 UDP 支持')
    configForm.udpSupport = false
  }
  
  if (configForm.useTLS && configForm.sni === '') {
    // 如果启用了 TLS 但没有填 SNI，默认使用服务器地址
    configForm.sni = configForm.server
  }
  
  // 更新活动配置
  Object.assign(activeConfig, configForm)
  
  ElMessage.success('配置已应用')
}

// 重置表单
const resetForm = () => {
  configForm.connectionName = 'SOCKS Proxy'
  configForm.server = ''
  configForm.port = 1080
  configForm.version = '5'
  configForm.useAuth = false
  configForm.username = ''
  configForm.password = ''
  configForm.useTLS = false
  configForm.sni = ''
  configForm.allowInsecure = false
  configForm.udpSupport = false
  configForm.remark = ''
  
  ElMessage.info('表单已重置')
}

// 加载预设配置
const loadPreset = (type) => {
  switch (type) {
    case 'socks4':
      configForm.connectionName = 'SOCKS4 代理'
      configForm.server = 'socks4.example.com'
      configForm.port = 1080
      configForm.version = '4'
      configForm.useAuth = false // SOCKS4 不支持用户名/密码认证
      configForm.username = ''
      configForm.password = ''
      configForm.useTLS = false
      configForm.sni = ''
      configForm.allowInsecure = false
      configForm.udpSupport = false
      configForm.remark = '基本 SOCKS4 代理'
      break
      
    case 'socks5':
      configForm.connectionName = 'SOCKS5 代理'
      configForm.server = 'socks5.example.com'
      configForm.port = 1080
      configForm.version = '5'
      configForm.useAuth = false
      configForm.username = ''
      configForm.password = ''
      configForm.useTLS = false
      configForm.sni = ''
      configForm.allowInsecure = false
      configForm.udpSupport = false
      configForm.remark = '基本 SOCKS5 代理'
      break
      
    case 'socks5auth':
      configForm.connectionName = 'SOCKS5 认证代理'
      configForm.server = 'secure-socks.example.com'
      configForm.port = 1080
      configForm.version = '5'
      configForm.useAuth = true
      configForm.username = 'user'
      configForm.password = generateRandomPassword()
      configForm.useTLS = false
      configForm.sni = ''
      configForm.allowInsecure = false
      configForm.udpSupport = false
      configForm.remark = '带认证的 SOCKS5 代理'
      break
      
    case 'socks5udp':
      configForm.connectionName = 'SOCKS5 UDP 代理'
      configForm.server = 'game-proxy.example.com'
      configForm.port = 1080
      configForm.version = '5'
      configForm.useAuth = true
      configForm.username = 'gamer'
      configForm.password = generateRandomPassword()
      configForm.useTLS = false
      configForm.sni = ''
      configForm.allowInsecure = false
      configForm.udpSupport = true
      configForm.remark = '支持 UDP 的游戏代理'
      break
  }
  
  ElMessage.success(`已加载${type}预设配置`)
  applyConfig()
}

// 加载实际配置示例
const loadRealConfig = () => {
  configForm.connectionName = '企业 SOCKS5 代理'
  configForm.server = 'socks.company.com'
  configForm.port = 1080
  configForm.version = '5'
  configForm.useAuth = true
  configForm.username = 'employee'
  configForm.password = 'Secure@Password123'
  configForm.useTLS = true
  configForm.sni = 'socks.company.com'
  configForm.allowInsecure = false
  configForm.udpSupport = true
  configForm.remark = '公司内部 SOCKS5 代理'
  
  ElMessage.success('已加载实际配置示例')
  applyConfig()
}
</script>

<style scoped>
.socks-test-page {
  padding: 20px;
}

.page-title {
  text-align: center;
  margin-bottom: 24px;
  color: var(--color-text-primary);
}

.config-card,
.qrcode-card,
.presets-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
}

.full-width {
  width: 100%;
}

.preset-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.preset-buttons .el-button {
  flex: 1;
  min-width: 120px;
  margin: 0;
}

.tip-text {
  font-size: 12px;
  color: #E6A23C;
  margin-top: 4px;
}

@media (max-width: 768px) {
  .form-actions,
  .preset-buttons {
    flex-direction: column;
    gap: 8px;
  }
  
  .form-actions .el-button,
  .preset-buttons .el-button {
    width: 100%;
  }
}
</style> 