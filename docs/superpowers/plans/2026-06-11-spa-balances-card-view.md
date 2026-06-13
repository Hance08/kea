# SPA Balances Card View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `?view=list|cards` toggle to `/balances` that switches both columns between the existing list view and a 2-column inner card grid, with the same sort and pagination URL params shared by both views.

**Architecture:** Extend the existing `balancesSearchSchema` with one new `view` param. The route reads `search.view`, computes a per-row `share` (% of column total) once per column, and passes both into the existing `BalanceColumn` — which gains an internal switch between its current list body and a new `BalanceCardGrid`. In cards mode, the sort toggle migrates from the (now-absent) `Account / Balance` subheader into the column header bar. A new `ViewToggle` icon-button group sits above the column grid.

**Tech Stack:** React 18, TanStack Router (`validateSearch`), TanStack Query, zod, Tailwind, shadcn-style UI (Card, Button), `lucide-react` icons (already in deps — `LedgerSwitcher.tsx` is a precedent), Vitest + React Testing Library, Biome.

**Working directory:** `/Users/hance/programming/kea` on branch `feat/spa-balances-layout`. Run `spa`-prefixed commands from `spa/`.

**Spec:** `docs/superpowers/specs/2026-06-11-spa-balances-card-view-design.md`

---

## File Structure

**New files:**

| Path | Responsibility |
|---|---|
| `spa/src/components/balances/ViewToggle.tsx` | Two-button icon group; controlled component, owns no state. |
| `spa/src/components/balances/BalanceCard.tsx` | One card. Strips the column-type prefix, applies ellipsis + `title`, renders currency badge, balance, share line, and wraps everything in a `<Link>`. |
| `spa/src/components/balances/BalanceCardGrid.tsx` | `grid-cols-2 gap-3` grid of `BalanceCard`s plus the same invisible-placeholder padding to a fixed slot count that `BalanceColumn`'s list body already uses. |
| `spa/src/test/components/ViewToggle.test.tsx` | Unit tests for the toggle. |
| `spa/src/test/components/BalanceCard.test.tsx` | Unit tests for the card (prefix strip, share visibility, color, link target). |

**Modified files:**

| Path | Change |
|---|---|
| `spa/src/lib/balances-search-params.ts` | Add `view: z.enum(['list', 'cards']).default('list')`. |
| `spa/src/test/lib/balances-search-params.test.ts` | Add 2 tests: default `view`, accept `'cards'`, reject invalid. |
| `spa/src/components/balances/BalanceColumn.tsx` | Accept `view: 'list' \| 'cards'` and `shares: (number \| null)[]` props. In list mode keep the existing body. In cards mode render `<BalanceCardGrid>` and move the sort toggle into the column header bar. |
| `spa/src/test/components/BalanceColumn.test.tsx` | Update existing tests to pass `view="list"` explicitly; add new tests for cards-mode rendering and the moved sort button. |
| `spa/src/routes/balances.tsx` | Read `search.view`. Compute per-column shares aligned to the post-sort row list, slice them with the same offset as the rows. Render `<ViewToggle>` above the column grid. Pass `view`, `shares`, and `label` down to each `BalanceColumn`. |
| `spa/src/test/balances.test.tsx` | Add 3 tests: default view is list; clicking the cards toggle updates only `view`; switching view preserves sort + offset. |

---

## Task 1: Extend search params schema with `view`

**Files:**
- Modify: `spa/src/lib/balances-search-params.ts`
- Test: `spa/src/test/lib/balances-search-params.test.ts`

- [ ] **Step 1: Add the failing tests**

Append the following two tests to `spa/src/test/lib/balances-search-params.test.ts` inside the existing `describe('parseBalancesSearch', () => { ... })` block:

```ts
it('defaults view to "list" and accepts "cards"', () => {
  expect(parseBalancesSearch({}).view).toBe('list');
  expect(parseBalancesSearch({ view: 'cards' }).view).toBe('cards');
});

it('rejects unknown view values', () => {
  expect(() => parseBalancesSearch({ view: 'grid' })).toThrow();
});
```

Also update the existing `'returns defaults when nothing is set'` test to include `view: 'list'` in its expected output:

```ts
it('returns defaults when nothing is set', () => {
  expect(parseBalancesSearch({})).toEqual({
    a_offset: 0,
    a_sort: 'balance_desc',
    l_offset: 0,
    l_sort: 'balance_desc',
    view: 'list',
  });
});
```

And the `'coerces offsets from query strings'` and `'accepts both sort values per side'` tests need the same `view: 'list'` added to their expected objects.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd spa && npm test -- src/test/lib/balances-search-params.test.ts`
Expected: FAIL — the schema doesn't yet include `view`, so the new tests fail and the updated existing tests show "expected view: 'list' but got undefined".

- [ ] **Step 3: Implement the schema change**

Edit `spa/src/lib/balances-search-params.ts`. Add the new field and a shared view-enum schema:

```ts
import { z } from 'zod';

const sortSchema = z.enum(['balance_desc', 'balance_asc']);
const viewSchema = z.enum(['list', 'cards']);

export const balancesSearchSchema = z.object({
  a_offset: z.coerce.number().int().nonnegative().default(0),
  a_sort: sortSchema.default('balance_desc'),
  l_offset: z.coerce.number().int().nonnegative().default(0),
  l_sort: sortSchema.default('balance_desc'),
  view: viewSchema.default('list'),
});

export type BalancesSearch = z.infer<typeof balancesSearchSchema>;

export function parseBalancesSearch(s: Record<string, unknown>): BalancesSearch {
  return balancesSearchSchema.parse(s);
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npm test -- src/test/lib/balances-search-params.test.ts`
Expected: PASS — all tests including the new and updated ones.

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/balances-search-params.ts spa/src/test/lib/balances-search-params.test.ts
git commit -m "feat(spa): add view URL param to balances search schema"
```

---

## Task 2: ViewToggle component

**Files:**
- Create: `spa/src/components/balances/ViewToggle.tsx`
- Test: `spa/src/test/components/ViewToggle.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/components/ViewToggle.test.tsx`:

```tsx
import { ViewToggle } from '@/components/balances/ViewToggle';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

describe('ViewToggle', () => {
  it('renders a list button and a cards button', () => {
    render(<ViewToggle value="list" onChange={vi.fn()} />);
    expect(screen.getByRole('button', { name: /list view/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /cards view/i })).toBeInTheDocument();
  });

  it('marks the active button with aria-pressed=true and the inactive with false', () => {
    render(<ViewToggle value="cards" onChange={vi.fn()} />);
    expect(screen.getByRole('button', { name: /cards view/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: /list view/i })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('calls onChange with the clicked view', async () => {
    const onChange = vi.fn();
    render(<ViewToggle value="list" onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: /cards view/i }));
    expect(onChange).toHaveBeenCalledWith('cards');
    await userEvent.click(screen.getByRole('button', { name: /list view/i }));
    expect(onChange).toHaveBeenCalledWith('list');
  });

  it('does not call onChange when clicking the already-active button', async () => {
    const onChange = vi.fn();
    render(<ViewToggle value="list" onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: /list view/i }));
    expect(onChange).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npm test -- src/test/components/ViewToggle.test.tsx`
Expected: FAIL — module `@/components/balances/ViewToggle` not found.

- [ ] **Step 3: Implement the component**

Create `spa/src/components/balances/ViewToggle.tsx`:

```tsx
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/cn';
import { LayoutGrid, List } from 'lucide-react';

export type BalancesView = 'list' | 'cards';

interface Props {
  value: BalancesView;
  onChange: (next: BalancesView) => void;
}

export function ViewToggle({ value, onChange }: Props) {
  return (
    <div className="inline-flex items-center gap-1 rounded-md border bg-card p-0.5">
      <ToggleButton
        active={value === 'list'}
        label="List view"
        onClick={() => value !== 'list' && onChange('list')}
      >
        <List className="h-4 w-4" />
      </ToggleButton>
      <ToggleButton
        active={value === 'cards'}
        label="Cards view"
        onClick={() => value !== 'cards' && onChange('cards')}
      >
        <LayoutGrid className="h-4 w-4" />
      </ToggleButton>
    </div>
  );
}

function ToggleButton({
  active,
  label,
  onClick,
  children,
}: {
  active: boolean;
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      aria-label={label}
      aria-pressed={active}
      onClick={onClick}
      className={cn('h-7 w-7 p-0', active && 'bg-muted text-foreground')}
    >
      {children}
    </Button>
  );
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npm test -- src/test/components/ViewToggle.test.tsx`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/balances/ViewToggle.tsx spa/src/test/components/ViewToggle.test.tsx
git commit -m "feat(spa): ViewToggle icon-button group for balances list/cards switch"
```

---

## Task 3: BalanceCard component

**Files:**
- Create: `spa/src/components/balances/BalanceCard.tsx`
- Test: `spa/src/test/components/BalanceCard.test.tsx`

`BalanceCard` is a stateless link card. It strips the canonical column-type prefix, shows the currency badge, balance, and share line. The whole card is a `<Link>` to `/accounts/$id`.

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/components/BalanceCard.test.tsx`:

```tsx
import { BalanceCard } from '@/components/balances/BalanceCard';
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
import { describe, expect, it } from 'vitest';

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

const assetRow: AccountBalance = {
  account_id: 1,
  name: 'Assets:Investments:00878',
  type: 'A',
  currency: 'USD',
  amount: 1629_79_00,
  is_hidden: false,
};

const liabilityRow: AccountBalance = {
  account_id: 2,
  name: 'Liabilities:CreditCard:Visa',
  type: 'L',
  currency: 'USD',
  amount: -2140_00,
  is_hidden: false,
};

describe('BalanceCard', () => {
  it('strips the Assets: prefix in the Assets column', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    expect(await screen.findByText('Investments:00878')).toBeInTheDocument();
    expect(screen.queryByText(/^Assets:/)).not.toBeInTheDocument();
  });

  it('strips the Liabilities: prefix in the Liabilities column', async () => {
    renderWithRouter(<BalanceCard row={liabilityRow} columnLabel="Liabilities" share={56} />);
    expect(await screen.findByText('CreditCard:Visa')).toBeInTheDocument();
    expect(screen.queryByText(/^Liabilities:/)).not.toBeInTheDocument();
  });

  it('leaves non-canonical names unchanged', async () => {
    const oddRow = { ...assetRow, name: 'Bank:Checking' };
    renderWithRouter(<BalanceCard row={oddRow} columnLabel="Assets" share={10} />);
    expect(await screen.findByText('Bank:Checking')).toBeInTheDocument();
  });

  it('renders the currency badge and the balance amount without sign', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    expect(await screen.findByText('USD')).toBeInTheDocument();
    expect(screen.getByText('$162,979.00')).toBeInTheDocument();
  });

  it('renders the share line when share is a number', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    expect(await screen.findByText('75% of assets')).toBeInTheDocument();
  });

  it('renders the liabilities-side share wording', async () => {
    renderWithRouter(<BalanceCard row={liabilityRow} columnLabel="Liabilities" share={56} />);
    expect(await screen.findByText('56% of liabilities')).toBeInTheDocument();
  });

  it('hides the share line when share is null', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={null} />);
    expect(await screen.findByText('Investments:00878')).toBeInTheDocument();
    expect(screen.queryByText(/% of assets/)).not.toBeInTheDocument();
  });

  it('puts the original un-stripped name in a title tooltip', async () => {
    renderWithRouter(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    const nameSpan = await screen.findByText('Investments:00878');
    expect(nameSpan).toHaveAttribute('title', 'Assets:Investments:00878');
  });

  it('applies red color to a negative balance and green to a positive one', async () => {
    const { rerender } = renderWithRouter(
      <BalanceCard row={liabilityRow} columnLabel="Liabilities" share={56} />,
    );
    expect(await screen.findByText('$2,140.00')).toHaveClass('text-red-600');
    rerender(<BalanceCard row={assetRow} columnLabel="Assets" share={75} />);
    expect(screen.getByText('$162,979.00')).toHaveClass('text-green-600');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npm test -- src/test/components/BalanceCard.test.tsx`
Expected: FAIL — module `@/components/balances/BalanceCard` not found.

- [ ] **Step 3: Implement the component**

Create `spa/src/components/balances/BalanceCard.tsx`:

```tsx
import { balanceColor } from '@/lib/accounts';
import { cn } from '@/lib/cn';
import { formatBalanceAbs } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  row: AccountBalance;
  columnLabel: 'Assets' | 'Liabilities';
  share: number | null;
}

// Strip the canonical column-type prefix when present; leave non-canonical
// names unchanged so users with quirky ledger naming see what they typed.
function stripColumnPrefix(name: string, columnLabel: 'Assets' | 'Liabilities'): string {
  const prefix = `${columnLabel}:`;
  return name.startsWith(prefix) ? name.slice(prefix.length) : name;
}

export function BalanceCard({ row, columnLabel, share }: Props) {
  const displayName = stripColumnPrefix(row.name, columnLabel);
  const shareWording = columnLabel.toLowerCase(); // 'assets' | 'liabilities'

  return (
    <Link
      to="/accounts/$id"
      params={{ id: String(row.account_id) }}
      search={{ include_hidden: false, show_parents: false, limit: 10, offset: 0 }}
      className="flex h-full flex-col justify-between rounded-md border bg-card p-3 text-sm hover:bg-muted/40"
    >
      <div className="flex items-start justify-between gap-2">
        <span className="truncate text-xs text-muted-foreground" title={row.name}>
          {displayName}
        </span>
        <span className="shrink-0 rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700">
          {row.currency}
        </span>
      </div>
      <div className="mt-2">
        <div className={cn('text-lg font-bold tabular-nums', balanceColor(row.type, row.amount))}>
          {formatBalanceAbs(row.amount)}
        </div>
        {share !== null && (
          <div className="mt-0.5 text-[11px] text-muted-foreground">
            {share}% of {shareWording}
          </div>
        )}
      </div>
    </Link>
  );
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npm test -- src/test/components/BalanceCard.test.tsx`
Expected: PASS (9 tests).

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/balances/BalanceCard.tsx spa/src/test/components/BalanceCard.test.tsx
git commit -m "feat(spa): BalanceCard with stripped name, currency badge, and share line"
```

---

## Task 4: BalanceCardGrid component

**Files:**
- Create: `spa/src/components/balances/BalanceCardGrid.tsx`

This is a thin layout component — `BalanceCard.test.tsx` and `BalanceColumn.test.tsx` (Task 5) cover its observable behavior. No standalone test file.

- [ ] **Step 1: Create the component**

Create `spa/src/components/balances/BalanceCardGrid.tsx`:

```tsx
import { BALANCE_COLUMN_PAGE_SIZE } from '@/components/balances/BalanceColumn';
import { BalanceCard } from '@/components/balances/BalanceCard';
import type { AccountBalance } from '@/lib/types';

interface Props {
  rows: AccountBalance[]; // already sorted and sliced by the parent
  shares: (number | null)[]; // aligned with rows, same length
  columnLabel: 'Assets' | 'Liabilities';
}

export function BalanceCardGrid({ rows, shares, columnLabel }: Props) {
  const placeholderCount = BALANCE_COLUMN_PAGE_SIZE - rows.length;
  return (
    <div className="grid grid-cols-2 gap-3 p-3">
      {rows.map((row, i) => (
        <BalanceCard
          key={row.account_id}
          row={row}
          columnLabel={columnLabel}
          share={shares[i] ?? null}
        />
      ))}
      {Array.from({ length: placeholderCount }).map((_, i) => (
        <div
          // biome-ignore lint/suspicious/noArrayIndexKey: placeholder cards have no identity
          key={`placeholder-${i}`}
          aria-hidden="true"
          className="h-[88px] rounded-md"
        />
      ))}
    </div>
  );
}
```

The fixed `h-[88px]` matches the natural height of a populated `BalanceCard` (border + p-3 + name line + balance line + share line ≈ 88px), keeping the pagination at the same vertical position on partial pages.

- [ ] **Step 2: Verify it type-checks**

Run: `cd spa && npx tsc -b`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/balances/BalanceCardGrid.tsx
git commit -m "feat(spa): BalanceCardGrid 2-col layout for balances cards view"
```

---

## Task 5: BalanceColumn — accept `view`, switch render, move sort affordance

**Files:**
- Modify: `spa/src/components/balances/BalanceColumn.tsx`
- Modify: `spa/src/test/components/BalanceColumn.test.tsx`

The component gains two new required props (`view`, `shares`). In list mode it behaves exactly as today. In cards mode it renders `<BalanceCardGrid>` and the sort affordance moves into the header bar.

- [ ] **Step 1: Add failing cards-mode tests**

Edit `spa/src/test/components/BalanceColumn.test.tsx`. Add `view` and `shares` to `baseProps`:

```ts
const baseProps = {
  label: 'Assets' as const,
  total: 24310_00,
  sortDir: 'desc' as const,
  onToggleSort: vi.fn(),
  offset: 0,
  onOffsetChange: vi.fn(),
  emptyText: 'No assets',
  view: 'list' as const,
  shares: [] as (number | null)[],
};
```

Append the following new `it` blocks inside the existing `describe('BalanceColumn', ...)`:

```tsx
it('renders the list-mode subheader Balance button when view is list', async () => {
  renderWithRouter(
    <BalanceColumn {...baseProps} rows={makeRows(1, 'A')} totalRowCount={1} view="list" />,
  );
  expect(await screen.findByRole('button', { name: /^Balance ▼$/i })).toBeInTheDocument();
});

it('does not render the list-mode subheader Balance button when view is cards', async () => {
  renderWithRouter(
    <BalanceColumn
      {...baseProps}
      rows={makeRows(1, 'A')}
      totalRowCount={1}
      shares={[100]}
      view="cards"
    />,
  );
  await screen.findByText('Assets'); // wait for content
  expect(screen.queryByRole('button', { name: /^Balance ▼$/i })).not.toBeInTheDocument();
});

it('renders a header-bar sort button with the arrow + total amount in cards mode', async () => {
  renderWithRouter(
    <BalanceColumn
      {...baseProps}
      rows={makeRows(1, 'A')}
      totalRowCount={1}
      shares={[100]}
      view="cards"
    />,
  );
  const btn = await screen.findByRole('button', { name: /Sort by balance/i });
  expect(btn).toHaveTextContent('▼');
  expect(btn).toHaveTextContent('$24,310.00');
});

it('header-bar sort button toggles when clicked in cards mode', async () => {
  const onToggleSort = vi.fn();
  renderWithRouter(
    <BalanceColumn
      {...baseProps}
      rows={makeRows(1, 'A')}
      totalRowCount={1}
      shares={[100]}
      view="cards"
      onToggleSort={onToggleSort}
    />,
  );
  const btn = await screen.findByRole('button', { name: /Sort by balance/i });
  await userEvent.click(btn);
  expect(onToggleSort).toHaveBeenCalledTimes(1);
});

it('renders cards-grid content in cards mode', async () => {
  renderWithRouter(
    <BalanceColumn
      {...baseProps}
      rows={makeRows(2, 'A')}
      totalRowCount={2}
      shares={[60, 40]}
      view="cards"
    />,
  );
  // BalanceCard renders the stripped name and share line.
  expect(await screen.findByText('Acct1')).toBeInTheDocument();
  expect(screen.getByText('Acct2')).toBeInTheDocument();
  expect(screen.getByText('60% of assets')).toBeInTheDocument();
  expect(screen.getByText('40% of assets')).toBeInTheDocument();
});
```

You also need to tweak `makeRows` to produce names that exercise the prefix-strip: change the name template to `'Assets:Acct' + (i+1)` for type `'A'` and `'Liabilities:Acct' + (i+1)` for type `'L'`. (The earlier list-mode tests assert on the existing `Assets:AcctN` substring, which still matches because `Investments:00878` is a substring inside `Assets:Investments:00878` — verify carefully when adjusting that one assertion in `makeRows`.)

After updating `makeRows`, the existing list-mode test that uses `getAllByText(/Assets:/)` still passes because list-mode rows render the full name via `BalanceColumnRow` (which is unchanged).

- [ ] **Step 2: Run tests to verify the new ones fail**

Run: `cd spa && npm test -- src/test/components/BalanceColumn.test.tsx`
Expected: FAIL on the 5 new tests — the component doesn't yet accept `view`.

- [ ] **Step 3: Update BalanceColumn**

Replace the entire contents of `spa/src/components/balances/BalanceColumn.tsx` with:

```tsx
import { BalanceCardGrid } from '@/components/balances/BalanceCardGrid';
import { BalanceColumnRow } from '@/components/balances/BalanceColumnRow';
import { Pagination } from '@/components/transactions/Pagination';
import { Card, CardContent } from '@/components/ui/card';
import { balanceColor } from '@/lib/accounts';
import { cn } from '@/lib/cn';
import { formatBalanceAbs } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';

export const BALANCE_COLUMN_PAGE_SIZE = 8;

interface Props {
  label: 'Assets' | 'Liabilities';
  total: number;
  rows: AccountBalance[]; // already sorted and sliced by the parent
  shares: (number | null)[]; // aligned with rows; only consumed in cards mode
  totalRowCount: number; // pre-slice length, used by pagination
  sortDir: 'asc' | 'desc';
  onToggleSort: () => void;
  offset: number;
  onOffsetChange: (offset: number) => void;
  emptyText: string;
  view: 'list' | 'cards';
}

export function BalanceColumn({
  label,
  total,
  rows,
  shares,
  totalRowCount,
  sortDir,
  onToggleSort,
  offset,
  onOffsetChange,
  emptyText,
  view,
}: Props) {
  const isEmpty = rows.length === 0;
  const totalType = label === 'Assets' ? 'A' : 'L';
  const totalText = formatBalanceAbs(total);
  const totalColorClass = balanceColor(totalType, total);
  const arrow = sortDir === 'asc' ? '▲' : '▼';

  return (
    <Card className="overflow-hidden">
      <div className="flex items-center justify-between border-b bg-muted/40 px-3 py-2">
        <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {label}
        </div>
        {view === 'cards' && !isEmpty ? (
          <button
            type="button"
            onClick={onToggleSort}
            aria-label="Sort by balance"
            className={cn(
              'inline-flex items-center gap-1 text-sm font-semibold tabular-nums',
              'text-muted-foreground hover:text-foreground',
            )}
          >
            <span aria-hidden="true">{arrow}</span>
            <span className={totalColorClass}>{totalText}</span>
          </button>
        ) : (
          <div className={cn('text-sm font-semibold tabular-nums', totalColorClass)}>
            {totalText}
          </div>
        )}
      </div>

      {isEmpty ? (
        <CardContent className="p-6 text-center text-sm text-muted-foreground">
          {emptyText}
        </CardContent>
      ) : view === 'list' ? (
        <>
          <div className="flex items-center justify-between border-b px-3 py-2 text-xs uppercase text-muted-foreground">
            <span>Account</span>
            <button
              type="button"
              onClick={onToggleSort}
              className="inline-flex items-center gap-1 uppercase hover:text-foreground"
            >
              Balance
              <span aria-hidden="true">{arrow}</span>
            </button>
          </div>
          <div>
            {rows.map((row) => (
              <BalanceColumnRow key={row.account_id} row={row} />
            ))}
            {Array.from({ length: BALANCE_COLUMN_PAGE_SIZE - rows.length }).map((_, i) => (
              <div
                // biome-ignore lint/suspicious/noArrayIndexKey: placeholder rows have no identity
                key={`placeholder-${i}`}
                aria-hidden="true"
                className="px-3 py-2 text-sm"
              >
                &nbsp;
              </div>
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
      ) : (
        <>
          <BalanceCardGrid rows={rows} shares={shares} columnLabel={label} />
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

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npm test -- src/test/components/BalanceColumn.test.tsx`
Expected: PASS — original list-mode tests still green plus the 5 new cards-mode tests.

- [ ] **Step 5: Commit**

```bash
git add spa/src/components/balances/BalanceColumn.tsx spa/src/test/components/BalanceColumn.test.tsx
git commit -m "feat(spa): BalanceColumn renders cards view and moves sort to header bar"
```

---

## Task 6: Wire the route — compute shares, render ViewToggle, pass props

**Files:**
- Modify: `spa/src/routes/balances.tsx`

- [ ] **Step 1: Rewrite the route**

Replace the contents of `spa/src/routes/balances.tsx` with:

```tsx
import { NetWorthCard } from '@/components/NetWorthCard';
import { BALANCE_COLUMN_PAGE_SIZE, BalanceColumn } from '@/components/balances/BalanceColumn';
import { ViewToggle } from '@/components/balances/ViewToggle';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { naturalAmount } from '@/lib/accounts';
import { getBalances } from '@/lib/api';
import { summarizeBalances } from '@/lib/balances';
import { type BalancesSearch, parseBalancesSearch } from '@/lib/balances-search-params';
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

// Pre-slice shares: each row's |amount| / |total| as a whole percent.
// Returns nulls when total is zero (avoids 0/0 and the "0%" line on every card).
function computeShares(rows: AccountBalance[], total: number): (number | null)[] {
  if (total === 0) return rows.map(() => null);
  const denom = Math.abs(total);
  return rows.map((r) => Math.round((Math.abs(r.amount) / denom) * 100));
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

  const setView = (next: 'list' | 'cards') => {
    navigate({ search: (prev) => ({ ...prev, view: next }) });
  };

  const summary = useMemo(
    () => (query.data ? summarizeBalances(query.data.items, DEFAULT_CURRENCY) : null),
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

  const assetsSortDir: 'asc' | 'desc' = search.a_sort === 'balance_asc' ? 'asc' : 'desc';
  const liabilitiesSortDir: 'asc' | 'desc' = search.l_sort === 'balance_asc' ? 'asc' : 'desc';

  const assetsSorted = useMemo(
    () => sortByNatural(assetsAll, assetsSortDir),
    [assetsAll, assetsSortDir],
  );
  const liabilitiesSorted = useMemo(
    () => sortByNatural(liabilitiesAll, liabilitiesSortDir),
    [liabilitiesAll, liabilitiesSortDir],
  );

  const assetsSharesAll = useMemo(
    () => computeShares(assetsSorted, summary?.assetsTotal ?? 0),
    [assetsSorted, summary?.assetsTotal],
  );
  const liabilitiesSharesAll = useMemo(
    () => computeShares(liabilitiesSorted, summary?.liabilitiesTotal ?? 0),
    [liabilitiesSorted, summary?.liabilitiesTotal],
  );

  const assetsPaged = useMemo(
    () => assetsSorted.slice(search.a_offset, search.a_offset + BALANCE_COLUMN_PAGE_SIZE),
    [assetsSorted, search.a_offset],
  );
  const liabilitiesPaged = useMemo(
    () => liabilitiesSorted.slice(search.l_offset, search.l_offset + BALANCE_COLUMN_PAGE_SIZE),
    [liabilitiesSorted, search.l_offset],
  );

  const assetsPagedShares = useMemo(
    () => assetsSharesAll.slice(search.a_offset, search.a_offset + BALANCE_COLUMN_PAGE_SIZE),
    [assetsSharesAll, search.a_offset],
  );
  const liabilitiesPagedShares = useMemo(
    () => liabilitiesSharesAll.slice(search.l_offset, search.l_offset + BALANCE_COLUMN_PAGE_SIZE),
    [liabilitiesSharesAll, search.l_offset],
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
        currency={DEFAULT_CURRENCY}
        excludedCount={summary.excludedByCurrency.length}
      />

      <div className="mb-3 flex justify-end">
        <ViewToggle value={search.view} onChange={setView} />
      </div>

      <div className="grid grid-cols-2 items-start gap-4">
        <BalanceColumn
          label="Assets"
          total={summary.assetsTotal}
          rows={assetsPaged}
          shares={assetsPagedShares}
          totalRowCount={assetsSorted.length}
          sortDir={assetsSortDir}
          onToggleSort={() => toggleSort('a')}
          offset={search.a_offset}
          onOffsetChange={(off) => setOffset('a', off)}
          emptyText="No assets"
          view={search.view}
        />
        <BalanceColumn
          label="Liabilities"
          total={summary.liabilitiesTotal}
          rows={liabilitiesPaged}
          shares={liabilitiesPagedShares}
          totalRowCount={liabilitiesSorted.length}
          sortDir={liabilitiesSortDir}
          onToggleSort={() => toggleSort('l')}
          offset={search.l_offset}
          onOffsetChange={(off) => setOffset('l', off)}
          emptyText="No liabilities"
          view={search.view}
        />
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Type-check and run existing tests**

Run: `cd spa && npx tsc -b && npm test -- src/test/balances.test.tsx`
Expected: type-check clean; existing balances tests still pass — they don't assert anything view-specific yet, and the default view is `'list'` which matches today's rendering.

- [ ] **Step 3: Commit**

```bash
git add spa/src/routes/balances.tsx
git commit -m "feat(spa): wire ViewToggle, per-row shares, and view prop into /balances"
```

---

## Task 7: Route integration tests for view toggle and state preservation

**Files:**
- Modify: `spa/src/test/balances.test.tsx`

- [ ] **Step 1: Add the new tests**

Append the following to `spa/src/test/balances.test.tsx` after the existing tests (the file's `beforeEach`/`afterEach` and `okResponse` helper carry over):

```tsx
test('default view is list', async () => {
  render(makeTestApp('/balances'));
  // List-mode subheader has a Balance ▼ button.
  expect(await screen.findByRole('button', { name: /^Balance ▼$/i })).toBeInTheDocument();
  // Cards-mode header sort button is absent.
  expect(screen.queryByRole('button', { name: /Sort by balance/i })).not.toBeInTheDocument();
});

test('clicking the cards toggle switches both columns to cards mode', async () => {
  render(makeTestApp('/balances'));
  await screen.findByRole('button', { name: /^Balance ▼$/i });

  const cardsBtn = screen.getByRole('button', { name: /cards view/i });
  await userEvent.click(cardsBtn);

  // After switching: no list-mode subheader button.
  await waitFor(() => {
    expect(screen.queryByRole('button', { name: /^Balance ▼$/i })).not.toBeInTheDocument();
  });
  // Cards-mode header sort buttons present on the populated column.
  expect(screen.getAllByRole('button', { name: /Sort by balance/i }).length).toBeGreaterThan(0);
});

test('switching view preserves a_offset and a_sort', async () => {
  render(makeTestApp('/balances?a_offset=8&a_sort=balance_asc&view=list'));

  // Initially in list mode (already swapped to asc + offset 8 via URL).
  await screen.findByRole('button', { name: /^Balance ▲$/i });

  await userEvent.click(screen.getByRole('button', { name: /cards view/i }));

  // Cards-mode sort button still shows ascending.
  await waitFor(() => {
    const headerSort = screen.getAllByRole('button', { name: /Sort by balance/i });
    expect(headerSort.some((b) => b.textContent?.includes('▲'))).toBe(true);
  });
});
```

Add an import at the top if `waitFor` isn't already imported from `@testing-library/react`:

```ts
import { within, waitFor } from '@testing-library/react';
```

(Adjust the existing `within` import line rather than duplicating.)

- [ ] **Step 2: Run the balances test file**

Run: `cd spa && npm test -- src/test/balances.test.tsx`
Expected: PASS — original tests plus the three new ones (9 total).

- [ ] **Step 3: Commit**

```bash
git add spa/src/test/balances.test.tsx
git commit -m "test(spa): cover balances view toggle and state preservation"
```

---

## Task 8: Final verification

- [ ] **Step 1: Run the full test suite**

Run: `cd spa && npm test`
Expected: all tests pass.

- [ ] **Step 2: Run Biome**

Run: `cd spa && npm run check`
Expected: clean.

- [ ] **Step 3: Build**

Run: `cd spa && npm run build`
Expected: build succeeds, no TypeScript errors.

- [ ] **Step 4: Manual smoke check**

Start the dev server (`cd spa && npm run dev`). With a backend serving data, visit `/balances` and verify:
- Default view is list (matches current behavior).
- Click the grid icon — both columns switch to a 2-up card grid. Sort + page state are preserved.
- The sort button in cards mode lives in the column header bar next to the total, shows the arrow + amount, and toggles correctly.
- On a partial last page in cards mode, pagination stays at the same vertical position (placeholder cards fill the slot).
- Account names lose their `Assets:` / `Liabilities:` prefix on cards. Long names get an ellipsis; hovering shows the full name in a tooltip.
- Share line shows `<n>% of <assets|liabilities>` when the column total is non-zero; absent when total is zero.
- Click the list icon — back to list view, sort + page state still preserved.

- [ ] **Step 5: No final commit needed** — verification only.

---

## Implementation Notes

- **Existing helpers reused:** `naturalAmount`, `balanceColor`, `summarizeBalances`, `formatBalanceAbs`, `BALANCE_COLUMN_PAGE_SIZE`. No new shared logic.
- **TanStack Router `validateSearch`:** the `view` param is parsed through zod alongside the existing four params; the route reads it via `Route.useSearch()`.
- **Card height parity:** `BalanceCardGrid` placeholders use `h-[88px]`. If a future card content change shifts that natural height, update the placeholder height to match.
- **Share calculation timing:** computed on the post-sort but pre-slice list, then sliced by the same offset as the rows, so the per-card shares always agree with the total in the header bar regardless of which page is shown.
- **Sort affordance in cards mode:** the whole `arrow + amount` cluster is the button. The color rule on the amount (green / red / default) still applies; the arrow is muted by default and shifts to `text-foreground` on hover via the button's `hover:text-foreground` class.
- **Accessibility:** `ViewToggle` uses `aria-label` + `aria-pressed`. The cards-mode sort button uses `aria-label="Sort by balance"`.
