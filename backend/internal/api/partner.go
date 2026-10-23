package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Signed requests, for the partner (wholesaler) endpoints. The caller signs
//
//	<X-Timestamp> "\n" <METHOD> "\n" <path?query> "\n" <hex sha256 of body>
//
// with HMAC-SHA256 and the partner secret, and sends the hex digest in
// X-Signature. A timestamp outside the allowed skew is refused, which stops
// a captured request being replayed later.

func SignRequest(secret, timestamp, method, pathAndQuery string, body []byte) string {
	sum := sha256.Sum256(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + strings.ToUpper(method) + "\n" + pathAndQuery + "\n" + hex.EncodeToString(sum[:])))
	return hex.EncodeToString(mac.Sum(nil))
}

// signed wraps a handler with the signature check. It has no principal.
func (s *Server) signed(h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if aerr := s.verifySignature(r); aerr != nil {
			writeError(w, r, aerr)
			return
		}
		if err := h(w, r, nil); err != nil {
			s.fail(w, r, err)
		}
	})
}

func (s *Server) verifySignature(r *http.Request) *apiError {
	client, stamp, sig := r.Header.Get("X-Client-Id"), r.Header.Get("X-Timestamp"), r.Header.Get("X-Signature")
	if client == "" || stamp == "" || sig == "" {
		return errf(http.StatusUnauthorized, "signature_missing", "Signed request required: send X-Client-Id, X-Timestamp and X-Signature.")
	}
	if client != s.cfg.PartnerClientID {
		return errf(http.StatusUnauthorized, "unknown_client", "X-Client-Id %q is not a known partner.", client)
	}
	sec, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return errf(http.StatusUnauthorized, "signature_invalid", "X-Timestamp must be unix seconds.")
	}
	skew := s.now().Sub(time.Unix(sec, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > s.cfg.SignatureSkew {
		return errf(http.StatusUnauthorized, "signature_expired", "X-Timestamp is %s away from the server clock; at most %s is allowed.",
			skew.Round(time.Second), s.cfg.SignatureSkew).with("server_time", s.unix())
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		return errf(http.StatusBadRequest, "invalid_body", "The request body could not be read.")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	want := SignRequest(s.cfg.PartnerSecret, stamp, r.Method, r.URL.RequestURI(), body)
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(sig))) {
		return errf(http.StatusUnauthorized, "signature_invalid", "X-Signature does not match. See /docs/authentication for the exact string to sign.")
	}
	return nil
}

func (s *Server) partnerInventory(w http.ResponseWriter, r *http.Request, _ *principal) error {
	var v validator
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		v.check(err == nil && n >= 1 && n <= 1000, "limit", "out_of_range", "must be between 1 and 1000")
		limit = min(max(n, 1), 1000)
	}
	if e := v.err(); e != nil {
		return e
	}
	where, args := "1=1", []any{}
	if sku := r.URL.Query().Get("sku"); sku != "" {
		where, args = "sku = ?", []any{sku}
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT sku, id, stock, price FROM products WHERE `+where+` ORDER BY sku LIMIT ?`, append(args, limit)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var sku, id string
		var stock int
		var price int64
		if err := rows.Scan(&sku, &id, &stock, &price); err != nil {
			return err
		}
		out = append(out, map[string]any{"sku": sku, "product_id": id, "stock": stock, "price": price, "currency": currency})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
	return nil
}

func (s *Server) partnerRestock(w http.ResponseWriter, r *http.Request, _ *principal) error {
	var in struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	}
	if e := decode(r, &in); e != nil {
		return e
	}
	var v validator
	v.check(in.SKU != "", "sku", "required", "is required")
	v.check(in.Quantity >= 1 && in.Quantity <= 10_000, "quantity", "out_of_range", "must be between 1 and 10000")
	if e := v.err(); e != nil {
		return e
	}
	var stock int
	var found bool
	err := s.store.Write(r.Context(), func(tx *sql.Tx) error {
		res, err := tx.ExecContext(r.Context(), `UPDATE products SET stock = stock + ? WHERE sku = ?`, in.Quantity, in.SKU)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if found = n > 0; !found {
			return nil
		}
		return tx.QueryRowContext(r.Context(), `SELECT stock FROM products WHERE sku = ?`, in.SKU).Scan(&stock)
	})
	if err != nil {
		return err
	}
	if !found {
		return errf(http.StatusNotFound, "product_not_found", "No product with SKU %q.", in.SKU)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sku": in.SKU, "added": in.Quantity, "stock": stock})
	return nil
}
