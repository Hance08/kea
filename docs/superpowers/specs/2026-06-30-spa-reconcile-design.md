# SPA Reconcile Page

**Date:** 2026-06-30
**Status:** Approved design — ready for implementation plan
**Scope:** React SPA pages for the reconciliation workflow — account chooser, per-account workspace, mismatch confirmation. Backed entirely by the existing three reconcile endpoints. No service, store, or API changes beyond a one-field extension to the SPA's `ApiError` for the 409 mismatch body.

## Context

The web API reconcile endpoints landed via [`2026-06-04-web-api-reconcile-design.md`](2026-06-04-web-api-reconcile-design.md): `GET /api/accounts/{id}/unreconciled`, `POST /api/accounts/{id}/reconcile/preview`, `POST /api/accounts/{id}/reconcile`. The TUI implementation in `ui/reconcile/` provides the proven interaction model — load entries, type a statement balance, tick boxes, watch a live difference, confirm any mismatch.

The SPA currently has a placeholder `{ label: 'Reconcile' }` item in `spa/src/components/Sidebar.tsx` that renders as a disabled "Coming soon" link. This spec fills it in.

The user-facing flow this spec implements:

1. Click **Reconcile** in the sidebar → land on `/reconcile` (account chooser).
2. Pick an Asset or Liability leaf account → land on `/reconcile/$id` (workspace).
3. Type a statement balance, tick the transactions that appear on the statement, watch the difference update live.
4. Click **Reconcile**. If the difference is zero, commit immediately. If not, a dialog asks "short/over by $X — reconcile anyway?".
5. On success: stay on the page, the unreconciled list reloads (now shorter), the statement input clears — ready for the next statement.

An alternate entry point — a **Reconcile** button on the account detail page — skips the chooser for Asset/Liability leaf accounts and routes straight to `/reconcile/$id`.

## Decisions

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Route shape | `/reconcile` (chooser) + `/reconcile/$id` (workspace) | Mirrors the existing `/accounts` / `/accounts/$id`, `/transactions` / `/transactions/$id` pattern. File-based router segments line up naturally. |
| Entry points | Sidebar link **and** per-account button | Covers both mental models: "I want to reconcile" → sidebar; "I want to reconcile *this* account" → account-page button. |
| Account scope | Asset and Liability leaf accounts only | Practical reconciliation targets bank/credit-card-style accounts. Equity/Revenue/Expense don't have external statements. The API still accepts any leaf; the SPA narrows the surface. |
| Statement input placement | Inline at the top of the workspace, editable any time | Single-page flow; no "next" step. Difference updates live as the field and selection change. |
| Mismatch UX | Confirm dialog ("short/over by $X — reconcile anyway?") | Mirrors the TUI's y/n prompt. `Reconcile` button stays enabled regardless of diff; the dialog is the gate. Yes re-fires with `allow_mismatch: true`. |
| Post-commit | Stay on the page; refetch unreconciled; reset statement + selection; toast | Matches a multi-statement backlog workflow — the next statement is one balance entry away. |
| List controls | None beyond date-ascending default | Reconcile sessions are short and focused; the unreconciled list for one account is typically small. Sort/filter would distract from the core balance-matching task. |
| Session state | Local component state (`useState`) | Reconcile is a one-sitting task. URL params get unwieldy for ID lists; localStorage invites stale data. Losing state on reload is acceptable. |
| Preview vs commit | Skip the explicit preview call from the SPA | The server runs `PreviewReconcile` internally when `allow_mismatch: false`, and the SPA already computes the diff client-side for live display. The extra round-trip would only cross-check what the server is already authoritative on. |
| `ApiError` extension | Add optional `difference?: number` field | The 409 `balance_mismatch` body includes `difference`. The shared `apiFetch` currently strips it. One-field addition keeps the rest of the error pipeline unchanged. |

## Routes and Files

**Routes (TanStack Router file-based):**

- `spa/src/routes/reconcile.tsx` — layout wrapper containing `<Outlet/>`.
- `spa/src/routes/reconcile.index.tsx` — chooser.
- `spa/src/routes/reconcile.$id.tsx` — workspace.

**Sidebar:** in `spa/src/components/Sidebar.tsx`, replace the placeholder `{ label: 'Reconcile' }` with `{ label: 'Reconcile', to: '/reconcile', prefix: true }` so both `/reconcile` and `/reconcile/$id` keep it highlighted.

**Account detail entry:** add a **Reconcile** button to `spa/src/components/accounts/AccountDetailHeader.tsx`. Shown only when the account is a leaf (no children in the tree) and `account.type === 'A' || account.type === 'L'`. Routes to `/reconcile/$id`.

## API Client

**New file: `spa/src/lib/reconcile.ts`**

```ts
import { apiFetch } from './api';
import type { TransactionStatus } from './types';

export interface ReconcileEntry {
  id: number;
  timestamp: number;
  description: string;
  status: TransactionStatus;
  amount: number;          // int64 cents, signed
  offset_account: string;
}

export interface UnreconciledResponse {
  entries: ReconcileEntry[];
  last_reconciled_balance: number; // int64 cents
}

export interface ReconcilePreviewResponse {
  difference: number;
}

export interface ReconcileCommitResponse {
  reconciled_count: number;
  difference: number;
  last_reconciled_balance: number;
}

export function getUnreconciled(accountId: number): Promise<UnreconciledResponse>;
export function previewReconcile(
  accountId: number,
  statementBalance: number,
  transactionIds: number[],
): Promise<ReconcilePreviewResponse>;
export function commitReconcile(
  accountId: number,
  statementBalance: number,
  transactionIds: number[],
  allowMismatch: boolean,
): Promise<ReconcileCommitResponse>;
```

`previewReconcile` is exported but the workspace does not call it — kept available for future use (e.g. a "Check balance" button without committing). All amounts are `int64` cents as `number`, matching the rest of the SPA.

**`ApiError` extension in `spa/src/lib/api.ts`:**

```ts
interface ApiErrorBody {
  message?: string;
  field?: string;
  difference?: number; // new — populated on balance_mismatch
}

export class ApiError extends Error {
  readonly status: number;
  readonly field?: string;
  readonly difference?: number; // new
  constructor(status: number, message: string, field?: string, difference?: number) { /* ... */ }
}
```

Pass `body.difference` through in `apiFetch`. All other call sites are unaffected — the field is optional.

**Types:** add `ReconcileEntry`, `UnreconciledResponse`, `ReconcilePreviewResponse`, `ReconcileCommitResponse` to `spa/src/lib/types.ts` alongside the other domain types (`TransactionDetail`, `Account`, etc.). The `reconcile.ts` API client imports them from there.

## Components

**New directory: `spa/src/components/reconcile/`**

### `AccountChooser.tsx`

Used by the chooser route. Pure list of selectable accounts.

- Loads `getAccountTree({ include_hidden: false })` and `getBalances()` from React Query (both already cached for other pages).
- Walks the tree, keeping only leaves whose `type` is `A` or `L`.
- Renders each as a clickable row showing name, currency, current balance (looked up by ID from the balances response). Uses the existing `balanceColor` helper.
- Click → `navigate({ to: '/reconcile/$id', params: { id: account.id } })`.
- Empty result → "No asset or liability accounts to reconcile."
- Loading → skeleton rows. Error → existing alert + retry pattern.

### `ReconcileHeader.tsx`

The header strip from Layout A.

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Assets:Checking                            USD · 12 unreconciled       │
│                                                                         │
│  Statement     Last reconciled    Cleared       Difference              │
│  [_______]     $1,800.00          $640.00       Short $10.00            │
└─────────────────────────────────────────────────────────────────────────┘
```

Props:
- `accountName: string`, `currency: string`, `unreconciledCount: number`
- `statement: string` (raw input), `onStatementChange(v: string)`
- `lastReconciledCents: number`, `clearedCents: number`, `differenceCents: number | null`
- `statementInvalid: boolean`

Uses `Input` from `@/components/ui/input` with `inputMode="decimal"`. Difference label colors:
- diff > 0 (short) → red; copy: `Short $X`
- diff < 0 (over) → orange/amber; copy: `Over $X`
- diff == 0 → green; copy: `Balanced`
- statement empty or invalid → muted; copy: `—`

Amount formatting goes through the existing `formatAmount` helper, which honors the `hide_decimals` config.

### `UnreconciledTable.tsx`

Sticky `<thead>`, scrollable `<tbody>`.

Columns: checkbox · date (`MM-DD` of `entry.timestamp`) · offset account · description · amount (right-aligned, sign-colored).

Props: `entries: ReconcileEntry[]`, `selectedIds: Set<number>`, `onToggle(id: number)`.

- Date display: `MM-DD` for the year-to-date case is consistent with the TUI; promote to `YY-MM-DD` if any entry's year differs from the current year (simple per-render check; same calculation used in the TUI).
- Amount color: red for negative, green for positive — same sign-based convention as `TransactionRow.tsx`. The rendered amount uses `Math.abs(amount)`; the negative sign is implied by the red color.
- Each row has `aria-label="Toggle <date> <description> <amount>"` on the checkbox for screen readers.
- Empty state: a single row spanning all columns — "No unreconciled transactions for this account."
- Scrollable container: `max-h-[60vh] overflow-y-auto` (tune at implementation time); the `<thead>` uses `position: sticky; top: 0; background: ...` so it stays visible while the body scrolls.

### `ReconcileWorkspace.tsx`

The route's main component. Owns the per-session state:

```ts
const [statement, setStatement] = useState('');
const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
```

Derived:

```ts
// Mirrors the existing transaction/account form pattern:
// trimmed string → Number → cents via Math.round(n * 100). NaN/empty → null.
function parseStatementCents(s: string): number | null {
  const trimmed = s.trim();
  if (trimmed === '') return null;
  const n = Number(trimmed);
  return Number.isFinite(n) ? Math.round(n * 100) : null;
}

const statementCents = parseStatementCents(statement); // null on invalid/empty
const clearedCents = entries
  .filter((e) => selectedIds.has(e.id))
  .reduce((sum, e) => sum + e.amount, 0);
const differenceCents = statementCents !== null
  ? statementCents - (lastReconciledCents + clearedCents)
  : null;
```

The parser is defined locally in `ReconcileWorkspace.tsx`. The same `Math.round(n * 100)` pattern is duplicated in `TransactionForm.tsx`, `SplitsEditor.tsx`, and `AccountForm.tsx`; consolidating those (and this one) into a shared `lib/format.ts` helper is a separate cleanup, not part of this change.

Layout: `<ReconcileHeader />` then `<UnreconciledTable />` then a footer action bar with `Cancel` (calls `router.history.back()`) and `Reconcile` (calls the mutation).

`Reconcile` button is disabled when:
- `statement` is empty or parses invalid, OR
- `selectedIds` is empty, OR
- `commitMutation.isPending`

Commit mutation:

```ts
const commit = useMutation({
  mutationFn: (vars: { allowMismatch: boolean }) =>
    commitReconcile(accountId, statementCents!, [...selectedIds], vars.allowMismatch),
  onSuccess: (resp) => {
    qc.invalidateQueries({ queryKey: ['unreconciled', accountId] });
    qc.invalidateQueries({ queryKey: ['balances'] });
    qc.invalidateQueries({ queryKey: ['transactions'] });
    qc.invalidateQueries({ queryKey: ['account', accountId, 'balance'] });
    qc.invalidateQueries({ queryKey: ['balances', 'history'] });
    toast.success(`Reconciled ${resp.reconciled_count} transaction${resp.reconciled_count === 1 ? '' : 's'}`);
    setStatement('');
    setSelectedIds(new Set());
    setMismatchOpen(false);
  },
  onError: (e: unknown) => {
    if (e instanceof ApiError && e.status === 409 && e.difference !== undefined) {
      setPendingMismatch({ difference: e.difference });
      setMismatchOpen(true);
      return;
    }
    if (e instanceof ApiError && e.status === 400) {
      qc.invalidateQueries({ queryKey: ['unreconciled', accountId] });
    }
    toast.error(e instanceof Error ? e.message : 'Reconcile failed');
  },
});
```

The default Reconcile click fires `commit.mutate({ allowMismatch: false })`. The dialog's "Reconcile anyway" fires `commit.mutate({ allowMismatch: true })`.

### `MismatchDialog.tsx`

shadcn `Dialog`. Open state controlled by parent.

```
Statement balance is short by $10.00 — reconcile anyway?

[ Cancel ]  [ Reconcile anyway ]
```

Copy adapts: `short` when `difference > 0`, `over` when `difference < 0`. Cancel closes; Reconcile anyway fires the parent's `onConfirm` (the `commit.mutate({ allowMismatch: true })` call). On the success cascade the parent closes the dialog.

## Data Flow

```
[Sidebar Reconcile click]
        │
        ▼
/reconcile  ── AccountChooser ──[click]── /reconcile/$id ── ReconcileWorkspace
                                                                   │
                                                                   ├─ useQuery(['unreconciled', id])
                                                                   │     └─ getUnreconciled(id) → entries, last_reconciled_balance
                                                                   │
                                                                   ├─ local state: statement, selectedIds
                                                                   │     └─ derived: clearedCents, differenceCents
                                                                   │
                                                                   └─ commit.mutate({ allowMismatch })
                                                                         ├─ 200  → invalidate queries, reset state, toast
                                                                         ├─ 409  → open MismatchDialog
                                                                         └─ 400  → refetch unreconciled, toast
```

## Error Handling and Edge Cases

| Case | Behavior |
|---|---|
| URL `$id` non-numeric | TanStack route's `parseInt` returns `NaN`; workspace shows "Account not found" alert + Link to `/reconcile`. |
| `getAccount(id)` returns 404 | "Account not found" alert + Link to `/reconcile`. |
| `account.type` not `A` or `L` | Alert: "Reconcile only applies to Asset and Liability accounts." Link back. |
| Account is a parent (children in tree) | Alert: "This account has child accounts; reconcile a leaf instead." Link back. |
| Zero unreconciled entries | Table shows empty-state row. Header subtitle reads "All caught up". Reconcile button disabled (no IDs). |
| Statement empty | Reconcile disabled. Difference shows `—`. |
| Statement non-empty but invalid | `aria-invalid` on the input. Reconcile disabled. Difference shows `—`. |
| Selection empty | Reconcile disabled. Difference still updates (uses `cleared = 0`). |
| 409 mismatch | Dialog opens with computed-server `difference`. Yes re-fires with `allow_mismatch: true`. |
| 409 mismatch acknowledged, mutation still 409 | Shouldn't happen with the gate off; if it does (server-side 409 for a different reason that surfaces here), close dialog and toast the message. |
| 400 (e.g., IDs no longer unreconciled — concurrent commit elsewhere) | Toast error message + refetch `['unreconciled', accountId]` so the user sees the new state. |
| Cancel button | `router.history.back()`. No dirty-state confirmation; reconcile state is cheap to rebuild. |
| Sidebar active state | `prefix: true` on the nav item highlights for `/reconcile` and `/reconcile/$id`. |

## Out of Scope

- Unreconcile / undo a committed reconcile — service doesn't expose this.
- Reconciliation history view (past sessions, statement balances at the time).
- Statement metadata (statement date, PDF attachment).
- Cross-currency statement entry.
- Multi-tab concurrent reconcile coordination beyond the existing 400-from-server fallback.
- Statement-balance auto-suggest (e.g., from previous month).
- Keyboard shortcut parity with the TUI (`a` for select-all, etc.). The Select All control itself is out of scope per the "List controls: none" decision.
- Showing pending vs cleared status distinction in the table (the API returns both; the column is omitted to match the TUI's view).

## Testing

All new SPA tests use Vitest + React Testing Library + MSW, matching the existing `spa/src/test/*.test.tsx` pattern.

### API client — `spa/src/lib/reconcile.test.ts`

- `getUnreconciled` returns parsed `entries` and `last_reconciled_balance`.
- `previewReconcile` POSTs `{statement_balance, transaction_ids}` and returns `{difference}`.
- `commitReconcile` POSTs all three fields including `allow_mismatch`.
- `commitReconcile` 409 path → throws `ApiError` with `status === 409` and `difference` populated.

### `spa/src/lib/api.test.ts` (extension)

- Add a row for the 409 body shape: `ApiError.difference` carries the value through.
- Existing rows unchanged.

### Chooser — `spa/src/test/reconcile.chooser.test.tsx`

- Renders Asset and Liability leaves with name + balance.
- Excludes Equity, Revenue, Expense leaves.
- Excludes parent accounts.
- Click → navigates to `/reconcile/$id` (assert the resulting URL via test router).
- Loading skeleton + retryable error state.

### Workspace — `spa/src/test/reconcile.workspace.test.tsx`

- Loads list; header shows `lastReconciled` from API.
- Toggling a checkbox updates `cleared` and `difference` cells.
- Typing in statement updates `difference`; invalid input disables Reconcile and sets `aria-invalid`.
- Empty selection → Reconcile disabled even when statement is valid.
- Happy path: matched statement → click Reconcile → POST has `allow_mismatch:false` → toast, list refetches without committed IDs, statement and selection reset.
- 409 mismatch path: server returns 409 + difference → dialog renders "short $X" copy; "Reconcile anyway" re-fires with `allow_mismatch:true` → success cascade.
- 409 dialog Cancel → no second POST, state unchanged.
- 400 from server → error toast + refetch.
- Account type not A/L → alert with link back.
- Account is a parent → alert with link back.
- Account 404 → alert with link back.
- Empty unreconciled set → empty-row copy + Reconcile disabled.
- Sign-of-difference copy: over (`diff < 0`) renders "Over $X" with the amber/orange class; balanced (`diff === 0`) renders "Balanced" with the green class.

### Sidebar — `spa/src/test/sidebar.reconcile.test.tsx`

- `/reconcile` highlights the Reconcile nav item.
- `/reconcile/123` also highlights it (prefix match).
- The item is no longer rendered as disabled.

### Account detail header — extension

In the existing account-detail test file (or a new `spa/src/test/account.reconcile-button.test.tsx`):
- Asset leaf account → Reconcile button visible and links to `/reconcile/<id>`.
- Liability leaf account → button visible.
- Expense leaf account → button hidden.
- Parent Asset account → button hidden.

### Not retested

- Server-side reconcile logic (`internal/api/reconcile_test.go`, `internal/service/reconcile_ops_test.go`).
- `getBalances`, `getAccountTree`, `Sonner`, shadcn `Dialog`.
- `apiFetch` JSON parsing infrastructure beyond the new field.

### Manual verification (before claiming done)

Run the dev server + API, walk through:
1. Sidebar → chooser → pick account → workspace.
2. Account page → Reconcile button → workspace (same account).
3. Enter statement that balances the selection → commit → toast → list shorter, statement cleared.
4. Enter statement that doesn't balance → mismatch dialog → confirm → toast.
5. Confirm Account detail page shows updated balance.
6. Confirm an Expense leaf account does NOT show the Reconcile button.

## Files Touched

**Added:**
- `spa/src/routes/reconcile.tsx` — layout wrapper.
- `spa/src/routes/reconcile.index.tsx` — chooser route.
- `spa/src/routes/reconcile.$id.tsx` — workspace route.
- `spa/src/lib/reconcile.ts` — API client + types.
- `spa/src/lib/reconcile.test.ts` — API client tests.
- `spa/src/components/reconcile/AccountChooser.tsx`
- `spa/src/components/reconcile/ReconcileHeader.tsx`
- `spa/src/components/reconcile/UnreconciledTable.tsx`
- `spa/src/components/reconcile/ReconcileWorkspace.tsx`
- `spa/src/components/reconcile/MismatchDialog.tsx`
- `spa/src/test/reconcile.chooser.test.tsx`
- `spa/src/test/reconcile.workspace.test.tsx`
- `spa/src/test/sidebar.reconcile.test.tsx` (or extension of existing sidebar test).

**Modified:**
- `spa/src/lib/api.ts` — extend `ApiError` and `ApiErrorBody` with optional `difference`.
- `spa/src/lib/api.test.ts` — row for `difference` pass-through.
- `spa/src/lib/types.ts` — add `ReconcileEntry`, `UnreconciledResponse`, `ReconcilePreviewResponse`, `ReconcileCommitResponse` (or co-located in `reconcile.ts`).
- `spa/src/components/Sidebar.tsx` — turn the placeholder Reconcile item into a real link with `prefix: true`.
- `spa/src/components/accounts/AccountDetailHeader.tsx` — Reconcile button slot for Asset/Liability leaves.
- The route tree file (`spa/src/routeTree.gen.ts`) regenerates from the new route files; not hand-edited.

**Unchanged:**
- All Go code (`internal/api`, `internal/service`, `internal/store`, `internal/model`, `cmd/`).
- The TUI in `ui/reconcile/`.
- Existing SPA routes, components, and tests outside the additions above.
