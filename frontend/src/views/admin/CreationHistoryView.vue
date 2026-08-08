<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">创作生成记录</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">查看用户、模型、参数、任务状态和生成结果。</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="load">
          <svg class="mr-1.5 h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20 11a8.1 8.1 0 0 0-14.9-4M4 5v4h4M4 13a8.1 8.1 0 0 0 14.9 4M20 19v-4h-4" />
          </svg>
          刷新
        </button>
      </div>

      <section class="card overflow-hidden">
        <div class="filter-toolbar flex flex-wrap items-center gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700 sm:px-5">
          <div class="relative min-w-[220px] flex-1 sm:max-w-sm">
            <svg class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <circle cx="11" cy="11" r="7" /><path stroke-linecap="round" d="m20 20-4-4" />
            </svg>
            <input v-model.trim="filters.search" type="search" class="input w-full pl-9" placeholder="搜索用户、模型、提示词或任务 ID" />
          </div>
          <Select v-model="filters.kind" :options="kindOptions" class="w-32" />
          <Select v-model="filters.status" :options="statusOptions" class="w-32" />
          <input v-model.trim="filters.model" type="search" class="input w-32" placeholder="模型名称" @keyup.enter="applyFilters" />
          <button type="button" class="filter-action btn btn-primary" :disabled="loading" @click="applyFilters">搜索</button>
          <button v-if="hasFilters" type="button" class="filter-action btn btn-ghost" @click="resetFilters">重置</button>
          <span class="ml-auto text-xs text-gray-400">共 {{ filteredTasks.length }} 条</span>
        </div>

        <div v-if="loading" class="p-10 text-center text-sm text-gray-500">正在加载记录…</div>
        <div v-else-if="!filteredTasks.length" class="p-14 text-center text-sm text-gray-500">暂无符合条件的创作记录</div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-[1120px] w-full table-fixed text-left text-sm">
            <colgroup>
              <col class="w-[150px]" /><col class="w-[190px]" /><col class="w-[82px]" /><col class="w-[160px]" />
              <col class="w-[180px]" /><col class="w-[320px]" /><col class="w-[112px]" /><col class="w-[150px]" /><col class="w-[148px]" />
            </colgroup>
            <thead class="bg-gray-50 text-xs font-medium text-gray-500 dark:bg-dark-800/80 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3">时间 / 任务</th>
                <th class="px-4 py-3">用户</th>
                <th class="px-4 py-3">类型</th>
                <th class="px-4 py-3">模型</th>
                <th class="px-4 py-3">Key / 分组</th>
                <th class="px-4 py-3">提示词</th>
                <th class="px-4 py-3">状态</th>
                <th class="px-4 py-3">参数</th>
                <th class="px-4 py-3">结果 / 操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="task in filteredTasks" :key="task.id" class="align-top transition-colors hover:bg-gray-50/80 dark:hover:bg-dark-800/50">
                <td class="px-4 py-4">
                  <div class="whitespace-nowrap text-xs text-gray-700 dark:text-gray-200">{{ formatDate(task.created_at) }}</div>
                  <div class="mt-1 text-xs text-gray-400">#{{ task.id }}<span v-if="task.provider_task_id"> · {{ task.provider_task_id }}</span></div>
                </td>
                <td class="px-4 py-4">
                  <div class="max-w-[170px] truncate font-medium text-gray-800 dark:text-gray-100" :title="task.user_email || `用户 #${task.user_id}`">{{ task.user_email || `用户 #${task.user_id}` }}</div>
                  <div class="mt-1 text-xs text-gray-400">用户 ID {{ task.user_id }}</div>
                </td>
                <td class="px-4 py-4">
                  <span class="inline-flex items-center gap-1.5 text-xs font-medium text-gray-700 dark:text-gray-200">
                    <span class="h-2 w-2 rounded-full" :class="task.kind === 'video' ? 'bg-violet-500' : 'bg-sky-500'" />
                    {{ task.kind === 'video' ? '视频' : '图片' }}
                  </span>
                </td>
                <td class="px-4 py-4"><span class="font-medium text-gray-800 dark:text-gray-100">{{ task.model || '未知模型' }}</span></td>
                <td class="px-4 py-4">
                  <div class="max-w-[165px] truncate text-xs font-medium text-gray-700 dark:text-gray-200" :title="task.api_key_name || `Key #${task.api_key_id}`">{{ task.api_key_name || `Key #${task.api_key_id || '-'}` }}</div>
                  <div class="mt-1 max-w-[165px] truncate text-xs text-gray-400" :title="task.group_name">{{ task.group_name || '未分组' }}</div>
                </td>
                <td class="px-4 py-4">
                  <p class="line-clamp-2 max-w-[300px] leading-5 text-gray-600 dark:text-gray-300" :title="task.prompt">{{ task.prompt || '（无提示词）' }}</p>
                  <button type="button" class="mt-1 text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="openDetails(task)">查看完整提示词</button>
                </td>
                <td class="px-4 py-4"><span class="inline-flex rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(task.status)">{{ statusLabel(task.status) }}</span><div v-if="task.error_message" class="mt-1 max-w-[105px] truncate text-xs text-red-500" :title="task.error_message">{{ task.error_message }}</div></td>
                <td class="px-4 py-4"><div class="flex flex-wrap gap-1.5"><span v-for="item in parameterSummary(task)" :key="item" class="rounded-md bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ item }}</span><span v-if="!parameterSummary(task).length" class="text-xs text-gray-400">-</span></div></td>
                <td class="px-4 py-4">
                  <div class="flex items-center gap-2">
                    <template v-if="task.assets.length">
                      <button v-for="asset in task.assets" :key="asset.id" type="button" class="h-10 w-12 overflow-hidden rounded-md bg-gray-100 dark:bg-dark-700" :title="`预览${asset.kind === 'video' ? '视频' : '图片'}`" @click="previewAsset(asset, task)">
                        <img v-if="asset.kind === 'image' && objectUrls.get(asset.id)" :src="objectUrls.get(asset.id)" class="h-full w-full object-cover" alt="生成结果" />
                        <span v-else class="flex h-full items-center justify-center text-[10px] font-medium text-gray-500">{{ asset.kind === 'video' ? 'VIDEO' : 'IMAGE' }}</span>
                      </button>
                    </template>
                    <span v-else class="text-xs text-gray-400">无结果</span>
                    <button v-if="task.assets.length" type="button" class="btn btn-secondary btn-sm whitespace-nowrap" :disabled="downloadingTaskId === task.id" @click="downloadTask(task)">{{ downloadingTaskId === task.id ? '处理中…' : '下载' }}</button>
                  </div>
                  <button type="button" class="mt-2 text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="openDetails(task)">详情</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="pagination.total_pages > 1" class="flex items-center justify-between border-t border-gray-100 px-5 py-4 text-sm dark:border-dark-700">
          <span class="text-gray-500">共 {{ pagination.total }} 条，第 {{ pagination.page }} / {{ pagination.total_pages }} 页</span>
          <div class="flex gap-2"><button type="button" class="btn btn-secondary btn-sm" :disabled="pagination.page <= 1 || loading" @click="changePage(pagination.page - 1)">上一页</button><button type="button" class="btn btn-secondary btn-sm" :disabled="pagination.page >= pagination.total_pages || loading" @click="changePage(pagination.page + 1)">下一页</button></div>
        </div>
      </section>
    </div>

    <div v-if="selectedTask" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="selectedTask = null">
      <section class="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl bg-white p-6 shadow-2xl dark:bg-dark-900">
        <div class="flex items-start justify-between gap-4"><div><div class="flex items-center gap-2"><span class="inline-flex rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(selectedTask.status)">{{ statusLabel(selectedTask.status) }}</span><span class="text-xs text-gray-400">#{{ selectedTask.id }}</span></div><h2 class="mt-3 text-lg font-semibold text-gray-900 dark:text-white">{{ selectedTask.model || '未知模型' }}</h2><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ selectedTask.user_email || `用户 #${selectedTask.user_id}` }} · {{ formatDate(selectedTask.created_at) }} · Key #{{ selectedTask.api_key_id }}</p></div><button type="button" class="btn btn-ghost" aria-label="关闭详情" @click="selectedTask = null">关闭</button></div>
        <div class="mt-5 grid gap-4 sm:grid-cols-3"><div class="detail-stat"><span>类型</span><strong>{{ selectedTask.kind === 'video' ? '视频' : '图片' }}</strong></div><div class="detail-stat"><span>完成时间</span><strong>{{ selectedTask.finished_at ? formatDate(selectedTask.finished_at) : '-' }}</strong></div><div class="detail-stat"><span>生成结果</span><strong>{{ selectedTask.assets.length }} 个文件</strong></div></div>
        <div class="mt-5 rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><div class="text-xs font-medium uppercase tracking-[0.14em] text-gray-400">完整提示词</div><p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-gray-700 dark:text-gray-200">{{ selectedTask.prompt || '（无提示词）' }}</p></div>
        <div class="mt-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-800"><div class="text-xs font-medium uppercase tracking-[0.14em] text-gray-400">生成参数</div><pre class="mt-2 overflow-x-auto whitespace-pre-wrap break-words text-xs leading-6 text-gray-700 dark:text-gray-200">{{ formatRequest(selectedTask) }}</pre></div>
        <div v-if="selectedTask.error_message" class="mt-4 rounded-xl bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ selectedTask.error_message }}</div>
        <div v-if="selectedTask.assets.length" class="mt-5 grid gap-3 sm:grid-cols-2"><div v-for="asset in selectedTask.assets" :key="asset.id" class="overflow-hidden rounded-xl bg-gray-100 dark:bg-dark-800"><img v-if="asset.kind === 'image' && objectUrls.get(asset.id)" :src="objectUrls.get(asset.id)" class="max-h-80 w-full object-contain" alt="生成结果" /><video v-else-if="asset.kind === 'video' && objectUrls.get(asset.id)" :src="objectUrls.get(asset.id)" class="max-h-80 w-full" controls playsinline preload="metadata" /><div v-else class="flex h-32 items-center justify-center text-sm text-gray-400">结果加载失败</div><button type="button" class="w-full border-t border-gray-200 px-3 py-2 text-left text-xs font-medium text-primary-600 dark:border-dark-700 dark:text-primary-400" @click="downloadAsset(asset, selectedTask)">下载此结果</button></div></div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { useAppStore } from '@/stores/app'
import { listAdminCreationTasks, loadAdminCreationAsset, type AdminCreationTask } from '@/api/admin/creation'
import type { CreationHistoryAsset } from '@/api/creationStudio'

const appStore = useAppStore()
const tasks = ref<AdminCreationTask[]>([])
const loading = ref(false)
const downloadingTaskId = ref<number | null>(null)
const selectedTask = ref<AdminCreationTask | null>(null)
const objectUrls = new Map<number, string>()
const filters = reactive({ search: '', kind: '', status: '', model: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0, total_pages: 0 })
const kindOptions: SelectOption[] = [
  { value: '', label: '所有类型' },
  { value: 'image', label: '图片' },
  { value: 'video', label: '视频' },
]
const statusOptions: SelectOption[] = [
  { value: '', label: '所有状态' },
  { value: 'completed', label: '已完成' },
  { value: 'processing', label: '生成中' },
  { value: 'queued', label: '排队中' },
  { value: 'failed', label: '失败' },
]

const hasFilters = computed(() => Boolean(filters.search || filters.kind || filters.status || filters.model))
const filteredTasks = computed(() => tasks.value)

function normalizeStatus(status: string) {
  const value = String(status || '').toLowerCase()
  if (['completed', 'succeeded', 'success'].includes(value)) return 'completed'
  if (['failed', 'error', 'cancelled'].includes(value)) return 'failed'
  if (['processing', 'running'].includes(value)) return 'processing'
  return 'queued'
}

function statusLabel(status: string) {
  return ({ completed: '已完成', failed: '失败', processing: '生成中', queued: '排队中' })[normalizeStatus(status)]
}

function statusClass(status: string) {
  return ({ completed: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300', failed: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300', processing: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300', queued: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300' })[normalizeStatus(status)]
}

function formatDate(value?: string) { return value ? new Date(value).toLocaleString() : '-' }

function parameterSummary(task: AdminCreationTask) {
  const request = task.request || {}
  const values: string[] = []
  if (typeof request.aspect_ratio === 'string') values.push(request.aspect_ratio)
  if (typeof request.resolution === 'string' && request.resolution !== 'auto') values.push(request.resolution)
  if (typeof request.quality === 'string' && request.quality !== 'auto') values.push(request.quality)
  if (typeof request.duration === 'number') values.push(`${request.duration}s`)
  if (request.reference_media_attached === true) values.push('参考素材')
  return values.slice(0, 4)
}

function formatRequest(task: AdminCreationTask) { return JSON.stringify(task.request || {}, null, 2) }
function openDetails(task: AdminCreationTask) { selectedTask.value = task }
function resetFilters() { filters.search = ''; filters.kind = ''; filters.status = ''; filters.model = '' }
async function applyFilters() { pagination.page = 1; await load() }

async function load() {
  loading.value = true
  try {
    const result = await listAdminCreationTasks(pagination.page, pagination.page_size, { ...filters })
    tasks.value = result.items || []
    Object.assign(pagination, result.pagination)
    await Promise.all(tasks.value.flatMap(task => task.assets.map(async asset => {
      if (!objectUrls.has(asset.id)) {
        try { objectUrls.set(asset.id, await loadAdminCreationAsset(asset.id)) } catch { /* Keep the record visible if a local asset is missing. */ }
      }
    })))
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '加载创作记录失败')
  } finally { loading.value = false }
}

async function changePage(page: number) { pagination.page = page; await load() }
function previewAsset(_asset: CreationHistoryAsset, task: AdminCreationTask) { selectedTask.value = task }

async function downloadAsset(asset: CreationHistoryAsset, task: AdminCreationTask) {
  try {
    const url = objectUrls.get(asset.id) || await loadAdminCreationAsset(asset.id)
    objectUrls.set(asset.id, url)
    const link = document.createElement('a'); link.href = url; link.download = `creation-${task.id}-${asset.kind}.${asset.kind === 'video' ? 'mp4' : 'png'}`; link.click()
  } catch (error) { appStore.showError(error instanceof Error ? error.message : '下载结果失败') }
}

async function downloadTask(task: AdminCreationTask) {
  downloadingTaskId.value = task.id
  try { for (const asset of task.assets) await downloadAsset(asset, task) } finally { downloadingTaskId.value = null }
}

onMounted(load)
onBeforeUnmount(() => { for (const url of objectUrls.values()) URL.revokeObjectURL(url) })
</script>

<style scoped>
.filter-toolbar { background: transparent; }
.filter-toolbar > * { flex: 0 0 auto; }
.filter-toolbar > .relative { flex: 1 1 18rem; }
.filter-action { height: 2.75rem; }
.detail-stat { border: 1px solid rgb(229 231 235 / 0.8); border-radius: 0.75rem; padding: 0.75rem 1rem; }
.detail-stat span { display: block; color: rgb(156 163 175); font-size: 0.72rem; }
.detail-stat strong { display: block; margin-top: 0.25rem; color: rgb(55 65 81); font-size: 0.875rem; }
:global(.dark) .detail-stat { border-color: rgb(75 85 99 / 0.5); }
:global(.dark) .detail-stat strong { color: rgb(229 231 235); }
</style>
