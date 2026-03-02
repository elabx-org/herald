import { useState } from 'react'
import { PlusCircle, Eye, EyeOff, CheckCircle2 } from 'lucide-react'
import { api } from '../lib/api'
import { useToast } from '../components/Toast'

export default function ProvisionPage() {
  const toast = useToast()
  const [vault, setVault] = useState('')
  const [item, setItem] = useState('')
  const [field, setField] = useState('')
  const [value, setValue] = useState('')
  const [showValue, setShowValue] = useState(false)
  const [loading, setLoading] = useState(false)
  const [lastCreated, setLastCreated] = useState<{ vault: string; item: string; field: string } | null>(null)

  const canSubmit = vault.trim() && item.trim() && field.trim() && value.trim()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!canSubmit) return
    setLoading(true)
    try {
      await api.provision(vault.trim(), item.trim(), field.trim(), value.trim())
      setLastCreated({ vault: vault.trim(), item: item.trim(), field: field.trim() })
      toast({ kind: 'success', title: 'Secret provisioned', description: `${item}/${field} created in ${vault}` })
      setValue('')
    } catch {
      toast({ kind: 'error', title: 'Provision failed', description: 'Could not create the secret. Check vault and item names.' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="max-w-xl">
      <div className="mb-7">
        <h1 className="text-2xl font-bold gradient-text">Provision Secret</h1>
        <p className="text-slate-500 text-sm mt-1">
          Create a new secret field in 1Password via Herald.
        </p>
      </div>

      <form onSubmit={handleSubmit} className="glass rounded-xl p-6 space-y-5">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-slate-300 text-sm font-medium mb-2">
              Vault <span className="text-red-400">*</span>
            </label>
            <input
              type="text"
              value={vault}
              onChange={e => setVault(e.target.value)}
              placeholder="e.g. HomeLab"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-4 py-2.5 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50 focus:bg-white/7 transition-all text-sm"
            />
          </div>
          <div>
            <label className="block text-slate-300 text-sm font-medium mb-2">
              Item name <span className="text-red-400">*</span>
            </label>
            <input
              type="text"
              value={item}
              onChange={e => setItem(e.target.value)}
              placeholder="e.g. my-service"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-4 py-2.5 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50 focus:bg-white/7 transition-all text-sm"
            />
          </div>
        </div>

        <div>
          <label className="block text-slate-300 text-sm font-medium mb-2">
            Field name <span className="text-red-400">*</span>
          </label>
          <input
            type="text"
            value={field}
            onChange={e => setField(e.target.value)}
            placeholder="e.g. password, jwt_secret, api_key"
            className="w-full bg-white/5 border border-white/10 rounded-lg px-4 py-2.5 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50 focus:bg-white/7 transition-all text-sm"
          />
        </div>

        <div>
          <label className="block text-slate-300 text-sm font-medium mb-2">
            Value <span className="text-red-400">*</span>
          </label>
          <div className="relative">
            <input
              type={showValue ? 'text' : 'password'}
              value={value}
              onChange={e => setValue(e.target.value)}
              placeholder="Secret value"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-4 pr-10 py-2.5 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50 focus:bg-white/7 transition-all text-sm font-mono"
            />
            <button
              type="button"
              onClick={() => setShowValue(v => !v)}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-300 transition-colors"
            >
              {showValue ? <EyeOff size={14} /> : <Eye size={14} />}
            </button>
          </div>
        </div>

        <button
          type="submit"
          disabled={loading || !canSubmit}
          className="w-full py-2.5 rounded-lg font-semibold text-slate-900 transition-all hover:opacity-90 disabled:opacity-40 text-sm flex items-center justify-center gap-2"
          style={{ background: 'linear-gradient(135deg, #22d3ee, #818cf8)' }}
        >
          <PlusCircle size={14} />
          {loading ? 'Provisioning…' : 'Create Secret'}
        </button>
      </form>

      {lastCreated && (
        <div className="glass rounded-xl p-4 mt-4 border border-emerald-500/20 flex items-start gap-3 text-emerald-400">
          <CheckCircle2 size={16} className="shrink-0 mt-0.5" />
          <div>
            <div className="font-medium text-sm">Secret created</div>
            <code className="text-xs text-emerald-400/70 mt-0.5 font-mono">
              op://{lastCreated.vault}/{lastCreated.item}/{lastCreated.field}
            </code>
          </div>
        </div>
      )}

      <div className="mt-6 glass rounded-xl p-4 space-y-2">
        <div className="text-xs text-slate-500 font-medium uppercase tracking-wider mb-1">CLI equivalent</div>
        <code className="block text-xs text-cyan-400/80 font-mono bg-white/3 rounded px-3 py-2 leading-relaxed break-all">
          herald-agent provision --vault {vault || 'MyVault'} --item {item || 'my-item'} --field {field || 'password'} --value ***
        </code>
      </div>
    </div>
  )
}
