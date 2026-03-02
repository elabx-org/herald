import { useState } from 'react'
import Login from './pages/Login'
import Sidebar from './components/Sidebar'
import DashboardPage from './pages/Dashboard'
import ProvidersPage from './pages/Providers'
import InventoryPage from './pages/Inventory'
import RotatePage from './pages/Rotate'
import AuditPage from './pages/Audit'
import CachePage from './pages/Cache'

function isAuthenticated() {
  return !!sessionStorage.getItem('herald_token')
}

export default function App() {
  const [authed, setAuthed] = useState(isAuthenticated)
  const [page, setPage] = useState('dashboard')

  if (!authed) {
    return <Login onLogin={() => setAuthed(true)} />
  }

  return (
    <div className="flex h-screen" style={{ background: 'var(--bg)' }}>
      <Sidebar active={page} onNavigate={setPage} />
      <main className="flex-1 overflow-auto p-8">
        {page === 'dashboard' && <DashboardPage />}
        {page === 'providers' && <ProvidersPage />}
        {page === 'inventory' && <InventoryPage />}
        {page === 'rotate' && <RotatePage />}
        {page === 'audit' && <AuditPage />}
        {page === 'cache' && <CachePage />}
      </main>
    </div>
  )
}
