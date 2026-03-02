import { useEffect, useState } from 'react'
import { Package, RefreshCw, Trash2, Terminal, Box, ChevronDown } from 'lucide-react'
import { api, type StackEntry } from '../lib/api'
import { useToast } from '../components/Toast'

export default function InventoryPage() {
  const toast = useToast()
  const [stacks, setStacks] = useState<StackEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [flushing, setFlushing] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)

  const load = () => {
    setRefreshing(true)
    api.inventory()
      .then(data => setStacks(Array.isArray(data) ? data : []))
      .catch(() => {})
      .finally(() => { setLoading(false); setRefreshing(false) })
  }

  useEffect(() => { load() }, [])

  const flushStack = async (stack: string) => {
    setFlushing(stack)
    try {
      const r = await api.cacheFlushStack(stack)
      toast({ kind: 'success', title: `Cache flushed for "${stack}"`, description: `${r.flushed} entries cleared` })
      load()
    } catch {
      toast({ kind: 'error', title: 'Flush failed', description: 'Could not flush stack cache' })
    } finally {
      setFlushing(null)
    }
  }

  const ago = (iso: string) => {
    const d = Math.round((Date.now() - new Date(iso).getTime()) / 1000)
    if (d < 60) return `${d}s ago`
    if (d < 3600) return `${Math.floor(d / 60)}m ago`
    if (d < 86400) return `${Math.floor(d / 3600)}h ago`
    return `${Math.floor(d / 86400)}d ago`
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <div>
          <h1 className="text-2xl font-bold gradient-text">Inventory</h1>
          <p className="text-slate-500 text-sm mt-0.5">Stacks that have resolved secrets through Herald</p>
        </div>
        <button
          onClick={load}
          disabled={refreshing}
          className="flex items-center gap-1.5 text-sm text-slate-400 hover:text-slate-200 transition-colors disabled:opacity-50"
        >
          <RefreshCw size={13} className={refreshing ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      <div className="mt-6">
        {loading ? (
          <div className="space-y-3">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="glass rounded-xl p-5 animate-pulse">
                <div className="h-4 bg-white/8 rounded w-1/5 mb-3" />
                <div className="h-3 bg-white/8 rounded w-1/2" />
              </div>
            ))}
          </div>
        ) : stacks.length === 0 ? (
          <div className="glass rounded-xl p-12 text-center">
            <Package size={32} className="text-slate-700 mx-auto mb-4" />
            <div className="text-slate-400 font-medium mb-1">No stacks indexed yet</div>
            <p className="text-slate-600 text-sm mb-6 max-w-sm mx-auto">
              Stacks appear here after the first{' '}
              <code className="bg-white/5 px-1 py-0.5 rounded text-cyan-700">herald-agent sync</code>{' '}
              call resolves secrets.
            </p>
            <div className="inline-flex items-center gap-2 bg-white/4 border border-white/10 rounded-lg px-4 py-3">
              <Terminal size={13} className="text-slate-500 shrink-0" />
              <code className="text-cyan-400 text-sm">herald-agent sync --stack myapp --out /run/secrets/.env</code>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {stacks.sort((a, b) => a.stack.localeCompare(b.stack)).map(s => (
              <StackCard
                key={s.stack}
                entry={s}
                flushing={flushing === s.stack}
                onFlush={() => flushStack(s.stack)}
                ago={ago}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function StackCard({ entry: s, flushing, onFlush, ago }: {
  entry: StackEntry
  flushing: boolean
  onFlush: () => void
  ago: (iso: string) => string
}) {
  const [expanded, setExpanded] = useState(false)
  const hasRefs = s.refs && s.refs.length > 0

  return (
    <div className="glass rounded-xl p-5">
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2.5 mb-2 flex-wrap">
            <Box size={14} className="text-cyan-400 shrink-0" />
            <span className="font-semibold text-slate-100">{s.stack}</span>
            <span className="text-xs text-slate-600 border border-white/8 rounded px-1.5 py-0.5">{s.resolve_count} resolves</span>
            <span className="text-xs text-slate-600">{ago(s.last_seen)}</span>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {(s.items || []).map(item => (
              <span
                key={item}
                className="text-xs px-2 py-0.5 rounded-full bg-cyan-500/8 text-cyan-400 border border-cyan-500/15 font-medium"
              >
                {item}
              </span>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          {hasRefs && (
            <button
              onClick={() => setExpanded(v => !v)}
              className="flex items-center gap-1 text-xs text-slate-500 hover:text-slate-300 transition-colors"
            >
              <ChevronDown size={13} className={`transition-transform ${expanded ? 'rotate-180' : ''}`} />
              {expanded ? 'Hide' : 'Details'}
            </button>
          )}
          <button
            onClick={onFlush}
            disabled={flushing}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg border border-white/10 text-slate-500 hover:text-red-400 hover:border-red-500/30 transition-colors disabled:opacity-50"
          >
            <Trash2 size={11} className={flushing ? 'animate-spin' : ''} />
            {flushing ? 'Flushing…' : 'Flush cache'}
          </button>
        </div>
      </div>

      {expanded && hasRefs && (
        <div className="mt-3 pt-3 border-t border-white/6">
          <div className="text-xs text-slate-600 uppercase tracking-wider mb-2">Secret references</div>
          <div className="space-y-1">
            {s.refs!.sort().map(ref => (
              <div key={ref} className="flex items-center gap-2 font-mono text-xs text-slate-400 bg-white/3 rounded px-2.5 py-1.5">
                <span className="text-slate-600 shrink-0">op://</span>
                <span className="text-cyan-300/70 truncate">{ref.replace(/^(?:op|herald):\/\//, '')}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
