<template>
  <div class="terms-container">
    <div class="terms-header">
      <h1>服务条款</h1>
      <p class="update-time">
        最后更新：{{ updateTime }}
      </p>
    </div>

    <div class="terms-content">
      <section>
        <h2>1. 服务说明</h2>
        <p>欢迎使用本服务。在使用本服务前，请您仔细阅读并理解本服务条款的全部内容。</p>
        <p>本服务是一个代理服务管理平台，为用户提供代理节点管理、订阅生成、流量统计等功能。</p>
      </section>

      <section>
        <h2>2. 用户注册与账户</h2>
        <h3>2.1 注册资格</h3>
        <p>您必须年满 18 周岁才能注册和使用本服务。注册时，您需要提供真实、准确、完整的信息。</p>
        
        <h3>2.2 账户安全</h3>
        <p>您有责任维护账户的安全性和保密性。您对使用您的账户进行的所有活动负责。</p>
        <p>如发现账户被未经授权使用，请立即通知我们。</p>
      </section>

      <section>
        <h2>3. 服务使用规范</h2>
        <h3>3.1 合法使用</h3>
        <p>您承诺仅将本服务用于合法目的，不得：</p>
        <ul>
          <li>违反任何适用的法律法规</li>
          <li>侵犯他人的知识产权或其他权利</li>
          <li>传播恶意软件、病毒或其他有害代码</li>
          <li>进行网络攻击、扫描或渗透测试</li>
          <li>发送垃圾邮件或进行其他滥用行为</li>
        </ul>

        <h3>3.2 禁止行为</h3>
        <p>严禁使用本服务进行以下活动：</p>
        <ul>
          <li>任何违法犯罪活动</li>
          <li>侵犯他人隐私或数据</li>
          <li>传播非法、有害、威胁、辱骂、骚扰、侵权、诽谤、粗俗、淫秽或其他令人反感的内容</li>
          <li>干扰或破坏服务器或网络</li>
        </ul>
      </section>

      <section>
        <h2>4. 服务内容</h2>
        <h3>4.1 服务提供</h3>
        <p>我们提供以下服务：</p>
        <ul>
          <li>代理节点管理</li>
          <li>订阅链接生成</li>
          <li>流量统计与监控</li>
          <li>多协议支持（Shadowsocks、VMess、Trojan 等）</li>
        </ul>

        <h3>4.2 服务变更</h3>
        <p>我们保留随时修改、暂停或终止服务的权利，恕不另行通知。</p>
      </section>

      <section>
        <h2>5. 付费服务</h2>
        <h3>5.1 套餐与定价</h3>
        <p>部分服务功能需要付费订阅。具体套餐和价格请参考套餐页面。</p>

        <h3>5.2 支付与退款</h3>
        <p>支付成功后，服务将立即生效。除非法律另有规定或我们明确同意，否则已支付的费用不予退还。</p>

        <h3>5.3 自动续费</h3>
        <p>如您选择自动续费，我们将在到期前自动从您的账户扣款。您可以随时取消自动续费。</p>
      </section>

      <section>
        <h2>6. 知识产权</h2>
        <p>本服务的所有内容，包括但不限于文本、图形、标识、图标、图像、音频片段、数字下载、数据编辑和软件，均为我们或内容提供者的财产，受版权法保护。</p>
      </section>

      <section>
        <h2>7. 免责声明</h2>
        <h3>7.1 服务可用性</h3>
        <p>我们努力确保服务的可用性，但不保证服务不会中断或无错误。</p>

        <h3>7.2 内容准确性</h3>
        <p>我们不对服务中的信息、内容或材料的准确性、完整性或可靠性作出任何保证。</p>

        <h3>7.3 第三方链接</h3>
        <p>服务可能包含第三方网站的链接。我们不对这些网站的内容或隐私做法负责。</p>
      </section>

      <section>
        <h2>8. 责任限制</h2>
        <p>在法律允许的最大范围内，我们不对以下情况承担责任：</p>
        <ul>
          <li>因使用或无法使用服务而导致的任何直接、间接、偶然、特殊或后果性损害</li>
          <li>数据丢失或损坏</li>
          <li>业务中断或利润损失</li>
          <li>第三方的行为或不作为</li>
        </ul>
      </section>

      <section>
        <h2>9. 账户终止</h2>
        <h3>9.1 用户终止</h3>
        <p>您可以随时停止使用服务并删除您的账户。</p>

        <h3>9.2 我们的终止权</h3>
        <p>如您违反本条款，我们有权立即暂停或终止您的账户，恕不另行通知。</p>
      </section>

      <section>
        <h2>10. 条款变更</h2>
        <p>我们保留随时修改本服务条款的权利。修改后的条款将在本页面发布。继续使用服务即表示您接受修改后的条款。</p>
      </section>

      <section>
        <h2>11. 适用法律</h2>
        <p>本条款受中华人民共和国法律管辖。因本条款引起的任何争议应通过友好协商解决。</p>
      </section>

      <section>
        <h2>12. 联系我们</h2>
        <p>如您对本服务条款有任何疑问，请通过以下方式联系我们：</p>
        <ul>
          <li>邮箱：support@example.com</li>
          <li>工单系统：登录后提交工单</li>
        </ul>
      </section>
    </div>

    <div class="terms-footer">
      <el-button @click="goBack">
        返回
      </el-button>
      <el-button
        type="primary"
        @click="acceptAndClose"
      >
        我已阅读并同意
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

const router = useRouter()
const updateTime = ref('2024年1月1日')

const goBack = () => {
  router.back()
}

const acceptAndClose = () => {
  ElMessage.success('感谢您的阅读')
  router.back()
}
</script>

<style scoped>
.terms-container {
  max-width: 900px;
  margin: 0 auto;
  padding: 40px 20px;
  background: var(--color-bg-card);
  min-height: 100vh;
  box-sizing: border-box;
}

.terms-header {
  text-align: center;
  margin-bottom: 40px;
  padding-bottom: 20px;
  border-bottom: 2px solid var(--color-border);
}

.terms-header h1 {
  font-size: 32px;
  color: var(--color-text-primary);
  margin-bottom: 10px;
}

.update-time {
  color: var(--color-text-secondary);
  font-size: 14px;
}

.terms-content {
  line-height: 1.8;
  color: var(--color-text-primary);
  overflow-wrap: anywhere;
  word-break: break-word;
}

.terms-content section {
  margin-bottom: 30px;
}

.terms-content h2 {
  font-size: 24px;
  color: var(--color-text-primary);
  margin-bottom: 15px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--color-border);
}

.terms-content h3 {
  font-size: 18px;
  color: var(--color-text-regular);
  margin: 20px 0 10px;
}

.terms-content p {
  margin-bottom: 15px;
  text-align: justify;
}

.terms-content ul {
  margin: 10px 0 15px 20px;
  padding-left: 20px;
}

.terms-content li {
  margin-bottom: 8px;
  list-style-type: disc;
}

.terms-footer {
  margin-top: 40px;
  padding-top: 20px;
  border-top: 2px solid var(--color-border);
  display: flex;
  justify-content: center;
  gap: 12px;
  flex-wrap: wrap;
}

.terms-footer .el-button {
  min-width: 120px;
}

@media (max-width: 768px) {
  .terms-container {
    padding: 24px 16px;
  }

  .terms-header {
    margin-bottom: 28px;
  }

  .terms-header h1 {
    font-size: 28px;
  }

  .terms-content section {
    margin-bottom: 24px;
  }

  .terms-content h2 {
    font-size: 22px;
  }

  .terms-content h3 {
    font-size: 17px;
  }

  .terms-content p {
    text-align: left;
  }

  .terms-content ul {
    margin-left: 0;
    padding-left: 18px;
  }
}

@media (max-width: 480px) {
  .terms-container {
    padding: 20px 12px;
  }

  .terms-header h1 {
    font-size: 24px;
  }

  .terms-content h2 {
    font-size: 20px;
  }

  .terms-footer {
    flex-direction: column;
  }

  .terms-footer .el-button {
    width: 100%;
    margin-left: 0;
  }
}
</style>
