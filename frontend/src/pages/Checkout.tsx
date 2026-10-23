import { AnimatePresence, motion, useAnimationControls } from 'motion/react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { DeliveryProgress } from '../components/CartDrawer'
import { formatCardNumber, PaymentCard } from '../components/PaymentCard'
import { PillIcon } from '../components/PillIcon'
import { Empty, ErrorNote, Field, Page } from '../components/ui'
import { ApiError, get, newIdempotencyKey, post } from '../lib/api'
import { useCart } from '../lib/cart'
import { money, pillColors } from '../lib/format'
import { useI18n } from '../lib/i18n'
import type { Address, Card, Order, Prescription } from '../lib/types'
import { useAsync } from '../lib/useAsync'

export default function Checkout() {
  const cart = useCart()
  const { t } = useI18n()
  const [order, setOrder] = useState<Order | null>(null)
  const [paid, setPaid] = useState(false)

  if (paid && order) return <PaidScreen order={order} />
  if (!order && cart.lines.length === 0) {
    return (
      <Page>
        <Empty title={t('cart.empty')} art={<PillIcon form="capsules" a="var(--coral)" b="var(--mint)" size={100} />} />
        <div className="center" style={{ marginTop: 16 }}>
          <Link to="/shop" className="btn coral">{t('hero.cta')}</Link>
        </div>
      </Page>
    )
  }

  return (
    <Page>
      <div className="stack" style={{ '--gap': '8px', marginBottom: 24 } as React.CSSProperties}>
        <h1 style={{ fontSize: 'clamp(36px, 5vw, 60px)' }}>{t('checkout.title')}</h1>
        <Steps step={order ? 2 : 1} />
      </div>
      <AnimatePresence mode="wait">
        {!order ? (
          <motion.div key="place" initial={{ opacity: 0, x: -20 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -40 }}>
            <PlaceOrder onPlaced={setOrder} />
          </motion.div>
        ) : (
          <motion.div key="pay" initial={{ opacity: 0, x: 40 }} animate={{ opacity: 1, x: 0 }}>
            <Pay
              order={order}
              onPaid={(o) => {
                setOrder(o)
                setPaid(true)
                cart.clear()
              }}
            />
          </motion.div>
        )}
      </AnimatePresence>
    </Page>
  )
}

function Steps({ step }: { step: number }) {
  return (
    <div className="row small" style={{ fontWeight: 700 }}>
      {['Order', 'Pay', 'Done'].map((s, i) => (
        <span key={s} className="row" style={{ '--gap': '8px' } as React.CSSProperties}>
          <motion.span
            animate={{ background: i + 1 <= step ? 'var(--ink)' : 'var(--bg-2)', color: i + 1 <= step ? 'var(--bg)' : 'var(--muted)' }}
            style={{ width: 26, height: 26, borderRadius: 13, display: 'grid', placeItems: 'center', fontSize: 12 }}
          >
            {i + 1}
          </motion.span>
          <span className={i + 1 <= step ? '' : 'muted'}>{s}</span>
          {i < 2 && <span className="muted">—</span>}
        </span>
      ))}
    </div>
  )
}

function PlaceOrder({ onPlaced }: { onPlaced: (o: Order) => void }) {
  const cart = useCart()
  const { t, lang } = useI18n()
  const addresses = useAsync((s) => get<{ data: Address[] }>('/me/addresses', undefined, { signal: s }), [])
  const rx = useAsync((s) => get<{ data: Prescription[] }>('/prescriptions', undefined, { signal: s }), [])
  const [delivery, setDelivery] = useState<'courier' | 'pickup'>('courier')
  const [addressId, setAddressId] = useState('')
  const [prescriptionId, setPrescriptionId] = useState('')
  const [note, setNote] = useState('')
  const [error, setError] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState(false)
  // One key per attempt: a double click or a retry after a timeout cannot create two orders.
  const [idemKey, setIdemKey] = useState(newIdempotencyKey)

  const chosenAddress = addressId || addresses.data?.data[0]?.id || ''
  const approved = rx.data?.data.filter((p) => p.status === 'approved') ?? []
  const fee = delivery === 'pickup' || cart.subtotal >= 300_000 ? 0 : 25_000

  const place = async () => {
    setBusy(true)
    setError(null)
    try {
      const body: Record<string, unknown> = {
        items: cart.lines.map((l) => ({ product_id: l.product.id, quantity: l.quantity })),
        delivery,
      }
      if (delivery === 'courier' && chosenAddress) body.address_id = chosenAddress
      if (prescriptionId) body.prescription_id = prescriptionId
      if (note.trim()) body.note = note.trim()
      onPlaced(await post<Order>('/orders', body, { idempotencyKey: idemKey }))
    } catch (e) {
      const err = e as ApiError
      setError(err)
      // A validation error means the body changes next time: use a fresh key.
      if (err.status === 422 || err.status === 409) setIdemKey(newIdempotencyKey())
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid-side">
      <div className="stack" style={{ '--gap': '18px' } as React.CSSProperties}>
        <div className="card stack" style={{ '--gap': '14px' } as React.CSSProperties}>
          <h3>{t('checkout.delivery')}</h3>
          <div className="row">
            {(['courier', 'pickup'] as const).map((d) => (
              <motion.button key={d} className={`chip ${delivery === d ? 'on' : ''}`} style={{ height: 52, padding: '0 20px' }} onClick={() => setDelivery(d)} whileTap={{ scale: 0.95 }}>
                {d === 'courier' ? '🛵 ' + t('checkout.courier') : '🏪 ' + t('checkout.pickup')}
              </motion.button>
            ))}
          </div>
          <AnimatePresence>
            {delivery === 'courier' && (
              <motion.div initial={{ height: 0, opacity: 0 }} animate={{ height: 'auto', opacity: 1 }} exit={{ height: 0, opacity: 0 }} style={{ overflow: 'hidden' }}>
                <Field label={t('checkout.address')} error={error?.fieldError('address_id')} hint={<Link to="/account?tab=addresses">Manage addresses</Link>}>
                  <select className="select" value={chosenAddress} onChange={(e) => setAddressId(e.target.value)}>
                    {addresses.data?.data.length === 0 && <option value="">No saved address</option>}
                    {addresses.data?.data.map((a) => (
                      <option key={a.id} value={a.id}>
                        {a.label} · {a.line1}, {a.city}
                      </option>
                    ))}
                  </select>
                </Field>
              </motion.div>
            )}
          </AnimatePresence>
          <DeliveryProgress subtotal={cart.subtotal} />
        </div>

        <div className="card stack" style={{ '--gap': '14px' } as React.CSSProperties}>
          <div className="spread">
            <h3>{t('checkout.prescription')}</h3>
            {cart.needsPrescription && <span className="badge rx">required</span>}
          </div>
          <Field label="Approved prescription" error={error?.fieldError('prescription_id')} hint={cart.needsPrescription ? 'Your basket holds prescription-only medicine.' : 'Not needed for this basket.'}>
            <select className="select" value={prescriptionId} onChange={(e) => setPrescriptionId(e.target.value)}>
              <option value="">None</option>
              {rx.data?.data.map((p) => (
                <option key={p.id} value={p.id} disabled={p.status !== 'approved'}>
                  {p.doctor_name} · {p.issued_on} · {p.status}
                </option>
              ))}
            </select>
          </Field>
          {cart.needsPrescription && approved.length === 0 && (
            <p className="small muted">
              You have no approved prescription yet. <Link to="/account?tab=prescriptions">Upload one</Link>; a pharmacist reviews it.
            </p>
          )}
          <Field label="Note for the courier" error={error?.fieldError('note')}>
            <input className="input" value={note} onChange={(e) => setNote(e.target.value)} maxLength={600} placeholder="Ring twice, please" />
          </Field>
        </div>
        <ErrorNote error={error} />
      </div>

      <aside className="card stack sticky" style={{ '--gap': '12px' } as React.CSSProperties}>
        <h3>{t('cart.title')}</h3>
        {cart.lines.map((l) => {
          const [a, b] = pillColors(l.product.id)
          return (
            <div key={l.product.id} className="row small" style={{ flexWrap: 'nowrap' }}>
              <PillIcon form={l.product.form} a={a} b={b} size={34} />
              <span className="grow">
                {l.quantity} × {l.product.name}
              </span>
              <b>{money(l.product.price * l.quantity, lang)}</b>
            </div>
          )
        })}
        <hr className="divider" />
        <div className="spread small">
          <span className="muted">{t('cart.subtotal')}</span>
          <span>{money(cart.subtotal, lang)}</span>
        </div>
        <div className="spread small">
          <span className="muted">{t('checkout.delivery')}</span>
          <span>{fee ? money(fee, lang) : 'free'}</span>
        </div>
        <div className="spread">
          <b>Total</b>
          <span className="price">{money(cart.subtotal + fee, lang)}</span>
        </div>
        <button className="btn coral" onClick={place} disabled={busy}>
          {busy ? '…' : t('checkout.place')}
        </button>
      </aside>
    </div>
  )
}

export function Pay({ order, onPaid }: { order: Order; onPaid: (o: Order) => void }) {
  const { t, lang } = useI18n()
  const cards = useAsync((s) => get<{ data: Card[] }>('/me/cards', undefined, { signal: s }), [])
  const [mode, setMode] = useState<'saved' | 'new'>('saved')
  const [cardId, setCardId] = useState('')
  const [number, setNumber] = useState('')
  const [name, setName] = useState('')
  const [exp, setExp] = useState('')
  const [cvc, setCvc] = useState('')
  const [flip, setFlip] = useState(false)
  const [confirmed, setConfirmed] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState(false)
  const shake = useAnimationControls()
  const idemKey = useMemo(newIdempotencyKey, [mode, cardId, number])

  const savedId = cardId || cards.data?.data[0]?.id || ''

  const pay = async () => {
    setBusy(true)
    setError(null)
    try {
      const [mm, yy] = exp.split('/')
      const body =
        mode === 'saved'
          ? { card_id: savedId }
          : { card: { number: number.replace(/\s/g, ''), exp_month: Number(mm), exp_year: 2000 + Number(yy), cvc } }
      const r = await post<{ order: Order }>(`/orders/${order.id}/pay`, body, { idempotencyKey: idemKey })
      onPaid(r.order)
    } catch (e) {
      setError(e as ApiError)
      void shake.start({ x: [0, -12, 12, -8, 8, 0], transition: { duration: 0.45 } })
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid-side">
      <motion.div className="card stack" style={{ '--gap': '18px' } as React.CSSProperties} animate={shake}>
        <div className="spread">
          <h3>{t('checkout.payWith')}</h3>
          <div className="tabs">
            {(['saved', 'new'] as const).map((m) => (
              <button key={m} className={`tab ${mode === m ? 'on' : ''}`} onClick={() => setMode(m)}>
                {mode === m && <motion.div layoutId="paymode" className="tab-pill" />}
                <span>{m === 'saved' ? t('checkout.savedCard') : t('checkout.newCard')}</span>
              </button>
            ))}
          </div>
        </div>

        {mode === 'saved' ? (
          <div className="stack" style={{ '--gap': '10px' } as React.CSSProperties}>
            {cards.data?.data.length === 0 && <p className="muted small">No saved cards. Use a new card.</p>}
            {cards.data?.data.map((c) => (
              <label key={c.id} className={`card tight row ${savedId === c.id ? '' : 'flat'}`} style={{ cursor: 'pointer', borderColor: savedId === c.id ? 'var(--ink)' : undefined }}>
                <input type="radio" name="card" checked={savedId === c.id} onChange={() => setCardId(c.id)} />
                <b style={{ textTransform: 'capitalize' }}>{c.brand}</b>
                <span className="mono">•••• {c.last4}</span>
                <span className="muted small">
                  {String(c.exp_month).padStart(2, '0')}/{c.exp_year}
                </span>
              </label>
            ))}
          </div>
        ) : (
          <div className="stack" style={{ '--gap': '16px' } as React.CSSProperties}>
            <PaymentCard number={number} name={name} exp={exp} cvc={cvc} flipped={flip} />
            <Field label="Card number" error={error?.fieldError('card.number')}>
              <input className="input mono" inputMode="numeric" autoComplete="off" value={number} onChange={(e) => setNumber(formatCardNumber(e.target.value))} placeholder="0000 0000 0000 0000" />
            </Field>
            <div className="grid-2" style={{ gap: 12 }}>
              <Field label="Name on card">
                <input className="input" value={name} onChange={(e) => setName(e.target.value)} />
              </Field>
              <div className="row" style={{ flexWrap: 'nowrap' }}>
                <Field label="Expires" error={error?.fieldError('card.exp_month') ?? error?.fieldError('card.exp_year')}>
                  <input className="input mono" value={exp} placeholder="MM/YY" onChange={(e) => setExp(e.target.value.replace(/[^\d]/g, '').slice(0, 4).replace(/^(\d\d)(\d)/, '$1/$2'))} />
                </Field>
                <Field label="CVC" error={error?.fieldError('card.cvc')}>
                  <input className="input mono" value={cvc} onFocus={() => setFlip(true)} onBlur={() => setFlip(false)} onChange={(e) => setCvc(e.target.value.replace(/\D/g, '').slice(0, 4))} />
                </Field>
              </div>
            </div>
          </div>
        )}
        <ErrorNote error={error} />
      </motion.div>

      <aside className="card stack sticky" style={{ '--gap': '14px' } as React.CSSProperties}>
        <div className="spread">
          <h3>Order</h3>
          <span className="mono tiny">{order.id}</span>
        </div>
        {order.items.map((it) => (
          <div key={it.product_id} className="spread small">
            <span>
              {it.quantity} × {it.name}
            </span>
            <span>{money(it.line_total, lang)}</span>
          </div>
        ))}
        <hr className="divider" />
        <div className="spread small">
          <span className="muted">Delivery ({order.delivery})</span>
          <span>{order.delivery_fee ? money(order.delivery_fee, lang) : 'free'}</span>
        </div>
        <div className="spread">
          <b>Total</b>
          <span className="price">{money(order.total, lang)}</span>
        </div>
        <label className="check small" style={{ alignItems: 'flex-start' }}>
          <input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} />
          <span>
            {t('checkout.confirm')} <b>{money(order.total, lang)}</b>
          </span>
        </label>
        <motion.button className="btn coral" onClick={pay} disabled={!confirmed || busy} whileTap={{ scale: 0.96 }}>
          {busy ? '…' : `${t('checkout.pay')} ${money(order.total, lang)}`}
        </motion.button>
        <p className="tiny muted">Your card is charged only after you confirm. Card numbers are never stored.</p>
      </aside>
    </div>
  )
}

function PaidScreen({ order }: { order: Order }) {
  const { t, lang } = useI18n()
  const bits = useMemo(
    () =>
      Array.from({ length: 36 }, (_, i) => ({
        x: (Math.random() - 0.5) * 900,
        y: -200 - Math.random() * 400,
        r: Math.random() * 720 - 360,
        form: (['capsules', 'tablets', 'capsules', 'sachets'] as const)[i % 4],
        colors: pillColors(String(i * 97)),
        delay: Math.random() * 0.3,
      })),
    [],
  )
  return (
    <Page>
      <div style={{ position: 'relative', minHeight: '60vh', display: 'grid', placeItems: 'center' }}>
        <div style={{ position: 'absolute', left: '50%', top: '55%', pointerEvents: 'none' }} aria-hidden>
          {bits.map((b, i) => (
            <motion.div
              key={i}
              style={{ position: 'absolute' }}
              initial={{ x: 0, y: 0, rotate: 0, opacity: 1, scale: 0.4 }}
              animate={{ x: b.x, y: [0, b.y, b.y + 700], rotate: b.r, opacity: [1, 1, 0], scale: 1 }}
              transition={{ duration: 2.6, delay: b.delay, ease: [0.2, 0.7, 0.4, 1] }}
            >
              <PillIcon form={b.form} a={b.colors[0]} b={b.colors[1]} size={40} />
            </motion.div>
          ))}
        </div>
        <motion.div className="card center stack" style={{ '--gap': '14px', alignItems: 'center', maxWidth: 520 } as React.CSSProperties} initial={{ scale: 0.8, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} transition={{ type: 'spring', delay: 0.2 }}>
          <motion.div initial={{ scale: 0 }} animate={{ scale: 1 }} transition={{ type: 'spring', stiffness: 300, delay: 0.4 }} style={{ width: 72, height: 72, borderRadius: 36, background: 'var(--mint)', display: 'grid', placeItems: 'center', fontSize: 36 }}>
            ✓
          </motion.div>
          <h2>{t('checkout.paid')}</h2>
          <p className="muted">
            {money(order.total, lang)} · <span className="mono">{order.id}</span>
          </p>
          <div className="row" style={{ justifyContent: 'center' }}>
            <Link to={`/orders/${order.id}`} className="btn">
              See the order
            </Link>
            <Link to="/shop" className="btn ghost">
              {t('hero.cta')}
            </Link>
          </div>
        </motion.div>
      </div>
    </Page>
  )
}
