import { useEffect, useState } from 'react'
import { api, type ProviderStatus } from '../lib/api'

export default function ProvidersPage() {
  const [providers, setProviders] = useState<ProviderStatus[]>([])
  const [loading, setLoading] = useState(true)
  const [checking, setChecking] = useState(false)

  const load = () => {
    setLoading(true)
    api.providers()
      .then(data => setProviders(Array.isArray(data) ? data : []))
      .finally(() => setLoading(false))
  }

  const checkNow = () => {
    setChecking(true)
    api.providers()
      .then(data => setProviders(Array.isArray(data) ? data : []))
      .finally(() => setChecking(false))
  }

  useEffect(() => { load() }, [])

  const ago = (iso: string) => {
    if (!iso) return 'never'
    const d = Math.round((Date.now() - new Date(iso).getTime()) / 1000)
    if (d < 60) return `${d}s ago`
    if (d < 3600) return `${Math.floor(d / 60)}m ago`
    return `${Math.floor(d / 3600)}h ago`
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold gradient-text">Providers</h1>
        <div className="flex gap-2">
          <button
            onClick={checkNow}
            disabled={checking}
            className="text-slate-400 hover:text-slate-200 text-sm transition-colors disabled:opacity-50"
          >
            {checking ? 'Checking...' : 'Check now'}
          </button>
          <button onClick={load} className="text-slate-400 hover:text-slate-200 text-sm transition-colors">
            ↻ Refresh
          </button>
        </div>
      </div>

      <p className="text-slate-500 text-sm mb-6">
        1Password providers configured for this Herald instance. Health checks run every 60 seconds in the background.
      </p>

      {loading ? (
        <div className="space-y-4">
          {[...Array(2)].map((_, i) => (
            <div key={i} className="glass rounded-xl p-6 animate-pulse">
              <div className="h-5 bg-white/10 rounded w-1/3 mb-3" />
              <div className="h-3 bg-white/10 rounded w-1/2" />
            </div>
          ))}
        </div>
      ) : providers.length === 0 ? (
        <div className="glass rounded-xl p-12 text-center">
          <div className="text-slate-400 text-lg font-medium mb-2">No providers configured</div>
          <p className="text-slate-600 text-sm max-w-md mx-auto">
            Configure a 1Password Connect server or service account to enable secret resolution.
            Set <code className="bg-white/5 px-1.5 py-0.5 rounded text-cyan-700">OP_CONNECT_SERVER_URL</code> and{' '}
            <code className="bg-white/5 px-1.5 py-0.5 rounded text-cyan-700">OP_CONNECT_TOKEN</code> environment variables.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {providers.sort((a, b) => a.priority - b.priority).map(p => {
            const neverChecked = !p.checked_at
            const statusColor = neverChecked ? '#64748b' : p.healthy ? '#34d399' : '#f87171'
            const statusLabel = neverChecked ? 'unknown' : p.healthy ? 'Healthy' : 'Unhealthy'

            return (
              <div key={p.name} className="glass rounded-xl p-6">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex items-start gap-3">
                    <span
                      className="w-3 h-3 rounded-full shrink-0 mt-1"
                      style={{
                        background: statusColor,
                        boxShadow: neverChecked ? 'none' : `0 0 8px ${statusColor}`,
                      }}
                    />
                    <div>
                      <div className="flex items-center gap-2 mb-1">
                        <span className="text-slate-100 font-semibold text-base">{p.name}</span>
                        <span className="text-xs px-2 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/20 text-cyan-400">
                          {p.type || 'unknown'}
                        </span>
                        <span className="text-xs px-2 py-0.5 rounded bg-white/5 border border-white/10 text-slate-400">
                          Priority #{p.priority + 1}
                        </span>
                      </div>
                      <div className="flex items-center gap-4 text-sm mt-2">
                        <span style={{ color: statusColor }} className="font-medium">{statusLabel}</span>
                        <span className="text-slate-500">
                          Latency: <span className="text-slate-300">{p.latency_ms > 0 ? `${p.latency_ms}ms` : '—'}</span>
                        </span>
                        <span className="text-slate-500">
                          Last checked: <span className="text-slate-300">{ago(p.checked_at)}</span>
                        </span>
                      </div>
                      {p.error && (
                        <div className="mt-3 text-red-400 text-sm bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
                          {p.error}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
