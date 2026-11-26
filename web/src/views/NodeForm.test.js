import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { reactive, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { nodesApi } from '@/api'
import NodeForm from './NodeForm.vue'

let route
let wrapper
vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ push: vi.fn() })
}))
vi.mock('@/api', () => ({
  certificatesApi: { list: async () => [] },
  nodeGroupsApi: { list: async () => [] },
  nodesApi: { get: vi.fn(), create: vi.fn(), update: vi.fn(), createAccessDomain: vi.fn(), suggestName: vi.fn() }
}))

beforeEach(() => {
  vi.resetAllMocks()
  route = reactive({ params: {} })
})
afterEach(() => wrapper?.unmount())

function mountForm() {
  wrapper = shallowMount(NodeForm, {
    global: {
      plugins: [createPinia(), ElementPlus],
      stubs: {
        AdminStickyChrome: { template: '<div><slot /></div>' },
        ElCard: { template: '<div><slot name="header" /><slot /></div>' },
        ElForm: { methods: { validate: async () => true, clearValidate() {} }, template: '<form><slot /></form>' },
        ElFormItem: { template: '<div><slot /></div>' }
      }
    }
  })
  return wrapper.vm
}

it('shows a legacy management domain and its IP without replacing the stored address', async () => {
  route.params.id = '28'
  nodesApi.get.mockResolvedValue({ id: 28, name: 'legacy', address: 'old.example.com', resolved_ips: ['2001:db8::2'], port: 18443 })
  nodesApi.update.mockResolvedValue({ id: 28 })
  const vm = mountForm()
  await flushPromises()
  expect(wrapper.find('.address-summary code').text()).toBe('2001:db8::2')
  expect(vm.form.address).toBe('old.example.com')
  await vm.submitForm()
  expect(nodesApi.update).toHaveBeenCalledWith('28', expect.objectContaining({ address: 'old.example.com' }))
  expect(nodesApi.update.mock.calls[0][1]).not.toHaveProperty('access_domain')
})

it('requests automatic DNS by default when creating an IP node', async () => {
  nodesApi.create.mockResolvedValue({ id: 42, access_domain: 'node-42.example.com' })
  const vm = mountForm()
  await flushPromises()
  vm.form.address = '2001:db8::2'
  vm.form.name = 'test-node'
  await vm.submitForm()
  expect(nodesApi.create).toHaveBeenCalledWith(expect.objectContaining({ address: '2001:db8::2', auto_dns: true }))
})

it('submits once when clicked twice while validation is pending', async () => {
  nodesApi.create.mockResolvedValue({ id: 42 })
  const vm = mountForm()
  await flushPromises()
  vm.form.address = '2001:db8::2'
  vm.form.name = 'test-node'
  await Promise.all([vm.submitForm(), vm.submitForm()])
  expect(nodesApi.create).toHaveBeenCalledTimes(1)
})

it('ignores the previous node response and prevents a write while the current node is loading', async () => {
  route.params.id = '28'
  let finishFirst
  let finishSecond
  nodesApi.get.mockReturnValueOnce(new Promise(resolve => { finishFirst = resolve }))
    .mockReturnValueOnce(new Promise(resolve => { finishSecond = resolve }))
  const vm = mountForm()
  route.params.id = '29'
  await nextTick()
  await vm.submitForm()
  await vm.generateAccessDomain()
  expect(nodesApi.update).not.toHaveBeenCalled()
  expect(nodesApi.createAccessDomain).not.toHaveBeenCalled()
  finishSecond({ id: 29, name: 'new', address: '2001:db8::3' })
  await flushPromises()
  finishFirst({ id: 28, name: 'old', address: 'old.example.com', resolved_ips: ['2001:db8::2'] })
  await flushPromises()
  expect(vm.form.address).toBe('2001:db8::3')
  expect(wrapper.find('.address-summary code').text()).toBe('2001:db8::3')
})
