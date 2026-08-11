<template>
  <AppLayout>
    <div class="space-y-6 p-6">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">经营仪表盘</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">收入、上游直接成本、固定/营销/返佣成本统一核算；充值金额不作为消耗收入。金额按系统计费单位展示。</p>
        </div>
        <div class="flex flex-wrap items-end gap-2">
          <label class="field"><span>开始日期</span><input v-model="startDate" class="input" type="date" /></label>
          <label class="field"><span>结束日期</span><input v-model="endDate" class="input" type="date" /></label>
          <button class="btn btn-primary" :disabled="loading" @click="reload">{{ loading ? '加载中…' : '刷新数据' }}</button>
        </div>
      </div>

      <div v-if="report" class="grid gap-4 md:grid-cols-3 xl:grid-cols-6">
        <MetricCard label="消耗收入" :value="money(report.summary.revenue)" />
        <MetricCard label="上游直接成本" :value="money(report.summary.direct_cost)" />
        <MetricCard label="综合总成本" :value="money(report.summary.total_cost)" />
        <MetricCard label="经营利润" :value="money(report.summary.operating_profit)" :danger="report.summary.operating_profit < 0" />
        <MetricCard label="经营利润率" :value="percent(report.summary.operating_margin)" :danger="report.summary.operating_margin < 0" />
        <MetricCard label="综合成本倍率" :value="report.summary.cost_multiplier ? report.summary.cost_multiplier.toFixed(4) + 'x' : '—'" />
      </div>

      <section v-if="report?.trend?.length" class="card overflow-hidden">
        <div class="border-b border-gray-200 p-5 dark:border-dark-700"><h2 class="font-semibold text-gray-900 dark:text-white">时间趋势</h2><p class="mt-1 text-sm text-gray-500">按 UTC 日汇总，成本按当前区间的配置和分摊规则分配。</p></div>
        <div class="overflow-x-auto"><table class="min-w-full text-left text-sm"><thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700"><tr><th class="px-4 py-3">日期</th><th class="px-4 py-3">收入</th><th class="px-4 py-3">直接成本</th><th class="px-4 py-3">总成本</th><th class="px-4 py-3">经营利润</th><th class="px-4 py-3">利润率</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in report.trend" :key="item.date"><td class="px-4 py-3">{{ item.date }}</td><td class="px-4 py-3">{{ money(item.revenue) }}</td><td class="px-4 py-3">{{ money(item.direct_cost) }}</td><td class="px-4 py-3">{{ money(item.total_cost) }}</td><td class="px-4 py-3" :class="item.operating_profit < 0 ? 'text-red-600' : ''">{{ money(item.operating_profit) }}</td><td class="px-4 py-3">{{ percent(item.operating_margin) }}</td></tr></tbody></table></div>
      </section>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(360px,1fr)]">
        <section class="card overflow-hidden">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 p-5 dark:border-dark-700">
            <div><h2 class="font-semibold text-gray-900 dark:text-white">成本与利润报表</h2><p class="mt-1 text-xs text-gray-500">当前按 {{ dimensionLabel }} 聚合</p></div>
            <div class="relative inline-flex items-center">
              <select v-model="dimension" class="input w-auto appearance-none pr-9 leading-5" @change="loadReport"><option value="group">分组</option><option value="channel">渠道</option><option value="model">模型</option><option value="account">账号</option></select>
              <svg class="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
            </div>
          </div>
          <div class="overflow-x-auto">
            <table class="min-w-full text-left text-sm">
              <thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700"><tr><th class="px-4 py-3">对象</th><th class="px-4 py-3">收入</th><th class="px-4 py-3">直接成本</th><th class="px-4 py-3">综合成本</th><th class="px-4 py-3">经营利润</th><th class="px-4 py-3">利润率</th><th class="px-4 py-3">倍率</th></tr></thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="row in report?.rows || []" :key="row.key"><td class="px-4 py-3 font-medium text-gray-900 dark:text-white">{{ row.name }}</td><td class="px-4 py-3">{{ money(row.revenue) }}</td><td class="px-4 py-3">{{ money(row.direct_cost) }}</td><td class="px-4 py-3">{{ money(row.total_cost) }}</td><td class="px-4 py-3" :class="row.operating_profit < 0 ? 'text-red-600' : ''">{{ money(row.operating_profit) }}</td><td class="px-4 py-3">{{ percent(row.operating_margin) }}</td><td class="px-4 py-3">{{ row.cost_multiplier ? row.cost_multiplier.toFixed(4) + 'x' : '—' }}</td></tr>
                <tr v-if="!report?.rows?.length"><td colspan="7" class="px-4 py-10 text-center text-gray-500">当前区间暂无使用数据</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="card p-5">
          <h2 class="font-semibold text-gray-900 dark:text-white">渠道利润与风险预警</h2>
          <div v-if="report?.alerts?.length" class="mt-4 space-y-3">
            <div v-for="alert in report.alerts" :key="`${alert.dimension}-${alert.key}-${alert.reason}`" class="rounded-lg border p-3" :class="alert.severity === 'critical' ? 'border-red-200 bg-red-50 dark:border-red-900/60 dark:bg-red-950/20' : 'border-amber-200 bg-amber-50 dark:border-amber-900/60 dark:bg-amber-950/20'"><div class="flex items-center justify-between gap-2"><span class="font-medium">{{ alert.name }}</span><span class="text-xs uppercase">{{ alert.severity }}</span></div><p class="mt-1 text-sm">{{ alert.reason }}</p><p class="mt-1 text-xs text-gray-500">利润 {{ money(alert.operating_profit) }} · 利润率 {{ percent(alert.operating_margin) }} · 倍率 {{ alert.cost_multiplier ? alert.cost_multiplier.toFixed(4) + 'x' : '—' }}</p></div>
          </div>
          <p v-else class="mt-6 rounded-lg bg-emerald-50 p-4 text-sm text-emerald-700 dark:bg-emerald-950/20 dark:text-emerald-300">当前区间未发现亏损或高成本倍率对象。</p>
          <div class="mt-6 border-t border-gray-200 pt-4 dark:border-dark-700"><h3 class="text-sm font-medium">成本构成</h3><div class="mt-3 space-y-2 text-sm"><div v-for="item in report?.components || []" :key="`${item.source}-${item.name}`" class="flex items-center justify-between gap-3"><span class="truncate text-gray-500">{{ item.name }}</span><span>{{ money(item.amount) }}</span></div><p v-if="!report?.components?.length" class="text-gray-500">暂无成本配置或费用台账</p></div></div>
        </section>
      </div>

      <section v-if="growth" class="card p-5">
        <div class="flex flex-wrap items-center justify-between gap-3"><div><h2 class="font-semibold text-gray-900 dark:text-white">增长与营销分析</h2><p class="mt-1 text-sm text-gray-500">在线人数按最近 15 分钟有使用记录估算。</p></div><div class="grid grid-cols-2 gap-3 text-sm md:grid-cols-5"><span>拉新 <b>{{ growth.new_users }}</b></span><span>活跃 <b>{{ growth.active_users }}</b></span><span>在线 <b>{{ growth.online_users }}</b></span><span>付费 <b>{{ growth.paying_users }}</b></span><span>充值 <b>{{ money(growth.recharge_amount) }}</b></span></div></div>
        <div class="mt-5 grid gap-4 md:grid-cols-4"><MetricCard label="营销成本" :value="money(growth.marketing_cost)" /><MetricCard label="返佣成本" :value="money(growth.affiliate_cost)" /><MetricCard label="CAC" :value="money(growth.cac)" /><MetricCard label="LTV / ROI" :value="`${money(growth.ltv)} / ${percent(growth.roi)}`" /></div>
        <div class="mt-5 overflow-x-auto"><table class="min-w-full text-left text-sm"><thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700"><tr><th class="px-3 py-2">注册来源</th><th class="px-3 py-2">拉新</th><th class="px-3 py-2">活跃</th><th class="px-3 py-2">付费</th><th class="px-3 py-2">消耗收入</th><th class="px-3 py-2">充值</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in growth.by_source" :key="item.source"><td class="px-3 py-2">{{ item.source }}</td><td class="px-3 py-2">{{ item.new_users }}</td><td class="px-3 py-2">{{ item.active_users }}</td><td class="px-3 py-2">{{ item.paying_users }}</td><td class="px-3 py-2">{{ money(item.revenue) }}</td><td class="px-3 py-2">{{ money(item.recharge) }}</td></tr></tbody></table></div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import businessFinanceAPI, { type FinanceGrowthReport, type FinanceReport } from '@/api/admin/businessFinance'

const appStore = useAppStore()
const loading = ref(false)
const report = ref<FinanceReport | null>(null)
const growth = ref<FinanceGrowthReport | null>(null)
const dimension = ref('channel')
const endDate = ref(new Date().toISOString().slice(0, 10))
const start = new Date()
start.setDate(start.getDate() - 30)
const startDate = ref(start.toISOString().slice(0, 10))
const dimensionLabel = computed(() => ({ group: '分组', channel: '渠道', model: '模型', account: '账号' })[dimension.value] || dimension.value)

function rangeParams() { return { start_time: new Date(`${startDate.value}T00:00:00`).toISOString(), end_time: new Date(`${endDate.value}T23:59:59.999`).toISOString() } }
async function loadReport() { const { data } = await businessFinanceAPI.getReport({ ...rangeParams(), dimension: dimension.value }); report.value = data }
async function loadGrowth() { const { data } = await businessFinanceAPI.getGrowth(rangeParams()); growth.value = data }
async function reload() { loading.value = true; try { await Promise.all([loadReport(), loadGrowth()]) } catch (error) { appStore.showError(error instanceof Error ? error.message : '经营数据加载失败') } finally { loading.value = false } }
function money(value: number) { return (value || 0).toFixed(4) }
function percent(value: number) { return `${((value || 0) * 100).toFixed(2)}%` }
onMounted(reload)
</script>

<script lang="ts">
import { defineComponent, h } from 'vue'
export default defineComponent({
  components: {
    MetricCard: defineComponent({ props: { label: String, value: String, danger: Boolean }, setup(props) { return () => h('div', { class: 'card p-4' }, [h('div', { class: 'text-xs text-gray-500' }, props.label), h('div', { class: ['mt-2 text-lg font-semibold', props.danger ? 'text-red-600' : 'text-gray-900 dark:text-white'] }, props.value)]) } }),
  },
})
</script>
