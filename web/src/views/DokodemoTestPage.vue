<template>
  <div class="dokodemo-test-page">
    <h1 class="page-title">
      Dokodemo-Door 协议测试
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
              <span>Dokodemo 配置</span>
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
              label="目标地址"
              required
            >
              <el-input
                v-model="configForm.address"
                placeholder="目标服务器地址"
              />
            </el-form-item>
            
            <el-form-item
              label="目标端口"
              required
            >
              <el-input-number
                v-model="configForm.port"
                :min="1"
                :max="65535"
                class="full-width"
              />
            </el-form-item>
            
            <el-form-item label="网络类型">
              <el-select
                v-model="configForm.network"
                class="full-width"
              >
                <el-option
                  label="TCP"
                  value="tcp"
                />
                <el-option
                  label="UDP"
                  value="udp"
                />
                <el-option
                  label="TCP+UDP"
                  value="tcp,udp"
                />
              </el-select>
            </el-form-item>
            
            <el-form-item label="跟随重定向">
              <el-switch v-model="configForm.followRedirect" />
            </el-form-item>
            
            <el-form-item label="超时时间(秒)">
              <el-input-number
                v-model="configForm.timeout"
                :min="1"
                :max="3600"
                class="full-width"
              />
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
              <span>Dokodemo 二维码</span>
            </div>
          </template>
          
          <DokodemoQRCode
            :connection-name="activeConfig.connectionName"
            :address="activeConfig.address"
            :port="activeConfig.port"
            :network="activeConfig.network"
            :follow-redirect="activeConfig.followRedirect"
            :timeout="activeConfig.timeout"
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
              @click="loadPreset('tcp')"
            >
              TCP 预设
            </el-button>
            
            <el-button 
              type="success" 
              plain 
              @click="loadPreset('udp')"
            >
              UDP 预设
            </el-button>
            
            <el-button 
              type="warning" 
              plain 
              @click="loadPreset('mixed')"
            >
              TCP+UDP 预设
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
import { reactive } from 'vue'
import { ElMessage } from 'element-plus'
import DokodemoQRCode from '../components/DokodemoQRCode.vue'

// 表单数据
const configForm = reactive({
  connectionName: 'Dokodemo',
  address: '',
  port: 80,
  network: 'tcp',
  followRedirect: false,
  timeout: 300,
  remark: ''
})

// 应用到二维码的活动配置
const activeConfig = reactive({
  connectionName: 'Dokodemo',
  address: '',
  port: 80,
  network: 'tcp',
  followRedirect: false,
  timeout: 300,
  remark: ''
})

// 应用配置
const applyConfig = () => {
  // 验证必填字段
  if (!configForm.address) {
    ElMessage.error('请输入目标地址')
    return
  }
  
  if (!configForm.port || configForm.port < 1 || configForm.port > 65535) {
    ElMessage.error('请输入有效的端口号(1-65535)')
    return
  }
  
  // 更新活动配置
  Object.assign(activeConfig, configForm)
  
  ElMessage.success('配置已应用')
}

// 重置表单
const resetForm = () => {
  configForm.connectionName = 'Dokodemo'
  configForm.address = ''
  configForm.port = 80
  configForm.network = 'tcp'
  configForm.followRedirect = false
  configForm.timeout = 300
  configForm.remark = ''
  
  ElMessage.info('表单已重置')
}

// 加载预设配置
const loadPreset = (type) => {
  switch (type) {
    case 'tcp':
      configForm.connectionName = 'Dokodemo TCP'
      configForm.address = '192.168.1.100'
      configForm.port = 80
      configForm.network = 'tcp'
      configForm.followRedirect = false
      configForm.timeout = 300
      configForm.remark = 'TCP转发示例'
      break
      
    case 'udp':
      configForm.connectionName = 'Dokodemo UDP'
      configForm.address = '8.8.8.8'
      configForm.port = 53
      configForm.network = 'udp'
      configForm.followRedirect = false
      configForm.timeout = 30
      configForm.remark = 'DNS转发示例'
      break
      
    case 'mixed':
      configForm.connectionName = 'Dokodemo 混合'
      configForm.address = '192.168.1.1'
      configForm.port = 5201
      configForm.network = 'tcp,udp'
      configForm.followRedirect = true
      configForm.timeout = 600
      configForm.remark = 'TCP+UDP混合转发'
      break
  }
  
  ElMessage.success(`已加载${type.toUpperCase()}预设配置`)
  applyConfig()
}

// 加载实际配置示例
const loadRealConfig = () => {
  configForm.connectionName = 'DNS转发'
  configForm.address = '8.8.8.8'
  configForm.port = 53
  configForm.network = 'tcp,udp'
  configForm.followRedirect = true
  configForm.timeout = 60
  configForm.remark = 'Google DNS'
  
  ElMessage.success('已加载实际配置示例')
  applyConfig()
}
</script>

<style scoped>
.dokodemo-test-page {
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
