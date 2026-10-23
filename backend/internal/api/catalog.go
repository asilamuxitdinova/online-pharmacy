package api

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const currency = "UZS"

type product struct {
	ID, SKU, CategoryID, NameEN, NameRU, NameUZ, Manufacturer, Form, Strength string
	PackSize                                                                  int
	Price                                                                     int64
	Stock                                                                     int
	RX                                                                        bool
	CategoryEN, CategoryRU, CategoryUZ                                        string
}

const productColumns = `p.id, p.sku, p.category_id, p.name_en, p.name_ru, p.name_uz, p.manufacturer, p.form, p.strength,
	p.pack_size, p.price, p.stock, p.requires_prescription, c.name_en, c.name_ru, c.name_uz`
const productFrom = ` FROM products p JOIN categories c ON c.id = p.category_id`

func scanProduct(row interface{ Scan(...any) error }) (*product, error) {
	var p product
	err := row.Scan(&p.ID, &p.SKU, &p.CategoryID, &p.NameEN, &p.NameRU, &p.NameUZ, &p.Manufacturer, &p.Form, &p.Strength,
		&p.PackSize, &p.Price, &p.Stock, &p.RX, &p.CategoryEN, &p.CategoryRU, &p.CategoryUZ)
	return &p, err
}

func pick(lang, en, ru, uz string) string {
	switch lang {
	case "ru":
		return ru
	case "uz":
		return uz
	}
	return en
}

func (p *product) name(lang string) string { return pick(lang, p.NameEN, p.NameRU, p.NameUZ) }

func (p *product) json(lang string) map[string]any {
	return map[string]any{
		"id": p.ID, "sku": p.SKU, "name": p.name(lang),
		"category":     map[string]string{"id": p.CategoryID, "name": pick(lang, p.CategoryEN, p.CategoryRU, p.CategoryUZ)},
		"manufacturer": p.Manufacturer, "form": p.Form, "strength": p.Strength, "pack_size": p.PackSize,
		"price": p.Price, "currency": currency, "stock": p.Stock, "in_stock": p.Stock > 0,
		"requires_prescription": p.RX,
	}
}

func loadProduct(ctx context.Context, q querier, id string) (*product, error) {
	return scanProduct(q.QueryRowContext(ctx, `SELECT `+productColumns+productFrom+` WHERE p.id = ?`, id))
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request, _ *principal) error {
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT c.id, c.name_en, c.name_ru, c.name_uz, COUNT(p.id)
		FROM categories c LEFT JOIN products p ON p.category_id = c.id GROUP BY c.id ORDER BY c.name_en`)
	if err != nil {
		return err
	}
	defer rows.Close()
	lang := locale(r)
	out := []map[string]any{}
	for rows.Next() {
		var id, en, ru, uz string
		var n int
		if err := rows.Scan(&id, &en, &ru, &uz, &n); err != nil {
			return err
		}
		out = append(out, map[string]any{"id": id, "name": pick(lang, en, ru, uz), "product_count": n})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
	return nil
}

var productSorts = map[string]string{
	"name": "p.name_en ASC", "-name": "p.name_en DESC",
	"price": "p.price ASC, p.id", "-price": "p.price DESC, p.id",
	"newest": "p.created_at DESC, p.id",
}

// pageParams reads page and page_size, adding problems to v.
func pageParams(r *http.Request, v *validator, defSize, maxSize int) (page, size int) {
	page, size = 1, defSize
	if s := r.URL.Query().Get("page"); s != "" {
		n, err := strconv.Atoi(s)
		v.check(err == nil && n >= 1, "page", "invalid_value", "must be a whole number of 1 or more")
		page = max(n, 1)
	}
	if s := r.URL.Query().Get("page_size"); s != "" {
		n, err := strconv.Atoi(s)
		v.check(err == nil && n >= 1 && n <= maxSize, "page_size", "out_of_range", "must be between 1 and %d", maxSize)
		size = min(max(n, 1), maxSize)
	}
	return page, size
}

func paged(data any, page, size, total int) map[string]any {
	return map[string]any{"data": data, "page": page, "page_size": size, "total": total,
		"total_pages": (total + size - 1) / size}
}

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request, _ *principal) error {
	q := r.URL.Query()
	var v validator
	page, size := pageParams(r, &v, 20, 100)

	where := []string{"1=1"}
	var args []any
	if text := strings.ToLower(trimmed(q.Get("q"))); text != "" {
		for _, word := range strings.Fields(text) {
			where = append(where, "p.search LIKE ?")
			args = append(args, "%"+word+"%")
		}
	}
	if c := q.Get("category"); c != "" {
		var exists int
		_ = s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM categories WHERE id = ?`, c).Scan(&exists)
		v.check(exists == 1, "category", "unknown_category", "is not a category id; GET /categories lists them")
		where = append(where, "p.category_id = ?")
		args = append(args, c)
	}
	for _, b := range []struct{ param, op string }{{"min_price", ">="}, {"max_price", "<="}} {
		if s := q.Get(b.param); s != "" {
			n, err := strconv.ParseInt(s, 10, 64)
			v.check(err == nil && n >= 0, b.param, "invalid_value", "must be a whole number of UZS, 0 or more")
			where = append(where, "p.price "+b.op+" ?")
			args = append(args, n)
		}
	}
	for _, b := range []struct{ param, cond string }{{"requires_prescription", "p.requires_prescription = ?"}, {"in_stock", "(p.stock > 0) = ?"}} {
		if s := q.Get(b.param); s != "" {
			val, err := strconv.ParseBool(s)
			v.check(err == nil, b.param, "invalid_value", "must be true or false")
			where = append(where, b.cond)
			args = append(args, val)
		}
	}
	order := productSorts["name"]
	if s := q.Get("sort"); s != "" {
		o, ok := productSorts[s]
		v.check(ok, "sort", "invalid_enum", "must be one of: name, -name, price, -price, newest")
		if ok {
			order = o
		}
	}
	if e := v.err(); e != nil {
		return e
	}

	cond := strings.Join(where, " AND ")
	var total int
	if err := s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM products p WHERE `+cond, args...).Scan(&total); err != nil {
		return err
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT `+productColumns+productFrom+` WHERE `+cond+
		` ORDER BY `+order+` LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	lang := locale(r)
	out := []map[string]any{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return err
		}
		out = append(out, p.json(lang))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, paged(out, page, size, total))
	return nil
}

func (s *Server) getProduct(w http.ResponseWriter, r *http.Request, _ *principal) error {
	p, err := loadProduct(r.Context(), s.store.DB, r.PathValue("id"))
	if errors.Is(err, sql.ErrNoRows) {
		return errf(http.StatusNotFound, "product_not_found", "No product with id %q.", r.PathValue("id"))
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, p.json(locale(r)))
	return nil
}

// exportProducts returns the whole catalog: a deliberately large answer.
func (s *Server) exportProducts(w http.ResponseWriter, r *http.Request, _ *principal) error {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		var v validator
		v.add("format", "invalid_enum", "must be csv or json")
		return v.err()
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT `+productColumns+productFrom+` ORDER BY p.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	lang := locale(r)
	var all []*product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return err
		}
		all = append(all, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if format == "json" {
		out := make([]map[string]any, len(all))
		for i, p := range all {
			out[i] = p.json(lang)
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out, "total": len(out)})
		return nil
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="products.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "sku", "name", "category", "manufacturer", "form", "strength", "pack_size", "price_uzs", "stock", "requires_prescription"})
	for _, p := range all {
		_ = cw.Write([]string{p.ID, p.SKU, p.name(lang), p.CategoryID, p.Manufacturer, p.Form, p.Strength,
			strconv.Itoa(p.PackSize), strconv.FormatInt(p.Price, 10), strconv.Itoa(p.Stock), strconv.FormatBool(p.RX)})
	}
	cw.Flush()
	return cw.Error()
}
