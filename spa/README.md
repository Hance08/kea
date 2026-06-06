# kea SPA

React SPA paired with `kea serve`. Currently ships one route — `/balances` — backed by `GET /api/balances`.

## Stack

- Vite + React + TypeScript
- TanStack Router (file-based routes)
- TanStack Query (data fetching)
- Tailwind + shadcn/ui (styling and primitives)
- Biome (lint + format)
- Vitest + @testing-library/react (tests)

## First-time setup

```bash
cd spa
npm install
```

## Dev workflow

Two terminals:

```bash
# Terminal 1 — Go API on :8080
make run

# Terminal 2 — SPA dev server on :5173
cd spa && npm run dev
# or: make spa-dev
```

Open <http://localhost:5173>. Vite proxies `/api/**` to `http://localhost:8080`.

## Scripts

- `npm run dev` — Vite dev server on :5173
- `npm run build` — Production build to `spa/dist/`
- `npm run preview` — Preview the production build
- `npm run test` — Run all tests once (Vitest)
- `npm run test:watch` — Watch mode
- `npm run check` — Biome lint + format check
- `npm run check:write` — Apply Biome fixes

## Configuration

`VITE_DEFAULT_CURRENCY` — currency used for the Net Worth headline. Defaults to `USD` if unset. Copy `.env.example` to `.env` to override.

## Status

- `/balances` — Net Worth dashboard
- `/accounts`, `/transactions`, `/reports`, `/reconcile` — sidebar stubs (disabled)
- Embed via `go:embed` for single-binary distribution
- `GET /api/config` endpoint for server-side default currency
