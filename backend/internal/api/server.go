// Package api is the HTTP surface of the test pharmacy.
package api

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/config"
	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

var Version = "dev"

type Server struct {
	cfg     config.Config
	store   *store.Store
	log     *slog.Logger
	mux     *http.ServeMux
	table   []Route
	started time.Time

	// Now is the clock. Tests replace it to move time forward.
	Now func() time.Time

	flakyMu sync.Mutex
	flaky   map[string]int // /_test/fail?flaky_key= → calls seen

	client *http.Client // outgoing webhooks
}

func New(cfg config.Config, st *store.Store, log *slog.Logger) *Server {
	s := &Server{
		cfg: cfg, store: st, log: log, mux: http.NewServeMux(),
		started: time.Now(), Now: time.Now, flaky: map[string]int{},
		client: &http.Client{Timeout: 10 * time.Second},
	}
	s.routes()
	return s
}

func (s *Server) now() time.Time { return s.Now().UTC() }
func (s *Server) unix() int64    { return s.now().Unix() }

// access levels, lowest first.
type level int

const (
	public level = iota
	customer
	staff
	admin
)

var levelName = map[level]string{customer: "customer", staff: "staff", admin: "admin"}
var roleLevel = map[string]level{"customer": customer, "staff": staff, "admin": admin}

type handlerFunc func(w http.ResponseWriter, r *http.Request, p *principal) error

// handle wraps a handler with authentication, the role check and error
// writing. The role is read from the database on every request, never from
// the token, so a role change takes effect on the next call.
func (s *Server) handle(min level, h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p *principal
		if min > public {
			var aerr *apiError
			p, aerr = s.authenticate(r)
			if aerr == nil {
				aerr = s.authorize(r, p, min)
			}
			if aerr != nil {
				writeError(w, r, aerr)
				return
			}
			if p.ActingFor != nil {
				w.Header().Set("X-Acting-As", p.User.ID)
			}
		}
		if err := h(w, r, p); err != nil {
			s.fail(w, r, err)
		}
	})
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var aerr *apiError
	if errors.As(err, &aerr) {
		writeError(w, r, aerr)
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, r, errf(http.StatusInternalServerError, "internal_error", "Something went wrong on our side. The failure was logged."))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := r.Header.Get("X-Request-Id")
	if id == "" {
		id = store.NewID("req")
	}
	w.Header().Set("X-Request-Id", id)
	w.Header().Set("Content-Language", locale(r))
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		if v := recover(); v != nil {
			if v == http.ErrAbortHandler {
				panic(v)
			}
			s.log.Error("panic", "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
			writeError(rec, r, errf(http.StatusInternalServerError, "internal_error", "Something went wrong on our side. The failure was logged."))
		}
		s.log.Info("request", "id", id, "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "ms", time.Since(start).Milliseconds())
	}()
	s.mux.ServeHTTP(rec, r)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// notFoundHandler answers unknown paths in the API's own error shape.
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, errf(http.StatusNotFound, "route_not_found", "No endpoint %s %s. See /openapi.yaml for the list.", r.Method, r.URL.Path))
}

func ts(unix int64) string { return time.Unix(unix, 0).UTC().Format(time.RFC3339) }

func tsPtr(v sql.NullInt64) *string {
	if !v.Valid {
		return nil
	}
	s := ts(v.Int64)
	return &s
}

func nullStr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func trimmed(s string) string { return strings.TrimSpace(s) }
