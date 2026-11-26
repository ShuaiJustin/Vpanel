<template>
  <div class="help-center-page">
    <!-- 搜索区域 -->
    <div class="search-section">
      <h1 class="search-title">
        帮助中心
      </h1>
      <p class="search-subtitle">
        搜索您需要的帮助文档
      </p>
      <div class="search-box">
        <el-input
          v-model="searchQuery"
          placeholder="搜索帮助文章..."
          size="large"
          :prefix-icon="Search"
          clearable
          @keyup.enter="handleSearch"
        />
        <el-button
          type="primary"
          size="large"
          @click="handleSearch"
        >
          搜索
        </el-button>
      </div>
    </div>

    <el-alert
      v-if="loadError"
      class="help-error"
      type="error"
      title="帮助内容加载失败"
      :closable="false"
      show-icon
    >
      <template #default>
        <p class="help-error__description">
          {{ loadError }}
        </p>
        <el-button
          type="danger"
          plain
          size="small"
          @click="loadInitialData"
        >
          重新加载
        </el-button>
      </template>
    </el-alert>

    <!-- 搜索结果 -->
    <div
      v-if="isSearching"
      class="search-results"
    >
      <div class="results-header">
        <h2>搜索结果</h2>
        <el-button
          link
          @click="clearSearch"
        >
          清除搜索
        </el-button>
      </div>

      <div
        v-if="loading"
        class="loading-state"
      >
        <el-icon class="loading-icon">
          <Loading />
        </el-icon>
        <p>搜索中...</p>
      </div>

      <el-empty
        v-else-if="searchResults.length === 0"
        description="未找到相关文章"
      />

      <div
        v-else
        class="articles-list"
      >
        <button
          v-for="article in searchResults" 
          :key="article.id"
          type="button"
          class="article-item"
          @click="viewArticle(article)"
        >
          <h3 class="article-title">
            {{ article.title }}
          </h3>
          <p class="article-summary">
            {{ article.summary }}
          </p>
          <div class="article-meta">
            <el-tag
              size="small"
              type="info"
            >
              {{ article.category }}
            </el-tag>
            <span class="view-count">
              <el-icon><View /></el-icon>
              {{ article.view_count }}
            </span>
          </div>
        </button>
      </div>
    </div>

    <!-- 分类浏览 -->
    <div
      v-else
      class="categories-section"
    >
      <!-- 精选文章 -->
      <div
        v-if="featuredArticles.length > 0"
        class="featured-section"
      >
        <h2 class="section-title">
          <el-icon><Star /></el-icon>
          精选文章
        </h2>
        <div class="featured-grid">
          <button
            v-for="article in featuredArticles" 
            :key="article.id"
            type="button"
            class="featured-card"
            @click="viewArticle(article)"
          >
            <h3 class="card-title">
              {{ article.title }}
            </h3>
            <p class="card-summary">
              {{ article.summary }}
            </p>
          </button>
        </div>
      </div>

      <!-- 分类列表 -->
      <h2 class="section-title">
        按分类浏览
      </h2>
      <div class="categories-grid">
        <button
          v-for="category in categories" 
          :key="category.key"
          type="button"
          class="category-card"
          :disabled="category.count === 0"
          :title="category.count === 0 ? `${category.name}暂无文章` : `浏览${category.name}`"
          @click="selectCategory(category.key)"
        >
          <div class="category-icon">
            <el-icon><component :is="category.icon" /></el-icon>
          </div>
          <div class="category-info">
            <h3 class="category-name">
              {{ category.name }}
            </h3>
            <span class="category-count">{{ category.count }} 篇文章</span>
          </div>
          <el-icon class="category-arrow">
            <ArrowRight />
          </el-icon>
        </button>
      </div>

      <!-- 分类文章列表 -->
      <div
        v-if="selectedCategory"
        class="category-articles"
      >
        <div class="category-header">
          <h2>{{ getCategoryName(selectedCategory) }}</h2>
          <el-button
            link
            @click="selectedCategory = null"
          >
            返回分类
          </el-button>
        </div>

        <div
          v-if="loading"
          class="loading-state"
        >
          <el-icon class="loading-icon">
            <Loading />
          </el-icon>
        </div>

        <el-empty
          v-else-if="categoryArticles.length === 0"
          :description="`${getCategoryName(selectedCategory)}暂无文章`"
        />

        <div
          v-else
          class="articles-list"
        >
          <button
            v-for="article in categoryArticles" 
            :key="article.id"
            type="button"
            class="article-item"
            @click="viewArticle(article)"
          >
            <h3 class="article-title">
              {{ article.title }}
            </h3>
            <p class="article-summary">
              {{ article.summary }}
            </p>
            <div class="article-meta">
              <span class="view-count">
                <el-icon><View /></el-icon>
                {{ article.view_count }}
              </span>
            </div>
          </button>
        </div>
      </div>

      <!-- 热门文章 -->
      <div
        v-if="popularArticles.length > 0"
        class="popular-section"
      >
        <h2 class="section-title">
          <el-icon><TrendCharts /></el-icon>
          热门文章
        </h2>
        <div class="popular-list">
          <button
            v-for="(article, index) in popularArticles" 
            :key="article.id"
            type="button"
            class="popular-item"
            @click="viewArticle(article)"
          >
            <span class="popular-rank">{{ index + 1 }}</span>
            <span class="popular-title">{{ article.title }}</span>
            <span class="popular-views">{{ article.view_count }} 次浏览</span>
          </button>
        </div>
      </div>
    </div>

    <!-- 联系支持 -->
    <el-card
      class="support-card"
      shadow="never"
    >
      <div class="support-content">
        <div class="support-info">
          <h3>没有找到答案？</h3>
          <p>如果帮助文档无法解决您的问题，请提交工单获取人工支持。</p>
        </div>
        <el-button
          type="primary"
          @click="createTicket"
        >
          <el-icon><ChatDotRound /></el-icon>
          提交工单
        </el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { 
  Search, Loading, View, Star, ArrowRight, TrendCharts, ChatDotRound,
  QuestionFilled, Setting, Connection, Document, CreditCard, Monitor
} from '@element-plus/icons-vue'
import { help as helpApi } from '@/api/modules/portal'

const router = useRouter()

// 状态
const loading = ref(false)
const searchQuery = ref('')
const isSearching = ref(false)
const selectedCategory = ref(null)
const loadError = ref('')

// 数据
const searchResults = ref([])
const featuredArticles = ref([])
const popularArticles = ref([])
const categoryArticles = ref([])

// 分类配置
const categories = ref([
  { key: 'getting-started', name: '快速入门', icon: QuestionFilled, count: 0 },
  { key: 'account', name: '账户相关', icon: Setting, count: 0 },
  { key: 'connection', name: '连接问题', icon: Connection, count: 0 },
  { key: 'subscription', name: '订阅管理', icon: Document, count: 0 },
  { key: 'payment', name: '支付问题', icon: CreditCard, count: 0 },
  { key: 'clients', name: '客户端使用', icon: Monitor, count: 0 }
])

// 方法
function getCategoryName(key) {
  const category = categories.value.find(c => c.key === key)
  return category ? category.name : key
}

async function handleSearch() {
  if (!searchQuery.value.trim()) {
    clearSearch()
    return
  }

  isSearching.value = true
  loading.value = true
  loadError.value = ''

  try {
    const response = await helpApi.searchArticles({ q: searchQuery.value }, { silent: true })
    searchResults.value = response.results || response.articles || []
  } catch (error) {
    console.error('Failed to search help articles:', error)
    loadError.value = '搜索服务暂时不可用，请稍后重试。'
  } finally {
    loading.value = false
  }
}

function clearSearch() {
  searchQuery.value = ''
  isSearching.value = false
  searchResults.value = []
}

async function selectCategory(key) {
  selectedCategory.value = key
  loading.value = true
  loadError.value = ''

  try {
    const response = await helpApi.getArticles({ category: key }, { silent: true })
    categoryArticles.value = response.articles || []
  } catch (error) {
    console.error('Failed to load help category:', error)
    loadError.value = '分类文章加载失败，请稍后重试。'
  } finally {
    loading.value = false
  }
}

function viewArticle(article) {
  router.push(`/user/help/${article.slug}`)
}

function createTicket() {
  router.push('/user/tickets/create')
}

async function loadInitialData() {
  loading.value = true
  loadError.value = ''
  try {
    const [featuredResult, popularResult, categoriesResult] = await Promise.allSettled([
      helpApi.getFeaturedArticles(4, { silent: true }),
      helpApi.getArticles({ limit: 5 }, { silent: true }),
      helpApi.getCategories({ silent: true })
    ])

    if (featuredResult.status === 'fulfilled') {
      featuredArticles.value = featuredResult.value.articles || []
    }
    if (popularResult.status === 'fulfilled') {
      popularArticles.value = popularResult.value.articles || []
    }
    if (categoriesResult.status === 'fulfilled' && categoriesResult.value.categories) {
      categories.value = categories.value.map(cat => ({
        ...cat,
        count: categoriesResult.value.categories[cat.key] || 0
      }))
    }

    const failures = [featuredResult, popularResult, categoriesResult]
      .filter(result => result.status === 'rejected')
    if (failures.length > 0) {
      loadError.value = failures.length === 3
        ? '帮助中心暂时不可用，请稍后重试。'
        : '部分帮助内容加载失败，您可以重试获取完整内容。'
    }
  } catch (error) {
    console.error('Failed to load help center data:', error)
    loadError.value = '帮助中心暂时不可用，请稍后重试。'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadInitialData()
})
</script>

<style scoped>
.help-center-page {
  padding: 20px;
  max-width: 1000px;
  margin: 0 auto;
  box-sizing: border-box;
}

/* 搜索区域 */
.search-section {
  text-align: center;
  padding: 40px 20px;
  background: linear-gradient(135deg, #409eff 0%, #66b1ff 100%);
  border-radius: 12px;
  margin-bottom: 32px;
  color: #fff;
}

.search-title {
  font-size: 28px;
  font-weight: 600;
  margin: 0 0 8px 0;
}

.search-subtitle {
  font-size: 14px;
  opacity: 0.9;
  margin: 0 0 24px 0;
}

.search-box {
  display: flex;
  gap: 12px;
  max-width: 600px;
  margin: 0 auto;
  align-items: stretch;
}

.search-box .el-input {
  flex: 1;
}

.help-error {
  margin-bottom: 24px;
}

.help-error__description {
  margin: 0 0 10px;
}

/* 搜索结果 */
.search-results {
  margin-bottom: 32px;
}

.results-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.results-header h2 {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
}

/* 加载状态 */
.loading-state {
  text-align: center;
  padding: 40px 0;
  color: var(--color-text-secondary);
}

.loading-icon {
  font-size: 32px;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* 文章列表 */
.articles-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.article-item {
  width: 100%;
  appearance: none;
  text-align: left;
  font: inherit;
  padding: 16px 20px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  cursor: pointer;
  transition: all 0.3s;
}

.article-item:hover,
.article-item:focus-visible {
  box-shadow: var(--shadow-md);
  transform: translateX(4px);
}

.article-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.article-summary {
  font-size: 14px;
  color: var(--color-text-regular);
  margin: 0 0 12px 0;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.article-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.view-count {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* 精选文章 */
.featured-section {
  margin-bottom: 32px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 16px 0;
}

.featured-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
  align-items: stretch;
}

.featured-card {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  padding: 20px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  cursor: pointer;
  transition: all 0.3s;
  width: 100%;
  appearance: none;
  text-align: left;
  font: inherit;
}

.featured-card:hover,
.featured-card:focus-visible {
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.card-summary {
  flex: 1;
  font-size: 13px;
  color: var(--color-text-secondary);
  margin: 0;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* 分类网格 */
.categories-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  margin-bottom: 32px;
  align-items: stretch;
}

.category-card {
  display: flex;
  align-items: center;
  min-height: 100%;
  padding: 20px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  cursor: pointer;
  transition: all 0.3s;
  width: 100%;
  appearance: none;
  text-align: left;
  font: inherit;
}

.category-card:hover:not(:disabled),
.category-card:focus-visible {
  box-shadow: var(--shadow-md);
  transform: translateX(4px);
}

.category-card:disabled {
  cursor: not-allowed;
  opacity: 0.58;
  box-shadow: none;
}

.category-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: rgba(64, 158, 255, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  color: #409eff;
  margin-right: 16px;
}

.category-info {
  flex: 1;
  min-width: 0;
}

.category-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 4px 0;
}

.category-count {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.category-arrow {
  color: var(--color-text-placeholder);
}

/* 分类文章 */
.category-articles {
  margin-bottom: 32px;
}

.category-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.category-header h2 {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
}

/* 热门文章 */
.popular-section {
  margin-bottom: 32px;
}

.popular-list {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  overflow: hidden;
}

.popular-item {
  display: flex;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border);
  cursor: pointer;
  transition: background 0.3s;
  width: 100%;
  appearance: none;
  text-align: left;
  font: inherit;
  background: var(--color-bg-card);
}

.popular-item:last-child {
  border-bottom: none;
}

.popular-item:hover,
.popular-item:focus-visible {
  background: var(--color-border-light);
}

.article-item:focus-visible,
.featured-card:focus-visible,
.category-card:focus-visible,
.popular-item:focus-visible {
  outline: 3px solid rgba(64, 158, 255, 0.35);
  outline-offset: 2px;
}

.popular-rank {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--color-border-light);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  margin-right: 12px;
}

.popular-item:nth-child(1) .popular-rank {
  background: #ffd700;
  color: #fff;
}

.popular-item:nth-child(2) .popular-rank {
  background: #c0c0c0;
  color: #fff;
}

.popular-item:nth-child(3) .popular-rank {
  background: #cd7f32;
  color: #fff;
}

.popular-title {
  flex: 1;
  font-size: 14px;
  color: var(--color-text-primary);
  min-width: 0;
}

.popular-views {
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* 联系支持 */
.support-card {
  border-radius: 16px;
}

.support-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.support-info h3 {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 4px 0;
}

.support-info p {
  font-size: 14px;
  color: var(--color-text-secondary);
  margin: 0;
}

/* 响应式 */
@media (max-width: 768px) {
  .help-center-page {
    padding: 0 0 96px;
  }

  .search-section {
    padding: 28px 16px;
    margin-bottom: 24px;
    border-radius: 16px;
  }

  .search-title {
    font-size: 24px;
  }

  .search-box {
    flex-direction: column;
  }

  .results-header,
  .category-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .featured-grid,
  .categories-grid {
    grid-template-columns: 1fr;
  }

  .article-item,
  .featured-card,
  .category-card,
  .popular-item {
    padding: 16px;
  }

  .category-card {
    align-items: flex-start;
  }

  .category-arrow {
    display: none;
  }

  .popular-item {
    flex-wrap: wrap;
    gap: 8px;
  }

  .popular-views {
    width: 100%;
    margin-left: 36px;
  }

  .support-content {
    flex-direction: column;
    text-align: center;
    gap: 16px;
  }

  .support-content :deep(.el-button) {
    width: 100%;
  }
}

@media (max-width: 480px) {
  .search-title {
    font-size: 22px;
  }

  .search-subtitle {
    margin-bottom: 18px;
  }

  .section-title {
    font-size: 17px;
  }
}
</style>
