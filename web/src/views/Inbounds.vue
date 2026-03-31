<template>
  <div class="inbounds-page">
    <div class="page-header">
      <div class="page-heading">
        <div class="title">
          代理服务
        </div>
        <div class="page-subtitle">
          更清晰地查看协议、节点、有效期和分享信息
        </div>
      </div>
      <el-button
        type="primary"
        class="add-btn"
        @click="openAddInboundDialog"
      >
        <el-icon class="el-icon--left">
          <Plus />
        </el-icon> 添加代理
      </el-button>
    </div>

    <div class="overview-strip">
      <div class="overview-card">
        <span class="overview-label">当前页</span>
        <strong class="overview-value">{{ paginatedInbounds.length }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">本页运行中</span>
        <strong class="overview-value is-success">{{ enabledInboundCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">本页已停止</span>
        <strong class="overview-value is-danger">{{ disabledInboundCount }}</strong>
      </div>
      <div class="overview-card">
        <span class="overview-label">总记录</span>
        <strong class="overview-value">{{ total }}</strong>
      </div>
    </div>

    <div class="toolbar-card">
      <div class="toolbar-filters">
        <el-input
          v-model="filters.keyword"
          clearable
          class="toolbar-search"
          placeholder="搜索名称、节点、地址或端口"
        />
        <el-select
          v-model="filters.protocol"
          clearable
          placeholder="协议类型"
        >
          <el-option
            v-for="option in protocolOptions"
            :key="option.value"
            :label="option.label"
            :value="option.value"
          />
        </el-select>
        <el-select
          v-model="filters.status"
          clearable
          placeholder="运行状态"
        >
          <el-option
            label="运行中"
            value="enabled"
          />
          <el-option
            label="已停止"
            value="disabled"
          />
        </el-select>
        <el-button @click="resetFilters">
          重置
        </el-button>
        <el-button @click="loadInbounds">
          刷新
        </el-button>
      </div>
      <div class="toolbar-actions">
        <span class="toolbar-summary">
          匹配 {{ filteredInbounds.length }} 条，已选 {{ selectedInboundIds.length }} 条
        </span>
        <el-button
          class="toolbar-batch-btn toolbar-batch-btn--success"
          :disabled="!selectedInboundIds.length"
          :loading="batchLoading === 'enable'"
          @click="runBatchOperation('enable')"
        >
          批量启用
        </el-button>
        <el-button
          class="toolbar-batch-btn toolbar-batch-btn--warning"
          :disabled="!selectedInboundIds.length"
          :loading="batchLoading === 'disable'"
          @click="runBatchOperation('disable')"
        >
          批量停用
        </el-button>
        <el-button
          class="toolbar-batch-btn toolbar-batch-btn--danger"
          :disabled="!selectedInboundIds.length"
          :loading="batchLoading === 'delete'"
          @click="runBatchOperation('delete')"
        >
          批量删除
        </el-button>
      </div>
    </div>

    <el-table
      ref="inboundsTableRef"
      v-loading="loading"
      :data="paginatedInbounds"
      border
      stripe
      class="inbounds-table"
      row-key="id"
      :empty-text="filteredInbounds.length ? '当前页暂无数据' : (hasActiveFilters ? '暂无匹配的代理服务' : '暂无代理服务')"
      @selection-change="handleSelectionChange"
    >
      <el-table-column
        type="selection"
        width="48"
        reserve-selection
      />
      <el-table-column
        label="代理服务"
        min-width="320"
      >
        <template #default="{ row }">
          <div class="service-cell">
            <div class="service-cell__header">
              <span
                class="service-name"
                :title="getInboundName(row)"
              >{{ getInboundName(row) }}</span>
              <span :class="['protocol-tag', normalizeProtocol(row.protocol)]">
                {{ formatProtocolLabel(row.protocol) }}
              </span>
            </div>
            <div class="service-cell__meta">
              <span>节点：{{ getInboundNodeLabel(row) }}</span>
              <span>端口：{{ row.port || '-' }}</span>
              <span>用户：{{ getInboundClientCount(row) }}</span>
            </div>
            <div
              v-if="getInboundServerDisplay(row)"
              class="service-cell__server"
              :title="getInboundServerDisplay(row)"
            >
              分享地址：{{ getInboundServerDisplay(row) }}
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column
        label="生命周期"
        min-width="240"
      >
        <template #default="{ row }">
          <div class="detail-cell">
            <div class="detail-item">
              <span class="detail-label">创建时间</span>
              <span class="detail-value">{{ row.created_at_display || '-' }}</span>
            </div>
            <div class="detail-item">
              <span class="detail-label">到期时间</span>
              <span class="detail-value">{{ row.expires_at_display || '未设置' }}</span>
            </div>
            <div
              v-if="row.expiry_source_label"
              class="detail-tags"
            >
              <el-tag
                size="small"
                effect="plain"
              >
                {{ row.expiry_source_label }}
              </el-tag>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column
        label="限制与状态"
        min-width="220"
      >
        <template #default="{ row }">
          <div class="detail-cell">
            <div class="detail-item">
              <span class="detail-label">流量限制</span>
              <span class="detail-value">{{ row.traffic_limit_display || '不限制' }}</span>
            </div>
            <div class="detail-item detail-item--status">
              <span class="detail-label">运行状态</span>
              <span :class="['status-tag', row.enable ? 'running' : 'stopped']">
                {{ row.enable ? '运行中' : '已停止' }}
              </span>
            </div>
            <div
              v-if="row.traffic_limit_source_label"
              class="detail-tags"
            >
              <el-tag
                size="small"
                effect="plain"
                type="warning"
              >
                {{ row.traffic_limit_source_label }}
              </el-tag>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column
        label="操作"
        width="190"
        align="right"
      >
        <template #default="{ row }">
          <div class="operation-btns">
            <el-button
              size="small"
              class="row-action row-action--primary"
              @click="copyLink(row)"
            >
              链接
            </el-button>

            <el-button
              size="small"
              class="row-action"
              :class="row.enable ? 'row-action--warning' : 'row-action--success'"
              @click="toggleStatus(row)"
            >
              {{ row.enable ? '停用' : '启用' }}
            </el-button>

            <el-dropdown
              trigger="click"
              @command="(command) => handleRowCommand(command, row)"
            >
              <el-button
                size="small"
                class="row-action row-action--more"
                circle
                title="更多操作"
              >
                <el-icon><MoreFilled /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="qr">
                    二维码
                  </el-dropdown-item>
                  <el-dropdown-item command="edit">
                    编辑
                  </el-dropdown-item>
                  <el-dropdown-item
                    command="delete"
                    divided
                  >
                    删除
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页控件 -->
    <div class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        :total="filteredInbounds.length"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>

    <!-- 添加入站对话框 -->
    <el-dialog
      v-model="addInboundDialogVisible"
      :title="dialogMode === 'edit' ? '编辑协议' : '添加协议'"
      width="560px"
      destroy-on-close
      :close-on-click-modal="false"
    >
      <el-form
        ref="inboundFormRef"
        v-loading="dialogLoading"
        :model="inboundForm"
        :rules="rules"
        label-width="100px"
        label-position="left"
      >
        <el-form-item
          label="协议"
          prop="protocol"
        >
          <el-select
            v-model="inboundForm.protocol"
            style="width: 100%"
            :disabled="dialogMode === 'edit'"
          >
            <el-option
              label="VMess"
              value="vmess"
            />
            <el-option
              label="VLESS"
              value="vless"
            />
            <el-option
              label="Trojan"
              value="trojan"
            />
            <el-option
              label="Shadowsocks"
              value="shadowsocks"
            />
            <el-option
              label="Dokodemo-Door"
              value="dokodemo-door"
            />
          </el-select>
        </el-form-item>
        
        <el-form-item
          label="备注"
          prop="remark"
        >
          <el-input
            v-model="inboundForm.remark"
            placeholder="请输入备注"
          />
        </el-form-item>
        
        <el-form-item
          label="部署节点"
          prop="node_id"
        >
          <el-select 
            v-model="inboundForm.node_id" 
            placeholder="选择节点（可选）" 
            clearable
            style="width: 100%"
          >
            <el-option 
              v-for="node in nodeList" 
              :key="node.id" 
              :label="node.name" 
              :value="node.id"
            >
              <span>{{ node.name }}</span>
              <span style="float: right; color: var(--el-text-color-secondary); font-size: 13px">
                {{ node.address }}
              </span>
            </el-option>
          </el-select>
          <div style="color: var(--el-text-color-secondary); font-size: 12px; margin-top: 4px">
            不选择节点时，协议将在主服务器上运行
          </div>
        </el-form-item>
        
        <el-form-item
          label="IP监听"
          prop="listen"
        >
          <el-input
            v-model="inboundForm.listen"
            placeholder="填空默认使用0.0.0.0"
          />
        </el-form-item>
        
        <el-form-item
          label="端口"
          prop="port"
        >
          <el-input-number 
            v-model="inboundForm.port" 
            :min="1" 
            :max="65535" 
            style="width: 100%" 
            controls-position="right"
          />
          <el-button 
            size="small" 
            type="primary" 
            style="margin-left: 10px" 
            @click="inboundForm.port = generateRandomPort()"
          >
            随机端口
          </el-button>
        </el-form-item>
        
        <!-- VMess 特有设置 -->
        <template v-if="inboundForm.protocol === 'vmess'">
          <el-form-item
            label="用户ID"
            prop="vmess_id"
          >
            <el-input
              v-model="inboundForm.vmess_id"
              placeholder="UUID格式"
            />
            <el-button 
              size="small" 
              type="primary" 
              style="margin-left: 10px" 
              @click="inboundForm.vmess_id = generateUUID()"
            >
              随机UUID
            </el-button>
          </el-form-item>
          
          <el-form-item
            label="额外ID"
            prop="vmess_aid"
          >
            <el-input-number 
              v-model="inboundForm.vmess_aid" 
              :min="0" 
              :max="65535" 
              style="width: 100%" 
              controls-position="right"
            />
          </el-form-item>
        </template>
        
        <!-- VLESS 特有设置 -->
        <template v-if="inboundForm.protocol === 'vless'">
          <el-form-item
            label="用户ID"
            prop="vless_id"
          >
            <el-input
              v-model="inboundForm.vless_id"
              placeholder="UUID格式"
            />
            <el-button 
              size="small" 
              type="primary" 
              style="margin-left: 10px" 
              @click="inboundForm.vless_id = generateUUID()"
            >
              随机UUID
            </el-button>
          </el-form-item>
          
          <el-form-item
            label="流控"
            prop="vless_flow"
          >
            <el-select
              v-model="inboundForm.vless_flow"
              style="width: 100%"
            >
              <el-option
                label="无流控"
                value="none"
              />
              <el-option
                label="xtls-rprx-vision"
                value="xtls-rprx-vision"
              />
              <el-option
                label="xtls-rprx-vision-udp443"
                value="xtls-rprx-vision-udp443"
              />
            </el-select>
            <div class="form-tip">
              VLESS 的 XTLS/Vision 由这里的流控值决定；下方 TLS 只负责证书与 SNI。
            </div>
          </el-form-item>
        </template>
        
        <!-- Trojan 特有设置 -->
        <template v-if="inboundForm.protocol === 'trojan'">
          <el-form-item
            label="密码"
            prop="trojan_password"
          >
            <el-input
              v-model="inboundForm.trojan_password"
              placeholder="请输入密码"
            />
            <el-button 
              size="small" 
              type="primary" 
              style="margin-left: 10px" 
              @click="generateRandomPassword()"
            >
              随机密码
            </el-button>
          </el-form-item>
          
          <el-form-item
            label="流控"
            prop="trojan_flow"
          >
            <el-select
              v-model="inboundForm.trojan_flow"
              style="width: 100%"
            >
              <el-option
                label="无流控"
                value="none"
              />
              <el-option
                label="xtls-rprx-direct"
                value="xtls-rprx-direct"
              />
              <el-option
                label="xtls-rprx-direct-udp443"
                value="xtls-rprx-direct-udp443"
              />
            </el-select>
            <div class="form-tip">
              Trojan 的 XTLS 流控由这里决定；下方 TLS 只负责证书与 SNI。
            </div>
          </el-form-item>
          
          <el-form-item
            label="回落"
            prop="trojan_fallbacks"
          >
            <el-button
              type="primary"
              @click="addFallback"
            >
              添加回落
            </el-button>
            
            <div
              v-for="(fallback, index) in inboundForm.trojan_fallbacks"
              :key="index"
              class="fallback-item"
            >
              <el-form-item
                label="地址"
                style="margin-bottom: 0; margin-right: 10px; flex: 1;"
              >
                <el-input
                  v-model="fallback.dest"
                  placeholder="回落地址，例如: 127.0.0.1"
                />
              </el-form-item>
              
              <el-form-item
                label="端口"
                style="margin-bottom: 0; margin-right: 10px; width: 150px;"
              >
                <el-input-number
                  v-model="fallback.port"
                  :min="1"
                  :max="65535"
                  style="width: 100%"
                />
              </el-form-item>
              
              <el-button
                type="danger"
                circle
                @click="removeFallback(index)"
              >
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
          </el-form-item>
        </template>
        
        <!-- Shadowsocks 特有设置 -->
        <template v-if="inboundForm.protocol === 'shadowsocks'">
          <el-form-item
            label="加密方式"
            prop="ss_method"
          >
            <el-select
              v-model="inboundForm.ss_method"
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
                label="chacha20-poly1305"
                value="chacha20-poly1305"
              />
              <el-option
                label="chacha20-ietf-poly1305"
                value="chacha20-ietf-poly1305"
              />
              <el-option
                label="none"
                value="none"
              />
            </el-select>
          </el-form-item>
          
          <el-form-item
            label="密码"
            prop="ss_password"
          >
            <el-input
              v-model="inboundForm.ss_password"
              placeholder="请输入密码"
            />
            <el-button 
              size="small" 
              type="primary" 
              style="margin-left: 10px" 
              @click="generateRandomPassword()"
            >
              随机密码
            </el-button>
          </el-form-item>
        </template>
        
        <!-- Dokodemo-Door 特有设置 -->
        <template v-if="inboundForm.protocol === 'dokodemo-door'">
          <el-form-item
            label="目标地址"
            prop="dokodemo_address"
          >
            <el-input
              v-model="inboundForm.dokodemo_address"
              placeholder="请输入目标地址"
            />
          </el-form-item>
          
          <el-form-item
            label="目标端口"
            prop="dokodemo_port"
          >
            <el-input-number 
              v-model="inboundForm.dokodemo_port" 
              :min="1" 
              :max="65535" 
              style="width: 100%" 
              controls-position="right"
            />
          </el-form-item>
        </template>
        
        <el-form-item
          label="网络"
          prop="network"
        >
          <el-select
            v-model="inboundForm.network"
            style="width: 100%"
          >
            <el-option
              label="TCP+UDP"
              value="tcp+udp"
            />
            <el-option
              label="TCP"
              value="tcp"
            />
            <el-option
              label="UDP"
              value="udp"
            />
          </el-select>
        </el-form-item>
        
        <el-divider content-position="left">
          传输设置
        </el-divider>
        
        <el-form-item label="传输协议">
          <el-select
            v-model="inboundForm.stream_settings.network"
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
              label="QUIC"
              value="quic"
            />
            <el-option
              label="gRPC"
              value="grpc"
            />
          </el-select>
        </el-form-item>
        
        <!-- TCP 设置 -->
        <template v-if="inboundForm.stream_settings.network === 'tcp'">
          <el-form-item label="伪装">
            <el-switch
              v-model="inboundForm.stream_settings.tcp_settings.is_http"
              active-text="HTTP伪装"
              inactive-text="不伪装"
            />
          </el-form-item>
          
          <template v-if="inboundForm.stream_settings.tcp_settings.is_http">
            <el-form-item label="域名">
              <el-input
                v-model="inboundForm.stream_settings.tcp_settings.http_settings.host"
                placeholder="请输入域名，多个域名用逗号分隔"
              />
            </el-form-item>
            
            <el-form-item label="路径">
              <el-input
                v-model="inboundForm.stream_settings.tcp_settings.http_settings.path"
                placeholder="请输入路径，例如: /api"
              />
            </el-form-item>
          </template>
        </template>
        
        <!-- WebSocket 设置 -->
        <template v-if="inboundForm.stream_settings.network === 'ws'">
          <el-form-item label="路径">
            <el-input
              v-model="inboundForm.stream_settings.ws_settings.path"
              placeholder="请输入路径，例如: /ws"
            />
          </el-form-item>
          
          <el-form-item label="域名">
            <el-input
              v-model="inboundForm.stream_settings.ws_settings.host"
              placeholder="请输入域名"
            />
          </el-form-item>
        </template>
        
        <!-- HTTP/2 设置 -->
        <template v-if="inboundForm.stream_settings.network === 'http'">
          <el-form-item label="域名">
            <el-input
              v-model="inboundForm.stream_settings.http_settings.host"
              placeholder="请输入域名，多个域名用逗号分隔"
            />
          </el-form-item>
          
          <el-form-item label="路径">
            <el-input
              v-model="inboundForm.stream_settings.http_settings.path"
              placeholder="请输入路径，例如: /h2"
            />
          </el-form-item>
        </template>
        
        <!-- gRPC 设置 -->
        <template v-if="inboundForm.stream_settings.network === 'grpc'">
          <el-form-item label="服务名称">
            <el-input
              v-model="inboundForm.stream_settings.grpc_settings.service_name"
              placeholder="请输入服务名称"
            />
          </el-form-item>

          <el-form-item label="多路复用">
            <el-switch
              v-model="inboundForm.stream_settings.grpc_settings.multi_mode"
              active-text="开启"
              inactive-text="关闭"
            />
            <div class="form-tip">
              对应 Xray 的 gRPC `multiMode`，仅在客户端也开启时生效。
            </div>
          </el-form-item>
        </template>

        <!-- QUIC 设置 -->
        <template v-if="inboundForm.stream_settings.network === 'quic'">
          <el-form-item label="加密方式">
            <el-select
              v-model="inboundForm.stream_settings.quic_settings.security"
              style="width: 100%"
            >
              <el-option
                label="none"
                value="none"
              />
              <el-option
                label="aes-128-gcm"
                value="aes-128-gcm"
              />
              <el-option
                label="chacha20-poly1305"
                value="chacha20-poly1305"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="密钥">
            <el-input
              v-model="inboundForm.stream_settings.quic_settings.key"
              placeholder="请输入 QUIC 密钥；加密方式为 none 时可留空"
            />
          </el-form-item>

          <el-form-item label="头类型">
            <el-select
              v-model="inboundForm.stream_settings.quic_settings.header_type"
              style="width: 100%"
            >
              <el-option
                label="none"
                value="none"
              />
              <el-option
                label="srtp"
                value="srtp"
              />
              <el-option
                label="utp"
                value="utp"
              />
              <el-option
                label="wechat-video"
                value="wechat-video"
              />
              <el-option
                label="dtls"
                value="dtls"
              />
              <el-option
                label="wireguard"
                value="wireguard"
              />
            </el-select>
          </el-form-item>

          <div class="form-tip">
            QUIC 会直接写入 `quicSettings`，不再依赖扁平参数。
          </div>
        </template>
        
        <el-divider content-position="left">
          安全设置
        </el-divider>

        <el-form-item
          v-if="supportsReality"
          label="安全协议"
        >
          <el-select
            v-model="securityMode"
            style="width: 100%"
          >
            <el-option
              label="无"
              value="none"
            />
            <el-option
              label="TLS"
              value="tls"
            />
            <el-option
              label="Reality"
              value="reality"
            />
          </el-select>
        </el-form-item>

        <el-form-item
          v-else
          label="TLS"
        >
          <el-switch v-model="tlsEnabled" />
        </el-form-item>
        
        <template v-if="tlsSettingsEnabled">
          <el-form-item label="域名">
            <el-select
              v-model="inboundForm.stream_settings.tls_settings.server_name"
              filterable
              clearable
              :loading="certificatesLoading"
              placeholder="请选择已签发证书对应的域名"
              style="width: 100%"
            >
              <el-option
                v-for="cert in availableCertificateOptions"
                :key="cert.id"
                :label="getCertificateOptionLabel(cert)"
                :value="cert.domain"
                :disabled="cert.disabled"
              />
            </el-select>
            <div class="form-tip">
              只能从“证书管理”中选择已签发且可用的证书域名。
            </div>
            <div
              v-if="selectedCertificateOption"
              class="form-tip"
            >
              当前证书：{{ selectedCertificateOption.domain }}<span v-if="selectedCertificateOption.expireDate && selectedCertificateOption.expireDate !== '-'">，到期 {{ selectedCertificateOption.expireDate }}</span>
            </div>
            <div
              v-else-if="!certificatesLoading && !availableCertificateOptions.length"
              class="form-tip"
            >
              当前没有可选证书，请先到“证书管理”申请或上传证书。
            </div>
            <div
              v-if="effectiveSNI"
              class="cert-input"
              style="margin-top: 8px"
            >
              <el-tag type="info">
                客户端连接预览
              </el-tag>
              <div
                v-if="effectiveServerAddress"
                class="form-tip"
              >
                服务器地址：{{ effectiveServerAddress }}<span v-if="effectiveServerAddressSource">（来源：{{ effectiveServerAddressSource }}）</span>
              </div>
              <div
                v-if="effectiveSNI"
                class="form-tip"
              >
                SNI：{{ effectiveSNI }}
              </div>
              <div class="form-tip">
                保存后，分享链接会优先使用这里展示的服务器地址与 SNI。
              </div>
            </div>
          </el-form-item>
          
          <el-form-item label="证书配置">
            <div class="cert-input">
              <el-tag type="success">
                自动匹配系统证书
              </el-tag>
              <div class="form-tip">
                保存后会按上面的域名，从“证书管理”里自动匹配已签发且可用的系统证书。
              </div>
            </div>
          </el-form-item>

          <el-form-item label="ALPN">
            <el-input
              v-model="inboundForm.stream_settings.tls_settings.alpn"
              placeholder="可选，例如: h2,http/1.1"
            />
            <div class="form-tip">
              多个值用逗号分隔，保存后会写入 TLS 的 ALPN 列表。
            </div>
          </el-form-item>
        </template>

        <template v-if="realitySettingsEnabled">
          <el-form-item label="目标地址">
            <el-input
              v-model="inboundForm.stream_settings.reality_settings.dest"
              placeholder="请输入 Reality dest，例如: www.cloudflare.com:443"
            />
            <div class="form-tip">
              这是 Reality 服务端回落目标，会写入 `realitySettings.dest`。
            </div>
          </el-form-item>

          <el-form-item label="ServerNames">
            <el-input
              v-model="inboundForm.stream_settings.reality_settings.server_names"
              placeholder="请输入 Server Names，多个值用逗号分隔"
            />
            <div class="form-tip">
              第一个域名会同时作为分享链接的 SNI。
            </div>
          </el-form-item>

          <el-form-item label="私钥">
            <el-input
              v-model="inboundForm.stream_settings.reality_settings.private_key"
              type="textarea"
              :rows="3"
              placeholder="请输入 xray x25519 生成的 private key"
            />
            <div class="form-tip">
              保存时会根据私钥自动推导公钥，并同步到订阅导出字段。
            </div>
          </el-form-item>

          <el-form-item label="公钥">
            <el-input
              v-model="inboundForm.stream_settings.reality_settings.public_key"
              disabled
              placeholder="保存后自动生成"
            />
          </el-form-item>

          <el-form-item label="Short IDs">
            <el-input
              v-model="inboundForm.stream_settings.reality_settings.short_ids"
              placeholder="可选，多个值用逗号分隔；留空将使用空 shortId"
            />
          </el-form-item>

          <el-form-item label="Xver">
            <el-input-number
              v-model="inboundForm.stream_settings.reality_settings.xver"
              :min="0"
              :max="2"
              style="width: 100%"
              controls-position="right"
            />
          </el-form-item>
        </template>

        <template v-if="clientFingerprintVisible">
          <el-form-item label="客户端指纹">
            <el-select
              v-model="inboundForm.stream_settings.client_settings.fingerprint"
              style="width: 100%"
              filterable
              allow-create
              clearable
              default-first-option
              placeholder="可选，例如: chrome"
            >
              <el-option
                v-for="option in fingerprintOptions"
                :key="option"
                :label="option"
                :value="option"
              />
            </el-select>
            <div class="form-tip">
              这是客户端分享/订阅参数，不影响服务端入站监听。
            </div>
          </el-form-item>
        </template>

        <template v-if="allowInsecureVisible">
          <el-form-item label="跳过证书校验">
            <el-switch v-model="inboundForm.stream_settings.client_settings.allow_insecure" />
            <div class="form-tip">
              这是客户端参数，只会写进分享链接和订阅导出。
            </div>
          </el-form-item>
        </template>
        
        <el-divider content-position="left">
          高级设置
        </el-divider>
        
        <el-form-item label="流量限制">
          <div class="expiry-readonly">
            <div class="expiry-readonly__value">
              {{ inboundForm.traffic_limit_display || '不适用' }}
            </div>
            <div
              v-if="inboundForm.traffic_limit_source_label"
              class="form-tip"
            >
              来源：{{ inboundForm.traffic_limit_source_label }}
            </div>
            <div class="form-tip">
              该额度跟随用户试用或订阅流量，不在代理协议里单独设置。
            </div>
          </div>
        </el-form-item>
        
        <el-form-item label="过期时间">
          <div class="expiry-readonly">
            <div class="expiry-readonly__value">
              {{ inboundForm.expires_at_display || '不适用' }}
            </div>
            <div
              v-if="inboundForm.expiry_source_label"
              class="form-tip"
            >
              来源：{{ inboundForm.expiry_source_label }}
            </div>
            <div class="form-tip">
              该时间跟随用户试用或订阅有效期，不在代理协议里单独设置。
            </div>
          </div>
        </el-form-item>
        
        <el-form-item label="嗅探">
          <el-switch v-model="inboundForm.sniffing.enabled" />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <div style="text-align: right">
          <el-button @click="addInboundDialogVisible = false">
            取消
          </el-button>
          <el-button
            type="primary"
            :loading="submitting"
            @click="saveInbound"
          >
            保存
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 二维码对话框 -->
    <el-dialog
      v-model="qrCodeDialogVisible"
      title="分享二维码"
      width="350px"
      destroy-on-close
      :close-on-click-modal="false"
    >
      <div class="qrcode-container">
        <div
          id="qrcode-display"
          class="qrcode"
        />
        <div class="protocol-name">
          {{ currentQrCodeInfo?.protocol.toUpperCase() }}
        </div>
        <div class="remark">
          {{ currentQrCodeInfo?.remark }}
        </div>
      </div>
      
      <template #footer>
        <div style="text-align: center">
          <el-button
            type="primary"
            @click="downloadQrCode"
          >
            下载二维码
          </el-button>
          <el-button @click="qrCodeDialogVisible = false">
            关闭
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Delete, MoreFilled } from '@element-plus/icons-vue'
import api from '@/api/index'
import QRCode from 'qrcode'
import { extractErrorMessage } from '@/utils/entitlement'

const route = useRoute()
const router = useRouter()

// 数据表格
const loading = ref(false)
const inbounds = ref([])
const nodeList = ref([])  // 节点列表
const certificates = ref([])
const certificatesLoading = ref(false)
const inboundsTableRef = ref(null)
const batchLoading = ref('')
const selectedInboundIds = ref([])
const filters = reactive({
  keyword: '',
  protocol: '',
  status: ''
})

// 表单对话框
const addInboundDialogVisible = ref(false)
const inboundFormRef = ref(null)
const submitting = ref(false)
const dialogLoading = ref(false)
const dialogMode = ref('add')

// 二维码相关
const qrCodeDialogVisible = ref(false)
const currentQrCodeInfo = ref(null)
const currentQrCodeLink = ref('')

// 默认表单
const defaultInboundForm = {
  remark: '',
  enable: true,
  protocol: 'vmess',
  listen: '',
  port: null,
  node_id: null,  // 节点ID
  traffic_limit: 0,
  traffic_limit_display: '不适用',
  traffic_limit_source: '',
  traffic_limit_source_label: '',
  expires_at: '',
  expires_at_display: '不适用',
  expiry_source: '',
  expiry_source_label: '',
  vmess_id: '',  // vmess 特有
  vmess_aid: 0,  // vmess 特有
  vless_id: '',  // vless 特有
  vless_flow: 'none', // vless 特有
  trojan_password: '', // trojan 特有
  trojan_flow: 'none',  // trojan 特有
  trojan_fallbacks: [], // trojan 特有
  ss_method: 'aes-256-gcm', // shadowsocks 特有
  ss_password: '', // shadowsocks 特有
  dokodemo_address: '', // dokodemo-door 特有
  dokodemo_port: null, // dokodemo-door 特有
  network: 'tcp+udp',
  stream_settings: {
    network: 'tcp',
    security: '',
    tcp_settings: {
      is_http: false,
      http_settings: {
        host: '',
        path: '/'
      }
    },
    ws_settings: {
      path: '/',
      host: ''
    },
    http_settings: {
      host: '',
      path: '/'
    },
    quic_settings: {
      security: 'none',
      key: '',
      header_type: 'none'
    },
    grpc_settings: {
      service_name: '',
      multi_mode: false
    },
    tls_settings: {
      server_name: '',
      alpn: ''
    },
    reality_settings: {
      dest: '',
      server_names: '',
      private_key: '',
      public_key: '',
      short_ids: '',
      xver: 0,
      show: false
    },
    client_settings: {
      fingerprint: '',
      allow_insecure: false
    }
  },
  sniffing: {
    enabled: true,
    dest_override: ['http', 'tls', 'quic']
  }
}

// 当前表单
const inboundForm = reactive({...defaultInboundForm})

const unwrapApiData = (response) => response?.data ?? response ?? null
const normalizeStringValue = (value) => typeof value === 'string' ? value.trim() : ''
const normalizeProtocol = (value) => normalizeStringValue(value).toLowerCase()
const formatProtocolLabel = (value) => {
  const protocol = normalizeProtocol(value)
  const labelMap = {
    vmess: 'VMess',
    vless: 'VLESS',
    trojan: 'Trojan',
    shadowsocks: 'Shadowsocks',
    'dokodemo-door': 'Dokodemo'
  }
  return labelMap[protocol] || (protocol ? protocol.toUpperCase() : '-')
}
const protocolOptions = [
  { label: 'VMess', value: 'vmess' },
  { label: 'VLESS', value: 'vless' },
  { label: 'Trojan', value: 'trojan' },
  { label: 'Shadowsocks', value: 'shadowsocks' },
  { label: 'Dokodemo', value: 'dokodemo-door' }
]
const firstStringValue = (value) => Array.isArray(value) ? normalizeStringValue(value[0]) : normalizeStringValue(value)
const splitCommaValues = (value) => normalizeStringValue(value)
  .split(',')
  .map(item => normalizeStringValue(item))
  .filter(Boolean)
const joinCommaValues = (value) => {
  if (Array.isArray(value)) {
    return value
      .map(item => normalizeStringValue(item))
      .filter(Boolean)
      .join(', ')
  }
  return normalizeStringValue(value)
}
const preferStructuredValue = (value, fallback) => {
  if (Array.isArray(value)) {
    return value.some(item => normalizeStringValue(item)) ? value : fallback
  }
  return normalizeStringValue(value) ? value : fallback
}
const asObject = (value) => value && typeof value === 'object' && !Array.isArray(value) ? value : {}
const normalizeBooleanValue = (value) => value === true || value === 'true' || value === 1 || value === '1'
const cloneDefaultInboundForm = () => JSON.parse(JSON.stringify(defaultInboundForm))
const resetInboundForm = () => Object.assign(inboundForm, cloneDefaultInboundForm())
const fingerprintOptions = ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', '360', 'qq', 'randomized']
const formatCertificateDate = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toISOString().slice(0, 10)
}
const formatDateTime = (value) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  })
}
const getExpirySourceLabel = (source) => {
  if (source === 'trial') return '试用'
  if (source === 'subscription') return '订阅'
  return ''
}
const formatProxyExpiryDisplay = (expiresAt, expirySource) => {
  if (expiresAt) return formatDateTime(expiresAt)
  if (expirySource === 'subscription') return '不限制'
  return '未设置'
}
const invalidShareHosts = new Set(['', '0.0.0.0', '::', '[::]', '0:0:0:0:0:0:0:0'])
const normalizeCertificatesResponse = (response) => {
  if (Array.isArray(response)) return response
  if (Array.isArray(response?.certificates)) return response.certificates
  if (Array.isArray(response?.data?.certificates)) return response.data.certificates
  if (Array.isArray(response?.data)) return response.data
  return []
}
const normalizeShareHost = (rawValue) => {
  const value = normalizeStringValue(rawValue)
  if (!value) return ''

  try {
    const normalized = value.includes('://') ? new URL(value).hostname : new URL(`https://${value}`).hostname
    return invalidShareHosts.has(normalized.toLowerCase()) ? '' : normalized
  } catch {
    return invalidShareHosts.has(value.toLowerCase()) ? '' : value
  }
}
const encodeBase64UTF8 = (value) => {
  const text = String(value ?? '')
  if (typeof window !== 'undefined' && typeof window.btoa === 'function') {
    const bytes = new TextEncoder().encode(text)
    let binary = ''
    bytes.forEach((byte) => {
      binary += String.fromCharCode(byte)
    })
    return window.btoa(binary)
  }
  return text
}
const getCurrentAccessHost = () => {
  if (typeof window === 'undefined') return ''
  return normalizeShareHost(window.location.hostname)
}
const getSettingString = (settings = {}, ...keys) => {
  for (const key of keys) {
    const value = settings?.[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}
const getNodeAddressByID = (nodeID) => {
  if (nodeID === null || nodeID === undefined) return ''
  const node = nodeList.value.find((item) => String(item.id) === String(nodeID))
  return normalizeShareHost(node?.address)
}
const getNodeNameByID = (nodeID) => {
  if (nodeID === null || nodeID === undefined) return ''
  const node = nodeList.value.find((item) => String(item.id) === String(nodeID))
  return normalizeStringValue(node?.name)
}
const getInboundName = (row = {}) => normalizeStringValue(row.remark || row.name) || `代理 #${row.id ?? '-'}`
const getInboundClientCount = (row = {}) => {
  const value = Number(row.clientCount ?? row.client_count ?? 0)
  return Number.isFinite(value) ? value : 0
}
const getInboundNodeLabel = (row = {}) => getNodeNameByID(row.node_id) || '主服务器'
const getInboundServerDisplay = (row = {}) => {
  const settings = asObject(row.settings)
  const candidates = [
    getNodeAddressByID(row.node_id),
    getSettingString(settings, 'server', 'add', 'address'),
    row.server,
    row.host
  ]

  for (const candidate of candidates) {
    const normalized = normalizeShareHost(candidate)
    if (normalized) return normalized
  }

  return ''
}
const mapInboundRow = (proxy = {}) => {
  const expirySource = normalizeStringValue(proxy.expiry_source)
  const trafficLimitSource = normalizeStringValue(proxy.traffic_limit_source || proxy.traffic_source)

  return {
    ...proxy,
    remark: getInboundName(proxy),
    protocol: normalizeProtocol(proxy.protocol),
    enable: proxy.enabled ?? proxy.enable ?? false,
    clientCount: getInboundClientCount(proxy),
    created_at_display: formatDateTime(proxy.created_at) || '-',
    traffic_limit_source: trafficLimitSource,
    traffic_limit_source_label: getExpirySourceLabel(trafficLimitSource),
    traffic_limit_display: formatProxyTrafficDisplay(proxy.traffic_limit),
    expiry_source: expirySource,
    expiry_source_label: getExpirySourceLabel(expirySource),
    expires_at_display: formatProxyExpiryDisplay(proxy.expires_at, expirySource)
  }
}
const filteredInbounds = computed(() => {
  const keyword = normalizeStringValue(filters.keyword).toLowerCase()

  return inbounds.value.filter((row) => {
    if (filters.protocol && normalizeProtocol(row.protocol) !== filters.protocol) {
      return false
    }

    if (filters.status === 'enabled' && !row.enable) {
      return false
    }

    if (filters.status === 'disabled' && row.enable) {
      return false
    }

    if (!keyword) {
      return true
    }

    const searchable = [
      getInboundName(row),
      formatProtocolLabel(row.protocol),
      getInboundNodeLabel(row),
      getInboundServerDisplay(row),
      row.created_at_display,
      row.expires_at_display,
      String(row.port ?? ''),
      row.enable ? '运行中' : '已停止'
    ]

    return searchable.some((item) => String(item || '').toLowerCase().includes(keyword))
  })
})
const paginatedInbounds = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredInbounds.value.slice(start, start + pageSize.value)
})
const enabledInboundCount = computed(() => paginatedInbounds.value.filter((item) => item.enable).length)
const disabledInboundCount = computed(() => paginatedInbounds.value.length - enabledInboundCount.value)
const hasActiveFilters = computed(() => Boolean(
  normalizeStringValue(filters.keyword) || filters.protocol || filters.status
))
const availableCertificateOptions = computed(() => certificates.value)
const firstAvailableCertificateOption = computed(() => availableCertificateOptions.value.find((cert) => !cert.disabled) || null)
const selectedCertificateOption = computed(() => {
  const selectedDomain = normalizeStringValue(inboundForm.stream_settings.tls_settings.server_name)
  if (!selectedDomain) return null
  return certificates.value.find((cert) => cert.domain === selectedDomain) || null
})
const getCertificateOptionLabel = (cert) => {
  if (!cert) return ''
  const suffix = cert.statusLabel ? `，${cert.statusLabel}` : ''
  return cert.expireDate && cert.expireDate !== '-'
    ? `${cert.domain}（到期 ${cert.expireDate}${suffix}）`
    : `${cert.domain}${suffix ? `（${cert.statusLabel}）` : ''}`
}
const effectiveSNI = computed(() => normalizeStringValue(inboundForm.stream_settings.tls_settings.server_name))
const selectedNodeAddress = computed(() => getNodeAddressByID(inboundForm.node_id))
const resolveLocalFallbackServer = (row = {}) => {
  const candidates = [
    getNodeAddressByID(row.node_id),
    row.server,
    row.host,
    getCurrentAccessHost()
  ]

  for (const candidate of candidates) {
    const normalized = normalizeShareHost(candidate)
    if (normalized) return normalized
  }

  return 'example.com'
}
const effectiveServerAddress = computed(() => {
  const candidates = [
    selectedNodeAddress.value,
    effectiveSNI.value,
    inboundForm.listen,
    getCurrentAccessHost()
  ]

  for (const candidate of candidates) {
    const normalized = normalizeShareHost(candidate)
    if (normalized) return normalized
  }

  return ''
})
const effectiveServerAddressSource = computed(() => {
  if (!effectiveServerAddress.value) return ''
  if (selectedNodeAddress.value && selectedNodeAddress.value === effectiveServerAddress.value) return '部署节点'
  if (effectiveSNI.value && normalizeShareHost(effectiveSNI.value) === effectiveServerAddress.value) return '证书域名'
  if (normalizeShareHost(inboundForm.listen) === effectiveServerAddress.value) return 'IP监听'
  if (getCurrentAccessHost() === effectiveServerAddress.value) return '当前访问地址'
  return ''
})

const supportsReality = computed(() => inboundForm.protocol === 'vless')
const tlsSettingsEnabled = computed(() => inboundForm.stream_settings.security === 'tls')
const realitySettingsEnabled = computed(() => supportsReality.value && inboundForm.stream_settings.security === 'reality')
const clientFingerprintVisible = computed(() => ['vmess', 'vless', 'trojan'].includes(inboundForm.protocol) && (tlsSettingsEnabled.value || realitySettingsEnabled.value))
const allowInsecureVisible = computed(() => ['vmess', 'vless', 'trojan'].includes(inboundForm.protocol) && tlsSettingsEnabled.value)
const securityMode = computed({
  get: () => {
    const security = normalizeStringValue(inboundForm.stream_settings.security)
    if (supportsReality.value) {
      if (security === 'reality') return 'reality'
      if (security === 'tls') return 'tls'
      return 'none'
    }
    return security === 'tls' ? 'tls' : 'none'
  },
  set: (value) => {
    if (value === 'tls') {
      inboundForm.stream_settings.security = 'tls'
      selectDefaultCertificateDomain()
      return
    }
    if (value === 'reality' && supportsReality.value) {
      inboundForm.stream_settings.security = 'reality'
      return
    }
    inboundForm.stream_settings.security = ''
  }
})

const shouldAutoSelectCertificateDomain = () => dialogMode.value === 'add'
  && tlsSettingsEnabled.value
  && !normalizeStringValue(inboundForm.stream_settings.tls_settings.server_name)

const selectDefaultCertificateDomain = () => {
  if (!shouldAutoSelectCertificateDomain()) {
    return
  }

  const defaultCertificate = firstAvailableCertificateOption.value
  if (defaultCertificate?.domain) {
    inboundForm.stream_settings.tls_settings.server_name = defaultCertificate.domain
  }
}

// TLS开关
const tlsEnabled = computed({
  get: () => tlsSettingsEnabled.value,
  set: (value) => {
    if (!value) {
      inboundForm.stream_settings.security = ''
      return
    }
    if (!tlsSettingsEnabled.value) {
      inboundForm.stream_settings.security = 'tls'
      selectDefaultCertificateDomain()
    }
  }
})

// 表单验证规则
const rules = {
  remark: [
    { required: true, message: '请输入备注', trigger: 'blur' }
  ],
  protocol: [
    { required: true, message: '请选择协议', trigger: 'change' }
  ],
  port: [
    { required: true, message: '请输入端口', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: '端口范围 1-65535', trigger: 'blur' }
  ],
  ss_method: [
    { required: true, message: '请选择加密方式', trigger: 'change' }
  ],
  ss_password: [
    { required: true, message: '请输入密码', trigger: 'blur' }
  ],
  trojan_password: [
    { required: true, message: '请输入密码', trigger: 'blur' }
  ],
  vmess_id: [
    { required: true, message: '请输入ID', trigger: 'blur' }
  ],
  vless_id: [
    { required: true, message: '请输入ID', trigger: 'blur' }
  ],
  dokodemo_address: [
    { required: true, message: '请输入目标地址', trigger: 'blur' }
  ],
  dokodemo_port: [
    { required: true, message: '请输入目标端口', trigger: 'blur' },
    { type: 'number', min: 1, max: 65535, message: '端口范围 1-65535', trigger: 'blur' }
  ]
}

// 添加运行时验证
const validateTrojanForm = () => {
  if (inboundForm.protocol === 'trojan') {
    tlsEnabled.value = true
  }
  if (inboundForm.protocol !== 'vless' && inboundForm.stream_settings.security === 'reality') {
    inboundForm.stream_settings.security = ''
  }
}

// 监听协议变化
watch(() => inboundForm.protocol, () => {
  validateTrojanForm();
})

// 分页相关
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

// 处理分页
const handleSizeChange = (size) => {
  pageSize.value = size
  currentPage.value = 1
}

const handleCurrentChange = (page) => {
  currentPage.value = page
}

const clearSelectionState = async () => {
  selectedInboundIds.value = []
  await nextTick()
  inboundsTableRef.value?.clearSelection()
}

const syncTableSelection = async () => {
  await nextTick()

  if (!inboundsTableRef.value) {
    return
  }

  inboundsTableRef.value.clearSelection()
  const selectedIds = new Set(selectedInboundIds.value.map((id) => String(id)))

  paginatedInbounds.value.forEach((row) => {
    if (selectedIds.has(String(row.id))) {
      inboundsTableRef.value.toggleRowSelection(row, true)
    }
  })
}

const handleSelectionChange = (rows) => {
  const currentPageIds = new Set(paginatedInbounds.value.map((row) => String(row.id)))
  const nextSelectedIds = selectedInboundIds.value.filter((id) => !currentPageIds.has(String(id)))

  rows.forEach((row) => {
    nextSelectedIds.push(row.id)
  })

  selectedInboundIds.value = Array.from(new Set(nextSelectedIds))
}

const resetFilters = () => {
  filters.keyword = ''
  filters.protocol = ''
  filters.status = ''
}

const runBatchOperation = async (operation) => {
  if (!selectedInboundIds.value.length || batchLoading.value) {
    return
  }

  const actionTextMap = {
    enable: '启用',
    disable: '停用',
    delete: '删除'
  }
  const actionText = actionTextMap[operation] || '处理'

  try {
    await ElMessageBox.confirm(
      `确定要${actionText}选中的 ${selectedInboundIds.value.length} 个代理服务吗？`,
      '批量操作确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: operation === 'delete' ? 'warning' : 'info'
      }
    )
  } catch {
    return
  }

  batchLoading.value = operation
  try {
    await api.post('/proxies/batch', {
      ids: selectedInboundIds.value,
      operation
    })
    ElMessage.success(`批量${actionText}成功`)
    await clearSelectionState()
    await loadInbounds()
  } catch (error) {
    console.error(`Failed to batch ${operation} inbounds:`, error)
    ElMessage.error(`批量${actionText}失败: ${extractErrorMessage(error) || '未知错误'}`)
  } finally {
    batchLoading.value = ''
  }
}

const handleRowCommand = (command, row) => {
  if (command === 'edit') {
    editInbound(row)
    return
  }
  if (command === 'qr') {
    showQrCode(row)
    return
  }
  if (command === 'delete') {
    deleteInbound(row)
  }
}

const consumeRoutePreset = async () => {
  const create = route.query.create === '1'
  const tlsDomain = normalizeStringValue(route.query.tls_domain)

  if (!create && !tlsDomain) {
    return
  }

  if (!addInboundDialogVisible.value || dialogMode.value !== 'add') {
    openAddInboundDialog()
  }

  if (tlsDomain) {
    inboundForm.stream_settings.security = 'tls'
    inboundForm.stream_settings.tls_settings.server_name = tlsDomain
  }

  await nextTick()

  const nextQuery = { ...route.query }
  delete nextQuery.create
  delete nextQuery.tls_domain
  router.replace({ path: route.path, query: nextQuery })
}

// 初始化
onMounted(() => {
  loadInbounds()
  loadNodes()
  loadCertificates()
})

watch(() => [route.query.create, route.query.tls_domain], () => {
  consumeRoutePreset()
}, { immediate: true })

watch(() => [filters.keyword, filters.protocol, filters.status], async () => {
  currentPage.value = 1
  await clearSelectionState()
})

watch(filteredInbounds, (rows) => {
  const maxPage = Math.max(1, Math.ceil(rows.length / pageSize.value))
  if (currentPage.value > maxPage) {
    currentPage.value = maxPage
  }
})

watch(paginatedInbounds, () => {
  syncTableSelection()
})

// 加载节点列表
const loadNodes = async () => {
  try {
    const response = await api.get('/admin/nodes')
    const data = unwrapApiData(response)
    nodeList.value = data.nodes || data.list || (Array.isArray(data) ? data : [])
  } catch (error) {
    console.error('加载节点列表失败:', error)
    nodeList.value = []
  }
}

const loadCertificates = async () => {
  certificatesLoading.value = true
  try {
    const response = await api.get('/certificates', {
      params: {
        limit: 1000,
        offset: 0
      }
    })
    const normalized = normalizeCertificatesResponse(unwrapApiData(response))
    certificates.value = normalized
      .map((cert) => {
        const status = normalizeStringValue(cert.status || '').toLowerCase()
        const isUsable = !['expired', 'failed', 'pending'].includes(status)
        const statusLabelMap = {
          valid: '有效',
          expiring: '即将过期',
          expired: '已过期',
          failed: '失败',
          pending: '处理中'
        }
        return {
          ...cert,
          domain: normalizeStringValue(cert.domain),
          expireDate: formatCertificateDate(cert.expires_at || cert.expiresAt),
          status,
          statusLabel: statusLabelMap[status] || '',
          disabled: !isUsable
        }
      })
      .filter((cert) => cert.domain)
      .sort((left, right) => {
        if (left.disabled !== right.disabled) return left.disabled ? 1 : -1
        return left.domain.localeCompare(right.domain)
      })

    selectDefaultCertificateDomain()
  } catch (error) {
    console.error('加载证书列表失败:', error)
    certificates.value = []
  } finally {
    certificatesLoading.value = false
  }
}

// 加载入站列表
const loadInbounds = async () => {
  loading.value = true
  try {
    const allRows = []
    const pageLimit = 200
    let expectedTotal = 0
    let offset = 0
    let requestCount = 0

    while (requestCount < 50) {
      const response = await api.get('/proxies', {
        params: {
          limit: pageLimit,
          offset
        }
      })
      const data = unwrapApiData(response)
      const list = Array.isArray(data) ? data : (data?.list || [])

      if (!Array.isArray(list) || list.length === 0) {
        if (!Array.isArray(data)) {
          expectedTotal = Number(data?.total || allRows.length)
        }
        break
      }

      allRows.push(...list)
      requestCount += 1

      if (Array.isArray(data)) {
        if (list.length < pageLimit) {
          break
        }
      } else {
        expectedTotal = Number(data?.total || allRows.length)
        if (allRows.length >= expectedTotal || list.length < pageLimit) {
          break
        }
      }

      offset += pageLimit
    }

    const dedupedRows = Array.from(new Map(allRows.map((item) => [item.id, item])).values())
    inbounds.value = dedupedRows.map((p) => mapInboundRow({
      id: p.id,
      user_id: p.user_id,
      name: p.name,
      remark: p.remark,
      protocol: p.protocol,
      port: p.port,
      host: p.host || '',
      node_id: p.node_id ?? null,
      settings: p.settings || {},
      enabled: p.enabled,
      clientCount: p.clientCount ?? p.client_count ?? 0,
      created_at: p.created_at,
      traffic_limit: p.traffic_limit || 0,
      traffic_limit_source: p.traffic_limit_source || '',
      expires_at: p.expires_at || '',
      expiry_source: p.expiry_source || ''
    }))
    total.value = expectedTotal || dedupedRows.length
    selectedInboundIds.value = selectedInboundIds.value.filter((id) =>
      inbounds.value.some((row) => String(row.id) === String(id))
    )
  } catch (error) {
    console.error('Failed to load inbounds:', error)
    ElMessage.error('加载入站列表失败')
    inbounds.value = []
    total.value = 0
    await clearSelectionState()
  } finally {
    loading.value = false
    syncTableSelection()
  }
}

// 流量格式化
const formatTraffic = (bytes) => {
  if (bytes === 0) return '0 B'
  
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const formatProxyTrafficDisplay = (trafficLimit) => {
  if (!trafficLimit || trafficLimit <= 0) return '不限制'
  return formatTraffic(trafficLimit)
}

// 打开添加入站对话框
function openAddInboundDialog() {
  dialogMode.value = 'add'
  dialogLoading.value = false
  loadCertificates()
  // 重置表单
  resetInboundForm()
  // 设置默认端口
  inboundForm.port = generateRandomPort()
  
  // 根据协议类型初始化特定字段
  if (inboundForm.protocol === 'vmess') {
    inboundForm.vmess_id = generateUUID()
  } else if (inboundForm.protocol === 'vless') {
    inboundForm.vless_id = generateUUID()
  } else if (inboundForm.protocol === 'trojan') {
    inboundForm.trojan_password = generateRandomPassword();
    // 确保TLS启用
    tlsEnabled.value = true
  }
  
  // 验证表单
  validateTrojanForm();
  selectDefaultCertificateDomain()
  addInboundDialogVisible.value = true
}

const normalizeProxyToInboundForm = (proxyData = {}) => {
  const settings = proxyData.settings || {}
  const form = cloneDefaultInboundForm()
  const wsSettings = asObject(settings.ws_settings)
  const wsHeaders = asObject(wsSettings.headers)
  const httpSettings = asObject(settings.http_settings)
  const grpcSettings = asObject(settings.grpc_settings)
  const quicSettings = asObject(settings.quic_settings)
  const quicHeader = asObject(quicSettings.header)
  const realitySettings = asObject(settings.reality_settings)
  const tcpSettings = asObject(settings.tcp_settings)
  const tcpHeader = asObject(tcpSettings.header)
  const tcpRequest = asObject(tcpHeader.request)
  const tcpRequestHeaders = asObject(tcpRequest.headers)

  form.id = proxyData.id || null
  form.remark = proxyData.name || proxyData.remark || ''
  form.enable = proxyData.enabled ?? true
  form.protocol = proxyData.protocol || form.protocol
  form.listen = proxyData.host || ''
  form.port = proxyData.port || null
  form.node_id = proxyData.node_id ?? null
  form.traffic_limit = Number(proxyData.traffic_limit || 0)
  form.traffic_limit_source = proxyData.traffic_limit_source || ''
  form.traffic_limit_source_label = getExpirySourceLabel(proxyData.traffic_limit_source)
  form.traffic_limit_display = formatProxyTrafficDisplay(proxyData.traffic_limit, proxyData.traffic_limit_source) || '不适用'
  form.expires_at = proxyData.expires_at || ''
  form.expiry_source = proxyData.expiry_source || ''
  form.expiry_source_label = getExpirySourceLabel(proxyData.expiry_source)
  form.expires_at_display = formatProxyExpiryDisplay(proxyData.expires_at, proxyData.expiry_source) || '不适用'

  const transportNetwork = settings.network || form.stream_settings.network
  form.stream_settings.network = transportNetwork

  const tlsDomain = normalizeStringValue(settings.server_name || settings.sni || settings.tls_domain)
  const hasLegacyCertificateMaterial = (normalizeStringValue(settings.cert_file) && normalizeStringValue(settings.key_file)) || (firstStringValue(settings.certificate) && firstStringValue(settings.key))
  const hasTLS = settings.security === 'tls' || settings.tls === true || !!tlsDomain || hasLegacyCertificateMaterial
  form.stream_settings.tls_settings.alpn = joinCommaValues(settings.alpn)
  form.stream_settings.client_settings.allow_insecure = normalizeBooleanValue(settings.allowInsecure)
  form.stream_settings.client_settings.fingerprint = normalizeStringValue(settings.fingerprint || settings.fp)

  if (settings.security === 'reality') {
    form.stream_settings.security = 'reality'
    form.stream_settings.reality_settings.dest = normalizeStringValue(realitySettings.dest || settings.dest)
    form.stream_settings.reality_settings.server_names = joinCommaValues(preferStructuredValue(realitySettings.serverNames, settings.server_name || settings.sni))
    form.stream_settings.reality_settings.private_key = normalizeStringValue(realitySettings.privateKey || settings.privateKey)
    form.stream_settings.reality_settings.public_key = normalizeStringValue(settings.publicKey || settings.pbk)
    form.stream_settings.reality_settings.short_ids = joinCommaValues(preferStructuredValue(realitySettings.shortIds, settings.shortId || settings.sid))
    form.stream_settings.reality_settings.xver = Number(realitySettings.xver ?? settings.xver ?? 0) || 0
  } else if (hasTLS) {
    form.stream_settings.security = 'tls'
    form.stream_settings.tls_settings.server_name = tlsDomain
  }

  switch (form.protocol) {
    case 'trojan':
      form.trojan_password = settings.password || ''
      form.trojan_flow = settings.flow || 'none'
      form.trojan_fallbacks = Array.isArray(settings.fallbacks) ? settings.fallbacks : []
      break
    case 'vless':
      form.vless_id = settings.uuid || ''
      form.vless_flow = settings.flow || 'none'
      break
    case 'vmess':
      form.vmess_id = settings.uuid || ''
      form.vmess_aid = Number(settings.alter_id ?? settings.alterId ?? 0)
      break
    case 'shadowsocks':
      form.ss_method = settings.method || form.ss_method
      form.ss_password = settings.password || ''
      form.network = settings.network || form.network
      break
    case 'dokodemo-door':
      form.dokodemo_address = settings.address || ''
      form.dokodemo_port = Number(settings.port || 0) || null
      form.network = settings.network || form.network
      break
  }

  switch (transportNetwork) {
    case 'ws':
      form.stream_settings.ws_settings.path = normalizeStringValue(wsSettings.path || settings.path) || '/'
      form.stream_settings.ws_settings.host = firstStringValue(wsHeaders.Host || wsSettings.host || settings.host)
      break
    case 'http':
      form.stream_settings.http_settings.path = normalizeStringValue(httpSettings.path || settings.path) || '/'
      form.stream_settings.http_settings.host = joinCommaValues(preferStructuredValue(httpSettings.host, settings.host))
      break
    case 'grpc':
      form.stream_settings.grpc_settings.service_name = normalizeStringValue(grpcSettings.serviceName || grpcSettings.service_name || settings.serviceName || settings.service_name)
      form.stream_settings.grpc_settings.multi_mode = normalizeBooleanValue(grpcSettings.multiMode ?? grpcSettings.multi_mode)
      break
    case 'quic':
      form.stream_settings.quic_settings.security = normalizeStringValue(quicSettings.security) || 'none'
      form.stream_settings.quic_settings.key = normalizeStringValue(quicSettings.key)
      form.stream_settings.quic_settings.header_type = normalizeStringValue(quicHeader.type || quicSettings.headerType) || 'none'
      break
    case 'tcp':
      if (tcpHeader.type === 'http' || settings.headerType === 'http') {
        form.stream_settings.tcp_settings.is_http = true
        form.stream_settings.tcp_settings.http_settings.path = firstStringValue(tcpRequest.path || settings.path) || '/'
        form.stream_settings.tcp_settings.http_settings.host = joinCommaValues(preferStructuredValue(tcpRequestHeaders.Host, settings.host))
      }
      break
  }

  return form
}

const buildTransportPayload = () => {
  const network = inboundForm.stream_settings.network || 'tcp'
  const payload = { network }

  if (network === 'ws') {
    const path = normalizeStringValue(inboundForm.stream_settings.ws_settings.path) || '/'
    const host = normalizeStringValue(inboundForm.stream_settings.ws_settings.host)
    payload.path = path
    if (host) payload.host = host
    payload.ws_settings = {
      path,
      ...(host ? { headers: { Host: host } } : {})
    }
  }

  if (network === 'http') {
    const path = normalizeStringValue(inboundForm.stream_settings.http_settings.path) || '/'
    const hostList = splitCommaValues(inboundForm.stream_settings.http_settings.host)
    payload.path = path
    if (hostList[0]) payload.host = hostList[0]
    payload.http_settings = {
      path,
      ...(hostList.length ? { host: hostList } : {})
    }
  }

  if (network === 'grpc') {
    const serviceName = normalizeStringValue(inboundForm.stream_settings.grpc_settings.service_name)
    const multiMode = !!inboundForm.stream_settings.grpc_settings.multi_mode
    if (serviceName) payload.serviceName = serviceName
    payload.grpc_settings = {
      ...(serviceName ? { serviceName } : {}),
      multiMode
    }
  }

  if (network === 'quic') {
    const security = normalizeStringValue(inboundForm.stream_settings.quic_settings.security) || 'none'
    const key = normalizeStringValue(inboundForm.stream_settings.quic_settings.key)
    const headerType = normalizeStringValue(inboundForm.stream_settings.quic_settings.header_type) || 'none'

    if (security !== 'none' && !key) {
      throw new Error('QUIC 开启加密时必须填写密钥')
    }

    payload.quic_settings = {
      security,
      ...(key ? { key } : {}),
      header: {
        type: headerType
      }
    }
  }

  if (network === 'tcp' && inboundForm.stream_settings.tcp_settings.is_http) {
    const path = normalizeStringValue(inboundForm.stream_settings.tcp_settings.http_settings.path) || '/'
    const hostList = splitCommaValues(inboundForm.stream_settings.tcp_settings.http_settings.host)
    payload.headerType = 'http'
    payload.path = path
    if (hostList[0]) payload.host = hostList[0]
    payload.tcp_settings = {
      header: {
        type: 'http',
        request: {
          path: [path],
          ...(hostList.length ? { headers: { Host: hostList } } : {})
        }
      }
    }
  }

  return payload
}

const buildClientSecurityPayload = () => {
  const payload = {}
  const fingerprint = normalizeStringValue(inboundForm.stream_settings.client_settings.fingerprint)
  if (clientFingerprintVisible.value && fingerprint) {
    payload.fingerprint = fingerprint
    payload.fp = fingerprint
  }

  if (allowInsecureVisible.value) {
    payload.allowInsecure = !!inboundForm.stream_settings.client_settings.allow_insecure
  }

  return payload
}

const buildTLSCertificatePayload = () => {
  if (!tlsSettingsEnabled.value) {
    return {}
  }

  const tlsSettings = inboundForm.stream_settings.tls_settings || {}
  const domain = normalizeStringValue(tlsSettings.server_name)
  if (!domain) {
    throw new Error('启用 TLS 时请选择证书域名')
  }

  const payload = {
    security: 'tls',
    sni: domain,
    server_name: domain,
    tls_domain: domain
  }

  const alpn = splitCommaValues(tlsSettings.alpn)
  if (alpn.length) {
    payload.alpn = alpn.join(',')
  }

  return payload
}

const buildRealityPayload = () => {
  if (!realitySettingsEnabled.value) {
    return {}
  }

  const realitySettings = inboundForm.stream_settings.reality_settings || {}
  const dest = normalizeStringValue(realitySettings.dest)
  const serverNames = splitCommaValues(realitySettings.server_names)
  const privateKey = normalizeStringValue(realitySettings.private_key)
  const publicKey = normalizeStringValue(realitySettings.public_key)
  const shortIds = splitCommaValues(realitySettings.short_ids)
  const xver = Number(realitySettings.xver || 0) || 0

  if (!dest) {
    throw new Error('Reality 需要填写目标地址')
  }
  if (!serverNames.length) {
    throw new Error('Reality 需要填写至少一个 Server Name')
  }
  if (!privateKey) {
    throw new Error('Reality 需要填写私钥')
  }

  return {
    security: 'reality',
    sni: serverNames[0],
    server_name: serverNames[0],
    ...(publicKey ? { publicKey, pbk: publicKey } : {}),
    ...(shortIds[0] ? { shortId: shortIds[0], sid: shortIds[0] } : {}),
    privateKey,
    reality_settings: {
      show: !!realitySettings.show,
      dest,
      xver,
      serverNames,
      privateKey,
      shortIds: shortIds.length ? shortIds : ['']
    }
  }
}

const buildProxyPayload = () => {
  const selectedNodeShareServer = selectedNodeAddress.value
  const payload = {
    name: normalizeStringValue(inboundForm.remark),
    protocol: inboundForm.protocol,
    port: inboundForm.port,
    host: normalizeStringValue(inboundForm.listen),
    node_id: inboundForm.node_id,
    enabled: inboundForm.enable,
    remark: normalizeStringValue(inboundForm.remark),
    settings: {}
  }

  const tlsPayload = buildTLSCertificatePayload()
  const realityPayload = buildRealityPayload()
  const clientSecurityPayload = buildClientSecurityPayload()
  const transportPayload = buildTransportPayload()

  switch (inboundForm.protocol) {
    case 'trojan':
      payload.settings = {
        password: inboundForm.trojan_password,
        flow: inboundForm.trojan_flow === 'none' ? '' : inboundForm.trojan_flow,
        ...transportPayload,
        ...(selectedNodeShareServer ? { server: selectedNodeShareServer } : {}),
        ...clientSecurityPayload,
        tls: true,
        fallbacks: inboundForm.trojan_fallbacks,
        ...tlsPayload
      }
      break
    case 'vless':
      payload.settings = {
        uuid: inboundForm.vless_id,
        flow: inboundForm.vless_flow === 'none' ? '' : inboundForm.vless_flow,
        ...transportPayload,
        ...(selectedNodeShareServer ? { server: selectedNodeShareServer } : {}),
        ...clientSecurityPayload,
        ...(realitySettingsEnabled.value ? realityPayload : {
          security: tlsSettingsEnabled.value ? 'tls' : 'none',
          ...tlsPayload
        })
      }
      break
    case 'vmess':
      payload.settings = {
        uuid: inboundForm.vmess_id,
        alter_id: inboundForm.vmess_aid,
        alterId: inboundForm.vmess_aid,
        ...transportPayload,
        ...(selectedNodeShareServer ? { server: selectedNodeShareServer } : {}),
        ...clientSecurityPayload,
        security: tlsSettingsEnabled.value ? 'tls' : 'none',
        ...tlsPayload
      }
      break
    case 'shadowsocks':
      payload.settings = {
        method: inboundForm.ss_method,
        password: inboundForm.ss_password,
        network: inboundForm.network || 'tcp,udp',
        ...transportPayload,
        ...(selectedNodeShareServer ? { server: selectedNodeShareServer } : {}),
        ...(tlsSettingsEnabled.value ? tlsPayload : {})
      }
      break
    case 'dokodemo-door':
      payload.settings = {
        address: inboundForm.dokodemo_address,
        port: inboundForm.dokodemo_port,
        network: inboundForm.network || 'tcp+udp'
      }
      break
    default:
      payload.settings = {
        ...transportPayload,
        ...(selectedNodeShareServer ? { server: selectedNodeShareServer } : {}),
        ...(tlsSettingsEnabled.value ? tlsPayload : {})
      }
      break
  }

  return payload
}

// 添加fallback
const addFallback = () => {
  if (!inboundForm.trojan_fallbacks) {
    inboundForm.trojan_fallbacks = []
  }
  
  inboundForm.trojan_fallbacks.push({
    dest: '',
    port: null
  })
}

// 删除fallback
const removeFallback = (index) => {
  inboundForm.trojan_fallbacks.splice(index, 1)
}

// 保存入站
const saveInbound = async () => {
  if (!inboundFormRef.value) return
  
  await inboundFormRef.value.validate(async (valid) => {
    if (!valid) return
    
    submitting.value = true
    try {
      const submittingData = buildProxyPayload()
      const isEdit = dialogMode.value === 'edit' && inboundForm.id
      const response = isEdit
        ? await api.put(`/proxies/${inboundForm.id}`, submittingData)
        : await api.post('/proxies', submittingData)
      const result = unwrapApiData(response)

      if (result?.id || result?.port || result?.name) {
        ElMessage.success(isEdit ? '更新入站成功' : '添加入站成功')
        addInboundDialogVisible.value = false
        loadInbounds()
      } else {
        ElMessage.error(result?.message || (isEdit ? '更新入站失败' : '添加入站失败'))
      }
    } catch (error) {
      console.error('Failed to save inbound:', error)
      ElMessage.error(`添加入站失败: ${extractErrorMessage(error) || '未知错误'}`)
    } finally {
      submitting.value = false
    }
  })
}

// 编辑入站
const editInbound = async (row) => {
  dialogMode.value = 'edit'
  dialogLoading.value = true
  loadCertificates()
  resetInboundForm()
  addInboundDialogVisible.value = true

  try {
    const response = await api.get(`/proxies/${row.id}`)
    const proxyData = unwrapApiData(response)
    const normalized = normalizeProxyToInboundForm(proxyData)
    Object.assign(inboundForm, normalized)
    validateTrojanForm()
  } catch (error) {
    console.error('加载入站详情失败:', error)
    ElMessage.error(`加载入站详情失败: ${extractErrorMessage(error) || '未知错误'}`)
    addInboundDialogVisible.value = false
  } finally {
    dialogLoading.value = false
  }
}

// 切换状态
const toggleStatus = (row) => {
  const action = row.enable ? '停止' : '启动'
  ElMessageBox.confirm(`确定要${action}入站 "${row.remark}" 吗?`, '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const response = await api.post(`/proxies/${row.id}/toggle`)
      const result = unwrapApiData(response)
      if (typeof result?.enabled === 'boolean') {
        row.enable = result.enabled
        ElMessage.success(`${action}入站成功`)
      } else {
        ElMessage.error(result?.message || `${action}入站失败`)
      }
    } catch (error) {
      console.error(`Failed to ${action} inbound:`, error)
      ElMessage.error(`${action}入站失败: ${extractErrorMessage(error) || '未知错误'}`)
    }
  }).catch(() => {
    // 取消操作
  })
}

// 删除入站
const deleteInbound = (row) => {
  ElMessageBox.confirm(`确定要删除入站 "${row.remark}" 吗?`, '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const response = await api.delete(`/proxies/${row.id}`)
      const result = unwrapApiData(response)
      if (result?.message || result === '' || result == null) {
        ElMessage.success('删除入站成功')
        loadInbounds()
      } else {
        ElMessage.error(result?.message || '删除入站失败')
      }
    } catch (error) {
      console.error('Failed to delete inbound:', error)
      ElMessage.error(`删除入站失败: ${extractErrorMessage(error) || '未知错误'}`)
    }
  }).catch(() => {
    // 取消删除
  })
}

// 复制链接
const copyLink = async (row) => {
  try {
    // 获取链接
    let link = '';
    // 使用API获取实际链接
    try {
      const response = await api.get(`/proxies/${row.id}/link`)
      const result = unwrapApiData(response)
      if (result?.link) {
        link = result.link
      } else {
        throw new Error('API返回链接为空')
      }
    } catch (apiError) {
      console.error('API获取链接失败:', apiError);
      // 使用本地生成备用链接
      link = getLocalGeneratedLink(row);
      ElMessage.warning('使用本地生成的链接');
    }
    
    // 确保链接有效
    if (!link) {
      throw new Error('无法生成有效链接');
    }
    
    // 使用更可靠的剪贴板复制方法
    try {
      // 先尝试使用navigator.clipboard
      await navigator.clipboard.writeText(link);
    } catch (clipError) {
      console.error('剪贴板API失败，使用备用方法:', clipError);
      // 备用复制方法
      const textarea = document.createElement('textarea');
      textarea.value = link;
      textarea.style.position = 'fixed';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
    }
    
    ElMessage.success('链接已复制到剪贴板');
  } catch (error) {
    console.error('复制链接失败:', error);
    ElMessage.error(`复制链接失败: ${extractErrorMessage(error) || '未知错误'}`);
  }
}

// 生成本地链接(备用)
const getLocalGeneratedLink = (row) => {
  const protocol = row.protocol
  const fallbackServer = resolveLocalFallbackServer(row)
  const settings = row?.settings || {}
  let link = ''
  
  switch (protocol) {
    case 'vmess':
      {
        const fp = getSettingString(settings, 'fp', 'fingerprint')
        const alpn = getSettingString(settings, 'alpn')
        const vmessConfig = {
          v: '2',
          ps: row.remark || '',
          add: fallbackServer,
          port: String(row.port ?? ''),
          id: settings.uuid || '8ad388ff-8d82-418c-9c44-fbb3a580c1fb',
          aid: String(settings.alter_id ?? settings.alterId ?? 0),
          scy: getSettingString(settings, 'scy', 'cipher') || 'auto',
          net: settings.network || 'tcp',
          type: getSettingString(settings, 'type') || 'none',
          host: getSettingString(settings, 'host') || '',
          path: getSettingString(settings, 'path') || '',
          tls: settings.security === 'tls' || settings.tls === true ? 'tls' : '',
          sni: getSettingString(settings, 'sni', 'server_name'),
          alpn,
          fp
        }
        if (settings.allowInsecure === true) vmessConfig.allowInsecure = true
        link = `vmess://${encodeBase64UTF8(JSON.stringify(vmessConfig))}`
      }
      break;
    case 'vless':
      {
        const params = new URLSearchParams()
        params.set('encryption', 'none')
        params.set('security', getSettingString(settings, 'security') || 'none')
        params.set('type', getSettingString(settings, 'network') || 'tcp')
        const sni = getSettingString(settings, 'sni', 'server_name')
        if (sni) params.set('sni', sni)
        const flow = getSettingString(settings, 'flow')
        if (flow) params.set('flow', flow)
        const pbk = getSettingString(settings, 'pbk', 'publicKey')
        if (pbk) params.set('pbk', pbk)
        const sid = getSettingString(settings, 'sid', 'shortId')
        if (sid) params.set('sid', sid)
        const fp = getSettingString(settings, 'fp', 'fingerprint')
        if (fp) params.set('fp', fp)
        if (settings.allowInsecure === true) params.set('allowInsecure', '1')
        link = `vless://${settings.uuid || '8ad388ff-8d82-418c-9c44-fbb3a580c1fb'}@${fallbackServer}:${row.port}?${params.toString()}#${encodeURIComponent(row.remark)}`
      }
      break;
    case 'trojan':
      {
        // 获取设置，如果有的话
        let password = 'password123'
        let sni = fallbackServer

        if (settings.password) {
          password = settings.password
        }
        if (getSettingString(settings, 'sni', 'server_name')) {
          sni = getSettingString(settings, 'sni', 'server_name')
        }

        // 标准Trojan链接格式
        const params = new URLSearchParams()
        params.set('security', getSettingString(settings, 'security') || 'tls')
        params.set('sni', sni)
        const alpn = getSettingString(settings, 'alpn')
        if (alpn) params.set('alpn', alpn)
        const fp = getSettingString(settings, 'fp', 'fingerprint')
        if (fp) params.set('fp', fp)
        if (settings.allowInsecure === true) params.set('allowInsecure', '1')
        link = `trojan://${encodeURIComponent(password)}@${fallbackServer}:${row.port}?${params.toString()}#${encodeURIComponent(row.remark)}`
      }
      break;
    default:
      link = `${protocol}://${fallbackServer}:${row.port}#${encodeURIComponent(row.remark)}`
  }
  
  return link
}

// 生成随机密码
const generateRandomPassword = () => {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let result = ''
  const length = 16
  
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  
  if (inboundForm.protocol === 'shadowsocks') {
    inboundForm.ss_password = result
  } else if (inboundForm.protocol === 'trojan') {
    inboundForm.trojan_password = result
  }
  
  return result;
}

// 生成随机端口
const generateRandomPort = () => {
  // 生成10000-60000之间的随机端口
  return Math.floor(Math.random() * 50000) + 10000
}

// 生成UUID
const generateUUID = () => {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0
    const v = c === 'x' ? r : (r & 0x3 | 0x8)
    return v.toString(16)
  })
}

// 获取分享链接
const getShareLink = async (row) => {
  try {
    const response = await api.get(`/proxies/${row.id}/link`)
    const result = unwrapApiData(response)
    if (result?.link) {
      return result.link
    }
    throw new Error('API返回链接为空')
  } catch (apiError) {
    console.error('API获取链接失败:', apiError)
    ElMessage.warning('使用本地生成的链接')
    return getLocalGeneratedLink(row)
  }
}

// 显示二维码
const showQrCode = async (row) => {
  try {
    // 获取链接
    const link = await getShareLink(row)
    currentQrCodeLink.value = link
    currentQrCodeInfo.value = row
    qrCodeDialogVisible.value = true
    
    // 等待DOM更新
    await nextTick()
    
    // 生成二维码
    const qrElement = document.getElementById('qrcode-display')
    if (qrElement) {
      // 清空已有内容
      qrElement.innerHTML = ''
      
      QRCode.toCanvas(link, {
        width: 256,
        margin: 1,
        color: {
          dark: '#000000',
          light: '#ffffff'
        }
      }).then(canvas => {
        qrElement.appendChild(canvas)
      }).catch(err => {
        console.error('QRCode generation error:', err)
        ElMessage.error(`生成二维码失败: ${extractErrorMessage(err) || '未知错误'}`)
      })
    }
  } catch (error) {
    console.error('Failed to show QR code:', error)
    ElMessage.error(`生成二维码失败: ${extractErrorMessage(error) || '未知错误'}`)
  }
}

// 下载二维码
const downloadQrCode = async () => {
  try {
    if (!currentQrCodeInfo.value) return
    
    // 获取QR码 canvas
    const qrCanvas = document.getElementById('qrcode-display')?.querySelector('canvas')
    if (!qrCanvas) {
      ElMessage.error('未找到二维码画布')
      return
    }
    
    // 创建临时canvas
    const canvas = document.createElement('canvas')
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('无法获取canvas上下文')
    
    // 设置画布大小
    canvas.width = 300
    canvas.height = 350
    
    // 填充白色背景
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    
    // 绘制二维码
    ctx.drawImage(qrCanvas, 22, 20, 256, 256)
    
    // 绘制协议名称
    ctx.font = 'bold 16px Arial'
    ctx.fillStyle = '#333333'
    ctx.textAlign = 'center'
    ctx.fillText(currentQrCodeInfo.value.protocol.toUpperCase(), canvas.width / 2, 296)
    
    // 绘制备注
    ctx.font = '14px Arial'
    ctx.fillStyle = '#666666'
    ctx.fillText(currentQrCodeInfo.value.remark, canvas.width / 2, 320)
    
    // 转换为图片并下载
    const link = document.createElement('a')
    link.download = `${currentQrCodeInfo.value.protocol}-${currentQrCodeInfo.value.remark}.png`
    link.href = canvas.toDataURL('image/png')
    link.click()
    
    ElMessage.success('二维码已下载')
  } catch (error) {
    console.error('Failed to download QR code:', error)
    ElMessage.error(`下载二维码失败: ${extractErrorMessage(error) || '未知错误'}`)
  }
}
</script>

<style scoped>
.inbounds-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 18px 20px;
  background: linear-gradient(135deg, #ffffff 0%, #f7faff 100%);
  border-radius: 16px;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
  border: 1px solid rgba(148, 163, 184, 0.16);
}

.page-heading {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.title {
  font-size: 22px;
  font-weight: 700;
  color: var(--el-text-color-primary, #1f2937);
  letter-spacing: 0.02em;
}

.page-subtitle {
  font-size: 13px;
  color: var(--el-text-color-secondary, #6b7280);
}

.add-btn {
  font-size: 13px;
  padding: 10px 18px;
}

.overview-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.overview-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px 18px;
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.96) 0%, rgba(248, 250, 252, 0.96) 100%);
  border: 1px solid rgba(148, 163, 184, 0.14);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.04);
}

.overview-label {
  font-size: 12px;
  color: var(--el-text-color-secondary, #6b7280);
}

.overview-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--el-text-color-primary, #111827);
}

.overview-value.is-success {
  color: #16a34a;
}

.overview-value.is-danger {
  color: #dc2626;
}

.toolbar-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  padding: 16px 18px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid rgba(148, 163, 184, 0.14);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.04);
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar-filters .el-select {
  width: 140px;
}

.toolbar-search {
  width: 280px;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
}

.toolbar-summary {
  font-size: 12px;
  color: var(--el-text-color-secondary, #6b7280);
}

:deep(.toolbar-batch-btn.el-button) {
  height: 34px;
  margin: 0;
  padding: 0 14px !important;
  border-radius: 12px;
  border: 1px solid transparent;
  font-size: 12px;
}

:deep(.toolbar-batch-btn.el-button.is-disabled) {
  opacity: 0.5;
}

:deep(.toolbar-batch-btn--success.el-button) {
  color: #166534;
  background: #edfdf3;
  border-color: #bbf7d0;
  box-shadow: none;
}

:deep(.toolbar-batch-btn--warning.el-button) {
  color: #b45309;
  background: #fff7ed;
  border-color: #fed7aa;
  box-shadow: none;
}

:deep(.toolbar-batch-btn--danger.el-button) {
  color: #b91c1c;
  background: #fef2f2;
  border-color: #fecaca;
  box-shadow: none;
}

:deep(.inbounds-table.el-table) {
  width: 100%;
  table-layout: auto;
  background-color: var(--el-bg-color, #fff);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.05);
  font-size: 13px;
}

:deep(.inbounds-table.el-table--border) {
  border: 1px solid rgba(148, 163, 184, 0.16);
}

:deep(.inbounds-table .el-table__header th) {
  background: #f8fafc;
  font-weight: 600;
  color: #475569;
  font-size: 12px;
  letter-spacing: 0.02em;
}

:deep(.inbounds-table .el-table__cell) {
  vertical-align: top;
}

:deep(.inbounds-table .cell) {
  padding: 12px 12px;
  line-height: 1.4;
  white-space: normal;
  overflow: hidden;
  text-overflow: ellipsis;
}

:deep(.inbounds-table .el-table__body tr:hover > td) {
  background-color: #f8fbff;
}

:deep(.inbounds-table.el-table--striped .el-table__body tr.el-table__row--striped > td) {
  background-color: #fcfdff;
}

:deep(.el-button--small) {
  font-size: 12px;
}

.protocol-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.protocol-tag.vmess {
  background-color: #eaf2ff;
  color: #3568d4;
}

.protocol-tag.vless {
  background-color: #eaf8ef;
  color: #23935a;
}

.protocol-tag.trojan {
  background-color: #fff4e5;
  color: #c97710;
}

.protocol-tag.shadowsocks {
  background-color: #ffecec;
  color: #dc4c64;
}

.protocol-tag.dokodemo-door {
  background-color: #f3f4f6;
  color: #4b5563;
}

.status-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 24px;
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.status-tag.running {
  background-color: #edf9f0;
  color: #1f9d55;
}

.status-tag.stopped {
  background-color: #fef0f0;
  color: #d64545;
}

.service-cell {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.service-cell__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.service-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary, #111827);
}

.service-cell__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  font-size: 12px;
  color: var(--el-text-color-secondary, #6b7280);
}

.service-cell__server {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  color: #334155;
  background: #f8fafc;
  border: 1px solid rgba(148, 163, 184, 0.14);
  border-radius: 10px;
  padding: 6px 10px;
}

.detail-cell {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.detail-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.detail-item--status {
  align-items: center;
}

.detail-label {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary, #6b7280);
}

.detail-value {
  text-align: right;
  font-size: 13px;
  color: var(--el-text-color-primary, #334155);
  word-break: break-word;
}

.detail-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.operation-btns {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  flex-wrap: nowrap;
  gap: 6px;
  width: 100%;
}

.operation-btns .el-button {
  margin: 0 !important;
}

:deep(.operation-btns .el-button) {
  min-width: 0;
  height: 30px;
  padding: 0 11px !important;
  border-radius: 10px;
  box-shadow: none;
}

:deep(.operation-btns .row-action.el-button) {
  border: 1px solid transparent;
  font-size: 12px;
  font-weight: 600;
}

:deep(.operation-btns .row-action--primary.el-button) {
  color: #1d4ed8;
  background: #eff6ff;
  border-color: #bfdbfe;
}

:deep(.operation-btns .row-action--success.el-button) {
  color: #15803d;
  background: #f0fdf4;
  border-color: #bbf7d0;
}

:deep(.operation-btns .row-action--warning.el-button) {
  color: #b45309;
  background: #fffbeb;
  border-color: #fde68a;
}

:deep(.operation-btns .row-action--more.el-button) {
  width: 30px;
  min-width: 30px;
  padding: 0 !important;
  color: #475569;
  background: #f8fafc;
  border-color: rgba(148, 163, 184, 0.2);
}

.pagination-container {
  display: flex;
  justify-content: flex-end;
  margin-top: 4px;
}

:deep(.el-pagination) {
  padding: 10px 0;
  font-weight: normal;
}

:deep(.el-pagination button) {
  min-width: 30px;
  height: 30px;
}

:deep(.el-pagination .el-select .el-input) {
  width: 104px;
}

.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
  margin-top: 4px;
}

.cert-input {
  margin-top: 10px;
  padding: 10px;
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 4px;
  background-color: var(--el-fill-color-lighter, #f9f9f9);
}

.expiry-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.expiry-readonly {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 8px;
  background-color: var(--el-fill-color-lighter, #f9f9f9);
}

.expiry-readonly__value {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary, #303133);
}

.fallback-item {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
  border: 1px dashed var(--el-border-color, #dcdfe6);
  padding: 10px;
  border-radius: 4px;
}

.qrcode-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 10px 0 15px;
}

.qrcode {
  margin-bottom: 15px;
  padding: 10px;
  background: var(--el-bg-color, white);
  border-radius: 4px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  display: flex;
  justify-content: center;
  align-items: center;
}

.protocol-name {
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 5px;
  color: var(--el-text-color-primary, #333);
}

.remark {
  font-size: 14px;
  color: var(--el-text-color-regular, #666);
  margin-bottom: 10px;
}

@media (max-width: 1280px) {
  .overview-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .toolbar-card {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-actions {
    justify-content: flex-start;
  }

  .service-cell__header {
    flex-direction: column;
    align-items: flex-start;
  }
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .overview-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .toolbar-filters,
  .toolbar-actions {
    width: 100%;
  }

  .toolbar-search,
  .toolbar-filters .el-select {
    width: 100%;
  }

  :deep(.inbounds-table .cell) {
    padding: 12px 10px;
  }

  .detail-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .detail-value {
    text-align: left;
  }

  .operation-btns {
    justify-content: flex-start;
  }

  .pagination-container {
    justify-content: center;
    overflow-x: auto;
  }
}
</style>
