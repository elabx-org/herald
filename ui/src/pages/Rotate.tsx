import { useEffect, useState } from 'react'
import { RefreshCw, CheckCircle2, AlertCircle, ChevronRight, Key } from 'lucide-react'
import { api, type RotateResult } from '../lib/api'
import { useToast } from '../components/Toast'

export default function RotatePage({ initialItem = '', initialVault = '' }: { initialItem?: string; initialVault?: string }) {
  const toast = useToast()
  const [item, setItem] = useState(initialItem)
  const [vault, setVault] = useState(initialVault)
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<RotateResult | null>(null)
  const [itemSuggestions, setItemSuggestions] = useState<string[]>([])
  const [vaultSuggestions, setVaultSuggestions] = useState<string[]>([])

  // Update form when prefill values change (navigating from Inventory)
  useEffect(() => {
    setItem(initialItem)
    setVault(initialVault)
    setResult(null)
  }, [initialItem, initialVault])

  // Load inventory for autocomplete suggestions
  useEffect(() => {
    api.inventory()
      .then(stacks => {
        const itemSet = new Set<string>()
        const vaultSet = new Set<string>()
        for (const s of (Array.isArray(stacks) ? stacks : [])) {
          for (const ref of (s.refs || [])) {
            const parts = ref.replace(/^(?:op|herald):\/\//, '').split('/')
            if (parts.length >= 2) {
              vaultSet.add(parts[0])
              itemSet.add(parts[1])
            }
          }
          // Also index items without refs
          for (const i of (s.items || [])) itemSet.add(i)
        }
        setItemSuggestions([...itemSet].sort())
        setVaultSuggestions([...vaultSet].sort())
      })
      .catch(() => {})
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!item.trim()) return
    setLoading(true)
    setResult(null)
    try {
      const r = await api.rotate(item.trim(), vault.trim() || undefined)
      setResult(r)
      toast({ kind: 'success', title: 'Rotation complete', description: `${r.cache_invalidated} cache entries invalidated` })
    } catch {
      toast({ kind: 'error', title: 'Rotation failed', description: 'Check the item name and try again' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-xl">
      <div className="mb-7">
        <h1 className="text-2xl font-bold gradient-text">Rotate Secret</h1>
        <p className="text-slate-500 text-sm mt-1">
          Invalidate cached secret values and trigger redeploy of affected stacks.
        </p>
      </div>

      <form onSubmit={handleSubmit} className="glass rounded-xl p-6 space-y-5">
        <div>
          <label className="block text-slate-300 text-sm font-medium mb-2">
            Item name <span className="text-red-400">*</span>
          </label>
          <div className="relative">
            <Key size={14} className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-500" />
            <input
              type="text"
              value={item}
              onChange={e => setItem(e.target.value)}
              placeholder="e.g. database-credentials"
              list="rotate-item-suggestions"
              className="w-full bg-white/5 border border-white/10 rounded-lg pl-9 pr-4 py-2.5 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50 focus:bg-white/7 transition-all text-sm"
            />
            {itemSuggestions.length > 0 && (
              <datalist id="rotate-item-suggestions">
                {itemSuggestions.map(i => <option key={i} value={i} />)}
              </datalist>
            )}
          </div>
        </div>

        <div>
          <label className="block text-slate-400 text-sm mb-2">
            Vault <span className="text-slate-600 font-normal">(optional)</span>
          </label>
          <input
            type="text"
            value={vault}
            onChange={e => setVault(e.target.value)}
            placeholder="e.g. HomeLab"
            list="rotate-vault-suggestions"
            className="w-full bg-white/5 border border-white/10 rounded-lg px-4 py-2.5 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50 focus:bg-white/7 transition-all text-sm"
          />
          {vaultSuggestions.length > 0 && (
            <datalist id="rotate-vault-suggestions">
              {vaultSuggestions.map(v => <option key={v} value={v} />)}
            </datalist>
          )}
          <p className="text-slate-600 text-xs mt-1.5">Scopes rotation to a specific vault. Leave blank to rotate across all vaults.</p>
        </div>

        <button
          type="submit"
          disabled={loading || !item.trim()}
          className="w-full py-2.5 rounded-lg font-semibold text-slate-900 transition-all hover:opacity-90 disabled:opacity-40 text-sm flex items-center justify-center gap-2"
          style={{ background: 'linear-gradient(135deg, #22d3ee, #818cf8)' }}
        >
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          {loading ? 'Rotating…' : 'Rotate Secret'}
        </button>
      </form>

      {result && (
        <div className="glass rounded-xl p-5 mt-4 border border-emerald-500/20">
          <div className="flex items-center gap-2 text-emerald-400 font-semibold mb-4">
            <CheckCircle2 size={16} />
            Rotation complete
          </div>

          <div className="grid grid-cols-2 gap-3 text-sm mb-4">
            <div className="bg-white/3 rounded-lg p-3">
              <div className="text-slate-500 text-xs mb-1">Cache invalidated</div>
              <div className="text-slate-100 font-semibold text-lg">{result.cache_invalidated}</div>
              <div className="text-slate-600 text-xs">entries cleared</div>
            </div>
            <div className="bg-white/3 rounded-lg p-3">
              <div className="text-slate-500 text-xs mb-1">Stacks triggered</div>
              <div className="text-slate-100 font-semibold text-lg">{result.stacks_redeployed?.length ?? 0}</div>
              <div className="text-slate-600 text-xs">redeployments</div>
            </div>
          </div>

          {result.stacks_redeployed?.length > 0 && (
            <div>
              <div className="text-slate-500 text-xs mb-2 font-medium uppercase tracking-wide">Redeployed stacks</div>
              <div className="flex flex-wrap gap-1.5">
                {result.stacks_redeployed.map(s => (
                  <span key={s} className="text-xs px-2.5 py-1 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">
                    {s}
                  </span>
                ))}
              </div>
            </div>
          )}

          {result.errors?.length > 0 && (
            <div className="mt-3 space-y-1.5">
              {result.errors.map((e, i) => (
                <div key={i} className="flex items-start gap-2 text-red-400 text-xs bg-red-500/8 rounded px-3 py-2">
                  <AlertCircle size={12} className="shrink-0 mt-0.5" />
                  {e}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <div className="mt-6 glass rounded-xl p-4 flex items-start gap-3">
        <ChevronRight size={14} className="text-slate-600 shrink-0 mt-0.5" />
        <p className="text-slate-600 text-xs leading-relaxed">
          Rotation clears all cache entries for the specified item, then triggers Komodo stack redeployments for any stacks that reference it. Stacks will re-fetch fresh secrets on next startup.
        </p>
      </div>
    </div>
  )
}
