import { beforeEach, describe, expect, it } from 'vitest'
import { applyRouteSEO, resolvePublicSEO } from '@/router/seo'

describe('resolvePublicSEO', () => {
  it('为三个公开页面分配唯一中文标题和摘要', () => {
    const pages = ['/', '/model-plaza', '/key-usage'].map((path) => resolvePublicSEO(path, 'AnyToken', 'zh'))

    expect(pages.every(Boolean)).toBe(true)
    expect(new Set(pages.map((page) => page?.title)).size).toBe(3)
    expect(new Set(pages.map((page) => page?.description)).size).toBe(3)
    expect(pages[0]?.title).toContain('多模型 AI API 聚合平台')
    expect(pages[1]?.title).toContain('AI 模型 API 价格')
    expect(pages[2]?.title).toContain('API Key 用量查询')
  })

  it('提供对应的英文元数据', () => {
    expect(resolvePublicSEO('/', 'AnyToken', 'en-US')?.title).toContain('Multi-model AI API Platform')
  })

  it('不把私有页面当作公开 SEO 页面', () => {
    expect(resolvePublicSEO('/dashboard', 'AnyToken', 'zh')).toBeNull()
  })
})

describe('applyRouteSEO', () => {
  beforeEach(() => {
    document.head.innerHTML = '<title>Initial</title>'
  })

  it('同步公开页的 title、description、canonical、OG 和结构化数据', () => {
    applyRouteSEO({ path: '/model-plaza', name: 'ModelPlaza', params: {}, meta: { title: 'Model Plaza' } }, 'AnyToken', 'zh')

    expect(document.title).toContain('AI 模型 API 价格与模型列表')
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toContain('实时数据')
    expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toContain('index,follow')
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('http://localhost:3000/model-plaza')
    expect(document.head.querySelector('meta[property="og:title"]')?.getAttribute('content')).toBe(document.title)
    expect(document.head.querySelector('#route-structured-data')?.textContent).toContain('CollectionPage')
  })

  it('沿用服务端注入的可信 canonical 域名', () => {
    document.head.innerHTML += '<link rel="canonical" href="https://anytoken.work/">'
    applyRouteSEO({ path: '/model-plaza', name: 'ModelPlaza', params: {}, meta: {} }, 'AnyToken', 'zh')

    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://anytoken.work/model-plaza')
  })

  it('路由切换到私有页面后清理 canonical、OG 和结构化数据并设置 noindex', () => {
    applyRouteSEO({ path: '/', name: 'Home', params: {}, meta: { title: 'Home' } }, 'AnyToken', 'zh')
    applyRouteSEO({ path: '/dashboard', name: 'Dashboard', params: {}, meta: { title: 'Dashboard' } }, 'AnyToken', 'zh')

    expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex, follow')
    expect(document.head.querySelector('link[rel="canonical"]')).toBeNull()
    expect(document.head.querySelector('meta[property^="og:"]')).toBeNull()
    expect(document.head.querySelector('#route-structured-data')).toBeNull()
  })
})
