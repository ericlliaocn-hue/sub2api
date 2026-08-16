import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type FinanceCategory =
  | 'server'
  | 'database'
  | 'redis'
  | 'bandwidth'
  | 'proxy'
  | 'payment_fee'
  | 'marketing'
  | 'affiliate'
  | 'account_purchase'
  | 'customer_service'
  | 'risk_reserve'
  | 'other'

export type AllocationMethod =
  | 'revenue_share'
  | 'request_share'
  | 'token_share'
  | 'account_average'
  | 'manual'
  | 'direct'

export type CostFrequency = 'one_time' | 'daily' | 'monthly' | 'yearly'

export interface BusinessCostConfig {
  id: number
  code: string
  name: string
  category: FinanceCategory
  amount: number
  currency: string
  exchange_rate_to_billing_unit: number
  allocation_method: AllocationMethod
  frequency: CostFrequency
  scope: Record<string, unknown>
  effective_from: string
  effective_to?: string | null
  enabled: boolean
  notes: string
  created_by?: number | null
  created_at: string
  updated_at: string
}

export interface BusinessCostConfigInput {
  code: string
  name: string
  category: FinanceCategory
  amount: number
  currency?: string
  exchange_rate_to_billing_unit?: number
  allocation_method: AllocationMethod
  frequency?: CostFrequency
  scope?: Record<string, unknown>
  effective_from?: string
  effective_to?: string | null
  enabled?: boolean
  notes?: string
}

export interface BusinessExpense {
  id: number
  category: FinanceCategory
  name: string
  amount: number
  currency: string
  exchange_rate_to_billing_unit: number
  occurred_at: string
  period_start?: string | null
  period_end?: string | null
  allocation_method: AllocationMethod
  scope: Record<string, unknown>
  status: 'active' | 'void'
  notes: string
  created_by?: number | null
  created_at: string
  updated_at: string
}

export interface BusinessExpenseInput {
  category: FinanceCategory
  name: string
  amount: number
  currency?: string
  exchange_rate_to_billing_unit?: number
  occurred_at: string
  period_start?: string | null
  period_end?: string | null
  allocation_method: AllocationMethod
  scope?: Record<string, unknown>
  notes?: string
}

export interface UpstreamCostPrices {
  input: number
  cache_read: number
  cache_write: number
  output: number
}

export interface UpstreamCostVersion {
  id: number
  account_id: number
  account_name: string
  model: 'gpt-5.6-luna' | 'gpt-5.6-terra'
  short_prices: UpstreamCostPrices
  long_context_threshold: number
  long_prices: UpstreamCostPrices
  declared_multiplier: number
  balance_unit_cost: number
  notes: string
  effective_from: string
  created_by?: number | null
  created_at: string
}

export type UpstreamCostVersionInput = Omit<UpstreamCostVersion, 'id' | 'account_name' | 'effective_from' | 'created_by' | 'created_at'>

export interface FinanceMetric {
  requests: number
  tokens: number
  revenue: number
  direct_cost: number
  operating_cost: number
  total_cost: number
  gross_profit: number
  operating_profit: number
  gross_margin: number
  operating_margin: number
  cost_multiplier: number
}

export interface FinanceReportRow extends FinanceMetric { key: string; name: string }
export interface FinanceTrendPoint extends FinanceMetric { date: string }
export interface FinanceCostComponent {
  category: string
  name: string
  amount: number
  original_amount?: number
  currency?: string
  exchange_rate_to_billing_unit?: number
  allocation_method: string
  source: string
}
export interface FinanceRiskAlert { severity: string; dimension: string; key: string; name: string; reason: string; operating_profit: number; operating_margin: number; cost_multiplier: number }
export interface FinanceReport { start_time: string; end_time: string; dimension: string; summary: FinanceMetric; rows: FinanceReportRow[]; trend: FinanceTrendPoint[]; components: FinanceCostComponent[]; alerts: FinanceRiskAlert[] }
export interface FinanceGrowthSource { source: string; new_users: number; active_users: number; paying_users: number; revenue: number; recharge: number }
export interface FinanceGrowthReport { start_time: string; end_time: string; new_users: number; active_users: number; online_users: number; paying_users: number; recharge_amount: number; revenue: number; marketing_cost: number; affiliate_cost: number; cac: number; ltv: number; roi: number; by_source: FinanceGrowthSource[] }

const businessFinanceAPI = {
  listCostConfigs() {
    return apiClient.get<BusinessCostConfig[]>('/admin/business-finance/cost-configs')
  },
  createCostConfig(input: BusinessCostConfigInput) {
    return apiClient.post<BusinessCostConfig>('/admin/business-finance/cost-configs', input)
  },
  updateCostConfig(id: number, input: BusinessCostConfigInput) {
    return apiClient.put<BusinessCostConfig>(`/admin/business-finance/cost-configs/${id}`, input)
  },
  disableCostConfig(id: number) {
    return apiClient.post(`/admin/business-finance/cost-configs/${id}/disable`)
  },
  deleteCostConfig(id: number) {
    return apiClient.delete(`/admin/business-finance/cost-configs/${id}`)
  },
  listExpenses(params?: {
    page?: number
    page_size?: number
    category?: FinanceCategory
    status?: 'active' | 'void'
    start_time?: string
    end_time?: string
  }) {
    return apiClient.get<PaginatedResponse<BusinessExpense>>('/admin/business-finance/expenses', { params })
  },
  createExpense(input: BusinessExpenseInput) {
    return apiClient.post<BusinessExpense>('/admin/business-finance/expenses', input)
  },
  updateExpense(id: number, input: BusinessExpenseInput) {
    return apiClient.put<BusinessExpense>(`/admin/business-finance/expenses/${id}`, input)
  },
  voidExpense(id: number) {
    return apiClient.post(`/admin/business-finance/expenses/${id}/void`)
  },
  listUpstreamCostVersions(params?: { account_id?: number; model?: string }) {
    return apiClient.get<UpstreamCostVersion[]>('/admin/business-finance/upstream-cost-versions', { params })
  },
  createUpstreamCostVersion(input: UpstreamCostVersionInput) {
    return apiClient.post<UpstreamCostVersion>('/admin/business-finance/upstream-cost-versions', input)
  },
  getReport(params?: { start_time?: string; end_time?: string; dimension?: string; group_id?: number; channel_id?: number; model?: string; min_margin?: number }) {
    return apiClient.get<FinanceReport>('/admin/business-finance/report', { params })
  },
  getGrowth(params?: { start_time?: string; end_time?: string }) {
    return apiClient.get<FinanceGrowthReport>('/admin/business-finance/growth', { params })
  },
}

export default businessFinanceAPI
