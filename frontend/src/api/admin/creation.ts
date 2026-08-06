import { apiClient, buildApiUrl } from '@/api/client'
import type { CreationSettings, CreationHistoryAsset } from '@/api/creationStudio'

export interface AdminCreationTask {
  id: number
  user_id: number
  user_email: string
  api_key_id: number
  kind: 'image' | 'video'
  model: string
  prompt: string
  status: string
  provider_task_id?: string
  error_message?: string
  created_at: string
  finished_at?: string
  assets: CreationHistoryAsset[]
}

export interface AdminCreationTaskPage {
  items: AdminCreationTask[]
  pagination: {
    page: number
    page_size: number
    total: number
    total_pages: number
  }
}

export async function getCreationSettings(): Promise<CreationSettings> {
  const { data } = await apiClient.get<CreationSettings>('/admin/settings/creation')
  return data
}

export async function updateCreationSettings(settings: CreationSettings): Promise<CreationSettings> {
  const { data } = await apiClient.put<CreationSettings>('/admin/settings/creation', settings)
  return data
}

export async function listAdminCreationTasks(page = 1, pageSize = 20): Promise<AdminCreationTaskPage> {
  const { data } = await apiClient.get<AdminCreationTaskPage>('/admin/creation/tasks', {
    params: { page, page_size: pageSize },
  })
  return data
}

export function adminCreationAssetUrl(assetId: number): string {
  return buildApiUrl(`/admin/creation/assets/${assetId}/content`)
}

export async function loadAdminCreationAsset(assetId: number): Promise<string> {
  const { data } = await apiClient.get<Blob>(`/admin/creation/assets/${assetId}/content`, {
    responseType: 'blob',
    timeout: 120000,
  })
  return URL.createObjectURL(data)
}
