import { useEffect, useState } from 'react'
import { ScrollText, RefreshCw, Search, CheckCircle2, XCircle, Clock, ChevronDown } from 'lucide-react'

interface AuditEntry {
  time: string
  action: string
  stack?: string
  provider?: string
  duration_ms?: number
  status?: string
}

const token = () => sessionStorage.getItem('herald_token') || ''

export default function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const [limit, setLimit] = useState(50)
  const [refreshing, setRefreshing] = useState(false)

  const load = (showSpinner = false) => {
    if (showSpinner) setRefreshing(true)
    setError('')
    fetch('/v2/audit', {
      headers: { 'Content-Type': 'application/json', ...(token() ? { Authorization: `Bearer ${token()}` } : {}) },
    })
      .then(r => r.json())
      .then(data => setEntries(Array.isArray(data) ? data : []))
      .catch(() => setError('Failed to load audit log'))
      .finally(() => { setLoading(false); setRefreshing(false) })
  }

  useEffect(() => { load() }, [])

  const filtered = entries.filter(e => {
    if (!search) return true
    const q = search.toLowerCase()
    return [e.action, e.stack, e.provider, e.status].some(v => v?.toLowerCase().includes(q))
  })

  const visible = filtered.slice(0, limit)

  const fmt = (iso: string) => {
    if (!iso) return '—'
    const d = new Date(iso)
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }
  const fmtDate = (iso: string) => new Date(iso).toLocaleDateString([], { month: 'short', day: 'numeric' })

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <div>
          <h1 className="text-2xl font-bold gradient-text">Audit Log</h1>
          <p className="text-slate-500 text-sm mt-0.5">All secret resolution and rotation activity</p>
        </div>
        <button
          onClick={() => load(true)}
          disabled={refreshing}
          className="flex items-center gap-1.5 text-sm text-slate-400 hover:text-slate-200 transition-colors disabled:opacity-50"
        >
          <RefreshCw size={13} className={refreshing ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {/* Search */}
      {!loading && entries.length > 0 && (
        <div className="relative mt-5 mb-4">
          <Search size={14} className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-500" />
          <input
            type="text"
            value={search}
            onChange={e => { setSearch(e.target.value); setLimit(50) }}
            placeholder="Filter by action, stack, provider…"
            className="w-full bg-white/4 border border-white/10 rounded-lg pl-9 pr-4 py-2.5 text-slate-200 placeholder-slate-600 focus:outline-none focus:border-cyan-500/40 text-sm transition-colors"
          />
          {search && (
            <button
              onClick={() => setSearch('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-600 hover:text-slate-300 text-xs"
            >
              ✕
            </button>
          )}
        </div>
      )}

      {error && (
        <div className="glass rounded-xl p-4 flex items-center gap-2 text-red-400 text-sm mb-4">
          <XCircle size={14} />
          {error}
        </div>
      )}

      {loading ? (
        <div className="glass rounded-xl p-6 space-y-3 animate-pulse mt-5">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="flex gap-4">
              <div className="h-3.5 bg-white/8 rounded w-16" />
              <div className="h-3.5 bg-white/8 rounded w-24" />
              <div className="h-3.5 bg-white/8 rounded w-20" />
              <div className="h-3.5 bg-white/8 rounded w-16 ml-auto" />
            </div>
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <div className="glass rounded-xl p-14 text-center mt-5">
          <ScrollText size={28} className="text-slate-700 mx-auto mb-3" />
          <div className="text-slate-400 font-medium mb-1">
            {search ? 'No matching entries' : 'No audit entries yet'}
          </div>
          <p className="text-slate-600 text-sm max-w-sm mx-auto">
            {search
              ? 'Try a different search term'
              : <>Activity appears here after the first materialize call. Run <code className="bg-white/5 px-1 py-0.5 rounded text-cyan-700">herald-agent sync</code> to get started.</>
            }
          </p>
        </div>
      ) : (
        <>
          <div className="glass rounded-xl overflow-hidden mt-5">
            {/* Summary bar */}
            <div className="flex items-center justify-between px-4 py-2.5 border-b border-white/5 text-xs text-slate-500">
              <span>{filtered.length} {search ? 'matching' : 'total'} entries</span>
              {search && <span>{entries.length} total</span>}
            </div>

            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-white/5">
                  <th className="text-left px-4 py-3 text-slate-600 font-medium text-xs uppercase tracking-wider w-28">Time</th>
                  <th className="text-left px-4 py-3 text-slate-600 font-medium text-xs uppercase tracking-wider">Action</th>
                  <th className="text-left px-4 py-3 text-slate-600 font-medium text-xs uppercase tracking-wider hidden md:table-cell">Stack</th>
                  <th className="text-left px-4 py-3 text-slate-600 font-medium text-xs uppercase tracking-wider hidden lg:table-cell">Provider</th>
                  <th className="text-right px-4 py-3 text-slate-600 font-medium text-xs uppercase tracking-wider hidden md:table-cell w-20">Duration</th>
                  <th className="text-left px-4 py-3 text-slate-600 font-medium text-xs uppercase tracking-wider w-20">Status</th>
                </tr>
              </thead>
              <tbody>
                {visible.map((e, i) => (
                  <tr key={i} className="border-b border-white/4 last:border-0 hover:bg-white/2 transition-colors">
                    <td className="px-4 py-3 text-slate-500 text-xs whitespace-nowrap">
                      <div>{fmt(e.time)}</div>
                      <div className="text-slate-700 text-[10px]">{fmtDate(e.time)}</div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-slate-200 font-medium text-xs">{e.action || '—'}</span>
                    </td>
                    <td className="px-4 py-3 text-slate-400 text-xs hidden md:table-cell">
                      {e.stack || <span className="text-slate-700">—</span>}
                    </td>
                    <td className="px-4 py-3 text-slate-500 text-xs hidden lg:table-cell">
                      {e.provider || <span className="text-slate-700">—</span>}
                    </td>
                    <td className="px-4 py-3 text-slate-400 text-xs text-right hidden md:table-cell">
                      {e.duration_ms != null ? `${e.duration_ms}ms` : <span className="text-slate-700">—</span>}
                    </td>
                    <td className="px-4 py-3">
                      {e.status ? (
                        <span className="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border"
                          style={
                            e.status === 'ok' || e.status === 'success'
                              ? { color: '#34d399', background: 'rgba(52,211,153,0.08)', borderColor: 'rgba(52,211,153,0.2)' }
                              : { color: '#f87171', background: 'rgba(248,113,113,0.08)', borderColor: 'rgba(248,113,113,0.2)' }
                          }
                        >
                          {e.status === 'ok' || e.status === 'success'
                            ? <CheckCircle2 size={9} />
                            : <XCircle size={9} />
                          }
                          {e.status}
                        </span>
                      ) : <span className="text-slate-700 text-xs">—</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {filtered.length > limit && (
            <button
              onClick={() => setLimit(l => l + 50)}
              className="mt-3 w-full py-2.5 text-sm text-slate-500 hover:text-slate-300 flex items-center justify-center gap-1.5 transition-colors"
            >
              <ChevronDown size={14} />
              Load {Math.min(50, filtered.length - limit)} more ({filtered.length - limit} remaining)
            </button>
          )}
        </>
      )}
    </div>
  )
}

// keep Clock imported
void Clock
