import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'

const mocks = vi.hoisted(() => ({
  getByOrderNo: vi.fn(), fetchOrders: vi.fn(), route: {}, replace: vi.fn()
}))
vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({ replace: mocks.replace })
}))
vi.mock('@/api/index', () => ({ ordersApi: { getByOrderNo: mocks.getByOrderNo } }))
vi.mock('@/stores/order', () => ({ useOrderStore: () => ({
  loading: false,
  orders: [],
  pagination: { page: 1, pageSize: 10, total: 0 },
  fetchOrders: mocks.fetchOrders,
  getStatusInfo: () => ({ label: '已取消' })
}) }))
vi.mock('element-plus', () => ({ ElMessage: { success: vi.fn(), error: vi.fn() } }))

import Orders from './Orders.vue'

function mountPage(query = { payment: 'success', order_no: 'ORD-123' }) {
  mocks.route = reactive({ query })
  return shallowMount(Orders, { global: { stubs: {
    ElAlert: {
      props: ['title', 'type'],
      template: '<section role="status" :data-type="type">{{ title }}<slot /></section>'
    },
    ElButton: { template: '<button><slot /></button>' },
    ElCard: true, ElDialog: true, ElRadioGroup: true, ElRadioButton: true,
    ElSkeleton: true, ElTable: true, ElTableColumn: true, ElTag: true,
    ElEmpty: true, ElPagination: true, ElDescriptions: true, ElDescriptionsItem: true, RouterLink: true
  } } })
}

function order(status, orderNo = 'ORD-123') {
  return { order: { id: 1, order_no: orderNo, status } }
}

describe('payment return notice', () => {
  beforeEach(() => vi.resetAllMocks())

  it('waits for a verified order and allows retry from pending to completed', async () => {
    mocks.getByOrderNo.mockResolvedValueOnce(order('pending')).mockResolvedValueOnce(order('completed'))
    const wrapper = mountPage()
    const notice = () => wrapper.get('[role="status"]')

    expect(notice().text()).toContain('正在查询支付结果')
    await flushPromises()
    expect(notice().text()).toContain('正在等待支付确认')
    expect(notice().attributes('data-type')).toBe('info')
    const retry = notice().findAll('button').find(button => button.text() === '重新查询')
    await retry.trigger('click')
    await flushPromises()

    expect(notice().text()).toContain('支付成功，订单已完成')
    expect(notice().attributes('data-type')).toBe('success')
    expect(mocks.getByOrderNo).toHaveBeenCalledTimes(2)
    expect(mocks.replace).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('allows retry after failure and distinguishes paid from completed', async () => {
    mocks.getByOrderNo.mockRejectedValueOnce(new Error('Timeout')).mockResolvedValueOnce(order('paid'))
    const wrapper = mountPage()
    await flushPromises()
    const notice = () => wrapper.get('[role="status"]')
    expect(notice().text()).toContain('暂时无法确认支付结果')
    expect(notice().attributes('data-type')).toBe('warning')

    await notice().get('button').trigger('click')
    await flushPromises()
    expect(notice().text()).toContain('支付已确认')
    expect(notice().text()).not.toContain('订单已完成')
    wrapper.unmount()
  })

  it('checks an Alipay return against the order status', async () => {
    mocks.getByOrderNo.mockResolvedValueOnce(order('pending'))
    const wrapper = mountPage({ out_trade_no: 'ORD-123', trade_status: 'TRADE_SUCCESS' })
    await flushPromises()
    expect(mocks.getByOrderNo).toHaveBeenCalledWith('ORD-123', { silent: true })
    expect(wrapper.get('[role="status"]').text()).toContain('正在等待支付确认')
    wrapper.unmount()
  })

  it('ignores an old response when navigation selects another order', async () => {
    let resolveOld
    mocks.getByOrderNo
      .mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
      .mockResolvedValueOnce(order('cancelled', 'ORD-456'))
    const wrapper = mountPage()
    mocks.route.query.order_no = 'ORD-456'
    await flushPromises()

    resolveOld(order('completed'))
    await flushPromises()
    const notice = wrapper.get('[role="status"]')
    expect(notice.text()).toContain('已取消')
    expect(notice.text()).not.toContain('支付成功')
    wrapper.unmount()
  })
})
