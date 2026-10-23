package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

const sessionCookie = "pharm_session"

type user struct {
	ID, Email, Name, Role string
	Phone, TIN            sql.NullString
	DisabledAt            sql.NullInt64
	CreatedAt             int64
}

const userColumns = `id, email, name, role, phone, tin, disabled_at, created_at`

func scanUser(row interface{ Scan(...any) error }) (*user, error) {
	var u user
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Phone, &u.TIN, &u.DisabledAt, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func loadUser(ctx context.Context, q querier, id string) (*user, error) {
	return scanUser(q.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// publicUser is what a customer may see about themselves: no taxpayer number.
func publicUser(u *user) map[string]any {
	return map[string]any{
		"id": u.ID, "email": u.Email, "name": u.Name, "phone": nullStr(u.Phone),
		"role": u.Role, "created_at": ts(u.CreatedAt),
	}
}

// principal is who is calling. User is whose data the request acts on; Actor
// is who signed in. They differ only when staff send X-Act-As.
type principal struct {
	User      *user
	Actor     *user
	SessionID string // empty for API keys
	Method    string // bearer, cookie, api_key
	ActingFor *user  // the customer, when acting on their behalf
}

func (p *principal) is(l level) bool { return roleLevel[p.Actor.Role] >= l }

func (s *Server) authenticate(r *http.Request) (*principal, *apiError) {
	ctx := r.Context()
	if h := r.Header.Get("Authorization"); h != "" {
		scheme, token, _ := strings.Cut(h, " ")
		token = strings.TrimSpace(token)
		if !strings.EqualFold(scheme, "Bearer") || token == "" {
			return nil, errf(http.StatusUnauthorized, "token_invalid", "The Authorization header must be \"Bearer <token>\".")
		}
		if strings.HasPrefix(token, "phk_") {
			return s.authAPIKey(ctx, token)
		}
		return s.authJWT(ctx, token)
	}
	if k := r.Header.Get("X-API-Key"); k != "" {
		return s.authAPIKey(ctx, k)
	}
	if k := r.URL.Query().Get("api_key"); k != "" {
		return s.authAPIKey(ctx, k)
	}
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return s.authCookie(ctx, c.Value)
	}
	return nil, errf(http.StatusUnauthorized, "token_missing", "Sign-in required: send an access token, an API key or a session cookie.")
}

type accessClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

func (s *Server) signAccessToken(userID, sessionID string) (string, error) {
	now := s.now()
	claims := accessClaims{
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.PublicURL,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTokenTTL)),
			ID:        store.NewID("at"),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
}

func (s *Server) authJWT(ctx context.Context, raw string) (*principal, *apiError) {
	var claims accessClaims
	_, err := jwt.ParseWithClaims(raw, &claims, func(*jwt.Token) (any, error) { return []byte(s.cfg.JWTSecret), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(s.cfg.PublicURL), jwt.WithTimeFunc(s.now), jwt.WithExpirationRequired())
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return nil, errf(http.StatusUnauthorized, "token_expired", "The access token has expired. Renew it with the refresh token.")
	case err != nil:
		return nil, errf(http.StatusUnauthorized, "token_invalid", "The access token is not valid.")
	}
	if aerr := s.checkSession(ctx, claims.SessionID); aerr != nil {
		return nil, aerr
	}
	return s.principalFor(ctx, claims.Subject, claims.SessionID, "bearer")
}

func (s *Server) authCookie(ctx context.Context, value string) (*principal, *apiError) {
	var sid, uid string
	err := s.store.DB.QueryRowContext(ctx, `SELECT id, user_id FROM sessions WHERE cookie_hash = ?`, store.Hash(value)).Scan(&sid, &uid)
	if err != nil {
		return nil, errf(http.StatusUnauthorized, "token_invalid", "The session cookie is not valid.")
	}
	if aerr := s.checkSession(ctx, sid); aerr != nil {
		return nil, aerr
	}
	return s.principalFor(ctx, uid, sid, "cookie")
}

func (s *Server) authAPIKey(ctx context.Context, key string) (*principal, *apiError) {
	var uid string
	var revoked sql.NullInt64
	err := s.store.DB.QueryRowContext(ctx, `SELECT user_id, revoked_at FROM api_keys WHERE key_hash = ?`, store.Hash(key)).Scan(&uid, &revoked)
	if err != nil {
		return nil, errf(http.StatusUnauthorized, "api_key_invalid", "The API key is not valid.")
	}
	if revoked.Valid {
		return nil, errf(http.StatusUnauthorized, "api_key_revoked", "The API key was revoked on %s.", ts(revoked.Int64))
	}
	return s.principalFor(ctx, uid, "", "api_key")
}

func (s *Server) checkSession(ctx context.Context, sid string) *apiError {
	var expires int64
	var ended sql.NullInt64
	var reason sql.NullString
	err := s.store.DB.QueryRowContext(ctx, `SELECT expires_at, ended_at, ended_reason FROM sessions WHERE id = ?`, sid).Scan(&expires, &ended, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return errf(http.StatusUnauthorized, "session_ended", "This sign-in no longer exists. Sign in again.").with("reason", "deleted")
	}
	if err != nil {
		return errf(http.StatusInternalServerError, "internal_error", "Could not check the session.")
	}
	if ended.Valid {
		return errf(http.StatusUnauthorized, "session_ended", "This sign-in was ended (%s). Sign in again.", reason.String).
			with("reason", reason.String).with("ended_at", ts(ended.Int64))
	}
	if expires <= s.unix() {
		return errf(http.StatusUnauthorized, "session_ended", "This sign-in reached its maximum age. Sign in again.").with("reason", "expired")
	}
	return nil
}

func (s *Server) principalFor(ctx context.Context, uid, sid, method string) (*principal, *apiError) {
	u, err := loadUser(ctx, s.store.DB, uid)
	if err != nil {
		return nil, errf(http.StatusUnauthorized, "session_ended", "The account behind this credential no longer exists.").with("reason", "account_deleted")
	}
	if u.DisabledAt.Valid {
		return nil, errf(http.StatusForbidden, "account_disabled", "This account was disabled by an administrator.")
	}
	return &principal{User: u, Actor: u, SessionID: sid, Method: method}, nil
}

// authorize checks the role and applies X-Act-As.
func (s *Server) authorize(r *http.Request, p *principal, min level) *apiError {
	if target := r.Header.Get("X-Act-As"); target != "" {
		if !p.is(staff) {
			return roleRequired(staff, p, "Only staff may act on a customer's behalf (X-Act-As).")
		}
		u, err := loadUser(r.Context(), s.store.DB, target)
		if err != nil || u.Role != "customer" {
			return errf(http.StatusNotFound, "customer_not_found", "X-Act-As names no customer with id %q.", target)
		}
		p.User, p.ActingFor = u, u
		// Acting as a customer means seeing what they see, nothing more.
		if min > customer {
			return errf(http.StatusForbidden, "act_as_not_allowed", "Staff and admin endpoints cannot be called while acting as a customer. Drop the X-Act-As header.")
		}
		return nil
	}
	if !p.is(min) {
		return roleRequired(min, p, "")
	}
	return nil
}

func roleRequired(min level, p *principal, msg string) *apiError {
	if msg == "" {
		msg = "This needs the " + levelName[min] + " role; you are signed in as " + p.Actor.Role + "."
	}
	e := errf(http.StatusForbidden, "role_required", "%s", msg)
	e.RequiredRole = levelName[min]
	return e.with("your_role", p.Actor.Role)
}

// tokenSet is the answer to every successful sign-in.
type tokenSet struct {
	AccessToken      string         `json:"access_token"`
	TokenType        string         `json:"token_type"`
	ExpiresIn        int            `json:"expires_in"`
	RefreshToken     string         `json:"refresh_token"`
	RefreshExpiresIn int            `json:"refresh_expires_in"`
	SessionID        string         `json:"session_id"`
	User             map[string]any `json:"user"`
}

// startSession creates a session and its first refresh token inside tx.
func (s *Server) startSession(ctx context.Context, tx *sql.Tx, u *user, kind string) (*tokenSet, error) {
	sid := store.NewID("ses")
	now := s.unix()
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (id, user_id, kind, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		sid, u.ID, kind, now, now+int64(s.cfg.RefreshTokenTTL.Seconds())); err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, tx, u, sid)
}

func (s *Server) issueTokens(ctx context.Context, tx *sql.Tx, u *user, sid string) (*tokenSet, error) {
	refresh := "rt_" + store.RandomString(30)
	now := s.unix()
	if _, err := tx.ExecContext(ctx, `INSERT INTO refresh_tokens (token_hash, session_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		store.Hash(refresh), sid, now, now+int64(s.cfg.RefreshTokenTTL.Seconds())); err != nil {
		return nil, err
	}
	access, err := s.signAccessToken(u.ID, sid)
	if err != nil {
		return nil, err
	}
	return &tokenSet{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: int(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh, RefreshExpiresIn: int(s.cfg.RefreshTokenTTL.Seconds()),
		SessionID: sid, User: publicUser(u),
	}, nil
}

func endSessions(ctx context.Context, tx *sql.Tx, now int64, reason, where string, args ...any) error {
	_, err := tx.ExecContext(ctx, `UPDATE sessions SET ended_at = ?, ended_reason = ? WHERE ended_at IS NULL AND `+where,
		append([]any{now, reason}, args...)...)
	return err
}

// rotateRefresh swaps a refresh token for a new pair. A token that was
// already used is a sign of theft: the whole session ends.
func (s *Server) rotateRefresh(ctx context.Context, token string) (*tokenSet, *apiError, error) {
	var out *tokenSet
	var aerr *apiError
	err := s.store.Write(ctx, func(tx *sql.Tx) error {
		var sid string
		var expires int64
		var used sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT session_id, expires_at, used_at FROM refresh_tokens WHERE token_hash = ?`,
			store.Hash(token)).Scan(&sid, &expires, &used)
		if errors.Is(err, sql.ErrNoRows) {
			aerr = errf(http.StatusUnauthorized, "refresh_invalid", "The refresh token is not valid.")
			return nil
		}
		if err != nil {
			return err
		}
		now := s.unix()
		if used.Valid {
			if err := endSessions(ctx, tx, now, "refresh_reused", "id = ?", sid); err != nil {
				return err
			}
			aerr = errf(http.StatusUnauthorized, "refresh_reused",
				"This refresh token was already used. As a precaution the whole sign-in was ended; sign in again.").with("session_id", sid)
			return nil
		}
		if aerr = s.checkSession(ctx, sid); aerr != nil {
			return nil
		}
		if expires <= now {
			aerr = errf(http.StatusUnauthorized, "refresh_expired", "The refresh token has expired. Sign in again.")
			return nil
		}
		var uid string
		if err := tx.QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE id = ?`, sid).Scan(&uid); err != nil {
			return err
		}
		u, err := loadUser(ctx, tx, uid)
		if err != nil {
			return err
		}
		if u.DisabledAt.Valid {
			aerr = errf(http.StatusForbidden, "account_disabled", "This account was disabled by an administrator.")
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE refresh_tokens SET used_at = ? WHERE token_hash = ?`, now, store.Hash(token)); err != nil {
			return err
		}
		out, err = s.issueTokens(ctx, tx, u, sid)
		return err
	})
	return out, aerr, err
}

// checkPassword applies the lockout rule shared by every password form.
// It returns the user on success.
func (s *Server) checkPassword(ctx context.Context, email, password string) (*user, *apiError, error) {
	var u *user
	var aerr *apiError
	err := s.store.Write(ctx, func(tx *sql.Tx) error {
		var hash string
		var failed int
		var locked sql.NullInt64
		var uid string
		err := tx.QueryRowContext(ctx, `SELECT id, password_hash, failed_logins, locked_until FROM users WHERE email = ? COLLATE NOCASE`,
			trimmed(email)).Scan(&uid, &hash, &failed, &locked)
		if errors.Is(err, sql.ErrNoRows) {
			_ = bcryptCompare(dummyHash, password) // same timing as a real check
			aerr = errf(http.StatusUnauthorized, "invalid_credentials", "Wrong email or password.")
			return nil
		}
		if err != nil {
			return err
		}
		now := s.unix()
		if locked.Valid && locked.Int64 > now {
			aerr = errf(http.StatusTooManyRequests, "too_many_attempts", "Too many failed sign-ins. Try again later.")
			aerr.RetryAfter = int(locked.Int64 - now)
			return nil
		}
		if bcryptCompare(hash, password) != nil {
			failed++
			if failed >= s.cfg.LoginMaxAttempts {
				until := now + int64(s.cfg.LoginLockout.Seconds())
				_, err = tx.ExecContext(ctx, `UPDATE users SET failed_logins = 0, locked_until = ? WHERE id = ?`, until, uid)
				aerr = errf(http.StatusTooManyRequests, "too_many_attempts", "Too many failed sign-ins. Try again later.")
				aerr.RetryAfter = int(until - now)
				return err
			}
			_, err = tx.ExecContext(ctx, `UPDATE users SET failed_logins = ? WHERE id = ?`, failed, uid)
			aerr = errf(http.StatusUnauthorized, "invalid_credentials", "Wrong email or password.").
				with("attempts_left", s.cfg.LoginMaxAttempts-failed)
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET failed_logins = 0, locked_until = NULL WHERE id = ?`, uid); err != nil {
			return err
		}
		u, err = loadUser(ctx, tx, uid)
		if err != nil {
			return err
		}
		if u.DisabledAt.Valid {
			aerr = errf(http.StatusForbidden, "account_disabled", "This account was disabled by an administrator.")
			u = nil
		}
		return nil
	})
	return u, aerr, err
}
