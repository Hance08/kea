# SPA Filter Memory

## Problem

The SPA has filter/search state on seven routes — `/transactions`, `/accounts`, `/balances`, and the four `/reports/*` pages. State is stored in URL search params and validated per page by a Zod schema. When the user navigates away (sidebar click, browser back, a deep link), then returns to a route, the new URL has no search params, so filters and pagination reset to schema defaults.

Users in a personal-accounting workflow expect to come back to the view they left, even across browser sessions. Today they have to re-apply filters every visit.

## Goal

When the user returns to any of the seven filterable pages, the page should auto-restore the most recent filter state, scoped to the active ledger, persisted across browser sessions. Explicit deep links continue to win — they override and update memory.

## Non-goals

- UI to inspect, export, import, or clear remembered filters across pages.
- Expiry, cross-device sync, or server-side persistence.
- Migration of pre-existing storage keys (none exist).

## Decisions

| Question | Decision |
|----------|----------|
| Scope | All seven filterable routes |
| Persistence | `localStorage` (across sessions) |
| Restore behavior | Auto-apply on entry; deep link with params overrides |
| Multi-ledger | Per-ledger memory; switching ledger surfaces that ledger's last view |
| Pagination | Remembered alongside filters |

## Approach

**Loader-based redirect.** Each filterable route gains a small TanStack Router `loader` that runs on every entry and every search-param change. The loader either restores remembered filters (via `throw redirect`) before the component renders, or saves the current URL search to storage.

This was chosen over a component-effect hook (visible flash, double render) and over a smart sidebar Link (only catches sidebar entries, not back button or other links).

## Module: `spa/src/lib/filter-memory.ts`

A single module owns all persistence. No other code reads or writes `localStorage` for filter state.

```ts
export type PageId =
  | 'transactions'
  | 'accounts'
  | 'balances'
  | 'reports/balance-sheet'
  | 'reports/income-statement'
  | 'reports/expense-breakdown'
  | 'reports/income-breakdown';

export function getActiveLedger(): string | null;
export function setActiveLedger(name: string): void;

export function loadFilters<T>(pageId: PageId): T | null;
export function saveFilters(pageId: PageId, search: object): void;
export function clearFilters(pageId: PageId): void;

export function isDefaultSearch(pageId: PageId, search: object): boolean;
```

### Storage keys

All under a `kea.` namespace for easy inspection and bulk clearing in DevTools:

- `kea.activeLedger` — plain string, the active ledger name
- `kea.filters.<ledgerName>.<pageId>` — JSON of the parsed search object

### Defaults check

`isDefaultSearch` compares the candidate against the schema's own defaults by calling the existing `parse*Search({})` per page. This avoids duplicating default values between the schema and the memory module.

### Robustness

- `JSON.parse` failure → treat as no memory.
- Loaded shape no longer validates against the current schema → treat as no memory. (Forward-compatible against future filter additions; an outdated key is silently ignored.)
- `localStorage` unavailable (privacy mode, quota exceeded) → all getters return `null`, setters are no-ops. Pages behave exactly as they do today.

## Route loaders

Each filterable route gains the same pattern, illustrated for transactions:

```ts
export const Route = createFileRoute('/transactions/')({
  loaderDeps: ({ search }) => search,
  loader: ({ search }) => {
    const ledger = getActiveLedger();
    if (!ledger) return;

    if (isDefaultSearch('transactions', search)) {
      const remembered = loadFilters<TransactionsSearch>('transactions');
      if (remembered && !isDefaultSearch('transactions', remembered)) {
        throw redirect({ to: '/transactions', search: remembered });
      }
      return;
    }

    saveFilters('transactions', search);
  },
  component: TransactionsListPage,
});
```

### Anti-loop guard

The redirect only fires when all three hold:

1. URL search equals defaults.
2. Memory exists for `(ledger, page)`.
3. Remembered search is *not* itself defaults.

After redirect, condition (1) is false, so the next loader run takes the save branch. No infinite loop.

### Deep links

A URL like `/transactions?type=Expense` skips restoration (condition 1 fails) and updates memory to `type=Expense`. Explicit user intent always wins, and memory tracks the latest "real" view.

### Excluded routes

`/transactions/new`, `/transactions/$id`, `/transactions/$id/edit`, `/accounts/new`, `/accounts/$id`, `/accounts/$id/edit` and similar detail/edit routes inherit search params for the back-link forwarding fix but do not memorize. Only the seven list/report routes save and restore.

### Clear button

Any page that exposes a Clear action (`/transactions` does today; others will be audited during implementation) must call `clearFilters(pageId)` *before* navigating to defaults. Otherwise the loader sees defaults-in-URL and restores the freshly-cleared memory, undoing the user's action.

## Active-ledger tracking

The loader runs synchronously and needs the active ledger name without a network call. We mirror it into `localStorage`:

- **App boot.** The root layout already fetches `/api/ledgers` to render the ledger switcher. Add `setActiveLedger(data.active)` to the `onSuccess` of that query.
- **Ledger switch.** The existing switch handler that calls `switchLedger(name)` gains a `setActiveLedger(name)` line before invalidating queries.
- **First-load race.** If a user lands directly on `/transactions` before `/api/ledgers` resolves, `getActiveLedger()` returns `null` and the loader returns silently — no restore, no save on that visit. Next navigation works normally. The alternative (blocking the loader on an async fetch) introduces visible latency on every route entry; the silent no-op is the better tradeoff.

Switching ledger reads from a different `kea.filters.<ledger>.*` keyset. Old ledger's keys are kept, so switching back restores its memory.

## Testing

Three layers.

### Unit: `spa/src/test/lib/filter-memory.test.ts`

- `setActiveLedger` / `getActiveLedger` round-trip.
- `saveFilters` / `loadFilters` round-trip; missing key returns `null`.
- Per-ledger isolation: writing under ledger A doesn't surface under ledger B.
- JSON corruption: malformed value returns `null` and does not throw.
- Schema invalidation: stored payload that no longer validates against current schema returns `null`.
- `isDefaultSearch` for each `PageId`: returns `true` for the result of `parseXxxSearch({})`, `false` for any non-default field.
- `clearFilters` removes only the `(ledger, page)` key.

### Route loader behavior

Extend the existing route tests (`transactions.list.test.tsx`, `accounts.list.test.tsx`, `balances.test.tsx`, the four reports tests) with three new scenarios per route:

- **Defaults + no memory.** Loader runs, no redirect, page renders with defaults.
- **Defaults + memory.** Loader redirects; component sees the remembered search.
- **Non-defaults.** Loader saves; storage contains the URL search.

Tests use the jsdom `localStorage` shim (already available in the existing `setup.tsx`) and seed `kea.activeLedger` to a fixed ledger name per test.

### Regression coverage

Existing tests run against a clean `localStorage` so the "no memory" branch matches today's behavior. The 19 transactions tests, accounts tests, balances tests, and reports tests must keep passing without modification.

## Out of scope

- Per-page UI to view, edit, or wipe memory.
- An "I want a fresh view this time" affordance beyond the existing Clear button (which the user can use to escape a restored view; clicking it both clears memory and resets the URL).
- Migration tooling for pre-existing keys (there are none).
- Expiry, versioning, or cross-device sync.

## Risks

- **Pagination on shrunk data.** If the user remembered `offset=20` and the list now has 15 items, the page renders empty. Accepted — the user can pick a smaller offset via the pagination control.
- **Surprising restoration.** A user who walks away for a week may not remember the filter state they had. Mitigated by the existing Clear buttons and by leaving the filter chips visible in `FilterBar`.
- **Schema evolution.** Adding a new field to a search schema means old stored payloads might fail validation; the module treats this as "no memory" and the user re-applies. Existing keys are never migrated; they're transparently overwritten on the next save.
