import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserBonusGrantModal from '../UserBonusGrantModal.vue'

const { grantExpiringBonus, showSuccess, showError } = vi.hoisted(() => ({
  grantExpiringBonus: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { users: { grantExpiringBonus } }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key
  })
}))

const mountModal = () => mount(UserBonusGrantModal, {
  props: { show: true, selectedIds: [4, 7] },
  global: {
    stubs: {
      BaseDialog: {
        props: ['show', 'title'],
        emits: ['close'],
        template: '<div v-if="show"><slot /><slot name="footer" /></div>'
      }
    }
  }
})

describe('UserBonusGrantModal', () => {
  beforeEach(() => {
    grantExpiringBonus.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    grantExpiringBonus.mockResolvedValue({ affected: 2 })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('submits an atomic batch with an absolute expiration and idempotency key', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mountModal()

    await wrapper.get('[data-test="bonus-amount"]').setValue('5')
    await wrapper.get('[data-test="bonus-expiry"]').setValue('2099-08-30T23:59')
    await wrapper.get('[data-test="bonus-campaign"]').setValue('weekend-20990830')
    await wrapper.get('[data-test="bonus-notes"]').setValue('weekend campaign')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(grantExpiringBonus).toHaveBeenCalledWith({
      user_ids: [4, 7],
      amount: 5,
      expires_at: new Date('2099-08-30T23:59').toISOString(),
      campaign_id: 'weekend-20990830',
      notes: 'weekend campaign'
    }, expect.any(String))
    expect(wrapper.emitted('success')).toEqual([[2]])
  })

  it('keeps the operation id stable when a retry follows a failed request', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    grantExpiringBonus.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({ affected: 2 })
    const wrapper = mountModal()

    await wrapper.get('[data-test="bonus-amount"]').setValue('5')
    await wrapper.get('[data-test="bonus-expiry"]').setValue('2099-08-30T23:59')
    await wrapper.get('[data-test="bonus-campaign"]').setValue('weekend-20990830')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(grantExpiringBonus).toHaveBeenCalledTimes(2)
    expect(grantExpiringBonus.mock.calls[0][1]).toBe(grantExpiringBonus.mock.calls[1][1])
  })

  it('does not send the grant when confirmation is cancelled', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const wrapper = mountModal()
    await wrapper.get('[data-test="bonus-amount"]').setValue('5')
    await wrapper.get('[data-test="bonus-expiry"]').setValue('2099-08-30T23:59')
    await wrapper.get('[data-test="bonus-campaign"]').setValue('weekend-20990830')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(grantExpiringBonus).not.toHaveBeenCalled()
  })
})
