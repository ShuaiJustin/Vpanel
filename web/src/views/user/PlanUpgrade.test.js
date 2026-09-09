import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

const mocks = vi.hoisted(() => ({
  push: vi.fn(() => Promise.resolve()),
  user: { expires_at: '2027-01-01T00:00:00Z', plan_id: 999 },
  fetchProfile: vi.fn(),
  current: vi.fn(),
  pending: vi.fn(),
  plans: vi.fn(),
  calculate: vi.fn(),
  upgrade: vi.fn(() => Promise.resolve()),
  downgrade: vi.fn(() => Promise.resolve())
}))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: mocks.push }) }))
vi.mock('@/stores/userPortal', () => ({ useUserPortalStore: () => ({ user: mocks.user, fetchProfile: mocks.fetchProfile }) }))
vi.mock('element-plus', () => ({
  ElMessage: { error: vi.fn(), info: vi.fn(), success: vi.fn() },
  ElMessageBox: { confirm: vi.fn(() => Promise.resolve()) }
}))
vi.mock('@/api', () => ({
  planChangeApi: {
    getCurrentPlan: mocks.current,
    getPendingDowngrade: mocks.pending,
    calculate: mocks.calculate,
    upgrade: mocks.upgrade,
    downgrade: mocks.downgrade,
    cancelDowngrade: vi.fn()
  },
  plansApi: { list: mocks.plans }
}))

import PlanUpgrade from './PlanUpgrade.vue'

const paidPlan = { id: 7, name: '已购套餐', price: 115, duration: 30, traffic_limit: 1000 }
const newPlan = { id: 8, name: '目标套餐', price: 300, duration: 30, traffic_limit: 2000, is_active: true }

function mountPage() {
  return shallowMount(PlanUpgrade, {
    global: { stubs: {
      'el-card': { template: '<div><slot name="header" /><slot /></div>' },
      'el-tag': { template: '<span><slot /></span>' },
      'el-button': { template: '<button @click="$emit(\'click\')"><slot /></button>' },
      'el-alert': { props: ['title'], template: '<div role="alert">{{ title }}<slot name="title" /><slot /></div>' },
      'el-empty': { template: '<div><slot name="description" /><slot /></div>' },
      'el-skeleton': true,
      'el-icon': true
    } }
  })
}

describe('PlanUpgrade server-owned plan', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.current.mockResolvedValue({ data: paidPlan })
    mocks.plans.mockResolvedValue({ plans: [newPlan] })
    mocks.pending.mockResolvedValue({ data: null })
    mocks.calculate.mockResolvedValue({ data: { current_plan: paidPlan, new_plan: newPlan, is_upgrade: true, price_difference: 100 } })
  })

  it('shows the server plan even when absent from active catalog and profile plan ID is stale', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(mocks.current).toHaveBeenCalledOnce()
    expect(wrapper.find('.current-plan-card').text()).toContain('已购套餐')
    expect(wrapper.find('.current-plan-card').text()).toContain('¥1.15')
    expect(wrapper.find('.pending-alert').exists()).toBe(false)
    expect(wrapper.find('.empty-state-card').exists()).toBe(false)
    wrapper.unmount()
  })

  it('quotes and upgrades using the server-confirmed plan ID', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('.plan-card').trigger('click')
    await flushPromises()
    expect(mocks.calculate).toHaveBeenCalledWith({ current_plan_id: 7, new_plan_id: 8 })
    const confirm = wrapper.findAll('button').find(button => button.text().includes('确认升级'))
    await confirm.trigger('click')
    await flushPromises()
    expect(mocks.upgrade).toHaveBeenCalledWith({ current_plan_id: 7, new_plan_id: 8 })
    expect(mocks.push).toHaveBeenCalledWith({ name: 'UserSubscription' })
    wrapper.unmount()
  })

  it('does not disguise an ownership lookup failure as a missing subscription', async () => {
    mocks.current.mockRejectedValue({ code: 'CURRENT_PLAN_UNKNOWN', message: '请联系管理员核对套餐归属' })
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('请联系管理员核对套餐归属')
    expect(wrapper.find('.empty-state-card').exists()).toBe(false)
    expect(wrapper.find('.plans-section').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows the purchase empty state only for a confirmed inactive subscription', async () => {
    mocks.current.mockRejectedValue({ code: 'NO_ACTIVE_SUBSCRIPTION' })
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.find('.empty-state-card').exists()).toBe(true)
    expect(wrapper.find('.current-plan-card').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows a failed pending-downgrade lookup instead of silently implying none exists', async () => {
    mocks.pending.mockRejectedValue({ message: '预约状态暂不可用' })
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.text()).toContain('预约状态暂不可用')
    expect(wrapper.find('.pending-alert').exists()).toBe(false)
    wrapper.unmount()
  })
})
