import type { ApiErrorBody, Lang, TokenSet } from './types'

// Every call goes to /api, which the dev server (and later the reverse proxy)
// forwards to the pharmacy backend.
export const API_BASE = import.meta.env.VITE_API_BASE ?? '/api'

export class ApiError extends Error {
  body: ApiErrorBody
  constructor(body: ApiErrorBody) {
    super(body.message)
    this.body = body
  }
  get status() {
    return this.body.status
  }
  get code() {
    return this.body.code
  }
  fieldError(field: string) {
    return this.body.fields?.find((f) => f.field === field)?.message
  }
}

// ---------------------------------------------------------------- credentials

export type Credentials =
  | { kind: 'token'; access: string; refresh: string; accessExpiresAt: number; refreshExpiresAt: number; sessionId: string; via: string }
  | { kind: 'cookie'; sessionId: string }
  | { kind: 'apikey'; key: string }

const STORE_KEY = 'dorixona.credentials'

function readStored(): Credentials | null {
  try {
    const raw = localStorage.getItem(STORE_KEY)
    return raw ? (JSON.parse(raw) as Credentials) : null
  } catch {
    return null
  }
}

let creds: Credentials | null = readStored()
let actAs: string | null = null
let lang: Lang = 'en'
const credListeners = new Set<() => void>()

export const session = {
  get: () => creds,
  set(next: Credentials | null) {
    creds = next
    try {
      if (next) localStorage.setItem(STORE_KEY, JSON.stringify(next))
      else localStorage.removeItem(STORE_KEY)
    } catch {
      /* private mode: keep it in memory */
    }
    credListeners.forEach((f) => f())
  },
  subscribe(f: () => void) {
    credListeners.add(f)
    return () => void credListeners.delete(f)
  },
  setActAs(id: string | null) {
    actAs = id
    credListeners.forEach((f) => f())
  },
  actAs: () => actAs,
  setLang(l: Lang) {
    lang = l
  },
}

export function tokensToCredentials(t: TokenSet, via: string): Credentials {
  const now = Date.now()
  return {
    kind: 'token',
    access: t.access_token,
    refresh: t.refresh_token,
    accessExpiresAt: now + t.expires_in * 1000,
    refreshExpiresAt: now + t.refresh_expires_in * 1000,
    sessionId: t.session_id,
    via,
  }
}

// ---------------------------------------------------------------- event log

export type LogKind = 'request' | 'renewed' | 'renew_failed' | 'expired' | 'signed_out' | 'signed_in' | 'error'
export interface LogEntry {
  id: number
  at: number
  kind: LogKind
  text: string
  status?: number
  ms?: number
}

let logSeq = 0
const log: LogEntry[] = []
const logListeners = new Set<() => void>()

export const activity = {
  push(kind: LogKind, text: string, extra: Partial<LogEntry> = {}) {
    log.unshift({ id: ++logSeq, at: Date.now(), kind, text, ...extra })
    if (log.length > 200) log.length = 200
    logListeners.forEach((f) => f())
  },
  all: () => log,
  subscribe(f: () => void) {
    logListeners.add(f)
    return () => void logListeners.delete(f)
  },
}

// ---------------------------------------------------------------- requests

export interface RequestOptions {
  body?: unknown
  query?: Record<string, string | number | boolean | undefined | null>
  headers?: Record<string, string>
  idempotencyKey?: string
  /** Return the raw text instead of parsing JSON (CSV, HTML). */
  text?: boolean
  /** Do not attach credentials (sign-in endpoints). */
  anonymous?: boolean
  signal?: AbortSignal
}

function buildUrl(path: string, query?: RequestOptions['query']) {
  const url = new URL(API_BASE + path, window.location.origin)
  for (const [k, v] of Object.entries(query ?? {})) {
    if (v !== undefined && v !== null && v !== '') url.searchParams.set(k, String(v))
  }
  return url.pathname + url.search
}

async function send(method: string, path: string, opts: RequestOptions): Promise<Response> {
  const headers: Record<string, string> = { Locale: lang, ...opts.headers }
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'
  if (opts.idempotencyKey) headers['Idempotency-Key'] = opts.idempotencyKey
  if (!opts.anonymous) {
    if (creds?.kind === 'token') headers.Authorization = `Bearer ${creds.access}`
    if (creds?.kind === 'apikey') headers['X-API-Key'] = creds.key
    if (actAs) headers['X-Act-As'] = actAs
  }
  return fetch(buildUrl(path, opts.query), {
    method,
    headers,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
    credentials: 'same-origin',
    signal: opts.signal,
  })
}

async function parseError(res: Response): Promise<ApiError> {
  try {
    const json = await res.json()
    if (json?.error?.code) return new ApiError(json.error)
    if (typeof json?.error === 'string') {
      return new ApiError({ status: res.status, code: json.error, message: json.error_description ?? json.error })
    }
  } catch {
    /* not JSON */
  }
  return new ApiError({ status: res.status, code: 'http_' + res.status, message: res.statusText || 'Request failed' })
}

export async function api<T = unknown>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
  const started = performance.now()
  let res: Response
  try {
    res = await send(method, path, opts)
  } catch (e) {
    if ((e as Error).name === 'AbortError') throw e
    activity.push('error', `${method} ${path} — network error`)
    throw new ApiError({ status: 0, code: 'network_error', message: 'The pharmacy could not be reached. Is the backend running?' })
  }

  // An expired access token is renewed once, quietly, and the call retried.
  if (res.status === 401 && creds?.kind === 'token' && !opts.anonymous) {
    const err = await parseError(res.clone())
    if (err.code === 'token_expired') {
      activity.push('expired', 'Access token expired — renewing')
      if (await renew()) res = await send(method, path, opts)
    } else if (err.code === 'session_ended') {
      signOutLocally(`Session ended (${String(err.body.details?.reason ?? 'unknown')})`)
    }
  }

  const ms = Math.round(performance.now() - started)
  activity.push('request', `${method} ${path}`, { status: res.status, ms })
  if (!res.ok) throw await parseError(res)
  if (res.status === 204) return undefined as T
  if (opts.text) return (await res.text()) as T
  return (await res.json()) as T
}

export const get = <T>(path: string, query?: RequestOptions['query'], opts: RequestOptions = {}) => api<T>('GET', path, { ...opts, query })
export const post = <T>(path: string, body?: unknown, opts: RequestOptions = {}) => api<T>('POST', path, { ...opts, body })
export const patch = <T>(path: string, body?: unknown, opts: RequestOptions = {}) => api<T>('PATCH', path, { ...opts, body })
export const put = <T>(path: string, body?: unknown, opts: RequestOptions = {}) => api<T>('PUT', path, { ...opts, body })
export const del = <T = void>(path: string, opts: RequestOptions = {}) => api<T>('DELETE', path, opts)

// ---------------------------------------------------------------- renewal

let renewing: Promise<boolean> | null = null

/** Swap the refresh token for a new pair. Concurrent callers share one attempt. */
export function renew(): Promise<boolean> {
  if (renewing) return renewing
  renewing = (async () => {
    const c = creds
    if (c?.kind !== 'token') return false
    try {
      const res = await send('POST', '/auth/refresh', { body: { refresh_token: c.refresh }, anonymous: true })
      if (!res.ok) {
        const err = await parseError(res)
        activity.push('renew_failed', `Renewal refused: ${err.code}`, { status: res.status })
        signOutLocally(err.code === 'refresh_reused' ? 'A spent refresh token was replayed — the whole sign-in was ended' : `Renewal refused (${err.code})`)
        return false
      }
      const t = (await res.json()) as TokenSet
      session.set(tokensToCredentials(t, c.via))
      activity.push('renewed', 'New access + refresh token issued (old refresh token is now dead)', { status: 200 })
      return true
    } finally {
      renewing = null
    }
  })()
  return renewing
}

export function signOutLocally(reason: string) {
  if (!creds) return
  session.set(null)
  session.setActAs(null)
  activity.push('signed_out', reason)
}

export function newIdempotencyKey() {
  return crypto.randomUUID()
}
