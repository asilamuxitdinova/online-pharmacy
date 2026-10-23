package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

// --- addresses ---

func (s *Server) listAddresses(w http.ResponseWriter, r *http.Request, p *principal) error {
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT id, label, line1, city, phone, created_at FROM addresses WHERE user_id = ? ORDER BY created_at`, p.User.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, label, line1, city, phone string
		var created int64
		if err := rows.Scan(&id, &label, &line1, &city, &phone, &created); err != nil {
			return err
		}
		out = append(out, map[string]any{"id": id, "label": label, "line1": line1, "city": city, "phone": phone, "created_at": ts(created)})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
	return nil
}

var cities = []string{"Tashkent", "Samarkand", "Bukhara", "Andijan", "Namangan", "Fergana", "Nukus", "Khiva"}

func (s *Server) addAddress(w http.ResponseWriter, r *http.Request, p *principal) error {
	var in struct {
		Label string `json:"label"`
		Line1 string `json:"line1"`
		City  string `json:"city"`
		Phone string `json:"phone"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	in.Label, in.Line1 = trimmed(in.Label), trimmed(in.Line1)
	var v validator
	v.check(len(in.Label) >= 1 && len(in.Label) <= 40, "label", "invalid_length", "must be 1 to 40 characters")
	v.check(len(in.Line1) >= 5 && len(in.Line1) <= 200, "line1", "invalid_length", "must be 5 to 200 characters")
	okCity := false
	for _, c := range cities {
		okCity = okCity || c == in.City
	}
	v.check(okCity, "city", "invalid_enum", "must be one of the cities we deliver to: %s", strings.Join(cities, ", "))
	v.check(phoneRe.MatchString(in.Phone), "phone", "invalid_phone", "must look like +998901234567")
	if e := v.err(); e != nil {
		return e
	}
	id := store.NewID("adr")
	now := s.unix()
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `INSERT INTO addresses (id, user_id, label, line1, city, phone, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, p.User.ID, in.Label, in.Line1, in.City, in.Phone, now)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "label": in.Label, "line1": in.Line1, "city": in.City, "phone": in.Phone, "created_at": ts(now)})
	return nil
}

func (s *Server) deleteAddress(w http.ResponseWriter, r *http.Request, p *principal) error {
	var n int
	if err := s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM orders WHERE address_id = ? AND status = 'pending_payment'`, r.PathValue("id")).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errf(http.StatusConflict, "address_in_use", "An order waiting for payment uses this address. Change or cancel that order first.")
	}
	return s.deleteOwned(w, r, "addresses", "address", p.User.ID)
}

// --- prescriptions ---

type prescription struct {
	ID, UserID, Doctor, Clinic, IssuedOn, Diagnosis, Status string
	Note, ReviewedBy                                        sql.NullString
	CreatedAt                                               int64
}

const prescriptionColumns = `id, user_id, doctor_name, clinic, issued_on, diagnosis, status, review_note, reviewed_by, created_at`

func scanPrescription(row interface{ Scan(...any) error }) (*prescription, error) {
	var x prescription
	err := row.Scan(&x.ID, &x.UserID, &x.Doctor, &x.Clinic, &x.IssuedOn, &x.Diagnosis, &x.Status, &x.Note, &x.ReviewedBy, &x.CreatedAt)
	return &x, err
}

func (x *prescription) json() map[string]any {
	return map[string]any{"id": x.ID, "user_id": x.UserID, "doctor_name": x.Doctor, "clinic": x.Clinic, "issued_on": x.IssuedOn,
		"diagnosis": x.Diagnosis, "status": x.Status, "review_note": nullStr(x.Note), "reviewed_by": nullStr(x.ReviewedBy), "created_at": ts(x.CreatedAt)}
}

func (s *Server) queryPrescriptions(r *http.Request, where string, args ...any) ([]map[string]any, error) {
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT `+prescriptionColumns+` FROM prescriptions WHERE `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		x, err := scanPrescription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x.json())
	}
	return out, rows.Err()
}

func (s *Server) listPrescriptions(w http.ResponseWriter, r *http.Request, p *principal) error {
	out, err := s.queryPrescriptions(r, "user_id = ?", p.User.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
	return nil
}

func (s *Server) getPrescription(w http.ResponseWriter, r *http.Request, p *principal) error {
	x, err := scanPrescription(s.store.DB.QueryRowContext(r.Context(), `SELECT `+prescriptionColumns+` FROM prescriptions WHERE id = ?`, r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && x.UserID != p.User.ID && (p.ActingFor != nil || !p.is(staff))) {
		return notFound("prescription")
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, x.json())
	return nil
}

func (s *Server) addPrescription(w http.ResponseWriter, r *http.Request, p *principal) error {
	var in struct {
		DoctorName string `json:"doctor_name"`
		Clinic     string `json:"clinic"`
		IssuedOn   string `json:"issued_on"`
		Diagnosis  string `json:"diagnosis"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	var v validator
	v.check(len(trimmed(in.DoctorName)) >= 3, "doctor_name", "invalid_length", "must be at least 3 characters")
	v.check(len(trimmed(in.Clinic)) >= 3, "clinic", "invalid_length", "must be at least 3 characters")
	v.check(len(trimmed(in.Diagnosis)) >= 3 && len(in.Diagnosis) <= 300, "diagnosis", "invalid_length", "must be 3 to 300 characters")
	issued, err := time.Parse(time.DateOnly, in.IssuedOn)
	if err != nil {
		v.add("issued_on", "invalid_date", "must be a date like 2026-10-01")
	} else {
		today := s.now().Truncate(24 * time.Hour)
		v.check(!issued.After(today), "issued_on", "in_future", "cannot be in the future")
		v.check(issued.After(today.AddDate(-1, 0, 0)), "issued_on", "too_old", "a prescription is valid for one year; this one has expired")
	}
	if e := v.err(); e != nil {
		return e
	}
	id := store.NewID("rx")
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `INSERT INTO prescriptions (id, user_id, doctor_name, clinic, issued_on, diagnosis, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, 'pending_review', ?)`, id, p.User.ID, trimmed(in.DoctorName), trimmed(in.Clinic), in.IssuedOn, trimmed(in.Diagnosis), s.unix())
		return err
	})
	if err != nil {
		return err
	}
	x, err := scanPrescription(s.store.DB.QueryRowContext(r.Context(), `SELECT `+prescriptionColumns+` FROM prescriptions WHERE id = ?`, id))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, x.json())
	return nil
}

func (s *Server) staffPrescriptions(w http.ResponseWriter, r *http.Request, _ *principal) error {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending_review"
	}
	var v validator
	v.check(status == "pending_review" || status == "approved" || status == "rejected", "status", "invalid_enum", "must be one of: pending_review, approved, rejected")
	if e := v.err(); e != nil {
		return e
	}
	out, err := s.queryPrescriptions(r, "status = ?", status)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
	return nil
}

func (s *Server) reviewPrescription(w http.ResponseWriter, r *http.Request, p *principal) error {
	var in struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	var v validator
	v.check(in.Decision == "approve" || in.Decision == "reject", "decision", "invalid_enum", "must be approve or reject")
	v.check(in.Decision != "reject" || len(trimmed(in.Note)) >= 3, "note", "required", "is required when rejecting, so the customer knows why")
	if e := v.err(); e != nil {
		return e
	}
	id := r.PathValue("id")
	var aerr *apiError
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		var status string
		err := tx.QueryRowContext(r.Context(), `SELECT status FROM prescriptions WHERE id = ?`, id).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			aerr = notFound("prescription")
			return nil
		}
		if err != nil {
			return err
		}
		if status != "pending_review" {
			aerr = errf(http.StatusConflict, "prescription_already_reviewed", "This prescription was already %s.", status).with("status", status)
			return nil
		}
		newStatus := map[string]string{"approve": "approved", "reject": "rejected"}[in.Decision]
		_, err = tx.ExecContext(r.Context(), `UPDATE prescriptions SET status = ?, review_note = ?, reviewed_by = ? WHERE id = ?`, newStatus, trimmed(in.Note), p.Actor.ID, id)
		return err
	})
	if err != nil {
		return err
	}
	if aerr != nil {
		return aerr
	}
	x, err := scanPrescription(s.store.DB.QueryRowContext(r.Context(), `SELECT `+prescriptionColumns+` FROM prescriptions WHERE id = ?`, id))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, x.json())
	return nil
}

// --- staff ---

func (s *Server) staffOrders(w http.ResponseWriter, r *http.Request, _ *principal) error {
	q := r.URL.Query()
	var v validator
	page, size := pageParams(r, &v, 20, 100)
	where, args := "1=1", []any{}
	if st := q.Get("status"); st != "" {
		validStatus(&v, st)
		where += " AND status = ?"
		args = append(args, st)
	}
	if u := q.Get("user_id"); u != "" {
		where += " AND user_id = ?"
		args = append(args, u)
	}
	if e := v.err(); e != nil {
		return e
	}
	var total int
	if err := s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM orders WHERE `+where, args...).Scan(&total); err != nil {
		return err
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT id FROM orders WHERE `+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		append(args, size, (page-1)*size)...)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	lang := locale(r)
	out := []map[string]any{}
	for _, id := range ids {
		o, err := loadOrder(r.Context(), s.store.DB, id)
		if err != nil {
			return err
		}
		out = append(out, o.json(lang))
	}
	writeJSON(w, http.StatusOK, paged(out, page, size, total))
	return nil
}

// staffUser includes the taxpayer number: staff-only data that redaction
// rules in Frege are meant to scrub.
func staffUser(u *user) map[string]any {
	out := publicUser(u)
	out["tin"] = nullStr(u.TIN)
	out["disabled"] = u.DisabledAt.Valid
	out["disabled_at"] = tsPtr(u.DisabledAt)
	return out
}

func (s *Server) listUsers(roles []string) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request, _ *principal) error {
		q := r.URL.Query()
		var v validator
		page, size := pageParams(r, &v, 20, 100)
		where := "role IN (" + strings.TrimSuffix(strings.Repeat("?,", len(roles)), ",") + ")"
		var args []any
		for _, role := range roles {
			args = append(args, role)
		}
		if role := q.Get("role"); role != "" {
			_, ok := roleLevel[role]
			v.check(ok, "role", "invalid_enum", "must be one of: customer, staff, admin")
			where += " AND role = ?"
			args = append(args, role)
		}
		if text := strings.ToLower(trimmed(q.Get("q"))); text != "" {
			where += " AND (lower(name) LIKE ? OR lower(email) LIKE ? OR phone LIKE ?)"
			args = append(args, "%"+text+"%", "%"+text+"%", "%"+text+"%")
		}
		if e := v.err(); e != nil {
			return e
		}
		var total int
		if err := s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users WHERE `+where, args...).Scan(&total); err != nil {
			return err
		}
		rows, err := s.store.DB.QueryContext(r.Context(), `SELECT `+userColumns+` FROM users WHERE `+where+` ORDER BY created_at, id LIMIT ? OFFSET ?`,
			append(args, size, (page-1)*size)...)
		if err != nil {
			return err
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			u, err := scanUser(rows)
			if err != nil {
				return err
			}
			out = append(out, staffUser(u))
		}
		if err := rows.Err(); err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, paged(out, page, size, total))
		return nil
	}
}

func (s *Server) getCustomer(w http.ResponseWriter, r *http.Request, _ *principal) error {
	u, err := loadUser(r.Context(), s.store.DB, r.PathValue("id"))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && u.Role != "customer") {
		return notFound("customer")
	}
	if err != nil {
		return err
	}
	out := staffUser(u)
	var orders, prescriptions int
	var spent int64
	_ = s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*), COALESCE(SUM(CASE WHEN paid_at IS NOT NULL THEN total - refunded END), 0) FROM orders WHERE user_id = ?`, u.ID).Scan(&orders, &spent)
	_ = s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM prescriptions WHERE user_id = ?`, u.ID).Scan(&prescriptions)
	out["order_count"], out["prescription_count"], out["lifetime_spent"] = orders, prescriptions, spent
	writeJSON(w, http.StatusOK, out)
	return nil
}

// --- admin ---

func (s *Server) adminTarget(r *http.Request, p *principal) (*user, *apiError, error) {
	u, err := loadUser(r.Context(), s.store.DB, r.PathValue("id"))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("user"), nil
	}
	if err != nil {
		return nil, nil, err
	}
	if u.ID == p.Actor.ID {
		return nil, errf(http.StatusConflict, "cannot_modify_self", "Admins cannot change, disable or delete their own account. Ask another admin."), nil
	}
	return u, nil, nil
}

func (s *Server) changeRole(w http.ResponseWriter, r *http.Request, p *principal) error {
	var in struct {
		Role string `json:"role"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	var v validator
	_, ok := roleLevel[in.Role]
	v.check(ok, "role", "invalid_enum", "must be one of: customer, staff, admin")
	if e := v.err(); e != nil {
		return e
	}
	u, aerr, err := s.adminTarget(r, p)
	if err != nil || aerr != nil {
		return firstErr(err, aerr)
	}
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `UPDATE users SET role = ? WHERE id = ?`, in.Role, u.ID); err != nil {
			return err
		}
		return addInbox(r.Context(), tx, s.unix(), "notice", u.Email, "Your role changed",
			"An administrator changed your role from "+u.Role+" to "+in.Role+".", map[string]any{"by": p.Actor.ID})
	})
	if err != nil {
		return err
	}
	return s.writeUser(w, r, u.ID, map[string]any{"previous_role": u.Role})
}

func (s *Server) setDisabled(disable bool) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request, p *principal) error {
		u, aerr, err := s.adminTarget(r, p)
		if err != nil || aerr != nil {
			return firstErr(err, aerr)
		}
		err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
			if disable {
				_, err := tx.ExecContext(r.Context(), `UPDATE users SET disabled_at = COALESCE(disabled_at, ?) WHERE id = ?`, s.unix(), u.ID)
				return err
			}
			_, err := tx.ExecContext(r.Context(), `UPDATE users SET disabled_at = NULL, failed_logins = 0, locked_until = NULL WHERE id = ?`, u.ID)
			return err
		})
		if err != nil {
			return err
		}
		return s.writeUser(w, r, u.ID, nil)
	}
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, p *principal) error {
	u, aerr, err := s.adminTarget(r, p)
	if err != nil || aerr != nil {
		return firstErr(err, aerr)
	}
	if confirm := r.URL.Query().Get("confirm_email"); !strings.EqualFold(confirm, u.Email) {
		var v validator
		v.add("confirm_email", "confirmation_mismatch", "must repeat the email of the account being deleted; deleting cannot be undone")
		return v.err()
	}
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		// Foreign keys cascade to everything the user owns.
		_, err := tx.ExecContext(r.Context(), `DELETE FROM users WHERE id = ?`, u.ID)
		return err
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) writeUser(w http.ResponseWriter, r *http.Request, id string, extra map[string]any) error {
	u, err := loadUser(r.Context(), s.store.DB, id)
	if err != nil {
		return err
	}
	out := staffUser(u)
	for k, v := range extra {
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func firstErr(err error, aerr *apiError) error {
	if err != nil {
		return err
	}
	return aerr
}
