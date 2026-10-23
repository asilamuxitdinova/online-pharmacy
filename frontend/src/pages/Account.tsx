import { AnimatePresence, motion } from 'motion/react'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { brandOf, formatCardNumber, PaymentCard } from '../components/PaymentCard'
import { PillIcon } from '../components/PillIcon'
import { Empty, ErrorNote, Field, Loading, Page, StatusBadge, Tabs } from '../components/ui'
import { ApiError, del, get, patch, post } from '../lib/api'
import { useAuth } from '../lib/auth'
import { day, money, pillColors, when } from '../lib/format'
import { useI18n } from '../lib/i18n'
import { useToast } from '../lib/toast'
import type { Address, ApiKey, Card, Cursored, Order, OrderStatus, Prescription } from '../lib/types'
import { useAsync } from '../lib/useAsync'

type Tab = 'orders' | 'prescriptions' | 'cards' | 'addresses' | 'keys' | 'profile'

export default function Account() {
  const { t } = useI18n()
  const { me } = useAuth()
  const [params, setParams] = useSearchParams()
  const tab = (params.get('tab') as Tab) || 'orders'
  return (
    <Page>
      <div className="stack" style={{ '--gap': '24px' } as React.CSSProperties}>
        <div className="stack" style={{ '--gap': '6px' } as React.CSSProperties}>
          <span className="kicker">{me?.email}</span>
          <h1 style={{ fontSize: 'clamp(36px, 5vw, 60px)' }}>
            {t('nav.account')}
            {me && <span className="muted">, {me.name.split(' ')[0]}</span>}
          </h1>
        </div>
        <Tabs<Tab>
          value={tab}
          onChange={(v) => setParams({ tab: v }, { replace: true })}
          items={[
            { id: 'orders', label: t('account.orders') },
            { id: 'prescriptions', label: t('account.prescriptions') },
            { id: 'cards', label: t('account.cards') },
            { id: 'addresses', label: t('account.addresses') },
            { id: 'keys', label: t('account.keys') },
            { id: 'profile', label: t('account.profile') },
          ]}
        />
        <AnimatePresence mode="wait">
          <motion.div key={tab} initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -8 }} transition={{ duration: 0.2 }}>
            {tab === 'orders' && <Orders />}
            {tab === 'prescriptions' && <Prescriptions />}
            {tab === 'cards' && <Cards />}
            {tab === 'addresses' && <Addresses />}
            {tab === 'keys' && <Keys />}
            {tab === 'profile' && <Profile />}
          </motion.div>
        </AnimatePresence>
      </div>
    </Page>
  )
}

// ---------------------------------------------------------------- orders (cursor paging)

function Orders() {
  const { lang } = useI18n()
  const [status, setStatus] = useState<OrderStatus | ''>('')
  const [extra, setExtra] = useState<Order[]>([])
  const [cursor, setCursor] = useState<string | null>(null)
  const [more, setMore] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const first = useAsync(
    async (signal) => {
      const r = await get<Cursored<Order>>('/orders', { limit: 6, status }, { signal })
      setExtra([])
      setCursor(r.next_cursor)
      setMore(r.has_more)
      return r
    },
    [status, lang],
  )
  const loadMore = async () => {
    if (!cursor) return
    setLoadingMore(true)
    try {
      const r = await get<Cursored<Order>>('/orders', { limit: 6, status, cursor })
      setExtra((x) => [...x, ...r.data])
      setCursor(r.next_cursor)
      setMore(r.has_more)
    } finally {
      setLoadingMore(false)
    }
  }
  const orders = [...(first.data?.data ?? []), ...extra]
  const statuses: (OrderStatus | '')[] = ['', 'pending_payment', 'paid', 'partially_refunded', 'refunded', 'cancelled']

  return (
    <div className="stack" style={{ '--gap': '16px' } as React.CSSProperties}>
      <div className="row">
        {statuses.map((s) => (
          <button key={s || 'all'} className={`chip ${status === s ? 'on' : ''}`} onClick={() => setStatus(s)}>
            {s ? <StatusBadge status={s} /> : 'All'}
          </button>
        ))}
      </div>
      <ErrorNote error={first.error} onRetry={first.reload} />
      {first.loading && !first.data && <Loading />}
      {first.data && orders.length === 0 && <Empty title="No orders here yet" art={<PillIcon form="sachets" a="var(--sun)" b="var(--bg-2)" size={90} />} />}
      <div className="stack" style={{ '--gap': '12px' } as React.CSSProperties}>
        <AnimatePresence initial={false}>
          {orders.map((o, i) => (
            <motion.div key={o.id} layout initial={{ opacity: 0, y: 16 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: (i % 6) * 0.04 }}>
              <Link to={`/orders/${o.id}`} className="card tight row" style={{ textDecoration: 'none', flexWrap: 'nowrap', '--gap': '16px' } as React.CSSProperties}>
                <div className="row hide-sm" style={{ '--gap': '0', flexShrink: 0 } as React.CSSProperties}>
                  {o.items.slice(0, 3).map((it, j) => {
                    const [a, b] = pillColors(it.product_id)
                    return (
                      <div key={it.product_id} style={{ marginLeft: j ? -18 : 0, background: b, borderRadius: 14, border: '2px solid var(--card)' }}>
                        <PillIcon form="capsules" a={a} b={b} size={44} />
                      </div>
                    )
                  })}
                </div>
                <div className="grow stack" style={{ '--gap': '2px' } as React.CSSProperties}>
                  <div className="row" style={{ '--gap': '8px' } as React.CSSProperties}>
                    <b className="mono small">{o.id}</b>
                    <StatusBadge status={o.status} />
                  </div>
                  <span className="small muted" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {o.items.map((i) => i.name.split(',')[0]).join(' · ')}
                  </span>
                  <span className="tiny muted">{when(o.created_at, lang)}</span>
                </div>
                <span className="price" style={{ fontSize: 18 }}>
                  {money(o.total, lang)}
                </span>
              </Link>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
      {more && (
        <button className="btn ghost" style={{ alignSelf: 'center' }} onClick={loadMore} disabled={loadingMore}>
          {loadingMore ? '…' : 'Load more'}
        </button>
      )}
    </div>
  )
}

// ---------------------------------------------------------------- prescriptions

function Prescriptions() {
  const { lang } = useI18n()
  const toast = useToast()
  const list = useAsync((s) => get<{ data: Prescription[] }>('/prescriptions', undefined, { signal: s }), [])
  const [f, setF] = useState({ doctor_name: '', clinic: '', issued_on: new Date().toISOString().slice(0, 10), diagnosis: '' })
  const [error, setError] = useState<ApiError | null>(null)
  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await post('/prescriptions', f)
      toast('Uploaded. A pharmacist will review it.', 'ok')
      setF({ ...f, doctor_name: '', clinic: '', diagnosis: '' })
      list.reload()
    } catch (err) {
      setError(err as ApiError)
    }
  }
  return (
    <div className="grid-side">
      <div className="stack" style={{ '--gap': '12px' } as React.CSSProperties}>
        {list.data?.data.length === 0 && <Empty title="No prescriptions yet" />}
        {list.data?.data.map((p, i) => (
          <motion.div key={p.id} className="card tight" style={{ position: 'relative', overflow: 'hidden' }} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: i * 0.05 }}>
            <div className="spread">
              <b>{p.doctor_name}</b>
              <StatusBadge status={p.status} />
            </div>
            <div className="small muted">
              {p.clinic} · issued {day(p.issued_on, lang)}
            </div>
            <div className="small" style={{ marginTop: 6 }}>{p.diagnosis}</div>
            {p.review_note && <div className="small muted" style={{ marginTop: 4 }}>“{p.review_note}”</div>}
            <div className="tiny mono muted" style={{ marginTop: 6 }}>{p.id}</div>
            {p.status !== 'pending_review' && (
              <motion.div
                initial={{ scale: 2.4, opacity: 0, rotate: -30 }}
                animate={{ scale: 1, opacity: 0.85, rotate: -14 }}
                transition={{ type: 'spring', stiffness: 260, damping: 14, delay: 0.2 + i * 0.05 }}
                style={{
                  position: 'absolute', right: 14, bottom: 12, padding: '4px 12px', border: '3px solid', borderRadius: 8,
                  fontWeight: 900, letterSpacing: '.12em', textTransform: 'uppercase', fontSize: 13,
                  color: p.status === 'approved' ? 'var(--mint-ink)' : 'var(--bad)',
                }}
              >
                {p.status}
              </motion.div>
            )}
          </motion.div>
        ))}
      </div>
      <form className="card stack sticky" style={{ '--gap': '12px' } as React.CSSProperties} onSubmit={submit}>
        <h3>Upload a prescription</h3>
        <Field label="Doctor" error={error?.fieldError('doctor_name')}>
          <input className="input" value={f.doctor_name} onChange={(e) => setF({ ...f, doctor_name: e.target.value })} />
        </Field>
        <Field label="Clinic" error={error?.fieldError('clinic')}>
          <input className="input" value={f.clinic} onChange={(e) => setF({ ...f, clinic: e.target.value })} />
        </Field>
        <Field label="Issued on" error={error?.fieldError('issued_on')} hint="Valid for one year, never in the future.">
          <input className="input" type="date" value={f.issued_on} onChange={(e) => setF({ ...f, issued_on: e.target.value })} />
        </Field>
        <Field label="Diagnosis" error={error?.fieldError('diagnosis')}>
          <textarea className="textarea" value={f.diagnosis} onChange={(e) => setF({ ...f, diagnosis: e.target.value })} />
        </Field>
        <ErrorNote error={error && !error.body.fields ? error : null} />
        <button className="btn coral">Upload</button>
      </form>
    </div>
  )
}

// ---------------------------------------------------------------- cards

const skin: Record<string, string> = {
  visa: 'linear-gradient(135deg, #1a3a8f, #7aa2ff)',
  mastercard: 'linear-gradient(135deg, #2a1a12, #ffb36b)',
  uzcard: 'linear-gradient(135deg, #0b4a3a, #7ff0c9)',
  humo: 'linear-gradient(135deg, #3a1a6b, #f0a6ff)',
}

function Cards() {
  const toast = useToast()
  const list = useAsync((s) => get<{ data: Card[] }>('/me/cards', undefined, { signal: s }), [])
  const [f, setF] = useState({ number: '', exp: '', cvc: '' })
  const [flip, setFlip] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const add = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    const [mm, yy] = f.exp.split('/')
    try {
      const c = await post<Card>('/me/cards', { number: f.number.replace(/\s/g, ''), exp_month: Number(mm), exp_year: 2000 + Number(yy), cvc: f.cvc })
      toast(`Saved ${c.brand} •••• ${c.last4}. The full number was not kept.`, 'ok')
      setF({ number: '', exp: '', cvc: '' })
      list.reload()
    } catch (err) {
      setError(err as ApiError)
    }
  }
  return (
    <div className="grid-side">
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: 16, alignContent: 'start' }}>
        <AnimatePresence>
          {list.data?.data.map((c) => (
            <motion.div
              key={c.id}
              layout
              initial={{ opacity: 0, rotateX: -40 }}
              animate={{ opacity: 1, rotateX: 0 }}
              exit={{ opacity: 0, scale: 0.8 }}
              whileHover={{ y: -6, rotate: -1.5 }}
              style={{ aspectRatio: '1.586', borderRadius: 16, padding: 18, color: '#fff', background: skin[c.brand], display: 'flex', flexDirection: 'column', justifyContent: 'space-between', boxShadow: 'var(--shadow-lg)' }}
            >
              <div className="spread">
                <b style={{ textTransform: 'uppercase', fontStyle: 'italic' }}>{c.brand}</b>
                <button
                  className="btn sm icon"
                  style={{ background: 'rgba(255,255,255,.18)', borderColor: 'transparent', color: '#fff' }}
                  aria-label="Remove card"
                  onClick={async () => {
                    await del(`/me/cards/${c.id}`)
                    list.reload()
                  }}
                >
                  ✕
                </button>
              </div>
              <div className="mono" style={{ fontSize: 18, letterSpacing: '.14em' }}>
                •••• •••• •••• {c.last4}
              </div>
              <div className="spread tiny mono" style={{ opacity: 0.85 }}>
                <span>{c.id}</span>
                <span>
                  {String(c.exp_month).padStart(2, '0')}/{String(c.exp_year).slice(2)}
                </span>
              </div>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
      <form className="card stack sticky" style={{ '--gap': '14px' } as React.CSSProperties} onSubmit={add}>
        <h3>Add a card</h3>
        <PaymentCard number={f.number} name="" exp={f.exp} cvc={f.cvc} flipped={flip} />
        <Field label={`Number ${brandOf(f.number) ? '· ' + brandOf(f.number) : ''}`} error={error?.fieldError('number')}>
          <input className="input mono" value={f.number} onChange={(e) => setF({ ...f, number: formatCardNumber(e.target.value) })} />
        </Field>
        <div className="row" style={{ flexWrap: 'nowrap' }}>
          <Field label="MM/YY" error={error?.fieldError('exp_month') ?? error?.fieldError('exp_year')}>
            <input className="input mono" value={f.exp} onChange={(e) => setF({ ...f, exp: e.target.value.replace(/\D/g, '').slice(0, 4).replace(/^(\d\d)(\d)/, '$1/$2') })} />
          </Field>
          <Field label="CVC" error={error?.fieldError('cvc')}>
            <input className="input mono" value={f.cvc} onFocus={() => setFlip(true)} onBlur={() => setFlip(false)} onChange={(e) => setF({ ...f, cvc: e.target.value.replace(/\D/g, '').slice(0, 4) })} />
          </Field>
        </div>
        <ErrorNote error={error && !error.body.fields ? error : null} />
        <button className="btn coral">Save card</button>
      </form>
    </div>
  )
}

// ---------------------------------------------------------------- addresses

const CITIES = ['Tashkent', 'Samarkand', 'Bukhara', 'Andijan', 'Namangan', 'Fergana', 'Nukus', 'Khiva']

function Addresses() {
  const list = useAsync((s) => get<{ data: Address[] }>('/me/addresses', undefined, { signal: s }), [])
  const [f, setF] = useState({ label: 'Home', line1: '', city: 'Tashkent', phone: '+998' })
  const [error, setError] = useState<ApiError | null>(null)
  const [delError, setDelError] = useState<ApiError | null>(null)
  const add = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await post('/me/addresses', f)
      setF({ ...f, line1: '' })
      list.reload()
    } catch (err) {
      setError(err as ApiError)
    }
  }
  return (
    <div className="grid-side">
      <div className="stack" style={{ '--gap': '12px' } as React.CSSProperties}>
        <ErrorNote error={delError} />
        <AnimatePresence>
          {list.data?.data.map((a) => (
            <motion.div key={a.id} layout className="card tight spread" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0, x: 40 }}>
              <div>
                <b>📍 {a.label}</b>
                <div className="small">
                  {a.line1}, {a.city}
                </div>
                <div className="tiny muted">{a.phone}</div>
              </div>
              <button
                className="btn ghost sm"
                onClick={async () => {
                  setDelError(null)
                  try {
                    await del(`/me/addresses/${a.id}`)
                    list.reload()
                  } catch (err) {
                    setDelError(err as ApiError)
                  }
                }}
              >
                Delete
              </button>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
      <form className="card stack sticky" style={{ '--gap': '12px' } as React.CSSProperties} onSubmit={add}>
        <h3>Add an address</h3>
        <Field label="Label" error={error?.fieldError('label')}>
          <input className="input" value={f.label} onChange={(e) => setF({ ...f, label: e.target.value })} />
        </Field>
        <Field label="Street and number" error={error?.fieldError('line1')}>
          <input className="input" value={f.line1} onChange={(e) => setF({ ...f, line1: e.target.value })} />
        </Field>
        <Field label="City" error={error?.fieldError('city')}>
          <select className="select" value={f.city} onChange={(e) => setF({ ...f, city: e.target.value })}>
            {CITIES.map((c) => (
              <option key={c}>{c}</option>
            ))}
          </select>
        </Field>
        <Field label="Phone" error={error?.fieldError('phone')}>
          <input className="input" value={f.phone} onChange={(e) => setF({ ...f, phone: e.target.value })} />
        </Field>
        <button className="btn coral">Add</button>
      </form>
    </div>
  )
}

// ---------------------------------------------------------------- API keys

function Keys() {
  const { lang } = useI18n()
  const list = useAsync((s) => get<{ data: ApiKey[] }>('/me/api-keys', undefined, { signal: s }), [])
  const [name, setName] = useState('')
  const [fresh, setFresh] = useState<ApiKey | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      setFresh(await post<ApiKey>('/me/api-keys', { name }))
      setName('')
      list.reload()
    } catch (err) {
      setError(err as ApiError)
    }
  }
  return (
    <div className="grid-side">
      <div className="card">
        <div className="table-scroll">
          <table className="grid">
            <thead>
              <tr>
                <th>Name</th>
                <th>Prefix</th>
                <th>Created</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {list.data?.data.map((k) => (
                <tr key={k.id} style={{ opacity: k.revoked_at ? 0.45 : 1 }}>
                  <td>{k.name}</td>
                  <td className="mono">{k.prefix}…</td>
                  <td className="small">{when(k.created_at, lang)}</td>
                  <td style={{ textAlign: 'right' }}>
                    {k.revoked_at ? (
                      <span className="badge bad">revoked</span>
                    ) : (
                      <button
                        className="btn ghost sm"
                        onClick={async () => {
                          await del(`/me/api-keys/${k.id}`)
                          list.reload()
                        }}
                      >
                        Revoke
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      <form className="card stack sticky" style={{ '--gap': '12px' } as React.CSSProperties} onSubmit={create}>
        <h3>New key</h3>
        <Field label="What is it for?" error={error?.fieldError('name')}>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="My integration" />
        </Field>
        <button className="btn coral">Create</button>
        <AnimatePresence>
          {fresh?.key && (
            <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0 }} className="card tight" style={{ background: 'var(--warn-bg)' }}>
              <div className="small" style={{ fontWeight: 700 }}>Shown once. Copy it now.</div>
              <code style={{ wordBreak: 'break-all' }}>{fresh.key}</code>
              <button type="button" className="btn sm" style={{ marginTop: 8 }} onClick={() => void navigator.clipboard.writeText(fresh.key!)}>
                Copy
              </button>
            </motion.div>
          )}
        </AnimatePresence>
      </form>
    </div>
  )
}

// ---------------------------------------------------------------- profile

function Profile() {
  const { me, refreshMe, signOut, creds } = useAuth()
  const toast = useToast()
  const [name, setName] = useState(me?.name ?? '')
  const [phone, setPhone] = useState(me?.phone ?? '')
  const [error, setError] = useState<ApiError | null>(null)
  if (!me) return null
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await patch('/me', { name, phone: phone || undefined })
      await refreshMe()
      toast('Saved', 'ok')
    } catch (err) {
      setError(err as ApiError)
    }
  }
  return (
    <div className="grid-2">
      <form className="card stack" style={{ '--gap': '14px' } as React.CSSProperties} onSubmit={save}>
        <h3>About you</h3>
        <Field label="Name" error={error?.fieldError('name')}>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label="Phone" error={error?.fieldError('phone')}>
          <input className="input" value={phone} onChange={(e) => setPhone(e.target.value)} />
        </Field>
        <ErrorNote error={error && !error.body.fields ? error : null} />
        <button className="btn coral">Save</button>
      </form>
      <div className="card stack" style={{ '--gap': '12px' } as React.CSSProperties}>
        <h3>Security</h3>
        <p className="small muted">Signed in as {me.email}. Signing out everywhere ends your sessions on every device.</p>
        <div className="row">
          <button className="btn ghost sm" onClick={() => void signOut(false)}>Sign out</button>
          <button className="btn danger sm" onClick={() => void signOut(true)} disabled={creds?.kind === 'cookie'}>Sign out everywhere</button>
        </div>
      </div>
    </div>
  )
}
