import { useEffect, useState } from 'react'
import {
  Shield, RefreshCw, Wifi, WifiOff, Clock, Gauge, AlertTriangle,
  Pencil, Trash2, ChevronDown, ChevronUp, Plus, X
} from 'lucide-react'
import { api, type ProviderStatus } from '../lib/api'
import { useToast } from '../components/Toast'

export default function ProvidersPage() {
  const toast = useToast()
  const [providers, setProviders] = useState<ProviderStatus[]>([])
  const [loading, setLoading] = useState(true)
  const [checking, setChecking] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [showEnvRef, setShowEnvRef] = useState<Record<string, boolean>>({})

  // Slide-in panel state
  const [panelOpen, setPanelOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<ProviderStatus | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form, setForm] = useState({
    name: '',
    type: '1password-connect' as '1password-connect' | '1password-sdk' | 'mock',
    priority: 0,
    url: '',
    token: '',
    tokenFormat: 'plain' as 'plain' | 'base64' | 'json',
  })

  const load = () => {
    setLoading(true)
    api.providers()
      .then(data => setProviders(Array.isArray(data) ? data : []))
      .finally(() => setLoading(false))
  }

  const reload = () => {
    api.providers()
      .then(data => setProviders(Array.isArray(data) ? data : []))
      .catch(() => toast({ kind: 'error', title: 'Refresh failed', description: 'Could not reload provider list' }))
  }

  const checkNow = () => {
    setChecking(true)
    api.providersCheck()
      .then(data => setProviders(Array.isArray(data) ? data : []))
      .catch(() => {})
      .finally(() => setChecking(false))
  }

  useEffect(() => { load() }, [])

  const ago = (iso: string) => {
    if (!iso) return 'never'
    const d = Math.round((Date.now() - new Date(iso).getTime()) / 1000)
    if (d < 60) return `${d}s ago`
    if (d < 3600) return `${Math.floor(d / 60)}m ago`
    return `${Math.floor(d / 3600)}h ago`
  }

  const openEdit = (p: ProviderStatus) => {
    setEditTarget(p)
    setForm({ name: p.name, type: p.type as '1password-connect' | '1password-sdk' | 'mock', priority: p.priority, url: p.url || '', token: '', tokenFormat: 'plain' })
    setPanelOpen(true)
  }

  const openAdd = () => {
    setEditTarget(null)
    setForm({ name: '', type: '1password-connect', priority: 0, url: '', token: '', tokenFormat: 'plain' })
    setPanelOpen(true)
  }

  const handleDelete = async (name: string) => {
    try {
      await api.deleteProvider(name)
      toast({ kind: 'success', title: 'Provider deleted', description: name })
      setConfirmDelete(null)
      setShowEnvRef(prev => {
        const next = { ...prev }
        delete next[name]
        return next
      })
      reload()
    } catch {
      toast({ kind: 'error', title: 'Delete failed', description: 'Could not delete provider' })
    }
  }

  const resolveToken = (raw: string, format: typeof form.tokenFormat): string | null => {
    if (!raw) return ''
    if (format === 'plain') return raw
    if (format === 'base64') {
      try {
        return atob(raw.trim())
      } catch {
        toast({ kind: 'error', title: 'Invalid base64', description: 'Token is not valid base64-encoded data' })
        return null
      }
    }
    // json credentials file
    try {
      const obj = JSON.parse(raw)
      const token = obj.credential ?? obj.token ?? obj.access_token ?? obj.jwt ?? ''
      if (!token) throw new Error('no token field')
      return typeof token === 'string' ? token : JSON.stringify(token)
    } catch {
      toast({ kind: 'error', title: 'Invalid credentials JSON', description: 'Could not find credential/token field in JSON' })
      return null
    }
  }

  const handleProviderSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)

    let resolvedToken: string | undefined
    if (form.token) {
      const decoded = resolveToken(form.token, form.tokenFormat)
      if (decoded === null) { setSubmitting(false); return }
      resolvedToken = decoded || undefined
    }

    try {
      if (editTarget) {
        await api.updateProvider(form.name, {
          type: form.type,
          priority: form.priority,
          url: form.url || undefined,
          token: resolvedToken,
        })
        toast({ kind: 'success', title: 'Provider updated', description: form.name })
      } else {
        await api.createProvider({ name: form.name, type: form.type, priority: form.priority, url: form.url || undefined, token: resolvedToken })
        toast({ kind: 'success', title: 'Provider added', description: form.name })
      }
      setPanelOpen(false)
      reload()
    } catch {
      toast({ kind: 'error', title: editTarget ? 'Update failed' : 'Create failed', description: 'Check configuration and try again' })
    } finally {
      setSubmitting(false)
    }
  }

  const toggleEnvRef = (name: string) => {
    setShowEnvRef(prev => ({ ...prev, [name]: !prev[name] }))
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <div>
          <h1 className="text-2xl font-bold gradient-text">Providers</h1>
          <p className="text-slate-500 text-sm mt-0.5">Health checks run every 60 s in the background</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={openAdd}
            className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-lg bg-gradient-to-r from-cyan-500 to-violet-500 text-white font-medium hover:opacity-90 transition-opacity"
          >
            <Plus size={13} />
            Add Provider
          </button>
          <button
            onClick={checkNow}
            disabled={checking}
            className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-lg border border-white/10 text-slate-400 hover:text-slate-200 hover:border-white/20 transition-colors disabled:opacity-50"
          >
            <RefreshCw size={12} className={checking ? 'animate-spin' : ''} />
            {checking ? 'Checking…' : 'Check now'}
          </button>
          <button onClick={load} className="flex items-center gap-1.5 text-sm text-slate-500 hover:text-slate-300 transition-colors px-2">
            <RefreshCw size={12} />
          </button>
        </div>
      </div>

      <div className="mt-6">
        {loading ? (
          <div className="space-y-3">
            {[...Array(2)].map((_, i) => (
              <div key={i} className="glass rounded-xl p-6 animate-pulse flex gap-4">
                <div className="w-10 h-10 bg-white/8 rounded-lg" />
                <div className="flex-1 space-y-2">
                  <div className="h-4 bg-white/8 rounded w-1/4" />
                  <div className="h-3 bg-white/8 rounded w-1/3" />
                </div>
              </div>
            ))}
          </div>
        ) : providers.length === 0 ? (
          <div className="glass rounded-xl p-14 text-center">
            <Shield size={28} className="text-slate-600 mx-auto mb-3" />
            <div className="text-slate-400 font-medium mb-2">No providers configured</div>
            <p className="text-slate-600 text-sm max-w-sm mx-auto">
              Set <code className="bg-white/5 px-1.5 py-0.5 rounded text-cyan-700">OP_CONNECT_SERVER_URL</code> and{' '}
              <code className="bg-white/5 px-1.5 py-0.5 rounded text-cyan-700">OP_CONNECT_TOKEN</code> to enable secret resolution.
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {providers.sort((a, b) => a.priority - b.priority).map(p => (
              <ProviderRow
                key={p.name}
                provider={p}
                ago={ago}
                showEnvRef={!!showEnvRef[p.name]}
                onToggleEnvRef={() => toggleEnvRef(p.name)}
                onEdit={() => openEdit(p)}
                onDelete={() => setConfirmDelete(p.name)}
              />
            ))}
          </div>
        )}
      </div>

      {/* Confirm Delete Modal */}
      {confirmDelete && (
        <div role="dialog" aria-modal="true" aria-label="Confirm provider deletion" className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="glass rounded-xl p-6 w-full max-w-sm mx-4 border border-white/10">
            <h2 className="text-lg font-semibold text-slate-100 mb-2">Delete Provider</h2>
            <p className="text-slate-400 text-sm mb-6">
              Are you sure you want to delete <span className="text-slate-200 font-medium">{confirmDelete}</span>? This cannot be undone.
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setConfirmDelete(null)}
                className="text-sm px-4 py-2 rounded-lg border border-white/10 text-slate-400 hover:text-slate-200 hover:border-white/20 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(confirmDelete)}
                className="text-sm px-4 py-2 rounded-lg bg-red-500/20 border border-red-500/30 text-red-400 hover:bg-red-500/30 hover:text-red-300 transition-colors"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Slide-in panel overlay */}
      {panelOpen && (
        <div className="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm" onClick={() => setPanelOpen(false)} />
      )}

      {/* Slide-in panel */}
      <div role="dialog" aria-modal="true" aria-label={editTarget ? 'Edit provider' : 'Add provider'} className={`fixed top-0 right-0 h-full z-50 w-full max-w-md bg-[#0f1117] border-l border-white/10 shadow-2xl transform transition-transform duration-300 ease-in-out flex flex-col ${panelOpen ? 'translate-x-0' : 'translate-x-full'}`}>
        <div className="flex items-center justify-between px-6 py-5 border-b border-white/10">
          <h2 className="text-lg font-semibold text-slate-100">{editTarget ? 'Edit Provider' : 'Add Provider'}</h2>
          <button onClick={() => setPanelOpen(false)} className="text-slate-500 hover:text-slate-300 transition-colors">
            <X size={18} />
          </button>
        </div>

        <form onSubmit={handleProviderSubmit} className="flex-1 overflow-y-auto px-6 py-5 space-y-5">
          {/* Name */}
          <div>
            <label className="block text-xs font-medium text-slate-400 mb-1.5">Name</label>
            <input
              type="text"
              required
              readOnly={!!editTarget}
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-slate-200 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50 read-only:opacity-60 read-only:cursor-not-allowed"
              placeholder="my-provider"
            />
          </div>

          {/* Type */}
          <div>
            <label className="block text-xs font-medium text-slate-400 mb-1.5">Type</label>
            <select
              value={form.type}
              onChange={e => setForm(f => ({ ...f, type: e.target.value as typeof f.type, token: '', tokenFormat: 'plain' }))}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-cyan-500/50"
            >
              <option value="1password-connect">1Password Connect</option>
              <option value="1password-sdk">1Password Service Account</option>
              <option value="mock">Mock</option>
            </select>
          </div>

          {/* Priority */}
          <div>
            <label className="block text-xs font-medium text-slate-400 mb-1.5">Priority</label>
            <input
              type="number"
              value={form.priority}
              onChange={e => setForm(f => ({ ...f, priority: Number(e.target.value) }))}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-cyan-500/50"
              min={0}
            />
            <p className="text-xs text-slate-600 mt-1">Lower number = higher priority (tried first)</p>
          </div>

          {/* URL — Connect and Mock only */}
          {(form.type === '1password-connect' || form.type === 'mock') && (
            <div>
              <label className="block text-xs font-medium text-slate-400 mb-1.5">
                {form.type === 'mock' ? 'Mock Path' : 'Connect Server URL'}
              </label>
              <input
                type="text"
                value={form.url}
                onChange={e => setForm(f => ({ ...f, url: e.target.value }))}
                className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-slate-200 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50"
                placeholder={form.type === 'mock' ? '/path/to/secrets.json' : 'https://connect.example.com'}
              />
            </div>
          )}

          {/* Token — Connect and SDK only */}
          {(form.type === '1password-connect' || form.type === '1password-sdk') && (
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="text-xs font-medium text-slate-400">Token</label>
                {/* Format toggle */}
                <div className="flex items-center gap-0.5 bg-white/5 border border-white/10 rounded-lg p-0.5">
                  {(['plain', 'base64', 'json'] as const).map(fmt => (
                    <button
                      key={fmt}
                      type="button"
                      onClick={() => setForm(f => ({ ...f, tokenFormat: fmt, token: '' }))}
                      className={`text-[10px] font-semibold px-2 py-0.5 rounded transition-colors ${
                        form.tokenFormat === fmt
                          ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30'
                          : 'text-slate-500 hover:text-slate-300'
                      }`}
                    >
                      {fmt === 'plain' ? 'Token' : fmt === 'base64' ? 'Base64' : 'JSON'}
                    </button>
                  ))}
                </div>
              </div>
              {form.tokenFormat === 'json' ? (
                <>
                  <textarea
                    value={form.token}
                    onChange={e => setForm(f => ({ ...f, token: e.target.value }))}
                    rows={6}
                    className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-xs text-slate-300 font-mono placeholder-slate-600 focus:outline-none focus:border-cyan-500/50 resize-none"
                    placeholder={'{\n  "credential": "eyJ..."\n}'}
                  />
                  <p className="text-[10px] text-slate-600 mt-1">
                    Paste credentials JSON — extracts <code className="text-slate-500">credential</code>, <code className="text-slate-500">token</code>, or <code className="text-slate-500">access_token</code> field
                  </p>
                </>
              ) : (
                <>
                  <input
                    type="password"
                    value={form.token}
                    onChange={e => setForm(f => ({ ...f, token: e.target.value }))}
                    className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-slate-200 placeholder-slate-600 focus:outline-none focus:border-cyan-500/50"
                    placeholder={editTarget ? 'Leave blank to keep existing token' : form.tokenFormat === 'base64' ? 'Paste base64-encoded token' : 'Paste token'}
                  />
                  {form.tokenFormat === 'base64' && (
                    <p className="text-[10px] text-slate-600 mt-1">Base64-encoded token will be decoded before storage</p>
                  )}
                </>
              )}
            </div>
          )}

          <div className="pt-2">
            <button
              type="submit"
              disabled={submitting}
              className="w-full py-2.5 rounded-lg bg-gradient-to-r from-cyan-500 to-violet-500 text-white text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {submitting ? 'Saving…' : editTarget ? 'Save Changes' : 'Add Provider'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function ProviderRow({
  provider: p,
  ago,
  showEnvRef,
  onToggleEnvRef,
  onEdit,
  onDelete,
}: {
  provider: ProviderStatus
  ago: (s: string) => string
  showEnvRef: boolean
  onToggleEnvRef: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const neverChecked = !p.checked_at
  const color = neverChecked ? '#64748b' : p.healthy ? '#34d399' : '#f87171'
  const StatusIcon = neverChecked ? Clock : p.healthy ? Wifi : WifiOff

  const envVarLines: string[] = []
  if (p.type === '1password-connect') {
    envVarLines.push(`OP_CONNECT_SERVER_URL=${p.url || '<url>'}`)
    envVarLines.push(`OP_CONNECT_TOKEN=<token>`)
  } else if (p.type === '1password-sdk') {
    envVarLines.push(`OP_SERVICE_ACCOUNT_TOKEN=<token>`)
  } else if (p.type === 'mock') {
    envVarLines.push(`HERALD_MOCK_PATH=${p.url || '<path>'}`)
  }

  return (
    <div className="glass rounded-xl p-5">
      <div className="flex items-start gap-4">
        {/* Status icon */}
        <div
          className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0"
          style={{ background: `${color}15`, border: `1px solid ${color}30` }}
        >
          <StatusIcon size={18} style={{ color }} />
        </div>

        {/* Info */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2.5 flex-wrap">
            <span className="text-slate-100 font-semibold">{p.name}</span>
            {/* Source badge */}
            <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded uppercase tracking-wider ${
              p.source === 'db'
                ? 'bg-violet-500/15 text-violet-400 border border-violet-500/25'
                : 'bg-slate-500/15 text-slate-400 border border-slate-500/25'
            }`}>
              {p.source === 'db' ? 'DB' : 'ENV'}
            </span>
            <span className="text-xs px-2 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/20 text-cyan-400">{p.type || 'unknown'}</span>
            <span className="text-xs px-2 py-0.5 rounded bg-white/5 border border-white/10 text-slate-400">Priority #{p.priority + 1}</span>
            <span className="text-xs font-semibold" style={{ color }}>
              {neverChecked ? 'not yet checked' : p.healthy ? 'healthy' : 'unhealthy'}
            </span>
          </div>

          <div className="mt-3 flex items-center gap-6 text-sm">
            <div className="flex items-center gap-1.5 text-slate-500">
              <Gauge size={13} />
              <span className="text-slate-400">{p.latency_ms > 0 ? `${p.latency_ms} ms` : '—'}</span>
            </div>
            <div className="flex items-center gap-1.5 text-slate-500">
              <Clock size={13} />
              <span className="text-slate-400">Checked {ago(p.checked_at)}</span>
            </div>
          </div>
        </div>

        {/* Action buttons + status dot */}
        <div className="flex items-center gap-2 shrink-0">
          {/* Env ref toggle */}
          <button
            onClick={onToggleEnvRef}
            className="text-slate-600 hover:text-slate-300 transition-colors"
            title={showEnvRef ? 'Hide env var reference' : 'Show env var reference'}
            aria-label={`${showEnvRef ? 'Hide' : 'Show'} environment variables for ${p.name}`}
          >
            {showEnvRef ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
          </button>
          {/* Edit button */}
          <button onClick={onEdit} className="text-slate-600 hover:text-slate-300 transition-colors" title="Edit" aria-label={`Edit provider ${p.name}`}>
            <Pencil size={13} />
          </button>
          {/* Delete button — db only */}
          {p.source === 'db' && (
            <button onClick={onDelete} className="text-slate-600 hover:text-red-400 transition-colors" title="Delete" aria-label={`Delete provider ${p.name}`}>
              <Trash2 size={13} />
            </button>
          )}
          {/* Status dot */}
          <span className="relative flex h-2.5 w-2.5 ml-1">
            <span
              className="absolute inline-flex h-full w-full rounded-full opacity-50"
              style={{ background: color, animation: p.healthy ? 'ping 1.5s cubic-bezier(0, 0, 0.2, 1) infinite' : 'none' }}
            />
            <span className="relative inline-flex rounded-full h-2.5 w-2.5" style={{ background: color }} />
          </span>
        </div>
      </div>

      {p.error && (
        <div className="mt-3 flex items-start gap-2 text-red-400 text-sm bg-red-500/8 border border-red-500/20 rounded-lg px-3 py-2.5">
          <AlertTriangle size={14} className="shrink-0 mt-0.5" />
          {p.error}
        </div>
      )}

      {/* Env var reference section */}
      {showEnvRef && envVarLines.length > 0 && (
        <div className="mt-3 bg-white/3 border border-white/8 rounded-lg px-3 py-2.5">
          <div className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider mb-1.5">Env var reference</div>
          {envVarLines.map(line => (
            <code key={line} className="block text-xs text-slate-400 font-mono">{line}</code>
          ))}
        </div>
      )}
    </div>
  )
}
