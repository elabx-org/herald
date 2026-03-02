import { useEffect, useState } from 'react'

interface AuditEntry {
  time: string
  action: string
  stack?: string
  provider?: string
  duration_ms?: number
  status?: string
}

export default function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const token = () => sessionStorage.getItem('herald_token') || ''

  const load = () => {
    setLoading(true)
    setError('')
    fetch('/v2/audit', {
      headers: {
        'Content-Type': 'application/json',
        ...(token() ? { Authorization: `Bearer ${token()}` } : {}),
      },
    })
      .then(r => r.json())
      .then(data => setEntries(Array.isArray(data) ? data : []))
      .catch(() => setError('Failed to load audit log'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [])

  const fmt = (iso: string) => {
    if (!iso) return '—'
    return new Date(iso).toLocaleString()
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold gradient-text">Audit Log</h1>
        <button onClick={load} className="text-slate-400 hover:text-slate-200 text-sm transition-colors">
          ↻ Refresh
        </button>
      </div>

      <p className="text-slate-500 text-sm mb-6">
        Activity log for all secret resolution and rotation operations.
      </p>

      {error && <p className="text-red-400 text-sm mb-4">{error}</p>}

      {loading ? (
        <div className="glass rounded-xl p-6 animate-pulse space-y-3">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-4 bg-white/10 rounded w-full" />
          ))}
        </div>
      ) : entries.length === 0 ? (
        <div className="glass rounded-xl p-12 text-center">
          <div className="text-slate-400 text-base font-medium mb-2">No audit entries yet</div>
          <p className="text-slate-600 text-sm max-w-md mx-auto">
            Activity will appear here after the first materialize call. Run{' '}
            <code className="bg-white/5 px-1.5 py-0.5 rounded text-cyan-700">herald-agent sync</code> to get started.
          </p>
        </div>
      ) : (
        <div className="glass rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-white/5">
                <th className="text-left px-4 py-3 text-slate-500 font-medium text-xs uppercase tracking-wider">Time</th>
                <th className="text-left px-4 py-3 text-slate-500 font-medium text-xs uppercase tracking-wider">Action</th>
                <th className="text-left px-4 py-3 text-slate-500 font-medium text-xs uppercase tracking-wider">Stack</th>
                <th className="text-left px-4 py-3 text-slate-500 font-medium text-xs uppercase tracking-wider">Provider</th>
                <th className="text-left px-4 py-3 text-slate-500 font-medium text-xs uppercase tracking-wider">Duration</th>
                <th className="text-left px-4 py-3 text-slate-500 font-medium text-xs uppercase tracking-wider">Status</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e, i) => (
                <tr key={i} className="border-b border-white/5 last:border-0 hover:bg-white/3 transition-colors">
                  <td className="px-4 py-3 text-slate-400 whitespace-nowrap">{fmt(e.time)}</td>
                  <td className="px-4 py-3 text-slate-200">{e.action || '—'}</td>
                  <td className="px-4 py-3 text-slate-300">{e.stack || '—'}</td>
                  <td className="px-4 py-3 text-slate-400">{e.provider || '—'}</td>
                  <td className="px-4 py-3 text-slate-400">
                    {e.duration_ms != null ? `${e.duration_ms}ms` : '—'}
                  </td>
                  <td className="px-4 py-3">
                    {e.status ? (
                      <span
                        className="text-xs px-2 py-0.5 rounded-full border"
                        style={
                          e.status === 'ok' || e.status === 'success'
                            ? { color: '#34d399', background: 'rgba(52,211,153,0.08)', borderColor: 'rgba(52,211,153,0.2)' }
                            : { color: '#f87171', background: 'rgba(248,113,113,0.08)', borderColor: 'rgba(248,113,113,0.2)' }
                        }
                      >
                        {e.status}
                      </span>
                    ) : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
