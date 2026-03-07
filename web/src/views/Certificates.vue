<template>
  <div class="certificates-container">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>SSL 证书管理</span>
          <div class="header-actions">
            <el-button type="primary" @click="handleApply">申请证书</el-button>
            <el-button type="success" @click="handleUpload">上传证书</el-button>
            <el-button type="info" @click="handleRefresh">刷新</el-button>
          </div>
        </div>
      </template>

      <!-- 证书列表 -->
      <el-table
        :data="certificates"
        border
        style="width: 100%"
        v-loading="loading"
      >
        <el-table-column prop="domain" label="域名" min-width="150" />
        <el-table-column prop="provider" label="提供商" width="120">
          <template #default="scope">
            <el-tag :type="getProviderType(scope.row.provider)">
              {{ scope.row.provider }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="getStatusType(scope.row.status)">
              {{ getStatusText(scope.row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="issueDate" label="创建日期" width="120" />
        <el-table-column prop="expireDate" label="过期日期" width="120">
          <template #default="scope">
            <el-tag :type="getExpireStatusType(scope.row)">
              {{ scope.row.expireDate }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="autoRenew" label="自动续期" width="100">
          <template #default="scope">
            <el-switch
              v-model="scope.row.autoRenew"
              @change="handleAutoRenewChange(scope.row)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="scope">
            <el-button
              type="primary"
              size="small"
              @click="handleRenew(scope.row)"
            >
              续期
            </el-button>
            <el-button
              type="success"
              size="small"
              @click="handleValidate(scope.row)"
            >
              验证
            </el-button>
            <el-button
              type="warning"
              size="small"
              @click="handleBackup(scope.row)"
            >
              备份
            </el-button>
            <el-button
              type="danger"
              size="small"
              @click="handleDelete(scope.row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 申请证书对话框 -->
    <el-dialog
      v-model="applyDialogVisible"
      title="申请证书"
      width="700px"
    >
      <!-- 配置说明 -->
      <el-alert
        title="配置说明"
        type="info"
        :closable="false"
        style="margin-bottom: 20px"
      >
        <template #default>
          <div class="config-guide">
            <p><strong>HTTP 验证：</strong>适用于可直接访问的域名，需要开放 80 端口</p>
            <p><strong>DNS 验证：</strong>适用于申请通配符证书或无法开放 80 端口的情况</p>
            <el-link type="primary" href="/docs/certificate-guide.md" target="_blank" style="margin-top: 10px">
              查看详细配置教程 →
            </el-link>
          </div>
        </template>
      </el-alert>

      <el-form
        ref="applyFormRef"
        :model="applyForm"
        :rules="applyRules"
        label-width="120px"
      >
        <el-form-item label="域名" prop="domain">
          <el-input v-model="applyForm.domain" placeholder="example.com 或 *.example.com">
            <template #append>
              <el-tooltip content="通配符证书只能使用 DNS 验证" placement="top">
                <el-icon><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
          </el-input>
        </el-form-item>

        <el-form-item label="泛域名证书" prop="wildcard">
          <el-switch 
            v-model="applyForm.wildcard"
            active-text="申请泛域名证书（*.domain.com）"
            inactive-text="申请单域名证书"
            @change="handleWildcardChange"
          />
          <div style="font-size: 12px; color: #909399; margin-top: 5px;">
            泛域名证书可以保护主域名及其所有子域名，但只能使用 DNS 验证方式
          </div>
        </el-form-item>

        <el-form-item label="Email" prop="email">
          <el-input v-model="applyForm.email" placeholder="用于接收证书过期通知" />
        </el-form-item>

        <el-form-item label="提供商" prop="provider">
          <el-select v-model="applyForm.provider" placeholder="请选择提供商">
            <el-option label="Let's Encrypt（推荐）" value="letsencrypt" />
            <el-option label="ZeroSSL" value="zerossl" />
          </el-select>
        </el-form-item>

        <el-form-item label="自动续期" prop="autoRenew">
          <el-switch
            v-model="applyForm.autoRenew"
            active-text="开启"
            inactive-text="关闭"
          />
        </el-form-item>

        <el-form-item label="验证方式" prop="validationMethod">
          <el-radio-group v-model="applyForm.validationMethod" @change="handleMethodChange">
            <el-radio value="http">HTTP 验证</el-radio>
            <el-radio value="dns">DNS 验证</el-radio>
          </el-radio-group>
        </el-form-item>

        <!-- HTTP 验证配置 -->
        <template v-if="applyForm.validationMethod === 'http'">
          <el-alert
            title="HTTP 验证要求"
            type="warning"
            :closable="false"
            style="margin-bottom: 15px"
          >
            <ul style="margin: 5px 0; padding-left: 20px">
              <li>域名必须解析到本服务器</li>
              <li>端口 80 必须开放</li>
              <li>Webroot 目录必须存在且可写</li>
            </ul>
          </el-alert>

          <el-form-item label="Webroot 路径" prop="webroot">
            <el-input v-model="applyForm.webroot" placeholder="/app/data/webroot">
              <template #append>
                <el-tooltip content="网站根目录，用于存放验证文件" placement="top">
                  <el-icon><QuestionFilled /></el-icon>
                </el-tooltip>
              </template>
            </el-input>
          </el-form-item>
        </template>

        <!-- DNS 验证配置 -->
        <template v-if="applyForm.validationMethod === 'dns'">
          <el-alert
            title="DNS 验证配置"
            type="info"
            :closable="false"
            style="margin-bottom: 15px"
          >
            <p>需要提供 DNS 提供商的 API 凭证，系统将自动添加 TXT 记录进行验证</p>
          </el-alert>

          <el-form-item label="DNS 提供商" prop="dnsProvider">
            <el-select 
              v-model="applyForm.dnsProvider" 
              placeholder="请选择 DNS 提供商"
              @change="handleDnsProviderChange"
            >
              <el-option label="Cloudflare" value="dns_cf" />
              <el-option label="阿里云" value="dns_ali" />
              <el-option label="腾讯云" value="dns_tencent" />
              <el-option label="DNSPod" value="dns_dp" />
              <el-option label="AWS Route53" value="dns_aws" />
            </el-select>
          </el-form-item>

          <!-- Cloudflare 配置 -->
          <template v-if="applyForm.dnsProvider === 'dns_cf'">
            <el-alert
              title="Cloudflare API Token 配置"
              type="success"
              :closable="false"
              style="margin-bottom: 15px"
            >
              <div style="font-size: 13px; line-height: 1.6">
                <p><strong>获取步骤：</strong></p>
                <ol style="margin: 5px 0; padding-left: 20px">
                  <li>登录 Cloudflare → My Profile → API Tokens</li>
                  <li>Create Token → Edit zone DNS 模板</li>
                  <li>Permissions: Zone → DNS → Edit</li>
                  <li>Zone Resources: 选择你的域名</li>
                  <li>创建并复制 Token 和 Account ID</li>
                </ol>
              </div>
            </el-alert>

            <el-form-item label="API Token" prop="cfToken">
              <el-input 
                v-model="applyForm.cfToken" 
                type="password" 
                show-password
                placeholder="Cloudflare API Token"
              />
            </el-form-item>

            <el-form-item label="Account ID" prop="cfAccountId">
              <el-input 
                v-model="applyForm.cfAccountId" 
                placeholder="在域名 Overview 页面右侧可找到"
              />
            </el-form-item>
          </template>

          <!-- 阿里云配置 -->
          <template v-if="applyForm.dnsProvider === 'dns_ali'">
            <el-alert
              title="阿里云 API 配置"
              type="success"
              :closable="false"
              style="margin-bottom: 15px"
            >
              <div style="font-size: 13px">
                <p>访问控制 → 用户 → 创建用户 → OpenAPI 调用访问</p>
              </div>
            </el-alert>

            <el-form-item label="AccessKey ID" prop="aliKey">
              <el-input v-model="applyForm.aliKey" placeholder="阿里云 AccessKey ID" />
            </el-form-item>

            <el-form-item label="AccessKey Secret" prop="aliSecret">
              <el-input 
                v-model="applyForm.aliSecret" 
                type="password" 
                show-password
                placeholder="阿里云 AccessKey Secret"
              />
            </el-form-item>
          </template>

          <!-- 腾讯云配置 -->
          <template v-if="applyForm.dnsProvider === 'dns_tencent'">
            <el-alert
              title="腾讯云 API 配置"
              type="success"
              :closable="false"
              style="margin-bottom: 15px"
            >
              <div style="font-size: 13px">
                <p>访问管理 → API 密钥管理 → 创建密钥</p>
              </div>
            </el-alert>

            <el-form-item label="SecretId" prop="tencentSecretId">
              <el-input v-model="applyForm.tencentSecretId" placeholder="腾讯云 SecretId" />
            </el-form-item>

            <el-form-item label="SecretKey" prop="tencentSecretKey">
              <el-input 
                v-model="applyForm.tencentSecretKey" 
                type="password" 
                show-password
                placeholder="腾讯云 SecretKey"
              />
            </el-form-item>
          </template>

          <!-- DNSPod 配置 -->
          <template v-if="applyForm.dnsProvider === 'dns_dp'">
            <el-alert
              title="DNSPod API 配置"
              type="success"
              :closable="false"
              style="margin-bottom: 15px"
            >
              <div style="font-size: 13px">
                <p>用户中心 → 安全设置 → API Token</p>
              </div>
            </el-alert>

            <el-form-item label="Token ID" prop="dpId">
              <el-input v-model="applyForm.dpId" placeholder="DNSPod Token ID" />
            </el-form-item>

            <el-form-item label="Token Key" prop="dpKey">
              <el-input 
                v-model="applyForm.dpKey" 
                type="password" 
                show-password
                placeholder="DNSPod Token Key"
              />
            </el-form-item>
          </template>

          <!-- AWS Route53 配置 -->
          <template v-if="applyForm.dnsProvider === 'dns_aws'">
            <el-alert
              title="AWS Route53 配置"
              type="success"
              :closable="false"
              style="margin-bottom: 15px"
            >
              <div style="font-size: 13px">
                <p>IAM → Users → 创建用户 → 附加 AmazonRoute53FullAccess 策略</p>
              </div>
            </el-alert>

            <el-form-item label="Access Key ID" prop="awsAccessKeyId">
              <el-input v-model="applyForm.awsAccessKeyId" placeholder="AWS Access Key ID" />
            </el-form-item>

            <el-form-item label="Secret Access Key" prop="awsSecretAccessKey">
              <el-input 
                v-model="applyForm.awsSecretAccessKey" 
                type="password" 
                show-password
                placeholder="AWS Secret Access Key"
              />
            </el-form-item>
          </template>
        </template>
        <el-form-item
          v-if="applyForm.validationMethod === 'dns'"
          label="DNS 记录"
        >
          <div v-for="(record, index) in applyForm.dnsRecords" :key="index" class="dns-record">
            <el-input v-model="record.name" placeholder="记录名" />
            <el-input v-model="record.type" placeholder="类型" />
            <el-input v-model="record.value" placeholder="值" />
            <el-button type="danger" @click="removeDnsRecord(index)">删除</el-button>
          </div>
          <el-button type="primary" @click="addDnsRecord">添加记录</el-button>
        </el-form-item>
        <el-form-item
          v-if="applyForm.validationMethod === 'http'"
          label="验证路径"
        >
          <el-input v-model="applyForm.validationPath" placeholder="验证路径" />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="applyDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="applying" :disabled="applying" @click="confirmApply">确认申请</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 上传证书对话框 -->
    <el-dialog
      v-model="uploadDialogVisible"
      title="上传证书"
      width="50%"
    >
      <el-form
        ref="uploadFormRef"
        :model="uploadForm"
        :rules="uploadRules"
        label-width="100px"
      >
        <el-form-item label="域名" prop="domain">
          <el-input v-model="uploadForm.domain" placeholder="请输入域名" />
        </el-form-item>
        <el-form-item label="证书文件" prop="certFile">
          <el-upload
            class="upload-demo"
            action="#"
            :auto-upload="false"
            :on-change="handleCertFileChange"
          >
            <el-button type="primary">选择文件</el-button>
            <template #tip>
              <div class="el-upload__tip">
                支持 .pem, .crt 格式的证书文件
              </div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item label="私钥文件" prop="keyFile">
          <el-upload
            class="upload-demo"
            action="#"
            :auto-upload="false"
            :on-change="handleKeyFileChange"
          >
            <el-button type="primary">选择文件</el-button>
            <template #tip>
              <div class="el-upload__tip">
                支持 .key 格式的私钥文件
              </div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="uploadDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="confirmUpload">确认上传</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 验证结果对话框 -->
    <el-dialog
      v-model="validateDialogVisible"
      title="证书验证"
      width="50%"
    >
      <div v-if="validateResult" class="validate-result">
        <div class="result-status">
          <el-tag :type="validateResult.success ? 'success' : 'danger'">
            {{ validateResult.success ? '验证成功' : '验证失败' }}
          </el-tag>
        </div>
        <div class="result-details">
          <div v-if="validateResult.message" class="detail-item">
            <span class="label">消息：</span>
            <span>{{ validateResult.message }}</span>
          </div>
          <div v-if="validateResult.details" class="detail-item">
            <span class="label">详情：</span>
            <pre class="details-content">{{ validateResult.details }}</pre>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { certificatesApi } from '@/api'

// 证书列表
const certificates = ref([])
const loading = ref(false)

// 申请证书
const applyDialogVisible = ref(false)
const applyFormRef = ref(null)
const applying = ref(false)
const applyForm = ref({
  domain: '',
  email: '',
  provider: 'letsencrypt',
  validationMethod: 'http',
  webroot: '/app/data/webroot',
  dnsProvider: '',
  cfToken: '',
  cfAccountId: '',
  aliKey: '',
  aliSecret: '',
  tencentSecretId: '',
  tencentSecretKey: '',
  dpId: '',
  dpKey: '',
  awsAccessKeyId: '',
  awsSecretAccessKey: '',
  dnsRecords: [],
  validationPath: '',
  wildcard: false,
  autoRenew: true
})
const applyRules = {
  domain: [
    { required: true, message: '请输入域名', trigger: 'blur' },
    { pattern: /^(\*\.)?[a-zA-Z0-9][-a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-a-zA-Z0-9]{0,62})+$/, message: '请输入有效的域名', trigger: 'blur' }
  ],
  email: [
    { required: true, message: '请输入 Email', trigger: 'blur' },
    { type: 'email', message: '请输入有效的 Email 地址', trigger: 'blur' }
  ],
  provider: [
    { required: true, message: '请选择提供商', trigger: 'change' }
  ],
  validationMethod: [
    { required: true, message: '请选择验证方式', trigger: 'change' }
  ],
  webroot: [
    { required: true, message: '请输入 Webroot 路径', trigger: 'blur' }
  ],
  dnsProvider: [
    { required: true, message: '请选择 DNS 提供商', trigger: 'change' }
  ]
}

// 上传证书
const uploadDialogVisible = ref(false)
const uploadFormRef = ref(null)
const uploadForm = ref({
  domain: '',
  certFile: null,
  keyFile: null
})
const uploadRules = {
  domain: [
    { required: true, message: '请输入域名', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9][-a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-a-zA-Z0-9]{0,62})+$/, message: '请输入有效的域名', trigger: 'blur' }
  ],
  certFile: [
    { required: true, message: '请上传证书文件', trigger: 'change' }
  ],
  keyFile: [
    { required: true, message: '请上传私钥文件', trigger: 'change' }
  ]
}

// 验证结果
const validateDialogVisible = ref(false)
const validateResult = ref(null)

const formatDate = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toISOString().slice(0, 10)
}

const normalizeCertificatesResponse = (response) => {
  if (Array.isArray(response)) return response
  if (Array.isArray(response?.certificates)) return response.certificates
  if (Array.isArray(response?.data?.certificates)) return response.data.certificates
  if (Array.isArray(response?.data)) return response.data
  return []
}

const mapCertificate = (cert) => {
  const autoRenew = cert.auto_renew ?? cert.autoRenew ?? false
  const status = cert.status || 'pending'
  const expiresAt = cert.expires_at || cert.expiresAt || ''

  return {
    ...cert,
    autoRenew,
    status,
    issueDate: formatDate(cert.created_at || cert.createdAt),
    expireDate: formatDate(expiresAt)
  }
}

// 生命周期钩子
onMounted(async () => {
  await fetchCertificates()
})

// 获取证书列表
const fetchCertificates = async () => {
  loading.value = true
  try {
    const response = await certificatesApi.list()
    const data = normalizeCertificatesResponse(response)
    certificates.value = data.map(mapCertificate)
  } catch (error) {
    console.error('Failed to fetch certificates:', error)
    ElMessage.error('获取证书列表失败')
    certificates.value = []
  } finally {
    loading.value = false
  }
}

// 处理申请证书
const handleApply = () => {
  applyForm.value = {
    domain: '',
    email: '',
    provider: 'letsencrypt',
    validationMethod: 'http',
    webroot: '/app/data/webroot',
    dnsProvider: '',
    cfToken: '',
    cfAccountId: '',
    aliKey: '',
    aliSecret: '',
    tencentSecretId: '',
    tencentSecretKey: '',
    dpId: '',
    dpKey: '',
    awsAccessKeyId: '',
    awsSecretAccessKey: '',
    dnsRecords: [],
    validationPath: '',
    wildcard: false,
    autoRenew: true
  }
  applyDialogVisible.value = true
}

// 处理验证方式变更
const handleMethodChange = (method) => {
  // 如果切换到 DNS 验证，清空 HTTP 相关字段
  if (method === 'dns') {
    applyForm.value.webroot = ''
  } else {
    // 如果切换到 HTTP 验证，设置默认 webroot
    applyForm.value.webroot = '/app/data/webroot'
    applyForm.value.dnsProvider = ''
    // HTTP 验证不支持泛域名
    if (applyForm.value.wildcard) {
      ElMessage.warning('HTTP 验证不支持泛域名证书，已自动切换为单域名模式')
      applyForm.value.wildcard = false
    }
  }
}

// 处理泛域名选项变更
const handleWildcardChange = (value) => {
  if (value) {
    // 开启泛域名，强制使用 DNS 验证
    if (applyForm.value.validationMethod !== 'dns') {
      applyForm.value.validationMethod = 'dns'
      ElMessage.info('泛域名证书只能使用 DNS 验证，已自动切换')
    }
  }
}

// 处理 DNS 提供商变更
const handleDnsProviderChange = (provider) => {
  // 清空所有 DNS API 凭证
  applyForm.value.cfToken = ''
  applyForm.value.cfAccountId = ''
  applyForm.value.aliKey = ''
  applyForm.value.aliSecret = ''
  applyForm.value.tencentSecretId = ''
  applyForm.value.tencentSecretKey = ''
  applyForm.value.dpId = ''
  applyForm.value.dpKey = ''
  applyForm.value.awsAccessKeyId = ''
  applyForm.value.awsSecretAccessKey = ''
}

// 添加DNS记录
const addDnsRecord = () => {
  applyForm.value.dnsRecords.push({
    name: '',
    type: 'TXT',
    value: ''
  })
}

// 移除DNS记录
const removeDnsRecord = (index) => {
  applyForm.value.dnsRecords.splice(index, 1)
}

// 确认申请证书
const confirmApply = async () => {
  if (!applyFormRef.value) return
  
  try {
    applying.value = true
    await applyFormRef.value.validate()
    
    // 构建请求数据
    const requestData = {
      domain: applyForm.value.domain,
      email: applyForm.value.email,
      provider: applyForm.value.provider,
      method: applyForm.value.validationMethod,
      webroot: applyForm.value.webroot,
      dns_provider: applyForm.value.dnsProvider,
      dns_env: {},
      auto_renew: applyForm.value.autoRenew,
      wildcard: applyForm.value.wildcard
    }

    // 根据 DNS 提供商添加相应的凭证
    if (applyForm.value.validationMethod === 'dns') {
      switch (applyForm.value.dnsProvider) {
        case 'dns_cf':
          requestData.dns_env = {
            CF_Token: applyForm.value.cfToken,
            CF_Account_ID: applyForm.value.cfAccountId
          }
          break
        case 'dns_ali':
          requestData.dns_env = {
            Ali_Key: applyForm.value.aliKey,
            Ali_Secret: applyForm.value.aliSecret
          }
          break
        case 'dns_tencent':
          requestData.dns_env = {
            Tencent_SecretId: applyForm.value.tencentSecretId,
            Tencent_SecretKey: applyForm.value.tencentSecretKey
          }
          break
        case 'dns_dp':
          requestData.dns_env = {
            DP_Id: applyForm.value.dpId,
            DP_Key: applyForm.value.dpKey
          }
          break
        case 'dns_aws':
          requestData.dns_env = {
            AWS_ACCESS_KEY_ID: applyForm.value.awsAccessKeyId,
            AWS_SECRET_ACCESS_KEY: applyForm.value.awsSecretAccessKey
          }
          break
      }
    }
    
    // 调用 API 申请证书
    const resp = await certificatesApi.apply(requestData)
    
    ElMessage.success(resp?.message || '证书申请已提交，请等待处理结果（通常需要 1-5 分钟）')
    applyDialogVisible.value = false
    
    // 重新获取证书列表
    fetchCertificates()
  } catch (error) {
    console.error('Failed to apply certificate:', error)
    const message = error?.response?.data?.error?.message || error?.message || '申请证书失败'
    ElMessage.error(message)
  } finally {
    applying.value = false
  }
}

// 处理上传证书
const handleUpload = () => {
  uploadForm.value = {
    domain: '',
    certFile: null,
    keyFile: null
  }
  uploadDialogVisible.value = true
}

// 处理证书文件选择
const handleCertFileChange = (file) => {
  uploadForm.value.certFile = file.raw
}

// 处理私钥文件选择
const handleKeyFileChange = (file) => {
  uploadForm.value.keyFile = file.raw
}

// 确认上传证书
const confirmUpload = async () => {
  if (!uploadFormRef.value) return
  
  try {
    await uploadFormRef.value.validate()

    await certificatesApi.upload({
      domain: uploadForm.value.domain,
      certFile: uploadForm.value.certFile,
      keyFile: uploadForm.value.keyFile,
      autoRenew: false
    })

    ElMessage.success('证书上传成功')
    uploadDialogVisible.value = false
    
    // 重新获取证书列表
    await fetchCertificates()
  } catch (error) {
    console.error('Failed to upload certificate:', error)
    ElMessage.error(error?.message || '上传证书失败')
  }
}

// 处理自动续期设置变更
const handleAutoRenewChange = async (row) => {
  const newValue = row.autoRenew
  try {
    await certificatesApi.updateAutoRenew(row.id, newValue)
    ElMessage.success(`${row.autoRenew ? '已开启' : '已关闭'}自动续期`)
  } catch (error) {
    console.error('Failed to update auto renew setting:', error)
    ElMessage.error('更新自动续期设置失败')
    // 恢复原状态
    row.autoRenew = !newValue
  }
}

// 处理续期证书
const handleRenew = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要为域名 ${row.domain} 续期证书吗？`, '续期证书', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    await certificatesApi.renew(row.id)
    ElMessage.success('证书续期已提交，请等待处理结果')
    await fetchCertificates()
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    console.error('Failed to renew certificate:', error)
    ElMessage.error('续期证书失败')
  }
}

// 处理验证证书
const handleValidate = async (row) => {
  try {
    const response = await certificatesApi.validate(row.id)
    const result = {
      success: !!response?.success,
      message: response?.message || '证书验证完成',
      details: response?.details || `域名: ${row.domain}\n状态: ${getStatusText(row.status)}`
    }
    
    validateResult.value = result
    validateDialogVisible.value = true
  } catch (error) {
    console.error('Failed to validate certificate:', error)
    ElMessage.error('验证证书失败')
  }
}

// 处理备份证书
const handleBackup = async (row) => {
  try {
    await certificatesApi.backup(row.id)
    ElMessage.success('证书备份已下载')
  } catch (error) {
    console.error('Failed to backup certificate:', error)
    ElMessage.error('当前版本暂不支持证书备份接口')
  }
}

// 处理删除证书
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要删除域名 ${row.domain} 的证书吗？此操作不可恢复！`, '删除证书', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    await certificatesApi.delete(row.id)
    ElMessage.success('证书已删除')
    await fetchCertificates()
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    console.error('Failed to delete certificate:', error)
    ElMessage.error('删除证书失败')
  }
}

// 处理刷新
const handleRefresh = () => {
  fetchCertificates()
}

// 获取提供商类型
const getProviderType = (provider) => {
  const types = {
    'letsencrypt': 'success',
    'zerossl': 'primary',
    'manual': 'warning',
    'self-signed': 'info'
  }
  return types[provider] || 'info'
}

// 获取状态标签类型
const getStatusType = (status) => {
  const types = {
    pending: 'info',
    failed: 'danger',
    expired: 'danger',
    expiring: 'warning',
    valid: 'success',
    active: 'success'
  }
  return types[status] || 'info'
}

const getStatusText = (status) => {
  const labels = {
    pending: '申请中',
    failed: '失败',
    expired: '已过期',
    expiring: '即将过期',
    valid: '有效',
    active: '有效'
  }
  return labels[status] || status || '未知'
}

// 获取过期时间标签类型
const getExpireStatusType = (row) => {
  if (!row?.expireDate || row.expireDate === '-') {
    return row?.status === 'failed' ? 'danger' : 'info'
  }

  const now = new Date()
  const expire = new Date(row.expireDate)
  const diff = expire - now
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  
  if (days < 0) {
    return 'danger'
  } else if (days < 30) {
    return 'warning'
  } else {
    return 'success'
  }
}
</script>

<style scoped>
.certificates-container {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.config-guide {
  font-size: 13px;
  line-height: 1.8;
}

.config-guide p {
  margin: 5px 0;
}

.dns-record {
  display: flex;
  gap: 10px;
  margin-bottom: 10px;
}

.validate-result {
  padding: 20px;
}

.result-status {
  margin-bottom: 20px;
}

.result-details {
  background-color: #f5f7fa;
  padding: 15px;
  border-radius: 4px;
}

.detail-item {
  margin-bottom: 10px;
}

.detail-item .label {
  font-weight: bold;
  margin-right: 10px;
}

.details-content {
  margin: 10px 0;
  padding: 10px;
  background-color: var(--el-bg-color, #fff);
  border: 1px solid var(--el-border-color, #eee);
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style> 
