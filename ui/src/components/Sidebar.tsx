import { useState } from 'react'
import {
  LayoutDashboard, Shield, Package, RefreshCw,
  ScrollText, Database, LogOut, ChevronLeft, ChevronRight, X, PlusCircle,
} from 'lucide-react'

const NAV = [
  { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { id: 'providers', label: 'Providers', icon: Shield },
  { id: 'inventory', label: 'Inventory', icon: Package },
  { id: 'rotate', label: 'Rotate', icon: RefreshCw },
  { id: 'provision', label: 'Provision', icon: PlusCircle },
  { id: 'audit', label: 'Audit Log', icon: ScrollText },
  { id: 'cache', label: 'Cache', icon: Database },
]

interface SidebarProps {
  active: string
  onNavigate: (id: string) => void
  mobileOpen?: boolean
  onMobileClose?: () => void
}

export default function Sidebar({ active, onNavigate, mobileOpen = false, onMobileClose }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false)

  const content = (
    <aside
      className="glass flex flex-col h-full transition-all duration-200"
      style={{ width: collapsed ? 56 : 220, borderRight: '1px solid var(--glass-border)' }}
    >
      {/* Logo */}
      <div className="flex items-center gap-2.5 px-3 py-4 border-b border-white/5">
        <div
          className="w-7 h-7 rounded-lg shrink-0 flex items-center justify-center font-bold text-slate-900 text-sm"
          style={{ background: 'linear-gradient(135deg, #22d3ee, #818cf8)' }}
        >
          H
        </div>
        {!collapsed && <span className="font-bold gradient-text tracking-tight">Herald</span>}
        <button
          onClick={() => setCollapsed(c => !c)}
          className="hidden lg:flex ml-auto text-slate-600 hover:text-slate-300 transition-colors"
          title={collapsed ? 'Expand' : 'Collapse'}
        >
          {collapsed ? <ChevronRight size={14} /> : <ChevronLeft size={14} />}
        </button>
        <button onClick={onMobileClose} className="ml-auto text-slate-600 hover:text-slate-300 lg:hidden">
          <X size={16} />
        </button>
      </div>

      {/* Nav items */}
      <nav className="flex-1 flex flex-col gap-0.5 px-2 py-3">
        {NAV.map(item => {
          const Icon = item.icon
          const isActive = active === item.id
          return (
            <button
              key={item.id}
              onClick={() => { onNavigate(item.id); onMobileClose?.() }}
              title={collapsed ? item.label : undefined}
              className={`flex items-center gap-3 px-2.5 py-2.5 rounded-lg text-sm transition-all w-full text-left group
                ${isActive
                  ? 'text-cyan-300 bg-cyan-500/10 border border-cyan-500/20'
                  : 'text-slate-400 hover:text-slate-100 hover:bg-white/5 border border-transparent'
                }`}
            >
              <Icon
                size={16}
                className={`shrink-0 transition-colors ${isActive ? 'text-cyan-400' : 'text-slate-500 group-hover:text-slate-300'}`}
              />
              {!collapsed && <span className="truncate font-medium text-[13px]">{item.label}</span>}
            </button>
          )
        })}
      </nav>

      {/* Sign out */}
      <div className="px-2 pb-4 border-t border-white/5 pt-3">
        <button
          onClick={() => { sessionStorage.removeItem('herald_token'); window.location.reload() }}
          title={collapsed ? 'Sign out' : undefined}
          className="flex items-center gap-3 px-2.5 py-2.5 text-slate-600 hover:text-slate-300 text-sm transition-colors w-full rounded-lg hover:bg-white/5 border border-transparent"
        >
          <LogOut size={16} className="shrink-0" />
          {!collapsed && <span className="font-medium text-[13px]">Sign out</span>}
        </button>
      </div>
    </aside>
  )

  return (
    <>
      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 bg-black/60 z-40 lg:hidden backdrop-blur-sm"
          onClick={onMobileClose}
        />
      )}

      {/* Desktop: always visible */}
      <div className="hidden lg:flex h-screen shrink-0">
        {content}
      </div>

      {/* Mobile: slide-in drawer */}
      <div
        className={`fixed inset-y-0 left-0 z-50 lg:hidden transition-transform duration-200 ${mobileOpen ? 'translate-x-0' : '-translate-x-full'}`}
      >
        <div className="h-full" style={{ width: 220 }}>
          {content}
        </div>
      </div>
    </>
  )
}
