import type { SubPool, SubPoolGroupKey } from '@/api/admin/subPools'
import type { Account } from '@/types'

export interface SubPoolRoutingRow {
  api_key_id: number
  name: string
  user_id: number
  user_label: string
  status: string
  pool_id: number | null
  pool_name: string
  reachable_names: string[]
  user_default_sub_pool_id: number | null
}

export interface SubPoolUserRow {
  user_id: number
  user_label: string
  pool_id: number | null
  pool_name: string
  pinned: boolean
  mixed: boolean
  key_count: number
  reachable_names: string[]
  keys: SubPoolRoutingRow[]
}

export function isAccountReachable(account: Account | undefined): boolean {
  return !!account && account.status === 'active' && account.schedulable === true
}

export function accountsInPool(pool: SubPool, accounts: Account[]): Account[] {
  const byId = new Map(accounts.map((account) => [account.id, account]))
  return pool.account_ids.map((id) => byId.get(id)).filter((account): account is Account => !!account)
}

export function reachableAccounts(pool: SubPool | undefined, accounts: Account[]): Account[] {
  if (!pool) {
    return accounts.filter(isAccountReachable)
  }
  return accountsInPool(pool, accounts).filter(isAccountReachable)
}

export function keyUserLabel(key: SubPoolGroupKey): string {
  return key.user_email || key.user_username || `u${key.user_id}`
}

export function buildRoutingRows(
  keys: SubPoolGroupKey[],
  pools: SubPool[],
  accounts: Account[],
  wholeGroupLabel: string
): SubPoolRoutingRow[] {
  const poolById = new Map(pools.map((pool) => [pool.id, pool]))
  return keys.map((key) => {
    const pool = key.sub_pool_id ? poolById.get(key.sub_pool_id) : undefined
    return {
      api_key_id: key.api_key_id,
      name: key.name,
      user_id: key.user_id,
      user_label: keyUserLabel(key),
      status: key.status,
      pool_id: key.sub_pool_id,
      pool_name: pool?.name ?? wholeGroupLabel,
      user_default_sub_pool_id: key.user_default_sub_pool_id,
      reachable_names: key.sub_pool_id
        ? pool
          ? reachableAccounts(pool, accounts).map((account) => account.name)
          : []
        : reachableAccounts(undefined, accounts).map((account) => account.name)
    }
  })
}

export function buildUserRoutingRows(rows: SubPoolRoutingRow[]): SubPoolUserRow[] {
  const byUser = new Map<number, SubPoolUserRow>()
  for (const row of rows) {
    const existing = byUser.get(row.user_id)
    if (existing) {
      existing.keys.push(row)
      existing.key_count += 1
      if (row.pool_id !== existing.keys[0]?.pool_id) {
        existing.mixed = true
      }
      continue
    }
    byUser.set(row.user_id, {
      user_id: row.user_id,
      user_label: row.user_label,
      pool_id: row.user_default_sub_pool_id ?? row.pool_id,
      pool_name: row.pool_name,
      pinned: row.user_default_sub_pool_id != null,
      mixed: false,
      key_count: 1,
      reachable_names: row.reachable_names,
      keys: [row]
    })
  }
  return [...byUser.values()]
}

export function usersInPool(users: SubPoolUserRow[], poolId: number): SubPoolUserRow[] {
  return users.filter(
    (user) => user.pool_id === poolId || user.keys.some((key) => key.pool_id === poolId)
  )
}
