<template>
  <div class="space-y-4" data-testid="profit-calendar">
    <div class="flex flex-wrap items-end gap-3">
      <label class="field">
        <span>开始</span>
        <input :value="startDate" class="input" type="date" data-testid="calendar-start" @change="emitDate('start', $event)" />
      </label>
      <label class="field">
        <span>结束</span>
        <input :value="endDate" class="input" type="date" data-testid="calendar-end" @change="emitDate('end', $event)" />
      </label>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn btn-secondary btn-sm" data-testid="calendar-preset-2d" @click="$emit('preset', '2d')">前天+昨天</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="$emit('preset', '7d')">近 7 天</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="$emit('preset', 'month')">本月</button>
        <button type="button" class="btn btn-secondary" data-testid="calendar-filter" @click="$emit('reload')">查询</button>
      </div>
    </div>

    <div v-if="calendar" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5" data-testid="calendar-summary">
      <div class="rounded-xl border border-gray-200/80 px-3 py-2.5 dark:border-dark-700">
        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-400">成本</div>
        <div class="mt-1 text-lg font-semibold tabular-nums">{{ money(calendar.summary.cost) }}</div>
      </div>
      <div class="rounded-xl border border-gray-200/80 px-3 py-2.5 dark:border-dark-700">
        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-400">官方扣费</div>
        <div class="mt-1 text-lg font-semibold tabular-nums">{{ money(calendar.summary.official_billing) }}</div>
      </div>
      <div class="rounded-xl border border-gray-200/80 px-3 py-2.5 dark:border-dark-700">
        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-400">用户扣费 / 收回</div>
        <div class="mt-1 text-lg font-semibold tabular-nums">{{ money(calendar.summary.user_billing) }}</div>
      </div>
      <div class="rounded-xl border border-gray-200/80 px-3 py-2.5 dark:border-dark-700">
        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-400">利润</div>
        <div class="mt-1 text-lg font-semibold tabular-nums" :class="profitClass(calendar.summary.profit)">{{ money(calendar.summary.profit) }}</div>
      </div>
      <div class="rounded-xl border border-gray-200/80 px-3 py-2.5 dark:border-dark-700">
        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-400">账号</div>
        <div class="mt-1 text-lg font-semibold tabular-nums">{{ calendar.summary.accounts }}</div>
        <div class="mt-0.5 text-xs text-gray-500">过本 {{ calendar.summary.recouped }} · 未过本 {{ calendar.summary.short }} · 未记 {{ calendar.summary.unrecorded }}</div>
      </div>
    </div>

    <div v-for="day in calendar?.days || []" :key="day.date" class="card overflow-hidden" :data-testid="`calendar-day-${day.date}`">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
        <div class="font-semibold text-gray-900 dark:text-white">{{ day.date }}</div>
        <div class="text-sm text-gray-500">
          成本 {{ money(day.cost) }} · 收回 {{ money(day.user_billing) }} ·
          <span :class="profitClass(day.profit)">利润 {{ money(day.profit) }}</span>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="min-w-full text-left text-sm">
          <thead class="border-b border-gray-200 text-xs uppercase text-gray-500 dark:border-dark-700">
            <tr>
              <th class="px-3 py-3">账号</th>
              <th class="px-3 py-3">导入时间</th>
              <th class="px-3 py-3">成本</th>
              <th class="px-3 py-3">收回</th>
              <th class="px-3 py-3">官方 Token</th>
              <th class="px-3 py-3">官方扣费</th>
              <th class="px-3 py-3">用户扣费</th>
              <th class="px-3 py-3">利润</th>
              <th class="px-3 py-3">状态</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="row in day.rows" :key="row.account_ids.join('-')">
              <td class="px-3 py-3">
                <div class="font-medium text-gray-900 dark:text-white">{{ row.account_name }}</div>
                <div class="text-xs text-gray-400">{{ row.account_ids.join(' / ') }}</div>
              </td>
              <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ formatDate(row.imported_at) }}</td>
              <td class="px-3 py-3 tabular-nums">
                <span v-if="row.cost_recorded">{{ money(row.cost) }}</span>
                <span v-else class="text-amber-600">未记</span>
              </td>
              <td class="px-3 py-3 tabular-nums">{{ money(row.user_billing) }}</td>
              <td class="px-3 py-3 tabular-nums text-gray-600 dark:text-gray-300">{{ formatTokensK(row.official_tokens) }}</td>
              <td class="px-3 py-3 tabular-nums">{{ money(row.official_billing) }}</td>
              <td class="px-3 py-3 tabular-nums">{{ money(row.user_billing) }}</td>
              <td class="px-3 py-3 tabular-nums font-medium" :class="profitClass(row.profit)">{{ money(row.profit) }}</td>
              <td class="px-3 py-3">
                <span class="badge" :class="statusClass(row)">{{ statusLabel(row) }}</span>
              </td>
            </tr>
            <tr v-if="!day.rows.length">
              <td colspan="9" class="px-3 py-6 text-center text-sm text-gray-400">这天没有导入账号</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ProfitCalendar, ProfitCalendarRow } from '@/api/admin/businessFinance'
import { formatTokensK } from '@/utils/format'

defineProps<{
  calendar: ProfitCalendar | null
  startDate: string
  endDate: string
}>()

const emit = defineEmits<{
  reload: []
  preset: [value: '2d' | '7d' | 'month']
  update: [field: 'start' | 'end', value: string]
}>()

function emitDate(field: 'start' | 'end', event: Event) {
  const value = (event.target as HTMLInputElement).value
  emit('update', field, value)
}

function money(value: number) {
  return (value || 0).toFixed(2)
}

function profitClass(value: number) {
  return value < 0 ? 'text-red-600' : 'text-emerald-600'
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function statusLabel(row: ProfitCalendarRow) {
  if (row.schedulable) return '在跑'
  if (row.status === 'active') return '停调度'
  if (row.status === 'error') return 'error'
  return row.status || '—'
}

function statusClass(row: ProfitCalendarRow) {
  if (row.schedulable) return 'badge-success'
  if (row.status === 'error') return 'badge-secondary'
  return 'badge-secondary'
}
</script>

<style scoped>
.field { display: flex; flex-direction: column; gap: 0.4rem; font-size: 0.875rem; color: rgb(107 114 128); }
.field span { font-weight: 500; }
.btn-sm { padding: 0.35rem 0.65rem; font-size: 0.75rem; }
.badge-success { color: rgb(22 101 52); background: rgb(220 252 231); }
.badge-secondary { color: rgb(75 85 99); background: rgb(243 244 246); }
</style>
