import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import WatchedTrafficCard from '../WatchedTrafficCard.vue'

const { getConfig, updateConfig, listLogs, showSuccess } = vi.hoisted(() => ({
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  listLogs: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    watchedTraffic: {
      getConfig,
      updateConfig,
      listLogs,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess,
  }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_err: unknown, fallback: string) => fallback,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('WatchedTrafficCard', () => {
  beforeEach(() => {
    getConfig.mockResolvedValue({ enabled: true, user_ids: [295] })
    updateConfig.mockImplementation(async (payload: { enabled?: boolean; user_ids?: number[] }) => ({
      enabled: payload.enabled ?? true,
      user_ids: payload.user_ids ?? [295],
    }))
    listLogs.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
  })

  it('turns recording off with one click', async () => {
    const wrapper = mount(WatchedTrafficCard, {
      global: {
        stubs: {
          BaseDialog: true,
          Toggle: {
            props: ['modelValue', 'disabled'],
            emits: ['update:modelValue'],
            template: '<button data-test="toggle" @click="$emit(\'update:modelValue\', false)" />',
          },
        },
      },
    })
    await flushPromises()
    await wrapper.get('[data-test="toggle"]').trigger('click')
    await flushPromises()
    expect(updateConfig).toHaveBeenCalledWith({ enabled: false })
    expect(showSuccess).toHaveBeenCalled()
  })
})
