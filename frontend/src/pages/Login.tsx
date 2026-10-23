import { AnimatePresence, motion } from 'motion/react'
import { useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { PillIcon } from '../components/PillIcon'
import { ErrorNote, Field, Page, Tabs } from '../components/ui'
import { ApiError, post } from '../lib/api'
import { useAuth } from '../lib/auth'
import { useI18n } from '../lib/i18n'
import type { TokenSet } from '../lib/types'
import { useInterval } from '../lib/useAsync'

type Method = 'password' | 'otp' | 'register'

export default function Login() {
  const { t } = useI18n()
  const { adoptTokens } = useAuth()
  const nav = useNavigate()
  const loc = useLocation()
  const from = (loc.state as { from?: string } | null)?.from ?? '/account'
  const [method, setMethod] = useState<Method>('password')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<ApiError | null>(null)
  const [busy, setBusy] = useState(false)

  const run = async (fn: () => Promise<void>) => {
    setBusy(true)
    setError(null)
    try {
      await fn()
      nav(from === '/login' ? '/account' : from, { replace: true })
    } catch (e) {
      setError(e as ApiError)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Page>
      <div className="grid-2" style={{ alignItems: 'center', gap: 40, minHeight: '60vh' }}>
        <div className="stack" style={{ '--gap': '18px' } as React.CSSProperties}>
          <motion.div animate={{ rotate: [0, -8, 8, 0], y: [0, -6, 0] }} transition={{ repeat: Infinity, duration: 5 }} style={{ width: 'fit-content' }}>
            <PillIcon form="capsules" a="var(--coral)" b="var(--mint)" size={96} />
          </motion.div>
          <h1 style={{ fontSize: 'clamp(40px, 5.5vw, 72px)' }}>{t('nav.signin')}</h1>
          <p className="muted" style={{ maxWidth: 420 }}>
            Track your orders, keep your prescriptions in one place and pay in two taps.
          </p>
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
              { id: 'otp', label: 'One-time code' },
              { id: 'register', label: 'Create account' },
            ]}
          />
          <AnimatePresence mode="wait">
            <motion.div key={method} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="stack" style={{ '--gap': '14px' } as React.CSSProperties}>
              {method === 'password' && (
                <form
                  className="stack"
                  style={{ '--gap': '14px' } as React.CSSProperties}
                  onSubmit={(e) => {
                    e.preventDefault()
                    void run(async () => adoptTokens(await post<TokenSet>('/auth/login', { email, password }, { anonymous: true }), 'password'))
                  }}
                >
                  <Field label="Email" error={error?.fieldError('email')}>
                    <input className="input" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} />
                  </Field>
                  <Field label="Password" error={error?.fieldError('password')}>
                    <input className="input" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
                  </Field>
                  <Attempts error={error} />
                  <button className="btn coral" disabled={busy}>
                    {busy ? '…' : t('nav.signin')}
                  </button>
                  <button type="button" className="btn ghost sm" style={{ alignSelf: 'center', border: 0 }} onClick={() => setMethod('otp')}>
                    Forgot your password? Sign in with a code
                  </button>
                </form>
              )}
              {method === 'otp' && <OtpFlow onTokens={(tk) => run(() => adoptTokens(tk, 'one-time code'))} />}
              {method === 'register' && (
                <RegisterForm
                  busy={busy}
                  error={error}
                  onSubmit={(body) => run(async () => adoptTokens(await post<TokenSet>('/auth/register', body, { anonymous: true }), 'register'))}
                />
              )}
              <ErrorNote error={error && !error.body.fields ? error : null} />
            </motion.div>
          </AnimatePresence>
        </motion.div>
      </div>
    </Page>
  )
}

/** Five dots that go out as wrong passwords pile up; a countdown when locked. */
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
        🔒 Too many tries. Try again in {Math.floor(lockLeft / 60)}:{String(lockLeft % 60).padStart(2, '0')}
      </motion.div>
    )
  }
  if (left === undefined) return null
  return (
    <div className="row small" style={{ '--gap': '6px' } as React.CSSProperties}>
      {Array.from({ length: 5 }, (_, i) => (
        <motion.span key={i} animate={{ scale: i < left ? 1 : 0.6, background: i < left ? 'var(--mint)' : 'var(--bad)' }} style={{ width: 12, height: 12, borderRadius: 6 }} />
      ))}
      <span className="muted">{left} tries left</span>
    </div>
  )
}

function OtpFlow({ onTokens }: { onTokens: (t: TokenSet) => void }) {
  const [channel, setChannel] = useState<'email' | 'sms'>('email')
  const [dest, setDest] = useState('')
  const [sent, setSent] = useState<{ destination_hint: string; expires_in: number } | null>(null)
  const [digits, setDigits] = useState(['', '', '', '', '', ''])
  const [error, setError] = useState<ApiError | null>(null)
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

  return (
    <div className="stack" style={{ '--gap': '14px' } as React.CSSProperties}>
      {!sent ? (
        <form
          className="stack"
          style={{ '--gap': '14px' } as React.CSSProperties}
          onSubmit={(e) => {
            e.preventDefault()
            void request()
          }}
        >
          <div className="row">
            {(['email', 'sms'] as const).map((c) => (
              <button type="button" key={c} className={`chip ${channel === c ? 'on' : ''}`} onClick={() => { setChannel(c); setDest('') }}>
                {c === 'email' ? '✉ Email' : '📱 SMS'}
              </button>
            ))}
          </div>
          <Field label={channel === 'email' ? 'Email' : 'Phone'} error={error?.fieldError('destination')}>
            <input className="input" value={dest} onChange={(e) => setDest(e.target.value)} placeholder={channel === 'email' ? 'you@example.com' : '+998 90 123 45 67'} />
          </Field>
          <button className="btn coral">Send me a code</button>
        </form>
      ) : (
        <>
          <p className="small">
            We sent a 6-digit code to <b>{sent.destination_hint}</b>. It works for {Math.round(sent.expires_in / 60)} minutes.
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
                autoComplete={i === 0 ? 'one-time-code' : 'off'}
                maxLength={6}
                value={d}
                animate={d ? { scale: [1.15, 1], borderColor: 'var(--ink)' } : {}}
                onChange={(e) => type(i, e.target.value)}
                onKeyDown={(e) => e.key === 'Backspace' && !d && i > 0 && refs.current[i - 1]?.focus()}
                aria-label={`Digit ${i + 1}`}
              />
            ))}
          </div>
          <button className="btn ghost sm" style={{ width: 'fit-content' }} onClick={() => setSent(null)}>
            ← Use a different address
          </button>
        </>
      )}
      <ErrorNote error={error && !error.body.fields ? error : null} />
    </div>
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
        <input className="input" value={f.name} onChange={set('name')} autoComplete="name" />
      </Field>
      <Field label="Email" error={error?.fieldError('email')}>
        <input className="input" type="email" value={f.email} onChange={set('email')} autoComplete="email" />
      </Field>
      <Field label="Phone (optional)" error={error?.fieldError('phone')}>
        <input className="input" value={f.phone} onChange={set('phone')} placeholder="+998901234567" autoComplete="tel" />
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
