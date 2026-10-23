import { motion } from 'motion/react'

// Rough positions of the branch cities on a 100×60 sketch of Uzbekistan.
const cities: Record<string, [number, number]> = {
  Tashkent: [74, 22],
  Samarkand: [58, 38],
  Bukhara: [44, 40],
  Fergana: [88, 30],
}

const outline =
  'M4 18 L20 10 L34 14 L40 6 L52 10 L60 16 L70 14 L78 18 L86 20 L96 24 L92 32 L82 34 L76 30 L70 34 L66 44 L56 52 L46 50 L38 44 L28 40 L18 36 L10 30 Z'

/** Branches light up as the stock-check job progresses. */
export function BranchMap({ progress, branches }: { progress: number; branches?: { branch: string; city: string; available: number }[] }) {
  const list = branches ?? Object.keys(cities).map((c) => ({ branch: c, city: c, available: -1 }))
  return (
    <svg viewBox="0 0 100 60" style={{ width: '100%', height: 'auto' }} role="img" aria-label="Branches checked so far">
      <path d={outline} fill="color-mix(in oklab, var(--mint) 12%, var(--card))" stroke="var(--line-2)" strokeWidth=".6" strokeLinejoin="round" />
      {list.map((b, i) => {
        const [x, y] = cities[b.city] ?? [50, 30]
        const jitter = list.filter((o, j) => j < i && o.city === b.city).length * 3.5
        const lit = progress >= ((i + 1) / list.length) * 100 || b.available >= 0
        return (
          <g key={b.branch} transform={`translate(${x + jitter} ${y + jitter * 0.6})`}>
            {lit && (
              <motion.circle r="2" fill="none" stroke="var(--mint-ink)" strokeWidth=".5" initial={{ r: 1.4, opacity: 0.9 }} animate={{ r: 6, opacity: 0 }} transition={{ repeat: Infinity, duration: 1.6 }} />
            )}
            <motion.circle r="1.6" animate={{ fill: lit ? (b.available === 0 ? 'var(--bad)' : 'var(--mint-ink)') : 'var(--line-2)' }} />
            {b.available >= 0 && (
              <motion.text initial={{ opacity: 0, y: 2 }} animate={{ opacity: 1, y: 0 }} y="-3" textAnchor="middle" fontSize="3" fontWeight="800" fill="var(--ink)">
                {b.available}
              </motion.text>
            )}
          </g>
        )
      })}
    </svg>
  )
}
