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

export interface SubPoolGroupKey {
  api_key_id: number
  name: string
  user_id: number
  user_email: string
  user_username: string
  status: string
  sub_pool_id: number | null
  user_default_sub_pool_id: number | null
}

export interface SubPoolUserDefault {
  user_id: number
  user_email: string
  user_username: string
  sub_pool_id: number
  operator: string
  note: string | null
  updated_at: string
}

export interface SubPoolPlacementPolicy {
  default_sub_pool_id: number | null
  users: SubPoolUserDefault[]
}

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

/** Admin routing board: user/key → pool. Never includes the key secret. */
export async function listGroupKeys(groupId: number): Promise<SubPoolGroupKey[]> {
  const { data } = await apiClient.get<SubPoolGroupKey[]>(`/admin/groups/${groupId}/sub-pool-keys`)
  return data
}

export async function getPlacementPolicy(groupId: number): Promise<SubPoolPlacementPolicy> {
  const { data } = await apiClient.get<SubPoolPlacementPolicy>(
    `/admin/groups/${groupId}/sub-pool-policy`
  )
  return data
}

export async function setGroupDefaultPool(
  groupId: number,
  subPoolId: number | null
): Promise<{ default_sub_pool_id: number | null }> {
  const { data } = await apiClient.put<{ default_sub_pool_id: number | null }>(
    `/admin/groups/${groupId}/default-sub-pool`,
    { sub_pool_id: subPoolId }
  )
  return data
}

export async function setUserDefaultPool(
  groupId: number,
  userId: number,
  subPoolId: number,
  note?: string
): Promise<{ bound: boolean }> {
  const { data } = await apiClient.put<{ bound: boolean }>(
    `/admin/groups/${groupId}/sub-pool-users`,
    { user_id: userId, sub_pool_id: subPoolId, note: note ?? null }
  )
  return data
}

export async function clearUserDefaultPool(
  groupId: number,
  userId: number
): Promise<{ cleared: boolean }> {
  const { data } = await apiClient.delete<{ cleared: boolean }>(
    `/admin/groups/${groupId}/sub-pool-users/${userId}`
  )
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

/**
 * Rules a key must satisfy to leave the probe pool. Global rather than
 * per-group: probation is about the key's own behaviour, not the product tier.
 */
export interface SubPoolGraduationPolicy {
  enabled: boolean
  probation_days: number
  /** Peak calls on any single day of probation. 0 disables the check. */
  max_daily_calls: number
}

export async function getGraduationPolicy(): Promise<SubPoolGraduationPolicy> {
  const { data } = await apiClient.get<SubPoolGraduationPolicy>('/admin/sub-pools/graduation')
  return data
}

export async function updateGraduationPolicy(
  payload: SubPoolGraduationPolicy
): Promise<SubPoolGraduationPolicy> {
  const { data } = await apiClient.put<SubPoolGraduationPolicy>(
    '/admin/sub-pools/graduation',
    payload
  )
  return data
}

/** Run one sweep now instead of waiting for the 10-minute ticker. */
export async function runGraduation(): Promise<{ graduated: number }> {
  const { data } = await apiClient.post<{ graduated: number }>('/admin/sub-pools/graduation/run')
  return data
}

/** One row of the "who burned this pool" incident report. */
export interface SubPoolKeyAttribution {
  api_key_id: number
  user_id: number
  calls: number
  /** Fraction of the pool's traffic in the window, 0..1. */
  share: number
  violations: number
  suspect: boolean
  suspect_why?: string
  first_call_at: string | null
  last_call_at: string | null
  bound_at: string | null
}

export interface SubPoolAttributionReport {
  sub_pool_id: number
  status: SubPoolStatus
  start: string
  end: string
  total_calls: number
  keys: SubPoolKeyAttribution[]
}

export interface SubPoolCoolingResult {
  cooled: number
  recovered: number
  migrated: number
}

export async function attribution(
  poolId: number,
  hours = 24
): Promise<SubPoolAttributionReport> {
  const { data } = await apiClient.get<SubPoolAttributionReport>(
    `/admin/sub-pools/${poolId}/attribution`,
    { params: { hours } }
  )
  return data
}

/** Push a key back into the probe pool: it keeps working, on disposable accounts. */
export async function demoteKey(keyId: number, note?: string): Promise<{ demoted: boolean }> {
  const { data } = await apiClient.post<{ demoted: boolean }>(
    `/admin/sub-pool-keys/${keyId}/demote`,
    { note: note ?? null }
  )
  return data
}

/** Stop the key from authenticating. The pool binding is kept for the record. */
export async function disableKey(keyId: number): Promise<{ disabled: boolean }> {
  const { data } = await apiClient.post<{ disabled: boolean }>(
    `/admin/sub-pool-keys/${keyId}/disable`
  )
  return data
}

/** Run one cooling sweep now instead of waiting for the 2-minute ticker. */
export async function runCooling(): Promise<SubPoolCoolingResult> {
  const { data } = await apiClient.post<SubPoolCoolingResult>('/admin/sub-pools/cooling/run')
  return data
}

/** Scoring thresholds that drive automatic key sanctions. */
export interface ReputationPolicy {
  enabled: boolean
  window_days: number
  demote_below: number
  ban_below: number
}

export interface APIKeyReputation {
  api_key_id: number
  score: number
  severe_hits: number
  total_hits: number
  last_event_at: string | null
  scored_at: string
  sanction: 'none' | 'demoted' | 'disabled'
  sanctioned_at: string | null
  sanction_reason: string | null
}

export interface ReputationSweepResult {
  scored: number
  demoted: number
  disabled: number
}

export async function getReputationPolicy(): Promise<ReputationPolicy> {
  const { data } = await apiClient.get<ReputationPolicy>('/admin/sub-pools/reputation')
  return data
}

export async function updateReputationPolicy(policy: ReputationPolicy): Promise<ReputationPolicy> {
  const { data } = await apiClient.put<ReputationPolicy>('/admin/sub-pools/reputation', policy)
  return data
}

/** Run one scoring sweep now instead of waiting for the 5-minute ticker. */
export async function runReputation(): Promise<ReputationSweepResult> {
  const { data } = await apiClient.post<ReputationSweepResult>('/admin/sub-pools/reputation/run')
  return data
}

/** The lowest-scoring keys across all groups. */
export async function worstReputations(limit = 50): Promise<{ items: APIKeyReputation[] }> {
  const { data } = await apiClient.get<{ items: APIKeyReputation[] }>(
    '/admin/sub-pools/reputation/worst',
    { params: { limit } }
  )
  return data
}

/** Overturn an automatic sanction. Does not re-enable the key by itself. */
export async function clearReputationSanction(keyId: number): Promise<{ cleared: boolean }> {
  const { data } = await apiClient.post<{ cleared: boolean }>(
    `/admin/sub-pool-keys/${keyId}/reputation/clear`
  )
  return data
}

export const subPoolsAPI = {
  listGroupKeys,
  getPlacementPolicy,
  setGroupDefaultPool,
  setUserDefaultPool,
  clearUserDefaultPool,
  getReputationPolicy,
  updateReputationPolicy,
  runReputation,
  worstReputations,
  clearReputationSanction,
  attribution,
  demoteKey,
  disableKey,
  runCooling,
  listByGroup,
  create,
  update,
  remove,
  setAccounts,
  bindKey,
  migrateCleanKeys,
  topKeysByAccount,
  getGraduationPolicy,
  updateGraduationPolicy,
  runGraduation
}

export default subPoolsAPI
