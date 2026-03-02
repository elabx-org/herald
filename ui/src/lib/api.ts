const token = () => sessionStorage.getItem('herald_token') || ''

const headers = () => ({
  'Content-Type': 'application/json',
  ...(token() ? { Authorization: `Bearer ${token()}` } : {}),
})

export interface Stats {
  cache_entries: number
  providers: string[]
  stacks: number
  uptime_seconds: number
}

export interface StackEntry {
  stack: string
  items: string[]
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

export const api = {
  async stats(): Promise<Stats> {
    const r = await fetch('/v2/stats', { headers: headers() })
    return r.json()
  },

  async inventory(): Promise<StackEntry[]> {
    const r = await fetch('/v2/inventory', { headers: headers() })
    return r.json()
  },

  async rotate(item: string, vault?: string): Promise<RotateResult> {
    const url = vault ? `/v2/rotate/${vault}/${item}` : `/v2/rotate/${item}`
    const r = await fetch(url, { method: 'POST', headers: headers() })
    return r.json()
  },

  async cacheFlush(): Promise<{ ok: boolean }> {
    const r = await fetch('/v2/cache', { method: 'DELETE', headers: headers() })
    return r.json()
  },

  async cacheFlushStack(stack: string): Promise<{ flushed: number }> {
    const r = await fetch(`/v2/cache/${stack}`, { method: 'DELETE', headers: headers() })
    return r.json()
  },
}
