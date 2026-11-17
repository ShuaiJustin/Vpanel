import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

const mocks = vi.hoisted(() => ({
  confirm: vi.fn(), pay: vi.fn(), fetchOrder: vi.fn(), replace: vi.fn(), error: vi.fn(),
  order: { id: 3, order_no: 'ORD-test', plan_id: 2, status: 'pending', pay_amount: 1000 }
}))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { order_id: 3 } }),
  useRouter: () => ({ replace: mocks.replace, push: vi.fn(() => Promise.resolve()) })
}))
vi.mock('@/stores/order', () => ({ useOrderStore: () => ({
  currentOrder: mocks.order, fetchOrder: mocks.fetchOrder, createPayment: mocks.pay,
  getStatusInfo: () => ({ type: 'info', label: '待支付' })
}) }))
vi.mock('@/stores/balance', () => ({ useBalanceStore: () => ({ balance: 5000, fetchBalance: vi.fn() }) }))
vi.mock('@/api', () => ({ paymentsApi: { getMethods: vi.fn(async () => ({ methods: ['balance'] })) } }))
vi.mock('element-plus', () => ({
  ElMessageBox: { confirm: mocks.confirm },
  ElMessage: { error: mocks.error, success: vi.fn(), warning: vi.fn() }
}))
import Payment from './Payment.vue'

function mountPage() {
  return shallowMount(Payment, { global: { stubs: {
    ElCard: { template: '<div><slot /><slot name="header" /></div>' },
    ElAlert: { props: ['title'], template: '<div role="alert">{{ title }}</div>' },
    ElButton: { template: '<button @click="$emit(\'click\')"><slot /></button>' },
    ElDescriptions: true, ElDescriptionsItem: true, ElDialog: true, ElIcon: true, ElEmpty: true,
    ElSkeleton: true, ElTag: true, ElProgress: true
  } } })
}

describe('Payment activation confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.confirm.mockResolvedValue('confirm')
    mocks.pay.mockResolvedValue({ payment: {} })
    mocks.order.pay_amount = 1000
    mocks.order.payment_expired = false
  })

  it('explains renewal versus replacement and does not pay when cancelled', async () => {
    mocks.confirm.mockRejectedValue('cancel')
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('原套餐剩余天数不叠加')
    await wrapper.find('.pay-button').trigger('click')
    await flushPromises()
    expect(mocks.confirm).toHaveBeenCalledOnce()
    expect(mocks.pay).not.toHaveBeenCalled()
    expect(mocks.error).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('blocks duplicate payment attempts while confirmation is open', async () => {
    let confirm
    mocks.confirm.mockImplementation(() => new Promise(resolve => { confirm = resolve }))
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('.pay-button').trigger('click')
    await wrapper.find('.pay-button').trigger('click')
    expect(mocks.confirm).toHaveBeenCalledOnce()
    expect(mocks.pay).not.toHaveBeenCalled()
    confirm('confirm')
    await flushPromises()
    expect(mocks.pay).toHaveBeenCalledExactlyOnceWith('ORD-test', 'balance')
    wrapper.unmount()
  })

  it('opens zero-price orders without requiring a configured payment gateway', async () => {
    mocks.order.pay_amount = 0
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.find('.pay-button').text()).toBe('免费开通')
    await wrapper.find('.pay-button').trigger('click')
    await flushPromises()
    expect(mocks.pay).toHaveBeenCalledExactlyOnceWith('ORD-test', 'free')
    wrapper.unmount()
  })

  it('shows the expired notice and does not offer checkout', async () => {
    mocks.order.payment_expired = true
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('该订单已过期')
    expect(wrapper.find('.pay-button').exists()).toBe(false)
    expect(mocks.pay).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
