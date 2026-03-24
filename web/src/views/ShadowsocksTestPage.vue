<template>
  <div class="test-page-container">
    <div class="page-header">
      <div class="title">
        Shadowsocks 协议测试
      </div>
    </div>
    
    <el-card
      shadow="hover"
      class="test-card"
    >
      <template #header>
        <div class="card-header">
          <span>测试 Shadowsocks 配置</span>
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
              <el-form-item label="加密方式">
                <el-select
                  v-model="configForm.method"
                  style="width: 100%"
                >
                  <el-option
                    label="aes-256-gcm"
                    value="aes-256-gcm"
                  />
                  <el-option
                    label="aes-128-gcm"
                    value="aes-128-gcm"
                  />
                  <el-option
                    label="chacha20-ietf-poly1305"
                    value="chacha20-ietf-poly1305"
                  />
                  <el-option
                    label="2022-blake3-aes-256-gcm"
                    value="2022-blake3-aes-256-gcm"
                  />
                  <el-option
                    label="2022-blake3-aes-128-gcm"
                    value="2022-blake3-aes-128-gcm"
                  />
                  <el-option
                    label="2022-blake3-chacha20-poly1305"
                    value="2022-blake3-chacha20-poly1305"
                  />
                  <el-option
                    label="none"
                    value="none"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            
            <el-col
              :xs="24"
              :md="12"
            >
              <el-form-item label="使用插件">
                <el-switch v-model="configForm.usePlugin" />
              </el-form-item>
            </el-col>
            
            <template v-if="configForm.usePlugin">
              <el-col
                :xs="24"
                :md="12"
              >
                <el-form-item label="插件名称">
                  <el-select
                    v-model="configForm.plugin"
                    style="width: 100%"
                  >
                    <el-option
                      label="v2ray-plugin"
                      value="v2ray-plugin"
                    />
                    <el-option
                      label="obfs-local"
                      value="obfs-local"
                    />
                    <el-option
                      label="simple-obfs"
                      value="simple-obfs"
                    />
                  </el-select>
                </el-form-item>
              </el-col>
              
              <el-col
                :xs="24"
                :md="12"
              >
                <el-form-item label="插件选项">
                  <el-input
                    v-model="configForm.pluginOpts"
                    placeholder="插件选项，如 obfs=tls;obfs-host=www.example.com"
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
          <span>Shadowsocks 二维码</span>
        </div>
      </template>
      
      <div class="qrcode-container">
        <ShadowsocksQRCode 
          :connection-name="configForm.name"
          :server="configForm.server"
          :port="configForm.port"
          :password="configForm.password"
          :method="configForm.method"
          :plugin="configForm.usePlugin ? configForm.plugin : ''"
          :plugin-opts="configForm.usePlugin ? configForm.pluginOpts : ''"
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
          @click="loadPreset('basic')"
        >
          基础预设
        </el-button>
        <el-button
          type="success"
          @click="loadPreset('2022')"
        >
          2022 预设
        </el-button>
        <el-button
          type="warning"
          @click="loadPreset('obfs')"
        >
          OBFS 预设
        </el-button>
        <el-button
          type="info"
          @click="loadPreset('v2ray')"
        >
          V2Ray 插件预设
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
import ShadowsocksQRCode from '../components/ShadowsocksQRCode.vue'

// 配置表单
const configForm = reactive({
  name: 'Shadowsocks 测试连接',
  server: 'ss.example.com',
  port: 8388,
  password: 'password123',
  method: 'aes-256-gcm',
  usePlugin: false,
  plugin: 'obfs-local',
  pluginOpts: 'obfs=tls;obfs-host=www.example.com',
  remark: ''
})

// 生成随机密码
const generateRandomPassword = () => {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+'
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
  configForm.name = 'Shadowsocks 测试连接'
  configForm.server = 'ss.example.com'
  configForm.port = 8388
  configForm.password = 'password123'
  configForm.method = 'aes-256-gcm'
  configForm.usePlugin = false
  configForm.plugin = 'obfs-local'
  configForm.pluginOpts = 'obfs=tls;obfs-host=www.example.com'
  configForm.remark = ''
  
  ElMessage.info('配置已重置')
}

// 加载预设配置
const loadPreset = (type) => {
  // 基础配置
  configForm.name = `Shadowsocks ${type.toUpperCase()} 测试`
  configForm.server = 'ss.example.com'
  configForm.port = 8388
  configForm.password = 'password123'
  configForm.remark = `Shadowsocks ${type.toUpperCase()} 配置`
  
  // 根据预设类型设置特殊配置
  switch (type) {
    case 'basic':
      configForm.method = 'aes-256-gcm'
      configForm.usePlugin = false
      configForm.plugin = ''
      configForm.pluginOpts = ''
      break
      
    case '2022':
      configForm.method = '2022-blake3-aes-256-gcm'
      configForm.usePlugin = false
      configForm.plugin = ''
      configForm.pluginOpts = ''
      break
      
    case 'obfs':
      configForm.method = 'aes-256-gcm'
      configForm.usePlugin = true
      configForm.plugin = 'obfs-local'
      configForm.pluginOpts = 'obfs=tls;obfs-host=www.example.com'
      break
      
    case 'v2ray':
      configForm.method = 'aes-256-gcm'
      configForm.usePlugin = true
      configForm.plugin = 'v2ray-plugin'
      configForm.pluginOpts = 'mode=websocket;tls=1;host=ss.example.com;path=/v2ray'
      break
      
    default:
      break
  }
  
  ElMessage.success(`已加载 ${type.toUpperCase()} 预设`)
}

// 加载真实配置样例
const loadRealConfig = () => {
  configForm.name = 'SS1'
  configForm.server = 'ss1.example.com'
  configForm.port = 8389
  configForm.password = 'realpassword123'
  configForm.method = 'chacha20-ietf-poly1305'
  configForm.usePlugin = true
  configForm.plugin = 'simple-obfs'
  configForm.pluginOpts = 'obfs=http;obfs-host=www.microsoft.com'
  configForm.remark = 'SS1'
  
  ElMessage.success('已加载真实配置')
}
</script>

<style scoped>
.test-page-container {
  padding: 20px;
  background-color: var(--color-bg-page);
  min-height: 100vh;
}

.page-header {
  margin-bottom: 20px;
}

.title {
  font-size: 22px;
  font-weight: 600;
  color: var(--color-text-primary);
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