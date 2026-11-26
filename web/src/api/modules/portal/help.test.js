import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/base', () => ({
  default: { get }
}))

import { getArticles, getCategories, getFeaturedArticles, searchArticles } from './help'

describe('portal help API', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('requests featured articles from the public featured endpoint', () => {
    getFeaturedArticles(4, { silent: true })

    expect(get).toHaveBeenCalledWith('/portal/help/featured', {
      silent: true,
      params: { limit: 4 }
    })
  })

  it('passes silent request options without losing query parameters', () => {
    getArticles({ category: 'connection' }, { silent: true })
    searchArticles({ q: '连接' }, { silent: true })
    getCategories({ silent: true })

    expect(get).toHaveBeenNthCalledWith(1, '/portal/help/articles', {
      silent: true,
      params: { category: 'connection' }
    })
    expect(get).toHaveBeenNthCalledWith(2, '/portal/help/search', {
      silent: true,
      params: { q: '连接' }
    })
    expect(get).toHaveBeenNthCalledWith(3, '/portal/help/categories', { silent: true })
  })
})
