import { motion } from 'motion/react'
import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { ErrorNote, Page } from '../components/ui'
import { ApiError, post } from '../lib/api'
import { useAuth } from '../lib/auth'
import { OAUTH_CLIENT_ID, OAUTH_CLIENT_SECRET, redirectUri, takePkce } from '../lib/pkce'
import type { TokenSet } from '../lib/types'

export default function OAuthCallback() {
  const [params] = useSearchParams()
  const { adoptTokens } = useAuth()
  const nav = useNavigate()
  const [error, setError] = useState<ApiError | null>(null)
  const ran = useRef(false)

  useEffect(() => {
    if (ran.current) return // StrictMode runs effects twice; a code works once.
    ran.current = true
    const fail = (code: string, message: string) => setError(new ApiError({ status: 400, code, message }))
    if (params.get('error')) return fail(params.get('error')!, params.get('error_description') ?? 'The pharmacy refused the sign-in.')
    const pkce = takePkce()
    if (!pkce) return fail('missing_verifier', 'This browser did not start the sign-in (no PKCE verifier stored).')
    if (pkce.state !== params.get('state')) return fail('state_mismatch', 'The state does not match: the redirect did not come from our request.')
    post<TokenSet>(
      '/oauth/token',
      {
        grant_type: 'authorization_code',
        code: params.get('code'),
        redirect_uri: redirectUri(),
        code_verifier: pkce.verifier,
        client_id: OAUTH_CLIENT_ID,
        client_secret: OAUTH_CLIENT_SECRET,
      },
      { anonymous: true },
    )
      .then(async (t) => {
        await adoptTokens(t, 'OAuth + PKCE')
        nav('/account', { replace: true })
      })
      .catch((e) => setError(e as ApiError))
  }, [params, adoptTokens, nav])

  return (
    <Page>
      <div className="stack center" style={{ '--gap': '18px', alignItems: 'center', paddingTop: 60 } as React.CSSProperties}>
        {!error ? (
          <>
            <motion.div animate={{ rotate: 360 }} transition={{ repeat: Infinity, duration: 1.2, ease: 'linear' }} style={{ width: 48, height: 22, borderRadius: 11, background: 'linear-gradient(90deg, var(--coral) 50%, var(--mint) 50%)' }} />
            <h2>Swapping the code for tokens…</h2>
            <p className="mono small muted">POST /oauth/token · grant_type=authorization_code · code_verifier</p>
          </>
        ) : (
          <div className="stack" style={{ maxWidth: 520, width: '100%' }}>
            <h2>OAuth sign-in failed</h2>
            <ErrorNote error={error} />
            <Link to="/login" className="btn">
              Back to sign-in
            </Link>
          </div>
        )}
      </div>
    </Page>
  )
}
