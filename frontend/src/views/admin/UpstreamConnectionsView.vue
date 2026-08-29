<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl space-y-6 p-4 sm:p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.upstreamConnections.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.upstreamConnections.description') }}</p>
        </div>
        <div class="flex gap-2">
          <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadConnections">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button class="btn btn-primary" @click="openCreate">
            <Icon name="plus" size="md" class="mr-2" />
            {{ t('admin.upstreamConnections.add') }}
          </button>
        </div>
      </div>

      <div class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div v-if="loading" class="p-8 text-center text-sm text-gray-500">{{ t('admin.upstreamConnections.loading') }}</div>
        <div v-else-if="connections.length === 0" class="p-12 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.upstreamConnections.empty') }}</div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th v-for="label in headers" :key="label" class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ label }}</th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('admin.upstreamConnections.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
              <tr v-for="connection in connections" :key="connection.id">
                <td class="px-4 py-3">
                  <div class="font-medium text-gray-900 dark:text-white">{{ connection.name }}</div>
                  <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ connection.base_url }}</div>
                </td>
                <td class="px-4 py-3"><span class="badge badge-gray">{{ typeLabel(connection.type) }}</span></td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">{{ connection.username }}</td>
                <td class="px-4 py-3"><span :class="['badge', statusClass(connection.status)]">{{ statusLabel(connection.status) }}</span></td>
                <td class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">{{ connection.last_test_at ? formatDateTime(connection.last_test_at) : '-' }}</td>
                <td class="px-4 py-3 text-right">
                  <div class="flex justify-end gap-1">
                    <button class="btn btn-secondary btn-sm" :disabled="testingId === connection.id" :title="t('admin.upstreamConnections.test')" @click="testConnection(connection)">
                      <Icon name="play" size="sm" :class="testingId === connection.id ? 'animate-pulse' : ''" />
                    </button>
                    <button class="btn btn-secondary btn-sm" :title="t('admin.upstreamConnections.edit')" @click="openEdit(connection)"><Icon name="edit" size="sm" /></button>
                    <button class="btn btn-danger btn-sm" :title="t('admin.upstreamConnections.delete')" @click="deleteConnection(connection)"><Icon name="trash" size="sm" /></button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="snapshot" class="grid gap-6 lg:grid-cols-2">
        <section class="rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
          <div class="flex items-center justify-between"><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.upstreamConnections.remoteAccounts') }}</h2><span class="text-sm text-gray-500">{{ snapshot.accounts.length }}</span></div>
          <div class="mt-4 max-h-96 overflow-auto"><div v-for="account in snapshot.accounts" :key="account.id" class="flex items-center justify-between border-b border-gray-100 py-3 text-sm last:border-0 dark:border-dark-800"><div class="min-w-0"><div class="truncate font-medium text-gray-800 dark:text-gray-200">{{ account.name || account.id }}</div><div class="mt-1 text-xs text-gray-500">{{ account.platform || '-' }} · {{ account.type || '-' }}</div></div><div class="ml-4 text-right text-xs text-gray-500"><div>{{ account.status || '-' }}</div><div v-if="account.rate_multiplier != null">x{{ account.rate_multiplier }}</div></div></div></div>
        </section>
        <section class="rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
          <div class="flex items-center justify-between"><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.upstreamConnections.remoteGroups') }}</h2><span class="text-sm text-gray-500">{{ snapshot.groups.length }}</span></div>
          <div class="mt-4 max-h-96 overflow-auto"><div v-for="group in snapshot.groups" :key="group.id" class="flex items-center justify-between border-b border-gray-100 py-3 text-sm last:border-0 dark:border-dark-800"><div><div class="font-medium text-gray-800 dark:text-gray-200">{{ group.name || group.id }}</div><div class="mt-1 text-xs text-gray-500">{{ group.platform || '-' }} · {{ group.status || '-' }}</div></div><div class="text-right text-xs text-gray-500"><div v-if="group.rate_multiplier != null">x{{ group.rate_multiplier }}</div><div v-if="group.account_count != null">{{ group.account_count }} {{ t('admin.upstreamConnections.accounts') }}</div></div></div></div>
        </section>
      </div>
    </div>

    <BaseDialog :show="showDialog" :title="editing ? t('admin.upstreamConnections.edit') : t('admin.upstreamConnections.add')" width="normal" @close="closeDialog">
      <form class="space-y-4" @submit.prevent="saveConnection">
        <label class="block"><span class="input-label">{{ t('admin.upstreamConnections.name') }}</span><input v-model="form.name" class="input mt-1" required /></label>
        <label class="block"><span class="input-label">{{ t('admin.upstreamConnections.type') }}</span><select v-model="form.type" class="input mt-1"><option value="sub2api">Sub2API</option><option value="newapi">NewAPI</option><option value="other">{{ t('admin.upstreamConnections.other') }}</option></select></label>
        <label class="block"><span class="input-label">{{ t('admin.upstreamConnections.baseUrl') }}</span><input v-model="form.base_url" type="url" class="input mt-1" placeholder="https://example.com" required /></label>
        <label class="block"><span class="input-label">{{ t('admin.upstreamConnections.username') }}</span><input v-model="form.username" class="input mt-1" required /></label>
        <label class="block"><span class="input-label">{{ t('admin.upstreamConnections.password') }}</span><input v-model="form.password" type="password" class="input mt-1" :placeholder="editing ? t('admin.upstreamConnections.passwordKeep') : ''" :required="!editing" /></label>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreamConnections.securityHint') }}</p>
        <div class="flex justify-end gap-2 pt-2"><button type="button" class="btn btn-secondary" @click="closeDialog">{{ t('common.cancel') }}</button><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('admin.upstreamConnections.saving') : t('common.save') }}</button></div>
      </form>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { UpstreamConnection, UpstreamConnectionInput, UpstreamConnectionSnapshot, UpstreamConnectionType } from '@/api/admin/upstreamConnections'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const connections = ref<UpstreamConnection[]>([])
const snapshot = ref<UpstreamConnectionSnapshot | null>(null)
const loading = ref(false)
const saving = ref(false)
const testingId = ref<number | null>(null)
const showDialog = ref(false)
const editingId = ref<number | null>(null)
const form = reactive<UpstreamConnectionInput>({ name: '', type: 'sub2api', base_url: '', username: '', password: '' })
const editing = computed(() => editingId.value !== null)
const headers = computed(() => [t('admin.upstreamConnections.connection'), t('admin.upstreamConnections.type'), t('admin.upstreamConnections.username'), t('admin.upstreamConnections.status'), t('admin.upstreamConnections.lastTest')])

async function loadConnections() {
  loading.value = true
  try { connections.value = await adminAPI.upstreamConnections.list() } catch (error: any) { appStore.showError(error?.message || t('admin.upstreamConnections.loadFailed')) } finally { loading.value = false }
}
function resetForm() { Object.assign(form, { name: '', type: 'sub2api', base_url: '', username: '', password: '' }); editingId.value = null }
function openCreate() { resetForm(); showDialog.value = true }
function openEdit(connection: UpstreamConnection) { Object.assign(form, { name: connection.name, type: connection.type, base_url: connection.base_url, username: connection.username, password: '' }); editingId.value = connection.id; showDialog.value = true }
function closeDialog() { if (!saving.value) showDialog.value = false }
async function saveConnection() { saving.value = true; try { if (editingId.value === null) await adminAPI.upstreamConnections.create(form); else await adminAPI.upstreamConnections.update(editingId.value, form); showDialog.value = false; await loadConnections(); appStore.showSuccess(t('admin.upstreamConnections.saved')) } catch (error: any) { appStore.showError(error?.message || t('admin.upstreamConnections.saveFailed')) } finally { saving.value = false } }
async function testConnection(connection: UpstreamConnection) { testingId.value = connection.id; try { snapshot.value = await adminAPI.upstreamConnections.test(connection.id); await loadConnections(); appStore.showSuccess(t('admin.upstreamConnections.testSuccess')) } catch (error: any) { appStore.showError(error?.message || t('admin.upstreamConnections.testFailed')); await loadConnections() } finally { testingId.value = null } }
async function deleteConnection(connection: UpstreamConnection) { if (!window.confirm(t('admin.upstreamConnections.deleteConfirm', { name: connection.name }))) return; try { await adminAPI.upstreamConnections.remove(connection.id); if (snapshot.value?.connection.id === connection.id) snapshot.value = null; await loadConnections() } catch (error: any) { appStore.showError(error?.message || t('admin.upstreamConnections.deleteFailed')) } }
function typeLabel(type: UpstreamConnectionType) { return type === 'newapi' ? 'NewAPI' : type === 'other' ? t('admin.upstreamConnections.other') : 'Sub2API' }
function statusLabel(status: string) { return status === 'healthy' ? t('admin.upstreamConnections.healthy') : status === 'error' ? t('admin.upstreamConnections.error') : t('admin.upstreamConnections.untested') }
function statusClass(status: string) { return status === 'healthy' ? 'badge-success' : status === 'error' ? 'badge-danger' : 'badge-gray' }
onMounted(loadConnections)
</script>
