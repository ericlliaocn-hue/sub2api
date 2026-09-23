<template>
  <section
    data-testid="usage-pressure-board"
    class="px-4 py-5 sm:px-6"
  >
    <div v-if="loading && !snapshot" class="flex justify-center py-8">
      <LoadingSpinner />
    </div>
    <div v-else-if="failed && !snapshot" class="py-6 text-center text-sm text-gray-400">
      {{ t('admin.usage.pressure.loadFailed') }}
    </div>
    <div v-else-if="snapshot" class="space-y-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div class="flex items-center gap-2">
            <span
              class="inline-block h-2 w-2 rounded-full"
              :class="dotClass"
              aria-hidden="true"
            />
            <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-100">
              {{ t('admin.usage.pressure.title') }}
            </h3>
            <span class="text-[11px] uppercase tracking-wide text-gray-400 dark:text-gray-500">
              {{ t('admin.usage.pressure.live') }}
            </span>
          </div>
          <p class="mt-1 text-lg font-semibold leading-tight" :class="moodClass">
            {{ t(`admin.usage.pressure.levels.${snapshot.level}`) }}
          </p>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t(`admin.usage.pressure.levelHints.${snapshot.level}`) }}
          </p>
        </div>
        <div class="text-right text-xs text-gray-400 dark:text-gray-500">
          <div>{{ t('admin.usage.pressure.refreshHint') }}</div>
          <div class="mt-1 tabular-nums">
            {{ t('admin.usage.pressure.hourly') }}
            <span class="font-medium text-gray-700 dark:text-gray-200">${{ fmtCost(snapshot.hourly_from_15m) }}</span>
          </div>
          <div class="mt-0.5">
            <template v-if="snapshot.peak_hour">
              {{ t('admin.usage.pressure.vsPeak', { hour: formatHour(snapshot.peak_hour.hour) }) }}
              · ${{ fmtCost(snapshot.peak_hour.billed) }}
              · {{ t('admin.usage.pressure.peakRatio', { pct: peakPct }) }}
            </template>
            <template v-else>
              {{ t('admin.usage.pressure.vsPeakNone') }}
            </template>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div
          v-for="win in windowCards"
          :key="win.key"
          class="rounded-xl border border-gray-200/80 px-3 py-2.5 dark:border-dark-700"
        >
          <div class="text-[11px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">
            {{ t(win.label) }}
          </div>
          <div class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-gray-100">
            ${{ fmtCost(win.data.billed) }}
          </div>
          <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ win.data.users }} {{ t('admin.usage.pressure.users') }}
            · {{ win.data.requests }} {{ t('admin.usage.pressure.requests') }}
          </div>
        </div>
      </div>

      <div v-if="hideActors" class="text-sm text-gray-600 dark:text-gray-300" data-testid="pressure-crowd-only">
        {{ t('admin.usage.pressure.crowdOnly', {
          users: snapshot.windows.m15.users,
          accounts: snapshot.windows.m15.accounts,
        }) }}
      </div>
      <div v-else class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div>
          <div class="mb-1.5 text-[11px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">
            {{ t('admin.usage.pressure.users') }}
          </div>
          <div v-if="snapshot.users.length === 0" class="text-sm text-gray-400">
            {{ t('admin.usage.pressure.nobody') }}
          </div>
          <div v-else class="flex flex-wrap gap-2">
            <button
              v-for="user in snapshot.users"
              :key="user.id"
              type="button"
              class="inline-flex max-w-full items-center gap-1.5 rounded-full border border-gray-200 bg-white px-2.5 py-1 text-left text-xs text-gray-700 transition-colors hover:border-primary-300 hover:text-primary-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-primary-500/60 dark:hover:text-primary-300"
              :data-testid="`pressure-user-${user.id}`"
              @click="emit('select-user', user.id, user.name)"
            >
              <span class="truncate font-medium">{{ user.name || `#${user.id}` }}</span>
              <span class="tabular-nums text-gray-400">${{ fmtCost(user.billed) }}</span>
              <span class="text-gray-400">{{ relTime(user.last_at) }}</span>
            </button>
          </div>
        </div>
        <div>
          <div class="mb-1.5 text-[11px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">
            {{ t('admin.usage.pressure.accounts') }}
          </div>
          <div v-if="snapshot.accounts.length === 0" class="text-sm text-gray-400">
            {{ t('admin.usage.pressure.nobody') }}
          </div>
          <div v-else class="flex flex-wrap gap-2">
            <span
              v-for="account in snapshot.accounts"
              :key="account.id"
              class="inline-flex max-w-full items-center gap-1.5 rounded-full border border-gray-200 bg-gray-50 px-2.5 py-1 text-xs text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200"
              :data-testid="`pressure-account-${account.id}`"
            >
              <span class="truncate font-medium">{{ account.name || `#${account.id}` }}</span>
              <span class="tabular-nums text-gray-400">${{ fmtCost(account.billed) }}</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getUsagePressure, type UsagePressureSnapshot, type UsagePressureWindow } from '@/api/admin/dashboard'
import { getDashboardPressure } from '@/api/usage'
import { formatCostFixed } from '@/utils/format'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const props = withDefaults(defineProps<{
  source?: 'admin' | 'user'
}>(), {
  source: 'admin',
})

const emit = defineEmits<{
  (e: 'select-user', userId: number, email: string): void
}>()

const hideActors = computed(() => props.source === 'user')

const { t } = useI18n()

const POLL_MS = 30_000
const snapshot = ref<UsagePressureSnapshot | null>(null)
const loading = ref(false)
const failed = ref(false)
let timer: ReturnType<typeof setInterval> | undefined
let reqSeq = 0

const fmtCost = (v: number) => formatCostFixed(v, 2)

const moodClass = computed(() => {
  switch (snapshot.value?.level) {
    case 'scramble':
      return 'text-rose-600 dark:text-rose-300'
    case 'busy':
      return 'text-orange-600 dark:text-orange-300'
    case 'warm':
      return 'text-amber-600 dark:text-amber-300'
    default:
      return 'text-slate-600 dark:text-slate-300'
  }
})

const dotClass = computed(() => {
  switch (snapshot.value?.level) {
    case 'scramble':
      return 'bg-rose-500'
    case 'busy':
      return 'bg-orange-500'
    case 'warm':
      return 'bg-amber-400'
    default:
      return 'bg-slate-400'
  }
})

const peakPct = computed(() => Math.round((snapshot.value?.peak_ratio || 0) * 100))

const windowCards = computed(() => {
  const windows = snapshot.value?.windows
  if (!windows) return []
  const cards: { key: string; label: string; data: UsagePressureWindow }[] = [
    { key: 'm5', label: 'admin.usage.pressure.window5', data: windows.m5 },
    { key: 'm15', label: 'admin.usage.pressure.window15', data: windows.m15 },
    { key: 'm60', label: 'admin.usage.pressure.window60', data: windows.m60 },
  ]
  return cards
})

const formatHour = (iso: string) => {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '--:--'
  return `${String(date.getHours()).padStart(2, '0')}:00`
}

const relTime = (iso: string) => {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  const minutes = Math.max(0, Math.floor((Date.now() - date.getTime()) / 60_000))
  if (minutes < 1) return t('admin.usage.pressure.justNow')
  return t('admin.usage.pressure.minutesAgo', { n: minutes })
}

const load = async () => {
  const seq = ++reqSeq
  if (!snapshot.value) loading.value = true
  try {
    const data = props.source === 'user' ? await getDashboardPressure() : await getUsagePressure()
    if (seq !== reqSeq) return
    snapshot.value = data
    failed.value = false
  } catch {
    if (seq !== reqSeq) return
    failed.value = true
  } finally {
    if (seq === reqSeq) loading.value = false
  }
}

onMounted(() => {
  void load()
  timer = setInterval(() => { void load() }, POLL_MS)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>
