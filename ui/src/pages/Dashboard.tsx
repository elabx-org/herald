import { useEffect, useState } from 'react'
import { api, type Stats } from '../lib/api'

export default function DashboardPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [error, setError] = useState('')

  const load = () => {
    api.stats().then(setStats).catch(() => setError('Failed to load stats'))
  }

  useEffect(() => { load() }, [])

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
      {stats ? (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <StatCard label="Cache Entries" value={fmt(stats.cache_entries)} color="var(--cyan)" />
          <StatCard label="Stacks Indexed" value={fmt(stats.stacks)} color="var(--emerald)" />
          <StatCard label="Providers" value={fmt(stats.providers?.length ?? 0)} color="var(--violet)" />
          <StatCard label="Uptime" value={uptime(stats.uptime_seconds)} color="var(--amber)" />
        </div>
      ) : (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          {[...Array(4)].map((_, i) => <SkeletonCard key={i} />)}
        </div>
      )}

      {stats?.providers && stats.providers.length > 0 && (
        <div className="mt-8">
          <h2 className="text-slate-400 text-sm font-medium uppercase tracking-wider mb-3">Providers</h2>
          <div className="flex flex-wrap gap-2">
            {stats.providers.map(p => (
              <span key={p} className="glass rounded-full px-3 py-1 text-sm text-emerald-300">
                ● {p}
              </span>
            ))}
          </div>
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
