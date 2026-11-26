import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { createPinia } from 'pinia'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { nodeGroupsApi, nodesApi, usersApi } from '@/api'
import NodeDetail from './NodeDetail.vue'

const { routerPush } = vi.hoisted(() => ({ routerPush: vi.fn() }))
let route
let wrapper

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push: routerPush })
}))

vi.mock('@/api', () => ({
  nodeGroupsApi: { list: vi.fn() },
  nodesApi: { get: vi.fn(), getTraffic: vi.fn(), getTopUsers: vi.fn(), getTrafficDiagnostic: vi.fn() },
  usersApi: { get: vi.fn() }
}))

beforeEach(() => {
  vi.resetAllMocks()
  route = reactive({ params: { id: '28' } })
  nodesApi.get.mockImplementation(async id => ({ id: Number(id), name: `Node ${id}` }))
  nodesApi.getTrafficDiagnostic.mockResolvedValue({})
  nodeGroupsApi.list.mockResolvedValue([])
})

afterEach(() => {
  wrapper?.unmount()
  vi.useRealTimers()
})

function mountDetail() {
  wrapper = shallowMount(NodeDetail, {
    global: {
      plugins: [createPinia(), ElementPlus],
      renderStubDefaultSlot: true,
      stubs: {
        ElTableColumn: { template: '<div />' },
        ElCard: { template: '<div><slot name="header" /><slot /></div>' },
        PageSectionHeader: { template: '<div><slot /></div>' }
      }
    }
  })
  return wrapper
}

it('polls an initial diagnosis only until a result arrives and requests a fresh diagnosis on click', async () => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  nodesApi.getTrafficDiagnostic
    .mockResolvedValueOnce({ node_id: 28, diagnostic_status: 'pending' })
    .mockResolvedValue({ node_id: 28, diagnostic_status: 'healthy_idle', traffic: { last_record_count: 0 } })
  const view = mountDetail()
  await flushPromises()
  expect(view.text()).toContain('正在读取节点的流量采集状态')
  expect(view.text()).not.toContain('最近记录数')

  await vi.advanceTimersByTimeAsync(1000)
  expect(view.text()).toContain('采集正常（空闲）')
  await vi.advanceTimersByTimeAsync(3000)
  expect(nodesApi.getTrafficDiagnostic.mock.calls).toEqual([[28, false], [28, false]])

  const refresh = view.findAllComponents({ name: 'ElButton' }).find(button => button.text().includes('刷新诊断'))
  await refresh.vm.$emit('click')
  expect(nodesApi.getTrafficDiagnostic).toHaveBeenLastCalledWith(28, true)
})

it('keeps the new node’s traffic, user ranking, group and diagnosis when the old node responds late', async () => {
  let resolveTraffic
  let resolveTop
  let resolveGroups
  let resolveDiagnostic
  nodesApi.getTraffic.mockReturnValueOnce(new Promise(resolve => { resolveTraffic = resolve }))
    .mockResolvedValueOnce({ upload: 29 })
  nodesApi.getTopUsers.mockReturnValueOnce(new Promise(resolve => { resolveTop = resolve }))
    .mockResolvedValueOnce({ top_users: [{ user_id: 29, username: 'second' }] })
  nodeGroupsApi.list.mockReturnValueOnce(new Promise(resolve => { resolveGroups = resolve }))
    .mockResolvedValueOnce([{ id: 2, nodes: [{ id: 29 }] }])
  nodesApi.getTrafficDiagnostic.mockReturnValueOnce(new Promise(resolve => { resolveDiagnostic = resolve }))
    .mockResolvedValueOnce({ node_id: 29 })

  const vm = mountDetail().vm
  await flushPromises()
  route.params.id = '29'
  await flushPromises()

  resolveTraffic({ upload: 28 })
  resolveTop({ top_users: [{ user_id: 28, username: 'first' }] })
  resolveGroups([{ id: 1, nodes: [{ id: 29 }] }])
  resolveDiagnostic({ node_id: 28 })
  await flushPromises()

  expect(vm.trafficStats.upload).toBe(29)
  expect(vm.topUsers[0].username).toBe('second')
  expect(vm.nodeGroups[0].id).toBe(2)
  expect(vm.trafficDiagnostic.node_id).toBe(29)
  expect(vm.trafficDiagnosticLoading).toBe(false)
})

it('refreshes both traffic views on period change and ignores late username lookups', async () => {
  let resolveUser
  usersApi.get.mockReturnValueOnce(new Promise(resolve => { resolveUser = resolve }))
  nodesApi.getTraffic.mockResolvedValueOnce({ upload: 1 }).mockResolvedValueOnce({ upload: 7 })
  nodesApi.getTopUsers.mockResolvedValueOnce({ top_users: [{ user_id: 1 }] })
    .mockResolvedValueOnce({ top_users: [{ user_id: 7, username: 'this-week' }] })

  const view = mountDetail()
  await flushPromises()
  expect(usersApi.get).toHaveBeenCalledWith(1)

  const period = view.findComponent({ name: 'ElRadioGroup' })
  period.vm.$emit('update:modelValue', 'week')
  period.vm.$emit('change', 'week')
  await flushPromises()
  resolveUser({ username: 'today' })
  await flushPromises()

  expect(nodesApi.getTraffic).toHaveBeenCalledTimes(2)
  expect(nodesApi.getTopUsers).toHaveBeenCalledTimes(2)
  const statsRange = nodesApi.getTraffic.mock.calls[1][1]
  expect(nodesApi.getTopUsers).toHaveBeenLastCalledWith(28, { ...statsRange, limit: 10 })
  expect(view.vm.trafficStats.upload).toBe(7)
  expect(view.vm.topUsers[0].username).toBe('this-week')
})
