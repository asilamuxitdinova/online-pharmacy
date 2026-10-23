package api

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

func bcryptCompare(hash any, password string) error {
	var h []byte
	switch v := hash.(type) {
	case string:
		h = []byte(v)
	case []byte:
		h = v
	}
	return bcrypt.CompareHashAndPassword(h, []byte(password))
}

var phoneRe = regexp.MustCompile(`^\+998\d{9}$`)

func validEmail(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s && strings.Contains(s, ".")
}

func validPassword(v *validator, field, pw string) {
	v.check(len(pw) >= 8, field, "too_short", "must be at least 8 characters")
	v.check(strings.ContainsAny(pw, "0123456789"), field, "needs_digit", "must contain a digit")
	v.check(strings.ToLower(pw) != pw, field, "needs_uppercase", "must contain an uppercase letter")
}

func (s *Server) register(w http.ResponseWriter, r *http.Request, _ *principal) error {
	var req struct {
		Email    string  `json:"email"`
		Password string  `json:"password"`
		Name     string  `json:"name"`
		Phone    *string `json:"phone"`
	}
	if e := decode(r, &req); e != nil {
		return e
	}
	req.Email, req.Name = strings.ToLower(trimmed(req.Email)), trimmed(req.Name)
	var v validator
	v.check(validEmail(req.Email), "email", "invalid_email", "must be an email address like name@example.com")
	validPassword(&v, "password", req.Password)
	v.check(len(req.Name) >= 2 && len(req.Name) <= 80, "name", "invalid_length", "must be 2 to 80 characters")
	if req.Phone != nil {
		v.check(phoneRe.MatchString(*req.Phone), "phone", "invalid_phone", "must look like +998901234567")
	}
	if e := v.err(); e != nil {
		return e
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	var tokens *tokenSet
	var aerr *apiError
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users WHERE email = ? OR (phone IS NOT NULL AND phone = ?)`,
			req.Email, req.Phone).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			aerr = errf(http.StatusConflict, "account_exists", "An account with this email or phone already exists. Sign in instead.")
			return nil
		}
		u := &user{ID: store.NewID("usr"), Email: req.Email, Name: req.Name, Role: "customer", CreatedAt: s.unix()}
		if req.Phone != nil {
			u.Phone = sql.NullString{String: *req.Phone, Valid: true}
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO users (id, email, phone, name, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?, 'customer', ?)`,
			u.ID, u.Email, u.Phone, u.Name, string(hash), u.CreatedAt); err != nil {
			return err
		}
		tokens, err = s.startSession(r.Context(), tx, u, "register")
		return err
	})
	if err != nil {
		return err
	}
	if aerr != nil {
		return aerr
	}
	writeJSON(w, http.StatusCreated, tokens)
	return nil
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c credentials) validate() *apiError {
	var v validator
	v.check(trimmed(c.Email) != "", "email", "required", "is required")
	v.check(c.Password != "", "password", "required", "is required")
	return v.err()
}

func (s *Server) login(w http.ResponseWriter, r *http.Request, _ *principal) error {
	var req credentials
	if e := decode(r, &req); e != nil {
		return e
	}
	if e := req.validate(); e != nil {
		return e
	}
	u, aerr, err := s.checkPassword(r.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}
	if aerr != nil {
		return aerr
	}
	var tokens *tokenSet
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		tokens, err = s.startSession(r.Context(), tx, u, "password")
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, tokens)
	return nil
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request, _ *principal) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if e := decode(r, &req); e != nil {
		return e
	}
	var v validator
	v.check(req.RefreshToken != "", "refresh_token", "required", "is required")
	if e := v.err(); e != nil {
		return e
	}
	tokens, aerr, err := s.rotateRefresh(r.Context(), req.RefreshToken)
	if err != nil {
		return err
	}
	if aerr != nil {
		return aerr
	}
	writeJSON(w, http.StatusOK, tokens)
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req struct {
		AllSessions bool `json:"all_sessions"`
	}
	if r.ContentLength != 0 {
		if e := decode(r, &req); e != nil {
			return e
		}
	}
	if !req.AllSessions && p.SessionID == "" {
		return errf(http.StatusBadRequest, "no_session", "An API key is not a session. Send {\"all_sessions\": true} to end every sign-in, or revoke the key with DELETE /me/api-keys/{id}.")
	}
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		if req.AllSessions {
			return endSessions(r.Context(), tx, s.unix(), "logout_all", "user_id = ?", p.Actor.ID)
		}
		return endSessions(r.Context(), tx, s.unix(), "logout", "id = ?", p.SessionID)
	})
	if err != nil {
		return err
	}
	if p.Method == "cookie" {
		s.clearCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- session cookie variant ---

func (s *Server) createCookieSession(w http.ResponseWriter, r *http.Request, _ *principal) error {
	var req credentials
	if e := decode(r, &req); e != nil {
		return e
	}
	if e := req.validate(); e != nil {
		return e
	}
	u, aerr, err := s.checkPassword(r.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}
	if aerr != nil {
		return aerr
	}
	value := "cs_" + store.RandomString(30)
	sid := store.NewID("ses")
	now := s.unix()
	expires := now + int64(s.cfg.RefreshTokenTTL.Seconds())
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `INSERT INTO sessions (id, user_id, kind, cookie_hash, created_at, expires_at) VALUES (?, ?, 'cookie', ?, ?, ?)`,
			sid, u.ID, store.Hash(value), now, expires)
		return err
	})
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: value, Path: "/", HttpOnly: true, Secure: s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode, Expires: time.Unix(expires, 0),
	})
	writeJSON(w, http.StatusOK, map[string]any{"session_id": sid, "expires_at": ts(expires), "user": publicUser(u)})
	return nil
}

func (s *Server) deleteCookieSession(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.Method != "cookie" {
		return errf(http.StatusBadRequest, "not_a_cookie_session", "This ends a cookie session; you are signed in with %s. Use POST /auth/logout instead.", p.Method)
	}
	return s.logout(w, r, p)
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.CookieSecure})
}

// --- one-time codes ---

const otpMaxAttempts = 5
const otpResendWait = 30 // seconds

func maskDestination(d string) string {
	if at := strings.IndexByte(d, '@'); at > 0 {
		return d[:1] + strings.Repeat("*", max(at-1, 1)) + d[at:]
	}
	if len(d) > 4 {
		return strings.Repeat("*", len(d)-4) + d[len(d)-4:]
	}
	return "****"
}

func (s *Server) requestOTP(w http.ResponseWriter, r *http.Request, _ *principal) error {
	var req struct {
		Channel     string `json:"channel"`
		Destination string `json:"destination"`
	}
	if e := decode(r, &req); e != nil {
		return e
	}
	req.Destination = trimmed(req.Destination)
	var v validator
	switch req.Channel {
	case "email":
		req.Destination = strings.ToLower(req.Destination)
		v.check(validEmail(req.Destination), "destination", "invalid_email", "must be an email address when channel is email")
	case "sms":
		v.check(phoneRe.MatchString(req.Destination), "destination", "invalid_phone", "must look like +998901234567 when channel is sms")
	default:
		v.add("channel", "invalid_enum", "must be one of: email, sms")
	}
	if e := v.err(); e != nil {
		return e
	}

	now := s.unix()
	var aerr *apiError
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		var last sql.NullInt64
		if err := tx.QueryRowContext(r.Context(), `SELECT MAX(created_at) FROM otp_codes WHERE destination = ?`, req.Destination).Scan(&last); err != nil {
			return err
		}
		if last.Valid && now-last.Int64 < otpResendWait {
			aerr = errf(http.StatusTooManyRequests, "too_many_attempts", "A code was sent moments ago. Wait before asking for another.")
			aerr.RetryAfter = int(otpResendWait - (now - last.Int64))
			return nil
		}
		col := "email"
		if req.Channel == "sms" {
			col = "phone"
		}
		var n int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users WHERE `+col+` = ? COLLATE NOCASE`, req.Destination).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			// Same answer as success, so the endpoint cannot be used to find accounts.
			_, err := tx.ExecContext(r.Context(), `INSERT INTO otp_codes (id, destination, code, attempts, expires_at, used_at, created_at) VALUES (?, ?, '', 0, ?, ?, ?)`,
				store.NewID("otp"), req.Destination, now, now, now)
			if err == nil {
				err = addInbox(r.Context(), tx, now, "otp", req.Destination, "Sign-in code requested",
					"No account uses this address, so no code was sent.", nil)
			}
			return err
		}
		code := s.cfg.FixedOTPCode
		if code == "" {
			nb, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
			if err != nil {
				return err
			}
			code = fmt.Sprintf("%06d", nb.Int64())
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO otp_codes (id, destination, code, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
			store.NewID("otp"), req.Destination, code, now+int64(s.cfg.OTPTTL.Seconds()), now); err != nil {
			return err
		}
		return addInbox(r.Context(), tx, now, "otp", req.Destination, "Your pharmacy sign-in code",
			fmt.Sprintf("Your code is %s. It expires in %d minutes.", code, int(s.cfg.OTPTTL.Minutes())), map[string]any{"code": code, "channel": req.Channel})
	})
	if err != nil {
		return err
	}
	if aerr != nil {
		return aerr
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status": "sent", "channel": req.Channel, "destination_hint": maskDestination(req.Destination),
		"expires_in": int(s.cfg.OTPTTL.Seconds()), "resend_after": otpResendWait,
	})
	return nil
}

func (s *Server) verifyOTP(w http.ResponseWriter, r *http.Request, _ *principal) error {
	var req struct {
		Destination string `json:"destination"`
		Code        string `json:"code"`
	}
	if e := decode(r, &req); e != nil {
		return e
	}
	req.Destination = trimmed(req.Destination)
	if strings.Contains(req.Destination, "@") {
		req.Destination = strings.ToLower(req.Destination)
	}
	var v validator
	v.check(req.Destination != "", "destination", "required", "is required")
	v.check(regexp.MustCompile(`^\d{6}$`).MatchString(req.Code), "code", "invalid_format", "must be 6 digits")
	if e := v.err(); e != nil {
		return e
	}
	var tokens *tokenSet
	var aerr *apiError
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		ctx := r.Context()
		var id, code string
		var attempts int
		var expires int64
		var used sql.NullInt64
		err := tx.QueryRowContext(ctx, `SELECT id, code, attempts, expires_at, used_at FROM otp_codes
			WHERE destination = ? ORDER BY created_at DESC, rowid DESC LIMIT 1`, req.Destination).Scan(&id, &code, &attempts, &expires, &used)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && used.Valid) {
			aerr = errf(http.StatusUnauthorized, "code_invalid", "No active code for this destination. Ask for a new one.")
			return nil
		}
		if err != nil {
			return err
		}
		now := s.unix()
		if expires <= now {
			aerr = errf(http.StatusUnauthorized, "code_expired", "The code has expired. Ask for a new one.")
			return nil
		}
		if attempts >= otpMaxAttempts {
			aerr = errf(http.StatusTooManyRequests, "too_many_attempts", "Too many wrong codes. Ask for a new one.")
			aerr.RetryAfter = otpResendWait
			return nil
		}
		if code != req.Code {
			attempts++
			_, err := tx.ExecContext(ctx, `UPDATE otp_codes SET attempts = ? WHERE id = ?`, attempts, id)
			aerr = errf(http.StatusUnauthorized, "code_invalid", "That code is wrong.").with("attempts_left", otpMaxAttempts-attempts)
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE otp_codes SET used_at = ? WHERE id = ?`, now, id); err != nil {
			return err
		}
		u, err := scanUser(tx.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE email = ? COLLATE NOCASE OR phone = ?`, req.Destination, req.Destination))
		if err != nil {
			return err
		}
		if u.DisabledAt.Valid {
			aerr = errf(http.StatusForbidden, "account_disabled", "This account was disabled by an administrator.")
			return nil
		}
		tokens, err = s.startSession(ctx, tx, u, "otp")
		return err
	})
	if err != nil {
		return err
	}
	if aerr != nil {
		return aerr
	}
	writeJSON(w, http.StatusOK, tokens)
	return nil
}

// --- who am I ---

func (s *Server) me(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := publicUser(p.User)
	out["auth_method"] = p.Method
	if p.SessionID != "" {
		out["session_id"] = p.SessionID
	}
	if p.ActingFor != nil {
		out["acting_as"] = true
		out["actor"] = map[string]any{"id": p.Actor.ID, "name": p.Actor.Name, "role": p.Actor.Role}
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req struct {
		Name  *string `json:"name"`
		Phone *string `json:"phone"`
	}
	if e := decode(r, &req); e != nil {
		return e
	}
	var v validator
	v.check(req.Name != nil || req.Phone != nil, "(body)", "empty_patch", "send at least one of: name, phone")
	if req.Name != nil {
		*req.Name = trimmed(*req.Name)
		v.check(len(*req.Name) >= 2 && len(*req.Name) <= 80, "name", "invalid_length", "must be 2 to 80 characters")
	}
	if req.Phone != nil {
		v.check(phoneRe.MatchString(*req.Phone), "phone", "invalid_phone", "must look like +998901234567")
	}
	if e := v.err(); e != nil {
		return e
	}
	var aerr *apiError
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		if req.Phone != nil {
			var n int
			if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users WHERE phone = ? AND id <> ?`, *req.Phone, p.User.ID).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				aerr = errf(http.StatusConflict, "phone_taken", "Another account already uses this phone number.")
				return nil
			}
		}
		_, err := tx.ExecContext(r.Context(), `UPDATE users SET name = COALESCE(?, name), phone = COALESCE(?, phone) WHERE id = ?`, req.Name, req.Phone, p.User.ID)
		return err
	})
	if err != nil {
		return err
	}
	if aerr != nil {
		return aerr
	}
	u, err := loadUser(r.Context(), s.store.DB, p.User.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, publicUser(u))
	return nil
}

// --- API keys ---

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request, p *principal) error {
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT id, name, prefix, created_at, revoked_at FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`, p.User.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, prefix string
		var created int64
		var revoked sql.NullInt64
		if err := rows.Scan(&id, &name, &prefix, &created, &revoked); err != nil {
			return err
		}
		out = append(out, map[string]any{"id": id, "name": name, "prefix": prefix, "created_at": ts(created), "revoked_at": tsPtr(revoked)})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
	return nil
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req struct {
		Name string `json:"name"`
	}
	if e := decode(r, &req); e != nil {
		return e
	}
	req.Name = trimmed(req.Name)
	var v validator
	v.check(len(req.Name) >= 1 && len(req.Name) <= 60, "name", "invalid_length", "must be 1 to 60 characters")
	if e := v.err(); e != nil {
		return e
	}
	key := "phk_test_" + strings.ToLower(store.RandomString(20))
	id := store.NewID("key")
	now := s.unix()
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `INSERT INTO api_keys (id, user_id, name, prefix, key_hash, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			id, p.User.ID, req.Name, key[:12], store.Hash(key), now)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": id, "name": req.Name, "prefix": key[:12], "created_at": ts(now), "key": key,
		"notice": "This is the only time the full key is shown. Store it now.",
	})
	return nil
}

func (s *Server) revokeAPIKey(w http.ResponseWriter, r *http.Request, p *principal) error {
	var found bool
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		res, err := tx.ExecContext(r.Context(), `UPDATE api_keys SET revoked_at = COALESCE(revoked_at, ?) WHERE id = ? AND user_id = ?`,
			s.unix(), r.PathValue("id"), p.User.ID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		found = n > 0
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return notFound("api_key")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
