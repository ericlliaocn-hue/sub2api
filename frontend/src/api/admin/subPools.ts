/**
 * Admin Sub-pools API endpoints
 *
 * A sub-pool is an internal isolation unit under a user-visible group: it binds
 * a small number of API keys to a small number of upstream accounts, so one
 * abusive key only burns its own pool and stays attributable.
 */

import { apiClient } from '../client'

export type SubPoolKind = 'formal' | 'probe'
export type SubPoolStatus = 'healthy' | 'cooling' | 'closed'

export interface SubPool {
  id: number
  group_id: number
  name: string
  description: string | null
  kind: SubPoolKind
  status: SubPoolStatus
  key_soft_limit: number
  cooling_until: string | null
  cooling_reason: string | null
  sort_order: number
  account_ids: number[]
  bound_keys: number
  created_at: string
  updated_at: string
}

export interface CreateSubPoolRequest {
  name: string
  description?: string | null
  kind?: SubPoolKind
  status?: SubPoolStatus
  key_soft_limit?: number
  sort_order?: number
  account_ids?: number[]
}

export interface UpdateSubPoolRequest {
  name: string
  description?: string | null
  kind: SubPoolKind
  status: SubPoolStatus
  key_soft_limit?: number
  sort_order?: number
  cooling_until?: string | null
  cooling_reason?: string | null
}

export interface SubPoolAccountKeyUsage {
  api_key_id: number
  user_id: number
  calls: number
  first_call_at: string | null
  last_call_at: string | null
}

export interface AccountTopKeysResponse {
  account_id: number
  start: string
  end: string
  items: SubPoolAccountKeyUsage[]
}

/** List the sub-pools of a group. */
export async function listByGroup(groupId: number): Promise<SubPool[]> {
  const { data } = await apiClient.get<SubPool[]>(`/admin/groups/${groupId}/sub-pools`)
  return data
}

/** Create a sub-pool inside a group. */
export async function create(groupId: number, payload: CreateSubPoolRequest): Promise<SubPool> {
  const { data } = await apiClient.post<SubPool>(`/admin/groups/${groupId}/sub-pools`, payload)
  return data
}

/** Update pool metadata. Account membership is managed by setAccounts. */
export async function update(poolId: number, payload: UpdateSubPoolRequest): Promise<SubPool> {
  const { data } = await apiClient.put<SubPool>(`/admin/sub-pools/${poolId}`, payload)
  return data
}

/** Delete a pool; its keys fall back to whole-group scheduling. */
export async function remove(poolId: number): Promise<void> {
  await apiClient.delete(`/admin/sub-pools/${poolId}`)
}

/** Replace the upstream accounts backing a pool. */
export async function setAccounts(poolId: number, accountIds: number[]): Promise<SubPool> {
  const { data } = await apiClient.put<SubPool>(`/admin/sub-pools/${poolId}/accounts`, {
    account_ids: accountIds
  })
  return data
}

/** Move one API key into this pool. */
export async function bindKey(
  poolId: number,
  apiKeyId: number,
  note?: string
): Promise<{ bound: boolean }> {
  const { data } = await apiClient.post<{ bound: boolean }>(`/admin/sub-pools/${poolId}/keys`, {
    api_key_id: apiKeyId,
    note: note ?? null
  })
  return data
}

/**
 * Drain a burned pool. Suspected keys stay behind: moving them would hand them
 * a fresh set of accounts and detach them from the evidence.
 */
export async function migrateCleanKeys(
  poolId: number,
  reason: string,
  suspectKeyIds: number[] = []
): Promise<{ moved: number }> {
  const { data } = await apiClient.post<{ moved: number }>(`/admin/sub-pools/${poolId}/migrate`, {
    reason,
    suspect_key_ids: suspectKeyIds
  })
  return data
}

/** Attribute an upstream account's traffic to the keys behind it. */
export async function topKeysByAccount(
  accountId: number,
  hours = 24,
  limit = 20
): Promise<AccountTopKeysResponse> {
  const { data } = await apiClient.get<AccountTopKeysResponse>(
    `/admin/accounts/${accountId}/top-keys`,
    { params: { hours, limit } }
  )
  return data
}

export const subPoolsAPI = {
  listByGroup,
  create,
  update,
  remove,
  setAccounts,
  bindKey,
  migrateCleanKeys,
  topKeysByAccount
}

export default subPoolsAPI
