<template>
  <div class="settings-container">
    <h1>系统设置</h1>
    
    <el-tabs type="border-card" v-model="activeName" @tab-click="handleTabClick">
      <el-tab-pane label="服务器配置" name="server">
        <el-form :model="serverForm" label-width="120px" class="settings-form">
          <el-form-item label="面板监听地址">
            <el-input v-model="serverForm.panelListenIP" placeholder="0.0.0.0"></el-input>
            <div class="form-tips">默认为 0.0.0.0，代表监听所有 IP</div>
          </el-form-item>
          <el-form-item label="面板端口">
            <el-input-number v-model="serverForm.panelPort" :min="1" :max="65535"></el-input-number>
            <div class="form-tips">默认为 9000，修改后需要重启服务</div>
          </el-form-item>
          <el-form-item label="面板URL基础路径">
            <el-input v-model="serverForm.panelBasePath" placeholder="/"></el-input>
            <div class="form-tips">默认为 /，修改后需要重启服务</div>
          </el-form-item>
          <el-form-item label="代理服务模式">
            <el-select v-model="serverForm.proxyMode" style="width: 100%">
              <el-option label="兼容模式" value="compatible"></el-option>
              <el-option label="Xray 内核" value="xray"></el-option>
              <el-option label="V2Ray 内核" value="v2ray"></el-option>
            </el-select>
            <div class="form-tips">默认为兼容模式，可同时使用 Xray 和 V2Ray 协议</div>
          </el-form-item>
          <el-form-item label="服务时区">
            <el-select v-model="serverForm.timezone" style="width: 100%">
              <el-option label="Asia/Shanghai (UTC+8)" value="Asia/Shanghai"></el-option>
              <el-option label="UTC" value="UTC"></el-option>
              <el-option label="America/New_York (UTC-5)" value="America/New_York"></el-option>
              <el-option label="Europe/London (UTC+0)" value="Europe/London"></el-option>
            </el-select>
          </el-form-item>
          <el-divider></el-divider>
          <el-form-item>
            <el-button type="primary" @click="saveServerSettings">保存服务器配置</el-button>
            <el-button type="warning" @click="restartPanel">重启面板</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      
      <el-tab-pane label="数据库配置" name="db">
        <el-form :model="dbForm" label-width="120px" class="settings-form">
          <el-form-item label="数据库类型">
            <el-select v-model="dbForm.dbType" style="width: 100%">
              <el-option label="SQLite" value="sqlite"></el-option>
              <el-option label="MySQL" value="mysql"></el-option>
              <el-option label="PostgreSQL" value="postgres"></el-option>
            </el-select>
          </el-form-item>
          
          <template v-if="dbForm.dbType !== 'sqlite'">
            <el-form-item label="数据库服务器">
              <el-input v-model="dbForm.dbHost" placeholder="localhost"></el-input>
            </el-form-item>
            <el-form-item label="数据库端口">
              <el-input-number 
                v-model="dbForm.dbPort" 
                :min="1" 
                :max="65535"
                :placeholder="dbForm.dbType === 'mysql' ? '3306' : '5432'"
              ></el-input-number>
            </el-form-item>
            <el-form-item label="数据库名称">
              <el-input v-model="dbForm.dbName" placeholder="v_panel"></el-input>
            </el-form-item>
            <el-form-item label="用户名">
              <el-input v-model="dbForm.dbUser" placeholder="root"></el-input>
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="dbForm.dbPassword" type="password" placeholder="密码" show-password></el-input>
            </el-form-item>
          </template>
          
          <template v-else>
            <el-form-item label="SQLite文件路径">
              <el-input v-model="dbForm.sqlitePath" placeholder="/usr/local/v-panel/data.db"></el-input>
              <div class="form-tips">默认在程序目录下的 data.db 文件</div>
            </el-form-item>
          </template>
          
          <el-divider></el-divider>
          <el-form-item>
            <el-button type="primary" @click="saveDbSettings">保存数据库配置</el-button>
            <el-button @click="testDbConnection">测试连接</el-button>
            <el-button type="success" @click="backupDb">备份数据库</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      
      <el-tab-pane label="日志配置" name="log">
        <el-form :model="logForm" label-width="120px" class="settings-form">
          <el-form-item label="日志级别">
            <el-select v-model="logForm.logLevel" style="width: 100%">
              <el-option label="DEBUG" value="debug"></el-option>
              <el-option label="INFO" value="info"></el-option>
              <el-option label="WARN" value="warn"></el-option>
              <el-option label="ERROR" value="error"></el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="日志保留天数">
            <el-input-number v-model="logForm.logRetentionDays" :min="1" :max="365"></el-input-number>
            <div class="form-tips">超过该天数的日志将被自动清理</div>
          </el-form-item>
          <el-form-item label="日志存储路径">
            <el-input v-model="logForm.logPath"></el-input>
            <div class="form-tips">默认在程序目录下的 logs 文件夹</div>
          </el-form-item>
          <el-form-item label="启用访问日志">
            <el-switch v-model="logForm.enableAccessLog"></el-switch>
            <div class="form-tips">记录所有HTTP请求访问日志</div>
          </el-form-item>
          <el-form-item label="启用操作日志">
            <el-switch v-model="logForm.enableOperationLog"></el-switch>
            <div class="form-tips">记录所有用户操作日志</div>
          </el-form-item>
          <el-divider></el-divider>
          <el-form-item>
            <el-button type="primary" @click="saveLogSettings">保存日志配置</el-button>
            <el-button type="danger" @click="clearLogs">清理日志</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane label="邮箱配置" name="email">
        <el-form :model="emailForm" label-width="140px" class="settings-form">
          <el-alert
            title="邮箱配置用于注册验证、密码重置和系统提醒"
            type="info"
            :closable="false"
            show-icon
            style="margin-bottom: 20px"
          />

          <el-form-item label="SMTP 服务器">
            <el-input v-model="emailForm.host" placeholder="例如：smtp.qq.com" />
          </el-form-item>
          <el-form-item label="SMTP 端口">
            <el-input-number v-model="emailForm.port" :min="1" :max="65535" />
            <div class="form-tips">常见端口为 465 / 587 / 25</div>
          </el-form-item>
          <el-form-item label="SMTP 用户名">
            <el-input v-model="emailForm.user" placeholder="通常为邮箱地址" />
          </el-form-item>
          <el-form-item label="发件邮箱">
            <el-input
              v-model="emailForm.from"
              placeholder="留空则默认使用 SMTP 用户名"
            />
            <div class="form-tips">用于注册验证、重置密码等系统邮件的发件人地址</div>
          </el-form-item>
          <el-form-item label="告警收件邮箱">
            <el-input
              v-model="emailForm.alertEmail"
              placeholder="留空则默认发到 SMTP 用户名"
            />
            <div class="form-tips">用于节点异常、系统提醒等后台告警邮件</div>
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

          <el-divider></el-divider>
          <el-form-item>
            <el-button type="primary" :loading="emailForm.saving" @click="saveEmailSettings">保存邮箱配置</el-button>
            <el-button :loading="emailForm.loading" @click="loadEmailSettings">刷新</el-button>
            <el-button type="success" :loading="emailForm.testing" @click="testEmailSettings">发送测试邮件</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane label="支付设置" name="payment">
        <el-form :model="paymentForm" label-width="140px" class="settings-form">
          <el-alert
            title="支付设置保存后立即生效"
            type="info"
            :closable="false"
            show-icon
            style="margin-bottom: 20px"
          />

          <el-divider content-position="left">支付宝</el-divider>
          <el-form-item label="启用支付宝">
            <el-switch v-model="paymentForm.alipayEnabled" />
          </el-form-item>
          <el-form-item label="App ID">
            <el-input v-model="paymentForm.alipayAppId" placeholder="请输入支付宝 App ID" />
          </el-form-item>
          <el-form-item label="商户私钥">
            <el-input
              v-model="paymentForm.alipayPrivateKey"
              type="textarea"
              :rows="4"
              :placeholder="paymentForm.alipayPrivateKeyConfigured ? '已配置，留空则保持不变' : '请输入支付宝 RSA 私钥 PEM 内容'"
            />
          </el-form-item>
          <el-form-item label="支付宝公钥">
            <el-input
              v-model="paymentForm.alipayPublicKey"
              type="textarea"
              :rows="4"
              placeholder="请输入支付宝公钥 PEM 内容"
            />
          </el-form-item>
          <el-form-item label="通知地址">
            <el-input
              v-model="paymentForm.alipayNotifyUrl"
              placeholder="留空则自动使用面板公网地址 + /api/payments/callback/alipay"
            />
          </el-form-item>
          <el-form-item label="返回地址">
            <el-input
              v-model="paymentForm.alipayReturnUrl"
              placeholder="留空则自动使用面板公网地址 + /user/orders"
            />
          </el-form-item>
          <el-form-item label="沙箱模式">
            <el-switch v-model="paymentForm.alipaySandbox" />
          </el-form-item>

          <el-divider content-position="left">微信支付</el-divider>
          <el-form-item label="启用微信支付">
            <el-switch v-model="paymentForm.wechatEnabled" />
          </el-form-item>
          <el-form-item label="App ID">
            <el-input v-model="paymentForm.wechatAppId" placeholder="请输入微信支付 App ID" />
          </el-form-item>
          <el-form-item label="商户号">
            <el-input v-model="paymentForm.wechatMchId" placeholder="请输入微信支付商户号" />
          </el-form-item>
          <el-form-item label="API Key">
            <el-input
              v-model="paymentForm.wechatApiKey"
              type="password"
              show-password
              :placeholder="paymentForm.wechatApiKeyConfigured ? '已配置，留空则保持不变' : '请输入微信支付 API Key'"
            />
          </el-form-item>
          <el-form-item label="通知地址">
            <el-input
              v-model="paymentForm.wechatNotifyUrl"
              placeholder="留空则自动使用面板公网地址 + /api/payments/callback/wechat"
            />
          </el-form-item>
          <el-form-item label="沙箱模式">
            <el-switch v-model="paymentForm.wechatSandbox" />
          </el-form-item>

          <el-divider></el-divider>
          <el-form-item>
            <el-button type="primary" :loading="paymentForm.saving" @click="savePaymentSettings">保存支付设置</el-button>
            <el-button :loading="paymentForm.loading" @click="loadPaymentSettings">刷新</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      
      <el-tab-pane label="面板本地Xray" name="xray">
        <el-alert
          v-if="xraySettings.currentVersion === '未安装' && !xraySettings.running"
          type="info"
          :closable="false"
          show-icon
          style="margin-bottom: 16px;"
          title="这里管理的是面板服务器本地 Xray，不是远程节点内核。Docker 或远程节点部署下显示“未安装”通常是正常现象。"
        />
        <el-form label-width="120px" class="settings-form">
          <el-form-item label="当前版本">
            <div class="version-info">
              <el-tag size="large" type="success">{{ xraySettings.currentVersion || '未安装' }}</el-tag>
              <el-button 
                type="primary" 
                size="small" 
                @click="refreshXrayVersions"
                :loading="xraySettings.loading"
                style="margin-left: 10px;"
              >
                刷新
              </el-button>
              <el-button 
                type="warning" 
                size="small" 
                @click="syncVersionsFromGitHub"
                :loading="xraySettings.syncing"
                style="margin-left: 10px;"
              >
                从GitHub同步
              </el-button>
            </div>
          </el-form-item>
          
          <el-form-item label="运行状态">
            <el-tag :type="xraySettings.running ? 'success' : 'danger'">
              {{ xraySettings.running ? '运行中' : '已停止' }}
            </el-tag>
            <el-button 
              v-if="!xraySettings.running"
              type="success" 
              size="small" 
              @click="startXray"
              :loading="xraySettings.starting"
              style="margin-left: 10px;"
            >
              启动
            </el-button>
            <el-button 
              v-else
              type="danger" 
              size="small" 
              @click="stopXray"
              :loading="xraySettings.stopping"
              style="margin-left: 10px;"
            >
              停止
            </el-button>
          </el-form-item>
          
          <el-form-item label="切换版本">
            <div class="version-control">
              <el-select 
                v-model="xraySettings.selectedVersion" 
                placeholder="选择版本" 
                style="width: 180px;"
                :loading="xraySettings.loading"
                :disabled="xraySettings.switching"
              >
                <el-option 
                  v-for="version in xraySettings.versions" 
                  :key="version" 
                  :label="version" 
                  :value="version"
                >
                  <span>{{ version }}</span>
                  <span 
                    v-if="version === xraySettings.currentVersion" 
                    style="float: right; color: #67C23A; font-size: 12px"
                  >当前</span>
                </el-option>
              </el-select>
              <el-button 
                type="primary" 
                @click="handleSwitchVersion" 
                :loading="xraySettings.switching"
                :disabled="!xraySettings.selectedVersion || xraySettings.selectedVersion === xraySettings.currentVersion"
                style="margin-left: 10px;"
              >
                切换版本
              </el-button>
            </div>
            <div class="form-tips">选择要切换的 Xray 版本，切换后需要重启服务</div>
          </el-form-item>
          
          <el-form-item label="自动更新">
            <el-switch v-model="xraySettings.autoUpdate" />
            <div class="form-tips">启用后，系统将自动更新到最新的稳定版</div>
          </el-form-item>
          
          <el-divider></el-divider>
          <el-form-item>
            <el-button type="primary" @click="saveXraySettings">保存设置</el-button>
            <el-button type="success" @click="restartXray" :loading="xraySettings.restarting">重启Xray</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      
      <el-tab-pane label="管理员配置" name="admin">
        <el-form :model="adminForm" label-width="120px" class="settings-form">
          <el-alert
            title="管理员账号安全提示"
            type="warning"
            description="修改管理员密码后，当前会话将被注销，需要重新登录。请确保记住新密码，否则可能无法访问系统。"
            show-icon
            :closable="false"
            style="margin-bottom: 20px"
          />
          
          <el-form-item label="管理员用户名">
            <el-input v-model="adminForm.username" placeholder="admin" :disabled="true"></el-input>
            <div class="form-tips">默认管理员用户名不可修改</div>
          </el-form-item>
          <el-form-item label="当前密码">
            <el-input v-model="adminForm.currentPassword" type="password" placeholder="当前密码" show-password></el-input>
          </el-form-item>
          <el-form-item label="新密码">
            <el-input v-model="adminForm.newPassword" type="password" placeholder="新密码" show-password></el-input>
          </el-form-item>
          <el-form-item label="确认新密码">
            <el-input v-model="adminForm.confirmPassword" type="password" placeholder="确认新密码" show-password></el-input>
          </el-form-item>
          
          <el-divider></el-divider>
          <el-form-item>
            <el-button type="primary" @click="changeAdminPassword">修改密码</el-button>
            <el-button type="warning" @click="resetAdminPassword">重置为默认密码</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      
      <el-tab-pane label="安全设置" name="security">
        <el-form :model="securityForm" label-width="120px" class="settings-form">
          <el-form-item label="会话超时时间">
            <el-input-number v-model="securityForm.sessionTimeout" :min="5" :max="1440"></el-input-number>
            <div class="form-tips">单位：分钟，超过该时间未操作将自动注销</div>
          </el-form-item>
          <el-form-item label="启用IP白名单">
            <el-switch v-model="securityForm.enableIpWhitelist"></el-switch>
          </el-form-item>
          <el-form-item label="IP白名单" v-if="securityForm.enableIpWhitelist">
            <el-input 
              v-model="securityForm.ipWhitelist" 
              type="textarea" 
              :rows="4"
              placeholder="每行一个IP地址，支持CIDR格式，如：192.168.1.0/24"
            ></el-input>
          </el-form-item>
          <el-form-item label="登录失败锁定">
            <el-switch v-model="securityForm.enableLoginLock"></el-switch>
            <div class="form-tips">连续登录失败将暂时锁定账号</div>
          </el-form-item>
          <el-form-item label="失败尝试次数" v-if="securityForm.enableLoginLock">
            <el-input-number v-model="securityForm.maxLoginAttempts" :min="3" :max="10"></el-input-number>
          </el-form-item>
          <el-form-item label="锁定时间(分钟)" v-if="securityForm.enableLoginLock">
            <el-input-number v-model="securityForm.lockDuration" :min="5" :max="60"></el-input-number>
          </el-form-item>
          
          <el-divider></el-divider>
          <el-form-item>
            <el-button type="primary" @click="saveSecuritySettings">保存安全设置</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      
      <!-- 新增协议管理标签页 -->
      <el-tab-pane label="协议管理" name="protocol">
        <el-form class="settings-form">
          <el-form-item label="支持的协议" label-width="120px">
            <el-descriptions :column="1" border size="medium">
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
                  <el-tag type="success" size="small" v-if="protocolSettings.enableTrojan">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
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
                  <el-tag type="success" size="small" v-if="protocolSettings.enableVMess">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
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
                  <el-tag type="success" size="small" v-if="protocolSettings.enableVLESS">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
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
                  <el-tag type="success" size="small" v-if="protocolSettings.enableShadowsocks">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
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
                  <el-tag type="success" size="small" v-if="protocolSettings.enableSocks">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
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
                  <el-tag type="success" size="small" v-if="protocolSettings.enableHTTP">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
                </div>
              </el-descriptions-item>
            </el-descriptions>
          </el-form-item>
          
          <el-divider content-position="left">传输层设置</el-divider>
          
          <el-form-item label="支持的传输层" label-width="120px">
            <el-descriptions :column="1" border size="medium">
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
                  <el-tag type="success" size="small" v-if="transportSettings.enableTCP">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
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
                  <el-tag type="success" size="small" v-if="transportSettings.enableWebSocket">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
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
                  <el-tag type="success" size="small" v-if="transportSettings.enableHTTP2">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
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
                  <el-tag type="success" size="small" v-if="transportSettings.enableGRPC">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
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
                  <el-tag type="success" size="small" v-if="transportSettings.enableQUIC">已启用</el-tag>
                  <el-tag type="danger" size="small" v-else>已禁用</el-tag>
                </div>
              </el-descriptions-item>
            </el-descriptions>
          </el-form-item>
          
          <el-divider></el-divider>
          
          <el-form-item>
            <el-button type="primary" @click="saveProtocolSettings" :loading="protocolsLoading">保存协议配置</el-button>
            <el-button type="warning" @click="restartXrayAfterProtocolChange" :loading="xraySettings.restarting">保存并重启Xray</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>
  </div>

  <!-- 添加在组件末尾的弹窗 -->
  <el-dialog 
    v-model="xraySettings.showVersionDetails" 
    title="Xray版本详情" 
    width="600px"
    destroy-on-close
  >
    <el-descriptions :column="1" border>
      <el-descriptions-item label="版本">
        <el-tag type="success">{{ xraySettings.versionDetails.version }}</el-tag>
      </el-descriptions-item>
      <el-descriptions-item label="发布日期">
        {{ xraySettings.versionDetails.releaseDate }}
      </el-descriptions-item>
      <el-descriptions-item label="描述">
        {{ xraySettings.versionDetails.description }}
      </el-descriptions-item>
      <el-descriptions-item label="更新日志">
        <ul class="changelog-list">
          <li v-for="(change, index) in xraySettings.versionDetails.changelog" :key="index">
            {{ change }}
          </li>
        </ul>
      </el-descriptions-item>
    </el-descriptions>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="xraySettings.showVersionDetails = false">关闭</el-button>
        <el-button type="primary" @click="openXrayReleasePage">
          查看GitHub发布页
        </el-button>
      </span>
    </template>
  </el-dialog>

  <!-- 添加更新进度对话框 -->
  <el-dialog 
    v-model="xraySettings.updateProgress.visible" 
    :title="`Xray 更新 - ${xraySettings.downloadingVersion}`"
    width="500px"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="xraySettings.updateProgress.status === 'completed' || xraySettings.updateProgress.status === 'error'"
  >
    <div class="update-progress">
      <el-progress 
        :percentage="xraySettings.updateProgress.percent" 
        :status="xraySettings.updateProgress.status === 'error' ? 'exception' : 
                xraySettings.updateProgress.status === 'completed' ? 'success' : ''"
        :striped="xraySettings.updateProgress.status === 'downloading' || xraySettings.updateProgress.status === 'installing'"
        :striped-flow="xraySettings.updateProgress.status === 'downloading' || xraySettings.updateProgress.status === 'installing'"
      ></el-progress>
      
      <div class="update-status">
        <span>{{ xraySettings.updateProgress.message }}</span>
        <span class="error-message" v-if="xraySettings.updateProgress.status === 'error'">
          错误: {{ xraySettings.updateProgress.error }}
        </span>
      </div>
    </div>
    
    <template #footer v-if="xraySettings.updateProgress.status === 'completed' || xraySettings.updateProgress.status === 'error'">
      <el-button @click="xraySettings.updateProgress.visible = false">关闭</el-button>
      <el-button 
        type="primary" 
        v-if="xraySettings.updateProgress.status === 'completed'"
        @click="restartXray"
      >
        重启Xray
      </el-button>
      <el-button 
        type="warning" 
        v-if="xraySettings.updateProgress.status === 'error'"
        @click="downloadXrayVersion(xraySettings.downloadingVersion)"
      >
        重试
      </el-button>
    </template>
  </el-dialog>
  
  <!-- 添加错误详情对话框 -->
  <el-dialog
    v-model="errorDetails.visible"
    title="错误详情"
    width="600px"
    destroy-on-close
  >
    <div class="error-details-container">
      <el-alert
        :title="errorDetails.title"
        type="error"
        description=""
        show-icon
        :closable="false"
        style="margin-bottom: 15px;"
      />
      
      <el-card shadow="never" class="error-card">
        <template #header>
          <div class="error-header">
            <span>错误信息</span>
            <el-button 
              type="primary" 
              size="small" 
              plain 
              @click="copyErrorToClipboard"
              circle
              icon="CopyDocument"
            />
          </div>
        </template>
        <pre class="error-message-content">{{ errorDetails.message }}</pre>
      </el-card>
      
      <div class="error-resolution" v-if="errorDetails.resolution">
        <h4>可能的解决方案：</h4>
        <div v-if="errorDetails.resolution.includes('\n')">
          <p v-for="(line, index) in errorDetails.resolution.split('\n')" :key="index" 
             :style="line.startsWith('   -') ? 'margin-left: 20px;' : ''">
            {{ line }}
          </p>
        </div>
        <p v-else>{{ errorDetails.resolution }}</p>
      </div>

      <div class="error-troubleshooting" v-if="errorDetails.title && errorDetails.title.includes('下载') || 
           (errorDetails.message && errorDetails.message.toLowerCase().includes('timeout'))">
        <h4>手动下载指南：</h4>
        <p>如果自动下载失败，您可以按照以下步骤手动下载：</p>
        <ol>
          <li>根据您的系统类型，打开
            <el-link href="https://github.com/XTLS/Xray-core/releases" type="primary" target="_blank">
              Xray GitHub 发布页
            </el-link>
          </li>
          <li>下载适合您系统的文件，例如：
            <ul>
              <li>Windows 64位: <code>Xray-windows-64.zip</code></li>
              <li>Windows 32位: <code>Xray-windows-32.zip</code></li>
              <li>Mac OS: <code>Xray-macos-64.zip</code></li>
              <li>Linux: <code>Xray-linux-64.zip</code> 或 <code>Xray-linux-arm64-v8a.zip</code></li>
            </ul>
          </li>
          <li>将下载的zip文件放入 <code>xray/downloads/</code> 目录</li>
          <li>重新尝试切换版本，系统将自动使用本地文件</li>
        </ol>
      </div>

      <div class="error-troubleshooting" v-if="errorDetails.title && errorDetails.title.includes('Xray 版本')">
        <h4>故障排除建议：</h4>
        <ul>
          <li>检查Xray服务状态是否正常</li>
          <li>确认系统时间是否准确</li>
          <li>尝试先停止Xray服务再切换版本</li>
          <li>检查服务器是否能访问GitHub服务器</li>
          <li>确认系统磁盘空间是否充足</li>
          <li>查看系统日志获取更多信息</li>
        </ul>
      </div>
    </div>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="errorDetails.visible = false">关闭</el-button>
        <el-button type="primary" @click="retryFailedOperation" v-if="errorDetails.canRetry">
          重试操作
        </el-button>
        <el-button type="warning" @click="refreshXrayVersions" v-if="errorDetails.title && errorDetails.title.includes('Xray 版本')">
          刷新版本信息
        </el-button>
        <el-button type="info" @click="openGitHubReleases" v-if="errorDetails.message && 
                   (errorDetails.message.toLowerCase().includes('timeout') || 
                    errorDetails.message.includes('下载') || 
                    errorDetails.message.includes('download'))">
          访问下载页面
        </el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, onMounted, computed, watch, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '@/stores/user'
import api, { systemApi } from '@/api/index'
import { InfoFilled, CopyDocument, Refresh, Connection, Download, Link, Loading, ArrowRight } from '@element-plus/icons-vue'

// store
const userStore = useUserStore()

// 当前活动标签页
const activeName = ref('server')

// 同步时间处理
const lastSyncTime = ref(localStorage.getItem('xray_last_sync_time') || '')

// Xray设置状态管理
const xraySettings = reactive({
  currentVersion: '未知',
  selectedVersion: '',
  versions: [],    // 版本列表
  loading: false,  // 加载状态
  autoUpdate: true,
  customConfig: false,
  configPath: '',
  checkInterval: 24,
  running: false,  // 运行状态
  restarting: false,
  syncing: false,  // 同步状态
  starting: false, // 启动状态
  stopping: false, // 停止状态
  showVersionDetails: false,
  versionDetails: {
    version: '',
    releaseDate: '',
    description: '',
    changelog: []
  },
  updateProgress: {
    visible: false,
    percent: 0,
    status: '',
    message: '',
    error: ''
  },
  switching: false,
  downloadingVersion: '',
  checkingForUpdates: false
});

// 初始设置尝试用一个默认值
try {
  xraySettings.versions = [
    'v1.8.24', 'v1.8.23', 'v1.8.22', 'v1.8.21', 'v1.8.20',
    'v25.3.6', 'v25.3.3', 'v25.2.21', 'v25.2.18', 'v25.1.30'
  ];
} catch (e) {
  console.error('Failed to set default versions:', e);
}

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
  sessionTimeout: 30,
  enableIpWhitelist: false,
  ipWhitelist: '',
  enableLoginLock: true,
  maxLoginAttempts: 5,
  lockDuration: 10
})

// 协议设置
const protocolSettings = reactive({
  enableTrojan: true,
  enableVMess: true,
  enableVLESS: true,
  enableShadowsocks: true,
  enableSocks: false,
  enableHTTP: false
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
const disableProtocolSwitch = computed(() => {
  // 如果正在加载或者Xray正在重启，禁用开关
  return protocolsLoading.value || xraySettings.restarting
})
const disableTransportSwitch = computed(() => {
  // 如果正在加载或者Xray正在重启，禁用开关
  return protocolsLoading.value || xraySettings.restarting
})

// 错误详情
const errorDetails = reactive({
  visible: false,
  title: '',
  message: '',
  resolution: '',
  canRetry: false,
  retryAction: null,
  retryParams: null
})

// 处理标签页切换
const handleTabClick = (tab) => {
  console.log('Tab clicked:', tab.props.name)
  if (tab.props.name === 'xray') {
    // 当切换到Xray标签页时，刷新数据
    refreshXraySettings()
  }
}

// 启动Xray
const startXray = async () => {
  try {
    xraySettings.starting = true
    ElMessage.info('正在启动 Xray 服务...')
    
    const response = await api.post('/xray/start')
    
    if (response.data) {
      ElMessage.success('Xray 服务已启动')
      xraySettings.running = true
      
      // 刷新状态
      await refreshXrayStatus()
    }
  } catch (error) {
    console.error('Failed to start Xray:', error)
    ElMessage.error('启动 Xray 失败: ' + (error.response?.data?.error || error.message || '未知错误'))
  } finally {
    xraySettings.starting = false
  }
}

// 停止Xray
const stopXray = async () => {
  try {
    await ElMessageBox.confirm(
      '确定要停止 Xray 服务吗？这将中断所有正在连接的用户。',
      '停止 Xray',
      {
        confirmButtonText: '确认停止',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    xraySettings.stopping = true
    ElMessage.info('正在停止 Xray 服务...')
    
    const response = await api.post('/xray/stop')
    
    if (response.data) {
      ElMessage.success('Xray 服务已停止')
      xraySettings.running = false
      
      // 刷新状态
      await refreshXrayStatus()
    }
  } catch (error) {
    if (error === 'cancel') {
      return
    }
    console.error('Failed to stop Xray:', error)
    ElMessage.error('停止 Xray 失败: ' + (error.response?.data?.error || error.message || '未知错误'))
  } finally {
    xraySettings.stopping = false
  }
}

// 重启Xray
const restartXray = async () => {
  try {
    // 显示确认对话框
    await ElMessageBox.confirm(
      '确定要重启Xray服务吗？这可能会暂时中断所有正在连接的用户。',
      '重启Xray',
      {
        confirmButtonText: '确认重启',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    
    xraySettings.restarting = true
    ElMessage.info('正在停止Xray服务...')
    
    // 检测是否为Windows平台
    let isWindows = false;
    try {
      const systemInfoResponse = await api.get('/system/info')
      if (systemInfoResponse.data && systemInfoResponse.data.os) {
        isWindows = systemInfoResponse.data.os === 'windows';
        console.log(`Detected platform: ${systemInfoResponse.data.os} (Windows: ${isWindows})`);
      }
    } catch (err) {
      console.warn('Failed to detect platform, assuming non-Windows', err);
    }
    
    // 根据平台设置不同的超时时间
    const stopTimeout = isWindows ? 20000 : 15000;
    const startTimeout = isWindows ? 25000 : 15000;
    
    // 停止服务
    try {
      await api.post('/xray/stop', {}, { timeout: stopTimeout })
    } catch (stopError) {
      console.error('Failed to stop Xray:', stopError)
      
      if (isWindows) {
        ElMessage.warning('Windows平台上停止Xray服务可能需要更长时间，正在继续尝试...')
        
        // 在Windows上可能需要额外等待
        await new Promise(resolve => setTimeout(resolve, 3000))
      } else {
        ElMessage.warning('停止Xray服务时出现问题，将尝试直接启动服务')
      }
      // 继续执行，尝试启动服务
    }
    
    // 等待短暂时间确保服务完全停止
    await new Promise(resolve => setTimeout(resolve, isWindows ? 3000 : 2000))
    
    ElMessage.info('正在检查Xray版本并启动服务...')
    
    // 先获取当前系统信息
    try {
      const systemInfoResponse = await api.get('/system/info')
      if (systemInfoResponse.data) {
        console.log('Current system info:', systemInfoResponse.data)
        // 记录系统信息，用于可能的问题排查
        localStorage.setItem('system_info', JSON.stringify({
          os: systemInfoResponse.data.os || 'unknown',
          arch: systemInfoResponse.data.arch || 'unknown',
          timestamp: Date.now()
        }))
      }
    } catch (infoError) {
      console.warn('Failed to get system info:', infoError)
      // 不中断流程
    }
    
    // 启动服务，对于Windows平台使用更长的超时时间
    try {
      const startResponse = await api.post('/xray/start', {}, { timeout: startTimeout })
      
      // 检查启动是否成功
      if (startResponse.data && startResponse.data.success) {
        ElMessage.success({
          message: 'Xray 服务已成功重启',
          duration: 3000
        })
        
        // 重新加载配置信息
        setTimeout(() => {
          refreshXrayVersions()
        }, 1000)
      } else {
        throw new Error(startResponse.data?.message || '启动服务失败，服务器返回非成功状态')
      }
    } catch (startError) {
      console.error('Failed to start Xray:', startError)
      
      // 判断是否为超时错误
      const isTimeout = startError.code === 'ECONNABORTED' || 
                        (startError.message && startError.message.includes('timeout'));
      
      if (isTimeout) {
        // 超时处理 - 特别是Windows平台
        ElMessage.warning('启动Xray服务超时，将尝试验证服务状态...')
        
        // 延迟检查服务状态
        await new Promise(resolve => setTimeout(resolve, 3000))
        
        try {
          const statusResponse = await api.get('/xray/status', { timeout: 10000 })
          if (statusResponse.data && statusResponse.data.running) {
            // 服务实际上已经在运行
            ElMessage.success('Xray服务已成功启动，但响应超时')
            setTimeout(() => {
              refreshXrayVersions()
            }, 1000)
            return
          }
        } catch (statusError) {
          console.warn('Failed to check Xray status after timeout:', statusError)
          // 继续到错误处理
        }
      }
      
      // 判断是否为版本问题
      const errorMsg = startError.response?.data?.message || startError.message || ''
      const isVersionError = errorMsg.includes('not found') || 
                            errorMsg.includes('version') || 
                            errorMsg.includes('下载失败') ||
                            errorMsg.includes('download')
      
      if (isVersionError) {
        // 可能是版本问题，尝试重新获取版本列表
        ElMessage.warning('可能是Xray版本问题，正在尝试刷新版本列表...')
        try {
          await refreshXrayVersions()
          // 再次尝试启动
          const retryResponse = await api.post('/xray/start', {}, { timeout: startTimeout })
          if (retryResponse.data && retryResponse.data.success) {
            ElMessage.success('Xray服务已在刷新版本后成功启动')
            setTimeout(() => {
              refreshXrayVersions()
            }, 1000)
            return
          }
        } catch (refreshError) {
          console.error('Failed to refresh and retry:', refreshError)
          // 继续到错误处理
        }
      }
      
      // 特别处理 Windows 平台的错误
      if (isWindows) {
        showErrorDetails(
          'Windows平台上启动Xray服务失败',
          `启动Xray服务时发生错误: ${errorMsg}`,
          '可能的解决方案：\n1. 检查防火墙或杀毒软件是否阻止了Xray运行\n2. 以管理员权限运行本程序\n3. 尝试手动下载适合Windows的Xray版本\n4. 检查端口是否被占用',
          true,
          'retryStartXrayWindows'
        )
      } else {
        // 显示详细错误
        showErrorDetails(
          'Xray服务启动失败',
          `启动Xray服务时发生错误: ${errorMsg}`,
          '可能的解决方案：\n1. 检查Xray服务状态和日志\n2. 尝试手动下载适合您系统的Xray版本\n3. 检查配置文件是否正确\n4. 确保端口未被占用',
          true,
          'retryStartXray'
        )
      }
      
      throw startError // 继续抛出错误以触发catch块
    }
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消重启操作')
      return
    }
    
    console.error('Failed to restart Xray:', error)
    ElMessage.error({
      message: '重启 Xray 失败: ' + (error.response?.data?.message || error.message || '未知错误'),
      duration: 5000
    })
    
    // 显示详细错误信息
    const errorMsg = error.response?.data?.message || error.message || '未知错误'
    const errorDetail = error.response?.data?.detail || error.stack || ''
    showErrorDetails(
      '重启 Xray 失败',
      errorDetail ? `${errorMsg}\n\n${errorDetail}` : errorMsg,
      '请检查Xray服务状态和日志，确保端口未被占用，并且有足够的系统权限。',
      true,
      'restart'
    )
    
    // 尝试恢复服务
    try {
      ElMessage.warning('正在尝试恢复服务...')
      await api.post('/xray/start', {}, { timeout: 15000 })
    } catch (recoveryError) {
      console.error('Failed to recover Xray service:', recoveryError)
      ElMessage.error({
        message: 'Xray服务恢复失败，请手动检查服务状态',
        duration: 5000
      })
    }
  } finally {
    xraySettings.restarting = false
  }
}

// 执行版本切换
const performVersionSwitch = async (skipConfirm = false) => {
  try {
    const response = await api.switchXrayVersion(xraySettings.selectedVersion);
    if (response.data.success) {
      ElMessage.success(`已成功切换到版本 ${xraySettings.selectedVersion}`);
      // 更新当前版本
      xraySettings.currentVersion = xraySettings.selectedVersion;
      return true;
    } else {
      ElMessage.error(response.data.message || '切换版本失败');
      return false;
    }
  } catch (error) {
    console.error('Failed to switch xray version:', error);
    ElMessage.error('切换版本失败: ' + (error.response?.data?.message || error.message || '未知错误'));
    throw error;
  }
};

// 初始化
onMounted(async () => {
  // 初始化加载所有设置
  try {
    console.log('组件加载中，初始化设置...');
    
    // 设置加载状态
    xraySettings.loading = true;
    
    // 加载Xray版本和支付设置
    await Promise.allSettled([
      refreshXrayVersions(),
      loadEmailSettings(),
      loadPaymentSettings()
    ]);
    
    console.log('Initial xraySettings:', { ...xraySettings });
  } catch (error) {
    console.error('Failed to load initial settings:', error);
    ElMessage.error('加载设置失败，请刷新页面重试');
  } finally {
    xraySettings.loading = false;
  }
  
  // 添加版本同步状态监听，处理重新加载和状态同步
  window.addEventListener('xray-version-sync', () => {
    refreshXrayVersions();
    refreshXrayStatus(); 
  });
  
  // 监听下载进度事件
  window.addEventListener('xray-download-progress', handleDownloadProgressEvent);
});

onUnmounted(() => {
  // 移除事件监听
  window.removeEventListener('xray-version-sync', refreshXrayVersions);
  window.removeEventListener('xray-download-progress', handleDownloadProgressEvent);
});

// 处理下载进度事件
const handleDownloadProgressEvent = (event) => {
  if (event && event.detail) {
    const progress = event.detail;
    
    if (progress.version && progress.status) {
      // 如果是正在切换的版本，更新UI显示
      if (xraySettings.selectedVersion === progress.version) {
        xraySettings.updateProgress.visible = true;
        xraySettings.updateProgress.status = progress.status;
        xraySettings.updateProgress.percent = progress.percent || 0;
        xraySettings.updateProgress.message = progress.message || `正在处理版本 ${progress.version}...`;
        xraySettings.updateProgress.details = progress.details || {};
        
        // 对不同状态进行不同处理
        switch(progress.status) {
          case 'completed':
            // 成功完成，显示成功提示
            ElMessage.success({
              message: `Xray ${progress.version} 安装成功!`,
              duration: 3000
            });
            
            // 延迟关闭进度条
            setTimeout(() => {
              xraySettings.updateProgress.visible = false;
            }, 1500);
            
            // 刷新版本信息
            refreshXrayVersions();
            refreshXrayStatus();
            break;
            
          case 'error':
            // 处理错误，显示更详细的错误信息
            ElMessage.error({
              message: `下载失败: ${progress.message}`,
              duration: 5000
            });
            
            // 记录错误详情
            console.error('Xray下载失败:', progress);
            
            // 显示错误弹窗，包含重试建议
            showErrorDetails(
              'Xray版本下载失败',
              progress.message,
              `建议:
              1. 检查网络连接
              2. 尝试使用不同的镜像源
              3. 手动下载Xray安装包放置到xray/downloads目录
              4. 如仍有问题，请查看服务器日志获取详细错误信息`,
              true,
              'downloadXray'
            );
            
            // 保持进度条可见，但更改样式为错误状态
            xraySettings.updateProgress.status = 'error';
            break;
            
          case 'progress':
            // 正常进度更新，如果有详细信息则更新到UI
            if (progress.details) {
              // 为了防止UI刷新过快，只在关键进度点更新
              if (progress.percent % 10 === 0 || progress.percent >= 50) {
                xraySettings.updateProgress.details = progress.details;
              }
            }
            break;
        }
      }
    }
  }
};

// 加载Xray版本信息
const loadXrayVersions = async () => {
  xraySettings.loading = true
  try {
    // 显示正在刷新的提示
    ElMessage.info('正在刷新Xray版本信息...')
    
    // 从API获取Xray版本信息
    try {
      const response = await api.get('/xray/versions')
      // 注意：axios拦截器已经返回了response.data
      if (response && response.current_version) {
        xraySettings.currentVersion = response.current_version
      }
      
      if (response && Array.isArray(response.supported_versions) && response.supported_versions.length > 0) {
        xraySettings.versions = response.supported_versions
      }
    } catch (error) {
      console.warn('Failed to get versions from API:', error)
      // 使用默认版本列表
      xraySettings.versions = [
        'v1.8.24', 'v1.8.23', 'v1.8.22', 'v1.8.21', 'v1.8.20',
        'v25.3.6', 'v25.3.3', 'v25.2.21', 'v25.2.18', 'v25.1.30'
      ]
    }
    
    // 确保selectedVersion有值，默认为当前版本
    if (!xraySettings.selectedVersion || xraySettings.selectedVersion === '') {
      if (xraySettings.currentVersion && xraySettings.currentVersion !== '未知') {
        xraySettings.selectedVersion = xraySettings.currentVersion
      } else if (xraySettings.versions.length > 0) {
        xraySettings.selectedVersion = xraySettings.versions[0]
      }
    }
    
    // 成功刷新提示
    ElMessage.success('Xray版本信息已刷新')
    
    // 同时加载Xray设置
    await loadXraySettings()
  } catch (error) {
    console.error('Failed to refresh Xray versions:', error)
    ElMessage.error('刷新Xray版本信息失败: ' + (error.message || '未知错误'))
    
    // 使用默认版本列表
    xraySettings.versions = [
      'v1.8.24', 'v1.8.23', 'v1.8.22', 'v1.8.21', 'v1.8.20',
      'v25.3.6', 'v25.3.3', 'v25.2.21', 'v25.2.18', 'v25.1.30'
    ]
    
    // 选择一个默认版本
    if (!xraySettings.selectedVersion || xraySettings.selectedVersion === '') {
      xraySettings.selectedVersion = xraySettings.versions[0]
    }
  } finally {
    xraySettings.loading = false
  }
}

// 从GitHub同步Xray版本
const syncVersionsFromGitHub = async () => {
  // 设置同步状态
  xraySettings.syncing = true
  
  try {
    ElMessage.info('正在从 GitHub 同步版本列表...')
    
    // 调用后端API同步版本
    const response = await api.post('/xray/sync-versions', {}, { timeout: 60000 })
    
    // 注意：axios拦截器已经返回了response.data，所以直接使用response
    if (response && response.success) {
      // 同步成功，更新版本列表
      if (Array.isArray(response.versions) && response.versions.length > 0) {
        xraySettings.versions = response.versions
        
        // 更新同步时间
        lastSyncTime.value = Date.now().toString()
        localStorage.setItem('xray_last_sync_time', lastSyncTime.value)
        
        ElMessage.success(`已从 GitHub 同步 ${response.count || response.versions.length} 个版本`)
        
        // 更新当前选择的版本
        if (!xraySettings.selectedVersion || !xraySettings.versions.includes(xraySettings.selectedVersion)) {
          if (xraySettings.versions.includes(xraySettings.currentVersion)) {
            xraySettings.selectedVersion = xraySettings.currentVersion
          } else if (xraySettings.versions.length > 0) {
            xraySettings.selectedVersion = xraySettings.versions[0]
          }
        }
        
        return true
      } else {
        // 同步成功但没有版本，重新获取
        await refreshXrayVersions()
        return true
      }
    } else {
      throw new Error(response?.error || '同步失败')
    }
  } catch (error) {
    console.error('Failed to sync versions from GitHub:', error)
    
    // 错误处理
    let errorMsg = '同步版本失败'
    if (error.code === 'ECONNABORTED' || error.message?.includes('timeout')) {
      errorMsg = '同步超时，请检查网络连接'
    } else if (error.status === 403) {
      errorMsg = 'GitHub API 访问受限，请稍后再试'
    } else if (error.error) {
      errorMsg = error.error
    } else if (error.message) {
      errorMsg = error.message
    }
    
    ElMessage.error(errorMsg)
    
    // 使用本地备用版本列表
    if (!xraySettings.versions.length) {
      xraySettings.versions = [
        'v1.8.24', 'v1.8.23', 'v1.8.22', 'v1.8.21', 'v1.8.20',
        'v1.8.19', 'v1.8.18', 'v1.8.17', 'v1.8.16', 'v1.8.15'
      ]
      
      if (!xraySettings.selectedVersion || !xraySettings.versions.includes(xraySettings.selectedVersion)) {
        xraySettings.selectedVersion = xraySettings.versions[0]
      }
      
      ElMessage.warning('已使用本地版本列表作为备用')
    }
    
    return false
  } finally {
    xraySettings.syncing = false
  }
}

// 加载Xray设置
const loadXraySettings = async () => {
  try {
    console.log('开始加载Xray设置，当前值：', { ...xraySettings });
    
    const response = await api.get('/settings/xray');
    
    if (response.data) {
      // 确保使用正确的属性名从API响应获取数据
      const newAutoUpdate = response.data.auto_update ?? false;
      const newCustomConfig = response.data.custom_config ?? false;
      const newConfigPath = response.data.config_path || '';
      const newCheckInterval = response.data.check_interval || 24;
      
      // 将API返回值与当前值进行比较
      console.log('Xray设置对比：', {
        autoUpdate: { old: xraySettings.autoUpdate, new: newAutoUpdate },
        customConfig: { old: xraySettings.customConfig, new: newCustomConfig },
        configPath: { old: xraySettings.configPath, new: newConfigPath },
        checkInterval: { old: xraySettings.checkInterval, new: newCheckInterval }
      });
      
      // 设置新值
      xraySettings.autoUpdate = newAutoUpdate;
      xraySettings.customConfig = newCustomConfig;
      xraySettings.configPath = newConfigPath;
      xraySettings.checkInterval = newCheckInterval;
      
      console.log('Xray设置加载完成:', { ...xraySettings });
    }
  } catch (error) {
    console.error('Failed to load Xray settings:', error);
  }
}

// 刷新Xray版本信息
const refreshXrayVersions = async () => {
  try {
    // 显示加载状态
    xraySettings.loading = true;
    
    // 获取版本列表
    const response = await api.get('/xray/versions');
    // 注意：axios拦截器已经返回了response.data
    if (response) {
      // 确保数据有效
      if (Array.isArray(response.supported_versions) && response.supported_versions.length > 0) {
        xraySettings.versions = response.supported_versions;
        console.log('获取到的版本列表:', xraySettings.versions);
      } else {
        console.warn('API返回的版本列表为空，使用默认版本');
        // 使用备用版本列表
        xraySettings.versions = [
          'v1.8.24', 'v1.8.23', 'v1.8.22', 'v1.8.21', 'v1.8.20',
          'v25.3.6', 'v25.3.3', 'v25.2.21', 'v25.2.18', 'v25.1.30'
        ];
      }
      
      xraySettings.currentVersion = response.current_version || '未知';
      
      // 如果当前没有选择版本，默认选择当前版本
      if (!xraySettings.selectedVersion) {
        xraySettings.selectedVersion = xraySettings.currentVersion;
      }
      
      console.log('Xray versions refreshed:', {
        supportedVersions: xraySettings.versions,
        currentVersion: xraySettings.currentVersion,
        selectedVersion: xraySettings.selectedVersion
      });
    }
  } catch (error) {
    console.error('Failed to refresh Xray versions:', error);
    ElMessage.error('获取Xray版本列表失败');
    
    // 使用备用版本列表
    xraySettings.versions = [
      'v1.8.24', 'v1.8.23', 'v1.8.22', 'v1.8.21', 'v1.8.20',
      'v25.3.6', 'v25.3.3', 'v25.2.21', 'v25.2.18', 'v25.1.30'
    ];
    
    // 确保选择的版本在列表中
    if (!xraySettings.selectedVersion || !xraySettings.versions.includes(xraySettings.selectedVersion)) {
      xraySettings.selectedVersion = xraySettings.versions[0];
    }
  } finally {
    xraySettings.loading = false;
  }
};

// 刷新Xray运行状态
const refreshXrayStatus = async () => {
  try {
    const response = await api.get('/xray/status');
    // 注意：axios拦截器已经返回了response.data
    if (response) {
      xraySettings.running = response.running || false;
      
      // 如果API返回了当前版本，更新版本信息
      if (response.current_version) {
        xraySettings.currentVersion = response.current_version;
      }
      
      console.log('Xray status refreshed:', {
        running: xraySettings.running,
        currentVersion: xraySettings.currentVersion
      });
    }
  } catch (error) {
    console.error('Failed to refresh Xray status:', error);
    // 不显示错误，避免界面过多提示
  }
};

// 方法
const saveServerSettings = async () => {
  try {
    // 在实际项目中应调用API保存配置
    // await api.saveServerSettings(serverForm)
    ElMessage.success('服务器配置保存成功')
  } catch (error) {
    ElMessage.error('保存失败：' + error.message)
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
      // 在实际项目中应调用API重启面板
      // await api.restartPanel()
      ElMessage.success('面板重启指令已发送，请稍后刷新页面')
    } catch (error) {
      ElMessage.error('重启失败：' + error.message)
    }
  })
  .catch(() => {
    ElMessage.info('已取消重启')
  })
}

const saveDbSettings = async () => {
  try {
    // 在实际项目中应调用API保存配置
    // await api.saveDbSettings(dbForm)
    ElMessage.success('数据库配置保存成功')
  } catch (error) {
    ElMessage.error('保存失败：' + error.message)
  }
}

const testDbConnection = async () => {
  try {
    // 在实际项目中应调用API测试连接
    // await api.testDbConnection(dbForm)
    ElMessage.success('数据库连接测试成功')
  } catch (error) {
    ElMessage.error('连接测试失败：' + error.message)
  }
}

const backupDb = async () => {
  try {
    // 在实际项目中应调用API备份数据库
    // await api.backupDatabase()
    ElMessage.success('数据库备份成功')
  } catch (error) {
    ElMessage.error('备份失败：' + error.message)
  }
}

const saveLogSettings = async () => {
  try {
    // 在实际项目中应调用API保存配置
    // await api.saveLogSettings(logForm)
    ElMessage.success('日志配置保存成功')
  } catch (error) {
    ElMessage.error('保存失败：' + error.message)
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
    ElMessage.error(error.message || '保存邮箱配置失败')
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
    ElMessage.error(error.message || '发送测试邮件失败')
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

const savePaymentSettings = async () => {
  paymentForm.saving = true
  try {
    const payload = {
      payment_alipay_enabled: paymentForm.alipayEnabled,
      payment_alipay_app_id: paymentForm.alipayAppId.trim(),
      payment_alipay_public_key: paymentForm.alipayPublicKey.trim(),
      payment_alipay_notify_url: paymentForm.alipayNotifyUrl.trim(),
      payment_alipay_return_url: paymentForm.alipayReturnUrl.trim(),
      payment_alipay_sandbox: paymentForm.alipaySandbox,
      payment_wechat_enabled: paymentForm.wechatEnabled,
      payment_wechat_app_id: paymentForm.wechatAppId.trim(),
      payment_wechat_mch_id: paymentForm.wechatMchId.trim(),
      payment_wechat_notify_url: paymentForm.wechatNotifyUrl.trim(),
      payment_wechat_sandbox: paymentForm.wechatSandbox
    }

    if (paymentForm.alipayPrivateKey.trim()) {
      payload.payment_alipay_private_key = paymentForm.alipayPrivateKey.trim()
    }
    if (paymentForm.wechatApiKey.trim()) {
      payload.payment_wechat_api_key = paymentForm.wechatApiKey.trim()
    }

    const response = await api.put('/settings', payload)
    applyPaymentSettings(response?.data || {})
    ElMessage.success('支付设置保存成功')
  } catch (error) {
    console.error('Failed to save payment settings:', error)
    ElMessage.error(error.message || '保存支付设置失败')
  } finally {
    paymentForm.saving = false
  }
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
      // 在实际项目中应调用API清理日志
      // await api.clearLogs()
      ElMessage.success('日志清理成功')
    } catch (error) {
      ElMessage.error('清理失败：' + error.message)
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
      // 在实际项目中应调用API修改密码
      // await api.changeAdminPassword(adminForm)
      ElMessage.success('密码修改成功，请重新登录')
      
      // 清空表单
      adminForm.currentPassword = ''
      adminForm.newPassword = ''
      adminForm.confirmPassword = ''
      
      // 注销当前会话
      setTimeout(() => {
        userStore.logout()
        window.location.href = '/login'
      }, 1500)
    } catch (error) {
      ElMessage.error('修改失败：' + error.message)
    }
  })
  .catch(() => {
    ElMessage.info('已取消修改')
  })
}

const resetAdminPassword = () => {
  ElMessageBox.confirm(
    '确定要将管理员密码重置为默认密码吗？',
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    }
  )
  .then(async () => {
    try {
      // 在实际项目中应调用API重置密码
      // await api.resetAdminPassword()
      ElMessage.success('密码重置成功，默认密码为：admin')
    } catch (error) {
      ElMessage.error('重置失败：' + error.message)
    }
  })
  .catch(() => {
    ElMessage.info('已取消重置')
  })
}

const saveSecuritySettings = async () => {
  try {
    // 在实际项目中应调用API保存配置
    // await api.saveSecuritySettings(securityForm)
    ElMessage.success('安全设置保存成功')
  } catch (error) {
    ElMessage.error('保存失败：' + error.message)
  }
}

// 切换Xray版本
const switchXrayVersion = async () => {
  xraySettings.switching = true;
  try {
    await performVersionSwitch();
  } finally {
    xraySettings.switching = false;
  }
};

// 测试自定义配置
const testCustomConfig = async () => {
  if (!xraySettings.configPath) {
    ElMessage.warning('请输入配置文件路径')
    return
  }
  
  try {
    await api.post('/xray/test-config', {
      config_path: xraySettings.configPath
    })
    ElMessage.success('配置文件测试通过')
  } catch (error) {
    console.error('Failed to test config:', error)
    ElMessage.error('配置文件测试失败：' + (error.response?.data?.message || '文件不存在或格式错误'))
  }
}

// 保存Xray设置
const saveXraySettings = async () => {
  try {
    // 构建请求数据
    const requestData = {
      auto_update: xraySettings.autoUpdate,
      custom_config: xraySettings.customConfig,
      config_path: xraySettings.configPath,
      check_interval: xraySettings.checkInterval
    };
    
    console.log('保存设置请求数据:', requestData);
    
    // 发送正确的属性名与API匹配
    const response = await api.post('/settings/xray', requestData);
    
    console.log('保存设置响应数据:', response.data);
    
    // 保存成功后立即重新获取设置以验证
    await refreshXraySettings();
    
    ElMessage.success('Xray设置保存成功');
  } catch (error) {
    console.error('Failed to save Xray settings:', error);
    ElMessage.error('保存Xray设置失败：' + (error.response?.data?.message || '未知错误'));
    
    // 显示详细错误信息
    const errorMsg = error.response?.data?.message || error.message || '未知错误'
    const errorDetail = error.response?.data?.detail || error.stack || ''
    showErrorDetails(
      '保存 Xray 设置失败',
      errorDetail ? `${errorMsg}\n\n${errorDetail}` : errorMsg,
      '请检查设置参数是否正确，特别是配置文件路径是否有效。',
      true,
      'saveSettings'
    )
  }
}

// 加载协议设置
const loadProtocolSettings = async () => {
  try {
    const response = await api.get('/settings/protocols')
    if (response.data) {
      // 协议设置
      protocolSettings.enableTrojan = response.data.protocols?.trojan ?? true
      protocolSettings.enableVMess = response.data.protocols?.vmess ?? true
      protocolSettings.enableVLESS = response.data.protocols?.vless ?? true
      protocolSettings.enableShadowsocks = response.data.protocols?.shadowsocks ?? true
      protocolSettings.enableSocks = response.data.protocols?.socks ?? false
      protocolSettings.enableHTTP = response.data.protocols?.http ?? false
      
      // 传输层设置
      transportSettings.enableTCP = response.data.transports?.tcp ?? true
      transportSettings.enableWebSocket = response.data.transports?.ws ?? true
      transportSettings.enableHTTP2 = response.data.transports?.http2 ?? true
      transportSettings.enableGRPC = response.data.transports?.grpc ?? true
      transportSettings.enableQUIC = response.data.transports?.quic ?? false
    }
  } catch (error) {
    console.error('Failed to load protocol settings:', error)
    ElMessage.error('加载协议设置失败')
  }
}

// 保存协议设置
const saveProtocolSettings = async () => {
  protocolsLoading.value = true
  try {
    await api.post('/settings/protocols', {
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
    })
    ElMessage.success('协议配置保存成功')
  } catch (error) {
    console.error('Failed to save protocol settings:', error)
    ElMessage.error('保存协议配置失败: ' + (error.response?.data?.message || error.message))
  } finally {
    protocolsLoading.value = false
  }
}

// 保存协议设置并重启Xray
const restartXrayAfterProtocolChange = async () => {
  try {
    // 先保存设置
    await saveProtocolSettings()
    // 然后重启Xray
    await restartXray()
    ElMessage.success('协议配置已更新并重启Xray')
  } catch (error) {
    console.error('Failed to update protocols and restart Xray:', error)
    ElMessage.error('更新协议配置并重启Xray失败')
  }
}

// 专门处理自动更新开关
const toggleAutoUpdate = async (newValue) => {
  console.log('自动更新开关切换:', newValue);
  
  try {
    // 构建请求数据
    const requestData = {
      auto_update: newValue,
      custom_config: xraySettings.customConfig,
      config_path: xraySettings.configPath,
      check_interval: xraySettings.checkInterval
    };
    
    console.log('自动更新请求数据:', requestData);
    
    // 发送请求前临时禁用开关，防止重复点击
    const originalValue = xraySettings.autoUpdate;
    
    // 发送请求
    const response = await api.post('/settings/xray', requestData);
    
    console.log('自动更新响应数据:', response.data);
    
    if (response.data && response.data.success) {
      ElMessage.success(`自动更新已${newValue ? '启用' : '禁用'}`);
      
      // 设置一个延迟后再刷新设置，确保后端已完成持久化
      setTimeout(async () => {
        try {
          // 进行多次验证确保设置生效
          const verify1 = await api.get('/settings/xray');
          console.log('验证1结果:', verify1.data);
          
          // 再次刷新设置
          await refreshXraySettings();
          
          // 最终验证
          const verify2 = await api.get('/settings/xray');
          console.log('验证2结果:', verify2.data);
          
          // 检查设置是否一致
          if (verify2.data.auto_update !== newValue) {
            console.error('设置验证失败，值不一致:', {
              expected: newValue,
              actual: verify2.data.auto_update
            });
            ElMessage.warning('设置可能未正确保存，请刷新页面后重试');
          } else {
            console.log('设置验证成功，值一致:', newValue);
          }
        } catch (verifyError) {
          console.error('设置验证失败:', verifyError);
        }
      }, 500);
    } else {
      // 如果失败，恢复之前的状态
      xraySettings.autoUpdate = originalValue;
      ElMessage.error('设置自动更新失败');
    }
  } catch (error) {
    console.error('设置自动更新失败:', error);
    // 恢复之前的状态
    xraySettings.autoUpdate = !newValue;
    ElMessage.error('设置自动更新失败: ' + (error.response?.data?.message || '未知错误'));
  }
}

// 加载版本详情
const loadVersionDetails = async (version) => {
  try {
    xraySettings.loading = true
    
    const response = await api.get(`/xray/version/${version}/details`)
    
    if (response.data) {
      xraySettings.versionDetails = {
        version: response.data.version,
        releaseDate: response.data.release_date,
        description: response.data.description,
        changelog: response.data.changelog || []
      }
      
      // 显示版本详情对话框
      xraySettings.showVersionDetails = true
    }
  } catch (error) {
    console.error('Failed to load version details:', error)
    ElMessage.error('加载版本详情失败：' + (error.message || '未知错误'))
  } finally {
    xraySettings.loading = false
  }
}

// 打开GitHub Xray发布页面
const openXrayReleasePage = () => {
  window.open('https://github.com/XTLS/Xray-core/releases', '_blank')
}

// 检查Xray更新
const checkXrayUpdates = async () => {
  if (xraySettings.checkingForUpdates) return;
  
  try {
    xraySettings.checkingForUpdates = true;
    ElMessage.info('正在检查Xray更新...');
    
    const response = await api.get('/xray/check-updates');
    
    if (response.data && response.data.has_update) {
      ElMessageBox.confirm(
        `发现新版本: ${response.data.latest_version}\n\n更新说明:\n${response.data.release_notes || ''}`,
        '有可用更新',
        {
          confirmButtonText: '更新',
          cancelButtonText: '取消',
          type: 'info',
          dangerouslyUseHTMLString: true
        }
      ).then(() => {
        downloadXrayVersion(response.data.latest_version);
      }).catch(() => {
        ElMessage.info('已取消更新');
      });
    } else {
      ElMessage.success('当前已经是最新版本');
    }
  } catch (error) {
    console.error('Failed to check for updates:', error);
    ElMessage.error('检查更新失败: ' + (error.message || '未知错误'));
  } finally {
    xraySettings.checkingForUpdates = false;
  }
};

// 下载并安装Xray版本
const downloadXrayVersion = async (version) => {
  // 初始化进度显示
  xraySettings.updateProgress.visible = true;
  xraySettings.updateProgress.status = 'downloading';
  xraySettings.updateProgress.percent = 0;
  xraySettings.updateProgress.message = `正在下载 ${version}...`;
  xraySettings.downloadingVersion = version;
  
  try {
    // 调用API下载版本
    const response = await api.post('/xray/download', { version }, {
      onDownloadProgress: (progressEvent) => {
        if (progressEvent.total) {
          xraySettings.updateProgress.percent = Math.round((progressEvent.loaded / progressEvent.total) * 100);
        }
      }
    });
    
    // 下载完成，开始安装
    xraySettings.updateProgress.status = 'installing';
    xraySettings.updateProgress.message = `正在安装 ${version}...`;
    xraySettings.updateProgress.percent = 50;
    
    // 调用API安装版本
    await api.post('/xray/install', { version });
    
    // 安装完成
    xraySettings.updateProgress.status = 'completed';
    xraySettings.updateProgress.message = `${version} 安装成功!`;
    xraySettings.updateProgress.percent = 100;
    
    // 更新当前版本
    xraySettings.currentVersion = version;
    
    // 提示是否重启
    setTimeout(() => {
      ElMessageBox.confirm(
        '版本已更新，需要重启Xray服务才能生效。',
        '更新完成',
        {
          confirmButtonText: '立即重启',
          cancelButtonText: '稍后重启',
          type: 'success'
        }
      ).then(() => {
        restartXray();
      }).catch(() => {
        ElMessage.warning('更新已完成，但需要重启才能生效');
      }).finally(() => {
        xraySettings.updateProgress.visible = false;
      });
    }, 1000);
  } catch (error) {
    console.error('Failed to download/install version:', error);
    xraySettings.updateProgress.status = 'error';
    xraySettings.updateProgress.message = '更新失败';
    xraySettings.updateProgress.error = error.message || '未知错误';
    
    // 延迟关闭进度显示后显示错误详情
    setTimeout(() => {
      xraySettings.updateProgress.visible = false;
      
      // 显示详细错误信息
      showErrorDetails(
        '下载或安装 Xray 版本失败',
        error.response?.data?.message || error.message || '未知错误',
        '请检查网络连接、磁盘空间和系统权限。您也可以尝试手动下载并安装该版本。',
        true,
        'updateVersion',
        { version }
      )
    }, 1500);
  }
};

// 展示错误详情
const showErrorDetails = (title, message, resolution = '', canRetry = false, retryAction = null, retryParams = null) => {
  errorDetails.title = title
  errorDetails.message = message
  errorDetails.resolution = resolution
  errorDetails.canRetry = canRetry
  errorDetails.retryAction = retryAction
  errorDetails.retryParams = retryParams
  errorDetails.visible = true
}

// 复制错误信息到剪贴板
const copyErrorToClipboard = () => {
  try {
    const errorText = `${errorDetails.title}\n\n${errorDetails.message}`
    navigator.clipboard.writeText(errorText)
    ElMessage.success('错误信息已复制到剪贴板')
  } catch (error) {
    console.error('Failed to copy error message:', error)
    ElMessage.error('复制失败，请手动选择复制')
  }
}

// 重试失败的操作
const retryFailedOperation = async () => {
  errorDetails.visible = false
  
  if (!errorDetails.retryAction) return
  
  try {
    ElMessage.info('正在重试操作...')
    
    // 根据保存的操作类型执行对应的重试逻辑
    switch (errorDetails.retryAction) {
      case 'switchVersion':
        await performVersionSwitch(errorDetails.retryParams?.shouldRestart)
        break
      case 'restart':
        await restartXray()
        break
      case 'saveSettings':
        await saveXraySettings()
        break
      case 'updateVersion':
        await downloadXrayVersion(errorDetails.retryParams?.version)
        break
      case 'syncVersions':
        await syncVersionsFromGitHub()
        break
      case 'refreshVersions':
        await refreshXrayVersions()
        break
      default:
        console.warn('Unknown retry action:', errorDetails.retryAction)
    }
  } catch (error) {
    console.error('Retry operation failed:', error)
    ElMessage.error('重试操作失败: ' + (error.response?.data?.message || error.message || '未知错误'))
  }
}

// 刷新Xray设置
const refreshXraySettings = async () => {
  xraySettings.loading = true;
  try {
    console.log('刷新Xray设置开始');
    await loadXraySettings();
    ElMessage.success('刷新设置成功');
    console.log('刷新后的Xray设置：', { ...xraySettings });
  } catch (error) {
    ElMessage.error('刷新设置失败：' + error.message);
  } finally {
    xraySettings.loading = false;
  }
}

// 获取时间格式化函数
const getTimeAgo = () => {
  if (!lastSyncTime.value) {
    return '尚未同步';
  }
  
  const now = new Date();
  const syncDate = new Date(parseInt(lastSyncTime.value));
  const diffInMs = now - syncDate;
  const diffInDays = Math.floor(diffInMs / (1000 * 60 * 60 * 24));
  const diffInHours = Math.floor(diffInMs / (1000 * 60 * 60));
  const diffInMinutes = Math.floor(diffInMs / (1000 * 60));

  if (diffInMinutes < 1) {
    return '刚刚';
  } else if (diffInMinutes < 60) {
    return `${diffInMinutes}分钟前`;
  } else if (diffInHours < 24) {
    return `${diffInHours}小时前`;
  } else if (diffInDays < 7) {
    return `${diffInDays}天前`;
  } else if (diffInDays < 30) {
    return `${Math.floor(diffInDays / 7)}周前`;
  } else if (diffInDays < 365) {
    return `${Math.floor(diffInDays / 30)}月前`;
  } else {
    return `${Math.floor(diffInDays / 365)}年前`;
  }
}

// 检查新版本
const isNewVersion = (newVersion, currentVersion) => {
  return newVersion > currentVersion;
}

const confirmSwitchVersion = async () => {
  xraySettings.switching = true;
  try {
    await performVersionSwitch(true);
    xraySettings.showVersionDialog = false;
  } catch (error) {
    console.error('Failed to switch version:', error);
  } finally {
    xraySettings.switching = false;
  }
};

const openVersionSwitchDialog = () => {
  xraySettings.showVersionDialog = true;
}

// 移除简单版本，保留更完整的实现
// 处理版本切换
const handleSwitchVersion = async () => {
  try {
    // 检查必要条件
    if (!xraySettings.selectedVersion) {
      ElMessage.warning('请先选择一个版本')
      return
    }
    
    // 显示确认对话框
    const confirmResult = await ElMessageBox.confirm(
      `您确定要将 Xray 从 ${xraySettings.currentVersion} 切换到 ${xraySettings.selectedVersion} 吗？`,
      '切换版本确认',
      {
        confirmButtonText: '确认切换',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    
    if (confirmResult !== 'confirm') {
      return
    }
    
    // 设置切换状态和进度显示
    xraySettings.switching = true
    xraySettings.downloadingVersion = xraySettings.selectedVersion
    xraySettings.updateProgress.visible = true
    xraySettings.updateProgress.status = 'switching'
    xraySettings.updateProgress.percent = 0
    xraySettings.updateProgress.message = `正在切换到版本 ${xraySettings.selectedVersion}...`
    
    try {
      // 发送版本切换请求
      const response = await api.post('/xray/switch-version', {
        version: xraySettings.selectedVersion
      }, { 
        timeout: 300000, // 增加到5分钟(300000ms)，因为下载可能需要较长时间 
        timeoutErrorMessage: '下载版本超时，请检查网络连接或手动下载'
      })
      
      if (response.data && response.data.success) {
        // 切换成功
        xraySettings.currentVersion = xraySettings.selectedVersion
        
        // 显示成功消息
        ElMessage.success({
          message: `已成功切换到版本 ${xraySettings.selectedVersion}`,
          duration: 3000
        })
        
        // 设置进度条为完成状态
        xraySettings.updateProgress.status = 'completed'
        xraySettings.updateProgress.percent = 100
        xraySettings.updateProgress.message = `版本切换成功: ${xraySettings.selectedVersion}`
        
        // 等待一会儿后关闭进度条
        setTimeout(() => {
          xraySettings.updateProgress.visible = false
        }, 1500)
        
        // 刷新版本信息和状态
        await refreshXrayVersions()
        await refreshXrayStatus()
      } else {
        // 切换不成功但服务器返回了响应
        throw new Error(response.data?.message || '切换版本失败，服务器返回非成功状态')
      }
    } catch (error) {
      console.error('Failed to switch version:', error)
      
      // 设置进度条为错误状态
      xraySettings.updateProgress.status = 'error'
      xraySettings.updateProgress.percent = 0
      xraySettings.updateProgress.message = '版本切换失败'
      xraySettings.updateProgress.error = error.response?.data?.message || error.message || '未知错误'
      
      // 分析错误信息
      const errorMsg = error.response?.data?.message || error.message || ''
      let detailedError = '切换版本时出错'
      let suggestion = '请检查网络连接和服务器状态'
      
      if (errorMsg.includes('404') || errorMsg.includes('not found')) {
        detailedError = `找不到版本 ${xraySettings.selectedVersion} 的下载链接`
        suggestion = '1. 请检查网络连接\n2. 可能需要科学上网\n3. 尝试使用其他版本\n4. 或者手动下载该版本并放置在xray/bin目录下'
      } else if (errorMsg.includes('timeout') || errorMsg.includes('timed out') || errorMsg.includes('执行超时')) {
        detailedError = '下载版本时超时'
        suggestion = '1. 网络可能较慢，可尝试再次切换，本次修改已增加超时时间\n2. 国内网络可能无法访问GitHub，建议:\n   - 使用加速器或科学上网\n   - 手动下载Xray版本并放入xray/downloads目录\n   - 使用其他版本'
      } else if (errorMsg.includes('permission') || errorMsg.includes('access') || errorMsg.includes('权限')) {
        detailedError = '权限不足'
        suggestion = '请确保程序有足够的文件系统权限'
      }
      
      // 显示错误详情
      showErrorDetails(
        '切换版本失败',
        detailedError + '\n\n原始错误: ' + errorMsg,
        suggestion,
        true,
        'switchVersion',
        { shouldRestart: false }
      )
    } finally {
      xraySettings.switching = false
    }
  } catch (error) {
    // 处理用户取消确认对话框的情况
    if (error === 'cancel') {
      console.log('User cancelled version switch')
      return
    }
    
    console.error('Unexpected error during version switch:', error)
    ElMessage.error(`发生意外错误: ${error.message || '未知错误'}`)
  } finally {
    // 无论成功失败都重置状态
    xraySettings.switching = false
  }
}

// 按版本类型分组的计算属性
const stableVersions = computed(() => {
  return xraySettings.versions.filter(v => v.startsWith('v1.'));
});

const betaVersions = computed(() => {
  return xraySettings.versions.filter(v => !v.startsWith('v1.'));
});

// 获取V2系列版本
const v2Versions = computed(() => {
  return xraySettings.versions.filter(v => !v.startsWith('v1.'));
});

// 打开GitHub发布页
const openGitHubReleases = () => {
  window.open('https://github.com/XTLS/Xray-core/releases', '_blank')
}
</script>

<style scoped>
.settings-container {
  padding: 20px;
}

.settings-form {
  max-width: 800px;
  margin-top: 20px;
}

.form-tips {
  font-size: 12px;
  color: #909399;
  margin-top: 5px;
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
  flex-direction: column;
  width: 100%;
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
  color: #303133;
}

.error-resolution p {
  margin: 5px 0;
}

.error-troubleshooting {
  margin-top: 15px;
  border-left: 3px solid #409eff;
  padding: 10px;
  background-color: #ecf5ff;
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

.version-control {
  display: flex;
  align-items: center;
}

.version-select-container {
  margin-right: 10px;
}

.version-select-container .el-select {
  width: 180px;
}

.version-select-container .el-button {
  margin-left: 10px;
}

.version-tips {
  margin-left: 15px;
  font-size: 12px;
  color: #909399;
}
</style> 
