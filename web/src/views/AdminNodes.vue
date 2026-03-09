<template>
  <div class="admin-nodes-page">
    <div class="page-header">
      <h1 class="page-title">节点管理</h1>
      <div class="header-actions">
        <el-button @click="fetchNodes">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="primary" @click="showCreateDialog">
          <el-icon><Plus /></el-icon>
          添加节点
        </el-button>
      </div>
    </div>

    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="never" class="stat-card">
          <div class="stat-content">
            <div class="stat-value">{{ nodeStore.nodeCount }}</div>
            <div class="stat-label">总节点数</div>
          </div>
          <el-icon class="stat-icon"><Monitor /></el-icon>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="never" class="stat-card online">
          <div class="stat-content">
            <div class="stat-value">{{ nodeStore.onlineCount }}</div>
            <div class="stat-label">在线节点</div>
          </div>
          <el-icon class="stat-icon"><CircleCheck /></el-icon>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="never" class="stat-card">
          <div class="stat-content">
            <div class="stat-value">{{ nodeStore.totalUsers }}</div>
            <div class="stat-label">总用户数</div>
          </div>
          <el-icon class="stat-icon"><User /></el-icon>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="never" class="stat-card">
          <div class="stat-content">
            <div class="stat-value">{{ nodeStore.averageLatency }}ms</div>
            <div class="stat-label">平均延迟</div>
          </div>
          <el-icon class="stat-icon"><Timer /></el-icon>
        </el-card>
      </el-col>
    </el-row>

    <!-- 筛选栏 -->
    <el-card shadow="never" class="filter-card">
      <el-row :gutter="16">
        <el-col :span="6">
          <el-input
            v-model="nodeStore.filters.search"
            placeholder="搜索节点名称/地址"
            clearable
            @clear="fetchNodes"
            @keyup.enter="fetchNodes"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </el-col>
        <el-col :span="4">
          <el-select
            v-model="nodeStore.filters.status"
            placeholder="状态"
            clearable
            @change="fetchNodes"
          >
            <el-option label="在线" value="online" />
            <el-option label="离线" value="offline" />
            <el-option label="不健康" value="unhealthy" />
          </el-select>
        </el-col>
        <el-col :span="4">
          <el-select
            v-model="nodeStore.filters.region"
            placeholder="地区"
            clearable
            @change="fetchNodes"
          >
            <el-option
              v-for="region in regions"
              :key="region"
              :label="region"
              :value="region"
            />
          </el-select>
        </el-col>
        <el-col :span="4">
          <el-button
            @click="
              nodeStore.clearFilters();
              fetchNodes();
            "
            >重置</el-button
          >
        </el-col>
      </el-row>
    </el-card>

    <!-- 节点列表 -->
    <el-card shadow="never">
      <el-table
        :data="nodeStore.filteredNodes"
        v-loading="nodeStore.loading"
        style="width: 100%"
      >
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="名称" min-width="120">
          <template #default="{ row }">
            <el-link type="primary" @click="viewNodeDetail(row)">{{
              row.name
            }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="address" label="地址" min-width="150">
          <template #default="{ row }">
            {{ row.address }}:{{ row.port }}
          </template>
        </el-table-column>
        <el-table-column label="TLS / 证书" min-width="180">
          <template #default="{ row }">
            <div class="tls-cell">
              <el-tag :type="row.tls_enabled ? 'success' : 'info'" size="small">
                {{ row.tls_enabled ? 'TLS 已启用' : '未启用 TLS' }}
              </el-tag>
              <div v-if="row.tls_domain" class="tls-domain-text">{{ row.tls_domain }}</div>
              <div v-if="row.certificate_id" class="tls-cert-text">
                证书：{{ getAssignedCertificateDisplay(row.certificate_id) }}
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="region" label="地区" width="100" />
        <el-table-column label="负载" width="120">
          <template #default="{ row }">
            <div class="load-info">
              <span>{{ row.current_users }}/{{ row.max_users || "∞" }}</span>
              <el-progress
                v-if="row.max_users > 0"
                :percentage="
                  Math.round((row.current_users / row.max_users) * 100)
                "
                :stroke-width="4"
                :show-text="false"
                :status="getLoadStatus(row)"
              />
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="latency" label="延迟" width="80">
          <template #default="{ row }">
            <span :class="getLatencyClass(row.latency)"
              >{{ row.latency }}ms</span
            >
          </template>
        </el-table-column>
        <el-table-column prop="weight" label="权重" width="70" />
        <el-table-column label="同步状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getSyncStatusType(row.sync_status)" size="small">
              {{ getSyncStatusText(row.sync_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="安装状态" width="110">
          <template #default="{ row }">
            <el-tag
              :type="getInstallStatusType(row.install_status)"
              size="small"
            >
              {{ getInstallStatusText(row.install_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="380" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="editNode(row)"
              >编辑</el-button
            >
            <el-button
              type="success"
              link
              size="small"
              @click="showDeployDialog(row)"
              >部署</el-button
            >
            <el-button
              type="info"
              link
              size="small"
              @click="downloadDeployScript(row)"
              >脚本</el-button
            >
            <el-button
              type="warning"
              link
              size="small"
              @click="showTokenDialog(row)"
              >Token</el-button
            >
            <el-button type="danger" link size="small" @click="deleteNode(row)"
              >删除</el-button
            >
          </template>
        </el-table-column>
      </el-table>

      <div v-if="nodeStore.total > 0" class="pagination-container">
        <el-pagination
          v-model:current-page="pagination.page"
          :total="nodeStore.total"
          :page-size="pagination.pageSize"
          layout="total, prev, pager, next"
          @current-change="fetchNodes"
        />
      </div>
    </el-card>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑节点' : '添加节点'"
      width="700px"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="120px">
        <el-divider content-position="left">节点信息</el-divider>

        <el-form-item label="节点名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入节点名称" />
        </el-form-item>

        <el-form-item label="地区" prop="region">
          <el-input v-model="form.region" placeholder="如：香港、日本、美国" />
        </el-form-item>

        <el-form-item label="权重" prop="weight">
          <el-input-number
            v-model="form.weight"
            :min="1"
            :max="100"
            style="width: 100%"
          />
          <span class="form-tip">负载均衡权重，数值越大分配用户越多</span>
        </el-form-item>

        <el-form-item label="最大用户数" prop="max_users">
          <el-input-number
            v-model="form.max_users"
            :min="0"
            style="width: 100%"
          />
          <span class="form-tip">0 表示无限制</span>
        </el-form-item>

        <el-form-item label="标签">
          <el-tag
            v-for="(tag, index) in form.tags"
            :key="index"
            closable
            @close="removeTag(index)"
            style="margin-right: 8px; margin-bottom: 8px"
          >
            {{ tag }}
          </el-tag>
          <el-input
            v-if="showTagInput"
            v-model="newTag"
            size="small"
            style="width: 120px"
            @keyup.enter="addTag"
            @blur="addTag"
          />
          <el-button v-else size="small" @click="showTagInput = true"
            >+ 添加标签</el-button
          >
        </el-form-item>

        <el-form-item label="IP 白名单">
          <el-input
            v-model="form.ip_whitelist_str"
            type="textarea"
            :rows="3"
            placeholder="每行一个 IP 地址，留空表示不限制"
          />
        </el-form-item>

        <el-divider content-position="left">TLS 与证书</el-divider>

        <el-form-item label="启用 TLS">
          <el-switch
            v-model="form.tls_enabled"
            active-text="开启"
            inactive-text="关闭"
          />
        </el-form-item>

        <el-form-item label="TLS 域名" prop="tls_domain">
          <el-input
            v-model="form.tls_domain"
            placeholder="如 jp.example.com"
          />
          <div class="form-tip-inline">用于节点 TLS 标识、健康检查和系统证书自动匹配。</div>
        </el-form-item>

        <el-form-item label="系统证书">
          <el-select
            v-model="form.certificate_id"
            filterable
            clearable
            :loading="certificatesLoading"
            placeholder="自动匹配或手动选择证书"
            style="width: 100%"
            @change="handleCertificateChange"
          >
            <el-option
              v-for="cert in certificates"
              :key="cert.id"
              :label="getCertificateOptionLabel(cert)"
              :value="cert.id"
            />
          </el-select>
          <div class="form-tip-inline">选择后会自动回填 TLS 域名，你仍可继续手动修改。</div>
          <div v-if="selectedCertificate" class="certificate-tip">
            当前证书：{{ selectedCertificate.domain }}
            <span v-if="selectedCertificate.expireDate && selectedCertificate.expireDate !== '-'">
              ，到期 {{ selectedCertificate.expireDate }}
            </span>
          </div>
        </el-form-item>

        <template v-if="!isEdit && form.installMethod === 'manual'">
          <el-divider content-position="left">节点连接信息</el-divider>

          <el-alert type="info" :closable="false" style="margin-bottom: 16px">
            手动安装模式需要提供节点地址和端口，用于后续连接
          </el-alert>

          <el-form-item label="节点地址" prop="address">
            <el-input v-model="form.address" placeholder="IP 地址或域名" />
          </el-form-item>

          <el-form-item label="节点端口" prop="port">
            <el-input-number
              v-model="form.port"
              :min="1"
              :max="65535"
              style="width: 100%"
            />
            <span class="form-tip">Agent 监听端口，默认 18443</span>
          </el-form-item>
        </template>

        <template v-if="!isEdit">
          <el-divider content-position="left">安装方式</el-divider>

          <el-form-item label="安装方式">
            <el-radio-group v-model="form.installMethod">
              <el-radio label="manual">稍后手动安装</el-radio>
              <el-radio label="auto">立即自动安装</el-radio>
            </el-radio-group>
          </el-form-item>

          <template v-if="form.installMethod === 'auto'">
            <el-alert type="info" :closable="false" style="margin-bottom: 16px">
              通过 SSH 连接到服务器并自动安装 Agent 和 Xray
            </el-alert>

            <el-form-item label="Panel 地址" prop="panel_url">
              <el-input
                v-model="form.panel_url"
                placeholder="http://your-panel-ip:8080"
              >
                <template #prepend>
                  <el-icon><Link /></el-icon>
                </template>
              </el-input>
              <span class="form-tip"
                >Agent 连接到 Panel 的地址，必须是 Agent
                服务器能访问的地址</span
              >
            </el-form-item>

            <el-form-item label="服务器地址" prop="ssh_host">
              <el-input v-model="form.ssh_host" placeholder="IP 地址或域名" />
            </el-form-item>

            <el-form-item label="SSH 端口" prop="ssh_port">
              <el-input-number
                v-model="form.ssh_port"
                :min="1"
                :max="65535"
                style="width: 100%"
              />
            </el-form-item>

            <el-form-item label="用户名" prop="ssh_username">
              <el-input
                v-model="form.ssh_username"
                placeholder="SSH 用户名 (通常为 root)"
              />
            </el-form-item>

            <el-form-item label="认证方式" prop="ssh_auth_method">
              <el-radio-group v-model="form.ssh_auth_method">
                <el-radio label="password">密码</el-radio>
                <el-radio label="key">私钥</el-radio>
              </el-radio-group>
            </el-form-item>

            <el-form-item
              v-if="form.ssh_auth_method === 'password'"
              label="密码"
              prop="ssh_password"
            >
              <el-input
                v-model="form.ssh_password"
                type="password"
                placeholder="SSH 密码"
                show-password
              />
            </el-form-item>

            <el-form-item
              v-if="form.ssh_auth_method === 'key'"
              label="私钥"
              prop="ssh_private_key"
            >
              <el-input
                v-model="form.ssh_private_key"
                type="textarea"
                :rows="6"
                placeholder="粘贴 SSH 私钥内容 (PEM 格式)"
              />
            </el-form-item>

            <el-form-item>
              <el-button
                @click="testSSHConnection"
                :loading="testingConnection"
              >
                <el-icon><Connection /></el-icon>
                测试连接
              </el-button>
              <span
                v-if="connectionTestResult"
                :style="{
                  marginLeft: '12px',
                  color: connectionTestResult.success
                    ? 'var(--el-color-success)'
                    : 'var(--el-color-danger)',
                }"
              >
                {{ connectionTestResult.message }}
              </span>
            </el-form-item>

            <el-alert
              v-if="connectionTestResult && !connectionTestResult.success"
              type="warning"
              :closable="false"
              style="margin-bottom: 16px"
            >
              <template #title>常见问题排查</template>
              <div style="font-size: 13px; line-height: 1.6">
                <p style="margin: 4px 0">
                  • 确认 SSH 服务正在运行: <code>systemctl status sshd</code>
                </p>
                <p style="margin: 4px 0">• 确认端口正确 (默认 22)</p>
                <p style="margin: 4px 0">• 确认防火墙允许 SSH 连接</p>
                <p style="margin: 4px 0">
                  • 如果使用密码认证，确认服务器允许:
                  <code>PasswordAuthentication yes</code>
                </p>
              </div>
            </el-alert>
          </template>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitForm">
          {{
            isEdit
              ? "保存"
              : form.installMethod === "auto"
                ? "添加并安装"
                : "添加节点"
          }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Token 管理对话框 -->
    <el-dialog v-model="tokenDialogVisible" title="Token 管理" width="500px">
      <div v-if="currentNode" class="token-dialog-content">
        <div class="token-info">
          <div class="token-label">节点名称</div>
          <div class="token-value">{{ currentNode.name }}</div>
        </div>
        <div class="token-info">
          <div class="token-label">当前 Token</div>
          <div class="token-value token-text">
            <span v-if="showToken">{{ currentToken || "未生成" }}</span>
            <span v-else>{{
              currentToken ? "••••••••••••••••" : "未生成"
            }}</span>
            <el-button v-if="currentToken" link @click="showToken = !showToken">
              <el-icon><View v-if="!showToken" /><Hide v-else /></el-icon>
            </el-button>
            <el-button v-if="currentToken" link @click="copyToken">
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </div>
        </div>
        <div class="token-actions">
          <el-button
            type="primary"
            @click="handleGenerateToken"
            :loading="tokenLoading"
          >
            {{ currentToken ? "重新生成" : "生成 Token" }}
          </el-button>
          <el-button
            v-if="currentToken"
            type="warning"
            @click="handleRotateToken"
            :loading="tokenLoading"
          >
            轮换 Token
          </el-button>
          <el-button
            v-if="currentToken"
            type="danger"
            @click="handleRevokeToken"
            :loading="tokenLoading"
          >
            撤销 Token
          </el-button>
        </div>
        <el-alert
          v-if="newToken"
          type="success"
          :closable="false"
          show-icon
          class="new-token-alert"
        >
          <template #title> 新 Token 已生成，请妥善保存： </template>
          <div class="new-token-text">{{ newToken }}</div>
        </el-alert>
      </div>
    </el-dialog>

    <!-- 部署进度对话框 -->
    <el-dialog
      v-model="deployProgressDialogVisible"
      title="部署进度"
      width="800px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
    >
      <div class="deploy-progress">
        <!-- 加载状态 - 只在没有步骤且没有结果时显示 -->
        <div
          v-if="!deployResult && deploySteps.length === 0 && !deployLogs"
          class="deploy-loading"
        >
          <el-icon class="is-loading" :size="40"><Loading /></el-icon>
          <p style="margin-top: 16px; color: #909399">
            {{ deployStatusMessage || "正在部署 Agent，请稍候..." }}
          </p>
        </div>

        <!-- 步骤显示 -->
        <el-steps
          v-if="deploySteps.length > 0"
          :active="deployStepActive"
          finish-status="success"
          process-status="finish"
          align-center
        >
          <el-step
            v-for="(step, index) in deploySteps"
            :key="index"
            :title="step.name || step"
            :status="getStepStatus(step, index)"
          />
        </el-steps>

        <!-- 日志显示 -->
        <div v-if="deployLogs" class="deploy-logs">
          <div class="logs-header">
            <span>部署日志</span>
            <el-button link @click="copyDeployLogs">
              <el-icon><CopyDocument /></el-icon>
              复制日志
            </el-button>
          </div>
          <pre class="logs-content">{{ deployLogs }}</pre>
        </div>

        <!-- 结果提示 -->
        <el-alert
          v-if="deployResult"
          :type="deployResult.success ? 'success' : 'error'"
          :title="deployResult.message"
          :closable="false"
          show-icon
          style="margin-top: 20px"
        />
      </div>

      <template #footer>
        <el-button @click="closeDeployProgress">{{
          deployResult ? "关闭" : "后台继续安装"
        }}</el-button>
      </template>
    </el-dialog>

    <!-- 部署到现有节点对话框 -->
    <el-dialog
      v-model="deployToNodeDialogVisible"
      title="部署 Agent 到节点"
      width="600px"
    >
      <el-alert type="info" :closable="false" style="margin-bottom: 20px">
        <template #title>
          将 Agent 部署到节点: {{ currentNode?.name }}
        </template>
      </el-alert>

      <el-form
        :model="deployForm"
        :rules="deployRules"
        ref="deployFormRef"
        label-width="100px"
      >
        <el-form-item label="服务器地址" prop="host">
          <el-input v-model="deployForm.host" placeholder="IP 地址或域名" />
          <span class="form-tip">通常与节点地址相同</span>
        </el-form-item>

        <el-form-item label="SSH 端口" prop="port">
          <el-input-number
            v-model="deployForm.port"
            :min="1"
            :max="65535"
            style="width: 100%"
          />
        </el-form-item>

        <el-form-item label="用户名" prop="username">
          <el-input v-model="deployForm.username" placeholder="SSH 用户名" />
        </el-form-item>

        <el-form-item label="认证方式" prop="authMethod">
          <el-radio-group v-model="deployForm.authMethod">
            <el-radio label="password">密码</el-radio>
            <el-radio label="key">私钥</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item
          v-if="deployForm.authMethod === 'password'"
          label="密码"
          prop="password"
        >
          <el-input
            v-model="deployForm.password"
            type="password"
            placeholder="SSH 密码"
            show-password
          />
        </el-form-item>

        <el-form-item
          v-if="deployForm.authMethod === 'key'"
          label="私钥"
          prop="privateKey"
        >
          <el-input
            v-model="deployForm.privateKey"
            type="textarea"
            :rows="6"
            placeholder="粘贴 SSH 私钥内容"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="deployToNodeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="deploying" @click="submitDeploy">
          开始部署
        </el-button>
      </template>
    </el-dialog>

    <!-- 节点详情对话框 -->
    <el-dialog v-model="detailDialogVisible" title="节点详情" width="700px">
      <div v-if="currentNode" class="node-detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="ID">{{
            currentNode.id
          }}</el-descriptions-item>
          <el-descriptions-item label="名称">{{
            currentNode.name
          }}</el-descriptions-item>
          <el-descriptions-item label="地址"
            >{{ currentNode.address }}:{{
              currentNode.port
            }}</el-descriptions-item
          >
          <el-descriptions-item label="状态">
            <el-tag :type="getStatusType(currentNode.status)">{{
              getStatusText(currentNode.status)
            }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="地区">{{
            currentNode.region || "-"
          }}</el-descriptions-item>
          <el-descriptions-item label="权重">{{
            currentNode.weight
          }}</el-descriptions-item>
          <el-descriptions-item label="当前用户">{{
            currentNode.current_users
          }}</el-descriptions-item>
          <el-descriptions-item label="最大用户">{{
            currentNode.max_users || "无限制"
          }}</el-descriptions-item>
          <el-descriptions-item label="延迟"
            >{{ currentNode.latency }}ms</el-descriptions-item
          >
          <el-descriptions-item label="同步状态">
            <el-tag :type="getSyncStatusType(currentNode.sync_status)">{{
              getSyncStatusText(currentNode.sync_status)
            }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="最后在线">{{
            formatTime(currentNode.last_seen_at)
          }}</el-descriptions-item>
          <el-descriptions-item label="最后同步">{{
            formatTime(currentNode.synced_at)
          }}</el-descriptions-item>
          <el-descriptions-item label="创建时间" :span="2">{{
            formatTime(currentNode.created_at)
          }}</el-descriptions-item>
        </el-descriptions>
        <div
          v-if="currentNode.tags && currentNode.tags.length"
          class="detail-tags"
        >
          <span class="tags-label">标签：</span>
          <el-tag
            v-for="tag in parseTags(currentNode.tags)"
            :key="tag"
            size="small"
            style="margin-right: 8px"
          >
            {{ tag }}
          </el-tag>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  Plus,
  Refresh,
  Search,
  Monitor,
  CircleCheck,
  User,
  Timer,
  View,
  Hide,
  CopyDocument,
  Upload,
  Connection,
} from "@element-plus/icons-vue";
import { useNodeStore } from "@/stores/node";
import { certificatesApi } from "@/api";
import { nodesApi } from "@/api/modules/nodes";

const nodeStore = useNodeStore();

const pagination = reactive({ page: 1, pageSize: 20 });
const dialogVisible = ref(false);
const tokenDialogVisible = ref(false);
const detailDialogVisible = ref(false);
const deployToNodeDialogVisible = ref(false);
const deployProgressDialogVisible = ref(false);
const isEdit = ref(false);
const submitting = ref(false);
const tokenLoading = ref(false);
const deploying = ref(false);
const testingConnection = ref(false);
const formRef = ref(null);
const deployFormRef = ref(null);
const showTagInput = ref(false);
const newTag = ref("");
const currentNode = ref(null);
const currentToken = ref("");
const newToken = ref("");
const showToken = ref(false);
const connectionTestResult = ref(null);
const deploySteps = ref([]);
const deployStepActive = ref(0);
const deployLogs = ref("");
const deployResult = ref(null);
const deployStatusMessage = ref("");
const certificates = ref([]);
const certificatesLoading = ref(false);

let installStatusTimer = null;
let shouldPollInstallStatus = false;

const form = reactive({
  id: null,
  name: "",
  address: "",
  port: 18443,
  region: "",
  weight: 1,
  max_users: 0,
  tags: [],
  ip_whitelist_str: "",
  tls_enabled: false,
  tls_domain: "",
  certificate_id: null,
  // 安装方式
  installMethod: "manual",
  // Panel URL
  panel_url: "",
  // SSH 配置
  ssh_host: "",
  ssh_port: 22,
  ssh_username: "root",
  ssh_auth_method: "password",
  ssh_password: "",
  ssh_private_key: "",
});

const deployForm = reactive({
  host: "",
  port: 22,
  username: "root",
  authMethod: "password",
  password: "",
  privateKey: "",
});

const rules = {
  name: [{ required: true, message: "请输入节点名称", trigger: "blur" }],
  panel_url: [
    {
      validator: (rule, value, callback) => {
        if (form.installMethod === "auto" && !value) {
          callback(new Error("请输入 Panel 地址"));
        } else if (value && !value.match(/^https?:\/\/.+/)) {
          callback(
            new Error("Panel 地址格式不正确，应以 http:// 或 https:// 开头"),
          );
        } else if (
          value &&
          (value.includes("localhost") || value.includes("127.0.0.1"))
        ) {
          callback(
            new Error(
              "Panel 地址不能使用 localhost 或 127.0.0.1，请使用服务器的实际 IP 或域名",
            ),
          );
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
  ssh_host: [
    {
      validator: (rule, value, callback) => {
        if (form.installMethod === "auto" && !value) {
          callback(new Error("请输入服务器地址"));
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
  ssh_username: [
    {
      validator: (rule, value, callback) => {
        if (form.installMethod === "auto" && !value) {
          callback(new Error("请输入用户名"));
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
  ssh_password: [
    {
      validator: (rule, value, callback) => {
        if (
          form.installMethod === "auto" &&
          form.ssh_auth_method === "password" &&
          !value
        ) {
          callback(new Error("请输入密码"));
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
  ssh_private_key: [
    {
      validator: (rule, value, callback) => {
        if (
          form.installMethod === "auto" &&
          form.ssh_auth_method === "key" &&
          !value
        ) {
          callback(new Error("请输入私钥"));
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
  tls_domain: [{ validator: validateTLSDomain, trigger: "blur" }],
};

const deployRules = {
  host: [{ required: true, message: "请输入服务器地址", trigger: "blur" }],
  username: [{ required: true, message: "请输入用户名", trigger: "blur" }],
  password: [
    {
      validator: (rule, value, callback) => {
        if (deployForm.authMethod === "password" && !value) {
          callback(new Error("请输入密码"));
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
  privateKey: [
    {
      validator: (rule, value, callback) => {
        if (deployForm.authMethod === "key" && !value) {
          callback(new Error("请输入私钥"));
        } else {
          callback();
        }
      },
      trigger: "blur",
    },
  ],
};

const normalizeText = (value) =>
  typeof value === "string" ? value.trim() : "";

const normalizeCertificateDomain = (domain) =>
  normalizeText(domain).replace(/^\*\./, "").toLowerCase();

const formatCertificateDate = (value) => {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toISOString().slice(0, 10);
};

const normalizeCertificatesResponse = (response) => {
  if (Array.isArray(response)) return response;
  if (Array.isArray(response?.certificates)) return response.certificates;
  if (Array.isArray(response?.data?.certificates)) return response.data.certificates;
  if (Array.isArray(response?.data)) return response.data;
  return [];
};

const selectedCertificate = computed(
  () =>
    certificates.value.find(
      (cert) => Number(cert.id) === Number(form.certificate_id),
    ) || null,
);

const fetchCertificates = async () => {
  certificatesLoading.value = true;
  try {
    const response = await certificatesApi.list();
    certificates.value = normalizeCertificatesResponse(response).map((cert) => ({
      ...cert,
      expireDate: formatCertificateDate(cert.expires_at || cert.expiresAt),
    }));
  } catch (e) {
    console.error("获取证书失败:", e);
    certificates.value = [];
  } finally {
    certificatesLoading.value = false;
  }
};

const getCertificateOptionLabel = (cert) => {
  if (!cert) return "";
  return cert.expireDate && cert.expireDate !== "-"
    ? `${cert.domain}（到期 ${cert.expireDate}）`
    : cert.domain;
};

const getAssignedCertificateDisplay = (certificateId) => {
  const certificate = certificates.value.find(
    (cert) => Number(cert.id) === Number(certificateId),
  );
  return certificate?.domain || `#${certificateId}`;
};

const handleCertificateChange = (certificateId) => {
  if (!certificateId) return;
  const certificate = certificates.value.find(
    (cert) => Number(cert.id) === Number(certificateId),
  );
  if (!certificate) return;

  const suggestedDomain = normalizeCertificateDomain(certificate.domain);
  form.tls_enabled = true;
  if (suggestedDomain) {
    form.tls_domain = suggestedDomain;
  }
};

function validateTLSDomain(rule, value, callback) {
  const domain = normalizeText(value).toLowerCase();
  const domainRegex =
    /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$/;

  if (form.tls_enabled && !domain) {
    callback(new Error("启用 TLS 时请输入 TLS 域名"));
    return;
  }

  if (domain && !domainRegex.test(domain)) {
    callback(new Error("请输入有效的 TLS 域名"));
    return;
  }

  callback();
}

const regions = computed(() => {
  const regionSet = new Set(
    nodeStore.nodes.map((n) => n.region).filter(Boolean),
  );
  return Array.from(regionSet);
});

const getStatusType = (status) => {
  const types = { online: "success", offline: "info", unhealthy: "danger" };
  return types[status] || "info";
};

const getStatusText = (status) => {
  const texts = { online: "在线", offline: "离线", unhealthy: "不健康" };
  return texts[status] || status;
};

const getSyncStatusType = (status) => {
  const types = { synced: "success", pending: "warning", failed: "danger" };
  return types[status] || "info";
};

const getSyncStatusText = (status) => {
  const texts = { synced: "已同步", pending: "待同步", failed: "同步失败" };
  return texts[status] || status;
};

const getInstallStatusType = (status) => {
  const types = {
    idle: "info",
    queued: "warning",
    running: "warning",
    success: "success",
    failed: "danger",
  };
  return types[status] || "info";
};

const getInstallStatusText = (status) => {
  const texts = {
    idle: "未安装",
    queued: "排队中",
    running: "安装中",
    success: "已完成",
    failed: "已失败",
  };
  return texts[status] || "未安装";
};

const getLoadStatus = (node) => {
  if (!node.max_users) return "";
  const ratio = node.current_users / node.max_users;
  if (ratio >= 0.9) return "exception";
  if (ratio >= 0.7) return "warning";
  return "success";
};

const getLatencyClass = (latency) => {
  if (latency < 100) return "latency-good";
  if (latency < 300) return "latency-medium";
  return "latency-bad";
};

// 获取步骤状态（用于 el-steps 组件）
const getStepStatus = (step, index) => {
  // 如果 step 是对象（新格式），直接返回状态
  if (typeof step === "object" && step.status) {
    const statusMap = {
      success: "finish",
      failed: "error",
      running: "process",
      pending: "wait",
    };
    return statusMap[step.status] || "wait";
  }

  // 兼容旧格式（纯字符串）
  if (index < deployStepActive.value) {
    return "finish";
  } else if (index === deployStepActive.value) {
    return deployResult.value?.success === false ? "error" : "process";
  }
  return "wait";
};

const clearInstallStatusPolling = () => {
  shouldPollInstallStatus = false;
  if (installStatusTimer) {
    clearTimeout(installStatusTimer);
    installStatusTimer = null;
  }
};

const resetDeployProgress = () => {
  clearInstallStatusPolling();
  deploySteps.value = [];
  deployStepActive.value = 0;
  deployLogs.value = "";
  deployResult.value = null;
  deployStatusMessage.value = "";
};

const applyInstallStatus = (status) => {
  deployStatusMessage.value = status?.message || "";
  deploySteps.value = status?.steps || [];
  deployLogs.value = status?.logs || "";

  deployStepActive.value = deploySteps.value.findIndex(
    (step) =>
      (typeof step === "object" && step.status === "running") ||
      (typeof step === "object" && step.status === "failed"),
  );

  if (deployStepActive.value === -1 && deploySteps.value.length > 0) {
    deployStepActive.value =
      status?.status === "success"
        ? deploySteps.value.length
        : Math.max(0, deploySteps.value.length - 1);
  }

  if (status?.status === "success" || status?.status === "failed") {
    deployResult.value = {
      success: status.status === "success",
      message:
        status.message ||
        (status.status === "success" ? "Agent 部署成功" : "Agent 部署失败"),
    };
  } else {
    deployResult.value = null;
  }
};

const pollInstallStatus = async (nodeId) => {
  clearInstallStatusPolling();
  shouldPollInstallStatus = true;

  const run = async () => {
    try {
      const status = await nodesApi.getInstallStatus(nodeId);
      applyInstallStatus(status);

      if (
        shouldPollInstallStatus &&
        (status?.status === "queued" || status?.status === "running")
      ) {
        installStatusTimer = setTimeout(run, 2000);
        return;
      }

      clearInstallStatusPolling();
      if (status?.status === "success" || status?.status === "failed") {
        fetchNodes();
      }
    } catch (e) {
      clearInstallStatusPolling();
      deployResult.value = {
        success: false,
        message: e.message || "获取安装状态失败",
      };
    }
  };

  await run();
};

const formatTime = (time) => {
  if (!time) return "-";
  return new Date(time).toLocaleString("zh-CN");
};

const parseTags = (tags) => {
  if (Array.isArray(tags)) return tags;
  if (typeof tags === "string") {
    try {
      return JSON.parse(tags);
    } catch {
      return [];
    }
  }
  return [];
};

const fetchNodes = async () => {
  try {
    await nodeStore.fetchNodes({
      limit: pagination.pageSize,
      offset: (pagination.page - 1) * pagination.pageSize,
      status: nodeStore.filters.status || undefined,
      region: nodeStore.filters.region || undefined,
    });
  } catch (e) {
    ElMessage.error(e.message || "获取节点列表失败");
  }
};

const showCreateDialog = () => {
  isEdit.value = false;

  // 自动检测当前访问的 Panel URL
  let currentUrl = window.location.origin;

  // 如果检测到 localhost，尝试获取实际 IP
  if (currentUrl.includes("localhost") || currentUrl.includes("127.0.0.1")) {
    // 提示用户需要手动输入实际地址
    currentUrl = "";
    ElMessage.warning(
      "检测到使用 localhost 访问，请手动输入 Panel 服务器的实际 IP 或域名",
    );
  }

  Object.assign(form, {
    id: null,
    name: "",
    address: "",
    port: 18443,
    region: "",
    weight: 1,
    max_users: 0,
    tags: [],
    ip_whitelist_str: "",
    tls_enabled: false,
    tls_domain: "",
    certificate_id: null,
    installMethod: "manual",
    panel_url: currentUrl, // 自动填充当前访问地址
    ssh_host: "",
    ssh_port: 22,
    ssh_username: "root",
    ssh_auth_method: "password",
    ssh_password: "",
    ssh_private_key: "",
  });
  connectionTestResult.value = null;
  dialogVisible.value = true;
};

const editNode = (node) => {
  isEdit.value = true;
  const tags = parseTags(node.tags);
  const ipWhitelist = node.ip_whitelist
    ? (Array.isArray(node.ip_whitelist) ? node.ip_whitelist : []).join("\n")
    : "";
  Object.assign(form, {
    id: node.id,
    name: node.name,
    address: node.address,
    port: node.port,
    region: node.region || "",
    weight: node.weight || 1,
    max_users: node.max_users || 0,
    tags: tags,
    ip_whitelist_str: ipWhitelist,
    tls_enabled: Boolean(node.tls_enabled),
    tls_domain: node.tls_domain || "",
    certificate_id: node.certificate_id ?? null,
  });
  dialogVisible.value = true;
};

const submitForm = async () => {
  await formRef.value.validate();

  if (form.tls_enabled && !normalizeText(form.tls_domain)) {
    ElMessage.error("启用 TLS 时请输入 TLS 域名");
    return;
  }

  submitting.value = true;

  try {
    const ipWhitelist = form.ip_whitelist_str
      .split("\n")
      .map((s) => s.trim())
      .filter(Boolean);
    const data = {
      name: form.name,
      region: form.region,
      weight: form.weight,
      max_users: form.max_users,
      tags: form.tags,
      ip_whitelist: ipWhitelist,
      tls_enabled: form.tls_enabled,
      tls_domain: normalizeText(form.tls_domain).toLowerCase(),
      certificate_id: form.certificate_id || null,
    };

    // 如果是编辑模式
    if (isEdit.value) {
      // 编辑时需要地址和端口
      data.address = form.address;
      data.port = form.port;
      await nodeStore.updateNode(form.id, data);
      ElMessage.success("更新成功");
      dialogVisible.value = false;
      fetchNodes();
      return;
    }

    // 创建模式 - 如果选择自动安装
    if (form.installMethod === "auto") {
      // 使用 SSH 地址作为节点地址
      data.address = form.ssh_host;
      data.port = 18443; // Agent 默认端口

      // 添加 SSH 配置
      data.ssh = {
        host: form.ssh_host,
        port: form.ssh_port,
        username: form.ssh_username,
        panel_url: form.panel_url, // 使用用户指定的 Panel URL
      };

      if (form.ssh_auth_method === "password") {
        data.ssh.password = form.ssh_password;
      } else {
        data.ssh.private_key = form.ssh_private_key;
      }

      dialogVisible.value = false;

      try {
        const result = await nodeStore.createNode(data);

        if (result.installing) {
          deployProgressDialogVisible.value = true;
          resetDeployProgress();
          deployStatusMessage.value =
            result.message || "节点创建成功，后台自动安装已开始";
          ElMessage.success(deployStatusMessage.value);
          fetchNodes();
          await pollInstallStatus(result.id);
        } else if (result.install_result) {
          deployProgressDialogVisible.value = true;
          deploySteps.value = result.install_result.steps || [];
          deployStepActive.value = deploySteps.value.findIndex(
            (s) =>
              (typeof s === "object" && s.status === "running") ||
              (typeof s === "object" && s.status === "failed"),
          );
          if (deployStepActive.value === -1) {
            deployStepActive.value = result.install_result.success
              ? deploySteps.value.length
              : 0;
          }
          deployLogs.value = result.install_result.logs || "";
          deployResult.value = result.install_result;

          if (result.install_result.success) {
            ElMessage.success("节点创建并安装成功");
            fetchNodes();
          } else {
            ElMessage.error(result.install_result.message || "安装失败");
          }
        } else {
          ElMessage.success("节点创建成功");
          fetchNodes();
        }
      } catch (e) {
        ElMessage.error(e.message || "创建失败");
      }
    } else {
      // 手动安装模式 - 需要用户提供地址和端口
      if (!form.address) {
        ElMessage.warning("手动安装模式需要填写节点地址");
        return;
      }
      data.address = form.address;
      data.port = form.port;

      await nodeStore.createNode(data);
      ElMessage.success("节点创建成功，请手动安装 Agent");
      dialogVisible.value = false;
      fetchNodes();
    }
  } catch (e) {
    ElMessage.error(e.message || "操作失败");
  } finally {
    submitting.value = false;
  }
};

const deleteNode = async (node) => {
  await ElMessageBox.confirm(
    `确定要删除节点 "${node.name}" 吗？该节点上的用户将被重新分配到其他节点。`,
    "删除确认",
    { type: "warning" },
  );
  try {
    await nodeStore.deleteNode(node.id);
    ElMessage.success("删除成功");
    fetchNodes();
  } catch (e) {
    ElMessage.error(e.message || "删除失败");
  }
};

const viewNodeDetail = async (node) => {
  currentNode.value = node;
  detailDialogVisible.value = true;
};

const showTokenDialog = (node) => {
  currentNode.value = node;
  currentToken.value = "";
  newToken.value = "";
  showToken.value = false;
  tokenDialogVisible.value = true;
};

const handleGenerateToken = async () => {
  tokenLoading.value = true;
  try {
    const res = await nodeStore.generateToken(currentNode.value.id);
    newToken.value = res.token;
    currentToken.value = res.token;
    ElMessage.success("Token 生成成功");
  } catch (e) {
    ElMessage.error(e.message || "生成 Token 失败");
  } finally {
    tokenLoading.value = false;
  }
};

const handleRotateToken = async () => {
  await ElMessageBox.confirm(
    "轮换 Token 后，旧 Token 将立即失效，确定继续？",
    "确认",
    { type: "warning" },
  );
  tokenLoading.value = true;
  try {
    const res = await nodeStore.rotateToken(currentNode.value.id);
    newToken.value = res.token;
    currentToken.value = res.token;
    ElMessage.success("Token 轮换成功");
  } catch (e) {
    ElMessage.error(e.message || "轮换 Token 失败");
  } finally {
    tokenLoading.value = false;
  }
};

const handleRevokeToken = async () => {
  await ElMessageBox.confirm(
    "撤销 Token 后，节点将无法连接，确定继续？",
    "确认",
    { type: "warning" },
  );
  tokenLoading.value = true;
  try {
    await nodeStore.revokeToken(currentNode.value.id);
    currentToken.value = "";
    newToken.value = "";
    ElMessage.success("Token 已撤销");
  } catch (e) {
    ElMessage.error(e.message || "撤销 Token 失败");
  } finally {
    tokenLoading.value = false;
  }
};

const copyToken = async () => {
  try {
    await navigator.clipboard.writeText(currentToken.value);
    ElMessage.success("已复制到剪贴板");
  } catch {
    ElMessage.error("复制失败");
  }
};

const addTag = () => {
  if (newTag.value.trim() && !form.tags.includes(newTag.value.trim())) {
    form.tags.push(newTag.value.trim());
  }
  newTag.value = "";
  showTagInput.value = false;
};

const removeTag = (index) => {
  form.tags.splice(index, 1);
};

const showDeployDialog = (node) => {
  currentNode.value = node;
  Object.assign(deployForm, {
    host: node.address,
    port: 22,
    username: "root",
    authMethod: "password",
    password: "",
    privateKey: "",
  });
  deployToNodeDialogVisible.value = true;
};

const viewInstallStatus = async (node) => {
  currentNode.value = node;
  deployProgressDialogVisible.value = true;
  resetDeployProgress();

  try {
    const status = await nodesApi.getInstallStatus(node.id);
    applyInstallStatus(status);
    if (status?.status === "queued" || status?.status === "running") {
      await pollInstallStatus(node.id);
    }
  } catch (e) {
    deployResult.value = {
      success: false,
      message: e.message || "获取安装状态失败",
    };
  }
};

const submitDeploy = async () => {
  try {
    await deployFormRef.value.validate();
  } catch {
    return;
  }

  deploying.value = true;

  try {
    const deployData = {
      host: deployForm.host,
      port: deployForm.port,
      username: deployForm.username,
    };

    if (deployForm.authMethod === "password") {
      deployData.password = deployForm.password;
    } else {
      deployData.private_key = deployForm.privateKey;
    }

    // 显示部署进度对话框
    deployToNodeDialogVisible.value = false;
    deployProgressDialogVisible.value = true;
    resetDeployProgress();

    // 开始部署
    try {
      const result = await nodesApi.deployAgent(
        currentNode.value.id,
        deployData,
      );

      deploySteps.value = result.steps || [];
      // 计算激活的步骤索引（找到最后一个非 pending 的步骤）
      deployStepActive.value = deploySteps.value.findIndex(
        (s) =>
          (typeof s === "object" && s.status === "running") ||
          (typeof s === "object" && s.status === "failed"),
      );
      if (deployStepActive.value === -1) {
        deployStepActive.value = result.success ? deploySteps.value.length : 0;
      }
      deployLogs.value = result.logs || "";
      deployResult.value = result;

      if (result.success) {
        ElMessage.success("Agent 部署成功");
        fetchNodes();
      } else {
        ElMessage.error(result.message || "Agent 部署失败");
      }
    } catch (deployError) {
      // 处理部署 API 错误
      let errorMessage = "Agent 部署失败";
      let errorLogs = "";
      let errorSteps = [];

      // 尝试从多个位置获取错误详情
      // 注意：base.js 的响应拦截器会将原始响应数据保存在 deployError.response.data 中
      const responseData = deployError.response?.data;

      if (responseData?.steps || responseData?.logs) {
        // 从保留的原始响应数据中获取部署详情
        errorSteps = responseData.steps || [];
        errorLogs = responseData.logs || "";
        errorMessage =
          responseData.message || deployError.message || "Agent 部署失败";
      } else if (deployError.details?.steps || deployError.details?.logs) {
        // 从 details 字段获取（备用）
        errorSteps = deployError.details.steps || [];
        errorLogs = deployError.details.logs || "";
        errorMessage = deployError.message || "Agent 部署失败";
      } else if (deployError.message) {
        // 网络错误或其他错误
        errorMessage = deployError.message;
        errorLogs = `错误详情:\n${deployError.message}\n\n错误ID: ${deployError.errorId || "N/A"}`;
      }

      deploySteps.value = errorSteps;
      // 计算激活的步骤索引
      deployStepActive.value = deploySteps.value.findIndex(
        (s) => typeof s === "object" && s.status === "failed",
      );
      if (deployStepActive.value === -1 && deploySteps.value.length > 0) {
        deployStepActive.value = Math.max(0, deploySteps.value.length - 1);
      }

      deployLogs.value = errorLogs;
      deployResult.value = {
        success: false,
        message: errorMessage,
      };

      // 如果有详细日志，不重复显示通用错误消息
      if (!errorLogs && !deployError.errorId) {
        ElMessage.error(errorMessage);
      }
    }
  } catch (e) {
    ElMessage.error(e.message || "部署失败");
    deployProgressDialogVisible.value = false;
  } finally {
    deploying.value = false;
  }
};

const closeDeployProgress = () => {
  deployProgressDialogVisible.value = false;
  resetDeployProgress();
};

const copyDeployLogs = async () => {
  try {
    await navigator.clipboard.writeText(deployLogs.value);
    ElMessage.success("已复制到剪贴板");
  } catch {
    ElMessage.error("复制失败");
  }
};

const downloadDeployScript = async (node) => {
  try {
    const blob = await nodesApi.getDeployScript(node.id);

    // 创建下载链接
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `install-agent-${node.name}.sh`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);

    ElMessage.success("部署脚本已下载");
  } catch (e) {
    ElMessage.error(e.message || "下载失败");
  }
};

const testSSHConnection = async () => {
  // 验证必填字段
  if (!form.ssh_host) {
    ElMessage.error("请输入服务器地址");
    return;
  }
  if (form.ssh_auth_method === "password" && !form.ssh_password) {
    ElMessage.error("请输入 SSH 密码");
    return;
  }
  if (form.ssh_auth_method === "key" && !form.ssh_private_key) {
    ElMessage.error("请输入 SSH 私钥");
    return;
  }

  testingConnection.value = true;
  connectionTestResult.value = null;

  try {
    const res = await nodesApi.testConnection({
      host: form.ssh_host,
      port: form.ssh_port,
      username: form.ssh_username,
      password: form.ssh_auth_method === "password" ? form.ssh_password : "",
      private_key: form.ssh_auth_method === "key" ? form.ssh_private_key : "",
    });

    connectionTestResult.value = {
      success: res.success,
      message: res.success ? "SSH 连接测试成功" : res.message || "SSH 连接失败",
    };

    if (res.success) {
      ElMessage.success("SSH 连接测试成功");
    } else {
      ElMessage.error(res.message || "SSH 连接失败");
    }
  } catch (e) {
    connectionTestResult.value = {
      success: false,
      message: e.message || "SSH 连接测试失败",
    };
    ElMessage.error(e.message || "SSH 连接测试失败");
  } finally {
    testingConnection.value = false;
  }
};

onMounted(async () => {
  await Promise.all([fetchNodes(), fetchCertificates()]);
});

onUnmounted(() => {
  clearInstallStatusPolling();
});
</script>

<style scoped>
.admin-nodes-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  position: relative;
  overflow: hidden;
}

.stat-card .stat-content {
  position: relative;
  z-index: 1;
}

.stat-card .stat-value {
  font-size: 28px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.stat-card .stat-label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.stat-card .stat-icon {
  position: absolute;
  right: 20px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 48px;
  color: var(--el-color-primary-light-7);
  opacity: 0.6;
}

.stat-card.online .stat-value {
  color: var(--el-color-success);
}

.stat-card.online .stat-icon {
  color: var(--el-color-success-light-5);
}

.filter-card {
  margin-bottom: 20px;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.load-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.latency-good {
  color: var(--el-color-success);
}
.latency-medium {
  color: var(--el-color-warning);
}
.latency-bad {
  color: var(--el-color-danger);
}

.form-tip {
  margin-left: 12px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.token-dialog-content {
  padding: 10px 0;
}

.token-info {
  display: flex;
  margin-bottom: 16px;
}

.token-label {
  width: 100px;
  color: var(--el-text-color-secondary);
}

.token-value {
  flex: 1;
}

.token-text {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: monospace;
}

.token-actions {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}

.new-token-alert {
  margin-top: 20px;
}

.new-token-text {
  font-family: monospace;
  word-break: break-all;
  margin-top: 8px;
  padding: 8px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
}

.node-detail {
  padding: 10px 0;
}

.detail-tags {
  margin-top: 16px;
  display: flex;
  align-items: center;
}

.tags-label {
  margin-right: 12px;
  color: var(--el-text-color-secondary);
}

.deploy-progress {
  padding: 20px 0;
}

.deploy-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  color: var(--el-text-color-secondary);
}

.deploy-logs {
  margin-top: 30px;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color);
  font-weight: 500;
}

.logs-content {
  padding: 16px;
  margin: 0;
  max-height: 400px;
  overflow-y: auto;
  background: var(--el-bg-color);
  font-family: "Courier New", Courier, monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.form-tip-inline {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 6px;
  line-height: 1.5;
}

.certificate-tip {
  margin-top: 8px;
  font-size: 12px;
  color: var(--el-color-primary);
}

.tls-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tls-domain-text,
.tls-cert-text {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.4;
}
</style>
