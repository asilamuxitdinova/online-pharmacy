import { AnimatePresence, motion } from 'motion/react'
import { useNavigate } from 'react-router'
import { useCart } from '../lib/cart'
import { money, pillColors } from '../lib/format'
import { useI18n } from '../lib/i18n'
import { PillIcon } from './PillIcon'

export function CartDrawer() {
  const cart = useCart()
  const { t, lang } = useI18n()
  const nav = useNavigate()
  return (
    <AnimatePresence>
      {cart.open && (
        <>
          <motion.div className="scrim" onClick={() => cart.setOpen(false)} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} />
          <motion.aside
            className="drawer"
            role="dialog"
            aria-label={t('cart.title')}
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 320, damping: 34 }}
          >
            <div className="drawer-head">
              <h3>{t('cart.title')}</h3>
              <button className="btn ghost sm icon" onClick={() => cart.setOpen(false)} aria-label={t('common.close')}>
                ✕
              </button>
            </div>
            <div className="drawer-body stack" style={{ '--gap': '10px' } as React.CSSProperties}>
              {cart.lines.length === 0 && (
                <div className="center stack muted" style={{ alignItems: 'center', paddingTop: 60 }}>
                  <motion.div animate={{ rotate: [0, -12, 12, 0] }} transition={{ repeat: Infinity, duration: 3 }}>
                    <PillIcon form="capsules" a="var(--coral)" b="var(--mint)" size={90} />
                  </motion.div>
                  <p>{t('cart.empty')}</p>
                </div>
              )}
              <AnimatePresence initial={false}>
                {cart.lines.map((l) => {
                  const [a, b] = pillColors(l.product.id)
                  return (
                    <motion.div
                      key={l.product.id}
                      layout
                      initial={{ opacity: 0, x: 40 }}
                      animate={{ opacity: 1, x: 0 }}
                      exit={{ opacity: 0, x: 80, height: 0, marginTop: 0 }}
                      className="card tight row"
                      style={{ '--gap': '12px', flexWrap: 'nowrap' } as React.CSSProperties}
                    >
                      <div style={{ background: b, borderRadius: 12, flexShrink: 0 }}>
                        <PillIcon form={l.product.form} a={a} b={b} size={56} />
                      </div>
                      <div className="grow stack" style={{ '--gap': '6px' } as React.CSSProperties}>
                        <span className="small" style={{ fontWeight: 700, lineHeight: 1.3 }}>
                          {l.product.name}
                        </span>
                        <div className="spread">
                          <div className="row" style={{ '--gap': '4px' } as React.CSSProperties}>
                            <button className="btn ghost sm icon" onClick={() => (l.quantity > 1 ? cart.setQty(l.product.id, l.quantity - 1) : cart.remove(l.product.id))} aria-label="Fewer">
                              −
                            </button>
                            <motion.span key={l.quantity} initial={{ scale: 1.4 }} animate={{ scale: 1 }} style={{ minWidth: 20, textAlign: 'center', fontWeight: 800 }}>
                              {l.quantity}
                            </motion.span>
                            <button className="btn ghost sm icon" onClick={() => cart.setQty(l.product.id, l.quantity + 1)} disabled={l.quantity >= 10} aria-label="More">
                              +
                            </button>
                          </div>
                          <b>{money(l.product.price * l.quantity, lang)}</b>
                        </div>
                      </div>
                    </motion.div>
                  )
                })}
              </AnimatePresence>
            </div>
            {cart.lines.length > 0 && (
              <div className="drawer-foot stack" style={{ '--gap': '12px' } as React.CSSProperties}>
                <DeliveryProgress subtotal={cart.subtotal} />
                <div className="spread">
                  <span className="muted">{t('cart.subtotal')}</span>
                  <span className="price">{money(cart.subtotal, lang)}</span>
                </div>
                <button
                  className="btn coral"
                  onClick={() => {
                    cart.setOpen(false)
                    nav('/checkout')
                  }}
                >
                  {t('cart.checkout')} →
                </button>
              </div>
            )}
          </motion.aside>
        </>
      )}
    </AnimatePresence>
  )
}

/** How far the basket is from free courier delivery (300 000 UZS). */
export function DeliveryProgress({ subtotal, threshold = 300_000 }: { subtotal: number; threshold?: number }) {
  const { lang } = useI18n()
  const pct = Math.min(100, (subtotal / threshold) * 100)
  const left = threshold - subtotal
  return (
    <div className="stack" style={{ '--gap': '6px' } as React.CSSProperties}>
      <div className="small" style={{ fontWeight: 600 }}>
        {left > 0 ? (
          <>
            {money(left, lang)} more for <b>free courier delivery</b>
          </>
        ) : (
          <>🎉 Courier delivery is free</>
        )}
      </div>
      <div style={{ height: 8, borderRadius: 4, background: 'var(--bg-2)', overflow: 'hidden' }}>
        <motion.div style={{ height: '100%', background: 'linear-gradient(90deg, var(--mint), var(--sky))', borderRadius: 4 }} animate={{ width: `${pct}%` }} transition={{ type: 'spring', stiffness: 120, damping: 20 }} />
      </div>
    </div>
  )
}
