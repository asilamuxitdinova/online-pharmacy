import { AnimatePresence, motion } from 'motion/react'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { JobBar, useJob } from '../components/JobTracker'
import { ThresholdMeter } from '../components/ThresholdMeter'
import { Empty, ErrorNote, Field, Loading, Page, StatusBadge, Tabs } from '../components/ui'
import { ApiError, get, post } from '../lib/api'
import { useAuth } from '../lib/auth'
import { day, money, when } from '../lib/format'
import { useI18n } from '../lib/i18n'
import { useToast } from '../lib/toast'
import type { Job, Order, Paged, Prescription, Refund, StaffUser } from '../lib/types'
import { useAsync } from '../lib/useAsync'

type Tab = 'refunds' | 'prescriptions' | 'orders' | 'customers' | 'reports'

export default function Staff() {
  const { t } = useI18n()
  const [tab, setTab] = useState<Tab>('refunds')
  return (
    <Page>
      <div className="stack" style={{ '--gap': '24px' } as React.CSSProperties}>
        <div className="stack" style={{ '--gap': '6px' } as React.CSSProperties}>
          <span className="kicker">role: staff</span>
          <h1 style={{ fontSize: 'clamp(36px, 5vw, 60px)' }}>{t('nav.staff')}</h1>
        </div>
        <Tabs<Tab>
          value={tab}
          onChange={setTab}
          items={[
            { id: 'refunds', label: 'Refund queue' },
            { id: 'prescriptions', label: 'Prescriptions' },
            { id: 'orders', label: 'All orders' },
            { id: 'customers', label: 'Customers' },
            { id: 'reports', label: 'Sales report' },
          ]}
        />
        <AnimatePresence mode="wait">
          <motion.div key={tab} initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }} transition={{ duration: 0.2 }}>
            {tab === 'refunds' && <RefundQueue />}
            {tab === 'prescriptions' && <RxQueue />}
            {tab === 'orders' && <AllOrders />}
            {tab === 'customers' && <Customers />}
            {tab === 'reports' && <Report />}
          </motion.div>
        </AnimatePresence>
      </div>
    </Page>
  )
}

function RefundQueue() {
  const { lang } = useI18n()
  const toast = useToast()
  const [status, setStatus] = useState<'pending_review' | 'approved' | 'rejected'>('pending_review')
  const q = useAsync((s) => get<Paged<Refund>>('/staff/refunds', { status }, { signal: s }), [status])
  const [decided, setDecided] = useState<Record<string, 'approved' | 'rejected'>>({})
  const [error, setError] = useState<ApiError | null>(null)
  const decide = async (r: Refund, approve: boolean) => {
    setError(null)
    try {
      await post(`/staff/refunds/${r.id}/${approve ? 'approve' : 'reject'}`)
      setDecided((d) => ({ ...d, [r.id]: approve ? 'approved' : 'rejected' }))
      toast(approve ? `Approved: ${money(r.amount, lang)} goes back to the card` : 'Rejected', approve ? 'ok' : 'info')
      window.setTimeout(q.reload, 900)
    } catch (e) {
      setError(e as ApiError)
    }
  }
  return (
    <div className="stack" style={{ '--gap': '16px' } as React.CSSProperties}>
      <div className="row">
        {(['pending_review', 'approved', 'rejected'] as const).map((s) => (
          <button key={s} className={`chip ${status === s ? 'on' : ''}`} onClick={() => setStatus(s)}>
            <StatusBadge status={s} />
          </button>
        ))}
      </div>
      <ErrorNote error={error ?? q.error} />
      {q.loading && !q.data && <Loading />}
      {q.data?.data.length === 0 && <Empty title="Nothing waiting" hint="Refunds over 200 000 UZS land here. Smaller ones were approved automatically." />}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 16 }}>
        {q.data?.data.map((r) => (
          <motion.div key={r.id} layout className="card stack" style={{ '--gap': '12px', position: 'relative', overflow: 'hidden' } as React.CSSProperties}>
            <div className="spread">
              <span className="price">{money(r.amount, lang)}</span>
              <StatusBadge status={r.status} />
            </div>
            <ThresholdMeter amount={r.amount} max={r.amount} />
            <div className="small">“{r.reason}”</div>
            <div className="tiny muted mono">
              {r.id} · {r.order_id} · {when(r.created_at, lang)}
            </div>
            {r.status === 'pending_review' && (
              <div className="row">
                <button className="btn mint sm" onClick={() => decide(r, true)}>Approve · moves money</button>
                <button className="btn ghost sm" onClick={() => decide(r, false)}>Reject</button>
              </div>
            )}
            <AnimatePresence>
              {decided[r.id] && (
                <motion.div
                  initial={{ scale: 3, opacity: 0, rotate: -40 }}
                  animate={{ scale: 1, opacity: 0.9, rotate: -12 }}
                  style={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', background: 'color-mix(in oklab, var(--card) 70%, transparent)' }}
                >
                  <span style={{ border: '4px solid', borderRadius: 10, padding: '6px 16px', fontWeight: 900, fontSize: 26, letterSpacing: '.14em', textTransform: 'uppercase', color: decided[r.id] === 'approved' ? 'var(--mint-ink)' : 'var(--bad)' }}>
                    {decided[r.id]}
                  </span>
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        ))}
      </div>
    </div>
  )
}

function RxQueue() {
  const { lang } = useI18n()
  const toast = useToast()
  const q = useAsync((s) => get<{ data: Prescription[] }>('/staff/prescriptions', undefined, { signal: s }), [])
  const [notes, setNotes] = useState<Record<string, string>>({})
  const [errors, setErrors] = useState<Record<string, ApiError>>({})
  const review = async (p: Prescription, decision: 'approve' | 'reject') => {
    try {
      await post(`/staff/prescriptions/${p.id}/review`, { decision, note: notes[p.id] ?? '' })
      toast(`Prescription ${decision}d`, 'ok')
      q.reload()
    } catch (e) {
      setErrors((x) => ({ ...x, [p.id]: e as ApiError }))
    }
  }
  return (
    <div className="stack" style={{ '--gap': '14px' } as React.CSSProperties}>
      {q.data?.data.length === 0 && <Empty title="No prescriptions to review" />}
      <AnimatePresence>
        {q.data?.data.map((p) => (
          <motion.div key={p.id} layout exit={{ opacity: 0, x: 80 }} className="card" style={{ display: 'grid', gap: 16, gridTemplateColumns: 'minmax(0,1fr)' }}>
            <div className="spread">
              <div>
                <b>{p.doctor_name}</b> <span className="muted small">· {p.clinic}</span>
                <div className="small">Issued {day(p.issued_on, lang)} · patient <span className="mono">{p.user_id}</span></div>
              </div>
              <StatusBadge status={p.status} />
            </div>
            <div className="card tight flat" style={{ background: 'var(--card-2)' }}>
              <span className="tiny muted">Diagnosis (medical information)</span>
              <div>{p.diagnosis}</div>
            </div>
            <div className="row" style={{ alignItems: 'flex-end' }}>
              <Field label="Note (required to reject)" error={errors[p.id]?.fieldError('note')}>
                <input className="input" value={notes[p.id] ?? ''} onChange={(e) => setNotes({ ...notes, [p.id]: e.target.value })} />
              </Field>
              <button className="btn mint" onClick={() => review(p, 'approve')}>Approve</button>
              <button className="btn ghost" onClick={() => review(p, 'reject')}>Reject</button>
            </div>
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  )
}

function AllOrders() {
  const { lang } = useI18n()
  const [page, setPage] = useState(1)
  const q = useAsync((s) => get<Paged<Order>>('/staff/orders', { page, page_size: 15 }, { signal: s }), [page, lang])
  return (
    <div className="card stack" style={{ '--gap': '12px' } as React.CSSProperties}>
      <ErrorNote error={q.error} />
      <div className="table-scroll">
        <table className="grid">
          <thead>
            <tr><th>Order</th><th>Customer</th><th>Status</th><th>Items</th><th style={{ textAlign: 'right' }}>Total</th><th>Placed</th></tr>
          </thead>
          <tbody>
            {q.data?.data.map((o) => (
              <tr key={o.id}>
                <td className="mono small">{o.id}</td>
                <td className="mono small">{o.user_id}</td>
                <td><StatusBadge status={o.status} /></td>
                <td>{o.items.length}</td>
                <td style={{ textAlign: 'right' }}><b>{money(o.total, lang)}</b></td>
                <td className="small muted">{when(o.created_at, lang)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {q.data && (
        <div className="spread small">
          <span className="muted">{q.data.total} orders · page {q.data.page} of {q.data.total_pages}</span>
          <div className="row">
            <button className="btn ghost sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>←</button>
            <button className="btn ghost sm" disabled={page >= q.data.total_pages} onClick={() => setPage(page + 1)}>→</button>
          </div>
        </div>
      )}
    </div>
  )
}

/** Sensitive values stay blurred until pointed at, like on a shared screen. */
export function Secret({ value }: { value: string | null }) {
  const [shown, setShown] = useState(false)
  if (!value) return <span className="muted">—</span>
  return (
    <span
      className="mono"
      onMouseEnter={() => setShown(true)}
      onMouseLeave={() => setShown(false)}
      onFocus={() => setShown(true)}
      onBlur={() => setShown(false)}
      tabIndex={0}
      title="Sensitive: taxpayer number"
      style={{ filter: shown ? 'none' : 'blur(5px)', transition: 'filter .2s', cursor: 'help' }}
    >
      {value}
    </span>
  )
}

function Customers() {
  const { lang } = useI18n()
  const { actAs } = useAuth()
  const nav = useNavigate()
  const [qtext, setQ] = useState('')
  const [open, setOpen] = useState<string | null>(null)
  const list = useAsync((s) => get<Paged<StaffUser>>('/staff/customers', { q: qtext }, { signal: s }), [qtext])
  const detail = useAsync((s) => (open ? get<StaffUser>(`/staff/customers/${open}`, undefined, { signal: s }) : Promise.resolve(undefined)), [open])
  return (
    <div className="grid-side">
      <div className="stack" style={{ '--gap': '12px' } as React.CSSProperties}>
        <input className="input" placeholder="Search name, email or phone" value={qtext} onChange={(e) => setQ(e.target.value)} />
        {list.data?.data.map((u) => (
          <motion.button key={u.id} layout className="card tight spread" style={{ cursor: 'pointer', textAlign: 'left', borderColor: open === u.id ? 'var(--ink)' : undefined }} onClick={() => setOpen(u.id)}>
            <div>
              <b>{u.name}</b> {u.disabled && <span className="badge bad">disabled</span>}
              <div className="small muted">{u.email} · {u.phone}</div>
            </div>
            <span className="muted">→</span>
          </motion.button>
        ))}
      </div>
      <div className="card stack sticky" style={{ '--gap': '12px' } as React.CSSProperties}>
        {!open && <p className="muted">Pick a customer.</p>}
        {detail.data && (
          <>
            <h3>{detail.data.name}</h3>
            <table className="grid">
              <tbody>
                <tr><td className="muted">Id</td><td className="mono">{detail.data.id}</td></tr>
                <tr><td className="muted">TIN</td><td><Secret value={detail.data.tin} /> <span className="badge warn" style={{ marginLeft: 6 }}>sensitive</span></td></tr>
                <tr><td className="muted">Email</td><td>{detail.data.email}</td></tr>
                <tr><td className="muted">Phone</td><td>{detail.data.phone}</td></tr>
                <tr><td className="muted">Orders</td><td>{detail.data.order_count}</td></tr>
                <tr><td className="muted">Spent</td><td>{money(detail.data.lifetime_spent ?? 0, lang)}</td></tr>
              </tbody>
            </table>
            <button
              className="btn"
              onClick={() => {
                actAs(detail.data!.id)
                nav('/account')
              }}
            >
              See what {detail.data.name.split(' ')[0]} sees <span className="tiny mono">X-Act-As</span>
            </button>
          </>
        )}
      </div>
    </div>
  )
}

function Report() {
  const { lang } = useI18n()
  const today = new Date().toISOString().slice(0, 10)
  const monthAgo = new Date(Date.now() - 30 * 864e5).toISOString().slice(0, 10)
  const [from, setFrom] = useState(monthAgo)
  const [to, setTo] = useState(today)
  const [loc, setLoc] = useState<string | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  const { job, error: jobError } = useJob(loc)
  const start = async () => {
    setError(null)
    setLoc(null)
    try {
      const j = await post<Job>('/staff/reports/sales', { from, to })
      setLoc(`/jobs/${j.id}`)
    } catch (e) {
      setError(e as ApiError)
    }
  }
  const r = job?.status === 'succeeded' ? (job.result as Record<string, number>) : null
  return (
    <div className="grid-2">
      <div className="card stack" style={{ '--gap': '14px' } as React.CSSProperties}>
        <h3>Sales report</h3>
        <p className="small muted">A slow job: 202 Accepted, then poll until it is done. An assistant must not report numbers before it finishes.</p>
        <div className="row" style={{ flexWrap: 'nowrap' }}>
          <Field label="From" error={error?.fieldError('from')}><input type="date" className="input" value={from} onChange={(e) => setFrom(e.target.value)} /></Field>
          <Field label="To" error={error?.fieldError('to')}><input type="date" className="input" value={to} onChange={(e) => setTo(e.target.value)} /></Field>
        </div>
        <button className="btn coral" onClick={start} disabled={job?.status === 'running'}>Run report</button>
        <ErrorNote error={error && !error.body.fields ? error : null} />
        <JobBar job={job} error={jobError} />
      </div>
      <div className="card" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20, alignContent: 'center' }}>
        {[
          ['Paid orders', r?.paid_orders, false],
          ['Revenue', r?.revenue, true],
          ['Refunded', r?.refunded, true],
          ['Net', r?.net, true],
        ].map(([label, v, isMoney]) => (
          <div key={label as string} className="stat">
            <motion.b key={String(v)} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} style={{ fontSize: 30 }}>
              {v === undefined ? '—' : isMoney ? money(v as number, lang) : (v as number)}
            </motion.b>
            <span className="muted small">{label as string}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
