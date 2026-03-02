import { useEffect, useState } from 'react'
import { api, type Stats, type ProviderStatus } from '../lib/api'

export default function DashboardPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [providers, setProviders] = useState<ProviderStatus[]>([])
  const [error, setError] = useState('')

  const load = () => {
    api.stats().then(setStats).catch(() => setError('Failed to load stats'))
    api.providers().then(data => setProviders(Array.isArray(data) ? data : [])).catch(() => {})
  }

  useEffect(() => {
    load()
    const interval = setInterval(load, 30000)
    return () => clearInterval(interval)
  }, [])

  const fmt = (n: number) => n.toLocaleString()
  const uptime = (s: number) => {
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60)
    return h > 0 ? `${h}h ${m}m` : `${m}m`
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold gradient-text">Dashboard</h1>
        <button onClick={load} className="text-slate-400 hover:text-slate-200 text-sm transition-colors">↻ Refresh</button>
      </div>
      {error && <p className="text-red-400 text-sm mb-4">{error}</p>}

      {/* Stats row */}
      {stats ? (
        <div className="grid grid-cols-2 lg:grid-cols-3 gap-4 mb-8">
          <StatCard label="Cache Entries" value={fmt(stats.cache_entries)} color="var(--cyan)" />
          <StatCard label="Stacks Indexed" value={fmt(stats.stacks)} color="var(--emerald)" />
          <StatCard label="Uptime" value={uptime(stats.uptime_seconds)} color="var(--amber)" />
        </div>
      ) : (
        <div className="grid grid-cols-2 lg:grid-cols-3 gap-4 mb-8">
          {[...Array(3)].map((_, i) => <SkeletonCard key={i} />)}
        </div>
      )}

      {/* Provider health cards */}
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-slate-400 text-sm font-medium uppercase tracking-wider">Provider Health</h2>
      </div>

      {providers.length === 0 ? (
        <div className="glass rounded-xl p-8 text-center text-slate-500 text-sm">
          No providers configured or health data not yet available.
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {providers.map(p => (
            <ProviderCard key={p.name} provider={p} />
          ))}
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div className="glass rounded-xl p-5">
      <div className="text-slate-400 text-xs uppercase tracking-wider mb-2">{label}</div>
      <div className="text-3xl font-bold" style={{ color }}>{value}</div>
    </div>
  )
}

function SkeletonCard() {
  return (
    <div className="glass rounded-xl p-5 animate-pulse">
      <div className="h-3 bg-white/10 rounded mb-3 w-2/3" />
      <div className="h-8 bg-white/10 rounded w-1/2" />
    </div>
  )
}

function ProviderCard({ provider: p }: { provider: ProviderStatus }) {
  const neverChecked = !p.checked_at
  const statusColor = neverChecked ? '#64748b' : p.healthy ? '#34d399' : '#f87171'
  const statusLabel = neverChecked ? 'unknown' : p.healthy ? 'healthy' : 'unhealthy'

  const ago = (iso: string) => {
    if (!iso) return 'never'
    const d = Math.round((Date.now() - new Date(iso).getTime()) / 1000)
    if (d < 60) return `${d}s ago`
    if (d < 3600) return `${Math.floor(d / 60)}m ago`
    return `${Math.floor(d / 3600)}h ago`
  }

  return (
    <div className="glass rounded-xl p-5 space-y-3">
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2">
          <span
            className="w-2.5 h-2.5 rounded-full shrink-0 mt-0.5"
            style={{ background: statusColor, boxShadow: neverChecked ? 'none' : `0 0 6px ${statusColor}` }}
          />
          <span className="font-semibold text-slate-100 text-sm leading-tight">{p.name}</span>
        </div>
        <span className="text-xs px-2 py-0.5 rounded-full bg-white/5 border border-white/10 text-slate-400 shrink-0">
          #{p.priority + 1}
        </span>
      </div>

      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs px-2 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/20 text-cyan-400">
          {p.type || 'unknown'}
        </span>
        <span className="text-xs text-slate-500" style={{ color: statusColor }}>{statusLabel}</span>
      </div>

      <div className="grid grid-cols-2 gap-2 text-xs">
        <div>
          <div className="text-slate-600 mb-0.5">Latency</div>
          <div className="text-slate-300">{p.latency_ms > 0 ? `${p.latency_ms}ms` : '—'}</div>
        </div>
        <div>
          <div className="text-slate-600 mb-0.5">Checked</div>
          <div className="text-slate-300">{ago(p.checked_at)}</div>
        </div>
      </div>

      {p.error && (
        <div className="text-red-400 text-xs bg-red-500/10 border border-red-500/20 rounded px-2 py-1.5 break-words">
          {p.error}
        </div>
      )}
    </div>
  )
}
