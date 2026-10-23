import { motion } from 'motion/react'
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { PillIcon } from '../components/PillIcon'
import { ThresholdMeter } from '../components/ThresholdMeter'
import { Empty, ErrorNote, Field, Loading, Modal, Page, StatusBadge } from '../components/ui'
import { ApiError, del, get, newIdempotencyKey, patch, post } from '../lib/api'
import { money, pillColors, when } from '../lib/format'
import { useI18n } from '../lib/i18n'
import { useToast } from '../lib/toast'
import type { Address, Order, Refund } from '../lib/types'
import { useAsync } from '../lib/useAsync'
import { Pay } from './Checkout'

interface RefundList {
  data: Refund[]
  refundable: number
  rule: string
}

export default function OrderDetail() {
  const { id = '' } = useParams()
  const { t, lang } = useI18n()
  const toast = useToast()
  const nav = useNavigate()
  const order = useAsync((s) => get<Order>(`/orders/${id}`, undefined, { signal: s }), [id, lang])
  const refunds = useAsync((s) => get<RefundList>(`/orders/${id}/refunds`, undefined, { signal: s }), [id])
  const [paying, setPaying] = useState(false)
  const [confirmCancel, setConfirmCancel] = useState(false)
  const [invoice, setInvoice] = useState<string | null>(null)
  const [actionError, setActionError] = useState<ApiError | null>(null)

  if (order.error?.status === 404) {
    return (
      <Page>
        <Empty title="No such order" hint="It does not exist, or it belongs to someone else — the API answers 404 either way." art={<PillIcon form="sachets" a="var(--line-2)" b="var(--bg-2)" size={100} />} />
      </Page>
    )
  }
  const o = order.data
  if (!o) return <Page><Loading /><ErrorNote error={order.error} /></Page>

  const act = async (fn: () => Promise<unknown>, ok: string) => {
    setActionError(null)
    try {
      await fn()
      toast(ok, 'ok')
      order.reload()
      refunds.reload()
    } catch (e) {
      setActionError(e as ApiError)
    }
  }

  const steps = [
    { label: 'Placed', at: o.created_at, done: true },
    { label: o.status === 'cancelled' ? 'Cancelled' : 'Paid', at: o.status === 'cancelled' ? o.updated_at : o.paid_at, done: o.status !== 'pending_payment', bad: o.status === 'cancelled' },
    ...(refunds.data?.data ?? []).map((r) => ({ label: `Refund ${money(r.amount, lang)} · ${r.status.replace('_', ' ')}`, at: r.decided_at ?? r.created_at, done: r.status === 'approved', bad: r.status === 'rejected' })),
  ]

  return (
    <Page>
      <Link to="/account" className="small muted" style={{ textDecoration: 'none' }}>
        ← {t('account.orders')}
      </Link>
      <div className="spread" style={{ margin: '14px 0 24px' }}>
        <div className="stack" style={{ '--gap': '8px' } as React.CSSProperties}>
          <span className="kicker mono">{o.id}</span>
          <h1 style={{ fontSize: 'clamp(32px, 4.5vw, 54px)' }}>{money(o.total, lang)}</h1>
        </div>
        <StatusBadge status={o.status} />
      </div>

      <div className="grid-side">
        <div className="stack" style={{ '--gap': '18px' } as React.CSSProperties}>
          <div className="card">
            <div className="row" style={{ '--gap': '0', alignItems: 'stretch', flexWrap: 'nowrap', overflowX: 'auto' } as React.CSSProperties}>
              {steps.map((s, i) => (
                <div key={i} className="stack" style={{ '--gap': '8px', minWidth: 150, flex: 1 } as React.CSSProperties}>
                  <div className="row" style={{ '--gap': '0', flexWrap: 'nowrap' } as React.CSSProperties}>
                    <motion.span
                      initial={{ scale: 0 }}
                      animate={{ scale: 1 }}
                      transition={{ delay: i * 0.15, type: 'spring' }}
                      style={{ width: 18, height: 18, borderRadius: 9, flexShrink: 0, background: s.bad ? 'var(--bad)' : s.done ? 'var(--mint)' : 'var(--bg-2)', border: '3px solid var(--card)', boxShadow: '0 0 0 2px var(--line-2)' }}
                    />
                    {i < steps.length - 1 && <motion.span initial={{ scaleX: 0 }} animate={{ scaleX: 1 }} transition={{ delay: i * 0.15 + 0.1 }} style={{ height: 3, flex: 1, background: steps[i + 1].done ? 'var(--mint)' : 'var(--line)', transformOrigin: 'left' }} />}
                  </div>
                  <b className="small">{s.label}</b>
                  <span className="tiny muted">{s.at ? when(s.at, lang) : 'waiting'}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="card stack" style={{ '--gap': '12px' } as React.CSSProperties}>
            {o.items.map((it) => {
              const [a, b] = pillColors(it.product_id)
              return (
                <Link key={it.product_id} to={`/p/${it.product_id}`} className="row" style={{ textDecoration: 'none', flexWrap: 'nowrap' }}>
                  <div style={{ background: b, borderRadius: 12 }}>
                    <PillIcon form="capsules" a={a} b={b} size={48} />
                  </div>
                  <span className="grow">
                    <b className="small">{it.name}</b>
                    <div className="tiny muted">
                      {it.quantity} × {money(it.unit_price, lang)}
                    </div>
                  </span>
                  <b>{money(it.line_total, lang)}</b>
                </Link>
              )
            })}
            <hr className="divider" />
            <div className="spread small"><span className="muted">Subtotal</span><span>{money(o.subtotal, lang)}</span></div>
            <div className="spread small"><span className="muted">Delivery · {o.delivery}</span><span>{o.delivery_fee ? money(o.delivery_fee, lang) : 'free'}</span></div>
            {o.refunded > 0 && <div className="spread small"><span className="muted">Refunded</span><span style={{ color: 'var(--mint-ink)' }}>−{money(o.refunded, lang)}</span></div>}
          </div>

          {paying && o.status === 'pending_payment' && (
            <Pay
              order={o}
              onPaid={() => {
                setPaying(false)
                toast(t('checkout.paid'), 'ok')
                order.reload()
              }}
            />
          )}

          {(o.status === 'paid' || o.status === 'partially_refunded') && refunds.data && (
            <RefundBox order={o} refundable={refunds.data.refundable} onDone={() => { order.reload(); refunds.reload() }} />
          )}
        </div>

        <aside className="stack sticky" style={{ '--gap': '14px' } as React.CSSProperties}>
          <div className="card stack" style={{ '--gap': '10px' } as React.CSSProperties}>
            <h3>Actions</h3>
            {o.status === 'pending_payment' && (
              <>
                <button className="btn coral" onClick={() => setPaying(true)}>{t('order.payNow')}</button>
                <DeliverySwitch order={o} onChange={(next) => act(() => patch(`/orders/${o.id}`, next), 'Delivery updated (PATCH)')} />
                <button className="btn ghost" onClick={() => setConfirmCancel(true)}>{t('order.cancel')}</button>
              </>
            )}
            {(o.status === 'pending_payment' || o.status === 'cancelled') && (
              <button className="btn ghost" onClick={() => act(async () => { await del(`/orders/${o.id}`); nav('/account') }, 'Order deleted (204)')}>
                {t('common.delete')}
              </button>
            )}
            <button
              className="btn ghost"
              onClick={async () => {
                try {
                  setInvoice(await get<string>(`/orders/${o.id}/invoice`, undefined, { text: true }))
                } catch (e) {
                  setActionError(e as ApiError)
                }
              }}
            >
              🧾 {t('order.invoice')} <span className="tiny muted">(text/html)</span>
            </button>
            <ErrorNote error={actionError} />
          </div>
          {refunds.data && refunds.data.data.length > 0 && (
            <div className="card stack" style={{ '--gap': '10px' } as React.CSSProperties}>
              <h3>Refunds</h3>
              {refunds.data.data.map((r) => (
                <div key={r.id} className="spread small">
                  <span>
                    {money(r.amount, lang)} <span className="muted">· {r.reason}</span>
                  </span>
                  <StatusBadge status={r.status} />
                </div>
              ))}
            </div>
          )}
        </aside>
      </div>

      <Modal open={confirmCancel} onClose={() => setConfirmCancel(false)}>
        <div className="stack" style={{ '--gap': '14px' } as React.CSSProperties}>
          <h3>Cancel {o.id}?</h3>
          <p className="muted">It has not been paid, so nothing is charged. A cancelled order cannot be paid later.</p>
          <div className="row">
            <button className="btn danger" onClick={() => { setConfirmCancel(false); void act(() => post(`/orders/${o.id}/cancel`), 'Order cancelled') }}>
              Yes, cancel it
            </button>
            <button className="btn ghost" onClick={() => setConfirmCancel(false)}>Keep it</button>
          </div>
        </div>
      </Modal>
      <Modal open={!!invoice} onClose={() => setInvoice(null)}>
        <div className="stack" style={{ '--gap': '12px' } as React.CSSProperties}>
          <div className="spread">
            <h3>{t('order.invoice')}</h3>
            <button className="btn ghost sm icon" onClick={() => setInvoice(null)} aria-label="Close">✕</button>
          </div>
          {invoice && <iframe title="Invoice" srcDoc={invoice} sandbox="" style={{ width: '100%', height: 460, border: '1px solid var(--line)', borderRadius: 12, background: '#fff' }} />}
        </div>
      </Modal>
    </Page>
  )
}

function DeliverySwitch({ order, onChange }: { order: Order; onChange: (b: Record<string, string>) => void }) {
  const addresses = useAsync((s) => get<{ data: Address[] }>('/me/addresses', undefined, { signal: s }), [])
  const next = order.delivery === 'courier' ? 'pickup' : 'courier'
  const first = addresses.data?.data[0]?.id
  return (
    <button
      className="btn ghost"
      disabled={next === 'courier' && !first}
      onClick={() => onChange(next === 'pickup' ? { delivery: 'pickup' } : { delivery: 'courier', address_id: first! })}
      title="PATCH /orders/{id} changes only the fields sent"
    >
      Switch to {next}
    </button>
  )
}

function RefundBox({ order, refundable, onDone }: { order: Order; refundable: number; onDone: () => void }) {
  const { t, lang } = useI18n()
  const toast = useToast()
  const [amount, setAmount] = useState(Math.min(50_000, refundable))
  const [reason, setReason] = useState('')
  const [error, setError] = useState<ApiError | null>(null)
  const [key, setKey] = useState(newIdempotencyKey)
  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      const r = await post<Refund>(`/orders/${order.id}/refunds`, { amount, reason }, { idempotencyKey: key })
      toast(r.auto_approved ? `Refunded ${money(r.amount, lang)} at once` : 'Sent to a pharmacist for review', r.auto_approved ? 'ok' : 'info')
      setKey(newIdempotencyKey())
      setReason('')
      onDone()
    } catch (err) {
      setError(err as ApiError)
    }
  }
  return (
    <form className="card stack" style={{ '--gap': '14px' } as React.CSSProperties} onSubmit={submit}>
      <div className="spread">
        <h3>{t('order.refund')}</h3>
        <span className="small muted">{money(refundable, lang)} refundable</span>
      </div>
      <input type="range" min={1000} max={Math.max(refundable, 1000)} step={1000} value={amount} onChange={(e) => setAmount(Number(e.target.value))} style={{ width: '100%', accentColor: 'var(--ink)' }} aria-label="Amount" />
      <ThresholdMeter amount={amount} max={refundable} />
      <div className="row" style={{ flexWrap: 'nowrap' }}>
        <Field label="Amount (UZS)" error={error?.fieldError('amount')}>
          <input className="input mono" inputMode="numeric" value={amount} onChange={(e) => setAmount(Number(e.target.value.replace(/\D/g, '')) || 0)} />
        </Field>
        <Field label="Reason" error={error?.fieldError('reason')}>
          <input className="input" value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Pack arrived damaged" />
        </Field>
      </div>
      <ErrorNote error={error && !error.body.fields ? error : null} />
      <button className="btn coral" disabled={amount <= 0}>
        Ask for {money(amount, lang)}
      </button>
    </form>
  )
}
