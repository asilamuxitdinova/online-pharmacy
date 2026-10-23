package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/config"
	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

type env struct {
	t   *testing.T
	srv *Server
	now time.Time
}

func testConfig() config.Config {
	return config.Config{
		Addr: ":0", PublicURL: "https://pharmacy.test.local",
		JWTSecret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour,
		LoginMaxAttempts: 5, LoginLockout: 5 * time.Minute, OTPTTL: 5 * time.Minute, FixedOTPCode: "123456",
		OAuthClientID: "frege-test", OAuthClientSecret: "oauth-secret",
		PartnerClientID: "partner-demo", PartnerSecret: "partner-secret", SignatureSkew: 5 * time.Minute,
		WebhookSecret: "webhook-secret", AutoRefundLimit: 200_000, FreeDeliveryThreshold: 300_000, DeliveryFee: 25_000,
		JobDuration: 10 * time.Second, TestHelpers: true,
	}
}

func newEnv(t *testing.T, tweak ...func(*config.Config)) *env {
	t.Helper()
	cfg := testConfig()
	for _, f := range tweak {
		f(&cfg)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, now: time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)}
	if err := st.Reset(ctx, e.now.Unix()); err != nil {
		t.Fatal(err)
	}
	e.srv = New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.srv.Now = func() time.Time { return e.now }
	return e
}

func (e *env) advance(d time.Duration) { e.now = e.now.Add(d) }

type resp struct {
	t      *testing.T
	Status int
	Header http.Header
	Raw    []byte
	JSON   map[string]any
}

// call sends a request. body may be nil, a string (sent as is) or a value
// (sent as JSON). headers are pairs: "Name", "value", ...
func (e *env) call(method, path string, body any, headers ...string) *resp {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	r := &resp{t: e.t, Status: rec.Code, Header: rec.Header(), Raw: rec.Body.Bytes()}
	_ = json.Unmarshal(r.Raw, &r.JSON)
	return r
}

func (r *resp) expect(status int) *resp {
	r.t.Helper()
	if r.Status != status {
		r.t.Fatalf("status = %d, want %d; body: %s", r.Status, status, r.Raw)
	}
	return r
}

// errCode returns error.code from an error body.
func (r *resp) errCode() string {
	if e, ok := r.JSON["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

func (r *resp) expectErr(status int, code string) *resp {
	r.t.Helper()
	r.expect(status)
	if got := r.errCode(); got != code {
		r.t.Fatalf("error code = %q, want %q; body: %s", got, code, r.Raw)
	}
	return r
}

func (r *resp) str(path ...string) string {
	r.t.Helper()
	var cur any = r.JSON
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			r.t.Fatalf("no %v in %s", path, r.Raw)
		}
		cur = m[p]
	}
	switch v := cur.(type) {
	case string:
		return v
	case nil:
		return ""
	}
	raw, _ := json.Marshal(cur)
	return string(raw)
}

func (r *resp) num(path ...string) int64 {
	r.t.Helper()
	var cur any = r.JSON
	for _, p := range path {
		cur = cur.(map[string]any)[p]
	}
	f, ok := cur.(float64)
	if !ok {
		r.t.Fatalf("%v is not a number in %s", path, r.Raw)
	}
	return int64(f)
}

func (r *resp) list(key string) []any {
	r.t.Helper()
	l, ok := r.JSON[key].([]any)
	if !ok {
		r.t.Fatalf("%s is not a list in %s", key, r.Raw)
	}
	return l
}

// login signs in a seeded user and returns the Authorization header pair.
func (e *env) login(email, password string) (auth []string, tokens *resp) {
	e.t.Helper()
	r := e.call("POST", "/auth/login", map[string]string{"email": email, "password": password}).expect(200)
	return []string{"Authorization", "Bearer " + r.str("access_token")}, r
}

func (e *env) as(who string) []string {
	e.t.Helper()
	pw := map[string]string{"aziza": "Customer123!", "bekzod": "Customer123!", "dilnoza": "Staff123!", "admin": "Admin123!"}[who]
	h, _ := e.login(who+"@pharmacy.test", pw)
	return h
}

func hdr(base []string, more ...string) []string { return append(append([]string{}, base...), more...) }
