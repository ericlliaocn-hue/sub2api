<template>
  <BaseDialog
    :show="show"
    :title="t('admin.groups.subPools.title')"
    width="extra-wide"
    @close="handleClose"
  >
    <div v-if="group" class="space-y-4">
      <!-- 分组信息 -->
      <div
        class="flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 px-4 py-2.5 text-sm dark:bg-dark-700"
      >
        <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
        <span class="text-gray-400">|</span>
        <span
          class="badge"
          :class="group.sub_pool_enabled ? 'badge-success' : 'badge-gray'"
        >
          {{
            group.sub_pool_enabled
              ? t('admin.groups.subPools.enabled')
              : t('admin.groups.subPools.disabled')
          }}
        </span>
        <span class="text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.subPools.groupAccounts', { count: groupAccountCount }) }}
        </span>
      </div>

      <p
        v-if="!group.sub_pool_enabled"
        class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-2.5 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ t('admin.groups.subPools.disabledHint') }}
      </p>

      <!-- 新建子池 -->
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-600">
        <h4 class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.groups.subPools.createPool') }}
        </h4>
        <div class="flex flex-wrap items-end gap-2">
          <div class="min-w-[180px] flex-1">
            <input
              v-model="newPool.name"
              type="text"
              autocomplete="off"
              class="input w-full"
              :placeholder="t('admin.groups.subPools.namePlaceholder')"
            />
          </div>
          <div class="w-36">
            <Select v-model="newPool.kind" :options="kindOptions" />
          </div>
          <div class="w-28">
            <input
              v-model.number="newPool.key_soft_limit"
              type="number"
              min="0"
              step="1"
              autocomplete="off"
              class="hide-spinner input w-full"
              :title="t('admin.groups.subPools.keySoftLimitHint')"
            />
          </div>
          <button
            type="button"
            class="btn btn-primary shrink-0"
            :disabled="creating || !newPool.name.trim()"
            @click="handleCreate"
          >
            <Icon v-if="creating" name="refresh" size="sm" class="mr-1 inline animate-spin" />
            {{ t('common.add') }}
          </button>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="flex justify-center py-6">
        <Icon name="refresh" size="md" class="animate-spin text-primary-500" />
      </div>

      <!-- 子池列表 -->
      <div v-else-if="pools.length === 0" class="py-8 text-center text-sm text-gray-400 dark:text-gray-500">
        {{ t('admin.groups.subPools.empty') }}
      </div>

      <div v-else class="space-y-3">
        <div
          v-for="pool in pools"
          :key="pool.id"
          class="rounded-lg border border-gray-200 p-3 dark:border-dark-600"
        >
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium text-gray-900 dark:text-white">{{ pool.name }}</span>
            <span class="badge" :class="kindBadgeClass(pool.kind)">
              {{ t('admin.groups.subPools.kinds.' + pool.kind) }}
            </span>
            <span class="badge" :class="statusBadgeClass(pool.status)">
              {{ t('admin.groups.subPools.statuses.' + pool.status) }}
            </span>
            <span class="text-sm text-gray-500 dark:text-gray-400">
              {{
                t('admin.groups.subPools.poolSummary', {
                  keys: pool.bound_keys,
                  limit: pool.key_soft_limit || '∞',
                  accounts: pool.account_ids.length
                })
              }}
            </span>

            <div class="ml-auto flex items-center gap-1">
              <button
                type="button"
                class="btn btn-sm btn-secondary"
                @click="openAccountEditor(pool)"
              >
                {{ t('admin.groups.subPools.editAccounts') }}
              </button>
              <button
                type="button"
                class="btn btn-sm btn-secondary"
                @click="openMigrate(pool)"
              >
                {{ t('admin.groups.subPools.migrate') }}
              </button>
              <button
                type="button"
                class="btn btn-sm btn-secondary"
                :disabled="savingPoolId === pool.id"
                @click="toggleCooling(pool)"
              >
                {{
                  pool.status === 'cooling'
                    ? t('admin.groups.subPools.resume')
                    : t('admin.groups.subPools.cool')
                }}
              </button>
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('common.delete')"
                @click="confirmDelete(pool)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>

          <p v-if="pool.cooling_reason" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
            {{ pool.cooling_reason }}
          </p>

          <!-- 账号编辑 -->
          <div
            v-if="accountEditorPoolId === pool.id"
            class="mt-3 border-t border-gray-100 pt-3 dark:border-dark-600"
          >
            <h5 class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.groups.subPools.pickAccounts') }}
            </h5>
            <div v-if="accountsLoading" class="py-3 text-center text-sm text-gray-400">
              <Icon name="refresh" size="sm" class="inline animate-spin" />
            </div>
            <div v-else-if="groupAccounts.length === 0" class="py-3 text-sm text-gray-400">
              {{ t('admin.groups.subPools.noGroupAccounts') }}
            </div>
            <div v-else class="max-h-52 space-y-1 overflow-auto">
              <label
                v-for="account in groupAccounts"
                :key="account.id"
                class="flex items-center gap-2 rounded px-2 py-1 text-sm hover:bg-gray-50 dark:hover:bg-dark-700"
                :class="{ 'opacity-50': isAccountTakenElsewhere(account.id, pool.id) }"
              >
                <input
                  type="checkbox"
                  :value="account.id"
                  v-model="accountDraft"
                  :disabled="isAccountTakenElsewhere(account.id, pool.id)"
                />
                <span class="text-gray-900 dark:text-white">{{ account.name }}</span>
                <span class="text-xs text-gray-400">#{{ account.id }}</span>
                <span
                  v-if="isAccountTakenElsewhere(account.id, pool.id)"
                  class="text-xs text-gray-400"
                >
                  {{ t('admin.groups.subPools.accountTaken') }}
                </span>
              </label>
            </div>
            <div class="mt-2 flex justify-end gap-2">
              <button type="button" class="btn btn-sm btn-secondary" @click="closeAccountEditor">
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="btn btn-sm btn-primary"
                :disabled="savingPoolId === pool.id"
                @click="saveAccounts(pool)"
              >
                {{ t('common.save') }}
              </button>
            </div>
          </div>

          <!-- 迁移 -->
          <div
            v-if="migratePoolId === pool.id"
            class="mt-3 border-t border-gray-100 pt-3 dark:border-dark-600"
          >
            <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.groups.subPools.migrateHint') }}
            </p>
            <input
              v-model="migrateReason"
              type="text"
              class="input w-full"
              :placeholder="t('admin.groups.subPools.migrateReasonPlaceholder')"
            />
            <input
              v-model="migrateSuspects"
              type="text"
              class="input mt-2 w-full"
              :placeholder="t('admin.groups.subPools.suspectKeysPlaceholder')"
            />
            <div class="mt-2 flex justify-end gap-2">
              <button type="button" class="btn btn-sm btn-secondary" @click="migratePoolId = null">
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="btn btn-sm btn-primary"
                :disabled="savingPoolId === pool.id || !migrateReason.trim()"
                @click="runMigrate(pool)"
              >
                {{ t('admin.groups.subPools.migrate') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :show="deleteTarget !== null"
      :title="t('admin.groups.subPools.deleteTitle')"
      :message="t('admin.groups.subPools.deleteMessage', { name: deleteTarget?.name || '' })"
      danger
      @confirm="handleDelete"
      @cancel="deleteTarget = null"
    />
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { SubPool, SubPoolKind } from '@/api/admin/subPools'
import type { Account, AdminGroup } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const creating = ref(false)
const savingPoolId = ref<number | null>(null)
const pools = ref<SubPool[]>([])

const accountsLoading = ref(false)
const groupAccounts = ref<Account[]>([])
const accountEditorPoolId = ref<number | null>(null)
const accountDraft = ref<number[]>([])

const migratePoolId = ref<number | null>(null)
const migrateReason = ref('')
const migrateSuspects = ref('')

const deleteTarget = ref<SubPool | null>(null)

const newPool = reactive({
  name: '',
  kind: 'formal' as SubPoolKind,
  key_soft_limit: 8
})

const kindOptions = computed(() => [
  { value: 'formal', label: t('admin.groups.subPools.kinds.formal') },
  { value: 'probe', label: t('admin.groups.subPools.kinds.probe') }
])

const groupAccountCount = computed(() => groupAccounts.value.length)

const kindBadgeClass = (kind: SubPoolKind) =>
  kind === 'probe' ? 'badge-warning' : 'badge-gray'

const statusBadgeClass = (status: SubPool['status']) => {
  switch (status) {
    case 'healthy':
      return 'badge-success'
    case 'cooling':
      return 'badge-danger'
    default:
      return 'badge-gray'
  }
}

// 一个账号在同一分组内只能属于一个子池，否则两个池共享爆炸半径、归因就糊了。
const isAccountTakenElsewhere = (accountId: number, poolId: number) =>
  pools.value.some((pool) => pool.id !== poolId && pool.account_ids.includes(accountId))

const loadPools = async () => {
  if (!props.group) return
  loading.value = true
  try {
    pools.value = await adminAPI.subPools.listByGroup(props.group.id)
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.loadFailed'))
    console.error('Error loading sub-pools:', error)
  } finally {
    loading.value = false
  }
}

const loadGroupAccounts = async () => {
  if (!props.group) return
  accountsLoading.value = true
  try {
    const result = await adminAPI.accounts.list(1, 200, {
      group: String(props.group.id),
      lite: 'true'
    })
    groupAccounts.value = result.items ?? []
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.loadFailed'))
    console.error('Error loading group accounts:', error)
  } finally {
    accountsLoading.value = false
  }
}

const handleCreate = async () => {
  if (!props.group || !newPool.name.trim()) return
  creating.value = true
  try {
    await adminAPI.subPools.create(props.group.id, {
      name: newPool.name.trim(),
      kind: newPool.kind,
      key_soft_limit: newPool.key_soft_limit
    })
    newPool.name = ''
    await loadPools()
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.createFailed'))
    console.error('Error creating sub-pool:', error)
  } finally {
    creating.value = false
  }
}

const openAccountEditor = (pool: SubPool) => {
  migratePoolId.value = null
  accountEditorPoolId.value = pool.id
  accountDraft.value = [...pool.account_ids]
  if (groupAccounts.value.length === 0) {
    loadGroupAccounts()
  }
}

const closeAccountEditor = () => {
  accountEditorPoolId.value = null
  accountDraft.value = []
}

const saveAccounts = async (pool: SubPool) => {
  savingPoolId.value = pool.id
  try {
    await adminAPI.subPools.setAccounts(pool.id, accountDraft.value)
    closeAccountEditor()
    await loadPools()
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.saveAccountsFailed'))
    console.error('Error saving sub-pool accounts:', error)
  } finally {
    savingPoolId.value = null
  }
}

const openMigrate = (pool: SubPool) => {
  closeAccountEditor()
  migratePoolId.value = pool.id
  migrateReason.value = ''
  migrateSuspects.value = ''
}

const parseSuspectKeyIds = (raw: string): number[] =>
  raw
    .split(/[,\s]+/)
    .map((part) => Number.parseInt(part, 10))
    .filter((id) => Number.isFinite(id) && id > 0)

const runMigrate = async (pool: SubPool) => {
  savingPoolId.value = pool.id
  try {
    const result = await adminAPI.subPools.migrateCleanKeys(
      pool.id,
      migrateReason.value.trim(),
      parseSuspectKeyIds(migrateSuspects.value)
    )
    appStore.showSuccess(t('admin.groups.subPools.migrateSuccess', { count: result.moved }))
    migratePoolId.value = null
    await loadPools()
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.migrateFailed'))
    console.error('Error migrating sub-pool keys:', error)
  } finally {
    savingPoolId.value = null
  }
}

const toggleCooling = async (pool: SubPool) => {
  savingPoolId.value = pool.id
  try {
    await adminAPI.subPools.update(pool.id, {
      name: pool.name,
      description: pool.description,
      kind: pool.kind,
      status: pool.status === 'cooling' ? 'healthy' : 'cooling',
      key_soft_limit: pool.key_soft_limit,
      sort_order: pool.sort_order,
      cooling_until: null,
      cooling_reason: pool.status === 'cooling' ? null : pool.cooling_reason
    })
    await loadPools()
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.saveFailed'))
    console.error('Error updating sub-pool:', error)
  } finally {
    savingPoolId.value = null
  }
}

const confirmDelete = (pool: SubPool) => {
  deleteTarget.value = pool
}

const handleDelete = async () => {
  const pool = deleteTarget.value
  if (!pool) return
  try {
    await adminAPI.subPools.remove(pool.id)
    await loadPools()
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.deleteFailed'))
    console.error('Error deleting sub-pool:', error)
  } finally {
    deleteTarget.value = null
  }
}

const handleClose = () => {
  closeAccountEditor()
  migratePoolId.value = null
  emit('close')
}

watch(
  () => props.show,
  (visible) => {
    if (visible && props.group) {
      pools.value = []
      groupAccounts.value = []
      newPool.name = ''
      newPool.kind = 'formal'
      newPool.key_soft_limit = 8
      loadPools()
      loadGroupAccounts()
    }
  }
)
</script>
