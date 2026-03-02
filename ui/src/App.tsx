import { useState } from 'react'
import { Menu } from 'lucide-react'
import { AnimatePresence, motion } from 'framer-motion'
import { ToastProvider } from './components/Toast'
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
  const [mobileOpen, setMobileOpen] = useState(false)

  if (!authed) {
    return (
      <ToastProvider>
        <Login onLogin={() => setAuthed(true)} />
      </ToastProvider>
    )
  }

  return (
    <ToastProvider>
      <div className="flex h-screen overflow-hidden" style={{ background: 'var(--bg)' }}>
        <Sidebar
          active={page}
          onNavigate={setPage}
          mobileOpen={mobileOpen}
          onMobileClose={() => setMobileOpen(false)}
        />

        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          {/* Mobile top bar */}
          <div className="lg:hidden flex items-center gap-3 px-4 py-3 shrink-0" style={{ borderBottom: '1px solid var(--glass-border)' }}>
            <button
              onClick={() => setMobileOpen(true)}
              className="text-slate-400 hover:text-slate-200 transition-colors"
            >
              <Menu size={20} />
            </button>
            <span className="font-bold gradient-text">Herald</span>
          </div>

          <main className="flex-1 overflow-auto p-6 lg:p-8">
            <AnimatePresence mode="wait" initial={false}>
              <motion.div
                key={page}
                initial={{ opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -6 }}
                transition={{ duration: 0.12, ease: 'easeOut' }}
              >
                {page === 'dashboard' && <DashboardPage onNavigate={setPage} />}
                {page === 'providers' && <ProvidersPage />}
                {page === 'inventory' && <InventoryPage />}
                {page === 'rotate' && <RotatePage />}
                {page === 'audit' && <AuditPage />}
                {page === 'cache' && <CachePage />}
              </motion.div>
            </AnimatePresence>
          </main>
        </div>
      </div>
    </ToastProvider>
  )
}
