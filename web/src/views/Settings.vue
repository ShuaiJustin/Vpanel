<template>
  <div class="settings-container">
    <div class="page-header">
      <div class="page-heading">
        <h1>系统设置</h1>
        <p class="page-subtitle">
          维护面板、数据库、邮件和核心运行参数
        </p>
      </div>
    </div>
    
    <el-tabs
      v-model="activeName"
      class="settings-tabs"
      type="border-card"
    >
      <el-tab-pane
        label="服务器配置"
        name="server"
      >
        <el-form
          :model="serverForm"
          :label-width="settingsLabelWidth"
          class="settings-form"
        >
          <el-form-item label="面板监听地址">
            <el-input
              v-model="serverForm.panelListenIP"
              placeholder="0.0.0.0"
            />
            <div class="form-tips">
              默认为 0.0.0.0，代表监听所有 IP
            </div>
          </el-form-item>
          <el-form-item label="面板端口">
            <el-input-number
              v-model="serverForm.panelPort"
              :min="1"
              :max="65535"
            />
            <div class="form-tips">
              默认为 9000，修改后需要重启服务
            </div>
          </el-form-item>
          <el-form-item label="面板URL基础路径">
            <el-input
              v-model="serverForm.panelBasePath"
              placeholder="/"
            />
            <div class="form-tips">
              默认为 /，修改后需要重启服务
            </div>
          </el-form-item>
          <el-form-item label="代理服务模式">
            <el-select
              v-model="serverForm.proxyMode"
              style="width: 100%"
            >
              <el-option
                label="兼容模式"
                value="compatible"
              />
              <el-option
                label="Xray 内核"
                value="xray"
              />
              <el-option
                label="V2Ray 内核"
                value="v2ray"
              />
            </el-select>
            <div class="form-tips">
              默认为兼容模式，可同时使用 Xray 和 V2Ray 协议
            </div>
          </el-form-item>
          <el-form-item label="服务时区">
            <el-select
              v-model="serverForm.timezone"
              style="width: 100%"
            >
              <el-option
                label="Asia/Shanghai (UTC+8)"
                value="Asia/Shanghai"
              />
              <el-option
                label="UTC"
                value="UTC"
              />
              <el-option
                label="America/New_York (UTC-5)"
                value="America/New_York"
              />
              <el-option
                label="Europe/London (UTC+0)"
                value="Europe/London"
              />
            </el-select>
          </el-form-item>
          <el-divider />
          <el-form-item class="form-actions-row">
            <el-button
              type="primary"
              @click="saveServerSettings"
            >
              保存服务器配置
            </el-button>
            <el-button
              type="warning"
              @click="restartPanel"
            >
              重启面板
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      
      <el-tab-pane
        label="数据库配置"
        name="db"
      >
        <el-form
          :model="dbForm"
          :label-width="settingsLabelWidth"
          class="settings-form"
        >
          <el-form-item label="数据库类型">
            <el-select
              v-model="dbForm.dbType"
              style="width: 100%"
            >
              <el-option
                label="SQLite"
                value="sqlite"
              />
              <el-option
                label="MySQL"
                value="mysql"
              />
              <el-option
                label="PostgreSQL"
                value="postgres"
              />
            </el-select>
          </el-form-item>
          
          <template v-if="dbForm.dbType !== 'sqlite'">
            <el-form-item label="数据库服务器">
              <el-input
                v-model="dbForm.dbHost"
                placeholder="localhost"
              />
            </el-form-item>
            <el-form-item label="数据库端口">
              <el-input-number 
                v-model="dbForm.dbPort" 
                :min="1" 
                :max="65535"
                :placeholder="dbForm.dbType === 'mysql' ? '3306' : '5432'"
              />
            </el-form-item>
            <el-form-item label="数据库名称">
              <el-input
                v-model="dbForm.dbName"
                placeholder="v_panel"
              />
            </el-form-item>
            <el-form-item label="用户名">
              <el-input
                v-model="dbForm.dbUser"
                placeholder="root"
              />
            </el-form-item>
            <el-form-item label="密码">
              <el-input
                v-model="dbForm.dbPassword"
                type="password"
                placeholder="密码"
                show-password
              />
            </el-form-item>
          </template>
          
          <template v-else>
            <el-form-item label="SQLite文件路径">
              <el-input
                v-model="dbForm.sqlitePath"
                placeholder="/usr/local/v-panel/data.db"
              />
              <div class="form-tips">
                默认在程序目录下的 data.db 文件
              </div>
            </el-form-item>
          </template>
          
          <el-divider />
          <el-form-item class="form-actions-row">
            <el-button
              type="primary"
              @click="saveDbSettings"
            >
              保存数据库配置
            </el-button>
            <el-button @click="testDbConnection">
              测试连接
            </el-button>
            <el-button
              type="success"
              @click="backupDb"
            >
              备份数据库
            </el-button>
            <el-button
              type="danger"
              :loading="dbMigrating"
              @click="migrateDb"
            >
              迁移数据到目标数据库
            </el-button>
          </el-form-item>
          <el-alert
            type="warning"
            :closable="false"
            show-icon
            class="db-migrate-tip"
          >
            <template #title>
              数据库切换说明
            </template>
            "迁移数据到目标数据库" 会把当前 DB 的所有表 + 数据复制到上方填写的目标 DB。<strong>迁移完成后，本服务仍连接旧 DB</strong>——请手动在 docker-compose.yml 或环境变量中设置 <code>V_DATABASE_DRIVER</code> / <code>V_DATABASE_DSN</code>，然后重启容器才会真正切换。建议先点 备份数据库 留好快照。
          </el-alert>
        </el-form>
      </el-tab-pane>
      
      <el-tab-pane
        label="日志配置"
        name="log"
      >
        <el-form
          :model="logForm"
          :label-width="settingsLabelWidth"
          class="settings-form"
        >
          <el-form-item label="日志级别">
            <el-select
              v-model="logForm.logLevel"
              style="width: 100%"
            >
              <el-option
                label="DEBUG"
                value="debug"
              />
              <el-option
                label="INFO"
                value="info"
              />
              <el-option
                label="WARN"
                value="warn"
              />
              <el-option
                label="ERROR"
                value="error"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="日志保留天数">
            <el-input-number
              v-model="logForm.logRetentionDays"
              :min="1"
              :max="365"
            />
            <div class="form-tips">
              超过该天数的日志将被自动清理
            </div>
          </el-form-item>
          <el-form-item label="日志存储路径">
            <el-input v-model="logForm.logPath" />
            <div class="form-tips">
              默认在程序目录下的 logs 文件夹
            </div>
          </el-form-item>
          <el-form-item label="启用访问日志">
            <el-switch v-model="logForm.enableAccessLog" />
            <div class="form-tips">
              记录所有HTTP请求访问日志
            </div>
          </el-form-item>
          <el-form-item label="启用操作日志">
            <el-switch v-model="logForm.enableOperationLog" />
            <div class="form-tips">
              记录所有用户操作日志
            </div>
          </el-form-item>
          <el-divider />
          <el-form-item class="form-actions-row">
            <el-button
              type="primary"
              @click="saveLogSettings"
            >
              保存日志配置
            </el-button>
            <el-button
              type="danger"
              @click="clearLogs"
            >
              清理日志
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane
        label="邮箱配置"
        name="email"
      >
        <el-form
          :model="emailForm"
          :label-width="settingsLabelWidth"
          class="settings-form"
        >
          <el-alert
            title="邮箱配置用于注册验证、密码重置和系统提醒"
            type="info"
            :closable="false"
            show-icon
            style="margin-bottom: 20px"
          />

          <el-form-item label="SMTP 服务器">
            <el-input
              v-model="emailForm.host"
              placeholder="例如：smtp.qq.com"
            />
          </el-form-item>
          <el-form-item label="SMTP 端口">
            <el-input-number
              v-model="emailForm.port"
              :min="1"
              :max="65535"
            />
            <div class="form-tips">
              常见端口为 465 / 587 / 25
            </div>
          </el-form-item>
          <el-form-item label="SMTP 用户名">
            <el-input
              v-model="emailForm.user"
              placeholder="通常为邮箱地址"
            />
          </el-form-item>
          <el-form-item label="发件邮箱">
            <el-input
              v-model="emailForm.from"
              placeholder="留空则默认使用 SMTP 用户名"
            />
            <div class="form-tips">
              用于注册验证、重置密码等系统邮件的发件人地址
            </div>
          </el-form-item>
          <el-form-item label="告警收件邮箱">
            <el-input
              v-model="emailForm.alertEmail"
              placeholder="留空则默认发到 SMTP 用户名"
            />
            <div class="form-tips">
              用于节点异常、系统提醒等后台告警邮件
            </div>
          </el-form-item>
          <el-form-item label="SMTP 密码">
            <el-input
              v-model="emailForm.password"
              type="password"
              show-password
              :placeholder="emailForm.passwordConfigured ? '已配置，留空则保持不变' : '请输入 SMTP 密码或授权码'"
            />
          </el-form-item>
          <el-form-item label="测试收件邮箱">
            <el-input
              v-model="emailForm.testTo"
              placeholder="留空则发送到告警收件邮箱或 SMTP 用户名"
            />
          </el-form-item>

          <el-divider />
          <el-form-item class="form-actions-row">
            <el-button
              type="primary"
              :loading="emailForm.saving"
              @click="saveEmailSettings"
            >
              保存邮箱配置
            </el-button>
            <el-button
              :loading="emailForm.loading"
              @click="loadEmailSettings"
            >
              刷新
            </el-button>
            <el-button
              type="success"
              :loading="emailForm.testing"
              @click="testEmailSettings"
            >
              发送测试邮件
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane
        label="支付设置"
        name="payment"
      >
        <el-alert
          title="支付/充值配置已迁移到商业化管理"
          description="为避免系统设置和商业化管理出现两套支付配置入口，这里只保留跳转入口。请统一在“商业化管理 -> 支付/充值配置”中维护商户参数。"
          type="info"
          :closable="false"
          show-icon
        />
        <div style="margin-top: 16px;">
          <el-button
            type="primary"
            @click="openPaymentSettingsPage"
          >
            前往商业化管理中的支付/充值配置
          </el-button>
        </div>
      </el-tab-pane>

      <el-tab-pane
        label="管理员配置"
        name="admin"
      >
        <el-form
          :model="adminForm"
          :label-width="settingsLabelWidth"
          class="settings-form"
        >
          <el-alert
            title="管理员账号安全提示"
            type="warning"
            description="修改管理员密码后，当前会话将被注销，需要重新登录。请确保记住新密码，否则可能无法访问系统。"
            show-icon
            :closable="false"
            style="margin-bottom: 20px"
          />
          
          <el-form-item label="管理员用户名">
            <el-input
              v-model="adminForm.username"
              placeholder="admin"
              :disabled="true"
            />
            <div class="form-tips">
              默认管理员用户名不可修改
            </div>
          </el-form-item>
          <el-form-item label="当前密码">
            <el-input
              v-model="adminForm.currentPassword"
              type="password"
              placeholder="当前密码"
              show-password
            />
          </el-form-item>
          <el-form-item label="新密码">
            <el-input
              v-model="adminForm.newPassword"
              type="password"
              placeholder="新密码"
              show-password
            />
          </el-form-item>
          <el-form-item label="确认新密码">
            <el-input
              v-model="adminForm.confirmPassword"
              type="password"
              placeholder="确认新密码"
              show-password
            />
          </el-form-item>
          
          <el-divider />
          <el-form-item class="form-actions-row">
            <el-button
              type="primary"
              @click="changeAdminPassword"
            >
              修改密码
            </el-button>
            <el-button
              type="warning"
              @click="resetAdminPassword"
            >
              重置为默认密码
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      
      <el-tab-pane
        label="安全设置"
        name="security"
      >
        <el-form
          :model="securityForm"
          :label-width="settingsLabelWidth"
          class="settings-form security-form"
        >
          <el-form-item
            label="会话超时时间"
            class="security-inline-item"
          >
            <div class="security-inline-control">
              <el-input-number
                v-model="securityForm.sessionTimeout"
                :min="5"
                :max="1440"
              />
            </div>
            <div class="form-tips">
              单位：分钟，超过该时间未操作将自动注销
            </div>
          </el-form-item>
          <el-form-item label="启用IP白名单">
            <el-switch v-model="securityForm.enableIpWhitelist" />
          </el-form-item>
          <el-form-item
            v-if="securityForm.enableIpWhitelist"
            label="IP白名单"
          >
            <el-input 
              v-model="securityForm.ipWhitelist" 
              type="textarea" 
              :rows="4"
              placeholder="每行一个IP地址，支持CIDR格式，如：192.168.1.0/24"
            />
          </el-form-item>
          <el-form-item
            label="登录失败锁定"
            class="security-inline-item"
          >
            <div class="security-inline-control">
              <el-switch v-model="securityForm.enableLoginLock" />
            </div>
            <div class="form-tips">
              连续登录失败将暂时锁定账号
            </div>
          </el-form-item>
          <el-form-item
            v-if="securityForm.enableLoginLock"
            label="失败尝试次数"
          >
            <el-input-number
              v-model="securityForm.maxLoginAttempts"
              :min="3"
              :max="10"
            />
          </el-form-item>
          <el-form-item
            v-if="securityForm.enableLoginLock"
            label="锁定时间(分钟)"
          >
            <el-input-number
              v-model="securityForm.lockDuration"
              :min="5"
              :max="60"
            />
          </el-form-item>
          
          <el-divider />
          <el-form-item class="form-actions-row">
            <el-button
              type="primary"
              :loading="securityState.saving"
              @click="saveSecuritySettings"
            >
              保存安全设置
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      
      <!-- 新增协议管理标签页 -->
      <el-tab-pane
        label="协议管理"
        name="protocol"
      >
        <el-form class="settings-form">
          <el-form-item
            label="支持的协议"
            :label-width="settingsLabelWidth"
          >
            <el-descriptions
              :column="1"
              border
              size="medium"
            >
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="protocolSettings.enableTrojan"
                    active-text="启用 Trojan 协议"
                    :disabled="disableProtocolSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>Trojan 协议：基于 TLS 的轻量级协议，伪装成 HTTPS 流量。</p>
                  <el-tag
                    v-if="protocolSettings.enableTrojan"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
              
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="protocolSettings.enableVMess"
                    active-text="启用 VMess 协议"
                    :disabled="disableProtocolSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>VMess 协议：V2Ray 的核心传输协议，支持多种传输层。</p>
                  <el-tag
                    v-if="protocolSettings.enableVMess"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
              
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="protocolSettings.enableVLESS"
                    active-text="启用 VLESS 协议"
                    :disabled="disableProtocolSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>VLESS 协议：轻量化的 VMess 协议，去除不必要的加密。</p>
                  <el-tag
                    v-if="protocolSettings.enableVLESS"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
              
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="protocolSettings.enableShadowsocks"
                    active-text="启用 Shadowsocks 协议"
                    :disabled="disableProtocolSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>Shadowsocks 协议：经典的加密代理协议。</p>
                  <el-tag
                    v-if="protocolSettings.enableShadowsocks"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
              
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="protocolSettings.enableSocks"
                    active-text="启用 SOCKS 协议"
                    :disabled="disableProtocolSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>SOCKS 协议：标准代理协议，支持 TCP/UDP。</p>
                  <el-tag
                    v-if="protocolSettings.enableSocks"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
              
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="protocolSettings.enableHTTP"
                    active-text="启用 HTTP 协议"
                    :disabled="disableProtocolSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>HTTP 协议：基础代理协议，明文传输。</p>
                  <el-tag
                    v-if="protocolSettings.enableHTTP"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
            </el-descriptions>
          </el-form-item>
          
          <el-divider content-position="left">
            自动生成节点
          </el-divider>

          <el-form-item
            label="协议优先级"
            :label-width="settingsLabelWidth"
          >
            <div class="auto-proxy-priority">
              <div
                v-for="(protocol, index) in autoProxySettings.protocolPriority"
                :key="index"
                class="priority-row"
              >
                <span class="priority-index">第 {{ index + 1 }} 优先</span>
                <el-select
                  v-model="autoProxySettings.protocolPriority[index]"
                  class="priority-select"
                  :disabled="protocolsLoading"
                  @change="ensureAutoProxyPriority"
                >
                  <el-option
                    v-for="option in autoProxyProtocolOptions"
                    :key="option.value"
                    :label="option.label"
                    :value="option.value"
                  />
                </el-select>
              </div>
            </div>
            <div class="form-tips">
              新用户或重建自动代理时按这个顺序选择协议；节点只配置单一协议时仍使用该协议。
            </div>
          </el-form-item>

          <el-divider content-position="left">
            传输层设置
          </el-divider>
          
          <el-form-item
            label="支持的传输层"
            :label-width="settingsLabelWidth"
          >
            <el-descriptions
              :column="1"
              border
              size="medium"
            >
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="transportSettings.enableTCP"
                    active-text="启用 TCP 传输"
                    :disabled="disableTransportSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>TCP 传输：最基础的传输方式。</p>
                  <el-tag
                    v-if="transportSettings.enableTCP"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
              
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="transportSettings.enableWebSocket"
                    active-text="启用 WebSocket 传输"
                    :disabled="disableTransportSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>WebSocket 传输：基于HTTP协议的持久化连接，兼容性好。</p>
                  <el-tag
                    v-if="transportSettings.enableWebSocket"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
              
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="transportSettings.enableHTTP2"
                    active-text="启用 HTTP/2 传输"
                    :disabled="disableTransportSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>HTTP/2 传输：新一代HTTP协议，多路复用，需启用TLS。</p>
                  <el-tag
                    v-if="transportSettings.enableHTTP2"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
              
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="transportSettings.enableGRPC"
                    active-text="启用 gRPC 传输"
                    :disabled="disableTransportSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>gRPC 传输：基于HTTP/2的高性能RPC框架，抗干扰能力强。</p>
                  <el-tag
                    v-if="transportSettings.enableGRPC"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
              
              <el-descriptions-item>
                <template #label>
                  <el-switch
                    v-model="transportSettings.enableQUIC"
                    active-text="启用 QUIC 传输"
                    :disabled="disableTransportSwitch"
                  />
                </template>
                <div class="protocol-description">
                  <p>QUIC 传输：基于UDP的传输层协议，低延迟。</p>
                  <el-tag
                    v-if="transportSettings.enableQUIC"
                    type="success"
                    size="small"
                  >
                    已启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="danger"
                    size="small"
                  >
                    已禁用
                  </el-tag>
                </div>
              </el-descriptions-item>
            </el-descriptions>
          </el-form-item>
          
          <el-divider />

          <el-form-item>
            <el-button
              type="primary"
              :loading="protocolsLoading"
              @click="saveProtocolSettings"
            >
              保存协议配置
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '@/stores/user'
import api from '@/api/index'
import { useViewport } from '@/composables/useViewport'
import { extractErrorMessage } from '@/utils/entitlement'

// store
const userStore = useUserStore()
const router = useRouter()
const { isMobile } = useViewport()

// 当前活动标签页
const activeName = ref('server')
const settingsLabelWidth = computed(() => (isMobile.value ? '100%' : '168px'))

// 表单数据
const serverForm = reactive({
  panelListenIP: '0.0.0.0',
  panelPort: 9000,
  panelBasePath: '/',
  proxyMode: 'compatible',
  timezone: 'Asia/Shanghai'
})

const dbForm = reactive({
  dbType: 'sqlite',
  dbHost: 'localhost',
  dbPort: 3306,
  dbName: 'v_panel',
  dbUser: 'root',
  dbPassword: '',
  sqlitePath: '/usr/local/v-panel/data.db'
})

const logForm = reactive({
  logLevel: 'info',
  logRetentionDays: 30,
  logPath: '/usr/local/v-panel/logs',
  enableAccessLog: true,
  enableOperationLog: true
})

const emailForm = reactive({
  loading: false,
  saving: false,
  testing: false,
  host: '',
  port: 587,
  user: '',
  from: '',
  alertEmail: '',
  password: '',
  passwordConfigured: false,
  testTo: ''
})

const paymentForm = reactive({
  loading: false,
  saving: false,
  alipayEnabled: false,
  alipayAppId: '',
  alipayPrivateKey: '',
  alipayPrivateKeyConfigured: false,
  alipayPublicKey: '',
  alipayNotifyUrl: '',
  alipayReturnUrl: '',
  alipaySandbox: false,
  wechatEnabled: false,
  wechatAppId: '',
  wechatMchId: '',
  wechatApiKey: '',
  wechatApiKeyConfigured: false,
  wechatNotifyUrl: '',
  wechatSandbox: false
})

const adminForm = reactive({
  username: 'admin',
  currentPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const securityForm = reactive({
  sessionTimeout: 1440,
  enableIpWhitelist: false,
  ipWhitelist: '',
  enableLoginLock: false,
  maxLoginAttempts: 5,
  lockDuration: 10
})

const securityState = reactive({
  loading: false,
  saving: false
})

const applyServerSettings = (settings) => {
  serverForm.panelListenIP = settings?.panel_access_ip || '0.0.0.0'
  serverForm.panelPort = settings?.panel_port || 9000
  serverForm.panelBasePath = settings?.panel_base_path || '/'
  serverForm.proxyMode = settings?.proxy_mode || 'compatible'
  serverForm.timezone = settings?.timezone || 'Asia/Shanghai'
}

const applyDbSettings = (settings) => {
  dbForm.dbType = settings?.db_type || 'sqlite'
  dbForm.dbHost = settings?.db_host || 'localhost'
  dbForm.dbPort = settings?.db_port || (dbForm.dbType === 'postgres' ? 5432 : 3306)
  dbForm.dbName = settings?.db_name || 'v_panel'
  dbForm.dbUser = settings?.db_user || 'root'
  dbForm.dbPassword = ''
  dbForm.sqlitePath = settings?.sqlite_path || './data/v.db'
}

const applyLogSettings = (settings) => {
  logForm.logLevel = settings?.log_level || 'info'
  logForm.logRetentionDays = settings?.log_retention_days || 30
  logForm.logPath = settings?.log_path || './logs'
  logForm.enableAccessLog = settings?.enable_access_log ?? true
  logForm.enableOperationLog = settings?.enable_operation_log ?? true
}

const loadGeneralSettings = async () => {
  const response = await api.get('/settings')
  const settings = response?.data || {}
  applyServerSettings(settings)
  applyDbSettings(settings)
  applyLogSettings(settings)
  return settings
}

// 协议设置
const protocolSettings = reactive({
  enableTrojan: true,
  enableVMess: true,
  enableVLESS: true,
  enableShadowsocks: true,
  enableSocks: false,
  enableHTTP: false
})

const defaultAutoProxyPriority = ['trojan', 'vmess', 'vless', 'shadowsocks']
const autoProxyProtocolOptions = [
  { label: 'Trojan', value: 'trojan' },
  { label: 'VMess', value: 'vmess' },
  { label: 'VLESS', value: 'vless' },
  { label: 'Shadowsocks', value: 'shadowsocks' }
]
const autoProxySettings = reactive({
  protocolPriority: [...defaultAutoProxyPriority]
})

// 传输层设置
const transportSettings = reactive({
  enableTCP: true,
  enableWebSocket: true,
  enableHTTP2: true,
  enableGRPC: true,
  enableQUIC: false
})

// 状态控制
const protocolsLoading = ref(false)
const disableProtocolSwitch = computed(() => protocolsLoading.value)
const disableTransportSwitch = computed(() => protocolsLoading.value)

const normalizeAutoProxyPriority = (priority = []) => {
  const allowed = new Set(defaultAutoProxyPriority)
  const seen = new Set()
  const normalized = []
  for (const protocol of priority) {
    const value = String(protocol || '').trim().toLowerCase()
    if (!allowed.has(value) || seen.has(value)) continue
    seen.add(value)
    normalized.push(value)
  }
  for (const protocol of defaultAutoProxyPriority) {
    if (seen.has(protocol)) continue
    normalized.push(protocol)
  }
  return normalized.slice(0, defaultAutoProxyPriority.length)
}

const ensureAutoProxyPriority = () => {
  autoProxySettings.protocolPriority.splice(
    0,
    autoProxySettings.protocolPriority.length,
    ...normalizeAutoProxyPriority(autoProxySettings.protocolPriority)
  )
}

const applyProtocolSettings = (settings = {}) => {
  const protocols = settings?.protocols || {}
  protocolSettings.enableTrojan = protocols.trojan ?? true
  protocolSettings.enableVMess = protocols.vmess ?? true
  protocolSettings.enableVLESS = protocols.vless ?? true
  protocolSettings.enableShadowsocks = protocols.shadowsocks ?? true
  protocolSettings.enableSocks = protocols.socks ?? false
  protocolSettings.enableHTTP = protocols.http ?? false

  const transports = settings?.transports || {}
  transportSettings.enableTCP = transports.tcp ?? true
  transportSettings.enableWebSocket = transports.ws ?? true
  transportSettings.enableHTTP2 = transports.http2 ?? true
  transportSettings.enableGRPC = transports.grpc ?? true
  transportSettings.enableQUIC = transports.quic ?? false
}

const applyAutoProxySettings = (settings = {}) => {
  autoProxySettings.protocolPriority.splice(
    0,
    autoProxySettings.protocolPriority.length,
    ...normalizeAutoProxyPriority(settings?.protocol_priority || settings?.protocolPriority)
  )
}

const loadProtocolSettings = async () => {
  protocolsLoading.value = true
  try {
    const [protocolResponse, autoProxyResponse] = await Promise.all([
      api.get('/settings/protocols'),
      api.get('/settings/auto-proxy')
    ])
    applyProtocolSettings(protocolResponse || {})
    applyAutoProxySettings(autoProxyResponse || {})
  } catch (error) {
    console.error('Failed to load protocol settings:', error)
    ElMessage.error('加载协议配置失败')
  } finally {
    protocolsLoading.value = false
  }
}

// 初始化
onMounted(async () => {
  try {
    await Promise.allSettled([
      loadGeneralSettings(),
      loadSecuritySettings(),
      loadEmailSettings(),
      loadPaymentSettings(),
      loadProtocolSettings()
    ]);
  } catch (error) {
    console.error('Failed to load initial settings:', error);
    ElMessage.error('加载设置失败，请刷新页面重试');
  }
});

// 方法
const saveServerSettings = async () => {
  try {
    const response = await api.put('/settings', {
      panel_access_ip: serverForm.panelListenIP.trim(),
      panel_port: serverForm.panelPort,
      panel_base_path: serverForm.panelBasePath.trim() || '/',
      proxy_mode: serverForm.proxyMode,
      timezone: serverForm.timezone
    })
    applyServerSettings(response?.data || {})
    ElMessage.success('服务器配置保存成功')
  } catch (error) {
    ElMessage.error('保存失败：' + (extractErrorMessage(error) || '未知错误'))
  }
}

const restartPanel = () => {
  ElMessageBox.confirm(
    '确定要重启面板吗？这将暂时中断所有连接。',
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )
  .then(async () => {
    try {
      await api.post('/admin/system/restart-panel')
      ElMessage.success('面板重启指令已发送，请稍后重新访问当前页面')
    } catch (error) {
      ElMessage.error('重启失败：' + (extractErrorMessage(error) || '未知错误'))
    }
  })
  .catch(() => {
    ElMessage.info('已取消重启')
  })
}

const saveDbSettings = async () => {
  try {
    const response = await api.put('/settings', {
      db_type: dbForm.dbType,
      db_host: dbForm.dbHost.trim(),
      db_port: dbForm.dbPort,
      db_name: dbForm.dbName.trim(),
      db_user: dbForm.dbUser.trim(),
      db_password: dbForm.dbPassword,
      sqlite_path: dbForm.sqlitePath.trim()
    })
    applyDbSettings(response?.data || {})
    ElMessage.success('数据库配置保存成功')
  } catch (error) {
    ElMessage.error('保存失败：' + (extractErrorMessage(error) || '未知错误'))
  }
}

const testDbConnection = async () => {
  try {
    await api.post('/settings/test-db', {
      db_type: dbForm.dbType,
      db_host: dbForm.dbHost.trim(),
      db_port: dbForm.dbPort,
      db_name: dbForm.dbName.trim(),
      db_user: dbForm.dbUser.trim(),
      db_password: dbForm.dbPassword,
      sqlite_path: dbForm.sqlitePath.trim()
    })
    ElMessage.success('数据库连接测试成功')
  } catch (error) {
    ElMessage.error('连接测试失败：' + (extractErrorMessage(error) || '未知错误'))
  }
}

const backupDb = async () => {
  try {
    const response = await api.post('/settings/backup-db')
    const backupPath = response?.backup_path || response?.data?.backup_path
    ElMessage.success(backupPath ? `数据库备份成功：${backupPath}` : '数据库备份成功')
  } catch (error) {
    ElMessage.error('备份失败：' + (extractErrorMessage(error) || '未知错误'))
  }
}

const dbMigrating = ref(false)
const migrateDb = async () => {
  if (dbForm.dbType === 'sqlite') {
    if (!dbForm.sqlitePath.trim()) {
      return ElMessage.warning('请填写目标 SQLite 路径')
    }
  } else if (!dbForm.dbHost.trim() || !dbForm.dbName.trim() || !dbForm.dbUser.trim()) {
    return ElMessage.warning('请填写目标数据库主机 / 库名 / 用户名')
  }

  try {
    await ElMessageBox.confirm(
      `即将把当前数据库的所有数据复制到目标 ${dbForm.dbType}。这是单向操作，目标库现有数据会被覆盖。建议先点"备份数据库"。`,
      '确认迁移',
      {
        confirmButtonText: '开始迁移',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
  } catch {
    return
  }

  dbMigrating.value = true
  try {
    const response = await api.post('/settings/migrate-db', {
      db_type: dbForm.dbType,
      db_host: dbForm.dbHost.trim(),
      db_port: dbForm.dbPort,
      db_name: dbForm.dbName.trim(),
      db_user: dbForm.dbUser.trim(),
      db_password: dbForm.dbPassword,
      sqlite_path: dbForm.sqlitePath.trim(),
      confirm: true,
    })

    const data = response?.data || response || {}
    const cutover = data.cutover_env || {}
    const envBlock = Object.entries(cutover)
      .map(([k, v]) => `${k}=${v}`)
      .join('\n')

    ElMessageBox.alert(
      `<div>
        <p><strong>迁移完成</strong>，共复制 ${data?.report?.total_rows ?? '?'} 行，跨 ${data?.report?.table_count ?? '?'} 张表。</p>
        <p style="margin-top:12px">本服务仍在使用旧数据库。请在容器环境变量或 docker-compose.yml 中设置：</p>
        <pre style="margin:8px 0;padding:10px;background:#f4f4f5;border-radius:6px;white-space:pre-wrap;word-break:break-all">${envBlock}</pre>
        <p>然后重启容器，新连接才会生效。</p>
      </div>`,
      '迁移成功',
      { dangerouslyUseHTMLString: true, confirmButtonText: '我已记下' }
    )
  } catch (error) {
    ElMessage.error('迁移失败：' + (extractErrorMessage(error) || '未知错误'))
  } finally {
    dbMigrating.value = false
  }
}

const saveLogSettings = async () => {
  try {
    const response = await api.put('/settings', {
      log_level: logForm.logLevel,
      log_retention_days: logForm.logRetentionDays,
      log_path: logForm.logPath.trim(),
      enable_access_log: logForm.enableAccessLog,
      enable_operation_log: logForm.enableOperationLog
    })
    applyLogSettings(response?.data || {})
    ElMessage.success('日志配置保存成功')
  } catch (error) {
    ElMessage.error('保存失败：' + (extractErrorMessage(error) || '未知错误'))
  }
}

const applyEmailSettings = (settings) => {
  emailForm.host = settings?.smtp_host || ''
  emailForm.port = settings?.smtp_port || 587
  emailForm.user = settings?.smtp_user || ''
  emailForm.from = settings?.smtp_from || ''
  emailForm.alertEmail = settings?.smtp_alert_email || ''
  emailForm.password = ''
  emailForm.passwordConfigured = settings?.smtp_password_configured ?? false
}

const loadEmailSettings = async () => {
  emailForm.loading = true
  try {
    const response = await api.get('/settings')
    applyEmailSettings(response?.data || {})
  } catch (error) {
    console.error('Failed to load email settings:', error)
    ElMessage.error('加载邮箱配置失败')
  } finally {
    emailForm.loading = false
  }
}

const saveEmailSettings = async () => {
  emailForm.saving = true
  try {
    const payload = {
      smtp_host: emailForm.host.trim(),
      smtp_port: emailForm.port,
      smtp_user: emailForm.user.trim(),
      smtp_from: emailForm.from.trim(),
      smtp_alert_email: emailForm.alertEmail.trim()
    }

    if (emailForm.password.trim()) {
      payload.smtp_password = emailForm.password.trim()
    }

    const response = await api.put('/settings', payload)
    applyEmailSettings(response?.data || {})
    ElMessage.success('邮箱配置保存成功')
  } catch (error) {
    console.error('Failed to save email settings:', error)
    ElMessage.error(extractErrorMessage(error) || '保存邮箱配置失败')
  } finally {
    emailForm.saving = false
  }
}

const testEmailSettings = async () => {
  emailForm.testing = true
  try {
    await api.post('/settings/test-email', {
      to: emailForm.testTo.trim()
    })
    ElMessage.success('测试邮件已发送，请检查收件箱')
  } catch (error) {
    console.error('Failed to send test email:', error)
    ElMessage.error(extractErrorMessage(error) || '发送测试邮件失败')
  } finally {
    emailForm.testing = false
  }
}

const applyPaymentSettings = (settings) => {
  paymentForm.alipayEnabled = settings?.payment_alipay_enabled ?? false
  paymentForm.alipayAppId = settings?.payment_alipay_app_id || ''
  paymentForm.alipayPrivateKey = ''
  paymentForm.alipayPrivateKeyConfigured = settings?.payment_alipay_private_key_configured ?? false
  paymentForm.alipayPublicKey = settings?.payment_alipay_public_key || ''
  paymentForm.alipayNotifyUrl = settings?.payment_alipay_notify_url || ''
  paymentForm.alipayReturnUrl = settings?.payment_alipay_return_url || ''
  paymentForm.alipaySandbox = settings?.payment_alipay_sandbox ?? false
  paymentForm.wechatEnabled = settings?.payment_wechat_enabled ?? false
  paymentForm.wechatAppId = settings?.payment_wechat_app_id || ''
  paymentForm.wechatMchId = settings?.payment_wechat_mch_id || ''
  paymentForm.wechatApiKey = ''
  paymentForm.wechatApiKeyConfigured = settings?.payment_wechat_api_key_configured ?? false
  paymentForm.wechatNotifyUrl = settings?.payment_wechat_notify_url || ''
  paymentForm.wechatSandbox = settings?.payment_wechat_sandbox ?? false
}

const loadPaymentSettings = async () => {
  paymentForm.loading = true
  try {
    const response = await api.get('/settings')
    applyPaymentSettings(response?.data || {})
  } catch (error) {
    console.error('Failed to load payment settings:', error)
    ElMessage.error('加载支付设置失败')
  } finally {
    paymentForm.loading = false
  }
}

const openPaymentSettingsPage = () => {
  window.location.href = '/admin/payment-settings'
}

const clearLogs = () => {
  ElMessageBox.confirm(
    '确定要清理所有日志吗？此操作不可恢复。',
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )
  .then(async () => {
    try {
      await api.post('/logs/cleanup', {
        retention_days: logForm.logRetentionDays
      })
      ElMessage.success('日志清理成功')
    } catch (error) {
      ElMessage.error('清理失败：' + (extractErrorMessage(error) || '未知错误'))
    }
  })
  .catch(() => {
    ElMessage.info('已取消清理')
  })
}

const changeAdminPassword = async () => {
  // 表单验证
  if (!adminForm.currentPassword) {
    return ElMessage.warning('请输入当前密码')
  }
  if (!adminForm.newPassword) {
    return ElMessage.warning('请输入新密码')
  }
  if (adminForm.newPassword.length < 6) {
    return ElMessage.warning('新密码长度不能少于6个字符')
  }
  if (adminForm.newPassword !== adminForm.confirmPassword) {
    return ElMessage.warning('两次输入的密码不一致')
  }
  
  ElMessageBox.confirm(
    '修改密码后，当前会话将被注销，需要重新登录。是否继续？',
    '提示',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )
  .then(async () => {
    try {
      await api.put('/auth/password', {
        old_password: adminForm.currentPassword,
        new_password: adminForm.newPassword
      })
      ElMessage.success('密码修改成功，请重新登录')
      
      // 清空表单
      adminForm.currentPassword = ''
      adminForm.newPassword = ''
      adminForm.confirmPassword = ''
      
      // 注销当前会话
      setTimeout(async () => {
        await userStore.logout()
        window.location.href = '/user/login'
      }, 1500)
    } catch (error) {
      ElMessage.error('修改失败：' + (extractErrorMessage(error) || '未知错误'))
    }
  })
  .catch(() => {
    ElMessage.info('已取消修改')
  })
}

const resetAdminPassword = () => {
  ElMessageBox.confirm(
    '系统密码重置请在 用户管理 中找到管理员账户并执行重置。是否跳转到用户管理页？',
    '提示',
    {
      confirmButtonText: '跳转',
      cancelButtonText: '取消',
      type: 'info'
    }
  )
  .then(() => {
    router.push('/admin/users?role=admin')
  })
  .catch(() => {})
}

const applySecuritySettings = (settings) => {
  securityForm.sessionTimeout = settings?.session_timeout || 1440
  securityForm.enableIpWhitelist = settings?.enable_ip_whitelist ?? false
  securityForm.ipWhitelist = settings?.ip_whitelist || ''
  securityForm.enableLoginLock = settings?.enable_login_lock ?? false
  securityForm.maxLoginAttempts = settings?.max_login_attempts || 5
  securityForm.lockDuration = settings?.lock_duration || 10
}

const loadSecuritySettings = async () => {
  securityState.loading = true
  try {
    const response = await api.get('/settings')
    applySecuritySettings(response?.data || {})
  } catch (error) {
    console.error('Failed to load security settings:', error)
    ElMessage.error('加载安全设置失败')
  } finally {
    securityState.loading = false
  }
}

const saveSecuritySettings = async () => {
  securityState.saving = true
  try {
    const payload = {
      session_timeout: securityForm.sessionTimeout,
      enable_ip_whitelist: securityForm.enableIpWhitelist,
      ip_whitelist: securityForm.ipWhitelist.trim(),
      enable_login_lock: securityForm.enableLoginLock,
      max_login_attempts: securityForm.maxLoginAttempts,
      lock_duration: securityForm.lockDuration
    }

    const response = await api.put('/settings', payload)
    applySecuritySettings(response?.data || {})
    ElMessage.success('安全设置保存成功')
  } catch (error) {
    console.error('Failed to save security settings:', error)
    ElMessage.error(extractErrorMessage(error) || '保存安全设置失败')
  } finally {
    securityState.saving = false
  }
}


// 保存协议设置
const saveProtocolSettings = async () => {
  protocolsLoading.value = true
  try {
    ensureAutoProxyPriority()
    await Promise.all([
      api.post('/settings/protocols', {
        protocols: {
          trojan: protocolSettings.enableTrojan,
          vmess: protocolSettings.enableVMess,
          vless: protocolSettings.enableVLESS,
          shadowsocks: protocolSettings.enableShadowsocks,
          socks: protocolSettings.enableSocks,
          http: protocolSettings.enableHTTP
        },
        transports: {
          tcp: transportSettings.enableTCP,
          ws: transportSettings.enableWebSocket,
          http2: transportSettings.enableHTTP2,
          grpc: transportSettings.enableGRPC,
          quic: transportSettings.enableQUIC
        }
      }),
      api.post('/settings/auto-proxy', {
        protocol_priority: autoProxySettings.protocolPriority
      })
    ])
    ElMessage.success('协议配置保存成功')
  } catch (error) {
    console.error('Failed to save protocol settings:', error)
    ElMessage.error('保存协议配置失败: ' + (extractErrorMessage(error) || '未知错误'))
  } finally {
    protocolsLoading.value = false
  }
}


</script>

<style scoped>
.settings-container {
  padding: 20px;
}

.settings-tabs :deep(.el-tabs__header) {
  margin-bottom: 22px;
}

.settings-tabs :deep(.el-tabs__nav-wrap) {
  overflow-x: auto;
  scrollbar-width: thin;
}

.settings-tabs :deep(.el-tabs__nav-scroll) {
  overflow-x: auto;
}

.settings-tabs :deep(.el-tabs__nav) {
  flex-wrap: nowrap;
}

.settings-tabs :deep(.el-tabs__item) {
  min-width: max-content;
  padding-inline: 24px;
}

.settings-tabs :deep(.el-tabs__content) {
  padding-top: 8px;
}

.settings-form {
  max-width: 800px;
  margin-top: 20px;
}

.form-tips {
  display: block;
  width: 100%;
  flex: 0 0 100%;
  font-size: 12px;
  line-height: 1.6;
  color: #909399;
  margin-top: 5px;
}

.settings-form :deep(.el-form-item__label) {
  width: 168px !important;
  white-space: nowrap;
  word-break: keep-all;
}

.settings-form :deep(.el-form-item__content) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  column-gap: 12px;
  row-gap: 6px;
  min-width: 0;
}

.settings-form :deep(.el-input),
.settings-form :deep(.el-select),
.settings-form :deep(.el-textarea),
.settings-form :deep(.el-date-editor),
.settings-form :deep(.el-cascader),
.settings-form :deep(.el-descriptions) {
  width: 100%;
  max-width: 100%;
}

.settings-form :deep(.el-divider__text) {
  padding: 0 14px;
  border-radius: 999px;
  background: var(--admin-surface-strong);
  color: var(--admin-title);
  font-weight: 700;
  letter-spacing: 0.01em;
}

.settings-form :deep(.form-actions-row .el-form-item__content) {
  width: 100%;
  justify-content: flex-start;
  align-items: center;
  gap: 12px;
}

.settings-form :deep(.form-actions-row .el-button) {
  min-width: 160px;
  min-height: 44px;
  padding-inline: 20px;
}

.settings-form :deep(.el-input-number) {
  width: min(320px, 100%);
  max-width: 100%;
  flex-shrink: 0;
  overflow: hidden;
}

.settings-form :deep(.el-input-number .el-input__wrapper) {
  border-radius: inherit;
}

.security-inline-control {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  width: 100%;
}

.security-form :deep(.security-inline-item .el-form-item__content) {
  align-items: flex-start;
}

.security-inline-control :deep(.el-input-number),
.security-inline-control :deep(.el-switch) {
  flex-shrink: 0;
}

.el-divider {
  margin: 20px 0;
}

.protocol-description {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.protocol-description p {
  margin: 0;
}

.auto-proxy-priority {
  display: grid;
  grid-template-columns: repeat(2, minmax(220px, 1fr));
  gap: 12px;
  width: 100%;
}

.priority-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.priority-index {
  width: 76px;
  flex: 0 0 auto;
  color: #606266;
  font-size: 13px;
  white-space: nowrap;
}

.priority-select {
  min-width: 0;
  flex: 1 1 auto;
}

.el-descriptions-item {
  margin-bottom: 10px;
}

.version-selector {
  display: flex;
  align-items: center;
}

.version-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.version-tips {
  margin-left: 15px;
  font-size: 12px;
  color: #909399;
}

.info-icon {
  margin-left: 5px;
}

.version-info {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  width: 100%;
}

.version-info__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.version-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  margin-top: 10px;
  gap: 10px;
}

.version-sync-info {
  margin-top: 15px;
  width: 100%;
}

.sync-info-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.sync-info-content {
  font-size: 13px;
  line-height: 1.6;
}

.sync-info-content p {
  margin: 5px 0;
}

.sync-status {
  width: 100%;
  max-width: 150px;
  margin-top: 5px;
}

.changelog-list {
  margin: 0;
  padding-left: 20px;
}

.changelog-list li {
  margin-bottom: 5px;
}

.update-progress {
  padding: 20px 0;
}

.update-status {
  margin-top: 15px;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.error-message {
  color: #f56c6c;
  margin-top: 10px;
}

/* 错误详情样式 */
.error-details-container {
  max-height: 70vh;
  overflow-y: auto;
  font-size: 14px;
}

.error-card {
  margin-bottom: 15px;
}

.error-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: bold;
}

.error-message-content {
  white-space: pre-wrap;
  margin: 0;
  max-height: 200px;
  overflow-y: auto;
  padding: 8px;
  background-color: #f8f8f8;
  border-radius: 4px;
  border: 1px solid #e0e0e0;
  font-family: monospace;
  font-size: 13px;
}

.error-resolution {
  margin-top: 15px;
  border-left: 3px solid #e6a23c;
  padding-left: 10px;
  background-color: #fdf6ec;
  padding: 10px;
  border-radius: 4px;
  line-height: 1.5;
}

.error-resolution h4, .error-troubleshooting h4 {
  margin-top: 0;
  margin-bottom: 10px;
  color: var(--color-text-primary);
}

.error-resolution p {
  margin: 5px 0;
}

.error-troubleshooting {
  margin-top: 15px;
  border-left: 3px solid var(--color-primary);
  padding: 10px;
  background-color: rgba(64, 158, 255, 0.12);
  border-radius: 4px;
}

.error-troubleshooting ul {
  padding-left: 20px;
  margin: 5px 0;
}

.error-troubleshooting li {
  margin-bottom: 5px;
  line-height: 1.5;
}

.error-troubleshooting ol {
  padding-left: 20px;
  margin: 5px 0;
}

.error-troubleshooting code {
  background: rgba(0,0,0,0.07);
  border-radius: 3px;
  padding: 2px 5px;
  font-family: monospace;
}

.error-troubleshooting el-link {
  display: inline;
}

.version-dropdown {
  max-height: 300px;
  overflow-y: auto;
}

.error-troubleshooting {
  margin-top: 15px;
  padding: 10px;
  border-left: 3px solid #E6A23C;
  background-color: rgba(230, 162, 60, 0.1);
}

.error-troubleshooting h4 {
  margin-top: 0;
  margin-bottom: 8px;
  color: #606266;
}

.error-troubleshooting ul {
  margin: 0;
  padding-left: 20px;
  color: #606266;
}

.error-troubleshooting li {
  margin-bottom: 5px;
}

.version-action-alert {
  margin: 15px 0;
  border-radius: 4px;
}

.version-select-container {
  display: flex;
  align-items: center;
}

.version-controls {
  margin-left: 15px;
}

.version-alert-content {
  display: flex;
  align-items: center;
}

.version-change-info {
  margin: 0 10px;
}

.version-action-buttons {
  display: flex;
  gap: 10px;
}

.version-dialog-content {
  padding: 20px;
  text-align: center;
}

.version-info-row {
  margin-bottom: 10px;
}

.version-label {
  font-weight: bold;
}

.dialog-footer {
  margin-top: 20px;
  display: flex;
  justify-content: space-between;
}

.status-control {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.version-control {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  width: 100%;
}

.version-control :deep(.el-select) {
  width: min(240px, 100%);
}

.version-control :deep(.el-button) {
  flex-shrink: 0;
}

.version-tips {
  margin-left: 15px;
  font-size: 12px;
  color: #909399;
}

@media (max-width: 768px) {
  .settings-container {
    padding: 12px;
  }

  .settings-form {
    max-width: 100%;
  }

  .settings-form :deep(.el-form-item__label) {
    width: 100% !important;
    text-align: left;
    line-height: 1.4;
    padding-bottom: 6px;
  }

  .settings-form :deep(.el-form-item__content) {
    margin-left: 0 !important;
  }

  .settings-tabs :deep(.el-tabs__item) {
    padding-inline: 18px;
  }

  .settings-form :deep(.form-actions-row .el-form-item__content) {
    width: 100%;
  }

  .protocol-description,
  .auto-proxy-priority,
  .version-info,
  .version-info__actions,
  .status-control,
  .version-control,
  .sync-info-title,
  .error-header,
  .dialog-footer {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .auto-proxy-priority {
    display: grid;
    grid-template-columns: 1fr;
  }

  .version-info__actions,
  .version-info__actions :deep(.el-button),
  .status-control,
  .status-control :deep(.el-button),
  .version-control :deep(.el-select),
  .version-control :deep(.el-button),
  .version-actions,
  .version-actions :deep(.el-button),
  .dialog-footer :deep(.el-button) {
    width: 100%;
    margin-left: 0 !important;
  }
}
</style> 
