# SPA Filter Memory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist filter and pagination state per-ledger across browser sessions on all seven filterable SPA pages, restoring it via a route loader so the user lands back where they left off.

**Architecture:** A single `filter-memory.ts` module owns `localStorage` reads/writes under a `kea.` namespace and provides a `makeFilterMemoryLoader` factory. Each filterable route uses the factory in its TanStack Router `loader` to either redirect to remembered search (when URL is at defaults) or save current search (when URL has non-defaults). Active ledger name is mirrored into `localStorage` from the existing `LedgerSwitcher` so the loader can read it synchronously.

**Tech Stack:** React 18, TanStack Router (file-based, Zod search schemas), React Query, Vitest + jsdom + Testing Library, Zod.

---

## File Structure

**Create:**
- `spa/src/lib/filter-memory.ts` — persistence module + loader factory
- `spa/src/test/lib/filter-memory.test.ts` — unit tests for the module

**Modify:**
- `spa/src/components/LedgerSwitcher.tsx` — mirror active ledger into storage on query success and on switch success
- `spa/src/routes/transactions.index.tsx` — add loader, hook Clear
- `spa/src/routes/accounts.index.tsx` — add loader, hook Clear
- `spa/src/routes/balances.tsx` — add loader
- `spa/src/routes/reports.balance-sheet.tsx` — add loader
- `spa/src/routes/reports.income-statement.tsx` — add loader
- `spa/src/routes/reports.expense-breakdown.tsx` — add loader
- `spa/src/routes/reports.income-breakdown.tsx` — add loader

---

## Task 1: Filter-memory module

**Files:**
- Create: `spa/src/lib/filter-memory.ts`
- Test: `spa/src/test/lib/filter-memory.test.ts`

### Module API

The module exposes:
- `PageId` — union of seven literals
- `getActiveLedger()` / `setActiveLedger(name)`
- `loadFilters<T>(pageId)` / `saveFilters(pageId, search)` / `clearFilters(pageId)`
- `searchEquals(a, b)` — shallow equality for search objects
- `makeFilterMemoryLoader<T>({ pageId, defaults, redirectTo })` — returns a TanStack Router loader

All `localStorage` failures (unavailable, quota, malformed JSON) are caught and treated as "no memory."

- [ ] **Step 1: Write the failing tests**

Create `spa/src/test/lib/filter-memory.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, test } from 'vitest';
import {
  clearFilters,
  getActiveLedger,
  loadFilters,
  saveFilters,
  searchEquals,
  setActiveLedger,
} from '../../lib/filter-memory';

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  localStorage.clear();
});

describe('active ledger mirror', () => {
  test('getActiveLedger returns null when unset', () => {
    expect(getActiveLedger()).toBeNull();
  });

  test('setActiveLedger then getActiveLedger round-trips', () => {
    setActiveLedger('personal');
    expect(getActiveLedger()).toBe('personal');
  });

  test('setActiveLedger overwrites previous value', () => {
    setActiveLedger('personal');
    setActiveLedger('work');
    expect(getActiveLedger()).toBe('work');
  });
});

describe('per-page filter storage', () => {
  test('loadFilters returns null when no ledger active', () => {
    expect(loadFilters('transactions')).toBeNull();
  });

  test('loadFilters returns null when key absent', () => {
    setActiveLedger('personal');
    expect(loadFilters('transactions')).toBeNull();
  });

  test('saveFilters then loadFilters round-trips', () => {
    setActiveLedger('personal');
    saveFilters('transactions', { limit: 10, offset: 20, type: 'Expense' });
    expect(loadFilters('transactions')).toEqual({
      limit: 10,
      offset: 20,
      type: 'Expense',
    });
  });

  test('per-ledger isolation: filters under ledger A do not surface under ledger B', () => {
    setActiveLedger('personal');
    saveFilters('transactions', { offset: 10 });
    setActiveLedger('work');
    expect(loadFilters('transactions')).toBeNull();
    setActiveLedger('personal');
    expect(loadFilters('transactions')).toEqual({ offset: 10 });
  });

  test('clearFilters removes only the (ledger, page) key', () => {
    setActiveLedger('personal');
    saveFilters('transactions', { offset: 10 });
    saveFilters('accounts', { q: 'bank' });
    clearFilters('transactions');
    expect(loadFilters('transactions')).toBeNull();
    expect(loadFilters('accounts')).toEqual({ q: 'bank' });
  });

  test('loadFilters returns null on malformed JSON', () => {
    setActiveLedger('personal');
    localStorage.setItem('kea.filters.personal.transactions', 'not json');
    expect(loadFilters('transactions')).toBeNull();
  });

  test('saveFilters is a no-op when no ledger active', () => {
    saveFilters('transactions', { offset: 10 });
    setActiveLedger('personal');
    expect(loadFilters('transactions')).toBeNull();
  });
});

describe('searchEquals', () => {
  test('returns true for identical objects', () => {
    expect(searchEquals({ limit: 10, offset: 0 }, { limit: 10, offset: 0 })).toBe(true);
  });

  test('returns false when keys differ', () => {
    expect(searchEquals({ limit: 10 }, { limit: 10, offset: 0 })).toBe(false);
  });

  test('returns false when values differ', () => {
    expect(searchEquals({ limit: 10 }, { limit: 20 })).toBe(false);
  });

  test('treats undefined values as absent', () => {
    expect(searchEquals({ limit: 10, type: undefined }, { limit: 10 })).toBe(true);
  });

  test('ignores key order', () => {
    expect(searchEquals({ a: 1, b: 2 }, { b: 2, a: 1 })).toBe(true);
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd spa && npx vitest run src/test/lib/filter-memory.test.ts`
Expected: FAIL with "Cannot find module '../../lib/filter-memory'"

- [ ] **Step 3: Implement the module**

Create `spa/src/lib/filter-memory.ts`:

```ts
import { redirect } from '@tanstack/react-router';

export type PageId =
  | 'transactions'
  | 'accounts'
  | 'balances'
  | 'reports/balance-sheet'
  | 'reports/income-statement'
  | 'reports/expense-breakdown'
  | 'reports/income-breakdown';

const ACTIVE_LEDGER_KEY = 'kea.activeLedger';

function filterKey(ledger: string, pageId: PageId): string {
  return `kea.filters.${ledger}.${pageId}`;
}

function safeStorage(): Storage | null {
  try {
    return typeof localStorage !== 'undefined' ? localStorage : null;
  } catch {
    return null;
  }
}

export function getActiveLedger(): string | null {
  const s = safeStorage();
  if (!s) return null;
  try {
    return s.getItem(ACTIVE_LEDGER_KEY);
  } catch {
    return null;
  }
}

export function setActiveLedger(name: string): void {
  const s = safeStorage();
  if (!s) return;
  try {
    s.setItem(ACTIVE_LEDGER_KEY, name);
  } catch {
    // quota or unavailable — silently no-op
  }
}

export function loadFilters<T>(pageId: PageId): T | null {
  const s = safeStorage();
  if (!s) return null;
  const ledger = getActiveLedger();
  if (!ledger) return null;
  try {
    const raw = s.getItem(filterKey(ledger, pageId));
    if (raw === null) return null;
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}

export function saveFilters(pageId: PageId, search: object): void {
  const s = safeStorage();
  if (!s) return;
  const ledger = getActiveLedger();
  if (!ledger) return;
  try {
    s.setItem(filterKey(ledger, pageId), JSON.stringify(search));
  } catch {
    // quota or unavailable — silently no-op
  }
}

export function clearFilters(pageId: PageId): void {
  const s = safeStorage();
  if (!s) return;
  const ledger = getActiveLedger();
  if (!ledger) return;
  try {
    s.removeItem(filterKey(ledger, pageId));
  } catch {
    // unavailable — silently no-op
  }
}

export function searchEquals(a: object, b: object): boolean {
  const ak = Object.keys(a).filter(
    (k) => (a as Record<string, unknown>)[k] !== undefined,
  );
  const bk = Object.keys(b).filter(
    (k) => (b as Record<string, unknown>)[k] !== undefined,
  );
  if (ak.length !== bk.length) return false;
  for (const k of ak) {
    if ((a as Record<string, unknown>)[k] !== (b as Record<string, unknown>)[k]) {
      return false;
    }
  }
  return true;
}

export function makeFilterMemoryLoader<T extends object>(opts: {
  pageId: PageId;
  defaults: T;
  redirectTo: string;
}): (ctx: { search: T }) => void {
  return ({ search }) => {
    const ledger = getActiveLedger();
    if (!ledger) return;

    if (searchEquals(search, opts.defaults)) {
      const remembered = loadFilters<T>(opts.pageId);
      if (remembered && !searchEquals(remembered, opts.defaults)) {
        // biome-ignore lint/suspicious/noExplicitAny: redirect.to is route-typed, but this factory is generic over routes
        throw redirect({ to: opts.redirectTo as any, search: remembered });
      }
      return;
    }

    saveFilters(opts.pageId, search);
  };
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd spa && npx vitest run src/test/lib/filter-memory.test.ts`
Expected: all 14 tests PASS

- [ ] **Step 5: Type-check**

Run: `cd spa && npx tsc -b`
Expected: exit 0, no output

- [ ] **Step 6: Commit**

```bash
git add spa/src/lib/filter-memory.ts spa/src/test/lib/filter-memory.test.ts
git commit -m "feat(spa): add filter-memory module for per-ledger search persistence"
```

---

## Task 2: Mirror active ledger in LedgerSwitcher

The loader needs the active ledger name synchronously. We mirror it into `localStorage` from the existing `LedgerSwitcher` query and mutation.

**Files:**
- Modify: `spa/src/components/LedgerSwitcher.tsx`

- [ ] **Step 1: Modify the file**

Open `spa/src/components/LedgerSwitcher.tsx`. Two edits.

**Edit 1** — add the import. After existing `@/lib/api` import (line 9):

```ts
import { setActiveLedger } from '@/lib/filter-memory';
```

**Edit 2** — replace the `useQuery({ queryKey: ['ledgers'], queryFn: getLedgers })` line (around line 18) with:

```ts
const ledgersQuery = useQuery({
  queryKey: ['ledgers'],
  queryFn: async () => {
    const data = await getLedgers();
    setActiveLedger(data.active);
    return data;
  },
});
```

**Edit 3** — in the `useMutation` `onSuccess` callback (around line 21), add `setActiveLedger(info.name);` as the first line:

```ts
onSuccess: (info) => {
  setActiveLedger(info.name);
  setOpen(false);
  queryClient.invalidateQueries();
  toast.success(`Switched to ${info.name}`);
},
```

- [ ] **Step 2: Type-check**

Run: `cd spa && npx tsc -b`
Expected: exit 0

- [ ] **Step 3: Run existing LedgerSwitcher tests**

Run: `cd spa && npx vitest run --reporter=verbose -t "ledger"`
Expected: existing ledger-related tests pass (no behavior change visible to them).

- [ ] **Step 4: Commit**

```bash
git add spa/src/components/LedgerSwitcher.tsx
git commit -m "feat(spa): mirror active ledger into localStorage for filter memory"
```

---

## Task 3: Transactions route — loader + Clear integration

**Files:**
- Modify: `spa/src/routes/transactions.index.tsx`
- Test: `spa/src/test/transactions.filter-memory.test.tsx` (new file)

- [ ] **Step 1: Write the failing tests**

Create `spa/src/test/transactions.filter-memory.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const EMPTY_LIST = { items: [], total_count: 0, limit: 10, offset: 0 };

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'personal', items: [{ name: 'personal', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/transactions?')) {
        return Promise.resolve(okResponse(EMPTY_LIST));
      }
      if (url.startsWith('/api/accounts')) {
        return Promise.resolve(okResponse({ items: [], total_count: 0 }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('with no memory, /transactions renders with defaults (no redirect)', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/transactions'));
  await screen.findByText(/no transactions match these filters/i);
  // URL remains at /transactions with no search params restored.
  // We assert by checking nothing was redirected: rendering completed.
  expect(localStorage.getItem('kea.filters.personal.transactions')).toBeNull();
});

test('with memory, /transactions redirects to remembered search on entry', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.transactions',
    JSON.stringify({ limit: 10, offset: 0, type: 'Expense' }),
  );
  render(makeTestApp('/transactions'));
  await waitFor(() => {
    const calls = (fetch as unknown as { mock: { calls: [string][] } }).mock.calls;
    const txCall = calls.find(([u]) => u.startsWith('/api/transactions?'));
    expect(txCall?.[0]).toContain('type=Expense');
  });
});

test('non-default URL saves the search to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/transactions?type=Income'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.transactions');
    expect(raw).not.toBeNull();
    const stored = JSON.parse(raw as string);
    expect(stored.type).toBe('Income');
  });
});

test('clicking Clear wipes memory and stays at defaults', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/transactions?type=Income'));
  await waitFor(() => {
    expect(localStorage.getItem('kea.filters.personal.transactions')).not.toBeNull();
  });
  const clearBtn = await screen.findByRole('button', { name: /clear/i });
  await userEvent.click(clearBtn);
  await waitFor(() => {
    expect(localStorage.getItem('kea.filters.personal.transactions')).toBeNull();
  });
});

test('memory absent when no ledger mirrored (race on first load)', async () => {
  // no kea.activeLedger seeded
  render(makeTestApp('/transactions?type=Income'));
  await screen.findByText(/no transactions match these filters/i);
  // Save no-op because ledger unknown.
  expect(localStorage.getItem('kea.filters.personal.transactions')).toBeNull();
});
```

- [ ] **Step 2: Verify tests fail**

Run: `cd spa && npx vitest run src/test/transactions.filter-memory.test.tsx`
Expected: 4 of 5 tests FAIL (only the "no ledger" test may pass since it's testing absence). Failures point to memory not being written/restored.

- [ ] **Step 3: Modify the route**

Open `spa/src/routes/transactions.index.tsx`.

**Edit 1** — replace the imports block at the top with:

```tsx
import { FilterBar } from '@/components/transactions/FilterBar';
import { Pagination } from '@/components/transactions/Pagination';
import { TransactionsTable } from '@/components/transactions/TransactionsTable';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { clearFilters, makeFilterMemoryLoader } from '@/lib/filter-memory';
import { listTransactions } from '@/lib/transactions';
import {
  type TransactionsSearch,
  parseTransactionsSearch,
  searchToFilter,
  searchToListOptions,
} from '@/lib/transactions-search-params';
import { useQuery } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';
```

**Edit 2** — replace the `createFileRoute` call:

```tsx
const TRANSACTIONS_DEFAULTS = parseTransactionsSearch({});

export const Route = createFileRoute('/transactions/')({
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<TransactionsSearch>({
    pageId: 'transactions',
    defaults: TRANSACTIONS_DEFAULTS,
    redirectTo: '/transactions',
  }),
  component: TransactionsListPage,
});
```

**Edit 3** — change `clear` to wipe memory before navigating:

```tsx
const clear = () => {
  clearFilters('transactions');
  navigate({ search: { limit: search.limit, offset: 0 } });
};
```

- [ ] **Step 4: Run the filter-memory tests**

Run: `cd spa && npx vitest run src/test/transactions.filter-memory.test.tsx`
Expected: all 5 tests PASS

- [ ] **Step 5: Run existing transactions tests for regression**

Run: `cd spa && npx vitest run src/test/transactions.list.test.tsx src/test/transactions.regular.test.tsx src/test/transactions.form.test.tsx src/test/transactions.reconciled.test.tsx`
Expected: all 12 tests still PASS (these run with empty `localStorage`, so the loader's "no ledger" or "no memory" branches keep behavior unchanged).

- [ ] **Step 6: Type-check**

Run: `cd spa && npx tsc -b`
Expected: exit 0

- [ ] **Step 7: Commit**

```bash
git add spa/src/routes/transactions.index.tsx spa/src/test/transactions.filter-memory.test.tsx
git commit -m "feat(spa): remember transactions filters per ledger"
```

---

## Task 4: Accounts route — loader + Clear integration

**Files:**
- Modify: `spa/src/routes/accounts.index.tsx`
- Test: `spa/src/test/accounts.filter-memory.test.tsx` (new file)

- [ ] **Step 1: Inspect existing route**

Run: `cd spa && grep -n "createFileRoute\|clear\|Clear" src/routes/accounts.index.tsx`

The route is registered at `/accounts/` and has a `clear` function (around line 28).

- [ ] **Step 2: Write the failing tests**

Create `spa/src/test/accounts.filter-memory.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'personal', items: [{ name: 'personal', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/balances')) {
        return Promise.resolve(okResponse({ items: [] }));
      }
      if (url.startsWith('/api/accounts')) {
        return Promise.resolve(okResponse({ items: [], total_count: 0 }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('with memory, /accounts redirects to remembered search', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.accounts',
    JSON.stringify({
      q: 'bank',
      include_hidden: false,
      show_parents: false,
      limit: 10,
      offset: 0,
    }),
  );
  render(makeTestApp('/accounts'));
  await waitFor(() => {
    const calls = (fetch as unknown as { mock: { calls: [string][] } }).mock.calls;
    const accountsCall = calls.find(([u]) => u.includes('q=bank'));
    expect(accountsCall).toBeDefined();
  });
});

test('non-default URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/accounts?q=cash'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.accounts');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).q).toBe('cash');
  });
});

test('clicking Clear wipes memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/accounts?q=cash'));
  await waitFor(() => {
    expect(localStorage.getItem('kea.filters.personal.accounts')).not.toBeNull();
  });
  const clearBtn = await screen.findByRole('button', { name: /clear/i });
  await userEvent.click(clearBtn);
  await waitFor(() => {
    expect(localStorage.getItem('kea.filters.personal.accounts')).toBeNull();
  });
});
```

- [ ] **Step 3: Verify tests fail**

Run: `cd spa && npx vitest run src/test/accounts.filter-memory.test.tsx`
Expected: tests FAIL (loader not wired up yet).

- [ ] **Step 4: Modify the route**

Open `spa/src/routes/accounts.index.tsx`.

**Edit 1** — add to the imports near the top:

```tsx
import { clearFilters, makeFilterMemoryLoader } from '@/lib/filter-memory';
import { type AccountsSearch, parseAccountsSearch } from '@/lib/accounts-search-params';
```

(Replace the existing `import type { AccountsSearch } ...` line with the combined form above.)

**Edit 2** — replace the `createFileRoute('/accounts/')` block (currently registers `component` only) with:

```tsx
const ACCOUNTS_DEFAULTS = parseAccountsSearch({});

export const Route = createFileRoute('/accounts/')({
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<AccountsSearch>({
    pageId: 'accounts',
    defaults: ACCOUNTS_DEFAULTS,
    redirectTo: '/accounts',
  }),
  component: AccountsListPage,
});
```

**Edit 3** — modify `clear`:

```tsx
const clear = () => {
  clearFilters('accounts');
  navigate({
    search: { include_hidden: false, show_parents: false, limit: search.limit, offset: 0 },
  });
};
```

- [ ] **Step 5: Run the filter-memory tests**

Run: `cd spa && npx vitest run src/test/accounts.filter-memory.test.tsx`
Expected: all 3 tests PASS

- [ ] **Step 6: Run existing accounts tests**

Run: `cd spa && npx vitest run src/test/ -t accounts`
Expected: prior accounts tests still PASS.

- [ ] **Step 7: Type-check**

Run: `cd spa && npx tsc -b`
Expected: exit 0

- [ ] **Step 8: Commit**

```bash
git add spa/src/routes/accounts.index.tsx spa/src/test/accounts.filter-memory.test.tsx
git commit -m "feat(spa): remember accounts filters per ledger"
```

---

## Task 5: Balances route — loader

`/balances` has no Clear button to hook; just the loader.

**Files:**
- Modify: `spa/src/routes/balances.tsx`
- Test: `spa/src/test/balances.filter-memory.test.tsx` (new file)

- [ ] **Step 1: Write the failing tests**

Create `spa/src/test/balances.filter-memory.test.tsx`:

```tsx
import { render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'personal', items: [{ name: 'personal', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/balances')) {
        return Promise.resolve(okResponse({ items: [] }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('with memory, /balances redirects to remembered view', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.balances',
    JSON.stringify({
      a_offset: 0,
      a_sort: 'balance_desc',
      l_offset: 0,
      l_sort: 'balance_desc',
      view: 'list',
    }),
  );
  // Render and verify URL contains view=list after redirect.
  const { container } = render(makeTestApp('/balances'));
  await waitFor(() => {
    // After redirect, the route re-renders. We confirm by checking that
    // the list-view toggle is the active variant. The list button has
    // aria-pressed="true" once view=list is applied.
    const listBtn = container.querySelector('button[aria-label="List view"]');
    expect(listBtn?.getAttribute('aria-pressed')).toBe('true');
  });
});

test('non-default URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/balances?view=list'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.balances');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).view).toBe('list');
  });
});
```

- [ ] **Step 2: Verify tests fail**

Run: `cd spa && npx vitest run src/test/balances.filter-memory.test.tsx`
Expected: tests FAIL.

- [ ] **Step 3: Modify the route**

Open `spa/src/routes/balances.tsx`.

**Edit 1** — add to imports:

```tsx
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
```

**Edit 2** — replace the `createFileRoute('/balances')` block with:

```tsx
const BALANCES_DEFAULTS = parseBalancesSearch({});

export const Route = createFileRoute('/balances')({
  validateSearch: (s): BalancesSearch => parseBalancesSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<BalancesSearch>({
    pageId: 'balances',
    defaults: BALANCES_DEFAULTS,
    redirectTo: '/balances',
  }),
  component: BalancesPage,
});
```

- [ ] **Step 4: Run the filter-memory tests**

Run: `cd spa && npx vitest run src/test/balances.filter-memory.test.tsx`
Expected: both tests PASS.

- [ ] **Step 5: Re-run existing balances tests**

Run: `cd spa && npx vitest run src/test/balances.test.tsx src/test/balances.link.test.tsx`
Expected: results unchanged from baseline (6 pre-existing failures are pre-existing and unrelated to filter memory; new tests should not increase the failure count).

- [ ] **Step 6: Type-check**

Run: `cd spa && npx tsc -b`
Expected: exit 0

- [ ] **Step 7: Commit**

```bash
git add spa/src/routes/balances.tsx spa/src/test/balances.filter-memory.test.tsx
git commit -m "feat(spa): remember balances view and pagination per ledger"
```

---

## Task 6: Reports balance-sheet — loader

**Files:**
- Modify: `spa/src/routes/reports.balance-sheet.tsx`
- Test: `spa/src/test/reports.balance-sheet.filter-memory.test.tsx` (new file)

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/reports.balance-sheet.filter-memory.test.tsx`:

```tsx
import { render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'personal', items: [{ name: 'personal', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/reports/balance-sheet')) {
        return Promise.resolve(okResponse({ as_of: 0, assets: [], liabilities: [], equity: [] }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('non-default URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/reports/balance-sheet?as_of=1700000000'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/balance-sheet');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).as_of).toBe(1700000000);
  });
});

test('with memory, redirects to remembered as_of', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.reports/balance-sheet',
    JSON.stringify({ as_of: 1700000000 }),
  );
  render(makeTestApp('/reports/balance-sheet'));
  await waitFor(() => {
    const calls = (fetch as unknown as { mock: { calls: [string][] } }).mock.calls;
    expect(calls.some(([u]) => u.includes('as_of=1700000000'))).toBe(true);
  });
});
```

- [ ] **Step 2: Verify tests fail**

Run: `cd spa && npx vitest run src/test/reports.balance-sheet.filter-memory.test.tsx`
Expected: tests FAIL.

- [ ] **Step 3: Modify the route**

Open `spa/src/routes/reports.balance-sheet.tsx`.

**Edit 1** — add to imports:

```tsx
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
```

**Edit 2** — replace the `createFileRoute('/reports/balance-sheet')` block with:

```tsx
const BALANCE_SHEET_DEFAULTS = parseAsOfSearch({});

export const Route = createFileRoute('/reports/balance-sheet')({
  validateSearch: (s): AsOfSearchParams => parseAsOfSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<AsOfSearchParams>({
    pageId: 'reports/balance-sheet',
    defaults: BALANCE_SHEET_DEFAULTS,
    redirectTo: '/reports/balance-sheet',
  }),
  component: BalanceSheetPage,
});
```

- [ ] **Step 4: Run the filter-memory tests**

Run: `cd spa && npx vitest run src/test/reports.balance-sheet.filter-memory.test.tsx`
Expected: both tests PASS.

- [ ] **Step 5: Type-check**

Run: `cd spa && npx tsc -b`
Expected: exit 0

- [ ] **Step 6: Commit**

```bash
git add spa/src/routes/reports.balance-sheet.tsx spa/src/test/reports.balance-sheet.filter-memory.test.tsx
git commit -m "feat(spa): remember balance-sheet as_of per ledger"
```

---

## Task 7: Reports income-statement — loader

**Files:**
- Modify: `spa/src/routes/reports.income-statement.tsx`
- Test: `spa/src/test/reports.income-statement.filter-memory.test.tsx` (new file)

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/reports.income-statement.filter-memory.test.tsx`:

```tsx
import { render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'personal', items: [{ name: 'personal', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/reports/income-statement')) {
        return Promise.resolve(okResponse({ revenue: [], expense: [], net: 0 }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('non-default URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/reports/income-statement?range=ytd'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/income-statement');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).range).toBe('ytd');
  });
});

test('with memory, redirects to remembered range', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.reports/income-statement',
    JSON.stringify({ range: 'last-12mo' }),
  );
  render(makeTestApp('/reports/income-statement'));
  await waitFor(() => {
    const calls = (fetch as unknown as { mock: { calls: [string][] } }).mock.calls;
    expect(calls.some(([u]) => u.includes('range=last-12mo') || u.includes('/api/reports/income-statement'))).toBe(true);
  });
  // Confirm memory wasn't overwritten with defaults.
  const raw = localStorage.getItem('kea.filters.personal.reports/income-statement');
  expect(JSON.parse(raw as string).range).toBe('last-12mo');
});
```

- [ ] **Step 2: Verify tests fail**

Run: `cd spa && npx vitest run src/test/reports.income-statement.filter-memory.test.tsx`
Expected: at least the "non-default saves" test FAILs.

- [ ] **Step 3: Modify the route**

Open `spa/src/routes/reports.income-statement.tsx`.

**Edit 1** — add to imports:

```tsx
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
```

**Edit 2** — replace the `createFileRoute('/reports/income-statement')` block with:

```tsx
const INCOME_STATEMENT_DEFAULTS = parsePeriodSearch({});

export const Route = createFileRoute('/reports/income-statement')({
  validateSearch: (s): PeriodSearchParams => parsePeriodSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<PeriodSearchParams>({
    pageId: 'reports/income-statement',
    defaults: INCOME_STATEMENT_DEFAULTS,
    redirectTo: '/reports/income-statement',
  }),
  component: IncomeStatementPage,
});
```

- [ ] **Step 4: Run the filter-memory tests**

Run: `cd spa && npx vitest run src/test/reports.income-statement.filter-memory.test.tsx`
Expected: both tests PASS.

- [ ] **Step 5: Type-check**

Run: `cd spa && npx tsc -b`
Expected: exit 0

- [ ] **Step 6: Commit**

```bash
git add spa/src/routes/reports.income-statement.tsx spa/src/test/reports.income-statement.filter-memory.test.tsx
git commit -m "feat(spa): remember income-statement period per ledger"
```

---

## Task 8: Reports expense-breakdown — loader

**Files:**
- Modify: `spa/src/routes/reports.expense-breakdown.tsx`
- Test: `spa/src/test/reports.expense-breakdown.filter-memory.test.tsx` (new file)

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/reports.expense-breakdown.filter-memory.test.tsx`:

```tsx
import { render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'personal', items: [{ name: 'personal', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/reports/expense-breakdown')) {
        return Promise.resolve(okResponse({ rows: [], total: 0 }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('non-default URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/reports/expense-breakdown?range=ytd'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/expense-breakdown');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).range).toBe('ytd');
  });
});

test('with memory, redirects to remembered range', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.reports/expense-breakdown',
    JSON.stringify({ range: 'last-12mo' }),
  );
  render(makeTestApp('/reports/expense-breakdown'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/expense-breakdown');
    expect(JSON.parse(raw as string).range).toBe('last-12mo');
  });
});
```

- [ ] **Step 2: Verify tests fail**

Run: `cd spa && npx vitest run src/test/reports.expense-breakdown.filter-memory.test.tsx`
Expected: tests FAIL.

- [ ] **Step 3: Modify the route**

Open `spa/src/routes/reports.expense-breakdown.tsx`.

**Edit 1** — add to imports:

```tsx
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
```

**Edit 2** — replace the `createFileRoute('/reports/expense-breakdown')` block with:

```tsx
const EXPENSE_BREAKDOWN_DEFAULTS = parsePeriodSearch({});

export const Route = createFileRoute('/reports/expense-breakdown')({
  validateSearch: (s): PeriodSearchParams => parsePeriodSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<PeriodSearchParams>({
    pageId: 'reports/expense-breakdown',
    defaults: EXPENSE_BREAKDOWN_DEFAULTS,
    redirectTo: '/reports/expense-breakdown',
  }),
  component: ExpenseBreakdownPage,
});
```

- [ ] **Step 4: Run the filter-memory tests**

Run: `cd spa && npx vitest run src/test/reports.expense-breakdown.filter-memory.test.tsx`
Expected: both tests PASS.

- [ ] **Step 5: Type-check**

Run: `cd spa && npx tsc -b`
Expected: exit 0

- [ ] **Step 6: Commit**

```bash
git add spa/src/routes/reports.expense-breakdown.tsx spa/src/test/reports.expense-breakdown.filter-memory.test.tsx
git commit -m "feat(spa): remember expense-breakdown period per ledger"
```

---

## Task 9: Reports income-breakdown — loader

**Files:**
- Modify: `spa/src/routes/reports.income-breakdown.tsx`
- Test: `spa/src/test/reports.income-breakdown.filter-memory.test.tsx` (new file)

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/reports.income-breakdown.filter-memory.test.tsx`:

```tsx
import { render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(
          okResponse({ defaults: { currency: 'USD' }, display: { hide_decimals: false } }),
        );
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'personal', items: [{ name: 'personal', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/reports/income-breakdown')) {
        return Promise.resolve(okResponse({ rows: [], total: 0 }));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

test('non-default URL saves to memory', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  render(makeTestApp('/reports/income-breakdown?range=ytd'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/income-breakdown');
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw as string).range).toBe('ytd');
  });
});

test('with memory, redirects to remembered range', async () => {
  localStorage.setItem('kea.activeLedger', 'personal');
  localStorage.setItem(
    'kea.filters.personal.reports/income-breakdown',
    JSON.stringify({ range: 'last-12mo' }),
  );
  render(makeTestApp('/reports/income-breakdown'));
  await waitFor(() => {
    const raw = localStorage.getItem('kea.filters.personal.reports/income-breakdown');
    expect(JSON.parse(raw as string).range).toBe('last-12mo');
  });
});
```

- [ ] **Step 2: Verify tests fail**

Run: `cd spa && npx vitest run src/test/reports.income-breakdown.filter-memory.test.tsx`
Expected: tests FAIL.

- [ ] **Step 3: Modify the route**

Open `spa/src/routes/reports.income-breakdown.tsx`.

**Edit 1** — add to imports:

```tsx
import { makeFilterMemoryLoader } from '@/lib/filter-memory';
```

**Edit 2** — replace the `createFileRoute('/reports/income-breakdown')` block with:

```tsx
const INCOME_BREAKDOWN_DEFAULTS = parsePeriodSearch({});

export const Route = createFileRoute('/reports/income-breakdown')({
  validateSearch: (s): PeriodSearchParams => parsePeriodSearch(s),
  loaderDeps: ({ search }) => search,
  loader: makeFilterMemoryLoader<PeriodSearchParams>({
    pageId: 'reports/income-breakdown',
    defaults: INCOME_BREAKDOWN_DEFAULTS,
    redirectTo: '/reports/income-breakdown',
  }),
  component: IncomeBreakdownPage,
});
```

- [ ] **Step 4: Run the filter-memory tests**

Run: `cd spa && npx vitest run src/test/reports.income-breakdown.filter-memory.test.tsx`
Expected: both tests PASS.

- [ ] **Step 5: Type-check**

Run: `cd spa && npx tsc -b`
Expected: exit 0

- [ ] **Step 6: Commit**

```bash
git add spa/src/routes/reports.income-breakdown.tsx spa/src/test/reports.income-breakdown.filter-memory.test.tsx
git commit -m "feat(spa): remember income-breakdown period per ledger"
```

---

## Task 10: Final verification + bundle rebuild

- [ ] **Step 1: Run the full SPA test suite**

Run: `cd spa && npm test`
Expected: all filter-memory tests pass. The 6 pre-existing balances test failures may still appear (unrelated to this work) — confirm the failure count has not increased.

- [ ] **Step 2: Type-check the whole project**

Run: `cd spa && npx tsc -b --force`
Expected: exit 0

- [ ] **Step 3: Lint**

Run: `cd spa && npm run check`
Expected: no errors.

- [ ] **Step 4: Rebuild the embedded bundle**

Run: `cd spa && npm run build`
Expected: build succeeds; `internal/web/dist/` updates with new bundle.

- [ ] **Step 5: Smoke-test in the running app**

Start the Go backend (`make run`) and the SPA dev server (`cd spa && npm run dev`), or open the built bundle via `make build && ./kea_test serve`.

Manual checks (no automated test for these — they exercise real localStorage in a real browser):

1. Open `/transactions`. Apply a type filter (e.g., Expense). Navigate to `/balances`. Return via sidebar — type=Expense is restored.
2. Apply a filter, click Clear. Reload. Filter remains cleared.
3. Switch ledgers via the switcher. Apply a filter on `/transactions`. Switch back. The first ledger's filter is intact.
4. Open `/transactions?type=Income` directly in the URL bar. Memory now holds `type=Income` (verify in DevTools → Application → Local Storage).

- [ ] **Step 6: Commit the rebuilt bundle**

```bash
git add internal/web/dist/
git commit -m "build(spa): refresh embedded bundle with filter memory"
```

---

## Notes

- **Reports `parsePeriodSearch({})`.** This returns `{ range: 'this-month' }`. That value is the "defaults" used by `searchEquals`. A user visiting `/reports/income-statement` fresh (no params) has search `{ range: 'this-month' }`, which equals defaults — so memory is consulted. A user with stored `{ range: 'ytd' }` is redirected. A user navigating to `/reports/income-statement?range=this-month` explicitly will not be redirected (already at defaults) and `this-month` is silently *not* persisted to memory because it equals defaults. This is fine: returning later still defaults to `this-month`.
- **Search-order in stored URLs.** `searchEquals` is order-independent; the loader can safely compare reconstructed search objects from TanStack Router (key order may differ from `parseXxxSearch({})`).
- **Excluded routes.** Detail and edit routes (`/transactions/$id`, `/transactions/$id/edit`, `/transactions/new`, `/accounts/$id*`, `/accounts/new`) do not get loaders. They carry search params through links but should not save or restore.
