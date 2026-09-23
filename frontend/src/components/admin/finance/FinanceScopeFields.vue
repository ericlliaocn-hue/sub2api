<template>
  <div class="grid gap-4 md:grid-cols-2">
    <label class="field md:col-span-2">
      <span>绑定账号</span>
      <div class="relative">
        <input
          v-model="accountQuery"
          class="input"
          type="search"
          :placeholder="selectedAccountLabel || '搜索邮箱或名称，买号回本要绑账号'"
          @focus="openAccountMenu"
          @input="onAccountInput"
        />
        <button
          v-if="modelValue.accountId"
          type="button"
          class="absolute right-2 top-1/2 -translate-y-1/2 text-xs text-gray-500"
          @click="clearAccount"
        >
          清除
        </button>
        <div
          v-if="accountMenuOpen"
          class="absolute z-20 mt-1 max-h-56 w-full overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-800"
        >
          <button
            v-for="account in accountOptions"
            :key="account.id"
            type="button"
            class="block w-full px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-dark-700"
            @mousedown.prevent="selectAccount(account)"
          >
            <div class="font-medium text-gray-900 dark:text-white">{{ account.name }}</div>
            <div class="text-xs text-gray-500">#{{ account.id }}</div>
          </button>
          <div v-if="accountSearching" class="px-3 py-2 text-xs text-gray-500">搜索中…</div>
          <div v-else-if="!accountOptions.length" class="px-3 py-2 text-xs text-gray-500">没有匹配的账号</div>
        </div>
      </div>
    </label>
    <label class="field">
      <span>分组（可选）</span>
      <div class="relative">
        <select :value="modelValue.groupId ?? ''" class="input appearance-none pr-10" @change="patch({ groupId: intOrNull(($event.target as HTMLSelectElement).value) })">
          <option value="">不限分组</option>
          <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
        </select>
        <svg class="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
      </div>
    </label>
    <label class="field">
      <span>渠道（可选）</span>
      <div class="relative">
        <select :value="modelValue.channelId ?? ''" class="input appearance-none pr-10" @change="patch({ channelId: intOrNull(($event.target as HTMLSelectElement).value) })">
          <option value="">不限渠道</option>
          <option v-for="channel in channels" :key="channel.id" :value="channel.id">{{ channel.name }}</option>
        </select>
        <svg class="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
      </div>
    </label>
    <label class="field md:col-span-2">
      <span>模型（可选）</span>
      <input :value="modelValue.model" class="input" maxlength="128" placeholder="例如 gpt-5.6-sol" @input="patch({ model: ($event.target as HTMLInputElement).value })" />
    </label>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { list as listAccounts, getById as getAccount } from '@/api/admin/accounts'
import type { AccountListItem } from '@/types'
import type { FinanceScopeFields } from '@/views/admin/businessFinanceScope'

const props = defineProps<{
  modelValue: FinanceScopeFields
  groups: { id: number; name: string }[]
  channels: { id: number; name: string }[]
}>()

const emit = defineEmits<{
  'update:modelValue': [FinanceScopeFields]
  'account-selected': [AccountListItem]
}>()

const accountQuery = ref('')
const accountOptions = ref<AccountListItem[]>([])
const accountMenuOpen = ref(false)
const accountSearching = ref(false)
const selectedAccount = ref<AccountListItem | null>(null)
let searchTimer: number | null = null
let searchGeneration = 0

const selectedAccountLabel = computed(() => {
  if (selectedAccount.value) return selectedAccount.value.name
  if (props.modelValue.accountId) return `账号 #${props.modelValue.accountId}`
  return ''
})

watch(() => props.modelValue.accountId, async (accountId) => {
  if (!accountId) {
    selectedAccount.value = null
    return
  }
  if (selectedAccount.value?.id === accountId) return
  const cached = accountOptions.value.find((item) => item.id === accountId)
  if (cached) {
    selectedAccount.value = cached
    return
  }
  try {
    selectedAccount.value = await getAccount(accountId)
  } catch {
    selectedAccount.value = null
  }
}, { immediate: true })

function patch(partial: Partial<FinanceScopeFields>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

function intOrNull(value: string) {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

function openAccountMenu() {
  accountMenuOpen.value = true
  if (!accountOptions.value.length) void searchAccounts(accountQuery.value)
}

function onAccountInput() {
  accountMenuOpen.value = true
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    void searchAccounts(accountQuery.value)
  }, 250)
}

async function searchAccounts(keyword: string) {
  const generation = ++searchGeneration
  accountSearching.value = true
  try {
    const data = await listAccounts(1, 20, { search: keyword.trim() || undefined, sort_by: 'id', sort_order: 'desc' })
    if (generation !== searchGeneration) return
    accountOptions.value = data.items || []
  } catch {
    if (generation !== searchGeneration) return
    accountOptions.value = []
  } finally {
    if (generation === searchGeneration) accountSearching.value = false
  }
}

function selectAccount(account: AccountListItem) {
  selectedAccount.value = account
  accountQuery.value = ''
  accountMenuOpen.value = false
  patch({ accountId: account.id })
  emit('account-selected', account)
}

function clearAccount() {
  selectedAccount.value = null
  accountQuery.value = ''
  patch({ accountId: null })
}

onBeforeUnmount(() => {
  if (searchTimer) window.clearTimeout(searchTimer)
})
</script>

<style scoped>
.field { display: flex; flex-direction: column; gap: 0.4rem; font-size: 0.875rem; color: rgb(107 114 128); }
.field span { font-weight: 500; }
</style>
