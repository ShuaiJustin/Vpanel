<template>
  <div class="admin-help-articles">
    <AdminStickyChrome>
      <div class="page-header">
        <div class="page-heading">
          <h1>帮助中心管理</h1>
          <p class="page-subtitle">维护用户门户的客户端使用指南与常见问题</p>
        </div>
        <el-button type="primary" @click="openCreate">
          <el-icon class="el-icon--left"><Plus /></el-icon>
          新建文章
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
          <el-button :loading="loading" @click="loadArticles">刷新</el-button>
        </div>

        <el-table v-loading="loading" :data="articles" border stripe empty-text="暂无帮助文章">
          <el-table-column prop="id" label="ID" width="75" />
          <el-table-column label="标题" min-width="220">
            <template #default="{ row }">
              <span>{{ row.title || row.Title }}</span>
              <el-tag v-if="row.is_featured ?? row.IsFeatured" size="small" type="primary" style="margin-left: 6px">推荐</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="Slug" min-width="150">
            <template #default="{ row }">{{ row.slug || row.Slug }}</template>
          </el-table-column>
          <el-table-column label="分类" width="115">
            <template #default="{ row }">
              {{ categoryLabels[row.category || row.Category] || (row.category || row.Category) }}
            </template>
          </el-table-column>
          <el-table-column label="状态" width="95">
            <template #default="{ row }">
              <el-tag :type="(row.is_published ?? row.IsPublished) ? 'success' : 'info'" size="small">
                {{ (row.is_published ?? row.IsPublished) ? '已发布' : '草稿' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="浏览 / 有帮助" width="120">
            <template #default="{ row }">
              {{ row.view_count ?? row.ViewCount ?? 0 }} / {{ row.helpful_count ?? row.HelpfulCount ?? 0 }}
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
          @current-change="loadArticles"
        />
      </el-card>
    </div>

    <el-dialog v-model="dialogOpen" :title="editingId ? '编辑文章' : '新建文章'" width="min(680px, 94vw)">
      <el-form :model="form" label-position="top">
        <el-form-item label="标题" required>
          <el-input v-model="form.title" maxlength="200" placeholder="例如：Clash 客户端快速导入教程" />
        </el-form-item>
        <div class="form-row-2">
          <el-form-item label="URL 标识 (Slug)" required>
            <el-input v-model="form.slug" maxlength="120" placeholder="例如：clash-quick-start" />
          </el-form-item>
          <el-form-item label="分类">
            <el-select v-model="form.category" style="width: 100%">
              <el-option v-for="(label, value) in categoryLabels" :key="value" :label="label" :value="value" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="标签（逗号分隔）">
          <el-input v-model="form.tags" placeholder="例如：clash,windows,订阅" />
        </el-form-item>
        <el-form-item label="正文内容" required>
          <el-input v-model="form.content" type="textarea" :rows="8" placeholder="输入帮助文档内容" />
        </el-form-item>
        <div class="switch-row">
          <el-checkbox v-model="form.is_featured">设为推荐文章</el-checkbox>
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
  'getting-started': '快速入门',
  'client-setup': '客户端设置',
  troubleshooting: '故障排除',
  faq: '常见问题'
}

const pageSize = 20
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const saving = ref(false)
const articles = ref([])
const filterCategory = ref('')
const filterPublished = ref('')
const dialogOpen = ref(false)
const editingId = ref(null)
const form = reactive({
  title: '',
  slug: '',
  category: 'getting-started',
  tags: '',
  content: '',
  is_featured: false,
  is_published: true
})

async function loadArticles() {
  loading.value = true
  try {
    const res = await api.get('/admin/help-articles', {
      params: {
        limit: pageSize,
        offset: (page.value - 1) * pageSize,
        category: filterCategory.value || undefined,
        is_published: filterPublished.value || undefined
      }
    })
    articles.value = res.articles || []
    total.value = res.total || 0
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '加载帮助文章失败')
  } finally {
    loading.value = false
  }
}

function applyFilter() {
  page.value = 1
  loadArticles()
}

function openCreate() {
  editingId.value = null
  Object.assign(form, {
    title: '',
    slug: '',
    category: 'getting-started',
    tags: '',
    content: '',
    is_featured: false,
    is_published: true
  })
  dialogOpen.value = true
}

function openEdit(row) {
  editingId.value = row.id ?? row.ID
  Object.assign(form, {
    title: row.title ?? row.Title ?? '',
    slug: row.slug ?? row.Slug ?? '',
    category: row.category ?? row.Category ?? 'getting-started',
    tags: row.tags ?? row.Tags ?? '',
    content: row.content ?? row.Content ?? '',
    is_featured: Boolean(row.is_featured ?? row.IsFeatured),
    is_published: Boolean(row.is_published ?? row.IsPublished)
  })
  dialogOpen.value = true
}

async function saveItem() {
  if (!form.title.trim() || !form.slug.trim() || !form.content.trim()) {
    ElMessage.warning('请填写标题、Slug 和正文内容')
    return
  }
  saving.value = true
  try {
    const payload = {
      title: form.title.trim(),
      slug: form.slug.trim(),
      category: form.category,
      tags: form.tags.trim(),
      content: form.content.trim(),
      is_featured: form.is_featured,
      is_published: form.is_published
    }
    if (editingId.value) {
      await api.put(`/admin/help-articles/${editingId.value}`, payload)
      ElMessage.success('帮助文章已更新')
    } else {
      await api.post('/admin/help-articles', payload)
      ElMessage.success('帮助文章已创建')
    }
    dialogOpen.value = false
    await loadArticles()
  } catch (error) {
    ElMessage.error(extractErrorMessage(error) || '保存帮助文章失败')
  } finally {
    saving.value = false
  }
}

async function removeItem(row) {
  const id = row.id ?? row.ID
  try {
    await ElMessageBox.confirm('确定删除这篇文章吗？', '删除文章', { type: 'warning' })
    await api.delete(`/admin/help-articles/${id}`)
    ElMessage.success('文章已删除')
    await loadArticles()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error(extractErrorMessage(error) || '删除文章失败')
    }
  }
}

onMounted(loadArticles)
</script>

<style scoped>
.admin-help-articles { padding: 20px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.page-heading h1 { margin: 0; font-size: 24px; }
.page-subtitle { margin: 6px 0 0; color: var(--el-text-color-secondary); }
.toolbar-row { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
.toolbar-filters { display: flex; gap: 12px; flex-wrap: wrap; }
.form-row-2 { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.switch-row { display: flex; gap: 20px; }
.pagination-row { margin-top: 16px; justify-content: flex-end; }
@media (max-width: 768px) { .form-row-2 { grid-template-columns: 1fr; } }
</style>
