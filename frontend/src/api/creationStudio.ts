import { apiClient, buildApiUrl, buildGatewayUrl } from './client'

export type CreationKind = 'image' | 'video'
export type CreationReferenceKind = 'image' | 'video'

export interface CreationReferenceMedia {
  kind: CreationReferenceKind
  dataUrl: string
}

export interface CreationModelOption {
  id: string
  name: string
  kind: CreationKind
  description: string
  accent: string
  platform: CreationModelPlatform
}

export type ImageResolution = 'auto' | '1K' | '2K' | '4K'
export type ImageQuality = 'auto' | 'low' | 'high'
export type VideoResolution = 'auto' | '720p' | '1080p'
export type CreationModelPlatform = 'openai' | 'gemini' | 'seedance'

export interface CreationSettings {
  enabled: boolean
  image_enabled: boolean
  video_enabled: boolean
}

export async function getCreationSettings(): Promise<CreationSettings> {
  const { data } = await apiClient.get<CreationSettings>('/creation/config')
  return data
}

export interface ImageGenerationResponse {
  created?: number
  data?: Array<{
    url?: string
    b64_json?: string
    mime_type?: string
  }>
  [key: string]: unknown
}

export const IMAGE_MODELS: CreationModelOption[] = [
  { id: 'image2', name: 'Image 2', kind: 'image', description: '细节与构图平衡', accent: 'from-[#f5f1ff] to-[#e9e0ff]', platform: 'openai' },
  { id: 'banana1', name: 'Banana 1', kind: 'image', description: '自然质感与人像', accent: 'from-[#fff6df] to-[#ffebc7]', platform: 'gemini' },
  { id: 'bananapro', name: 'Banana Pro', kind: 'image', description: '专业级控制力', accent: 'from-[#e7f6f0] to-[#d8efe7]', platform: 'gemini' },
  { id: 'banana2', name: 'Banana 2', kind: 'image', description: '新一代创意表达', accent: 'from-[#e9f0ff] to-[#dfe9ff]', platform: 'gemini' },
]

export const VIDEO_MODELS: CreationModelOption[] = [
  { id: 'seedance2.5', name: 'Seedance 2.5', kind: 'video', description: '镜头稳定与自然运动', accent: 'from-[#eef6ff] to-[#dcecff]', platform: 'seedance' },
  { id: 'seedance2', name: 'Seedance 2', kind: 'video', description: '电影感运动镜头', accent: 'from-[#f3efff] to-[#e6ddff]', platform: 'seedance' },
  { id: 'seedance-mini', name: 'Seedance Mini', kind: 'video', description: '快速创意预览', accent: 'from-[#fff3e7] to-[#ffe7d0]', platform: 'seedance' },
  { id: 'seedance-fast', name: 'Seedance Fast', kind: 'video', description: '低等待快速出片', accent: 'from-[#edf8ed] to-[#dff2e1]', platform: 'seedance' },
]

export interface VideoGenerationResponse {
  id?: string
  request_id?: string
  object?: string
  status?: string
  [key: string]: unknown
}

export interface VideoTaskSnapshot extends VideoGenerationResponse {
  id: string
}

function normalizeVideoTaskResponse(data: VideoGenerationResponse, id: string): VideoTaskSnapshot {
  const nested = data.data && typeof data.data === 'object'
    ? data.data as { status?: unknown; progress?: unknown }
    : undefined
  const status = String(data.status || nested?.status || '').trim()
  const progress = data.progress ?? nested?.progress

  return {
    ...data,
    id,
    ...(status ? { status } : {}),
    ...(progress !== undefined ? { progress } : {}),
  }
}

export interface CreationHistoryAsset {
  id: number
  kind: CreationKind
  mime_type?: string
  content_url: string
  created_at: string
}

export interface CreationHistoryItem {
  id: number
  api_key_id: number
  kind: CreationKind
  model: string
  prompt: string
  status: string
  provider_task_id?: string
  error_message?: string
  created_at: string
  finished_at?: string
  assets: CreationHistoryAsset[]
}

export interface CreationHistoryPage {
  items: CreationHistoryItem[]
  pagination: {
    page: number
    page_size: number
    total: number
    total_pages: number
  }
}

export async function createCreationTask(input: {
  api_key_id: number
  kind: CreationKind
  model: string
  prompt: string
  request: Record<string, unknown>
  idempotency_key: string
}): Promise<{ id: number; status: string }> {
  const { data } = await apiClient.post<{ id: number; status: string }>('/creation/tasks', input)
  return data
}

export async function listCreationHistory(page = 1, pageSize = 20): Promise<CreationHistoryPage> {
  const { data } = await apiClient.get<CreationHistoryPage>('/creation/tasks', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function updateCreationTask(taskId: number, input: {
  status?: string
  provider_task_id?: string
  error_message?: string
}): Promise<void> {
  await apiClient.patch(`/creation/tasks/${taskId}`, input)
}

export async function uploadCreationAsset(taskId: number, kind: CreationKind, file: Blob): Promise<CreationHistoryAsset> {
  const form = new FormData()
  form.append('kind', kind)
  form.append('file', file, kind === 'video' ? 'creation.mp4' : 'creation.png')
  const { data } = await apiClient.post<CreationHistoryAsset>(`/creation/tasks/${taskId}/assets`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 120000,
  })
  return data
}

export async function saveCreationRemoteAsset(taskId: number, kind: CreationKind, url: string): Promise<CreationHistoryAsset> {
  const { data } = await apiClient.post<CreationHistoryAsset>(`/creation/tasks/${taskId}/assets`, { kind, url }, {
    timeout: 120000,
  })
  return data
}

export async function deleteCreationTask(taskId: number): Promise<void> {
  await apiClient.delete(`/creation/tasks/${taskId}`)
}

function normalizeMimeType(value: string | undefined): string {
  return String(value || '').split(';', 1)[0].trim().toLowerCase()
}

function normalizeMediaBlob(blob: Blob, preferredMimeType = ''): Blob {
  const preferred = normalizeMimeType(preferredMimeType)
  const detected = normalizeMimeType(blob.type)
  const mimeType = /^(image|video)\//.test(preferred)
    ? preferred
    : detected
  if (!mimeType || blob.type === mimeType) return blob
  return new Blob([blob], { type: mimeType })
}

export async function loadCreationAsset(contentUrl: string, mimeType = ''): Promise<string> {
  // History responses are API-relative. Normalize older responses that may
  // still contain /api/v1 so Axios does not produce /api/v1/api/v1/... .
  const normalizedUrl = contentUrl.replace(/^\/api\/v1(?=\/|$)/, '') || '/'
  const { data } = await apiClient.get<Blob>(normalizedUrl, { responseType: 'blob', timeout: 120000 })
  return URL.createObjectURL(normalizeMediaBlob(data, mimeType))
}

export async function mediaUrlToDataUrl(mediaUrl: string, mimeType = ''): Promise<string> {
  if (mediaUrl.startsWith('data:')) return mediaUrl
  const response = await fetch(mediaUrl)
  if (!response.ok) throw new Error(`媒体读取失败：HTTP ${response.status}`)
  const blob = normalizeMediaBlob(await response.blob(), mimeType)
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error || new Error('媒体读取失败'))
    reader.onload = () => resolve(String(reader.result || ''))
    reader.readAsDataURL(blob)
  })
}

export function creationAssetUrl(assetId: number): string {
  return buildApiUrl(`/creation/assets/${assetId}/content`)
}

async function parseGatewayError(response: Response): Promise<Error> {
  let message = response.statusText || `HTTP ${response.status}`
  try {
    const body = await response.json()
    message = body?.error?.message || body?.message || message
  } catch {
    // Keep the HTTP status when the gateway did not return JSON.
  }
  const error = new Error(message)
  ;(error as Error & { status?: number }).status = response.status
  return error
}

function authHeaders(apiKey: string, extra?: HeadersInit): HeadersInit {
  return { Authorization: `Bearer ${apiKey}`, ...extra }
}

export async function submitCreationImage(
  apiKey: string,
  model: string,
  prompt: string,
  aspectRatio: string,
  idempotencyKey: string,
  referenceImageDataUrl = '',
  options: {
    resolution?: ImageResolution
    quality?: ImageQuality
    n?: number
  } = {},
): Promise<ImageGenerationResponse> {
  const hasReferenceImage = referenceImageDataUrl.startsWith('data:image/')
  const resolution = options.resolution || 'auto'
  // 创作中心使用稳定的产品别名；Sub2API 现有 OpenAI 图片渠道使用
  // gpt-image-2 作为实际调度模型名。历史记录仍保留 image2 展示名。
  const gatewayModel = model === 'image2' ? 'gpt-image-2' : model
  const response = await fetch(buildGatewayUrl(hasReferenceImage ? '/v1/images/edits' : '/v1/images/generations'), {
    method: 'POST',
    headers: authHeaders(apiKey, {
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
    }),
    body: JSON.stringify({
      model: gatewayModel,
      prompt,
      n: Math.max(1, Math.min(4, Math.floor(options.n || 1))),
      size: imageSizeForAspectRatio(aspectRatio, resolution),
      aspect_ratio: aspectRatio,
      quality: options.quality || 'auto',
      ...(hasReferenceImage ? { images: [{ image_url: referenceImageDataUrl }] } : {}),
    }),
  })
  if (!response.ok) throw await parseGatewayError(response)
  return response.json() as Promise<ImageGenerationResponse>
}

export function imageGenerationAssetUrl(item: NonNullable<ImageGenerationResponse['data']>[number]): string {
  if (item.url) return item.url
  if (item.b64_json) return `data:${item.mime_type || 'image/png'};base64,${item.b64_json}`
  return ''
}

function imageSizeForAspectRatio(aspectRatio: string, resolution: ImageResolution): string {
  const sizes: Record<ImageResolution, Record<string, string>> = {
    auto: {
      '1:1': '1024x1024', '16:9': '1792x1024', '9:16': '1024x1792',
      '4:3': '1536x1024', '3:4': '1024x1536', '21:9': '1792x768',
    },
    '1K': {
      '1:1': '1024x1024', '16:9': '1536x864', '9:16': '864x1536',
      '4:3': '1365x1024', '3:4': '1024x1365', '21:9': '1792x768',
    },
    '2K': {
      '1:1': '2048x2048', '16:9': '2048x1152', '9:16': '1152x2048',
      '4:3': '2048x1536', '3:4': '1536x2048', '21:9': '2048x878',
    },
    '4K': {
      '1:1': '4096x4096', '16:9': '3840x2160', '9:16': '2160x3840',
      '4:3': '4096x3072', '3:4': '3072x4096', '21:9': '4096x1755',
    },
  }
  return sizes[resolution][aspectRatio] || sizes[resolution]['1:1']
}

export async function submitCreationVideo(
  apiKey: string,
  model: string,
  prompt: string,
  aspectRatio: string,
  duration: number,
  referenceMedia: CreationReferenceMedia | undefined,
  resolution: VideoResolution = 'auto',
  creationTaskId?: number,
): Promise<VideoTaskSnapshot> {
	const headers: Record<string, string> = { 'Content-Type': 'application/json' }
	if (creationTaskId && creationTaskId > 0) headers['X-Creation-Task-ID'] = String(creationTaskId)
  const response = await fetch(buildGatewayUrl('/v1/videos/generations'), {
    method: 'POST',
    headers: authHeaders(apiKey, headers),
    body: JSON.stringify({
      model,
      prompt,
      aspect_ratio: aspectRatio,
      duration,
      ...(resolution !== 'auto' ? { resolution } : {}),
      ...(referenceMedia?.kind === 'image' && referenceMedia.dataUrl.startsWith('data:image/')
        ? { reference_images: [{ url: referenceMedia.dataUrl }] }
        : {}),
      ...(referenceMedia?.kind === 'video' && referenceMedia.dataUrl.startsWith('data:video/')
        ? { reference_videos: [{ url: referenceMedia.dataUrl }] }
        : {}),
    }),
  })
  if (!response.ok) throw await parseGatewayError(response)
  const data = (await response.json()) as VideoGenerationResponse
  const id = String(data.id || data.request_id || '').trim()
  if (!id) throw new Error('视频服务没有返回任务编号')
  return normalizeVideoTaskResponse(data, id)
}

export async function getCreationVideoTask(apiKey: string, taskId: string): Promise<VideoTaskSnapshot> {
  const response = await fetch(buildGatewayUrl(`/v1/videos/${encodeURIComponent(taskId)}`), {
    headers: authHeaders(apiKey),
  })
  if (!response.ok) throw await parseGatewayError(response)
  const data = (await response.json()) as VideoGenerationResponse
  return normalizeVideoTaskResponse(data, taskId)
}

export async function getCreationVideoContent(apiKey: string, taskId: string): Promise<Blob> {
  const response = await fetch(buildGatewayUrl(`/v1/videos/${encodeURIComponent(taskId)}/content`), {
    headers: authHeaders(apiKey),
  })
  if (!response.ok) throw await parseGatewayError(response)
  const blob = await response.blob()
  const contentType = normalizeMimeType(response.headers.get('content-type') || '')
  return normalizeMediaBlob(blob, contentType.startsWith('video/') ? contentType : 'video/mp4')
}

export function isTerminalCreationStatus(status: string | undefined, kind: CreationKind): boolean {
  const normalized = String(status || '').toLowerCase()
  if (kind === 'image') {
    return ['completed', 'failed', 'cancelled', 'output_deleted'].includes(normalized)
  }
  return ['completed', 'succeeded', 'success', 'failed', 'cancelled', 'error'].includes(normalized)
}

export function isSuccessfulCreationStatus(status: string | undefined, kind: CreationKind): boolean {
  const normalized = String(status || '').toLowerCase()
  return kind === 'image'
    ? normalized === 'completed'
    : ['completed', 'succeeded', 'success'].includes(normalized)
}
