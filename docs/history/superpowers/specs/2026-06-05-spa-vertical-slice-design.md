# SPA Vertical Slice — Balances Dashboard

**Status:** Design approved 2026-06-05.
**Predecessor:** [`2026-06-02-pre-development-review.md`](../../web-layer/2026-06-02-pre-development-review.md) §6 item 7 ("React SPA: Build incrementally against the running API").
**Sibling:** [`2026-06-05-web-api-bulk-balances-design.md`](2026-06-05-web-api-bulk-balances-design.md) — the API endpoint this slice consumes.

## Goal

Stand up a React SPA at `spa/` that pairs with `kea serve`, and ship one working route — `/balances` — with the chrome (sidebar shell) the rest of the app will grow into. The Balances route renders a Net Worth headline, type totals, and a per-account list from `GET /api/balances`.

This is a **vertical slice**: the tech-stack foundation + one fully-built screen + the dev workflow to keep building. Other routes are stubbed (visible in sidebar, disabled).

## Tech stack

| Concern | Choice | Rationale |
|---|---|---|
| Build tool | **Vite 5** | Recommended in pre-dev review; fast HMR, simple config |
| Framework | **React 18 + TypeScript 5** | End-to-end type safety against API contract |
| Routing | **TanStack Router** | Fully typed routes, search-param helpers, generated route tree; same team as TanStack Query |
| Data fetching | **TanStack Query v5** | Idiomatic for ListResult shape; built-in pending/error/refetch |
| Styling | **Tailwind 3 + shadcn/ui** | Copy-paste accessible components, no runtime component-library dependency |
| Lint + format | **Biome** | Single binary; replaces ESLint + Prettier |
| Tests | **Vitest + @testing-library/react + jsdom** | Same module graph as Vite; one config |

## Location

`spa/` at repo root, sibling to `cmd/`, `internal/`, `ui/`, `migrations/`. Chosen for the short name and parity with the pre-dev review's wording.

## File structure

```
spa/
├── package.json
├── tsconfig.json
├── tsconfig.node.json
├── vite.config.ts          # dev server + /api/** proxy to localhost:8080
├── biome.json
├── index.html
├── README.md
├── src/
│   ├── main.tsx            # React root + QueryClientProvider + RouterProvider
│   ├── routes/
│   │   ├── __root.tsx      # layout shell (Sidebar + <Outlet/>)
│   │   ├── index.tsx       # / -> redirect to /balances
│   │   └── balances.tsx    # the only built route
│   ├── components/
│   │   ├── Sidebar.tsx     # 5 nav items, only Balances enabled
│   │   ├── NetWorthCard.tsx
│   │   ├── TypeTotalCard.tsx
│   │   └── AccountListRow.tsx
│   ├── components/ui/      # shadcn/ui scaffolded: button, card, skeleton, alert
│   ├── lib/
│   │   ├── api.ts          # typed fetch wrapper + getBalances()
│   │   ├── types.ts        # hand-written mirrors of Go types we consume
│   │   ├── balances.ts     # aggregation: summarizeBalances()
│   │   └── format.ts       # formatCents(cents, currency) via Intl.NumberFormat
│   ├── styles/
│   │   └── globals.css     # Tailwind directives + theme tokens
│   └── test/
│       ├── setup.ts        # vitest + testing-library setup
│       └── balances.test.tsx
├── public/
│   └── favicon.svg
└── .gitignore              # node_modules, dist, .DS_Store, *.local
```

Also: a top-level `.gitignore` addition for `spa/node_modules/` and `spa/dist/` so `git status` stays clean from the project root.

## Data shape (TypeScript)

`src/lib/types.ts` mirrors the Go structs that the SPA consumes:

```ts
export type AccountType = 'A' | 'L' | 'C' | 'R' | 'E';

export interface AccountBalance {
  account_id: number;
  name: string;
  type: AccountType;
  parent_id?: number;
  currency: string;
  amount: number;   // int64 cents
  is_hidden: boolean;
}

export interface ListResult<T> {
  items: T[];
  total_count: number;
  limit: number;
  offset: number;
}
```

Hand-written for the slice. A future task can codegen these from the Go JSON tags once the API surface stabilizes (tracked separately, not in this PR).

## Aggregation

`src/lib/balances.ts`:

```ts
export interface BalancesSummary {
  assetsTotal: number;       // int64 cents
  liabilitiesTotal: number;  // negative or zero (project convention)
  netWorth: number;          // assetsTotal + liabilitiesTotal
  included: AccountBalance[];
  excludedByCurrency: AccountBalance[]; // A/L rows in a non-target currency
  excludedByType: AccountBalance[];     // Equity / Revenue / Expense rows (always silent in this view)
}

export function summarizeBalances(
  rows: AccountBalance[],
  summaryCurrency: string,
): BalancesSummary;
```

Single pass over `rows`. A row is "included" iff `row.type === 'A' || row.type === 'L'` AND `row.currency === summaryCurrency`. Asset rows add to `assetsTotal`. Liability rows add to `liabilitiesTotal` as-stored (the project convention is liabilities stored negative: a $420 credit card balance is `-42000` cents, so adding it to net worth via `assets + liabilities` is the correct subtraction).

All amounts stay in int64 cents until formatting at the leaves via `formatCents`.

## Default currency (slice-only)

The headline targets one currency. For the slice, the SPA reads it from `import.meta.env.VITE_DEFAULT_CURRENCY` with `'USD'` as the fallback. `spa/.env.example` documents the variable.

A real `GET /api/config` endpoint (returning `{ default_currency: cfg.Defaults.Currency }`) is **out of scope** for this PR — flagged as the obvious next step once the SPA needs to read more server-side config.

## Routing

TanStack Router with file-based routes:

- `routes/__root.tsx` — persistent layout shell. Imports `Sidebar` on the left and `<Outlet />` for the main pane.
- `routes/index.tsx` — `/` redirects to `/balances`.
- `routes/balances.tsx` — the dashboard.

The other 4 sidebar entries (Accounts, Transactions, Reports, Reconcile) are rendered by `Sidebar.tsx` as visually-disabled items (`aria-disabled`, muted text, no `<Link>`). When their routes are built later, swap each to a `<Link>` and the sidebar auto-enables.

## Components

- **`NetWorthCard`** — hero block at top of the dashboard. Big `netWorth` headline (formatted), one-line breakdown ("$X assets − $Y liabilities"), small footnote if `excludedByCurrency.length > 0`: "N accounts in other currencies not included". Type-mismatched rows (Income/Expense/Equity) are silently filtered upstream and never trigger the footnote.
- **`TypeTotalCard`** — two stacked or side-by-side cards: Assets total (positive), Liabilities total (rendered as displayed-positive owed amount with a "−" prefix). Uses shadcn `Card`.
- **`AccountListRow`** — `{name, amount, currency}` row. Negative amounts red.
- **Sidebar** — fixed-width left rail. Active route highlighted via TanStack Router's `useRouterState`.

All using Tailwind for layout and the four shadcn primitives: `button`, `card`, `skeleton`, `alert`.

## Loading & error states

TanStack Query yields `isPending` / `isError` / `data`. The route renders:

- **Pending**: shadcn `Skeleton` placeholders matching the hero + 2 cards + 6 list rows. No layout shift between pending and success.
- **Error**: shadcn `Alert` (`variant="destructive"`) with the error message and a Retry button calling `query.refetch()`.
- **Empty success**: `items.length === 0` shows a centered message "No accounts yet — run `kea ledger add` then create one via the CLI."

## API client

`src/lib/api.ts`:

```ts
export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(path, init);
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ message: resp.statusText }));
    throw new ApiError(resp.status, body.message ?? resp.statusText, body.field);
  }
  return resp.json() as Promise<T>;
}

export function getBalances(): Promise<ListResult<AccountBalance>> {
  return apiFetch<ListResult<AccountBalance>>('/api/balances');
}
```

`ApiError` carries status + message + optional field — usable by future write paths but only `message` is shown in the slice.

## Dev workflow

`vite.config.ts` proxies `/api/**` to `http://localhost:8080`. CORS for `localhost:5173` is already permitted by `ServerConfig.CORSOrigins` defaults (PR #175), but the proxy avoids cross-origin entirely during dev.

`spa/README.md` documents two terminals:

```bash
# Terminal 1
make run                  # kea serve on :8080

# Terminal 2
cd spa
npm install               # first time only
npm run dev               # Vite on :5173
# open http://localhost:5173
```

`Makefile` gets three new targets:

```makefile
spa-install:
	cd spa && npm install

spa-dev:
	cd spa && npm run dev

spa-build:
	cd spa && npm run build
```

`spa-build` produces `spa/dist/` — the future `go:embed` target.

## Tests

One smoke test at `src/test/balances.test.tsx`:

```ts
test('renders Net Worth headline from a balances response', async () => {
  vi.spyOn(global, 'fetch').mockResolvedValueOnce(
    new Response(JSON.stringify({
      items: [
        { account_id: 1, name: 'Assets:Bank', type: 'A', currency: 'USD', amount: 125000, is_hidden: false },
        { account_id: 2, name: 'Assets:Cash', type: 'A', currency: 'USD', amount: 3500, is_hidden: false },
        { account_id: 3, name: 'Liab:Card',  type: 'L', currency: 'USD', amount: -42000, is_hidden: false },
      ],
      total_count: 3, limit: 0, offset: 0,
    }), { status: 200 }),
  );
  render(<TestApp initialPath="/balances" />);
  expect(await screen.findByText(/Net Worth/i)).toBeInTheDocument();
  expect(await screen.findByText(/\$865\.00/)).toBeInTheDocument(); // 125000 + 3500 - 42000 = 86500 cents
});
```

`TestApp` is a test helper exported from `src/test/setup.ts` that mounts the router at a given path with a fresh `QueryClient` and `MemoryHistory`.

The test proves: query fires, aggregation math is correct, components render, the slice doesn't crash. It establishes the testing infrastructure for future tests without overcommitting.

## Lint / format

`biome.json` enables recommended rules + format on save. One `npm run check` script runs lint + format-check; `npm run check:write` applies fixes.

## `package.json` scripts

```json
{
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "check": "biome check src",
    "check:write": "biome check --write src"
  }
}
```

## Verification

- `cd spa && npm install` — succeeds.
- `cd spa && npm run check` — passes.
- `cd spa && npm run test` — passes.
- `cd spa && npm run build` — produces `dist/` without errors.
- Manual: `make run` in one terminal, `cd spa && npm run dev` in another, browse `http://localhost:5173/balances` with a seeded ledger; dashboard renders, Net Worth math matches the seeded data.
- `go test ./...` — still green (no Go changes in this PR beyond Makefile targets).
- `go build ./...` — still clean.

## Out of scope (explicit)

- **The other 4 routes** (Accounts, Transactions, Reports, Reconcile). Stubs in sidebar only.
- **`go:embed` integration**. Requires `spa/dist/` to exist as a build artifact path; a follow-up wires the embed.
- **`GET /api/config`** for default currency. SPA uses `VITE_DEFAULT_CURRENCY` env var (default `'USD'`).
- **Multi-currency net worth**. Single-currency only, with the "N accounts in other currencies not included" footnote.
- **Hidden-account toggle in UI**. Always excludes hidden (the API default).
- **`as_of` date picker**. Always "now".
- **Auth / login**. Local-only single-user, per pre-dev review §1.
- **E2E / Playwright tests**. Vitest smoke only.
- **CI hooks / pre-commit integration**. Lint/test runnable locally; CI wiring is a separate concern.
- **TypeScript codegen from Go JSON tags**. Hand-written types for the slice.

## Commit shape

Single commit on a `feat/spa-vertical-slice` branch, scope `feat(spa)`:

```
feat(spa): vertical-slice SPA at spa/ with Balances dashboard

Stands up the React SPA workspace (Vite + React + TypeScript + TanStack
Router + TanStack Query + Tailwind + shadcn/ui + Biome + Vitest) at spa/
and ships one working route — /balances — backed by GET /api/balances.

The Balances route renders a Net Worth headline, Assets/Liabilities totals,
and a per-account list. Aggregation is client-side (single pass) over the
endpoint's flat list. Multi-currency is single-currency-only with a
"N accounts in other currencies not included" footnote.

The sidebar exposes 5 nav entries; only Balances is wired up. The other
four (Accounts, Transactions, Reports, Reconcile) render as disabled stubs
ready to be enabled as their routes ship.

Dev workflow: `make run` + `cd spa && npm run dev` in two terminals; Vite
proxies /api/** to localhost:8080. `npm run check` / `npm run test` /
`npm run build` all green. Single Vitest smoke test pins the aggregation
math and proves the slice mounts without crashing.

Out of scope: go:embed integration, the other 4 routes, /api/config,
multi-currency net worth, codegen of TypeScript types from Go JSON tags.
```
