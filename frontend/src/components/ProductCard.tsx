import { motion, useMotionValue, useSpring, useTransform } from 'motion/react'
import { Link } from 'react-router'
import { useCart } from '../lib/cart'
import { money, pillColors } from '../lib/format'
import { useI18n } from '../lib/i18n'
import type { Product } from '../lib/types'
import { PillIcon } from './PillIcon'

/** A product card that tilts toward the pointer, like a card picked up off a shelf. */
export function ProductCard({ p, index = 0 }: { p: Product; index?: number }) {
  const { t, lang } = useI18n()
  const cart = useCart()
  const [a, b] = pillColors(p.id)
  const mx = useMotionValue(0.5)
  const my = useMotionValue(0.5)
  const rx = useSpring(useTransform(my, [0, 1], [7, -7]), { stiffness: 260, damping: 22 })
  const ry = useSpring(useTransform(mx, [0, 1], [-9, 9]), { stiffness: 260, damping: 22 })
  const inCart = cart.lines.some((l) => l.product.id === p.id)

  return (
    <motion.div
      initial={{ opacity: 0, y: 24 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: Math.min(index, 12) * 0.035, type: 'spring', stiffness: 260, damping: 26 }}
      style={{ perspective: 900 }}
    >
      <motion.div
        className="pcard"
        style={{ rotateX: rx, rotateY: ry }}
        whileHover={{ y: -4 }}
        onPointerMove={(e) => {
          const r = e.currentTarget.getBoundingClientRect()
          mx.set((e.clientX - r.left) / r.width)
          my.set((e.clientY - r.top) / r.height)
        }}
        onPointerLeave={() => {
          mx.set(0.5)
          my.set(0.5)
        }}
      >
        <Link to={`/p/${p.id}`} style={{ textDecoration: 'none' }} className="stack" aria-label={p.name}>
          <div className="pcard-art" style={{ background: `radial-gradient(circle at 30% 25%, ${b}, color-mix(in oklab, ${a} 30%, var(--card-2)))` }}>
            <motion.div style={{ translateZ: 40 }} whileHover={{ rotate: -8, scale: 1.08 }}>
              <PillIcon form={p.form} a={a} b={b} size={110} />
            </motion.div>
            <div className="row" style={{ position: 'absolute', top: 10, left: 10, '--gap': '6px' } as React.CSSProperties}>
              {p.requires_prescription && <span className="badge rx">{t('product.rx')}</span>}
              {!p.in_stock && <span className="badge bad">{t('product.out')}</span>}
            </div>
          </div>
          <div className="stack" style={{ '--gap': '4px' } as React.CSSProperties}>
            <span className="tiny muted">
              {p.category.name} · {p.manufacturer}
            </span>
            <span className="pcard-name">{p.name}</span>
          </div>
        </Link>
        <div className="spread" style={{ marginTop: 'auto' }}>
          <span className="price">{money(p.price, lang)}</span>
          <motion.button
            className={`btn sm ${inCart ? 'mint' : ''}`}
            whileTap={{ scale: 0.9 }}
            disabled={!p.in_stock}
            onClick={() => cart.add(p)}
            aria-label={t('product.add')}
          >
            {inCart ? `✓ ${t('product.added')}` : '+'}
          </motion.button>
        </div>
      </motion.div>
    </motion.div>
  )
}

export function ProductCardSkeleton() {
  return (
    <div className="pcard">
      <div className="skeleton" style={{ aspectRatio: '4/3' }} />
      <div className="skeleton" style={{ height: 14, width: '60%' }} />
      <div className="skeleton" style={{ height: 18 }} />
      <div className="skeleton" style={{ height: 26, width: '40%' }} />
    </div>
  )
}
