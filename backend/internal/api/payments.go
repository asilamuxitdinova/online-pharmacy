package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

// --- test cards ---

// cardOutcomes are the numbers that fail on purpose. Every other valid
// number is approved. See docs: /docs/payments.
var cardOutcomes = map[string]string{
	"4000000000000002": "card_declined",
	"4000000000009995": "insufficient_funds",
	"8600000000000999": "card_declined",
	"9860000000000992": "insufficient_funds",
}

func luhn(n string) bool {
	sum := 0
	for i := range len(n) {
		d := int(n[len(n)-1-i] - '0')
		if i%2 == 1 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

func cardBrand(n string) string {
	switch {
	case strings.HasPrefix(n, "4"):
		return "visa"
	case len(n) >= 2 && n[0] == '5' && n[1] >= '1' && n[1] <= '5':
		return "mastercard"
	case strings.HasPrefix(n, "8600"):
		return "uzcard"
	case strings.HasPrefix(n, "9860"):
		return "humo"
	}
	return ""
}

type cardInput struct {
	Number   string `json:"number"`
	ExpMonth int    `json:"exp_month"`
	ExpYear  int    `json:"exp_year"`
	CVC      string `json:"cvc"`
}

var cvcRe = regexp.MustCompile(`^\d{3,4}$`)

type checkedCard struct {
	brand, last4, outcome string
	expMonth, expYear     int
}

// checkCard validates a card. The number is used here and then dropped; only
// the brand and last four digits are ever stored or returned.
func (s *Server) checkCard(c cardInput, prefix string) (*checkedCard, *apiError) {
	num := strings.NewReplacer(" ", "", "-", "").Replace(c.Number)
	var v validator
	digits := regexp.MustCompile(`^\d{16}$`).MatchString(num)
	v.check(digits, prefix+"number", "invalid_card_number", "must be 16 digits")
	if digits {
		v.check(luhn(num), prefix+"number", "invalid_card_number", "fails the checksum; check for a typo")
		v.check(cardBrand(num) != "", prefix+"number", "unsupported_card", "must be a Visa, Mastercard, Uzcard or Humo card")
	}
	v.check(c.ExpMonth >= 1 && c.ExpMonth <= 12, prefix+"exp_month", "out_of_range", "must be 1 to 12")
	now := s.now()
	v.check(c.ExpYear >= now.Year() && c.ExpYear <= now.Year()+20, prefix+"exp_year", "out_of_range", "must be between %d and %d", now.Year(), now.Year()+20)
	v.check(cvcRe.MatchString(c.CVC), prefix+"cvc", "invalid_cvc", "must be 3 or 4 digits")
	if e := v.err(); e != nil {
		return nil, e
	}
	if c.ExpYear == now.Year() && c.ExpMonth < int(now.Month()) {
		return nil, errf(http.StatusPaymentRequired, "card_expired", "The card expired in %02d/%d.", c.ExpMonth, c.ExpYear)
	}
	outcome := cardOutcomes[num]
	if outcome == "" {
		outcome = "approved"
	}
	return &checkedCard{brand: cardBrand(num), last4: num[12:], outcome: outcome, expMonth: c.ExpMonth, expYear: c.ExpYear}, nil
}

func cardJSON(id, brand, last4 string, m, y int, created int64) map[string]any {
	return map[string]any{"id": id, "brand": brand, "last4": last4, "exp_month": m, "exp_year": y, "created_at": ts(created)}
}

func (s *Server) listCards(w http.ResponseWriter, r *http.Request, p *principal) error {
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT id, brand, last4, exp_month, exp_year, created_at FROM cards WHERE user_id = ? ORDER BY created_at`, p.User.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, brand, last4 string
		var m, y int
		var created int64
		if err := rows.Scan(&id, &brand, &last4, &m, &y, &created); err != nil {
			return err
		}
		out = append(out, cardJSON(id, brand, last4, m, y, created))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
	return nil
}

func (s *Server) addCard(w http.ResponseWriter, r *http.Request, p *principal) error {
	var in cardInput
	if e := decode(r, &in); e != nil {
		return e
	}
	c, aerr := s.checkCard(in, "")
	if aerr != nil {
		return aerr
	}
	id := store.NewID("card")
	now := s.unix()
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `INSERT INTO cards (id, user_id, brand, last4, exp_month, exp_year, outcome, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, p.User.ID, c.brand, c.last4, c.expMonth, c.expYear, c.outcome, now)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, cardJSON(id, c.brand, c.last4, c.expMonth, c.expYear, now))
	return nil
}

func (s *Server) deleteCard(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.deleteOwned(w, r, "cards", "card", p.User.ID)
}

// deleteOwned removes a row the caller owns, answering 404 for anyone else's.
func (s *Server) deleteOwned(w http.ResponseWriter, r *http.Request, table, what, userID string) error {
	var n int64
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		res, err := tx.ExecContext(r.Context(), `DELETE FROM `+table+` WHERE id = ? AND user_id = ?`, r.PathValue("id"), userID)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound(what)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- paying an order ---

func (s *Server) payOrder(w http.ResponseWriter, r *http.Request, p *principal) error {
	o, err := s.orderFor(r.Context(), p, r.PathValue("id"), true)
	if err != nil {
		return err
	}
	var in struct {
		CardID *string    `json:"card_id"`
		Card   *cardInput `json:"card"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	if (in.CardID == nil) == (in.Card == nil) {
		var v validator
		v.add("(body)", "one_of_required", "send exactly one of: card_id (a saved card) or card (a card used once)")
		return v.err()
	}
	switch o.Status {
	case "pending_payment":
	case "cancelled":
		return errf(http.StatusConflict, "order_cancelled", "This order was cancelled and cannot be paid.").with("status", o.Status)
	default:
		return errf(http.StatusConflict, "order_already_paid", "This order is already paid. It will not be charged twice.").
			with("status", o.Status).with("paid_at", tsPtr(o.PaidAt))
	}

	var card *checkedCard
	if in.CardID != nil {
		card = &checkedCard{}
		err := s.store.DB.QueryRowContext(r.Context(), `SELECT brand, last4, outcome, exp_month, exp_year FROM cards WHERE id = ? AND user_id = ?`,
			*in.CardID, o.UserID).Scan(&card.brand, &card.last4, &card.outcome, &card.expMonth, &card.expYear)
		if errors.Is(err, sql.ErrNoRows) {
			var v validator
			v.add("card_id", "not_found", "is not one of your saved cards; GET /me/cards lists them")
			return v.err()
		}
		if err != nil {
			return err
		}
	} else {
		var aerr *apiError
		if card, aerr = s.checkCard(*in.Card, "card."); aerr != nil {
			return aerr
		}
	}

	payID := store.NewID("pay")
	now := s.unix()
	var aerr *apiError
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		ctx := r.Context()
		// Re-read inside the lock: two concurrent payments must not both win.
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id = ?`, o.ID).Scan(&status); err != nil {
			return err
		}
		if status != "pending_payment" {
			aerr = errf(http.StatusConflict, "order_already_paid", "This order is already paid. It will not be charged twice.").with("status", status)
			return nil
		}
		if card.outcome != "approved" {
			aerr = errf(http.StatusPaymentRequired, card.outcome, "The payment was refused (%s). No money was taken.", strings.ReplaceAll(card.outcome, "_", " "))
			_, err := tx.ExecContext(ctx, `INSERT INTO payments (id, order_id, amount, brand, last4, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				payID, o.ID, o.Total, card.brand, card.last4, "failed:"+card.outcome, now)
			return err
		}
		for _, it := range o.Items {
			res, err := tx.ExecContext(ctx, `UPDATE products SET stock = stock - ? WHERE id = ? AND stock >= ?`, it.Quantity, it.ProductID, it.Quantity)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				aerr = errf(http.StatusConflict, "insufficient_stock", "%s sold out while this order waited. Change the order with PUT and pay again.", it.ProductID).
					with("product_id", it.ProductID)
				return errRollback
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO payments (id, order_id, amount, brand, last4, status, created_at) VALUES (?, ?, ?, ?, ?, 'succeeded', ?)`,
			payID, o.ID, o.Total, card.brand, card.last4, now); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE orders SET status = 'paid', paid_at = ?, updated_at = ? WHERE id = ?`, now, now, o.ID)
		return err
	})
	if err != nil && !errors.Is(err, errRollback) {
		return err
	}
	if aerr != nil {
		return aerr
	}
	paid, err := loadOrder(r.Context(), s.store.DB, o.ID)
	if err != nil {
		return err
	}
	s.emit(r.Context(), "order.paid", map[string]any{"order_id": o.ID, "amount": o.Total, "currency": currency})
	writeJSON(w, http.StatusOK, map[string]any{
		"order":   paid.json(locale(r)),
		"payment": map[string]any{"id": payID, "amount": o.Total, "currency": currency, "status": "succeeded", "card": map[string]string{"brand": card.brand, "last4": card.last4}, "created_at": ts(now)},
	})
	return nil
}

var errRollback = errors.New("rollback")

// --- refunds ---

type refund struct {
	ID, OrderID, Reason, Status string
	Amount                      int64
	DecidedBy                   sql.NullString
	CreatedAt                   int64
	DecidedAt                   sql.NullInt64
}

const refundColumns = `id, order_id, amount, reason, status, decided_by, created_at, decided_at`

func scanRefund(row interface{ Scan(...any) error }) (*refund, error) {
	var f refund
	err := row.Scan(&f.ID, &f.OrderID, &f.Amount, &f.Reason, &f.Status, &f.DecidedBy, &f.CreatedAt, &f.DecidedAt)
	return &f, err
}

func (f *refund) json() map[string]any {
	return map[string]any{"id": f.ID, "order_id": f.OrderID, "amount": f.Amount, "currency": currency, "reason": f.Reason,
		"status": f.Status, "decided_by": nullStr(f.DecidedBy), "created_at": ts(f.CreatedAt), "decided_at": tsPtr(f.DecidedAt)}
}

func (s *Server) refundRule() string {
	return fmt.Sprintf("Refunds up to %d UZS are approved automatically; larger ones wait for a pharmacist.", s.cfg.AutoRefundLimit)
}

// refundable is what can still be asked for: total minus approved and pending refunds.
func refundable(ctx context.Context, q querier, o *order) (int64, error) {
	var pending int64
	err := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM refunds WHERE order_id = ? AND status = 'pending_review'`, o.ID).Scan(&pending)
	return o.Total - o.Refunded - pending, err
}

func (s *Server) requestRefund(w http.ResponseWriter, r *http.Request, p *principal) error {
	o, err := s.orderFor(r.Context(), p, r.PathValue("id"), true)
	if err != nil {
		return err
	}
	var in struct {
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	in.Reason = trimmed(in.Reason)
	var v validator
	v.check(in.Amount >= 1, "amount", "out_of_range", "must be at least 1 UZS")
	v.check(len(in.Reason) >= 3 && len(in.Reason) <= 300, "reason", "invalid_length", "must be 3 to 300 characters")
	if e := v.err(); e != nil {
		return e
	}
	if o.Status != "paid" && o.Status != "partially_refunded" {
		return errf(http.StatusConflict, "order_not_refundable", "Only a paid order can be refunded; this one is %s.", o.Status).with("status", o.Status)
	}
	id := store.NewID("ref")
	now := s.unix()
	auto := in.Amount <= s.cfg.AutoRefundLimit
	var aerr *apiError
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		ctx := r.Context()
		fresh, err := scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM orders WHERE id = ?`, o.ID))
		if err != nil {
			return err
		}
		left, err := refundable(ctx, tx, fresh)
		if err != nil {
			return err
		}
		if in.Amount > left {
			aerr = &apiError{Status: http.StatusUnprocessableEntity, Code: "validation_failed", Message: "Some fields are invalid. Fix every field listed.",
				Fields: []fieldError{{Field: "amount", Code: "exceeds_refundable", Message: fmt.Sprintf("must be at most %d UZS, the amount still refundable", left)}}}
			return nil
		}
		status := "pending_review"
		var decided any
		var by any
		if auto {
			status, decided, by = "approved", now, "auto"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO refunds (id, order_id, amount, reason, status, decided_by, created_at, decided_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, o.ID, in.Amount, in.Reason, status, by, now, decided); err != nil {
			return err
		}
		if auto {
			return applyRefund(ctx, tx, o.ID, in.Amount, now)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if aerr != nil {
		return aerr
	}
	f, err := scanRefund(s.store.DB.QueryRowContext(r.Context(), `SELECT `+refundColumns+` FROM refunds WHERE id = ?`, id))
	if err != nil {
		return err
	}
	out := f.json()
	out["auto_approved"] = auto
	out["rule"] = s.refundRule()
	if auto {
		s.emit(r.Context(), "refund.approved", map[string]any{"refund_id": id, "order_id": o.ID, "amount": in.Amount})
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func applyRefund(ctx context.Context, tx *sql.Tx, orderID string, amount, now int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE orders SET refunded = refunded + ?,
		status = CASE WHEN refunded + ? >= total THEN 'refunded' ELSE 'partially_refunded' END, updated_at = ? WHERE id = ?`,
		amount, amount, now, orderID)
	return err
}

func (s *Server) listOrderRefunds(w http.ResponseWriter, r *http.Request, p *principal) error {
	o, err := s.orderFor(r.Context(), p, r.PathValue("id"), false)
	if err != nil {
		return err
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT `+refundColumns+` FROM refunds WHERE order_id = ? ORDER BY created_at`, o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		f, err := scanRefund(rows)
		if err != nil {
			return err
		}
		out = append(out, f.json())
	}
	if err := rows.Err(); err != nil {
		return err
	}
	left, err := refundable(r.Context(), s.store.DB, o)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out, "refundable": left, "currency": currency, "rule": s.refundRule()})
	return nil
}

func (s *Server) listRefundsForReview(w http.ResponseWriter, r *http.Request, _ *principal) error {
	var v validator
	page, size := pageParams(r, &v, 20, 100)
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending_review"
	}
	v.check(status == "pending_review" || status == "approved" || status == "rejected", "status", "invalid_enum", "must be one of: pending_review, approved, rejected")
	if e := v.err(); e != nil {
		return e
	}
	var total int
	if err := s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM refunds WHERE status = ?`, status).Scan(&total); err != nil {
		return err
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT `+refundColumns+` FROM refunds WHERE status = ? ORDER BY created_at LIMIT ? OFFSET ?`,
		status, size, (page-1)*size)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		f, err := scanRefund(rows)
		if err != nil {
			return err
		}
		out = append(out, f.json())
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, paged(out, page, size, total))
	return nil
}

func (s *Server) decideRefund(approve bool) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request, p *principal) error {
		var in struct {
			Note string `json:"note"`
		}
		if r.ContentLength != 0 {
			if e := decode(r, &in); e != nil {
				return e
			}
		}
		id := r.PathValue("id")
		now := s.unix()
		var aerr *apiError
		var orderID string
		var amount int64
		err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
			ctx := r.Context()
			f, err := scanRefund(tx.QueryRowContext(ctx, `SELECT `+refundColumns+` FROM refunds WHERE id = ?`, id))
			if errors.Is(err, sql.ErrNoRows) {
				aerr = notFound("refund")
				return nil
			}
			if err != nil {
				return err
			}
			if f.Status != "pending_review" {
				aerr = errf(http.StatusConflict, "refund_already_decided", "This refund was already %s.", f.Status).with("status", f.Status)
				return nil
			}
			status := "rejected"
			if approve {
				status = "approved"
			}
			if _, err := tx.ExecContext(ctx, `UPDATE refunds SET status = ?, decided_by = ?, decided_at = ? WHERE id = ?`, status, p.Actor.ID, now, id); err != nil {
				return err
			}
			orderID, amount = f.OrderID, f.Amount
			if approve {
				return applyRefund(ctx, tx, f.OrderID, f.Amount, now)
			}
			return nil
		})
		if err != nil {
			return err
		}
		if aerr != nil {
			return aerr
		}
		if approve {
			s.emit(r.Context(), "refund.approved", map[string]any{"refund_id": id, "order_id": orderID, "amount": amount})
		}
		f, err := scanRefund(s.store.DB.QueryRowContext(r.Context(), `SELECT `+refundColumns+` FROM refunds WHERE id = ?`, id))
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, f.json())
		return nil
	}
}
