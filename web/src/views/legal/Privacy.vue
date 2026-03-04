<template>
  <div class="legal-page">
    <div class="legal-container">
      <h1 class="legal-title">隐私政策</h1>
      <div class="legal-meta">
        <span>生效日期：{{ effectiveDate }}</span>
        <span>最后更新：{{ lastUpdated }}</span>
      </div>

      <div class="legal-content">
        <section>
          <h2>引言</h2>
          <p>我们非常重视您的隐私。本隐私政策说明了我们如何收集、使用、存储和保护您的个人信息。</p>
          <p>使用我们的服务即表示您同意本隐私政策中描述的做法。</p>
        </section>

        <section>
          <h2>1. 我们收集的信息</h2>
          
          <h3>1.1 您主动提供的信息</h3>
          <ul>
            <li><strong>账户信息：</strong>用户名、电子邮件地址、密码（加密存储）</li>
            <li><strong>个人资料：</strong>您选择提供的其他信息</li>
            <li><strong>支付信息：</strong>通过第三方支付处理器处理，我们不存储完整的支付卡信息</li>
            <li><strong>通信记录：</strong>您与我们的客服或支持团队的沟通内容</li>
          </ul>

          <h3>1.2 自动收集的信息</h3>
          <ul>
            <li><strong>使用数据：</strong>访问时间、使用的功能、操作记录</li>
            <li><strong>设备信息：</strong>IP 地址、浏览器类型、操作系统、设备标识符</li>
            <li><strong>日志数据：</strong>服务器日志、错误报告、性能数据</li>
            <li><strong>Cookie 和类似技术：</strong>用于身份验证和偏好设置</li>
          </ul>

          <h3>1.3 服务使用信息</h3>
          <ul>
            <li><strong>流量统计：</strong>上传/下载流量、连接时长</li>
            <li><strong>节点使用：</strong>使用的节点、连接记录</li>
            <li><strong>订阅信息：</strong>套餐类型、到期时间、续费记录</li>
          </ul>
        </section>

        <section>
          <h2>2. 信息使用方式</h2>
          <p>我们使用收集的信息用于：</p>
          <ul>
            <li><strong>提供服务：</strong>创建和管理您的账户，提供您请求的服务</li>
            <li><strong>改进服务：</strong>分析使用模式，优化性能和用户体验</li>
            <li><strong>客户支持：</strong>响应您的询问和请求，解决技术问题</li>
            <li><strong>安全保护：</strong>检测和防止欺诈、滥用和安全威胁</li>
            <li><strong>通信：</strong>发送服务通知、更新和营销信息（您可以选择退出）</li>
            <li><strong>合规：</strong>遵守法律义务和执行我们的条款</li>
            <li><strong>统计分析：</strong>生成匿名的使用统计和趋势分析</li>
          </ul>
        </section>

        <section>
          <h2>3. 信息共享</h2>
          <p>我们不会出售您的个人信息。我们可能在以下情况下共享信息：</p>
          
          <h3>3.1 服务提供商</h3>
          <p>我们可能与第三方服务提供商共享信息，以帮助我们运营服务：</p>
          <ul>
            <li>云服务提供商（数据存储和处理）</li>
            <li>支付处理器（处理交易）</li>
            <li>分析服务（了解服务使用情况）</li>
            <li>客户支持工具（提供技术支持）</li>
          </ul>
          <p>这些提供商仅在必要范围内访问信息，并受保密协议约束。</p>

          <h3>3.2 法律要求</h3>
          <p>在以下情况下，我们可能披露您的信息：</p>
          <ul>
            <li>遵守法律、法规或法律程序</li>
            <li>响应政府或执法机构的合法请求</li>
            <li>保护我们的权利、财产或安全</li>
            <li>防止欺诈或安全威胁</li>
          </ul>

          <h3>3.3 业务转让</h3>
          <p>如果我们参与合并、收购或资产出售，您的信息可能会作为交易的一部分转让。</p>
        </section>

        <section>
          <h2>4. 数据安全</h2>
          <p>我们采取合理的技术和组织措施来保护您的信息：</p>
          <ul>
            <li><strong>加密：</strong>使用 HTTPS/TLS 加密传输数据</li>
            <li><strong>密码保护：</strong>使用行业标准的哈希算法存储密码</li>
            <li><strong>访问控制：</strong>限制对个人信息的访问权限</li>
            <li><strong>安全审计：</strong>定期进行安全评估和漏洞扫描</li>
            <li><strong>备份：</strong>定期备份数据以防止丢失</li>
          </ul>
          <p>但请注意，没有任何方法可以保证 100% 的安全。您也有责任保护自己的账户凭据。</p>
        </section>

        <section>
          <h2>5. 数据保留</h2>
          <p>我们会在必要期限内保留您的信息：</p>
          <ul>
            <li><strong>账户信息：</strong>在账户活跃期间及之后合理期限内保留</li>
            <li><strong>交易记录：</strong>根据法律要求保留（通常为 3-7 年）</li>
            <li><strong>日志数据：</strong>通常保留 30-90 天</li>
            <li><strong>已删除账户：</strong>在删除后 30 天内完全清除数据</li>
          </ul>
          <p>您可以随时请求删除您的账户和相关数据。</p>
        </section>

        <section>
          <h2>6. 您的权利</h2>
          <p>根据适用的数据保护法律，您拥有以下权利：</p>
          <ul>
            <li><strong>访问权：</strong>请求访问我们持有的关于您的个人信息</li>
            <li><strong>更正权：</strong>更正不准确或不完整的信息</li>
            <li><strong>删除权：</strong>请求删除您的个人信息</li>
            <li><strong>限制处理权：</strong>在某些情况下限制我们处理您的信息</li>
            <li><strong>数据可携权：</strong>以结构化、常用格式接收您的数据</li>
            <li><strong>反对权：</strong>反对我们处理您的信息</li>
            <li><strong>撤回同意权：</strong>撤回您之前给予的同意</li>
          </ul>
          <p>要行使这些权利，请通过下方联系方式与我们联系。</p>
        </section>

        <section>
          <h2>7. Cookie 和跟踪技术</h2>
          <p>我们使用 Cookie 和类似技术来：</p>
          <ul>
            <li>保持您的登录状态</li>
            <li>记住您的偏好设置</li>
            <li>分析网站流量和使用模式</li>
            <li>提供个性化内容</li>
          </ul>
          <p>您可以通过浏览器设置管理 Cookie 偏好，但这可能影响某些功能的使用。</p>
        </section>

        <section>
          <h2>8. 第三方链接</h2>
          <p>我们的服务可能包含指向第三方网站的链接。我们不对这些网站的隐私做法负责。</p>
          <p>我们建议您查看访问的任何第三方网站的隐私政策。</p>
        </section>

        <section>
          <h2>9. 儿童隐私</h2>
          <p>我们的服务不面向 18 岁以下的儿童。我们不会故意收集儿童的个人信息。</p>
          <p>如果您认为我们可能持有儿童的信息，请联系我们，我们将立即删除。</p>
        </section>

        <section>
          <h2>10. 国际数据传输</h2>
          <p>您的信息可能会被传输到您所在国家/地区以外的服务器并在那里处理。</p>
          <p>我们会采取适当措施确保您的数据在传输和处理过程中得到充分保护。</p>
        </section>

        <section>
          <h2>11. 隐私政策变更</h2>
          <p>我们可能会不时更新本隐私政策。重大变更将通过以下方式通知您：</p>
          <ul>
            <li>在服务中显示通知</li>
            <li>发送电子邮件通知</li>
            <li>更新本页面顶部的"最后更新"日期</li>
          </ul>
          <p>继续使用服务即表示您接受修订后的隐私政策。</p>
        </section>

        <section>
          <h2>12. 联系我们</h2>
          <p>如对本隐私政策有任何疑问或要行使您的权利，请通过以下方式联系我们：</p>
          <ul>
            <li>邮箱：privacy@example.com</li>
            <li>工单系统：<router-link to="/user/tickets" class="link">提交工单</router-link></li>
          </ul>
          <p>我们将在 30 天内回复您的请求。</p>
        </section>
      </div>

      <div class="legal-footer">
        <el-button @click="goBack">返回</el-button>
        <el-button type="primary" @click="acceptPrivacy">我已阅读并理解</el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

const router = useRouter()

const effectiveDate = ref('2024-01-01')
const lastUpdated = ref('2024-01-01')

const goBack = () => {
  router.back()
}

const acceptPrivacy = () => {
  ElMessage.success('感谢您阅读隐私政策')
  router.back()
}
</script>

<style scoped>
.legal-page {
  min-height: 100vh;
  background: #f5f7fa;
  padding: 40px 20px;
}

.legal-container {
  max-width: 900px;
  margin: 0 auto;
  background: white;
  border-radius: 8px;
  padding: 40px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.legal-title {
  font-size: 32px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 16px;
  text-align: center;
}

.legal-meta {
  display: flex;
  justify-content: center;
  gap: 24px;
  color: #909399;
  font-size: 14px;
  margin-bottom: 40px;
  padding-bottom: 24px;
  border-bottom: 1px solid #ebeef5;
}

.legal-content {
  line-height: 1.8;
  color: #606266;
}

.legal-content section {
  margin-bottom: 32px;
}

.legal-content h2 {
  font-size: 20px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 16px;
}

.legal-content h3 {
  font-size: 16px;
  font-weight: 600;
  color: #606266;
  margin: 16px 0 12px;
}

.legal-content p {
  margin-bottom: 12px;
  text-align: justify;
}

.legal-content ul {
  margin: 12px 0;
  padding-left: 24px;
}

.legal-content li {
  margin-bottom: 8px;
}

.legal-content strong {
  color: #303133;
  font-weight: 600;
}

.legal-content .link {
  color: #409eff;
  text-decoration: none;
}

.legal-content .link:hover {
  text-decoration: underline;
}

.legal-footer {
  margin-top: 40px;
  padding-top: 24px;
  border-top: 1px solid #ebeef5;
  display: flex;
  justify-content: center;
  gap: 16px;
}

@media (max-width: 768px) {
  .legal-container {
    padding: 24px 16px;
  }

  .legal-title {
    font-size: 24px;
  }

  .legal-meta {
    flex-direction: column;
    gap: 8px;
  }
}
</style>
