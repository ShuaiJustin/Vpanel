import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

const admin = vi.hoisted(() => ({ list: vi.fn(), refund: vi.fn(), get: vi.fn() }))
const issueReceipt = vi.hoisted(() => vi.fn())
vi.mock('@/api/index', () => ({
  ordersApi: { admin },
  invoicesApi: { admin: { issue: issueReceipt } },
  paymentsApi: { getMethods: async () => ({ methods: [] }) }
}))
vi.mock('element-plus', () => ({
  ElMessage: { error: vi.fn(), success: vi.fn() },
  ElMessageBox: { confirm: vi.fn() }
}))

import AdminOrders from './AdminOrders.vue'

function mountPage() {
  return shallowMount(AdminOrders, { global: {
    directives: { loading: () => {} },
    stubs: {
      AdminStickyChrome: true, ElTable: true, ElTableColumn: true,
      ElOption: true, ElSelect: true, ElDatePicker: true, ElInputNumber: true,
      ElPagination: true, ElDescriptions: true, ElDescriptionsItem: true, ElTag: true,
      ElDialog: {
        props: ['modelValue', 'title'],
        template: '<div v-if="modelValue" :aria-label="title"><slot /><slot name="footer" /></div>'
      },
      ElForm: { template: '<form><slot /></form>' },
      ElFormItem: { props: ['label', 'error'], template: '<label>{{ label }}<slot />{{ error }}</label>' },
      ElInput: {
        props: ['modelValue'], emits: ['update:modelValue'],
        template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
      },
      ElAlert: { props: ['title'], template: '<div role="status">{{ title }}<slot /></div>' },
      ElButton: { template: '<button type="button"><slot /></button>' }
    }
  } })
}

describe('refund confirmation', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    admin.list.mockResolvedValue({ orders: [], total: 0 })
    admin.refund.mockResolvedValue({})
  })

  it.each([
    ['0', '全额退款 ¥12.34', '订阅可能提前到期', 1234],
    ['12.34', '全额退款 ¥12.34', '订阅可能提前到期', 1234],
    ['2.34', '部分退款 ¥2.34', '保留已发放的套餐权益', 234]
  ])('shows entitlement and balance impact before confirming %s', async (input, title, impact, amount) => {
    const wrapper = mountPage()
    await wrapper.vm.openRefundDialog({ id: 4, order_no: 'ORD-4', status: 'paid', pay_amount: 1000, balance_used: 234 })
    const dialog = wrapper.get('[aria-label="订单退款"]')
    await dialog.get('input[inputmode="decimal"]').setValue(input)

    expect(dialog.text()).toContain('款项退至用户账户余额')
    expect(dialog.get('[role="status"]').text()).toContain(title)
    expect(dialog.get('[role="status"]').text()).toContain(impact)
    await dialog.findAll('button').find(button => button.text().startsWith('确认退款')).trigger('click')
    await flushPromises()
    expect(admin.refund).toHaveBeenCalledExactlyOnceWith(4, { amount, reason: '' })
    wrapper.unmount()
  })

  it('prevents confirmation when the entered amount exceeds the order total', async () => {
    const wrapper = mountPage()
    await wrapper.vm.openRefundDialog({ id: 4, order_no: 'ORD-4', status: 'paid', pay_amount: 1000, balance_used: 234 })
    const dialog = wrapper.get('[aria-label="订单退款"]')
    await dialog.get('input[inputmode="decimal"]').setValue('12.35')

    expect(dialog.text()).toContain('请输入可退范围内的金额')
    expect(dialog.findAll('button').find(button => button.text() === '确认退款').element.disabled).toBe(true)
    expect(admin.refund).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})

describe('transaction receipt', () => {
  it('offers issuance for a paid order and shows its receipt number', async () => {
    admin.get.mockResolvedValueOnce({ order: { id: 4, status: 'pending' } })
    admin.get.mockResolvedValueOnce({ order: { id: 4, status: 'paid' } })
    issueReceipt.mockResolvedValueOnce({ invoice: { invoice_no: 'RCPT-4' } })
    const wrapper = mountPage()
    await wrapper.vm.viewDetail({ id: 4 })
    const dialog = () => wrapper.get('[aria-label="订单详情"]')
    expect(dialog().text()).not.toContain('生成交易凭证')
    await wrapper.vm.viewDetail({ id: 4 })
    await dialog().get('button').trigger('click')
    await flushPromises()
    expect(issueReceipt).toHaveBeenCalledExactlyOnceWith(4)
    expect(dialog().get('[role="status"]').text()).toContain('交易凭证 RCPT-4 已生成')
    wrapper.unmount()
  })
})
