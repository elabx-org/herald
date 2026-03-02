import { useState } from 'react'
import { api } from '../lib/api'

export default function CachePage() {
  const [loading, setLoading] = useState(false)
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')
  const [confirm, setConfirm] = useState(false)

  const flush = async () => {
    setLoading(true)
    setMsg('')
    setError('')
    setConfirm(false)
    try {
      await api.cacheFlush()
      setMsg('Cache flushed successfully.')
    } catch {
      setError('Failed to flush cache.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-xl">
      <h1 className="text-2xl font-bold gradient-text mb-6">Cache</h1>
      <p className="text-slate-400 text-sm mb-6">
        Flush all cached secret values. The next materialize request will re-fetch from providers.
      </p>

      <div className="glass rounded-xl p-6 space-y-4">
        {!confirm ? (
          <button
            onClick={() => setConfirm(true)}
            className="w-full py-2.5 rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors text-sm font-medium"
          >
            Flush entire cache
          </button>
        ) : (
          <div className="space-y-3">
            <p className="text-amber-400 text-sm">This will clear all cached secrets. Are you sure?</p>
            <div className="flex gap-3">
              <button
                onClick={flush}
                disabled={loading}
                className="flex-1 py-2.5 rounded-lg bg-red-500/20 border border-red-500/30 text-red-400 hover:bg-red-500/30 transition-colors text-sm font-medium disabled:opacity-50"
              >
                {loading ? 'Flushing…' : 'Yes, flush all'}
              </button>
              <button
                onClick={() => setConfirm(false)}
                className="flex-1 py-2.5 rounded-lg border border-white/10 text-slate-400 hover:text-slate-200 transition-colors text-sm"
              >
                Cancel
              </button>
            </div>
          </div>
        )}
      </div>

      {msg && <p className="text-emerald-400 text-sm mt-4">✓ {msg}</p>}
      {error && <p className="text-red-400 text-sm mt-4">{error}</p>}
    </div>
  )
}
