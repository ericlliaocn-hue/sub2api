import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UpstreamConnectionsView from '../UpstreamConnectionsView.vue'

const { listConnections } = vi.hoisted(() => ({ listConnections: vi.fn() }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    upstreamConnections: {
      list: listConnections,
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
      test: vi.fn()
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show" data-test="dialog"><slot /></div>'
}

describe('admin UpstreamConnectionsView', () => {
  beforeEach(() => {
    listConnections.mockReset()
    listConnections.mockResolvedValue([])
  })

  it('renders the empty state and defaults the add form to Sub2API', async () => {
    const wrapper = mount(UpstreamConnectionsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          BaseDialog: BaseDialogStub,
          Icon: true
        }
      }
    })

    await flushPromises()
    expect(wrapper.text()).toContain('admin.upstreamConnections.empty')

    await wrapper.get('button.btn-primary').trigger('click')
    const typeSelect = wrapper.get('select')
    expect((typeSelect.element as HTMLSelectElement).value).toBe('sub2api')
    expect(typeSelect.findAll('option').map((option) => option.attributes('value'))).toEqual([
      'sub2api',
      'newapi',
      'other'
    ])
    expect(wrapper.get('input[type="password"]').exists()).toBe(true)
  })
})
