import { apiClient } from '../client'

export interface PromotionPromoter { id:number; name:string; contact:string; commission_rate:number; enabled:boolean; notes:string }
export interface PromotionChannel { id:number; code:string; name:string; channel_type:string; promoter_id?:number|null; promoter_name?:string; enabled:boolean; notes:string }
export interface PromotionReportRow { channel_id:number; code:string; name:string; channel_type:string; promoter_name:string; new_users:number; paying_users:number; active_users:number; recharge:number; revenue:number; marketing_cost:number; profit:number; cac:number; roi:number }
export interface PromotionReport { start_time:string; end_time:string; rows:PromotionReportRow[] }
export default {
  listPromoters: () => apiClient.get<PromotionPromoter[]>('/admin/promotion/promoters'),
  createPromoter: (data: Omit<PromotionPromoter,'id'>) => apiClient.post<PromotionPromoter>('/admin/promotion/promoters', data),
  updatePromoter: (id:number,data:Omit<PromotionPromoter,'id'>) => apiClient.put<PromotionPromoter>(`/admin/promotion/promoters/${id}`, data),
  listChannels: () => apiClient.get<PromotionChannel[]>('/admin/promotion/channels'),
  createChannel: (data: Omit<PromotionChannel,'id'>) => apiClient.post<PromotionChannel>('/admin/promotion/channels', data),
  updateChannel: (id:number,data:Omit<PromotionChannel,'id'>) => apiClient.put<PromotionChannel>(`/admin/promotion/channels/${id}`, data),
  report: (params?:{start_time?:string;end_time?:string}) => apiClient.get<PromotionReport>('/admin/promotion/report',{params}),
}
