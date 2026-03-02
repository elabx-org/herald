const token = () => sessionStorage.getItem('herald_token') || ''

const authHeaders = () => ({
  'Content-Type': 'application/json',
  ...(token() ? { Authorization: `Bearer ${token()}` } : {}),
})

async function fetchJSON<T>(url: string, opts?: RequestInit): Promise<T> {
  const r = await fetch(url, {
    ...opts,
    headers: { ...authHeaders(), ...(opts?.headers as Record<string, string> ?? {}) },
  })
  if (r.status === 401) {
    sessionStorage.removeItem('herald_token')
    window.location.reload()
    throw new Error('Unauthorized')
  }
  if (!r.ok) {
    throw new Error(`HTTP ${r.status}: ${r.statusText}`)
  }
  if (r.status === 204 || r.status === 205) {
    return undefined as T
  }
  return r.json()
}

export interface Stats {
  cache_entries: number
  providers: string[]
  stacks: number
  uptime_seconds: number
}

export interface StackEntry {
  stack: string
  items: string[]
  refs?: string[]
  last_seen: string
  resolve_count: number
}

export interface RotateResult {
  item_id: string
  vault: string
  cache_invalidated: number
  stacks_redeployed: string[]
  errors: string[]
}

export interface ProviderStatus {
  name: string
  type: string
  priority: number
  healthy: boolean
  latency_ms: number
  error?: string
  checked_at: string
  url?: string
  source: 'env' | 'db'
}

export interface ProviderRequest {
  name: string
  type: '1password-connect' | '1password-sdk' | 'mock'
  priority: number
  url?: string
  token?: string
}

export interface CacheEntry {
  key: string
  provider: string
  expires_at: string
  stale: boolean
}

export interface AuditEntry {
  ts: string
  action: string
  stack?: string
  secret?: string
  provider?: string
  duration_ms?: number
  policy?: string
  error?: string
}

export const api = {
  stats: () => fetchJSON<Stats>('/v2/stats'),
  inventory: () => fetchJSON<StackEntry[]>('/v2/inventory'),
  providers: () => fetchJSON<ProviderStatus[]>('/v2/providers'),
  providersCheck: () => fetchJSON<ProviderStatus[]>('/v2/providers/check', { method: 'POST' }),

  createProvider: (req: ProviderRequest) =>
    fetchJSON<{ name: string; source: string }>('/v2/providers', {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  updateProvider: (name: string, req: Omit<ProviderRequest, 'name'>) =>
    fetchJSON<{ name: string; source: string }>(`/v2/providers/${encodeURIComponent(name)}`, {
      method: 'PUT',
      body: JSON.stringify(req),
    }),

  deleteProvider: (name: string) =>
    fetchJSON<void>(`/v2/providers/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  rotate: (item: string, vault?: string) =>
    fetchJSON<RotateResult>(vault ? `/v2/rotate/${vault}/${item}` : `/v2/rotate/${item}`, { method: 'POST' }),

  cacheList: () => fetchJSON<CacheEntry[]>('/v2/cache'),
  cacheFlush: () => fetchJSON<{ ok: boolean }>('/v2/cache', { method: 'DELETE' }),
  cacheFlushStack: (stack: string) => fetchJSON<{ flushed: number }>(`/v2/cache/${stack}`, { method: 'DELETE' }),

  audit: (params?: { stack?: string; secret?: string }) => {
    const q = new URLSearchParams()
    if (params?.stack) q.set('stack', params.stack)
    if (params?.secret) q.set('secret', params.secret)
    const qs = q.toString()
    return fetchJSON<AuditEntry[]>(`/v2/audit${qs ? '?' + qs : ''}`)
  },

  provision: (vault: string, item: string, field: string, value: string) =>
    fetchJSON<{ ok: boolean }>('/v2/provision', {
      method: 'POST',
      body: JSON.stringify({ vault, item, field, value }),
    }),
}
