import { useEffect, useState } from 'react'
import { Database, Trash2, RefreshCw, HardDrive, AlertTriangle } from 'lucide-react'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'

export default function CachePage() {
  const toast = useToast()
  const [entryCount, setEntryCount] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)
  const [flushing, setFlushing] = useState(false)
  const [confirm, setConfirm] = useState(false)
  const [lastFlushed, setLastFlushed] = useState<Date | null>(null)

  const loadStats = () => {
    setLoading(true)
    api.stats()
      .then(s => setEntryCount(s.cache_entries))
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { loadStats() }, [])

  const flush = async () => {
    setFlushing(true)
    setConfirm(false)
    try {
      await api.cacheFlush()
      setLastFlushed(new Date())
      setEntryCount(0)
      toast({ kind: 'success', title: 'Cache flushed', description: 'All cached secrets have been cleared' })
    } catch {
      toast({ kind: 'error', title: 'Flush failed', description: 'Could not flush the cache. Check server logs.' })
    } finally {
      setFlushing(false)
    }
  }

  return (
    <div className="max-w-xl">
      <div className="mb-7">
        <h1 className="text-2xl font-bold gradient-text">Cache</h1>
        <p className="text-slate-500 text-sm mt-1">
          Manage Herald's encrypted secret cache. Flushing forces re-fetch from providers on next request.
        </p>
      </div>

      {/* Stats card */}
      <div className="glass rounded-xl p-5 mb-4">
        <div className="flex items-center justify-between mb-4">
          <span className="text-slate-400 text-sm font-medium">Cache Status</span>
          <button
            onClick={loadStats}
            disabled={loading}
            className="text-slate-500 hover:text-slate-300 transition-colors"
            title="Refresh"
          >
            <RefreshCw size={13} className={loading ? 'animate-spin' : ''} />
          </button>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="bg-white/3 rounded-lg p-3">
            <div className="flex items-center gap-2 mb-1">
              <HardDrive size={13} className="text-cyan-400" />
              <span className="text-slate-500 text-xs">Entries</span>
            </div>
            {loading ? (
              <div className="h-7 bg-white/8 rounded animate-pulse w-12 mt-1" />
            ) : (
              <div className="text-2xl font-bold text-cyan-400">{entryCount ?? '—'}</div>
            )}
          </div>

          <div className="bg-white/3 rounded-lg p-3">
            <div className="flex items-center gap-2 mb-1">
              <Database size={13} className="text-slate-400" />
              <span className="text-slate-500 text-xs">Last flushed</span>
            </div>
            <div className="text-sm text-slate-300 mt-1">
              {lastFlushed ? lastFlushed.toLocaleTimeString() : 'never this session'}
            </div>
          </div>
        </div>
      </div>

      {/* Flush section */}
      <div className="glass rounded-xl p-5">
        <div className="flex items-center gap-2 mb-1">
          <Trash2 size={15} className="text-red-400" />
          <span className="text-slate-300 font-medium text-sm">Flush entire cache</span>
        </div>
        <p className="text-slate-500 text-xs mb-4 leading-relaxed">
          Clears all {entryCount != null && entryCount > 0 ? `${entryCount} cached` : 'cached'} secret values across all stacks and vaults. Stacks will re-fetch fresh secrets from providers on next materialize.
        </p>

        {!confirm ? (
          <button
            onClick={() => setConfirm(true)}
            disabled={flushing}
            className="flex items-center gap-2 px-4 py-2 rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors text-sm font-medium disabled:opacity-50"
          >
            <Trash2 size={13} />
            Flush all cached secrets
          </button>
        ) : (
          <div className="bg-amber-500/8 border border-amber-500/20 rounded-lg p-4 space-y-3">
            <div className="flex items-start gap-2 text-amber-400 text-sm">
              <AlertTriangle size={15} className="shrink-0 mt-0.5" />
              <span>This will clear all {entryCount != null && entryCount > 0 ? entryCount : ''} cached secrets. Confirm?</span>
            </div>
            <div className="flex gap-2">
              <button
                onClick={flush}
                disabled={flushing}
                className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-red-500/20 border border-red-500/30 text-red-400 hover:bg-red-500/30 transition-colors text-sm font-medium disabled:opacity-50"
              >
                <Trash2 size={13} className={flushing ? 'animate-spin' : ''} />
                {flushing ? 'Flushing…' : 'Yes, flush all'}
              </button>
              <button
                onClick={() => setConfirm(false)}
                className="px-4 py-2 rounded-lg border border-white/10 text-slate-400 hover:text-slate-200 transition-colors text-sm"
              >
                Cancel
              </button>
            </div>
          </div>
        )}
      </div>

      <p className="text-slate-700 text-xs mt-4 leading-relaxed">
        Tip: To flush a single stack's cache, use the Flush button on the Inventory page.
      </p>
    </div>
  )
}
