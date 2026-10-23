import { AnimatePresence, motion } from 'motion/react'
import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import { TokenRing } from '../components/TokenRing'
import { ErrorNote, Field, Modal, Page } from '../components/ui'
import { activity, API_BASE, ApiError, del, get, post, renew, session, type LogEntry } from '../lib/api'
import { useAuth } from '../lib/auth'
import { useToast } from '../lib/toast'
import type { InboxMessage } from '../lib/types'
import { useAsync, useInterval } from '../lib/useAsync'

function decodeJwt(token: string): Record<string, unknown> | null {
  try {
    const part = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    return JSON.parse(atob(part))
  } catch {
    return null
  }
}

export default function Lab() {
  return (
    <Page wide>
      <div className="stack" style={{ '--gap': '24px' } as React.CSSProperties}>
        <div className="spread" style={{ alignItems: 'flex-end' }}>
          <div className="stack" style={{ '--gap': '8px' } as React.CSSProperties}>
            <span className="kicker">For people testing Frege</span>
            <h1 style={{ fontSize: 'clamp(40px, 6vw, 76px)' }}>
              The Lab <motion.span style={{ display: 'inline-block' }} animate={{ rotate: [0, -10, 10, 0] }} transition={{ repeat: Infinity, duration: 4 }}>⚗</motion.span>
            </h1>
            <p className="muted" style={{ maxWidth: 620 }}>Watch tokens age and renew, break things on purpose, read the inbox the pharmacy writes to, and send it webhooks.</p>
          </div>
          <div className="row">
            <a className="btn ghost sm" href="/openapi.yaml" target="_blank" rel="noreferrer">openapi.yaml</a>
            <a className="btn ghost sm" href="/docs/" target="_blank" rel="noreferrer">Docs</a>
          </div>
        </div>
        <div style={{ display: 'grid', gap: 20, gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 420px), 1fr))' }}>
          <SessionPanel />
          <ActivityPanel />
          <ChaosPanel />
          <InboxPanel />
          <WebhookPanel />
          <SettingsPanel />
        </div>
      </div>
    </Page>
  )
}

function Panel({ title, tag, children }: { title: string; tag?: string; children: React.ReactNode }) {
  return (
    <motion.section className="card stack" style={{ '--gap': '14px', minWidth: 0 } as React.CSSProperties} initial={{ opacity: 0, y: 20 }} whileInView={{ opacity: 1, y: 0 }} viewport={{ once: true }}>
      <div className="spread">
        <h3>{title}</h3>
        {tag && <span className="badge plain mono">{tag}</span>}
      </div>
      {children}
    </motion.section>
  )
}

// ---------------------------------------------------------------- session

function SessionPanel() {
  const { me, creds, autoRenew, setAutoRenew, signOut } = useAuth()
  const toast = useToast()
  const spent = useRef<string[]>([])
  const lastRefresh = useRef<string | null>(null)
  const [spentCount, setSpentCount] = useState(0)
  const [result, setResult] = useState<ApiError | null>(null)

  // Remember every refresh token this tab has seen die, to replay one later.
  useEffect(() => {
    if (creds?.kind !== 'token') return
    if (lastRefresh.current && lastRefresh.current !== creds.refresh) {
      spent.current.push(lastRefresh.current)
      setSpentCount(spent.current.length)
    }
    lastRefresh.current = creds.refresh
  }, [creds])

  if (!creds || !me) {
    return (
      <Panel title="Session" tag="no credential">
        <p className="muted">Sign in to watch a token live.</p>
        <Link to="/login" className="btn coral" state={{ from: '/lab' }}>Sign in</Link>
      </Panel>
    )
  }
  const claims = creds.kind === 'token' ? decodeJwt(creds.access) : null
  const lifetime = claims ? ((claims.exp as number) - (claims.iat as number)) * 1000 : 0

  const replay = async () => {
    const old = spent.current[0]
    if (!old) return
    try {
      await post('/auth/refresh', { refresh_token: old }, { anonymous: true })
    } catch (e) {
      const err = e as ApiError
      setResult(err)
      if (err.code === 'refresh_reused') {
        activity.push('renew_failed', 'Replayed a spent refresh token → the whole sign-in ended')
        toast('Reuse detected: the sign-in was ended for everyone holding it', 'bad')
        // Our own tokens belong to the dead session now; the next call proves it.
        await get('/me').catch(() => undefined)
      }
    }
  }

  return (
    <Panel title="Session" tag={creds.kind === 'token' ? `bearer · ${creds.via}` : creds.kind}>
      <div className="row" style={{ alignItems: 'center', '--gap': '22px' } as React.CSSProperties}>
        {creds.kind === 'token' ? (
          <TokenRing expiresAt={creds.accessExpiresAt} lifetime={lifetime} />
        ) : (
          <div style={{ width: 168, height: 168, borderRadius: 84, display: 'grid', placeItems: 'center', background: 'var(--bg-2)', textAlign: 'center' }} className="small">
            {creds.kind === 'cookie' ? '🍪 cookie\nno token to watch' : '🔑 API key\nnever expires'}
          </div>
        )}
        <div className="stack grow" style={{ '--gap': '8px', minWidth: 180 } as React.CSSProperties}>
          <b>{me.name}</b>
          <span className="small muted">{me.email} · {me.role}</span>
          {claims && (
            <pre className="mono tiny" style={{ margin: 0, padding: 10, borderRadius: 10, background: 'var(--bg-2)', overflowX: 'auto' }}>
              {JSON.stringify({ sub: claims.sub, sid: claims.sid, exp: new Date((claims.exp as number) * 1000).toLocaleTimeString() }, null, 1)}
            </pre>
          )}
        </div>
      </div>
      {creds.kind === 'token' && (
        <>
          <label className="check small">
            <input type="checkbox" checked={autoRenew} onChange={(e) => setAutoRenew(e.target.checked)} />
            Renew quietly 10s before expiry (turn off to see a 401 token_expired, then a renewal on demand)
          </label>
          <div className="row">
            <button className="btn mint sm" onClick={() => void renew()}>Renew now</button>
            <button className="btn sm" onClick={() => void get('/me')}>Call GET /me</button>
            <button className="btn danger sm" onClick={replay} disabled={spentCount === 0} title="POST /auth/refresh with a token that was already used">
              Replay a spent refresh token ({spentCount})
            </button>
          </div>
          {spentCount === 0 && <span className="tiny muted">Renew once to get a spent token to replay.</span>}
        </>
      )}
      <ErrorNote error={result} />
      <div className="row">
        <button className="btn ghost sm" onClick={() => void signOut(false)}>Sign out</button>
        <span className="tiny muted">Set ACCESS_TOKEN_TTL=60s on the backend (<code>make dev</code>) to see renewal within a minute.</span>
      </div>
    </Panel>
  )
}

// ---------------------------------------------------------------- activity

const kindColor: Record<string, string> = {
  request: 'var(--ink-2)',
  renewed: 'var(--mint-ink)',
  renew_failed: 'var(--bad)',
  expired: '#c88a00',
  signed_out: 'var(--bad)',
  signed_in: 'var(--mint-ink)',
  error: 'var(--bad)',
}

function ActivityPanel() {
  // The log is mutated in place, so subscribe and copy it on every change.
  const [entries, setEntries] = useState<LogEntry[]>(() => [...activity.all()])
  useEffect(() => activity.subscribe(() => setEntries([...activity.all()])), [])
  return (
    <Panel title="Activity" tag="this tab">
      <div className="stack" style={{ '--gap': '4px', maxHeight: 360, overflowY: 'auto' } as React.CSSProperties}>
        {entries.length === 0 && <p className="muted small">Requests show up here as they happen.</p>}
        <AnimatePresence initial={false}>
          {entries.slice(0, 60).map((e: LogEntry) => (
            <motion.div key={e.id} initial={{ opacity: 0, x: -16, backgroundColor: 'var(--warn-bg)' }} animate={{ opacity: 1, x: 0, backgroundColor: 'rgba(0,0,0,0)' }} transition={{ duration: 0.5 }} className="row mono tiny" style={{ '--gap': '8px', padding: '4px 6px', borderRadius: 6, flexWrap: 'nowrap' } as React.CSSProperties}>
              <span className="muted">{new Date(e.at).toLocaleTimeString()}</span>
              {e.status !== undefined && (
                <span style={{ fontWeight: 800, color: e.status >= 500 ? 'var(--bad)' : e.status >= 400 ? '#c88a00' : 'var(--mint-ink)' }}>{e.status}</span>
              )}
              <span className="grow" style={{ color: kindColor[e.kind], overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontWeight: e.kind === 'request' ? 400 : 700 }}>
                {e.text}
              </span>
              {e.ms !== undefined && <span className="muted">{e.ms}ms</span>}
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
    </Panel>
  )
}

// ---------------------------------------------------------------- chaos

const statuses = [400, 401, 403, 404, 409, 422, 429, 500, 502, 503, 504]

function ChaosPanel() {
  const [delay, setDelay] = useState(0)
  const [out, setOut] = useState<{ label: string; status: number; body: string; ms: number } | null>(null)
  const [busy, setBusy] = useState(false)
  const flakyKey = useRef(`k${Date.now()}`)

  const fire = async (label: string, query: string) => {
    setBusy(true)
    const t0 = performance.now()
    try {
      const res = await fetch(`${API_BASE}/_test/${query}`)
      const body = await res.text()
      activity.push('request', `GET /_test/${query}`, { status: res.status, ms: Math.round(performance.now() - t0) })
      setOut({ label, status: res.status, body: pretty(body) + (res.headers.get('Retry-After') ? `\n\nRetry-After: ${res.headers.get('Retry-After')}` : ''), ms: Math.round(performance.now() - t0) })
    } catch (e) {
      setOut({ label, status: 0, body: `Network error: ${(e as Error).message}\n(the connection was dropped — what a timeout or refused connection looks like)`, ms: Math.round(performance.now() - t0) })
    } finally {
      setBusy(false)
    }
  }
  return (
    <Panel title="Break it on purpose" tag="/_test/fail">
      <div className="row" style={{ '--gap': '6px' } as React.CSSProperties}>
        {statuses.map((s) => (
          <motion.button key={s} className="chip mono" whileTap={{ scale: 0.85, rotate: -6 }} disabled={busy} onClick={() => fire(String(s), `fail?status=${s}&delay=${delay}`)} style={{ borderColor: s >= 500 ? 'var(--bad)' : undefined }}>
            {s}
          </motion.button>
        ))}
      </div>
      <div className="row">
        <button className="btn sm" disabled={busy} onClick={() => fire('flaky', `fail?status=503&flaky_key=${flakyKey.current}`)}>Flaky (fails, then works)</button>
        <button className="btn sm" disabled={busy} onClick={() => fire('drop', 'fail?mode=close')}>Drop the connection</button>
        <button className="btn sm" disabled={busy} onClick={() => fire('delay', `delay?seconds=${delay || 3}`)}>Slow answer</button>
      </div>
      <label className="row small" style={{ flexWrap: 'nowrap' }}>
        Delay
        <input type="range" min={0} max={10} step={0.5} value={delay} onChange={(e) => setDelay(Number(e.target.value))} className="grow" style={{ accentColor: 'var(--coral)' }} />
        <b className="mono">{delay}s</b>
      </label>
      <AnimatePresence mode="wait">
        {busy ? (
          <motion.div key="busy" className="small muted" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
            Waiting… <motion.span animate={{ opacity: [0.2, 1, 0.2] }} transition={{ repeat: Infinity, duration: 1 }}>●</motion.span>
          </motion.div>
        ) : out ? (
          <motion.div key={out.label + out.ms} initial={{ opacity: 0, scale: 0.97 }} animate={{ opacity: 1, scale: 1 }}>
            <div className="spread small" style={{ marginBottom: 6 }}>
              <b style={{ color: out.status >= 500 || out.status === 0 ? 'var(--bad)' : out.status >= 400 ? '#c88a00' : 'var(--mint-ink)' }}>
                {out.status || 'no answer'}
              </b>
              <span className="muted mono">{out.ms}ms</span>
            </div>
            <pre className="mono tiny" style={{ margin: 0, padding: 10, borderRadius: 10, background: 'var(--bg-2)', maxHeight: 220, overflow: 'auto', whiteSpace: 'pre-wrap' }}>{out.body}</pre>
          </motion.div>
        ) : null}
      </AnimatePresence>
    </Panel>
  )
}

function pretty(s: string) {
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

// ---------------------------------------------------------------- inbox

function InboxPanel() {
  const [msgs, setMsgs] = useState<InboxMessage[]>([])
  const [off, setOff] = useState(false)
  const load = async () => {
    try {
      const r = await get<{ data: InboxMessage[] }>('/_test/inbox', { limit: 30 }, { anonymous: true })
      setMsgs(r.data)
      setOff(false)
    } catch {
      setOff(true)
    }
  }
  useEffect(() => {
    void load()
  }, [])
  useInterval(load, 2500)
  return (
    <Panel title="Demo inbox" tag="codes · notices · webhooks">
      {off && <p className="small muted">The test helpers look switched off (TEST_HELPERS=false).</p>}
      <div className="stack" style={{ '--gap': '8px', maxHeight: 360, overflowY: 'auto' } as React.CSSProperties}>
        {!off && msgs.length === 0 && <p className="muted small">Empty. Ask for a sign-in code to see one arrive.</p>}
        <AnimatePresence initial={false}>
          {msgs.map((m) => (
            <motion.div key={m.id} layout initial={{ opacity: 0, y: -20, scale: 0.95 }} animate={{ opacity: 1, y: 0, scale: 1 }} className="card tight flat" style={{ background: 'var(--card-2)' }}>
              <div className="spread tiny">
                <span className="badge plain">{m.kind}</span>
                <span className="muted">{new Date(m.created_at).toLocaleTimeString()}</span>
              </div>
              <div className="small" style={{ fontWeight: 700, marginTop: 4 }}>{m.subject}</div>
              <div className="tiny muted">to {m.recipient}</div>
              {typeof m.meta.code === 'string' ? (
                <div className="mono" style={{ fontSize: 22, letterSpacing: '.25em' }}>{m.meta.code}</div>
              ) : (
                <div className="tiny mono" style={{ wordBreak: 'break-all', marginTop: 4 }}>{m.body.slice(0, 200)}</div>
              )}
              {'verified' in m.meta && (
                <div className="tiny" style={{ fontWeight: 800, color: m.meta.verified ? 'var(--mint-ink)' : 'var(--bad)' }}>
                  {m.meta.verified ? '✓ signature verified' : `✗ ${String(m.meta.failure)}`}
                </div>
              )}
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
      <div className="row">
        <a className="btn ghost sm" href={`${API_BASE}/_test/inbox/view`} target="_blank" rel="noreferrer">Full-page inbox</a>
        <button className="btn ghost sm" onClick={async () => { await del('/_test/inbox', { anonymous: true }); void load() }}>Clear</button>
      </div>
    </Panel>
  )
}

// ---------------------------------------------------------------- webhooks

async function hmacHex(secret: string, message: string) {
  const key = await crypto.subtle.importKey('raw', new TextEncoder().encode(secret), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign'])
  const sig = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(message))
  return [...new Uint8Array(sig)].map((b) => b.toString(16).padStart(2, '0')).join('')
}

function WebhookPanel() {
  const [secret, setSecret] = useState('dev-only-webhook-secret')
  const [body, setBody] = useState('{"type":"customer.signed_in","data":{"customer_id":"usr_aziza"}}')
  const [result, setResult] = useState<{ ok: boolean; text: string } | null>(null)

  const send = async (mode: 'signed' | 'tampered' | 'stale' | 'unsigned') => {
    const stamp = String(Math.floor(Date.now() / 1000) - (mode === 'stale' ? 3600 : 0))
    const sig = 'sha256=' + (await hmacHex(secret, `${stamp}.${body}`))
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (mode !== 'unsigned') {
      headers['X-Webhook-Timestamp'] = stamp
      headers['X-Webhook-Signature'] = sig
    }
    const sent = mode === 'tampered' ? body.replace('usr_aziza', 'usr_admin') : body
    const res = await fetch(`${API_BASE}/_test/webhooks`, { method: 'POST', headers, body: sent })
    const text = await res.text()
    activity.push('request', `POST /_test/webhooks (${mode})`, { status: res.status })
    setResult({ ok: res.ok, text: `${res.status} ${pretty(text)}` })
  }

  return (
    <Panel title="Webhook receiver" tag="HMAC-SHA256">
      <p className="tiny muted">
        Signature = <code>sha256=</code> hex HMAC of <code>"&lt;timestamp&gt;.&lt;body&gt;"</code>. Signed here in the browser with the secret below; the receiver checks it and writes every delivery to the inbox.
      </p>
      <Field label="WEBHOOK_SECRET">
        <input className="input mono" value={secret} onChange={(e) => setSecret(e.target.value)} />
      </Field>
      <Field label="Body">
        <textarea className="textarea mono" style={{ fontSize: 12 }} value={body} onChange={(e) => setBody(e.target.value)} />
      </Field>
      <div className="row">
        <button className="btn mint sm" onClick={() => send('signed')}>Send signed</button>
        <button className="btn sm" onClick={() => send('tampered')}>Tamper with the body</button>
        <button className="btn sm" onClick={() => send('stale')}>Hour-old timestamp</button>
        <button className="btn ghost sm" onClick={() => send('unsigned')}>No signature</button>
      </div>
      <AnimatePresence mode="wait">
        {result && (
          <motion.pre key={result.text} initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} className="mono tiny" style={{ margin: 0, padding: 10, borderRadius: 10, whiteSpace: 'pre-wrap', background: result.ok ? 'var(--ok-bg)' : 'var(--bad-bg)' }}>
            {result.text}
          </motion.pre>
        )}
      </AnimatePresence>
    </Panel>
  )
}

// ---------------------------------------------------------------- settings and reset

function SettingsPanel() {
  const toast = useToast()
  const cfg = useAsync((s) => get<Record<string, unknown>>('/_test/config', undefined, { signal: s, anonymous: true }), [])
  const [confirm, setConfirm] = useState(false)
  const [token, setToken] = useState('')
  const [error, setError] = useState<ApiError | null>(null)
  const rows: [string, string][] = cfg.data
    ? [
        ['Access token lives', `${cfg.data.access_token_ttl_seconds}s`],
        ['Refresh token lives', `${Math.round(Number(cfg.data.refresh_token_ttl_seconds) / 86400)} days`],
        ['Lockout', `${cfg.data.login_max_attempts} tries → ${cfg.data.login_lockout_seconds}s`],
        ['Fixed one-time code', cfg.data.fixed_otp_code ? 'on' : 'off'],
        ['Auto-refund limit', `${cfg.data.auto_refund_limit} UZS`],
        ['Free delivery from', `${cfg.data.free_delivery_threshold} UZS`],
        ['Slow jobs take', `${cfg.data.job_duration_seconds}s`],
        ['OAuth client', String(cfg.data.oauth_client_id)],
      ]
    : []
  const reset = async () => {
    setError(null)
    try {
      await post('/_test/reset', undefined, { anonymous: true, headers: token ? { 'X-Test-Token': token } : {} })
      session.set(null)
      setConfirm(false)
      toast('Fresh pharmacy. Every session was wiped, including yours.', 'ok')
    } catch (e) {
      setError(e as ApiError)
    }
  }
  return (
    <Panel title="Settings & reset" tag="/_test/config">
      <ErrorNote error={cfg.error} />
      <table className="grid">
        <tbody>
          {rows.map(([k, v]) => (
            <tr key={k}>
              <td className="muted small">{k}</td>
              <td className="mono small">{v}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <button className="btn danger" onClick={() => setConfirm(true)}>Reset all data…</button>
      <Modal open={confirm} onClose={() => setConfirm(false)}>
        <div className="stack" style={{ '--gap': '14px' } as React.CSSProperties}>
          <h3>Reset the pharmacy?</h3>
          <p className="small muted">Every order, account change and session is wiped and the demo data is seeded again. Everyone signed in is signed out.</p>
          {Boolean(cfg.data?.reset_needs_token) && (
            <Field label="X-Test-Token">
              <input className="input mono" value={token} onChange={(e) => setToken(e.target.value)} />
            </Field>
          )}
          <ErrorNote error={error} />
          <div className="row">
            <button className="btn danger" onClick={reset}>Reset</button>
            <button className="btn ghost" onClick={() => setConfirm(false)}>Cancel</button>
          </div>
        </div>
      </Modal>
    </Panel>
  )
}
