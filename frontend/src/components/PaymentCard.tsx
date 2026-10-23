import { motion } from 'motion/react'

export function brandOf(num: string) {
  const n = num.replace(/\D/g, '')
  if (n.startsWith('4')) return 'visa'
  if (/^5[1-5]/.test(n)) return 'mastercard'
  if (n.startsWith('8600')) return 'uzcard'
  if (n.startsWith('9860')) return 'humo'
  return ''
}

const skins: Record<string, string> = {
  visa: 'linear-gradient(135deg, #1a3a8f, #3f6fe0 60%, #7aa2ff)',
  mastercard: 'linear-gradient(135deg, #2a1a12, #c2421f 55%, #ffb36b)',
  uzcard: 'linear-gradient(135deg, #0b4a3a, #13a37a 60%, #7ff0c9)',
  humo: 'linear-gradient(135deg, #3a1a6b, #7b4fd6 55%, #f0a6ff)',
  '': 'linear-gradient(135deg, #2e4a43, #5b7a70 60%, #9fb8ae)',
}

export function formatCardNumber(v: string) {
  return v.replace(/\D/g, '').slice(0, 16).replace(/(.{4})/g, '$1 ').trim()
}

/**
 * A card that shows what is typed, changes skin with the brand, and turns over
 * while the CVC field has focus.
 */
export function PaymentCard({ number, name, exp, cvc, flipped }: { number: string; name: string; exp: string; cvc: string; flipped: boolean }) {
  const brand = brandOf(number)
  const shown = (number.replace(/\D/g, '') + '•'.repeat(16)).slice(0, 16).replace(/(.{4})/g, '$1 ').trim()
  const face: React.CSSProperties = {
    position: 'absolute',
    inset: 0,
    borderRadius: 18,
    padding: 22,
    color: '#fff',
    backfaceVisibility: 'hidden',
    WebkitBackfaceVisibility: 'hidden',
    boxShadow: '0 30px 50px -25px rgba(0,0,0,.55)',
    overflow: 'hidden',
  }
  return (
    <div style={{ perspective: 1200, width: '100%', maxWidth: 380, aspectRatio: '1.586', margin: '0 auto' }}>
      <motion.div style={{ position: 'relative', width: '100%', height: '100%', transformStyle: 'preserve-3d' }} animate={{ rotateY: flipped ? 180 : 0 }} transition={{ type: 'spring', stiffness: 160, damping: 20 }}>
        <motion.div style={{ ...face, background: skins[brand] }} animate={{ background: skins[brand] }}>
          <div style={{ position: 'absolute', width: 260, height: 260, borderRadius: '50%', background: 'rgba(255,255,255,.08)', right: -80, top: -120 }} />
          <div className="spread">
            <div style={{ width: 44, height: 32, borderRadius: 6, background: 'linear-gradient(135deg,#f6d27a,#c79a2c)' }} />
            <motion.span key={brand} initial={{ opacity: 0, y: -8 }} animate={{ opacity: 1, y: 0 }} style={{ fontWeight: 800, fontStyle: 'italic', fontSize: 20, letterSpacing: '.04em', textTransform: 'uppercase' }}>
              {brand || 'card'}
            </motion.span>
          </div>
          <div className="mono" style={{ fontSize: 'clamp(16px, 4.6vw, 22px)', letterSpacing: '.12em', marginTop: 30 }}>
            {shown}
          </div>
          <div className="spread" style={{ marginTop: 18, fontSize: 12, textTransform: 'uppercase', letterSpacing: '.08em' }}>
            <span>{name || 'Card holder'}</span>
            <span className="mono">{exp || 'MM/YY'}</span>
          </div>
        </motion.div>
        <div style={{ ...face, background: skins[brand], transform: 'rotateY(180deg)', padding: 0 }}>
          <div style={{ height: 44, background: '#111', marginTop: 24 }} />
          <div style={{ margin: '18px 22px', background: '#fff', color: '#111', borderRadius: 6, padding: '8px 12px', textAlign: 'right' }} className="mono">
            {cvc.replace(/./g, '•') || 'CVC'}
          </div>
          <p style={{ margin: '0 22px', fontSize: 11, opacity: 0.8 }}>Your card number is checked, then forgotten: we keep only the brand and the last four digits.</p>
        </div>
      </motion.div>
    </div>
  )
}
