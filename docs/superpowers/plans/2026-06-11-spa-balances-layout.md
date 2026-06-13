# SPA Balances Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single mixed list on the SPA `/balances` route with two side-by-side columns (Assets left, Liabilities right). Each column has its own sortable Balance header, its own pagination at 8 rows per page, and its own type total folded into the column header bar.

**Architecture:** The existing `summarizeBalances(rows, currency)` helper already partitions input into included A/L rows plus totals — we keep using it. New URL search params (`a_offset`, `a_sort`, `l_offset`, `l_sort`) drive per-side sort and pagination through TanStack Router's `validateSearch`. The route splits `summary.included` by `type`, sorts each side with the existing `naturalAmount` comparator (already used by `/accounts`), slices by offset, and renders two `<BalanceColumn>` cards in a `grid-cols-2 items-start` container.

**Tech Stack:** React 18, TanStack Router, TanStack Query, zod, Tailwind, shadcn-style UI (Card, Button), Vitest + React Testing Library, Biome.

**Working directory:** `/Users/hance/programming/kea` (branch `feat/spa-balances-layout` already created). All commands below assume the repo root unless otherwise noted; commands prefixed with `spa/` should be run from `spa/`.

**Spec:** `docs/superpowers/specs/2026-06-11-spa-balances-layout-design.md`

---

## File Structure

**New files:**

| Path | Responsibility |
|---|---|
| `spa/src/lib/balances-search-params.ts` | zod schema + `parseBalancesSearch` for the four URL params |
| `spa/src/components/balances/BalanceColumn.tsx` | One self-contained column: header bar (label + total), sortable `Account/Balance` subheader, rows, pagination, empty state |
| `spa/src/components/balances/BalanceColumnRow.tsx` | One row: link to `/accounts/$id`, name on left, balance on right |
| `spa/src/test/lib/balances-search-params.test.ts` | Schema unit tests |
| `spa/src/test/components/BalanceColumn.test.tsx` | Component unit tests |

**Modified files:**

| Path | Change |
|---|---|
| `spa/src/routes/balances.tsx` | Add `validateSearch`, partition+sort+slice per side, render two `<BalanceColumn>` in a grid; drop NetWorth-card-row of `TypeTotalCard`s |
| `spa/src/test/balances.test.tsx` | Expand to cover the new layout (partition, default sort, independent sort + pagination, natural-direction order) |

**Removed files:**

| Path | Reason |
|---|---|
| `spa/src/components/AccountListRow.tsx` | Only caller was `balances.tsx`; replaced by `BalanceColumnRow` |
| `spa/src/components/TypeTotalCard.tsx` | Only caller was `balances.tsx`; totals now live inside `BalanceColumn`'s header bar |

---

## Task 1: URL search params schema

**Files:**
- Create: `spa/src/lib/balances-search-params.ts`
- Test: `spa/src/test/lib/balances-search-params.test.ts`

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/lib/balances-search-params.test.ts`:

```ts
import { parseBalancesSearch } from '@/lib/balances-search-params';
import { describe, expect, it } from 'vitest';

describe('parseBalancesSearch', () => {
  it('returns defaults when nothing is set', () => {
    expect(parseBalancesSearch({})).toEqual({
      a_offset: 0,
      a_sort: 'balance_desc',
      l_offset: 0,
      l_sort: 'balance_desc',
    });
  });

  it('coerces offsets from query strings', () => {
    expect(parseBalancesSearch({ a_offset: '16', l_offset: '8' })).toEqual({
      a_offset: 16,
      a_sort: 'balance_desc',
      l_offset: 8,
      l_sort: 'balance_desc',
    });
  });

  it('accepts both sort values per side', () => {
    expect(parseBalancesSearch({ a_sort: 'balance_asc', l_sort: 'balance_desc' })).toEqual({
      a_offset: 0,
      a_sort: 'balance_asc',
      l_offset: 0,
      l_sort: 'balance_desc',
    });
  });

  it('rejects negative offsets', () => {
    expect(() => parseBalancesSearch({ a_offset: '-1' })).toThrow();
  });

  it('rejects unknown sort values', () => {
    expect(() => parseBalancesSearch({ a_sort: 'name_asc' })).toThrow();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npm test -- src/test/lib/balances-search-params.test.ts`
Expected: FAIL — module `@/lib/balances-search-params` not found.

- [ ] **Step 3: Implement the schema**

Create `spa/src/lib/balances-search-params.ts`:

```ts
import { z } from 'zod';

const sortSchema = z.enum(['balance_desc', 'balance_asc']);

export const balancesSearchSchema = z.object({
  a_offset: z.coerce.number().int().nonnegative().default(0),
  a_sort: sortSchema.default('balance_desc'),
  l_offset: z.coerce.number().int().nonnegative().default(0),
  l_sort: sortSchema.default('balance_desc'),
});

export type BalancesSearch = z.infer<typeof balancesSearchSchema>;

export function parseBalancesSearch(s: Record<string, unknown>): BalancesSearch {
  return balancesSearchSchema.parse(s);
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npm test -- src/test/lib/balances-search-params.test.ts`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/balances-search-params.ts spa/src/test/lib/balances-search-params.test.ts
git commit -m "feat(spa): zod schema for balances URL search params"
```

---

## Task 2: BalanceColumnRow component

**Files:**
- Create: `spa/src/components/balances/BalanceColumnRow.tsx`

This component is small and stateless; the surrounding `BalanceColumn` test covers it (Task 3). No standalone test file.

- [ ] **Step 1: Create the component**

Create `spa/src/components/balances/BalanceColumnRow.tsx`:

```tsx
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  row: AccountBalance;
}

export function BalanceColumnRow({ row }: Props) {
  const negative = row.amount < 0;
  return (
    <Link
      to="/accounts/$id"
      params={{ id: String(row.account_id) }}
      search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
      className="flex items-center justify-between border-t border-border/60 px-3 py-2 text-sm hover:bg-muted/40"
    >
      <span className="truncate">{row.name}</span>
      <span className={cn('tabular-nums', negative && 'text-destructive')}>
        {formatCents(row.amount, row.currency)}
      </span>
    </Link>
  );
}
```

- [ ] **Step 2: Verify it type-checks**

Run: `cd spa && npx tsc -b`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/balances/BalanceColumnRow.tsx
git commit -m "feat(spa): BalanceColumnRow link component for balances columns"
```

---

## Task 3: BalanceColumn component

**Files:**
- Create: `spa/src/components/balances/BalanceColumn.tsx`
- Test: `spa/src/test/components/BalanceColumn.test.tsx`

`BalanceColumn` owns the column's header bar (label + total), the sortable `Account / Balance` subheader (hidden when empty), the rows, the per-side `Pagination`, and the empty-state message. It is purely presentational — sort/offset live in the parent.

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/components/BalanceColumn.test.tsx`:

```tsx
import { BalanceColumn } from '@/components/balances/BalanceColumn';
import type { AccountBalance } from '@/lib/types';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  RootRoute,
  Route,
  Router,
  RouterProvider,
  createMemoryHistory,
} from '@tanstack/react-router';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

// BalanceColumn renders <Link>, so it needs a router context. Build a
// minimal one with a single dummy route so the link has somewhere to go.
function renderWithRouter(ui: React.ReactNode) {
  const rootRoute = new RootRoute({ component: () => <>{ui}</> });
  const dummy = new Route({
    getParentRoute: () => rootRoute,
    path: '/',
    component: () => null,
  });
  const router = new Router({
    routeTree: rootRoute.addChildren([dummy]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(
    <QueryClientProvider client={qc}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

function makeRows(count: number, type: 'A' | 'L'): AccountBalance[] {
  return Array.from({ length: count }, (_, i) => ({
    account_id: i + 1,
    name: `${type === 'A' ? 'Assets' : 'Liab'}:Acct${i + 1}`,
    type,
    currency: 'USD',
    amount: type === 'A' ? (i + 1) * 1000 : -(i + 1) * 1000,
    is_hidden: false,
  }));
}

describe('BalanceColumn', () => {
  const baseProps = {
    label: 'Assets' as const,
    total: 24310_00,
    currency: 'USD',
    sortDir: 'desc' as const,
    onToggleSort: vi.fn(),
    offset: 0,
    onOffsetChange: vi.fn(),
    emptyText: 'No assets',
  };

  it('renders the type label and total in the header bar', () => {
    renderWithRouter(<BalanceColumn {...baseProps} rows={makeRows(3, 'A')} totalRowCount={3} />);
    expect(screen.getByText('Assets')).toBeInTheDocument();
    expect(screen.getByText('$24,310.00')).toBeInTheDocument();
  });

  it('renders the sort arrow matching sortDir', () => {
    const { rerender } = renderWithRouter(
      <BalanceColumn {...baseProps} rows={makeRows(1, 'A')} totalRowCount={1} sortDir="desc" />,
    );
    expect(screen.getByRole('button', { name: /Balance/i })).toHaveTextContent('▼');
    rerender(
      <BalanceColumn {...baseProps} rows={makeRows(1, 'A')} totalRowCount={1} sortDir="asc" />,
    );
    expect(screen.getByRole('button', { name: /Balance/i })).toHaveTextContent('▲');
  });

  it('calls onToggleSort when the Balance header is clicked', async () => {
    const onToggleSort = vi.fn();
    renderWithRouter(
      <BalanceColumn
        {...baseProps}
        rows={makeRows(1, 'A')}
        totalRowCount={1}
        onToggleSort={onToggleSort}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /Balance/i }));
    expect(onToggleSort).toHaveBeenCalledTimes(1);
  });

  it('hides pagination when totalRowCount is 8 or fewer', () => {
    renderWithRouter(<BalanceColumn {...baseProps} rows={makeRows(8, 'A')} totalRowCount={8} />);
    expect(screen.queryByText(/Page \d+ of/)).not.toBeInTheDocument();
  });

  it('shows pagination when totalRowCount exceeds 8', () => {
    renderWithRouter(<BalanceColumn {...baseProps} rows={makeRows(8, 'A')} totalRowCount={15} />);
    expect(screen.getByText(/Page 1 of 2/)).toBeInTheDocument();
  });

  it('shows the empty text and hides sort/pagination when rows is empty', () => {
    renderWithRouter(<BalanceColumn {...baseProps} rows={[]} totalRowCount={0} />);
    expect(screen.getByText('No assets')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Balance/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/Page \d+ of/)).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npm test -- src/test/components/BalanceColumn.test.tsx`
Expected: FAIL — module `@/components/balances/BalanceColumn` not found.

- [ ] **Step 3: Implement the component**

Create `spa/src/components/balances/BalanceColumn.tsx`:

```tsx
import { BalanceColumnRow } from '@/components/balances/BalanceColumnRow';
import { Pagination } from '@/components/transactions/Pagination';
import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';

export const BALANCE_COLUMN_PAGE_SIZE = 8;

interface Props {
  label: 'Assets' | 'Liabilities';
  total: number;
  currency: string;
  rows: AccountBalance[]; // already sorted and sliced by the parent
  totalRowCount: number; // pre-slice length, used by pagination
  sortDir: 'asc' | 'desc';
  onToggleSort: () => void;
  offset: number;
  onOffsetChange: (offset: number) => void;
  emptyText: string;
}

export function BalanceColumn({
  label,
  total,
  currency,
  rows,
  totalRowCount,
  sortDir,
  onToggleSort,
  offset,
  onOffsetChange,
  emptyText,
}: Props) {
  const isEmpty = rows.length === 0;
  const negativeTotal = label === 'Liabilities';

  return (
    <Card className="overflow-hidden">
      <div className="flex items-center justify-between border-b bg-muted/40 px-3 py-2">
        <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {label}
        </div>
        <div
          className={cn(
            'text-sm font-semibold tabular-nums',
            negativeTotal ? 'text-destructive' : 'text-foreground',
          )}
        >
          {formatCents(total, currency)}
        </div>
      </div>

      {isEmpty ? (
        <CardContent className="p-6 text-center text-sm text-muted-foreground">
          {emptyText}
        </CardContent>
      ) : (
        <>
          <div className="flex items-center justify-between border-b px-3 py-2 text-xs uppercase text-muted-foreground">
            <span>Account</span>
            <button
              type="button"
              onClick={onToggleSort}
              className="inline-flex items-center gap-1 uppercase hover:text-foreground"
            >
              Balance
              <span aria-hidden="true">{sortDir === 'asc' ? '▲' : '▼'}</span>
            </button>
          </div>
          <div>
            {rows.map((row) => (
              <BalanceColumnRow key={row.account_id} row={row} />
            ))}
          </div>
          <div className="px-3 pb-3">
            <Pagination
              total={totalRowCount}
              limit={BALANCE_COLUMN_PAGE_SIZE}
              offset={offset}
              onChange={onOffsetChange}
            />
          </div>
        </>
      )}
    </Card>
  );
}
```

Note: the existing `Pagination` component already returns `null` when `total <= limit`, so the 8-row threshold is enforced there — no extra guard needed in `BalanceColumn`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npm test -- src/test/components/BalanceColumn.test.tsx`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/balances/BalanceColumn.tsx spa/src/test/components/BalanceColumn.test.tsx
git commit -m "feat(spa): BalanceColumn card with sort, pagination, empty state"
```

---

## Task 4: Wire the route

**Files:**
- Modify: `spa/src/routes/balances.tsx` (full rewrite)

Replace the route with the two-column layout. The existing `balances.test.tsx` ("renders Net Worth headline") must still pass after this task — Net Worth card markup is unchanged.

- [ ] **Step 1: Rewrite the route**

Replace the entire contents of `spa/src/routes/balances.tsx` with:

```tsx
import { BALANCE_COLUMN_PAGE_SIZE, BalanceColumn } from '@/components/balances/BalanceColumn';
import { NetWorthCard } from '@/components/NetWorthCard';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { naturalAmount } from '@/lib/accounts';
import { getBalances } from '@/lib/api';
import { summarizeBalances } from '@/lib/balances';
import {
  type BalancesSearch,
  parseBalancesSearch,
} from '@/lib/balances-search-params';
import { useServerConfig } from '@/lib/server-config';
import type { AccountBalance } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useMemo } from 'react';

export const Route = createFileRoute('/balances')({
  validateSearch: (s): BalancesSearch => parseBalancesSearch(s),
  component: BalancesPage,
});

function sortByNatural(rows: AccountBalance[], dir: 'asc' | 'desc'): AccountBalance[] {
  const out = [...rows];
  out.sort((a, b) => {
    const an = naturalAmount(a.type, a.amount);
    const bn = naturalAmount(b.type, b.amount);
    return dir === 'asc' ? an - bn : bn - an;
  });
  return out;
}

function BalancesPage() {
  const { defaults } = useServerConfig();
  const DEFAULT_CURRENCY = defaults.currency;
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/balances' });
  const query = useQuery({ queryKey: ['balances'], queryFn: getBalances });

  const toggleSort = (side: 'a' | 'l') => {
    const key = side === 'a' ? 'a_sort' : 'l_sort';
    const offsetKey = side === 'a' ? 'a_offset' : 'l_offset';
    navigate({
      search: (prev) => ({
        ...prev,
        [key]: prev[key] === 'balance_desc' ? 'balance_asc' : 'balance_desc',
        [offsetKey]: 0,
      }),
    });
  };

  const setOffset = (side: 'a' | 'l', offset: number) => {
    const key = side === 'a' ? 'a_offset' : 'l_offset';
    navigate({ search: (prev) => ({ ...prev, [key]: offset }) });
  };

  const summary = useMemo(
    () =>
      query.data
        ? summarizeBalances(query.data.items, DEFAULT_CURRENCY)
        : null,
    [query.data, DEFAULT_CURRENCY],
  );

  const assetsAll = useMemo(
    () => (summary ? summary.included.filter((r) => r.type === 'A') : []),
    [summary],
  );
  const liabilitiesAll = useMemo(
    () => (summary ? summary.included.filter((r) => r.type === 'L') : []),
    [summary],
  );

  const assetsSortDir: 'asc' | 'desc' =
    search.a_sort === 'balance_asc' ? 'asc' : 'desc';
  const liabilitiesSortDir: 'asc' | 'desc' =
    search.l_sort === 'balance_asc' ? 'asc' : 'desc';

  const assetsSorted = useMemo(
    () => sortByNatural(assetsAll, assetsSortDir),
    [assetsAll, assetsSortDir],
  );
  const liabilitiesSorted = useMemo(
    () => sortByNatural(liabilitiesAll, liabilitiesSortDir),
    [liabilitiesAll, liabilitiesSortDir],
  );

  const assetsPaged = useMemo(
    () =>
      assetsSorted.slice(search.a_offset, search.a_offset + BALANCE_COLUMN_PAGE_SIZE),
    [assetsSorted, search.a_offset],
  );
  const liabilitiesPaged = useMemo(
    () =>
      liabilitiesSorted.slice(
        search.l_offset,
        search.l_offset + BALANCE_COLUMN_PAGE_SIZE,
      ),
    [liabilitiesSorted, search.l_offset],
  );

  if (query.isPending) {
    return (
      <div>
        <Skeleton className="mb-6 h-32 w-full" />
        <div className="grid grid-cols-2 items-start gap-4">
          <Skeleton className="h-72 w-full" />
          <Skeleton className="h-72 w-full" />
        </div>
      </div>
    );
  }

  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load balances</AlertTitle>
        <AlertDescription className="mt-2 space-y-3">
          <div>{query.error instanceof Error ? query.error.message : 'Unknown error'}</div>
          <Button onClick={() => query.refetch()} size="sm">
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  if (!summary || query.data.items.length === 0) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <p className="max-w-md text-center text-sm text-muted-foreground">
          No accounts yet — run <code className="font-mono">kea ledger add</code> then create one
          via the CLI.
        </p>
      </div>
    );
  }

  return (
    <div>
      <NetWorthCard
        netWorth={summary.netWorth}
        assetsTotal={summary.assetsTotal}
        liabilitiesTotal={summary.liabilitiesTotal}
        currency={DEFAULT_CURRENCY}
        excludedCount={summary.excludedByCurrency.length}
      />

      <div className="mt-6 grid grid-cols-2 items-start gap-4">
        <BalanceColumn
          label="Assets"
          total={summary.assetsTotal}
          currency={DEFAULT_CURRENCY}
          rows={assetsPaged}
          totalRowCount={assetsSorted.length}
          sortDir={assetsSortDir}
          onToggleSort={() => toggleSort('a')}
          offset={search.a_offset}
          onOffsetChange={(off) => setOffset('a', off)}
          emptyText="No assets"
        />
        <BalanceColumn
          label="Liabilities"
          total={summary.liabilitiesTotal}
          currency={DEFAULT_CURRENCY}
          rows={liabilitiesPaged}
          totalRowCount={liabilitiesSorted.length}
          sortDir={liabilitiesSortDir}
          onToggleSort={() => toggleSort('l')}
          offset={search.l_offset}
          onOffsetChange={(off) => setOffset('l', off)}
          emptyText="No liabilities"
        />
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Regenerate the route tree (TanStack Router)**

Run: `cd spa && npx tsc -b` (this also triggers the router plugin via Vite — or if Vite codegen is needed, the dev server will re-emit `routeTree.gen.ts` on next start; `tsc -b` will surface type errors from the new search shape).

If the existing `routeTree.gen.ts` is stale on the search type, run `cd spa && npm run build` once to regenerate it, then revert any unwanted build artifacts before committing.

Expected: no TypeScript errors.

- [ ] **Step 3: Run the existing balances test to confirm Net Worth still renders**

Run: `cd spa && npm test -- src/test/balances.test.tsx`
Expected: PASS (the single existing test "renders Net Worth headline" still passes — Net Worth card markup is unchanged).

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/balances.tsx spa/src/routeTree.gen.ts
git commit -m "feat(spa): split balances into Assets/Liabilities columns with per-side sort and pagination"
```

(If `routeTree.gen.ts` did not change, omit it from the add.)

---

## Task 5: Route integration tests

**Files:**
- Modify: `spa/src/test/balances.test.tsx`

Expand the test file to cover the new behavior. Keep the existing Net Worth test as-is.

- [ ] **Step 1: Add the new test cases**

Append the following tests to `spa/src/test/balances.test.tsx` (after the existing `renders Net Worth headline` test, inside the same file scope so they share the `beforeEach` fetch stub):

```tsx
import userEvent from '@testing-library/user-event';
import { within } from '@testing-library/react';

test('partitions rows into Assets and Liabilities columns', async () => {
  render(makeTestApp('/balances'));

  // Column header bars
  const assetsHeader = await screen.findByText('Assets');
  const liabilitiesHeader = await screen.findByText('Liabilities');

  // Each column is a Card — the closest Card ancestor of the label header
  // holds the rows. Find the Card by walking up to the element with a role
  // or by scoping queries to the body container.
  const assetsCol = assetsHeader.closest('[class*="overflow-hidden"]');
  const liabilitiesCol = liabilitiesHeader.closest('[class*="overflow-hidden"]');
  if (!assetsCol || !liabilitiesCol) throw new Error('columns not found');

  expect(within(assetsCol as HTMLElement).getByText('Assets:Bank')).toBeInTheDocument();
  expect(within(assetsCol as HTMLElement).getByText('Assets:Cash')).toBeInTheDocument();
  expect(within(assetsCol as HTMLElement).queryByText('Liab:Card')).not.toBeInTheDocument();

  expect(within(liabilitiesCol as HTMLElement).getByText('Liab:Card')).toBeInTheDocument();
  expect(within(liabilitiesCol as HTMLElement).queryByText('Assets:Bank')).not.toBeInTheDocument();
});

test('default sort is descending by natural amount on both sides', async () => {
  render(makeTestApp('/balances'));

  // Assets: 125000 should come before 3500 (biggest first)
  const assetsHeader = await screen.findByText('Assets');
  const assetsCol = assetsHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const assetsNames = within(assetsCol).getAllByText(/Assets:/);
  expect(assetsNames[0]).toHaveTextContent('Assets:Bank');
  expect(assetsNames[1]).toHaveTextContent('Assets:Cash');

  // Both Balance headers show ▼ by default
  const balanceButtons = await screen.findAllByRole('button', { name: /Balance/i });
  expect(balanceButtons).toHaveLength(2);
  for (const btn of balanceButtons) {
    expect(btn).toHaveTextContent('▼');
  }
});

test('toggling the Assets sort changes only the Assets arrow', async () => {
  render(makeTestApp('/balances'));

  const assetsHeader = await screen.findByText('Assets');
  const assetsCol = assetsHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const assetsBtn = within(assetsCol).getByRole('button', { name: /Balance/i });
  const liabilitiesHeader = await screen.findByText('Liabilities');
  const liabilitiesCol = liabilitiesHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const liabilitiesBtn = within(liabilitiesCol).getByRole('button', { name: /Balance/i });

  expect(assetsBtn).toHaveTextContent('▼');
  expect(liabilitiesBtn).toHaveTextContent('▼');

  await userEvent.click(assetsBtn);

  expect(assetsBtn).toHaveTextContent('▲');
  expect(liabilitiesBtn).toHaveTextContent('▼');
});

test('sorts liabilities by natural amount so biggest debt comes first by default', async () => {
  // Override the global fetch stub with multiple liabilities of different sizes.
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({
            active: 'personal',
            items: [{ name: 'personal', path: '/p/personal.db', active: true }],
          }),
        );
      }
      if (url === '/api/balances') {
        return Promise.resolve(
          okResponse({
            items: [
              {
                account_id: 1,
                name: 'Liab:Small',
                type: 'L',
                currency: 'USD',
                amount: -1000, // smallest debt
                is_hidden: false,
              },
              {
                account_id: 2,
                name: 'Liab:Big',
                type: 'L',
                currency: 'USD',
                amount: -50000, // biggest debt
                is_hidden: false,
              },
            ],
            total_count: 2,
            limit: 0,
            offset: 0,
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );

  render(makeTestApp('/balances'));

  const liabilitiesHeader = await screen.findByText('Liabilities');
  const liabilitiesCol = liabilitiesHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const liabNames = within(liabilitiesCol).getAllByText(/Liab:/);
  // Biggest absolute debt first under natural-direction descending.
  expect(liabNames[0]).toHaveTextContent('Liab:Big');
  expect(liabNames[1]).toHaveTextContent('Liab:Small');
});

test('Assets pagination advances a_offset without touching the Liabilities column', async () => {
  // Build 10 assets (so pagination appears with 8-per-page) and 1 liability.
  const assetItems = Array.from({ length: 10 }, (_, i) => ({
    account_id: i + 1,
    name: `Assets:Acct${String(i + 1).padStart(2, '0')}`,
    type: 'A' as const,
    currency: 'USD',
    amount: (10 - i) * 1000, // descending so Acct01 is biggest
    is_hidden: false,
  }));
  const liabItem = {
    account_id: 100,
    name: 'Liab:Card',
    type: 'L' as const,
    currency: 'USD',
    amount: -4200,
    is_hidden: false,
  };

  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({
            active: 'personal',
            items: [{ name: 'personal', path: '/p/personal.db', active: true }],
          }),
        );
      }
      if (url === '/api/balances') {
        return Promise.resolve(
          okResponse({
            items: [...assetItems, liabItem],
            total_count: 11,
            limit: 0,
            offset: 0,
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );

  render(makeTestApp('/balances'));

  // Page 1 shows the 8 biggest assets — Acct09 and Acct10 are off-screen.
  await screen.findByText('Assets:Acct01');
  expect(screen.queryByText('Assets:Acct09')).not.toBeInTheDocument();
  expect(screen.queryByText('Assets:Acct10')).not.toBeInTheDocument();

  // Liabilities side: 1 row, no pagination on that side.
  const liabilitiesHeader = screen.getByText('Liabilities');
  const liabilitiesCol = liabilitiesHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  expect(within(liabilitiesCol).getByText('Liab:Card')).toBeInTheDocument();
  expect(within(liabilitiesCol).queryByText(/Page \d+ of/)).not.toBeInTheDocument();

  // Click Next on the Assets pagination.
  const assetsHeader = screen.getByText('Assets');
  const assetsCol = assetsHeader.closest('[class*="overflow-hidden"]') as HTMLElement;
  const nextBtn = within(assetsCol).getByRole('button', { name: /Next/i });
  await userEvent.click(nextBtn);

  // Page 2 shows the smallest two assets.
  await screen.findByText('Assets:Acct09');
  expect(screen.getByText('Assets:Acct10')).toBeInTheDocument();
  expect(screen.queryByText('Assets:Acct01')).not.toBeInTheDocument();

  // Liabilities side unchanged.
  expect(within(liabilitiesCol).getByText('Liab:Card')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the balances test file**

Run: `cd spa && npm test -- src/test/balances.test.tsx`
Expected: PASS — original test plus the five new ones (6 total).

- [ ] **Step 3: Commit**

```bash
git add spa/src/test/balances.test.tsx
git commit -m "test(spa): cover partition, default sort, and independent sort on /balances"
```

---

## Task 6: Cleanup — delete superseded components

**Files:**
- Delete: `spa/src/components/AccountListRow.tsx`
- Delete: `spa/src/components/TypeTotalCard.tsx`

- [ ] **Step 1: Verify no remaining callers**

Run:
```bash
grep -rn "AccountListRow\|TypeTotalCard" spa/src --include="*.ts" --include="*.tsx"
```
Expected: no results outside the two files themselves. If anything else appears, stop and report — the spec assumed these were the only callers.

- [ ] **Step 2: Delete the files**

```bash
rm spa/src/components/AccountListRow.tsx spa/src/components/TypeTotalCard.tsx
```

- [ ] **Step 3: Type-check the project**

Run: `cd spa && npx tsc -b`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add spa/src/components/AccountListRow.tsx spa/src/components/TypeTotalCard.tsx
git commit -m "chore(spa): remove AccountListRow and TypeTotalCard, superseded by BalanceColumn"
```

---

## Task 7: Final verification

- [ ] **Step 1: Run the full test suite**

Run: `cd spa && npm test`
Expected: all tests pass.

- [ ] **Step 2: Run the linter**

Run: `cd spa && npm run check`
Expected: clean output (no Biome errors).

- [ ] **Step 3: Build**

Run: `cd spa && npm run build`
Expected: build succeeds, no TypeScript errors.

- [ ] **Step 4: Manual smoke check (optional but recommended)**

Start the dev server (`cd spa && npm run dev`), point a browser at `/balances` for a ledger with both Assets and Liabilities. Verify:
- NetWorth card unchanged on top.
- Two columns visible, Assets on the left, Liabilities on the right, tops aligned.
- Each column shows its type total in the header bar.
- Balance column sortable per side; arrow flips on click.
- With more than 8 accounts of one type, pagination appears for that side only.
- Empty side (e.g., no liabilities) shows "No liabilities".

- [ ] **Step 5: No final commit needed** — verification only.

---

## Implementation Notes

- **No backend changes.** `/api/balances` and `summarizeBalances` are unchanged.
- **TanStack Router `validateSearch`.** Putting `validateSearch` directly on `/balances` (a leaf route) works the same way as on the `/accounts` layout route — the router parses URL params through zod before the component runs.
- **Sort comparator reuse.** `naturalAmount` from `spa/src/lib/accounts.ts` handles the A vs L sign convention. Don't reinvent it.
- **Pagination null-guard.** The shared `Pagination` component returns `null` when `total <= limit`, so the "hide pagination when ≤ 8 rows" rule is enforced inside it — `BalanceColumn` doesn't need its own guard.
- **Test router boilerplate.** `BalanceColumn.test.tsx` builds a tiny in-memory router because `<Link>` requires one. The route-level tests reuse `makeTestApp` which already provides the real router.
- **Column-identifier query.** Tests scope queries to a column by walking up from the `Assets` / `Liabilities` header to the nearest element with the `overflow-hidden` class (the `<Card>`). If the styling changes, that selector will need updating — keep it consistent with `BalanceColumn.tsx`.
