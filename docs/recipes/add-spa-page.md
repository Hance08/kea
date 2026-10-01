# Recipe: Add a SPA page

> **Read this when:** you need to add a new page, route, or API-backed screen to the React SPA in `spa/`, including its client code, tests, and sidebar entry.
>
> **Related:** [development.md](../development.md#spa-toolchain), [http-api.md](../http-api.md), [architecture.md](../architecture.md), [add-api-endpoint.md](add-api-endpoint.md), [decisions.md](../decisions.md)

## When to use
- A backend endpoint exists (or you add it first with [add-api-endpoint.md](add-api-endpoint.md)) and the UI needs a screen for it.
- An existing page needs a new sub-route such as `a.$id.edit.tsx`.
- Do NOT use this for a small change inside an existing component; edit it and its test directly.

## Steps
1. Add the API client function and wire types.
   - Types mirror the Go JSON (snake_case fields, amounts as int64 cents, timestamps as Unix seconds) in `spa/src/lib/types.ts`. Shapes are in [http-api.md](../http-api.md).
   - Put calls in a per-feature module such as `spa/src/lib/reconcile.ts`. Each function wraps `apiFetch<T>` from `spa/src/lib/api.ts`.
   - Writes pass `method`, a `Content-Type: application/json` header and `JSON.stringify(body)` (see `previewReconcile`).
   - `apiFetch` throws `ApiError` (`status`, `message`, optional `field`, optional `difference`) for any non-2xx response. To expose a new field from the error body, extend `ApiErrorBody` and `ApiError`.
2. Create the route files in `spa/src/routes/`. The file name is the URL.
   - `a.tsx` is the layout for `/a` and renders `<Outlet />` (`reconcile.tsx`). `a.index.tsx` is `/a/` (`reconcile.index.tsx`).
   - `a.$id.tsx` is `/a/$id`; read the param with `Route.useParams()` (strings, so convert with `Number`).
   - Adding `a.$id.edit.tsx` nests it under `a.$id.tsx`, which must then become a pure `<Outlet />` layout. Move the detail view to `a.$id.index.tsx` (see `spa/src/routes/accounts.$id.tsx` and `accounts.$id.index.tsx`).
   - A simple page can be one leaf file: `spa/src/routes/balances.tsx` holds `validateSearch`, the loader and the component together.
   - Each file exports `Route = createFileRoute('/a/$id')({ component })`. Keep route files thin and render a component from `spa/src/components/<feature>/`.
   - Regenerate `spa/src/routeTree.gen.ts` by running the dev server or a build, and commit it ([development.md](../development.md#spa-toolchain)).
3. Validate search params if the page has filters.
   - Define a zod schema and a `parseXSearch` function in a `*-search-params.ts` file under `spa/src/lib/` (`spa/src/lib/transactions-search-params.ts` is the model).
   - Attach it on the layout route: `validateSearch: (s) => parseTransactionsSearch(s)` in `spa/src/routes/transactions.tsx`.
   - Read with `Route.useSearch()` and change with `useNavigate`. Reset `offset` to 0 when a filter changes.
4. Fetch with TanStack Query.
   - `useQuery({ queryKey, queryFn })` where `queryFn` is the client function. Handle `isPending` (`Skeleton`), `isError` (destructive `Alert` with a retry button) and empty states, as in `spa/src/routes/transactions.index.tsx`.
   - Query keys are inline arrays that start with the resource name, then params: `['transactions', filter]`, `['unreconciled', accountId]`, `['account', id, 'balance']`.
   - There is no key factory, and there are exceptions (`['dashboard', ...]`, `['balance-history']` in `spa/src/routes/balances.tsx` versus `['balances', 'history']` invalidated by `ReconcileWorkspace`).
   - Grep existing keys before choosing invalidation targets.
   - The default `QueryClient` in `spa/src/main.tsx` sets `staleTime` 30s and `refetchOnWindowFocus: false`.
5. Mutate with `useMutation` and invalidate by key prefix.
   - In `onSuccess` call `qc.invalidateQueries({ queryKey: [...] })` for every list or aggregate the write affects (`ReconcileWorkspace` invalidates `unreconciled`, `balances`, `transactions`, account balance and balance history).
   - In `onError` branch on `e instanceof ApiError` and `e.status` / `e.field` / `e.difference`; show failures with `toast` from `sonner`.
6. For create or edit forms, copy `spa/src/routes/accounts.new.tsx` and `spa/src/components/accounts/AccountForm.tsx`.
   - Inputs are controlled `useState`; `ApiError.field` maps to inline field errors and anything else to a form-level alert. `react-hook-form` is in `spa/package.json` but unused.
   - There is no shared amount parser. Forms parse locally with `Math.round(n * 100)` (`parseAmount` in `AccountForm.tsx`, `parseCents` in `spa/src/components/transactions/TransactionForm.tsx`).
7. Format amounts with `useAmountFormat()` from `spa/src/lib/server-config.tsx`.
   - It returns `formatCents(cents, currency)`, `formatAmount(cents)` and `formatBalanceAbs(cents)` and applies the server's `display.hide_decimals` setting.
   - The pure helpers are in `spa/src/lib/format.ts`. Never divide cents by 100 in a component.
8. Build UI from `spa/src/components/ui/` (shadcn primitives: `button`, `card`, `alert`, `input`, `skeleton`, and others).
   - Add a missing primitive with the shadcn CLI, using `spa/components.json` (aliases `@/components/ui`, `@/lib/cn`). Biome ignores that directory.
   - Compose class names with `cn` from `spa/src/lib/cn.ts`.
9. Add the sidebar entry in `spa/src/components/Sidebar.tsx`: one object in `NAV`, `{ label, to, prefix: true }`. `prefix` keeps the item active on sub-routes. Pages like Settings go in `FOOTER_NAV`.
10. Remember filters per ledger (optional, for list pages with filters).
   - Add the page id to `PageId` in `spa/src/lib/filter-memory.ts`.
   - On the index route set `loaderDeps: ({ search }) => search` and `loader: makeFilterMemoryLoader({ pageId, defaults, redirectTo })`, with `defaults = parseXSearch({})`.
   - Call `clearFilters(pageId)` from the page's Clear action. State is kept in `localStorage` under a key built by `filterKey` from the ledger name and page id, keyed by the ledger set via `setActiveLedger`.
11. Write tests (see Conventions and Checklist), then build as described in [development.md](../development.md#building-the-embedded-spa).

## Worked example
The reconcile page landed as one commit per layer. Copy the order.
- `be472c0` — `spa/src/lib/reconcile.ts` and response types in `spa/src/lib/types.ts`, with `spa/src/test/api.reconcile.test.ts` (stubs `fetch`, checks URL and body).
- `be6a4b9` — `ApiError` carries `difference` from the 409 `balance_mismatch` body. Make the error contract usable before any component needs it.
- `ea195b4` — `ReconcileHeader` component with `spa/src/test/components/reconcile-header.test.tsx`.
- `fc47d88` — `UnreconciledTable` component with `spa/src/test/components/unreconciled-table.test.tsx`.
- `7f80bd3` — `spa/src/routes/reconcile.$id.tsx` and `reconcile.tsx` (layout), the `ReconcileWorkspace` component with its mutation and invalidation, and the regenerated `routeTree.gen.ts`.
- `c7fc50e` — `reconcile.index.tsx` chooser route, `AccountChooser`, and `spa/src/test/reconcile.chooser.test.tsx`.
- `9e4ee0b` — the sidebar link (`NAV` entry with `prefix: true`) and `spa/src/test/sidebar.reconcile.test.tsx`.
- `1d34109` — per-ledger filter memory on `transactions.index.tsx`, with `spa/src/test/transactions.filter-memory.test.tsx`.
- `cda1cdd` — updated only the tracked placeholder `internal/web/dist/index.html`.
  The rest of `internal/web/dist` was already ignored. Do not repeat it: `npm run build` rewrites that file,
  so restore it with `make spa-clean` or `git checkout internal/web/dist/index.html`.

## Conventions
- Imports use the `@` alias (`@/lib/...`, `@/components/...`). Biome enforces style; see [development.md](../development.md#spa-toolchain).
- Amounts stay int64 cents in state and props; format only at render time.
- Ledger switch: `LedgerSwitcher` calls `setActiveLedger(name)` then `queryClient.invalidateQueries()` with no filter, so every query refetches. Pages need no ledger handling, but all server data must come through `useQuery`, not local state.
- Server config (currency, `hide_decimals`) is loaded once by `ServerConfigProvider`; do not fetch it again in a page.
- Tests use one of two patterns (see [development.md](../development.md#spa-tests)).
  - `vi.mock` on client modules, spreading `vi.importActual` and replacing only the needed functions (`spa/src/test/reconcile.workspace.test.tsx`; about 13 files).
  - `vi.stubGlobal('fetch', ...)` routing by URL (`spa/src/test/transactions.list.test.tsx`, `balances.test.tsx`, `settings.test.tsx`; most route tests).
  - Full-app renders also hit `/api/config` (server config) and `/api/ledgers` (sidebar `LedgerSwitcher`, which shows plain "kea" on error). A fetch stub must answer both; with `vi.mock` of `@/lib/api`, mock `getConfig` and `getLedgers`.
  - Render a route with `makeTestApp('/reconcile/3')` from `spa/src/test/test-app.tsx`; component tests without the router use `withServerConfig`.
- `spa/README.md` still describes most pages as stubs; trim it rather than copying its status text.

## Checklist
- [ ] API client test in `spa/src/test/` (URL, method, body, error mapping)
- [ ] Component tests for leaf components, in `spa/src/test/components/`
- [ ] Route-level test with `makeTestApp` covering loading, error, empty and success, plus one mutation path
- [ ] Filter-memory test if the page remembers filters (`*.filter-memory.test.tsx`)
- [ ] Regenerated `spa/src/routeTree.gen.ts` committed
- [ ] Update `spa/README.md` route table if adding a top-level page
- [ ] `npm test` and `npm run check` pass in `spa/`
