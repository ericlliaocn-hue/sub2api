<template>
  <div class="overflow-x-auto">
    <table class="min-w-full whitespace-nowrap text-left text-sm">
      <thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700">
        <tr>
          <th class="px-4 py-2">{{ keyLabel }}</th>
          <th class="px-4 py-2">访问</th>
          <th class="px-4 py-2">注册</th>
          <th class="px-4 py-2">访问→注册</th>
          <th class="px-4 py-2">付费</th>
          <th class="px-4 py-2">收入</th>
          <th v-if="extraLabel" class="px-4 py-2">{{ extraLabel }}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
        <tr v-for="row in rows" :key="row.key">
          <td class="max-w-[280px] truncate px-4 py-2 font-mono text-xs" :title="row.label">{{ row.label || row.key }}</td>
          <td class="px-4 py-2">{{ row.visits }}</td>
          <td class="px-4 py-2 font-semibold">{{ row.new_users }}</td>
          <td class="px-4 py-2">{{ row.visits ? percent(row.new_users / row.visits) : '—' }}</td>
          <td class="px-4 py-2">{{ row.paying_users }}</td>
          <td class="px-4 py-2">{{ money(row.revenue) }}</td>
          <td v-if="extraLabel" class="px-4 py-2">{{ money(row.extra) }}</td>
        </tr>
        <tr v-if="!rows.length">
          <td :colspan="extraLabel ? 7 : 6" class="px-4 py-6 text-center text-xs text-gray-500">{{ empty }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import type { PromotionBreakdownRow } from '@/api/admin/promotion'

withDefaults(defineProps<{ rows: PromotionBreakdownRow[]; keyLabel: string; extraLabel?: string; empty?: string }>(), {
  extraLabel: '',
  empty: '暂无数据'
})

function money(value: number) {
  return (Number(value) || 0).toFixed(4)
}
function percent(value: number) {
  return `${((Number(value) || 0) * 100).toFixed(2)}%`
}
</script>
