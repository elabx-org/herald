import Sidebar from '../components/Sidebar'
import { useState } from 'react'

export default function Dashboard() {
  const [page, setPage] = useState('dashboard')

  return (
    <div className="flex h-screen" style={{ background: 'var(--bg)' }}>
      <Sidebar active={page} onNavigate={setPage} />
      <main className="flex-1 p-8 overflow-auto">
        <h1 className="text-2xl font-bold gradient-text mb-6">
          {page.charAt(0).toUpperCase() + page.slice(1)}
        </h1>
        {page === 'dashboard' && <DashboardContent />}
        {page !== 'dashboard' && (
          <div className="glass rounded-xl p-8 text-slate-400 text-center">
            Coming soon
          </div>
        )}
      </main>
    </div>
  )
}

function DashboardContent() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
      {[
        { label: 'Stacks Indexed', value: '—', color: 'var(--cyan)' },
        { label: 'Cache Entries', value: '—', color: 'var(--emerald)' },
        { label: 'Providers Active', value: '—', color: 'var(--violet)' },
      ].map((card) => (
        <div key={card.label} className="glass rounded-xl p-6">
          <div className="text-slate-400 text-sm mb-2">{card.label}</div>
          <div className="text-3xl font-bold" style={{ color: card.color }}>{card.value}</div>
        </div>
      ))}
    </div>
  )
}
