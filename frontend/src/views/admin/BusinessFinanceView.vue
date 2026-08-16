<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">经营管理</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            维护周期成本和费用台账，经营仪表盘会按用量、上游成本和分摊规则生成利润报表。
          </p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="reload">
          {{ loading ? '刷新中…' : '刷新' }}
        </button>
      </div>

      <div class="flex gap-2 border-b border-gray-200 dark:border-dark-700">
        <button
          class="border-b-2 px-4 py-3 text-sm font-medium"
          :class="activeTab === 'configs' ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500'"
          @click="activeTab = 'configs'"
        >
          成本配置
        </button>
        <button
          class="border-b-2 px-4 py-3 text-sm font-medium"
          :class="activeTab === 'expenses' ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500'"
          @click="activeTab = 'expenses'"
        >
          费用台账
        </button>
      </div>

      <section v-if="activeTab === 'configs'" class="space-y-6">
        <div class="card p-6">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 class="font-semibold text-gray-900 dark:text-white">成本配置</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">适合域名、服务器等周期成本或预付费用摊销；按日、月或年自动计入报表。</p>
            </div>
            <button class="btn btn-primary" @click="startNewConfig">新增配置</button>
          </div>

          <div class="overflow-x-auto">
            <table class="min-w-full text-left text-sm">
              <thead class="border-b border-gray-200 text-xs uppercase text-gray-500 dark:border-dark-700">
                <tr>
                  <th class="px-3 py-3">名称</th>
                  <th class="px-3 py-3">类别</th>
                  <th class="px-3 py-3">金额</th>
                  <th class="px-3 py-3">折算率</th>
                  <th class="px-3 py-3">分摊方式</th>
                  <th class="px-3 py-3">作用范围</th>
                  <th class="px-3 py-3">生效时间</th>
                  <th class="px-3 py-3">状态</th>
                  <th class="px-3 py-3">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="item in configs" :key="item.id">
                  <td class="px-3 py-3">
                    <div class="font-medium text-gray-900 dark:text-white">{{ item.name }}</div>
                  </td>
                  <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ categoryLabel(item.category) }}</td>
                  <td class="px-3 py-3 text-gray-900 dark:text-white">{{ item.amount.toFixed(2) }} {{ item.currency }}</td>
                  <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ item.exchange_rate_to_billing_unit }}x</td>
                  <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ allocationMethodLabel(item.allocation_method) }}</td>
                  <td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ formatScope(item.scope) }}</td>
                  <td class="px-3 py-3 text-gray-500">{{ formatDate(item.effective_from) }}</td>
                  <td class="px-3 py-3">
                    <span class="badge" :class="item.enabled ? 'badge-success' : 'badge-secondary'">{{ item.enabled ? '启用' : '停用' }}</span>
                  </td>
                  <td class="px-3 py-3">
                    <div class="flex gap-2">
                      <button class="btn btn-secondary btn-sm" @click="editConfig(item)">编辑</button>
                      <button v-if="item.enabled" class="btn btn-danger btn-sm" @click="disableConfig(item.id)">停用</button>
                      <button class="btn btn-danger btn-sm" @click="deleteConfig(item)">删除</button>
                    </div>
                  </td>
                </tr>
                <tr v-if="!configs.length">
                  <td colspan="9" class="px-3 py-10 text-center text-sm text-gray-500">暂无成本配置</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div v-if="configFormVisible" class="card p-6">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="font-semibold text-gray-900 dark:text-white">{{ editingConfigId ? '编辑成本配置' : '新增成本配置' }}</h2>
            <button class="text-sm text-gray-500" @click="configFormVisible = false">取消</button>
          </div>
          <form class="grid gap-4 md:grid-cols-2" @submit.prevent="saveConfig">
            <label class="field"><span>编码</span><input v-model="configForm.code" class="input" required maxlength="64" /></label>
            <label class="field"><span>名称</span><input v-model="configForm.name" class="input" required maxlength="128" /></label>
            <label class="field">
              <span>类别</span>
              <div class="relative">
                <select v-model="configForm.category" class="input appearance-none pr-10">
                  <option v-for="value in categories" :key="value" :value="value">{{ categoryLabel(value) }}</option>
                </select>
                <svg class="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
              </div>
            </label>
            <label class="field"><span>金额</span><input v-model.number="configForm.amount" class="input" type="number" min="0" step="0.00000001" required /></label>
            <label class="field"><span>币种</span><input v-model="configForm.currency" class="input" maxlength="3" /></label>
            <label class="field"><span>折算率（原币 → 系统计费单位）</span><input v-model.number="configForm.exchange_rate_to_billing_unit" class="input" type="number" min="0.00000001" step="0.00000001" required /><small class="text-gray-500">报表金额 = 原币金额 × 折算率；金额已是系统计费单位时填 1。</small></label>
            <label class="field">
              <span>分摊方式</span>
              <div class="relative">
                <select v-model="configForm.allocation_method" class="input appearance-none pr-10">
                  <option v-for="value in allocationMethods" :key="value" :value="value">{{ allocationMethodLabel(value) }}</option>
                </select>
                <svg class="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
              </div>
            </label>
            <label class="field">
              <span>计费周期</span>
              <div class="relative">
                <select v-model="configForm.frequency" class="input appearance-none pr-10">
                  <option v-for="value in frequencies" :key="value" :value="value">{{ frequencyLabel(value) }}</option>
                </select>
                <svg class="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
              </div>
            </label>
            <label class="field"><span>生效时间</span><input v-model="configForm.effective_from" class="input" type="datetime-local" required /></label>
            <label class="field"><span>结束时间（可选）</span><input v-model="configForm.effective_to" class="input" type="datetime-local" /></label>
            <label class="field md:col-span-2"><span>作用范围 JSON</span><textarea v-model="configScopeJSON" class="input min-h-20 font-mono text-xs" placeholder="{} 或 {&quot;group_id&quot;: 1} / {&quot;channel_id&quot;: 2} / {&quot;model&quot;: &quot;gpt-4o&quot;}"></textarea><small class="text-gray-500">留空或填写 {} 表示全局；支持 group_id、channel_id、account_id、model，可组合填写。</small></label>
            <label class="field"><span>备注</span><input v-model="configForm.notes" class="input" maxlength="500" /></label>
            <div class="flex items-center gap-2 md:col-span-2"><input id="config-enabled" v-model="configForm.enabled" type="checkbox" /><label for="config-enabled">启用</label></div>
            <div class="flex justify-end gap-2 md:col-span-2"><button type="button" class="btn btn-secondary" @click="configFormVisible = false">取消</button><button class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button></div>
          </form>
        </div>
      </section>

      <section v-else-if="activeTab === 'expenses'" class="space-y-6">
        <div class="card p-6">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 class="font-semibold text-gray-900 dark:text-white">费用台账</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">适合已经发生、一次确认的临时费用或调整；费用周期只决定报表归属，不按天自动摊销。</p>
            </div>
            <button class="btn btn-primary" @click="startNewExpense">新增费用</button>
          </div>
          <div class="overflow-x-auto">
            <table class="min-w-full text-left text-sm">
              <thead class="border-b border-gray-200 text-xs uppercase text-gray-500 dark:border-dark-700"><tr><th class="px-3 py-3">名称</th><th class="px-3 py-3">类别</th><th class="px-3 py-3">金额</th><th class="px-3 py-3">发生时间</th><th class="px-3 py-3">费用周期</th><th class="px-3 py-3">分摊方式</th><th class="px-3 py-3">作用范围</th><th class="px-3 py-3">状态</th><th class="px-3 py-3">操作</th></tr></thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="item in expenses" :key="item.id"><td class="px-3 py-3"><div class="font-medium text-gray-900 dark:text-white">{{ item.name }}</div><div class="text-xs text-gray-500">{{ item.notes }}</div></td><td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ categoryLabel(item.category) }}</td><td class="px-3 py-3 text-gray-900 dark:text-white">{{ item.amount.toFixed(2) }} {{ item.currency }}<div v-if="item.exchange_rate_to_billing_unit !== 1" class="text-xs text-gray-500">折算 {{ (item.amount * item.exchange_rate_to_billing_unit).toFixed(4) }}</div></td><td class="px-3 py-3 text-gray-500">{{ formatDate(item.occurred_at) }}</td><td class="px-3 py-3 text-gray-500">{{ formatPeriod(item.period_start, item.period_end) }}</td><td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ allocationMethodLabel(item.allocation_method) }}</td><td class="px-3 py-3 text-gray-600 dark:text-gray-300">{{ formatScope(item.scope) }}</td><td class="px-3 py-3"><span class="badge" :class="item.status === 'active' ? 'badge-success' : 'badge-secondary'">{{ item.status === 'active' ? '有效' : '已作废' }}</span></td><td class="px-3 py-3"><div class="flex gap-2"><button v-if="item.status === 'active'" class="btn btn-secondary btn-sm" @click="editExpense(item)">编辑</button><button v-if="item.status === 'active'" class="btn btn-danger btn-sm" @click="voidExpense(item.id)">作废</button></div></td></tr>
                <tr v-if="!expenses.length"><td colspan="9" class="px-3 py-10 text-center text-sm text-gray-500">暂无费用记录</td></tr>
              </tbody>
            </table>
          </div>
          <div class="mt-4 flex items-center justify-between text-sm text-gray-500"><span>共 {{ expenseTotal }} 条</span><div class="flex gap-2"><button class="btn btn-secondary btn-sm" :disabled="expensePage <= 1" @click="expensePage--; loadExpenses()">上一页</button><button class="btn btn-secondary btn-sm" :disabled="expenses.length < pageSize" @click="expensePage++; loadExpenses()">下一页</button></div></div>
        </div>

        <div v-if="expenseFormVisible" class="card p-6">
          <div class="mb-4 flex items-center justify-between"><h2 class="font-semibold text-gray-900 dark:text-white">{{ editingExpenseId ? '编辑费用' : '新增费用' }}</h2><button class="text-sm text-gray-500" @click="expenseFormVisible = false">取消</button></div>
          <form class="grid gap-4 md:grid-cols-2" @submit.prevent="saveExpense">
            <label class="field"><span>名称</span><input v-model="expenseForm.name" class="input" required maxlength="128" /></label>
            <label class="field">
              <span>类别</span>
              <div class="relative">
                <select v-model="expenseForm.category" class="input appearance-none pr-10">
                  <option v-for="value in categories" :key="value" :value="value">{{ categoryLabel(value) }}</option>
                </select>
                <svg class="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
              </div>
            </label>
            <label class="field"><span>金额</span><input v-model.number="expenseForm.amount" class="input" type="number" min="0" step="0.00000001" required /></label>
            <label class="field"><span>币种</span><input v-model="expenseForm.currency" class="input" maxlength="3" /></label>
            <label class="field"><span>折算率（原币 → 系统计费单位）</span><input v-model.number="expenseForm.exchange_rate_to_billing_unit" class="input" type="number" min="0.00000001" step="0.00000001" required /><small class="text-gray-500">报表金额 = 原币金额 × 折算率；金额已是系统计费单位时填 1。</small></label>
            <label class="field"><span>发生时间</span><input v-model="expenseForm.occurred_at" class="input" type="datetime-local" required /></label>
            <label class="field"><span>费用周期开始（可选）</span><input v-model="expenseForm.period_start" class="input" type="datetime-local" /></label>
            <label class="field"><span>费用周期结束（可选）</span><input v-model="expenseForm.period_end" class="input" type="datetime-local" /></label>
            <label class="field">
              <span>分摊方式</span>
              <div class="relative">
                <select v-model="expenseForm.allocation_method" class="input appearance-none pr-10">
                  <option v-for="value in allocationMethods" :key="value" :value="value">{{ allocationMethodLabel(value) }}</option>
                </select>
                <svg class="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
              </div>
            </label>
            <label class="field md:col-span-2"><span>作用范围 JSON</span><textarea v-model="expenseScopeJSON" class="input min-h-20 font-mono text-xs" placeholder="{} 或 {&quot;group_id&quot;: 1} / {&quot;channel_id&quot;: 2} / {&quot;model&quot;: &quot;gpt-4o&quot;}"></textarea><small class="text-gray-500">留空或填写 {} 表示全局；支持 group_id、channel_id、account_id、model，可组合填写。</small></label>
            <label class="field md:col-span-2"><span>备注</span><input v-model="expenseForm.notes" class="input" maxlength="500" /></label>
            <div class="flex justify-end gap-2 md:col-span-2"><button type="button" class="btn btn-secondary" @click="expenseFormVisible = false">取消</button><button class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button></div>
          </form>
        </div>
      </section>

    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import businessFinanceAPI, { type BusinessCostConfig, type BusinessExpense, type BusinessCostConfigInput, type BusinessExpenseInput, type FinanceCategory, type AllocationMethod, type CostFrequency } from '@/api/admin/businessFinance'

const appStore = useAppStore()
const activeTab = ref<'configs' | 'expenses'>('configs')
const loading = ref(false)
const saving = ref(false)
const configs = ref<BusinessCostConfig[]>([])
const expenses = ref<BusinessExpense[]>([])
const expenseTotal = ref(0)
const expensePage = ref(1)
const pageSize = 20
const configFormVisible = ref(false)
const expenseFormVisible = ref(false)
const editingConfigId = ref<number | null>(null)
const editingExpenseId = ref<number | null>(null)
const categories: FinanceCategory[] = ['server', 'database', 'redis', 'bandwidth', 'domain', 'compliance', 'proxy', 'payment_fee', 'marketing', 'affiliate', 'account_purchase', 'customer_service', 'risk_reserve', 'other']
const categoryLabels: Record<FinanceCategory, string> = {
  server: '服务器',
  database: '数据库',
  redis: 'redis',
  bandwidth: '带宽',
  domain: '域名',
  compliance: '合规/备案',
  proxy: '代理',
  payment_fee: '支付手续费',
  marketing: '推广',
  affiliate: '渠道分成',
  account_purchase: '账号采购',
  customer_service: '客服',
  risk_reserve: '风险准备金',
  other: '其他',
}
const allocationMethods: AllocationMethod[] = ['revenue_share', 'request_share', 'token_share', 'account_average', 'manual', 'direct']
const frequencies: CostFrequency[] = ['one_time', 'daily', 'monthly', 'yearly']
const allocationMethodLabels: Record<AllocationMethod, string> = {
  revenue_share: '按收入占比',
  request_share: '按请求占比',
  token_share: '按 Token 占比',
  account_average: '按账号均摊',
  manual: '手动分摊',
  direct: '直接计入',
}
const frequencyLabels: Record<CostFrequency, string> = {
  one_time: '一次性',
  daily: '每日',
  monthly: '每月',
  yearly: '每年',
}

function categoryLabel(value: FinanceCategory) {
  return categoryLabels[value] || value
}

function allocationMethodLabel(value: AllocationMethod) {
  return allocationMethodLabels[value] || value
}

function frequencyLabel(value: CostFrequency) {
  return frequencyLabels[value] || value
}

const configForm = reactive<BusinessCostConfigInput & { effective_from: string; effective_to: string; enabled: boolean }>({ code: '', name: '', category: 'server', amount: 0, currency: 'CNY', exchange_rate_to_billing_unit: 1, allocation_method: 'revenue_share', frequency: 'monthly', scope: {}, effective_from: toLocalInput(new Date().toISOString()), effective_to: '', enabled: true, notes: '' })
const expenseForm = reactive<BusinessExpenseInput>({ category: 'server', name: '', amount: 0, currency: 'CNY', exchange_rate_to_billing_unit: 1, occurred_at: toLocalInput(new Date().toISOString()), allocation_method: 'revenue_share', scope: {}, notes: '' })
const configScopeJSON = ref('{}')
const expenseScopeJSON = ref('{}')


async function loadConfigs() {
  const { data } = await businessFinanceAPI.listCostConfigs()
  configs.value = data
}

async function loadExpenses() {
  const { data } = await businessFinanceAPI.listExpenses({ page: expensePage.value, page_size: pageSize })
  expenses.value = data.items || []
  expenseTotal.value = data.total || 0
}


async function reload() {
  loading.value = true
  try {
    await Promise.all([loadConfigs(), loadExpenses()])
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '经营数据加载失败')
  } finally {
    loading.value = false
  }
}







function startNewConfig() {
  editingConfigId.value = null
  Object.assign(configForm, { code: '', name: '', category: 'server', amount: 0, currency: 'CNY', exchange_rate_to_billing_unit: 1, allocation_method: 'revenue_share', frequency: 'monthly', scope: {}, effective_from: toLocalInput(new Date().toISOString()), effective_to: '', enabled: true, notes: '' })
  configScopeJSON.value = '{}'
  configFormVisible.value = true
}

function editConfig(item: BusinessCostConfig) {
  editingConfigId.value = item.id
  Object.assign(configForm, { ...item, effective_from: toLocalInput(item.effective_from), effective_to: optionalLocalInput(item.effective_to) })
  configScopeJSON.value = stringifyScope(item.scope)
  configFormVisible.value = true
}

async function saveConfig() {
  saving.value = true
  try {
    const input: BusinessCostConfigInput = { ...configForm, scope: parseScope(configScopeJSON.value), effective_from: new Date(configForm.effective_from).toISOString(), effective_to: toOptionalISOString(configForm.effective_to) }
    if (editingConfigId.value) await businessFinanceAPI.updateCostConfig(editingConfigId.value, input)
    else await businessFinanceAPI.createCostConfig(input)
    configFormVisible.value = false
    await loadConfigs()
    appStore.showSuccess('成本配置已保存')
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '成本配置保存失败')
  } finally { saving.value = false }
}

async function disableConfig(id: number) {
	if (!window.confirm('确定停用这条成本配置吗？')) return
	try { await businessFinanceAPI.disableCostConfig(id); await loadConfigs(); appStore.showSuccess('成本配置已停用') } catch (error) { appStore.showError(error instanceof Error ? error.message : '停用失败') }
}

async function deleteConfig(item: BusinessCostConfig) {
  if (!window.confirm(`确定永久删除成本配置“${item.name}”吗？删除后无法恢复，也不会再参与成本计算。`)) return
  try {
    await businessFinanceAPI.deleteCostConfig(item.id)
    if (editingConfigId.value === item.id) configFormVisible.value = false
    await loadConfigs()
    appStore.showSuccess('成本配置已删除')
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '删除失败')
  }
}

function startNewExpense() {
  editingExpenseId.value = null
  Object.assign(expenseForm, { category: 'server', name: '', amount: 0, currency: 'CNY', exchange_rate_to_billing_unit: 1, occurred_at: toLocalInput(new Date().toISOString()), period_start: null, period_end: null, allocation_method: 'revenue_share', scope: {}, notes: '' })
  expenseScopeJSON.value = '{}'
  expenseFormVisible.value = true
}

function editExpense(item: BusinessExpense) {
  editingExpenseId.value = item.id
  Object.assign(expenseForm, { ...item, occurred_at: toLocalInput(item.occurred_at), period_start: optionalLocalInput(item.period_start), period_end: optionalLocalInput(item.period_end) })
  expenseScopeJSON.value = stringifyScope(item.scope)
  expenseFormVisible.value = true
}

async function saveExpense() {
  saving.value = true
  try {
    const input: BusinessExpenseInput = { ...expenseForm, scope: parseScope(expenseScopeJSON.value), occurred_at: new Date(expenseForm.occurred_at).toISOString(), period_start: toOptionalISOString(expenseForm.period_start), period_end: toOptionalISOString(expenseForm.period_end) }
    if (editingExpenseId.value) await businessFinanceAPI.updateExpense(editingExpenseId.value, input)
    else await businessFinanceAPI.createExpense(input)
    expenseFormVisible.value = false
    await loadExpenses()
    appStore.showSuccess('费用记录已保存')
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '费用记录保存失败')
  } finally { saving.value = false }
}

async function voidExpense(id: number) {
  if (!window.confirm('确定作废这条费用记录吗？作废后不会参与成本计算。')) return
  try { await businessFinanceAPI.voidExpense(id); await loadExpenses(); appStore.showSuccess('费用记录已作废') } catch (error) { appStore.showError(error instanceof Error ? error.message : '费用作废失败') }
}

function formatDate(value: string) { return new Date(value).toLocaleString() }
function formatPeriod(start: string | null | undefined, end: string | null | undefined) { return start || end ? `${start ? formatDate(start) : '—'} ~ ${end ? formatDate(end) : '持续'}` : '单点费用' }
function formatScope(scope: Record<string, unknown> | undefined) { return scope && Object.keys(scope).length ? Object.entries(scope).map(([key, value]) => `${key}=${String(value)}`).join(', ') : '全局' }
function stringifyScope(scope: Record<string, unknown> | undefined) { return JSON.stringify(scope || {}, null, 2) }
function parseScope(raw: string) { const parsed: unknown = JSON.parse(raw.trim() || '{}'); if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('作用范围必须是 JSON 对象'); return parsed as Record<string, unknown> }
function optionalLocalInput(value: string | null | undefined) { return value ? toLocalInput(value) : '' }
function toOptionalISOString(value: string | null | undefined) { return value ? new Date(value).toISOString() : null }
function toLocalInput(value: string) { const date = new Date(value); const offset = date.getTimezoneOffset(); return new Date(date.getTime() - offset * 60000).toISOString().slice(0, 16) }

onMounted(reload)
</script>

<style scoped>
.field { display: flex; flex-direction: column; gap: 0.4rem; font-size: 0.875rem; color: rgb(107 114 128); }
.field span { font-weight: 500; }
.btn-sm { padding: 0.35rem 0.65rem; font-size: 0.75rem; }
.badge-success { color: rgb(22 101 52); background: rgb(220 252 231); }
.badge-secondary { color: rgb(75 85 99); background: rgb(243 244 246); }
</style>
