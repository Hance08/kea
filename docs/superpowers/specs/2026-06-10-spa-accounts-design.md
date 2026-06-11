# SPA Accounts — Full CRUD Vertical Slice

**Date:** 2026-06-10
**Status:** Design approved — ready for implementation plan
**Predecessor:** [`2026-06-10-spa-transactions-design.md`](2026-06-10-spa-transactions-design.md) — most recent SPA slice; sets the precedent for routing, mutations, form library, testing
**API dependency:** All required endpoints (`GET/POST /api/accounts`, `GET /api/accounts/tree`, `GET/PATCH/DELETE /api/accounts/{id}`, `GET /api/accounts/{id}/balance`) are already merged.

## Goal

Replace the disabled **Accounts** sidebar item with a working route that supports list (hierarchical tree + search), view (adaptive detail page), create, edit, and delete. This is the third built-out route in the SPA after Balances and Transactions.

The slice mirrors the CLI/TUI surface (`kea account create`, `kea account list`, `kea account search`, `kea account edit`, `kea account delete`, `kea account info`). The service layer is unchanged — every operation maps onto an existing method on `svc.Account()` and an existing endpoint under `/api/accounts`.

It also closes a latent gap: Balances rows become clickable links into the new Accounts detail page, giving the detail view its highest-traffic entry point.

## Decisions

| Concern | Choice | Rationale |
|---|---|---|
| Scope | Full CRUD in one slice | Same call as Transactions; half an Accounts page would read worse than waiting |
| List shape | Tree by default; typing in the search box switches to flat ranked results | Chart of accounts is fundamentally hierarchical; search is the escape hatch when looking for a specific account |
| Detail page | Adaptive: header + recent-transactions (leaf) **or** child-accounts (parent) | Matches the "only leaves hold transactions" rule from the model — a parent has no transactions to show |
| Create form | Single page; opening-balance behind a toggle; parent picker drives type and currency | Mirrors model rules in the UI; reduces the invalid-state surface |
| Edit form | `name`, `description`, `is_hidden` editable; `type` and `parent_id` rendered as read-only display rows | Matches [`2026-04-22-account-edit-design.md`](2026-04-22-account-edit-design.md); avoids "why is this greyed out?" guessing |
| Delete | Disabled with tooltip when account has children or transactions; absent for system accounts | Pre-empts wasted clicks; backend is still authoritative |
| System accounts | `SystemAccountBanner` on detail and edit; Delete absent; Name field read-only | Mirrors the `ReconciledBanner` idiom shipped in the Transactions slice |
| Hidden accounts | Off by default; "Show hidden" toggle in the filter bar; muted styling + "hidden" tag | Respects `is_hidden` as a user-driven curation flag |
| Balances → Accounts linking | `AccountListRow` wrapped in `<Link to="/accounts/$id">` | One-line change; gives the detail page its highest-traffic entry point |
| Routing | Dedicated routes (`/accounts`, `/accounts/new`, `/accounts/$id`, `/accounts/$id/edit`) | Deep-linkable; URL is source of truth; matches Balances + Transactions precedent |
| Mutation strategy | Refetch via `invalidateQueries`; no optimistic updates | Matches Transactions; optimistic deferred as a separate follow-up |
| Form library | `react-hook-form` (already added in the Transactions slice) | No new dependency |
| Filter state location | URL search params via TanStack Router `validateSearch` (zod schema) | Already the SPA convention; back/forward and refresh just work |

### Out of scope — tracked as follow-ups

These are explicit cuts so v1 stays tractable. Each will get its own issue:

- Reassigning child accounts / merging accounts (would need a new backend operation)
- Bulk hide/unhide
- Drag-and-drop reparenting (backend doesn't support reparent — `parent_id` is immutable per the account-edit spec)
- "Move transactions to another account" before delete
- Account-level CSV / JSON export
- Optimistic mutation updates (will follow the same precedent as Transactions issue [#195](https://github.com/Hance08/kea/issues/195))

## Routes & File Layout

```
spa/src/routes/
├── accounts.tsx              → /accounts            (layout: search bar + filter toggles + <Outlet/>)
├── accounts.index.tsx        → /accounts            (tree or flat search results, default child)
├── accounts.new.tsx          → /accounts/new        (create form)
├── accounts.$id.tsx          → /accounts/$id        (detail, adaptive parent/leaf)
└── accounts.$id.edit.tsx     → /accounts/$id/edit   (edit form)
```

Sidebar gets the `Accounts` item enabled with `to: '/accounts'` (see [`spa/src/components/Sidebar.tsx`](../../../spa/src/components/Sidebar.tsx)).

Balances gets a one-line update: [`spa/src/components/AccountListRow.tsx`](../../../spa/src/components/AccountListRow.tsx) wraps its body in `<Link to="/accounts/$id" params={{ id: row.account_id }}>` so each row is clickable.

### Search params on `/accounts`

Validated via TanStack Router's `validateSearch`:

```ts
const accountsSearchSchema = z.object({
  q: z.string().optional(),                              // when present → flat search mode
  type: z.enum(['A', 'L', 'C', 'R', 'E']).optional(),    // matches model.AccountType
  include_hidden: z.coerce.boolean().default(false),
});
```

No pagination on the tree view (it's a full chart of accounts and small in practice). The flat search results page caps display at 100 rows for v1 and surfaces a "Refine search" hint past that — this is the only intentional UI limit, and it's a stop-gap rather than a load-bearing constraint.

## Component Split

```
spa/src/components/accounts/
├── AccountTree.tsx              # recursive tree, expand/collapse, balance per node
├── AccountTreeNode.tsx          # one node row
├── AccountSearchResults.tsx     # flat results when q is set
├── AccountSearchBox.tsx         # debounced 200ms; writes ?q= into URL
├── AccountFilters.tsx           # type + show-hidden toggles
├── AccountForm.tsx              # shared shell for create/edit
├── ParentAccountPicker.tsx      # combobox; constrains by chosen type once one is picked
├── TypeSelect.tsx               # A/L/C/R/E radio group; disabled when parent is picked
├── OpeningBalanceField.tsx      # checkbox-revealed amount input; currency from chosen currency
├── AccountDetailHeader.tsx      # name, type badge, currency, current balance, description, hidden tag
├── ChildAccountsCard.tsx        # for parent-type detail pages
├── RecentTransactionsCard.tsx   # for leaf-type detail pages; reuses TypeBadge, displayAmount
├── SystemAccountBanner.tsx      # mirror of ReconciledBanner
└── DeleteAccountButton.tsx      # encapsulates "disabled with tooltip" logic for each blocked state
```

Component-level rules (same as the Transactions slice):

- Each file stays under ~200 lines. `AccountForm` is the most complex; `ParentAccountPicker`, `TypeSelect`, and `OpeningBalanceField` are its sub-views.
- Presentational components take props only — no `useQuery` inside them. Data fetching lives in route components.
- `ParentAccountPicker` and `RecentTransactionsCard` reuse what we already shipped: the existing `AccountCombobox` from the Transactions slice powers the picker; `listTransactions({ account_id, limit: 20 })` powers the card.

## API Layer

```
spa/src/lib/
├── accounts.ts                  # CRUD wrappers over /api/accounts/*
├── accounts-search-params.ts    # zod schema + helpers for FilterBar ↔ URL
└── types.ts                     # extended with the types below
```

### `types.ts` additions

Mirror the Go structs in [`internal/model/account.go`](../../../internal/model/account.go):

```ts
export type AccountType = 'A' | 'L' | 'C' | 'R' | 'E';

export interface Account {
  id: number;
  name: string;
  type: AccountType;
  parent_id?: number;
  currency: string;
  description: string;
  is_hidden: boolean;
}

export interface AccountNode {
  account: Account;
  children: AccountNode[];
}

export interface CreateAccountInput {
  name: string;
  type: AccountType;
  parent_id?: number;
  currency: string;
  description?: string;
  balance?: number;            // optional opening balance, in cents (matches Go field name)
}

export interface UpdateAccountInput {
  name?: string;
  description?: string;
  is_hidden?: boolean;
}
```

`AccountBalance` already exists in `types.ts` from the Balances slice.

### `accounts.ts`

```ts
listAccounts(opts): Promise<ListResult<Account>>          // GET /api/accounts (uses SearchAccounts when q/filter present)
getAccountTree(opts): Promise<AccountNode[]>              // GET /api/accounts/tree
getAccount(id: number): Promise<Account>                  // GET /api/accounts/{id}
getAccountBalance(id: number): Promise<BalanceResponse>   // GET /api/accounts/{id}/balance
createAccount(input: CreateAccountInput): Promise<Account>
updateAccount(id: number, patch: UpdateAccountInput): Promise<Account>
deleteAccount(id: number): Promise<void>
```

All call through the existing `apiFetch` helper in [`spa/src/lib/api.ts`](../../../spa/src/lib/api.ts) so the error envelope and base URL stay consistent.

## Data Flow & State

### TanStack Query keys

```ts
['accounts', 'tree', { include_hidden }]
['accounts', 'search', { q, type, include_hidden }]
['account', id]
['account', id, 'balance']
['transactions', { account_id: id, limit: 20 }]   // for detail's recent-transactions card
```

### Mutation invalidations

```ts
queryClient.invalidateQueries({ queryKey: ['accounts'] });      // catches both tree and search keys
queryClient.invalidateQueries({ queryKey: ['account', id] });   // for edits
queryClient.invalidateQueries({ queryKey: ['balances'] });      // Balances dashboard reflects any change
```

Create also invalidates `['accounts']`. Delete additionally navigates back to `/accounts`.

### Filter state

Lives in URL search params, not React state. `AccountSearchBox` and `AccountFilters` read via TanStack Router's `useSearch()` and write via `navigate({ search: ... })`. Tree/search mode is driven entirely off `q`'s presence — there is no explicit toggle. Refresh, back/forward, and shared links all restore the right view.

### Form state

`react-hook-form` for both Create and Edit. Submit handler:

1. Build the input payload from form values.
2. Call the mutation.
3. On success: navigate to `/accounts/$id`.
4. On error: parse the API envelope and map `field` back to the form.

### Field error mapping

API error envelope (already standard):

```json
{ "error": "validation_failed", "message": "name must not contain ':'", "field": "name" }
```

| `field` value | UI surface |
|---|---|
| `name` | Name field helper text |
| `parent` / `parent_id` | Parent picker helper text |
| `type` | Type select helper text |
| `currency` | Currency field helper text |
| `description` | Description field helper text |
| `balance` | Opening-balance amount field helper text |
| absent | Top-of-form alert with `message` |

## Constraint & Lockout UI

A unified pattern for surfacing backend-enforced rules at UI level, so users don't waste clicks on operations that will fail. The backend remains authoritative — failed operations still surface as toasts as a fallback.

| State | UI |
|---|---|
| System account (`Equity:OpeningBalances_<CCY>`) | `SystemAccountBanner` on detail + edit; Delete absent; Name field read-only on edit |
| Has children | Delete button disabled; tooltip: *"Has N child accounts. Reassign or delete them first."* |
| Has transactions | Delete button disabled; tooltip: *"Has N transactions. Delete or reassign them first."* |
| Hidden | Detail header shows muted "hidden" tag; tree row rendered with muted styling |
| Immutable fields on edit | `type`, `parent_id`, `currency` shown as read-only display rows with a one-line helper: *"Type / parent / currency cannot be changed. Create a new account and move transactions instead."* |

`DeleteAccountButton` encapsulates the disabled-with-tooltip logic; the system-account check uses `model.IsOpeningBalancesAccount(name)` ported to TS (a single function in `lib/accounts.ts`).

Detecting "has children" / "has transactions" up front:

- **Has children** — already known from the tree query; the form route can read it from cache.
- **Has transactions** — the detail route's `RecentTransactionsCard` already issues `listTransactions({ account_id, limit: 20 })`; the `ListResult.total_count` reflects the full count for the filter, not just the page, so `total_count > 0` is the existence check. For the case where the user navigates straight to `/accounts/$id/edit` without the detail page priming the cache, the edit route issues a small sibling query (`listTransactions({ account_id, limit: 1 })`) purely to populate the check.

## List Page Layout

Filter bar (top, sticky):

```
[ Search accounts… ]   [ Type ▾ ]   [ ☐ Show hidden ]                            [ + New ]
```

**Tree mode** (default, when `q` is empty):

```
▾ Assets                                               $12,340.00
  ▾ Bank                                                $8,200.00
      Checking                                          $3,200.00
      Savings                                           $5,000.00
    Wallet                                                $140.00
  Investments                                           $4,000.00
▸ Liabilities                                          -$2,500.00
▸ Equity
▸ Revenue
▸ Expense
```

- Expand/collapse via the ▸/▾ chevron. Default: roots collapsed.
- Each node shows its name (leaf-only segment, not the full colon-path) and its balance. The balance is joined client-side: the tree query supplies the structure (`AccountNode[]`), the existing `['balances']` cache supplies the amounts (keyed by `account_id`). If `['balances']` is not yet populated, the row's balance column renders as a small skeleton until it loads.
- Hidden accounts hidden by default; rendered muted with a "hidden" tag when shown.
- Row is clickable → navigates to `/accounts/$id`.

**Flat search results mode** (when `q` is set):

| Name | Type | Currency | Balance |
|---|---|---|---:|
| Assets:Bank:Checking | Asset | USD | $3,200.00 |
| Assets:Bank:Savings | Asset | USD | $5,000.00 |

- Full colon-path name. Sorted by relevance (server-side via `SearchAccounts`).
- Same row-click → detail navigation.
- Capped at 100 rows for v1 by sending `?limit=100` to `SearchAccounts`; if the response's `total_count > 100`, a "Refine search — showing first 100 of N matches" hint renders above the table.

Empty state for tree: *"No accounts yet — click + New to create your first."*
Empty state for search: *"No accounts match `<q>`."* with a "Clear search" link.

Loading state: skeleton rows matching the active layout (tree skeleton or table skeleton). Mirrors the Balances / Transactions approach.

Error state: red alert with retry button.

## Detail Page Layout

Adaptive to leaf vs. parent:

### Leaf account

```
┌────────────────────────────────────────────────────────────┐
│ Assets:Bank:Checking            [ Edit ]   [ Delete ]      │
│ Asset · USD · $3,200.00                                    │
│ "Daily checking account"                                    │
├────────────────────────────────────────────────────────────┤
│ Recent transactions                                         │
│ 06-09  Expense   Coffee with team    -$12.50   Cleared     │
│ 06-08  Income    Paycheck         +$2,000.00   Cleared     │
│ ...                                                         │
│                          View all in Transactions →        │
└────────────────────────────────────────────────────────────┘
```

- "View all" link → `/transactions?account_id=$id` (re-uses the filter shipped in the Transactions slice).
- Recent transactions reuses `TypeBadge`, `StatusText`, and `displayAmount` from the Transactions slice — no new presentational components.

### Parent account

```
┌────────────────────────────────────────────────────────────┐
│ Assets:Bank                     [ Edit ]   [ Delete ]      │
│ Asset · USD · $8,200.00                                    │
│ "All bank accounts"                                         │
├────────────────────────────────────────────────────────────┤
│ Child accounts                                              │
│   Checking                                       $3,200.00 │
│   Savings                                        $5,000.00 │
│   Wallet                                           $140.00 │
└────────────────────────────────────────────────────────────┘
```

- No Recent transactions card (parents cannot hold transactions per the model rule).
- Child rows are clickable → child detail.
- Delete on a parent is disabled with the "has N child accounts" tooltip.

### System account

```
┌────────────────────────────────────────────────────────────┐
│ ⚠ This is a system account. It cannot be deleted, and its   │
│   name is managed automatically. Edit description or hide  │
│   it from views if needed.                                  │
├────────────────────────────────────────────────────────────┤
│ Equity:OpeningBalances_USD            [ Edit ]              │
│ Equity · USD · $0.00                                       │
│ ...                                                         │
└────────────────────────────────────────────────────────────┘
```

Same banner renders on `/accounts/$id/edit` with the Name field read-only.

## Create / Edit Page Layout

### Create (`/accounts/new`)

```
┌─────────────────────────────────────────────────┐
│ New account                                     │
├─────────────────────────────────────────────────┤
│ Parent account (optional)                       │
│ [ Search… ▾ ]                                   │
│                                                 │
│ Type                                            │
│ ( ) Asset ( ) Liability ( ) Equity              │
│ ( ) Revenue ( ) Expense                         │
│ (disabled if Parent is set — inherits)          │
│                                                 │
│ Name           Currency                         │
│ [______]       [USD ▾]   (inherits from parent) │
│                                                 │
│ Description                                     │
│ [_______________________________________]       │
│                                                 │
│ (Hidden is set after creation — use the edit    │
│  form to hide this account from default views.) │
│                                                 │
│ ☐ Set opening balance                           │
│   ┌── revealed when checked ──────────────────┐ │
│   │ Amount   [______]   posts against         │ │
│   │          Equity:OpeningBalances_<CCY>     │ │
│   └───────────────────────────────────────────┘ │
│                                                 │
│                       [Cancel]   [Create]       │
└─────────────────────────────────────────────────┘
```

Form-field rules:

- **Parent picker.** Reuses `AccountCombobox`. Selecting a parent disables Type and Currency (which then inherit). Clearing the parent re-enables both.
- **Type.** Five-button radio group (Asset / Liability / Equity / Revenue / Expense). Disabled when a parent is picked.
- **Name.** Single segment only — colons are rejected client-side with a helper message *"Name should not contain ':' — use the Parent picker to nest."*
- **Currency.** Inherits from parent when one is picked. Otherwise defaults to `defaults.currency` from `useServerConfig`. A select with the few common codes plus a free-type fallback (mirrors the Transactions slice's currency input).
- **Opening balance.** Hidden behind a "Set opening balance" checkbox. When checked, an amount input appears; the helper text shows which system account the offsetting split will post against (`Equity:OpeningBalances_<currency>`). Posting a real opening transaction later is always an option, so this is deliberately opt-in.

Submit posts the form values directly as the `CreateAccountInput` shape.

### Edit (`/accounts/$id/edit`)

```
┌─────────────────────────────────────────────────┐
│ Edit account                                    │
├─────────────────────────────────────────────────┤
│ Type:    Asset            (cannot change)       │
│ Parent:  Assets:Bank      (cannot change)       │
│ Currency: USD             (cannot change)       │
│                                                 │
│ Name                                            │
│ [Checking_________________]                     │
│                                                 │
│ Description                                     │
│ [_______________________________________]       │
│                                                 │
│ ☐ Hidden                                        │
│                                                 │
│                       [Cancel]   [Save]         │
└─────────────────────────────────────────────────┘
```

- Type / Parent / Currency are read-only display rows (not disabled inputs), per the system-account / immutable-field decision.
- Helper line under the read-only rows: *"Type, parent, and currency cannot be changed. Create a new account and move transactions instead."*
- For system accounts: Name field becomes read-only with the system banner on top; Description and Hidden remain editable.
- Submit issues a PATCH only with the fields that actually changed (idiomatic for `UpdateAccount`).

## Sidebar & Balances Updates

[`spa/src/components/Sidebar.tsx`](../../../spa/src/components/Sidebar.tsx) — change:

```ts
{ label: 'Accounts' },
```

to:

```ts
{ label: 'Accounts', to: '/accounts' },
```

[`spa/src/components/AccountListRow.tsx`](../../../spa/src/components/AccountListRow.tsx) — wrap the existing row body in:

```tsx
<Link to="/accounts/$id" params={{ id: row.account_id }} className="block hover:bg-muted">
  ...existing row markup...
</Link>
```

That's the entire footprint outside of the new Accounts components/routes/tests.

## New Dependencies

None. `react-hook-form` and `zod` were both added in the Transactions slice.

## Testing

Following the SPA's intentionally-thin convention (Vitest + `@testing-library/react`, jsdom, no Playwright/MSW):

**Pure logic — unit tests:**

- `lib/accounts-search-params.test.ts` — zod schema defaults, parse failures, round-trip.
- `lib/accounts.test.ts` — `IsOpeningBalancesAccount` mirror; account-list → tree flattening helpers if any custom logic lives client-side.

**Component tests:**

- `accounts.tree.test.tsx` — renders tree from a seeded `AccountNode[]`; expand/collapse works; "Show hidden" toggles correctly; rows are clickable.
- `accounts.search.test.tsx` — typing `q` switches to flat mode; results render with full colon-paths; debounce verified via fake timers.
- `accounts.form.test.tsx` — parent pick disables Type+Currency; opening-balance toggle reveals the field; invalid name surfaces field-mapped error; valid submit calls the mutation with expected payload.
- `accounts.detail.test.tsx` — leaf account shows `RecentTransactionsCard`; parent account shows `ChildAccountsCard`; system banner renders on `Equity:OpeningBalances_USD`.
- `accounts.delete.test.tsx` — Delete is disabled with tooltip when children present; Delete is disabled with tooltip when transactions present; Delete is absent for system accounts.
- `balances.link.test.tsx` — clicking a Balances row navigates to `/accounts/$id`.

**Mocking:** `vi.mock('@/lib/accounts')` to stub the API module; `vi.mock('@/lib/transactions')` for the `RecentTransactionsCard`.

**Not tested in unit/component:**

- Visual rendering of Tailwind classes beyond what discriminates a design choice (e.g., muted hidden rows).
- TanStack Query internals — covered upstream.
- Full happy-path E2E (tree → new → submit → detail → edit → delete) — manual verification step in the implementation plan.

## Manual Verification Checklist

To be expanded in the implementation plan; the spine is:

1. `kea serve &` then `cd spa && npm run dev`
2. Navigate to `/accounts` — tree renders, expand/collapse works, "Show hidden" toggles correctly.
3. Type into the search box — results switch to flat mode, debounce visible, "Clear search" link restores tree.
4. `+ New` → set Parent = `Assets:Bank`, Name = `Checking2`, Opening Balance = `$1000` → Create → land on `/accounts/$id`.
5. Detail page (leaf) — `RecentTransactionsCard` shows the synthesised opening transaction; "View all" link navigates to `/transactions?account_id=$id`.
6. Edit → change Description, toggle Hidden → Save → tree reflects (muted with "hidden" tag once "Show hidden" is on).
7. Detail page on `Assets:Bank` (parent) — `ChildAccountsCard` shows the new account; Delete on the parent is disabled with the "has N children" tooltip.
8. Delete `Checking2` → returns to `/accounts`; tree no longer shows it.
9. Navigate to `Equity:OpeningBalances_USD` — system banner present, Delete absent, Edit page shows Name field read-only.
10. Click a row on the Balances page → lands on `/accounts/$id` for that account.
11. Toggle the ledger via the sidebar switcher — `/accounts` refetches against the new ledger.
12. Refresh on `/accounts?q=bank&type=A&include_hidden=true` — flat search mode restored with filters intact.

## Files Touched

**Added:**

- `spa/src/routes/accounts.tsx`
- `spa/src/routes/accounts.index.tsx`
- `spa/src/routes/accounts.new.tsx`
- `spa/src/routes/accounts.$id.tsx`
- `spa/src/routes/accounts.$id.edit.tsx`
- `spa/src/components/accounts/AccountTree.tsx`
- `spa/src/components/accounts/AccountTreeNode.tsx`
- `spa/src/components/accounts/AccountSearchResults.tsx`
- `spa/src/components/accounts/AccountSearchBox.tsx`
- `spa/src/components/accounts/AccountFilters.tsx`
- `spa/src/components/accounts/AccountForm.tsx`
- `spa/src/components/accounts/ParentAccountPicker.tsx`
- `spa/src/components/accounts/TypeSelect.tsx`
- `spa/src/components/accounts/OpeningBalanceField.tsx`
- `spa/src/components/accounts/AccountDetailHeader.tsx`
- `spa/src/components/accounts/ChildAccountsCard.tsx`
- `spa/src/components/accounts/RecentTransactionsCard.tsx`
- `spa/src/components/accounts/SystemAccountBanner.tsx`
- `spa/src/components/accounts/DeleteAccountButton.tsx`
- `spa/src/lib/accounts.ts`
- `spa/src/lib/accounts-search-params.ts`
- `spa/src/test/accounts.tree.test.tsx`
- `spa/src/test/accounts.search.test.tsx`
- `spa/src/test/accounts.form.test.tsx`
- `spa/src/test/accounts.detail.test.tsx`
- `spa/src/test/accounts.delete.test.tsx`
- `spa/src/test/balances.link.test.tsx`
- `spa/src/test/lib/accounts-search-params.test.ts`
- `spa/src/test/lib/accounts.test.ts`

**Modified:**

- `spa/src/components/Sidebar.tsx` — enable Accounts nav item
- `spa/src/components/AccountListRow.tsx` — wrap row body in `<Link to="/accounts/$id">`
- `spa/src/lib/types.ts` — extend with `Account`, `AccountNode`, `CreateAccountInput`, `UpdateAccountInput`
- `spa/src/routeTree.gen.ts` — regenerated by TanStack Router on dev start

**Unchanged:**

- Everything outside `spa/`. No Go changes. No service, repo, API, or model edits.
