<template>
  <AppLayout>
    <div class="space-y-6">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">创作中心设置</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">控制创作中心及图片、视频能力的展示和使用权限。</p>
      </div>

      <div v-if="loading" class="card p-6 text-sm text-gray-500">正在加载设置…</div>
      <div v-else class="space-y-6">
        <section class="card divide-y divide-gray-100 dark:divide-dark-700">
          <div class="flex items-center justify-between gap-6 p-6">
            <div>
              <h2 class="font-semibold text-gray-900 dark:text-white">创作中心总开关</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">关闭后，用户菜单和直接访问路由都会被拦截。</p>
            </div>
            <Toggle v-model="form.enabled" />
          </div>
          <div class="flex items-center justify-between gap-6 p-6" :class="!form.enabled ? 'opacity-50' : ''">
            <div>
              <h2 class="font-semibold text-gray-900 dark:text-white">图片生成</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">控制图片创作标签和图片任务接口。</p>
            </div>
            <Toggle v-model="form.image_enabled" :disabled="!form.enabled" />
          </div>
          <div class="flex items-center justify-between gap-6 p-6" :class="!form.enabled ? 'opacity-50' : ''">
            <div>
              <h2 class="font-semibold text-gray-900 dark:text-white">视频生成</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">当前默认关闭，打开后才会显示视频创作入口。</p>
            </div>
            <Toggle v-model="form.video_enabled" :disabled="!form.enabled" />
          </div>
        </section>

        <section class="card p-6">
          <h2 class="font-semibold text-gray-900 dark:text-white">渠道与密钥来源</h2>
          <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">
            创作中心不新增一套渠道密钥。图片和视频均复用现有 API Key → 分组 → 账号（地址/API Key）的调用链路；请继续在“分组管理”和“账号管理”中配置和校验渠道。
          </p>
          <div class="mt-4 flex flex-wrap gap-3">
            <RouterLink to="/admin/groups" class="btn btn-secondary">分组管理</RouterLink>
            <RouterLink to="/admin/accounts" class="btn btn-secondary">账号管理</RouterLink>
            <RouterLink to="/admin/creation/history" class="btn btn-secondary">查看生成记录</RouterLink>
          </div>
        </section>

        <div class="flex justify-end">
          <button type="button" class="btn btn-primary" :disabled="saving" @click="save">
            {{ saving ? '保存中…' : '保存设置' }}
          </button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { getCreationSettings, updateCreationSettings } from '@/api/admin/creation'
import type { CreationSettings } from '@/api/creationStudio'

const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const form = reactive<CreationSettings>({ enabled: true, image_enabled: true, video_enabled: false })

async function load() {
  loading.value = true
  try {
    Object.assign(form, await getCreationSettings())
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '加载创作中心设置失败')
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const saved = await updateCreationSettings({
      enabled: form.enabled,
      image_enabled: form.enabled && form.image_enabled,
      video_enabled: form.enabled && form.video_enabled,
    })
    Object.assign(form, saved)
    await appStore.fetchCreationSettings(true)
    appStore.showSuccess('创作中心设置已保存')
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : '保存创作中心设置失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
