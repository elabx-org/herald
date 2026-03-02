import { useState } from 'react'

const NAV = [
  { id: 'dashboard', label: 'Dashboard', icon: '⬡' },
  { id: 'stacks', label: 'Stacks', icon: '⚙' },
  { id: 'rotate', label: 'Rotate', icon: '↻' },
  { id: 'audit', label: 'Audit', icon: '⊟' },
  { id: 'settings', label: 'Settings', icon: '⚙' },
]

interface SidebarProps {
  active: string
  onNavigate: (id: string) => void
}

export default function Sidebar({ active, onNavigate }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false)

  return (
    <aside
      className="glass flex flex-col transition-all duration-300"
      style={{ width: collapsed ? 64 : 220, minHeight: '100vh', borderRight: '1px solid var(--glass-border)' }}
    >
      <div className="flex items-center justify-between px-4 py-5">
        {!collapsed && <span className="font-bold gradient-text text-lg">Herald</span>}
        <button
          onClick={() => setCollapsed((c) => !c)}
          className="text-slate-400 hover:text-slate-200 transition-colors ml-auto"
          aria-label="Toggle sidebar"
        >
          {collapsed ? '→' : '←'}
        </button>
      </div>

      <nav className="flex-1 flex flex-col gap-1 px-2 mt-2">
        {NAV.map((item) => (
          <button
            key={item.id}
            onClick={() => onNavigate(item.id)}
            className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors w-full text-left
              ${active === item.id
                ? 'text-cyan-300 bg-white/10'
                : 'text-slate-400 hover:text-slate-200 hover:bg-white/5'
              }`}
          >
            <span className="text-lg leading-none">{item.icon}</span>
            {!collapsed && <span>{item.label}</span>}
          </button>
        ))}
      </nav>

      <div className="px-4 py-4">
        <button
          onClick={() => {
            sessionStorage.removeItem('herald_token')
            window.location.hash = '#/'
          }}
          className="flex items-center gap-3 text-slate-500 hover:text-slate-300 text-sm transition-colors w-full"
        >
          <span>⇥</span>
          {!collapsed && <span>Sign out</span>}
        </button>
      </div>
    </aside>
  )
}
