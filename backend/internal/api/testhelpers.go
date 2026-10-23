package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

// --- the demo inbox ---

func addInbox(ctx context.Context, tx *sql.Tx, now int64, kind, recipient, subject, body string, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	m, _ := json.Marshal(meta)
	_, err := tx.ExecContext(ctx, `INSERT INTO inbox (kind, recipient, subject, body, meta, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		kind, recipient, subject, body, string(m), now)
	return err
}

type inboxMessage struct {
	ID        int64          `json:"id"`
	Kind      string         `json:"kind"`
	Recipient string         `json:"recipient"`
	Subject   string         `json:"subject"`
	Body      string         `json:"body"`
	Meta      map[string]any `json:"meta"`
	CreatedAt string         `json:"created_at"`
}

func (s *Server) readInbox(r *http.Request) ([]inboxMessage, error) {
	q := r.URL.Query()
	where, args := "1=1", []any{}
	if v := q.Get("recipient"); v != "" {
		where += " AND recipient = ? COLLATE NOCASE"
		args = append(args, v)
	}
	if v := q.Get("kind"); v != "" {
		where += " AND kind = ?"
		args = append(args, v)
	}
	limit := 50
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 && n <= 500 {
		limit = n
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT id, kind, recipient, subject, body, meta, created_at FROM inbox WHERE `+where+` ORDER BY id DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []inboxMessage{}
	for rows.Next() {
		var m inboxMessage
		var meta string
		var created int64
		if err := rows.Scan(&m.ID, &m.Kind, &m.Recipient, &m.Subject, &m.Body, &meta, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(meta), &m.Meta)
		m.CreatedAt = ts(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Server) inbox(w http.ResponseWriter, r *http.Request, _ *principal) error {
	msgs, err := s.readInbox(r)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": msgs})
	return nil
}

var inboxPage = template.Must(template.New("inbox").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="5"><title>Demo inbox · Test Pharmacy</title>
<style>
:root{--bg:#f6f7f9;--card:#fff;--text:#1d2329;--muted:#66707a;--line:#e2e6ea;--ok:#0b7a5a;--bad:#b42318;--code:#eef6f2}
@media (prefers-color-scheme:dark){:root{--bg:#14171a;--card:#1d2226;--text:#e6e9ec;--muted:#98a2ad;--line:#2c3338;--ok:#3ccf9a;--bad:#ff8a80;--code:#1f3a30}}
body{font-family:system-ui,sans-serif;background:var(--bg);color:var(--text);margin:0;padding:24px 16px}
main{max-width:860px;margin:0 auto}h1{font-size:22px;margin:0 0 4px}p.sub{color:var(--muted);margin:0 0 20px}
.msg{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:14px 16px;margin-bottom:10px}
.top{display:flex;gap:8px;flex-wrap:wrap;align-items:baseline;font-size:13px;color:var(--muted)}
.kind{font-weight:600;text-transform:uppercase;letter-spacing:.04em;font-size:11px;padding:2px 6px;border-radius:4px;background:var(--line);color:var(--text)}
.subj{font-weight:600;margin:6px 0 4px}.body{white-space:pre-wrap;word-break:break-word;font-size:14px}
.code{font-family:ui-monospace,monospace;font-size:26px;letter-spacing:.2em;background:var(--code);padding:4px 10px;border-radius:6px;display:inline-block;margin-top:6px}
.ok{color:var(--ok);font-weight:600}.bad{color:var(--bad);font-weight:600}
</style></head><body><main>
<h1>Demo inbox</h1>
<p class="sub">One-time codes, notices and webhook deliveries that would otherwise go to email, SMS or a server. Refreshes every 5 seconds.</p>
{{range .}}<div class="msg"><div class="top"><span class="kind">{{.Kind}}</span><span>to {{.Recipient}}</span><span>{{.CreatedAt}}</span>
{{with .Meta.verified}}<span class="ok">signature verified</span>{{end}}{{with .Meta.failure}}<span class="bad">{{.}}</span>{{end}}</div>
<div class="subj">{{.Subject}}</div>{{with .Meta.code}}<div class="code">{{.}}</div>{{else}}<div class="body">{{.Body}}</div>{{end}}</div>
{{else}}<p>Nothing yet. Ask for a one-time code with POST /auth/otp/request.</p>{{end}}
</main></body></html>`))

func (s *Server) inboxView(w http.ResponseWriter, r *http.Request, _ *principal) error {
	msgs, err := s.readInbox(r)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return inboxPage.Execute(w, msgs)
}

func (s *Server) clearInbox(w http.ResponseWriter, r *http.Request, _ *principal) error {
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `DELETE FROM inbox`)
		return err
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- reset ---

func (s *Server) reset(w http.ResponseWriter, r *http.Request, _ *principal) error {
	if s.cfg.TestResetToken != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Test-Token")), []byte(s.cfg.TestResetToken)) != 1 {
		return errf(http.StatusUnauthorized, "test_token_invalid", "Send the configured X-Test-Token to reset the data.")
	}
	if err := s.store.Reset(r.Context(), s.unix()); err != nil {
		return err
	}
	s.flakyMu.Lock()
	s.flaky = map[string]int{}
	s.flakyMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset", "at": ts(s.unix())})
	return nil
}

// --- failures on purpose ---

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var failMessages = map[int]string{
	400: "Simulated bad request.", 401: "Simulated: not signed in.", 403: "Simulated: wrong role.",
	404: "Simulated: record not found.", 409: "Simulated: state conflict.", 422: "Simulated: invalid fields.",
	429: "Simulated: too many requests.", 500: "Simulated server bug.", 502: "Simulated bad gateway.",
	503: "Simulated outage: the service is temporarily unavailable.", 504: "Simulated gateway timeout.",
}

func (s *Server) testFail(w http.ResponseWriter, r *http.Request, _ *principal) error {
	q := r.URL.Query()
	var v validator
	status := 503
	if st := q.Get("status"); st != "" {
		n, err := strconv.Atoi(st)
		v.check(err == nil && n >= 400 && n <= 599, "status", "out_of_range", "must be an HTTP error status, 400 to 599")
		status = n
	}
	var delay time.Duration
	if d := q.Get("delay"); d != "" {
		n, err := strconv.ParseFloat(d, 64)
		v.check(err == nil && n >= 0 && n <= 60, "delay", "out_of_range", "must be 0 to 60 seconds")
		delay = time.Duration(n * float64(time.Second))
	}
	mode := q.Get("mode")
	v.check(mode == "" || mode == "close", "mode", "invalid_enum", "must be close (drop the connection) or empty")
	if e := v.err(); e != nil {
		return e
	}
	if err := sleepCtx(r.Context(), delay); err != nil {
		return err
	}
	if key := q.Get("flaky_key"); key != "" {
		s.flakyMu.Lock()
		s.flaky[key]++
		calls := s.flaky[key]
		s.flakyMu.Unlock()
		if calls%2 == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "attempt": calls, "message": "Worked on the retry."})
			return nil
		}
	}
	if mode == "close" {
		if conn, _, err := http.NewResponseController(w).Hijack(); err == nil {
			_ = conn.Close()
			return nil
		}
		panic(http.ErrAbortHandler)
	}
	msg := failMessages[status]
	if msg == "" {
		msg = "Simulated failure."
	}
	e := errf(status, "simulated_failure", "%s", msg).with("simulated", true)
	if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		e.RetryAfter = 3
	}
	return e
}

func (s *Server) testDelay(w http.ResponseWriter, r *http.Request, _ *principal) error {
	n, err := strconv.ParseFloat(r.URL.Query().Get("seconds"), 64)
	if err != nil || n < 0 || n > 120 {
		var v validator
		v.add("seconds", "out_of_range", "must be 0 to 120")
		return v.err()
	}
	start := time.Now()
	if err := sleepCtx(r.Context(), time.Duration(n*float64(time.Second))); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"waited_seconds": time.Since(start).Seconds()})
	return nil
}

// --- webhooks ---

// WebhookSignature is how this API signs webhooks, and how the receiver
// below checks them: hex HMAC-SHA256 over "<timestamp>.<body>".
func WebhookSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// receiveWebhook proves a signature check end to end: it records every
// delivery in the inbox, verified or not, and answers 401 for a bad one.
func (s *Server) receiveWebhook(w http.ResponseWriter, r *http.Request, _ *principal) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		return err
	}
	stamp, sig := r.Header.Get("X-Webhook-Timestamp"), r.Header.Get("X-Webhook-Signature")
	failure := ""
	sec, perr := strconv.ParseInt(stamp, 10, 64)
	switch {
	case stamp == "" || sig == "":
		failure = "missing X-Webhook-Timestamp or X-Webhook-Signature"
	case perr != nil:
		failure = "X-Webhook-Timestamp is not unix seconds"
	case s.now().Sub(time.Unix(sec, 0)).Abs() > s.cfg.SignatureSkew:
		failure = "timestamp outside the allowed window (replay?)"
	case !hmac.Equal([]byte(WebhookSignature(s.cfg.WebhookSecret, stamp, body)), []byte(sig)):
		failure = "signature does not match"
	}
	meta := map[string]any{"verified": failure == "", "headers": map[string]string{
		"X-Webhook-Timestamp": stamp, "X-Webhook-Signature": sig, "Content-Type": r.Header.Get("Content-Type")}}
	if failure != "" {
		meta["verified"] = nil
		meta["failure"] = failure
	}
	var event struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(body, &event)
	subject := "Webhook received"
	if event.Type != "" {
		subject += ": " + event.Type
	}
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		return addInbox(r.Context(), tx, s.unix(), "webhook_received", r.URL.Path, subject, string(body), meta)
	})
	if err != nil {
		return err
	}
	if failure != "" {
		return errf(http.StatusUnauthorized, "webhook_signature_invalid", "Rejected: %s.", failure)
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": true, "verified": true})
	return nil
}

// emit sends an event to WEBHOOK_URL, signed, in the background. Without a
// URL it does nothing. Every attempt is written to the inbox.
func (s *Server) emit(ctx context.Context, eventType string, data map[string]any) {
	if s.cfg.WebhookURL == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{"id": store.NewID("evt"), "type": eventType, "created_at": ts(s.unix()), "data": data})
	go func() {
		ctx := context.WithoutCancel(ctx)
		stamp := strconv.FormatInt(s.unix(), 10)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.WebhookURL, bytes.NewReader(body))
		if err != nil {
			s.log.Error("webhook request", "err", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Timestamp", stamp)
		req.Header.Set("X-Webhook-Signature", WebhookSignature(s.cfg.WebhookSecret, stamp, body))
		meta := map[string]any{"url": s.cfg.WebhookURL}
		resp, err := s.client.Do(req)
		if err != nil {
			meta["failure"] = err.Error()
		} else {
			meta["status"] = resp.StatusCode
			resp.Body.Close()
		}
		_ = s.store.Write(ctx, func(tx *sql.Tx) error {
			return addInbox(ctx, tx, s.unix(), "webhook_sent", s.cfg.WebhookURL, "Webhook sent: "+eventType, string(body), meta)
		})
	}()
}

// --- health and settings ---

func (s *Server) health(w http.ResponseWriter, r *http.Request, _ *principal) error {
	status, code := "ok", http.StatusOK
	if err := s.store.DB.PingContext(r.Context()); err != nil {
		status, code = "degraded", http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{"status": status, "version": Version, "time": ts(s.unix()),
		"uptime_seconds": int(time.Since(s.started).Seconds())})
	return nil
}

// testConfig shows the settings a tester needs to know, never the secrets.
func (s *Server) testConfig(w http.ResponseWriter, r *http.Request, _ *principal) error {
	c := s.cfg
	writeJSON(w, http.StatusOK, map[string]any{
		"public_url": c.PublicURL, "access_token_ttl_seconds": int(c.AccessTokenTTL.Seconds()),
		"refresh_token_ttl_seconds": int(c.RefreshTokenTTL.Seconds()), "login_max_attempts": c.LoginMaxAttempts,
		"login_lockout_seconds": int(c.LoginLockout.Seconds()), "fixed_otp_code": c.FixedOTPCode != "",
		"oauth_client_id": c.OAuthClientID, "oauth_redirect_uris": c.OAuthRedirectURIs, "partner_client_id": c.PartnerClientID,
		"auto_refund_limit": c.AutoRefundLimit, "free_delivery_threshold": c.FreeDeliveryThreshold, "delivery_fee": c.DeliveryFee,
		"job_duration_seconds": c.JobDuration.Seconds(), "webhook_url_set": c.WebhookURL != "", "reset_needs_token": c.TestResetToken != "",
		"seed_accounts": seedAccounts(),
	})
	return nil
}

func seedAccounts() []map[string]any {
	var out []map[string]any
	for _, u := range store.SeedUsers {
		out = append(out, map[string]any{"id": u.ID, "email": u.Email, "phone": u.Phone, "password": u.Password, "role": u.Role,
			"disabled": u.Disabled, "api_key": u.APIKey})
	}
	return out
}
