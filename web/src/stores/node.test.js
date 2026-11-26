import { beforeEach, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { nodesApi } from '@/api'
import { useNodeStore } from './node'

vi.mock('@/api', () => ({
  nodesApi: { list: vi.fn(), get: vi.fn(), update: vi.fn(), delete: vi.fn(), rotateToken: vi.fn(), revokeToken: vi.fn() }
}))

beforeEach(() => {
  vi.resetAllMocks()
  setActivePinia(createPinia())
})

it('keeps the newest filter results when an older response arrives later', async () => {
  let resolveOld
  nodesApi.list.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    .mockResolvedValueOnce({ nodes: [], total: 0 })
  const store = useNodeStore()
  const oldRequest = store.fetchNodes({ status: 'online' })
  await store.fetchNodes({ status: 'offline' })
  resolveOld({ nodes: [{ id: 1 }], total: 1 })
  await oldRequest
  expect(store.nodes).toEqual([])
  expect(store.total).toBe(0)
  expect(store.loading).toBe(false)
})

it('keeps the selected node when a previous node request completes late', async () => {
  let resolveOld
  nodesApi.get.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    .mockResolvedValueOnce({ id: 29 })
  const store = useNodeStore()
  const oldRequest = store.fetchNode(28)
  await store.fetchNode(29)
  resolveOld({ id: 28 })
  await oldRequest
  expect(store.currentNode.id).toBe(29)
})

it('updates token availability for the matching node without storing the generated secret', async () => {
  const store = useNodeStore()
  store.nodes = [{ id: 28, has_token: false }, { id: 29, has_token: true }]
  store.currentNode = { id: 28, has_token: false }
  nodesApi.rotateToken.mockResolvedValue({ token: 'only-for-the-dialog' })
  nodesApi.revokeToken.mockResolvedValue()

  await store.rotateToken(28)
  expect(store.nodes[0]).toEqual({ id: 28, has_token: true })
  expect(store.currentNode).toEqual({ id: 28, has_token: true })
  await store.revokeToken(28)
  expect(store.nodes).toEqual([{ id: 28, has_token: false }, { id: 29, has_token: true }])
  expect(store.currentNode).toEqual({ id: 28, has_token: false })
})

it('keeps a confirmed revocation when earlier list and detail reads return late', async () => {
  let resolveList
  let resolveDetail
  nodesApi.list.mockReturnValueOnce(new Promise(resolve => { resolveList = resolve }))
  nodesApi.get.mockReturnValueOnce(new Promise(resolve => { resolveDetail = resolve }))
  nodesApi.revokeToken.mockResolvedValue()
  const store = useNodeStore()

  const list = store.fetchNodes()
  const detail = store.fetchNode(28)
  await store.revokeToken(28)
  resolveList({ nodes: [{ id: 28, has_token: true }, { id: 29, has_token: true }] })
  resolveDetail({ id: 28, has_token: true })
  await Promise.all([list, detail])

  expect(store.nodes).toEqual([{ id: 28, has_token: false }, { id: 29, has_token: true }])
  expect(store.currentNode.has_token).toBe(false)

  nodesApi.get.mockResolvedValueOnce({ id: 28, has_token: true })
  await store.fetchNode(28)
  expect(store.currentNode.has_token).toBe(true)
})

it.each([28, '28'])('applies the edit for ID %p and preserves a revocation during the edit', async (id) => {
  const store = useNodeStore()
  store.nodes = [{ id: 28, has_token: true }, { id: 29, has_token: true }]
  store.currentNode = { id: 28, has_token: true }
  let finishEdit
  nodesApi.update.mockReturnValueOnce(new Promise(resolve => { finishEdit = resolve }))
  nodesApi.revokeToken.mockResolvedValueOnce()

  const edit = store.updateNode(id, { name: 'renamed' })
  nodesApi.list.mockResolvedValueOnce({ nodes: [{ id: 29, has_token: true }, { id: 28, has_token: true }] })
  await store.fetchNodes()
  await store.revokeToken(28)
  finishEdit({ id: 28, name: 'renamed', has_token: true })
  await edit

  expect(store.nodes).toEqual([{ id: 29, has_token: true }, { id: 28, name: 'renamed', has_token: false }])
  expect(store.currentNode).toEqual({ id: 28, name: 'renamed', has_token: false })
})

it('leaves the current page intact if an earlier deletion fails', async () => {
  const store = useNodeStore()
  store.nodes = [{ id: 28 }]
  store.total = 1
  let rejectDelete
  nodesApi.delete.mockReturnValueOnce(new Promise((_, reject) => { rejectDelete = reject }))

  const deletion = expect(store.deleteNode(28)).rejects.toThrow('delete failed')
  nodesApi.list.mockResolvedValueOnce({ nodes: [{ id: 29 }], total: 1 })
  await store.fetchNodes()
  rejectDelete(new Error('delete failed'))
  await deletion

  expect(store.nodes).toEqual([{ id: 29 }])
  expect(store.total).toBe(1)
})
