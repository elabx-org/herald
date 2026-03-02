import { useEffect, useState } from 'react'
import { api, type StackEntry } from '../lib/api'

export default function InventoryPage() {
  const [stacks, setStacks] = useState<StackEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [flushing, setFlushing] = useState<string | null>(null)
  const [msg, setMsg] = useState('')

  const load = () => {
    setLoading(true)
    api.inventory()
      .then(data => setStacks(Array.isArray(data) ? data : []))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [])

  const flushStack = async (stack: string) => {
    setFlushing(stack)
    setMsg('')
    const r = await api.cacheFlushStack(stack)
    setMsg(`Flushed ${r.flushed} cache entries for "${stack}"`)
    setFlushing(null)
    load()
  }

  const ago = (iso: string) => {
    const d = Math.round((Date.now() - new Date(iso).getTime()) / 1000)
    if (d < 60) return `${d}s ago`
    if (d < 3600) return `${Math.floor(d / 60)}m ago`
    return `${Math.floor(d / 3600)}h ago`
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold gradient-text">Inventory</h1>
        <button onClick={load} className="text-slate-400 hover:text-slate-200 text-sm transition-colors">↻ Refresh</button>
      </div>

      <p className="text-slate-500 text-sm mb-6">
        Stacks that have resolved secrets through Herald. Each entry shows which 1Password items were accessed.
      </p>

      {msg && <p className="text-emerald-400 text-sm mb-4">{msg}</p>}

      {loading ? (
        <div className="space-y-3">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="glass rounded-xl p-5 animate-pulse">
              <div className="h-4 bg-white/10 rounded w-1/4 mb-3" />
              <div className="h-3 bg-white/10 rounded w-2/3" />
            </div>
          ))}
        </div>
      ) : stacks.length === 0 ? (
        <div className="glass rounded-xl p-10 text-center">
          <div className="text-slate-400 text-base font-medium mb-2">No stacks indexed yet</div>
          <p className="text-slate-600 text-sm mb-6 max-w-md mx-auto">
            Stacks appear here after the first materialize call. Run <code className="bg-white/5 px-1.5 py-0.5 rounded text-cyan-700">herald-agent sync</code> to populate.
          </p>
          <div className="bg-white/5 border border-white/10 rounded-lg p-4 text-left inline-block">
            <code className="text-cyan-400 text-sm">herald-agent sync --stack myapp --out /run/secrets/.env</code>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {stacks.sort((a, b) => a.stack.localeCompare(b.stack)).map(s => (
            <div key={s.stack} className="glass rounded-xl p-5">
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-3 mb-2">
                    <span className="font-semibold text-slate-100">{s.stack}</span>
                    <span className="text-xs text-slate-500">{ago(s.last_seen)}</span>
                    <span className="text-xs text-slate-600">{s.resolve_count} resolves</span>
                  </div>
                  <div className="flex flex-wrap gap-1">
                    {(s.items || []).map(item => (
                      <span key={item} className="text-xs px-2 py-0.5 rounded-full bg-cyan-500/10 text-cyan-400 border border-cyan-500/20">
                        {item}
                      </span>
                    ))}
                  </div>
                </div>
                <button
                  onClick={() => flushStack(s.stack)}
                  disabled={flushing === s.stack}
                  className="shrink-0 text-xs px-3 py-1.5 rounded-lg border border-white/10 text-slate-400 hover:text-slate-200 hover:border-white/20 transition-colors disabled:opacity-50"
                >
                  {flushing === s.stack ? '...' : 'Flush cache'}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
