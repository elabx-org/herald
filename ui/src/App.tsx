import Login from './pages/Login'
import Dashboard from './pages/Dashboard'

function getPage() {
  const hash = window.location.hash
  if (hash.startsWith('#/dashboard')) return 'dashboard'
  return 'login'
}

export default function App() {
  const page = getPage()
  const token = sessionStorage.getItem('herald_token')

  if (page === 'dashboard' && token) {
    return <Dashboard />
  }
  return <Login />
}
