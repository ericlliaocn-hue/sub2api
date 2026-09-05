<template>
  <AppLayout>
    <div class="space-y-6 p-6">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">推广管理</h1>
          <p class="mt-1 text-sm text-gray-500">渠道首次归因、转化成本、佣金冻结与人工结算。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button v-for="item in tabs" :key="item.key" class="btn" :class="tab === item.key ? 'btn-primary' : 'btn-secondary'" @click="tab = item.key">
            {{ item.label }}
          </button>
        </div>
      </div>

      <template v-if="tab === 'report'">
        <section class="card p-5">
          <div class="flex flex-wrap items-end gap-3">
            <label class="field"><span>报表口径</span><select v-model="reportMode" class="input"><option value="operation">经营期</option><option value="acquisition">拉新同期</option></select></label>
            <label class="field"><span>开始日期</span><input v-model="start" type="date" class="input" /></label>
            <label class="field"><span>结束日期</span><input v-model="end" type="date" class="input" /></label>
            <button class="btn btn-primary" :disabled="loading" @click="loadReport">刷新报表</button>
            <p class="text-xs text-gray-500">{{ reportMode === 'operation' ? '统计周期内所有已归因用户产生的经营数据' : '统计周期内注册用户截至结束日期的转化数据' }}</p>
          </div>
        </section>

        <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <div class="card p-5"><div class="text-sm text-gray-500">新增用户</div><div class="mt-2 text-2xl font-semibold">{{ totals.newUsers }}</div></div>
          <div class="card p-5"><div class="text-sm text-gray-500">付费用户</div><div class="mt-2 text-2xl font-semibold">{{ totals.payingUsers }}</div></div>
          <div class="card p-5"><div class="text-sm text-gray-500">消耗收入</div><div class="mt-2 text-2xl font-semibold">{{ money(totals.revenue) }}</div></div>
          <div class="card p-5"><div class="text-sm text-gray-500">贡献利润</div><div class="mt-2 text-2xl font-semibold" :class="totals.profit < 0 ? 'text-red-500' : 'text-emerald-500'">{{ money(totals.profit) }}</div></div>
        </section>

        <section class="card overflow-hidden">
          <div class="border-b border-gray-200 p-5 dark:border-dark-700"><h2 class="font-semibold">渠道经营明细</h2></div>
          <div class="overflow-x-auto">
            <table class="min-w-full whitespace-nowrap text-left text-sm">
              <thead class="border-b border-gray-200 text-xs text-gray-500"><tr><th class="px-4 py-3">渠道</th><th class="px-4 py-3">负责人</th><th class="px-4 py-3">注册</th><th class="px-4 py-3">付费/活跃</th><th class="px-4 py-3">充值</th><th class="px-4 py-3">收入</th><th class="px-4 py-3">上游成本</th><th class="px-4 py-3">赠送/返利/佣金</th><th class="px-4 py-3">手续费/营销</th><th class="px-4 py-3">贡献利润</th><th class="px-4 py-3">CAC/LTV</th><th class="px-4 py-3">ROI</th></tr></thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="row in report?.rows" :key="row.channel_id">
                  <td class="px-4 py-3"><b>{{ row.name }}</b><div class="font-mono text-xs text-gray-500">{{ row.code }} · {{ row.channel_type }}</div></td>
                  <td class="px-4 py-3">{{ row.promoter_name || '—' }}</td><td class="px-4 py-3">{{ row.new_users }}</td><td class="px-4 py-3">{{ row.paying_users }} / {{ row.active_users }}</td>
                  <td class="px-4 py-3">{{ money(row.recharge) }}</td><td class="px-4 py-3">{{ money(row.revenue) }}</td><td class="px-4 py-3">{{ money(row.upstream_cost) }}</td>
                  <td class="px-4 py-3"><div>{{ money(row.bonus_cost) }}</div><div class="text-xs text-gray-500">返 {{ money(row.affiliate_cost) }} · 佣 {{ money(row.commission_cost) }}</div></td>
                  <td class="px-4 py-3"><div>{{ money(row.payment_fee) }}</div><div class="text-xs text-gray-500">营销 {{ money(row.marketing_cost) }}</div></td>
                  <td class="px-4 py-3 font-semibold" :class="row.profit < 0 ? 'text-red-500' : 'text-emerald-500'">{{ money(row.profit) }}</td><td class="px-4 py-3">{{ money(row.cac) }} / {{ money(row.ltv) }}</td><td class="px-4 py-3">{{ percent(row.roi * 100) }}</td>
                </tr>
                <tr v-if="!report?.rows.length"><td colspan="12" class="px-4 py-10 text-center text-gray-500">暂无渠道数据</td></tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>

      <template v-else-if="tab === 'config'">
        <section class="grid gap-6 xl:grid-cols-2">
          <div class="card p-5">
            <div class="mb-4 flex items-center justify-between"><div><h2 class="font-semibold">推广成员</h2><p class="text-xs text-gray-500">默认佣金比例和退款观察冻结期</p></div><button class="btn btn-primary btn-sm" @click="newPromoter">新增</button></div>
            <div v-for="item in promoters" :key="item.id" class="flex items-center justify-between border-b border-gray-100 py-3 dark:border-dark-700">
              <div><div class="flex items-center gap-2"><b>{{ item.name }}</b><span class="rounded px-2 py-0.5 text-xs" :class="item.enabled ? 'bg-emerald-100 text-emerald-700' : 'bg-gray-100 text-gray-500'">{{ item.enabled ? '启用' : '停用' }}</span></div><div class="text-xs text-gray-500">{{ item.contact || '无联系方式' }} · 佣金 {{ percent(item.commission_rate) }} · 冻结 {{ item.commission_freeze_days }} 天</div></div>
              <button class="btn btn-secondary btn-sm" @click="editPromoter(item)">编辑</button>
            </div>
            <div v-if="!promoters.length" class="py-10 text-center text-sm text-gray-500">暂无推广成员</div>
          </div>
          <div class="card p-5">
            <div class="mb-4 flex items-center justify-between"><div><h2 class="font-semibold">推广渠道</h2><p class="text-xs text-gray-500">注册地址使用 ?source=渠道编码</p></div><button class="btn btn-primary btn-sm" @click="newChannel">新增</button></div>
            <div v-for="item in channels" :key="item.id" class="flex items-center justify-between gap-3 border-b border-gray-100 py-3 dark:border-dark-700">
              <div class="min-w-0"><div class="flex items-center gap-2"><b>{{ item.name }}</b><span class="rounded px-2 py-0.5 text-xs" :class="item.enabled ? 'bg-emerald-100 text-emerald-700' : 'bg-gray-100 text-gray-500'">{{ item.enabled ? '启用' : '停用' }}</span></div><div class="truncate font-mono text-xs text-gray-500">{{ item.code }} · {{ item.channel_type }} · {{ item.promoter_name || '未分配' }} · {{ item.commission_rate == null ? '成员默认佣金' : percent(item.commission_rate) }}</div></div>
              <div class="flex gap-2"><button class="btn btn-secondary btn-sm" @click="copyLink(item)">复制链接</button><button class="btn btn-secondary btn-sm" @click="editChannel(item)">编辑</button></div>
            </div>
            <div v-if="!channels.length" class="py-10 text-center text-sm text-gray-500">暂无推广渠道</div>
          </div>
        </section>

        <section v-if="modal" class="card p-5">
          <div class="mb-4 flex items-center justify-between"><h2 class="font-semibold">{{ modal.kind === 'promoter' ? '推广成员' : '推广渠道' }}</h2><button class="text-sm text-gray-500" @click="modal = null">关闭</button></div>
          <form class="grid gap-4 md:grid-cols-2" @submit.prevent="saveConfig">
            <label class="field"><span>名称</span><input v-model.trim="form.name" class="input" required /></label>
            <label v-if="modal.kind === 'promoter'" class="field"><span>联系方式</span><input v-model.trim="form.contact" class="input" /></label>
            <label v-else class="field"><span>渠道编码</span><input v-model.trim="form.code" class="input font-mono uppercase" maxlength="64" required /></label>
            <template v-if="modal.kind === 'promoter'">
              <label class="field"><span>默认佣金比例（%）</span><input v-model.number="form.commission_rate" class="input" type="number" min="0" max="100" step="0.01" required /></label>
              <label class="field"><span>佣金冻结天数</span><input v-model.number="form.freeze_days" class="input" type="number" min="0" max="365" required /></label>
            </template>
            <template v-else>
              <label class="field"><span>渠道类型</span><input v-model.trim="form.channel_type" class="input" placeholder="SEO / 论坛 / TG群 / 自定义" /></label>
              <label class="field"><span>推广成员</span><select v-model="form.promoter_id" class="input"><option :value="null">未分配</option><option v-for="item in promoters" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
              <label class="field"><span>渠道覆盖佣金（留空使用成员默认）</span><input v-model="form.channel_rate" class="input" type="number" min="0" max="100" step="0.01" /></label>
            </template>
            <label class="field md:col-span-2"><span>备注</span><input v-model.trim="form.notes" class="input" /></label>
            <label class="flex items-center gap-2 text-sm"><input v-model="form.enabled" type="checkbox" />启用</label>
            <div class="flex justify-end gap-2 md:col-span-2"><button type="button" class="btn btn-secondary" @click="modal = null">取消</button><button class="btn btn-primary" :disabled="saving">保存</button></div>
          </form>
        </section>
      </template>

      <template v-else-if="tab === 'attribution'">
        <section class="card overflow-hidden"><div class="flex items-center justify-between border-b border-gray-200 p-5 dark:border-dark-700"><div><h2 class="font-semibold">注册归因审计</h2><p class="text-xs text-gray-500">无效、停用和重复归因同样会被记录</p></div><button class="btn btn-secondary btn-sm" @click="loadEvents">刷新</button></div><div class="overflow-x-auto"><table class="min-w-full text-left text-sm"><thead class="border-b text-xs text-gray-500"><tr><th class="px-4 py-3">时间</th><th class="px-4 py-3">用户</th><th class="px-4 py-3">请求编码</th><th class="px-4 py-3">渠道</th><th class="px-4 py-3">结果</th></tr></thead><tbody class="divide-y dark:divide-dark-700"><tr v-for="item in events" :key="item.id"><td class="px-4 py-3">{{ dateTime(item.created_at) }}</td><td class="px-4 py-3">#{{ item.user_id }}<div class="text-xs text-gray-500">{{ item.user_email }}</div></td><td class="px-4 py-3 font-mono">{{ item.requested_code }}</td><td class="px-4 py-3">{{ item.channel_name || '—' }}</td><td class="px-4 py-3"><span :class="item.outcome === 'attributed' ? 'text-emerald-500' : item.outcome === 'already_attributed' ? 'text-amber-500' : 'text-red-500'">{{ outcomeLabel(item.outcome) }}</span><div class="text-xs text-gray-500">{{ item.detail }}</div></td></tr><tr v-if="!events.length"><td colspan="5" class="px-4 py-10 text-center text-gray-500">暂无归因记录</td></tr></tbody></table></div></section>
      </template>

      <template v-else>
        <section class="card p-5"><div class="flex flex-wrap items-end gap-3"><label class="field"><span>推广成员</span><select v-model="commissionPromoter" class="input"><option :value="0">全部</option><option v-for="item in promoters" :key="item.id" :value="item.id">{{ item.name }}</option></select></label><label class="field"><span>佣金状态</span><select v-model="commissionStatus" class="input"><option value="">全部</option><option value="frozen">冻结中</option><option value="available">可结算</option><option value="settled">已结算</option><option value="reversed">已冲正</option></select></label><button class="btn btn-secondary" @click="loadCommissions">筛选</button><button class="btn btn-primary" :disabled="!commissionPromoter" @click="openSettlement">生成结算单</button></div></section>
        <section v-if="settlementForm" class="card p-5"><h2 class="mb-4 font-semibold">生成结算单</h2><form class="grid gap-4 md:grid-cols-3" @submit.prevent="createSettlement"><label class="field"><span>截止时间</span><input v-model="settlementForm.period_end" type="datetime-local" class="input" required /></label><label class="field md:col-span-2"><span>备注</span><input v-model.trim="settlementForm.notes" class="input" /></label><div class="flex gap-2 md:col-span-3"><button class="btn btn-primary">确认生成</button><button type="button" class="btn btn-secondary" @click="settlementForm = null">取消</button></div></form></section>
        <section class="card overflow-hidden"><div class="border-b p-5"><h2 class="font-semibold">佣金流水</h2></div><div class="overflow-x-auto"><table class="min-w-full text-left text-sm"><thead class="border-b text-xs text-gray-500"><tr><th class="px-4 py-3">订单</th><th class="px-4 py-3">推广成员/渠道</th><th class="px-4 py-3">用户</th><th class="px-4 py-3">基数 × 比例</th><th class="px-4 py-3">佣金/冲正</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">时间</th></tr></thead><tbody class="divide-y dark:divide-dark-700"><tr v-for="item in commissions" :key="item.id"><td class="px-4 py-3">#{{ item.payment_order_id }}</td><td class="px-4 py-3">{{ item.promoter_name }}<div class="text-xs text-gray-500">{{ item.channel_name }} · {{ item.channel_code }}</div></td><td class="px-4 py-3">#{{ item.user_id }}<div class="text-xs text-gray-500">{{ item.user_email }}</div></td><td class="px-4 py-3">{{ money(item.base_amount) }} × {{ percent(item.commission_rate) }}</td><td class="px-4 py-3">{{ money(item.amount) }}<div v-if="item.reversed_amount" class="text-xs text-red-500">冲正 {{ money(item.reversed_amount) }}</div></td><td class="px-4 py-3">{{ commissionStatusLabel(item.status) }}<div v-if="item.frozen_until && item.status === 'frozen'" class="text-xs text-gray-500">至 {{ dateTime(item.frozen_until) }}</div></td><td class="px-4 py-3">{{ dateTime(item.created_at) }}</td></tr><tr v-if="!commissions.length"><td colspan="7" class="px-4 py-10 text-center text-gray-500">暂无佣金流水</td></tr></tbody></table></div></section>
        <section class="card overflow-hidden"><div class="border-b p-5"><h2 class="font-semibold">结算单</h2></div><div class="overflow-x-auto"><table class="min-w-full text-left text-sm"><thead class="border-b text-xs text-gray-500"><tr><th class="px-4 py-3">编号</th><th class="px-4 py-3">推广成员</th><th class="px-4 py-3">截止时间</th><th class="px-4 py-3">金额</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y dark:divide-dark-700"><tr v-for="item in settlements" :key="item.id"><td class="px-4 py-3">#{{ item.id }}</td><td class="px-4 py-3">{{ item.promoter_name }}</td><td class="px-4 py-3">{{ dateTime(item.period_end) }}</td><td class="px-4 py-3 font-semibold">{{ money(item.amount) }}</td><td class="px-4 py-3">{{ settlementStatusLabel(item.status) }}</td><td class="px-4 py-3"><div v-if="item.status === 'draft'" class="flex gap-2"><button class="btn btn-primary btn-sm" @click="updateSettlement(item.id, 'paid')">确认已支付</button><button class="btn btn-secondary btn-sm" @click="updateSettlement(item.id, 'cancelled')">取消</button></div><span v-else>—</span></td></tr><tr v-if="!settlements.length"><td colspan="6" class="px-4 py-10 text-center text-gray-500">暂无结算单</td></tr></tbody></table></div></section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import promotionAPI, { type PromotionAttributionEvent, type PromotionChannel, type PromotionCommission, type PromotionPromoter, type PromotionReport, type PromotionSettlement } from '@/api/admin/promotion'

type Tab = 'report' | 'config' | 'attribution' | 'commission'
const tabs: Array<{ key: Tab; label: string }> = [{ key: 'report', label: '渠道报表' }, { key: 'config', label: '成员与渠道' }, { key: 'attribution', label: '归因审计' }, { key: 'commission', label: '佣金结算' }]
const appStore = useAppStore()
const tab = ref<Tab>('report')
const loading = ref(false), saving = ref(false)
const promoters = ref<PromotionPromoter[]>([]), channels = ref<PromotionChannel[]>([])
const report = ref<PromotionReport | null>(null), events = ref<PromotionAttributionEvent[]>([]), commissions = ref<PromotionCommission[]>([]), settlements = ref<PromotionSettlement[]>([])
const end = ref(new Date().toISOString().slice(0, 10)); const initialStart = new Date(); initialStart.setDate(initialStart.getDate() - 30); const start = ref(initialStart.toISOString().slice(0, 10))
const reportMode = ref<'operation' | 'acquisition'>('operation')
const commissionPromoter = ref(0), commissionStatus = ref('')
const modal = ref<{ kind: 'promoter' | 'channel'; id?: number } | null>(null)
const form = reactive({ name: '', contact: '', code: '', commission_rate: 0, freeze_days: 7, channel_type: 'other', promoter_id: null as number | null, channel_rate: '' as string | number, notes: '', enabled: true })
const settlementForm = ref<{ period_end: string; notes: string } | null>(null)
const totals = computed(() => (report.value?.rows || []).reduce((sum, row) => ({ newUsers: sum.newUsers + row.new_users, payingUsers: sum.payingUsers + row.paying_users, revenue: sum.revenue + row.revenue, profit: sum.profit + row.profit }), { newUsers: 0, payingUsers: 0, revenue: 0, profit: 0 }))

function money(value: number) { return (Number(value) || 0).toFixed(4) }
function percent(value: number) { return `${(Number(value) || 0).toFixed(2)}%` }
function dateTime(value: string) { return value ? new Date(value).toLocaleString() : '—' }
function errorMessage(error: unknown) { return error instanceof Error ? error.message : '操作失败' }
function resetForm() { Object.assign(form, { name: '', contact: '', code: '', commission_rate: 0, freeze_days: 7, channel_type: 'other', promoter_id: null, channel_rate: '', notes: '', enabled: true }) }
function newPromoter() { resetForm(); modal.value = { kind: 'promoter' } }
function newChannel() { resetForm(); modal.value = { kind: 'channel' } }
function editPromoter(item: PromotionPromoter) { resetForm(); Object.assign(form, { name: item.name, contact: item.contact, commission_rate: item.commission_rate, freeze_days: item.commission_freeze_days, notes: item.notes, enabled: item.enabled }); modal.value = { kind: 'promoter', id: item.id } }
function editChannel(item: PromotionChannel) { resetForm(); Object.assign(form, { name: item.name, code: item.code, channel_type: item.channel_type, promoter_id: item.promoter_id ?? null, channel_rate: item.commission_rate ?? '', notes: item.notes, enabled: item.enabled }); modal.value = { kind: 'channel', id: item.id } }
async function copyLink(item: PromotionChannel) { await navigator.clipboard.writeText(`${window.location.origin}/register?source=${encodeURIComponent(item.code)}`); appStore.showSuccess('推广链接已复制') }
async function loadBase() { const [a, b] = await Promise.all([promotionAPI.listPromoters(), promotionAPI.listChannels()]); promoters.value = a.data; channels.value = b.data }
async function loadReport() { loading.value = true; try { report.value = (await promotionAPI.report({ start_time: new Date(`${start.value}T00:00:00`).toISOString(), end_time: new Date(`${end.value}T23:59:59.999`).toISOString(), mode: reportMode.value })).data } catch (error) { appStore.showError(errorMessage(error)) } finally { loading.value = false } }
async function loadEvents() { try { events.value = (await promotionAPI.listAttributionEvents({ limit: 200 })).data } catch (error) { appStore.showError(errorMessage(error)) } }
async function loadCommissions() { try { const params = { promoter_id: commissionPromoter.value || undefined, status: commissionStatus.value || undefined, limit: 300 }; const [a, b] = await Promise.all([promotionAPI.listCommissions(params), promotionAPI.listSettlements({ promoter_id: commissionPromoter.value || undefined, limit: 100 })]); commissions.value = a.data; settlements.value = b.data } catch (error) { appStore.showError(errorMessage(error)) } }
async function saveConfig() { if (!modal.value) return; saving.value = true; try { if (modal.value.kind === 'promoter') { const payload = { name: form.name, contact: form.contact, commission_rate: Number(form.commission_rate), commission_freeze_days: Number(form.freeze_days), enabled: form.enabled, notes: form.notes }; modal.value.id ? await promotionAPI.updatePromoter(modal.value.id, payload) : await promotionAPI.createPromoter(payload) } else { const payload = { name: form.name, code: form.code.toUpperCase(), channel_type: form.channel_type || 'other', promoter_id: form.promoter_id, commission_rate: form.channel_rate === '' ? null : Number(form.channel_rate), enabled: form.enabled, notes: form.notes }; modal.value.id ? await promotionAPI.updateChannel(modal.value.id, payload) : await promotionAPI.createChannel(payload) } modal.value = null; await Promise.all([loadBase(), loadReport()]); appStore.showSuccess('推广配置已保存') } catch (error) { appStore.showError(errorMessage(error)) } finally { saving.value = false } }
function openSettlement() { const now = new Date(); now.setMinutes(now.getMinutes() - now.getTimezoneOffset()); settlementForm.value = { period_end: now.toISOString().slice(0, 16), notes: '' } }
async function createSettlement() { if (!settlementForm.value || !commissionPromoter.value) return; try { await promotionAPI.createSettlement({ promoter_id: commissionPromoter.value, period_end: new Date(settlementForm.value.period_end).toISOString(), notes: settlementForm.value.notes }); settlementForm.value = null; await loadCommissions(); appStore.showSuccess('结算单已生成，确认实际打款后再标记已支付') } catch (error) { appStore.showError(errorMessage(error)) } }
async function updateSettlement(id: number, status: 'paid' | 'cancelled') { try { await promotionAPI.updateSettlementStatus(id, status); await loadCommissions(); appStore.showSuccess(status === 'paid' ? '结算单已标记支付' : '结算单已取消') } catch (error) { appStore.showError(errorMessage(error)) } }
function outcomeLabel(value: PromotionAttributionEvent['outcome']) { return ({ attributed: '归因成功', already_attributed: '保留原归因', invalid_code: '无效编码', channel_disabled: '渠道已停用' })[value] }
function commissionStatusLabel(value: PromotionCommission['status']) { return ({ frozen: '冻结中', available: '可结算', settled: '已结算', reversed: '已冲正' })[value] }
function settlementStatusLabel(value: PromotionSettlement['status']) { return ({ draft: '待支付', paid: '已支付', cancelled: '已取消' })[value] }
watch(tab, value => { if (value === 'attribution') void loadEvents(); if (value === 'commission') void loadCommissions() })
onMounted(async () => { try { await Promise.all([loadBase(), loadReport()]) } catch (error) { appStore.showError(errorMessage(error)) } })
</script>
