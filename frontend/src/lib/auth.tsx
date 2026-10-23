import { createContext, useCallback, useContext, useEffect, useState, useSyncExternalStore, type ReactNode } from 'react'
import { activity, del, get, post, renew, session, signOutLocally, tokensToCredentials, type Credentials } from './api'
import type { Me, Role, TokenSet } from './types'

interface Auth {
  me: Me | null
  creds: Credentials | null
  loading: boolean
  actingAs: string | null
  autoRenew: boolean
  setAutoRenew: (v: boolean) => void
  is: (role: Role) => boolean
  adoptTokens: (t: TokenSet, via: string) => Promise<void>
  signInWithCookie: (email: string, password: string) => Promise<void>
  signInWithApiKey: (key: string) => Promise<void>
  signOut: (everywhere?: boolean) => Promise<void>
  actAs: (customerId: string | null) => void
  refreshMe: () => Promise<void>
}

const Ctx = createContext<Auth | null>(null)
const rank: Record<Role, number> = { customer: 1, staff: 2, admin: 3 }

export function AuthProvider({ children }: { children: ReactNode }) {
  const creds = useSyncExternalStore(session.subscribe, session.get)
  const actingAs = useSyncExternalStore(session.subscribe, session.actAs)
  const [me, setMe] = useState<Me | null>(null)
  const [loading, setLoading] = useState(true)
  const [autoRenew, setAutoRenew] = useState(true)

  const refreshMe = useCallback(async () => {
    if (!session.get()) {
      setMe(null)
      setLoading(false)
      return
    }
    try {
      setMe(await get<Me>('/me'))
    } catch {
      setMe(null)
    } finally {
      setLoading(false)
    }
  }, [])

  // Who am I is re-read whenever the credential or the acted-as customer changes.
  const credKey = creds ? (creds.kind === 'token' ? creds.sessionId : creds.kind) : 'none'
  useEffect(() => {
    void refreshMe()
  }, [credKey, actingAs, refreshMe])

  // Renew quietly ten seconds before the access token expires.
  useEffect(() => {
    if (!autoRenew || creds?.kind !== 'token') return
    const wait = Math.max(creds.accessExpiresAt - Date.now() - 10_000, 1_000)
    const timer = window.setTimeout(() => void renew(), wait)
    return () => window.clearTimeout(timer)
  }, [creds, autoRenew])

  const adoptTokens = async (t: TokenSet, via: string) => {
    session.set(tokensToCredentials(t, via))
    activity.push('signed_in', `Signed in as ${t.user.email} via ${via}`)
  }

  const signInWithCookie = async (email: string, password: string) => {
    const r = await post<{ session_id: string; user: { email: string } }>('/auth/session', { email, password }, { anonymous: true })
    session.set({ kind: 'cookie', sessionId: r.session_id })
    activity.push('signed_in', `Signed in as ${r.user.email} with a session cookie`)
  }

  const signInWithApiKey = async (key: string) => {
    session.set({ kind: 'apikey', key })
    try {
      await get<Me>('/me')
      activity.push('signed_in', `Using API key ${key.slice(0, 12)}…`)
    } catch (e) {
      session.set(null)
      throw e
    }
  }

  const signOut = async (everywhere = false) => {
    try {
      if (creds?.kind === 'cookie') await del('/auth/session')
      else if (creds?.kind === 'token') await post('/auth/logout', everywhere ? { all_sessions: true } : undefined)
    } catch {
      /* already gone */
    }
    signOutLocally(everywhere ? 'Signed out everywhere' : 'Signed out')
  }

  const value: Auth = {
    me,
    creds,
    loading,
    actingAs,
    autoRenew,
    setAutoRenew,
    is: (role) => !!me && rank[(me.actor?.role ?? me.role) as Role] >= rank[role],
    adoptTokens,
    signInWithCookie,
    signInWithApiKey,
    signOut,
    actAs: (id) => session.setActAs(id),
    refreshMe,
  }
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useAuth() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth outside AuthProvider')
  return v
}
