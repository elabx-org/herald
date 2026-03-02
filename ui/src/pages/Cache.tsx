import { useEffect, useState } from 'react'
import { Database, Trash2, RefreshCw, HardDrive, AlertTriangle, Clock, ChevronDown, ChevronUp } from 'lucide-react'
import { api, type CacheEntry } from '../lib/api'
import { useToast } from '../components/Toast'

export default function CachePage() {
  const toast = useToast()
  const [entryCount, setEntryCount] = useState<number | null>(null)
  const [entries, setEntries] = useState<CacheEntry[]>([])
  const [loadingStats, setLoadingStats] = useState(false)
  const [loadingEntries, setLoadingEntries] = useState(false)
  const [flushing, setFlushing] = useState(false)
  const [confirm, setConfirm] = useState(false)
  const [lastFlushed, setLastFlushed] = useState<Date | null>(null)
  const [showEntries, setShowEntries] = useState(false)
  const [entryLimit, setEntryLimit] = useState(50)

  const loadStats = () => {
    setLoadingStats(true)
    api.stats()
      .then(s => setEntryCount(s.cache_entries))
      .catch(() => {})
      .finally(() => setLoadingStats(false))
  }

  const loadEntries = () => {
    setLoadingEntries(true)
    api.cacheList()
      .then(data => setEntries(Array.isArray(data) ? data : []))
      .catch(() => {})
      .finally(() => setLoadingEntries(false))
  }

  useEffect(() => { loadStats() }, [])

  const toggleEntries = () => {
    if (!showEntries && entries.length === 0) loadEntries()
    setShowEntries(v => !v)
  }

  const flush = async () => {
    setFlushing(true)
    setConfirm(false)
    try {
      await api.cacheFlush()
      setLastFlushed(new Date())
      setEntryCount(0)
      setEntries([])
      toast({ kind: 'success', title: 'Cache flushed', description: 'All cached secrets have been cleared' })
    } catch {
      toast({ kind: 'error', title: 'Flush failed', description: 'Could not flush the cache. Check server logs.' })
    } finally {
      setFlushing(false)
    }
  }

  const ttlLabel = (iso: string, stale: boolean) => {
    if (stale) return 'expired'
    const secs = Math.round((new Date(iso).getTime() - Date.now()) / 1000)
    if (secs <= 0) return 'expired'
    if (secs < 60) return `${secs}s`
    if (secs < 3600) return `${Math.floor(secs / 60)}m`
    return `${Math.floor(secs / 3600)}h`
  }

  // key format: provider/vault/item/field
  const parseKey = (key: string) => {
    const parts = key.split('/')
    if (parts.length === 4) return { provider: parts[0], vault: parts[1], item: parts[2], field: parts[3] }
    return { provider: '', vault: '', item: key, field: '' }
  }

  return (
    <div className="max-w-2xl">
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
          <button onClick={loadStats} disabled={loadingStats} className="text-slate-500 hover:text-slate-300 transition-colors" title="Refresh">
            <RefreshCw size={13} className={loadingStats ? 'animate-spin' : ''} />
          </button>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="bg-white/3 rounded-lg p-3">
            <div className="flex items-center gap-2 mb-1">
              <HardDrive size={13} className="text-cyan-400" />
              <span className="text-slate-500 text-xs">Entries</span>
            </div>
            {loadingStats ? (
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

      {/* Cache entries */}
      {(entryCount ?? 0) > 0 && (
        <div className="glass rounded-xl mb-4 overflow-hidden">
          <button
            onClick={toggleEntries}
            className="w-full flex items-center justify-between px-5 py-3.5 hover:bg-white/3 transition-colors"
          >
            <div className="flex items-center gap-2 text-slate-400 text-sm font-medium">
              <HardDrive size={13} className="text-cyan-400" />
              Cached entries ({entryCount})
            </div>
            {showEntries ? <ChevronUp size={14} className="text-slate-600" /> : <ChevronDown size={14} className="text-slate-600" />}
          </button>

          {showEntries && (
            <div className="border-t border-white/5">
              <div className="flex items-center justify-between px-5 py-2 border-b border-white/5">
                <span className="text-xs text-slate-600">{entries.length} loaded</span>
                <button onClick={loadEntries} disabled={loadingEntries} className="text-slate-600 hover:text-slate-400 transition-colors">
                  <RefreshCw size={11} className={loadingEntries ? 'animate-spin' : ''} />
                </button>
              </div>

              {loadingEntries ? (
                <div className="p-4 space-y-2 animate-pulse">
                  {[...Array(4)].map((_, i) => <div key={i} className="h-3 bg-white/8 rounded" />)}
                </div>
              ) : (
                <>
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b border-white/5 text-slate-600">
                        <th className="text-left px-4 py-2 font-medium">Item</th>
                        <th className="text-left px-4 py-2 font-medium hidden sm:table-cell">Vault</th>
                        <th className="text-left px-4 py-2 font-medium hidden md:table-cell">Field</th>
                        <th className="text-right px-4 py-2 font-medium w-20">TTL</th>
                      </tr>
                    </thead>
                    <tbody>
                      {entries.slice(0, entryLimit).map((e, i) => {
                        const p = parseKey(e.key)
                        return (
                          <tr key={i} className="border-b border-white/4 last:border-0 hover:bg-white/2">
                            <td className="px-4 py-2 text-slate-300 font-medium">{p.item}</td>
                            <td className="px-4 py-2 text-slate-500 hidden sm:table-cell">{p.vault || '—'}</td>
                            <td className="px-4 py-2 text-slate-500 hidden md:table-cell font-mono">{p.field || '—'}</td>
                            <td className="px-4 py-2 text-right">
                              <span className={`font-mono ${e.stale ? 'text-red-400' : 'text-emerald-400'}`}>
                                <Clock size={9} className="inline mr-1 opacity-60" />
                                {ttlLabel(e.expires_at, e.stale)}
                              </span>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                  {entries.length > entryLimit && (
                    <button
                      onClick={() => setEntryLimit(l => l + 50)}
                      className="w-full py-2 text-xs text-slate-600 hover:text-slate-400 transition-colors border-t border-white/5"
                    >
                      Show {Math.min(50, entries.length - entryLimit)} more
                    </button>
                  )}
                </>
              )}
            </div>
          )}
        </div>
      )}

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
              <button onClick={() => setConfirm(false)} className="px-4 py-2 rounded-lg border border-white/10 text-slate-400 hover:text-slate-200 transition-colors text-sm">
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
