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
}

export const api = {
  stats: () => fetchJSON<Stats>('/v2/stats'),
  inventory: () => fetchJSON<StackEntry[]>('/v2/inventory'),
  providers: () => fetchJSON<ProviderStatus[]>('/v2/providers'),

  rotate: (item: string, vault?: string) =>
    fetchJSON<RotateResult>(vault ? `/v2/rotate/${vault}/${item}` : `/v2/rotate/${item}`, { method: 'POST' }),

  cacheFlush: () => fetchJSON<{ ok: boolean }>('/v2/cache', { method: 'DELETE' }),
  cacheFlushStack: (stack: string) => fetchJSON<{ flushed: number }>(`/v2/cache/${stack}`, { method: 'DELETE' }),
}
