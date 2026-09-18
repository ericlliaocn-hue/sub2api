/**
 * 获客落地证据（官网 / SEO / 邀请 / 其他）。
 *
 * 前端只负责采集并存进一方 Cookie `s2a_touch`，分类由服务端注册时裁定。
 * 字段名与后端 service/acquisition.go 的 acquisitionTouchWire 保持一致：
 *   v 版本 / t 首次时间 / p 落地路径 / r referrer host / s source / a aff / u utm / c 付费点击 / k 证据等级
 *
 * 规则：证据等级只允许升级不允许降级。官网逛过再点推广链接算推广；
 * 推广进来后随便逛不会被改回官网。
 */

export const ACQUISITION_TOUCH_COOKIE = 's2a_touch'
const COOKIE_MAX_AGE_SECONDS = 30 * 24 * 60 * 60
const FIELD_LIMIT = 512

export const TOUCH_LEVEL = {
  official: 0,
  external: 1,
  search: 2,
  paid: 3,
  aff: 4,
  source: 5
} as const

export interface AcquisitionTouchWire {
  v: 1
  t: number
  p: string
  r: string
  s: string
  a: string
  u: { s: string; m: string; c: string }
  c: boolean
  k: number
}

const PAID_CLICK_PARAMS = ['gclid', 'gbraid', 'wbraid', 'fbclid', 'msclkid', 'ttclid', 'dclid', 'yclid']
const PAID_UTM_MEDIUMS = new Set(['cpc', 'ppc', 'paid', 'paidsocial', 'paid_social', 'cpm', 'display'])

// 与后端名单保持一致；前端只用来算等级，最终分类以服务端为准。
const SEARCH_HOST_SUFFIXES = [
  'google.', 'bing.com', 'baidu.com', 'sogou.com', 'so.com', 'sm.cn', 'yandex.', 'duckduckgo.com',
  'naver.com', 'yahoo.', 'ecosia.org', 'brave.com',
  'chatgpt.com', 'openai.com', 'perplexity.ai', 'claude.ai', 'gemini.google.com', 'kimi.moonshot.cn',
  'kimi.com', 'doubao.com', 'metaso.cn', 'yiyan.baidu.com', 'tongyi.aliyun.com', 'deepseek.com', 'you.com'
]

function clip(value: unknown, limit = FIELD_LIMIT): string {
  const text = typeof value === 'string' ? value.trim() : ''
  return text.length > limit ? text.slice(0, limit) : text
}

export function normalizeHost(input: string): string {
  let host = (input || '').trim().toLowerCase()
  if (!host) return ''
  if (host.includes('://')) {
    try {
      host = new URL(host).host
    } catch {
      return ''
    }
  }
  host = host.split('/')[0].split(':')[0]
  return host.startsWith('www.') ? host.slice(4) : host
}

export function isSearchHost(host: string): boolean {
  const h = normalizeHost(host)
  if (!h) return false
  return SEARCH_HOST_SUFFIXES.some((suffix) => {
    if (suffix.endsWith('.')) return h.startsWith(suffix) || h.includes(`.${suffix}`)
    return h === suffix || h.endsWith(`.${suffix}`)
  })
}

function isOwnHost(host: string): boolean {
  if (typeof window === 'undefined') return false
  return normalizeHost(host) === normalizeHost(window.location.host)
}

export function buildTouchFromLocation(href: string, referrer: string, now = Date.now()): AcquisitionTouchWire {
  let url: URL | null = null
  try {
    url = new URL(href)
  } catch {
    url = null
  }
  const q = url?.searchParams
  const get = (key: string) => clip(q?.get(key) ?? '')
  const paid = !!q && PAID_CLICK_PARAMS.some((key) => !!q.get(key))
  let referrerHost = normalizeHost(referrer)
  if (referrerHost && isOwnHost(referrerHost)) referrerHost = ''
  const touch: AcquisitionTouchWire = {
    v: 1,
    t: now,
    p: clip(url?.pathname ?? ''),
    r: clip(referrerHost, 255),
    s: get('source').toUpperCase().slice(0, 64),
    a: (get('aff') || get('aff_code')).slice(0, 64),
    u: {
      s: get('utm_source').toLowerCase().slice(0, 128),
      m: get('utm_medium').toLowerCase().slice(0, 128),
      c: get('utm_campaign').slice(0, 128)
    },
    c: paid,
    k: 0
  }
  touch.k = touchLevel(touch)
  return touch
}

export function touchLevel(touch: AcquisitionTouchWire): number {
  if (touch.s) return TOUCH_LEVEL.source
  if (touch.a) return TOUCH_LEVEL.aff
  if (touch.c || PAID_UTM_MEDIUMS.has(touch.u.m)) return TOUCH_LEVEL.paid
  if (isSearchHost(touch.r)) return TOUCH_LEVEL.search
  if (touch.r) return TOUCH_LEVEL.external
  return TOUCH_LEVEL.official
}

export function hasEvidence(touch: AcquisitionTouchWire): boolean {
  return !!(touch.s || touch.a || touch.r || touch.c || touch.u.s || touch.u.m || touch.u.c)
}

function encode(touch: AcquisitionTouchWire): string {
  const json = JSON.stringify(touch)
  const bytes = new TextEncoder().encode(json)
  let binary = ''
  bytes.forEach((b) => {
    binary += String.fromCharCode(b)
  })
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function decode(raw: string): AcquisitionTouchWire | null {
  if (!raw || raw.length > 4096) return null
  try {
    const padded = raw.replace(/-/g, '+').replace(/_/g, '/')
    const binary = atob(padded + '='.repeat((4 - (padded.length % 4)) % 4))
    const bytes = Uint8Array.from(binary, (ch) => ch.charCodeAt(0))
    const parsed = JSON.parse(new TextDecoder().decode(bytes)) as Partial<AcquisitionTouchWire>
    if (parsed?.v !== 1) return null
    return {
      v: 1,
      t: Number(parsed.t) || 0,
      p: clip(parsed.p),
      r: clip(parsed.r, 255),
      s: clip(parsed.s, 64),
      a: clip(parsed.a, 64),
      u: { s: clip(parsed.u?.s, 128), m: clip(parsed.u?.m, 128), c: clip(parsed.u?.c, 128) },
      c: !!parsed.c,
      k: Number(parsed.k) || 0
    }
  } catch {
    return null
  }
}

export function readTouchCookie(): AcquisitionTouchWire | null {
  if (typeof document === 'undefined') return null
  const prefix = `${ACQUISITION_TOUCH_COOKIE}=`
  const part = document.cookie.split('; ').find((item) => item.startsWith(prefix))
  if (!part) return null
  return decode(decodeURIComponent(part.slice(prefix.length)))
}

export function writeTouchCookie(touch: AcquisitionTouchWire): void {
  if (typeof document === 'undefined') return
  const secure = typeof window !== 'undefined' && window.location.protocol === 'https:' ? '; Secure' : ''
  document.cookie = `${ACQUISITION_TOUCH_COOKIE}=${encode(touch)}; Path=/; Max-Age=${COOKIE_MAX_AGE_SECONDS}; SameSite=Lax${secure}`
}

export function clearTouchCookie(): void {
  if (typeof document === 'undefined') return
  document.cookie = `${ACQUISITION_TOUCH_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`
}

/**
 * 合并规则：新触达等级更高才覆盖；否则只补旧包的空字段。
 * 返回 null 表示不需要写回。
 */
export function mergeTouch(
  existing: AcquisitionTouchWire | null,
  incoming: AcquisitionTouchWire
): AcquisitionTouchWire | null {
  if (!existing) return incoming
  const oldLevel = touchLevel(existing)
  const newLevel = touchLevel(incoming)
  if (newLevel > oldLevel) {
    return {
      ...incoming,
      p: incoming.p || existing.p,
      r: incoming.r || existing.r,
      s: incoming.s || existing.s,
      a: incoming.a || existing.a,
      u: {
        s: incoming.u.s || existing.u.s,
        m: incoming.u.m || existing.u.m,
        c: incoming.u.c || existing.u.c
      },
      c: incoming.c || existing.c,
      k: newLevel
    }
  }
  let changed = false
  const merged: AcquisitionTouchWire = { ...existing, u: { ...existing.u }, k: oldLevel }
  if (!merged.p && incoming.p) {
    merged.p = incoming.p
    changed = true
  }
  if (!merged.u.c && incoming.u.c) {
    merged.u.c = incoming.u.c
    changed = true
  }
  return changed ? merged : null
}

export interface CaptureResult {
  touch: AcquisitionTouchWire
  upgraded: boolean
}

/**
 * 首次页面加载时调用一次：采集、合并、写 Cookie。
 * upgraded 为 true 时才值得向服务端发信标（计访问、服务端续 Cookie）。
 */
export function captureLandingTouch(href: string, referrer: string, now = Date.now()): CaptureResult | null {
  const incoming = buildTouchFromLocation(href, referrer, now)
  const existing = readTouchCookie()
  const merged = mergeTouch(existing, incoming)
  if (!merged) return null
  writeTouchCookie(merged)
  const upgraded = !existing || touchLevel(merged) > touchLevel(existing)
  return { touch: merged, upgraded }
}
