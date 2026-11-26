<template>
  <div class="admin-tickets">
    <AdminStickyChrome>
      <h1>工单管理</h1>
    </AdminStickyChrome>

    <div class="admin-page-body">
      <el-card shadow="never">
        <div class="ticket-toolbar">
          <el-select v-model="status" aria-label="工单状态" @change="applyFilter">
            <el-option label="全部状态" value="" />
            <el-option
              v-for="(item, value) in ticketStatuses"
              :key="value" :label="item.label" :value="value"
            />
          </el-select>
          <el-button :loading="loading" @click="loadTickets">刷新</el-button>
        </div>
        <el-alert v-if="listError" :title="listError" type="error" :closable="false" />
        <el-table v-else v-loading="loading" :data="tickets" empty-text="暂无工单">
          <el-table-column prop="id" label="编号" width="75" />
          <el-table-column label="主题" min-width="160">
            <template #default="{ row }">
              <el-button class="ticket-subject-button" link type="primary" @click="openTicket(row.id)">
                {{ row.subject }}
              </el-button>
            </template>
          </el-table-column>
          <el-table-column prop="user_id" label="用户 ID" width="90" />
          <el-table-column label="状态" width="130">
            <template #default="{ row }">
              <el-tag :type="ticketStatuses[row.status]?.type || 'info'">
                {{ ticketStatuses[row.status]?.label || row.status }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="分类" width="105">
            <template #default="{ row }">{{ ticketCategories[row.category] || row.category }}</template>
          </el-table-column>
          <el-table-column label="优先级" width="85">
            <template #default="{ row }">{{ ticketPriorities[row.priority]?.label || row.priority }}</template>
          </el-table-column>
          <el-table-column label="最近更新" width="175">
            <template #default="{ row }">{{ formatDate(row.updated_at) }}</template>
          </el-table-column>
        </el-table>
        <el-pagination
          v-if="!listError && total > pageSize"
          v-model:current-page="page"
          class="ticket-pagination"
          :page-size="pageSize" :total="total" layout="prev, pager, next"
          @current-change="loadTickets"
        />
      </el-card>
    </div>

    <el-dialog v-model="detailOpen" title="工单详情" width="min(640px, 94vw)">
      <div v-if="loadingDetail" role="status">正在加载工单…</div>
      <el-alert v-else-if="detailError" :title="detailError" type="error" :closable="false">
        <el-button link type="primary" @click="loadDetail">重试</el-button>
      </el-alert>
      <template v-else-if="ticket">
        <h2 class="ticket-heading">#{{ ticket.id }} {{ ticket.subject }}</h2>
        <el-tag :type="ticketStatuses[ticket.status]?.type || 'info'">
          {{ ticketStatuses[ticket.status]?.label || ticket.status }}
        </el-tag>
        <div v-for="message in ticket.messages" :key="message.id" class="ticket-message">
          <div class="ticket-message-meta">
            <strong>{{ message.is_admin ? '客服' : '用户' }}</strong>
            <time :datetime="message.created_at">{{ formatDate(message.created_at) }}</time>
          </div>
          <p>{{ message.content }}</p>
        </div>
        <el-form v-if="ticket.status !== 'closed'" label-position="top" @submit.prevent="sendReply">
          <el-form-item label="回复内容">
            <el-input
              v-model="reply" type="textarea" :rows="4" :maxlength="10000"
              :disabled="saving" placeholder="输入回复内容"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" native-type="submit" :loading="saving" :disabled="!reply.trim()">
              发送回复
            </el-button>
            <el-button :disabled="saving" @click="closeTicket">关闭工单</el-button>
          </el-form-item>
        </el-form>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api/base'
import AdminStickyChrome from '@/components/AdminStickyChrome.vue'
import { extractErrorMessage } from '@/utils/entitlement'
import { ticketCategories, ticketPriorities, ticketStatuses } from '@/utils/tickets'

const apiPath = '/admin/tickets'
const pageSize = 20
const status = ref('')
const page = ref(1)
const total = ref(0)
const tickets = ref([])
const loading = ref(false)
const listError = ref('')
const detailOpen = ref(false)
const ticket = ref(null)
const activeTicketId = ref(null)
const loadingDetail = ref(false)
const detailError = ref('')
const reply = ref('')
const saving = ref(false)
let listRequest = 0
let detailRequest = 0

function formatDate(value) {
  return value ? new Date(value).toLocaleString('zh-CN') : '—'
}

async function loadTickets() {
  const request = ++listRequest
  loading.value = true
  listError.value = ''
  try {
    const result = await api.get(apiPath, {
      params: { limit: pageSize, offset: (page.value - 1) * pageSize, status: status.value || undefined },
      silent: true
    })
    if (request !== listRequest) return
    tickets.value = result.tickets
    total.value = result.total
    const lastPage = Math.max(1, Math.ceil(total.value / pageSize))
    if (page.value > lastPage) {
      page.value = lastPage
      await loadTickets()
    }
  } catch (error) {
    if (request === listRequest) listError.value = extractErrorMessage(error) || '加载工单失败'
  } finally {
    if (request === listRequest) loading.value = false
  }
}

function applyFilter() {
  page.value = 1
  loadTickets()
}

function openTicket(id) {
  activeTicketId.value = id
  reply.value = ''
  detailOpen.value = true
  loadDetail()
}

async function loadDetail() {
  const request = ++detailRequest
  loadingDetail.value = true
  detailError.value = ''
  try {
    const result = await api.get(`${apiPath}/${activeTicketId.value}`, { silent: true })
    if (request === detailRequest) ticket.value = result
  } catch (error) {
    if (request === detailRequest) detailError.value = extractErrorMessage(error) || '加载工单详情失败'
  } finally {
    if (request === detailRequest) loadingDetail.value = false
  }
}

async function sendReply() {
  const content = reply.value.trim()
  if (!content || saving.value) return
  await updateTicket('reply', { content }, '回复已发送')
}

async function closeTicket() {
  const id = activeTicketId.value
  try {
    await ElMessageBox.confirm('关闭后，用户可以重新打开工单。', '关闭工单', { type: 'warning' })
  } catch {
    return
  }
  if (activeTicketId.value === id && !saving.value) {
    await updateTicket('close', undefined, '工单已关闭')
  }
}

async function updateTicket(action, body, message) {
  const id = activeTicketId.value
  saving.value = true
  try {
    await api.post(`${apiPath}/${id}/${action}`, body, { silent: true })
    ElMessage.success(message)
    if (activeTicketId.value === id) {
      reply.value = ''
      await loadDetail()
    }
    await loadTickets()
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '操作失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadTickets)
</script>

<style scoped>
.admin-tickets {
  padding: 20px;
}

.ticket-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.ticket-toolbar .el-select {
  width: 180px;
}

.ticket-subject-button {
  max-width: 100%;
  height: auto;
  white-space: normal;
  text-align: left;
}

.ticket-pagination {
  justify-content: center;
  margin-block-start: 16px;
}

.ticket-heading {
  margin-top: 0;
  overflow-wrap: anywhere;
}

.ticket-message {
  margin-block: 16px;
  padding: 12px 16px;
  background: var(--el-fill-color-light);
  border-radius: var(--el-border-radius-base);
}

.ticket-message-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 4px 12px;
}

.ticket-message-meta time {
  color: var(--el-text-color-secondary);
}

.ticket-message p {
  margin-block: 8px 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
