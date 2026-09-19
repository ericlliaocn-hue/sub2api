export const DEFAULT_CREATE_ACCOUNT_PROXY_NAME = 'JP-WARP-SSH-103'

export function pickDefaultCreateAccountProxyId(
  proxies: Array<{ id: number; name?: string | null; status?: string | null }>
): number | null {
  const found = proxies.find(
    (proxy) =>
      proxy.name === DEFAULT_CREATE_ACCOUNT_PROXY_NAME &&
      (proxy.status == null || proxy.status === 'active')
  )
  return found?.id ?? null
}
