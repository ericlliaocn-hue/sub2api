<template>
  <div data-test="watched-traffic-card" class="rounded-lg border border-amber-200 bg-amber-50/70 px-5 py-4 shadow-sm dark:border-amber-900/60 dark:bg-amber-950/20">
    <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.watchedTraffic.title') }}</h2>
          <span
            class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
            :class="config.enabled ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300' : 'bg-gray-200 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
          >
            {{ config.enabled ? t('admin.watchedTraffic.on') : t('admin.watchedTraffic.off') }}
          </span>
        </div>
        <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.watchedTraffic.hint') }}</p>
      </div>
      <div class="flex items-center gap-3">
        <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.watchedTraffic.switchLabel') }}</span>
        <Toggle :model-value="config.enabled" :disabled="saving || loading" @update:model-value="toggleEnabled" />
      </div>
    </div>

    <div class="mt-4 grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
      <div>
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.watchedTraffic.userIds') }}</label>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.watchedTraffic.userIdsHint') }}</p>
        <input
          v-model="userIdsText"
          type="text"
          class="input mt-2"
          :placeholder="t('admin.watchedTraffic.userIdsPlaceholder')"
        />
      </div>
      <button type="button" class="btn btn-secondary" :disabled="saving || loading" @click="saveUserIds">
        {{ t('admin.watchedTraffic.saveUsers') }}
      </button>
    </div>

    <div class="mt-4">
      <div class="mb-2 flex items-center justify-between gap-2">
        <h3 class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.watchedTraffic.recent') }}</h3>
        <button type="button" class="text-xs text-primary-600 hover:underline" :disabled="logsLoading" @click="loadLogs">
          {{ t('admin.watchedTraffic.refresh') }}
        </button>
      </div>
      <div v-if="logsLoading" class="py-6 text-center text-sm text-gray-500">{{ t('admin.watchedTraffic.loading') }}</div>
      <div v-else-if="logs.length === 0" class="py-6 text-center text-sm text-gray-500">{{ t('admin.watchedTraffic.empty') }}</div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full text-left text-xs">
          <thead class="text-gray-500 dark:text-gray-400">
            <tr>
              <th class="py-2 pr-3 font-medium">{{ t('admin.watchedTraffic.time') }}</th>
              <th class="py-2 pr-3 font-medium">{{ t('admin.watchedTraffic.model') }}</th>
              <th class="py-2 pr-3 font-medium">{{ t('admin.watchedTraffic.status') }}</th>
              <th class="py-2 pr-3 font-medium">{{ t('admin.watchedTraffic.account') }}</th>
              <th class="py-2 pr-3 font-medium">{{ t('admin.watchedTraffic.prompt') }}</th>
              <th class="py-2 font-medium">{{ t('admin.watchedTraffic.response') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-amber-100 dark:divide-amber-900/40">
            <tr
              v-for="row in logs"
              :key="row.id"
              class="cursor-pointer align-top hover:bg-white/70 dark:hover:bg-dark-800/60"
              @click="detail = row"
            >
              <td class="whitespace-nowrap py-2 pr-3 text-gray-600 dark:text-gray-300">{{ formatTime(row.created_at) }}</td>
              <td class="max-w-[140px] truncate py-2 pr-3 font-mono text-gray-800 dark:text-gray-200">{{ row.model || '-' }}</td>
              <td class="py-2 pr-3 font-mono" :class="row.status_code >= 400 || row.error_text ? 'text-red-600' : 'text-gray-700 dark:text-gray-300'">
                {{ row.status_code }}
              </td>
              <td class="py-2 pr-3 font-mono text-gray-600 dark:text-gray-300">{{ row.account_id || '-' }}</td>
              <td class="max-w-[220px] truncate py-2 pr-3 text-gray-700 dark:text-gray-300">{{ preview(row.prompt_text) }}</td>
              <td class="max-w-[220px] truncate py-2 text-gray-700 dark:text-gray-300">{{ preview(row.error_text || row.response_text) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <BaseDialog :show="!!detail" :title="t('admin.watchedTraffic.detail')" @close="detail = null">
      <div v-if="detail" class="space-y-3 text-sm">
        <p class="text-xs text-gray-500">
          {{ detail.created_at }} · {{ detail.user_email || detail.user_id }} · {{ detail.api_key_name }} ·
          {{ detail.group_name }} · {{ t('admin.watchedTraffic.account') }} {{ detail.account_id || '-' }} ·
          {{ detail.model }} · HTTP {{ detail.status_code }}
        </p>
        <div>
          <p class="mb-1 font-medium text-gray-800 dark:text-gray-200">{{ t('admin.watchedTraffic.prompt') }}</p>
          <pre class="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 text-xs dark:bg-dark-800">{{ detail.prompt_text || '-' }}</pre>
        </div>
        <div>
          <p class="mb-1 font-medium text-gray-800 dark:text-gray-200">{{ t('admin.watchedTraffic.response') }}</p>
          <pre class="max-h-64 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 text-xs dark:bg-dark-800">{{ detail.response_text || '-' }}</pre>
        </div>
        <div v-if="detail.error_text">
          <p class="mb-1 font-medium text-red-700 dark:text-red-300">{{ t('admin.watchedTraffic.error') }}</p>
          <pre class="max-h-40 overflow-auto whitespace-pre-wrap rounded bg-red-50 p-3 text-xs dark:bg-red-950/30">{{ detail.error_text }}</pre>
        </div>
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { WatchedTrafficConfig, WatchedTrafficLog } from '@/api/admin/watchedTraffic'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const logsLoading = ref(false)
const userIdsText = ref('')
const config = ref<WatchedTrafficConfig>({ enabled: false, user_ids: [] })
const logs = ref<WatchedTrafficLog[]>([])
const detail = ref<WatchedTrafficLog | null>(null)

function parseUserIds(raw: string): number[] {
  return [...new Set(raw.split(/[,\s]+/).map((part) => Number(part.trim())).filter((id) => Number.isInteger(id) && id > 0))]
}

function formatUserIds(ids: number[]): string {
  return ids.join(', ')
}

function preview(text: string): string {
  const value = (text || '').replace(/\s+/g, ' ').trim()
  if (!value) return '-'
  return value.length > 80 ? `${value.slice(0, 80)}…` : value
}

function formatTime(value: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

async function loadConfig() {
  config.value = await adminAPI.watchedTraffic.getConfig()
  userIdsText.value = formatUserIds(config.value.user_ids || [])
}

async function loadLogs() {
  logsLoading.value = true
  try {
    const userID = config.value.user_ids[0]
    const result = await adminAPI.watchedTraffic.listLogs({
      user_id: userID,
      page: 1,
      page_size: 20,
    })
    logs.value = result.items || []
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.watchedTraffic.logsFailed')))
  } finally {
    logsLoading.value = false
  }
}

async function toggleEnabled(enabled: boolean) {
  saving.value = true
  try {
    config.value = await adminAPI.watchedTraffic.updateConfig({ enabled })
    appStore.showSuccess(enabled ? t('admin.watchedTraffic.enabled') : t('admin.watchedTraffic.disabled'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.watchedTraffic.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function saveUserIds() {
  saving.value = true
  try {
    config.value = await adminAPI.watchedTraffic.updateConfig({ user_ids: parseUserIds(userIdsText.value) })
    userIdsText.value = formatUserIds(config.value.user_ids || [])
    appStore.showSuccess(t('admin.watchedTraffic.saved'))
    await loadLogs()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.watchedTraffic.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  try {
    await loadConfig()
    await loadLogs()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.watchedTraffic.loadFailed')))
  } finally {
    loading.value = false
  }
})
</script>
