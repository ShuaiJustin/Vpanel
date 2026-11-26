<!-- eslint-disable vue/no-v-html -->
<template>
  <div class="help-article-page">
    <!-- 返回按钮 -->
    <div class="back-bar">
      <el-button
        link
        @click="goBack"
      >
        <el-icon><ArrowLeft /></el-icon>
        返回帮助中心
      </el-button>
    </div>

    <!-- 加载状态 -->
    <div
      v-if="loading"
      class="loading-state"
    >
      <el-icon class="loading-icon">
        <Loading />
      </el-icon>
      <p>加载文章内容...</p>
    </div>

    <!-- 文章不存在 -->
    <el-empty
      v-else-if="!article"
      description="文章不存在或已删除"
    />

    <!-- 文章内容 -->
    <template v-else>
      <el-card
        class="article-card"
        shadow="never"
      >
        <div class="article-header">
          <div class="header-meta">
            <el-tag
              size="small"
              type="info"
            >
              {{ article.category }}
            </el-tag>
            <span class="view-count">
              <el-icon><View /></el-icon>
              {{ article.view_count }} 次浏览
            </span>
          </div>
          <h1 class="article-title">
            {{ article.title }}
          </h1>
          <div class="article-info">
            <span class="info-item">
              <el-icon><Calendar /></el-icon>
              更新于 {{ formatDate(article.updated_at) }}
            </span>
          </div>
        </div>

        <el-divider />

        <!-- eslint-disable-next-line vue/no-v-html -->
        <div
          class="article-content"
          v-html="safeArticleContent"
        />

        <!-- 标签 -->
        <div
          v-if="article.tags && article.tags.length"
          class="article-tags"
        >
          <span class="tags-label">相关标签：</span>
          <el-tag 
            v-for="tag in article.tags" 
            :key="tag"
            size="small"
            type="info"
            class="tag-item"
          >
            {{ tag }}
          </el-tag>
        </div>

        <el-divider />

        <!-- 反馈 -->
        <div class="article-feedback">
          <span class="feedback-label">这篇文章对您有帮助吗？</span>
          <div class="feedback-buttons">
            <el-button 
              :type="feedback === 'helpful' ? 'success' : 'default'"
              @click="submitFeedback('helpful')"
            >
              <el-icon><CircleCheck /></el-icon>
              有帮助
            </el-button>
            <el-button 
              :type="feedback === 'not_helpful' ? 'danger' : 'default'"
              @click="submitFeedback('not_helpful')"
            >
              <el-icon><CircleClose /></el-icon>
              没帮助
            </el-button>
          </div>
        </div>
      </el-card>

      <!-- 相关文章 -->
      <el-card
        v-if="relatedArticles.length > 0"
        class="related-card"
        shadow="never"
      >
        <template #header>
          <span>相关文章</span>
        </template>

        <div class="related-list">
          <div 
            v-for="item in relatedArticles" 
            :key="item.id"
            class="related-item"
            @click="viewArticle(item)"
          >
            <span class="related-title">{{ item.title }}</span>
            <el-icon><ArrowRight /></el-icon>
          </div>
        </div>
      </el-card>
    </template>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { 
  ArrowLeft, ArrowRight, Loading, View, Calendar, 
  CircleCheck, CircleClose 
} from '@element-plus/icons-vue'
import { help as helpApi } from '@/api/modules/portal'
import { sanitizeHtml } from '@/utils/htmlSanitizer'

const router = useRouter()
const route = useRoute()

// 状态
const loading = ref(false)
const article = ref(null)
const relatedArticles = ref([])
const feedback = ref(null)
const safeArticleContent = computed(() => sanitizeHtml(article.value?.content || ''))

// 方法
function formatDate(dateStr) {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  })
}

function goBack() {
  router.push('/user/help')
}

function viewArticle(item) {
  router.push(`/user/help/${item.slug}`)
}

async function submitFeedback(type) {
  if (feedback.value === type) return

  try {
    await helpApi.rateArticle(article.value.slug, { helpful: type === 'helpful' })
    feedback.value = type
    ElMessage.success('感谢您的反馈！')
  } catch (error) {
    ElMessage.error('提交反馈失败')
  }
}

async function loadArticle() {
  const slug = route.params.slug
  if (!slug) {
    router.push('/user/help')
    return
  }

  loading.value = true
  feedback.value = null

  try {
    article.value = await helpApi.getArticle(slug)

    // 加载相关文章
    if (article.value.category) {
      const response = await helpApi.getArticles({ 
        category: article.value.category,
        limit: 5,
        exclude: article.value.id
      })
      relatedArticles.value = (response.articles || []).filter(a => a.id !== article.value.id)
    }
  } catch (error) {
    ElMessage.error('加载文章失败')
    article.value = null
  } finally {
    loading.value = false
  }
}

// 监听路由变化
watch(() => route.params.slug, () => {
  if (route.params.slug) {
    loadArticle()
  }
})

onMounted(() => {
  loadArticle()
})
</script>

<style scoped>
.help-article-page {
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
  box-sizing: border-box;
}

.back-bar {
  margin-bottom: 20px;
}

/* 加载状态 */
.loading-state {
  text-align: center;
  padding: 60px 0;
  color: #909399;
}

.loading-icon {
  font-size: 32px;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* 文章卡片 */
.article-card {
  margin-bottom: 20px;
  border-radius: 12px;
}

.article-header {
  margin-bottom: 16px;
}

.header-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.view-count {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #909399;
}

.article-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 12px 0;
  line-height: 1.4;
}

.article-info {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}

.info-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #909399;
}

/* 文章内容 */
.article-content {
  font-size: 15px;
  color: #606266;
  line-height: 1.8;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.article-content :deep(h1),
.article-content :deep(h2),
.article-content :deep(h3) {
  color: var(--color-text-primary);
  margin: 24px 0 12px 0;
}

.article-content :deep(h2) {
  font-size: 20px;
}

.article-content :deep(h3) {
  font-size: 17px;
}

.article-content :deep(p) {
  margin: 12px 0;
}

.article-content :deep(ul),
.article-content :deep(ol) {
  padding-left: 24px;
  margin: 12px 0;
}

.article-content :deep(li) {
  margin: 8px 0;
}

.article-content :deep(a) {
  color: #409eff;
  text-decoration: none;
}

.article-content :deep(a:hover) {
  text-decoration: underline;
}

.article-content :deep(code) {
  background: var(--color-bg-page);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 14px;
}

.article-content :deep(pre) {
  background: var(--color-bg-page);
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
}

.article-content :deep(pre code) {
  background: none;
  padding: 0;
}

.article-content :deep(blockquote) {
  border-left: 4px solid #409eff;
  padding-left: 16px;
  margin: 16px 0;
  color: #909399;
}

.article-content :deep(img) {
  max-width: 100%;
  border-radius: 8px;
  margin: 16px 0;
}

.article-content :deep(table) {
  display: block;
  width: 100%;
  border-collapse: collapse;
  margin: 16px 0;
  overflow-x: auto;
}

.article-content :deep(th),
.article-content :deep(td) {
  border: 1px solid #ebeef5;
  padding: 12px;
  text-align: left;
}

.article-content :deep(th) {
  background: var(--color-bg-page);
  font-weight: 600;
}

/* 标签 */
.article-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 20px;
}

.tags-label {
  font-size: 14px;
  color: #909399;
}

.tag-item {
  cursor: pointer;
}

/* 反馈 */
.article-feedback {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 20px 0;
}

.feedback-label {
  font-size: 14px;
  color: #606266;
}

.feedback-buttons {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

/* 相关文章 */
.related-card {
  border-radius: 8px;
}

.related-list {
  display: flex;
  flex-direction: column;
}

.related-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #ebeef5;
  cursor: pointer;
  transition: color 0.3s;
}

.related-item:last-child {
  border-bottom: none;
}

.related-item:hover {
  color: #409eff;
}

.related-title {
  font-size: 14px;
  flex: 1;
  min-width: 0;
}

/* 响应式 */
@media (max-width: 768px) {
  .help-article-page {
    padding: 0;
  }

  .article-title {
    font-size: 20px;
  }

  .article-feedback {
    flex-direction: column;
    align-items: stretch;
  }

  .feedback-buttons {
    width: 100%;
    flex-direction: column;
  }

  .feedback-buttons :deep(.el-button) {
    width: 100%;
    margin-left: 0;
  }

  .related-item {
    gap: 12px;
    align-items: flex-start;
  }

  .article-card :deep(.el-card__body),
  .related-card :deep(.el-card__body) {
    padding: 18px;
  }
}
</style>
