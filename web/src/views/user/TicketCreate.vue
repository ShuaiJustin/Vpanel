<template>
  <div class="ticket-create-page">
    <!-- 返回按钮 -->
    <div class="back-bar">
      <el-button
        link
        @click="goBack"
      >
        <el-icon><ArrowLeft /></el-icon>
        返回工单列表
      </el-button>
    </div>

    <el-card
      class="create-card"
      shadow="never"
    >
      <template #header>
        <h2 class="card-title">
          创建工单
        </h2>
      </template>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="80px"
        label-position="top"
      >
        <el-form-item
          label="工单主题"
          prop="subject"
        >
          <el-input
            v-model="form.subject"
            placeholder="请简要描述您的问题"
            maxlength="100"
            show-word-limit
          />
        </el-form-item>

        <el-form-item
          label="问题类型"
          prop="category"
        >
          <el-select
            v-model="form.category"
            placeholder="请选择问题类型"
            style="width: 100%"
          >
            <el-option
              label="账户问题"
              value="account"
            />
            <el-option
              label="连接问题"
              value="connection"
            />
            <el-option
              label="订阅问题"
              value="subscription"
            />
            <el-option
              label="支付问题"
              value="payment"
            />
            <el-option
              label="功能建议"
              value="suggestion"
            />
            <el-option
              label="其他问题"
              value="other"
            />
          </el-select>
        </el-form-item>

        <el-form-item
          label="优先级"
          prop="priority"
        >
          <el-radio-group v-model="form.priority">
            <el-radio value="low">
              低
            </el-radio>
            <el-radio value="normal">
              普通
            </el-radio>
            <el-radio value="high">
              高
            </el-radio>
            <el-radio value="urgent">
              紧急
            </el-radio>
          </el-radio-group>
          <div class="form-tip">
            请根据问题的紧急程度选择优先级，紧急问题将优先处理
          </div>
        </el-form-item>

        <el-form-item
          label="问题描述"
          prop="content"
        >
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="6"
            placeholder="请详细描述您遇到的问题，包括：&#10;1. 问题发生的时间&#10;2. 使用的客户端和版本&#10;3. 具体的错误信息&#10;4. 已尝试的解决方法"
            maxlength="2000"
            show-word-limit
          />
        </el-form-item>

        <el-form-item label="附件">
          <el-upload
            v-model:file-list="form.attachments"
            :auto-upload="false"
            :limit="3"
            accept=".jpg,.jpeg,.png,.gif,.pdf,.doc,.docx"
            list-type="picture-card"
          >
            <el-icon><Plus /></el-icon>
            <template #tip>
              <div class="upload-tip">
                支持 jpg/png/gif/pdf/doc 格式，单个文件不超过 5MB，最多 3 个文件
              </div>
            </template>
          </el-upload>
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            :loading="submitting"
            @click="submitTicket"
          >
            提交工单
          </el-button>
          <el-button @click="goBack">
            取消
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 常见问题提示 -->
    <el-card
      class="tips-card"
      shadow="never"
    >
      <template #header>
        <span>
          <el-icon><InfoFilled /></el-icon>
          提交前请先查看
        </span>
      </template>

      <div class="tips-content">
        <p>在提交工单前，建议您先查看以下内容：</p>
        <ul>
          <li>
            <router-link to="/user/help">
              帮助中心
            </router-link>
            - 查看常见问题解答
          </li>
          <li>
            <router-link to="/user/announcements">
              公告中心
            </router-link>
            - 查看是否有相关维护通知
          </li>
        </ul>
        <p class="tips-note">
          工单通常会在 24 小时内得到回复，紧急问题会优先处理。
        </p>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Plus, InfoFilled } from '@element-plus/icons-vue'
import { usePortalTicketsStore } from '@/stores/portalTickets'

const router = useRouter()
const ticketsStore = usePortalTicketsStore()

// 表单引用
const formRef = ref(null)

// 状态
const submitting = ref(false)

// 表单数据
const form = reactive({
  subject: '',
  category: '',
  priority: 'normal',
  content: '',
  attachments: []
})

// 验证规则
const rules = {
  subject: [
    { required: true, message: '请输入工单主题', trigger: 'blur' },
    { min: 5, max: 100, message: '主题长度应为 5-100 个字符', trigger: 'blur' }
  ],
  category: [
    { required: true, message: '请选择问题类型', trigger: 'change' }
  ],
  content: [
    { required: true, message: '请输入问题描述', trigger: 'blur' },
    { min: 20, message: '问题描述至少 20 个字符', trigger: 'blur' }
  ]
}

// 方法
function goBack() {
  router.push('/user/tickets')
}

async function submitTicket() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
    submitting.value = true

    const ticket = await ticketsStore.createTicket({
      subject: form.subject,
      category: form.category,
      priority: form.priority,
      content: form.content
    })

    ElMessage.success('工单已提交')
    router.push(`/user/tickets/${ticket.id}`)
  } catch (error) {
    if (error !== false) {
      ElMessage.error(error.message || '提交失败')
    }
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.ticket-create-page {
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
}

.back-bar {
  margin-bottom: 20px;
}

.create-card {
  margin-bottom: 20px;
  border-radius: 8px;
}

.card-title {
  font-size: 18px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.upload-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 8px;
}

/* 提示卡片 */
.tips-card {
  border-radius: 8px;
}

.tips-card :deep(.el-card__header) {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tips-content {
  font-size: 14px;
  color: #606266;
}

.tips-content p {
  margin: 0 0 12px 0;
}

.tips-content ul {
  margin: 0 0 12px 0;
  padding-left: 20px;
}

.tips-content li {
  margin: 8px 0;
}

.tips-content a {
  color: #409eff;
  text-decoration: none;
}

.tips-content a:hover {
  text-decoration: underline;
}

.tips-note {
  color: #909399;
  font-size: 13px;
}

/* 响应式 */
@media (max-width: 768px) {
  .ticket-create-page {
    padding: 16px;
  }
}
</style>
