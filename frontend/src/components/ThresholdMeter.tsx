import { motion } from 'motion/react'
import { money } from '../lib/format'
import { useI18n } from '../lib/i18n'

/**
 * The refund rule as a picture: a bar up to the refundable amount, with the
 * 200 000 UZS line where approval stops being automatic.
 */
export function ThresholdMeter({ amount, max, limit = 200_000 }: { amount: number; max: number; limit?: number }) {
  const { lang } = useI18n()
  const scale = Math.max(max, limit * 1.25, 1)
  const pct = Math.min(100, (amount / scale) * 100)
  const linePct = (limit / scale) * 100
  const auto = amount > 0 && amount <= limit
  return (
    <div className="stack" style={{ '--gap': '10px' } as React.CSSProperties}>
      <div style={{ position: 'relative', height: 46, borderRadius: 14, background: 'var(--bg-2)', overflow: 'hidden' }}>
        <div style={{ position: 'absolute', inset: 0, left: `${(max / scale) * 100}%`, background: 'repeating-linear-gradient(45deg, transparent 0 6px, color-mix(in oklab, var(--ink) 8%, transparent) 6px 12px)' }} />
        <motion.div
          style={{ position: 'absolute', top: 0, bottom: 0, left: 0, borderRadius: 14 }}
          animate={{ width: `${pct}%`, background: auto ? 'linear-gradient(90deg, var(--mint), #7ff0c9)' : 'linear-gradient(90deg, var(--sun), var(--coral))' }}
          transition={{ type: 'spring', stiffness: 160, damping: 22 }}
        />
        <div style={{ position: 'absolute', top: 0, bottom: 0, left: `${linePct}%`, width: 2, background: 'var(--ink)' }} />
        <div className="tiny" style={{ position: 'absolute', left: `calc(${linePct}% + 6px)`, top: 4, fontWeight: 800 }}>
          {money(limit, lang)}
        </div>
      </div>
      <motion.div key={auto ? 'a' : 'b'} initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} className="small" style={{ fontWeight: 700 }}>
        {amount <= 0 ? (
          <span className="muted">Choose an amount.</span>
        ) : amount > max ? (
          <span style={{ color: 'var(--bad)' }}>More than the {money(max, lang)} still refundable.</span>
        ) : auto ? (
          <span style={{ color: 'var(--mint-ink)' }}>⚡ Approved automatically. The money goes back at once.</span>
        ) : (
          <span>🧑‍⚕️ Over the limit: a pharmacist reviews it first. No money moves until then.</span>
        )}
      </motion.div>
    </div>
  )
}
