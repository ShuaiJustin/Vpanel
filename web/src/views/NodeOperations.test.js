import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { createPinia } from 'pinia'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { nodesApi } from '@/api'
import NodeOperations from './NodeOperations.vue'

let route
let wrapper

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push: vi.fn() })
}))

vi.mock('@/api', () => ({
  nodesApi: {
    get: vi.fn(), getNetworkOptimizationProfile: vi.fn(), listAvailableCoreVersions: vi.fn(),
    saveSSHMetadata: vi.fn(), inspectNetworkOptimization: vi.fn()
  }
}))

const profile = host => ({
  ssh_defaults: { host, port: 22, username: 'root', has_saved_private_key: true }
})

beforeEach(() => {
  vi.resetAllMocks()
  route = reactive({ params: { id: '28' } })
  nodesApi.get.mockImplementation(async id => ({ id: Number(id), name: `Node ${id}` }))
  nodesApi.getNetworkOptimizationProfile.mockImplementation(async id => profile(`2001:db8::${id}`))
  nodesApi.listAvailableCoreVersions.mockResolvedValue({ versions: [] })
})

afterEach(() => {
  wrapper?.unmount()
  vi.useRealTimers()
})

it('refreshes an active version switch until the Agent reports success', async () => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  nodesApi.get.mockResolvedValueOnce({
    id: 28, xray_version: 'Xray 26.10.31 (Linux)',
    recent_recovery_events: [{ command_id: 'install-1', command_type: 'xray_install_version', status: 'dispatched' }]
  }).mockResolvedValueOnce({
    id: 28, xray_version: 'Xray 26.11.9 (Linux)',
    recent_recovery_events: [{ command_id: 'install-1', command_type: 'xray_install_version', status: 'success' }]
  })
  const vm = mountOperations()
  await flushPromises()
  expect(vm.coreVersionPending).toBe(true)
  expect(vm.normalizeVersion(vm.node.xray_version)).toBe('26.10.31')
  await vi.advanceTimersByTimeAsync(3000)
  expect(vm.coreVersionPending).toBe(false)
  expect(vm.normalizeVersion(vm.node.xray_version)).toBe('26.11.9')
  await vi.advanceTimersByTimeAsync(3000)
  expect(nodesApi.get).toHaveBeenCalledTimes(2)
})

function mountOperations() {
  wrapper = shallowMount(NodeOperations, {
    global: { plugins: [createPinia(), ElementPlus], renderStubDefaultSlot: true }
  })
  return wrapper.vm
}

it('warns about incomplete history, preserves recovered records, and reserves the empty state for a successful load', async () => {
  nodesApi.get.mockResolvedValueOnce({ id: 28, operation_history_unavailable: true })
  const vm = mountOperations()
  await flushPromises()
  vm.activeWorkspace = 'events'
  await flushPromises()
  const warning = () => wrapper.find('el-alert-stub[title="历史操作记录未能完整恢复"]')
  const emptyState = () => wrapper.find('el-empty-stub[description="暂无操作记录"]')
  expect(warning().exists()).toBe(true)
  expect(emptyState().exists()).toBe(false)

  nodesApi.get.mockResolvedValueOnce({
    id: 28, operation_history_unavailable: true,
    recent_recovery_events: [{ command_id: 'sync-1', command_type: 'config_sync', status: 'success' }]
  })
  await vm.refreshData()
  expect(warning().exists()).toBe(true)
  expect(wrapper.find('el-timeline-item-stub').exists()).toBe(true)

  await vm.refreshData()
  expect(warning().exists()).toBe(false)
  expect(emptyState().exists()).toBe(true)
})

it('shows the version list timeout inline and clears it after syncing again', async () => {
  const message = 'GitHub 版本列表同步超时，请重试或直接输入版本号'
  nodesApi.listAvailableCoreVersions.mockRejectedValueOnce({ status: 502, requestId: 'req-1', message })
  const vm = mountOperations()
  await flushPromises()
  expect(wrapper.find('.core-version-panel [role="status"]').text()).toContain(message)

  nodesApi.listAvailableCoreVersions.mockResolvedValueOnce({ versions: [{ version: '26.11.9' }] })
  await vm.loadAvailableVersions(true)
  expect(wrapper.find('.core-version-panel').text()).not.toContain(message)
  expect(vm.availableVersions).toEqual([{ version: '26.11.9' }])
})

it('keeps the newer SSH defaults after leaving a node and then returning to it', async () => {
  let resolveProfile
  nodesApi.getNetworkOptimizationProfile.mockReturnValueOnce(new Promise(resolve => { resolveProfile = resolve }))
  const vm = mountOperations()
  await flushPromises()
  route.params.id = '29'
  await flushPromises()
  expect(vm.sshEndpoint).toBe('[2001:db8::29]:22 / root')

  route.params.id = '28'
  await flushPromises()
  resolveProfile(profile('2001:db8::a'))
  await flushPromises()

  expect(vm.sshEndpoint).toBe('[2001:db8::28]:22 / root')
})

it('keeps the SSH form open and skips inspection if saving fails', async () => {
  nodesApi.saveSSHMetadata.mockRejectedValue(new Error('SSH save failed'))
  const vm = mountOperations()
  await flushPromises()
  vm.networkOptimizationDialogVisible = true
  vm.sshForm.private_key = 'entered-private-key'
  await vm.useSavedSSHAndInspect()

  expect(nodesApi.saveSSHMetadata).toHaveBeenCalledWith(28, {
    host: '2001:db8::28', port: 22, username: 'root', private_key: 'entered-private-key'
  })
  expect(nodesApi.inspectNetworkOptimization).not.toHaveBeenCalled()
  expect(vm.networkOptimizationDialogVisible).toBe(true)
  expect(vm.sshForm.private_key).toBe('entered-private-key')
  expect(vm.savingSSHConfig).toBe(false)
})

it('saves SSH details without inspecting or discarding network settings in progress', async () => {
  nodesApi.saveSSHMetadata.mockResolvedValue({})
  nodesApi.inspectNetworkOptimization.mockResolvedValue({ state: { bbr_available: true } })
  const vm = mountOperations()
  await flushPromises()
  vm.activeWorkspace = 'network'
  await flushPromises()
  nodesApi.inspectNetworkOptimization.mockClear()
  vm.networkOptimizationForm.enable_fq = false
  vm.sshForm.port = 2222

  await vm.saveSSHConfig()
  expect(nodesApi.saveSSHMetadata).toHaveBeenCalledWith(28, expect.objectContaining({ port: 2222 }))
  expect(nodesApi.inspectNetworkOptimization).not.toHaveBeenCalled()
  expect(vm.networkOptimizationForm.enable_fq).toBe(false)

  await vm.saveSSHConfig({ inspectAfter: true })
  expect(nodesApi.inspectNetworkOptimization).toHaveBeenCalledTimes(1)
})

it('clears a profile read error after refreshing successfully', async () => {
  nodesApi.getNetworkOptimizationProfile.mockRejectedValueOnce(new Error('Profile unavailable'))
  const vm = mountOperations()
  await flushPromises()
  expect(vm.networkOptimizationError?.message).toContain('Profile unavailable')
  await vm.refreshData()
  expect(vm.networkOptimizationError).toBeNull()
})

it('can inspect with newly saved credentials when refreshing the profile fails', async () => {
  nodesApi.getNetworkOptimizationProfile.mockResolvedValueOnce({
    ssh_defaults: { host: '2001:db8::28', port: 22, username: 'root' }
  }).mockRejectedValueOnce(new Error('Profile unavailable'))
  nodesApi.saveSSHMetadata.mockResolvedValue({})
  nodesApi.inspectNetworkOptimization.mockResolvedValue({ state: { bbr_available: true } })
  const vm = mountOperations()
  await flushPromises()
  vm.sshForm.private_key = 'entered-private-key'
  await vm.saveSSHConfig({ inspectAfter: true })
  expect(vm.hasSavedSSHCredentials).toBe(true)
  expect(vm.sshForm.private_key).toBe('')
  expect(nodesApi.inspectNetworkOptimization).toHaveBeenCalledWith(28, expect.objectContaining({
    ssh: expect.objectContaining({ host: '2001:db8::28', private_key: '' })
  }))
})
