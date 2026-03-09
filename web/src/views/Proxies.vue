<template>
  <div class="proxies-container">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>协议管理</span>
          <el-button type="primary" @click="showAddDialog">添加协议</el-button>
        </div>
      </template>
      
      <el-table 
        :data="proxiesList" 
        border 
        style="width: 100%" 
        v-loading="loading"
      >
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="type" label="协议类型" width="100">
          <template #default="{ row }">
            <el-tag :type="getProtocolType(row.type)">
              {{ row.type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="port" label="端口" width="80" />
        <el-table-column prop="users" label="用户数" width="80" />
        <el-table-column prop="created" label="创建时间" width="160" />
        <el-table-column prop="status" label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status ? 'success' : 'danger'">
              {{ row.status ? '运行中' : '已停止' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button size="small" type="primary" @click="handleEdit(row)">编辑</el-button>
              <el-button size="small" type="success" @click="handleView(row)">查看</el-button>
              <el-button 
                size="small" 
                :type="row.status ? 'warning' : 'success'"
                @click="handleToggleStatus(row)"
              >
                {{ row.status ? '停止' : '启动' }}
              </el-button>
              <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
    
    <!-- 添加/编辑协议对话框 -->
    <el-dialog 
      :title="dialogType === 'add' ? '添加协议' : '编辑协议'" 
      v-model="dialogVisible"
      width="65%"
    >
      <el-form 
        ref="proxyFormRef"
        :model="proxyForm"
        :rules="formRules"
        label-width="120px"
        label-position="left"
      >
        <el-tabs v-model="activeTab">
          <el-tab-pane label="基本设置" name="basic">
            <el-form-item label="协议名称" prop="name">
              <el-input v-model="proxyForm.name" placeholder="请输入协议名称" />
            </el-form-item>
            <el-form-item label="协议类型" prop="type">
              <el-select v-model="proxyForm.type" placeholder="请选择协议类型" style="width: 100%">
                <el-option label="VMess" value="vmess" />
                <el-option label="VLESS" value="vless" />
                <el-option label="Trojan" value="trojan" />
                <el-option label="Shadowsocks" value="shadowsocks" />
                <el-option label="Socks" value="socks" />
                <el-option label="HTTP" value="http" />
              </el-select>
            </el-form-item>
            <el-form-item label="部署节点" prop="node_id">
              <el-select v-model="proxyForm.node_id" placeholder="请选择部署节点" style="width: 100%" clearable>
                <el-option 
                  v-for="node in nodesList" 
                  :key="node.id" 
                  :label="`${node.name} (${node.address})`" 
                  :value="node.id"
                />
              </el-select>
              <div class="form-tip">选择代理将部署到哪个节点服务器</div>
            </el-form-item>
            <el-form-item label="监听IP" prop="listen">
              <el-input v-model="proxyForm.listen" placeholder="默认为 0.0.0.0" />
            </el-form-item>
            <el-form-item label="端口" prop="port">
              <el-input-number v-model="proxyForm.port" :min="1" :max="65535" style="width: 100%" />
            </el-form-item>
            <el-form-item label="启用" prop="enabled">
              <el-switch v-model="proxyForm.enabled" />
            </el-form-item>
          </el-tab-pane>
          <el-tab-pane label="传输设置" name="transport">
            <el-form-item label="传输协议" prop="streamSettings.network">
              <el-select 
                v-model="proxyForm.streamSettings.network" 
                placeholder="请选择传输协议"
                style="width: 100%"
              >
                <el-option label="TCP" value="tcp" />
                <el-option label="KCP" value="kcp" />
                <el-option label="WebSocket" value="ws" />
                <el-option label="HTTP/2" value="http" />
                <el-option label="QUIC" value="quic" />
                <el-option label="gRPC" value="grpc" />
              </el-select>
            </el-form-item>
            
            <!-- TCP 设置 -->
            <template v-if="proxyForm.streamSettings.network === 'tcp'">
              <el-form-item label="伪装类型" prop="streamSettings.tcpSettings.type">
                <el-select v-model="proxyForm.streamSettings.tcpSettings.type" style="width: 100%">
                  <el-option label="none" value="none" />
                  <el-option label="http" value="http" />
                </el-select>
              </el-form-item>
              <el-form-item 
                v-if="proxyForm.streamSettings.tcpSettings.type === 'http'" 
                label="请求头"
              >
                <el-input 
                  type="textarea" 
                  v-model="proxyForm.streamSettings.tcpSettings.request" 
                  rows="4" 
                  placeholder="HTTP 请求头，JSON 格式" 
                />
              </el-form-item>
            </template>
            
            <!-- WebSocket 设置 -->
            <template v-if="proxyForm.streamSettings.network === 'ws'">
              <el-form-item label="路径" prop="streamSettings.wsSettings.path">
                <el-input v-model="proxyForm.streamSettings.wsSettings.path" placeholder="/" />
              </el-form-item>
              <el-form-item label="请求头">
                <el-input 
                  type="textarea" 
                  v-model="proxyForm.streamSettings.wsSettings.headers" 
                  rows="4" 
                  placeholder="HTTP 请求头，JSON 格式"
                />
              </el-form-item>
            </template>
            
            <!-- HTTP/2 设置 -->
            <template v-if="proxyForm.streamSettings.network === 'http'">
              <el-form-item label="域名" prop="streamSettings.httpSettings.host">
                <el-input v-model="proxyForm.streamSettings.httpSettings.host" placeholder="请输入域名" />
              </el-form-item>
              <el-form-item label="路径" prop="streamSettings.httpSettings.path">
                <el-input v-model="proxyForm.streamSettings.httpSettings.path" placeholder="/" />
              </el-form-item>
            </template>
            
            <!-- QUIC 设置 -->
            <template v-if="proxyForm.streamSettings.network === 'quic'">
              <el-form-item label="加密方式" prop="streamSettings.quicSettings.security">
                <el-select v-model="proxyForm.streamSettings.quicSettings.security" style="width: 100%">
                  <el-option label="none" value="none" />
                  <el-option label="aes-128-gcm" value="aes-128-gcm" />
                  <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
                </el-select>
              </el-form-item>
              <el-form-item label="密钥" prop="streamSettings.quicSettings.key">
                <el-input v-model="proxyForm.streamSettings.quicSettings.key" placeholder="请输入密钥" />
              </el-form-item>
            </template>
            
            <!-- gRPC 设置 -->
            <template v-if="proxyForm.streamSettings.network === 'grpc'">
              <el-form-item label="服务名称" prop="streamSettings.grpcSettings.serviceName">
                <el-input v-model="proxyForm.streamSettings.grpcSettings.serviceName" placeholder="请输入服务名称" />
              </el-form-item>
              <el-form-item label="多路复用" prop="streamSettings.grpcSettings.multiMode">
                <el-switch v-model="proxyForm.streamSettings.grpcSettings.multiMode" />
              </el-form-item>
            </template>
          </el-tab-pane>
          <el-tab-pane label="TLS 设置" name="tls">
            <el-form-item label="启用 TLS" prop="streamSettings.security">
              <el-select v-model="proxyForm.streamSettings.security" style="width: 100%">
                <el-option label="不启用" value="none" />
                <el-option label="TLS" value="tls" />
              </el-select>
            </el-form-item>
            
            <template v-if="proxyForm.streamSettings.security === 'tls'">
              <el-form-item label="域名" prop="streamSettings.tlsSettings.serverName">
                <el-input
                  v-model="proxyForm.streamSettings.tlsSettings.serverName"
                  placeholder="请输入 TLS 域名，例如 vpn.example.com"
                />
                <div class="form-tip">自动模式会按这里填写的域名，从“证书管理”里匹配已签发且可用的证书。</div>
              </el-form-item>
              <el-form-item label="证书配置" prop="streamSettings.tlsSettings.certificates">
                <el-radio-group v-model="certType">
                  <el-radio value="auto">自动匹配系统证书</el-radio>
                  <el-radio value="file">证书文件路径</el-radio>
                  <el-radio value="content">证书文件内容</el-radio>
                </el-radio-group>

                <div v-if="certType === 'auto'" class="cert-input">
                  <div class="form-tip">保存后节点下发配置时会优先按域名自动匹配证书；如果同域名证书不存在，可再切到手动模式填写。</div>
                </div>

                <div v-if="certType === 'file'" class="cert-input">
                  <el-form-item label="公钥文件路径">
                    <el-input v-model="proxyForm.streamSettings.tlsSettings.certificates.certFile" placeholder="请输入公钥文件路径" />
                  </el-form-item>
                  <el-form-item label="私钥文件路径">
                    <el-input v-model="proxyForm.streamSettings.tlsSettings.certificates.keyFile" placeholder="请输入私钥文件路径" />
                  </el-form-item>
                </div>

                <div v-if="certType === 'content'" class="cert-input">
                  <el-form-item label="证书内容">
                    <el-input
                      type="textarea"
                      v-model="proxyForm.streamSettings.tlsSettings.certificates.certificate"
                      rows="4"
                      placeholder="证书内容"
                    />
                  </el-form-item>
                  <el-form-item label="私钥内容">
                    <el-input
                      type="textarea"
                      v-model="proxyForm.streamSettings.tlsSettings.certificates.key"
                      rows="4"
                      placeholder="私钥内容"
                    />
                  </el-form-item>
                </div>
              </el-form-item>
            </template>
          </el-tab-pane>
          <el-tab-pane label="高级设置" name="advanced" v-if="proxyForm.type !== 'http' && proxyForm.type !== 'socks'">
            <el-form-item v-if="proxyForm.type === 'vmess'" label="加密方式" prop="settings.vmess.security">
              <el-select v-model="proxyForm.settings.vmess.security" style="width: 100%">
                <el-option label="auto" value="auto" />
                <el-option label="aes-128-gcm" value="aes-128-gcm" />
                <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
                <el-option label="none" value="none" />
              </el-select>
            </el-form-item>
            
            <el-form-item v-if="proxyForm.type === 'shadowsocks'" label="加密方式" prop="settings.shadowsocks.method">
              <el-select v-model="proxyForm.settings.shadowsocks.method" style="width: 100%">
                <el-option label="aes-256-gcm" value="aes-256-gcm" />
                <el-option label="aes-128-gcm" value="aes-128-gcm" />
                <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
                <el-option label="2022-blake3-aes-128-gcm" value="2022-blake3-aes-128-gcm" />
                <el-option label="2022-blake3-aes-256-gcm" value="2022-blake3-aes-256-gcm" />
              </el-select>
            </el-form-item>
            
            <el-form-item label="禁止 sniffing" prop="sniffing.enabled">
              <el-switch v-model="proxyForm.sniffing.enabled" :active-value="false" :inactive-value="true" />
            </el-form-item>
            
            <el-form-item label="总流量限制(GB)" prop="totalGB">
              <el-input-number v-model="proxyForm.totalGB" :min="0" :precision="1" style="width: 100%" />
            </el-form-item>
            
            <el-form-item label="到期时间" prop="expiryTime">
              <el-date-picker
                v-model="proxyForm.expiryTime"
                type="datetime"
                placeholder="选择日期时间"
                style="width: 100%"
              />
            </el-form-item>
          </el-tab-pane>
        </el-tabs>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSaveProxy">保存</el-button>
        </span>
      </template>
    </el-dialog>
    
    <!-- 查看协议详情对话框 -->
    <el-dialog 
      title="协议详情" 
      v-model="viewDialogVisible"
      width="60%"
    >
      <div v-if="selectedProxy" class="protocol-details">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="协议名称">{{ selectedProxy.name }}</el-descriptions-item>
          <el-descriptions-item label="协议类型">{{ selectedProxy.type }}</el-descriptions-item>
          <el-descriptions-item label="监听地址">{{ selectedProxy.listen || '0.0.0.0' }}:{{ selectedProxy.port }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="selectedProxy.status ? 'success' : 'danger'">
              {{ selectedProxy.status ? '运行中' : '已停止' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ selectedProxy.created }}</el-descriptions-item>
          <el-descriptions-item label="配置详情">
            <pre class="config-json">{{ JSON.stringify(selectedProxy, null, 2) }}</pre>
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { proxies as proxiesApi } from '@/api'

// 协议列表
const proxiesList = ref([])
const nodesList = ref([]) // 节点列表
const loading = ref(false)
const dialogVisible = ref(false)
const viewDialogVisible = ref(false)
const dialogType = ref('add')
const activeTab = ref('basic')
const selectedProxy = ref(null)
const certType = ref('auto')
const proxyFormRef = ref(null)

// 表单校验规则
const formRules = {
  name: [
    { required: true, message: '请输入协议名称', trigger: 'blur' }
  ],
  type: [
    { required: true, message: '请选择协议类型', trigger: 'change' }
  ],
  port: [
    { required: true, message: '请输入端口号', trigger: 'blur' }
  ]
}

// 协议表单
const proxyForm = reactive({
  id: null,
  name: '',
  type: '',
  node_id: null, // 节点 ID
  port: 10000,
  listen: '0.0.0.0',
  enabled: true,
  streamSettings: {
    network: 'tcp',
    security: 'none',
    tcpSettings: {
      type: 'none',
      request: ''
    },
    wsSettings: {
      path: '/',
      headers: ''
    },
    httpSettings: {
      host: '',
      path: '/'
    },
    quicSettings: {
      security: 'none',
      key: ''
    },
    grpcSettings: {
      serviceName: '',
      multiMode: false
    },
    tlsSettings: {
      serverName: '',
      certificates: {
        certFile: '',
        keyFile: '',
        certificate: '',
        key: ''
      }
    }
  },
  settings: {
    vmess: {
      security: 'auto'
    },
    shadowsocks: {
      method: 'aes-256-gcm'
    }
  },
  sniffing: {
    enabled: true,
    destOverride: ['http', 'tls']
  },
  totalGB: 0,
  expiryTime: ''
})

// 生命周期钩子
onMounted(() => {
  fetchProxies()
  fetchNodes()
})

// 获取节点列表
const fetchNodes = async () => {
  try {
    // 调用节点列表 API
    const response = await fetch('/api/admin/nodes', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      }
    })
    if (response.ok) {
      const data = await response.json()
      nodesList.value = data.data || []
    }
  } catch (error) {
    console.error('获取节点列表失败:', error)
  }
}

// 获取协议列表
const fetchProxies = async () => {
  loading.value = true
  try {
    const res = await proxiesApi.list()
    proxiesList.value = res.data || []
  } catch (error) {
    ElMessage.error('获取协议列表失败')
    proxiesList.value = []
  } finally {
    loading.value = false
  }
}

// 显示添加对话框
const showAddDialog = () => {
  dialogType.value = 'add'
  resetForm()
  dialogVisible.value = true
  activeTab.value = 'basic'
}

// 重置表单
const resetForm = () => {
  Object.assign(proxyForm, {
    id: null,
    name: '',
    type: '',
    node_id: null, // 重置节点选择
    port: 10000,
    listen: '0.0.0.0',
    enabled: true,
    streamSettings: {
      network: 'tcp',
      security: 'none',
      tcpSettings: {
        type: 'none',
        request: ''
      },
      wsSettings: {
        path: '/',
        headers: ''
      },
      httpSettings: {
        host: '',
        path: '/'
      },
      quicSettings: {
        security: 'none',
        key: ''
      },
      grpcSettings: {
        serviceName: '',
        multiMode: false
      },
      tlsSettings: {
        serverName: '',
        certificates: {
          certFile: '',
          keyFile: '',
          certificate: '',
          key: ''
        }
      }
    },
    settings: {
      vmess: {
        security: 'auto'
      },
      shadowsocks: {
        method: 'aes-256-gcm'
      }
    },
    sniffing: {
      enabled: true,
      destOverride: ['http', 'tls']
    },
    totalGB: 0,
    expiryTime: ''
  })
  certType.value = 'auto'
}

// 处理编辑
const handleEdit = (row) => {
  dialogType.value = 'edit'
  resetForm()
  // 深拷贝以避免直接修改列表数据
  Object.assign(proxyForm, JSON.parse(JSON.stringify(row)))
  dialogVisible.value = true
  activeTab.value = 'basic'
}

// 处理查看详情
const handleView = (row) => {
  selectedProxy.value = { ...row }
  viewDialogVisible.value = true
}

// 处理启动/停止
const handleToggleStatus = async (row) => {
  try {
    // 实际项目中应调用API
    // await proxiesApi.toggleStatus(row.id)
    
    const action = row.status ? '停止' : '启动'
    ElMessage.success(`已${action}协议：${row.name}`)
    row.status = !row.status
  } catch (error) {
    console.error('Failed to toggle proxy status:', error)
    ElMessage.error(`操作失败`)
  }
}

// 处理删除
const handleDelete = (row) => {
  ElMessageBox.confirm(`确定要删除协议 ${row.name} 吗?`, '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      // 实际项目中应调用API
      // await proxiesApi.delete(row.id)
      
      proxiesList.value = proxiesList.value.filter(p => p.id !== row.id)
      ElMessage.success('删除成功')
    } catch (error) {
      console.error('Failed to delete proxy:', error)
      ElMessage.error('删除失败')
    }
  }).catch(() => {
    ElMessage.info('已取消删除')
  })
}

// 保存协议
const handleSaveProxy = async () => {
  if (!proxyFormRef.value) return
  
  try {
    await proxyFormRef.value.validate()
    
    if (dialogType.value === 'add') {
      // 添加新协议
      // 实际项目中应调用API
      // await proxiesApi.create(proxyForm)
      
      const newProxy = {
        ...JSON.parse(JSON.stringify(proxyForm)),
        id: Date.now(),
        users: 0,
        created: new Date().toLocaleString(),
        status: proxyForm.enabled
      }
      proxiesList.value.push(newProxy)
      ElMessage.success('添加成功')
    } else {
      // 更新现有协议
      // 实际项目中应调用API
      // await proxiesApi.update(proxyForm.id, proxyForm)
      
      const index = proxiesList.value.findIndex(p => p.id === proxyForm.id)
      if (index !== -1) {
        const updatedProxy = {
          ...JSON.parse(JSON.stringify(proxyForm)),
          status: proxyForm.enabled
        }
        proxiesList.value[index] = { ...proxiesList.value[index], ...updatedProxy }
        ElMessage.success('更新成功')
      }
    }
    dialogVisible.value = false
  } catch (error) {
    console.error('Form validation failed:', error)
    ElMessage.error('表单验证失败，请检查输入')
  }
}

// 获取协议类型对应的样式
const getProtocolType = (type) => {
  const types = {
    'vmess': 'primary',
    'vless': 'success',
    'trojan': 'warning',
    'shadowsocks': 'danger',
    'socks': 'info',
    'http': 'default'
  }
  return types[type] || 'default'
}
</script>

<style scoped>
.proxies-container {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.cert-input {
  margin-top: 10px;
  padding: 10px;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  background-color: #f9f9f9;
}

.config-json {
  background-color: #f5f7fa;
  padding: 12px;
  border-radius: 4px;
  font-family: monospace;
  white-space: pre-wrap;
  max-height: 300px;
  overflow-y: auto;
}

.protocol-details {
  max-height: 70vh;
  overflow-y: auto;
}
</style> 