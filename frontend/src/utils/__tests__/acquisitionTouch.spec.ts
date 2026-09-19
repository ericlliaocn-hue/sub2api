import { beforeEach, describe, expect, it } from 'vitest'
import {
  ACQUISITION_TOUCH_COOKIE,
  TOUCH_LEVEL,
  buildTouchFromLocation,
  captureLandingTouch,
  clearTouchCookie,
  isSearchHost,
  mergeTouch,
  readTouchCookie,
  touchLevel,
  writeTouchCookie
} from '../acquisitionTouch'

describe('acquisitionTouch', () => {
  beforeEach(() => {
    clearTouchCookie()
  })

  it('classifies search hosts including AI search', () => {
    expect(isSearchHost('https://www.baidu.com/s?wd=x')).toBe(true)
    expect(isSearchHost('www.google.com.hk')).toBe(true)
    expect(isSearchHost('chatgpt.com')).toBe(true)
    expect(isSearchHost('kimi.moonshot.cn')).toBe(true)
    expect(isSearchHost('t.me')).toBe(false)
    expect(isSearchHost('notgoogle.com')).toBe(false)
  })

  it('builds touch from landing url and computes level', () => {
    const touch = buildTouchFromLocation('https://anytoken.work/register?source=tg1&aff=ABC&gclid=1', 'https://www.google.com/', 1000)
    expect(touch.s).toBe('TG1')
    expect(touch.a).toBe('ABC')
    expect(touch.p).toBe('/register')
    expect(touch.r).toBe('google.com')
    expect(touch.c).toBe(true)
    expect(touch.k).toBe(TOUCH_LEVEL.source)

    expect(touchLevel(buildTouchFromLocation('https://anytoken.work/', '', 1))).toBe(TOUCH_LEVEL.official)
    expect(touchLevel(buildTouchFromLocation('https://anytoken.work/', 'https://www.baidu.com/', 1))).toBe(TOUCH_LEVEL.search)
    expect(touchLevel(buildTouchFromLocation('https://anytoken.work/?utm_medium=cpc', 'https://www.baidu.com/', 1))).toBe(TOUCH_LEVEL.paid)
    expect(touchLevel(buildTouchFromLocation('https://anytoken.work/', 'https://t.me/', 1))).toBe(TOUCH_LEVEL.external)
  })

  it('own-site referrer is treated as official', () => {
    const own = `${window.location.protocol}//${window.location.host}/pricing`
    const touch = buildTouchFromLocation(`${window.location.origin}/register`, own, 1)
    expect(touch.r).toBe('')
    expect(touchLevel(touch)).toBe(TOUCH_LEVEL.official)
  })

  it('round-trips through the cookie', () => {
    const touch = buildTouchFromLocation('https://anytoken.work/docs?utm_campaign=中文', 'https://www.bing.com/', 12345)
    writeTouchCookie(touch)
    expect(document.cookie).toContain(`${ACQUISITION_TOUCH_COOKIE}=`)
    const read = readTouchCookie()
    expect(read).not.toBeNull()
    expect(read!.r).toBe('bing.com')
    expect(read!.u.c).toBe('中文')
    expect(read!.t).toBe(12345)
  })

  it('only upgrades, never downgrades', () => {
    const search = buildTouchFromLocation('https://anytoken.work/', 'https://www.baidu.com/', 1)
    const official = buildTouchFromLocation('https://anytoken.work/pricing', '', 2)
    const source = buildTouchFromLocation('https://anytoken.work/?source=tg1', '', 3)

    // 搜索之后逛官网：不变
    expect(mergeTouch(search, official)).toBeNull()
    // 搜索之后点了推广链接：升级，保留搜索 referrer 作为补充证据
    const upgraded = mergeTouch(search, source)
    expect(upgraded).not.toBeNull()
    expect(upgraded!.s).toBe('TG1')
    expect(upgraded!.r).toBe('baidu.com')
    expect(upgraded!.k).toBe(TOUCH_LEVEL.source)
    // 推广之后再从搜索来：不降级
    expect(mergeTouch(source, search)).toBeNull()
  })

  it('captureLandingTouch reports upgraded only when level rises', () => {
    const first = captureLandingTouch('https://anytoken.work/', '', 1)
    expect(first?.upgraded).toBe(true)
    const again = captureLandingTouch('https://anytoken.work/docs', '', 2)
    expect(again === null || again.upgraded === false).toBe(true)
    const promo = captureLandingTouch('https://anytoken.work/?aff=XYZ', '', 3)
    expect(promo?.upgraded).toBe(true)
    expect(readTouchCookie()?.a).toBe('XYZ')
  })
})
