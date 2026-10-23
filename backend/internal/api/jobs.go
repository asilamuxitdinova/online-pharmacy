package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"hash/fnv"
	"net/http"
	"strconv"
	"time"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

// Jobs need no worker. A job stores when it started and how long it takes;
// its status and progress are worked out from the clock on every read. That
// makes it restart-proof and lets tests move time instead of sleeping.

var branches = []struct{ name, city string }{
	{"Chilonzor branch", "Tashkent"}, {"Yunusobod branch", "Tashkent"}, {"Mirzo Ulug'bek branch", "Tashkent"},
	{"Registan branch", "Samarkand"}, {"Old City branch", "Bukhara"}, {"Central branch", "Fergana"},
}

func (s *Server) startStockCheck(w http.ResponseWriter, r *http.Request, p *principal) error {
	var in struct {
		ProductID       string `json:"product_id"`
		SimulateFailure bool   `json:"simulate_failure"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	var v validator
	v.check(in.ProductID != "", "product_id", "required", "is required")
	if e := v.err(); e != nil {
		return e
	}
	if _, err := loadProduct(r.Context(), s.store.DB, in.ProductID); errors.Is(err, sql.ErrNoRows) {
		v.add("product_id", "not_found", "no product with id %q", in.ProductID)
		return v.err()
	} else if err != nil {
		return err
	}
	var fail *string
	if in.SimulateFailure {
		reason := "supplier_timeout: the Fergana warehouse did not answer within 30 seconds"
		fail = &reason
	}
	input, _ := json.Marshal(map[string]string{"product_id": in.ProductID})
	return s.startJob(w, r, p, "stock_check", string(input), fail)
}

func (s *Server) startSalesReport(w http.ResponseWriter, r *http.Request, p *principal) error {
	var in struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	var v validator
	from, err1 := time.Parse(time.DateOnly, in.From)
	to, err2 := time.Parse(time.DateOnly, in.To)
	v.check(err1 == nil, "from", "invalid_date", "must be a date like 2026-10-01")
	v.check(err2 == nil, "to", "invalid_date", "must be a date like 2026-10-31")
	if err1 == nil && err2 == nil {
		v.check(!to.Before(from), "to", "before_from", "must not be before from")
		v.check(to.Sub(from) <= 366*24*time.Hour, "to", "range_too_long", "the range can be at most one year")
	}
	if e := v.err(); e != nil {
		return e
	}
	input, _ := json.Marshal(map[string]string{"from": in.From, "to": in.To})
	return s.startJob(w, r, p, "sales_report", string(input), nil)
}

func (s *Server) startJob(w http.ResponseWriter, r *http.Request, p *principal, kind, input string, fail *string) error {
	id := store.NewID("job")
	nowMs := s.now().UnixMilli()
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `INSERT INTO jobs (id, user_id, kind, input, duration_ms, fail_reason, created_at_ms) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, p.User.ID, kind, input, s.cfg.JobDuration.Milliseconds(), fail, nowMs)
		return err
	})
	if err != nil {
		return err
	}
	w.Header().Set("Location", "/jobs/"+id)
	w.Header().Set("Retry-After", "2")
	return s.writeJob(w, r, id, http.StatusAccepted)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request, p *principal) error {
	var owner string
	err := s.store.DB.QueryRowContext(r.Context(), `SELECT user_id FROM jobs WHERE id = ?`, r.PathValue("id")).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && owner != p.User.ID && (p.ActingFor != nil || !p.is(staff))) {
		return notFound("job")
	}
	if err != nil {
		return err
	}
	return s.writeJob(w, r, r.PathValue("id"), http.StatusOK)
}

func (s *Server) writeJob(w http.ResponseWriter, r *http.Request, id string, status int) error {
	var kind, input string
	var durationMs, createdMs int64
	var fail sql.NullString
	if err := s.store.DB.QueryRowContext(r.Context(), `SELECT kind, input, duration_ms, fail_reason, created_at_ms FROM jobs WHERE id = ?`, id).
		Scan(&kind, &input, &durationMs, &fail, &createdMs); err != nil {
		return err
	}
	elapsed := s.now().UnixMilli() - createdMs
	out := map[string]any{
		"id": id, "kind": kind, "created_at": time.UnixMilli(createdMs).UTC().Format(time.RFC3339),
		"links": map[string]string{"self": "/jobs/" + id},
	}
	var params map[string]string
	_ = json.Unmarshal([]byte(input), &params)
	out["input"] = params

	switch {
	case elapsed < durationMs:
		out["status"] = "running"
		out["progress"] = int(elapsed * 100 / max(durationMs, 1))
		out["estimated_seconds_left"] = (durationMs - elapsed + 999) / 1000
		if status == http.StatusOK {
			w.Header().Set("Retry-After", "2")
		}
	case fail.Valid:
		// A failing job gives up two thirds of the way through.
		out["status"] = "failed"
		out["progress"] = 66
		out["error"] = map[string]string{"code": "job_failed", "message": fail.String}
		out["finished_at"] = time.UnixMilli(createdMs + durationMs).UTC().Format(time.RFC3339)
	default:
		out["status"] = "succeeded"
		out["progress"] = 100
		out["finished_at"] = time.UnixMilli(createdMs + durationMs).UTC().Format(time.RFC3339)
		result, err := s.jobResult(r, kind, params)
		if err != nil {
			return err
		}
		out["result"] = result
	}
	writeJSON(w, status, out)
	return nil
}

func (s *Server) jobResult(r *http.Request, kind string, params map[string]string) (any, error) {
	switch kind {
	case "stock_check":
		p, err := loadProduct(r.Context(), s.store.DB, params["product_id"])
		if err != nil {
			return nil, err
		}
		var rows []map[string]any
		total := 0
		for _, b := range branches {
			h := fnv.New32a()
			h.Write([]byte(p.ID + b.name))
			n := int(h.Sum32() % 25)
			if p.Stock == 0 {
				n = 0
			}
			total += n
			rows = append(rows, map[string]any{"branch": b.name, "city": b.city, "available": n})
		}
		return map[string]any{"product_id": p.ID, "name": p.name(locale(r)), "online_stock": p.Stock, "branches": rows, "branch_total": total}, nil
	case "sales_report":
		from, _ := time.Parse(time.DateOnly, params["from"])
		to, _ := time.Parse(time.DateOnly, params["to"])
		var orders int
		var revenue, refunded int64
		err := s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*), COALESCE(SUM(total), 0), COALESCE(SUM(refunded), 0) FROM orders
			WHERE paid_at IS NOT NULL AND paid_at >= ? AND paid_at < ?`, from.Unix(), to.AddDate(0, 0, 1).Unix()).Scan(&orders, &revenue, &refunded)
		if err != nil {
			return nil, err
		}
		return map[string]any{"from": params["from"], "to": params["to"], "paid_orders": orders, "revenue": revenue,
			"refunded": refunded, "net": revenue - refunded, "currency": currency}, nil
	}
	return nil, errors.New("unknown job kind " + strconv.Quote(kind))
}
