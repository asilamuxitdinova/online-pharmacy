package api

import (
	"net/http"
	"strings"

	"github.com/asilamuxitdinova/online-pharmacy/backend/docs"
	"github.com/asilamuxitdinova/online-pharmacy/backend/openapi"
)

// Route is one endpoint. The table is exported so a test can check that
// every route is in the OpenAPI spec and every spec operation is routed.
type Route struct {
	Pattern string // "METHOD /path"
	InSpec  bool   // false for tester helpers and browser pages kept out of the tool list
	handler http.Handler
}

func (s *Server) Routes() []Route { return s.table }

func (s *Server) routes() {
	h := s.handle
	spec := func(pattern string, handler http.Handler) Route { return Route{pattern, true, handler} }
	hidden := func(pattern string, handler http.Handler) Route { return Route{pattern, false, handler} }

	s.table = []Route{
		// Authorization
		spec("POST /auth/register", h(public, s.register)),
		spec("POST /auth/login", h(public, s.login)),
		spec("POST /auth/otp/request", h(public, s.requestOTP)),
		spec("POST /auth/otp/verify", h(public, s.verifyOTP)),
		spec("POST /auth/refresh", h(public, s.refresh)),
		spec("POST /auth/logout", h(customer, s.logout)),
		spec("POST /auth/session", h(public, s.createCookieSession)),
		spec("DELETE /auth/session", h(customer, s.deleteCookieSession)),
		spec("GET /oauth/authorize", h(public, s.authorizeForm)),
		hidden("POST /oauth/authorize", h(public, s.authorizeSubmit)),
		spec("POST /oauth/token", h(public, s.oauthToken)),

		// The signed-in user
		spec("GET /me", h(customer, s.me)),
		spec("PATCH /me", h(customer, s.updateMe)),
		spec("GET /me/addresses", h(customer, s.listAddresses)),
		spec("POST /me/addresses", h(customer, s.addAddress)),
		spec("DELETE /me/addresses/{id}", h(customer, s.deleteAddress)),
		spec("GET /me/cards", h(customer, s.listCards)),
		spec("POST /me/cards", h(customer, s.addCard)),
		spec("DELETE /me/cards/{id}", h(customer, s.deleteCard)),
		spec("GET /me/api-keys", h(customer, s.listAPIKeys)),
		spec("POST /me/api-keys", h(customer, s.createAPIKey)),
		spec("DELETE /me/api-keys/{id}", h(customer, s.revokeAPIKey)),

		// Catalog (public)
		spec("GET /health", h(public, s.health)),
		spec("GET /categories", h(public, s.listCategories)),
		spec("GET /products", h(public, s.listProducts)),
		spec("GET /products/export", h(public, s.exportProducts)),
		spec("GET /products/{id}", h(public, s.getProduct)),

		// Prescriptions
		spec("GET /prescriptions", h(customer, s.listPrescriptions)),
		spec("POST /prescriptions", h(customer, s.addPrescription)),
		spec("GET /prescriptions/{id}", h(customer, s.getPrescription)),

		// Orders
		spec("GET /orders", h(customer, s.listOrders)),
		spec("POST /orders", h(customer, s.idempotent(s.createOrder))),
		spec("GET /orders/{id}", h(customer, s.getOrder)),
		spec("PUT /orders/{id}", h(customer, s.replaceOrder)),
		spec("PATCH /orders/{id}", h(customer, s.updateOrder)),
		spec("DELETE /orders/{id}", h(customer, s.deleteOrder)),
		spec("POST /orders/{id}/cancel", h(customer, s.cancelOrder)),
		spec("POST /orders/{id}/pay", h(customer, s.idempotent(s.payOrder))),
		spec("GET /orders/{id}/invoice", h(customer, s.invoice)),
		spec("GET /orders/{id}/refunds", h(customer, s.listOrderRefunds)),
		spec("POST /orders/{id}/refunds", h(customer, s.idempotent(s.requestRefund))),

		// Slow work
		spec("POST /stock-checks", h(customer, s.startStockCheck)),
		spec("GET /jobs/{id}", h(customer, s.getJob)),

		// Staff
		spec("GET /staff/orders", h(staff, s.staffOrders)),
		spec("GET /staff/customers", h(staff, s.listUsers([]string{"customer"}))),
		spec("GET /staff/customers/{id}", h(staff, s.getCustomer)),
		spec("GET /staff/refunds", h(staff, s.listRefundsForReview)),
		spec("POST /staff/refunds/{id}/approve", h(staff, s.decideRefund(true))),
		spec("POST /staff/refunds/{id}/reject", h(staff, s.decideRefund(false))),
		spec("GET /staff/prescriptions", h(staff, s.staffPrescriptions)),
		spec("POST /staff/prescriptions/{id}/review", h(staff, s.reviewPrescription)),
		spec("POST /staff/reports/sales", h(staff, s.startSalesReport)),

		// Admin
		spec("GET /admin/users", h(admin, s.listUsers([]string{"customer", "staff", "admin"}))),
		spec("PATCH /admin/users/{id}/role", h(admin, s.changeRole)),
		spec("POST /admin/users/{id}/disable", h(admin, s.setDisabled(true))),
		spec("POST /admin/users/{id}/enable", h(admin, s.setDisabled(false))),
		spec("DELETE /admin/users/{id}", h(admin, s.deleteUser)),

		// Partner (signed requests)
		spec("GET /partner/inventory", s.signed(s.partnerInventory)),
		spec("POST /partner/restock", s.signed(s.partnerRestock)),

		// Failures on purpose
		spec("GET /_test/fail", h(public, s.testFail)),
		spec("GET /_test/delay", h(public, s.testDelay)),

		// Spec and docs
		hidden("GET /openapi.yaml", http.HandlerFunc(s.serveSpec)),
		hidden("GET /docs/", http.HandlerFunc(serveDocs)),
		hidden("GET /{$}", http.RedirectHandler("/docs/", http.StatusFound)),
	}
	if s.cfg.TestHelpers {
		s.table = append(s.table,
			hidden("GET /_test/config", h(public, s.testConfig)),
			hidden("POST /_test/reset", h(public, s.reset)),
			hidden("GET /_test/inbox", h(public, s.inbox)),
			hidden("GET /_test/inbox/view", h(public, s.inboxView)),
			hidden("DELETE /_test/inbox", h(public, s.clearInbox)),
			hidden("POST /_test/webhooks", h(public, s.receiveWebhook)),
			hidden("POST /_test/webhooks/{name}", h(public, s.receiveWebhook)),
		)
	}
	for _, rt := range s.table {
		s.mux.Handle(rt.Pattern, rt.handler)
	}
	s.mux.HandleFunc("/", notFoundHandler)
}

// serveSpec fills in the one https server address from PUBLIC_URL.
func (s *Server) serveSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write([]byte(strings.ReplaceAll(openapi.Spec, openapi.ServerPlaceholder, s.cfg.PublicURL)))
}

func serveDocs(w http.ResponseWriter, r *http.Request) {
	page := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/docs/"), "/")
	if page == "" {
		page = "index"
	}
	body, ok := docs.Page(page)
	if !ok {
		writeError(w, r, errf(http.StatusNotFound, "page_not_found", "No documentation page %q. See /docs/.", page))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body)
}
