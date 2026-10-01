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
   - `a.$id.tsx` is `/a/$id`; read the param with `Route.useParams()` (strings, so convert with `Number`). `a.$id.edit.tsx` nests under it.
   - Each file exports `Route = createFileRoute('/a/$id')({ component })`. Keep route files thin and render a component from `spa/src/components/<feature>/`.
   - Run `npm run dev` or `npm run build` once. The `TanStackRouterVite` plugin regenerates `spa/src/routeTree.gen.ts`; commit it. See [development.md](../development.md#spa-toolchain).
3. Validate search params if the page has filters.
   - Define a zod schema and a `parseXSearch` function in a `*-search-params.ts` file under `spa/src/lib/` (`spa/src/lib/transactions-search-params.ts` is the model).
   - Attach it on the layout route: `validateSearch: (s) => parseTransactionsSearch(s)` in `spa/src/routes/transactions.tsx`.
   - Read with `Route.useSearch()` and change with `useNavigate`. Reset `offset` to 0 when a filter changes.
4. Fetch with TanStack Query.
   - `useQuery({ queryKey, queryFn })` where `queryFn` is the client function. Handle `isPending` (`Skeleton`), `isError` (destructive `Alert` with a retry button) and empty states, as in `spa/src/routes/transactions.index.tsx`.
   - Query keys are inline arrays that start with the resource name, then params: `['transactions', filter]`, `['unreconciled', accountId]`, `['account', id, 'balance']`. There is no key factory.
   - The default `QueryClient` in `spa/src/main.tsx` sets `staleTime` 30s and `refetchOnWindowFocus: false`.
5. Mutate with `useMutation` and invalidate by key prefix.
   - In `onSuccess` call `qc.invalidateQueries({ queryKey: [...] })` for every list or aggregate the write affects (`ReconcileWorkspace` invalidates `unreconciled`, `balances`, `transactions`, account balance and balance history).
   - In `onError` branch on `e instanceof ApiError` and `e.status` / `e.field` / `e.difference`; show failures with `toast` from `sonner`.
6. Format amounts with `useAmountFormat()` from `spa/src/lib/server-config.tsx`.
   - It returns `formatCents(cents, currency)`, `formatAmount(cents)` and `formatBalanceAbs(cents)` and applies the server's `display.hide_decimals` setting.
   - The pure helpers are in `spa/src/lib/format.ts`. Never divide cents by 100 in a component.
7. Build UI from `spa/src/components/ui/` (shadcn primitives: `button`, `card`, `alert`, `input`, `skeleton`, and others).
   - Add a missing primitive with the shadcn CLI, using `spa/components.json` (aliases `@/components/ui`, `@/lib/cn`). Biome ignores that directory.
   - Compose class names with `cn` from `spa/src/lib/cn.ts`.
8. Add the sidebar entry in `spa/src/components/Sidebar.tsx`: one object in `NAV`, `{ label, to, prefix: true }`. `prefix` keeps the item active on sub-routes. Pages like Settings go in `FOOTER_NAV`.
9. Remember filters per ledger (optional, for list pages with filters).
   - Add the page id to `PageId` in `spa/src/lib/filter-memory.ts`.
   - On the index route set `loaderDeps: ({ search }) => search` and `loader: makeFilterMemoryLoader({ pageId, defaults, redirectTo })`, with `defaults = parseXSearch({})`.
   - Call `clearFilters(pageId)` from the page's Clear action. State is kept in `localStorage` under a key built by `filterKey` from the ledger name and page id, keyed by the ledger set via `setActiveLedger`.
10. Write tests (see Checklist), then build as described in [development.md](../development.md#building-the-embedded-spa).

## Worked example
The reconcile page landed as one commit per layer. Copy the order.
- `be472c0` — `spa/src/lib/reconcile.ts` and response types in `spa/src/lib/types.ts`, with `spa/src/test/api.reconcile.test.ts` (stubs `fetch`, checks URL and body).
- `be6a4b9` — `ApiError` carries `difference` from the 409 `balance_mismatch` body. Make the error contract usable before any component needs it.
- `ea195b4`, `fc47d88` — leaf components `ReconcileHeader` and `UnreconciledTable` with tests in `spa/src/test/components/`.
- `7f80bd3` — `spa/src/routes/reconcile.$id.tsx` and `reconcile.tsx` (layout), the `ReconcileWorkspace` component with its mutation and invalidation, and the regenerated `routeTree.gen.ts`.
- `c7fc50e` — `reconcile.index.tsx` chooser route, `AccountChooser`, and `spa/src/test/reconcile.chooser.test.tsx`.
- `9e4ee0b` — the sidebar link (`NAV` entry with `prefix: true`) and `spa/src/test/sidebar.reconcile.test.tsx`.
- `1d34109` — per-ledger filter memory on `transactions.index.tsx`, with `spa/src/test/transactions.filter-memory.test.tsx`.
- `cda1cdd` — refreshed the committed bundle in `internal/web/dist`. This is obsolete: the bundle is now gitignored, so never commit build output.

## Conventions
- Imports use the `@` alias (`@/lib/...`, `@/components/...`). Biome enforces style; see [development.md](../development.md#spa-toolchain).
- Amounts stay int64 cents in state and props; format only at render time.
- Ledger switch: `LedgerSwitcher` calls `setActiveLedger(name)` then `queryClient.invalidateQueries()` with no filter, so every query refetches. Pages need no ledger handling, but all server data must come through `useQuery`, not local state.
- Server config (currency, `hide_decimals`) is loaded once by `ServerConfigProvider`; do not fetch it again in a page.
- Tests mock the client modules, not `fetch`: `vi.mock('@/lib/reconcile', async () => ({ ...(await vi.importActual<object>('@/lib/reconcile')), fn: mock }))`.
  - Also mock `@/lib/api` `getConfig` and `getBalances` when rendering the full app (the sidebar and provider call them), as in `spa/src/test/reconcile.workspace.test.tsx`.
  - Render a route with `render(makeTestApp('/reconcile/3'))` from `spa/src/test/test-app.tsx`. It builds a memory-history router from the real `routeTree`, with retries off and `gcTime` 0.
  - Component tests that skip the router wrap in `withServerConfig(...)`, which stubs the config context.
  - `spa/src/test/setup.tsx` is global setup (jest-dom, cleanup, a `localStorage` shim). Keep it free of `routeTree` imports so `vi.mock` runs first.
- `spa/README.md` still describes most pages as stubs; trim it rather than copying its status text.

## Checklist
- [ ] API client test in `spa/src/test/` (URL, method, body, error mapping)
- [ ] Component tests for leaf components, in `spa/src/test/components/`
- [ ] Route-level test with `makeTestApp` covering loading, error, empty and success, plus one mutation path
- [ ] Filter-memory test if the page remembers filters (`*.filter-memory.test.tsx`)
- [ ] Regenerated `spa/src/routeTree.gen.ts` committed
- [ ] Update spa/README.md route table if adding a top-level page
- [ ] `npm test` and `npm run check` pass in `spa/`
