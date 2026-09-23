import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import UsagePressureBoard from '../UsagePressureBoard.vue'

const getUsagePressure = vi.fn()
const getDashboardPressure = vi.fn()

vi.mock('@/api/admin/dashboard', () => ({
  getUsagePressure: (...args: unknown[]) => getUsagePressure(...args),
}))

vi.mock('@/api/usage', () => ({
  getDashboardPressure: (...args: unknown[]) => getDashboardPressure(...args),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (params?.n != null) return `${key}:${params.n}`
        if (params?.pct != null) return `${key}:${params.pct}`
        if (params?.hour) return `${key}:${params.hour}`
        if (params?.users != null) return `${key}:${params.users}/${params.accounts}`
        return key
      },
    }),
  }
})

const snapshot = {
  generated_at: '2026-09-20T13:05:00Z',
  level: 'warm',
  hourly_from_15m: 16.84,
  peak_ratio: 0.3368,
  windows: {
    m5: { requests: 4, users: 2, accounts: 1, billed: 1.2 },
    m15: { requests: 10, users: 6, accounts: 2, billed: 4.21 },
    m60: { requests: 30, users: 8, accounts: 3, billed: 12.5 },
  },
  today: { requests: 80, users: 12, accounts: 4, billed: 40 },
  peak_hour: { hour: '2026-09-20T06:00:00Z', requests: 20, users: 9, billed: 50 },
  users: [
    { id: 163, name: 'ericlliao@test.com', requests: 5, billed: 2.1, last_at: '2026-09-20T13:03:00Z' },
  ],
  accounts: [
    { id: 66274, name: 'linda', requests: 8, billed: 4, last_at: '2026-09-20T13:04:00Z' },
  ],
}

const mountBoard = () =>
  mount(UsagePressureBoard, {
    global: { stubs: { LoadingSpinner: true } },
  })

describe('UsagePressureBoard', () => {
  let wrapper: ReturnType<typeof mountBoard> | undefined

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-20T13:05:00Z'))
    getUsagePressure.mockReset()
    getDashboardPressure.mockReset()
    getUsagePressure.mockResolvedValue(snapshot)
    getDashboardPressure.mockResolvedValue(snapshot)
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    vi.useRealTimers()
  })

  it('renders live mood, windows, who is on, and emits select-user', async () => {
    wrapper = mountBoard()
    await flushPromises()

    expect(getUsagePressure).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('admin.usage.pressure.levels.warm')
    expect(wrapper.text()).toContain('$4.21')
    expect(wrapper.text()).toContain('ericlliao@test.com')
    expect(wrapper.text()).toContain('linda')
    expect(wrapper.text()).toContain('admin.usage.pressure.peakRatio:34')

    await wrapper.find('[data-testid="pressure-user-163"]').trigger('click')
    expect(wrapper.emitted('select-user')![0]).toEqual([163, 'ericlliao@test.com'])
  })

  it('polls every 30 seconds', async () => {
    wrapper = mountBoard()
    await flushPromises()
    expect(getUsagePressure).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(30_000)
    await flushPromises()
    expect(getUsagePressure).toHaveBeenCalledTimes(2)
  })

  it('shows empty copy when the live window is idle', async () => {
    getUsagePressure.mockResolvedValue({
      ...snapshot,
      level: 'idle',
      hourly_from_15m: 0,
      peak_ratio: 0,
      windows: {
        m5: { requests: 0, users: 0, accounts: 0, billed: 0 },
        m15: { requests: 0, users: 0, accounts: 0, billed: 0 },
        m60: { requests: 0, users: 0, accounts: 0, billed: 0 },
      },
      peak_hour: null,
      users: [],
      accounts: [],
    })
    wrapper = mountBoard()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.usage.pressure.nobody')
    expect(wrapper.text()).toContain('admin.usage.pressure.vsPeakNone')
  })

  it('hides who/account chips for the user-facing board', async () => {
    wrapper = mount(UsagePressureBoard, {
      props: { source: 'user' },
      global: { stubs: { LoadingSpinner: true } },
    })
    await flushPromises()

    expect(getDashboardPressure).toHaveBeenCalledTimes(1)
    expect(getUsagePressure).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="pressure-crowd-only"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pressure-user-163"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.usage.pressure.crowdOnly:6/2')
  })
})
