import type { RouteLocationNormalizedLoaded } from 'vue-router'
import type { CustomMenuItem } from '@/types'
import { resolveRouteDocumentTitle } from './title'

type Locale = 'zh' | 'en'

interface PublicSEO {
  title: string
  description: string
  heading: string
  schemaType: 'home' | 'collection' | 'tool'
}

let trustedCanonicalBase = ''

function normalizedSiteName(siteName?: string): string {
  if (typeof siteName !== 'string' || !siteName.trim() || ['anytoken', 'sub2api'].includes(siteName.trim().toLowerCase())) return 'oioio'
  return siteName.trim()
}

export function resolvePublicSEO(path: string, siteName?: string, locale = 'zh'): PublicSEO | null {
  const name = normalizedSiteName(siteName)
  const lang: Locale = locale.toLowerCase().startsWith('en') ? 'en' : 'zh'

  if (path === '/') {
    return lang === 'en'
      ? {
          title: `${name} - AI Digital Club | Models, Workflows and API Access`,
          description: `${name} is a digital club for sharing AI models, workflows and experiments, with one API key for available models, clear pricing and usage records.`,
          heading: 'AI digital club',
          schemaType: 'home',
        }
      : {
          title: `${name} - AI数字俱乐部｜模型、玩法与 API 入口`,
          description: `${name} 是一个分享 AI 模型、玩法与工作流的数字俱乐部，通过一个 API Key 接入当前可用模型，并提供价格、用量查询和开发文档。`,
          heading: `${name} AI数字俱乐部`,
          schemaType: 'home',
        }
  }

  if (path === '/model-plaza') {
    return lang === 'en'
      ? {
          title: `AI Model API Pricing and Catalog | ${name} Model Plaza`,
          description: `Browse currently available ${name} models and API pricing. Compare input and output token billing, rates, capabilities and available groups using live page data.`,
          heading: 'AI model API pricing and catalog',
          schemaType: 'collection',
        }
      : {
          title: `AI 模型 API 价格与模型列表｜${name} 模型广场`,
          description: `查看 ${name} 当前可用模型及 API 价格，比较输入、输出 Token 计费、倍率、模型能力和可用分组；价格与可用性以页面实时数据为准。`,
          heading: 'AI 模型 API 价格与模型列表',
          schemaType: 'collection',
        }
  }

  if (path === '/key-usage') {
    return lang === 'en'
      ? {
          title: `${name} API Key Usage - Credit, Spend and Request Records`,
          description: `Use an API key locally in the browser to query ${name} credit, spend and request records. The page does not store the key.`,
          heading: 'API Key usage lookup',
          schemaType: 'tool',
        }
      : {
          title: `${name} API Key 用量查询 - 额度、消费与请求记录`,
          description: `在浏览器本地使用 API Key 查询 ${name} 额度、消费和请求记录；Key 不会被页面存储。`,
          heading: 'API Key 用量查询',
          schemaType: 'tool',
        }
  }

  return null
}

function ensureMeta(selector: string, attributes: Record<string, string>): HTMLMetaElement {
  let element = document.head.querySelector<HTMLMetaElement>(selector)
  if (!element) {
    element = document.createElement('meta')
    document.head.appendChild(element)
  }
  Object.entries(attributes).forEach(([key, value]) => element?.setAttribute(key, value))
  return element
}

function ensureCanonical(href: string): void {
  let element = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!element) {
    element = document.createElement('link')
    element.rel = 'canonical'
    document.head.appendChild(element)
  }
  element.href = href
}

function resolveCanonicalBase(): string {
  const normalizeBase = (candidate?: string): string => {
    if (!candidate) return ''
    try {
      const parsed = new URL(candidate)
      if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
        return new URL('/', parsed).href
      }
    } catch {
      return ''
    }
    return ''
  }

  const existing = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')?.href
  const existingBase = normalizeBase(existing)
  if (existingBase) trustedCanonicalBase = existingBase
  if (trustedCanonicalBase) return trustedCanonicalBase

  trustedCanonicalBase = normalizeBase(window.location.origin)
  return trustedCanonicalBase
}

function removeCanonicalAndSocialMetadata(): void {
  document.head.querySelector('link[rel="canonical"]')?.remove()
  document.head.querySelectorAll('meta[property^="og:"]').forEach((element) => element.remove())
  document.head.querySelector('meta[name="twitter:card"]')?.remove()
  document.head.querySelector('#route-structured-data')?.remove()
}

function applyStructuredData(seo: PublicSEO, siteName: string, canonical: string): void {
  const organizationID = `${new URL('/', canonical).href}#organization`
  let payload: Record<string, unknown>

  if (seo.schemaType === 'home') {
    payload = {
      '@context': 'https://schema.org',
      '@graph': [
        {
          '@type': 'Organization',
          '@id': organizationID,
          name: siteName,
          url: canonical,
          logo: new URL('/oioio-logo.svg', canonical).href,
        },
        {
          '@type': 'WebSite',
          '@id': `${canonical}#website`,
          name: siteName,
          url: canonical,
          publisher: { '@id': organizationID },
        },
      ],
    }
  } else if (seo.schemaType === 'collection') {
    payload = {
      '@context': 'https://schema.org',
      '@type': 'CollectionPage',
      name: seo.heading,
      description: seo.description,
      url: canonical,
    }
  } else {
    payload = {
      '@context': 'https://schema.org',
      '@type': 'WebApplication',
      name: seo.heading,
      description: seo.description,
      url: canonical,
      applicationCategory: 'DeveloperApplication',
      operatingSystem: 'Web',
    }
  }

  let script = document.head.querySelector<HTMLScriptElement>('#route-structured-data')
  if (!script) {
    script = document.createElement('script')
    script.id = 'route-structured-data'
    script.type = 'application/ld+json'
    const nonce = document.head.querySelector<HTMLScriptElement>('script[nonce]')?.nonce
    if (nonce) script.nonce = nonce
    document.head.appendChild(script)
  }
  script.textContent = JSON.stringify(payload)
}

export function applyRouteSEO(
  route: Pick<RouteLocationNormalizedLoaded, 'path' | 'name' | 'params' | 'meta'>,
  siteName?: string,
  locale = 'zh',
  customMenuItems: CustomMenuItem[] = [],
): void {
  const name = normalizedSiteName(siteName)
  const seo = resolvePublicSEO(route.path, name, locale)

  if (!seo) {
    document.title = resolveRouteDocumentTitle(route, name, customMenuItems)
    ensureMeta('meta[name="robots"]', { name: 'robots', content: 'noindex, follow' })
    document.head.querySelector('meta[name="description"]')?.remove()
    removeCanonicalAndSocialMetadata()
    return
  }

  const canonical = new URL(route.path === '/' ? '/' : route.path, resolveCanonicalBase() || window.location.origin).href
  document.title = seo.title
  ensureMeta('meta[name="description"]', { name: 'description', content: seo.description })
  ensureMeta('meta[name="robots"]', { name: 'robots', content: 'index,follow,max-image-preview:large' })
  ensureCanonical(canonical)
  ensureMeta('meta[property="og:type"]', { property: 'og:type', content: seo.schemaType === 'home' ? 'website' : 'article' })
  ensureMeta('meta[property="og:site_name"]', { property: 'og:site_name', content: name })
  ensureMeta('meta[property="og:title"]', { property: 'og:title', content: seo.title })
  ensureMeta('meta[property="og:description"]', { property: 'og:description', content: seo.description })
  ensureMeta('meta[property="og:url"]', { property: 'og:url', content: canonical })
  ensureMeta('meta[property="og:image"]', { property: 'og:image', content: new URL('/oioio-logo.svg', canonical).href })
  ensureMeta('meta[name="twitter:card"]', { name: 'twitter:card', content: 'summary' })
  applyStructuredData(seo, name, canonical)
}
