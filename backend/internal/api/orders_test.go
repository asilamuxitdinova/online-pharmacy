package api

import (
	"strings"
	"testing"
	"time"
)

func TestCatalog(t *testing.T) {
	e := newEnv(t)
	r := e.call("GET", "/products?page=2&page_size=5", nil).expect(200)
	if r.num("total") != 1000 || r.num("total_pages") != 200 || len(r.list("data")) != 5 {
		t.Fatalf("paging: total=%d pages=%d", r.num("total"), r.num("total_pages"))
	}

	ru := e.call("GET", "/products/prd_0001", nil, "Locale", "ru").expect(200)
	if !strings.Contains(ru.str("name"), "Парацетамол") || ru.Header.Get("Content-Language") != "ru" {
		t.Fatalf("locale: %s", ru.Raw)
	}
	uz := e.call("GET", "/products?q=paratsetamol&page_size=1", nil, "Accept-Language", "uz-UZ,uz;q=0.9").expect(200)
	if uz.num("total") == 0 {
		t.Fatal("search should match Uzbek names")
	}
	// Cyrillic search is case-insensitive too.
	if e.call("GET", "/products?q=ПАРАЦЕТАМОЛ", nil).expect(200).num("total") == 0 {
		t.Fatal("search should match Russian names in any case")
	}

	f := e.call("GET", "/products?category=antibiotics&max_price=30000&sort=-price&requires_prescription=true", nil).expect(200)
	var last int64 = 1 << 62
	for _, it := range f.list("data") {
		p := it.(map[string]any)
		price := int64(p["price"].(float64))
		if price > 30000 || price > last || p["requires_prescription"] != true {
			t.Fatalf("filter/sort broken: %v", p)
		}
		last = price
	}

	empty := e.call("GET", "/products?q=unobtainium", nil).expect(200)
	if empty.num("total") != 0 || len(empty.list("data")) != 0 {
		t.Fatalf("empty result: %s", empty.Raw)
	}

	bad := e.call("GET", "/products?page=0&page_size=500&sort=cheapest&category=sweets", nil).expectErr(422, "validation_failed")
	if n := len(bad.JSON["error"].(map[string]any)["fields"].([]any)); n != 4 {
		t.Fatalf("all four bad parameters should be listed, got %d: %s", n, bad.Raw)
	}

	e.call("GET", "/products/prd_9999", nil).expectErr(404, "product_not_found")

	csv := e.call("GET", "/products/export", nil).expect(200)
	if !strings.HasPrefix(csv.Header.Get("Content-Type"), "text/csv") || strings.Count(string(csv.Raw), "\n") != 1001 {
		t.Fatalf("csv export: %s, %d lines", csv.Header.Get("Content-Type"), strings.Count(string(csv.Raw), "\n"))
	}
	if js := e.call("GET", "/products/export?format=json", nil).expect(200); js.num("total") != 1000 {
		t.Fatal("json export should hold 1000 products")
	}
	if cats := e.call("GET", "/categories", nil).expect(200); len(cats.list("data")) != 12 {
		t.Fatalf("categories: %s", cats.Raw)
	}
}

func TestStrictRequestBodies(t *testing.T) {
	e := newEnv(t)
	a := e.as("aziza")
	e.call("POST", "/orders", `{"items": [`, a...).expectErr(400, "invalid_json")
	r := e.call("POST", "/orders", map[string]any{"items": []any{}, "delivery": "pickup", "coupon": "FREE"}, a...).expectErr(400, "unknown_field")
	if r.str("error", "details", "field") != "coupon" {
		t.Fatalf("unknown field should be named: %s", r.Raw)
	}
	e.call("POST", "/orders", map[string]any{"items": "lots", "delivery": "pickup"}, a...).expectErr(422, "validation_failed")

	all := e.call("POST", "/orders", map[string]any{
		"items":    []map[string]any{{"product_id": "prd_9999", "quantity": 0}, {"product_id": "", "quantity": 11}},
		"delivery": "drone", "note": strings.Repeat("x", 501),
	}, a...).expectErr(422, "validation_failed")
	fields := map[string]bool{}
	for _, f := range all.JSON["error"].(map[string]any)["fields"].([]any) {
		fields[f.(map[string]any)["field"].(string)] = true
	}
	for _, want := range []string{"items[0].quantity", "items[0].product_id", "items[1].quantity", "items[1].product_id", "delivery", "note"} {
		if !fields[want] {
			t.Errorf("422 should list %s; got %v", want, fields)
		}
	}
}

func TestOwnershipAndActingAs(t *testing.T) {
	e := newEnv(t)
	aziza, bekzod, staff := e.as("aziza"), e.as("bekzod"), e.as("dilnoza")

	e.call("GET", "/orders/ord_aziza_paid", nil, aziza...).expect(200)
	e.call("GET", "/orders/ord_aziza_paid", nil, bekzod...).expectErr(404, "order_not_found")
	e.call("POST", "/orders/ord_aziza_pending/cancel", nil, bekzod...).expectErr(404, "order_not_found")
	e.call("GET", "/orders/ord_aziza_paid/invoice", nil, bekzod...).expectErr(404, "order_not_found")

	// Staff read anything, but change a customer's order only while acting as them.
	e.call("GET", "/orders/ord_aziza_paid", nil, staff...).expect(200)
	e.call("POST", "/orders/ord_aziza_pending/cancel", nil, staff...).expectErr(403, "act_as_required")
	r := e.call("GET", "/orders", nil, hdr(staff, "X-Act-As", "usr_aziza")...).expect(200)
	if r.Header.Get("X-Acting-As") != "usr_aziza" || len(r.list("data")) != 4 {
		t.Fatalf("acting as aziza: %s", r.Raw)
	}
	me := e.call("GET", "/me", nil, hdr(staff, "X-Act-As", "usr_aziza")...).expect(200)
	if me.str("id") != "usr_aziza" || me.str("actor", "id") != "usr_dilnoza" {
		t.Fatalf("me while acting: %s", me.Raw)
	}
	e.call("POST", "/orders/ord_aziza_pending/cancel", nil, hdr(staff, "X-Act-As", "usr_aziza")...).expect(200)
	// Acting as a customer means seeing only what they see.
	e.call("GET", "/orders/ord_bekzod_paid", nil, hdr(staff, "X-Act-As", "usr_aziza")...).expectErr(404, "order_not_found")
	e.call("GET", "/staff/orders", nil, hdr(staff, "X-Act-As", "usr_aziza")...).expectErr(403, "act_as_not_allowed")
	// Customers cannot act as anyone.
	e.call("GET", "/orders", nil, hdr(bekzod, "X-Act-As", "usr_aziza")...).expectErr(403, "role_required")
}

func TestSensitiveFieldsAreStaffOnly(t *testing.T) {
	e := newEnv(t)
	staff := e.as("dilnoza")
	c := e.call("GET", "/staff/customers/usr_aziza", nil, staff...).expect(200)
	if c.str("tin") != "301234567" {
		t.Fatalf("staff should see the taxpayer number: %s", c.Raw)
	}
	e.call("GET", "/staff/customers/usr_aziza", nil, e.as("aziza")...).expectErr(403, "role_required")
	if found := e.call("GET", "/staff/customers?q=bekzod", nil, staff...).expect(200); found.num("total") != 1 {
		t.Fatalf("customer search: %s", found.Raw)
	}
}

func TestOrderLifecycle(t *testing.T) {
	e := newEnv(t)
	a := e.as("aziza")
	body := map[string]any{"items": []map[string]any{{"product_id": "prd_0001", "quantity": 2}}, "delivery": "courier", "address_id": "adr_aziza_home"}

	// Safe retry: same key, same body → same order, created once.
	first := e.call("POST", "/orders", body, hdr(a, "Idempotency-Key", "k1")...).expect(201)
	again := e.call("POST", "/orders", body, hdr(a, "Idempotency-Key", "k1")...).expect(201)
	if first.str("id") != again.str("id") || again.Header.Get("Idempotent-Replayed") != "true" || again.Header.Get("Location") == "" {
		t.Fatalf("idempotent replay: %s vs %s", first.Raw, again.Raw)
	}
	e.call("POST", "/orders", map[string]any{"items": []map[string]any{{"product_id": "prd_0002", "quantity": 1}}, "delivery": "pickup"},
		hdr(a, "Idempotency-Key", "k1")...).expectErr(422, "idempotency_key_reused")
	id := first.str("id")
	if first.num("delivery_fee") != 25000 || first.num("total") != first.num("subtotal")+25000 {
		t.Fatalf("delivery fee under the threshold: %s", first.Raw)
	}

	// PATCH changes delivery only; PUT replaces everything.
	p := e.call("PATCH", "/orders/"+id, map[string]any{"delivery": "pickup", "note": "after 6pm"}, a...).expect(200)
	if p.num("delivery_fee") != 0 || p.str("address_id") != "" || len(p.list("items")) != 1 {
		t.Fatalf("patch: %s", p.Raw)
	}
	e.call("PATCH", "/orders/"+id, map[string]any{"items": []any{}}, a...).expectErr(400, "unknown_field")
	e.call("PATCH", "/orders/"+id, map[string]any{}, a...).expectErr(422, "validation_failed")
	put := e.call("PUT", "/orders/"+id, map[string]any{"items": []map[string]any{{"product_id": "prd_0029", "quantity": 10}}, "delivery": "courier", "address_id": "adr_aziza_home"}, a...).expect(200)
	if put.num("subtotal") < 300000 || put.num("delivery_fee") != 0 || put.str("note") != "" {
		t.Fatalf("PUT should replace everything and courier is free over 300 000: %s", put.Raw)
	}

	// Pay with a declined card: 402, nothing changes.
	e.call("POST", "/orders/"+id+"/pay", map[string]string{"card_id": "card_aziza_declines"}, a...).expectErr(402, "card_declined")
	e.call("POST", "/orders/"+id+"/pay", map[string]any{"card": map[string]any{"number": "4000 0000 0000 9995", "exp_month": 1, "exp_year": 2030, "cvc": "123"}}, a...).
		expectErr(402, "insufficient_funds")
	e.call("POST", "/orders/"+id+"/pay", map[string]any{"card_id": "card_aziza_visa", "card": map[string]any{}}, a...).expectErr(422, "validation_failed")
	stockBefore := e.call("GET", "/products/prd_0029", nil).num("stock")

	paid := e.call("POST", "/orders/"+id+"/pay", map[string]string{"card_id": "card_aziza_visa"}, hdr(a, "Idempotency-Key", "pay-1")...).expect(200)
	if paid.str("order", "status") != "paid" || paid.str("payment", "card", "last4") != "4242" {
		t.Fatalf("pay: %s", paid.Raw)
	}
	if after := e.call("GET", "/products/prd_0029", nil).num("stock"); after != stockBefore-10 {
		t.Fatalf("stock should drop by 10: %d → %d", stockBefore, after)
	}
	// A retry with the key replays; without it, paying twice is a conflict.
	e.call("POST", "/orders/"+id+"/pay", map[string]string{"card_id": "card_aziza_visa"}, hdr(a, "Idempotency-Key", "pay-1")...).expect(200)
	e.call("POST", "/orders/"+id+"/pay", map[string]string{"card_id": "card_aziza_visa"}, a...).expectErr(409, "order_already_paid")
	e.call("POST", "/orders/"+id+"/cancel", nil, a...).expectErr(409, "order_paid")
	e.call("PUT", "/orders/"+id, body, a...).expectErr(409, "order_not_editable")
	e.call("DELETE", "/orders/"+id, nil, a...).expectErr(409, "order_not_deletable")

	invoice := e.call("GET", "/orders/"+id+"/invoice", nil, a...).expect(200)
	if !strings.HasPrefix(invoice.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(invoice.Raw), id) {
		t.Fatal("invoice should be an HTML page")
	}

	// Refunds: small is automatic, large waits for a pharmacist.
	total := paid.num("order", "total")
	small := e.call("POST", "/orders/"+id+"/refunds", map[string]any{"amount": 50000, "reason": "one pack damaged"}, a...).expect(201)
	if small.str("status") != "approved" || small.str("auto_approved") != "true" {
		t.Fatalf("small refund: %s", small.Raw)
	}
	big := e.call("POST", "/orders/"+id+"/refunds", map[string]any{"amount": 250000, "reason": "wrong strength"}, a...).expect(201)
	if big.str("status") != "pending_review" {
		t.Fatalf("big refund: %s", big.Raw)
	}
	tooMuch := e.call("POST", "/orders/"+id+"/refunds", map[string]any{"amount": total, "reason": "everything"}, a...).expectErr(422, "validation_failed")
	if !strings.Contains(string(tooMuch.Raw), "exceeds_refundable") {
		t.Fatalf("over-refund: %s", tooMuch.Raw)
	}
	o := e.call("GET", "/orders/"+id, nil, a...).expect(200)
	if o.str("status") != "partially_refunded" || o.num("refunded") != 50000 {
		t.Fatalf("after small refund: %s", o.Raw)
	}

	staff := e.as("dilnoza")
	e.call("POST", "/staff/refunds/"+big.str("id")+"/approve", nil, a...).expectErr(403, "role_required")
	queue := e.call("GET", "/staff/refunds", nil, staff...).expect(200)
	if queue.num("total") != 1 {
		t.Fatalf("review queue: %s", queue.Raw)
	}
	e.call("POST", "/staff/refunds/"+big.str("id")+"/approve", map[string]string{"note": "checked"}, staff...).expect(200)
	e.call("POST", "/staff/refunds/"+big.str("id")+"/reject", nil, staff...).expectErr(409, "refund_already_decided")
	if o := e.call("GET", "/orders/"+id, nil, a...); o.num("refunded") != 300000 {
		t.Fatalf("after approval: %s", o.Raw)
	}
	refunds := e.call("GET", "/orders/"+id+"/refunds", nil, a...).expect(200)
	if refunds.num("refundable") != total-300000 {
		t.Fatalf("refundable: %s", refunds.Raw)
	}
}

func TestInsufficientStock(t *testing.T) {
	e := newEnv(t)
	a := e.as("aziza")
	// prd_0017 is seeded with no stock (every 17th product is sold out).
	r := e.call("POST", "/orders", map[string]any{"items": []map[string]any{{"product_id": "prd_0017", "quantity": 1}}, "delivery": "pickup", "prescription_id": "rx_aziza_approved"}, a...).
		expectErr(409, "insufficient_stock")
	if r.num("error", "details", "available") != 0 {
		t.Fatalf("details: %s", r.Raw)
	}
}

func TestPrescriptionRules(t *testing.T) {
	e := newEnv(t)
	a := e.as("aziza")
	rx := e.call("GET", "/products?requires_prescription=true&in_stock=true&page_size=1", nil).expect(200)
	product := rx.list("data")[0].(map[string]any)["id"].(string)
	order := func(prescription any) *resp {
		body := map[string]any{"items": []map[string]any{{"product_id": product, "quantity": 1}}, "delivery": "pickup"}
		if prescription != nil {
			body["prescription_id"] = prescription
		}
		return e.call("POST", "/orders", body, a...)
	}
	if r := order(nil).expectErr(422, "validation_failed"); !strings.Contains(string(r.Raw), "prescription_required") {
		t.Fatal(string(r.Raw))
	}
	if r := order("rx_aziza_pending").expectErr(422, "validation_failed"); !strings.Contains(string(r.Raw), "prescription_not_approved") {
		t.Fatal(string(r.Raw))
	}
	order("rx_aziza_approved").expect(201)

	up := e.call("POST", "/prescriptions", map[string]string{"doctor_name": "Dr. Test", "clinic": "Clinic 0", "issued_on": "2026-10-20", "diagnosis": "made-up"}, a...).expect(201)
	e.call("POST", "/prescriptions", map[string]string{"doctor_name": "Dr. Test", "clinic": "Clinic 0", "issued_on": "2027-01-01", "diagnosis": "made-up"}, a...).
		expectErr(422, "validation_failed")
	staff := e.as("dilnoza")
	e.call("POST", "/staff/prescriptions/"+up.str("id")+"/review", map[string]string{"decision": "reject"}, staff...).expectErr(422, "validation_failed")
	e.call("POST", "/staff/prescriptions/"+up.str("id")+"/review", map[string]string{"decision": "approve"}, staff...).expect(200)
	order(up.str("id")).expect(201)
	e.call("GET", "/prescriptions/"+up.str("id"), nil, e.as("bekzod")...).expectErr(404, "prescription_not_found")
}

func TestCursorPaging(t *testing.T) {
	e := newEnv(t)
	a := e.as("aziza")
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		path := "/orders?limit=3"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := e.call("GET", path, nil, a...).expect(200)
		for _, o := range r.list("data") {
			id := o.(map[string]any)["id"].(string)
			if seen[id] {
				t.Fatalf("order %s on two pages", id)
			}
			seen[id] = true
		}
		pages++
		if r.str("has_more") != "true" {
			if r.str("next_cursor") != "" {
				t.Fatal("last page must have no next_cursor")
			}
			break
		}
		cursor = r.str("next_cursor")
	}
	if len(seen) != 4 || pages != 2 {
		t.Fatalf("saw %d orders in %d pages", len(seen), pages)
	}
	e.call("GET", "/orders?cursor=bm9wZQ", nil, a...).expectErr(422, "validation_failed")
}

func TestCardsNeverEchoTheNumber(t *testing.T) {
	e := newEnv(t)
	a := e.as("aziza")
	r := e.call("POST", "/me/cards", map[string]any{"number": "8600 0000 0000 0007", "exp_month": 12, "exp_year": 2031, "cvc": "123"}, a...).expect(201)
	if r.str("brand") != "uzcard" || r.str("last4") != "0007" || strings.Contains(string(r.Raw), "8600000000000007") {
		t.Fatalf("card: %s", r.Raw)
	}
	e.call("POST", "/me/cards", map[string]any{"number": "4242424242424241", "exp_month": 12, "exp_year": 2031, "cvc": "123"}, a...).expectErr(422, "validation_failed")
	e.call("POST", "/me/cards", map[string]any{"number": "4242424242424242", "exp_month": 1, "exp_year": 2026, "cvc": "123"}, a...).expectErr(402, "card_expired")
	if list := e.call("GET", "/me/cards", nil, a...).expect(200); len(list.list("data")) != 3 {
		t.Fatalf("cards: %s", list.Raw)
	}
	e.call("DELETE", "/me/cards/"+r.str("id"), nil, e.as("bekzod")...).expectErr(404, "card_not_found")
	e.call("DELETE", "/me/cards/"+r.str("id"), nil, a...).expect(204)
}

func TestJobs(t *testing.T) {
	e := newEnv(t)
	a := e.as("aziza")
	start := e.call("POST", "/stock-checks", map[string]any{"product_id": "prd_0001"}, a...).expect(202)
	loc := start.Header.Get("Location")
	if !strings.HasPrefix(loc, "/jobs/") || start.str("status") != "running" || start.num("progress") != 0 {
		t.Fatalf("start: %s", start.Raw)
	}
	e.advance(5 * time.Second)
	mid := e.call("GET", loc, nil, a...).expect(200)
	if mid.str("status") != "running" || mid.num("progress") != 50 || mid.Header.Get("Retry-After") == "" {
		t.Fatalf("halfway: %s", mid.Raw)
	}
	e.call("GET", loc, nil, e.as("bekzod")...).expectErr(404, "job_not_found")
	e.advance(6 * time.Second)
	done := e.call("GET", loc, nil, a...).expect(200)
	if done.str("status") != "succeeded" || len(done.JSON["result"].(map[string]any)["branches"].([]any)) != 6 {
		t.Fatalf("done: %s", done.Raw)
	}

	failing := e.call("POST", "/stock-checks", map[string]any{"product_id": "prd_0001", "simulate_failure": true}, a...).expect(202)
	e.advance(11 * time.Second)
	f := e.call("GET", failing.Header.Get("Location"), nil, a...).expect(200)
	if f.str("status") != "failed" || !strings.Contains(f.str("error", "message"), "supplier_timeout") {
		t.Fatalf("failed job: %s", f.Raw)
	}

	staff := e.as("dilnoza")
	rep := e.call("POST", "/staff/reports/sales", map[string]string{"from": "2026-10-01", "to": "2026-10-31"}, staff...).expect(202)
	e.advance(11 * time.Second)
	report := e.call("GET", rep.Header.Get("Location"), nil, staff...).expect(200)
	if report.num("result", "paid_orders") != 3 {
		t.Fatalf("report: %s", report.Raw)
	}
}

func TestFailuresOnPurpose(t *testing.T) {
	e := newEnv(t)
	r := e.call("GET", "/_test/fail", nil).expectErr(503, "simulated_failure")
	if r.Header.Get("Retry-After") != "3" {
		t.Fatal("503 should carry Retry-After")
	}
	e.call("GET", "/_test/fail?status=500", nil).expectErr(500, "simulated_failure")
	e.call("GET", "/_test/fail?status=200", nil).expectErr(422, "validation_failed")
	e.call("GET", "/_test/fail?status=503&flaky_key=a", nil).expect(503)
	e.call("GET", "/_test/fail?status=503&flaky_key=a", nil).expect(200)
	e.call("GET", "/_test/delay?seconds=0", nil).expect(200)
	e.call("GET", "/nope", nil).expectErr(404, "route_not_found")
}

func TestWebhookReceiver(t *testing.T) {
	e := newEnv(t)
	body := `{"type":"customer.signed_in","data":{"id":"usr_aziza"}}`
	stamp := "1792659600" // 2026-10-22T09:00:00Z, the test clock
	sig := WebhookSignature("webhook-secret", stamp, []byte(body))
	e.call("POST", "/_test/webhooks", body, "X-Webhook-Timestamp", stamp, "X-Webhook-Signature", sig).expect(200)
	e.call("POST", "/_test/webhooks", body, "X-Webhook-Timestamp", stamp, "X-Webhook-Signature", "sha256=00").expectErr(401, "webhook_signature_invalid")
	e.call("POST", "/_test/webhooks", body).expectErr(401, "webhook_signature_invalid")
	in := e.call("GET", "/_test/inbox?kind=webhook_received", nil).expect(200)
	if len(in.list("data")) != 3 {
		t.Fatalf("every delivery should be recorded: %s", in.Raw)
	}
}

func TestResetRestoresSeed(t *testing.T) {
	e := newEnv(t)
	a := e.as("aziza")
	e.call("DELETE", "/orders/ord_aziza_cancelled", nil, a...).expect(204)
	e.call("POST", "/_test/reset", nil).expect(200)
	e.call("GET", "/orders/ord_aziza_cancelled", nil, e.as("aziza")...).expect(200)
	// Old tokens belong to sessions that no longer exist.
	e.call("GET", "/me", nil, a...).expectErr(401, "session_ended")
}

func TestErrorMessagesFollowLocale(t *testing.T) {
	e := newEnv(t)
	r := e.call("GET", "/me", nil, "Locale", "uz").expectErr(401, "token_missing")
	if !strings.Contains(r.str("error", "message"), "Avtorizatsiya") {
		t.Fatalf("uz message: %s", r.Raw)
	}
}

func TestDocs(t *testing.T) {
	e := newEnv(t)
	for _, page := range []string{"/docs/", "/docs/refunds", "/docs/errors", "/docs/testing"} {
		r := e.call("GET", page, nil).expect(200)
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/html") {
			t.Fatalf("%s: %s", page, r.Header.Get("Content-Type"))
		}
	}
	if r := e.call("GET", "/docs/refunds", nil); !strings.Contains(string(r.Raw), "200 000 UZS") {
		t.Fatal("the refund page should state the threshold")
	}
	e.call("GET", "/docs/nope", nil).expectErr(404, "page_not_found")
	spec := e.call("GET", "/openapi.yaml", nil).expect(200)
	if !strings.Contains(string(spec.Raw), "https://pharmacy.test.local") || strings.Contains(string(spec.Raw), "pharmacy.example.test") {
		t.Fatal("the served spec should carry PUBLIC_URL as its only server")
	}
}
