<template>
  <BaseDialog :show="show" :title="t('keys.poolPeers.title')" width="wide" @close="emit('close')">
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('keys.poolPeers.description') }}
      </p>

      <div v-if="loading" class="flex justify-center py-8">
        <Icon name="refresh" size="md" class="animate-spin text-primary-500" />
      </div>

      <div
        v-else-if="errorMessage"
        class="rounded-lg bg-gray-50 px-4 py-6 text-center text-sm text-gray-500 dark:bg-dark-700 dark:text-gray-400"
      >
        {{ errorMessage }}
      </div>

      <template v-else-if="board">
        <div class="rounded-lg bg-gray-50 px-4 py-3 dark:bg-dark-700">
          <div class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('keys.poolPeers.memberCount') }}
          </div>
          <div class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ board.member_count }}
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-left dark:border-dark-600">
                <th class="px-3 py-2 font-medium text-gray-500 dark:text-gray-400">
                  {{ t('keys.poolPeers.member') }}
                </th>
                <th class="px-3 py-2 font-medium text-gray-500 dark:text-gray-400">
                  {{ t('keys.poolPeers.firstCall') }}
                </th>
                <th class="px-3 py-2 font-medium text-gray-500 dark:text-gray-400">
                  {{ t('keys.poolPeers.lastCall') }}
                </th>
                <th class="px-3 py-2 text-right font-medium text-gray-500 dark:text-gray-400">
                  {{ t('keys.poolPeers.todayCalls') }}
                </th>
                <th class="px-3 py-2 text-right font-medium text-gray-500 dark:text-gray-400">
                  {{ t('keys.poolPeers.weekCalls') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="peer in board.peers"
                :key="peer.label"
                class="border-b border-gray-100 last:border-0 dark:border-dark-700"
                :class="{ 'bg-primary-50/50 dark:bg-primary-900/10': peer.is_self }"
              >
                <td class="px-3 py-2 text-gray-900 dark:text-white">
                  {{ peer.label }}
                  <span v-if="peer.is_self" class="ml-1 text-xs text-primary-600 dark:text-primary-400">
                    {{ t('keys.poolPeers.you') }}
                  </span>
                </td>
                <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                  {{ peer.first_call_at ? formatDateTime(peer.first_call_at) : '-' }}
                </td>
                <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                  {{ peer.last_call_at ? formatDateTime(peer.last_call_at) : '-' }}
                </td>
                <td class="px-3 py-2 text-right tabular-nums text-gray-900 dark:text-white">
                  {{ peer.today_calls }}
                </td>
                <td class="px-3 py-2 text-right tabular-nums text-gray-900 dark:text-white">
                  {{ peer.week_calls }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <p class="text-xs text-gray-400 dark:text-gray-500">
          {{ t('keys.poolPeers.windowHint', { hours: board.week_window_hours }) }}
        </p>
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { keysAPI, type PoolPeerBoard } from '@/api/keys'
import { formatDateTime } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  show: boolean
  keyId: number | null
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()

const loading = ref(false)
const board = ref<PoolPeerBoard | null>(null)
const errorMessage = ref('')

const loadBoard = async () => {
  if (!props.keyId) return
  loading.value = true
  errorMessage.value = ''
  board.value = null
  try {
    board.value = await keysAPI.getPoolPeers(props.keyId)
  } catch (error) {
    // 分组未开启子池时后端返回业务错误，这里当成「无数据」而不是故障。
    errorMessage.value = t('keys.poolPeers.unavailable')
    console.error('Failed to load pool peers:', error)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (visible) => {
    if (visible) loadBoard()
  }
)
</script>
