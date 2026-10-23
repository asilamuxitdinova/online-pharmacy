# Dorixona — the Test Pharmacy frontend

The storefront and test console for `../backend`. It is TypeScript, React 19 and Vite, with Motion for animation and React Three Fiber/drei for the 3D pills.

```bash
cd ../backend && make dev      # API on :8080 (60s tokens, code 123456)
make dev                       # this app on http://localhost:5173
```

If the API runs elsewhere, use `make dev PHARMACY_API=http://127.0.0.1:8090`. The dev server proxies `/api`, `/oauth/*`, `/docs` and `/openapi.yaml` to the backend, so no CORS is needed.

## Pages

| Route | What it exercises |
|---|---|
| `/` | 3D hero (click a pill), categories, house rules |
| `/shop` | Page paging, filters, sorting, search in en/ru/uz, empty results |
| `/p/:id` | Draggable 3D model of the product's form, the slow stock-check job on a branch map |
| `/checkout` | 422 field errors on the right field, `Idempotency-Key`, 402 declines, an explicit "I confirm" before paying |
| `/login` | Password (lockout dots), one-time code (live inbox), OAuth + PKCE, cookie session, API key, sign-up |
| `/account` | Cursor-paged orders, prescriptions, cards (number never kept), addresses, API keys shown once |
| `/orders/:id` | Timeline, PATCH vs cancel vs delete, HTML invoice, the refund threshold meter |
| `/staff` | Refund queue, prescription review, all orders, customers (TIN blurred), `X-Act-As`, sales report job |
| `/admin` | Role changes, disable/enable, delete with typed-email confirmation |
| `/lab` | Token lifetime ring, renewal, spent-token replay, request log, failures on demand, webhook signing, inbox, reset |

## Layout

- `src/lib`: API client (renewal, single-flight refresh, activity log), auth, i18n, cart, PKCE.
- `src/three`: procedural models (capsule, tablet, bottle, tube, spray, sachet) and scenes. They are lazy-loaded into their own chunk.
- `src/components`, `src/pages`.
