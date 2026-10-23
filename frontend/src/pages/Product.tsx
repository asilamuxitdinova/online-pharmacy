import { motion } from 'motion/react'
import { lazy, Suspense, useState } from 'react'
import { Link, useParams } from 'react-router'
import { BranchMap } from '../components/BranchMap'
import { JobBar, useJob } from '../components/JobTracker'
import { PillIcon } from '../components/PillIcon'
import { Empty, ErrorNote, Page, Skeleton } from '../components/ui'
import { ApiError, get, post } from '../lib/api'
import { useAuth } from '../lib/auth'
import { useCart } from '../lib/cart'
import { money, pillColors } from '../lib/format'
import { useI18n } from '../lib/i18n'
import type { Job, Product as P, StockResult } from '../lib/types'
import { useAsync } from '../lib/useAsync'

const ProductModel = lazy(() => import('../three/ProductModel'))

export default function Product() {
  const { id = '' } = useParams()
  const { t, lang } = useI18n()
  const { me } = useAuth()
  const cart = useCart()
  const [qty, setQty] = useState(1)
  const [jobLoc, setJobLoc] = useState<string | null>(null)
  const [jobErr, setJobErr] = useState<ApiError | null>(null)
  const [simulateFailure, setSimulateFailure] = useState(false)
  const res = useAsync((signal) => get<P>(`/products/${id}`, undefined, { signal }), [id, lang])
  const { job, error } = useJob(jobLoc)

  if (res.error?.status === 404) {
    return (
      <Page>
        <Empty title="No such medicine" hint={res.error.message} art={<PillIcon form="tablets" a="var(--line-2)" b="var(--line)" size={100} />} />
      </Page>
    )
  }
  const p = res.data
  const [a, b] = pillColors(id)

  const startCheck = async () => {
    setJobErr(null)
    setJobLoc(null)
    try {
      const j = await post<Job>('/stock-checks', { product_id: id, simulate_failure: simulateFailure })
      setJobLoc(`/jobs/${j.id}`)
    } catch (e) {
      setJobErr(e as ApiError)
    }
  }
  const stock = job?.status === 'succeeded' ? (job.result as unknown as StockResult) : undefined

  return (
    <Page>
      <Link to="/shop" className="small muted" style={{ textDecoration: 'none' }}>
        ← {t('nav.shop')}
      </Link>
      <div className="grid-2" style={{ marginTop: 16, alignItems: 'start' }}>
        <motion.div
          className="card flat"
          style={{ aspectRatio: '1', padding: 0, overflow: 'hidden', background: `radial-gradient(circle at 35% 30%, ${b}, color-mix(in oklab, ${a} 25%, var(--card-2)))`, position: 'relative' }}
          initial={{ opacity: 0, scale: 0.96 }}
          animate={{ opacity: 1, scale: 1 }}
        >
          {p && (
            <Suspense fallback={<div style={{ display: 'grid', placeItems: 'center', height: '100%' }}><PillIcon form={p.form} a={a} b={b} size={200} /></div>}>
              <ProductModel form={p.form} a={a} b={b} />
            </Suspense>
          )}
          <span className="tiny" style={{ position: 'absolute', bottom: 14, left: 0, right: 0, textAlign: 'center', opacity: 0.6, fontWeight: 700 }}>
            drag to turn
          </span>
        </motion.div>

        <div className="stack" style={{ '--gap': '20px' } as React.CSSProperties}>
          <ErrorNote error={res.error} onRetry={res.reload} />
          {!p ? (
            <div className="stack">
              <Skeleton h={18} w="40%" />
              <Skeleton h={60} />
              <Skeleton h={30} w="30%" />
            </div>
          ) : (
            <>
              <div className="stack" style={{ '--gap': '10px' } as React.CSSProperties}>
                <div className="row">
                  <span className="kicker">{p.category.name}</span>
                  {p.requires_prescription && <span className="badge rx">{t('product.rx')}</span>}
                  {p.in_stock ? <span className="badge ok">{p.stock} {t('product.left')}</span> : <span className="badge bad">{t('product.out')}</span>}
                </div>
                <h1 style={{ fontSize: 'clamp(32px, 4.4vw, 54px)' }}>{p.name}</h1>
                <span className="price" style={{ fontSize: 34 }}>
                  {money(p.price, lang)}
                </span>
              </div>

              <div className="row">
                <div className="row chip" style={{ height: 46, '--gap': '2px', padding: 4 } as React.CSSProperties}>
                  <button className="btn ghost sm icon" style={{ border: 0 }} onClick={() => setQty((q) => Math.max(1, q - 1))} aria-label="Fewer">
                    −
                  </button>
                  <motion.b key={qty} initial={{ y: -8, opacity: 0 }} animate={{ y: 0, opacity: 1 }} style={{ minWidth: 26, textAlign: 'center' }}>
                    {qty}
                  </motion.b>
                  <button className="btn ghost sm icon" style={{ border: 0 }} onClick={() => setQty((q) => Math.min(10, q + 1))} aria-label="More">
                    +
                  </button>
                </div>
                <motion.button className="btn coral" whileTap={{ scale: 0.94 }} disabled={!p.in_stock} onClick={() => { cart.add(p, qty); cart.setOpen(true) }}>
                  {t('product.add')}
                </motion.button>
              </div>
              {p.requires_prescription && (
                <p className="small muted">
                  This medicine needs a prescription a pharmacist has approved. Upload one under <Link to="/account?tab=prescriptions">{t('account.prescriptions')}</Link> and pick it at checkout.
                </p>
              )}

              <table className="grid">
                <tbody>
                  {[
                    ['SKU', p.sku],
                    ['Form', p.form],
                    ['Strength', p.strength],
                    ['Pack', `${p.pack_size}`],
                    ['Maker', p.manufacturer],
                    ['Id', p.id],
                  ].map(([k, v]) => (
                    <tr key={k}>
                      <td className="muted" style={{ width: 120 }}>{k}</td>
                      <td className="mono">{v}</td>
                    </tr>
                  ))}
                </tbody>
              </table>

              <div className="card stack" style={{ '--gap': '14px' } as React.CSSProperties}>
                <div className="spread">
                  <h3>{t('product.stockCheck')}</h3>
                  <span className="badge plain mono">POST /stock-checks → 202</span>
                </div>
                <p className="small muted">{t('product.stockCheckHint')}</p>
                <BranchMap progress={job?.progress ?? 0} branches={stock?.branches} />
                {me ? (
                  <div className="row">
                    <button className="btn mint sm" onClick={startCheck} disabled={job?.status === 'running'}>
                      {job?.status === 'running' ? 'Checking…' : 'Start check'}
                    </button>
                    <label className="check small">
                      <input type="checkbox" checked={simulateFailure} onChange={(e) => setSimulateFailure(e.target.checked)} />
                      make it fail
                    </label>
                  </div>
                ) : (
                  <Link to="/login" className="btn sm ghost">
                    Sign in to run a stock check
                  </Link>
                )}
                <ErrorNote error={jobErr} />
                <JobBar job={job} error={error} />
                {stock && (
                  <motion.p initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="small" style={{ fontWeight: 700 }}>
                    {stock.branch_total} in branches · {stock.online_stock} online
                  </motion.p>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </Page>
  )
}
