import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

const api = vi.hoisted(() => ({ list: vi.fn(), create: vi.fn(), update: vi.fn() }))
vi.mock('@/api/index', () => ({ plansApi: { admin: api } }))
vi.mock('element-plus', () => ({
  ElMessage: { error: vi.fn(), success: vi.fn() },
  ElMessageBox: { confirm: vi.fn() }
}))

import AdminPlans from './AdminPlans.vue'

function mountPage() {
  return shallowMount(AdminPlans, {
    global: {
      directives: { loading: () => {} },
      stubs: {
        ElIcon: true,
        ElSwitch: true,
        ElTag: true,
        ElTable: true,
        ElTableColumn: true,
        ElPagination: true,
        AdminStickyChrome: { template: '<div><slot /></div>' },
        ElDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ElForm: {
          methods: { validate: () => Promise.resolve(true), clearValidate: () => {} },
          template: '<form><slot /></form>'
        },
        ElFormItem: { props: ['label'], template: '<label :data-field="label"><slot /></label>' },
        ElSelect: {
          props: ['modelValue'], emits: ['update:modelValue', 'change'],
          template: `<select :value="modelValue" @change="$emit('update:modelValue', $event.target.value); $emit('change', $event.target.value)"><slot /></select>`
        },
        ElOption: { props: ['label', 'value'], template: '<option :value="value">{{ label }}</option>' },
        ElInputNumber: {
          props: ['modelValue', 'disabled'], emits: ['update:modelValue', 'change'],
          template: `<input type="number" :value="modelValue" :disabled="disabled" @input="$emit('update:modelValue', $event.target.valueAsNumber); $emit('change', $event.target.valueAsNumber)" />`
        },
        ElInput: {
          props: ['modelValue'], emits: ['update:modelValue'],
          template: `<input :value="modelValue" @input="$emit('update:modelValue', $event.target.value)" />`
        },
        ElButton: { template: '<button type="button" @click="$emit(\'click\')"><slot /></button>' }
      }
    }
  })
}

const field = (wrapper, label, selector = 'input') => wrapper.find(`[data-field="${label}"] ${selector}`)
const duration = wrapper => Number(field(wrapper, '时长(天)').element.value)
const monthly = wrapper => Number(field(wrapper, '每月流量').element.value)
const total = wrapper => Number(field(wrapper, '套餐流量').element.value)

async function submit(wrapper) {
  await wrapper.findAll('button').find(button => button.text() === '确定').trigger('click')
  await flushPromises()
}

describe('AdminPlans period presets', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.list.mockResolvedValue({ plans: [], total: 0 })
    api.create.mockResolvedValue({})
    api.update.mockResolvedValue({})
  })

  it('puts the period first and keeps the monthly quota without converting it to an annual total', async () => {
    const wrapper = mountPage()
    expect(wrapper.find('form label').attributes('data-field')).toBe('套餐周期')
    await field(wrapper, '每月流量').setValue(100)
    await field(wrapper, '价格(元)').setValue(25)
    expect(monthly(wrapper)).toBe(100)
    await field(wrapper, '套餐周期', 'select').setValue('quarterly')
    expect(duration(wrapper)).toBe(90)
    expect(monthly(wrapper)).toBe(100)
    await field(wrapper, '套餐周期', 'select').setValue('yearly')
    expect(duration(wrapper)).toBe(365)
    expect(monthly(wrapper)).toBe(100)
    await field(wrapper, '套餐周期', 'select').setValue('monthly')
    expect(duration(wrapper)).toBe(30)
    expect(monthly(wrapper)).toBe(100)
    expect(Number(field(wrapper, '价格(元)').element.value)).toBe(25)
    wrapper.unmount()
  })

  it('allows custom days and total quota and submits the custom type', async () => {
    const wrapper = mountPage()
    await field(wrapper, '套餐周期', 'select').setValue('custom')
    expect(field(wrapper, '每月流量').exists()).toBe(false)
    expect(field(wrapper, '套餐流量').element.disabled).toBe(false)
    await field(wrapper, '时长(天)').setValue(45)
    await field(wrapper, '套餐流量').setValue(123.45)
    await field(wrapper, '名称').setValue('自定义套餐')
    await submit(wrapper)
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({
      duration: 45, plan_type: 'custom', traffic_limit: Math.round(123.45 * 1024 ** 3)
    }))
    await field(wrapper, '套餐周期', 'select').setValue('quarterly')
    expect(duration(wrapper)).toBe(90)
    expect(monthly(wrapper)).toBeCloseTo(123.45)
    wrapper.unmount()
  })

  it('switches a manually changed preset duration to custom and retains its quota', async () => {
    const wrapper = mountPage()
    await field(wrapper, '每月流量').setValue(100)
    await field(wrapper, '套餐周期', 'select').setValue('yearly')
    await field(wrapper, '时长(天)').setValue(180)
    expect(field(wrapper, '套餐周期', 'select').element.value).toBe('custom')
    expect(total(wrapper)).toBe(100)
    expect(field(wrapper, '套餐流量').exists()).toBe(true)
    wrapper.unmount()
  })

  it('keeps an existing annual quota byte-for-byte when opened and saved', async () => {
    const wrapper = mountPage()
    const quota = 1000 * 1024 ** 3 + 123
    await wrapper.vm.editPlan({
      id: 2, name: 'Existing annual', plan_type: 'yearly', duration: 365,
      traffic_limit: quota, price: 10000, reset_cycle: 'monthly'
    })
    await submit(wrapper)
    expect(api.update).toHaveBeenCalledWith(2, expect.objectContaining({
      plan_type: 'yearly', duration: 365, traffic_limit: quota, price: 10000
    }))
    wrapper.unmount()
  })

  it('keeps unlimited quota across presets and resets the reference when creating again', async () => {
    const wrapper = mountPage()
    await field(wrapper, '套餐周期', 'select').setValue('yearly')
    expect(monthly(wrapper)).toBe(0)
    await field(wrapper, '每月流量').setValue(100)
    await wrapper.vm.showCreateDialog()
    expect(duration(wrapper)).toBe(30)
    expect(monthly(wrapper)).toBe(0)
    expect(Number(field(wrapper, '每月流量').element.value)).toBe(0)
    wrapper.unmount()
  })

  it('does not treat an empty reference as an unlimited quota', async () => {
    const wrapper = mountPage()
    await field(wrapper, '每月流量').setValue('')
    expect(field(wrapper, '每月流量').element.value).toBe('')
    wrapper.unmount()
  })
})
