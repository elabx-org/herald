import { createContext, useCallback, useContext, useRef, useState } from 'react'
import * as T from '@radix-ui/react-toast'
import { CheckCircle2, XCircle, Info, X } from 'lucide-react'

type Kind = 'success' | 'error' | 'info'

interface Msg { id: string; kind: Kind; title: string; description?: string }
interface Ctx { toast: (opts: { kind?: Kind; title: string; description?: string }) => void }

const ToastCtx = createContext<Ctx | null>(null)

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [msgs, setMsgs] = useState<Msg[]>([])
  const counter = useRef(0)

  const toast = useCallback(({ kind = 'info', title, description }: { kind?: Kind; title: string; description?: string }) => {
    const id = String(++counter.current)
    setMsgs(prev => [...prev, { id, kind, title, description }])
    setTimeout(() => setMsgs(prev => prev.filter(m => m.id !== id)), 4500)
  }, [])

  const remove = (id: string) => setMsgs(prev => prev.filter(m => m.id !== id))

  const icons = { success: CheckCircle2, error: XCircle, info: Info }
  const colors = { success: '#34d399', error: '#f87171', info: '#22d3ee' }

  return (
    <ToastCtx.Provider value={{ toast }}>
      <T.Provider swipeDirection="right">
        {children}
        {msgs.map(m => {
          const Icon = icons[m.kind]
          return (
            <T.Root
              key={m.id}
              open
              onOpenChange={open => { if (!open) remove(m.id) }}
              className="glass rounded-xl p-4 flex items-start gap-3 w-80 shadow-2xl data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:slide-out-to-right-full data-[state=open]:slide-in-from-right-full"
              style={{ border: `1px solid ${colors[m.kind]}22` }}
            >
              <Icon size={17} style={{ color: colors[m.kind] }} className="shrink-0 mt-0.5" />
              <div className="flex-1 min-w-0">
                <T.Title className="text-slate-100 text-sm font-medium leading-snug">{m.title}</T.Title>
                {m.description && (
                  <T.Description className="text-slate-400 text-xs mt-0.5 leading-relaxed">{m.description}</T.Description>
                )}
              </div>
              <T.Close onClick={() => remove(m.id)} className="text-slate-600 hover:text-slate-300 transition-colors shrink-0 -mt-0.5">
                <X size={14} />
              </T.Close>
            </T.Root>
          )
        })}
        <T.Viewport className="fixed bottom-5 right-5 flex flex-col gap-2.5 z-[9999] max-w-xs" />
      </T.Provider>
    </ToastCtx.Provider>
  )
}

export function useToast() {
  const ctx = useContext(ToastCtx)
  if (!ctx) throw new Error('useToast outside ToastProvider')
  return ctx.toast
}
