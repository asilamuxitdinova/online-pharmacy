package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/asilamuxitdinova/online-pharmacy/backend/internal/store"
)

type orderItem struct {
	ProductID string
	Quantity  int
	UnitPrice int64
	product   *product
}

type order struct {
	ID, UserID, Status, Delivery, Note     string
	AddressID, PrescriptionID              sql.NullString
	Subtotal, DeliveryFee, Total, Refunded int64
	CreatedAt, UpdatedAt                   int64
	PaidAt                                 sql.NullInt64
	Items                                  []orderItem
}

const orderColumns = `id, user_id, status, delivery, note, address_id, prescription_id, subtotal, delivery_fee, total, refunded, created_at, updated_at, paid_at`

func scanOrder(row interface{ Scan(...any) error }) (*order, error) {
	var o order
	err := row.Scan(&o.ID, &o.UserID, &o.Status, &o.Delivery, &o.Note, &o.AddressID, &o.PrescriptionID,
		&o.Subtotal, &o.DeliveryFee, &o.Total, &o.Refunded, &o.CreatedAt, &o.UpdatedAt, &o.PaidAt)
	return &o, err
}

func loadOrder(ctx context.Context, q interface {
	querier
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, id string) (*order, error) {
	o, err := scanOrder(q.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM orders WHERE id = ?`, id))
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, `SELECT product_id, quantity, unit_price FROM order_items WHERE order_id = ? ORDER BY product_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it orderItem
		if err := rows.Scan(&it.ProductID, &it.Quantity, &it.UnitPrice); err != nil {
			return nil, err
		}
		o.Items = append(o.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range o.Items {
		if o.Items[i].product, err = loadProduct(ctx, q, o.Items[i].ProductID); err != nil {
			return nil, err
		}
	}
	return o, nil
}

func (o *order) json(lang string) map[string]any {
	items := make([]map[string]any, len(o.Items))
	for i, it := range o.Items {
		items[i] = map[string]any{
			"product_id": it.ProductID, "name": it.product.name(lang), "quantity": it.Quantity,
			"unit_price": it.UnitPrice, "line_total": it.UnitPrice * int64(it.Quantity),
		}
	}
	return map[string]any{
		"id": o.ID, "user_id": o.UserID, "status": o.Status, "items": items, "delivery": o.Delivery,
		"address_id": nullStr(o.AddressID), "prescription_id": nullStr(o.PrescriptionID), "note": o.Note,
		"subtotal": o.Subtotal, "delivery_fee": o.DeliveryFee, "total": o.Total, "refunded": o.Refunded,
		"currency": currency, "created_at": ts(o.CreatedAt), "updated_at": ts(o.UpdatedAt), "paid_at": tsPtr(o.PaidAt),
		"links": map[string]string{"self": "/orders/" + o.ID, "invoice": "/orders/" + o.ID + "/invoice"},
	}
}

// orderFor loads an order the caller may see. Someone else's order is 404,
// not 403: the API does not confirm that it exists. Staff may read any order
// but must act as the customer (X-Act-As) to change one.
func (s *Server) orderFor(ctx context.Context, p *principal, id string, write bool) (*order, error) {
	o, err := loadOrder(ctx, s.store.DB, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("order")
	}
	if err != nil {
		return nil, err
	}
	if o.UserID == p.User.ID {
		return o, nil
	}
	if p.ActingFor == nil && p.is(staff) {
		if !write {
			return o, nil
		}
		return nil, errf(http.StatusForbidden, "act_as_required",
			"Staff may read this order but must act as its customer to change it: send X-Act-As: %s.", o.UserID)
	}
	return nil, notFound("order")
}

// --- list (cursor paging) ---

func encodeCursor(created int64, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(created, 10) + ":" + id))
}

func decodeCursor(c string) (int64, string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return 0, "", false
	}
	a, b, ok := strings.Cut(string(raw), ":")
	n, err := strconv.ParseInt(a, 10, 64)
	return n, b, ok && err == nil
}

var orderStatuses = []string{"pending_payment", "paid", "cancelled", "partially_refunded", "refunded"}

func validStatus(v *validator, s string) {
	for _, st := range orderStatuses {
		if s == st {
			return
		}
	}
	v.add("status", "invalid_enum", "must be one of: %s", strings.Join(orderStatuses, ", "))
}

func (s *Server) listOrders(w http.ResponseWriter, r *http.Request, p *principal) error {
	q := r.URL.Query()
	var v validator
	limit := 20
	if l := q.Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		v.check(err == nil && n >= 1 && n <= 50, "limit", "out_of_range", "must be between 1 and 50")
		limit = min(max(n, 1), 50)
	}
	where, args := "user_id = ?", []any{p.User.ID}
	if st := q.Get("status"); st != "" {
		validStatus(&v, st)
		where += " AND status = ?"
		args = append(args, st)
	}
	if c := q.Get("cursor"); c != "" {
		created, id, ok := decodeCursor(c)
		v.check(ok, "cursor", "invalid_cursor", "is not a cursor this API issued; use next_cursor from the previous page")
		where += " AND (created_at < ? OR (created_at = ? AND id < ?))"
		args = append(args, created, created, id)
	}
	if e := v.err(); e != nil {
		return e
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT id FROM orders WHERE `+where+` ORDER BY created_at DESC, id DESC LIMIT ?`, append(args, limit+1)...)
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
	if err := rows.Err(); err != nil {
		return err
	}
	hasMore := len(ids) > limit
	if hasMore {
		ids = ids[:limit]
	}
	lang := locale(r)
	out := []map[string]any{}
	var last *order
	for _, id := range ids {
		o, err := loadOrder(r.Context(), s.store.DB, id)
		if err != nil {
			return err
		}
		out = append(out, o.json(lang))
		last = o
	}
	var next *string
	if hasMore && last != nil {
		c := encodeCursor(last.CreatedAt, last.ID)
		next = &c
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out, "next_cursor": next, "has_more": hasMore})
	return nil
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request, p *principal) error {
	o, err := s.orderFor(r.Context(), p, r.PathValue("id"), false)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, o.json(locale(r)))
	return nil
}

// --- create and replace ---

type itemInput struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type orderInput struct {
	Items          []itemInput `json:"items"`
	Delivery       string      `json:"delivery"`
	AddressID      *string     `json:"address_id"`
	PrescriptionID *string     `json:"prescription_id"`
	Note           *string     `json:"note"`
}

type priced struct {
	items       []orderItem
	subtotal    int64
	deliveryFee int64
}

func (s *Server) deliveryFee(delivery string, subtotal int64) int64 {
	if delivery == "pickup" || subtotal >= s.cfg.FreeDeliveryThreshold {
		return 0
	}
	return s.cfg.DeliveryFee
}

// checkOrder validates an order body against the catalog and the customer's
// own addresses and prescriptions. Field problems are 422; not enough stock
// is a 409 because the request is fine and the shelf is not.
func (s *Server) checkOrder(ctx context.Context, userID string, in *orderInput) (*priced, error) {
	var v validator
	v.check(len(in.Items) >= 1 && len(in.Items) <= 20, "items", "invalid_length", "must hold 1 to 20 items")
	seen := map[string]bool{}
	out := &priced{}
	needsRx := false
	for i, it := range in.Items {
		f := fmt.Sprintf("items[%d]", i)
		v.check(it.Quantity >= 1 && it.Quantity <= 10, f+".quantity", "out_of_range", "must be between 1 and 10")
		if it.ProductID == "" {
			v.add(f+".product_id", "required", "is required")
			continue
		}
		if seen[it.ProductID] {
			v.add(f+".product_id", "duplicate", "appears twice; combine the quantities into one item")
			continue
		}
		seen[it.ProductID] = true
		p, err := loadProduct(ctx, s.store.DB, it.ProductID)
		if errors.Is(err, sql.ErrNoRows) {
			v.add(f+".product_id", "not_found", "no product with id %q", it.ProductID)
			continue
		}
		if err != nil {
			return nil, err
		}
		needsRx = needsRx || p.RX
		out.items = append(out.items, orderItem{ProductID: p.ID, Quantity: it.Quantity, UnitPrice: p.Price, product: p})
		out.subtotal += p.Price * int64(it.Quantity)
	}
	switch in.Delivery {
	case "courier":
		if in.AddressID == nil {
			v.add("address_id", "required", "is required when delivery is courier")
		}
	case "pickup":
	default:
		v.add("delivery", "invalid_enum", "must be one of: courier, pickup")
	}
	if in.AddressID != nil {
		var n int
		if err := s.store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM addresses WHERE id = ? AND user_id = ?`, *in.AddressID, userID).Scan(&n); err != nil {
			return nil, err
		}
		v.check(n == 1, "address_id", "not_found", "is not one of your addresses; GET /me/addresses lists them")
	}
	if in.PrescriptionID != nil {
		var status string
		err := s.store.DB.QueryRowContext(ctx, `SELECT status FROM prescriptions WHERE id = ? AND user_id = ?`, *in.PrescriptionID, userID).Scan(&status)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			v.add("prescription_id", "not_found", "is not one of your prescriptions")
		case err != nil:
			return nil, err
		case status != "approved":
			v.add("prescription_id", "prescription_not_approved", "is %s; only an approved prescription can be used", status)
		}
	} else if needsRx {
		v.add("prescription_id", "prescription_required", "is required: the order contains prescription-only medicine")
	}
	if in.Note != nil {
		v.check(len(*in.Note) <= 500, "note", "too_long", "must be at most 500 characters")
	}
	if e := v.err(); e != nil {
		return nil, e
	}
	for _, it := range out.items {
		if it.Quantity > it.product.Stock {
			return nil, errf(http.StatusConflict, "insufficient_stock", "Only %d of %s left in stock.", it.product.Stock, it.ProductID).
				with("product_id", it.ProductID).with("available", it.product.Stock).with("requested", it.Quantity)
		}
	}
	out.deliveryFee = s.deliveryFee(in.Delivery, out.subtotal)
	return out, nil
}

func insertItems(ctx context.Context, tx *sql.Tx, orderID string, items []orderItem) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM order_items WHERE order_id = ?`, orderID); err != nil {
		return err
	}
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_items (order_id, product_id, quantity, unit_price) VALUES (?, ?, ?, ?)`,
			orderID, it.ProductID, it.Quantity, it.UnitPrice); err != nil {
			return err
		}
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request, p *principal) error {
	var in orderInput
	if e := decode(r, &in); e != nil {
		return e
	}
	pr, err := s.checkOrder(r.Context(), p.User.ID, &in)
	if err != nil {
		return err
	}
	id := store.NewID("ord")
	now := s.unix()
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO orders (id, user_id, status, delivery, address_id, prescription_id, note, subtotal, delivery_fee, total, created_at, updated_at)
			VALUES (?, ?, 'pending_payment', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, p.User.ID, in.Delivery, in.AddressID, in.PrescriptionID, deref(in.Note), pr.subtotal, pr.deliveryFee, pr.subtotal+pr.deliveryFee, now, now); err != nil {
			return err
		}
		return insertItems(r.Context(), tx, id, pr.items)
	})
	if err != nil {
		return err
	}
	o, err := loadOrder(r.Context(), s.store.DB, id)
	if err != nil {
		return err
	}
	w.Header().Set("Location", "/orders/"+id)
	writeJSON(w, http.StatusCreated, o.json(locale(r)))
	return nil
}

func editable(o *order) *apiError {
	if o.Status != "pending_payment" {
		return errf(http.StatusConflict, "order_not_editable", "Only an order waiting for payment can be changed; this one is %s.", o.Status).
			with("status", o.Status)
	}
	return nil
}

func (s *Server) replaceOrder(w http.ResponseWriter, r *http.Request, p *principal) error {
	o, err := s.orderFor(r.Context(), p, r.PathValue("id"), true)
	if err != nil {
		return err
	}
	var in orderInput
	if e := decode(r, &in); e != nil {
		return e
	}
	if e := editable(o); e != nil {
		return e
	}
	pr, err := s.checkOrder(r.Context(), o.UserID, &in)
	if err != nil {
		return err
	}
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `UPDATE orders SET delivery = ?, address_id = ?, prescription_id = ?, note = ?,
			subtotal = ?, delivery_fee = ?, total = ?, updated_at = ? WHERE id = ?`,
			in.Delivery, in.AddressID, in.PrescriptionID, deref(in.Note), pr.subtotal, pr.deliveryFee, pr.subtotal+pr.deliveryFee, s.unix(), o.ID); err != nil {
			return err
		}
		return insertItems(r.Context(), tx, o.ID, pr.items)
	})
	if err != nil {
		return err
	}
	return s.writeOrder(w, r, o.ID, http.StatusOK)
}

// updateOrder changes delivery details only. Items change through PUT.
func (s *Server) updateOrder(w http.ResponseWriter, r *http.Request, p *principal) error {
	o, err := s.orderFor(r.Context(), p, r.PathValue("id"), true)
	if err != nil {
		return err
	}
	var in struct {
		Delivery       *string `json:"delivery"`
		AddressID      *string `json:"address_id"`
		PrescriptionID *string `json:"prescription_id"`
		Note           *string `json:"note"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	if in.Delivery == nil && in.AddressID == nil && in.PrescriptionID == nil && in.Note == nil {
		var v validator
		v.add("(body)", "empty_patch", "send at least one of: delivery, address_id, prescription_id, note (items change with PUT)")
		return v.err()
	}
	if e := editable(o); e != nil {
		return e
	}
	// Merge onto the current order and validate the whole result.
	merged := orderInput{Delivery: o.Delivery, AddressID: nullStr(o.AddressID), PrescriptionID: nullStr(o.PrescriptionID), Note: &o.Note}
	for _, it := range o.Items {
		merged.Items = append(merged.Items, itemInput{ProductID: it.ProductID, Quantity: it.Quantity})
	}
	if in.Delivery != nil {
		merged.Delivery = *in.Delivery
		if *in.Delivery == "pickup" && in.AddressID == nil {
			merged.AddressID = nil
		}
	}
	if in.AddressID != nil {
		merged.AddressID = in.AddressID
	}
	if in.PrescriptionID != nil {
		merged.PrescriptionID = in.PrescriptionID
	}
	if in.Note != nil {
		merged.Note = in.Note
	}
	pr, err := s.checkOrder(r.Context(), o.UserID, &merged)
	if err != nil {
		return err
	}
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `UPDATE orders SET delivery = ?, address_id = ?, prescription_id = ?, note = ?,
			delivery_fee = ?, total = subtotal + ?, updated_at = ? WHERE id = ?`,
			merged.Delivery, merged.AddressID, merged.PrescriptionID, deref(merged.Note), pr.deliveryFee, pr.deliveryFee, s.unix(), o.ID)
		return err
	})
	if err != nil {
		return err
	}
	return s.writeOrder(w, r, o.ID, http.StatusOK)
}

func (s *Server) writeOrder(w http.ResponseWriter, r *http.Request, id string, status int) error {
	o, err := loadOrder(r.Context(), s.store.DB, id)
	if err != nil {
		return err
	}
	writeJSON(w, status, o.json(locale(r)))
	return nil
}

func (s *Server) deleteOrder(w http.ResponseWriter, r *http.Request, p *principal) error {
	o, err := s.orderFor(r.Context(), p, r.PathValue("id"), true)
	if err != nil {
		return err
	}
	if o.Status != "pending_payment" && o.Status != "cancelled" {
		return errf(http.StatusConflict, "order_not_deletable", "A %s order is part of the payment record and cannot be deleted.", o.Status).with("status", o.Status)
	}
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `DELETE FROM orders WHERE id = ?`, o.ID)
		return err
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request, p *principal) error {
	o, err := s.orderFor(r.Context(), p, r.PathValue("id"), true)
	if err != nil {
		return err
	}
	switch o.Status {
	case "cancelled":
		return errf(http.StatusConflict, "order_already_cancelled", "This order is already cancelled.").with("status", o.Status)
	case "pending_payment":
	default:
		return errf(http.StatusConflict, "order_paid", "A paid order cannot be cancelled; request a refund with POST /orders/%s/refunds.", o.ID).with("status", o.Status)
	}
	err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `UPDATE orders SET status = 'cancelled', updated_at = ? WHERE id = ?`, s.unix(), o.ID)
		return err
	})
	if err != nil {
		return err
	}
	return s.writeOrder(w, r, o.ID, http.StatusOK)
}

// --- invoice (HTML, not JSON) ---

var invoiceTmpl = template.Must(template.New("invoice").Funcs(template.FuncMap{
	"money": func(n int64) string {
		s := strconv.FormatInt(n, 10)
		var b strings.Builder
		for i, c := range s {
			if i > 0 && (len(s)-i)%3 == 0 {
				b.WriteRune(' ')
			}
			b.WriteRune(c)
		}
		return b.String() + " UZS"
	},
}).Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><title>Invoice {{.O.ID}}</title>
<style>body{font-family:system-ui,sans-serif;max-width:720px;margin:40px auto;padding:0 16px;color:#222}
table{width:100%;border-collapse:collapse}td,th{padding:8px;border-bottom:1px solid #ddd;text-align:left}
td.n,th.n{text-align:right}.total td{font-weight:600}</style></head><body>
<h1>Test Pharmacy — invoice</h1>
<p>Order <strong>{{.O.ID}}</strong> · status {{.O.Status}} · issued {{.Issued}}</p>
<p>Customer: {{.Customer}}</p>
<table><thead><tr><th>Item</th><th class="n">Qty</th><th class="n">Price</th><th class="n">Total</th></tr></thead><tbody>
{{range .Lines}}<tr><td>{{.Name}}</td><td class="n">{{.Qty}}</td><td class="n">{{money .Price}}</td><td class="n">{{money .Total}}</td></tr>
{{end}}<tr><td colspan="3">Subtotal</td><td class="n">{{money .O.Subtotal}}</td></tr>
<tr><td colspan="3">Delivery ({{.O.Delivery}})</td><td class="n">{{money .O.DeliveryFee}}</td></tr>
<tr class="total"><td colspan="3">Total</td><td class="n">{{money .O.Total}}</td></tr>
{{if .O.Refunded}}<tr><td colspan="3">Refunded</td><td class="n">−{{money .O.Refunded}}</td></tr>{{end}}
</tbody></table>
<p><small>All values on this invoice are made up. This pharmacy exists only for testing.</small></p>
</body></html>`))

func (s *Server) invoice(w http.ResponseWriter, r *http.Request, p *principal) error {
	o, err := s.orderFor(r.Context(), p, r.PathValue("id"), false)
	if err != nil {
		return err
	}
	owner, err := loadUser(r.Context(), s.store.DB, o.UserID)
	if err != nil {
		return err
	}
	lang := locale(r)
	type line struct {
		Name         string
		Qty          int
		Price, Total int64
	}
	var lines []line
	for _, it := range o.Items {
		lines = append(lines, line{it.product.name(lang), it.Quantity, it.UnitPrice, it.UnitPrice * int64(it.Quantity)})
	}
	var buf bytes.Buffer
	if err := invoiceTmpl.Execute(&buf, map[string]any{"O": o, "Lines": lines, "Lang": lang, "Customer": owner.Name, "Issued": ts(o.CreatedAt)}); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = w.Write(buf.Bytes())
	return err
}

// --- Idempotency-Key ---

type captured struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *captured) Header() http.Header         { return c.header }
func (c *captured) Write(b []byte) (int, error) { return c.body.Write(b) }
func (c *captured) WriteHeader(code int)        { c.status = code }

type savedResponse struct {
	ContentType string `json:"content_type"`
	Location    string `json:"location,omitempty"`
	Body        []byte `json:"body"`
}

// idempotent makes a POST safe to retry. The first answer to a key is stored
// and replayed for every repeat with the same body; the same key with a
// different body is refused. Server errors are not stored, so they can be
// retried for real.
func (s *Server) idempotent(h handlerFunc) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request, p *principal) error {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			return h(w, r, p)
		}
		if len(key) > 255 {
			return errf(http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be at most 255 characters.")
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(r.Method + " " + r.URL.Path + "\n" + string(body)))
		hash := hex.EncodeToString(sum[:])

		var replay *savedResponse
		var replayStatus int
		var aerr *apiError
		err = s.store.Write(r.Context(), func(tx *sql.Tx) error {
			var reqHash string
			var status sql.NullInt64
			var resp []byte
			err := tx.QueryRowContext(r.Context(), `SELECT request_hash, status_code, response FROM idempotency_keys WHERE user_id = ? AND key = ?`,
				p.User.ID, key).Scan(&reqHash, &status, &resp)
			if errors.Is(err, sql.ErrNoRows) {
				_, err = tx.ExecContext(r.Context(), `INSERT INTO idempotency_keys (user_id, key, route, request_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
					p.User.ID, key, r.Method+" "+r.URL.Path, hash, s.unix())
				return err
			}
			if err != nil {
				return err
			}
			switch {
			case reqHash != hash:
				aerr = errf(http.StatusUnprocessableEntity, "idempotency_key_reused",
					"This Idempotency-Key was already used for a different request. Use a new key for a new request.")
			case !status.Valid:
				aerr = errf(http.StatusConflict, "idempotency_in_progress", "The first request with this Idempotency-Key is still running. Retry shortly.")
				aerr.RetryAfter = 1
			default:
				replay, replayStatus = &savedResponse{}, int(status.Int64)
				return json.Unmarshal(resp, replay)
			}
			return nil
		})
		if err != nil {
			return err
		}
		if aerr != nil {
			return aerr
		}
		if replay != nil {
			w.Header().Set("Content-Type", replay.ContentType)
			if replay.Location != "" {
				w.Header().Set("Location", replay.Location)
			}
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(replayStatus)
			_, err := w.Write(replay.Body)
			return err
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		rec := &captured{header: http.Header{}, status: http.StatusOK}
		if err := h(rec, r, p); err != nil {
			s.fail(rec, r, err)
		}
		saveErr := s.store.Write(context.WithoutCancel(r.Context()), func(tx *sql.Tx) error {
			if rec.status >= 500 {
				_, err := tx.ExecContext(r.Context(), `DELETE FROM idempotency_keys WHERE user_id = ? AND key = ?`, p.User.ID, key)
				return err
			}
			resp, _ := json.Marshal(savedResponse{ContentType: rec.header.Get("Content-Type"), Location: rec.header.Get("Location"), Body: rec.body.Bytes()})
			_, err := tx.ExecContext(r.Context(), `UPDATE idempotency_keys SET status_code = ?, response = ? WHERE user_id = ? AND key = ?`,
				rec.status, resp, p.User.ID, key)
			return err
		})
		if saveErr != nil {
			s.log.Error("save idempotent response", "err", saveErr)
		}
		for k, vs := range rec.header {
			w.Header()[k] = vs
		}
		w.WriteHeader(rec.status)
		_, err = w.Write(rec.body.Bytes())
		return err
	}
}
