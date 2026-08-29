import { apiClient } from '../client'

export type UpstreamConnectionType = 'sub2api' | 'newapi' | 'other'

export interface UpstreamConnection {
  id: number
  name: string
  type: UpstreamConnectionType
  base_url: string
  username: string
  has_password: boolean
  status: 'untested' | 'healthy' | 'error' | string
  last_test_at?: string
  last_error?: string
  created_at: string
  updated_at: string
}

export interface UpstreamRemoteAccount {
  id: string
  name: string
  platform?: string
  type?: string
  status?: string
  schedulable?: boolean
  concurrency?: number
  priority?: number
  balance?: number
  rate_multiplier?: number
  last_used_at?: string
  error_message?: string
}

export interface UpstreamRemoteGroup {
  id: string
  name: string
  platform?: string
  status?: string
  rate_multiplier?: number
  account_count?: number
}

export interface UpstreamConnectionSnapshot {
  connection: UpstreamConnection
  remote_user?: Record<string, unknown>
  accounts: UpstreamRemoteAccount[]
  groups: UpstreamRemoteGroup[]
  tested_at: string
}

export interface UpstreamConnectionInput {
  name: string
  type: UpstreamConnectionType
  base_url: string
  username: string
  password: string
}

async function list(): Promise<UpstreamConnection[]> {
  const { data } = await apiClient.get<UpstreamConnection[]>('/admin/upstream-connections')
  return data
}

async function create(input: UpstreamConnectionInput): Promise<UpstreamConnection> {
  const { data } = await apiClient.post<UpstreamConnection>('/admin/upstream-connections', input)
  return data
}

async function update(id: number, input: UpstreamConnectionInput): Promise<UpstreamConnection> {
  const { data } = await apiClient.put<UpstreamConnection>(`/admin/upstream-connections/${id}`, input)
  return data
}

async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/upstream-connections/${id}`)
}

async function test(id: number): Promise<UpstreamConnectionSnapshot> {
  const { data } = await apiClient.post<UpstreamConnectionSnapshot>(`/admin/upstream-connections/${id}/test`)
  return data
}

export default { list, create, update, remove, test }
