import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface WatchedTrafficConfig {
  enabled: boolean
  user_ids: number[]
}

export interface UpdateWatchedTrafficConfig {
  enabled?: boolean
  user_ids?: number[]
}

export interface WatchedTrafficLog {
  id: number
  request_id: string
  user_id: number
  user_email: string
  api_key_id?: number | null
  api_key_name: string
  group_id?: number | null
  group_name: string
  account_id?: number | null
  model: string
  endpoint: string
  status_code: number
  prompt_text: string
  response_text: string
  error_text: string
  created_at: string
}

export interface ListWatchedTrafficLogsParams {
  user_id?: number
  page?: number
  page_size?: number
}

export async function getWatchedTrafficConfig(): Promise<WatchedTrafficConfig> {
  const { data } = await apiClient.get<WatchedTrafficConfig>('/admin/watched-traffic/config')
  return data
}

export async function updateWatchedTrafficConfig(
  payload: UpdateWatchedTrafficConfig
): Promise<WatchedTrafficConfig> {
  const { data } = await apiClient.put<WatchedTrafficConfig>('/admin/watched-traffic/config', payload)
  return data
}

export async function listWatchedTrafficLogs(
  params: ListWatchedTrafficLogsParams = {}
): Promise<PaginatedResponse<WatchedTrafficLog>> {
  const { data } = await apiClient.get<PaginatedResponse<WatchedTrafficLog>>('/admin/watched-traffic/logs', {
    params,
  })
  return data
}

export const watchedTrafficAPI = {
  getConfig: getWatchedTrafficConfig,
  updateConfig: updateWatchedTrafficConfig,
  listLogs: listWatchedTrafficLogs,
}

export default watchedTrafficAPI
