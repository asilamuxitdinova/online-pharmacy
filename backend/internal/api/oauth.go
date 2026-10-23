package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

// OAuth 2.0 authorization code flow with PKCE (S256 only). The token
// endpoint speaks RFC 6749 error JSON, not the API's own error shape,
// because that is what OAuth clients expect.

var authorizePage = template.Must(template.New("authorize").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in · Dorixona</title>
<style>
body{font-family:system-ui,sans-serif;background:#f4f7f6;margin:0;display:flex;min-height:100vh;align-items:center;justify-content:center}
form{background:#fff;padding:32px;border-radius:12px;box-shadow:0 2px 16px rgba(0,0,0,.08);width:min(360px,90vw)}
h1{font-size:20px;margin:0 0 4px}p{color:#555;font-size:14px;margin:0 0 20px}
label{display:block;font-size:13px;margin:12px 0 4px}input{width:100%;box-sizing:border-box;padding:10px;border:1px solid #ccc;border-radius:8px;font-size:15px}
button{margin-top:20px;width:100%;padding:11px;border:0;border-radius:8px;background:#0b7a5a;color:#fff;font-size:15px;cursor:pointer}
.err{background:#fde8e8;color:#9b1c1c;padding:10px;border-radius:8px;font-size:14px}
</style></head><body>
<form method="post" action="/oauth/authorize">
<h1>Dorixona</h1>
<p><strong>{{.ClientID}}</strong> wants to act on your behalf.</p>
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
<label for="email">Email</label><input id="email" name="email" type="email" autocomplete="username" value="{{.Email}}" required>
<label for="password">Password</label><input id="password" name="password" type="password" autocomplete="current-password" required>
{{range $k, $v := .Hidden}}<input type="hidden" name="{{$k}}" value="{{$v}}">{{end}}
<button type="submit">Sign in and allow</button>
</form></body></html>`))

var authorizeParams = []string{"response_type", "client_id", "redirect_uri", "code_challenge", "code_challenge_method", "state", "scope"}

func (s *Server) redirectAllowed(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil || u.Fragment != "" || u.Host == "" {
		return false
	}
	if len(s.cfg.OAuthRedirectURIs) > 0 {
		return slices.Contains(s.cfg.OAuthRedirectURIs, uri)
	}
	host := u.Hostname()
	return u.Scheme == "https" || (u.Scheme == "http" && (host == "localhost" || host == "127.0.0.1"))
}

func oauthPageError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte("<!doctype html><title>Authorization error</title><h1>Authorization error</h1><p>" + template.HTMLEscapeString(msg) + "</p>"))
}

func redirectWith(w http.ResponseWriter, r *http.Request, redirect string, params url.Values) {
	u, _ := url.Parse(redirect)
	q := u.Query()
	for k, v := range params {
		q[k] = v
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// checkAuthorizeRequest validates the parameters. A bad client or redirect
// is shown on the page (never redirect to an unchecked address); anything
// else is sent back to the client's redirect_uri.
func (s *Server) checkAuthorizeRequest(w http.ResponseWriter, r *http.Request, q url.Values) bool {
	if q.Get("client_id") != s.cfg.OAuthClientID {
		oauthPageError(w, "Unknown client_id.")
		return false
	}
	redirect := q.Get("redirect_uri")
	if !s.redirectAllowed(redirect) {
		oauthPageError(w, "redirect_uri is missing or not registered for this client.")
		return false
	}
	fail := func(code, desc string) bool {
		redirectWith(w, r, redirect, url.Values{"error": {code}, "error_description": {desc}, "state": {q.Get("state")}})
		return false
	}
	if q.Get("response_type") != "code" {
		return fail("unsupported_response_type", "response_type must be code")
	}
	if q.Get("code_challenge") == "" {
		return fail("invalid_request", "PKCE is required: send code_challenge and code_challenge_method=S256")
	}
	if q.Get("code_challenge_method") != "S256" {
		return fail("invalid_request", "code_challenge_method must be S256")
	}
	return true
}

func (s *Server) authorizeForm(w http.ResponseWriter, r *http.Request, _ *principal) error {
	q := r.URL.Query()
	if !s.checkAuthorizeRequest(w, r, q) {
		return nil
	}
	return renderAuthorize(w, q, "", "", http.StatusOK)
}

func renderAuthorize(w http.ResponseWriter, q url.Values, email, errMsg string, status int) error {
	hidden := map[string]string{}
	for _, k := range authorizeParams {
		if v := q.Get(k); v != "" {
			hidden[k] = v
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	return authorizePage.Execute(w, map[string]any{"ClientID": q.Get("client_id"), "Hidden": hidden, "Email": email, "Error": errMsg})
}

func (s *Server) authorizeSubmit(w http.ResponseWriter, r *http.Request, _ *principal) error {
	if err := r.ParseForm(); err != nil {
		oauthPageError(w, "The form could not be read.")
		return nil
	}
	q := r.PostForm
	if !s.checkAuthorizeRequest(w, r, q) {
		return nil
	}
	email := q.Get("email")
	u, aerr, err := s.checkPassword(r.Context(), email, q.Get("password"))
	if err != nil {
		return err
	}
	if aerr != nil {
		return renderAuthorize(w, q, email, aerr.Message, aerr.Status)
	}
	code := "ac_" + store.RandomString(24)
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `INSERT INTO oauth_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
			store.Hash(code), q.Get("client_id"), u.ID, q.Get("redirect_uri"), q.Get("code_challenge"), s.unix()+600)
		return err
	})
	if err != nil {
		return err
	}
	redirectWith(w, r, q.Get("redirect_uri"), url.Values{"code": {code}, "state": {q.Get("state")}})
	return nil
}

func oauthError(w http.ResponseWriter, status int, code, desc string) {
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Basic realm="pharmacy-oauth"`)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

func (s *Server) oauthToken(w http.ResponseWriter, r *http.Request, _ *principal) error {
	form := url.Values{}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var body map[string]string
		if e := decode(r, &body); e != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", e.Message)
			return nil
		}
		for k, v := range body {
			form.Set(k, v)
		}
	} else {
		if err := r.ParseForm(); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "the form body could not be read")
			return nil
		}
		form = r.PostForm
	}

	clientID, secret, basic := r.BasicAuth()
	if !basic {
		clientID, secret = form.Get("client_id"), form.Get("client_secret")
	}
	if clientID != s.cfg.OAuthClientID || subtle.ConstantTimeCompare([]byte(secret), []byte(s.cfg.OAuthClientSecret)) != 1 {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "unknown client or wrong client secret")
		return nil
	}
	w.Header().Set("Cache-Control", "no-store")

	switch form.Get("grant_type") {
	case "authorization_code":
		return s.exchangeCode(w, r, form, clientID)
	case "refresh_token":
		tokens, aerr, err := s.rotateRefresh(r.Context(), form.Get("refresh_token"))
		if err != nil {
			return err
		}
		if aerr != nil {
			oauthError(w, http.StatusBadRequest, "invalid_grant", aerr.Code+": "+aerr.Message)
			return nil
		}
		writeJSON(w, http.StatusOK, tokens)
		return nil
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
		return nil
	}
}

func (s *Server) exchangeCode(w http.ResponseWriter, r *http.Request, form url.Values, clientID string) error {
	code, verifier := form.Get("code"), form.Get("code_verifier")
	if code == "" || verifier == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "code and code_verifier are required")
		return nil
	}
	var tokens *tokenSet
	var grantErr string
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		ctx := r.Context()
		var client, uid, redirect, challenge string
		var expires int64
		var used sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT client_id, user_id, redirect_uri, code_challenge, expires_at, used_at FROM oauth_codes WHERE code_hash = ?`,
			store.Hash(code)).Scan(&client, &uid, &redirect, &challenge, &expires, &used)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			grantErr = "the authorization code is not valid"
			return nil
		case err != nil:
			return err
		case used.Valid:
			grantErr = "the authorization code was already used"
			return nil
		case expires <= s.unix():
			grantErr = "the authorization code has expired"
			return nil
		case client != clientID:
			grantErr = "the code was issued to another client"
			return nil
		case form.Get("redirect_uri") != "" && form.Get("redirect_uri") != redirect:
			grantErr = "redirect_uri does not match the one used to get the code"
			return nil
		}
		sum := sha256.Sum256([]byte(verifier))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
			grantErr = "code_verifier does not match the code_challenge (PKCE)"
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE oauth_codes SET used_at = ? WHERE code_hash = ?`, s.unix(), store.Hash(code)); err != nil {
			return err
		}
		u, err := loadUser(ctx, tx, uid)
		if err != nil {
			return err
		}
		tokens, err = s.startSession(ctx, tx, u, "oauth")
		return err
	})
	if err != nil {
		return err
	}
	if grantErr != "" {
		oauthError(w, http.StatusBadRequest, "invalid_grant", grantErr)
		return nil
	}
	writeJSON(w, http.StatusOK, tokens)
	return nil
}
