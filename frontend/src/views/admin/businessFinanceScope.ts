import type { BusinessExpense, ExpenseRecoup } from '@/api/admin/businessFinance'

export type FinanceScopeFields = {
  accountId: number | null
  groupId: number | null
  channelId: number | null
  model: string
}

export function emptyFinanceScopeFields(): FinanceScopeFields {
  return { accountId: null, groupId: null, channelId: null, model: '' }
}

export function buildFinanceScope(fields: FinanceScopeFields): Record<string, unknown> {
  const scope: Record<string, unknown> = {}
  if (fields.accountId && fields.accountId > 0) scope.account_id = fields.accountId
  if (fields.groupId && fields.groupId > 0) scope.group_id = fields.groupId
  if (fields.channelId && fields.channelId > 0) scope.channel_id = fields.channelId
  const model = fields.model.trim()
  if (model) scope.model = model
  return scope
}

export function readFinanceScope(scope?: Record<string, unknown> | null): FinanceScopeFields {
  return {
    accountId: asPositiveInt(scope?.account_id),
    groupId: asPositiveInt(scope?.group_id),
    channelId: asPositiveInt(scope?.channel_id),
    model: typeof scope?.model === 'string' ? scope.model : '',
  }
}

export function asPositiveInt(value: unknown): number | null {
  if (typeof value === 'number' && Number.isInteger(value) && value > 0) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isInteger(parsed) && parsed > 0) return parsed
  }
  return null
}

export function summarizeExpenseRecoup(items: BusinessExpense[]) {
  let cost = 0
  let billed = 0
  let recouped = 0
  let short = 0
  let bound = 0
  for (const item of items) {
    if (item.status !== 'active' || !item.recoup) continue
    bound += 1
    cost += item.recoup.cost
    billed += item.recoup.billed
    if (item.recoup.profit >= 0) recouped += 1
    else short += 1
  }
  return { cost, billed, profit: billed - cost, recouped, short, bound }
}

export function recoupProgress(recoup?: ExpenseRecoup | null): number {
  if (!recoup || recoup.cost <= 0) return 0
  return Math.min(100, (recoup.billed / recoup.cost) * 100)
}

export function formatScopeLabel(
  scope: Record<string, unknown> | undefined,
  names: { account?: string; group?: string; channel?: string },
): string {
  const fields = readFinanceScope(scope)
  const parts: string[] = []
  if (fields.accountId) parts.push(names.account || `账号 #${fields.accountId}`)
  if (fields.groupId) parts.push(names.group || `分组 #${fields.groupId}`)
  if (fields.channelId) parts.push(names.channel || `渠道 #${fields.channelId}`)
  if (fields.model) parts.push(fields.model)
  return parts.length ? parts.join(' · ') : '全局'
}
