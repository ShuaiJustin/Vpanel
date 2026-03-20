/**
 * 用户前台帮助中心 API 模块
 */
import api from '@/api/base'

const BASE_URL = '/portal/help'

/**
 * 获取帮助文章列表
 * @param {Object} [params] - 查询参数
 * @param {string} [params.category] - 分类筛选
 * @param {boolean} [params.featured] - 是否只获取精选文章
 * @param {number} [params.limit] - 每页数量
 * @param {number} [params.offset] - 偏移量
 * @returns {Promise}
 */
export function getArticles(params = {}) {
  return api.get(`${BASE_URL}/articles`, { params })
}

/**
 * 获取文章详情
 * @param {string} slug - 文章 slug
 * @returns {Promise}
 */
export function getArticle(slug) {
  return api.get(`${BASE_URL}/articles/${slug}`)
}

/**
 * 搜索文章
 * @param {Object} params - 搜索参数
 * @param {string} params.q - 搜索关键词
 * @param {string} [params.category] - 分类筛选
 * @param {number} [params.limit] - 每页数量
 * @returns {Promise}
 */
export function searchArticles(params) {
  return api.get(`${BASE_URL}/search`, { params })
}

/**
 * 获取文章分类列表
 * @returns {Promise}
 */
export function getCategories() {
  return api.get(`${BASE_URL}/categories`)
}

/**
 * 评价文章是否有帮助
 * @param {string} slug - 文章 slug
 * @param {Object} data - 评价数据
 * @param {boolean} data.helpful - 是否有帮助
 * @returns {Promise}
 */
export function rateArticle(slug, data) {
  return api.post(`${BASE_URL}/articles/${slug}/helpful`, data)
}

export default {
  getArticles,
  getArticle,
  searchArticles,
  getCategories,
  rateArticle
}
