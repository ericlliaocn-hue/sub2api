/** 获客分类展示：官网 / 搜索 / 邀请 / 其他。推广报表与财务增长表共用。 */

const LABELS: Record<string, string> = {
  official: '官网直接',
  seo: '搜索 SEO',
  invite: '邀请',
  other: '其他渠道'
}

const BADGES: Record<string, string> = {
  official: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
  seo: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
  invite: 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300',
  other: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
}

export function acquisitionClassLabel(value: string): string {
  return LABELS[value] ?? value
}

export function acquisitionClassBadge(value: string): string {
  return BADGES[value] ?? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}
