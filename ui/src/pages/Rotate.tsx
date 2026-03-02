import { useState } from 'react'
import { api, type RotateResult } from '../lib/api'

export default function RotatePage() {
  const [item, setItem] = useState('')
  const [vault, setVault] = useState('')
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<RotateResult | null>(null)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!item.trim()) return
    setLoading(true)
    setResult(null)
    setError('')
    try {
      const r = await api.rotate(item.trim(), vault.trim() || undefined)
      setResult(r)
    } catch {
      setError('Rotation request failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-xl">
      <h1 className="text-2xl font-bold gradient-text mb-6">Rotate Secret</h1>
      <p className="text-slate-400 text-sm mb-6">
        Invalidate cached secret values and trigger redeploy of affected stacks.
      </p>

      <form onSubmit={handleSubmit} className="glass rounded-xl p-6 space-y-4">
        <div>
          <label className="block text-slate-400 text-sm mb-1.5">Item name <span className="text-red-400">*</span></label>
          <input
            type="text"
            value={item}
            onChange={e => setItem(e.target.value)}
            placeholder="e.g. database-credentials"
            className="w-full bg-white/5 border border-white/10 rounded-lg px-4 py-2.5 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-cyan-400/50 transition-colors text-sm"
          />
        </div>
        <div>
          <label className="block text-slate-400 text-sm mb-1.5">Vault <span className="text-slate-600">(optional — scopes to specific vault)</span></label>
          <input
            type="text"
            value={vault}
            onChange={e => setVault(e.target.value)}
            placeholder="e.g. HomeLab"
            className="w-full bg-white/5 border border-white/10 rounded-lg px-4 py-2.5 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-cyan-400/50 transition-colors text-sm"
          />
        </div>
        <button
          type="submit"
          disabled={loading || !item.trim()}
          className="w-full py-2.5 rounded-lg font-semibold text-slate-900 transition-opacity hover:opacity-90 disabled:opacity-40 text-sm"
          style={{ background: 'linear-gradient(135deg, #22d3ee, #818cf8)' }}
        >
          {loading ? 'Rotating…' : 'Rotate'}
        </button>
      </form>

      {error && <p className="text-red-400 text-sm mt-4">{error}</p>}

      {result && (
        <div className="glass rounded-xl p-5 mt-4 space-y-3 text-sm">
          <div className="flex items-center gap-2 text-emerald-400 font-medium">
            ✓ Rotation complete
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <div className="text-slate-500 text-xs mb-1">Cache invalidated</div>
              <div className="text-slate-200">{result.cache_invalidated} entries</div>
            </div>
            <div>
              <div className="text-slate-500 text-xs mb-1">Stacks redeployed</div>
              <div className="text-slate-200">{result.stacks_redeployed?.length ?? 0}</div>
            </div>
          </div>
          {result.stacks_redeployed?.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {result.stacks_redeployed.map(s => (
                <span key={s} className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">{s}</span>
              ))}
            </div>
          )}
          {result.errors?.length > 0 && (
            <div className="space-y-1">
              {result.errors.map((e, i) => (
                <div key={i} className="text-red-400 text-xs">{e}</div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
