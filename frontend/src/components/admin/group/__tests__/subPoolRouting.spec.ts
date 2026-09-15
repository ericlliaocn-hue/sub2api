import { describe, expect, it } from 'vitest'
import type { SubPool, SubPoolGroupKey } from '@/api/admin/subPools'
import type { Account } from '@/types'
import { buildRoutingRows, buildUserRoutingRows, isAccountReachable } from '../subPoolRouting'

const account = (partial: Partial<Account> & Pick<Account, 'id' | 'name'>): Account =>
  ({
    status: 'active',
    schedulable: true,
    type: 'apikey',
    platform: 'openai',
    ...partial
  }) as Account

const pool = (partial: Partial<SubPool> & Pick<SubPool, 'id' | 'name' | 'account_ids'>): SubPool =>
  ({
    group_id: 3,
    description: null,
    kind: 'formal',
    status: 'healthy',
    key_soft_limit: 8,
    cooling_until: null,
    cooling_reason: null,
    sort_order: 0,
    bound_keys: 0,
    created_at: '',
    updated_at: '',
    ...partial
  })

const key = (partial: Partial<SubPoolGroupKey> & Pick<SubPoolGroupKey, 'api_key_id'>): SubPoolGroupKey => ({
  name: 'sol',
  user_id: 1,
  user_email: 'a@b.c',
  user_username: 'a',
  status: 'active',
  sub_pool_id: null,
  user_default_sub_pool_id: null,
  ...partial
})

describe('subPoolRouting', () => {
  it('only treats active schedulable accounts as reachable', () => {
    expect(isAccountReachable(account({ id: 1, name: 'on' }))).toBe(true)
    expect(isAccountReachable(account({ id: 2, name: 'off', schedulable: false }))).toBe(false)
    expect(isAccountReachable(account({ id: 3, name: 'err', status: 'error' }))).toBe(false)
  })

  it('maps bound keys to that pool’s reachable accounts only', () => {
    const rows = buildRoutingRows(
      [key({ api_key_id: 154, user_email: '145@qq.com', sub_pool_id: 2 })],
      [
        pool({ id: 1, name: '正池', account_ids: [349] }),
        pool({ id: 2, name: '观察池', kind: 'probe', account_ids: [34, 27] })
      ],
      [
        account({ id: 349, name: 'zbj9786' }),
        account({ id: 34, name: 'wdai(0.12)' }),
        account({ id: 27, name: 'inwo', schedulable: false })
      ],
      '整组'
    )
    expect(rows).toHaveLength(1)
    expect(rows[0].pool_name).toBe('观察池')
    expect(rows[0].reachable_names).toEqual(['wdai(0.12)'])
  })

  it('keeps unbound keys on the whole group', () => {
    const rows = buildRoutingRows(
      [key({ api_key_id: 155, sub_pool_id: null })],
      [pool({ id: 1, name: '正池', account_ids: [349] })],
      [account({ id: 349, name: 'zbj9786' }), account({ id: 34, name: 'wdai(0.12)' })],
      '整组'
    )
    expect(rows[0].pool_name).toBe('整组')
    expect(rows[0].reachable_names).toEqual(['zbj9786', 'wdai(0.12)'])
  })

  it('groups keys by user for the ops board', () => {
    const rows = buildRoutingRows(
      [
        key({ api_key_id: 154, user_id: 35, user_email: '145@qq.com', sub_pool_id: 2, user_default_sub_pool_id: 2 }),
        key({ api_key_id: 468, user_id: 35, user_email: '145@qq.com', name: 'sol2', sub_pool_id: 2, user_default_sub_pool_id: 2 })
      ],
      [pool({ id: 2, name: '观察池', kind: 'probe', account_ids: [34] })],
      [account({ id: 34, name: 'wdai(0.12)' })],
      '整组'
    )
    const users = buildUserRoutingRows(rows)
    expect(users).toHaveLength(1)
    expect(users[0].key_count).toBe(2)
    expect(users[0].pinned).toBe(true)
    expect(users[0].mixed).toBe(false)
    expect(users[0].pool_name).toBe('观察池')
  })
})
