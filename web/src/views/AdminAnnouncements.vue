<template>
  <div class="admin-announcements">
    <AdminStickyChrome>
      <div class="page-header">
        <div class="page-heading">
          <h1>公告管理</h1>
          <p class="page-subtitle">发布、置顶和维护用户门户公告</p>
        </div>
        <el-button type="primary" @click="openCreate">
          <el-icon class="el-icon--left"><Plus /></el-icon>
          发布公告
        </el-button>
      </div>
    </AdminStickyChrome>

    <div class="admin-page-body">
      <el-card shadow="never">
        <div class="toolbar-row">
          <div class="toolbar-filters">
            <el-select v-model="filterCategory" placeholder="全部分类" clearable @change="applyFilter">
              <el-option v-for="(label, value) in categoryLabels" :key="value" :label="label" :value="value" />
            </el-select>
            <el-select v-model="filterPublished" placeholder="发布状态" clearable @change="applyFilter">
              <el-option label="已发布" value="true" />
              <el-option label="草稿" value="false" />
            </el-select>
          </div>
          <el-button :loading="loading" @click="loadAnnouncements">刷新</el-button>
        </div>

        <el-table v-loading="loading" :data="announcements" border stripe empty-text="暂无公告">
          <el-table-column prop="id" label="ID" width="75" />
          <el-table-column label="标题" min-width="220">
            <template #default="{ row }">
              <span>{{ row.title || row.Title }}</span>
              <el-tag v-if="row.is_pinned ?? row.IsPinned" size="small" type="warning" style="margin-left: 6px">置顶</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="分类" width="100">
            <template #default="{ row }">
              {{ categoryLabels[row.category || row.Category] || (row.category || row.Category) }}
            </template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="(row.is_published ?? row.IsPublished) ? 'success' : 'info'" size="small">
                {{ (row.is_published ?? row.IsPublished) ? '已发布' : '草稿' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="更新时间" width="175">
            <template #default="{ row }">
              {{ formatDate(row.updated_at || row.UpdatedAt) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150" align="right">
            <template #default="{ row }">
              <el-button size="small" @click="openEdit(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="removeItem(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-pagination
          v-if="total > pageSize"
          v-model:current-page="page"
          class="pagination-row"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="loadAnnouncements"
        />
      </el-card>
    </div>

    <el-dialog v-model="dialogOpen" :title="editingId ? '编辑公告' : '发布公告'" width="min(640px, 94vw)">
      <el-form :model="form" label-position="top">
        <el-form-item label="标题" required>
          <el-input v-model="form.title" maxlength="200" placeholder="输入公告标题" />
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="form.category" style="width: 100%">
            <el-option v-for="(label, value) in categoryLabels" :key="value" :label="label" :value="value" />
          </el-select>
        </el-form-item>
        <el-form-item label="内容（支持纯文本或基础 HTML）" required>
          <el-input v-model="form.content" type="textarea" :rows="6" placeholder="输入公告正文" />
        </el-form-item>
        <div class="switch-row">
          <el-checkbox v-model="form.is_pinned">置顶显示</el-checkbox>
          <el-checkbox v-model="form.is_published">立即发布</el-checkbox>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="dialogOpen = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveItem">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '@/api/base'
import AdminStickyChrome from '@/components/AdminStickyChrome.vue'
import { extractErrorMessage } from '@/utils/entitlement'

const categoryLabels = {
  general: '公告',
  maintenance: '维护',
  update: '更新',
  promotion: '活动'
}

const pageSize = 20
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const saving = ref(false)
const announcements = ref([])
const filterCategory = ref('')
const filterPublished = ref('')
const dialogOpen = ref(false)
const editingId = ref(null)
const form = reactive({
  title: '',
  category: 'general',
  content: '',
  is_pinned: false,
  is_published: true
})

const formatDate = (value) => (value ? new Date(value).toLocaleString('zh-CN') : '—')

async function loadAnnouncements() {
  loading.value = true
  try {
    const res = await api.get('/admin/announcements', {
      params: {
        limit: pageSize,
        offset: (page.value - 1) * pageSize,
        category: filterCategory.value || undefined,
        is_published: filterPublished.value || undefined
      }
    })
    announcements.value = res.announcements || []
    total.value = res.total || 0
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载公告失败')
  } finally {
    loading.value = false
  }
}

function applyFilter() {
  page.value = 1
  loadAnnouncements()
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { title: '', category: 'general', content: '', is_pinned: false, is_published: true })
  dialogOpen.value = true
}

function openEdit(row) {
  editingId.value = row.id ?? row.ID
  Object.assign(form, {
    title: row.title ?? row.Title ?? '',
    category: row.category ?? row.Category ?? 'general',
    content: row.content ?? row.Content ?? '',
    is_pinned: Boolean(row.is_pinned ?? row.IsPinned),
    is_published: Boolean(row.is_published ?? row.IsPublished)
  })
  dialogOpen.value = true
}

async function saveItem() {
  if (!form.title.trim() || !form.content.trim()) {
    ElMessage.warning('请填写公告标题和内容')
    return
  }
  saving.value = true
  try {
    const payload = {
      title: form.title.trim(),
      category: form.category,
      content: form.content.trim(),
      is_pinned: form.is_pinned,
      is_published: form.is_published
    }
    if (editingId.value) {
      await api.put(`/admin/announcements/${editingId.value}`, payload)
      ElMessage.success('公告已更新')
    } else {
      await api.post('/admin/announcements', payload)
      ElMessage.success('公告已发布')
    }
    dialogOpen.value = false
    await loadAnnouncements()
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '保存公告失败')
  } finally {
    saving.value = false
  }
}

async function removeItem(row) {
  const id = row.id ?? row.ID
  try {
    await ElMessageBox.confirm('确定删除这条公告吗？', '删除公告', { type: 'warning' })
    await api.delete(`/admin/announcements/${id}`)
    ElMessage.success('公告已删除')
    await loadAnnouncements()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error(extractErrorMessage(error) || '删除公告失败')
    }
  }
}

onMounted(loadAnnouncements)
</script>

<style scoped>
.admin-announcements { padding: 20px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.page-heading h1 { margin: 0; font-size: 24px; }
.page-subtitle { margin: 6px 0 0; color: var(--el-text-color-secondary); }
.toolbar-row { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
.toolbar-filters { display: flex; gap: 12px; flex-wrap: wrap; }
.switch-row { display: flex; gap: 20px; }
.pagination-row { margin-top: 16px; justify-content: flex-end; }
</style>
