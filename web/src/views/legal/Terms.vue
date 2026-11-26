<template>
  <div class="legal-page">
    <div class="legal-container">
      <h1 class="legal-title">
        服务条款
      </h1>
      <div class="legal-meta">
        <span>生效日期：{{ effectiveDate }}</span>
        <span>最后更新：{{ lastUpdated }}</span>
      </div>

      <div class="legal-content">
        <section>
          <h2>1. 服务说明</h2>
          <p>欢迎使用本服务。通过访问或使用本服务，您同意受本服务条款的约束。</p>
          <p>本服务提供代理节点管理、订阅服务、流量统计等功能。我们保留随时修改或终止服务的权利，恕不另行通知。</p>
        </section>

        <section>
          <h2>2. 用户账户</h2>
          <p>2.1 您需要创建账户才能使用本服务。注册时，您必须提供准确、完整的信息。</p>
          <p>2.2 您有责任维护账户安全，并对账户下的所有活动负责。</p>
          <p>2.3 如发现账户被未经授权使用，请立即通知我们。</p>
          <p>2.4 我们保留随时暂停或终止违反本条款的账户的权利。</p>
        </section>

        <section>
          <h2>3. 使用规范</h2>
          <p>使用本服务时，您同意不会：</p>
          <ul>
            <li>违反任何适用的法律法规</li>
            <li>侵犯他人的知识产权或其他权利</li>
            <li>上传、传输或分发任何恶意软件或病毒</li>
            <li>干扰或破坏服务的正常运行</li>
            <li>尝试未经授权访问系统或网络</li>
            <li>使用自动化工具（如机器人）访问服务</li>
            <li>转售或商业化利用服务</li>
          </ul>
        </section>

        <section>
          <h2>4. 付费服务</h2>
          <p>4.1 部分功能可能需要付费订阅。价格和计费周期将在购买时明确说明。</p>
          <p>4.2 订阅费用不可退款，除非法律另有规定。</p>
          <p>4.3 我们保留随时调整价格的权利，但会提前通知现有用户。</p>
          <p>4.4 未按时支付费用可能导致服务暂停或终止。</p>
        </section>

        <section>
          <h2>5. 知识产权</h2>
          <p>5.1 本服务及其所有内容（包括但不限于文本、图形、代码、界面）均受知识产权法保护。</p>
          <p>5.2 未经明确授权，您不得复制、修改、分发或创建衍生作品。</p>
          <p>5.3 您保留对自己上传内容的所有权，但授予我们使用这些内容以提供服务的许可。</p>
        </section>

        <section>
          <h2>6. 隐私保护</h2>
          <p>
            我们重视您的隐私。有关我们如何收集、使用和保护您的个人信息，请参阅我们的<router-link
              to="/legal/privacy"
              class="link"
            >
              隐私政策
            </router-link>。
          </p>
        </section>

        <section>
          <h2>7. 免责声明</h2>
          <p>7.1 本服务按"现状"提供，不提供任何明示或暗示的保证。</p>
          <p>7.2 我们不保证服务不会中断、及时、安全或无错误。</p>
          <p>7.3 您使用本服务的风险由您自行承担。</p>
          <p>7.4 我们不对因使用或无法使用服务而导致的任何损失负责。</p>
        </section>

        <section>
          <h2>8. 责任限制</h2>
          <p>在法律允许的最大范围内，我们对任何间接、偶然、特殊、后果性或惩罚性损害不承担责任，包括但不限于利润损失、数据丢失或业务中断。</p>
          <p>我们的总责任不超过您在过去12个月内支付给我们的费用。</p>
        </section>

        <section>
          <h2>9. 服务变更与终止</h2>
          <p>9.1 我们保留随时修改、暂停或终止服务的权利。</p>
          <p>9.2 您可以随时停止使用服务并删除账户。</p>
          <p>9.3 服务终止后，您的数据可能会被删除，且无法恢复。</p>
        </section>

        <section>
          <h2>10. 条款变更</h2>
          <p>我们可能会不时更新本服务条款。重大变更将通过电子邮件或服务内通知告知您。</p>
          <p>继续使用服务即表示您接受修订后的条款。</p>
        </section>

        <section>
          <h2>11. 争议解决</h2>
          <p>11.1 本条款受中华人民共和国法律管辖。</p>
          <p>11.2 因本条款引起的任何争议，双方应首先通过友好协商解决。</p>
          <p>11.3 协商不成的，任何一方可向服务提供方所在地人民法院提起诉讼。</p>
        </section>

        <section>
          <h2>12. 联系我们</h2>
          <p>如对本服务条款有任何疑问，请通过以下方式联系我们：</p>
          <ul>
            <li>邮箱：support@example.com</li>
            <li>
              工单系统：<router-link
                to="/user/tickets"
                class="link"
              >
                提交工单
              </router-link>
            </li>
          </ul>
        </section>
      </div>

      <div class="legal-footer">
        <el-button @click="goBack">
          返回
        </el-button>
        <el-button
          type="primary"
          @click="acceptTerms"
        >
          我已阅读并同意
        </el-button>
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

const acceptTerms = () => {
  ElMessage.success('感谢您阅读服务条款')
  router.back()
}
</script>

<style scoped>
.legal-page {
  min-height: 100vh;
  background: var(--color-bg-page);
  padding: 40px 20px;
}

.legal-container {
  max-width: 900px;
  margin: 0 auto;
  background: var(--color-bg-card);
  border-radius: 8px;
  padding: 40px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.legal-title {
  font-size: 32px;
  font-weight: 600;
  color: var(--color-text-primary);
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
  color: var(--color-text-primary);
  margin-bottom: 16px;
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
