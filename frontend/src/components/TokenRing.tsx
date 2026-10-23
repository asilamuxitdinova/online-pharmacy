import { motion } from 'motion/react'
import { useEffect, useState } from 'react'

/** A ring that empties as the access token ages. Red in its last ten seconds. */
export function TokenRing({ expiresAt, lifetime, size = 168 }: { expiresAt: number; lifetime: number; size?: number }) {
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 250)
    return () => window.clearInterval(id)
  }, [])
  const left = Math.max(0, expiresAt - now)
  const frac = lifetime > 0 ? Math.min(1, left / lifetime) : 0
  const r = 44
  const c = 2 * Math.PI * r
  const urgent = left < 10_000
  const secs = Math.ceil(left / 1000)
  const label = secs >= 120 ? `${Math.floor(secs / 60)}m ${secs % 60}s` : `${secs}s`
  return (
    <div style={{ position: 'relative', width: size, height: size }}>
      <svg viewBox="0 0 100 100" width={size} height={size} style={{ transform: 'rotate(-90deg)' }}>
        <circle cx="50" cy="50" r={r} fill="none" stroke="var(--bg-2)" strokeWidth="8" />
        <motion.circle
          cx="50"
          cy="50"
          r={r}
          fill="none"
          strokeWidth="8"
          strokeLinecap="round"
          strokeDasharray={c}
          animate={{ strokeDashoffset: c * (1 - frac), stroke: urgent ? 'var(--coral)' : 'var(--mint)' }}
          transition={{ duration: 0.25, ease: 'linear' }}
        />
      </svg>
      <div style={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', textAlign: 'center' }}>
        <div>
          <motion.div key={urgent ? 'u' : 'n'} animate={urgent ? { scale: [1, 1.08, 1] } : {}} transition={{ repeat: Infinity, duration: 1 }} className="display" style={{ fontSize: size / 5 }}>
            {left > 0 ? label : 'expired'}
          </motion.div>
          <div className="tiny muted">access token</div>
        </div>
      </div>
    </div>
  )
}
