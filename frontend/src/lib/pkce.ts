// OAuth 2.0 authorization code with PKCE (S256), done in the browser.

const b64url = (bytes: ArrayBuffer | Uint8Array) =>
  btoa(String.fromCharCode(...new Uint8Array(bytes))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')

export const OAUTH_CLIENT_ID = import.meta.env.VITE_OAUTH_CLIENT_ID ?? 'frege-test'
// A test client. A real browser app would be a public client with no secret;
// the pharmacy's test client requires one, so the dev default is used here.
export const OAUTH_CLIENT_SECRET = import.meta.env.VITE_OAUTH_CLIENT_SECRET ?? 'dev-only-oauth-secret'

export function redirectUri() {
  return `${window.location.origin}/oauth/callback`
}

export async function startOAuth() {
  const verifier = b64url(crypto.getRandomValues(new Uint8Array(48)))
  const challenge = b64url(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier)))
  const state = b64url(crypto.getRandomValues(new Uint8Array(12)))
  sessionStorage.setItem('dorixona.pkce', JSON.stringify({ verifier, state }))
  const q = new URLSearchParams({
    response_type: 'code',
    client_id: OAUTH_CLIENT_ID,
    redirect_uri: redirectUri(),
    code_challenge: challenge,
    code_challenge_method: 'S256',
    state,
  })
  window.location.assign(`/oauth/authorize?${q}`)
}

export function takePkce(): { verifier: string; state: string } | null {
  const raw = sessionStorage.getItem('dorixona.pkce')
  sessionStorage.removeItem('dorixona.pkce')
  return raw ? JSON.parse(raw) : null
}
