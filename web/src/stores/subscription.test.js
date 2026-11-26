import { beforeEach, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { subscriptionApi } from '@/api/index'
import { useSubscriptionStore } from './subscription'

vi.mock('@/api/index', () => ({
  subscriptionApi: { getLink: vi.fn(), getInfo: vi.fn(), regenerate: vi.fn() }
}))

beforeEach(() => {
  setActivePinia(createPinia())
  vi.resetAllMocks()
})

it('keeps the regenerated link when an earlier read finishes late', async () => {
  let finishRead
  subscriptionApi.getLink.mockReturnValueOnce(new Promise(resolve => { finishRead = resolve }))
  subscriptionApi.regenerate.mockResolvedValue({ link: 'https://example.test/new' })
  const store = useSubscriptionStore()
  const oldRead = store.fetchLink()
  const reset = store.regenerate()
  const readDuringReset = store.fetchLink()
  expect(await readDuringReset).toEqual(await reset)
  finishRead({ link: 'https://example.test/old' })
  expect(await oldRead).toBeUndefined()
  expect(store.link).toBe('https://example.test/new')
})

it('clears forbidden links and ignores reads from a cleared session', async () => {
  const store = useSubscriptionStore()
  subscriptionApi.getLink.mockResolvedValueOnce({ link: 'https://example.test/old' })
  await store.fetchLink()
  subscriptionApi.getLink.mockRejectedValueOnce({ status: 403, message: '套餐已到期' })
  await expect(store.fetchLink()).rejects.toMatchObject({ status: 403 })
  expect(store.link).toBe('')

  let finishRead
  subscriptionApi.getLink.mockReturnValueOnce(new Promise(resolve => { finishRead = resolve }))
  const pending = store.fetchLink()
  store.clearSubscription()
  finishRead({ link: 'https://example.test/old' })
  expect(await pending).toBeUndefined()
  expect(store.link).toBe('')
})
