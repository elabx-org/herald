import { useEffect, useState } from 'react'
import { HardDrive, Layers, Clock, RefreshCw, Database, ArrowRight } from 'lucide-react'

import { api, type Stats, type ProviderStatus } from '../lib/api'

interface Props { onNavigate: (page: string) => void }

export default function DashboardPage({ onNavigate }: Props) {
  const [stats, setStats] = useState<Stats | null>(null)
  const [providers, setProviders] = useState<ProviderStatus[]>([])
  const [error, setError] = useState('')
  const [refreshing, setRefreshing] = useState(false)

  const load = async () => {
    setRefreshing(true)
    try {
      const [s, p] = await Promise.all([
        api.stats().catch(() => null),
        api.providers().catch(() => []),
      ])
      if (s) setStats(s)
      else setError('Failed to load stats')
      setProviders(Array.isArray(p) ? p : [])
    } finally {
      setRefreshing(false)
    }
  }

  useEffect(() => {
    load()
    const t = setInterval(load, 30_000)
    return () => clearInterval(t)
  }, [])

  const uptime = (s: number) => {
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60)
    return h > 0 ? `${h}h ${m}m` : `${m}m`
  }

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-7">
        <div>
          <h1 className="text-2xl font-bold gradient-text">Dashboard</h1>
          <p className="text-slate-500 text-sm mt-0.5">Herald secret management overview</p>
        </div>
        <button
          onClick={load}
          disabled={refreshing}
          className="flex items-center gap-1.5 text-slate-400 hover:text-slate-200 text-sm transition-colors disabled:opacity-50"
        >
          <RefreshCw size={13} className={refreshing ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {error && <p className="text-red-400 text-sm mb-4">{error}</p>}

      {/* Stat cards */}
      {stats ? (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
          <StatCard icon={HardDrive} label="Cache Entries" value={stats.cache_entries.toLocaleString()} color="var(--cyan)" onClick={() => onNavigate('cache')} />
          <StatCard icon={Layers} label="Stacks Indexed" value={stats.stacks.toLocaleString()} color="var(--emerald)" onClick={() => onNavigate('inventory')} />
          <StatCard icon={Clock} label="Uptime" value={uptime(stats.uptime_seconds)} color="var(--amber)" />
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
          {[...Array(3)].map((_, i) => <SkeletonCard key={i} />)}
        </div>
      )}

      {/* Provider health */}
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Provider Health</h2>
        <button onClick={() => onNavigate('providers')} className="flex items-center gap-1 text-xs text-slate-500 hover:text-cyan-400 transition-colors">
          View all <ArrowRight size={11} />
        </button>
      </div>

      {providers.length === 0 ? (
        <div className="glass rounded-xl p-8 text-center text-slate-500 text-sm">
          No providers configured yet.
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 mb-8">
          {providers.map(p => <ProviderCard key={p.name} provider={p} />)}
        </div>
      )}

      {/* Quick actions */}
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Quick Actions</h2>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <QuickAction
          icon={RefreshCw}
          title="Rotate a secret"
          description="Invalidate cache and trigger stack redeployment"
          onClick={() => onNavigate('rotate')}
          color="var(--cyan)"
        />
        <QuickAction
          icon={Database}
          title="Manage cache"
          description="View cache stats and flush entries"
          onClick={() => onNavigate('cache')}
          color="var(--violet)"
        />
      </div>
    </div>
  )
}

function StatCard({ icon: Icon, label, value, color, onClick }: {
  icon: React.ElementType; label: string; value: string; color: string; onClick?: () => void
}) {
  return (
    <div
      className={`glass rounded-xl p-5 ${onClick ? 'cursor-pointer hover:bg-white/5 transition-colors' : ''}`}
      onClick={onClick}
    >
      <div className="flex items-center justify-between mb-3">
        <span className="text-slate-500 text-xs font-medium uppercase tracking-wider">{label}</span>
        <Icon size={15} style={{ color }} className="opacity-60" />
      </div>
      <div className="text-3xl font-bold tracking-tight" style={{ color }}>{value}</div>
    </div>
  )
}

function SkeletonCard() {
  return (
    <div className="glass rounded-xl p-5 animate-pulse">
      <div className="h-3 bg-white/8 rounded mb-3 w-2/3" />
      <div className="h-8 bg-white/8 rounded w-1/2" />
    </div>
  )
}

function ProviderCard({ provider: p }: { provider: ProviderStatus }) {
  const neverChecked = !p.checked_at
  const healthy = !neverChecked && p.healthy
  const color = neverChecked ? '#64748b' : p.healthy ? '#34d399' : '#f87171'

  const ago = (iso: string) => {
    if (!iso) return 'never'
    const d = Math.round((Date.now() - new Date(iso).getTime()) / 1000)
    if (d < 60) return `${d}s ago`
    if (d < 3600) return `${Math.floor(d / 60)}m ago`
    return `${Math.floor(d / 3600)}h ago`
  }

  return (
    <div className="glass rounded-xl p-4 space-y-2.5">
      <div className="flex items-center gap-2.5">
        <span
          className="w-2 h-2 rounded-full shrink-0 relative"
          style={{ background: color, boxShadow: neverChecked ? 'none' : `0 0 6px ${color}` }}
        >
          {healthy && (
            <span
              className="absolute inset-0 rounded-full animate-ping opacity-40"
              style={{ background: color }}
            />
          )}
        </span>
        <span className="text-slate-100 font-semibold text-sm truncate">{p.name}</span>
        <span className="ml-auto text-xs text-slate-600">#{p.priority + 1}</span>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-xs px-2 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/20 text-cyan-400">{p.type || 'unknown'}</span>
        <span className="text-xs font-medium" style={{ color }}>{neverChecked ? 'unknown' : p.healthy ? 'healthy' : 'unhealthy'}</span>
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
        <div className="text-red-400 text-xs bg-red-500/10 border border-red-500/20 rounded px-2 py-1.5 break-words">{p.error}</div>
      )}
    </div>
  )
}

function QuickAction({ icon: Icon, title, description, onClick, color }: {
  icon: React.ElementType; title: string; description: string; onClick: () => void; color: string
}) {
  return (
    <button
      onClick={onClick}
      className="glass rounded-xl p-4 text-left hover:bg-white/5 transition-all group flex items-center gap-4"
    >
      <div className="w-9 h-9 rounded-lg flex items-center justify-center shrink-0" style={{ background: `${color}18`, border: `1px solid ${color}30` }}>
        <Icon size={16} style={{ color }} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-slate-200 font-medium text-sm">{title}</div>
        <div className="text-slate-500 text-xs mt-0.5">{description}</div>
      </div>
      <ArrowRight size={14} className="text-slate-600 group-hover:text-slate-400 transition-colors shrink-0" />
    </button>
  )
}

