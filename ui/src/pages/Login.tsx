import { useEffect, useRef, useState } from 'react'

function ParticleCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    let animId: number
    const particles: { x: number; y: number; vx: number; vy: number; r: number; a: number }[] = []

    const resize = () => {
      canvas.width = canvas.offsetWidth
      canvas.height = canvas.offsetHeight
    }
    resize()
    window.addEventListener('resize', resize)

    for (let i = 0; i < 80; i++) {
      particles.push({
        x: Math.random() * canvas.width,
        y: Math.random() * canvas.height,
        vx: (Math.random() - 0.5) * 0.4,
        vy: (Math.random() - 0.5) * 0.4,
        r: Math.random() * 1.5 + 0.5,
        a: Math.random(),
      })
    }

    const draw = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height)
      for (const p of particles) {
        p.x += p.vx
        p.y += p.vy
        if (p.x < 0) p.x = canvas.width
        if (p.x > canvas.width) p.x = 0
        if (p.y < 0) p.y = canvas.height
        if (p.y > canvas.height) p.y = 0

        ctx.beginPath()
        ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2)
        ctx.fillStyle = `rgba(34, 211, 238, ${p.a * 0.6})`
        ctx.fill()
      }

      for (let i = 0; i < particles.length; i++) {
        for (let j = i + 1; j < particles.length; j++) {
          const dx = particles[i].x - particles[j].x
          const dy = particles[i].y - particles[j].y
          const d = Math.sqrt(dx * dx + dy * dy)
          if (d < 100) {
            ctx.beginPath()
            ctx.moveTo(particles[i].x, particles[i].y)
            ctx.lineTo(particles[j].x, particles[j].y)
            ctx.strokeStyle = `rgba(129, 140, 248, ${(1 - d / 100) * 0.3})`
            ctx.lineWidth = 0.5
            ctx.stroke()
          }
        }
      }
      animId = requestAnimationFrame(draw)
    }
    draw()

    return () => {
      cancelAnimationFrame(animId)
      window.removeEventListener('resize', resize)
    }
  }, [])

  return <canvas ref={canvasRef} className="absolute inset-0 w-full h-full" />
}

export default function Login({ onLogin }: { onLogin: () => void }) {
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await fetch('/v2/health', {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      })
      if (res.ok) {
        sessionStorage.setItem('herald_token', token)
        onLogin()
      } else {
        setError('Invalid API token')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen">
      {/* Left: particle canvas with wordmark */}
      <div className="relative flex-1 hidden lg:flex flex-col overflow-hidden" style={{ background: '#060b14' }}>
        <ParticleCanvas />
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-4 z-10 pointer-events-none">
          <div className="text-6xl font-bold gradient-text tracking-tight">Herald</div>
          <div className="text-slate-400 text-lg">Secret management for your stacks</div>
          <div className="mt-4 flex flex-col gap-2 text-slate-600 text-sm text-center">
            <span>Resolve <code className="text-cyan-700 bg-white/5 px-1.5 py-0.5 rounded">op://</code> references at deploy time</span>
            <span>Encrypted cache · Provider fallback · Rotation hooks</span>
          </div>
        </div>
      </div>

      {/* Right: glass login card */}
      <div
        className="flex w-full max-w-md items-center justify-center"
        style={{ background: 'linear-gradient(135deg, #060b14 0%, #0d1424 100%)' }}
      >
        <form
          onSubmit={handleSubmit}
          className="glass aurora-glow rounded-2xl p-10 w-full flex flex-col gap-6 mx-8"
        >
          <div className="text-3xl font-bold gradient-text text-center lg:hidden">Herald</div>
          <div className="text-center mb-2">
            <div className="text-slate-300 font-semibold text-lg">Welcome back</div>
            <div className="text-slate-500 text-sm mt-1">Enter your API token to continue</div>
          </div>
          <div>
            <label className="block text-slate-400 text-sm mb-2">API Token</label>
            <input
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="Enter your API token"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-4 py-3 text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-400 transition-colors"
              autoFocus
            />
          </div>
          {error && <p className="text-red-400 text-sm">{error}</p>}
          <button
            type="submit"
            disabled={loading}
            className="w-full py-3 rounded-lg font-semibold text-slate-900 transition-opacity hover:opacity-90 disabled:opacity-50"
            style={{ background: 'linear-gradient(135deg, #22d3ee, #818cf8)' }}
          >
            {loading ? 'Signing in...' : 'Sign In'}
          </button>
          <p className="text-slate-600 text-xs text-center">
            Leave token blank if no authentication is configured.
          </p>
        </form>
      </div>
    </div>
  )
}
