import { AnimatePresence, motion } from 'motion/react'
import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router'
import { PillIcon } from '../components/PillIcon'
import { ProductCard, ProductCardSkeleton } from '../components/ProductCard'
import { Empty, ErrorNote, Page } from '../components/ui'
import { get } from '../lib/api'
import { useI18n } from '../lib/i18n'
import type { Category, Paged, Product } from '../lib/types'
import { useAsync } from '../lib/useAsync'

const sorts = [
  { id: 'name', label: 'A → Z' },
  { id: '-name', label: 'Z → A' },
  { id: 'price', label: 'Cheapest' },
  { id: '-price', label: 'Priciest' },
  { id: 'newest', label: 'Newest' },
]

export default function Shop() {
  const { t, lang } = useI18n()
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const category = params.get('category') ?? ''
  const rx = params.get('requires_prescription') ?? ''
  const inStock = params.get('in_stock') ?? ''
  const sort = params.get('sort') ?? 'name'
  const page = Number(params.get('page') ?? 1)
  const minPrice = params.get('min_price') ?? ''
  const maxPrice = params.get('max_price') ?? ''
  const [draft, setDraft] = useState(q)

  const set = (patch: Record<string, string>) => {
    const next = new URLSearchParams(params)
    for (const [k, v] of Object.entries(patch)) (v ? next.set(k, v) : next.delete(k))
    if (!('page' in patch)) next.delete('page')
    setParams(next, { replace: true })
  }

  // Search as you type, a moment after typing stops.
  useEffect(() => {
    const id = window.setTimeout(() => draft !== q && set({ q: draft }), 280)
    return () => window.clearTimeout(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft])

  const cats = useAsync((signal) => get<{ data: Category[] }>('/categories', undefined, { signal }), [lang])
  const res = useAsync(
    (signal) =>
      get<Paged<Product>>(
        '/products',
        { q, category, requires_prescription: rx, in_stock: inStock, sort, page, page_size: 24, min_price: minPrice, max_price: maxPrice },
        { signal },
      ),
    [q, category, rx, inStock, sort, page, minPrice, maxPrice, lang],
  )

  return (
    <Page>
      <div className="stack" style={{ '--gap': '22px' } as React.CSSProperties}>
        <div className="stack" style={{ '--gap': '6px' } as React.CSSProperties}>
          <h1 style={{ fontSize: 'clamp(36px, 5vw, 60px)' }}>{t('nav.shop')}</h1>
        </div>

        <div style={{ position: 'relative' }}>
          <input className="input" style={{ height: 60, fontSize: 18, paddingLeft: 54, borderRadius: 999 }} placeholder={t('shop.search')} value={draft} onChange={(e) => setDraft(e.target.value)} aria-label="Search" />
          <motion.span style={{ position: 'absolute', left: 20, top: 18 }} animate={res.loading ? { rotate: 360 } : { rotate: 0 }} transition={res.loading ? { repeat: Infinity, duration: 0.8, ease: 'linear' } : {}}>
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round">
              <circle cx="11" cy="11" r="7" />
              <path d="m20 20-3.5-3.5" />
            </svg>
          </motion.span>
        </div>

        <div className="row" style={{ overflowX: 'auto', flexWrap: 'nowrap', paddingBottom: 4, scrollbarWidth: 'none' }}>
          <button className={`chip ${!category ? 'on' : ''}`} onClick={() => set({ category: '' })}>
            {t('shop.all')}
          </button>
          {cats.data?.data.map((c) => (
            <button key={c.id} className={`chip ${category === c.id ? 'on' : ''}`} onClick={() => set({ category: category === c.id ? '' : c.id })}>
              {c.name}
            </button>
          ))}
        </div>

        <div className="spread">
          <div className="row">
            <label className="check small">
              <input type="checkbox" checked={rx === 'true'} onChange={(e) => set({ requires_prescription: e.target.checked ? 'true' : '' })} />
              {t('shop.rxOnly')}
            </label>
            <label className="check small">
              <input type="checkbox" checked={inStock === 'true'} onChange={(e) => set({ in_stock: e.target.checked ? 'true' : '' })} />
              {t('shop.inStock')}
            </label>
            <input className="input" style={{ width: 120, height: 36 }} inputMode="numeric" placeholder="min UZS" defaultValue={minPrice} onBlur={(e) => set({ min_price: e.target.value })} aria-label="Minimum price" />
            <input className="input" style={{ width: 120, height: 36 }} inputMode="numeric" placeholder="max UZS" defaultValue={maxPrice} onBlur={(e) => set({ max_price: e.target.value })} aria-label="Maximum price" />
          </div>
          <div className="row">
            <span className="small muted">
              {res.data ? (
                <>
                  <motion.b key={res.data.total} initial={{ opacity: 0, y: -6 }} animate={{ opacity: 1, y: 0 }} style={{ display: 'inline-block' }}>
                    {res.data.total}
                  </motion.b>{' '}
                  {t('shop.results')}
                </>
              ) : null}
            </span>
            <select className="chip" value={sort} onChange={(e) => set({ sort: e.target.value })} aria-label={t('shop.sort')}>
              {sorts.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        <ErrorNote error={res.error} onRetry={res.reload} />

        {res.data && res.data.total === 0 ? (
          <Empty
            title={t('shop.empty')}
            hint={t('shop.emptyHint')}
            art={
              <motion.div animate={{ y: [0, -10, 0], rotate: [0, 8, 0] }} transition={{ repeat: Infinity, duration: 2.4 }}>
                <PillIcon form="syrup" a="var(--lilac)" b="var(--sun)" size={110} />
              </motion.div>
            }
          />
        ) : (
          <div className="products" style={{ opacity: res.loading && res.data ? 0.55 : 1, transition: 'opacity .2s' }}>
            <AnimatePresence>
              {res.data ? res.data.data.map((p, i) => <ProductCard key={p.id} p={p} index={i} />) : Array.from({ length: 12 }, (_, i) => <ProductCardSkeleton key={i} />)}
            </AnimatePresence>
          </div>
        )}

        {res.data && res.data.total_pages > 1 && <Pager page={page} pages={res.data.total_pages} onPage={(p) => set({ page: String(p) })} />}
      </div>
    </Page>
  )
}

function Pager({ page, pages, onPage }: { page: number; pages: number; onPage: (p: number) => void }) {
  const around = new Set([1, pages, page - 1, page, page + 1].filter((p) => p >= 1 && p <= pages))
  const list = [...around].sort((a, b) => a - b)
  return (
    <nav className="row" style={{ justifyContent: 'center', marginTop: 12 }} aria-label="Pages">
      <button className="btn ghost sm" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        ←
      </button>
      {list.map((p, i) => (
        <span key={p} className="row" style={{ '--gap': '4px' } as React.CSSProperties}>
          {i > 0 && list[i - 1] !== p - 1 && <span className="muted">…</span>}
          <button className={`chip ${p === page ? 'on' : ''}`} onClick={() => onPage(p)} style={{ position: 'relative' }}>
            {p}
          </button>
        </span>
      ))}
      <button className="btn ghost sm" disabled={page >= pages} onClick={() => onPage(page + 1)}>
        →
      </button>
    </nav>
  )
}
