import { useState } from 'react'
import Login from './pages/Login'
import Sidebar from './components/Sidebar'
import DashboardPage from './pages/Dashboard'
import StacksPage from './pages/Stacks'
import RotatePage from './pages/Rotate'
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
        {page === 'stacks' && <StacksPage />}
        {page === 'rotate' && <RotatePage />}
        {page === 'cache' && <CachePage />}
      </main>
    </div>
  )
}
