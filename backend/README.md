# Test Pharmacy backend

An online pharmacy that exists only so Frege has something real to sign in to, read from, write to and fail against. It follows `../Frege-testing-API-guide.pdf`. Everything in it is made up.

Go, the standard library router, and embedded SQLite (pure Go, no CGO). The result is one static binary and one data file, run by systemd.

```bash
make dev        # http://127.0.0.1:8080: 60s tokens, one-time code 123456
make test       # the whole suite
make help       # every target
```

Open `http://127.0.0.1:8080/docs/` for the policy pages and `/openapi.yaml` for the spec. The demo inbox (one-time codes, webhook deliveries) is at `/_test/inbox/view`.

## Accounts

| Email | Password | Role |
|---|---|---|
| aziza@pharmacy.test | Customer123! | customer |
| bekzod@pharmacy.test | Customer123! | customer (for "someone else's record") |
| dilnoza@pharmacy.test | Staff123! | staff (pharmacist) |
| admin@pharmacy.test | Admin123! | admin |
| timur@pharmacy.test | Customer123! | customer, disabled |

API keys, test cards and the other seeded records are listed on `/docs/testing`.

## How the guide maps to the code

| Guide section | Where |
|---|---|
| 1. Authorization | `internal/api/auth.go`, `auth_handlers.go`, `oauth.go`, `partner.go` |
| 2. Roles | `handle` / `authorize` in `server.go` and `auth.go`. The role is read from the database on every request. |
| 3. Reading | `catalog.go` (page paging, filters, locale, CSV), `orders.go` (cursor paging, HTML invoice) |
| 4. Writing | `orders.go` (PUT vs PATCH, DELETE 204, cancel action, `Idempotency-Key`), `errors.go` (400 vs 422) |
| 5. Dangerous | `payments.go` (pay, refunds with the 200 000 UZS rule), `people.go` (roles, disable, delete) |
| 6. Slow work | `jobs.go`. Status is derived from the clock, so no worker is needed. |
| 7. Sensitive data | Taxpayer numbers in `staffUser`, cards keep brand and last four only, API keys shown once |
| 8. Failures | `testhelpers.go`: `/_test/fail`, `/_test/delay` |
| 9. Docs and spec | `docs/pages/*.html`, `openapi/openapi.yaml` |
| 10. Helpers | `testhelpers.go`: inbox, reset, webhook receiver, `/_test/config` |

## Checking the spec

- `make spec-check` checks that every route is in the spec and every operation is routed. It also checks that operation ids are unique snake_case, that request bodies are JSON, and that exactly the sign-in operations carry `x-frege-auth-op`.
- `make frege-check` loads the spec with Frege's own parser from `FREGE_DIR` (default `~/Documents/frege-backend`) and prints the tool list Frege would build. It uses a Go build overlay, so nothing is written to the Frege checkout.

## Deployment (systemd)

```bash
make build
sudo make install    # binary, units, /etc/frege-pharmacy/pharmacy.env, service user
sudoedit /etc/frege-pharmacy/pharmacy.env
sudo make enable     # service + nightly reset timer
```

The units are in `deploy/systemd/`. Every setting is documented in `deploy/pharmacy.env.example`. The service listens on `127.0.0.1:8080` and expects a TLS reverse proxy in front, because `PUBLIC_URL` must be https.
