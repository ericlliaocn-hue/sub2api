<template>
  <AppLayout>
    <div class="creation-studio">
      <div class="studio-page">
        <div v-if="errorMessage" class="studio-error">
          <span>{{ errorMessage }}</span>
          <button type="button" @click="errorMessage = ''">关闭</button>
        </div>

        <section class="studio-layout">
          <div class="preview-column">
            <div class="preview-card">
              <div class="preview-stage">
                <template v-if="currentWork?.mediaUrl">
                  <img v-if="currentWork.kind === 'image'" :src="currentWork.mediaUrl" alt="生成的图片" />
                  <video v-else controls playsinline preload="auto">
                    <source :src="currentWork.mediaUrl" :type="currentWork.mediaMimeType || 'video/mp4'" />
                  </video>
                </template>
                <div v-else class="preview-empty">
                  <div class="preview-empty-mark">✦</div>
                  <strong>让画面先发生</strong>
                  <span>输入描述，选择模型，生成结果会在这里呈现。</span>
                </div>
                <div class="preview-overlay preview-overlay-top">
                  <span>{{ activeKind === 'video' ? '视频' : '图片' }} · {{ currentWork?.modelName || selectedModel?.name }}</span>
                </div>
                <div v-if="busy" class="preview-progress">
                  <div class="progress-meta"><span>{{ statusText }}</span><span>{{ progress }}%</span></div>
                  <div class="progress-track"><span :style="{ width: `${progress}%` }"></span></div>
                </div>
              </div>
              <div class="preview-timeline">
                <button v-for="work in timelineWorks" :key="work.id" type="button" class="timeline-thumb" :class="{ 'is-current': currentWork?.id === work.id }" :title="work.modelName" @click="selectWork(work)">
                  <img v-if="work.kind === 'image' && work.mediaUrl" :src="work.mediaUrl" alt="作品缩略图" />
                  <video v-else-if="work.kind === 'video' && work.mediaUrl" muted playsinline preload="metadata">
                    <source :src="work.mediaUrl" :type="work.mediaMimeType || 'video/mp4'" />
                  </video>
                  <span v-else>{{ work.statusLabel }}</span>
                </button>
                <label class="creation-thumb-add timeline-add" title="上传参考素材">
                  <input type="file" :accept="referenceAccept" @change="handleReferenceMediaFile" />
                  <span>＋</span>
                  <small>添加</small>
                </label>
              </div>
            </div>
          </div>

          <aside class="control-column">
            <div class="control-card model-control-card">
              <div v-if="tabs.length > 1" class="mode-switch">
                <button v-for="tab in tabs" :key="tab.value" type="button" :class="{ 'is-active': activeKind === tab.value }" @click="activeKind = tab.value">{{ tab.label }}</button>
              </div>
              <div class="control-heading"><div><span>MODEL LIBRARY</span><h2>选择模型</h2></div><small>{{ activeKind === 'video' ? '适合镜头运动和图生视频' : '适合细节表现和构图创作' }}</small></div>
              <div class="model-picker" :class="{ 'is-open': modelMenuOpen }">
                <button type="button" class="model-picker-trigger" aria-haspopup="listbox" :aria-expanded="modelMenuOpen" @click="modelMenuOpen = !modelMenuOpen" @keydown.esc="modelMenuOpen = false">
                  <span class="model-icon" :class="`model-icon-${selectedModel?.kind || activeKind}`">
                    <PlatformIcon v-if="selectedModel?.platform !== 'seedance'" :platform="(selectedModel?.platform || 'openai') as GroupPlatform" size="lg" />
                    <svg v-else viewBox="0 0 24 24" aria-hidden="true">
                      <path d="M19.877 1.469 24 2.533v18.942l-4.123 1.056V1.469ZM6.529 10.896l4.115 1.064v8.979l-4.115 1.064V10.896ZM0 2.572l4.115 1.064v16.736L0 21.428V2.572Zm17.455 5.621v11.107l-4.123-1.064V9.256l4.123-1.063Z" fill="currentColor" />
                    </svg>
                  </span>
                  <span class="model-picker-copy"><small>当前模型</small><strong>{{ selectedModel?.name || '选择模型' }}</strong></span>
                  <span class="model-picker-arrow">⌄</span>
                </button>
                <div v-if="modelMenuOpen" class="model-menu" role="listbox" :aria-label="`${activeKind === 'video' ? '视频' : '图片'}模型`">
                  <button v-for="model in modelOptions" :key="model.id" type="button" role="option" :aria-selected="selectedModelId === model.id" class="model-menu-option" :class="{ 'is-selected': selectedModelId === model.id }" @click="selectModel(model.id)">
                    <span class="model-icon" :class="`model-icon-${model.kind}`">
                      <PlatformIcon v-if="model.platform !== 'seedance'" :platform="model.platform as GroupPlatform" size="md" />
                      <svg v-else viewBox="0 0 24 24" aria-hidden="true">
                        <path d="M19.877 1.469 24 2.533v18.942l-4.123 1.056V1.469ZM6.529 10.896l4.115 1.064v8.979l-4.115 1.064V10.896ZM0 2.572l4.115 1.064v16.736L0 21.428V2.572Zm17.455 5.621v11.107l-4.123-1.064V9.256l4.123-1.063Z" fill="currentColor" />
                      </svg>
                    </span>
                    <span class="model-option-copy"><strong>{{ model.name }}</strong><small>{{ model.description }}</small></span>
                    <i v-if="selectedModelId === model.id">✓</i>
                  </button>
                </div>
              </div>
              <div class="field-block">
                <div class="field-label"><span>画面比例</span><em>{{ aspectRatio }}</em></div>
                <select v-model="aspectRatio" class="ratio-select">
                  <option v-for="ratio in aspectRatios" :key="ratio.value" :value="ratio.value">{{ ratio.value }} · {{ ratio.label }}</option>
                </select>
              </div>
              <div class="side-reference">
                <div class="side-reference-heading"><span>参考素材</span><small>{{ referenceMedia ? `已添加 1 个${referenceMedia.kind === 'video' ? '视频' : '图片'}` : '可选' }}</small></div>
                <div class="side-reference-list">
                  <div v-if="referenceMedia" class="reference-preview">
                    <img v-if="referenceMedia.kind === 'image'" :src="referenceMedia.previewUrl" :alt="referenceMedia.name" />
                    <video v-else :src="referenceMedia.previewUrl" muted playsinline preload="metadata" />
                    <button type="button" title="移除参考素材" aria-label="移除参考素材" @click="clearReferenceMedia">×</button>
                  </div>
                  <label class="creation-thumb-add reference-upload" :class="{ 'has-reference': Boolean(referenceMedia) }" :title="referenceMedia ? '更换参考素材' : '添加参考素材'">
                    <input type="file" :accept="referenceAccept" @change="handleReferenceMediaFile" />
                    <span>＋</span>
                    <small>{{ referenceMedia ? '更换素材' : '添加素材' }}</small>
                  </label>
                </div>
              </div>
            </div>

            <div class="control-card advanced-card">
              <div class="control-heading"><div><span>COMPOSITION</span><h2>高级参数</h2></div><small>按需调整</small></div>
              <label class="parameter-row key-selector-row">
                <span>工作密钥</span>
                <select v-model="selectedApiKeyId" :disabled="loadingKeys || compatibleApiKeys.length === 0">
                  <option v-if="compatibleApiKeys.length === 0" :value="''">暂无对应类型 Key</option>
                  <option v-for="key in compatibleApiKeys" :key="key.id" :value="key.id">{{ key.name || `Key #${key.id}` }}</option>
                </select>
              </label>
              <div v-if="activeKind === 'image'" class="parameter-grid">
                <div class="field-block compact-field">
                  <div class="field-label"><span>分辨率</span><em>{{ imageResolution }}</em></div>
                  <select v-model="imageResolution" class="ratio-select">
                    <option v-for="option in imageResolutionOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                  </select>
                </div>
                <div class="field-block compact-field">
                  <div class="field-label"><span>质量</span><em>{{ imageQualityLabel }}</em></div>
                  <select v-model="imageQuality" class="ratio-select">
                    <option v-for="option in imageQualityOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                  </select>
                </div>
              </div>
              <div v-if="activeKind === 'video'" class="field-block duration-block">
                <div class="field-label"><span>视频时长</span><em>{{ videoDuration }} 秒</em></div>
                <div class="duration-options"><button v-for="duration in durations" :key="duration" type="button" :class="{ 'is-selected': videoDuration === duration }" @click="videoDuration = duration">{{ duration }} 秒</button></div>
              </div>
              <div v-if="activeKind === 'video'" class="field-block">
                <div class="field-label"><span>视频分辨率</span><em>{{ videoResolution }}</em></div>
                <select v-model="videoResolution" class="ratio-select">
                  <option v-for="option in videoResolutionOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
              </div>
              <div class="parameter-row"><span>任务状态</span><strong><i :class="busy ? 'is-busy' : ''"></i>{{ busy ? statusText : '就绪，可开始创作' }}</strong></div>
            </div>
          </aside>
        </section>

        <div class="prompt-card">
          <div class="prompt-heading"><label for="creation-prompt">✦&nbsp; 描述你想创作的画面</label><span>{{ prompt.length }} / 2000</span></div>
          <textarea id="creation-prompt" v-model="prompt" maxlength="2000" rows="3" placeholder="例如：清晨的海边美术馆，白色建筑被柔和的阳光照亮，画面干净、克制、电影感……"></textarea>
          <div class="prompt-footer">
            <div class="prompt-tags"><button v-for="tag in promptTags" :key="tag" type="button" @click="prompt = prompt ? `${prompt}，${tag}` : tag">{{ tag }}</button></div>
            <button type="button" class="generate-button" :disabled="busy || !selectedApiKey || !prompt.trim()" @click="generate"><span>✦</span>{{ busy ? '生成中…' : '开始生成' }}<span aria-hidden="true">↑</span></button>
          </div>
        </div>

        <section class="recent-works">
          <header class="recent-works-heading">
            <div><h2>最近作品</h2><p>你的创作会自动保存在这里</p></div>
            <button type="button" @click="historyOpen = true">查看全部 <span aria-hidden="true">→</span></button>
          </header>
          <div v-if="recentWorks.length" class="recent-works-grid">
            <article v-for="work in recentWorks.slice(0, 5)" :key="work.id" class="recent-work-card">
              <button type="button" class="recent-work-card-main" @click="selectWork(work)">
                <div class="recent-work-media">
                  <img v-if="work.kind === 'image' && work.mediaUrl" :src="work.mediaUrl" alt="作品缩略图" />
                  <video v-else-if="work.kind === 'video' && work.mediaUrl" muted playsinline preload="metadata">
                    <source :src="work.mediaUrl" :type="work.mediaMimeType || 'video/mp4'" />
                  </video>
                  <span v-else>{{ work.statusLabel }}</span>
                </div>
                <strong>{{ work.modelName }}</strong>
                <small>{{ work.prompt }}</small>
                <em>{{ work.statusLabel }}</em>
              </button>
              <div class="work-actions">
                <button type="button" title="复用提示词和参考素材" aria-label="复用提示词和参考素材" @click="reuseWork(work)">
                  <Icon name="copy" size="sm" />
                  <span>复用</span>
                </button>
                <button type="button" title="下载作品" aria-label="下载作品" :disabled="!work.mediaUrl || downloadingWorkId === work.id" @click="downloadWork(work)">
                  <Icon :name="downloadingWorkId === work.id ? 'refresh' : 'download'" size="sm" :class="{ 'animate-spin': downloadingWorkId === work.id }" />
                  <span>下载</span>
                </button>
              </div>
            </article>
          </div>
          <div v-else class="recent-works-empty">完成一次创作后，作品会出现在这里。</div>
        </section>

        <div v-if="historyOpen" class="history-modal" role="dialog" aria-modal="true" aria-labelledby="creation-history-title" @click.self="historyOpen = false">
          <section class="history-panel">
            <header class="history-panel-header">
              <div>
                <span class="studio-eyebrow">CREATION HISTORY</span>
                <h2 id="creation-history-title">创作记录</h2>
              </div>
              <button type="button" class="history-close" aria-label="关闭创作记录" @click="historyOpen = false">×</button>
            </header>
            <div v-if="historyLoading" class="history-empty">正在加载创作记录…</div>
            <div v-else-if="recentWorks.length" class="history-grid">
              <article v-for="work in recentWorks" :key="work.id" class="history-item">
                <button type="button" class="history-item-main" @click="selectWork(work); historyOpen = false">
                <div class="history-item-media">
                  <img v-if="work.kind === 'image' && work.mediaUrl" :src="work.mediaUrl" alt="作品缩略图" />
                  <video v-else-if="work.kind === 'video' && work.mediaUrl" muted playsinline preload="metadata">
                    <source :src="work.mediaUrl" :type="work.mediaMimeType || 'video/mp4'" />
                  </video>
                  <span v-else>{{ work.statusLabel }}</span>
                </div>
                <strong>{{ work.modelName }}</strong>
                <small>{{ work.prompt }}</small>
                <em>{{ work.statusLabel }}</em>
                </button>
                <div class="work-actions">
                  <button type="button" title="复用提示词和参考素材" aria-label="复用提示词和参考素材" @click="reuseWork(work); historyOpen = false">
                    <Icon name="copy" size="sm" />
                    <span>复用</span>
                  </button>
                  <button type="button" title="下载作品" aria-label="下载作品" :disabled="!work.mediaUrl || downloadingWorkId === work.id" @click="downloadWork(work)">
                    <Icon :name="downloadingWorkId === work.id ? 'refresh' : 'download'" size="sm" :class="{ 'animate-spin': downloadingWorkId === work.id }" />
                    <span>下载</span>
                  </button>
                </div>
              </article>
            </div>
          </section>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { keysAPI } from '@/api/keys'
import type { ApiKey } from '@/types'
import type { GroupPlatform } from '@/types'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  IMAGE_MODELS,
  VIDEO_MODELS,
  createCreationTask,
  getCreationVideoContent,
  getCreationVideoTask,
  imageGenerationAssetUrl,
  isSuccessfulCreationStatus,
  isTerminalCreationStatus,
  listCreationHistory,
  loadCreationAsset,
  mediaUrlToDataUrl,
  saveCreationRemoteAsset,
  submitCreationImage,
  submitCreationVideo,
  updateCreationTask,
  uploadCreationAsset,
  type CreationKind,
  type CreationReferenceKind,
  type CreationModelOption,
  type CreationSettings,
  type ImageQuality,
  type ImageResolution,
  type VideoResolution,
} from '@/api/creationStudio'

interface WorkItem {
  id: string
  kind: CreationKind
  modelId: string
  modelName: string
  prompt: string
  mediaUrl?: string
  mediaMimeType?: string
  mediaAssetId?: number
  statusLabel: string
  createdAt: number
  historyTaskId?: number
}

interface ReferenceMediaState {
  kind: CreationReferenceKind
  name: string
  dataUrl: string
  previewUrl: string
  mimeType: string
}

const appStore = useAppStore()
const creationSettings = ref<CreationSettings | null>(null)
const tabs = computed(() => {
  if (!creationSettings.value) return []
  const available = [] as Array<{ value: CreationKind; label: string }>
  if (creationSettings.value.image_enabled) available.push({ value: 'image', label: '图片创作' })
  if (creationSettings.value.video_enabled) available.push({ value: 'video', label: '视频创作' })
  return available
})
const activeKind = ref<CreationKind>('image')
const apiKeys = ref<ApiKey[]>([])
const selectedApiKeyId = ref<number | ''>('')
const loadingKeys = ref(false)
const selectedModelId = ref(IMAGE_MODELS[0].id)
const prompt = ref('')
const aspectRatio = ref('16:9')
const imageResolution = ref<ImageResolution>('auto')
const imageQuality = ref<ImageQuality>('auto')
const videoResolution = ref<VideoResolution>('auto')
const videoDuration = ref(5)
const busy = ref(false)
const progress = ref(0)
const statusText = ref('准备中…')
const errorMessage = ref('')
const currentWork = ref<WorkItem | null>(null)
const recentWorks = ref<WorkItem[]>([])
const historyOpen = ref(false)
const historyLoading = ref(false)
const referenceMedia = ref<ReferenceMediaState | null>(null)
const downloadingWorkId = ref<string | null>(null)
const modelMenuOpen = ref(false)
let pollTimer: number | null = null
const objectUrls = new Set<string>()

const modelOptions = computed<CreationModelOption[]>(() => activeKind.value === 'image' ? IMAGE_MODELS : VIDEO_MODELS)
const selectedModel = computed(() => modelOptions.value.find(model => model.id === selectedModelId.value) || modelOptions.value[0])
const selectedApiKey = computed(() => apiKeys.value.find(key => key.id === Number(selectedApiKeyId.value)) || null)
const timelineWorks = computed(() => recentWorks.value.filter(work => work.kind === activeKind.value).slice(0, 5))
const compatibleApiKeys = computed(() => apiKeys.value.filter(key => {
  const platform = key.group?.platform
  return activeKind.value === 'image' ? platform === 'openai' : platform === 'seedance'
}))
const activeKindEnabled = computed(() => tabs.value.some(tab => tab.value === activeKind.value))
const aspectRatios = [
  { value: '1:1', label: '方形', icon: '□' },
  { value: '16:9', label: '横屏', icon: '▭' },
  { value: '9:16', label: '竖屏', icon: '▯' },
  { value: '4:3', label: '经典', icon: '▭' },
  { value: '3:4', label: '人像', icon: '▯' },
  { value: '21:9', label: '宽银幕', icon: '▬' },
]
const imageResolutionOptions: Array<{ value: ImageResolution; label: string }> = [
  { value: 'auto', label: '自动 · 推荐' },
  { value: '1K', label: '1K · 轻量快速' },
  { value: '2K', label: '2K · 高清创作' },
  { value: '3K', label: '3K · 高解析度' },
  { value: '4K', label: '4K · 超清细节' },
  { value: '5K', label: '5K · 极致细节' },
  { value: '6K', label: '6K · 旗舰画质' },
]
const imageQualityOptions: Array<{ value: ImageQuality; label: string }> = [
  { value: 'auto', label: '自动 · 平衡' },
  { value: 'low', label: '标准 · 更快' },
  { value: 'high', label: '高质量 · 更细腻' },
]
const videoResolutionOptions: Array<{ value: VideoResolution; label: string }> = [
  { value: 'auto', label: '自动 · 推荐' },
  { value: '720p', label: '720p · 快速预览' },
  { value: '1080p', label: '1080p · 高清成片' },
]
const durations = [4, 5, 8]
const promptTags = ['电影感', '高级留白', '柔和自然光', '真实材质']
const imageQualityLabel = computed(() => imageQualityOptions.find(option => option.value === imageQuality.value)?.label.split(' · ')[0] || '自动')
const referenceAccept = computed(() => activeKind.value === 'video'
  ? 'image/png,image/jpeg,image/webp,video/mp4,video/webm,video/quicktime'
  : 'image/png,image/jpeg,image/webp')

watch(activeKind, () => {
  if (!activeKindEnabled.value) {
    activeKind.value = tabs.value[0]?.value || 'image'
    return
  }
  selectedModelId.value = modelOptions.value[0]?.id || ''
  modelMenuOpen.value = false
  if (activeKind.value === 'image' && referenceMedia.value?.kind === 'video') clearReferenceMedia()
  if (!compatibleApiKeys.value.some(key => key.id === Number(selectedApiKeyId.value))) {
    selectedApiKeyId.value = compatibleApiKeys.value[0]?.id || ''
  }
})

function selectModel(modelId: string) {
  selectedModelId.value = modelId
  modelMenuOpen.value = false
}

function handleDocumentPointerDown(event: PointerEvent) {
  const target = event.target
  if (target instanceof Element && !target.closest('.model-picker')) modelMenuOpen.value = false
}

function rememberUrl(url: string) {
  objectUrls.add(url)
  return url
}

function clearPollTimer() {
  if (pollTimer !== null) window.clearTimeout(pollTimer)
  pollTimer = null
}

function schedulePoll(callback: () => void, delay: number) {
  clearPollTimer()
  pollTimer = window.setTimeout(callback, delay)
}

function setError(error: unknown) {
  const message = error instanceof Error ? error.message : String((error as { message?: string })?.message || '生成失败，请稍后重试')
  errorMessage.value = message
  appStore.showError(message)
}

function clearReferenceMedia() {
  const previewUrl = referenceMedia.value?.previewUrl
  if (previewUrl?.startsWith('blob:') && !objectUrls.has(previewUrl)) URL.revokeObjectURL(previewUrl)
  referenceMedia.value = null
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error || new Error('参考素材读取失败'))
    reader.onload = () => resolve(String(reader.result || ''))
    reader.readAsDataURL(file)
  })
}

async function handleReferenceMediaFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  const kind: CreationReferenceKind = file.type.startsWith('video/') ? 'video' : 'image'
  const accepted = kind === 'image'
    ? ['image/png', 'image/jpeg', 'image/webp'].includes(file.type)
    : ['video/mp4', 'video/webm', 'video/quicktime'].includes(file.type)
  if (!accepted || activeKind.value === 'image' && kind !== 'image') {
    appStore.showError(activeKind.value === 'video' ? '参考素材仅支持 PNG、JPG、WebP、MP4、WebM 或 MOV。' : '图片创作仅支持图片参考素材。')
    return
  }
  const maxSize = kind === 'video' ? 24 * 1024 * 1024 : 10 * 1024 * 1024
  if (file.size > maxSize) {
    appStore.showError(kind === 'video' ? '参考视频不能超过 24MB。' : '参考图不能超过 10MB。')
    return
  }
  try {
    const dataUrl = await readFileAsDataUrl(file)
    clearReferenceMedia()
    referenceMedia.value = {
      kind,
      name: file.name,
      dataUrl,
      previewUrl: URL.createObjectURL(file),
      mimeType: file.type,
    }
  } catch (error) {
    setError(error)
  }
}

function pushWork(work: WorkItem) {
  recentWorks.value = [work, ...recentWorks.value.filter(item => item.id !== work.id)].slice(0, 10)
  currentWork.value = work
}

function historyStatusLabel(status: string) {
  const normalized = String(status || '').toLowerCase()
  if (['completed', 'succeeded', 'success'].includes(normalized)) return '已完成'
  if (['failed', 'error', 'cancelled'].includes(normalized)) return '生成失败'
  if (['processing', 'running'].includes(normalized)) return '生成中'
  return '排队中'
}

function modelNameForHistory(modelId: string) {
  return [...IMAGE_MODELS, ...VIDEO_MODELS].find(model => model.id === modelId)?.name || modelId || '未知模型'
}

async function loadHistory() {
  historyLoading.value = true
  // Do not retain the previous session's in-memory works while the current
  // user's scoped API response is loading (for example after an SSO account
  // switch without a full page reload).
  recentWorks.value = []
  currentWork.value = null
  try {
    const result = await listCreationHistory(1, 20)
    const items = await Promise.all((result.items || []).map(async item => {
      const asset = item.assets?.[0]
      let mediaUrl = ''
      const mediaMimeType = asset?.mime_type || (item.kind === 'video' ? 'video/mp4' : 'image/png')
      let statusLabel = historyStatusLabel(item.status)
      if (asset?.content_url) {
        try {
          mediaUrl = await loadCreationAsset(asset.content_url, asset.mime_type)
          rememberUrl(mediaUrl)
        } catch {
          mediaUrl = ''
        }
      }

      // A browser tab can be closed while a video is still running. Reconcile
      // those durable history rows on the next page load so they do not stay
      // in "生成中" forever. The API key lookup is constrained to the key ID
      // persisted with this user's task.
      if (item.kind === 'video' && item.provider_task_id && !['completed', 'failed', 'cancelled', 'error'].includes(String(item.status).toLowerCase())) {
        const apiKey = apiKeys.value.find(key => key.id === item.api_key_id)
        if (apiKey) {
          try {
            const providerTask = await getCreationVideoTask(apiKey.key, item.provider_task_id)
            if (isTerminalCreationStatus(providerTask.status, 'video')) {
              if (isSuccessfulCreationStatus(providerTask.status, 'video')) {
                if (!mediaUrl) {
                  const mediaBlob = await getCreationVideoContent(apiKey.key, item.provider_task_id)
                  mediaUrl = rememberUrl(URL.createObjectURL(mediaBlob))
                  try {
                    await uploadCreationAsset(item.id, 'video', mediaBlob)
                  } catch (assetError) {
                    setError(assetError)
                  }
                }
                await updateCreationTask(item.id, { status: 'completed' })
                statusLabel = '已完成'
              } else {
                await updateCreationTask(item.id, { status: 'failed', error_message: `视频任务失败：${String(providerTask.status || 'unknown')}` })
                statusLabel = '生成失败'
              }
            } else {
              statusLabel = historyStatusLabel(providerTask.status || item.status)
            }
          } catch {
            // A transient provider/network error must not erase the durable
            // history status; the next page load can retry reconciliation.
          }
        }
      }
      return {
        id: String(item.id),
        historyTaskId: item.id,
        kind: item.kind,
        modelId: item.model,
        modelName: modelNameForHistory(item.model),
        prompt: item.prompt,
        mediaMimeType,
        mediaUrl,
        mediaAssetId: asset?.id,
        statusLabel,
        createdAt: Date.parse(item.created_at) || Date.now(),
      } satisfies WorkItem
    }))
    recentWorks.value = items
  } catch (error) {
    setError(error)
  } finally {
    historyLoading.value = false
  }
}

async function generate() {
  if (!activeKindEnabled.value) {
    errorMessage.value = '当前创作类型已被管理员关闭。'
    return
  }
  const key = selectedApiKey.value
  const selected = selectedModel.value
  const text = prompt.value.trim()
  if (!key) {
    errorMessage.value = '请先选择一个可用的 API Key。'
    return
  }
  if (!text || !selected) return
  clearPollTimer()
  errorMessage.value = ''
  busy.value = true
  progress.value = 8
  statusText.value = '正在提交任务…'
  const idempotencyKey = `creation-${key.id}-${Date.now()}-${crypto.randomUUID()}`
  const base: WorkItem = { id: `local_${Date.now()}`, kind: activeKind.value, modelId: selected.id, modelName: selected.name, prompt: text, statusLabel: '排队中', createdAt: Date.now() }
  currentWork.value = base
  try {
    const referenceDataUrl = await resolveReferenceDataUrl()
    const referenceKind = referenceMedia.value?.kind
    const historyTask = await createCreationTask({
      api_key_id: key.id,
      kind: activeKind.value,
      model: selected.id,
      prompt: text,
      request: {
        model: selected.id,
        prompt: text,
        aspect_ratio: aspectRatio.value,
        reference_media_attached: Boolean(referenceDataUrl),
        ...(referenceKind ? { reference_media_kind: referenceKind } : {}),
        ...(activeKind.value === 'image'
          ? { resolution: imageResolution.value, quality: imageQuality.value }
          : { duration: videoDuration.value, resolution: videoResolution.value }),
      },
      idempotency_key: idempotencyKey,
    })
    base.id = String(historyTask.id)
    base.historyTaskId = historyTask.id
    if (activeKind.value === 'image') {
      const result = await submitCreationImage(key.key, selected.id, text, aspectRatio.value, idempotencyKey, referenceKind === 'image' ? referenceDataUrl : '', {
        resolution: imageResolution.value,
        quality: imageQuality.value,
      })
      const item = result.data?.[0]
      const mediaUrl = item ? imageGenerationAssetUrl(item) : ''
      if (!mediaUrl) throw new Error('图片服务已返回成功，但没有返回图片内容')
      base.mediaUrl = rememberUrl(mediaUrl)
      base.statusLabel = '已完成'
      progress.value = 100
      busy.value = false
      await updateCreationTask(historyTask.id, { status: 'completed' })
      try {
        const savedAsset = await saveCreationRemoteAsset(historyTask.id, 'image', mediaUrl)
        if (savedAsset.content_url) {
          const localMediaUrl = await loadCreationAsset(savedAsset.content_url, savedAsset.mime_type)
          base.mediaUrl = rememberUrl(localMediaUrl)
          base.mediaMimeType = savedAsset.mime_type || 'image/png'
        }
      } catch (assetError) {
        setError(assetError)
      }
      pushWork({ ...base })
    } else {
      const task = await submitCreationVideo(
        key.key,
        selected.id,
        text,
        aspectRatio.value,
        videoDuration.value,
        referenceKind ? { kind: referenceKind, dataUrl: referenceDataUrl } : undefined,
        videoResolution.value,
        historyTask.id,
      )
      base.id = task.id
      await updateCreationTask(historyTask.id, { status: 'processing', provider_task_id: task.id })
      await pollVideo(key.key, task.id, base)
    }
  } catch (error) {
    base.statusLabel = '生成失败'
    currentWork.value = { ...base }
    if (base.historyTaskId) {
      try {
        await updateCreationTask(base.historyTaskId, {
          status: 'failed',
          error_message: error instanceof Error ? error.message : String(error),
        })
      } catch {
        // Keep the original generation error visible when history update fails.
      }
    }
    setError(error)
    busy.value = false
    clearPollTimer()
  }
}

async function pollVideo(apiKey: string, taskId: string, work: WorkItem): Promise<void> {
  const task = await getCreationVideoTask(apiKey, taskId)
  progress.value = Math.min(92, progress.value + 12)
  statusText.value = `视频任务 · ${String(task.status || 'processing')}`
  work.statusLabel = String(task.status || 'processing')
  currentWork.value = { ...work }
  if (!isTerminalCreationStatus(task.status, 'video')) {
    await new Promise<void>(resolve => schedulePoll(() => resolve(), 5000))
    return pollVideo(apiKey, taskId, work)
  }
  if (!isSuccessfulCreationStatus(task.status, 'video')) throw new Error(`视频任务失败：${String(task.status || 'unknown')}`)
  const mediaBlob = await getCreationVideoContent(apiKey, taskId)
  const mediaUrl = rememberUrl(URL.createObjectURL(mediaBlob))
  work.mediaUrl = mediaUrl
  work.mediaMimeType = mediaBlob.type || 'video/mp4'
  work.statusLabel = '已完成'
  progress.value = 100
  busy.value = false
  if (work.historyTaskId) {
    await updateCreationTask(work.historyTaskId, { status: 'completed' })
    try {
      await uploadCreationAsset(work.historyTaskId, 'video', mediaBlob)
    } catch (assetError) {
      setError(assetError)
    }
  }
  pushWork({ ...work })
}

function selectWork(work: WorkItem) {
  currentWork.value = work
  activeKind.value = work.kind
  selectedModelId.value = work.modelId
  prompt.value = work.prompt
}

function reuseWork(work: WorkItem) {
  if (!work.mediaUrl) return
  selectWork(work)
  clearReferenceMedia()
  referenceMedia.value = {
    kind: work.kind,
    name: `${work.modelName}.${work.kind === 'video' ? 'mp4' : 'png'}`,
    dataUrl: '',
    previewUrl: work.mediaUrl,
    mimeType: work.mediaMimeType || (work.kind === 'video' ? 'video/mp4' : 'image/png'),
  }
  void nextTick(() => {
    selectedModelId.value = work.modelId
  })
  appStore.showSuccess('提示词和参考素材已复用，可直接修改后生成')
}

async function downloadWork(work: WorkItem) {
  if (!work.mediaUrl || downloadingWorkId.value) return
  downloadingWorkId.value = work.id
  try {
    let downloadUrl = work.mediaUrl
    let shouldRevokeDownloadUrl = false
    let mimeType = work.mediaMimeType || ''
    if (!work.mediaUrl.startsWith('blob:') && !work.mediaUrl.startsWith('data:')) {
      const response = await fetch(work.mediaUrl)
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      const blob = await response.blob()
      mimeType = blob.type || mimeType
      downloadUrl = URL.createObjectURL(blob)
      shouldRevokeDownloadUrl = true
    }
    const extension = work.kind === 'video'
      ? (mimeType.includes('webm') ? 'webm' : 'mp4')
      : (mimeType.includes('webp') ? 'webp' : mimeType.includes('jpeg') ? 'jpg' : 'png')
    const link = document.createElement('a')
    link.href = downloadUrl
    link.download = `creation-${work.id}.${extension}`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    if (shouldRevokeDownloadUrl) window.setTimeout(() => URL.revokeObjectURL(downloadUrl), 1000)
  } catch (error) {
    setError(error)
  } finally {
    downloadingWorkId.value = null
  }
}

async function resolveReferenceDataUrl(): Promise<string> {
  if (!referenceMedia.value) return ''
  if (referenceMedia.value.dataUrl) return referenceMedia.value.dataUrl
  const dataUrl = await mediaUrlToDataUrl(referenceMedia.value.previewUrl, referenceMedia.value.mimeType)
  referenceMedia.value.dataUrl = dataUrl
  return dataUrl
}

async function loadApiKeys() {
  loadingKeys.value = true
  try {
    const response = await keysAPI.list(1, 100, { status: 'active', sort_by: 'created_at', sort_order: 'desc' })
    apiKeys.value = response.items || []
    if (!compatibleApiKeys.value.some(key => key.id === Number(selectedApiKeyId.value))) {
      selectedApiKeyId.value = compatibleApiKeys.value[0]?.id || ''
    }
  } catch (error) {
    setError(error)
  } finally {
    loadingKeys.value = false
  }
}

onMounted(async () => {
  document.addEventListener('pointerdown', handleDocumentPointerDown)
  creationSettings.value = await appStore.fetchCreationSettings()
  if (!creationSettings.value?.enabled || !tabs.value.length) {
    errorMessage.value = '当前没有可用的创作类型。'
    return
  }
  await loadApiKeys()
  await loadHistory()
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handleDocumentPointerDown)
  clearPollTimer()
  clearReferenceMedia()
  for (const url of objectUrls) URL.revokeObjectURL(url)
})
</script>

<style scoped>
.creation-studio {
  --studio-bg: #f5f6f8;
  --studio-surface: rgba(255, 255, 255, 0.95);
  --studio-surface-strong: rgba(255, 255, 255, 0.88);
  --studio-surface-soft: #fafbfc;
  --studio-heading: #171719;
  --studio-text: #1d1d1f;
  --studio-muted: #a0a1a7;
  --studio-border: rgba(20, 20, 30, 0.09);
  --studio-primary: #2f6eea;
  --studio-primary-strong: #2458c7;
  --studio-primary-ring: rgba(47, 110, 234, 0.18);
  --studio-input: #fafbfc;
  --studio-input-focus: #ffffff;
  --studio-subtle: #f0f1f4;
  --studio-preview: linear-gradient(145deg, #e8ebef, #f6f7f8 52%, #dfe4eb);
  --studio-preview-glow: radial-gradient(circle at 18% 15%, rgba(255, 255, 255, 0.95), transparent 34%), radial-gradient(circle at 84% 78%, rgba(206, 216, 231, 0.72), transparent 42%);
  min-height: calc(100vh - 5rem);
  margin: -2rem;
  padding: 1.75rem 2rem 2.5rem;
  color: var(--studio-text);
  background: var(--studio-bg);
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "SF Pro Display", "Segoe UI", sans-serif;
}

:global(html.dark .creation-studio) {
  --studio-bg: #020617;
  --studio-surface: rgba(30, 41, 59, 0.92);
  --studio-surface-strong: rgba(30, 41, 59, 0.86);
  --studio-surface-soft: #0f172a;
  --studio-heading: #f8fafc;
  --studio-text: #e2e8f0;
  --studio-muted: #94a3b8;
  --studio-border: rgba(148, 163, 184, 0.18);
  --studio-primary: #bcd3ff;
  --studio-primary-strong: #9dbbff;
  --studio-primary-ring: rgba(188, 211, 255, 0.24);
  --studio-input: #111827;
  --studio-input-focus: #172033;
  --studio-subtle: #172033;
  --studio-preview: linear-gradient(145deg, #1e293b, #111827 52%, #0f172a);
  --studio-preview-glow: radial-gradient(circle at 18% 15%, rgba(71, 85, 105, 0.4), transparent 34%), radial-gradient(circle at 84% 78%, rgba(47, 110, 234, 0.18), transparent 42%);
}

.studio-page {
  width: min(100%, 1640px);
  margin: 0 auto;
}

.studio-eyebrow,
.control-heading > div > span {
  color: var(--studio-muted);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.parameter-row strong i {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 999px;
  background: #55bd8d;
  box-shadow: 0 0 0 4px rgba(85, 189, 141, 0.12);
}

.parameter-row strong i.is-busy {
  background: #e3a157;
  box-shadow: 0 0 0 4px rgba(227, 161, 87, 0.12);
}

.studio-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
  padding: 0.85rem 1rem;
  border: 1px solid color-mix(in srgb, #c24141 22%, var(--studio-border));
  border-radius: 1rem;
  background: #fff8f8;
  color: #a44949;
  font-size: 0.82rem;
}

.studio-error button {
  color: #8e3333;
  font-size: 0.75rem;
  font-weight: 700;
}

.studio-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 420px;
  gap: 1.35rem;
  align-items: start;
}

.preview-column,
.control-column {
  min-width: 0;
}

.preview-card,
.prompt-card,
.control-card {
  border: 1px solid var(--studio-border);
  border-radius: 1.45rem;
  background: var(--studio-surface);
  box-shadow: 0 18px 60px rgba(30, 35, 50, 0.07);
}

.preview-card {
  overflow: hidden;
}

.preview-stage {
  position: relative;
  display: flex;
  aspect-ratio: 16 / 9;
  min-height: 0;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: var(--studio-preview);
}

.preview-stage::before {
  position: absolute;
  inset: 0;
  background: var(--studio-preview-glow);
  content: '';
}

.preview-stage > img,
.preview-stage > video {
  position: relative;
  z-index: 1;
  width: 100%;
  height: 100%;
  min-height: 0;
  object-fit: cover;
}

.preview-stage > video {
  background: #151619;
}

.preview-empty {
  position: relative;
  z-index: 1;
  display: grid;
  max-width: 18rem;
  justify-items: center;
  gap: 0.6rem;
  padding: 2rem;
  text-align: center;
}

.preview-empty-mark {
  display: grid;
  width: 4.75rem;
  height: 4.75rem;
  place-items: center;
  border-radius: 1.5rem;
  background: var(--studio-surface-strong);
  color: var(--studio-heading);
  font-size: 2rem;
  box-shadow: 0 18px 45px rgba(45, 50, 70, 0.12);
}

.preview-empty strong {
  color: var(--studio-heading);
  font-size: 1.05rem;
}

.preview-empty span {
  color: var(--studio-muted);
  font-size: 0.78rem;
  line-height: 1.6;
}

.preview-overlay {
  position: absolute;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  color: #fff;
  font-size: 0.75rem;
  font-weight: 600;
}

.preview-overlay-top {
  top: 1rem;
  right: 1rem;
  left: 1rem;
}

.preview-overlay-top > span:first-child {
  padding: 0.55rem 0.75rem;
  border-radius: 0.8rem;
  background: rgba(25, 27, 31, 0.62);
  backdrop-filter: blur(12px);
}

.preview-timeline {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  overflow-x: auto;
  min-height: 6.2rem;
  padding: 0.9rem 1rem 1rem;
  border-top: 1px solid var(--studio-border);
  background: var(--studio-surface);
}

.timeline-thumb,
.timeline-add {
  width: 6rem;
  height: 4.25rem;
  flex: 0 0 auto;
  border-radius: 0.85rem;
}

.timeline-thumb {
  display: grid;
  overflow: hidden;
  place-items: center;
  border: 2px solid transparent;
  background: var(--studio-subtle);
  color: var(--studio-muted);
  font-size: 0.64rem;
  transition: 180ms ease;
}

.timeline-thumb:hover,
.timeline-thumb.is-current {
  border-color: #1687ff;
  box-shadow: 0 0 0 2px rgba(22, 135, 255, 0.15);
}

.timeline-thumb img,
.timeline-thumb video {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.timeline-add {
  width: 6rem;
}

.preview-progress {
  position: absolute;
  z-index: 3;
  right: 1.25rem;
  bottom: 1.15rem;
  left: 1.25rem;
  padding: 0.75rem 0.9rem;
  border: 1px solid rgba(255, 255, 255, 0.65);
  border-radius: 0.95rem;
  background: rgba(255, 255, 255, 0.86);
  color: #66666d;
  box-shadow: 0 12px 30px rgba(35, 38, 50, 0.1);
  backdrop-filter: blur(14px);
}

.progress-meta,
.field-label,
.parameter-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.progress-meta {
  margin-bottom: 0.5rem;
  font-size: 0.68rem;
}

.progress-track {
  height: 0.35rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--studio-subtle);
}

.progress-track span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--studio-primary), var(--studio-primary-strong));
  transition: width 500ms ease;
}

.reference-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  min-height: 6.1rem;
  padding: 0.85rem 1rem;
  border-top: 1px solid var(--studio-border);
  background: var(--studio-surface-soft);
}

.reference-copy {
  display: grid;
  min-width: 0;
  gap: 0.2rem;
}

.reference-eyebrow {
  color: var(--studio-primary-strong);
  font-size: 0.6rem;
  font-weight: 750;
  letter-spacing: 0.16em;
}

.reference-copy strong {
  color: var(--studio-heading);
  font-size: 0.78rem;
}

.reference-copy small {
  overflow: hidden;
  color: var(--studio-muted);
  font-size: 0.66rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.reference-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.65rem;
}

.reference-preview,
.creation-thumb-add {
  display: grid;
  width: 5.7rem;
  height: 4.25rem;
  flex: 0 0 auto;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: 0.9rem;
  background: var(--studio-subtle);
  color: var(--studio-muted);
  font-size: 0.7rem;
}

.reference-preview {
  position: relative;
  border-style: solid;
}

.reference-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.creation-thumb-add {
  position: relative;
  border-style: dashed;
  background: var(--studio-input);
  cursor: pointer;
  gap: 0.05rem;
  font-size: 1.45rem;
  transition: 180ms ease;
}

.creation-thumb-add:hover,
.creation-thumb-add.has-reference {
  border-color: var(--studio-primary);
  color: var(--studio-primary);
  box-shadow: 0 0 0 2px var(--studio-primary-ring);
}

.creation-thumb-add input {
  display: none;
}

.creation-thumb-add small {
  color: inherit;
  font-size: 0.62rem;
  font-weight: 700;
}

.reference-preview > button {
  position: absolute;
  top: 0.25rem;
  right: 0.25rem;
  display: grid;
  width: 1.2rem;
  height: 1.2rem;
  place-items: center;
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.76);
  color: #fff;
  font-size: 0.8rem;
  line-height: 1;
}

.prompt-card {
  margin-top: 1.35rem;
  padding: 1rem 1.1rem 1.1rem;
}

.prompt-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.6rem;
}

.prompt-heading label {
  color: var(--studio-heading);
  font-size: 0.78rem;
  font-weight: 700;
}

.prompt-heading span,
.control-heading small {
  color: var(--studio-muted);
  font-size: 0.68rem;
}

.prompt-card textarea {
  display: block;
  width: 100%;
  resize: vertical;
  border: 1px solid var(--studio-border);
  border-radius: 0.9rem;
  background: var(--studio-input);
  padding: 0.8rem 0.9rem;
  color: var(--studio-text);
  font-size: 0.82rem;
  line-height: 1.6;
  outline: none;
}

.prompt-card textarea:focus {
  border-color: var(--studio-primary);
  background: var(--studio-input-focus);
  box-shadow: 0 0 0 4px var(--studio-primary-ring);
}

.prompt-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
  margin-top: 0.75rem;
}

.prompt-tags,
.duration-options {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.prompt-tags button,
.duration-options button {
  padding: 0.45rem 0.65rem;
  border: 1px solid var(--studio-border);
  border-radius: 999px;
  background: var(--studio-surface-strong);
  color: var(--studio-muted);
  font-size: 0.68rem;
  transition: 180ms ease;
}

.prompt-tags button:hover,
.duration-options button:hover {
  border-color: var(--studio-primary);
  color: var(--studio-heading);
}

.generate-button {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 2.5rem;
  padding: 0.65rem 1rem;
  border-radius: 0.85rem;
  background: linear-gradient(135deg, var(--studio-primary), var(--studio-primary-strong));
  color: #fff;
  font-size: 0.76rem;
  font-weight: 700;
  box-shadow: 0 10px 22px var(--studio-primary-ring);
  transition: 180ms ease;
}

.generate-button:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 14px 28px var(--studio-primary-ring);
}

.generate-button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.control-column {
  display: grid;
  gap: 1rem;
}

.control-card {
  padding: 1.1rem;
}

.mode-switch {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.25rem;
  margin-bottom: 1.15rem;
  padding: 0.25rem;
  border-radius: 0.9rem;
  background: var(--studio-subtle);
}

.mode-switch button {
  min-height: 2.5rem;
  border-radius: 0.7rem;
  color: var(--studio-muted);
  font-size: 0.76rem;
  font-weight: 600;
}

.mode-switch button.is-active {
  background: var(--studio-surface-strong);
  color: var(--studio-primary-strong);
  box-shadow: 0 5px 15px rgba(30, 35, 50, 0.08);
}

.control-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.9rem;
}

.control-heading h2 {
  margin-top: 0.35rem;
  color: var(--studio-heading);
  font-size: 1rem;
  font-weight: 700;
  letter-spacing: -0.025em;
}

.control-heading small {
  max-width: 9rem;
  text-align: right;
  line-height: 1.5;
}

.model-picker {
  position: relative;
  z-index: 5;
}

.model-picker-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  width: 100%;
  min-height: 3.65rem;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--studio-border);
  border-radius: 1rem;
  background: var(--studio-input);
  text-align: left;
  transition: 180ms ease;
}

.model-picker-trigger:hover,
.model-picker.is-open .model-picker-trigger {
  border-color: var(--studio-primary);
  background: var(--studio-input-focus);
  box-shadow: 0 0 0 3px var(--studio-primary-ring);
}

.model-icon {
  display: grid;
  width: 2.25rem;
  height: 2.25rem;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid var(--studio-border);
  border-radius: 0.75rem;
  background: linear-gradient(145deg, #fff, var(--studio-subtle));
  color: var(--studio-primary-strong);
  font-size: 1.05rem;
  font-weight: 700;
  box-shadow: 0 5px 14px rgba(38, 46, 62, 0.08);
}

.model-icon :deep(svg) {
  width: 1.2rem;
  height: 1.2rem;
}

.model-icon-video {
  color: #3c8cff;
}

.model-picker-copy {
  display: grid;
  min-width: 0;
  flex: 1;
  gap: 0.18rem;
}

.model-picker-copy small {
  color: var(--studio-muted);
  font-size: 0.61rem;
  font-weight: 600;
}

.model-picker-copy strong {
  overflow: hidden;
  color: var(--studio-heading);
  font-size: 0.82rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-picker-arrow {
  color: var(--studio-muted);
  font-size: 1.1rem;
  line-height: 1;
  transition: transform 180ms ease;
}

.model-picker.is-open .model-picker-arrow {
  transform: rotate(180deg);
}

.model-menu {
  position: absolute;
  z-index: 10;
  top: calc(100% + 0.55rem);
  right: 0;
  left: 0;
  display: grid;
  gap: 0.3rem;
  padding: 0.4rem;
  border: 1px solid var(--studio-border);
  border-radius: 1rem;
  background: var(--studio-surface-strong);
  box-shadow: 0 18px 40px rgba(28, 34, 48, 0.16);
  backdrop-filter: blur(18px);
}

.model-menu-option {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  min-width: 0;
  padding: 0.55rem;
  border: 1px solid transparent;
  border-radius: 0.75rem;
  text-align: left;
  transition: 180ms ease;
}

.model-menu-option:hover,
.model-menu-option.is-selected {
  border-color: var(--studio-primary);
  background: var(--studio-input);
}

.model-menu-option .model-icon {
  width: 1.9rem;
  height: 1.9rem;
  border-radius: 0.6rem;
  font-size: 0.85rem;
}

.model-menu-option .model-icon :deep(svg) {
  width: 1rem;
  height: 1rem;
}

.model-option-copy {
  display: grid;
  min-width: 0;
  gap: 0.25rem;
}

.model-option-copy strong {
  color: var(--studio-heading);
  font-size: 0.76rem;
}

.model-option-copy small {
  overflow: hidden;
  color: var(--studio-muted);
  font-size: 0.65rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-menu-option > i {
  margin-left: auto;
  color: var(--studio-primary);
  font-size: 0.8rem;
}

.field-block {
  margin-top: 1.1rem;
}

.side-reference {
  margin-top: 1.15rem;
  padding-top: 1rem;
  border-top: 1px solid var(--studio-border);
}

.side-reference-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.65rem;
  color: var(--studio-muted);
  font-size: 0.7rem;
  font-weight: 650;
}

.side-reference-heading small {
  color: var(--studio-muted);
  font-size: 0.65rem;
  font-weight: 500;
}

.side-reference-list {
  display: flex;
  gap: 0.55rem;
  min-height: 4.25rem;
}

.side-reference-list .reference-preview,
.side-reference-list .creation-thumb-add {
  width: 6.2rem;
  height: 4.25rem;
}

.parameter-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.7rem;
}

.compact-field {
  min-width: 0;
  margin-top: 0.75rem;
}

.compact-field .ratio-select {
  min-height: 2.55rem;
  padding-right: 0.45rem;
  padding-left: 0.6rem;
  font-size: 0.7rem;
}

.field-label {
  margin-bottom: 0.45rem;
  color: var(--studio-muted);
  font-size: 0.7rem;
  font-weight: 600;
}

.field-label em {
  color: var(--studio-muted);
  font-style: normal;
  font-weight: 500;
}

.ratio-select {
  width: 100%;
  min-height: 2.75rem;
  padding: 0 0.8rem;
  border: 1px solid var(--studio-border);
  border-radius: 0.85rem;
  background: var(--studio-input);
  color: var(--studio-text);
  font-size: 0.75rem;
  outline: none;
}

.advanced-card .control-heading {
  margin-bottom: 1rem;
}

.parameter-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 2.6rem;
  padding: 0.65rem 0;
  border-top: 1px solid var(--studio-border);
  color: var(--studio-muted);
  font-size: 0.72rem;
}

.key-selector-row select {
  max-width: 68%;
  border: 0;
  background: transparent;
  color: var(--studio-heading);
  font-size: 0.72rem;
  font-weight: 600;
  outline: none;
  text-align: right;
}

.parameter-row strong {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  max-width: 12rem;
  overflow: hidden;
  color: var(--studio-heading);
  font-size: 0.72rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.duration-options button {
  border-radius: 0.7rem;
}

.duration-options button.is-selected {
  border-color: var(--studio-primary);
  background: var(--studio-primary);
  color: #fff;
}

.history-modal {
  position: fixed;
  z-index: 40;
  inset: 0;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 6rem 2rem 2rem;
  background: rgba(15, 23, 42, 0.42);
  backdrop-filter: blur(12px);
}

.history-panel {
  width: min(100%, 960px);
  max-height: calc(100vh - 8rem);
  overflow: auto;
  padding: 1.25rem;
  border: 1px solid var(--studio-border);
  border-radius: 1.45rem;
  background: var(--studio-surface);
  box-shadow: 0 30px 90px rgba(15, 23, 42, 0.24);
}

.history-panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}

.history-panel-header h2 {
  margin-top: 0.35rem;
  color: var(--studio-heading);
  font-size: 1.2rem;
  font-weight: 700;
}

.history-close {
  display: grid;
  width: 2rem;
  height: 2rem;
  place-items: center;
  border-radius: 999px;
  background: var(--studio-subtle);
  color: var(--studio-muted);
  font-size: 1.25rem;
  line-height: 1;
}

.history-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0.75rem;
}

.history-item {
  display: grid;
  min-width: 0;
  gap: 0.35rem;
  padding: 0.65rem;
  border: 1px solid var(--studio-border);
  border-radius: 1rem;
  background: var(--studio-input);
  text-align: left;
  transition: 180ms ease;
}

.history-item-main,
.recent-work-card-main {
  display: grid;
  min-width: 0;
  gap: 0.35rem;
  padding: 0;
  color: inherit;
  text-align: left;
}

.history-item:hover {
  border-color: var(--studio-primary);
  box-shadow: 0 0 0 2px var(--studio-primary-ring);
  transform: translateY(-1px);
}

.history-item-media {
  display: grid;
  aspect-ratio: 16 / 10;
  overflow: hidden;
  place-items: center;
  border-radius: 0.7rem;
  background: var(--studio-subtle);
  color: var(--studio-muted);
  font-size: 0.7rem;
}

.history-item-media img,
.history-item-media video {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.history-item strong {
  overflow: hidden;
  color: var(--studio-heading);
  font-size: 0.73rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-item small {
  display: -webkit-box;
  overflow: hidden;
  color: var(--studio-muted);
  font-size: 0.65rem;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.history-item em {
  color: var(--studio-primary-strong);
  font-size: 0.62rem;
  font-style: normal;
  font-weight: 650;
}

.history-empty {
  display: grid;
  min-height: 10rem;
  place-items: center;
  color: var(--studio-muted);
  font-size: 0.78rem;
  text-align: center;
}

.recent-works {
  margin-top: 2rem;
}

.recent-works-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.9rem;
}

.recent-works-heading h2 {
  margin: 0;
  color: var(--studio-heading);
  font-size: 1.2rem;
  font-weight: 750;
  letter-spacing: -0.035em;
}

.recent-works-heading p {
  margin-top: 0.25rem;
  color: var(--studio-muted);
  font-size: 0.75rem;
}

.recent-works-heading button {
  color: #1687ff;
  font-size: 0.75rem;
  font-weight: 650;
}

.recent-works-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 1rem;
}

.recent-work-card {
  display: grid;
  min-width: 0;
  gap: 0.35rem;
  padding: 0.65rem;
  border: 1px solid var(--studio-border);
  border-radius: 1rem;
  background: var(--studio-surface);
  text-align: left;
  box-shadow: 0 12px 32px rgba(30, 35, 50, 0.05);
  transition: 180ms ease;
}

.recent-work-card:hover {
  border-color: #1687ff;
  box-shadow: 0 0 0 2px rgba(22, 135, 255, 0.13), 0 16px 34px rgba(30, 35, 50, 0.08);
  transform: translateY(-1px);
}

.recent-work-card-main:focus-visible,
.history-item-main:focus-visible {
  outline: 2px solid var(--studio-primary);
  outline-offset: 3px;
}

.recent-work-media {
  display: grid;
  aspect-ratio: 1.16 / 1;
  overflow: hidden;
  place-items: center;
  border-radius: 0.75rem;
  background: var(--studio-subtle);
  color: var(--studio-muted);
  font-size: 0.7rem;
}

.recent-work-media img,
.recent-work-media video {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.recent-work-card strong {
  overflow: hidden;
  color: var(--studio-heading);
  font-size: 0.74rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recent-work-card small {
  display: -webkit-box;
  overflow: hidden;
  min-height: 2.1em;
  color: var(--studio-muted);
  font-size: 0.65rem;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.recent-work-card em {
  color: var(--studio-primary-strong);
  font-size: 0.62rem;
  font-style: normal;
  font-weight: 650;
}

.work-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.35rem;
  padding-top: 0.35rem;
  border-top: 1px solid var(--studio-border);
}

.work-actions button {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  min-height: 1.8rem;
  padding: 0.25rem 0.45rem;
  border-radius: 0.55rem;
  color: var(--studio-muted);
  font-size: 0.65rem;
  font-weight: 650;
  transition: 160ms ease;
}

.work-actions button:hover:not(:disabled) {
  background: var(--studio-subtle);
  color: var(--studio-primary-strong);
}

.work-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.recent-works-empty {
  display: grid;
  min-height: 7rem;
  place-items: center;
  border: 1px dashed var(--studio-border);
  border-radius: 1rem;
  color: var(--studio-muted);
  font-size: 0.75rem;
}

@media (max-width: 1280px) {
  .studio-layout {
    grid-template-columns: minmax(0, 1fr) 360px;
  }

  .recent-works-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

}

@media (max-width: 1024px) {
  .creation-studio {
    margin: -1.5rem;
    padding: 1.25rem;
  }

  .studio-layout {
    grid-template-columns: 1fr;
  }

  .control-column {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 680px) {
  .creation-studio {
    margin: -1rem;
    padding: 1rem;
  }

  .history-modal {
    padding: 5rem 1rem 1rem;
  }

  .reference-strip {
    align-items: flex-start;
  }

  .reference-copy small {
    white-space: normal;
  }

  .preview-stage,
  .preview-stage > img,
  .preview-stage > video {
    min-height: 56vw;
  }

  .prompt-footer,
  .control-column {
    display: grid;
  }

  .recent-works-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .generate-button {
    justify-content: center;
  }

  .parameter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
