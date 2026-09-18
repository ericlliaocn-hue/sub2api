import { apiClient } from '../client'

export interface PromotionPromoter {
  id: number
  name: string
  contact: string
  commission_rate: number
  commission_freeze_days: number
  enabled: boolean
  notes: string
  created_at?: string
  updated_at?: string
}

export interface PromotionPromoterInput {
  name: string
  contact: string
  commission_rate: number
  commission_freeze_days: number
  enabled: boolean
  notes: string
}

/** 获客分类：官网 / 搜索 / 邀请 / 其他。与后端 service.AcquisitionClass* 一致。 */
export type AcquisitionClass = 'official' | 'seo' | 'invite' | 'other'
export const ACQUISITION_CLASSES: AcquisitionClass[] = ['official', 'seo', 'invite', 'other']

export interface PromotionChannel {
  id: number
  code: string
  name: string
  channel_type: AcquisitionClass | string
  promoter_id?: number | null
  promoter_name?: string
  commission_rate?: number | null
  enabled: boolean
  /** 系统渠道（OFFICIAL / SEO / INVITE / MANUAL / EXTERNAL）：只能改名称和备注 */
  system: boolean
  notes: string
  created_at?: string
  updated_at?: string
}

export interface PromotionChannelInput {
  code: string
  name: string
  channel_type: AcquisitionClass
  promoter_id: number | null
  commission_rate: number | null
  enabled: boolean
  notes: string
}

export interface PromotionReportRow {
  channel_id: number
  code: string
  name: string
  channel_type: string
  acquisition_class: AcquisitionClass
  system: boolean
  promoter_name: string
  visits: number
  conversion_rate: number
  new_users: number
  invited_users: number
  paying_users: number
  active_users: number
  recharge: number
  revenue: number
  upstream_cost: number
  bonus_cost: number
  affiliate_cost: number
  commission_cost: number
  payment_fee: number
  marketing_cost: number
  profit: number
  cac: number
  ltv: number
  roi: number
}

export interface PromotionReportClassRow {
  class: AcquisitionClass
  visits: number
  conversion_rate: number
  new_users: number
  new_users_share: number
  invited_users: number
  paying_users: number
  active_users: number
  recharge: number
  revenue: number
  upstream_cost: number
  bonus_cost: number
  affiliate_cost: number
  commission_cost: number
  payment_fee: number
  marketing_cost: number
  profit: number
  cac: number
  ltv: number
  roi: number
}

export interface PromotionReportTotals {
  visits: number
  conversion_rate: number
  registered_users: number
  new_users: number
  unattributed_users: number
  invited_users: number
  paying_users: number
  active_users: number
  recharge: number
  revenue: number
  profit: number
}

export interface PromotionBreakdownRow {
  key: string
  label: string
  visits: number
  new_users: number
  paying_users: number
  revenue: number
  extra: number
}

export interface PromotionReport {
  start_time: string
  end_time: string
  mode: 'operation' | 'acquisition'
  totals: PromotionReportTotals
  classes: PromotionReportClassRow[]
  rows: PromotionReportRow[]
  seo_engines: PromotionBreakdownRow[]
  seo_landing_pages: PromotionBreakdownRow[]
  inviters: PromotionBreakdownRow[]
  external_hosts: PromotionBreakdownRow[]
}

export type PromotionAttributionOutcome =
  | 'attributed'
  | 'already_attributed'
  | 'invalid_code'
  | 'channel_disabled'
  | 'default_official'
  | 'resolved'

export interface PromotionAttributionEvent {
  id: number
  user_id: number
  user_email: string
  requested_code: string
  channel_id?: number | null
  channel_name: string
  acquisition_class: AcquisitionClass | ''
  outcome: PromotionAttributionOutcome
  detail: string
  created_at: string
}

export interface PromotionCommission {
  id: number
  payment_order_id: number
  user_id: number
  user_email: string
  channel_id: number
  channel_code: string
  channel_name: string
  promoter_id: number
  promoter_name: string
  base_amount: number
  commission_rate: number
  amount: number
  reversed_amount: number
  currency: string
  status: 'frozen' | 'available' | 'settled' | 'reversed'
  frozen_until?: string | null
  settlement_id?: number | null
  created_at: string
}

export interface PromotionSettlement {
  id: number
  promoter_id: number
  promoter_name: string
  period_end: string
  amount: number
  status: 'draft' | 'paid' | 'cancelled'
  notes: string
  paid_at?: string | null
  created_at: string
}

export default {
  listPromoters: () => apiClient.get<PromotionPromoter[]>('/admin/promotion/promoters'),
  createPromoter: (data: PromotionPromoterInput) => apiClient.post<PromotionPromoter>('/admin/promotion/promoters', data),
  updatePromoter: (id: number, data: PromotionPromoterInput) => apiClient.put<PromotionPromoter>(`/admin/promotion/promoters/${id}`, data),
  listChannels: () => apiClient.get<PromotionChannel[]>('/admin/promotion/channels'),
  createChannel: (data: PromotionChannelInput) => apiClient.post<PromotionChannel>('/admin/promotion/channels', data),
  updateChannel: (id: number, data: PromotionChannelInput) => apiClient.put<PromotionChannel>(`/admin/promotion/channels/${id}`, data),
  report: (params?: { start_time?: string; end_time?: string; mode?: 'operation' | 'acquisition' }) => apiClient.get<PromotionReport>('/admin/promotion/report', { params }),
  listAttributionEvents: (params?: { limit?: number }) => apiClient.get<PromotionAttributionEvent[]>('/admin/promotion/attribution-events', { params }),
  listCommissions: (params?: { promoter_id?: number; status?: string; limit?: number }) => apiClient.get<PromotionCommission[]>('/admin/promotion/commissions', { params }),
  listSettlements: (params?: { promoter_id?: number; limit?: number }) => apiClient.get<PromotionSettlement[]>('/admin/promotion/settlements', { params }),
  createSettlement: (data: { promoter_id: number; period_end: string; notes: string }) => apiClient.post<PromotionSettlement>('/admin/promotion/settlements', data),
  updateSettlementStatus: (id: number, status: 'paid' | 'cancelled') => apiClient.put<PromotionSettlement>(`/admin/promotion/settlements/${id}/status`, { status }),
}
