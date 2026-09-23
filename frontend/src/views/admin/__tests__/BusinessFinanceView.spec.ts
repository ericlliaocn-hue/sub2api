import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { listExpenses, listCostConfigs, listGroups, listChannels, getProfitCalendar } = vi.hoisted(() => ({
  listExpenses: vi.fn(),
  listCostConfigs: vi.fn(),
  listGroups: vi.fn(),
  listChannels: vi.fn(),
  getProfitCalendar: vi.fn(),
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<div><slot /></div>' },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/api/admin/businessFinance', () => ({
  default: {
    listCostConfigs,
    listExpenses,
    createExpense: vi.fn(),
    updateExpense: vi.fn(),
    voidExpense: vi.fn(),
    getProfitCalendar,
  },
}))

vi.mock('@/api/admin/groups', () => ({
  list: listGroups,
}))

vi.mock('@/api/admin/channels', () => ({
  list: listChannels,
}))

vi.mock('@/api/admin/accounts', () => ({
  list: vi.fn().mockResolvedValue({ items: [], total: 0 }),
  getById: vi.fn(),
}))

import BusinessFinanceView from '../BusinessFinanceView.vue'

describe('BusinessFinanceView expenses', () => {
  beforeEach(() => {
    listCostConfigs.mockReset().mockResolvedValue({ data: [] })
    listGroups.mockReset().mockResolvedValue({ items: [{ id: 19, name: 'pro号池' }], total: 1 })
    listChannels.mockReset().mockResolvedValue({ items: [], total: 0 })
    getProfitCalendar.mockReset().mockResolvedValue({
      data: {
        start_time: '2026-09-21T00:00:00+08:00',
        end_time: '2026-09-23T00:00:00+08:00',
        timezone: 'Asia/Shanghai',
        summary: {
          accounts: 1, recorded: 1, unrecorded: 0, recouped: 1, short: 0,
          cost: 68, official_tokens: 800, official_billing: 15, user_billing: 92.21, profit: 24.21,
        },
        days: [{
          date: '2026-09-22',
          cost: 68, official_tokens: 800, official_billing: 15, user_billing: 92.21, profit: 24.21,
          recorded: 1, unrecorded: 0,
          rows: [{
            account_ids: [66288],
            account_name: 'barbaragreend523@gmail.com',
            imported_at: '2026-09-22T17:15:13+08:00',
            status: 'active',
            schedulable: true,
            expense_id: 56,
            cost: 68,
            cost_recorded: true,
            requests: 8,
            official_tokens: 800,
            official_billing: 15,
            user_billing: 92.21,
            profit: 24.21,
          }],
        }],
      },
    })
    listExpenses.mockReset().mockResolvedValue({
      data: {
        items: [
          {
            id: 8,
            category: 'account_purchase',
            name: 'linda',
            amount: 45,
            currency: 'CNY',
            exchange_rate_to_billing_unit: 1,
            occurred_at: '2026-09-20T11:26:00Z',
            period_start: '2026-09-20T11:26:00Z',
            period_end: '2026-09-20T15:00:00Z',
            allocation_method: 'direct',
            scope: { account_id: 66274 },
            status: 'active',
            notes: '',
            recoup: {
              account_id: 66274,
              account_name: 'lindaphillipsn905+inv@gmail.com',
              billed: 32.89,
              requests: 3176,
              cost: 45,
              profit: -12.11,
            },
          },
        ],
        total: 1,
      },
    })
  })

  it('shows recoup columns and summary on the expense tab', async () => {
    const wrapper = mount(BusinessFinanceView)
    await flushPromises()
    await wrapper.get('[data-testid="tab-expenses"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('已收回')
    expect(wrapper.text()).toContain('32.89')
    expect(wrapper.text()).toContain('-12.11')
    expect(wrapper.text()).toContain('lindaphillipsn905+inv@gmail.com')
    expect(wrapper.get('[data-testid="expense-recoup-summary"]').text()).toContain('45.00')
    expect(wrapper.get('[data-testid="expense-recoup-summary"]').text()).toContain('未过本 1')
  })

  it('defaults a new expense to account purchase and direct allocation', async () => {
    const wrapper = mount(BusinessFinanceView)
    await flushPromises()
    await wrapper.get('[data-testid="tab-expenses"]').trigger('click')
    await wrapper.get('[data-testid="expense-create"]').trigger('click')

    expect(wrapper.text()).toContain('记一笔采购')
    const formSelects = wrapper.findAll('form select')
    expect((formSelects[0].element as HTMLSelectElement).value).toBe('account_purchase')
    expect((formSelects[1].element as HTMLSelectElement).value).toBe('direct')
  })

  it('passes filters to the expense list API', async () => {
    const wrapper = mount(BusinessFinanceView)
    await flushPromises()
    await wrapper.get('[data-testid="tab-expenses"]').trigger('click')
    await wrapper.get('input[placeholder="名称或备注"]').setValue('linda')
    await wrapper.findAll('select')[0].setValue('account_purchase')
    await wrapper.get('[data-testid="expense-filter"]').trigger('click')
    await flushPromises()

    expect(listExpenses).toHaveBeenCalledWith(expect.objectContaining({
      keyword: 'linda',
      category: 'account_purchase',
      status: 'active',
    }))
  })

  it('shows the profit calendar on the default tab', async () => {
    const wrapper = mount(BusinessFinanceView)
    await flushPromises()

    expect(wrapper.get('[data-testid="profit-calendar"]').text()).toContain('barbaragreend523@gmail.com')
    expect(wrapper.get('[data-testid="calendar-summary"]').text()).toContain('24.21')
    expect(wrapper.get('[data-testid="calendar-summary"]').text()).toContain('92.21')
    expect(wrapper.text()).toContain('在跑')
    expect(getProfitCalendar).toHaveBeenCalled()
  })

  it('queries the last two shanghai days from the preset', async () => {
    const wrapper = mount(BusinessFinanceView)
    await flushPromises()
    getProfitCalendar.mockClear()
    await wrapper.get('[data-testid="calendar-preset-2d"]').trigger('click')
    await flushPromises()

    expect(getProfitCalendar).toHaveBeenCalledWith(expect.objectContaining({
      start_time: expect.stringMatching(/T00:00:00\+08:00$/),
      end_time: expect.stringMatching(/T00:00:00\+08:00$/),
    }))
  })
})
