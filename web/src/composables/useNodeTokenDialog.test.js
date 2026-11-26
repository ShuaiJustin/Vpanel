import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { effectScope } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useNodeStore } from '@/stores/node'
import { useNodeTokenDialog } from './useNodeTokenDialog'

vi.mock('element-plus', () => ({
  ElMessage: { error: vi.fn(), success: vi.fn() },
  ElMessageBox: { confirm: vi.fn() }
}))
vi.mock('@/stores/node', () => ({
  useNodeStore: vi.fn(() => ({ nodes: [], generateToken: vi.fn(), rotateToken: vi.fn(), revokeToken: vi.fn() }))
}))

const first = { id: 28, name: 'First' }
const second = { id: 29, name: 'Second' }
let scope
let dialog
let store

beforeEach(() => {
  vi.clearAllMocks()
  ElMessageBox.confirm.mockResolvedValue()
  store = useNodeStore()
  useNodeStore.mockReturnValueOnce(store)
  scope = effectScope()
  dialog = scope.run(useNodeTokenDialog)
})

afterEach(() => scope.stop())

it('keeps a late token and completion message out of another node’s dialog', async () => {
  let resolveFirst
  store.generateToken
    .mockReturnValueOnce(new Promise(resolve => { resolveFirst = resolve }))
    .mockResolvedValueOnce({ token: 'second-token' })

  dialog.openTokenDialog(first)
  const pending = dialog.handleGenerateToken()
  await Promise.resolve()
  expect(store.generateToken).toHaveBeenCalledWith(first.id)

  dialog.tokenDialogVisible.value = false
  dialog.openTokenDialog(second)
  await dialog.handleGenerateToken()
  resolveFirst({ token: 'first-token' })
  await pending

  expect(store.generateToken).toHaveBeenLastCalledWith(second.id)
  expect(dialog.currentToken.value).toBe('second-token')
  expect(ElMessage.success).toHaveBeenCalledTimes(1)
})

it('discards a rotation result when the same node’s dialog is reopened', async () => {
  let resolveRotation
  store.rotateToken.mockReturnValue(new Promise(resolve => { resolveRotation = resolve }))
  dialog.openTokenDialog(first)
  const pending = dialog.handleRotateToken()
  await Promise.resolve()
  dialog.tokenDialogVisible.value = false
  dialog.openTokenDialog(first)

  expect(dialog.tokenLoading.value).toBe(true)
  await dialog.handleGenerateToken()
  expect(store.generateToken).not.toHaveBeenCalled()
  resolveRotation({ token: 'old-session-token' })
  await pending

  expect(dialog.currentToken.value).toBe('')
  expect(dialog.tokenLoading.value).toBe(false)
})

it('discards confirmation after closing the dialog and handles cancellation', async () => {
  let confirm
  ElMessageBox.confirm.mockReturnValueOnce(new Promise(resolve => { confirm = resolve }))
    .mockRejectedValueOnce('cancel')
  dialog.openTokenDialog(first)
  const pending = dialog.handleRevokeToken()
  dialog.tokenDialogVisible.value = false
  dialog.openTokenDialog(second)
  confirm()
  await pending
  await dialog.handleRevokeToken()

  expect(store.revokeToken).not.toHaveBeenCalled()
  expect(ElMessage.error).not.toHaveBeenCalled()
  expect(dialog.tokenLoading.value).toBe(false)
})

it('knows an existing token is configured without loading the secret into the dialog', async () => {
  store.nodes = [{ ...first, has_token: true }]
  dialog.openTokenDialog(first)
  expect(dialog.hasToken.value).toBe(true)
  expect(dialog.currentToken.value).toBe('')

  await dialog.handleRevokeToken()
  expect(store.revokeToken).toHaveBeenCalledWith(first.id)
})
