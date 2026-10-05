# SPA Transactions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable the Transactions sidebar item with list, view, create, edit, and delete against the existing `/api/transactions` endpoints — the second built-out SPA route after Balances.

**Architecture:** TanStack Router file-based routes under `spa/src/routes/transactions.*`. URL search params are the source of truth for filter/pagination state (validated via zod). TanStack Query for data, react-hook-form for the create/edit form's dynamic splits array. Two TS ports of Go logic (`determineType`, `transactionDisplay`) keep client UX hints in sync with the server's classification.

**Tech Stack:** React 18 + TypeScript + TanStack Router + TanStack Query + Tailwind + shadcn/ui primitives. New deps: `react-hook-form`, `zod`.

**Spec:** [`docs/superpowers/specs/2026-06-10-spa-transactions-design.md`](../specs/2026-06-10-spa-transactions-design.md)

**Working directory for all commands:** `/Users/hance/programming/kea/spa` unless noted.

---

## Conventions used in this plan

- All file paths are absolute under `/Users/hance/programming/kea/`.
- `npm` commands run from `/Users/hance/programming/kea/spa/`.
- `go test` commands run from `/Users/hance/programming/kea/`.
- Test-first wherever the unit is pure logic or a presentational component. For routes that wire many pieces, write the integration test as the final step of the route's task.
- Commit at the end of every task. Use Conventional Commits prefixes (`feat(spa):`, `test(spa):`, `chore(spa):`).
- Follow the existing SPA test convention: stub `fetch` via `vi.stubGlobal('fetch', ...)`. Do not use `vi.mock('@/lib/api')` — fetch-stubbing matches the established pattern in `spa/src/test/balances.test.tsx`.

---

## Task 1: Install dependencies

**Files:**
- Modify: `spa/package.json`
- Modify: `spa/package-lock.json` (auto)

- [ ] **Step 1: Install react-hook-form and zod**

Run from `/Users/hance/programming/kea/spa`:
```bash
npm install react-hook-form zod
```

Expected: both packages added to `dependencies`; lockfile updated.

- [ ] **Step 2: Verify install**

Run:
```bash
npm ls react-hook-form zod
```

Expected: both listed with resolved versions; no `UNMET DEPENDENCY` warnings.

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/package.json spa/package-lock.json
git commit -m "chore(spa): add react-hook-form and zod for transactions form"
```

---

## Task 2: Extend types and export apiFetch

**Files:**
- Modify: `spa/src/lib/types.ts`
- Modify: `spa/src/lib/api.ts`

- [ ] **Step 1: Add transaction types to `spa/src/lib/types.ts`**

Append the following to the end of `spa/src/lib/types.ts`:

```ts
export type TransactionType =
  | 'Expense'
  | 'Income'
  | 'Transfer'
  | 'Opening'
  | 'Deposit'
  | 'Withdrawal'
  | 'Other';

export type TransactionStatus = 'Pending' | 'Cleared' | 'Reconciled';

export interface SplitDetail {
  id: number;
  account_id: number;
  account_name: string;
  account_type: AccountType;
  amount: number; // int64 cents, signed
  currency: string;
  memo: string;
}

export interface TransactionDetail {
  id: number;
  timestamp: number; // Unix seconds
  description: string;
  status: TransactionStatus;
  type: TransactionType;
  splits: SplitDetail[];
}

export interface SplitInput {
  id?: number;
  account_name: string;
  amount: number;
  currency: string;
  memo?: string;
}

export interface CreateTransactionInput {
  splits: SplitInput[];
  description: string;
  timestamp: number;
  status: TransactionStatus;
  type?: TransactionType;
}

export interface UpdateTransactionInput {
  id: number;
  description: string;
  timestamp: number;
  status: TransactionStatus;
  type?: TransactionType;
  splits: SplitInput[];
}

export interface TransactionFilter {
  account_id?: number;
  type?: TransactionType;
  status?: TransactionStatus;
  start_time?: number;
  end_time?: number;
  description?: string;
}

export interface Account {
  id: number;
  name: string;
  type: AccountType;
  parent_id?: number;
  currency: string;
  description: string;
  is_hidden: boolean;
}
```

- [ ] **Step 2: Export `apiFetch` from `spa/src/lib/api.ts`**

In `spa/src/lib/api.ts`, change the line:
```ts
async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
```
to:
```ts
export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
```

- [ ] **Step 3: Type-check**

Run from `spa/`:
```bash
npm run build
```

Expected: success; no TypeScript errors. (This compiles the whole tree.)

- [ ] **Step 4: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/lib/types.ts spa/src/lib/api.ts
git commit -m "feat(spa): add transaction domain types and export apiFetch"
```

---

## Task 3: Transactions API client

**Files:**
- Create: `spa/src/lib/transactions.ts`

- [ ] **Step 1: Create `spa/src/lib/transactions.ts`**

```ts
import { apiFetch } from './api';
import type {
  Account,
  CreateTransactionInput,
  ListResult,
  TransactionDetail,
  TransactionFilter,
  TransactionStatus,
  UpdateTransactionInput,
} from './types';

interface ListOptions {
  limit?: number;
  offset?: number;
  include_count?: boolean;
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

export function listTransactions(
  filter: TransactionFilter,
  opts: ListOptions = {},
): Promise<ListResult<TransactionDetail>> {
  const q = buildQuery({
    account_id: filter.account_id,
    type: filter.type,
    status: filter.status,
    start_time: filter.start_time,
    end_time: filter.end_time,
    description: filter.description,
    limit: opts.limit,
    offset: opts.offset,
    include_count: opts.include_count ?? true,
  });
  return apiFetch<ListResult<TransactionDetail>>(`/api/transactions${q}`);
}

export function getTransaction(id: number): Promise<TransactionDetail> {
  return apiFetch<TransactionDetail>(`/api/transactions/${id}`);
}

export function createTransaction(input: CreateTransactionInput): Promise<TransactionDetail> {
  return apiFetch<TransactionDetail>('/api/transactions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
}

export function updateTransaction(input: UpdateTransactionInput): Promise<TransactionDetail> {
  return apiFetch<TransactionDetail>(`/api/transactions/${input.id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
}

export function updateTransactionStatus(
  id: number,
  status: TransactionStatus,
): Promise<TransactionDetail> {
  return apiFetch<TransactionDetail>(`/api/transactions/${id}/status`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  });
}

export function deleteTransaction(id: number): Promise<{ deleted: boolean; id: number }> {
  return apiFetch<{ deleted: boolean; id: number }>(`/api/transactions/${id}`, {
    method: 'DELETE',
  });
}

export function listAccounts(): Promise<ListResult<Account>> {
  return apiFetch<ListResult<Account>>('/api/accounts?include_hidden=false');
}

export function searchAccounts(query: string): Promise<ListResult<Account>> {
  const q = buildQuery({ q: query, limit: 20 });
  return apiFetch<ListResult<Account>>(`/api/accounts${q}`);
}
```

- [ ] **Step 2: Verify HTTP method for update**

Run from `/Users/hance/programming/kea`:
```bash
grep -n 'Method\|Put\|"PUT"' internal/api/router.go | grep -i transaction
```

Expected: confirms that `PUT /api/transactions/{id}` is the route for `handleUpdateTransaction`. If the actual verb differs (`PATCH`), update the method in `updateTransaction` above to match. **Do not proceed past this step without confirming.**

- [ ] **Step 3: Type-check**

Run from `spa/`:
```bash
npm run build
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/lib/transactions.ts
git commit -m "feat(spa): add transactions and accounts API client"
```

---

## Task 4: Port `determineType` (TDD)

**Files:**
- Create: `spa/src/lib/determineType.ts`
- Create: `spa/src/test/lib/determineType.test.ts`

**Reference:** Go source `internal/service/transaction_classifier.go:15-104` (`DetermineType` function) and tests `internal/service/transaction_classifier_test.go::TestDetermineType`.

- [ ] **Step 1: Write the failing test at `spa/src/test/lib/determineType.test.ts`**

```ts
import { describe, expect, test } from 'vitest';
import { determineType } from '@/lib/determineType';
import type { SplitDetail } from '@/lib/types';

const sp = (
  account_name: string,
  account_type: SplitDetail['account_type'],
  amount: number,
  memo = '',
): SplitDetail => ({
  id: 0,
  account_id: 0,
  account_name,
  account_type,
  amount,
  currency: 'USD',
  memo,
});

describe('determineType', () => {
  test('empty splits → Other', () => {
    expect(determineType([])).toBe('Other');
  });

  test('Expense: E + A', () => {
    expect(
      determineType([sp('Expenses:Food', 'E', 500), sp('Assets:Bank', 'A', -500)]),
    ).toBe('Expense');
  });

  test('Income: R + A (positive total revenue dominates)', () => {
    expect(
      determineType([sp('Revenue:Salary', 'R', -3000), sp('Assets:Bank', 'A', 3000)]),
    ).toBe('Income');
  });

  test('Transfer: A + A', () => {
    expect(
      determineType([sp('Assets:Savings', 'A', 1000), sp('Assets:Checking', 'A', -1000)]),
    ).toBe('Transfer');
  });

  test('Transfer: A + L', () => {
    expect(
      determineType([sp('Assets:Bank', 'A', 1000), sp('Liabilities:Card', 'L', -1000)]),
    ).toBe('Transfer');
  });

  test('Income + Expense where revenue > expense → Income', () => {
    expect(
      determineType([
        sp('Revenue:Salary', 'R', -5000),
        sp('Expenses:Tax', 'E', 1000),
        sp('Assets:Bank', 'A', 4000),
      ]),
    ).toBe('Income');
  });

  test('Income + Expense where expense > revenue → Expense', () => {
    expect(
      determineType([
        sp('Revenue:Bonus', 'R', -500),
        sp('Expenses:Tax', 'E', 1000),
        sp('Assets:Bank', 'A', -500),
      ]),
    ).toBe('Expense');
  });

  test('Expense with 2 Asset/Liab and asset increase greater than expense → Transfer', () => {
    expect(
      determineType([
        sp('Assets:Bank', 'A', 1000),
        sp('Assets:Cash', 'A', -1500),
        sp('Expenses:Fees:Stocks', 'E', 500),
      ]),
    ).toBe('Transfer');
  });

  test('Expense with single Asset → Expense', () => {
    expect(
      determineType([sp('Assets:Cash', 'A', -50), sp('Expenses:Food', 'E', 50)]),
    ).toBe('Expense');
  });

  test('Equity + asset increase → Deposit', () => {
    expect(
      determineType([sp('Equity:Retained', 'C', -100), sp('Assets:Bank', 'A', 100)]),
    ).toBe('Deposit');
  });

  test('Equity + asset decrease → Withdrawal', () => {
    expect(
      determineType([sp('Equity:Retained', 'C', 100), sp('Assets:Bank', 'A', -100)]),
    ).toBe('Withdrawal');
  });

  test('Opening memo override → Opening', () => {
    expect(
      determineType([
        sp('Assets:Bank', 'A', 1000, 'Opening Balance'),
        sp('Equity:OpeningBalances_USD', 'C', -1000),
      ]),
    ).toBe('Opening');
  });
});
```

- [ ] **Step 2: Confirm the actual opening memo constant**

Run from `/Users/hance/programming/kea`:
```bash
grep -n 'OpeningAccountMemo' internal/model/*.go
```

Expected: shows the exact string literal (e.g., `"Opening Balance"`). If it differs from the value in the test above, update the test.

- [ ] **Step 3: Run test and verify it fails**

Run from `spa/`:
```bash
npm test -- determineType
```

Expected: FAIL with "Cannot find module '@/lib/determineType'".

- [ ] **Step 4: Implement `spa/src/lib/determineType.ts`**

```ts
import type { SplitDetail, TransactionType } from './types';

const OPENING_MEMO = 'Opening Balance'; // mirrors model.OpeningAccountMemo

export function determineType(splits: SplitDetail[]): TransactionType {
  if (splits.length === 0) return 'Other';

  let totalRevenueAmount = 0;
  let totalExpenseAmount = 0;
  let totalPositiveAssetLiabAmount = 0;

  let hasExpense = false;
  let hasRevenue = false;
  let hasEquity = false;
  let assetOrLiabCnt = 0;
  let isOpening = false;
  let isAssetIncrease = false;

  for (const s of splits) {
    if (s.memo === OPENING_MEMO) isOpening = true;
    switch (s.account_type) {
      case 'E':
        hasExpense = true;
        totalExpenseAmount += Math.abs(s.amount);
        break;
      case 'R':
        hasRevenue = true;
        totalRevenueAmount += Math.abs(s.amount);
        break;
      case 'A':
        assetOrLiabCnt++;
        if (s.amount > 0) {
          isAssetIncrease = true;
          totalPositiveAssetLiabAmount += s.amount;
        }
        break;
      case 'L':
        assetOrLiabCnt++;
        if (s.amount > 0) totalPositiveAssetLiabAmount += s.amount;
        break;
      case 'C':
        hasEquity = true;
        break;
    }
  }

  if (isOpening) return 'Opening';

  if (hasExpense && hasRevenue) {
    return totalRevenueAmount >= totalExpenseAmount ? 'Income' : 'Expense';
  }

  if (hasExpense && assetOrLiabCnt >= 2) {
    return totalPositiveAssetLiabAmount > totalExpenseAmount ? 'Transfer' : 'Expense';
  }
  if (hasExpense && assetOrLiabCnt === 1) return 'Expense';

  if (hasRevenue && assetOrLiabCnt >= 1) return 'Income';

  if (assetOrLiabCnt >= 2) return 'Transfer';

  if (hasEquity && assetOrLiabCnt >= 1) {
    return isAssetIncrease ? 'Deposit' : 'Withdrawal';
  }

  return 'Other';
}
```

- [ ] **Step 5: Run test and verify it passes**

```bash
npm test -- determineType
```

Expected: PASS (12 tests).

- [ ] **Step 6: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/lib/determineType.ts spa/src/test/lib/determineType.test.ts
git commit -m "feat(spa): port determineType from service layer to TS"
```

---

## Task 5: Port `transactionDisplay` (TDD)

**Files:**
- Create: `spa/src/lib/transactionDisplay.ts`
- Create: `spa/src/test/lib/transactionDisplay.test.ts`

**Reference:** `internal/service/transaction_classifier.go:106-186` for `GetDisplayAccount`, `GetDisplayAmount`, `GetDisplayOffsetAccount`.

- [ ] **Step 1: Write failing test at `spa/src/test/lib/transactionDisplay.test.ts`**

```ts
import { describe, expect, test } from 'vitest';
import {
  displayAccount,
  displayAmount,
  displayOffsetAccount,
} from '@/lib/transactionDisplay';
import type { SplitDetail } from '@/lib/types';

const sp = (
  account_name: string,
  account_type: SplitDetail['account_type'],
  amount: number,
): SplitDetail => ({
  id: 0,
  account_id: 0,
  account_name,
  account_type,
  amount,
  currency: 'USD',
  memo: '',
});

describe('displayAccount', () => {
  test('Expense → returns the Expense account', () => {
    const splits = [sp('Assets:Bank', 'A', -500), sp('Expenses:Food', 'E', 500)];
    expect(displayAccount(splits, 'Expense')).toBe('Expenses:Food');
  });

  test('Income → returns the Revenue account', () => {
    const splits = [sp('Assets:Bank', 'A', 3000), sp('Revenue:Salary', 'R', -3000)];
    expect(displayAccount(splits, 'Income')).toBe('Revenue:Salary');
  });

  test('Transfer → returns the positive Asset/Liability account', () => {
    const splits = [sp('Assets:Checking', 'A', -1000), sp('Assets:Savings', 'A', 1000)];
    expect(displayAccount(splits, 'Transfer')).toBe('Assets:Savings');
  });

  test('Opening → returns the non-equity account', () => {
    const splits = [
      sp('Equity:OpeningBalances_USD', 'C', -1000),
      sp('Assets:Bank', 'A', 1000),
    ];
    expect(displayAccount(splits, 'Opening')).toBe('Assets:Bank');
  });

  test('Other → returns the first positive amount account', () => {
    const splits = [sp('Assets:A', 'A', -100), sp('Assets:B', 'A', 100)];
    expect(displayAccount(splits, 'Other')).toBe('Assets:B');
  });

  test('empty splits → "-"', () => {
    expect(displayAccount([], 'Expense')).toBe('-');
  });
});

describe('displayOffsetAccount', () => {
  test('Expense with one offset → returns the Asset account', () => {
    const splits = [sp('Assets:Bank', 'A', -500), sp('Expenses:Food', 'E', 500)];
    expect(displayOffsetAccount(splits, 'Expense', 'Expenses:Food')).toBe('Assets:Bank');
  });

  test('Income with one offset → returns the Asset account', () => {
    const splits = [sp('Assets:Bank', 'A', 3000), sp('Revenue:Salary', 'R', -3000)];
    expect(displayOffsetAccount(splits, 'Income', 'Revenue:Salary')).toBe('Assets:Bank');
  });

  test('Transfer with one offset → returns the negative account', () => {
    const splits = [sp('Assets:Checking', 'A', -1000), sp('Assets:Savings', 'A', 1000)];
    expect(displayOffsetAccount(splits, 'Transfer', 'Assets:Savings')).toBe('Assets:Checking');
  });

  test('multiple offsets → returns "(multiple)"', () => {
    const splits = [
      sp('Assets:Cash', 'A', -100),
      sp('Expenses:Food', 'E', 60),
      sp('Expenses:Household', 'E', 40),
    ];
    // Primary is one of the Expenses, two distinct offsets remain
    expect(displayOffsetAccount(splits, 'Expense', 'Expenses:Food')).toBe('(multiple)');
  });

  test('empty splits → "-"', () => {
    expect(displayOffsetAccount([], 'Expense', 'x')).toBe('-');
  });
});

describe('displayAmount', () => {
  test('Expense → negative signed amount', () => {
    const splits = [sp('Assets:Bank', 'A', -500), sp('Expenses:Food', 'E', 500)];
    expect(displayAmount(splits, 'Expense')).toEqual({ amount: -500, currency: 'USD' });
  });

  test('Income → positive signed amount', () => {
    const splits = [sp('Assets:Bank', 'A', 3000), sp('Revenue:Salary', 'R', -3000)];
    expect(displayAmount(splits, 'Income')).toEqual({ amount: 3000, currency: 'USD' });
  });

  test('Transfer → absolute positive amount', () => {
    const splits = [sp('Assets:Checking', 'A', -1000), sp('Assets:Savings', 'A', 1000)];
    expect(displayAmount(splits, 'Transfer')).toEqual({ amount: 1000, currency: 'USD' });
  });

  test('Other → max positive amount (matches CLI behavior)', () => {
    const splits = [sp('Assets:A', 'A', -200), sp('Assets:B', 'A', 200)];
    expect(displayAmount(splits, 'Other')).toEqual({ amount: 200, currency: 'USD' });
  });

  test('empty splits → 0 with empty currency', () => {
    expect(displayAmount([], 'Expense')).toEqual({ amount: 0, currency: '' });
  });
});
```

- [ ] **Step 2: Run test and verify it fails**

```bash
npm test -- transactionDisplay
```

Expected: FAIL with "Cannot find module '@/lib/transactionDisplay'".

- [ ] **Step 3: Implement `spa/src/lib/transactionDisplay.ts`**

```ts
import type { SplitDetail, TransactionType } from './types';

export function displayAccount(splits: SplitDetail[], type: TransactionType | string): string {
  if (splits.length === 0) return '-';

  switch (type) {
    case 'Expense':
      for (const s of splits) if (s.account_type === 'E') return s.account_name;
      break;
    case 'Income':
      for (const s of splits) if (s.account_type === 'R') return s.account_name;
      break;
    case 'Transfer':
      for (const s of splits) {
        if (s.amount > 0 && (s.account_type === 'A' || s.account_type === 'L')) {
          return s.account_name;
        }
      }
      break;
    case 'Opening':
      for (const s of splits) if (s.account_type !== 'C') return s.account_name;
      break;
    case 'Other':
      for (const s of splits) if (s.amount > 0) return s.account_name;
      break;
  }
  return splits[0]?.account_name ?? '-';
}

export function displayOffsetAccount(
  splits: SplitDetail[],
  type: TransactionType | string,
  primaryAccount: string,
): string {
  if (splits.length === 0) return '-';

  const seen = new Set<string>();
  const primaryType =
    type === 'Expense' ? 'E' : type === 'Income' ? 'R' : null;

  if (primaryType !== null) {
    for (const s of splits) {
      if (s.account_type !== primaryType) seen.add(s.account_name);
    }
  } else {
    for (const s of splits) {
      if (s.account_name !== primaryAccount) seen.add(s.account_name);
    }
  }

  if (seen.size === 0) return '-';
  if (seen.size === 1) return seen.values().next().value as string;
  return '(multiple)';
}

export function displayAmount(
  splits: SplitDetail[],
  type: TransactionType | string,
): { amount: number; currency: string } {
  if (splits.length === 0) return { amount: 0, currency: '' };

  const currency = splits[0].currency;

  switch (type) {
    case 'Expense': {
      const ex = splits.find((s) => s.account_type === 'E');
      if (ex) return { amount: -Math.abs(ex.amount), currency: ex.currency };
      break;
    }
    case 'Income': {
      const rv = splits.find((s) => s.account_type === 'R');
      if (rv) return { amount: Math.abs(rv.amount), currency: rv.currency };
      break;
    }
    case 'Transfer': {
      const positive = splits.find(
        (s) => s.amount > 0 && (s.account_type === 'A' || s.account_type === 'L'),
      );
      if (positive) return { amount: positive.amount, currency: positive.currency };
      break;
    }
  }

  // Fallback (Other / Opening / Deposit / Withdrawal): max positive amount + its currency
  let maxAmount = 0;
  let chosenCurrency = currency;
  for (const s of splits) {
    if (s.amount > maxAmount) {
      maxAmount = s.amount;
      chosenCurrency = s.currency;
    }
  }
  return { amount: maxAmount, currency: chosenCurrency };
}
```

- [ ] **Step 4: Run test and verify it passes**

```bash
npm test -- transactionDisplay
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/lib/transactionDisplay.ts spa/src/test/lib/transactionDisplay.test.ts
git commit -m "feat(spa): port transactionDisplay helpers (account, offset, amount)"
```

---

## Task 6: Search params schema (TDD)

**Files:**
- Create: `spa/src/lib/transactions-search-params.ts`
- Create: `spa/src/test/lib/transactions-search-params.test.ts`

- [ ] **Step 1: Write failing test**

```ts
import { describe, expect, test } from 'vitest';
import {
  parseTransactionsSearch,
  searchToFilter,
  searchToListOptions,
  transactionsSearchSchema,
} from '@/lib/transactions-search-params';

describe('transactionsSearchSchema', () => {
  test('all empty → defaults to limit=50 offset=0', () => {
    expect(transactionsSearchSchema.parse({})).toEqual({ limit: 50, offset: 0 });
  });

  test('valid full search', () => {
    expect(
      transactionsSearchSchema.parse({
        account_id: '3',
        type: 'Expense',
        status: 'Cleared',
        start_time: '1700000000',
        end_time: '1733000000',
        description: 'coffee',
        limit: '20',
        offset: '40',
      }),
    ).toEqual({
      account_id: 3,
      type: 'Expense',
      status: 'Cleared',
      start_time: 1700000000,
      end_time: 1733000000,
      description: 'coffee',
      limit: 20,
      offset: 40,
    });
  });

  test('invalid type rejected', () => {
    expect(() => transactionsSearchSchema.parse({ type: 'Bogus' })).toThrow();
  });

  test('negative offset rejected', () => {
    expect(() => transactionsSearchSchema.parse({ offset: '-1' })).toThrow();
  });

  test('zero limit rejected (must be positive)', () => {
    expect(() => transactionsSearchSchema.parse({ limit: '0' })).toThrow();
  });
});

describe('searchToFilter / searchToListOptions', () => {
  test('extracts filter fields only', () => {
    const search = transactionsSearchSchema.parse({
      account_id: '3',
      type: 'Income',
      limit: '10',
      offset: '20',
    });
    expect(searchToFilter(search)).toEqual({ account_id: 3, type: 'Income' });
    expect(searchToListOptions(search)).toEqual({
      limit: 10,
      offset: 20,
      include_count: true,
    });
  });
});

describe('parseTransactionsSearch (lenient wrapper)', () => {
  test('returns defaults when input fails validation entirely', () => {
    expect(parseTransactionsSearch({ type: 'Bogus' })).toEqual({ limit: 50, offset: 0 });
  });
});
```

- [ ] **Step 2: Run test and verify it fails**

```bash
npm test -- transactions-search-params
```

Expected: FAIL with module-not-found.

- [ ] **Step 3: Implement `spa/src/lib/transactions-search-params.ts`**

```ts
import { z } from 'zod';
import type { TransactionFilter } from './types';

export const transactionsSearchSchema = z.object({
  account_id: z.coerce.number().int().positive().optional(),
  type: z
    .enum(['Expense', 'Income', 'Transfer', 'Opening', 'Deposit', 'Withdrawal', 'Other'])
    .optional(),
  status: z.enum(['Pending', 'Cleared', 'Reconciled']).optional(),
  start_time: z.coerce.number().int().optional(),
  end_time: z.coerce.number().int().optional(),
  description: z.string().min(1).optional(),
  limit: z.coerce.number().int().positive().default(50),
  offset: z.coerce.number().int().nonnegative().default(0),
});

export type TransactionsSearch = z.infer<typeof transactionsSearchSchema>;

export function parseTransactionsSearch(input: unknown): TransactionsSearch {
  const result = transactionsSearchSchema.safeParse(input);
  if (result.success) return result.data;
  return transactionsSearchSchema.parse({});
}

export function searchToFilter(search: TransactionsSearch): TransactionFilter {
  const f: TransactionFilter = {};
  if (search.account_id !== undefined) f.account_id = search.account_id;
  if (search.type !== undefined) f.type = search.type;
  if (search.status !== undefined) f.status = search.status;
  if (search.start_time !== undefined) f.start_time = search.start_time;
  if (search.end_time !== undefined) f.end_time = search.end_time;
  if (search.description !== undefined) f.description = search.description;
  return f;
}

export function searchToListOptions(search: TransactionsSearch) {
  return { limit: search.limit, offset: search.offset, include_count: true };
}
```

- [ ] **Step 4: Run test and verify it passes**

```bash
npm test -- transactions-search-params
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/lib/transactions-search-params.ts spa/src/test/lib/transactions-search-params.test.ts
git commit -m "feat(spa): add transactions search params zod schema"
```

---

## Task 7: Atomic display components (TypeBadge, StatusText, ReconciledBanner)

**Files:**
- Create: `spa/src/components/transactions/TypeBadge.tsx`
- Create: `spa/src/components/transactions/StatusText.tsx`
- Create: `spa/src/components/transactions/ReconciledBanner.tsx`

- [ ] **Step 1: Create the transactions component directory**

```bash
mkdir -p /Users/hance/programming/kea/spa/src/components/transactions
```

- [ ] **Step 2: Create `TypeBadge.tsx`**

```tsx
import { cn } from '@/lib/cn';
import type { TransactionType } from '@/lib/types';

const TYPE_CLASSES: Record<TransactionType, string> = {
  Expense: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200',
  Income: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200',
  Transfer: 'bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-200',
  Opening: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200',
  Deposit: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200',
  Withdrawal: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200',
  Other: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200',
};

interface Props {
  type: TransactionType;
  className?: string;
}

export function TypeBadge({ type, className }: Props) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium',
        TYPE_CLASSES[type],
        className,
      )}
    >
      {type}
    </span>
  );
}
```

- [ ] **Step 3: Create `StatusText.tsx`**

```tsx
import type { TransactionStatus } from '@/lib/types';

interface Props {
  status: TransactionStatus;
}

// Plain text only — no color, no emoji (design decision).
export function StatusText({ status }: Props) {
  return <span className="text-xs text-foreground">{status}</span>;
}
```

- [ ] **Step 4: Create `ReconciledBanner.tsx`**

```tsx
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';

export function ReconciledBanner() {
  return (
    <Alert>
      <AlertTitle>This transaction is reconciled</AlertTitle>
      <AlertDescription>
        Reconciled transactions are locked. To edit or delete this transaction, unreconcile
        it from the Reconcile screen first.
      </AlertDescription>
    </Alert>
  );
}
```

- [ ] **Step 5: Type-check**

```bash
cd /Users/hance/programming/kea/spa
npm run build
```

Expected: success.

- [ ] **Step 6: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/components/transactions/
git commit -m "feat(spa): add TypeBadge, StatusText, ReconciledBanner"
```

---

## Task 8: AccountCombobox

**Files:**
- Create: `spa/src/components/transactions/AccountCombobox.tsx`

A minimal account picker: typed input + dropdown of matches from `searchAccounts`. No external combobox library — uses native HTML + state.

- [ ] **Step 1: Create `AccountCombobox.tsx`**

```tsx
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/cn';
import { searchAccounts } from '@/lib/transactions';
import type { Account } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useId, useRef, useState } from 'react';

interface Props {
  value: string;             // account name (the canonical identifier in API inputs)
  onChange: (name: string, account?: Account) => void;
  placeholder?: string;
  // Restrict suggestions to these account types (e.g., ['A','L'] for Transfer).
  allowedTypes?: Account['type'][];
  disabled?: boolean;
  id?: string;
  'aria-invalid'?: boolean;
}

export function AccountCombobox({
  value,
  onChange,
  placeholder = 'Account…',
  allowedTypes,
  disabled,
  id,
  ...aria
}: Props) {
  const reactId = useId();
  const inputId = id ?? `acc-${reactId}`;
  const [query, setQuery] = useState(value);
  const [open, setOpen] = useState(false);
  const [debounced, setDebounced] = useState(query);
  const containerRef = useRef<HTMLDivElement>(null);

  // Keep input in sync if parent value changes (e.g., form reset).
  useEffect(() => {
    setQuery(value);
  }, [value]);

  // Debounce search query.
  useEffect(() => {
    const t = setTimeout(() => setDebounced(query), 200);
    return () => clearTimeout(t);
  }, [query]);

  // Close dropdown on outside click.
  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (!containerRef.current?.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, []);

  const enabled = open && debounced.length > 0;
  const search = useQuery({
    queryKey: ['accounts', 'search', debounced],
    queryFn: () => searchAccounts(debounced),
    enabled,
    staleTime: 30_000,
  });

  const allItems = search.data?.items ?? [];
  const items = allowedTypes
    ? allItems.filter((a) => allowedTypes.includes(a.type))
    : allItems;

  return (
    <div ref={containerRef} className="relative">
      <Input
        id={inputId}
        value={query}
        disabled={disabled}
        placeholder={placeholder}
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
          onChange(e.target.value);
        }}
        {...aria}
      />
      {enabled && items.length > 0 && (
        <ul
          role="listbox"
          className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md border bg-popover shadow-md"
        >
          {items.map((acc) => (
            <li
              key={acc.id}
              role="option"
              aria-selected={acc.name === value}
              className={cn(
                'cursor-pointer px-3 py-1.5 text-sm hover:bg-accent hover:text-accent-foreground',
                acc.name === value && 'bg-accent text-accent-foreground',
              )}
              onMouseDown={(e) => {
                e.preventDefault();
                setQuery(acc.name);
                setOpen(false);
                onChange(acc.name, acc);
              }}
            >
              <div>{acc.name}</div>
              <div className="text-xs text-muted-foreground">
                {acc.type} · {acc.currency}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Scaffold the shadcn Input component (one-time)**

Check whether `spa/src/components/ui/input.tsx` exists:
```bash
ls /Users/hance/programming/kea/spa/src/components/ui/input.tsx 2>/dev/null || echo "missing"
```

If missing, create `spa/src/components/ui/input.tsx`:

```tsx
import * as React from 'react';
import { cn } from '@/lib/cn';

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type, ...props }, ref) => (
    <input
      type={type}
      className={cn(
        'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      ref={ref}
      {...props}
    />
  ),
);
Input.displayName = 'Input';
```

- [ ] **Step 3: Type-check**

```bash
cd /Users/hance/programming/kea/spa
npm run build
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/components/transactions/AccountCombobox.tsx spa/src/components/ui/input.tsx
git commit -m "feat(spa): add AccountCombobox and Input UI primitive"
```

---

## Task 9: Pagination component

**Files:**
- Create: `spa/src/components/transactions/Pagination.tsx`
- Create: `spa/src/test/components/Pagination.test.tsx`

- [ ] **Step 1: Write failing test at `spa/src/test/components/Pagination.test.tsx`**

```tsx
import { Pagination } from '@/components/transactions/Pagination';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, test, vi } from 'vitest';

describe('Pagination', () => {
  test('renders page X of Y', () => {
    render(<Pagination total={120} limit={50} offset={50} onChange={() => {}} />);
    expect(screen.getByText('Page 2 of 3')).toBeInTheDocument();
  });

  test('Prev disabled on first page', () => {
    render(<Pagination total={120} limit={50} offset={0} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: /prev/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /next/i })).not.toBeDisabled();
  });

  test('Next disabled on last page', () => {
    render(<Pagination total={120} limit={50} offset={100} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: /next/i })).toBeDisabled();
  });

  test('clicking Next calls onChange with next offset', () => {
    const onChange = vi.fn();
    render(<Pagination total={120} limit={50} offset={0} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: /next/i }));
    expect(onChange).toHaveBeenCalledWith(50);
  });

  test('clicking Prev calls onChange with previous offset', () => {
    const onChange = vi.fn();
    render(<Pagination total={120} limit={50} offset={100} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: /prev/i }));
    expect(onChange).toHaveBeenCalledWith(50);
  });

  test('total <= limit hides pagination', () => {
    const { container } = render(
      <Pagination total={10} limit={50} offset={0} onChange={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
```

- [ ] **Step 2: Run test and verify failure**

```bash
cd /Users/hance/programming/kea/spa
npm test -- Pagination
```

Expected: FAIL — module not found.

- [ ] **Step 3: Implement `spa/src/components/transactions/Pagination.tsx`**

```tsx
import { Button } from '@/components/ui/button';

interface Props {
  total: number;
  limit: number;
  offset: number;
  onChange: (newOffset: number) => void;
}

export function Pagination({ total, limit, offset, onChange }: Props) {
  if (total <= limit) return null;

  const pageCount = Math.max(1, Math.ceil(total / limit));
  const currentPage = Math.floor(offset / limit) + 1;
  const canPrev = offset > 0;
  const canNext = offset + limit < total;

  return (
    <div className="mt-4 flex items-center justify-between text-sm text-muted-foreground">
      <span>
        Page {currentPage} of {pageCount}
      </span>
      <div className="flex gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={!canPrev}
          onClick={() => onChange(Math.max(0, offset - limit))}
        >
          ‹ Prev
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={!canNext}
          onClick={() => onChange(offset + limit)}
        >
          Next ›
        </Button>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run test and verify it passes**

```bash
npm test -- Pagination
```

Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/components/transactions/Pagination.tsx spa/src/test/components/Pagination.test.tsx
git commit -m "feat(spa): add Pagination component"
```

---

## Task 10: FilterBar component

**Files:**
- Create: `spa/src/components/transactions/FilterBar.tsx`

- [ ] **Step 1: Scaffold a tiny `Label` primitive at `spa/src/components/ui/label.tsx`**

(Skip if it already exists.) Check first:
```bash
ls /Users/hance/programming/kea/spa/src/components/ui/label.tsx 2>/dev/null || echo "missing"
```

If missing, create:
```tsx
import * as React from 'react';
import { cn } from '@/lib/cn';

export const Label = React.forwardRef<
  HTMLLabelElement,
  React.LabelHTMLAttributes<HTMLLabelElement>
>(({ className, ...props }, ref) => (
  <label
    ref={ref}
    className={cn('text-xs font-medium text-foreground', className)}
    {...props}
  />
));
Label.displayName = 'Label';
```

- [ ] **Step 2: Create `FilterBar.tsx`**

```tsx
import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import type { TransactionsSearch } from '@/lib/transactions-search-params';
import type { TransactionStatus, TransactionType } from '@/lib/types';

const TYPES: TransactionType[] = [
  'Expense',
  'Income',
  'Transfer',
  'Opening',
  'Deposit',
  'Withdrawal',
  'Other',
];
const STATUSES: TransactionStatus[] = ['Pending', 'Cleared', 'Reconciled'];

interface Props {
  search: TransactionsSearch;
  onChange: (partial: Partial<TransactionsSearch>) => void;
  onClear: () => void;
}

function unixToDate(u?: number): string {
  if (!u) return '';
  return new Date(u * 1000).toISOString().slice(0, 10);
}

function dateToUnix(d: string, endOfDay: boolean): number | undefined {
  if (!d) return undefined;
  const ms = new Date(`${d}T${endOfDay ? '23:59:59' : '00:00:00'}Z`).getTime();
  return Number.isNaN(ms) ? undefined : Math.floor(ms / 1000);
}

export function FilterBar({ search, onChange, onClear }: Props) {
  const hasAny =
    search.account_id !== undefined ||
    search.type !== undefined ||
    search.status !== undefined ||
    search.start_time !== undefined ||
    search.end_time !== undefined ||
    (search.description !== undefined && search.description !== '');

  return (
    <div className="mb-4 grid grid-cols-1 gap-3 rounded-md border bg-card p-3 md:grid-cols-6">
      <div className="md:col-span-2">
        <Label htmlFor="f-account">Account</Label>
        <AccountCombobox
          id="f-account"
          value="" // filter uses account_id only; combobox returns a name then we resolve
          onChange={(_name, account) => onChange({ account_id: account?.id })}
          placeholder="Any account"
        />
      </div>

      <div>
        <Label htmlFor="f-type">Type</Label>
        <select
          id="f-type"
          className="flex h-10 w-full rounded-md border border-input bg-background px-2 text-sm"
          value={search.type ?? ''}
          onChange={(e) =>
            onChange({ type: (e.target.value || undefined) as TransactionType | undefined })
          }
        >
          <option value="">Any</option>
          {TYPES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </div>

      <div>
        <Label htmlFor="f-status">Status</Label>
        <select
          id="f-status"
          className="flex h-10 w-full rounded-md border border-input bg-background px-2 text-sm"
          value={search.status ?? ''}
          onChange={(e) =>
            onChange({
              status: (e.target.value || undefined) as TransactionStatus | undefined,
            })
          }
        >
          <option value="">Any</option>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </div>

      <div>
        <Label htmlFor="f-from">From date</Label>
        <Input
          id="f-from"
          type="date"
          value={unixToDate(search.start_time)}
          onChange={(e) => onChange({ start_time: dateToUnix(e.target.value, false) })}
        />
      </div>

      <div>
        <Label htmlFor="f-to">To date</Label>
        <Input
          id="f-to"
          type="date"
          value={unixToDate(search.end_time)}
          onChange={(e) => onChange({ end_time: dateToUnix(e.target.value, true) })}
        />
      </div>

      <div className="md:col-span-4">
        <Label htmlFor="f-desc">Description contains</Label>
        <Input
          id="f-desc"
          type="text"
          value={search.description ?? ''}
          onChange={(e) =>
            onChange({ description: e.target.value === '' ? undefined : e.target.value })
          }
          placeholder="Search description…"
        />
      </div>

      <div className="flex items-end gap-2 md:col-span-2">
        {hasAny && (
          <Button variant="outline" size="sm" onClick={onClear}>
            Clear filters
          </Button>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Type-check**

```bash
cd /Users/hance/programming/kea/spa
npm run build
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/components/transactions/FilterBar.tsx spa/src/components/ui/label.tsx
git commit -m "feat(spa): add FilterBar for transactions list"
```

---

## Task 11: TransactionsTable + TransactionRow

**Files:**
- Create: `spa/src/components/transactions/TransactionRow.tsx`
- Create: `spa/src/components/transactions/TransactionsTable.tsx`

- [ ] **Step 1: Create `TransactionRow.tsx`**

```tsx
import { StatusText } from '@/components/transactions/StatusText';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { cn } from '@/lib/cn';
import { formatCents } from '@/lib/format';
import { displayAccount, displayAmount, displayOffsetAccount } from '@/lib/transactionDisplay';
import type { TransactionDetail } from '@/lib/types';
import { Link } from '@tanstack/react-router';

interface Props {
  tx: TransactionDetail;
}

function fmtDate(unix: number): string {
  const d = new Date(unix * 1000);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

export function TransactionRow({ tx }: Props) {
  const acc = displayAccount(tx.splits, tx.type);
  const offset = displayOffsetAccount(tx.splits, tx.type, acc);
  const { amount, currency } = displayAmount(tx.splits, tx.type);
  const signClass = amount < 0 ? 'text-red-600' : amount > 0 ? 'text-green-600' : '';

  return (
    <Link
      to="/transactions/$id"
      params={{ id: String(tx.id) }}
      className="grid grid-cols-[80px_80px_1fr_1fr_120px_90px] items-center gap-3 border-t px-3 py-2 text-sm hover:bg-muted/50"
    >
      <span className="text-muted-foreground">{fmtDate(tx.timestamp)}</span>
      <TypeBadge type={tx.type} />
      <span className="truncate" title={tx.description}>
        {tx.description}
      </span>
      <span className="truncate text-muted-foreground">
        {acc} → {offset}
      </span>
      <span className={cn('text-right tabular-nums', signClass)}>
        {formatCents(amount, currency)}
      </span>
      <StatusText status={tx.status} />
    </Link>
  );
}
```

- [ ] **Step 2: Create `TransactionsTable.tsx`**

```tsx
import { TransactionRow } from '@/components/transactions/TransactionRow';
import type { TransactionDetail } from '@/lib/types';

interface Props {
  items: TransactionDetail[];
}

export function TransactionsTable({ items }: Props) {
  return (
    <div className="rounded-md border bg-card">
      <div className="grid grid-cols-[80px_80px_1fr_1fr_120px_90px] gap-3 bg-muted/50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        <span>Date</span>
        <span>Type</span>
        <span>Description</span>
        <span>Account → Offset</span>
        <span className="text-right">Amount</span>
        <span>Status</span>
      </div>
      {items.map((tx) => (
        <TransactionRow key={tx.id} tx={tx} />
      ))}
    </div>
  );
}
```

- [ ] **Step 3: Type-check**

```bash
cd /Users/hance/programming/kea/spa
npm run build
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/components/transactions/TransactionRow.tsx spa/src/components/transactions/TransactionsTable.tsx
git commit -m "feat(spa): add TransactionsTable and TransactionRow"
```

---

## Task 12: List route (layout + index) and enable sidebar

**Files:**
- Create: `spa/src/routes/transactions.tsx`
- Create: `spa/src/routes/transactions.index.tsx`
- Modify: `spa/src/components/Sidebar.tsx`

- [ ] **Step 1: Create `spa/src/routes/transactions.tsx`** (layout route)

```tsx
import {
  parseTransactionsSearch,
  type TransactionsSearch,
} from '@/lib/transactions-search-params';
import { Outlet, createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions')({
  validateSearch: (s): TransactionsSearch => parseTransactionsSearch(s),
  component: TransactionsLayout,
});

function TransactionsLayout() {
  return (
    <div>
      <Outlet />
    </div>
  );
}
```

- [ ] **Step 2: Create `spa/src/routes/transactions.index.tsx`** (the list page)

```tsx
import { FilterBar } from '@/components/transactions/FilterBar';
import { Pagination } from '@/components/transactions/Pagination';
import { TransactionsTable } from '@/components/transactions/TransactionsTable';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  searchToFilter,
  searchToListOptions,
  type TransactionsSearch,
} from '@/lib/transactions-search-params';
import { listTransactions } from '@/lib/transactions';
import { useQuery } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions/')({
  component: TransactionsListPage,
});

function TransactionsListPage() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: '/transactions' });

  const filter = searchToFilter(search);
  const opts = searchToListOptions(search);

  const query = useQuery({
    queryKey: ['transactions', { ...filter, ...opts }],
    queryFn: () => listTransactions(filter, opts),
  });

  const setSearch = (partial: Partial<TransactionsSearch>) => {
    navigate({
      search: (prev) => ({ ...prev, ...partial, offset: 0 }),
    });
  };
  const clear = () => {
    navigate({ search: { limit: search.limit, offset: 0 } });
  };
  const setOffset = (offset: number) => {
    navigate({ search: (prev) => ({ ...prev, offset }) });
  };

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Transactions</h1>
        <Button asChild size="sm">
          <Link to="/transactions/new">+ New transaction</Link>
        </Button>
      </div>

      <FilterBar search={search} onChange={setSearch} onClear={clear} />

      {query.isPending && (
        <div className="space-y-2">
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-9 w-full" />
          ))}
        </div>
      )}

      {query.isError && (
        <Alert variant="destructive">
          <AlertTitle>Failed to load transactions</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>
              {query.error instanceof Error ? query.error.message : 'Unknown error'}
            </div>
            <Button onClick={() => query.refetch()} size="sm">
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {query.isSuccess && query.data.items.length === 0 && (
        <div className="rounded-md border bg-card p-8 text-center text-sm text-muted-foreground">
          No transactions match these filters.
        </div>
      )}

      {query.isSuccess && query.data.items.length > 0 && (
        <>
          <TransactionsTable items={query.data.items} />
          <Pagination
            total={query.data.total_count}
            limit={search.limit}
            offset={search.offset}
            onChange={setOffset}
          />
        </>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Enable the sidebar item**

Edit `spa/src/components/Sidebar.tsx`. Change:
```ts
{ label: 'Transactions' },
```
to:
```ts
{ label: 'Transactions', to: '/transactions' },
```

- [ ] **Step 4: Start dev server to regenerate `routeTree.gen.ts`**

```bash
cd /Users/hance/programming/kea/spa
npm run dev &
sleep 5
kill %1 2>/dev/null
```

Verify the routes appeared:
```bash
grep -E "/transactions|TransactionsLayout|TransactionsListPage" /Users/hance/programming/kea/spa/src/routeTree.gen.ts | head -5
```

Expected: route entries present.

- [ ] **Step 5: Type-check + lint**

```bash
cd /Users/hance/programming/kea/spa
npm run build
npm run check
```

Expected: both succeed.

- [ ] **Step 6: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/routes/transactions.tsx spa/src/routes/transactions.index.tsx spa/src/components/Sidebar.tsx spa/src/routeTree.gen.ts
git commit -m "feat(spa): add transactions list route and enable sidebar item"
```

---

## Task 13: List integration test

**Files:**
- Create: `spa/src/test/transactions.list.test.tsx`

- [ ] **Step 1: Create the test**

```tsx
import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const SEEDED_LIST = {
  items: [
    {
      id: 1,
      timestamp: 1733184000,
      description: 'Coffee with team',
      status: 'Cleared',
      type: 'Expense',
      splits: [
        {
          id: 10,
          account_id: 1,
          account_name: 'Assets:Bank',
          account_type: 'A',
          amount: -1250,
          currency: 'USD',
          memo: '',
        },
        {
          id: 11,
          account_id: 2,
          account_name: 'Expenses:Coffee',
          account_type: 'E',
          amount: 1250,
          currency: 'USD',
          memo: '',
        },
      ],
    },
    {
      id: 2,
      timestamp: 1733097600,
      description: 'June salary',
      status: 'Cleared',
      type: 'Income',
      splits: [
        {
          id: 20,
          account_id: 1,
          account_name: 'Assets:Bank',
          account_type: 'A',
          amount: 420000,
          currency: 'USD',
          memo: '',
        },
        {
          id: 21,
          account_id: 3,
          account_name: 'Revenue:Salary',
          account_type: 'R',
          amount: -420000,
          currency: 'USD',
          memo: '',
        },
      ],
    },
  ],
  total_count: 2,
  limit: 50,
  offset: 0,
};

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/transactions?')) {
        return Promise.resolve(okResponse(SEEDED_LIST));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('renders the transactions table with seeded rows', async () => {
  render(makeTestApp('/transactions'));

  expect(await screen.findByText('Coffee with team')).toBeInTheDocument();
  expect(await screen.findByText('June salary')).toBeInTheDocument();
});

test('Status column renders as plain text without color or emoji', async () => {
  render(makeTestApp('/transactions'));

  const cleared = await screen.findAllByText('Cleared');
  // At least one Cleared status text exists.
  expect(cleared.length).toBeGreaterThan(0);
  // None of them carries a role="img" (i.e., no emoji label) or text-green/text-red coloring.
  for (const el of cleared) {
    expect(el).not.toHaveAttribute('role', 'img');
    const cls = el.className;
    expect(cls).not.toMatch(/text-(green|red|amber|blue)-/);
  }
});

test('pagination is hidden when total <= limit', async () => {
  render(makeTestApp('/transactions'));
  await screen.findByText('Coffee with team');
  expect(screen.queryByRole('button', { name: /next/i })).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run the test**

```bash
cd /Users/hance/programming/kea/spa
npm test -- transactions.list
```

Expected: PASS (3 tests). If `Status` text matches more than one element due to the FilterBar's Status `<select>` option also rendering "Cleared", scope the assertion: use `screen.findAllByText('Cleared')` and filter to only the ones inside the table — or assert specifically against the row count.

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/test/transactions.list.test.tsx
git commit -m "test(spa): integration test for transactions list page"
```

---

## Task 14: Detail route (`/transactions/$id`)

**Files:**
- Create: `spa/src/routes/transactions.$id.tsx`

- [ ] **Step 1: Create the route**

```tsx
import { ReconciledBanner } from '@/components/transactions/ReconciledBanner';
import { StatusText } from '@/components/transactions/StatusText';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { formatCents } from '@/lib/format';
import { deleteTransaction, getTransaction } from '@/lib/transactions';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';

export const Route = createFileRoute('/transactions/$id')({
  component: TransactionDetailPage,
});

function TransactionDetailPage() {
  const { id } = Route.useParams();
  const txId = Number(id);
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const query = useQuery({
    queryKey: ['transaction', txId],
    queryFn: () => getTransaction(txId),
  });

  const deleteMut = useMutation({
    mutationFn: () => deleteTransaction(txId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['transactions'] });
      queryClient.invalidateQueries({ queryKey: ['balances'] });
      navigate({ to: '/transactions' });
    },
  });

  if (query.isPending) {
    return <Skeleton className="h-48 w-full" />;
  }

  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load transaction</AlertTitle>
        <AlertDescription>
          {query.error instanceof Error ? query.error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    );
  }

  const tx = query.data;
  const isReconciled = tx.status === 'Reconciled';
  const date = new Date(tx.timestamp * 1000).toISOString().slice(0, 10);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Link to="/transactions" className="text-sm text-muted-foreground hover:underline">
          ← Back to transactions
        </Link>
        {!isReconciled && (
          <div className="flex gap-2">
            <Button asChild size="sm" variant="outline">
              <Link to="/transactions/$id/edit" params={{ id: String(tx.id) }}>
                Edit
              </Link>
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={() => setConfirmingDelete(true)}
              disabled={deleteMut.isPending}
            >
              Delete
            </Button>
          </div>
        )}
      </div>

      {isReconciled && <ReconciledBanner />}

      <div className="rounded-md border bg-card p-4">
        <div className="mb-3 flex items-center gap-3">
          <h2 className="text-lg font-semibold">{tx.description}</h2>
          <TypeBadge type={tx.type} />
        </div>
        <dl className="grid grid-cols-2 gap-2 text-sm">
          <dt className="text-muted-foreground">Date</dt>
          <dd>{date}</dd>
          <dt className="text-muted-foreground">Status</dt>
          <dd>
            <StatusText status={tx.status} />
          </dd>
        </dl>
      </div>

      <div className="rounded-md border bg-card">
        <div className="grid grid-cols-[1fr_120px_1fr] gap-3 bg-muted/50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          <span>Account</span>
          <span className="text-right">Amount</span>
          <span>Memo</span>
        </div>
        {tx.splits.map((s) => (
          <div
            key={s.id}
            className="grid grid-cols-[1fr_120px_1fr] gap-3 border-t px-3 py-2 text-sm"
          >
            <span>{s.account_name}</span>
            <span
              className={`text-right tabular-nums ${
                s.amount < 0 ? 'text-red-600' : s.amount > 0 ? 'text-green-600' : ''
              }`}
            >
              {formatCents(s.amount, s.currency)}
            </span>
            <span className="text-muted-foreground">{s.memo || '—'}</span>
          </div>
        ))}
      </div>

      {confirmingDelete && (
        <Alert variant="destructive">
          <AlertTitle>Delete this transaction?</AlertTitle>
          <AlertDescription className="mt-2 space-y-3">
            <div>This cannot be undone.</div>
            <div className="flex gap-2">
              <Button
                size="sm"
                variant="destructive"
                onClick={() => deleteMut.mutate()}
                disabled={deleteMut.isPending}
              >
                Yes, delete
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setConfirmingDelete(false)}
              >
                Cancel
              </Button>
            </div>
            {deleteMut.isError && (
              <div className="text-sm">
                {deleteMut.error instanceof Error
                  ? deleteMut.error.message
                  : 'Delete failed'}
              </div>
            )}
          </AlertDescription>
        </Alert>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Regenerate route tree and build**

```bash
cd /Users/hance/programming/kea/spa
npm run dev &
sleep 5
kill %1 2>/dev/null
npm run build
```

Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/routes/transactions.\$id.tsx spa/src/routeTree.gen.ts
git commit -m "feat(spa): add transaction detail route with delete"
```

---

## Task 15: Detail test + Reconciled test

**Files:**
- Create: `spa/src/test/transactions.reconciled.test.tsx`

- [ ] **Step 1: Create the test**

```tsx
import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { makeTestApp } from './test-app';

const okResponse = (body: unknown) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });

const RECONCILED_TX = {
  id: 99,
  timestamp: 1733184000,
  description: 'Old groceries',
  status: 'Reconciled',
  type: 'Expense',
  splits: [
    {
      id: 100,
      account_id: 6,
      account_name: 'Assets:Cash',
      account_type: 'A',
      amount: -8320,
      currency: 'USD',
      memo: '',
    },
    {
      id: 101,
      account_id: 2,
      account_name: 'Expenses:Groceries',
      account_type: 'E',
      amount: 8320,
      currency: 'USD',
      memo: '',
    },
  ],
};

const CLEARED_TX = { ...RECONCILED_TX, id: 100, status: 'Cleared' };

function setupFetch(tx: typeof RECONCILED_TX) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p', active: true }] }),
        );
      }
      if (url === `/api/transactions/${tx.id}`) {
        return Promise.resolve(okResponse(tx));
      }
      throw new Error(`unexpected fetch: ${url}`);
    }),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

test('reconciled detail shows ReconciledBanner and hides Edit/Delete', async () => {
  setupFetch(RECONCILED_TX);
  render(makeTestApp(`/transactions/${RECONCILED_TX.id}`));

  expect(await screen.findByText(/This transaction is reconciled/i)).toBeInTheDocument();
  expect(screen.queryByRole('link', { name: /edit/i })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
});

test('non-reconciled detail shows Edit and Delete', async () => {
  setupFetch(CLEARED_TX);
  render(makeTestApp(`/transactions/${CLEARED_TX.id}`));

  expect(await screen.findByText('Old groceries')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /edit/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
  expect(screen.queryByText(/This transaction is reconciled/i)).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run the test**

```bash
cd /Users/hance/programming/kea/spa
npm test -- transactions.reconciled
```

Expected: PASS (2 tests).

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/test/transactions.reconciled.test.tsx
git commit -m "test(spa): assert reconciled lockout on detail page"
```

---

## Task 16: SimpleFields component

**Files:**
- Create: `spa/src/components/transactions/SimpleFields.tsx`

This component is presentational — its parent (`TransactionForm`) owns react-hook-form state. We use it for the Simple-mode From/To/Amount sub-fields plus the live type-derivation badge.

- [ ] **Step 1: Create `SimpleFields.tsx`**

```tsx
import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { determineType } from '@/lib/determineType';
import { listAccounts } from '@/lib/transactions';
import type { TransactionType } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

interface Props {
  fromAccount: string;
  toAccount: string;
  amount: string; // form text input (decimal string)
  onFromChange: (name: string) => void;
  onToChange: (name: string) => void;
  onAmountChange: (value: string) => void;
  fieldErrors?: { fromAccount?: string; toAccount?: string; amount?: string };
}

export function SimpleFields(props: Props) {
  const accounts = useQuery({
    queryKey: ['accounts', 'list'],
    queryFn: listAccounts,
    staleTime: 60_000,
  });

  const derivedType: TransactionType | '…' = useMemo(() => {
    if (!accounts.data) return '…';
    const map = new Map(accounts.data.items.map((a) => [a.name, a.type]));
    const fromType = map.get(props.fromAccount);
    const toType = map.get(props.toAccount);
    if (!fromType || !toType) return '…';
    const amountNum = Math.round(Number(props.amount) * 100);
    if (!Number.isFinite(amountNum) || amountNum === 0) return '…';
    return determineType([
      {
        id: 0,
        account_id: 0,
        account_name: props.fromAccount,
        account_type: fromType,
        amount: -amountNum,
        currency: 'USD',
        memo: '',
      },
      {
        id: 0,
        account_id: 0,
        account_name: props.toAccount,
        account_type: toType,
        amount: amountNum,
        currency: 'USD',
        memo: '',
      },
    ]);
  }, [accounts.data, props.fromAccount, props.toAccount, props.amount]);

  return (
    <>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label htmlFor="from">From account</Label>
          <AccountCombobox
            id="from"
            value={props.fromAccount}
            onChange={(name) => props.onFromChange(name)}
            placeholder="Money comes from…"
            aria-invalid={!!props.fieldErrors?.fromAccount}
          />
          {props.fieldErrors?.fromAccount && (
            <p className="mt-1 text-xs text-destructive">{props.fieldErrors.fromAccount}</p>
          )}
        </div>
        <div>
          <Label htmlFor="to">To account</Label>
          <AccountCombobox
            id="to"
            value={props.toAccount}
            onChange={(name) => props.onToChange(name)}
            placeholder="Money goes to…"
            aria-invalid={!!props.fieldErrors?.toAccount}
          />
          {props.fieldErrors?.toAccount && (
            <p className="mt-1 text-xs text-destructive">{props.fieldErrors.toAccount}</p>
          )}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label htmlFor="amount">Amount</Label>
          <Input
            id="amount"
            type="text"
            inputMode="decimal"
            value={props.amount}
            onChange={(e) => props.onAmountChange(e.target.value)}
            placeholder="0.00"
            aria-invalid={!!props.fieldErrors?.amount}
          />
          {props.fieldErrors?.amount && (
            <p className="mt-1 text-xs text-destructive">{props.fieldErrors.amount}</p>
          )}
        </div>
        <div>
          <Label>Type (auto)</Label>
          <div className="flex h-10 items-center">
            {derivedType === '…' ? (
              <span className="text-sm text-muted-foreground">…</span>
            ) : (
              <TypeBadge type={derivedType} />
            )}
          </div>
        </div>
      </div>
    </>
  );
}
```

- [ ] **Step 2: Type-check**

```bash
cd /Users/hance/programming/kea/spa
npm run build
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/components/transactions/SimpleFields.tsx
git commit -m "feat(spa): add SimpleFields with live type derivation"
```

---

## Task 17: SplitsEditor component

**Files:**
- Create: `spa/src/components/transactions/SplitsEditor.tsx`

- [ ] **Step 1: Create `SplitsEditor.tsx`**

```tsx
import { AccountCombobox } from '@/components/transactions/AccountCombobox';
import { TypeBadge } from '@/components/transactions/TypeBadge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { determineType } from '@/lib/determineType';
import { listAccounts } from '@/lib/transactions';
import type { SplitInput, TransactionType } from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

interface SplitRow extends SplitInput {
  // amountStr is the editable text; amount (cents) is derived on submit.
  amountStr: string;
}

interface Props {
  splits: SplitRow[];
  onChange: (next: SplitRow[]) => void;
  splitsError?: string;
}

function parseCents(s: string): number {
  const n = Number(s);
  if (!Number.isFinite(n)) return Number.NaN;
  return Math.round(n * 100);
}

function balance(splits: SplitRow[]): number {
  return splits.reduce((acc, s) => {
    const cents = parseCents(s.amountStr);
    return Number.isFinite(cents) ? acc + cents : acc;
  }, 0);
}

export function SplitsEditor({ splits, onChange, splitsError }: Props) {
  const accounts = useQuery({
    queryKey: ['accounts', 'list'],
    queryFn: listAccounts,
    staleTime: 60_000,
  });

  const accountTypeMap = useMemo(() => {
    return new Map(accounts.data?.items.map((a) => [a.name, a.type]) ?? []);
  }, [accounts.data]);

  const derivedType: TransactionType | '…' = useMemo(() => {
    if (!accounts.data) return '…';
    const sd = splits.map((s) => {
      const cents = parseCents(s.amountStr);
      const t = accountTypeMap.get(s.account_name);
      if (!t || !Number.isFinite(cents)) return null;
      return {
        id: 0,
        account_id: 0,
        account_name: s.account_name,
        account_type: t,
        amount: cents,
        currency: s.currency || 'USD',
        memo: s.memo ?? '',
      };
    });
    if (sd.some((x) => x === null)) return '…';
    return determineType(sd as Parameters<typeof determineType>[0]);
  }, [accounts.data, accountTypeMap, splits]);

  const bal = balance(splits);
  const balanced = bal === 0 && splits.length >= 2;

  const updateRow = (i: number, partial: Partial<SplitRow>) => {
    const next = splits.map((s, idx) => (idx === i ? { ...s, ...partial } : s));
    onChange(next);
  };

  const addRow = () => {
    onChange([...splits, { account_name: '', amountStr: '', currency: 'USD', memo: '' }]);
  };

  const removeRow = (i: number) => {
    onChange(splits.filter((_, idx) => idx !== i));
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>Splits</Label>
        {derivedType !== '…' && (
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span>Type:</span>
            <TypeBadge type={derivedType} />
          </div>
        )}
      </div>

      {splitsError && (
        <div className="rounded-md border border-destructive bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {splitsError}
        </div>
      )}

      <div className="rounded-md border bg-card">
        <div className="grid grid-cols-[2fr_1fr_1fr_120px_30px] gap-2 bg-muted/50 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          <span>Account</span>
          <span className="text-right">Amount</span>
          <span>Memo</span>
          <span>Currency</span>
          <span />
        </div>
        {splits.map((s, i) => (
          <div
            key={i}
            className="grid grid-cols-[2fr_1fr_1fr_120px_30px] items-center gap-2 border-t px-3 py-2 text-sm"
          >
            <AccountCombobox
              value={s.account_name}
              onChange={(name) => updateRow(i, { account_name: name })}
              placeholder="Account…"
            />
            <Input
              type="text"
              inputMode="decimal"
              className="text-right"
              value={s.amountStr}
              onChange={(e) => updateRow(i, { amountStr: e.target.value })}
              placeholder="0.00"
            />
            <Input
              type="text"
              value={s.memo ?? ''}
              onChange={(e) => updateRow(i, { memo: e.target.value })}
              placeholder="(optional)"
            />
            <Input
              type="text"
              value={s.currency}
              onChange={(e) => updateRow(i, { currency: e.target.value.toUpperCase() })}
            />
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => removeRow(i)}
              aria-label={`Remove split ${i + 1}`}
            >
              ×
            </Button>
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between text-sm">
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          + Add split
        </Button>
        <span className={balanced ? 'text-green-600' : 'text-red-600'}>
          Balance: {(bal / 100).toFixed(2)} {balanced ? '✓' : '✗'}
        </span>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Type-check**

```bash
cd /Users/hance/programming/kea/spa
npm run build
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/components/transactions/SplitsEditor.tsx
git commit -m "feat(spa): add SplitsEditor with live balance and type derivation"
```

---

## Task 18: TransactionForm shell

**Files:**
- Create: `spa/src/components/transactions/TransactionForm.tsx`

This is the form shell — owns react-hook-form state, picks Simple vs Advanced sub-view, handles submit, maps API errors.

- [ ] **Step 1: Create `TransactionForm.tsx`**

```tsx
import { SimpleFields } from '@/components/transactions/SimpleFields';
import { SplitsEditor } from '@/components/transactions/SplitsEditor';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ApiError } from '@/lib/api';
import { listAccounts } from '@/lib/transactions';
import type {
  CreateTransactionInput,
  TransactionDetail,
  TransactionStatus,
  UpdateTransactionInput,
} from '@/lib/types';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

interface SplitRow {
  id?: number;
  account_name: string;
  amountStr: string;
  currency: string;
  memo?: string;
}

interface FormState {
  description: string;
  date: string; // YYYY-MM-DD
  status: TransactionStatus;
  // Simple-only:
  fromAccount: string;
  toAccount: string;
  amount: string;
  // Advanced:
  splits: SplitRow[];
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

function dateToUnix(d: string): number {
  return Math.floor(new Date(`${d}T12:00:00Z`).getTime() / 1000);
}

function unixToDate(u: number): string {
  return new Date(u * 1000).toISOString().slice(0, 10);
}

function parseCents(s: string): number {
  const n = Number(s);
  if (!Number.isFinite(n)) return Number.NaN;
  return Math.round(n * 100);
}

function initialFromDetail(tx: TransactionDetail): FormState {
  return {
    description: tx.description,
    date: unixToDate(tx.timestamp),
    status: tx.status,
    fromAccount: '',
    toAccount: '',
    amount: '',
    splits: tx.splits.map((s) => ({
      id: s.id,
      account_name: s.account_name,
      amountStr: (s.amount / 100).toString(),
      currency: s.currency,
      memo: s.memo,
    })),
  };
}

function initialEmpty(): FormState {
  return {
    description: '',
    date: todayISO(),
    status: 'Cleared',
    fromAccount: '',
    toAccount: '',
    amount: '',
    splits: [
      { account_name: '', amountStr: '', currency: 'USD', memo: '' },
      { account_name: '', amountStr: '', currency: 'USD', memo: '' },
    ],
  };
}

interface Props {
  mode: 'create' | 'edit';
  initial?: TransactionDetail;
  onSubmit: (
    payload: CreateTransactionInput | UpdateTransactionInput,
  ) => Promise<TransactionDetail>;
  onSuccess: (tx: TransactionDetail) => void;
  onCancel: () => void;
}

export function TransactionForm({ mode, initial, onSubmit, onSuccess, onCancel }: Props) {
  const isEdit = mode === 'edit';
  const [state, setState] = useState<FormState>(
    initial ? initialFromDetail(initial) : initialEmpty(),
  );
  const [advanced, setAdvanced] = useState(isEdit); // edit always opens in Advanced
  const [submitting, setSubmitting] = useState(false);
  const [topError, setTopError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const accountsQuery = useQuery({
    queryKey: ['accounts', 'list'],
    queryFn: listAccounts,
    staleTime: 60_000,
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setTopError(null);
    setFieldErrors({});

    try {
      const timestamp = dateToUnix(state.date);
      let splits: { id?: number; account_name: string; amount: number; currency: string; memo?: string }[];

      if (advanced) {
        splits = state.splits.map((s) => ({
          id: s.id,
          account_name: s.account_name.trim(),
          amount: parseCents(s.amountStr),
          currency: s.currency || 'USD',
          memo: s.memo,
        }));
        if (splits.some((s) => !Number.isFinite(s.amount))) {
          setFieldErrors({ splits: 'All split amounts must be valid numbers.' });
          setSubmitting(false);
          return;
        }
      } else {
        const amt = parseCents(state.amount);
        if (!Number.isFinite(amt) || amt === 0) {
          setFieldErrors({ amount: 'Amount must be a non-zero number.' });
          setSubmitting(false);
          return;
        }
        if (!state.fromAccount.trim()) {
          setFieldErrors({ fromAccount: 'From account is required.' });
          setSubmitting(false);
          return;
        }
        if (!state.toAccount.trim()) {
          setFieldErrors({ toAccount: 'To account is required.' });
          setSubmitting(false);
          return;
        }
        const fromAcc = accountsQuery.data?.items.find(
          (a) => a.name === state.fromAccount,
        );
        const currency = fromAcc?.currency ?? 'USD';
        splits = [
          {
            account_name: state.fromAccount.trim(),
            amount: -amt,
            currency,
          },
          {
            account_name: state.toAccount.trim(),
            amount: amt,
            currency,
          },
        ];
      }

      const payload = isEdit
        ? ({
            id: initial!.id,
            description: state.description,
            timestamp,
            status: state.status,
            splits,
          } satisfies UpdateTransactionInput)
        : ({
            description: state.description,
            timestamp,
            status: state.status,
            splits,
          } satisfies CreateTransactionInput);

      const result = await onSubmit(payload);
      onSuccess(result);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.field) {
          // Map server field names to form field names where they diverge.
          const mapped =
            err.field === 'splits'
              ? 'splits'
              : err.field === 'from_account'
                ? 'fromAccount'
                : err.field === 'to_account'
                  ? 'toAccount'
                  : err.field;
          setFieldErrors({ [mapped]: err.message });
        } else {
          setTopError(err.message);
        }
      } else {
        setTopError(err instanceof Error ? err.message : 'Submission failed');
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">
          {isEdit ? 'Edit transaction' : 'New transaction'}
        </h1>
        {!isEdit && (
          <label className="flex items-center gap-2 text-sm text-muted-foreground">
            <input
              type="checkbox"
              checked={advanced}
              onChange={(e) => setAdvanced(e.target.checked)}
            />
            Advanced (edit splits)
          </label>
        )}
      </div>

      {topError && (
        <div className="rounded-md border border-destructive bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {topError}
        </div>
      )}

      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label htmlFor="date">Date</Label>
          <Input
            id="date"
            type="date"
            value={state.date}
            onChange={(e) => setState((s) => ({ ...s, date: e.target.value }))}
            aria-invalid={!!fieldErrors.timestamp}
          />
          {fieldErrors.timestamp && (
            <p className="mt-1 text-xs text-destructive">{fieldErrors.timestamp}</p>
          )}
        </div>
        <div>
          <Label htmlFor="status">Status</Label>
          <select
            id="status"
            className="flex h-10 w-full rounded-md border border-input bg-background px-2 text-sm"
            value={state.status}
            onChange={(e) =>
              setState((s) => ({ ...s, status: e.target.value as TransactionStatus }))
            }
          >
            <option value="Pending">Pending</option>
            <option value="Cleared">Cleared</option>
          </select>
        </div>
      </div>

      <div>
        <Label htmlFor="desc">Description</Label>
        <Input
          id="desc"
          type="text"
          value={state.description}
          onChange={(e) => setState((s) => ({ ...s, description: e.target.value }))}
          aria-invalid={!!fieldErrors.description}
        />
        {fieldErrors.description && (
          <p className="mt-1 text-xs text-destructive">{fieldErrors.description}</p>
        )}
      </div>

      {advanced ? (
        <SplitsEditor
          splits={state.splits}
          onChange={(next) => setState((s) => ({ ...s, splits: next }))}
          splitsError={fieldErrors.splits}
        />
      ) : (
        <SimpleFields
          fromAccount={state.fromAccount}
          toAccount={state.toAccount}
          amount={state.amount}
          onFromChange={(name) => setState((s) => ({ ...s, fromAccount: name }))}
          onToChange={(name) => setState((s) => ({ ...s, toAccount: name }))}
          onAmountChange={(value) => setState((s) => ({ ...s, amount: value }))}
          fieldErrors={{
            fromAccount: fieldErrors.fromAccount,
            toAccount: fieldErrors.toAccount,
            amount: fieldErrors.amount,
          }}
        />
      )}

      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" disabled={submitting}>
          {isEdit ? 'Save' : 'Create'}
        </Button>
      </div>
    </form>
  );
}
```

- [ ] **Step 2: Type-check**

```bash
cd /Users/hance/programming/kea/spa
npm run build
```

Expected: success. If TS complains about the `splits.some((s) => !Number.isFinite(s.amount))` narrowing or the `satisfies` clauses, add the necessary explicit types — do not loosen with `any`.

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/components/transactions/TransactionForm.tsx
git commit -m "feat(spa): add TransactionForm shell with Simple/Advanced toggle"
```

---

## Task 19: Create route `/transactions/new`

**Files:**
- Create: `spa/src/routes/transactions.new.tsx`

- [ ] **Step 1: Create the route**

```tsx
import { TransactionForm } from '@/components/transactions/TransactionForm';
import { createTransaction } from '@/lib/transactions';
import { useQueryClient } from '@tanstack/react-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions/new')({
  component: NewTransactionPage,
});

function NewTransactionPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  return (
    <TransactionForm
      mode="create"
      onSubmit={async (payload) => {
        // payload is CreateTransactionInput in create mode.
        const created = await createTransaction(
          payload as Parameters<typeof createTransaction>[0],
        );
        queryClient.invalidateQueries({ queryKey: ['transactions'] });
        queryClient.invalidateQueries({ queryKey: ['balances'] });
        return created;
      }}
      onSuccess={(tx) => navigate({ to: '/transactions/$id', params: { id: String(tx.id) } })}
      onCancel={() => navigate({ to: '/transactions' })}
    />
  );
}
```

- [ ] **Step 2: Regenerate route tree and build**

```bash
cd /Users/hance/programming/kea/spa
npm run dev &
sleep 5
kill %1 2>/dev/null
npm run build
```

Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/routes/transactions.new.tsx spa/src/routeTree.gen.ts
git commit -m "feat(spa): add create transaction route"
```

---

## Task 20: Edit route `/transactions/$id/edit`

**Files:**
- Create: `spa/src/routes/transactions.$id.edit.tsx`

- [ ] **Step 1: Create the route**

```tsx
import { ReconciledBanner } from '@/components/transactions/ReconciledBanner';
import { TransactionForm } from '@/components/transactions/TransactionForm';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { getTransaction, updateTransaction } from '@/lib/transactions';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions/$id/edit')({
  component: EditTransactionPage,
});

function EditTransactionPage() {
  const { id } = Route.useParams();
  const txId = Number(id);
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ['transaction', txId],
    queryFn: () => getTransaction(txId),
  });

  if (query.isPending) return <Skeleton className="h-48 w-full" />;
  if (query.isError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Failed to load transaction</AlertTitle>
        <AlertDescription>
          {query.error instanceof Error ? query.error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    );
  }

  const tx = query.data;
  if (tx.status === 'Reconciled') {
    return (
      <div className="space-y-4">
        <ReconciledBanner />
        <Button asChild variant="outline">
          <Link to="/transactions/$id" params={{ id: String(tx.id) }}>
            ← Back to detail
          </Link>
        </Button>
      </div>
    );
  }

  return (
    <TransactionForm
      mode="edit"
      initial={tx}
      onSubmit={async (payload) => {
        const updated = await updateTransaction(
          payload as Parameters<typeof updateTransaction>[0],
        );
        queryClient.invalidateQueries({ queryKey: ['transactions'] });
        queryClient.invalidateQueries({ queryKey: ['transaction', txId] });
        queryClient.invalidateQueries({ queryKey: ['balances'] });
        return updated;
      }}
      onSuccess={(updated) =>
        navigate({ to: '/transactions/$id', params: { id: String(updated.id) } })
      }
      onCancel={() =>
        navigate({ to: '/transactions/$id', params: { id: String(tx.id) } })
      }
    />
  );
}
```

- [ ] **Step 2: Regenerate route tree and build**

```bash
cd /Users/hance/programming/kea/spa
npm run dev &
sleep 5
kill %1 2>/dev/null
npm run build
```

Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/routes/transactions.\$id.edit.tsx spa/src/routeTree.gen.ts
git commit -m "feat(spa): add edit transaction route with reconciled lockout"
```

---

## Task 21: Form integration test

**Files:**
- Create: `spa/src/test/transactions.form.test.tsx`

- [ ] **Step 1: Create the test**

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

const ACCOUNTS = {
  items: [
    {
      id: 1,
      name: 'Assets:Bank',
      type: 'A',
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
    {
      id: 2,
      name: 'Expenses:Coffee',
      type: 'E',
      currency: 'USD',
      description: '',
      is_hidden: false,
    },
  ],
  total_count: 2,
  limit: 0,
  offset: 0,
};

let postedBody: unknown = null;

beforeEach(() => {
  postedBody = null;
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/config') {
        return Promise.resolve(okResponse({ defaults: { currency: 'USD' } }));
      }
      if (url === '/api/ledgers') {
        return Promise.resolve(
          okResponse({ active: 'p', items: [{ name: 'p', path: '/p', active: true }] }),
        );
      }
      if (url.startsWith('/api/accounts')) {
        return Promise.resolve(okResponse(ACCOUNTS));
      }
      if (url === '/api/transactions' && init?.method === 'POST') {
        postedBody = init.body ? JSON.parse(init.body as string) : null;
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: 42,
              timestamp: Math.floor(Date.now() / 1000),
              description: 'Coffee',
              status: 'Cleared',
              type: 'Expense',
              splits: [],
            }),
            { status: 201, headers: { 'Content-Type': 'application/json' } },
          ),
        );
      }
      if (url === '/api/transactions/42') {
        return Promise.resolve(
          okResponse({
            id: 42,
            timestamp: Math.floor(Date.now() / 1000),
            description: 'Coffee',
            status: 'Cleared',
            type: 'Expense',
            splits: [],
          }),
        );
      }
      throw new Error(`unexpected fetch: ${url} (method=${init?.method ?? 'GET'})`);
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('Simple mode submits a 2-split transaction', async () => {
  const user = userEvent.setup();
  render(makeTestApp('/transactions/new'));

  await user.type(await screen.findByLabelText('Description'), 'Coffee');

  // Type the From and To account names directly into the combobox inputs.
  await user.type(screen.getByLabelText('From account'), 'Assets:Bank');
  await user.type(screen.getByLabelText('To account'), 'Expenses:Coffee');
  await user.type(screen.getByLabelText('Amount'), '12.50');

  await user.click(screen.getByRole('button', { name: /create/i }));

  await waitFor(() => expect(postedBody).not.toBeNull());
  expect(postedBody).toMatchObject({
    description: 'Coffee',
    status: 'Cleared',
    splits: [
      { account_name: 'Assets:Bank', amount: -1250, currency: 'USD' },
      { account_name: 'Expenses:Coffee', amount: 1250, currency: 'USD' },
    ],
  });
});

test('Advanced toggle reveals the splits editor with balance indicator', async () => {
  const user = userEvent.setup();
  render(makeTestApp('/transactions/new'));

  // Toggle Advanced.
  await user.click(await screen.findByLabelText(/Advanced \(edit splits\)/));

  // Balance label appears.
  expect(await screen.findByText(/Balance:/)).toBeInTheDocument();
  // "+ Add split" button is visible.
  expect(screen.getByRole('button', { name: /add split/i })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the test**

```bash
cd /Users/hance/programming/kea/spa
npm test -- transactions.form
```

Expected: PASS (2 tests). If the combobox dropdown intercepts the typed input and causes test flake, debounce timing may need adjustment — use `vi.useFakeTimers()` + `vi.advanceTimersByTime(250)` to bypass the 200ms debounce before typing.

- [ ] **Step 3: Commit**

```bash
cd /Users/hance/programming/kea
git add spa/src/test/transactions.form.test.tsx
git commit -m "test(spa): integration test for transaction create form"
```

---

## Task 22: Full test pass + lint

- [ ] **Step 1: Run the entire SPA test suite**

```bash
cd /Users/hance/programming/kea/spa
npm test
```

Expected: all tests pass (including the existing balances, ledger-switcher, server-config tests plus the seven new tests added in this slice).

- [ ] **Step 2: Run lint**

```bash
npm run check
```

Expected: no errors. If Biome flags imports or formatting, run:
```bash
npm run check:write
```
then `npm run check` again. Review the autofixes before staging.

- [ ] **Step 3: Build**

```bash
npm run build
```

Expected: success.

- [ ] **Step 4: Commit any lint autofixes**

If `check:write` modified files:
```bash
cd /Users/hance/programming/kea
git add spa/
git commit -m "chore(spa): apply biome formatting to transactions slice"
```

If nothing changed, skip this commit.

---

## Task 23: Manual verification

**Goal:** Exercise the full slice in a browser against a real `kea serve` backend. This is required before claiming completion — unit/component tests don't catch routing, query-param round-trips, or visual regressions.

**Pre-flight:** ensure a ledger with some sample data exists. If working with a fresh DB, seed via CLI first:
```bash
cd /Users/hance/programming/kea
make build
./kea_test ledger add manual-test
./kea_test account add --name "Assets:Bank" --type A --currency USD
./kea_test account add --name "Expenses:Coffee" --type E --currency USD
./kea_test account add --name "Revenue:Salary" --type R --currency USD
./kea_test add --desc "Seed coffee" --amount 5 --from "Assets:Bank" --to "Expenses:Coffee" --status cleared
```

- [ ] **Step 1: Start backend and frontend**

In one terminal:
```bash
cd /Users/hance/programming/kea
./kea_test serve
```

In a second terminal:
```bash
cd /Users/hance/programming/kea/spa
npm run dev
```

Open the printed Vite URL (usually `http://localhost:5173`).

- [ ] **Step 2: Verify each item below; check the box only after the actual behavior matches**

- [ ] **a)** Sidebar shows "Transactions" as a clickable link (not greyed out).
- [ ] **b)** Clicking it lands on `/transactions`. The seeded transaction appears in the table.
- [ ] **c)** Filter by `Type = Expense` — URL updates to `?type=Expense`; only expense rows visible.
- [ ] **d)** Filter by date range (today − 7d to today) — URL has `start_time` and `end_time`; correct rows.
- [ ] **e)** Hit `Clear filters` — URL drops to `/transactions?limit=50&offset=0`; full list returns.
- [ ] **f)** Refresh the page on `/transactions?type=Expense` — filter persists, page reloads correctly.
- [ ] **g)** Click `+ New transaction` → land on `/transactions/new`.
- [ ] **h)** In Simple mode, enter description, pick From = Assets:Bank, To = Expenses:Coffee, amount = 3.50 → Type badge derives to "Expense" live → click Create → land on `/transactions/$id` with the splits showing the right signs.
- [ ] **i)** Click Edit on the new transaction → form opens in Advanced mode with splits pre-filled → change amount on one split, balance indicator turns red until you adjust the other split → save successfully.
- [ ] **j)** Click Delete on the same transaction → confirm dialog → confirm → land on `/transactions` with the row gone.
- [ ] **k)** Navigate to a Reconciled transaction (you may need to reconcile one via the CLI: `./kea_test reconcile`). On its detail page, the ReconciledBanner shows; Edit and Delete are absent.
- [ ] **l)** Manually navigate to `/transactions/<reconciled_id>/edit` → banner shows; form is not mounted; "Back to detail" link works.
- [ ] **m)** Switch ledger using the sidebar switcher → transactions list refetches against the new ledger; URL search params are preserved.
- [ ] **n)** Visit Balances after creating/editing/deleting a transaction → balances reflect the change (Query cache invalidation works).

- [ ] **Step 3: Stop dev server and backend**

`Ctrl-C` in each terminal.

- [ ] **Step 4: Note in the commit message any deviation from the spec**

If everything worked as designed, commit nothing — this task is verification only.

If anything diverged, file a small fix as a separate commit before considering the slice done.

---

## Done

If all 23 tasks are checked off:
- 6 new lib modules, 11 new components, 4 new routes, 7 test files, the Sidebar update, and the two new deps are in place.
- The Transactions sidebar item is live.
- The follow-up issues (#190–#195) remain open for the next slice.
