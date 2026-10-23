import { AnimatePresence, motion } from 'motion/react'
import { useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { ErrorNote, Field, Page, Tabs } from '../components/ui'
import { ApiError, get, post } from '../lib/api'
import { useAuth } from '../lib/auth'
import { useI18n } from '../lib/i18n'
import { startOAuth } from '../lib/pkce'
import type { InboxMessage, TokenSet } from '../lib/types'
import { useInterval } from '../lib/useAsync'

type Method = 'password' | 'otp' | 'oauth' | 'cookie' | 'apikey' | 'register'

const DEMO = [
  { name: 'Aziza', email: 'aziza@pharmacy.test', password: 'Customer123!', role: 'customer', color: 'var(--coral)' },
  { name: 'Bekzod', email: 'bekzod@pharmacy.test', password: 'Customer123!', role: 'customer', color: 'var(--sky)' },
  { name: 'Dilnoza', email: 'dilnoza@pharmacy.test', password: 'Staff123!', role: 'pharmacist', color: 'var(--mint)' },
  { name: 'Admin', email: 'admin@pharmacy.test', password: 'Admin123!', role: 'admin', color: 'var(--lilac)' },
  { name: 'Timur', email: 'timur@pharmacy.test', password: 'Customer123!', role: 'disabled', color: 'var(--line-2)' },
]

const methodInfo: Record<Method, string> = {
  password: 'POST /auth/login → access + refresh token',
  otp: 'POST /auth/otp/request, then /auth/otp/verify',
  oauth: 'Authorization code with PKCE (S256)',
  cookie: 'POST /auth/session → HttpOnly cookie, no token',
  apikey: 'A fixed key sent as X-API-Key on every call',
  register: 'POST /auth/register → account + tokens',
}

export default function Login() {
  const { t } = useI18n()
  const { adoptTokens, signInWithCookie, signInWithApiKey } = useAuth()
  const nav = useNavigate()
  const loc = useLocation()
  const from = (loc.state as { from?: string } | null)?.from ?? '/account'
  const [method, setMethod] = useState<Method>('password')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState(false)

  const done = () => nav(from === '/login' ? '/account' : from, { replace: true })

  const run = async (fn: () => Promise<void>) => {
    setBusy(true)
    setError(null)
    try {
      await fn()
      done()
    } catch (e) {
      setError(e as ApiError)
    } finally {
      setBusy(false)
    }
  }

  const signInPassword = (e = email, p = password) =>
    run(async () => {
      if (method === 'cookie') return signInWithCookie(e, p)
      await adoptTokens(await post<TokenSet>('/auth/login', { email: e, password: p }, { anonymous: true }), 'password')
    })

  return (
    <Page>
      <div className="grid-2" style={{ alignItems: 'start', gap: 40 }}>
        <div className="stack" style={{ '--gap': '18px', paddingTop: 20 } as React.CSSProperties}>
          <span className="kicker">Six ways in</span>
          <h1 style={{ fontSize: 'clamp(40px, 5.5vw, 72px)' }}>{t('nav.signin')}</h1>
          <p className="muted" style={{ maxWidth: 440 }}>
            Every sign-in style Frege has to handle, against the same pharmacy. Pick a demo person to skip the typing.
          </p>
          <div className="stack" style={{ '--gap': '10px' } as React.CSSProperties}>
            {DEMO.map((d, i) => (
              <motion.button
                key={d.email}
                className="card tight row"
                style={{ cursor: 'pointer', textAlign: 'left', border: '1px solid var(--line)', flexWrap: 'nowrap' }}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: i * 0.06 }}
                whileHover={{ x: 6 }}
                onClick={() => {
                  setEmail(d.email)
                  setPassword(d.password)
                  if (method === 'password' || method === 'cookie') void signInPassword(d.email, d.password)
                }}
              >
                <span style={{ width: 38, height: 38, borderRadius: 19, background: d.color, display: 'grid', placeItems: 'center', fontWeight: 800, color: '#12302a', flexShrink: 0 }}>{d.name[0]}</span>
                <span className="grow">
                  <b>{d.name}</b> <span className="muted small">· {d.role}</span>
                  <div className="tiny muted mono">{d.email}</div>
                </span>
                <span className="muted">→</span>
              </motion.button>
            ))}
          </div>
        </div>

        <motion.div className="card stack" style={{ '--gap': '18px' } as React.CSSProperties} layout>
          <Tabs<Method>
            value={method}
            onChange={(m) => {
              setMethod(m)
              setError(null)
            }}
            items={[
              { id: 'password', label: 'Password' },
              { id: 'otp', label: 'Code' },
              { id: 'oauth', label: 'OAuth' },
              { id: 'cookie', label: 'Cookie' },
              { id: 'apikey', label: 'API key' },
              { id: 'register', label: 'New' },
            ]}
          />
          <span className="tiny muted mono">{methodInfo[method]}</span>
          <AnimatePresence mode="wait">
            <motion.div key={method} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="stack" style={{ '--gap': '14px' } as React.CSSProperties}>
              {(method === 'password' || method === 'cookie') && (
                <form className="stack" style={{ '--gap': '14px' } as React.CSSProperties} onSubmit={(e) => { e.preventDefault(); void signInPassword() }}>
                  <Field label="Email" error={error?.fieldError('email')}>
                    <input className="input" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} />
                  </Field>
                  <Field label="Password" error={error?.fieldError('password')}>
                    <input className="input" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
                  </Field>
                  <Attempts error={error} />
                  <button className="btn coral" disabled={busy}>
                    {busy ? '…' : method === 'cookie' ? 'Sign in with a cookie' : t('nav.signin')}
                  </button>
                </form>
              )}
              {method === 'otp' && <OtpFlow onTokens={(tk) => run(() => adoptTokens(tk, 'one-time code'))} />}
              {method === 'oauth' && (
                <div className="stack" style={{ '--gap': '14px' } as React.CSSProperties}>
                  <OAuthDiagram />
                  <button className="btn" onClick={() => void startOAuth()}>
                    Continue to the pharmacy's sign-in page →
                  </button>
                  <p className="tiny muted">You leave this app, sign in on the pharmacy's own page, and come back with a code that is exchanged (with the PKCE verifier) for tokens.</p>
                </div>
              )}
              {method === 'apikey' && <ApiKeyForm busy={busy} onKey={(k) => run(() => signInWithApiKey(k))} />}
              {method === 'register' && <RegisterForm busy={busy} error={error} onSubmit={(body) => run(async () => adoptTokens(await post<TokenSet>('/auth/register', body, { anonymous: true }), 'register'))} />}
              <ErrorNote error={error} />
            </motion.div>
          </AnimatePresence>
        </motion.div>
      </div>
    </Page>
  )
}

/** Five dots that go out as wrong passwords pile up; a timer when locked. */
function Attempts({ error }: { error: ApiError | null }) {
  const left = error?.body.details?.attempts_left as number | undefined
  const [lockLeft, setLockLeft] = useState(0)
  useEffect(() => {
    if (error?.code === 'too_many_attempts' && error.body.retry_after) setLockLeft(error.body.retry_after)
  }, [error])
  useInterval(() => setLockLeft((s) => Math.max(0, s - 1)), 1000, lockLeft > 0)
  if (lockLeft > 0) {
    return (
      <motion.div className="card tight row" style={{ background: 'var(--bad-bg)', color: 'var(--bad)', fontWeight: 700 }} initial={{ scale: 0.9 }} animate={{ scale: 1 }}>
        🔒 Locked for {Math.floor(lockLeft / 60)}:{String(lockLeft % 60).padStart(2, '0')}
      </motion.div>
    )
  }
  if (left === undefined) return null
  return (
    <div className="row small" style={{ '--gap': '6px' } as React.CSSProperties}>
      {Array.from({ length: 5 }, (_, i) => (
        <motion.span key={i} animate={{ scale: i < left ? 1 : 0.6, background: i < left ? 'var(--mint)' : 'var(--bad)' }} style={{ width: 12, height: 12, borderRadius: 6 }} />
      ))}
      <span className="muted">{left} tries before a 5-minute lock</span>
    </div>
  )
}

function OtpFlow({ onTokens }: { onTokens: (t: TokenSet) => void }) {
  const [channel, setChannel] = useState<'email' | 'sms'>('email')
  const [dest, setDest] = useState('aziza@pharmacy.test')
  const [sent, setSent] = useState<{ destination_hint: string; expires_in: number } | null>(null)
  const [digits, setDigits] = useState(['', '', '', '', '', ''])
  const [error, setError] = useState<ApiError | null>(null)
  const [msg, setMsg] = useState<InboxMessage | null>(null)
  const refs = useRef<(HTMLInputElement | null)[]>([])

  const request = async () => {
    setError(null)
    try {
      setSent(await post('/auth/otp/request', { channel, destination: dest }, { anonymous: true }))
      setTimeout(() => refs.current[0]?.focus(), 100)
    } catch (e) {
      setError(e as ApiError)
    }
  }

  // The "phone" in the corner: the demo inbox, polled for this destination.
  useInterval(
    async () => {
      try {
        const r = await get<{ data: InboxMessage[] }>('/_test/inbox', { recipient: dest, kind: 'otp', limit: 1 }, { anonymous: true })
        if (r.data[0] && r.data[0].id !== msg?.id) setMsg(r.data[0])
      } catch {
        /* helpers may be off */
      }
    },
    1500,
    !!sent,
  )

  const verify = async (code: string) => {
    setError(null)
    try {
      onTokens(await post<TokenSet>('/auth/otp/verify', { destination: dest, code }, { anonymous: true }))
    } catch (e) {
      setError(e as ApiError)
      setDigits(['', '', '', '', '', ''])
      refs.current[0]?.focus()
    }
  }

  const type = (i: number, v: string) => {
    const clean = v.replace(/\D/g, '')
    if (clean.length > 1) {
      const next = clean.slice(0, 6).split('')
      setDigits([...next, ...Array(6 - next.length).fill('')])
      if (next.length === 6) void verify(next.join(''))
      return
    }
    const next = [...digits]
    next[i] = clean
    setDigits(next)
    if (clean && i < 5) refs.current[i + 1]?.focus()
    if (next.every((d) => d)) void verify(next.join(''))
  }

  const code = typeof msg?.meta.code === 'string' ? msg.meta.code : null

  return (
    <div className="stack" style={{ '--gap': '14px' } as React.CSSProperties}>
      {!sent ? (
        <>
          <div className="row">
            {(['email', 'sms'] as const).map((c) => (
              <button key={c} className={`chip ${channel === c ? 'on' : ''}`} onClick={() => { setChannel(c); setDest(c === 'email' ? 'aziza@pharmacy.test' : '+998900000001') }}>
                {c === 'email' ? '✉ Email' : '📱 SMS'}
              </button>
            ))}
          </div>
          <Field label={channel === 'email' ? 'Email' : 'Phone'} error={error?.fieldError('destination')}>
            <input className="input" value={dest} onChange={(e) => setDest(e.target.value)} />
          </Field>
          <button className="btn coral" onClick={request}>
            Send me a code
          </button>
        </>
      ) : (
        <>
          <p className="small">
            Sent to <b>{sent.destination_hint}</b>. It lives {Math.round(sent.expires_in / 60)} minutes.
          </p>
          <div className="row" style={{ '--gap': '8px', flexWrap: 'nowrap' } as React.CSSProperties}>
            {digits.map((d, i) => (
              <motion.input
                key={i}
                ref={(el) => {
                  refs.current[i] = el
                }}
                className="input mono"
                style={{ width: 48, height: 58, textAlign: 'center', fontSize: 24, padding: 0 }}
                inputMode="numeric"
                maxLength={6}
                value={d}
                animate={d ? { scale: [1.15, 1], borderColor: 'var(--ink)' } : {}}
                onChange={(e) => type(i, e.target.value)}
                onKeyDown={(e) => e.key === 'Backspace' && !d && i > 0 && refs.current[i - 1]?.focus()}
                aria-label={`Digit ${i + 1}`}
              />
            ))}
          </div>
          <button className="btn ghost sm" style={{ width: 'fit-content' }} onClick={() => { setSent(null); setMsg(null) }}>
            ← Different address
          </button>
        </>
      )}
      <AnimatePresence>
        {msg && (
          <motion.div
            key={msg.id}
            initial={{ y: -30, opacity: 0, scale: 0.9 }}
            animate={{ y: 0, opacity: 1, scale: 1 }}
            exit={{ opacity: 0 }}
            className="card tight"
            style={{ background: 'var(--ink)', color: 'var(--bg)', borderRadius: 18, cursor: code ? 'pointer' : 'default' }}
            onClick={() => code && type(0, code)}
            title={code ? 'Click to paste the code' : undefined}
          >
            <div className="spread tiny" style={{ opacity: 0.7 }}>
              <span>📬 Demo inbox · {msg.recipient}</span>
              <span>now</span>
            </div>
            <div style={{ fontWeight: 700, marginTop: 4 }}>{msg.subject}</div>
            {code ? <div className="mono" style={{ fontSize: 28, letterSpacing: '.3em', marginTop: 4 }}>{code}</div> : <div className="small">{msg.body}</div>}
          </motion.div>
        )}
      </AnimatePresence>
      <ErrorNote error={error} />
    </div>
  )
}

function OAuthDiagram() {
  const steps = ['This app', 'Pharmacy sign-in', 'Back with ?code', 'Token exchange']
  return (
    <div className="row" style={{ '--gap': '6px', flexWrap: 'nowrap', overflowX: 'auto' } as React.CSSProperties}>
      {steps.map((s, i) => (
        <span key={s} className="row" style={{ '--gap': '6px', flexWrap: 'nowrap' } as React.CSSProperties}>
          <motion.span className="badge plain" initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: i * 0.15 }}>
            {s}
          </motion.span>
          {i < steps.length - 1 && (
            <motion.span animate={{ x: [0, 4, 0] }} transition={{ repeat: Infinity, duration: 1.2, delay: i * 0.2 }}>
              →
            </motion.span>
          )}
        </span>
      ))}
    </div>
  )
}

function ApiKeyForm({ busy, onKey }: { busy: boolean; onKey: (k: string) => void }) {
  const [key, setKey] = useState('')
  const seeded = ['phk_test_customer_aziza_0000000000000001', 'phk_test_staff_dilnoza_00000000000000003', 'phk_test_admin_000000000000000000000004']
  return (
    <form className="stack" style={{ '--gap': '14px' } as React.CSSProperties} onSubmit={(e) => { e.preventDefault(); onKey(key.trim()) }}>
      <Field label="API key">
        <input className="input mono" value={key} onChange={(e) => setKey(e.target.value)} placeholder="phk_test_…" />
      </Field>
      <div className="row" style={{ '--gap': '6px' } as React.CSSProperties}>
        {seeded.map((k) => (
          <button type="button" key={k} className="chip mono" style={{ fontSize: 11, height: 28 }} onClick={() => setKey(k)}>
            {k.split('_').slice(2, 4).join(' ')}
          </button>
        ))}
      </div>
      <button className="btn coral" disabled={busy || !key}>
        Use this key
      </button>
    </form>
  )
}

function RegisterForm({ busy, error, onSubmit }: { busy: boolean; error: ApiError | null; onSubmit: (b: Record<string, string>) => void }) {
  const [f, setF] = useState({ name: '', email: '', password: '', phone: '' })
  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement>) => setF({ ...f, [k]: e.target.value })
  const checks = [
    { ok: f.password.length >= 8, label: '8+ characters' },
    { ok: /\d/.test(f.password), label: 'a digit' },
    { ok: /[A-Z]/.test(f.password), label: 'an uppercase letter' },
  ]
  return (
    <form
      className="stack"
      style={{ '--gap': '14px' } as React.CSSProperties}
      onSubmit={(e) => {
        e.preventDefault()
        const body: Record<string, string> = { name: f.name, email: f.email, password: f.password }
        if (f.phone) body.phone = f.phone
        onSubmit(body)
      }}
    >
      <Field label="Name" error={error?.fieldError('name')}>
        <input className="input" value={f.name} onChange={set('name')} />
      </Field>
      <Field label="Email" error={error?.fieldError('email')}>
        <input className="input" type="email" value={f.email} onChange={set('email')} />
      </Field>
      <Field label="Phone (optional)" error={error?.fieldError('phone')}>
        <input className="input" value={f.phone} onChange={set('phone')} placeholder="+998901234567" />
      </Field>
      <Field label="Password" error={error?.fieldError('password')}>
        <input className="input" type="password" value={f.password} onChange={set('password')} autoComplete="new-password" />
      </Field>
      <div className="row small" style={{ '--gap': '12px' } as React.CSSProperties}>
        {checks.map((c) => (
          <motion.span key={c.label} animate={{ color: c.ok ? 'var(--mint-ink)' : 'var(--muted)' }} style={{ fontWeight: 700 }}>
            {c.ok ? '✓' : '○'} {c.label}
          </motion.span>
        ))}
      </div>
      <button className="btn coral" disabled={busy}>
        Create account
      </button>
    </form>
  )
}
