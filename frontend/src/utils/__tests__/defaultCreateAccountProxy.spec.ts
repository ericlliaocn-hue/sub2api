import { describe, expect, it } from 'vitest'
import { pickDefaultCreateAccountProxyId } from '../defaultCreateAccountProxy'

describe('pickDefaultCreateAccountProxyId', () => {
  it('selects JP-WARP-SSH-103', () => {
    expect(
      pickDefaultCreateAccountProxyId([
        { id: 2, name: 'SG-OpenAI-01', status: 'active' },
        { id: 5, name: 'JP-WARP-SSH-103', status: 'active' },
      ])
    ).toBe(5)
  })

  it('returns null when the default proxy is missing', () => {
    expect(pickDefaultCreateAccountProxyId([{ id: 2, name: 'SG-OpenAI-01', status: 'active' }])).toBeNull()
  })
})
