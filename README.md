# Online Pharmacy (Dorixona)

A made-up online pharmacy that exists to test [Frege](https://github.com/MultiAI-Labs/frege). It gives Frege a real API to sign in to, read from, write to and fail against. Every person, card number and medicine in it is fake.

It follows [`Frege-testing-API-guide.pdf`](Frege-testing-API-guide.pdf): every kind of sign-in, four access levels, paging, safe retries, dangerous actions, slow jobs, sensitive fields, failures on demand, policy docs and a careful OpenAPI spec.

| Part | What it is |
|---|---|
| [`backend/`](backend/) | The API. Go with embedded SQLite, one static binary, run by systemd. The spec is `backend/openapi/openapi.yaml`. |
| [`frontend/`](frontend/) | The storefront and test console. TypeScript, React, Motion and React Three Fiber. |

## Run it locally

```bash
make -C backend dev        # API on 127.0.0.1:8080, 60s tokens, sign-in code 123456
make -C frontend dev       # app on http://localhost:5173
```

If port 8080 is taken, run `make -C backend dev ADDR=127.0.0.1:8090`, then start the frontend with `PHARMACY_API=http://127.0.0.1:8090`.

Demo accounts (`aziza@pharmacy.test` / `Customer123!`, staff, admin and a disabled user) are listed in [`backend/README.md`](backend/README.md) and on the sign-in page.

## Checks

```bash
make -C backend check      # gofmt, vet, tests (including spec ↔ router parity)
make -C backend frege-check  # load the spec with Frege's own parser (needs a Frege checkout)
make -C frontend check     # typecheck and production build
```
