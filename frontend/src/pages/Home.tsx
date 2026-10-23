import { animate, motion, useInView, useScroll, useTransform } from 'motion/react'
import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import { PillIcon } from '../components/PillIcon'
import { ProductCard, ProductCardSkeleton } from '../components/ProductCard'
import { Page } from '../components/ui'
import { get } from '../lib/api'
import { money, pillColors } from '../lib/format'
import { useI18n } from '../lib/i18n'
import type { Category, Paged, Product, ProductForm } from '../lib/types'
import { useAsync } from '../lib/useAsync'

const PillScene = lazy(() => import('../three/PillScene'))

const shelfForm: Record<string, ProductForm> = {
  'pain-relief': 'tablets',
  'cold-flu': 'syrup',
  antibiotics: 'capsules',
  allergy: 'spray',
  digestion: 'sachets',
  vitamins: 'capsules',
  heart: 'tablets',
  diabetes: 'tablets',
  skin: 'cream',
  'eye-ear': 'drops',
  'first-aid': 'spray',
  baby: 'drops',
}

function Headline({ text }: { text: string }) {
  const words = text.split(' ')
  return (
    <h1 aria-label={text}>
      {words.map((w, i) => (
        <span key={i} style={{ display: 'inline-block', overflow: 'hidden', verticalAlign: 'top', paddingBottom: '0.08em' }}>
          <motion.span
            aria-hidden
            style={{ display: 'inline-block' }}
            initial={{ y: '110%', rotate: 6 }}
            animate={{ y: 0, rotate: 0 }}
            transition={{ delay: 0.15 + i * 0.07, type: 'spring', stiffness: 160, damping: 18 }}
          >
            {w}&nbsp;
          </motion.span>
        </span>
      ))}
    </h1>
  )
}

/** Counts up to a number the first time it scrolls into view. */
function CountUp({ to, format = (n: number) => String(Math.round(n)) }: { to: number; format?: (n: number) => string }) {
  const ref = useRef<HTMLElement>(null)
  const seen = useInView(ref, { once: true, margin: '-40px' })
  const [v, setV] = useState(0)
  useEffect(() => {
    if (!seen) return
    const c = animate(0, to, { duration: 1.4, ease: [0.16, 1, 0.3, 1], onUpdate: setV })
    return () => c.stop()
  }, [seen, to])
  return <b ref={ref}>{format(v)}</b>
}

function useDark() {
  const [dark, setDark] = useState(false)
  useEffect(() => {
    const check = () => {
      const t = document.documentElement.dataset.theme
      setDark(t ? t === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches)
    }
    check()
    const obs = new MutationObserver(check)
    obs.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
    return () => obs.disconnect()
  }, [])
  return dark
}

function useWide() {
  const [wide, setWide] = useState(() => window.innerWidth >= 900)
  useEffect(() => {
    const on = () => setWide(window.innerWidth >= 900)
    window.addEventListener('resize', on)
    return () => window.removeEventListener('resize', on)
  }, [])
  return wide
}

export default function Home() {
  const { t, lang } = useI18n()
  const dark = useDark()
  const wide = useWide()
  const cats = useAsync((signal) => get<{ data: Category[] }>('/categories', undefined, { signal }), [lang])
  const fresh = useAsync((signal) => get<Paged<Product>>('/products', { sort: 'newest', page_size: 8, in_stock: true }, { signal }), [lang])
  const marquee = useAsync((signal) => get<Paged<Product>>('/products', { page_size: 24, sort: '-price' }, { signal }), [lang])

  const heroRef = useRef<HTMLDivElement>(null)
  const { scrollYProgress } = useScroll({ target: heroRef, offset: ['start start', 'end start'] })
  const sceneY = useTransform(scrollYProgress, [0, 1], ['0%', '25%'])
  const sceneOpacity = useTransform(scrollYProgress, [0, 0.9], [1, 0.2])

  return (
    <Page>
      <section ref={heroRef} style={{ position: 'relative', minHeight: 'min(760px, calc(100vh - 100px))', display: 'grid', alignItems: 'center' }}>
        <motion.div style={{ position: 'absolute', top: -60, bottom: 0, left: 'calc(50% - 50vw)', right: 'calc(50% - 50vw)', y: sceneY, opacity: wide ? sceneOpacity : 0.55, zIndex: 0 }} aria-hidden>
          <Suspense fallback={null}>
            <PillScene dark={dark} offsetX={wide ? 3.2 : 0} count={wide ? 22 : 12} />
          </Suspense>
        </motion.div>
        <div className="stack" style={{ '--gap': '22px', position: 'relative', zIndex: 1, maxWidth: 680, pointerEvents: 'none', padding: '24px 24px 24px 0', background: 'radial-gradient(closest-side, color-mix(in oklab, var(--bg) 80%, transparent), transparent)' } as React.CSSProperties}>
          <motion.span className="kicker" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
            ● {t('hero.kicker')}
          </motion.span>
          <Headline key={lang} text={t('hero.title')} />
          <motion.p className="muted" style={{ fontSize: 19, maxWidth: 560 }} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.6 }}>
            {t('hero.sub')}
          </motion.p>
          <motion.div className="row" style={{ pointerEvents: 'auto' }} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.75 }}>
            <Link to="/shop" className="btn coral">
              {t('hero.cta')} →
            </Link>
            <a href="/docs/" className="btn ghost">
              {t('hero.policies')}
            </a>
          </motion.div>
        </div>
      </section>

      <div className="marquee" style={{ margin: '10px 0 70px' }} aria-hidden>
        <div className="marquee-track">
          {[...(marquee.data?.data ?? []), ...(marquee.data?.data ?? [])].map((p, i) => {
            const [a, b] = pillColors(p.id)
            return (
              <span key={i} className="row" style={{ '--gap': '10px', fontFamily: 'var(--display)', fontSize: 24, fontWeight: 600 } as React.CSSProperties}>
                <PillIcon form={p.form} a={a} b={b} size={36} />
                {p.name.split(',')[0]}
              </span>
            )
          })}
        </div>
      </div>

      <section className="stack" style={{ '--gap': '24px', marginBottom: 80 } as React.CSSProperties}>
        <div className="spread">
          <h2>{t('home.categories')}</h2>
          <Link to="/shop" className="btn ghost sm">
            {t('shop.all')} →
          </Link>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(170px, 1fr))', gap: 14 }}>
          {(cats.data?.data ?? Array.from({ length: 12 }, () => null)).map((c, i) => {
            if (!c) return <div key={i} className="skeleton" style={{ height: 150, borderRadius: 20 }} />
            const [a, b] = pillColors(c.id + 'shelf')
            return (
              <motion.div key={c.id} initial={{ opacity: 0, scale: 0.9 }} whileInView={{ opacity: 1, scale: 1 }} viewport={{ once: true }} transition={{ delay: (i % 6) * 0.05 }}>
                <Link to={`/shop?category=${c.id}`} style={{ textDecoration: 'none' }}>
                  <motion.div
                    className="card flat"
                    style={{ padding: 16, height: 150, display: 'flex', flexDirection: 'column', justifyContent: 'space-between', background: `linear-gradient(160deg, ${b}, var(--card) 70%)`, overflow: 'hidden', position: 'relative' }}
                    whileHover="hover"
                  >
                    <motion.div style={{ position: 'absolute', right: -6, top: -6 }} variants={{ hover: { rotate: 25, scale: 1.15, x: -6, y: 6 } }} transition={{ type: 'spring', stiffness: 300, damping: 14 }}>
                      <PillIcon form={shelfForm[c.id] ?? 'capsules'} a={a} b={b} size={84} />
                    </motion.div>
                    <span />
                    <div>
                      <div style={{ fontWeight: 800, lineHeight: 1.2 }}>{c.name}</div>
                      <div className="tiny muted">{c.product_count} items</div>
                    </div>
                  </motion.div>
                </Link>
              </motion.div>
            )
          })}
        </div>
      </section>

      <section className="card" style={{ marginBottom: 80, background: 'var(--ink)', color: 'var(--bg)', borderColor: 'transparent', overflow: 'hidden', position: 'relative' }}>
        <div className="stack" style={{ '--gap': '28px', padding: 'clamp(8px, 2vw, 24px)' } as React.CSSProperties}>
          <h2>{t('home.rules')}</h2>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 28 }}>
            <div className="stat">
              <CountUp to={300000} format={(n) => money(Math.round(n / 1000) * 1000, lang)} />
              <span style={{ opacity: 0.7 }}>{t('rule.delivery')}</span>
            </div>
            <div className="stat">
              <CountUp to={200000} format={(n) => money(Math.round(n / 1000) * 1000, lang)} />
              <span style={{ opacity: 0.7 }}>{t('rule.refund')}</span>
            </div>
            <div className="stat">
              <CountUp to={8} />
              <span style={{ opacity: 0.7 }}>{t('rule.cities')}</span>
            </div>
            <div className="stat">
              <CountUp to={16} format={(n) => `${Math.round(n)}:00`} />
              <span style={{ opacity: 0.7 }}>{t('rule.sameDay')}</span>
            </div>
          </div>
          <a href="/docs/" className="small" style={{ opacity: 0.8 }}>
            {t('hero.policies')} →
          </a>
        </div>
      </section>

      <section className="stack" style={{ '--gap': '24px' } as React.CSSProperties}>
        <h2>{t('home.featured')}</h2>
        <div className="products">
          {fresh.data ? fresh.data.data.map((p, i) => <ProductCard key={p.id} p={p} index={i} />) : Array.from({ length: 8 }, (_, i) => <ProductCardSkeleton key={i} />)}
        </div>
      </section>
    </Page>
  )
}
