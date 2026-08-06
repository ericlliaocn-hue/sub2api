<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">创作生成记录</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">查看用户提示词、模型、任务状态和生成结果。</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="load">刷新</button>
      </div>

      <div class="card overflow-hidden">
        <div v-if="loading" class="p-8 text-center text-sm text-gray-500">正在加载记录…</div>
        <div v-else-if="!tasks.length" class="p-12 text-center text-sm text-gray-500">暂无创作记录</div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <article v-for="task in tasks" :key="task.id" class="p-5 sm:p-6">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(task.status)">{{ statusLabel(task.status) }}</span>
                  <span class="text-xs text-gray-400">#{{ task.id }}</span>
                  <span class="text-xs text-gray-400">{{ task.kind === 'video' ? '视频' : '图片' }}</span>
                </div>
                <h2 class="mt-3 font-medium text-gray-900 dark:text-white">{{ task.model || '未知模型' }}</h2>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ task.user_email || `用户 #${task.user_id}` }} · {{ formatDate(task.created_at) }} · Key #{{ task.api_key_id }}</p>
              </div>
              <div v-if="task.assets.length" class="flex flex-wrap gap-2">
                <button v-for="asset in task.assets" :key="asset.id" type="button" class="btn btn-secondary btn-sm" :disabled="downloadingAssetId === asset.id" @click="downloadAsset(asset, task)">
                  {{ downloadingAssetId === asset.id ? '处理中…' : '下载结果' }}
                </button>
              </div>
            </div>
            <div class="mt-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-800/70">
              <div class="text-xs font-medium uppercase tracking-[0.14em] text-gray-400">提示词</div>
              <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-gray-700 dark:text-gray-200">{{ task.prompt || '（无提示词）' }}</p>
            </div>
            <div v-if="task.error_message" class="mt-3 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ task.error_message }}</div>
            <div v-if="task.assets.length" class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
              <div v-for="asset in task.assets" :key="`preview-${asset.id}`" class="aspect-video overflow-hidden rounded-xl bg-gray-100 dark:bg-dark-800">
                <img v-if="assetPreview(asset).kind === 'image'" :src="assetPreview(asset).url" class="h-full w-full object-cover" alt="生成结果" />
                <video v-else :src="assetPreview(asset).url" class="h-full w-full object-cover" controls playsinline preload="metadata" />
              </div>
            </div>
          </article>
        </div>
        <div v-if="pagination.total_pages > 1" class="flex items-center justify-between border-t border-gray-100 px-5 py-4 text-sm dark:border-dark-700">
          <span class="text-gray-500">共 {{ pagination.total }} 条，第 {{ pagination.page }} / {{ pagination.total_pages }} 页</span>
          <div class="flex gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="pagination.page <= 1 || loading" @click="changePage(pagination.page - 1)">上一页</button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="pagination.page >= pagination.total_pages || loading" @click="changePage(pagination.page + 1)">下一页</button>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { listAdminCreationTasks, loadAdminCreationAsset, type AdminCreationTask } from '@/api/admin/creation'
import type { CreationHistoryAsset } from '@/api/creationStudio'

const appStore = useAppStore()
const tasks = ref<AdminCreationTask[]>([])
const loading = ref(false)
const downloadingAssetId = ref<number | null>(null)
const objectUrls = new Map<number, string>()
const pagination = reactive({ page: 1, page_size: 20, total: 0, total_pages: 0 })

function assetPreview(asset: CreationHistoryAsset) {
  return { kind: asset.kind, url: objectUrls.get(asset.id) || '' }
}

async function load() {
  loading.value = true
  try {
    const result = await listAdminCreationTasks(pagination.page, pagination.page_size)
    tasks.value = result.items || []
    Object.assign(pagination, result.pagination)
    await Promise.all(tasks.value.flatMap(task => task.assets.map(async asset => {
      if (!objectUrls.has(asset.id)) {
        try {
          objectUrls.set(asset.id, await loadAdminCreationAsset(asset.id))
        } catch {
          // Keep the record visible even if an old local asset was removed.
        }
      }
    })))
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '加载创作记录失败')
  } finally {
    loading.value = false
  }
}

async function changePage(page: number) {
  pagination.page = page
  await load()
}

function statusLabel(status: string) {
  const value = String(status || '').toLowerCase()
  if (['completed', 'succeeded', 'success'].includes(value)) return '已完成'
  if (['failed', 'error', 'cancelled'].includes(value)) return '失败'
  if (['processing', 'running'].includes(value)) return '生成中'
  return '排队中'
}

function statusClass(status: string) {
  const value = String(status || '').toLowerCase()
  if (['completed', 'succeeded', 'success'].includes(value)) return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  if (['failed', 'error', 'cancelled'].includes(value)) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

function formatDate(value: string) {
  return new Date(value).toLocaleString()
}

async function downloadAsset(asset: CreationHistoryAsset, task: AdminCreationTask) {
  downloadingAssetId.value = asset.id
  try {
    const url = objectUrls.get(asset.id) || await loadAdminCreationAsset(asset.id)
    objectUrls.set(asset.id, url)
    const link = document.createElement('a')
    link.href = url
    link.download = `creation-${task.id}-${asset.kind}.${asset.kind === 'video' ? 'mp4' : 'png'}`
    link.click()
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '下载结果失败')
  } finally {
    downloadingAssetId.value = null
  }
}

onMounted(load)
onBeforeUnmount(() => {
  for (const url of objectUrls.values()) URL.revokeObjectURL(url)
})
</script>
