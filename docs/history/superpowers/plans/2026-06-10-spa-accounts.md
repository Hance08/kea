# SPA Accounts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the disabled **Accounts** sidebar item with a fully working route: hierarchical tree + flat search list, adaptive detail page (recent transactions for leaves; child accounts for parents), create + edit + delete forms, plus making Balances rows clickable into the detail page.

**Architecture:** New routes under `/accounts/*` (TanStack Router file-based), thin presentational components under `spa/src/components/accounts/`, API wrapper at `spa/src/lib/accounts.ts`, search-param zod schema at `spa/src/lib/accounts-search-params.ts`. Mutations refetch via `invalidateQueries`. No backend changes — all `/api/accounts/*` endpoints are already shipped. Reuses existing primitives from the Transactions slice where possible (`AccountCombobox`, `TypeBadge`, `StatusText`, `displayAmount`, `apiFetch`).

**Tech Stack:** TypeScript, React, TanStack Router, TanStack Query, react-hook-form, zod, Tailwind, shadcn/ui, Vitest + Testing Library + jsdom.

**Spec:** [`docs/superpowers/specs/2026-06-10-spa-accounts-design.md`](../specs/2026-06-10-spa-accounts-design.md)

---

## Conventions for every task

- All paths are relative to the repo root unless otherwise noted.
- Run commands from `spa/` (use `cd spa && …` if your shell is at the repo root).
- After each implementation task, run `npm run lint` and `npm test -- --run` from `spa/` before committing.
- Commit early and often — every numbered task ends with a commit step.
- Never include the `dist/` directory or anything in `.tanstack/` in commits.

---

## Phase A — Foundation: types, API client, search params

### Task 1: Extend `types.ts` with account-mutation shapes

**Files:**
- Modify: `spa/src/lib/types.ts`

`Account` and `AccountBalance` already exist. We add `AccountNode`, `CreateAccountInput`, `UpdateAccountInput`, and a `BalanceResponse`.

- [ ] **Step 1: Add types**

Append to `spa/src/lib/types.ts`:

```ts
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
  balance?: number; // optional opening balance, in cents
}

export interface UpdateAccountInput {
  name?: string;
  description?: string;
  is_hidden?: boolean;
}

export interface BalanceResponse {
  account_id: number;
  amount: number;
  currency: string;
}
```

- [ ] **Step 2: Verify types compile**

Run: `cd spa && npx tsc --noEmit`
Expected: no new errors. (Pre-existing errors, if any, must not have grown.)

- [ ] **Step 3: Commit**

```bash
git add spa/src/lib/types.ts
git commit -m "feat(spa): extend types with account-mutation shapes"
```

---

### Task 2: Create `lib/accounts.ts` API client

**Files:**
- Create: `spa/src/lib/accounts.ts`

Wrappers over `/api/accounts/*`. Includes the `IsOpeningBalancesAccount` helper used for the system-account banner.

- [ ] **Step 1: Write the failing test first**

Create `spa/src/test/lib/accounts.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { isOpeningBalancesAccount } from '@/lib/accounts';

describe('isOpeningBalancesAccount', () => {
  it('matches per-currency system accounts', () => {
    expect(isOpeningBalancesAccount('Equity:OpeningBalances_USD')).toBe(true);
    expect(isOpeningBalancesAccount('Equity:OpeningBalances_EUR')).toBe(true);
  });

  it('matches legacy single-name system account', () => {
    expect(isOpeningBalancesAccount('Equity:OpeningBalances')).toBe(true);
  });

  it('does not match unrelated accounts', () => {
    expect(isOpeningBalancesAccount('Equity:RetainedEarnings')).toBe(false);
    expect(isOpeningBalancesAccount('Assets:Bank:Checking')).toBe(false);
    expect(isOpeningBalancesAccount('')).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npm test -- --run src/test/lib/accounts.test.ts`
Expected: FAIL with "Cannot find module '@/lib/accounts'" or similar.

- [ ] **Step 3: Implement `lib/accounts.ts`**

Create `spa/src/lib/accounts.ts`:

```ts
import { apiFetch } from './api';
import type {
  Account,
  AccountNode,
  AccountType,
  BalanceResponse,
  CreateAccountInput,
  ListResult,
  UpdateAccountInput,
} from './types';

interface ListAccountsOpts {
  q?: string;
  type?: AccountType;
  include_hidden?: boolean;
  limit?: number;
  offset?: number;
}

function buildQuery(params: Record<string, string | number | boolean | undefined>): string {
  const usp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '' || v === null) continue;
    usp.set(k, String(v));
  }
  const s = usp.toString();
  return s ? `?${s}` : '';
}

export function listAccounts(opts: ListAccountsOpts = {}): Promise<ListResult<Account>> {
  const q = buildQuery({
    q: opts.q,
    type: opts.type,
    include_hidden: opts.include_hidden,
    limit: opts.limit,
    offset: opts.offset,
    include_count: true,
  });
  return apiFetch<ListResult<Account>>(`/api/accounts${q}`);
}

export function getAccountTree(opts: { include_hidden?: boolean } = {}): Promise<AccountNode[]> {
  const q = buildQuery({ include_hidden: opts.include_hidden });
  return apiFetch<AccountNode[]>(`/api/accounts/tree${q}`);
}

export function getAccount(id: number): Promise<Account> {
  return apiFetch<Account>(`/api/accounts/${id}`);
}

export function getAccountBalance(id: number): Promise<BalanceResponse> {
  return apiFetch<BalanceResponse>(`/api/accounts/${id}/balance`);
}

export function createAccount(input: CreateAccountInput): Promise<Account> {
  return apiFetch<Account>('/api/accounts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
}

export function updateAccount(id: number, patch: UpdateAccountInput): Promise<Account> {
  return apiFetch<Account>(`/api/accounts/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  });
}

export function deleteAccount(id: number): Promise<{ deleted: boolean; id: number }> {
  return apiFetch<{ deleted: boolean; id: number }>(`/api/accounts/${id}`, {
    method: 'DELETE',
  });
}

// Mirrors model.IsOpeningBalancesAccount: matches the per-currency form
// (Equity:OpeningBalances_<CCY>) and the legacy single-name form.
export function isOpeningBalancesAccount(name: string): boolean {
  if (name === 'Equity:OpeningBalances') return true;
  return /^Equity:OpeningBalances_[A-Z]{3}$/.test(name);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd spa && npm test -- --run src/test/lib/accounts.test.ts`
Expected: PASS — 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/accounts.ts spa/src/test/lib/accounts.test.ts
git commit -m "feat(spa): add accounts API client + isOpeningBalancesAccount helper"
```

---

### Task 3: `accounts-search-params.ts` zod schema

**Files:**
- Create: `spa/src/lib/accounts-search-params.ts`
- Test: `spa/src/test/lib/accounts-search-params.test.ts`

Validates the URL search params on `/accounts`. Used by the layout route's `validateSearch`.

- [ ] **Step 1: Write the failing test**

Create `spa/src/test/lib/accounts-search-params.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { parseAccountsSearch } from '@/lib/accounts-search-params';

describe('parseAccountsSearch', () => {
  it('returns defaults when nothing is set', () => {
    expect(parseAccountsSearch({})).toEqual({ include_hidden: false });
  });

  it('coerces include_hidden from string', () => {
    expect(parseAccountsSearch({ include_hidden: 'true' }).include_hidden).toBe(true);
    expect(parseAccountsSearch({ include_hidden: 'false' }).include_hidden).toBe(false);
  });

  it('keeps q and type when valid', () => {
    expect(parseAccountsSearch({ q: 'bank', type: 'A' })).toEqual({
      q: 'bank',
      type: 'A',
      include_hidden: false,
    });
  });

  it('drops unknown fields and rejects invalid type', () => {
    expect(() => parseAccountsSearch({ type: 'X' })).toThrow();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd spa && npm test -- --run src/test/lib/accounts-search-params.test.ts`
Expected: FAIL with "Cannot find module".

- [ ] **Step 3: Implement the schema**

Create `spa/src/lib/accounts-search-params.ts`:

```ts
import { z } from 'zod';

export const accountsSearchSchema = z.object({
  q: z.string().optional(),
  type: z.enum(['A', 'L', 'C', 'R', 'E']).optional(),
  include_hidden: z.coerce.boolean().default(false),
});

export type AccountsSearch = z.infer<typeof accountsSearchSchema>;

export function parseAccountsSearch(s: Record<string, unknown>): AccountsSearch {
  return accountsSearchSchema.parse(s);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd spa && npm test -- --run src/test/lib/accounts-search-params.test.ts`
Expected: PASS — 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add spa/src/lib/accounts-search-params.ts spa/src/test/lib/accounts-search-params.test.ts
git commit -m "feat(spa): add accounts-search-params zod schema"
```

---

## Phase B — Route skeletons + sidebar enablement

### Task 4: Layout route + placeholder list page + sidebar item

**Files:**
- Create: `spa/src/routes/accounts.tsx`
- Create: `spa/src/routes/accounts.index.tsx`
- Modify: `spa/src/components/Sidebar.tsx`

- [ ] **Step 1: Create the layout route**

Create `spa/src/routes/accounts.tsx`:

```tsx
import { type AccountsSearch, parseAccountsSearch } from '@/lib/accounts-search-params';
import { Outlet, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts')({
  validateSearch: (s): AccountsSearch => parseAccountsSearch(s),
  component: AccountsLayout,
});

function AccountsLayout() {
  return (
    <div>
      <Outlet />
    </div>
  );
}
```

- [ ] **Step 2: Create the placeholder index route**

Create `spa/src/routes/accounts.index.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/')({
  component: AccountsListPage,
});

function AccountsListPage() {
  return (
    <div>
      <h1 className="text-xl font-semibold">Accounts</h1>
      <p className="mt-2 text-sm text-muted-foreground">List view goes here.</p>
    </div>
  );
}
```

- [ ] **Step 3: Enable the sidebar item**

In `spa/src/components/Sidebar.tsx`, change:

```ts
{ label: 'Accounts' },
```

to:

```ts
{ label: 'Accounts', to: '/accounts' },
```

- [ ] **Step 4: Verify TanStack Router regenerates the route tree**

Run: `cd spa && npm run dev` in a background terminal long enough for it to write `routeTree.gen.ts`, then stop it. (Or run the typecheck which will fail informatively if the route tree isn't regenerated.)

Run: `cd spa && npx tsc --noEmit`
Expected: no new errors. If the route tree complains, run the dev server once to regenerate.

- [ ] **Step 5: Verify lint + tests pass**

```bash
cd spa && npm run lint && npm test -- --run
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add spa/src/routes/accounts.tsx spa/src/routes/accounts.index.tsx \
       spa/src/components/Sidebar.tsx spa/src/routeTree.gen.ts
git commit -m "feat(spa): scaffold accounts layout route + enable sidebar item"
```

---

### Task 5: Placeholder detail / new / edit routes

**Files:**
- Create: `spa/src/routes/accounts.$id.tsx`
- Create: `spa/src/routes/accounts.$id.index.tsx`
- Create: `spa/src/routes/accounts.$id.edit.tsx`
- Create: `spa/src/routes/accounts.new.tsx`

Following the Transactions precedent (`transactions.$id.tsx` is a layout, `transactions.$id.index.tsx` is the detail content), split `/accounts/$id` similarly so the edit child renders cleanly.

- [ ] **Step 1: Create the $id layout**

Create `spa/src/routes/accounts.$id.tsx`:

```tsx
import { Outlet, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/$id')({
  component: AccountIdLayout,
});

function AccountIdLayout() {
  return <Outlet />;
}
```

- [ ] **Step 2: Create the detail placeholder**

Create `spa/src/routes/accounts.$id.index.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/$id/')({
  component: AccountDetailPage,
});

function AccountDetailPage() {
  const { id } = Route.useParams();
  return <div>Detail for {id}</div>;
}
```

- [ ] **Step 3: Create the edit placeholder**

Create `spa/src/routes/accounts.$id.edit.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/$id/edit')({
  component: AccountEditPage,
});

function AccountEditPage() {
  const { id } = Route.useParams();
  return <div>Edit {id}</div>;
}
```

- [ ] **Step 4: Create the create placeholder**

Create `spa/src/routes/accounts.new.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/new')({
  component: AccountNewPage,
});

function AccountNewPage() {
  return <div>New account</div>;
}
```

- [ ] **Step 5: Regenerate route tree + verify**

Run: `cd spa && npm run dev` briefly to regenerate `routeTree.gen.ts`, then stop.
Run: `cd spa && npx tsc --noEmit && npm run lint && npm test -- --run`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add spa/src/routes/accounts.\$id.tsx spa/src/routes/accounts.\$id.index.tsx \
       spa/src/routes/accounts.\$id.edit.tsx spa/src/routes/accounts.new.tsx \
       spa/src/routeTree.gen.ts
git commit -m "feat(spa): scaffold accounts detail/new/edit routes"
```

---

## Phase C — Tree view list

### Task 6: `AccountTreeNode` presentational component

**Files:**
- Create: `spa/src/components/accounts/AccountTreeNode.tsx`

One row of the tree: chevron, indent based on depth, name (leaf segment), type/currency, balance.

- [ ] **Step 1: Implement the component**

Create `spa/src/components/accounts/AccountTreeNode.tsx`:

```tsx
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { Account } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  account: Account;
  depth: number;
  hasChildren: boolean;
  expanded: boolean;
  onToggle: () => void;
  balance?: { amount: number; currency: string };
}

export function AccountTreeNode({
  account,
  depth,
  hasChildren,
  expanded,
  onToggle,
  balance,
}: Props) {
  const leafName = account.name.split(':').pop() ?? account.name;
  return (
    <div
      className={cn(
        'flex items-center justify-between border-b border-border/60 px-2 py-1.5 text-sm hover:bg-muted/40',
        account.is_hidden && 'text-muted-foreground',
      )}
      style={{ paddingLeft: `${0.5 + depth * 1.25}rem` }}
    >
      <div className="flex items-center gap-1">
        {hasChildren ? (
          <button
            type="button"
            aria-label={expanded ? 'Collapse' : 'Expand'}
            aria-expanded={expanded}
            onClick={onToggle}
            className="inline-flex h-4 w-4 items-center justify-center text-xs text-muted-foreground"
          >
            {expanded ? '▾' : '▸'}
          </button>
        ) : (
          <span className="inline-block h-4 w-4" aria-hidden="true" />
        )}
        <Link
          to="/accounts/$id"
          params={{ id: String(account.id) }}
          className="hover:underline"
        >
          {leafName}
        </Link>
        {account.is_hidden && <span className="ml-2 text-xs uppercase">hidden</span>}
      </div>
      <div className="tabular-nums">
        {balance ? (
          <span className={cn(balance.amount < 0 && 'text-destructive')}>
            {formatCents(balance.amount, balance.currency)}
          </span>
        ) : (
          <Skeleton className="h-4 w-16" />
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify typecheck**

Run: `cd spa && npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/accounts/AccountTreeNode.tsx
git commit -m "feat(spa): add AccountTreeNode presentational component"
```

---

### Task 7: `AccountTree` recursive component with expand/collapse state

**Files:**
- Create: `spa/src/components/accounts/AccountTree.tsx`

Walks `AccountNode[]`, holds expand/collapse state, joins balances by `account_id`.

- [ ] **Step 1: Implement the component**

Create `spa/src/components/accounts/AccountTree.tsx`:

```tsx
import { AccountTreeNode } from '@/components/accounts/AccountTreeNode';
import type { AccountBalance, AccountNode } from '@/lib/types';
import { useState } from 'react';

interface Props {
  nodes: AccountNode[];
  balances?: AccountBalance[];
}

export function AccountTree({ nodes, balances }: Props) {
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const balanceById = new Map<number, { amount: number; currency: string }>();
  if (balances) {
    for (const b of balances) {
      balanceById.set(b.account_id, { amount: b.amount, currency: b.currency });
    }
  }

  const toggle = (id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div className="rounded-md border bg-card">
      {nodes.map((node) => (
        <Branch
          key={node.account.id}
          node={node}
          depth={0}
          expanded={expanded}
          onToggle={toggle}
          balanceById={balanceById}
        />
      ))}
    </div>
  );
}

interface BranchProps {
  node: AccountNode;
  depth: number;
  expanded: Set<number>;
  onToggle: (id: number) => void;
  balanceById: Map<number, { amount: number; currency: string }>;
}

function Branch({ node, depth, expanded, onToggle, balanceById }: BranchProps) {
  const hasChildren = node.children.length > 0;
  const isOpen = expanded.has(node.account.id);
  return (
    <>
      <AccountTreeNode
        account={node.account}
        depth={depth}
        hasChildren={hasChildren}
        expanded={isOpen}
        onToggle={() => onToggle(node.account.id)}
        balance={balanceById.get(node.account.id)}
      />
      {isOpen &&
        node.children.map((child) => (
          <Branch
            key={child.account.id}
            node={child}
            depth={depth + 1}
            expanded={expanded}
            onToggle={onToggle}
            balanceById={balanceById}
          />
        ))}
    </>
  );
}
```

- [ ] **Step 2: Verify typecheck**

Run: `cd spa && npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/accounts/AccountTree.tsx
git commit -m "feat(spa): add AccountTree recursive component with balance join"
```

---

### Task 8: `AccountFilters` (type + show-hidden)

**Files:**
- Create: `spa/src/components/accounts/AccountFilters.tsx`

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/AccountFilters.tsx`:

```tsx
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { AccountsSearch } from '@/lib/accounts-search-params';

interface Props {
  search: AccountsSearch;
  onChange: (partial: Partial<AccountsSearch>) => void;
  onClear: () => void;
}

const TYPE_OPTIONS: { value: AccountsSearch['type']; label: string }[] = [
  { value: undefined, label: 'All types' },
  { value: 'A', label: 'Asset' },
  { value: 'L', label: 'Liability' },
  { value: 'C', label: 'Equity' },
  { value: 'R', label: 'Revenue' },
  { value: 'E', label: 'Expense' },
];

export function AccountFilters({ search, onChange, onClear }: Props) {
  return (
    <div className="mb-4 flex flex-wrap items-center gap-2">
      <Input
        value={search.q ?? ''}
        placeholder="Search accounts…"
        className="w-56"
        onChange={(e) => onChange({ q: e.target.value || undefined })}
      />
      <select
        value={search.type ?? ''}
        onChange={(e) =>
          onChange({ type: (e.target.value || undefined) as AccountsSearch['type'] })
        }
        className="rounded-md border bg-background px-2 py-1.5 text-sm"
      >
        {TYPE_OPTIONS.map((opt) => (
          <option key={opt.label} value={opt.value ?? ''}>
            {opt.label}
          </option>
        ))}
      </select>
      <label className="flex items-center gap-1.5 text-sm">
        <input
          type="checkbox"
          checked={search.include_hidden}
          onChange={(e) => onChange({ include_hidden: e.target.checked })}
        />
        Show hidden
      </label>
      {(search.q || search.type || search.include_hidden) && (
        <Button size="sm" variant="ghost" onClick={onClear}>
          Clear
        </Button>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify typecheck**

Run: `cd spa && npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/accounts/AccountFilters.tsx
git commit -m "feat(spa): add AccountFilters with search/type/show-hidden controls"
```

---

### Task 9: Wire the tree into `accounts.index.tsx`

**Files:**
- Modify: `spa/src/routes/accounts.index.tsx`

Replace the placeholder. Renders tree mode when `q` is empty. (Search mode comes in Phase D — for now, when `q` is set, show a placeholder.)

- [ ] **Step 1: Replace the placeholder**

Overwrite `spa/src/routes/accounts.index.tsx`:

```tsx
import { AccountFilters } from '@/components/accounts/AccountFilters';
import { AccountTree } from '@/components/accounts/AccountTree';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getAccountTree } from '@/lib/accounts';
import type { AccountsSearch } from '@/lib/accounts-search-params';
import { getBalances } from '@/lib/api';
import { useQuery } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/')({
  component: AccountsListPage,
});

function AccountsListPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/accounts' });

  const setSearch = (partial: Partial<AccountsSearch>) => {
    navigate({ search: (prev) => ({ ...prev, ...partial }) });
  };
  const clear = () => {
    navigate({ search: { include_hidden: false } });
  };

  const treeQuery = useQuery({
    queryKey: ['accounts', 'tree', { include_hidden: search.include_hidden }],
    queryFn: () => getAccountTree({ include_hidden: search.include_hidden }),
    enabled: !search.q,
  });

  const balancesQuery = useQuery({
    queryKey: ['balances'],
    queryFn: getBalances,
  });

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Accounts</h1>
        <Button asChild size="sm">
          <Link to="/accounts/new">+ New account</Link>
        </Button>
      </div>

      <AccountFilters search={search} onChange={setSearch} onClear={clear} />

      {search.q && (
        <div className="rounded-md border bg-card p-8 text-center text-sm text-muted-foreground">
          Search mode is coming soon.
        </div>
      )}

      {!search.q && treeQuery.isPending && (
        <div className="space-y-2">
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-8 w-full" />
          ))}
        </div>
      )}

      {!search.q && treeQuery.isError && (
        <Alert variant="destructive">
          <AlertTitle>Failed to load accounts</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>
              {treeQuery.error instanceof Error ? treeQuery.error.message : 'Unknown error'}
            </div>
            <Button onClick={() => treeQuery.refetch()} size="sm">
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {!search.q && treeQuery.isSuccess && treeQuery.data.length === 0 && (
        <div className="rounded-md border bg-card p-8 text-center text-sm text-muted-foreground">
          No accounts yet — click + New account to create your first.
        </div>
      )}

      {!search.q && treeQuery.isSuccess && treeQuery.data.length > 0 && (
        <AccountTree nodes={treeQuery.data} balances={balancesQuery.data?.items} />
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify the route compiles**

Run: `cd spa && npx tsc --noEmit && npm run lint`
Expected: no new errors.

- [ ] **Step 3: Manual smoke test**

Run: `cd spa && npm run dev` in one terminal and `go run ./cmd/kea serve` in another.
Open the browser to the dev URL → click "Accounts" → tree should render with at least the root accounts. Expanding a root reveals children. Toggling "Show hidden" issues a new request. Stop the dev server.

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/accounts.index.tsx
git commit -m "feat(spa): render account tree on /accounts with filters"
```

---

### Task 10: Tree integration test

**Files:**
- Create: `spa/src/test/accounts.tree.test.tsx`

- [ ] **Step 1: Write the test**

Create `spa/src/test/accounts.tree.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeTestApp } from './test-app';
import type { AccountNode } from '@/lib/types';

const seedTree: AccountNode[] = [
  {
    account: {
      id: 1,
      name: 'Assets',
      type: 'A',
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
    children: [
      {
        account: {
          id: 2,
          name: 'Assets:Bank',
          type: 'A',
          parent_id: 1,
          currency: 'USD',
          description: '',
          is_hidden: false,
        },
        children: [
          {
            account: {
              id: 3,
              name: 'Assets:Bank:Checking',
              type: 'A',
              parent_id: 2,
              currency: 'USD',
              description: '',
              is_hidden: false,
            },
            children: [],
          },
        ],
      },
    ],
  },
];

vi.mock('@/lib/accounts', async () => ({
  ...(await vi.importActual<object>('@/lib/accounts')),
  getAccountTree: vi.fn().mockResolvedValue(seedTree),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getBalances: vi.fn().mockResolvedValue({
    items: [
      { account_id: 1, name: 'Assets', type: 'A', currency: 'USD', amount: 12340, is_hidden: false },
      {
        account_id: 2,
        name: 'Assets:Bank',
        type: 'A',
        currency: 'USD',
        amount: 8200,
        is_hidden: false,
      },
      {
        account_id: 3,
        name: 'Assets:Bank:Checking',
        type: 'A',
        currency: 'USD',
        amount: 3200,
        is_hidden: false,
      },
    ],
    total_count: 3,
    limit: 0,
    offset: 0,
  }),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
}));

describe('accounts list — tree mode', () => {
  it('renders root accounts and expands on chevron click', async () => {
    render(makeTestApp('/accounts'));
    await waitFor(() => expect(screen.getByText('Assets')).toBeInTheDocument());

    // 'Bank' is hidden until the root is expanded.
    expect(screen.queryByText('Bank')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /expand/i }));
    await waitFor(() => expect(screen.getByText('Bank')).toBeInTheDocument());
  });
});
```

- [ ] **Step 2: Run and verify it passes**

Run: `cd spa && npm test -- --run src/test/accounts.tree.test.tsx`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add spa/src/test/accounts.tree.test.tsx
git commit -m "test(spa): integration test for accounts tree list"
```

---

## Phase D — Search/flat mode

### Task 11: `AccountSearchResults` flat-table component

**Files:**
- Create: `spa/src/components/accounts/AccountSearchResults.tsx`

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/AccountSearchResults.tsx`:

```tsx
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { Account, AccountBalance } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  accounts: Account[];
  balances?: AccountBalance[];
  totalCount: number;
}

const TYPE_LABEL: Record<Account['type'], string> = {
  A: 'Asset',
  L: 'Liability',
  C: 'Equity',
  R: 'Revenue',
  E: 'Expense',
};

export function AccountSearchResults({ accounts, balances, totalCount }: Props) {
  const balanceById = new Map<number, { amount: number; currency: string }>();
  if (balances) {
    for (const b of balances) {
      balanceById.set(b.account_id, { amount: b.amount, currency: b.currency });
    }
  }
  return (
    <div className="space-y-2">
      {totalCount > accounts.length && (
        <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
          Refine search — showing first {accounts.length} of {totalCount} matches.
        </div>
      )}
      <div className="overflow-hidden rounded-md border bg-card">
        <table className="w-full text-sm">
          <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th className="px-3 py-2 text-left">Name</th>
              <th className="px-3 py-2 text-left">Type</th>
              <th className="px-3 py-2 text-left">Currency</th>
              <th className="px-3 py-2 text-right">Balance</th>
            </tr>
          </thead>
          <tbody>
            {accounts.map((acc) => {
              const bal = balanceById.get(acc.id);
              return (
                <tr key={acc.id} className="border-t hover:bg-muted/40">
                  <td className="px-3 py-1.5">
                    <Link
                      to="/accounts/$id"
                      params={{ id: String(acc.id) }}
                      className="hover:underline"
                    >
                      {acc.name}
                    </Link>
                  </td>
                  <td className="px-3 py-1.5">{TYPE_LABEL[acc.type]}</td>
                  <td className="px-3 py-1.5">{acc.currency}</td>
                  <td className="px-3 py-1.5 text-right tabular-nums">
                    {bal ? (
                      <span className={cn(bal.amount < 0 && 'text-destructive')}>
                        {formatCents(bal.amount, bal.currency)}
                      </span>
                    ) : (
                      '—'
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify typecheck**

Run: `cd spa && npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/accounts/AccountSearchResults.tsx
git commit -m "feat(spa): add AccountSearchResults flat table"
```

---

### Task 12: Wire search mode into `accounts.index.tsx`

**Files:**
- Modify: `spa/src/routes/accounts.index.tsx`

When `q` is set, swap from tree mode to search-results mode. Debounce is supplied by TanStack Query's stable query key + the user's natural typing cadence; for explicit debounce, the filter input writes immediately (this matches the Transactions filter precedent — URL is the source of truth, refetches are coalesced).

- [ ] **Step 1: Replace the placeholder**

In `spa/src/routes/accounts.index.tsx`, replace the placeholder branch:

```tsx
{search.q && (
  <div className="rounded-md border bg-card p-8 text-center text-sm text-muted-foreground">
    Search mode is coming soon.
  </div>
)}
```

with:

```tsx
{search.q && <SearchMode search={search} />}
```

Then add the following imports and component at the bottom of the file:

```tsx
import { AccountSearchResults } from '@/components/accounts/AccountSearchResults';
import { listAccounts } from '@/lib/accounts';

function SearchMode({ search }: { search: AccountsSearch }) {
  const balancesQuery = useQuery({
    queryKey: ['balances'],
    queryFn: getBalances,
  });
  const query = useQuery({
    queryKey: [
      'accounts',
      'search',
      { q: search.q, type: search.type, include_hidden: search.include_hidden },
    ],
    queryFn: () =>
      listAccounts({
        q: search.q,
        type: search.type,
        include_hidden: search.include_hidden,
        limit: 100,
      }),
  });

  if (query.isPending) {
    return (
      <div className="space-y-2">
        {[0, 1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-8 w-full" />
        ))}
      </div>
    );
  }

  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Search failed</AlertTitle>
        <AlertDescription className="mt-2 space-y-3">
          <div>{query.error instanceof Error ? query.error.message : 'Unknown error'}</div>
          <Button onClick={() => query.refetch()} size="sm">
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  if (query.data.items.length === 0) {
    return (
      <div className="rounded-md border bg-card p-8 text-center text-sm text-muted-foreground">
        No accounts match "{search.q}".
      </div>
    );
  }

  return (
    <AccountSearchResults
      accounts={query.data.items}
      balances={balancesQuery.data?.items}
      totalCount={query.data.total_count}
    />
  );
}
```

- [ ] **Step 2: Verify typecheck + lint**

Run: `cd spa && npx tsc --noEmit && npm run lint`
Expected: no new errors.

- [ ] **Step 3: Smoke test**

Run dev server + backend. Open `/accounts`, type "bank" into the search box, expect the page to switch to a flat table of matching accounts. Clear it; tree returns.

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/accounts.index.tsx
git commit -m "feat(spa): switch /accounts to flat search results when q is set"
```

---

### Task 13: Search integration test

**Files:**
- Create: `spa/src/test/accounts.search.test.tsx`

- [ ] **Step 1: Write the test**

Create `spa/src/test/accounts.search.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeTestApp } from './test-app';
import type { Account } from '@/lib/types';

const matches: Account[] = [
  {
    id: 3,
    name: 'Assets:Bank:Checking',
    type: 'A',
    parent_id: 2,
    currency: 'USD',
    description: '',
    is_hidden: false,
  },
  {
    id: 4,
    name: 'Assets:Bank:Savings',
    type: 'A',
    parent_id: 2,
    currency: 'USD',
    description: '',
    is_hidden: false,
  },
];

vi.mock('@/lib/accounts', async () => ({
  ...(await vi.importActual<object>('@/lib/accounts')),
  getAccountTree: vi.fn().mockResolvedValue([]),
  listAccounts: vi.fn().mockResolvedValue({
    items: matches,
    total_count: 2,
    limit: 100,
    offset: 0,
  }),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getBalances: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 0,
    offset: 0,
  }),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
}));

describe('accounts list — search mode', () => {
  it('renders flat results when q is typed', async () => {
    render(makeTestApp('/accounts'));
    const input = await screen.findByPlaceholderText(/search accounts/i);
    await userEvent.type(input, 'bank');

    await waitFor(() =>
      expect(screen.getByText('Assets:Bank:Checking')).toBeInTheDocument(),
    );
    expect(screen.getByText('Assets:Bank:Savings')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run and verify**

Run: `cd spa && npm test -- --run src/test/accounts.search.test.tsx`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add spa/src/test/accounts.search.test.tsx
git commit -m "test(spa): integration test for accounts search mode"
```

---

## Phase E — Detail page

### Task 14: `SystemAccountBanner` component

**Files:**
- Create: `spa/src/components/accounts/SystemAccountBanner.tsx`

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/SystemAccountBanner.tsx`:

```tsx
export function SystemAccountBanner() {
  return (
    <div className="mb-4 rounded-md border border-amber-500/40 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
      This is a system account. It cannot be deleted, and its name is managed automatically. Edit
      its description or hide it from views if needed.
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add spa/src/components/accounts/SystemAccountBanner.tsx
git commit -m "feat(spa): add SystemAccountBanner component"
```

---

### Task 15: `AccountDetailHeader` component

**Files:**
- Create: `spa/src/components/accounts/AccountDetailHeader.tsx`

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/AccountDetailHeader.tsx`:

```tsx
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { Account, BalanceResponse } from '@/lib/types';
import { Link } from '@tanstack/react-router';
import type { ReactNode } from 'react';

interface Props {
  account: Account;
  balance?: BalanceResponse;
  deleteSlot: ReactNode;
}

const TYPE_LABEL: Record<Account['type'], string> = {
  A: 'Asset',
  L: 'Liability',
  C: 'Equity',
  R: 'Revenue',
  E: 'Expense',
};

export function AccountDetailHeader({ account, balance, deleteSlot }: Props) {
  return (
    <div className="mb-4 rounded-md border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">{account.name}</h1>
          <div className="mt-1 text-sm text-muted-foreground">
            {TYPE_LABEL[account.type]} · {account.currency}
            {balance && (
              <>
                {' · '}
                <span className={cn(balance.amount < 0 && 'text-destructive', 'tabular-nums')}>
                  {formatCents(balance.amount, balance.currency)}
                </span>
              </>
            )}
            {account.is_hidden && (
              <span className="ml-2 rounded bg-muted px-1.5 py-0.5 text-xs uppercase">
                hidden
              </span>
            )}
          </div>
          {account.description && (
            <p className="mt-2 text-sm text-muted-foreground">{account.description}</p>
          )}
        </div>
        <div className="flex gap-2">
          <Button asChild size="sm" variant="outline">
            <Link to="/accounts/$id/edit" params={{ id: String(account.id) }}>
              Edit
            </Link>
          </Button>
          {deleteSlot}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify typecheck**

Run: `cd spa && npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/accounts/AccountDetailHeader.tsx
git commit -m "feat(spa): add AccountDetailHeader component"
```

---

### Task 16: `ChildAccountsCard` component

**Files:**
- Create: `spa/src/components/accounts/ChildAccountsCard.tsx`

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/ChildAccountsCard.tsx`:

```tsx
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance, AccountNode } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  children: AccountNode[];
  balances?: AccountBalance[];
}

export function ChildAccountsCard({ children, balances }: Props) {
  const balanceById = new Map<number, { amount: number; currency: string }>();
  if (balances) {
    for (const b of balances) {
      balanceById.set(b.account_id, { amount: b.amount, currency: b.currency });
    }
  }
  return (
    <div className="rounded-md border bg-card">
      <div className="border-b bg-muted/40 px-3 py-2 text-xs font-semibold uppercase text-muted-foreground">
        Child accounts
      </div>
      {children.length === 0 && (
        <div className="px-3 py-4 text-sm text-muted-foreground">No child accounts.</div>
      )}
      {children.map((node) => {
        const acc = node.account;
        const bal = balanceById.get(acc.id);
        const leafName = acc.name.split(':').pop() ?? acc.name;
        return (
          <div
            key={acc.id}
            className="flex items-center justify-between border-b border-border/60 px-3 py-2 text-sm last:border-b-0 hover:bg-muted/40"
          >
            <Link
              to="/accounts/$id"
              params={{ id: String(acc.id) }}
              className="hover:underline"
            >
              {leafName}
            </Link>
            {bal && (
              <span className={cn('tabular-nums', bal.amount < 0 && 'text-destructive')}>
                {formatCents(bal.amount, bal.currency)}
              </span>
            )}
          </div>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add spa/src/components/accounts/ChildAccountsCard.tsx
git commit -m "feat(spa): add ChildAccountsCard component"
```

---

### Task 17: `RecentTransactionsCard` component

**Files:**
- Create: `spa/src/components/accounts/RecentTransactionsCard.tsx`

Reuses pieces from the transactions slice. Reuses `displayAmount` and `TypeBadge`.

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/RecentTransactionsCard.tsx`:

```tsx
import { StatusText } from '@/components/transactions/StatusText';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { Skeleton } from '@/components/ui/skeleton';
import { formatCents } from '@/lib/format';
import { displayAmount } from '@/lib/transactionDisplay';
import { listTransactions } from '@/lib/transactions';
import type { TransactionDetail } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';

interface Props {
  accountId: number;
}

function formatDate(unix: number): string {
  const d = new Date(unix * 1000);
  return d.toISOString().slice(0, 10);
}

export function RecentTransactionsCard({ accountId }: Props) {
  const query = useQuery({
    queryKey: ['transactions', { account_id: accountId, limit: 20 }],
    queryFn: () => listTransactions({ account_id: accountId }, { limit: 20 }),
  });

  return (
    <div className="rounded-md border bg-card">
      <div className="flex items-center justify-between border-b bg-muted/40 px-3 py-2 text-xs font-semibold uppercase text-muted-foreground">
        <span>Recent transactions</span>
        <Link
          to="/transactions"
          search={{ account_id: accountId, limit: 50, offset: 0 }}
          className="font-normal lowercase hover:underline"
        >
          View all →
        </Link>
      </div>
      {query.isPending && (
        <div className="space-y-2 p-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-6 w-full" />
          ))}
        </div>
      )}
      {query.isSuccess && query.data.items.length === 0 && (
        <div className="px-3 py-4 text-sm text-muted-foreground">No transactions yet.</div>
      )}
      {query.isSuccess &&
        query.data.items.map((tx: TransactionDetail) => {
          const amount = displayAmount(tx);
          const split = tx.splits[0];
          return (
            <Link
              key={tx.id}
              to="/transactions/$id"
              params={{ id: String(tx.id) }}
              className="grid grid-cols-[6rem_5rem_1fr_6rem_5rem] items-center gap-3 border-b border-border/60 px-3 py-1.5 text-sm last:border-b-0 hover:bg-muted/40"
            >
              <span className="tabular-nums">{formatDate(tx.timestamp)}</span>
              <TypeBadge type={tx.type} />
              <span className="truncate">{tx.description}</span>
              <span className="text-right tabular-nums">
                {formatCents(amount, split?.currency ?? 'USD')}
              </span>
              <StatusText status={tx.status} />
            </Link>
          );
        })}
    </div>
  );
}
```

- [ ] **Step 2: Verify typecheck**

Run: `cd spa && npx tsc --noEmit`
Expected: no new errors. (If `displayAmount`'s signature differs, adjust the call accordingly — see [`spa/src/lib/transactionDisplay.ts`](../../spa/src/lib/transactionDisplay.ts).)

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/accounts/RecentTransactionsCard.tsx
git commit -m "feat(spa): add RecentTransactionsCard for account detail page"
```

---

### Task 18: `DeleteAccountButton` with blocked-delete tooltips

**Files:**
- Create: `spa/src/components/accounts/DeleteAccountButton.tsx`

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/DeleteAccountButton.tsx`:

```tsx
import { Button } from '@/components/ui/button';

interface Props {
  blockedReason?: string;
  onDelete: () => void;
  pending?: boolean;
}

export function DeleteAccountButton({ blockedReason, onDelete, pending }: Props) {
  if (blockedReason) {
    return (
      <Button size="sm" variant="destructive" disabled title={blockedReason}>
        Delete
      </Button>
    );
  }
  return (
    <Button size="sm" variant="destructive" onClick={onDelete} disabled={pending}>
      {pending ? 'Deleting…' : 'Delete'}
    </Button>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add spa/src/components/accounts/DeleteAccountButton.tsx
git commit -m "feat(spa): add DeleteAccountButton with blocked-reason tooltip"
```

---

### Task 19: Wire `accounts.$id.index.tsx` (detail page)

**Files:**
- Modify: `spa/src/routes/accounts.$id.index.tsx`

- [ ] **Step 1: Replace the placeholder**

Overwrite `spa/src/routes/accounts.$id.index.tsx`:

```tsx
import { AccountDetailHeader } from '@/components/accounts/AccountDetailHeader';
import { ChildAccountsCard } from '@/components/accounts/ChildAccountsCard';
import { DeleteAccountButton } from '@/components/accounts/DeleteAccountButton';
import { RecentTransactionsCard } from '@/components/accounts/RecentTransactionsCard';
import { SystemAccountBanner } from '@/components/accounts/SystemAccountBanner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  deleteAccount,
  getAccount,
  getAccountBalance,
  getAccountTree,
  isOpeningBalancesAccount,
} from '@/lib/accounts';
import { listTransactions } from '@/lib/transactions';
import type { AccountNode } from '@/lib/types';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';

export const Route = createFileRoute('/accounts/$id/')({
  component: AccountDetailPage,
});

function findChildren(roots: AccountNode[] | undefined, id: number): AccountNode[] | undefined {
  if (!roots) return undefined;
  const stack = [...roots];
  while (stack.length > 0) {
    const node = stack.pop();
    if (!node) continue;
    if (node.account.id === id) return node.children;
    stack.push(...node.children);
  }
  return undefined;
}

function AccountDetailPage() {
  const { id: idParam } = Route.useParams();
  const id = Number(idParam);
  const navigate = useNavigate();
  const qc = useQueryClient();

  const accountQuery = useQuery({ queryKey: ['account', id], queryFn: () => getAccount(id) });
  const balanceQuery = useQuery({
    queryKey: ['account', id, 'balance'],
    queryFn: () => getAccountBalance(id),
  });
  const treeQuery = useQuery({
    queryKey: ['accounts', 'tree', { include_hidden: true }],
    queryFn: () => getAccountTree({ include_hidden: true }),
  });
  const txQuery = useQuery({
    queryKey: ['transactions', { account_id: id, limit: 20 }],
    queryFn: () => listTransactions({ account_id: id }, { limit: 20 }),
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteAccount(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['accounts'] });
      qc.invalidateQueries({ queryKey: ['balances'] });
      toast.success('Account deleted');
      navigate({ to: '/accounts' });
    },
    onError: (e: unknown) => {
      toast.error(e instanceof Error ? e.message : 'Delete failed');
    },
  });

  if (accountQuery.isPending) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }
  if (accountQuery.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load account</AlertTitle>
        <AlertDescription className="mt-2 space-y-3">
          <div>
            {accountQuery.error instanceof Error
              ? accountQuery.error.message
              : 'Unknown error'}
          </div>
          <Button onClick={() => accountQuery.refetch()} size="sm">
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  const account = accountQuery.data;
  const isSystem = isOpeningBalancesAccount(account.name);
  const children = findChildren(treeQuery.data, id) ?? [];
  const isParent = children.length > 0;
  const hasTransactions = (txQuery.data?.total_count ?? 0) > 0;

  let blockedReason: string | undefined;
  if (isSystem) blockedReason = 'System account cannot be deleted.';
  else if (isParent) blockedReason = `Has ${children.length} child accounts. Delete or reassign them first.`;
  else if (hasTransactions)
    blockedReason = `Has ${txQuery.data?.total_count} transactions. Delete or reassign them first.`;

  const deleteSlot = isSystem ? null : (
    <DeleteAccountButton
      blockedReason={blockedReason}
      onDelete={() => deleteMutation.mutate()}
      pending={deleteMutation.isPending}
    />
  );

  return (
    <div>
      {isSystem && <SystemAccountBanner />}
      <AccountDetailHeader account={account} balance={balanceQuery.data} deleteSlot={deleteSlot} />
      {isParent ? (
        <ChildAccountsCard children={children} balances={undefined} />
      ) : (
        <RecentTransactionsCard accountId={id} />
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify typecheck + lint**

Run: `cd spa && npx tsc --noEmit && npm run lint`
Expected: no new errors.

- [ ] **Step 3: Smoke test**

Run dev server + backend. Navigate to `/accounts`, click a leaf row, expect detail header + recent transactions. Click a parent row, expect detail header + child accounts. Navigate to `/accounts/$id` of `Equity:OpeningBalances_USD` (look up the id via the API or the tree) — banner shows; Delete is absent.

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/accounts.\$id.index.tsx
git commit -m "feat(spa): wire account detail page (adaptive leaf/parent)"
```

---

### Task 20: Detail integration test

**Files:**
- Create: `spa/src/test/accounts.detail.test.tsx`

- [ ] **Step 1: Write the test**

Create `spa/src/test/accounts.detail.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { makeTestApp } from './test-app';
import type { Account, AccountNode } from '@/lib/types';

const leaf: Account = {
  id: 3,
  name: 'Assets:Bank:Checking',
  type: 'A',
  parent_id: 2,
  currency: 'USD',
  description: '',
  is_hidden: false,
};
const parent: Account = {
  id: 2,
  name: 'Assets:Bank',
  type: 'A',
  parent_id: 1,
  currency: 'USD',
  description: '',
  is_hidden: false,
};
const system: Account = {
  id: 99,
  name: 'Equity:OpeningBalances_USD',
  type: 'C',
  currency: 'USD',
  description: '',
  is_hidden: false,
};

const tree: AccountNode[] = [
  { account: parent, children: [{ account: leaf, children: [] }] },
  { account: system, children: [] },
];

vi.mock('@/lib/accounts', async () => {
  const actual = await vi.importActual<typeof import('@/lib/accounts')>('@/lib/accounts');
  return {
    ...actual,
    getAccount: vi.fn((id: number) =>
      Promise.resolve(id === 3 ? leaf : id === 2 ? parent : system),
    ),
    getAccountBalance: vi.fn(() =>
      Promise.resolve({ account_id: 0, amount: 0, currency: 'USD' }),
    ),
    getAccountTree: vi.fn().mockResolvedValue(tree),
  };
});

vi.mock('@/lib/transactions', async () => ({
  ...(await vi.importActual<object>('@/lib/transactions')),
  listTransactions: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 20,
    offset: 0,
  }),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
  getBalances: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 0,
    offset: 0,
  }),
}));

describe('accounts detail page', () => {
  it('renders recent transactions card for a leaf', async () => {
    render(makeTestApp('/accounts/3'));
    await waitFor(() => expect(screen.getByText(/recent transactions/i)).toBeInTheDocument());
  });

  it('renders child accounts card for a parent', async () => {
    render(makeTestApp('/accounts/2'));
    await waitFor(() => expect(screen.getByText(/child accounts/i)).toBeInTheDocument());
  });

  it('renders the system banner and hides delete for a system account', async () => {
    render(makeTestApp('/accounts/99'));
    await waitFor(() =>
      expect(screen.getByText(/system account/i)).toBeInTheDocument(),
    );
    expect(screen.queryByRole('button', { name: /^delete$/i })).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run and verify**

Run: `cd spa && npm test -- --run src/test/accounts.detail.test.tsx`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add spa/src/test/accounts.detail.test.tsx
git commit -m "test(spa): integration test for accounts detail page"
```

---

### Task 21: Delete-button blocked-state integration test

**Files:**
- Create: `spa/src/test/accounts.delete.test.tsx`

- [ ] **Step 1: Write the test**

Create `spa/src/test/accounts.delete.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { makeTestApp } from './test-app';
import type { Account, AccountNode } from '@/lib/types';

const leafWithTx: Account = {
  id: 5,
  name: 'Assets:Bank:Spending',
  type: 'A',
  parent_id: 2,
  currency: 'USD',
  description: '',
  is_hidden: false,
};

const tree: AccountNode[] = [{ account: leafWithTx, children: [] }];

vi.mock('@/lib/accounts', async () => {
  const actual = await vi.importActual<typeof import('@/lib/accounts')>('@/lib/accounts');
  return {
    ...actual,
    getAccount: vi.fn().mockResolvedValue(leafWithTx),
    getAccountBalance: vi.fn().mockResolvedValue({
      account_id: 5,
      amount: 1000,
      currency: 'USD',
    }),
    getAccountTree: vi.fn().mockResolvedValue(tree),
  };
});

vi.mock('@/lib/transactions', async () => ({
  ...(await vi.importActual<object>('@/lib/transactions')),
  listTransactions: vi.fn().mockResolvedValue({
    items: [],
    total_count: 7,
    limit: 20,
    offset: 0,
  }),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
  getBalances: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 0,
    offset: 0,
  }),
}));

describe('delete button blocked states', () => {
  it('disables delete with tooltip when account has transactions', async () => {
    render(makeTestApp('/accounts/5'));
    const button = await waitFor(() => screen.getByRole('button', { name: /^delete$/i }));
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('title', expect.stringContaining('7 transactions'));
  });
});
```

- [ ] **Step 2: Run and verify**

Run: `cd spa && npm test -- --run src/test/accounts.delete.test.tsx`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add spa/src/test/accounts.delete.test.tsx
git commit -m "test(spa): integration test for blocked delete state"
```

---

## Phase F — Create form

### Task 22: `TypeSelect` component

**Files:**
- Create: `spa/src/components/accounts/TypeSelect.tsx`

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/TypeSelect.tsx`:

```tsx
import { cn } from '@/lib/cn';
import type { AccountType } from '@/lib/types';

interface Props {
  value: AccountType | '';
  onChange: (value: AccountType) => void;
  disabled?: boolean;
}

const OPTIONS: { value: AccountType; label: string }[] = [
  { value: 'A', label: 'Asset' },
  { value: 'L', label: 'Liability' },
  { value: 'C', label: 'Equity' },
  { value: 'R', label: 'Revenue' },
  { value: 'E', label: 'Expense' },
];

export function TypeSelect({ value, onChange, disabled }: Props) {
  return (
    <div className="flex flex-wrap gap-2">
      {OPTIONS.map((opt) => (
        <button
          key={opt.value}
          type="button"
          disabled={disabled}
          onClick={() => onChange(opt.value)}
          className={cn(
            'rounded-md border px-3 py-1.5 text-sm transition-colors',
            value === opt.value
              ? 'border-primary bg-primary text-primary-foreground'
              : 'hover:bg-muted',
            disabled && 'cursor-not-allowed opacity-50',
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add spa/src/components/accounts/TypeSelect.tsx
git commit -m "feat(spa): add TypeSelect component"
```

---

### Task 23: `ParentAccountPicker` wrapper

**Files:**
- Create: `spa/src/components/accounts/ParentAccountPicker.tsx`

Thin wrapper around the existing `AccountCombobox` that constrains by the chosen type.

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/ParentAccountPicker.tsx`:

```tsx
import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import type { Account, AccountType } from '@/lib/types';

interface Props {
  value: string;
  onChange: (name: string, account?: Account) => void;
  // When set, only accounts of this type may be picked as parent (child must match parent type).
  restrictToType?: AccountType;
}

export function ParentAccountPicker({ value, onChange, restrictToType }: Props) {
  return (
    <AccountCombobox
      value={value}
      onChange={onChange}
      placeholder="No parent (top-level)"
      allowedTypes={restrictToType ? [restrictToType] : undefined}
    />
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add spa/src/components/accounts/ParentAccountPicker.tsx
git commit -m "feat(spa): add ParentAccountPicker wrapping AccountCombobox"
```

---

### Task 24: `OpeningBalanceField` component

**Files:**
- Create: `spa/src/components/accounts/OpeningBalanceField.tsx`

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/OpeningBalanceField.tsx`:

```tsx
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

interface Props {
  enabled: boolean;
  onEnabledChange: (next: boolean) => void;
  amount: string;
  onAmountChange: (v: string) => void;
  currency: string;
  error?: string;
}

export function OpeningBalanceField({
  enabled,
  onEnabledChange,
  amount,
  onAmountChange,
  currency,
  error,
}: Props) {
  return (
    <div className="rounded-md border bg-muted/20 p-3">
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => onEnabledChange(e.target.checked)}
        />
        Set opening balance
      </label>
      {enabled && (
        <div className="mt-3 space-y-1">
          <Label htmlFor="opening-amount">Amount ({currency})</Label>
          <Input
            id="opening-amount"
            inputMode="decimal"
            value={amount}
            onChange={(e) => onAmountChange(e.target.value)}
            placeholder="0.00"
            aria-invalid={error ? true : undefined}
          />
          <p className="text-xs text-muted-foreground">
            Posts an opening transaction against Equity:OpeningBalances_{currency}.
          </p>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add spa/src/components/accounts/OpeningBalanceField.tsx
git commit -m "feat(spa): add OpeningBalanceField component"
```

---

### Task 25: `AccountForm` shell (used by create + edit)

**Files:**
- Create: `spa/src/components/accounts/AccountForm.tsx`

Handles both create and edit modes. In edit mode, type/parent/currency render as read-only display rows.

- [ ] **Step 1: Implement**

Create `spa/src/components/accounts/AccountForm.tsx`:

```tsx
import { OpeningBalanceField } from '@/components/accounts/OpeningBalanceField';
import { ParentAccountPicker } from '@/components/accounts/ParentAccountPicker';
import { TypeSelect } from '@/components/accounts/TypeSelect';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { parseAmount } from '@/lib/format';
import { useServerConfig } from '@/lib/server-config';
import type { Account, AccountType, CreateAccountInput, UpdateAccountInput } from '@/lib/types';
import { useState } from 'react';

interface CreateProps {
  mode: 'create';
  onSubmit: (input: CreateAccountInput) => void;
  pending?: boolean;
  formError?: string;
  fieldErrors?: Record<string, string | undefined>;
}

interface EditProps {
  mode: 'edit';
  account: Account;
  parentName?: string;
  systemReadOnlyName?: boolean;
  onSubmit: (input: UpdateAccountInput) => void;
  pending?: boolean;
  formError?: string;
  fieldErrors?: Record<string, string | undefined>;
}

type Props = CreateProps | EditProps;

export function AccountForm(props: Props) {
  return props.mode === 'create' ? <CreateForm {...props} /> : <EditForm {...props} />;
}

function CreateForm(props: CreateProps) {
  const { defaults } = useServerConfig();
  const [parentName, setParentName] = useState('');
  const [parentAcc, setParentAcc] = useState<Account | undefined>(undefined);
  const [type, setType] = useState<AccountType | ''>('');
  const [name, setName] = useState('');
  const [currency, setCurrency] = useState(defaults.currency);
  const [description, setDescription] = useState('');
  const [balanceEnabled, setBalanceEnabled] = useState(false);
  const [balanceText, setBalanceText] = useState('');

  const effectiveType: AccountType | '' = parentAcc?.type ?? type;
  const effectiveCurrency = parentAcc?.currency ?? currency;

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!effectiveType) return;
    if (!name) return;
    const fullName = parentName ? `${parentName}:${name}` : name;
    const balanceCents = balanceEnabled ? parseAmount(balanceText) : undefined;
    props.onSubmit({
      name: fullName,
      type: effectiveType,
      parent_id: parentAcc?.id,
      currency: effectiveCurrency,
      description: description || undefined,
      balance: balanceCents,
    });
  };

  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-md border bg-card p-4">
      <h1 className="text-xl font-semibold">New account</h1>

      {props.formError && (
        <Alert variant="destructive">
          <AlertDescription>{props.formError}</AlertDescription>
        </Alert>
      )}

      <div className="space-y-1">
        <Label>Parent account (optional)</Label>
        <ParentAccountPicker
          value={parentName}
          onChange={(n, acc) => {
            setParentName(n);
            setParentAcc(acc);
          }}
        />
        {props.fieldErrors?.parent_id && (
          <p className="text-xs text-destructive">{props.fieldErrors.parent_id}</p>
        )}
      </div>

      <div className="space-y-1">
        <Label>Type</Label>
        <TypeSelect
          value={effectiveType}
          onChange={setType}
          disabled={Boolean(parentAcc)}
        />
        {parentAcc && (
          <p className="text-xs text-muted-foreground">
            Inherited from parent ({parentAcc.type}).
          </p>
        )}
        {props.fieldErrors?.type && (
          <p className="text-xs text-destructive">{props.fieldErrors.type}</p>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <Label htmlFor="acc-name">Name</Label>
          <Input
            id="acc-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={parentAcc ? 'Checking' : 'Assets'}
            aria-invalid={props.fieldErrors?.name ? true : undefined}
          />
          {name.includes(':') && (
            <p className="text-xs text-destructive">
              Name should not contain ':'. Use the Parent picker to nest.
            </p>
          )}
          {props.fieldErrors?.name && (
            <p className="text-xs text-destructive">{props.fieldErrors.name}</p>
          )}
        </div>
        <div className="space-y-1">
          <Label htmlFor="acc-currency">Currency</Label>
          <Input
            id="acc-currency"
            value={effectiveCurrency}
            onChange={(e) => setCurrency(e.target.value.toUpperCase())}
            disabled={Boolean(parentAcc)}
          />
          {parentAcc && (
            <p className="text-xs text-muted-foreground">Inherited from parent.</p>
          )}
          {props.fieldErrors?.currency && (
            <p className="text-xs text-destructive">{props.fieldErrors.currency}</p>
          )}
        </div>
      </div>

      <div className="space-y-1">
        <Label htmlFor="acc-desc">Description</Label>
        <Input
          id="acc-desc"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        {props.fieldErrors?.description && (
          <p className="text-xs text-destructive">{props.fieldErrors.description}</p>
        )}
      </div>

      <p className="text-xs text-muted-foreground">
        Hidden is set after creation — use the edit form to hide this account from default views.
      </p>

      <OpeningBalanceField
        enabled={balanceEnabled}
        onEnabledChange={setBalanceEnabled}
        amount={balanceText}
        onAmountChange={setBalanceText}
        currency={effectiveCurrency}
        error={props.fieldErrors?.balance}
      />

      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={() => window.history.back()}>
          Cancel
        </Button>
        <Button
          type="submit"
          disabled={props.pending || !effectiveType || !name || name.includes(':')}
        >
          {props.pending ? 'Creating…' : 'Create'}
        </Button>
      </div>
    </form>
  );
}

function EditForm(props: EditProps) {
  const [name, setName] = useState(props.account.name.split(':').pop() ?? props.account.name);
  const [description, setDescription] = useState(props.account.description);
  const [isHidden, setIsHidden] = useState(props.account.is_hidden);

  const TYPE_LABEL: Record<AccountType, string> = {
    A: 'Asset',
    L: 'Liability',
    C: 'Equity',
    R: 'Revenue',
    E: 'Expense',
  };

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const patch: UpdateAccountInput = {};
    const segments = props.account.name.split(':');
    segments[segments.length - 1] = name;
    const newFullName = segments.join(':');
    if (newFullName !== props.account.name) patch.name = newFullName;
    if (description !== props.account.description) patch.description = description;
    if (isHidden !== props.account.is_hidden) patch.is_hidden = isHidden;
    props.onSubmit(patch);
  };

  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-md border bg-card p-4">
      <h1 className="text-xl font-semibold">Edit account</h1>

      {props.formError && (
        <Alert variant="destructive">
          <AlertDescription>{props.formError}</AlertDescription>
        </Alert>
      )}

      <dl className="grid grid-cols-[8rem_1fr] gap-1 text-sm">
        <dt className="text-muted-foreground">Type</dt>
        <dd>
          {TYPE_LABEL[props.account.type]}{' '}
          <span className="text-xs text-muted-foreground">(cannot change)</span>
        </dd>
        <dt className="text-muted-foreground">Parent</dt>
        <dd>
          {props.parentName ?? '(top-level)'}{' '}
          <span className="text-xs text-muted-foreground">(cannot change)</span>
        </dd>
        <dt className="text-muted-foreground">Currency</dt>
        <dd>
          {props.account.currency}{' '}
          <span className="text-xs text-muted-foreground">(cannot change)</span>
        </dd>
      </dl>
      <p className="text-xs text-muted-foreground">
        Type, parent, and currency cannot be changed. Create a new account and move transactions
        instead.
      </p>

      <div className="space-y-1">
        <Label htmlFor="acc-name">Name</Label>
        <Input
          id="acc-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={props.systemReadOnlyName}
          aria-invalid={props.fieldErrors?.name ? true : undefined}
        />
        {props.fieldErrors?.name && (
          <p className="text-xs text-destructive">{props.fieldErrors.name}</p>
        )}
      </div>

      <div className="space-y-1">
        <Label htmlFor="acc-desc">Description</Label>
        <Input
          id="acc-desc"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>

      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={isHidden}
          onChange={(e) => setIsHidden(e.target.checked)}
        />
        Hidden
      </label>

      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={() => window.history.back()}>
          Cancel
        </Button>
        <Button type="submit" disabled={props.pending}>
          {props.pending ? 'Saving…' : 'Save'}
        </Button>
      </div>
    </form>
  );
}
```

- [ ] **Step 2: Verify typecheck**

Run: `cd spa && npx tsc --noEmit`
Expected: no new errors. If `parseAmount` does not exist or has a different signature, check [`spa/src/lib/format.ts`](../../spa/src/lib/format.ts) and adapt the call. The CLI ports use `int64` cents — `parseAmount` should return cents as a number.

- [ ] **Step 3: Commit**

```bash
git add spa/src/components/accounts/AccountForm.tsx
git commit -m "feat(spa): add AccountForm shell with create + edit modes"
```

---

### Task 26: Wire `accounts.new.tsx` with mutation

**Files:**
- Modify: `spa/src/routes/accounts.new.tsx`

- [ ] **Step 1: Replace the placeholder**

Overwrite `spa/src/routes/accounts.new.tsx`:

```tsx
import { AccountForm } from '@/components/accounts/AccountForm';
import { createAccount } from '@/lib/accounts';
import { ApiError } from '@/lib/api';
import type { CreateAccountInput } from '@/lib/types';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { toast } from 'sonner';

export const Route = createFileRoute('/accounts/new')({
  component: NewAccountPage,
});

function NewAccountPage() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [fieldErrors, setFieldErrors] = useState<Record<string, string | undefined>>({});
  const [formError, setFormError] = useState<string | undefined>(undefined);

  const mutation = useMutation({
    mutationFn: (input: CreateAccountInput) => createAccount(input),
    onSuccess: (acc) => {
      qc.invalidateQueries({ queryKey: ['accounts'] });
      qc.invalidateQueries({ queryKey: ['balances'] });
      toast.success('Account created');
      navigate({ to: '/accounts/$id', params: { id: String(acc.id) } });
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.field) {
          setFieldErrors({ [err.field]: err.message });
          setFormError(undefined);
        } else {
          setFormError(err.message);
        }
        return;
      }
      setFormError(err instanceof Error ? err.message : 'Create failed');
    },
  });

  return (
    <AccountForm
      mode="create"
      pending={mutation.isPending}
      formError={formError}
      fieldErrors={fieldErrors}
      onSubmit={(input) => {
        setFieldErrors({});
        setFormError(undefined);
        mutation.mutate(input);
      }}
    />
  );
}
```

- [ ] **Step 2: Verify typecheck + lint**

Run: `cd spa && npx tsc --noEmit && npm run lint`
Expected: no new errors.

- [ ] **Step 3: Smoke test**

Run dev server + backend. From `/accounts`, click "+ New account", fill the form (Name: `TestAcct`, Type: Asset, Currency: USD), submit. Expect navigation to `/accounts/$id` for the new account. Also test the opening-balance path: tick the checkbox, enter `100.00`, submit. Verify the new account shows up in the tree.

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/accounts.new.tsx
git commit -m "feat(spa): wire account create form with mutation + field error mapping"
```

---

### Task 27: Create-form integration test

**Files:**
- Create: `spa/src/test/accounts.form.test.tsx`

- [ ] **Step 1: Write the test**

Create `spa/src/test/accounts.form.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeTestApp } from './test-app';

const createAccount = vi.fn().mockResolvedValue({
  id: 42,
  name: 'Assets:NewBank',
  type: 'A',
  currency: 'USD',
  description: '',
  is_hidden: false,
});

vi.mock('@/lib/accounts', async () => ({
  ...(await vi.importActual<object>('@/lib/accounts')),
  createAccount,
  getAccountTree: vi.fn().mockResolvedValue([]),
}));

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
  getBalances: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 0,
    offset: 0,
  }),
}));

vi.mock('@/lib/transactions', async () => ({
  ...(await vi.importActual<object>('@/lib/transactions')),
  searchAccounts: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 20,
    offset: 0,
  }),
  listTransactions: vi.fn().mockResolvedValue({
    items: [],
    total_count: 0,
    limit: 20,
    offset: 0,
  }),
}));

describe('accounts create form', () => {
  it('submits a top-level account with type/currency/name', async () => {
    render(makeTestApp('/accounts/new'));

    await waitFor(() => expect(screen.getByText(/new account/i)).toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: /asset/i }));
    await userEvent.type(screen.getByLabelText(/^name$/i), 'NewBank');
    await userEvent.click(screen.getByRole('button', { name: /create/i }));

    await waitFor(() =>
      expect(createAccount).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'NewBank', type: 'A', currency: 'USD' }),
      ),
    );
  });

  it('rejects names containing a colon', async () => {
    render(makeTestApp('/accounts/new'));

    await waitFor(() => expect(screen.getByText(/new account/i)).toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: /asset/i }));
    await userEvent.type(screen.getByLabelText(/^name$/i), 'Assets:Bank');

    expect(screen.getByText(/should not contain/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create/i })).toBeDisabled();
  });
});
```

- [ ] **Step 2: Run and verify**

Run: `cd spa && npm test -- --run src/test/accounts.form.test.tsx`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add spa/src/test/accounts.form.test.tsx
git commit -m "test(spa): integration test for accounts create form"
```

---

## Phase G — Edit form

### Task 28: Wire `accounts.$id.edit.tsx` using `AccountForm`

**Files:**
- Modify: `spa/src/routes/accounts.$id.edit.tsx`

- [ ] **Step 1: Replace the placeholder**

Overwrite `spa/src/routes/accounts.$id.edit.tsx`:

```tsx
import { AccountForm } from '@/components/accounts/AccountForm';
import { SystemAccountBanner } from '@/components/accounts/SystemAccountBanner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getAccount, isOpeningBalancesAccount, updateAccount } from '@/lib/accounts';
import { ApiError } from '@/lib/api';
import type { UpdateAccountInput } from '@/lib/types';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { toast } from 'sonner';

export const Route = createFileRoute('/accounts/$id/edit')({
  component: EditAccountPage,
});

function EditAccountPage() {
  const { id: idParam } = Route.useParams();
  const id = Number(idParam);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [fieldErrors, setFieldErrors] = useState<Record<string, string | undefined>>({});
  const [formError, setFormError] = useState<string | undefined>(undefined);

  const accountQuery = useQuery({ queryKey: ['account', id], queryFn: () => getAccount(id) });

  const mutation = useMutation({
    mutationFn: (patch: UpdateAccountInput) => updateAccount(id, patch),
    onSuccess: (acc) => {
      qc.invalidateQueries({ queryKey: ['accounts'] });
      qc.invalidateQueries({ queryKey: ['account', id] });
      qc.invalidateQueries({ queryKey: ['balances'] });
      toast.success('Account updated');
      navigate({ to: '/accounts/$id', params: { id: String(acc.id) } });
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.field) {
          setFieldErrors({ [err.field]: err.message });
          setFormError(undefined);
        } else {
          setFormError(err.message);
        }
        return;
      }
      setFormError(err instanceof Error ? err.message : 'Save failed');
    },
  });

  if (accountQuery.isPending) return <Skeleton className="h-96 w-full" />;
  if (accountQuery.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load account</AlertTitle>
        <AlertDescription className="mt-2 space-y-3">
          <div>
            {accountQuery.error instanceof Error
              ? accountQuery.error.message
              : 'Unknown error'}
          </div>
          <Button onClick={() => accountQuery.refetch()} size="sm">
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  const account = accountQuery.data;
  const isSystem = isOpeningBalancesAccount(account.name);
  const segments = account.name.split(':');
  const parentName = segments.length > 1 ? segments.slice(0, -1).join(':') : undefined;

  return (
    <div>
      {isSystem && <SystemAccountBanner />}
      <AccountForm
        mode="edit"
        account={account}
        parentName={parentName}
        systemReadOnlyName={isSystem}
        pending={mutation.isPending}
        formError={formError}
        fieldErrors={fieldErrors}
        onSubmit={(patch) => {
          setFieldErrors({});
          setFormError(undefined);
          mutation.mutate(patch);
        }}
      />
    </div>
  );
}
```

- [ ] **Step 2: Verify typecheck + lint**

Run: `cd spa && npx tsc --noEmit && npm run lint`
Expected: no new errors.

- [ ] **Step 3: Smoke test**

Run dev server + backend. Navigate to `/accounts/$id/edit` for a leaf account. Change description; save. Detail page reflects the change. Navigate to `/accounts/$id/edit` for `Equity:OpeningBalances_USD`. Banner shows; Name field disabled; Description editable.

- [ ] **Step 4: Commit**

```bash
git add spa/src/routes/accounts.\$id.edit.tsx
git commit -m "feat(spa): wire account edit form with read-only immutable fields"
```

---

### Task 29: Edit-form integration test

**Files:**
- Modify: `spa/src/test/accounts.form.test.tsx`

Extend the existing test file with an edit-path case.

- [ ] **Step 1: Append the test**

At the bottom of `spa/src/test/accounts.form.test.tsx`, before the closing brace of the outer file, add a new `describe` block:

```tsx
import { updateAccount as updateAccountMock } from '@/lib/accounts';

const editLeaf = {
  id: 7,
  name: 'Assets:Bank:OldName',
  type: 'A' as const,
  parent_id: 2,
  currency: 'USD',
  description: 'old',
  is_hidden: false,
};

// Note: the original module mock for '@/lib/accounts' must also expose
// getAccount and updateAccount. If your mock object above does not include
// them, edit it to add:
//   getAccount: vi.fn().mockResolvedValue(editLeaf),
//   updateAccount: vi.fn().mockResolvedValue({ ...editLeaf, description: 'new' }),
// (Re-export them so the edit page can read them.)

describe('accounts edit form', () => {
  it('disables type/parent/currency fields and submits only changed fields', async () => {
    render(makeTestApp('/accounts/7/edit'));
    await waitFor(() => expect(screen.getByText(/edit account/i)).toBeInTheDocument());

    const desc = screen.getByLabelText(/description/i);
    await userEvent.clear(desc);
    await userEvent.type(desc, 'new');
    await userEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() =>
      expect(updateAccountMock).toHaveBeenCalledWith(7, { description: 'new' }),
    );
  });
});
```

- [ ] **Step 2: Update the top-of-file `vi.mock('@/lib/accounts', …)` block**

At the top of `spa/src/test/accounts.form.test.tsx`, extend the existing mock so it exposes `getAccount` and `updateAccount`:

```tsx
const updateAccount = vi.fn().mockImplementation((_id: number, patch: object) =>
  Promise.resolve({ ...editLeaf, ...patch }),
);
const getAccount = vi.fn().mockResolvedValue(editLeaf);

vi.mock('@/lib/accounts', async () => ({
  ...(await vi.importActual<object>('@/lib/accounts')),
  createAccount,
  updateAccount,
  getAccount,
  getAccountTree: vi.fn().mockResolvedValue([]),
}));
```

(Move the `editLeaf` literal above the `vi.mock` block — Vitest hoists `vi.mock` to the top of the module, but the factory body is allowed to capture variables declared with `const` above it as long as no other code runs first.)

- [ ] **Step 3: Run and verify**

Run: `cd spa && npm test -- --run src/test/accounts.form.test.tsx`
Expected: all PASS (create cases + edit case).

- [ ] **Step 4: Commit**

```bash
git add spa/src/test/accounts.form.test.tsx
git commit -m "test(spa): extend accounts form test with edit case"
```

---

## Phase H — Balances row becomes a link

### Task 30: Wrap `AccountListRow` in `<Link>`

**Files:**
- Modify: `spa/src/components/AccountListRow.tsx`

- [ ] **Step 1: Replace the component body**

Overwrite `spa/src/components/AccountListRow.tsx`:

```tsx
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import type { AccountBalance } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  row: AccountBalance;
}

export function AccountListRow({ row }: Props) {
  const negative = row.amount < 0;
  return (
    <Link
      to="/accounts/$id"
      params={{ id: String(row.account_id) }}
      className="flex items-center justify-between border-b border-border/60 px-2 py-2 text-sm hover:bg-muted/40"
    >
      <span>{row.name}</span>
      <span className={cn('tabular-nums', negative && 'text-destructive')}>
        {formatCents(row.amount, row.currency)}
      </span>
    </Link>
  );
}
```

- [ ] **Step 2: Verify typecheck + lint**

Run: `cd spa && npx tsc --noEmit && npm run lint`
Expected: no new errors. If the existing `balances.test.tsx` asserted the row was a `div`, update that assertion too (re-run tests in step 3 first to see what breaks).

- [ ] **Step 3: Run existing balance tests**

Run: `cd spa && npm test -- --run src/test/balances.test.tsx`
Expected: PASS. If failures point at element-type assertions on the row, update them to expect a link.

- [ ] **Step 4: Commit**

```bash
git add spa/src/components/AccountListRow.tsx spa/src/test/balances.test.tsx
git commit -m "feat(spa): make Balances rows clickable into account detail"
```

---

### Task 31: Balances-link integration test

**Files:**
- Create: `spa/src/test/balances.link.test.tsx`

- [ ] **Step 1: Write the test**

Create `spa/src/test/balances.link.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { makeTestApp } from './test-app';

vi.mock('@/lib/api', async () => ({
  ...(await vi.importActual<object>('@/lib/api')),
  getConfig: vi.fn().mockResolvedValue({ defaults: { currency: 'USD' } }),
  getBalances: vi.fn().mockResolvedValue({
    items: [
      {
        account_id: 3,
        name: 'Assets:Bank:Checking',
        type: 'A',
        currency: 'USD',
        amount: 3200,
        is_hidden: false,
      },
    ],
    total_count: 1,
    limit: 0,
    offset: 0,
  }),
}));

describe('balances row link', () => {
  it('renders each row as a link to its account detail', async () => {
    render(makeTestApp('/balances'));
    const link = await waitFor(() =>
      screen.getByRole('link', { name: /Assets:Bank:Checking/ }),
    );
    expect(link).toHaveAttribute('href', '/accounts/3');
  });
});
```

- [ ] **Step 2: Run and verify**

Run: `cd spa && npm test -- --run src/test/balances.link.test.tsx`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add spa/src/test/balances.link.test.tsx
git commit -m "test(spa): integration test for balances row link"
```

---

## Phase I — Final manual verification + PR prep

### Task 32: Full manual verification pass

**Files:** none (manual)

Run the full checklist from the spec. Anything that fails goes back to the relevant task to fix.

- [ ] **Step 1: Start backend + frontend**

```bash
go run ./cmd/kea serve &
cd spa && npm run dev
```

- [ ] **Step 2: Walk through the checklist**

For each, verify behavior in the browser (note pass/fail beside each line):

  - [ ] Navigate to `/accounts` — tree renders; expand/collapse works.
  - [ ] Toggle "Show hidden" — hidden accounts appear with muted styling.
  - [ ] Type "bank" into the search box — flat results render; clear restores tree.
  - [ ] `+ New account` → parent `Assets:Bank`, name `Checking2`, opening balance `1000` → submit → land on `/accounts/$id`.
  - [ ] Detail page (leaf) shows recent-transactions card with the synthesized opening transaction; "View all" navigates to `/transactions?account_id=$id`.
  - [ ] Edit the new account → change description, toggle hidden → save → tree reflects (muted with "hidden" tag once "Show hidden" is on).
  - [ ] Detail page on `Assets:Bank` (parent) → child accounts card shows; Delete button disabled with the "has N children" tooltip.
  - [ ] Delete `Checking2` → goes back to `/accounts`; tree no longer shows it.
  - [ ] Navigate to `Equity:OpeningBalances_USD` → system banner present, Delete absent, Edit page shows Name field read-only.
  - [ ] Click a Balances row → lands on `/accounts/$id`.
  - [ ] Toggle the ledger via the sidebar switcher → `/accounts` refetches against the new ledger.
  - [ ] Refresh on `/accounts?q=bank&type=A&include_hidden=true` → flat search mode restored with filters intact.

- [ ] **Step 3: Stop servers**

Kill the dev server (Ctrl-C in its terminal) and the backend (`kill %1` or `fg` + Ctrl-C).

- [ ] **Step 4: Run the full test suite**

```bash
cd spa && npm run lint && npm test -- --run
```
Expected: lint clean; all tests pass.

- [ ] **Step 5: Run Go tests for safety**

```bash
go test ./...
```
Expected: PASS (we haven't touched Go, but verify the existing suite still passes).

- [ ] **Step 6: Commit anything outstanding**

If lint or tests required edits, commit them:

```bash
git status
git add <files>
git commit -m "fix(spa): <what>"
```

---

### Task 33: Open the PR

- [ ] **Step 1: Verify branch + push**

```bash
git status
git log --oneline master..HEAD
git push -u origin HEAD
```

- [ ] **Step 2: Open the PR via gh**

```bash
gh pr create --title "feat(spa): accounts route with tree, search, CRUD, system-account lockout" \
  --body "$(cat <<'EOF'
## Summary
- Adds `/accounts` (tree view with type + show-hidden filters; search switches to flat results), `/accounts/new`, `/accounts/$id` (adaptive leaf/parent), `/accounts/$id/edit`.
- Implements the system-account banner / blocked-delete tooltip / immutable-field display-row patterns from the spec.
- Makes Balances rows clickable into account detail.
- No backend changes — all endpoints already exist.

## Test plan
- [ ] `cd spa && npm test -- --run` — all green
- [ ] `cd spa && npm run lint` — clean
- [ ] `go test ./...` — green
- [ ] Manual checklist from the design spec walked end-to-end (see plan task 32)

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 3: Report PR URL back to user**

---

## Self-Review

**Spec coverage** — every section in the spec has at least one corresponding task:

| Spec section | Task |
|---|---|
| Routes & File Layout | Tasks 4, 5 |
| Search params | Task 3 |
| Component Split | Tasks 6, 7, 8, 11, 14, 15, 16, 17, 18, 22, 23, 24, 25 |
| API Layer (`accounts.ts`) | Task 2 |
| `types.ts` additions | Task 1 |
| TanStack Query keys + invalidations | Tasks 9, 12, 19, 26, 28 |
| Filter state in URL | Tasks 8, 9 |
| Form state + field error mapping | Tasks 25, 26, 28 |
| Constraint & Lockout UI (system banner, blocked delete, immutable display rows) | Tasks 14, 18, 19, 25, 28 |
| Tree view layout + balance join | Tasks 6, 7, 9 |
| Search results layout + 100-row cap | Tasks 11, 12 |
| Detail page (leaf vs. parent) | Tasks 15, 16, 17, 19 |
| Create form layout (parent picker, type, opening balance) | Tasks 22, 23, 24, 25, 26 |
| Edit form layout (immutable display rows, system handling) | Tasks 25, 28 |
| Sidebar + Balances link updates | Tasks 4, 30 |
| Testing (unit + component) | Tasks 2, 3, 10, 13, 20, 21, 27, 29, 31 |
| Manual verification checklist | Task 32 |

**Placeholder scan** — none. All code blocks are complete; commit messages are concrete; no `TBD`s or `similar to Task N`.

**Type consistency** — `Account`, `AccountNode`, `CreateAccountInput`, `UpdateAccountInput`, `BalanceResponse` are introduced in Task 1 and used consistently. `isOpeningBalancesAccount` (Task 2) is invoked in Tasks 19 and 28 with the same name. `AccountsSearch` (Task 3) is consumed by Tasks 8, 9, 12.

---

Plan complete and saved to `docs/superpowers/plans/2026-06-10-spa-accounts.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
