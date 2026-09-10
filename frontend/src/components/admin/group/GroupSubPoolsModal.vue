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
        <button
          type="button"
          class="btn btn-sm btn-secondary ml-auto"
          :disabled="runningCooling"
          :title="t('admin.groups.subPools.cooling.hint')"
          @click="triggerCooling"
        >
          <Icon v-if="runningCooling" name="refresh" size="sm" class="mr-1 inline animate-spin" />
          {{ t('admin.groups.subPools.cooling.runNow') }}
        </button>
      </div>

      <p
        v-if="!group.sub_pool_enabled"
        class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-2.5 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ t('admin.groups.subPools.disabledHint') }}
      </p>

      <!-- 观察期毕业规则（全局，非本分组） -->
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-600">
        <div class="mb-2 flex items-center justify-between gap-2">
          <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.groups.subPools.graduation.title') }}
          </h4>
          <button
            type="button"
            role="switch"
            :aria-checked="graduation.enabled"
            :aria-label="t('admin.groups.subPools.graduation.title')"
            @click="graduation.enabled = !graduation.enabled"
            class="relative inline-flex h-6 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
            :class="graduation.enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'"
          >
            <span
              class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
              :class="graduation.enabled ? 'translate-x-6' : 'translate-x-1'"
            />
          </button>
        </div>
        <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.subPools.graduation.hint') }}
        </p>
        <div class="flex flex-wrap items-end gap-2">
          <label class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.subPools.graduation.probationDays') }}
            <input
              v-model.number="graduation.probation_days"
              type="number"
              min="1"
              max="365"
              step="1"
              class="hide-spinner input mt-1 w-24"
            />
          </label>
          <label class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.subPools.graduation.maxDailyCalls') }}
            <input
              v-model.number="graduation.max_daily_calls"
              type="number"
              min="0"
              step="1"
              class="hide-spinner input mt-1 w-32"
              :title="t('admin.groups.subPools.graduation.maxDailyCallsHint')"
            />
          </label>
          <button
            type="button"
            class="btn btn-sm btn-primary"
            :disabled="savingGraduation"
            @click="saveGraduation"
          >
            {{ t('common.save') }}
          </button>
          <button
            type="button"
            class="btn btn-sm btn-secondary"
            :disabled="runningGraduation || !graduation.enabled"
            @click="triggerGraduation"
          >
            <Icon v-if="runningGraduation" name="refresh" size="sm" class="mr-1 inline animate-spin" />
            {{ t('admin.groups.subPools.graduation.runNow') }}
          </button>
        </div>
      </div>

      <!-- Key 信誉分（全局，非本分组） -->
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-600">
        <div class="mb-2 flex items-center justify-between gap-2">
          <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.groups.subPools.reputation.title') }}
          </h4>
          <button
            type="button"
            role="switch"
            :aria-checked="reputation.enabled"
            :aria-label="t('admin.groups.subPools.reputation.title')"
            @click="reputation.enabled = !reputation.enabled"
            class="relative inline-flex h-6 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
            :class="reputation.enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'"
          >
            <span
              class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
              :class="reputation.enabled ? 'translate-x-6' : 'translate-x-1'"
            />
          </button>
        </div>
        <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.subPools.reputation.hint') }}
        </p>
        <div class="flex flex-wrap items-end gap-2">
          <label class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.subPools.reputation.windowDays') }}
            <input
              v-model.number="reputation.window_days"
              type="number"
              min="1"
              max="365"
              step="1"
              class="hide-spinner input mt-1 w-24"
            />
          </label>
          <label class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.subPools.reputation.demoteBelow') }}
            <input
              v-model.number="reputation.demote_below"
              type="number"
              min="1"
              max="100"
              step="1"
              class="hide-spinner input mt-1 w-24"
            />
          </label>
          <label class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.subPools.reputation.banBelow') }}
            <input
              v-model.number="reputation.ban_below"
              type="number"
              min="1"
              max="100"
              step="1"
              class="hide-spinner input mt-1 w-24"
            />
          </label>
          <button
            type="button"
            class="btn btn-sm btn-primary"
            :disabled="savingReputation"
            @click="saveReputation"
          >
            {{ t('common.save') }}
          </button>
          <button
            type="button"
            class="btn btn-sm btn-secondary"
            :disabled="runningReputation || !reputation.enabled"
            @click="triggerReputation"
          >
            <Icon
              v-if="runningReputation"
              name="refresh"
              size="sm"
              class="mr-1 inline animate-spin"
            />
            {{ t('admin.groups.subPools.reputation.runNow') }}
          </button>
          <button type="button" class="btn btn-sm btn-secondary" @click="loadWorstReputations">
            {{ t('admin.groups.subPools.reputation.showWorst') }}
          </button>
        </div>

        <div v-if="worstKeys.length > 0" class="mt-3 overflow-x-auto">
          <table class="w-full text-xs">
            <thead class="text-gray-500 dark:text-gray-400">
              <tr>
                <th class="py-1 pr-3 text-left font-medium">
                  {{ t('admin.groups.subPools.attribution.keyId') }}
                </th>
                <th class="py-1 pr-3 text-right font-medium">
                  {{ t('admin.groups.subPools.reputation.score') }}
                </th>
                <th class="py-1 pr-3 text-right font-medium">
                  {{ t('admin.groups.subPools.reputation.hits') }}
                </th>
                <th class="py-1 pr-3 text-left font-medium">
                  {{ t('admin.groups.subPools.reputation.sanction') }}
                </th>
                <th class="py-1 text-right font-medium">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in worstKeys"
                :key="row.api_key_id"
                class="border-t border-gray-100 dark:border-dark-700"
              >
                <td class="py-1 pr-3 font-mono">#{{ row.api_key_id }}</td>
                <td class="py-1 pr-3 text-right font-medium" :class="reputationScoreClass(row)">
                  {{ row.score }}
                </td>
                <td class="py-1 pr-3 text-right">{{ row.severe_hits }} / {{ row.total_hits }}</td>
                <td class="py-1 pr-3">
                  {{ t(`admin.groups.subPools.reputation.sanctions.${row.sanction}`) }}
                </td>
                <td class="py-1 text-right">
                  <button
                    v-if="row.sanction !== 'none'"
                    type="button"
                    class="btn btn-xs btn-secondary"
                    @click="clearSanction(row.api_key_id)"
                  >
                    {{ t('admin.groups.subPools.reputation.clearSanction') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

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
                @click="openAttribution(pool)"
              >
                {{ t('admin.groups.subPools.attribution.action') }}
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

          <!-- 归因报表：谁打的 -->
          <div
            v-if="attributionPoolId === pool.id"
            class="mt-3 border-t border-gray-100 pt-3 dark:border-dark-600"
          >
            <div class="mb-2 flex flex-wrap items-center gap-2">
              <h5 class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.groups.subPools.attribution.title') }}
              </h5>
              <Select v-model="attributionHours" :options="hoursOptions" class="w-32" />
              <button
                type="button"
                class="btn btn-sm btn-secondary"
                :disabled="attributionLoading"
                @click="loadAttribution(pool.id)"
              >
                {{ t('common.refresh') }}
              </button>
              <span
                v-if="attribution"
                class="text-xs text-gray-400"
              >
                {{
                  t('admin.groups.subPools.attribution.total', { calls: attribution.total_calls })
                }}
              </span>
            </div>

            <div v-if="attributionLoading" class="py-3 text-center">
              <Icon name="refresh" size="sm" class="inline animate-spin text-primary-500" />
            </div>
            <div
              v-else-if="!attribution || attribution.keys.length === 0"
              class="py-3 text-sm text-gray-400"
            >
              {{ t('admin.groups.subPools.attribution.empty') }}
            </div>
            <div v-else class="overflow-x-auto">
              <table class="min-w-full text-sm">
                <thead>
                  <tr class="border-b border-gray-200 text-left dark:border-dark-600">
                    <th class="px-2 py-1.5 font-medium text-gray-500 dark:text-gray-400">Key</th>
                    <th class="px-2 py-1.5 text-right font-medium text-gray-500 dark:text-gray-400">
                      {{ t('admin.groups.subPools.attribution.calls') }}
                    </th>
                    <th class="px-2 py-1.5 text-right font-medium text-gray-500 dark:text-gray-400">
                      {{ t('admin.groups.subPools.attribution.share') }}
                    </th>
                    <th class="px-2 py-1.5 text-right font-medium text-gray-500 dark:text-gray-400">
                      {{ t('admin.groups.subPools.attribution.violations') }}
                    </th>
                    <th class="px-2 py-1.5 font-medium text-gray-500 dark:text-gray-400">
                      {{ t('admin.groups.subPools.attribution.lastCall') }}
                    </th>
                    <th class="px-2 py-1.5"></th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="row in attribution.keys"
                    :key="row.api_key_id"
                    class="border-b border-gray-100 last:border-0 dark:border-dark-700"
                    :class="{ 'bg-red-50/50 dark:bg-red-900/10': row.suspect }"
                  >
                    <td class="px-2 py-1.5">
                      <span class="text-gray-900 dark:text-white">#{{ row.api_key_id }}</span>
                      <span class="ml-1 text-xs text-gray-400">u{{ row.user_id }}</span>
                      <span
                        v-if="row.suspect"
                        class="badge badge-danger ml-1"
                        :title="row.suspect_why"
                      >
                        {{ t('admin.groups.subPools.attribution.suspect') }}
                      </span>
                    </td>
                    <td class="px-2 py-1.5 text-right tabular-nums text-gray-900 dark:text-white">
                      {{ row.calls }}
                    </td>
                    <td class="px-2 py-1.5 text-right tabular-nums text-gray-600 dark:text-gray-300">
                      {{ (row.share * 100).toFixed(1) }}%
                    </td>
                    <td
                      class="px-2 py-1.5 text-right tabular-nums"
                      :class="
                        row.violations > 0
                          ? 'text-red-600 dark:text-red-400'
                          : 'text-gray-600 dark:text-gray-300'
                      "
                    >
                      {{ row.violations }}
                    </td>
                    <td class="px-2 py-1.5 text-xs text-gray-500 dark:text-gray-400">
                      {{ row.last_call_at ? formatDateTime(row.last_call_at) : '-' }}
                    </td>
                    <td class="px-2 py-1.5">
                      <div class="flex justify-end gap-1">
                        <button
                          type="button"
                          class="btn btn-sm btn-secondary"
                          :disabled="sanctioningKeyId === row.api_key_id"
                          @click="demoteKey(pool, row.api_key_id)"
                        >
                          {{ t('admin.groups.subPools.attribution.demote') }}
                        </button>
                        <button
                          type="button"
                          class="btn btn-sm btn-danger"
                          :disabled="sanctioningKeyId === row.api_key_id"
                          @click="confirmDisable(pool, row.api_key_id)"
                        >
                          {{ t('admin.groups.subPools.attribution.disable') }}
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
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
      :show="disableTarget !== null"
      :title="t('admin.groups.subPools.attribution.disableTitle')"
      :message="
        t('admin.groups.subPools.attribution.disableMessage', { id: disableTarget?.keyId || 0 })
      "
      danger
      @confirm="handleDisable"
      @cancel="disableTarget = null"
    />

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
import type {
  SubPool,
  SubPoolAttributionReport,
  SubPoolGraduationPolicy,
  ReputationPolicy,
  APIKeyReputation,
  SubPoolKind
} from '@/api/admin/subPools'
import { formatDateTime } from '@/utils/format'
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

const attributionPoolId = ref<number | null>(null)
const attributionLoading = ref(false)
const attributionHours = ref(24)
const attribution = ref<SubPoolAttributionReport | null>(null)
const sanctioningKeyId = ref<number | null>(null)
const disableTarget = ref<{ pool: SubPool; keyId: number } | null>(null)

const savingGraduation = ref(false)
const runningGraduation = ref(false)
const runningCooling = ref(false)
const graduation = reactive<SubPoolGraduationPolicy>({
  enabled: false,
  probation_days: 7,
  max_daily_calls: 0
})

const savingReputation = ref(false)
const runningReputation = ref(false)
const worstKeys = ref<APIKeyReputation[]>([])
const reputation = reactive<ReputationPolicy>({
  enabled: false,
  window_days: 30,
  demote_below: 60,
  ban_below: 25
})

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

const hoursOptions = computed(() => [
  { value: 1, label: t('admin.groups.subPools.attribution.lastHours', { hours: 1 }) },
  { value: 6, label: t('admin.groups.subPools.attribution.lastHours', { hours: 6 }) },
  { value: 24, label: t('admin.groups.subPools.attribution.lastHours', { hours: 24 }) },
  { value: 72, label: t('admin.groups.subPools.attribution.lastHours', { hours: 72 }) },
  { value: 168, label: t('admin.groups.subPools.attribution.lastHours', { hours: 168 }) }
])

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

const loadGraduation = async () => {
  try {
    Object.assign(graduation, await adminAPI.subPools.getGraduationPolicy())
  } catch (error) {
    console.error('Error loading graduation policy:', error)
  }
}

const saveGraduation = async () => {
  savingGraduation.value = true
  try {
    Object.assign(graduation, await adminAPI.subPools.updateGraduationPolicy({ ...graduation }))
    appStore.showSuccess(t('common.saved'))
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.graduation.saveFailed'))
    console.error('Error saving graduation policy:', error)
  } finally {
    savingGraduation.value = false
  }
}

const triggerCooling = async () => {
  runningCooling.value = true
  try {
    const result = await adminAPI.subPools.runCooling()
    appStore.showSuccess(
      t('admin.groups.subPools.cooling.runSuccess', {
        cooled: result.cooled,
        recovered: result.recovered,
        migrated: result.migrated
      })
    )
    await loadPools()
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.cooling.runFailed'))
    console.error('Error running cooling sweep:', error)
  } finally {
    runningCooling.value = false
  }
}

const reputationScoreClass = (row: APIKeyReputation) => {
  if (row.score < reputation.ban_below) return 'text-red-600 dark:text-red-400'
  if (row.score < reputation.demote_below) return 'text-amber-600 dark:text-amber-400'
  return 'text-gray-700 dark:text-gray-300'
}

const loadReputation = async () => {
  try {
    Object.assign(reputation, await adminAPI.subPools.getReputationPolicy())
  } catch (error) {
    console.error('Error loading reputation policy:', error)
  }
}

const saveReputation = async () => {
  savingReputation.value = true
  try {
    Object.assign(reputation, await adminAPI.subPools.updateReputationPolicy({ ...reputation }))
    appStore.showSuccess(t('common.saved'))
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.reputation.saveFailed'))
    console.error('Error saving reputation policy:', error)
  } finally {
    savingReputation.value = false
  }
}

const loadWorstReputations = async () => {
  try {
    const { items } = await adminAPI.subPools.worstReputations()
    worstKeys.value = items
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.reputation.loadWorstFailed'))
    console.error('Error loading worst reputations:', error)
  }
}

const triggerReputation = async () => {
  runningReputation.value = true
  try {
    const result = await adminAPI.subPools.runReputation()
    appStore.showSuccess(
      t('admin.groups.subPools.reputation.runSuccess', {
        scored: result.scored,
        demoted: result.demoted,
        disabled: result.disabled
      })
    )
    await loadWorstReputations()
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.reputation.runFailed'))
    console.error('Error running reputation sweep:', error)
  } finally {
    runningReputation.value = false
  }
}

const clearSanction = async (keyId: number) => {
  try {
    await adminAPI.subPools.clearReputationSanction(keyId)
    appStore.showSuccess(t('admin.groups.subPools.reputation.clearSuccess'))
    await loadWorstReputations()
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.reputation.clearFailed'))
    console.error('Error clearing reputation sanction:', error)
  }
}

const triggerGraduation = async () => {
  runningGraduation.value = true
  try {
    const result = await adminAPI.subPools.runGraduation()
    appStore.showSuccess(
      t('admin.groups.subPools.graduation.runSuccess', { count: result.graduated })
    )
    await loadPools()
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.graduation.runFailed'))
    console.error('Error running graduation:', error)
  } finally {
    runningGraduation.value = false
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

const openAttribution = (pool: SubPool) => {
  closeAccountEditor()
  migratePoolId.value = null
  attributionPoolId.value = pool.id
  attribution.value = null
  loadAttribution(pool.id)
}

const loadAttribution = async (poolId: number) => {
  attributionLoading.value = true
  try {
    attribution.value = await adminAPI.subPools.attribution(poolId, attributionHours.value)
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.attribution.loadFailed'))
    console.error('Error loading attribution report:', error)
  } finally {
    attributionLoading.value = false
  }
}

const demoteKey = async (pool: SubPool, keyId: number) => {
  sanctioningKeyId.value = keyId
  try {
    await adminAPI.subPools.demoteKey(keyId)
    appStore.showSuccess(t('admin.groups.subPools.attribution.demoteSuccess', { id: keyId }))
    await Promise.all([loadPools(), loadAttribution(pool.id)])
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.attribution.demoteFailed'))
    console.error('Error demoting key:', error)
  } finally {
    sanctioningKeyId.value = null
  }
}

const confirmDisable = (pool: SubPool, keyId: number) => {
  disableTarget.value = { pool, keyId }
}

const handleDisable = async () => {
  const target = disableTarget.value
  disableTarget.value = null
  if (!target) return
  sanctioningKeyId.value = target.keyId
  try {
    await adminAPI.subPools.disableKey(target.keyId)
    appStore.showSuccess(
      t('admin.groups.subPools.attribution.disableSuccess', { id: target.keyId })
    )
    await loadAttribution(target.pool.id)
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.subPools.attribution.disableFailed'))
    console.error('Error disabling key:', error)
  } finally {
    sanctioningKeyId.value = null
  }
}

const openMigrate = (pool: SubPool) => {
  closeAccountEditor()
  attributionPoolId.value = null
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
  attributionPoolId.value = null
  attribution.value = null
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
      loadGraduation()
      loadReputation()
    }
  }
)
</script>
