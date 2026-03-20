<template>
  <div class="certificates-container">
    <div class="page-header">
      <div class="page-heading">
        <h1 class="page-title">证书管理</h1>
        <p class="page-subtitle">集中处理证书申请、上传、续期和可用性检查</p>
      </div>
      <div class="page-actions">
        <el-button type="primary" @click="handleApply">申请证书</el-button>
        <el-button type="success" @click="handleUpload">上传证书</el-button>
        <el-button @click="handleRefresh">刷新</el-button>
      </div>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">当前匹配</span>
        <strong class="overview-value">{{ displayCertificateTotal }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">有效证书</span>
        <strong class="overview-value is-success">{{ validCertificateCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">即将过期</span>
        <strong class="overview-value is-warning">{{ expiringCertificateCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">异常证书</span>
        <strong class="overview-value is-danger">{{ failedCertificateCount }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-filters">
        <el-input
          v-model="searchQuery"
          class="toolbar-search"
          clearable
          placeholder="搜索域名或提供商"
          @input="handleFilterChange"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select
          v-model="providerFilter"
          clearable
          placeholder="提供商"
          @change="handleFilterChange"
        >
          <el-option
            v-for="provider in providerOptions"
            :key="provider"
            :label="formatProviderLabel(provider)"
            :value="provider"
          />
        </el-select>
        <el-select
          v-model="statusFilter"
          clearable
          placeholder="状态"
          @change="handleFilterChange"
        >
          <el-option
            v-for="option in statusOptions"
            :key="option.value"
            :label="option.label"
            :value="option.value"
          />
        </el-select>
        <el-button @click="resetFilters">重置</el-button>
      </div>
      <div class="toolbar-actions">
        <span class="toolbar-summary">当前筛选 {{ displayCertificateTotal }} 张证书，当前页 {{ paginatedCertificates.length }} 张</span>
      </div>
    </div>

    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>证书列表</span>
          <span class="toolbar-summary">展示 {{ paginatedCertificates.length }} / {{ displayCertificateTotal }} 张</span>
        </div>
      </template>

      <el-alert
        v-if="applyProgress"
        class="apply-progress-alert"
        :title="getApplyProgressTitle()"
        :description="getApplyProgressDescription()"
        :type="getApplyProgressType()"
        show-icon
        @close="clearApplyProgress"
      />

      <div v-if="canQuickCreateInbound" class="apply-progress-actions">
        <el-button type="primary" size="small" @click="openInboundWithCertificateDomain">
          用此域名新增 TLS 入站
        </el-button>
      </div>

      <div class="table-shell">
        <el-table
          :data="paginatedCertificates"
          border
          stripe
          v-loading="loading"
          class="certificates-table"
          row-key="id"
          :empty-text="displayCertificateTotal ? '当前页暂无数据' : (hasCertificateFilters ? '暂无匹配的证书' : '暂无证书记录')"
        >
          <el-table-column label="证书对象" min-width="280">
            <template #default="{ row }">
              <div class="entity-cell">
                <div class="entity-cell__header">
                  <span class="entity-cell__title" :title="row.domain">{{ row.domain }}</span>
                  <span :class="['metric-pill', getProviderPillClass(row.provider)]">
                    {{ formatProviderLabel(row.provider) }}
                  </span>
                </div>
                <div class="entity-cell__meta">
                  <span>ID：{{ row.id }}</span>
                  <span>{{ row.domain?.startsWith('*.') ? '通配符证书' : '单域名证书' }}</span>
                </div>
                <div class="entity-cell__hint">
                  {{ row.domain?.startsWith('*.') ? '适合同一主域名下多个子域名复用。' : '适合单个业务域名直接使用。' }}
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="生命周期" min-width="220">
            <template #default="{ row }">
              <div class="stack-cell">
                <div class="stack-item">
                  <span class="stack-label">创建日期</span>
                  <span class="stack-value">{{ row.issueDate }}</span>
                </div>
                <div class="stack-item">
                  <span class="stack-label">过期日期</span>
                  <span :class="['stack-value', getExpireValueClass(row)]">{{ row.expireDate }}</span>
                </div>
                <div class="entity-cell__hint">
                  {{ getExpireHint(row) }}
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="续期与状态" min-width="250">
            <template #default="{ row }">
              <div class="stack-cell">
                <div class="stack-item stack-item--inline">
                  <span class="stack-label">证书状态</span>
                  <span :class="['metric-pill', getStatusPillClass(row.status)]">
                    {{ getStatusText(row.status) }}
                  </span>
                </div>
                <div class="stack-item stack-item--inline">
                  <span class="stack-label">自动续期</span>
                  <el-switch
                    v-model="row.autoRenew"
                    @change="handleAutoRenewChange(row)"
                  />
                </div>
                <div :class="['entity-cell__hint', row.errorMessage ? 'is-danger' : '']">
                  {{ row.errorMessage || getRenewHint(row) }}
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="操作" width="190" align="right" fixed="right">
            <template #default="{ row }">
              <div class="operation-btns">
                <el-button
                  size="small"
                  class="row-action row-action--primary"
                  @click="handleRenew(row)"
                >
                  续期
                </el-button>
                <el-button
                  size="small"
                  class="row-action row-action--success"
                  @click="handleValidate(row)"
                >
                  验证
                </el-button>
                <el-dropdown trigger="click" @command="(command) => handleRowCommand(command, row)">
                  <el-button size="small" class="row-action row-action--more" circle title="更多操作">
                    <el-icon><MoreFilled /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="backup">备份证书</el-dropdown-item>
                      <el-dropdown-item command="delete" divided>删除证书</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-card>

    <div class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        :total="displayCertificateTotal"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>

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
                  <li>Permissions: Zone → DNS → Edit，Zone → Zone → Read</li>
                  <li>Zone Resources: 选择你的域名</li>
                  <li>创建并复制 Token、Account ID（可选）和 Zone ID（推荐）</li>
                </ol>
                <p style="margin-top: 8px"><strong>兼容模式：</strong>也可切换为 x-ui 同款的 “邮箱 + Global API Key” 认证。</p>
              </div>
            </el-alert>

            <el-form-item label="认证方式">
              <el-radio-group v-model="applyForm.cfAuthMode">
                <el-radio value="token">API Token（推荐）</el-radio>
                <el-radio value="global">Global API Key（x-ui）</el-radio>
              </el-radio-group>
            </el-form-item>

            <template v-if="applyForm.cfAuthMode === 'token'">
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
                  placeholder="可选：Cloudflare 账户 ID（非区域 ID）"
                />
              </el-form-item>

              <el-form-item label="Zone ID" prop="cfZoneId">
                <el-input 
                  v-model="applyForm.cfZoneId" 
                  placeholder="推荐：Cloudflare 区域 ID（Overview 页面右侧）"
                />
              </el-form-item>
            </template>

            <template v-else>
              <el-form-item label="Cloudflare Email" prop="cfEmail">
                <el-input 
                  v-model="applyForm.cfEmail"
                  placeholder="Cloudflare 注册邮箱"
                />
              </el-form-item>

              <el-form-item label="Global API Key" prop="cfGlobalKey">
                <el-input 
                  v-model="applyForm.cfGlobalKey"
                  type="password"
                  show-password
                  placeholder="Cloudflare Global API Key"
                />
              </el-form-item>

              <el-form-item label="Zone ID" prop="cfZoneId">
                <el-input 
                  v-model="applyForm.cfZoneId" 
                  placeholder="可选：Cloudflare 区域 ID（可加速定位 Zone）"
                />
              </el-form-item>
            </template>
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

        <el-form-item label="自动关联节点">
          <el-select
            v-model="applyForm.node_ids"
            multiple
            filterable
            clearable
            collapse-tags
            collapse-tags-tooltip
            :loading="nodesLoading"
            placeholder="签发成功后自动关联到这些节点"
            style="width: 100%"
          >
            <el-option
              v-for="node in nodes"
              :key="node.id"
              :label="`${node.name} (${node.address})`"
              :value="node.id"
            />
          </el-select>
          <div class="form-tip-inline">
            证书签发成功后会自动写入所选节点的证书关联；如果节点已配置 SSH，也会尝试自动下发证书。
          </div>
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
        <el-form-item label="自动关联节点">
          <el-select
            v-model="uploadForm.node_ids"
            multiple
            filterable
            clearable
            collapse-tags
            collapse-tags-tooltip
            :loading="nodesLoading"
            placeholder="上传成功后自动关联到这些节点"
            style="width: 100%"
          >
            <el-option
              v-for="node in nodes"
              :key="node.id"
              :label="`${node.name} (${node.address})`"
              :value="node.id"
            />
          </el-select>
          <div class="form-tip-inline">
            上传成功后会自动关联所选节点，并尝试把证书下发到已配置 SSH 的节点。
          </div>
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
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { MoreFilled, Search } from '@element-plus/icons-vue'
import { certificatesApi, nodesApi } from '@/api'

const router = useRouter()

// 证书列表
const certificates = ref([])
const loading = ref(false)
const nodes = ref([])
const nodesLoading = ref(false)
const searchQuery = ref('')
const providerFilter = ref('')
const statusFilter = ref('')
const currentPage = ref(1)
const pageSize = ref(10)

// 申请证书
const applyDialogVisible = ref(false)
const applyFormRef = ref(null)
const applying = ref(false)
const applyProgress = ref(null)
let applyProgressTimer = null
let applyProgressPolling = false
const applyForm = ref({
  domain: '',
  email: '',
  provider: 'letsencrypt',
  validationMethod: 'dns',
  webroot: '',
  dnsProvider: 'dns_cf',
  cfAuthMode: 'global',
  cfToken: '',
  cfAccountId: '',
  cfZoneId: '',
  cfEmail: '',
  cfGlobalKey: '',
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
  wildcard: true,
  autoRenew: true,
  node_ids: []
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
  keyFile: null,
  node_ids: []
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

const providerOptions = computed(() => [...new Set(certificates.value.map((item) => item.provider).filter(Boolean))])
const statusOptions = [
  { label: '申请中', value: 'pending' },
  { label: '有效', value: 'active' },
  { label: '即将过期', value: 'expiring' },
  { label: '已过期', value: 'expired' },
  { label: '失败', value: 'failed' }
]

const filteredCertificates = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()

  return certificates.value.filter((item) => {
    const matchesQuery = !query || item.domain?.toLowerCase().includes(query) || String(item.provider || '').toLowerCase().includes(query)
    const matchesProvider = !providerFilter.value || item.provider === providerFilter.value
    const matchesStatus = !statusFilter.value || item.status === statusFilter.value

    return matchesQuery && matchesProvider && matchesStatus
  })
})

const displayCertificateTotal = computed(() => filteredCertificates.value.length)
const hasCertificateFilters = computed(() => Boolean(searchQuery.value.trim() || providerFilter.value || statusFilter.value))
const paginatedCertificates = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredCertificates.value.slice(start, end)
})

const totalCertificateCount = computed(() => filteredCertificates.value.length)
const validCertificateCount = computed(() =>
  filteredCertificates.value.filter((item) => ['valid', 'active'].includes(item.status)).length
)
const expiringCertificateCount = computed(() =>
  filteredCertificates.value.filter((item) => getExpireStatusType(item) === 'warning').length
)
const failedCertificateCount = computed(() =>
  filteredCertificates.value.filter((item) =>
    ['failed', 'expired'].includes(item.status) || getExpireStatusType(item) === 'danger'
  ).length
)

const normalizeDomain = (domain = '') => domain.replace(/^\*\./, '').trim().toLowerCase()

const syncCurrentPage = () => {
  const maxPage = Math.max(1, Math.ceil(displayCertificateTotal.value / pageSize.value))
  if (currentPage.value > maxPage) {
    currentPage.value = maxPage
  }
}

const getApplyErrorMessage = (error) => {
  const rawMessage = [
    error?.response?.data?.error?.message,
    error?.response?.data?.message,
    error?.response?.data?.error,
    error?.message
  ].find(item => typeof item === 'string' && item.trim())

  if (!rawMessage) return '申请证书失败，请稍后重试。'

  if (rawMessage.includes('forbidden domain "example.com"') || rawMessage.includes('forbidden domain \\\"example.com\\\"')) {
    return '申请失败：邮箱域名 example.com 不允许用于 ACME 注册，请填写可用邮箱后重试。'
  }

  if (rawMessage.includes('invalid domain') && rawMessage.includes('_acme-challenge')) {
    return '申请失败：DNS 验证记录写入失败，请检查 DNS 提供商凭证、域名托管区域和权限。'
  }

  if (rawMessage.includes('正在申请中')) {
    return '该域名证书正在申请中，请稍后刷新查看结果。'
  }

  if (rawMessage.includes('已存在')) {
    return '该域名证书已存在，请先删除旧证书或直接续期。'
  }

  const compactMessage = rawMessage.replace(/\s+/g, ' ').trim()
  return compactMessage.length > 180 ? `${compactMessage.slice(0, 180)}...` : compactMessage
}

const mapCertificate = (cert) => {
  const autoRenew = cert.auto_renew ?? cert.autoRenew ?? false
  const status = cert.status || 'pending'
  const expiresAt = cert.expires_at || cert.expiresAt || ''
  const errorMessage = cert.error_message || cert.errorMessage || ''

  return {
    ...cert,
    autoRenew,
    status,
    errorMessage,
    issueDate: formatDate(cert.created_at || cert.createdAt),
    expireDate: formatDate(expiresAt)
  }
}

// 生命周期钩子
onMounted(async () => {
  await Promise.all([fetchCertificates(), fetchNodes()])
})

onUnmounted(() => {
  stopApplyProgressTracking()
})

// 获取证书列表
const fetchCertificates = async ({ silent = false } = {}) => {
  if (!silent) {
    loading.value = true
  }
  try {
    const response = await certificatesApi.list()
    const data = normalizeCertificatesResponse(response)
    certificates.value = data.map(mapCertificate)
    syncCurrentPage()
  } catch (error) {
    console.error('Failed to fetch certificates:', error)
    if (!silent) {
      ElMessage.error('获取证书列表失败')
      certificates.value = []
    }
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

const fetchNodes = async () => {
  nodesLoading.value = true
  try {
    const response = await nodesApi.list({ limit: 1000, offset: 0 })
    if (Array.isArray(response)) {
      nodes.value = response
      return
    }
    if (Array.isArray(response?.nodes)) {
      nodes.value = response.nodes
      return
    }
    if (Array.isArray(response?.data)) {
      nodes.value = response.data
      return
    }
    nodes.value = []
  } catch (error) {
    console.error('Failed to fetch nodes:', error)
    nodes.value = []
  } finally {
    nodesLoading.value = false
  }
}

const stopApplyProgressTracking = () => {
  applyProgressPolling = false
  if (applyProgressTimer) {
    clearInterval(applyProgressTimer)
    applyProgressTimer = null
  }
}

const clearApplyProgress = () => {
  stopApplyProgressTracking()
  applyProgress.value = null
}

const canQuickCreateInbound = computed(() => ['active', 'valid', 'expiring'].includes(applyProgress.value?.status))

const openInboundWithCertificateDomain = () => {
  if (!applyProgress.value?.domain) return
  const sourceDomain = String(applyProgress.value.domain).trim()
  const suggestedDomain = normalizeDomain(sourceDomain)
  if (sourceDomain.startsWith('*.')) {
    ElMessage.warning('已带入主域名，请在入站页面改成实际使用的子域名，例如 api.example.com')
  }
  router.push({
    path: '/admin/inbounds',
    query: {
      create: '1',
      tls_domain: suggestedDomain
    }
  })
}

const getApplyProgressType = () => {
  if (!applyProgress.value) return 'info'
  if (['active', 'valid', 'expiring'].includes(applyProgress.value.status)) return 'success'
  if (applyProgress.value.status === 'pending') return 'info'
  if (applyProgress.value.status === 'timeout') return 'warning'
  return 'error'
}

const getApplyProgressTitle = () => {
  if (!applyProgress.value) return ''
  const status = applyProgress.value.status
  if (status === 'pending') return `证书申请处理中：${applyProgress.value.domain}`
  if (['active', 'valid', 'expiring'].includes(status)) return `证书申请完成：${applyProgress.value.domain}`
  if (status === 'timeout') return `证书申请仍在处理中：${applyProgress.value.domain}`
  return `证书申请失败：${applyProgress.value.domain}`
}

const getApplyProgressDescription = () => {
  if (!applyProgress.value) return ''
  const status = applyProgress.value.status
  if (status === 'pending') {
    const elapsedSeconds = Math.max(1, Math.floor((Date.now() - (applyProgress.value.submittedAt || Date.now())) / 1000))
    return `已提交到后台，系统每 5 秒自动刷新状态（已等待 ${elapsedSeconds} 秒）。你可以继续操作其他页面。`
  }
  if (['active', 'valid', 'expiring'].includes(status)) {
    const expireInfo = applyProgress.value.expireDate && applyProgress.value.expireDate !== '-' ? `，过期日期：${applyProgress.value.expireDate}` : ''
    const nodeInfo = applyProgress.value.requestedNodeCount > 0 ? ` 已自动关联 ${applyProgress.value.requestedNodeCount} 个节点。` : ''
    return `证书已签发${expireInfo}。${nodeInfo}`
  }
  if (status === 'timeout') {
    return '后台申请时间较长，已停止自动轮询。你可以点击“刷新”继续查看结果。'
  }
  return applyProgress.value.errorMessage || '后台申请失败，请检查 DNS 凭证、邮箱和服务器日志。'
}

const updateApplyProgressStatus = () => {
  if (!applyProgress.value) return
  const hasCertId = applyProgress.value.certId !== null && applyProgress.value.certId !== undefined && applyProgress.value.certId !== ''
  const trackedDomain = normalizeDomain(applyProgress.value.domain)
  const current = hasCertId
    ? certificates.value.find(cert => Number(cert.id) === Number(applyProgress.value.certId))
    : certificates.value.find(cert => normalizeDomain(cert.domain) === trackedDomain)
  if (!current) return

  if (!hasCertId) {
    applyProgress.value.certId = current.id
  }

  applyProgress.value.status = current.status
  applyProgress.value.errorMessage = current.errorMessage || current.error_message || ''
  applyProgress.value.expireDate = current.expireDate

  if (['active', 'valid', 'expiring', 'failed', 'expired'].includes(current.status)) {
    stopApplyProgressTracking()
  }
}

const startApplyProgressTracking = async (certId, domain, requestedNodeCount = 0) => {
  stopApplyProgressTracking()
  const normalizedCertId = certId === null || certId === undefined || certId === '' ? null : Number(certId)
  applyProgress.value = {
    certId: Number.isFinite(normalizedCertId) ? normalizedCertId : null,
    domain,
    status: 'pending',
    errorMessage: '',
    expireDate: '-',
    checks: 0,
    requestedNodeCount,
    submittedAt: Date.now()
  }

  const poll = async () => {
    if (!applyProgress.value || applyProgressPolling) return
    applyProgressPolling = true
    applyProgress.value.checks += 1

    try {
      await fetchCertificates({ silent: true })
      updateApplyProgressStatus()

      if (!applyProgress.value) return
      if (applyProgress.value.checks >= 120 && applyProgress.value.status === 'pending') {
        applyProgress.value.status = 'timeout'
        stopApplyProgressTracking()
      }
    } finally {
      applyProgressPolling = false
    }
  }

  await poll()
  if (applyProgress.value?.status === 'pending') {
    applyProgressTimer = setInterval(poll, 5000)
  }
}

// 处理申请证书
const handleApply = () => {
  applyForm.value = {
    domain: '',
    email: '',
    provider: 'letsencrypt',
    validationMethod: 'dns',
    webroot: '',
    dnsProvider: 'dns_cf',
    cfAuthMode: 'global',
    cfToken: '',
    cfAccountId: '',
    cfZoneId: '',
    cfEmail: '',
    cfGlobalKey: '',
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
    wildcard: true,
    autoRenew: true,
    node_ids: []
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
  applyForm.value.cfZoneId = ''
  applyForm.value.cfEmail = ''
  applyForm.value.cfGlobalKey = ''
  applyForm.value.cfAuthMode = 'global'
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
  let requestData = null
  
  try {
    applying.value = true
    const formValid = await applyFormRef.value.validate().then(() => true).catch(() => false)
    if (!formValid) return
    
    // 构建请求数据
    requestData = {
      domain: applyForm.value.domain,
      email: applyForm.value.email,
      provider: applyForm.value.provider,
      method: applyForm.value.validationMethod,
      webroot: applyForm.value.webroot,
      dns_provider: applyForm.value.dnsProvider,
      dns_env: {},
      auto_renew: applyForm.value.autoRenew,
      wildcard: applyForm.value.wildcard,
      node_ids: [...applyForm.value.node_ids]
    }

    // 根据 DNS 提供商添加相应的凭证
    if (applyForm.value.validationMethod === 'dns') {
      switch (applyForm.value.dnsProvider) {
        case 'dns_cf':
          if (applyForm.value.cfAuthMode === 'global') {
            if (!applyForm.value.cfEmail || !applyForm.value.cfGlobalKey) {
              ElMessage.warning('Cloudflare Global API Key 模式下，请填写 Email 和 Global API Key')
              return
            }
            requestData.dns_env = {
              CF_Email: applyForm.value.cfEmail,
              CF_Key: applyForm.value.cfGlobalKey
            }
          } else {
            if (!applyForm.value.cfToken) {
              ElMessage.warning('Cloudflare Token 模式下，请填写 API Token')
              return
            }
            requestData.dns_env = {
              CF_Token: applyForm.value.cfToken,
              CF_Account_ID: applyForm.value.cfAccountId
            }
          }
          if (applyForm.value.cfZoneId) {
            requestData.dns_env.CF_Zone_ID = applyForm.value.cfZoneId
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
    const resp = await certificatesApi.apply(requestData, {
      silent: true,
      // 首次安装 acme.sh 可能超过默认 30s 超时
      timeout: 300000
    })
    
    const requestedNodeCount = requestData.node_ids.length
    ElMessage.success(requestedNodeCount > 0 ? `证书申请已提交，签发成功后会自动关联 ${requestedNodeCount} 个节点` : (resp?.message || '证书申请已提交，请等待处理结果（通常需要 1-5 分钟）'))
    applyDialogVisible.value = false
    
    // 重新获取证书列表
    await fetchCertificates()

    await startApplyProgressTracking(resp?.cert_id ?? null, requestData.domain, requestData.node_ids.length)
  } catch (error) {
    console.error('Failed to apply certificate:', error)
    const message = getApplyErrorMessage(error)
    applyProgress.value = {
      certId: null,
      domain: requestData?.domain || applyForm.value.domain || '-',
      status: 'failed',
      errorMessage: message,
      expireDate: '-',
      checks: 0,
      requestedNodeCount: requestData?.node_ids?.length || 0,
      submittedAt: Date.now()
    }
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
    keyFile: null,
    node_ids: []
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

    const requestData = {
      domain: uploadForm.value.domain,
      certFile: uploadForm.value.certFile,
      keyFile: uploadForm.value.keyFile,
      autoRenew: false,
      node_ids: [...uploadForm.value.node_ids]
    }

    const response = await certificatesApi.upload(requestData)

    ElMessage.success(requestData.node_ids.length > 0 ? `证书上传成功，已自动关联 ${requestData.node_ids.length} 个节点` : '证书上传成功')
    uploadDialogVisible.value = false
    
    // 重新获取证书列表
    await fetchCertificates()

    const uploadedCert = certificates.value.find(cert => normalizeDomain(cert.domain) === normalizeDomain(requestData.domain))
    const responseCert = response?.certificate || response
    applyProgress.value = {
      certId: uploadedCert?.id ?? responseCert?.id ?? null,
      domain: requestData.domain,
      status: uploadedCert?.status || responseCert?.status || 'active',
      errorMessage: '',
      expireDate: uploadedCert?.expireDate || '-',
      checks: 0,
      requestedNodeCount: requestData.node_ids.length,
      submittedAt: Date.now()
    }
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
    if (applyProgress.value?.certId === row.id) {
      clearApplyProgress()
    }
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

const handleFilterChange = () => {
  currentPage.value = 1
  syncCurrentPage()
}

const resetFilters = () => {
  searchQuery.value = ''
  providerFilter.value = ''
  statusFilter.value = ''
  currentPage.value = 1
}

const handleSizeChange = (value) => {
  pageSize.value = value
  syncCurrentPage()
}

const handleCurrentChange = (value) => {
  currentPage.value = value
}

const handleRowCommand = (command, row) => {
  if (command === 'backup') {
    handleBackup(row)
    return
  }

  if (command === 'delete') {
    handleDelete(row)
  }
}

const formatProviderLabel = (provider) => {
  const labels = {
    letsencrypt: "Let's Encrypt",
    zerossl: 'ZeroSSL',
    manual: '手动上传',
    'self-signed': '自签名'
  }

  return labels[provider] || provider || '未识别'
}

const getProviderPillClass = (provider) => {
  const classes = {
    letsencrypt: 'is-success',
    zerossl: 'is-primary',
    manual: 'is-warning',
    'self-signed': 'is-muted'
  }

  return classes[provider] || 'is-muted'
}

const getStatusPillClass = (status) => {
  const classes = {
    pending: 'is-primary',
    failed: 'is-danger',
    expired: 'is-danger',
    expiring: 'is-warning',
    valid: 'is-success',
    active: 'is-success'
  }

  return classes[status] || 'is-muted'
}

const getExpireValueClass = (row) => {
  const type = getExpireStatusType(row)
  if (type === 'danger') return 'is-danger'
  if (type === 'warning') return 'is-warning'
  if (type === 'success') return 'is-success'
  return ''
}

const getExpireHint = (row) => {
  if (!row?.expireDate || row.expireDate === '-') {
    return row?.status === 'pending' ? '等待签发完成后生成过期时间。' : '当前没有可用的过期时间记录。'
  }

  const expire = new Date(row.expireDate)
  if (Number.isNaN(expire.getTime())) {
    return '过期时间格式异常，请刷新后重试。'
  }

  const diff = expire.getTime() - Date.now()
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))

  if (days < 0) {
    return `证书已过期 ${Math.abs(days)} 天，请立即续期或重新签发。`
  }

  if (days < 30) {
    return `距离过期还有 ${days} 天，建议尽快检查续期链路。`
  }

  return `距离过期还有 ${days} 天，当前处于稳定周期。`
}

const getRenewHint = (row) => {
  if (!row.autoRenew) {
    return '当前关闭自动续期，需要人工关注到期时间。'
  }

  if (['failed', 'expired'].includes(row.status)) {
    return '建议先排查签发失败原因，再重新续期。'
  }

  return '已开启自动续期，系统会在到期前自动尝试更新。'
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

.certificates-table {
  width: 100%;
  min-width: 930px;
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

.apply-progress-alert {
  margin-bottom: 16px;
}

.apply-progress-actions {
  margin: -4px 0 16px;
  display: flex;
  justify-content: flex-end;
}

.form-tip-inline {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 6px;
  line-height: 1.5;
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

@media (max-width: 768px) {
  .certificates-container {
    padding: 12px;
  }

  .certificates-table {
    min-width: 760px;
  }
}
</style> 
