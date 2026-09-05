<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.bonusGrant.title')"
    width="normal"
    @close="emit('close')"
  >
    <form id="bonus-grant-form" class="space-y-5" @submit.prevent="handleSubmit">
      <div class="rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-700">
        <p class="font-medium text-gray-900 dark:text-gray-100">
          {{ singleUserEmail || t('admin.users.bonusGrant.selectedCount', { count: selectedIds.length }) }}
        </p>
        <p class="mt-1 text-gray-500 dark:text-gray-400">
          {{ t('admin.users.bonusGrant.atomicHint') }}
        </p>
      </div>

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <label class="input-label" for="bonus-grant-amount">
            {{ t('admin.users.bonusGrant.amount') }}
          </label>
          <div class="relative">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
            <input
              id="bonus-grant-amount"
              v-model.number="amount"
              data-test="bonus-amount"
              type="number"
              min="0.01"
              max="999999999999"
              step="0.01"
              class="input pl-8"
              required
            />
          </div>
        </div>
        <div>
          <label class="input-label" for="bonus-grant-expiry">
            {{ t('admin.users.bonusGrant.expiresAt') }}
          </label>
          <input
            id="bonus-grant-expiry"
            v-model="expiresAt"
            data-test="bonus-expiry"
            type="datetime-local"
            :min="minimumExpiry"
            class="input"
            required
          />
        </div>
      </div>

      <div>
        <label class="input-label" for="bonus-grant-campaign">
          {{ t('admin.users.bonusGrant.campaignId') }}
        </label>
        <input
          id="bonus-grant-campaign"
          v-model.trim="campaignId"
          data-test="bonus-campaign"
          type="text"
          maxlength="100"
          class="input"
          :placeholder="t('admin.users.bonusGrant.campaignPlaceholder')"
          required
        />
      </div>

      <div>
        <label class="input-label" for="bonus-grant-notes">
          {{ t('admin.users.notes') }}
        </label>
        <textarea
          id="bonus-grant-notes"
          v-model="notes"
          data-test="bonus-notes"
          rows="3"
          maxlength="255"
          class="input"
        ></textarea>
      </div>

      <div v-if="validAmount" class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm dark:border-amber-800 dark:bg-amber-950/40">
        <div class="flex justify-between gap-4">
          <span>{{ t('admin.users.bonusGrant.totalGranted') }}</span>
          <strong>${{ totalGranted.toFixed(2) }}</strong>
        </div>
        <p class="mt-2 text-amber-800 dark:text-amber-200">
          {{ t('admin.users.bonusGrant.expiryWarning') }}
        </p>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="bonus-grant-form"
          class="btn btn-primary"
          data-test="bonus-submit"
          :disabled="!canSubmit"
        >
          {{ submitting ? t('admin.users.bonusGrant.granting') : t('admin.users.bonusGrant.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  show: boolean
  selectedIds: number[]
  singleUserEmail?: string
}>()

const emit = defineEmits<{
  close: []
  success: [affected: number]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const amount = ref<number | null>(null)
const expiresAt = ref('')
const campaignId = ref('')
const notes = ref('')
const operationId = ref('')
const submitting = ref(false)

const toLocalDateTime = (date: Date) => {
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

const minimumExpiry = computed(() => toLocalDateTime(new Date(Date.now() + 60_000)))
const validAmount = computed(() => typeof amount.value === 'number' && Number.isFinite(amount.value) && amount.value > 0)
const totalGranted = computed(() => (validAmount.value ? amount.value! * props.selectedIds.length : 0))
const canSubmit = computed(() =>
  props.selectedIds.length > 0
  && props.selectedIds.length <= 500
  && validAmount.value
  && !!expiresAt.value
  && new Date(expiresAt.value).getTime() > Date.now()
  && !!campaignId.value.trim()
  && !submitting.value
)

const newOperationId = () => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

const reset = () => {
  amount.value = null
  const defaultExpiry = new Date()
  defaultExpiry.setDate(defaultExpiry.getDate() + 1)
  defaultExpiry.setHours(23, 59, 0, 0)
  expiresAt.value = toLocalDateTime(defaultExpiry)
  campaignId.value = `campaign-${new Date().toISOString().slice(0, 10).replace(/-/g, '')}`
  notes.value = ''
  operationId.value = newOperationId()
  submitting.value = false
}

watch(() => props.show, (show) => {
  if (show) reset()
}, { immediate: true })

const handleSubmit = async () => {
  if (!canSubmit.value || amount.value === null) return
  const confirmed = window.confirm(t('admin.users.bonusGrant.confirmMessage', {
    count: props.selectedIds.length,
    amount: amount.value.toFixed(2),
    total: totalGranted.value.toFixed(2),
    expiresAt: new Date(expiresAt.value).toLocaleString()
  }))
  if (!confirmed) return

  submitting.value = true
  try {
    const result = await adminAPI.users.grantExpiringBonus({
      user_ids: [...props.selectedIds],
      amount: amount.value,
      expires_at: new Date(expiresAt.value).toISOString(),
      campaign_id: campaignId.value.trim(),
      notes: notes.value.trim()
    }, operationId.value)
    appStore.showSuccess(t('admin.users.bonusGrant.success', { count: result.affected }))
    emit('success', result.affected)
    emit('close')
  } catch (error: any) {
    appStore.showError(
      error.response?.data?.message
      || error.response?.data?.detail
      || error.message
      || t('admin.users.bonusGrant.failed')
    )
  } finally {
    submitting.value = false
  }
}
</script>
