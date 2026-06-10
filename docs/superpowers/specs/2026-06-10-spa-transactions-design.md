# SPA Transactions — Full CRUD Vertical Slice

**Date:** 2026-06-10
**Status:** Design approved — ready for implementation plan
**Predecessor:** [`2026-06-05-spa-vertical-slice-design.md`](2026-06-05-spa-vertical-slice-design.md) — established SPA chrome, Balances route, tech stack
**API dependency:** [`2026-06-03-web-api-read-endpoints-design.md`](2026-06-03-web-api-read-endpoints-design.md) and the corresponding write endpoints — already merged

## Goal

Replace the disabled **Transactions** sidebar item with a working route that supports list (with all API-supported filters), view, create, edit, and delete. This is the second built-out route in the SPA after Balances, and the first one with write capabilities.

The slice mirrors the CLI/TUI surface: `kea add`, `kea tx ls`, `kea tx edit`, `kea tx rm`. The service layer is unchanged — every operation maps onto an existing method on `svc.Transaction()` and an existing endpoint under `/api/transactions`.

## Decisions

| Concern | Choice | Rationale |
|---|---|---|
| Scope | Full CRUD in one slice | User chose this knowing it's chunkier than prior slices |
| Filtering | All API-supported dimensions (`account_id`, `type`, `status`, `start_time`, `end_time`, `description`) | Daily reconciling needs all of them; no half-feature |
| Pagination | Page-based (`?limit=&offset=`) | Composes cleanly with URL filter state; deep-linkable; matches `ListResult` shape |
| Create form | Simple-by-default with "Advanced (edit splits)" toggle | Mirrors CLI ergonomics; common 2-split case stays fast |
| Edit form | Always opens in Advanced (splits-editor) mode | `UpdateTransactionComplete` takes splits; no value in pretending otherwise |
| Routing | Dedicated routes (`/transactions`, `/transactions/new`, `/transactions/$id`, `/transactions/$id/edit`) | Deep-linkable; URL is source of truth; matches Balances precedent |
| List layout | Dense single-line table | Maximises rows per screen; spreadsheet feel suits the reconcile-against-bank-statement use case |
| Status display | Plain text (no color, no emoji) | Per user call — keeps the status column low-noise |
| Type display | Colored badge, auto-derived from splits | Type is a function of the splits; user shouldn't have to set it |
| Reconciled handling | Edit and Delete blocked at UI level + banner | Backend is authoritative; UI signals the lockout |
| Currency in Simple mode | Inherits From account's currency | Matches `CreateSimpleTransaction` behavior |
| Currency in Advanced mode | Per-split picker | Cross-currency rare but supported |
| Type derivation in form | Client-side TS mirror of `DetermineType` | Live UX hint; server is still authoritative on submit |
| Mutation strategy | Refetch via `invalidateQueries`; no optimistic updates | Optimistic deferred to [#195](https://github.com/Hance08/kea/issues/195) |
| Form library | `react-hook-form` (new dep, ~25KB gzipped) | Required by the dynamic splits array + per-field validation wiring |
| Filter state location | URL search params (TanStack Router `validateSearch`) | Already the SPA convention; back/forward and refresh just work |

### Out of scope — tracked as separate issues

These are explicit follow-ups so v1 stays tractable:

- [#190](https://github.com/Hance08/kea/issues/190) — Bulk multi-select operations (delete / mark cleared)
- [#191](https://github.com/Hance08/kea/issues/191) — CSV / JSON import and export
- [#192](https://github.com/Hance08/kea/issues/192) — Inline "mark cleared / pending" action on list rows
- [#193](https://github.com/Hance08/kea/issues/193) — Saved filter presets
- [#194](https://github.com/Hance08/kea/issues/194) — Keyboard shortcuts
- [#195](https://github.com/Hance08/kea/issues/195) — Optimistic updates

Also out of scope: the reconcile workflow itself (already its own sidebar item, separate slice).

## Routes & File Layout

```
spa/src/routes/
├── transactions.tsx              → /transactions          (layout route + filter bar + <Outlet/>)
├── transactions.index.tsx        → /transactions          (list, default child of the layout route)
├── transactions.new.tsx          → /transactions/new      (create form)
├── transactions.$id.tsx          → /transactions/$id      (read-only detail view)
└── transactions.$id.edit.tsx     → /transactions/$id/edit (edit form, splits-editor mode)
```

Sidebar gets the `Transactions` item enabled with `to: '/transactions'` (see `spa/src/components/Sidebar.tsx`).

### Search params on `/transactions`

Validated via TanStack Router's `validateSearch`:

```ts
const transactionsSearchSchema = z.object({
  account_id: z.coerce.number().int().positive().optional(),
  type: z.enum(['Expense','Income','Transfer','Opening','Deposit','Withdrawal','Other']).optional(),
  status: z.enum(['Pending','Cleared','Reconciled']).optional(),
  start_time: z.coerce.number().int().optional(),  // Unix seconds
  end_time: z.coerce.number().int().optional(),
  description: z.string().optional(),
  limit: z.coerce.number().int().positive().default(50),
  offset: z.coerce.number().int().nonnegative().default(0),
});
```

Adding `zod` if not already present is a one-line concern.

## Component Split

```
spa/src/components/transactions/
├── TransactionsTable.tsx        # dense list, presentational
├── TransactionRow.tsx           # one row
├── FilterBar.tsx                # reads/writes URL search params
├── Pagination.tsx               # prev/next + "Page N of M" indicator
├── TransactionForm.tsx          # shell with Simple/Advanced toggle
├── SimpleFields.tsx             # from/to/amount sub-fields
├── SplitsEditor.tsx             # editable splits table for Advanced mode
├── AccountCombobox.tsx          # account picker; debounced /api/accounts?q= search
├── TypeBadge.tsx                # colored badge per transaction type
├── StatusText.tsx               # plain-text status (no color, no emoji)
└── ReconciledBanner.tsx         # lockout banner on detail/edit pages
```

Component-level rules:
- Each file stays under ~200 lines. `TransactionForm` is the most complex; `SimpleFields` and `SplitsEditor` are its sub-views.
- Presentational components take props only — no `useQuery` inside them. Data fetching lives in route components.
- `AccountCombobox` is reused: Simple form's From/To pickers, Advanced form's per-split picker, and `FilterBar`'s account filter.

## API Layer

```
spa/src/lib/
├── transactions.ts              # CRUD functions over /api/transactions/*
├── accounts-search.ts           # searchAccounts(query) wrapper
├── determineType.ts             # client-side type derivation, mirrors Go logic
├── transactionDisplay.ts        # displayAccount, displayOffsetAccount, displayAmount
├── types.ts                     # extended with the types below
└── transactions-search-params.ts # zod schema + helpers for FilterBar ↔ URL ↔ API
```

### `types.ts` additions

Mirror the Go structs (existing `types.ts` already mirrors `AccountBalance`):

```ts
export type TransactionType =
  | 'Expense' | 'Income' | 'Transfer'
  | 'Opening' | 'Deposit' | 'Withdrawal' | 'Other';

export type TransactionStatus = 'Pending' | 'Cleared' | 'Reconciled';

export interface SplitDetail {
  id: number;
  account_id: number;
  account_name: string;
  account_type: AccountType;
  amount: number;       // int64 cents, signed
  currency: string;
  memo: string;
}

export interface TransactionDetail {
  id: number;
  timestamp: number;    // Unix seconds
  description: string;
  status: TransactionStatus;
  type: TransactionType;
  splits: SplitDetail[];
}

export interface CreateTransactionFromSplitsInput {
  splits: { account_name: string; amount: number; currency: string; memo?: string }[];
  description: string;
  timestamp: number;
  status: TransactionStatus;
  type?: TransactionType;  // omitted → server derives
}

export interface UpdateTransactionInput {
  id: number;
  description: string;
  timestamp: number;
  status: TransactionStatus;
  type?: TransactionType;
  splits: { id?: number; account_name: string; amount: number; currency: string; memo?: string }[];
}

export interface TransactionFilter {
  account_id?: number;
  type?: TransactionType;
  status?: TransactionStatus;
  start_time?: number;
  end_time?: number;
  description?: string;
}
```

### `transactions.ts`

```ts
listTransactions(filter, opts): Promise<ListResult<TransactionDetail>>
getTransaction(id: number): Promise<TransactionDetail>
createTransaction(input: CreateTransactionFromSplitsInput): Promise<TransactionDetail>
updateTransaction(input: UpdateTransactionInput): Promise<TransactionDetail>
updateTransactionStatus(id: number, status: TransactionStatus): Promise<TransactionDetail>
deleteTransaction(id: number): Promise<void>
```

All call through the existing `apiFetch` helper used by `lib/api.ts` so the error envelope and base URL stay consistent.

### `determineType.ts`

Port of `internal/service/transaction_classifier.go::DetermineType`. Takes `SplitDetail[]` plus an account-type lookup function (so the form can resolve types from the cached accounts list). Returns a `TransactionType`. Comes with the same test cases as the Go version (`internal/service/transaction_classifier_test.go`) to lock the mirror.

Acceptable drift: when the client cache doesn't know an account's type (newly created account not yet in cache), the mirror returns `'Other'` and the badge re-renders once the cache populates. The server is still authoritative on submit.

## Data Flow & State

### TanStack Query keys

```ts
['transactions', filtersObject]            // list
['transaction', id]                        // single detail
['accounts', 'search', query]              // combobox, debounced 200ms
['accounts', 'list']                       // full list for Simple mode pickers; cached aggressively
```

Mutations call:
```ts
queryClient.invalidateQueries({ queryKey: ['transactions'] });
queryClient.invalidateQueries({ queryKey: ['transaction', id] });  // edit/status/delete
```
Also invalidate `['balances']` after any mutation — the Balances dashboard reflects the change.

### Filter state

Lives in URL search params, not React state. `FilterBar` reads via TanStack Router's `useSearch()` and writes via `navigate({ search: ... })`. The list page's `useQuery` keys directly off the validated search object. Back/forward, refresh, and shared links all refetch the correct page.

### Form state

`react-hook-form` for both Simple and Advanced modes. Splits in Advanced mode use `useFieldArray`. Submit handler:

1. Build the input payload from form values (translating Simple → 2-split synthesis if needed).
2. Call the mutation.
3. On success: navigate to `/transactions/$id`.
4. On error: parse the API envelope and map `field` back to the form.

### Field error mapping

API error envelope:
```json
{ "error": "validation_failed", "message": "splits must sum to zero", "field": "splits" }
```

Mapping rules:
| `field` value | UI surface |
|---|---|
| `splits` | banner above the splits table (Advanced) or top-of-form alert (Simple) |
| `from_account` | Simple mode's "From account" field helper text |
| `to_account` | Simple mode's "To account" field helper text |
| `amount` | "Amount" field helper text |
| `description` | "Description" field helper text |
| `timestamp` | "Date" field helper text |
| `type` | top-of-form alert (rare — only when user overrode) |
| absent | top-of-form alert with `message` |

## Reconciled Lockout

A transaction whose `status === 'Reconciled'` is rendered with these UI rules:

- **List page** — the row's status column shows `"Reconciled"` (plain text, per the styling decision). No special icon. Row is still clickable.
- **Detail page** — Edit and Delete buttons are absent. `ReconciledBanner` renders above the splits with copy: *"This transaction is reconciled and cannot be edited or deleted. Unreconcile from the Reconcile screen first."*
- **Edit route** — if navigated to directly (`/transactions/$id/edit` on a reconciled tx), the page renders the banner and a "Back to detail" link. Form is not mounted.

Backend is the source of truth — `ErrReconciled` is mapped to HTTP 409 in the API. The UI checks are a courtesy to avoid wasted requests.

## Type Derivation in the Form

As the user fills the form, the badge updates live via the TS mirror:

- **Simple mode** — once both From and To accounts are picked, look up each account's type from the `['accounts', 'list']` cache, synthesize the two-split structure (From negative, To positive, amount from the Amount field), and feed it to `determineType()`.
- **Advanced mode** — whenever the splits array changes, re-derive.

The badge renders next to "Type" in the form. If derivation returns `'Other'` because the cache hasn't loaded yet, the badge shows `'…'` until cached.

Override: there's no in-form override in v1. If a user genuinely needs to force a type, they can do so via the CLI (`--type` flag). This is a deliberate scope cut — the derived type is correct for ~all real transactions.

## List Page Layout

Filter bar (top, sticky):
```
[ Account ▾ ]  [ Type ▾ ]  [ Status ▾ ]  [ From date ] [ To date ]  [ Search description ]   [ Clear filters ]   [ + New ]
```

Dense table below (per the approved mockup):

| Date | Type | Description | Account → Offset | Amount | Status |
|---|---|---|---|---:|---|
| 06-09 | `Expense` | Coffee with team | Bank:Checking → Coffee | -12.50 | Cleared |

- **Date** — `YYYY-MM-DD`, no time component (matches `model.DateFormat`).
- **Type** — colored badge via `TypeBadge` (Expense=red, Income=green, Transfer=blue, others muted).
- **Description** — truncated with ellipsis at the column width; hover shows full text via `title`.
- **Account → Offset** — primary account + arrow + offset account (or "(multiple)" when >2 splits beyond primary). Computed client-side via `transactionDisplay.ts::displayAccount` + `displayOffsetAccount`, ports of `service.GetDisplayAccount` / `GetDisplayOffsetAccount`.
- **Amount** — right-aligned, signed, red for negative / green for positive. Sign is computed by `transactionDisplay.ts::displayAmount(type, splits)` — expense shows negative (money out), income shows positive (money in), transfer shows absolute, others fall back to the existing `GetDisplayAmount` rule (max positive amount). This is a SPA convention; the CLI's `tx ls` shows positive-only.
- **Status** — plain text. No color, no emoji.

Row is clickable → navigates to `/transactions/$id`.

Pagination footer: `Page 1 of 12  ‹ Prev   Next ›`. Page numbers derived from `total_count / limit`.

Empty state: `"No transactions match these filters."` with a "Clear filters" button if any filters are active, or a "+ New transaction" button otherwise.

Loading state: skeleton rows matching the table layout (same approach as Balances).

Error state: red alert with retry button, mirroring Balances.

## Create / Edit Page Layout

Per the approved mockup:

**Simple mode** (default for `/transactions/new`):
```
┌────────────────────────────────────────────────┐
│ New transaction         ☐ Advanced (edit splits) │
├────────────────────────────────────────────────┤
│  Date          Status                          │
│  [____]        [Cleared ▾]                     │
│  Description                                   │
│  [____________________________________________]│
│  From account            To account            │
│  [Assets:Bank ▾]         [Expenses:Coffee ▾]   │
│  Amount                  Type (auto)           │
│  [____]                  [Expense]             │
│                                                │
│                    [Cancel]   [Create]         │
└────────────────────────────────────────────────┘
```

**Advanced mode** (toggle on; mandatory for `/transactions/$id/edit`):
- Replaces From/To/Amount with the splits editor.
- Splits table: Account | Amount | Memo | × (remove). "+ Add split" button below.
- Live balance indicator: `"Balance: 0.00 ✓"` (green) or `"Balance: 12.50 ✗"` (red).
- Submit disabled while balance ≠ 0.

Edit page (`/transactions/$id/edit`) opens with the toggle pre-checked and disabled (Advanced-only). All fields pre-populated from `GET /api/transactions/{id}`.

## Sidebar Update

`spa/src/components/Sidebar.tsx` — change:
```ts
{ label: 'Transactions' },
```
to:
```ts
{ label: 'Transactions', to: '/transactions' },
```

That's the entire Sidebar diff.

## New Dependencies

- `react-hook-form` (~25KB gzipped) — splits dynamic array + per-field validation. Required.
- `zod` — only if not already in the SPA; used for `validateSearch`. Tiny.

Both go through `npm install` in `spa/`. No transitive surprises.

## Testing

Following the SPA's intentionally-thin convention (`spa/src/test/setup.ts`): Vitest + @testing-library/react, jsdom, no Playwright/MSW.

**Pure logic — unit tests:**
- `lib/determineType.test.ts` — port of cases from `internal/service/transaction_classifier_test.go`. Same inputs and expected outputs so divergence is caught.
- `lib/transactionDisplay.test.ts` — mirrors `GetDisplayAccount` / `GetDisplayOffsetAccount` / `GetDisplayAmount` test cases from `internal/service/transaction_classifier_test.go`, plus SPA-specific sign cases for `displayAmount`.
- `lib/transactions-search-params.test.ts` — `validateSearch` zod schema: defaults, parse failures, round-trip.

**Component tests:**
- `transactions.list.test.tsx` — renders three seeded transactions; asserts dense table layout; asserts StatusText is plain text (no `role="img"`, no color classes); asserts pagination renders; filter changes push search params.
- `transactions.form.test.tsx` — Simple mode by default; toggling Advanced reveals the splits editor; balance indicator updates as amounts change; submitting invalid splits surfaces banner-mapped error; valid submit triggers the mutation with the expected payload.
- `transactions.reconciled.test.tsx` — given a reconciled detail response, asserts Edit and Delete buttons absent and ReconciledBanner renders.

**Mocking:** `vi.mock('@/lib/transactions')` to stub the API module — same approach as existing Balances tests.

**Not tested in unit/component:**
- Visual rendering of Tailwind classes beyond what discriminates the design choice (e.g., plain-text status).
- TanStack Query internals — covered by upstream.
- Full happy-path E2E (list → new → submit → detail) — manual verification step in the implementation plan.

## Manual Verification Checklist

To be expanded in the implementation plan, but the spine is:

1. `kea serve &` then `cd spa && npm run dev`
2. Navigate to `/transactions` — list loads, filters work, pagination works
3. `+ New` → Simple mode → fill From=Checking, To=Coffee, Amount=12.50, Description=Test → Create → land on detail page
4. Edit the new transaction → splits editor pre-populated → change amount → save → detail reflects
5. Delete the new transaction → list updates
6. Navigate to a reconciled transaction's detail → banner shows, Edit/Delete absent
7. Toggle ledger via the sidebar switcher — list refetches against the new ledger
8. Refresh on `/transactions?type=Expense&status=Cleared&offset=50` → filters and page restored

## Files Touched

**Added:**
- `spa/src/routes/transactions.tsx`
- `spa/src/routes/transactions.index.tsx`
- `spa/src/routes/transactions.new.tsx`
- `spa/src/routes/transactions.$id.tsx`
- `spa/src/routes/transactions.$id.edit.tsx`
- `spa/src/components/transactions/TransactionsTable.tsx`
- `spa/src/components/transactions/TransactionRow.tsx`
- `spa/src/components/transactions/FilterBar.tsx`
- `spa/src/components/transactions/Pagination.tsx`
- `spa/src/components/transactions/TransactionForm.tsx`
- `spa/src/components/transactions/SimpleFields.tsx`
- `spa/src/components/transactions/SplitsEditor.tsx`
- `spa/src/components/transactions/AccountCombobox.tsx`
- `spa/src/components/transactions/TypeBadge.tsx`
- `spa/src/components/transactions/StatusText.tsx`
- `spa/src/components/transactions/ReconciledBanner.tsx`
- `spa/src/lib/transactions.ts`
- `spa/src/lib/accounts-search.ts`
- `spa/src/lib/determineType.ts`
- `spa/src/lib/transactionDisplay.ts`
- `spa/src/lib/transactions-search-params.ts`
- `spa/src/test/transactions.list.test.tsx`
- `spa/src/test/transactions.form.test.tsx`
- `spa/src/test/transactions.reconciled.test.tsx`
- `spa/src/test/lib/determineType.test.ts`
- `spa/src/test/lib/transactionDisplay.test.ts`
- `spa/src/test/lib/transactions-search-params.test.ts`

**Modified:**
- `spa/src/components/Sidebar.tsx` — enable Transactions nav item
- `spa/src/lib/types.ts` — extend with transaction-domain types
- `spa/package.json` — add `react-hook-form`, `zod` (if absent)
- `spa/src/routeTree.gen.ts` — regenerated by TanStack Router on dev start

**Unchanged:**
- Everything outside `spa/`. No Go changes. No service, repo, API, or model edits.
