import { useEffect, useState } from 'react'
import { Shield, RefreshCw, Wifi, WifiOff, Clock, Gauge, AlertTriangle } from 'lucide-react'
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
    api.providersCheck()
      .then(data => setProviders(Array.isArray(data) ? data : []))
      .catch(() => {})
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
      <div className="flex items-center justify-between mb-2">
        <div>
          <h1 className="text-2xl font-bold gradient-text">Providers</h1>
          <p className="text-slate-500 text-sm mt-0.5">Health checks run every 60 s in the background</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={checkNow}
            disabled={checking}
            className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-lg border border-white/10 text-slate-400 hover:text-slate-200 hover:border-white/20 transition-colors disabled:opacity-50"
          >
            <RefreshCw size={12} className={checking ? 'animate-spin' : ''} />
            {checking ? 'Checking…' : 'Check now'}
          </button>
          <button onClick={load} className="flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-300 transition-colors px-2">
            <RefreshCw size={12} />
          </button>
        </div>
      </div>

      <div className="mt-6">
        {loading ? (
          <div className="space-y-3">
            {[...Array(2)].map((_, i) => (
              <div key={i} className="glass rounded-xl p-6 animate-pulse flex gap-4">
                <div className="w-10 h-10 bg-white/8 rounded-lg" />
                <div className="flex-1 space-y-2">
                  <div className="h-4 bg-white/8 rounded w-1/4" />
                  <div className="h-3 bg-white/8 rounded w-1/3" />
                </div>
              </div>
            ))}
          </div>
        ) : providers.length === 0 ? (
          <div className="glass rounded-xl p-14 text-center">
            <Shield size={28} className="text-slate-600 mx-auto mb-3" />
            <div className="text-slate-400 font-medium mb-2">No providers configured</div>
            <p className="text-slate-600 text-sm max-w-sm mx-auto">
              Set <code className="bg-white/5 px-1.5 py-0.5 rounded text-cyan-700">OP_CONNECT_SERVER_URL</code> and{' '}
              <code className="bg-white/5 px-1.5 py-0.5 rounded text-cyan-700">OP_CONNECT_TOKEN</code> to enable secret resolution.
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {providers.sort((a, b) => a.priority - b.priority).map(p => (
              <ProviderRow key={p.name} provider={p} ago={ago} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function ProviderRow({ provider: p, ago }: { provider: ProviderStatus; ago: (s: string) => string }) {
  const neverChecked = !p.checked_at
  const color = neverChecked ? '#64748b' : p.healthy ? '#34d399' : '#f87171'
  const StatusIcon = neverChecked ? Clock : p.healthy ? Wifi : WifiOff

  return (
    <div className="glass rounded-xl p-5">
      <div className="flex items-start gap-4">
        {/* Status icon */}
        <div
          className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0"
          style={{ background: `${color}15`, border: `1px solid ${color}30` }}
        >
          <StatusIcon size={18} style={{ color }} />
        </div>

        {/* Info */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2.5 flex-wrap">
            <span className="text-slate-100 font-semibold">{p.name}</span>
            <span className="text-xs px-2 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/20 text-cyan-400">{p.type || 'unknown'}</span>
            <span className="text-xs px-2 py-0.5 rounded bg-white/5 border border-white/10 text-slate-400">Priority #{p.priority + 1}</span>
            <span className="text-xs font-semibold" style={{ color }}>
              {neverChecked ? 'not yet checked' : p.healthy ? 'healthy' : 'unhealthy'}
            </span>
          </div>

          <div className="mt-3 flex items-center gap-6 text-sm">
            <div className="flex items-center gap-1.5 text-slate-500">
              <Gauge size={13} />
              <span className="text-slate-400">{p.latency_ms > 0 ? `${p.latency_ms} ms` : '—'}</span>
            </div>
            <div className="flex items-center gap-1.5 text-slate-500">
              <Clock size={13} />
              <span className="text-slate-400">Checked {ago(p.checked_at)}</span>
            </div>
          </div>
        </div>

        {/* Status dot */}
        <div className="flex items-center gap-2 shrink-0">
          <span className="relative flex h-2.5 w-2.5">
            <span
              className="absolute inline-flex h-full w-full rounded-full opacity-50"
              style={{ background: color, animation: p.healthy ? 'ping 1.5s cubic-bezier(0, 0, 0.2, 1) infinite' : 'none' }}
            />
            <span className="relative inline-flex rounded-full h-2.5 w-2.5" style={{ background: color }} />
          </span>
        </div>
      </div>

      {p.error && (
        <div className="mt-3 flex items-start gap-2 text-red-400 text-sm bg-red-500/8 border border-red-500/20 rounded-lg px-3 py-2.5">
          <AlertTriangle size={14} className="shrink-0 mt-0.5" />
          {p.error}
        </div>
      )}
    </div>
  )
}
