import { describe, expect, it } from 'vitest'

import {
  asPositiveInt,
  buildFinanceScope,
  emptyFinanceScopeFields,
  formatScopeLabel,
  readFinanceScope,
  recoupProgress,
  summarizeExpenseRecoup,
} from '../businessFinanceScope'
import type { BusinessExpense } from '@/api/admin/businessFinance'

const expense = (overrides: Partial<BusinessExpense> = {}): BusinessExpense => ({
  id: 1,
  category: 'account_purchase',
  name: 'linda',
  amount: 45,
  currency: 'CNY',
  exchange_rate_to_billing_unit: 1,
  occurred_at: '2026-09-20T11:26:00Z',
  allocation_method: 'direct',
  scope: { account_id: 66274 },
  status: 'active',
  notes: '',
  created_at: '2026-09-20T11:26:00Z',
  updated_at: '2026-09-20T11:26:00Z',
  recoup: {
    account_id: 66274,
    account_name: 'lindaphillipsn905+inv@gmail.com',
    billed: 32.89,
    requests: 3176,
    cost: 45,
    profit: -12.11,
  },
  ...overrides,
})

describe('businessFinanceScope', () => {
  it('builds and reads account-scoped fields', () => {
    const scope = buildFinanceScope({
      accountId: 66274,
      groupId: 19,
      channelId: null,
      model: ' gpt-5.6-sol ',
    })
    expect(scope).toEqual({ account_id: 66274, group_id: 19, model: 'gpt-5.6-sol' })
    expect(readFinanceScope(scope)).toEqual({
      accountId: 66274,
      groupId: 19,
      channelId: null,
      model: 'gpt-5.6-sol',
    })
    expect(emptyFinanceScopeFields().accountId).toBeNull()
    expect(asPositiveInt('66274')).toBe(66274)
  })

  it('summarizes recoup for bound active expenses only', () => {
    const summary = summarizeExpenseRecoup([
      expense(),
      expense({
        id: 2,
        name: 'thannnkhr',
        recoup: { account_id: 66272, billed: 73.39, requests: 5015, cost: 40, profit: 33.39 },
      }),
      expense({ id: 3, status: 'void', recoup: { account_id: 1, billed: 10, requests: 1, cost: 10, profit: 0 } }),
      expense({ id: 4, recoup: null, scope: {} }),
    ])
    expect(summary.bound).toBe(2)
    expect(summary.recouped).toBe(1)
    expect(summary.short).toBe(1)
    expect(summary.cost).toBeCloseTo(85)
    expect(summary.billed).toBeCloseTo(106.28)
    expect(summary.profit).toBeCloseTo(21.28)
  })

  it('caps recoup progress at 100', () => {
    expect(recoupProgress({ account_id: 1, billed: 60, requests: 10, cost: 45, profit: 15 })).toBeCloseTo(100)
    expect(recoupProgress({ account_id: 1, billed: 22.5, requests: 10, cost: 45, profit: -22.5 })).toBeCloseTo(50)
    expect(recoupProgress(null)).toBe(0)
  })

  it('prefers account names over raw ids', () => {
    expect(formatScopeLabel({ account_id: 66274 }, { account: 'linda@x.com' })).toBe('linda@x.com')
    expect(formatScopeLabel({}, {})).toBe('全局')
  })
})
