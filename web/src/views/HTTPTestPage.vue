<template>
  <div class="http-test-page">
    <h1 class="page-title">
      HTTP 协议测试
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
              <span>HTTP 配置</span>
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
              <span>HTTP 二维码</span>
            </div>
          </template>
          
          <HTTPQRCode
            :connection-name="activeConfig.connectionName"
            :server="activeConfig.server"
            :port="activeConfig.port"
            :username="activeConfig.useAuth ? activeConfig.username : ''"
            :password="activeConfig.useAuth ? activeConfig.password : ''"
            :use-tls="activeConfig.useTLS"
            :sni="activeConfig.useTLS ? activeConfig.sni : ''"
            :allow-insecure="activeConfig.useTLS ? activeConfig.allowInsecure : false"
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
              @click="loadPreset('plainhttp')"
            >
              普通 HTTP
            </el-button>
            
            <el-button 
              type="success" 
              plain 
              @click="loadPreset('httpwithauth')"
            >
              HTTP 认证
            </el-button>
            
            <el-button 
              type="warning" 
              plain 
              @click="loadPreset('https')"
            >
              HTTPS
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
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { View, Hide } from '@element-plus/icons-vue'
import HTTPQRCode from '../components/HTTPQRCode.vue'

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
  connectionName: 'HTTP Proxy',
  server: '',
  port: 8080,
  useAuth: false,
  username: '',
  password: '',
  useTLS: false,
  sni: '',
  allowInsecure: false,
  remark: ''
})

// 应用到二维码的活动配置
const activeConfig = reactive({
  connectionName: 'HTTP Proxy',
  server: '',
  port: 8080,
  useAuth: false,
  username: '',
  password: '',
  useTLS: false,
  sni: '',
  allowInsecure: false,
  remark: ''
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
  configForm.connectionName = 'HTTP Proxy'
  configForm.server = ''
  configForm.port = 8080
  configForm.useAuth = false
  configForm.username = ''
  configForm.password = ''
  configForm.useTLS = false
  configForm.sni = ''
  configForm.allowInsecure = false
  configForm.remark = ''
  
  ElMessage.info('表单已重置')
}

// 加载预设配置
const loadPreset = (type) => {
  switch (type) {
    case 'plainhttp':
      configForm.connectionName = '普通 HTTP'
      configForm.server = 'proxy.example.com'
      configForm.port = 8080
      configForm.useAuth = false
      configForm.username = ''
      configForm.password = ''
      configForm.useTLS = false
      configForm.sni = ''
      configForm.allowInsecure = false
      configForm.remark = '基本 HTTP 代理'
      break
      
    case 'httpwithauth':
      configForm.connectionName = 'HTTP 认证'
      configForm.server = 'proxy.example.com'
      configForm.port = 8080
      configForm.useAuth = true
      configForm.username = 'user'
      configForm.password = generateRandomPassword()
      configForm.useTLS = false
      configForm.sni = ''
      configForm.allowInsecure = false
      configForm.remark = '带认证的 HTTP 代理'
      break
      
    case 'https':
      configForm.connectionName = 'HTTPS 代理'
      configForm.server = 'secure-proxy.example.com'
      configForm.port = 443
      configForm.useAuth = true
      configForm.username = 'admin'
      configForm.password = generateRandomPassword()
      configForm.useTLS = true
      configForm.sni = 'secure-proxy.example.com'
      configForm.allowInsecure = false
      configForm.remark = '安全 HTTPS 代理'
      break
  }
  
  ElMessage.success(`已加载${type}预设配置`)
  applyConfig()
}

// 加载实际配置示例
const loadRealConfig = () => {
  configForm.connectionName = '企业 HTTP 代理'
  configForm.server = 'proxy.company.com'
  configForm.port = 3128
  configForm.useAuth = true
  configForm.username = 'employee'
  configForm.password = 'Secure@Password123'
  configForm.useTLS = true
  configForm.sni = 'proxy.company.com'
  configForm.allowInsecure = false
  configForm.remark = '公司内部代理'
  
  ElMessage.success('已加载实际配置示例')
  applyConfig()
}
</script>

<style scoped>
.http-test-page {
  padding: 20px;
}

.page-title {
  text-align: center;
  margin-bottom: 24px;
  color: #303133;
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