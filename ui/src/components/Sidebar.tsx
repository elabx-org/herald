import { useState } from 'react'

const NAV = [
  { id: 'dashboard', label: 'Dashboard', icon: '◈' },
  { id: 'stacks', label: 'Stacks', icon: '⬡' },
  { id: 'rotate', label: 'Rotate', icon: '↻' },
  { id: 'cache', label: 'Cache', icon: '◻' },
]

interface SidebarProps {
  active: string
  onNavigate: (id: string) => void
}

export default function Sidebar({ active, onNavigate }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false)

  return (
    <aside
      className="glass flex flex-col transition-all duration-200 shrink-0"
      style={{ width: collapsed ? 56 : 200, minHeight: '100vh', borderRight: '1px solid var(--glass-border)' }}
    >
      <div className="flex items-center justify-between px-3 py-4">
        {!collapsed && <span className="font-bold gradient-text">Herald</span>}
        <button
          onClick={() => setCollapsed(c => !c)}
          className="text-slate-500 hover:text-slate-300 transition-colors ml-auto text-xs"
          title={collapsed ? 'Expand' : 'Collapse'}
        >
          {collapsed ? '›' : '‹'}
        </button>
      </div>

      <nav className="flex-1 flex flex-col gap-0.5 px-2">
        {NAV.map(item => (
          <button
            key={item.id}
            onClick={() => onNavigate(item.id)}
            title={collapsed ? item.label : undefined}
            className={`flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm transition-colors w-full text-left
              ${active === item.id
                ? 'bg-white/10 text-cyan-300'
                : 'text-slate-400 hover:text-slate-200 hover:bg-white/5'
              }`}
          >
            <span className="text-base leading-none shrink-0">{item.icon}</span>
            {!collapsed && <span>{item.label}</span>}
          </button>
        ))}
      </nav>

      <div className="px-2 pb-4">
        <button
          onClick={() => {
            sessionStorage.removeItem('herald_token')
            window.location.hash = '#/'
            window.location.reload()
          }}
          title={collapsed ? 'Sign out' : undefined}
          className="flex items-center gap-2.5 px-2.5 py-2 text-slate-500 hover:text-slate-300 text-sm transition-colors w-full rounded-lg hover:bg-white/5"
        >
          <span className="text-base leading-none shrink-0">⇥</span>
          {!collapsed && <span>Sign out</span>}
        </button>
      </div>
    </aside>
  )
}
